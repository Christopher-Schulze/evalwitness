package sprt

import "testing"

func TestSPRT_ContinueBeforeMinReps(t *testing.T) {
	p := DefaultParams()
	d, _ := Decide([]float64{0.5}, p)
	if d != Continue {
		t.Errorf("expected Continue, got %v", d)
	}
}

func TestSPRT_AcceptH1OnStrongPositive(t *testing.T) {
	p := DefaultParams()
	p.MinReps = 2
	diffs := []float64{0.3, 0.4, 0.5, 0.4, 0.6}
	d, n := Decide(diffs, p)
	if d != AcceptH1 {
		t.Errorf("expected AcceptH1, got %v after %d reps", d, n)
	}
}

func TestSPRT_AcceptH0OnStrongNegative(t *testing.T) {
	p := DefaultParams()
	p.MinReps = 2
	diffs := []float64{-0.3, -0.4, -0.5, -0.4}
	d, _ := Decide(diffs, p)
	if d != AcceptH0 {
		t.Errorf("expected AcceptH0, got %v", d)
	}
}

func TestSPRT_ForceFinalizeAtMaxReps(t *testing.T) {
	p := DefaultParams()
	p.MaxReps = 3
	diffs := []float64{0.01, 0.02, 0.01}
	d, n := Decide(diffs, p)
	if d == Continue {
		t.Errorf("expected forced decision at MaxReps, got Continue (n=%d)", n)
	}
}

func TestAbsoluteVarianceDecision(t *testing.T) {
	p := DefaultParams()
	p.MinReps = 2
	if AbsoluteVarianceDecision([]float64{0.5}, p) {
		t.Errorf("should not stop before MinReps")
	}
	if !AbsoluteVarianceDecision([]float64{0.5, 0.5, 0.5}, p) {
		t.Errorf("low variance should trigger stop")
	}
	if AbsoluteVarianceDecision([]float64{0.1, 0.9, 0.2, 0.8}, p) {
		t.Errorf("high variance should not stop")
	}
}
