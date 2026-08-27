package reliance

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	EvidenceInterventionSchemaVersion = "evalwitness.evidence-intervention.v1"
	EvidenceInterventionValidator     = "evalwitness.evidence-intervention-validator.v1"
	EvidenceInterventionRelation      = "evidence_reliance_intervention"
	InterventionOutcomeMechanism      = "evalwitness.evidence-intervention-outcome.v1"
	TypedMaskText                     = "[MASKED]"
)

type InterventionAdmissibility string

const (
	InterventionAdmissible   InterventionAdmissibility = "admissible"
	InterventionInadmissible InterventionAdmissibility = "inadmissible"
	InterventionUnresolved   InterventionAdmissibility = "unresolved"
)

type InterventionFailureCode string

const (
	FailureAssignmentInvalid           InterventionFailureCode = "assignment_invalid"
	FailureDependencyClosureRequired   InterventionFailureCode = "dependency_closure_required"
	FailureDerivationInvalid           InterventionFailureCode = "derivation_invalid"
	FailureEstimandInvalid             InterventionFailureCode = "estimand_invalid"
	FailureNoEvidenceChange            InterventionFailureCode = "no_evidence_change"
	FailureOperatorInvalid             InterventionFailureCode = "operator_invalid"
	FailureOutcomeInputInvalid         InterventionFailureCode = "outcome_input_invalid"
	FailureOutcomePreservationMissing  InterventionFailureCode = "outcome_preservation_missing"
	FailureOutcomePreservationReject   InterventionFailureCode = "outcome_preservation_rejected"
	FailureOutcomeStateUnresolved      InterventionFailureCode = "outcome_state_unresolved"
	FailureQualityChangeNotEstablished InterventionFailureCode = "quality_change_not_established"
	FailureRelationAdmissionRequired   InterventionFailureCode = "relation_admission_required"
	FailureRelationEvidenceInvalid     InterventionFailureCode = "relation_evidence_invalid"
	FailureRelationFormalOnly          InterventionFailureCode = "relation_formal_only"
	FailureRelationHumanContradicted   InterventionFailureCode = "relation_human_contradicted"
	FailureRelationHumanUnresolved     InterventionFailureCode = "relation_human_unresolved"
	FailureTargetInvalid               InterventionFailureCode = "target_invalid"
	FailureTaskLineageMismatch         InterventionFailureCode = "task_lineage_mismatch"
	FailureValueInvalid                InterventionFailureCode = "value_invalid"
)

type InterventionError struct {
	Code   InterventionFailureCode
	Detail string
}

func (e *InterventionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

type InterventionValue struct {
	Text         *string                   `json:"text,omitempty"`
	Integer      *int                      `json:"integer,omitempty"`
	Boolean      *bool                     `json:"boolean,omitempty"`
	ContentParts *[]preprocess.ContentPart `json:"content_parts,omitempty"`
}

type InterventionTargetRequest struct {
	EventID     string               `json:"event_id"`
	FieldPath   preprocess.FieldPath `json:"field_path"`
	Replacement *InterventionValue   `json:"replacement,omitempty"`
}

type EvidenceInterventionRequest struct {
	FactorID          FactorID                    `json:"factor_id"`
	Operator          InterventionOperator        `json:"operator"`
	EstimandFamily    EstimandFamily              `json:"estimand_family"`
	Targets           []InterventionTargetRequest `json:"targets"`
	SourceOutcome     *outcome.Record             `json:"source_outcome,omitempty"`
	IntervenedOutcome *outcome.Record             `json:"intervened_outcome,omitempty"`
}

type EvidenceInterventionTarget struct {
	ParentEventID     string               `json:"parent_event_id"`
	IntervenedEventID string               `json:"intervened_event_id"`
	EventKind         preprocess.EventKind `json:"event_kind"`
	FieldPath         preprocess.FieldPath `json:"field_path"`
	ValueKind         FieldValueKind       `json:"value_kind"`
	BeforeStateDigest string               `json:"before_state_digest"`
	AfterStateDigest  string               `json:"after_state_digest"`
	Changed           bool                 `json:"changed"`
}

type EvidenceIntervention struct {
	SchemaVersion                string                       `json:"schema_version"`
	CanonicalPolicy              string                       `json:"canonical_policy"`
	InterventionID               string                       `json:"intervention_id"`
	OntologyDigest               string                       `json:"ontology_digest"`
	EstimandCatalogDigest        string                       `json:"estimand_catalog_digest"`
	AssignmentSetDigest          string                       `json:"assignment_set_digest"`
	FactorID                     FactorID                     `json:"factor_id"`
	Operator                     InterventionOperator         `json:"operator"`
	EstimandFamily               EstimandFamily               `json:"estimand_family"`
	SourceTrajectoryDigest       string                       `json:"source_trajectory_digest"`
	IntervenedTrajectoryDigest   string                       `json:"intervened_trajectory_digest"`
	Targets                      []EvidenceInterventionTarget `json:"targets"`
	ChangedEventIDs              []string                     `json:"changed_event_ids"`
	ChangedFieldPaths            []preprocess.FieldPath       `json:"changed_field_paths"`
	OutcomePreservation          *outcome.Preservation        `json:"outcome_preservation,omitempty"`
	Admissibility                InterventionAdmissibility    `json:"admissibility"`
	AdmissibilityReasons         []InterventionFailureCode    `json:"admissibility_reasons"`
	DenominatorEligible          bool                         `json:"denominator_eligible"`
	TaskIdentityPreserved        bool                         `json:"task_identity_preserved"`
	AssignmentFrozenBeforeOutput bool                         `json:"assignment_frozen_before_output"`
	ProviderCalls                int                          `json:"provider_calls"`
	NetworkRequired              bool                         `json:"network_required"`
	Digest                       string                       `json:"digest"`
}

type EvidenceInterventionResult struct {
	Intervention EvidenceIntervention
	Trajectory   preprocess.Trajectory
}
