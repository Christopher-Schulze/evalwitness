package reliance

import (
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	EvidenceSelectorAuditSchemaVersion = "evalwitness.evidence-selector-audit.v1"
	EvidenceSelectorAuditPolicyVersion = "evalwitness.evidence-selector-audit-policy.v1"
	SelectorEffectDetectionRule        = "any_declared_term_outcome_adjusted_p_lte_alpha"
	LegacyLineSelectorStatus           = "legacy_pipeline_only_not_production_verifier_path"
)

type SelectorEffectStatus string

const (
	SelectorEffectDetected     SelectorEffectStatus = "adjusted_effect_detected"
	SelectorEffectNotDetected  SelectorEffectStatus = "no_adjusted_effect_detected"
	SelectorEffectInconclusive SelectorEffectStatus = "analysis_inconclusive"
)

type SelectorRiskFlag string

const (
	SelectorRiskDetectedEffectNonexact   SelectorRiskFlag = "adjusted_effect_detected_with_nonexact_or_unrendered_target"
	SelectorRiskUndetectedEffectRetained SelectorRiskFlag = "no_adjusted_effect_detected_with_rendered_target_retention"
	SelectorRiskInconclusive             SelectorRiskFlag = "analysis_inconclusive_no_selector_alignment_claim"
)

type EvidenceSelectorAuditSource struct {
	SourceTaskID  string
	Trajectory    preprocess.Trajectory
	Assignments   FactorAssignmentSet
	TreatmentPlan FactorTreatmentPlan
}

type EvidenceSelectorAuditParents struct {
	Ontology        FactorOntology
	Estimands       EstimandCatalog
	Preregistration Preregistration
	Preflight       ReliancePreflight
	Registration    ReliancePanelRegistration
	Executions      []EvidenceTaskPanelExecution
	Failures        []RelianceCellFailureReceipt
	Sources         []EvidenceSelectorAuditSource
}

type SelectorTermOutcomeAudit struct {
	TermID            string            `json:"term_id"`
	Factors           []string          `json:"factors"`
	OutcomeID         OutcomeID         `json:"outcome_id"`
	FitStatus         RelianceFitStatus `json:"fit_status"`
	EstimateAvailable bool              `json:"estimate_available"`
	Estimate          float64           `json:"estimate,omitempty"`
	Lower             float64           `json:"lower,omitempty"`
	Upper             float64           `json:"upper,omitempty"`
	AdjustedPValue    float64           `json:"adjusted_p_value,omitempty"`
}

type SelectorFactorBudgetAudit struct {
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

type SelectorFactorAudit struct {
	FactorID          FactorID                    `json:"factor_id"`
	AssignmentTargets int                         `json:"assignment_targets"`
	MinimumEventScore int                         `json:"minimum_event_score"`
	MaximumEventScore int                         `json:"maximum_event_score"`
	EffectStatus      SelectorEffectStatus        `json:"effect_status"`
	TermOutcomes      []SelectorTermOutcomeAudit  `json:"term_outcomes"`
	Budgets           []SelectorFactorBudgetAudit `json:"budgets"`
}

type SelectorCategoryBudgetAudit struct {
	BudgetTokens   int                  `json:"budget_tokens"`
	EventKind      preprocess.EventKind `json:"event_kind"`
	OriginalEvents int                  `json:"original_events"`
	RetainedEvents int                  `json:"retained_events"`
	OriginalBytes  int                  `json:"original_bytes"`
	RetainedBytes  int                  `json:"retained_bytes"`
}

type LegacyLineSelectorAudit struct {
	Status      string                                   `json:"status"`
	Selector    string                                   `json:"selector"`
	ScorePolicy string                                   `json:"score_policy"`
	Probes      []preprocess.EvidenceLineScoreInspection `json:"probes"`
}

type EvidenceSelectorAudit struct {
	SchemaVersion            string                                      `json:"schema_version"`
	CanonicalPolicy          string                                      `json:"canonical_policy"`
	PolicyVersion            string                                      `json:"policy_version"`
	RegistrationDigest       string                                      `json:"registration_digest"`
	AnalysisDigest           string                                      `json:"analysis_digest"`
	PreregistrationDigest    string                                      `json:"preregistration_digest"`
	PreflightDigest          string                                      `json:"preflight_digest"`
	SourceArtifactSetDigest  string                                      `json:"source_artifact_set_digest"`
	ProductionPolicy         preprocess.EvidenceSelectionPolicyInventory `json:"production_policy"`
	LegacyLinePolicy         LegacyLineSelectorAudit                     `json:"legacy_line_policy"`
	EffectDetectionRule      string                                      `json:"effect_detection_rule"`
	AdjustedEffectAlpha      float64                                     `json:"adjusted_effect_alpha"`
	Budgets                  []int                                       `json:"budgets"`
	SourceTasks              int                                         `json:"source_tasks"`
	AssignmentTargets        int                                         `json:"assignment_targets"`
	EventBytesAreNonAdditive bool                                        `json:"event_bytes_are_non_additive"`
	Factors                  []SelectorFactorAudit                       `json:"factors"`
	Categories               []SelectorCategoryBudgetAudit               `json:"categories"`
	ProviderCalls            int                                         `json:"provider_calls"`
	NetworkRequired          bool                                        `json:"network_required"`
	Digest                   string                                      `json:"digest"`
}
