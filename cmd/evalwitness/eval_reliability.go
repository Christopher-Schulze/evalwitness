package main

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

// evalReliabilityRows converts one task result into versioned descriptive
// source rows. Pairwise probabilities and absolute scores remain separate so a
// rank correlation can never masquerade as probability calibration.
func evalReliabilityRows(taskID string, rewards []int, selection *mode.Selection, pairCallLimit int) ([]reliability.Observation, []reliability.RankObservation) {
	if selection == nil {
		return nil, nil
	}
	if len(selection.PairDecisions) == 0 {
		return nil, absoluteReliabilityRows(taskID, rewards, selection)
	}

	rows := make([]reliability.Observation, 0, len(selection.PairDecisions))
	for _, decision := range selection.PairDecisions {
		i, j := decision.Pair[0], decision.Pair[1]
		if i < 0 || j < 0 || i >= len(rewards) || j >= len(rewards) || rewards[i] == rewards[j] {
			continue
		}
		rows = append(rows, reliability.Observation{
			ID:             fmt.Sprintf("%s:pair:%d:%d", taskID, i, j),
			TaskID:         taskID,
			Pair:           decision.Pair,
			ExtractionMode: selection.Usage.ExtractionMode,
			Predicted:      decision.WinProbability,
			Won:            rewards[i] > rewards[j],
			MeanDifference: decision.MeanDifference,
			ScoreMass:      decision.ScoreMass,
			Calls:          decision.Calls,
			PairCallLimit:  pairCallLimit,
			OrderPolicy:    decision.OrderPolicy,
			FirstOrder:     decision.FirstOrder,
			OutcomeID:      fmt.Sprintf("%s:reward:%d=%d:%d=%d", taskID, i, rewards[i], j, rewards[j]),
			Inconsistent:   decision.Inconsistent,
		})
	}
	return rows, nil
}

func absoluteReliabilityRows(taskID string, rewards []int, selection *mode.Selection) []reliability.RankObservation {
	if len(selection.Scores) != len(rewards) || !hasDifferentRewards(rewards) {
		return nil
	}
	rows := make([]reliability.RankObservation, len(rewards))
	for i, reward := range rewards {
		rows[i] = reliability.RankObservation{
			ID:             fmt.Sprintf("%s:trajectory:%d", taskID, i),
			TaskID:         taskID,
			Trajectory:     i,
			ExtractionMode: selection.Usage.ExtractionMode,
			Predicted:      selection.Scores[i],
			Actual:         float64(reward),
			OutcomeID:      fmt.Sprintf("%s:reward:%d=%d", taskID, i, reward),
		}
	}
	return rows
}

func hasDifferentRewards(rewards []int) bool {
	return stats.DecidableBinary(rewards)
}

func printReliabilitySummary(report *reliability.Report) {
	if report == nil {
		return
	}
	if report.Count > 0 {
		fmt.Printf("Reliability: schema=%s role=%s mode=%s pairs=%d tasks=%d%s\n",
			report.SchemaVersion, report.DataRole, report.ExtractionMode, report.Count,
			report.ClusterCount, lowSampleSuffix(report.LowSample))
		fmt.Printf("  ECE %.3f%s  Brier %.3f%s  AUC %.3f%s  accuracy %.1f%%%s\n",
			report.ECE, intervalSuffix(report.Metrics.ECE.Interval),
			report.Brier, intervalSuffix(report.Metrics.Brier.Interval),
			report.AUC, intervalSuffix(report.Metrics.AUC.Interval),
			report.Accuracy*100, intervalPercentSuffix(report.Metrics.Accuracy.Interval))
		fmt.Printf("  directional errors: %d across %d tasks", report.Errors.TotalWrong, report.Errors.TotalWrongTasks)
		for _, stratum := range report.Errors.Strata {
			fmt.Printf("  %s=%d", stratum.ID, stratum.Count)
		}
		fmt.Println()
	}
	if report.Absolute != nil {
		metric := report.Absolute.Spearman
		fmt.Printf("Absolute discrimination: Spearman %.3f%s  rows=%d%s\n",
			metric.Value, intervalSuffix(metric.Interval), metric.Count, lowSampleSuffix(metric.LowSample))
	}
}

func lowSampleSuffix(lowSample bool) string {
	if lowSample {
		return " [low sample]"
	}
	return ""
}

func intervalSuffix(interval *reliability.Interval) string {
	if interval == nil {
		return ""
	}
	return fmt.Sprintf(" [95%% %.3f, %.3f]", interval.Lower, interval.Upper)
}

func intervalPercentSuffix(interval *reliability.Interval) string {
	if interval == nil {
		return ""
	}
	return fmt.Sprintf(" [95%% %.1f%%, %.1f%%]", interval.Lower*100, interval.Upper*100)
}
