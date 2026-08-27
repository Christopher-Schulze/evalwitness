package preprocess

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func ExportAgentTrace(result TraceImportResult, evidence *VerifierEvidenceReference) (TraceExportResult, error) {
	if err := result.Trajectory.Validate(); err != nil {
		return TraceExportResult{}, err
	}
	if !result.Envelope.PrivacyClass.IncludesAttribution() {
		return TraceExportResult{}, fmt.Errorf("agent-trace export requires attribution_authorized or content_and_attribution_authorized privacy")
	}
	record := agentTraceRecord{Version: AgentTraceVersion, Files: []agentTraceFile{}}
	if agentTraceUUID.MatchString(firstSourceEventID(result.Trajectory.Events)) {
		record.ID = firstSourceEventID(result.Trajectory.Events)
	} else {
		record.ID = deterministicUUID(result.Trajectory.Digest)
	}
	record.Timestamp = result.Envelope.CaptureInterval.End
	if record.Timestamp == "" {
		record.Timestamp = result.Envelope.CaptureInterval.Start
	}
	if record.Timestamp == "" {
		return TraceExportResult{}, fmt.Errorf("agent-trace export requires a source capture timestamp")
	}
	if result.Envelope.Agent.Name != "" || result.Envelope.Agent.Version != "" {
		if result.Envelope.Agent.Name == "" || result.Envelope.Agent.Version == "" {
			return TraceExportResult{}, fmt.Errorf("agent-trace export requires both tool name and version")
		}
		record.Tool = &agentTraceTool{Name: result.Envelope.Agent.Name, Version: result.Envelope.Agent.Version}
	}
	type conversationKey struct{ path, url, contributorType, modelID, vcs, revision string }
	groups := make(map[conversationKey][]agentTraceRange)
	report := MappingReport{SourceRecords: len(result.Trajectory.Events) + len(result.Trajectory.Report.ProviderUsage), Entries: []MappingEntry{}}
	for _, event := range result.Trajectory.Events {
		if event.Contribution == nil {
			report.add("/events/"+event.ID, "", MappingUnsupported, "canonical event is not Agent Trace attribution and is not inserted into the attribution record")
			continue
		}
		contribution := event.Contribution
		if contribution.Path == "" {
			return TraceExportResult{}, fmt.Errorf("contribution event %q has no authorized repository path", event.ID)
		}
		key := conversationKey{path: contribution.Path, url: contribution.ConversationURL, contributorType: contribution.ContributorType, modelID: contribution.ModelID, vcs: contribution.VCS, revision: contribution.Revision}
		groups[key] = append(groups[key], agentTraceRange{StartLine: contribution.StartLine, EndLine: contribution.EndLine, ContentHash: contribution.ContentHash})
		report.add("/events/"+event.ID+"/contribution", "/files", MappingExact, "Agent Trace attribution semantics retained")
	}
	if len(groups) == 0 {
		return TraceExportResult{}, fmt.Errorf("trajectory contains no Agent Trace contribution events")
	}
	for usageIndex := range result.Trajectory.Report.ProviderUsage {
		report.add(fmt.Sprintf("/report/provider_usage/%d", usageIndex), "", MappingUnsupported, "Agent Trace is attribution-only and cannot represent runtime token usage")
	}
	keys := make([]conversationKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprintf("%s\x00%s\x00%s\x00%s", keys[i].path, keys[i].url, keys[i].contributorType, keys[i].modelID) < fmt.Sprintf("%s\x00%s\x00%s\x00%s", keys[j].path, keys[j].url, keys[j].contributorType, keys[j].modelID)
	})
	files := make(map[string][]agentTraceConversation)
	for _, key := range keys {
		ranges := groups[key]
		sort.Slice(ranges, func(i, j int) bool {
			if ranges[i].StartLine == ranges[j].StartLine {
				return ranges[i].EndLine < ranges[j].EndLine
			}
			return ranges[i].StartLine < ranges[j].StartLine
		})
		conversation := agentTraceConversation{URL: key.url, Contributor: &agentTraceContributor{Type: key.contributorType, ModelID: key.modelID}, Ranges: ranges}
		files[key.path] = append(files[key.path], conversation)
		if key.vcs != "" {
			candidate := &agentTraceVCS{Type: key.vcs, Revision: key.revision}
			if record.VCS == nil {
				record.VCS = candidate
			} else if *record.VCS != *candidate {
				return TraceExportResult{}, fmt.Errorf("agent-trace export cannot represent multiple VCS revisions in one record")
			}
		}
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		record.Files = append(record.Files, agentTraceFile{Path: path, Conversations: files[path]})
	}
	if evidence != nil {
		if !canonicalSHA256(evidence.ProtocolRunDigest) {
			return TraceExportResult{}, fmt.Errorf("verifier evidence reference protocol_run_digest must be a lowercase sha256 digest")
		}
		if evidence.DecisionDigest != "" && !canonicalSHA256(evidence.DecisionDigest) {
			return TraceExportResult{}, fmt.Errorf("verifier evidence reference decision_digest must be a lowercase sha256 digest")
		}
		if len(evidence.AuditCaseID) > 250 {
			return TraceExportResult{}, fmt.Errorf("verifier evidence reference audit_case_id exceeds 250 characters")
		}
		raw, err := json.Marshal(evidence)
		if err != nil {
			return TraceExportResult{}, err
		}
		record.Metadata = map[string]json.RawMessage{"org.evalwitness.verifier_evidence.v1": raw}
		report.add("/verifier_evidence", "/metadata/org.evalwitness.verifier_evidence.v1", MappingNormalized, "quality evidence remains a namespaced reference outside Agent Trace attribution semantics")
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return TraceExportResult{}, fmt.Errorf("encode Agent Trace export: %w", err)
	}
	if err := validateAgentTrace(record); err != nil {
		return TraceExportResult{}, fmt.Errorf("validate Agent Trace export: %w", err)
	}
	return finalizeTraceExport(SourceAgentTrace, AgentTraceVersion, encoded, report)
}

func canonicalSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func ExportOTLPJSON(result TraceImportResult, privacy PrivacyClass) (TraceExportResult, error) {
	if err := result.Trajectory.Validate(); err != nil {
		return TraceExportResult{}, err
	}
	if _, err := ParsePrivacyClass(string(privacy)); err != nil {
		return TraceExportResult{}, err
	}
	parent := make(map[string]string)
	for _, link := range result.Trajectory.Links {
		if link.Kind == LinkParent {
			if _, exists := parent[link.ToID]; !exists {
				parent[link.ToID] = link.FromID
			}
		}
	}
	spanIDByEvent := make(map[string]string, len(result.Trajectory.Events))
	for _, event := range result.Trajectory.Events {
		if event.External != nil {
			spanIDByEvent[event.ID] = event.External.SpanID
		} else {
			spanIDByEvent[event.ID] = digestString(event.ID)[:16]
		}
	}
	spans := make([]map[string]any, 0, len(result.Trajectory.Events))
	report := MappingReport{SourceRecords: len(result.Trajectory.Events) + len(result.Trajectory.Report.ProviderUsage), Entries: []MappingEntry{}}
	externalSpanIndex := make(map[string]int)
	mappedUsage := make([]bool, len(result.Trajectory.Report.ProviderUsage))
	for _, event := range result.Trajectory.Events {
		if event.Kind == EventEvaluation {
			report.add("/events/"+event.ID, "", MappingUnsupported, "OpenTelemetry GenAI evaluation events are log events; they are not misrepresented as trace spans")
			continue
		}
		spanID := spanIDByEvent[event.ID]
		traceID := result.Trajectory.Digest[:32]
		if event.External != nil {
			traceID = event.External.TraceID
			key := event.External.TraceID + "\x00" + event.External.SpanID
			if spanIndex, exists := externalSpanIndex[key]; exists {
				span := spans[spanIndex]
				attributes, _ := span["attributes"].([]map[string]any)
				span["attributes"] = append(attributes, otelEventAttributes(event, privacy)...)
				if timestamp := otlpNanoTimestamp(event.Timestamp); timestamp != "" {
					span["endTimeUnixNano"] = timestamp
				}
				if otelEventStatus(event) == 2 {
					span["status"] = map[string]any{"code": 2}
				}
				disposition := MappingNormalized
				reason := "canonical events originating from one OTLP span were semantically coalesced"
				if eventContainsContent(event) && !privacy.IncludesContent() {
					disposition = MappingRedacted
					reason = "canonical event coalesced while opt-in content remained excluded"
				}
				report.add("/events/"+event.ID, fmt.Sprintf("/resourceSpans/0/scopeSpans/0/spans/%d", spanIndex), disposition, reason)
				continue
			}
			externalSpanIndex[key] = len(spans)
		}
		operation, name := otelOperation(event)
		attributes := []map[string]any{
			otlpStringAttribute("gen_ai.operation.name", operation),
			otlpStringAttribute("org.evalwitness.trace.event.id", event.ID),
			otlpStringAttribute("org.evalwitness.trace.trajectory.digest", result.Trajectory.Digest),
			otlpStringAttribute("org.evalwitness.trace.mapping.digest", result.Mapping.Digest),
			otlpStringAttribute("org.evalwitness.trace.content.digest", event.ContentDigest),
			otlpStringAttribute("org.evalwitness.trace.source.format", string(result.Trajectory.SourceFormat)),
		}
		if event.External != nil {
			attributes = append(attributes, otelExternalAttributes(*event.External)...)
		} else {
			if result.Envelope.Agent.Provider != "" {
				attributes = append(attributes, otlpStringAttribute("gen_ai.provider.name", result.Envelope.Agent.Provider))
			}
			if result.Envelope.Agent.Model != "" {
				attributes = append(attributes, otlpStringAttribute("gen_ai.request.model", result.Envelope.Agent.Model))
			}
		}
		attributes = append(attributes, otelEventAttributes(event, privacy)...)
		for usageIndex, usage := range result.Trajectory.Report.ProviderUsage {
			if mappedUsage[usageIndex] || usage.SourceEventID == "" || usage.SourceEventID != event.SourceEventID {
				continue
			}
			usageAttributes := otelUsageAttributes(usage)
			if len(usageAttributes) == 0 {
				continue
			}
			attributes = append(attributes, usageAttributes...)
			mappedUsage[usageIndex] = true
			report.add(fmt.Sprintf("/report/provider_usage/%d", usageIndex), "/resourceSpans/0/scopeSpans/0/spans/attributes", MappingNormalized, "typed provider usage mapped to pinned OpenTelemetry GenAI token attributes")
			break
		}
		span := map[string]any{"traceId": traceID, "spanId": spanID, "name": name, "kind": 1, "attributes": attributes, "status": map[string]any{"code": otelEventStatus(event)}}
		if parentID := parent[event.ID]; parentID != "" {
			span["parentSpanId"] = spanIDByEvent[parentID]
		}
		if timestamp := otlpNanoTimestamp(event.Timestamp); timestamp != "" {
			span["startTimeUnixNano"] = timestamp
			span["endTimeUnixNano"] = timestamp
		}
		spans = append(spans, span)
		disposition := MappingNormalized
		reason := "canonical event mapped to covered OpenTelemetry GenAI span semantics"
		if event.External == nil {
			disposition = MappingSynthesized
			reason = "OTLP trace and span identities synthesized deterministically; no conversation identifier fabricated"
		} else if eventContainsContent(event) && !privacy.IncludesContent() {
			disposition = MappingRedacted
			reason = "covered OTLP span semantics retained while opt-in content remained excluded"
		}
		report.add("/events/"+event.ID, "/resourceSpans/0/scopeSpans/0/spans", disposition, reason)
	}
	for usageIndex, mapped := range mappedUsage {
		if !mapped {
			report.add(fmt.Sprintf("/report/provider_usage/%d", usageIndex), "", MappingUnsupported, "provider usage has no unique source event for OTLP span export")
		}
	}
	if len(spans) == 0 {
		return TraceExportResult{}, fmt.Errorf("trajectory has no events representable as OpenTelemetry spans")
	}
	document := map[string]any{"resourceSpans": []any{map[string]any{
		"resource":   map[string]any{"attributes": []any{otlpStringAttribute("service.name", "evalwitness-trace-export")}},
		"schemaUrl":  OTelSchemaURL,
		"scopeSpans": []any{map[string]any{"scope": map[string]any{"name": "org.evalwitness.tracebridge", "version": TraceMappingPolicyVersion}, "schemaUrl": OTelSchemaURL, "spans": spans}},
	}}}
	encoded, err := json.Marshal(document)
	if err != nil {
		return TraceExportResult{}, fmt.Errorf("encode OTLP JSON export: %w", err)
	}
	return finalizeTraceExport(SourceOTLPJSON, sourceVersionFor(SourceOTLPJSON), encoded, report)
}

func otelExternalAttributes(external ExternalTraceContext) []map[string]any {
	values := []struct{ key, value string }{
		{"gen_ai.provider.name", external.ProviderName},
		{"gen_ai.request.model", external.RequestModel},
		{"gen_ai.response.model", external.ResponseModel},
		{"gen_ai.conversation.id", external.ConversationID},
		{"gen_ai.response.id", external.ResponseID},
		{"gen_ai.response.finish_reasons", external.FinishReasons},
		{"gen_ai.tool.type", external.ToolType},
		{"gen_ai.agent.id", external.AgentID},
		{"gen_ai.agent.name", external.AgentName},
		{"gen_ai.agent.version", external.AgentVersion},
	}
	attributes := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if value.value != "" {
			attributes = append(attributes, otlpStringAttribute(value.key, value.value))
		}
	}
	return attributes
}

func otelUsageAttributes(usage ProviderTokenUsage) []map[string]any {
	values := []struct {
		key   string
		value int
	}{
		{"gen_ai.usage.input_tokens", usage.InputTokens},
		{"gen_ai.usage.output_tokens", usage.OutputTokens},
		{"gen_ai.usage.reasoning.output_tokens", usage.ReasoningTokens},
		{"gen_ai.usage.cache_read.input_tokens", usage.CachedInputTokens},
		{"gen_ai.usage.cache_creation.input_tokens", usage.CacheCreationInputTokens},
	}
	attributes := make([]map[string]any, 0, len(values))
	observed := make(map[string]struct{}, len(usage.ObservedFields))
	for _, field := range usage.ObservedFields {
		observed[field] = struct{}{}
	}
	for _, value := range values {
		_, explicitlyObserved := observed[value.key]
		if (len(observed) > 0 && !explicitlyObserved) || (len(observed) == 0 && value.value == 0) {
			continue
		}
		attributes = append(attributes, map[string]any{"key": value.key, "value": map[string]any{"intValue": strconv.Itoa(value.value)}})
	}
	return attributes
}

func eventContainsContent(event Event) bool {
	if event.ToolCall != nil && event.ToolCall.Arguments != "" {
		return true
	}
	if event.ToolResult != nil {
		for _, parts := range [][]ContentPart{event.ToolResult.Stdout, event.ToolResult.Stderr, event.ToolResult.Output} {
			for _, part := range parts {
				if part.Text != "" {
					return true
				}
			}
		}
	}
	if event.Message != nil {
		for _, part := range event.Message.Parts {
			if part.Text != "" {
				return true
			}
		}
	}
	return false
}

func finalizeTraceExport(format SourceFormat, schemaVersion string, encoded []byte, report MappingReport) (TraceExportResult, error) {
	if err := validateMappingAccounting(report); err != nil {
		return TraceExportResult{}, err
	}
	report.SchemaVersion = TraceMappingReportSchema
	report.PolicyVersion = TraceMappingPolicyVersion
	report.Lossless = report.Totals.Normalized == 0 && report.Totals.Synthesized == 0 && report.Totals.Redacted == 0 && report.Totals.Unsupported == 0 && report.Totals.Ambiguous == 0 && report.Totals.Dropped == 0
	sort.Slice(report.Entries, func(i, j int) bool { return report.Entries[i].SourcePath < report.Entries[j].SourcePath })
	material := report
	material.Digest = ""
	raw, err := json.Marshal(material)
	if err != nil {
		return TraceExportResult{}, err
	}
	report.Digest = digestBytes(raw)
	return TraceExportResult{Format: format, SchemaVersion: schemaVersion, ArtifactDigest: digestBytes(encoded), Mapping: report, Bytes: encoded}, nil
}

func firstSourceEventID(events []Event) string {
	for _, event := range events {
		if event.SourceEventID != "" {
			return event.SourceEventID
		}
	}
	return ""
}

func deterministicUUID(digest string) string {
	value := []byte(digest[:32])
	value[12] = '5'
	value[16] = '8'
	return string(value[:8]) + "-" + string(value[8:12]) + "-" + string(value[12:16]) + "-" + string(value[16:20]) + "-" + string(value[20:32])
}

func otlpStringAttribute(key, value string) map[string]any {
	return map[string]any{"key": key, "value": map[string]any{"stringValue": value}}
}

func otelOperation(event Event) (string, string) {
	switch event.Kind {
	case EventToolCall, EventToolResult, EventCommand, EventOutput:
		name := "execute_tool"
		if event.ToolCall != nil && event.ToolCall.ToolName != "" {
			name += " " + event.ToolCall.ToolName
		}
		return "execute_tool", name
	case EventMessage:
		return "chat", "chat"
	default:
		return "invoke_agent", "invoke_agent"
	}
}

func otelEventAttributes(event Event, privacy PrivacyClass) []map[string]any {
	attributes := make([]map[string]any, 0)
	if event.ToolCall != nil {
		attributes = append(attributes, otlpStringAttribute("gen_ai.tool.name", event.ToolCall.ToolName))
		if event.ToolCall.CallID != "" {
			attributes = append(attributes, otlpStringAttribute("gen_ai.tool.call.id", event.ToolCall.CallID))
		}
		if privacy.IncludesContent() && event.ToolCall.Arguments != "" {
			attributes = append(attributes, otlpStringAttribute("gen_ai.tool.call.arguments", event.ToolCall.Arguments))
		}
	}
	if event.ToolResult != nil && privacy.IncludesContent() {
		parts := make([]string, 0, len(event.ToolResult.Stdout)+len(event.ToolResult.Stderr)+len(event.ToolResult.Output))
		for _, part := range event.ToolResult.Stdout {
			if part.Text != "" {
				parts = append(parts, "[stdout]\n"+part.Text)
			}
		}
		for _, part := range event.ToolResult.Stderr {
			if part.Text != "" {
				parts = append(parts, "[stderr]\n"+part.Text)
			}
		}
		for _, part := range event.ToolResult.Output {
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
		if len(parts) > 0 {
			attributes = append(attributes, otlpStringAttribute("gen_ai.tool.call.result", strings.Join(parts, "\n")))
		}
	}
	if event.Message != nil && privacy.IncludesContent() {
		parts := make([]string, 0, len(event.Message.Parts))
		for _, part := range event.Message.Parts {
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
		if len(parts) > 0 {
			key := "gen_ai.input.messages"
			if event.Message.Role == "assistant" {
				key = "gen_ai.output.messages"
			}
			attributes = append(attributes, otlpStringAttribute(key, strings.Join(parts, "\n")))
		}
	}
	return attributes
}

func otelEventStatus(event Event) int {
	if event.Error != nil || event.ToolResult != nil && event.ToolResult.Error {
		return 2
	}
	return 1
}

func otlpNanoTimestamp(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.UnixNano() <= 0 {
		return ""
	}
	return strconv.FormatInt(parsed.UnixNano(), 10)
}
