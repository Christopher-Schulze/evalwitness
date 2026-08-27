package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

const (
	HeldOutPartitionLockSchemaVersion = "evalwitness.stress-held-out-partition-lock.v1"
	HeldOutRunSealSchemaVersion       = "evalwitness.stress-held-out-run-seal.v1"
	NextVersionDiscoverySchemaVersion = "evalwitness.stress-next-version-discovery-ledger.v1"
	heldOutRunPolicy                  = "execute_locked_test_partition_once_without_reopen"
	heldOutDiscoveryPolicy            = "route_test_failure_discoveries_to_exact_next_catalog_without_retrofit"
	heldOutInferencePolicy            = "test_only_confirmatory_inference_with_complete_supported_cell_accounting"
)

type HeldOutPartitionLock struct {
	SchemaVersion          string   `json:"schema_version"`
	CanonicalPolicy        string   `json:"canonical_policy"`
	RegistryDigest         string   `json:"registry_digest"`
	SourceCatalogVersion   string   `json:"source_catalog_version"`
	NextCatalogVersion     string   `json:"next_catalog_version"`
	ReleaseDigest          string   `json:"release_digest"`
	ArmPlanDigest          string   `json:"arm_plan_digest"`
	AnalysisDesignDigest   string   `json:"analysis_design_digest"`
	DataRole               string   `json:"data_role"`
	RunPolicy              string   `json:"run_policy"`
	DiscoveryPolicy        string   `json:"discovery_policy"`
	InferencePolicy        string   `json:"inference_policy"`
	TestCaseIDs            []string `json:"test_case_ids"`
	TestCellIDs            []string `json:"test_cell_ids"`
	SupportedTestCellIDs   []string `json:"supported_test_cell_ids"`
	UnsupportedTestCellIDs []string `json:"unsupported_test_cell_ids"`
	TestCases              int      `json:"test_cases"`
	TestCells              int      `json:"test_cells"`
	SupportedTestCells     int      `json:"supported_test_cells"`
	UnsupportedTestCells   int      `json:"unsupported_test_cells"`
	Digest                 string   `json:"digest"`
}

type HeldOutRunSeal struct {
	SchemaVersion        string `json:"schema_version"`
	CanonicalPolicy      string `json:"canonical_policy"`
	PartitionDigest      string `json:"partition_digest"`
	RegistryDigest       string `json:"registry_digest"`
	SourceCatalogVersion string `json:"source_catalog_version"`
	NextCatalogVersion   string `json:"next_catalog_version"`
	ArmPlanDigest        string `json:"arm_plan_digest"`
	AnalysisDesignDigest string `json:"analysis_design_digest"`
	ArmReportDigest      string `json:"arm_report_digest"`
	AnalysisReportDigest string `json:"analysis_report_digest"`
	RunPolicy            string `json:"run_policy"`
	RunCount             int    `json:"run_count"`
	Reopened             bool   `json:"reopened"`
	Completed            bool   `json:"completed"`
	TestCells            int    `json:"test_cells"`
	SupportedTestCells   int    `json:"supported_test_cells"`
	ExecutedTestCells    int    `json:"executed_test_cells"`
	UnsupportedTestCells int    `json:"unsupported_test_cells"`
	ViolatedTestCells    int    `json:"violated_test_cells"`
	WitnessesBound       int    `json:"witnesses_bound"`
	WitnessesMissing     int    `json:"witnesses_missing"`
	Digest               string `json:"digest"`
}

type RelationDiscovery struct {
	SourceCellID            string   `json:"source_cell_id"`
	DiscoveryEvidenceDigest string   `json:"discovery_evidence_digest"`
	Candidate               Relation `json:"candidate_relation"`
}

type RoutedRelationDiscovery struct {
	DiscoveryID             string   `json:"discovery_id"`
	SourceCellID            string   `json:"source_cell_id"`
	SourceResultDigest      string   `json:"source_result_digest"`
	SourceRelationID        string   `json:"source_relation_id"`
	SourceRelationDigest    string   `json:"source_relation_digest"`
	DiscoveryEvidenceDigest string   `json:"discovery_evidence_digest"`
	TargetCatalogVersion    string   `json:"target_catalog_version"`
	Candidate               Relation `json:"candidate_relation"`
}

type NextVersionDiscoveryLedger struct {
	SchemaVersion            string                    `json:"schema_version"`
	CanonicalPolicy          string                    `json:"canonical_policy"`
	HeldOutRunSealDigest     string                    `json:"held_out_run_seal_digest"`
	RegistryDigest           string                    `json:"registry_digest"`
	ArmReportDigest          string                    `json:"arm_report_digest"`
	SourceCatalogVersion     string                    `json:"source_catalog_version"`
	TargetCatalogVersion     string                    `json:"target_catalog_version"`
	DiscoveryPolicy          string                    `json:"discovery_policy"`
	CurrentCatalogFrozen     bool                      `json:"current_catalog_frozen"`
	TestPartitionRetrofitted bool                      `json:"test_partition_retrofitted"`
	Discoveries              []RoutedRelationDiscovery `json:"discoveries"`
	DiscoveryCount           int                       `json:"discovery_count"`
	Digest                   string                    `json:"digest"`
}

func BuildHeldOutPartitionLock(
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
) (HeldOutPartitionLock, error) {
	if err := design.ValidateAgainst(plan, registry, replayed); err != nil {
		return HeldOutPartitionLock{}, err
	}
	nextVersion, err := nextCatalogVersion(registry.CatalogVersion)
	if err != nil {
		return HeldOutPartitionLock{}, err
	}
	testCases := make(map[string]struct{})
	for _, item := range replayed {
		if item.Split == study.RoleTest {
			testCases[item.CaseID] = struct{}{}
		}
	}
	value := HeldOutPartitionLock{
		SchemaVersion: HeldOutPartitionLockSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistryDigest: registry.Digest, SourceCatalogVersion: registry.CatalogVersion, NextCatalogVersion: nextVersion,
		ReleaseDigest: registry.ReleaseDigest, ArmPlanDigest: plan.Digest, AnalysisDesignDigest: design.Digest,
		DataRole: string(study.RoleTest), RunPolicy: heldOutRunPolicy, DiscoveryPolicy: heldOutDiscoveryPolicy,
		InferencePolicy: heldOutInferencePolicy,
	}
	for caseID := range testCases {
		value.TestCaseIDs = append(value.TestCaseIDs, caseID)
	}
	for _, cell := range plan.Cells {
		if _, exists := testCases[cell.CaseID]; !exists {
			continue
		}
		value.TestCellIDs = append(value.TestCellIDs, cell.CellID)
		if cell.Support == ArmSupported {
			value.SupportedTestCellIDs = append(value.SupportedTestCellIDs, cell.CellID)
		} else {
			value.UnsupportedTestCellIDs = append(value.UnsupportedTestCellIDs, cell.CellID)
		}
	}
	sort.Strings(value.TestCaseIDs)
	sort.Strings(value.TestCellIDs)
	sort.Strings(value.SupportedTestCellIDs)
	sort.Strings(value.UnsupportedTestCellIDs)
	value.TestCases = len(value.TestCaseIDs)
	value.TestCells = len(value.TestCellIDs)
	value.SupportedTestCells = len(value.SupportedTestCellIDs)
	value.UnsupportedTestCells = len(value.UnsupportedTestCellIDs)
	value.Digest, err = heldOutPartitionLockDigest(value)
	if err != nil {
		return HeldOutPartitionLock{}, err
	}
	if err := value.Validate(); err != nil {
		return HeldOutPartitionLock{}, err
	}
	return value, nil
}

func (value HeldOutPartitionLock) Validate() error {
	if value.SchemaVersion != HeldOutPartitionLockSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.RegistryDigest) || !validDigest(value.ReleaseDigest) || !validDigest(value.ArmPlanDigest) ||
		!validDigest(value.AnalysisDesignDigest) || value.DataRole != string(study.RoleTest) ||
		value.RunPolicy != heldOutRunPolicy || value.DiscoveryPolicy != heldOutDiscoveryPolicy || value.InferencePolicy != heldOutInferencePolicy {
		return errors.New("stress held-out partition lock identity or policy is invalid")
	}
	nextVersion, err := nextCatalogVersion(value.SourceCatalogVersion)
	if err != nil || value.NextCatalogVersion != nextVersion {
		return errors.New("stress held-out partition lock does not route to the exact next catalog version")
	}
	if err := validateCanonicalIDs("test case", value.TestCaseIDs, true); err != nil {
		return err
	}
	if err := validateCanonicalIDs("test cell", value.TestCellIDs, true); err != nil {
		return err
	}
	if err := validateCanonicalIDs("supported test cell", value.SupportedTestCellIDs, true); err != nil {
		return err
	}
	if err := validateCanonicalIDs("unsupported test cell", value.UnsupportedTestCellIDs, true); err != nil {
		return err
	}
	if !partitionCellSetsMatch(value.TestCellIDs, value.SupportedTestCellIDs, value.UnsupportedTestCellIDs) ||
		value.TestCases != len(value.TestCaseIDs) || value.TestCells != len(value.TestCellIDs) ||
		value.SupportedTestCells != len(value.SupportedTestCellIDs) || value.UnsupportedTestCells != len(value.UnsupportedTestCellIDs) ||
		value.TestCells != value.SupportedTestCells+value.UnsupportedTestCells {
		return errors.New("stress held-out partition lock counts or support partition are invalid")
	}
	expected, err := heldOutPartitionLockDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out partition lock digest is invalid")
	}
	return nil
}

func (value HeldOutPartitionLock) ValidateAgainst(
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	want, err := BuildHeldOutPartitionLock(design, plan, registry, replayed)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out partition lock differs from the frozen test split, registry, arm plan, or analysis design")
	}
	return nil
}

func SealHeldOutRun(
	existing []HeldOutRunSeal,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	armReport ArmComparisonReport,
	analysis StressAnalysisReport,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	replayEvidence []ArmReplayEvidence,
	zeroCostEvidence []ZeroCostExecution,
	protocolProof *ProtocolAdapterProof,
	counterexamples []Counterexample,
) (HeldOutRunSeal, error) {
	if len(existing) != 0 {
		return HeldOutRunSeal{}, &AdmissionError{State: InvalidLockedPartitionUsed, Reason: "held-out test partition already has a run seal"}
	}
	if err := lock.ValidateAgainst(design, plan, registry, replayed); err != nil {
		return HeldOutRunSeal{}, err
	}
	if err := armReport.ValidateAgainst(plan, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof); err != nil {
		return HeldOutRunSeal{}, err
	}
	if err := analysis.ValidateAgainst(design, plan, armReport, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof, counterexamples); err != nil {
		return HeldOutRunSeal{}, err
	}
	stats, err := heldOutCompletionStats(lock, armReport, analysis)
	if err != nil {
		return HeldOutRunSeal{}, err
	}
	value := HeldOutRunSeal{
		SchemaVersion: HeldOutRunSealSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PartitionDigest: lock.Digest, RegistryDigest: registry.Digest, SourceCatalogVersion: lock.SourceCatalogVersion,
		NextCatalogVersion: lock.NextCatalogVersion, ArmPlanDigest: plan.Digest, AnalysisDesignDigest: design.Digest,
		ArmReportDigest: armReport.Digest, AnalysisReportDigest: analysis.Digest, RunPolicy: heldOutRunPolicy,
		RunCount: 1, Reopened: false, Completed: true, TestCells: lock.TestCells,
		SupportedTestCells: lock.SupportedTestCells, ExecutedTestCells: stats.executed,
		UnsupportedTestCells: lock.UnsupportedTestCells, ViolatedTestCells: stats.violated,
		WitnessesBound: stats.witnessesBound, WitnessesMissing: stats.witnessesMissing,
	}
	value.Digest, err = heldOutRunSealDigest(value)
	if err != nil {
		return HeldOutRunSeal{}, err
	}
	if err := value.ValidateAgainst(lock, armReport, analysis); err != nil {
		return HeldOutRunSeal{}, err
	}
	return value, nil
}

func (value HeldOutRunSeal) Validate() error {
	if value.SchemaVersion != HeldOutRunSealSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.PartitionDigest) || !validDigest(value.RegistryDigest) || !validDigest(value.ArmPlanDigest) ||
		!validDigest(value.AnalysisDesignDigest) || !validDigest(value.ArmReportDigest) || !validDigest(value.AnalysisReportDigest) ||
		value.RunPolicy != heldOutRunPolicy || value.RunCount != 1 || value.Reopened || !value.Completed ||
		value.TestCells <= 0 || value.SupportedTestCells <= 0 || value.ExecutedTestCells != value.SupportedTestCells ||
		value.UnsupportedTestCells < 0 || value.TestCells != value.SupportedTestCells+value.UnsupportedTestCells ||
		value.ViolatedTestCells < 0 || value.ViolatedTestCells > value.ExecutedTestCells ||
		value.WitnessesBound < 0 || value.WitnessesMissing < 0 || value.WitnessesBound+value.WitnessesMissing != value.ViolatedTestCells {
		return errors.New("stress held-out run seal identity, once-only state, or complete test accounting is invalid")
	}
	nextVersion, err := nextCatalogVersion(value.SourceCatalogVersion)
	if err != nil || value.NextCatalogVersion != nextVersion {
		return errors.New("stress held-out run seal does not preserve exact next-version routing")
	}
	expected, err := heldOutRunSealDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out run seal digest is invalid")
	}
	return nil
}

func (value HeldOutRunSeal) ValidateAgainst(lock HeldOutPartitionLock, armReport ArmComparisonReport, analysis StressAnalysisReport) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := armReport.Validate(); err != nil {
		return err
	}
	if err := validateStressAnalysisArtifact(analysis, value.AnalysisDesignDigest, armReport.Digest); err != nil {
		return err
	}
	stats, err := heldOutCompletionStats(lock, armReport, analysis)
	if err != nil {
		return err
	}
	if value.PartitionDigest != lock.Digest || value.RegistryDigest != lock.RegistryDigest ||
		value.SourceCatalogVersion != lock.SourceCatalogVersion || value.NextCatalogVersion != lock.NextCatalogVersion ||
		value.ArmPlanDigest != lock.ArmPlanDigest || value.AnalysisDesignDigest != lock.AnalysisDesignDigest ||
		value.ArmReportDigest != armReport.Digest || value.AnalysisReportDigest != analysis.Digest ||
		value.TestCells != lock.TestCells || value.SupportedTestCells != lock.SupportedTestCells ||
		value.ExecutedTestCells != stats.executed || value.UnsupportedTestCells != lock.UnsupportedTestCells ||
		value.ViolatedTestCells != stats.violated || value.WitnessesBound != stats.witnessesBound ||
		value.WitnessesMissing != stats.witnessesMissing {
		return errors.New("stress held-out run seal differs from its partition, reports, or test-only accounting")
	}
	return nil
}

func RouteHeldOutDiscoveries(
	lock HeldOutPartitionLock,
	seal HeldOutRunSeal,
	registry RelationRegistry,
	armReport ArmComparisonReport,
	analysis StressAnalysisReport,
	discoveries []RelationDiscovery,
) (NextVersionDiscoveryLedger, error) {
	if err := lock.Validate(); err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	if err := seal.ValidateAgainst(lock, armReport, analysis); err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	if err := registry.Validate(); err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	if err := armReport.Validate(); err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	if seal.PartitionDigest != lock.Digest || seal.RegistryDigest != registry.Digest || seal.ArmReportDigest != armReport.Digest ||
		seal.SourceCatalogVersion != registry.CatalogVersion || seal.NextCatalogVersion != lock.NextCatalogVersion {
		return NextVersionDiscoveryLedger{}, errors.New("stress discovery routing artifacts do not share one held-out run and frozen catalog")
	}
	value, err := buildNextVersionDiscoveryLedger(lock, seal.Digest, registry, armReport, seal.NextCatalogVersion, discoveries)
	if err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	if err := value.ValidateAgainst(lock, seal, registry, armReport, analysis); err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	return value, nil
}

func buildNextVersionDiscoveryLedger(
	lock HeldOutPartitionLock,
	sealDigest string,
	registry RelationRegistry,
	armReport ArmComparisonReport,
	targetCatalogVersion string,
	discoveries []RelationDiscovery,
) (NextVersionDiscoveryLedger, error) {
	testCells := stringSet(lock.TestCellIDs)
	reportCells := make(map[string]ArmComparisonObservation, len(armReport.Cells))
	for _, cell := range armReport.Cells {
		reportCells[cell.CellID] = cell
	}
	currentIDs := make(map[string]struct{}, len(registry.Relations))
	currentDigests := make(map[string]struct{}, len(registry.Relations))
	for _, relation := range registry.Relations {
		currentIDs[relation.ID] = struct{}{}
		currentDigests[relation.Digest] = struct{}{}
	}
	value := NextVersionDiscoveryLedger{
		SchemaVersion: NextVersionDiscoverySchemaVersion, CanonicalPolicy: CanonicalPolicy,
		HeldOutRunSealDigest: sealDigest, RegistryDigest: registry.Digest, ArmReportDigest: armReport.Digest,
		SourceCatalogVersion: registry.CatalogVersion, TargetCatalogVersion: targetCatalogVersion,
		DiscoveryPolicy: heldOutDiscoveryPolicy, CurrentCatalogFrozen: true, TestPartitionRetrofitted: false,
	}
	seenCandidateIDs := make(map[string]struct{}, len(discoveries))
	seenCandidateDigests := make(map[string]struct{}, len(discoveries))
	for _, discovery := range discoveries {
		cell, exists := reportCells[discovery.SourceCellID]
		_, heldOut := testCells[discovery.SourceCellID]
		if !exists || !heldOut || cell.Status != ArmCellExecuted || cell.Outcome != OutcomeViolated {
			return NextVersionDiscoveryLedger{}, fmt.Errorf("stress relation discovery source cell %q is not one executed held-out violation", discovery.SourceCellID)
		}
		if !validDigest(discovery.DiscoveryEvidenceDigest) {
			return NextVersionDiscoveryLedger{}, errors.New("stress relation discovery evidence digest is invalid")
		}
		if err := discovery.Candidate.Validate(); err != nil {
			return NextVersionDiscoveryLedger{}, fmt.Errorf("validate next-version relation candidate %q: %w", discovery.Candidate.ID, err)
		}
		if _, exists := currentIDs[discovery.Candidate.ID]; exists {
			return NextVersionDiscoveryLedger{}, fmt.Errorf("stress relation discovery %q retrofits an ID in the frozen catalog", discovery.Candidate.ID)
		}
		if _, exists := currentDigests[discovery.Candidate.Digest]; exists {
			return NextVersionDiscoveryLedger{}, fmt.Errorf("stress relation discovery %q duplicates the frozen catalog", discovery.Candidate.ID)
		}
		if _, exists := seenCandidateIDs[discovery.Candidate.ID]; exists {
			return NextVersionDiscoveryLedger{}, fmt.Errorf("duplicate next-version relation candidate ID %q", discovery.Candidate.ID)
		}
		if _, exists := seenCandidateDigests[discovery.Candidate.Digest]; exists {
			return NextVersionDiscoveryLedger{}, fmt.Errorf("duplicate next-version relation candidate digest %q", discovery.Candidate.Digest)
		}
		seenCandidateIDs[discovery.Candidate.ID] = struct{}{}
		seenCandidateDigests[discovery.Candidate.Digest] = struct{}{}
		entry := RoutedRelationDiscovery{
			SourceCellID: discovery.SourceCellID, SourceResultDigest: cell.ResultDigest,
			SourceRelationID: cell.RelationID, SourceRelationDigest: cell.RelationDigest,
			DiscoveryEvidenceDigest: discovery.DiscoveryEvidenceDigest, TargetCatalogVersion: targetCatalogVersion,
			Candidate: discovery.Candidate,
		}
		discoveryID, err := relationDiscoveryID(entry)
		if err != nil {
			return NextVersionDiscoveryLedger{}, err
		}
		entry.DiscoveryID = discoveryID
		value.Discoveries = append(value.Discoveries, entry)
	}
	sort.Slice(value.Discoveries, func(left, right int) bool {
		return value.Discoveries[left].DiscoveryID < value.Discoveries[right].DiscoveryID
	})
	value.DiscoveryCount = len(value.Discoveries)
	var err error
	value.Digest, err = nextVersionDiscoveryLedgerDigest(value)
	if err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	if err := value.validateAgainstBoundArtifacts(lock, sealDigest, registry, armReport, targetCatalogVersion); err != nil {
		return NextVersionDiscoveryLedger{}, err
	}
	return value, nil
}

func (value NextVersionDiscoveryLedger) Validate() error {
	if value.SchemaVersion != NextVersionDiscoverySchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.HeldOutRunSealDigest) || !validDigest(value.RegistryDigest) || !validDigest(value.ArmReportDigest) ||
		value.DiscoveryPolicy != heldOutDiscoveryPolicy || !value.CurrentCatalogFrozen || value.TestPartitionRetrofitted ||
		value.DiscoveryCount != len(value.Discoveries) {
		return errors.New("stress next-version discovery ledger identity or frozen-catalog boundary is invalid")
	}
	nextVersion, err := nextCatalogVersion(value.SourceCatalogVersion)
	if err != nil || value.TargetCatalogVersion != nextVersion {
		return errors.New("stress next-version discovery ledger does not target the exact next catalog")
	}
	previousID := ""
	seenCandidateIDs := make(map[string]struct{}, len(value.Discoveries))
	seenCandidateDigests := make(map[string]struct{}, len(value.Discoveries))
	for _, discovery := range value.Discoveries {
		if !identifierPattern.MatchString(discovery.DiscoveryID) || discovery.DiscoveryID <= previousID ||
			!identifierPattern.MatchString(discovery.SourceCellID) || !validDigest(discovery.SourceResultDigest) ||
			!identifierPattern.MatchString(discovery.SourceRelationID) || !validDigest(discovery.SourceRelationDigest) ||
			!validDigest(discovery.DiscoveryEvidenceDigest) || discovery.TargetCatalogVersion != value.TargetCatalogVersion {
			return errors.New("stress next-version discovery entry identity, order, or source evidence is invalid")
		}
		if err := discovery.Candidate.Validate(); err != nil {
			return err
		}
		expectedID, err := relationDiscoveryID(discovery)
		if err != nil {
			return err
		}
		if discovery.DiscoveryID != expectedID {
			return errors.New("stress next-version discovery ID does not bind its source, evidence, target, and candidate")
		}
		if _, exists := seenCandidateIDs[discovery.Candidate.ID]; exists {
			return errors.New("stress next-version discovery ledger duplicates a candidate ID")
		}
		if _, exists := seenCandidateDigests[discovery.Candidate.Digest]; exists {
			return errors.New("stress next-version discovery ledger duplicates a candidate digest")
		}
		seenCandidateIDs[discovery.Candidate.ID] = struct{}{}
		seenCandidateDigests[discovery.Candidate.Digest] = struct{}{}
		previousID = discovery.DiscoveryID
	}
	expected, err := nextVersionDiscoveryLedgerDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress next-version discovery ledger digest is invalid")
	}
	return nil
}

func (value NextVersionDiscoveryLedger) ValidateAgainst(
	lock HeldOutPartitionLock,
	seal HeldOutRunSeal,
	registry RelationRegistry,
	armReport ArmComparisonReport,
	analysis StressAnalysisReport,
) error {
	if err := seal.ValidateAgainst(lock, armReport, analysis); err != nil {
		return err
	}
	return value.validateAgainstBoundArtifacts(lock, seal.Digest, registry, armReport, seal.NextCatalogVersion)
}

func (value NextVersionDiscoveryLedger) validateAgainstBoundArtifacts(
	lock HeldOutPartitionLock,
	sealDigest string,
	registry RelationRegistry,
	armReport ArmComparisonReport,
	targetCatalogVersion string,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := registry.Validate(); err != nil {
		return err
	}
	if err := armReport.Validate(); err != nil {
		return err
	}
	if value.HeldOutRunSealDigest != sealDigest || value.RegistryDigest != registry.Digest ||
		value.ArmReportDigest != armReport.Digest || value.SourceCatalogVersion != registry.CatalogVersion ||
		value.TargetCatalogVersion != targetCatalogVersion || targetCatalogVersion != lock.NextCatalogVersion {
		return errors.New("stress next-version discovery ledger differs from its held-out run, registry, arm report, or target catalog")
	}
	testCells := stringSet(lock.TestCellIDs)
	reportCells := make(map[string]ArmComparisonObservation, len(armReport.Cells))
	for _, cell := range armReport.Cells {
		reportCells[cell.CellID] = cell
	}
	currentIDs := make(map[string]struct{}, len(registry.Relations))
	currentDigests := make(map[string]struct{}, len(registry.Relations))
	for _, relation := range registry.Relations {
		currentIDs[relation.ID] = struct{}{}
		currentDigests[relation.Digest] = struct{}{}
	}
	for _, discovery := range value.Discoveries {
		cell, exists := reportCells[discovery.SourceCellID]
		_, heldOut := testCells[discovery.SourceCellID]
		if !exists || !heldOut || cell.Status != ArmCellExecuted || cell.Outcome != OutcomeViolated ||
			discovery.SourceResultDigest != cell.ResultDigest || discovery.SourceRelationID != cell.RelationID ||
			discovery.SourceRelationDigest != cell.RelationDigest {
			return fmt.Errorf("stress next-version discovery %q does not bind one exact held-out violation", discovery.DiscoveryID)
		}
		if _, exists := currentIDs[discovery.Candidate.ID]; exists {
			return fmt.Errorf("stress next-version discovery %q retrofits an ID in the frozen catalog", discovery.DiscoveryID)
		}
		if _, exists := currentDigests[discovery.Candidate.Digest]; exists {
			return fmt.Errorf("stress next-version discovery %q duplicates the frozen catalog", discovery.DiscoveryID)
		}
	}
	return nil
}

type heldOutStats struct {
	executed         int
	violated         int
	witnessesBound   int
	witnessesMissing int
}

func heldOutCompletionStats(lock HeldOutPartitionLock, report ArmComparisonReport, analysis StressAnalysisReport) (heldOutStats, error) {
	if err := lock.Validate(); err != nil {
		return heldOutStats{}, err
	}
	if err := report.Validate(); err != nil {
		return heldOutStats{}, err
	}
	if err := validateStressAnalysisArtifact(analysis, lock.AnalysisDesignDigest, report.Digest); err != nil {
		return heldOutStats{}, err
	}
	reportCells := make(map[string]ArmComparisonObservation, len(report.Cells))
	for _, cell := range report.Cells {
		reportCells[cell.CellID] = cell
	}
	stats := heldOutStats{}
	violations := make(map[string]struct{})
	for _, cellID := range lock.SupportedTestCellIDs {
		cell, exists := reportCells[cellID]
		if !exists || cell.Status != ArmCellExecuted || cell.Support != ArmSupported {
			return heldOutStats{}, fmt.Errorf("stress held-out supported cell %q was not executed exactly once", cellID)
		}
		stats.executed++
		if cell.Outcome == OutcomeViolated {
			stats.violated++
			violations[cellID] = struct{}{}
		}
	}
	for _, cellID := range lock.UnsupportedTestCellIDs {
		cell, exists := reportCells[cellID]
		if !exists || cell.Status != ArmCellUnsupported || cell.Support != ArmUnsupported {
			return heldOutStats{}, fmt.Errorf("stress held-out unsupported cell %q changed support status", cellID)
		}
	}
	supportedSummaryCells := 0
	for _, summary := range analysis.Summaries {
		if summary.Split != AnalysisTest {
			continue
		}
		supportedSummaryCells += summary.SupportedCells
		if summary.SupportedCells > 0 && (summary.NotRunCells != 0 || summary.Status != AnalysisAdjustedComplete) {
			return heldOutStats{}, fmt.Errorf("stress held-out analysis summary %q is incomplete", summary.SummaryID)
		}
	}
	if supportedSummaryCells != lock.SupportedTestCells {
		return heldOutStats{}, errors.New("stress held-out analysis does not account for every supported test cell")
	}
	witnesses := make(map[string]MinimalWitnessBinding, len(analysis.MinimalWitnesses))
	for _, witness := range analysis.MinimalWitnesses {
		witnesses[witness.CellID] = witness
	}
	for cellID := range violations {
		witness, exists := witnesses[cellID]
		if exists && (witness.Status == WitnessBoundPrivate || witness.Status == WitnessBoundPublic) {
			stats.witnessesBound++
		} else {
			stats.witnessesMissing++
		}
	}
	return stats, nil
}

func validateStressAnalysisArtifact(value StressAnalysisReport, designDigest, armReportDigest string) error {
	if value.SchemaVersion != StressAnalysisReportSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.DesignDigest != designDigest || value.ArmReportDigest != armReportDigest || value.GlobalScore || !validDigest(value.Digest) {
		return errors.New("stress analysis report identity or global-score boundary is invalid")
	}
	expected, err := stressAnalysisReportDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress analysis report digest is invalid")
	}
	return nil
}

func validateCanonicalIDs(name string, values []string, require bool) error {
	if require && len(values) == 0 {
		return fmt.Errorf("stress held-out %s identities are empty", name)
	}
	for index, value := range values {
		if !identifierPattern.MatchString(value) || index > 0 && values[index-1] >= value {
			return fmt.Errorf("stress held-out %s identities are invalid, duplicated, or unsorted", name)
		}
	}
	return nil
}

func partitionCellSetsMatch(all, supported, unsupported []string) bool {
	if len(all) != len(supported)+len(unsupported) {
		return false
	}
	combined := append(slices.Clone(supported), unsupported...)
	sort.Strings(combined)
	return slices.Equal(all, combined)
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func nextCatalogVersion(current string) (string, error) {
	separator := strings.LastIndex(current, ".v")
	if separator <= 0 || separator+2 >= len(current) {
		return "", errors.New("stress catalog version must end in .v followed by one positive integer")
	}
	version, err := strconv.Atoi(current[separator+2:])
	if err != nil || version < 1 || version == int(^uint(0)>>1) {
		return "", errors.New("stress catalog version suffix is invalid")
	}
	return current[:separator] + ".v" + strconv.Itoa(version+1), nil
}

func relationDiscoveryID(value RoutedRelationDiscovery) (string, error) {
	digest, err := digestDocument(struct {
		SourceCellID            string `json:"source_cell_id"`
		SourceResultDigest      string `json:"source_result_digest"`
		DiscoveryEvidenceDigest string `json:"discovery_evidence_digest"`
		TargetCatalogVersion    string `json:"target_catalog_version"`
		CandidateDigest         string `json:"candidate_digest"`
	}{value.SourceCellID, value.SourceResultDigest, value.DiscoveryEvidenceDigest, value.TargetCatalogVersion, value.Candidate.Digest})
	if err != nil {
		return "", err
	}
	return "discovery-" + digest[:24], nil
}

func heldOutPartitionLockDigest(value HeldOutPartitionLock) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func heldOutRunSealDigest(value HeldOutRunSeal) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func nextVersionDiscoveryLedgerDigest(value NextVersionDiscoveryLedger) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
