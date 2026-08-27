package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type stubDispatcher struct {
	tools  []Tool
	called []string
}

func (s *stubDispatcher) Tools() []Tool { return s.tools }
func (s *stubDispatcher) Call(_ context.Context, name string, args json.RawMessage) (any, error) {
	s.called = append(s.called, name)
	if name == "echo" {
		return map[string]any{"echoed": json.RawMessage(args)}, nil
	}
	return nil, &ToolError{Code: -32601, Message: "unknown tool"}
}

type rwBuf struct {
	in  *strings.Reader
	out *strings.Builder
}

func newRW(input string) *rwBuf {
	return &rwBuf{in: strings.NewReader(input), out: &strings.Builder{}}
}
func (r *rwBuf) Read(p []byte) (int, error)  { return r.in.Read(p) }
func (r *rwBuf) Write(p []byte) (int, error) { return r.out.Write(p) }

func TestServerInitializeAndToolsList(t *testing.T) {
	disp := &stubDispatcher{tools: []Tool{
		{Name: "echo", Description: "echoes input", InputSchema: map[string]any{"type": "object"}},
	}}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	}, "\n") + "\n"

	rw := newRW(in)
	srv := NewServer("evalwitness-test", "1.0.0", disp, rw, rw)
	if err := srv.Serve(context.Background()); err != nil && err != io.EOF {
		t.Fatalf("serve: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(rw.out.String()))
	var responses []map[string]any
	for scanner.Scan() {
		var m map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatalf("unmarshal output: %v\nline: %s", err, scanner.Text())
		}
		responses = append(responses, m)
	}
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	init := responses[0]
	result, ok := init["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize: missing result: %v", init)
	}
	if result["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v, want legacy version echo", result["protocolVersion"])
	}
	if info, ok := result["serverInfo"].(map[string]any); !ok || info["name"] != "evalwitness-test" {
		t.Errorf("serverInfo wrong: %v", result["serverInfo"])
	}

	listRes, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list missing result")
	}
	tools, ok := listRes["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools list wrong: %v", listRes)
	}
}

func TestModernServerDiscoveryAndStatelessToolsList(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":"discover","method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test-client","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}`,
		`{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`,
	}, "\n") + "\n"
	rw := newRW(input)
	server := NewServer("evalwitness-test", "1.2.3", &stubDispatcher{tools: []Tool{{Name: "echo"}}}, rw, rw)
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	responses := decodeServerResponses(t, rw.out.String())
	if len(responses) != 2 {
		t.Fatalf("modern response count = %d, want 2", len(responses))
	}
	discovery := responses[0]["result"].(map[string]any)
	versions := discovery["supportedVersions"].([]any)
	if len(versions) != len(supportedProtocolVersions) || versions[0] != ProtocolVersion || discovery["resultType"] != "complete" || discovery["cacheScope"] != "public" {
		t.Fatalf("modern discovery result = %v", discovery)
	}
	assertModernServerInfo(t, discovery, "1.2.3")
	tools := responses[1]["result"].(map[string]any)
	if tools["resultType"] != "complete" || tools["cacheScope"] != "public" || len(tools["tools"].([]any)) != 1 {
		t.Fatalf("modern tools/list result = %v", tools)
	}
	assertModernServerInfo(t, tools, "1.2.3")
}

func TestModernRequestsRejectMissingOrUnsupportedMetadata(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"1900-01-01","io.modelcontextprotocol/clientCapabilities":{}}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`,
	}, "\n") + "\n"
	rw := newRW(input)
	server := NewServer("evalwitness-test", "1.2.3", &stubDispatcher{}, rw, rw)
	_ = server.Serve(context.Background())
	responses := decodeServerResponses(t, rw.out.String())
	first := responses[0]["error"].(map[string]any)
	if first["code"] != float64(unsupportedVersionCode) {
		t.Fatalf("unsupported-version response = %v", first)
	}
	data := first["data"].(map[string]any)
	if data["requested"] != "1900-01-01" || data["supported"].([]any)[0] != ProtocolVersion {
		t.Fatalf("unsupported-version data = %v", data)
	}
	second := responses[1]["error"].(map[string]any)
	if second["code"] != float64(-32602) {
		t.Fatalf("missing-capabilities response = %v", second)
	}
}

func TestModernRequestsRejectRemovedPingAndLoggingMethods(t *testing.T) {
	metadata := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"ping","params":{` + metadata + `}}`,
		`{"jsonrpc":"2.0","id":2,"method":"logging/setLevel","params":{"level":"debug",` + metadata + `}}`,
	}, "\n") + "\n"
	rw := newRW(input)
	server := NewServer("evalwitness-test", "1.2.3", &stubDispatcher{}, rw, rw)
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	responses := decodeServerResponses(t, rw.out.String())
	if len(responses) != 2 {
		t.Fatalf("removed-method response count = %d, want 2", len(responses))
	}
	for index, response := range responses {
		rpcError := response["error"].(map[string]any)
		if rpcError["code"] != float64(-32601) {
			t.Fatalf("removed-method response %d = %v", index, response)
		}
	}
}

func TestModernToolCallReturnsStructuredContentWithoutInitialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"value":7},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	dispatcher := &stubDispatcher{tools: []Tool{{Name: "echo"}}}
	rw := newRW(input)
	server := NewServer("evalwitness-test", "1.2.3", dispatcher, rw, rw)
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	result := decodeServerResponses(t, rw.out.String())[0]["result"].(map[string]any)
	if result["resultType"] != "complete" || result["isError"] != false {
		t.Fatalf("modern tool result = %v", result)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["echoed"].(map[string]any)["value"] != float64(7) {
		t.Fatalf("modern structured content = %v", structured)
	}
	assertModernServerInfo(t, result, "1.2.3")
}

func decodeServerResponses(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var responses []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		var response map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatalf("decode MCP response: %v: %s", err, scanner.Text())
		}
		responses = append(responses, response)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return responses
}

func assertModernServerInfo(t *testing.T, result map[string]any, version string) {
	t.Helper()
	metadata := result["_meta"].(map[string]any)
	server := metadata["io.modelcontextprotocol/serverInfo"].(map[string]any)
	if server["name"] != "evalwitness-test" || server["version"] != version {
		t.Fatalf("modern server info = %v", server)
	}
}

func TestServerToolCall(t *testing.T) {
	disp := &stubDispatcher{tools: []Tool{{Name: "echo", InputSchema: map[string]any{"type": "object"}}}}
	in := `{"jsonrpc":"2.0","id":10,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"x":1}}}` + "\n"
	rw := newRW(in)
	srv := NewServer("evalwitness-test", "1.0.0", disp, rw, rw)
	if err := srv.Serve(context.Background()); err != nil && err != io.EOF {
		t.Fatalf("serve: %v", err)
	}
	if len(disp.called) != 1 || disp.called[0] != "echo" {
		t.Errorf("dispatcher.Call not invoked correctly: %v", disp.called)
	}
	var resp map[string]any
	lines := strings.Split(strings.TrimSpace(rw.out.String()), "\n")
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &resp); err != nil {
		t.Fatalf("parse response: %v\n%s", err, rw.out.String())
	}
	res, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result")
	}
	if isErr, ok := res["isError"].(bool); !ok || isErr {
		t.Errorf("expected isError=false, got %v", res["isError"])
	}
}

func TestServerInvalidJSON(t *testing.T) {
	disp := &stubDispatcher{}
	in := "not-json\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	rw := newRW(in)
	srv := NewServer("evalwitness-test", "1.0.0", disp, rw, rw)
	_ = srv.Serve(context.Background())
	if !strings.Contains(rw.out.String(), `"code":-32700`) {
		t.Errorf("expected parse error, got: %s", rw.out.String())
	}
	if !strings.Contains(rw.out.String(), `"id":1`) {
		t.Errorf("expected ping response, got: %s", rw.out.String())
	}
}

func TestServerUnknownMethod(t *testing.T) {
	disp := &stubDispatcher{}
	in := `{"jsonrpc":"2.0","id":1,"method":"some/unknown"}` + "\n"
	rw := newRW(in)
	srv := NewServer("evalwitness-test", "1.0.0", disp, rw, rw)
	_ = srv.Serve(context.Background())
	if !strings.Contains(rw.out.String(), `"code":-32601`) {
		t.Errorf("expected method not found, got: %s", rw.out.String())
	}
}
