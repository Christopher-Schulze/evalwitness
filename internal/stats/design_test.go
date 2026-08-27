package stats

import (
	"math"
	"testing"
)

func TestDecidableBinary(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []int
		want     bool
	}{
		{name: "mixed", outcomes: []int{0, 1, 0}, want: true},
		{name: "all pass", outcomes: []int{1, 1}, want: false},
		{name: "all fail", outcomes: []int{0, 0}, want: false},
		{name: "one candidate", outcomes: []int{0}, want: false},
		{name: "non binary", outcomes: []int{0, 2}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DecidableBinary(test.outcomes); got != test.want {
				t.Fatalf("DecidableBinary(%v) = %t, want %t", test.outcomes, got, test.want)
			}
		})
	}
}

func TestExactMcNemarRejectionBoundary(t *testing.T) {
	for discordant := 0; discordant < 6; discordant++ {
		region, err := ExactMcNemarRejectionRegion(discordant, 0.05)
		if err != nil {
			t.Fatal(err)
		}
		if len(region.RejectedA) != 0 {
			t.Fatalf("n=%d rejected outcomes %v, want none", discordant, region.RejectedA)
		}
	}
	region, err := ExactMcNemarRejectionRegion(6, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	if len(region.RejectedA) != 2 || region.RejectedA[0] != 0 || region.RejectedA[1] != 6 {
		t.Fatalf("n=6 rejection region = %v, want [0 6]", region.RejectedA)
	}
}

func TestExactMcNemarPowerHandValues(t *testing.T) {
	tests := []struct {
		name      string
		n         int
		q         float64
		want      float64
		tolerance float64
	}{
		{name: "no rejection region", n: 5, q: 1, want: 0, tolerance: 1e-15},
		{name: "null actual size", n: 6, q: 0.5, want: 2.0 / 64.0, tolerance: 1e-15},
		{name: "alternative", n: 6, q: 0.75, want: math.Pow(0.75, 6) + math.Pow(0.25, 6), tolerance: 1e-15},
		{name: "complete separation", n: 6, q: 1, want: 1, tolerance: 1e-15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ExactMcNemarPower(test.n, test.q, 0.05)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-test.want) > test.tolerance {
				t.Fatalf("power = %.16f, want %.16f", got, test.want)
			}
		})
	}
}

func TestUnconditionalPowerAtCertainDisagreementMatchesConditional(t *testing.T) {
	conditional, err := ExactMcNemarPower(12, 0.8, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	unconditional, err := ExactMcNemarUnconditionalPower(12, 1, 0.8, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(conditional-unconditional) > 1e-14 {
		t.Fatalf("conditional %.16f != unconditional %.16f", conditional, unconditional)
	}
}

func TestMinimumDetectableEffectReportsImpossibleDesign(t *testing.T) {
	row, err := MinimumDetectablePairedEffect(89, 0.02, 0.05, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if row.MinimumPairedEffect != nil {
		t.Fatalf("MDE = %v, want impossible", *row.MinimumPairedEffect)
	}
	if row.PowerAtCompleteSeparation >= 0.8 {
		t.Fatalf("complete-separation power = %.4f, want < .8", row.PowerAtCompleteSeparation)
	}
}

func TestPairedBinaryScoreIntervalAndTypedInference(t *testing.T) {
	interval, err := PairedBinaryScoreInterval(20, 20, 7, 42, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(interval.Estimate-13.0/89.0) > 1e-15 {
		t.Fatalf("estimate = %.16f, want %.16f", interval.Estimate, 13.0/89.0)
	}
	if interval.Lower >= interval.Estimate || interval.Upper <= interval.Estimate {
		t.Fatalf("interval %#v does not contain estimate", interval)
	}
	equivalence, err := EvaluatePairedQuestion(QuestionEquivalence, 0.05, 0.05, interval, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if equivalence.Established {
		t.Fatal("non-significant superiority result became equivalence")
	}
}

func TestPairedInferenceConfidenceMatchesQuestion(t *testing.T) {
	tests := []struct {
		question InferenceQuestion
		want     float64
	}{
		{question: QuestionSuperiority, want: 0.95},
		{question: QuestionNonInferiority, want: 0.90},
		{question: QuestionEquivalence, want: 0.90},
	}
	for _, test := range tests {
		got, err := PairedInferenceConfidence(test.question, 0.05)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(got-test.want) > 1e-15 {
			t.Fatalf("%s confidence = %.16f, want %.16f", test.question, got, test.want)
		}
	}
}

func TestPairedInferenceRejectsUndercoveredInterval(t *testing.T) {
	interval := PairedInterval{Confidence: 0.95, Lower: -0.01, Upper: 0.01}
	if _, err := EvaluatePairedQuestion(QuestionEquivalence, 0.05, 0.01, interval, 0.5); err == nil {
		t.Fatal("equivalence accepted an interval below the family-adjusted confidence requirement")
	}
}

func TestPoissonBinomialUpperTailHandValue(t *testing.T) {
	got, err := PoissonBinomialUpperTail([]float64{0.5, 0.25}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-0.625) > 1e-15 {
		t.Fatalf("tail = %.16f, want .625", got)
	}
}

func TestExactBinomialUpperDesignUsesStrictFamilyAdjustedTail(t *testing.T) {
	design, err := ExactBinomialUpperDesign(40, 0.5, 0.8, 0.05/8)
	if err != nil {
		t.Fatal(err)
	}
	if design.CriticalSuccesses <= 20 || design.ActualNullTail >= 0.05/8 || design.Power < 0.8 {
		t.Fatalf("exact controlled-relation design is underpowered or invalid: %+v", design)
	}
	previousTail := binomialUpperTail(40, design.CriticalSuccesses-1, 0.5)
	if previousTail < 0.05/8 {
		t.Fatalf("critical boundary %d is not minimal; previous tail %.12f", design.CriticalSuccesses, previousTail)
	}
	if _, err := ExactBinomialUpperDesign(40, 0.8, 0.5, 0.05); err == nil {
		t.Fatal("exact binomial design accepted an alternative below the null")
	}
}
