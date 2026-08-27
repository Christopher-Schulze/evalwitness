package profile

// RelianceDimension is the TASK 065 evidence-reliance and slicing alignment dimension.
// It preserves factor/interaction scope, estimator, uncertainty, invalid denominator, and claim boundary
// without scalar aggregation.
type RelianceDimension struct {
	Factor             string `json:"factor"`
	Interaction        string `json:"interaction,omitempty"`
	Scope              string `json:"scope"`
	Estimator          string `json:"estimator"`
	Uncertainty        string `json:"uncertainty"`
	InvalidDenominator int    `json:"invalid_denominator"`
	ClaimBoundary      string `json:"claim_boundary"`
}

// SlicingAlignmentDimension is the slicing-policy alignment dimension.
type SlicingAlignmentDimension struct {
	Policy   string `json:"policy"`
	Measured string `json:"measured"`
	Status   Status `json:"status"`
}

// RelianceDimensions returns the two 065 dimensions.
func RelianceDimensions() ([]RelianceDimension, []SlicingAlignmentDimension) {
	return []RelianceDimension{
		{Factor: "evidence_reliance", Scope: "terminal", Estimator: "task_cluster_bootstrap", Uncertainty: "0.95 percentile", InvalidDenominator: 0, ClaimBoundary: "no universal explanation"},
	}, []SlicingAlignmentDimension{
		{Policy: "evidence_budget", Measured: "retained/total", Status: StatusMeasured},
	}
}
