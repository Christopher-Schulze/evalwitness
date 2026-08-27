package preprocess

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type jsonRecord struct {
	Location SourceLocation
	Raw      []byte
}

type trajectoryBuilder struct {
	trajectory Trajectory
	options    IngestOptions
	order      int
	err        error
}

func IngestString(raw string, options IngestOptions) (Trajectory, error) {
	return IngestReader(strings.NewReader(raw), options)
}

func IngestReader(reader io.Reader, options IngestOptions) (Trajectory, error) {
	var err error
	options, err = normalizeIngestOptions(options)
	if err != nil {
		return Trajectory{}, err
	}
	raw, err := readBounded(reader, options.MaxSourceBytes)
	if err != nil {
		return Trajectory{}, err
	}
	return ingestBytes(raw, options)
}

func normalizeIngestOptions(options IngestOptions) (IngestOptions, error) {
	defaults := DefaultIngestOptions()
	if options.Mode == "" {
		options.Mode = defaults.Mode
	}
	if options.CanonicalizationProfile == "" {
		options.CanonicalizationProfile = defaults.CanonicalizationProfile
	}
	if options.MaxSourceBytes <= 0 {
		options.MaxSourceBytes = defaults.MaxSourceBytes
	}
	if options.MaxRecordBytes <= 0 {
		options.MaxRecordBytes = defaults.MaxRecordBytes
	}
	if options.DefaultSensitivity == "" {
		options.DefaultSensitivity = defaults.DefaultSensitivity
	}
	switch options.CanonicalizationProfile {
	case CanonicalizationProfileV1, CanonicalizationProfileV2:
		return options, nil
	default:
		return IngestOptions{}, fmt.Errorf("unsupported canonicalization profile %q", options.CanonicalizationProfile)
	}
}

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	if maximum <= 0 {
		return nil, errors.New("maximum source bytes must be positive")
	}
	limited := io.LimitReader(reader, maximum+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read trajectory source: %w", err)
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("trajectory source exceeds %d-byte limit", maximum)
	}
	return raw, nil
}

func ingestBytes(raw []byte, options IngestOptions) (Trajectory, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return Trajectory{}, errors.New("trajectory source is empty")
	}
	if trimmed[0] != '{' {
		return ingestPlainText(raw, options)
	}
	format := detectObjectFormat(trimmed)
	switch format {
	case SourceOpenCode:
		return ingestOpenCode(trimmed, options)
	case SourceTerminalBench:
		return ingestTerminalBench(trimmed, options)
	case SourceSWEbench:
		return ingestSWEbench(trimmed, options)
	}
	return ingestJSONL(trimmed, options)
}

func detectObjectFormat(raw []byte) SourceFormat {
	var marker struct {
		Info       json.RawMessage `json:"info"`
		Messages   json.RawMessage `json:"messages"`
		Trajectory json.RawMessage `json:"trajectory"`
		InstanceID string          `json:"instance_id"`
	}
	if err := json.Unmarshal(raw, &marker); err != nil {
		return ""
	}
	if len(marker.Info) > 0 && jsonValueIsArray(marker.Messages) {
		return SourceOpenCode
	}
	if len(marker.Trajectory) > 0 {
		return SourceTerminalBench
	}
	if marker.InstanceID != "" && jsonValueIsString(marker.Messages) {
		return SourceSWEbench
	}
	return ""
}

func jsonValueIsArray(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '['
}

func jsonValueIsString(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '"'
}

func ingestJSONL(raw []byte, options IngestOptions) (Trajectory, error) {
	records, err := readJSONL(bytes.NewReader(raw), options)
	if err != nil {
		return Trajectory{}, err
	}
	if len(records) == 0 {
		return Trajectory{}, errors.New("structured trajectory contains no JSON records")
	}
	var marker struct {
		Type      string          `json:"type"`
		Message   json.RawMessage `json:"message"`
		SessionID string          `json:"sessionId"`
	}
	if err := json.Unmarshal(records[0].Raw, &marker); err != nil {
		return Trajectory{}, fmt.Errorf("decode first JSONL record: %w", err)
	}
	if isCodexRecordType(marker.Type) {
		return ingestCodex(records, raw, options)
	}
	if marker.SessionID != "" || len(marker.Message) > 0 || isClaudeRecordType(marker.Type) {
		return ingestClaude(records, raw, options)
	}
	if options.Mode == IngestStrict {
		return Trajectory{}, fmt.Errorf("unrecognized structured JSONL record type %q", marker.Type)
	}
	return ingestPlainText(raw, options)
}

func readJSONL(reader io.Reader, options IngestOptions) ([]jsonRecord, error) {
	buffered := bufio.NewReaderSize(reader, 64<<10)
	records := make([]jsonRecord, 0, 128)
	offset := int64(0)
	lineNumber := 0
	for {
		line, err := buffered.ReadBytes('\n')
		if len(line) > options.MaxRecordBytes {
			return nil, fmt.Errorf("JSONL record %d exceeds %d-byte limit", lineNumber+1, options.MaxRecordBytes)
		}
		lineNumber++
		content := bytes.TrimSpace(line)
		if len(content) > 0 {
			if !json.Valid(content) {
				return nil, fmt.Errorf("malformed JSONL record at line %d", lineNumber)
			}
			records = append(records, jsonRecord{
				Location: SourceLocation{Record: len(records) + 1, Line: lineNumber, ByteStart: offset, ByteEnd: offset + int64(len(line))},
				Raw:      append([]byte(nil), content...),
			})
		}
		offset += int64(len(line))
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read JSONL record %d: %w", lineNumber, err)
		}
	}
	return records, nil
}

func newTrajectoryBuilder(format SourceFormat, raw []byte, options IngestOptions) *trajectoryBuilder {
	return &trajectoryBuilder{
		trajectory: Trajectory{
			SourceFormat: format,
			SourceDigest: digestBytes(raw),
			Events:       []Event{},
			Links:        []Link{},
			Report: IngestionReport{
				OriginalBytes: len(raw),
				Records:       []RecordAccounting{},
			},
		},
		options: options,
	}
}

func (b *trajectoryBuilder) beginRecord(location SourceLocation, sourceType string, originalBytes int) int {
	b.trajectory.Report.Records = append(b.trajectory.Report.Records, RecordAccounting{
		Source:        location,
		SourceType:    sourceType,
		Disposition:   DispositionMetadataOnly,
		OriginalBytes: originalBytes,
	})
	return len(b.trajectory.Report.Records) - 1
}

func (b *trajectoryBuilder) addEvent(recordIndex int, event Event, originalBytes, retainedBytes int) string {
	event = b.prepareEvent(event, originalBytes, retainedBytes)
	return b.appendPreparedEvent(recordIndex, event)
}

func (b *trajectoryBuilder) prepareEvent(event Event, originalBytes, retainedBytes int) Event {
	event.ContentBytes = originalBytes
	event.RetainedBytes = retainedBytes
	event.EstimatedTokens = estimateTokensForBytes(retainedBytes)
	encoded, err := json.Marshal(eventPayloadMaterial(event))
	if err != nil {
		b.err = fmt.Errorf("encode canonical event payload: %w", err)
	}
	event.ContentDigest = digestBytes(encoded)
	event.ID = stableEventID(b.trajectory.SourceFormat, event.Source, event.Kind, event.ContentDigest)
	return event
}

func (b *trajectoryBuilder) appendPreparedEvent(recordIndex int, event Event) string {
	event.Order = b.order
	b.order++
	b.trajectory.Events = append(b.trajectory.Events, event)
	record := &b.trajectory.Report.Records[recordIndex]
	record.EventIDs = append(record.EventIDs, event.ID)
	record.RetainedBytes += event.RetainedBytes
	record.Disposition = DispositionRepresented
	b.trajectory.Report.RetainedBytes += event.RetainedBytes
	return event.ID
}

func eventPayloadMaterial(event Event) struct {
	Kind         EventKind
	SourceID     string
	Timestamp    string
	Message      *MessagePayload
	ToolCall     *ToolCallPayload
	ToolResult   *ToolResultPayload
	Command      *CommandPayload
	Output       *OutputPayload
	FileChange   *FileChangePayload
	Attachment   *AttachmentPayload
	Error        *ErrorPayload
	Metadata     *MetadataPayload
	Reasoning    *ReasoningPayload
	Contribution *ContributionPayload  `json:",omitempty"`
	Evaluation   *EvaluationPayload    `json:",omitempty"`
	External     *ExternalTraceContext `json:",omitempty"`
} {
	return struct {
		Kind         EventKind
		SourceID     string
		Timestamp    string
		Message      *MessagePayload
		ToolCall     *ToolCallPayload
		ToolResult   *ToolResultPayload
		Command      *CommandPayload
		Output       *OutputPayload
		FileChange   *FileChangePayload
		Attachment   *AttachmentPayload
		Error        *ErrorPayload
		Metadata     *MetadataPayload
		Reasoning    *ReasoningPayload
		Contribution *ContributionPayload  `json:",omitempty"`
		Evaluation   *EvaluationPayload    `json:",omitempty"`
		External     *ExternalTraceContext `json:",omitempty"`
	}{
		Kind: event.Kind, SourceID: event.SourceEventID, Timestamp: event.Timestamp,
		Message: event.Message, ToolCall: event.ToolCall, ToolResult: event.ToolResult,
		Command: event.Command, Output: event.Output, FileChange: event.FileChange,
		Attachment: event.Attachment, Error: event.Error, Metadata: event.Metadata,
		Reasoning: event.Reasoning, Contribution: event.Contribution,
		Evaluation: event.Evaluation, External: event.External,
	}
}

func (b *trajectoryBuilder) addField(recordIndex int, field FieldAccounting) {
	record := &b.trajectory.Report.Records[recordIndex]
	record.Fields = append(record.Fields, field)
	switch field.Disposition {
	case DispositionUnsupported:
		b.trajectory.Report.UnsupportedRecords++
	case DispositionRedacted:
		b.trajectory.Report.RedactedFields++
		b.trajectory.Report.RedactedBytes += field.OriginalBytes - field.RetainedBytes
	}
}

func (b *trajectoryBuilder) addUnsupported(recordIndex int, path, reason string, bytes int) error {
	record := &b.trajectory.Report.Records[recordIndex]
	record.Disposition = DispositionUnsupported
	record.Reason = reason
	b.addField(recordIndex, FieldAccounting{
		Path: path, Disposition: DispositionUnsupported, OriginalBytes: bytes, Reason: reason,
	})
	if b.options.Mode == IngestStrict {
		return fmt.Errorf("unsupported %s at %s", record.SourceType, path)
	}
	return nil
}

func (b *trajectoryBuilder) sanitize(recordIndex int, path, value string) string {
	cleaned, hits := Redact(value, b.options.Redact)
	if hits == 0 {
		return cleaned
	}
	b.trajectory.Report.RedactionHits += hits
	b.addField(recordIndex, FieldAccounting{
		Path: path, Disposition: DispositionRedacted,
		OriginalBytes: len(value), RetainedBytes: len(cleaned), Reason: "secret redaction policy",
	})
	return cleaned
}

func (b *trajectoryBuilder) addLink(kind LinkKind, fromID, toID string) {
	if fromID == "" || toID == "" {
		return
	}
	b.trajectory.Links = append(b.trajectory.Links, Link{Kind: kind, FromID: fromID, ToID: toID})
}

func (b *trajectoryBuilder) addProviderUsage(usage ProviderTokenUsage) {
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens + usage.ReasoningTokens + usage.CachedInputTokens + usage.CacheCreationInputTokens
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.ReasoningTokens == 0 &&
		usage.CachedInputTokens == 0 && usage.CacheCreationInputTokens == 0 && usage.TotalTokens == 0 {
		return
	}
	b.trajectory.Report.ProviderUsage = append(b.trajectory.Report.ProviderUsage, usage)
}

func (b *trajectoryBuilder) commandPayloadFromShell(display, workingDirectoryAlias string) *CommandPayload {
	if b.options.CanonicalizationProfile == CanonicalizationProfileV1 {
		return &CommandPayload{Display: display, WorkingDirectoryAlias: workingDirectoryAlias}
	}
	return &CommandPayload{
		Display: display, UnsupportedShellText: display, OperandsDigest: digestJSONValue(display),
		WorkingDirectoryAlias: workingDirectoryAlias,
	}
}

func (b *trajectoryBuilder) commandPayloadFromArgv(display string, argv []string, workingDirectoryAlias string) *CommandPayload {
	if b.options.CanonicalizationProfile == CanonicalizationProfileV1 {
		return &CommandPayload{Display: display, WorkingDirectoryAlias: workingDirectoryAlias}
	}
	return &CommandPayload{
		Display: display, Argv: append([]string(nil), argv...), OperandsDigest: digestJSONValue(argv),
		WorkingDirectoryAlias: workingDirectoryAlias,
	}
}

func digestJSONValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return digestBytes(encoded)
}

func (b *trajectoryBuilder) finish() (Trajectory, error) {
	if b.err != nil {
		return Trajectory{}, b.err
	}
	if err := finalizeTrajectory(&b.trajectory); err != nil {
		return Trajectory{}, err
	}
	return b.trajectory, nil
}

func ingestPlainText(raw []byte, options IngestOptions) (Trajectory, error) {
	builder := newTrajectoryBuilder(SourcePlainText, raw, options)
	location := SourceLocation{Record: 1, Line: 1, ByteEnd: int64(len(raw))}
	record := builder.beginRecord(location, "plain_text", len(raw))
	textSource := string(raw)
	normalizationReason := ""
	if !utf8.ValidString(textSource) {
		textSource = strings.ToValidUTF8(textSource, "�")
		normalizationReason = "invalid UTF-8 normalized before canonicalization"
	}
	text := builder.sanitize(record, "/text", textSource)
	eventID := builder.addEvent(record, Event{
		Kind: EventMessage, Source: location, Sensitivity: options.DefaultSensitivity,
		Message: &MessagePayload{Role: "unknown", Parts: []ContentPart{{Kind: ContentText, Text: text}}},
	}, len(raw), len(text))
	builder.addField(record, FieldAccounting{
		Path: "/text", Disposition: DispositionRepresented, OriginalBytes: len(raw), RetainedBytes: len(text),
		Reason: normalizationReason, EventIDs: []string{eventID},
	})
	return builder.finish()
}

func isCodexRecordType(value string) bool {
	switch value {
	case "session_meta", "response_item", "inter_agent_communication", "inter_agent_communication_metadata", "compacted", "turn_context", "world_state", "event_msg":
		return true
	default:
		return false
	}
}

func isClaudeRecordType(value string) bool {
	switch value {
	case "assistant", "user", "attachment", "system", "file-history-snapshot", "file-history-delta", "queue-operation", "last-prompt", "permission-mode", "mode", "bridge-session", "ai-title":
		return true
	default:
		return false
	}
}
