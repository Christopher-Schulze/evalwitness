package relation

import (
	"bytes"
	"slices"
	"testing"
)

type relationTerminalFixture struct {
	plan        Plan
	bundle      ReviewBundle
	mappings    []PrivateMapping
	handbook    ReviewerHandbook
	ambiguity   RelationAmbiguityAnalysis
	left        ReviewAssignment
	leftBatch   JudgmentBatch
	leftProbes  ConditionProbeBatch
	right       ReviewAssignment
	rightBatch  JudgmentBatch
	rightProbes ConditionProbeBatch
	tie         ReviewAssignment
	tieBatch    JudgmentBatch
	reveal      MappingReveal
}

func TestFormalHumanComparisonAndTerminalLedgerPreserveThreeEvidenceLayers(t *testing.T) {
	fixture := newRelationTerminalFixture(t)
	result, err := BuildFormalHumanComparison(
		fixture.plan, fixture.bundle, fixture.mappings, fixture.reveal, fixture.ambiguity,
		fixture.left, fixture.leftBatch, fixture.leftProbes, fixture.right, fixture.rightBatch, fixture.rightProbes,
		&fixture.tie, &fixture.tieBatch, "2026-08-09T18:15:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Comparison.PacketStates != (StateTally{Denominator: 3, Supports: 1, Contradicts: 1, Unresolved: 1}) {
		t.Fatalf("unexpected formal-human states: %#v", result.Comparison.PacketStates)
	}
	if len(result.Comparison.FamilyDenominators) != 1 || result.Comparison.FamilyDenominators[0].Denominator != 3 ||
		len(result.Comparison.TaskGroupDenominators) != 1 || result.Comparison.TaskClusters.Contradicts != 1 ||
		result.Comparison.Sensitivity.UnresolvedRetained != 1 {
		t.Fatalf("formal-human comparison lost exact strata or unresolved sensitivity: %#v", result.Comparison)
	}
	for _, probes := range result.Comparison.ReviewerProbeSummaries {
		if probes.Packets != 3 || probes.FamilyGuess.Unknown != 3 || probes.DirectionGuess.Unknown != 3 || probes.SourceCondition.Unknown != 3 {
			t.Fatalf("unexpected post-label probe summary: %#v", probes)
		}
	}
	encoded, err := EncodeIndented(result.Comparison)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("verifier_score"), []byte("verifier_decision"), []byte("task_identity_guess")} {
		if bytes.Contains(encoded, forbidden) {
			t.Fatalf("formal-human comparison leaked forbidden verifier or private probe field %q", forbidden)
		}
	}
	ledger, err := BuildTerminalRelationLedger(fixture.bundle, fixture.reveal, fixture.ambiguity, result.Comparison, result.Resolutions, "2026-08-09T18:16:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Status != "complete_with_unresolved" || len(ledger.Entries) != 3 || ledger.PacketStates != result.Comparison.PacketStates {
		t.Fatalf("terminal relation ledger lost complete state custody: %#v", ledger)
	}
	for _, entry := range ledger.Entries {
		if entry.VerifierRelationStatus != VerifierRelationNotConsulted || !slices.Contains([]string{RelationAdmissibilityHumanSupported, RelationAdmissibilityHumanContradicted, RelationAdmissibilityHumanUnresolved}, entry.AdmissibilityStatus) {
			t.Fatalf("terminal relation ledger collapsed evidence layers: %#v", entry)
		}
	}
}

func TestRelationV2PostInspectionWorkflowIsVersionClosed(t *testing.T) {
	fixture := newRelationTerminalFixtureForProtocol(t, ProtocolVersionV2)
	result, err := BuildFormalHumanComparison(
		fixture.plan, fixture.bundle, fixture.mappings, fixture.reveal, fixture.ambiguity,
		fixture.left, fixture.leftBatch, fixture.leftProbes, fixture.right, fixture.rightBatch, fixture.rightProbes,
		&fixture.tie, &fixture.tieBatch, "2026-08-09T18:15:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := BuildTerminalRelationLedger(fixture.bundle, fixture.reveal, fixture.ambiguity, result.Comparison, result.Resolutions, "2026-08-09T18:16:00Z")
	if err != nil {
		t.Fatal(err)
	}
	kit, err := BuildReviewerKit(fixture.bundle, fixture.left, fixture.handbook, "2026-08-09T18:02:30Z")
	if err != nil {
		t.Fatal(err)
	}
	identities := []struct {
		name     string
		protocol string
		schema   string
		want     string
	}{
		{"bundle", fixture.bundle.ProtocolVersion, fixture.bundle.SchemaVersion, ReviewBundleSchemaVersionV2},
		{"reviewer", fixture.left.Reviewer.ProtocolVersion, fixture.left.Reviewer.SchemaVersion, ReviewerRecordSchemaVersionV2},
		{"assignment", fixture.left.ProtocolVersion, fixture.left.SchemaVersion, ReviewAssignmentSchemaVersionV2},
		{"reviewer kit", kit.ProtocolVersion, kit.SchemaVersion, ReviewerKitSchemaVersionV2},
		{"judgment", fixture.leftBatch.Judgments[0].ProtocolVersion, fixture.leftBatch.Judgments[0].SchemaVersion, PairJudgmentSchemaVersionV2},
		{"judgment batch", fixture.leftBatch.ProtocolVersion, fixture.leftBatch.SchemaVersion, JudgmentBatchSchemaVersionV2},
		{"ambiguity", fixture.ambiguity.ProtocolVersion, fixture.ambiguity.SchemaVersion, RelationAmbiguitySchemaVersionV2},
		{"probe", fixture.leftProbes.Probes[0].ProtocolVersion, fixture.leftProbes.Probes[0].SchemaVersion, ConditionProbeSchemaVersionV2},
		{"probe batch", fixture.leftProbes.ProtocolVersion, fixture.leftProbes.SchemaVersion, ConditionProbeBatchSchemaVersionV2},
		{"reveal", fixture.reveal.ProtocolVersion, fixture.reveal.SchemaVersion, MappingRevealSchemaVersionV2},
		{"resolution", result.Resolutions[0].ProtocolVersion, result.Resolutions[0].SchemaVersion, RelationResolutionSchemaVersionV2},
		{"translation", result.Resolutions[0].Translation.ProtocolVersion, result.Resolutions[0].Translation.SchemaVersion, TranslationResultSchemaVersionV2},
		{"comparison", result.Comparison.ProtocolVersion, result.Comparison.SchemaVersion, FormalHumanComparisonSchemaVersionV2},
		{"ledger", ledger.ProtocolVersion, ledger.SchemaVersion, TerminalRelationLedgerSchemaVersionV2},
	}
	for _, identity := range identities {
		if identity.protocol != ProtocolVersionV2 || identity.schema != identity.want {
			t.Fatalf("%s escaped the v2 protocol: protocol=%q schema=%q", identity.name, identity.protocol, identity.schema)
		}
	}
	tampered := result.Resolutions[0]
	tampered.ProtocolVersion = ProtocolVersionV1
	if _, err := SealRelationResolution(tampered); err == nil {
		t.Fatal("v1 resolution envelope accepted a nested v2 translation")
	}
	legacyReviewer, err := NewReviewerRecord("legacy-reviewer", ReviewerRolePrimary, "2026-08-09T17:00:00Z", true, true, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildPrimaryAssignment(fixture.bundle, fixture.mappings, legacyReviewer, fixture.left.Qualification, 1, bytes.Repeat([]byte{0x73}, 32), "2026-08-09T18:02:00Z"); err == nil {
		t.Fatal("v2 bundle accepted a v1 reviewer record")
	}
}

func TestRelationV3ReviewerToTerminalWorkflowIsVersionClosed(t *testing.T) {
	fixture := newRelationTerminalFixtureForProtocol(t, ProtocolVersionV3)
	result, err := BuildFormalHumanComparison(
		fixture.plan, fixture.bundle, fixture.mappings, fixture.reveal, fixture.ambiguity,
		fixture.left, fixture.leftBatch, fixture.leftProbes, fixture.right, fixture.rightBatch, fixture.rightProbes,
		&fixture.tie, &fixture.tieBatch, "2026-08-09T18:15:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := BuildTerminalRelationLedger(fixture.bundle, fixture.reveal, fixture.ambiguity, result.Comparison, result.Resolutions, "2026-08-09T18:16:00Z")
	if err != nil {
		t.Fatal(err)
	}
	kit, err := BuildReviewerKit(fixture.bundle, fixture.left, fixture.handbook, "2026-08-09T18:02:30Z")
	if err != nil {
		t.Fatal(err)
	}
	identities := []struct {
		name, protocol, schema, want string
	}{
		{"bundle", fixture.bundle.ProtocolVersion, fixture.bundle.SchemaVersion, ReviewBundleSchemaVersionV3},
		{"reviewer", fixture.left.Reviewer.ProtocolVersion, fixture.left.Reviewer.SchemaVersion, ReviewerRecordSchemaVersionV3},
		{"qualification report", fixture.left.Qualification.ProtocolVersion, fixture.left.Qualification.SchemaVersion, QualificationReportSchemaVersionV3},
		{"assignment", fixture.left.ProtocolVersion, fixture.left.SchemaVersion, ReviewAssignmentSchemaVersionV3},
		{"reviewer kit", kit.ProtocolVersion, kit.SchemaVersion, ReviewerKitSchemaVersionV3},
		{"judgment", fixture.leftBatch.Judgments[0].ProtocolVersion, fixture.leftBatch.Judgments[0].SchemaVersion, PairJudgmentSchemaVersionV3},
		{"judgment batch", fixture.leftBatch.ProtocolVersion, fixture.leftBatch.SchemaVersion, JudgmentBatchSchemaVersionV3},
		{"ambiguity", fixture.ambiguity.ProtocolVersion, fixture.ambiguity.SchemaVersion, RelationAmbiguitySchemaVersionV3},
		{"probe", fixture.leftProbes.Probes[0].ProtocolVersion, fixture.leftProbes.Probes[0].SchemaVersion, ConditionProbeSchemaVersionV3},
		{"probe batch", fixture.leftProbes.ProtocolVersion, fixture.leftProbes.SchemaVersion, ConditionProbeBatchSchemaVersionV3},
		{"reveal", fixture.reveal.ProtocolVersion, fixture.reveal.SchemaVersion, MappingRevealSchemaVersionV3},
		{"resolution", result.Resolutions[0].ProtocolVersion, result.Resolutions[0].SchemaVersion, RelationResolutionSchemaVersionV3},
		{"translation", result.Resolutions[0].Translation.ProtocolVersion, result.Resolutions[0].Translation.SchemaVersion, TranslationResultSchemaVersionV3},
		{"comparison", result.Comparison.ProtocolVersion, result.Comparison.SchemaVersion, FormalHumanComparisonSchemaVersionV3},
		{"ledger", ledger.ProtocolVersion, ledger.SchemaVersion, TerminalRelationLedgerSchemaVersionV3},
	}
	for _, identity := range identities {
		if identity.protocol != ProtocolVersionV3 || identity.schema != identity.want {
			t.Fatalf("%s escaped protocol v3: protocol=%q schema=%q", identity.name, identity.protocol, identity.schema)
		}
	}
	tampered := result.Resolutions[0]
	tampered.ProtocolVersion = ProtocolVersionV2
	if _, err := SealRelationResolution(tampered); err == nil {
		t.Fatal("v2 resolution envelope accepted a nested v3 translation")
	}
}

func TestRelationResolutionInformationInsufficiencyCannotBeOutvoted(t *testing.T) {
	resolved, rule := resolveVisibleRating(AxisInformation, RatingSufficient, RatingInsufficient, RatingSufficient)
	if resolved != RatingInsufficient || rule != "information_insufficiency_veto" {
		t.Fatalf("information insufficiency was converted by majority vote: %s %s", resolved, rule)
	}
}

func TestRelationResolutionMultiFactorAndAmbiguousTaskCaveatsForceUnresolvedInformation(t *testing.T) {
	for _, caveat := range []ReasonCode{ReasonMultiFactorChange, ReasonAmbiguousTask} {
		resolved, rule := resolveVisibleRatingWithCaveats(AxisInformation, RatingSufficient, RatingSufficient, "", []ReasonCode{caveat})
		if resolved != RatingIndeterminate || rule != "construct_caveat_veto" {
			t.Fatalf("construct caveat %q did not force unresolved information: %s %s", caveat, resolved, rule)
		}
	}
}

func TestFormalHumanComparisonRejectsDenominatorDeletion(t *testing.T) {
	fixture := newRelationTerminalFixture(t)
	result, err := BuildFormalHumanComparison(
		fixture.plan, fixture.bundle, fixture.mappings, fixture.reveal, fixture.ambiguity,
		fixture.left, fixture.leftBatch, fixture.leftProbes, fixture.right, fixture.rightBatch, fixture.rightProbes,
		&fixture.tie, &fixture.tieBatch, "2026-08-09T18:15:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	tampered := result.Comparison
	tampered.TaskGroupDenominators[0].Unresolved--
	tampered.TaskGroupDenominators[0].Denominator--
	if _, err := SealFormalHumanComparison(tampered); err == nil {
		t.Fatal("relation formal-human comparison accepted denominator deletion")
	}
}

func TestTerminalRelationLedgerRejectsUnauthorizedStatusMutation(t *testing.T) {
	fixture := newRelationTerminalFixture(t)
	result, err := BuildFormalHumanComparison(
		fixture.plan, fixture.bundle, fixture.mappings, fixture.reveal, fixture.ambiguity,
		fixture.left, fixture.leftBatch, fixture.leftProbes, fixture.right, fixture.rightBatch, fixture.rightProbes,
		&fixture.tie, &fixture.tieBatch, "2026-08-09T18:15:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := BuildTerminalRelationLedger(fixture.bundle, fixture.reveal, fixture.ambiguity, result.Comparison, result.Resolutions, "2026-08-09T18:16:00Z")
	if err != nil {
		t.Fatal(err)
	}
	ledger.ExternalActionStatus = "authorized"
	if _, err := SealTerminalRelationLedger(ledger); err == nil {
		t.Fatal("relation terminal ledger fabricated external authorization")
	}
}

func newRelationTerminalFixture(t *testing.T) relationTerminalFixture {
	return newRelationTerminalFixtureForProtocol(t, ProtocolVersionV1)
}

func newRelationTerminalFixtureForProtocol(t *testing.T, protocolVersion string) relationTerminalFixture {
	t.Helper()
	plan, material, context := relationCandidateBlindingFixture(t)
	switch protocolVersion {
	case ProtocolVersionV1:
		// The base fixture is already v1.
	case ProtocolVersionV2:
		plan, material, context = relationCandidateBlindingFixtureV2(t)
	case ProtocolVersionV3:
		plan, material, context = relationCandidateBlindingFixtureV3(t)
	default:
		t.Fatalf("unsupported relation protocol version %q", protocolVersion)
	}
	packets := make([]BlindPacket, 3)
	mappings := make([]PrivateMapping, 3)
	for index, keyByte := range []byte{0x31, 0x52, 0x63} {
		packet, mapping, err := buildBlindedPacket(plan, material, context, "relation-terminal-key-"+string(rune('a'+index)), bytes.Repeat([]byte{keyByte}, 32))
		if err != nil {
			t.Fatal(err)
		}
		packets[index], mappings[index] = packet, mapping
	}
	qualification, answerKey, err := DefaultQualification(plan, "qualification-terminal-key", bytes.Repeat([]byte{0x19}, 32))
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(plan, qualification)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := BuildReviewBundle(plan, digestText("terminal-pilot-sample"), ReviewDataDevelopmentPilot, packets, mappings, qualification, handbook, "2026-08-09T18:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	left := mustPrimaryAssignment(t, bundle, mappings, qualification, answerKey, "reviewer-left-terminal", 1, 0x71)
	right := mustPrimaryAssignment(t, bundle, mappings, qualification, answerKey, "reviewer-right-terminal", 2, 0x72)
	packetIDs := []string{bundle.Packets[0].PacketID, bundle.Packets[1].PacketID, bundle.Packets[2].PacketID}
	leftRatings := []Rating{RatingEqual, RatingLeft, RatingEqual}
	rightRatings := []Rating{RatingEqual, RatingLeft, RatingRight}
	leftJudgments := make([]PairJudgment, 3)
	rightJudgments := make([]PairJudgment, 3)
	for index, packetID := range packetIDs {
		leftReason := ReasonTaskQualityDiffers
		if leftRatings[index] == RatingEqual {
			leftReason = ReasonNoJudgmentChange
		}
		rightReason := ReasonTaskQualityDiffers
		if rightRatings[index] == RatingEqual {
			rightReason = ReasonNoJudgmentChange
		}
		leftJudgments[index] = mustPairJudgment(t, bundle, left, relationJudgmentDraft(packetID, leftRatings[index], leftReason, "2026-08-09T18:03:00Z", "synthetic terminal fixture"), nil)
		rightJudgments[index] = mustPairJudgment(t, bundle, right, relationJudgmentDraft(packetID, rightRatings[index], rightReason, "2026-08-09T18:03:10Z", "synthetic terminal fixture"), nil)
	}
	leftBatch, err := BuildJudgmentBatch(bundle, left, leftJudgments, "2026-08-09T18:05:00Z")
	if err != nil {
		t.Fatal(err)
	}
	rightBatch, err := BuildJudgmentBatch(bundle, right, rightJudgments, "2026-08-09T18:06:00Z")
	if err != nil {
		t.Fatal(err)
	}
	ambiguity, err := BuildRelationAmbiguityAnalysis(bundle, left, leftBatch, right, rightBatch, "2026-08-09T18:07:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(ambiguity.TieBreakPacketIDs) != 1 {
		t.Fatalf("terminal fixture expected one disagreement, got %v", ambiguity.TieBreakPacketIDs)
	}
	tieReviewer, err := NewReviewerRecordForProtocol(bundle.ProtocolVersion, "reviewer-tie-terminal", ReviewerRoleTieBreak, "2026-08-09T17:00:00Z", true, true, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	tieQualification, err := GradeQualification(qualification, answerKey, tieReviewer.ReviewerAlias, qualificationResponsesFromKey(answerKey), "2026-08-09T17:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	tieSeed := bytes.Repeat([]byte{0x44}, 32)
	tie, err := BuildTieBreakAssignment(bundle, mappings, tieReviewer, tieQualification, ambiguity, left, leftBatch, right, rightBatch, tieSeed, "2026-08-09T18:08:00Z")
	if err != nil {
		t.Fatal(err)
	}
	tieJudgment := mustPairJudgment(t, bundle, tie, relationJudgmentDraft(tie.PacketIDs[0], RatingIndeterminate, ReasonInsufficientInformation, "2026-08-09T18:09:00Z", "synthetic terminal fixture"), nil)
	tieBatch, err := BuildJudgmentBatch(bundle, tie, []PairJudgment{tieJudgment}, "2026-08-09T18:10:00Z")
	if err != nil {
		t.Fatal(err)
	}
	leftProbes, err := BuildConditionProbeBatch(bundle, left, leftBatch, relationProbeDrafts(left.PacketIDs, "2026-08-09T18:11:00Z"), "2026-08-09T18:12:00Z")
	if err != nil {
		t.Fatal(err)
	}
	rightProbes, err := BuildConditionProbeBatch(bundle, right, rightBatch, relationProbeDrafts(right.PacketIDs, "2026-08-09T18:11:10Z"), "2026-08-09T18:13:00Z")
	if err != nil {
		t.Fatal(err)
	}
	seeds := []AssignmentSeed{{left.Digest, bytes.Repeat([]byte{0x71}, 32)}, {right.Digest, bytes.Repeat([]byte{0x72}, 32)}, {tie.Digest, tieSeed}}
	reveal, err := BuildMappingReveal(bundle, mappings, left, leftBatch, leftProbes, right, rightBatch, rightProbes, ambiguity, &tie, &tieBatch, seeds, "2026-08-09T18:14:00Z", "study-owner")
	if err != nil {
		t.Fatal(err)
	}
	return relationTerminalFixture{plan, bundle, mappings, handbook, ambiguity, left, leftBatch, leftProbes, right, rightBatch, rightProbes, tie, tieBatch, reveal}
}
