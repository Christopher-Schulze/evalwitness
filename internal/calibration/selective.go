package calibration

import (
	"math"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
)

// SelectiveMetrics are held-out selective risk, coverage, AURC.
type SelectiveMetrics struct {
	Coverage         float64               `json:"coverage"`
	SelectiveRisk    float64               `json:"selective_risk"`
	AURC             float64               `json:"aurc"`
	ExcessAURC       float64               `json:"excess_aurc"`
	CoverageAtRisk   map[string]float64    `json:"coverage_at_risk,omitempty"`
	IntervalRisk     *reliability.Interval `json:"interval_risk,omitempty"`
	IntervalCoverage *reliability.Interval `json:"interval_coverage,omitempty"`
}

// EvaluateSelectiveWithIntervals computes selective metrics plus task-cluster bootstrap intervals.
// seed controls bootstrap reproducibility; use fixed seed per task.
func EvaluateSelectiveWithIntervals(observations []Observation, threshold float64, estimate func(Observation) float64, seed uint64) SelectiveMetrics {
	m := EvaluateSelective(observations, threshold, estimate)
	if len(observations) == 0 {
		return m
	}
	m.IntervalCoverage = clusterBootstrapInterval(observations, func(sample []Observation) float64 {
		return coverageAtThreshold(sample, threshold, estimate)
	}, seed)
	m.IntervalRisk = clusterBootstrapInterval(observations, func(sample []Observation) float64 {
		return selectiveRiskAtThreshold(sample, threshold, estimate)
	}, seed+1)
	if m.IntervalRisk != nil && (math.IsNaN(m.IntervalRisk.Lower) || math.IsNaN(m.IntervalRisk.Upper)) {
		m.IntervalRisk = nil
	}
	return m
}

func coverageAtThreshold(observations []Observation, threshold float64, estimate func(Observation) float64) float64 {
	if len(observations) == 0 {
		return math.NaN()
	}
	selected := 0
	for _, o := range observations {
		if o.Won == nil {
			continue
		}
		if estimate(o) >= threshold {
			selected++
		}
	}
	return float64(selected) / float64(len(observations))
}

func selectiveRiskAtThreshold(observations []Observation, threshold float64, estimate func(Observation) float64) float64 {
	selected := 0
	errors := 0
	for _, o := range observations {
		if o.Won == nil {
			continue
		}
		if estimate(o) >= threshold {
			selected++
			if !*o.Won {
				errors++
			}
		}
	}
	if selected == 0 {
		return math.NaN()
	}
	return float64(errors) / float64(selected)
}

// EvaluateSelective computes selective metrics for a threshold.
// Observations must be test split only; abstentions, invalid and provider failures are not dropped.
func EvaluateSelective(observations []Observation, threshold float64, estimate func(Observation) float64) SelectiveMetrics {
	if len(observations) == 0 {
		return SelectiveMetrics{}
	}
	type row struct {
		obs     Observation
		est     float64
		correct bool
	}
	var rows []row
	for _, o := range observations {
		if o.Won == nil {
			continue
		}
		est := estimate(o)
		rows = append(rows, row{obs: o, est: est, correct: *o.Won})
	}
	if len(rows) == 0 {
		return SelectiveMetrics{}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].est > rows[j].est })

	selected := 0
	errors := 0
	for _, r := range rows {
		if r.est >= threshold {
			selected++
			if !r.correct {
				errors++
			}
		}
	}
	coverage := float64(selected) / float64(len(observations))
	selectiveRisk := math.NaN()
	if selected > 0 {
		selectiveRisk = float64(errors) / float64(selected)
	}
	var aurc float64
	prevCoverage := 0.0
	prevRisk := 0.0
	for i := range rows {
		cov := float64(i+1) / float64(len(rows))
		errCount := 0
		for j := 0; j <= i; j++ {
			if !rows[j].correct {
				errCount++
			}
		}
		risk := float64(errCount) / float64(i+1)
		aurc += (cov - prevCoverage) * (risk + prevRisk) / 2
		prevCoverage = cov
		prevRisk = risk
	}
	correctCount := 0
	for _, r := range rows {
		if r.correct {
			correctCount++
		}
	}
	p := float64(correctCount) / float64(len(rows))
	var oracleAURC float64
	switch {
	case p <= 0:
		oracleAURC = 1
	case p >= 1:
		oracleAURC = 0
	default:
		oracleAURC = 1 - p + p*math.Log(p)
	}
	excess := aurc - oracleAURC
	if excess < 0 && excess > -1e-12 {
		excess = 0
	}

	return SelectiveMetrics{
		Coverage:      coverage,
		SelectiveRisk: selectiveRisk,
		AURC:          aurc,
		ExcessAURC:    excess,
	}
}

// IsDeployable returns true if upper bound risk <= target and coverage >= min.
func IsDeployable(m SelectiveMetrics, targetRisk, minCoverage float64) bool {
	if m.IntervalRisk != nil {
		if m.IntervalRisk.Upper > targetRisk {
			return false
		}
	} else {
		if math.IsNaN(m.SelectiveRisk) || m.SelectiveRisk > targetRisk {
			return false
		}
	}
	if m.Coverage < minCoverage {
		return false
	}
	return true
}
