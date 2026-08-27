package profile

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/calibration"
	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
)

// CalibrationInput binds 048 held-out output to profile dimension without leakage.
type CalibrationInput struct {
	Report      calibration.ReportArtifact
	Reliability reliability.Report
}

// ToDimensions projects calibration and reliability into profile dimensions.
func ToDimensions(in CalibrationInput) ([]Dimension, error) {
	if in.Report.Digest == "" {
		return nil, fmt.Errorf("profile: calibration report digest empty")
	}
	metric := fmt.Sprintf("ece=%.3f", in.Reliability.ECE)
	d1 := Dimension{
		ID:            "calibration",
		Status:        StatusMeasured,
		Metric:        &metric,
		Scope:         in.Report.Lifecycle.SplitDigest[:8],
		EvidenceLevel: "E1",
		CapsuleExpr:   "calibration.metrics.ece",
		Denominator:   in.Report.ObservationCount,
		SampleUnit:    "task",
	}
	selMetric := fmt.Sprintf("risk=%.3f coverage=%.3f", in.Report.Selective.SelectiveRisk, in.Report.Selective.Coverage)
	d2 := Dimension{
		ID:            "selective_risk_coverage",
		Status:        StatusMeasured,
		Metric:        &selMetric,
		Scope:         in.Report.Lifecycle.SplitDigest[:8],
		EvidenceLevel: "E1",
		CapsuleExpr:   "calibration.selective",
		Denominator:   in.Report.TaskCount,
		SampleUnit:    "task",
	}
	return []Dimension{d1, d2}, nil
}
