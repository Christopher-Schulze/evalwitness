package stats

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestFitClusteredFactorialRecoversMainInteractionAndNullEffects(t *testing.T) {
	terms, observations, want := factorialRecoveryFixture()
	fit, err := FitClusteredFactorial(terms, observations, 0.05, 20)
	if err != nil {
		t.Fatal(err)
	}
	assertFactorialEstimates(t, fit, want)
	for repetition := 0; repetition < 32; repetition++ {
		reordered := slices.Clone(observations)
		if repetition%2 == 0 {
			slices.Reverse(reordered)
		}
		repeated, err := FitClusteredFactorial(terms, reordered, 0.05, 20)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(fit, repeated) {
			t.Fatalf("factorial fit is not deterministic at repetition %d:\nfirst=%+v\nrepeated=%+v", repetition, fit, repeated)
		}
	}
}

func factorialRecoveryFixture() ([]FactorialTerm, []FactorialObservation, map[string]float64) {
	terms := []FactorialTerm{
		{ID: "a", Factors: []string{"a"}},
		{ID: "a_x_b", Factors: []string{"a", "b"}},
		{ID: "b", Factors: []string{"b"}},
		{ID: "null", Factors: []string{"null"}},
	}
	observations := []FactorialObservation{}
	for cluster := 0; cluster < 6; cluster++ {
		clusterEffect := float64(cluster-3) * 0.01
		for cell := 0; cell < 8; cell++ {
			a := factorialTestLevel(cell, 0)
			b := factorialTestLevel(cell, 1)
			null := factorialTestLevel(cell, 2)
			outcome := 0.5 + clusterEffect + 0.2*a/2 - 0.1*b/2 + 0.08*a*b/2
			observations = append(observations, FactorialObservation{
				ObservationID: factorialTestID(cluster, cell), ClusterID: factorialTestID(cluster, 0),
				Levels:  []FactorialLevel{{FactorID: "a", Level: a}, {FactorID: "b", Level: b}, {FactorID: "null", Level: null}},
				Outcome: outcome,
			})
		}
	}
	want := map[string]float64{"a": 0.2, "a_x_b": 0.08, "b": -0.1, "null": 0}
	return terms, observations, want
}

func assertFactorialEstimates(t *testing.T, fit ClusteredFactorialFit, want map[string]float64) {
	t.Helper()
	for _, estimate := range fit.Estimates {
		if difference := estimate.Estimate - want[estimate.TermID]; difference < -1e-12 || difference > 1e-12 {
			t.Fatalf("term %q estimate = %.17g, want %.17g", estimate.TermID, estimate.Estimate, want[estimate.TermID])
		}
	}
}

func TestFitClusteredFactorialRejectsInvalidOrAliasedDesign(t *testing.T) {
	valid := []FactorialObservation{
		{ObservationID: "o1", ClusterID: "c1", Levels: []FactorialLevel{{FactorID: "a", Level: -1}}, Outcome: 0},
		{ObservationID: "o2", ClusterID: "c1", Levels: []FactorialLevel{{FactorID: "a", Level: 1}}, Outcome: 1},
		{ObservationID: "o3", ClusterID: "c2", Levels: []FactorialLevel{{FactorID: "a", Level: -1}}, Outcome: 0},
		{ObservationID: "o4", ClusterID: "c2", Levels: []FactorialLevel{{FactorID: "a", Level: 1}}, Outcome: 1},
	}
	tests := []struct {
		name         string
		terms        []FactorialTerm
		observations []FactorialObservation
		contains     string
	}{
		{name: "invalid level", terms: []FactorialTerm{{ID: "a", Factors: []string{"a"}}}, observations: func() []FactorialObservation {
			copy := slices.Clone(valid)
			copy[0].Levels = []FactorialLevel{{FactorID: "a", Level: 0}}
			return copy
		}(), contains: "coded as -1 or +1"},
		{name: "aliased terms", terms: []FactorialTerm{{ID: "a", Factors: []string{"a"}}, {ID: "duplicate", Factors: []string{"a"}}}, observations: valid, contains: "aliased terms"},
		{name: "family too small", terms: []FactorialTerm{{ID: "a", Factors: []string{"a"}}}, observations: valid, contains: "family covering every fitted term"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			familySize := len(test.terms)
			if test.name == "family too small" {
				familySize = 0
			}
			_, err := FitClusteredFactorial(test.terms, test.observations, 0.05, familySize)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("fit error = %v, want substring %q", err, test.contains)
			}
			if test.name == "aliased terms" && !errors.Is(err, ErrFactorialNotEstimable) {
				t.Fatalf("aliased fit error = %v, want ErrFactorialNotEstimable", err)
			}
		})
	}
}

func factorialTestLevel(cell, bit int) float64 {
	if cell&(1<<bit) == 0 {
		return -1
	}
	return 1
}

func factorialTestID(left, right int) string {
	return string(rune('a'+left)) + string(rune('a'+right))
}
