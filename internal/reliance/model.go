// Package reliance defines frozen, auditable interventions over observable
// canonical trajectory evidence. It measures verifier-output movement only.
package reliance

import "github.com/Christopher-Schulze/evalwitness/internal/preprocess"

const (
	CanonicalPolicy                 = "evalwitness.reliance-canonical-json.v1"
	FactorOntologySchemaVersion     = "evalwitness.evidence-factor-ontology.v1"
	FactorAssignmentSchemaVersion   = "evalwitness.evidence-factor-assignment.v1"
	EstimandCatalogSchemaVersion    = "evalwitness.reliance-estimand-catalog.v1"
	PreregistrationSchemaVersion    = "evalwitness.reliance-preregistration.v1"
	FactorOntologyVersion           = "evalwitness.coding-evidence-factors.v1"
	AssignmentFreezeStagePreExecute = "pre_verifier_execution"
)

type FactorID string

const (
	FactorCommandExit         FactorID = "command_exit"
	FactorErrorOutput         FactorID = "error_output"
	FactorExecutableOutcome   FactorID = "executable_outcome"
	FactorIrrelevantVerbosity FactorID = "irrelevant_verbosity"
	FactorMetadata            FactorID = "metadata"
	FactorPatchEdit           FactorID = "patch_edit"
	FactorPromptInjection     FactorID = "prompt_injection"
	FactorSuccessFailureProse FactorID = "success_failure_prose"
	FactorTestResult          FactorID = "test_result"
	FactorToolOutput          FactorID = "tool_output"
)

type FactorFamily string

const (
	FamilyAdversarial       FactorFamily = "adversarial_content"
	FamilyErrorToolOutput   FactorFamily = "error_and_tool_output"
	FamilyExecutableOutcome FactorFamily = "executable_outcome"
	FamilyMetadataFormat    FactorFamily = "metadata_and_format"
	FamilyNarrative         FactorFamily = "narrative"
	FamilyPatchEdit         FactorFamily = "patch_and_edit"
	FamilyVerbosity         FactorFamily = "verbosity"
)

type FieldValueKind string

const (
	ValueBoolean      FieldValueKind = "boolean"
	ValueContentParts FieldValueKind = "content_parts"
	ValueInteger      FieldValueKind = "integer"
	ValueText         FieldValueKind = "text"
)

type InterventionOperator string

const (
	OperatorControlledReplacement InterventionOperator = "controlled_replacement"
	OperatorRemove                InterventionOperator = "remove"
	OperatorRetain                InterventionOperator = "retain"
	OperatorTypedMask             InterventionOperator = "typed_mask"
)

type AssignmentMethod string

const (
	AssignmentBlindedHuman        AssignmentMethod = "blinded_human"
	AssignmentDeclaredAdversarial AssignmentMethod = "declared_adversarial"
	AssignmentFrozenFixture       AssignmentMethod = "frozen_fixture"
	AssignmentStructuralRule      AssignmentMethod = "structural_rule"
)

type EstimandFamily string

const (
	EstimandEvidenceOnly    EstimandFamily = "evidence_only"
	EstimandQualityChanging EstimandFamily = "quality_changing"
)

type QualityCondition string

const (
	QualityChanged   QualityCondition = "changed"
	QualityPreserved QualityCondition = "preserved"
)

type OutcomeID string

const (
	OutcomeAbstentionTransition OutcomeID = "abstention_transition"
	OutcomeConditionalMean      OutcomeID = "conditional_expected_score"
	OutcomeConditionalVariance  OutcomeID = "conditional_variance"
	OutcomeDecisionFlip         OutcomeID = "decision_flip"
	OutcomeSupportJaccard       OutcomeID = "support_jaccard"
	OutcomeValidMass            OutcomeID = "valid_score_mass"
	OutcomeVisibleMass          OutcomeID = "visible_probability_mass"
)

type OutcomeScale string

const (
	ScaleBinaryTransition OutcomeScale = "binary_transition"
	ScaleContinuous       OutcomeScale = "continuous"
)

type FieldTarget struct {
	EventKind preprocess.EventKind `json:"event_kind"`
	FieldPath preprocess.FieldPath `json:"field_path"`
	ValueKind FieldValueKind       `json:"value_kind"`
	Optional  bool                 `json:"optional"`
}

type EvidenceFactor struct {
	FactorID       FactorID      `json:"factor_id"`
	Family         FactorFamily  `json:"family"`
	Description    string        `json:"description"`
	AllowedTargets []FieldTarget `json:"allowed_targets"`
}

type OperatorDefinition struct {
	Operator            InterventionOperator `json:"operator"`
	ChangesEvidence     bool                 `json:"changes_evidence"`
	RequiresReplacement bool                 `json:"requires_replacement"`
	AllowedTargets      []FieldTarget        `json:"allowed_targets"`
}

type FactorOntology struct {
	SchemaVersion             string               `json:"schema_version"`
	CanonicalPolicy           string               `json:"canonical_policy"`
	OntologyVersion           string               `json:"ontology_version"`
	Factors                   []EvidenceFactor     `json:"factors"`
	Operators                 []OperatorDefinition `json:"operators"`
	AmbiguityRules            []string             `json:"ambiguity_rules"`
	IdentificationAssumptions []string             `json:"identification_assumptions"`
	Digest                    string               `json:"digest"`
}

type EstimandDefinition struct {
	Family            EstimandFamily   `json:"family"`
	Interpretation    string           `json:"interpretation"`
	PrimaryUnit       string           `json:"primary_unit"`
	QualityCondition  QualityCondition `json:"quality_condition"`
	RequiredEvidence  []string         `json:"required_evidence"`
	AllowedClaims     []string         `json:"allowed_claims"`
	ForbiddenClaims   []string         `json:"forbidden_claims"`
	DenominatorPolicy string           `json:"denominator_policy"`
	ResultTableID     string           `json:"result_table_id"`
}

type EstimandCatalog struct {
	SchemaVersion   string               `json:"schema_version"`
	CanonicalPolicy string               `json:"canonical_policy"`
	Definitions     []EstimandDefinition `json:"definitions"`
	Digest          string               `json:"digest"`
}

type InteractionDefinition struct {
	InteractionID string   `json:"interaction_id"`
	Terms         []string `json:"terms"`
}

type OutcomeDefinition struct {
	OutcomeID   OutcomeID    `json:"outcome_id"`
	Scale       OutcomeScale `json:"scale"`
	EstimatorID string       `json:"estimator_id"`
	Primary     bool         `json:"primary"`
}

type RandomizationPlan struct {
	Algorithm        string   `json:"algorithm"`
	Unit             string   `json:"unit"`
	Balance          string   `json:"balance"`
	SeedSource       string   `json:"seed_source"`
	AssignmentTiming string   `json:"assignment_timing"`
	Blocking         []string `json:"blocking"`
}

type MultiplicityPlan struct {
	Method             string `json:"method"`
	FamilyConstruction string `json:"family_construction"`
	FamilySize         int    `json:"family_size"`
}

type MissingnessPlan struct {
	DenominatorPolicy        string `json:"denominator_policy"`
	Imputation               string `json:"imputation"`
	InvalidCaseTreatment     string `json:"invalid_case_treatment"`
	IncompletePairTreatment  string `json:"incomplete_pair_treatment"`
	UnsupportedCellTreatment string `json:"unsupported_cell_treatment"`
}

type StoppingPlan struct {
	Method                    string `json:"method"`
	SequentialLooks           int    `json:"sequential_looks"`
	OutcomeDependent          bool   `json:"outcome_dependent"`
	LiveFallback              bool   `json:"live_fallback"`
	AdaptiveScreeningDataRole string `json:"adaptive_screening_data_role"`
}

type Preregistration struct {
	SchemaVersion              string                  `json:"schema_version"`
	CanonicalPolicy            string                  `json:"canonical_policy"`
	StudySchemaVersion         string                  `json:"study_schema_version"`
	StudyKind                  string                  `json:"study_kind"`
	OntologyDigest             string                  `json:"ontology_digest"`
	EstimandCatalogDigest      string                  `json:"estimand_catalog_digest"`
	MainEffects                []FactorID              `json:"main_effects"`
	Interactions               []InteractionDefinition `json:"interactions"`
	PrimaryOutcomes            []OutcomeDefinition     `json:"primary_outcomes"`
	Randomization              RandomizationPlan       `json:"randomization"`
	PreRandomizationExclusions []string                `json:"pre_randomization_exclusions"`
	RetainedPostRandomization  []string                `json:"retained_post_randomization_statuses"`
	Multiplicity               MultiplicityPlan        `json:"multiplicity"`
	Missingness                MissingnessPlan         `json:"missingness"`
	Stopping                   StoppingPlan            `json:"stopping"`
	Digest                     string                  `json:"digest"`
}

type AssignmentRequest struct {
	EventID   string
	FieldPath preprocess.FieldPath
	FactorID  FactorID
	Method    AssignmentMethod
	Rationale string
}

type AssignmentTarget struct {
	EventID   string               `json:"event_id"`
	FieldPath preprocess.FieldPath `json:"field_path"`
}

type EvidenceAssignment struct {
	AssignmentID string               `json:"assignment_id"`
	EventID      string               `json:"event_id"`
	EventKind    preprocess.EventKind `json:"event_kind"`
	FieldPath    preprocess.FieldPath `json:"field_path"`
	FactorID     FactorID             `json:"factor_id"`
	ValueDigest  string               `json:"value_digest"`
	Method       AssignmentMethod     `json:"method"`
	Rationale    string               `json:"rationale"`
}

type FactorAssignmentSet struct {
	SchemaVersion    string               `json:"schema_version"`
	CanonicalPolicy  string               `json:"canonical_policy"`
	OntologyDigest   string               `json:"ontology_digest"`
	TrajectoryDigest string               `json:"trajectory_digest"`
	FreezeStage      string               `json:"freeze_stage"`
	Assignments      []EvidenceAssignment `json:"assignments"`
	Digest           string               `json:"digest"`
}
