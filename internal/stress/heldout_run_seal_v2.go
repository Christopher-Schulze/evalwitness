package stress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const (
	HeldOutRunSealV2SchemaVersion = "evalwitness.stress-held-out-run-seal.v2"
	heldOutRunSealV2Status        = "admission_filtered_execution_complete_and_sealed"
	heldOutRunSealV2Policy        = "seal_once_after_admission_filtered_execution_and_denominator_closure"
	heldOutRunSealV2AnalysisState = "incomplete_due_to_pre_execution_exclusions"
	heldOutRunSealV2MaxBytes      = 256 << 10
)

type HeldOutRunSealV2 struct {
	SchemaVersion                   string                               `json:"schema_version"`
	CanonicalPolicy                 string                               `json:"canonical_policy"`
	SealAuthority                   HeldOutExecutionReservationAuthority `json:"seal_authority"`
	SealKey                         string                               `json:"seal_key"`
	PartitionDigest                 string                               `json:"partition_digest"`
	CampaignDigest                  string                               `json:"campaign_digest"`
	AdmissionPlanDigest             string                               `json:"admission_plan_digest"`
	ExecutionBatchBindingDigest     string                               `json:"execution_batch_binding_digest"`
	PermitDigest                    string                               `json:"permit_digest"`
	ReservationDigest               string                               `json:"reservation_digest"`
	ExecutionLedgerDigest           string                               `json:"execution_ledger_digest"`
	RegistryDigest                  string                               `json:"registry_digest"`
	ReleaseDigest                   string                               `json:"release_digest"`
	SourceCatalogVersion            string                               `json:"source_catalog_version"`
	NextCatalogVersion              string                               `json:"next_catalog_version"`
	ArmPlanDigest                   string                               `json:"arm_plan_digest"`
	AnalysisDesignDigest            string                               `json:"analysis_design_digest"`
	ArmReportDigest                 string                               `json:"arm_report_digest"`
	AnalysisReportDigest            string                               `json:"analysis_report_digest"`
	LiveBatchEvidenceSetDigest      string                               `json:"live_batch_evidence_set_digest"`
	LiveReplayVerificationSetDigest string                               `json:"live_replay_verification_set_digest"`
	ReplayCaptureSourceSetDigest    string                               `json:"replay_capture_source_set_digest"`
	ReplayCaptureSources            int                                  `json:"replay_capture_sources"`
	RunPolicy                       string                               `json:"run_policy"`
	RunCount                        int                                  `json:"run_count"`
	Reopened                        bool                                 `json:"reopened"`
	Completed                       bool                                 `json:"completed"`
	TestCells                       int                                  `json:"test_cells"`
	StructurallySupportedCells      int                                  `json:"structurally_supported_cells"`
	StructuralUnsupportedCells      int                                  `json:"structural_unsupported_cells"`
	ExecutionEligibleCells          int                                  `json:"execution_eligible_cells"`
	ExecutedCells                   int                                  `json:"executed_cells"`
	PreExecutionIneligibleCells     int                                  `json:"pre_execution_ineligible_cells"`
	ProviderCalls                   int                                  `json:"provider_calls"`
	ExecutedEvidenceUnits           int                                  `json:"executed_evidence_units"`
	LiveResponseEvidenceObserved    bool                                 `json:"live_response_evidence_observed"`
	ViolatedExecutedCells           int                                  `json:"violated_executed_cells"`
	WitnessesRequired               int                                  `json:"witnesses_required"`
	WitnessesBound                  int                                  `json:"witnesses_bound"`
	WitnessesMissing                int                                  `json:"witnesses_missing"`
	AnalysisCompletionStatus        string                               `json:"analysis_completion_status"`
	ConfirmatoryInferenceComplete   bool                                 `json:"confirmatory_inference_complete"`
	PopulationGeneralization        bool                                 `json:"population_generalization"`
	Status                          string                               `json:"status"`
	ClaimBoundary                   HeldOutCampaignClaimBoundary         `json:"claim_boundary"`
	Digest                          string                               `json:"digest"`
}

type HeldOutRunSealV2Parents struct {
	Lock                    HeldOutPartitionLock
	Campaign                HeldOutCampaignPlan
	Admission               HeldOutAdmissionPlan
	Execution               HeldOutExecutionBatchBinding
	Permit                  HeldOutExecutionPermit
	Reservation             HeldOutExecutionReservation
	LiveEvidence            []HeldOutLiveBatchEvidence
	LiveReplayVerifications []HeldOutLiveReplayVerification
	ExecutionLedger         HeldOutExecutionLedger
	Design                  StressAnalysisDesign
	Plan                    ArmComparisonPlan
	ArmReport               ArmComparisonReport
	Analysis                StressAnalysisReport
	Registry                RelationRegistry
	Replayed                []ReplayedRelationCaseV3
	ReplayEvidence          []ArmReplayEvidence
	ZeroCostEvidence        []ZeroCostExecution
	ProtocolProof           *ProtocolAdapterProof
	Counterexamples         []Counterexample
}

type HeldOutRunSealStore struct {
	root      *safety.CacheRoot
	authority HeldOutExecutionReservationAuthority
}

func NewHeldOutRunSealStore(root *safety.CacheRoot) (*HeldOutRunSealStore, error) {
	authority, err := NewHeldOutExecutionReservationAuthority(root)
	if err != nil {
		return nil, err
	}
	return &HeldOutRunSealStore{root: root, authority: authority}, nil
}

func (store *HeldOutRunSealStore) Seal(parents HeldOutRunSealV2Parents) (HeldOutRunSealV2, error) {
	if store == nil || store.root == nil {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal requires an initialized owner-only store")
	}
	if parents.Permit.ReservationAuthority != store.authority || parents.Reservation.Authority != store.authority {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal store differs from the permit-bound reservation authority")
	}
	if err := parents.Validate(); err != nil {
		return HeldOutRunSealV2{}, err
	}
	value, err := buildHeldOutRunSealV2(store.authority, parents)
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	value.Digest, err = heldOutRunSealV2Digest(value)
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	if err := value.Validate(); err != nil {
		return HeldOutRunSealV2{}, err
	}
	raw, err := EncodeIndented(value)
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	if err := store.root.PublishSensitiveExclusive(filepath.FromSlash(value.SealKey), raw); err != nil {
		if existing, loadErr := store.Load(parents.Lock); loadErr == nil {
			return HeldOutRunSealV2{}, fmt.Errorf("stress held-out partition was already sealed with %s: %w", existing.Digest, err)
		}
		return HeldOutRunSealV2{}, fmt.Errorf("publish stress held-out run seal atomically: %w", err)
	}
	return value, nil
}

func (store *HeldOutRunSealStore) Load(lock HeldOutPartitionLock) (HeldOutRunSealV2, error) {
	if store == nil || store.root == nil {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal requires an initialized owner-only store")
	}
	if err := lock.Validate(); err != nil {
		return HeldOutRunSealV2{}, err
	}
	raw, err := store.root.ReadSensitive(filepath.FromSlash(heldOutRunSealV2Key(lock.Digest)), heldOutRunSealV2MaxBytes)
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	value, err := DecodeHeldOutRunSealV2(bytes.NewReader(raw))
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	if value.PartitionDigest != lock.Digest || value.RegistryDigest != lock.RegistryDigest || value.SealAuthority != store.authority {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal differs from its partition or owner-only authority")
	}
	return value, nil
}

func (parents HeldOutRunSealV2Parents) Validate() error {
	if err := parents.Lock.ValidateAgainst(parents.Design, parents.Plan, parents.Registry, parents.Replayed); err != nil {
		return err
	}
	if err := parents.Campaign.ValidateAgainst(parents.Lock, parents.Design, parents.Plan, parents.Registry, parents.Replayed); err != nil {
		return err
	}
	if err := parents.Admission.Validate(); err != nil {
		return err
	}
	if err := parents.Execution.Validate(); err != nil {
		return err
	}
	if err := parents.Permit.Validate(); err != nil {
		return err
	}
	reservedAt, err := parseHeldOutExecutionPermitTime(parents.Reservation.ReservedAt)
	if err != nil {
		return err
	}
	if err := parents.Reservation.ValidateAgainst(parents.Permit, reservedAt); err != nil {
		return errors.New("stress held-out run seal lacks its exact valid permit reservation")
	}
	if err := parents.ArmReport.ValidateAgainst(
		parents.Plan, parents.Registry, parents.Replayed, parents.ReplayEvidence, parents.ZeroCostEvidence, parents.ProtocolProof,
	); err != nil {
		return err
	}
	if err := parents.Analysis.ValidateAgainst(
		parents.Design, parents.Plan, parents.ArmReport, parents.Registry, parents.Replayed,
		parents.ReplayEvidence, parents.ZeroCostEvidence, parents.ProtocolProof, parents.Counterexamples,
	); err != nil {
		return err
	}
	liveByArm := make(map[string]HeldOutLiveBatchEvidence, len(parents.LiveEvidence))
	for _, evidence := range parents.LiveEvidence {
		if _, duplicate := liveByArm[evidence.ArmID]; duplicate {
			return fmt.Errorf("stress held-out run seal repeats live evidence for arm %q", evidence.ArmID)
		}
		liveByArm[evidence.ArmID] = evidence
	}
	for _, verification := range parents.LiveReplayVerifications {
		live, exists := liveByArm[verification.ArmID]
		if !exists {
			return fmt.Errorf("stress held-out run seal replay verification for arm %q lacks live evidence", verification.ArmID)
		}
		if err := verification.ValidateAgainst(
			parents.Plan, parents.Registry, parents.Replayed, parents.Admission, live, parents.ReplayEvidence,
		); err != nil {
			return err
		}
	}
	if err := parents.ExecutionLedger.ValidateAgainst(
		parents.Lock, parents.Campaign, parents.Admission, parents.Execution, parents.Permit, parents.Reservation,
		parents.LiveEvidence, parents.LiveReplayVerifications, parents.ArmReport, parents.Analysis,
	); err != nil {
		return err
	}
	if parents.Admission.CampaignDigest != parents.Campaign.Digest || parents.Admission.PartitionDigest != parents.Lock.Digest ||
		parents.Admission.AnalysisDesignDigest != parents.Design.Digest || parents.Admission.ArmPlanDigest != parents.Plan.Digest ||
		parents.Admission.RegistryDigest != parents.Registry.Digest || parents.Execution.CampaignDigest != parents.Campaign.Digest ||
		parents.Execution.AdmissionPlanDigest != parents.Admission.Digest || parents.Execution.PartitionDigest != parents.Lock.Digest ||
		parents.Permit.ExecutionBatchBindingDigest != parents.Execution.Digest || parents.ExecutionLedger.ExecutionBatchBindingDigest != parents.Execution.Digest {
		return errors.New("stress held-out run seal parents do not share one frozen partition, admission, execution, and analysis lineage")
	}
	return nil
}

func (value HeldOutRunSealV2) Validate() error {
	digests := []string{
		value.PartitionDigest, value.CampaignDigest, value.AdmissionPlanDigest, value.ExecutionBatchBindingDigest,
		value.PermitDigest, value.ReservationDigest, value.ExecutionLedgerDigest, value.RegistryDigest, value.ReleaseDigest,
		value.ArmPlanDigest, value.AnalysisDesignDigest, value.ArmReportDigest, value.AnalysisReportDigest,
		value.LiveBatchEvidenceSetDigest, value.LiveReplayVerificationSetDigest, value.ReplayCaptureSourceSetDigest,
	}
	if value.SchemaVersion != HeldOutRunSealV2SchemaVersion || value.CanonicalPolicy != CanonicalPolicy {
		return errors.New("stress held-out run seal v2 identity is invalid")
	}
	for _, digest := range digests {
		if !validDigest(digest) {
			return errors.New("stress held-out run seal v2 lineage digest is invalid")
		}
	}
	if err := value.SealAuthority.Validate(); err != nil {
		return err
	}
	if value.SealKey != heldOutRunSealV2Key(value.PartitionDigest) || value.RunPolicy != heldOutRunSealV2Policy ||
		value.RunCount != 1 || value.Reopened || !value.Completed || value.Status != heldOutRunSealV2Status {
		return errors.New("stress held-out run seal v2 weakens atomic once-only completion")
	}
	if value.TestCells <= 0 || value.StructurallySupportedCells <= 0 || value.StructuralUnsupportedCells < 0 ||
		value.ExecutionEligibleCells <= 0 || value.ExecutedCells != value.ExecutionEligibleCells || value.PreExecutionIneligibleCells < 0 ||
		value.TestCells != value.StructurallySupportedCells+value.StructuralUnsupportedCells ||
		value.StructurallySupportedCells != value.ExecutionEligibleCells+value.PreExecutionIneligibleCells ||
		value.ExecutedEvidenceUnits != value.ExecutedCells || value.ProviderCalls <= 0 || !value.LiveResponseEvidenceObserved ||
		value.ReplayCaptureSources <= 0 {
		return errors.New("stress held-out run seal v2 denominator or execution accounting is invalid")
	}
	if value.ViolatedExecutedCells < 0 || value.ViolatedExecutedCells > value.ExecutedCells ||
		value.WitnessesRequired != value.ViolatedExecutedCells || value.WitnessesBound < 0 || value.WitnessesMissing < 0 ||
		value.WitnessesBound+value.WitnessesMissing != value.WitnessesRequired {
		return errors.New("stress held-out run seal v2 violation or witness accounting is invalid")
	}
	if value.AnalysisCompletionStatus != heldOutRunSealV2AnalysisState || value.ConfirmatoryInferenceComplete || value.PopulationGeneralization ||
		value.ClaimBoundary.SupportedClaim != heldOutRunSealV2SupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutRunSealV2UnsupportedClaims) {
		return errors.New("stress held-out run seal v2 promotes admission-filtered evidence beyond its analysis boundary")
	}
	nextVersion, err := nextCatalogVersion(value.SourceCatalogVersion)
	if err != nil || value.NextCatalogVersion != nextVersion {
		return errors.New("stress held-out run seal v2 does not preserve exact next-version routing")
	}
	expected, err := heldOutRunSealV2Digest(value)
	if err != nil || expected != value.Digest {
		return errors.New("stress held-out run seal v2 digest is invalid")
	}
	return nil
}

func (value HeldOutRunSealV2) ValidateAgainst(parents HeldOutRunSealV2Parents) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := parents.Validate(); err != nil {
		return err
	}
	want, err := buildHeldOutRunSealV2(value.SealAuthority, parents)
	if err != nil {
		return err
	}
	want.Digest, err = heldOutRunSealV2Digest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out run seal v2 differs from its exact partition, execution ledger, evidence, replay, analysis, or witness parents")
	}
	return nil
}

func RouteHeldOutDiscoveriesV2(
	seal HeldOutRunSealV2,
	parents HeldOutRunSealV2Parents,
	discoveries []RelationDiscovery,
) (NextVersionDiscoveryLedger, error) {
	if err := seal.ValidateAgainst(parents); err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	return buildNextVersionDiscoveryLedger(
		parents.Lock, seal.Digest, parents.Registry, parents.ArmReport, seal.NextCatalogVersion, discoveries,
	)
}

func (value NextVersionDiscoveryLedger) ValidateAgainstV2(
	seal HeldOutRunSealV2,
	parents HeldOutRunSealV2Parents,
) error {
	if err := seal.ValidateAgainst(parents); err != nil {
		return err
	}
	return value.validateAgainstBoundArtifacts(
		parents.Lock, seal.Digest, parents.Registry, parents.ArmReport, seal.NextCatalogVersion,
	)
}

func DecodeHeldOutRunSealV2(reader io.Reader) (HeldOutRunSealV2, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, heldOutRunSealV2MaxBytes+1))
	if err != nil {
		return HeldOutRunSealV2{}, fmt.Errorf("read stress held-out run seal v2: %w", err)
	}
	if len(raw) > heldOutRunSealV2MaxBytes {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal v2 exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutRunSealV2
	if err := decoder.Decode(&value); err != nil {
		return HeldOutRunSealV2{}, fmt.Errorf("decode stress held-out run seal v2: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal v2 has trailing JSON")
	}
	return value, value.Validate()
}

func buildHeldOutRunSealV2(
	authority HeldOutExecutionReservationAuthority,
	parents HeldOutRunSealV2Parents,
) (HeldOutRunSealV2, error) {
	if err := authority.Validate(); err != nil {
		return HeldOutRunSealV2{}, err
	}
	if parents.Permit.ReservationAuthority != authority || parents.Reservation.Authority != authority {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal v2 authority differs from its permit and reservation")
	}
	stats, err := admissionFilteredHeldOutSealStats(parents.Lock, parents.Admission, parents.ArmReport, parents.Analysis)
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	liveSetDigest, err := heldOutRunSealEvidenceSetDigest(parents.LiveEvidence, func(value HeldOutLiveBatchEvidence) string { return value.Digest })
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	replaySetDigest, err := heldOutRunSealEvidenceSetDigest(parents.LiveReplayVerifications, func(value HeldOutLiveReplayVerification) string { return value.Digest })
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	captureDigests := make([]string, len(parents.LiveReplayVerifications))
	for index, verification := range parents.LiveReplayVerifications {
		captureDigests[index], err = digestDocument(verification.ReplaySource)
		if err != nil {
			return HeldOutRunSealV2{}, err
		}
	}
	sort.Strings(captureDigests)
	captureDigests = slices.Compact(captureDigests)
	if len(captureDigests) == 0 {
		return HeldOutRunSealV2{}, errors.New("stress held-out run seal v2 requires at least one exact capture source")
	}
	captureSetDigest, err := digestDocument(captureDigests)
	if err != nil {
		return HeldOutRunSealV2{}, err
	}
	ledger := parents.ExecutionLedger
	value := HeldOutRunSealV2{
		SchemaVersion: HeldOutRunSealV2SchemaVersion, CanonicalPolicy: CanonicalPolicy,
		SealAuthority: authority, SealKey: heldOutRunSealV2Key(parents.Lock.Digest),
		PartitionDigest: parents.Lock.Digest, CampaignDigest: parents.Campaign.Digest,
		AdmissionPlanDigest: parents.Admission.Digest, ExecutionBatchBindingDigest: parents.Execution.Digest,
		PermitDigest: parents.Permit.Digest, ReservationDigest: parents.Reservation.Digest,
		ExecutionLedgerDigest: ledger.Digest, RegistryDigest: parents.Registry.Digest, ReleaseDigest: parents.Registry.ReleaseDigest,
		SourceCatalogVersion: parents.Lock.SourceCatalogVersion, NextCatalogVersion: parents.Lock.NextCatalogVersion,
		ArmPlanDigest: parents.Plan.Digest, AnalysisDesignDigest: parents.Design.Digest,
		ArmReportDigest: parents.ArmReport.Digest, AnalysisReportDigest: parents.Analysis.Digest,
		LiveBatchEvidenceSetDigest: liveSetDigest, LiveReplayVerificationSetDigest: replaySetDigest,
		ReplayCaptureSourceSetDigest: captureSetDigest, ReplayCaptureSources: len(captureDigests),
		RunPolicy: heldOutRunSealV2Policy, RunCount: 1, Completed: true,
		TestCells: ledger.TestCells, StructurallySupportedCells: ledger.StructurallySupportedCells,
		StructuralUnsupportedCells: ledger.StructuralUnsupportedCells, ExecutionEligibleCells: ledger.ExecutionEligibleCells,
		ExecutedCells: ledger.ExecutedCells, PreExecutionIneligibleCells: ledger.PreExecutionIneligibleCells,
		ProviderCalls: ledger.ProviderCalls, ExecutedEvidenceUnits: ledger.ExecutedEvidenceUnits,
		LiveResponseEvidenceObserved: ledger.LiveResponseEvidenceObserved,
		ViolatedExecutedCells:        stats.violated, WitnessesRequired: stats.violated,
		WitnessesBound: stats.witnessesBound, WitnessesMissing: stats.witnessesMissing,
		AnalysisCompletionStatus: ledger.AnalysisCompletionStatus,
		Status:                   heldOutRunSealV2Status,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim: heldOutRunSealV2SupportedClaim, UnsupportedClaims: slices.Clone(heldOutRunSealV2UnsupportedClaims),
		},
	}
	return value, nil
}

func admissionFilteredHeldOutSealStats(
	lock HeldOutPartitionLock,
	admission HeldOutAdmissionPlan,
	report ArmComparisonReport,
	analysis StressAnalysisReport,
) (heldOutStats, error) {
	if err := lock.Validate(); err != nil {
		return heldOutStats{}, err
	}
	if err := admission.Validate(); err != nil {
		return heldOutStats{}, err
	}
	if err := report.Validate(); err != nil {
		return heldOutStats{}, err
	}
	if err := validateStressAnalysisArtifact(analysis, lock.AnalysisDesignDigest, report.Digest); err != nil {
		return heldOutStats{}, err
	}
	testCells := stringSet(lock.TestCellIDs)
	eligible := stringSet(admission.ExecutionEligibleCellIDs)
	ineligible := stringSet(admission.PreExecutionIneligibleCellIDs)
	unsupported := stringSet(lock.UnsupportedTestCellIDs)
	violations := make(map[string]struct{})
	executed := 0
	seen := make(map[string]struct{}, len(testCells))
	for _, cell := range report.Cells {
		if _, isTest := testCells[cell.CellID]; !isTest {
			continue
		}
		if _, duplicate := seen[cell.CellID]; duplicate {
			return heldOutStats{}, fmt.Errorf("stress held-out run seal v2 repeats test cell %q", cell.CellID)
		}
		seen[cell.CellID] = struct{}{}
		switch {
		case hasString(eligible, cell.CellID):
			if cell.Status != ArmCellExecuted || cell.Support != ArmSupported {
				return heldOutStats{}, fmt.Errorf("stress held-out run seal v2 eligible cell %q was not executed", cell.CellID)
			}
			executed++
			if cell.Outcome == OutcomeViolated {
				violations[cell.CellID] = struct{}{}
			}
		case hasString(ineligible, cell.CellID):
			if cell.Status != ArmCellNotRun || cell.Support != ArmSupported {
				return heldOutStats{}, fmt.Errorf("stress held-out run seal v2 ineligible cell %q contains execution evidence", cell.CellID)
			}
		case hasString(unsupported, cell.CellID):
			if cell.Status != ArmCellUnsupported || cell.Support != ArmUnsupported {
				return heldOutStats{}, fmt.Errorf("stress held-out run seal v2 structural cell %q changed support status", cell.CellID)
			}
		default:
			return heldOutStats{}, fmt.Errorf("stress held-out run seal v2 cell %q is outside the denominator partition", cell.CellID)
		}
	}
	if len(seen) != lock.TestCells || executed != admission.ExecutionEligibleCells {
		return heldOutStats{}, errors.New("stress held-out run seal v2 does not close the complete frozen test denominator")
	}
	testSupported, testNotRun := 0, 0
	for _, summary := range analysis.Summaries {
		if summary.Split != AnalysisTest {
			continue
		}
		testSupported += summary.SupportedCells
		testNotRun += summary.NotRunCells
		if summary.SupportedCells != summary.CompletedCells+summary.NotRunCells ||
			summary.PlannedCells != summary.SupportedCells+summary.StructuralUnsupportedCells {
			return heldOutStats{}, fmt.Errorf("stress held-out run seal v2 analysis summary %q loses denominator cells", summary.SummaryID)
		}
		if summary.NotRunCells > 0 && summary.Status != AnalysisIncomplete && summary.Status != AnalysisNotRun {
			return heldOutStats{}, fmt.Errorf("stress held-out run seal v2 analysis summary %q promotes incomplete evidence", summary.SummaryID)
		}
	}
	if testSupported != lock.SupportedTestCells || testNotRun != admission.PreExecutionIneligibleCells {
		return heldOutStats{}, errors.New("stress held-out run seal v2 analysis differs from the admission-filtered supported denominator")
	}
	witnesses := make(map[string]MinimalWitnessBinding, len(analysis.MinimalWitnesses))
	for _, witness := range analysis.MinimalWitnesses {
		if _, isTest := testCells[witness.CellID]; isTest {
			witnesses[witness.CellID] = witness
		}
	}
	stats := heldOutStats{executed: executed, violated: len(violations)}
	for cellID := range violations {
		witness, exists := witnesses[cellID]
		if exists && (witness.Status == WitnessBoundPrivate || witness.Status == WitnessBoundPublic) {
			stats.witnessesBound++
		} else {
			stats.witnessesMissing++
		}
	}
	if len(witnesses) != len(violations) {
		return heldOutStats{}, errors.New("stress held-out run seal v2 witness ledger differs from the executed violation set")
	}
	return stats, nil
}

func heldOutRunSealEvidenceSetDigest[T any](values []T, digest func(T) string) (string, error) {
	digests := make([]string, len(values))
	for index, value := range values {
		digests[index] = digest(value)
		if !validDigest(digests[index]) {
			return "", errors.New("stress held-out run seal v2 evidence set contains an invalid digest")
		}
	}
	sort.Strings(digests)
	if len(digests) == 0 || len(slices.Compact(slices.Clone(digests))) != len(digests) {
		return "", errors.New("stress held-out run seal v2 evidence set is empty or duplicated")
	}
	return digestDocument(digests)
}

func heldOutRunSealV2Key(partitionDigest string) string {
	return "held-out-run-seals/" + partitionDigest + ".json"
}

func heldOutRunSealV2Digest(value HeldOutRunSealV2) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutRunSealV2SupportedClaim = "one authority-bound run seal closes the frozen held-out denominator across structural unsupported, pre-execution ineligible, and admission-eligible executed cells and binds the exact execution ledger"

var heldOutRunSealV2UnsupportedClaims = []string{
	"complete confirmatory inference across every structurally supported held-out cell",
	"execution evidence for pre-execution-ineligible cells",
	"global exactly-once execution or rollback protection outside the bound owner-only authority",
	"independent capture-byte validation without retained and re-inspected capture parents",
	"packet-level proof that network transport occurred",
	"population generalization or provider and verifier superiority",
}
