package reliance

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestEvidenceInterventionCoversFrozenOperatorTargetMatrix(t *testing.T) {
	ontology, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	estimands, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	parent := completeInterventionTrajectory(t)
	source := interventionOutcomeRecord(t, "complete-task", "complete-source", outcome.StateSolved)
	intervened := interventionOutcomeRecord(t, "complete-task", "complete-intervened", outcome.StateSolved)
	for _, operator := range ontology.Operators {
		for _, target := range operator.AllowedTargets {
			name := fmt.Sprintf("%s/%s%s", operator.Operator, target.EventKind, target.FieldPath)
			t.Run(name, func(t *testing.T) {
				testInterventionOperatorTarget(t, ontology, estimands, parent, source, intervened, operator.Operator, target)
			})
		}
	}
}

func testInterventionOperatorTarget(
	t *testing.T,
	ontology FactorOntology,
	estimands EstimandCatalog,
	parent preprocess.Trajectory,
	source, intervened outcome.Record,
	operator InterventionOperator,
	target FieldTarget,
) {
	t.Helper()
	event := eventOfKind(t, parent, target.EventKind)
	factorID := interventionFactorForTarget(t, ontology, target)
	assignments := interventionAssignments(t, ontology, parent, factorID, event.ID, target.FieldPath)
	request := InterventionTargetRequest{EventID: event.ID, FieldPath: target.FieldPath}
	if operator == OperatorControlledReplacement {
		request.Replacement = interventionReplacementForTarget(t, target)
	}
	result, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, EvidenceInterventionRequest{
		FactorID: factorID, Operator: operator, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{request}, SourceOutcome: &source, IntervenedOutcome: &intervened,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intervention.Admissibility != InterventionAdmissible || !result.Intervention.DenominatorEligible {
		t.Fatalf("operator target is not admissible: %+v", result.Intervention)
	}
}

func interventionFactorForTarget(t *testing.T, ontology FactorOntology, target FieldTarget) FactorID {
	t.Helper()
	for _, factor := range ontology.Factors {
		if factor.Allows(target) {
			return factor.FactorID
		}
	}
	t.Fatalf("no factor admits target %+v", target)
	return ""
}

func interventionReplacementForTarget(t *testing.T, target FieldTarget) *InterventionValue {
	t.Helper()
	text := "replacement:" + string(target.FieldPath)
	integer := 47
	boolean := target.FieldPath == PathToolResultError
	parts := []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: text}}
	switch target.ValueKind {
	case ValueText:
		return &InterventionValue{Text: &text}
	case ValueInteger:
		return &InterventionValue{Integer: &integer}
	case ValueBoolean:
		return &InterventionValue{Boolean: &boolean}
	case ValueContentParts:
		return &InterventionValue{ContentParts: &parts}
	default:
		t.Fatalf("unsupported intervention target value kind %q", target.ValueKind)
		return nil
	}
}

func completeInterventionTrajectory(t *testing.T) preprocess.Trajectory {
	t.Helper()
	events := append(completeExecutionInterventionEvents(), completeNarrativeInterventionEvents()...)
	for index := range events {
		events[index].Order = index
		events[index].Source = preprocess.SourceLocation{Record: 0, Line: index + 1}
		events[index].Sensitivity = preprocess.SensitivityPublic
		rebuilt, err := preprocess.RebuildDerivedEvent(preprocess.SourcePlainText, events[index])
		if err != nil {
			t.Fatal(err)
		}
		events[index] = rebuilt
	}
	trajectory := preprocess.Trajectory{
		SchemaVersion: preprocess.CanonicalTrajectorySchema, SourceFormat: preprocess.SourcePlainText,
		SourceDigest: protocolkit.DigestBytes([]byte("complete-intervention-source")),
		Digest:       protocolkit.DigestBytes([]byte("complete-intervention-trajectory")), Events: events,
		Report: preprocess.IngestionReport{
			SchemaVersion: preprocess.CanonicalTrajectorySchema, SourceRecords: 1, AccountedRecords: 1,
			CanonicalEvents: len(events), Records: []preprocess.RecordAccounting{{Source: preprocess.SourceLocation{Record: 0}, Disposition: preprocess.DispositionRepresented}},
		},
	}
	if err := trajectory.Validate(); err != nil {
		t.Fatal(err)
	}
	return trajectory
}

func completeExecutionInterventionEvents() []preprocess.Event {
	exitCode := 0
	return []preprocess.Event{
		{Kind: preprocess.EventCommand, Command: &preprocess.CommandPayload{
			Display: "go test ./...", Argv: []string{"go", "test", "./..."},
			OperandsDigest: protocolkit.DigestBytes([]byte("go test ./...")), ExitCode: &exitCode,
		}},
		{Kind: preprocess.EventError, Error: &preprocess.ErrorPayload{Class: "test_failure", SafeMessage: "one test failed"}},
		{Kind: preprocess.EventEvaluation, Evaluation: &preprocess.EvaluationPayload{
			Name: "tests", ScoreValue: "0.9", ScoreLabel: "pass", Explanation: "tests passed", ErrorType: "none",
		}},
		{Kind: preprocess.EventFileChange, FileChange: &preprocess.FileChangePayload{
			Operation: "update", PathAlias: "source.go", Diff: "+change", SizeBytes: 7,
			ContentDigest: protocolkit.DigestBytes([]byte("content")), DiffDigest: protocolkit.DigestBytes([]byte("+change")),
		}},
	}
}

func completeNarrativeInterventionEvents() []preprocess.Event {
	exitCode := 0
	return []preprocess.Event{
		{Kind: preprocess.EventMessage, Message: &preprocess.MessagePayload{
			Role: "assistant", Phase: "final", Parts: []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "all checks passed"}},
		}},
		{Kind: preprocess.EventMetadata, Metadata: &preprocess.MetadataPayload{
			Name: "route", Value: "fixture", ValueDigest: protocolkit.DigestBytes([]byte("fixture")), Present: true,
		}},
		{Kind: preprocess.EventOutput, Output: &preprocess.OutputPayload{Stream: "stdout", Text: "PASS", Status: "completed"}},
		{Kind: preprocess.EventToolCall, ToolCall: &preprocess.ToolCallPayload{ToolName: "shell", Arguments: "go test ./..."}},
		{Kind: preprocess.EventToolResult, ToolResult: &preprocess.ToolResultPayload{
			Status: "completed", Error: false, ExitCode: &exitCode,
			Stdout: []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "PASS"}},
			Stderr: []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "warning"}},
			Output: []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "complete"}},
		}},
	}
}

func TestEvidenceInterventionControlledReplacementPreservesLineageAndOutcome(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	message := eventOfKind(t, parent, preprocess.EventMessage)
	toolResult := eventOfKind(t, parent, preprocess.EventToolResult)
	parent.Links = []preprocess.Link{{Kind: preprocess.LinkReference, FromID: toolResult.ID, ToID: message.ID}}
	assignments := interventionAssignments(t, ontology, parent, FactorSuccessFailureProse, message.ID, PathMessageParts)
	sourceOutcome := interventionOutcomeRecord(t, "task-fixture", "source-solved", outcome.StateSolved)
	intervenedOutcome := interventionOutcomeRecord(t, "task-fixture", "intervened-solved", outcome.StateSolved)
	replacement := []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "The task failed despite the executable result."}}

	result, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, EvidenceInterventionRequest{
		FactorID: FactorSuccessFailureProse, Operator: OperatorControlledReplacement, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{{
			EventID: message.ID, FieldPath: PathMessageParts,
			Replacement: &InterventionValue{ContentParts: &replacement},
		}},
		SourceOutcome: &sourceOutcome, IntervenedOutcome: &intervenedOutcome,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertControlledReplacementResult(t, result, ontology, estimands, assignments, parent, message, toolResult)
}

func assertControlledReplacementResult(
	t *testing.T,
	result EvidenceInterventionResult,
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	message, toolResult preprocess.Event,
) {
	t.Helper()
	if err := result.Validate(ontology, estimands, assignments, parent); err != nil {
		t.Fatal(err)
	}
	intervention := result.Intervention
	if intervention.Admissibility != InterventionAdmissible || !intervention.DenominatorEligible ||
		intervention.OutcomePreservation == nil || !intervention.OutcomePreservation.Admissible {
		t.Fatalf("preserved intervention is not admissible: %+v", intervention)
	}
	if intervention.ProviderCalls != 0 || intervention.NetworkRequired || intervention.InterventionID != "intervention-"+intervention.Digest {
		t.Fatalf("intervention execution or identity boundary is invalid: %+v", intervention)
	}
	if result.Trajectory.SourceDigest != parent.SourceDigest || result.Trajectory.Derivation == nil ||
		result.Trajectory.Derivation.ParentDigest != parent.Digest ||
		result.Trajectory.Derivation.Validator != EvidenceInterventionValidator {
		t.Fatalf("canonical derivation lineage is invalid: %+v", result.Trajectory.Derivation)
	}
	target := intervention.Targets[0]
	if target.ParentEventID != message.ID || target.IntervenedEventID == message.ID || !target.Changed ||
		!slices.Equal(intervention.ChangedFieldPaths, []preprocess.FieldPath{derivedInterventionFieldPath(message.ID, PathMessageParts)}) {
		t.Fatalf("changed target accounting is invalid: %+v", intervention)
	}
	if len(result.Trajectory.Links) != 1 || result.Trajectory.Links[0].ToID != target.IntervenedEventID ||
		result.Trajectory.Links[0].FromID != toolResult.ID {
		t.Fatalf("canonical links were not remapped exactly: %+v", result.Trajectory.Links)
	}
	if parent.Events[1].Message.Parts[0].Text != "All checks succeeded." {
		t.Fatal("intervention mutated its parent trajectory")
	}
}

func TestEvidenceInterventionOperatorsAndTypedAdmissibility(t *testing.T) {
	t.Run("remove", testRemoveIntervention)
	t.Run("typed mask", testTypedMaskIntervention)
	t.Run("retain", testRetainIntervention)
}

func testRemoveIntervention(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	metadata := eventOfKind(t, parent, preprocess.EventMetadata)
	metadataAssignments := interventionAssignments(t, ontology, parent, FactorMetadata, metadata.ID, PathMetadataValue)
	removed, err := ApplyEvidenceIntervention(ontology, estimands, metadataAssignments, parent, EvidenceInterventionRequest{
		FactorID: FactorMetadata, Operator: OperatorRemove, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{{EventID: metadata.ID, FieldPath: PathMetadataValue}},
	})
	if err != nil {
		t.Fatal(err)
	}
	removedMetadata := eventOfKind(t, removed.Trajectory, preprocess.EventMetadata)
	if removedMetadata.Metadata.Value != "" || removed.Intervention.Admissibility != InterventionUnresolved ||
		removed.Intervention.DenominatorEligible ||
		!slices.Equal(removed.Intervention.AdmissibilityReasons, []InterventionFailureCode{FailureOutcomePreservationMissing}) {
		t.Fatalf("remove or missing-outcome state is invalid: %+v", removed.Intervention)
	}
}

func testTypedMaskIntervention(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	toolResult := eventOfKind(t, parent, preprocess.EventToolResult)
	toolAssignments := interventionAssignments(t, ontology, parent, FactorToolOutput, toolResult.ID, PathToolResultStdout)
	sourceOutcome := interventionOutcomeRecord(t, "task-fixture", "source-mask", outcome.StateSolved)
	intervenedOutcome := interventionOutcomeRecord(t, "task-fixture", "intervened-mask", outcome.StateSolved)
	masked, err := ApplyEvidenceIntervention(ontology, estimands, toolAssignments, parent, EvidenceInterventionRequest{
		FactorID: FactorToolOutput, Operator: OperatorTypedMask, EstimandFamily: EstimandEvidenceOnly,
		Targets:       []InterventionTargetRequest{{EventID: toolResult.ID, FieldPath: PathToolResultStdout}},
		SourceOutcome: &sourceOutcome, IntervenedOutcome: &intervenedOutcome,
	})
	if err != nil {
		t.Fatal(err)
	}
	maskedTool := eventOfKind(t, masked.Trajectory, preprocess.EventToolResult)
	if len(maskedTool.ToolResult.Stdout) != 1 || maskedTool.ToolResult.Stdout[0].Text != TypedMaskText ||
		masked.Intervention.Admissibility != InterventionAdmissible {
		t.Fatalf("typed mask is invalid: %+v", maskedTool.ToolResult.Stdout)
	}
}

func testRetainIntervention(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	message := eventOfKind(t, parent, preprocess.EventMessage)
	retainAssignments := interventionAssignments(t, ontology, parent, FactorSuccessFailureProse, message.ID, PathMessageParts)
	sourceOutcome := interventionOutcomeRecord(t, "task-fixture", "source-retain", outcome.StateSolved)
	intervenedOutcome := interventionOutcomeRecord(t, "task-fixture", "intervened-retain", outcome.StateSolved)
	retained, err := ApplyEvidenceIntervention(ontology, estimands, retainAssignments, parent, EvidenceInterventionRequest{
		FactorID: FactorSuccessFailureProse, Operator: OperatorRetain, EstimandFamily: EstimandEvidenceOnly,
		Targets:       []InterventionTargetRequest{{EventID: message.ID, FieldPath: PathMessageParts}},
		SourceOutcome: &sourceOutcome, IntervenedOutcome: &intervenedOutcome,
	})
	if err != nil {
		t.Fatal(err)
	}
	if retained.Intervention.Admissibility != InterventionAdmissible || len(retained.Intervention.ChangedEventIDs) != 0 ||
		len(retained.Intervention.ChangedFieldPaths) != 0 || retained.Intervention.Targets[0].Changed ||
		retained.Intervention.Targets[0].ParentEventID != retained.Intervention.Targets[0].IntervenedEventID {
		t.Fatalf("retain control is invalid: %+v", retained.Intervention)
	}
}

func TestEvidenceInterventionSeparatesOutcomeAndQualityStates(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	message := eventOfKind(t, parent, preprocess.EventMessage)
	assignments := interventionAssignments(t, ontology, parent, FactorSuccessFailureProse, message.ID, PathMessageParts)
	replacement := []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "The task failed."}}
	solved := interventionOutcomeRecord(t, "task-fixture", "source-quality", outcome.StateSolved)
	unsolved := interventionOutcomeRecord(t, "task-fixture", "intervened-quality", outcome.StateUnsolved)
	baseRequest := EvidenceInterventionRequest{
		FactorID: FactorSuccessFailureProse, Operator: OperatorControlledReplacement,
		Targets: []InterventionTargetRequest{{
			EventID: message.ID, FieldPath: PathMessageParts,
			Replacement: &InterventionValue{ContentParts: &replacement},
		}},
		SourceOutcome: &solved, IntervenedOutcome: &unsolved,
	}
	assertEvidenceOnlyOutcomeChangeRejected(t, ontology, estimands, assignments, parent, baseRequest)
	assertQualityChangeRequiresRelation(t, ontology, estimands, assignments, parent, baseRequest)
	assertIndeterminateOutcomeUnresolved(t, ontology, estimands, assignments, parent, baseRequest)
}

func assertEvidenceOnlyOutcomeChangeRejected(
	t *testing.T,
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	request EvidenceInterventionRequest,
) {
	t.Helper()
	evidenceOnly := request
	evidenceOnly.EstimandFamily = EstimandEvidenceOnly
	rejected, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, evidenceOnly)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Intervention.Admissibility != InterventionInadmissible || rejected.Intervention.DenominatorEligible ||
		!slices.Equal(rejected.Intervention.AdmissibilityReasons, []InterventionFailureCode{FailureOutcomePreservationReject}) {
		t.Fatalf("changed outcome entered evidence-only analysis: %+v", rejected.Intervention)
	}
}

func assertQualityChangeRequiresRelation(
	t *testing.T,
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	request EvidenceInterventionRequest,
) {
	t.Helper()
	qualityChanging := request
	qualityChanging.EstimandFamily = EstimandQualityChanging
	pending, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, qualityChanging)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Intervention.Admissibility != InterventionUnresolved || pending.Intervention.DenominatorEligible ||
		!slices.Equal(pending.Intervention.AdmissibilityReasons, []InterventionFailureCode{FailureRelationAdmissionRequired}) {
		t.Fatalf("quality-changing case bypassed relation admission: %+v", pending.Intervention)
	}
}

func assertIndeterminateOutcomeUnresolved(
	t *testing.T,
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	request EvidenceInterventionRequest,
) {
	t.Helper()
	indeterminateSource := interventionOutcomeRecord(t, "task-fixture", "source-indeterminate", outcome.StateIndeterminate)
	indeterminateChild := interventionOutcomeRecord(t, "task-fixture", "intervened-indeterminate", outcome.StateIndeterminate)
	unresolvedRequest := request
	unresolvedRequest.EstimandFamily = EstimandEvidenceOnly
	unresolvedRequest.SourceOutcome = &indeterminateSource
	unresolvedRequest.IntervenedOutcome = &indeterminateChild
	unresolved, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, unresolvedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if unresolved.Intervention.Admissibility != InterventionUnresolved ||
		!slices.Equal(unresolved.Intervention.AdmissibilityReasons, []InterventionFailureCode{FailureOutcomeStateUnresolved}) {
		t.Fatalf("indeterminate outcome was not retained as unresolved: %+v", unresolved.Intervention)
	}
}

func TestEvidenceInterventionFailsClosedWithTypedErrors(t *testing.T) {
	t.Run("wrong replacement type", testWrongInterventionReplacementType)
	t.Run("no evidence change", testInterventionNoEvidenceChange)
	t.Run("partial outcome pair", testInterventionPartialOutcomePair)
	t.Run("dependency closure", testInterventionDependencyClosure)
}

func testWrongInterventionReplacementType(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	message := eventOfKind(t, parent, preprocess.EventMessage)
	assignments := interventionAssignments(t, ontology, parent, FactorSuccessFailureProse, message.ID, PathMessageParts)
	text := "wrong type"
	_, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, EvidenceInterventionRequest{
		FactorID: FactorSuccessFailureProse, Operator: OperatorControlledReplacement, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{{EventID: message.ID, FieldPath: PathMessageParts, Replacement: &InterventionValue{Text: &text}}},
	})
	assertInterventionErrorCode(t, err, FailureValueInvalid)
}

func testInterventionNoEvidenceChange(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	message := eventOfKind(t, parent, preprocess.EventMessage)
	assignments := interventionAssignments(t, ontology, parent, FactorSuccessFailureProse, message.ID, PathMessageParts)
	sameParts := slices.Clone(message.Message.Parts)
	_, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, EvidenceInterventionRequest{
		FactorID: FactorSuccessFailureProse, Operator: OperatorControlledReplacement, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{{EventID: message.ID, FieldPath: PathMessageParts, Replacement: &InterventionValue{ContentParts: &sameParts}}},
	})
	assertInterventionErrorCode(t, err, FailureNoEvidenceChange)
}

func testInterventionPartialOutcomePair(t *testing.T) {
	ontology, estimands, parent := interventionContracts(t)
	message := eventOfKind(t, parent, preprocess.EventMessage)
	assignments := interventionAssignments(t, ontology, parent, FactorSuccessFailureProse, message.ID, PathMessageParts)
	sourceOutcome := interventionOutcomeRecord(t, "task-fixture", "source-error", outcome.StateSolved)
	_, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, EvidenceInterventionRequest{
		FactorID: FactorSuccessFailureProse, Operator: OperatorRetain, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{{EventID: message.ID, FieldPath: PathMessageParts}}, SourceOutcome: &sourceOutcome,
	})
	assertInterventionErrorCode(t, err, FailureOutcomeInputInvalid)
}

func testInterventionDependencyClosure(t *testing.T) {
	ontology, estimands, referenceParent := interventionContracts(t)
	referenceMessage := eventOfKind(t, referenceParent, preprocess.EventMessage)
	for index := range referenceParent.Events {
		if referenceParent.Events[index].ID != referenceMessage.ID {
			continue
		}
		referenceParent.Events[index].Message.Parts = []preprocess.ContentPart{{
			Kind: preprocess.ContentEventReference, EventID: referenceParent.Events[0].ID,
		}}
		rebuilt, err := preprocess.RebuildDerivedEvent(referenceParent.SourceFormat, referenceParent.Events[index])
		if err != nil {
			t.Fatal(err)
		}
		referenceParent.Events[index] = rebuilt
		referenceMessage = rebuilt
	}
	referenceAssignments := interventionAssignments(t, ontology, referenceParent, FactorSuccessFailureProse, referenceMessage.ID, PathMessageParts)
	_, err := ApplyEvidenceIntervention(ontology, estimands, referenceAssignments, referenceParent, EvidenceInterventionRequest{
		FactorID: FactorSuccessFailureProse, Operator: OperatorTypedMask, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{{EventID: referenceMessage.ID, FieldPath: PathMessageParts}},
	})
	assertInterventionErrorCode(t, err, FailureDependencyClosureRequired)
}

func TestEvidenceInterventionValidationRejectsUntargetedOrArtifactTampering(t *testing.T) {
	ontology, estimands, assignments, parent, result := interventionTamperFixture(t)
	assertUntargetedInterventionTamperingRejected(t, ontology, estimands, assignments, parent, result)
	assertInterventionArtifactTamperingRejected(t, ontology, estimands, assignments, parent, result)
}

func interventionTamperFixture(
	t *testing.T,
) (FactorOntology, EstimandCatalog, FactorAssignmentSet, preprocess.Trajectory, EvidenceInterventionResult) {
	t.Helper()
	ontology, estimands, parent := interventionContracts(t)
	metadata := eventOfKind(t, parent, preprocess.EventMetadata)
	assignments := interventionAssignments(t, ontology, parent, FactorMetadata, metadata.ID, PathMetadataValue)
	sourceOutcome := interventionOutcomeRecord(t, "task-fixture", "source-tamper", outcome.StateSolved)
	intervenedOutcome := interventionOutcomeRecord(t, "task-fixture", "intervened-tamper", outcome.StateSolved)
	replacement := "changed"
	result, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, EvidenceInterventionRequest{
		FactorID: FactorMetadata, Operator: OperatorControlledReplacement, EstimandFamily: EstimandEvidenceOnly,
		Targets: []InterventionTargetRequest{{
			EventID: metadata.ID, FieldPath: PathMetadataValue, Replacement: &InterventionValue{Text: &replacement},
		}},
		SourceOutcome: &sourceOutcome, IntervenedOutcome: &intervenedOutcome,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ontology, estimands, assignments, parent, result
}

func assertUntargetedInterventionTamperingRejected(
	t *testing.T,
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	result EvidenceInterventionResult,
) {
	t.Helper()
	tamperedTrajectory := result
	messageIndex := -1
	for index := range tamperedTrajectory.Trajectory.Events {
		if tamperedTrajectory.Trajectory.Events[index].Kind == preprocess.EventMessage {
			messageIndex = index
		}
	}
	if messageIndex < 0 {
		t.Fatal("fixture lacks message event")
	}
	tamperedTrajectory.Trajectory.Events[messageIndex].Message.Phase = "tampered"
	if err := tamperedTrajectory.Validate(ontology, estimands, assignments, parent); err == nil {
		t.Fatal("untargeted trajectory change was accepted")
	}
}

func assertInterventionArtifactTamperingRejected(
	t *testing.T,
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	result EvidenceInterventionResult,
) {
	t.Helper()
	tamperedArtifact := result
	tamperedArtifact.Intervention.Targets[0].AfterStateDigest = strings.Repeat("f", 64)
	intervention, err := sealEvidenceIntervention(tamperedArtifact.Intervention)
	if err != nil {
		t.Fatal(err)
	}
	tamperedArtifact.Intervention = intervention
	if err := tamperedArtifact.Validate(ontology, estimands, assignments, parent); err == nil {
		t.Fatal("digest-recomputed intervention target tampering was accepted")
	}
}

func interventionContracts(t *testing.T) (FactorOntology, EstimandCatalog, preprocess.Trajectory) {
	t.Helper()
	ontology, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	estimands, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	return ontology, estimands, relianceTrajectory(t)
}

func interventionAssignments(
	t *testing.T,
	ontology FactorOntology,
	trajectory preprocess.Trajectory,
	factorID FactorID,
	eventID string,
	fieldPath preprocess.FieldPath,
) FactorAssignmentSet {
	t.Helper()
	assignments, err := SealFactorAssignments(ontology, trajectory, []AssignmentRequest{{
		EventID: eventID, FieldPath: fieldPath, FactorID: factorID,
		Method: AssignmentStructuralRule, Rationale: "deterministic intervention fixture",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return assignments
}

func interventionOutcomeRecord(t *testing.T, taskAlias, evidenceID string, state outcome.State) outcome.Record {
	t.Helper()
	evidence, err := outcome.SealEvidence(outcome.Evidence{
		ID: evidenceID, Kind: outcome.EvidenceBenchmarkReward, State: state,
		ArtifactDigest: protocolkit.DigestBytes([]byte("artifact-" + evidenceID)),
		ObservedAt:     "2026-08-14T00:00:00Z", Public: true,
		Limitation: "deterministic intervention test fixture", ParentDigests: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err := outcome.SealRecord(outcome.Record{
		TaskAlias: taskAlias, Revision: 1, Evidence: []outcome.Evidence{evidence}, Resolution: state,
		ResolutionBasis: []string{evidence.ID}, Limitations: []string{"test_fixture_only"},
		AuthorID: "fixture", RevisionReason: "intervention test fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func assertInterventionErrorCode(t *testing.T, err error, code InterventionFailureCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected intervention error %q", code)
	}
	var interventionErr *InterventionError
	if !errors.As(err, &interventionErr) || interventionErr.Code != code {
		t.Fatalf("error = %v, want typed code %q", err, code)
	}
}
