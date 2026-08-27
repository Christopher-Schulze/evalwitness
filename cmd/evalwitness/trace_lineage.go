package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
)

func runTraceLineage(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "trace lineage: expected adapter-conformance, capability-matrix, capability-matrix-verify, corpus-feasibility, corpus-feasibility-verify, development-dataset-card, development-dataset-card-verify, development-release, development-release-verify, fixture-witnesses, golden-vectors, holdout-readiness, holdout-readiness-verify, intake, lineage-graph, lineage-graph-verify, limitations, limitations-verify, loss-certificate, loss-certificate-verify, offline-audit, offline-audit-verify, offline-bom, offline-bom-verify, offline-proof, offline-proof-verify, parser-lock, parser-lock-verify, plan, schema-inventory, source-inventory, source-readiness, source-readiness-verify, source-specifications, schema, or validate")
		return 2
	}
	switch args[0] {
	case "plan":
		return runTraceLineagePlan(args[1:])
	case "schema-inventory":
		return runTraceLineageSchemaInventory(args[1:])
	case "source-specifications":
		return runTraceLineageSourceSpecifications(args[1:])
	case "source-inventory":
		return runTraceLineageSourceInventory(args[1:])
	case "fixture-witnesses":
		return runTraceLineageFixtureWitnesses(args[1:])
	case "golden-vectors":
		return runTraceLineageGoldenVectors(args[1:])
	case "adapter-conformance":
		return runTraceLineageAdapterConformance(args[1:])
	case "parser-lock":
		return runTraceLineageParserLock(args[1:])
	case "parser-lock-verify":
		return runTraceLineageParserLockVerify(args[1:])
	case "source-readiness":
		return runTraceLineageSourceReadiness(args[1:])
	case "source-readiness-verify":
		return runTraceLineageSourceReadinessVerify(args[1:])
	case "holdout-readiness":
		return runTraceLineageHoldoutReadiness(args[1:])
	case "holdout-readiness-verify":
		return runTraceLineageHoldoutReadinessVerify(args[1:])
	case "corpus-feasibility":
		return runTraceLineageCorpusFeasibility(args[1:])
	case "corpus-feasibility-verify":
		return runTraceLineageCorpusFeasibilityVerify(args[1:])
	case "capability-matrix":
		return runTraceLineageCapabilityMatrix(args[1:])
	case "capability-matrix-verify":
		return runTraceLineageCapabilityMatrixVerify(args[1:])
	case "offline-proof":
		return runTraceLineageOfflineProof(args[1:])
	case "offline-proof-verify":
		return runTraceLineageOfflineProofVerify(args[1:])
	case "loss-certificate":
		return runTraceLineageLossCertificate(args[1:])
	case "loss-certificate-verify":
		return runTraceLineageLossCertificateVerify(args[1:])
	case "lineage-graph":
		return runTraceLineageGraph(args[1:])
	case "lineage-graph-verify":
		return runTraceLineageGraphVerify(args[1:])
	case "offline-bom":
		return runTraceLineageOfflineBOM(args[1:])
	case "offline-bom-verify":
		return runTraceLineageOfflineBOMVerify(args[1:])
	case "offline-audit":
		return runTraceLineageOfflineAudit(args[1:])
	case "offline-audit-verify":
		return runTraceLineageOfflineAuditVerify(args[1:])
	case "development-dataset-card":
		return runTraceLineageDevelopmentDatasetCard(args[1:])
	case "development-dataset-card-verify":
		return runTraceLineageDevelopmentDatasetCardVerify(args[1:])
	case "limitations":
		return runTraceLineageLimitations(args[1:])
	case "limitations-verify":
		return runTraceLineageLimitationsVerify(args[1:])
	case "development-release":
		return runTraceLineageDevelopmentRelease(args[1:])
	case "development-release-verify":
		return runTraceLineageDevelopmentReleaseVerify(args[1:])
	case "intake":
		return runTraceLineageIntake(args[1:])
	case "fixture-child":
		return runTraceLineageFixtureChild(args[1:])
	case "schema":
		return runTraceLineageSchema(args[1:])
	case "validate":
		return runTraceLineageValidate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "trace lineage: unknown operation %q\n", args[0])
		return 2
	}
}

func runTraceLineageIntake(args []string) int {
	flags := flag.NewFlagSet("trace lineage intake", flag.ContinueOnError)
	source := flags.String("source", "", "trace source path or - for stdin")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *source == "" {
		fmt.Fprintln(os.Stderr, "trace lineage intake: --source is required and positional arguments are forbidden")
		return 2
	}
	result, err := loadTraceImport(*source, "metadata_only")
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage intake:", err)
		return 1
	}
	report, err := lineage.BuildVerificationLineageTraceIntakeReport(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage intake:", err)
		return 1
	}
	return printLineageJSON("trace lineage intake", report)
}

func runTraceLineageOfflineProof(args []string) int {
	flags := flag.NewFlagSet("trace lineage offline-proof", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage offline-proof: positional arguments are forbidden")
		return 2
	}
	proof, err := lineage.BuildVerificationLineageOfflineProof(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-proof:", err)
		return 1
	}
	return printLineageJSON("trace lineage offline-proof", proof)
}

func runTraceLineageOfflineProofVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage offline-proof-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	documentPath := flags.String("document", "", "offline-proof document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage offline-proof-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-proof-verify:", err)
		return 2
	}
	defer closeDocument()
	proof, err := lineage.DecodeOfflineProof(*repositoryRoot, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-proof-verify:", err)
		return 1
	}
	actual, err := lineage.BuildVerificationLineageOfflineProof(*repositoryRoot)
	if err != nil || actual.Digest != proof.Digest {
		if err == nil {
			err = errors.New("offline proof differs from sealed development evidence")
		}
		fmt.Fprintln(os.Stderr, "trace lineage offline-proof-verify:", err)
		return 1
	}
	output := struct {
		Digest  string `json:"digest"`
		ProofID string `json:"proof_id"`
		Valid   bool   `json:"valid"`
		Version string `json:"version"`
	}{Digest: proof.Digest, ProofID: proof.ProofID, Valid: true, Version: proof.Version}
	return printLineageJSON("trace lineage offline-proof-verify", output)
}

func runTraceLineageLossCertificate(args []string) int {
	flags := flag.NewFlagSet("trace lineage loss-certificate", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage loss-certificate: positional arguments are forbidden")
		return 2
	}
	certificate, err := lineage.BuildVerificationLineageLossCertificate(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage loss-certificate:", err)
		return 1
	}
	return printLineageJSON("trace lineage loss-certificate", certificate)
}

func runTraceLineageLossCertificateVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage loss-certificate-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	documentPath := flags.String("document", "", "loss-certificate document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage loss-certificate-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage loss-certificate-verify:", err)
		return 2
	}
	defer closeDocument()
	certificate, err := lineage.DecodeLossCertificate(*repositoryRoot, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage loss-certificate-verify:", err)
		return 1
	}
	output := struct {
		CertificateID string `json:"certificate_id"`
		Digest        string `json:"digest"`
		Valid         bool   `json:"valid"`
		Version       string `json:"version"`
	}{CertificateID: certificate.CertificateID, Digest: certificate.Digest, Valid: true, Version: certificate.Version}
	return printLineageJSON("trace lineage loss-certificate-verify", output)
}

func runTraceLineageGraph(args []string) int {
	flags := flag.NewFlagSet("trace lineage lineage-graph", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	outputFormat := flags.String("format", "json", "output format: json or svg")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || (*outputFormat != "json" && *outputFormat != "svg") {
		fmt.Fprintln(os.Stderr, "trace lineage lineage-graph: --format must be json or svg and positional arguments are forbidden")
		return 2
	}
	graph, err := lineage.BuildVerificationLineageGraph(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage lineage-graph:", err)
		return 1
	}
	if *outputFormat == "json" {
		return printLineageJSON("trace lineage lineage-graph", graph)
	}
	encoded, err := lineage.RenderVerificationLineageGraphSVG(graph)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage lineage-graph:", err)
		return 1
	}
	return writeCommandOutput("trace lineage lineage-graph", encoded)
}

func runTraceLineageGraphVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage lineage-graph-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	documentPath := flags.String("document", "", "lineage-graph JSON document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage lineage-graph-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage lineage-graph-verify:", err)
		return 2
	}
	defer closeDocument()
	graph, err := lineage.DecodeLineageGraph(*repositoryRoot, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage lineage-graph-verify:", err)
		return 1
	}
	output := struct {
		Digest  string `json:"digest"`
		GraphID string `json:"graph_id"`
		Valid   bool   `json:"valid"`
		Version string `json:"version"`
	}{Digest: graph.Digest, GraphID: graph.GraphID, Valid: true, Version: graph.Version}
	return printLineageJSON("trace lineage lineage-graph-verify", output)
}

func runTraceLineageOfflineBOM(args []string) int {
	flags := flag.NewFlagSet("trace lineage offline-bom", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage offline-bom: positional arguments are forbidden")
		return 2
	}
	bom, err := lineage.BuildVerificationLineageOfflineBOM(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-bom:", err)
		return 1
	}
	return printLineageJSON("trace lineage offline-bom", bom)
}

func runTraceLineageOfflineAudit(args []string) int {
	flags := flag.NewFlagSet("trace lineage offline-audit", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage offline-audit: positional arguments are forbidden")
		return 2
	}
	audit, err := lineage.BuildVerificationLineageOfflineAudit(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-audit:", err)
		return 1
	}
	return printLineageJSON("trace lineage offline-audit", audit)
}

func runTraceLineageOfflineAuditVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage offline-audit-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	documentPath := flags.String("document", "", "offline audit document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage offline-audit-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-audit-verify:", err)
		return 2
	}
	defer closeDocument()
	audit, err := lineage.DecodeOfflineAudit(reader)
	if err == nil {
		err = lineage.VerifyVerificationLineageOfflineAudit(*repositoryRoot, audit)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-audit-verify:", err)
		return 1
	}
	return printLineageJSON("trace lineage offline-audit-verify", lineageValidationOutput{Digest: audit.Header.Digest, ObjectID: audit.Header.ObjectID, Valid: true, Version: audit.Header.SchemaVersion})
}

func runTraceLineageOfflineBOMVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage offline-bom-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	documentPath := flags.String("document", "", "offline BOM document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage offline-bom-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-bom-verify:", err)
		return 2
	}
	defer closeDocument()
	bom, err := lineage.DecodeOfflineBOM(reader)
	if err == nil {
		err = lineage.VerifyVerificationLineageOfflineBOM(*repositoryRoot, bom)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage offline-bom-verify:", err)
		return 1
	}
	return printLineageJSON("trace lineage offline-bom-verify", lineageValidationOutput{Digest: bom.Header.Digest, ObjectID: bom.Header.ObjectID, Valid: true, Version: bom.Header.SchemaVersion})
}

func runTraceLineageDevelopmentDatasetCard(args []string) int {
	flags := flag.NewFlagSet("trace lineage development-dataset-card", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage development-dataset-card: positional arguments are forbidden")
		return 2
	}
	card, err := lineage.BuildVerificationLineageDevelopmentDatasetCard(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage development-dataset-card:", err)
		return 1
	}
	return printLineageJSON("trace lineage development-dataset-card", card)
}

func runTraceLineageDevelopmentDatasetCardVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage development-dataset-card-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	documentPath := flags.String("document", "", "development dataset-card document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage development-dataset-card-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage development-dataset-card-verify:", err)
		return 2
	}
	defer closeDocument()
	card, err := lineage.DecodeDevelopmentDatasetCard(reader)
	if err == nil {
		err = lineage.VerifyVerificationLineageDevelopmentDatasetCard(*repositoryRoot, card)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage development-dataset-card-verify:", err)
		return 1
	}
	return printLineageJSON("trace lineage development-dataset-card-verify", lineageValidationOutput{Digest: card.Header.Digest, ObjectID: card.Header.ObjectID, Valid: true, Version: card.Header.SchemaVersion})
}

func runTraceLineageLimitations(args []string) int {
	flags := flag.NewFlagSet("trace lineage limitations", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage limitations: positional arguments are forbidden")
		return 2
	}
	ledger, err := lineage.BuildVerificationLineageLimitationsLedger(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage limitations:", err)
		return 1
	}
	return printLineageJSON("trace lineage limitations", ledger)
}

func runTraceLineageLimitationsVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage limitations-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed development evidence")
	documentPath := flags.String("document", "", "limitations document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage limitations-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage limitations-verify:", err)
		return 2
	}
	defer closeDocument()
	ledger, err := lineage.DecodeLimitationsLedger(reader)
	if err == nil {
		err = lineage.VerifyVerificationLineageLimitationsLedger(*repositoryRoot, ledger)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage limitations-verify:", err)
		return 1
	}
	return printLineageJSON("trace lineage limitations-verify", lineageValidationOutput{Digest: ledger.Digest, ObjectID: ledger.LedgerID, Valid: true, Version: ledger.Version})
}

func runTraceLineageDevelopmentRelease(args []string) int {
	flags := flag.NewFlagSet("trace lineage development-release", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the complete development package")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage development-release: positional arguments are forbidden")
		return 2
	}
	release, err := lineage.BuildVerificationLineageDevelopmentRelease(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage development-release:", err)
		return 1
	}
	return printLineageJSON("trace lineage development-release", release)
}

func runTraceLineageDevelopmentReleaseVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage development-release-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the complete development package")
	documentPath := flags.String("document", "", "development release document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage development-release-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage development-release-verify:", err)
		return 2
	}
	defer closeDocument()
	release, err := lineage.DecodeDevelopmentRelease(reader)
	if err == nil {
		err = lineage.VerifyVerificationLineageDevelopmentRelease(*repositoryRoot, release)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage development-release-verify:", err)
		return 1
	}
	return printLineageJSON("trace lineage development-release-verify", lineageValidationOutput{Digest: release.Header.Digest, ObjectID: release.Header.ObjectID, Valid: true, Version: release.Header.SchemaVersion})
}

type lineageValidationOutput struct {
	Digest   string `json:"digest"`
	ObjectID string `json:"object_id"`
	Valid    bool   `json:"valid"`
	Version  string `json:"version"`
}

func runTraceLineageCapabilityMatrix(args []string) int {
	flags := flag.NewFlagSet("trace lineage capability-matrix", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage capability-matrix: positional arguments are forbidden")
		return 2
	}
	matrix, err := lineage.BuildVerificationLineageCapabilityMatrix()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage capability-matrix:", err)
		return 1
	}
	return printLineageJSON("trace lineage capability-matrix", matrix)
}

func runTraceLineageCapabilityMatrixVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage capability-matrix-verify", flag.ContinueOnError)
	documentPath := flags.String("document", "", "capability-matrix document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage capability-matrix-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage capability-matrix-verify:", err)
		return 2
	}
	defer closeDocument()
	matrix, err := lineage.DecodeCapabilityMatrix(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage capability-matrix-verify:", err)
		return 1
	}
	actual, err := lineage.BuildVerificationLineageCapabilityMatrix()
	if err != nil || actual.Digest != matrix.Digest {
		if err == nil {
			err = errors.New("capability matrix differs from sealed specifications and vectors")
		}
		fmt.Fprintln(os.Stderr, "trace lineage capability-matrix-verify:", err)
		return 1
	}
	output := struct {
		Digest   string `json:"digest"`
		MatrixID string `json:"matrix_id"`
		Valid    bool   `json:"valid"`
		Version  string `json:"version"`
	}{Digest: matrix.Digest, MatrixID: matrix.MatrixID, Valid: true, Version: matrix.Version}
	return printLineageJSON("trace lineage capability-matrix-verify", output)
}

func runTraceLineageCorpusFeasibility(args []string) int {
	flags := flag.NewFlagSet("trace lineage corpus-feasibility", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed readiness evidence")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage corpus-feasibility: positional arguments are forbidden")
		return 2
	}
	decision, err := lineage.BuildVerificationLineageCorpusFeasibilityDecision(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage corpus-feasibility:", err)
		return 1
	}
	return printLineageJSON("trace lineage corpus-feasibility", decision)
}

func runTraceLineageCorpusFeasibilityVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage corpus-feasibility-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed readiness evidence")
	documentPath := flags.String("document", "", "corpus-feasibility decision document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage corpus-feasibility-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage corpus-feasibility-verify:", err)
		return 2
	}
	defer closeDocument()
	decision, err := lineage.DecodeCorpusFeasibilityDecision(*repositoryRoot, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage corpus-feasibility-verify:", err)
		return 1
	}
	if err := lineage.VerifyVerificationLineageCorpusFeasibilityDecision(*repositoryRoot, decision); err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage corpus-feasibility-verify:", err)
		return 1
	}
	output := struct {
		DecisionID string `json:"decision_id"`
		Digest     string `json:"digest"`
		Valid      bool   `json:"valid"`
		Version    string `json:"version"`
	}{DecisionID: decision.DecisionID, Digest: decision.Digest, Valid: true, Version: decision.Version}
	return printLineageJSON("trace lineage corpus-feasibility-verify", output)
}

func runTraceLineageHoldoutReadiness(args []string) int {
	flags := flag.NewFlagSet("trace lineage holdout-readiness", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed plan, parser lock, and golden vectors")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage holdout-readiness: positional arguments are forbidden")
		return 2
	}
	audit, err := lineage.BuildVerificationLineageHoldoutReadinessAudit(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage holdout-readiness:", err)
		return 1
	}
	return printLineageJSON("trace lineage holdout-readiness", audit)
}

func runTraceLineageHoldoutReadinessVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage holdout-readiness-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed plan, parser lock, and golden vectors")
	documentPath := flags.String("document", "", "holdout-readiness audit document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage holdout-readiness-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage holdout-readiness-verify:", err)
		return 2
	}
	defer closeDocument()
	audit, err := lineage.DecodeHoldoutReadinessAudit(*repositoryRoot, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage holdout-readiness-verify:", err)
		return 1
	}
	if err := lineage.VerifyVerificationLineageHoldoutReadinessAudit(*repositoryRoot, audit); err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage holdout-readiness-verify:", err)
		return 1
	}
	output := struct {
		AuditID string `json:"audit_id"`
		Digest  string `json:"digest"`
		Valid   bool   `json:"valid"`
		Version string `json:"version"`
	}{AuditID: audit.AuditID, Digest: audit.Digest, Valid: true, Version: audit.Version}
	return printLineageJSON("trace lineage holdout-readiness-verify", output)
}

func runTraceLineageSourceReadiness(args []string) int {
	flags := flag.NewFlagSet("trace lineage source-readiness", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed inventory and parser lock inputs")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage source-readiness: positional arguments are forbidden")
		return 2
	}
	audit, err := lineage.BuildVerificationLineageSourceReadinessAudit(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage source-readiness:", err)
		return 1
	}
	return printLineageJSON("trace lineage source-readiness", audit)
}

func runTraceLineageSourceReadinessVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage source-readiness-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the sealed inventory and parser lock inputs")
	documentPath := flags.String("document", "", "source-readiness audit document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage source-readiness-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage source-readiness-verify:", err)
		return 2
	}
	defer closeDocument()
	audit, err := lineage.DecodeSourceReadinessAudit(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage source-readiness-verify:", err)
		return 1
	}
	if err := lineage.VerifyVerificationLineageSourceReadinessAudit(*repositoryRoot, audit); err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage source-readiness-verify:", err)
		return 1
	}
	output := struct {
		AuditID string `json:"audit_id"`
		Digest  string `json:"digest"`
		Valid   bool   `json:"valid"`
		Version string `json:"version"`
	}{AuditID: audit.AuditID, Digest: audit.Digest, Valid: true, Version: audit.Version}
	return printLineageJSON("trace lineage source-readiness-verify", output)
}

func runTraceLineageParserLock(args []string) int {
	flags := flag.NewFlagSet("trace lineage parser-lock", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the bound source and governance artifacts")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage parser-lock: positional arguments are forbidden")
		return 2
	}
	lock, err := lineage.BuildVerificationLineageParserLock(*repositoryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage parser-lock:", err)
		return 1
	}
	return printLineageJSON("trace lineage parser-lock", lock)
}

func runTraceLineageParserLockVerify(args []string) int {
	flags := flag.NewFlagSet("trace lineage parser-lock-verify", flag.ContinueOnError)
	repositoryRoot := flags.String("repository-root", ".", "repository root containing the bound source and governance artifacts")
	documentPath := flags.String("document", "", "parser lock document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage parser-lock-verify: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage parser-lock-verify:", err)
		return 2
	}
	defer closeDocument()
	lock, err := lineage.DecodeParserLock(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage parser-lock-verify:", err)
		return 1
	}
	if err := lineage.VerifyVerificationLineageParserLock(*repositoryRoot, lock); err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage parser-lock-verify:", err)
		return 1
	}
	output := struct {
		Digest  string `json:"digest"`
		LockID  string `json:"lock_id"`
		Valid   bool   `json:"valid"`
		Version string `json:"version"`
	}{Digest: lock.Digest, LockID: lock.LockID, Valid: true, Version: lock.Version}
	return printLineageJSON("trace lineage parser-lock-verify", output)
}

func runTraceLineageAdapterConformance(args []string) int {
	flags := flag.NewFlagSet("trace lineage adapter-conformance", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage adapter-conformance: positional arguments are forbidden")
		return 2
	}
	report, err := lineage.BuildAdapterConformanceReport()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage adapter-conformance:", err)
		return 1
	}
	return printLineageJSON("trace lineage adapter-conformance", report)
}

func runTraceLineageGoldenVectors(args []string) int {
	flags := flag.NewFlagSet("trace lineage golden-vectors", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage golden-vectors: positional arguments are forbidden")
		return 2
	}
	fixtures, err := lineage.BuildGoldenVectorFixtureSet()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage golden-vectors:", err)
		return 1
	}
	return printLineageJSON("trace lineage golden-vectors", fixtures)
}

func runTraceLineageFixtureWitnesses(args []string) int {
	flags := flag.NewFlagSet("trace lineage fixture-witnesses", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage fixture-witnesses: positional arguments are forbidden")
		return 2
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage fixture-witnesses: resolve executable:", err)
		return 1
	}
	observations := make([]lineage.SyntheticExecutionObservation, 0, len(lineage.SyntheticFixtureCommandSpecs()))
	for _, specification := range lineage.SyntheticFixtureCommandSpecs() {
		observation, captureErr := captureSyntheticFixture(executable, specification)
		if captureErr != nil {
			fmt.Fprintf(os.Stderr, "trace lineage fixture-witnesses: case %q: %v\n", specification.CaseID, captureErr)
			return 1
		}
		observations = append(observations, observation)
	}
	fixtures, err := lineage.BuildSyntheticWitnessFixtureSet(observations)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage fixture-witnesses:", err)
		return 1
	}
	return printLineageJSON("trace lineage fixture-witnesses", fixtures)
}

func captureSyntheticFixture(executable string, specification lineage.SyntheticFixtureCommandSpec) (observation lineage.SyntheticExecutionObservation, returnErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "trace", "lineage", "fixture-child", specification.CaseID)
	command.Env = []string{"EVALWITNESS_FIXTURE_CHILD=1"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	stateBefore := append([]byte(nil), specification.StateBefore...)
	stateAfter := append([]byte(nil), specification.StateAfter...)
	if specification.Behavior == lineage.SyntheticStateChange {
		stateFile, err := os.CreateTemp("", "evalwitness-lineage-state-*")
		if err != nil {
			return lineage.SyntheticExecutionObservation{}, fmt.Errorf("create state fixture: %w", err)
		}
		statePath := stateFile.Name()
		defer func() {
			if removeErr := os.Remove(statePath); removeErr != nil && returnErr == nil {
				returnErr = fmt.Errorf("remove state fixture: %w", removeErr)
			}
		}()
		defer func() {
			if closeErr := stateFile.Close(); closeErr != nil && returnErr == nil {
				returnErr = fmt.Errorf("close state fixture: %w", closeErr)
			}
		}()
		if _, err := stateFile.Write(specification.StateBefore); err != nil {
			return lineage.SyntheticExecutionObservation{}, fmt.Errorf("write state fixture: %w", err)
		}
		if err := stateFile.Sync(); err != nil {
			return lineage.SyntheticExecutionObservation{}, fmt.Errorf("sync state fixture: %w", err)
		}
		command.ExtraFiles = []*os.File{stateFile}
		exitStatus, err := runSyntheticFixtureCommand(ctx, command)
		if err != nil {
			return lineage.SyntheticExecutionObservation{}, err
		}
		if _, err := stateFile.Seek(0, io.SeekStart); err != nil {
			return lineage.SyntheticExecutionObservation{}, fmt.Errorf("seek state fixture: %w", err)
		}
		stateAfter, err = io.ReadAll(stateFile)
		if err != nil {
			return lineage.SyntheticExecutionObservation{}, fmt.Errorf("read state fixture: %w", err)
		}
		return syntheticObservation(specification.CaseID, stdout.Bytes(), stderr.Bytes(), exitStatus, stateBefore, stateAfter), nil
	}
	exitStatus, err := runSyntheticFixtureCommand(ctx, command)
	if err != nil {
		return lineage.SyntheticExecutionObservation{}, err
	}
	return syntheticObservation(specification.CaseID, stdout.Bytes(), stderr.Bytes(), exitStatus, stateBefore, stateAfter), nil
}

func runSyntheticFixtureCommand(ctx context.Context, command *exec.Cmd) (int, error) {
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	if ctx.Err() != nil {
		return 0, fmt.Errorf("fixed child exceeded deadline: %w", ctx.Err())
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return 0, fmt.Errorf("start fixed child: %w", err)
	}
	exitStatus := exitError.ExitCode()
	if exitStatus < 0 {
		return 0, errors.New("fixed child exited without a portable exit status")
	}
	return exitStatus, nil
}

func syntheticObservation(caseID string, stdout, stderr []byte, exitStatus int, stateBefore, stateAfter []byte) lineage.SyntheticExecutionObservation {
	return lineage.SyntheticExecutionObservation{
		CaseID: caseID, Stdout: append([]byte(nil), stdout...), Stderr: append([]byte(nil), stderr...), ExitStatus: exitStatus,
		RepositoryStateBefore: append([]byte(nil), stateBefore...), RepositoryStateAfter: append([]byte(nil), stateAfter...),
	}
}

func runTraceLineageFixtureChild(args []string) (exitCode int) {
	if os.Getenv("EVALWITNESS_FIXTURE_CHILD") != "1" || len(args) != 1 {
		fmt.Fprintln(os.Stderr, "trace lineage fixture-child: unavailable outside the fixed fixture harness")
		return 2
	}
	specification, found := syntheticFixtureSpec(args[0])
	if !found {
		fmt.Fprintln(os.Stderr, "trace lineage fixture-child: unknown fixed case")
		return 2
	}
	switch specification.Behavior {
	case lineage.SyntheticDirect:
		return writeSyntheticFixtureOutput(specification)
	case lineage.SyntheticStateChange:
		stateFile := os.NewFile(3, "fixture-state")
		if stateFile == nil {
			fmt.Fprintln(os.Stderr, "state fixture descriptor is unavailable")
			return 70
		}
		defer func() {
			if err := stateFile.Close(); err != nil && exitCode == 0 {
				fmt.Fprintln(os.Stderr, "close state fixture:", err)
				exitCode = 70
			}
		}()
		if err := stateFile.Truncate(0); err != nil {
			fmt.Fprintln(os.Stderr, "truncate state fixture:", err)
			return 70
		}
		if _, err := stateFile.Seek(0, io.SeekStart); err != nil {
			fmt.Fprintln(os.Stderr, "seek state fixture:", err)
			return 70
		}
		if _, err := stateFile.Write(specification.StateAfter); err != nil {
			fmt.Fprintln(os.Stderr, "write state fixture:", err)
			return 70
		}
		if err := stateFile.Sync(); err != nil {
			fmt.Fprintln(os.Stderr, "sync state fixture:", err)
			return 70
		}
		return writeSyntheticFixtureOutput(specification)
	case lineage.SyntheticWrapperPropagates, lineage.SyntheticWrapperMasks:
		return runSyntheticFixtureWrapper(specification)
	default:
		fmt.Fprintln(os.Stderr, "trace lineage fixture-child: unsupported fixed behavior")
		return 70
	}
}

func runSyntheticFixtureWrapper(specification lineage.SyntheticFixtureCommandSpec) int {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve wrapper executable:", err)
		return 70
	}
	inner, found := syntheticFixtureSpec(specification.InnerCaseID)
	if !found || inner.Behavior != lineage.SyntheticDirect {
		fmt.Fprintln(os.Stderr, "fixed wrapper inner case is invalid")
		return 70
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "trace", "lineage", "fixture-child", inner.CaseID)
	command.Env = []string{"EVALWITNESS_FIXTURE_CHILD=1"}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	exitStatus, err := runSyntheticFixtureCommand(ctx, command)
	if err != nil || !bytes.Equal(stdout.Bytes(), inner.Stdout) || !bytes.Equal(stderr.Bytes(), inner.Stderr) || exitStatus != inner.ExitStatus {
		fmt.Fprintln(os.Stderr, "fixed wrapper inner observation is invalid")
		return 70
	}
	if specification.Behavior == lineage.SyntheticWrapperPropagates {
		if _, err := os.Stdout.Write(stdout.Bytes()); err != nil {
			return 70
		}
		if _, err := os.Stderr.Write(stderr.Bytes()); err != nil {
			return 70
		}
		return exitStatus
	}
	return writeSyntheticFixtureOutput(specification)
}

func writeSyntheticFixtureOutput(specification lineage.SyntheticFixtureCommandSpec) int {
	if _, err := os.Stdout.Write(specification.Stdout); err != nil {
		return 70
	}
	if _, err := os.Stderr.Write(specification.Stderr); err != nil {
		return 70
	}
	return specification.ExitStatus
}

func syntheticFixtureSpec(caseID string) (lineage.SyntheticFixtureCommandSpec, bool) {
	for _, specification := range lineage.SyntheticFixtureCommandSpecs() {
		if specification.CaseID == caseID {
			return specification, true
		}
	}
	return lineage.SyntheticFixtureCommandSpec{}, false
}

func runTraceLineageSourceInventory(args []string) int {
	flags := flag.NewFlagSet("trace lineage source-inventory", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage source-inventory: positional arguments are forbidden")
		return 2
	}
	inventory, err := lineage.DefaultSourceInventory()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage source-inventory:", err)
		return 1
	}
	return printLineageJSON("trace lineage source-inventory", inventory)
}

func runTraceLineageSourceSpecifications(args []string) int {
	flags := flag.NewFlagSet("trace lineage source-specifications", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage source-specifications: positional arguments are forbidden")
		return 2
	}
	registry, err := lineage.DefaultTraceSourceSpecificationRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage source-specifications:", err)
		return 1
	}
	return printLineageJSON("trace lineage source-specifications", registry)
}

func runTraceLineageSchemaInventory(args []string) int {
	flags := flag.NewFlagSet("trace lineage schema-inventory", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage schema-inventory: positional arguments are forbidden")
		return 2
	}
	inventory, err := lineage.DefaultSchemaInventory()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage schema-inventory:", err)
		return 1
	}
	return printLineageJSON("trace lineage schema-inventory", inventory)
}

func runTraceLineagePlan(args []string) int {
	flags := flag.NewFlagSet("trace lineage plan", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage plan: positional arguments are forbidden")
		return 2
	}
	plan, err := lineage.DefaultPlan()
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage plan:", err)
		return 1
	}
	return printLineageJSON("trace lineage plan", plan)
}

func runTraceLineageSchema(args []string) int {
	flags := flag.NewFlagSet("trace lineage schema", flag.ContinueOnError)
	documentType := flags.String("type", "plan", "lineage document type")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "trace lineage schema: positional arguments are forbidden")
		return 2
	}
	schema, err := lineage.Schema(*documentType)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage schema:", err)
		return 2
	}
	return printLineageJSON("trace lineage schema", schema)
}

func runTraceLineageValidate(args []string) int {
	flags := flag.NewFlagSet("trace lineage validate", flag.ContinueOnError)
	documentType := flags.String("type", "plan", "lineage document type")
	documentPath := flags.String("document", "", "lineage document (@file or - for stdin)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *documentPath == "" {
		fmt.Fprintln(os.Stderr, "trace lineage validate: --document is required and positional arguments are forbidden")
		return 2
	}
	reader, closeDocument, err := openStudyDocument(*documentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage validate:", err)
		return 2
	}
	defer closeDocument()
	summary, err := lineage.DecodeDocument(*documentType, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace lineage validate:", err)
		return 1
	}
	return printLineageJSON("trace lineage validate", summary)
}

func printLineageJSON(scope string, value any) int {
	encoded, err := lineage.EncodeIndented(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, scope+": encode output:", err)
		return 1
	}
	return writeCommandOutput(scope, encoded)
}
