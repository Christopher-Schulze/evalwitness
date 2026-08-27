package preprocess

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The literal below is synthetic and must stay shaped like a key: the test
// exists to prove that anything of this shape never reaches a provider or an
// artifact. It is the only key-shaped string in the repository.
func TestRedact(t *testing.T) {
	in := "API key: sk-proj-EXAMPLEKEYNOTREAL0000000000"
	out, hits := Redact(in, true)
	if hits == 0 {
		t.Fatalf("expected hit")
	}
	if strings.Contains(out, "sk-proj-EXAMPLEKEY") {
		t.Errorf("secret leaked: %s", out)
	}
}

func TestRedactDisabled(t *testing.T) {
	in := "API key: sk-proj-EXAMPLEKEYNOTREAL0000000000"
	out, hits := Redact(in, false)
	if hits != 0 {
		t.Errorf("hits=%d, want 0", hits)
	}
	if out != in {
		t.Errorf("disabled redact modified input")
	}
}

func TestParseJSONTrajectory(t *testing.T) {
	raw := `{
		"trajectory": {
			"steps": [
				{"step_id": 1, "source": "agent", "message": "thinking", "tool_calls": [{"arguments": {"keystrokes": "ls -la"}}], "observation": {"results": [{"content": "drwxr-xr-x ."}]}},
				{"step_id": 2, "source": "agent", "message": "done"}
			]
		}
	}`
	steps, ok := ParseJSONTrajectory(raw)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Command != "ls -la" || steps[0].Output != "drwxr-xr-x ." {
		t.Errorf("step 0 mismatch: %+v", steps[0])
	}
}

func TestParseJSONTrajectory_NotJSON(t *testing.T) {
	if _, ok := ParseJSONTrajectory("--- Agent Step 1 ---\nplain text"); ok {
		t.Fatalf("expected non-JSON to fail parse")
	}
}

func TestImportanceTruncate(t *testing.T) {
	steps := []Step{
		{StepID: "1", Source: "agent", Message: "planning"},
		{StepID: "2", Source: "agent", Command: "rm -rf /tmp/x", Output: "ok"},
		{StepID: "3", Source: "agent", Message: "more thinking"},
		{StepID: "4", Source: "agent", Output: "Error: permission denied", Command: "chmod 700 file"},
		{StepID: "5", Source: "agent", Message: "wrap up"},
	}
	text, total, kept, _ := ImportanceTruncate(steps, 200)
	if total != 5 {
		t.Errorf("total=%d, want 5", total)
	}
	if kept == 0 || kept > 5 {
		t.Errorf("kept=%d, expected (0, 5]", kept)
	}
	if !strings.Contains(text, "Error: permission denied") {
		t.Errorf("expected error step kept, got: %s", text)
	}
}

func TestEvidenceSliceRetainsHighValueSignalsWithinBudget(t *testing.T) {
	noise := make([]string, 0, 4000)
	for index := 0; index < 4000; index++ {
		noise = append(noise, "repeated low-value package download progress")
	}
	noise[1700] = "diff --git a/internal/a.go b/internal/a.go"
	noise[1701] = "@@ -10,2 +10,2 @@"
	noise[1702] = "+fixed actual winner propagation"
	noise[2300] = "ERROR: regression test failed with exit code 1"
	noise[3998] = "go test ./... passed"
	noise[3999] = "FINAL: working tree verified clean"
	steps := []Step{{StepID: "1", Command: "run evaluation", Output: strings.Join(noise, "\n")}}

	text, _, kept, evidenceSliced := ImportanceTruncate(steps, 300)
	if kept != 1 {
		t.Fatalf("kept steps = %d, want 1", kept)
	}
	if !evidenceSliced {
		t.Fatal("oversized step must report evidence slicing")
	}
	for _, signal := range []string{"diff --git", "fixed actual winner propagation", "ERROR:", "exit code 1", "go test ./... passed", "FINAL:"} {
		if !strings.Contains(text, signal) {
			t.Fatalf("sliced evidence lost %q", signal)
		}
	}
	if tokens := EstimateTokens(text); tokens > 320 {
		t.Fatalf("sliced evidence uses %d tokens for a 300-token budget", tokens)
	}
}

func TestPipelineMarksWithinStepEvidenceSlicingAsTruncated(t *testing.T) {
	output := strings.Repeat("low-value output line\n", 5000) + "FINAL: tests passed"
	raw := `{"trajectory":{"steps":[{"step_id":1,"source":"agent","message":"done","observation":{"results":[{"content":` +
		strconv.Quote(output) + `}]}}]}}`
	result := Pipeline(raw, true, 300)
	if !result.Truncated {
		t.Fatal("pipeline must mark within-step evidence slicing as truncation")
	}
	if result.KeptSteps != result.OriginalSteps {
		t.Fatalf("whole-step count changed: kept=%d original=%d", result.KeptSteps, result.OriginalSteps)
	}
}

func TestEvidenceSliceBoundsOversizedCommand(t *testing.T) {
	command := "python -c " + strings.Repeat("verbose_generated_argument ", 5000) + " --final-check"
	steps, sliced := EvidenceSlice([]Step{{StepID: "1", Command: command, Output: "tests passed"}}, 300)
	if !sliced {
		t.Fatal("oversized command must report evidence slicing")
	}
	if len(steps) != 1 || len(steps[0].Command) >= len(command) {
		t.Fatalf("oversized command was not bounded: before=%d after=%d", len(command), len(steps[0].Command))
	}
	if !strings.Contains(steps[0].Command, "--final-check") {
		t.Fatal("command slicing lost the final command state")
	}
}

func TestPipelineRawText(t *testing.T) {
	r := Pipeline("--- Agent Step 1 ---\nhello", true, 0)
	if r.Text == "" {
		t.Fatalf("empty text")
	}
	if r.Hash == "" {
		t.Errorf("expected hash")
	}
	if r.OriginalSteps != 0 {
		t.Errorf("OriginalSteps should be 0 for raw text, got %d", r.OriginalSteps)
	}
}

func TestPipelineJSON(t *testing.T) {
	raw := `{"trajectory":{"steps":[{"step_id":1,"source":"agent","message":"hi"}]}}`
	r := Pipeline(raw, true, 0)
	if r.OriginalSteps != 1 {
		t.Errorf("expected 1 step, got %d", r.OriginalSteps)
	}
	if !strings.Contains(r.Text, "Agent Step 1") {
		t.Errorf("unexpected formatted text: %s", r.Text)
	}
}

func TestRedactPasswordAssignment(t *testing.T) {
	in := `config: password = "hunter2-secret" and "password": "abc123"`
	out, hits := Redact(in, true)
	if strings.Contains(out, "hunter2-secret") || strings.Contains(out, "abc123") {
		t.Errorf("password values not redacted: %s", out)
	}
	if hits < 2 {
		t.Errorf("expected 2+ hits, got %d", hits)
	}
}

func TestLoadCustomPatterns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patterns.json")
	if err := os.WriteFile(path, []byte(`[{"pattern":"CUSTOM-[0-9]{4}","replacement":"[REDACTED_CUSTOM]"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadCustomPatterns(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		customMu.Lock()
		customPatterns = nil
		customMu.Unlock()
	})
	out, hits := Redact("token CUSTOM-1234 leaked", true)
	if !strings.Contains(out, "[REDACTED_CUSTOM]") || hits != 1 {
		t.Errorf("custom pattern not applied: %q hits=%d", out, hits)
	}
	if err := LoadCustomPatterns(filepath.Join(dir, "missing.json")); err == nil {
		t.Errorf("expected error for missing file")
	}
	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte(`[{"pattern":"([","replacement":"x"}]`), 0o600)
	if err := LoadCustomPatterns(bad); err == nil {
		t.Errorf("expected error for invalid regex")
	}
}
