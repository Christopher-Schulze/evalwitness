package verifier

import (
	"errors"
	"math"
	"strings"
)

const ScoreEvidenceComparisonSchemaVersion = "evalwitness.score-evidence-comparison.v1"

type MissingTailBounds struct {
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

type ScoreEvidenceComparison struct {
	SchemaVersion                      string            `json:"schema_version"`
	LeftTag                            string            `json:"left_tag"`
	RightTag                           string            `json:"right_tag"`
	CommonSupport                      []string          `json:"common_support"`
	SupportUnion                       []string          `json:"support_union"`
	SupportJaccard                     float64           `json:"support_jaccard"`
	ProbabilityOverlap                 float64           `json:"probability_overlap"`
	CommonSupportConditionalDivergence *float64          `json:"common_support_conditional_divergence,omitempty"`
	VisibleMassMovement                float64           `json:"visible_mass_movement"`
	ValidScoreMassMovement             float64           `json:"valid_score_mass_movement"`
	ConditionalScoreMovement           *float64          `json:"conditional_score_movement,omitempty"`
	ConditionalVarianceMovement        *float64          `json:"conditional_variance_movement,omitempty"`
	LeftMissingTailBounds              MissingTailBounds `json:"left_missing_tail_bounds"`
	RightMissingTailBounds             MissingTailBounds `json:"right_missing_tail_bounds"`
	MissingTailIntervalsOverlap        bool              `json:"missing_tail_intervals_overlap"`
}

func CompareScoreEvidence(left, right ScoreEvidence) ScoreEvidenceComparison {
	leftSupport := supportProbabilities(left)
	rightSupport := supportProbabilities(right)
	comparison := ScoreEvidenceComparison{
		SchemaVersion:          ScoreEvidenceComparisonSchemaVersion,
		LeftTag:                left.Tag,
		RightTag:               right.Tag,
		CommonSupport:          []string{},
		SupportUnion:           []string{},
		VisibleMassMovement:    right.VisibleProbabilityMass - left.VisibleProbabilityMass,
		ValidScoreMassMovement: right.ValidScoreMass - left.ValidScoreMass,
		LeftMissingTailBounds:  missingTailBounds(left),
		RightMissingTailBounds: missingTailBounds(right),
	}
	for letter := range leftSupport {
		comparison.SupportUnion = append(comparison.SupportUnion, letter)
		if _, ok := rightSupport[letter]; ok {
			comparison.CommonSupport = append(comparison.CommonSupport, letter)
		}
	}
	for letter := range rightSupport {
		if _, ok := leftSupport[letter]; !ok {
			comparison.SupportUnion = append(comparison.SupportUnion, letter)
		}
	}
	sortScoreLetters(comparison.CommonSupport)
	sortScoreLetters(comparison.SupportUnion)
	if len(comparison.SupportUnion) > 0 {
		comparison.SupportJaccard = float64(len(comparison.CommonSupport)) / float64(len(comparison.SupportUnion))
	}
	for _, letter := range comparison.CommonSupport {
		comparison.ProbabilityOverlap += math.Min(leftSupport[letter], rightSupport[letter])
	}
	if divergence, ok := commonSupportDivergence(comparison.CommonSupport, leftSupport, rightSupport); ok {
		comparison.CommonSupportConditionalDivergence = &divergence
	}
	if left.ConditionalExpectedScore != nil && right.ConditionalExpectedScore != nil {
		movement := *right.ConditionalExpectedScore - *left.ConditionalExpectedScore
		comparison.ConditionalScoreMovement = &movement
	}
	if left.ConditionalVariance != nil && right.ConditionalVariance != nil {
		movement := *right.ConditionalVariance - *left.ConditionalVariance
		comparison.ConditionalVarianceMovement = &movement
	}
	comparison.MissingTailIntervalsOverlap = comparison.LeftMissingTailBounds.Lower <= comparison.RightMissingTailBounds.Upper &&
		comparison.RightMissingTailBounds.Lower <= comparison.LeftMissingTailBounds.Upper
	return comparison
}

func ValidateScoreEvidenceComparison(value ScoreEvidenceComparison) error {
	if value.SchemaVersion != ScoreEvidenceComparisonSchemaVersion || strings.TrimSpace(value.LeftTag) == "" || value.LeftTag != strings.TrimSpace(value.LeftTag) ||
		strings.TrimSpace(value.RightTag) == "" || value.RightTag != strings.TrimSpace(value.RightTag) ||
		!validCanonicalSupport(value.CommonSupport) || !validCanonicalSupport(value.SupportUnion) || !supportSubset(value.CommonSupport, value.SupportUnion) {
		return errors.New("score-evidence comparison identity or support is invalid")
	}
	expectedJaccard := 0.0
	if len(value.SupportUnion) > 0 {
		expectedJaccard = float64(len(value.CommonSupport)) / float64(len(value.SupportUnion))
	}
	if !finiteComparisonMetric(value.SupportJaccard, 0, 1) || math.Abs(value.SupportJaccard-expectedJaccard) > 1e-12 ||
		!finiteComparisonMetric(value.ProbabilityOverlap, 0, 1) || !finiteComparisonMetric(value.VisibleMassMovement, -1, 1) ||
		!finiteComparisonMetric(value.ValidScoreMassMovement, -1, 1) || !validOptionalComparisonMetric(value.CommonSupportConditionalDivergence, 0, 1) ||
		!validOptionalComparisonMetric(value.ConditionalScoreMovement, -1, 1) || !validOptionalComparisonMetric(value.ConditionalVarianceMovement, -0.25, 0.25) {
		return errors.New("score-evidence comparison metric is invalid")
	}
	if len(value.CommonSupport) == 0 && value.CommonSupportConditionalDivergence != nil {
		return errors.New("score-evidence comparison has divergence without common support")
	}
	if !validMissingTailBounds(value.LeftMissingTailBounds) || !validMissingTailBounds(value.RightMissingTailBounds) {
		return errors.New("score-evidence comparison missing-tail bounds are invalid")
	}
	expectedOverlap := value.LeftMissingTailBounds.Lower <= value.RightMissingTailBounds.Upper &&
		value.RightMissingTailBounds.Lower <= value.LeftMissingTailBounds.Upper
	if value.MissingTailIntervalsOverlap != expectedOverlap {
		return errors.New("score-evidence comparison missing-tail overlap is invalid")
	}
	return nil
}

func supportProbabilities(evidence ScoreEvidence) map[string]float64 {
	out := make(map[string]float64, len(evidence.Support))
	for _, support := range evidence.Support {
		out[support.Letter] = support.Probability
	}
	return out
}

func commonSupportDivergence(common []string, left, right map[string]float64) (float64, bool) {
	leftMass := 0.0
	rightMass := 0.0
	for _, letter := range common {
		leftMass += left[letter]
		rightMass += right[letter]
	}
	if leftMass == 0 || rightMass == 0 {
		return 0, false
	}
	totalVariation := 0.0
	for _, letter := range common {
		totalVariation += math.Abs(left[letter]/leftMass - right[letter]/rightMass)
	}
	return totalVariation / 2, true
}

func missingTailBounds(evidence ScoreEvidence) MissingTailBounds {
	knownMass := evidence.ValidScoreMass
	unknownMass := evidence.UnobservedProbabilityMass
	denominator := knownMass + unknownMass
	if denominator <= 0 || evidence.ConditionalExpectedScore == nil {
		return MissingTailBounds{Lower: 0, Upper: 1}
	}
	knownWeightedScore := knownMass * *evidence.ConditionalExpectedScore
	return MissingTailBounds{
		Lower: knownWeightedScore / denominator,
		Upper: (knownWeightedScore + unknownMass) / denominator,
	}
}

func sortScoreLetters(letters []string) {
	for left := 0; left < len(letters); left++ {
		for right := left + 1; right < len(letters); right++ {
			leftValue, _ := TokenValue(letters[left])
			rightValue, _ := TokenValue(letters[right])
			if rightValue > leftValue {
				letters[left], letters[right] = letters[right], letters[left]
			}
		}
	}
}

func validCanonicalSupport(values []string) bool {
	previous := ValueMax + 1
	for _, value := range values {
		score, ok := TokenValue(value)
		if !ok || value != strings.ToUpper(value) || float64(score) >= previous {
			return false
		}
		previous = float64(score)
	}
	return true
}

func supportSubset(subset, superset []string) bool {
	available := make(map[string]struct{}, len(superset))
	for _, value := range superset {
		available[value] = struct{}{}
	}
	for _, value := range subset {
		if _, exists := available[value]; !exists {
			return false
		}
	}
	return true
}

func finiteComparisonMetric(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

func validOptionalComparisonMetric(value *float64, minimum, maximum float64) bool {
	return value == nil || finiteComparisonMetric(*value, minimum, maximum)
}

func validMissingTailBounds(value MissingTailBounds) bool {
	return finiteComparisonMetric(value.Lower, 0, 1) && finiteComparisonMetric(value.Upper, 0, 1) && value.Lower <= value.Upper
}
