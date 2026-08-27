package provider

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func writeOK(w http.ResponseWriter) {
	resp := map[string]any{
		"id": "test-1",
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "<score>A</score>",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func TestOpenAIRetryOn429(t *testing.T) {
	var calls int32
	var reservedAttempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		writeOK(w)
	}))
	defer server.Close()

	p, err := newOpenAI(Config{
		Name: "openai", BaseURL: server.URL, APIKey: "test", Model: "test-model",
		MaxRetries: 5, LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := requestForTest(t, p, "test", 10)
	request.Temperature = 0
	request.BeforeAttempt = func() error {
		atomic.AddInt32(&reservedAttempts, 1)
		return nil
	}
	resp, err := p.Score(context.Background(), request)
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
	if got := atomic.LoadInt32(&reservedAttempts); got != 3 {
		t.Errorf("expected 3 reserved attempts, got %d", got)
	}
	if !strings.Contains(resp.RawText, "<score>A</score>") {
		t.Errorf("unexpected response: %q", resp.RawText)
	}
}

func TestOpenAIAuthFailedNoRetry(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"unauthorized: invalid api key"}}`))
	}))
	defer server.Close()

	p, _ := newOpenAI(Config{
		Name: "openai", BaseURL: server.URL, APIKey: "bad", Model: "test-model",
		MaxRetries: 5, LiveIntent: true,
	})
	_, err := p.Score(context.Background(), requestForTest(t, p, "test", 10))
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected single attempt for auth failure, got %d", got)
	}
}

func TestOpenAIPaymentRequiredNoRetry(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"message":"Prompt tokens limit exceeded: 553639 > 476873. Add credits."}}`))
	}))
	defer server.Close()

	p, err := newOpenAI(Config{
		Name: "openrouter-ambient", BaseURL: server.URL, APIKey: "test", Model: "test-model",
		MaxRetries: 5, LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Score(context.Background(), requestForTest(t, p, "test", 10))
	if err == nil {
		t.Fatal("expected payment-required error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("HTTP attempts = %d, want 1", got)
	}
}

func TestOpenAINumericErrorCodeRetries(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			_, _ = w.Write([]byte(`{"error":{"message":"upstream unavailable","type":"provider_error","code":503}}`))
			return
		}
		writeOK(w)
	}))
	defer server.Close()

	p, err := newOpenAI(Config{
		Name: "deepseek", BaseURL: server.URL, APIKey: "test", Model: "test-model",
		MaxRetries: 1, LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Score(context.Background(), requestForTest(t, p, "test", 10)); err != nil {
		t.Fatalf("numeric retryable error code must be accepted and retried: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestOpenAIOfflineRefuses(t *testing.T) {
	p, _ := newOpenAI(Config{
		Name: "openai", BaseURL: "http://nope", APIKey: "x", Model: "test-model",
		Offline: true,
	})
	_, err := p.Score(context.Background(), requestForTest(t, p, "test", 10))
	if err != ErrOffline {
		t.Errorf("expected ErrOffline, got %v", err)
	}
}

func TestOpenAIRequiresExplicitLiveIntentBeforeDispatch(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeOK(w)
	}))
	defer server.Close()

	p, err := newOpenAI(Config{
		Name: "openai", BaseURL: server.URL, APIKey: "test", Model: "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Score(context.Background(), requestForTest(t, p, "test", 10))
	if err != ErrLiveIntentRequired {
		t.Fatalf("error = %v, want ErrLiveIntentRequired", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0", got)
	}
}

func TestOpenAICustomCAFile(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeOK(w)
	}))
	defer server.Close()

	certificate := server.Certificate()
	if certificate == nil {
		t.Fatal("TLS test server has no certificate")
	}
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newOpenAI(Config{
		Name: "openai", BaseURL: server.URL, APIKey: "test", Model: "test-model", CAFile: caPath,
		LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Score(context.Background(), requestForTest(t, p, "test", 10)); err != nil {
		t.Fatalf("custom CA request failed: %v", err)
	}
}

func TestClassifyError(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   ErrorClass
	}{
		{429, "rate limited", ClassRateLimited},
		{402, "Prompt tokens limit exceeded: add credits", ClassPaymentRequired},
		{500, "internal error", ClassRetryable},
		{401, "invalid api key", ClassAuthFailed},
		{400, "context length exceeded", ClassContextOverflow},
		{400, "logprobs not supported", ClassCapabilityMissing},
		{404, "model not found", ClassBadConfig},
	}
	for _, c := range cases {
		got := classifyError(c.status, c.body)
		if got != c.want {
			t.Errorf("classifyError(%d, %q) = %v, want %v", c.status, c.body, got, c.want)
		}
	}
}

func TestOpenAILogprobsParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": "test-2",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "<score>A</score>",
					},
					"finish_reason": "stop",
					"logprobs": map[string]any{
						"content": []any{
							map[string]any{
								"token":   "<score>",
								"logprob": -0.1,
								"top_logprobs": []any{
									map[string]any{"token": "<score>", "logprob": -0.1},
								},
							},
							map[string]any{
								"token":   "A",
								"logprob": -0.05,
								"top_logprobs": []any{
									map[string]any{"token": "A", "logprob": -0.05},
									map[string]any{"token": "B", "logprob": -2.5},
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 3},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := newOpenAI(Config{
		Name: "openai", BaseURL: server.URL, APIKey: "test", Model: "m",
		Caps: Capabilities{TopLogprobsMax: 20}, LiveIntent: true,
	})
	request := requestForTest(t, p, "test", 10)
	request.Logprobs = true
	request.TopLogprobs = 20
	request.ScoreTags = []string{"<score>"}
	resp, err := p.Score(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.HasLogprobs {
		t.Errorf("expected HasLogprobs=true")
	}
	dist := resp.Distributions["<score>"]
	if dist == nil {
		t.Fatalf("missing distribution for <score>")
	}
	if _, ok := dist["A"]; !ok {
		t.Errorf("expected token A in distribution")
	}
}

func TestOpenAILogprobsParsingMergedScoreToken(t *testing.T) {
	dist := extractDistributions("<score>A</score>", []openaiLogprobToken{
		{Token: "<", Logprob: -0.01},
		{Token: "score", Logprob: -0.01},
		{
			Token:   ">A",
			Logprob: -0.05,
			TopLogprobs: []openaiLogprobAlternative{
				{Token: ">A", Logprob: -0.05},
				{Token: ">B", Logprob: -2.5},
				{Token: " A", Logprob: -9},
			},
		},
		{Token: "</", Logprob: -0.01},
		{Token: "score>", Logprob: -0.01},
	}, []string{"<score>"})["<score>"]

	if dist == nil {
		t.Fatalf("missing distribution for merged score token")
	}
	if _, ok := dist["A"]; !ok {
		t.Fatalf("expected token A in distribution: %#v", dist)
	}
	if dist["A"] < 0.9 {
		t.Fatalf("lower-probability A alias overwrote chosen token: %#v", dist)
	}
	if _, ok := dist["B"]; !ok {
		t.Fatalf("expected token B in distribution: %#v", dist)
	}
	if _, ok := dist[">A"]; ok {
		t.Fatalf("expected normalized score letters, got raw merged token: %#v", dist)
	}
}

func TestExtractDistributionsAlignsAcrossOmittedMarkupTokens(t *testing.T) {
	dist := extractDistributions("<score_A>A</score_A>", []openaiLogprobToken{
		{Token: "score", Logprob: -0.01},
		{Token: "_A", Logprob: -0.01},
		{Token: ">", Logprob: -0.01},
		{
			Token:   "A",
			Logprob: -0.05,
			TopLogprobs: []openaiLogprobAlternative{
				{Token: "A", Logprob: -0.05},
				{Token: "B", Logprob: -2.5},
			},
		},
		{Token: "score", Logprob: -0.01},
		{Token: "_A", Logprob: -0.01},
		{Token: ">", Logprob: -0.01},
	}, []string{"<score_A>"})["<score_A>"]

	if _, ok := dist["A"]; !ok {
		t.Fatalf("missing chosen score after markup-gap alignment: %#v", dist)
	}
	if _, ok := dist["B"]; !ok {
		t.Fatalf("missing alternative score after markup-gap alignment: %#v", dist)
	}
}

func TestExtractDistributionsUsesProviderTokenCoordinatesAcrossMarkupGaps(t *testing.T) {
	raw := "analysis mentions score repeatedly\n<score_A_error_signals>A</score_A_error_signals>\n<score_B_error_signals>B</score_B_error_signals>"
	tokens := []openaiLogprobToken{
		{Token: "analysis example <score_A_error_signals>", Logprob: 0},
		{
			Token:   "L",
			Logprob: -0.01,
			TopLogprobs: []openaiLogprobAlternative{
				{Token: "L", Logprob: -0.01},
				{Token: "M", Logprob: -2.5},
			},
		},
		{Token: "</score_A_error_signals>\n", Logprob: 0},
		{Token: "score", Logprob: 0},
		{Token: "_A", Logprob: 0},
		{Token: "_error", Logprob: 0},
		{Token: "_sign", Logprob: 0},
		{Token: "als", Logprob: 0},
		{
			Token:   ">A",
			Logprob: -0.01,
			TopLogprobs: []openaiLogprobAlternative{
				{Token: ">A", Logprob: -0.01},
				{Token: ">C", Logprob: -2.5},
			},
		},
		{Token: "score", Logprob: 0},
		{Token: "_A", Logprob: 0},
		{Token: "_error", Logprob: 0},
		{Token: "_sign", Logprob: 0},
		{Token: "als", Logprob: 0},
		{Token: ">\n", Logprob: 0},
		{Token: "score", Logprob: 0},
		{Token: "_B", Logprob: 0},
		{Token: "_error", Logprob: 0},
		{Token: "_sign", Logprob: 0},
		{Token: "als", Logprob: 0},
		{Token: ">", Logprob: 0},
		{
			Token:   "B",
			Logprob: -0.02,
			TopLogprobs: []openaiLogprobAlternative{
				{Token: "B", Logprob: -0.02},
				{Token: "D", Logprob: -3},
			},
		},
	}
	tags := []string{"<score_A_error_signals>", "<score_B_error_signals>"}
	distributions := extractDistributions(raw, tokens, tags)
	if got := distributions[tags[0]]["A"]; got < 0.9 {
		t.Fatalf("merged A score probability = %v, want > 0.9; distribution=%#v", got, distributions[tags[0]])
	}
	if got := distributions[tags[1]]["B"]; got < 0.9 {
		t.Fatalf("separate B score probability = %v, want > 0.9; distribution=%#v", got, distributions[tags[1]])
	}
}

func TestExtractDistributionsRejectsClosingTagFollowedByOmittedNewline(t *testing.T) {
	raw := "<score_A>H</score_A>\n<score_B>T</score_B>"
	tokens := []openaiLogprobToken{
		{Token: "score_A>", Logprob: 0},
		{Token: "H", Logprob: -0.1, TopLogprobs: []openaiLogprobAlternative{{Token: "H", Logprob: -0.1}, {Token: "J", Logprob: -2}}},
		{Token: "score_A>", Logprob: 0},
		// A gateway can omit both closing markup and the newline. The next
		// token begins with lowercase s, which is also a valid score letter.
		{Token: "score_B>", Logprob: 0, TopLogprobs: []openaiLogprobAlternative{{Token: "S", Logprob: -4}}},
		{Token: "T", Logprob: -0.2, TopLogprobs: []openaiLogprobAlternative{{Token: "T", Logprob: -0.2}, {Token: "S", Logprob: -1.8}}},
		{Token: "score_B>", Logprob: 0},
	}
	distributions := extractDistributions(raw, tokens, []string{"<score_A>", "<score_B>"})
	if got := distributions["<score_A>"]["H"]; got < 0.8 {
		t.Fatalf("score A distribution selected closing-tag drift: %#v", distributions["<score_A>"])
	}
	if got := distributions["<score_B>"]["T"]; got < 0.8 {
		t.Fatalf("score B distribution = %#v", distributions["<score_B>"])
	}
}

func TestOpenAISendsThinkingModeWhenConfigured(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeOK(w)
	}))
	defer server.Close()

	p, err := newOpenAI(Config{
		Name:       "deepseek",
		BaseURL:    server.URL,
		APIKey:     "test",
		Model:      "deepseek-v4-pro",
		Thinking:   "disabled",
		Caps:       Capabilities{Logprobs: true, TopLogprobsMax: 20},
		LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := requestForTest(t, p, "test", 10)
	request.Logprobs = true
	request.TopLogprobs = 20
	_, err = p.Score(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	thinking, ok := got["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("missing thinking object in request: %#v", got)
	}
	if thinking["type"] != "disabled" {
		t.Fatalf("thinking.type = %#v, want disabled", thinking["type"])
	}
}

func TestOpenAIOmitsThinkingModeByDefault(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeOK(w)
	}))
	defer server.Close()

	p, err := newOpenAI(Config{
		Name:       "openai",
		BaseURL:    server.URL,
		APIKey:     "test",
		Model:      "gpt-test",
		LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Score(context.Background(), requestForTest(t, p, "test", 10))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["thinking"]; ok {
		t.Fatalf("thinking object should be omitted by default: %#v", got)
	}
}

func TestOpenRouterRequestPinsUpstreamAndDisablesFallbacks(t *testing.T) {
	var got openaiChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeOK(w)
	}))
	defer server.Close()

	p, err := newOpenAI(Config{
		Name: "openrouter-morph", BaseURL: server.URL, APIKey: "test", Model: "deepseek/deepseek-v4-flash-0731",
		UpstreamProvider: "morph", LiveIntent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Score(context.Background(), requestForTest(t, p, "test", 10)); err != nil {
		t.Fatal(err)
	}
	if got.Provider == nil {
		t.Fatal("missing provider routing object")
	}
	if len(got.Provider.Only) != 1 || got.Provider.Only[0] != "morph" {
		t.Fatalf("provider.only = %#v", got.Provider.Only)
	}
	if !got.Provider.RequireParameters || got.Provider.AllowFallbacks {
		t.Fatalf("provider routing flags = require_parameters:%t allow_fallbacks:%t", got.Provider.RequireParameters, got.Provider.AllowFallbacks)
	}
}

func TestOpenRouterUpstreamMustMatchProviderIdentity(t *testing.T) {
	_, err := newOpenAI(Config{
		Name: "openrouter-ambient", APIKey: "test", UpstreamProvider: "morph",
	})
	if err == nil || !strings.Contains(err.Error(), `must bind pinned upstream "morph" as "openrouter-morph"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestClientTimeoutRetriesWhenCallerContextAlive(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			time.Sleep(3 * time.Second) // exceed the 1s client timeout once
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"message":{"role":"assistant","content":"<score>A</score>"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer srv.Close()

	p, err := newOpenAI(Config{Name: "test", BaseURL: srv.URL, APIKey: "k", Model: "m", Timeout: 1, LiveIntent: true})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Score(context.Background(), requestForTest(t, p, "hi", 8))
	if err != nil {
		t.Fatalf("client timeout must be retried, got: %v", err)
	}
	if resp.RawText != "<score>A</score>" {
		t.Errorf("raw = %q", resp.RawText)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Errorf("expected retry after client timeout, calls=%d", calls)
	}
}

func TestCallerCancelAbortsWithoutRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	p, _ := newOpenAI(Config{Name: "test", BaseURL: srv.URL, APIKey: "k", Model: "m", Timeout: 30, LiveIntent: true})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := p.Score(ctx, requestForTest(t, p, "hi", 8))
	if err == nil {
		t.Fatal("expected error on caller cancel")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("caller cancel must not retry, calls=%d", got)
	}
}

func TestDegenerateLogprobsAreDetected(t *testing.T) {
	// A stack that divides logits by the sampling temperature emits this at
	// temperature 0: the chosen token at logprob 0 and every alternative pinned
	// to a sentinel. It is argmax, not a distribution, and downstream it would
	// look like a maximally confident verifier answer with mass 1 and variance 0.
	sentinel := []openaiLogprobToken{
		{Token: "A", Logprob: 0, TopLogprobs: []openaiLogprobAlternative{
			{Token: "A", Logprob: 0}, {Token: "B", Logprob: -9999}, {Token: "C", Logprob: -9999},
		}},
		{Token: "<", Logprob: 0, TopLogprobs: []openaiLogprobAlternative{
			{Token: "<", Logprob: 0}, {Token: " ", Logprob: -9999},
		}},
	}
	if !logprobsAreDegenerate(sentinel) {
		t.Fatal("sentinel alternatives must be reported as degenerate")
	}

	real := []openaiLogprobToken{
		{Token: "A", Logprob: -0.386, TopLogprobs: []openaiLogprobAlternative{
			{Token: "A", Logprob: -0.386}, {Token: "B", Logprob: -1.14}, {Token: "E", Logprob: -10.51},
		}},
		{Token: "<", Logprob: -0.01, TopLogprobs: []openaiLogprobAlternative{
			{Token: "<", Logprob: -0.01}, {Token: " ", Logprob: -6.2},
		}},
	}
	if logprobsAreDegenerate(real) {
		t.Fatal("a real distribution must not be reported as degenerate")
	}

	// One genuinely certain token inside an otherwise informative response is
	// not degeneracy.
	mixed := []openaiLogprobToken{
		{Token: "A", Logprob: 0, TopLogprobs: []openaiLogprobAlternative{
			{Token: "A", Logprob: 0}, {Token: "B", Logprob: -9999},
		}},
		{Token: "x", Logprob: -0.2, TopLogprobs: []openaiLogprobAlternative{
			{Token: "x", Logprob: -0.2}, {Token: "y", Logprob: -2.1},
		}},
	}
	if logprobsAreDegenerate(mixed) {
		t.Fatal("a single certain token must not trip the guard")
	}

	// Too little evidence to judge.
	if logprobsAreDegenerate(sentinel[:1]) {
		t.Fatal("a single token is not enough evidence for degeneracy")
	}
}
