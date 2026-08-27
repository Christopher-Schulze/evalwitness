package preprocess

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type terminalBenchEnvelopeV1 struct {
	TrialID    string          `json:"trial_id"`
	TrialName  string          `json:"trial_name"`
	TaskName   string          `json:"task_name"`
	Trajectory json.RawMessage `json:"trajectory"`
}

type terminalBenchBodyV1 struct {
	SchemaVersion string            `json:"schema_version"`
	SessionID     string            `json:"session_id"`
	Agent         json.RawMessage   `json:"agent"`
	FinalMetrics  json.RawMessage   `json:"final_metrics"`
	Steps         []json.RawMessage `json:"steps"`
}

type terminalBenchStepV1 struct {
	StepID      json.RawMessage            `json:"step_id"`
	Source      string                     `json:"source"`
	Message     string                     `json:"message"`
	Timestamp   string                     `json:"timestamp"`
	ToolCalls   []terminalBenchToolCallV1  `json:"tool_calls"`
	Observation terminalBenchObservationV1 `json:"observation"`
}

type terminalBenchToolCallV1 struct {
	ID        string          `json:"tool_call_id"`
	Name      string          `json:"function_name"`
	Arguments json.RawMessage `json:"arguments"`
}

type terminalBenchObservationV1 struct {
	Results []struct {
		Content string `json:"content"`
	} `json:"results"`
}

type sweBenchCacheItemV1 struct {
	InstanceID   string `json:"instance_id"`
	Messages     string `json:"messages"`
	ModelName    string `json:"model_name"`
	NumSteps     int    `json:"num_steps"`
	OutputPatch  string `json:"output_patch"`
	TrajectoryID string `json:"trajectory_id"`
}

type sweBenchMessageV1 struct {
	Role       string               `json:"role"`
	Content    json.RawMessage      `json:"content"`
	Name       string               `json:"name"`
	ToolCallID string               `json:"tool_call_id"`
	ToolCalls  []sweBenchToolCallV1 `json:"tool_calls"`
}

type sweBenchToolCallV1 struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func ingestTerminalBench(raw []byte, options IngestOptions) (Trajectory, error) {
	var envelope terminalBenchEnvelopeV1
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Trajectory{}, fmt.Errorf("decode Terminal-Bench trajectory: %w", err)
	}
	var body terminalBenchBodyV1
	if err := json.Unmarshal(envelope.Trajectory, &body); err != nil {
		return Trajectory{}, fmt.Errorf("decode Terminal-Bench trajectory body: %w", err)
	}
	if len(body.Steps) == 0 {
		return Trajectory{}, fmt.Errorf("Terminal-Bench trajectory contains no steps")
	}
	builder := newTrajectoryBuilder(SourceTerminalBench, raw, options)
	metadataLocation := SourceLocation{Record: 1, JSONPointer: "/trajectory"}
	metadataRecord := builder.beginRecord(metadataLocation, "trajectory_metadata", len(envelope.Trajectory))
	builder.addEvent(metadataRecord, Event{
		Kind: EventMetadata, Source: metadataLocation, SourceEventID: body.SessionID, Sensitivity: SensitivityPrivate,
		Metadata: &MetadataPayload{Name: "terminal_bench.trajectory", Value: body.SchemaVersion, ValueDigest: digestString(envelope.TrialID + envelope.TrialName + envelope.TaskName + string(body.Agent) + string(body.FinalMetrics)), Present: true},
	}, len(envelope.Trajectory), len(body.SchemaVersion))
	for index, rawStep := range body.Steps {
		location := SourceLocation{Record: index + 2, JSONPointer: "/trajectory/steps/" + strconv.Itoa(index)}
		recordIndex := builder.beginRecord(location, "step", len(rawStep))
		if err := ingestTerminalBenchStep(builder, recordIndex, location, rawStep); err != nil {
			return Trajectory{}, err
		}
	}
	return builder.finish()
}

func ingestTerminalBenchStep(builder *trajectoryBuilder, recordIndex int, location SourceLocation, raw json.RawMessage) error {
	var step terminalBenchStepV1
	if err := json.Unmarshal(raw, &step); err != nil {
		return fmt.Errorf("decode Terminal-Bench step %d: %w", location.Record-1, err)
	}
	stepID := rawScalarString(step.StepID)
	children := make([]Event, 0, len(step.ToolCalls)*3)
	refs := make([]ContentPart, 0, len(step.ToolCalls)+1)
	message := builder.sanitize(recordIndex, location.JSONPointer+"/message", step.Message)
	if message != "" {
		refs = append(refs, ContentPart{Kind: ContentText, Text: message})
	}
	callIDs := make([]string, 0, len(step.ToolCalls))
	for index, call := range step.ToolCalls {
		callLocation := location
		callLocation.Part = index + 1
		callLocation.JSONPointer += "/tool_calls/" + strconv.Itoa(index)
		arguments := builder.sanitize(recordIndex, callLocation.JSONPointer+"/arguments", compactJSON(call.Arguments))
		callEvent := builder.prepareEvent(Event{
			Kind: EventToolCall, Source: callLocation, SourceEventID: call.ID, Timestamp: step.Timestamp,
			Sensitivity: builder.options.DefaultSensitivity,
			ToolCall:    &ToolCallPayload{CallID: call.ID, ToolName: call.Name, Arguments: arguments, ArgumentsDigest: digestString(arguments), Status: "requested"},
		}, len(call.Arguments), len(arguments)+len(call.Name))
		children = append(children, callEvent)
		callIDs = append(callIDs, callEvent.ID)
		refs = append(refs, ContentPart{Kind: ContentEventReference, EventID: callEvent.ID})
		if command := commandFromArguments(call.Arguments); command != "" {
			command = builder.sanitize(recordIndex, callLocation.JSONPointer+"/arguments/command", command)
			commandLocation := callLocation
			commandLocation.JSONPointer += "/arguments/command"
			children = append(children, builder.prepareEvent(Event{
				Kind: EventCommand, Source: commandLocation, Timestamp: step.Timestamp, Sensitivity: builder.options.DefaultSensitivity,
				Command: builder.commandPayloadFromShell(command, ""),
			}, len(command), len(command)))
		}
	}
	messageLocation := location
	messageLocation.JSONPointer += "/message"
	messageEvent := builder.prepareEvent(Event{
		Kind: EventMessage, Source: messageLocation, SourceEventID: stepID, Timestamp: step.Timestamp,
		Sensitivity: builder.options.DefaultSensitivity,
		Message:     &MessagePayload{Role: normalizeAgentRole(step.Source), Parts: refs},
	}, len(step.Message), contentPartsBytes(refs))
	messageID := builder.appendPreparedEvent(recordIndex, messageEvent)
	for _, child := range children {
		childID := builder.appendPreparedEvent(recordIndex, child)
		builder.addLink(LinkParent, messageID, childID)
		if child.Kind == EventCommand {
			for _, callID := range callIDs {
				if child.Source.Part == eventByID(builder.trajectory.Events, callID).Source.Part {
					builder.addLink(LinkParent, callID, childID)
					break
				}
			}
		}
	}
	for index, result := range step.Observation.Results {
		resultLocation := location
		resultLocation.Part = len(step.ToolCalls) + index + 1
		resultLocation.JSONPointer += "/observation/results/" + strconv.Itoa(index)
		output := builder.sanitize(recordIndex, resultLocation.JSONPointer+"/content", result.Content)
		if index < len(callIDs) {
			resultID := builder.addEvent(recordIndex, Event{
				Kind: EventToolResult, Source: resultLocation, Timestamp: step.Timestamp, Sensitivity: builder.options.DefaultSensitivity,
				ToolResult: &ToolResultPayload{CallID: step.ToolCalls[index].ID, Status: "completed", Output: []ContentPart{{Kind: ContentText, Text: output}}},
			}, len(result.Content), len(output))
			builder.addLink(LinkCallResult, callIDs[index], resultID)
		} else {
			builder.addEvent(recordIndex, Event{
				Kind: EventOutput, Source: resultLocation, Timestamp: step.Timestamp, Sensitivity: builder.options.DefaultSensitivity,
				Output: &OutputPayload{Text: output, Status: "unpaired"},
			}, len(result.Content), len(output))
			builder.trajectory.Report.UnpairedToolResults++
		}
	}
	if len(step.ToolCalls) > len(step.Observation.Results) {
		builder.trajectory.Report.UnpairedToolCalls += len(step.ToolCalls) - len(step.Observation.Results)
	}
	return nil
}

func ingestSWEbench(raw []byte, options IngestOptions) (Trajectory, error) {
	var item sweBenchCacheItemV1
	if err := json.Unmarshal(raw, &item); err != nil {
		return Trajectory{}, fmt.Errorf("decode SWE-bench cache item: %w", err)
	}
	var rawMessages []json.RawMessage
	if err := json.Unmarshal([]byte(item.Messages), &rawMessages); err != nil {
		return Trajectory{}, fmt.Errorf("decode SWE-bench messages: %w", err)
	}
	builder := newTrajectoryBuilder(SourceSWEbench, raw, options)
	metadataLocation := SourceLocation{Record: 1, JSONPointer: "/"}
	metadataRecord := builder.beginRecord(metadataLocation, "cache_item_metadata", len(raw)-len(item.Messages)-len(item.OutputPatch))
	builder.addEvent(metadataRecord, Event{
		Kind: EventMetadata, Source: metadataLocation, SourceEventID: item.TrajectoryID, Sensitivity: SensitivityPrivate,
		Metadata: &MetadataPayload{Name: "swe_bench.cache_item", Value: item.ModelName, ValueDigest: digestString(item.InstanceID + item.TrajectoryID), Present: true},
	}, len(item.InstanceID)+len(item.ModelName)+len(item.TrajectoryID), len(item.ModelName))
	pending := make(map[string]string)
	for index, rawMessage := range rawMessages {
		location := SourceLocation{Record: index + 2, JSONPointer: "/messages/" + strconv.Itoa(index)}
		recordIndex := builder.beginRecord(location, "message", len(rawMessage))
		if err := ingestSWEbenchMessage(builder, recordIndex, location, rawMessage, pending); err != nil {
			return Trajectory{}, err
		}
	}
	patchLocation := SourceLocation{Record: len(rawMessages) + 2, JSONPointer: "/output_patch"}
	patchRecord := builder.beginRecord(patchLocation, "output_patch", len(item.OutputPatch))
	patch := builder.sanitize(patchRecord, "/output_patch", item.OutputPatch)
	builder.addEvent(patchRecord, Event{
		Kind: EventFileChange, Source: patchLocation, SourceEventID: item.TrajectoryID, Sensitivity: builder.options.DefaultSensitivity,
		FileChange: &FileChangePayload{Operation: "final_patch", Diff: patch, DiffDigest: digestString(patch), SizeBytes: len(patch)},
	}, len(item.OutputPatch), len(patch))
	builder.trajectory.Report.UnpairedToolCalls += len(pending)
	return builder.finish()
}

func ingestSWEbenchMessage(builder *trajectoryBuilder, recordIndex int, location SourceLocation, raw json.RawMessage, pending map[string]string) error {
	var message sweBenchMessageV1
	if err := json.Unmarshal(raw, &message); err != nil {
		return fmt.Errorf("decode SWE-bench message %d: %w", location.Record-1, err)
	}
	text, err := decodeTextContent(message.Content)
	if err != nil {
		return fmt.Errorf("decode SWE-bench content at message %d: %w", location.Record-1, err)
	}
	text = builder.sanitize(recordIndex, location.JSONPointer+"/content", text)
	parts := []ContentPart{}
	var resultEvent *Event
	if message.Role == "tool" {
		prepared := builder.prepareEvent(Event{
			Kind: EventToolResult, Source: SourceLocation{Record: location.Record, Part: 1, JSONPointer: location.JSONPointer + "/content"},
			SourceEventID: message.ToolCallID, Sensitivity: builder.options.DefaultSensitivity,
			ToolResult: &ToolResultPayload{CallID: message.ToolCallID, Status: "completed", Output: []ContentPart{{Kind: ContentText, Text: text}}},
		}, len(message.Content), len(text))
		resultEvent = &prepared
		parts = append(parts, ContentPart{Kind: ContentEventReference, EventID: prepared.ID})
	} else if text != "" {
		parts = append(parts, ContentPart{Kind: ContentText, Text: text})
	}
	callEvents := make([]Event, 0, len(message.ToolCalls))
	for index, call := range message.ToolCalls {
		callLocation := location
		callLocation.Part = index + 1
		callLocation.JSONPointer += "/tool_calls/" + strconv.Itoa(index)
		arguments := builder.sanitize(recordIndex, callLocation.JSONPointer+"/function/arguments", call.Function.Arguments)
		callEvent := builder.prepareEvent(Event{
			Kind: EventToolCall, Source: callLocation, SourceEventID: call.ID, Sensitivity: builder.options.DefaultSensitivity,
			ToolCall: &ToolCallPayload{CallID: call.ID, ToolName: call.Function.Name, Arguments: arguments, ArgumentsDigest: digestString(arguments), Status: "requested"},
		}, len(call.Function.Arguments), len(arguments)+len(call.Function.Name))
		callEvents = append(callEvents, callEvent)
		parts = append(parts, ContentPart{Kind: ContentEventReference, EventID: callEvent.ID})
	}
	messageEvent := builder.prepareEvent(Event{
		Kind: EventMessage, Source: location, Sensitivity: builder.options.DefaultSensitivity,
		Message: &MessagePayload{Role: message.Role, Parts: parts},
	}, len(message.Content), contentPartsBytes(parts))
	messageID := builder.appendPreparedEvent(recordIndex, messageEvent)
	for index, callEvent := range callEvents {
		callID := builder.appendPreparedEvent(recordIndex, callEvent)
		builder.addLink(LinkParent, messageID, callID)
		pending[message.ToolCalls[index].ID] = callID
	}
	if resultEvent != nil {
		resultID := builder.appendPreparedEvent(recordIndex, *resultEvent)
		builder.addLink(LinkParent, messageID, resultID)
		if callID := pending[message.ToolCallID]; callID != "" {
			builder.addLink(LinkCallResult, callID, resultID)
			delete(pending, message.ToolCallID)
		} else {
			builder.trajectory.Report.UnpairedToolResults++
		}
	}
	return nil
}

func decodeTextContent(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var output string
	for _, part := range parts {
		if part.Type != "text" {
			return "", fmt.Errorf("unsupported content part %q", part.Type)
		}
		if output != "" {
			output += "\n"
		}
		output += part.Text
	}
	return output, nil
}

func rawScalarString(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return compactJSON(raw)
}

func normalizeAgentRole(role string) string {
	if role == "agent" {
		return "assistant"
	}
	return role
}

func eventByID(events []Event, id string) Event {
	for _, event := range events {
		if event.ID == id {
			return event
		}
	}
	return Event{}
}
