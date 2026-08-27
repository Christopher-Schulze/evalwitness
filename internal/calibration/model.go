package calibration

import "github.com/Christopher-Schulze/evalwitness/internal/reliability"

// SchemaVersion is the versioned held-out calibration artifact.
const SchemaVersion = "evalwitness.calibration.v1"

// Observation is one decision-time feature row for the held-out fit.
// Features are strictly TASK 044 score evidence available at decision time.
// TaskID, benchmark reward and post-selection outcome are forbidden.
type Observation struct {
	ID                    string    `json:"id"`
	TaskID                string    `json:"task_id"`    // kept for task-cluster bootstrap only, never a feature
	SplitRole             SplitRole `json:"split_role"` // TASK 049 role, must match ValidateSplit expectedRole
	ConditionalDiff       float64   `json:"conditional_diff"`
	MinValidMass          float64   `json:"min_valid_mass"`
	MeanValidMass         float64   `json:"mean_valid_mass"`
	VisibleMass           float64   `json:"visible_mass"`
	MissingMass           float64   `json:"missing_mass"`
	ConditionalVar        float64   `json:"conditional_variance"`
	OrderEffect           float64   `json:"order_effect"`
	RepeatDispersion      float64   `json:"repeat_dispersion"`
	SupportCount          int       `json:"support_count"`
	TopK                  int       `json:"top_k"`
	EvidenceBudget        int       `json:"evidence_budget"`
	Retention             float64   `json:"retention"`
	ExtractionDegradation string    `json:"extraction_degradation,omitempty"`
	Predicted             float64   `json:"predicted"`
	Won                   *bool     `json:"won,omitempty"`
}

// FeatureSchema lists allowed feature keys for the calibrator.
type FeatureSchema struct {
	Version string   `json:"version"`
	Keys    []string `json:"keys"`
}

// ModelArtifact is the versioned calibration fit bound to its training manifest.
type ModelArtifact struct {
	SchemaVersion    string               `json:"schema_version"`
	ModelType        string               `json:"model_type"` // platt, isotonic, uncalibrated, legacy
	FeatureSchema    FeatureSchema        `json:"feature_schema"`
	TrainingDigest   string               `json:"training_manifest_digest"`
	CalibratorDigest string               `json:"calibrator_digest"`
	RouteScope       string               `json:"route_scope"`
	DomainScope      string               `json:"domain_scope"`
	EvidencePolicy   string               `json:"evidence_policy"`
	PromptVersion    string               `json:"prompt_version"`
	TopKContract     int                  `json:"top_k_contract"`
	Metrics          *reliability.Metrics `json:"metrics,omitempty"`
}

// Applicability reports whether the calibrator may be used on a new decision.
type Applicability struct {
	Applicable    bool   `json:"applicable"`
	Reason        string `json:"reason,omitempty"`
	RouteScope    string `json:"route_scope"`
	DomainScope   string `json:"domain_scope"`
	PolicyVersion string `json:"policy_version"`
}

// SelectiveDecision is a policy output: select, abstain or fallback.
type SelectiveDecision struct {
	Select             bool           `json:"select"`
	AbstainReason      string         `json:"abstain_reason,omitempty"`
	EstimatedCorrect   float64        `json:"estimated_correct"`
	Threshold          float64        `json:"threshold"`
	CalibrationVersion string         `json:"calibration_version"`
	Applicability      Applicability  `json:"applicability"`
	Fallback           FallbackPolicy `json:"fallback"`
	FallbackAccounted  bool           `json:"fallback_accounted"`
}
