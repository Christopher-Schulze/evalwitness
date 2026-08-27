package preprocess

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type openCodeEnvelope struct {
	Info     json.RawMessage   `json:"info"`
	Messages []json.RawMessage `json:"messages"`
}

type openCodeSessionInfo struct {
	ID       string          `json:"id"`
	Version  string          `json:"version"`
	ParentID string          `json:"parentID"`
	Summary  json.RawMessage `json:"summary"`
	Time     json.RawMessage `json:"time"`
}

type openCodeMessage struct {
	Info  openCodeMessageInfo `json:"info"`
	Parts []json.RawMessage   `json:"parts"`
}

type openCodeMessageInfo struct {
	ID         string          `json:"id"`
	Role       string          `json:"role"`
	ParentID   string          `json:"parentID"`
	ModelID    string          `json:"modelID"`
	ProviderID string          `json:"providerID"`
	Agent      string          `json:"agent"`
	Mode       string          `json:"mode"`
	Time       openCodeTime    `json:"time"`
	Error      json.RawMessage `json:"error"`
	Finish     string          `json:"finish"`
	Phase      string          `json:"phase"`
}

type openCodeTime struct {
	Created   int64 `json:"created"`
	Completed int64 `json:"completed"`
}

type openCodePart struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Text        string          `json:"text"`
	Synthetic   bool            `json:"synthetic"`
	Ignored     bool            `json:"ignored"`
	Mime        string          `json:"mime"`
	Filename    string          `json:"filename"`
	URL         string          `json:"url"`
	Source      json.RawMessage `json:"source"`
	Name        string          `json:"name"`
	Agent       string          `json:"agent"`
	Prompt      string          `json:"prompt"`
	Description string          `json:"description"`
	Command     string          `json:"command"`
	Tool        string          `json:"tool"`
	CallID      string          `json:"callID"`
	State       json.RawMessage `json:"state"`
	Hash        string          `json:"hash"`
	Files       []string        `json:"files"`
	Snapshot    string          `json:"snapshot"`
	Reason      string          `json:"reason"`
	Cost        float64         `json:"cost"`
	Tokens      json.RawMessage `json:"tokens"`
	Attempt     int             `json:"attempt"`
	Error       json.RawMessage `json:"error"`
	Auto        bool            `json:"auto"`
	Overflow    bool            `json:"overflow"`
}

type openCodeToolState struct {
	Status      string            `json:"status"`
	Input       json.RawMessage   `json:"input"`
	Raw         string            `json:"raw"`
	Output      string            `json:"output"`
	Error       string            `json:"error"`
	Title       string            `json:"title"`
	Metadata    json.RawMessage   `json:"metadata"`
	Attachments []json.RawMessage `json:"attachments"`
}

type openCodeTokenUsage struct {
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

type preparedOpenCodeEvent struct {
	event        Event
	linkFromCall string
	linkKind     LinkKind
}

func ingestOpenCode(raw []byte, options IngestOptions) (Trajectory, error) {
	var envelope openCodeEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Trajectory{}, fmt.Errorf("decode OpenCode export: %w", err)
	}
	var session openCodeSessionInfo
	if err := json.Unmarshal(envelope.Info, &session); err != nil {
		return Trajectory{}, fmt.Errorf("decode OpenCode session info: %w", err)
	}
	if session.ID == "" {
		return Trajectory{}, fmt.Errorf("OpenCode export has no session identity")
	}
	builder := newTrajectoryBuilder(SourceOpenCode, raw, options)
	infoLocation := SourceLocation{Record: 1, JSONPointer: "/info"}
	infoRecord := builder.beginRecord(infoLocation, "session_info", len(envelope.Info))
	builder.addEvent(infoRecord, Event{
		Kind: EventMetadata, Source: infoLocation, SourceEventID: session.ID, Sensitivity: SensitivityPrivate,
		Metadata: &MetadataPayload{Name: "opencode.session", Value: session.Version, ValueDigest: digestBytes(envelope.Info), Present: true},
	}, len(envelope.Info), len(session.Version))
	messageIDs := make(map[string]string)
	seenCallIDs := make(map[string]struct{})
	pendingParents := make([]Link, 0, len(envelope.Messages))
	for index, rawMessage := range envelope.Messages {
		location := SourceLocation{Record: index + 2, JSONPointer: "/messages/" + strconv.Itoa(index)}
		recordIndex := builder.beginRecord(location, "message", len(rawMessage))
		if err := ingestOpenCodeMessage(builder, recordIndex, location, rawMessage, messageIDs, seenCallIDs, &pendingParents); err != nil {
			return Trajectory{}, err
		}
	}
	for _, link := range pendingParents {
		if parentID := messageIDs[link.FromID]; parentID != "" {
			builder.addLink(LinkParent, parentID, link.ToID)
		}
	}
	return builder.finish()
}

func ingestOpenCodeMessage(builder *trajectoryBuilder, recordIndex int, location SourceLocation, raw json.RawMessage, messageIDs map[string]string, seenCallIDs map[string]struct{}, parents *[]Link) error {
	var message openCodeMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		return fmt.Errorf("decode OpenCode message %d: %w", location.Record-1, err)
	}
	if builder.options.CanonicalizationProfile != CanonicalizationProfileV1 {
		if err := validateOpenCodeCallIdentities(builder, recordIndex, location, message.Parts, seenCallIDs); err != nil {
			return err
		}
	}
	parts, children, err := parseOpenCodeParts(builder, recordIndex, location, message.Parts)
	if err != nil {
		return err
	}
	messageLocation := location
	messageLocation.JSONPointer += "/info"
	messageEvent := builder.prepareEvent(Event{
		Kind: EventMessage, Source: messageLocation, SourceEventID: message.Info.ID,
		Timestamp: strconv.FormatInt(message.Info.Time.Created, 10), Sensitivity: builder.options.DefaultSensitivity,
		Message: &MessagePayload{Role: message.Info.Role, Parts: parts, Phase: message.Info.Phase},
	}, len(raw), contentPartsBytes(parts))
	messageID := builder.appendPreparedEvent(recordIndex, messageEvent)
	messageIDs[message.Info.ID] = messageID
	if message.Info.ParentID != "" {
		*parents = append(*parents, Link{FromID: message.Info.ParentID, ToID: messageID})
	}
	for _, child := range children {
		childID := builder.appendPreparedEvent(recordIndex, child.event)
		builder.addLink(LinkParent, messageID, childID)
		if child.linkFromCall != "" {
			builder.addLink(child.linkKind, child.linkFromCall, childID)
		}
	}
	if len(message.Info.Error) > 0 && string(message.Info.Error) != "null" {
		errorLocation := location
		errorLocation.JSONPointer += "/info/error"
		errorID := builder.addEvent(recordIndex, Event{
			Kind: EventError, Source: errorLocation, SourceEventID: message.Info.ID,
			Timestamp: strconv.FormatInt(message.Info.Time.Completed, 10), Sensitivity: builder.options.DefaultSensitivity,
			Error: &ErrorPayload{Class: "opencode_assistant_error", SafeMessage: "assistant message ended with an error"},
		}, len(message.Info.Error), len("assistant message ended with an error"))
		builder.addLink(LinkParent, messageID, errorID)
	}
	return nil
}

func validateOpenCodeCallIdentities(builder *trajectoryBuilder, recordIndex int, location SourceLocation, rawParts []json.RawMessage, seen map[string]struct{}) error {
	for index, rawPart := range rawParts {
		var part openCodePart
		if err := json.Unmarshal(rawPart, &part); err != nil {
			return fmt.Errorf("decode OpenCode part identity %d: %w", index, err)
		}
		if part.Type != "tool" {
			continue
		}
		_, duplicate := seen[part.CallID]
		if part.CallID == "" || duplicate {
			reason := "OpenCode tool state has no call identity"
			if duplicate {
				reason = "OpenCode tool state repeats a call identity"
			}
			path := location.JSONPointer + "/parts/" + strconv.Itoa(index) + "/callID"
			if err := builder.addUnsupported(recordIndex, path, reason, len(part.CallID)); err != nil {
				return err
			}
		}
		if part.CallID != "" {
			seen[part.CallID] = struct{}{}
		}
	}
	return nil
}

func parseOpenCodeParts(builder *trajectoryBuilder, recordIndex int, location SourceLocation, rawParts []json.RawMessage) ([]ContentPart, []preparedOpenCodeEvent, error) {
	parts := make([]ContentPart, 0, len(rawParts))
	children := make([]preparedOpenCodeEvent, 0, len(rawParts))
	for index, rawPart := range rawParts {
		partLocation := location
		partLocation.Part = index + 1
		partLocation.JSONPointer += "/parts/" + strconv.Itoa(index)
		content, events, err := parseOpenCodePart(builder, recordIndex, partLocation, rawPart)
		if err != nil {
			return nil, nil, err
		}
		parts = append(parts, content...)
		children = append(children, events...)
	}
	return parts, children, nil
}

func parseOpenCodePart(builder *trajectoryBuilder, recordIndex int, location SourceLocation, raw json.RawMessage) ([]ContentPart, []preparedOpenCodeEvent, error) {
	var part openCodePart
	if err := json.Unmarshal(raw, &part); err != nil {
		return nil, nil, fmt.Errorf("decode OpenCode part %s: %w", location.JSONPointer, err)
	}
	switch part.Type {
	case "text":
		text := builder.sanitize(recordIndex, location.JSONPointer+"/text", part.Text)
		return []ContentPart{{Kind: ContentText, Text: text}}, nil, nil
	case "reasoning":
		event := builder.prepareEvent(Event{
			Kind: EventReasoning, Source: location, SourceEventID: part.ID, Sensitivity: SensitivityRestrictedReasoning,
			Reasoning: &ReasoningPayload{Present: true, Omitted: true},
		}, len(part.Text), 0)
		builder.addField(recordIndex, FieldAccounting{
			Path: location.JSONPointer + "/text", Disposition: DispositionOmittedSensitive,
			OriginalBytes: len(part.Text), Reason: "private reasoning content is never verifier evidence", EventIDs: []string{event.ID},
		})
		return []ContentPart{{Kind: ContentEventReference, EventID: event.ID}}, []preparedOpenCodeEvent{{event: event}}, nil
	case "file":
		event := builder.prepareEvent(Event{
			Kind: EventAttachment, Source: location, SourceEventID: part.ID, Sensitivity: builder.options.DefaultSensitivity,
			Attachment: &AttachmentPayload{MediaType: part.Mime, NameAlias: aliasValue("file", part.Filename), SizeBytes: len(part.URL), Digest: digestString(part.URL + string(part.Source)), Availability: "reference_only"},
		}, len(part.URL)+len(part.Source), len(part.Mime)+64)
		return []ContentPart{{Kind: ContentFile, MediaType: part.Mime, NameAlias: event.Attachment.NameAlias, Digest: event.Attachment.Digest, EventID: event.ID}}, []preparedOpenCodeEvent{{event: event}}, nil
	case "tool":
		return openCodeToolEvents(builder, recordIndex, location, part)
	case "patch":
		return openCodePatchEvents(builder, location, part)
	case "subtask":
		return openCodeSubtaskEvent(builder, recordIndex, location, part)
	case "retry":
		event := builder.prepareEvent(Event{
			Kind: EventError, Source: location, SourceEventID: part.ID, Sensitivity: builder.options.DefaultSensitivity,
			Error: &ErrorPayload{Class: "opencode_retry", SafeMessage: fmt.Sprintf("retry attempt %d", part.Attempt)},
		}, len(part.Error), len(fmt.Sprintf("retry attempt %d", part.Attempt)))
		return []ContentPart{{Kind: ContentEventReference, EventID: event.ID}}, []preparedOpenCodeEvent{{event: event}}, nil
	case "snapshot", "step-start", "step-finish", "agent", "compaction":
		if part.Type == "step-finish" {
			var usage openCodeTokenUsage
			if err := json.Unmarshal(part.Tokens, &usage); err != nil && len(part.Tokens) > 0 && string(part.Tokens) != "null" {
				return nil, nil, fmt.Errorf("decode OpenCode token usage %s: %w", location.JSONPointer, err)
			}
			builder.addProviderUsage(ProviderTokenUsage{
				Provider: "opencode", Scope: "step", SourceEventID: part.ID,
				Source:      location,
				InputTokens: usage.Input, OutputTokens: usage.Output, ReasoningTokens: usage.Reasoning,
				CachedInputTokens: usage.Cache.Read, CacheCreationInputTokens: usage.Cache.Write,
			})
		}
		event := builder.prepareEvent(Event{
			Kind: EventMetadata, Source: location, SourceEventID: part.ID, Sensitivity: SensitivityPrivate,
			Metadata: &MetadataPayload{Name: "opencode.part." + part.Type, ValueDigest: digestBytes(raw), Present: true},
		}, len(raw), 0)
		return []ContentPart{{Kind: ContentEventReference, EventID: event.ID}}, []preparedOpenCodeEvent{{event: event}}, nil
	default:
		return nil, nil, builder.addUnsupported(recordIndex, location.JSONPointer+"/type", "unknown OpenCode part", len(raw))
	}
}

func openCodeToolEvents(builder *trajectoryBuilder, recordIndex int, location SourceLocation, part openCodePart) ([]ContentPart, []preparedOpenCodeEvent, error) {
	var state openCodeToolState
	if err := json.Unmarshal(part.State, &state); err != nil {
		return nil, nil, fmt.Errorf("decode OpenCode tool state %s: %w", location.JSONPointer, err)
	}
	arguments := builder.sanitize(recordIndex, location.JSONPointer+"/state/input", compactJSON(state.Input))
	call := builder.prepareEvent(Event{
		Kind: EventToolCall, Source: location, SourceEventID: part.ID, Sensitivity: builder.options.DefaultSensitivity,
		ToolCall: &ToolCallPayload{CallID: part.CallID, ToolName: part.Tool, Arguments: arguments, ArgumentsDigest: digestString(arguments), Status: state.Status},
	}, len(state.Input), len(arguments)+len(part.Tool))
	events := []preparedOpenCodeEvent{{event: call}}
	if command := commandFromArguments(state.Input); command != "" {
		command = builder.sanitize(recordIndex, location.JSONPointer+"/state/input/command", command)
		commandLocation := location
		commandLocation.JSONPointer += "/state/input/command"
		commandEvent := builder.prepareEvent(Event{
			Kind: EventCommand, Source: commandLocation, Sensitivity: builder.options.DefaultSensitivity,
			Command: builder.commandPayloadFromShell(command, ""),
		}, len(command), len(command))
		events = append(events, preparedOpenCodeEvent{event: commandEvent, linkFromCall: call.ID, linkKind: LinkParent})
	}
	if state.Status == "completed" || state.Status == "error" {
		output := state.Output
		if state.Status == "error" {
			output = state.Error
		}
		output = builder.sanitize(recordIndex, location.JSONPointer+"/state/output", output)
		resultLocation := location
		resultLocation.JSONPointer += "/state/output"
		result := builder.prepareEvent(Event{
			Kind: EventToolResult, Source: resultLocation, SourceEventID: part.ID, Sensitivity: builder.options.DefaultSensitivity,
			ToolResult: &ToolResultPayload{CallID: part.CallID, Status: state.Status, Error: state.Status == "error", Output: []ContentPart{{Kind: ContentText, Text: output}}},
		}, len(state.Output)+len(state.Error), len(output))
		events = append(events, preparedOpenCodeEvent{event: result, linkFromCall: call.ID, linkKind: LinkCallResult})
		for index, rawAttachment := range state.Attachments {
			attachmentLocation := location
			attachmentLocation.Part += index + 1
			attachmentLocation.JSONPointer += "/state/attachments/" + strconv.Itoa(index)
			attachment := attachmentEvent(builder, attachmentLocation, rawAttachment, "tool_result_attachment")
			events = append(events, preparedOpenCodeEvent{event: attachment, linkFromCall: result.ID, linkKind: LinkParent})
		}
	} else {
		builder.trajectory.Report.UnpairedToolCalls++
	}
	return []ContentPart{{Kind: ContentEventReference, EventID: call.ID}}, events, nil
}

func openCodePatchEvents(builder *trajectoryBuilder, location SourceLocation, part openCodePart) ([]ContentPart, []preparedOpenCodeEvent, error) {
	parts := make([]ContentPart, 0, len(part.Files))
	events := make([]preparedOpenCodeEvent, 0, len(part.Files))
	for index, path := range part.Files {
		changeLocation := location
		changeLocation.Part += index
		changeLocation.JSONPointer += "/files/" + strconv.Itoa(index)
		event := builder.prepareEvent(Event{
			Kind: EventFileChange, Source: changeLocation, SourceEventID: part.ID, Sensitivity: builder.options.DefaultSensitivity,
			FileChange: &FileChangePayload{Operation: "patch", PathAlias: aliasValue("path", path), ContentDigest: part.Hash},
		}, len(path)+len(part.Hash), len(part.Hash))
		parts = append(parts, ContentPart{Kind: ContentEventReference, EventID: event.ID})
		events = append(events, preparedOpenCodeEvent{event: event})
	}
	return parts, events, nil
}

func openCodeSubtaskEvent(builder *trajectoryBuilder, recordIndex int, location SourceLocation, part openCodePart) ([]ContentPart, []preparedOpenCodeEvent, error) {
	arguments := part.Prompt + "\n" + part.Description
	if part.Command != "" {
		arguments += "\n" + part.Command
	}
	arguments = builder.sanitize(recordIndex, location.JSONPointer, arguments)
	event := builder.prepareEvent(Event{
		Kind: EventToolCall, Source: location, SourceEventID: part.ID, Sensitivity: builder.options.DefaultSensitivity,
		ToolCall: &ToolCallPayload{ToolName: "subtask:" + part.Agent, Arguments: arguments, ArgumentsDigest: digestString(arguments), Status: "requested"},
	}, len(part.Prompt)+len(part.Description)+len(part.Command), len(arguments))
	return []ContentPart{{Kind: ContentEventReference, EventID: event.ID}}, []preparedOpenCodeEvent{{event: event}}, nil
}
