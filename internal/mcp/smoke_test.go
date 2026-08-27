package mcp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

// TestMCPEndToEndSmokeProducesSelection drives the complete shipped MCP path:
// JSON-RPC protocol layer → Server → ToolHandler → verification.Service →
// Selection. No other test covers this full chain. server_test.go exercises
// the protocol with a stub dispatcher; tools_test.go exercises the handler
// without the server. If any layer rots silently — the server stops routing
// tools/call, the handler stops wiring the service, or the service stops
// producing a marshalable Selection — this test breaks.
func TestMCPEndToEndSmokeProducesSelection(t *testing.T) {
	handler := testHandler(t)
	rw := newPipeRW()

	server := NewServer("evalwitness-smoke", "test", handler, rw, rw)
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background()) }()

	frames := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"evalwitness_pairwise","arguments":{"task":"reverse a string","trajectories":["attempt one","attempt two"]}}}`,
	}
	for _, frame := range frames {
		if _, err := rw.pw.Write([]byte(frame + "\n")); err != nil {
			t.Fatal(err)
		}
	}

	// Poll for the tool-call response before closing the pipe. The tool call
	// runs in a goroutine whose context is derived from the server's serve
	// context; closing the write end before the call finishes would cancel it.
	deadline := time.After(10 * time.Second)
	closePipe := func() {
		if err := rw.pw.Close(); err != nil {
			t.Errorf("close MCP write pipe: %v", err)
		}
	}
	for strings.Count(rw.Output(), "\n") < 2 {
		select {
		case <-deadline:
			closePipe()
			t.Fatal("MCP server did not produce a tool-call response")
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	closePipe()
	select {
	case err := <-done:
		if err != nil && err != io.EOF {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("MCP server did not terminate")
	}

	responses := decodeServerResponses(t, rw.Output())
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses (initialize + tool result), got %d", len(responses))
	}
	toolResult, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call missing result: %v", responses[1])
	}
	if toolResult["isError"] == true {
		t.Fatalf("tools/call returned isError=true: %v", toolResult)
	}
	content, ok := toolResult["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("tools/call content wrong: %v", toolResult)
	}
	text, ok := content[0].(map[string]any)["text"].(string)
	if !ok {
		t.Fatalf("tools/call content text missing: %v", content[0])
	}
	var selection mode.Selection
	if err := json.Unmarshal([]byte(text), &selection); err != nil {
		t.Fatalf("selection does not decode: %v\nraw: %s", err, text)
	}
	if selection.State != verifier.DecisionTied {
		t.Fatalf("expected tied (equal fixture evidence), got state=%s", selection.State)
	}
	if selection.BestIndex != -1 {
		t.Fatalf("expected best_index=-1 for a tie, got %d", selection.BestIndex)
	}
	if len(selection.Scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(selection.Scores))
	}
}
