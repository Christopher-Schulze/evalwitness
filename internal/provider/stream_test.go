package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Streaming is the default request path (EVALWITNESS_STREAM defaults to true) and
// had no test at all. It is also where a published artifact once reported
// input_tokens 0 across 326 calls: the stream was cut the moment the score tags
// closed, and providers emit the usage chunk last.

// sseChunk renders one delta with optional logprobs for the emitted text.
func sseChunk(t *testing.T, content string, withLogprobs bool) string {
	t.Helper()
	type top struct {
		Token   string  `json:"token"`
		Logprob float64 `json:"logprob"`
	}
	type tok struct {
		Token       string  `json:"token"`
		Logprob     float64 `json:"logprob"`
		TopLogprobs []top   `json:"top_logprobs"`
	}
	chunk := map[string]any{
		"id":    "chatcmpl-test",
		"model": "served-model-name",
		"choices": []map[string]any{{
			"delta": map[string]any{"content": content},
		}},
	}
	if withLogprobs {
		chunk["choices"].([]map[string]any)[0]["logprobs"] = map[string]any{
			"content": []tok{{
				Token:   content,
				Logprob: -0.01,
				TopLogprobs: []top{
					{Token: content, Logprob: -0.01},
					{Token: "T", Logprob: -4.5},
				},
			}},
		}
	}
	b, err := json.Marshal(chunk)
	if err != nil {
		t.Fatal(err)
	}
	return "data: " + string(b) + "\n\n"
}

func usageChunk(t *testing.T, prompt, completion, cached int) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-test",
		"model":   "served-model-name",
		"choices": []map[string]any{},
		"usage": map[string]any{
			"prompt_tokens":         prompt,
			"completion_tokens":     completion,
			"prompt_tokens_details": map[string]any{"cached_tokens": cached},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return "data: " + string(b) + "\n\n"
}

func streamServer(t *testing.T, body func() string) (*httptest.Server, *int32) {
	t.Helper()
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		var decoded map[string]any
		if err := json.NewDecoder(r.Body).Decode(&decoded); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		if decoded["stream"] != true {
			t.Errorf("stream flag = %v, want true", decoded["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := fmt.Fprint(w, body()); err != nil {
			t.Errorf("write stream body: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func streamingProvider(t *testing.T, url string) Provider {
	t.Helper()
	p, err := newOpenAI(Config{
		Name: "deepseek", BaseURL: url, APIKey: "test", Model: "m",
		MaxRetries: 0, Caps: Capabilities{Logprobs: true, TopLogprobsMax: 20, Streaming: true}, LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStreamCollectsUsageEmittedAfterTheScoreTags(t *testing.T) {
	// The regression that produced input_tokens 0 for a whole published run.
	// Usage arrives after the closing tag, so stopping at the tag loses it.
	srv, _ := streamServer(t, func() string {
		return sseChunk(t, "<score_A>", false) +
			sseChunk(t, "B", true) +
			sseChunk(t, "</score_A>", false) +
			usageChunk(t, 1234, 7, 900) +
			"data: [DONE]\n\n"
	})
	p := streamingProvider(t, srv.URL)

	got, err := p.Score(context.Background(), streamRequestForTest(t, p, "x", 64, "<score_A>"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Usage.Input != 1234 {
		t.Fatalf("input tokens = %d, want 1234; the usage chunk after the score tag was dropped", got.Usage.Input)
	}
	if got.Usage.Output != 7 {
		t.Fatalf("output tokens = %d, want 7", got.Usage.Output)
	}
	if got.Usage.Cached != 900 {
		t.Fatalf("cached tokens = %d, want 900", got.Usage.Cached)
	}
	if got.ServedModel != "served-model-name" {
		t.Fatalf("served model = %q; provenance is lost", got.ServedModel)
	}
	if !got.HasLogprobs || len(got.Distributions) == 0 {
		t.Fatal("logprobs from the stream did not reach the response")
	}
}

func TestStreamStopsAfterABoundedDrainWhenUsageNeverArrives(t *testing.T) {
	// A model that keeps talking after the score tags must not hold the run
	// open. The drain is bounded, and the response is still usable without
	// usage rather than being an error.
	var filler strings.Builder
	for range maxUsageDrainChunks + 40 {
		filler.WriteString(sseChunk(t, "still talking ", false))
	}
	srv, _ := streamServer(t, func() string {
		return sseChunk(t, "<score_A>", true) +
			sseChunk(t, "C", true) +
			sseChunk(t, "</score_A>", false) +
			filler.String() +
			"data: [DONE]\n\n"
	})
	p := streamingProvider(t, srv.URL)

	got, err := p.Score(context.Background(), streamRequestForTest(t, p, "x", 64, "<score_A>"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.RawText, "</score_A>") {
		t.Fatal("the score tag never closed in the collected text")
	}
	if strings.Contains(got.RawText, "still talking") {
		t.Fatalf("post-score content entered semantic evidence: %q", got.RawText)
	}
	if !strings.Contains(string(got.NormalizedBody), "still talking") {
		t.Fatal("the bounded usage drain did not retain transport evidence")
	}
}

func TestStreamReadsToCompletionWhenTheTagsNeverClose(t *testing.T) {
	// A model that answers without ever emitting the closing tag gives the
	// early stop nothing to trigger on. The stream must then be read to the end,
	// so usage is still collected and the extraction gate sees the real text and
	// can reject it, rather than the run failing on a truncated response.
	srv, _ := streamServer(t, func() string {
		return sseChunk(t, "I decline ", false) +
			sseChunk(t, "to score this", false) +
			usageChunk(t, 11, 2, 0) +
			"data: [DONE]\n\n"
	})
	p := streamingProvider(t, srv.URL)

	got, err := p.Score(context.Background(), streamRequestForTest(t, p, "x", 64, "<score_A>"))
	if err != nil {
		t.Fatal(err)
	}
	if got.RawText != "I decline to score this" {
		t.Fatalf("raw text = %q", got.RawText)
	}
	if got.Usage.Input != 11 {
		t.Fatalf("input tokens = %d, want 11; usage was dropped", got.Usage.Input)
	}
	if got.HasLogprobs {
		t.Fatal("a response with no logprob tokens reported HasLogprobs")
	}
}

func TestStreamRejectsMalformedDataFrames(t *testing.T) {
	// Keep-alive comments remain ignorable SSE metadata. A malformed data frame
	// is response evidence, so accepting later chunks would silently publish a
	// partial normalized body as complete.
	srv, _ := streamServer(t, func() string {
		return ": keep-alive\n\n" +
			"data: {not json at all\n\n" +
			sseChunk(t, "<score_A>", false) +
			sseChunk(t, "A", true) +
			sseChunk(t, "</score_A>", false) +
			usageChunk(t, 50, 3, 0) +
			"data: [DONE]\n\n"
	})
	p := streamingProvider(t, srv.URL)

	_, err := p.Score(context.Background(), streamRequestForTest(t, p, "x", 64, "<score_A>"))
	if err == nil || !strings.Contains(err.Error(), "parse stream chunk") {
		t.Fatalf("malformed data frame error = %v", err)
	}
	if strings.Contains(err.Error(), "http 0") {
		t.Fatalf("malformed response rendered a synthetic HTTP status: %v", err)
	}
}

func TestStreamSurfacesHTTPErrorsAsProviderErrors(t *testing.T) {
	// A 400 rather than a 429 on purpose: a rate-limited status is retried with
	// the provider's own Retry-After, which would make this a sleeping test. The
	// error shape is the same and Retry-After parsing is covered separately.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := fmt.Fprint(w, `{"error":{"message":"does not support return_logprob yet"}}`); err != nil {
			t.Errorf("write stream error body: %v", err)
		}
	}))
	defer srv.Close()
	p := streamingProvider(t, srv.URL)

	_, err := p.Score(context.Background(), streamRequestForTest(t, p, "x", 16, "<score_A>"))
	if err == nil {
		t.Fatal("a failing stream returned no error")
	}
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is %T, want *ProviderError", err)
	}
	if pe.Status != 400 {
		t.Fatalf("status = %d, want 400", pe.Status)
	}
	if pe.RetryAfter != 7 {
		t.Fatalf("retry after = %d, want the header parsed even on a fatal status", pe.RetryAfter)
	}
	// The body has to survive: this exact message is how a route that passes a
	// probe and fails real work identifies itself.
	if !strings.Contains(pe.Body, "return_logprob") {
		t.Fatalf("body = %q, want the provider's own message preserved", pe.Body)
	}
}

func TestStreamHonorsContextCancellation(t *testing.T) {
	srv, _ := streamServer(t, func() string {
		return sseChunk(t, "partial", false)
	})
	p := streamingProvider(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Score(ctx, streamRequestForTest(t, p, "x", 16, "<score_A>")); err == nil {
		t.Fatal("a cancelled context produced no error")
	}
}
