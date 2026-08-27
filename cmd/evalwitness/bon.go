package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	"golang.org/x/sync/errgroup"
)

// maxTranscriptBytes bounds captured agent output per attempt; importance
// truncation later fits it to the provider context.
const maxTranscriptBytes = 4 * 1024 * 1024
const maxAttemptDiffBytes = 64 * 1024 * 1024

type bonAttempt struct {
	Index               int     `json:"index"`
	ExitCode            int     `json:"exit_code"`
	TimedOut            bool    `json:"timed_out"`
	DiffPath            string  `json:"diff_path"`
	DiffDigest          string  `json:"diff_digest"`
	DiffEmpty           bool    `json:"diff_empty"`
	TranscriptPath      string  `json:"transcript_path"`
	TranscriptDigest    string  `json:"transcript_digest"`
	TranscriptTruncated bool    `json:"transcript_truncated"`
	TranscriptDropped   int64   `json:"transcript_dropped_bytes"`
	DurationS           float64 `json:"duration_seconds"`
	WorktreePath        string  `json:"worktree_path,omitempty"`
	Transcript          string  `json:"-"`
	Diff                string  `json:"-"`
}

type bonResult struct {
	SchemaVersion    string                    `json:"schema_version"`
	State            verifier.DecisionState    `json:"state"`
	AbstentionReason verifier.AbstentionReason `json:"abstention_reason,omitempty"`
	WinnerIndex      int                       `json:"winner_index"`
	Confidence       float64                   `json:"decision_strength"`
	Scores           []float64                 `json:"conditional_scores"`
	Wins             []float64                 `json:"wins"`
	Attempts         []bonAttempt              `json:"attempts"`
	WinnerDiff       string                    `json:"winner_diff_path"`
	Applied          bool                      `json:"applied"`
	PatchSummary     string                    `json:"patch_summary,omitempty"`
	RunDirectory     string                    `json:"run_directory"`
	Source           bonSourceSnapshot         `json:"source"`
	Destination      bonDestinationState       `json:"destination_before_attempts"`
	KeptWorktrees    []string                  `json:"kept_worktrees,omitempty"`
	Selection        *mode.Selection           `json:"selection,omitempty"`
}

func runBon(args []string) (exitCode int) {
	fs := flag.NewFlagSet("bon", flag.ExitOnError)
	n := fs.Int("n", 3, "number of attempts (2-10)")
	taskFlag := fs.String("task", "", "task description (inline or @file); required")
	criteriaFlag := fs.String("criteria", "specification,error_signals,code_quality", "comma-separated criterion ids for selection")
	nReps := fs.Int("n-reps", 0, "reps per pair/criterion (0 = config default)")
	parallel := fs.Bool("parallel", false, "run attempts concurrently (default: sequential)")
	keep := fs.Bool("keep", false, "keep attempt worktrees for inspection")
	apply := fs.Bool("apply", false, "apply the winning diff without staging after destination verification")
	includeWorkingTree := fs.Bool("include-working-tree", false, "snapshot tracked and allowed untracked source changes without modifying the index")
	passEnvironment := fs.String("pass-env", "", "comma-separated environment variable names explicitly passed to child agents")
	resumeRun := fs.String("resume-run", "", "resume an immutable live selection from a generated run.json manifest")
	timeoutSec := fs.Int("timeout-sec", 0, "per-attempt timeout in seconds (0 = none)")
	output := fs.String("output", "text", "json/text")
	liveFlags := addVerifyLiveFlags(fs)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `usage: evalwitness bon -n <N> --task <inline|@file> [flags] -- <agent command...>

Runs the agent command N times in isolated git worktrees of an exact clean
commit or an explicit working-tree snapshot, captures each attempt's transcript
and diff, and selects the best attempt with the pairwise verifier. Use sh -c
'...' as the command for shell pipelines.`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := validateVerifyLiveFlags(liveFlags); err != nil {
		fmt.Fprintln(os.Stderr, "bon:", err)
		return 2
	}
	if strings.TrimSpace(*resumeRun) != "" {
		if len(fs.Args()) != 0 {
			fmt.Fprintln(os.Stderr, "bon: --resume-run does not accept an agent command")
			return 2
		}
		return runBonResume(*resumeRun, *liveFlags.authorize, *apply, *output)
	}
	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "bon: agent command required after --")
		return 2
	}
	if *n < 2 || *n > 10 {
		fmt.Fprintln(os.Stderr, "bon: -n must be between 2 and 10")
		return 2
	}
	if *timeoutSec < 0 {
		fmt.Fprintln(os.Stderr, "bon: --timeout-sec must be >= 0")
		return 2
	}
	if *taskFlag == "" {
		fmt.Fprintln(os.Stderr, "bon: --task required (the verifier scores trajectories against it)")
		return 2
	}
	passedEnvironment := splitTrim(*passEnvironment)
	if err := validateBonPassEnvironment(passedEnvironment); err != nil {
		fmt.Fprintln(os.Stderr, "bon:", err)
		return 2
	}
	if err := validateBonCommandArguments(cmdArgs, passedEnvironment); err != nil {
		fmt.Fprintln(os.Stderr, "bon:", err)
		return 2
	}
	task, err := loadInline(*taskFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon: task:", err)
		return 2
	}

	repoRoot, err := gitTopLevel(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon: must run inside a git repository (worktrees provide attempt isolation):", err)
		return 2
	}

	crits, err := verifier.ResolveCriteria(splitTrim(*criteriaFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon: criteria:", err)
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon init:", err)
		return 1
	}
	reps := *nReps
	if reps <= 0 {
		reps = cfg.DefaultReps
	}
	runDir, err := os.MkdirTemp("", "evalwitness-bon-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon:", err)
		return 1
	}
	retainRunDirectory := false
	defer func() {
		if retainRunDirectory {
			return
		}
		if removeErr := os.Remove(runDir); removeErr != nil && !os.IsNotExist(removeErr) {
			fmt.Fprintln(os.Stderr, "bon preflight cleanup:", removeErr)
			exitCode = 1
		}
	}()
	sourceSnapshot, destinationState, err := prepareBonSource(repoRoot, runDir, *includeWorkingTree)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon source:", err)
		return 2
	}
	policy := verification.PolicyFromConfig(cfg, reps, cfg.Epsilon, cfg.BiasMitigation, cfg.SPRT.Enabled, "pairwise")
	service, err := verification.NewProductionService(cfg, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon authorization service:", err)
		return 1
	}
	maximumTrajectory := strings.Repeat("x", maxInt(1, cfg.EvidenceTokens)*4+1024)
	contractTrajectories := make([]string, *n)
	for index := range contractTrajectories {
		contractTrajectories[index] = maximumTrajectory
	}
	previewPlan, err := service.Plan(verification.Input{
		Entrypoint: "best-of-n", Mode: verification.ModePairwise, Task: task,
		Trajectories: contractTrajectories, Criteria: crits, Policy: policy,
		Limits: mode.BudgetLimits{
			MaxCalls: *liveFlags.maxCalls, MaxAttempts: *liveFlags.maxAttempts,
			MaxEstimatedInputTokens: *liveFlags.maxInputTokens, MaxReservedOutputTokens: *liveFlags.maxOutputTokens,
			MaxConcurrent: *liveFlags.maxConcurrent, MaxCostUSD: *liveFlags.maxCostUSD, MaxDuration: *liveFlags.maxDuration,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon authorization plan:", err)
		return 1
	}
	requestSet := previewPlan.Requests
	worstCalls := requestSet.WorstLogicalCalls
	limits := previewPlan.Input.Limits
	manifestFingerprint, err := bonManifestFingerprint(task, cmdArgs, *n, *parallel, *keep, *apply, *timeoutSec, crits, reps, sourceSnapshot, passedEnvironment)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon manifest:", err)
		return 1
	}
	authorization, err := mode.BuildAuthorizationPlan(mode.AuthorizationSpec{
		Entrypoint: "best-of-n", RouteID: requestSet.Requests[0].RouteID(),
		RequestFingerprint: manifestFingerprint, RequestContractDigest: requestSet.ContractDigest,
		Limits: limits, MaxRetries: cfg.MaxRetries, MaxWorkers: cfg.MaxWorkers,
		MaxOutputTokens: requestSet.MaximumOutputTokens, ExpectedCalls: worstCalls, WorstCalls: worstCalls,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon authorization:", err)
		return 2
	}
	offlineExecution := cfg.ReplayFrom != "" || cfg.Offline
	if !offlineExecution {
		if strings.TrimSpace(*liveFlags.authorize) == "" {
			printAuthorizationPlan(authorization)
			return 0
		}
		if err := authorization.Verify(*liveFlags.authorize); err != nil {
			fmt.Fprintln(os.Stderr, err)
			printAuthorizationPlan(authorization)
			return 2
		}
	}

	retainRunDirectory = true
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	attempts, err := runBonAttempts(ctx, bonRunSpec{
		RepoRoot:   repoRoot,
		RunDir:     runDir,
		Source:     sourceSnapshot.SnapshotCommit,
		N:          *n,
		CmdArgs:    cmdArgs,
		PassEnv:    passedEnvironment,
		Parallel:   *parallel,
		Keep:       *keep,
		TimeoutSec: *timeoutSec,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon:", err)
		return 1
	}
	trajectories := make([]string, len(attempts))
	for index, attempt := range attempts {
		trajectories[index] = attempt.trajectoryText(cmdArgs)
	}
	selectionInput := verification.Input{
		Entrypoint: "best-of-n", Mode: verification.ModePairwise, Task: task,
		Trajectories: trajectories, Criteria: crits, Policy: policy, Limits: limits,
		BudgetStatePath: cfg.RunBudgetState,
	}
	selectionPlan, err := service.Plan(selectionInput)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon selection plan:", err)
		return 1
	}
	manifest := bonRunManifest{
		RepositoryRoot: repoRoot, RunDirectory: runDir, Source: sourceSnapshot, Destination: destinationState,
		Task: task, Command: append([]string(nil), cmdArgs...), ApplyAuthorized: *apply,
		Criteria: append([]verifier.Criterion(nil), crits...),
		Policy:   policy, Limits: newBonBudgetLimits(limits), BudgetStatePath: cfg.RunBudgetState, Attempts: attempts,
		PlanFingerprint: selectionPlan.RunFingerprint, SelectionAuthorization: selectionPlan.Authorization,
	}
	manifestPath, err := writeBonManifest(manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon manifest:", err)
		return 1
	}
	if !offlineExecution {
		if selectionPlan.Authorization == nil {
			fmt.Fprintln(os.Stderr, "bon: live selection plan omitted authorization")
			return 1
		}
		printAuthorizationPlan(*selectionPlan.Authorization)
		fmt.Fprintf(os.Stderr, "resume exact selection: evalwitness bon --resume-run %s --authorize %s\n", manifestPath, selectionPlan.Authorization.AuthorizationDigest)
		return 0
	}
	runResult, err := service.Execute(ctx, selectionPlan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon selection:", err)
		return 1
	}
	if runResult.Selection == nil {
		fmt.Fprintln(os.Stderr, "bon selection: service returned no pairwise selection")
		return 1
	}
	return finishBon(repoRoot, runDir, sourceSnapshot, destinationState, attempts, *runResult.Selection, *keep, *apply, *output)
}

func bonManifestFingerprint(task string, command []string, attempts int, parallel, keep, apply bool, timeoutSeconds int, criteria []verifier.Criterion, reps int, source bonSourceSnapshot, passEnvironment []string) (provider.Fingerprint, error) {
	criterionIDs := make([]string, len(criteria))
	for index, criterion := range criteria {
		criterionIDs[index] = criterion.ID
	}
	payload := struct {
		Task            string            `json:"task"`
		Command         []string          `json:"command"`
		Attempts        int               `json:"attempts"`
		Parallel        bool              `json:"parallel"`
		Keep            bool              `json:"keep"`
		Apply           bool              `json:"apply"`
		Timeout         int               `json:"timeout_seconds"`
		Criteria        []string          `json:"criteria"`
		Reps            int               `json:"reps"`
		Source          bonSourceSnapshot `json:"source"`
		PassEnvironment []string          `json:"pass_environment"`
	}{task, append([]string(nil), command...), attempts, parallel, keep, apply, timeoutSeconds, criterionIDs, reps, source, append([]string(nil), passEnvironment...)}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return provider.Fingerprint(hex.EncodeToString(digest[:])), nil
}

type bonRunSpec struct {
	RepoRoot      string
	RunDir        string
	Source        string
	N             int
	CmdArgs       []string
	PassEnv       []string
	Parallel      bool
	Keep          bool
	TimeoutSec    int
	WriteArtifact func(string, []byte) error
}

func bonWorktreePath(runDir string, index int) string {
	return filepath.Join(runDir, fmt.Sprintf("attempt-%d", index))
}

// runBonAttempts creates one worktree per attempt, runs the agent command in
// it, captures transcript + diff, and removes the worktree unless Keep.
func runBonAttempts(ctx context.Context, spec bonRunSpec) ([]bonAttempt, error) {
	attempts := make([]bonAttempt, spec.N)
	runOne := func(i int) error {
		a, err := runBonAttempt(ctx, spec, i)
		if err != nil {
			return fmt.Errorf("attempt %d: %w", i, err)
		}
		attempts[i] = a
		return nil
	}
	if spec.Parallel {
		g, gctx := errgroup.WithContext(ctx)
		for i := 0; i < spec.N; i++ {
			i := i
			g.Go(func() error {
				a, err := runBonAttempt(gctx, spec, i)
				if err != nil {
					return fmt.Errorf("attempt %d: %w", i, err)
				}
				attempts[i] = a
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	} else {
		for i := 0; i < spec.N; i++ {
			if err := runOne(i); err != nil {
				return nil, err
			}
		}
	}
	return attempts, nil
}

func runBonAttempt(ctx context.Context, spec bonRunSpec, index int) (bonAttempt, error) {
	wt := bonWorktreePath(spec.RunDir, index)
	source := spec.Source
	if source == "" {
		source = "HEAD"
	}
	if out, err := gitRun(spec.RepoRoot, "worktree", "add", "--detach", wt, source); err != nil {
		return bonAttempt{}, fmt.Errorf("worktree add: %v: %s", err, out)
	}
	removeWorktree := func() error {
		if spec.Keep {
			return nil
		}
		output, err := gitRun(spec.RepoRoot, "worktree", "remove", "--force", wt)
		if err != nil {
			return fmt.Errorf("cleanup retained worktree %s: %w: %s", wt, err, output)
		}
		return nil
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if spec.TimeoutSec > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(spec.TimeoutSec)*time.Second)
		defer cancel()
	}
	start := time.Now()
	cmd := exec.CommandContext(runCtx, spec.CmdArgs[0], spec.CmdArgs[1:]...)
	cmd.Dir = wt
	environment, err := bonChildEnvironment(spec.PassEnv, index)
	if err != nil {
		return bonAttempt{}, errors.Join(err, removeWorktree())
	}
	cmd.Env = environment
	cmd.WaitDelay = 5 * time.Second
	configureBonProcessGroup(cmd)
	output := newBoundedTailWriter(maxTranscriptBytes)
	cmd.Stdout = output
	cmd.Stderr = output
	runErr := cmd.Run()
	exitCode, exitErr := processExitCode(runErr)
	if exitErr != nil {
		return bonAttempt{}, errors.Join(fmt.Errorf("run command: %w", exitErr), removeWorktree())
	}
	if err := ctx.Err(); err != nil {
		return bonAttempt{}, errors.Join(fmt.Errorf("parent cancelled attempt: %w", err), removeWorktree())
	}
	combined, dropped := output.Snapshot()
	combined = redactBonTranscript(combined, spec.PassEnv)
	if len(combined) > maxTranscriptBytes {
		dropped += int64(len(combined) - maxTranscriptBytes)
		combined = combined[len(combined)-maxTranscriptBytes:]
	}

	diff, err := diffBonWorkingTree(wt, source, spec.RunDir)
	if err != nil {
		return bonAttempt{}, errors.Join(err, removeWorktree())
	}

	diffPath := secureArtifactPath(spec.RunDir, fmt.Sprintf("attempt-%d.diff", index))
	writeArtifact := spec.WriteArtifact
	if writeArtifact == nil {
		writeArtifact = writeBonArtifact
	}
	if err := writeArtifact(diffPath, []byte(diff)); err != nil {
		return bonAttempt{}, errors.Join(err, removeWorktree())
	}
	transcriptPath := secureArtifactPath(spec.RunDir, fmt.Sprintf("attempt-%d.transcript", index))
	if err := writeArtifact(transcriptPath, combined); err != nil {
		return bonAttempt{}, errors.Join(err, removeWorktree())
	}
	if err := removeWorktree(); err != nil {
		return bonAttempt{}, err
	}

	return bonAttempt{
		Index: index, ExitCode: exitCode, TimedOut: errors.Is(runCtx.Err(), context.DeadlineExceeded),
		DiffPath: diffPath, DiffDigest: digestBytes([]byte(diff)), DiffEmpty: strings.TrimSpace(diff) == "",
		TranscriptPath: transcriptPath, TranscriptDigest: digestBytes(combined), TranscriptTruncated: dropped > 0,
		TranscriptDropped: dropped, DurationS: time.Since(start).Seconds(),
		WorktreePath: keptBonWorktree(spec.Keep, wt), Transcript: string(combined), Diff: diff,
	}, nil
}

func writeBonArtifact(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, safety.SensitiveFileMode)
	if err != nil {
		return fmt.Errorf("create Best-of-N artifact %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(fmt.Errorf("write Best-of-N artifact %s: %w", path, err), file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(fmt.Errorf("sync Best-of-N artifact %s: %w", path, err), file.Close())
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Best-of-N artifact %s: %w", path, err)
	}
	return nil
}

func keptBonWorktree(keep bool, path string) string {
	if keep {
		return path
	}
	return ""
}

// trajectoryText renders an attempt in the canonical step format so the
// preprocess importance scoring treats transcript and diff as steps.
func (a bonAttempt) trajectoryText(cmdArgs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- Agent Step 1 ---\n[Command] %s\n[Output]\n%s\n", strings.Join(cmdArgs, " "), a.Transcript)
	fmt.Fprintf(&b, "exit code: %d\n\n", a.ExitCode)
	b.WriteString("--- Agent Step 2 ---\n[Command] git diff (full attempt footprint)\n[Output]\n")
	if a.DiffEmpty {
		b.WriteString("(no changes)\n")
	} else {
		b.WriteString(a.Diff)
		b.WriteString("\n")
	}
	return b.String()
}

func gitTopLevel(dir string) (string, error) {
	out, err := gitRun(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runBonResume(manifestPath, authorizationDigest string, apply bool, output string) int {
	manifest, err := loadBonManifest(manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon resume:", err)
		return 2
	}
	repoRoot, err := gitTopLevel(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon resume:", err)
		return 2
	}
	if filepath.Clean(repoRoot) != filepath.Clean(manifest.RepositoryRoot) {
		fmt.Fprintf(os.Stderr, "bon resume: manifest repository %s does not match current repository %s\n", manifest.RepositoryRoot, repoRoot)
		return 2
	}
	if apply && !manifest.ApplyAuthorized {
		fmt.Fprintln(os.Stderr, "bon resume: --apply was not part of the authorized attempt manifest")
		return 2
	}
	currentDestination, err := captureBonDestination(repoRoot, manifest.RunDirectory)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon resume destination:", err)
		return 1
	}
	if currentDestination.Digest != manifest.Destination.Digest {
		fmt.Fprintf(os.Stderr, "bon resume: destination changed since attempts: expected %s, observed %s\n", manifest.Destination.Digest, currentDestination.Digest)
		return 1
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon resume init:", err)
		return 1
	}
	if cfg.RunBudgetState != manifest.BudgetStatePath {
		fmt.Fprintln(os.Stderr, "bon resume: persistent budget state path changed since artifact capture")
		return 2
	}
	service, err := verification.NewProductionService(cfg, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon resume init:", err)
		return 1
	}
	trajectories := make([]string, len(manifest.Attempts))
	for index, attempt := range manifest.Attempts {
		trajectories[index] = attempt.trajectoryText(manifest.Command)
	}
	plan, err := service.Plan(verification.Input{
		Entrypoint: "best-of-n", Mode: verification.ModePairwise, Task: manifest.Task,
		Trajectories: trajectories, Criteria: manifest.Criteria, Policy: manifest.Policy, Limits: manifest.Limits.modeLimits(),
		AuthorizationDigest: authorizationDigest, BudgetStatePath: manifest.BudgetStatePath,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon resume plan:", err)
		return 2
	}
	if plan.RunFingerprint != manifest.PlanFingerprint || !reflect.DeepEqual(plan.Authorization, manifest.SelectionAuthorization) {
		fmt.Fprintln(os.Stderr, "bon resume: configuration or exact selection plan changed since artifact capture")
		return 2
	}
	if plan.Authorization != nil && strings.TrimSpace(authorizationDigest) == "" {
		printAuthorizationPlan(*plan.Authorization)
		return 0
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := service.Execute(ctx, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bon resume selection:", err)
		return 1
	}
	if result.Selection == nil {
		fmt.Fprintln(os.Stderr, "bon resume selection: service returned no pairwise selection")
		return 1
	}
	return finishBon(repoRoot, manifest.RunDirectory, manifest.Source, manifest.Destination, manifest.Attempts, *result.Selection, hasKeptBonWorktree(manifest.Attempts), apply, output)
}

func finishBon(repoRoot, runDir string, source bonSourceSnapshot, destination bonDestinationState, attempts []bonAttempt, selection mode.Selection, keep, apply bool, output string) int {
	result := bonResult{
		SchemaVersion: "evalwitness.best-of-n.v3", State: selection.State, AbstentionReason: selection.AbstentionReason,
		WinnerIndex: selection.BestIndex, Confidence: selection.Confidence, Scores: selection.Scores, Wins: selection.Wins,
		Attempts: attempts, RunDirectory: runDir, Source: source, Destination: destination, Selection: &selection,
	}
	if selection.State == verifier.DecisionSelected && selection.BestIndex >= 0 && selection.BestIndex < len(attempts) {
		result.WinnerDiff = attempts[selection.BestIndex].DiffPath
	} else {
		result.WinnerIndex = -1
	}
	if keep {
		for _, attempt := range attempts {
			if attempt.WorktreePath != "" {
				result.KeptWorktrees = append(result.KeptWorktrees, attempt.WorktreePath)
			}
		}
	}
	if apply && result.WinnerIndex < 0 {
		fmt.Fprintln(os.Stderr, "bon: verifier abstained; refusing to apply an unselected attempt")
		return 1
	}
	if apply {
		patchSummary, err := applyBonDiff(repoRoot, runDir, destination, attempts[result.WinnerIndex])
		result.PatchSummary = patchSummary
		if err != nil {
			fmt.Fprintln(os.Stderr, "bon: apply:", err)
			return 1
		}
		fmt.Fprint(os.Stderr, patchSummary)
		result.Applied = true
	}
	return emitBonResult(result, output)
}

func hasKeptBonWorktree(attempts []bonAttempt) bool {
	for _, attempt := range attempts {
		if attempt.WorktreePath != "" {
			return true
		}
	}
	return false
}

func emitBonResult(result bonResult, output string) int {
	switch output {
	case "json":
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "bon output:", err)
			return 1
		}
		fmt.Println(string(encoded))
	case "text":
		printBonText(result)
	default:
		fmt.Fprintln(os.Stderr, "unknown --output:", output)
		return 2
	}
	return 0
}

func printBonText(res bonResult) {
	fmt.Printf("state: %s\n", res.State)
	if res.WinnerIndex >= 0 {
		fmt.Printf("winner: attempt %d (decision strength %.4f)\n", res.WinnerIndex, res.Confidence)
	} else {
		fmt.Printf("winner: none (%s)\n", res.AbstentionReason)
	}
	fmt.Printf("conditional_scores: %s\n", formatFloatSlice(res.Scores))
	fmt.Printf("wins: %s\n", formatFloatSlice(res.Wins))
	for _, a := range res.Attempts {
		marker := " "
		if res.WinnerIndex >= 0 && a.Index == res.WinnerIndex {
			marker = "*"
		}
		fmt.Printf("%s attempt %d: exit=%d timeout=%t diff=%s empty=%t truncated=%t %.1fs\n",
			marker, a.Index, a.ExitCode, a.TimedOut, a.DiffPath, a.DiffEmpty, a.TranscriptTruncated, a.DurationS)
	}
	if res.WinnerDiff != "" {
		fmt.Printf("winner diff: %s\n", res.WinnerDiff)
	}
	if res.Applied {
		fmt.Println("applied winning diff to working tree without staging")
	} else if res.WinnerIndex >= 0 && !res.Attempts[res.WinnerIndex].DiffEmpty {
		fmt.Printf("apply with: git apply %s\n", res.WinnerDiff)
	}
	for _, w := range res.KeptWorktrees {
		fmt.Printf("kept worktree: %s\n", w)
	}
	if res.Selection != nil {
		printUsage(res.Selection.Usage)
	}
}
