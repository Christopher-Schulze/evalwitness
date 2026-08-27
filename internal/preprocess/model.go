package preprocess

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const CanonicalTrajectorySchema = "evalwitness.trajectory.v1"

type SourceFormat string

const (
	SourcePlainText     SourceFormat = "plain_text"
	SourceClaudeCode    SourceFormat = "claude_code_jsonl"
	SourceCodexRollout  SourceFormat = "codex_rollout_jsonl"
	SourceOpenCode      SourceFormat = "opencode_export_json"
	SourceTerminalBench SourceFormat = "terminal_bench_trajectory_json"
	SourceSWEbench      SourceFormat = "swe_bench_cache_item_json"
	SourceOTLPJSON      SourceFormat = "otlp_json"
	SourceAgentTrace    SourceFormat = "agent_trace_json"
)

type EventKind string

const (
	EventMessage      EventKind = "message"
	EventToolCall     EventKind = "tool_call"
	EventToolResult   EventKind = "tool_result"
	EventCommand      EventKind = "command"
	EventOutput       EventKind = "output"
	EventFileChange   EventKind = "file_change"
	EventError        EventKind = "error"
	EventAttachment   EventKind = "attachment"
	EventMetadata     EventKind = "metadata"
	EventReasoning    EventKind = "reasoning"
	EventContribution EventKind = "contribution"
	EventEvaluation   EventKind = "evaluation"
)

type ContentKind string

const (
	ContentText           ContentKind = "text"
	ContentImage          ContentKind = "image"
	ContentAudio          ContentKind = "audio"
	ContentFile           ContentKind = "file"
	ContentEventReference ContentKind = "event_reference"
)

type SensitivityClass string

const (
	SensitivityPublic              SensitivityClass = "public"
	SensitivityPrivate             SensitivityClass = "private"
	SensitivityPotentialSecret     SensitivityClass = "potential_secret"
	SensitivityRestrictedReasoning SensitivityClass = "restricted_reasoning"
)

type LinkKind string

const (
	LinkParent     LinkKind = "parent"
	LinkCallResult LinkKind = "call_result"
	LinkFileChange LinkKind = "file_change"
	LinkDerivation LinkKind = "derivation"
	LinkReference  LinkKind = "reference"
)

type AccountingDisposition string

const (
	DispositionRepresented      AccountingDisposition = "represented"
	DispositionMetadataOnly     AccountingDisposition = "metadata_only"
	DispositionOmittedSensitive AccountingDisposition = "omitted_sensitive"
	DispositionUnsupported      AccountingDisposition = "unsupported"
	DispositionRedacted         AccountingDisposition = "redacted"
	DispositionTruncated        AccountingDisposition = "truncated"
	DispositionRejected         AccountingDisposition = "rejected"
)

type IngestMode string

const (
	IngestStrict        IngestMode = "strict"
	IngestCompatibility IngestMode = "compatibility"
)

type CanonicalizationProfile string

const (
	CanonicalizationProfileV1 CanonicalizationProfile = "evalwitness.canonicalization.v1"
	CanonicalizationProfileV2 CanonicalizationProfile = "evalwitness.canonicalization.v2"
)

type SourceLocation struct {
	Record      int    `json:"record"`
	Line        int    `json:"line,omitempty"`
	Part        int    `json:"part,omitempty"`
	JSONPointer string `json:"json_pointer,omitempty"`
	ByteStart   int64  `json:"byte_start,omitempty"`
	ByteEnd     int64  `json:"byte_end,omitempty"`
}

func (l SourceLocation) key() string {
	return fmt.Sprintf("%d:%d:%d:%s:%d:%d", l.Record, l.Line, l.Part, l.JSONPointer, l.ByteStart, l.ByteEnd)
}

type ContentPart struct {
	Kind         ContentKind `json:"kind"`
	Text         string      `json:"text,omitempty"`
	MediaType    string      `json:"media_type,omitempty"`
	NameAlias    string      `json:"name_alias,omitempty"`
	Digest       string      `json:"digest,omitempty"`
	SizeBytes    int         `json:"size_bytes,omitempty"`
	Availability string      `json:"availability,omitempty"`
	EventID      string      `json:"event_id,omitempty"`
}

type MessagePayload struct {
	Role  string        `json:"role"`
	Parts []ContentPart `json:"parts"`
	Phase string        `json:"phase,omitempty"`
}

type ToolCallPayload struct {
	CallID          string `json:"call_id,omitempty"`
	ToolName        string `json:"tool_name"`
	Arguments       string `json:"arguments,omitempty"`
	ArgumentsDigest string `json:"arguments_digest,omitempty"`
	Status          string `json:"status,omitempty"`
}

type ToolResultPayload struct {
	CallID   string        `json:"call_id,omitempty"`
	Status   string        `json:"status"`
	Error    bool          `json:"error"`
	ExitCode *int          `json:"exit_code,omitempty"`
	Stdout   []ContentPart `json:"stdout,omitempty"`
	Stderr   []ContentPart `json:"stderr,omitempty"`
	Output   []ContentPart `json:"output,omitempty"`
}

type CommandPayload struct {
	Display               string   `json:"display"`
	Argv                  []string `json:"argv,omitempty"`
	UnsupportedShellText  string   `json:"unsupported_shell_text,omitempty"`
	OperandsDigest        string   `json:"operands_digest,omitempty"`
	WorkingDirectoryAlias string   `json:"working_directory_alias,omitempty"`
	ExitCode              *int     `json:"exit_code,omitempty"`
}

type OutputPayload struct {
	Stream string `json:"stream,omitempty"`
	Text   string `json:"text,omitempty"`
	Status string `json:"status,omitempty"`
}

type FileChangePayload struct {
	Operation     string `json:"operation"`
	PathAlias     string `json:"path_alias,omitempty"`
	Diff          string `json:"diff,omitempty"`
	ContentDigest string `json:"content_digest,omitempty"`
	DiffDigest    string `json:"diff_digest,omitempty"`
	SizeBytes     int    `json:"size_bytes,omitempty"`
}

type AttachmentPayload struct {
	MediaType    string `json:"media_type,omitempty"`
	NameAlias    string `json:"name_alias,omitempty"`
	SizeBytes    int    `json:"size_bytes,omitempty"`
	Digest       string `json:"digest,omitempty"`
	Availability string `json:"availability"`
}

type ErrorPayload struct {
	Class       string `json:"class"`
	SafeMessage string `json:"safe_message,omitempty"`
}

type MetadataPayload struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	ValueDigest string `json:"value_digest,omitempty"`
	Present     bool   `json:"present"`
}

type ReasoningPayload struct {
	Present   bool `json:"present"`
	Omitted   bool `json:"omitted"`
	Encrypted bool `json:"encrypted,omitempty"`
}

type ExternalTraceContext struct {
	TraceID         string `json:"trace_id"`
	SpanID          string `json:"span_id"`
	ParentSpanID    string `json:"parent_span_id,omitempty"`
	OperationName   string `json:"operation_name,omitempty"`
	ProviderName    string `json:"provider_name,omitempty"`
	RequestModel    string `json:"request_model,omitempty"`
	ResponseModel   string `json:"response_model,omitempty"`
	ConversationID  string `json:"conversation_id,omitempty"`
	ResponseID      string `json:"response_id,omitempty"`
	FinishReasons   string `json:"finish_reasons,omitempty"`
	ToolType        string `json:"tool_type,omitempty"`
	AgentID         string `json:"agent_id,omitempty"`
	AgentName       string `json:"agent_name,omitempty"`
	AgentVersion    string `json:"agent_version,omitempty"`
	SchemaURL       string `json:"schema_url,omitempty"`
	Instrumentation string `json:"instrumentation,omitempty"`
}

type ContributionPayload struct {
	Path               string `json:"path,omitempty"`
	PathAlias          string `json:"path_alias"`
	StartLine          int    `json:"start_line"`
	EndLine            int    `json:"end_line"`
	ContributorType    string `json:"contributor_type"`
	ModelID            string `json:"model_id,omitempty"`
	ConversationURL    string `json:"conversation_url,omitempty"`
	ConversationDigest string `json:"conversation_digest,omitempty"`
	ContentHash        string `json:"content_hash,omitempty"`
	VCS                string `json:"vcs,omitempty"`
	Revision           string `json:"revision,omitempty"`
}

type EvaluationPayload struct {
	Name        string `json:"name"`
	ScoreValue  string `json:"score_value,omitempty"`
	ScoreLabel  string `json:"score_label,omitempty"`
	Explanation string `json:"explanation,omitempty"`
	ResponseID  string `json:"response_id,omitempty"`
	ErrorType   string `json:"error_type,omitempty"`
}

type Event struct {
	ID              string                `json:"id"`
	Kind            EventKind             `json:"kind"`
	Order           int                   `json:"order"`
	Source          SourceLocation        `json:"source"`
	SourceEventID   string                `json:"source_event_id,omitempty"`
	Timestamp       string                `json:"timestamp,omitempty"`
	Sensitivity     SensitivityClass      `json:"sensitivity"`
	ContentBytes    int                   `json:"content_bytes"`
	RetainedBytes   int                   `json:"retained_bytes"`
	EstimatedTokens int                   `json:"estimated_tokens"`
	ContentDigest   string                `json:"content_digest"`
	Message         *MessagePayload       `json:"message,omitempty"`
	ToolCall        *ToolCallPayload      `json:"tool_call,omitempty"`
	ToolResult      *ToolResultPayload    `json:"tool_result,omitempty"`
	Command         *CommandPayload       `json:"command,omitempty"`
	Output          *OutputPayload        `json:"output,omitempty"`
	FileChange      *FileChangePayload    `json:"file_change,omitempty"`
	Attachment      *AttachmentPayload    `json:"attachment,omitempty"`
	Error           *ErrorPayload         `json:"error,omitempty"`
	Metadata        *MetadataPayload      `json:"metadata,omitempty"`
	Reasoning       *ReasoningPayload     `json:"reasoning,omitempty"`
	Contribution    *ContributionPayload  `json:"contribution,omitempty"`
	Evaluation      *EvaluationPayload    `json:"evaluation,omitempty"`
	External        *ExternalTraceContext `json:"external_trace_context,omitempty"`
}

type Link struct {
	Kind   LinkKind `json:"kind"`
	FromID string   `json:"from_id"`
	ToID   string   `json:"to_id"`
}

type FieldAccounting struct {
	Path          string                `json:"path"`
	Disposition   AccountingDisposition `json:"disposition"`
	OriginalBytes int                   `json:"original_bytes"`
	RetainedBytes int                   `json:"retained_bytes"`
	Reason        string                `json:"reason,omitempty"`
	EventIDs      []string              `json:"event_ids,omitempty"`
}

type RecordAccounting struct {
	Source        SourceLocation        `json:"source"`
	SourceType    string                `json:"source_type"`
	Disposition   AccountingDisposition `json:"disposition"`
	OriginalBytes int                   `json:"original_bytes"`
	RetainedBytes int                   `json:"retained_bytes"`
	EventIDs      []string              `json:"event_ids,omitempty"`
	Fields        []FieldAccounting     `json:"fields,omitempty"`
	Reason        string                `json:"reason,omitempty"`
}

type CategoryRetention struct {
	Kind           EventKind `json:"kind"`
	OriginalEvents int       `json:"original_events"`
	RetainedEvents int       `json:"retained_events"`
	OriginalBytes  int       `json:"original_bytes"`
	RetainedBytes  int       `json:"retained_bytes"`
}

type TruncationBoundary struct {
	Applied        bool   `json:"applied"`
	BudgetTokens   int    `json:"budget_tokens,omitempty"`
	RetainedTokens int    `json:"retained_tokens,omitempty"`
	EventID        string `json:"event_id,omitempty"`
	FieldPath      string `json:"field_path,omitempty"`
	Reason         string `json:"reason,omitempty"`
	OriginalDigest string `json:"original_digest,omitempty"`
	RetainedDigest string `json:"retained_digest,omitempty"`
}

type EventSelection struct {
	EventID          string                `json:"event_id"`
	Score            int                   `json:"score"`
	OriginalTokens   int                   `json:"original_tokens"`
	RetainedTokens   int                   `json:"retained_tokens"`
	Disposition      AccountingDisposition `json:"disposition"`
	Reason           string                `json:"reason"`
	LinkedConstraint string                `json:"linked_constraint,omitempty"`
}

type ProviderTokenUsage struct {
	Provider                 string         `json:"provider"`
	Scope                    string         `json:"scope"`
	SourceEventID            string         `json:"source_event_id,omitempty"`
	Source                   SourceLocation `json:"source"`
	ObservedFields           []string       `json:"observed_fields,omitempty"`
	InputTokens              int            `json:"input_tokens"`
	OutputTokens             int            `json:"output_tokens"`
	ReasoningTokens          int            `json:"reasoning_tokens"`
	CachedInputTokens        int            `json:"cached_input_tokens"`
	CacheCreationInputTokens int            `json:"cache_creation_input_tokens"`
	TotalTokens              int            `json:"total_tokens"`
}

type IngestionReport struct {
	SchemaVersion       string               `json:"schema_version"`
	SourceRecords       int                  `json:"source_records"`
	AccountedRecords    int                  `json:"accounted_records"`
	CanonicalEvents     int                  `json:"canonical_events"`
	UnsupportedRecords  int                  `json:"unsupported_records"`
	ParseErrors         int                  `json:"parse_errors"`
	UnpairedToolCalls   int                  `json:"unpaired_tool_calls"`
	UnpairedToolResults int                  `json:"unpaired_tool_results"`
	OriginalBytes       int                  `json:"original_bytes"`
	RetainedBytes       int                  `json:"retained_bytes"`
	RedactedFields      int                  `json:"redacted_fields"`
	RedactionHits       int                  `json:"redaction_hits"`
	RedactedBytes       int                  `json:"redacted_bytes"`
	Records             []RecordAccounting   `json:"records"`
	Categories          []CategoryRetention  `json:"categories"`
	Truncation          TruncationBoundary   `json:"truncation"`
	Selection           []EventSelection     `json:"selection,omitempty"`
	ProviderUsage       []ProviderTokenUsage `json:"provider_usage,omitempty"`
}

type Derivation struct {
	ParentDigest      string      `json:"parent_digest"`
	Relation          string      `json:"relation"`
	Validator         string      `json:"validator"`
	ChangedEventIDs   []string    `json:"changed_event_ids"`
	ChangedFieldPaths []FieldPath `json:"changed_field_paths"`
}

type FieldPath string

type DerivationSpec struct {
	Relation          string
	Validator         string
	ChangedEventIDs   []string
	ChangedFieldPaths []FieldPath
}

type Trajectory struct {
	SchemaVersion string          `json:"schema_version"`
	SourceFormat  SourceFormat    `json:"source_format"`
	SourceDigest  string          `json:"source_digest"`
	Digest        string          `json:"digest"`
	Events        []Event         `json:"events"`
	Links         []Link          `json:"links"`
	Report        IngestionReport `json:"report"`
	Derivation    *Derivation     `json:"derivation,omitempty"`
}

type AccountingSummary struct {
	SchemaVersion       string             `json:"schema_version"`
	SourceFormat        SourceFormat       `json:"source_format"`
	SourceDigest        string             `json:"source_digest"`
	TrajectoryDigest    string             `json:"trajectory_digest"`
	IngestionDigest     string             `json:"ingestion_digest"`
	TraceEnvelopeDigest string             `json:"trace_envelope_digest,omitempty"`
	TraceMappingDigest  string             `json:"trace_mapping_digest,omitempty"`
	TraceMappingPolicy  string             `json:"trace_mapping_policy,omitempty"`
	SourceRecords       int                `json:"source_records"`
	CanonicalEvents     int                `json:"canonical_events"`
	RetainedEvents      int                `json:"retained_events"`
	OriginalBytes       int                `json:"original_bytes"`
	RetainedBytes       int                `json:"retained_bytes"`
	UnsupportedRecords  int                `json:"unsupported_records"`
	ParseErrors         int                `json:"parse_errors"`
	UnpairedToolCalls   int                `json:"unpaired_tool_calls"`
	UnpairedToolResults int                `json:"unpaired_tool_results"`
	RedactionHits       int                `json:"redaction_hits"`
	Truncation          TruncationBoundary `json:"truncation"`
}

type FormatAccounting struct {
	SourceFormat SourceFormat `json:"source_format"`
	Trajectories int          `json:"trajectories"`
}

type AccountingAggregate struct {
	SchemaVersion         string             `json:"schema_version"`
	Trajectories          int                `json:"trajectories"`
	Formats               []FormatAccounting `json:"formats"`
	SourceRecords         int                `json:"source_records"`
	CanonicalEvents       int                `json:"canonical_events"`
	RetainedEvents        int                `json:"retained_events"`
	OriginalBytes         int                `json:"original_bytes"`
	RetainedBytes         int                `json:"retained_bytes"`
	TruncatedTrajectories int                `json:"truncated_trajectories"`
	UnsupportedRecords    int                `json:"unsupported_records"`
	UnpairedToolCalls     int                `json:"unpaired_tool_calls"`
	UnpairedToolResults   int                `json:"unpaired_tool_results"`
	RedactionHits         int                `json:"redaction_hits"`
}

type IngestOptions struct {
	Mode                    IngestMode
	CanonicalizationProfile CanonicalizationProfile
	Redact                  bool
	MaxSourceBytes          int64
	MaxRecordBytes          int
	DefaultSensitivity      SensitivityClass
}

func DefaultIngestOptions() IngestOptions {
	return IngestOptions{
		Mode:                    IngestStrict,
		CanonicalizationProfile: CanonicalizationProfileV2,
		Redact:                  true,
		MaxSourceBytes:          256 << 20,
		MaxRecordBytes:          16 << 20,
		DefaultSensitivity:      SensitivityPrivate,
	}
}

// FrozenCanonicalizationV1IngestOptions reproduces canonical trajectories used
// by the frozen pre-lineage research artifacts. New ingestion must use
// DefaultIngestOptions and the current V2 profile.
func FrozenCanonicalizationV1IngestOptions() IngestOptions {
	options := DefaultIngestOptions()
	options.CanonicalizationProfile = CanonicalizationProfileV1
	return options
}

func (t *Trajectory) Validate() error {
	if t.SchemaVersion != CanonicalTrajectorySchema {
		return fmt.Errorf("unsupported canonical trajectory schema %q", t.SchemaVersion)
	}
	if t.SourceFormat == "" || t.SourceDigest == "" {
		return errors.New("canonical trajectory requires source format and digest")
	}
	ids := make(map[string]struct{}, len(t.Events))
	lastOrder := -1
	for index := range t.Events {
		event := &t.Events[index]
		if event.ID == "" {
			return fmt.Errorf("event %d has empty identity", index)
		}
		if _, exists := ids[event.ID]; exists {
			return fmt.Errorf("duplicate event identity %q", event.ID)
		}
		if event.Order < lastOrder {
			return fmt.Errorf("event %q is out of source order", event.ID)
		}
		if payloadCount(*event) != 1 {
			return fmt.Errorf("event %q must have exactly one typed payload", event.ID)
		}
		if err := validateTracePayload(*event); err != nil {
			return fmt.Errorf("event %q: %w", event.ID, err)
		}
		ids[event.ID] = struct{}{}
		lastOrder = event.Order
	}
	if err := validateLinks(t.Links, ids); err != nil {
		return err
	}
	if t.Report.SourceRecords != len(t.Report.Records) {
		return fmt.Errorf("source record count %d does not match accounting entries %d", t.Report.SourceRecords, len(t.Report.Records))
	}
	if t.Report.AccountedRecords != t.Report.SourceRecords {
		return fmt.Errorf("only %d of %d source records accounted", t.Report.AccountedRecords, t.Report.SourceRecords)
	}
	if err := validateDerivation(t.Derivation); err != nil {
		return err
	}
	return nil
}

func validateTracePayload(event Event) error {
	if event.Command != nil {
		command := event.Command
		legacyV1 := len(command.Argv) == 0 && command.UnsupportedShellText == "" && command.OperandsDigest == ""
		if command.Display == "" || !legacyV1 && ((len(command.Argv) == 0) == (command.UnsupportedShellText == "") || !canonicalSHA256(command.OperandsDigest)) {
			return errors.New("command payload requires display, one syntax form, and operand digest")
		}
	}
	if event.External != nil {
		if err := validateOTLPID(event.External.TraceID, 16, "external trace_id"); err != nil {
			return err
		}
		if err := validateOTLPID(event.External.SpanID, 8, "external span_id"); err != nil {
			return err
		}
		if event.External.ParentSpanID != "" {
			if err := validateOTLPID(event.External.ParentSpanID, 8, "external parent_span_id"); err != nil {
				return err
			}
		}
	}
	if event.Contribution != nil {
		if event.Kind != EventContribution || event.Contribution.PathAlias == "" || event.Contribution.StartLine < 1 || event.Contribution.EndLine < event.Contribution.StartLine || event.Contribution.ContributorType == "" {
			return errors.New("contribution payload requires matching kind, alias, contributor, and valid line range")
		}
		if event.Contribution.Path != "" {
			if err := validateRelativeTracePath(event.Contribution.Path); err != nil {
				return fmt.Errorf("contribution path: %w", err)
			}
		}
		if event.Contribution.ConversationURL != "" {
			if err := validateTraceURL(event.Contribution.ConversationURL); err != nil {
				return fmt.Errorf("contribution conversation URL: %w", err)
			}
		}
	}
	if event.Evaluation != nil && (event.Kind != EventEvaluation || strings.TrimSpace(event.Evaluation.Name) == "") {
		return errors.New("evaluation payload requires matching kind and name")
	}
	return nil
}

func validateDerivation(derivation *Derivation) error {
	if derivation == nil {
		return nil
	}
	if derivation.ParentDigest == "" || derivation.Relation == "" || derivation.Validator == "" {
		return errors.New("derived trajectory requires parent digest, relation, and validator")
	}
	seenEvents := make(map[string]struct{}, len(derivation.ChangedEventIDs))
	for _, eventID := range derivation.ChangedEventIDs {
		if eventID == "" {
			return errors.New("derived trajectory has empty changed-event identity")
		}
		if _, exists := seenEvents[eventID]; exists {
			return fmt.Errorf("derived trajectory repeats changed event %q", eventID)
		}
		seenEvents[eventID] = struct{}{}
	}
	seenFields := make(map[FieldPath]struct{}, len(derivation.ChangedFieldPaths))
	for _, path := range derivation.ChangedFieldPaths {
		value := string(path)
		remainder := strings.TrimPrefix(value, "/events/")
		if remainder == value || !strings.Contains(remainder, "/") {
			return fmt.Errorf("derived trajectory field path %q is not event-addressed", path)
		}
		if _, exists := seenFields[path]; exists {
			return fmt.Errorf("derived trajectory repeats changed field %q", path)
		}
		seenFields[path] = struct{}{}
	}
	return nil
}

func DeriveTrajectory(parent Trajectory, events []Event, links []Link, spec DerivationSpec) (Trajectory, error) {
	if err := parent.Validate(); err != nil {
		return Trajectory{}, fmt.Errorf("validate derivation parent: %w", err)
	}
	child, err := cloneTrajectory(parent)
	if err != nil {
		return Trajectory{}, err
	}
	child.Events = append([]Event(nil), events...)
	child.Links = append([]Link(nil), links...)
	remapReportEventIDs(&child.Report, eventIdentityRemap(parent.Events, child.Events))
	child.Derivation = &Derivation{
		ParentDigest: parent.Digest, Relation: spec.Relation, Validator: spec.Validator,
		ChangedEventIDs:   append([]string(nil), spec.ChangedEventIDs...),
		ChangedFieldPaths: append([]FieldPath(nil), spec.ChangedFieldPaths...),
	}
	if err := validateChangedReferences(parent.Events, child.Events, *child.Derivation); err != nil {
		return Trajectory{}, err
	}
	child.Report.Categories = categoryRetention(child.Events)
	child.Report.RetainedBytes = retainedEventBytes(child.Events)
	if err := recomputeTrajectoryDigest(&child); err != nil {
		return Trajectory{}, err
	}
	return child, child.Validate()
}

func eventIdentityRemap(parentEvents, childEvents []Event) map[string]string {
	childBySource := make(map[string]string, len(childEvents))
	for _, event := range childEvents {
		childBySource[eventSourceIdentity(event)] = event.ID
	}
	remapped := make(map[string]string)
	for _, event := range parentEvents {
		childID, exists := childBySource[eventSourceIdentity(event)]
		if exists && childID != event.ID {
			remapped[event.ID] = childID
		}
	}
	return remapped
}

func eventSourceIdentity(event Event) string {
	return event.Source.key() + "\x00" + string(event.Kind) + "\x00" + event.SourceEventID
}

func remapReportEventIDs(report *IngestionReport, remapped map[string]string) {
	remap := func(eventIDs []string) {
		for index, eventID := range eventIDs {
			if replacement, exists := remapped[eventID]; exists {
				eventIDs[index] = replacement
			}
		}
	}
	for recordIndex := range report.Records {
		remap(report.Records[recordIndex].EventIDs)
		for fieldIndex := range report.Records[recordIndex].Fields {
			remap(report.Records[recordIndex].Fields[fieldIndex].EventIDs)
		}
	}
	for selectionIndex := range report.Selection {
		if replacement, exists := remapped[report.Selection[selectionIndex].EventID]; exists {
			report.Selection[selectionIndex].EventID = replacement
		}
	}
	if replacement, exists := remapped[report.Truncation.EventID]; exists {
		report.Truncation.EventID = replacement
	}
}

// RebuildDerivedEvent recalculates the canonical identity and accounting of one
// typed event after a controlled mutation. It never interprets or executes event
// content.
func RebuildDerivedEvent(format SourceFormat, event Event) (Event, error) {
	if format == "" {
		return Event{}, errors.New("derived event requires a source format")
	}
	cloned, err := cloneEvent(event)
	if err != nil {
		return Event{}, err
	}
	if payloadCount(cloned) != 1 {
		return Event{}, errors.New("derived event must have exactly one typed payload")
	}
	if err := validateTracePayload(cloned); err != nil {
		return Event{}, err
	}
	encoded, err := json.Marshal(eventPayloadMaterial(cloned))
	if err != nil {
		return Event{}, fmt.Errorf("encode derived event payload: %w", err)
	}
	retainedBytes := mutableEventBytes(cloned)
	if retainedBytes == 0 {
		retainedBytes = len(encoded)
	}
	cloned.ContentBytes = retainedBytes
	cloned.RetainedBytes = retainedBytes
	cloned.EstimatedTokens = estimateTokensForBytes(retainedBytes)
	cloned.ContentDigest = digestBytes(encoded)
	cloned.ID = stableEventID(format, cloned.Source, cloned.Kind, cloned.ContentDigest)
	return cloned, nil
}

func validateChangedReferences(parentEvents, childEvents []Event, derivation Derivation) error {
	known := make(map[string]struct{}, len(parentEvents)+len(childEvents))
	for _, event := range parentEvents {
		known[event.ID] = struct{}{}
	}
	for _, event := range childEvents {
		known[event.ID] = struct{}{}
	}
	changed := make(map[string]struct{}, len(derivation.ChangedEventIDs))
	for _, eventID := range derivation.ChangedEventIDs {
		if _, exists := known[eventID]; !exists {
			return fmt.Errorf("derived trajectory references unknown changed event %q", eventID)
		}
		changed[eventID] = struct{}{}
	}
	for _, path := range derivation.ChangedFieldPaths {
		remainder := strings.TrimPrefix(string(path), "/events/")
		eventID, _, _ := strings.Cut(remainder, "/")
		if _, exists := changed[eventID]; !exists {
			return fmt.Errorf("derived trajectory field %q does not address a declared changed event", path)
		}
	}
	return nil
}

func payloadCount(event Event) int {
	values := []bool{
		event.Message != nil, event.ToolCall != nil, event.ToolResult != nil,
		event.Command != nil, event.Output != nil, event.FileChange != nil,
		event.Attachment != nil, event.Error != nil, event.Metadata != nil,
		event.Reasoning != nil, event.Contribution != nil, event.Evaluation != nil,
	}
	count := 0
	for _, present := range values {
		if present {
			count++
		}
	}
	return count
}

func validateLinks(links []Link, ids map[string]struct{}) error {
	adjacency := make(map[string][]string)
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		switch link.Kind {
		case LinkParent, LinkCallResult, LinkFileChange, LinkDerivation, LinkReference:
		default:
			return fmt.Errorf("unsupported link kind %q", link.Kind)
		}
		if _, ok := ids[link.FromID]; !ok {
			return fmt.Errorf("link %s has missing source %q", link.Kind, link.FromID)
		}
		if _, ok := ids[link.ToID]; !ok {
			return fmt.Errorf("link %s has missing target %q", link.Kind, link.ToID)
		}
		key := string(link.Kind) + "\x00" + link.FromID + "\x00" + link.ToID
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate link %s from %q to %q", link.Kind, link.FromID, link.ToID)
		}
		seen[key] = struct{}{}
		if link.Kind != LinkReference {
			adjacency[link.FromID] = append(adjacency[link.FromID], link.ToID)
		}
	}
	return rejectLinkCycles(adjacency)
}

func rejectLinkCycles(adjacency map[string][]string) error {
	const visiting = 1
	const visited = 2
	state := make(map[string]int)
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == visiting {
			return fmt.Errorf("causal link cycle reaches %q", id)
		}
		if state[id] == visited {
			return nil
		}
		state[id] = visiting
		for _, next := range adjacency[id] {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[id] = visited
		return nil
	}
	for id := range adjacency {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func finalizeTrajectory(trajectory *Trajectory) error {
	trajectory.SchemaVersion = CanonicalTrajectorySchema
	trajectory.Report.SchemaVersion = "evalwitness.ingestion-report.v1"
	trajectory.Report.SourceRecords = len(trajectory.Report.Records)
	trajectory.Report.AccountedRecords = len(trajectory.Report.Records)
	trajectory.Report.CanonicalEvents = len(trajectory.Events)
	trajectory.Report.Categories = categoryRetention(trajectory.Events)
	encoded, err := json.Marshal(trajectoryDigestMaterial(*trajectory))
	if err != nil {
		return fmt.Errorf("encode canonical trajectory: %w", err)
	}
	trajectory.Digest = digestBytes(encoded)
	return trajectory.Validate()
}

func trajectoryDigestMaterial(trajectory Trajectory) struct {
	SchemaVersion string
	SourceFormat  SourceFormat
	SourceDigest  string
	Events        []Event
	Links         []Link
	Derivation    *Derivation
} {
	return struct {
		SchemaVersion string
		SourceFormat  SourceFormat
		SourceDigest  string
		Events        []Event
		Links         []Link
		Derivation    *Derivation
	}{
		SchemaVersion: trajectory.SchemaVersion,
		SourceFormat:  trajectory.SourceFormat,
		SourceDigest:  trajectory.SourceDigest,
		Events:        trajectory.Events,
		Links:         trajectory.Links,
		Derivation:    trajectory.Derivation,
	}
}

func categoryRetention(events []Event) []CategoryRetention {
	byKind := make(map[EventKind]*CategoryRetention)
	for _, event := range events {
		entry := byKind[event.Kind]
		if entry == nil {
			entry = &CategoryRetention{Kind: event.Kind}
			byKind[event.Kind] = entry
		}
		entry.OriginalEvents++
		entry.OriginalBytes += event.ContentBytes
		if event.RetainedBytes > 0 || event.ContentBytes == 0 {
			entry.RetainedEvents++
		}
		entry.RetainedBytes += event.RetainedBytes
	}
	kinds := make([]string, 0, len(byKind))
	for kind := range byKind {
		kinds = append(kinds, string(kind))
	}
	sort.Strings(kinds)
	out := make([]CategoryRetention, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, *byKind[EventKind(kind)])
	}
	return out
}

func stableEventID(format SourceFormat, location SourceLocation, kind EventKind, digest string) string {
	material := string(format) + "\x00" + location.key() + "\x00" + string(kind) + "\x00" + digest
	return "evt_" + digestString(material)[:24]
}

func digestString(value string) string {
	return digestBytes([]byte(value))
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func estimateTokensForBytes(bytes int) int {
	if bytes <= 0 {
		return 0
	}
	return (bytes + 3) / 4
}
