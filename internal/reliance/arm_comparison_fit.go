package reliance

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func constructRelianceArmComparison(
	preregistration Preregistration,
	preflight ReliancePreflight,
	states map[string]relianceArmComparisonState,
	specs []RelianceArmContrastSpec,
) (RelianceArmComparison, error) {
	familySize := preregistration.Multiplicity.FamilySize * len(specs)
	contrasts := make([]RelianceArmContrast, len(specs))
	for index, spec := range specs {
		contrast, err := buildRelianceArmContrast(spec, states, preregistration, familySize)
		if err != nil {
			return RelianceArmComparison{}, err
		}
		contrasts[index] = contrast
	}
	value := RelianceArmComparison{
		SchemaVersion: RelianceArmComparisonSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		ContrastPolicyVersion: RelianceArmContrastPolicyVersion, PreregistrationDigest: preregistration.Digest,
		PreflightDigest: preflight.Digest, MultiplicityMethod: preregistration.Multiplicity.Method,
		MultiplicityFamilySize: familySize, Arms: orderedRelianceArmSummaries(states), Contrasts: contrasts,
		ProviderCalls: 0, NetworkRequired: false,
	}
	digest, err := relianceArmComparisonDigest(value)
	if err != nil {
		return RelianceArmComparison{}, err
	}
	value.Digest = digest
	return value, nil
}

func orderedRelianceArmSummaries(states map[string]relianceArmComparisonState) []RelianceArmSummary {
	result := make([]RelianceArmSummary, 0, len(states))
	for _, state := range states {
		result = append(result, state.summary)
	}
	slices.SortFunc(result, func(left, right RelianceArmSummary) int {
		return strings.Compare(left.ArmID, right.ArmID)
	})
	return result
}

func buildRelianceArmContrast(
	spec RelianceArmContrastSpec,
	states map[string]relianceArmComparisonState,
	preregistration Preregistration,
	familySize int,
) (RelianceArmContrast, error) {
	reference, comparator := states[spec.ReferenceArmID], states[spec.ComparatorArmID]
	dimensions := relianceArmChangedDimensions(reference.summary, comparator.summary)
	value := RelianceArmContrast{
		ContrastID: spec.ContrastID, Kind: spec.Kind, ReferenceArmID: spec.ReferenceArmID,
		ComparatorArmID: spec.ComparatorArmID, Direction: RelianceArmContrastDirection,
		ChangedDimensions: dimensions, Support: RelianceArmContrastUnsupported,
		PairingStatusCounts: []ReliancePairingStatusCount{}, OutcomeFits: []RelianceArmContrastOutcomeFit{},
	}
	if reason := relianceArmContrastUnsupportedReason(spec.Kind, reference, comparator, dimensions); reason != "" {
		value.Reason = reason
		return value, nil
	}
	counts, observations, err := pairedRelianceObservations(reference.corpus, comparator.corpus, preregistration)
	if err != nil {
		return RelianceArmContrast{}, err
	}
	value.Support = RelianceArmContrastSupported
	value.RegisteredPairs = reference.corpus.RegisteredCells
	value.PairingStatusCounts = counts
	value.EligiblePairs = pairingStatusCells(counts, ReliancePairPaired)
	value.OutcomeFits, err = fitRelianceArmContrastOutcomes(preregistration, observations, value.RegisteredPairs, familySize)
	return value, err
}

func relianceArmContrastUnsupportedReason(
	kind RelianceArmContrastKind,
	reference relianceArmComparisonState,
	comparator relianceArmComparisonState,
	dimensions []string,
) string {
	if !sameRelianceArmSourcePanel(reference, comparator) {
		return "source_panel_or_study_mismatch"
	}
	if kind == RelianceContrastEvidencePolicy && !sameRelianceArmResponseRecords(reference, comparator) {
		return "evidence_policy_response_records_mismatch"
	}
	if kind == RelianceContrastModelFamily && (reference.summary.ModelIdentityStatus != RelianceModelIdentityNamedFamilyEvidenceBound ||
		comparator.summary.ModelIdentityStatus != RelianceModelIdentityNamedFamilyEvidenceBound) {
		return "model_family_identity_evidence_not_bound"
	}
	if !slices.Equal(dimensions, expectedRelianceContrastDimensions(kind)) {
		if len(dimensions) == 0 {
			return "declared_contrast_has_no_changed_dimension"
		}
		return "declared_contrast_dimension_mismatch:" + strings.Join(dimensions, ",")
	}
	return ""
}

func sameRelianceArmResponseRecords(left, right relianceArmComparisonState) bool {
	for _, sourceTask := range left.registration.SourceTasks {
		leftSource, leftFound := left.replaySources[sourceTask.SourceTaskID]
		rightSource, rightFound := right.replaySources[sourceTask.SourceTaskID]
		if leftFound && rightFound && !reflect.DeepEqual(leftSource, rightSource) {
			return false
		}
	}
	return true
}

func sameRelianceArmSourcePanel(left, right relianceArmComparisonState) bool {
	leftRegistration, rightRegistration := left.registration, right.registration
	return leftRegistration.StudyManifestDigest == rightRegistration.StudyManifestDigest &&
		leftRegistration.PreregistrationDigest == rightRegistration.PreregistrationDigest &&
		leftRegistration.PreflightDigest == rightRegistration.PreflightDigest &&
		leftRegistration.AssignmentAlgorithm == rightRegistration.AssignmentAlgorithm &&
		leftRegistration.CellsPerTask == rightRegistration.CellsPerTask &&
		leftRegistration.RegisteredCells == rightRegistration.RegisteredCells &&
		reflect.DeepEqual(leftRegistration.SourceTasks, rightRegistration.SourceTasks)
}

func relianceArmChangedDimensions(left, right RelianceArmSummary) []string {
	result := make([]string, 0, 7)
	fields := []struct {
		name        string
		left, right string
	}{
		{"criterion", left.Arm.CriterionID, right.Arm.CriterionID},
		{"entrypoint", left.Arm.Entrypoint, right.Arm.Entrypoint},
		{"evidence_policy", string(left.Arm.EvidencePolicy), string(right.Arm.EvidencePolicy)},
		{"model_family", left.ModelFamilyID, right.ModelFamilyID},
		{"model_identity_status", string(left.ModelIdentityStatus), string(right.ModelIdentityStatus)},
		{"provider", left.Arm.ProviderID, right.Arm.ProviderID},
		{"requested_model", left.Arm.RequestedModel, right.Arm.RequestedModel},
		{"route", left.Arm.RouteID, right.Arm.RouteID},
		{"score_tag", left.Arm.ScoreTag, right.Arm.ScoreTag},
	}
	for _, field := range fields {
		if field.left != field.right {
			result = append(result, field.name)
		}
	}
	return result
}

func expectedRelianceContrastDimensions(kind RelianceArmContrastKind) []string {
	switch kind {
	case RelianceContrastEvidencePolicy:
		return []string{"evidence_policy"}
	case RelianceContrastEntrypoint:
		return []string{"entrypoint"}
	case RelianceContrastModelFamily:
		return []string{"model_family", "requested_model", "route"}
	case RelianceContrastProvider:
		return []string{"provider", "route"}
	default:
		return []string{"route"}
	}
}

func pairedRelianceObservations(
	reference RelianceAnalysisCorpus,
	comparator RelianceAnalysisCorpus,
	preregistration Preregistration,
) ([]ReliancePairingStatusCount, [][]stats.FactorialObservation, error) {
	observations := make([][]stats.FactorialObservation, len(preregistration.PrimaryOutcomes))
	counts := make(map[ReliancePairingStatus]int)
	for index, referenceCell := range reference.Cells {
		comparatorCell := comparator.Cells[index]
		if referenceCell.SourceTaskID != comparatorCell.SourceTaskID || referenceCell.CellIndex != comparatorCell.CellIndex ||
			!reflect.DeepEqual(referenceCell.Levels, comparatorCell.Levels) {
			return nil, nil, errors.New("reliance arm comparison cells are not exactly paired")
		}
		status := reliancePairingStatus(referenceCell, comparatorCell, len(preregistration.PrimaryOutcomes))
		if status == ReliancePairPaired && (referenceCell.InterventionCellDigest != comparatorCell.InterventionCellDigest ||
			referenceCell.PresentationDigest != comparatorCell.PresentationDigest) {
			return nil, nil, errors.New("reliance arm comparison paired outcomes use different intervention or presentation cells")
		}
		counts[status]++
		if status == ReliancePairPaired {
			appendPairedRelianceOutcomes(observations, referenceCell, comparatorCell)
		}
	}
	return orderedReliancePairingCounts(counts), observations, nil
}

func reliancePairingStatus(
	reference RelianceAnalysisCell,
	comparator RelianceAnalysisCell,
	outcomes int,
) ReliancePairingStatus {
	referencePresent := len(reference.OutcomeValues) == outcomes
	comparatorPresent := len(comparator.OutcomeValues) == outcomes
	switch {
	case referencePresent && comparatorPresent:
		return ReliancePairPaired
	case referencePresent:
		return ReliancePairReferenceOnly
	case comparatorPresent:
		return ReliancePairComparatorOnly
	default:
		return ReliancePairBothMissing
	}
}

func appendPairedRelianceOutcomes(
	observations [][]stats.FactorialObservation,
	reference RelianceAnalysisCell,
	comparator RelianceAnalysisCell,
) {
	for outcomeIndex := range observations {
		observations[outcomeIndex] = append(observations[outcomeIndex], stats.FactorialObservation{
			ObservationID: reference.ObservationID, ClusterID: reference.SourceTaskID,
			Levels:  slices.Clone(reference.Levels),
			Outcome: comparator.OutcomeValues[outcomeIndex].Value - reference.OutcomeValues[outcomeIndex].Value,
		})
	}
}

func orderedReliancePairingCounts(counts map[ReliancePairingStatus]int) []ReliancePairingStatusCount {
	statuses := []ReliancePairingStatus{
		ReliancePairBothMissing, ReliancePairComparatorOnly, ReliancePairPaired, ReliancePairReferenceOnly,
	}
	result := make([]ReliancePairingStatusCount, len(statuses))
	for index, status := range statuses {
		result[index] = ReliancePairingStatusCount{Status: status, Cells: counts[status]}
	}
	return result
}

func pairingStatusCells(counts []ReliancePairingStatusCount, status ReliancePairingStatus) int {
	for _, count := range counts {
		if count.Status == status {
			return count.Cells
		}
	}
	return 0
}

func fitRelianceArmContrastOutcomes(
	preregistration Preregistration,
	observations [][]stats.FactorialObservation,
	registeredPairs int,
	familySize int,
) ([]RelianceArmContrastOutcomeFit, error) {
	result := make([]RelianceArmContrastOutcomeFit, len(preregistration.PrimaryOutcomes))
	for index, outcome := range preregistration.PrimaryOutcomes {
		fit, err := fitRelianceArmContrastOutcome(outcome.OutcomeID, observations[index], registeredPairs, familySize)
		if err != nil {
			return nil, err
		}
		result[index] = fit
	}
	return result, nil
}

func fitRelianceArmContrastOutcome(
	outcomeID OutcomeID,
	observations []stats.FactorialObservation,
	registeredPairs int,
	familySize int,
) (RelianceArmContrastOutcomeFit, error) {
	result := RelianceArmContrastOutcomeFit{
		OutcomeID: outcomeID, RegisteredPairs: registeredPairs, EligiblePairs: len(observations),
		ExcludedFromFit: registeredPairs - len(observations),
	}
	if len(observations) == 0 {
		result.Status = RelianceFitInconclusive
		result.Reason = "paired clustered factorial fit unavailable: no paired outcome-bearing cells"
		return result, nil
	}
	fit, err := stats.FitClusteredFactorial(referenceFactorialTerms(), observations, referenceNominalAlpha, familySize)
	if err != nil {
		if !errors.Is(err, stats.ErrFactorialNotEstimable) {
			return RelianceArmContrastOutcomeFit{}, fmt.Errorf("fit reliance arm contrast outcome %q: %w", outcomeID, err)
		}
		result.Status = RelianceFitInconclusive
		result.Reason = "paired clustered factorial fit unavailable: " + err.Error()
		return result, nil
	}
	result.Status, result.Fit = RelianceFitMeasured, &fit
	return result, nil
}

func relianceArmComparisonDigest(value RelianceArmComparison) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
