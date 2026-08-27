package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIRequestsCarryEvalWitnessUserAgent(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		streaming := streaming
		t.Run(fmt.Sprintf("streaming=%t", streaming), func(t *testing.T) {
			header := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				header <- r.Header.Get("User-Agent")
				if streaming {
					w.Header().Set("Content-Type", "text/event-stream")
					if _, err := fmt.Fprintln(w, `data: {"id":"request-1","model":"served","choices":[{"delta":{"content":"<score_A>A</score_A>"},"logprobs":null}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`); err != nil {
						t.Errorf("write streaming response: %v", err)
					}
					if _, err := fmt.Fprintln(w, "data: [DONE]"); err != nil {
						t.Errorf("write streaming terminator: %v", err)
					}
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := fmt.Fprintln(w, `{"id":"request-1","model":"served","choices":[{"message":{"role":"assistant","content":"A"},"logprobs":null}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			p, err := newOpenAI(Config{
				Name:       "test",
				WireFormat: "openai",
				BaseURL:    server.URL,
				APIKey:     "test-key",
				Model:      "model",
				Timeout:    5,
				MaxRetries: 1,
				LiveIntent: true,
				UserAgent:  "evalwitness/0.1.0-test",
				Caps:       Capabilities{Streaming: streaming, TopLogprobsMax: 20},
			})
			if err != nil {
				t.Fatal(err)
			}
			req := requestForTest(t, p, "test", 8)
			if streaming {
				req = streamRequestForTest(t, p, "test", 8, "<score_A>")
			}
			if _, err := p.Score(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if got := <-header; got != "evalwitness/0.1.0-test" {
				t.Fatalf("User-Agent = %q", got)
			}
		})
	}
}

func TestServedIdentityPoliciesRejectUnknownModelsAndNoncanonicalSets(t *testing.T) {
	tests := []struct {
		name           string
		policy         string
		expectedModel  string
		expectedModels []string
		observedModel  string
		wantErr        bool
	}{
		{name: "exact match", policy: ServedIdentityPolicyExactObserved, expectedModel: "deepseek-v4-flash", observedModel: "deepseek-v4-flash"},
		{name: "exact mismatch", policy: ServedIdentityPolicyExactObserved, expectedModel: "deepseek-v4-flash", observedModel: "deepseek-v4-flash-202605", wantErr: true},
		{name: "set member", policy: ServedIdentityPolicyExactObservedSet, expectedModels: []string{"deepseek-v4-flash", "deepseek-v4-flash-202605"}, observedModel: "deepseek-v4-flash-202605"},
		{name: "unknown set member", policy: ServedIdentityPolicyExactObservedSet, expectedModels: []string{"deepseek-v4-flash", "deepseek-v4-flash-202605"}, observedModel: "deepseek-v4-flash-unknown", wantErr: true},
		{name: "unsorted set", policy: ServedIdentityPolicyExactObservedSet, expectedModels: []string{"deepseek-v4-flash-202605", "deepseek-v4-flash"}, observedModel: "deepseek-v4-flash", wantErr: true},
		{name: "duplicate set", policy: ServedIdentityPolicyExactObservedSet, expectedModels: []string{"deepseek-v4-flash", "deepseek-v4-flash"}, observedModel: "deepseek-v4-flash", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := MatchServedIdentity(test.policy, test.expectedModel, test.expectedModels, test.observedModel)
			if (err != nil) != test.wantErr {
				t.Fatalf("MatchServedIdentity() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
