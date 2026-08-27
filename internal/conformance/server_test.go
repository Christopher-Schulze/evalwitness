package conformance

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func conformanceRequest(t *testing.T, baseURL string, stream bool) provider.RequestEnvelope {
	t.Helper()
	request, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID: "deterministic-conformance", BaseURL: baseURL, RequestedModel: "requested-alias",
		Messages: []provider.Message{{Role: "user", Content: "score the trajectory"}}, Temperature: 0.7,
		MaxOutputTokens: 64, Logprobs: true, TopLogprobs: 20, ScoreTags: []string{"<score_A>"}, Stream: stream,
	})
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func runScenario(t *testing.T, scenario Scenario, stream bool) (provider.ResponseRecord, *DeterministicServer, error) {
	t.Helper()
	handler, err := NewDeterministicServer(scenario)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := provider.New(provider.Config{
		Name: "deterministic-conformance", WireFormat: "openai", BaseURL: server.URL,
		APIKey: "synthetic", Model: "requested-alias", LiveIntent: true, MaxRetries: 1,
		Caps: provider.Capabilities{Logprobs: true, TopLogprobsMax: 20, Streaming: stream},
	})
	if err != nil {
		t.Fatal(err)
	}
	response, scoreErr := client.Score(context.Background(), conformanceRequest(t, server.URL, stream))
	return response, handler, scoreErr
}

func TestDeterministicConformanceFailureMatrix(t *testing.T) {
	tests := []struct {
		scenario    Scenario
		degradation verifier.DegradationCode
	}{
		{ScenarioMissingTopK, verifier.DegradationInsufficientTopK},
		{ScenarioLowScoreMass, verifier.DegradationLowScoreMass},
		{ScenarioTruncation, verifier.DegradationResponseTruncated},
		{ScenarioDegenerate, verifier.DegradationDegenerateLogprobs},
	}
	for _, test := range tests {
		t.Run(string(test.scenario), func(t *testing.T) {
			response, _, err := runScenario(t, test.scenario, false)
			if err != nil {
				t.Fatal(err)
			}
			request := conformanceRequest(t, "https://conformance.invalid", false)
			evidence := verifier.ExtractAllScoreEvidence(request, response, verifier.ExtractionModeVerifier)["<score_A>"]
			if evidence.Extracted || !containsDegradation(evidence, test.degradation) {
				t.Fatalf("extracted=%t degradations=%+v", evidence.Extracted, evidence.Degradations)
			}
		})
	}
}

func TestDeterministicConformanceOperationalScenarios(t *testing.T) {
	response, _, err := runScenario(t, ScenarioMissingUsage, true)
	if err != nil {
		t.Fatal(err)
	}
	if response.Usage != (provider.TokenUsage{}) {
		t.Fatalf("missing-usage scenario returned %+v", response.Usage)
	}
	if response.UsageObserved {
		t.Fatal("missing-usage scenario claimed upstream usage was observed")
	}

	response, _, err = runScenario(t, ScenarioAliasMismatch, false)
	if err != nil {
		t.Fatal(err)
	}
	if response.ServedModel != "different-served-alias" {
		t.Fatalf("served model = %q", response.ServedModel)
	}
	if !response.UsageObserved {
		t.Fatal("valid conformance response did not record upstream usage presence")
	}

	_, _, err = runScenario(t, ScenarioMalformedSSE, true)
	var providerError *provider.ProviderError
	if !errors.As(err, &providerError) || providerError.Class != provider.ClassMalformedResponse || !errors.Is(err, provider.ErrMalformedResponse) {
		t.Fatalf("malformed SSE error = %T %v", err, err)
	}
}

func TestDeterministicRetryAfterHonorsCallerDeadline(t *testing.T) {
	handler, err := NewDeterministicServer(ScenarioDelayedRetryAfter)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, err := provider.New(provider.Config{
		Name: "deterministic-conformance", WireFormat: "openai", BaseURL: server.URL,
		APIKey: "synthetic", Model: "requested-alias", LiveIntent: true, MaxRetries: 3,
		Caps: provider.Capabilities{Logprobs: true, TopLogprobsMax: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = client.Score(ctx, conformanceRequest(t, server.URL, false))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("retry-after error = %v", err)
	}
	if handler.Requests() != 1 {
		t.Fatalf("requests = %d, want one before total deadline", handler.Requests())
	}
}

func containsDegradation(evidence verifier.ScoreEvidence, want verifier.DegradationCode) bool {
	for _, degradation := range evidence.Degradations {
		if degradation.Code == want {
			return true
		}
	}
	return false
}
