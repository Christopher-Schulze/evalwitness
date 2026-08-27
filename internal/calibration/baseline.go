package calibration

// UncalibratedPolicy returns raw conditional diff logistic without fit.
type UncalibratedPolicy struct{}

// Estimate returns predicted directly.
func (UncalibratedPolicy) Estimate(o Observation) float64 { return o.Predicted }

// Calibrate is identity.
func (UncalibratedPolicy) Calibrate(p float64) float64 { return p }

// LegacyPolicy is the old fixed threshold logic retained for comparison.
type LegacyPolicy struct {
	Threshold float64
}

// Estimate returns predicted with legacy threshold semantics.
func (l LegacyPolicy) Estimate(o Observation) float64 { return o.Predicted }

// Decides via threshold comparison, preserved for benchmark.
func (l LegacyPolicy) Decide(o Observation) bool { return o.Predicted >= l.Threshold }
