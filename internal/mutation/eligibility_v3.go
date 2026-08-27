package mutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func verifiedExecutionInvocation(trajectory preprocess.Trajectory, evidence preprocess.Event) (InvocationProof, string, bool) {
	var rejected []InvocationProof
	for _, toolCall := range trajectory.Events {
		if toolCall.ToolCall == nil || !toolCallLinkedToEvidence(trajectory, toolCall, evidence) {
			continue
		}
		commands := linkedCommandEvents(trajectory, toolCall.ID)
		if len(commands) == 0 {
			rejected = append(rejected, InvocationProof{
				ParserVersion: InvocationParserVersion, ToolCallEventID: toolCall.ID,
				EvidenceEventID: evidence.ID, SegmentIndex: 0, Executable: "<missing-command-event>",
				Arguments: []string{}, WrapperChain: []string{}, DirectInvocation: false,
				ParseStatus: InvocationRejected, Decision: "missing_command_display_event",
			})
			continue
		}
		for _, command := range commands {
			proofs := invocationProofs(toolCall.ID, command, evidence.ID)
			for _, proof := range proofs {
				if proof.DirectInvocation {
					return proof, proof.SemanticRole, true
				}
				rejected = append(rejected, proof)
			}
		}
	}
	if len(rejected) == 0 {
		return InvocationProof{
			ParserVersion: InvocationParserVersion, EvidenceEventID: evidence.ID,
			SegmentIndex: 0, Executable: "<missing-linked-tool-call>", Arguments: []string{}, WrapperChain: []string{},
			ParseStatus: InvocationRejected, Decision: "missing_linked_tool_call",
		}, "", false
	}
	sort.Slice(rejected, func(left, right int) bool {
		if rejected[left].ToolCallEventID != rejected[right].ToolCallEventID {
			return rejected[left].ToolCallEventID < rejected[right].ToolCallEventID
		}
		if rejected[left].CommandEventID != rejected[right].CommandEventID {
			return rejected[left].CommandEventID < rejected[right].CommandEventID
		}
		return rejected[left].SegmentIndex < rejected[right].SegmentIndex
	})
	return rejected[0], "", false
}

func toolCallLinkedToEvidence(trajectory preprocess.Trajectory, toolCall, evidence preprocess.Event) bool {
	if evidence.ToolResult != nil && evidence.ToolResult.CallID != "" && evidence.ToolResult.CallID == toolCall.ToolCall.CallID {
		return true
	}
	for _, link := range trajectory.Links {
		if link.ToID == evidence.ID && link.FromID == toolCall.ID && (link.Kind == preprocess.LinkCallResult || link.Kind == preprocess.LinkParent || link.Kind == preprocess.LinkReference) {
			return true
		}
	}
	return false
}

func linkedCommandEvents(trajectory preprocess.Trajectory, toolCallEventID string) []preprocess.Event {
	byID := make(map[string]preprocess.Event, len(trajectory.Events))
	for _, event := range trajectory.Events {
		byID[event.ID] = event
	}
	var commands []preprocess.Event
	for _, link := range trajectory.Links {
		if link.FromID != toolCallEventID || link.Kind != preprocess.LinkParent {
			continue
		}
		if event, exists := byID[link.ToID]; exists && event.Command != nil {
			commands = append(commands, event)
		}
	}
	sort.Slice(commands, func(left, right int) bool {
		if commands[left].Order != commands[right].Order {
			return commands[left].Order < commands[right].Order
		}
		return commands[left].ID < commands[right].ID
	})
	return commands
}

func invocationProofs(toolCallEventID string, command preprocess.Event, evidenceEventID string) []InvocationProof {
	segments, err := lexShellSegments(command.Command.Display)
	if err != nil || len(segments) == 0 {
		return []InvocationProof{{
			ParserVersion: InvocationParserVersion, ToolCallEventID: toolCallEventID, CommandEventID: command.ID,
			EvidenceEventID: evidenceEventID, SegmentIndex: 0, Executable: "<unparsed>", Arguments: []string{}, WrapperChain: []string{},
			DirectInvocation: false, ParseStatus: InvocationRejected, Decision: "unsupported_shell_syntax",
		}}
	}
	proofs := make([]InvocationProof, 0, len(segments))
	for index, segment := range segments {
		executable, arguments, wrappers, role, direct, decision := classifyInvocation(segment, 0)
		proofs = append(proofs, InvocationProof{
			ParserVersion: InvocationParserVersion, ToolCallEventID: toolCallEventID, CommandEventID: command.ID,
			EvidenceEventID: evidenceEventID, SegmentIndex: index, Executable: executable,
			Arguments: arguments, WrapperChain: wrappers, SemanticRole: role,
			DirectInvocation: direct, ParseStatus: InvocationParsed, Decision: decision,
		})
	}
	return proofs
}

func lexShellSegments(command string) ([][]string, error) {
	var segments [][]string
	var segment []string
	var token strings.Builder
	quote := rune(0)
	escaped := false
	flushToken := func() {
		if token.Len() == 0 {
			return
		}
		segment = append(segment, token.String())
		token.Reset()
	}
	flushSegment := func() {
		flushToken()
		if len(segment) > 0 {
			segments = append(segments, segment)
			segment = nil
		}
	}
	runes := []rune(strings.TrimSpace(command))
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		if escaped {
			token.WriteRune(current)
			escaped = false
			continue
		}
		if quote != 0 {
			if current == quote {
				quote = 0
				continue
			}
			if current == '\\' && quote == '"' {
				escaped = true
				continue
			}
			token.WriteRune(current)
			continue
		}
		switch current {
		case '\\':
			escaped = true
		case '\'', '"':
			quote = current
		case '`', '(', ')', '<', '>':
			return nil, errors.New("unsupported shell syntax")
		case '$':
			if index+1 < len(runes) && runes[index+1] == '(' {
				return nil, errors.New("command substitution is unsupported")
			}
			token.WriteRune(current)
		case '#':
			if token.Len() == 0 {
				for index+1 < len(runes) && runes[index+1] != '\n' {
					index++
				}
				flushSegment()
				continue
			}
			token.WriteRune(current)
		case ';', '\n', '|', '&':
			flushSegment()
			if index+1 < len(runes) && runes[index+1] == current && (current == '|' || current == '&') {
				index++
			}
		default:
			if unicode.IsSpace(current) {
				flushToken()
			} else {
				token.WriteRune(current)
			}
		}
	}
	if escaped || quote != 0 {
		return nil, errors.New("unterminated shell token")
	}
	flushSegment()
	return segments, nil
}

func classifyInvocation(segment []string, depth int) (string, []string, []string, string, bool, string) {
	if len(segment) == 0 || depth > 2 {
		return "<empty>", []string{}, []string{}, "", false, "empty_or_nested_too_deep"
	}
	index := 0
	for index < len(segment) && isEnvironmentAssignment(segment[index]) {
		index++
	}
	wrappers := make([]string, 0)
	for index < len(segment) {
		base := filepath.Base(strings.ToLower(segment[index]))
		switch base {
		case "env":
			wrappers = append(wrappers, base)
			index++
			for index < len(segment) && (strings.HasPrefix(segment[index], "-") || isEnvironmentAssignment(segment[index])) {
				index++
			}
		case "command", "time", "nice":
			wrappers = append(wrappers, base)
			index++
			for index < len(segment) && strings.HasPrefix(segment[index], "-") {
				index++
			}
		default:
			goto classified
		}
	}

classified:
	if index >= len(segment) {
		return "<missing>", []string{}, wrappers, "", false, "wrapper_without_executable"
	}
	executable := filepath.Base(strings.ToLower(segment[index]))
	arguments := append([]string(nil), segment[index+1:]...)
	if executable == "bash" || executable == "sh" || executable == "zsh" {
		for argumentIndex := 0; argumentIndex+1 < len(arguments); argumentIndex++ {
			if arguments[argumentIndex] != "-c" && arguments[argumentIndex] != "-lc" {
				continue
			}
			nested, err := lexShellSegments(arguments[argumentIndex+1])
			if err != nil || len(nested) != 1 {
				return executable, arguments, append(wrappers, executable), "", false, "shell_wrapper_payload_rejected"
			}
			nestedExecutable, nestedArguments, nestedWrappers, role, direct, decision := classifyInvocation(nested[0], depth+1)
			return nestedExecutable, nestedArguments, append(append(wrappers, executable), nestedWrappers...), role, direct, "shell_wrapper:" + decision
		}
	}
	role := exactVerificationRole(executable, arguments)
	if role == "" {
		return executable, arguments, wrappers, "", false, "non_verification_executable"
	}
	return executable, arguments, wrappers, role, true, "direct_verification_invocation"
}

func isEnvironmentAssignment(value string) bool {
	name, _, found := strings.Cut(value, "=")
	if !found || name == "" {
		return false
	}
	for index, current := range name {
		if current != '_' && !unicode.IsLetter(current) && (index == 0 || !unicode.IsDigit(current)) {
			return false
		}
	}
	return true
}

func exactVerificationRole(executable string, arguments []string) string {
	lower := make([]string, len(arguments))
	for index, argument := range arguments {
		lower[index] = strings.ToLower(argument)
	}
	arg := func(index int, value string) bool { return index < len(lower) && lower[index] == value }
	switch executable {
	case "go":
		if arg(0, "test") {
			return "test"
		}
		if arg(0, "vet") {
			return "check"
		}
		if arg(0, "build") {
			return "build"
		}
	case "cargo":
		if arg(0, "test") {
			return "test"
		}
		if arg(0, "check") || arg(0, "clippy") || arg(0, "fmt") {
			return "check"
		}
		if arg(0, "build") {
			return "build"
		}
	case "pytest", "py.test", "ctest":
		return "test"
	case "python", "python3":
		if arg(0, "-m") && (arg(1, "pytest") || arg(1, "unittest")) {
			return "test"
		}
		if len(lower) > 0 {
			base := filepath.Base(lower[0])
			stem := strings.TrimSuffix(base, ".py")
			if strings.HasSuffix(base, ".py") && (strings.HasPrefix(stem, "test_") || strings.HasSuffix(stem, "_test")) {
				return "test"
			}
		}
	case "bun", "npm", "pnpm", "yarn":
		if arg(0, "test") || (arg(0, "run") && arg(1, "test")) {
			return "test"
		}
		if arg(0, "build") || (arg(0, "run") && arg(1, "build")) {
			return "build"
		}
	case "make", "mvn", "gradle", "gradlew":
		if arg(0, "test") {
			return "test"
		}
		if arg(0, "build") {
			return "build"
		}
	case "staticcheck", "golangci-lint", "eslint":
		return "check"
	case "ruff":
		if arg(0, "check") {
			return "check"
		}
	case "tsc":
		for _, argument := range lower {
			if argument == "--noemit" {
				return "check"
			}
		}
	case "diff", "cmp", "sha256sum", "shasum", "assert":
		return "outcome_probe"
	}
	return ""
}

func presentationProof(event preprocess.Event) PresentationProof {
	proof := PresentationProof{
		ClassifierVersion: PresentationClassifierVersion, EventID: event.ID,
		MessageRole: "<none>", ContentKind: PresentationUnknown, Decision: "message_absent",
		LineCount: 1,
	}
	if event.Message == nil {
		return proof
	}
	proof.MessageRole = event.Message.Role
	if proof.MessageRole == "" {
		proof.MessageRole = "<empty>"
	}
	texts := make([]string, 0, len(event.Message.Parts))
	for _, part := range event.Message.Parts {
		if part.Kind == preprocess.ContentText {
			proof.TextPartCount++
			texts = append(texts, part.Text)
		}
	}
	text := strings.Join(texts, "\n")
	trimmed := strings.TrimSpace(text)
	proof.TokenCount = len(strings.Fields(trimmed))
	lines := strings.Split(trimmed, "\n")
	proof.LineCount = len(lines)
	for _, line := range lines {
		if len(line) > proof.MaximumLineBytes {
			proof.MaximumLineBytes = len(line)
		}
	}
	if proof.MessageRole != "assistant" {
		proof.ContentKind, proof.Decision = PresentationNonAssistantRole, "message_role_not_assistant"
		return proof
	}
	if proof.TextPartCount != 1 || trimmed == "" {
		proof.ContentKind, proof.Decision = PresentationUnknown, "requires_one_nonempty_text_part"
		return proof
	}
	if strings.Contains(trimmed, "```") || strings.Contains(trimmed, "~~~") {
		proof.ContentKind, proof.Decision = PresentationCode, "code_fence_present"
		return proof
	}
	var structured any
	if json.Unmarshal([]byte(trimmed), &structured) == nil {
		proof.ContentKind, proof.Decision = PresentationStructuredData, "complete_json_value"
		return proof
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "% ") || strings.HasPrefix(lower, "execute [") {
		proof.ContentKind, proof.Decision = PresentationTerminalCommand, "terminal_command_prefix"
		return proof
	}
	if strings.Contains(lower, "\n$ ") || strings.Contains(lower, "\n% ") {
		proof.ContentKind, proof.Decision = PresentationTerminalTranscript, "terminal_prompt_line_present"
		return proof
	}
	if segments, err := lexShellSegments(trimmed); err == nil && len(segments) > 0 {
		executable, arguments, _, role, direct, _ := classifyInvocation(segments[0], 0)
		if direct || (len(lines) == 1 && commandLikeExecutable(executable, arguments)) || (role != "" && direct) {
			proof.ContentKind, proof.Decision = PresentationTerminalCommand, "command_grammar_match"
			return proof
		}
	}
	proof.ContentKind, proof.Decision = PresentationAssistantProse, "assistant_role_natural_language"
	return proof
}

func commandLikeExecutable(executable string, arguments []string) bool {
	if len(arguments) == 0 {
		return false
	}
	known := []string{"bash", "sh", "zsh", "go", "cargo", "python", "python3", "pytest", "git", "make", "npm", "pnpm", "yarn", "bun", "curl", "wget", "grep", "rg", "sed", "awk", "echo", "printf"}
	for _, candidate := range known {
		if executable == candidate {
			return true
		}
	}
	return strings.HasPrefix(executable, "./")
}

func naturalFormattingV3(event preprocess.Event) (string, PresentationProof, bool) {
	proof := presentationProof(event)
	if proof.ContentKind != PresentationAssistantProse {
		return "", proof, false
	}
	before := ""
	for _, part := range event.Message.Parts {
		if part.Kind == preprocess.ContentText {
			before = part.Text
		}
	}
	trimmed := strings.TrimSpace(before)
	if len(strings.Fields(trimmed)) < 12 || len(trimmed) <= 72 || !strings.ContainsAny(trimmed, ".,;:!?") {
		proof.Decision = "assistant_prose_outside_wrap_envelope"
		return "", proof, false
	}
	after := wrapTokens(strings.Fields(trimmed), 72)
	if after == before || hasSingleTokenLine(after) {
		proof.Decision = "assistant_prose_wrap_not_natural"
		return "", proof, false
	}
	proof.Decision = "assistant_prose_wrap_eligible"
	return after, proof, true
}

func invocationProofEventIDs(proof InvocationProof) []string {
	values := []string{proof.ToolCallEventID, proof.CommandEventID, proof.EvidenceEventID}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return sortedStrings(result)
}

func describeInvocation(proof InvocationProof) string {
	return fmt.Sprintf("executable=%s direct=%t decision=%s", proof.Executable, proof.DirectInvocation, proof.Decision)
}
