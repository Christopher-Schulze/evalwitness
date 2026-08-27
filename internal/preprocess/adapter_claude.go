package preprocess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type claudeRecord struct {
	Type            string          `json:"type"`
	UUID            string          `json:"uuid"`
	ParentUUID      string          `json:"parentUuid"`
	Timestamp       string          `json:"timestamp"`
	Subtype         string          `json:"subtype"`
	Message         json.RawMessage `json:"message"`
	Attachment      json.RawMessage `json:"attachment"`
	Operation       string          `json:"operation"`
	Content         json.RawMessage `json:"content"`
	Snapshot        json.RawMessage `json:"snapshot"`
	Backup          json.RawMessage `json:"backup"`
	TrackingPath    string          `json:"trackingPath"`
	PermissionMode  string          `json:"permissionMode"`
	Mode            string          `json:"mode"`
	LastPrompt      string          `json:"lastPrompt"`
	AITitle         string          `json:"aiTitle"`
	BridgeSessionID string          `json:"bridgeSessionId"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Usage   claudeUsage     `json:"usage"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type claudePart struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
	Source    json.RawMessage `json:"source"`
}

type preparedClaudeEvent struct {
	event         Event
	recordLink    LinkKind
	callParent    string
	eventParentID string
}

func ingestClaude(records []jsonRecord, raw []byte, options IngestOptions) (Trajectory, error) {
	builder := newTrajectoryBuilder(SourceClaudeCode, raw, options)
	messageBySourceID := make(map[string]string)
	pendingParents := make([]Link, 0)
	pendingCalls := make(map[string]string)
	seenCalls := make(map[string]struct{})
	for _, record := range records {
		var decoded claudeRecord
		if err := json.Unmarshal(record.Raw, &decoded); err != nil {
			return Trajectory{}, fmt.Errorf("decode Claude record %d: %w", record.Location.Record, err)
		}
		recordIndex := builder.beginRecord(record.Location, decoded.Type, len(record.Raw))
		if err := ingestClaudeRecord(builder, recordIndex, record.Location, decoded, pendingCalls, seenCalls, messageBySourceID, &pendingParents); err != nil {
			return Trajectory{}, err
		}
	}
	for _, link := range pendingParents {
		fromID := messageBySourceID[link.FromID]
		if fromID != "" {
			builder.addLink(LinkParent, fromID, link.ToID)
		}
	}
	builder.trajectory.Report.UnpairedToolCalls = len(pendingCalls)
	return builder.finish()
}

func ingestClaudeRecord(
	builder *trajectoryBuilder,
	recordIndex int,
	location SourceLocation,
	record claudeRecord,
	pendingCalls map[string]string,
	seenCalls map[string]struct{},
	messages map[string]string,
	pendingParents *[]Link,
) error {
	switch record.Type {
	case "assistant", "user":
		return ingestClaudeMessage(builder, recordIndex, location, record, pendingCalls, seenCalls, messages, pendingParents)
	case "attachment":
		return ingestClaudeAttachment(builder, recordIndex, location, record)
	case "file-history-snapshot", "file-history-delta":
		ingestClaudeFileHistory(builder, recordIndex, location, record)
		return nil
	case "system", "queue-operation", "last-prompt", "permission-mode", "mode", "bridge-session", "ai-title":
		ingestClaudeMetadata(builder, recordIndex, location, record)
		return nil
	default:
		return builder.addUnsupported(recordIndex, "/type", "unknown Claude Code record type", len(record.Type))
	}
}

func ingestClaudeMessage(
	builder *trajectoryBuilder,
	recordIndex int,
	location SourceLocation,
	record claudeRecord,
	pendingCalls map[string]string,
	seenCalls map[string]struct{},
	messages map[string]string,
	pendingParents *[]Link,
) error {
	var message claudeMessage
	if err := json.Unmarshal(record.Message, &message); err != nil {
		return fmt.Errorf("decode Claude message record %d: %w", location.Record, err)
	}
	if message.Role == "" {
		message.Role = record.Type
	}
	parts, children, err := parseClaudeParts(builder, recordIndex, location, message, pendingCalls)
	if err != nil {
		return err
	}
	messageLocation := location
	messageLocation.JSONPointer = "/message"
	messageEvent := builder.prepareEvent(Event{
		Kind: EventMessage, Source: messageLocation, SourceEventID: record.UUID,
		Timestamp: record.Timestamp, Sensitivity: builder.options.DefaultSensitivity,
		Message: &MessagePayload{Role: message.Role, Parts: parts},
	}, len(message.Content), contentPartsBytes(parts))
	messageID := builder.appendPreparedEvent(recordIndex, messageEvent)
	if record.UUID != "" {
		messages[record.UUID] = messageID
	}
	builder.addProviderUsage(ProviderTokenUsage{
		Provider: "anthropic", Scope: "message", SourceEventID: record.UUID,
		Source:      location,
		InputTokens: message.Usage.InputTokens, OutputTokens: message.Usage.OutputTokens,
		CachedInputTokens:        message.Usage.CacheReadInputTokens,
		CacheCreationInputTokens: message.Usage.CacheCreationInputTokens,
	})
	if record.ParentUUID != "" {
		*pendingParents = append(*pendingParents, Link{FromID: record.ParentUUID, ToID: messageID})
	}
	for _, child := range children {
		if err := validateClaudeCallIdentity(builder, recordIndex, child, pendingCalls, seenCalls); err != nil {
			return err
		}
		childID := builder.appendPreparedEvent(recordIndex, child.event)
		builder.addLink(child.recordLink, messageID, childID)
		if child.eventParentID != "" {
			builder.addLink(LinkParent, child.eventParentID, childID)
		}
		if child.callParent != "" {
			if callID := pendingCalls[child.callParent]; callID != "" {
				builder.addLink(LinkCallResult, callID, childID)
				delete(pendingCalls, child.callParent)
			} else {
				builder.trajectory.Report.UnpairedToolResults++
			}
		}
		if child.event.ToolCall != nil && child.event.ToolCall.CallID != "" {
			pendingCalls[child.event.ToolCall.CallID] = childID
		}
	}
	builder.addField(recordIndex, FieldAccounting{
		Path: "/message", Disposition: DispositionRepresented,
		OriginalBytes: len(record.Message), RetainedBytes: contentPartsBytes(parts), EventIDs: []string{messageID},
	})
	return nil
}

func validateClaudeCallIdentity(builder *trajectoryBuilder, recordIndex int, child preparedClaudeEvent, pendingCalls map[string]string, seenCalls map[string]struct{}) error {
	if builder.options.CanonicalizationProfile == CanonicalizationProfileV1 {
		return nil
	}
	if child.callParent != "" {
		if callID := pendingCalls[child.callParent]; callID == "" {
			return builder.addUnsupported(recordIndex, child.event.Source.JSONPointer+"/tool_use_id", "Claude tool result has missing, unknown, or out-of-order call identity", len(child.callParent))
		}
	} else if child.event.ToolResult != nil {
		return builder.addUnsupported(recordIndex, child.event.Source.JSONPointer+"/tool_use_id", "Claude tool result has no call identity", 0)
	}
	if child.event.ToolCall == nil {
		return nil
	}
	callID := child.event.ToolCall.CallID
	_, duplicate := seenCalls[callID]
	if callID == "" || duplicate {
		reason := "Claude tool call has no call identity"
		if duplicate {
			reason = "Claude tool call repeats a call identity"
		}
		if err := builder.addUnsupported(recordIndex, child.event.Source.JSONPointer+"/id", reason, len(callID)); err != nil {
			return err
		}
	}
	if callID != "" {
		seenCalls[callID] = struct{}{}
	}
	return nil
}

func parseClaudeParts(
	builder *trajectoryBuilder,
	recordIndex int,
	location SourceLocation,
	message claudeMessage,
	pendingCalls map[string]string,
) ([]ContentPart, []preparedClaudeEvent, error) {
	var text string
	if err := json.Unmarshal(message.Content, &text); err == nil {
		cleaned := builder.sanitize(recordIndex, "/message/content", text)
		return []ContentPart{{Kind: ContentText, Text: cleaned}}, nil, nil
	}
	var rawParts []json.RawMessage
	if err := json.Unmarshal(message.Content, &rawParts); err != nil {
		return nil, nil, fmt.Errorf("decode Claude content at record %d: %w", location.Record, err)
	}
	parts := make([]ContentPart, 0, len(rawParts))
	children := make([]preparedClaudeEvent, 0, len(rawParts))
	for index, rawPart := range rawParts {
		partLocation := location
		partLocation.Part = index + 1
		partLocation.JSONPointer = "/message/content/" + strconv.Itoa(index)
		content, childEvents, err := parseClaudePart(builder, recordIndex, partLocation, rawPart, pendingCalls)
		if err != nil {
			return nil, nil, err
		}
		parts = append(parts, content...)
		children = append(children, childEvents...)
	}
	return parts, children, nil
}

func parseClaudePart(
	builder *trajectoryBuilder,
	recordIndex int,
	location SourceLocation,
	raw json.RawMessage,
	pendingCalls map[string]string,
) ([]ContentPart, []preparedClaudeEvent, error) {
	var part claudePart
	if err := json.Unmarshal(raw, &part); err != nil {
		return nil, nil, fmt.Errorf("decode Claude part %s: %w", location.JSONPointer, err)
	}
	switch part.Type {
	case "text":
		text := builder.sanitize(recordIndex, location.JSONPointer+"/text", part.Text)
		return []ContentPart{{Kind: ContentText, Text: text}}, nil, nil
	case "tool_use":
		return claudeToolCall(builder, recordIndex, location, part)
	case "tool_result":
		return claudeToolResult(builder, recordIndex, location, part, pendingCalls)
	case "thinking":
		event := builder.prepareEvent(Event{
			Kind: EventReasoning, Source: location, Sensitivity: SensitivityRestrictedReasoning,
			Reasoning: &ReasoningPayload{Present: true, Omitted: true},
		}, len(part.Thinking), 0)
		builder.addField(recordIndex, FieldAccounting{
			Path: location.JSONPointer + "/thinking", Disposition: DispositionOmittedSensitive,
			OriginalBytes: len(part.Thinking), Reason: "private reasoning content is never verifier evidence", EventIDs: []string{event.ID},
		})
		return []ContentPart{{Kind: ContentEventReference, EventID: event.ID}}, []preparedClaudeEvent{{event: event, recordLink: LinkParent}}, nil
	case "image":
		event := attachmentEvent(builder, location, part.Source, "claude_message_image")
		return []ContentPart{{Kind: ContentEventReference, EventID: event.ID}}, []preparedClaudeEvent{{event: event, recordLink: LinkParent}}, nil
	default:
		return nil, nil, builder.addUnsupported(recordIndex, location.JSONPointer+"/type", "unknown Claude content part", len(raw))
	}
}

func claudeToolCall(builder *trajectoryBuilder, recordIndex int, location SourceLocation, part claudePart) ([]ContentPart, []preparedClaudeEvent, error) {
	arguments := compactJSON(part.Input)
	arguments = builder.sanitize(recordIndex, location.JSONPointer+"/input", arguments)
	call := builder.prepareEvent(Event{
		Kind: EventToolCall, Source: location, SourceEventID: part.ID,
		Sensitivity: builder.options.DefaultSensitivity,
		ToolCall:    &ToolCallPayload{CallID: part.ID, ToolName: part.Name, Arguments: arguments, ArgumentsDigest: digestString(arguments), Status: "requested"},
	}, len(part.Input), len(arguments)+len(part.Name))
	events := []preparedClaudeEvent{{event: call, recordLink: LinkParent}}
	if command := commandFromArguments(part.Input); command != "" {
		command = builder.sanitize(recordIndex, location.JSONPointer+"/input/command", command)
		commandLocation := location
		commandLocation.JSONPointer += "/command"
		commandEvent := builder.prepareEvent(Event{
			Kind: EventCommand, Source: commandLocation, Sensitivity: builder.options.DefaultSensitivity,
			Command: builder.commandPayloadFromShell(command, ""),
		}, len(command), len(command))
		events = append(events, preparedClaudeEvent{event: commandEvent, recordLink: LinkParent, eventParentID: call.ID})
	}
	return []ContentPart{{Kind: ContentEventReference, EventID: call.ID}}, events, nil
}

func claudeToolResult(builder *trajectoryBuilder, recordIndex int, location SourceLocation, part claudePart, _ map[string]string) ([]ContentPart, []preparedClaudeEvent, error) {
	output, outputBytes, err := claudeResultParts(builder, recordIndex, location, part.Content)
	if err != nil {
		return nil, nil, err
	}
	status := "completed"
	if part.IsError {
		status = "error"
	}
	event := builder.prepareEvent(Event{
		Kind: EventToolResult, Source: location, Sensitivity: builder.options.DefaultSensitivity,
		ToolResult: &ToolResultPayload{CallID: part.ToolUseID, Status: status, Error: part.IsError, Output: output},
	}, len(part.Content), outputBytes)
	return []ContentPart{{Kind: ContentEventReference, EventID: event.ID}}, []preparedClaudeEvent{{event: event, recordLink: LinkParent, callParent: part.ToolUseID}}, nil
}

func claudeResultParts(builder *trajectoryBuilder, recordIndex int, location SourceLocation, raw json.RawMessage) ([]ContentPart, int, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		cleaned := builder.sanitize(recordIndex, location.JSONPointer+"/content", text)
		return []ContentPart{{Kind: ContentText, Text: cleaned}}, len(cleaned), nil
	}
	var parts []struct {
		Type     string `json:"type"`
		ToolName string `json:"tool_name"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, 0, fmt.Errorf("decode Claude tool result %s: %w", location.JSONPointer, err)
	}
	output := make([]ContentPart, 0, len(parts))
	bytes := 0
	for _, part := range parts {
		value := ""
		switch part.Type {
		case "tool_reference":
			value = "tool_reference:" + part.ToolName
		case "text":
			value = builder.sanitize(recordIndex, location.JSONPointer+"/content", part.Text)
		default:
			return nil, 0, fmt.Errorf("unsupported Claude tool result part %q at %s", part.Type, location.JSONPointer)
		}
		output = append(output, ContentPart{Kind: ContentText, Text: value})
		bytes += len(value)
	}
	return output, bytes, nil
}

func ingestClaudeAttachment(builder *trajectoryBuilder, recordIndex int, location SourceLocation, record claudeRecord) error {
	var attachment struct {
		Type     string `json:"type"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(record.Attachment, &attachment); err != nil {
		return fmt.Errorf("decode Claude attachment record %d: %w", location.Record, err)
	}
	attachmentLocation := location
	attachmentLocation.JSONPointer = "/attachment"
	event := attachmentEvent(builder, attachmentLocation, record.Attachment, attachment.Type)
	if attachment.Filename != "" {
		event.Attachment.NameAlias = aliasValue("file", attachment.Filename)
		event = builder.prepareEvent(event, len(record.Attachment), event.RetainedBytes+len(event.Attachment.NameAlias))
	}
	eventID := builder.appendPreparedEvent(recordIndex, event)
	builder.addField(recordIndex, FieldAccounting{
		Path: "/attachment", Disposition: DispositionMetadataOnly,
		OriginalBytes: len(record.Attachment), RetainedBytes: event.RetainedBytes, EventIDs: []string{eventID},
		Reason: "attachment payload retained as metadata and digest only",
	})
	return nil
}

func ingestClaudeFileHistory(builder *trajectoryBuilder, recordIndex int, location SourceLocation, record claudeRecord) {
	operation := "snapshot"
	payload := record.Snapshot
	if record.Type == "file-history-delta" {
		operation = "delta"
		payload = record.Backup
	}
	event := Event{
		Kind: EventFileChange, Source: location, Timestamp: record.Timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		FileChange:  &FileChangePayload{Operation: operation, PathAlias: aliasValue("path", record.TrackingPath), ContentDigest: digestBytes(payload), SizeBytes: len(payload)},
	}
	eventID := builder.addEvent(recordIndex, event, builder.trajectory.Report.Records[recordIndex].OriginalBytes, 0)
	builder.addField(recordIndex, FieldAccounting{
		Path: "/", Disposition: DispositionMetadataOnly,
		OriginalBytes: builder.trajectory.Report.Records[recordIndex].OriginalBytes, Reason: "file history content retained by digest", EventIDs: []string{eventID},
	})
}

func ingestClaudeMetadata(builder *trajectoryBuilder, recordIndex int, location SourceLocation, record claudeRecord) {
	name := "claude." + record.Type
	value := ""
	switch record.Type {
	case "system":
		name += "." + record.Subtype
	case "queue-operation":
		name += "." + record.Operation
	case "permission-mode":
		value = record.PermissionMode
	case "mode":
		value = record.Mode
	}
	value = builder.sanitize(recordIndex, "/value", value)
	originalBytes := builder.trajectory.Report.Records[recordIndex].OriginalBytes
	builder.addEvent(recordIndex, Event{
		Kind: EventMetadata, Source: location, Timestamp: record.Timestamp, Sensitivity: SensitivityPrivate,
		Metadata: &MetadataPayload{Name: name, Value: value, ValueDigest: claudeMetadataDigest(record), Present: true},
	}, originalBytes, len(value))
}

func claudeMetadataDigest(record claudeRecord) string {
	material := record.LastPrompt + record.AITitle + record.BridgeSessionID + string(record.Content)
	if material == "" {
		return ""
	}
	return digestString(material)
}

func attachmentEvent(builder *trajectoryBuilder, location SourceLocation, raw json.RawMessage, mediaType string) Event {
	digest := digestBytes(raw)
	event := Event{
		Kind: EventAttachment, Source: location, Sensitivity: builder.options.DefaultSensitivity,
		Attachment: &AttachmentPayload{MediaType: mediaType, SizeBytes: len(raw), Digest: digest, Availability: "metadata_only"},
	}
	return builder.prepareEvent(event, len(raw), len(mediaType)+len(digest))
}

func compactJSON(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	var output bytes.Buffer
	if err := json.Compact(&output, trimmed); err != nil {
		return string(trimmed)
	}
	return output.String()
}

func commandFromArguments(raw json.RawMessage) string {
	var arguments struct {
		Command    json.RawMessage `json:"command"`
		Cmd        json.RawMessage `json:"cmd"`
		Keystrokes json.RawMessage `json:"keystrokes"`
	}
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return ""
	}
	for _, value := range []json.RawMessage{arguments.Command, arguments.Cmd, arguments.Keystrokes} {
		var command string
		if err := json.Unmarshal(value, &command); err == nil && command != "" {
			return command
		}
		var commandParts []string
		if err := json.Unmarshal(value, &commandParts); err == nil && len(commandParts) > 0 {
			return strings.Join(commandParts, " ")
		}
	}
	return ""
}

func aliasValue(prefix, value string) string {
	if value == "" {
		return ""
	}
	return prefix + "_" + digestString(value)[:12]
}

func contentPartsBytes(parts []ContentPart) int {
	total := 0
	for _, part := range parts {
		total += len(part.Text) + len(part.MediaType) + len(part.NameAlias) + len(part.Digest) + len(part.EventID)
	}
	return total
}
