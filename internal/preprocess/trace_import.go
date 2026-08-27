package preprocess

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

func ImportTraceReader(reader io.Reader, options TraceImportOptions) (TraceImportResult, error) {
	var err error
	options.Ingest, err = normalizeIngestOptions(options.Ingest)
	if err != nil {
		return TraceImportResult{}, err
	}
	if options.Privacy == "" {
		options.Privacy = PrivacyMetadataOnly
	}
	if _, err := ParsePrivacyClass(string(options.Privacy)); err != nil {
		return TraceImportResult{}, err
	}
	raw, err := readBounded(reader, options.Ingest.MaxSourceBytes)
	if err != nil {
		return TraceImportResult{}, err
	}
	return ImportTraceBytes(raw, options)
}

func ImportTraceBytes(raw []byte, options TraceImportOptions) (TraceImportResult, error) {
	var err error
	options.Ingest, err = normalizeIngestOptions(options.Ingest)
	if err != nil {
		return TraceImportResult{}, err
	}
	if int64(len(raw)) > options.Ingest.MaxSourceBytes {
		return TraceImportResult{}, fmt.Errorf("source exceeds %d-byte limit", options.Ingest.MaxSourceBytes)
	}
	if options.Privacy == "" {
		options.Privacy = PrivacyMetadataOnly
	}
	if _, err := ParsePrivacyClass(string(options.Privacy)); err != nil {
		return TraceImportResult{}, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return TraceImportResult{}, errors.New("trace source is empty")
	}
	if len(trimmed) > options.Ingest.MaxRecordBytes && bytes.Count(trimmed, []byte{'\n'}) == 0 {
		return TraceImportResult{}, fmt.Errorf("trace record exceeds %d-byte limit", options.Ingest.MaxRecordBytes)
	}
	format, err := detectTraceFormat(trimmed)
	if err != nil {
		return TraceImportResult{}, err
	}
	switch format {
	case SourceOTLPJSON:
		return importOTLPJSON(trimmed, options)
	case SourceAgentTrace:
		return importAgentTrace(trimmed, options)
	default:
		trajectory, err := ingestBytes(raw, options.Ingest)
		if err != nil {
			return TraceImportResult{}, err
		}
		return vendorTraceResult(trajectory, options.Privacy)
	}
}

func detectTraceFormat(raw []byte) (SourceFormat, error) {
	if raw[0] == '[' {
		return "", nil
	}
	var marker struct {
		Version       string          `json:"version"`
		Files         json.RawMessage `json:"files"`
		ResourceSpans json.RawMessage `json:"resourceSpans"`
		Data          json.RawMessage `json:"data"`
	}
	markerRaw := firstJSONRecord(raw)
	if json.Valid(raw) {
		markerRaw = raw
	}
	if err := json.Unmarshal(markerRaw, &marker); err != nil {
		return "", nil
	}
	if marker.Version != "" && len(marker.Files) > 0 {
		return SourceAgentTrace, nil
	}
	if len(marker.ResourceSpans) > 0 {
		return SourceOTLPJSON, nil
	}
	if len(marker.Data) > 0 && bytes.Contains(marker.Data, []byte(`"spans"`)) {
		return "", errors.New("jaeger UI/query JSON is intentionally unsupported: Jaeger documents it as an internal, unstable API; export OTLP/JSON instead")
	}
	return "", nil
}

func firstJSONRecord(raw []byte) []byte {
	if index := bytes.IndexByte(raw, '\n'); index >= 0 {
		return bytes.TrimSpace(raw[:index])
	}
	return raw
}

func decodeSingleJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func decodeSingleJSONStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func validateJSONNesting(raw []byte, maximum int) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	depth := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			continue
		}
		switch delimiter {
		case '{', '[':
			depth++
			if depth > maximum {
				return fmt.Errorf("JSON nesting exceeds %d", maximum)
			}
		case '}', ']':
			depth--
		}
	}
	if depth != 0 {
		return errors.New("unbalanced JSON nesting")
	}
	return nil
}

func vendorTraceResult(trajectory Trajectory, privacy PrivacyClass) (TraceImportResult, error) {
	report := MappingReport{SourceRecords: trajectory.Report.SourceRecords, Entries: []MappingEntry{}}
	for index, record := range trajectory.Report.Records {
		base := fmt.Sprintf("/records/%d", index)
		if len(record.Fields) == 0 {
			report.add(base, eventTargets(record.EventIDs), mappingDisposition(record.Disposition), record.Reason)
			continue
		}
		for _, field := range record.Fields {
			report.add(base+field.Path, eventTargets(field.EventIDs), mappingDisposition(field.Disposition), field.Reason)
		}
	}
	result := TraceImportResult{
		Trajectory: trajectory,
		Mapping:    report,
		Envelope: TraceEnvelope{
			Source:          TraceSourceIdentity{Format: trajectory.SourceFormat, SchemaVersion: sourceVersionFor(trajectory.SourceFormat), MediaType: sourceMediaType(trajectory.SourceFormat)},
			CaptureInterval: captureInterval(trajectory.Events), PrivacyClass: privacy,
		},
	}
	return result, finalizeTraceResult(&result)
}

func eventTargets(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return "/events/" + strings.Join(ids, ",")
}

func mappingDisposition(disposition AccountingDisposition) MappingDisposition {
	switch disposition {
	case DispositionRepresented, DispositionMetadataOnly:
		return MappingNormalized
	case DispositionRedacted:
		return MappingRedacted
	case DispositionUnsupported:
		return MappingUnsupported
	case DispositionOmittedSensitive, DispositionTruncated, DispositionRejected:
		return MappingDropped
	default:
		return MappingAmbiguous
	}
}

func captureInterval(events []Event) CaptureInterval {
	var earliest time.Time
	var latest time.Time
	for _, event := range events {
		parsed, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil {
			continue
		}
		if earliest.IsZero() || parsed.Before(earliest) {
			earliest = parsed
		}
		if latest.IsZero() || parsed.After(latest) {
			latest = parsed
		}
	}
	if earliest.IsZero() {
		return CaptureInterval{}
	}
	return CaptureInterval{Start: earliest.UTC().Format(time.RFC3339Nano), End: latest.UTC().Format(time.RFC3339Nano)}
}
