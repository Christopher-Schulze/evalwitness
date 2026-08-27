package main

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestEvalPlanBoundedAllPairsCalls(t *testing.T) {
	// The tournament is the production default, so the exhaustive path has to be
	// requested explicitly here; that is exactly what this test covers.
	cfg := config.Default()
	cfg.SingleElim = false
	criteria := verifier.PaperTerminalCriteria()
	tasks := makeEvalPlanTasks(17, 5)
	plan, err := buildEvalPlan(tasks, cfg, cost.New(0.14, 0.0028, 0.28, false), criteria, 1, "adaptive")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PairMatches != 170 {
		t.Fatalf("pair matches = %d, want 170", plan.PairMatches)
	}
	// Best and expected are one bundled call per pair: production no longer
	// spends extra calls from legacy PairConfidence. Worst remains the hard
	// per-pair ceiling for an explicit legacy opt-in.
	wantExpected := 170
	wantWorst := 170 * cfg.MaxPairCalls
	if plan.Calls.Best != 170 || plan.Calls.Expected != wantExpected || plan.Calls.Worst != wantWorst {
		t.Fatalf("calls = %+v, want 170/%d/%d with confidence escalation disabled",
			plan.Calls, wantExpected, wantWorst)
	}
	if plan.ConfidenceEscalation != mode.ConfidenceEscalationDisabled {
		t.Fatalf("confidence escalation = %q", plan.ConfidenceEscalation)
	}
	if plan.EstimatedInputTokens.Best <= 0 || plan.EstimatedInputTokens.Worst != cfg.MaxPairCalls*plan.EstimatedInputTokens.Best {
		t.Fatalf("input estimate = %+v at ceiling %d", plan.EstimatedInputTokens, cfg.MaxPairCalls)
	}
}

func TestEvalPlanPaperParityAndDynamicTournamentCalls(t *testing.T) {
	criteria := verifier.PaperTerminalCriteria()
	paper := config.Default()
	paper.MultiCriterionBundle = false
	paper.SingleElim = false
	paperPlan, err := buildEvalPlan(makeEvalPlanTasks(17, 5), paper, nil, criteria, 4, "single")
	if err != nil {
		t.Fatal(err)
	}
	if paperPlan.Calls.Best != 2040 || paperPlan.Calls.Expected != 2040 || paperPlan.Calls.Worst != 2040 {
		t.Fatalf("paper calls = %+v, want fixed 2040", paperPlan.Calls)
	}

	tournament := config.Default()
	tournament.SingleElim = true
	tournamentPlan, err := buildEvalPlan(makeEvalPlanTasks(1, 5), tournament, nil, criteria, 1, "adaptive")
	if err != nil {
		t.Fatal(err)
	}
	if tournamentPlan.PairMatches != 4 || tournamentPlan.Calls.Best != 4 || tournamentPlan.Calls.Worst != 4*tournament.MaxPairCalls {
		t.Fatalf("tournament plan = pairs %d calls %+v at ceiling %d",
			tournamentPlan.PairMatches, tournamentPlan.Calls, tournament.MaxPairCalls)
	}
}

func TestEvalPlanAbsoluteSelectionCountsCandidateScoresInsteadOfPairs(t *testing.T) {
	cfg := config.Default()
	cfg.Selection = "absolute"
	cfg.MultiCriterionBundle = false
	criteria := verifier.PaperTerminalCriteria()
	plan, err := buildEvalPlan(makeEvalPlanTasks(2, 4), cfg, nil, criteria, 3, "adaptive")
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := 2 * 4 * len(criteria) * 3
	if plan.PairMatches != 0 || plan.CandidateScores != 8 {
		t.Fatalf("absolute units = pairs:%d candidates:%d", plan.PairMatches, plan.CandidateScores)
	}
	if plan.Calls != (evalEstimateInt{Best: wantCalls, Expected: wantCalls, Worst: wantCalls}) {
		t.Fatalf("absolute calls = %+v, want fixed %d", plan.Calls, wantCalls)
	}
	if plan.EstimatedInputTokens.Best <= 0 || plan.EstimatedInputTokens.Best != plan.EstimatedInputTokens.Expected ||
		plan.EstimatedInputTokens.Best != plan.EstimatedInputTokens.Worst {
		t.Fatalf("absolute input estimates = %+v", plan.EstimatedInputTokens)
	}
}

func TestEvalPlanJointAbsoluteUsesOneCallPerTask(t *testing.T) {
	cfg := config.Default()
	cfg.Selection = "joint_absolute"
	criteria := verifier.PaperTerminalCriteria()
	plan, err := buildEvalPlan(makeEvalPlanTasks(60, 5), cfg, nil, criteria, 1, "adaptive")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Strategy != "joint-absolute" || plan.ScoredTasks != 60 || plan.CandidateScores != 300 {
		t.Fatalf("joint plan shape = %+v", plan)
	}
	if plan.PairMatches != 0 || plan.Calls != (evalEstimateInt{Best: 60, Expected: 60, Worst: 60}) {
		t.Fatalf("joint calls = pairs:%d calls:%+v", plan.PairMatches, plan.Calls)
	}
	if plan.EstimatedInputTokens.Best <= 0 || plan.EstimatedInputTokens.Best != plan.EstimatedInputTokens.Worst {
		t.Fatalf("joint input estimate = %+v", plan.EstimatedInputTokens)
	}
}

func TestEvalPlanRejectsAdaptiveOrderLargerThanPairBudget(t *testing.T) {
	cfg := config.Default()
	cfg.MultiCriterionBundle = false
	cfg.MaxPairCalls = 2
	_, err := buildEvalPlan(makeEvalPlanTasks(1, 2), cfg, nil, verifier.PaperTerminalCriteria(), 1, "adaptive")
	if err == nil {
		t.Fatal("expected adaptive pair-budget error")
	}
}

func TestEvalEscalationSummary(t *testing.T) {
	selection := mode.Selection{PairDecisions: []mode.PairDecision{
		{Calls: 1, Confidence: 0.9, EvidenceStrength: verifier.EvidenceStrength{MinimumValidMass: 0.8}},
		{Calls: 2, Confidence: 0.7, EvidenceStrength: verifier.EvidenceStrength{MinimumValidMass: 0.6}, Inconsistent: true},
		{Calls: 4, Confidence: 0.5, EvidenceStrength: verifier.EvidenceStrength{MinimumValidMass: 0.4}},
	}}
	summary := addEvalEscalation(nil, selection)
	if summary.PairsEvaluated != 3 || summary.EscalatedPairs != 2 || summary.TotalPairCalls != 7 {
		t.Fatalf("counts = %+v", summary)
	}
	if summary.OneCallPairs != 1 || summary.TwoCallPairs != 1 || summary.FourCallPairs != 1 || summary.InconsistentPairs != 1 {
		t.Fatalf("histogram = %+v", summary)
	}
	if math.Abs(summary.MeanDecisionStrength-0.7) > 1e-12 || math.Abs(summary.MeanMinimumValidMass-0.6) > 1e-12 {
		t.Fatalf("means = decision strength %.6f minimum valid mass %.6f", summary.MeanDecisionStrength, summary.MeanMinimumValidMass)
	}
}

func makeEvalPlanTasks(taskCount, trajectories int) []evalPlanTask {
	tasks := make([]evalPlanTask, taskCount)
	for taskIndex := range tasks {
		traces := make([]string, trajectories)
		for trajectoryIndex := range traces {
			traces[trajectoryIndex] = fmt.Sprintf("task %d trajectory %d final tests passed", taskIndex, trajectoryIndex)
		}
		tasks[taskIndex] = evalPlanTask{Task: fmt.Sprintf("task %d", taskIndex), PreparedTrajectories: traces}
	}
	return tasks
}

func TestDerivedLimitsSeparateCallsAndAttempts(t *testing.T) {
	plan := evalPlan{
		Calls:                evalEstimateInt{Best: 100, Expected: 100, Worst: 100},
		EstimatedInputTokens: evalEstimateInt{Best: 1000, Expected: 1000, Worst: 1000},
	}
	var zeroCalls, zeroAttempts, zeroTokens, zeroOutput, zeroConcurrent int
	var zeroCost float64
	var zeroDuration time.Duration
	derived := resolveEvalLimits(evalLimitFlags{
		maxCalls: &zeroCalls, maxAttempts: &zeroAttempts, maxInputTokens: &zeroTokens,
		maxOutputTokens: &zeroOutput, maxConcurrent: &zeroConcurrent,
		maxCostUSD: &zeroCost, maxDuration: &zeroDuration,
	}, plan, config.Default())
	if derived.MaxCalls != 100 {
		t.Fatalf("derived logical call limit = %d, want 100", derived.MaxCalls)
	}
	if derived.MaxAttempts != 600 || derived.MaxEstimatedInputTokens != 6000 {
		t.Fatalf("derived attempts/input = %d/%d, want 600/6000", derived.MaxAttempts, derived.MaxEstimatedInputTokens)
	}

	// An explicit flag is a deliberate ceiling and must be used verbatim.
	explicitCalls := 100
	explicit := resolveEvalLimits(evalLimitFlags{
		maxCalls: &explicitCalls, maxAttempts: &zeroAttempts, maxInputTokens: &zeroTokens,
		maxOutputTokens: &zeroOutput, maxConcurrent: &zeroConcurrent,
		maxCostUSD: &zeroCost, maxDuration: &zeroDuration,
	}, plan, config.Default())
	if explicit.MaxCalls != 100 {
		t.Fatalf("explicit call limit = %d, want it honored verbatim", explicit.MaxCalls)
	}
}
