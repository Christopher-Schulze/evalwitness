package main

import (
	"flag"
	"math"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/baseline"
)

func TestBuildEvalStatisticalPlanCountsOnlyDecidableTasks(t *testing.T) {
	flags := testEvalDesignFlags(t)
	plan, err := buildEvalStatisticalPlan(4, [][]int{{0, 1}, {1, 1}, {0, 0}, {1, 0, 1}}, flags)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalTasks != 4 || plan.DecidableTasks != 2 || plan.DecidableShare != 0.5 {
		t.Fatalf("task counts = total %d decidable %d share %.3f", plan.TotalTasks, plan.DecidableTasks, plan.DecidableShare)
	}
	if len(plan.Rows) != 4 {
		t.Fatalf("sensitivity rows = %d, want 4", len(plan.Rows))
	}
	if plan.Rows[0].MDEAdjusted != nil {
		t.Fatal("zero disagreement produced a detectable effect")
	}
}

func TestBuildEvalStatisticalPlanReportsNominalAndAdjustedRequirements(t *testing.T) {
	flags := testEvalDesignFlags(t)
	*flags.familySize = 4
	plan, err := buildEvalStatisticalPlan(500, repeatRewardRows(500, []int{0, 1}), flags)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(plan.AdjustedAlpha-0.0125) > 1e-15 {
		t.Fatalf("adjusted alpha = %.16f, want .0125", plan.AdjustedAlpha)
	}
	for _, row := range plan.Rows {
		if row.MDENominal != nil && row.MDEAdjusted != nil && *row.MDEAdjusted < *row.MDENominal {
			t.Fatalf("adjusted MDE %.6f is smaller than nominal %.6f", *row.MDEAdjusted, *row.MDENominal)
		}
	}
}

func TestBuildObservedComparisonsKeepsNullSuperiorityTyped(t *testing.T) {
	flags := testEvalDesignFlags(t)
	plan, err := buildEvalStatisticalPlan(89, repeatRewardRows(89, []int{0, 1}), flags)
	if err != nil {
		t.Fatal(err)
	}
	comparisons, err := buildObservedComparisons([]baseline.Result{{
		Name: "judge", PairedTasks: 89, BothCorrect: 30, BothWrong: 57,
		SubjectOnly: 1, BaselineOnly: 1, Discordant: 2, McNemarP: 1,
	}}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(comparisons) != 1 || comparisons[0].Inference.Established {
		t.Fatalf("comparison = %#v, want superiority not established", comparisons)
	}
	if comparisons[0].Inference.Conclusion != "superiority_not_established" {
		t.Fatalf("conclusion = %q, want typed superiority failure", comparisons[0].Inference.Conclusion)
	}
	if comparisons[0].SmallestSignificantWin != nil {
		t.Fatalf("n=2 has significant boundary %d", *comparisons[0].SmallestSignificantWin)
	}
}

func testEvalDesignFlags(t *testing.T) evalDesignFlags {
	t.Helper()
	fs := flag.NewFlagSet("design-test", flag.ContinueOnError)
	flags := addEvalDesignFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	return flags
}

func repeatRewardRows(count int, rewards []int) [][]int {
	rows := make([][]int, count)
	for index := range rows {
		rows[index] = append([]int(nil), rewards...)
	}
	return rows
}
