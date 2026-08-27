package preprocess

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type codexRecordEnvelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type codexResponsePayload struct {
	Type             string          `json:"type"`
	ID               string          `json:"id"`
	Role             string          `json:"role"`
	Author           string          `json:"author"`
	Recipient        string          `json:"recipient"`
	Content          json.RawMessage `json:"content"`
	Summary          json.RawMessage `json:"summary"`
	EncryptedContent string          `json:"encrypted_content"`
	Phase            string          `json:"phase"`
	CallID           string          `json:"call_id"`
	Name             string          `json:"name"`
	Namespace        string          `json:"namespace"`
	Arguments        json.RawMessage `json:"arguments"`
	Input            string          `json:"input"`
	Output           json.RawMessage `json:"output"`
	Status           string          `json:"status"`
	Action           json.RawMessage `json:"action"`
	Result           string          `json:"result"`
	RevisedPrompt    string          `json:"revised_prompt"`
	Message          string          `json:"message"`
}

type codexContentPart struct {
	Type             string `json:"type"`
	Text             string `json:"text"`
	ImageURL         string `json:"image_url"`
	AudioURL         string `json:"audio_url"`
	EncryptedContent string `json:"encrypted_content"`
}

type codexEventPayload struct {
	Type             string                     `json:"type"`
	Message          string                     `json:"message"`
	Phase            string                     `json:"phase"`
	CallID           string                     `json:"call_id"`
	Command          []string                   `json:"command"`
	CWD              string                     `json:"cwd"`
	Stream           string                     `json:"stream"`
	Chunk            string                     `json:"chunk"`
	Stdout           string                     `json:"stdout"`
	Stderr           string                     `json:"stderr"`
	AggregatedOutput string                     `json:"aggregated_output"`
	FormattedOutput  string                     `json:"formatted_output"`
	ExitCode         *int                       `json:"exit_code"`
	Status           string                     `json:"status"`
	Success          bool                       `json:"success"`
	Changes          map[string]json.RawMessage `json:"changes"`
	UnifiedDiff      string                     `json:"unified_diff"`
	Invocation       json.RawMessage            `json:"invocation"`
	Result           json.RawMessage            `json:"result"`
	Images           []string                   `json:"images"`
	LocalImages      []string                   `json:"local_images"`
	Audio            []string                   `json:"audio"`
	LocalAudio       []string                   `json:"local_audio"`
	Info             *codexTokenUsageInfo       `json:"info"`
}

type codexTokenUsageInfo struct {
	LastTokenUsage codexTokenUsage `json:"last_token_usage"`
}

type codexTokenUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	CacheWriteInputTokens int `json:"cache_write_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

func ingestCodex(records []jsonRecord, raw []byte, options IngestOptions) (Trajectory, error) {
	builder := newTrajectoryBuilder(SourceCodexRollout, raw, options)
	pendingCalls := make(map[string]string)
	seenCalls := make(map[string]struct{})
	for _, record := range records {
		var envelope codexRecordEnvelope
		if err := json.Unmarshal(record.Raw, &envelope); err != nil {
			return Trajectory{}, fmt.Errorf("decode Codex record %d: %w", record.Location.Record, err)
		}
		recordIndex := builder.beginRecord(record.Location, envelope.Type, len(record.Raw))
		if builder.options.CanonicalizationProfile != CanonicalizationProfileV1 {
			if err := validateCodexCallIdentity(builder, recordIndex, envelope, pendingCalls, seenCalls); err != nil {
				return Trajectory{}, err
			}
		}
		if err := ingestCodexRecord(builder, recordIndex, record.Location, envelope, pendingCalls); err != nil {
			return Trajectory{}, err
		}
	}
	builder.trajectory.Report.UnpairedToolCalls = len(pendingCalls)
	return builder.finish()
}

func validateCodexCallIdentity(builder *trajectoryBuilder, recordIndex int, envelope codexRecordEnvelope, pending map[string]string, seen map[string]struct{}) error {
	callID := ""
	path := "/payload/call_id"
	isCall := false
	isResult := false
	switch envelope.Type {
	case "response_item":
		var payload codexResponsePayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return fmt.Errorf("decode Codex response identity: %w", err)
		}
		callID = payload.CallID
		switch payload.Type {
		case "function_call", "custom_tool_call", "tool_search_call", "local_shell_call":
			isCall = true
		case "function_call_output", "custom_tool_call_output", "tool_search_output":
			isResult = true
		}
	case "event_msg":
		var payload codexEventPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return fmt.Errorf("decode Codex event identity: %w", err)
		}
		callID = payload.CallID
		switch payload.Type {
		case "exec_command_begin":
			isCall = true
		case "exec_command_output_delta", "exec_command_end":
			isResult = true
		}
	}
	if !isCall && !isResult {
		return nil
	}
	if isCall {
		_, duplicate := seen[callID]
		if callID == "" || duplicate {
			reason := "Codex tool call has no call identity"
			if duplicate {
				reason = "Codex tool call repeats a call identity"
			}
			if err := builder.addUnsupported(recordIndex, path, reason, len(callID)); err != nil {
				return err
			}
		}
		if callID != "" {
			seen[callID] = struct{}{}
		}
		return nil
	}
	if callID == "" || pending[callID] == "" {
		return builder.addUnsupported(recordIndex, path, "Codex tool result has missing, unknown, or out-of-order call identity", len(callID))
	}
	return nil
}

func ingestCodexRecord(builder *trajectoryBuilder, recordIndex int, location SourceLocation, envelope codexRecordEnvelope, pending map[string]string) error {
	switch envelope.Type {
	case "response_item":
		return ingestCodexResponseItem(builder, recordIndex, location, envelope.Timestamp, envelope.Payload, pending)
	case "event_msg":
		return ingestCodexEvent(builder, recordIndex, location, envelope.Timestamp, envelope.Payload, pending)
	case "session_meta", "turn_context", "world_state", "compacted", "inter_agent_communication", "inter_agent_communication_metadata":
		builder.addEvent(recordIndex, Event{
			Kind: EventMetadata, Source: location, Timestamp: envelope.Timestamp, Sensitivity: SensitivityPrivate,
			Metadata: &MetadataPayload{Name: "codex." + envelope.Type, ValueDigest: digestBytes(envelope.Payload), Present: true},
		}, len(envelope.Payload), 0)
		return nil
	default:
		return builder.addUnsupported(recordIndex, "/type", "unknown Codex rollout record type", len(envelope.Type))
	}
}

func ingestCodexResponseItem(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, raw json.RawMessage, pending map[string]string) error {
	var payload codexResponsePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode Codex response item at record %d: %w", location.Record, err)
	}
	location.JSONPointer = "/payload"
	switch payload.Type {
	case "message", "agent_message":
		return ingestCodexMessage(builder, recordIndex, location, timestamp, payload)
	case "reasoning":
		bytes := len(payload.Content) + len(payload.Summary) + len(payload.EncryptedContent)
		eventID := builder.addEvent(recordIndex, Event{
			Kind: EventReasoning, Source: location, SourceEventID: payload.ID, Timestamp: timestamp,
			Sensitivity: SensitivityRestrictedReasoning,
			Reasoning:   &ReasoningPayload{Present: bytes > 0, Omitted: true, Encrypted: payload.EncryptedContent != ""},
		}, bytes, 0)
		builder.addField(recordIndex, FieldAccounting{
			Path: "/payload/reasoning", Disposition: DispositionOmittedSensitive,
			OriginalBytes: bytes, Reason: "private reasoning content is never verifier evidence", EventIDs: []string{eventID},
		})
		return nil
	case "function_call", "custom_tool_call", "tool_search_call":
		return ingestCodexToolCall(builder, recordIndex, location, timestamp, payload, pending)
	case "function_call_output", "custom_tool_call_output", "tool_search_output":
		return ingestCodexToolResult(builder, recordIndex, location, timestamp, payload, pending)
	case "local_shell_call":
		return ingestCodexLocalShell(builder, recordIndex, location, timestamp, payload, pending)
	case "web_search_call":
		return ingestCodexWebSearch(builder, recordIndex, location, timestamp, payload)
	case "image_generation_call":
		return ingestCodexImageGeneration(builder, recordIndex, location, timestamp, payload)
	case "compaction", "compaction_summary", "context_compaction", "compaction_trigger", "additional_tools":
		builder.addEvent(recordIndex, Event{
			Kind: EventMetadata, Source: location, SourceEventID: payload.ID, Timestamp: timestamp, Sensitivity: SensitivityPrivate,
			Metadata: &MetadataPayload{Name: "codex.response_item." + payload.Type, ValueDigest: digestBytes(raw), Present: true},
		}, len(raw), 0)
		return nil
	default:
		return builder.addUnsupported(recordIndex, "/payload/type", "unknown Codex response item", len(raw))
	}
}

func ingestCodexMessage(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexResponsePayload) error {
	var rawParts []json.RawMessage
	if err := json.Unmarshal(payload.Content, &rawParts); err != nil {
		return fmt.Errorf("decode Codex message content at record %d: %w", location.Record, err)
	}
	parts := make([]ContentPart, 0, len(rawParts))
	attachments := make([]Event, 0)
	for index, rawPart := range rawParts {
		var part codexContentPart
		if err := json.Unmarshal(rawPart, &part); err != nil {
			return fmt.Errorf("decode Codex message part %d at record %d: %w", index, location.Record, err)
		}
		partLocation := location
		partLocation.Part = index + 1
		partLocation.JSONPointer = "/payload/content/" + strconv.Itoa(index)
		switch part.Type {
		case "input_text", "output_text":
			text := builder.sanitize(recordIndex, partLocation.JSONPointer+"/text", part.Text)
			parts = append(parts, ContentPart{Kind: ContentText, Text: text})
		case "input_image", "input_audio":
			rawMedia := part.ImageURL
			kind := ContentImage
			mediaType := "image"
			if part.Type == "input_audio" {
				rawMedia = part.AudioURL
				kind = ContentAudio
				mediaType = "audio"
			}
			event := builder.prepareEvent(Event{
				Kind: EventAttachment, Source: partLocation, Timestamp: timestamp, Sensitivity: builder.options.DefaultSensitivity,
				Attachment: &AttachmentPayload{MediaType: mediaType, SizeBytes: len(rawMedia), Digest: digestString(rawMedia), Availability: "reference_only"},
			}, len(rawMedia), len(mediaType)+64)
			parts = append(parts, ContentPart{Kind: kind, Digest: event.Attachment.Digest, SizeBytes: len(rawMedia), Availability: "reference_only", EventID: event.ID})
			attachments = append(attachments, event)
		case "encrypted_content":
			event := builder.prepareEvent(Event{
				Kind: EventMetadata, Source: partLocation, Timestamp: timestamp, Sensitivity: SensitivityRestrictedReasoning,
				Metadata: &MetadataPayload{Name: "codex.encrypted_message_content", ValueDigest: digestString(part.EncryptedContent), Present: true},
			}, len(part.EncryptedContent), 0)
			parts = append(parts, ContentPart{Kind: ContentEventReference, EventID: event.ID})
			attachments = append(attachments, event)
		default:
			return builder.addUnsupported(recordIndex, partLocation.JSONPointer+"/type", "unknown Codex message content part", len(rawPart))
		}
	}
	role := payload.Role
	if payload.Type == "agent_message" {
		role = "assistant"
	}
	message := builder.prepareEvent(Event{
		Kind: EventMessage, Source: location, SourceEventID: payload.ID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		Message:     &MessagePayload{Role: role, Parts: parts, Phase: payload.Phase},
	}, len(payload.Content), contentPartsBytes(parts))
	messageID := builder.appendPreparedEvent(recordIndex, message)
	for _, attachment := range attachments {
		attachmentID := builder.appendPreparedEvent(recordIndex, attachment)
		builder.addLink(LinkParent, messageID, attachmentID)
	}
	return nil
}

func ingestCodexToolCall(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexResponsePayload, pending map[string]string) error {
	arguments := rawStringOrCompact(payload.Arguments)
	if payload.Type == "custom_tool_call" {
		arguments = payload.Input
	}
	if payload.Type == "tool_search_call" && arguments == "" {
		arguments = compactJSON(payload.Action)
	}
	arguments = builder.sanitize(recordIndex, "/payload/arguments", arguments)
	toolName := payload.Name
	if payload.Namespace != "" {
		toolName = payload.Namespace + "." + toolName
	}
	if toolName == "" {
		toolName = payload.Type
	}
	callID := builder.addEvent(recordIndex, Event{
		Kind: EventToolCall, Source: location, SourceEventID: payload.ID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolCall:    &ToolCallPayload{CallID: payload.CallID, ToolName: toolName, Arguments: arguments, ArgumentsDigest: digestString(arguments), Status: payload.Status},
	}, len(arguments)+len(toolName), len(arguments)+len(toolName))
	if payload.CallID != "" {
		pending[payload.CallID] = callID
	}
	if command := commandFromArguments(json.RawMessage(arguments)); command != "" {
		command = builder.sanitize(recordIndex, "/payload/arguments/command", command)
		commandID := builder.addEvent(recordIndex, Event{
			Kind: EventCommand, Source: SourceLocation{Record: location.Record, Line: location.Line, Part: 1, JSONPointer: "/payload/arguments/command"},
			Timestamp: timestamp, Sensitivity: builder.options.DefaultSensitivity,
			Command: builder.commandPayloadFromShell(command, ""),
		}, len(command), len(command))
		builder.addLink(LinkParent, callID, commandID)
	}
	return nil
}

func ingestCodexToolResult(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexResponsePayload, pending map[string]string) error {
	parts, retained, errorResult, err := codexOutputParts(builder, recordIndex, payload.Output)
	if err != nil {
		return fmt.Errorf("decode Codex tool output at record %d: %w", location.Record, err)
	}
	status := payload.Status
	if status == "" {
		status = "completed"
	}
	resultID := builder.addEvent(recordIndex, Event{
		Kind: EventToolResult, Source: location, SourceEventID: payload.ID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolResult:  &ToolResultPayload{CallID: payload.CallID, Status: status, Error: errorResult, Output: parts},
	}, len(payload.Output), retained)
	if callID := pending[payload.CallID]; callID != "" {
		builder.addLink(LinkCallResult, callID, resultID)
		delete(pending, payload.CallID)
	} else {
		builder.trajectory.Report.UnpairedToolResults++
	}
	return nil
}

func codexOutputParts(builder *trajectoryBuilder, recordIndex int, raw json.RawMessage) ([]ContentPart, int, bool, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		cleaned := builder.sanitize(recordIndex, "/payload/output", text)
		return []ContentPart{{Kind: ContentText, Text: cleaned}}, len(cleaned), false, nil
	}
	var rawParts []json.RawMessage
	if err := json.Unmarshal(raw, &rawParts); err != nil {
		return nil, 0, false, err
	}
	parts := make([]ContentPart, 0, len(rawParts))
	retained := 0
	for index, rawPart := range rawParts {
		var part codexContentPart
		if err := json.Unmarshal(rawPart, &part); err != nil {
			return nil, 0, false, err
		}
		switch part.Type {
		case "input_text":
			text := builder.sanitize(recordIndex, "/payload/output/"+strconv.Itoa(index)+"/text", part.Text)
			parts = append(parts, ContentPart{Kind: ContentText, Text: text})
			retained += len(text)
		case "input_image", "input_audio":
			value := part.ImageURL
			kind := ContentImage
			if part.Type == "input_audio" {
				value = part.AudioURL
				kind = ContentAudio
			}
			parts = append(parts, ContentPart{Kind: kind, Digest: digestString(value), SizeBytes: len(value), Availability: "reference_only"})
			retained += 64
		case "encrypted_content":
			parts = append(parts, ContentPart{Kind: ContentEventReference, Digest: digestString(part.EncryptedContent), Availability: "omitted_sensitive"})
		default:
			return nil, 0, false, fmt.Errorf("unknown output content part %q", part.Type)
		}
	}
	return parts, retained, false, nil
}

func ingestCodexLocalShell(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexResponsePayload, pending map[string]string) error {
	var action struct {
		Type             string   `json:"type"`
		Command          []string `json:"command"`
		WorkingDirectory string   `json:"working_directory"`
	}
	if err := json.Unmarshal(payload.Action, &action); err != nil {
		return fmt.Errorf("decode Codex local shell action at record %d: %w", location.Record, err)
	}
	display := builder.sanitize(recordIndex, "/payload/action/command", strings.Join(action.Command, " "))
	callID := builder.addEvent(recordIndex, Event{
		Kind: EventToolCall, Source: location, SourceEventID: payload.ID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolCall:    &ToolCallPayload{CallID: payload.CallID, ToolName: "local_shell", ArgumentsDigest: digestBytes(payload.Action), Status: payload.Status},
	}, len(payload.Action), len(display))
	commandID := builder.addEvent(recordIndex, Event{
		Kind: EventCommand, Source: SourceLocation{Record: location.Record, Line: location.Line, Part: 1, JSONPointer: "/payload/action"},
		Timestamp: timestamp, Sensitivity: builder.options.DefaultSensitivity,
		Command: builder.commandPayloadFromArgv(display, action.Command, aliasValue("cwd", action.WorkingDirectory)),
	}, len(payload.Action), len(display))
	builder.addLink(LinkParent, callID, commandID)
	if payload.CallID != "" {
		pending[payload.CallID] = callID
	}
	return nil
}

func ingestCodexWebSearch(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexResponsePayload) error {
	arguments := builder.sanitize(recordIndex, "/payload/action", compactJSON(payload.Action))
	builder.addEvent(recordIndex, Event{
		Kind: EventToolCall, Source: location, SourceEventID: payload.ID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolCall:    &ToolCallPayload{ToolName: "web_search", Arguments: arguments, ArgumentsDigest: digestString(arguments), Status: payload.Status},
	}, len(payload.Action), len(arguments))
	return nil
}

func ingestCodexImageGeneration(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexResponsePayload) error {
	digest := digestString(payload.Result)
	builder.addEvent(recordIndex, Event{
		Kind: EventAttachment, Source: location, SourceEventID: payload.ID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		Attachment:  &AttachmentPayload{MediaType: "generated_image", SizeBytes: len(payload.Result), Digest: digest, Availability: "metadata_only"},
	}, len(payload.Result)+len(payload.RevisedPrompt), len(digest))
	return nil
}

func ingestCodexEvent(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, raw json.RawMessage, pending map[string]string) error {
	var payload codexEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode Codex event at record %d: %w", location.Record, err)
	}
	location.JSONPointer = "/payload"
	if !knownCodexEventType(payload.Type) {
		return builder.addUnsupported(recordIndex, "/payload/type", "unknown Codex event message", len(raw))
	}
	switch payload.Type {
	case "error", "stream_error", "warning", "guardian_warning":
		message := builder.sanitize(recordIndex, "/payload/message", payload.Message)
		builder.addEvent(recordIndex, Event{
			Kind: EventError, Source: location, Timestamp: timestamp, Sensitivity: builder.options.DefaultSensitivity,
			Error: &ErrorPayload{Class: payload.Type, SafeMessage: message},
		}, len(payload.Message), len(message))
	case "exec_command_begin":
		return ingestCodexExecBegin(builder, recordIndex, location, timestamp, payload, pending)
	case "exec_command_output_delta":
		return ingestCodexExecDelta(builder, recordIndex, location, timestamp, payload, pending)
	case "exec_command_end":
		return ingestCodexExecEnd(builder, recordIndex, location, timestamp, payload, pending)
	case "patch_apply_begin", "patch_apply_updated", "patch_apply_end", "turn_diff":
		return ingestCodexPatch(builder, recordIndex, location, timestamp, payload, pending)
	case "user_message", "agent_message":
		builder.addEvent(recordIndex, Event{
			Kind: EventMetadata, Source: location, Timestamp: timestamp, Sensitivity: SensitivityPrivate,
			Metadata: &MetadataPayload{Name: "codex.legacy_event." + payload.Type, ValueDigest: digestString(payload.Message), Present: true},
		}, len(payload.Message), 0)
	case "agent_reasoning", "agent_reasoning_raw_content", "reasoning_content_delta", "reasoning_raw_content_delta", "agent_reasoning_section_break":
		builder.addEvent(recordIndex, Event{
			Kind: EventReasoning, Source: location, Timestamp: timestamp, Sensitivity: SensitivityRestrictedReasoning,
			Reasoning: &ReasoningPayload{Present: true, Omitted: true},
		}, len(raw), 0)
	case "token_count":
		if payload.Info != nil {
			usage := payload.Info.LastTokenUsage
			builder.addProviderUsage(ProviderTokenUsage{
				Provider: "openai", Scope: "turn", InputTokens: usage.InputTokens,
				Source:       location,
				OutputTokens: usage.OutputTokens, ReasoningTokens: usage.ReasoningOutputTokens,
				CachedInputTokens:        usage.CachedInputTokens,
				CacheCreationInputTokens: usage.CacheWriteInputTokens, TotalTokens: usage.TotalTokens,
			})
		}
		builder.addEvent(recordIndex, Event{
			Kind: EventMetadata, Source: location, Timestamp: timestamp, Sensitivity: SensitivityPrivate,
			Metadata: &MetadataPayload{Name: "codex.event.token_count", ValueDigest: digestBytes(raw), Present: true},
		}, len(raw), 0)
	default:
		builder.addEvent(recordIndex, Event{
			Kind: EventMetadata, Source: location, Timestamp: timestamp, Sensitivity: SensitivityPrivate,
			Metadata: &MetadataPayload{Name: "codex.event." + payload.Type, ValueDigest: digestBytes(raw), Present: true},
		}, len(raw), 0)
	}
	return nil
}

func ingestCodexExecBegin(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexEventPayload, pending map[string]string) error {
	display := builder.sanitize(recordIndex, "/payload/command", strings.Join(payload.Command, " "))
	callID := builder.addEvent(recordIndex, Event{
		Kind: EventToolCall, Source: location, SourceEventID: payload.CallID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolCall:    &ToolCallPayload{CallID: payload.CallID, ToolName: "exec_command", ArgumentsDigest: digestString(display), Status: "running"},
	}, len(display), len(display))
	commandID := builder.addEvent(recordIndex, Event{
		Kind: EventCommand, Source: SourceLocation{Record: location.Record, Line: location.Line, Part: 1, JSONPointer: "/payload/command"},
		Timestamp: timestamp, Sensitivity: builder.options.DefaultSensitivity,
		Command: builder.commandPayloadFromArgv(display, payload.Command, aliasValue("cwd", payload.CWD)),
	}, len(display), len(display))
	builder.addLink(LinkParent, callID, commandID)
	pending[payload.CallID] = callID
	return nil
}

func ingestCodexExecDelta(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexEventPayload, pending map[string]string) error {
	decoded, err := base64.StdEncoding.DecodeString(payload.Chunk)
	if err != nil {
		return fmt.Errorf("decode Codex command output chunk at record %d: %w", location.Record, err)
	}
	text := builder.sanitize(recordIndex, "/payload/chunk", string(decoded))
	outputID := builder.addEvent(recordIndex, Event{
		Kind: EventOutput, Source: location, Timestamp: timestamp, Sensitivity: builder.options.DefaultSensitivity,
		Output: &OutputPayload{Stream: payload.Stream, Text: text, Status: "streaming"},
	}, len(decoded), len(text))
	if callID := pending[payload.CallID]; callID != "" {
		builder.addLink(LinkParent, callID, outputID)
	} else {
		builder.trajectory.Report.UnpairedToolResults++
	}
	return nil
}

func ingestCodexExecEnd(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexEventPayload, pending map[string]string) error {
	if builder.options.CanonicalizationProfile == CanonicalizationProfileV1 {
		return ingestCodexExecEndV1(builder, recordIndex, location, timestamp, payload, pending)
	}
	output := payload.FormattedOutput
	outputPath := "/payload/formatted_output"
	if output == "" {
		output = payload.AggregatedOutput
		outputPath = "/payload/aggregated_output"
	}
	output = builder.sanitize(recordIndex, outputPath, output)
	stdout := builder.sanitize(recordIndex, "/payload/stdout", payload.Stdout)
	stderr := builder.sanitize(recordIndex, "/payload/stderr", payload.Stderr)
	errorResult := payload.ExitCode != nil && *payload.ExitCode != 0
	resultOutput := textContent(output)
	stdoutOutput := textContent(stdout)
	stderrOutput := textContent(stderr)
	resultID := builder.addEvent(recordIndex, Event{
		Kind: EventToolResult, Source: location, SourceEventID: payload.CallID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolResult: &ToolResultPayload{
			CallID: payload.CallID, Status: payload.Status, Error: errorResult, ExitCode: cloneInt(payload.ExitCode),
			Stdout: stdoutOutput, Stderr: stderrOutput, Output: resultOutput,
		},
	}, len(payload.FormattedOutput)+len(payload.AggregatedOutput)+len(payload.Stdout)+len(payload.Stderr), len(output)+len(stdout)+len(stderr))
	if callID := pending[payload.CallID]; callID != "" {
		builder.addLink(LinkCallResult, callID, resultID)
		delete(pending, payload.CallID)
	} else {
		builder.trajectory.Report.UnpairedToolResults++
	}
	return nil
}

func ingestCodexExecEndV1(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexEventPayload, pending map[string]string) error {
	text := payload.FormattedOutput
	if text == "" {
		text = payload.AggregatedOutput
	}
	if text == "" {
		text = payload.Stdout + payload.Stderr
	}
	text = builder.sanitize(recordIndex, "/payload/formatted_output", text)
	exitCode := 0
	if payload.ExitCode != nil {
		exitCode = *payload.ExitCode
	}
	resultID := builder.addEvent(recordIndex, Event{
		Kind: EventToolResult, Source: location, SourceEventID: payload.CallID, Timestamp: timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolResult:  &ToolResultPayload{CallID: payload.CallID, Status: payload.Status, Error: exitCode != 0, Output: []ContentPart{{Kind: ContentText, Text: text}}},
	}, len(payload.FormattedOutput)+len(payload.AggregatedOutput)+len(payload.Stdout)+len(payload.Stderr), len(text))
	if callID := pending[payload.CallID]; callID != "" {
		builder.addLink(LinkCallResult, callID, resultID)
		delete(pending, payload.CallID)
	} else {
		builder.trajectory.Report.UnpairedToolResults++
	}
	return nil
}

func textContent(value string) []ContentPart {
	if value == "" {
		return nil
	}
	return []ContentPart{{Kind: ContentText, Text: value}}
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func ingestCodexPatch(builder *trajectoryBuilder, recordIndex int, location SourceLocation, timestamp string, payload codexEventPayload, pending map[string]string) error {
	if payload.Type == "turn_diff" {
		diff := builder.sanitize(recordIndex, "/payload/unified_diff", payload.UnifiedDiff)
		changeID := builder.addEvent(recordIndex, Event{
			Kind: EventFileChange, Source: location, Timestamp: timestamp, Sensitivity: builder.options.DefaultSensitivity,
			FileChange: &FileChangePayload{Operation: "turn_diff", DiffDigest: digestString(diff), SizeBytes: len(diff)},
		}, len(payload.UnifiedDiff), 64)
		if callID := pending[payload.CallID]; callID != "" {
			builder.addLink(LinkFileChange, callID, changeID)
		}
		return nil
	}
	paths := make([]string, 0, len(payload.Changes))
	for path := range payload.Changes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for index, path := range paths {
		change := payload.Changes[path]
		changeLocation := location
		changeLocation.Part = index + 1
		changeLocation.JSONPointer = "/payload/changes/" + strconv.Itoa(index)
		changeID := builder.addEvent(recordIndex, Event{
			Kind: EventFileChange, Source: changeLocation, SourceEventID: payload.CallID, Timestamp: timestamp,
			Sensitivity: builder.options.DefaultSensitivity,
			FileChange:  &FileChangePayload{Operation: payload.Type, PathAlias: aliasValue("path", path), ContentDigest: digestBytes(change), SizeBytes: len(change)},
		}, len(change), 64)
		if callID := pending[payload.CallID]; callID != "" {
			builder.addLink(LinkFileChange, callID, changeID)
		}
	}
	if len(paths) == 0 {
		builder.addEvent(recordIndex, Event{
			Kind: EventMetadata, Source: location, SourceEventID: payload.CallID, Timestamp: timestamp, Sensitivity: SensitivityPrivate,
			Metadata: &MetadataPayload{Name: "codex.event." + payload.Type, ValueDigest: digestString(payload.Stdout + payload.Stderr), Present: true},
		}, len(payload.Stdout)+len(payload.Stderr), 0)
	}
	return nil
}

func knownCodexEventType(value string) bool {
	_, ok := codexEventTypes[value]
	return ok
}

func rawStringOrCompact(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return compactJSON(raw)
}

var codexEventTypes = map[string]struct{}{
	"error": {}, "warning": {}, "guardian_warning": {}, "realtime_conversation_started": {},
	"realtime_conversation_realtime": {}, "realtime_conversation_closed": {}, "realtime_conversation_sdp": {},
	"model_reroute": {}, "model_verification": {}, "turn_moderation_metadata": {}, "safety_buffering": {},
	"context_compacted": {}, "thread_rolled_back": {}, "task_started": {}, "turn_started": {},
	"thread_settings_applied": {}, "task_complete": {}, "turn_complete": {}, "token_count": {},
	"agent_message": {}, "user_message": {}, "agent_reasoning": {}, "agent_reasoning_raw_content": {},
	"agent_reasoning_section_break": {}, "session_configured": {}, "environment_connected": {},
	"environment_disconnected": {}, "thread_goal_updated": {}, "thread_queue_changed": {},
	"mcp_startup_update": {}, "mcp_startup_complete": {}, "mcp_tool_call_begin": {}, "mcp_tool_call_end": {},
	"web_search_begin": {}, "web_search_end": {}, "image_generation_begin": {}, "image_generation_end": {},
	"exec_command_begin": {}, "exec_command_output_delta": {}, "terminal_interaction": {}, "exec_command_end": {},
	"view_image_tool_call": {}, "exec_approval_request": {}, "request_permissions": {}, "request_user_input": {},
	"dynamic_tool_call_request": {}, "dynamic_tool_call_response": {}, "elicitation_request": {},
	"apply_patch_approval_request": {}, "guardian_assessment": {}, "deprecation_notice": {}, "stream_error": {},
	"patch_apply_begin": {}, "patch_apply_updated": {}, "patch_apply_end": {}, "turn_diff": {},
	"realtime_conversation_list_voices_response": {}, "plan_update": {}, "turn_aborted": {},
	"entered_review_mode": {}, "exited_review_mode": {}, "raw_response_item": {}, "raw_response_completed": {},
	"item_started": {}, "item_completed": {}, "hook_started": {}, "hook_completed": {},
	"agent_message_content_delta": {}, "plan_delta": {}, "reasoning_content_delta": {}, "reasoning_raw_content_delta": {},
	"collab_agent_spawn_begin": {}, "collab_agent_spawn_end": {}, "collab_agent_interaction_begin": {},
	"collab_agent_interaction_end": {}, "collab_waiting_begin": {}, "collab_waiting_end": {},
	"collab_close_begin": {}, "collab_close_end": {}, "collab_resume_begin": {}, "collab_resume_end": {},
	"sub_agent_activity": {},
}
