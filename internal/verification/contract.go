package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const RunSchemaVersion = "evalwitness.verification-run.v1"

const (
	MaxTaskBytes       = 1 << 20
	MaxTrajectoryBytes = 8 << 20
	MaxTotalInputBytes = 64 << 20
	MaxTrajectories    = 10
	MaxCriteria        = 16
	MaxRepetitions     = 16
)

type Mode string

const (
	ModeDelta    Mode = "delta"
	ModeAbsolute Mode = "absolute"
	ModePairwise Mode = "pairwise"
)

type EvidencePolicy string

const (
	EvidenceStrictVerifier EvidencePolicy = "strict_verifier"
	EvidenceExplicitJudge  EvidencePolicy = "explicit_judge"
)

type Policy struct {
	Evidence                   EvidencePolicy `json:"evidence"`
	NReps                      int            `json:"n_reps"`
	Epsilon                    float64        `json:"epsilon"`
	BiasMitigation             string         `json:"bias_mitigation"`
	InconsistencyPolicy        string         `json:"inconsistency_policy"`
	SelectionStrategy          string         `json:"selection_strategy"`
	SingleElim                 bool           `json:"single_elimination"`
	UseSPRT                    bool           `json:"use_sprt"`
	SPRTParams                 SPRTParameters `json:"sprt"`
	MaxWorkers                 int            `json:"max_workers"`
	MinDispatchIntervalSeconds int            `json:"min_dispatch_interval_seconds,omitempty"`
	MaxPairCalls               int            `json:"max_pair_calls"`
	ConfidenceThreshold        float64        `json:"confidence_threshold"`
	CalibrationSigma           float64        `json:"calibration_sigma"`
	ConfidenceEscalation       string         `json:"confidence_escalation"`
}

// SPRTParameters keeps the application contract JSON independent from the
// concrete mode package while preserving every value that changes a decision.
type SPRTParameters struct {
	Alpha   float64 `json:"alpha"`
	Beta    float64 `json:"beta"`
	Sigma   float64 `json:"sigma"`
	Epsilon float64 `json:"epsilon"`
	MinReps int     `json:"min_reps"`
	MaxReps int     `json:"max_reps"`
}

type Input struct {
	Entrypoint           string               `json:"entrypoint"`
	Mode                 Mode                 `json:"mode"`
	Task                 string               `json:"task"`
	Trajectories         []string             `json:"trajectories"`
	Criteria             []verifier.Criterion `json:"criteria"`
	Policy               Policy               `json:"policy"`
	Limits               mode.BudgetLimits    `json:"limits"`
	StudyManifestDigest  string               `json:"study_manifest_digest,omitempty"`
	StudyVariant         string               `json:"study_variant,omitempty"`
	ServedIdentityPolicy string               `json:"served_identity_policy,omitempty"`
	ExpectedServedModel  string               `json:"expected_served_model,omitempty"`
	ExpectedServedModels []string             `json:"expected_served_models,omitempty"`
	AuthorizationDigest  string               `json:"authorization_digest,omitempty"`
	BudgetStatePath      string               `json:"budget_state_path,omitempty"`
	DisableCache         bool                 `json:"disable_cache,omitempty"`
	Lineage              LineageReferences    `json:"lineage"`
}

type LineageReferences struct {
	AuditCaseID           string `json:"audit_case_id,omitempty"`
	TransformationID      string `json:"transformation_id,omitempty"`
	OutcomeEvidenceDigest string `json:"outcome_evidence_digest,omitempty"`
	ProfilePolicyDigest   string `json:"profile_policy_digest,omitempty"`
	CapsuleDigest         string `json:"capsule_digest,omitempty"`
	StudyCellID           string `json:"study_cell_id,omitempty"`
}

type RequestPlan struct {
	Requests            []provider.RequestEnvelope `json:"requests"`
	Fingerprints        []string                   `json:"fingerprints"`
	SetFingerprint      string                     `json:"set_fingerprint"`
	ContractDigest      string                     `json:"contract_digest"`
	MaximumInputTokens  int                        `json:"maximum_input_tokens"`
	MaximumOutputTokens int                        `json:"maximum_output_tokens"`
	WorstLogicalCalls   int                        `json:"worst_logical_calls"`
}

type Plan struct {
	SchemaVersion        string                         `json:"schema_version"`
	RunFingerprint       string                         `json:"run_fingerprint"`
	Input                Input                          `json:"input"`
	PreparedTrajectories []preprocess.Result            `json:"-"`
	PreparedTextDigests  []string                       `json:"prepared_text_digests"`
	TrajectoryEvidence   []preprocess.AccountingSummary `json:"trajectory_evidence"`
	Requests             RequestPlan                    `json:"requests"`
	Authorization        *mode.AuthorizationPlan        `json:"authorization,omitempty"`
}

type AuthorizationRequiredError struct {
	Plan  mode.AuthorizationPlan
	Cause error
}

func (err *AuthorizationRequiredError) Error() string {
	if err.Cause != nil {
		return err.Cause.Error()
	}
	return "verification requires explicit live authorization"
}

func (err *AuthorizationRequiredError) Unwrap() error { return err.Cause }

type BudgetProfile struct {
	MaxRetries     int
	MaxWorkers     int
	RequestTimeout time.Duration
	InputUSDPerM   float64
	CachedUSDPerM  float64
	OutputUSDPerM  float64
	Subscription   bool
}

func (profile BudgetProfile) validate() error {
	if profile.MaxRetries < 0 || profile.MaxWorkers <= 0 || profile.RequestTimeout <= 0 {
		return errors.New("verification budget profile requires non-negative retries, positive workers, and positive timeout")
	}
	if profile.InputUSDPerM < 0 || profile.CachedUSDPerM < 0 || profile.OutputUSDPerM < 0 {
		return errors.New("verification budget profile prices must be non-negative")
	}
	return nil
}

type LifecycleState string

const (
	LifecycleComplete LifecycleState = "complete"
	LifecycleFailed   LifecycleState = "failed"
)

type Lifecycle struct {
	RuntimeOpen LifecycleState `json:"runtime_open"`
	Execution   LifecycleState `json:"execution"`
	Cleanup     LifecycleState `json:"cleanup"`
	Audit       LifecycleState `json:"audit"`
	Error       string         `json:"error,omitempty"`
}

type Result struct {
	SchemaVersion     string                  `json:"schema_version"`
	RunFingerprint    string                  `json:"run_fingerprint"`
	Mode              Mode                    `json:"mode"`
	State             verifier.DecisionState  `json:"state"`
	Delta             *mode.Verdict           `json:"delta,omitempty"`
	Absolute          *mode.Score             `json:"absolute,omitempty"`
	Selection         *mode.Selection         `json:"selection,omitempty"`
	Budget            mode.BudgetSnapshot     `json:"budget"`
	Lifecycle         Lifecycle               `json:"lifecycle"`
	CalibrationPolicy CalibrationPolicyStatus `json:"calibration_policy"`
	Fallback          FallbackAccount         `json:"fallback"`
}

const (
	CalibrationPolicyUnsupported = "unsupported_no_held_out_policy"
)

type CalibrationPolicyStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func DefaultCalibrationPolicy() CalibrationPolicyStatus {
	return CalibrationPolicyStatus{
		Status: CalibrationPolicyUnsupported,
		Reason: "no locked TASK 049 held-out calibration policy is bound to this run",
	}
}

type FallbackAccount struct {
	Kind    string `json:"kind"`
	Charged bool   `json:"charged"`
	Calls   int    `json:"calls"`
	Tokens  int    `json:"tokens"`
	Reason  string `json:"reason"`
}

func DefaultFallbackAccount() FallbackAccount {
	return FallbackAccount{
		Kind:   "none",
		Reason: "no locked TASK 049 fallback policy is bound to this run",
	}
}

func normalizeInput(input Input) (Input, error) {
	input.Entrypoint = strings.TrimSpace(input.Entrypoint)
	input.StudyManifestDigest = strings.TrimSpace(input.StudyManifestDigest)
	input.StudyVariant = strings.TrimSpace(input.StudyVariant)
	input.ExpectedServedModels = append([]string(nil), input.ExpectedServedModels...)
	if input.Entrypoint == "" {
		return Input{}, errors.New("verification entrypoint is required")
	}
	if strings.TrimSpace(input.Task) == "" {
		return Input{}, errors.New("verification task is required")
	}
	switch input.Mode {
	case ModeAbsolute:
		if len(input.Trajectories) != 1 {
			return Input{}, fmt.Errorf("absolute verification requires exactly 1 trajectory, got %d", len(input.Trajectories))
		}
	case ModeDelta:
		if len(input.Trajectories) != 2 {
			return Input{}, fmt.Errorf("delta verification requires exactly 2 trajectories, got %d", len(input.Trajectories))
		}
	case ModePairwise:
		if len(input.Trajectories) < 2 || len(input.Trajectories) > MaxTrajectories {
			return Input{}, fmt.Errorf("pairwise verification requires 2 to %d trajectories, got %d", MaxTrajectories, len(input.Trajectories))
		}
	default:
		return Input{}, fmt.Errorf("unsupported verification mode %q", input.Mode)
	}
	if len(input.Task) > MaxTaskBytes {
		return Input{}, fmt.Errorf("verification task exceeds %d bytes", MaxTaskBytes)
	}
	totalBytes := len(input.Task)
	for index, trajectory := range input.Trajectories {
		if strings.TrimSpace(trajectory) == "" {
			return Input{}, fmt.Errorf("verification trajectory %d is empty", index)
		}
		if len(trajectory) > MaxTrajectoryBytes {
			return Input{}, fmt.Errorf("verification trajectory %d exceeds %d bytes", index, MaxTrajectoryBytes)
		}
		totalBytes += len(trajectory)
	}
	if totalBytes > MaxTotalInputBytes {
		return Input{}, fmt.Errorf("verification input exceeds %d bytes", MaxTotalInputBytes)
	}
	if len(input.Criteria) == 0 {
		return Input{}, errors.New("verification criteria are required")
	}
	if len(input.Criteria) > MaxCriteria {
		return Input{}, fmt.Errorf("verification criteria exceed %d items", MaxCriteria)
	}
	criteria := append([]verifier.Criterion(nil), input.Criteria...)
	criterionIDs := make(map[string]struct{}, len(criteria))
	for index, criterion := range criteria {
		if strings.TrimSpace(criterion.ID) == "" || strings.TrimSpace(criterion.Name) == "" || strings.TrimSpace(criterion.Description) == "" {
			return Input{}, fmt.Errorf("verification criterion %d is incomplete", index)
		}
		if _, exists := criterionIDs[criterion.ID]; exists {
			return Input{}, fmt.Errorf("verification criterion %q is duplicated", criterion.ID)
		}
		criterionIDs[criterion.ID] = struct{}{}
	}
	input.Criteria = criteria
	if err := validatePolicy(input.Policy); err != nil {
		return Input{}, err
	}
	if err := validateLimits(input.Limits); err != nil {
		return Input{}, err
	}
	if input.StudyManifestDigest != "" {
		decoded, err := hex.DecodeString(input.StudyManifestDigest)
		if err != nil || len(decoded) != sha256.Size || strings.ToLower(input.StudyManifestDigest) != input.StudyManifestDigest {
			return Input{}, errors.New("verification study_manifest_digest must be a lowercase sha256 digest")
		}
	}
	if input.StudyVariant != "" && (len(input.StudyVariant) > 250 || !utf8.ValidString(input.StudyVariant) || strings.IndexFunc(input.StudyVariant, unicode.IsControl) >= 0) {
		return Input{}, errors.New("verification study_variant is invalid")
	}
	if err := provider.ValidateServedIdentityPolicy(input.ServedIdentityPolicy, input.ExpectedServedModel, input.ExpectedServedModels); err != nil {
		return Input{}, fmt.Errorf("verification served identity policy: %w", err)
	}
	if err := validateLineageReferences(input.Lineage); err != nil {
		return Input{}, err
	}
	return input, nil
}

func validateLineageReferences(lineage LineageReferences) error {
	for name, value := range map[string]string{
		"audit_case_id": lineage.AuditCaseID, "transformation_id": lineage.TransformationID, "study_cell_id": lineage.StudyCellID,
	} {
		if value == "" {
			continue
		}
		if value != strings.TrimSpace(value) || len(value) > 250 || !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return fmt.Errorf("verification lineage %s is invalid", name)
		}
	}
	for name, value := range map[string]string{
		"outcome_evidence_digest": lineage.OutcomeEvidenceDigest,
		"profile_policy_digest":   lineage.ProfilePolicyDigest,
		"capsule_digest":          lineage.CapsuleDigest,
	} {
		if value == "" {
			continue
		}
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size || strings.ToLower(value) != value {
			return fmt.Errorf("verification lineage %s must be a lowercase sha256 digest", name)
		}
	}
	return nil
}

func validatePolicy(policy Policy) error {
	switch policy.Evidence {
	case EvidenceStrictVerifier, EvidenceExplicitJudge:
	default:
		return fmt.Errorf("unsupported evidence policy %q", policy.Evidence)
	}
	if policy.NReps <= 0 || policy.NReps > MaxRepetitions {
		return fmt.Errorf("verification n_reps must be between 1 and %d, got %d", MaxRepetitions, policy.NReps)
	}
	if policy.BiasMitigation == "adaptive" && policy.NReps != 1 && policy.SelectionStrategy != "absolute" && policy.SelectionStrategy != "joint_absolute" {
		return errors.New("adaptive bias mitigation requires n_reps=1 unless selection_strategy is absolute or joint_absolute")
	}
	if policy.Epsilon <= 0 || policy.Epsilon >= 1 {
		return fmt.Errorf("verification epsilon must be in (0,1), got %g", policy.Epsilon)
	}
	switch policy.BiasMitigation {
	case "adaptive", "both", "single", "disabled":
	default:
		return fmt.Errorf("unsupported bias mitigation %q", policy.BiasMitigation)
	}
	switch policy.InconsistencyPolicy {
	case "adaptive", "flag-only":
	default:
		return fmt.Errorf("unsupported inconsistency policy %q", policy.InconsistencyPolicy)
	}
	switch policy.SelectionStrategy {
	case "pairwise", "absolute", "joint_absolute":
	default:
		return fmt.Errorf("unsupported selection strategy %q", policy.SelectionStrategy)
	}
	if policy.MaxWorkers <= 0 {
		return errors.New("verification max_workers must be positive")
	}
	if policy.MinDispatchIntervalSeconds < 0 {
		return errors.New("verification min_dispatch_interval_seconds must be non-negative")
	}
	if policy.MinDispatchIntervalSeconds > 0 && policy.MaxWorkers != 1 {
		return errors.New("verification min_dispatch_interval_seconds requires max_workers=1")
	}
	if policy.MaxPairCalls <= 0 || policy.MaxPairCalls > 4 {
		return fmt.Errorf("verification max_pair_calls must be between 1 and 4, got %d", policy.MaxPairCalls)
	}
	if policy.ConfidenceThreshold <= 0.5 || policy.ConfidenceThreshold >= 1 {
		return fmt.Errorf("verification confidence_threshold must be in (0.5,1), got %g", policy.ConfidenceThreshold)
	}
	switch policy.ConfidenceEscalation {
	case "", mode.ConfidenceEscalationDisabled, mode.ConfidenceEscalationLegacy:
	default:
		return fmt.Errorf("unsupported confidence escalation %q", policy.ConfidenceEscalation)
	}
	if policy.CalibrationSigma < 0 {
		return errors.New("verification calibration_sigma must be non-negative")
	}
	if policy.UseSPRT {
		if policy.SelectionStrategy == "joint_absolute" {
			return errors.New("joint_absolute selection requires fixed repetitions")
		}
		if policy.SPRTParams.MinReps <= 0 || policy.SPRTParams.MaxReps < policy.SPRTParams.MinReps {
			return errors.New("verification SPRT repetition bounds are invalid")
		}
	}
	return nil
}

func validateLimits(limits mode.BudgetLimits) error {
	if limits.MaxCalls < 0 || limits.MaxAttempts < 0 || limits.MaxEstimatedInputTokens < 0 ||
		limits.MaxReservedOutputTokens < 0 || limits.MaxConcurrent < 0 || limits.MaxCostUSD < 0 || limits.MaxDuration < 0 {
		return errors.New("verification budget limits must be non-negative")
	}
	return nil
}

func planFingerprint(input Input, textDigests []string, evidence []preprocess.AccountingSummary, requests RequestPlan) (string, error) {
	type budgetMaterial struct {
		MaxCalls                int     `json:"max_calls"`
		MaxAttempts             int     `json:"max_attempts"`
		MaxEstimatedInputTokens int     `json:"max_estimated_input_tokens"`
		MaxReservedOutputTokens int     `json:"max_reserved_output_tokens"`
		MaxConcurrent           int     `json:"max_concurrent"`
		MaxCostUSD              float64 `json:"max_cost_usd"`
		MaxDurationNanoseconds  int64   `json:"max_duration_nanoseconds"`
	}
	budget := budgetMaterial{
		input.Limits.MaxCalls, input.Limits.MaxAttempts, input.Limits.MaxEstimatedInputTokens,
		input.Limits.MaxReservedOutputTokens, input.Limits.MaxConcurrent, input.Limits.MaxCostUSD,
		int64(input.Limits.MaxDuration),
	}
	material := struct {
		SchemaVersion        string                         `json:"schema_version"`
		Mode                 Mode                           `json:"mode"`
		Task                 string                         `json:"task"`
		Criteria             []verifier.Criterion           `json:"criteria"`
		Policy               Policy                         `json:"policy"`
		Lineage              LineageReferences              `json:"lineage"`
		StudyManifest        string                         `json:"study_manifest_digest,omitempty"`
		StudyVariant         string                         `json:"study_variant,omitempty"`
		ServedIdentityPolicy string                         `json:"served_identity_policy,omitempty"`
		ExpectedServedModel  string                         `json:"expected_served_model,omitempty"`
		ExpectedServedModels []string                       `json:"expected_served_models,omitempty"`
		Limits               budgetMaterial                 `json:"limits"`
		TextDigests          []string                       `json:"prepared_text_digests"`
		Evidence             []preprocess.AccountingSummary `json:"trajectory_evidence"`
		RequestSet           string                         `json:"request_set_fingerprint"`
		Contract             string                         `json:"request_contract_digest"`
	}{RunSchemaVersion, input.Mode, input.Task, input.Criteria, input.Policy, input.Lineage, input.StudyManifestDigest, input.StudyVariant,
		input.ServedIdentityPolicy, input.ExpectedServedModel, input.ExpectedServedModels,
		budget, textDigests, evidence, requests.SetFingerprint, requests.ContractDigest}
	encoded, err := json.Marshal(material)
	if err != nil {
		return "", fmt.Errorf("encode verification plan: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
