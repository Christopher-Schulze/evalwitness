package reliance

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func SealFactorAssignments(
	ontology FactorOntology,
	trajectory preprocess.Trajectory,
	requests []AssignmentRequest,
) (FactorAssignmentSet, error) {
	if err := ontology.Validate(); err != nil {
		return FactorAssignmentSet{}, fmt.Errorf("validate reliance ontology: %w", err)
	}
	if err := trajectory.Validate(); err != nil {
		return FactorAssignmentSet{}, fmt.Errorf("validate reliance trajectory: %w", err)
	}
	if len(requests) == 0 {
		return FactorAssignmentSet{}, errors.New("reliance factor assignment set must not be empty")
	}
	events := indexEvents(trajectory.Events)
	assignments := make([]EvidenceAssignment, 0, len(requests))
	seenTargets := make(map[string]FactorID, len(requests))
	for index, request := range requests {
		assignment, err := buildAssignment(ontology, events, request)
		if err != nil {
			return FactorAssignmentSet{}, fmt.Errorf("factor assignment %d: %w", index, err)
		}
		key := assignmentTargetKey(assignment.EventID, assignment.FieldPath)
		if previous, duplicate := seenTargets[key]; duplicate {
			if previous != assignment.FactorID {
				return FactorAssignmentSet{}, fmt.Errorf("evidence target %s%s has multiple factor assignments", assignment.EventID, assignment.FieldPath)
			}
			return FactorAssignmentSet{}, fmt.Errorf("evidence target %s%s repeats one factor assignment", assignment.EventID, assignment.FieldPath)
		}
		seenTargets[key] = assignment.FactorID
		assignments = append(assignments, assignment)
	}
	slices.SortFunc(assignments, compareAssignments)
	value := FactorAssignmentSet{
		SchemaVersion: FactorAssignmentSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		OntologyDigest: ontology.Digest, TrajectoryDigest: trajectory.Digest,
		FreezeStage: AssignmentFreezeStagePreExecute, Assignments: assignments,
	}
	digest, err := factorAssignmentSetDigest(value)
	if err != nil {
		return FactorAssignmentSet{}, err
	}
	value.Digest = digest
	if err := value.Validate(ontology, trajectory); err != nil {
		return FactorAssignmentSet{}, err
	}
	return value, nil
}

func (value FactorAssignmentSet) Validate(ontology FactorOntology, trajectory preprocess.Trajectory) error {
	if err := ontology.Validate(); err != nil {
		return fmt.Errorf("validate reliance ontology: %w", err)
	}
	if err := trajectory.Validate(); err != nil {
		return fmt.Errorf("validate reliance trajectory: %w", err)
	}
	if value.SchemaVersion != FactorAssignmentSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.OntologyDigest != ontology.Digest || value.TrajectoryDigest != trajectory.Digest ||
		value.FreezeStage != AssignmentFreezeStagePreExecute || len(value.Assignments) == 0 {
		return errors.New("reliance factor assignment identity or freeze boundary is invalid")
	}
	events := indexEvents(trajectory.Events)
	seenTargets := make(map[string]FactorID, len(value.Assignments))
	for index, assignment := range value.Assignments {
		if index > 0 && compareAssignments(value.Assignments[index-1], assignment) >= 0 {
			return errors.New("reliance factor assignments are unordered or duplicated")
		}
		request := AssignmentRequest{
			EventID: assignment.EventID, FieldPath: assignment.FieldPath, FactorID: assignment.FactorID,
			Method: assignment.Method, Rationale: assignment.Rationale,
		}
		expected, err := buildAssignment(ontology, events, request)
		if err != nil {
			return fmt.Errorf("validate factor assignment %d: %w", index, err)
		}
		if assignment != expected {
			return fmt.Errorf("factor assignment %d differs from its frozen trajectory target", index)
		}
		key := assignmentTargetKey(assignment.EventID, assignment.FieldPath)
		if previous, duplicate := seenTargets[key]; duplicate {
			if previous != assignment.FactorID {
				return errors.New("reliance evidence target has multiple factor assignments")
			}
			return errors.New("reliance evidence target repeats one factor assignment")
		}
		seenTargets[key] = assignment.FactorID
	}
	digest, err := factorAssignmentSetDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance factor assignment digest is invalid")
	}
	return nil
}

func ValidateOperatorTargets(
	ontology FactorOntology,
	assignments FactorAssignmentSet,
	trajectory preprocess.Trajectory,
	operator InterventionOperator,
	factorID FactorID,
	targets []AssignmentTarget,
) error {
	if err := assignments.Validate(ontology, trajectory); err != nil {
		return err
	}
	return validateOperatorTargets(ontology, assignments, trajectory, operator, factorID, targets)
}

func validateOperatorTargets(
	ontology FactorOntology,
	assignments FactorAssignmentSet,
	trajectory preprocess.Trajectory,
	operator InterventionOperator,
	factorID FactorID,
	targets []AssignmentTarget,
) error {
	definition, found := ontology.Operator(operator)
	if !found {
		return fmt.Errorf("unknown reliance intervention operator %q", operator)
	}
	if _, found := ontology.Factor(factorID); !found {
		return fmt.Errorf("unknown reliance factor %q", factorID)
	}
	if len(targets) == 0 {
		return errors.New("reliance intervention has no assigned targets")
	}
	assigned := make(map[string]EvidenceAssignment, len(assignments.Assignments))
	for _, assignment := range assignments.Assignments {
		assigned[assignmentTargetKey(assignment.EventID, assignment.FieldPath)] = assignment
	}
	seen := make(map[string]struct{}, len(targets))
	for _, requested := range targets {
		key := assignmentTargetKey(requested.EventID, requested.FieldPath)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("reliance intervention repeats target %s%s", requested.EventID, requested.FieldPath)
		}
		seen[key] = struct{}{}
		assignment, found := assigned[key]
		if !found {
			return fmt.Errorf("reliance intervention target %s%s is unassigned", requested.EventID, requested.FieldPath)
		}
		if assignment.FactorID != factorID {
			return fmt.Errorf("reliance intervention target %s%s is assigned to factor %q, not %q", requested.EventID, requested.FieldPath, assignment.FactorID, factorID)
		}
		target, _, err := eventFieldMaterial(indexEvents(trajectory.Events)[requested.EventID], requested.FieldPath)
		if err != nil {
			return err
		}
		if !definition.Allows(target) {
			return fmt.Errorf("reliance operator %q does not allow %s%s", operator, target.EventKind, target.FieldPath)
		}
	}
	return nil
}

func buildAssignment(
	ontology FactorOntology,
	events map[string]preprocess.Event,
	request AssignmentRequest,
) (EvidenceAssignment, error) {
	event, found := events[request.EventID]
	if !found {
		return EvidenceAssignment{}, fmt.Errorf("unknown canonical event %q", request.EventID)
	}
	if !request.Method.Valid() || strings.TrimSpace(request.Rationale) == "" || request.Rationale != strings.TrimSpace(request.Rationale) {
		return EvidenceAssignment{}, errors.New("factor assignment method or rationale is invalid")
	}
	target, valueDigest, err := eventFieldMaterial(event, request.FieldPath)
	if err != nil {
		return EvidenceAssignment{}, err
	}
	if err := validateFactorTarget(ontology, request.FactorID, target); err != nil {
		return EvidenceAssignment{}, err
	}
	assignment := EvidenceAssignment{
		EventID: event.ID, EventKind: event.Kind, FieldPath: request.FieldPath, FactorID: request.FactorID,
		ValueDigest: valueDigest, Method: request.Method, Rationale: request.Rationale,
	}
	assignmentID, err := evidenceAssignmentID(assignment)
	if err != nil {
		return EvidenceAssignment{}, err
	}
	assignment.AssignmentID = assignmentID
	return assignment, nil
}

func eventFieldMaterial(event preprocess.Event, path preprocess.FieldPath) (FieldTarget, string, error) {
	target, found := canonicalTarget(event.Kind, path)
	if !found {
		return FieldTarget{}, "", fmt.Errorf("unknown reliance field %s%s", event.Kind, path)
	}
	var value any
	present := true
	switch path {
	case PathCommandExitCode:
		if event.Command == nil {
			return FieldTarget{}, "", errors.New("command target lacks command payload")
		}
		value, present = event.Command.ExitCode, event.Command.ExitCode != nil
	case PathErrorClass:
		if event.Error == nil {
			return FieldTarget{}, "", errors.New("error target lacks error payload")
		}
		value = event.Error.Class
	case PathErrorSafeMessage:
		if event.Error == nil {
			return FieldTarget{}, "", errors.New("error target lacks error payload")
		}
		value, present = event.Error.SafeMessage, event.Error.SafeMessage != ""
	case PathEvaluationErrorType, PathEvaluationExplanation, PathEvaluationScoreLabel, PathEvaluationScoreValue:
		if event.Evaluation == nil {
			return FieldTarget{}, "", errors.New("evaluation target lacks evaluation payload")
		}
		switch path {
		case PathEvaluationErrorType:
			value, present = event.Evaluation.ErrorType, event.Evaluation.ErrorType != ""
		case PathEvaluationExplanation:
			value, present = event.Evaluation.Explanation, event.Evaluation.Explanation != ""
		case PathEvaluationScoreLabel:
			value, present = event.Evaluation.ScoreLabel, event.Evaluation.ScoreLabel != ""
		case PathEvaluationScoreValue:
			value, present = event.Evaluation.ScoreValue, event.Evaluation.ScoreValue != ""
		}
	case PathFileChangeContentDigest, PathFileChangeDiff, PathFileChangeDiffDigest, PathFileChangeOperation,
		PathFileChangePathAlias, PathFileChangeSizeBytes:
		if event.FileChange == nil {
			return FieldTarget{}, "", errors.New("file-change target lacks file-change payload")
		}
		switch path {
		case PathFileChangeContentDigest:
			value, present = event.FileChange.ContentDigest, event.FileChange.ContentDigest != ""
		case PathFileChangeDiff:
			value, present = event.FileChange.Diff, event.FileChange.Diff != ""
		case PathFileChangeDiffDigest:
			value, present = event.FileChange.DiffDigest, event.FileChange.DiffDigest != ""
		case PathFileChangeOperation:
			value = event.FileChange.Operation
		case PathFileChangePathAlias:
			value, present = event.FileChange.PathAlias, event.FileChange.PathAlias != ""
		case PathFileChangeSizeBytes:
			value = event.FileChange.SizeBytes
		}
	case PathMessageParts, PathMessagePhase:
		if event.Message == nil {
			return FieldTarget{}, "", errors.New("message target lacks message payload")
		}
		if path == PathMessageParts {
			value = event.Message.Parts
		} else {
			value, present = event.Message.Phase, event.Message.Phase != ""
		}
	case PathMetadataName, PathMetadataPresent, PathMetadataValue, PathMetadataValueDigest:
		if event.Metadata == nil {
			return FieldTarget{}, "", errors.New("metadata target lacks metadata payload")
		}
		switch path {
		case PathMetadataName:
			value = event.Metadata.Name
		case PathMetadataPresent:
			value = event.Metadata.Present
		case PathMetadataValue:
			value, present = event.Metadata.Value, event.Metadata.Value != ""
		case PathMetadataValueDigest:
			value, present = event.Metadata.ValueDigest, event.Metadata.ValueDigest != ""
		}
	case PathOutputStatus, PathOutputStream, PathOutputText:
		if event.Output == nil {
			return FieldTarget{}, "", errors.New("output target lacks output payload")
		}
		switch path {
		case PathOutputStatus:
			value, present = event.Output.Status, event.Output.Status != ""
		case PathOutputStream:
			value, present = event.Output.Stream, event.Output.Stream != ""
		case PathOutputText:
			value, present = event.Output.Text, event.Output.Text != ""
		}
	case PathToolCallArguments:
		if event.ToolCall == nil {
			return FieldTarget{}, "", errors.New("tool-call target lacks tool-call payload")
		}
		value, present = event.ToolCall.Arguments, event.ToolCall.Arguments != ""
	case PathToolResultError, PathToolResultExitCode, PathToolResultOutput, PathToolResultStderr,
		PathToolResultStdout, PathToolResultStatus:
		if event.ToolResult == nil {
			return FieldTarget{}, "", errors.New("tool-result target lacks tool-result payload")
		}
		switch path {
		case PathToolResultError:
			value = event.ToolResult.Error
		case PathToolResultExitCode:
			value, present = event.ToolResult.ExitCode, event.ToolResult.ExitCode != nil
		case PathToolResultOutput:
			value, present = event.ToolResult.Output, len(event.ToolResult.Output) > 0
		case PathToolResultStderr:
			value, present = event.ToolResult.Stderr, len(event.ToolResult.Stderr) > 0
		case PathToolResultStdout:
			value, present = event.ToolResult.Stdout, len(event.ToolResult.Stdout) > 0
		case PathToolResultStatus:
			value = event.ToolResult.Status
		}
	default:
		return FieldTarget{}, "", fmt.Errorf("unsupported reliance field %q", path)
	}
	if target.Optional && !present {
		return FieldTarget{}, "", fmt.Errorf("reliance field %s%s is absent", event.Kind, path)
	}
	canonical, err := protocolkit.CanonicalMarshal(value)
	if err != nil {
		return FieldTarget{}, "", fmt.Errorf("canonicalize reliance field %s%s: %w", event.Kind, path, err)
	}
	return target, protocolkit.DigestBytes(canonical), nil
}

func canonicalTarget(kind preprocess.EventKind, path preprocess.FieldPath) (FieldTarget, bool) {
	for _, target := range allCanonicalTargets() {
		if target.EventKind == kind && target.FieldPath == path {
			return target, true
		}
	}
	return FieldTarget{}, false
}

func indexEvents(events []preprocess.Event) map[string]preprocess.Event {
	result := make(map[string]preprocess.Event, len(events))
	for _, event := range events {
		result[event.ID] = event
	}
	return result
}

func (method AssignmentMethod) Valid() bool {
	switch method {
	case AssignmentBlindedHuman, AssignmentDeclaredAdversarial, AssignmentFrozenFixture, AssignmentStructuralRule:
		return true
	default:
		return false
	}
}

func evidenceAssignmentID(value EvidenceAssignment) (string, error) {
	value.AssignmentID = ""
	return protocolkit.Digest(value)
}

func factorAssignmentSetDigest(value FactorAssignmentSet) (string, error) {
	value.Digest = ""
	return protocolkit.Digest(value)
}

func assignmentTargetKey(eventID string, path preprocess.FieldPath) string {
	return eventID + "\x00" + string(path)
}

func compareAssignments(left, right EvidenceAssignment) int {
	leftKey := assignmentTargetKey(left.EventID, left.FieldPath) + "\x00" + string(left.FactorID)
	rightKey := assignmentTargetKey(right.EventID, right.FieldPath) + "\x00" + string(right.FactorID)
	return strings.Compare(leftKey, rightKey)
}
