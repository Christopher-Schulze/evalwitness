package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

// TestMeasureRetentionOnLiveTerminalBenchTraces runs MeasureRetention against
// real Terminal-Bench trajectory files to measure per-kind evidence survival
// at 16k, 32k, and 64k budgets on live transcript formats.
func TestMeasureRetentionOnLiveTerminalBenchTraces(t *testing.T) {
	if testing.Short() {
		t.Skip("retention live-format measurement")
	}
	trajsDir := filepath.Join("..", "..", "eval", "trajectories", "terminal_trajs", "forge_gpt54")
	entries, err := os.ReadDir(trajsDir)
	if err != nil {
		t.Skipf("trajectory dir missing: %v", err)
	}

	budgets := []int{16000, 32000, 64000}
	measured := 0
	type kindStats struct {
		original         int
		retainedByBudget map[int]int
	}
	kindStatsByKind := map[string]*kindStats{}

	for _, taskEntry := range entries {
		if !taskEntry.IsDir() || measured >= 10 {
			continue
		}
		taskDir := filepath.Join(trajsDir, taskEntry.Name())
		files, err := os.ReadDir(taskDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), "_trajectory.json") || measured >= 10 {
				continue
			}
			path := filepath.Join(trajsDir, taskEntry.Name(), f.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			result, err := preprocess.ImportTraceBytes(raw, preprocess.DefaultTraceImportOptions())
			if err != nil {
				t.Logf("skip %s: %v", f.Name(), err)
				continue
			}

			points, err := MeasureRetention(result.Trajectory, budgets)
			if err != nil {
				t.Logf("retention failed for %s: %v", f.Name(), err)
				continue
			}

			for _, point := range points {
				kind := fmt.Sprintf("%v", point.Category)
				if kindStatsByKind[kind] == nil {
					kindStatsByKind[kind] = &kindStats{retainedByBudget: map[int]int{}}
				}
				ks := kindStatsByKind[kind]
				ks.original += point.Original
				ks.retainedByBudget[point.Budget] += point.Retained
			}
			measured++
		}
	}

	if measured == 0 {
		t.Skip("no trajectories successfully measured")
	}

	t.Logf("measured %d trajectories across %d event kinds", measured, len(kindStatsByKind))
	for kind, ks := range kindStatsByKind {
		parts := []string{}
		for _, budget := range budgets {
			if r, ok := ks.retainedByBudget[budget]; ok {
				pct := float64(r) / float64(ks.original) * 100
				parts = append(parts, fmt.Sprintf("%dk=%d (%.1f%%)", budget/1000, r, pct))
			}
		}
		t.Logf("  %s: original=%d [%s]", kind, ks.original, strings.Join(parts, ", "))
	}

	if len(kindStatsByKind) == 0 {
		t.Fatal("no event kinds measured")
	}
}
