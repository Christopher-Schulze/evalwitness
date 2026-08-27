package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

const (
	HeldOutCampaignPlanSchemaVersion = "evalwitness.stress-held-out-campaign-plan.v1"
	heldOutCampaignID                = "task-056-held-out-campaign"
	heldOutCampaignStatus            = "topology_locked_live_bindings_absent"
	heldOutCampaignSidePolicy        = "original_and_transformed_verification_input_per_supported_provider_cell"
	heldOutCampaignRequestPolicy     = "bind_two_live_provider_batches_and_one_sealed_replay_adapter_batch_before_execution_authority"
	heldOutCampaignExternalStatus    = "not_authorized"
	heldOutProviderSidesPerCell      = 2
)

type HeldOutCampaignExecutionClass string

const (
	HeldOutExecutionLiveProvider         HeldOutCampaignExecutionClass = "live_provider"
	HeldOutExecutionSealedProviderReplay HeldOutCampaignExecutionClass = "sealed_provider_replay"
	HeldOutExecutionDeterministicLocal   HeldOutCampaignExecutionClass = "deterministic_local"
)

type HeldOutCampaignArm struct {
	ArmID                              string                        `json:"arm_id"`
	Kind                               ArmKind                       `json:"kind"`
	ExecutionClass                     HeldOutCampaignExecutionClass `json:"execution_class"`
	ProviderDependent                  bool                          `json:"provider_dependent"`
	Entrypoint                         string                        `json:"entrypoint,omitempty"`
	EvidencePolicy                     verification.EvidencePolicy   `json:"evidence_policy,omitempty"`
	TestCellSetDigest                  string                        `json:"test_cell_set_digest"`
	SupportedTestCellSetDigest         string                        `json:"supported_test_cell_set_digest"`
	UnsupportedTestCellSetDigest       string                        `json:"unsupported_test_cell_set_digest"`
	TestCells                          int                           `json:"test_cells"`
	SupportedTestCells                 int                           `json:"supported_test_cells"`
	UnsupportedTestCells               int                           `json:"unsupported_test_cells"`
	ProviderVerificationInputs         int                           `json:"provider_verification_inputs"`
	PlannedProviderSideRepetitions     int                           `json:"planned_provider_side_repetitions"`
	PlannedZeroCostRepetitions         int                           `json:"planned_zero_cost_repetitions"`
	ExternalProcessConformanceRequired bool                          `json:"external_process_conformance_required"`
}

type HeldOutCampaignLiveBindings struct {
	StudyRecordBound              bool `json:"study_record_bound"`
	ExecutionBindingsBound        bool `json:"execution_bindings_bound"`
	LiveProviderRequestPlansBound bool `json:"live_provider_request_plans_bound"`
	SealedReplayPlanBound         bool `json:"sealed_replay_plan_bound"`
	ProviderCallCountsBound       bool `json:"provider_call_counts_bound"`
	ProviderBudgetsBound          bool `json:"provider_budgets_bound"`
	CurrentRoutesAttested         bool `json:"current_routes_attested"`
	AuthorizationDigestsBound     bool `json:"authorization_digests_bound"`
	PrivateCapsuleFamilyBound     bool `json:"private_capsule_family_bound"`
}

type HeldOutCampaignClaimBoundary struct {
	SupportedClaim    string   `json:"supported_claim"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

type HeldOutCampaignPlan struct {
	SchemaVersion                           string                       `json:"schema_version"`
	CanonicalPolicy                         string                       `json:"canonical_policy"`
	CampaignID                              string                       `json:"campaign_id"`
	PartitionDigest                         string                       `json:"partition_digest"`
	RegistryDigest                          string                       `json:"registry_digest"`
	ReleaseDigest                           string                       `json:"release_digest"`
	ArmPlanDigest                           string                       `json:"arm_plan_digest"`
	AnalysisDesignDigest                    string                       `json:"analysis_design_digest"`
	DataRole                                string                       `json:"data_role"`
	RunPolicy                               string                       `json:"run_policy"`
	VerificationSidePolicy                  string                       `json:"verification_side_policy"`
	RequestBindingPolicy                    string                       `json:"request_binding_policy"`
	RepeatKind                              RepeatKind                   `json:"repeat_kind"`
	FixedRepetitions                        int                          `json:"fixed_repetitions"`
	Arms                                    []HeldOutCampaignArm         `json:"arms"`
	TestCases                               int                          `json:"test_cases"`
	TestCells                               int                          `json:"test_cells"`
	SupportedTestCells                      int                          `json:"supported_test_cells"`
	StructuralUnsupportedTestCells          int                          `json:"structural_unsupported_test_cells"`
	ProviderDependentArms                   int                          `json:"provider_dependent_arms"`
	LiveProviderArms                        int                          `json:"live_provider_arms"`
	SealedReplayArms                        int                          `json:"sealed_replay_arms"`
	ZeroCostArms                            int                          `json:"zero_cost_arms"`
	ProviderDependentSupportedTestCells     int                          `json:"provider_dependent_supported_test_cells"`
	LiveProviderSupportedTestCells          int                          `json:"live_provider_supported_test_cells"`
	SealedReplaySupportedTestCells          int                          `json:"sealed_replay_supported_test_cells"`
	ZeroCostSupportedTestCells              int                          `json:"zero_cost_supported_test_cells"`
	ProviderDependentVerificationInputs     int                          `json:"provider_dependent_verification_inputs"`
	LiveProviderVerificationInputs          int                          `json:"live_provider_verification_inputs"`
	SealedReplayVerificationInputs          int                          `json:"sealed_replay_verification_inputs"`
	PlannedProviderDependentSideRepetitions int                          `json:"planned_provider_dependent_side_repetitions"`
	PlannedLiveProviderSideRepetitions      int                          `json:"planned_live_provider_side_repetitions"`
	PlannedSealedReplaySideRepetitions      int                          `json:"planned_sealed_replay_side_repetitions"`
	PlannedZeroCostRepetitions              int                          `json:"planned_zero_cost_repetitions"`
	LiveBindings                            HeldOutCampaignLiveBindings  `json:"live_bindings"`
	Status                                  string                       `json:"status"`
	RunAuthorized                           bool                         `json:"run_authorized"`
	ExecutionPermitIssued                   bool                         `json:"execution_permit_issued"`
	ExternalActionStatus                    string                       `json:"external_action_status"`
	ProviderCalls                           int                          `json:"provider_calls"`
	EmpiricalUnits                          int                          `json:"empirical_units"`
	NetworkRequired                         bool                         `json:"network_required"`
	ClaimBoundary                           HeldOutCampaignClaimBoundary `json:"claim_boundary"`
	Digest                                  string                       `json:"digest"`
}

func BuildHeldOutCampaignPlan(
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
) (HeldOutCampaignPlan, error) {
	if err := lock.ValidateAgainst(design, plan, registry, replayed); err != nil {
		return HeldOutCampaignPlan{}, err
	}
	value, err := heldOutCampaignPlan(lock, design, plan, registry)
	if err != nil {
		return HeldOutCampaignPlan{}, err
	}
	value.Digest, err = heldOutCampaignPlanDigest(value)
	if err != nil {
		return HeldOutCampaignPlan{}, err
	}
	if err := value.ValidateAgainst(lock, design, plan, registry, replayed); err != nil {
		return HeldOutCampaignPlan{}, err
	}
	return value, nil
}

func (value HeldOutCampaignPlan) Validate() error {
	if value.SchemaVersion != HeldOutCampaignPlanSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.CampaignID != heldOutCampaignID || !validDigest(value.PartitionDigest) || !validDigest(value.RegistryDigest) ||
		!validDigest(value.ReleaseDigest) || !validDigest(value.ArmPlanDigest) || !validDigest(value.AnalysisDesignDigest) ||
		value.DataRole != string(study.RoleTest) || value.RunPolicy != heldOutRunPolicy ||
		value.VerificationSidePolicy != heldOutCampaignSidePolicy || value.RequestBindingPolicy != heldOutCampaignRequestPolicy ||
		value.RepeatKind != RepeatFixed || value.FixedRepetitions <= 0 {
		return errors.New("stress held-out campaign identity, policy, or repetition contract is invalid")
	}
	if err := validateHeldOutCampaignArms(value); err != nil {
		return err
	}
	if value.TestCases <= 0 || value.TestCells <= 0 || value.SupportedTestCells <= 0 || value.StructuralUnsupportedTestCells < 0 ||
		value.TestCells != value.SupportedTestCells+value.StructuralUnsupportedTestCells ||
		value.ProviderDependentArms != value.LiveProviderArms+value.SealedReplayArms ||
		value.ProviderDependentArms+value.ZeroCostArms != len(value.Arms) ||
		value.ProviderDependentSupportedTestCells != value.LiveProviderSupportedTestCells+value.SealedReplaySupportedTestCells ||
		value.ProviderDependentSupportedTestCells+value.ZeroCostSupportedTestCells != value.SupportedTestCells ||
		value.ProviderDependentVerificationInputs != value.LiveProviderVerificationInputs+value.SealedReplayVerificationInputs ||
		value.ProviderDependentVerificationInputs != value.ProviderDependentSupportedTestCells*heldOutProviderSidesPerCell ||
		value.LiveProviderVerificationInputs != value.LiveProviderSupportedTestCells*heldOutProviderSidesPerCell ||
		value.SealedReplayVerificationInputs != value.SealedReplaySupportedTestCells*heldOutProviderSidesPerCell ||
		value.PlannedProviderDependentSideRepetitions != value.PlannedLiveProviderSideRepetitions+value.PlannedSealedReplaySideRepetitions ||
		value.PlannedProviderDependentSideRepetitions != value.ProviderDependentVerificationInputs*value.FixedRepetitions ||
		value.PlannedLiveProviderSideRepetitions != value.LiveProviderVerificationInputs*value.FixedRepetitions ||
		value.PlannedSealedReplaySideRepetitions != value.SealedReplayVerificationInputs*value.FixedRepetitions ||
		value.PlannedZeroCostRepetitions != value.ZeroCostSupportedTestCells*value.FixedRepetitions {
		return errors.New("stress held-out campaign aggregate topology or workload counts are invalid")
	}
	if value.LiveBindings != (HeldOutCampaignLiveBindings{}) || value.Status != heldOutCampaignStatus || value.RunAuthorized ||
		value.ExecutionPermitIssued || value.ExternalActionStatus != heldOutCampaignExternalStatus || value.ProviderCalls != 0 ||
		value.EmpiricalUnits != 0 || value.NetworkRequired || value.ClaimBoundary.SupportedClaim != heldOutCampaignSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutCampaignUnsupportedClaims) {
		return errors.New("stress held-out campaign plan promoted an absent live binding, external action, evidence, or claim")
	}
	expected, err := heldOutCampaignPlanDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out campaign plan digest is invalid")
	}
	return nil
}

func (value HeldOutCampaignPlan) ValidateAgainst(
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := lock.ValidateAgainst(design, plan, registry, replayed); err != nil {
		return err
	}
	want, err := heldOutCampaignPlan(lock, design, plan, registry)
	if err != nil {
		return err
	}
	want.Digest, err = heldOutCampaignPlanDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out campaign plan differs from the exact locked partition, arm topology, registry, or analysis design")
	}
	return nil
}

func heldOutCampaignPlan(
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
) (HeldOutCampaignPlan, error) {
	fixedRepetitions, err := heldOutCampaignFixedRepetitions(registry)
	if err != nil {
		return HeldOutCampaignPlan{}, err
	}
	testCells := stringSet(lock.TestCellIDs)
	byArm := make(map[string][]ArmComparisonCell, len(plan.Arms))
	for _, cell := range plan.Cells {
		if _, exists := testCells[cell.CellID]; exists {
			byArm[cell.ArmID] = append(byArm[cell.ArmID], cell)
		}
	}
	value := HeldOutCampaignPlan{
		SchemaVersion: HeldOutCampaignPlanSchemaVersion, CanonicalPolicy: CanonicalPolicy, CampaignID: heldOutCampaignID,
		PartitionDigest: lock.Digest, RegistryDigest: registry.Digest, ReleaseDigest: registry.ReleaseDigest,
		ArmPlanDigest: plan.Digest, AnalysisDesignDigest: design.Digest, DataRole: string(study.RoleTest), RunPolicy: lock.RunPolicy,
		VerificationSidePolicy: heldOutCampaignSidePolicy, RequestBindingPolicy: heldOutCampaignRequestPolicy,
		RepeatKind: RepeatFixed, FixedRepetitions: fixedRepetitions,
		TestCases: lock.TestCases, TestCells: lock.TestCells, SupportedTestCells: lock.SupportedTestCells,
		StructuralUnsupportedTestCells: lock.UnsupportedTestCells, Status: heldOutCampaignStatus,
		ExternalActionStatus: heldOutCampaignExternalStatus,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim: heldOutCampaignSupportedClaim, UnsupportedClaims: slices.Clone(heldOutCampaignUnsupportedClaims),
		},
	}
	for _, arm := range plan.Arms {
		campaignArm, buildErr := buildHeldOutCampaignArm(arm, byArm[arm.ID], fixedRepetitions)
		if buildErr != nil {
			return HeldOutCampaignPlan{}, buildErr
		}
		value.Arms = append(value.Arms, campaignArm)
		switch campaignArm.ExecutionClass {
		case HeldOutExecutionLiveProvider:
			value.ProviderDependentArms++
			value.LiveProviderArms++
			value.ProviderDependentSupportedTestCells += campaignArm.SupportedTestCells
			value.LiveProviderSupportedTestCells += campaignArm.SupportedTestCells
			value.ProviderDependentVerificationInputs += campaignArm.ProviderVerificationInputs
			value.LiveProviderVerificationInputs += campaignArm.ProviderVerificationInputs
			value.PlannedProviderDependentSideRepetitions += campaignArm.PlannedProviderSideRepetitions
			value.PlannedLiveProviderSideRepetitions += campaignArm.PlannedProviderSideRepetitions
		case HeldOutExecutionSealedProviderReplay:
			value.ProviderDependentArms++
			value.SealedReplayArms++
			value.ProviderDependentSupportedTestCells += campaignArm.SupportedTestCells
			value.SealedReplaySupportedTestCells += campaignArm.SupportedTestCells
			value.ProviderDependentVerificationInputs += campaignArm.ProviderVerificationInputs
			value.SealedReplayVerificationInputs += campaignArm.ProviderVerificationInputs
			value.PlannedProviderDependentSideRepetitions += campaignArm.PlannedProviderSideRepetitions
			value.PlannedSealedReplaySideRepetitions += campaignArm.PlannedProviderSideRepetitions
		case HeldOutExecutionDeterministicLocal:
			value.ZeroCostArms++
			value.ZeroCostSupportedTestCells += campaignArm.SupportedTestCells
			value.PlannedZeroCostRepetitions += campaignArm.PlannedZeroCostRepetitions
		default:
			return HeldOutCampaignPlan{}, errors.New("stress held-out campaign arm has an unknown execution class")
		}
	}
	sort.Slice(value.Arms, func(left, right int) bool { return value.Arms[left].ArmID < value.Arms[right].ArmID })
	return value, nil
}

func buildHeldOutCampaignArm(arm ArmDefinition, cells []ArmComparisonCell, fixedRepetitions int) (HeldOutCampaignArm, error) {
	testIDs := make([]string, 0, len(cells))
	supportedIDs := make([]string, 0, len(cells))
	unsupportedIDs := make([]string, 0, len(cells))
	for _, cell := range cells {
		if cell.ArmID != arm.ID {
			return HeldOutCampaignArm{}, errors.New("stress held-out campaign arm received a foreign comparison cell")
		}
		testIDs = append(testIDs, cell.CellID)
		if cell.Support == ArmSupported {
			supportedIDs = append(supportedIDs, cell.CellID)
		} else {
			unsupportedIDs = append(unsupportedIDs, cell.CellID)
		}
	}
	sort.Strings(testIDs)
	sort.Strings(supportedIDs)
	sort.Strings(unsupportedIDs)
	testDigest, err := digestDocument(testIDs)
	if err != nil {
		return HeldOutCampaignArm{}, err
	}
	supportedDigest, err := digestDocument(supportedIDs)
	if err != nil {
		return HeldOutCampaignArm{}, err
	}
	unsupportedDigest, err := digestDocument(unsupportedIDs)
	if err != nil {
		return HeldOutCampaignArm{}, err
	}
	executionClass, err := heldOutExecutionClass(arm)
	if err != nil {
		return HeldOutCampaignArm{}, err
	}
	value := HeldOutCampaignArm{
		ArmID: arm.ID, Kind: arm.Kind, ProviderDependent: arm.ProviderDependent, Entrypoint: arm.Entrypoint,
		ExecutionClass: executionClass,
		EvidencePolicy: arm.EvidencePolicy, TestCellSetDigest: testDigest, SupportedTestCellSetDigest: supportedDigest,
		UnsupportedTestCellSetDigest: unsupportedDigest, TestCells: len(testIDs), SupportedTestCells: len(supportedIDs),
		UnsupportedTestCells: len(unsupportedIDs), ExternalProcessConformanceRequired: arm.ExternalProcessConformanceRequired,
	}
	if arm.ProviderDependent {
		value.ProviderVerificationInputs = value.SupportedTestCells * heldOutProviderSidesPerCell
		value.PlannedProviderSideRepetitions = value.ProviderVerificationInputs * fixedRepetitions
	} else {
		value.PlannedZeroCostRepetitions = value.SupportedTestCells * fixedRepetitions
	}
	return value, nil
}

func validateHeldOutCampaignArms(value HeldOutCampaignPlan) error {
	canonicalArms := canonicalArmDefinitions()
	if len(value.Arms) != len(canonicalArms) {
		return errors.New("stress held-out campaign plan does not contain every canonical arm exactly once")
	}
	providerArms, liveProviderArms, sealedReplayArms, zeroCostArms := 0, 0, 0, 0
	testCells, supported, unsupported := 0, 0, 0
	providerSupported, liveProviderSupported, sealedReplaySupported, zeroCostSupported := 0, 0, 0, 0
	providerInputs, liveProviderInputs, sealedReplayInputs := 0, 0, 0
	providerRepetitions, liveProviderRepetitions, sealedReplayRepetitions, zeroCostRepetitions := 0, 0, 0, 0
	previousID := ""
	for index, arm := range value.Arms {
		if strings.TrimSpace(arm.ArmID) == "" || (previousID != "" && arm.ArmID <= previousID) || !validDigest(arm.TestCellSetDigest) ||
			!validDigest(arm.SupportedTestCellSetDigest) || !validDigest(arm.UnsupportedTestCellSetDigest) || arm.TestCells <= 0 ||
			arm.SupportedTestCells < 0 || arm.UnsupportedTestCells < 0 || arm.TestCells != arm.SupportedTestCells+arm.UnsupportedTestCells {
			return errors.New("stress held-out campaign arm identity, order, cell digests, or counts are invalid")
		}
		canonical := canonicalArms[index]
		executionClass, err := heldOutExecutionClass(canonical)
		if err != nil {
			return err
		}
		if arm.ArmID != canonical.ID || arm.Kind != canonical.Kind || arm.ExecutionClass != executionClass || arm.ProviderDependent != canonical.ProviderDependent ||
			arm.Entrypoint != canonical.Entrypoint || arm.EvidencePolicy != canonical.EvidencePolicy ||
			arm.ExternalProcessConformanceRequired != canonical.ExternalProcessConformanceRequired {
			return errors.New("stress held-out campaign arm differs from its canonical execution surface")
		}
		previousID = arm.ArmID
		if arm.ProviderDependent {
			providerArms++
			providerSupported += arm.SupportedTestCells
			if strings.TrimSpace(arm.Entrypoint) == "" || arm.EvidencePolicy == "" || arm.ProviderVerificationInputs != arm.SupportedTestCells*heldOutProviderSidesPerCell ||
				arm.PlannedProviderSideRepetitions != arm.ProviderVerificationInputs*value.FixedRepetitions || arm.PlannedZeroCostRepetitions != 0 {
				return errors.New("stress held-out provider arm lacks exact verification-input or repetition accounting")
			}
			switch arm.ExecutionClass {
			case HeldOutExecutionLiveProvider:
				liveProviderArms++
				liveProviderSupported += arm.SupportedTestCells
				liveProviderInputs += arm.ProviderVerificationInputs
				liveProviderRepetitions += arm.PlannedProviderSideRepetitions
			case HeldOutExecutionSealedProviderReplay:
				sealedReplayArms++
				sealedReplaySupported += arm.SupportedTestCells
				sealedReplayInputs += arm.ProviderVerificationInputs
				sealedReplayRepetitions += arm.PlannedProviderSideRepetitions
			default:
				return errors.New("stress held-out provider arm has an invalid execution class")
			}
		} else {
			zeroCostArms++
			zeroCostSupported += arm.SupportedTestCells
			if arm.ExecutionClass != HeldOutExecutionDeterministicLocal || arm.Kind != ArmZeroCostControl || arm.Entrypoint != "" || arm.EvidencePolicy != "" || arm.ExternalProcessConformanceRequired ||
				arm.ProviderVerificationInputs != 0 || arm.PlannedProviderSideRepetitions != 0 ||
				arm.PlannedZeroCostRepetitions != arm.SupportedTestCells*value.FixedRepetitions {
				return errors.New("stress held-out zero-cost arm carries provider work or incorrect deterministic repetition accounting")
			}
		}
		testCells += arm.TestCells
		supported += arm.SupportedTestCells
		unsupported += arm.UnsupportedTestCells
		providerInputs += arm.ProviderVerificationInputs
		providerRepetitions += arm.PlannedProviderSideRepetitions
		zeroCostRepetitions += arm.PlannedZeroCostRepetitions
	}
	if providerArms != value.ProviderDependentArms || liveProviderArms != value.LiveProviderArms || sealedReplayArms != value.SealedReplayArms ||
		zeroCostArms != value.ZeroCostArms || testCells != value.TestCells ||
		supported != value.SupportedTestCells || unsupported != value.StructuralUnsupportedTestCells ||
		providerSupported != value.ProviderDependentSupportedTestCells || liveProviderSupported != value.LiveProviderSupportedTestCells ||
		sealedReplaySupported != value.SealedReplaySupportedTestCells || zeroCostSupported != value.ZeroCostSupportedTestCells ||
		providerInputs != value.ProviderDependentVerificationInputs || liveProviderInputs != value.LiveProviderVerificationInputs ||
		sealedReplayInputs != value.SealedReplayVerificationInputs || providerRepetitions != value.PlannedProviderDependentSideRepetitions ||
		liveProviderRepetitions != value.PlannedLiveProviderSideRepetitions || sealedReplayRepetitions != value.PlannedSealedReplaySideRepetitions ||
		zeroCostRepetitions != value.PlannedZeroCostRepetitions {
		return errors.New("stress held-out campaign arm totals differ from aggregate topology")
	}
	return nil
}

func heldOutExecutionClass(arm ArmDefinition) (HeldOutCampaignExecutionClass, error) {
	switch arm.Kind {
	case ArmScoreTokenVerifier, ArmExplicitTextJudge:
		if !arm.ProviderDependent || arm.ExternalProcessConformanceRequired {
			return "", errors.New("stress held-out live arm has an invalid provider or external-process boundary")
		}
		return HeldOutExecutionLiveProvider, nil
	case ArmProtocolAdapter:
		if !arm.ProviderDependent || !arm.ExternalProcessConformanceRequired {
			return "", errors.New("stress held-out protocol arm has an invalid provider or external-process boundary")
		}
		return HeldOutExecutionSealedProviderReplay, nil
	case ArmZeroCostControl:
		if arm.ProviderDependent || arm.ExternalProcessConformanceRequired {
			return "", errors.New("stress held-out zero-cost arm has an invalid provider or external-process boundary")
		}
		return HeldOutExecutionDeterministicLocal, nil
	default:
		return "", errors.New("stress held-out arm kind has no execution class")
	}
}

func heldOutCampaignFixedRepetitions(registry RelationRegistry) (int, error) {
	if err := registry.Validate(); err != nil {
		return 0, err
	}
	fixed := 0
	for _, relation := range registry.Relations {
		if relation.Repeat.Kind != RepeatFixed || relation.Repeat.MinimumRepetitions != relation.Repeat.MaximumRepetitions {
			return 0, fmt.Errorf("stress held-out campaign relation %q is not fixed-repeat", relation.ID)
		}
		if fixed == 0 {
			fixed = relation.Repeat.MaximumRepetitions
		} else if relation.Repeat.MaximumRepetitions != fixed {
			return 0, errors.New("stress held-out campaign relations do not share one fixed repetition count")
		}
	}
	if fixed == 0 {
		return 0, errors.New("stress held-out campaign registry has no repetition contract")
	}
	return fixed, nil
}

func heldOutCampaignPlanDigest(value HeldOutCampaignPlan) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutCampaignSupportedClaim = "the exact held-out arm topology, execution classes, and registered repetition workload are locked provider-free while every live execution binding remains absent"

var heldOutCampaignUnsupportedClaims = []string{
	"provider request count",
	"provider budget",
	"live route currency",
	"execution authorization",
	"held-out execution",
	"empirical verifier reliability",
}
