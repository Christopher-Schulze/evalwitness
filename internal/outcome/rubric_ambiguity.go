package outcome

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"time"
)

const RubricAmbiguitySchemaVersion = "evalwitness.outcome-rubric-ambiguity-analysis.v1"

type RubricAxis string

const (
	RubricAxisTaskSatisfaction     RubricAxis = "task_satisfaction"
	RubricAxisTechnicalCorrectness RubricAxis = "technical_correctness"
	RubricAxisVerificationQuality  RubricAxis = "verification_quality"
	RubricAxisHarmfulSideEffects   RubricAxis = "harmful_side_effects"
	RubricAxisEvidenceSufficiency  RubricAxis = "evidence_sufficiency"
)

type RatingCount struct {
	Rating AxisRating `json:"rating"`
	Count  int        `json:"count"`
}

type RubricAxisMetric struct {
	Axis                 RubricAxis    `json:"axis"`
	Comparisons          int           `json:"comparisons"`
	Disagreements        int           `json:"disagreements"`
	DisagreementRate     float64       `json:"disagreement_rate"`
	DisagreementInterval Interval      `json:"disagreement_interval"`
	UnclearRatings       int           `json:"unclear_ratings"`
	UnclearRate          float64       `json:"unclear_rate"`
	UnclearInterval      Interval      `json:"unclear_interval"`
	LeftPrevalence       []RatingCount `json:"left_prevalence"`
	RightPrevalence      []RatingCount `json:"right_prevalence"`
}

type RubricAmbiguityItem struct {
	PacketID                   string       `json:"packet_id"`
	TaskGroupID                string       `json:"task_group_id"`
	LeftLabel                  Label        `json:"left_label"`
	RightLabel                 Label        `json:"right_label"`
	PrimaryOutcomeDisagreement bool         `json:"primary_outcome_disagreement"`
	DisagreementAxes           []RubricAxis `json:"disagreement_axes"`
	ReasonJaccardDistance      float64      `json:"reason_jaccard_distance"`
	ReasonExactMatch           bool         `json:"reason_exact_match"`
	ReasonZeroOverlap          bool         `json:"reason_zero_overlap"`
}

type RubricAmbiguityAnalysis struct {
	SchemaVersion                      string                      `json:"schema_version"`
	CanonicalPolicy                    string                      `json:"canonical_policy"`
	BundleDigest                       string                      `json:"bundle_digest"`
	PrimaryCommitments                 []ReviewCommitmentReference `json:"primary_commitments"`
	Packets                            int                         `json:"packets"`
	LabelObservations                  int                         `json:"label_observations"`
	AxisComparisons                    int                         `json:"axis_comparisons"`
	PrimaryOutcomeDisagreements        int                         `json:"primary_outcome_disagreements"`
	PrimaryOutcomeDisagreementRate     float64                     `json:"primary_outcome_disagreement_rate"`
	PrimaryOutcomeDisagreementInterval Interval                    `json:"primary_outcome_disagreement_interval"`
	AnyAxisDisagreements               int                         `json:"any_axis_disagreements"`
	AnyAxisDisagreementRate            float64                     `json:"any_axis_disagreement_rate"`
	AnyAxisDisagreementInterval        Interval                    `json:"any_axis_disagreement_interval"`
	AxisMetrics                        []RubricAxisMetric          `json:"axis_metrics"`
	PrimaryStatePrevalence             []StateCount                `json:"primary_state_prevalence"`
	IndeterminateLabels                int                         `json:"indeterminate_labels"`
	IndeterminateRate                  float64                     `json:"indeterminate_rate"`
	IndeterminateInterval              Interval                    `json:"indeterminate_interval"`
	UnclearRatings                     int                         `json:"unclear_ratings"`
	UnclearRate                        float64                     `json:"unclear_rate"`
	UnclearInterval                    Interval                    `json:"unclear_interval"`
	ExactReasonMatches                 int                         `json:"exact_reason_matches"`
	ExactReasonMatchRate               float64                     `json:"exact_reason_match_rate"`
	ExactReasonMatchInterval           Interval                    `json:"exact_reason_match_interval"`
	ZeroReasonOverlaps                 int                         `json:"zero_reason_overlaps"`
	ZeroReasonOverlapRate              float64                     `json:"zero_reason_overlap_rate"`
	ZeroReasonOverlapInterval          Interval                    `json:"zero_reason_overlap_interval"`
	MeanReasonJaccardDistance          float64                     `json:"mean_reason_jaccard_distance"`
	Items                              []RubricAmbiguityItem       `json:"items"`
	AnalyzedAt                         string                      `json:"analyzed_at"`
	Limitations                        []string                    `json:"limitations"`
	Digest                             string                      `json:"digest"`
}

func BuildRubricAmbiguityAnalysis(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, analyzedAt string) (RubricAmbiguityAnalysis, error) {
	if err := validatePrimaryReviewPair(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch); err != nil {
		return RubricAmbiguityAnalysis{}, err
	}
	latestCommit, err := laterTimestamp(leftBatch.CommittedAt, rightBatch.CommittedAt)
	if err != nil {
		return RubricAmbiguityAnalysis{}, err
	}
	if err := requireStrictlyAfter("rubric ambiguity analysis", analyzedAt, latestCommit); err != nil {
		return RubricAmbiguityAnalysis{}, err
	}
	leftAssignment, leftBatch, rightAssignment, rightBatch = primaryReviewsBySlot(leftAssignment, leftBatch, rightAssignment, rightBatch)
	leftLabels, rightLabels := labelsByPacket(leftBatch.Labels), labelsByPacket(rightBatch.Labels)
	items := make([]RubricAmbiguityItem, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		items = append(items, buildRubricAmbiguityItem(item, leftLabels[item.Packet.PacketID], rightLabels[item.Packet.PacketID]))
	}
	analysis := summarizeRubricAmbiguityItems(items)
	analysis.BundleDigest = bundle.Digest
	analysis.PrimaryCommitments = primaryCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch)
	analysis.AnalyzedAt = analyzedAt
	analysis.Limitations = rubricAmbiguityLimitations()
	return SealRubricAmbiguityAnalysis(analysis)
}

func SealRubricAmbiguityAnalysis(analysis RubricAmbiguityAnalysis) (RubricAmbiguityAnalysis, error) {
	analysis.SchemaVersion, analysis.CanonicalPolicy, analysis.Digest = RubricAmbiguitySchemaVersion, CanonicalPolicy, ""
	digest, err := rubricAmbiguityDigest(analysis)
	if err != nil {
		return RubricAmbiguityAnalysis{}, err
	}
	analysis.Digest = digest
	return analysis, analysis.Validate()
}

func (analysis RubricAmbiguityAnalysis) Validate() error {
	if analysis.SchemaVersion != RubricAmbiguitySchemaVersion || analysis.CanonicalPolicy != CanonicalPolicy || !validDigest(analysis.BundleDigest) ||
		analysis.Packets < 1 || analysis.LabelObservations != analysis.Packets*2 || analysis.AxisComparisons != analysis.Packets*len(rubricAxes()) || len(analysis.Items) != analysis.Packets {
		return errors.New("rubric ambiguity analysis identity, binding, or denominators are invalid")
	}
	if err := validateReviewCommitments(analysis.PrimaryCommitments); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, analysis.AnalyzedAt); err != nil {
		return errors.New("rubric ambiguity analysis timestamp must be RFC3339")
	}
	if !equalStrings(analysis.Limitations, rubricAmbiguityLimitations()) {
		return errors.New("rubric ambiguity analysis limitations are invalid")
	}
	for index, item := range analysis.Items {
		if err := validateRubricAmbiguityItem(item); err != nil {
			return fmt.Errorf("rubric ambiguity item %d: %w", index, err)
		}
		if index > 0 && analysis.Items[index-1].PacketID >= item.PacketID {
			return errors.New("rubric ambiguity items must be unique and sorted by packet")
		}
	}
	recomputed := summarizeRubricAmbiguityItems(analysis.Items)
	if !equalRubricAmbiguitySummary(analysis, recomputed) {
		return errors.New("rubric ambiguity statistics are inconsistent with embedded labels")
	}
	if !boundedProbability(analysis.PrimaryOutcomeDisagreementRate) || !validInterval(analysis.PrimaryOutcomeDisagreementInterval, 0, 1) ||
		!boundedProbability(analysis.AnyAxisDisagreementRate) || !validInterval(analysis.AnyAxisDisagreementInterval, 0, 1) ||
		!boundedProbability(analysis.IndeterminateRate) || !validInterval(analysis.IndeterminateInterval, 0, 1) ||
		!boundedProbability(analysis.UnclearRate) || !validInterval(analysis.UnclearInterval, 0, 1) ||
		!boundedProbability(analysis.ExactReasonMatchRate) || !validInterval(analysis.ExactReasonMatchInterval, 0, 1) ||
		!boundedProbability(analysis.ZeroReasonOverlapRate) || !validInterval(analysis.ZeroReasonOverlapInterval, 0, 1) ||
		!boundedProbability(analysis.MeanReasonJaccardDistance) {
		return errors.New("rubric ambiguity rates or intervals are invalid")
	}
	expected, err := rubricAmbiguityDigest(analysis)
	if err != nil || analysis.Digest != expected {
		return errors.New("rubric ambiguity analysis digest is invalid")
	}
	return nil
}

func validateRubricAmbiguityForLedger(analysis RubricAmbiguityAnalysis, bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, reveal MappingReveal) error {
	if err := analysis.Validate(); err != nil {
		return err
	}
	if analysis.BundleDigest != bundle.Digest || !equalReviewCommitments(analysis.PrimaryCommitments, primaryCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch)) {
		return errors.New("adjudication ledger rubric ambiguity analysis does not bind its bundle and primary commitments")
	}
	if err := exactPacketCoverage(reviewBundlePacketIDs(bundle), rubricAmbiguityPacketIDs(analysis.Items)); err != nil {
		return err
	}
	_, leftBatch, _, rightBatch = primaryReviewsBySlot(leftAssignment, leftBatch, rightAssignment, rightBatch)
	leftLabels, rightLabels := labelsByPacket(leftBatch.Labels), labelsByPacket(rightBatch.Labels)
	for _, item := range analysis.Items {
		if item.LeftLabel.Digest != leftLabels[item.PacketID].Digest || item.RightLabel.Digest != rightLabels[item.PacketID].Digest {
			return errors.New("adjudication ledger rubric ambiguity items do not reproduce the committed primary labels")
		}
	}
	return requireStrictlyAfter("mapping reveal", reveal.RevealedAt, analysis.AnalyzedAt)
}

func DecodeRubricAmbiguityAnalysis(reader io.Reader) (RubricAmbiguityAnalysis, error) {
	var value RubricAmbiguityAnalysis
	if err := decodeStrict(reader, &value); err != nil {
		return RubricAmbiguityAnalysis{}, fmt.Errorf("decode rubric ambiguity analysis: %w", err)
	}
	return value, value.Validate()
}

func buildRubricAmbiguityItem(item ReviewItem, left, right Label) RubricAmbiguityItem {
	result := RubricAmbiguityItem{
		PacketID: item.Packet.PacketID, TaskGroupID: item.TaskGroupID, LeftLabel: left, RightLabel: right,
		PrimaryOutcomeDisagreement: left.PrimaryOutcome != right.PrimaryOutcome,
	}
	for _, axis := range rubricAxes() {
		if ratingForAxis(left, axis) != ratingForAxis(right, axis) {
			result.DisagreementAxes = append(result.DisagreementAxes, axis)
		}
	}
	intersection, union := reasonSetSizes(left.ReasonCodes, right.ReasonCodes)
	result.ReasonExactMatch = equalReasonCodes(left.ReasonCodes, right.ReasonCodes)
	result.ReasonZeroOverlap = intersection == 0
	result.ReasonJaccardDistance = 1 - ratio(intersection, union)
	return result
}

func validateRubricAmbiguityItem(item RubricAmbiguityItem) error {
	if !validOpaquePacketID(item.PacketID) || !validTaskGroupID(item.TaskGroupID) || item.LeftLabel.PacketID != item.PacketID || item.RightLabel.PacketID != item.PacketID {
		return errors.New("packet, task group, or embedded-label binding is invalid")
	}
	if err := item.LeftLabel.Validate(); err != nil {
		return err
	}
	if err := item.RightLabel.Validate(); err != nil {
		return err
	}
	if item.LeftLabel.ReviewerSlot != 1 || item.RightLabel.ReviewerSlot != 2 || item.LeftLabel.AdjudicatorAlias == item.RightLabel.AdjudicatorAlias || item.LeftLabel.RubricVersion != item.RightLabel.RubricVersion {
		return errors.New("embedded primary labels do not represent independent slots under one rubric")
	}
	expected := buildRubricAmbiguityItem(ReviewItem{TaskGroupID: item.TaskGroupID, Packet: BlindPacket{PacketID: item.PacketID}}, item.LeftLabel, item.RightLabel)
	if item.PrimaryOutcomeDisagreement != expected.PrimaryOutcomeDisagreement || !equalRubricAxes(item.DisagreementAxes, expected.DisagreementAxes) ||
		item.ReasonJaccardDistance != expected.ReasonJaccardDistance || item.ReasonExactMatch != expected.ReasonExactMatch || item.ReasonZeroOverlap != expected.ReasonZeroOverlap {
		return errors.New("item ambiguity flags are inconsistent with embedded labels")
	}
	return nil
}

func summarizeRubricAmbiguityItems(items []RubricAmbiguityItem) RubricAmbiguityAnalysis {
	analysis := RubricAmbiguityAnalysis{
		Packets: len(items), LabelObservations: len(items) * 2, AxisComparisons: len(items) * len(rubricAxes()), Items: append([]RubricAmbiguityItem(nil), items...),
	}
	stateCounts := make(map[State]int)
	axisMetrics := make(map[RubricAxis]*RubricAxisMetric, len(rubricAxes()))
	for _, axis := range rubricAxes() {
		axisMetrics[axis] = &RubricAxisMetric{Axis: axis, Comparisons: len(items)}
	}
	for _, item := range items {
		if item.PrimaryOutcomeDisagreement {
			analysis.PrimaryOutcomeDisagreements++
		}
		if len(item.DisagreementAxes) > 0 {
			analysis.AnyAxisDisagreements++
		}
		stateCounts[item.LeftLabel.PrimaryOutcome]++
		stateCounts[item.RightLabel.PrimaryOutcome]++
		if item.LeftLabel.PrimaryOutcome == StateIndeterminate {
			analysis.IndeterminateLabels++
		}
		if item.RightLabel.PrimaryOutcome == StateIndeterminate {
			analysis.IndeterminateLabels++
		}
		if item.ReasonExactMatch {
			analysis.ExactReasonMatches++
		}
		if item.ReasonZeroOverlap {
			analysis.ZeroReasonOverlaps++
		}
		analysis.MeanReasonJaccardDistance += item.ReasonJaccardDistance
		for _, axis := range rubricAxes() {
			metric := axisMetrics[axis]
			leftRating, rightRating := ratingForAxis(item.LeftLabel, axis), ratingForAxis(item.RightLabel, axis)
			if leftRating != rightRating {
				metric.Disagreements++
			}
			if leftRating == RatingUnclear {
				metric.UnclearRatings++
				analysis.UnclearRatings++
			}
			if rightRating == RatingUnclear {
				metric.UnclearRatings++
				analysis.UnclearRatings++
			}
			metric.LeftPrevalence = incrementRatingCount(metric.LeftPrevalence, leftRating)
			metric.RightPrevalence = incrementRatingCount(metric.RightPrevalence, rightRating)
		}
	}
	analysis.PrimaryOutcomeDisagreementRate = ratio(analysis.PrimaryOutcomeDisagreements, analysis.Packets)
	analysis.PrimaryOutcomeDisagreementInterval = wilsonInterval(analysis.PrimaryOutcomeDisagreements, analysis.Packets)
	analysis.AnyAxisDisagreementRate = ratio(analysis.AnyAxisDisagreements, analysis.Packets)
	analysis.AnyAxisDisagreementInterval = wilsonInterval(analysis.AnyAxisDisagreements, analysis.Packets)
	analysis.IndeterminateRate = ratio(analysis.IndeterminateLabels, analysis.LabelObservations)
	analysis.IndeterminateInterval = wilsonInterval(analysis.IndeterminateLabels, analysis.LabelObservations)
	analysis.UnclearRate = ratio(analysis.UnclearRatings, analysis.AxisComparisons*2)
	analysis.UnclearInterval = wilsonInterval(analysis.UnclearRatings, analysis.AxisComparisons*2)
	analysis.ExactReasonMatchRate = ratio(analysis.ExactReasonMatches, analysis.Packets)
	analysis.ExactReasonMatchInterval = wilsonInterval(analysis.ExactReasonMatches, analysis.Packets)
	analysis.ZeroReasonOverlapRate = ratio(analysis.ZeroReasonOverlaps, analysis.Packets)
	analysis.ZeroReasonOverlapInterval = wilsonInterval(analysis.ZeroReasonOverlaps, analysis.Packets)
	analysis.MeanReasonJaccardDistance = ratioFloat(analysis.MeanReasonJaccardDistance, analysis.Packets)
	analysis.PrimaryStatePrevalence = sortedStateCounts(stateCounts)
	for _, axis := range rubricAxes() {
		metric := axisMetrics[axis]
		metric.DisagreementRate = ratio(metric.Disagreements, metric.Comparisons)
		metric.DisagreementInterval = wilsonInterval(metric.Disagreements, metric.Comparisons)
		metric.UnclearRate = ratio(metric.UnclearRatings, metric.Comparisons*2)
		metric.UnclearInterval = wilsonInterval(metric.UnclearRatings, metric.Comparisons*2)
		sortRatingCounts(metric.LeftPrevalence)
		sortRatingCounts(metric.RightPrevalence)
		analysis.AxisMetrics = append(analysis.AxisMetrics, *metric)
	}
	return analysis
}

func primaryReviewsBySlot(leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch) (ReviewAssignment, LabelBatch, ReviewAssignment, LabelBatch) {
	if leftAssignment.ReviewerSlot == 1 {
		return leftAssignment, leftBatch, rightAssignment, rightBatch
	}
	return rightAssignment, rightBatch, leftAssignment, leftBatch
}

func rubricAxes() []RubricAxis {
	return []RubricAxis{
		RubricAxisTaskSatisfaction, RubricAxisTechnicalCorrectness, RubricAxisVerificationQuality,
		RubricAxisHarmfulSideEffects, RubricAxisEvidenceSufficiency,
	}
}

func ratingForAxis(label Label, axis RubricAxis) AxisRating {
	switch axis {
	case RubricAxisTaskSatisfaction:
		return label.TaskSatisfaction
	case RubricAxisTechnicalCorrectness:
		return label.TechnicalCorrectness
	case RubricAxisVerificationQuality:
		return label.VerificationQuality
	case RubricAxisHarmfulSideEffects:
		return label.HarmfulSideEffects
	case RubricAxisEvidenceSufficiency:
		return label.EvidenceSufficiency
	default:
		return ""
	}
}

func reasonSetSizes(left, right []ReasonCode) (int, int) {
	leftSet := make(map[ReasonCode]struct{}, len(left))
	union := make(map[ReasonCode]struct{}, len(left)+len(right))
	for _, value := range left {
		leftSet[value] = struct{}{}
		union[value] = struct{}{}
	}
	intersection := 0
	for _, value := range right {
		if _, exists := leftSet[value]; exists {
			intersection++
		}
		union[value] = struct{}{}
	}
	return intersection, len(union)
}

func equalReasonCodes(left, right []ReasonCode) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func incrementRatingCount(values []RatingCount, rating AxisRating) []RatingCount {
	for index := range values {
		if values[index].Rating == rating {
			values[index].Count++
			return values
		}
	}
	return append(values, RatingCount{Rating: rating, Count: 1})
}

func sortRatingCounts(values []RatingCount) {
	sort.Slice(values, func(left, right int) bool { return values[left].Rating < values[right].Rating })
}

func sortedStateCounts(counts map[State]int) []StateCount {
	values := make([]StateCount, 0, len(counts))
	for state, count := range counts {
		values = append(values, StateCount{State: state, Count: count})
	}
	sort.Slice(values, func(left, right int) bool { return values[left].State < values[right].State })
	return values
}

func ratioFloat(numerator float64, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / float64(denominator)
}

func rubricAmbiguityLimitations() []string {
	return []string{
		"axis and reason-code disagreements are descriptive signals, not proof that the rubric is defective",
		"development pilot labels cannot validate calibration or test outcomes",
		"two-reviewer estimates are unstable for small samples and require complete denominators",
	}
}

func rubricAmbiguityPacketIDs(items []RubricAmbiguityItem) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.PacketID)
	}
	return values
}

func equalRubricAxes(left, right []RubricAxis) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalRubricAxisMetrics(left, right []RubricAxisMetric) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Axis != right[index].Axis || left[index].Comparisons != right[index].Comparisons ||
			left[index].Disagreements != right[index].Disagreements || left[index].DisagreementRate != right[index].DisagreementRate ||
			left[index].DisagreementInterval != right[index].DisagreementInterval || left[index].UnclearRatings != right[index].UnclearRatings ||
			left[index].UnclearRate != right[index].UnclearRate || left[index].UnclearInterval != right[index].UnclearInterval ||
			!equalRatingCounts(left[index].LeftPrevalence, right[index].LeftPrevalence) ||
			!equalRatingCounts(left[index].RightPrevalence, right[index].RightPrevalence) {
			return false
		}
	}
	return true
}

func equalRatingCounts(left, right []RatingCount) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalStateCounts(left, right []StateCount) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalRubricAmbiguitySummary(left, right RubricAmbiguityAnalysis) bool {
	return left.Packets == right.Packets && left.LabelObservations == right.LabelObservations && left.AxisComparisons == right.AxisComparisons &&
		left.PrimaryOutcomeDisagreements == right.PrimaryOutcomeDisagreements && left.PrimaryOutcomeDisagreementRate == right.PrimaryOutcomeDisagreementRate &&
		left.PrimaryOutcomeDisagreementInterval == right.PrimaryOutcomeDisagreementInterval && left.AnyAxisDisagreements == right.AnyAxisDisagreements &&
		left.AnyAxisDisagreementRate == right.AnyAxisDisagreementRate && left.AnyAxisDisagreementInterval == right.AnyAxisDisagreementInterval &&
		left.IndeterminateLabels == right.IndeterminateLabels && left.IndeterminateRate == right.IndeterminateRate && left.IndeterminateInterval == right.IndeterminateInterval &&
		left.UnclearRatings == right.UnclearRatings && left.UnclearRate == right.UnclearRate && left.UnclearInterval == right.UnclearInterval &&
		left.ExactReasonMatches == right.ExactReasonMatches && left.ExactReasonMatchRate == right.ExactReasonMatchRate &&
		left.ExactReasonMatchInterval == right.ExactReasonMatchInterval && left.ZeroReasonOverlaps == right.ZeroReasonOverlaps &&
		left.ZeroReasonOverlapRate == right.ZeroReasonOverlapRate && left.ZeroReasonOverlapInterval == right.ZeroReasonOverlapInterval &&
		left.MeanReasonJaccardDistance == right.MeanReasonJaccardDistance && equalRubricAxisMetrics(left.AxisMetrics, right.AxisMetrics) &&
		equalStateCounts(left.PrimaryStatePrevalence, right.PrimaryStatePrevalence)
}

func rubricAmbiguityDigest(value RubricAmbiguityAnalysis) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
