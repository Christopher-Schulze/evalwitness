package verification

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/sprt"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type Runtime struct {
	Runner     *mode.Runner
	Close      func() error
	CloseAudit func() error
}

type RuntimeFactory func(context.Context, Plan) (Runtime, error)

type Service struct {
	redact           bool
	preprocessBudget int
	requestProfile   RequestProfile
	budgetProfile    BudgetProfile
	offline          bool
	openRuntime      RuntimeFactory
}

type Config struct {
	Redact           bool
	PreprocessBudget int
	RequestProfile   RequestProfile
	BudgetProfile    BudgetProfile
	Offline          bool
}

func NewService(config Config, openRuntime RuntimeFactory) (*Service, error) {
	if config.PreprocessBudget < 0 {
		return nil, errors.New("verification preprocess budget must be non-negative")
	}
	if err := config.RequestProfile.validate(); err != nil {
		return nil, err
	}
	if err := config.BudgetProfile.validate(); err != nil {
		return nil, err
	}
	if openRuntime == nil {
		return nil, errors.New("verification runtime factory is required")
	}
	return &Service{
		redact: config.Redact, preprocessBudget: config.PreprocessBudget, requestProfile: config.RequestProfile,
		budgetProfile: config.BudgetProfile, offline: config.Offline, openRuntime: openRuntime,
	}, nil
}

func (service *Service) Plan(input Input) (Plan, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return Plan{}, err
	}
	prepared := make([]preprocess.Result, len(normalized.Trajectories))
	for index, trajectory := range normalized.Trajectories {
		prepared[index], err = preprocess.CanonicalPipeline(trajectory, service.redact, service.preprocessBudget)
		if err != nil {
			return Plan{}, fmt.Errorf("prepare verification trajectory %d: %w", index, err)
		}
	}
	evidence := preprocess.AccountingSummaries(prepared...)
	textDigests := make([]string, len(prepared))
	for index, result := range prepared {
		textDigests[index] = preprocess.Hash(result.Text)
	}
	requests, err := buildRequestPlan(service.requestProfile, normalized, prepared)
	if err != nil {
		return Plan{}, err
	}
	normalized.Limits = service.resolveLimits(normalized.Limits, requests)
	fingerprint, err := planFingerprint(normalized, textDigests, evidence, requests)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{
		SchemaVersion: RunSchemaVersion, RunFingerprint: fingerprint, Input: normalized,
		PreparedTrajectories: prepared, PreparedTextDigests: textDigests, TrajectoryEvidence: evidence, Requests: requests,
	}
	if !service.offline {
		authorization, authErr := service.authorizationFor(normalized.Entrypoint, normalized.Policy.MaxWorkers, normalized.Policy.MinDispatchIntervalSeconds, normalized.Limits, requests, normalized.StudyManifestDigest)
		if authErr != nil {
			return Plan{}, authErr
		}
		plan.Authorization = &authorization
	}
	return plan, nil
}

func (service *Service) Run(ctx context.Context, input Input) (Result, error) {
	plan, err := service.Plan(input)
	if err != nil {
		return Result{}, err
	}
	return service.Execute(ctx, plan)
}

func (service *Service) Execute(ctx context.Context, plan Plan) (result Result, err error) {
	if err := service.validatePlan(plan); err != nil {
		return Result{}, err
	}
	if plan.Authorization != nil {
		provided := strings.TrimSpace(plan.Input.AuthorizationDigest)
		if provided == "" {
			return Result{}, &AuthorizationRequiredError{Plan: *plan.Authorization}
		}
		if err := plan.Authorization.Verify(provided); err != nil {
			return Result{}, &AuthorizationRequiredError{Plan: *plan.Authorization, Cause: err}
		}
	}
	result = Result{
		SchemaVersion:  RunSchemaVersion,
		RunFingerprint: plan.RunFingerprint,
		Mode:           plan.Input.Mode,
		Lifecycle: Lifecycle{
			RuntimeOpen: LifecycleFailed, Execution: LifecycleFailed, Cleanup: LifecycleFailed, Audit: LifecycleFailed,
		},
		CalibrationPolicy: DefaultCalibrationPolicy(),
		Fallback:          DefaultFallbackAccount(),
	}
	runtime, err := service.openRuntime(ctx, plan)
	if err != nil {
		result.Lifecycle.Error = err.Error()
		return result, fmt.Errorf("open verification runtime: %w", err)
	}
	if runtime.Runner == nil {
		return result, errors.Join(
			errors.New("verification runtime returned no runner"),
			closeRuntimeResources(runtime, &result.Lifecycle),
			closeRuntimeAudit(runtime, &result.Lifecycle),
		)
	}
	result.Lifecycle.RuntimeOpen = LifecycleComplete
	runErr := service.dispatch(ctx, runtime.Runner, plan, &result)
	if runErr == nil {
		result.Lifecycle.Execution = LifecycleComplete
	}
	result.Budget = runtime.Runner.Budget.Snapshot()
	cleanupErr := closeRuntimeResources(runtime, &result.Lifecycle)
	auditErr := writeRunAudit(runtime.Runner, plan, result, runErr, cleanupErr)
	if auditErr == nil {
		result.Lifecycle.Audit = LifecycleComplete
	}
	auditCloseErr := closeRuntimeAudit(runtime, &result.Lifecycle)
	err = errors.Join(runErr, cleanupErr, auditErr, auditCloseErr)
	if err != nil {
		result.Lifecycle.Error = err.Error()
	}
	return result, err
}

func closeRuntimeResources(runtime Runtime, lifecycle *Lifecycle) error {
	if runtime.Close == nil {
		lifecycle.Cleanup = LifecycleComplete
		return nil
	}
	if err := runtime.Close(); err != nil {
		lifecycle.Cleanup = LifecycleFailed
		return fmt.Errorf("close verification runtime: %w", err)
	}
	lifecycle.Cleanup = LifecycleComplete
	return nil
}

func closeRuntimeAudit(runtime Runtime, lifecycle *Lifecycle) error {
	if runtime.CloseAudit == nil {
		if lifecycle.Audit == LifecycleFailed {
			lifecycle.Audit = LifecycleComplete
		}
		return nil
	}
	if err := runtime.CloseAudit(); err != nil {
		lifecycle.Audit = LifecycleFailed
		return fmt.Errorf("close verification audit: %w", err)
	}
	if lifecycle.Audit == LifecycleFailed {
		lifecycle.Audit = LifecycleComplete
	}
	return nil
}

func (service *Service) validatePlan(plan Plan) error {
	if plan.SchemaVersion != RunSchemaVersion {
		return fmt.Errorf("verification plan schema %q is unsupported", plan.SchemaVersion)
	}
	normalized, err := normalizeInput(plan.Input)
	if err != nil {
		return err
	}
	if len(plan.PreparedTrajectories) != len(normalized.Trajectories) || len(plan.PreparedTextDigests) != len(normalized.Trajectories) || len(plan.TrajectoryEvidence) != len(normalized.Trajectories) {
		return errors.New("verification plan trajectory evidence is incomplete")
	}
	for index, prepared := range plan.PreparedTrajectories {
		if preprocess.Hash(prepared.Text) != plan.PreparedTextDigests[index] || prepared.Hash != plan.PreparedTextDigests[index] {
			return fmt.Errorf("verification plan trajectory %d text changed", index)
		}
		if prepared.AccountingSummary() != plan.TrajectoryEvidence[index] {
			return fmt.Errorf("verification plan trajectory %d evidence changed", index)
		}
	}
	requests, err := buildRequestPlan(service.requestProfile, normalized, plan.PreparedTrajectories)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(requests, plan.Requests) {
		return errors.New("verification plan request set changed")
	}
	if service.offline {
		if plan.Authorization != nil {
			return errors.New("offline verification plan carries live authorization")
		}
	} else {
		expectedAuthorization, authErr := service.authorizationFor(normalized.Entrypoint, normalized.Policy.MaxWorkers, normalized.Policy.MinDispatchIntervalSeconds, normalized.Limits, requests, normalized.StudyManifestDigest)
		if authErr != nil {
			return authErr
		}
		if plan.Authorization == nil || !reflect.DeepEqual(expectedAuthorization, *plan.Authorization) {
			return errors.New("verification plan authorization changed")
		}
	}
	fingerprint, err := planFingerprint(normalized, plan.PreparedTextDigests, plan.TrajectoryEvidence, requests)
	if err != nil {
		return err
	}
	if fingerprint != plan.RunFingerprint {
		return errors.New("verification plan fingerprint changed")
	}
	return nil
}

func (service *Service) authorizationFor(entrypoint string, maxWorkers, minDispatchIntervalSeconds int, limits mode.BudgetLimits, requests RequestPlan, studyManifestDigest string) (mode.AuthorizationPlan, error) {
	if len(requests.Requests) == 0 {
		return mode.AuthorizationPlan{}, errors.New("verification request plan is empty")
	}
	return mode.BuildAuthorizationPlan(mode.AuthorizationSpec{
		Entrypoint: entrypoint, RouteID: requests.Requests[0].RouteID(),
		RequestFingerprint: provider.Fingerprint(requests.SetFingerprint), RequestContractDigest: requests.ContractDigest,
		Limits: limits, MaxRetries: service.budgetProfile.MaxRetries, MaxWorkers: maxWorkers,
		MinDispatchIntervalSeconds: minDispatchIntervalSeconds,
		MaxOutputTokens:            requests.MaximumOutputTokens, ExpectedCalls: requests.WorstLogicalCalls, WorstCalls: requests.WorstLogicalCalls,
		StudyManifestDigest: studyManifestDigest,
	})
}

func (service *Service) resolveLimits(requested mode.BudgetLimits, requests RequestPlan) mode.BudgetLimits {
	limits := requested
	if limits.MaxCalls == 0 {
		limits.MaxCalls = requests.WorstLogicalCalls
	}
	if limits.MaxAttempts == 0 {
		limits.MaxAttempts = requests.WorstLogicalCalls * (service.budgetProfile.MaxRetries + 1)
	}
	if limits.MaxEstimatedInputTokens == 0 {
		limits.MaxEstimatedInputTokens = requests.MaximumInputTokens * limits.MaxAttempts
	}
	if limits.MaxReservedOutputTokens == 0 {
		limits.MaxReservedOutputTokens = requests.MaximumOutputTokens * limits.MaxAttempts
	}
	if limits.MaxConcurrent == 0 {
		limits.MaxConcurrent = service.budgetProfile.MaxWorkers
	}
	if limits.MaxDuration == 0 {
		waves := (limits.MaxAttempts + limits.MaxConcurrent - 1) / limits.MaxConcurrent
		limits.MaxDuration = time.Duration(maximum(1, waves)) * service.budgetProfile.RequestTimeout
	}
	if limits.MaxCostUSD == 0 {
		calculator := cost.New(
			service.budgetProfile.InputUSDPerM, service.budgetProfile.CachedUSDPerM,
			service.budgetProfile.OutputUSDPerM, service.budgetProfile.Subscription,
		)
		if estimate := calculator.EstimatePromptCost(limits.MaxEstimatedInputTokens, limits.MaxReservedOutputTokens); estimate != nil {
			limits.MaxCostUSD = *estimate
		}
	}
	return limits
}

func maximum(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func (service *Service) dispatch(ctx context.Context, runner *mode.Runner, plan Plan, result *Result) error {
	ctx = mode.ContextWithObservationScope(ctx, plan.RunFingerprint)
	baseLineage, err := baseRequestLineage(plan.Input)
	if err != nil {
		return err
	}
	ctx = mode.ContextWithRequestLineage(ctx, baseLineage)
	policy := plan.Input.Policy
	pairwise := mode.PairwiseConfig{
		NReps: policy.NReps, Epsilon: policy.Epsilon, BiasMitigation: policy.BiasMitigation,
		InconsistencyPolicy: policy.InconsistencyPolicy, SingleElim: policy.SingleElim,
		UseSPRT: policy.UseSPRT, SPRTParams: sprt.Params{
			Alpha: policy.SPRTParams.Alpha, Beta: policy.SPRTParams.Beta, Sigma: policy.SPRTParams.Sigma,
			Epsilon: policy.SPRTParams.Epsilon, MinReps: policy.SPRTParams.MinReps, MaxReps: policy.SPRTParams.MaxReps,
		}, MaxWorkers: policy.MaxWorkers, MaxPairCalls: policy.MaxPairCalls,
		ConfidenceThreshold: policy.ConfidenceThreshold, CalibrationSigma: policy.CalibrationSigma,
		ConfidenceEscalation: policy.ConfidenceEscalation,
	}
	err = nil
	switch plan.Input.Mode {
	case ModeDelta:
		var value mode.Verdict
		value, err = mode.RunDelta(ctx, runner, mode.DeltaInput{
			Task: plan.Input.Task, TrajectoryA: plan.PreparedTrajectories[0].Text,
			TrajectoryB: plan.PreparedTrajectories[1].Text, PreparedTrajectories: plan.PreparedTrajectories,
			Criteria: plan.Input.Criteria,
			Cfg: mode.DeltaConfig{
				NReps: pairwise.NReps, Epsilon: pairwise.Epsilon, BiasMitigation: pairwise.BiasMitigation,
				UseSPRT: pairwise.UseSPRT, SPRTParams: pairwise.SPRTParams, MaxPairCalls: pairwise.MaxPairCalls,
				ConfidenceThreshold: pairwise.ConfidenceThreshold, CalibrationSigma: pairwise.CalibrationSigma,
				ConfidenceEscalation: pairwise.ConfidenceEscalation,
			},
		})
		result.Delta = &value
		result.State = value.State
	case ModeAbsolute:
		var value mode.Score
		value, err = mode.RunAbsolute(ctx, runner, mode.AbsoluteInput{
			Task: plan.Input.Task, Trajectory: plan.PreparedTrajectories[0].Text,
			PreparedTrajectory: &plan.PreparedTrajectories[0], Criteria: plan.Input.Criteria,
			Cfg: mode.AbsoluteConfig{NReps: policy.NReps, UseSPRT: policy.UseSPRT, SPRTParams: pairwise.SPRTParams},
		})
		result.Absolute = &value
		result.State = value.State
	case ModePairwise:
		value := mode.Selection{}
		switch policy.SelectionStrategy {
		case "joint_absolute":
			value, err = mode.RunJointAbsoluteSelection(ctx, runner, mode.AbsoluteSelectionInput{
				Task: plan.Input.Task, Trajectories: preparedTexts(plan.PreparedTrajectories),
				PreparedTrajectories: plan.PreparedTrajectories, Criteria: plan.Input.Criteria, Cfg: pairwise,
			})
		case "absolute":
			value, err = mode.RunAbsoluteSelection(ctx, runner, mode.AbsoluteSelectionInput{
				Task: plan.Input.Task, Trajectories: preparedTexts(plan.PreparedTrajectories),
				PreparedTrajectories: plan.PreparedTrajectories, Criteria: plan.Input.Criteria, Cfg: pairwise,
			})
		default:
			value, err = mode.RunPairwise(ctx, runner, mode.PairwiseInput{
				Task: plan.Input.Task, Trajectories: preparedTexts(plan.PreparedTrajectories),
				PreparedTrajectories: plan.PreparedTrajectories, Criteria: plan.Input.Criteria, Cfg: pairwise,
			})
		}
		result.Selection = &value
		result.State = value.State
	}
	if err != nil {
		return err
	}
	return validateEvidencePolicy(plan.Input.Policy.Evidence, *result)
}

func preparedTexts(prepared []preprocess.Result) []string {
	texts := make([]string, len(prepared))
	for index, trajectory := range prepared {
		texts[index] = trajectory.Text
	}
	return texts
}

func validateEvidencePolicy(policy EvidencePolicy, result Result) error {
	if result.State != verifier.DecisionSelected || policy == EvidenceExplicitJudge {
		return nil
	}
	var strength verifier.EvidenceStrength
	var usage mode.UsageSummary
	switch {
	case result.Delta != nil:
		strength, usage = result.Delta.EvidenceStrength, result.Delta.Usage
	case result.Absolute != nil:
		strength, usage = result.Absolute.EvidenceStrength, result.Absolute.Usage
	case result.Selection != nil:
		strength, usage = result.Selection.EvidenceStrength, result.Selection.Usage
	default:
		return errors.New("selected verification result has no decision payload")
	}
	if strength.ExtractionMode != verifier.ExtractionModeVerifier || usage.ExtractionMode != string(verifier.ExtractionModeVerifier) ||
		usage.JudgeTextCalls != 0 || usage.UnextractedScores != 0 || strength.ExtractedObservations != strength.Observations {
		return fmt.Errorf("strict verifier rejected selected result: evidence=%s usage=%s extracted=%d/%d judge_calls=%d unextracted=%d",
			strength.ExtractionMode, usage.ExtractionMode, strength.ExtractedObservations, strength.Observations,
			usage.JudgeTextCalls, usage.UnextractedScores)
	}
	return nil
}
