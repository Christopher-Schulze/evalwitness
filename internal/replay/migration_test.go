package replay

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestMigrateLegacyPreservesSourceAndEmitsInspectionOnlyCandidate(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "legacy.jsonl")
	candidatePath := filepath.Join(root, "legacy.inspection.jsonl")
	legacy := []byte(`{"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","provider":"old-route","model":"old-model","response":{"raw_text":"<score>A</score>"}}` + "\n")
	if err := os.WriteFile(sourcePath, legacy, safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}

	report, err := MigrateLegacy(sourcePath, candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unchanged, legacy) {
		t.Fatal("legacy source changed during migration")
	}
	if report.ReplayStatus != provider.ReplayStatusLegacy || report.Records != 1 || len(report.AmbiguousFields) == 0 {
		t.Fatalf("migration report = %+v", report)
	}
	if report.SourceDigest == "" || report.CandidateDigest == "" || report.SourceDigest == report.CandidateDigest {
		t.Fatalf("migration digests = source %q candidate %q", report.SourceDigest, report.CandidateDigest)
	}
	raw, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	var record legacyInspectionRecord
	if err := json.Unmarshal(bytes.TrimSpace(raw), &record); err != nil {
		t.Fatal(err)
	}
	if record.InspectionSchemaVersion != 1 || record.ReplayStatus != provider.ReplayStatusLegacy {
		t.Fatalf("inspection record = %+v", record)
	}
	if _, err := LoadReplay(candidatePath, "old-route", "old-model", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "legacy or unsupported") {
		t.Fatalf("inspection candidate loaded as exact replay: %v", err)
	}
}

func TestMigrateLegacyRejectsAmbiguousOrDestructiveTargets(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "legacy.jsonl")
	if err := os.WriteFile(sourcePath, []byte("{}\n"), safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateLegacy(sourcePath, sourcePath); err == nil {
		t.Fatal("migration accepted its source as candidate")
	}
	candidatePath := filepath.Join(root, "existing.jsonl")
	if err := os.WriteFile(candidatePath, []byte("owned"), safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateLegacy(sourcePath, candidatePath); err == nil {
		t.Fatal("migration accepted an existing candidate")
	}
	got, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "owned" {
		t.Fatalf("existing candidate was overwritten: %q", got)
	}
}

func TestSchemaTwoFixtureIsInspectionOnlyAndNeverExactReplay(t *testing.T) {
	sourcePath := "../../scripts/tests/golden-delta-replay.schema2.jsonl"
	candidatePath := filepath.Join(t.TempDir(), "schema2.inspection.jsonl")
	report, err := MigrateLegacy(sourcePath, candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	if report.Records != 1 || !slices.Contains(report.AmbiguousFields, "valid_score_mass") {
		t.Fatalf("schema-2 inspection report = %+v", report)
	}
	raw, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	var record legacyInspectionRecord
	if err := json.Unmarshal(bytes.TrimSpace(raw), &record); err != nil {
		t.Fatal(err)
	}
	if record.SourceCaptureSchema != 2 || len(record.Request) == 0 || len(record.Response) == 0 {
		t.Fatalf("schema-2 inspection record = %+v", record)
	}
	if _, err := LoadReplay(sourcePath, "replay", "golden-delta", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "legacy or unsupported schema 2") {
		t.Fatalf("schema-2 fixture loaded as exact evidence: %v", err)
	}
	if _, err := LoadReplay(candidatePath, "replay", "golden-delta", provider.Capabilities{}); err == nil {
		t.Fatal("inspection candidate loaded as exact replay")
	}
}
