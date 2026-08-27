package reliance

const RelianceExplorerProjectionSchemaVersion = "evalwitness.reliance-explorer-projection.v1"

type RelianceExplorerArtifactRef struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	SchemaVersion  string `json:"schema_version"`
	PayloadSHA256  string `json:"payload_sha256"`
	ArtifactDigest string `json:"artifact_digest"`
}

type RelianceExplorerEstimate struct {
	Estimate       string `json:"estimate"`
	StandardError  string `json:"standard_error"`
	Lower          string `json:"lower"`
	Upper          string `json:"upper"`
	AdjustedPValue string `json:"adjusted_p_value"`
}

type RelianceExplorerOutcome struct {
	DimensionID          string                    `json:"dimension_id"`
	MapTermID            string                    `json:"map_term_id"`
	TermKind             RelianceMapTermKind       `json:"term_kind"`
	Factors              []string                  `json:"factors"`
	OutcomeID            OutcomeID                 `json:"outcome_id"`
	Status               string                    `json:"status"`
	Numerator            int                       `json:"numerator"`
	Denominator          int                       `json:"denominator"`
	InvalidCells         int                       `json:"invalid_cells"`
	Estimate             *RelianceExplorerEstimate `json:"estimate,omitempty"`
	Reason               string                    `json:"reason,omitempty"`
	Unit                 string                    `json:"unit"`
	Interval             string                    `json:"interval"`
	Policy               string                    `json:"policy"`
	Route                string                    `json:"route"`
	Domain               string                    `json:"domain"`
	DataRole             string                    `json:"data_role"`
	EvidenceLevel        string                    `json:"evidence_level"`
	CapsuleID            string                    `json:"capsule_id"`
	CapsuleExpression    string                    `json:"capsule_expression"`
	SourceAnalysisDigest string                    `json:"source_analysis_digest"`
	ClaimBoundary        string                    `json:"claim_boundary"`
}

type RelianceExplorerSelectorBudget struct {
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

type RelianceExplorerSelector struct {
	TermID            string                           `json:"term_id"`
	FactorID          string                           `json:"factor_id"`
	EffectStatus      SelectorEffectStatus             `json:"effect_status"`
	AssignmentTargets int                              `json:"assignment_targets"`
	MinimumEventScore int                              `json:"minimum_event_score"`
	MaximumEventScore int                              `json:"maximum_event_score"`
	Budgets           []RelianceExplorerSelectorBudget `json:"budgets"`
}

type RelianceExplorerArmContrast struct {
	ComparisonDigest  string                     `json:"comparison_digest"`
	ContrastID        string                     `json:"contrast_id"`
	Kind              RelianceArmContrastKind    `json:"kind"`
	ReferenceArmID    string                     `json:"reference_arm_id"`
	ComparatorArmID   string                     `json:"comparator_arm_id"`
	Direction         string                     `json:"direction"`
	ChangedDimensions []string                   `json:"changed_dimensions"`
	Support           RelianceArmContrastSupport `json:"support"`
	Reason            string                     `json:"reason,omitempty"`
	Numerator         int                        `json:"numerator"`
	Denominator       int                        `json:"denominator"`
	InvalidPairs      int                        `json:"invalid_pairs"`
}

type RelianceExplorerWitness struct {
	CaseID                    string `json:"case_id"`
	RelationDigest            string `json:"relation_digest"`
	WitnessDigest             string `json:"witness_digest"`
	FinalEvaluationDigest     string `json:"final_evaluation_digest"`
	SourceAnalysisDigest      string `json:"source_analysis_digest"`
	BindingStatus             string `json:"binding_status"`
	OriginalInputDigest       string `json:"original_input_digest"`
	ReducedInputDigest        string `json:"reduced_input_digest"`
	FinalUnits                int    `json:"final_units"`
	Evaluations               int    `json:"evaluations"`
	RawTrajectoryContentShown bool   `json:"raw_trajectory_content_shown"`
}

type RelianceExplorerProjection struct {
	SchemaVersion         string                        `json:"schema_version"`
	Availability          string                        `json:"availability"`
	BaseCapsuleID         string                        `json:"base_capsule_id"`
	CapsuleID             string                        `json:"capsule_id"`
	ManifestDigest        string                        `json:"manifest_digest"`
	MapDigest             string                        `json:"map_digest"`
	LedgerDigest          string                        `json:"ledger_digest"`
	ProfileDigest         string                        `json:"profile_digest"`
	PaperDigest           string                        `json:"paper_digest"`
	Source                RelianceExplorerArtifactRef   `json:"source"`
	Scope                 ReliancePublicationScope      `json:"scope"`
	SourceTasks           int                           `json:"source_tasks"`
	RegisteredCells       int                           `json:"registered_cells"`
	OutcomeBearingCells   int                           `json:"outcome_bearing_cells"`
	ExcludedCells         int                           `json:"excluded_cells"`
	CompletedLogicalCalls int                           `json:"completed_logical_calls"`
	CellStatusCounts      []RelianceCellStatusCount     `json:"cell_status_counts"`
	ProfileStatusCounts   []RelianceProfileStatusCount  `json:"profile_status_counts"`
	Outcomes              []RelianceExplorerOutcome     `json:"outcomes"`
	Selectors             []RelianceExplorerSelector    `json:"selectors"`
	ArmContrasts          []RelianceExplorerArmContrast `json:"arm_contrasts"`
	Witnesses             []RelianceExplorerWitness     `json:"witnesses"`
	AllowedClaims         []string                      `json:"allowed_claims"`
	ForbiddenClaims       []string                      `json:"forbidden_claims"`
	Limitations           []string                      `json:"limitations"`
	CurrentClaimIDs       []string                      `json:"current_claim_ids"`
	UnsupportedClaimIDs   []string                      `json:"unsupported_claim_ids"`
	GlobalScoreProhibited bool                          `json:"global_score_prohibited"`
	ProviderCalls         int                           `json:"provider_calls"`
	NetworkRequired       bool                          `json:"network_required"`
	Digest                string                        `json:"digest"`
}
