package mode

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestAbsoluteWireContractHasEvidenceAndNoConfidenceClaim(t *testing.T) {
	provider := &scriptedProvider{letterFor: func(string) (string, string) { return "", "" }}
	runner := &Runner{Provider: provider, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	score, err := RunAbsolute(context.Background(), runner, AbsoluteInput{
		Task:       "task",
		Trajectory: "trajectory",
		Criteria:   []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:        AbsoluteConfig{NReps: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Confidence != 0 {
		t.Fatalf("absolute confidence = %v, want absent/zero", score.Confidence)
	}
	encoded, err := json.Marshal(score)
	if err != nil {
		t.Fatal(err)
	}
	wire := string(encoded)
	for _, forbidden := range []string{`"confidence":`, `"score":`, `"per_criterion":`} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("legacy naked-score field %s survived: %s", forbidden, wire)
		}
	}
	for _, required := range []string{`"schema_version":"evalwitness.absolute-score.v2"`, `"conditional_score":`, `"criterion_evidence":`, `"evidence_strength":`} {
		if !strings.Contains(wire, required) {
			t.Fatalf("required field %s missing: %s", required, wire)
		}
	}
	if score.EvidenceStrength.Observations != 1 || score.EvidenceStrength.ExtractedObservations != 1 {
		t.Fatalf("evidence strength = %+v", score.EvidenceStrength)
	}
}

func TestPairUncertaintyComponentsRemainSeparate(t *testing.T) {
	observations := []pairObservation{
		observationFromEvidence(contractPairEvidence(0.8, 0.6, 0.01), false, 0),
		observationFromEvidence(contractPairEvidence(0.9, 0.5, 0.02), false, 1),
		observationFromEvidence(contractPairEvidence(0.55, 0.75, 0.03), true, 0),
		observationFromEvidence(contractPairEvidence(0.5, 0.8, 0.04), true, 1),
	}
	aggregate := aggregatePairObservations(observations, 0.05)
	uncertainty := aggregate.uncertainty
	if uncertainty.ConditionalTokenVariance <= 0 || uncertainty.RepeatedSampleVariance <= 0 || uncertainty.PresentationOrderVariance <= 0 || uncertainty.PolicyVariance <= 0 {
		t.Fatalf("uncertainty was collapsed or discarded: %+v", uncertainty)
	}
	wantTotal := uncertainty.ConditionalTokenVariance + uncertainty.RepeatedSampleVariance + uncertainty.PresentationOrderVariance + uncertainty.PolicyVariance
	if math.Abs(uncertainty.TotalVariance-wantTotal) > 1e-15 || aggregate.variance != uncertainty.TotalVariance {
		t.Fatalf("uncertainty total = %+v aggregate variance=%v", uncertainty, aggregate.variance)
	}
	if countPairRepeats(observations) != 2 || len(pairObservationRecords(observations)) != 4 {
		t.Fatalf("repeat/observation accounting failed")
	}
}

func TestOrderInstabilityProducesExplicitAbstention(t *testing.T) {
	observations := []pairObservation{
		observationFromEvidence(contractPairEvidence(0.9, 0.1, 0.01), false, 0),
		observationFromEvidence(contractPairEvidence(0.9, 0.1, 0.01), true, 0),
	}
	aggregate := aggregatePairObservations(observations, 0.05)
	decision := PairDecision{}
	setPairDecisionState(&decision, aggregate, PairwiseConfig{Epsilon: 0.02, ConfidenceThreshold: 0.6})
	if !aggregate.inconsistent || decision.State != verifier.DecisionAbstained || decision.AbstentionReason != verifier.AbstentionUnstableOrder || decision.Winner != -1 {
		t.Fatalf("aggregate=%+v decision=%+v", aggregate, decision)
	}
}

func TestSelectionCannotUpgradePairAbstentionOrTieToSelection(t *testing.T) {
	tests := []struct {
		name     string
		decision PairDecision
		want     verifier.DecisionState
	}{
		{name: "abstention", decision: PairDecision{State: verifier.DecisionAbstained, AbstentionReason: verifier.AbstentionEvidenceCeiling}, want: verifier.DecisionAbstained},
		{name: "tie", decision: PairDecision{State: verifier.DecisionTied}, want: verifier.DecisionTied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection := finalizeSelection(Selection{PairDecisions: []PairDecision{test.decision}}, 2, 1, []float64{0.5, 0.5}, []float64{0.5, 0.5}, []int{1, 1})
			if selection.State != test.want {
				t.Fatalf("selection state = %s, want %s", selection.State, test.want)
			}
			if selection.BestIndex != -1 || selection.Confidence != 0 {
				t.Fatalf("non-selected ranking leaked: %+v", selection)
			}
		})
	}
}

func TestDynamicTournamentStopsWhenAMatchDoesNotSelect(t *testing.T) {
	provider := &scriptedProvider{letterFor: func(string) (string, string) { return "M", "M" }}
	runner := &Runner{Provider: provider, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	selection, err := RunPairwise(context.Background(), runner, PairwiseInput{
		Task:         "task",
		Trajectories: []string{"first", "second", "third"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg: PairwiseConfig{
			NReps:               1,
			BiasMitigation:      "adaptive",
			SingleElim:          true,
			MaxWorkers:          1,
			MaxPairCalls:        2,
			ConfidenceThreshold: 0.6,
			CalibrationSigma:    0.05,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State == verifier.DecisionSelected || selection.BestIndex != -1 {
		t.Fatalf("tournament upgraded an unresolved match: %+v", selection)
	}
	if selection.PairsEvaluated != 1 {
		t.Fatalf("tournament continued after unresolved match: pairs=%d", selection.PairsEvaluated)
	}
}

func TestPairwiseWireContractNamesConditionalAndUncalibratedQuantities(t *testing.T) {
	provider := &scriptedProvider{letterFor: func(string) (string, string) { return "A", "T" }}
	runner := &Runner{Provider: provider, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	selection, err := RunPairwise(context.Background(), runner, PairwiseInput{
		Task:         "task",
		Trajectories: []string{"first", "second"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg: PairwiseConfig{
			NReps:               1,
			BiasMitigation:      "adaptive",
			MaxWorkers:          1,
			MaxPairCalls:        2,
			ConfidenceThreshold: 0.6,
			CalibrationSigma:    0.05,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	wire := string(encoded)
	if strings.Contains(wire, `"confidence":`) || strings.Contains(wire, `"scores":`) || strings.Contains(wire, `"score_mass":`) {
		t.Fatalf("legacy public fields survived: %s", wire)
	}
	for _, required := range []string{`"conditional_scores":`, `"decision_strength":`, `"calibrated":false`, `"uncertainty":`, `"observations":`, `"valid_score_mass":`} {
		if !strings.Contains(wire, required) {
			t.Fatalf("required pair evidence field %s missing: %s", required, wire)
		}
	}
}

func contractPairEvidence(scoreA, scoreB, variance float64) PairEvidenceScores {
	return PairEvidenceScores{
		"criterion": {
			A: contractScoreEvidence("<score_A>", scoreA, variance),
			B: contractScoreEvidence("<score_B>", scoreB, variance),
		},
	}
}

func contractScoreEvidence(tag string, score, variance float64) verifier.ScoreEvidence {
	return verifier.ScoreEvidence{
		SchemaVersion:             verifier.ScoreEvidenceSchemaVersion,
		PolicyVersion:             verifier.StrictPolicyVersion,
		Tag:                       tag,
		ExtractionMode:            verifier.ExtractionModeVerifier,
		AlignmentStatus:           verifier.AlignmentExact,
		RequestedTopK:             20,
		ReturnedTopK:              20,
		VisibleProbabilityMass:    1,
		ValidScoreMass:            1,
		UnobservedProbabilityMass: 0,
		ConditionalExpectedScore:  &score,
		ConditionalVariance:       &variance,
		Extracted:                 true,
		Support:                   []verifier.ScoreSupport{},
		Alternatives:              []verifier.VisibleAlternative{},
		Degradations:              []verifier.Degradation{},
	}
}
