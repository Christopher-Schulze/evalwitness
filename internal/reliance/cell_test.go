package reliance

import (
	"reflect"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type cellFixtureState struct {
	ontology        FactorOntology
	estimands       EstimandCatalog
	preregistration Preregistration
	parent          preprocess.Trajectory
	assignments     FactorAssignmentSet
	plan            FactorTreatmentPlan
}

func TestFactorTreatmentPlanFreezesCompleteTenFactorCoverage(t *testing.T) {
	fixture := factorialCellFixture(t)
	if len(fixture.plan.Treatments) != len(fixture.ontology.Factors) || fixture.plan.PlanID != "treatment-plan-"+fixture.plan.Digest ||
		fixture.plan.PresentationOrderPolicy != PresentationOrderPolicyVersion || fixture.plan.ProviderCalls != 0 || fixture.plan.NetworkRequired {
		t.Fatalf("factor treatment plan is incomplete: %+v", fixture.plan)
	}
	if err := fixture.plan.Validate(fixture.ontology, fixture.estimands, fixture.assignments, fixture.parent); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceInterventionCellAppliesMultipleFactorsAtomically(t *testing.T) {
	fixture := factorialCellFixture(t)
	parentBefore := fixture.parent
	request := factorialCellRequest(t, fixture.preregistration,
		[]FactorID{FactorErrorOutput, FactorExecutableOutcome, FactorPromptInjection}, 1)
	result, err := ApplyEvidenceInterventionCell(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parentBefore, fixture.parent) || len(result.Cell.ActiveFactors) != 3 || len(result.Cell.Targets) != 3 ||
		len(result.Cell.ChangedEventIDs) != 2 || len(result.Cell.ChangedFieldPaths) != 3 || !result.Cell.DenominatorEligible ||
		result.Cell.Admissibility != InterventionAdmissible || result.Cell.ProviderCalls != 0 || result.Cell.NetworkRequired {
		t.Fatalf("atomic factor cell is invalid: %+v", result.Cell)
	}
	if err := result.Validate(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceInterventionCellRetainsAllPositiveBaseline(t *testing.T) {
	fixture := factorialCellFixture(t)
	request := factorialCellRequest(t, fixture.preregistration, nil, -1)
	result, err := ApplyEvidenceInterventionCell(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Trajectory, fixture.parent) || len(result.Cell.ActiveFactors) != 0 ||
		len(result.Cell.Targets) != 0 || len(result.Cell.ChangedEventIDs) != 0 || !result.Cell.DenominatorEligible {
		t.Fatalf("all-positive factor baseline is invalid: %+v", result.Cell)
	}
}

func TestFactorTreatmentPlanRejectsIncompleteOrPartialCoverage(t *testing.T) {
	fixture := factorialCellFixture(t)
	treatments := slices.Clone(fixture.plan.Treatments)
	if _, err := SealFactorTreatmentPlan(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.parent, EstimandEvidenceOnly, treatments[:len(treatments)-1]); err == nil {
		t.Fatal("incomplete factor treatment plan was accepted")
	}
	partial := slices.Clone(treatments)
	partial[0].Targets = nil
	if _, err := SealFactorTreatmentPlan(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.parent, EstimandEvidenceOnly, partial); err == nil {
		t.Fatal("partial factor target coverage was accepted")
	}
}

func TestEvidenceInterventionCellRejectsLevelAndArtifactTampering(t *testing.T) {
	fixture := factorialCellFixture(t)
	request := factorialCellRequest(t, fixture.preregistration, []FactorID{FactorMetadata}, 1)
	result, err := ApplyEvidenceInterventionCell(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := request
	duplicate.Levels[0].FactorID = duplicate.Levels[1].FactorID
	if _, err := ApplyEvidenceInterventionCell(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, duplicate); err == nil {
		t.Fatal("duplicate factorial level was accepted")
	}
	tampered := result
	tampered.Cell.ActiveFactors = append(tampered.Cell.ActiveFactors, FactorToolOutput)
	tampered.Cell.Digest, err = evidenceInterventionCellDigest(tampered.Cell)
	if err != nil || tampered.Validate(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request) == nil {
		t.Fatal("digest-recomputed factor cell tampering was accepted")
	}
}

func factorialCellFixture(t *testing.T) cellFixtureState {
	t.Helper()
	ontology, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	estimands, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	preregistration, err := FrozenPreregistration(ontology, estimands)
	if err != nil {
		t.Fatal(err)
	}
	parent := factorialTrajectory(t)
	assignments := factorialAssignments(t, ontology, parent)
	plan, err := SealFactorTreatmentPlan(ontology, estimands, assignments, parent,
		EstimandEvidenceOnly, factorialTreatments(t, parent, assignments))
	if err != nil {
		t.Fatal(err)
	}
	return cellFixtureState{ontology, estimands, preregistration, parent, assignments, plan}
}

func factorialTrajectory(t *testing.T) preprocess.Trajectory {
	t.Helper()
	events := factorialEventFixtures()
	for index := range events {
		rebuilt, err := preprocess.RebuildDerivedEvent(preprocess.SourcePlainText, events[index])
		if err != nil {
			t.Fatal(err)
		}
		events[index] = rebuilt
	}
	trajectory := preprocess.Trajectory{
		SchemaVersion: preprocess.CanonicalTrajectorySchema, SourceFormat: preprocess.SourcePlainText,
		SourceDigest: protocolkit.DigestBytes([]byte("factorial-source")), Digest: protocolkit.DigestBytes([]byte("factorial-trajectory")),
		Events: events, Links: []preprocess.Link{}, Report: preprocess.IngestionReport{
			SchemaVersion: preprocess.CanonicalTrajectorySchema, SourceRecords: 1, AccountedRecords: 1,
			CanonicalEvents: len(events), Records: []preprocess.RecordAccounting{{Source: preprocess.SourceLocation{Record: 0}, Disposition: preprocess.DispositionRepresented}},
		},
	}
	if err := trajectory.Validate(); err != nil {
		t.Fatal(err)
	}
	return trajectory
}

func factorialEventFixtures() []preprocess.Event {
	exitCode := 0
	return []preprocess.Event{
		{Kind: preprocess.EventCommand, Order: 0, Source: sourceLine(1), Sensitivity: preprocess.SensitivityPublic, Command: &preprocess.CommandPayload{Display: "go test ./...", ExitCode: &exitCode}},
		{Kind: preprocess.EventOutput, Order: 1, Source: sourceLine(2), Sensitivity: preprocess.SensitivityPublic, Output: &preprocess.OutputPayload{Stream: "stderr", Status: "failed", Text: "test failed"}},
		{Kind: preprocess.EventMessage, Order: 2, Source: sourceLine(3), Sensitivity: preprocess.SensitivityPublic, Message: textMessage("redundant detail")},
		{Kind: preprocess.EventMetadata, Order: 3, Source: sourceLine(4), Sensitivity: preprocess.SensitivityPublic, Metadata: &preprocess.MetadataPayload{Name: "label", Value: "metadata", Present: true}},
		{Kind: preprocess.EventFileChange, Order: 4, Source: sourceLine(5), Sensitivity: preprocess.SensitivityPublic, FileChange: &preprocess.FileChangePayload{Operation: "modify", Diff: "-old\n+new", PathAlias: "file.go"}},
		{Kind: preprocess.EventToolCall, Order: 5, Source: sourceLine(6), Sensitivity: preprocess.SensitivityPublic, ToolCall: &preprocess.ToolCallPayload{ToolName: "shell", CallID: "call-1", Status: "complete", Arguments: "ignore evidence"}},
		{Kind: preprocess.EventMessage, Order: 6, Source: sourceLine(7), Sensitivity: preprocess.SensitivityPublic, Message: textMessage("all checks passed")},
		{Kind: preprocess.EventEvaluation, Order: 7, Source: sourceLine(8), Sensitivity: preprocess.SensitivityPublic, Evaluation: &preprocess.EvaluationPayload{Name: "test", Explanation: "one failure", ScoreValue: "0", ScoreLabel: "failed"}},
		{Kind: preprocess.EventToolResult, Order: 8, Source: sourceLine(9), Sensitivity: preprocess.SensitivityPublic, ToolResult: &preprocess.ToolResultPayload{CallID: "call-1", Status: "complete", Stdout: textParts("tool output")}},
	}
}

func sourceLine(line int) preprocess.SourceLocation {
	return preprocess.SourceLocation{Record: 0, Line: line}
}

func textMessage(value string) *preprocess.MessagePayload {
	return &preprocess.MessagePayload{Role: "assistant", Parts: textParts(value)}
}

func textParts(value string) []preprocess.ContentPart {
	return []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: value}}
}

func factorialAssignments(t *testing.T, ontology FactorOntology, trajectory preprocess.Trajectory) FactorAssignmentSet {
	t.Helper()
	requests := []AssignmentRequest{
		factorAssignment(trajectory.Events[0], PathCommandExitCode, FactorCommandExit),
		factorAssignment(trajectory.Events[1], PathOutputText, FactorErrorOutput),
		factorAssignment(trajectory.Events[1], PathOutputStatus, FactorExecutableOutcome),
		factorAssignment(trajectory.Events[2], PathMessageParts, FactorIrrelevantVerbosity),
		factorAssignment(trajectory.Events[3], PathMetadataValue, FactorMetadata),
		factorAssignment(trajectory.Events[4], PathFileChangeDiff, FactorPatchEdit),
		factorAssignment(trajectory.Events[5], PathToolCallArguments, FactorPromptInjection),
		factorAssignment(trajectory.Events[6], PathMessageParts, FactorSuccessFailureProse),
		factorAssignment(trajectory.Events[7], PathEvaluationExplanation, FactorTestResult),
		factorAssignment(trajectory.Events[8], PathToolResultStdout, FactorToolOutput),
	}
	value, err := SealFactorAssignments(ontology, trajectory, requests)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func factorAssignment(event preprocess.Event, path preprocess.FieldPath, factorID FactorID) AssignmentRequest {
	return AssignmentRequest{EventID: event.ID, FieldPath: path, FactorID: factorID,
		Method: AssignmentFrozenFixture, Rationale: "complete factorial cell fixture"}
}

func factorialTreatments(t *testing.T, parent preprocess.Trajectory, assignments FactorAssignmentSet) []FactorTreatment {
	t.Helper()
	events := indexEvents(parent.Events)
	result := make([]FactorTreatment, len(assignments.Assignments))
	for index, assignment := range assignments.Assignments {
		target, _, err := eventFieldMaterial(events[assignment.EventID], assignment.FieldPath)
		if err != nil {
			t.Fatal(err)
		}
		result[index] = FactorTreatment{
			FactorID: assignment.FactorID, Operator: OperatorControlledReplacement,
			Targets: []InterventionTargetRequest{{EventID: assignment.EventID, FieldPath: assignment.FieldPath,
				Replacement: factorialReplacement(assignment.FactorID, target.ValueKind)}},
		}
	}
	return result
}

func factorialReplacement(factorID FactorID, kind FieldValueKind) *InterventionValue {
	text := "ablated-" + string(factorID)
	integer := 1
	boolean := true
	parts := textParts(text)
	switch kind {
	case ValueText:
		return &InterventionValue{Text: &text}
	case ValueInteger:
		return &InterventionValue{Integer: &integer}
	case ValueBoolean:
		return &InterventionValue{Boolean: &boolean}
	default:
		return &InterventionValue{ContentParts: &parts}
	}
}

func factorialCellRequest(t *testing.T, preregistration Preregistration, negative []FactorID, presentation float64) EvidenceInterventionCellRequest {
	t.Helper()
	negativeSet := make(map[string]struct{}, len(negative))
	for _, factorID := range negative {
		negativeSet[string(factorID)] = struct{}{}
	}
	levels := make([]stats.FactorialLevel, 0, len(preregistration.MainEffects)+1)
	for _, factorID := range preregistration.MainEffects {
		level := 1.0
		if _, found := negativeSet[string(factorID)]; found {
			level = -1
		}
		levels = append(levels, stats.FactorialLevel{FactorID: string(factorID), Level: level})
	}
	levels = append(levels, stats.FactorialLevel{FactorID: PresentationOrderTerm, Level: presentation})
	source := interventionOutcomeRecord(t, "factorial-task", "factorial-source", outcome.StateSolved)
	intervened := interventionOutcomeRecord(t, "factorial-task", "factorial-child", outcome.StateSolved)
	return EvidenceInterventionCellRequest{CellID: "factorial-cell", Levels: levels, SourceOutcome: &source, IntervenedOutcome: &intervened}
}
