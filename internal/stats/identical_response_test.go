package stats

import (
	"math"
	"testing"
)

func TestComputeIdenticalResponseDesignDeterministic(t *testing.T) {
	spec := IdenticalResponseDesignSpec{
		SourceTaskGroups:         40,
		DisagreementRates:        []float64{0.02, 0.14, 0.31},
		PrimaryDisagreementRate:  0.14,
		Alpha:                    0.05,
		FamilySize:               1,
		TargetPower:              0.80,
		DiscordantWinProbability: 0.75,
		InvalidRate:              0.02,
		MissingRate:              0.01,
		AbstentionRate:           0.02,
		RouteFailureRate:         0.01,
	}
	first, err := ComputeIdenticalResponseDesign(spec)
	if err != nil {
		t.Fatalf("compute identical-response design: %v", err)
	}
	second, err := ComputeIdenticalResponseDesign(spec)
	if err != nil {
		t.Fatalf("recompute identical-response design: %v", err)
	}
	if first.DesignDigest != second.DesignDigest {
		t.Fatalf("design digest is not deterministic: %q != %q", first.DesignDigest, second.DesignDigest)
	}
	if first.Counterfactual != "distribution_aware_vs_chosen_token" {
		t.Fatalf("counterfactual = %q", first.Counterfactual)
	}
	if len(first.Rows) != 3 {
		t.Fatalf("expected 3 disagreement rows, got %d", len(first.Rows))
	}
	if len(first.FailureSensitivity) != 5 {
		t.Fatalf("expected 5 failure sensitivity rows, got %d", len(first.FailureSensitivity))
	}
}

func TestComputeIdenticalResponseDesignEffectiveGroups(t *testing.T) {
	// 40 groups with 10% combined loss -> 36 effective groups.
	spec := IdenticalResponseDesignSpec{
		SourceTaskGroups:         40,
		DisagreementRates:        []float64{0.14},
		PrimaryDisagreementRate:  0.14,
		Alpha:                    0.05,
		FamilySize:               1,
		TargetPower:              0.80,
		DiscordantWinProbability: 0.75,
		InvalidRate:              0.05,
		MissingRate:              0.05,
	}
	report, err := ComputeIdenticalResponseDesign(spec)
	if err != nil {
		t.Fatalf("compute identical-response design: %v", err)
	}
	if report.EffectiveTaskGroups != 36 {
		t.Fatalf("effective groups = %d, want 36", report.EffectiveTaskGroups)
	}
	if math.Abs(report.CombinedLoss-0.10) > 1e-12 {
		t.Fatalf("combined loss = %.6f, want 0.10", report.CombinedLoss)
	}
	if report.AdjustedAlpha != 0.05 {
		t.Fatalf("adjusted alpha = %.6f, want 0.05", report.AdjustedAlpha)
	}
}

func TestComputeIdenticalResponseDesignRejectsInvalid(t *testing.T) {
	cases := []IdenticalResponseDesignSpec{
		{SourceTaskGroups: 0},
		{SourceTaskGroups: 40, PrimaryDisagreementRate: 0.14, Alpha: 0, FamilySize: 1, TargetPower: 0.8, DiscordantWinProbability: 0.75},
		{SourceTaskGroups: 40, PrimaryDisagreementRate: 0.14, Alpha: 0.05, FamilySize: 1, TargetPower: 0.8, DiscordantWinProbability: 0.5},
		{SourceTaskGroups: 40, PrimaryDisagreementRate: 0.14, Alpha: 0.05, FamilySize: 1, TargetPower: 0.8, DiscordantWinProbability: 0.75, InvalidRate: 0.6, MissingRate: 0.6},
	}
	for index, spec := range cases {
		if _, err := ComputeIdenticalResponseDesign(spec); err == nil {
			t.Fatalf("case %d: expected error for invalid spec", index)
		}
	}
}

func TestIdenticalResponseMDEHonestAvailability(t *testing.T) {
	// At 40 source task groups the exact McNemar design cannot reach 80% power
	// under complete separation for a 14% disagreement rate, so the MDE must be
	// absent (never fabricated). At 31% it becomes resolvable.
	spec := IdenticalResponseDesignSpec{
		SourceTaskGroups:         40,
		DisagreementRates:        []float64{0.14, 0.31},
		PrimaryDisagreementRate:  0.31,
		Alpha:                    0.05,
		FamilySize:               1,
		TargetPower:              0.80,
		DiscordantWinProbability: 0.75,
	}
	report, err := ComputeIdenticalResponseDesign(spec)
	if err != nil {
		t.Fatalf("compute identical-response design: %v", err)
	}
	low := report.Rows[0]
	if low.MDENominal != nil {
		t.Fatalf("14%% disagreement at 40 groups must not fabricate an MDE, got %v", *low.MDENominal)
	}
	if low.PowerAtSeparationNominal >= spec.TargetPower {
		t.Fatalf("14%% complete-separation power %.3f should be below target %.2f", low.PowerAtSeparationNominal, spec.TargetPower)
	}
	high := report.Rows[1]
	if high.MDENominal == nil {
		t.Fatal("expected a nominal MDE for the 31% disagreement rate")
	}
	if *high.MDENominal <= 0 || *high.MDENominal >= 1 {
		t.Fatalf("nominal MDE out of range: %v", *high.MDENominal)
	}
}
