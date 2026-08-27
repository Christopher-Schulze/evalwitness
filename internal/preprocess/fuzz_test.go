package preprocess

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzIngestNeverPanics(f *testing.F) {
	for _, seed := range []string{"plain", claudeCodeFixture, codexFixture, openCodeFixture, "{", "\xff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		options := DefaultIngestOptions()
		options.MaxSourceBytes = 1 << 20
		options.MaxRecordBytes = 256 << 10
		trajectory, err := IngestString(raw, options)
		if err != nil {
			return
		}
		if err := trajectory.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(RenderTrajectory(trajectory)) > 8<<20 {
			t.Fatal("canonical rendering exceeded bounded expansion")
		}
	})
}

func FuzzEvidenceBudgetNeverPanics(f *testing.F) {
	f.Add("small", uint16(16))
	f.Add(strings.Repeat("🧪", 4096), uint16(128))
	f.Add("00\x9f", uint16(0))
	f.Fuzz(func(t *testing.T, raw string, rawBudget uint16) {
		if len(raw) == 0 || len(raw) > 1<<20 {
			t.Skip()
		}
		trajectory, err := IngestString(raw, DefaultIngestOptions())
		if err != nil {
			return
		}
		budget := int(rawBudget%4096) + 1
		retained, err := ApplyEvidenceBudget(trajectory, budget)
		if err != nil {
			return
		}
		text := RenderTrajectory(retained)
		if estimateTokensForBytes(len(text)) > budget {
			t.Fatalf("retained evidence exceeds %d-token budget", budget)
		}
		if !utf8.ValidString(text) {
			t.Fatal("retained evidence is not valid UTF-8")
		}
	})
}
