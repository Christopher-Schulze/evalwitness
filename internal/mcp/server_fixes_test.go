package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

// blockingDispatcher blocks tool calls until the context is cancelled, so
// tests can exercise notifications/cancelled semantics.
type blockingDispatcher struct {
	started chan struct{}
	once    sync.Once
}

func (d *blockingDispatcher) Tools() []Tool { return nil }
func (d *blockingDispatcher) Call(ctx context.Context, _ string, _ json.RawMessage) (any, error) {
	d.once.Do(func() { close(d.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

// pipeRW feeds scripted input lines with control over timing.
type pipeRW struct {
	pr  *io.PipeReader
	pw  *io.PipeWriter
	out strings.Builder
	mu  sync.Mutex
}

func newPipeRW() *pipeRW {
	pr, pw := io.Pipe()
	return &pipeRW{pr: pr, pw: pw}
}

func (p *pipeRW) Read(b []byte) (int, error) { return p.pr.Read(b) }
func (p *pipeRW) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.Write(b)
}
func (p *pipeRW) Output() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.String()
}

func TestServerCancelledCallSendsNoResponse(t *testing.T) {
	disp := &blockingDispatcher{started: make(chan struct{})}
	rw := newPipeRW()
	srv := NewServer("evalwitness-test", "1.0.0", disp, rw, rw)

	done := make(chan error, 1)
	go func() { done <- srv.Serve(context.Background()) }()
	for _, frame := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	} {
		if _, err := rw.pw.Write([]byte(frame + "\n")); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := rw.pw.Write([]byte(`{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"slow","arguments":{}}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-disp.started:
	case <-time.After(5 * time.Second):
		t.Fatal("tool call never started")
	}
	if _, err := rw.pw.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":42}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	// ping proves the loop is still alive after the cancel.
	if _, err := rw.pw.Write([]byte(`{"jsonrpc":"2.0","id":99,"method":"ping"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	_ = rw.pw.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not terminate")
	}

	scanner := bufio.NewScanner(strings.NewReader(rw.Output()))
	for scanner.Scan() {
		var m map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatalf("bad frame: %s", scanner.Text())
		}
		if id, ok := m["id"].(float64); ok && int(id) == 42 {
			t.Fatalf("cancelled call must emit no response, got: %s", scanner.Text())
		}
	}
	if !strings.Contains(rw.Output(), `"id":99`) {
		t.Errorf("ping after cancel got no response: %s", rw.Output())
	}
}

// errDispatcher returns a fixed error for every call.
type errDispatcher struct{ err error }

func (d *errDispatcher) Tools() []Tool { return nil }
func (d *errDispatcher) Call(_ context.Context, _ string, _ json.RawMessage) (any, error) {
	return nil, translateError(d.err)
}

func TestErrorTaxonomyMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode float64
	}{
		{"rate limited", &provider.ProviderError{Provider: "p", Status: 429, Class: provider.ClassRateLimited, RetryAfter: 7}, -32002},
		{"auth failed", &provider.ProviderError{Provider: "p", Status: 401, Class: provider.ClassAuthFailed}, -32007},
		{"context overflow", &provider.ProviderError{Provider: "p", Status: 400, Class: provider.ClassContextOverflow}, -32005},
		{"capability missing", &provider.ProviderError{Provider: "p", Status: 400, Class: provider.ClassCapabilityMissing}, -32003},
		{"generic provider", &provider.ProviderError{Provider: "p", Status: 500, Class: provider.ClassRetryable, Body: "boom"}, -32001},
		{"cost cap", &mode.CostCapError{EstCostUSD: 1.5, CapUSD: 0.5}, -32008},
		{"timeout", context.DeadlineExceeded, -32004},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			disp := &errDispatcher{err: tc.err}
			in := `{"jsonrpc":"2.0","id":10,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n" +
				`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
				`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{}}}` + "\n"
			rw := newRW(in)
			srv := NewServer("evalwitness-test", "1.0.0", disp, rw, rw)
			_ = srv.Serve(context.Background())
			var resp map[string]any
			lines := strings.Split(strings.TrimSpace(rw.out.String()), "\n")
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &resp); err != nil {
				t.Fatalf("parse: %v\n%s", err, rw.out.String())
			}
			rpcErr, ok := resp["error"].(map[string]any)
			if !ok {
				t.Fatalf("expected error response, got %v", resp)
			}
			if rpcErr["code"] != tc.wantCode {
				t.Errorf("code = %v, want %v (%s)", rpcErr["code"], tc.wantCode, rw.out.String())
			}
		})
	}
}

func TestModernToolFailureIsAVisibleToolResult(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	rw := newRW(input)
	server := NewServer("evalwitness-test", "1.2.3", &errDispatcher{err: context.DeadlineExceeded}, rw, rw)
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	response := decodeServerResponses(t, rw.out.String())[0]
	if response["error"] != nil {
		t.Fatalf("modern tool failure became a protocol error: %v", response)
	}
	result := response["result"].(map[string]any)
	if result["resultType"] != "complete" || result["isError"] != true {
		t.Fatalf("modern tool failure result = %v", result)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["code"] != float64(-32004) || structured["message"] != "request timed out" {
		t.Fatalf("modern tool failure content = %v", structured)
	}
}

func TestInitializeEchoesOlderProtocolVersionAndRejectsUnsupported(t *testing.T) {
	disp := &stubDispatcher{}
	in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}` + "\n"
	rw := newRW(in)
	srv := NewServer("evalwitness-test", "1.0.0", disp, rw, rw)
	_ = srv.Serve(context.Background())

	scanner := bufio.NewScanner(strings.NewReader(rw.out.String()))
	var responses []map[string]any
	for scanner.Scan() {
		var m map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		responses = append(responses, m)
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	first := responses[0]["result"].(map[string]any)
	if first["protocolVersion"] != "2024-11-05" {
		t.Errorf("older supported version not echoed: %q", first["protocolVersion"])
	}
	second := responses[1]["error"].(map[string]any)
	if second["code"] != float64(-32600) {
		t.Errorf("repeated initialize error = %v", second)
	}
}

func TestUnsupportedProtocolVersionFailsInsteadOfFallingForward(t *testing.T) {
	rw := newRW(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}` + "\n")
	server := NewServer("evalwitness-test", "1.0.0", &stubDispatcher{}, rw, rw)
	_ = server.Serve(context.Background())
	var response map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(rw.out.String())), &response); err != nil {
		t.Fatal(err)
	}
	rpcError := response["error"].(map[string]any)
	if rpcError["code"] != float64(-32602) {
		t.Fatalf("unsupported version response = %v", response)
	}
}

func TestToolCallNotificationNeverInvokesDispatcher(t *testing.T) {
	dispatcher := &stubDispatcher{}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"echo","arguments":{"x":1}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	}, "\n") + "\n"
	rw := newRW(input)
	server := NewServer("evalwitness-test", "1.0.0", dispatcher, rw, rw)
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.called) != 0 {
		t.Fatalf("notification invoked tools: %v", dispatcher.called)
	}
	if lines := strings.Count(strings.TrimSpace(rw.out.String()), "\n") + 1; lines != 2 {
		t.Fatalf("notification produced a response: %s", rw.out.String())
	}
}

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestResponseWriteFailureTerminatesServeWithError(t *testing.T) {
	want := errors.New("synthetic write failure")
	server := NewServer("evalwitness-test", "1.0.0", &stubDispatcher{}, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), failingWriter{err: want})
	err := server.Serve(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("serve error = %v, want write failure", err)
	}
}

func TestResponseWriteFailureInterruptsBlockingInput(t *testing.T) {
	reader, writer := io.Pipe()
	want := errors.New("synthetic write failure")
	server := NewServer("evalwitness-test", "1.0.0", &stubDispatcher{}, reader, failingWriter{err: want})
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background()) }()
	if _, err := writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, want) {
			t.Fatalf("serve error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("write failure did not interrupt blocking MCP input")
	}
	_ = writer.Close()
}

func TestNullAndReusedRequestIDsAreRejected(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":null,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	}, "\n") + "\n"
	rw := newRW(input)
	server := NewServer("evalwitness-test", "1.0.0", &stubDispatcher{}, rw, rw)
	_ = server.Serve(context.Background())
	if !strings.Contains(rw.out.String(), `"id":null`) || !strings.Contains(rw.out.String(), "request id was already used") {
		t.Fatalf("invalid ID responses = %s", rw.out.String())
	}
}
