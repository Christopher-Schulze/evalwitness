package preprocess

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	TraceEnvelopeSchema       = "evalwitness.trace-envelope.v1"
	TraceMappingReportSchema  = "evalwitness.trace-mapping-report.v1"
	TraceMappingPolicyVersion = "evalwitness.trace-mapping.v1"
	OTLPProtocolVersion       = "1.8.0"
	OTelGenAISemConvVersion   = "1.41.0"
	OTelSchemaURL             = "https://opentelemetry.io/schemas/1.41.0"
	AgentTraceVersion         = "0.1.0"
	AgentTraceUpstreamCommit  = "2754f077f3e50c1fb5088183f5c9362077cc8ca1"
)

type PrivacyClass string

const (
	PrivacyMetadataOnly PrivacyClass = "metadata_only"
	PrivacyContent      PrivacyClass = "content_authorized"
	PrivacyAttribution  PrivacyClass = "attribution_authorized"
	PrivacyFull         PrivacyClass = "content_and_attribution_authorized"
)

func ParsePrivacyClass(value string) (PrivacyClass, error) {
	privacy := PrivacyClass(value)
	switch privacy {
	case PrivacyMetadataOnly, PrivacyContent, PrivacyAttribution, PrivacyFull:
		return privacy, nil
	default:
		return "", fmt.Errorf("unsupported privacy class %q", value)
	}
}

func (p PrivacyClass) IncludesContent() bool {
	return p == PrivacyContent || p == PrivacyFull
}

func (p PrivacyClass) IncludesAttribution() bool {
	return p == PrivacyAttribution || p == PrivacyFull
}

type MappingDisposition string

const (
	MappingExact       MappingDisposition = "exact"
	MappingNormalized  MappingDisposition = "normalized"
	MappingSynthesized MappingDisposition = "synthesized"
	MappingRedacted    MappingDisposition = "redacted"
	MappingUnsupported MappingDisposition = "unsupported"
	MappingAmbiguous   MappingDisposition = "ambiguous"
	MappingDropped     MappingDisposition = "dropped"
)

type TraceSourceIdentity struct {
	Format         SourceFormat `json:"format"`
	SchemaVersion  string       `json:"schema_version"`
	UpstreamCommit string       `json:"upstream_commit,omitempty"`
	MediaType      string       `json:"media_type"`
}

type CaptureInterval struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

type TraceAgentIdentity struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Version  string `json:"version,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type MappingEntry struct {
	SourcePath  string             `json:"source_path"`
	TargetPath  string             `json:"target_path,omitempty"`
	Disposition MappingDisposition `json:"disposition"`
	Count       int                `json:"count"`
	Reason      string             `json:"reason,omitempty"`
}

type MappingTotals struct {
	Exact       int `json:"exact"`
	Normalized  int `json:"normalized"`
	Synthesized int `json:"synthesized"`
	Redacted    int `json:"redacted"`
	Unsupported int `json:"unsupported"`
	Ambiguous   int `json:"ambiguous"`
	Dropped     int `json:"dropped"`
}

type MappingReport struct {
	SchemaVersion string         `json:"schema_version"`
	PolicyVersion string         `json:"policy_version"`
	SourceRecords int            `json:"source_records"`
	SourceFields  int            `json:"source_fields"`
	Lossless      bool           `json:"lossless"`
	Entries       []MappingEntry `json:"entries"`
	Totals        MappingTotals  `json:"totals"`
	Digest        string         `json:"digest"`
}

type TraceEnvelope struct {
	SchemaVersion             string              `json:"schema_version"`
	MappingPolicyVersion      string              `json:"mapping_policy_version"`
	Source                    TraceSourceIdentity `json:"source"`
	SourceDigest              string              `json:"source_digest"`
	CaptureInterval           CaptureInterval     `json:"capture_interval"`
	Agent                     TraceAgentIdentity  `json:"agent"`
	PrivacyClass              PrivacyClass        `json:"privacy_class"`
	CanonicalTrajectoryDigest string              `json:"canonical_trajectory_digest"`
	MappingReportDigest       string              `json:"mapping_report_digest"`
	Digest                    string              `json:"digest"`
}

type TraceImportResult struct {
	Envelope   TraceEnvelope `json:"envelope"`
	Mapping    MappingReport `json:"mapping"`
	Trajectory Trajectory    `json:"trajectory"`
}

type TraceExportResult struct {
	Format         SourceFormat  `json:"format"`
	SchemaVersion  string        `json:"schema_version"`
	ArtifactDigest string        `json:"artifact_digest"`
	Mapping        MappingReport `json:"mapping"`
	Bytes          []byte        `json:"-"`
}

type VerifierEvidenceReference struct {
	ProtocolRunDigest string `json:"protocol_run_digest"`
	AuditCaseID       string `json:"audit_case_id,omitempty"`
	DecisionDigest    string `json:"decision_digest,omitempty"`
}

type TraceInspectReport struct {
	SchemaVersion   string               `json:"schema_version"`
	Envelope        TraceEnvelope        `json:"envelope"`
	SourceRecords   int                  `json:"source_records"`
	CanonicalEvents int                  `json:"canonical_events"`
	CausalLinks     int                  `json:"causal_links"`
	RootEvents      int                  `json:"root_events"`
	MaximumDepth    int                  `json:"maximum_depth"`
	OriginalBytes   int                  `json:"original_bytes"`
	RetainedBytes   int                  `json:"retained_bytes"`
	Mapping         MappingReport        `json:"mapping"`
	EventKinds      []CategoryRetention  `json:"event_kinds"`
	ProviderUsage   []ProviderTokenUsage `json:"provider_usage,omitempty"`
}

type TraceImportOptions struct {
	Ingest  IngestOptions
	Privacy PrivacyClass
}

func DefaultTraceImportOptions() TraceImportOptions {
	return TraceImportOptions{Ingest: DefaultIngestOptions(), Privacy: PrivacyMetadataOnly}
}

func (r *MappingReport) add(sourcePath, targetPath string, disposition MappingDisposition, reason string) {
	for index := range r.Entries {
		entry := &r.Entries[index]
		if entry.SourcePath == sourcePath && entry.TargetPath == targetPath && entry.Disposition == disposition && entry.Reason == reason {
			entry.Count++
			r.SourceFields++
			r.increment(disposition)
			return
		}
	}
	r.Entries = append(r.Entries, MappingEntry{SourcePath: sourcePath, TargetPath: targetPath, Disposition: disposition, Count: 1, Reason: reason})
	r.SourceFields++
	r.increment(disposition)
}

func (r *MappingReport) increment(disposition MappingDisposition) {
	switch disposition {
	case MappingExact:
		r.Totals.Exact++
	case MappingNormalized:
		r.Totals.Normalized++
	case MappingSynthesized:
		r.Totals.Synthesized++
	case MappingRedacted:
		r.Totals.Redacted++
	case MappingUnsupported:
		r.Totals.Unsupported++
	case MappingAmbiguous:
		r.Totals.Ambiguous++
	case MappingDropped:
		r.Totals.Dropped++
	}
}

func finalizeTraceResult(result *TraceImportResult) error {
	if err := result.Trajectory.Validate(); err != nil {
		return fmt.Errorf("validate imported trajectory: %w", err)
	}
	if err := validateMappingAccounting(result.Mapping); err != nil {
		return err
	}
	result.Mapping.SchemaVersion = TraceMappingReportSchema
	result.Mapping.PolicyVersion = TraceMappingPolicyVersion
	sort.Slice(result.Mapping.Entries, func(i, j int) bool {
		left := result.Mapping.Entries[i]
		right := result.Mapping.Entries[j]
		return left.SourcePath+"\x00"+left.TargetPath+"\x00"+string(left.Disposition)+"\x00"+left.Reason <
			right.SourcePath+"\x00"+right.TargetPath+"\x00"+string(right.Disposition)+"\x00"+right.Reason
	})
	result.Mapping.Lossless = result.Mapping.Totals.Normalized == 0 && result.Mapping.Totals.Synthesized == 0 &&
		result.Mapping.Totals.Redacted == 0 && result.Mapping.Totals.Unsupported == 0 &&
		result.Mapping.Totals.Ambiguous == 0 && result.Mapping.Totals.Dropped == 0
	mappingMaterial := result.Mapping
	mappingMaterial.Digest = ""
	mappingRaw, err := json.Marshal(mappingMaterial)
	if err != nil {
		return fmt.Errorf("encode mapping report: %w", err)
	}
	result.Mapping.Digest = digestBytes(mappingRaw)
	result.Envelope.SchemaVersion = TraceEnvelopeSchema
	result.Envelope.MappingPolicyVersion = TraceMappingPolicyVersion
	result.Envelope.SourceDigest = result.Trajectory.SourceDigest
	result.Envelope.CanonicalTrajectoryDigest = result.Trajectory.Digest
	result.Envelope.MappingReportDigest = result.Mapping.Digest
	envelopeMaterial := result.Envelope
	envelopeMaterial.Digest = ""
	envelopeRaw, err := json.Marshal(envelopeMaterial)
	if err != nil {
		return fmt.Errorf("encode trace envelope: %w", err)
	}
	result.Envelope.Digest = digestBytes(envelopeRaw)
	return ValidateTraceImportResult(*result)
}

func validateMappingAccounting(report MappingReport) error {
	entryTotal := 0
	for _, entry := range report.Entries {
		if entry.SourcePath == "" || entry.Count <= 0 {
			return errors.New("mapping entry source path and count are required")
		}
		entryTotal += entry.Count
	}
	total := report.Totals.Exact + report.Totals.Normalized + report.Totals.Synthesized + report.Totals.Redacted +
		report.Totals.Unsupported + report.Totals.Ambiguous + report.Totals.Dropped
	if report.SourceRecords < 0 || report.SourceFields != entryTotal || report.SourceFields != total {
		return fmt.Errorf("mapping accounting is inconsistent: records=%d fields=%d entries=%d totals=%d", report.SourceRecords, report.SourceFields, entryTotal, total)
	}
	return nil
}

func InspectTrace(result TraceImportResult) (TraceInspectReport, error) {
	if err := result.Trajectory.Validate(); err != nil {
		return TraceInspectReport{}, err
	}
	depth, roots, err := causalDepth(result.Trajectory)
	if err != nil {
		return TraceInspectReport{}, err
	}
	return TraceInspectReport{
		SchemaVersion: "evalwitness.trace-inspection.v1", Envelope: result.Envelope,
		SourceRecords: result.Trajectory.Report.SourceRecords, CanonicalEvents: len(result.Trajectory.Events),
		CausalLinks: len(result.Trajectory.Links), RootEvents: roots, MaximumDepth: depth,
		OriginalBytes: result.Trajectory.Report.OriginalBytes, RetainedBytes: result.Trajectory.Report.RetainedBytes,
		Mapping: result.Mapping, EventKinds: append([]CategoryRetention(nil), result.Trajectory.Report.Categories...),
		ProviderUsage: append([]ProviderTokenUsage(nil), result.Trajectory.Report.ProviderUsage...),
	}, nil
}

func causalDepth(trajectory Trajectory) (int, int, error) {
	parents := make(map[string][]string)
	children := make(map[string][]string)
	for _, link := range trajectory.Links {
		if link.Kind == LinkReference {
			continue
		}
		parents[link.ToID] = append(parents[link.ToID], link.FromID)
		children[link.FromID] = append(children[link.FromID], link.ToID)
	}
	roots := make([]string, 0)
	for _, event := range trajectory.Events {
		if len(parents[event.ID]) == 0 {
			roots = append(roots, event.ID)
		}
	}
	maximum := 0
	var visit func(string, int) error
	visiting := make(map[string]bool)
	visit = func(id string, depth int) error {
		if visiting[id] {
			return fmt.Errorf("causal hierarchy cycle reaches %q", id)
		}
		visiting[id] = true
		if depth > maximum {
			maximum = depth
		}
		for _, child := range children[id] {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		delete(visiting, id)
		return nil
	}
	for _, root := range roots {
		if err := visit(root, 1); err != nil {
			return 0, 0, err
		}
	}
	return maximum, len(roots), nil
}

func validateCaptureInterval(interval CaptureInterval) error {
	if interval.Start == "" && interval.End == "" {
		return nil
	}
	start, err := time.Parse(time.RFC3339Nano, interval.Start)
	if err != nil {
		return fmt.Errorf("capture start: %w", err)
	}
	end, err := time.Parse(time.RFC3339Nano, interval.End)
	if err != nil {
		return fmt.Errorf("capture end: %w", err)
	}
	if end.Before(start) {
		return errors.New("capture end precedes start")
	}
	return nil
}

func sourceVersionFor(format SourceFormat) string {
	switch format {
	case SourceOTLPJSON:
		return "otlp-json-" + OTLPProtocolVersion + "+gen-ai-" + OTelGenAISemConvVersion
	case SourceAgentTrace:
		return AgentTraceVersion
	default:
		return "unversioned-export+" + TraceMappingPolicyVersion
	}
}

func sourceMediaType(format SourceFormat) string {
	switch format {
	case SourceClaudeCode, SourceCodexRollout:
		return "application/x-ndjson"
	case SourceAgentTrace:
		return "application/vnd.agent-trace.record+json"
	case SourceOTLPJSON:
		return "application/json"
	default:
		return "application/json"
	}
}
