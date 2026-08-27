package reliance

import (
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
)

const (
	RelationBackedAdmissionSchemaVersion = "evalwitness.relation-backed-intervention-admission.v1"
	RelationEventGraphSchemaVersion      = "evalwitness.relation-event-graph.v1"
)

type RelationAdmissionInputs struct {
	Ontology             FactorOntology
	Estimands            EstimandCatalog
	Assignments          FactorAssignmentSet
	Parent               preprocess.Trajectory
	Intervention         EvidenceInterventionResult
	Relation             stress.Relation
	ConstructAdmission   stress.ConstructAdmission
	ReplayedRelationCase stress.ReplayedRelationCaseV3
}

type RelationBackedInterventionAdmission struct {
	SchemaVersion                    string                    `json:"schema_version"`
	CanonicalPolicy                  string                    `json:"canonical_policy"`
	InterventionDigest               string                    `json:"intervention_digest"`
	AssignmentSetDigest              string                    `json:"assignment_set_digest"`
	FactorID                         FactorID                  `json:"factor_id"`
	Operator                         InterventionOperator      `json:"operator"`
	EstimandFamily                   EstimandFamily            `json:"estimand_family"`
	InterventionAdmissibility        InterventionAdmissibility `json:"intervention_admissibility"`
	InterventionAdmissibilityReasons []InterventionFailureCode `json:"intervention_admissibility_reasons"`
	RelationID                       string                    `json:"relation_id"`
	RelationDigest                   string                    `json:"relation_digest"`
	RelationCaseID                   string                    `json:"relation_case_id"`
	RelationManifestDigest           string                    `json:"relation_manifest_digest"`
	ConstructAdmissionDigest         string                    `json:"construct_admission_digest"`
	FormalWitnessDigest              string                    `json:"formal_witness_digest"`
	ConstructFirewallDigest          string                    `json:"construct_firewall_digest"`
	OwnerAttestationDigest           string                    `json:"owner_attestation_digest"`
	TerminalLedgerDigest             string                    `json:"terminal_ledger_digest,omitempty"`
	HumanResolutionDigest            string                    `json:"human_resolution_digest,omitempty"`
	ConstructStatus                  stress.AdmissionStatus    `json:"construct_status"`
	OriginalEventGraphDigest         string                    `json:"original_event_graph_digest"`
	TransformedEventGraphDigest      string                    `json:"transformed_event_graph_digest"`
	Admissibility                    InterventionAdmissibility `json:"admissibility"`
	AdmissibilityReasons             []InterventionFailureCode `json:"admissibility_reasons"`
	PrimaryEligible                  bool                      `json:"primary_eligible"`
	SensitivityEligible              bool                      `json:"sensitivity_eligible"`
	AssignmentFrozenBeforeOutput     bool                      `json:"assignment_frozen_before_output"`
	ProviderCalls                    int                       `json:"provider_calls"`
	NetworkRequired                  bool                      `json:"network_required"`
	Digest                           string                    `json:"digest"`
}
