package reliance

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func SealFactorTreatmentPlan(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	family EstimandFamily,
	treatments []FactorTreatment,
) (FactorTreatmentPlan, error) {
	return constructFactorTreatmentPlan(ontology, estimands, assignments, parent, family, treatments)
}

func (value FactorTreatmentPlan) Validate(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
) error {
	expected, err := constructFactorTreatmentPlan(ontology, estimands, assignments, parent, value.EstimandFamily, value.Treatments)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("factor treatment plan differs from its frozen parents and treatments")
	}
	return nil
}

func constructFactorTreatmentPlan(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	family EstimandFamily,
	treatments []FactorTreatment,
) (FactorTreatmentPlan, error) {
	if err := validateTreatmentPlanParents(ontology, estimands, assignments, parent, family); err != nil {
		return FactorTreatmentPlan{}, err
	}
	normalized, err := normalizeFactorTreatments(treatments)
	if err != nil {
		return FactorTreatmentPlan{}, err
	}
	if err := validateCompleteFactorTreatments(ontology, estimands, assignments, parent, family, normalized); err != nil {
		return FactorTreatmentPlan{}, err
	}
	value := FactorTreatmentPlan{
		SchemaVersion: FactorTreatmentPlanSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		OntologyDigest: ontology.Digest, EstimandCatalogDigest: estimands.Digest,
		AssignmentSetDigest: assignments.Digest, SourceTrajectoryDigest: parent.Digest,
		EstimandFamily: family, Treatments: normalized, PresentationOrderPolicy: PresentationOrderPolicyVersion,
		AssignmentFrozenBeforeOutput: assignments.FreezeStage == AssignmentFreezeStagePreExecute,
		ProviderCalls:                0, NetworkRequired: false,
	}
	digest, err := factorTreatmentPlanDigest(value)
	if err != nil {
		return FactorTreatmentPlan{}, err
	}
	value.PlanID, value.Digest = "treatment-plan-"+digest, digest
	return value, nil
}

func validateTreatmentPlanParents(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	family EstimandFamily,
) error {
	if err := ontology.Validate(); err != nil {
		return err
	}
	if err := estimands.Validate(); err != nil {
		return err
	}
	if _, found := estimands.Definition(family); !found {
		return fmt.Errorf("factor treatment plan has unknown estimand family %q", family)
	}
	return assignments.Validate(ontology, parent)
}

func normalizeFactorTreatments(values []FactorTreatment) ([]FactorTreatment, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("clone factor treatments: %w", err)
	}
	var result []FactorTreatment
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, fmt.Errorf("clone factor treatments: %w", err)
	}
	for index := range result {
		slices.SortFunc(result[index].Targets, compareInterventionTargetRequests)
	}
	slices.SortFunc(result, func(left, right FactorTreatment) int {
		return strings.Compare(string(left.FactorID), string(right.FactorID))
	})
	return result, nil
}

func validateCompleteFactorTreatments(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	family EstimandFamily,
	treatments []FactorTreatment,
) error {
	if len(treatments) != len(ontology.Factors) {
		return fmt.Errorf("factor treatment plan covers %d factors, want %d", len(treatments), len(ontology.Factors))
	}
	for index, treatment := range treatments {
		if treatment.FactorID != ontology.Factors[index].FactorID || treatment.Operator == OperatorRetain {
			return errors.New("factor treatment plan is incomplete, unordered, or uses retain as a negative-level treatment")
		}
		if err := validateOneFactorTreatment(ontology, estimands, assignments, parent, family, treatment); err != nil {
			return fmt.Errorf("factor treatment %q: %w", treatment.FactorID, err)
		}
	}
	return nil
}

func validateOneFactorTreatment(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	parent preprocess.Trajectory,
	family EstimandFamily,
	treatment FactorTreatment,
) error {
	request := EvidenceInterventionRequest{
		FactorID: treatment.FactorID, Operator: treatment.Operator,
		EstimandFamily: family, Targets: treatment.Targets,
	}
	targets, err := validateInterventionInputs(ontology, estimands, assignments, parent, request)
	if err != nil {
		return err
	}
	if !slices.Equal(assignmentTargetsForFactor(assignments, treatment.FactorID), requestAssignmentTargets(targets)) {
		return errors.New("treatment targets do not cover every frozen assignment for the factor")
	}
	prepared, err := prepareIntervention(parent, request, targets)
	if err != nil {
		return err
	}
	_, err = rebuildInterventionChanges(parent, prepared)
	return err
}

func assignmentTargetsForFactor(assignments FactorAssignmentSet, factorID FactorID) []AssignmentTarget {
	result := make([]AssignmentTarget, 0)
	for _, assignment := range assignments.Assignments {
		if assignment.FactorID == factorID {
			result = append(result, AssignmentTarget{EventID: assignment.EventID, FieldPath: assignment.FieldPath})
		}
	}
	slices.SortFunc(result, compareAssignmentTargets)
	return result
}

func requestAssignmentTargets(targets []InterventionTargetRequest) []AssignmentTarget {
	result := make([]AssignmentTarget, len(targets))
	for index, target := range targets {
		result[index] = AssignmentTarget{EventID: target.EventID, FieldPath: target.FieldPath}
	}
	slices.SortFunc(result, compareAssignmentTargets)
	return result
}

func compareAssignmentTargets(left, right AssignmentTarget) int {
	return strings.Compare(left.EventID+"\x00"+string(left.FieldPath), right.EventID+"\x00"+string(right.FieldPath))
}

func factorTreatmentPlanDigest(value FactorTreatmentPlan) (string, error) {
	value.PlanID, value.Digest = "", ""
	return protocolkit.Digest(value)
}
