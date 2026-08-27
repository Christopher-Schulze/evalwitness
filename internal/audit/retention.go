package audit

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

// RetentionPoint is one budget's retained fraction for a single category.
type RetentionPoint struct {
	Budget   int     `json:"budget"`
	Category string  `json:"category"`
	Retained int     `json:"retained"`
	Original int     `json:"original"`
	Fraction float64 `json:"fraction"`
}

// MeasureRetention evaluates what fraction of each prioritised category
// survives at each budget. Budgets are hard evidence-token limits (16k,
// 32k, 64k per acceptance). The source trajectory is the live format's
// canonical ingest, not a benchmark-only trace.
func MeasureRetention(trajectory preprocess.Trajectory, budgets []int) ([]RetentionPoint, error) {
	if trajectory.Digest == "" {
		return nil, fmt.Errorf("retention: trajectory digest empty")
	}
	if len(budgets) == 0 {
		return nil, fmt.Errorf("retention: no budgets")
	}
	originalCounts := countByKind(trajectory.Events)
	var points []RetentionPoint
	for _, budget := range budgets {
		retained, err := preprocess.ApplyEvidenceBudget(trajectory, budget)
		if err != nil {
			return nil, fmt.Errorf("retention budget %d: %w", budget, err)
		}
		retainedCounts := countByKind(retained.Events)
		for kind, original := range originalCounts {
			retainedCount := retainedCounts[kind]
			fraction := 0.0
			if original > 0 {
				fraction = float64(retainedCount) / float64(original)
			}
			points = append(points, RetentionPoint{
				Budget:   budget,
				Category: string(kind),
				Retained: retainedCount,
				Original: original,
				Fraction: fraction,
			})
		}
	}
	return points, nil
}

func countByKind(events []preprocess.Event) map[preprocess.EventKind]int {
	counts := make(map[preprocess.EventKind]int)
	for _, event := range events {
		counts[event.Kind]++
	}
	return counts
}
