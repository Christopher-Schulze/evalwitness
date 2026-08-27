package reliance

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func (value EvidenceRelianceMap) Validate() error {
	if err := validateEvidenceRelianceMapIdentity(value); err != nil {
		return err
	}
	if err := validateRelianceMapCounts(value); err != nil {
		return err
	}
	if err := validateRelianceMapTerms(value.Terms, value.RegisteredCells); err != nil {
		return err
	}
	if err := validatePublishedRelianceComparisons(value); err != nil {
		return err
	}
	if err := validatePublishedRelianceWitnesses(value.AnalysisDigest, value.Witnesses); err != nil {
		return err
	}
	if err := validateRelianceDerivedProjections(value); err != nil {
		return err
	}
	digest, err := evidenceRelianceMapDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("evidence reliance map digest is invalid")
	}
	return nil
}

func validateEvidenceRelianceMapIdentity(value EvidenceRelianceMap) error {
	if value.SchemaVersion != EvidenceRelianceMapSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.PublicationPolicy != ReliancePublicationPolicyVersion || value.ProjectionProviderCalls != 0 ||
		value.NetworkRequired || !validRelianceDigest(value.Digest) {
		return errors.New("evidence reliance map identity or execution boundary is invalid")
	}
	for _, digest := range []string{
		value.OntologyDigest, value.EstimandCatalogDigest, value.PreregistrationDigest,
		value.PreflightDigest, value.RegistrationDigest, value.CorpusDigest,
		value.AnalysisDigest, value.SelectorAuditDigest,
	} {
		if !validRelianceDigest(digest) {
			return errors.New("evidence reliance map parent digest is invalid")
		}
	}
	return validateReliancePublicationScope(value.Scope)
}

func validateReliancePublicationScope(value ReliancePublicationScope) error {
	if value.EvidenceRole != RelianceLocalMechanismRole || value.DataRole != RelianceLocalMechanismDataRole ||
		value.EvidenceLevel != RelianceLocalEvidenceLevel || value.Domain != "coding_agent_trajectory" || value.Empirical ||
		!validRelianceDigest(value.StudyManifestDigest) || !validPanelIdentifier(value.Entrypoint) ||
		!validPanelIdentifier(value.CriterionID) || !validPanelIdentifier(value.ScoreTag) ||
		value.EvidencePolicy == "" || !validPanelIdentifier(value.ProviderID) ||
		!strings.HasPrefix(value.RouteID, "route-") || !validPanelIdentifier(value.RequestedModel) {
		return errors.New("evidence reliance map scope is invalid or exceeds local mechanism evidence")
	}
	return nil
}

func validateRelianceMapCounts(value EvidenceRelianceMap) error {
	if value.SourceTasks != RelianceSelectedSourceTasks || value.RegisteredCells != RelianceSelectedSourceTasks*ReferenceCellsPerTask ||
		value.OutcomeBearingCells < 0 || value.OutcomeBearingCells > value.RegisteredCells ||
		value.ExcludedCells != value.RegisteredCells-value.OutcomeBearingCells || value.CompletedLogicalCalls < 0 ||
		len(value.StatusCounts) != len(canonicalRelianceCellStatuses()) {
		return errors.New("evidence reliance map denominator or logical-call accounting is invalid")
	}
	total := 0
	for index, count := range value.StatusCounts {
		if count.Status != canonicalRelianceCellStatuses()[index] || count.Cells < 0 {
			return errors.New("evidence reliance map status counts are invalid")
		}
		total += count.Cells
	}
	if total != value.RegisteredCells {
		return errors.New("evidence reliance map status counts do not conserve the denominator")
	}
	return nil
}

func validateRelianceMapTerms(values []RelianceMapTerm, registeredCells int) error {
	want := referenceFactorialTerms()
	slices.SortFunc(want, func(left, right stats.FactorialTerm) int { return strings.Compare(left.ID, right.ID) })
	if len(values) != len(want) {
		return errors.New("evidence reliance map does not cover every frozen term")
	}
	for index, value := range values {
		if value.TermID != want[index].ID || !slices.Equal(value.Factors, want[index].Factors) ||
			value.Kind != relianceMapTermKind(want[index]) {
			return errors.New("evidence reliance map term identity differs from the frozen design")
		}
		if err := validateRelianceMapTermOutcomes(value.Outcomes, registeredCells); err != nil {
			return fmt.Errorf("validate reliance map term %q: %w", value.TermID, err)
		}
		if err := validateRelianceMapSelector(value, want[index]); err != nil {
			return err
		}
	}
	return nil
}

func relianceMapTermKind(term stats.FactorialTerm) RelianceMapTermKind {
	if len(term.Factors) == 1 {
		return RelianceMapMainEffect
	}
	return RelianceMapInteraction
}

func validateRelianceMapTermOutcomes(values []RelianceMapTermOutcome, registeredCells int) error {
	want := reliancePublicationOutcomeIDs()
	if len(values) != len(want) {
		return errors.New("term does not cover every frozen outcome")
	}
	for index, value := range values {
		if value.OutcomeID != want[index] || value.RegisteredCells != registeredCells ||
			value.EligibleObservations < 0 || value.ExcludedFromFit < 0 ||
			value.EligibleObservations+value.ExcludedFromFit != registeredCells {
			return errors.New("term outcome identity or denominator is invalid")
		}
		if err := validateRelianceMapOutcomeState(value); err != nil {
			return err
		}
	}
	return nil
}

func validateRelianceMapOutcomeState(value RelianceMapTermOutcome) error {
	switch value.Status {
	case RelianceMapMeasured:
		if value.Reason != "" || validateRelianceMapEstimate(value.Estimate) != nil {
			return errors.New("measured reliance map outcome lacks one valid estimate")
		}
	case RelianceMapInconclusive:
		if value.Reason == "" || value.Estimate != nil {
			return errors.New("inconclusive reliance map outcome lacks a reason or carries an estimate")
		}
	default:
		return errors.New("primary reliance map outcome has an unsupported publication status")
	}
	return nil
}

func validateRelianceMapEstimate(value *RelianceMapEstimate) error {
	if value == nil || !finiteRelianceNumber(value.Estimate) || !finiteRelianceNumber(value.StandardError) ||
		!finiteRelianceNumber(value.Lower) || !finiteRelianceNumber(value.Upper) ||
		!finiteRelianceNumber(value.AdjustedPValue) || value.StandardError < 0 || value.Lower > value.Upper ||
		value.AdjustedPValue < 0 || value.AdjustedPValue > 1 {
		return errors.New("reliance map estimate is invalid")
	}
	return nil
}

func finiteRelianceNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func reliancePublicationOutcomeIDs() []OutcomeID {
	return []OutcomeID{
		OutcomeAbstentionTransition, OutcomeConditionalMean, OutcomeConditionalVariance,
		OutcomeDecisionFlip, OutcomeSupportJaccard, OutcomeValidMass, OutcomeVisibleMass,
	}
}

func validateRelianceMapSelector(value RelianceMapTerm, term stats.FactorialTerm) error {
	if len(term.Factors) != 1 || term.Factors[0] == PresentationOrderTerm {
		if value.Selector != nil {
			return errors.New("evidence reliance interaction unexpectedly carries a selector projection")
		}
		return nil
	}
	if value.Selector == nil || value.Selector.AssignmentTargets <= 0 || len(value.Selector.Budgets) == 0 ||
		!validSelectorEffectStatus(value.Selector.EffectStatus) {
		return errors.New("evidence reliance main effect lacks selector evidence")
	}
	for index, budget := range value.Selector.Budgets {
		if index > 0 && budget.BudgetTokens <= value.Selector.Budgets[index-1].BudgetTokens {
			return errors.New("evidence reliance selector budgets are not canonical")
		}
		if err := validateRelianceSelectorBudget(budget, value.Selector.AssignmentTargets, value.Selector.EffectStatus); err != nil {
			return err
		}
	}
	return nil
}

func validSelectorEffectStatus(value SelectorEffectStatus) bool {
	return value == SelectorEffectDetected || value == SelectorEffectNotDetected || value == SelectorEffectInconclusive
}

func validateRelianceSelectorBudget(
	value RelianceMapSelectorBudget,
	assignmentTargets int,
	effectStatus SelectorEffectStatus,
) error {
	classified := value.ExactTargets + value.ChangedTargets + value.UnrenderedTargets + value.DroppedTargets
	if value.BudgetTokens <= 0 || value.AssignmentTargets != assignmentTargets || classified != assignmentTargets ||
		value.ExactTargets < 0 || value.ChangedTargets < 0 || value.UnrenderedTargets < 0 || value.DroppedTargets < 0 ||
		value.AssignedEventBytes < 0 || value.RetainedAssignedEventBytes < 0 || value.AssignedRenderedBytes < 0 ||
		value.RetainedAssignedRenderedBytes < 0 || value.RetainedAssignedEventBytes > value.AssignedEventBytes ||
		value.RetainedAssignedRenderedBytes > value.AssignedRenderedBytes {
		return errors.New("evidence reliance selector budget does not conserve targets or bytes")
	}
	return validateRelianceSelectorRiskFlags(value, effectStatus)
}

func validateRelianceSelectorRiskFlags(value RelianceMapSelectorBudget, status SelectorEffectStatus) error {
	want := []SelectorRiskFlag{}
	switch {
	case status == SelectorEffectInconclusive:
		want = []SelectorRiskFlag{SelectorRiskInconclusive}
	case status == SelectorEffectDetected && value.ChangedTargets+value.UnrenderedTargets+value.DroppedTargets > 0:
		want = []SelectorRiskFlag{SelectorRiskDetectedEffectNonexact}
	case status == SelectorEffectNotDetected && value.ExactTargets+value.ChangedTargets > 0:
		want = []SelectorRiskFlag{SelectorRiskUndetectedEffectRetained}
	}
	if !slices.Equal(value.RiskFlags, want) {
		return errors.New("evidence reliance selector risk flags differ from the declared effect status")
	}
	return nil
}

func validateRelianceDerivedProjections(value EvidenceRelianceMap) error {
	if !reflect.DeepEqual(value.ProfileDimensions, buildRelianceProfileDimensions(value)) ||
		!reflect.DeepEqual(value.PaperRows, buildReliancePaperRows(value)) ||
		!slices.Equal(value.PaperLimitations, reliancePaperLimitations()) ||
		!slices.Equal(value.AllowedClaims, relianceAllowedClaims()) ||
		!slices.Equal(value.ForbiddenClaims, relianceForbiddenClaims()) {
		return errors.New("evidence reliance profile, paper, or claim projection differs from the canonical map")
	}
	return nil
}

func evidenceRelianceMapDigest(value EvidenceRelianceMap) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
