package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const BatchPlanBindingSchemaVersion = "evalwitness.verification-batch-binding.v1"

type BatchBudgetBinding struct {
	MaxCalls                int     `json:"max_calls"`
	MaxAttempts             int     `json:"max_attempts"`
	MaxEstimatedInputTokens int     `json:"max_estimated_input_tokens"`
	MaxReservedOutputTokens int     `json:"max_reserved_output_tokens"`
	MaxConcurrent           int     `json:"max_concurrent"`
	MaxCostUSD              float64 `json:"max_cost_usd"`
	MaxDurationNanoseconds  int64   `json:"max_duration_nanoseconds"`
}

type BatchPlanBinding struct {
	SchemaVersion                 string             `json:"schema_version"`
	RunFingerprint                string             `json:"run_fingerprint"`
	Offline                       bool               `json:"offline"`
	Entrypoint                    string             `json:"entrypoint"`
	EvidencePolicy                EvidencePolicy     `json:"evidence_policy"`
	StudyManifestDigest           string             `json:"study_manifest_digest"`
	BudgetStatePath               string             `json:"budget_state_path,omitempty"`
	DisableCache                  bool               `json:"disable_cache"`
	InputCount                    int                `json:"input_count"`
	InputModes                    []Mode             `json:"input_modes"`
	TaskDigests                   []string           `json:"task_digests"`
	CriteriaDigests               []string           `json:"criteria_digests"`
	BasePolicyDigests             []string           `json:"base_policy_digests"`
	Repetitions                   []int              `json:"repetitions"`
	AdaptiveRepetitions           []bool             `json:"adaptive_repetitions"`
	RawTrajectoryDigests          [][]string         `json:"raw_trajectory_digests"`
	StudyCellIDs                  []string           `json:"study_cell_ids"`
	StudyVariants                 []string           `json:"study_variants"`
	AuditCaseIDs                  []string           `json:"audit_case_ids"`
	TransformationIDs             []string           `json:"transformation_ids"`
	OutcomeEvidenceDigests        []string           `json:"outcome_evidence_digests"`
	ProfilePolicyDigests          []string           `json:"profile_policy_digests"`
	CapsuleDigests                []string           `json:"capsule_digests"`
	PlanFingerprints              []string           `json:"plan_fingerprints"`
	PlanFingerprintDigest         string             `json:"plan_fingerprint_digest"`
	PreparedTextDigests           [][]string         `json:"prepared_text_digests"`
	TrajectoryEvidenceDigests     []string           `json:"trajectory_evidence_digests"`
	RequestTemplates              int                `json:"request_templates"`
	DistinctRequestFingerprints   int                `json:"distinct_request_fingerprints"`
	RequestFingerprints           []string           `json:"request_fingerprints"`
	RequestSetFingerprint         string             `json:"request_set_fingerprint"`
	RequestContractDigest         string             `json:"request_contract_digest"`
	BatchRequestContractDigests   []string           `json:"batch_request_contract_digests"`
	CapabilityContractDigests     []string           `json:"capability_contract_digests"`
	CapabilityContractSetDigest   string             `json:"capability_contract_set_digest"`
	RouteID                       string             `json:"route_id"`
	RouteConfigDigest             string             `json:"route_config_digest"`
	MaximumInputTokensPerRequest  int                `json:"maximum_input_tokens_per_request"`
	MaximumOutputTokensPerRequest int                `json:"maximum_output_tokens_per_request"`
	WorstLogicalCalls             int                `json:"worst_logical_calls"`
	MaxWorkers                    int                `json:"max_workers"`
	Budget                        BatchBudgetBinding `json:"budget"`
	RequiredAuthorizationDigest   string             `json:"required_authorization_digest,omitempty"`
	Digest                        string             `json:"digest"`
}

func (service *Service) BindBatchPlan(batch BatchPlan) (BatchPlanBinding, error) {
	if service == nil {
		return BatchPlanBinding{}, errors.New("verification batch binding requires a service")
	}
	if strings.TrimSpace(batch.AuthorizationDigest) != "" {
		return BatchPlanBinding{}, errors.New("verification batch binding accepts only an unapproved authorization preview")
	}
	if err := service.validateBatchPlan(batch); err != nil {
		return BatchPlanBinding{}, fmt.Errorf("validate verification batch before binding: %w", err)
	}
	first := batch.Plans[0]
	if !validBatchBindingDigest(first.Input.StudyManifestDigest) {
		return BatchPlanBinding{}, errors.New("verification batch binding requires an exact study manifest digest")
	}
	value := BatchPlanBinding{
		SchemaVersion: BatchPlanBindingSchemaVersion, RunFingerprint: batch.RunFingerprint,
		Offline: service.offline, Entrypoint: first.Input.Entrypoint, EvidencePolicy: first.Input.Policy.Evidence,
		StudyManifestDigest: first.Input.StudyManifestDigest, BudgetStatePath: first.Input.BudgetStatePath,
		DisableCache: first.Input.DisableCache, InputCount: len(batch.Plans),
		RequestTemplates: len(batch.Requests.Requests), DistinctRequestFingerprints: len(batch.Requests.Fingerprints),
		RequestFingerprints: slices.Clone(batch.Requests.Fingerprints), RequestSetFingerprint: batch.Requests.SetFingerprint,
		RequestContractDigest: batch.Requests.ContractDigest, MaximumInputTokensPerRequest: batch.Requests.MaximumInputTokens,
		MaximumOutputTokensPerRequest: batch.Requests.MaximumOutputTokens, WorstLogicalCalls: batch.Requests.WorstLogicalCalls,
		Budget: batchBudgetBinding(first.Input),
	}
	routeIDs := make(map[string]struct{})
	routeConfigDigests := make(map[string]struct{})
	capabilityContractDigests := make(map[string]struct{})
	for _, request := range batch.Requests.Requests {
		routeIDs[request.RouteID()] = struct{}{}
		routeConfigDigests[conformance.RouteConfigDigest(request)] = struct{}{}
		capabilityContractDigests[conformance.CapabilityContractDigest(request)] = struct{}{}
	}
	if len(routeIDs) != 1 || len(routeConfigDigests) != 1 || len(capabilityContractDigests) == 0 {
		return BatchPlanBinding{}, errors.New("verification batch binding requires one route and at least one exact request contract")
	}
	value.RouteID = sortedKeys(routeIDs)[0]
	value.RouteConfigDigest = sortedKeys(routeConfigDigests)[0]
	value.CapabilityContractDigests = sortedKeys(capabilityContractDigests)
	value.CapabilityContractSetDigest = digestOrderedStrings(value.CapabilityContractDigests)
	batchRequestContractDigests := make(map[string]struct{})
	for _, plan := range batch.Plans {
		if plan.Input.StudyManifestDigest != value.StudyManifestDigest || strings.TrimSpace(plan.Input.StudyVariant) == "" ||
			strings.TrimSpace(plan.Input.Lineage.StudyCellID) == "" ||
			strings.TrimSpace(plan.Input.Lineage.AuditCaseID) == "" || strings.TrimSpace(plan.Input.Lineage.TransformationID) == "" {
			return BatchPlanBinding{}, errors.New("verification batch binding requires complete and consistent research lineage")
		}
		value.InputModes = append(value.InputModes, plan.Input.Mode)
		value.TaskDigests = append(value.TaskDigests, preprocess.Hash(plan.Input.Task))
		criteriaDigest, err := batchEvidenceDigest(plan.Input.Criteria)
		if err != nil {
			return BatchPlanBinding{}, err
		}
		value.CriteriaDigests = append(value.CriteriaDigests, criteriaDigest)
		basePolicy := plan.Input.Policy
		basePolicy.Evidence = ""
		basePolicyDigest, err := batchEvidenceDigest(basePolicy)
		if err != nil {
			return BatchPlanBinding{}, err
		}
		value.BasePolicyDigests = append(value.BasePolicyDigests, basePolicyDigest)
		value.Repetitions = append(value.Repetitions, plan.Input.Policy.NReps)
		value.AdaptiveRepetitions = append(value.AdaptiveRepetitions, plan.Input.Policy.UseSPRT)
		rawDigests := make([]string, len(plan.Input.Trajectories))
		for index, trajectory := range plan.Input.Trajectories {
			rawDigests[index] = preprocess.Hash(trajectory)
		}
		value.RawTrajectoryDigests = append(value.RawTrajectoryDigests, rawDigests)
		value.StudyCellIDs = append(value.StudyCellIDs, plan.Input.Lineage.StudyCellID)
		value.StudyVariants = append(value.StudyVariants, plan.Input.StudyVariant)
		value.AuditCaseIDs = append(value.AuditCaseIDs, plan.Input.Lineage.AuditCaseID)
		value.TransformationIDs = append(value.TransformationIDs, plan.Input.Lineage.TransformationID)
		value.OutcomeEvidenceDigests = append(value.OutcomeEvidenceDigests, plan.Input.Lineage.OutcomeEvidenceDigest)
		value.ProfilePolicyDigests = append(value.ProfilePolicyDigests, plan.Input.Lineage.ProfilePolicyDigest)
		value.CapsuleDigests = append(value.CapsuleDigests, plan.Input.Lineage.CapsuleDigest)
		value.PlanFingerprints = append(value.PlanFingerprints, plan.RunFingerprint)
		value.PreparedTextDigests = append(value.PreparedTextDigests, slices.Clone(plan.PreparedTextDigests))
		evidenceDigest, err := batchEvidenceDigest(plan.TrajectoryEvidence)
		if err != nil {
			return BatchPlanBinding{}, err
		}
		value.TrajectoryEvidenceDigests = append(value.TrajectoryEvidenceDigests, evidenceDigest)
		batchRequestContractDigests[plan.Requests.ContractDigest] = struct{}{}
		if plan.Input.Policy.MaxWorkers > value.MaxWorkers {
			value.MaxWorkers = plan.Input.Policy.MaxWorkers
		}
	}
	value.BatchRequestContractDigests = sortedKeys(batchRequestContractDigests)
	value.PlanFingerprintDigest = digestOrderedStrings(value.PlanFingerprints)
	if service.offline {
		if batch.Authorization != nil {
			return BatchPlanBinding{}, errors.New("offline verification batch binding carries live authorization")
		}
	} else {
		if batch.Authorization == nil || batch.Authorization.StudyManifestDigest != value.StudyManifestDigest {
			return BatchPlanBinding{}, errors.New("live verification batch binding lacks study-bound authorization")
		}
		if err := batch.Authorization.Verify(batch.Authorization.AuthorizationDigest); err != nil {
			return BatchPlanBinding{}, fmt.Errorf("verify required batch authorization: %w", err)
		}
		value.RequiredAuthorizationDigest = batch.Authorization.AuthorizationDigest
	}
	var err error
	value.Digest, err = batchPlanBindingDigest(value)
	if err != nil {
		return BatchPlanBinding{}, err
	}
	if err := value.Validate(); err != nil {
		return BatchPlanBinding{}, err
	}
	return value, nil
}

func (value BatchPlanBinding) Validate() error {
	if value.SchemaVersion != BatchPlanBindingSchemaVersion || !validBatchBindingDigest(value.RunFingerprint) ||
		strings.TrimSpace(value.Entrypoint) == "" || !validBatchBindingDigest(value.StudyManifestDigest) || value.InputCount <= 0 ||
		value.InputCount != len(value.InputModes) || value.InputCount != len(value.TaskDigests) || value.InputCount != len(value.CriteriaDigests) ||
		value.InputCount != len(value.BasePolicyDigests) || value.InputCount != len(value.Repetitions) ||
		value.InputCount != len(value.AdaptiveRepetitions) || value.InputCount != len(value.RawTrajectoryDigests) ||
		value.InputCount != len(value.StudyCellIDs) || value.InputCount != len(value.StudyVariants) || value.InputCount != len(value.AuditCaseIDs) ||
		value.InputCount != len(value.TransformationIDs) || value.InputCount != len(value.OutcomeEvidenceDigests) ||
		value.InputCount != len(value.ProfilePolicyDigests) || value.InputCount != len(value.CapsuleDigests) || value.InputCount != len(value.PlanFingerprints) ||
		value.InputCount != len(value.PreparedTextDigests) || value.InputCount != len(value.TrajectoryEvidenceDigests) ||
		!validBatchBindingDigest(value.PlanFingerprintDigest) || value.PlanFingerprintDigest != digestOrderedStrings(value.PlanFingerprints) ||
		value.RequestTemplates <= 0 || value.DistinctRequestFingerprints <= 0 || value.DistinctRequestFingerprints != len(value.RequestFingerprints) ||
		!validBatchBindingDigest(value.RequestSetFingerprint) || !validBatchBindingDigest(value.RequestContractDigest) ||
		len(value.BatchRequestContractDigests) == 0 || len(value.CapabilityContractDigests) == 0 ||
		!validBatchBindingDigest(value.CapabilityContractSetDigest) || !strings.HasPrefix(value.RouteID, "route-") ||
		!validBatchBindingDigest(strings.TrimPrefix(value.RouteID, "route-")) || !validBatchBindingDigest(value.RouteConfigDigest) ||
		value.MaximumInputTokensPerRequest <= 0 || value.MaximumOutputTokensPerRequest <= 0 || value.WorstLogicalCalls <= 0 || value.MaxWorkers <= 0 {
		return errors.New("verification batch binding identity, lineage, request, route, or workload contract is invalid")
	}
	if value.EvidencePolicy != EvidenceStrictVerifier && value.EvidencePolicy != EvidenceExplicitJudge {
		return errors.New("verification batch binding evidence policy is invalid")
	}
	if value.RequestTemplates < value.DistinctRequestFingerprints || value.RequestSetFingerprint != digestOrderedStrings(value.RequestFingerprints) ||
		value.RequestContractDigest != digestOrderedStrings(value.BatchRequestContractDigests) ||
		value.CapabilityContractSetDigest != digestOrderedStrings(value.CapabilityContractDigests) || value.WorstLogicalCalls > value.Budget.MaxCalls {
		return errors.New("verification batch binding request-set, contract, or call budget is inconsistent")
	}
	for _, values := range [][]string{value.StudyCellIDs, value.StudyVariants, value.AuditCaseIDs, value.TransformationIDs} {
		for _, item := range values {
			if strings.TrimSpace(item) == "" || item != strings.TrimSpace(item) {
				return errors.New("verification batch binding research lineage is incomplete")
			}
		}
	}
	for _, values := range [][]string{value.OutcomeEvidenceDigests, value.ProfilePolicyDigests, value.CapsuleDigests} {
		if err := validateBatchBindingOptionalDigestValues(values); err != nil {
			return err
		}
	}
	for _, mode := range value.InputModes {
		if mode != ModeAbsolute && mode != ModeDelta && mode != ModePairwise {
			return errors.New("verification batch binding input mode is invalid")
		}
	}
	for _, repetitions := range value.Repetitions {
		if repetitions <= 0 || repetitions > MaxRepetitions {
			return errors.New("verification batch binding repetition contract is invalid")
		}
	}
	for _, values := range [][]string{value.TaskDigests, value.CriteriaDigests, value.BasePolicyDigests} {
		if err := validateBatchBindingDigestValues("input contract digest", values); err != nil {
			return err
		}
	}
	for _, values := range value.RawTrajectoryDigests {
		if len(values) == 0 {
			return errors.New("verification batch binding raw trajectory digest set is empty")
		}
		if err := validateBatchBindingDigestValues("raw trajectory digest", values); err != nil {
			return err
		}
	}
	if err := validateBatchBindingDigests("plan fingerprint", value.PlanFingerprints, false); err != nil {
		return err
	}
	for _, digests := range value.PreparedTextDigests {
		if len(digests) == 0 {
			return errors.New("verification batch binding prepared-text digest set is empty")
		}
		if err := validateBatchBindingDigestValues("prepared text digest", digests); err != nil {
			return err
		}
	}
	if err := validateBatchBindingDigestValues("trajectory evidence digest", value.TrajectoryEvidenceDigests); err != nil {
		return err
	}
	if err := validateBatchBindingDigests("request fingerprint", value.RequestFingerprints, true); err != nil {
		return err
	}
	if err := validateBatchBindingDigests("batch request contract", value.BatchRequestContractDigests, true); err != nil {
		return err
	}
	if err := validateBatchBindingDigests("capability contract", value.CapabilityContractDigests, true); err != nil {
		return err
	}
	if err := value.Budget.validate(); err != nil {
		return err
	}
	if value.MaxWorkers > value.Budget.MaxConcurrent {
		return errors.New("verification batch binding workers exceed the bound concurrency budget")
	}
	if value.Offline {
		if value.RequiredAuthorizationDigest != "" {
			return errors.New("offline verification batch binding carries a required authorization digest")
		}
	} else if !validBatchBindingDigest(value.RequiredAuthorizationDigest) {
		return errors.New("live verification batch binding lacks a required authorization digest")
	}
	expected, err := batchPlanBindingDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("verification batch binding digest is invalid")
	}
	return nil
}

func (value BatchBudgetBinding) validate() error {
	if value.MaxCalls <= 0 || value.MaxAttempts <= 0 || value.MaxEstimatedInputTokens <= 0 ||
		value.MaxReservedOutputTokens <= 0 || value.MaxConcurrent <= 0 || value.MaxCostUSD < 0 ||
		math.IsNaN(value.MaxCostUSD) || math.IsInf(value.MaxCostUSD, 0) || value.MaxDurationNanoseconds <= 0 {
		return errors.New("verification batch binding budget is incomplete or invalid")
	}
	return nil
}

func (value BatchBudgetBinding) Validate() error {
	return value.validate()
}

func batchBudgetBinding(input Input) BatchBudgetBinding {
	return BatchBudgetBinding{
		MaxCalls: input.Limits.MaxCalls, MaxAttempts: input.Limits.MaxAttempts,
		MaxEstimatedInputTokens: input.Limits.MaxEstimatedInputTokens,
		MaxReservedOutputTokens: input.Limits.MaxReservedOutputTokens,
		MaxConcurrent:           input.Limits.MaxConcurrent, MaxCostUSD: input.Limits.MaxCostUSD,
		MaxDurationNanoseconds: int64(input.Limits.MaxDuration),
	}
}

func validateBatchBindingDigests(name string, values []string, sorted bool) error {
	seen := make(map[string]struct{}, len(values))
	previous := ""
	for _, value := range values {
		if !validBatchBindingDigest(value) {
			return fmt.Errorf("verification batch binding %s is invalid", name)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("verification batch binding %s is duplicated", name)
		}
		if sorted && previous != "" && value <= previous {
			return fmt.Errorf("verification batch binding %ss are not sorted", name)
		}
		seen[value] = struct{}{}
		previous = value
	}
	return nil
}

func validateBatchBindingDigestValues(name string, values []string) error {
	for _, value := range values {
		if !validBatchBindingDigest(value) {
			return fmt.Errorf("verification batch binding %s is invalid", name)
		}
	}
	return nil
}

func validateBatchBindingOptionalDigestValues(values []string) error {
	for _, value := range values {
		if value != "" && !validBatchBindingDigest(value) {
			return errors.New("verification batch binding optional lineage digest is invalid")
		}
	}
	return nil
}

func batchPlanBindingDigest(value BatchPlanBinding) (string, error) {
	value.Digest = ""
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode verification batch binding: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func batchEvidenceDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode verification batch trajectory evidence: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validBatchBindingDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}
