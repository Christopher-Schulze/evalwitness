package stress

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestDevelopmentCaseStudyReproducesOneMinimalPublicWitness(t *testing.T) {
	root := filepath.Join("..", "..")
	first, err := BuildDevelopmentCaseStudy(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildDevelopmentCaseStudy(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("stress development case study is not deterministic")
	}
	if first.Observation.Outcome != OutcomeViolated || first.Observation.ControlArmID != developmentControlArmID ||
		first.OriginalLineUnits != 32 || first.FinalLineUnits != 2 || first.AcceptedReductions != 30 || first.ReductionPercent != 93.75 ||
		first.EmpiricalUnits != 0 || first.ProviderCalls != 0 || first.NetworkRequired ||
		len(first.FinalWitness) != 2 || first.FinalWitness[0].Content != "5" || first.FinalWitness[1].Content != "-1" {
		t.Fatalf("unexpected stress development case-study result: %+v", first)
	}
	if err := first.ValidateAgainstRepository(root); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(first)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeDevelopmentCaseStudy(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, decoded) {
		t.Fatal("stress development case study changed across strict codec round trip")
	}
	markdown, err := RenderDevelopmentCaseStudyMarkdown(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{"32 fixture lines to 2, 93.75% removed", "`trajectory-a-line-013`", "`trajectory-b-line-013`", "not a verifier reliability result", "| Empirical units | 0 |"} {
		if !strings.Contains(markdown, wanted) {
			t.Fatalf("stress development case-study Markdown lacks %q", wanted)
		}
	}
}

func TestDevelopmentCaseStudyIgnoresProviderAndNetworkConfiguration(t *testing.T) {
	root := filepath.Join("..", "..")
	want, err := BuildDevelopmentCaseStudy(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_API_KEY", "must-not-be-read")
	t.Setenv("DEEPSEEK_API_KEY", "must-not-be-read")
	t.Setenv("EVALWITNESS_BASE_URL", "http://127.0.0.1:1")
	got, err := BuildDevelopmentCaseStudy(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("provider or network configuration changed the provider-free case study")
	}
}

func TestDevelopmentCaseStudySVGIsDeterministicAndClaimBounded(t *testing.T) {
	value, err := BuildDevelopmentCaseStudy(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	first, err := RenderDevelopmentCaseStudySVG(value)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderDevelopmentCaseStudySVG(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("stress development case-study SVG is not deterministic")
	}
	var document struct{}
	if err := xml.Unmarshal(first, &document); err != nil {
		t.Fatalf("stress development case-study SVG is not well formed: %v", err)
	}
	for _, wanted := range []string{"MECHANISM ONLY", "53 attempts", "30 accepted removals", "23 rejected removals", "final rejection pass 2/2", "one_minimal", "trajectory-a-line-013", "trajectory-b-line-013", value.ClaimBoundary, value.Digest[:12]} {
		if !bytes.Contains(first, []byte(wanted)) {
			t.Fatalf("stress development case-study SVG lacks %q", wanted)
		}
	}
	if got := bytes.Count(first, []byte(`<rect class="blue"`)); got != value.AcceptedReductions {
		t.Fatalf("stress development case-study SVG accepted marks = %d, want %d", got, value.AcceptedReductions)
	}
	if got := bytes.Count(first, []byte(`<rect class="red"`)); got != len(value.Counterexample.Steps)-value.AcceptedReductions {
		t.Fatalf("stress development case-study SVG rejected marks = %d, want %d", got, len(value.Counterexample.Steps)-value.AcceptedReductions)
	}
	for _, forbidden := range []string{"<script", "<image", "<metadata", "href="} {
		if bytes.Contains(first, []byte(forbidden)) {
			t.Fatalf("stress development case-study SVG contains forbidden payload %q", forbidden)
		}
	}
}

func TestDevelopmentCaseStudyOracleStopsWhenDecisiveEvidenceIsRemoved(t *testing.T) {
	root := filepath.Join("..", "..")
	fixtures, err := readDevelopmentFixtures(root)
	if err != nil {
		t.Fatal(err)
	}
	input, err := newDevelopmentCaseInput(fixtures.trajectoryA.raw, fixtures.trajectoryB.raw)
	if err != nil {
		t.Fatal(err)
	}
	relation, err := sealV3CatalogRelation(mutation.FamilyCandidateOrderReversal, EstimandSensitivity, []preprocess.SourceFormat{preprocess.SourcePlainText})
	if err != nil {
		t.Fatal(err)
	}
	privacyDigest, err := digestDocument("privacy")
	if err != nil {
		t.Fatal(err)
	}
	oracle := developmentCaseOracle{relation: relation, privacyPolicyDigest: privacyDigest, allowedLines: input.lineIndex(), requiredUnits: input.decisiveUnits()}
	removed, err := input.Remove(input.decisiveUnits()[0])
	if err != nil {
		t.Fatal(err)
	}
	observation, err := oracle.Evaluate(context.Background(), removed)
	if err != nil {
		t.Fatal(err)
	}
	if observation.RelationRevalidated || observation.ViolationPreserved {
		t.Fatal("development oracle retained a violation after decisive evidence was removed")
	}
}

func TestDevelopmentCaseStudyRejectsTamperingAndRepositoryDrift(t *testing.T) {
	root := filepath.Join("..", "..")
	value, err := BuildDevelopmentCaseStudy(root)
	if err != nil {
		t.Fatal(err)
	}
	tampered := value
	tampered.EmpiricalUnits = 1
	tampered.Digest, err = developmentCaseStudyDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.Validate(); err == nil {
		t.Fatal("stress development case study accepted a fabricated empirical unit")
	}

	raw, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["unknown"] = true
	unknown, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDevelopmentCaseStudy(bytes.NewReader(unknown)); err == nil {
		t.Fatal("stress development case study accepted an unknown property")
	}

	temporary := t.TempDir()
	copyDevelopmentFixtureTree(t, root, temporary)
	if err := value.ValidateAgainstRepository(temporary); err != nil {
		t.Fatal(err)
	}
	trajectoryPath := filepath.Join(temporary, filepath.FromSlash(developmentTrajectoryAPath))
	trajectory, err := os.ReadFile(trajectoryPath)
	if err != nil {
		t.Fatal(err)
	}
	trajectory = bytes.Replace(trajectory, []byte("\n5\n"), []byte("\n6\n"), 1)
	if err := os.WriteFile(trajectoryPath, trajectory, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := value.ValidateAgainstRepository(temporary); err == nil {
		t.Fatal("stress development case study accepted a changed repository fixture")
	}
}

func TestDevelopmentCaseStudySchemaIsClosed(t *testing.T) {
	schema, err := Schema("development-case-study")
	if err != nil {
		t.Fatal(err)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("stress development case-study schema permits unknown root properties")
	}
	properties := schema["properties"].(map[string]any)
	version := properties["schema_version"].(JSONSchema)
	if version["const"] != DevelopmentCaseStudySchemaVersion {
		t.Fatalf("development case-study schema version = %v", version["const"])
	}
}

func copyDevelopmentFixtureTree(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	for _, path := range []string{developmentTaskPath, developmentTrajectoryAPath, developmentTrajectoryBPath, developmentLicensePath} {
		raw, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(destinationRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
