package reliance

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type relianceAnalysisFixtureState struct {
	ontology        FactorOntology
	estimands       EstimandCatalog
	preregistration Preregistration
	preflight       ReliancePreflight
	registration    ReliancePanelRegistration
	executions      []EvidenceTaskPanelExecution
	sources         []EvidenceSelectorAuditSource
}

var relianceAnalysisFixtureCache struct {
	once  sync.Once
	value relianceAnalysisFixtureState
}

func TestReliancePanelRegistrationAndFailureReceiptAreFailClosed(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	sourceTaskIDs := fixture.registration.SourceTaskIDs()
	if fixture.registration.RegisteredCells != 1_536 || fixture.registration.PlannedLogicalCalls != 1_560 {
		t.Fatalf("registration dimensions = %+v", fixture.registration)
	}
	tampered := fixture.registration
	tampered.FreezeStage = "after_verifier_output"
	var err error
	tampered.Digest, err = reliancePanelRegistrationDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.Validate(); err == nil {
		t.Fatal("resealed post-output registration was accepted")
	}
	receipt := analysisFailureReceipt(t, fixture, sourceTaskIDs[0], 0, RelianceCellProviderFailed)
	receipt.Levels[0].Level *= -1
	receipt.Digest, err = relianceCellFailureReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.Validate(fixture.registration, fixture.preregistration); err == nil {
		t.Fatal("resealed failure-cell assignment was accepted")
	}
}

func TestReliancePanelRegistrationRejectsPseudoreplicatedSourceTrajectory(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	tampered := fixture.registration
	tampered.SourceTasks = append([]RelianceSourceTaskRegistration(nil), fixture.registration.SourceTasks...)
	tampered.SourceTasks[1].SourceTrajectoryDigest = tampered.SourceTasks[0].SourceTrajectoryDigest
	var err error
	tampered.Digest, err = reliancePanelRegistrationDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.Validate(); err == nil {
		t.Fatal("registration accepted one trajectory under two independent task clusters")
	}
}

func TestEvidenceRelianceAnalysisFitsEveryPreregisteredOutcome(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	corpus, err := BuildRelianceAnalysisCorpus(
		fixture.registration, fixture.preregistration, fixture.executions, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertCompleteRelianceCorpus(t, corpus, 1_536, 24, 0)
	analysis, err := AnalyzeEvidenceReliance(
		fixture.registration, fixture.preregistration, fixture.executions, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertMeasuredRelianceFits(t, analysis, 1_536, 24)
	if err := analysis.Validate(fixture.registration, fixture.preregistration, fixture.executions, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceRelianceAnalysisRetainsFailedCellsWithoutImputation(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	executions := fixture.executions[:len(fixture.executions)-1]
	sourceTaskIDs := fixture.registration.SourceTaskIDs()
	failedTask := sourceTaskIDs[len(sourceTaskIDs)-1]
	failures := analysisTaskFailures(t, fixture, []string{failedTask}, RelianceCellProviderFailed)
	corpus, err := BuildRelianceAnalysisCorpus(fixture.registration, fixture.preregistration, executions, failures)
	if err != nil {
		t.Fatal(err)
	}
	assertCompleteRelianceCorpus(t, corpus, 1_472, 23, 64)
	analysis, err := AnalyzeEvidenceReliance(fixture.registration, fixture.preregistration, executions, failures)
	if err != nil {
		t.Fatal(err)
	}
	assertMeasuredRelianceFits(t, analysis, 1_472, 23)
	if corpus.RegisteredCells != 1_536 || corpus.OutcomeBearingCells+corpus.FailureReceipts != 1_536 {
		t.Fatalf("failed cells escaped denominator: %+v", corpus)
	}
}

func TestEvidenceRelianceAnalysisIsTypedInconclusiveWhenClustersCollapse(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	failedTasks := fixture.registration.SourceTaskIDs()[1:]
	failures := analysisTaskFailures(t, fixture, failedTasks, RelianceCellRouteFailed)
	analysis, err := AnalyzeEvidenceReliance(
		fixture.registration, fixture.preregistration, fixture.executions[:1], failures,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range analysis.OutcomeFits {
		if outcome.Status != RelianceFitInconclusive || outcome.Fit != nil || outcome.EligibleObservations != 64 ||
			outcome.ExcludedFromFit != 1_472 || !strings.Contains(outcome.Reason, "clustered factorial fit unavailable") {
			t.Fatalf("collapsed-cluster outcome = %+v", outcome)
		}
	}
}

func TestEvidenceRelianceAnalysisIsTypedInconclusiveWithoutMeasuredCells(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	failures := analysisTaskFailures(t, fixture, fixture.registration.SourceTaskIDs(), RelianceCellBudgetExhausted)
	analysis, err := AnalyzeEvidenceReliance(fixture.registration, fixture.preregistration, nil, failures)
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range analysis.OutcomeFits {
		if outcome.Status != RelianceFitInconclusive || outcome.EligibleObservations != 0 ||
			outcome.ExcludedFromFit != 1_536 || outcome.Reason != "clustered factorial fit unavailable: no outcome-bearing cells" {
			t.Fatalf("empty-outcome fit = %+v", outcome)
		}
	}
}

func TestRelianceAnalysisRejectsMissingOrDuplicateRegisteredCells(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	sourceTaskIDs := fixture.registration.SourceTaskIDs()
	failedTask := sourceTaskIDs[len(sourceTaskIDs)-1]
	failures := analysisTaskFailures(t, fixture, []string{failedTask}, RelianceCellUnsupported)
	executions := fixture.executions[:len(fixture.executions)-1]
	if _, err := BuildRelianceAnalysisCorpus(
		fixture.registration, fixture.preregistration, executions, failures[:len(failures)-1],
	); err == nil {
		t.Fatal("analysis accepted an omitted registered cell")
	}
	failures = append(failures, failures[0])
	if _, err := BuildRelianceAnalysisCorpus(
		fixture.registration, fixture.preregistration, executions, failures,
	); err == nil {
		t.Fatal("analysis accepted a duplicate registered cell")
	}
}

func TestRelianceAnalysisRejectsMixedRegisteredArms(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	executions := append([]EvidenceTaskPanelExecution(nil), fixture.executions...)
	executions[0].ReplaySource.RequestedModel = "foreign-model"
	var err error
	executions[0].Digest, err = evidenceTaskPanelDigest(executions[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := executions[0].Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildRelianceAnalysisCorpus(
		fixture.registration, fixture.preregistration, executions, nil,
	); err == nil {
		t.Fatal("analysis pooled a panel from a foreign model arm")
	}
}

func TestEvidenceRelianceAnalysisRejectsResealedFitTampering(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	analysis, err := AnalyzeEvidenceReliance(fixture.registration, fixture.preregistration, fixture.executions, nil)
	if err != nil {
		t.Fatal(err)
	}
	analysis.OutcomeFits[0].Fit.Estimates[0].Estimate += 0.01
	analysis.Digest, err = evidenceRelianceAnalysisDigest(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if err := analysis.Validate(fixture.registration, fixture.preregistration, fixture.executions, nil); err == nil {
		t.Fatal("resealed evidence-reliance estimate was accepted")
	}
}

func relianceAnalysisFixture(t *testing.T) relianceAnalysisFixtureState {
	t.Helper()
	relianceAnalysisFixtureCache.once.Do(func() {
		relianceAnalysisFixtureCache.value = buildRelianceAnalysisFixture(t)
	})
	return relianceAnalysisFixtureCache.value
}

func buildRelianceAnalysisFixture(t *testing.T) relianceAnalysisFixtureState {
	t.Helper()
	base := factorialCellFixture(t)
	preflight, err := BuildReliancePreflight(base.preregistration, strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	sourceTaskIDs := analysisSourceTaskIDs()
	sources := analysisSelectorSources(t, base, sourceTaskIDs)
	sourceTasks := analysisSourceTaskRegistrations(sources)
	studyDigest := protocolkit.DigestBytes([]byte("reliance-analysis-study"))
	arm := analysisFixtureArm()
	registration, err := SealReliancePanelRegistration(base.preregistration, preflight, studyDigest, arm, sourceTasks)
	if err != nil {
		t.Fatal(err)
	}
	executions := make([]EvidenceTaskPanelExecution, len(sourceTaskIDs))
	for index, sourceTaskID := range sourceTaskIDs {
		executions[index] = syntheticAnalysisPanel(t, base.preregistration, registration, sourceTaskID, index)
	}
	return relianceAnalysisFixtureState{
		base.ontology, base.estimands, base.preregistration, preflight, registration, executions, sources,
	}
}

func analysisSourceTaskIDs() []string {
	result := make([]string, RelianceSelectedSourceTasks)
	for index := range result {
		result[index] = fmt.Sprintf("analysis-task-%02d", index+1)
	}
	return result
}

func analysisSourceTaskRegistrations(sources []EvidenceSelectorAuditSource) []RelianceSourceTaskRegistration {
	result := make([]RelianceSourceTaskRegistration, len(sources))
	for index, source := range sources {
		result[index] = RelianceSourceTaskRegistration{
			SourceTaskID: source.SourceTaskID, SourceTrajectoryDigest: source.Trajectory.Digest,
			AssignmentSetDigest: source.Assignments.Digest, TreatmentPlanDigest: source.TreatmentPlan.Digest,
			OutcomeEvidenceSetDigest: analysisDigest("outcomes", source.SourceTaskID),
		}
	}
	return result
}

func analysisSelectorSources(
	t *testing.T,
	base cellFixtureState,
	sourceTaskIDs []string,
) []EvidenceSelectorAuditSource {
	t.Helper()
	result := make([]EvidenceSelectorAuditSource, len(sourceTaskIDs))
	for index, sourceTaskID := range sourceTaskIDs {
		trajectory := analysisSelectorTrajectory(t, base.parent, sourceTaskID)
		assignments := factorialAssignments(t, base.ontology, trajectory)
		plan, err := SealFactorTreatmentPlan(base.ontology, base.estimands, assignments, trajectory,
			EstimandEvidenceOnly, factorialTreatments(t, trajectory, assignments))
		if err != nil {
			t.Fatal(err)
		}
		result[index] = EvidenceSelectorAuditSource{sourceTaskID, trajectory, assignments, plan}
	}
	return result
}

func analysisSelectorTrajectory(
	t *testing.T,
	parent preprocess.Trajectory,
	sourceTaskID string,
) preprocess.Trajectory {
	t.Helper()
	events := append([]preprocess.Event(nil), parent.Events...)
	changedEventIDs := []string{events[2].ID, events[3].ID, events[7].ID}
	events[2].Message = textMessage(sourceTaskID + ":" + strings.Repeat("irrelevant verbose detail ", 14_000))
	events[3].Metadata = &preprocess.MetadataPayload{
		Name: "label", Value: sourceTaskID + ":" + strings.Repeat("metadata detail ", 3_000), Present: true,
	}
	events[7].Evaluation = &preprocess.EvaluationPayload{
		Name: "test", Explanation: sourceTaskID + ":" + strings.Repeat("failed evidence ", 12_000),
		ScoreValue: "0", ScoreLabel: "failed",
	}
	for _, index := range []int{2, 3, 7} {
		rebuilt, err := preprocess.RebuildDerivedEvent(parent.SourceFormat, events[index])
		if err != nil {
			t.Fatal(err)
		}
		events[index] = rebuilt
	}
	result, err := preprocess.DeriveTrajectory(parent, events, parent.Links, preprocess.DerivationSpec{
		Relation: "selector_audit_source_fixture", Validator: "evalwitness.selector-audit-source-fixture.v1",
		ChangedEventIDs: changedEventIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func analysisFixtureArm() RelianceAnalysisArm {
	return RelianceAnalysisArm{
		Entrypoint: "reliance-analysis-fixture", CriterionID: "correctness",
		ScoreTag:       referenceScoreTag,
		EvidencePolicy: verification.EvidenceStrictVerifier, ProviderID: "exact-replay",
		RouteID:        "route-" + protocolkit.DigestBytes([]byte("reliance-analysis-route")),
		RequestedModel: "reference-adapter",
	}
}

func syntheticAnalysisPanel(
	t *testing.T,
	preregistration Preregistration,
	registration ReliancePanelRegistration,
	sourceTaskID string,
	taskIndex int,
) EvidenceTaskPanelExecution {
	t.Helper()
	registered, found := registration.sourceTask(sourceTaskID)
	if !found {
		t.Fatalf("source task %q is absent from registration", sourceTaskID)
	}
	records := syntheticAnalysisRecords(t, sourceTaskID, taskIndex)
	baseline := syntheticAnalysisBaseline(t, records[0])
	cells := make([]EvidenceTaskPanelCell, len(records))
	for index, record := range records {
		cells[index] = syntheticAnalysisPanelCell(t, sourceTaskID, record, baseline)
	}
	value := EvidenceTaskPanelExecution{
		SchemaVersion: EvidenceTaskPanelSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		SourceTaskID: sourceTaskID, PreregistrationDigest: preregistration.Digest,
		TreatmentPlanDigest: registered.TreatmentPlanDigest, AssignmentSetDigest: registered.AssignmentSetDigest,
		SourceTrajectoryDigest: registered.SourceTrajectoryDigest, StudyManifestDigest: registration.StudyManifestDigest,
		OutcomeEvidenceSetDigest: registered.OutcomeEvidenceSetDigest, Entrypoint: registration.Arm.Entrypoint,
		EvidencePolicy: registration.Arm.EvidencePolicy, BatchRunFingerprint: analysisDigest("batch", sourceTaskID),
		ReplaySource:   syntheticAnalysisReplaySource(sourceTaskID, registration.Arm),
		BaselineReplay: syntheticPanelReplayReference(sourceTaskID, "baseline"), BaselineState: records[0].BaselineState,
		BaselineEvidence: []PanelScoreEvidence{baseline}, Cells: cells,
		LogicalCalls: ReferenceCellsPerTask + 1, NetworkRequired: false,
	}
	digest, err := evidenceTaskPanelDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	return value
}

func syntheticAnalysisBaseline(t *testing.T, record ReferenceRecord) PanelScoreEvidence {
	t.Helper()
	digest, err := referenceJSONDigest(record.BaselineEvidence)
	if err != nil {
		t.Fatal(err)
	}
	return PanelScoreEvidence{"correctness", 0, record.BaselineEvidence, digest}
}

func syntheticAnalysisRecords(t *testing.T, sourceTaskID string, taskIndex int) []ReferenceRecord {
	t.Helper()
	result := make([]ReferenceRecord, ReferenceCellsPerTask)
	for cellIndex := range result {
		record, err := constructReferenceRecord(
			taskIndex%ReferenceSourceTasks, cellIndex, canonicalReferenceMasks(), canonicalReferenceTermEffects(),
		)
		if err != nil {
			t.Fatal(err)
		}
		record.SourceTaskID = sourceTaskID
		record.ObservationID = relianceObservationID(sourceTaskID, cellIndex)
		result[cellIndex] = record
	}
	return result
}

func syntheticAnalysisPanelCell(
	t *testing.T,
	sourceTaskID string,
	record ReferenceRecord,
	baseline PanelScoreEvidence,
) EvidenceTaskPanelCell {
	t.Helper()
	interventionDigest, err := referenceJSONDigest(record.InterventionEvidence)
	if err != nil {
		t.Fatal(err)
	}
	contrast := PanelCriterionContrast{
		CriterionID: baseline.CriterionID, Repetition: 0, BaselineEvidenceDigest: baseline.EvidenceDigest,
		InterventionEvidence: record.InterventionEvidence, InterventionEvidenceDigest: interventionDigest,
		Comparison: record.Comparison,
	}
	return EvidenceTaskPanelCell{
		CellIndex: record.Cell, CellID: fmt.Sprintf("%s-factorial-cell-%02d", sourceTaskID, record.Cell),
		CellDigest: analysisDigest("cell", record.ObservationID), PresentationDigest: analysisDigest("presentation", record.ObservationID),
		Levels: record.Levels, Replay: syntheticPanelReplayReference(sourceTaskID, fmt.Sprintf("cell-%02d", record.Cell)),
		BaselineState: record.BaselineState, InterventionState: record.InterventionState,
		DecisionFlip: record.DecisionFlip, AbstentionTransition: record.AbstentionTransition,
		CriterionContrasts: []PanelCriterionContrast{contrast},
	}
}

func syntheticAnalysisReplaySource(sourceTaskID string, arm RelianceAnalysisArm) provider.ExactReplaySource {
	return provider.ExactReplaySource{
		SchemaVersion: provider.ExactReplaySourceSchemaVersion, CaptureSHA256: analysisDigest("capture", sourceTaskID),
		Bytes: 6_500, Records: ReferenceCellsPerTask + 1, CaptureSchemaVersion: provider.CaptureSchemaVersion,
		RequestSchemaVersion: provider.RequestSchemaVersion, ParserContractVersion: provider.ParserContractVersion,
		ProviderID: arm.ProviderID, RouteID: arm.RouteID, RequestedModel: arm.RequestedModel,
		RequestSetDigest: analysisDigest("requests", sourceTaskID), LineageSetDigest: analysisDigest("lineage", sourceTaskID),
		ResponseBodySetDigest: analysisDigest("responses", sourceTaskID), EvidenceSetDigest: analysisDigest("evidence", sourceTaskID),
		RecordSetDigest: analysisDigest("records", sourceTaskID),
	}
}

func syntheticPanelReplayReference(sourceTaskID, label string) PanelReplayReference {
	return PanelReplayReference{
		InputDigest:          analysisDigest("input", sourceTaskID, label),
		PlanFingerprint:      analysisDigest("plan", sourceTaskID, label),
		ObservationSetDigest: analysisDigest("observations", sourceTaskID, label),
		StageTraceDigest:     analysisDigest("trace", sourceTaskID, label),
	}
}

func analysisDigest(parts ...string) string {
	return protocolkit.DigestBytes([]byte(strings.Join(parts, ":")))
}

func analysisTaskFailures(
	t *testing.T,
	fixture relianceAnalysisFixtureState,
	sourceTaskIDs []string,
	status RelianceCellStatus,
) []RelianceCellFailureReceipt {
	t.Helper()
	result := make([]RelianceCellFailureReceipt, 0, len(sourceTaskIDs)*ReferenceCellsPerTask)
	for _, sourceTaskID := range sourceTaskIDs {
		for cellIndex := 0; cellIndex < ReferenceCellsPerTask; cellIndex++ {
			result = append(result, analysisFailureReceipt(t, fixture, sourceTaskID, cellIndex, status))
		}
	}
	return result
}

func analysisFailureReceipt(
	t *testing.T,
	fixture relianceAnalysisFixtureState,
	sourceTaskID string,
	cellIndex int,
	status RelianceCellStatus,
) RelianceCellFailureReceipt {
	t.Helper()
	receipt, err := SealRelianceCellFailureReceipt(
		fixture.registration, fixture.preregistration, sourceTaskID, cellIndex, status,
		"evalwitness.test-failure-evidence.v1",
		analysisDigest("failure-evidence", sourceTaskID, fmt.Sprint(cellIndex)), 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func assertCompleteRelianceCorpus(
	t *testing.T,
	corpus RelianceAnalysisCorpus,
	outcomeBearing int,
	panels int,
	failures int,
) {
	t.Helper()
	if corpus.RegisteredCells != 1_536 || len(corpus.Cells) != 1_536 || corpus.OutcomeBearingCells != outcomeBearing ||
		corpus.PanelExecutions != panels || corpus.FailureReceipts != failures || corpus.Imputation != "none" {
		t.Fatalf("reliance corpus dimensions = registered %d cells %d outcomes %d panels %d failures %d imputation %q",
			corpus.RegisteredCells, len(corpus.Cells), corpus.OutcomeBearingCells,
			corpus.PanelExecutions, corpus.FailureReceipts, corpus.Imputation)
	}
}

func assertMeasuredRelianceFits(
	t *testing.T,
	analysis EvidenceRelianceAnalysis,
	observations int,
	clusters int,
) {
	t.Helper()
	if len(analysis.OutcomeFits) != 7 || analysis.MultiplicityFamilySize != 98 {
		t.Fatalf("analysis outcome family = %d fits, size %d", len(analysis.OutcomeFits), analysis.MultiplicityFamilySize)
	}
	for _, outcome := range analysis.OutcomeFits {
		if outcome.Status != RelianceFitMeasured || outcome.Fit == nil || outcome.Fit.Observations != observations ||
			outcome.Fit.Clusters != clusters || outcome.Fit.FamilySize != 98 || outcome.ExcludedFromFit != 1_536-observations {
			t.Fatalf("measured outcome %s = %+v", outcome.OutcomeID, outcome)
		}
	}
}

func TestSyntheticAnalysisPanelsRemainStandaloneValid(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	for _, execution := range fixture.executions {
		if err := execution.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if reflect.DeepEqual(fixture.executions[0], fixture.executions[1]) {
		t.Fatal("synthetic task panels collapsed distinct source-task evidence")
	}
}
