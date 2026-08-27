package drift

// Threshold is descriptive until TASK 049 locks a study.
type Threshold struct {
	Metric      string  `json:"metric"`
	Description string  `json:"description"`
	Value       float64 `json:"value"`
	Locked      bool    `json:"locked"`
}

// DescriptiveThresholds returns current descriptive thresholds; none are locked before study.
func DescriptiveThresholds() []Threshold {
	return []Threshold{
		{Metric: "capability_change", Description: "any capability field change", Value: 0, Locked: false},
		{Metric: "distribution_shift", Description: "visible mass shift >0.05", Value: 0.05, Locked: false},
		{Metric: "calibration_invalidation", Description: "calibration applicability fails", Value: 0, Locked: false},
	}
}
