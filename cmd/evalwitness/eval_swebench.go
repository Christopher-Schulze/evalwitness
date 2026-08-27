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
	"regexp"
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
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

var (
	swePRDescriptionRE = regexp.MustCompile(`(?s)<pr_description>(.*?)</pr_description>`)
	sweInstructionsRE  = regexp.MustCompile(`(?s)<instructions>(.*?)</instructions>`)
)

type swebenchCacheItem struct {
	InstanceID   string  `json:"instance_id"`
	Messages     string  `json:"messages"`
	ModelName    string  `json:"model_name"`
	NumSteps     int     `json:"num_steps"`
	OutputPatch  string  `json:"output_patch"`
	Reward       float64 `json:"reward"`
	TrajectoryID string  `json:"trajectory_id"`
}

type swebenchTrial struct {
	Run      string
	Name     string
	Reward   int
	Problem  string
	Trace    string
	Model    string
	NumSteps int
	Baseline baseline.Candidate
}

type swebenchTask struct {
	InstanceID string
	Trials     []swebenchTrial
}

type swebenchSummary struct {
	SchemaVersion   string               `json:"schema_version"`
	Runs            []string             `json:"runs"`
	Tasks           int                  `json:"tasks"`
	Trials          int                  `json:"trials"`
	SwingTasks      int                  `json:"swing_tasks"`
	AllPassTasks    int                  `json:"all_pass_tasks"`
	AllFailTasks    int                  `json:"all_fail_tasks"`
	PassAt1Score    float64              `json:"pass_at_1_score"`
	PassAt1Rate     float64              `json:"pass_at_1_rate"`
	OracleScore     float64              `json:"oracle_score"`
	OracleRate      float64              `json:"oracle_rate"`
	VerifierScore   *float64             `json:"verifier_score,omitempty"`
	VerifierRate    *float64             `json:"verifier_rate,omitempty"`
	DryRun          bool                 `json:"dry_run"`
	Usage           mode.UsageSummary    `json:"usage"`
	DurationSeconds float64              `json:"duration_seconds"`
	Details         []swebenchTaskResult `json:"details,omitempty"`
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
	Route             *evalRoute                           `json:"route,omitempty"`
	Fidelity          *preprocess.AccountingAggregate      `json:"fidelity,omitempty"`
	Comparisons       []evalObservedComparison             `json:"paired_comparisons,omitempty"`
}

type swebenchTaskResult struct {
	InstanceID     string            `json:"instance_id"`
	Runs           []string          `json:"runs"`
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

func runEvalSWEbench(args []string) (exitCode int) {
	fs := flag.NewFlagSet("eval-swebench", flag.ExitOnError)
	root := fs.String("root", "eval/trajectories/swebench_verified_trajs", "SWE-bench trajectory root")
	runsFlag := fs.String("runs", "", "comma-separated run directories (default: all)")
	criteriaFlag := fs.String("criteria", "root_cause,code_review,verification", "comma-separated criterion ids")
	nReps := fs.Int("n-reps", 0, "reps per pair/criterion (0 = config default)")
	limit := fs.Int("limit", 0, "max instances to evaluate (0 = all)")
	maxWorkers := fs.Int("max-workers", 0, "override EVALWITNESS_MAX_WORKERS")
	noBoth := fs.Bool("no-bias-mit", false, "disable order-bias mitigation")
	noCrit := fs.Bool("no-critique", false, "disable critique-then-score")
	noSPRT := fs.Bool("no-sprt", false, "disable SPRT adaptive reps")
	paperParity := fs.Bool("paper-parity", false, "reference-parity mode: paper criteria + note, no critique, no bundling, single order, 4 reps")
	judgeMode := fs.Bool("judge-mode", false, "LLM-as-a-Judge comparison run: no logprob requests, raw-text extraction")
	noCache := fs.Bool("no-cache", false, "disable disk cache for this run")
	dryRun := fs.Bool("dry-run", false, "load/tally dataset without provider calls")
	output := fs.String("output", "json", "json/text")
	details := fs.Bool("details", false, "include per-instance details in JSON output")
	limitFlags := addEvalLimitFlags(fs)
	designFlags := addEvalDesignFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *limit < 0 {
		fmt.Fprintln(os.Stderr, "eval-swebench: --limit must be >= 0")
		return 2
	}
	if err := validateEvalLimitFlags(limitFlags); err != nil {
		fmt.Fprintln(os.Stderr, "eval-swebench:", err)
		return 2
	}
	if err := validateEvalDesignFlags(designFlags); err != nil {
		fmt.Fprintln(os.Stderr, "eval-swebench:", err)
		return 2
	}
	if *maxWorkers > 0 {
		if err := os.Setenv("EVALWITNESS_MAX_WORKERS", fmt.Sprintf("%d", *maxWorkers)); err != nil {
			fmt.Fprintln(os.Stderr, "eval-swebench: set max workers:", err)
			return 2
		}
	}
	if *noCrit {
		if err := os.Setenv("EVALWITNESS_CRITIQUE_THEN_SCORE", "false"); err != nil {
			fmt.Fprintln(os.Stderr, "eval-swebench: disable critique:", err)
			return 2
		}
	}
	if *judgeMode {
		if err := os.Setenv("EVALWITNESS_JUDGE_MODE", "true"); err != nil {
			fmt.Fprintln(os.Stderr, "eval-swebench: enable judge mode:", err)
			return 2
		}
	}

	criteria, err := verifier.ResolveCriteria(splitTrim(*criteriaFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, "criteria:", err)
		return 2
	}
	if *paperParity {
		criteria = verifier.PaperSWECriteria()
		if err := applyPaperParityEnv(fs, "criteria", nReps); err != nil {
			fmt.Fprintln(os.Stderr, "eval-swebench: paper parity:", err)
			return 2
		}
		*noBoth = true
		*noSPRT = true
	}
	tasks, runs, err := loadSWEbenchTasks(*root, splitTrim(*runsFlag), *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval-swebench:", err)
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
	service, serviceErr := verification.NewProductionService(cfg, version)
	if serviceErr != nil {
		fmt.Fprintln(os.Stderr, "eval-swebench init:", serviceErr)
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
	preparedTasks = make([][]preprocess.Result, len(tasks))
	for taskIndex, task := range tasks {
		rewardSum := 0
		for _, trial := range task.Trials {
			rewardSum += trial.Reward
		}
		if rewardSum > 0 && rewardSum < len(task.Trials) {
			trajectories := make([]string, len(task.Trials))
			for trialIndex, trial := range task.Trials {
				trajectories[trialIndex] = trial.Trace
			}
			taskBatchIndexes[taskIndex] = len(inputs)
			inputs = append(inputs, verification.Input{
				Entrypoint: "eval-swebench", Mode: verification.ModePairwise, Task: task.Trials[0].Problem,
				Trajectories: trajectories, Criteria: criteria,
				Policy:              verification.PolicyFromConfig(cfg, reps, cfg.Epsilon, biasMitigation, useSPRT, cfg.Selection),
				Limits:              requestedEvalLimits(limitFlags),
				AuthorizationDigest: *limitFlags.authorize, BudgetStatePath: cfg.RunBudgetState, DisableCache: *noCache,
			})
		}
	}
	var batchPlan verification.BatchPlan
	if len(inputs) > 0 {
		batchPlan, err = service.PlanBatch(inputs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eval-swebench preflight:", err)
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
		fmt.Fprintln(os.Stderr, "eval-swebench preflight:", err)
		return 2
	}
	if len(inputs) > 0 {
		calculated.Limits = batchPlan.Plans[0].Input.Limits
		calculated.HardDurationSeconds = int(math.Ceil(calculated.Limits.MaxDuration.Seconds()))
	} else {
		applyEvalLimits(&calculated, limitFlags, cfg)
	}
	statisticalPlan, designErr := buildEvalStatisticalPlan(len(tasks), swebenchRewardRows(tasks), designFlags)
	if designErr != nil {
		fmt.Fprintln(os.Stderr, "eval-swebench design:", designErr)
		return 2
	}
	calculated.StatisticalDesign = &statisticalPlan
	plan = &calculated
	if *dryRun {
		printEvalPlan(calculated)
	} else {
		offlineExecution := cfg.ReplayFrom != "" || cfg.Offline
		if !offlineExecution {
			authorized, authorizationErr := authorizeEvalPlan(plan, cfg, criteria, limitFlags, "eval-swebench")
			printEvalPlan(calculated)
			if authorizationErr != nil {
				fmt.Fprintln(os.Stderr, "eval-swebench authorization:", authorizationErr)
				return 2
			}
			if !authorized {
				return 0
			}
		} else {
			printEvalPlan(calculated)
		}
		if len(inputs) > 0 {
			start = time.Now()
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			batchResult, err = service.ExecuteBatch(ctx, batchPlan)
			cancel()
			if err != nil {
				fmt.Fprintln(os.Stderr, "eval-swebench:", err)
				return 1
			}
		}
	}

	summary := swebenchSummary{SchemaVersion: evalArtifactSchemaVersion, Runs: runs, DryRun: *dryRun, Plan: plan, CalibrationPolicy: verification.DefaultCalibrationPolicy()}
	if len(preparedTasks) > 0 {
		fidelity := aggregatePreparedEvidence(preparedTasks)
		summary.Fidelity = &fidelity
	}
	if *details {
		summary.Details = make([]swebenchTaskResult, 0, len(tasks))
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
		res := swebenchTaskResult{InstanceID: task.InstanceID}
		trialCount := len(task.Trials)
		summary.Tasks++
		summary.Trials += trialCount

		rewards := make([]int, trialCount)
		runNames := make([]string, trialCount)
		oracle := 0
		rewardSum := 0
		for i, tr := range task.Trials {
			rewards[i] = tr.Reward
			runNames[i] = tr.Run
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
		res.Runs = runNames
		res.Oracle = oracle
		if trialCount > 0 {
			res.PassAt1 = float64(rewardSum) / float64(trialCount)
		}
		summary.PassAt1Score += res.PassAt1
		summary.OracleScore += float64(oracle)

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
			res.Selection = &selection
			res.Usage = selection.Usage
			mode.AddUsage(&summary.Usage, selection.Usage)
			summary.Escalation = addEvalEscalation(summary.Escalation, selection)
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
		pairRows, rankRows := evalReliabilityRows(task.InstanceID, rewards, res.Selection, cfg.MaxPairCalls)
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
	summary.Baselines = baseline.Evaluate(baseline.SWEbench(), baselineTasks)
	if !*dryRun && plan != nil && plan.StatisticalDesign != nil {
		summary.Comparisons, err = buildObservedComparisons(summary.Baselines, *plan.StatisticalDesign)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eval-swebench paired analysis:", err)
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
		printSWEbenchText(summary)
	default:
		fmt.Fprintln(os.Stderr, "unknown --output:", *output)
		return 2
	}
	return 0
}

func loadSWEbenchTasks(root string, requestedRuns []string, limit int) ([]swebenchTask, []string, error) {
	runs, err := resolveSWEbenchRuns(root, requestedRuns)
	if err != nil {
		return nil, nil, err
	}
	runData := make(map[string]map[string]swebenchCacheItem, len(runs))
	idCounts := make(map[string]int)
	for _, run := range runs {
		items, err := loadSWEbenchRun(filepath.Join(root, run))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", run, err)
		}
		if len(items) == 0 {
			continue
		}
		byID := make(map[string]swebenchCacheItem, len(items))
		for _, item := range items {
			if item.InstanceID == "" {
				continue
			}
			byID[item.InstanceID] = item
		}
		runData[run] = byID
		for id := range byID {
			idCounts[id]++
		}
	}

	ids := make([]string, 0, len(idCounts))
	for id, count := range idCounts {
		if count >= 2 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	if len(ids) == 0 {
		return nil, nil, errors.New("no SWE-bench instances present in at least two runs")
	}

	tasks := make([]swebenchTask, 0, len(ids))
	for _, id := range ids {
		task := swebenchTask{InstanceID: id}
		for _, run := range runs {
			item, ok := runData[run][id]
			if !ok {
				continue
			}
			problem := extractSWEbenchProblem(item.Messages)
			if problem == "" {
				problem = "(Instance: " + id + ")"
			}
			trace := formatSWEbenchTrace(item.Messages)
			patch := strings.TrimSpace(item.OutputPatch)
			if patch == "" {
				patch = "(no patch produced)"
			}
			reward := 0
			if item.Reward == 1.0 {
				reward = 1
			}
			patchBytes, patchFiles, patchHunks := baseline.PatchFeatures(item.OutputPatch)
			fullTrace := trace + "\n\n--- Final Code Patch ---\n" + patch
			rawItem, err := json.Marshal(item)
			if err != nil {
				return nil, nil, fmt.Errorf("encode SWE-bench item %s/%s: %w", run, id, err)
			}
			task.Trials = append(task.Trials, swebenchTrial{
				Run:      run,
				Name:     run + "_" + id,
				Reward:   reward,
				Problem:  problem,
				Trace:    string(rawItem),
				Model:    item.ModelName,
				NumSteps: item.NumSteps,
				Baseline: baseline.Candidate{
					Steps:      item.NumSteps,
					TraceBytes: len(fullTrace),
					ErrorWords: baseline.CountErrorWords(item.Messages),
					PatchBytes: patchBytes,
					PatchFiles: patchFiles,
					PatchHunks: patchHunks,
				},
			})
		}
		if len(task.Trials) >= 2 {
			tasks = append(tasks, task)
		}
	}
	return tasks, runs, nil
}

func resolveSWEbenchRuns(root string, requested []string) ([]string, error) {
	if len(requested) > 0 {
		runs := append([]string(nil), requested...)
		sort.Strings(runs)
		return runs, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("trajectory data not found at %s; fetch it with eval/fetch-eval-data.sh (release asset, ~226 MB unpacked)", root)
		}
		return nil, err
	}
	runs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			runs = append(runs, entry.Name())
		}
	}
	sort.Strings(runs)
	if len(runs) == 0 {
		return nil, errors.New("no SWE-bench run directories found")
	}
	return runs, nil
}

func loadSWEbenchRun(runDir string) ([]swebenchCacheItem, error) {
	single := filepath.Join(runDir, "data_cache.json")
	if _, err := os.Stat(single); err == nil {
		return readSWEbenchItems(single)
	}
	matches, err := filepath.Glob(filepath.Join(runDir, "data_cache*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var items []swebenchCacheItem
	for _, path := range matches {
		part, err := readSWEbenchItems(path)
		if err != nil {
			return nil, err
		}
		items = append(items, part...)
	}
	return items, nil
}

func readSWEbenchItems(path string) ([]swebenchCacheItem, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []swebenchCacheItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func extractSWEbenchProblem(messages string) string {
	for _, msg := range parseSWEbenchMessages(messages) {
		if msg.role != "user" {
			continue
		}
		text := contentText(msg.content)
		pr := submatchTrim(swePRDescriptionRE, text)
		instructions := submatchTrim(sweInstructionsRE, text)
		parts := make([]string, 0, 2)
		if pr != "" {
			parts = append(parts, "<pr_description>\n"+pr+"\n</pr_description>")
		}
		if instructions != "" {
			parts = append(parts, "<instructions>\n"+instructions+"\n</instructions>")
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n")
		}
		return strings.TrimSpace(text)
	}
	return ""
}

func formatSWEbenchTrace(messages string) string {
	parts := []string{}
	step := 0
	for _, msg := range parseSWEbenchMessages(messages) {
		text := stripSWEbenchProblemBlocks(contentText(msg.content))
		switch msg.role {
		case "system", "user":
			if step == 0 {
				continue
			}
			if text != "" {
				parts = append(parts, "[Output]\n"+truncateSWEbenchText(text))
			}
		case "assistant":
			step++
			parts = append(parts, fmt.Sprintf("--- Agent Step %d ---", step))
			if text != "" {
				parts = append(parts, truncateSWEbenchText(text))
			}
			parts = append(parts, "")
		case "tool":
			if step > 0 && text != "" {
				parts = append(parts, "[Output]\n"+truncateSWEbenchText(text))
			}
		}
	}
	if len(parts) == 0 {
		return "(no trajectory data)"
	}
	return strings.Join(parts, "\n")
}

type swebenchMessage struct {
	role    string
	content any
}

func parseSWEbenchMessages(messages string) []swebenchMessage {
	var raw []map[string]any
	if err := json.Unmarshal([]byte(messages), &raw); err != nil {
		return nil
	}
	out := make([]swebenchMessage, 0, len(raw))
	for _, msg := range raw {
		role, _ := msg["role"].(string)
		out = append(out, swebenchMessage{role: role, content: msg["content"]})
	}
	return out
}

func contentText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			switch x := item.(type) {
			case string:
				parts = append(parts, x)
			case map[string]any:
				if typ, _ := x["type"].(string); typ != "" && typ != "text" {
					continue
				}
				if text, _ := x["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func stripSWEbenchProblemBlocks(text string) string {
	text = swePRDescriptionRE.ReplaceAllString(text, "")
	text = sweInstructionsRE.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func submatchTrim(re *regexp.Regexp, text string) string {
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func truncateSWEbenchText(text string) string {
	const max = 2000
	if len(text) <= max {
		return text
	}
	return text[:max] + "\n... (truncated)"
}

func printSWEbenchText(summary swebenchSummary) {
	fmt.Printf("SWE-bench Verified runs: %s\n", strings.Join(summary.Runs, ", "))
	fmt.Printf("Instances: %d  Trials: %d  Swing: %d  All-pass: %d  All-fail: %d\n",
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

func swebenchRewardRows(tasks []swebenchTask) [][]int {
	rows := make([][]int, len(tasks))
	for taskIndex, task := range tasks {
		rows[taskIndex] = make([]int, len(task.Trials))
		for trialIndex, trial := range task.Trials {
			rows[taskIndex][trialIndex] = trial.Reward
		}
	}
	return rows
}
