package stress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
)

const (
	HeldOutExecutionLedgerSchemaVersion = "evalwitness.stress-held-out-execution-ledger.v1"
	heldOutExecutionLedgerStatus        = "admission_filtered_execution_complete"
	heldOutExecutionLedgerPolicy        = "execute_every_admitted_supported_cell_once_and_retain_every_excluded_or_unsupported_cell"
)

type HeldOutExecutionEvidenceAuthority string

const (
	HeldOutEvidenceLiveProviderReservation HeldOutExecutionEvidenceAuthority = "live_provider_reservation"
	HeldOutEvidenceSealedProviderReplay    HeldOutExecutionEvidenceAuthority = "sealed_provider_replay"
	HeldOutEvidenceDeterministicLocal      HeldOutExecutionEvidenceAuthority = "deterministic_local"
)

type HeldOutExecutionArmLedger struct {
	ArmID                        string                            `json:"arm_id"`
	ExecutionClass               HeldOutCampaignExecutionClass     `json:"execution_class"`
	EvidenceAuthority            HeldOutExecutionEvidenceAuthority `json:"evidence_authority"`
	EligibleCellSetDigest        string                            `json:"eligible_cell_set_digest"`
	EligibleCells                int                               `json:"eligible_cells"`
	ExecutedCells                int                               `json:"executed_cells"`
	PreExecutionIneligibleCells  int                               `json:"pre_execution_ineligible_cells"`
	StructuralUnsupportedCells   int                               `json:"structural_unsupported_cells"`
	ProviderCalls                int                               `json:"provider_calls"`
	PermitDigest                 string                            `json:"permit_digest,omitempty"`
	ReservationDigest            string                            `json:"reservation_digest,omitempty"`
	LiveBatchEvidenceDigest      string                            `json:"live_batch_evidence_digest,omitempty"`
	LiveReplayVerificationDigest string                            `json:"live_replay_verification_digest,omitempty"`
	ReplayEvidenceSetDigest      string                            `json:"replay_evidence_set_digest,omitempty"`
	ReplaySourceArmID            string                            `json:"replay_source_arm_id,omitempty"`
	ReplaySourceCellSetDigest    string                            `json:"replay_source_cell_set_digest,omitempty"`
}

type HeldOutExecutionLedger struct {
	SchemaVersion                string                       `json:"schema_version"`
	CanonicalPolicy              string                       `json:"canonical_policy"`
	PartitionDigest              string                       `json:"partition_digest"`
	CampaignDigest               string                       `json:"campaign_digest"`
	AdmissionPlanDigest          string                       `json:"admission_plan_digest"`
	ExecutionBatchBindingDigest  string                       `json:"execution_batch_binding_digest"`
	PermitDigest                 string                       `json:"permit_digest"`
	ReservationDigest            string                       `json:"reservation_digest"`
	ArmReportDigest              string                       `json:"arm_report_digest"`
	AnalysisReportDigest         string                       `json:"analysis_report_digest"`
	AnalysisCompletionStatus     string                       `json:"analysis_completion_status"`
	ExecutionPolicy              string                       `json:"execution_policy"`
	Arms                         []HeldOutExecutionArmLedger  `json:"arms"`
	TestCells                    int                          `json:"test_cells"`
	StructurallySupportedCells   int                          `json:"structurally_supported_cells"`
	StructuralUnsupportedCells   int                          `json:"structural_unsupported_cells"`
	ExecutionEligibleCells       int                          `json:"execution_eligible_cells"`
	ExecutedCells                int                          `json:"executed_cells"`
	PreExecutionIneligibleCells  int                          `json:"pre_execution_ineligible_cells"`
	ProviderCalls                int                          `json:"provider_calls"`
	Status                       string                       `json:"status"`
	ExecutedEvidenceUnits        int                          `json:"executed_evidence_units"`
	LiveResponseEvidenceObserved bool                         `json:"live_response_evidence_observed"`
	ClaimBoundary                HeldOutCampaignClaimBoundary `json:"claim_boundary"`
	Digest                       string                       `json:"digest"`
}

func BuildHeldOutExecutionLedger(
	lock HeldOutPartitionLock,
	campaign HeldOutCampaignPlan,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
	liveEvidence []HeldOutLiveBatchEvidence,
	liveReplayVerifications []HeldOutLiveReplayVerification,
	armReport ArmComparisonReport,
	analysis StressAnalysisReport,
) (HeldOutExecutionLedger, error) {
	value, err := buildHeldOutExecutionLedger(lock, campaign, admission, execution, permit, reservation, liveEvidence, liveReplayVerifications, armReport, analysis)
	if err != nil {
		return HeldOutExecutionLedger{}, err
	}
	value.Digest, err = heldOutExecutionLedgerDigest(value)
	if err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if err := value.ValidateAgainst(lock, campaign, admission, execution, permit, reservation, liveEvidence, liveReplayVerifications, armReport, analysis); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	return value, nil
}

func (value HeldOutExecutionLedger) Validate() error {
	if value.SchemaVersion != HeldOutExecutionLedgerSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.PartitionDigest) || !validDigest(value.CampaignDigest) || !validDigest(value.AdmissionPlanDigest) ||
		!validDigest(value.ExecutionBatchBindingDigest) || !validDigest(value.PermitDigest) || !validDigest(value.ReservationDigest) ||
		!validDigest(value.ArmReportDigest) || !validDigest(value.AnalysisReportDigest) || value.AnalysisCompletionStatus != "incomplete_due_to_pre_execution_exclusions" ||
		value.ExecutionPolicy != heldOutExecutionLedgerPolicy ||
		value.Status != heldOutExecutionLedgerStatus || value.ExecutedEvidenceUnits != value.ExecutedCells || !value.LiveResponseEvidenceObserved ||
		value.TestCells <= 0 || value.StructurallySupportedCells <= 0 || value.ExecutionEligibleCells <= 0 ||
		value.StructuralUnsupportedCells < 0 || value.PreExecutionIneligibleCells < 0 || value.ProviderCalls < 0 ||
		value.TestCells != value.StructurallySupportedCells+value.StructuralUnsupportedCells ||
		value.StructurallySupportedCells != value.ExecutionEligibleCells+value.PreExecutionIneligibleCells ||
		value.ExecutedCells != value.ExecutionEligibleCells || value.ClaimBoundary.SupportedClaim != heldOutExecutionLedgerSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutExecutionLedgerUnsupportedClaims) {
		return errors.New("stress held-out execution ledger identity, denominator, execution, or claim boundary is invalid")
	}
	if err := validateHeldOutExecutionLedgerArms(value); err != nil {
		return err
	}
	expected, err := heldOutExecutionLedgerDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out execution ledger digest is invalid")
	}
	return nil
}

func (value HeldOutExecutionLedger) ValidateAgainst(
	lock HeldOutPartitionLock,
	campaign HeldOutCampaignPlan,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
	liveEvidence []HeldOutLiveBatchEvidence,
	liveReplayVerifications []HeldOutLiveReplayVerification,
	armReport ArmComparisonReport,
	analysis StressAnalysisReport,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	want, err := buildHeldOutExecutionLedger(lock, campaign, admission, execution, permit, reservation, liveEvidence, liveReplayVerifications, armReport, analysis)
	if err != nil {
		return err
	}
	want.Digest, err = heldOutExecutionLedgerDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out execution ledger differs from its admission, execution, permit, reservation, report, or analysis parents")
	}
	return nil
}

func DecodeHeldOutExecutionLedger(reader io.Reader) (HeldOutExecutionLedger, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutExecutionLedger{}, fmt.Errorf("read stress held-out execution ledger: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutExecutionLedger{}, errors.New("stress held-out execution ledger exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutExecutionLedger
	if err := decoder.Decode(&value); err != nil {
		return HeldOutExecutionLedger{}, fmt.Errorf("decode stress held-out execution ledger: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutExecutionLedger{}, errors.New("stress held-out execution ledger has trailing JSON")
	}
	return value, value.Validate()
}

func buildHeldOutExecutionLedger(
	lock HeldOutPartitionLock,
	campaign HeldOutCampaignPlan,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
	liveEvidence []HeldOutLiveBatchEvidence,
	liveReplayVerifications []HeldOutLiveReplayVerification,
	armReport ArmComparisonReport,
	analysis StressAnalysisReport,
) (HeldOutExecutionLedger, error) {
	if err := lock.Validate(); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if err := campaign.Validate(); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if err := admission.Validate(); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if err := execution.Validate(); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	reservedAt, err := parseHeldOutExecutionPermitTime(reservation.ReservedAt)
	if err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if err := reservation.ValidateAgainst(permit, reservedAt); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if err := armReport.Validate(); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if err := validateStressAnalysisArtifact(analysis, lock.AnalysisDesignDigest, armReport.Digest); err != nil {
		return HeldOutExecutionLedger{}, err
	}
	if campaign.PartitionDigest != lock.Digest || admission.CampaignDigest != campaign.Digest ||
		admission.PartitionDigest != lock.Digest || execution.CampaignDigest != campaign.Digest ||
		execution.AdmissionPlanDigest != admission.Digest || execution.PartitionDigest != lock.Digest ||
		permit.CampaignDigest != campaign.Digest || permit.AdmissionPlanDigest != admission.Digest ||
		permit.ExecutionBatchBindingDigest != execution.Digest || permit.PartitionDigest != lock.Digest ||
		analysis.ArmReportDigest != armReport.Digest {
		return HeldOutExecutionLedger{}, errors.New("stress held-out execution ledger parents do not share one campaign, admission, execution, and report lineage")
	}
	eligible := stringSet(admission.ExecutionEligibleCellIDs)
	ineligible := stringSet(admission.PreExecutionIneligibleCellIDs)
	testCells := stringSet(lock.TestCellIDs)
	structuralUnsupported := stringSet(lock.UnsupportedTestCellIDs)
	liveByArm := make(map[string]HeldOutLiveBatchEvidence, len(liveEvidence))
	for _, evidence := range liveEvidence {
		if _, duplicate := liveByArm[evidence.ArmID]; duplicate {
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger repeats live evidence for arm %q", evidence.ArmID)
		}
		binding, exists := heldOutExecutionArmBindingByID(execution.Arms, evidence.ArmID)
		if !exists || evidence.ValidateAgainst(binding, permit, reservation) != nil {
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger live evidence for arm %q differs from its exact execution parents", evidence.ArmID)
		}
		liveByArm[evidence.ArmID] = evidence
	}
	if len(liveByArm) != execution.LiveBatchBindings {
		return HeldOutExecutionLedger{}, errors.New("stress held-out execution ledger lacks one exact live-evidence artifact per permitted arm")
	}
	liveReplayByArm := make(map[string]HeldOutLiveReplayVerification, len(liveReplayVerifications))
	for _, verification := range liveReplayVerifications {
		if err := verification.Validate(); err != nil {
			return HeldOutExecutionLedger{}, err
		}
		live, exists := liveByArm[verification.ArmID]
		binding, bindingExists := heldOutExecutionArmBindingByID(execution.Arms, verification.ArmID)
		if !exists || verification.AdmissionPlanDigest != admission.Digest || verification.LiveBatchEvidenceDigest != live.Digest ||
			verification.EligibleCellSetDigest != live.EligibleCellSetDigest || verification.EligibleCells != live.EligibleCells ||
			!bindingExists || verification.RouteID != binding.Batch.RouteID || verification.ReplaySource.RouteID != binding.Batch.RouteID {
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out live replay verification for arm %q differs from its admission or live-capture parent", verification.ArmID)
		}
		if _, duplicate := liveReplayByArm[verification.ArmID]; duplicate {
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger repeats live replay verification for arm %q", verification.ArmID)
		}
		liveReplayByArm[verification.ArmID] = verification
	}
	if len(liveReplayByArm) != execution.LiveBatchBindings {
		return HeldOutExecutionLedger{}, errors.New("stress held-out execution ledger lacks one exact replay verification per live arm")
	}
	arms := make(map[string]*HeldOutExecutionArmLedger, len(campaign.Arms))
	providerCalls := 0
	for _, campaignArm := range campaign.Arms {
		authority, authorityErr := heldOutExecutionAuthorityForArm(campaignArm)
		if authorityErr != nil {
			return HeldOutExecutionLedger{}, authorityErr
		}
		arm := &HeldOutExecutionArmLedger{ArmID: campaignArm.ArmID, ExecutionClass: campaignArm.ExecutionClass, EvidenceAuthority: authority}
		if authority == HeldOutEvidenceLiveProviderReservation {
			arm.PermitDigest, arm.ReservationDigest = permit.Digest, reservation.Digest
			live, exists := liveByArm[campaignArm.ArmID]
			if !exists {
				return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger lacks live evidence for arm %q", campaignArm.ArmID)
			}
			replayVerification, replayExists := liveReplayByArm[campaignArm.ArmID]
			if !replayExists {
				return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger lacks exact replay verification for arm %q", campaignArm.ArmID)
			}
			arm.ProviderCalls = live.ProviderCalls
			arm.LiveBatchEvidenceDigest = live.Digest
			arm.LiveReplayVerificationDigest = replayVerification.Digest
			arm.ReplayEvidenceSetDigest = replayVerification.ReplayEvidenceSetDigest
			providerCalls += live.ProviderCalls
		}
		if authority == HeldOutEvidenceSealedProviderReplay {
			arm.ReplaySourceArmID = execution.Replay.SourceArmID
			source, exists := heldOutExecutionArmBindingByID(execution.Arms, execution.Replay.SourceArmID)
			if !exists {
				return HeldOutExecutionLedger{}, errors.New("stress held-out execution ledger lacks its sealed-replay source arm")
			}
			arm.ReplaySourceCellSetDigest = source.EligibleTestCellSetDigest
		}
		arms[campaignArm.ArmID] = arm
	}
	armCellIDs := make(map[string][]string, len(arms))
	heldOutCellsSeen := 0
	for _, cell := range armReport.Cells {
		if _, heldOut := testCells[cell.CellID]; !heldOut {
			continue
		}
		heldOutCellsSeen++
		arm := arms[cell.ArmID]
		if arm == nil {
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger cell %q has an unknown arm", cell.CellID)
		}
		switch {
		case cell.Support == ArmUnsupported:
			if _, expected := structuralUnsupported[cell.CellID]; !expected || cell.Status != ArmCellUnsupported {
				return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger structural cell %q changed support status", cell.CellID)
			}
			arm.StructuralUnsupportedCells++
		case hasString(ineligible, cell.CellID):
			if cell.Status != ArmCellNotRun {
				return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger ineligible cell %q contains execution evidence", cell.CellID)
			}
			arm.PreExecutionIneligibleCells++
		case hasString(eligible, cell.CellID):
			if cell.Status != ArmCellExecuted {
				return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger eligible cell %q was not executed", cell.CellID)
			}
			arm.EligibleCells++
			arm.ExecutedCells++
			armCellIDs[cell.ArmID] = append(armCellIDs[cell.ArmID], cell.CellID)
		default:
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger supported cell %q is outside the admission partition", cell.CellID)
		}
	}
	for armID, live := range liveByArm {
		reportCellIDs := slices.Clone(armCellIDs[armID])
		sort.Strings(reportCellIDs)
		liveCellIDs := make([]string, len(live.Cells))
		for index, cell := range live.Cells {
			liveCellIDs[index] = cell.CellID
		}
		if !slices.Equal(reportCellIDs, liveCellIDs) {
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger arm %q report cells differ from its live evidence", armID)
		}
		verification := liveReplayByArm[armID]
		reportReplayDigests := make([]string, 0, len(reportCellIDs))
		for _, cell := range armReport.Cells {
			if cell.ArmID == armID && cell.Status == ArmCellExecuted {
				reportReplayDigests = append(reportReplayDigests, cell.EvidenceDigest)
			}
		}
		sort.Strings(reportReplayDigests)
		reportReplaySetDigest, digestErr := digestDocument(reportReplayDigests)
		if digestErr != nil || reportReplaySetDigest != verification.ReplayEvidenceSetDigest {
			return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger arm %q report evidence differs from its independently verified exact replay set", armID)
		}
	}
	if heldOutCellsSeen != lock.TestCells {
		return HeldOutExecutionLedger{}, fmt.Errorf("stress held-out execution ledger observed %d held-out cells, want %d", heldOutCellsSeen, lock.TestCells)
	}
	value := HeldOutExecutionLedger{
		SchemaVersion: HeldOutExecutionLedgerSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PartitionDigest: lock.Digest, CampaignDigest: campaign.Digest, AdmissionPlanDigest: admission.Digest,
		ExecutionBatchBindingDigest: execution.Digest, PermitDigest: permit.Digest, ReservationDigest: reservation.Digest,
		ArmReportDigest: armReport.Digest, AnalysisReportDigest: analysis.Digest,
		AnalysisCompletionStatus: "incomplete_due_to_pre_execution_exclusions", ExecutionPolicy: heldOutExecutionLedgerPolicy,
		TestCells: lock.TestCells, StructurallySupportedCells: admission.StructurallySupportedCells,
		StructuralUnsupportedCells: lock.UnsupportedTestCells, ExecutionEligibleCells: admission.ExecutionEligibleCells,
		ExecutedCells: admission.ExecutionEligibleCells, PreExecutionIneligibleCells: admission.PreExecutionIneligibleCells,
		ProviderCalls: providerCalls, Status: heldOutExecutionLedgerStatus, ExecutedEvidenceUnits: admission.ExecutionEligibleCells,
		LiveResponseEvidenceObserved: providerCalls > 0,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim: heldOutExecutionLedgerSupportedClaim, UnsupportedClaims: slices.Clone(heldOutExecutionLedgerUnsupportedClaims),
		},
	}
	for _, campaignArm := range campaign.Arms {
		arm := arms[campaignArm.ArmID]
		sort.Strings(armCellIDs[arm.ArmID])
		arm.EligibleCellSetDigest, err = digestDocument(armCellIDs[arm.ArmID])
		if err != nil {
			return HeldOutExecutionLedger{}, err
		}
		value.Arms = append(value.Arms, *arm)
	}
	sort.Slice(value.Arms, func(left, right int) bool { return value.Arms[left].ArmID < value.Arms[right].ArmID })
	return value, nil
}

func validateHeldOutExecutionLedgerArms(value HeldOutExecutionLedger) error {
	if len(value.Arms) != len(canonicalArmDefinitions()) {
		return errors.New("stress held-out execution ledger must account for every canonical arm")
	}
	previous := ""
	eligible, executed, ineligible, unsupported, providerCalls := 0, 0, 0, 0, 0
	for _, arm := range value.Arms {
		if arm.ArmID <= previous || !validDigest(arm.EligibleCellSetDigest) || arm.EligibleCells <= 0 ||
			arm.ExecutedCells != arm.EligibleCells || arm.PreExecutionIneligibleCells < 0 || arm.StructuralUnsupportedCells < 0 ||
			arm.ProviderCalls < 0 {
			return errors.New("stress held-out execution arm ledger identity or counts are invalid")
		}
		definition, exists := canonicalArmByID(arm.ArmID)
		if !exists {
			return errors.New("stress held-out execution ledger contains an unknown arm")
		}
		executionClass, err := heldOutExecutionClass(definition)
		if err != nil || executionClass != arm.ExecutionClass {
			return errors.New("stress held-out execution arm ledger class is invalid")
		}
		switch arm.EvidenceAuthority {
		case HeldOutEvidenceLiveProviderReservation:
			if executionClass != HeldOutExecutionLiveProvider || !validDigest(arm.PermitDigest) || !validDigest(arm.ReservationDigest) ||
				!validDigest(arm.LiveBatchEvidenceDigest) || !validDigest(arm.LiveReplayVerificationDigest) || !validDigest(arm.ReplayEvidenceSetDigest) ||
				arm.ReplaySourceArmID != "" || arm.ReplaySourceCellSetDigest != "" || arm.ProviderCalls <= 0 {
				return errors.New("stress held-out live arm lacks reservation-bound provider evidence")
			}
		case HeldOutEvidenceSealedProviderReplay:
			if executionClass != HeldOutExecutionSealedProviderReplay || arm.PermitDigest != "" || arm.ReservationDigest != "" ||
				arm.LiveBatchEvidenceDigest != "" || arm.LiveReplayVerificationDigest != "" || arm.ReplayEvidenceSetDigest != "" ||
				arm.ReplaySourceArmID == "" || !validDigest(arm.ReplaySourceCellSetDigest) || arm.ProviderCalls != 0 {
				return errors.New("stress held-out replay arm lacks its exact sealed source")
			}
		case HeldOutEvidenceDeterministicLocal:
			if executionClass != HeldOutExecutionDeterministicLocal || arm.PermitDigest != "" || arm.ReservationDigest != "" ||
				arm.LiveBatchEvidenceDigest != "" || arm.LiveReplayVerificationDigest != "" || arm.ReplayEvidenceSetDigest != "" ||
				arm.ReplaySourceArmID != "" || arm.ReplaySourceCellSetDigest != "" || arm.ProviderCalls != 0 {
				return errors.New("stress held-out local arm carries external execution authority")
			}
		default:
			return errors.New("stress held-out execution arm evidence authority is invalid")
		}
		previous = arm.ArmID
		eligible += arm.EligibleCells
		executed += arm.ExecutedCells
		ineligible += arm.PreExecutionIneligibleCells
		unsupported += arm.StructuralUnsupportedCells
		providerCalls += arm.ProviderCalls
	}
	if eligible != value.ExecutionEligibleCells || executed != value.ExecutedCells || ineligible != value.PreExecutionIneligibleCells ||
		unsupported != value.StructuralUnsupportedCells || providerCalls != value.ProviderCalls {
		return errors.New("stress held-out execution arm ledgers do not reproduce aggregate denominators")
	}
	return nil
}

func heldOutExecutionAuthorityForArm(arm HeldOutCampaignArm) (HeldOutExecutionEvidenceAuthority, error) {
	switch arm.ExecutionClass {
	case HeldOutExecutionLiveProvider:
		return HeldOutEvidenceLiveProviderReservation, nil
	case HeldOutExecutionSealedProviderReplay:
		return HeldOutEvidenceSealedProviderReplay, nil
	case HeldOutExecutionDeterministicLocal:
		return HeldOutEvidenceDeterministicLocal, nil
	default:
		return "", fmt.Errorf("stress held-out execution arm %q has an unsupported execution class", arm.ArmID)
	}
}

func hasString(values map[string]struct{}, value string) bool {
	_, exists := values[value]
	return exists
}

func heldOutExecutionLedgerDigest(value HeldOutExecutionLedger) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutExecutionLedgerSupportedClaim = "every admission-eligible held-out cell has one arm-appropriate execution evidence record while every ineligible and structural-unsupported cell remains in the denominator"

var heldOutExecutionLedgerUnsupportedClaims = []string{
	"packet-level proof that network transport occurred",
	"population generalization",
	"provider or verifier superiority without the bound analysis",
	"exactly-once provider execution",
	"run-seal completion",
}
