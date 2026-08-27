package relation

import (
	"bytes"
	"slices"
	"testing"
)

type relationJudgmentFixture struct {
	bundle        ReviewBundle
	mappings      []PrivateMapping
	qualification QualificationSet
	answerKey     QualificationAnswerKey
	handbook      ReviewerHandbook
	left          ReviewAssignment
	right         ReviewAssignment
}

func TestRelationJudgmentRevisionBatchAmbiguityAndTieBreak(t *testing.T) {
	fixture := newRelationJudgmentFixture(t)
	firstPacket, secondPacket := fixture.bundle.Packets[0].PacketID, fixture.bundle.Packets[1].PacketID

	initial := mustPairJudgment(t, fixture.bundle, fixture.left, relationJudgmentDraft(firstPacket, RatingLeft, ReasonTaskQualityDiffers, "2026-08-09T18:03:00Z", "initial independent response"), nil)
	revised := mustPairJudgment(t, fixture.bundle, fixture.left, relationJudgmentDraft(firstPacket, RatingEqual, ReasonNoJudgmentChange, "2026-08-09T18:04:00Z", "correct visible-side transcription"), &initial)
	if revised.Revision != 2 || revised.ParentDigest != initial.Digest || revised.Digest == initial.Digest {
		t.Fatal("relation judgment revision did not preserve an immutable parent chain")
	}
	leftSecond := mustPairJudgment(t, fixture.bundle, fixture.left, relationJudgmentDraft(secondPacket, RatingEqual, ReasonNoJudgmentChange, "2026-08-09T18:03:30Z", "initial independent response"), nil)
	leftBatch, err := BuildJudgmentBatch(fixture.bundle, fixture.left, []PairJudgment{leftSecond, revised}, "2026-08-09T18:05:00Z")
	if err != nil {
		t.Fatal(err)
	}

	rightFirst := mustPairJudgment(t, fixture.bundle, fixture.right, relationJudgmentDraft(firstPacket, RatingEqual, ReasonNoJudgmentChange, "2026-08-09T18:03:10Z", "initial independent response"), nil)
	rightSecond := mustPairJudgment(t, fixture.bundle, fixture.right, relationJudgmentDraft(secondPacket, RatingRight, ReasonTaskQualityDiffers, "2026-08-09T18:03:40Z", "initial independent response"), nil)
	rightBatch, err := BuildJudgmentBatch(fixture.bundle, fixture.right, []PairJudgment{rightSecond, rightFirst}, "2026-08-09T18:06:00Z")
	if err != nil {
		t.Fatal(err)
	}

	analysis, err := BuildRelationAmbiguityAnalysis(fixture.bundle, fixture.right, rightBatch, fixture.left, leftBatch, "2026-08-09T18:07:00Z")
	if err != nil {
		t.Fatal(err)
	}
	disagreementItem := analysis.Items[0]
	if disagreementItem.PacketID != secondPacket {
		disagreementItem = analysis.Items[1]
	}
	if analysis.RevealStatus != "not_revealed" || analysis.PacketsWithAnyDisagreement != 1 || !slices.Equal(analysis.TieBreakPacketIDs, []string{secondPacket}) ||
		disagreementItem.PacketID != secondPacket || !slices.Equal(disagreementItem.DisagreementAxes, []Axis{AxisSemanticQuality}) {
		t.Fatalf("unexpected prereveal ambiguity result: %#v", analysis)
	}

	tieReviewer, err := NewReviewerRecord("reviewer-tie", ReviewerRoleTieBreak, "2026-08-09T17:00:00Z", true, true, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	tieQualification, err := GradeQualification(fixture.qualification, fixture.answerKey, tieReviewer.ReviewerAlias, qualificationResponsesFromKey(fixture.answerKey), "2026-08-09T17:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	tie, err := BuildTieBreakAssignment(fixture.bundle, fixture.mappings, tieReviewer, tieQualification, analysis, fixture.left, leftBatch, fixture.right, rightBatch, bytes.Repeat([]byte{0x44}, 32), "2026-08-09T18:08:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if tie.Purpose != AssignmentPurposeTieBreak || tie.ReviewerSlot != 3 || !slices.Equal(tie.PacketIDs, []string{secondPacket}) {
		t.Fatalf("tie-break assignment escaped disagreement-only scope: %#v", tie.PacketIDs)
	}
	kit, err := BuildReviewerKit(fixture.bundle, tie, fixture.handbook, "2026-08-09T18:09:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReviewerKit(kit, fixture.bundle, tie); err != nil {
		t.Fatal(err)
	}
	tieJudgment := mustPairJudgment(t, fixture.bundle, tie, relationJudgmentDraft(secondPacket, RatingIndeterminate, ReasonInsufficientInformation, "2026-08-09T18:10:00Z", "independent tie-break response"), nil)
	tieBatch, err := BuildJudgmentBatch(fixture.bundle, tie, []PairJudgment{tieJudgment}, "2026-08-09T18:11:00Z")
	if err != nil {
		t.Fatal(err)
	}
	leftProbes, err := BuildConditionProbeBatch(fixture.bundle, fixture.left, leftBatch, relationProbeDrafts(fixture.left.PacketIDs, "2026-08-09T18:12:00Z"), "2026-08-09T18:13:00Z")
	if err != nil {
		t.Fatal(err)
	}
	rightProbes, err := BuildConditionProbeBatch(fixture.bundle, fixture.right, rightBatch, relationProbeDrafts(fixture.right.PacketIDs, "2026-08-09T18:12:10Z"), "2026-08-09T18:14:00Z")
	if err != nil {
		t.Fatal(err)
	}
	seeds := []AssignmentSeed{{fixture.left.Digest, bytes.Repeat([]byte{0x71}, 32)}, {fixture.right.Digest, bytes.Repeat([]byte{0x72}, 32)}, {tie.Digest, bytes.Repeat([]byte{0x44}, 32)}}
	if _, err := BuildMappingReveal(fixture.bundle, fixture.mappings, fixture.left, leftBatch, leftProbes, fixture.right, rightBatch, rightProbes, analysis, &tie, &tieBatch, seeds, "2026-08-09T18:14:00Z", "study-owner"); err == nil {
		t.Fatal("relation mapping reveal accepted a time not strictly after every commitment")
	}
	reveal, err := BuildMappingReveal(fixture.bundle, fixture.mappings, fixture.left, leftBatch, leftProbes, fixture.right, rightBatch, rightProbes, analysis, &tie, &tieBatch, seeds, "2026-08-09T18:15:00Z", "study-owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(reveal.Mappings) != 2 || len(reveal.OrderingSeeds) != 3 || reveal.TieBreakBatchDigest != tieBatch.Digest || reveal.ExternalActionStatus != ExternalActionNotAuthorized {
		t.Fatalf("relation mapping reveal lost complete commit-before-reveal custody: %#v", reveal)
	}
}

func TestRelationJudgmentGatesPartialLateAndOutcomeShapedInputs(t *testing.T) {
	fixture := newRelationJudgmentFixture(t)
	packetID := fixture.bundle.Packets[0].PacketID
	judgment := mustPairJudgment(t, fixture.bundle, fixture.left, relationJudgmentDraft(packetID, RatingEqual, ReasonNoJudgmentChange, "2026-08-09T18:03:00Z", "initial response"), nil)
	if _, err := BuildJudgmentBatch(fixture.bundle, fixture.left, []PairJudgment{judgment}, "2026-08-09T18:04:00Z"); err == nil {
		t.Fatal("relation judgment batch accepted partial assignment coverage")
	}
	late := relationJudgmentDraft(packetID, RatingEqual, ReasonNoJudgmentChange, "2026-08-09T18:01:00Z", "pre-assignment response")
	if _, err := BuildPairJudgment(fixture.bundle, fixture.left, late, nil); err == nil {
		t.Fatal("relation judgment accepted a pre-assignment submission")
	}
	unknownReason := relationJudgmentDraft(packetID, RatingEqual, "outcome_solved", "2026-08-09T18:03:00Z", "outcome-shaped response")
	if _, err := BuildPairJudgment(fixture.bundle, fixture.left, unknownReason, nil); err == nil {
		t.Fatal("relation judgment accepted an outcome-shaped reason")
	}
}

func newRelationJudgmentFixture(t *testing.T) relationJudgmentFixture {
	t.Helper()
	plan, material, context := relationCandidateBlindingFixture(t)
	packetA, mappingA, err := buildBlindedPacket(plan, material, context, "relation-judgment-key-a", bytes.Repeat([]byte{0x31}, 32))
	if err != nil {
		t.Fatal(err)
	}
	packetB, mappingB, err := buildBlindedPacket(plan, material, context, "relation-judgment-key-b", bytes.Repeat([]byte{0x52}, 32))
	if err != nil {
		t.Fatal(err)
	}
	qualification, answerKey, err := DefaultQualification(plan, "qualification-judgment-key", bytes.Repeat([]byte{0x19}, 32))
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(plan, qualification)
	if err != nil {
		t.Fatal(err)
	}
	mappings := []PrivateMapping{mappingA, mappingB}
	bundle, err := BuildReviewBundle(plan, digestText("pilot-sample"), ReviewDataDevelopmentPilot, []BlindPacket{packetA, packetB}, mappings, qualification, handbook, "2026-08-09T18:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	left := mustPrimaryAssignment(t, bundle, mappings, qualification, answerKey, "reviewer-left", 1, 0x71)
	right := mustPrimaryAssignment(t, bundle, mappings, qualification, answerKey, "reviewer-right", 2, 0x72)
	return relationJudgmentFixture{bundle: bundle, mappings: mappings, qualification: qualification, answerKey: answerKey, handbook: handbook, left: left, right: right}
}

func mustPrimaryAssignment(t *testing.T, bundle ReviewBundle, mappings []PrivateMapping, qualification QualificationSet, key QualificationAnswerKey, alias string, slot int, seed byte) ReviewAssignment {
	t.Helper()
	reviewer, err := NewReviewerRecordForProtocol(bundle.ProtocolVersion, alias, ReviewerRolePrimary, "2026-08-09T17:00:00Z", true, true, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := GradeQualification(qualification, key, alias, qualificationResponsesFromKey(key), "2026-08-09T17:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := BuildPrimaryAssignment(bundle, mappings, reviewer, report, slot, bytes.Repeat([]byte{seed}, 32), "2026-08-09T18:02:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return assignment
}

func relationJudgmentDraft(packetID string, semantic Rating, reason ReasonCode, submittedAt, revisionReason string) PairJudgmentDraft {
	return PairJudgmentDraft{
		PacketID: packetID,
		Observations: []VisibleAxisObservation{
			{Axis: AxisCausalIntegrity, Rating: RatingNotApplicable},
			{Axis: AxisEvidenceStrength, Rating: RatingEqual},
			{Axis: AxisExecutableSupport, Rating: RatingNotApplicable},
			{Axis: AxisInformation, Rating: RatingSufficient},
			{Axis: AxisPresentation, Rating: RatingEqual},
			{Axis: AxisSemanticQuality, Rating: semantic},
			{Axis: AxisUntrustedControl, Rating: RatingNotApplicable},
		},
		ReasonCodes: []ReasonCode{reason}, SubmittedAt: submittedAt, RevisionReason: revisionReason,
	}
}

func relationProbeDrafts(packetIDs []string, submittedAt string) []ConditionProbeDraft {
	result := make([]ConditionProbeDraft, len(packetIDs))
	for index, packetID := range packetIDs {
		result[index] = ConditionProbeDraft{
			PacketID: packetID, FamilyGuess: UnknownProbeValue, DirectionGuess: DirectionUnknown, SourceConditionGuess: UnknownProbeValue,
			RecognizedTask: false, TaskIdentityGuess: UnknownProbeValue, RecognitionBasis: RecognitionNone, Confidence: 0, SubmittedAt: submittedAt,
		}
	}
	return result
}

func mustPairJudgment(t *testing.T, bundle ReviewBundle, assignment ReviewAssignment, draft PairJudgmentDraft, parent *PairJudgment) PairJudgment {
	t.Helper()
	judgment, err := BuildPairJudgment(bundle, assignment, draft, parent)
	if err != nil {
		t.Fatal(err)
	}
	return judgment
}
