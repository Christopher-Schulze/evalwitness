package lineage

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestGoldenVectorsSealCompleteThreeFormatTaxonomy(t *testing.T) {
	set, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		t.Fatal(err)
	}
	want := GoldenVectorFixtureSummary{
		Vectors: 63, Formats: 3, SemanticCases: 21,
		PositiveControls: 21, NegativeControls: 27, CapabilityBoundaries: 15,
		Accepted: 49, Rejected: 11, NotApplicable: 3, KnownMappingGaps: 0,
	}
	if set.Summary != want {
		t.Fatalf("golden-vector summary = %#v, want %#v", set.Summary, want)
	}
	seen := make(map[string]int)
	for _, vector := range set.Vectors {
		seen[vector.CaseID]++
		if vector.DataRole != RoleAdapterDevelopment {
			t.Fatalf("vector %q entered role %q", vector.VectorID, vector.DataRole)
		}
		if vector.Expectation.RepresentableByFormat == CapabilityUnsupported && vector.Observation.ImportOutcome != GoldenImportNotApplicable {
			t.Fatalf("unsupported vector %q was observed as %q", vector.VectorID, vector.Observation.ImportOutcome)
		}
	}
	for _, definition := range goldenCaseDefinitions() {
		if seen[definition.id] != 3 {
			t.Fatalf("case %q has %d format vectors", definition.id, seen[definition.id])
		}
	}
}

func TestGoldenVectorsProveIdentityAndExitMappingRepairs(t *testing.T) {
	set, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		t.Fatal(err)
	}
	repairedRejections := 0
	exitPreserved := false
	for _, vector := range set.Vectors {
		if vector.KnownMappingGap != "" {
			t.Fatalf("vector %q retains mapping gap %q", vector.VectorID, vector.KnownMappingGap)
		}
		if vector.CaseID == "missing_call_id" || vector.CaseID == "duplicate_call_id" || vector.CaseID == "out_of_order_result" {
			if vector.Observation.ImportOutcome == GoldenImportRejected {
				repairedRejections++
			}
		}
		if vector.VectorID == "codex_rollout_jsonl/native_exit_status" {
			exitPreserved = vector.Observation.ExitStatusObservations == 1
		}
	}
	if repairedRejections != 8 || !exitPreserved {
		t.Fatalf("mapping repairs drifted: identity rejections=%d exit_preserved=%t", repairedRejections, exitPreserved)
	}
}

func TestGoldenVectorsCoverTask069RequiredCases(t *testing.T) {
	required := []string{
		"direct_test_invocation", "direct_check_invocation", "direct_build_invocation", "direct_outcome_probe",
		"printed_verification_name", "searched_verification_name", "wrapper_chain", "compound_command",
		"missing_command_display", "missing_call_id", "duplicate_call_id", "out_of_order_result",
		"bounded_truncation", "secret_redaction", "unsupported_native_syntax", "agent_prose_attached_to_result",
		"same_path_comparison_false_target", "structured_success_after_text_omission", "distinct_operand_comparison",
		"planning_prose_false_target", "native_exit_status",
	}
	definitions := goldenCaseDefinitions()
	got := make([]string, len(definitions))
	for index, definition := range definitions {
		got[index] = definition.id
	}
	if !reflect.DeepEqual(got, required) {
		t.Fatalf("golden-vector taxonomy drifted\ngot:  %v\nwant: %v", got, required)
	}
}

func TestGoldenVectorsAreDeterministicAndPublishNoRawSecret(t *testing.T) {
	left, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		t.Fatal(err)
	}
	right, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		t.Fatal(err)
	}
	leftJSON, err := EncodeIndented(left)
	if err != nil {
		t.Fatal(err)
	}
	rightJSON, err := EncodeIndented(right)
	if err != nil {
		t.Fatal(err)
	}
	if string(leftJSON) != string(rightJSON) {
		t.Fatal("repeated golden-vector generation changed bytes")
	}
	for _, forbidden := range []string{"fixture-token-value", "Bearer ", "/Users/", "private chain", "chain of thought"} {
		if strings.Contains(string(leftJSON), forbidden) {
			t.Fatalf("public golden-vector artifact contains forbidden material %q", forbidden)
		}
	}
	var decoded GoldenVectorFixtureSet
	if err := json.Unmarshal(leftJSON, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestGoldenVectorsRetainRepresentabilityAndObservationAsSeparateAxes(t *testing.T) {
	set, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		t.Fatal(err)
	}
	for _, vector := range set.Vectors {
		if vector.Format == preprocess.SourceClaudeCode && vector.Expectation.RepresentableByFormat != CapabilityUnspecified {
			t.Fatalf("Claude vector %q fabricated format-wide capability %q", vector.VectorID, vector.Expectation.RepresentableByFormat)
		}
		if vector.Observation.ImportOutcome == GoldenImportAccepted && vector.Observation.TrajectoryDigest == "" {
			t.Fatalf("accepted vector %q lacks observed canonical identity", vector.VectorID)
		}
	}
}
