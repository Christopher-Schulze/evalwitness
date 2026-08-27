package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/log"
)

const (
	ProtocolVersion        = "2026-07-28"
	modernListTTLMillis    = 3_600_000
	unsupportedVersionCode = -32022
)

// supportedProtocolVersions is newest-first so discovery is deterministic and
// modern clients select the stateless revision before legacy handshake modes.
var supportedProtocolVersions = []string{
	ProtocolVersion,
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

type protocolEra uint8

const (
	eraLegacy protocolEra = iota
	eraModern
)

type Server struct {
	handler ToolDispatcher
	in      *bufio.Scanner
	inClose io.Closer
	out     io.Writer
	outMu   sync.Mutex
	name    string
	version string

	inFlightMu sync.Mutex
	inFlight   map[string]*inFlightCall

	wg sync.WaitGroup

	state    serverState
	seenIDs  map[string]struct{}
	writeErr error
}

type serverState uint8

const (
	statePreInitialize serverState = iota
	stateInitializing
	stateReady
)

// inFlightCall tracks one running tools/call so notifications/cancelled can
// abort it and mark it as client-cancelled (cancelled calls get no response).
type inFlightCall struct {
	cancel    context.CancelFunc
	cancelled bool
}

type ToolDispatcher interface {
	Tools() []Tool
	Call(ctx context.Context, name string, args json.RawMessage) (any, error)
}

func NewServer(name, version string, handler ToolDispatcher, in io.Reader, out io.Writer) *Server {
	s := &Server{
		handler:  handler,
		out:      out,
		name:     name,
		version:  version,
		inFlight: map[string]*inFlightCall{},
		seenIDs:  map[string]struct{}{},
	}
	s.in = bufio.NewScanner(in)
	if closer, ok := in.(io.Closer); ok {
		s.inClose = closer
	}
	s.in.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	return s
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type ToolError struct {
	Code    int
	Message string
	Data    any
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (s *Server) Serve(ctx context.Context) error {
	serveCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		s.wg.Wait()
	}()
	for s.in.Scan() {
		if s.currentWriteError() != nil {
			break
		}
		line := s.in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(json.RawMessage("null"), -32700, "parse error", err.Error())
			continue
		}
		if err := validateRPCRequest(req); err != nil {
			id := req.ID
			if len(id) == 0 || string(id) == "null" {
				id = json.RawMessage("null")
			}
			s.sendError(id, -32600, "invalid request", err.Error())
			continue
		}
		if len(req.ID) == 0 {
			s.dispatchNotification(req)
			continue
		}
		era, err := s.resolveRequestEra(req)
		if err != nil {
			var protocolError *requestProtocolError
			if errors.As(err, &protocolError) {
				s.sendError(req.ID, protocolError.code, protocolError.message, protocolError.data)
			} else {
				s.sendError(req.ID, -32602, "invalid params", err.Error())
			}
			continue
		}
		key := string(req.ID)
		if _, exists := s.seenIDs[key]; exists {
			s.sendError(req.ID, -32600, "invalid request", "request id was already used")
			continue
		}
		s.seenIDs[key] = struct{}{}
		s.dispatch(serveCtx, req, era)
	}
	cancel()
	s.wg.Wait()
	return errors.Join(s.in.Err(), s.currentWriteError())
}

// dispatch routes a single request. Long-running tools/call requests are
// handled in a goroutine so the read loop continues processing cancellation
// notifications and additional requests in parallel.
func (s *Server) dispatch(ctx context.Context, req rpcRequest, era protocolEra) {
	switch req.Method {
	case "server/discover":
		if era != eraModern {
			s.sendError(req.ID, -32601, "method not found", req.Method)
			return
		}
		s.sendResult(req.ID, map[string]any{
			"supportedVersions": supportedVersionNames(),
			"capabilities":      map[string]any{"tools": map[string]any{}},
			"instructions":      "Audit coding-agent trajectories with evidence-bound verifier tools; live execution requires explicit authorization.",
			"ttlMs":             modernListTTLMillis,
			"cacheScope":        "public",
		}, era)
	case "initialize":
		if s.state != statePreInitialize {
			s.sendError(req.ID, -32600, "invalid request", "initialize was already completed")
			return
		}
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(req.Params) == 0 || json.Unmarshal(req.Params, &p) != nil || p.ProtocolVersion == "" {
			s.sendError(req.ID, -32602, "invalid params", "initialize requires protocolVersion")
			return
		}
		if !legacyProtocolVersion(p.ProtocolVersion) {
			s.sendError(req.ID, -32602, "unsupported protocol version", map[string]any{"supported": supportedVersionNames(), "requested": p.ProtocolVersion})
			return
		}
		s.state = stateInitializing
		s.sendResult(req.ID, map[string]any{
			"protocolVersion": p.ProtocolVersion,
			"capabilities": map[string]any{
				"tools":   map[string]any{"listChanged": false},
				"logging": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    s.name,
				"version": s.version,
			},
		}, eraLegacy)
	case "tools/list":
		if era == eraLegacy && !s.requireReady(req.ID) {
			return
		}
		result := map[string]any{"tools": s.handler.Tools()}
		if era == eraModern {
			result["ttlMs"] = modernListTTLMillis
			result["cacheScope"] = "public"
		}
		s.sendResult(req.ID, result, era)
	case "tools/call":
		if era == eraLegacy && !s.requireReady(req.ID) {
			return
		}
		s.wg.Add(1)
		go s.handleToolCall(ctx, req, era)
	case "ping":
		if era == eraModern {
			s.sendError(req.ID, -32601, "method not found", req.Method)
			return
		}
		s.sendResult(req.ID, map[string]any{}, era)
	case "logging/setLevel":
		if era == eraModern {
			s.sendError(req.ID, -32601, "method not found", req.Method)
			return
		}
		if !s.requireReady(req.ID) {
			return
		}
		var p struct {
			Level string `json:"level"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.Level != "" {
			log.SetLevel(p.Level)
		}
		s.sendResult(req.ID, map[string]any{}, era)
	default:
		s.sendError(req.ID, -32601, "method not found", req.Method)
	}
}

func (s *Server) dispatchNotification(req rpcRequest) {
	switch req.Method {
	case "notifications/initialized":
		if s.state == stateInitializing {
			s.state = stateReady
		}
	case "notifications/cancelled":
		var params struct {
			RequestID json.RawMessage `json:"requestId"`
		}
		if json.Unmarshal(req.Params, &params) == nil && validRequestID(params.RequestID) {
			s.cancelInFlight(params.RequestID)
		}
	}
}

func (s *Server) requireReady(id json.RawMessage) bool {
	if s.state == stateReady {
		return true
	}
	s.sendError(id, -32600, "invalid request", "server has not completed initialization")
	return false
}

func (s *Server) handleToolCall(parent context.Context, req rpcRequest, era protocolEra) {
	defer s.wg.Done()

	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.sendError(req.ID, -32602, "invalid params", err.Error())
		return
	}

	ctx, cancel := context.WithCancel(parent)
	idKey := s.registerInFlight(req.ID, cancel)
	defer s.unregisterInFlight(idKey)

	result, err := s.handler.Call(ctx, p.Name, p.Arguments)
	if err != nil {
		if s.wasCancelled(idKey) {
			// MCP: a call aborted by notifications/cancelled returns no
			// response at all.
			return
		}
		s.sendCallError(req.ID, err, era)
		return
	}
	b, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		s.sendError(req.ID, -32603, "internal error", "encode tool result: "+marshalErr.Error())
		return
	}
	response := map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": string(b)},
		},
		"isError": false,
	}
	if era == eraModern {
		response["structuredContent"] = result
	}
	s.sendResult(req.ID, response, era)
}

func (s *Server) registerInFlight(id json.RawMessage, cancel context.CancelFunc) string {
	if id == nil {
		return ""
	}
	key := string(id)
	s.inFlightMu.Lock()
	s.inFlight[key] = &inFlightCall{cancel: cancel}
	s.inFlightMu.Unlock()
	return key
}

func (s *Server) unregisterInFlight(key string) {
	if key == "" {
		return
	}
	s.inFlightMu.Lock()
	delete(s.inFlight, key)
	s.inFlightMu.Unlock()
}

func (s *Server) cancelInFlight(id json.RawMessage) {
	if len(id) == 0 {
		return
	}
	s.inFlightMu.Lock()
	call, ok := s.inFlight[string(id)]
	if ok {
		call.cancelled = true
	}
	s.inFlightMu.Unlock()
	if ok {
		call.cancel()
	}
}

// wasCancelled reports whether the call was aborted via
// notifications/cancelled, and cleans up the tombstone entry.
func (s *Server) wasCancelled(key string) bool {
	if key == "" {
		return false
	}
	s.inFlightMu.Lock()
	defer s.inFlightMu.Unlock()
	call, ok := s.inFlight[key]
	if !ok {
		return false
	}
	if call.cancelled {
		delete(s.inFlight, key)
		return true
	}
	return false
}

func (s *Server) sendResult(id json.RawMessage, result map[string]any, era protocolEra) {
	if len(id) == 0 {
		return
	}
	if era == eraModern {
		result["resultType"] = "complete"
		result["_meta"] = map[string]any{"io.modelcontextprotocol/serverInfo": map[string]any{
			"name": s.name, "version": s.version,
		}}
	}
	s.write(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) sendError(id json.RawMessage, code int, msg string, data any) {
	s.write(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg, Data: data}})
}

func (s *Server) sendCallError(id json.RawMessage, err error, era protocolEra) {
	var te *ToolError
	if errors.As(err, &te) {
		if era == eraModern && te.Code != -32601 {
			s.sendResult(id, map[string]any{
				"content": []any{map[string]any{"type": "text", "text": te.Message}},
				"structuredContent": map[string]any{
					"code": te.Code, "message": te.Message, "data": te.Data,
				},
				"isError": true,
			}, era)
			return
		}
		s.sendError(id, te.Code, te.Message, te.Data)
		return
	}
	s.sendError(id, -32603, "internal error", err.Error())
}

type requestProtocolError struct {
	code    int
	message string
	data    any
}

func (failure *requestProtocolError) Error() string { return failure.message }

func (s *Server) resolveRequestEra(request rpcRequest) (protocolEra, error) {
	if request.Method == "initialize" {
		return eraLegacy, nil
	}
	version, modern, err := requestProtocolMetadata(request.Params)
	if err != nil {
		return eraLegacy, err
	}
	if modern {
		if version != ProtocolVersion {
			return eraLegacy, &requestProtocolError{
				code: unsupportedVersionCode, message: "Unsupported protocol version",
				data: map[string]any{"supported": []string{ProtocolVersion}, "requested": version},
			}
		}
		return eraModern, nil
	}
	if request.Method == "server/discover" {
		return eraLegacy, errors.New("server/discover requires per-request MCP metadata")
	}
	return eraLegacy, nil
}

func requestProtocolMetadata(raw json.RawMessage) (string, bool, error) {
	if len(raw) == 0 {
		return "", false, nil
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil {
		return "", false, errors.New("request params must be an object")
	}
	metaRaw, present := params["_meta"]
	if !present {
		return "", false, nil
	}
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(metaRaw, &metadata); err != nil || metadata == nil {
		return "", true, errors.New("request _meta must be an object")
	}
	var version string
	if err := json.Unmarshal(metadata["io.modelcontextprotocol/protocolVersion"], &version); err != nil || version == "" {
		return "", true, errors.New("request _meta requires io.modelcontextprotocol/protocolVersion")
	}
	var capabilities map[string]json.RawMessage
	if err := json.Unmarshal(metadata["io.modelcontextprotocol/clientCapabilities"], &capabilities); err != nil || capabilities == nil {
		return "", true, errors.New("request _meta requires object io.modelcontextprotocol/clientCapabilities")
	}
	if clientRaw, found := metadata["io.modelcontextprotocol/clientInfo"]; found {
		var client struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(clientRaw, &client); err != nil || client.Name == "" || client.Version == "" {
			return "", true, errors.New("request clientInfo requires name and version")
		}
	}
	return version, true, nil
}

func (s *Server) write(resp rpcResponse) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	if s.writeErr != nil {
		return
	}
	enc := json.NewEncoder(s.out)
	if err := enc.Encode(resp); err != nil {
		s.writeErr = fmt.Errorf("write MCP response: %w", err)
		if s.inClose != nil {
			_ = s.inClose.Close()
		}
	}
}

func (s *Server) currentWriteError() error {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	return s.writeErr
}

func validateRPCRequest(request rpcRequest) error {
	if request.JSONRPC != "2.0" {
		return errors.New("jsonrpc must equal 2.0")
	}
	if request.Method == "" {
		return errors.New("method is required")
	}
	if len(request.ID) > 0 && !validRequestID(request.ID) {
		return errors.New("id must be a string or integer and must not be null")
	}
	return nil
}

func validRequestID(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return true
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&number) != nil {
		return false
	}
	_, err := number.Int64()
	return err == nil
}

func supportedVersionNames() []string {
	versions := slices.Clone(supportedProtocolVersions)
	return versions
}

func legacyProtocolVersion(version string) bool {
	return slices.Contains(supportedProtocolVersions[1:], version)
}
