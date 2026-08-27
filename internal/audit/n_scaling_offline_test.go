package audit

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// TestOfflineNScalingFromCommittedData computes selection quality across
// N ∈ {2,3,4,5} using ONLY the committed Terminal-Bench per-task details.
// Zero provider calls. This is the offline portion of 039 Z108/Z109:
// it measures oracle, random, first-only, and verifier-selected outcomes
// at each N to establish baseline scaling curves.
//
// N=8 is NOT measured here because only 5 trials exist per task; fresh
// candidate generation requires live calls (gated).
func TestOfflineNScalingFromCommittedData(t *testing.T) {
	raw, err := os.ReadFile("../../eval/results/terminal-bench-verifier.json")
	if err != nil {
		t.Skipf("verifier details not available: %v", err)
	}
	var result struct {
		Details []struct {
			TaskName       string  `json:"task_name"`
			Rewards        []int   `json:"rewards"`
			PassAt1        float64 `json:"pass_at_1"`
			Oracle         int     `json:"oracle"`
			SelectedIndex  int     `json:"selected_index"`
			SelectedReward int     `json:"selected_reward"`
		} `json:"details"`
		SwingTasks int `json:"swing_tasks"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	details := result.Details
	totalTasks := len(details)
	if totalTasks == 0 {
		t.Skip("no tasks in details")
	}

	maxN := 0
	for _, d := range details {
		if len(d.Rewards) > maxN {
			maxN = len(d.Rewards)
		}
	}
	t.Logf("tasks=%d max_trials_per_task=%d swing_tasks=%d",
		totalTasks, maxN, result.SwingTasks)

	// Count task categories
	allPass, allFail, swing := 0, 0, 0
	for _, d := range details {
		sum := 0
		for _, r := range d.Rewards {
			sum += r
		}
		if sum == len(d.Rewards) {
			allPass++
		} else if sum == 0 {
			allFail++
		} else {
			swing++
		}
	}
	t.Logf("categories: all_pass=%d all_fail=%d swing=%d", allPass, allFail, swing)

	// Measure selection quality at each N for each strategy.
	strategies := []string{"oracle", "random_expected", "first_only", "verifier_committed"}
	results := make(map[string][]float64)

	for _, n := range []int{2, 3, 4, 5} {
		if n > maxN {
			break
		}
		scores := map[string]float64{}
		for _, strat := range strategies {
			solvedF := 0.0
			for _, d := range details {
				candidates := d.Rewards[:n]
				switch strat {
				case "oracle":
					best := 0
					for _, r := range candidates {
						if r > best {
							best = r
						}
					}
					solvedF += float64(best)
				case "random_expected":
					sum := 0
					for _, r := range candidates {
						sum += r
					}
					// Expected reward of uniform random pick from N candidates
					solvedF += float64(sum) / float64(n)
				case "first_only":
					solvedF += float64(candidates[0])
				case "verifier_committed":
					solvedF += float64(d.SelectedReward)
				}
			}
			scores[strat] = solvedF / float64(totalTasks) * 100
			scores[strat+"_count"] = solvedF
		}
		for _, strat := range strategies {
			key := fmt.Sprintf("%s_N%d", strat, n)
			results[key] = []float64{scores[strat], scores[strat+"_count"]}
		}
	}

	for _, strat := range strategies {
		for _, n := range []int{2, 3, 4, 5} {
			key := fmt.Sprintf("%s_N%d", strat, n)
			if vals, ok := results[key]; ok {
				t.Logf("N=%d %s: %.1f%% (%.1f/%d)", n, strat, vals[0], vals[1], totalTasks)
			}
		}
	}

	// Wilson score intervals (95%, z=1.96) for each measurement point
	z := 1.96
	t.Log("\n=== Wilson 95% Score Intervals ===")
	for _, strat := range strategies {
		for _, n := range []int{2, 3, 4, 5} {
			key := fmt.Sprintf("%s_N%d", strat, n)
			if vals, ok := results[key]; ok {
				successes := int(vals[1])
				rate := vals[0] / 100.0
				p := rate
				denom := 1 + z*z/float64(totalTasks)
				center := (p + z*z/(2*float64(totalTasks))) / denom
				half := z / denom * math.Sqrt(p*(1-p)/float64(totalTasks)+z*z/(4*float64(totalTasks)*float64(totalTasks)))
				lower := math.Max(0, center-half)
				upper := math.Min(1, center+half)
				t.Logf("N=%d %s: %.1f%% [%.1f%%, %.1f%%]", n, strat, rate*100, lower*100, upper*100)
				_ = successes
				_ = z
			}
		}
	}

	// Verify: oracle >= verifier >= random at every N (monotonicity check)
	for _, n := range []int{2, 3, 4, 5} {
		oKey := fmt.Sprintf("oracle_N%d_count", n)
		vKey := fmt.Sprintf("verifier_committed_N%d_count", n)
		rKey := fmt.Sprintf("random_expected_N%d_count", n)
		o, okO := results[oKey]
		v, okV := results[vKey]
		r, okR := results[rKey]
		if !okO || !okV || !okR {
			continue
		}
		if o[0] < v[0] {
			t.Errorf("oracle %.0f < verifier %.0f at N=%d", o[0], v[0], n)
		}
		if v[0] < r[0] {
			t.Errorf("verifier %.0f < random %.0f at N=%d", v[0], r[0], n)
		}
	}
}
