package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func runTrace(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "trace: expected inspect, export, or lineage")
		return 2
	}
	switch args[0] {
	case "inspect":
		return runTraceInspect(args[1:])
	case "export":
		return runTraceExport(args[1:])
	case "lineage":
		return runTraceLineage(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "trace: unknown operation %q\n", args[0])
		return 2
	}
}

func runTraceInspect(args []string) int {
	flags := flag.NewFlagSet("trace inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", "", "trace source path or - for stdin")
	privacyName := flags.String("privacy-class", string(preprocess.PrivacyMetadataOnly), "metadata_only/content_authorized/attribution_authorized/content_and_attribution_authorized")
	output := flags.String("output", "json", "json/text")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *source == "" {
		fmt.Fprintln(os.Stderr, "trace inspect: --source is required and positional arguments are forbidden")
		return 2
	}
	result, err := loadTraceImport(*source, *privacyName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace inspect:", err)
		return 1
	}
	report, err := preprocess.InspectTrace(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace inspect:", err)
		return 1
	}
	switch *output {
	case "json":
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "trace inspect:", err)
			return 1
		}
		fmt.Println(string(encoded))
	case "text":
		fmt.Printf("format=%s version=%s privacy=%s source=%s canonical=%s mapping=%s records=%d events=%d links=%d roots=%d depth=%d usage=%d bytes=%d/%d lossless=%t\n",
			report.Envelope.Source.Format, report.Envelope.Source.SchemaVersion, report.Envelope.PrivacyClass,
			report.Envelope.SourceDigest, report.Envelope.CanonicalTrajectoryDigest, report.Envelope.MappingReportDigest,
			report.SourceRecords, report.CanonicalEvents, report.CausalLinks, report.RootEvents, report.MaximumDepth, len(report.ProviderUsage),
			report.RetainedBytes, report.OriginalBytes, report.Mapping.Lossless)
		fmt.Printf("mapping exact=%d normalized=%d synthesized=%d redacted=%d unsupported=%d ambiguous=%d dropped=%d\n",
			report.Mapping.Totals.Exact, report.Mapping.Totals.Normalized, report.Mapping.Totals.Synthesized,
			report.Mapping.Totals.Redacted, report.Mapping.Totals.Unsupported, report.Mapping.Totals.Ambiguous, report.Mapping.Totals.Dropped)
	default:
		fmt.Fprintln(os.Stderr, "trace inspect: --output must be json or text")
		return 2
	}
	return 0
}

func runTraceExport(args []string) int {
	flags := flag.NewFlagSet("trace export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", "", "trace source path or - for stdin")
	target := flags.String("target", "canonical", "canonical/otlp/agent-trace")
	privacyName := flags.String("privacy-class", string(preprocess.PrivacyMetadataOnly), "explicit import/export privacy class")
	output := flags.String("output", "json", "json/artifact")
	protocolRunDigest := flags.String("protocol-run-digest", "", "optional namespaced verifier-evidence reference for Agent Trace")
	auditCaseID := flags.String("audit-case-id", "", "optional verifier audit case identity")
	decisionDigest := flags.String("decision-digest", "", "optional verifier decision digest")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *source == "" {
		fmt.Fprintln(os.Stderr, "trace export: --source is required and positional arguments are forbidden")
		return 2
	}
	privacy, err := preprocess.ParsePrivacyClass(*privacyName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace export:", err)
		return 2
	}
	result, err := loadTraceImport(*source, *privacyName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace export:", err)
		return 1
	}
	var exported preprocess.TraceExportResult
	switch *target {
	case "canonical":
		artifact, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			err = marshalErr
			break
		}
		exported = preprocess.TraceExportResult{Format: result.Trajectory.SourceFormat, SchemaVersion: preprocess.TraceEnvelopeSchema, ArtifactDigest: fmt.Sprintf("%x", sha256.Sum256(artifact)), Mapping: result.Mapping, Bytes: artifact}
	case "otlp":
		exported, err = preprocess.ExportOTLPJSON(result, privacy)
	case "agent-trace":
		var evidence *preprocess.VerifierEvidenceReference
		if *protocolRunDigest != "" || *auditCaseID != "" || *decisionDigest != "" {
			evidence = &preprocess.VerifierEvidenceReference{ProtocolRunDigest: *protocolRunDigest, AuditCaseID: *auditCaseID, DecisionDigest: *decisionDigest}
		}
		exported, err = preprocess.ExportAgentTrace(result, evidence)
	default:
		fmt.Fprintln(os.Stderr, "trace export: --target must be canonical, otlp, or agent-trace")
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace export:", err)
		return 1
	}
	switch *output {
	case "artifact":
		if _, err := os.Stdout.Write(append(exported.Bytes, '\n')); err != nil {
			fmt.Fprintln(os.Stderr, "trace export:", err)
			return 1
		}
	case "json":
		wrapper := struct {
			Format         preprocess.SourceFormat  `json:"format"`
			SchemaVersion  string                   `json:"schema_version"`
			ArtifactDigest string                   `json:"artifact_digest"`
			Mapping        preprocess.MappingReport `json:"mapping"`
			Artifact       json.RawMessage          `json:"artifact"`
		}{exported.Format, exported.SchemaVersion, exported.ArtifactDigest, exported.Mapping, exported.Bytes}
		encoded, marshalErr := json.MarshalIndent(wrapper, "", "  ")
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "trace export:", marshalErr)
			return 1
		}
		fmt.Println(string(encoded))
	default:
		fmt.Fprintln(os.Stderr, "trace export: --output must be json or artifact")
		return 2
	}
	return 0
}

func loadTraceImport(source, privacyName string) (result preprocess.TraceImportResult, returnErr error) {
	privacy, err := preprocess.ParsePrivacyClass(privacyName)
	if err != nil {
		return preprocess.TraceImportResult{}, err
	}
	options := preprocess.DefaultTraceImportOptions()
	options.Privacy = privacy
	if source == "-" {
		return preprocess.ImportTraceReader(os.Stdin, options)
	}
	path, err := expandPath(source)
	if err != nil {
		return preprocess.TraceImportResult{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return preprocess.TraceImportResult{}, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close trace import: %w", closeErr)
		}
	}()
	return preprocess.ImportTraceReader(file, options)
}
