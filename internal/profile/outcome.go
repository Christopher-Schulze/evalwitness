package profile

// OutcomeDimension keeps agreement, rubric-ambiguity, leakage, reconciliation disaggregated.
type OutcomeDimension struct {
	Agreement            string `json:"agreement"`
	RubricAmbiguity      string `json:"rubric_ambiguity"`
	SourceLeakage        string `json:"source_leakage"`
	SourceReconciliation string `json:"source_reconciliation"`
	BenchmarkTransition  string `json:"benchmark_transition"`
	Status               Status `json:"status"`
}

// OutcomeDimensions returns TASK 057 outcome-validity dimensions.
func OutcomeDimensions() []OutcomeDimension {
	return []OutcomeDimension{
		{Agreement: "dual", RubricAmbiguity: "Wilson", SourceLeakage: "blinded", SourceReconciliation: "post-ledger", BenchmarkTransition: "original_vs_adjudicated", Status: StatusNotMeasured},
	}
}
