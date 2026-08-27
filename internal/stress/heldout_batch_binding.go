package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

const (
	HeldOutCampaignBatchBindingSchemaVersion = "evalwitness.stress-held-out-campaign-batch-binding.v1"
	heldOutBatchBindingStatus                = "provider_batches_bound_execution_not_authorized"
	heldOutBatchBindingExternalStatus        = "not_authorized"
	heldOutReplaySourceArmID                 = "score-token-verifier"
	heldOutReplayTargetArmID                 = "external-protocol-adapter"
)

// HeldOutCampaignBatchCandidate is an in-memory preflight input. The service
// validates and binds the real BatchPlan; this type is never persisted.
type HeldOutCampaignBatchCandidate struct {
	ArmID   string
	Service *verification.Service
	Batch   verification.BatchPlan
}

type HeldOutCampaignArmBatchBinding struct {
	ArmID                       string                        `json:"arm_id"`
	ExecutionClass              HeldOutCampaignExecutionClass `json:"execution_class"`
	SupportedTestCellSetDigest  string                        `json:"supported_test_cell_set_digest"`
	VerificationInputs          int                           `json:"verification_inputs"`
	OriginalInputs              int                           `json:"original_inputs"`
	TransformedInputs           int                           `json:"transformed_inputs"`
	InputContractDigest         string                        `json:"input_contract_digest"`
	OutcomeEvidenceDigestsBound bool                          `json:"outcome_evidence_digests_bound"`
	ProfilePolicyDigestsBound   bool                          `json:"profile_policy_digests_bound"`
	CapsuleDigestsBound         bool                          `json:"capsule_digests_bound"`
	Batch                       verification.BatchPlanBinding `json:"batch"`
}

type HeldOutCampaignReplayBinding struct {
	SourceArmID                 string `json:"source_arm_id"`
	TargetArmID                 string `json:"target_arm_id"`
	RouteID                     string `json:"route_id"`
	RouteConfigDigest           string `json:"route_config_digest"`
	RequestSetFingerprint       string `json:"request_set_fingerprint"`
	RequestContractDigest       string `json:"request_contract_digest"`
	CapabilityContractSetDigest string `json:"capability_contract_set_digest"`
	RequestTemplates            int    `json:"request_templates"`
	WorstLogicalCalls           int    `json:"worst_logical_calls"`
}

type HeldOutCampaignBatchBinding struct {
	SchemaVersion                  string                           `json:"schema_version"`
	CanonicalPolicy                string                           `json:"canonical_policy"`
	CampaignDigest                 string                           `json:"campaign_digest"`
	PartitionDigest                string                           `json:"partition_digest"`
	RegistryDigest                 string                           `json:"registry_digest"`
	ReleaseDigest                  string                           `json:"release_digest"`
	ArmPlanDigest                  string                           `json:"arm_plan_digest"`
	AnalysisDesignDigest           string                           `json:"analysis_design_digest"`
	StudyManifestDigest            string                           `json:"study_manifest_digest"`
	SharedInputContractDigest      string                           `json:"shared_input_contract_digest"`
	RouteID                        string                           `json:"route_id"`
	RouteConfigDigest              string                           `json:"route_config_digest"`
	Arms                           []HeldOutCampaignArmBatchBinding `json:"arms"`
	LiveBatchBindings              int                              `json:"live_batch_bindings"`
	SealedReplayBatchBindings      int                              `json:"sealed_replay_batch_bindings"`
	VerificationInputs             int                              `json:"verification_inputs"`
	LiveVerificationInputs         int                              `json:"live_verification_inputs"`
	SealedReplayVerificationInputs int                              `json:"sealed_replay_verification_inputs"`
	LiveBudget                     verification.BatchBudgetBinding  `json:"live_budget"`
	SealedReplayBudget             verification.BatchBudgetBinding  `json:"sealed_replay_budget"`
	RequiredAuthorizationDigests   []string                         `json:"required_authorization_digests"`
	Replay                         HeldOutCampaignReplayBinding     `json:"replay"`
	Status                         string                           `json:"status"`
	StudyRecordVerified            bool                             `json:"study_record_verified"`
	ExecutionBindingsVerified      bool                             `json:"execution_bindings_verified"`
	CurrentRoutesAttested          bool                             `json:"current_routes_attested"`
	PrivateCapsuleFamilyVerified   bool                             `json:"private_capsule_family_verified"`
	RunAuthorized                  bool                             `json:"run_authorized"`
	ExecutionPermitIssued          bool                             `json:"execution_permit_issued"`
	ExternalActionStatus           string                           `json:"external_action_status"`
	ProviderCalls                  int                              `json:"provider_calls"`
	EmpiricalUnits                 int                              `json:"empirical_units"`
	NetworkRequired                bool                             `json:"network_required"`
	ClaimBoundary                  HeldOutCampaignClaimBoundary     `json:"claim_boundary"`
	Digest                         string                           `json:"digest"`
}

func BuildHeldOutCampaignArmBatchBinding(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	candidate HeldOutCampaignBatchCandidate,
) (HeldOutCampaignArmBatchBinding, error) {
	if err := campaign.ValidateAgainst(lock, design, armPlan, registry, replayed); err != nil {
		return HeldOutCampaignArmBatchBinding{}, err
	}
	if strings.TrimSpace(candidate.ArmID) == "" || candidate.Service == nil {
		return HeldOutCampaignArmBatchBinding{}, errors.New("stress held-out batch candidate lacks an arm or verification service")
	}
	campaignArm, exists := heldOutCampaignArmByID(campaign.Arms, candidate.ArmID)
	if !exists || !campaignArm.ProviderDependent {
		return HeldOutCampaignArmBatchBinding{}, fmt.Errorf("stress held-out batch candidate arm %q is not provider-dependent", candidate.ArmID)
	}
	binding, err := candidate.Service.BindBatchPlan(candidate.Batch)
	if err != nil {
		return HeldOutCampaignArmBatchBinding{}, fmt.Errorf("bind stress held-out arm %q: %w", candidate.ArmID, err)
	}
	inputContractDigest, original, transformed, err := validateHeldOutBatchCandidate(campaignArm, armPlan, replayed, candidate.Batch, binding)
	if err != nil {
		return HeldOutCampaignArmBatchBinding{}, fmt.Errorf("validate stress held-out arm %q: %w", candidate.ArmID, err)
	}
	value := HeldOutCampaignArmBatchBinding{
		ArmID: candidate.ArmID, ExecutionClass: campaignArm.ExecutionClass,
		SupportedTestCellSetDigest: campaignArm.SupportedTestCellSetDigest,
		VerificationInputs:         binding.InputCount, OriginalInputs: original, TransformedInputs: transformed,
		InputContractDigest:         inputContractDigest,
		OutcomeEvidenceDigestsBound: allHeldOutDigestsBound(binding.OutcomeEvidenceDigests),
		ProfilePolicyDigestsBound:   allHeldOutDigestsBound(binding.ProfilePolicyDigests),
		CapsuleDigestsBound:         allHeldOutDigestsBound(binding.CapsuleDigests), Batch: binding,
	}
	if err := validateHeldOutArmBatchBindingAgainst(campaignArm, armPlan, replayed, value); err != nil {
		return HeldOutCampaignArmBatchBinding{}, err
	}
	return value, nil
}

func BuildHeldOutCampaignBatchBinding(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	armBindings []HeldOutCampaignArmBatchBinding,
) (HeldOutCampaignBatchBinding, error) {
	if err := campaign.ValidateAgainst(lock, design, armPlan, registry, replayed); err != nil {
		return HeldOutCampaignBatchBinding{}, err
	}
	value, err := buildHeldOutCampaignBatchBinding(campaign, armPlan, replayed, armBindings)
	if err != nil {
		return HeldOutCampaignBatchBinding{}, err
	}
	value.Digest, err = heldOutCampaignBatchBindingDigest(value)
	if err != nil {
		return HeldOutCampaignBatchBinding{}, err
	}
	if err := value.ValidateAgainst(campaign, lock, design, armPlan, registry, replayed, armBindings); err != nil {
		return HeldOutCampaignBatchBinding{}, err
	}
	return value, nil
}

func (value HeldOutCampaignBatchBinding) Validate() error {
	if value.SchemaVersion != HeldOutCampaignBatchBindingSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.CampaignDigest) || !validDigest(value.PartitionDigest) || !validDigest(value.RegistryDigest) ||
		!validDigest(value.ReleaseDigest) || !validDigest(value.ArmPlanDigest) || !validDigest(value.AnalysisDesignDigest) ||
		!validDigest(value.StudyManifestDigest) || !validDigest(value.SharedInputContractDigest) ||
		!strings.HasPrefix(value.RouteID, "route-") || !validDigest(strings.TrimPrefix(value.RouteID, "route-")) ||
		!validDigest(value.RouteConfigDigest) {
		return errors.New("stress held-out batch binding identity, parent, study, input, or route contract is invalid")
	}
	if err := validateHeldOutCampaignBatchArms(value); err != nil {
		return err
	}
	if value.LiveBatchBindings != 2 || value.SealedReplayBatchBindings != 1 ||
		value.VerificationInputs != value.LiveVerificationInputs+value.SealedReplayVerificationInputs ||
		value.VerificationInputs <= 0 || value.LiveVerificationInputs <= 0 || value.SealedReplayVerificationInputs <= 0 {
		return errors.New("stress held-out batch binding workload totals are invalid")
	}
	if err := value.LiveBudget.Validate(); err != nil {
		return fmt.Errorf("stress held-out live aggregate budget: %w", err)
	}
	if err := value.SealedReplayBudget.Validate(); err != nil {
		return fmt.Errorf("stress held-out replay budget: %w", err)
	}
	if len(value.RequiredAuthorizationDigests) != value.LiveBatchBindings || !slices.IsSorted(value.RequiredAuthorizationDigests) {
		return errors.New("stress held-out batch binding authorization preview set is incomplete or unsorted")
	}
	for index, digest := range value.RequiredAuthorizationDigests {
		if !validDigest(digest) || index > 0 && value.RequiredAuthorizationDigests[index-1] == digest {
			return errors.New("stress held-out batch binding authorization preview set is invalid or duplicated")
		}
	}
	if err := validateHeldOutCampaignReplayBinding(value); err != nil {
		return err
	}
	if value.Status != heldOutBatchBindingStatus || value.StudyRecordVerified || value.ExecutionBindingsVerified ||
		value.CurrentRoutesAttested || value.PrivateCapsuleFamilyVerified || value.RunAuthorized || value.ExecutionPermitIssued ||
		value.ExternalActionStatus != heldOutBatchBindingExternalStatus || value.ProviderCalls != 0 || value.EmpiricalUnits != 0 ||
		value.NetworkRequired || value.ClaimBoundary.SupportedClaim != heldOutBatchBindingSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutBatchBindingUnsupportedClaims) {
		return errors.New("stress held-out batch binding promoted an unverified authority, action, observation, or claim")
	}
	expected, err := heldOutCampaignBatchBindingDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out batch binding digest is invalid")
	}
	return nil
}

func (value HeldOutCampaignBatchBinding) ValidateAgainst(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	armBindings []HeldOutCampaignArmBatchBinding,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := campaign.ValidateAgainst(lock, design, armPlan, registry, replayed); err != nil {
		return err
	}
	want, err := buildHeldOutCampaignBatchBinding(campaign, armPlan, replayed, armBindings)
	if err != nil {
		return err
	}
	want.Digest, err = heldOutCampaignBatchBindingDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out batch binding differs from the exact campaign, corpus, input, request, route, budget, or authorization previews")
	}
	return nil
}

func buildHeldOutCampaignBatchBinding(
	campaign HeldOutCampaignPlan,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	armBindings []HeldOutCampaignArmBatchBinding,
) (HeldOutCampaignBatchBinding, error) {
	providerArms := heldOutProviderCampaignArms(campaign)
	if len(armBindings) != len(providerArms) {
		return HeldOutCampaignBatchBinding{}, fmt.Errorf("stress held-out batch binding received %d provider batches, want %d", len(armBindings), len(providerArms))
	}
	bindingByArm := make(map[string]HeldOutCampaignArmBatchBinding, len(armBindings))
	for _, binding := range armBindings {
		if strings.TrimSpace(binding.ArmID) == "" {
			return HeldOutCampaignBatchBinding{}, errors.New("stress held-out arm batch binding lacks an arm identity")
		}
		if _, duplicate := bindingByArm[binding.ArmID]; duplicate {
			return HeldOutCampaignBatchBinding{}, fmt.Errorf("stress held-out batch binding repeats arm %q", binding.ArmID)
		}
		bindingByArm[binding.ArmID] = binding
	}
	value := HeldOutCampaignBatchBinding{
		SchemaVersion: HeldOutCampaignBatchBindingSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		CampaignDigest: campaign.Digest, PartitionDigest: campaign.PartitionDigest, RegistryDigest: campaign.RegistryDigest,
		ReleaseDigest: campaign.ReleaseDigest, ArmPlanDigest: campaign.ArmPlanDigest, AnalysisDesignDigest: campaign.AnalysisDesignDigest,
		Status: heldOutBatchBindingStatus, ExternalActionStatus: heldOutBatchBindingExternalStatus,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim: heldOutBatchBindingSupportedClaim, UnsupportedClaims: slices.Clone(heldOutBatchBindingUnsupportedClaims),
		},
	}
	var sharedInputContractDigest string
	for _, campaignArm := range providerArms {
		armBinding, exists := bindingByArm[campaignArm.ArmID]
		if !exists {
			return HeldOutCampaignBatchBinding{}, fmt.Errorf("stress held-out batch binding lacks arm %q", campaignArm.ArmID)
		}
		if err := validateHeldOutArmBatchBindingAgainst(campaignArm, armPlan, replayed, armBinding); err != nil {
			return HeldOutCampaignBatchBinding{}, fmt.Errorf("validate stress held-out arm %q: %w", campaignArm.ArmID, err)
		}
		if sharedInputContractDigest == "" {
			sharedInputContractDigest = armBinding.InputContractDigest
		} else if armBinding.InputContractDigest != sharedInputContractDigest {
			return HeldOutCampaignBatchBinding{}, errors.New("stress held-out provider arms do not share one exact corpus and verification-input contract")
		}
		binding := armBinding.Batch
		if value.StudyManifestDigest == "" {
			value.StudyManifestDigest = binding.StudyManifestDigest
		} else if binding.StudyManifestDigest != value.StudyManifestDigest {
			return HeldOutCampaignBatchBinding{}, errors.New("stress held-out provider arms bind different study manifests")
		}
		if value.RouteID == "" {
			value.RouteID, value.RouteConfigDigest = binding.RouteID, binding.RouteConfigDigest
		} else if binding.RouteID != value.RouteID || binding.RouteConfigDigest != value.RouteConfigDigest {
			return HeldOutCampaignBatchBinding{}, errors.New("stress held-out provider arms span different route identities")
		}
		if !armBinding.OutcomeEvidenceDigestsBound || !armBinding.ProfilePolicyDigestsBound || !armBinding.CapsuleDigestsBound {
			return HeldOutCampaignBatchBinding{}, errors.New("stress held-out provider batch lacks complete outcome, profile-policy, or capsule digest lineage")
		}
		value.Arms = append(value.Arms, armBinding)
		value.VerificationInputs += binding.InputCount
		switch campaignArm.ExecutionClass {
		case HeldOutExecutionLiveProvider:
			value.LiveBatchBindings++
			value.LiveVerificationInputs += binding.InputCount
			value.LiveBudget = addHeldOutBatchBudgets(value.LiveBudget, binding.Budget)
			value.RequiredAuthorizationDigests = append(value.RequiredAuthorizationDigests, binding.RequiredAuthorizationDigest)
		case HeldOutExecutionSealedProviderReplay:
			value.SealedReplayBatchBindings++
			value.SealedReplayVerificationInputs += binding.InputCount
			value.SealedReplayBudget = binding.Budget
		default:
			return HeldOutCampaignBatchBinding{}, errors.New("stress held-out batch binding received a deterministic-local arm")
		}
	}
	value.SharedInputContractDigest = sharedInputContractDigest
	sort.Slice(value.Arms, func(left, right int) bool { return value.Arms[left].ArmID < value.Arms[right].ArmID })
	sort.Strings(value.RequiredAuthorizationDigests)
	source, sourceOK := heldOutArmBatchBindingByID(value.Arms, heldOutReplaySourceArmID)
	target, targetOK := heldOutArmBatchBindingByID(value.Arms, heldOutReplayTargetArmID)
	if !sourceOK || !targetOK {
		return HeldOutCampaignBatchBinding{}, errors.New("stress held-out batch binding lacks its strict-verifier replay source or adapter target")
	}
	if err := requireHeldOutExactReplaySource(source.Batch, target.Batch); err != nil {
		return HeldOutCampaignBatchBinding{}, err
	}
	value.Replay = HeldOutCampaignReplayBinding{
		SourceArmID: source.ArmID, TargetArmID: target.ArmID, RouteID: source.Batch.RouteID,
		RouteConfigDigest: source.Batch.RouteConfigDigest, RequestSetFingerprint: source.Batch.RequestSetFingerprint,
		RequestContractDigest:       source.Batch.RequestContractDigest,
		CapabilityContractSetDigest: source.Batch.CapabilityContractSetDigest,
		RequestTemplates:            source.Batch.RequestTemplates, WorstLogicalCalls: source.Batch.WorstLogicalCalls,
	}
	return value, nil
}

func validateHeldOutBatchCandidate(
	campaignArm HeldOutCampaignArm,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	batch verification.BatchPlan,
	binding verification.BatchPlanBinding,
) (string, int, int, error) {
	expected, err := heldOutExpectedBatchInputs(campaignArm, armPlan, replayed)
	if err != nil {
		return "", 0, 0, err
	}
	return validateHeldOutBatchCandidateAgainstExpected(campaignArm, batch, binding, expected)
}

func validateHeldOutBatchCandidateAgainstExpected(
	campaignArm HeldOutCampaignArm,
	batch verification.BatchPlan,
	binding verification.BatchPlanBinding,
	expected []heldOutExpectedBatchInput,
) (string, int, int, error) {
	arm, exists := canonicalArmByID(campaignArm.ArmID)
	if !exists || arm.Entrypoint != binding.Entrypoint || arm.EvidencePolicy != binding.EvidencePolicy {
		return "", 0, 0, errors.New("batch entrypoint or evidence policy differs from the canonical arm")
	}
	wantOffline := campaignArm.ExecutionClass == HeldOutExecutionSealedProviderReplay
	if binding.Offline != wantOffline || binding.InputCount != campaignArm.ProviderVerificationInputs || !binding.DisableCache ||
		wantOffline && binding.BudgetStatePath != "" || !wantOffline && strings.TrimSpace(binding.BudgetStatePath) == "" {
		return "", 0, 0, errors.New("batch offline, input-count, cache, or persistent-budget boundary is invalid")
	}
	if len(batch.Plans) != len(expected) {
		return "", 0, 0, fmt.Errorf("batch has %d inputs, want %d", len(batch.Plans), len(expected))
	}
	original, transformed := 0, 0
	for index, want := range expected {
		input := batch.Plans[index].Input
		if input.Entrypoint != arm.Entrypoint || input.Policy.Evidence != arm.EvidencePolicy || input.StudyManifestDigest != binding.StudyManifestDigest ||
			input.StudyVariant != want.Variant || input.Lineage.StudyCellID != want.Cell.CellID ||
			input.Lineage.AuditCaseID != want.Cell.CaseID || input.Lineage.TransformationID != want.Cell.RelationID ||
			input.Lineage.OutcomeEvidenceDigest != want.OutcomeEvidenceDigest ||
			input.AuthorizationDigest != "" || input.DisableCache != binding.DisableCache || input.BudgetStatePath != binding.BudgetStatePath {
			return "", 0, 0, fmt.Errorf("batch input %d differs from its exact arm, study, side, lineage, or execution boundary", index)
		}
		if input.Policy.NReps != campaignArm.PlannedProviderSideRepetitions/campaignArm.ProviderVerificationInputs || input.Policy.UseSPRT {
			return "", 0, 0, fmt.Errorf("batch input %d differs from the registered fixed repetition policy", index)
		}
		if input.Mode != heldOutVerificationMode(len(want.Trajectories)) || !reflect.DeepEqual(input.Trajectories, want.Trajectories) {
			return "", 0, 0, fmt.Errorf("batch input %d substitutes the exact held-out corpus trajectory set", index)
		}
		if !validDigest(input.Lineage.OutcomeEvidenceDigest) || !validDigest(input.Lineage.ProfilePolicyDigest) || !validDigest(input.Lineage.CapsuleDigest) {
			return "", 0, 0, fmt.Errorf("batch input %d lacks complete evidence and capsule lineage", index)
		}
		if want.Variant == "original" {
			original++
		} else {
			transformed++
		}
	}
	if original != campaignArm.SupportedTestCells || transformed != campaignArm.SupportedTestCells {
		return "", 0, 0, errors.New("batch does not contain exactly one original and transformed input per supported cell")
	}
	digest, err := heldOutSharedInputContractDigest(binding)
	return digest, original, transformed, err
}

type heldOutExpectedBatchInput struct {
	Cell                  ArmComparisonCell
	Variant               string
	Trajectories          []string
	OutcomeEvidenceDigest string
}

func heldOutExpectedBatchInputs(campaignArm HeldOutCampaignArm, armPlan ArmComparisonPlan, replayed []ReplayedRelationCaseV3) ([]heldOutExpectedBatchInput, error) {
	return heldOutExpectedBatchInputsForCellSet(campaignArm, armPlan, replayed, nil)
}

func heldOutExpectedBatchInputsForCellSet(
	campaignArm HeldOutCampaignArm,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	allowedCellIDs map[string]struct{},
) ([]heldOutExpectedBatchInput, error) {
	caseByID := make(map[string]ReplayedRelationCaseV3, len(replayed))
	for _, item := range replayed {
		caseByID[item.CaseID] = item
	}
	cells := make([]ArmComparisonCell, 0, campaignArm.SupportedTestCells)
	for _, cell := range armPlan.Cells {
		item, exists := caseByID[cell.CaseID]
		_, allowed := allowedCellIDs[cell.CellID]
		if cell.ArmID == campaignArm.ArmID && cell.Support == ArmSupported && exists && item.Split == study.RoleTest && (allowedCellIDs == nil || allowed) {
			cells = append(cells, cell)
		}
	}
	sort.Slice(cells, func(left, right int) bool {
		if cells[left].CaseID != cells[right].CaseID {
			return cells[left].CaseID < cells[right].CaseID
		}
		if cells[left].RelationID != cells[right].RelationID {
			return cells[left].RelationID < cells[right].RelationID
		}
		return cells[left].CellID < cells[right].CellID
	})
	if len(cells) != campaignArm.SupportedTestCells {
		return nil, errors.New("campaign arm supported-cell count differs from the exact arm plan")
	}
	cellIDs := make([]string, len(cells))
	result := make([]heldOutExpectedBatchInput, 0, len(cells)*heldOutProviderSidesPerCell)
	for index, cell := range cells {
		cellIDs[index] = cell.CellID
		item, exists := caseByID[cell.CaseID]
		if !exists || item.Split != study.RoleTest || !slices.Contains(item.RelationIDs, cell.RelationID) {
			return nil, fmt.Errorf("supported cell %q lacks its exact held-out replay case", cell.CellID)
		}
		original := renderHeldOutTrajectories(item.Original)
		transformed := renderHeldOutTrajectories(item.Transformed)
		result = append(result,
			heldOutExpectedBatchInput{Cell: cell, Variant: "original", Trajectories: original, OutcomeEvidenceDigest: item.OutcomeEvidenceDigest},
			heldOutExpectedBatchInput{Cell: cell, Variant: "transformed", Trajectories: transformed, OutcomeEvidenceDigest: item.OutcomeEvidenceDigest},
		)
	}
	sort.Strings(cellIDs)
	digest, err := digestDocument(cellIDs)
	if err != nil || digest != campaignArm.SupportedTestCellSetDigest {
		return nil, errors.New("campaign arm supported-cell set differs from its locked digest")
	}
	return result, nil
}

func renderHeldOutTrajectories(values []preprocess.Trajectory) []string {
	result := make([]string, len(values))
	for index, trajectory := range values {
		result[index] = preprocess.RenderTrajectory(trajectory)
	}
	return result
}

func heldOutVerificationMode(trajectories int) verification.Mode {
	if trajectories == 1 {
		return verification.ModeAbsolute
	}
	return verification.ModePairwise
}

func validateHeldOutArmBatchBindingAgainst(
	campaignArm HeldOutCampaignArm,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	value HeldOutCampaignArmBatchBinding,
) error {
	expected, err := heldOutExpectedBatchInputs(campaignArm, armPlan, replayed)
	if err != nil {
		return err
	}
	return validateHeldOutArmBatchBindingAgainstExpected(campaignArm, value, expected)
}

func validateHeldOutArmBatchBindingAgainstExpected(
	campaignArm HeldOutCampaignArm,
	value HeldOutCampaignArmBatchBinding,
	expected []heldOutExpectedBatchInput,
) error {
	definition, exists := canonicalArmByID(campaignArm.ArmID)
	if !exists || value.ArmID != campaignArm.ArmID || value.ExecutionClass != campaignArm.ExecutionClass ||
		value.SupportedTestCellSetDigest != campaignArm.SupportedTestCellSetDigest ||
		value.VerificationInputs != campaignArm.ProviderVerificationInputs || value.OriginalInputs != campaignArm.SupportedTestCells ||
		value.TransformedInputs != campaignArm.SupportedTestCells || !validDigest(value.InputContractDigest) ||
		!value.OutcomeEvidenceDigestsBound || !value.ProfilePolicyDigestsBound || !value.CapsuleDigestsBound {
		return errors.New("stress held-out arm batch binding identity, cell set, input count, or lineage state is invalid")
	}
	if err := value.Batch.Validate(); err != nil {
		return err
	}
	wantOffline := campaignArm.ExecutionClass == HeldOutExecutionSealedProviderReplay
	if value.Batch.Entrypoint != definition.Entrypoint || value.Batch.EvidencePolicy != definition.EvidencePolicy ||
		value.Batch.Offline != wantOffline || value.Batch.InputCount != value.VerificationInputs || !value.Batch.DisableCache ||
		wantOffline && value.Batch.BudgetStatePath != "" || !wantOffline && strings.TrimSpace(value.Batch.BudgetStatePath) == "" ||
		!allHeldOutDigestsBound(value.Batch.OutcomeEvidenceDigests) || !allHeldOutDigestsBound(value.Batch.ProfilePolicyDigests) ||
		!allHeldOutDigestsBound(value.Batch.CapsuleDigests) {
		return errors.New("stress held-out arm batch binding differs from its execution class, canonical arm, or complete lineage")
	}
	if len(expected) != value.Batch.InputCount {
		return errors.New("stress held-out arm batch binding input count differs from the exact corpus")
	}
	repetitions := campaignArm.PlannedProviderSideRepetitions / campaignArm.ProviderVerificationInputs
	for index, want := range expected {
		rawDigests := make([]string, len(want.Trajectories))
		for trajectoryIndex, trajectory := range want.Trajectories {
			rawDigests[trajectoryIndex] = preprocess.Hash(trajectory)
		}
		if value.Batch.InputModes[index] != heldOutVerificationMode(len(want.Trajectories)) ||
			!slices.Equal(value.Batch.RawTrajectoryDigests[index], rawDigests) || value.Batch.StudyCellIDs[index] != want.Cell.CellID ||
			value.Batch.StudyVariants[index] != want.Variant || value.Batch.AuditCaseIDs[index] != want.Cell.CaseID ||
			value.Batch.TransformationIDs[index] != want.Cell.RelationID || value.Batch.OutcomeEvidenceDigests[index] != want.OutcomeEvidenceDigest ||
			value.Batch.Repetitions[index] != repetitions ||
			value.Batch.AdaptiveRepetitions[index] {
			return fmt.Errorf("stress held-out arm batch binding input %d differs from the exact corpus, side, lineage, mode, or repetition contract", index)
		}
	}
	digest, err := heldOutSharedInputContractDigest(value.Batch)
	if err != nil || digest != value.InputContractDigest {
		return errors.New("stress held-out arm batch binding shared input contract digest is invalid")
	}
	return nil
}

func heldOutSharedInputContractDigest(value verification.BatchPlanBinding) (string, error) {
	material := struct {
		InputModes             []verification.Mode `json:"input_modes"`
		TaskDigests            []string            `json:"task_digests"`
		CriteriaDigests        []string            `json:"criteria_digests"`
		BasePolicyDigests      []string            `json:"base_policy_digests"`
		RawTrajectoryDigests   [][]string          `json:"raw_trajectory_digests"`
		StudyVariants          []string            `json:"study_variants"`
		AuditCaseIDs           []string            `json:"audit_case_ids"`
		TransformationIDs      []string            `json:"transformation_ids"`
		OutcomeEvidenceDigests []string            `json:"outcome_evidence_digests"`
		ProfilePolicyDigests   []string            `json:"profile_policy_digests"`
	}{
		InputModes: value.InputModes, TaskDigests: value.TaskDigests, CriteriaDigests: value.CriteriaDigests,
		BasePolicyDigests: value.BasePolicyDigests, RawTrajectoryDigests: value.RawTrajectoryDigests,
		StudyVariants: value.StudyVariants, AuditCaseIDs: value.AuditCaseIDs, TransformationIDs: value.TransformationIDs,
		OutcomeEvidenceDigests: value.OutcomeEvidenceDigests, ProfilePolicyDigests: value.ProfilePolicyDigests,
	}
	return digestDocument(material)
}

func validateHeldOutCampaignBatchArms(value HeldOutCampaignBatchBinding) error {
	if len(value.Arms) != 3 {
		return errors.New("stress held-out batch binding must contain exactly three provider-dependent arms")
	}
	previousID := ""
	live, replay, inputs, liveInputs, replayInputs := 0, 0, 0, 0, 0
	var liveBudget verification.BatchBudgetBinding
	var replayBudget verification.BatchBudgetBinding
	authorizations := make([]string, 0, 2)
	liveBudgetStatePaths := make(map[string]struct{}, 2)
	for _, arm := range value.Arms {
		if arm.ArmID <= previousID || !validDigest(arm.SupportedTestCellSetDigest) || arm.VerificationInputs <= 0 ||
			arm.VerificationInputs != arm.OriginalInputs+arm.TransformedInputs || arm.OriginalInputs != arm.TransformedInputs ||
			arm.InputContractDigest != value.SharedInputContractDigest ||
			!arm.OutcomeEvidenceDigestsBound || !arm.ProfilePolicyDigestsBound || !arm.CapsuleDigestsBound {
			return errors.New("stress held-out arm batch identity, order, side accounting, or lineage binding is invalid")
		}
		definition, exists := canonicalArmByID(arm.ArmID)
		if !exists {
			return errors.New("stress held-out batch binding contains a non-canonical arm")
		}
		executionClass, err := heldOutExecutionClass(definition)
		if err != nil || executionClass != arm.ExecutionClass || arm.Batch.Entrypoint != definition.Entrypoint || arm.Batch.EvidencePolicy != definition.EvidencePolicy ||
			arm.Batch.StudyManifestDigest != value.StudyManifestDigest || arm.Batch.RouteID != value.RouteID ||
			arm.Batch.RouteConfigDigest != value.RouteConfigDigest || arm.Batch.InputCount != arm.VerificationInputs || !arm.Batch.DisableCache {
			return errors.New("stress held-out arm batch differs from its canonical arm, study, route, or workload")
		}
		if err := arm.Batch.Validate(); err != nil {
			return fmt.Errorf("validate stress held-out arm batch %q: %w", arm.ArmID, err)
		}
		if !allHeldOutDigestsBound(arm.Batch.OutcomeEvidenceDigests) || !allHeldOutDigestsBound(arm.Batch.ProfilePolicyDigests) ||
			!allHeldOutDigestsBound(arm.Batch.CapsuleDigests) {
			return errors.New("stress held-out arm batch lineage flags contradict its bound digest arrays")
		}
		previousID = arm.ArmID
		inputs += arm.VerificationInputs
		switch arm.ExecutionClass {
		case HeldOutExecutionLiveProvider:
			if arm.Batch.Offline || strings.TrimSpace(arm.Batch.BudgetStatePath) == "" {
				return errors.New("stress held-out live arm is offline or lacks persistent budget state")
			}
			if _, duplicate := liveBudgetStatePaths[arm.Batch.BudgetStatePath]; duplicate {
				return errors.New("stress held-out live arms share one persistent budget-state path")
			}
			liveBudgetStatePaths[arm.Batch.BudgetStatePath] = struct{}{}
			live++
			liveInputs += arm.VerificationInputs
			liveBudget = addHeldOutBatchBudgets(liveBudget, arm.Batch.Budget)
			authorizations = append(authorizations, arm.Batch.RequiredAuthorizationDigest)
		case HeldOutExecutionSealedProviderReplay:
			if !arm.Batch.Offline || arm.Batch.BudgetStatePath != "" || arm.Batch.RequiredAuthorizationDigest != "" {
				return errors.New("stress held-out sealed replay arm carries live execution state")
			}
			replay++
			replayInputs += arm.VerificationInputs
			replayBudget = arm.Batch.Budget
		default:
			return errors.New("stress held-out batch binding contains a deterministic-local arm")
		}
	}
	sort.Strings(authorizations)
	if live != value.LiveBatchBindings || replay != value.SealedReplayBatchBindings || inputs != value.VerificationInputs ||
		liveInputs != value.LiveVerificationInputs || replayInputs != value.SealedReplayVerificationInputs ||
		!reflect.DeepEqual(liveBudget, value.LiveBudget) || !reflect.DeepEqual(replayBudget, value.SealedReplayBudget) ||
		!slices.Equal(authorizations, value.RequiredAuthorizationDigests) {
		return errors.New("stress held-out arm batch totals differ from the aggregate bindings")
	}
	return nil
}

func validateHeldOutCampaignReplayBinding(value HeldOutCampaignBatchBinding) error {
	source, sourceOK := heldOutArmBatchBindingByID(value.Arms, value.Replay.SourceArmID)
	target, targetOK := heldOutArmBatchBindingByID(value.Arms, value.Replay.TargetArmID)
	if !sourceOK || !targetOK || value.Replay.SourceArmID != heldOutReplaySourceArmID || value.Replay.TargetArmID != heldOutReplayTargetArmID {
		return errors.New("stress held-out replay source or target identity is invalid")
	}
	if err := requireHeldOutExactReplaySource(source.Batch, target.Batch); err != nil {
		return err
	}
	if value.Replay.RouteID != source.Batch.RouteID || value.Replay.RouteConfigDigest != source.Batch.RouteConfigDigest ||
		value.Replay.RequestSetFingerprint != source.Batch.RequestSetFingerprint || value.Replay.RequestContractDigest != source.Batch.RequestContractDigest ||
		value.Replay.CapabilityContractSetDigest != source.Batch.CapabilityContractSetDigest ||
		value.Replay.RequestTemplates != source.Batch.RequestTemplates || value.Replay.WorstLogicalCalls != source.Batch.WorstLogicalCalls {
		return errors.New("stress held-out replay projection differs from the exact strict-verifier capture contract")
	}
	return nil
}

func requireHeldOutExactReplaySource(source, target verification.BatchPlanBinding) error {
	if source.Offline || !target.Offline || source.EvidencePolicy != verification.EvidenceStrictVerifier ||
		target.EvidencePolicy != verification.EvidenceStrictVerifier || source.StudyManifestDigest != target.StudyManifestDigest ||
		source.RequestTemplates != target.RequestTemplates || source.DistinctRequestFingerprints != target.DistinctRequestFingerprints ||
		!slices.Equal(source.RequestFingerprints, target.RequestFingerprints) || source.RequestSetFingerprint != target.RequestSetFingerprint ||
		source.RequestContractDigest != target.RequestContractDigest || !slices.Equal(source.BatchRequestContractDigests, target.BatchRequestContractDigests) ||
		!slices.Equal(source.CapabilityContractDigests, target.CapabilityContractDigests) ||
		source.CapabilityContractSetDigest != target.CapabilityContractSetDigest || source.RouteID != target.RouteID ||
		source.RouteConfigDigest != target.RouteConfigDigest || source.MaximumInputTokensPerRequest != target.MaximumInputTokensPerRequest ||
		source.MaximumOutputTokensPerRequest != target.MaximumOutputTokensPerRequest || source.WorstLogicalCalls != target.WorstLogicalCalls ||
		!reflect.DeepEqual(source.Budget, target.Budget) {
		return errors.New("stress held-out protocol adapter is not an exact offline replay of the strict-verifier request and route contract")
	}
	return nil
}

func heldOutProviderCampaignArms(campaign HeldOutCampaignPlan) []HeldOutCampaignArm {
	result := make([]HeldOutCampaignArm, 0, campaign.ProviderDependentArms)
	for _, arm := range campaign.Arms {
		if arm.ProviderDependent {
			result = append(result, arm)
		}
	}
	return result
}

func heldOutCampaignArmByID(values []HeldOutCampaignArm, id string) (HeldOutCampaignArm, bool) {
	index := slices.IndexFunc(values, func(value HeldOutCampaignArm) bool { return value.ArmID == id })
	if index < 0 {
		return HeldOutCampaignArm{}, false
	}
	return values[index], true
}

func heldOutArmBatchBindingByID(values []HeldOutCampaignArmBatchBinding, id string) (HeldOutCampaignArmBatchBinding, bool) {
	index := slices.IndexFunc(values, func(value HeldOutCampaignArmBatchBinding) bool { return value.ArmID == id })
	if index < 0 {
		return HeldOutCampaignArmBatchBinding{}, false
	}
	return values[index], true
}

func allHeldOutDigestsBound(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !validDigest(value) {
			return false
		}
	}
	return true
}

func addHeldOutBatchBudgets(left, right verification.BatchBudgetBinding) verification.BatchBudgetBinding {
	return verification.BatchBudgetBinding{
		MaxCalls: left.MaxCalls + right.MaxCalls, MaxAttempts: left.MaxAttempts + right.MaxAttempts,
		MaxEstimatedInputTokens: left.MaxEstimatedInputTokens + right.MaxEstimatedInputTokens,
		MaxReservedOutputTokens: left.MaxReservedOutputTokens + right.MaxReservedOutputTokens,
		MaxConcurrent:           left.MaxConcurrent + right.MaxConcurrent, MaxCostUSD: left.MaxCostUSD + right.MaxCostUSD,
		MaxDurationNanoseconds: left.MaxDurationNanoseconds + right.MaxDurationNanoseconds,
	}
}

func heldOutCampaignBatchBindingDigest(value HeldOutCampaignBatchBinding) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutBatchBindingSupportedClaim = "the exact held-out corpus inputs, two live provider request previews, one strict-verifier sealed replay plan, routes, budgets, authorization requirements, and lineage digests are content-bound without authorizing or executing the campaign"

var heldOutBatchBindingUnsupportedClaims = []string{
	"authorized study lifecycle",
	"verified execution binding",
	"current route attestation",
	"verified private capsule family",
	"held-out execution",
	"empirical verifier reliability",
}
