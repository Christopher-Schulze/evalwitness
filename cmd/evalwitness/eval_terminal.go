package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/baseline"
	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type terminalEvalTrajectory struct {
	TrialName string `json:"trial_name"`
	TaskName  string `json:"task_name"`
	Reward    int    `json:"reward"`
	// The cost fields back the zero-cost baselines. They are recorded by the
	// dataset and never reach a provider; a selector that reads them has to be
	// beaten before a selector that calls a model is worth its calls.
	OutputTokens    int     `json:"n_output_tokens"`
	DurationSeconds float64 `json:"duration_seconds"`
	CostCents       float64 `json:"cost_cents"`
	Trajectory      struct {
		Steps []struct {
			Source  string `json:"source"`
			Message string `json:"message"`
		} `json:"steps"`
	} `json:"trajectory"`
}

type terminalEvalTrial struct {
	Name     string
	Path     string
	Reward   int
	Raw      string
	Task     string
	Baseline baseline.Candidate
}

type terminalEvalTask struct {
	Name   string
	Trials []terminalEvalTrial
}

type terminalEvalSummary struct {
	SchemaVersion   string                   `json:"schema_version"`
	TrajectorySet   string                   `json:"trajectory_set"`
	Route           *evalRoute               `json:"route,omitempty"`
	Tasks           int                      `json:"tasks"`
	Trials          int                      `json:"trials"`
	SwingTasks      int                      `json:"swing_tasks"`
	AllPassTasks    int                      `json:"all_pass_tasks"`
	AllFailTasks    int                      `json:"all_fail_tasks"`
	PassAt1Score    float64                  `json:"pass_at_1_score"`
	PassAt1Rate     float64                  `json:"pass_at_1_rate"`
	OracleScore     float64                  `json:"oracle_score"`
	OracleRate      float64                  `json:"oracle_rate"`
	VerifierScore   *float64                 `json:"verifier_score,omitempty"`
	VerifierRate    *float64                 `json:"verifier_rate,omitempty"`
	DryRun          bool                     `json:"dry_run"`
	Usage           mode.UsageSummary        `json:"usage"`
	DurationSeconds float64                  `json:"duration_seconds"`
	Details         []terminalEvalTaskResult `json:"details,omitempty"`
	// Baselines score zero-cost selectors over the same decidable tasks, so a
	// verifier result is reported against something other than random. They
	// need no provider and are therefore present in dry runs too.
	Baselines []baseline.Result `json:"baselines,omitempty"`
	// Calibration asks whether win_probability means what it says. Three
	// mechanisms read that number - the K=1 stop, escalation, and
	// reverse-order evaluation - and none was ever checked against ground
	// truth. Present only where rewards exist, hence eval-only.
	Calibration       *reliability.Report                  `json:"calibration,omitempty"`
	CalibrationPolicy verification.CalibrationPolicyStatus `json:"calibration_policy"`
	Plan              *evalPlan                            `json:"plan,omitempty"`
	Budget            mode.BudgetSnapshot                  `json:"budget"`
	Escalation        *evalEscalationSummary               `json:"escalation,omitempty"`
	Fidelity          *preprocess.AccountingAggregate      `json:"fidelity,omitempty"`
	Comparisons       []evalObservedComparison             `json:"paired_comparisons,omitempty"`
}

type terminalEvalTaskResult struct {
	TaskName       string            `json:"task_name"`
	Rewards        []int             `json:"rewards"`
	PassAt1        float64           `json:"pass_at_1"`
	Oracle         int               `json:"oracle"`
	SelectedIndex  *int              `json:"selected_index,omitempty"`
	SelectedReward *int              `json:"selected_reward,omitempty"`
	Selection      *mode.Selection   `json:"selection,omitempty"`
	SkippedReason  string            `json:"skipped_reason,omitempty"`
	Usage          mode.UsageSummary `json:"usage,omitempty"`
	Error          string            `json:"error,omitempty"`
}

func runEvalTerminal(args []string) (exitCode int) {
	fs := flag.NewFlagSet("eval-terminal", flag.ExitOnError)
	trajs := fs.String("trajs", "forge_gpt54", "trajectory set under eval/trajectories/terminal_trajs")
	root := fs.String("root", "eval/trajectories/terminal_trajs", "terminal trajectory root")
	criteriaFlag := fs.String("criteria", "specification,output_match,error_signals", "comma-separated criterion ids")
	nReps := fs.Int("n-reps", 0, "reps per pair/criterion (0 = config default)")
	limit := fs.Int("limit", 0, "max tasks to evaluate (0 = all)")
	maxWorkers := fs.Int("max-workers", 0, "override EVALWITNESS_MAX_WORKERS")
	noBoth := fs.Bool("no-bias-mit", false, "disable order-bias mitigation")
	noCrit := fs.Bool("no-critique", false, "disable critique-then-score")
	noSPRT := fs.Bool("no-sprt", false, "disable SPRT adaptive reps")
	paperParity := fs.Bool("paper-parity", false, "reference-parity mode: paper criteria + note, no critique, no bundling, single order, 4 reps")
	judgeMode := fs.Bool("judge-mode", false, "LLM-as-a-Judge comparison run: no logprob requests, raw-text extraction")
	noCache := fs.Bool("no-cache", false, "disable disk cache for this run")
	dryRun := fs.Bool("dry-run", false, "load/tally dataset without provider calls")
	output := fs.String("output", "json", "json/text")
	details := fs.Bool("details", false, "include per-task details in JSON output")
	limitFlags := addEvalLimitFlags(fs)
	designFlags := addEvalDesignFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *limit < 0 {
		fmt.Fprintln(os.Stderr, "eval-terminal: --limit must be >= 0")
		return 2
	}
	if err := validateEvalLimitFlags(limitFlags); err != nil {
		fmt.Fprintln(os.Stderr, "eval-terminal:", err)
		return 2
	}
	if err := validateEvalDesignFlags(designFlags); err != nil {
		fmt.Fprintln(os.Stderr, "eval-terminal:", err)
		return 2
	}
	if *maxWorkers > 0 {
		if err := os.Setenv("EVALWITNESS_MAX_WORKERS", fmt.Sprintf("%d", *maxWorkers)); err != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal:", err)
			return 1
		}
	}
	if *noCrit {
		if err := os.Setenv("EVALWITNESS_CRITIQUE_THEN_SCORE", "false"); err != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal:", err)
			return 1
		}
	}
	if *judgeMode {
		if err := os.Setenv("EVALWITNESS_JUDGE_MODE", "true"); err != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal:", err)
			return 1
		}
	}

	criteria, err := verifier.ResolveCriteria(splitTrim(*criteriaFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, "criteria:", err)
		return 2
	}
	if *paperParity {
		criteria = verifier.PaperTerminalCriteria()
		if err := applyPaperParityEnv(fs, "criteria", nReps); err != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal: paper parity:", err)
			return 1
		}
		*noBoth = true
		*noSPRT = true
	}
	tasks, err := loadTerminalEvalTasks(filepath.Join(*root, *trajs), *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-terminal:", err)
		return 1
	}
	start := time.Now()

	var cfg config.Config
	var plan *evalPlan
	var batchResult verification.BatchResult
	var preparedTasks [][]preprocess.Result
	taskBatchIndexes := make([]int, len(tasks))
	for index := range taskBatchIndexes {
		taskBatchIndexes[index] = -1
	}
	if *dryRun {
		cfg, err = config.LoadForDiagnostics()
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	var researchRecord *study.Record
	if cfg.ReplayTo != "" {
		record, closeRecord, recordErr := readStudyRecord(*limitFlags.studyRecord)
		if recordErr != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal research capture:", recordErr)
			return 1
		}
		closeRecord()
		researchRecord = &record
		tasks, err = filterTerminalResearchTasks(tasks, record)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal research capture:", err)
			return 1
		}
	}
	service, serviceErr := verification.NewProductionService(cfg, version)
	if serviceErr != nil {
		fmt.Fprintln(os.Stderr, "eval-terminal init:", serviceErr)
		return 1
	}
	reps := *nReps
	if reps <= 0 {
		reps = cfg.DefaultReps
	}
	biasMitigation := cfg.BiasMitigation
	if *noBoth {
		biasMitigation = "single"
	}
	useSPRT := cfg.SPRT.Enabled && !*noSPRT
	inputs := make([]verification.Input, 0, len(tasks))
	planTasks := make([]evalPlanTask, 0, len(tasks))
	swingTaskCount := 0
	preparedTasks = make([][]preprocess.Result, len(tasks))
	for taskIndex, task := range tasks {
		rewardSum := 0
		for _, trial := range task.Trials {
			rewardSum += trial.Reward
		}
		if rewardSum > 0 && rewardSum < len(task.Trials) {
			swingTaskCount++
		}
		if (rewardSum > 0 && rewardSum < len(task.Trials)) || cfg.Selection == "joint_absolute" {
			trajectories := make([]string, len(task.Trials))
			for trialIndex, trial := range task.Trials {
				trajectories[trialIndex] = trial.Raw
			}
			taskBatchIndexes[taskIndex] = len(inputs)
			input := verification.Input{
				Entrypoint: "eval-terminal", Mode: verification.ModePairwise, Task: task.Trials[0].Task,
				Trajectories: trajectories, Criteria: criteria,
				Policy:              verification.PolicyFromConfig(cfg, reps, cfg.Epsilon, biasMitigation, useSPRT, cfg.Selection),
				Limits:              requestedEvalLimits(limitFlags),
				AuthorizationDigest: *limitFlags.authorize, BudgetStatePath: cfg.RunBudgetState, DisableCache: *noCache,
			}
			if researchRecord != nil {
				if err := bindTerminalResearchLineage(&input, *researchRecord, task.Name); err != nil {
					fmt.Fprintln(os.Stderr, "eval-terminal research capture:", err)
					return 1
				}
			}
			inputs = append(inputs, input)
		}
	}
	var batchPlan verification.BatchPlan
	if len(inputs) > 0 {
		batchPlan, err = service.PlanBatch(inputs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal preflight:", err)
			return 2
		}
		for taskIndex, batchIndex := range taskBatchIndexes {
			if batchIndex < 0 {
				continue
			}
			preparedTasks[taskIndex] = batchPlan.Plans[batchIndex].PreparedTrajectories
			planTasks = append(planTasks, evalPlanTask{
				Task:                 batchPlan.Plans[batchIndex].Input.Task,
				PreparedTrajectories: evalPreparedTexts(batchPlan.Plans[batchIndex].PreparedTrajectories),
			})
		}
	}
	calculator := cost.New(cfg.InputUSDPerM, cfg.CachedUSDPerM, cfg.OutputUSDPerM, cfg.BillingModel == "subscription")
	calculated, err := buildEvalPlan(planTasks, cfg, calculator, criteria, *nReps, biasMitigation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-terminal preflight:", err)
		return 2
	}
	calculated.SwingTasks = swingTaskCount
	if len(inputs) > 0 {
		calculated.Limits = batchPlan.Plans[0].Input.Limits
		calculated.HardDurationSeconds = int(math.Ceil(calculated.Limits.MaxDuration.Seconds()))
	} else {
		applyEvalLimits(&calculated, limitFlags, cfg)
	}
	statisticalPlan, designErr := buildEvalStatisticalPlan(len(tasks), terminalRewardRows(tasks), designFlags)
	if designErr != nil {
		fmt.Fprintln(os.Stderr, "eval-terminal design:", designErr)
		return 2
	}
	calculated.StatisticalDesign = &statisticalPlan
	plan = &calculated
	if *dryRun {
		printEvalPlan(calculated)
	} else {
		offlineExecution := cfg.ReplayFrom != "" || cfg.Offline
		if !offlineExecution {
			authorized, authorizationErr := authorizeEvalPlan(plan, cfg, criteria, limitFlags, "eval-terminal")
			printEvalPlan(calculated)
			if authorizationErr != nil {
				fmt.Fprintln(os.Stderr, "eval-terminal authorization:", authorizationErr)
				return 2
			}
			if !authorized {
				return 0
			}
		} else {
			printEvalPlan(calculated)
		}
		if len(inputs) > 0 && batchPlan.Authorization != nil {
			fmt.Fprintf(os.Stderr, "batch authorization digest: %s\n", batchPlan.Authorization.AuthorizationDigest)
		}
		if len(inputs) > 0 {
			start = time.Now()
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			batchResult, err = service.ExecuteBatch(ctx, batchPlan)
			cancel()
			if err != nil {
				fmt.Fprintln(os.Stderr, "eval-terminal:", err)
				return 1
			}
		}
	}

	summary := terminalEvalSummary{SchemaVersion: evalArtifactSchemaVersion, TrajectorySet: *trajs, DryRun: *dryRun, Plan: plan, CalibrationPolicy: verification.DefaultCalibrationPolicy()}
	if len(preparedTasks) > 0 {
		fidelity := aggregatePreparedEvidence(preparedTasks)
		summary.Fidelity = &fidelity
	}
	if *details {
		summary.Details = make([]terminalEvalTaskResult, 0, len(tasks))
	}
	baselineTasks := make([]baseline.Task, 0, len(tasks))
	var calibration []reliability.Observation
	var absoluteReliability []reliability.RankObservation
	selections := make([]mode.Selection, len(tasks))
	if !*dryRun {
		for taskIndex, batchIndex := range taskBatchIndexes {
			if batchIndex >= 0 {
				selections[taskIndex] = *batchResult.Results[batchIndex].Selection
			}
		}
	}

	var verifierScore float64
	verifierCount := 0
	for taskIndex, task := range tasks {
		res := terminalEvalTaskResult{TaskName: task.Name}
		trialCount := len(task.Trials)
		summary.Tasks++
		summary.Trials += trialCount

		rewards := make([]int, trialCount)
		oracle := 0
		rewardSum := 0
		for i, tr := range task.Trials {
			rewards[i] = tr.Reward
			rewardSum += tr.Reward
			if tr.Reward > oracle {
				oracle = tr.Reward
			}
		}
		res.Rewards = rewards
		candidates := make([]baseline.Candidate, trialCount)
		for i, tr := range task.Trials {
			candidates[i] = tr.Baseline
		}
		res.Oracle = oracle
		if trialCount > 0 {
			res.PassAt1 = float64(rewardSum) / float64(trialCount)
		}
		summary.PassAt1Score += res.PassAt1
		summary.OracleScore += float64(oracle)
		if !*dryRun && taskBatchIndexes[taskIndex] >= 0 {
			selection := selections[taskIndex]
			res.Selection = &selection
			res.Usage = selection.Usage
			mode.AddUsage(&summary.Usage, selection.Usage)
			summary.Escalation = addEvalEscalation(summary.Escalation, selection)
		}

		switch rewardSum {
		case trialCount:
			summary.AllPassTasks++
			selected := 0
			selectedReward := 1
			res.SelectedIndex = &selected
			res.SelectedReward = &selectedReward
			res.SkippedReason = "all_pass"
			verifierScore += 1
			verifierCount++
		case 0:
			summary.AllFailTasks++
			selected := 0
			selectedReward := 0
			res.SelectedIndex = &selected
			res.SelectedReward = &selectedReward
			res.SkippedReason = "all_fail"
			verifierCount++
		default:
			summary.SwingTasks++
			if *dryRun {
				res.SkippedReason = "dry_run"
				break
			}
			selection := selections[taskIndex]
			if selection.State != verifier.DecisionSelected {
				res.SkippedReason = "selection_" + string(selection.State)
				break
			}
			selected := selection.BestIndex
			selectedReward := task.Trials[selected].Reward
			res.SelectedIndex = &selected
			res.SelectedReward = &selectedReward
			verifierScore += float64(selectedReward)
			verifierCount++
		}
		pairRows, rankRows := evalReliabilityRows(task.Name, rewards, res.Selection, cfg.MaxPairCalls)
		calibration = append(calibration, pairRows...)
		absoluteReliability = append(absoluteReliability, rankRows...)
		baselineTasks = append(baselineTasks, baseline.Task{
			Candidates:    candidates,
			Rewards:       rewards,
			SelectedIndex: res.SelectedIndex,
		})
		if *details {
			summary.Details = append(summary.Details, res)
		}
	}

	if len(calibration) > 0 || len(absoluteReliability) > 0 {
		report := reliability.AnalyzeWithAbsolute(calibration, absoluteReliability)
		summary.Calibration = &report
	}
	summary.Baselines = baseline.Evaluate(baseline.Terminal(), baselineTasks)
	if !*dryRun && plan != nil && plan.StatisticalDesign != nil {
		summary.Comparisons, err = buildObservedComparisons(summary.Baselines, *plan.StatisticalDesign)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eval-terminal paired analysis:", err)
			return 1
		}
	}

	if summary.Tasks > 0 {
		summary.PassAt1Rate = summary.PassAt1Score / float64(summary.Tasks)
		summary.OracleRate = summary.OracleScore / float64(summary.Tasks)
	}
	if verifierCount == summary.Tasks && !*dryRun {
		vScore := verifierScore
		vRate := verifierScore / float64(summary.Tasks)
		summary.VerifierScore = &vScore
		summary.VerifierRate = &vRate
	}
	summary.DurationSeconds = math.Round(time.Since(start).Seconds()*1000) / 1000
	summary.Budget = batchResult.Budget
	if !*dryRun {
		summary.Route = buildEvalRoute(cfg, batchResult.ServedModel)
	}

	switch *output {
	case "json":
		b, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Println(string(b))
	case "text":
		printTerminalEvalText(summary)
	default:
		fmt.Fprintln(os.Stderr, "unknown --output:", *output)
		return 2
	}
	return 0
}

func loadTerminalEvalTasks(agentDir string, limit int) ([]terminalEvalTask, error) {
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("trajectory data not found at %s; fetch it with eval/fetch-eval-data.sh (release asset, ~226 MB unpacked)", agentDir)
		}
		return nil, err
	}
	var tasks []terminalEvalTask
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskDir := filepath.Join(agentDir, entry.Name())
		trials, err := loadTerminalEvalTrials(taskDir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if len(trials) == 0 {
			continue
		}
		tasks = append(tasks, terminalEvalTask{Name: entry.Name(), Trials: trials})
		if limit > 0 && len(tasks) >= limit {
			break
		}
	}
	if len(tasks) == 0 {
		return nil, errors.New("no terminal-bench tasks found")
	}
	return tasks, nil
}

func loadTerminalEvalTrials(taskDir string) ([]terminalEvalTrial, error) {
	matches, err := filepath.Glob(filepath.Join(taskDir, "*_trajectory.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	trials := make([]terminalEvalTrial, 0, len(matches))
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var parsed terminalEvalTrajectory
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, err
		}
		task := extractTerminalEvalTask(parsed)
		if task == "" {
			task = parsed.TaskName
		}
		var trace strings.Builder
		for _, step := range parsed.Trajectory.Steps {
			trace.WriteString(step.Message)
			trace.WriteByte('\n')
		}
		trials = append(trials, terminalEvalTrial{
			Name:   parsed.TrialName,
			Path:   path,
			Reward: parsed.Reward,
			Raw:    string(raw),
			Task:   task,
			Baseline: baseline.Candidate{
				Steps:        len(parsed.Trajectory.Steps),
				OutputTokens: parsed.OutputTokens,
				TraceBytes:   trace.Len(),
				ErrorWords:   baseline.CountErrorWords(trace.String()),
				DurationSec:  parsed.DurationSeconds,
				CostCents:    parsed.CostCents,
			},
		})
	}
	return trials, nil
}

func extractTerminalEvalTask(parsed terminalEvalTrajectory) string {
	for _, step := range parsed.Trajectory.Steps {
		if step.Source == "user" && strings.TrimSpace(step.Message) != "" {
			return step.Message
		}
	}
	return ""
}

func printTerminalEvalText(summary terminalEvalSummary) {
	fmt.Printf("Terminal-Bench trajectory set: %s\n", summary.TrajectorySet)
	fmt.Printf("Tasks: %d  Trials: %d  Swing: %d  All-pass: %d  All-fail: %d\n",
		summary.Tasks, summary.Trials, summary.SwingTasks, summary.AllPassTasks, summary.AllFailTasks)
	fmt.Printf("Pass@1: %.1f/%d (%.1f%%)\n", summary.PassAt1Score, summary.Tasks, summary.PassAt1Rate*100)
	fmt.Printf("Oracle: %.1f/%d (%.1f%%)\n", summary.OracleScore, summary.Tasks, summary.OracleRate*100)
	if summary.VerifierScore != nil && summary.VerifierRate != nil {
		fmt.Printf("evalwitness selected: %.1f/%d (%.1f%%)\n", *summary.VerifierScore, summary.Tasks, *summary.VerifierRate*100)
	} else if summary.DryRun {
		fmt.Println("evalwitness selected: skipped (--dry-run)")
	}
	fmt.Printf("Provider calls: %d  Cache hits: %d  Input tokens: %d  Output tokens: %d\n",
		summary.Usage.Calls, summary.Usage.CacheHitCalls, summary.Usage.InputTokens, summary.Usage.OutputTokens)
	if summary.Escalation != nil {
		fmt.Printf("Pair calls: %d  Escalated pairs: %d/%d  Inconsistent pairs: %d\n",
			summary.Escalation.TotalPairCalls, summary.Escalation.EscalatedPairs,
			summary.Escalation.PairsEvaluated, summary.Escalation.InconsistentPairs)
	}
	printReliabilitySummary(summary.Calibration)
	if summary.Plan != nil && summary.Plan.StatisticalDesign != nil {
		printEvalStatisticalPlan(*summary.Plan.StatisticalDesign)
	}
	printObservedComparisons(summary.Comparisons)
}

func terminalRewardRows(tasks []terminalEvalTask) [][]int {
	rows := make([][]int, len(tasks))
	for taskIndex, task := range tasks {
		rows[taskIndex] = make([]int, len(task.Trials))
		for trialIndex, trial := range task.Trials {
			rows[taskIndex][trialIndex] = trial.Reward
		}
	}
	return rows
}
