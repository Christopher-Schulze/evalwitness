package outcome

import (
	"slices"
	"testing"
)

type reviewWorkflowFixture struct {
	bundle        ReviewBundle
	mappings      []PrivateMapping
	leftReviewer  ReviewerRecord
	rightReviewer ReviewerRecord
	tieReviewer   ReviewerRecord
	leftReport    QualificationReport
	rightReport   QualificationReport
	tieReport     QualificationReport
}

func TestReviewWorkflowCommitsIndependentLabelsBeforeMappingReveal(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	leftAssignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	rightAssignment := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '2')
	leftBatch := mustReviewBatch(t, leftAssignment, map[string]State{
		fixture.bundle.Items[0].Packet.PacketID: StateSolved,
		fixture.bundle.Items[1].Packet.PacketID: StateSolved,
		fixture.bundle.Items[2].Packet.PacketID: StateUnsolved,
	}, "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	rightBatch := mustReviewBatch(t, rightAssignment, map[string]State{
		fixture.bundle.Items[0].Packet.PacketID: StateSolved,
		fixture.bundle.Items[1].Packet.PacketID: StateUnsolved,
		fixture.bundle.Items[2].Packet.PacketID: StateUnsolved,
	}, "2026-08-09T14:35:00Z", "2026-08-09T15:00:00Z")
	tieAssignment, err := BuildTieBreakAssignment(
		fixture.bundle, fixture.tieReviewer, fixture.tieReport, leftAssignment, leftBatch, rightAssignment, rightBatch,
		reviewSeed('3'), "2026-08-09T15:10:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	wantedTiePacket := fixture.bundle.Items[1].Packet.PacketID
	if len(tieAssignment.PacketIDs) != 1 || tieAssignment.PacketIDs[0] != wantedTiePacket {
		t.Fatalf("tie-break assignment = %#v", tieAssignment.PacketIDs)
	}
	tieBatch := mustReviewBatch(t, tieAssignment, map[string]State{wantedTiePacket: StateSolved}, "2026-08-09T15:20:00Z", "2026-08-09T15:30:00Z")
	reveal, err := BuildMappingReveal(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch,
		fixture.mappings, reviewAssignmentSeeds(leftAssignment, rightAssignment, &tieAssignment, '1', '2', '3'), "2026-08-09T16:00:00Z", "study-owner",
	)
	if err != nil {
		t.Fatal(err)
	}
	blinding := mustBlindingAnalysis(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch, reveal)
	rubricAmbiguity := mustRubricAmbiguity(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch)
	result, err := BuildAdjudicationLedger(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch, reveal,
		rubricAmbiguity, blinding, "2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "review-workflow-bootstrap-v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ledger.Status != AdjudicationComplete || len(result.Ledger.UnresolvedPacketIDs) != 0 || len(result.Resolutions) != len(fixture.bundle.Items) {
		t.Fatalf("adjudication result = %#v", result)
	}
	if result.Agreement.RawAgreement != 2.0/3.0 || result.Agreement.Pairs != 3 {
		t.Fatalf("agreement = %#v", result.Agreement)
	}
	for _, resolution := range result.Resolutions {
		if resolution.PacketID == wantedTiePacket && resolution.AgreementState != "third_adjudicator_resolution" {
			t.Fatalf("disagreement resolution = %#v", resolution)
		}
	}
}

func TestReviewWorkflowFailsClosedAtStudyBoundaries(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	leftAssignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	rightAssignment := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '2')
	leftStates := allReviewStates(fixture.bundle, StateSolved)
	rightStates := allReviewStates(fixture.bundle, StateSolved)
	rightStates[fixture.bundle.Items[1].Packet.PacketID] = StateUnsolved
	leftBatch := mustReviewBatch(t, leftAssignment, leftStates, "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	rightBatch := mustReviewBatch(t, rightAssignment, rightStates, "2026-08-09T14:35:00Z", "2026-08-09T15:00:00Z")

	t.Run("incomplete primary batch", func(t *testing.T) {
		labels := labelsForAssignment(t, leftAssignment, leftStates, "2026-08-09T14:30:00Z")
		if _, err := BuildLabelBatch(leftAssignment, labels[:len(labels)-1], "2026-08-09T15:00:00Z"); err == nil {
			t.Fatal("accepted an incomplete primary label commitment")
		}
	})

	t.Run("tie break before primary commitment", func(t *testing.T) {
		if _, err := BuildTieBreakAssignment(
			fixture.bundle, fixture.tieReviewer, fixture.tieReport, leftAssignment, leftBatch, rightAssignment, rightBatch,
			reviewSeed('3'), "2026-08-09T14:59:59Z",
		); err == nil {
			t.Fatal("accepted a tie-break assignment before primary labels were committed")
		}
	})

	tieAssignment, err := BuildTieBreakAssignment(
		fixture.bundle, fixture.tieReviewer, fixture.tieReport, leftAssignment, leftBatch, rightAssignment, rightBatch,
		reviewSeed('3'), "2026-08-09T15:10:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	tieStates := map[string]State{tieAssignment.PacketIDs[0]: StateSolved}
	tieBatch := mustReviewBatch(t, tieAssignment, tieStates, "2026-08-09T15:20:00Z", "2026-08-09T15:30:00Z")

	t.Run("mapping reveal before tie commitment", func(t *testing.T) {
		if _, err := BuildMappingReveal(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch,
			fixture.mappings, reviewAssignmentSeeds(leftAssignment, rightAssignment, &tieAssignment, '1', '2', '3'), "2026-08-09T15:29:59Z", "study-owner",
		); err == nil {
			t.Fatal("accepted mapping reveal before the tie-break commitment")
		}
	})

	t.Run("incomplete mapping reveal", func(t *testing.T) {
		if _, err := BuildMappingReveal(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch,
			fixture.mappings[:len(fixture.mappings)-1], reviewAssignmentSeeds(leftAssignment, rightAssignment, &tieAssignment, '1', '2', '3'), "2026-08-09T16:00:00Z", "study-owner",
		); err == nil {
			t.Fatal("accepted an incomplete mapping reveal")
		}
	})

	t.Run("unreproducible assignment randomization", func(t *testing.T) {
		if _, err := BuildMappingReveal(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch,
			fixture.mappings, reviewAssignmentSeeds(leftAssignment, rightAssignment, &tieAssignment, '9', '2', '3'), "2026-08-09T16:00:00Z", "study-owner",
		); err == nil {
			t.Fatal("accepted an ordering seed that did not reproduce the committed assignment")
		}
	})

	t.Run("resealed incomplete mapping evidence", func(t *testing.T) {
		reveal, revealErr := BuildMappingReveal(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch,
			fixture.mappings, reviewAssignmentSeeds(leftAssignment, rightAssignment, &tieAssignment, '1', '2', '3'), "2026-08-09T16:00:00Z", "study-owner",
		)
		if revealErr != nil {
			t.Fatal(revealErr)
		}
		reveal.Mappings = reveal.Mappings[:len(reveal.Mappings)-1]
		reveal, revealErr = SealMappingReveal(reveal)
		if revealErr != nil {
			t.Fatal(revealErr)
		}
		if _, ledgerErr := BuildAdjudicationLedger(
			fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, &tieAssignment, &tieBatch, reveal,
			RubricAmbiguityAnalysis{}, BlindingAnalysis{}, "2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "review-workflow-bootstrap-v1",
		); ledgerErr == nil {
			t.Fatal("accepted a resealed reveal without complete packet mappings")
		}
	})

	t.Run("same reviewer in both primary slots", func(t *testing.T) {
		duplicate := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 2, "2026-08-09T14:00:00Z", '4')
		duplicateBatch := mustReviewBatch(t, duplicate, leftStates, "2026-08-09T14:35:00Z", "2026-08-09T15:00:00Z")
		if _, err := BuildMappingReveal(
			fixture.bundle, leftAssignment, leftBatch, duplicate, duplicateBatch, nil, nil,
			fixture.mappings, reviewAssignmentSeeds(leftAssignment, duplicate, nil, '1', '4', 0), "2026-08-09T16:00:00Z", "study-owner",
		); err == nil {
			t.Fatal("accepted one reviewer in both primary slots")
		}
	})

	t.Run("public bundle with restricted packet", func(t *testing.T) {
		items := append([]ReviewItem(nil), fixture.bundle.Items...)
		items[0].Packet.PublicReleasable = false
		items[0].Packet, err = SealBlindPacket(items[0].Packet, items[0].Packet.PacketID)
		if err != nil {
			t.Fatal(err)
		}
		plan := validPlan(t)
		set, setErr := DefaultQualificationSet()
		if setErr != nil {
			t.Fatal(setErr)
		}
		handbook, handbookErr := DefaultReviewerHandbook(set)
		if handbookErr != nil {
			t.Fatal(handbookErr)
		}
		if _, buildErr := BuildReviewBundle(plan, set, handbook, items, ReviewDataTest, ReviewVisibilityPublic, "2026-08-09T12:00:00Z"); buildErr == nil {
			t.Fatal("accepted a restricted packet in a public review bundle")
		}
	})
}

func TestReviewAssignmentOrderIsDeterministicAndReviewerSpecific(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	first := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	repeated := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	other := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '1')
	if first.Digest != repeated.Digest || !slices.Equal(first.PacketIDs, repeated.PacketIDs) {
		t.Fatal("identical assignment inputs were not deterministic")
	}
	if first.OrderingSeedDigest != other.OrderingSeedDigest || first.Digest == other.Digest {
		t.Fatal("assignment commitment was not reviewer-specific")
	}
}

func TestReviewWorkflowKeepsUnresolvedDisagreementExplicitWithoutTieBreak(t *testing.T) {
	fixture := newReviewWorkflowFixture(t)
	leftAssignment := mustPrimaryAssignment(t, fixture, fixture.leftReviewer, fixture.leftReport, 1, "2026-08-09T14:00:00Z", '1')
	rightAssignment := mustPrimaryAssignment(t, fixture, fixture.rightReviewer, fixture.rightReport, 2, "2026-08-09T14:00:00Z", '2')
	leftStates := allReviewStates(fixture.bundle, StateSolved)
	rightStates := allReviewStates(fixture.bundle, StateSolved)
	rightStates[fixture.bundle.Items[1].Packet.PacketID] = StateUnsolved
	leftBatch := mustReviewBatch(t, leftAssignment, leftStates, "2026-08-09T14:30:00Z", "2026-08-09T15:00:00Z")
	rightBatch := mustReviewBatch(t, rightAssignment, rightStates, "2026-08-09T14:35:00Z", "2026-08-09T15:00:00Z")
	reveal, err := BuildMappingReveal(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil,
		fixture.mappings, reviewAssignmentSeeds(leftAssignment, rightAssignment, nil, '1', '2', 0), "2026-08-09T16:00:00Z", "study-owner",
	)
	if err != nil {
		t.Fatal(err)
	}
	blinding := mustBlindingAnalysis(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch, reveal)
	rubricAmbiguity := mustRubricAmbiguity(t, fixture, leftAssignment, leftBatch, rightAssignment, rightBatch)
	result, err := BuildAdjudicationLedger(
		fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, nil, nil, reveal,
		rubricAmbiguity, blinding, "2026-08-09T16:05:00Z", "third independent reviewer or unresolved", 10_000, "review-workflow-bootstrap-v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ledger.Status != AdjudicationUnresolved || !slices.Equal(result.Ledger.UnresolvedPacketIDs, []string{fixture.bundle.Items[1].Packet.PacketID}) {
		t.Fatalf("unresolved ledger = %#v", result.Ledger)
	}
}

func newReviewWorkflowFixture(t *testing.T) reviewWorkflowFixture {
	t.Helper()
	plan := validPlan(t)
	set, err := DefaultQualificationSet()
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(set)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]ReviewItem, 0, 3)
	mappings := make([]PrivateMapping, 0, 3)
	for index := 1; index <= 3; index++ {
		content := "independent task evidence " + string(rune('0'+index))
		condition := "workflow-fixture-alpha"
		if index > 1 {
			condition = "workflow-fixture-beta"
		}
		request := BlindBuildRequest{
			SchemaVersion: BlindBuildSchemaVersion, PlanDigest: plan.Digest, TaskAlias: "task-fixture-" + string(rune('0'+index)),
			Evidence:        []PacketEvidence{{Slot: "source", Kind: "trajectory", Content: content, ContentDigest: digestText(content), License: "MIT", Limitation: "synthetic workflow fixture"}},
			RubricQuestions: []string{"Is the implementation technically correct?"}, PrivacyClass: "public", PublicReleasable: true,
			SourceCaseDigest: digestText("private-case-" + content), Condition: condition, ExpectedRelation: "manual_outcome",
			SlotMappings: []SlotMapping{{Slot: "source", SourceDigest: digestText(content + "-source")}}, BlindingKeyID: "workflow-fixture-key",
			ForbiddenValues: []string{"manual_outcome", "workflow-fixture"},
		}
		packet, mapping, buildErr := BuildBlindedPacketFromRequest(request, reviewSeed('k'))
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		items = append(items, ReviewItem{TaskGroupID: "group-" + digestText("group-"+content), Packet: packet})
		mappings = append(mappings, mapping)
	}
	bundle, err := BuildReviewBundle(plan, set, handbook, items, ReviewDataTest, ReviewVisibilityPublic, "2026-08-09T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return reviewWorkflowFixture{
		bundle: bundle, mappings: mappings,
		leftReviewer:  mustReviewerRecord(t, "reviewer-left", ReviewerRolePrimary),
		rightReviewer: mustReviewerRecord(t, "reviewer-right", ReviewerRolePrimary),
		tieReviewer:   mustReviewerRecord(t, "reviewer-tie", ReviewerRoleTieBreak),
		leftReport:    mustQualificationReport(t, set, "reviewer-left"),
		rightReport:   mustQualificationReport(t, set, "reviewer-right"),
		tieReport:     mustQualificationReport(t, set, "reviewer-tie"),
	}
}

func mustReviewerRecord(t *testing.T, alias string, role ReviewerRole) ReviewerRecord {
	t.Helper()
	reviewer, err := NewReviewerRecord(alias, role, "2026-08-09T12:30:00Z", true, true, true, []string{})
	if err != nil {
		t.Fatal(err)
	}
	return reviewer
}

func mustQualificationReport(t *testing.T, set QualificationSet, reviewer string) QualificationReport {
	t.Helper()
	labels := make([]Label, 0, len(set.Cases))
	for _, item := range set.Cases {
		labels = append(labels, mustQualificationLabel(t, set, item, reviewer))
	}
	report, err := ScoreQualification(set, labels)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func mustPrimaryAssignment(t *testing.T, fixture reviewWorkflowFixture, reviewer ReviewerRecord, report QualificationReport, slot int, assignedAt string, seedByte byte) ReviewAssignment {
	t.Helper()
	assignment, err := BuildPrimaryAssignment(fixture.bundle, reviewer, report, slot, reviewSeed(seedByte), assignedAt)
	if err != nil {
		t.Fatal(err)
	}
	return assignment
}

func mustReviewBatch(t *testing.T, assignment ReviewAssignment, states map[string]State, submittedAt, committedAt string) LabelBatch {
	t.Helper()
	batch, err := BuildLabelBatch(assignment, labelsForAssignment(t, assignment, states, submittedAt), committedAt)
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func labelsForAssignment(t *testing.T, assignment ReviewAssignment, states map[string]State, submittedAt string) []Label {
	t.Helper()
	labels := make([]Label, 0, len(assignment.PacketIDs))
	for _, packetID := range assignment.PacketIDs {
		state, exists := states[packetID]
		if !exists {
			t.Fatalf("missing fixture state for %s", packetID)
		}
		label, err := SealLabel(Label{
			PacketID: packetID, AdjudicatorAlias: assignment.Reviewer.AdjudicatorAlias, ReviewerSlot: assignment.ReviewerSlot, PrimaryOutcome: state,
			TaskSatisfaction: RatingSufficient, TechnicalCorrectness: RatingSufficient, VerificationQuality: RatingSufficient,
			HarmfulSideEffects: RatingNotApplicable, EvidenceSufficiency: RatingSufficient, ReasonCodes: []ReasonCode{ReasonEvidenceConsistent},
			SubmittedAt: submittedAt, RubricVersion: assignment.RubricVersion,
			QualificationDigest: assignment.Qualification.Digest, ConflictsOfInterest: []string{},
		})
		if err != nil {
			t.Fatal(err)
		}
		labels = append(labels, label)
	}
	return labels
}

func allReviewStates(bundle ReviewBundle, state State) map[string]State {
	states := make(map[string]State, len(bundle.Items))
	for _, item := range bundle.Items {
		states[item.Packet.PacketID] = state
	}
	return states
}

func mustBlindingAnalysis(t *testing.T, fixture reviewWorkflowFixture, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, reveal MappingReveal) BlindingAnalysis {
	t.Helper()
	protocol, err := BuildBlindingProtocol(fixture.bundle, fixture.mappings, "2026-08-09T13:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	leftProbes := mustProbeBatch(t, protocol, fixture.mappings, leftAssignment, leftBatch)
	rightProbes := mustProbeBatch(t, protocol, fixture.mappings, rightAssignment, rightBatch)
	analysis, err := BuildBlindingAnalysis(fixture.bundle, reveal, fixture.mappings, leftProbes, rightProbes, "2026-08-09T16:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

func mustRubricAmbiguity(t *testing.T, fixture reviewWorkflowFixture, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch) RubricAmbiguityAnalysis {
	t.Helper()
	analysis, err := BuildRubricAmbiguityAnalysis(fixture.bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, "2026-08-09T15:05:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

func mustProbeBatch(t *testing.T, protocol BlindingProtocol, mappings []PrivateMapping, assignment ReviewAssignment, labels LabelBatch) BlindingProbeBatch {
	t.Helper()
	conditionByPacket := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		conditionByPacket[mapping.PacketID] = mapping.Condition
	}
	drafts := make([]BlindingProbeDraft, 0, len(assignment.PacketIDs))
	for _, packetID := range assignment.PacketIDs {
		drafts = append(drafts, BlindingProbeDraft{
			PacketID: packetID, ConditionGuess: conditionByPacket[packetID], Confidence: 0.8,
			RecognizedTask: false, RecognitionBasis: RecognitionNone, SubmittedAt: "2026-08-09T15:40:00Z",
		})
	}
	batch, err := BuildBlindingProbeBatch(protocol, assignment, labels, drafts, "2026-08-09T15:45:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func reviewSeed(value byte) []byte {
	seed := make([]byte, 32)
	for index := range seed {
		seed[index] = value
	}
	return seed
}

func reviewAssignmentSeeds(left, right ReviewAssignment, tie *ReviewAssignment, leftSeed, rightSeed, tieSeed byte) []AssignmentSeed {
	values := []AssignmentSeed{{AssignmentDigest: left.Digest, Seed: reviewSeed(leftSeed)}, {AssignmentDigest: right.Digest, Seed: reviewSeed(rightSeed)}}
	if tie != nil {
		values = append(values, AssignmentSeed{AssignmentDigest: tie.Digest, Seed: reviewSeed(tieSeed)})
	}
	return values
}
