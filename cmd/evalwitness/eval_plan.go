package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const evalArtifactSchemaVersion = "evalwitness.evaluation.v2"

type evalEstimateInt struct {
	Best     int `json:"best"`
	Expected int `json:"expected"`
	Worst    int `json:"worst"`
}

type evalEstimateFloat struct {
	Best     float64 `json:"best"`
	Expected float64 `json:"expected"`
	Worst    float64 `json:"worst"`
}

type evalPlan struct {
	Strategy             string                  `json:"strategy"`
	SwingTasks           int                     `json:"swing_tasks"`
	ScoredTasks          int                     `json:"scored_tasks"`
	PairMatches          int                     `json:"pair_matches"`
	CandidateScores      int                     `json:"candidate_scores,omitempty"`
	EvidenceTokens       int                     `json:"evidence_tokens_per_trajectory"`
	BundledCriteria      bool                    `json:"bundled_criteria"`
	Calls                evalEstimateInt         `json:"calls"`
	EstimatedInputTokens evalEstimateInt         `json:"estimated_input_tokens"`
	EstimatedCostUSD     evalEstimateFloat       `json:"estimated_cost_usd"`
	EstimatedDurationSec evalEstimateInt         `json:"estimated_duration_seconds"`
	Limits               mode.BudgetLimits       `json:"limits"`
	HardDurationSeconds  int                     `json:"hard_duration_seconds"`
	Authorization        *mode.AuthorizationPlan `json:"authorization,omitempty"`
	CallMethod           string                  `json:"call_method"`
	TokenMethod          string                  `json:"token_method"`
	DurationMethod       string                  `json:"duration_method"`
	StatisticalDesign    *evalStatisticalPlan    `json:"statistical_design,omitempty"`
	ConfidenceEscalation string                  `json:"confidence_escalation"`
}

type evalPlanTask struct {
	Task                 string
	PreparedTrajectories []string
}

func evalPreparedTexts(prepared []preprocess.Result) []string {
	texts := make([]string, len(prepared))
	for index, trajectory := range prepared {
		texts[index] = trajectory.Text
	}
	return texts
}

type evalEscalationSummary struct {
	PairsEvaluated       int     `json:"pairs_evaluated"`
	EscalatedPairs       int     `json:"escalated_pairs"`
	TotalPairCalls       int     `json:"total_pair_calls"`
	OneCallPairs         int     `json:"one_call_pairs"`
	TwoCallPairs         int     `json:"two_call_pairs"`
	ThreeCallPairs       int     `json:"three_call_pairs"`
	FourCallPairs        int     `json:"four_call_pairs"`
	OtherCallPairs       int     `json:"other_call_pairs,omitempty"`
	InconsistentPairs    int     `json:"inconsistent_pairs"`
	MeanDecisionStrength float64 `json:"mean_decision_strength"`
	MeanMinimumValidMass float64 `json:"mean_minimum_valid_score_mass"`

	decisionStrengthSum float64
	minimumValidMassSum float64
}

func addEvalEscalation(summary *evalEscalationSummary, selection mode.Selection) *evalEscalationSummary {
	if len(selection.PairDecisions) == 0 {
		return summary
	}
	if summary == nil {
		summary = &evalEscalationSummary{}
	}
	for _, decision := range selection.PairDecisions {
		summary.PairsEvaluated++
		summary.TotalPairCalls += decision.Calls
		summary.decisionStrengthSum += decision.Confidence
		summary.minimumValidMassSum += decision.EvidenceStrength.MinimumValidMass
		if decision.Calls > 1 {
			summary.EscalatedPairs++
		}
		if decision.Inconsistent {
			summary.InconsistentPairs++
		}
		switch decision.Calls {
		case 1:
			summary.OneCallPairs++
		case 2:
			summary.TwoCallPairs++
		case 3:
			summary.ThreeCallPairs++
		case 4:
			summary.FourCallPairs++
		default:
			summary.OtherCallPairs++
		}
	}
	count := float64(summary.PairsEvaluated)
	summary.MeanDecisionStrength = summary.decisionStrengthSum / count
	summary.MeanMinimumValidMass = summary.minimumValidMassSum / count
	return summary
}

type evalLimitFlags struct {
	maxCalls            *int
	maxAttempts         *int
	maxInputTokens      *int
	maxOutputTokens     *int
	maxConcurrent       *int
	maxCostUSD          *float64
	maxDuration         *time.Duration
	authorize           *string
	studyRecord         *string
	studyManifestDigest *string
}

func addEvalLimitFlags(fs *flag.FlagSet) evalLimitFlags {
	return evalLimitFlags{
		maxCalls:            fs.Int("max-calls", 0, "hard provider-call limit (0 = preflight worst case)"),
		maxAttempts:         fs.Int("max-attempts", 0, "hard HTTP-attempt limit (0 = calls times retry ceiling)"),
		maxInputTokens:      fs.Int("max-input-tokens", 0, "hard estimated-input-token limit (0 = preflight worst case)"),
		maxOutputTokens:     fs.Int("max-output-tokens", 0, "hard reserved-output-token limit (0 = attempts times request ceiling)"),
		maxConcurrent:       fs.Int("max-concurrent", 0, "hard concurrent-attempt limit (0 = configured workers)"),
		maxCostUSD:          fs.Float64("max-cost-usd", 0, "hard estimated cost limit (0 = preflight worst case or disabled when unknown/free)"),
		maxDuration:         fs.Duration("max-duration", 0, "hard wall-clock limit (0 = preflight worst-case timeout bound)"),
		authorize:           fs.String("authorize", "", "execute only when this digest matches the printed plan"),
		studyRecord:         fs.String("study-record", "", "authorized TASK 049 study record (@file)"),
		studyManifestDigest: fs.String("study-manifest-digest", "", "optional expected manifest SHA-256; a digest alone never authorizes"),
	}
}

func buildEvalPlan(tasks []evalPlanTask, cfg config.Config, calculator *cost.Calculator, criteria []verifier.Criterion, reps int, biasMitigation string) (evalPlan, error) {
	plan := evalPlan{
		Strategy:             "all-pairs",
		SwingTasks:           len(tasks),
		ScoredTasks:          len(tasks),
		EvidenceTokens:       cfg.EvidenceTokens,
		BundledCriteria:      cfg.MultiCriterionBundle,
		CallMethod:           "logical model requests before disk-cache hits; HTTP attempts, including retries, are budgeted separately",
		TokenMethod:          "preprocessed prompt bytes / 4; output cost reserves max_tokens per call",
		DurationMethod:       "best assumes one second per concurrency wave; expected uses half the request timeout; worst uses the full request timeout",
		ConfidenceEscalation: mode.ConfidenceEscalationDisabled,
	}
	if cfg.SingleElim {
		plan.Strategy = "dynamic-single-elimination"
	}
	if cfg.Selection == "absolute" {
		plan.Strategy = "absolute"
	}
	if cfg.Selection == "joint_absolute" {
		plan.Strategy = "joint-absolute"
		plan.BundledCriteria = true
	}
	if reps <= 0 {
		reps = cfg.DefaultReps
	}
	perOrderCalls := len(criteria)
	if cfg.MultiCriterionBundle && len(criteria) > 1 {
		perOrderCalls = 1
	}
	if perOrderCalls == 0 {
		perOrderCalls = 1
	}
	absolute := cfg.Selection == "absolute"
	jointAbsolute := cfg.Selection == "joint_absolute"
	adaptive := !absolute && !jointAbsolute && biasMitigation == "adaptive"
	if adaptive && perOrderCalls > cfg.MaxPairCalls {
		return evalPlan{}, fmt.Errorf("adaptive max_pair_calls=%d cannot fit one order requiring %d provider calls; enable criterion bundling or raise the limit", cfg.MaxPairCalls, perOrderCalls)
	}
	baseOrders := reps
	maxOrders := reps
	if !absolute && biasMitigation == "both" {
		baseOrders *= 2
		maxOrders *= 2
	}
	if adaptive {
		baseOrders = 1
		maxOrders = cfg.MaxPairCalls / perOrderCalls
		if maxOrders < 1 {
			maxOrders = 1
		}
	}

	bestDurationWaves := 0
	expectedDurationWaves := 0
	worstDurationWaves := 0
	for _, task := range tasks {
		preprocessed := task.PreparedTrajectories
		if jointAbsolute {
			prompt, _ := verifier.PromptJointAbsolute(task.Task, preprocessed, criteria, verifier.PromptOptions{CritiqueThenScore: cfg.CritiqueThenScore})
			inputTokens := preprocess.EstimateTokens(prompt) * reps
			plan.CandidateScores += len(preprocessed)
			plan.Calls.Best += reps
			plan.Calls.Expected += reps
			plan.Calls.Worst += reps
			plan.EstimatedInputTokens.Best += inputTokens
			plan.EstimatedInputTokens.Expected += inputTokens
			plan.EstimatedInputTokens.Worst += inputTokens
			bestDurationWaves += reps
			expectedDurationWaves += reps
			worstDurationWaves += reps
			continue
		}
		if absolute {
			inputs := allAbsoluteInputTokens(task.Task, preprocessed, criteria, cfg)
			if len(inputs) == 0 {
				continue
			}
			candidateScores := len(inputs)
			inputTokens := sumInts(inputs)
			calls := candidateScores * perOrderCalls * reps
			plan.CandidateScores += candidateScores
			plan.Calls.Best += calls
			plan.Calls.Expected += calls
			plan.Calls.Worst += calls
			plan.EstimatedInputTokens.Best += inputTokens * reps
			plan.EstimatedInputTokens.Expected += inputTokens * reps
			plan.EstimatedInputTokens.Worst += inputTokens * reps
			waves := absoluteScoreWaves(candidateScores, cfg.MaxWorkers, reps*perOrderCalls)
			bestDurationWaves += waves
			expectedDurationWaves += waves
			worstDurationWaves += waves
			continue
		}
		pairInputs := allPairInputTokens(task.Task, preprocessed, criteria, cfg)
		if len(pairInputs) == 0 {
			continue
		}
		pairMatches := len(pairInputs)
		baseInput := sumInts(pairInputs)
		minInput, maxInput := minMaxInts(pairInputs)
		averageInput := int(math.Ceil(float64(baseInput) / float64(len(pairInputs))))
		if cfg.SingleElim {
			pairMatches = len(preprocessed) - 1
			baseInput = averageInput * pairMatches
		}
		plan.PairMatches += pairMatches
		baseCalls := pairMatches * perOrderCalls * baseOrders
		worstCalls := pairMatches * perOrderCalls * maxOrders
		expectedOrders := float64(baseOrders)
		if adaptive && plan.ConfidenceEscalation == mode.ConfidenceEscalationLegacy {
			expectedOrders += cfg.ExpectedEscalationRate * float64(maxOrders-baseOrders)
		}
		expectedCalls := int(math.Ceil(float64(pairMatches*perOrderCalls) * expectedOrders))
		plan.Calls.Best += baseCalls
		plan.Calls.Expected += expectedCalls
		plan.Calls.Worst += worstCalls

		if cfg.SingleElim {
			plan.EstimatedInputTokens.Best += minInput * pairMatches * baseOrders
			plan.EstimatedInputTokens.Expected += int(math.Ceil(float64(averageInput*pairMatches) * expectedOrders))
			plan.EstimatedInputTokens.Worst += maxInput * pairMatches * maxOrders
		} else {
			plan.EstimatedInputTokens.Best += baseInput * baseOrders
			plan.EstimatedInputTokens.Expected += int(math.Ceil(float64(baseInput) * expectedOrders))
			plan.EstimatedInputTokens.Worst += baseInput * maxOrders
		}
		bestDurationWaves += tournamentWaves(len(preprocessed), cfg.SingleElim, cfg.MaxWorkers, baseOrders*perOrderCalls)
		expectedDurationWaves += tournamentWaves(len(preprocessed), cfg.SingleElim, cfg.MaxWorkers, int(math.Ceil(expectedOrders))*perOrderCalls)
		worstDurationWaves += tournamentWaves(len(preprocessed), cfg.SingleElim, cfg.MaxWorkers, maxOrders*perOrderCalls)
	}

	maxOutputTokens := cfg.MaxTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = 4096
	}
	plan.EstimatedCostUSD.Best = estimatePlanCost(calculator, plan.EstimatedInputTokens.Best, plan.Calls.Best*maxOutputTokens)
	plan.EstimatedCostUSD.Expected = estimatePlanCost(calculator, plan.EstimatedInputTokens.Expected, plan.Calls.Expected*maxOutputTokens)
	plan.EstimatedCostUSD.Worst = estimatePlanCost(calculator, plan.EstimatedInputTokens.Worst, plan.Calls.Worst*maxOutputTokens)
	plan.EstimatedDurationSec.Best = bestDurationWaves
	plan.EstimatedDurationSec.Expected = expectedDurationWaves * maxInt(1, cfg.TimeoutSec/2)
	plan.EstimatedDurationSec.Worst = worstDurationWaves * cfg.TimeoutSec
	return plan, nil
}

func allPairInputTokens(task string, trajectories []string, criteria []verifier.Criterion, cfg config.Config) []int {
	inputs := make([]int, 0, len(trajectories)*(len(trajectories)-1)/2)
	for i := 0; i < len(trajectories); i++ {
		for j := i + 1; j < len(trajectories); j++ {
			inputTokens := 0
			if cfg.MultiCriterionBundle && len(criteria) > 1 {
				prompt, _ := verifier.PromptPairwiseBundled(task, trajectories[i], trajectories[j], criteria, verifier.PromptOptions{CritiqueThenScore: cfg.CritiqueThenScore})
				inputTokens = preprocess.EstimateTokens(prompt)
			} else {
				for _, criterion := range criteria {
					prompt, _ := verifier.PromptPairwise(task, trajectories[i], trajectories[j], criterion, verifier.PromptOptions{CritiqueThenScore: cfg.CritiqueThenScore})
					inputTokens += preprocess.EstimateTokens(prompt)
				}
			}
			inputs = append(inputs, inputTokens)
		}
	}
	return inputs
}

func allAbsoluteInputTokens(task string, trajectories []string, criteria []verifier.Criterion, cfg config.Config) []int {
	inputs := make([]int, 0, len(trajectories))
	for _, trajectory := range trajectories {
		inputTokens := 0
		if cfg.MultiCriterionBundle && len(criteria) > 1 {
			prompt, _ := verifier.PromptAbsoluteBundled(task, trajectory, criteria, verifier.PromptOptions{CritiqueThenScore: cfg.CritiqueThenScore})
			inputTokens = preprocess.EstimateTokens(prompt)
		} else {
			for _, criterion := range criteria {
				prompt, _ := verifier.PromptAbsolute(task, trajectory, criterion, verifier.PromptOptions{CritiqueThenScore: cfg.CritiqueThenScore})
				inputTokens += preprocess.EstimateTokens(prompt)
			}
		}
		inputs = append(inputs, inputTokens)
	}
	return inputs
}

func absoluteScoreWaves(candidates, workers, callsPerCandidate int) int {
	if candidates < 1 || callsPerCandidate < 1 {
		return 0
	}
	if workers < 1 {
		workers = 1
	}
	return int(math.Ceil(float64(candidates)/float64(workers))) * callsPerCandidate
}

func tournamentWaves(trajectories int, singleElim bool, workers, orders int) int {
	if trajectories < 2 || orders < 1 {
		return 0
	}
	if workers < 1 {
		workers = 1
	}
	if !singleElim {
		pairs := trajectories * (trajectories - 1) / 2
		return int(math.Ceil(float64(pairs)/float64(workers))) * orders
	}
	waves := 0
	active := trajectories
	for active > 1 {
		matches := active / 2
		waves += int(math.Ceil(float64(matches)/float64(workers))) * orders
		active = (active + 1) / 2
	}
	return waves
}

func resolveEvalLimits(flags evalLimitFlags, plan evalPlan, cfg config.Config) mode.BudgetLimits {
	limits := requestedEvalLimits(flags)
	if limits.MaxCalls == 0 {
		limits.MaxCalls = plan.Calls.Worst
	}
	if limits.MaxAttempts == 0 {
		limits.MaxAttempts = plan.Calls.Worst * (cfg.MaxRetries + 1)
	}
	if limits.MaxEstimatedInputTokens == 0 {
		limits.MaxEstimatedInputTokens = plan.EstimatedInputTokens.Worst * (cfg.MaxRetries + 1)
	}
	maxOutputTokens := cfg.MaxTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultAttestationOutputTokens
	}
	if limits.MaxReservedOutputTokens == 0 {
		limits.MaxReservedOutputTokens = limits.MaxAttempts * maxOutputTokens
	}
	if limits.MaxConcurrent == 0 {
		limits.MaxConcurrent = cfg.MaxWorkers
	}
	if limits.MaxCostUSD == 0 {
		limits.MaxCostUSD = plan.EstimatedCostUSD.Worst * float64(cfg.MaxRetries+1)
	}
	if limits.MaxDuration == 0 {
		limits.MaxDuration = time.Duration(maxInt(1, plan.EstimatedDurationSec.Worst*(cfg.MaxRetries+1))) * time.Second
	}
	return limits
}

func requestedEvalLimits(flags evalLimitFlags) mode.BudgetLimits {
	return mode.BudgetLimits{
		MaxCalls:                *flags.maxCalls,
		MaxAttempts:             *flags.maxAttempts,
		MaxEstimatedInputTokens: *flags.maxInputTokens,
		MaxReservedOutputTokens: *flags.maxOutputTokens,
		MaxConcurrent:           *flags.maxConcurrent,
		MaxCostUSD:              *flags.maxCostUSD,
		MaxDuration:             *flags.maxDuration,
	}
}

func applyEvalLimits(plan *evalPlan, flags evalLimitFlags, cfg config.Config) {
	plan.Limits = resolveEvalLimits(flags, *plan, cfg)
	plan.HardDurationSeconds = int(math.Ceil(plan.Limits.MaxDuration.Seconds()))
}

func validateEvalLimitFlags(flags evalLimitFlags) error {
	if *flags.maxCalls < 0 {
		return fmt.Errorf("--max-calls must be >= 0")
	}
	if *flags.maxInputTokens < 0 {
		return fmt.Errorf("--max-input-tokens must be >= 0")
	}
	if *flags.maxAttempts < 0 || *flags.maxOutputTokens < 0 || *flags.maxConcurrent < 0 {
		return fmt.Errorf("--max-attempts, --max-output-tokens, and --max-concurrent must be >= 0")
	}
	if *flags.maxCostUSD < 0 {
		return fmt.Errorf("--max-cost-usd must be >= 0")
	}
	if *flags.maxDuration < 0 {
		return fmt.Errorf("--max-duration must be >= 0")
	}
	return nil
}

func printEvalPlan(plan evalPlan) {
	fmt.Fprintf(os.Stderr, "preflight: strategy=%s swing=%d scored=%d pairs=%d candidates=%d evidence=%d\n", plan.Strategy, plan.SwingTasks, plan.ScoredTasks, plan.PairMatches, plan.CandidateScores, plan.EvidenceTokens)
	fmt.Fprintf(os.Stderr, "preflight calls: best=%d expected=%d worst=%d\n", plan.Calls.Best, plan.Calls.Expected, plan.Calls.Worst)
	fmt.Fprintf(os.Stderr, "preflight input: best=%d expected=%d worst=%d estimated_tokens\n", plan.EstimatedInputTokens.Best, plan.EstimatedInputTokens.Expected, plan.EstimatedInputTokens.Worst)
	fmt.Fprintf(os.Stderr, "preflight cost: best=$%.4f expected=$%.4f worst=$%.4f\n", plan.EstimatedCostUSD.Best, plan.EstimatedCostUSD.Expected, plan.EstimatedCostUSD.Worst)
	fmt.Fprintf(os.Stderr, "hard limits: calls=%d attempts=%d input=%d output=%d concurrency=%d cost=$%.4f duration=%s\n", plan.Limits.MaxCalls, plan.Limits.MaxAttempts, plan.Limits.MaxEstimatedInputTokens, plan.Limits.MaxReservedOutputTokens, plan.Limits.MaxConcurrent, plan.Limits.MaxCostUSD, plan.Limits.MaxDuration)
	if plan.Authorization != nil {
		fmt.Fprintf(os.Stderr, "authorization digest: %s\n", plan.Authorization.AuthorizationDigest)
	}
	if plan.StatisticalDesign != nil {
		fmt.Fprintf(os.Stderr, "preflight design: total=%d decidable=%d (%.1f%%) nominal_alpha=%.4f adjusted_alpha=%.4f target_power=%.2f\n",
			plan.StatisticalDesign.TotalTasks, plan.StatisticalDesign.DecidableTasks, plan.StatisticalDesign.DecidableShare*100,
			plan.StatisticalDesign.NominalAlpha, plan.StatisticalDesign.AdjustedAlpha, plan.StatisticalDesign.TargetPower)
		for _, warning := range plan.StatisticalDesign.Warnings {
			fmt.Fprintln(os.Stderr, "preflight design WARNING:", warning)
		}
	}
}

func estimatePlanCost(calculator *cost.Calculator, inputTokens, outputTokens int) float64 {
	if calculator == nil {
		return 0
	}
	estimate := calculator.Estimate(inputTokens, outputTokens, 0)
	if estimate == nil {
		return 0
	}
	return *estimate
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func minMaxInts(values []int) (int, int) {
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return minimum, maximum
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// evalRoute records which route produced a result. A filename is not provenance:
// a published benchmark number has to name the provider, the requested model and
// the identifier the endpoint actually reported, which can differ when a gateway
// sits in between.
type evalRoute struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	ServedModel      string `json:"served_model,omitempty"`
	BaseURL          string `json:"base_url"`
	EvaluatedAt      string `json:"evaluated_at"`
	EvalWitnessBuild string `json:"evalwitness_version"`
}

func buildEvalRoute(cfg config.Config, servedModel string) *evalRoute {
	return &evalRoute{
		Provider:         cfg.Provider,
		Model:            cfg.Model,
		ServedModel:      servedModel,
		BaseURL:          cfg.BaseURL,
		EvaluatedAt:      time.Now().UTC().Format(time.RFC3339),
		EvalWitnessBuild: version,
	}
}
