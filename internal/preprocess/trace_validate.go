package preprocess

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

func ValidateTraceImportResult(result TraceImportResult) error {
	if err := result.Trajectory.Validate(); err != nil {
		return fmt.Errorf("validate trace trajectory: %w", err)
	}
	if err := ValidateTraceMappingReport(result.Mapping); err != nil {
		return err
	}
	if err := ValidateTraceEnvelope(result.Envelope); err != nil {
		return err
	}
	if result.Envelope.SourceDigest != result.Trajectory.SourceDigest ||
		result.Envelope.CanonicalTrajectoryDigest != result.Trajectory.Digest ||
		result.Envelope.MappingReportDigest != result.Mapping.Digest ||
		result.Envelope.Source.Format != result.Trajectory.SourceFormat {
		return errors.New("trace import envelope, mapping, and trajectory bindings disagree")
	}
	return nil
}

func ValidateTraceMappingReport(report MappingReport) error {
	if report.SchemaVersion != TraceMappingReportSchema || report.PolicyVersion != TraceMappingPolicyVersion || !validTraceDigest(report.Digest) {
		return errors.New("trace mapping report identity is invalid")
	}
	if err := validateMappingAccounting(report); err != nil {
		return err
	}
	previous := ""
	computed := MappingTotals{}
	for _, entry := range report.Entries {
		key := entry.SourcePath + "\x00" + entry.TargetPath + "\x00" + string(entry.Disposition) + "\x00" + entry.Reason
		if key <= previous || !validMappingDisposition(entry.Disposition) {
			return errors.New("trace mapping entries are invalid, duplicated, or unsorted")
		}
		incrementMappingTotal(&computed, entry.Disposition, entry.Count)
		previous = key
	}
	if computed != report.Totals {
		return errors.New("trace mapping disposition totals differ from its entries")
	}
	expectedLossless := report.Totals.Normalized == 0 && report.Totals.Synthesized == 0 && report.Totals.Redacted == 0 &&
		report.Totals.Unsupported == 0 && report.Totals.Ambiguous == 0 && report.Totals.Dropped == 0
	if report.Lossless != expectedLossless {
		return errors.New("trace mapping lossless state differs from its disposition totals")
	}
	material := report
	material.Digest = ""
	raw, err := json.Marshal(material)
	if err != nil || digestBytes(raw) != report.Digest {
		return errors.New("trace mapping report digest is invalid")
	}
	return nil
}

func ValidateTraceEnvelope(envelope TraceEnvelope) error {
	if envelope.SchemaVersion != TraceEnvelopeSchema || envelope.MappingPolicyVersion != TraceMappingPolicyVersion ||
		!validSourceFormat(envelope.Source.Format) || envelope.Source.SchemaVersion != sourceVersionFor(envelope.Source.Format) ||
		envelope.Source.MediaType != sourceMediaType(envelope.Source.Format) || !validTraceDigest(envelope.SourceDigest) ||
		!validTraceDigest(envelope.CanonicalTrajectoryDigest) || !validTraceDigest(envelope.MappingReportDigest) || !validTraceDigest(envelope.Digest) {
		return errors.New("trace envelope identity or content bindings are invalid")
	}
	if envelope.Source.Format == SourceAgentTrace && envelope.Source.UpstreamCommit != AgentTraceUpstreamCommit ||
		envelope.Source.Format != SourceAgentTrace && envelope.Source.UpstreamCommit != "" {
		return errors.New("trace envelope upstream revision is invalid for its source format")
	}
	if _, err := ParsePrivacyClass(string(envelope.PrivacyClass)); err != nil {
		return err
	}
	if err := validateCaptureInterval(envelope.CaptureInterval); err != nil {
		return err
	}
	material := envelope
	material.Digest = ""
	raw, err := json.Marshal(material)
	if err != nil || digestBytes(raw) != envelope.Digest {
		return errors.New("trace envelope digest is invalid")
	}
	return nil
}

func validSourceFormat(format SourceFormat) bool {
	return slices.Contains([]SourceFormat{
		SourcePlainText, SourceClaudeCode, SourceCodexRollout, SourceOpenCode,
		SourceTerminalBench, SourceSWEbench, SourceOTLPJSON, SourceAgentTrace,
	}, format)
}

func validMappingDisposition(disposition MappingDisposition) bool {
	return slices.Contains([]MappingDisposition{
		MappingExact, MappingNormalized, MappingSynthesized, MappingRedacted,
		MappingUnsupported, MappingAmbiguous, MappingDropped,
	}, disposition)
}

func incrementMappingTotal(total *MappingTotals, disposition MappingDisposition, count int) {
	switch disposition {
	case MappingExact:
		total.Exact += count
	case MappingNormalized:
		total.Normalized += count
	case MappingSynthesized:
		total.Synthesized += count
	case MappingRedacted:
		total.Redacted += count
	case MappingUnsupported:
		total.Unsupported += count
	case MappingAmbiguous:
		total.Ambiguous += count
	case MappingDropped:
		total.Dropped += count
	}
}

func validTraceDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
