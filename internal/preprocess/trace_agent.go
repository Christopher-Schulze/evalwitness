package preprocess

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	agentTraceUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	gitRevision    = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

type agentTraceRecord struct {
	Version   string                     `json:"version"`
	ID        string                     `json:"id"`
	Timestamp string                     `json:"timestamp"`
	VCS       *agentTraceVCS             `json:"vcs,omitempty"`
	Tool      *agentTraceTool            `json:"tool,omitempty"`
	Files     []agentTraceFile           `json:"files"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

type agentTraceVCS struct {
	Type     string `json:"type"`
	Revision string `json:"revision"`
}

type agentTraceTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type agentTraceFile struct {
	Path          string                   `json:"path"`
	Conversations []agentTraceConversation `json:"conversations"`
}

type agentTraceConversation struct {
	URL         string                 `json:"url,omitempty"`
	Contributor *agentTraceContributor `json:"contributor,omitempty"`
	Ranges      []agentTraceRange      `json:"ranges"`
	Related     []agentTraceRelated    `json:"related,omitempty"`
}

type agentTraceContributor struct {
	Type    string `json:"type"`
	ModelID string `json:"model_id,omitempty"`
}

type agentTraceRange struct {
	StartLine   int                    `json:"start_line"`
	EndLine     int                    `json:"end_line"`
	ContentHash string                 `json:"content_hash,omitempty"`
	Contributor *agentTraceContributor `json:"contributor,omitempty"`
}

type agentTraceRelated struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func importAgentTrace(raw []byte, options TraceImportOptions) (TraceImportResult, error) {
	if err := validateJSONNesting(raw, 64); err != nil {
		return TraceImportResult{}, err
	}
	var record agentTraceRecord
	if err := decodeSingleJSONStrict(raw, &record); err != nil {
		return TraceImportResult{}, fmt.Errorf("decode Agent Trace record: %w", err)
	}
	if err := validateAgentTrace(record); err != nil {
		return TraceImportResult{}, err
	}
	builder := newTrajectoryBuilder(SourceAgentTrace, raw, options.Ingest)
	location := SourceLocation{Record: 1, JSONPointer: "/", ByteEnd: int64(len(raw))}
	recordIndex := builder.beginRecord(location, "agent_trace_record", len(raw))
	metadataID := builder.addEvent(recordIndex, Event{
		Kind: EventMetadata, Source: location, SourceEventID: record.ID, Timestamp: record.Timestamp,
		Sensitivity: SensitivityPrivate,
		Metadata:    &MetadataPayload{Name: "agent_trace.record", Value: record.Version, ValueDigest: digestString(record.ID), Present: true},
	}, len(record.ID)+len(record.Version), len(record.Version)+64)
	report := MappingReport{SourceRecords: 1, Entries: []MappingEntry{}}
	report.add("/version", "/events/"+metadataID+"/metadata/value", MappingExact, "Agent Trace RFC version retained")
	report.add("/id", "/events/"+metadataID+"/source_event_id", MappingExact, "trace record identity retained")
	report.add("/timestamp", "/events/"+metadataID+"/timestamp", MappingExact, "RFC 3339 timestamp retained")
	if record.VCS != nil {
		report.add("/vcs/type", "/events/contribution/vcs", MappingNormalized, "record VCS identity replicated onto contribution events")
		report.add("/vcs/revision", "/events/contribution/revision", MappingNormalized, "record revision replicated onto contribution events")
	}
	if record.Tool != nil {
		report.add("/tool/name", "/envelope/agent/name", MappingExact, "Agent Trace tool name retained as capture agent identity")
		report.add("/tool/version", "/envelope/agent/version", MappingExact, "Agent Trace tool version retained as capture agent identity")
	}
	for fileIndex, file := range record.Files {
		pathDisposition := MappingRedacted
		pathTarget := "/events/contribution/path_alias"
		if options.Privacy.IncludesAttribution() {
			pathDisposition = MappingExact
			pathTarget = "/events/contribution/path"
		}
		report.add(fmt.Sprintf("/files/%d/path", fileIndex), pathTarget, pathDisposition, privacyReason(pathDisposition, "repository path"))
		for conversationIndex, conversation := range file.Conversations {
			conversationPointer := fmt.Sprintf("/files/%d/conversations/%d", fileIndex, conversationIndex)
			if conversation.URL != "" {
				urlDisposition := MappingRedacted
				urlTarget := "/events/contribution/conversation_digest"
				if options.Privacy.IncludesAttribution() {
					urlDisposition = MappingExact
					urlTarget = "/events/contribution/conversation_url"
				}
				report.add(conversationPointer+"/url", urlTarget, urlDisposition, privacyReason(urlDisposition, "conversation URL"))
			}
			if conversation.Contributor != nil {
				report.add(conversationPointer+"/contributor/type", "/events/contribution/contributor_type", MappingNormalized, "conversation contributor applies unless a range overrides it")
				if conversation.Contributor.ModelID != "" {
					report.add(conversationPointer+"/contributor/model_id", "/events/contribution/model_id", MappingNormalized, "conversation model applies unless a range overrides it")
				}
			}
			for rangeIndex, attributedRange := range conversation.Ranges {
				contributor := conversation.Contributor
				if attributedRange.Contributor != nil {
					contributor = attributedRange.Contributor
				}
				if contributor == nil {
					contributor = &agentTraceContributor{Type: "unknown"}
				}
				pointer := fmt.Sprintf("/files/%d/conversations/%d/ranges/%d", fileIndex, conversationIndex, rangeIndex)
				eventLocation := SourceLocation{Record: 1, Part: len(builder.trajectory.Events) + 1, JSONPointer: pointer}
				pathValue := ""
				conversationURL := ""
				if options.Privacy.IncludesAttribution() {
					pathValue = file.Path
					conversationURL = conversation.URL
				}
				payload := &ContributionPayload{
					Path: pathValue, PathAlias: aliasValue("path", file.Path),
					StartLine: attributedRange.StartLine, EndLine: attributedRange.EndLine,
					ContributorType: contributor.Type, ModelID: contributor.ModelID,
					ConversationURL: conversationURL, ConversationDigest: digestString(conversation.URL),
					ContentHash: attributedRange.ContentHash,
				}
				if record.VCS != nil {
					payload.VCS = record.VCS.Type
					payload.Revision = record.VCS.Revision
				}
				eventID := builder.addEvent(recordIndex, Event{
					Kind: EventContribution, Source: eventLocation, SourceEventID: record.ID, Timestamp: record.Timestamp,
					Sensitivity: SensitivityPrivate, Contribution: payload,
				}, len(file.Path)+len(conversation.URL)+len(attributedRange.ContentHash)+len(contributor.ModelID), contributionRetainedBytes(*payload))
				builder.addLink(LinkParent, metadataID, eventID)
				target := "/events/" + eventID + "/contribution"
				report.add(pointer+"/start_line", target+"/start_line", MappingExact, "one-indexed line retained")
				report.add(pointer+"/end_line", target+"/end_line", MappingExact, "one-indexed line retained")
				if attributedRange.ContentHash != "" {
					report.add(pointer+"/content_hash", target+"/content_hash", MappingExact, "opaque upstream hash retained")
				}
				if attributedRange.Contributor != nil {
					report.add(pointer+"/contributor/type", target+"/contributor_type", MappingExact, "range contributor override retained")
					if attributedRange.Contributor.ModelID != "" {
						report.add(pointer+"/contributor/model_id", target+"/model_id", MappingExact, "range model override retained")
					}
				} else if conversation.Contributor == nil {
					report.add(pointer+"/contributor", target+"/contributor_type", MappingSynthesized, "missing contributor normalized to explicit unknown")
				}
			}
			for relatedIndex := range conversation.Related {
				report.add(fmt.Sprintf("/files/%d/conversations/%d/related/%d", fileIndex, conversationIndex, relatedIndex), "", MappingUnsupported, "Agent Trace related resources have no canonical quality-evidence meaning")
			}
		}
	}
	for key := range record.Metadata {
		report.add("/metadata/"+escapeJSONPointer(key), "", MappingUnsupported, "vendor metadata is counted but not promoted into canonical evidence")
	}
	trajectory, err := builder.finish()
	if err != nil {
		return TraceImportResult{}, err
	}
	result := TraceImportResult{
		Trajectory: trajectory, Mapping: report,
		Envelope: TraceEnvelope{
			Source:          TraceSourceIdentity{Format: SourceAgentTrace, SchemaVersion: AgentTraceVersion, UpstreamCommit: AgentTraceUpstreamCommit, MediaType: sourceMediaType(SourceAgentTrace)},
			CaptureInterval: CaptureInterval{Start: record.Timestamp, End: record.Timestamp},
			PrivacyClass:    options.Privacy,
		},
	}
	if record.Tool != nil {
		result.Envelope.Agent = TraceAgentIdentity{Name: record.Tool.Name, Version: record.Tool.Version}
	}
	return result, finalizeTraceResult(&result)
}

func validateAgentTrace(record agentTraceRecord) error {
	switch {
	case record.Version != AgentTraceVersion:
		return fmt.Errorf("unsupported Agent Trace version %q; pinned version is %s", record.Version, AgentTraceVersion)
	case !agentTraceUUID.MatchString(record.ID):
		return fmt.Errorf("agent-trace id %q is not a supported UUID", record.ID)
	case len(record.Files) == 0:
		return fmt.Errorf("agent-trace record has no files")
	case len(record.Files) > 10_000:
		return fmt.Errorf("agent-trace file count %d exceeds 10000", len(record.Files))
	}
	if _, err := time.Parse(time.RFC3339Nano, record.Timestamp); err != nil {
		return fmt.Errorf("agent-trace timestamp: %w", err)
	}
	if record.Tool != nil && (strings.TrimSpace(record.Tool.Name) == "" || strings.TrimSpace(record.Tool.Version) == "") {
		return fmt.Errorf("agent-trace tool requires name and version under the pinned reference schema")
	}
	if record.VCS != nil {
		if !oneOf(record.VCS.Type, "git", "jj", "hg", "svn") || strings.TrimSpace(record.VCS.Revision) == "" {
			return fmt.Errorf("agent-trace vcs is invalid")
		}
		if record.VCS.Type == "git" && !gitRevision.MatchString(record.VCS.Revision) {
			return fmt.Errorf("agent-trace git revision must be 40 hexadecimal characters")
		}
	}
	ranges := 0
	for fileIndex, file := range record.Files {
		if err := validateRelativeTracePath(file.Path); err != nil {
			return fmt.Errorf("agent-trace files[%d].path: %w", fileIndex, err)
		}
		if len(file.Conversations) == 0 {
			return fmt.Errorf("agent-trace files[%d] has no conversations", fileIndex)
		}
		for conversationIndex, conversation := range file.Conversations {
			if conversation.URL != "" {
				if err := validateTraceURL(conversation.URL); err != nil {
					return fmt.Errorf("agent-trace files[%d].conversations[%d].url: %w", fileIndex, conversationIndex, err)
				}
			}
			if err := validateAgentContributor(conversation.Contributor); err != nil {
				return fmt.Errorf("agent-trace files[%d].conversations[%d].contributor: %w", fileIndex, conversationIndex, err)
			}
			if len(conversation.Ranges) == 0 {
				return fmt.Errorf("agent-trace files[%d].conversations[%d] has no ranges", fileIndex, conversationIndex)
			}
			for rangeIndex, attributedRange := range conversation.Ranges {
				ranges++
				if ranges > 100_000 {
					return fmt.Errorf("agent-trace range count exceeds 100000")
				}
				if attributedRange.StartLine < 1 || attributedRange.EndLine < attributedRange.StartLine {
					return fmt.Errorf("agent-trace range %d/%d/%d is invalid", fileIndex, conversationIndex, rangeIndex)
				}
				if err := validateAgentContributor(attributedRange.Contributor); err != nil {
					return fmt.Errorf("agent-trace range %d/%d/%d contributor: %w", fileIndex, conversationIndex, rangeIndex, err)
				}
			}
			for _, related := range conversation.Related {
				if strings.TrimSpace(related.Type) == "" {
					return fmt.Errorf("agent-trace related resource type is empty")
				}
				if err := validateTraceURL(related.URL); err != nil {
					return fmt.Errorf("agent-trace related resource: %w", err)
				}
			}
		}
	}
	return nil
}

func validateAgentContributor(contributor *agentTraceContributor) error {
	if contributor == nil {
		return nil
	}
	if !oneOf(contributor.Type, "human", "ai", "mixed", "unknown") {
		return fmt.Errorf("unsupported contributor type %q", contributor.Type)
	}
	if len(contributor.ModelID) > 250 {
		return fmt.Errorf("model_id exceeds 250 characters")
	}
	return nil
}

func validateRelativeTracePath(value string) error {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return fmt.Errorf("path must be a non-empty portable relative path")
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != value {
		return fmt.Errorf("path %q is not canonical and repository-relative", value)
	}
	return nil
}

func validateTraceURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("URL %q is not absolute", value)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("URL scheme %q is unsupported", parsed.Scheme)
	}
	return nil
}

func contributionRetainedBytes(payload ContributionPayload) int {
	return len(payload.Path) + len(payload.PathAlias) + len(payload.ContributorType) + len(payload.ModelID) +
		len(payload.ConversationURL) + len(payload.ConversationDigest) + len(payload.ContentHash) + len(payload.VCS) + len(payload.Revision)
}

func privacyReason(disposition MappingDisposition, subject string) string {
	if disposition == MappingExact {
		return subject + " retained under explicit attribution authorization"
	}
	return subject + " replaced by a digest or alias under metadata-only privacy"
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func escapeJSONPointer(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}
