package lineage

import "testing"

func TestCorpusFeasibilityPreservesFrozenNegativeDecision(t *testing.T) {
	decision, err := BuildVerificationLineageCorpusFeasibilityDecision("../..")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Threshold.Passed || decision.Threshold.EligibleTaskGroupShortfall != 40 || decision.ClaimBoundary.FutureImpossibility {
		t.Fatalf("invalid corpus-feasibility decision: %#v", decision)
	}
}

func TestCorpusFeasibilityRejectsResealedThresholdWeakening(t *testing.T) {
	decision, err := BuildVerificationLineageCorpusFeasibilityDecision("../..")
	if err != nil {
		t.Fatal(err)
	}
	decision.Threshold.RequiredCalibrationTaskGroups = 0
	decision.Threshold.RequiredLockedTestTaskGroups = 0
	decision.Threshold.RequiredEligibleTaskGroups = 0
	decision.Threshold.EligibleTaskGroupShortfall = 0
	decision.Threshold.Passed = true
	decision.Digest, err = corpusFeasibilityDecisionDigest(decision)
	if err != nil {
		t.Fatal(err)
	}
	if err := decision.Validate("../.."); err == nil {
		t.Fatal("resealed threshold weakening was accepted")
	}
}

func TestCorpusFeasibilityRejectsFutureImpossibilityPromotion(t *testing.T) {
	decision, err := BuildVerificationLineageCorpusFeasibilityDecision("../..")
	if err != nil {
		t.Fatal(err)
	}
	decision.ClaimBoundary.FutureImpossibility = true
	decision.Digest, err = corpusFeasibilityDecisionDigest(decision)
	if err != nil {
		t.Fatal(err)
	}
	if err := decision.Validate("../.."); err == nil {
		t.Fatal("future impossibility was inferred from the current negative decision")
	}
}
