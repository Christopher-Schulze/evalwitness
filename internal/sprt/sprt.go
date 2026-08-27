// Package sprt implements Wald's Sequential Probability Ratio Test for
// adaptive verification rep budgeting.
package sprt

import "math"

// Decision is the outcome of one SPRT step.
type Decision int

const (
	Continue Decision = 0
	AcceptH1 Decision = 1  // diffs come from positive-mean distribution (i wins)
	AcceptH0 Decision = -1 // diffs come from negative-mean distribution (j wins)
)

// Params for the test. Sigma is the per-rep score-diff stddev estimate.
// Epsilon is the half-magnitude of mu under H0/H1: H1 mean = +epsilon, H0 mean = -epsilon.
type Params struct {
	Alpha   float64
	Beta    float64
	Sigma   float64
	Epsilon float64
	MinReps int
	MaxReps int
}

// DefaultParams returns sensible defaults aligned with spec.
func DefaultParams() Params {
	return Params{
		Alpha:   0.05,
		Beta:    0.05,
		Sigma:   0.15,
		Epsilon: 0.05,
		MinReps: 2,
		MaxReps: 16,
	}
}

// Decide runs Wald's SPRT given the accumulated score differences (diffs[k] = s_i_k - s_j_k).
// Returns the decision and how many reps were observed when terminating.
func Decide(diffs []float64, p Params) (Decision, int) {
	n := len(diffs)
	if n >= p.MaxReps {
		return finalize(diffs, p)
	}
	if n < p.MinReps {
		return Continue, n
	}
	logA := math.Log((1.0 - p.Beta) / p.Alpha)
	logB := math.Log(p.Beta / (1.0 - p.Alpha))
	sum := 0.0
	for _, d := range diffs {
		sum += d
	}
	logLR := 2 * p.Epsilon * sum / (p.Sigma * p.Sigma)
	switch {
	case logLR >= logA:
		return AcceptH1, n
	case logLR <= logB:
		return AcceptH0, n
	}
	return Continue, n
}

func finalize(diffs []float64, p Params) (Decision, int) {
	if len(diffs) == 0 {
		return Continue, 0
	}
	sum := 0.0
	for _, d := range diffs {
		sum += d
	}
	switch {
	case sum > 0:
		return AcceptH1, len(diffs)
	case sum < 0:
		return AcceptH0, len(diffs)
	}
	return Continue, len(diffs)
}

// AbsoluteVarianceDecision adapts SPRT-style early stop for absolute-mode reps.
// Returns true when accumulated score variance (after MinReps) suggests
// further reps add little information.
func AbsoluteVarianceDecision(scores []float64, p Params) bool {
	n := len(scores)
	if n < p.MinReps {
		return false
	}
	if n >= p.MaxReps {
		return true
	}
	mean := 0.0
	for _, s := range scores {
		mean += s
	}
	mean /= float64(n)
	variance := 0.0
	for _, s := range scores {
		d := s - mean
		variance += d * d
	}
	variance /= float64(n)
	return variance < (p.Sigma * p.Sigma * 0.25)
}
