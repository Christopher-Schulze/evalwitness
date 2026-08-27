package main

import (
	"encoding/json"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
)

func TestEvalReliabilityRowsPreservePairEvidence(t *testing.T) {
	selection := &mode.Selection{
		Usage: mode.UsageSummary{ExtractionMode: "verifier"},
		PairDecisions: []mode.PairDecision{
			{
				Pair:           [2]int{0, 1},
				OrderPolicy:    "adaptive",
				FirstOrder:     "reversed",
				Calls:          2,
				MeanDifference: -0.35,
				ScoreMass:      0.81,
				WinProbability: 0.1,
				Inconsistent:   true,
			},
			{Pair: [2]int{1, 2}, WinProbability: 0.9},
		},
	}
	rows, ranks := evalReliabilityRows("task-1", []int{1, 0, 0}, selection, 4)
	if len(rows) != 1 || len(ranks) != 0 {
		t.Fatalf("rows/ranks = %d/%d, want 1/0", len(rows), len(ranks))
	}
	row := rows[0]
	if row.ID != "task-1:pair:0:1" || row.TaskID != "task-1" || !row.Won {
		t.Fatalf("identity/outcome = %#v", row)
	}
	if row.MeanDifference != -0.35 || row.ScoreMass != 0.81 || row.PairCallLimit != 4 {
		t.Fatalf("score evidence = %#v", row)
	}
	if row.OrderPolicy != "adaptive" || row.FirstOrder != "reversed" || row.ExtractionMode != "verifier" {
		t.Fatalf("execution identity = %#v", row)
	}
}

func TestEvalReliabilityRowsKeepAbsoluteScoresSeparate(t *testing.T) {
	selection := &mode.Selection{
		Scores: []float64{0.2, 0.8},
		Usage:  mode.UsageSummary{ExtractionMode: "judge"},
	}
	rows, ranks := evalReliabilityRows("task-2", []int{0, 1}, selection, 4)
	if len(rows) != 0 || len(ranks) != 2 {
		t.Fatalf("rows/ranks = %d/%d, want 0/2", len(rows), len(ranks))
	}
	if ranks[1].Predicted != 0.8 || ranks[1].Actual != 1 || ranks[1].ExtractionMode != "judge" {
		t.Fatalf("absolute row = %#v", ranks[1])
	}
}

func TestEvalReliabilityRowsExcludeUndecidableAbsoluteTask(t *testing.T) {
	selection := &mode.Selection{Scores: []float64{0.2, 0.8}}
	rows, ranks := evalReliabilityRows("task-equal", []int{1, 1}, selection, 2)
	if len(rows) != 0 || len(ranks) != 0 {
		t.Fatalf("equal-reward task produced rows/ranks = %d/%d", len(rows), len(ranks))
	}
}

func TestAbsoluteReliabilityJSONOmitsPairwiseMetrics(t *testing.T) {
	report := reliability.AnalyzeWithAbsolute(nil, []reliability.RankObservation{
		{TaskID: "task-1", Predicted: 0.2, Actual: 0},
		{TaskID: "task-1", Predicted: 0.8, Actual: 1},
	})
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"metrics", "error_decomposition", "count", "ece", "auc", "monotone"} {
		if _, found := object[forbidden]; found {
			t.Fatalf("absolute-only JSON contains pairwise field %q: %s", forbidden, encoded)
		}
	}
	if _, found := object["absolute"]; !found {
		t.Fatalf("absolute-only JSON lacks rank report: %s", encoded)
	}
}

func TestEvalReliabilityRowsRejectMalformedPairIndexes(t *testing.T) {
	selection := &mode.Selection{PairDecisions: []mode.PairDecision{{Pair: [2]int{-1, 2}}}}
	rows, ranks := evalReliabilityRows("task-3", []int{0, 1}, selection, 2)
	if len(rows) != 0 || len(ranks) != 0 {
		t.Fatalf("malformed indexes produced rows/ranks = %d/%d", len(rows), len(ranks))
	}
}
