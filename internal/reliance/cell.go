package reliance

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type cellTargetMetadata struct {
	factorID FactorID
	operator InterventionOperator
}

func ApplyEvidenceInterventionCell(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	plan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
) (EvidenceInterventionCellResult, error) {
	result, err := constructEvidenceInterventionCell(ontology, estimands, assignments, preregistration, parent, plan, request)
	if err != nil {
		return EvidenceInterventionCellResult{}, err
	}
	if err := result.Validate(ontology, estimands, assignments, preregistration, parent, plan, request); err != nil {
		return EvidenceInterventionCellResult{}, err
	}
	return result, nil
}

func (result EvidenceInterventionCellResult) Validate(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	plan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
) error {
	expected, err := constructEvidenceInterventionCell(ontology, estimands, assignments, preregistration, parent, plan, request)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(result, expected) {
		return errors.New("evidence intervention cell differs from its frozen parents and level assignment")
	}
	return nil
}

func constructEvidenceInterventionCell(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	plan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
) (EvidenceInterventionCellResult, error) {
	levels, err := validateCellParentsAndRequest(ontology, estimands, assignments, preregistration, parent, plan, request)
	if err != nil {
		return EvidenceInterventionCellResult{}, err
	}
	return constructEvidenceInterventionCellFromLevels(
		ontology, estimands, assignments, preregistration, parent, plan, request, levels,
	)
}

func constructEvidenceInterventionCellFromValidatedParents(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	plan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
) (EvidenceInterventionCellResult, error) {
	levels, err := validateCellRequest(preregistration, request)
	if err != nil {
		return EvidenceInterventionCellResult{}, err
	}
	return constructEvidenceInterventionCellFromLevels(
		ontology, estimands, assignments, preregistration, parent, plan, request, levels,
	)
}

func constructEvidenceInterventionCellFromLevels(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	plan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
	levels []stats.FactorialLevel,
) (EvidenceInterventionCellResult, error) {
	child, changes, active, metadata, err := applyFactorialCellTreatments(parent, plan, levels)
	if err != nil {
		return EvidenceInterventionCellResult{}, err
	}
	preservation, err := buildCellPreservation(plan.EstimandFamily, request)
	if err != nil {
		return EvidenceInterventionCellResult{}, err
	}
	status, reasons := assessInterventionAdmissibility(plan.EstimandFamily, preservation)
	cell, err := sealEvidenceInterventionCell(newEvidenceInterventionCell(
		ontology, estimands, assignments, preregistration, parent, child, plan, request.CellID,
		levels, active, cellTargets(changes.targets, metadata), changes, preservation, status, reasons,
	))
	if err != nil {
		return EvidenceInterventionCellResult{}, err
	}
	return EvidenceInterventionCellResult{Cell: cell, Trajectory: child}, nil
}

func validateCellParentsAndRequest(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	plan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
) ([]stats.FactorialLevel, error) {
	if err := preregistration.Validate(ontology, estimands); err != nil {
		return nil, err
	}
	if err := plan.Validate(ontology, estimands, assignments, parent); err != nil {
		return nil, err
	}
	return validateCellRequest(preregistration, request)
}

func validateCellRequest(preregistration Preregistration, request EvidenceInterventionCellRequest) ([]stats.FactorialLevel, error) {
	if strings.TrimSpace(request.CellID) == "" || request.CellID != strings.TrimSpace(request.CellID) ||
		strings.IndexFunc(request.CellID, unicode.IsControl) >= 0 {
		return nil, errors.New("evidence intervention cell ID is invalid")
	}
	if (request.SourceOutcome == nil) != (request.IntervenedOutcome == nil) {
		return nil, interventionFailure(FailureOutcomeInputInvalid, errors.New("cell outcome records must be supplied together"))
	}
	return normalizeCellLevels(preregistration, request.Levels)
}

func normalizeCellLevels(preregistration Preregistration, values []stats.FactorialLevel) ([]stats.FactorialLevel, error) {
	result := slices.Clone(values)
	slices.SortFunc(result, func(left, right stats.FactorialLevel) int {
		return strings.Compare(left.FactorID, right.FactorID)
	})
	want := preregisteredCellLevelIDs(preregistration)
	if len(result) != len(want) {
		return nil, fmt.Errorf("evidence intervention cell has %d levels, want %d", len(result), len(want))
	}
	for index, level := range result {
		if level.FactorID != want[index] || level.Level != -1 && level.Level != 1 {
			return nil, errors.New("evidence intervention cell levels must be complete, unique, and coded as -1 or +1")
		}
	}
	return result, nil
}

func preregisteredCellLevelIDs(preregistration Preregistration) []string {
	result := make([]string, 0, len(preregistration.MainEffects)+1)
	for _, factorID := range preregistration.MainEffects {
		result = append(result, string(factorID))
	}
	result = append(result, PresentationOrderTerm)
	slices.Sort(result)
	return result
}

func applyFactorialCellTreatments(
	parent preprocess.Trajectory,
	plan FactorTreatmentPlan,
	levels []stats.FactorialLevel,
) (preprocess.Trajectory, interventionChanges, []FactorID, map[string]cellTargetMetadata, error) {
	prepared, err := newPreparedIntervention(parent)
	if err != nil {
		return preprocess.Trajectory{}, interventionChanges{}, nil, nil, err
	}
	levelByID := cellLevelMap(levels)
	active := make([]FactorID, 0, len(plan.Treatments))
	metadata := make(map[string]cellTargetMetadata)
	for _, treatment := range plan.Treatments {
		if levelByID[string(treatment.FactorID)] > 0 {
			continue
		}
		active = append(active, treatment.FactorID)
		if err := appendCellTreatment(&prepared, treatment, metadata); err != nil {
			return preprocess.Trajectory{}, interventionChanges{}, nil, nil, err
		}
	}
	if len(active) == 0 {
		return parent, emptyInterventionChanges(parent), active, metadata, nil
	}
	changes, err := rebuildInterventionChanges(parent, prepared)
	if err != nil {
		return preprocess.Trajectory{}, interventionChanges{}, nil, nil, err
	}
	child, err := deriveInterventionCellTrajectory(parent, changes)
	return child, changes, active, metadata, err
}

func appendCellTreatment(prepared *preparedIntervention, treatment FactorTreatment, metadata map[string]cellTargetMetadata) error {
	for _, target := range treatment.Targets {
		key := assignmentTargetKey(target.EventID, target.FieldPath)
		if _, duplicate := metadata[key]; duplicate {
			return interventionFailure(FailureTargetInvalid, fmt.Errorf("factorial cell repeats target %s%s", target.EventID, target.FieldPath))
		}
		if err := prepareInterventionTarget(prepared, treatment.Operator, target); err != nil {
			return err
		}
		metadata[key] = cellTargetMetadata{factorID: treatment.FactorID, operator: treatment.Operator}
	}
	return nil
}

func deriveInterventionCellTrajectory(parent preprocess.Trajectory, changes interventionChanges) (preprocess.Trajectory, error) {
	links := remapInterventionLinks(parent.Links, changes.eventIDRemap)
	child, err := preprocess.DeriveTrajectory(parent, changes.events, links, preprocess.DerivationSpec{
		Relation: EvidenceInterventionCellRelation, Validator: EvidenceInterventionCellValidator,
		ChangedEventIDs: changes.changedEventIDs, ChangedFieldPaths: changes.changedFieldPaths,
	})
	if err != nil {
		return preprocess.Trajectory{}, interventionFailure(FailureDerivationInvalid, err)
	}
	return child, nil
}

func emptyInterventionChanges(parent preprocess.Trajectory) interventionChanges {
	return interventionChanges{
		events: parent.Events, targets: []EvidenceInterventionTarget{}, changedEventIDs: []string{},
		changedFieldPaths: []preprocess.FieldPath{}, eventIDRemap: map[string]string{},
	}
}

func cellLevelMap(levels []stats.FactorialLevel) map[string]float64 {
	result := make(map[string]float64, len(levels))
	for _, level := range levels {
		result[level.FactorID] = level.Level
	}
	return result
}

func buildCellPreservation(family EstimandFamily, request EvidenceInterventionCellRequest) (*outcome.Preservation, error) {
	return buildInterventionPreservation(EvidenceInterventionRequest{
		EstimandFamily: family, SourceOutcome: request.SourceOutcome, IntervenedOutcome: request.IntervenedOutcome,
	})
}

func cellTargets(values []EvidenceInterventionTarget, metadata map[string]cellTargetMetadata) []EvidenceCellTarget {
	result := make([]EvidenceCellTarget, len(values))
	for index, target := range values {
		item := metadata[assignmentTargetKey(target.ParentEventID, target.FieldPath)]
		result[index] = EvidenceCellTarget{FactorID: item.factorID, Operator: item.operator, Target: target}
	}
	slices.SortFunc(result, compareEvidenceCellTargets)
	return result
}

func compareEvidenceCellTargets(left, right EvidenceCellTarget) int {
	leftKey := string(left.FactorID) + "\x00" + left.Target.ParentEventID + "\x00" + string(left.Target.FieldPath)
	rightKey := string(right.FactorID) + "\x00" + right.Target.ParentEventID + "\x00" + string(right.Target.FieldPath)
	return strings.Compare(leftKey, rightKey)
}

func newEvidenceInterventionCell(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent, child preprocess.Trajectory,
	plan FactorTreatmentPlan,
	cellID string,
	levels []stats.FactorialLevel,
	active []FactorID,
	targets []EvidenceCellTarget,
	changes interventionChanges,
	preservation *outcome.Preservation,
	status InterventionAdmissibility,
	reasons []InterventionFailureCode,
) EvidenceInterventionCell {
	return EvidenceInterventionCell{
		SchemaVersion: EvidenceInterventionCellSchemaVersion, CanonicalPolicy: CanonicalPolicy, CellID: cellID,
		PreregistrationDigest: preregistration.Digest, TreatmentPlanDigest: plan.Digest,
		OntologyDigest: ontology.Digest, EstimandCatalogDigest: estimands.Digest, AssignmentSetDigest: assignments.Digest,
		EstimandFamily: plan.EstimandFamily, SourceTrajectoryDigest: parent.Digest, IntervenedTrajectoryDigest: child.Digest,
		Levels: slices.Clone(levels), ActiveFactors: slices.Clone(active), Targets: targets,
		ChangedEventIDs: slices.Clone(changes.changedEventIDs), ChangedFieldPaths: slices.Clone(changes.changedFieldPaths),
		OutcomePreservation: preservation, Admissibility: status, AdmissibilityReasons: slices.Clone(reasons),
		DenominatorEligible: status == InterventionAdmissible, TaskIdentityPreserved: parent.SourceDigest == child.SourceDigest,
		AssignmentFrozenBeforeOutput: assignments.FreezeStage == AssignmentFreezeStagePreExecute,
		ProviderCalls:                0, NetworkRequired: false,
	}
}

func sealEvidenceInterventionCell(value EvidenceInterventionCell) (EvidenceInterventionCell, error) {
	digest, err := evidenceInterventionCellDigest(value)
	if err != nil {
		return EvidenceInterventionCell{}, err
	}
	value.Digest = digest
	return value, nil
}

func evidenceInterventionCellDigest(value EvidenceInterventionCell) (string, error) {
	value.Digest = ""
	return protocolkit.Digest(value)
}
