package preprocess

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Capture adapters: parse agent session transcripts (Claude Code, Codex,
// OpenCode) into the canonical Step sequence so users can verify real
// sessions directly: evalwitness verify --trajectory @session.jsonl.
// Shapes were derived from real session files of each tool.

const maxInlineToolInput = 400

// ParseAgentSession tries all known session-transcript formats in order.
// Returns (steps, true) on the first match.
func ParseAgentSession(s string) ([]Step, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || trimmed[0] != '{' {
		return nil, false
	}
	if steps, ok := ParseOpenCodeExport(trimmed); ok {
		return steps, true
	}
	if steps, ok := ParseClaudeCodeSession(trimmed); ok {
		return steps, true
	}
	if steps, ok := ParseCodexRollout(trimmed); ok {
		return steps, true
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Claude Code session JSONL
// ---------------------------------------------------------------------------

type claudeCodeLine struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type claudeCodeContentItem struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Text      string          `json:"text"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

// ParseClaudeCodeSession parses a Claude Code session JSONL transcript
// (~/.claude/projects/<slug>/<session>.jsonl). Assistant text and tool_use
// entries become steps; tool_result entries attach as the output of the
// step that issued the matching tool_use id.
func ParseClaudeCodeSession(s string) ([]Step, bool) {
	lines := strings.Split(s, "\n")
	var steps []Step
	pendingByToolID := map[string]int{}
	sawSessionLine := false

	appendStep := func(st Step) int {
		st.StepID = fmt.Sprintf("%d", len(steps)+1)
		st.Source = "agent"
		steps = append(steps, st)
		return len(steps) - 1
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var l claudeCodeLine
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			return nil, false
		}
		switch l.Type {
		case "assistant":
			if l.Message.Role != "assistant" {
				continue
			}
			sawSessionLine = true
			var items []claudeCodeContentItem
			if err := json.Unmarshal(l.Message.Content, &items); err != nil {
				continue
			}
			var textParts []string
			for _, it := range items {
				switch it.Type {
				case "text":
					if strings.TrimSpace(it.Text) != "" {
						textParts = append(textParts, strings.TrimSpace(it.Text))
					}
				case "tool_use":
					idx := appendStep(Step{
						Message: strings.Join(textParts, "\n"),
						Command: formatToolInvocation(it.Name, it.Input),
					})
					textParts = nil
					if it.ID != "" {
						pendingByToolID[it.ID] = idx
					}
				}
			}
			if len(textParts) > 0 {
				appendStep(Step{Message: strings.Join(textParts, "\n")})
			}
		case "user":
			// Tool results come back as user-role lines.
			var items []claudeCodeContentItem
			if err := json.Unmarshal(l.Message.Content, &items); err != nil {
				continue
			}
			for _, it := range items {
				if it.Type != "tool_result" {
					continue
				}
				sawSessionLine = true
				out := decodeClaudeToolResult(it.Content)
				if idx, ok := pendingByToolID[it.ToolUseID]; ok {
					steps[idx].Output = out
					delete(pendingByToolID, it.ToolUseID)
				} else if out != "" {
					appendStep(Step{Output: out})
				}
			}
		}
	}
	if !sawSessionLine || len(steps) == 0 {
		return nil, false
	}
	return steps, true
}

func decodeClaudeToolResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var asItems []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &asItems); err == nil {
		var parts []string
		for _, it := range asItems {
			if it.Type == "text" && it.Text != "" {
				parts = append(parts, it.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func formatToolInvocation(name string, input json.RawMessage) string {
	if name == "" {
		return ""
	}
	// Shell-like tools render their command directly; everything else shows
	// compact JSON input.
	var shell struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &shell); err == nil && shell.Command != "" {
		return name + ": " + shell.Command
	}
	compact := strings.TrimSpace(string(input))
	if len(compact) > maxInlineToolInput {
		compact = compact[:maxInlineToolInput] + "...(truncated)"
	}
	if compact == "" || compact == "null" {
		return name
	}
	return name + " " + compact
}

// ---------------------------------------------------------------------------
// Codex rollout JSONL
// ---------------------------------------------------------------------------

type codexLine struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Input     string          `json:"input"`
	Output    string          `json:"output"`
	CallID    string          `json:"call_id"`
}

// ParseCodexRollout parses a Codex CLI session rollout JSONL
// (~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl). response_item payloads
// carry assistant messages, function calls, and their outputs.
func ParseCodexRollout(s string) ([]Step, bool) {
	lines := strings.Split(s, "\n")
	var steps []Step
	pendingByCallID := map[string]int{}
	sawCodexMarker := false

	appendStep := func(st Step) int {
		st.StepID = fmt.Sprintf("%d", len(steps)+1)
		st.Source = "agent"
		steps = append(steps, st)
		return len(steps) - 1
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var l codexLine
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			return nil, false
		}
		switch l.Type {
		case "session_meta", "turn_context":
			sawCodexMarker = true
			continue
		case "response_item":
			sawCodexMarker = true
		default:
			continue
		}
		var p codexPayload
		if err := json.Unmarshal(l.Payload, &p); err != nil {
			continue
		}
		switch p.Type {
		case "message":
			if p.Role != "assistant" {
				continue
			}
			text := codexMessageText(p.Content)
			if text != "" {
				appendStep(Step{Message: text})
			}
		case "function_call":
			idx := appendStep(Step{Command: codexCommand(p.Name, p.Arguments)})
			if p.CallID != "" {
				pendingByCallID[p.CallID] = idx
			}
		case "function_call_output", "custom_tool_call_output":
			if idx, ok := pendingByCallID[p.CallID]; ok {
				steps[idx].Output = p.Output
				delete(pendingByCallID, p.CallID)
			} else if p.Output != "" {
				appendStep(Step{Output: p.Output})
			}
		case "custom_tool_call":
			idx := appendStep(Step{Command: codexCommand(p.Name, p.Input)})
			if p.CallID != "" {
				pendingByCallID[p.CallID] = idx
			}
		}
	}
	if !sawCodexMarker || len(steps) == 0 {
		return nil, false
	}
	return steps, true
}

func codexMessageText(raw json.RawMessage) string {
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var parts []string
	for _, it := range items {
		if (it.Type == "output_text" || it.Type == "input_text") && strings.TrimSpace(it.Text) != "" {
			parts = append(parts, strings.TrimSpace(it.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func codexCommand(name, args string) string {
	// exec-style tools carry a JSON argument object with cmd/command.
	var shell struct {
		Cmd     any `json:"cmd"`
		Command any `json:"command"`
	}
	if err := json.Unmarshal([]byte(args), &shell); err == nil {
		for _, v := range []any{shell.Cmd, shell.Command} {
			switch c := v.(type) {
			case string:
				if c != "" {
					return name + ": " + c
				}
			case []any:
				var parts []string
				for _, e := range c {
					if s, ok := e.(string); ok {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 {
					return name + ": " + strings.Join(parts, " ")
				}
			}
		}
	}
	compact := strings.TrimSpace(args)
	if len(compact) > maxInlineToolInput {
		compact = compact[:maxInlineToolInput] + "...(truncated)"
	}
	if compact == "" {
		return name
	}
	return name + " " + compact
}

// ---------------------------------------------------------------------------
// OpenCode export JSON
// ---------------------------------------------------------------------------

type openCodeExport struct {
	Info struct {
		ID string `json:"id"`
	} `json:"info"`
	Messages []struct {
		Info struct {
			Role string `json:"role"`
		} `json:"info"`
		Parts []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Tool  string          `json:"tool"`
			State json.RawMessage `json:"state"`
		} `json:"parts"`
	} `json:"messages"`
}

// ParseOpenCodeExport parses the JSON produced by `opencode export
// <sessionID>`: {info: {...}, messages: [{info: {role}, parts: [...]}]}.
func ParseOpenCodeExport(s string) ([]Step, bool) {
	var e openCodeExport
	if err := json.Unmarshal([]byte(s), &e); err != nil {
		return nil, false
	}
	if e.Info.ID == "" || len(e.Messages) == 0 {
		return nil, false
	}
	var steps []Step
	appendStep := func(st Step) {
		st.StepID = fmt.Sprintf("%d", len(steps)+1)
		st.Source = "agent"
		steps = append(steps, st)
	}
	for _, m := range e.Messages {
		if m.Info.Role != "assistant" {
			continue
		}
		var textParts []string
		for _, p := range m.Parts {
			switch p.Type {
			case "text":
				if strings.TrimSpace(p.Text) != "" {
					textParts = append(textParts, strings.TrimSpace(p.Text))
				}
			case "tool":
				var st struct {
					Input  json.RawMessage `json:"input"`
					Output string          `json:"output"`
				}
				_ = json.Unmarshal(p.State, &st)
				appendStep(Step{
					Message: strings.Join(textParts, "\n"),
					Command: formatToolInvocation(p.Tool, st.Input),
					Output:  st.Output,
				})
				textParts = nil
			}
		}
		if len(textParts) > 0 {
			appendStep(Step{Message: strings.Join(textParts, "\n")})
		}
	}
	if len(steps) == 0 {
		return nil, false
	}
	return steps, true
}
