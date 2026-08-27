package preprocess

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type otlpTracesData struct {
	ResourceSpans []json.RawMessage `json:"resourceSpans"`
}

type otlpResourceSpans struct {
	Resource   otlpResource      `json:"resource"`
	ScopeSpans []json.RawMessage `json:"scopeSpans"`
	SchemaURL  string            `json:"schemaUrl"`
}

type otlpResource struct {
	Attributes             []otlpAttribute   `json:"attributes"`
	DroppedAttributesCount uint32            `json:"droppedAttributesCount"`
	EntityRefs             []json.RawMessage `json:"entityRefs"`
}

type otlpScopeSpans struct {
	Scope     otlpInstrumentationScope `json:"scope"`
	Spans     []json.RawMessage        `json:"spans"`
	SchemaURL string                   `json:"schemaUrl"`
}

type otlpInstrumentationScope struct {
	Name                   string          `json:"name"`
	Version                string          `json:"version"`
	Attributes             []otlpAttribute `json:"attributes"`
	DroppedAttributesCount uint32          `json:"droppedAttributesCount"`
}

type otlpSpan struct {
	TraceID                string          `json:"traceId"`
	SpanID                 string          `json:"spanId"`
	TraceState             string          `json:"traceState"`
	ParentSpanID           string          `json:"parentSpanId"`
	Flags                  uint32          `json:"flags"`
	Name                   string          `json:"name"`
	Kind                   int             `json:"kind"`
	StartTimeUnixNano      json.RawMessage `json:"startTimeUnixNano"`
	EndTimeUnixNano        json.RawMessage `json:"endTimeUnixNano"`
	Attributes             []otlpAttribute `json:"attributes"`
	DroppedAttributesCount uint32          `json:"droppedAttributesCount"`
	Events                 []otlpSpanEvent `json:"events"`
	DroppedEventsCount     uint32          `json:"droppedEventsCount"`
	Links                  []otlpSpanLink  `json:"links"`
	DroppedLinksCount      uint32          `json:"droppedLinksCount"`
	Status                 otlpStatus      `json:"status"`
}

type otlpAttribute struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type otlpSpanEvent struct {
	TimeUnixNano           json.RawMessage `json:"timeUnixNano"`
	Name                   string          `json:"name"`
	Attributes             []otlpAttribute `json:"attributes"`
	DroppedAttributesCount uint32          `json:"droppedAttributesCount"`
}

type otlpSpanLink struct {
	TraceID                string          `json:"traceId"`
	SpanID                 string          `json:"spanId"`
	TraceState             string          `json:"traceState"`
	Attributes             []otlpAttribute `json:"attributes"`
	DroppedAttributesCount uint32          `json:"droppedAttributesCount"`
	Flags                  uint32          `json:"flags"`
}

type otlpStatus struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type pendingOTLPLink struct {
	kind        LinkKind
	fromTraceID string
	fromSpanID  string
	toEventID   string
	pointer     string
}

func importOTLPJSON(raw []byte, options TraceImportOptions) (TraceImportResult, error) {
	documents, err := splitOTLPDocuments(raw, options.Ingest.MaxRecordBytes)
	if err != nil {
		return TraceImportResult{}, err
	}
	builder := newTrajectoryBuilder(SourceOTLPJSON, raw, options.Ingest)
	report := MappingReport{Entries: []MappingEntry{}}
	primary := make(map[string]string)
	pending := make([]pendingOTLPLink, 0)
	seen := make(map[string]struct{})
	var earliest time.Time
	var latest time.Time
	agent := TraceAgentIdentity{}
	spanRecord := 0
	for documentIndex, document := range documents {
		if err := validateJSONNesting(document, 64); err != nil {
			return TraceImportResult{}, fmt.Errorf("OTLP document %d: %w", documentIndex+1, err)
		}
		var traces otlpTracesData
		if err := decodeSingleJSONStrict(document, &traces); err != nil {
			return TraceImportResult{}, fmt.Errorf("decode OTLP document %d: %w", documentIndex+1, err)
		}
		if len(traces.ResourceSpans) == 0 {
			return TraceImportResult{}, fmt.Errorf("OTLP document %d has no resourceSpans", documentIndex+1)
		}
		for resourceIndex, rawResource := range traces.ResourceSpans {
			var resource otlpResourceSpans
			if err := decodeSingleJSONStrict(rawResource, &resource); err != nil {
				return TraceImportResult{}, fmt.Errorf("decode OTLP resourceSpans[%d]: %w", resourceIndex, err)
			}
			resourceAttributes, err := decodeOTLPAttributes(resource.Resource.Attributes, 128)
			if err != nil {
				return TraceImportResult{}, fmt.Errorf("OTLP resourceSpans[%d].resource.attributes: %w", resourceIndex, err)
			}
			resourcePointer := fmt.Sprintf("/documents/%d/resourceSpans/%d/resource", documentIndex, resourceIndex)
			for key := range resourceAttributes {
				report.add(resourcePointer+"/attributes/"+escapeJSONPointer(key), "", MappingUnsupported, "resource attribute is counted but not promoted into verifier evidence")
			}
			if resource.Resource.DroppedAttributesCount > 0 {
				report.add(resourcePointer+"/droppedAttributesCount", "", MappingDropped, "upstream resource attributes were already dropped before import")
			}
			if len(resource.Resource.EntityRefs) > 128 {
				return TraceImportResult{}, fmt.Errorf("OTLP resourceSpans[%d] entity reference count exceeds 128", resourceIndex)
			}
			for entityIndex := range resource.Resource.EntityRefs {
				report.add(fmt.Sprintf("%s/entityRefs/%d", resourcePointer, entityIndex), "", MappingUnsupported, "OTLP resource entity reference has no canonical verifier-evidence mapping")
			}
			for scopeIndex, rawScope := range resource.ScopeSpans {
				var scope otlpScopeSpans
				if err := decodeSingleJSONStrict(rawScope, &scope); err != nil {
					return TraceImportResult{}, fmt.Errorf("decode OTLP scopeSpans[%d]: %w", scopeIndex, err)
				}
				scopeAttributes, err := decodeOTLPAttributes(scope.Scope.Attributes, 128)
				if err != nil {
					return TraceImportResult{}, fmt.Errorf("OTLP scopeSpans[%d].scope.attributes: %w", scopeIndex, err)
				}
				scopePointer := fmt.Sprintf("/documents/%d/resourceSpans/%d/scopeSpans/%d/scope", documentIndex, resourceIndex, scopeIndex)
				for key := range scopeAttributes {
					report.add(scopePointer+"/attributes/"+escapeJSONPointer(key), "", MappingUnsupported, "instrumentation-scope attribute has no canonical verifier-evidence mapping")
				}
				if scope.Scope.DroppedAttributesCount > 0 {
					report.add(scopePointer+"/droppedAttributesCount", "", MappingDropped, "upstream instrumentation-scope attributes were already dropped before import")
				}
				schemaURL := scope.SchemaURL
				if schemaURL == "" {
					schemaURL = resource.SchemaURL
				}
				if schemaURL != OTelSchemaURL {
					return TraceImportResult{}, fmt.Errorf("unsupported OpenTelemetry schema URL %q; pinned GenAI convention is %s", schemaURL, OTelSchemaURL)
				}
				for spanIndex, rawSpan := range scope.Spans {
					spanRecord++
					if spanRecord > 10_000 {
						return TraceImportResult{}, fmt.Errorf("OTLP span count exceeds 10000")
					}
					pointer := fmt.Sprintf("/documents/%d/resourceSpans/%d/scopeSpans/%d/spans/%d", documentIndex, resourceIndex, scopeIndex, spanIndex)
					spanResult, err := importOTLPSpan(builder, spanRecord, pointer, rawSpan, schemaURL, scope.Scope.Name, options.Privacy, &report)
					if err != nil {
						return TraceImportResult{}, err
					}
					key := spanResult.traceID + "\x00" + spanResult.spanID
					if _, exists := seen[key]; exists {
						return TraceImportResult{}, fmt.Errorf("repeated OTLP span identity trace=%s span=%s", spanResult.traceID, spanResult.spanID)
					}
					seen[key] = struct{}{}
					primary[key] = spanResult.primaryEventID
					pending = append(pending, spanResult.links...)
					if earliest.IsZero() || spanResult.start.Before(earliest) {
						earliest = spanResult.start
					}
					if latest.IsZero() || spanResult.end.After(latest) {
						latest = spanResult.end
					}
					if agent.Provider == "" {
						agent.Provider = spanResult.provider
					}
					if agent.Model == "" {
						agent.Model = spanResult.model
					}
					if agent.Name == "" {
						agent.Name = spanResult.agentName
					}
					if agent.ID == "" {
						agent.ID = spanResult.agentID
					}
					if agent.Version == "" {
						agent.Version = spanResult.agentVersion
					}
				}
			}
		}
	}
	if spanRecord == 0 {
		return TraceImportResult{}, fmt.Errorf("OTLP trace contains no spans")
	}
	report.SourceRecords = spanRecord
	for _, link := range pending {
		from := primary[link.fromTraceID+"\x00"+link.fromSpanID]
		if from == "" {
			report.add(link.pointer, "", MappingAmbiguous, "referenced parent or linked span is absent from the imported file")
			continue
		}
		builder.addLink(link.kind, from, link.toEventID)
		report.add(link.pointer, "/links", MappingExact, "causal parentage retained independently from timestamp order")
	}
	trajectory, err := builder.finish()
	if err != nil {
		return TraceImportResult{}, err
	}
	interval := CaptureInterval{Start: earliest.UTC().Format(time.RFC3339Nano), End: latest.UTC().Format(time.RFC3339Nano)}
	if err := validateCaptureInterval(interval); err != nil {
		return TraceImportResult{}, err
	}
	result := TraceImportResult{
		Trajectory: trajectory, Mapping: report,
		Envelope: TraceEnvelope{
			Source:          TraceSourceIdentity{Format: SourceOTLPJSON, SchemaVersion: sourceVersionFor(SourceOTLPJSON), MediaType: sourceMediaType(SourceOTLPJSON)},
			CaptureInterval: interval, Agent: agent, PrivacyClass: options.Privacy,
		},
	}
	return result, finalizeTraceResult(&result)
}

type importedOTLPSpan struct {
	traceID, spanID, primaryEventID string
	provider, model, agentName      string
	agentID, agentVersion           string
	start, end                      time.Time
	links                           []pendingOTLPLink
}

func importOTLPSpan(builder *trajectoryBuilder, recordNumber int, pointer string, raw json.RawMessage, schemaURL, instrumentation string, privacy PrivacyClass, report *MappingReport) (importedOTLPSpan, error) {
	var span otlpSpan
	if err := decodeSingleJSONStrict(raw, &span); err != nil {
		return importedOTLPSpan{}, fmt.Errorf("decode OTLP span at %s: %w", pointer, err)
	}
	if err := validateOTLPID(span.TraceID, 16, "traceId"); err != nil {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s: %w", pointer, err)
	}
	if err := validateOTLPID(span.SpanID, 8, "spanId"); err != nil {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s: %w", pointer, err)
	}
	if span.ParentSpanID != "" {
		if err := validateOTLPID(span.ParentSpanID, 8, "parentSpanId"); err != nil {
			return importedOTLPSpan{}, fmt.Errorf("OTLP span %s: %w", pointer, err)
		}
	}
	start, err := parseUnixNano(span.StartTimeUnixNano)
	if err != nil {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s startTimeUnixNano: %w", pointer, err)
	}
	end, err := parseUnixNano(span.EndTimeUnixNano)
	if err != nil {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s endTimeUnixNano: %w", pointer, err)
	}
	if end.Before(start) {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s ends before it starts", pointer)
	}
	if strings.TrimSpace(span.Name) == "" {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s name is empty", pointer)
	}
	if span.Kind < 0 || span.Kind > 5 {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s kind %d is invalid", pointer, span.Kind)
	}
	if span.Status.Code < 0 || span.Status.Code > 2 {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s status code %d is invalid", pointer, span.Status.Code)
	}
	attributes, err := decodeOTLPAttributes(span.Attributes, 256)
	if err != nil {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s attributes: %w", pointer, err)
	}
	if len(span.Events) > 10_000 {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s event count exceeds 10000", pointer)
	}
	if len(span.Links) > 10_000 {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s link count exceeds 10000", pointer)
	}
	operation := attributes["gen_ai.operation.name"]
	provider := attributes["gen_ai.provider.name"]
	model := attributes["gen_ai.response.model"]
	if model == "" {
		model = attributes["gen_ai.request.model"]
	}
	agentName := attributes["gen_ai.agent.name"]
	agentID := attributes["gen_ai.agent.id"]
	agentVersion := attributes["gen_ai.agent.version"]
	location := SourceLocation{Record: recordNumber, JSONPointer: pointer}
	recordIndex := builder.beginRecord(location, "otlp_span", len(raw))
	external := &ExternalTraceContext{
		TraceID: strings.ToLower(span.TraceID), SpanID: strings.ToLower(span.SpanID), ParentSpanID: strings.ToLower(span.ParentSpanID),
		OperationName: operation, ProviderName: provider, RequestModel: attributes["gen_ai.request.model"], ResponseModel: attributes["gen_ai.response.model"],
		ConversationID: attributes["gen_ai.conversation.id"], ResponseID: attributes["gen_ai.response.id"],
		FinishReasons: attributes["gen_ai.response.finish_reasons"], ToolType: attributes["gen_ai.tool.type"],
		AgentID: agentID, AgentName: agentName, AgentVersion: agentVersion, SchemaURL: schemaURL, Instrumentation: instrumentation,
	}
	primaryEventID := ""
	if operation == "execute_tool" {
		arguments := attributes["gen_ai.tool.call.arguments"]
		retainedArguments := ""
		if privacy.IncludesContent() {
			retainedArguments = arguments
		}
		primaryEventID = builder.addEvent(recordIndex, Event{
			Kind: EventToolCall, Source: location, SourceEventID: span.SpanID, Timestamp: start.UTC().Format(time.RFC3339Nano),
			Sensitivity: SensitivityPrivate, External: external,
			ToolCall: &ToolCallPayload{CallID: attributes["gen_ai.tool.call.id"], ToolName: attributes["gen_ai.tool.name"], Arguments: retainedArguments, ArgumentsDigest: digestString(arguments), Status: otlpStatusName(span.Status)},
		}, len(raw), len(attributes["gen_ai.tool.name"])+len(retainedArguments)+64)
		if resultValue, ok := attributes["gen_ai.tool.call.result"]; ok || span.Status.Code == 2 {
			retainedResult := ""
			if privacy.IncludesContent() {
				retainedResult = resultValue
			}
			resultLocation := location
			resultLocation.Part = 1
			resultLocation.JSONPointer += "/attributes/gen_ai.tool.call.result"
			resultID := builder.addEvent(recordIndex, Event{
				Kind: EventToolResult, Source: resultLocation, SourceEventID: span.SpanID, Timestamp: end.UTC().Format(time.RFC3339Nano),
				Sensitivity: SensitivityPrivate, External: external,
				ToolResult: &ToolResultPayload{CallID: attributes["gen_ai.tool.call.id"], Status: otlpStatusName(span.Status), Error: span.Status.Code == 2, Output: []ContentPart{{Kind: ContentText, Text: retainedResult, Digest: digestString(resultValue), Availability: contentAvailability(privacy)}}},
			}, len(resultValue), len(retainedResult)+64)
			builder.addLink(LinkCallResult, primaryEventID, resultID)
		}
	} else {
		value := strings.Trim(strings.Join([]string{operation, provider, model, span.Name}, "|"), "|")
		primaryEventID = builder.addEvent(recordIndex, Event{
			Kind: EventMetadata, Source: location, SourceEventID: span.SpanID, Timestamp: start.UTC().Format(time.RFC3339Nano),
			Sensitivity: SensitivityPrivate, External: external,
			Metadata: &MetadataPayload{Name: "otel.span", Value: value, ValueDigest: digestBytes(raw), Present: true},
		}, len(raw), len(value)+64)
	}
	report.add(pointer+"/traceId", "/events/"+primaryEventID+"/external_trace_context/trace_id", MappingExact, "OTLP trace identity retained")
	report.add(pointer+"/spanId", "/events/"+primaryEventID+"/external_trace_context/span_id", MappingExact, "OTLP span identity retained")
	report.add(pointer+"/name", "/events/"+primaryEventID+"/external_trace_context/operation_name", MappingNormalized, "span name and GenAI operation are retained separately")
	report.add(pointer+"/startTimeUnixNano", "/events/"+primaryEventID+"/timestamp", MappingNormalized, "OTLP nanoseconds normalized to RFC 3339")
	report.add(pointer+"/endTimeUnixNano", "/events/"+primaryEventID+"/timestamp", MappingNormalized, "span end retained on result or error events when representable")
	report.add(pointer+"/kind", "/events/"+primaryEventID+"/external_trace_context", MappingNormalized, "span kind validated as OTLP structure but canonical behavior derives from GenAI operation")
	if span.Status.Code != 0 {
		report.add(pointer+"/status/code", "/events", MappingNormalized, "OTLP status retained on canonical tool, result, or error evidence")
	}
	if span.Status.Message != "" {
		disposition := MappingRedacted
		reason := "OTLP status message retained by digest under metadata-only privacy"
		if privacy.IncludesContent() {
			disposition = MappingExact
			reason = "OTLP status message retained under explicit content authorization"
		}
		report.add(pointer+"/status/message", "/events", disposition, reason)
	}
	if span.TraceState != "" {
		report.add(pointer+"/traceState", "", MappingUnsupported, "W3C trace state is outside canonical verifier evidence")
	}
	if span.Flags != 0 {
		report.add(pointer+"/flags", "", MappingUnsupported, "OTLP span flags are outside canonical verifier evidence")
	}
	if span.DroppedAttributesCount > 0 {
		report.add(pointer+"/droppedAttributesCount", "", MappingDropped, "upstream span attributes were already dropped before import")
	}
	if span.DroppedEventsCount > 0 {
		report.add(pointer+"/droppedEventsCount", "", MappingDropped, "upstream span events were already dropped before import")
	}
	if span.DroppedLinksCount > 0 {
		report.add(pointer+"/droppedLinksCount", "", MappingDropped, "upstream span links were already dropped before import")
	}
	links := make([]pendingOTLPLink, 0, len(span.Links)+1)
	if span.ParentSpanID != "" {
		links = append(links, pendingOTLPLink{kind: LinkParent, fromTraceID: strings.ToLower(span.TraceID), fromSpanID: strings.ToLower(span.ParentSpanID), toEventID: primaryEventID, pointer: pointer + "/parentSpanId"})
	}
	for index, link := range span.Links {
		if err := validateOTLPID(link.TraceID, 16, "link traceId"); err != nil {
			return importedOTLPSpan{}, err
		}
		if err := validateOTLPID(link.SpanID, 8, "link spanId"); err != nil {
			return importedOTLPSpan{}, err
		}
		if _, err := decodeOTLPAttributes(link.Attributes, 128); err != nil {
			return importedOTLPSpan{}, fmt.Errorf("OTLP span %s link %d attributes: %w", pointer, index, err)
		}
		if link.TraceState != "" || len(link.Attributes) > 0 || link.Flags != 0 {
			report.add(fmt.Sprintf("%s/links/%d/context", pointer, index), "", MappingUnsupported, "link trace state, attributes, and flags are outside canonical verifier evidence")
		}
		if link.DroppedAttributesCount > 0 {
			report.add(fmt.Sprintf("%s/links/%d/droppedAttributesCount", pointer, index), "", MappingDropped, "upstream link attributes were already dropped before import")
		}
		links = append(links, pendingOTLPLink{kind: LinkReference, fromTraceID: strings.ToLower(link.TraceID), fromSpanID: strings.ToLower(link.SpanID), toEventID: primaryEventID, pointer: fmt.Sprintf("%s/links/%d", pointer, index)})
	}
	if err := importOTLPContent(builder, recordIndex, location, primaryEventID, span, attributes, external, privacy, report, pointer); err != nil {
		return importedOTLPSpan{}, err
	}
	if err := addOTLPUsage(builder, location, span.SpanID, provider, attributes); err != nil {
		return importedOTLPSpan{}, fmt.Errorf("OTLP span %s usage: %w", pointer, err)
	}
	accountOTLPAttributes(report, pointer, attributes, primaryEventID, privacy, span.Status)
	return importedOTLPSpan{traceID: strings.ToLower(span.TraceID), spanID: strings.ToLower(span.SpanID), primaryEventID: primaryEventID, provider: provider, model: model, agentName: agentName, agentID: agentID, agentVersion: agentVersion, start: start, end: end, links: links}, nil
}

func importOTLPContent(builder *trajectoryBuilder, recordIndex int, location SourceLocation, parentID string, span otlpSpan, attributes map[string]string, external *ExternalTraceContext, privacy PrivacyClass, report *MappingReport, pointer string) error {
	for _, content := range []struct{ key, role string }{{"gen_ai.input.messages", "user"}, {"gen_ai.output.messages", "assistant"}, {"gen_ai.system_instructions", "system"}} {
		value, exists := attributes[content.key]
		if !exists {
			continue
		}
		childLocation := location
		childLocation.Part = len(builder.trajectory.Events) + 1
		childLocation.JSONPointer += "/attributes/" + content.key
		if privacy.IncludesContent() {
			cleaned := builder.sanitize(recordIndex, childLocation.JSONPointer, value)
			childID := builder.addEvent(recordIndex, Event{
				Kind: EventMessage, Source: childLocation, SourceEventID: span.SpanID, Timestamp: eventTimestamp(span.StartTimeUnixNano),
				Sensitivity: SensitivityPrivate, External: external,
				Message: &MessagePayload{Role: content.role, Parts: []ContentPart{{Kind: ContentText, Text: cleaned}}},
			}, len(value), len(cleaned))
			builder.addLink(LinkParent, parentID, childID)
			report.add(pointer+"/attributes/"+content.key, "/events/"+childID+"/message", MappingNormalized, "structured GenAI content retained under explicit content authorization")
		} else {
			childID := builder.addEvent(recordIndex, Event{
				Kind: EventMetadata, Source: childLocation, SourceEventID: span.SpanID, Timestamp: eventTimestamp(span.StartTimeUnixNano),
				Sensitivity: SensitivityPrivate, External: external,
				Metadata: &MetadataPayload{Name: "otel.content_digest." + content.key, ValueDigest: digestString(value), Present: true},
			}, len(value), 64)
			builder.addLink(LinkParent, parentID, childID)
			report.add(pointer+"/attributes/"+content.key, "/events/"+childID+"/metadata/value_digest", MappingRedacted, "raw GenAI content excluded by metadata-only privacy")
		}
	}
	if span.Status.Code == 2 {
		message := ""
		if privacy.IncludesContent() {
			message = builder.sanitize(recordIndex, pointer+"/status/message", span.Status.Message)
		}
		errorLocation := location
		errorLocation.Part = len(builder.trajectory.Events) + 1
		errorLocation.JSONPointer += "/status"
		errorID := builder.addEvent(recordIndex, Event{
			Kind: EventError, Source: errorLocation, SourceEventID: span.SpanID, Timestamp: eventTimestamp(span.EndTimeUnixNano),
			Sensitivity: SensitivityPrivate, External: external,
			Error: &ErrorPayload{Class: attributes["error.type"], SafeMessage: message},
		}, len(span.Status.Message), len(message)+len(attributes["error.type"]))
		builder.addLink(LinkParent, parentID, errorID)
	}
	for index, event := range span.Events {
		if _, err := parseUnixNano(event.TimeUnixNano); err != nil {
			return fmt.Errorf("OTLP span event at %s/events/%d timeUnixNano: %w", pointer, index, err)
		}
		if strings.TrimSpace(event.Name) == "" {
			return fmt.Errorf("OTLP span event at %s/events/%d name is empty", pointer, index)
		}
		if _, err := decodeOTLPAttributes(event.Attributes, 256); err != nil {
			return fmt.Errorf("OTLP span event at %s/events/%d attributes: %w", pointer, index, err)
		}
		report.add(fmt.Sprintf("%s/events/%d", pointer, index), "", MappingUnsupported, "OTLP span events are not GenAI evaluation events; the pinned convention defines evaluation as an OTel Event carried by LogRecord.event_name")
	}
	return nil
}

func decodeOTLPAttributes(attributes []otlpAttribute, maximum int) (map[string]string, error) {
	if len(attributes) > maximum {
		return nil, fmt.Errorf("attribute count %d exceeds %d", len(attributes), maximum)
	}
	values := make(map[string]string, len(attributes))
	for _, attribute := range attributes {
		if strings.TrimSpace(attribute.Key) == "" {
			return nil, fmt.Errorf("attribute key is empty")
		}
		if _, exists := values[attribute.Key]; exists {
			return nil, fmt.Errorf("duplicate attribute %q", attribute.Key)
		}
		value, err := decodeOTLPAnyValue(attribute.Value)
		if err != nil {
			return nil, fmt.Errorf("attribute %q: %w", attribute.Key, err)
		}
		if len(value) > 1<<20 {
			return nil, fmt.Errorf("attribute %q exceeds 1 MiB", attribute.Key)
		}
		values[attribute.Key] = value
	}
	return values, nil
}

func decodeOTLPAnyValue(raw json.RawMessage) (string, error) {
	var value map[string]json.RawMessage
	if err := decodeSingleJSON(raw, &value); err != nil {
		return "", err
	}
	known := []string{"stringValue", "boolValue", "intValue", "doubleValue", "arrayValue", "kvlistValue", "bytesValue"}
	selected := ""
	for _, key := range known {
		if _, exists := value[key]; exists {
			if selected != "" {
				return "", fmt.Errorf("AnyValue contains multiple variants")
			}
			selected = key
		}
	}
	if selected == "" {
		return "", fmt.Errorf("AnyValue has no supported variant")
	}
	if len(value) != 1 {
		return "", fmt.Errorf("AnyValue contains an unknown field beside %s", selected)
	}
	switch selected {
	case "stringValue", "bytesValue":
		var output string
		if err := json.Unmarshal(value[selected], &output); err != nil {
			return "", err
		}
		return output, nil
	case "boolValue":
		var output bool
		if err := json.Unmarshal(value[selected], &output); err != nil {
			return "", err
		}
		return strconv.FormatBool(output), nil
	case "intValue":
		var number json.Number
		if err := json.Unmarshal(value[selected], &number); err == nil {
			return number.String(), nil
		}
		var output string
		if err := json.Unmarshal(value[selected], &output); err != nil {
			return "", err
		}
		if _, err := strconv.ParseInt(output, 10, 64); err != nil {
			return "", err
		}
		return output, nil
	case "doubleValue":
		var output float64
		if err := json.Unmarshal(value[selected], &output); err != nil || math.IsNaN(output) || math.IsInf(output, 0) {
			return "", fmt.Errorf("doubleValue must be finite")
		}
		return strconv.FormatFloat(output, 'g', -1, 64), nil
	default:
		return compactJSON(value[selected]), nil
	}
}

func splitOTLPDocuments(raw []byte, maximumRecord int) ([][]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	var whole otlpTracesData
	if json.Unmarshal(trimmed, &whole) == nil && len(whole.ResourceSpans) > 0 {
		return [][]byte{trimmed}, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), maximumRecord)
	documents := make([][]byte, 0)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			return nil, fmt.Errorf("malformed OTLP JSONL record %d", len(documents)+1)
		}
		documents = append(documents, append([]byte(nil), line...))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read OTLP JSONL: %w", err)
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("OTLP JSONL contains no records")
	}
	return documents, nil
}

func validateOTLPID(value string, bytesLength int, field string) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != bytesLength {
		return fmt.Errorf("%s must be %d hexadecimal bytes", field, bytesLength)
	}
	allZero := true
	for _, current := range decoded {
		allZero = allZero && current == 0
	}
	if allZero {
		return fmt.Errorf("%s must not be all zero", field)
	}
	return nil
}

func parseUnixNano(raw json.RawMessage) (time.Time, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		var number json.Number
		if err := json.Unmarshal(raw, &number); err != nil {
			return time.Time{}, fmt.Errorf("must be a decimal string or number")
		}
		value = number.String()
	}
	nanoseconds, err := strconv.ParseUint(value, 10, 63)
	if err != nil || nanoseconds == 0 {
		return time.Time{}, fmt.Errorf("invalid positive signed-64 nanoseconds %q", value)
	}
	return time.Unix(0, int64(nanoseconds)).UTC(), nil
}

func eventTimestamp(raw json.RawMessage) string {
	value, err := parseUnixNano(raw)
	if err != nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func otlpStatusName(status otlpStatus) string {
	switch status.Code {
	case 1:
		return "ok"
	case 2:
		return "error"
	default:
		return "unset"
	}
}

func contentAvailability(privacy PrivacyClass) string {
	if privacy.IncludesContent() {
		return "content_authorized"
	}
	return "metadata_only"
}

func addOTLPUsage(builder *trajectoryBuilder, source SourceLocation, sourceID, provider string, attributes map[string]string) error {
	keys := []string{
		"gen_ai.usage.input_tokens", "gen_ai.usage.output_tokens", "gen_ai.usage.reasoning.output_tokens",
		"gen_ai.usage.cache_read.input_tokens", "gen_ai.usage.cache_creation.input_tokens",
	}
	values := make([]int, len(keys))
	observedFields := make([]string, 0, len(keys))
	observed := false
	for index, key := range keys {
		raw, exists := attributes[key]
		if !exists {
			continue
		}
		observed = true
		observedFields = append(observedFields, key)
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return fmt.Errorf("%s must be a non-negative platform integer", key)
		}
		values[index] = value
	}
	if !observed {
		return nil
	}
	builder.addProviderUsage(ProviderTokenUsage{
		Provider: provider, Scope: "otel_span", SourceEventID: sourceID, Source: source,
		ObservedFields: observedFields,
		InputTokens:    values[0], OutputTokens: values[1], ReasoningTokens: values[2],
		CachedInputTokens: values[3], CacheCreationInputTokens: values[4],
	})
	return nil
}

func accountOTLPAttributes(report *MappingReport, pointer string, attributes map[string]string, eventID string, privacy PrivacyClass, status otlpStatus) {
	for key := range attributes {
		if key == "gen_ai.input.messages" || key == "gen_ai.output.messages" || key == "gen_ai.system_instructions" {
			continue
		}
		target := "/events/" + eventID
		disposition := MappingUnsupported
		reason := "attribute is outside the pinned verifier-relevant GenAI mapping"
		switch key {
		case "gen_ai.operation.name", "gen_ai.provider.name", "gen_ai.request.model", "gen_ai.response.model",
			"gen_ai.agent.name", "gen_ai.agent.id", "gen_ai.agent.version", "gen_ai.conversation.id",
			"gen_ai.response.id", "gen_ai.tool.type":
			disposition = MappingExact
			target += "/external_trace_context"
			reason = "pinned OpenTelemetry GenAI identity retained"
		case "gen_ai.response.finish_reasons":
			disposition = MappingNormalized
			target += "/external_trace_context/finish_reasons"
			reason = "finish-reason AnyValue retained as deterministic text"
		case "gen_ai.tool.name", "gen_ai.tool.call.id", "gen_ai.tool.call.arguments":
			disposition = MappingExact
			reason = "pinned OpenTelemetry tool attribute retained"
		case "gen_ai.tool.call.result":
			disposition = MappingExact
			target = "/events"
			reason = "pinned OpenTelemetry tool result retained as a linked canonical event"
		case "gen_ai.usage.input_tokens", "gen_ai.usage.output_tokens", "gen_ai.usage.reasoning.output_tokens",
			"gen_ai.usage.cache_read.input_tokens", "gen_ai.usage.cache_creation.input_tokens":
			disposition = MappingNormalized
			target = "/report/provider_usage"
			reason = "token count retained as a typed provider-usage observation"
		case "error.type":
			if status.Code == 2 {
				disposition = MappingExact
				target = "/events"
				reason = "error type retained on the canonical error event"
			} else {
				disposition = MappingAmbiguous
				target = ""
				reason = "error.type is present without OTLP error status"
			}
		}
		if (key == "gen_ai.tool.call.arguments" || key == "gen_ai.tool.call.result") && !privacy.IncludesContent() {
			disposition = MappingRedacted
			reason = "sensitive tool content retained by digest only"
		}
		report.add(pointer+"/attributes/"+escapeJSONPointer(key), target, disposition, reason)
	}
}
