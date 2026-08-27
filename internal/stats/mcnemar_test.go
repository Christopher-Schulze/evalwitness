package stats

import (
	"math"
	"testing"
)

func TestMcNemarExactMatchesHandComputedValues(t *testing.T) {
	// Every expectation is 2 * (sum of binomial coefficients up to min(b,c)) /
	// 2^n, computed by hand so a rewrite of the summation cannot quietly agree
	// with itself.
	cases := []struct {
		name string
		b, c int
		want float64
	}{
		{"no disagreement carries no information", 0, 0, 1},
		{"a single disagreement is one coin flip", 1, 0, 1},
		{"an even split is maximally uninformative", 7, 7, 1},
		// 2*(1)/2^5 = 0.0625, the largest n where a clean sweep still misses 0.05
		{"five to nothing falls just short", 5, 0, 0.0625},
		// 2*(1)/2^6 = 0.03125, the smallest sweep that reaches significance
		{"six to nothing is the smallest significant sweep", 6, 0, 0.03125},
		// 2*(1+14)/2^14 = 0.0018310546875
		{"the published bounded-versus-reference shape", 13, 1, 0.0018310546875},
		// 2*(1+2)/2^2 = 1.5 -> clamped
		{"a probability can never exceed one", 1, 1, 1},
		// 2 * sum(C(27,i), i=0..7) / 2^27 = 2*1285624/134217728
		{"twenty to seven, the SWE-bench detectable split", 20, 7, 0.0191572904586792},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := McNemarExact(tc.b, tc.c)
			if math.Abs(got-tc.want) > 1e-12 {
				t.Fatalf("McNemarExact(%d, %d) = %v, want %v", tc.b, tc.c, got, tc.want)
			}
		})
	}
}

func TestMcNemarExactIsSymmetric(t *testing.T) {
	for b := 0; b <= 12; b++ {
		for c := 0; c <= 12; c++ {
			if McNemarExact(b, c) != McNemarExact(c, b) {
				t.Fatalf("asymmetric at (%d, %d)", b, c)
			}
		}
	}
}

func TestMcNemarExactSurvivesCountsThatOverflowBinomials(t *testing.T) {
	// C(2000, 1000) is far beyond float64. A naive summation returns +Inf here,
	// whose ratio reads as a p-value of 1 and would silently declare a lopsided
	// result to be noise.
	p := McNemarExact(1200, 800)
	if math.IsNaN(p) || math.IsInf(p, 0) {
		t.Fatalf("p = %v, want a finite probability", p)
	}
	if p >= 0.05 {
		t.Fatalf("p = %v for a 1200:800 split, want significance", p)
	}
}

func TestSmallestSignificantSplitFindsTheDetectionBoundary(t *testing.T) {
	cases := []struct {
		n          int
		wantSplit  int
		detectable bool
	}{
		{0, 0, false},
		{2, 0, false},
		// Below six discordant pairs no outcome at all reaches 0.05, which is
		// the Terminal-Bench verifier-versus-judge comparison exactly.
		{5, 0, false},
		{6, 6, true},
		{14, 12, true},
		{27, 20, true},
		{35, 24, true},
	}
	for _, tc := range cases {
		split, ok := SmallestSignificantSplit(tc.n, 0.05)
		if ok != tc.detectable || split != tc.wantSplit {
			t.Fatalf("SmallestSignificantSplit(%d) = (%d, %t), want (%d, %t)",
				tc.n, split, ok, tc.wantSplit, tc.detectable)
		}
		if ok {
			if McNemarExact(split, tc.n-split) >= 0.05 {
				t.Fatalf("reported split %d:%d is not significant", split, tc.n-split)
			}
			if split > (tc.n+1)/2 && McNemarExact(split-1, tc.n-split+1) < 0.05 {
				t.Fatalf("split %d is not the smallest significant one for n=%d", split, tc.n)
			}
		}
	}
}
