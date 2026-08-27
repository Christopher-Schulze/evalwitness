package preprocess

import (
	"errors"
	"fmt"
	"sort"
)

const FidelityReportSchema = "evalwitness.transcript-fidelity.v1"

type TokenEstimateComparison struct {
	ObservationCount      int     `json:"observation_count"`
	EstimatedTokens       int     `json:"estimated_tokens"`
	ProviderReportedTotal int     `json:"provider_reported_total"`
	AbsoluteDifference    int     `json:"absolute_difference"`
	RelativeDifference    float64 `json:"relative_difference,omitempty"`
}

type BudgetFidelity struct {
	BudgetTokens     int                 `json:"budget_tokens"`
	RetainedTokens   int                 `json:"retained_tokens"`
	RetainedEvents   int                 `json:"retained_events"`
	RetainedBytes    int                 `json:"retained_bytes"`
	TrajectoryDigest string              `json:"trajectory_digest"`
	Truncation       TruncationBoundary  `json:"truncation"`
	Categories       []CategoryRetention `json:"categories"`
}

type FidelityReport struct {
	SchemaVersion    string                   `json:"schema_version"`
	SourceFormat     SourceFormat             `json:"source_format"`
	SourceDigest     string                   `json:"source_digest"`
	TrajectoryDigest string                   `json:"trajectory_digest"`
	Ingestion        IngestionReport          `json:"ingestion"`
	EstimatedTokens  int                      `json:"estimated_tokens"`
	ProviderUsage    []ProviderTokenUsage     `json:"provider_usage,omitempty"`
	TokenComparison  *TokenEstimateComparison `json:"token_comparison,omitempty"`
	Budgets          []BudgetFidelity         `json:"budgets"`
}

func AuditFidelity(trajectory Trajectory, budgets []int) (FidelityReport, error) {
	if err := trajectory.Validate(); err != nil {
		return FidelityReport{}, fmt.Errorf("validate fidelity source: %w", err)
	}
	if len(budgets) == 0 {
		return FidelityReport{}, errors.New("fidelity audit requires at least one evidence budget")
	}
	orderedBudgets := append([]int(nil), budgets...)
	sort.Ints(orderedBudgets)
	for index, budget := range orderedBudgets {
		if budget <= 0 {
			return FidelityReport{}, fmt.Errorf("fidelity budget %d must be positive", budget)
		}
		if index > 0 && budget == orderedBudgets[index-1] {
			return FidelityReport{}, fmt.Errorf("duplicate fidelity budget %d", budget)
		}
	}
	estimated := estimateTokensForBytes(len(RenderTrajectory(trajectory)))
	report := FidelityReport{
		SchemaVersion: FidelityReportSchema, SourceFormat: trajectory.SourceFormat,
		SourceDigest: trajectory.SourceDigest, TrajectoryDigest: trajectory.Digest,
		Ingestion: trajectory.Report, EstimatedTokens: estimated,
		ProviderUsage: append([]ProviderTokenUsage(nil), trajectory.Report.ProviderUsage...),
		Budgets:       make([]BudgetFidelity, 0, len(orderedBudgets)),
	}
	if comparison := compareProviderUsage(estimated, trajectory.Report.ProviderUsage); comparison.ObservationCount > 0 {
		report.TokenComparison = &comparison
	}
	for _, budget := range orderedBudgets {
		retained, err := ApplyEvidenceBudget(trajectory, budget)
		if err != nil {
			return FidelityReport{}, fmt.Errorf("apply %d-token fidelity budget: %w", budget, err)
		}
		report.Budgets = append(report.Budgets, BudgetFidelity{
			BudgetTokens: budget, RetainedTokens: estimateTokensForBytes(len(RenderTrajectory(retained))),
			RetainedEvents: len(retained.Events), RetainedBytes: retained.Report.RetainedBytes,
			TrajectoryDigest: retained.Digest, Truncation: retained.Report.Truncation,
			Categories: append([]CategoryRetention(nil), retained.Report.Categories...),
		})
	}
	return report, nil
}

func compareProviderUsage(estimated int, usages []ProviderTokenUsage) TokenEstimateComparison {
	seen := make(map[string]struct{}, len(usages))
	comparison := TokenEstimateComparison{EstimatedTokens: estimated}
	for _, usage := range usages {
		sourceIdentity := usage.SourceEventID
		if sourceIdentity == "" {
			sourceIdentity = usage.Source.key()
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00%d\x00%d\x00%d",
			usage.Provider, usage.Scope, sourceIdentity, usage.InputTokens, usage.OutputTokens,
			usage.ReasoningTokens, usage.CachedInputTokens, usage.CacheCreationInputTokens, usage.TotalTokens)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		comparison.ObservationCount++
		comparison.ProviderReportedTotal += usage.TotalTokens
	}
	comparison.AbsoluteDifference = estimated - comparison.ProviderReportedTotal
	if comparison.AbsoluteDifference < 0 {
		comparison.AbsoluteDifference = -comparison.AbsoluteDifference
	}
	if comparison.ProviderReportedTotal > 0 {
		comparison.RelativeDifference = float64(comparison.AbsoluteDifference) / float64(comparison.ProviderReportedTotal)
	}
	return comparison
}
