package stress

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const constraintFloatTolerance = 1e-12

func EvaluateConstraint(expected ExpectedConstraint, originalValue, transformedValue *float64, originalState, transformedState string) (ConstraintResult, error) {
	if err := validateConstraints([]ExpectedConstraint{expected}); err != nil {
		return ConstraintResult{}, err
	}
	result := ConstraintResult{ConstraintID: expected.ID, Metric: expected.Metric, Operator: expected.Operator}
	if comparisonMetric(expected.Metric) {
		return ConstraintResult{}, errors.New("pair-comparison metric requires EvaluateComparisonConstraint")
	}
	if expected.Metric == MetricDecision {
		if originalValue != nil || transformedValue != nil || strings.TrimSpace(originalState) == "" || strings.TrimSpace(transformedState) == "" ||
			originalState != strings.TrimSpace(originalState) || transformedState != strings.TrimSpace(transformedState) {
			return ConstraintResult{}, errors.New("decision constraint evaluation requires two exact states and no numeric values")
		}
		result.OriginalState, result.TransformedState = originalState, transformedState
		if decisionConstraintSatisfied(expected, originalState, transformedState) {
			result.Status = ConstraintSatisfied
		} else {
			result.Status = ConstraintViolated
		}
		return result, nil
	}
	if originalValue == nil || transformedValue == nil || originalState != "" || transformedState != "" ||
		!finite(*originalValue) || !finite(*transformedValue) ||
		!validMovementObservation(expected.Metric, *originalValue) || !validMovementObservation(expected.Metric, *transformedValue) {
		return ConstraintResult{}, errors.New("numeric constraint evaluation requires two finite in-domain metric values and no states")
	}
	original, transformed := *originalValue, *transformedValue
	difference := transformed - original
	result.OriginalValue, result.TransformedValue, result.ObservedDifference = &original, &transformed, &difference
	if numericConstraintSatisfied(expected, difference) {
		result.Status = ConstraintSatisfied
	} else {
		result.Status = ConstraintViolated
	}
	return result, nil
}

func EvaluateComparisonConstraint(expected ExpectedConstraint, comparisonValue float64) (ConstraintResult, error) {
	if err := validateConstraints([]ExpectedConstraint{expected}); err != nil {
		return ConstraintResult{}, err
	}
	if !comparisonMetric(expected.Metric) || !finite(comparisonValue) || comparisonValue < 0 || comparisonValue > 1 {
		return ConstraintResult{}, errors.New("comparison constraint evaluation requires one finite pair metric")
	}
	difference := comparisonValue - *expected.TargetValue
	result := ConstraintResult{
		ConstraintID: expected.ID, Metric: expected.Metric, Operator: expected.Operator,
		ComparisonValue: &comparisonValue, ObservedDifference: &difference,
	}
	if comparisonConstraintSatisfied(expected, comparisonValue) {
		result.Status = ConstraintSatisfied
	} else {
		result.Status = ConstraintViolated
	}
	return result, nil
}

func SealResult(spec Relation, value Result) (Result, error) {
	if err := spec.Validate(); err != nil {
		return Result{}, err
	}
	value.SchemaVersion = ResultSchemaVersion
	value.CanonicalPolicy = CanonicalPolicy
	value.RelationDigest = spec.Digest
	value.ConstraintResults = cloneConstraintResults(value.ConstraintResults)
	value.DistributionComparisons = append([]TaggedDistributionComparison(nil), value.DistributionComparisons...)
	sort.Slice(value.ConstraintResults, func(left, right int) bool {
		return value.ConstraintResults[left].ConstraintID < value.ConstraintResults[right].ConstraintID
	})
	sort.Slice(value.DistributionComparisons, func(left, right int) bool {
		return value.DistributionComparisons[left].Tag < value.DistributionComparisons[right].Tag
	})
	value.Digest = ""
	digest, err := resultDigest(value)
	if err != nil {
		return Result{}, err
	}
	value.Digest = digest
	if err := value.ValidateAgainst(spec); err != nil {
		return Result{}, err
	}
	return value, nil
}

func (value Result) ValidateAgainst(spec Relation) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if value.SchemaVersion != ResultSchemaVersion || value.CanonicalPolicy != CanonicalPolicy || value.RelationDigest != spec.Digest ||
		!identifierPattern.MatchString(value.CaseID) || !identifierPattern.MatchString(value.TaskGroupID) ||
		!slices.Contains([]Outcome{OutcomeSatisfied, OutcomeViolated, OutcomeAbstained, OutcomeInvalid, OutcomeUnsupported, OutcomeProviderFailed, OutcomeInconclusive}, value.Outcome) ||
		value.PlannedRepetitions < 0 || value.CompletedRepetitions < 0 || value.CompletedRepetitions > value.PlannedRepetitions || value.ProviderCalls < 0 ||
		value.CapsuleDigest != "" && !validDigest(value.CapsuleDigest) {
		return errors.New("stress result identity, outcome, repetition, or capsule evidence is invalid")
	}
	if value.Outcome == OutcomeInvalid {
		if !validInvalidState(value.InvalidState) || value.CompletedRepetitions != 0 || value.ProviderCalls != 0 || len(value.ConstraintResults) != 0 ||
			len(value.DistributionComparisons) != 0 || value.StageComparison != nil {
			return errors.New("invalid stress result contains execution evidence or lacks one closed invalid state")
		}
		if err := validateInvalidAdmission(value); err != nil {
			return err
		}
		return validateResultDigest(value)
	}
	if value.InvalidState != "" {
		return errors.New("non-invalid stress result carries an invalid-state code")
	}
	if value.Outcome == OutcomeUnsupported {
		if value.CompletedRepetitions != 0 || value.ProviderCalls != 0 || len(value.ConstraintResults) != 0 || len(value.DistributionComparisons) != 0 ||
			value.StageComparison != nil || value.Admission != nil {
			return errors.New("unsupported stress result contains execution evidence or fabricated admission")
		}
		return validateResultDigest(value)
	}
	if value.Admission == nil {
		return errors.New("executed stress result lacks construct admission")
	}
	if err := value.Admission.Validate(); err != nil {
		return err
	}
	if value.Admission.CaseID != value.CaseID || value.Admission.Status == AdmissionHumanContradicted {
		return errors.New("stress result admission does not bind the case or permits a contradicted construct")
	}
	if spec.StatisticalFamily.Estimand == EstimandPrimaryCore && !value.Admission.PrimaryEligible {
		return errors.New("primary-core stress result lacks human-supported construct admission")
	}
	if spec.StatisticalFamily.Estimand != EstimandPrimaryCore && !value.Admission.SensitivityEligible {
		return errors.New("non-primary stress result lacks sensitivity eligibility")
	}
	if value.PlannedRepetitions != spec.Repeat.MaximumRepetitions {
		return errors.New("stress result planned repetitions differ from the registered relation")
	}
	if value.Outcome != OutcomeProviderFailed && value.CompletedRepetitions < spec.Repeat.MinimumRepetitions {
		return errors.New("stress result stopped before the registered minimum repetition count")
	}
	if err := validateConstraintResults(spec, value); err != nil {
		return err
	}
	if err := validateDistributionComparisons(value.DistributionComparisons); err != nil {
		return err
	}
	if value.StageComparison != nil {
		if err := value.StageComparison.Validate(); err != nil {
			return err
		}
		if value.StageComparison.RelationDigest != spec.Digest {
			return errors.New("stress result stage comparison belongs to another relation")
		}
	}
	return validateResultDigest(value)
}

func validateInvalidAdmission(value Result) error {
	if value.InvalidState != InvalidHumanContradicted {
		if value.Admission != nil {
			return errors.New("pre-admission invalid stress result carries construct admission")
		}
		return nil
	}
	if value.Admission == nil {
		return errors.New("human-contradicted invalid stress result lacks its construct admission")
	}
	if err := value.Admission.Validate(); err != nil {
		return err
	}
	if value.Admission.CaseID != value.CaseID || value.Admission.Status != AdmissionHumanContradicted {
		return errors.New("human-contradicted invalid stress result does not bind the contradicted case")
	}
	return nil
}

func validateConstraintResults(spec Relation, result Result) error {
	byID := make(map[string]ExpectedConstraint, len(spec.Constraints))
	for _, constraint := range spec.Constraints {
		byID[constraint.ID] = constraint
	}
	statuses := make(map[string]ConstraintStatus, len(result.ConstraintResults))
	for index, observed := range result.ConstraintResults {
		expected, exists := byID[observed.ConstraintID]
		if !exists || observed.Metric != expected.Metric || observed.Operator != expected.Operator || index > 0 && result.ConstraintResults[index-1].ConstraintID >= observed.ConstraintID {
			return errors.New("stress constraint results are unknown, mismatched, duplicated, or unsorted")
		}
		if !slices.Contains([]ConstraintStatus{ConstraintSatisfied, ConstraintViolated, ConstraintAbstained, ConstraintUnsupported, ConstraintInconclusive}, observed.Status) {
			return fmt.Errorf("stress constraint result %q has an invalid status", observed.ConstraintID)
		}
		if observed.Status == ConstraintSatisfied || observed.Status == ConstraintViolated {
			var recomputed ConstraintResult
			var err error
			if comparisonMetric(expected.Metric) {
				if observed.ComparisonValue == nil || observed.OriginalValue != nil || observed.TransformedValue != nil || observed.OriginalState != "" || observed.TransformedState != "" {
					return fmt.Errorf("pair-comparison result %q carries the wrong observation shape", observed.ConstraintID)
				}
				recomputed, err = EvaluateComparisonConstraint(expected, *observed.ComparisonValue)
			} else {
				if observed.ComparisonValue != nil {
					return fmt.Errorf("movement or decision result %q carries a pair-comparison value", observed.ConstraintID)
				}
				recomputed, err = EvaluateConstraint(expected, observed.OriginalValue, observed.TransformedValue, observed.OriginalState, observed.TransformedState)
			}
			if err != nil || recomputed.Status != observed.Status || !equalConstraintObservation(recomputed, observed) {
				return fmt.Errorf("stress constraint result %q does not reproduce from its observations", observed.ConstraintID)
			}
		} else if observed.OriginalValue != nil || observed.TransformedValue != nil || observed.ComparisonValue != nil || observed.ObservedDifference != nil || observed.OriginalState != "" || observed.TransformedState != "" {
			return fmt.Errorf("non-observed stress constraint result %q carries fabricated observations", observed.ConstraintID)
		}
		statuses[observed.ConstraintID] = observed.Status
	}
	for _, constraint := range spec.Constraints {
		if constraint.Required {
			if _, exists := statuses[constraint.ID]; !exists && result.Outcome != OutcomeProviderFailed {
				return fmt.Errorf("stress result omits required constraint %q", constraint.ID)
			}
		}
	}
	violated, abstained, unsupported, inconclusive := false, false, false, false
	for _, constraint := range spec.Constraints {
		if !constraint.Required {
			continue
		}
		switch statuses[constraint.ID] {
		case ConstraintViolated:
			violated = true
		case ConstraintAbstained:
			abstained = true
		case ConstraintUnsupported:
			unsupported = true
		case ConstraintInconclusive:
			inconclusive = true
		}
	}
	switch result.Outcome {
	case OutcomeSatisfied:
		if violated || abstained || unsupported || inconclusive {
			return errors.New("satisfied stress result has a non-satisfied required constraint")
		}
	case OutcomeViolated:
		if !violated {
			return errors.New("violated stress result has no violated required constraint")
		}
	case OutcomeAbstained:
		if violated || !abstained {
			return errors.New("abstained stress result lacks abstention or hides a violation")
		}
	case OutcomeInconclusive:
		if violated || !inconclusive {
			return errors.New("inconclusive stress result lacks inconclusive evidence or hides a violation")
		}
	case OutcomeProviderFailed:
		if violated {
			return errors.New("provider-failed stress result also claims a relation violation")
		}
	default:
		return errors.New("stress result outcome is invalid for executed constraint evidence")
	}
	return nil
}

func cloneConstraintResults(values []ConstraintResult) []ConstraintResult {
	result := append([]ConstraintResult(nil), values...)
	for index := range result {
		result[index].OriginalValue = cloneOptionalFloat(values[index].OriginalValue)
		result[index].TransformedValue = cloneOptionalFloat(values[index].TransformedValue)
		result[index].ComparisonValue = cloneOptionalFloat(values[index].ComparisonValue)
		result[index].ObservedDifference = cloneOptionalFloat(values[index].ObservedDifference)
	}
	return result
}

func cloneOptionalFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func validateDistributionComparisons(values []TaggedDistributionComparison) error {
	for index, value := range values {
		comparison := value.Comparison
		if strings.TrimSpace(value.Tag) == "" || value.Tag != strings.TrimSpace(value.Tag) || index > 0 && values[index-1].Tag >= value.Tag ||
			comparison.LeftTag != value.Tag || comparison.RightTag != value.Tag || verifier.ValidateScoreEvidenceComparison(comparison) != nil {
			return fmt.Errorf("stress distribution comparison %d is invalid", index)
		}
	}
	return nil
}

func decisionConstraintSatisfied(expected ExpectedConstraint, original, transformed string) bool {
	switch expected.Operator {
	case OperatorEqual:
		return original == transformed
	case OperatorNotEqual:
		return original != transformed
	case OperatorOriginalPreferred, OperatorTransformedPreferred:
		return original == expected.TargetState && transformed == expected.TargetState
	default:
		return false
	}
}

func numericConstraintSatisfied(expected ExpectedConstraint, difference float64) bool {
	switch expected.Operator {
	case OperatorEqual:
		return math.Abs(difference) <= expected.AbsoluteTolerance+constraintFloatTolerance
	case OperatorNotEqual:
		return math.Abs(difference) > expected.AbsoluteTolerance+constraintFloatTolerance
	case OperatorLessOrEqual:
		return difference <= -expected.MinimumEffect+expected.AbsoluteTolerance+constraintFloatTolerance
	case OperatorGreaterOrEqual:
		return difference >= expected.MinimumEffect-expected.AbsoluteTolerance-constraintFloatTolerance
	case OperatorOriginalPreferred:
		return expected.Metric == MetricRank && difference >= expected.MinimumEffect-expected.AbsoluteTolerance-constraintFloatTolerance
	case OperatorTransformedPreferred:
		return expected.Metric == MetricRank && difference <= -expected.MinimumEffect+expected.AbsoluteTolerance+constraintFloatTolerance
	default:
		return false
	}
}

func comparisonConstraintSatisfied(expected ExpectedConstraint, observed float64) bool {
	difference := observed - *expected.TargetValue
	switch expected.Operator {
	case OperatorEqual:
		return math.Abs(difference) <= expected.AbsoluteTolerance+constraintFloatTolerance
	case OperatorNotEqual:
		return math.Abs(difference) > expected.AbsoluteTolerance+constraintFloatTolerance
	case OperatorLessOrEqual:
		return observed <= *expected.TargetValue+expected.AbsoluteTolerance+constraintFloatTolerance
	case OperatorGreaterOrEqual:
		return observed >= *expected.TargetValue-expected.AbsoluteTolerance-constraintFloatTolerance
	default:
		return false
	}
}

func validMovementObservation(metric Metric, value float64) bool {
	switch metric {
	case MetricRank:
		return value >= 1 && value <= 10 && math.Trunc(value) == value
	case MetricConditionalScore, MetricVisibleMass, MetricValidMass, MetricUnobservedMass:
		return value >= 0 && value <= 1
	case MetricConditionalVariance:
		return value >= 0 && value <= 0.25
	default:
		return false
	}
}

func equalConstraintObservation(left, right ConstraintResult) bool {
	if left.ConstraintID != right.ConstraintID || left.Metric != right.Metric || left.Operator != right.Operator || left.Status != right.Status ||
		left.OriginalState != right.OriginalState || left.TransformedState != right.TransformedState {
		return false
	}
	return equalOptionalFloat(left.OriginalValue, right.OriginalValue) && equalOptionalFloat(left.TransformedValue, right.TransformedValue) &&
		equalOptionalFloat(left.ComparisonValue, right.ComparisonValue) && equalOptionalFloat(left.ObservedDifference, right.ObservedDifference)
}

func equalOptionalFloat(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return math.Abs(*left-*right) <= constraintFloatTolerance
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validateResultDigest(value Result) error {
	expected, err := resultDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress result digest is invalid")
	}
	return nil
}

func resultDigest(value Result) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
