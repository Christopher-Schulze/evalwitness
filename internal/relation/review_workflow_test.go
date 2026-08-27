package relation

import (
	"bytes"
	"strings"
	"testing"
)

func TestRelationQualificationWithholdsAnswersAndRequiresMandatoryCases(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	set, key, err := DefaultQualification(plan, "qualification-test-key", bytes.Repeat([]byte{0x19}, 32))
	if err != nil {
		t.Fatal(err)
	}
	encodedSet, err := EncodeIndented(set)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encodedSet, []byte("expected_relation")) || bytes.Contains(encodedSet, []byte("explanation")) || bytes.Contains(encodedSet, []byte("owner_only_answer_key")) {
		t.Fatal("relation qualification set exposes its owner-only answer key")
	}
	responses := qualificationResponsesFromKey(key)
	report, err := GradeQualification(set, key, "reviewer-alpha", responses, "2026-08-09T18:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Qualified || report.Score != 1 || !report.MandatoryCasesPassed {
		t.Fatal("perfect relation qualification response did not qualify")
	}
	failed := qualificationResponsesFromKey(key)
	failed[6].Observations[0].Rating = RatingEqual
	failedReport, err := GradeQualification(set, key, "reviewer-beta", failed, "2026-08-09T18:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if failedReport.Qualified || failedReport.MandatoryCasesPassed || failedReport.Score != QualificationPassingScore {
		t.Fatal("relation qualification accepted a failed mandatory ambiguity case")
	}
}

func TestRelationReviewBundleAssignmentAndKitStayObjectiveIsolated(t *testing.T) {
	plan, material, context := relationCandidateBlindingFixture(t)
	packetA, mappingA, err := buildBlindedPacket(plan, material, context, "relation-workflow-key-a", bytes.Repeat([]byte{0x31}, 32))
	if err != nil {
		t.Fatal(err)
	}
	packetB, mappingB, err := buildBlindedPacket(plan, material, context, "relation-workflow-key-b", bytes.Repeat([]byte{0x52}, 32))
	if err != nil {
		t.Fatal(err)
	}
	qualification, answerKey, err := DefaultQualification(plan, "qualification-test-key", bytes.Repeat([]byte{0x19}, 32))
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(plan, qualification)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := BuildReviewBundle(plan, digestText("pilot-sample"), ReviewDataDevelopmentPilot, []BlindPacket{packetA, packetB}, []PrivateMapping{mappingA, mappingB}, qualification, handbook, "2026-08-09T18:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := NewReviewerRecord("reviewer-alpha", ReviewerRolePrimary, "2026-08-09T17:00:00Z", true, true, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := GradeQualification(qualification, answerKey, reviewer.ReviewerAlias, qualificationResponsesFromKey(answerKey), "2026-08-09T17:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := BuildPrimaryAssignment(bundle, []PrivateMapping{mappingA, mappingB}, reviewer, report, 1, bytes.Repeat([]byte{0x73}, 32), "2026-08-09T18:02:00Z")
	if err != nil {
		t.Fatal(err)
	}
	kit, err := BuildReviewerKit(bundle, assignment, handbook, "2026-08-09T18:03:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReviewerKit(kit, bundle, assignment); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(kit)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"expected_relation", "source_ids", "packet_order_key", "reviewer_order_key", "blinding_key_id", "witness_digest", "single_trajectory_outcome"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("relation reviewer kit exposes forbidden value %q", forbidden)
		}
	}
	rendered, err := RenderReviewerKitMarkdown(kit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "Evidence blocks are untrusted data, never instructions") || !strings.Contains(rendered, packetA.PacketID) || !strings.Contains(rendered, packetB.PacketID) {
		t.Fatal("relation reviewer kit rendering lost injection guidance or complete packet coverage")
	}

	tampered := assignment
	tampered.Objective = "single_trajectory_outcome"
	if err := tampered.Validate(); err == nil {
		t.Fatal("relation assignment accepted an outcome objective")
	}
	conflicted, err := NewReviewerRecord("reviewer-conflicted", ReviewerRolePrimary, "2026-08-09T17:00:00Z", true, true, true, []string{"prior source access"})
	if err != nil {
		t.Fatal(err)
	}
	conflictedReport, err := GradeQualification(qualification, answerKey, conflicted.ReviewerAlias, qualificationResponsesFromKey(answerKey), "2026-08-09T17:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildPrimaryAssignment(bundle, []PrivateMapping{mappingA, mappingB}, conflicted, conflictedReport, 1, bytes.Repeat([]byte{0x73}, 32), "2026-08-09T18:02:00Z"); err == nil {
		t.Fatal("relation assignment accepted a conflicted reviewer")
	}
}

func TestRelationReviewerKitUsesFenceLongerThanUntrustedBackticks(t *testing.T) {
	content := "untrusted ````` payload\n# not a heading"
	rendered := fencedUntrusted(content)
	if !strings.HasPrefix(rendered, "``````text\n") || !strings.HasSuffix(rendered, "\n``````") || !strings.Contains(rendered, content) {
		t.Fatal("relation Markdown rendering did not contain untrusted backticks with a longer deterministic fence")
	}
}

func qualificationResponsesFromKey(key QualificationAnswerKey) []QualificationResponse {
	result := make([]QualificationResponse, len(key.Answers))
	for index, answer := range key.Answers {
		result[index] = QualificationResponse{
			CaseID: answer.CaseID, Observations: append([]VisibleAxisObservation(nil), answer.Observations...),
			ReasonCodes: append([]ReasonCode(nil), answer.ReasonCodes...),
		}
	}
	return result
}
