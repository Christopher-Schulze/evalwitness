package reliance

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestSealFactorAssignmentsBindsExactPreExecutionEvidence(t *testing.T) {
	ontology := relianceOntology(t)
	trajectory := relianceTrajectory(t)
	toolResult := eventOfKind(t, trajectory, preprocess.EventToolResult)
	message := eventOfKind(t, trajectory, preprocess.EventMessage)
	metadata := eventOfKind(t, trajectory, preprocess.EventMetadata)
	requests := []AssignmentRequest{
		{EventID: metadata.ID, FieldPath: PathMetadataValue, FactorID: FactorMetadata, Method: AssignmentStructuralRule, Rationale: "typed metadata value"},
		{EventID: message.ID, FieldPath: PathMessageParts, FactorID: FactorSuccessFailureProse, Method: AssignmentBlindedHuman, Rationale: "blinded narrative label"},
		{EventID: toolResult.ID, FieldPath: PathToolResultStatus, FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "typed tool status"},
		{EventID: toolResult.ID, FieldPath: PathToolResultStdout, FactorID: FactorTestResult, Method: AssignmentStructuralRule, Rationale: "registered test output"},
	}
	first, err := SealFactorAssignments(ontology, trajectory, requests)
	if err != nil {
		t.Fatal(err)
	}
	reversed := slices.Clone(requests)
	slices.Reverse(reversed)
	second, err := SealFactorAssignments(ontology, trajectory, reversed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("assignment seal depends on request order:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if first.FreezeStage != AssignmentFreezeStagePreExecute || first.OntologyDigest != ontology.Digest ||
		first.TrajectoryDigest != trajectory.Digest || len(first.Assignments) != len(requests) {
		t.Fatalf("assignment seal lacks its freeze boundary: %+v", first)
	}
	for _, assignment := range first.Assignments {
		if assignment.AssignmentID == "" || assignment.ValueDigest == "" {
			t.Fatalf("assignment lacks exact target identity: %+v", assignment)
		}
	}
	tampered := first
	tampered.FreezeStage = "after_verifier_output"
	if err := tampered.Validate(ontology, trajectory); err == nil {
		t.Fatal("assignment set accepted post-output classification")
	}
	tamperedTrajectory := trajectory
	tamperedTrajectory.Events = slices.Clone(trajectory.Events)
	tamperedEvent := tamperedTrajectory.Events[0]
	tamperedPayload := *tamperedEvent.ToolResult
	tamperedPayload.Stdout = slices.Clone(tamperedEvent.ToolResult.Stdout)
	tamperedPayload.Stdout[0].Text = "FAIL package/example"
	tamperedEvent.ToolResult = &tamperedPayload
	tamperedTrajectory.Events[0] = tamperedEvent
	if err := first.Validate(ontology, tamperedTrajectory); err == nil {
		t.Fatal("assignment set accepted changed target bytes under a stale trajectory identity")
	}
}

func TestSealFactorAssignmentsFailsClosedOnAmbiguityAndUnknownEvidence(t *testing.T) {
	ontology := relianceOntology(t)
	trajectory := relianceTrajectory(t)
	toolResult := eventOfKind(t, trajectory, preprocess.EventToolResult)
	tests := []struct {
		name     string
		requests []AssignmentRequest
		contains string
	}{
		{
			name: "multiple factors", contains: "multiple factor assignments",
			requests: []AssignmentRequest{
				{EventID: toolResult.ID, FieldPath: PathToolResultStdout, FactorID: FactorTestResult, Method: AssignmentStructuralRule, Rationale: "test evidence"},
				{EventID: toolResult.ID, FieldPath: PathToolResultStdout, FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "tool evidence"},
			},
		},
		{
			name: "factor path mismatch", contains: "does not allow",
			requests: []AssignmentRequest{{EventID: toolResult.ID, FieldPath: PathToolResultStdout, FactorID: FactorMetadata, Method: AssignmentStructuralRule, Rationale: "wrong factor"}},
		},
		{
			name: "unknown factor", contains: "unknown reliance factor",
			requests: []AssignmentRequest{{EventID: toolResult.ID, FieldPath: PathToolResultStdout, FactorID: "future_factor", Method: AssignmentStructuralRule, Rationale: "unknown factor"}},
		},
		{
			name: "unknown field", contains: "unknown reliance field",
			requests: []AssignmentRequest{{EventID: toolResult.ID, FieldPath: "/tool_result/future", FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "unknown field"}},
		},
		{
			name: "absent optional field", contains: "is absent",
			requests: []AssignmentRequest{{EventID: toolResult.ID, FieldPath: PathToolResultOutput, FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "absent field"}},
		},
		{
			name: "unknown event", contains: "unknown canonical event",
			requests: []AssignmentRequest{{EventID: "missing-event", FieldPath: PathToolResultStdout, FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "unknown event"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := SealFactorAssignments(ontology, trajectory, test.requests)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("assignment error = %v, want substring %q", err, test.contains)
			}
		})
	}
}

func TestValidateOperatorTargetsEnforcesAssignmentsAndTypedBoundaries(t *testing.T) {
	ontology := relianceOntology(t)
	trajectory := relianceTrajectory(t)
	toolResult := eventOfKind(t, trajectory, preprocess.EventToolResult)
	assignments, err := SealFactorAssignments(ontology, trajectory, []AssignmentRequest{
		{EventID: toolResult.ID, FieldPath: PathToolResultError, FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "typed error bit"},
		{EventID: toolResult.ID, FieldPath: PathToolResultStatus, FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "typed status"},
		{EventID: toolResult.ID, FieldPath: PathToolResultStdout, FactorID: FactorToolOutput, Method: AssignmentStructuralRule, Rationale: "typed output"},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		operator InterventionOperator
		factor   FactorID
		target   AssignmentTarget
		wantErr  string
	}{
		{name: "remove optional", operator: OperatorRemove, factor: FactorToolOutput, target: AssignmentTarget{toolResult.ID, PathToolResultStdout}},
		{name: "mask typed content", operator: OperatorTypedMask, factor: FactorToolOutput, target: AssignmentTarget{toolResult.ID, PathToolResultStdout}},
		{name: "replace boolean", operator: OperatorControlledReplacement, factor: FactorToolOutput, target: AssignmentTarget{toolResult.ID, PathToolResultError}},
		{name: "remove required", operator: OperatorRemove, factor: FactorToolOutput, target: AssignmentTarget{toolResult.ID, PathToolResultStatus}, wantErr: "does not allow"},
		{name: "mask boolean", operator: OperatorTypedMask, factor: FactorToolOutput, target: AssignmentTarget{toolResult.ID, PathToolResultError}, wantErr: "does not allow"},
		{name: "wrong factor", operator: OperatorRetain, factor: FactorTestResult, target: AssignmentTarget{toolResult.ID, PathToolResultStdout}, wantErr: "not \"test_result\""},
		{name: "unassigned", operator: OperatorRetain, factor: FactorToolOutput, target: AssignmentTarget{toolResult.ID, PathToolResultExitCode}, wantErr: "is unassigned"},
		{name: "unknown operator", operator: "future_operator", factor: FactorToolOutput, target: AssignmentTarget{toolResult.ID, PathToolResultStdout}, wantErr: "unknown reliance intervention operator"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateOperatorTargets(ontology, assignments, trajectory, test.operator, test.factor, []AssignmentTarget{test.target})
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("operator error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func relianceOntology(t *testing.T) FactorOntology {
	t.Helper()
	ontology, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	return ontology
}

func relianceTrajectory(t *testing.T) preprocess.Trajectory {
	t.Helper()
	exitCode := 0
	events := []preprocess.Event{
		{
			Kind: preprocess.EventToolResult, Order: 0, Source: preprocess.SourceLocation{Record: 0, Line: 1},
			Sensitivity: preprocess.SensitivityPublic,
			ToolResult: &preprocess.ToolResultPayload{
				Status: "completed", Error: false, ExitCode: &exitCode,
				Stdout: []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "PASS package/example"}},
			},
		},
		{
			Kind: preprocess.EventMessage, Order: 1, Source: preprocess.SourceLocation{Record: 0, Line: 2},
			Sensitivity: preprocess.SensitivityPublic,
			Message:     &preprocess.MessagePayload{Role: "assistant", Parts: []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "All checks succeeded."}}},
		},
		{
			Kind: preprocess.EventMetadata, Order: 2, Source: preprocess.SourceLocation{Record: 0, Line: 3},
			Sensitivity: preprocess.SensitivityPublic,
			Metadata:    &preprocess.MetadataPayload{Name: "display_label", Value: "fixture", Present: true},
		},
	}
	for index := range events {
		rebuilt, err := preprocess.RebuildDerivedEvent(preprocess.SourcePlainText, events[index])
		if err != nil {
			t.Fatal(err)
		}
		events[index] = rebuilt
	}
	trajectory := preprocess.Trajectory{
		SchemaVersion: preprocess.CanonicalTrajectorySchema, SourceFormat: preprocess.SourcePlainText,
		SourceDigest: protocolkit.DigestBytes([]byte("reliance-source")),
		Digest:       protocolkit.DigestBytes([]byte("reliance-trajectory")),
		Events:       events, Links: []preprocess.Link{},
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

func eventOfKind(t *testing.T, trajectory preprocess.Trajectory, kind preprocess.EventKind) preprocess.Event {
	t.Helper()
	for _, event := range trajectory.Events {
		if event.Kind == kind {
			return event
		}
	}
	t.Fatalf("trajectory lacks event kind %q", kind)
	return preprocess.Event{}
}
