package relation

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"time"
)

const (
	RelationAmbiguitySchemaVersionV1 = "evalwitness.relation-prereveal-ambiguity-analysis.v1"
	RelationAmbiguitySchemaVersionV2 = "evalwitness.relation-prereveal-ambiguity-analysis.v2"
	RelationAmbiguitySchemaVersionV3 = "evalwitness.relation-prereveal-ambiguity-analysis.v3"
	RelationAmbiguitySchemaVersion   = RelationAmbiguitySchemaVersionV1
)

type RateInterval struct {
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

type VisibleRatingCount struct {
	Rating Rating `json:"rating"`
	Count  int    `json:"count"`
}

type RelationAxisMetric struct {
	Axis                 Axis                 `json:"axis"`
	Comparisons          int                  `json:"comparisons"`
	Disagreements        int                  `json:"disagreements"`
	DisagreementRate     float64              `json:"disagreement_rate"`
	DisagreementInterval RateInterval         `json:"disagreement_interval"`
	UnclearRatings       int                  `json:"unclear_ratings"`
	NotApplicableRatings int                  `json:"not_applicable_ratings"`
	LeftPrevalence       []VisibleRatingCount `json:"left_prevalence"`
	RightPrevalence      []VisibleRatingCount `json:"right_prevalence"`
}

type JudgmentCommitmentReference struct {
	ReviewerSlot     int    `json:"reviewer_slot"`
	AssignmentDigest string `json:"assignment_digest"`
	BatchDigest      string `json:"batch_digest"`
}

type RelationAmbiguityItem struct {
	PacketID              string       `json:"packet_id"`
	PacketDigest          string       `json:"packet_digest"`
	SlotOneJudgment       PairJudgment `json:"slot_one_judgment"`
	SlotTwoJudgment       PairJudgment `json:"slot_two_judgment"`
	DisagreementAxes      []Axis       `json:"disagreement_axes"`
	ReasonExactMatch      bool         `json:"reason_exact_match"`
	ReasonZeroOverlap     bool         `json:"reason_zero_overlap"`
	ReasonJaccardDistance float64      `json:"reason_jaccard_distance"`
	TieBreakRequired      bool         `json:"tie_break_required"`
}

type RelationAmbiguityAnalysis struct {
	SchemaVersion              string                        `json:"schema_version"`
	CanonicalPolicy            string                        `json:"canonical_policy"`
	ProtocolVersion            string                        `json:"protocol_version"`
	Objective                  ReviewObjective               `json:"review_objective"`
	PlanDigest                 string                        `json:"plan_digest"`
	BundleDigest               string                        `json:"bundle_digest"`
	PrimaryCommitments         []JudgmentCommitmentReference `json:"primary_commitments"`
	Packets                    int                           `json:"packets"`
	JudgmentObservations       int                           `json:"judgment_observations"`
	AxisComparisons            int                           `json:"axis_comparisons"`
	PacketsWithAnyDisagreement int                           `json:"packets_with_any_disagreement"`
	AnyDisagreementRate        float64                       `json:"any_disagreement_rate"`
	AnyDisagreementInterval    RateInterval                  `json:"any_disagreement_interval"`
	AxisMetrics                []RelationAxisMetric          `json:"axis_metrics"`
	UnclearRatings             int                           `json:"unclear_ratings"`
	UnclearRate                float64                       `json:"unclear_rate"`
	UnclearInterval            RateInterval                  `json:"unclear_interval"`
	NotApplicableRatings       int                           `json:"not_applicable_ratings"`
	NotApplicableRate          float64                       `json:"not_applicable_rate"`
	NotApplicableInterval      RateInterval                  `json:"not_applicable_interval"`
	ExactReasonMatches         int                           `json:"exact_reason_matches"`
	ExactReasonMatchRate       float64                       `json:"exact_reason_match_rate"`
	ExactReasonMatchInterval   RateInterval                  `json:"exact_reason_match_interval"`
	ZeroReasonOverlaps         int                           `json:"zero_reason_overlaps"`
	ZeroReasonOverlapRate      float64                       `json:"zero_reason_overlap_rate"`
	ZeroReasonOverlapInterval  RateInterval                  `json:"zero_reason_overlap_interval"`
	MeanReasonJaccardDistance  float64                       `json:"mean_reason_jaccard_distance"`
	TieBreakPacketIDs          []string                      `json:"tie_break_packet_ids"`
	Items                      []RelationAmbiguityItem       `json:"items"`
	AnalyzedAt                 string                        `json:"analyzed_at"`
	RevealStatus               string                        `json:"reveal_status"`
	Limitations                []string                      `json:"limitations"`
	ExternalActionStatus       ExternalActionStatus          `json:"external_action_status"`
	Digest                     string                        `json:"digest"`
}

func BuildRelationAmbiguityAnalysis(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch JudgmentBatch, rightAssignment ReviewAssignment, rightBatch JudgmentBatch, analyzedAt string) (RelationAmbiguityAnalysis, error) {
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := validatePrimaryJudgmentPair(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch)
	if err != nil {
		return RelationAmbiguityAnalysis{}, err
	}
	latestCommit := leftBatch.CommittedAt
	leftTime, _ := time.Parse(time.RFC3339, leftBatch.CommittedAt)
	rightTime, _ := time.Parse(time.RFC3339, rightBatch.CommittedAt)
	if rightTime.After(leftTime) {
		latestCommit = rightBatch.CommittedAt
	}
	if err := requireRelationTimeAfter("relation prereveal ambiguity analysis", analyzedAt, latestCommit); err != nil {
		return RelationAmbiguityAnalysis{}, err
	}
	leftByPacket, rightByPacket := judgmentsByPacket(leftBatch.Judgments), judgmentsByPacket(rightBatch.Judgments)
	items := make([]RelationAmbiguityItem, len(bundle.Packets))
	for index, packet := range bundle.Packets {
		items[index] = buildRelationAmbiguityItem(packet, leftByPacket[packet.PacketID], rightByPacket[packet.PacketID])
	}
	sort.Slice(items, func(left, right int) bool { return items[left].PacketID < items[right].PacketID })
	analysis := summarizeRelationAmbiguity(items)
	analysis.ProtocolVersion, analysis.Objective, analysis.PlanDigest, analysis.BundleDigest = bundle.ProtocolVersion, ReviewObjectiveControlledRelation, bundle.PlanDigest, bundle.Digest
	analysis.PrimaryCommitments = primaryJudgmentCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch)
	analysis.AnalyzedAt, analysis.RevealStatus = analyzedAt, "not_revealed"
	analysis.Limitations, analysis.ExternalActionStatus = relationAmbiguityLimitations(), ExternalActionNotAuthorized
	return SealRelationAmbiguityAnalysis(analysis)
}

func SealRelationAmbiguityAnalysis(analysis RelationAmbiguityAnalysis) (RelationAmbiguityAnalysis, error) {
	schemaVersion, err := schemaVersionForProtocol(analysis.ProtocolVersion, RelationAmbiguitySchemaVersionV1, RelationAmbiguitySchemaVersionV2, RelationAmbiguitySchemaVersionV3)
	if err != nil {
		return RelationAmbiguityAnalysis{}, err
	}
	analysis.SchemaVersion, analysis.CanonicalPolicy, analysis.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := relationAmbiguityDigest(analysis)
	if err != nil {
		return RelationAmbiguityAnalysis{}, err
	}
	analysis.Digest = digest
	return analysis, analysis.Validate()
}

func (analysis RelationAmbiguityAnalysis) Validate() error {
	if !validVersionedIdentity(analysis.SchemaVersion, analysis.ProtocolVersion, RelationAmbiguitySchemaVersionV1, RelationAmbiguitySchemaVersionV2, RelationAmbiguitySchemaVersionV3) || analysis.CanonicalPolicy != CanonicalPolicy ||
		analysis.Objective != ReviewObjectiveControlledRelation || !validDigest(analysis.PlanDigest) || !validDigest(analysis.BundleDigest) ||
		analysis.Packets < 1 || analysis.JudgmentObservations != analysis.Packets*2 || analysis.AxisComparisons != analysis.Packets*7 ||
		len(analysis.Items) != analysis.Packets || len(analysis.AxisMetrics) != 7 || analysis.RevealStatus != "not_revealed" ||
		!slices.Equal(analysis.Limitations, relationAmbiguityLimitations()) || analysis.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation ambiguity analysis identity, objective, denominators, reveal, or authorization boundary is invalid")
	}
	if err := validateJudgmentCommitments(analysis.PrimaryCommitments); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, analysis.AnalyzedAt); err != nil {
		return errors.New("relation ambiguity analysis time must be RFC3339")
	}
	for index, item := range analysis.Items {
		if err := validateRelationAmbiguityItem(item); err != nil {
			return fmt.Errorf("relation ambiguity item %d: %w", index, err)
		}
		if item.SlotOneJudgment.ProtocolVersion != analysis.ProtocolVersion || item.SlotTwoJudgment.ProtocolVersion != analysis.ProtocolVersion {
			return errors.New("relation ambiguity item protocol differs from the analysis")
		}
		if index > 0 && analysis.Items[index-1].PacketID >= item.PacketID {
			return errors.New("relation ambiguity items must be unique and sorted")
		}
	}
	recomputed := summarizeRelationAmbiguity(analysis.Items)
	if !equalRelationAmbiguitySummary(analysis, recomputed) {
		return errors.New("relation ambiguity statistics do not reproduce embedded judgments")
	}
	expected, err := relationAmbiguityDigest(analysis)
	if err != nil || analysis.Digest != expected {
		return errors.New("relation ambiguity analysis digest is invalid")
	}
	return nil
}

func BuildTieBreakAssignment(bundle ReviewBundle, mappings []PrivateMapping, reviewer ReviewerRecord, qualification QualificationReport, analysis RelationAmbiguityAnalysis, leftAssignment ReviewAssignment, leftBatch JudgmentBatch, rightAssignment ReviewAssignment, rightBatch JudgmentBatch, seed []byte, assignedAt string) (ReviewAssignment, error) {
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := validatePrimaryJudgmentPair(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch)
	if err != nil {
		return ReviewAssignment{}, err
	}
	if err := verifyRelationAmbiguityAgainstPair(analysis, bundle, leftAssignment, leftBatch, rightAssignment, rightBatch); err != nil {
		return ReviewAssignment{}, err
	}
	if len(analysis.TieBreakPacketIDs) == 0 {
		return ReviewAssignment{}, errors.New("relation tie-break assignment requires at least one committed primary disagreement")
	}
	if reviewer.ReviewerAlias == leftAssignment.Reviewer.ReviewerAlias || reviewer.ReviewerAlias == rightAssignment.Reviewer.ReviewerAlias {
		return ReviewAssignment{}, errors.New("relation tie-break reviewer must be independent of both primary reviewers")
	}
	if err := requireRelationTimeAfter("relation tie-break assignment", assignedAt, analysis.AnalyzedAt); err != nil {
		return ReviewAssignment{}, err
	}
	return buildReviewAssignment(bundle, mappings, reviewer, qualification, AssignmentPurposeTieBreak, 3, analysis.TieBreakPacketIDs, seed, assignedAt)
}

func validatePrimaryJudgmentPair(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch JudgmentBatch, rightAssignment ReviewAssignment, rightBatch JudgmentBatch) (ReviewAssignment, JudgmentBatch, ReviewAssignment, JudgmentBatch, error) {
	if err := VerifyJudgmentBatch(leftBatch, leftAssignment, bundle); err != nil {
		return ReviewAssignment{}, JudgmentBatch{}, ReviewAssignment{}, JudgmentBatch{}, err
	}
	if err := VerifyJudgmentBatch(rightBatch, rightAssignment, bundle); err != nil {
		return ReviewAssignment{}, JudgmentBatch{}, ReviewAssignment{}, JudgmentBatch{}, err
	}
	if leftAssignment.Purpose != AssignmentPurposePrimary || rightAssignment.Purpose != AssignmentPurposePrimary || leftAssignment.ReviewerSlot == rightAssignment.ReviewerSlot ||
		leftAssignment.Reviewer.ReviewerAlias == rightAssignment.Reviewer.ReviewerAlias || !slices.Equal([]int{min(leftAssignment.ReviewerSlot, rightAssignment.ReviewerSlot), max(leftAssignment.ReviewerSlot, rightAssignment.ReviewerSlot)}, []int{1, 2}) {
		return ReviewAssignment{}, JudgmentBatch{}, ReviewAssignment{}, JudgmentBatch{}, errors.New("relation primary judgment pair requires independent reviewer slots one and two")
	}
	if leftAssignment.ReviewerSlot == 2 {
		leftAssignment, rightAssignment = rightAssignment, leftAssignment
		leftBatch, rightBatch = rightBatch, leftBatch
	}
	return leftAssignment, leftBatch, rightAssignment, rightBatch, nil
}

func verifyRelationAmbiguityAgainstPair(analysis RelationAmbiguityAnalysis, bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch JudgmentBatch, rightAssignment ReviewAssignment, rightBatch JudgmentBatch) error {
	if err := analysis.Validate(); err != nil {
		return err
	}
	if analysis.ProtocolVersion != bundle.ProtocolVersion || analysis.PlanDigest != bundle.PlanDigest || analysis.BundleDigest != bundle.Digest ||
		!slices.Equal(analysis.PrimaryCommitments, primaryJudgmentCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch)) {
		return errors.New("relation ambiguity analysis does not bind the exact primary commitments")
	}
	leftByPacket, rightByPacket := judgmentsByPacket(leftBatch.Judgments), judgmentsByPacket(rightBatch.Judgments)
	for _, item := range analysis.Items {
		if item.SlotOneJudgment.Digest != leftByPacket[item.PacketID].Digest || item.SlotTwoJudgment.Digest != rightByPacket[item.PacketID].Digest {
			return errors.New("relation ambiguity analysis does not reproduce the committed primary judgments")
		}
	}
	return nil
}

func buildRelationAmbiguityItem(packet BlindPacket, left, right PairJudgment) RelationAmbiguityItem {
	item := RelationAmbiguityItem{PacketID: packet.PacketID, PacketDigest: packet.Digest, SlotOneJudgment: left, SlotTwoJudgment: right}
	for index, observation := range left.Observations {
		if observation.Rating != right.Observations[index].Rating {
			item.DisagreementAxes = append(item.DisagreementAxes, observation.Axis)
		}
	}
	intersection, union := reasonSetSizes(left.ReasonCodes, right.ReasonCodes)
	item.ReasonExactMatch = slices.Equal(left.ReasonCodes, right.ReasonCodes)
	item.ReasonZeroOverlap = intersection == 0
	item.ReasonJaccardDistance = 1 - relationRatio(intersection, union)
	item.TieBreakRequired = len(item.DisagreementAxes) > 0 || !item.ReasonExactMatch
	return item
}

func validateRelationAmbiguityItem(item RelationAmbiguityItem) error {
	if !validOpaqueID(item.PacketID, "relation-packet-") || !validDigest(item.PacketDigest) || item.SlotOneJudgment.PacketID != item.PacketID ||
		item.SlotTwoJudgment.PacketID != item.PacketID || item.SlotOneJudgment.PacketDigest != item.PacketDigest || item.SlotTwoJudgment.PacketDigest != item.PacketDigest {
		return errors.New("relation ambiguity item packet binding is invalid")
	}
	if err := item.SlotOneJudgment.Validate(); err != nil {
		return err
	}
	if err := item.SlotTwoJudgment.Validate(); err != nil {
		return err
	}
	if item.SlotOneJudgment.ProtocolVersion != item.SlotTwoJudgment.ProtocolVersion || item.SlotOneJudgment.ReviewerSlot != 1 || item.SlotTwoJudgment.ReviewerSlot != 2 || item.SlotOneJudgment.ReviewerAlias == item.SlotTwoJudgment.ReviewerAlias {
		return errors.New("relation ambiguity item does not contain independent primary judgments")
	}
	expected := buildRelationAmbiguityItem(BlindPacket{PacketID: item.PacketID, Digest: item.PacketDigest}, item.SlotOneJudgment, item.SlotTwoJudgment)
	if !slices.Equal(item.DisagreementAxes, expected.DisagreementAxes) || item.ReasonExactMatch != expected.ReasonExactMatch || item.ReasonZeroOverlap != expected.ReasonZeroOverlap ||
		item.ReasonJaccardDistance != expected.ReasonJaccardDistance || item.TieBreakRequired != expected.TieBreakRequired {
		return errors.New("relation ambiguity item disagreement evidence is inconsistent")
	}
	return nil
}

func summarizeRelationAmbiguity(items []RelationAmbiguityItem) RelationAmbiguityAnalysis {
	analysis := RelationAmbiguityAnalysis{Packets: len(items), JudgmentObservations: len(items) * 2, AxisComparisons: len(items) * 7, Items: append([]RelationAmbiguityItem(nil), items...)}
	metrics := make(map[Axis]*RelationAxisMetric, 7)
	for _, definition := range defaultAxes() {
		metrics[definition.ID] = &RelationAxisMetric{Axis: definition.ID, Comparisons: len(items), LeftPrevalence: zeroRatingCounts(definition.AllowedRatings), RightPrevalence: zeroRatingCounts(definition.AllowedRatings)}
	}
	for _, item := range items {
		accumulateAmbiguityItem(&analysis, metrics, item)
	}
	finalizeAmbiguitySummary(&analysis, metrics)
	return analysis
}

func accumulateAmbiguityItem(analysis *RelationAmbiguityAnalysis, metrics map[Axis]*RelationAxisMetric, item RelationAmbiguityItem) {
	if item.TieBreakRequired {
		analysis.PacketsWithAnyDisagreement++
		analysis.TieBreakPacketIDs = append(analysis.TieBreakPacketIDs, item.PacketID)
	}
	if item.ReasonExactMatch {
		analysis.ExactReasonMatches++
	}
	if item.ReasonZeroOverlap {
		analysis.ZeroReasonOverlaps++
	}
	analysis.MeanReasonJaccardDistance += item.ReasonJaccardDistance
	for index, left := range item.SlotOneJudgment.Observations {
		right, metric := item.SlotTwoJudgment.Observations[index], metrics[left.Axis]
		if left.Rating != right.Rating {
			metric.Disagreements++
		}
		incrementRating(metric.LeftPrevalence, left.Rating)
		incrementRating(metric.RightPrevalence, right.Rating)
		for _, rating := range []Rating{left.Rating, right.Rating} {
			if rating == RatingIndeterminate || rating == RatingInsufficient {
				metric.UnclearRatings++
				analysis.UnclearRatings++
			}
			if rating == RatingNotApplicable {
				metric.NotApplicableRatings++
				analysis.NotApplicableRatings++
			}
		}
	}
}

func finalizeAmbiguitySummary(analysis *RelationAmbiguityAnalysis, metrics map[Axis]*RelationAxisMetric) {
	analysis.AnyDisagreementRate = relationRatio(analysis.PacketsWithAnyDisagreement, analysis.Packets)
	analysis.AnyDisagreementInterval = relationWilsonInterval(analysis.PacketsWithAnyDisagreement, analysis.Packets)
	allRatings := analysis.AxisComparisons * 2
	analysis.UnclearRate, analysis.UnclearInterval = relationRatio(analysis.UnclearRatings, allRatings), relationWilsonInterval(analysis.UnclearRatings, allRatings)
	analysis.NotApplicableRate, analysis.NotApplicableInterval = relationRatio(analysis.NotApplicableRatings, allRatings), relationWilsonInterval(analysis.NotApplicableRatings, allRatings)
	analysis.ExactReasonMatchRate, analysis.ExactReasonMatchInterval = relationRatio(analysis.ExactReasonMatches, analysis.Packets), relationWilsonInterval(analysis.ExactReasonMatches, analysis.Packets)
	analysis.ZeroReasonOverlapRate, analysis.ZeroReasonOverlapInterval = relationRatio(analysis.ZeroReasonOverlaps, analysis.Packets), relationWilsonInterval(analysis.ZeroReasonOverlaps, analysis.Packets)
	analysis.MeanReasonJaccardDistance /= float64(analysis.Packets)
	for _, definition := range defaultAxes() {
		metric := metrics[definition.ID]
		metric.DisagreementRate, metric.DisagreementInterval = relationRatio(metric.Disagreements, metric.Comparisons), relationWilsonInterval(metric.Disagreements, metric.Comparisons)
		analysis.AxisMetrics = append(analysis.AxisMetrics, *metric)
	}
}

func equalRelationAmbiguitySummary(left, right RelationAmbiguityAnalysis) bool {
	return left.Packets == right.Packets && left.JudgmentObservations == right.JudgmentObservations && left.AxisComparisons == right.AxisComparisons &&
		left.PacketsWithAnyDisagreement == right.PacketsWithAnyDisagreement && left.AnyDisagreementRate == right.AnyDisagreementRate && left.AnyDisagreementInterval == right.AnyDisagreementInterval &&
		left.UnclearRatings == right.UnclearRatings && left.UnclearRate == right.UnclearRate && left.UnclearInterval == right.UnclearInterval &&
		left.NotApplicableRatings == right.NotApplicableRatings && left.NotApplicableRate == right.NotApplicableRate && left.NotApplicableInterval == right.NotApplicableInterval &&
		left.ExactReasonMatches == right.ExactReasonMatches && left.ExactReasonMatchRate == right.ExactReasonMatchRate && left.ExactReasonMatchInterval == right.ExactReasonMatchInterval &&
		left.ZeroReasonOverlaps == right.ZeroReasonOverlaps && left.ZeroReasonOverlapRate == right.ZeroReasonOverlapRate && left.ZeroReasonOverlapInterval == right.ZeroReasonOverlapInterval &&
		left.MeanReasonJaccardDistance == right.MeanReasonJaccardDistance && slices.Equal(left.TieBreakPacketIDs, right.TieBreakPacketIDs) && reflect.DeepEqual(left.AxisMetrics, right.AxisMetrics)
}

func primaryJudgmentCommitments(leftAssignment ReviewAssignment, leftBatch JudgmentBatch, rightAssignment ReviewAssignment, rightBatch JudgmentBatch) []JudgmentCommitmentReference {
	values := []JudgmentCommitmentReference{{leftAssignment.ReviewerSlot, leftAssignment.Digest, leftBatch.Digest}, {rightAssignment.ReviewerSlot, rightAssignment.Digest, rightBatch.Digest}}
	sort.Slice(values, func(left, right int) bool { return values[left].ReviewerSlot < values[right].ReviewerSlot })
	return values
}

func validateJudgmentCommitments(values []JudgmentCommitmentReference) error {
	if len(values) != 2 || values[0].ReviewerSlot != 1 || values[1].ReviewerSlot != 2 {
		return errors.New("relation primary commitments require slots one and two")
	}
	for _, value := range values {
		if !validDigest(value.AssignmentDigest) || !validDigest(value.BatchDigest) {
			return errors.New("relation primary commitment digest is invalid")
		}
	}
	return nil
}

func judgmentsByPacket(values []PairJudgment) map[string]PairJudgment {
	result := make(map[string]PairJudgment, len(values))
	for _, value := range values {
		result[value.PacketID] = value
	}
	return result
}

func reasonSetSizes(left, right []ReasonCode) (int, int) {
	leftSet, union := make(map[ReasonCode]struct{}, len(left)), make(map[ReasonCode]struct{}, len(left)+len(right))
	for _, value := range left {
		leftSet[value], union[value] = struct{}{}, struct{}{}
	}
	intersection := 0
	for _, value := range right {
		union[value] = struct{}{}
		if _, exists := leftSet[value]; exists {
			intersection++
		}
	}
	return intersection, len(union)
}

func zeroRatingCounts(ratings []Rating) []VisibleRatingCount {
	result := make([]VisibleRatingCount, len(ratings))
	for index, rating := range ratings {
		result[index].Rating = rating
	}
	return result
}

func incrementRating(values []VisibleRatingCount, rating Rating) {
	for index := range values {
		if values[index].Rating == rating {
			values[index].Count++
			return
		}
	}
}

func relationRatio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func relationWilsonInterval(successes, observations int) RateInterval {
	if observations == 0 {
		return RateInterval{}
	}
	z := 1.959963984540054
	n := float64(observations)
	p := float64(successes) / n
	denominator := 1 + z*z/n
	center := (p + z*z/(2*n)) / denominator
	margin := z * math.Sqrt((p*(1-p)+z*z/(4*n))/n) / denominator
	return RateInterval{Lower: math.Max(0, center-margin), Upper: math.Min(1, center+margin)}
}

func relationAmbiguityLimitations() []string {
	return []string{
		"This prereveal analysis uses only committed visible-side judgments and cannot access family, direction, source condition, formal relation, mapping, validator, or verifier data.",
		"Disagreement is descriptive for the frozen sample and reviewer population; it is not a population prevalence estimate or a construct-validity result.",
		"A tie-break is required for any axis or reason-code disagreement, but it cannot convert insufficient visible information into support.",
	}
}

func relationAmbiguityDigest(value RelationAmbiguityAnalysis) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
