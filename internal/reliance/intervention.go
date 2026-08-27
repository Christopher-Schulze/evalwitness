package reliance

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type interventionFieldStateMaterial struct {
	Present     bool   `json:"present"`
	ValueDigest string `json:"value_digest,omitempty"`
}

type pendingInterventionTarget struct {
	request      InterventionTargetRequest
	target       FieldTarget
	operator     InterventionOperator
	beforeDigest string
}

type preparedIntervention struct {
	events             []preprocess.Event
	pending            []pendingInterventionTarget
	pathsByEvent       map[string][]preprocess.FieldPath
	parentEventIndexes map[string]int
}

type interventionChanges struct {
	events            []preprocess.Event
	targets           []EvidenceInterventionTarget
	changedEventIDs   []string
	changedFieldPaths []preprocess.FieldPath
	eventIDRemap      map[string]string
}

type interventionValidationState struct {
	assignmentTargets  []AssignmentTarget
	changedEventSet    map[string]struct{}
	changedFields      []preprocess.FieldPath
	eventIDRemap       map[string]string
	pathsByParentEvent map[string][]preprocess.FieldPath
	parentIndexes      map[string]int
	childIndexes       map[string]int
}

func ApplyEvidenceIntervention(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	request EvidenceInterventionRequest,
) (EvidenceInterventionResult, error) {
	targets, err := validateInterventionInputs(ontology, estimands, assignments, parent, request)
	if err != nil {
		return EvidenceInterventionResult{}, err
	}
	prepared, err := prepareIntervention(parent, request, targets)
	if err != nil {
		return EvidenceInterventionResult{}, err
	}
	changes, err := rebuildInterventionChanges(parent, prepared)
	if err != nil {
		return EvidenceInterventionResult{}, err
	}
	links := remapInterventionLinks(parent.Links, changes.eventIDRemap)
	child, err := preprocess.DeriveTrajectory(parent, changes.events, links, preprocess.DerivationSpec{
		Relation: EvidenceInterventionRelation, Validator: EvidenceInterventionValidator,
		ChangedEventIDs: changes.changedEventIDs, ChangedFieldPaths: changes.changedFieldPaths,
	})
	if err != nil {
		return EvidenceInterventionResult{}, interventionFailure(FailureDerivationInvalid, err)
	}
	preservation, err := buildInterventionPreservation(request)
	if err != nil {
		return EvidenceInterventionResult{}, err
	}
	admissibility, reasons := assessInterventionAdmissibility(request.EstimandFamily, preservation)
	value := newEvidenceIntervention(ontology, estimands, assignments, parent, child, request, changes, preservation, admissibility, reasons)
	return finalizeEvidenceIntervention(ontology, estimands, assignments, parent, child, value)
}

func finalizeEvidenceIntervention(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent, child preprocess.Trajectory,
	value EvidenceIntervention,
) (EvidenceInterventionResult, error) {
	value, err := sealEvidenceIntervention(value)
	if err != nil {
		return EvidenceInterventionResult{}, err
	}
	result := EvidenceInterventionResult{Intervention: value, Trajectory: child}
	if err := result.Validate(ontology, estimands, assignments, parent); err != nil {
		return EvidenceInterventionResult{}, err
	}
	return result, nil
}

func validateInterventionInputs(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	request EvidenceInterventionRequest,
) ([]InterventionTargetRequest, error) {
	if err := ontology.Validate(); err != nil {
		return nil, interventionFailure(FailureAssignmentInvalid, err)
	}
	if err := estimands.Validate(); err != nil {
		return nil, interventionFailure(FailureEstimandInvalid, err)
	}
	if _, found := estimands.Definition(request.EstimandFamily); !found {
		return nil, interventionFailure(FailureEstimandInvalid, fmt.Errorf("unknown estimand family %q", request.EstimandFamily))
	}
	if err := assignments.Validate(ontology, parent); err != nil {
		return nil, interventionFailure(FailureAssignmentInvalid, err)
	}
	if _, found := ontology.Operator(request.Operator); !found {
		return nil, interventionFailure(FailureOperatorInvalid, fmt.Errorf("unknown operator %q", request.Operator))
	}
	if len(request.Targets) == 0 {
		return nil, interventionFailure(FailureTargetInvalid, errors.New("intervention has no targets"))
	}
	if (request.SourceOutcome == nil) != (request.IntervenedOutcome == nil) {
		return nil, interventionFailure(FailureOutcomeInputInvalid, errors.New("source and intervened outcome records must be supplied together"))
	}
	targets := slices.Clone(request.Targets)
	slices.SortFunc(targets, compareInterventionTargetRequests)
	assignmentTargets := make([]AssignmentTarget, len(targets))
	for index, target := range targets {
		assignmentTargets[index] = AssignmentTarget{EventID: target.EventID, FieldPath: target.FieldPath}
	}
	if err := validateOperatorTargets(ontology, assignments, parent, request.Operator, request.FactorID, assignmentTargets); err != nil {
		return nil, interventionFailure(FailureTargetInvalid, err)
	}
	return targets, nil
}

func prepareIntervention(
	parent preprocess.Trajectory,
	request EvidenceInterventionRequest,
	targets []InterventionTargetRequest,
) (preparedIntervention, error) {
	prepared, err := newPreparedIntervention(parent)
	if err != nil {
		return preparedIntervention{}, err
	}
	for _, target := range targets {
		if err := prepareInterventionTarget(&prepared, request.Operator, target); err != nil {
			return preparedIntervention{}, err
		}
	}
	return prepared, nil
}

func newPreparedIntervention(parent preprocess.Trajectory) (preparedIntervention, error) {
	events, err := cloneInterventionEvents(parent.Events)
	if err != nil {
		return preparedIntervention{}, interventionFailure(FailureDerivationInvalid, err)
	}
	prepared := preparedIntervention{
		events: events, pending: make([]pendingInterventionTarget, 0, len(parent.Events)),
		pathsByEvent: make(map[string][]preprocess.FieldPath), parentEventIndexes: make(map[string]int, len(parent.Events)),
	}
	for index, event := range parent.Events {
		prepared.parentEventIndexes[event.ID] = index
	}
	return prepared, nil
}

func prepareInterventionTarget(
	prepared *preparedIntervention,
	operator InterventionOperator,
	request InterventionTargetRequest,
) error {
	index, found := prepared.parentEventIndexes[request.EventID]
	if !found {
		return interventionFailure(FailureTargetInvalid, fmt.Errorf("unknown event %q", request.EventID))
	}
	target, beforeDigest, present, err := interventionFieldState(prepared.events[index], request.FieldPath)
	if err != nil || !present {
		if err == nil {
			err = errors.New("assigned intervention field is absent")
		}
		return interventionFailure(FailureTargetInvalid, err)
	}
	if operator != OperatorRetain && interventionTargetHasEventReference(prepared.events[index], request.FieldPath) {
		return interventionFailure(FailureDependencyClosureRequired, fmt.Errorf("target %s%s contains an event reference", request.EventID, request.FieldPath))
	}
	if err := applyInterventionOperator(&prepared.events[index], target, operator, request.Replacement); err != nil {
		return err
	}
	prepared.pending = append(prepared.pending, pendingInterventionTarget{
		request: request, target: target, operator: operator, beforeDigest: beforeDigest,
	})
	prepared.pathsByEvent[request.EventID] = append(prepared.pathsByEvent[request.EventID], request.FieldPath)
	return nil
}

func rebuildInterventionChanges(
	parent preprocess.Trajectory,
	prepared preparedIntervention,
) (interventionChanges, error) {
	changes := interventionChanges{
		events: prepared.events, changedEventIDs: make([]string, 0, len(prepared.pathsByEvent)),
		changedFieldPaths: make([]preprocess.FieldPath, 0, len(prepared.pending)), eventIDRemap: make(map[string]string),
	}
	for eventID, paths := range prepared.pathsByEvent {
		if err := rebuildInterventionEvent(parent, prepared, &changes, eventID, paths); err != nil {
			return interventionChanges{}, err
		}
	}
	targets, err := buildInterventionTargets(parent, prepared, changes.events)
	if err != nil {
		return interventionChanges{}, err
	}
	changes.targets = targets
	sort.Strings(changes.changedEventIDs)
	slices.SortFunc(changes.changedFieldPaths, compareInterventionFieldPaths)
	return changes, nil
}

func rebuildInterventionEvent(
	parent preprocess.Trajectory,
	prepared preparedIntervention,
	changes *interventionChanges,
	parentEventID string,
	paths []preprocess.FieldPath,
) error {
	index := prepared.parentEventIndexes[parentEventID]
	changed, err := validatePendingEventChanges(changes.events[index], prepared.pending, parentEventID)
	if err != nil || !changed {
		return err
	}
	rebuilt, err := preprocess.RebuildDerivedEvent(parent.SourceFormat, changes.events[index])
	if err != nil {
		return interventionFailure(FailureDerivationInvalid, err)
	}
	changes.events[index] = rebuilt
	changes.eventIDRemap[parentEventID] = rebuilt.ID
	changes.changedEventIDs = append(changes.changedEventIDs, parentEventID)
	for _, path := range paths {
		changed, stateErr := pendingTargetChanged(prepared.pending, parentEventID, path, rebuilt)
		if stateErr != nil {
			return stateErr
		}
		if changed {
			changes.changedFieldPaths = append(changes.changedFieldPaths, derivedInterventionFieldPath(parentEventID, path))
		}
	}
	return nil
}

func validatePendingEventChanges(
	event preprocess.Event,
	pending []pendingInterventionTarget,
	parentEventID string,
) (bool, error) {
	changed := false
	for _, item := range pending {
		if item.request.EventID != parentEventID {
			continue
		}
		_, afterDigest, present, err := interventionFieldState(event, item.request.FieldPath)
		if err != nil {
			return false, interventionFailure(FailureValueInvalid, err)
		}
		if item.operator == OperatorRemove && present || item.operator != OperatorRemove && !present {
			return false, interventionFailure(FailureValueInvalid, fmt.Errorf("operator %q produced invalid presence for %s%s", item.operator, parentEventID, item.request.FieldPath))
		}
		itemChanged := item.beforeDigest != afterDigest
		if item.operator != OperatorRetain && !itemChanged {
			return false, interventionFailure(FailureNoEvidenceChange, fmt.Errorf("operator %q did not change %s%s", item.operator, parentEventID, item.request.FieldPath))
		}
		if item.operator == OperatorRetain && itemChanged {
			return false, interventionFailure(FailureValueInvalid, fmt.Errorf("retain changed %s%s", parentEventID, item.request.FieldPath))
		}
		changed = changed || itemChanged
	}
	return changed, nil
}

func pendingTargetChanged(
	pending []pendingInterventionTarget,
	eventID string,
	path preprocess.FieldPath,
	event preprocess.Event,
) (bool, error) {
	for _, item := range pending {
		if item.request.EventID == eventID && item.request.FieldPath == path {
			_, afterDigest, _, err := interventionFieldState(event, path)
			if err != nil {
				return false, interventionFailure(FailureValueInvalid, err)
			}
			return item.beforeDigest != afterDigest, nil
		}
	}
	return false, interventionFailure(FailureTargetInvalid, fmt.Errorf("missing pending target %s%s", eventID, path))
}

func buildInterventionTargets(
	parent preprocess.Trajectory,
	prepared preparedIntervention,
	events []preprocess.Event,
) ([]EvidenceInterventionTarget, error) {
	targets := make([]EvidenceInterventionTarget, 0, len(prepared.pending))
	for _, item := range prepared.pending {
		index := prepared.parentEventIndexes[item.request.EventID]
		_, afterDigest, _, err := interventionFieldState(events[index], item.request.FieldPath)
		if err != nil {
			return nil, interventionFailure(FailureValueInvalid, err)
		}
		targets = append(targets, EvidenceInterventionTarget{
			ParentEventID: item.request.EventID, IntervenedEventID: events[index].ID,
			EventKind: parent.Events[index].Kind, FieldPath: item.request.FieldPath, ValueKind: item.target.ValueKind,
			BeforeStateDigest: item.beforeDigest, AfterStateDigest: afterDigest, Changed: item.beforeDigest != afterDigest,
		})
	}
	return targets, nil
}

func buildInterventionPreservation(request EvidenceInterventionRequest) (*outcome.Preservation, error) {
	if request.SourceOutcome == nil {
		return nil, nil
	}
	value, err := outcome.EvaluatePreservation(*request.SourceOutcome, *request.IntervenedOutcome, InterventionOutcomeMechanism)
	if err != nil {
		return nil, interventionFailure(FailureOutcomeInputInvalid, err)
	}
	return &value, nil
}

func newEvidenceIntervention(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent, child preprocess.Trajectory,
	request EvidenceInterventionRequest,
	changes interventionChanges,
	preservation *outcome.Preservation,
	admissibility InterventionAdmissibility,
	reasons []InterventionFailureCode,
) EvidenceIntervention {
	return EvidenceIntervention{
		SchemaVersion: EvidenceInterventionSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		OntologyDigest: ontology.Digest, EstimandCatalogDigest: estimands.Digest, AssignmentSetDigest: assignments.Digest,
		FactorID: request.FactorID, Operator: request.Operator, EstimandFamily: request.EstimandFamily,
		SourceTrajectoryDigest: parent.Digest, IntervenedTrajectoryDigest: child.Digest,
		Targets: changes.targets, ChangedEventIDs: changes.changedEventIDs, ChangedFieldPaths: changes.changedFieldPaths,
		OutcomePreservation: preservation, Admissibility: admissibility, AdmissibilityReasons: reasons,
		DenominatorEligible: admissibility == InterventionAdmissible, TaskIdentityPreserved: parent.SourceDigest == child.SourceDigest,
		AssignmentFrozenBeforeOutput: assignments.FreezeStage == AssignmentFreezeStagePreExecute,
		ProviderCalls:                0, NetworkRequired: false,
	}
}

func (result EvidenceInterventionResult) Validate(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
) error {
	value := result.Intervention
	child := result.Trajectory
	if err := validateInterventionParents(ontology, estimands, assignments, parent, child); err != nil {
		return err
	}
	if err := validateInterventionHeader(value, ontology, estimands, assignments, parent, child); err != nil {
		return err
	}
	state, err := inspectInterventionTargets(value, parent, child)
	if err != nil {
		return err
	}
	if err := validateInterventionTargetContract(value, ontology, assignments, parent, state); err != nil {
		return err
	}
	if err := validateInterventionEventIsolation(parent, child, state.pathsByParentEvent); err != nil {
		return err
	}
	if err := validateInterventionDerivation(value, parent, child, state); err != nil {
		return err
	}
	if err := validateInterventionAdmissibility(value); err != nil {
		return err
	}
	return validateInterventionDigest(value)
}

func validateInterventionParents(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent, child preprocess.Trajectory,
) error {
	if err := ontology.Validate(); err != nil {
		return err
	}
	if err := estimands.Validate(); err != nil {
		return err
	}
	if err := assignments.Validate(ontology, parent); err != nil {
		return err
	}
	return child.Validate()
}

func validateInterventionHeader(
	value EvidenceIntervention,
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent, child preprocess.Trajectory,
) error {
	if value.SchemaVersion != EvidenceInterventionSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.OntologyDigest != ontology.Digest || value.EstimandCatalogDigest != estimands.Digest ||
		value.AssignmentSetDigest != assignments.Digest || value.SourceTrajectoryDigest != parent.Digest ||
		value.IntervenedTrajectoryDigest != child.Digest || value.ProviderCalls != 0 || value.NetworkRequired {
		return errors.New("evidence intervention identity or execution boundary is invalid")
	}
	if !value.TaskIdentityPreserved || parent.SourceDigest != child.SourceDigest ||
		!value.AssignmentFrozenBeforeOutput || assignments.FreezeStage != AssignmentFreezeStagePreExecute {
		return errors.New("evidence intervention lineage or assignment freeze is invalid")
	}
	if _, found := estimands.Definition(value.EstimandFamily); !found {
		return errors.New("evidence intervention estimand family is invalid")
	}
	if _, found := ontology.Operator(value.Operator); !found {
		return errors.New("evidence intervention operator is invalid")
	}
	if len(value.Targets) == 0 || len(child.Events) != len(parent.Events) {
		return errors.New("evidence intervention target or event count is invalid")
	}
	return nil
}

func inspectInterventionTargets(
	value EvidenceIntervention,
	parent, child preprocess.Trajectory,
) (interventionValidationState, error) {
	state := newInterventionValidationState(value, parent, child)
	for index, target := range value.Targets {
		if index > 0 && compareInterventionTargets(value.Targets[index-1], target) >= 0 {
			return interventionValidationState{}, errors.New("evidence intervention targets are unordered or duplicated")
		}
		if err := inspectInterventionTarget(value, parent, child, target, index, &state); err != nil {
			return interventionValidationState{}, err
		}
	}
	return state, nil
}

func newInterventionValidationState(
	value EvidenceIntervention,
	parent, child preprocess.Trajectory,
) interventionValidationState {
	state := interventionValidationState{
		assignmentTargets: make([]AssignmentTarget, len(value.Targets)), changedEventSet: make(map[string]struct{}),
		changedFields: make([]preprocess.FieldPath, 0, len(value.Targets)), eventIDRemap: make(map[string]string),
		pathsByParentEvent: make(map[string][]preprocess.FieldPath), parentIndexes: make(map[string]int, len(parent.Events)),
		childIndexes: make(map[string]int, len(child.Events)),
	}
	for index, event := range parent.Events {
		state.parentIndexes[event.ID] = index
	}
	for index, event := range child.Events {
		state.childIndexes[event.ID] = index
	}
	return state
}

func inspectInterventionTarget(
	value EvidenceIntervention,
	parent, child preprocess.Trajectory,
	target EvidenceInterventionTarget,
	index int,
	state *interventionValidationState,
) error {
	parentIndex, parentFound := state.parentIndexes[target.ParentEventID]
	childIndex, childFound := state.childIndexes[target.IntervenedEventID]
	if !parentFound || !childFound || parentIndex != childIndex {
		return errors.New("evidence intervention target event lineage is invalid")
	}
	parentEvent, childEvent := parent.Events[parentIndex], child.Events[childIndex]
	if !sameInterventionEventLineage(parentEvent, childEvent) || target.EventKind != parentEvent.Kind {
		return errors.New("evidence intervention changed immutable event lineage")
	}
	fieldTarget, beforeDigest, beforePresent, err := interventionFieldState(parentEvent, target.FieldPath)
	if err != nil || !beforePresent {
		return errors.New("evidence intervention parent field state is invalid")
	}
	_, afterDigest, afterPresent, err := interventionFieldState(childEvent, target.FieldPath)
	if err != nil || value.Operator == OperatorRemove && afterPresent || value.Operator != OperatorRemove && !afterPresent {
		return errors.New("evidence intervention child field state is invalid")
	}
	changed := beforeDigest != afterDigest
	if target.ValueKind != fieldTarget.ValueKind || target.BeforeStateDigest != beforeDigest ||
		target.AfterStateDigest != afterDigest || target.Changed != changed ||
		value.Operator == OperatorRetain && changed || value.Operator != OperatorRetain && !changed {
		return errors.New("evidence intervention target state differs from its trajectories")
	}
	recordInspectedInterventionTarget(target, index, changed, state)
	return nil
}

func recordInspectedInterventionTarget(
	target EvidenceInterventionTarget,
	index int,
	changed bool,
	state *interventionValidationState,
) {
	state.assignmentTargets[index] = AssignmentTarget{EventID: target.ParentEventID, FieldPath: target.FieldPath}
	state.pathsByParentEvent[target.ParentEventID] = append(state.pathsByParentEvent[target.ParentEventID], target.FieldPath)
	if !changed {
		return
	}
	state.changedEventSet[target.ParentEventID] = struct{}{}
	state.changedFields = append(state.changedFields, derivedInterventionFieldPath(target.ParentEventID, target.FieldPath))
	state.eventIDRemap[target.ParentEventID] = target.IntervenedEventID
}

func validateInterventionTargetContract(
	value EvidenceIntervention,
	ontology FactorOntology,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	state interventionValidationState,
) error {
	if err := validateOperatorTargets(ontology, assignments, parent, value.Operator, value.FactorID, state.assignmentTargets); err != nil {
		return err
	}
	changedEvents := make([]string, 0, len(state.changedEventSet))
	for eventID := range state.changedEventSet {
		changedEvents = append(changedEvents, eventID)
	}
	sort.Strings(changedEvents)
	slices.SortFunc(state.changedFields, compareInterventionFieldPaths)
	if !slices.Equal(value.ChangedEventIDs, changedEvents) || !slices.Equal(value.ChangedFieldPaths, state.changedFields) {
		return errors.New("evidence intervention changed-field accounting is invalid")
	}
	return nil
}

func validateInterventionEventIsolation(
	parent, child preprocess.Trajectory,
	pathsByParentEvent map[string][]preprocess.FieldPath,
) error {
	for index := range parent.Events {
		parentEvent, childEvent := parent.Events[index], child.Events[index]
		paths := pathsByParentEvent[parentEvent.ID]
		if len(paths) == 0 {
			if !reflect.DeepEqual(parentEvent, childEvent) {
				return errors.New("evidence intervention changed an untargeted event")
			}
			continue
		}
		parentProjection, err := untargetedInterventionEventDigest(parentEvent, paths)
		if err != nil {
			return err
		}
		childProjection, err := untargetedInterventionEventDigest(childEvent, paths)
		if err != nil || parentProjection != childProjection {
			return errors.New("evidence intervention changed an untargeted event field")
		}
	}
	return nil
}

func validateInterventionDerivation(
	value EvidenceIntervention,
	parent, child preprocess.Trajectory,
	state interventionValidationState,
) error {
	expectedLinks := remapInterventionLinks(parent.Links, state.eventIDRemap)
	expectedChild, err := preprocess.DeriveTrajectory(parent, child.Events, expectedLinks, preprocess.DerivationSpec{
		Relation: EvidenceInterventionRelation, Validator: EvidenceInterventionValidator,
		ChangedEventIDs: value.ChangedEventIDs, ChangedFieldPaths: value.ChangedFieldPaths,
	})
	if err != nil || !reflect.DeepEqual(child, expectedChild) {
		return errors.New("evidence intervention trajectory does not reproduce from canonical derivation")
	}
	return nil
}

func validateInterventionAdmissibility(value EvidenceIntervention) error {
	if value.OutcomePreservation != nil {
		if err := value.OutcomePreservation.Validate(); err != nil {
			return err
		}
		if value.OutcomePreservation.Mechanism != InterventionOutcomeMechanism {
			return errors.New("evidence intervention outcome mechanism is invalid")
		}
	}
	admissibility, reasons := assessInterventionAdmissibility(value.EstimandFamily, value.OutcomePreservation)
	if value.Admissibility != admissibility || !slices.Equal(value.AdmissibilityReasons, reasons) ||
		value.DenominatorEligible != (admissibility == InterventionAdmissible) {
		return errors.New("evidence intervention admissibility is invalid")
	}
	return nil
}

func validateInterventionDigest(value EvidenceIntervention) error {
	digest, err := evidenceInterventionDigest(value)
	if err != nil || value.Digest != digest || value.InterventionID != "intervention-"+digest {
		return errors.New("evidence intervention digest is invalid")
	}
	return nil
}

func applyInterventionOperator(
	event *preprocess.Event,
	target FieldTarget,
	operator InterventionOperator,
	replacement *InterventionValue,
) error {
	if operator == OperatorControlledReplacement {
		if err := validateInterventionReplacement(target, replacement); err != nil {
			return interventionFailure(FailureValueInvalid, err)
		}
	} else if replacement != nil {
		return interventionFailure(FailureValueInvalid, fmt.Errorf("operator %q does not accept a replacement", operator))
	}
	switch operator {
	case OperatorRetain:
		return nil
	case OperatorRemove:
		clearInterventionField(event, target.FieldPath)
		return nil
	case OperatorTypedMask:
		return maskInterventionField(event, target.FieldPath)
	case OperatorControlledReplacement:
		return replaceInterventionField(event, target.FieldPath, *replacement)
	default:
		return interventionFailure(FailureOperatorInvalid, fmt.Errorf("unknown operator %q", operator))
	}
}

func validateInterventionReplacement(target FieldTarget, replacement *InterventionValue) error {
	if replacement == nil {
		return errors.New("controlled replacement requires one typed value")
	}
	if interventionReplacementValueCount(*replacement) != 1 {
		return errors.New("controlled replacement requires exactly one typed value")
	}
	switch target.ValueKind {
	case ValueText:
		if replacement.Text == nil || *replacement.Text == "" {
			return errors.New("text replacement must be nonempty")
		}
	case ValueInteger:
		if replacement.Integer == nil {
			return errors.New("integer target requires an integer replacement")
		}
	case ValueBoolean:
		if replacement.Boolean == nil {
			return errors.New("boolean target requires a boolean replacement")
		}
	case ValueContentParts:
		return validateInterventionContentReplacement(replacement.ContentParts)
	default:
		return fmt.Errorf("unsupported replacement value kind %q", target.ValueKind)
	}
	return nil
}

func interventionReplacementValueCount(value InterventionValue) int {
	count := 0
	for _, present := range []bool{value.Text != nil, value.Integer != nil, value.Boolean != nil, value.ContentParts != nil} {
		if present {
			count++
		}
	}
	return count
}

func validateInterventionContentReplacement(parts *[]preprocess.ContentPart) error {
	if parts == nil || len(*parts) == 0 {
		return errors.New("content-parts replacement must be nonempty")
	}
	for _, part := range *parts {
		if part.Kind == preprocess.ContentEventReference {
			return errors.New("content-parts replacement requires explicit dependency closure for event references")
		}
		switch part.Kind {
		case preprocess.ContentText, preprocess.ContentImage, preprocess.ContentAudio, preprocess.ContentFile:
		default:
			return fmt.Errorf("unsupported replacement content kind %q", part.Kind)
		}
		if part.Kind == preprocess.ContentText && part.Text == "" {
			return errors.New("text content replacement must be nonempty")
		}
	}
	return nil
}

func replaceInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	switch event.Kind {
	case preprocess.EventCommand:
		replacement := *value.Integer
		event.Command.ExitCode = &replacement
	case preprocess.EventError:
		return replaceErrorInterventionField(event, path, value)
	case preprocess.EventEvaluation:
		return replaceEvaluationInterventionField(event, path, value)
	case preprocess.EventFileChange:
		return replaceFileChangeInterventionField(event, path, value)
	case preprocess.EventMessage:
		return replaceMessageInterventionField(event, path, value)
	case preprocess.EventMetadata:
		return replaceMetadataInterventionField(event, path, value)
	case preprocess.EventOutput:
		return replaceOutputInterventionField(event, path, value)
	case preprocess.EventToolCall:
		event.ToolCall.Arguments = *value.Text
	case preprocess.EventToolResult:
		return replaceToolResultInterventionField(event, path, value)
	default:
		return fmt.Errorf("unsupported intervention field %q", path)
	}
	return nil
}

func replaceErrorInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	switch path {
	case PathErrorClass:
		event.Error.Class = *value.Text
	case PathErrorSafeMessage:
		event.Error.SafeMessage = *value.Text
	default:
		return fmt.Errorf("unsupported error intervention field %q", path)
	}
	return nil
}

func replaceEvaluationInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	switch path {
	case PathEvaluationErrorType:
		event.Evaluation.ErrorType = *value.Text
	case PathEvaluationExplanation:
		event.Evaluation.Explanation = *value.Text
	case PathEvaluationScoreLabel:
		event.Evaluation.ScoreLabel = *value.Text
	case PathEvaluationScoreValue:
		event.Evaluation.ScoreValue = *value.Text
	default:
		return fmt.Errorf("unsupported evaluation intervention field %q", path)
	}
	return nil
}

func replaceFileChangeInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	switch path {
	case PathFileChangeContentDigest:
		event.FileChange.ContentDigest = *value.Text
	case PathFileChangeDiff:
		event.FileChange.Diff = *value.Text
	case PathFileChangeDiffDigest:
		event.FileChange.DiffDigest = *value.Text
	case PathFileChangeOperation:
		event.FileChange.Operation = *value.Text
	case PathFileChangePathAlias:
		event.FileChange.PathAlias = *value.Text
	case PathFileChangeSizeBytes:
		event.FileChange.SizeBytes = *value.Integer
	default:
		return fmt.Errorf("unsupported file-change intervention field %q", path)
	}
	return nil
}

func replaceMessageInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	if path == PathMessageParts {
		event.Message.Parts = slices.Clone(*value.ContentParts)
		return nil
	}
	if path == PathMessagePhase {
		event.Message.Phase = *value.Text
		return nil
	}
	return fmt.Errorf("unsupported message intervention field %q", path)
}

func replaceMetadataInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	switch path {
	case PathMetadataName:
		event.Metadata.Name = *value.Text
	case PathMetadataPresent:
		event.Metadata.Present = *value.Boolean
	case PathMetadataValue:
		event.Metadata.Value = *value.Text
	case PathMetadataValueDigest:
		event.Metadata.ValueDigest = *value.Text
	default:
		return fmt.Errorf("unsupported metadata intervention field %q", path)
	}
	return nil
}

func replaceOutputInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	switch path {
	case PathOutputStatus:
		event.Output.Status = *value.Text
	case PathOutputStream:
		event.Output.Stream = *value.Text
	case PathOutputText:
		event.Output.Text = *value.Text
	default:
		return fmt.Errorf("unsupported output intervention field %q", path)
	}
	return nil
}

func replaceToolResultInterventionField(event *preprocess.Event, path preprocess.FieldPath, value InterventionValue) error {
	switch path {
	case PathToolResultError:
		event.ToolResult.Error = *value.Boolean
	case PathToolResultExitCode:
		replacement := *value.Integer
		event.ToolResult.ExitCode = &replacement
	case PathToolResultOutput:
		event.ToolResult.Output = slices.Clone(*value.ContentParts)
	case PathToolResultStderr:
		event.ToolResult.Stderr = slices.Clone(*value.ContentParts)
	case PathToolResultStdout:
		event.ToolResult.Stdout = slices.Clone(*value.ContentParts)
	case PathToolResultStatus:
		event.ToolResult.Status = *value.Text
	default:
		return fmt.Errorf("unsupported tool-result intervention field %q", path)
	}
	return nil
}

func maskInterventionField(event *preprocess.Event, path preprocess.FieldPath) error {
	maskedText := TypedMaskText
	maskedParts := []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: TypedMaskText}}
	target, found := canonicalTarget(event.Kind, path)
	if !found {
		return fmt.Errorf("unknown intervention field %s%s", event.Kind, path)
	}
	value := InterventionValue{}
	switch target.ValueKind {
	case ValueText:
		value.Text = &maskedText
	case ValueContentParts:
		value.ContentParts = &maskedParts
	default:
		return fmt.Errorf("field %s%s cannot be typed-masked", event.Kind, path)
	}
	return replaceInterventionField(event, path, value)
}

func clearInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	switch event.Kind {
	case preprocess.EventCommand:
		event.Command.ExitCode = nil
	case preprocess.EventError:
		clearErrorInterventionField(event, path)
	case preprocess.EventEvaluation:
		clearEvaluationInterventionField(event, path)
	case preprocess.EventFileChange:
		clearFileChangeInterventionField(event, path)
	case preprocess.EventMessage:
		clearMessageInterventionField(event, path)
	case preprocess.EventMetadata:
		clearMetadataInterventionField(event, path)
	case preprocess.EventOutput:
		clearOutputInterventionField(event, path)
	case preprocess.EventToolCall:
		event.ToolCall.Arguments = ""
	case preprocess.EventToolResult:
		clearToolResultInterventionField(event, path)
	}
}

func clearErrorInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	if path == PathErrorClass {
		event.Error.Class = ""
	} else {
		event.Error.SafeMessage = ""
	}
}

func clearEvaluationInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	switch path {
	case PathEvaluationErrorType:
		event.Evaluation.ErrorType = ""
	case PathEvaluationExplanation:
		event.Evaluation.Explanation = ""
	case PathEvaluationScoreLabel:
		event.Evaluation.ScoreLabel = ""
	case PathEvaluationScoreValue:
		event.Evaluation.ScoreValue = ""
	}
}

func clearFileChangeInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	switch path {
	case PathFileChangeContentDigest:
		event.FileChange.ContentDigest = ""
	case PathFileChangeDiff:
		event.FileChange.Diff = ""
	case PathFileChangeDiffDigest:
		event.FileChange.DiffDigest = ""
	case PathFileChangeOperation:
		event.FileChange.Operation = ""
	case PathFileChangePathAlias:
		event.FileChange.PathAlias = ""
	case PathFileChangeSizeBytes:
		event.FileChange.SizeBytes = 0
	}
}

func clearMessageInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	if path == PathMessageParts {
		event.Message.Parts = nil
	} else {
		event.Message.Phase = ""
	}
}

func clearMetadataInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	switch path {
	case PathMetadataName:
		event.Metadata.Name = ""
	case PathMetadataPresent:
		event.Metadata.Present = false
	case PathMetadataValue:
		event.Metadata.Value = ""
	case PathMetadataValueDigest:
		event.Metadata.ValueDigest = ""
	}
}

func clearOutputInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	switch path {
	case PathOutputStatus:
		event.Output.Status = ""
	case PathOutputStream:
		event.Output.Stream = ""
	case PathOutputText:
		event.Output.Text = ""
	}
}

func clearToolResultInterventionField(event *preprocess.Event, path preprocess.FieldPath) {
	switch path {
	case PathToolResultError:
		event.ToolResult.Error = false
	case PathToolResultExitCode:
		event.ToolResult.ExitCode = nil
	case PathToolResultOutput:
		event.ToolResult.Output = nil
	case PathToolResultStderr:
		event.ToolResult.Stderr = nil
	case PathToolResultStdout:
		event.ToolResult.Stdout = nil
	case PathToolResultStatus:
		event.ToolResult.Status = ""
	}
}

func interventionFieldState(event preprocess.Event, path preprocess.FieldPath) (FieldTarget, string, bool, error) {
	target, found := canonicalTarget(event.Kind, path)
	if !found {
		return FieldTarget{}, "", false, fmt.Errorf("unknown intervention field %s%s", event.Kind, path)
	}
	present, err := interventionFieldPresent(event, path)
	if err != nil {
		return FieldTarget{}, "", false, err
	}
	valueDigest := ""
	if present {
		_, valueDigest, err = eventFieldMaterial(event, path)
		if err != nil {
			return FieldTarget{}, "", false, err
		}
	}
	digest, err := protocolkit.Digest(interventionFieldStateMaterial{Present: present, ValueDigest: valueDigest})
	if err != nil {
		return FieldTarget{}, "", false, err
	}
	return target, digest, present, nil
}

func interventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) (bool, error) {
	switch event.Kind {
	case preprocess.EventCommand:
		return event.Command != nil && event.Command.ExitCode != nil, nil
	case preprocess.EventError:
		return errorInterventionFieldPresent(event, path), nil
	case preprocess.EventEvaluation:
		return evaluationInterventionFieldPresent(event, path), nil
	case preprocess.EventFileChange:
		return fileChangeInterventionFieldPresent(event, path), nil
	case preprocess.EventMessage:
		return messageInterventionFieldPresent(event, path), nil
	case preprocess.EventMetadata:
		return metadataInterventionFieldPresent(event, path), nil
	case preprocess.EventOutput:
		return outputInterventionFieldPresent(event, path), nil
	case preprocess.EventToolCall:
		return event.ToolCall != nil && event.ToolCall.Arguments != "", nil
	case preprocess.EventToolResult:
		return toolResultInterventionFieldPresent(event, path), nil
	default:
		return false, fmt.Errorf("unsupported intervention field %q", path)
	}
}

func errorInterventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) bool {
	if event.Error == nil {
		return false
	}
	return path == PathErrorClass || path == PathErrorSafeMessage && event.Error.SafeMessage != ""
}

func evaluationInterventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) bool {
	if event.Evaluation == nil {
		return false
	}
	switch path {
	case PathEvaluationErrorType:
		return event.Evaluation.ErrorType != ""
	case PathEvaluationExplanation:
		return event.Evaluation.Explanation != ""
	case PathEvaluationScoreLabel:
		return event.Evaluation.ScoreLabel != ""
	case PathEvaluationScoreValue:
		return event.Evaluation.ScoreValue != ""
	default:
		return false
	}
}

func fileChangeInterventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) bool {
	if event.FileChange == nil {
		return false
	}
	switch path {
	case PathFileChangeContentDigest:
		return event.FileChange.ContentDigest != ""
	case PathFileChangeDiff:
		return event.FileChange.Diff != ""
	case PathFileChangeDiffDigest:
		return event.FileChange.DiffDigest != ""
	case PathFileChangeOperation, PathFileChangeSizeBytes:
		return true
	case PathFileChangePathAlias:
		return event.FileChange.PathAlias != ""
	default:
		return false
	}
}

func messageInterventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) bool {
	if event.Message == nil {
		return false
	}
	return path == PathMessageParts || path == PathMessagePhase && event.Message.Phase != ""
}

func metadataInterventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) bool {
	if event.Metadata == nil {
		return false
	}
	switch path {
	case PathMetadataName, PathMetadataPresent:
		return true
	case PathMetadataValue:
		return event.Metadata.Value != ""
	case PathMetadataValueDigest:
		return event.Metadata.ValueDigest != ""
	default:
		return false
	}
}

func outputInterventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) bool {
	if event.Output == nil {
		return false
	}
	switch path {
	case PathOutputStatus:
		return event.Output.Status != ""
	case PathOutputStream:
		return event.Output.Stream != ""
	case PathOutputText:
		return event.Output.Text != ""
	default:
		return false
	}
}

func toolResultInterventionFieldPresent(event preprocess.Event, path preprocess.FieldPath) bool {
	if event.ToolResult == nil {
		return false
	}
	switch path {
	case PathToolResultError, PathToolResultStatus:
		return true
	case PathToolResultExitCode:
		return event.ToolResult.ExitCode != nil
	case PathToolResultOutput:
		return len(event.ToolResult.Output) > 0
	case PathToolResultStderr:
		return len(event.ToolResult.Stderr) > 0
	case PathToolResultStdout:
		return len(event.ToolResult.Stdout) > 0
	default:
		return false
	}
}

func interventionTargetHasEventReference(event preprocess.Event, path preprocess.FieldPath) bool {
	var parts []preprocess.ContentPart
	switch path {
	case PathMessageParts:
		parts = event.Message.Parts
	case PathToolResultOutput:
		parts = event.ToolResult.Output
	case PathToolResultStderr:
		parts = event.ToolResult.Stderr
	case PathToolResultStdout:
		parts = event.ToolResult.Stdout
	default:
		return false
	}
	for _, part := range parts {
		if part.Kind == preprocess.ContentEventReference {
			return true
		}
	}
	return false
}

func untargetedInterventionEventDigest(event preprocess.Event, paths []preprocess.FieldPath) (string, error) {
	cloned, err := cloneInterventionEvent(event)
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		clearInterventionField(&cloned, path)
	}
	cloned.ID = ""
	cloned.ContentBytes = 0
	cloned.RetainedBytes = 0
	cloned.EstimatedTokens = 0
	cloned.ContentDigest = ""
	return protocolkit.Digest(cloned)
}

func assessInterventionAdmissibility(
	family EstimandFamily,
	preservation *outcome.Preservation,
) (InterventionAdmissibility, []InterventionFailureCode) {
	if preservation == nil {
		return InterventionUnresolved, []InterventionFailureCode{FailureOutcomePreservationMissing}
	}
	hasReason := func(reason string) bool { return slices.Contains(preservation.InadmissibilityReasons, reason) }
	if hasReason("task_lineage_mismatch") {
		return InterventionInadmissible, []InterventionFailureCode{FailureTaskLineageMismatch}
	}
	if hasReason("outcome_state_not_decisive") {
		return InterventionUnresolved, []InterventionFailureCode{FailureOutcomeStateUnresolved}
	}
	switch family {
	case EstimandEvidenceOnly:
		if preservation.Admissible {
			return InterventionAdmissible, []InterventionFailureCode{}
		}
		return InterventionInadmissible, []InterventionFailureCode{FailureOutcomePreservationReject}
	case EstimandQualityChanging:
		if preservation.Admissible {
			return InterventionInadmissible, []InterventionFailureCode{FailureQualityChangeNotEstablished}
		}
		if hasReason("outcome_state_changed") {
			return InterventionUnresolved, []InterventionFailureCode{FailureRelationAdmissionRequired}
		}
		return InterventionInadmissible, []InterventionFailureCode{FailureOutcomePreservationReject}
	default:
		return InterventionInadmissible, []InterventionFailureCode{FailureEstimandInvalid}
	}
}

func sealEvidenceIntervention(value EvidenceIntervention) (EvidenceIntervention, error) {
	digest, err := evidenceInterventionDigest(value)
	if err != nil {
		return EvidenceIntervention{}, err
	}
	value.InterventionID = "intervention-" + digest
	value.Digest = digest
	return value, nil
}

func evidenceInterventionDigest(value EvidenceIntervention) (string, error) {
	value.InterventionID = ""
	value.Digest = ""
	return protocolkit.Digest(value)
}

func cloneInterventionEvents(source []preprocess.Event) ([]preprocess.Event, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("clone intervention events: %w", err)
	}
	var result []preprocess.Event
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, fmt.Errorf("clone intervention events: %w", err)
	}
	return result, nil
}

func cloneInterventionEvent(source preprocess.Event) (preprocess.Event, error) {
	events, err := cloneInterventionEvents([]preprocess.Event{source})
	if err != nil {
		return preprocess.Event{}, err
	}
	return events[0], nil
}

func remapInterventionLinks(links []preprocess.Link, remapped map[string]string) []preprocess.Link {
	result := slices.Clone(links)
	for index := range result {
		if replacement, found := remapped[result[index].FromID]; found {
			result[index].FromID = replacement
		}
		if replacement, found := remapped[result[index].ToID]; found {
			result[index].ToID = replacement
		}
	}
	return result
}

func sameInterventionEventLineage(left, right preprocess.Event) bool {
	return left.Kind == right.Kind && left.Order == right.Order && left.Source == right.Source &&
		left.SourceEventID == right.SourceEventID && left.Timestamp == right.Timestamp && left.Sensitivity == right.Sensitivity
}

func derivedInterventionFieldPath(eventID string, path preprocess.FieldPath) preprocess.FieldPath {
	return preprocess.FieldPath("/events/" + eventID + string(path))
}

func compareInterventionTargetRequests(left, right InterventionTargetRequest) int {
	return strings.Compare(left.EventID+"\x00"+string(left.FieldPath), right.EventID+"\x00"+string(right.FieldPath))
}

func compareInterventionTargets(left, right EvidenceInterventionTarget) int {
	return strings.Compare(left.ParentEventID+"\x00"+string(left.FieldPath), right.ParentEventID+"\x00"+string(right.FieldPath))
}

func compareInterventionFieldPaths(left, right preprocess.FieldPath) int {
	return strings.Compare(string(left), string(right))
}

func interventionFailure(code InterventionFailureCode, err error) error {
	return &InterventionError{Code: code, Detail: err.Error()}
}
