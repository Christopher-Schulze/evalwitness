package calibration

import (
	"math"
	"sort"

	randv2 "math/rand/v2"

	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
)

// bootstrapReplicates and intervalLevel mirror reliability for determinism.
const (
	bootstrapReplicates = 2000
	intervalLevel       = 0.95
)

// clusterBootstrapInterval returns a task-cluster percentile interval for fn.
// Clusters are TaskID groups; resampling is with replacement, seeded deterministically.
func clusterBootstrapInterval(observations []Observation, fn func([]Observation) float64, seed uint64) *reliability.Interval {
	if len(observations) == 0 {
		return nil
	}
	clusters := groupByTask(observations)
	if len(clusters) < 2 {
		return nil
	}
	src := randv2.New(randv2.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	estimates := make([]float64, bootstrapReplicates)
	for i := range estimates {
		sample := resampleClusters(clusters, src)
		estimates[i] = fn(sample)
	}
	lower, upper := percentileInterval(estimates, intervalLevel)
	return &reliability.Interval{
		Lower:               lower,
		Upper:               upper,
		Level:               intervalLevel,
		Method:              "task_cluster_bootstrap_percentile",
		Replicates:          bootstrapReplicates,
		EffectiveReplicates: bootstrapReplicates,
		Clusters:            len(clusters),
	}
}

func groupByTask(obs []Observation) map[string][]Observation {
	m := make(map[string][]Observation, len(obs))
	for _, o := range obs {
		m[o.TaskID] = append(m[o.TaskID], o)
	}
	return m
}
func resampleClusters(clusters map[string][]Observation, rnd *randv2.Rand) []Observation {
	keys := make([]string, 0, len(clusters))
	for k := range clusters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []Observation
	for range keys {
		k := keys[rnd.IntN(len(keys))]
		out = append(out, clusters[k]...)
	}
	return out
}
func percentileInterval(values []float64, level float64) (float64, float64) {
	if len(values) == 0 {
		return math.NaN(), math.NaN()
	}
	// Filter NaNs — NaN breaks > comparisons and corrupts insertion sort,
	// and selectiveRisk can be NaN when bootstrap sample has selected==0.
	filtered := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == 0 {
		return math.NaN(), math.NaN()
	}
	tmp := append([]float64(nil), filtered...)
	for i := 1; i < len(tmp); i++ {
		j := i
		for j > 0 && tmp[j-1] > tmp[j] {
			tmp[j-1], tmp[j] = tmp[j], tmp[j-1]
			j--
		}
	}
	alpha := (1 - level) / 2
	loIdx := int(math.Floor(alpha * float64(len(tmp))))
	hiIdx := int(math.Ceil((1-alpha)*float64(len(tmp)))) - 1
	if loIdx < 0 {
		loIdx = 0
	}
	if hiIdx >= len(tmp) {
		hiIdx = len(tmp) - 1
	}
	return tmp[loIdx], tmp[hiIdx]
}
