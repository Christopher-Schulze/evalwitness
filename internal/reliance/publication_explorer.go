package reliance

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/claim"
)

const relianceExplorerInterval = "bonferroni_family_adjusted_two_sided_cluster_sandwich_interval"

func BuildRelianceExplorerProjection(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claim.Ledger,
) (RelianceExplorerProjection, error) {
	value, component, err := verifiedRelianceProjectionSources(ctx, base, child, ledger)
	if err != nil {
		return RelianceExplorerProjection{}, err
	}
	profile, err := newRelianceProfileProjection(value, component, child.Manifest, ledger)
	if err != nil {
		return RelianceExplorerProjection{}, err
	}
	paper, err := newReliancePaperProjection(value, component, child.Manifest, ledger)
	if err != nil {
		return RelianceExplorerProjection{}, err
	}
	projection := newRelianceExplorerProjection(base, child, component, value, profile, paper)
	projection.Digest, err = relianceExplorerProjectionDigest(projection)
	if err != nil {
		return RelianceExplorerProjection{}, err
	}
	return projection, projection.Validate()
}

func newRelianceExplorerProjection(
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	component capsule.ComponentRecord,
	value EvidenceRelianceMap,
	profile RelianceProfileProjection,
	paper ReliancePaperProjection,
) RelianceExplorerProjection {
	return RelianceExplorerProjection{
		SchemaVersion: RelianceExplorerProjectionSchemaVersion, Availability: "available",
		BaseCapsuleID: base.Manifest.CapsuleID, CapsuleID: child.Manifest.CapsuleID,
		ManifestDigest: child.Manifest.ManifestDigest, MapDigest: value.Digest,
		LedgerDigest: profile.LedgerDigest, ProfileDigest: profile.Digest, PaperDigest: paper.Digest,
		Source: relianceExplorerSource(component, value.Digest), Scope: value.Scope,
		SourceTasks: value.SourceTasks, RegisteredCells: value.RegisteredCells,
		OutcomeBearingCells: value.OutcomeBearingCells, ExcludedCells: value.ExcludedCells,
		CompletedLogicalCalls: value.CompletedLogicalCalls,
		CellStatusCounts:      slices.Clone(value.StatusCounts), ProfileStatusCounts: slices.Clone(profile.StatusCounts),
		Outcomes: buildRelianceExplorerOutcomes(profile), Selectors: buildRelianceExplorerSelectors(value.Terms),
		ArmContrasts:  buildRelianceExplorerArmContrasts(value.ArmComparisons),
		Witnesses:     buildRelianceExplorerWitnesses(value.Witnesses),
		AllowedClaims: slices.Clone(value.AllowedClaims), ForbiddenClaims: slices.Clone(value.ForbiddenClaims),
		Limitations: slices.Clone(paper.Limitations), CurrentClaimIDs: slices.Clone(paper.CurrentClaimIDs),
		UnsupportedClaimIDs: slices.Clone(paper.UnsupportedClaimIDs), GlobalScoreProhibited: profile.GlobalScoreProhibited,
		ProviderCalls: profile.ProviderCalls + paper.ProviderCalls, NetworkRequired: profile.NetworkRequired || paper.NetworkRequired,
	}
}

func relianceExplorerSource(component capsule.ComponentRecord, mapDigest string) RelianceExplorerArtifactRef {
	return RelianceExplorerArtifactRef{
		Kind: "capsule_component", ID: component.ComponentID, SchemaVersion: component.TypeID,
		PayloadSHA256: component.Payload.Digest, ArtifactDigest: mapDigest,
	}
}

func buildRelianceExplorerOutcomes(profile RelianceProfileProjection) []RelianceExplorerOutcome {
	result := make([]RelianceExplorerOutcome, len(profile.Dimensions))
	for index, dimension := range profile.Dimensions {
		result[index] = RelianceExplorerOutcome{
			DimensionID: dimension.DimensionID, MapTermID: dimension.MapTermID,
			TermKind: dimension.TermKind, Factors: slices.Clone(dimension.Factors), OutcomeID: dimension.OutcomeID,
			Status: dimension.Status, Numerator: dimension.EligibleObservations,
			Denominator: dimension.RegisteredCells, InvalidCells: dimension.InvalidCells,
			Estimate: relianceExplorerEstimate(dimension.Estimate), Reason: dimension.Reason,
			Unit: dimension.Unit, Interval: relianceExplorerInterval, Policy: dimension.Policy,
			Route: dimension.Scope.RouteID, Domain: dimension.Scope.Domain, DataRole: dimension.Scope.DataRole,
			EvidenceLevel: dimension.EvidenceLevel, CapsuleID: profile.CapsuleID,
			CapsuleExpression: dimension.CapsuleExpression, SourceAnalysisDigest: dimension.SourceAnalysisDigest,
			ClaimBoundary: dimension.ClaimBoundary,
		}
	}
	return result
}

func relianceExplorerEstimate(value *RelianceMapEstimate) *RelianceExplorerEstimate {
	if value == nil {
		return nil
	}
	return &RelianceExplorerEstimate{
		Estimate:      formatRelianceExplorerNumber(value.Estimate),
		StandardError: formatRelianceExplorerNumber(value.StandardError),
		Lower:         formatRelianceExplorerNumber(value.Lower), Upper: formatRelianceExplorerNumber(value.Upper),
		AdjustedPValue: formatRelianceExplorerNumber(value.AdjustedPValue),
	}
}

func formatRelianceExplorerNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func buildRelianceExplorerSelectors(terms []RelianceMapTerm) []RelianceExplorerSelector {
	result := make([]RelianceExplorerSelector, 0, len(terms))
	for _, term := range terms {
		if term.Selector == nil {
			continue
		}
		result = append(result, RelianceExplorerSelector{
			TermID: term.TermID, FactorID: term.Factors[0], EffectStatus: term.Selector.EffectStatus,
			AssignmentTargets: term.Selector.AssignmentTargets, MinimumEventScore: term.Selector.MinimumEventScore,
			MaximumEventScore: term.Selector.MaximumEventScore,
			Budgets:           buildRelianceExplorerSelectorBudgets(term.Selector.Budgets),
		})
	}
	return result
}

func buildRelianceExplorerSelectorBudgets(values []RelianceMapSelectorBudget) []RelianceExplorerSelectorBudget {
	result := make([]RelianceExplorerSelectorBudget, len(values))
	for index, value := range values {
		result[index] = RelianceExplorerSelectorBudget{
			BudgetTokens: value.BudgetTokens, AssignmentTargets: value.AssignmentTargets,
			ExactTargets: value.ExactTargets, ChangedTargets: value.ChangedTargets,
			UnrenderedTargets: value.UnrenderedTargets, DroppedTargets: value.DroppedTargets,
			AssignedEventBytes:            value.AssignedEventBytes,
			RetainedAssignedEventBytes:    value.RetainedAssignedEventBytes,
			AssignedRenderedBytes:         value.AssignedRenderedBytes,
			RetainedAssignedRenderedBytes: value.RetainedAssignedRenderedBytes,
			RiskFlags:                     slices.Clone(value.RiskFlags),
		}
	}
	return result
}

func buildRelianceExplorerArmContrasts(values []RelianceMapArmComparison) []RelianceExplorerArmContrast {
	result := []RelianceExplorerArmContrast{}
	for _, value := range values {
		for _, contrast := range value.Comparison.Contrasts {
			result = append(result, RelianceExplorerArmContrast{
				ComparisonDigest: value.Comparison.Digest, ContrastID: contrast.ContrastID, Kind: contrast.Kind,
				ReferenceArmID: contrast.ReferenceArmID, ComparatorArmID: contrast.ComparatorArmID,
				Direction: contrast.Direction, ChangedDimensions: slices.Clone(contrast.ChangedDimensions),
				Support: contrast.Support, Reason: contrast.Reason, Numerator: contrast.EligiblePairs,
				Denominator: contrast.RegisteredPairs, InvalidPairs: contrast.RegisteredPairs - contrast.EligiblePairs,
			})
		}
	}
	slices.SortFunc(result, compareRelianceExplorerArmContrasts)
	return result
}

func compareRelianceExplorerArmContrasts(left, right RelianceExplorerArmContrast) int {
	if compared := strings.Compare(left.ComparisonDigest, right.ComparisonDigest); compared != 0 {
		return compared
	}
	return strings.Compare(left.ContrastID, right.ContrastID)
}

func buildRelianceExplorerWitnesses(values []RelianceMapWitness) []RelianceExplorerWitness {
	result := make([]RelianceExplorerWitness, len(values))
	for index, value := range values {
		witness := value.Witness
		result[index] = RelianceExplorerWitness{
			CaseID: witness.CaseID, RelationDigest: witness.RelationDigest, WitnessDigest: witness.Digest,
			FinalEvaluationDigest: witness.FinalEvaluationDigest, SourceAnalysisDigest: value.SourceAnalysisDigest,
			BindingStatus: value.BindingStatus, OriginalInputDigest: witness.Counterexample.OriginalInputDigest,
			ReducedInputDigest: witness.Counterexample.ReducedInputDigest, FinalUnits: len(witness.FinalUnits),
			Evaluations: len(witness.Evaluations), RawTrajectoryContentShown: false,
		}
	}
	return result
}

func relianceExplorerProjectionDigest(value RelianceExplorerProjection) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
