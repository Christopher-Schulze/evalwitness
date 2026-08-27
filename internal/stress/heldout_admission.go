package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

const (
	HeldOutAdmissionPlanSchemaVersion = "evalwitness.stress-held-out-admission-plan.v1"
	heldOutAdmissionPolicy            = "partition_every_structurally_supported_test_cell_by_registered_human_admission_before_execution_binding"
)

type HeldOutAdmissionEligibility string

const (
	HeldOutAdmissionEligible                   HeldOutAdmissionEligibility = "eligible"
	HeldOutAdmissionMissingHumanResolution     HeldOutAdmissionEligibility = "ineligible_missing_human_resolution"
	HeldOutAdmissionHumanContradicted          HeldOutAdmissionEligibility = "ineligible_human_contradicted"
	HeldOutAdmissionHumanUnresolvedPrimaryCore HeldOutAdmissionEligibility = "ineligible_human_unresolved_primary_core"
)

type HeldOutAdmissionEntry struct {
	RelationID                   string                      `json:"relation_id"`
	RelationDigest               string                      `json:"relation_digest"`
	CaseID                       string                      `json:"case_id"`
	TaskGroupID                  string                      `json:"task_group_id"`
	ManifestDigest               string                      `json:"manifest_digest"`
	Estimand                     Estimand                    `json:"estimand"`
	Admission                    ConstructAdmission          `json:"construct_admission"`
	Eligibility                  HeldOutAdmissionEligibility `json:"eligibility"`
	ExecutionEligible            bool                        `json:"execution_eligible"`
	StructurallySupportedCellIDs []string                    `json:"structurally_supported_cell_ids"`
	LiveProviderCells            int                         `json:"live_provider_cells"`
	SealedReplayCells            int                         `json:"sealed_replay_cells"`
	DeterministicLocalCells      int                         `json:"deterministic_local_cells"`
	Digest                       string                      `json:"digest"`
}

type HeldOutAdmissionClaimBoundary struct {
	SupportedClaim    string   `json:"supported_claim"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

type HeldOutAdmissionPlan struct {
	SchemaVersion                          string                        `json:"schema_version"`
	CanonicalPolicy                        string                        `json:"canonical_policy"`
	CampaignDigest                         string                        `json:"campaign_digest"`
	PartitionDigest                        string                        `json:"partition_digest"`
	AnalysisDesignDigest                   string                        `json:"analysis_design_digest"`
	ArmPlanDigest                          string                        `json:"arm_plan_digest"`
	RegistryDigest                         string                        `json:"registry_digest"`
	CorpusPlanDigest                       string                        `json:"corpus_plan_digest"`
	CorpusAuditDigest                      string                        `json:"corpus_audit_digest"`
	CorpusReleaseDigest                    string                        `json:"corpus_release_digest"`
	RelationPlanDigest                     string                        `json:"relation_plan_digest"`
	PrimarySampleDigest                    string                        `json:"primary_sample_digest"`
	OwnerAttestationDigest                 string                        `json:"owner_attestation_digest"`
	ExpectedOwnerPackageDigest             string                        `json:"expected_owner_package_digest"`
	TerminalLedgerDigest                   string                        `json:"terminal_ledger_digest"`
	DataRole                               string                        `json:"data_role"`
	AdmissionPolicy                        string                        `json:"admission_policy"`
	FixedRepetitions                       int                           `json:"fixed_repetitions"`
	HeldOutCases                           int                           `json:"held_out_cases"`
	RelationCases                          int                           `json:"relation_cases"`
	PrimaryRelationCases                   int                           `json:"primary_relation_cases"`
	SensitivityRelationCases               int                           `json:"sensitivity_relation_cases"`
	PrimarySampleCases                     int                           `json:"primary_sample_cases"`
	PrimarySampleTestCases                 int                           `json:"primary_sample_test_cases"`
	TerminalLedgerCases                    int                           `json:"terminal_ledger_cases"`
	TerminalLedgerTestCases                int                           `json:"terminal_ledger_test_cases"`
	HumanSupportedTestCases                int                           `json:"human_supported_test_cases"`
	HumanContradictedTestCases             int                           `json:"human_contradicted_test_cases"`
	HumanUnresolvedTestCases               int                           `json:"human_unresolved_test_cases"`
	Entries                                []HeldOutAdmissionEntry       `json:"entries"`
	StructurallySupportedCellIDs           []string                      `json:"structurally_supported_cell_ids"`
	ExecutionEligibleCellIDs               []string                      `json:"execution_eligible_cell_ids"`
	PreExecutionIneligibleCellIDs          []string                      `json:"pre_execution_ineligible_cell_ids"`
	StructurallySupportedCells             int                           `json:"structurally_supported_cells"`
	ExecutionEligibleCells                 int                           `json:"execution_eligible_cells"`
	PreExecutionIneligibleCells            int                           `json:"pre_execution_ineligible_cells"`
	EligibleLiveProviderCells              int                           `json:"eligible_live_provider_cells"`
	IneligibleLiveProviderCells            int                           `json:"ineligible_live_provider_cells"`
	EligibleSealedReplayCells              int                           `json:"eligible_sealed_replay_cells"`
	IneligibleSealedReplayCells            int                           `json:"ineligible_sealed_replay_cells"`
	EligibleDeterministicLocalCells        int                           `json:"eligible_deterministic_local_cells"`
	IneligibleDeterministicLocalCells      int                           `json:"ineligible_deterministic_local_cells"`
	EligibleProviderVerificationInputs     int                           `json:"eligible_provider_verification_inputs"`
	EligibleLiveVerificationInputs         int                           `json:"eligible_live_verification_inputs"`
	EligibleSealedReplayVerificationInputs int                           `json:"eligible_sealed_replay_verification_inputs"`
	PlannedEligibleProviderRepetitions     int                           `json:"planned_eligible_provider_repetitions"`
	PlannedEligibleLiveRepetitions         int                           `json:"planned_eligible_live_repetitions"`
	PlannedEligibleSealedReplayRepetitions int                           `json:"planned_eligible_sealed_replay_repetitions"`
	PlannedEligibleLocalRepetitions        int                           `json:"planned_eligible_local_repetitions"`
	ExternalActionStatus                   string                        `json:"external_action_status"`
	RunAuthorized                          bool                          `json:"run_authorized"`
	ExecutionPermitIssued                  bool                          `json:"execution_permit_issued"`
	ProviderCalls                          int                           `json:"provider_calls"`
	EmpiricalUnits                         int                           `json:"empirical_units"`
	NetworkRequired                        bool                          `json:"network_required"`
	ClaimBoundary                          HeldOutAdmissionClaimBoundary `json:"claim_boundary"`
	Digest                                 string                        `json:"digest"`
}

func BuildHeldOutAdmissionPlan(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	corpusPlan mutation.CorpusDevelopmentPlan,
	corpusAudit mutation.CorpusDevelopmentAuditV3,
	corpusRelease mutation.CorpusReleaseV3,
	relationPlan relationevidence.RelationPlanV3,
	primarySample relationevidence.PrimarySampleV3,
	owner relationevidence.OwnerInspectionPublicAttestation,
	expectedOwnerPackageDigest string,
	terminalLedger relationevidence.TerminalRelationLedger,
) (HeldOutAdmissionPlan, error) {
	if err := validateHeldOutAdmissionParents(campaign, lock, design, armPlan, registry, replayed, corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample, owner, expectedOwnerPackageDigest, terminalLedger); err != nil {
		return HeldOutAdmissionPlan{}, err
	}
	value, err := buildHeldOutAdmissionPlan(campaign, lock, design, armPlan, registry, replayed, corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample, owner, expectedOwnerPackageDigest, terminalLedger)
	if err != nil {
		return HeldOutAdmissionPlan{}, err
	}
	value.Digest, err = heldOutAdmissionPlanDigest(value)
	if err != nil {
		return HeldOutAdmissionPlan{}, err
	}
	if err := value.ValidateAgainst(campaign, lock, design, armPlan, registry, replayed, corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample, owner, expectedOwnerPackageDigest, terminalLedger); err != nil {
		return HeldOutAdmissionPlan{}, err
	}
	return value, nil
}

func (value HeldOutAdmissionPlan) Validate() error {
	if value.SchemaVersion != HeldOutAdmissionPlanSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.CampaignDigest) || !validDigest(value.PartitionDigest) || !validDigest(value.AnalysisDesignDigest) ||
		!validDigest(value.ArmPlanDigest) || !validDigest(value.RegistryDigest) || !validDigest(value.CorpusPlanDigest) ||
		!validDigest(value.CorpusAuditDigest) || !validDigest(value.CorpusReleaseDigest) || !validDigest(value.RelationPlanDigest) ||
		!validDigest(value.PrimarySampleDigest) || !validDigest(value.OwnerAttestationDigest) || !validDigest(value.ExpectedOwnerPackageDigest) ||
		!validDigest(value.TerminalLedgerDigest) || value.DataRole != string(study.RoleTest) || value.AdmissionPolicy != heldOutAdmissionPolicy ||
		value.FixedRepetitions <= 0 || value.HeldOutCases != 57 || value.RelationCases != 114 || value.PrimaryRelationCases != 57 ||
		value.SensitivityRelationCases != 57 || value.PrimarySampleCases != 28 || value.PrimarySampleTestCases != 14 ||
		value.TerminalLedgerCases != 28 || value.TerminalLedgerTestCases != 14 || len(value.Entries) != value.RelationCases {
		return errors.New("stress held-out admission identity, parent custody, sample boundary, or relation-case counts are invalid")
	}
	if err := validateHeldOutAdmissionEntries(value); err != nil {
		return err
	}
	if value.StructurallySupportedCells != 440 || value.StructurallySupportedCells != value.ExecutionEligibleCells+value.PreExecutionIneligibleCells ||
		len(value.StructurallySupportedCellIDs) != value.StructurallySupportedCells || len(value.ExecutionEligibleCellIDs) != value.ExecutionEligibleCells ||
		len(value.PreExecutionIneligibleCellIDs) != value.PreExecutionIneligibleCells ||
		value.EligibleProviderVerificationInputs != value.EligibleLiveVerificationInputs+value.EligibleSealedReplayVerificationInputs ||
		value.EligibleLiveVerificationInputs != value.EligibleLiveProviderCells*heldOutProviderSidesPerCell ||
		value.EligibleSealedReplayVerificationInputs != value.EligibleSealedReplayCells*heldOutProviderSidesPerCell ||
		value.EligibleProviderVerificationInputs != (value.EligibleLiveProviderCells+value.EligibleSealedReplayCells)*heldOutProviderSidesPerCell ||
		value.PlannedEligibleProviderRepetitions != value.PlannedEligibleLiveRepetitions+value.PlannedEligibleSealedReplayRepetitions ||
		value.PlannedEligibleProviderRepetitions != value.EligibleProviderVerificationInputs*value.FixedRepetitions ||
		value.PlannedEligibleLiveRepetitions != value.EligibleLiveVerificationInputs*value.FixedRepetitions ||
		value.PlannedEligibleSealedReplayRepetitions != value.EligibleSealedReplayVerificationInputs*value.FixedRepetitions ||
		value.PlannedEligibleLocalRepetitions != value.EligibleDeterministicLocalCells*value.FixedRepetitions {
		return errors.New("stress held-out admission cell partition or eligible workload arithmetic is invalid")
	}
	if value.EligibleLiveProviderCells+value.IneligibleLiveProviderCells != 228 ||
		value.EligibleSealedReplayCells+value.IneligibleSealedReplayCells != 114 ||
		value.EligibleDeterministicLocalCells+value.IneligibleDeterministicLocalCells != 98 {
		return errors.New("stress held-out admission execution-class partition differs from the locked 440-cell topology")
	}
	if value.HumanSupportedTestCases < 0 || value.HumanContradictedTestCases < 0 || value.HumanUnresolvedTestCases < 0 ||
		value.HumanSupportedTestCases+value.HumanContradictedTestCases+value.HumanUnresolvedTestCases != value.TerminalLedgerTestCases {
		return errors.New("stress held-out admission human-state counts do not cover the ledger test cases")
	}
	if value.ExternalActionStatus != heldOutBatchBindingExternalStatus || value.RunAuthorized || value.ExecutionPermitIssued ||
		value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired ||
		value.ClaimBoundary.SupportedClaim != heldOutAdmissionSupportedClaim || !slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutAdmissionUnsupportedClaims) {
		return errors.New("stress held-out admission plan promoted eligibility accounting into execution authority, evidence, or an empirical claim")
	}
	expected, err := heldOutAdmissionPlanDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out admission plan digest is invalid")
	}
	return nil
}

func (value HeldOutAdmissionPlan) ValidateAgainst(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	corpusPlan mutation.CorpusDevelopmentPlan,
	corpusAudit mutation.CorpusDevelopmentAuditV3,
	corpusRelease mutation.CorpusReleaseV3,
	relationPlan relationevidence.RelationPlanV3,
	primarySample relationevidence.PrimarySampleV3,
	owner relationevidence.OwnerInspectionPublicAttestation,
	expectedOwnerPackageDigest string,
	terminalLedger relationevidence.TerminalRelationLedger,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := validateHeldOutAdmissionParents(campaign, lock, design, armPlan, registry, replayed, corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample, owner, expectedOwnerPackageDigest, terminalLedger); err != nil {
		return err
	}
	want, err := buildHeldOutAdmissionPlan(campaign, lock, design, armPlan, registry, replayed, corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample, owner, expectedOwnerPackageDigest, terminalLedger)
	if err != nil {
		return err
	}
	want.Digest, err = heldOutAdmissionPlanDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out admission plan differs from its exact corpus, human ledger, owner custody, or locked test topology")
	}
	return nil
}

func validateHeldOutAdmissionParents(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	corpusPlan mutation.CorpusDevelopmentPlan,
	corpusAudit mutation.CorpusDevelopmentAuditV3,
	corpusRelease mutation.CorpusReleaseV3,
	relationPlan relationevidence.RelationPlanV3,
	primarySample relationevidence.PrimarySampleV3,
	owner relationevidence.OwnerInspectionPublicAttestation,
	expectedOwnerPackageDigest string,
	terminalLedger relationevidence.TerminalRelationLedger,
) error {
	if err := campaign.ValidateAgainst(lock, design, armPlan, registry, replayed); err != nil {
		return err
	}
	if err := registry.ValidateAgainst(corpusPlan, corpusAudit, corpusRelease); err != nil {
		return err
	}
	wantPlan, err := relationevidence.BuildPlanV3(corpusPlan, corpusAudit, corpusRelease)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(relationPlan, wantPlan) {
		return errors.New("stress held-out admission relation plan differs from the exact v3 corpus governance plan")
	}
	wantSample, err := relationevidence.BuildPrimarySampleV3(relationPlan, corpusRelease)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(primarySample, wantSample) {
		return errors.New("stress held-out admission primary sample differs from the exact frozen v3 selection")
	}
	if err := validateOwnerCustody(owner, expectedOwnerPackageDigest); err != nil {
		return err
	}
	if err := validateHeldOutTerminalLedger(relationPlan, primarySample, corpusRelease, terminalLedger); err != nil {
		return err
	}
	return nil
}

func validateHeldOutTerminalLedger(
	relationPlan relationevidence.RelationPlanV3,
	primarySample relationevidence.PrimarySampleV3,
	corpusRelease mutation.CorpusReleaseV3,
	terminalLedger relationevidence.TerminalRelationLedger,
) error {
	if err := terminalLedger.Validate(); err != nil {
		return &AdmissionError{State: InvalidCustody, Reason: err.Error()}
	}
	if terminalLedger.ProtocolVersion != relationevidence.ProtocolVersionV3 || terminalLedger.Objective != relationevidence.ReviewObjectiveControlledRelation ||
		terminalLedger.PlanDigest != relationPlan.Digest || terminalLedger.SampleDigest != primarySample.Digest || terminalLedger.DataRole != relationevidence.ReviewDataPrimaryAudit {
		return &AdmissionError{State: InvalidCustody, Reason: "terminal ledger does not bind the exact v3 relation plan, primary sample, objective, and audit role"}
	}
	releaseCases := make(map[string]mutation.CorpusCaseV3, len(corpusRelease.Cases))
	for _, item := range corpusRelease.Cases {
		releaseCases[item.ID] = item
	}
	sampleCases := make(map[string]relationevidence.GovernedCaseReferenceV3, len(primarySample.Cases))
	for _, reference := range primarySample.Cases {
		if _, exists := sampleCases[reference.CaseID]; exists {
			return &AdmissionError{State: InvalidCustody, Reason: "primary sample repeats a case identity"}
		}
		sampleCases[reference.CaseID] = reference
	}
	if len(terminalLedger.Entries) != len(sampleCases) {
		return &AdmissionError{State: InvalidCustody, Reason: "terminal ledger does not cover the exact primary-sample case set"}
	}
	seen := make(map[string]struct{}, len(terminalLedger.Entries))
	for _, entry := range terminalLedger.Entries {
		reference, sampled := sampleCases[entry.CaseID]
		item, released := releaseCases[entry.CaseID]
		if !sampled || !released || reference.Family != item.Family || reference.DataRole != item.Split ||
			reference.TaskGroupID != item.Manifest.SplitGroupID || reference.ConstructFirewallDigest != item.ConstructFirewall.Digest {
			return &AdmissionError{State: InvalidCrossVersion, Reason: "terminal ledger entry is outside or differs from the exact released primary sample"}
		}
		if _, duplicate := seen[entry.CaseID]; duplicate {
			return &AdmissionError{State: InvalidCustody, Reason: "terminal ledger repeats a primary-sample case"}
		}
		seen[entry.CaseID] = struct{}{}
		if _, err := verifiedLedgerEntry(terminalLedger, item); err != nil {
			return err
		}
	}
	return nil
}

func buildHeldOutAdmissionPlan(
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	corpusPlan mutation.CorpusDevelopmentPlan,
	corpusAudit mutation.CorpusDevelopmentAuditV3,
	corpusRelease mutation.CorpusReleaseV3,
	relationPlan relationevidence.RelationPlanV3,
	primarySample relationevidence.PrimarySampleV3,
	owner relationevidence.OwnerInspectionPublicAttestation,
	expectedOwnerPackageDigest string,
	terminalLedger relationevidence.TerminalRelationLedger,
) (HeldOutAdmissionPlan, error) {
	value := HeldOutAdmissionPlan{
		SchemaVersion: HeldOutAdmissionPlanSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		CampaignDigest: campaign.Digest, PartitionDigest: lock.Digest, AnalysisDesignDigest: design.Digest,
		ArmPlanDigest: armPlan.Digest, RegistryDigest: registry.Digest, CorpusPlanDigest: corpusPlan.Digest,
		CorpusAuditDigest: corpusAudit.Digest, CorpusReleaseDigest: corpusRelease.Digest, RelationPlanDigest: relationPlan.Digest,
		PrimarySampleDigest: primarySample.Digest, OwnerAttestationDigest: owner.Digest,
		ExpectedOwnerPackageDigest: expectedOwnerPackageDigest, TerminalLedgerDigest: terminalLedger.Digest,
		DataRole: string(study.RoleTest), AdmissionPolicy: heldOutAdmissionPolicy, FixedRepetitions: campaign.FixedRepetitions,
		HeldOutCases: lock.TestCases, PrimarySampleCases: len(primarySample.Cases), TerminalLedgerCases: len(terminalLedger.Entries),
		ExternalActionStatus: heldOutBatchBindingExternalStatus,
		ClaimBoundary: HeldOutAdmissionClaimBoundary{
			SupportedClaim: heldOutAdmissionSupportedClaim, UnsupportedClaims: slices.Clone(heldOutAdmissionUnsupportedClaims),
		},
	}
	relationByID := make(map[string]Relation, len(registry.Relations))
	for _, spec := range registry.Relations {
		relationByID[spec.ID] = spec
	}
	releaseByID := make(map[string]mutation.CorpusCaseV3, len(corpusRelease.Cases))
	for _, item := range corpusRelease.Cases {
		releaseByID[item.ID] = item
	}
	sampleTestCases := make(map[string]struct{}, 14)
	for _, reference := range primarySample.Cases {
		if reference.DataRole == study.RoleTest {
			sampleTestCases[reference.CaseID] = struct{}{}
		}
	}
	value.PrimarySampleTestCases = len(sampleTestCases)
	for _, entry := range terminalLedger.Entries {
		if _, heldOut := sampleTestCases[entry.CaseID]; !heldOut {
			continue
		}
		value.TerminalLedgerTestCases++
		switch entry.HumanState {
		case relationevidence.TranslationSupports:
			value.HumanSupportedTestCases++
		case relationevidence.TranslationContradicts:
			value.HumanContradictedTestCases++
		case relationevidence.TranslationUnresolved:
			value.HumanUnresolvedTestCases++
		}
	}
	armClass := make(map[string]HeldOutCampaignExecutionClass, len(campaign.Arms))
	for _, arm := range campaign.Arms {
		armClass[arm.ArmID] = arm.ExecutionClass
	}
	supportedCells := stringSet(lock.SupportedTestCellIDs)
	cellsByRelationCase := make(map[string][]ArmComparisonCell, lock.SupportedTestCells)
	for _, cell := range armPlan.Cells {
		if _, supported := supportedCells[cell.CellID]; !supported {
			continue
		}
		key := heldOutRelationCaseKey(cell.RelationID, cell.CaseID)
		cellsByRelationCase[key] = append(cellsByRelationCase[key], cell)
	}
	for _, replay := range replayed {
		if replay.Split != study.RoleTest {
			continue
		}
		item, exists := releaseByID[replay.CaseID]
		if !exists || replay.ManifestDigest != item.Manifest.Digest || replay.TaskGroupID != item.Manifest.SplitGroupID {
			return HeldOutAdmissionPlan{}, errors.New("stress held-out admission replay differs from its released test case")
		}
		for _, relationID := range replay.RelationIDs {
			spec, exists := relationByID[relationID]
			if !exists {
				return HeldOutAdmissionPlan{}, fmt.Errorf("stress held-out admission lacks registered relation %q", relationID)
			}
			var ledger *relationevidence.TerminalRelationLedger
			if _, sampled := sampleTestCases[item.ID]; sampled {
				ledger = &terminalLedger
			}
			admission, err := AdmitMutationCase(spec, item, owner, expectedOwnerPackageDigest, ledger)
			if err != nil {
				return HeldOutAdmissionPlan{}, fmt.Errorf("admit held-out relation %q case %q: %w", relationID, item.ID, err)
			}
			eligible, eligibility, err := heldOutAdmissionEligibility(spec.StatisticalFamily.Estimand, admission)
			if err != nil {
				return HeldOutAdmissionPlan{}, err
			}
			entry := HeldOutAdmissionEntry{
				RelationID: spec.ID, RelationDigest: spec.Digest, CaseID: item.ID, TaskGroupID: replay.TaskGroupID,
				ManifestDigest: replay.ManifestDigest, Estimand: spec.StatisticalFamily.Estimand, Admission: admission,
				Eligibility: eligibility, ExecutionEligible: eligible,
			}
			for _, cell := range cellsByRelationCase[heldOutRelationCaseKey(spec.ID, item.ID)] {
				entry.StructurallySupportedCellIDs = append(entry.StructurallySupportedCellIDs, cell.CellID)
				switch armClass[cell.ArmID] {
				case HeldOutExecutionLiveProvider:
					entry.LiveProviderCells++
				case HeldOutExecutionSealedProviderReplay:
					entry.SealedReplayCells++
				case HeldOutExecutionDeterministicLocal:
					entry.DeterministicLocalCells++
				default:
					return HeldOutAdmissionPlan{}, fmt.Errorf("stress held-out admission cell %q has no campaign execution class", cell.CellID)
				}
			}
			sort.Strings(entry.StructurallySupportedCellIDs)
			if len(entry.StructurallySupportedCellIDs) == 0 {
				return HeldOutAdmissionPlan{}, fmt.Errorf("stress held-out admission relation %q case %q has no structurally supported cells", spec.ID, item.ID)
			}
			entry.Digest, err = heldOutAdmissionEntryDigest(entry)
			if err != nil {
				return HeldOutAdmissionPlan{}, err
			}
			value.Entries = append(value.Entries, entry)
			value.RelationCases++
			switch spec.StatisticalFamily.Estimand {
			case EstimandPrimaryCore:
				value.PrimaryRelationCases++
			case EstimandSensitivity:
				value.SensitivityRelationCases++
			}
		}
	}
	sort.Slice(value.Entries, func(left, right int) bool {
		if value.Entries[left].RelationID != value.Entries[right].RelationID {
			return value.Entries[left].RelationID < value.Entries[right].RelationID
		}
		return value.Entries[left].CaseID < value.Entries[right].CaseID
	})
	for _, entry := range value.Entries {
		value.StructurallySupportedCellIDs = append(value.StructurallySupportedCellIDs, entry.StructurallySupportedCellIDs...)
		if entry.ExecutionEligible {
			value.ExecutionEligibleCellIDs = append(value.ExecutionEligibleCellIDs, entry.StructurallySupportedCellIDs...)
			value.EligibleLiveProviderCells += entry.LiveProviderCells
			value.EligibleSealedReplayCells += entry.SealedReplayCells
			value.EligibleDeterministicLocalCells += entry.DeterministicLocalCells
		} else {
			value.PreExecutionIneligibleCellIDs = append(value.PreExecutionIneligibleCellIDs, entry.StructurallySupportedCellIDs...)
			value.IneligibleLiveProviderCells += entry.LiveProviderCells
			value.IneligibleSealedReplayCells += entry.SealedReplayCells
			value.IneligibleDeterministicLocalCells += entry.DeterministicLocalCells
		}
	}
	sort.Strings(value.StructurallySupportedCellIDs)
	sort.Strings(value.ExecutionEligibleCellIDs)
	sort.Strings(value.PreExecutionIneligibleCellIDs)
	value.StructurallySupportedCells = len(value.StructurallySupportedCellIDs)
	value.ExecutionEligibleCells = len(value.ExecutionEligibleCellIDs)
	value.PreExecutionIneligibleCells = len(value.PreExecutionIneligibleCellIDs)
	value.EligibleLiveVerificationInputs = value.EligibleLiveProviderCells * heldOutProviderSidesPerCell
	value.EligibleSealedReplayVerificationInputs = value.EligibleSealedReplayCells * heldOutProviderSidesPerCell
	value.EligibleProviderVerificationInputs = value.EligibleLiveVerificationInputs + value.EligibleSealedReplayVerificationInputs
	value.PlannedEligibleLiveRepetitions = value.EligibleLiveVerificationInputs * value.FixedRepetitions
	value.PlannedEligibleSealedReplayRepetitions = value.EligibleSealedReplayVerificationInputs * value.FixedRepetitions
	value.PlannedEligibleProviderRepetitions = value.PlannedEligibleLiveRepetitions + value.PlannedEligibleSealedReplayRepetitions
	value.PlannedEligibleLocalRepetitions = value.EligibleDeterministicLocalCells * value.FixedRepetitions
	return value, nil
}

func validateHeldOutAdmissionEntries(value HeldOutAdmissionPlan) error {
	structural := make([]string, 0, value.StructurallySupportedCells)
	eligible := make([]string, 0, value.ExecutionEligibleCells)
	ineligible := make([]string, 0, value.PreExecutionIneligibleCells)
	primary, sensitivity := 0, 0
	eligibleLive, ineligibleLive, eligibleReplay, ineligibleReplay, eligibleLocal, ineligibleLocal := 0, 0, 0, 0, 0, 0
	previousRelation, previousCase := "", ""
	seenCases := make(map[string]struct{}, len(value.Entries))
	for _, entry := range value.Entries {
		if previousRelation != "" && (entry.RelationID < previousRelation || entry.RelationID == previousRelation && entry.CaseID <= previousCase) {
			return errors.New("stress held-out admission entries must be unique and relation-case sorted")
		}
		previousRelation, previousCase = entry.RelationID, entry.CaseID
		key := heldOutRelationCaseKey(entry.RelationID, entry.CaseID)
		if _, duplicate := seenCases[key]; duplicate {
			return errors.New("stress held-out admission entries repeat a relation-case")
		}
		seenCases[key] = struct{}{}
		if err := entry.Validate(); err != nil {
			return err
		}
		structural = append(structural, entry.StructurallySupportedCellIDs...)
		if entry.ExecutionEligible {
			eligible = append(eligible, entry.StructurallySupportedCellIDs...)
			eligibleLive += entry.LiveProviderCells
			eligibleReplay += entry.SealedReplayCells
			eligibleLocal += entry.DeterministicLocalCells
		} else {
			ineligible = append(ineligible, entry.StructurallySupportedCellIDs...)
			ineligibleLive += entry.LiveProviderCells
			ineligibleReplay += entry.SealedReplayCells
			ineligibleLocal += entry.DeterministicLocalCells
		}
		switch entry.Estimand {
		case EstimandPrimaryCore:
			primary++
		case EstimandSensitivity:
			sensitivity++
		default:
			return errors.New("stress held-out admission entry entered an unregistered held-out estimand")
		}
	}
	sort.Strings(structural)
	sort.Strings(eligible)
	sort.Strings(ineligible)
	if !uniqueSortedStrings(structural) || !uniqueSortedStrings(eligible) || !uniqueSortedStrings(ineligible) ||
		!slices.Equal(structural, value.StructurallySupportedCellIDs) || !slices.Equal(eligible, value.ExecutionEligibleCellIDs) ||
		!slices.Equal(ineligible, value.PreExecutionIneligibleCellIDs) || primary != value.PrimaryRelationCases || sensitivity != value.SensitivityRelationCases ||
		eligibleLive != value.EligibleLiveProviderCells || ineligibleLive != value.IneligibleLiveProviderCells ||
		eligibleReplay != value.EligibleSealedReplayCells || ineligibleReplay != value.IneligibleSealedReplayCells ||
		eligibleLocal != value.EligibleDeterministicLocalCells || ineligibleLocal != value.IneligibleDeterministicLocalCells {
		return errors.New("stress held-out admission entries do not reproduce the exact cell, estimand, or execution-class partitions")
	}
	return nil
}

func (value HeldOutAdmissionEntry) Validate() error {
	if !identifierPattern.MatchString(value.RelationID) || !validDigest(value.RelationDigest) || !identifierPattern.MatchString(value.CaseID) ||
		!identifierPattern.MatchString(value.TaskGroupID) || !validDigest(value.ManifestDigest) || len(value.StructurallySupportedCellIDs) == 0 ||
		!slices.IsSorted(value.StructurallySupportedCellIDs) || !uniqueSortedStrings(value.StructurallySupportedCellIDs) ||
		value.LiveProviderCells < 0 || value.SealedReplayCells < 0 || value.DeterministicLocalCells < 0 ||
		value.LiveProviderCells+value.SealedReplayCells+value.DeterministicLocalCells != len(value.StructurallySupportedCellIDs) {
		return errors.New("stress held-out admission entry identity, supported cells, or execution-class counts are invalid")
	}
	if err := value.Admission.Validate(); err != nil {
		return err
	}
	if value.Admission.CaseID != value.CaseID {
		return errors.New("stress held-out admission entry and construct admission disagree on case identity")
	}
	eligible, eligibility, err := heldOutAdmissionEligibility(value.Estimand, value.Admission)
	if err != nil {
		return err
	}
	if value.ExecutionEligible != eligible || value.Eligibility != eligibility {
		return errors.New("stress held-out admission entry eligibility contradicts its registered estimand and construct admission")
	}
	expected, err := heldOutAdmissionEntryDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out admission entry digest is invalid")
	}
	return nil
}

func heldOutAdmissionEligibility(estimand Estimand, admission ConstructAdmission) (bool, HeldOutAdmissionEligibility, error) {
	switch estimand {
	case EstimandPrimaryCore:
		if admission.PrimaryEligible {
			return true, HeldOutAdmissionEligible, nil
		}
		switch admission.Status {
		case AdmissionFormalOnly:
			return false, HeldOutAdmissionMissingHumanResolution, nil
		case AdmissionHumanContradicted:
			return false, HeldOutAdmissionHumanContradicted, nil
		case AdmissionHumanUnresolved:
			return false, HeldOutAdmissionHumanUnresolvedPrimaryCore, nil
		default:
			return false, "", errors.New("stress primary-core admission has an inconsistent eligibility state")
		}
	case EstimandSensitivity:
		if admission.SensitivityEligible {
			return true, HeldOutAdmissionEligible, nil
		}
		if admission.Status == AdmissionHumanContradicted {
			return false, HeldOutAdmissionHumanContradicted, nil
		}
		return false, "", errors.New("stress sensitivity admission has an inconsistent eligibility state")
	default:
		return false, "", errors.New("stress held-out admission supports only primary-core and sensitivity estimands")
	}
}

func uniqueSortedStrings(values []string) bool {
	for index, value := range values {
		if value == "" || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func heldOutRelationCaseKey(relationID, caseID string) string {
	return relationID + "\x00" + caseID
}

func heldOutAdmissionEntryDigest(value HeldOutAdmissionEntry) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func heldOutAdmissionPlanDigest(value HeldOutAdmissionPlan) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutAdmissionSupportedClaim = "the exact 440 structurally supported held-out cells are partitioned by the frozen v3 human-admission contract before any execution binding or provider call"

var heldOutAdmissionUnsupportedClaims = []string{
	"execution authorization",
	"provider request binding",
	"provider budget",
	"live route currency",
	"held-out execution",
	"empirical verifier reliability",
}
