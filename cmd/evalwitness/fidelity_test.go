package main

import (
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestParsePositiveIntegers(t *testing.T) {
	values, err := parsePositiveIntegers("65536, 16384,32768")
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 3 || values[0] != 65536 || values[1] != 16384 || values[2] != 32768 {
		t.Fatalf("unexpected budgets: %v", values)
	}
	for _, invalid := range []string{"", "0", "-1", "one"} {
		if _, err := parsePositiveIntegers(invalid); err == nil {
			t.Fatalf("accepted invalid budget %q", invalid)
		}
	}
}

func TestAuditFidelitySourceUsesBoundedCanonicalReader(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "preprocess", "testdata", "golden", "codex-rollout.jsonl")
	report, err := auditFidelitySource(path, []int{16384, 32768, 65536}, preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceFormat != preprocess.SourceCodexRollout || report.Ingestion.SourceRecords != 5 {
		t.Fatalf("unexpected fidelity report: %+v", report)
	}
	if report.TokenComparison == nil || report.TokenComparison.ObservationCount != 1 {
		t.Fatalf("missing token comparison: %+v", report.TokenComparison)
	}
}
