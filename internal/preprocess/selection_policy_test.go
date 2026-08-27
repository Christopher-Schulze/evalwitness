package preprocess

import "testing"

func TestEvidenceSelectionPolicyInspectionSeparatesCanonicalAndLegacyScorers(t *testing.T) {
	inventory := InspectEvidenceSelectionPolicies()
	if inventory.CanonicalSelector != CanonicalEvidenceSelector || inventory.CanonicalScorePolicy != EvidenceEventScorePolicyVersion ||
		inventory.CanonicalRenderer != CanonicalEvidenceRenderer ||
		inventory.LegacySelector != LegacyEvidenceSelector || inventory.LegacyScorePolicy != LegacyEvidenceLineScorePolicyVersion {
		t.Fatalf("evidence selection policy inventory = %+v", inventory)
	}
	result, err := CanonicalPipeline("go test ./... passed", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	scores, err := InspectEvidenceEventScores(result.Trajectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 1 || scores[0].Score != 22 || scores[0].EventKind != EventMessage {
		t.Fatalf("canonical event score inspection = %+v", scores)
	}
}

func TestCanonicalEvidenceEventRenderingMakesOmittedFieldsInspectable(t *testing.T) {
	event := Event{Kind: EventEvaluation, Evaluation: &EvaluationPayload{
		Name: "test", Explanation: "omitted explanation", ScoreValue: "0", ScoreLabel: "failed",
	}}
	withExplanation := RenderCanonicalEvidenceEvent(event)
	event.Evaluation.Explanation = ""
	withoutExplanation := RenderCanonicalEvidenceEvent(event)
	if withExplanation != withoutExplanation {
		t.Fatal("evaluation explanation unexpectedly reached canonical verifier evidence")
	}
}

func TestLegacyEvidenceLineScoreInspectionUsesFrozenRules(t *testing.T) {
	lines := []string{
		"ERROR: failed with exit code 1", "go test ./... passed", "diff --git a/a.go b/a.go",
		"+created: output", "FINAL summary", "neutral terminal noise",
	}
	want := []int{12, 10, 9, 12, 5, 0}
	inspections := InspectLegacyEvidenceLineScores(lines)
	if len(inspections) != len(want) {
		t.Fatalf("legacy line score inspections = %d, want %d", len(inspections), len(want))
	}
	for index, inspection := range inspections {
		if inspection.Score != want[index] || inspection.LineDigest == "" {
			t.Fatalf("legacy line score %d = %+v, want %d", index, inspection, want[index])
		}
	}
}
