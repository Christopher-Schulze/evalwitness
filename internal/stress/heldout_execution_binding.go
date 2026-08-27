package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

const (
	HeldOutExecutionBatchBindingSchemaVersion = "evalwitness.stress-held-out-execution-batch-binding.v1"
	heldOutExecutionBindingStatus             = "admission_filtered_provider_batches_bound_execution_not_authorized"
)

type HeldOutExecutionArmBatchBinding struct {
	ArmID                       string                        `json:"arm_id"`
	ExecutionClass              HeldOutCampaignExecutionClass `json:"execution_class"`
	EligibleTestCellIDs         []string                      `json:"eligible_test_cell_ids"`
	EligibleTestCellSetDigest   string                        `json:"eligible_test_cell_set_digest"`
	EligibleTestCells           int                           `json:"eligible_test_cells"`
	VerificationInputs          int                           `json:"verification_inputs"`
	OriginalInputs              int                           `json:"original_inputs"`
	TransformedInputs           int                           `json:"transformed_inputs"`
	InputContractDigest         string                        `json:"input_contract_digest"`
	OutcomeEvidenceDigestsBound bool                          `json:"outcome_evidence_digests_bound"`
	ProfilePolicyDigestsBound   bool                          `json:"profile_policy_digests_bound"`
	CapsuleDigestsBound         bool                          `json:"capsule_digests_bound"`
	Batch                       verification.BatchPlanBinding `json:"batch"`
}

type HeldOutExecutionBatchBinding struct {
	SchemaVersion                  string                            `json:"schema_version"`
	CanonicalPolicy                string                            `json:"canonical_policy"`
	CampaignDigest                 string                            `json:"campaign_digest"`
	AdmissionPlanDigest            string                            `json:"admission_plan_digest"`
	PartitionDigest                string                            `json:"partition_digest"`
	RegistryDigest                 string                            `json:"registry_digest"`
	ReleaseDigest                  string                            `json:"release_digest"`
	ArmPlanDigest                  string                            `json:"arm_plan_digest"`
	AnalysisDesignDigest           string                            `json:"analysis_design_digest"`
	AdmissionEligibleCellSetDigest string                            `json:"admission_eligible_cell_set_digest"`
	ProviderEligibleCellIDs        []string                          `json:"provider_eligible_cell_ids"`
	ProviderEligibleCellSetDigest  string                            `json:"provider_eligible_cell_set_digest"`
	StudyManifestDigest            string                            `json:"study_manifest_digest"`
	ProfilePolicyDigest            string                            `json:"profile_policy_digest"`
	SharedInputContractDigest      string                            `json:"shared_input_contract_digest"`
	RouteID                        string                            `json:"route_id"`
	RouteConfigDigest              string                            `json:"route_config_digest"`
	Arms                           []HeldOutExecutionArmBatchBinding `json:"arms"`
	LiveBatchBindings              int                               `json:"live_batch_bindings"`
	SealedReplayBatchBindings      int                               `json:"sealed_replay_batch_bindings"`
	ProviderEligibleCells          int                               `json:"provider_eligible_cells"`
	LiveProviderEligibleCells      int                               `json:"live_provider_eligible_cells"`
	SealedReplayEligibleCells      int                               `json:"sealed_replay_eligible_cells"`
	VerificationInputs             int                               `json:"verification_inputs"`
	LiveVerificationInputs         int                               `json:"live_verification_inputs"`
	SealedReplayVerificationInputs int                               `json:"sealed_replay_verification_inputs"`
	LiveBudget                     verification.BatchBudgetBinding   `json:"live_budget"`
	SealedReplayBudget             verification.BatchBudgetBinding   `json:"sealed_replay_budget"`
	RequiredAuthorizationDigests   []string                          `json:"required_authorization_digests"`
	Replay                         HeldOutCampaignReplayBinding      `json:"replay"`
	Status                         string                            `json:"status"`
	ExternalActionStatus           string                            `json:"external_action_status"`
	StudyRecordVerified            bool                              `json:"study_record_verified"`
	ExecutionBindingsVerified      bool                              `json:"execution_bindings_verified"`
	CurrentRoutesAttested          bool                              `json:"current_routes_attested"`
	PrivateCapsuleFamilyVerified   bool                              `json:"private_capsule_family_verified"`
	RunAuthorized                  bool                              `json:"run_authorized"`
	ExecutionPermitIssued          bool                              `json:"execution_permit_issued"`
	ProviderCalls                  int                               `json:"provider_calls"`
	EmpiricalUnits                 int                               `json:"empirical_units"`
	NetworkRequired                bool                              `json:"network_required"`
	ClaimBoundary                  HeldOutCampaignClaimBoundary      `json:"claim_boundary"`
	Digest                         string                            `json:"digest"`
}

func BuildHeldOutExecutionArmBatchBinding(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	candidate HeldOutCampaignBatchCandidate,
) (HeldOutExecutionArmBatchBinding, error) {
	if err := validateHeldOutExecutionBindingParents(campaign, lock, design, armPlan, registry, replayed, admission); err != nil {
		return HeldOutExecutionArmBatchBinding{}, err
	}
	if strings.TrimSpace(candidate.ArmID) == "" || candidate.Service == nil {
		return HeldOutExecutionArmBatchBinding{}, errors.New("stress held-out execution batch candidate lacks an arm or verification service")
	}
	projected, eligibleCellIDs, expected, err := heldOutEligibleProviderArm(campaign, armPlan, replayed, admission, candidate.ArmID)
	if err != nil {
		return HeldOutExecutionArmBatchBinding{}, err
	}
	binding, err := candidate.Service.BindBatchPlan(candidate.Batch)
	if err != nil {
		return HeldOutExecutionArmBatchBinding{}, fmt.Errorf("bind stress held-out eligible arm %q: %w", candidate.ArmID, err)
	}
	inputContractDigest, original, transformed, err := validateHeldOutBatchCandidateAgainstExpected(projected, candidate.Batch, binding, expected)
	if err != nil {
		return HeldOutExecutionArmBatchBinding{}, fmt.Errorf("validate stress held-out eligible arm %q: %w", candidate.ArmID, err)
	}
	internal := HeldOutCampaignArmBatchBinding{
		ArmID: candidate.ArmID, ExecutionClass: projected.ExecutionClass,
		SupportedTestCellSetDigest: projected.SupportedTestCellSetDigest,
		VerificationInputs:         binding.InputCount, OriginalInputs: original, TransformedInputs: transformed,
		InputContractDigest:         inputContractDigest,
		OutcomeEvidenceDigestsBound: allHeldOutDigestsBound(binding.OutcomeEvidenceDigests),
		ProfilePolicyDigestsBound:   allHeldOutDigestsBound(binding.ProfilePolicyDigests),
		CapsuleDigestsBound:         allHeldOutDigestsBound(binding.CapsuleDigests), Batch: binding,
	}
	if err := validateHeldOutArmBatchBindingAgainstExpected(projected, internal, expected); err != nil {
		return HeldOutExecutionArmBatchBinding{}, err
	}
	return HeldOutExecutionArmBatchBinding{
		ArmID: internal.ArmID, ExecutionClass: internal.ExecutionClass, EligibleTestCellIDs: eligibleCellIDs,
		EligibleTestCellSetDigest: internal.SupportedTestCellSetDigest, EligibleTestCells: len(eligibleCellIDs),
		VerificationInputs: internal.VerificationInputs, OriginalInputs: internal.OriginalInputs, TransformedInputs: internal.TransformedInputs,
		InputContractDigest: internal.InputContractDigest, OutcomeEvidenceDigestsBound: internal.OutcomeEvidenceDigestsBound,
		ProfilePolicyDigestsBound: internal.ProfilePolicyDigestsBound, CapsuleDigestsBound: internal.CapsuleDigestsBound, Batch: internal.Batch,
	}, nil
}

func BuildHeldOutExecutionBatchBinding(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	armBindings []HeldOutExecutionArmBatchBinding,
) (HeldOutExecutionBatchBinding, error) {
	if err := validateHeldOutExecutionBindingParents(campaign, lock, design, armPlan, registry, replayed, admission); err != nil {
		return HeldOutExecutionBatchBinding{}, err
	}
	value, err := buildHeldOutExecutionBatchBinding(campaign, armPlan, replayed, admission, armBindings)
	if err != nil {
		return HeldOutExecutionBatchBinding{}, err
	}
	value.Digest, err = heldOutExecutionBatchBindingDigest(value)
	if err != nil {
		return HeldOutExecutionBatchBinding{}, err
	}
	if err := value.ValidateAgainst(campaign, lock, design, armPlan, registry, replayed, admission, armBindings); err != nil {
		return HeldOutExecutionBatchBinding{}, err
	}
	return value, nil
}

func (value HeldOutExecutionBatchBinding) Validate() error {
	if value.SchemaVersion != HeldOutExecutionBatchBindingSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.CampaignDigest) || !validDigest(value.AdmissionPlanDigest) || !validDigest(value.PartitionDigest) ||
		!validDigest(value.RegistryDigest) || !validDigest(value.ReleaseDigest) || !validDigest(value.ArmPlanDigest) ||
		!validDigest(value.AnalysisDesignDigest) || !validDigest(value.AdmissionEligibleCellSetDigest) ||
		!validDigest(value.ProviderEligibleCellSetDigest) || !validDigest(value.StudyManifestDigest) || !validDigest(value.ProfilePolicyDigest) ||
		!validDigest(value.SharedInputContractDigest) || !strings.HasPrefix(value.RouteID, "route-") ||
		!validDigest(strings.TrimPrefix(value.RouteID, "route-")) || !validDigest(value.RouteConfigDigest) {
		return errors.New("stress held-out execution binding identity, admission, study, input, or route contract is invalid")
	}
	if len(value.ProviderEligibleCellIDs) != value.ProviderEligibleCells || !slices.IsSorted(value.ProviderEligibleCellIDs) ||
		!uniqueSortedStrings(value.ProviderEligibleCellIDs) {
		return errors.New("stress held-out execution binding provider-eligible cell set is invalid")
	}
	providerDigest, err := digestDocument(value.ProviderEligibleCellIDs)
	if err != nil || providerDigest != value.ProviderEligibleCellSetDigest {
		return errors.New("stress held-out execution binding provider-eligible cell-set digest is invalid")
	}
	if err := validateHeldOutExecutionBatchArms(value); err != nil {
		return err
	}
	if value.LiveBatchBindings != 2 || value.SealedReplayBatchBindings != 1 ||
		value.ProviderEligibleCells != value.LiveProviderEligibleCells+value.SealedReplayEligibleCells ||
		value.VerificationInputs != value.LiveVerificationInputs+value.SealedReplayVerificationInputs ||
		value.VerificationInputs != value.ProviderEligibleCells*heldOutProviderSidesPerCell ||
		value.LiveVerificationInputs != value.LiveProviderEligibleCells*heldOutProviderSidesPerCell ||
		value.SealedReplayVerificationInputs != value.SealedReplayEligibleCells*heldOutProviderSidesPerCell ||
		value.ProviderEligibleCells <= 0 || value.LiveProviderEligibleCells <= 0 || value.SealedReplayEligibleCells <= 0 {
		return errors.New("stress held-out execution binding admission-filtered workload totals are invalid")
	}
	if err := value.LiveBudget.Validate(); err != nil {
		return fmt.Errorf("stress held-out eligible live aggregate budget: %w", err)
	}
	if err := value.SealedReplayBudget.Validate(); err != nil {
		return fmt.Errorf("stress held-out eligible replay budget: %w", err)
	}
	if len(value.RequiredAuthorizationDigests) != value.LiveBatchBindings || !slices.IsSorted(value.RequiredAuthorizationDigests) ||
		!uniqueSortedStrings(value.RequiredAuthorizationDigests) {
		return errors.New("stress held-out execution binding authorization preview set is incomplete, duplicated, or unsorted")
	}
	for _, digest := range value.RequiredAuthorizationDigests {
		if !validDigest(digest) {
			return errors.New("stress held-out execution binding authorization preview digest is invalid")
		}
	}
	if err := validateHeldOutExecutionReplayBinding(value); err != nil {
		return err
	}
	if value.Status != heldOutExecutionBindingStatus || value.ExternalActionStatus != heldOutBatchBindingExternalStatus ||
		value.StudyRecordVerified || value.ExecutionBindingsVerified ||
		value.CurrentRoutesAttested || value.PrivateCapsuleFamilyVerified || value.RunAuthorized || value.ExecutionPermitIssued ||
		value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired ||
		value.ClaimBoundary.SupportedClaim != heldOutExecutionBindingSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutExecutionBindingUnsupportedClaims) {
		return errors.New("stress held-out execution binding promoted a filtered request preview into authority, action, observation, or claim")
	}
	expected, err := heldOutExecutionBatchBindingDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out execution binding digest is invalid")
	}
	return nil
}

func (value HeldOutExecutionBatchBinding) ValidateAgainst(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	armBindings []HeldOutExecutionArmBatchBinding,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := validateHeldOutExecutionBindingParents(campaign, lock, design, armPlan, registry, replayed, admission); err != nil {
		return err
	}
	want, err := buildHeldOutExecutionBatchBinding(campaign, armPlan, replayed, admission, armBindings)
	if err != nil {
		return err
	}
	want.Digest, err = heldOutExecutionBatchBindingDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out execution binding differs from its exact admission-filtered corpus, requests, routes, budgets, or authorization previews")
	}
	return nil
}

func validateHeldOutExecutionBindingParents(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
) error {
	if err := campaign.ValidateAgainst(lock, design, armPlan, registry, replayed); err != nil {
		return err
	}
	if err := admission.Validate(); err != nil {
		return err
	}
	if admission.CampaignDigest != campaign.Digest || admission.PartitionDigest != lock.Digest ||
		admission.AnalysisDesignDigest != design.Digest || admission.ArmPlanDigest != armPlan.Digest ||
		admission.RegistryDigest != registry.Digest || admission.CorpusReleaseDigest != registry.ReleaseDigest {
		return errors.New("stress held-out execution binding admission plan differs from the exact locked campaign parents")
	}
	return nil
}

func buildHeldOutExecutionBatchBinding(
	campaign HeldOutCampaignPlan,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	armBindings []HeldOutExecutionArmBatchBinding,
) (HeldOutExecutionBatchBinding, error) {
	providerArms := heldOutProviderCampaignArms(campaign)
	if len(armBindings) != len(providerArms) {
		return HeldOutExecutionBatchBinding{}, fmt.Errorf("stress held-out execution binding received %d eligible provider batches, want %d", len(armBindings), len(providerArms))
	}
	bindingByArm := make(map[string]HeldOutExecutionArmBatchBinding, len(armBindings))
	for _, binding := range armBindings {
		if strings.TrimSpace(binding.ArmID) == "" {
			return HeldOutExecutionBatchBinding{}, errors.New("stress held-out eligible arm batch binding lacks an arm identity")
		}
		if _, duplicate := bindingByArm[binding.ArmID]; duplicate {
			return HeldOutExecutionBatchBinding{}, fmt.Errorf("stress held-out execution binding repeats arm %q", binding.ArmID)
		}
		bindingByArm[binding.ArmID] = binding
	}
	admissionEligibleDigest, err := digestDocument(admission.ExecutionEligibleCellIDs)
	if err != nil {
		return HeldOutExecutionBatchBinding{}, err
	}
	value := HeldOutExecutionBatchBinding{
		SchemaVersion: HeldOutExecutionBatchBindingSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		CampaignDigest: campaign.Digest, AdmissionPlanDigest: admission.Digest, PartitionDigest: campaign.PartitionDigest,
		RegistryDigest: campaign.RegistryDigest, ReleaseDigest: campaign.ReleaseDigest, ArmPlanDigest: campaign.ArmPlanDigest,
		AnalysisDesignDigest: campaign.AnalysisDesignDigest, AdmissionEligibleCellSetDigest: admissionEligibleDigest,
		Status: heldOutExecutionBindingStatus, ExternalActionStatus: heldOutBatchBindingExternalStatus,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim: heldOutExecutionBindingSupportedClaim, UnsupportedClaims: slices.Clone(heldOutExecutionBindingUnsupportedClaims),
		},
	}
	var sharedInputContractDigest string
	for _, campaignArm := range providerArms {
		armBinding, exists := bindingByArm[campaignArm.ArmID]
		if !exists {
			return HeldOutExecutionBatchBinding{}, fmt.Errorf("stress held-out execution binding lacks arm %q", campaignArm.ArmID)
		}
		projected, eligibleCellIDs, expected, err := heldOutEligibleProviderArm(campaign, armPlan, replayed, admission, campaignArm.ArmID)
		if err != nil {
			return HeldOutExecutionBatchBinding{}, err
		}
		if !slices.Equal(armBinding.EligibleTestCellIDs, eligibleCellIDs) || armBinding.EligibleTestCellSetDigest != projected.SupportedTestCellSetDigest ||
			armBinding.EligibleTestCells != projected.SupportedTestCells {
			return HeldOutExecutionBatchBinding{}, fmt.Errorf("stress held-out execution arm %q differs from its admission-filtered cell set", campaignArm.ArmID)
		}
		if err := validateHeldOutArmBatchBindingAgainstExpected(projected, executionArmInternalBinding(armBinding), expected); err != nil {
			return HeldOutExecutionBatchBinding{}, fmt.Errorf("validate stress held-out eligible arm %q: %w", campaignArm.ArmID, err)
		}
		if sharedInputContractDigest == "" {
			sharedInputContractDigest = armBinding.InputContractDigest
		} else if armBinding.InputContractDigest != sharedInputContractDigest {
			return HeldOutExecutionBatchBinding{}, errors.New("stress held-out eligible provider arms do not share one exact admission-filtered input contract")
		}
		binding := armBinding.Batch
		if value.StudyManifestDigest == "" {
			value.StudyManifestDigest = binding.StudyManifestDigest
		} else if binding.StudyManifestDigest != value.StudyManifestDigest {
			return HeldOutExecutionBatchBinding{}, errors.New("stress held-out eligible provider arms bind different study manifests")
		}
		if value.RouteID == "" {
			value.RouteID, value.RouteConfigDigest = binding.RouteID, binding.RouteConfigDigest
		} else if binding.RouteID != value.RouteID || binding.RouteConfigDigest != value.RouteConfigDigest {
			return HeldOutExecutionBatchBinding{}, errors.New("stress held-out eligible provider arms span different route identities")
		}
		if !armBinding.OutcomeEvidenceDigestsBound || !armBinding.ProfilePolicyDigestsBound || !armBinding.CapsuleDigestsBound {
			return HeldOutExecutionBatchBinding{}, errors.New("stress held-out eligible provider batch lacks complete outcome, profile-policy, or capsule lineage")
		}
		for _, digest := range binding.ProfilePolicyDigests {
			if value.ProfilePolicyDigest == "" {
				value.ProfilePolicyDigest = digest
			} else if digest != value.ProfilePolicyDigest {
				return HeldOutExecutionBatchBinding{}, errors.New("stress held-out eligible provider batches span different profile policies")
			}
		}
		value.Arms = append(value.Arms, armBinding)
		value.ProviderEligibleCellIDs = append(value.ProviderEligibleCellIDs, armBinding.EligibleTestCellIDs...)
		value.ProviderEligibleCells += armBinding.EligibleTestCells
		value.VerificationInputs += binding.InputCount
		switch campaignArm.ExecutionClass {
		case HeldOutExecutionLiveProvider:
			value.LiveBatchBindings++
			value.LiveProviderEligibleCells += armBinding.EligibleTestCells
			value.LiveVerificationInputs += binding.InputCount
			value.LiveBudget = addHeldOutBatchBudgets(value.LiveBudget, binding.Budget)
			value.RequiredAuthorizationDigests = append(value.RequiredAuthorizationDigests, binding.RequiredAuthorizationDigest)
		case HeldOutExecutionSealedProviderReplay:
			value.SealedReplayBatchBindings++
			value.SealedReplayEligibleCells += armBinding.EligibleTestCells
			value.SealedReplayVerificationInputs += binding.InputCount
			value.SealedReplayBudget = binding.Budget
		default:
			return HeldOutExecutionBatchBinding{}, errors.New("stress held-out execution binding received a deterministic-local arm")
		}
	}
	value.SharedInputContractDigest = sharedInputContractDigest
	sort.Slice(value.Arms, func(left, right int) bool { return value.Arms[left].ArmID < value.Arms[right].ArmID })
	sort.Strings(value.ProviderEligibleCellIDs)
	sort.Strings(value.RequiredAuthorizationDigests)
	value.ProviderEligibleCellSetDigest, err = digestDocument(value.ProviderEligibleCellIDs)
	if err != nil {
		return HeldOutExecutionBatchBinding{}, err
	}
	if value.ProviderEligibleCells != admission.EligibleLiveProviderCells+admission.EligibleSealedReplayCells ||
		value.LiveProviderEligibleCells != admission.EligibleLiveProviderCells ||
		value.SealedReplayEligibleCells != admission.EligibleSealedReplayCells ||
		value.VerificationInputs != admission.EligibleProviderVerificationInputs ||
		value.LiveVerificationInputs != admission.EligibleLiveVerificationInputs ||
		value.SealedReplayVerificationInputs != admission.EligibleSealedReplayVerificationInputs {
		return HeldOutExecutionBatchBinding{}, errors.New("stress held-out execution binding workload differs from the exact admission plan")
	}
	source, sourceOK := heldOutExecutionArmBindingByID(value.Arms, heldOutReplaySourceArmID)
	target, targetOK := heldOutExecutionArmBindingByID(value.Arms, heldOutReplayTargetArmID)
	if !sourceOK || !targetOK {
		return HeldOutExecutionBatchBinding{}, errors.New("stress held-out execution binding lacks its strict-verifier replay source or adapter target")
	}
	if err := requireHeldOutExactReplaySource(source.Batch, target.Batch); err != nil {
		return HeldOutExecutionBatchBinding{}, err
	}
	value.Replay = HeldOutCampaignReplayBinding{
		SourceArmID: source.ArmID, TargetArmID: target.ArmID, RouteID: source.Batch.RouteID,
		RouteConfigDigest: source.Batch.RouteConfigDigest, RequestSetFingerprint: source.Batch.RequestSetFingerprint,
		RequestContractDigest: source.Batch.RequestContractDigest, CapabilityContractSetDigest: source.Batch.CapabilityContractSetDigest,
		RequestTemplates: source.Batch.RequestTemplates, WorstLogicalCalls: source.Batch.WorstLogicalCalls,
	}
	return value, nil
}

func heldOutEligibleProviderArm(
	campaign HeldOutCampaignPlan,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	armID string,
) (HeldOutCampaignArm, []string, []heldOutExpectedBatchInput, error) {
	campaignArm, exists := heldOutCampaignArmByID(campaign.Arms, armID)
	if !exists || !campaignArm.ProviderDependent {
		return HeldOutCampaignArm{}, nil, nil, fmt.Errorf("stress held-out eligible arm %q is not provider-dependent", armID)
	}
	eligible := stringSet(admission.ExecutionEligibleCellIDs)
	cellIDs := make([]string, 0)
	for _, cell := range armPlan.Cells {
		if cell.ArmID != armID || cell.Support != ArmSupported {
			continue
		}
		if _, allowed := eligible[cell.CellID]; allowed {
			cellIDs = append(cellIDs, cell.CellID)
		}
	}
	sort.Strings(cellIDs)
	if len(cellIDs) == 0 {
		return HeldOutCampaignArm{}, nil, nil, fmt.Errorf("stress held-out eligible arm %q has no admitted cells", armID)
	}
	cellSetDigest, err := digestDocument(cellIDs)
	if err != nil {
		return HeldOutCampaignArm{}, nil, nil, err
	}
	projected := campaignArm
	projected.SupportedTestCellSetDigest = cellSetDigest
	projected.SupportedTestCells = len(cellIDs)
	projected.ProviderVerificationInputs = len(cellIDs) * heldOutProviderSidesPerCell
	projected.PlannedProviderSideRepetitions = projected.ProviderVerificationInputs * campaign.FixedRepetitions
	allowed := stringSet(cellIDs)
	expected, err := heldOutExpectedBatchInputsForCellSet(projected, armPlan, replayed, allowed)
	if err != nil {
		return HeldOutCampaignArm{}, nil, nil, err
	}
	return projected, cellIDs, expected, nil
}

func validateHeldOutExecutionBatchArms(value HeldOutExecutionBatchBinding) error {
	if len(value.Arms) != 3 {
		return errors.New("stress held-out execution binding must contain exactly three provider-dependent arms")
	}
	previousID := ""
	live, replay, cells, liveCells, replayCells, inputs, liveInputs, replayInputs := 0, 0, 0, 0, 0, 0, 0, 0
	var liveBudget verification.BatchBudgetBinding
	var replayBudget verification.BatchBudgetBinding
	authorizations := make([]string, 0, 2)
	allCellIDs := make([]string, 0, value.ProviderEligibleCells)
	liveBudgetStatePaths := make(map[string]struct{}, 2)
	for _, arm := range value.Arms {
		if arm.ArmID <= previousID || arm.EligibleTestCells <= 0 || len(arm.EligibleTestCellIDs) != arm.EligibleTestCells ||
			!slices.IsSorted(arm.EligibleTestCellIDs) || !uniqueSortedStrings(arm.EligibleTestCellIDs) || !validDigest(arm.EligibleTestCellSetDigest) ||
			arm.VerificationInputs != arm.EligibleTestCells*heldOutProviderSidesPerCell || arm.OriginalInputs != arm.EligibleTestCells ||
			arm.TransformedInputs != arm.EligibleTestCells || arm.InputContractDigest != value.SharedInputContractDigest ||
			!arm.OutcomeEvidenceDigestsBound || !arm.ProfilePolicyDigestsBound || !arm.CapsuleDigestsBound {
			return errors.New("stress held-out eligible arm identity, cell set, side accounting, or lineage binding is invalid")
		}
		digest, err := digestDocument(arm.EligibleTestCellIDs)
		if err != nil || digest != arm.EligibleTestCellSetDigest {
			return errors.New("stress held-out eligible arm cell-set digest is invalid")
		}
		definition, exists := canonicalArmByID(arm.ArmID)
		if !exists {
			return errors.New("stress held-out execution binding contains a non-canonical arm")
		}
		executionClass, err := heldOutExecutionClass(definition)
		if err != nil || executionClass != arm.ExecutionClass || arm.Batch.Entrypoint != definition.Entrypoint ||
			arm.Batch.EvidencePolicy != definition.EvidencePolicy || arm.Batch.StudyManifestDigest != value.StudyManifestDigest ||
			arm.Batch.RouteID != value.RouteID || arm.Batch.RouteConfigDigest != value.RouteConfigDigest ||
			arm.Batch.InputCount != arm.VerificationInputs || !arm.Batch.DisableCache {
			return errors.New("stress held-out eligible arm differs from its canonical arm, study, route, or workload")
		}
		if err := arm.Batch.Validate(); err != nil {
			return fmt.Errorf("validate stress held-out eligible arm batch %q: %w", arm.ArmID, err)
		}
		if !allHeldOutDigestsBound(arm.Batch.OutcomeEvidenceDigests) || !allHeldOutDigestsBound(arm.Batch.ProfilePolicyDigests) ||
			!allHeldOutDigestsBound(arm.Batch.CapsuleDigests) {
			return errors.New("stress held-out eligible arm lineage flags contradict its bound digest arrays")
		}
		for _, digest := range arm.Batch.ProfilePolicyDigests {
			if digest != value.ProfilePolicyDigest {
				return errors.New("stress held-out eligible arm profile-policy lineage differs from the aggregate binding")
			}
		}
		previousID = arm.ArmID
		allCellIDs = append(allCellIDs, arm.EligibleTestCellIDs...)
		cells += arm.EligibleTestCells
		inputs += arm.VerificationInputs
		switch arm.ExecutionClass {
		case HeldOutExecutionLiveProvider:
			if arm.Batch.Offline || strings.TrimSpace(arm.Batch.BudgetStatePath) == "" {
				return errors.New("stress held-out eligible live arm is offline or lacks persistent budget state")
			}
			if _, duplicate := liveBudgetStatePaths[arm.Batch.BudgetStatePath]; duplicate {
				return errors.New("stress held-out eligible live arms share one persistent budget-state path")
			}
			liveBudgetStatePaths[arm.Batch.BudgetStatePath] = struct{}{}
			live++
			liveCells += arm.EligibleTestCells
			liveInputs += arm.VerificationInputs
			liveBudget = addHeldOutBatchBudgets(liveBudget, arm.Batch.Budget)
			authorizations = append(authorizations, arm.Batch.RequiredAuthorizationDigest)
		case HeldOutExecutionSealedProviderReplay:
			if !arm.Batch.Offline || arm.Batch.BudgetStatePath != "" || arm.Batch.RequiredAuthorizationDigest != "" {
				return errors.New("stress held-out eligible sealed replay arm carries live execution state")
			}
			replay++
			replayCells += arm.EligibleTestCells
			replayInputs += arm.VerificationInputs
			replayBudget = arm.Batch.Budget
		default:
			return errors.New("stress held-out execution binding contains a deterministic-local arm")
		}
	}
	sort.Strings(authorizations)
	sort.Strings(allCellIDs)
	if !uniqueSortedStrings(allCellIDs) || !slices.Equal(allCellIDs, value.ProviderEligibleCellIDs) ||
		live != value.LiveBatchBindings || replay != value.SealedReplayBatchBindings || cells != value.ProviderEligibleCells ||
		liveCells != value.LiveProviderEligibleCells || replayCells != value.SealedReplayEligibleCells || inputs != value.VerificationInputs ||
		liveInputs != value.LiveVerificationInputs || replayInputs != value.SealedReplayVerificationInputs ||
		!reflect.DeepEqual(liveBudget, value.LiveBudget) || !reflect.DeepEqual(replayBudget, value.SealedReplayBudget) ||
		!slices.Equal(authorizations, value.RequiredAuthorizationDigests) {
		return errors.New("stress held-out eligible arm batches do not reproduce the aggregate admission-filtered binding")
	}
	return nil
}

func validateHeldOutExecutionReplayBinding(value HeldOutExecutionBatchBinding) error {
	source, sourceOK := heldOutExecutionArmBindingByID(value.Arms, value.Replay.SourceArmID)
	target, targetOK := heldOutExecutionArmBindingByID(value.Arms, value.Replay.TargetArmID)
	if !sourceOK || !targetOK || value.Replay.SourceArmID != heldOutReplaySourceArmID || value.Replay.TargetArmID != heldOutReplayTargetArmID {
		return errors.New("stress held-out eligible replay source or target identity is invalid")
	}
	if err := requireHeldOutExactReplaySource(source.Batch, target.Batch); err != nil {
		return err
	}
	if value.Replay.RouteID != source.Batch.RouteID || value.Replay.RouteConfigDigest != source.Batch.RouteConfigDigest ||
		value.Replay.RequestSetFingerprint != source.Batch.RequestSetFingerprint || value.Replay.RequestContractDigest != source.Batch.RequestContractDigest ||
		value.Replay.CapabilityContractSetDigest != source.Batch.CapabilityContractSetDigest ||
		value.Replay.RequestTemplates != source.Batch.RequestTemplates || value.Replay.WorstLogicalCalls != source.Batch.WorstLogicalCalls {
		return errors.New("stress held-out eligible replay projection differs from the exact strict-verifier capture contract")
	}
	return nil
}

func executionArmInternalBinding(value HeldOutExecutionArmBatchBinding) HeldOutCampaignArmBatchBinding {
	return HeldOutCampaignArmBatchBinding{
		ArmID: value.ArmID, ExecutionClass: value.ExecutionClass, SupportedTestCellSetDigest: value.EligibleTestCellSetDigest,
		VerificationInputs: value.VerificationInputs, OriginalInputs: value.OriginalInputs, TransformedInputs: value.TransformedInputs,
		InputContractDigest: value.InputContractDigest, OutcomeEvidenceDigestsBound: value.OutcomeEvidenceDigestsBound,
		ProfilePolicyDigestsBound: value.ProfilePolicyDigestsBound, CapsuleDigestsBound: value.CapsuleDigestsBound, Batch: value.Batch,
	}
}

func heldOutExecutionArmBindingByID(values []HeldOutExecutionArmBatchBinding, id string) (HeldOutExecutionArmBatchBinding, bool) {
	index := slices.IndexFunc(values, func(value HeldOutExecutionArmBatchBinding) bool { return value.ArmID == id })
	if index < 0 {
		return HeldOutExecutionArmBatchBinding{}, false
	}
	return values[index], true
}

func heldOutExecutionBatchBindingDigest(value HeldOutExecutionBatchBinding) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutExecutionBindingSupportedClaim = "the exact human-admission-eligible held-out provider cells, request previews, strict-verifier replay source, routes, budgets, authorization requirements, and lineage digests are content-bound without authorizing or executing the campaign"

var heldOutExecutionBindingUnsupportedClaims = []string{
	"authorized study lifecycle",
	"verified execution binding",
	"current route attestation",
	"verified private preflight capsule family",
	"held-out execution",
	"empirical verifier reliability",
}
