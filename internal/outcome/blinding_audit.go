package outcome

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"
)

const (
	BlindingProtocolSchemaVersion   = "evalwitness.outcome-blinding-protocol.v1"
	BlindingProbeSchemaVersion      = "evalwitness.outcome-blinding-probe.v1"
	BlindingProbeBatchSchemaVersion = "evalwitness.outcome-blinding-probe-batch.v1"
	BlindingAnalysisSchemaVersion   = "evalwitness.outcome-blinding-analysis.v1"
	BlindingProtocolVersion         = "evalwitness.semantic-blinding-audit.v1"
	UnknownCondition                = "unknown"
)

type RecognitionBasis string

const (
	RecognitionNone                  RecognitionBasis = "none"
	RecognitionTaskText              RecognitionBasis = "task_text"
	RecognitionTrajectoryContent     RecognitionBasis = "trajectory_content"
	RecognitionRepositoryFamiliarity RecognitionBasis = "repository_familiarity"
	RecognitionPriorExposure         RecognitionBasis = "prior_exposure"
	RecognitionOther                 RecognitionBasis = "other"
)

type BlindingProtocol struct {
	SchemaVersion       string   `json:"schema_version"`
	CanonicalPolicy     string   `json:"canonical_policy"`
	ProtocolVersion     string   `json:"protocol_version"`
	BundleDigest        string   `json:"bundle_digest"`
	ConditionCandidates []string `json:"condition_candidates"`
	UnknownToken        string   `json:"unknown_token"`
	Prompt              string   `json:"prompt"`
	PrimaryMetric       string   `json:"primary_metric"`
	SecondaryMetrics    []string `json:"secondary_metrics"`
	CreatedAt           string   `json:"created_at"`
	Digest              string   `json:"digest"`
}

type BlindingProbeDraft struct {
	PacketID         string           `json:"packet_id"`
	ConditionGuess   string           `json:"condition_guess"`
	Confidence       float64          `json:"confidence"`
	RecognizedTask   bool             `json:"recognized_task"`
	RecognitionBasis RecognitionBasis `json:"recognition_basis"`
	SubmittedAt      string           `json:"submitted_at"`
}

type BlindingProbe struct {
	SchemaVersion    string           `json:"schema_version"`
	CanonicalPolicy  string           `json:"canonical_policy"`
	ProtocolDigest   string           `json:"protocol_digest"`
	AssignmentDigest string           `json:"assignment_digest"`
	LabelBatchDigest string           `json:"label_batch_digest"`
	LabelDigest      string           `json:"label_digest"`
	PacketID         string           `json:"packet_id"`
	AdjudicatorAlias string           `json:"adjudicator_alias"`
	ReviewerSlot     int              `json:"reviewer_slot"`
	ConditionGuess   string           `json:"condition_guess"`
	Confidence       float64          `json:"confidence"`
	RecognizedTask   bool             `json:"recognized_task"`
	RecognitionBasis RecognitionBasis `json:"recognition_basis"`
	SubmittedAt      string           `json:"submitted_at"`
	Digest           string           `json:"digest"`
}

type BlindingProbeBatch struct {
	SchemaVersion   string           `json:"schema_version"`
	CanonicalPolicy string           `json:"canonical_policy"`
	Protocol        BlindingProtocol `json:"protocol"`
	Assignment      ReviewAssignment `json:"assignment"`
	LabelBatch      LabelBatch       `json:"label_batch"`
	CommittedAt     string           `json:"committed_at"`
	Probes          []BlindingProbe  `json:"probes"`
	Digest          string           `json:"digest"`
}

type BlindingGuessResult struct {
	ReviewerSlot     int              `json:"reviewer_slot"`
	AdjudicatorAlias string           `json:"adjudicator_alias"`
	ConditionGuess   string           `json:"condition_guess"`
	Confidence       float64          `json:"confidence"`
	RecognizedTask   bool             `json:"recognized_task"`
	RecognitionBasis RecognitionBasis `json:"recognition_basis"`
	ProbeDigest      string           `json:"probe_digest"`
	Correct          bool             `json:"correct"`
}

type BlindingItemResult struct {
	PacketID       string                `json:"packet_id"`
	MappingDigest  string                `json:"mapping_digest"`
	TruthCondition string                `json:"truth_condition"`
	Guesses        []BlindingGuessResult `json:"guesses"`
}

type ConditionLeakageMetric struct {
	Condition         string  `json:"condition"`
	Observations      int     `json:"observations"`
	Attempts          int     `json:"attempts"`
	Correct           int     `json:"correct"`
	Accuracy          float64 `json:"accuracy"`
	SelectiveAccuracy float64 `json:"selective_accuracy"`
	Coverage          float64 `json:"coverage"`
}

type ReviewerLeakageMetric struct {
	ReviewerSlot      int     `json:"reviewer_slot"`
	AdjudicatorAlias  string  `json:"adjudicator_alias"`
	Observations      int     `json:"observations"`
	Attempts          int     `json:"attempts"`
	Correct           int     `json:"correct"`
	Accuracy          float64 `json:"accuracy"`
	SelectiveAccuracy float64 `json:"selective_accuracy"`
	Coverage          float64 `json:"coverage"`
	MeanConfidence    float64 `json:"mean_confidence"`
	GuessBrierScore   float64 `json:"guess_brier_score"`
	RecognizedTasks   int     `json:"recognized_tasks"`
}

type BlindingAnalysis struct {
	SchemaVersion             string                   `json:"schema_version"`
	CanonicalPolicy           string                   `json:"canonical_policy"`
	BundleDigest              string                   `json:"bundle_digest"`
	MappingRevealDigest       string                   `json:"mapping_reveal_digest"`
	Protocol                  BlindingProtocol         `json:"protocol"`
	ProbeBatchDigests         []string                 `json:"probe_batch_digests"`
	Packets                   int                      `json:"packets"`
	Reviewers                 int                      `json:"reviewers"`
	Observations              int                      `json:"observations"`
	Attempts                  int                      `json:"attempts"`
	Correct                   int                      `json:"correct"`
	ConditionGuessAccuracy    float64                  `json:"condition_guess_accuracy"`
	AccuracyInterval          Interval                 `json:"accuracy_interval"`
	GuessCoverage             float64                  `json:"guess_coverage"`
	SelectiveAccuracy         float64                  `json:"selective_accuracy"`
	SelectiveAccuracyDefined  bool                     `json:"selective_accuracy_defined"`
	SelectiveAccuracyInterval Interval                 `json:"selective_accuracy_interval"`
	ExpectedChanceAccuracy    float64                  `json:"expected_chance_accuracy"`
	CohenKappa                float64                  `json:"cohen_kappa"`
	CohenKappaDefined         bool                     `json:"cohen_kappa_defined"`
	MeanConfidence            float64                  `json:"mean_confidence"`
	GuessBrierScore           float64                  `json:"guess_brier_score"`
	ConfidenceMetricsDefined  bool                     `json:"confidence_metrics_defined"`
	RecognizedTasks           int                      `json:"recognized_tasks"`
	RecognizedTaskRate        float64                  `json:"recognized_task_rate"`
	RecognitionRateInterval   Interval                 `json:"recognition_rate_interval"`
	ByCondition               []ConditionLeakageMetric `json:"by_condition"`
	ByReviewer                []ReviewerLeakageMetric  `json:"by_reviewer"`
	Items                     []BlindingItemResult     `json:"items"`
	Limitations               []string                 `json:"limitations"`
	AnalyzedAt                string                   `json:"analyzed_at"`
	Digest                    string                   `json:"digest"`
}

func BuildBlindingProtocol(bundle ReviewBundle, mappings []PrivateMapping, createdAt string) (BlindingProtocol, error) {
	if err := bundle.Validate(); err != nil {
		return BlindingProtocol{}, err
	}
	if _, err := mappingReferences(bundle, mappings); err != nil {
		return BlindingProtocol{}, errors.New("blinding protocol mappings do not reproduce the review bundle")
	}
	if err := requireNotBefore("blinding protocol", createdAt, bundle.CreatedAt); err != nil {
		return BlindingProtocol{}, err
	}
	conditions := uniqueMappingConditions(mappings)
	if len(conditions) < 2 {
		return BlindingProtocol{}, errors.New("blinding protocol requires at least two possible source conditions")
	}
	return SealBlindingProtocol(BlindingProtocol{
		ProtocolVersion: BlindingProtocolVersion, BundleDigest: bundle.Digest, ConditionCandidates: conditions,
		UnknownToken: UnknownCondition, Prompt: blindingProtocolPrompt(), PrimaryMetric: "condition_guess_accuracy",
		SecondaryMetrics: blindingProtocolSecondaryMetrics(), CreatedAt: createdAt,
	})
}

func SealBlindingProtocol(protocol BlindingProtocol) (BlindingProtocol, error) {
	protocol.SchemaVersion, protocol.CanonicalPolicy, protocol.Digest = BlindingProtocolSchemaVersion, CanonicalPolicy, ""
	digest, err := blindingProtocolDigest(protocol)
	if err != nil {
		return BlindingProtocol{}, err
	}
	protocol.Digest = digest
	return protocol, protocol.Validate()
}

func (protocol BlindingProtocol) Validate() error {
	if protocol.SchemaVersion != BlindingProtocolSchemaVersion || protocol.CanonicalPolicy != CanonicalPolicy ||
		protocol.ProtocolVersion != BlindingProtocolVersion || !validDigest(protocol.BundleDigest) || protocol.UnknownToken != UnknownCondition ||
		protocol.Prompt != blindingProtocolPrompt() || protocol.PrimaryMetric != "condition_guess_accuracy" ||
		!equalStrings(protocol.SecondaryMetrics, blindingProtocolSecondaryMetrics()) || len(protocol.ConditionCandidates) < 2 {
		return errors.New("blinding protocol identity, frozen policy, bundle, or candidate universe is invalid")
	}
	if err := uniqueSorted("blinding protocol condition candidates", protocol.ConditionCandidates); err != nil {
		return err
	}
	for _, condition := range protocol.ConditionCandidates {
		if condition == UnknownCondition || !validConditionName(condition) {
			return errors.New("blinding protocol condition candidates must be bounded portable labels and cannot contain the unknown token")
		}
	}
	if _, err := time.Parse(time.RFC3339, protocol.CreatedAt); err != nil {
		return errors.New("blinding protocol timestamp must be RFC3339")
	}
	expected, err := blindingProtocolDigest(protocol)
	if err != nil || protocol.Digest != expected {
		return errors.New("blinding protocol digest is invalid")
	}
	return nil
}

func BuildBlindingProbeBatch(protocol BlindingProtocol, assignment ReviewAssignment, labels LabelBatch, drafts []BlindingProbeDraft, committedAt string) (BlindingProbeBatch, error) {
	if err := validateProbeBatchInputs(protocol, assignment, labels, drafts, committedAt); err != nil {
		return BlindingProbeBatch{}, err
	}
	labelByPacket := labelsByPacket(labels.Labels)
	probes := make([]BlindingProbe, 0, len(drafts))
	for _, draft := range drafts {
		probe, err := sealBlindingProbe(protocol, assignment, labels, labelByPacket[draft.PacketID], draft)
		if err != nil {
			return BlindingProbeBatch{}, err
		}
		probes = append(probes, probe)
	}
	sort.Slice(probes, func(left, right int) bool { return probes[left].PacketID < probes[right].PacketID })
	return SealBlindingProbeBatch(BlindingProbeBatch{Protocol: protocol, Assignment: assignment, LabelBatch: labels, CommittedAt: committedAt, Probes: probes})
}

func SealBlindingProbeBatch(batch BlindingProbeBatch) (BlindingProbeBatch, error) {
	batch.SchemaVersion, batch.CanonicalPolicy, batch.Digest = BlindingProbeBatchSchemaVersion, CanonicalPolicy, ""
	digest, err := blindingProbeBatchDigest(batch)
	if err != nil {
		return BlindingProbeBatch{}, err
	}
	batch.Digest = digest
	return batch, batch.Validate()
}

func (batch BlindingProbeBatch) Validate() error {
	if batch.SchemaVersion != BlindingProbeBatchSchemaVersion || batch.CanonicalPolicy != CanonicalPolicy || missing(batch.CommittedAt) || len(batch.Probes) == 0 {
		return errors.New("blinding probe batch identity, time, or probes are invalid")
	}
	if err := batch.Protocol.Validate(); err != nil {
		return err
	}
	if err := validateAssignmentBatchForProbe(batch.Protocol, batch.Assignment, batch.LabelBatch); err != nil {
		return err
	}
	committedAt, err := time.Parse(time.RFC3339, batch.CommittedAt)
	if err != nil {
		return errors.New("blinding probe batch timestamp must be RFC3339")
	}
	for index, probe := range batch.Probes {
		if err := validateProbeAgainstBatch(probe, batch, committedAt); err != nil {
			return fmt.Errorf("blinding probe batch item %d: %w", index, err)
		}
		if index > 0 && batch.Probes[index-1].PacketID >= probe.PacketID {
			return errors.New("blinding probes must be unique and sorted by packet")
		}
	}
	if err := exactPacketCoverage(batch.Assignment.PacketIDs, probePacketIDs(batch.Probes)); err != nil {
		return err
	}
	expected, err := blindingProbeBatchDigest(batch)
	if err != nil || batch.Digest != expected {
		return errors.New("blinding probe batch digest is invalid")
	}
	return nil
}

func BuildBlindingAnalysis(bundle ReviewBundle, reveal MappingReveal, mappings []PrivateMapping, left, right BlindingProbeBatch, analyzedAt string) (BlindingAnalysis, error) {
	if err := validateBlindingAnalysisInputs(bundle, reveal, mappings, left, right, analyzedAt); err != nil {
		return BlindingAnalysis{}, err
	}
	mappingByPacket := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByPacket[mapping.PacketID] = mapping
	}
	items := buildBlindingItemResults(bundle, mappingByPacket, left, right)
	analysis := summarizeBlindingItems(items)
	analysis.BundleDigest, analysis.MappingRevealDigest, analysis.Protocol = bundle.Digest, reveal.Digest, left.Protocol
	analysis.ProbeBatchDigests = []string{left.Digest, right.Digest}
	sort.Strings(analysis.ProbeBatchDigests)
	analysis.Packets, analysis.Reviewers, analysis.AnalyzedAt = len(bundle.Items), 2, analyzedAt
	analysis.Limitations = blindingAnalysisLimitations(analysis)
	return SealBlindingAnalysis(analysis)
}

func SealBlindingAnalysis(analysis BlindingAnalysis) (BlindingAnalysis, error) {
	analysis.SchemaVersion, analysis.CanonicalPolicy, analysis.Digest = BlindingAnalysisSchemaVersion, CanonicalPolicy, ""
	digest, err := blindingAnalysisDigest(analysis)
	if err != nil {
		return BlindingAnalysis{}, err
	}
	analysis.Digest = digest
	return analysis, analysis.Validate()
}

func (analysis BlindingAnalysis) Validate() error {
	if analysis.SchemaVersion != BlindingAnalysisSchemaVersion || analysis.CanonicalPolicy != CanonicalPolicy || !validDigest(analysis.BundleDigest) ||
		!validDigest(analysis.MappingRevealDigest) || analysis.Packets < 1 || analysis.Reviewers != 2 ||
		analysis.Observations != analysis.Packets*analysis.Reviewers || len(analysis.Items) != analysis.Packets {
		return errors.New("blinding analysis identity, bindings, or denominators are invalid")
	}
	if err := uniqueSortedDigests("blinding analysis probe batches", analysis.ProbeBatchDigests); err != nil || len(analysis.ProbeBatchDigests) != 2 {
		return errors.New("blinding analysis requires two distinct probe batch digests")
	}
	if _, err := time.Parse(time.RFC3339, analysis.AnalyzedAt); err != nil {
		return errors.New("blinding analysis timestamp must be RFC3339")
	}
	if err := analysis.Protocol.Validate(); err != nil {
		return err
	}
	if analysis.Protocol.BundleDigest != analysis.BundleDigest {
		return errors.New("blinding analysis protocol does not bind its bundle")
	}
	if err := validateBlindingItems(analysis.Items, analysis.Protocol); err != nil {
		return err
	}
	recomputed := summarizeBlindingItems(analysis.Items)
	recomputed.Packets, recomputed.Reviewers = len(analysis.Items), 2
	if !equalBlindingSummary(analysis, recomputed) || !equalStrings(analysis.Limitations, blindingAnalysisLimitations(recomputed)) ||
		!boundedProbability(analysis.ConditionGuessAccuracy) || !validInterval(analysis.AccuracyInterval, 0, 1) ||
		!boundedProbability(analysis.GuessCoverage) || !boundedProbability(analysis.SelectiveAccuracy) ||
		!validInterval(analysis.SelectiveAccuracyInterval, 0, 1) || !boundedProbability(analysis.ExpectedChanceAccuracy) ||
		!finite(analysis.CohenKappa) || analysis.CohenKappa < -1 || analysis.CohenKappa > 1 || !boundedProbability(analysis.MeanConfidence) ||
		!boundedProbability(analysis.GuessBrierScore) || !boundedProbability(analysis.RecognizedTaskRate) ||
		!validInterval(analysis.RecognitionRateInterval, 0, 1) {
		return errors.New("blinding analysis statistics or limitations are inconsistent with item results")
	}
	expected, err := blindingAnalysisDigest(analysis)
	if err != nil || analysis.Digest != expected {
		return errors.New("blinding analysis digest is invalid")
	}
	return nil
}

func validateBlindingAnalysisForLedger(analysis BlindingAnalysis, bundle ReviewBundle, reveal MappingReveal) error {
	if err := analysis.Validate(); err != nil {
		return err
	}
	if analysis.BundleDigest != bundle.Digest || analysis.MappingRevealDigest != reveal.Digest {
		return errors.New("adjudication ledger blinding analysis does not bind its bundle and mapping reveal")
	}
	if err := exactPacketCoverage(reviewBundlePacketIDs(bundle), blindingItemPacketIDs(analysis.Items)); err != nil {
		return err
	}
	return requireNotBefore("blinding analysis", analysis.AnalyzedAt, reveal.RevealedAt)
}

func DecodeBlindingProtocol(reader io.Reader) (BlindingProtocol, error) {
	var value BlindingProtocol
	if err := decodeStrict(reader, &value); err != nil {
		return BlindingProtocol{}, fmt.Errorf("decode blinding protocol: %w", err)
	}
	return value, value.Validate()
}

func DecodeBlindingProbeDrafts(reader io.Reader) ([]BlindingProbeDraft, error) {
	var values []BlindingProbeDraft
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode blinding probe drafts: %w", err)
	}
	return values, nil
}

func DecodeBlindingProbe(reader io.Reader) (BlindingProbe, error) {
	var value BlindingProbe
	if err := decodeStrict(reader, &value); err != nil {
		return BlindingProbe{}, fmt.Errorf("decode blinding probe: %w", err)
	}
	return value, value.Validate()
}

func DecodeBlindingProbeBatch(reader io.Reader) (BlindingProbeBatch, error) {
	var value BlindingProbeBatch
	if err := decodeStrict(reader, &value); err != nil {
		return BlindingProbeBatch{}, fmt.Errorf("decode blinding probe batch: %w", err)
	}
	return value, value.Validate()
}

func DecodeBlindingAnalysis(reader io.Reader) (BlindingAnalysis, error) {
	var value BlindingAnalysis
	if err := decodeStrict(reader, &value); err != nil {
		return BlindingAnalysis{}, fmt.Errorf("decode blinding analysis: %w", err)
	}
	return value, value.Validate()
}

func (probe BlindingProbe) Validate() error {
	if probe.SchemaVersion != BlindingProbeSchemaVersion || probe.CanonicalPolicy != CanonicalPolicy || !validDigest(probe.ProtocolDigest) ||
		!validDigest(probe.AssignmentDigest) || !validDigest(probe.LabelBatchDigest) || !validDigest(probe.LabelDigest) ||
		!validOpaquePacketID(probe.PacketID) || missing(probe.AdjudicatorAlias, probe.ConditionGuess, probe.SubmittedAt) ||
		probe.ReviewerSlot < 1 || probe.ReviewerSlot > 2 || !boundedProbability(probe.Confidence) || !validRecognition(probe.RecognizedTask, probe.RecognitionBasis) {
		return errors.New("blinding probe identity, bindings, guess, confidence, recognition, or time is invalid")
	}
	if probe.ConditionGuess == UnknownCondition && probe.Confidence != 0 {
		return errors.New("unknown blinding guess requires zero confidence")
	}
	if _, err := time.Parse(time.RFC3339, probe.SubmittedAt); err != nil {
		return errors.New("blinding probe timestamp must be RFC3339")
	}
	expected, err := blindingProbeDigest(probe)
	if err != nil || probe.Digest != expected {
		return errors.New("blinding probe digest is invalid")
	}
	return nil
}

func validateProbeBatchInputs(protocol BlindingProtocol, assignment ReviewAssignment, labels LabelBatch, drafts []BlindingProbeDraft, committedAt string) error {
	if err := protocol.Validate(); err != nil {
		return err
	}
	if err := validateAssignmentBatchForProbe(protocol, assignment, labels); err != nil {
		return err
	}
	if len(drafts) == 0 {
		return errors.New("blinding probe batch requires complete drafts")
	}
	if err := exactPacketCoverage(assignment.PacketIDs, probeDraftPacketIDs(drafts)); err != nil {
		return err
	}
	if err := requireStrictlyAfter("blinding probe batch commitment", committedAt, labels.CommittedAt); err != nil {
		return err
	}
	return nil
}

func validateAssignmentBatchForProbe(protocol BlindingProtocol, assignment ReviewAssignment, labels LabelBatch) error {
	if err := assignment.Validate(); err != nil {
		return err
	}
	if err := labels.Validate(); err != nil {
		return err
	}
	if assignment.Purpose != AssignmentPrimary || assignment.ReviewerSlot < 1 || assignment.ReviewerSlot > 2 || protocol.BundleDigest != assignment.BundleDigest {
		return errors.New("blinding probes require a primary assignment bound to the protocol bundle")
	}
	if err := validateLabelBatchAgainstAssignment(labels, assignment); err != nil {
		return err
	}
	return requireNotBefore("review assignment", assignment.AssignedAt, protocol.CreatedAt)
}

func sealBlindingProbe(protocol BlindingProtocol, assignment ReviewAssignment, labels LabelBatch, label Label, draft BlindingProbeDraft) (BlindingProbe, error) {
	probe := BlindingProbe{
		ProtocolDigest: protocol.Digest, AssignmentDigest: assignment.Digest, LabelBatchDigest: labels.Digest, LabelDigest: label.Digest,
		PacketID: draft.PacketID, AdjudicatorAlias: assignment.Reviewer.AdjudicatorAlias, ReviewerSlot: assignment.ReviewerSlot,
		ConditionGuess: draft.ConditionGuess, Confidence: draft.Confidence, RecognizedTask: draft.RecognizedTask,
		RecognitionBasis: draft.RecognitionBasis, SubmittedAt: draft.SubmittedAt,
	}
	probe.SchemaVersion, probe.CanonicalPolicy = BlindingProbeSchemaVersion, CanonicalPolicy
	if !validProbeGuess(protocol, probe) {
		return BlindingProbe{}, errors.New("blinding probe guess is outside the frozen protocol")
	}
	if err := requireStrictlyAfter("blinding probe submission", probe.SubmittedAt, labels.CommittedAt); err != nil {
		return BlindingProbe{}, err
	}
	probe.Digest = ""
	digest, err := blindingProbeDigest(probe)
	if err != nil {
		return BlindingProbe{}, err
	}
	probe.Digest = digest
	return probe, probe.Validate()
}

func validateProbeAgainstBatch(probe BlindingProbe, batch BlindingProbeBatch, committedAt time.Time) error {
	if err := probe.Validate(); err != nil {
		return err
	}
	labels := labelsByPacket(batch.LabelBatch.Labels)
	label, exists := labels[probe.PacketID]
	if !exists || probe.ProtocolDigest != batch.Protocol.Digest || probe.AssignmentDigest != batch.Assignment.Digest ||
		probe.LabelBatchDigest != batch.LabelBatch.Digest || probe.LabelDigest != label.Digest ||
		probe.AdjudicatorAlias != batch.Assignment.Reviewer.AdjudicatorAlias || probe.ReviewerSlot != batch.Assignment.ReviewerSlot || !validProbeGuess(batch.Protocol, probe) {
		return errors.New("blinding probe does not bind its protocol, assignment, label, or reviewer")
	}
	submittedAt, err := time.Parse(time.RFC3339, probe.SubmittedAt)
	if err != nil || submittedAt.After(committedAt) {
		return errors.New("blinding probe submission occurs after its batch commitment")
	}
	return requireStrictlyAfter("blinding probe submission", probe.SubmittedAt, batch.LabelBatch.CommittedAt)
}

func validateBlindingAnalysisInputs(bundle ReviewBundle, reveal MappingReveal, mappings []PrivateMapping, left, right BlindingProbeBatch, analyzedAt string) error {
	if err := bundle.Validate(); err != nil {
		return err
	}
	if err := reveal.Validate(); err != nil {
		return err
	}
	if err := left.Validate(); err != nil {
		return err
	}
	if err := right.Validate(); err != nil {
		return err
	}
	if left.Assignment.ReviewerSlot == right.Assignment.ReviewerSlot || left.Assignment.Reviewer.AdjudicatorAlias == right.Assignment.Reviewer.AdjudicatorAlias ||
		left.Protocol.Digest != right.Protocol.Digest || left.Protocol.BundleDigest != bundle.Digest {
		return errors.New("blinding analysis requires two independent primary probe batches under one protocol")
	}
	if reveal.BundleDigest != bundle.Digest || !equalReviewCommitments(reveal.PrimaryCommitments, primaryCommitments(left.Assignment, left.LabelBatch, right.Assignment, right.LabelBatch)) {
		return errors.New("blinding analysis reveal does not bind the probed primary review sequence")
	}
	references, err := mappingReferences(bundle, mappings)
	if err != nil || !equalMappingReferences(references, reveal.Mappings) {
		return errors.New("blinding analysis private mappings do not reproduce the reveal")
	}
	for _, batch := range []BlindingProbeBatch{left, right} {
		if err := requireStrictlyAfter("mapping reveal", reveal.RevealedAt, batch.CommittedAt); err != nil {
			return err
		}
	}
	return requireNotBefore("blinding analysis", analyzedAt, reveal.RevealedAt)
}

func validateBlindingItems(items []BlindingItemResult, protocol BlindingProtocol) error {
	reviewerAliases := make(map[int]string, 2)
	probeDigests := make(map[string]struct{}, len(items)*2)
	mappingDigests := make(map[string]struct{}, len(items))
	for index, item := range items {
		if !validOpaquePacketID(item.PacketID) || !validDigest(item.MappingDigest) || !validConditionCandidate(protocol, item.TruthCondition) || len(item.Guesses) != 2 ||
			index > 0 && items[index-1].PacketID >= item.PacketID {
			return errors.New("blinding analysis items must have unique sorted packets, truth, and two guesses")
		}
		if _, duplicate := mappingDigests[item.MappingDigest]; duplicate {
			return errors.New("blinding analysis reuses a private mapping digest")
		}
		mappingDigests[item.MappingDigest] = struct{}{}
		for guessIndex, guess := range item.Guesses {
			if guess.ReviewerSlot != guessIndex+1 || missing(guess.AdjudicatorAlias, guess.ConditionGuess) || !validDigest(guess.ProbeDigest) ||
				!boundedProbability(guess.Confidence) || !validRecognition(guess.RecognizedTask, guess.RecognitionBasis) ||
				guess.ConditionGuess != UnknownCondition && !validConditionCandidate(protocol, guess.ConditionGuess) ||
				guess.ConditionGuess == UnknownCondition && guess.Confidence != 0 || guess.Correct != (guess.ConditionGuess == item.TruthCondition) {
				return errors.New("blinding analysis guess identity, confidence, recognition, or correctness is invalid")
			}
			if alias, exists := reviewerAliases[guess.ReviewerSlot]; exists && alias != guess.AdjudicatorAlias {
				return errors.New("blinding analysis reviewer alias changes within a slot")
			}
			if _, duplicate := probeDigests[guess.ProbeDigest]; duplicate {
				return errors.New("blinding analysis reuses a probe digest")
			}
			reviewerAliases[guess.ReviewerSlot] = guess.AdjudicatorAlias
			probeDigests[guess.ProbeDigest] = struct{}{}
		}
		if item.Guesses[0].AdjudicatorAlias == item.Guesses[1].AdjudicatorAlias {
			return errors.New("blinding analysis requires distinct primary reviewers")
		}
	}
	return nil
}

func buildBlindingItemResults(bundle ReviewBundle, mappings map[string]PrivateMapping, left, right BlindingProbeBatch) []BlindingItemResult {
	leftByPacket, rightByPacket := probesByPacket(left.Probes), probesByPacket(right.Probes)
	items := make([]BlindingItemResult, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		packetID := item.Packet.PacketID
		mapping := mappings[packetID]
		condition := mapping.Condition
		guesses := []BlindingGuessResult{blindingGuessResult(leftByPacket[packetID], condition), blindingGuessResult(rightByPacket[packetID], condition)}
		sort.Slice(guesses, func(left, right int) bool { return guesses[left].ReviewerSlot < guesses[right].ReviewerSlot })
		items = append(items, BlindingItemResult{PacketID: packetID, MappingDigest: mapping.Digest, TruthCondition: condition, Guesses: guesses})
	}
	sort.Slice(items, func(left, right int) bool { return items[left].PacketID < items[right].PacketID })
	return items
}

func blindingGuessResult(probe BlindingProbe, truth string) BlindingGuessResult {
	return BlindingGuessResult{
		ReviewerSlot: probe.ReviewerSlot, AdjudicatorAlias: probe.AdjudicatorAlias, ConditionGuess: probe.ConditionGuess,
		Confidence: probe.Confidence, RecognizedTask: probe.RecognizedTask, RecognitionBasis: probe.RecognitionBasis,
		ProbeDigest: probe.Digest, Correct: probe.ConditionGuess == truth,
	}
}

func summarizeBlindingItems(items []BlindingItemResult) BlindingAnalysis {
	analysis := BlindingAnalysis{Items: append([]BlindingItemResult(nil), items...)}
	conditionCounts := make(map[string]*metricAccumulator)
	reviewerCounts := make(map[int]*metricAccumulator)
	predicted, actual := make(map[string]int), make(map[string]int)
	for _, item := range items {
		for _, guess := range item.Guesses {
			updateBlindingCounts(&analysis, conditionCounts, reviewerCounts, predicted, actual, item.TruthCondition, guess)
		}
	}
	finalizeBlindingSummary(&analysis, conditionCounts, reviewerCounts, predicted, actual)
	return analysis
}

type metricAccumulator struct {
	alias      string
	observed   int
	attempted  int
	correct    int
	recognized int
	confidence float64
	brier      float64
}

func updateBlindingCounts(analysis *BlindingAnalysis, conditions map[string]*metricAccumulator, reviewers map[int]*metricAccumulator, predicted, actual map[string]int, truth string, guess BlindingGuessResult) {
	analysis.Observations++
	actual[truth]++
	condition := getAccumulator(conditions, truth)
	reviewer := getReviewerAccumulator(reviewers, guess.ReviewerSlot, guess.AdjudicatorAlias)
	condition.observed++
	reviewer.observed++
	if guess.RecognizedTask {
		analysis.RecognizedTasks++
		condition.recognized++
		reviewer.recognized++
	}
	if guess.ConditionGuess == UnknownCondition {
		return
	}
	analysis.Attempts++
	predicted[guess.ConditionGuess]++
	condition.attempted++
	reviewer.attempted++
	condition.confidence += guess.Confidence
	reviewer.confidence += guess.Confidence
	target := 0.0
	if guess.Correct {
		analysis.Correct++
		condition.correct++
		reviewer.correct++
		target = 1
	}
	errorValue := guess.Confidence - target
	condition.brier += errorValue * errorValue
	reviewer.brier += errorValue * errorValue
}

func finalizeBlindingSummary(analysis *BlindingAnalysis, conditions map[string]*metricAccumulator, reviewers map[int]*metricAccumulator, predicted, actual map[string]int) {
	analysis.ConditionGuessAccuracy = ratio(analysis.Correct, analysis.Observations)
	analysis.AccuracyInterval = wilsonInterval(analysis.Correct, analysis.Observations)
	analysis.GuessCoverage = ratio(analysis.Attempts, analysis.Observations)
	analysis.RecognizedTaskRate = ratio(analysis.RecognizedTasks, analysis.Observations)
	analysis.RecognitionRateInterval = wilsonInterval(analysis.RecognizedTasks, analysis.Observations)
	if analysis.Attempts > 0 {
		analysis.SelectiveAccuracyDefined = true
		analysis.ConfidenceMetricsDefined = true
		analysis.SelectiveAccuracy = ratio(analysis.Correct, analysis.Attempts)
		analysis.SelectiveAccuracyInterval = wilsonInterval(analysis.Correct, analysis.Attempts)
		analysis.MeanConfidence, analysis.GuessBrierScore = accumulatorConfidence(reviewers, analysis.Attempts)
	}
	analysis.ExpectedChanceAccuracy = chanceAccuracy(predicted, actual, analysis.Observations)
	if analysis.Attempts > 0 && analysis.ExpectedChanceAccuracy != 1 {
		analysis.CohenKappaDefined = true
		analysis.CohenKappa = (analysis.ConditionGuessAccuracy - analysis.ExpectedChanceAccuracy) / (1 - analysis.ExpectedChanceAccuracy)
	}
	analysis.ByCondition = conditionMetrics(conditions)
	analysis.ByReviewer = reviewerMetrics(reviewers)
}

func conditionMetrics(values map[string]*metricAccumulator) []ConditionLeakageMetric {
	conditions := sortedAccumulatorKeys(values)
	result := make([]ConditionLeakageMetric, 0, len(conditions))
	for _, condition := range conditions {
		value := values[condition]
		result = append(result, ConditionLeakageMetric{
			Condition: condition, Observations: value.observed, Attempts: value.attempted, Correct: value.correct,
			Accuracy: ratio(value.correct, value.observed), SelectiveAccuracy: ratio(value.correct, value.attempted), Coverage: ratio(value.attempted, value.observed),
		})
	}
	return result
}

func reviewerMetrics(values map[int]*metricAccumulator) []ReviewerLeakageMetric {
	result := make([]ReviewerLeakageMetric, 0, len(values))
	for slot := 1; slot <= 2; slot++ {
		value := values[slot]
		metric := ReviewerLeakageMetric{
			ReviewerSlot: slot, AdjudicatorAlias: value.alias, Observations: value.observed, Attempts: value.attempted,
			Correct: value.correct, Accuracy: ratio(value.correct, value.observed), SelectiveAccuracy: ratio(value.correct, value.attempted),
			Coverage: ratio(value.attempted, value.observed), RecognizedTasks: value.recognized,
		}
		if value.attempted > 0 {
			metric.MeanConfidence, metric.GuessBrierScore = value.confidence/float64(value.attempted), value.brier/float64(value.attempted)
		}
		result = append(result, metric)
	}
	return result
}

func equalBlindingSummary(left, right BlindingAnalysis) bool {
	return left.Observations == right.Observations && left.Attempts == right.Attempts && left.Correct == right.Correct &&
		left.ConditionGuessAccuracy == right.ConditionGuessAccuracy && left.AccuracyInterval == right.AccuracyInterval &&
		left.GuessCoverage == right.GuessCoverage && left.SelectiveAccuracy == right.SelectiveAccuracy &&
		left.SelectiveAccuracyDefined == right.SelectiveAccuracyDefined && left.SelectiveAccuracyInterval == right.SelectiveAccuracyInterval &&
		left.ExpectedChanceAccuracy == right.ExpectedChanceAccuracy && left.CohenKappa == right.CohenKappa &&
		left.CohenKappaDefined == right.CohenKappaDefined && left.MeanConfidence == right.MeanConfidence &&
		left.GuessBrierScore == right.GuessBrierScore && left.ConfidenceMetricsDefined == right.ConfidenceMetricsDefined &&
		left.RecognizedTasks == right.RecognizedTasks && left.RecognizedTaskRate == right.RecognizedTaskRate &&
		left.RecognitionRateInterval == right.RecognitionRateInterval && equalConditionMetrics(left.ByCondition, right.ByCondition) &&
		equalReviewerMetrics(left.ByReviewer, right.ByReviewer)
}

func blindingAnalysisLimitations(analysis BlindingAnalysis) []string {
	values := []string{
		"Condition-guess accuracy measures detectable source-condition cues, not whether a cue changed an outcome label.",
		"Recognized-task reports can reflect legitimate prior familiarity and are not proof of reviewer misconduct.",
		"The candidate universe was disclosed only after label commitment, so the audit measures post-label identification under that fixed universe.",
	}
	if analysis.Packets < 30 {
		values = append(values, "Fewer than 30 packets make leakage estimates imprecise; Wilson intervals are reported and point estimates must remain descriptive.")
	}
	for _, metric := range analysis.ByCondition {
		if metric.Observations*2 > analysis.Observations {
			values = append(values, "Condition imbalance can inflate raw accuracy; expected-chance accuracy and Cohen kappa are reported alongside it.")
			break
		}
	}
	sort.Strings(values)
	return values
}

func validProbeGuess(protocol BlindingProtocol, probe BlindingProbe) bool {
	if probe.ConditionGuess == UnknownCondition {
		return probe.Confidence == 0
	}
	return validConditionCandidate(protocol, probe.ConditionGuess)
}

func validConditionCandidate(protocol BlindingProtocol, value string) bool {
	index := sort.SearchStrings(protocol.ConditionCandidates, value)
	return index < len(protocol.ConditionCandidates) && protocol.ConditionCandidates[index] == value
}

func validConditionName(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validRecognition(recognized bool, basis RecognitionBasis) bool {
	valid := basis == RecognitionNone || basis == RecognitionTaskText || basis == RecognitionTrajectoryContent || basis == RecognitionRepositoryFamiliarity || basis == RecognitionPriorExposure || basis == RecognitionOther
	return valid && (recognized && basis != RecognitionNone || !recognized && basis == RecognitionNone)
}

func uniqueMappingConditions(mappings []PrivateMapping) []string {
	set := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		set[mapping.Condition] = struct{}{}
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func equalMappingReferences(left, right []MappingReference) bool {
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

func probesByPacket(probes []BlindingProbe) map[string]BlindingProbe {
	values := make(map[string]BlindingProbe, len(probes))
	for _, probe := range probes {
		values[probe.PacketID] = probe
	}
	return values
}

func probePacketIDs(probes []BlindingProbe) []string {
	values := make([]string, 0, len(probes))
	for _, probe := range probes {
		values = append(values, probe.PacketID)
	}
	return values
}

func probeDraftPacketIDs(drafts []BlindingProbeDraft) []string {
	values := make([]string, 0, len(drafts))
	for _, draft := range drafts {
		values = append(values, draft.PacketID)
	}
	return values
}

func blindingItemPacketIDs(items []BlindingItemResult) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.PacketID)
	}
	return values
}

func getAccumulator(values map[string]*metricAccumulator, key string) *metricAccumulator {
	if values[key] == nil {
		values[key] = &metricAccumulator{}
	}
	return values[key]
}

func getReviewerAccumulator(values map[int]*metricAccumulator, slot int, alias string) *metricAccumulator {
	if values[slot] == nil {
		values[slot] = &metricAccumulator{alias: alias}
	}
	return values[slot]
}

func sortedAccumulatorKeys(values map[string]*metricAccumulator) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func accumulatorConfidence(values map[int]*metricAccumulator, attempts int) (float64, float64) {
	confidence, brier := 0.0, 0.0
	for _, value := range values {
		confidence += value.confidence
		brier += value.brier
	}
	return confidence / float64(attempts), brier / float64(attempts)
}

func chanceAccuracy(predicted, actual map[string]int, observations int) float64 {
	if observations == 0 {
		return 0
	}
	value := 0.0
	for condition, count := range predicted {
		value += float64(count*actual[condition]) / float64(observations*observations)
	}
	return value
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func wilsonInterval(successes, observations int) Interval {
	if observations == 0 {
		return Interval{}
	}
	z := 1.959963984540054
	n := float64(observations)
	p := float64(successes) / n
	denominator := 1 + z*z/n
	center := (p + z*z/(2*n)) / denominator
	margin := z * math.Sqrt((p*(1-p)+z*z/(4*n))/n) / denominator
	return Interval{Lower: math.Max(0, center-margin), Upper: math.Min(1, center+margin)}
}

func equalConditionMetrics(left, right []ConditionLeakageMetric) bool {
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

func equalReviewerMetrics(left, right []ReviewerLeakageMetric) bool {
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

func blindingProtocolPrompt() string {
	return "After committing outcome labels and before any mapping reveal, identify the most likely hidden source condition for each packet or select unknown; this response cannot alter the committed outcome label."
}

func blindingProtocolSecondaryMetrics() []string {
	return []string{"cohen_kappa", "guess_brier_score", "guess_coverage", "mean_confidence", "recognized_task_rate", "selective_accuracy", "wilson_interval_95"}
}

func blindingProtocolDigest(value BlindingProtocol) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func blindingProbeDigest(value BlindingProbe) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func blindingProbeBatchDigest(value BlindingProbeBatch) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func blindingAnalysisDigest(value BlindingAnalysis) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
