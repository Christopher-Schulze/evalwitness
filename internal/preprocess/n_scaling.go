package preprocess

// NScalingResult reports selection scaling across N. Calls and coverage are structural projections (no outcome data).
// SelectiveRisk is omitted until real outcome data exists (eval/results/swebench-all-pairs.json simulation).
type NScalingResult struct {
	N        int     `json:"n"`
	Calls    int     `json:"calls"`
	Coverage float64 `json:"coverage"`
}

// MeasureNScaling computes deterministic N-scaling projection for tournament selection.
// For each N, calls = N-1 (single elimination), coverage = 1 or total/N. No selective risk claim.
func MeasureNScaling(ns []int, totalCandidates int) []NScalingResult {
	var out []NScalingResult
	for _, n := range ns {
		calls := n - 1
		if calls < 0 {
			calls = 0
		}
		coverage := 1.0
		if totalCandidates > 0 && n > totalCandidates {
			coverage = float64(totalCandidates) / float64(n)
		}
		out = append(out, NScalingResult{N: n, Calls: calls, Coverage: coverage})
	}
	return out
}

// SliceRetention reports fraction retained per budget per format.
type SliceRetention struct {
	Format   string  `json:"format"`
	Budget   int     `json:"budget"`
	Retained float64 `json:"retained"`
	Total    int     `json:"total"`
}

// MeasureSliceRetention computes retained/total for canonical trajectory at budgets.
func MeasureSliceRetention(traj Trajectory, budgets []int) []SliceRetention {
	var out []SliceRetention
	total := len(traj.Events)
	for _, b := range budgets {
		retainedTraj, err := ApplyEvidenceBudget(traj, b)
		if err != nil {
			out = append(out, SliceRetention{Format: string(traj.SourceFormat), Budget: b, Retained: 0, Total: total})
			continue
		}
		retained := float64(len(retainedTraj.Events)) / float64(total)
		if total == 0 {
			retained = 0
		}
		out = append(out, SliceRetention{Format: string(traj.SourceFormat), Budget: b, Retained: retained, Total: total})
	}
	return out
}
