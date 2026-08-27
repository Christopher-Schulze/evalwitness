package preprocess

import (
	"fmt"
	"sort"
	"strings"
)

func RenderTrajectory(trajectory Trajectory) string {
	if text, ok := renderPlainTextTrajectory(trajectory); ok {
		return text
	}
	events := append([]Event(nil), trajectory.Events...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].Order < events[j].Order })
	blocks := make([]string, 0, len(events))
	for _, event := range events {
		blocks = append(blocks, renderEvent(event))
	}
	return strings.Join(blocks, "\n\n")
}

func RenderTrajectoryInOrder(trajectory Trajectory, eventIDs []string) (string, error) {
	if err := trajectory.Validate(); err != nil {
		return "", err
	}
	events, err := eventsInPresentationOrder(trajectory, eventIDs)
	if err != nil {
		return "", err
	}
	if len(events) == 1 {
		if text, ok := renderPlainTextTrajectory(Trajectory{SourceFormat: trajectory.SourceFormat, Events: events}); ok {
			return text, nil
		}
	}
	blocks := make([]string, len(events))
	for index, event := range events {
		blocks[index] = renderEvent(event)
	}
	return strings.Join(blocks, "\n\n"), nil
}

func eventsInPresentationOrder(trajectory Trajectory, eventIDs []string) ([]Event, error) {
	if len(eventIDs) != len(trajectory.Events) {
		return nil, fmt.Errorf("presentation order has %d events, want %d", len(eventIDs), len(trajectory.Events))
	}
	events := make(map[string]Event, len(trajectory.Events))
	for _, event := range trajectory.Events {
		events[event.ID] = event
	}
	positions := make(map[string]int, len(eventIDs))
	ordered := make([]Event, len(eventIDs))
	for index, eventID := range eventIDs {
		event, found := events[eventID]
		if !found {
			return nil, fmt.Errorf("presentation order references unknown event %q", eventID)
		}
		if _, duplicate := positions[eventID]; duplicate {
			return nil, fmt.Errorf("presentation order repeats event %q", eventID)
		}
		positions[eventID], ordered[index] = index, event
	}
	for _, link := range trajectory.Links {
		if positions[link.FromID] >= positions[link.ToID] {
			return nil, fmt.Errorf("presentation order violates %s dependency %s -> %s", link.Kind, link.FromID, link.ToID)
		}
	}
	return ordered, nil
}

func renderPlainTextTrajectory(trajectory Trajectory) (string, bool) {
	if trajectory.SourceFormat != SourcePlainText || len(trajectory.Events) != 1 {
		return "", false
	}
	event := trajectory.Events[0]
	if event.Kind != EventMessage || event.Message == nil || len(event.Message.Parts) != 1 {
		return "", false
	}
	part := event.Message.Parts[0]
	if part.Kind != ContentText {
		return "", false
	}
	return part.Text, true
}

func renderEvent(event Event) string {
	var body strings.Builder
	fmt.Fprintf(&body, "--- Event %s | %s", event.ID, event.Kind)
	if event.SourceEventID != "" {
		fmt.Fprintf(&body, " | source=%s", event.SourceEventID)
	}
	body.WriteString(" ---\n")
	switch event.Kind {
	case EventMessage:
		if event.Message == nil {
			body.WriteString("[Message payload missing]\n")
		} else {
			renderMessage(&body, event.Message)
		}
	case EventToolCall:
		if event.ToolCall == nil {
			body.WriteString("[Tool call payload missing]\n")
		} else {
			renderToolCall(&body, event.ToolCall)
		}
	case EventToolResult:
		if event.ToolResult == nil {
			body.WriteString("[Tool result payload missing]\n")
		} else {
			renderToolResult(&body, event.ToolResult)
		}
	case EventCommand:
		if event.Command == nil {
			body.WriteString("[Command payload missing]\n")
		} else {
			renderCommand(&body, event.Command)
		}
	case EventOutput:
		if event.Output == nil {
			body.WriteString("[Output payload missing]\n")
		} else {
			renderOutput(&body, event.Output)
		}
	case EventFileChange:
		if event.FileChange == nil {
			body.WriteString("[File change payload missing]\n")
		} else {
			renderFileChange(&body, event.FileChange)
		}
	case EventAttachment:
		if event.Attachment == nil {
			body.WriteString("[Attachment payload missing]\n")
		} else {
			renderAttachment(&body, event.Attachment)
		}
	case EventError:
		if event.Error == nil {
			body.WriteString("[Error payload missing]\n")
		} else {
			renderError(&body, event.Error)
		}
	case EventMetadata:
		if event.Metadata == nil {
			body.WriteString("[Metadata payload missing]\n")
		} else {
			renderMetadata(&body, event.Metadata)
		}
	case EventReasoning:
		body.WriteString("[Reasoning] present; content omitted by policy\n")
	case EventContribution:
		if event.Contribution == nil {
			body.WriteString("[Contribution payload missing]\n")
		} else {
			fmt.Fprintf(&body, "[Contribution path=%s lines=%d-%d contributor=%s model=%s]\n",
				event.Contribution.PathAlias, event.Contribution.StartLine, event.Contribution.EndLine,
				event.Contribution.ContributorType, event.Contribution.ModelID)
		}
	case EventEvaluation:
		if event.Evaluation == nil {
			body.WriteString("[Evaluation payload missing]\n")
		} else {
			fmt.Fprintf(&body, "[Evaluation name=%s score=%s label=%s response=%s]\n",
				event.Evaluation.Name, event.Evaluation.ScoreValue, event.Evaluation.ScoreLabel, event.Evaluation.ResponseID)
		}
	}
	return strings.TrimRight(body.String(), "\n")
}

func renderMessage(output *strings.Builder, payload *MessagePayload) {
	fmt.Fprintf(output, "[Message role=%s", payload.Role)
	if payload.Phase != "" {
		fmt.Fprintf(output, " phase=%s", payload.Phase)
	}
	output.WriteString("]\n")
	for _, part := range payload.Parts {
		renderContentPart(output, part)
	}
}

func renderContentPart(output *strings.Builder, part ContentPart) {
	switch part.Kind {
	case ContentText:
		output.WriteString(part.Text)
		if !strings.HasSuffix(part.Text, "\n") {
			output.WriteByte('\n')
		}
	case ContentEventReference:
		fmt.Fprintf(output, "[Event reference: %s]\n", part.EventID)
	case ContentImage, ContentAudio, ContentFile:
		fmt.Fprintf(output, "[Attachment kind=%s media=%s name=%s digest=%s availability=%s]\n",
			part.Kind, part.MediaType, part.NameAlias, part.Digest, part.Availability)
	}
}

func renderToolCall(output *strings.Builder, payload *ToolCallPayload) {
	fmt.Fprintf(output, "[Tool call name=%s call_id=%s status=%s]\n", payload.ToolName, payload.CallID, payload.Status)
	if payload.Arguments != "" {
		output.WriteString(payload.Arguments)
		output.WriteByte('\n')
	}
}

func renderToolResult(output *strings.Builder, payload *ToolResultPayload) {
	fmt.Fprintf(output, "[Tool result call_id=%s status=%s error=%t", payload.CallID, payload.Status, payload.Error)
	if payload.ExitCode != nil {
		fmt.Fprintf(output, " exit=%d", *payload.ExitCode)
	}
	output.WriteString("]\n")
	renderResultStream(output, "stdout", payload.Stdout)
	renderResultStream(output, "stderr", payload.Stderr)
	for _, part := range payload.Output {
		renderContentPart(output, part)
	}
}

func renderResultStream(output *strings.Builder, stream string, parts []ContentPart) {
	if len(parts) == 0 {
		return
	}
	fmt.Fprintf(output, "[%s]\n", stream)
	for _, part := range parts {
		renderContentPart(output, part)
	}
}

func renderCommand(output *strings.Builder, payload *CommandPayload) {
	fmt.Fprintf(output, "[Command cwd=%s", payload.WorkingDirectoryAlias)
	if payload.ExitCode != nil {
		fmt.Fprintf(output, " exit=%d", *payload.ExitCode)
	}
	output.WriteString("]\n")
	output.WriteString(payload.Display)
	output.WriteByte('\n')
}

func renderOutput(output *strings.Builder, payload *OutputPayload) {
	fmt.Fprintf(output, "[Output stream=%s status=%s]\n", payload.Stream, payload.Status)
	output.WriteString(payload.Text)
	output.WriteByte('\n')
}

func renderFileChange(output *strings.Builder, payload *FileChangePayload) {
	fmt.Fprintf(output, "[File change operation=%s path=%s digest=%s]\n", payload.Operation, payload.PathAlias, payload.ContentDigest)
	if payload.Diff != "" {
		output.WriteString(payload.Diff)
		output.WriteByte('\n')
	}
}

func renderAttachment(output *strings.Builder, payload *AttachmentPayload) {
	fmt.Fprintf(output, "[Attachment media=%s name=%s bytes=%d digest=%s availability=%s]\n",
		payload.MediaType, payload.NameAlias, payload.SizeBytes, payload.Digest, payload.Availability)
}

func renderError(output *strings.Builder, payload *ErrorPayload) {
	fmt.Fprintf(output, "[Error class=%s]\n", payload.Class)
	if payload.SafeMessage != "" {
		output.WriteString(payload.SafeMessage)
		output.WriteByte('\n')
	}
}

func renderMetadata(output *strings.Builder, payload *MetadataPayload) {
	fmt.Fprintf(output, "[Metadata name=%s present=%t", payload.Name, payload.Present)
	if payload.ValueDigest != "" {
		fmt.Fprintf(output, " digest=%s", payload.ValueDigest)
	}
	output.WriteString("]\n")
	if payload.Value != "" {
		output.WriteString(payload.Value)
		output.WriteByte('\n')
	}
}
