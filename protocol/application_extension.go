package protocol

const (
	ApplicationExtensionNamespace = "org.evalwitness.application"
	ApplicationInvocationSchema   = "evalwitness.protocol.application-invocation.v1"
	ApplicationResultSchema       = "evalwitness.protocol.application-result.v1"
)

type ApplicationCriterion struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ApplicationSPRT struct {
	Alpha   string `json:"alpha_decimal"`
	Beta    string `json:"beta_decimal"`
	Sigma   string `json:"sigma_decimal"`
	Epsilon string `json:"epsilon_decimal"`
	MinReps int    `json:"min_reps"`
	MaxReps int    `json:"max_reps"`
}

type ApplicationPolicy struct {
	Evidence            string          `json:"evidence"`
	NReps               int             `json:"n_reps"`
	Epsilon             string          `json:"epsilon_decimal"`
	BiasMitigation      string          `json:"bias_mitigation"`
	InconsistencyPolicy string          `json:"inconsistency_policy"`
	SelectionStrategy   string          `json:"selection_strategy"`
	SingleElimination   bool            `json:"single_elimination"`
	UseSPRT             bool            `json:"use_sprt"`
	SPRT                ApplicationSPRT `json:"sprt"`
	MaxWorkers          int             `json:"max_workers"`
	MaxPairCalls        int             `json:"max_pair_calls"`
	ConfidenceThreshold string          `json:"confidence_threshold_decimal"`
	CalibrationSigma    string          `json:"calibration_sigma_decimal"`
}

type ApplicationInvocation struct {
	SchemaVersion string                 `json:"schema_version"`
	Mode          string                 `json:"mode"`
	Task          string                 `json:"task"`
	Criteria      []ApplicationCriterion `json:"criteria"`
	Policy        ApplicationPolicy      `json:"policy"`
	Lineage       ApplicationLineage     `json:"lineage"`
}

type ApplicationLineage struct {
	AuditCaseID           string `json:"audit_case_id,omitempty"`
	TransformationID      string `json:"transformation_id,omitempty"`
	OutcomeEvidenceDigest string `json:"outcome_evidence_digest,omitempty"`
	ProfilePolicyDigest   string `json:"profile_policy_digest,omitempty"`
	CapsuleDigest         string `json:"capsule_digest,omitempty"`
	StudyCellID           string `json:"study_cell_id,omitempty"`
}

type ApplicationResult struct {
	SchemaVersion         string   `json:"schema_version"`
	RunFingerprint        string   `json:"run_fingerprint"`
	RequestSetFingerprint string   `json:"request_set_fingerprint"`
	RequestFingerprints   []string `json:"request_fingerprints"`
	DecisionUTF8Hex       string   `json:"decision_utf8_hex"`
	BudgetDigest          string   `json:"budget_digest"`
	TrajectoryDigests     []string `json:"trajectory_digests"`
}
