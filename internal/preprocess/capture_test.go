package preprocess

import (
	"strings"
	"testing"
)

const claudeCodeFixture = `{"type":"ai-title","title":"noise"}
{"type":"user","message":{"role":"user","content":"fix the bug in foo.go"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Looking at foo.go first."},{"type":"tool_use","id":"toolu_01","name":"Bash","input":{"command":"go test ./...","description":"Run tests"}}]}}
{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_01","type":"tool_result","content":"--- FAIL: TestFoo (0.00s)\nexit status 1"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_02","name":"Read","input":{"file_path":"/repo/foo.go"}}]}}
{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_02","type":"tool_result","content":[{"type":"text","text":"package foo"}]}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Fixed the off-by-one."}]}}`

func TestParseClaudeCodeSession(t *testing.T) {
	steps, ok := ParseClaudeCodeSession(claudeCodeFixture)
	if !ok {
		t.Fatal("claude code session not detected")
	}
	if len(steps) != 3 {
		t.Fatalf("steps = %d, want 3: %+v", len(steps), steps)
	}
	if !strings.Contains(steps[0].Command, "go test ./...") {
		t.Errorf("step 1 command = %q", steps[0].Command)
	}
	if !strings.Contains(steps[0].Output, "FAIL: TestFoo") {
		t.Errorf("step 1 output = %q", steps[0].Output)
	}
	if steps[0].Message != "Looking at foo.go first." {
		t.Errorf("step 1 message = %q", steps[0].Message)
	}
	if steps[1].Output != "package foo" {
		t.Errorf("step 2 list-content tool_result = %q", steps[1].Output)
	}
	if steps[2].Message != "Fixed the off-by-one." {
		t.Errorf("step 3 message = %q", steps[2].Message)
	}
}

const codexFixture = `{"timestamp":"2026-07-03T11:49:58","type":"session_meta","payload":{"id":"019f2762"}}
{"timestamp":"t","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the bug"}]}}
{"timestamp":"t","type":"response_item","payload":{"type":"reasoning","id":"rs_1","summary":[],"encrypted_content":"xxx"}}
{"timestamp":"t","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Running the tests first."}]}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call","id":"fc_1","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\",\"workdir\":\"/repo\"}","call_id":"call_1"}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call_output","call_id":"call_1","output":"ok  \tfoo\t0.1s"}}
{"timestamp":"t","type":"response_item","payload":{"type":"custom_tool_call","id":"ctc_1","status":"completed","call_id":"call_2","name":"apply_patch","input":"*** Begin Patch\n*** Update File: foo.go"}}
{"timestamp":"t","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"call_2","output":"Success."}}
{"timestamp":"t","type":"event_msg","payload":{"type":"agent_message","message":"duplicate of response_item, must be ignored"}}`

func TestParseCodexRollout(t *testing.T) {
	steps, ok := ParseCodexRollout(codexFixture)
	if !ok {
		t.Fatal("codex rollout not detected")
	}
	if len(steps) != 3 {
		t.Fatalf("steps = %d, want 3: %+v", len(steps), steps)
	}
	if steps[0].Message != "Running the tests first." {
		t.Errorf("step 1 message = %q", steps[0].Message)
	}
	if steps[1].Command != "exec_command: go test ./..." {
		t.Errorf("step 2 command = %q", steps[1].Command)
	}
	if !strings.Contains(steps[1].Output, "ok") {
		t.Errorf("step 2 output = %q", steps[1].Output)
	}
	if !strings.Contains(steps[2].Command, "apply_patch") || steps[2].Output != "Success." {
		t.Errorf("step 3 = %+v", steps[2])
	}
}

const openCodeFixture = `{
  "info": {"id": "ses_abc", "title": "fix bug"},
  "messages": [
    {"info": {"role": "user"}, "parts": [{"type": "text", "text": "fix the bug"}]},
    {"info": {"role": "assistant"}, "parts": [
      {"type": "step-start"},
      {"type": "reasoning", "text": "thinking..."},
      {"type": "text", "text": "Checking the failing test."},
      {"type": "tool", "tool": "bash", "callID": "call-1", "state": {"status": "completed", "input": {"command": "go test ./..."}, "output": "FAIL foo"}},
      {"type": "step-finish"}
    ]},
    {"info": {"role": "assistant"}, "parts": [{"type": "text", "text": "Done."}]}
  ]
}`

func TestParseOpenCodeExport(t *testing.T) {
	steps, ok := ParseOpenCodeExport(openCodeFixture)
	if !ok {
		t.Fatal("opencode export not detected")
	}
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2: %+v", len(steps), steps)
	}
	if steps[0].Message != "Checking the failing test." || !strings.Contains(steps[0].Command, "go test") || steps[0].Output != "FAIL foo" {
		t.Errorf("step 1 = %+v", steps[0])
	}
	if steps[1].Message != "Done." {
		t.Errorf("step 2 = %+v", steps[1])
	}
}

func TestPipelineDetectsAgentSessions(t *testing.T) {
	for name, fixture := range map[string]string{
		"claude-code": claudeCodeFixture,
		"codex":       codexFixture,
		"opencode":    openCodeFixture,
	} {
		r := Pipeline(fixture, true, 0)
		if r.OriginalSteps == 0 {
			t.Errorf("%s: pipeline did not detect session transcript", name)
		}
		if !strings.Contains(r.Text, "--- Agent Step 1 ---") {
			t.Errorf("%s: canonical format missing: %s", name, r.Text[:min(120, len(r.Text))])
		}
	}
	// Terminal-bench JSON must still take precedence.
	tb := `{"trajectory":{"steps":[{"step_id":1,"source":"agent","message":"hi"}]}}`
	if r := Pipeline(tb, true, 0); r.OriginalSteps != 1 {
		t.Errorf("terminal-bench detection broken")
	}
	// Plain text stays raw.
	if r := Pipeline("just some text", true, 0); r.OriginalSteps != 0 {
		t.Errorf("plain text misdetected as session")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
