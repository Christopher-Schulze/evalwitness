package preprocess

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCodexExecEndPreservesObservedExitAndSeparatedStreams(t *testing.T) {
	raw := `{"timestamp":"t","type":"event_msg","payload":{"type":"exec_command_begin","call_id":"call-1","command":["go","test","./..."],"cwd":"/repo"}}
{"timestamp":"t","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-1","status":"failed","exit_code":7,"stdout":"partial stdout\n","stderr":"decisive stderr\n"}}`
	trajectory, err := IngestString(raw, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	var command *CommandPayload
	var result *ToolResultPayload
	for index := range trajectory.Events {
		if trajectory.Events[index].Command != nil {
			command = trajectory.Events[index].Command
		}
		if trajectory.Events[index].ToolResult != nil {
			result = trajectory.Events[index].ToolResult
		}
	}
	if command == nil || len(command.Argv) != 3 || command.Argv[0] != "go" || command.UnsupportedShellText != "" || !canonicalSHA256(command.OperandsDigest) {
		t.Fatalf("current canonicalization did not preserve structured command operands: %#v", command)
	}
	if result == nil || result.ExitCode == nil || *result.ExitCode != 7 || !result.Error {
		t.Fatalf("Codex exit observation was not preserved: %#v", result)
	}
	if len(result.Stdout) != 1 || result.Stdout[0].Text != "partial stdout\n" || len(result.Stderr) != 1 || result.Stderr[0].Text != "decisive stderr\n" || len(result.Output) != 0 {
		t.Fatalf("Codex streams were flattened or fabricated: %#v", result)
	}
	rendered := RenderTrajectory(trajectory)
	for _, expected := range []string{"exit=7", "[stdout]", "partial stdout", "[stderr]", "decisive stderr"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("canonical rendering lost %q: %s", expected, rendered)
		}
	}
}

func TestFrozenCanonicalizationV1ReproducesLegacyCodexShape(t *testing.T) {
	raw := `{"timestamp":"t","type":"event_msg","payload":{"type":"exec_command_begin","call_id":"call-1","command":["go","test","./..."],"cwd":"/repo"}}
{"timestamp":"t","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-1","status":"failed","exit_code":7,"stdout":"partial stdout\n","stderr":"decisive stderr\n"}}`
	trajectory, err := IngestString(raw, FrozenCanonicalizationV1IngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	if trajectory.Digest != "288d6133c395545fd95e3d4367c0642de7ef5aaeb477597a59749cbf8f97982b" {
		t.Fatalf("frozen V1 digest changed: %s", trajectory.Digest)
	}
	var command *CommandPayload
	var result *ToolResultPayload
	for index := range trajectory.Events {
		if trajectory.Events[index].Command != nil {
			command = trajectory.Events[index].Command
		}
		if trajectory.Events[index].ToolResult != nil {
			result = trajectory.Events[index].ToolResult
		}
	}
	if command == nil || command.Display != "go test ./..." || len(command.Argv) != 0 || command.UnsupportedShellText != "" || command.OperandsDigest != "" {
		t.Fatalf("frozen V1 command shape changed: %#v", command)
	}
	if result == nil || result.ExitCode != nil || len(result.Stdout) != 0 || len(result.Stderr) != 0 || len(result.Output) != 1 || result.Output[0].Text != "partial stdout\ndecisive stderr\n" || !result.Error {
		t.Fatalf("frozen V1 result shape changed: %#v", result)
	}
	encoded, err := json.Marshal(trajectory)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"argv"`, `"unsupported_shell_text"`, `"operands_digest"`, `"exit_code"`, `"stdout"`, `"stderr"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("frozen V1 encoding gained %s: %s", forbidden, encoded)
		}
	}
}

func TestCodexExecEndDoesNotFabricateAbsentExitStatus(t *testing.T) {
	raw := `{"timestamp":"t","type":"event_msg","payload":{"type":"exec_command_begin","call_id":"call-1","command":["go","test","./..."]}}
{"timestamp":"t","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-1","status":"completed","stdout":"ok\n"}}`
	trajectory, err := IngestString(raw, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range trajectory.Events {
		if event.ToolResult != nil && event.ToolResult.ExitCode != nil {
			t.Fatalf("absent native exit status became observed zero: %#v", event.ToolResult)
		}
	}
}

func TestStrictNativeAdaptersRejectAmbiguousCallIdentity(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "claude missing", raw: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`},
		{name: "claude duplicate", raw: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"call-1","name":"Bash","input":{"command":"go test ./..."}},{"type":"tool_use","id":"call-1","name":"Bash","input":{"command":"go vet ./..."}}]}}`},
		{name: "claude out of order", raw: `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-1","content":"ok"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"call-1","name":"Bash","input":{"command":"go test ./..."}}]}}`},
		{name: "codex missing", raw: `{"timestamp":"t","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"}}`},
		{name: "codex duplicate", raw: `{"timestamp":"t","type":"response_item","payload":{"type":"function_call","id":"fc-1","call_id":"call-1","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call","id":"fc-2","call_id":"call-1","name":"exec_command","arguments":"{\"cmd\":\"go vet ./...\"}"}}`},
		{name: "codex out of order", raw: `{"timestamp":"t","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"ok"}}
{"timestamp":"t","type":"response_item","payload":{"type":"function_call","id":"fc-1","call_id":"call-1","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"}}`},
		{name: "opencode missing", raw: `{"info":{"id":"session-1"},"messages":[{"info":{"id":"message-1","role":"assistant"},"parts":[{"id":"part-1","type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"go test ./..."},"output":"ok"}}]}]}`},
		{name: "opencode duplicate", raw: `{"info":{"id":"session-1"},"messages":[{"info":{"id":"message-1","role":"assistant"},"parts":[{"id":"part-1","type":"tool","tool":"bash","callID":"call-1","state":{"status":"completed","input":{"command":"go test ./..."},"output":"ok"}},{"id":"part-2","type":"tool","tool":"bash","callID":"call-1","state":{"status":"completed","input":{"command":"go vet ./..."},"output":"ok"}}]}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := IngestString(test.raw, DefaultIngestOptions()); err == nil {
				t.Fatal("strict ingestion accepted ambiguous native call identity")
			}
		})
	}
}

func TestFrozenCanonicalizationV1AcceptsHistoricalAmbiguousCallIdentity(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "claude", raw: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`},
		{name: "codex", raw: `{"timestamp":"t","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"}}`},
		{name: "opencode", raw: `{"info":{"id":"session-1"},"messages":[{"info":{"id":"message-1","role":"assistant"},"parts":[{"id":"part-1","type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"go test ./..."},"output":"ok"}}]}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := IngestString(test.raw, FrozenCanonicalizationV1IngestOptions()); err != nil {
				t.Fatalf("frozen V1 rejected historically accepted identity shape: %v", err)
			}
		})
	}
}

func TestIngestionRejectsUnknownCanonicalizationProfile(t *testing.T) {
	options := DefaultIngestOptions()
	options.CanonicalizationProfile = CanonicalizationProfile("evalwitness.canonicalization.unknown")
	if _, err := IngestString("content", options); err == nil {
		t.Fatal("ingestion accepted an unknown canonicalization profile")
	}
}
