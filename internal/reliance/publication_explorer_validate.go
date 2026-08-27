package reliance

import (
	"errors"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func (value RelianceExplorerProjection) Validate() error {
	for _, validation := range []func() error{
		func() error { return validateRelianceExplorerIdentity(value) },
		func() error { return validateRelianceExplorerCounts(value) },
		func() error { return validateRelianceExplorerOutcomes(value) },
		func() error { return validateRelianceExplorerSelectors(value.Selectors) },
		func() error { return validateRelianceExplorerArmContrasts(value.ArmContrasts) },
		func() error { return validateRelianceExplorerWitnesses(value.Witnesses) },
		func() error { return validateRelianceExplorerClaims(value) },
	} {
		if err := validation(); err != nil {
			return err
		}
	}
	digest, err := relianceExplorerProjectionDigest(value)
	if err != nil || digest != value.Digest {
		return errors.New("reliance explorer projection digest is invalid")
	}
	return nil
}

func validateRelianceExplorerIdentity(value RelianceExplorerProjection) error {
	if value.SchemaVersion != RelianceExplorerProjectionSchemaVersion || value.Availability != "available" ||
		!validRelianceDigest(value.BaseCapsuleID) || !validRelianceDigest(value.CapsuleID) ||
		value.BaseCapsuleID == value.CapsuleID || !validRelianceDigest(value.ManifestDigest) ||
		!validRelianceDigest(value.MapDigest) || !validRelianceDigest(value.LedgerDigest) ||
		!validRelianceDigest(value.ProfileDigest) || !validRelianceDigest(value.PaperDigest) ||
		value.ProviderCalls != 0 || value.NetworkRequired || !value.GlobalScoreProhibited ||
		!validRelianceDigest(value.Digest) || validateReliancePublicationScope(value.Scope) != nil {
		return errors.New("reliance explorer projection identity or execution boundary is invalid")
	}
	return validateRelianceExplorerSource(value.Source, value.MapDigest)
}

func validateRelianceExplorerSource(value RelianceExplorerArtifactRef, mapDigest string) error {
	if value.Kind != "capsule_component" || !validRelianceDigest(value.ID) ||
		value.SchemaVersion != EvidenceRelianceMapSchemaVersion || !validRelianceDigest(value.PayloadSHA256) ||
		value.ArtifactDigest != mapDigest {
		return errors.New("reliance explorer source identity is invalid")
	}
	return nil
}

func validateRelianceExplorerCounts(value RelianceExplorerProjection) error {
	if value.SourceTasks != RelianceSelectedSourceTasks ||
		value.RegisteredCells != RelianceSelectedSourceTasks*ReferenceCellsPerTask ||
		value.OutcomeBearingCells < 0 || value.OutcomeBearingCells > value.RegisteredCells ||
		value.ExcludedCells != value.RegisteredCells-value.OutcomeBearingCells || value.CompletedLogicalCalls < 0 {
		return errors.New("reliance explorer denominator accounting is invalid")
	}
	if err := validateRelianceExplorerCellStatusCounts(value.CellStatusCounts, value.RegisteredCells); err != nil {
		return err
	}
	return validateRelianceExplorerProfileStatusCounts(value.ProfileStatusCounts, len(value.Outcomes))
}

func validateRelianceExplorerCellStatusCounts(values []RelianceCellStatusCount, denominator int) error {
	want := canonicalRelianceCellStatuses()
	if len(values) != len(want) {
		return errors.New("reliance explorer cell status accounting is incomplete")
	}
	total := 0
	for index, value := range values {
		if value.Status != want[index] || value.Cells < 0 {
			return errors.New("reliance explorer cell status accounting is invalid")
		}
		total += value.Cells
	}
	if total != denominator {
		return errors.New("reliance explorer cell status accounting does not conserve the denominator")
	}
	return nil
}

func validateRelianceExplorerProfileStatusCounts(values []RelianceProfileStatusCount, denominator int) error {
	want := []string{"failed", "measured", "not_applicable", "not_measured", "unsupported"}
	if len(values) != len(want) {
		return errors.New("reliance explorer profile status accounting is incomplete")
	}
	total := 0
	for index, value := range values {
		if value.Status != want[index] || value.Dimensions < 0 {
			return errors.New("reliance explorer profile status accounting is invalid")
		}
		total += value.Dimensions
	}
	if total != denominator {
		return errors.New("reliance explorer profile statuses do not conserve dimensions")
	}
	return nil
}

func validateRelianceExplorerOutcomes(value RelianceExplorerProjection) error {
	terms := relianceExplorerTermIndex()
	want := len(terms) * len(reliancePublicationOutcomeIDs())
	if len(value.Outcomes) != want {
		return errors.New("reliance explorer outcomes do not cover the frozen map")
	}
	previous := ""
	for _, outcome := range value.Outcomes {
		term, found := terms[outcome.MapTermID]
		if !found || outcome.DimensionID <= previous || !slices.Equal(outcome.Factors, term.Factors) ||
			outcome.TermKind != relianceExplorerTermKind(term.Factors) {
			return errors.New("reliance explorer outcome identity is invalid or unordered")
		}
		if err := validateRelianceExplorerOutcome(value, outcome); err != nil {
			return err
		}
		previous = outcome.DimensionID
	}
	return nil
}

func relianceExplorerTermKind(factors []string) RelianceMapTermKind {
	if len(factors) == 1 {
		return RelianceMapMainEffect
	}
	return RelianceMapInteraction
}

func relianceExplorerTermIndex() map[string]struct {
	ID      string
	Factors []string
} {
	terms := referenceFactorialTerms()
	result := make(map[string]struct {
		ID      string
		Factors []string
	}, len(terms))
	for _, term := range terms {
		result[term.ID] = struct {
			ID      string
			Factors []string
		}{ID: term.ID, Factors: slices.Clone(term.Factors)}
	}
	return result
}

func validateRelianceExplorerOutcome(value RelianceExplorerProjection, outcome RelianceExplorerOutcome) error {
	if outcome.DimensionID != relianceProjectionID("evidence_reliance", outcome.MapTermID, string(outcome.OutcomeID)) ||
		!slices.Contains(reliancePublicationOutcomeIDs(), outcome.OutcomeID) ||
		outcome.Numerator < 0 || outcome.Denominator != value.RegisteredCells || outcome.InvalidCells < 0 ||
		outcome.Numerator+outcome.InvalidCells != outcome.Denominator || outcome.Unit == "" ||
		outcome.Interval != relianceExplorerInterval || outcome.Policy != "source_task_cluster_sandwich_ols.v1+bonferroni" ||
		outcome.Route != value.Scope.RouteID || outcome.Domain != value.Scope.Domain ||
		outcome.DataRole != value.Scope.DataRole || outcome.EvidenceLevel != value.Scope.EvidenceLevel ||
		outcome.CapsuleID != value.CapsuleID || !strings.HasPrefix(outcome.CapsuleExpression, "/terms/") ||
		!validRelianceDigest(outcome.SourceAnalysisDigest) || outcome.ClaimBoundary != RelianceProfileClaimBoundary {
		return errors.New("reliance explorer chart metadata or denominator is invalid")
	}
	return validateRelianceExplorerOutcomeState(outcome)
}

func validateRelianceExplorerOutcomeState(value RelianceExplorerOutcome) error {
	switch value.Status {
	case "measured":
		if value.Reason != "" || validateRelianceExplorerEstimate(value.Estimate) != nil {
			return errors.New("measured reliance explorer outcome lacks a valid estimate")
		}
	case "not_measured", "unsupported":
		if value.Reason == "" || value.Estimate != nil {
			return errors.New("unavailable reliance explorer outcome lacks an exact reason")
		}
	default:
		return errors.New("reliance explorer outcome status is invalid")
	}
	return nil
}

func validateRelianceExplorerEstimate(value *RelianceExplorerEstimate) error {
	if value == nil {
		return errors.New("reliance explorer estimate is absent")
	}
	estimate, err := parseRelianceExplorerNumber(value.Estimate)
	if err != nil {
		return err
	}
	standardError, err := parseRelianceExplorerNumber(value.StandardError)
	if err != nil {
		return err
	}
	lower, err := parseRelianceExplorerNumber(value.Lower)
	if err != nil {
		return err
	}
	upper, err := parseRelianceExplorerNumber(value.Upper)
	if err != nil {
		return err
	}
	adjusted, err := parseRelianceExplorerNumber(value.AdjustedPValue)
	if err != nil || standardError < 0 || lower > upper || adjusted < 0 || adjusted > 1 || math.IsNaN(estimate) {
		return errors.New("reliance explorer estimate bounds are invalid")
	}
	return nil
}

func parseRelianceExplorerNumber(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || formatRelianceExplorerNumber(parsed) != value {
		return 0, errors.New("reliance explorer number is not a canonical finite decimal")
	}
	return parsed, nil
}

func validateRelianceExplorerSelectors(values []RelianceExplorerSelector) error {
	want := relianceExplorerSelectorTerms()
	if len(values) != len(want) {
		return errors.New("reliance explorer selector coverage is incomplete")
	}
	for index, value := range values {
		if value.TermID != want[index].ID || value.FactorID != want[index].Factors[0] ||
			!validSelectorEffectStatus(value.EffectStatus) || value.AssignmentTargets < 1 ||
			value.MinimumEventScore > value.MaximumEventScore || len(value.Budgets) == 0 {
			return errors.New("reliance explorer selector identity is invalid")
		}
		if err := validateRelianceExplorerSelectorBudgets(value); err != nil {
			return err
		}
	}
	return nil
}

func relianceExplorerSelectorTerms() []struct {
	ID      string
	Factors []string
} {
	terms := referenceFactorialTerms()
	result := make([]struct {
		ID      string
		Factors []string
	}, 0, len(terms))
	for _, term := range terms {
		if len(term.Factors) == 1 && term.Factors[0] != PresentationOrderTerm {
			result = append(result, struct {
				ID      string
				Factors []string
			}{term.ID, slices.Clone(term.Factors)})
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func validateRelianceExplorerSelectorBudgets(selector RelianceExplorerSelector) error {
	previous := 0
	for _, budget := range selector.Budgets {
		classified := budget.ExactTargets + budget.ChangedTargets + budget.UnrenderedTargets + budget.DroppedTargets
		if budget.BudgetTokens <= previous || budget.AssignmentTargets != selector.AssignmentTargets ||
			classified != selector.AssignmentTargets || budget.ExactTargets < 0 || budget.ChangedTargets < 0 ||
			budget.UnrenderedTargets < 0 || budget.DroppedTargets < 0 || budget.AssignedEventBytes < 0 ||
			budget.RetainedAssignedEventBytes < 0 || budget.RetainedAssignedEventBytes > budget.AssignedEventBytes ||
			budget.AssignedRenderedBytes < 0 || budget.RetainedAssignedRenderedBytes < 0 ||
			budget.RetainedAssignedRenderedBytes > budget.AssignedRenderedBytes {
			return errors.New("reliance explorer selector budget is invalid")
		}
		previous = budget.BudgetTokens
	}
	return nil
}

func validateRelianceExplorerArmContrasts(values []RelianceExplorerArmContrast) error {
	if len(values) < 5 {
		return errors.New("reliance explorer arm contrast coverage is incomplete")
	}
	foundKinds := make(map[RelianceArmContrastKind]bool, 5)
	previous := ""
	for _, value := range values {
		identity := value.ComparisonDigest + "/" + value.ContrastID
		if identity <= previous || !validRelianceDigest(value.ComparisonDigest) || !validRelianceContrastKind(value.Kind) ||
			value.ReferenceArmID == "" || value.ComparatorArmID == "" || value.Direction != RelianceArmContrastDirection ||
			len(value.ChangedDimensions) == 0 || value.Numerator < 0 || value.Denominator < value.Numerator ||
			value.InvalidPairs != value.Denominator-value.Numerator {
			return errors.New("reliance explorer arm contrast is invalid or unordered")
		}
		if err := validateRelianceExplorerArmSupport(value); err != nil {
			return err
		}
		foundKinds[value.Kind], previous = true, identity
	}
	if len(foundKinds) != 5 {
		return errors.New("reliance explorer arm contrast families are incomplete")
	}
	return nil
}

func validateRelianceExplorerArmSupport(value RelianceExplorerArmContrast) error {
	if value.Support == RelianceArmContrastSupported && value.Reason == "" {
		return nil
	}
	if value.Support == RelianceArmContrastUnsupported && value.Reason != "" {
		return nil
	}
	return errors.New("reliance explorer arm contrast support is invalid")
}

func validateRelianceExplorerWitnesses(values []RelianceExplorerWitness) error {
	if len(values) == 0 {
		return errors.New("reliance explorer minimal witnesses are absent")
	}
	for _, value := range values {
		if value.CaseID == "" || !validRelianceDigest(value.RelationDigest) || !validRelianceDigest(value.WitnessDigest) ||
			!validRelianceDigest(value.FinalEvaluationDigest) || !validRelianceDigest(value.SourceAnalysisDigest) ||
			value.BindingStatus != RelianceWitnessRelationBinding || !validRelianceDigest(value.OriginalInputDigest) ||
			!validRelianceDigest(value.ReducedInputDigest) || value.FinalUnits < 1 || value.Evaluations < 1 ||
			value.RawTrajectoryContentShown {
			return errors.New("reliance explorer minimal witness boundary is invalid")
		}
	}
	return nil
}

func validateRelianceExplorerClaims(value RelianceExplorerProjection) error {
	if !slices.Equal(value.AllowedClaims, relianceAllowedClaims()) ||
		!slices.Equal(value.ForbiddenClaims, relianceForbiddenClaims()) ||
		!slices.Equal(value.Limitations, reliancePaperLimitations()) || len(value.CurrentClaimIDs) != 8 ||
		len(value.UnsupportedClaimIDs) != 3 || validateSortedRelianceExplorerStrings(value.CurrentClaimIDs) != nil ||
		validateSortedRelianceExplorerStrings(value.UnsupportedClaimIDs) != nil {
		return errors.New("reliance explorer claim or limitation boundary is invalid")
	}
	return nil
}

func validateSortedRelianceExplorerStrings(values []string) error {
	if len(values) == 0 || !slices.IsSorted(values) || len(slices.Compact(slices.Clone(values))) != len(values) {
		return errors.New("reliance explorer identifiers are empty, duplicated, or unordered")
	}
	for _, value := range values {
		if value == "" {
			return errors.New("reliance explorer identifier is empty")
		}
	}
	return nil
}
