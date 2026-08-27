package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const defaultAttestationOutputTokens = 4096

type attestationFlags struct {
	mode             *string
	criteria         *string
	noCritique       *bool
	authorize        *string
	maxCalls         *int
	maxAttempts      *int
	maxInputTokens   *int
	maxOutputTokens  *int
	maxDuration      *time.Duration
	maxConcurrent    *int
	maxCostUSD       *float64
	freshness        *time.Duration
	checkpoint       *string
	checkpointSource *string
	task             *string
	trajectories     *stringSlice
}

func addAttestationFlags(flags *flag.FlagSet) attestationFlags {
	trajectories := new(stringSlice)
	flags.Var(trajectories, "trajectory", "qualification trajectory text or @file; repeat for production-shaped attestation")
	return attestationFlags{
		mode:             flags.String("mode", "pairwise", "qualification request contract: pairwise, absolute, or joint_absolute"),
		criteria:         flags.String("criteria", "code_review", "comma-separated criterion IDs shaping the qualification request"),
		noCritique:       flags.Bool("no-critique", false, "qualify the no-critique prompt contract"),
		authorize:        flags.String("authorize", "", "execute only when this digest matches the printed plan"),
		maxCalls:         flags.Int("max-calls", 1, "hard logical-call limit"),
		maxAttempts:      flags.Int("max-attempts", 0, "hard HTTP-attempt limit (0 derives retries + initial attempt)"),
		maxInputTokens:   flags.Int("max-input-tokens", 0, "hard reserved input-token limit (0 derives from the qualification request)"),
		maxOutputTokens:  flags.Int("max-output-tokens", 0, "hard reserved output-token limit (0 derives from the request ceiling and attempts)"),
		maxDuration:      flags.Duration("max-duration", 0, "hard total deadline (0 derives from provider timeout and attempts)"),
		maxConcurrent:    flags.Int("max-concurrent", 1, "hard concurrent-attempt limit"),
		maxCostUSD:       flags.Float64("max-cost-usd", 0, "optional hard monetary limit; zero is valid for subscription or unknown pricing"),
		freshness:        flags.Duration("freshness", 24*time.Hour, "attestation freshness window"),
		checkpoint:       flags.String("checkpoint-assertion", "", "operator checkpoint assertion, kept separate from served model identity"),
		checkpointSource: flags.String("checkpoint-source", "", "source for --checkpoint-assertion"),
		task:             flags.String("task", "", "qualification task text or @file; requires --trajectory"),
		trajectories:     trajectories,
	}
}

func runAttest(args []string) (exitCode int) {
	flags := flag.NewFlagSet("attest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	liveFlags := addAttestationFlags(flags)
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "attest:", err)
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	criteria, err := verifier.ResolveCriteria(splitTrim(*liveFlags.criteria))
	if err != nil {
		fmt.Fprintln(os.Stderr, "attest criteria:", err)
		return 2
	}
	request, err := qualificationRequestFromInputs(
		cfg, *liveFlags.mode, criteria, !*liveFlags.noCritique && cfg.CritiqueThenScore,
		*liveFlags.task, *liveFlags.trajectories,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "attest request:", err)
		return 1
	}
	limits := resolveAttestationLimits(cfg, request, liveFlags)
	requestFingerprint, err := request.Fingerprint()
	if err != nil {
		fmt.Fprintln(os.Stderr, "attest fingerprint:", err)
		return 1
	}
	plan, err := mode.BuildAuthorizationPlan(mode.AuthorizationSpec{
		Entrypoint: "cli.attest", RouteID: request.RouteID(), Limits: limits,
		RequestFingerprint: requestFingerprint, RequestContractDigest: conformance.CapabilityContractDigest(request),
		MaxRetries: cfg.MaxRetries, MaxWorkers: 1, MaxOutputTokens: request.MaxOutputTokens,
		ExpectedCalls: 1, WorstCalls: 1,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "attest authorization:", err)
		return 2
	}
	if strings.TrimSpace(*liveFlags.authorize) == "" {
		printAuthorizationPlan(plan)
		return 0
	}
	if err := plan.Verify(*liveFlags.authorize); err != nil {
		fmt.Fprintln(os.Stderr, err)
		printAuthorizationPlan(plan)
		return 2
	}
	if (*liveFlags.checkpoint == "") != (*liveFlags.checkpointSource == "") {
		fmt.Fprintln(os.Stderr, "attest: checkpoint assertion and source must be supplied together")
		return 2
	}

	runtimeBundle, err := loadRuntime(runtimeLoadOptions{LiveIntent: true, Qualification: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	defer closeRuntime(runtimeBundle, &exitCode)
	budget, err := newAttestationBudget(limits, cfg.RunBudgetState)
	if err != nil {
		fmt.Fprintln(os.Stderr, "attest budget:", err)
		return 1
	}
	if err := budget.ReserveCall(); err != nil {
		fmt.Fprintln(os.Stderr, "attest budget:", err)
		return 1
	}
	started := time.Now().UTC()
	ctx := context.Background()
	if deadline, ok := budget.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	bindAttemptBudget(&request, budget, ctx, runtimeBundle.Cost)
	response, err := runtimeBundle.Provider.Score(ctx, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "attest provider:", err)
		return 1
	}
	if *liveFlags.checkpoint != "" {
		response.CheckpointAssertion = strings.TrimSpace(*liveFlags.checkpoint)
		response, err = provider.FinalizeResponse(request, response)
		if err != nil {
			fmt.Fprintln(os.Stderr, "attest checkpoint assertion:", err)
			return 1
		}
	}
	evidence := verifier.ExtractAllScoreEvidence(request, response, verifier.ExtractionModeVerifier)
	observedAt := time.Now().UTC()
	attestation, qualificationErr := conformance.EvaluateBounded(request, response, evidence, conformance.QualificationContext{
		ObservedAt: observedAt, ExpiresAt: observedAt.Add(*liveFlags.freshness), Latency: observedAt.Sub(started),
		HTTPAttempts: budget.Snapshot().Attempts, StreamingObserved: request.Stream,
		UsageObserved:             response.UsageObserved,
		CheckpointAssertionSource: strings.TrimSpace(*liveFlags.checkpointSource), Build: currentBuildIdentity(),
	})
	store, storeErr := conformance.OpenStore(cfg.CacheDir)
	if storeErr == nil {
		storeErr = store.Save(attestation)
	}
	if storeErr != nil {
		fmt.Fprintln(os.Stderr, "attest store:", storeErr)
		return 1
	}
	encoded, _ := json.MarshalIndent(attestation, "", "  ")
	fmt.Println(string(encoded))
	if qualificationErr != nil {
		fmt.Fprintln(os.Stderr, "attest qualification:", qualificationErr)
		return 1
	}
	return 0
}

func defaultQualificationRequest(cfg config.Config) (provider.RequestEnvelope, error) {
	return qualificationRequest(cfg, "pairwise", []verifier.Criterion{verifier.BuiltinCriteria["code_review"]}, cfg.CritiqueThenScore)
}

func qualificationRequest(cfg config.Config, modeName string, criteria []verifier.Criterion, critique bool) (provider.RequestEnvelope, error) {
	if len(criteria) == 0 {
		return provider.RequestEnvelope{}, errors.New("qualification requires at least one criterion")
	}
	task := "Repair a deterministic defect and prove the requested behavior."
	successful := "The agent edited the responsible code path and the final targeted test passed."
	failed := "The agent described a possible fix but left the failing test unresolved."
	options := verifier.PromptOptions{CritiqueThenScore: critique}
	var prompt string
	var tags []string
	switch modeName {
	case "pairwise", "delta":
		if cfg.MultiCriterionBundle && len(criteria) > 1 {
			var criterionTags []verifier.CriterionTags
			prompt, criterionTags = verifier.PromptPairwiseBundled(task, successful, failed, criteria, options)
			tags = verifier.AllTags(criterionTags)
		} else {
			prompt, tags = verifier.PromptPairwise(task, successful, failed, criteria[0], options)
		}
	case "absolute":
		if cfg.MultiCriterionBundle && len(criteria) > 1 {
			var criterionTags []verifier.CriterionTags
			prompt, criterionTags = verifier.PromptAbsoluteBundled(task, successful, criteria, options)
			tags = verifier.AllTags(criterionTags)
		} else {
			prompt, tags = verifier.PromptAbsolute(task, successful, criteria[0], options)
		}
	case "joint_absolute":
		trajectories := []string{
			successful,
			failed,
			"The agent repaired the code but did not run the relevant test.",
			"The agent ran diagnostics and proved the reported behavior was already correct.",
			"The agent changed an unrelated file and claimed success without evidence.",
		}
		var criterionTags []verifier.CriterionTags
		prompt, criterionTags = verifier.PromptJointAbsolute(task, trajectories, criteria, options)
		tags = verifier.AllTags(criterionTags)
	default:
		return provider.RequestEnvelope{}, fmt.Errorf("unknown qualification mode %q", modeName)
	}
	maxOutputTokens := cfg.MaxTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultAttestationOutputTokens
	}
	return provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID: cfg.Provider, BaseURL: cfg.BaseURL, RequestedModel: cfg.Model, ThinkingMode: cfg.ThinkingMode,
		Messages: []provider.Message{{Role: "user", Content: prompt}}, Temperature: cfg.Temperature,
		Seed: cfg.Seed, MaxOutputTokens: maxOutputTokens, Logprobs: true, TopLogprobs: verifier.MinimumVerifierTopK,
		ScoreTags: tags, ResponseFormat: provider.ResponseFormatText, Stream: cfg.Stream,
		Lineage: provider.RequestLineage{CriterionID: "bounded-qualification", SamplingSlot: "bounded-qualification", Entrypoint: "cli.attest"},
	})
}

func qualificationRequestFromInputs(
	cfg config.Config,
	modeName string,
	criteria []verifier.Criterion,
	critique bool,
	taskInput string,
	trajectoryInputs []string,
) (provider.RequestEnvelope, error) {
	customTask := strings.TrimSpace(taskInput) != ""
	customTrajectories := len(trajectoryInputs) > 0
	if !customTask && !customTrajectories {
		return qualificationRequest(cfg, modeName, criteria, critique)
	}
	if !customTask || !customTrajectories {
		return provider.RequestEnvelope{}, errors.New("production-shaped qualification requires --task and at least one --trajectory together")
	}
	task, err := loadInline(taskInput)
	if err != nil {
		return provider.RequestEnvelope{}, fmt.Errorf("load qualification task: %w", err)
	}
	trajectories := make([]string, len(trajectoryInputs))
	for index, trajectoryInput := range trajectoryInputs {
		trajectories[index], err = loadInline(trajectoryInput)
		if err != nil {
			return provider.RequestEnvelope{}, fmt.Errorf("load qualification trajectory %d: %w", index, err)
		}
	}
	return productionQualificationRequest(cfg, modeName, criteria, critique, task, trajectories)
}

func productionQualificationRequest(
	cfg config.Config,
	modeName string,
	criteria []verifier.Criterion,
	critique bool,
	task string,
	trajectories []string,
) (provider.RequestEnvelope, error) {
	inputMode := verification.ModeDelta
	selectionStrategy := "pairwise"
	biasMitigation := "single"
	switch modeName {
	case "pairwise", "delta":
	case "absolute":
		inputMode = verification.ModeAbsolute
	case "joint_absolute":
		inputMode = verification.ModePairwise
		selectionStrategy = "joint_absolute"
		biasMitigation = cfg.BiasMitigation
	default:
		return provider.RequestEnvelope{}, fmt.Errorf("unknown qualification mode %q", modeName)
	}
	cfg.CritiqueThenScore = critique
	service, err := verification.NewProductionService(cfg, version)
	if err != nil {
		return provider.RequestEnvelope{}, fmt.Errorf("initialize production qualification: %w", err)
	}
	plan, err := service.Plan(verification.Input{
		Entrypoint:   "cli.attest",
		Mode:         inputMode,
		Task:         task,
		Trajectories: trajectories,
		Criteria:     criteria,
		Policy: verification.PolicyFromConfig(
			cfg, 1, cfg.Epsilon, biasMitigation, false, selectionStrategy,
		),
	})
	if err != nil {
		return provider.RequestEnvelope{}, fmt.Errorf("plan production qualification: %w", err)
	}
	if len(plan.Requests.Requests) != 1 {
		return provider.RequestEnvelope{}, fmt.Errorf("production qualification planned %d requests; require exactly 1", len(plan.Requests.Requests))
	}
	return plan.Requests.Requests[0], nil
}

func resolveAttestationLimits(cfg config.Config, request provider.RequestEnvelope, flags attestationFlags) mode.BudgetLimits {
	attempts := *flags.maxAttempts
	if attempts <= 0 {
		attempts = cfg.MaxRetries + 1
	}
	inputTokens := *flags.maxInputTokens
	if inputTokens <= 0 {
		inputTokens = preprocess.EstimateTokens(request.Messages[0].Content) * attempts
	}
	outputTokens := *flags.maxOutputTokens
	if outputTokens <= 0 {
		outputTokens = request.MaxOutputTokens * attempts
	}
	duration := *flags.maxDuration
	if duration <= 0 {
		duration = time.Duration(cfg.TimeoutSec*attempts) * time.Second
	}
	return mode.BudgetLimits{
		MaxCalls: *flags.maxCalls, MaxAttempts: attempts, MaxEstimatedInputTokens: inputTokens,
		MaxReservedOutputTokens: outputTokens, MaxConcurrent: *flags.maxConcurrent,
		MaxCostUSD: *flags.maxCostUSD, MaxDuration: duration,
	}
}

func newAttestationBudget(limits mode.BudgetLimits, statePath string) (*mode.RunBudget, error) {
	if statePath == "" {
		return mode.NewRunBudget(limits), nil
	}
	return mode.NewPersistentRunBudget(limits, statePath)
}

func bindAttemptBudget(request *provider.RequestEnvelope, budget *mode.RunBudget, ctx context.Context, calculator *cost.Calculator) {
	inputTokens := preprocess.EstimateTokens(request.Messages[0].Content)
	estimatedCost := 0.0
	if estimate := calculator.EstimatePromptCost(inputTokens, request.MaxOutputTokens); estimate != nil {
		estimatedCost = *estimate
	}
	var releaseAttempt func(mode.AttemptActual) error
	request.BeforeAttempt = func() error {
		var reserveErr error
		releaseAttempt, reserveErr = budget.AcquireAttempt(ctx, mode.AttemptReservation{
			EstimatedInputTokens: inputTokens, MaxOutputTokens: request.MaxOutputTokens, EstimatedCostUSD: estimatedCost,
		})
		return reserveErr
	}
	request.AfterAttempt = func(result provider.AttemptResult) error {
		if releaseAttempt != nil {
			actual := mode.AttemptActual{
				InputTokens:   result.Usage.Input,
				OutputTokens:  result.Usage.Output,
				UsageObserved: result.UsageObserved,
			}
			if result.UsageObserved && calculator != nil {
				actual.CostUSD = calculator.Estimate(result.Usage.Input, result.Usage.Output, result.Usage.Cached)
			}
			releaseErr := releaseAttempt(actual)
			releaseAttempt = nil
			return releaseErr
		}
		return nil
	}
}

func resolveProbeLimits(cfg config.Config, request provider.RequestEnvelope, maxCalls, maxAttempts, maxInputTokens, maxOutputTokens int, maxDuration time.Duration, maxConcurrent int, maxCostUSD float64) mode.BudgetLimits {
	if maxAttempts <= 0 {
		maxAttempts = cfg.MaxRetries + 2
	}
	if maxInputTokens <= 0 {
		maxInputTokens = preprocess.EstimateTokens(request.Messages[0].Content) * maxAttempts
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = request.MaxOutputTokens * maxAttempts
	}
	if maxDuration <= 0 {
		maxDuration = time.Duration(cfg.TimeoutSec*maxAttempts) * time.Second
	}
	return mode.BudgetLimits{
		MaxCalls: maxCalls, MaxAttempts: maxAttempts, MaxEstimatedInputTokens: maxInputTokens,
		MaxReservedOutputTokens: maxOutputTokens, MaxConcurrent: maxConcurrent,
		MaxCostUSD: maxCostUSD, MaxDuration: maxDuration,
	}
}

func buildProbeAuthorization(cfg config.Config, request provider.RequestEnvelope, fingerprint provider.Fingerprint, limits mode.BudgetLimits, entrypoint string) (mode.AuthorizationPlan, error) {
	return mode.BuildAuthorizationPlan(mode.AuthorizationSpec{
		Entrypoint: entrypoint, RouteID: request.RouteID(), RequestFingerprint: fingerprint,
		RequestContractDigest: conformance.CapabilityContractDigest(request), Limits: limits,
		MaxRetries: cfg.MaxRetries, MaxWorkers: 1, MaxOutputTokens: request.MaxOutputTokens,
		ExpectedCalls: 1, WorstCalls: 1,
	})
}

func executeProbe(cfg config.Config, request provider.RequestEnvelope, limits mode.BudgetLimits) (provider.Capabilities, error) {
	runtimeBundle, err := loadRuntime(runtimeLoadOptions{LiveIntent: true})
	if err != nil {
		return provider.Capabilities{}, err
	}
	budget, err := newAttestationBudget(limits, cfg.RunBudgetState)
	if err != nil {
		return provider.Capabilities{}, errors.Join(err, runtimeBundle.Close())
	}
	if err := budget.ReserveCall(); err != nil {
		return provider.Capabilities{}, errors.Join(err, runtimeBundle.Close())
	}
	ctx := context.Background()
	if deadline, ok := budget.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	bindAttemptBudget(&request, budget, ctx, runtimeBundle.Cost)
	caps, probeErr := provider.ProbeWithRequest(ctx, runtimeBundle.Provider, cfg.CacheDir, request)
	return caps, errors.Join(probeErr, runtimeBundle.Close())
}

func printAuthorizationPlan(plan mode.AuthorizationPlan) {
	encoded, _ := json.MarshalIndent(plan, "", "  ")
	fmt.Println(string(encoded))
}

func currentBuildIdentity() conformance.BuildIdentity {
	identity := conformance.BuildIdentity{Commit: "unversioned-build"}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if setting.Value != "" {
					identity.Commit = setting.Value
				}
			case "vcs.modified":
				identity.Dirty = setting.Value == "true"
			}
		}
	}
	if executable, err := os.Executable(); err == nil {
		if file, openErr := os.Open(executable); openErr == nil {
			digest := sha256.New()
			if _, copyErr := io.Copy(digest, file); copyErr == nil {
				identity.BinarySHA256 = hex.EncodeToString(digest.Sum(nil))
			}
			_ = file.Close()
		}
	}
	return identity
}
