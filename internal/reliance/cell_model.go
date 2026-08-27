package reliance

import (
	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

const (
	FactorTreatmentPlanSchemaVersion      = "evalwitness.factor-treatment-plan.v1"
	EvidenceInterventionCellSchemaVersion = "evalwitness.evidence-intervention-cell.v1"
	EvidenceInterventionCellRelation      = "evidence_reliance_factorial_cell"
	EvidenceInterventionCellValidator     = "evalwitness.evidence-intervention-cell-validator.v1"
	PresentationOrderPolicyVersion        = "evalwitness.dependency-valid-presentation-order.v1"
)

type FactorTreatment struct {
	FactorID FactorID                    `json:"factor_id"`
	Operator InterventionOperator        `json:"operator"`
	Targets  []InterventionTargetRequest `json:"targets"`
}

type FactorTreatmentPlan struct {
	SchemaVersion                string            `json:"schema_version"`
	CanonicalPolicy              string            `json:"canonical_policy"`
	PlanID                       string            `json:"plan_id"`
	OntologyDigest               string            `json:"ontology_digest"`
	EstimandCatalogDigest        string            `json:"estimand_catalog_digest"`
	AssignmentSetDigest          string            `json:"assignment_set_digest"`
	SourceTrajectoryDigest       string            `json:"source_trajectory_digest"`
	EstimandFamily               EstimandFamily    `json:"estimand_family"`
	Treatments                   []FactorTreatment `json:"treatments"`
	PresentationOrderPolicy      string            `json:"presentation_order_policy"`
	AssignmentFrozenBeforeOutput bool              `json:"assignment_frozen_before_output"`
	ProviderCalls                int               `json:"provider_calls"`
	NetworkRequired              bool              `json:"network_required"`
	Digest                       string            `json:"digest"`
}

type EvidenceCellTarget struct {
	FactorID FactorID                   `json:"factor_id"`
	Operator InterventionOperator       `json:"operator"`
	Target   EvidenceInterventionTarget `json:"target"`
}

type EvidenceInterventionCellRequest struct {
	CellID            string                 `json:"cell_id"`
	Levels            []stats.FactorialLevel `json:"levels"`
	SourceOutcome     *outcome.Record        `json:"source_outcome,omitempty"`
	IntervenedOutcome *outcome.Record        `json:"intervened_outcome,omitempty"`
}

type EvidenceInterventionCell struct {
	SchemaVersion                string                    `json:"schema_version"`
	CanonicalPolicy              string                    `json:"canonical_policy"`
	CellID                       string                    `json:"cell_id"`
	PreregistrationDigest        string                    `json:"preregistration_digest"`
	TreatmentPlanDigest          string                    `json:"treatment_plan_digest"`
	OntologyDigest               string                    `json:"ontology_digest"`
	EstimandCatalogDigest        string                    `json:"estimand_catalog_digest"`
	AssignmentSetDigest          string                    `json:"assignment_set_digest"`
	EstimandFamily               EstimandFamily            `json:"estimand_family"`
	SourceTrajectoryDigest       string                    `json:"source_trajectory_digest"`
	IntervenedTrajectoryDigest   string                    `json:"intervened_trajectory_digest"`
	Levels                       []stats.FactorialLevel    `json:"levels"`
	ActiveFactors                []FactorID                `json:"active_factors"`
	Targets                      []EvidenceCellTarget      `json:"targets"`
	ChangedEventIDs              []string                  `json:"changed_event_ids"`
	ChangedFieldPaths            []preprocess.FieldPath    `json:"changed_field_paths"`
	OutcomePreservation          *outcome.Preservation     `json:"outcome_preservation,omitempty"`
	Admissibility                InterventionAdmissibility `json:"admissibility"`
	AdmissibilityReasons         []InterventionFailureCode `json:"admissibility_reasons"`
	DenominatorEligible          bool                      `json:"denominator_eligible"`
	TaskIdentityPreserved        bool                      `json:"task_identity_preserved"`
	AssignmentFrozenBeforeOutput bool                      `json:"assignment_frozen_before_output"`
	ProviderCalls                int                       `json:"provider_calls"`
	NetworkRequired              bool                      `json:"network_required"`
	Digest                       string                    `json:"digest"`
}

type EvidenceInterventionCellResult struct {
	Cell       EvidenceInterventionCell
	Trajectory preprocess.Trajectory
}
