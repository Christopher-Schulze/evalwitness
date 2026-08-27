package verification

import (
	"strings"
	"testing"
)

func TestBatchPacingChangesPlanAndAuthorizationIdentity(t *testing.T) {
	manifestDigest := strings.Repeat("a", 64)
	service := batchBindingService(t, false)
	input := batchBindingInput("cell-1", "case-1", "original", manifestDigest)
	input.Policy.MaxWorkers = 1
	unpaced, err := service.PlanBatch([]Input{input})
	if err != nil {
		t.Fatal(err)
	}
	input.Policy.MinDispatchIntervalSeconds = 90
	paced, err := service.PlanBatch([]Input{input})
	if err != nil {
		t.Fatal(err)
	}
	if unpaced.RunFingerprint == paced.RunFingerprint {
		t.Fatal("batch pacing did not change run identity")
	}
	if unpaced.Authorization.AuthorizationDigest == paced.Authorization.AuthorizationDigest {
		t.Fatal("batch pacing did not change live authorization identity")
	}
}

func TestBatchPacingRejectsConcurrentWorkers(t *testing.T) {
	input := batchBindingInput("cell-1", "case-1", "original", strings.Repeat("a", 64))
	input.Policy.MinDispatchIntervalSeconds = 90
	if _, err := batchBindingService(t, false).PlanBatch([]Input{input}); err == nil || !strings.Contains(err.Error(), "requires max_workers=1") {
		t.Fatalf("concurrent pacing error = %v", err)
	}
}
