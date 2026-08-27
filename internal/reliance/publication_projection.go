package reliance

import (
	"slices"
	"strconv"
)

func buildRelianceProfileDimensions(value EvidenceRelianceMap) []RelianceProfileDimension {
	result := make([]RelianceProfileDimension, 0, relianceMapOutcomeCapacity(value.Terms))
	for termIndex, term := range value.Terms {
		for outcomeIndex, outcome := range term.Outcomes {
			result = append(result, RelianceProfileDimension{
				DimensionID: relianceProjectionID("evidence_reliance", term.TermID, string(outcome.OutcomeID)),
				MapTermID:   term.TermID, TermKind: term.Kind, Factors: slices.Clone(term.Factors),
				OutcomeID: outcome.OutcomeID,
				Status:    profileStatusForRelianceMap(outcome.Status), EvidenceLevel: value.Scope.EvidenceLevel,
				Scope:           value.Scope,
				RegisteredCells: outcome.RegisteredCells, EligibleObservations: outcome.EligibleObservations,
				ExcludedFromFit: outcome.ExcludedFromFit, InvalidCells: outcome.ExcludedFromFit,
				Estimate: cloneRelianceMapEstimate(outcome.Estimate),
				Reason:   outcome.Reason, SourceAnalysisDigest: value.AnalysisDigest,
				Policy: "source_task_cluster_sandwich_ols.v1+bonferroni",
				Unit:   "source_task_clustered_cell_contrast", InterventionFamily: RelianceInterventionFamily,
				ClaimBoundary:     RelianceProfileClaimBoundary,
				CapsuleExpression: relianceMapOutcomePointer(termIndex, outcomeIndex),
				Caveats:           relianceProjectionCaveats(),
			})
		}
	}
	return result
}

func buildReliancePaperRows(value EvidenceRelianceMap) []ReliancePaperRow {
	result := make([]ReliancePaperRow, 0, relianceMapOutcomeCapacity(value.Terms))
	for termIndex, term := range value.Terms {
		for outcomeIndex, outcome := range term.Outcomes {
			result = append(result, ReliancePaperRow{
				RowID:     relianceProjectionID("reliance", term.TermID, string(outcome.OutcomeID)),
				MapTermID: term.TermID, TermKind: term.Kind, Factors: slices.Clone(term.Factors),
				OutcomeID: outcome.OutcomeID, Status: outcome.Status, Scope: value.Scope,
				RegisteredCells: outcome.RegisteredCells, EligibleObservations: outcome.EligibleObservations,
				ExcludedFromFit: outcome.ExcludedFromFit, Estimate: cloneRelianceMapEstimate(outcome.Estimate),
				Reason: outcome.Reason, SourceAnalysisDigest: value.AnalysisDigest,
				CapsuleExpression: relianceMapOutcomePointer(termIndex, outcomeIndex),
			})
		}
	}
	return result
}

func profileStatusForRelianceMap(status RelianceMapStatus) string {
	switch status {
	case RelianceMapMeasured:
		return "measured"
	case RelianceMapAmbiguous, RelianceMapUnsupported:
		return "unsupported"
	default:
		return "not_measured"
	}
}

func relianceProjectionID(prefix, termID, outcomeID string) string {
	return prefix + "." + termID + "." + outcomeID
}

func relianceMapOutcomePointer(termIndex, outcomeIndex int) string {
	return "/terms/" + strconv.Itoa(termIndex) + "/outcomes/" + strconv.Itoa(outcomeIndex)
}

func cloneRelianceMapEstimate(value *RelianceMapEstimate) *RelianceMapEstimate {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func relianceProjectionCaveats() []string {
	return slices.Clone([]string{
		"E1 local mechanism evidence only",
		"No agent-step causality or model-internal attribution",
		"No provider, model-family, route, entrypoint, or population transfer claim",
	})
}

func relianceMapOutcomeCapacity(terms []RelianceMapTerm) int {
	total := 0
	for _, term := range terms {
		total += len(term.Outcomes)
	}
	return total
}
