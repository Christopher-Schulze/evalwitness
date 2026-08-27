package claim

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

func TestCheckProfileFeedsIntoClaimcheck(t *testing.T) {
	metric := "0.12"
	p, err := profile.Build("test-profile", "evalwitness.protocol.v1", "routeA", []profile.Dimension{
		{ID: "calibration", Status: profile.StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "calibration.metrics.ece", Denominator: 100, SampleUnit: "task"},
	})
	if err != nil {
		t.Fatalf("build %v", err)
	}
	exprs, err := CheckProfile(p)
	if err != nil {
		t.Fatalf("check %v", err)
	}
	if len(exprs) != 1 || exprs[0] == "" {
		t.Fatalf("exprs %v", exprs)
	}
	// Tampered capsule expression must fail inside CheckProfile
	p2 := p
	p2.Dimensions[0].CapsuleExpr = ""
	if _, err := CheckProfile(p2); err == nil {
		t.Fatal("expected fail on empty capsule expr")
	}
	// Profile view projection must not be empty
	view, err := ProfileStepsView(buildTestAutopsyForProfile(t))
	if err != nil {
		t.Fatalf("view %v", err)
	}
	steps, err := profile.StepsFromAutopsyView(view)
	if err != nil {
		t.Fatalf("steps %v", err)
	}
	if len(steps) != 3 || steps[0].Denominator != 3 || steps[2].Denominator != 198 {
		t.Fatalf("steps %+v", steps)
	}
}

func buildTestAutopsyForProfile(t *testing.T) Autopsy {
	t.Helper()
	// Minimal autopsy that satisfies StepsFromAutopsyView invariants
	return Autopsy{
		SchemaVersion: AutopsySchemaVersion,
		MethodIntegrity: MethodIntegrityLane{
			Generations: []MethodGeneration{
				{Generation: "v1", State: "falsified", FrozenDenominators: []AutopsyCount{{Name: "corrected_rejections", Value: 3}}, NonClaims: []string{"no held-out validity for v1"}, Evidence: []AutopsyEvidenceRef{{ComponentID: "repair"}}},
				{Generation: "v2", State: "superseded", FrozenDenominators: []AutopsyCount{{Name: "v2_false_acceptances", Value: 5}}, NonClaims: []string{"no population inference from v2"}, Evidence: []AutopsyEvidenceRef{{ComponentID: "challenge"}}},
				{Generation: "v3", State: "admitted_development", FrozenDenominators: []AutopsyCount{{Name: "scarcity_attempted", Value: 198}}, NonClaims: []string{"zero test-role scarcity cases forbid held-out validity claims"}, Evidence: []AutopsyEvidenceRef{{ComponentID: "scarcity"}}},
			},
			Transitions: []MethodTransition{
				{From: "v1", To: "v2", Reason: "three frozen v1 acceptances are rejected by v2 under closed reasons", EvidenceComponentIDs: []string{"repair"}, GuardingTest: "TestConstructRepairCasesReproduceLegacyAcceptanceAndCorrectedRejection"},
				{From: "v2", To: "v3", Reason: "five frozen v2 false acceptances are rejected by v3 while six positive controls and three shared guards are preserved", EvidenceComponentIDs: []string{"challenge"}, GuardingTest: "TestConstructChallengeReproducesV2FalsificationAndV3Repair"},
			},
			Current:  "v3",
			Boundary: "v3 is admitted only as provider-free development evidence; the 280-case inferential core and 3-of-40 scarcity result remain separate and zero test-role scarcity cases forbid held-out validity claims",
		},
	}
}
