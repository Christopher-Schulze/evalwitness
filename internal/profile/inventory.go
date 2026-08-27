package profile

// CandidateMetric maps a candidate metric to its scientific unit and evidence.
type CandidateMetric struct {
	Task         string `json:"task"`
	Metric       string `json:"metric"`
	Unit         string `json:"unit"`
	Scope        string `json:"scope"`
	Source       string `json:"source"`
	FailureState Status `json:"failure_state"`
}

// Inventory034_065 returns all outputs from TASK 034 through TASK 065 mapped to units.
func Inventory034_065() []CandidateMetric {
	return []CandidateMetric{
		{Task: "034", Metric: "ece", Unit: "probability", Scope: "terminal", Source: "reliability.Report", FailureState: StatusFailed},
		{Task: "034", Metric: "mce", Unit: "probability", Scope: "terminal", Source: "reliability.Report", FailureState: StatusFailed},
		{Task: "034", Metric: "brier", Unit: "probability", Scope: "terminal", Source: "reliability.Report", FailureState: StatusFailed},
		{Task: "034", Metric: "auc", Unit: "rank", Scope: "terminal", Source: "reliability.Report", FailureState: StatusUnsupported},
		{Task: "048", Metric: "selective_risk", Unit: "risk", Scope: "terminal", Source: "calibration.SelectiveMetrics", FailureState: StatusFailed},
		{Task: "048", Metric: "coverage", Unit: "fraction", Scope: "terminal", Source: "calibration.SelectiveMetrics", FailureState: StatusFailed},
		{Task: "048", Metric: "aurc", Unit: "risk_coverage", Scope: "terminal", Source: "calibration.SelectiveMetrics", FailureState: StatusFailed},
		{Task: "048", Metric: "excess_aurc", Unit: "risk_coverage", Scope: "terminal", Source: "calibration.SelectiveMetrics", FailureState: StatusUnsupported},
		{Task: "065", Metric: "evidence_reliance", Unit: "effect", Scope: "terminal", Source: "reliance.Report", FailureState: StatusNotMeasured},
		{Task: "065", Metric: "slicing_alignment", Unit: "fraction", Scope: "terminal", Source: "reliance.Report", FailureState: StatusNotMeasured},
	}
}
