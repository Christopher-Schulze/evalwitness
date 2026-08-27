package mode

import (
	"context"
	"strings"
	"testing"
)

func TestScoreObserverReceivesDigestOnlyExecutionEvidence(t *testing.T) {
	providerFixture := &requestRecordingProvider{}
	runner := &Runner{
		Provider: providerFixture,
		Cfg: RunnerConfig{
			Model: "m", BaseURL: "https://mock.invalid/v1", ThinkingMode: "disabled",
			Entrypoint: "replay", Temperature: 1, TopLogprobs: 20,
		},
	}
	observations := []ScoreObservation{}
	scope := strings.Repeat("a", 64)
	ctx := ContextWithObservationScope(context.Background(), scope)
	ctx = ContextWithScoreObserver(ctx, func(observation ScoreObservation) error {
		observations = append(observations, observation)
		return nil
	})
	if _, err := runner.Score(ctx, "criterion@r0", "canonical prompt", nil, 128); err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 {
		t.Fatalf("score observations = %d, want 1", len(observations))
	}
	observation := observations[0]
	if observation.SchemaVersion != ScoreObservationSchemaVersion || observation.Scope != scope || observation.CriterionID != "criterion@r0" ||
		observation.Entrypoint != "replay" || observation.RequestFingerprint == "" || observation.ResponseBodyDigest == "" ||
		observation.ParsedPayloadDigest == "" || observation.ResponseEvidenceDigest == "" || observation.ScoreEvidenceDigest == "" ||
		observation.ProviderRequestID == "" || observation.ExtractionStatus != ExtractionObservationComplete || observation.ExtractionErrorDigest != "" {
		t.Fatalf("incomplete score observation: %+v", observation)
	}
}
