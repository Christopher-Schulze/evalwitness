package reliance

import (
	"errors"
	"math"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func validatePublishedRelianceComparisons(value EvidenceRelianceMap) error {
	if len(value.ArmComparisons) == 0 {
		return errors.New("evidence reliance map lacks an arm comparison")
	}
	previous := ""
	for _, published := range value.ArmComparisons {
		comparison := published.Comparison
		if comparison.Digest <= previous {
			return errors.New("published reliance arm comparisons are duplicated or noncanonical")
		}
		if err := validatePublishedRelianceComparison(value, comparison); err != nil {
			return err
		}
		previous = comparison.Digest
	}
	return validatePublishedRelianceContrastCoverage(value.ArmComparisons)
}

func validatePublishedRelianceContrastCoverage(values []RelianceMapArmComparison) error {
	want := []RelianceArmContrastKind{
		RelianceContrastEvidencePolicy, RelianceContrastEntrypoint, RelianceContrastModelFamily,
		RelianceContrastProvider, RelianceContrastRoute,
	}
	found := make(map[RelianceArmContrastKind]bool, len(want))
	for _, published := range values {
		for _, contrast := range published.Comparison.Contrasts {
			found[contrast.Kind] = true
		}
	}
	for _, kind := range want {
		if !found[kind] {
			return errors.New("evidence reliance map lacks a prespecified arm-contrast family")
		}
	}
	return nil
}

func validatePublishedRelianceComparison(value EvidenceRelianceMap, comparison RelianceArmComparison) error {
	if comparison.SchemaVersion != RelianceArmComparisonSchemaVersion || comparison.CanonicalPolicy != CanonicalPolicy ||
		comparison.ContrastPolicyVersion != RelianceArmContrastPolicyVersion ||
		comparison.PreregistrationDigest != value.PreregistrationDigest || comparison.PreflightDigest != value.PreflightDigest ||
		comparison.MultiplicityMethod != "bonferroni" || comparison.ProviderCalls != 0 || comparison.NetworkRequired ||
		comparison.MultiplicityFamilySize != len(comparison.Contrasts)*len(referenceFactorialTerms())*len(reliancePublicationOutcomeIDs()) {
		return errors.New("published reliance arm comparison identity or multiplicity boundary is invalid")
	}
	if err := validatePublishedRelianceArms(comparison.Arms); err != nil {
		return err
	}
	if err := validatePublishedRelianceContrasts(value, comparison); err != nil {
		return err
	}
	digest, err := relianceArmComparisonDigest(comparison)
	if err != nil || comparison.Digest != digest {
		return errors.New("published reliance arm comparison digest is invalid")
	}
	return nil
}

func validatePublishedRelianceArms(values []RelianceArmSummary) error {
	if len(values) < 2 {
		return errors.New("published reliance arm comparison has fewer than two arms")
	}
	for index, value := range values {
		if index > 0 && value.ArmID <= values[index-1].ArmID {
			return errors.New("published reliance arms are duplicated or noncanonical")
		}
		if !validPanelIdentifier(value.ArmID) || !validPanelIdentifier(value.ModelFamilyID) ||
			!validRelianceModelIdentityStatus(value.ModelIdentityStatus) || value.Arm.Validate() != nil {
			return errors.New("published reliance arm identity is invalid")
		}
		for _, digest := range []string{value.RouteAttestationDigest, value.RegistrationDigest, value.CorpusDigest, value.AnalysisDigest} {
			if !validRelianceDigest(digest) {
				return errors.New("published reliance arm contains an invalid evidence digest")
			}
		}
	}
	return nil
}

func validatePublishedRelianceContrasts(value EvidenceRelianceMap, comparison RelianceArmComparison) error {
	if len(comparison.Contrasts) == 0 {
		return errors.New("published reliance arm comparison lacks a contrast")
	}
	arms := publishedRelianceArmIDs(comparison.Arms)
	for index, contrast := range comparison.Contrasts {
		if index > 0 && contrast.ContrastID <= comparison.Contrasts[index-1].ContrastID {
			return errors.New("published reliance contrasts are duplicated or noncanonical")
		}
		if err := validatePublishedRelianceContrast(value, comparison, contrast, arms); err != nil {
			return err
		}
	}
	return nil
}

func publishedRelianceArmIDs(values []RelianceArmSummary) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value.ArmID] = struct{}{}
	}
	return result
}

func validatePublishedRelianceContrast(
	value EvidenceRelianceMap,
	comparison RelianceArmComparison,
	contrast RelianceArmContrast,
	arms map[string]struct{},
) error {
	_, referenceFound := arms[contrast.ReferenceArmID]
	_, comparatorFound := arms[contrast.ComparatorArmID]
	if !validPanelIdentifier(contrast.ContrastID) || !validRelianceContrastKind(contrast.Kind) ||
		contrast.ReferenceArmID == contrast.ComparatorArmID || !referenceFound || !comparatorFound ||
		contrast.Direction != RelianceArmContrastDirection || !validRelianceChangedDimensions(contrast.ChangedDimensions) {
		return errors.New("published reliance contrast identity or dimensions are invalid")
	}
	switch contrast.Support {
	case RelianceArmContrastSupported:
		return validateSupportedRelianceContrast(value, comparison, contrast)
	case RelianceArmContrastUnsupported:
		if contrast.Reason == "" || contrast.RegisteredPairs != 0 || contrast.EligiblePairs != 0 ||
			len(contrast.PairingStatusCounts) != 0 || len(contrast.OutcomeFits) != 0 {
			return errors.New("unsupported published reliance contrast carries fitted evidence")
		}
		return nil
	default:
		return errors.New("published reliance contrast support status is invalid")
	}
}

func validRelianceChangedDimensions(values []string) bool {
	allowed := []string{
		"criterion", "entrypoint", "evidence_policy", "model_family", "model_identity_status",
		"provider", "requested_model", "route", "score_tag",
	}
	if !slices.IsSorted(values) {
		return false
	}
	for index, value := range values {
		if !slices.Contains(allowed, value) || index > 0 && value == values[index-1] {
			return false
		}
	}
	return true
}

func validateSupportedRelianceContrast(
	value EvidenceRelianceMap,
	comparison RelianceArmComparison,
	contrast RelianceArmContrast,
) error {
	if contrast.Reason != "" || !slices.Equal(contrast.ChangedDimensions, expectedRelianceContrastDimensions(contrast.Kind)) ||
		contrast.RegisteredPairs != value.RegisteredCells || contrast.EligiblePairs < 0 ||
		contrast.EligiblePairs > contrast.RegisteredPairs {
		return errors.New("supported published reliance contrast scope or denominator is invalid")
	}
	if err := validatePublishedReliancePairingCounts(contrast); err != nil {
		return err
	}
	return validatePublishedRelianceOutcomeFits(comparison, contrast)
}

func validatePublishedReliancePairingCounts(contrast RelianceArmContrast) error {
	want := []ReliancePairingStatus{
		ReliancePairBothMissing, ReliancePairComparatorOnly, ReliancePairPaired, ReliancePairReferenceOnly,
	}
	if len(contrast.PairingStatusCounts) != len(want) {
		return errors.New("published reliance pairing statuses are incomplete")
	}
	total := 0
	for index, count := range contrast.PairingStatusCounts {
		if count.Status != want[index] || count.Cells < 0 {
			return errors.New("published reliance pairing statuses are invalid or noncanonical")
		}
		total += count.Cells
	}
	if total != contrast.RegisteredPairs || pairingStatusCells(contrast.PairingStatusCounts, ReliancePairPaired) != contrast.EligiblePairs {
		return errors.New("published reliance pairing statuses do not conserve the denominator")
	}
	return nil
}

func validatePublishedRelianceOutcomeFits(
	comparison RelianceArmComparison,
	contrast RelianceArmContrast,
) error {
	want := reliancePublicationOutcomeIDs()
	if len(contrast.OutcomeFits) != len(want) {
		return errors.New("published reliance contrast does not cover every outcome")
	}
	for index, outcome := range contrast.OutcomeFits {
		if outcome.OutcomeID != want[index] || outcome.RegisteredPairs != contrast.RegisteredPairs ||
			outcome.EligiblePairs != contrast.EligiblePairs || outcome.ExcludedFromFit != contrast.RegisteredPairs-contrast.EligiblePairs {
			return errors.New("published reliance contrast outcome denominator is invalid")
		}
		if err := validatePublishedRelianceOutcomeFit(comparison, outcome); err != nil {
			return err
		}
	}
	return nil
}

func validatePublishedRelianceOutcomeFit(
	comparison RelianceArmComparison,
	value RelianceArmContrastOutcomeFit,
) error {
	switch value.Status {
	case RelianceFitMeasured:
		if value.Reason != "" || validatePublishedFactorialFit(value.Fit, value.EligiblePairs, comparison.MultiplicityFamilySize) != nil {
			return errors.New("measured published reliance contrast fit is invalid")
		}
	case RelianceFitInconclusive:
		if value.Reason == "" || value.Fit != nil {
			return errors.New("inconclusive published reliance contrast fit lacks a reason or carries an estimate")
		}
	default:
		return errors.New("published reliance contrast fit status is invalid")
	}
	return nil
}

func validatePublishedFactorialFit(value *stats.ClusteredFactorialFit, observations int, familySize int) error {
	terms := referenceFactorialTerms()
	if value == nil || value.SchemaVersion != stats.ClusteredFactorialFitSchemaVersion ||
		value.Method != "source_task_cluster_sandwich_ols.v1" || value.Observations != observations ||
		value.Clusters < 2 || value.Clusters > RelianceSelectedSourceTasks || value.Parameters != len(terms)+1 ||
		value.Rank != value.Parameters || value.NominalAlpha != referenceNominalAlpha || value.FamilySize != familySize ||
		value.FamilyAdjustedAlpha != value.NominalAlpha/float64(value.FamilySize) ||
		!finiteRelianceNumber(value.CriticalValue) || value.CriticalValue <= 0 || len(value.Estimates) != len(terms) {
		return errors.New("published clustered factorial fit identity or dimensions are invalid")
	}
	for index, estimate := range value.Estimates {
		if estimate.TermID != terms[index].ID || !validPublishedFactorialEstimate(estimate, *value) {
			return errors.New("published clustered factorial estimate is invalid or noncanonical")
		}
	}
	return nil
}

func validPublishedFactorialEstimate(value stats.FactorialEstimate, fit stats.ClusteredFactorialFit) bool {
	if !finiteRelianceNumber(value.Estimate) || !finiteRelianceNumber(value.StandardError) ||
		!finiteRelianceNumber(value.Lower) || !finiteRelianceNumber(value.Upper) ||
		!finiteRelianceNumber(value.RawPValue) || !finiteRelianceNumber(value.AdjustedPValue) ||
		value.StandardError < 0 || value.RawPValue < 0 || value.RawPValue > 1 ||
		value.AdjustedPValue < 0 || value.AdjustedPValue > 1 || !value.FamilyAdjusted {
		return false
	}
	wantLower := value.Estimate - fit.CriticalValue*value.StandardError
	wantUpper := value.Estimate + fit.CriticalValue*value.StandardError
	wantAdjusted := math.Min(1, value.RawPValue*float64(fit.FamilySize))
	return closeRelianceFloat(value.Lower, wantLower) && closeRelianceFloat(value.Upper, wantUpper) &&
		closeRelianceFloat(value.AdjustedPValue, wantAdjusted)
}

func closeRelianceFloat(left, right float64) bool {
	scale := math.Max(1, math.Max(math.Abs(left), math.Abs(right)))
	return math.Abs(left-right) <= 1e-12*scale
}
