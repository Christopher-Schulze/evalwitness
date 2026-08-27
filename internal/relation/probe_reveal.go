package relation

import (
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	ConditionProbeSchemaVersionV1      = "evalwitness.relation-condition-probe.v1"
	ConditionProbeSchemaVersionV2      = "evalwitness.relation-condition-probe.v2"
	ConditionProbeSchemaVersionV3      = "evalwitness.relation-condition-probe.v3"
	ConditionProbeSchemaVersion        = ConditionProbeSchemaVersionV1
	ConditionProbeBatchSchemaVersionV1 = "evalwitness.relation-condition-probe-batch.v1"
	ConditionProbeBatchSchemaVersionV2 = "evalwitness.relation-condition-probe-batch.v2"
	ConditionProbeBatchSchemaVersionV3 = "evalwitness.relation-condition-probe-batch.v3"
	ConditionProbeBatchSchemaVersion   = ConditionProbeBatchSchemaVersionV1
	MappingRevealSchemaVersionV1       = "evalwitness.relation-mapping-reveal.v1"
	MappingRevealSchemaVersionV2       = "evalwitness.relation-mapping-reveal.v2"
	MappingRevealSchemaVersionV3       = "evalwitness.relation-mapping-reveal.v3"
	MappingRevealSchemaVersion         = MappingRevealSchemaVersionV1
	UnknownProbeValue                  = "unknown"
)

type DirectionGuess string

const (
	DirectionLeftOriginal  DirectionGuess = "left_is_original"
	DirectionRightOriginal DirectionGuess = "right_is_original"
	DirectionUnknown       DirectionGuess = "unknown"
)

type RecognitionBasis string

const (
	RecognitionNone              RecognitionBasis = "none"
	RecognitionPriorExposure     RecognitionBasis = "prior_exposure"
	RecognitionRepositoryContext RecognitionBasis = "repository_context"
	RecognitionTaskText          RecognitionBasis = "task_text"
	RecognitionTraceContent      RecognitionBasis = "trace_content"
)

type ConditionProbeDraft struct {
	PacketID             string           `json:"packet_id"`
	FamilyGuess          string           `json:"family_guess"`
	DirectionGuess       DirectionGuess   `json:"direction_guess"`
	SourceConditionGuess string           `json:"source_condition_guess"`
	RecognizedTask       bool             `json:"recognized_task"`
	TaskIdentityGuess    string           `json:"task_identity_guess"`
	RecognitionBasis     RecognitionBasis `json:"recognition_basis"`
	Confidence           float64          `json:"confidence"`
	SubmittedAt          string           `json:"submitted_at"`
}

type ConditionProbe struct {
	SchemaVersion        string               `json:"schema_version"`
	CanonicalPolicy      string               `json:"canonical_policy"`
	ProtocolVersion      string               `json:"protocol_version"`
	Objective            ReviewObjective      `json:"review_objective"`
	PlanDigest           string               `json:"plan_digest"`
	BundleDigest         string               `json:"bundle_digest"`
	AssignmentDigest     string               `json:"assignment_digest"`
	JudgmentBatchDigest  string               `json:"judgment_batch_digest"`
	JudgmentDigest       string               `json:"judgment_digest"`
	PacketID             string               `json:"packet_id"`
	ReviewerAlias        string               `json:"reviewer_alias"`
	ReviewerSlot         int                  `json:"reviewer_slot"`
	FamilyGuess          string               `json:"family_guess"`
	DirectionGuess       DirectionGuess       `json:"direction_guess"`
	SourceConditionGuess string               `json:"source_condition_guess"`
	RecognizedTask       bool                 `json:"recognized_task"`
	TaskIdentityGuess    string               `json:"task_identity_guess"`
	RecognitionBasis     RecognitionBasis     `json:"recognition_basis"`
	Confidence           float64              `json:"confidence"`
	SubmittedAt          string               `json:"submitted_at"`
	ExternalActionStatus ExternalActionStatus `json:"external_action_status"`
	Digest               string               `json:"digest"`
}

type ConditionProbeBatch struct {
	SchemaVersion             string               `json:"schema_version"`
	CanonicalPolicy           string               `json:"canonical_policy"`
	ProtocolVersion           string               `json:"protocol_version"`
	Objective                 ReviewObjective      `json:"review_objective"`
	PlanDigest                string               `json:"plan_digest"`
	BundleDigest              string               `json:"bundle_digest"`
	AssignmentDigest          string               `json:"assignment_digest"`
	JudgmentBatchDigest       string               `json:"judgment_batch_digest"`
	ReviewerAlias             string               `json:"reviewer_alias"`
	ReviewerSlot              int                  `json:"reviewer_slot"`
	FamilyCandidates          []mutation.Family    `json:"family_candidates"`
	DirectionCandidates       []DirectionGuess     `json:"direction_candidates"`
	SourceConditionCandidates []string             `json:"source_condition_candidates"`
	UnknownToken              string               `json:"unknown_token"`
	CommittedAt               string               `json:"committed_at"`
	Probes                    []ConditionProbe     `json:"probes"`
	ExternalActionStatus      ExternalActionStatus `json:"external_action_status"`
	Digest                    string               `json:"digest"`
}

type MappingReference struct {
	PacketID      string `json:"packet_id"`
	MappingDigest string `json:"mapping_digest"`
}

type OrderingSeedReveal struct {
	AssignmentDigest string `json:"assignment_digest"`
	Seed             string `json:"seed"`
}

type AssignmentSeed struct {
	AssignmentDigest string
	Seed             []byte
}

type MappingReveal struct {
	SchemaVersion            string                        `json:"schema_version"`
	CanonicalPolicy          string                        `json:"canonical_policy"`
	ProtocolVersion          string                        `json:"protocol_version"`
	Objective                ReviewObjective               `json:"review_objective"`
	PlanDigest               string                        `json:"plan_digest"`
	BundleDigest             string                        `json:"bundle_digest"`
	AmbiguityAnalysisDigest  string                        `json:"ambiguity_analysis_digest"`
	PrimaryCommitments       []JudgmentCommitmentReference `json:"primary_commitments"`
	ProbeBatchDigests        []string                      `json:"probe_batch_digests"`
	TieBreakAssignmentDigest string                        `json:"tie_break_assignment_digest,omitempty"`
	TieBreakBatchDigest      string                        `json:"tie_break_batch_digest,omitempty"`
	OrderingSeeds            []OrderingSeedReveal          `json:"ordering_seeds"`
	Mappings                 []MappingReference            `json:"mappings"`
	RevealedAt               string                        `json:"revealed_at"`
	RevealedBy               string                        `json:"revealed_by"`
	ExternalActionStatus     ExternalActionStatus          `json:"external_action_status"`
	Digest                   string                        `json:"digest"`
}

func BuildConditionProbeBatch(bundle ReviewBundle, assignment ReviewAssignment, batch JudgmentBatch, drafts []ConditionProbeDraft, committedAt string) (ConditionProbeBatch, error) {
	if err := VerifyJudgmentBatch(batch, assignment, bundle); err != nil {
		return ConditionProbeBatch{}, err
	}
	if assignment.Purpose != AssignmentPurposePrimary || len(drafts) != len(assignment.PacketIDs) {
		return ConditionProbeBatch{}, errors.New("relation condition probes require one complete primary-assignment draft set")
	}
	judgments := judgmentsByPacket(batch.Judgments)
	probes := make([]ConditionProbe, len(drafts))
	seen := make(map[string]struct{}, len(drafts))
	for index, draft := range drafts {
		if _, exists := seen[draft.PacketID]; exists || !slices.Contains(assignment.PacketIDs, draft.PacketID) {
			return ConditionProbeBatch{}, errors.New("relation condition probe coverage is duplicated or outside the assignment")
		}
		seen[draft.PacketID] = struct{}{}
		probe, err := buildConditionProbe(bundle, assignment, batch, judgments[draft.PacketID], draft)
		if err != nil {
			return ConditionProbeBatch{}, err
		}
		probes[index] = probe
	}
	sort.Slice(probes, func(left, right int) bool { return probes[left].PacketID < probes[right].PacketID })
	result := ConditionProbeBatch{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: bundle.PlanDigest, BundleDigest: bundle.Digest,
		AssignmentDigest: assignment.Digest, JudgmentBatchDigest: batch.Digest, ReviewerAlias: assignment.Reviewer.ReviewerAlias, ReviewerSlot: assignment.ReviewerSlot,
		FamilyCandidates: probeFamilyCandidates(), DirectionCandidates: probeDirectionCandidates(), SourceConditionCandidates: probeSourceConditionCandidates(),
		UnknownToken: UnknownProbeValue, CommittedAt: committedAt, Probes: probes, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	if err := validateConditionProbeBatchAgainst(result, assignment, batch); err != nil {
		return ConditionProbeBatch{}, err
	}
	return SealConditionProbeBatch(result)
}

func buildConditionProbe(bundle ReviewBundle, assignment ReviewAssignment, batch JudgmentBatch, judgment PairJudgment, draft ConditionProbeDraft) (ConditionProbe, error) {
	if err := validateConditionProbeDraft(draft); err != nil {
		return ConditionProbe{}, err
	}
	if err := requireRelationTimeAfter("relation condition probe submission", draft.SubmittedAt, batch.CommittedAt); err != nil {
		return ConditionProbe{}, err
	}
	probe := ConditionProbe{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: bundle.PlanDigest, BundleDigest: bundle.Digest,
		AssignmentDigest: assignment.Digest, JudgmentBatchDigest: batch.Digest, JudgmentDigest: judgment.Digest, PacketID: draft.PacketID,
		ReviewerAlias: assignment.Reviewer.ReviewerAlias, ReviewerSlot: assignment.ReviewerSlot, FamilyGuess: draft.FamilyGuess,
		DirectionGuess: draft.DirectionGuess, SourceConditionGuess: draft.SourceConditionGuess, RecognizedTask: draft.RecognizedTask,
		TaskIdentityGuess: strings.TrimSpace(draft.TaskIdentityGuess), RecognitionBasis: draft.RecognitionBasis, Confidence: draft.Confidence,
		SubmittedAt: draft.SubmittedAt, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealConditionProbe(probe)
}

func SealConditionProbe(probe ConditionProbe) (ConditionProbe, error) {
	schemaVersion, err := schemaVersionForProtocol(probe.ProtocolVersion, ConditionProbeSchemaVersionV1, ConditionProbeSchemaVersionV2, ConditionProbeSchemaVersionV3)
	if err != nil {
		return ConditionProbe{}, err
	}
	probe.SchemaVersion, probe.CanonicalPolicy, probe.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := conditionProbeDigest(probe)
	if err != nil {
		return ConditionProbe{}, err
	}
	probe.Digest = digest
	return probe, probe.Validate()
}

func (probe ConditionProbe) Validate() error {
	if !validVersionedIdentity(probe.SchemaVersion, probe.ProtocolVersion, ConditionProbeSchemaVersionV1, ConditionProbeSchemaVersionV2, ConditionProbeSchemaVersionV3) || probe.CanonicalPolicy != CanonicalPolicy ||
		probe.Objective != ReviewObjectiveControlledRelation || !validDigest(probe.PlanDigest) || !validDigest(probe.BundleDigest) || !validDigest(probe.AssignmentDigest) ||
		!validDigest(probe.JudgmentBatchDigest) || !validDigest(probe.JudgmentDigest) || !validOpaqueID(probe.PacketID, "relation-packet-") ||
		strings.TrimSpace(probe.ReviewerAlias) == "" || probe.ReviewerSlot < 1 || probe.ReviewerSlot > 2 || probe.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation condition probe identity, objective, reviewer, or authorization boundary is invalid")
	}
	if err := validateConditionProbeDraft(ConditionProbeDraft{probe.PacketID, probe.FamilyGuess, probe.DirectionGuess, probe.SourceConditionGuess, probe.RecognizedTask, probe.TaskIdentityGuess, probe.RecognitionBasis, probe.Confidence, probe.SubmittedAt}); err != nil {
		return err
	}
	expected, err := conditionProbeDigest(probe)
	if err != nil || probe.Digest != expected {
		return errors.New("relation condition probe digest is invalid")
	}
	return nil
}

func SealConditionProbeBatch(batch ConditionProbeBatch) (ConditionProbeBatch, error) {
	schemaVersion, err := schemaVersionForProtocol(batch.ProtocolVersion, ConditionProbeBatchSchemaVersionV1, ConditionProbeBatchSchemaVersionV2, ConditionProbeBatchSchemaVersionV3)
	if err != nil {
		return ConditionProbeBatch{}, err
	}
	batch.SchemaVersion, batch.CanonicalPolicy, batch.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := conditionProbeBatchDigest(batch)
	if err != nil {
		return ConditionProbeBatch{}, err
	}
	batch.Digest = digest
	return batch, batch.Validate()
}

func (batch ConditionProbeBatch) Validate() error {
	if !validVersionedIdentity(batch.SchemaVersion, batch.ProtocolVersion, ConditionProbeBatchSchemaVersionV1, ConditionProbeBatchSchemaVersionV2, ConditionProbeBatchSchemaVersionV3) || batch.CanonicalPolicy != CanonicalPolicy ||
		batch.Objective != ReviewObjectiveControlledRelation || !validDigest(batch.PlanDigest) || !validDigest(batch.BundleDigest) || !validDigest(batch.AssignmentDigest) ||
		!validDigest(batch.JudgmentBatchDigest) || strings.TrimSpace(batch.ReviewerAlias) == "" || batch.ReviewerSlot < 1 || batch.ReviewerSlot > 2 || len(batch.Probes) == 0 ||
		!slices.Equal(batch.FamilyCandidates, probeFamilyCandidates()) || !slices.Equal(batch.DirectionCandidates, probeDirectionCandidates()) ||
		!slices.Equal(batch.SourceConditionCandidates, probeSourceConditionCandidates()) || batch.UnknownToken != UnknownProbeValue || batch.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation condition probe batch identity, candidates, coverage, or authorization boundary is invalid")
	}
	committedAt, err := time.Parse(time.RFC3339, batch.CommittedAt)
	if err != nil {
		return errors.New("relation condition probe batch commitment time must be RFC3339")
	}
	for index, probe := range batch.Probes {
		if err := probe.Validate(); err != nil {
			return err
		}
		if index > 0 && batch.Probes[index-1].PacketID >= probe.PacketID || probe.ProtocolVersion != batch.ProtocolVersion || probe.PlanDigest != batch.PlanDigest || probe.BundleDigest != batch.BundleDigest ||
			probe.AssignmentDigest != batch.AssignmentDigest || probe.JudgmentBatchDigest != batch.JudgmentBatchDigest || probe.ReviewerAlias != batch.ReviewerAlias || probe.ReviewerSlot != batch.ReviewerSlot {
			return errors.New("relation condition probes are unsorted, duplicated, or cross-bound")
		}
		submittedAt, _ := time.Parse(time.RFC3339, probe.SubmittedAt)
		if !committedAt.After(submittedAt) {
			return errors.New("relation condition probe batch must commit strictly after every probe")
		}
	}
	expected, err := conditionProbeBatchDigest(batch)
	if err != nil || batch.Digest != expected {
		return errors.New("relation condition probe batch digest is invalid")
	}
	return nil
}

func validateConditionProbeBatchAgainst(probes ConditionProbeBatch, assignment ReviewAssignment, judgments JudgmentBatch) error {
	if probes.ProtocolVersion != assignment.ProtocolVersion || probes.ProtocolVersion != judgments.ProtocolVersion || probes.AssignmentDigest != assignment.Digest || probes.JudgmentBatchDigest != judgments.Digest || probes.ReviewerAlias != assignment.Reviewer.ReviewerAlias ||
		probes.ReviewerSlot != assignment.ReviewerSlot || len(probes.Probes) != len(assignment.PacketIDs) {
		return errors.New("relation condition probe batch does not bind its exact judgment commitment")
	}
	want := append([]string(nil), assignment.PacketIDs...)
	got := make([]string, len(probes.Probes))
	judgmentByPacket := judgmentsByPacket(judgments.Judgments)
	for index, probe := range probes.Probes {
		got[index] = probe.PacketID
		if probe.JudgmentDigest != judgmentByPacket[probe.PacketID].Digest {
			return errors.New("relation condition probe does not bind the committed packet judgment")
		}
	}
	sort.Strings(want)
	return requireEqualPacketIDs("relation condition probe batch", want, got)
}

func BuildMappingReveal(bundle ReviewBundle, mappings []PrivateMapping, leftAssignment ReviewAssignment, leftBatch JudgmentBatch, leftProbes ConditionProbeBatch, rightAssignment ReviewAssignment, rightBatch JudgmentBatch, rightProbes ConditionProbeBatch, ambiguity RelationAmbiguityAnalysis, tieAssignment *ReviewAssignment, tieBatch *JudgmentBatch, seeds []AssignmentSeed, revealedAt, revealedBy string) (MappingReveal, error) {
	leftAssignment, leftBatch, rightAssignment, rightBatch, err := validatePrimaryJudgmentPair(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch)
	if err != nil {
		return MappingReveal{}, err
	}
	if err := verifyRelationAmbiguityAgainstPair(ambiguity, bundle, leftAssignment, leftBatch, rightAssignment, rightBatch); err != nil {
		return MappingReveal{}, err
	}
	leftProbes, rightProbes, err = probesBySlot(leftProbes, rightProbes)
	if err != nil || validateConditionProbeBatchAgainst(leftProbes, leftAssignment, leftBatch) != nil || validateConditionProbeBatchAgainst(rightProbes, rightAssignment, rightBatch) != nil {
		return MappingReveal{}, errors.New("relation mapping reveal probe batches do not bind both primary commitments")
	}
	if err := validateTieBreakForReveal(bundle, ambiguity, tieAssignment, tieBatch); err != nil {
		return MappingReveal{}, err
	}
	references, mappingByPacket, err := relationMappingReferences(bundle, mappings)
	if err != nil {
		return MappingReveal{}, err
	}
	assignments := []ReviewAssignment{leftAssignment, rightAssignment}
	if tieAssignment != nil {
		assignments = append(assignments, *tieAssignment)
	}
	seedReveals, err := buildRelationSeedReveals(bundle, mappingByPacket, assignments, seeds)
	if err != nil {
		return MappingReveal{}, err
	}
	boundary := latestRelationTime(ambiguity.AnalyzedAt, leftProbes.CommittedAt, rightProbes.CommittedAt)
	if tieBatch != nil {
		boundary = latestRelationTime(boundary, tieBatch.CommittedAt)
	}
	if strings.TrimSpace(revealedBy) == "" || requireRelationTimeAfter("relation mapping reveal", revealedAt, boundary) != nil {
		return MappingReveal{}, errors.New("relation mapping reveal requires an actor and a time after every judgment/probe commitment")
	}
	reveal := MappingReveal{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: bundle.PlanDigest, BundleDigest: bundle.Digest,
		AmbiguityAnalysisDigest: ambiguity.Digest, PrimaryCommitments: primaryJudgmentCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch),
		ProbeBatchDigests: []string{leftProbes.Digest, rightProbes.Digest}, OrderingSeeds: seedReveals, Mappings: references,
		RevealedAt: revealedAt, RevealedBy: strings.TrimSpace(revealedBy), ExternalActionStatus: ExternalActionNotAuthorized,
	}
	sort.Strings(reveal.ProbeBatchDigests)
	if tieAssignment != nil {
		reveal.TieBreakAssignmentDigest, reveal.TieBreakBatchDigest = tieAssignment.Digest, tieBatch.Digest
	}
	return SealMappingReveal(reveal)
}

func SealMappingReveal(reveal MappingReveal) (MappingReveal, error) {
	schemaVersion, err := schemaVersionForProtocol(reveal.ProtocolVersion, MappingRevealSchemaVersionV1, MappingRevealSchemaVersionV2, MappingRevealSchemaVersionV3)
	if err != nil {
		return MappingReveal{}, err
	}
	reveal.SchemaVersion, reveal.CanonicalPolicy, reveal.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := mappingRevealDigest(reveal)
	if err != nil {
		return MappingReveal{}, err
	}
	reveal.Digest = digest
	return reveal, reveal.Validate()
}

func (reveal MappingReveal) Validate() error {
	if !validVersionedIdentity(reveal.SchemaVersion, reveal.ProtocolVersion, MappingRevealSchemaVersionV1, MappingRevealSchemaVersionV2, MappingRevealSchemaVersionV3) || reveal.CanonicalPolicy != CanonicalPolicy ||
		reveal.Objective != ReviewObjectiveControlledRelation || !validDigest(reveal.PlanDigest) || !validDigest(reveal.BundleDigest) || !validDigest(reveal.AmbiguityAnalysisDigest) ||
		len(reveal.ProbeBatchDigests) != 2 || !slices.IsSorted(reveal.ProbeBatchDigests) || hasDuplicate(reveal.ProbeBatchDigests) || len(reveal.Mappings) == 0 ||
		len(reveal.OrderingSeeds) < 2 || strings.TrimSpace(reveal.RevealedBy) == "" || reveal.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation mapping reveal identity, commitments, actor, or authorization boundary is invalid")
	}
	if err := validateJudgmentCommitments(reveal.PrimaryCommitments); err != nil {
		return err
	}
	if (reveal.TieBreakAssignmentDigest == "") != (reveal.TieBreakBatchDigest == "") || reveal.TieBreakAssignmentDigest != "" && (!validDigest(reveal.TieBreakAssignmentDigest) || !validDigest(reveal.TieBreakBatchDigest)) {
		return errors.New("relation mapping reveal tie-break commitment is incomplete")
	}
	if _, err := time.Parse(time.RFC3339, reveal.RevealedAt); err != nil {
		return errors.New("relation mapping reveal time must be RFC3339")
	}
	for index, reference := range reveal.Mappings {
		if !validOpaqueID(reference.PacketID, "relation-packet-") || !validDigest(reference.MappingDigest) || index > 0 && reveal.Mappings[index-1].PacketID >= reference.PacketID {
			return errors.New("relation mapping references must be complete, valid, and sorted")
		}
	}
	for index, seed := range reveal.OrderingSeeds {
		if !validDigest(seed.AssignmentDigest) || len(seed.Seed) != 64 || index > 0 && reveal.OrderingSeeds[index-1].AssignmentDigest >= seed.AssignmentDigest {
			return errors.New("relation ordering seed reveals must be valid, unique, and sorted")
		}
		if seed.Seed != strings.ToLower(seed.Seed) {
			return errors.New("relation ordering seed reveal is not lowercase hexadecimal")
		}
		if _, err := hex.DecodeString(seed.Seed); err != nil {
			return errors.New("relation ordering seed reveal is not lowercase hexadecimal")
		}
	}
	expected, err := mappingRevealDigest(reveal)
	if err != nil || reveal.Digest != expected {
		return errors.New("relation mapping reveal digest is invalid")
	}
	return nil
}

func validateConditionProbeDraft(draft ConditionProbeDraft) error {
	if !validOpaqueID(draft.PacketID, "relation-packet-") || draft.Confidence < 0 || draft.Confidence > 1 ||
		!slices.Contains(probeDirectionCandidates(), draft.DirectionGuess) || !slices.Contains(probeSourceConditionCandidatesWithUnknown(), draft.SourceConditionGuess) ||
		!validFamilyGuess(draft.FamilyGuess) || !slices.Contains([]RecognitionBasis{RecognitionNone, RecognitionPriorExposure, RecognitionRepositoryContext, RecognitionTaskText, RecognitionTraceContent}, draft.RecognitionBasis) {
		return errors.New("relation condition probe guess, direction, confidence, or recognition basis is invalid")
	}
	guess := strings.TrimSpace(draft.TaskIdentityGuess)
	if !draft.RecognizedTask && (guess != UnknownProbeValue || draft.RecognitionBasis != RecognitionNone) || draft.RecognizedTask && (guess == "" || guess == UnknownProbeValue || len(guess) > 256 || draft.RecognitionBasis == RecognitionNone) {
		return errors.New("relation task-recognition declaration and identity guess disagree")
	}
	if _, err := time.Parse(time.RFC3339, draft.SubmittedAt); err != nil {
		return errors.New("relation condition probe submission time must be RFC3339")
	}
	return nil
}

func validateTieBreakForReveal(bundle ReviewBundle, ambiguity RelationAmbiguityAnalysis, assignment *ReviewAssignment, batch *JudgmentBatch) error {
	required := len(ambiguity.TieBreakPacketIDs) > 0
	if required != (assignment != nil && batch != nil) || !required && (assignment != nil || batch != nil) {
		return errors.New("relation mapping reveal tie-break presence contradicts committed disagreements")
	}
	if !required {
		return nil
	}
	if err := VerifyJudgmentBatch(*batch, *assignment, bundle); err != nil {
		return err
	}
	want, got := append([]string(nil), ambiguity.TieBreakPacketIDs...), append([]string(nil), assignment.PacketIDs...)
	sort.Strings(got)
	return requireEqualPacketIDs("relation mapping reveal tie-break", want, got)
}

func relationMappingReferences(bundle ReviewBundle, mappings []PrivateMapping) ([]MappingReference, map[string]PrivateMapping, error) {
	if err := VerifyReviewBundle(bundle, mappings); err != nil {
		return nil, nil, err
	}
	byPacket := make(map[string]PrivateMapping, len(mappings))
	references := make([]MappingReference, len(mappings))
	for index, mapping := range mappings {
		byPacket[mapping.PacketID] = mapping
		references[index] = MappingReference{mapping.PacketID, mapping.Digest}
	}
	sort.Slice(references, func(left, right int) bool { return references[left].PacketID < references[right].PacketID })
	return references, byPacket, nil
}

func buildRelationSeedReveals(bundle ReviewBundle, mappings map[string]PrivateMapping, assignments []ReviewAssignment, seeds []AssignmentSeed) ([]OrderingSeedReveal, error) {
	seedByAssignment := make(map[string][]byte, len(seeds))
	for _, seed := range seeds {
		if len(seed.Seed) < 32 || seedByAssignment[seed.AssignmentDigest] != nil {
			return nil, errors.New("relation mapping reveal requires one unique 32-byte seed per assignment")
		}
		seedByAssignment[seed.AssignmentDigest] = seed.Seed
	}
	if len(seedByAssignment) != len(assignments) {
		return nil, errors.New("relation mapping reveal seed coverage differs from assignments")
	}
	reveals := make([]OrderingSeedReveal, len(assignments))
	for index, assignment := range assignments {
		seed := seedByAssignment[assignment.Digest]
		if assignment.OrderingSeedDigest != digestText(string(seed)) || !assignmentOrderMatches(bundle, mappings, assignment, seed) {
			return nil, errors.New("relation mapping reveal seed does not reproduce an assignment")
		}
		reveals[index] = OrderingSeedReveal{assignment.Digest, hex.EncodeToString(seed)}
	}
	sort.Slice(reveals, func(left, right int) bool { return reveals[left].AssignmentDigest < reveals[right].AssignmentDigest })
	return reveals, nil
}

func assignmentOrderMatches(bundle ReviewBundle, mappings map[string]PrivateMapping, assignment ReviewAssignment, seed []byte) bool {
	want := append([]string(nil), assignment.PacketIDs...)
	sort.Slice(want, func(left, right int) bool {
		leftKey := relationKeyedDigest(seed, domainAssignmentPacketOrder, bundle.Digest, assignment.Reviewer.Digest, fmt.Sprint(assignment.ReviewerSlot), mappings[want[left]].ReviewerOrderKey)
		rightKey := relationKeyedDigest(seed, domainAssignmentPacketOrder, bundle.Digest, assignment.Reviewer.Digest, fmt.Sprint(assignment.ReviewerSlot), mappings[want[right]].ReviewerOrderKey)
		if leftKey == rightKey {
			return want[left] < want[right]
		}
		return leftKey < rightKey
	})
	return slices.Equal(want, assignment.PacketIDs)
}

func probesBySlot(left, right ConditionProbeBatch) (ConditionProbeBatch, ConditionProbeBatch, error) {
	if err := left.Validate(); err != nil {
		return ConditionProbeBatch{}, ConditionProbeBatch{}, err
	}
	if err := right.Validate(); err != nil {
		return ConditionProbeBatch{}, ConditionProbeBatch{}, err
	}
	if left.ReviewerSlot == 2 {
		left, right = right, left
	}
	if left.ProtocolVersion != right.ProtocolVersion || left.ReviewerSlot != 1 || right.ReviewerSlot != 2 {
		return ConditionProbeBatch{}, ConditionProbeBatch{}, errors.New("relation condition probe batches require primary slots one and two")
	}
	return left, right, nil
}

func validFamilyGuess(value string) bool {
	if value == UnknownProbeValue {
		return true
	}
	for _, candidate := range probeFamilyCandidates() {
		if value == string(candidate) {
			return true
		}
	}
	return false
}

func probeFamilyCandidates() []mutation.Family {
	values := make([]mutation.Family, len(defaultFamilyContracts()))
	for index, contract := range defaultFamilyContracts() {
		values[index] = contract.Family
	}
	return values
}

func probeDirectionCandidates() []DirectionGuess {
	return []DirectionGuess{DirectionLeftOriginal, DirectionRightOriginal, DirectionUnknown}
}

func probeSourceConditionCandidates() []string {
	return []string{"decoy", "negative", "relation"}
}

func probeSourceConditionCandidatesWithUnknown() []string {
	return append(probeSourceConditionCandidates(), UnknownProbeValue)
}

func latestRelationTime(values ...string) string {
	latest := values[0]
	latestTime, _ := time.Parse(time.RFC3339, latest)
	for _, value := range values[1:] {
		current, _ := time.Parse(time.RFC3339, value)
		if current.After(latestTime) {
			latest, latestTime = value, current
		}
	}
	return latest
}

func conditionProbeDigest(value ConditionProbe) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func conditionProbeBatchDigest(value ConditionProbeBatch) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func mappingRevealDigest(value MappingReveal) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
