package reliance

const (
	EvidenceRelianceMapSchemaVersion = "evalwitness.evidence-reliance-map.v1"
	ReliancePublicationPolicyVersion = "evalwitness.reliance-publication-policy.v1"
	RelianceLocalMechanismRole       = "local_mechanism"
	RelianceLocalMechanismDataRole   = "mechanism_fixture"
	RelianceLocalEvidenceLevel       = "E1"
	RelianceWitnessRelationBinding   = "relation_level_not_factorial_cell_attributed"
	RelianceProfileClaimBoundary     = "local_mechanism_only_no_transfer_or_agent_causality"
	RelianceInterventionFamily       = "evidence_only"
)

type RelianceMapStatus string

const (
	RelianceMapMeasured     RelianceMapStatus = "measured"
	RelianceMapAmbiguous    RelianceMapStatus = "ambiguous"
	RelianceMapUnsupported  RelianceMapStatus = "unsupported"
	RelianceMapInconclusive RelianceMapStatus = "inconclusive"
)

type RelianceMapTermKind string

const (
	RelianceMapMainEffect  RelianceMapTermKind = "main_effect"
	RelianceMapInteraction RelianceMapTermKind = "interaction"
)

type ReliancePublicationScope struct {
	EvidenceRole        string `json:"evidence_role"`
	DataRole            string `json:"data_role"`
	EvidenceLevel       string `json:"evidence_level"`
	Domain              string `json:"domain"`
	StudyManifestDigest string `json:"study_manifest_digest"`
	Entrypoint          string `json:"entrypoint"`
	CriterionID         string `json:"criterion_id"`
	ScoreTag            string `json:"score_tag"`
	EvidencePolicy      string `json:"evidence_policy"`
	ProviderID          string `json:"provider_id"`
	RouteID             string `json:"route_id"`
	RequestedModel      string `json:"requested_model"`
	Empirical           bool   `json:"empirical"`
}

type RelianceMapEstimate struct {
	Estimate       float64 `json:"estimate"`
	StandardError  float64 `json:"standard_error"`
	Lower          float64 `json:"lower"`
	Upper          float64 `json:"upper"`
	AdjustedPValue float64 `json:"adjusted_p_value"`
}

type RelianceMapTermOutcome struct {
	OutcomeID            OutcomeID            `json:"outcome_id"`
	Status               RelianceMapStatus    `json:"status"`
	RegisteredCells      int                  `json:"registered_cells"`
	EligibleObservations int                  `json:"eligible_observations"`
	ExcludedFromFit      int                  `json:"excluded_from_fit"`
	Reason               string               `json:"reason,omitempty"`
	Estimate             *RelianceMapEstimate `json:"estimate,omitempty"`
}

type RelianceMapSelectorBudget struct {
	BudgetTokens                  int                `json:"budget_tokens"`
	AssignmentTargets             int                `json:"assignment_targets"`
	ExactTargets                  int                `json:"exact_targets"`
	ChangedTargets                int                `json:"changed_targets"`
	UnrenderedTargets             int                `json:"unrendered_targets"`
	DroppedTargets                int                `json:"dropped_targets"`
	AssignedEventBytes            int                `json:"assigned_event_bytes"`
	RetainedAssignedEventBytes    int                `json:"retained_assigned_event_bytes"`
	AssignedRenderedBytes         int                `json:"assigned_rendered_bytes"`
	RetainedAssignedRenderedBytes int                `json:"retained_assigned_rendered_bytes"`
	RiskFlags                     []SelectorRiskFlag `json:"risk_flags"`
}

type RelianceMapSelector struct {
	EffectStatus      SelectorEffectStatus        `json:"effect_status"`
	AssignmentTargets int                         `json:"assignment_targets"`
	MinimumEventScore int                         `json:"minimum_event_score"`
	MaximumEventScore int                         `json:"maximum_event_score"`
	Budgets           []RelianceMapSelectorBudget `json:"budgets"`
}

type RelianceMapTerm struct {
	TermID   string                   `json:"term_id"`
	Kind     RelianceMapTermKind      `json:"kind"`
	Factors  []string                 `json:"factors"`
	Outcomes []RelianceMapTermOutcome `json:"outcomes"`
	Selector *RelianceMapSelector     `json:"selector,omitempty"`
}

type RelianceMapArmComparison struct {
	Comparison RelianceArmComparison `json:"comparison"`
}

type RelianceMapWitness struct {
	SourceAnalysisDigest string          `json:"source_analysis_digest"`
	BindingStatus        string          `json:"binding_status"`
	Witness              RelianceWitness `json:"witness"`
}

type RelianceProfileDimension struct {
	DimensionID          string                   `json:"dimension_id"`
	MapTermID            string                   `json:"map_term_id"`
	TermKind             RelianceMapTermKind      `json:"term_kind"`
	Factors              []string                 `json:"factors"`
	OutcomeID            OutcomeID                `json:"outcome_id"`
	Status               string                   `json:"status"`
	EvidenceLevel        string                   `json:"evidence_level"`
	Scope                ReliancePublicationScope `json:"scope"`
	RegisteredCells      int                      `json:"registered_cells"`
	EligibleObservations int                      `json:"eligible_observations"`
	ExcludedFromFit      int                      `json:"excluded_from_fit"`
	InvalidCells         int                      `json:"invalid_cells"`
	Estimate             *RelianceMapEstimate     `json:"estimate,omitempty"`
	Reason               string                   `json:"reason,omitempty"`
	SourceAnalysisDigest string                   `json:"source_analysis_digest"`
	Policy               string                   `json:"policy"`
	Unit                 string                   `json:"unit"`
	InterventionFamily   string                   `json:"intervention_family"`
	ClaimBoundary        string                   `json:"claim_boundary"`
	CapsuleExpression    string                   `json:"capsule_expression"`
	Caveats              []string                 `json:"caveats"`
}

type ReliancePaperRow struct {
	RowID                string                   `json:"row_id"`
	MapTermID            string                   `json:"map_term_id"`
	TermKind             RelianceMapTermKind      `json:"term_kind"`
	Factors              []string                 `json:"factors"`
	OutcomeID            OutcomeID                `json:"outcome_id"`
	Status               RelianceMapStatus        `json:"status"`
	Scope                ReliancePublicationScope `json:"scope"`
	RegisteredCells      int                      `json:"registered_cells"`
	EligibleObservations int                      `json:"eligible_observations"`
	ExcludedFromFit      int                      `json:"excluded_from_fit"`
	Estimate             *RelianceMapEstimate     `json:"estimate,omitempty"`
	Reason               string                   `json:"reason,omitempty"`
	SourceAnalysisDigest string                   `json:"source_analysis_digest"`
	CapsuleExpression    string                   `json:"capsule_expression"`
}

type EvidenceRelianceMap struct {
	SchemaVersion           string                     `json:"schema_version"`
	CanonicalPolicy         string                     `json:"canonical_policy"`
	PublicationPolicy       string                     `json:"publication_policy"`
	Scope                   ReliancePublicationScope   `json:"scope"`
	OntologyDigest          string                     `json:"ontology_digest"`
	EstimandCatalogDigest   string                     `json:"estimand_catalog_digest"`
	PreregistrationDigest   string                     `json:"preregistration_digest"`
	PreflightDigest         string                     `json:"preflight_digest"`
	RegistrationDigest      string                     `json:"registration_digest"`
	CorpusDigest            string                     `json:"corpus_digest"`
	AnalysisDigest          string                     `json:"analysis_digest"`
	SelectorAuditDigest     string                     `json:"selector_audit_digest"`
	SourceTasks             int                        `json:"source_tasks"`
	RegisteredCells         int                        `json:"registered_cells"`
	OutcomeBearingCells     int                        `json:"outcome_bearing_cells"`
	ExcludedCells           int                        `json:"excluded_cells"`
	CompletedLogicalCalls   int                        `json:"completed_logical_calls"`
	StatusCounts            []RelianceCellStatusCount  `json:"status_counts"`
	Terms                   []RelianceMapTerm          `json:"terms"`
	ArmComparisons          []RelianceMapArmComparison `json:"arm_comparisons"`
	Witnesses               []RelianceMapWitness       `json:"witnesses"`
	ProfileDimensions       []RelianceProfileDimension `json:"profile_dimensions"`
	PaperRows               []ReliancePaperRow         `json:"paper_rows"`
	PaperLimitations        []string                   `json:"paper_limitations"`
	AllowedClaims           []string                   `json:"allowed_claims"`
	ForbiddenClaims         []string                   `json:"forbidden_claims"`
	ProjectionProviderCalls int                        `json:"projection_provider_calls"`
	NetworkRequired         bool                       `json:"network_required"`
	Digest                  string                     `json:"digest"`
}

type RelianceArmComparisonEvidence struct {
	Comparison RelianceArmComparison
	Arms       []RelianceArmEvidence
	Specs      []RelianceArmContrastSpec
}

type RelianceWitnessPublicationEvidence struct {
	Witness   RelianceWitness
	Request   RelationExecutionRequest
	Execution RelationExecutionResult
}

type EvidenceRelianceMapRequest struct {
	SelectorParents EvidenceSelectorAuditParents
	SelectorAudit   EvidenceSelectorAudit
	ArmComparisons  []RelianceArmComparisonEvidence
	Witnesses       []RelianceWitnessPublicationEvidence
}
