package verification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/cache"
	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	logging "github.com/Christopher-Schulze/evalwitness/internal/log"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func NewProductionService(cfg config.Config, version string) (*Service, error) {
	logging.Init(cfg.LogLevel)
	logging.RegisterSecret(cfg.APIKey)
	if cfg.RedactPatternsFile != "" {
		if err := preprocess.LoadCustomPatterns(cfg.RedactPatternsFile); err != nil {
			return nil, fmt.Errorf("load verification redaction patterns: %w", err)
		}
	}
	maxOutputTokens := cfg.MaxTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = mode.DefaultMaxTokens
	}
	serviceConfig := Config{
		Redact:           !cfg.NoRedact,
		PreprocessBudget: boundedPreprocessBudget(cfg.ContextLimit, maxOutputTokens, cfg.EvidenceTokens),
		RequestProfile: RequestProfile{
			ProviderID: cfg.Provider, BaseURL: cfg.BaseURL, RequestedModel: cfg.Model,
			ThinkingMode: cfg.ThinkingMode, Temperature: effectiveTemperature(cfg.Temperature), Seed: cfg.Seed,
			MaxOutputTokens: maxOutputTokens, TopLogprobs: verifier.MinimumVerifierTopK, Stream: cfg.Stream,
			MultiCriterionBundle: cfg.MultiCriterionBundle, CritiqueThenScore: cfg.CritiqueThenScore,
		},
		BudgetProfile: BudgetProfile{
			MaxRetries: cfg.MaxRetries, MaxWorkers: cfg.MaxWorkers, RequestTimeout: time.Duration(cfg.TimeoutSec) * time.Second,
			InputUSDPerM: cfg.InputUSDPerM, CachedUSDPerM: cfg.CachedUSDPerM, OutputUSDPerM: cfg.OutputUSDPerM,
			Subscription: cfg.BillingModel == "subscription",
		},
		Offline: cfg.ReplayFrom != "" || cfg.Offline,
	}
	runtimeSlots := make(chan struct{}, cfg.MaxWorkers)
	return NewService(serviceConfig, func(ctx context.Context, plan Plan) (Runtime, error) {
		select {
		case runtimeSlots <- struct{}{}:
		case <-ctx.Done():
			return Runtime{}, ctx.Err()
		}
		runtime, err := openProductionRuntime(ctx, cfg, version, plan)
		if err != nil {
			<-runtimeSlots
			return Runtime{}, err
		}
		closeRuntime := runtime.Close
		runtime.Close = func() error {
			defer func() { <-runtimeSlots }()
			if closeRuntime == nil {
				return nil
			}
			return closeRuntime()
		}
		return runtime, nil
	})
}

func PolicyFromConfig(cfg config.Config, reps int, epsilon float64, biasMitigation string, useSPRT bool, selectionStrategy string) Policy {
	evidence := EvidenceStrictVerifier
	if cfg.JudgeMode {
		evidence = EvidenceExplicitJudge
	}
	if selectionStrategy == "" {
		selectionStrategy = "pairwise"
	}
	return Policy{
		Evidence: evidence, NReps: reps, Epsilon: epsilon, BiasMitigation: biasMitigation,
		InconsistencyPolicy: cfg.InconsistencyPolicy, SelectionStrategy: selectionStrategy,
		SingleElim: cfg.SingleElim, UseSPRT: useSPRT,
		SPRTParams: SPRTParameters{
			Alpha: cfg.SPRT.Alpha, Beta: cfg.SPRT.Beta, Sigma: cfg.SPRT.Sigma,
			Epsilon: 0.05, MinReps: cfg.SPRT.MinReps, MaxReps: cfg.SPRT.MaxReps,
		},
		MaxWorkers: cfg.MaxWorkers, MinDispatchIntervalSeconds: cfg.MinDispatchIntervalSec, MaxPairCalls: cfg.MaxPairCalls,
		ConfidenceThreshold: cfg.PairConfidence, CalibrationSigma: cfg.PairCalibrationSigma,
		ConfidenceEscalation: mode.ConfidenceEscalationDisabled,
	}
}

func openProductionRuntime(ctx context.Context, cfg config.Config, version string, plan Plan) (Runtime, error) {
	if err := ctx.Err(); err != nil {
		return Runtime{}, err
	}
	judgePolicy := plan.Input.Policy.Evidence == EvidenceExplicitJudge
	if cfg.JudgeMode != judgePolicy {
		return Runtime{}, fmt.Errorf("configured judge mode %t does not match run evidence policy %q", cfg.JudgeMode, plan.Input.Policy.Evidence)
	}
	offline := cfg.ReplayFrom != "" || cfg.Offline
	if !offline && !judgePolicy {
		if err := requireAttestations(cfg, plan.Requests); err != nil {
			return Runtime{}, err
		}
	}
	caps, _, cachedCaps := provider.LoadCachedCapsWithLegacy(cfg.CacheDir, cfg.LegacyCacheDir, cfg.Provider, cfg.Model)
	contextLimit := cfg.ContextLimit
	if contextLimit <= 0 {
		contextLimit = provider.DefaultContextLimit
	}
	if !cachedCaps {
		caps = provider.Capabilities{
			Logprobs: true, TopLogprobsMax: verifier.MinimumVerifierTopK,
			MaxConcurrent: cfg.MaxWorkers, ContextLimit: contextLimit,
		}
	} else if cfg.ContextLimit > 0 {
		caps.ContextLimit = contextLimit
	}
	caps.Streaming = cfg.Stream
	if !judgePolicy && (!caps.Logprobs || caps.TopLogprobsMax < verifier.MinimumVerifierTopK) {
		return Runtime{}, fmt.Errorf("strict verifier route exposes Top-%d logprobs; require Top-%d", caps.TopLogprobsMax, verifier.MinimumVerifierTopK)
	}
	providerConfig := provider.Config{
		Name: cfg.Provider, WireFormat: cfg.WireFormat, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey,
		CAFile: cfg.CAFile, Model: cfg.Model, Timeout: cfg.TimeoutSec, Caps: caps,
		UpstreamProvider: cfg.UpstreamProvider,
		Thinking:         cfg.ThinkingMode, Offline: cfg.Offline, LiveIntent: !offline,
		MaxRetries: cfg.MaxRetries, UserAgent: "evalwitness/" + version,
	}
	providerInstance, err := productionProvider(cfg, providerConfig, plan)
	if err != nil {
		return Runtime{}, err
	}
	auditLogger, err := audit.New(cfg.AuditLog)
	if err != nil {
		closeErr := closeProvider(providerInstance)
		return Runtime{}, errors.Join(fmt.Errorf("open audit log: %w", err), closeErr)
	}
	calculator := cost.New(cfg.InputUSDPerM, cfg.CachedUSDPerM, cfg.OutputUSDPerM, cfg.BillingModel == "subscription")
	runBudget, err := productionBudget(plan.Input.Limits, plan.Input.BudgetStatePath)
	if err != nil {
		return Runtime{}, errors.Join(err, closeProductionResources(providerInstance, auditLogger))
	}
	cacheInstance := cache.NewWithLegacyImport(cfg.CacheDir, cfg.LegacyCacheDir, !cfg.NoCache && !plan.Input.DisableCache)
	runner := &mode.Runner{
		Provider: providerInstance, Cache: cacheInstance, Cost: calculator, Audit: auditLogger, Budget: runBudget,
		Cfg: mode.RunnerConfig{
			Model: cfg.Model, BaseURL: cfg.BaseURL, ThinkingMode: cfg.ThinkingMode, Stream: cfg.Stream,
			Entrypoint: plan.Input.Entrypoint, NoRedact: cfg.NoRedact,
			PreprocessBudget:  boundedPreprocessBudget(cfg.ContextLimit, maximumOutputTokens(cfg), cfg.EvidenceTokens),
			CritiqueThenScore: cfg.CritiqueThenScore, MultiCriterionBundle: cfg.MultiCriterionBundle,
			Temperature: cfg.Temperature, MaxTokens: cfg.MaxTokens, TopLogprobs: verifier.MinimumVerifierTopK,
			MaxCostPerCall: cfg.MaxCostUSDPerCall, MaxWorkers: plan.Input.Policy.MaxWorkers,
			Seed: cfg.Seed, RequireLogprobs: !judgePolicy, JudgeMode: judgePolicy,
		},
	}
	runner.Cfg.Lineage, err = baseRequestLineage(plan.Input)
	if err != nil {
		return Runtime{}, errors.Join(err, closeProductionResources(providerInstance, auditLogger))
	}
	return Runtime{Runner: runner, Close: func() error { return closeProvider(providerInstance) }, CloseAudit: auditLogger.Close}, nil
}

func productionProvider(cfg config.Config, providerConfig provider.Config, plan Plan) (provider.Provider, error) {
	if cfg.ReplayFrom != "" {
		instance, err := replay.LoadReplay(cfg.ReplayFrom, cfg.Provider, cfg.Model, providerConfig.Caps)
		if err != nil {
			return nil, fmt.Errorf("load replay: %w", err)
		}
		return instance, nil
	}
	instance, err := provider.New(providerConfig)
	if err != nil {
		return nil, err
	}
	if cfg.ReplayTo == "" {
		return instance, nil
	}
	metadata, err := researchCaptureMetadata(cfg, plan)
	if err != nil {
		return nil, errors.Join(err, closeProvider(instance))
	}
	capture, err := replay.WrapResearchCapture(instance, cfg.Model, cfg.ReplayTo, cfg.ReplayOverwrite, metadata)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open replay capture: %w", err), closeProvider(instance))
	}
	return capture, nil
}

func researchCaptureMetadata(cfg config.Config, plan Plan) (replay.CaptureMetadata, error) {
	if strings.TrimSpace(plan.Input.StudyManifestDigest) == "" {
		return replay.CaptureMetadata{}, errors.New("research capture requires a study manifest digest")
	}
	if len(plan.Requests.Requests) == 0 {
		return replay.CaptureMetadata{}, errors.New("research capture requires at least one request")
	}
	first := plan.Requests.Requests[0]
	for _, request := range plan.Requests.Requests {
		if err := request.Lineage.ValidateResearch(); err != nil {
			return replay.CaptureMetadata{}, fmt.Errorf("research capture request lineage: %w", err)
		}
		if conformance.RouteConfigDigest(request) != conformance.RouteConfigDigest(first) ||
			conformance.CapabilityContractDigest(request) != conformance.CapabilityContractDigest(first) {
			return replay.CaptureMetadata{}, errors.New("research capture requires one attested route and request contract")
		}
	}
	store, err := conformance.OpenExistingStore(cfg.CacheDir)
	if err != nil {
		return replay.CaptureMetadata{}, fmt.Errorf("open research capture attestation store: %w", err)
	}
	attestation, state, reason, err := store.Load(
		conformance.RouteConfigDigest(first), conformance.CapabilityContractDigest(first), time.Now().UTC(), plan.Input.StudyManifestDigest,
	)
	if err != nil {
		return replay.CaptureMetadata{}, fmt.Errorf("load research capture attestation: %w", err)
	}
	if state != conformance.StateBoundedQualified && state != conformance.StateStudyQualified {
		return replay.CaptureMetadata{}, fmt.Errorf("research capture attestation state is %s (%s)", state, reason)
	}
	if err := provider.MatchServedIdentity(plan.Input.ServedIdentityPolicy, plan.Input.ExpectedServedModel, plan.Input.ExpectedServedModels, attestation.Identity.ServedModel); err != nil {
		return replay.CaptureMetadata{}, fmt.Errorf("research capture attestation served identity: %w", err)
	}
	return replay.CaptureMetadata{
		CapabilityAttestationID: attestation.AttestationID,
		ServedIdentityPolicy:    plan.Input.ServedIdentityPolicy,
		ExpectedServedModel:     plan.Input.ExpectedServedModel,
		ExpectedServedModels:    append([]string(nil), plan.Input.ExpectedServedModels...),
		CheckpointAssertion:     attestation.Identity.CheckpointAssertion,
	}, nil
}

func productionBudget(limits mode.BudgetLimits, statePath string) (*mode.RunBudget, error) {
	if statePath == "" {
		return mode.NewRunBudget(limits), nil
	}
	budget, err := mode.NewPersistentRunBudget(limits, statePath)
	if err != nil {
		return nil, fmt.Errorf("open persistent verification budget: %w", err)
	}
	return budget, nil
}

func requireAttestations(cfg config.Config, requests RequestPlan) error {
	store, err := conformance.OpenExistingStore(cfg.CacheDir)
	if err != nil {
		return errors.New("no attestation store; run evalwitness attest for this request contract")
	}
	seen := make(map[string]struct{}, len(requests.Requests))
	for _, request := range requests.Requests {
		contractDigest := conformance.CapabilityContractDigest(request)
		if _, ok := seen[contractDigest]; ok {
			continue
		}
		seen[contractDigest] = struct{}{}
		_, state, reason, loadErr := store.Load(
			conformance.RouteConfigDigest(request), contractDigest, time.Now().UTC(), "",
		)
		if loadErr != nil {
			return fmt.Errorf("contract %s is not attested: %w", contractDigest, loadErr)
		}
		if state != conformance.StateBoundedQualified && state != conformance.StateStudyQualified {
			return fmt.Errorf("contract %s attestation state is %s (%s)", contractDigest, state, reason)
		}
	}
	return nil
}

func closeProductionResources(providerInstance provider.Provider, auditLogger *audit.Logger) error {
	return errors.Join(closeProvider(providerInstance), auditLogger.Close())
}

func closeProvider(instance provider.Provider) error {
	if closer, ok := instance.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("close provider: %w", err)
		}
	}
	return nil
}

func boundedPreprocessBudget(contextLimit, maxOutputTokens, evidenceTokens int) int {
	contextBudget := mode.DerivePreprocessBudget(contextLimit, maxOutputTokens)
	if contextBudget == 0 || evidenceTokens < contextBudget {
		return evidenceTokens
	}
	return contextBudget
}

func maximumOutputTokens(cfg config.Config) int {
	if cfg.MaxTokens > 0 {
		return cfg.MaxTokens
	}
	return mode.DefaultMaxTokens
}

func effectiveTemperature(value float64) float64 {
	if value == 0 {
		return 1
	}
	return value
}
