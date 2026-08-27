package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/explorer"
)

const claimRenderResultSchemaVersion = "evalwitness.claim-render-result.v1"

type claimRenderResult struct {
	SchemaVersion       string `json:"schema_version"`
	Destination         string `json:"destination"`
	Bytes               int    `json:"bytes"`
	HTMLSHA256          string `json:"html_sha256"`
	RendererDigest      string `json:"renderer_digest"`
	ReportDigest        string `json:"report_digest"`
	ReportPayloadSHA256 string `json:"report_payload_sha256"`
	NetworkRequired     bool   `json:"network_required"`
	ProviderCalls       int    `json:"provider_calls"`
}

func runClaimRender(args []string) int {
	flags := flag.NewFlagSet("claim render", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "verified public capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	repositoryRoot := flags.String("repository-root", "", "repository root containing the bound TASK 069 release files")
	destination := flags.String("destination", "", "new self-contained HTML destination")
	relianceCapsulePath := flags.String("reliance-capsule", "", "optional verified TASK 065 reliance capsule directory")
	relianceLedgerPath := flags.String("reliance-ledger", "", "optional canonical TASK 065 reliance claim ledger")
	profilePath := flags.String("profile", "", "optional verified TASK 058 reliability profile JSON")
	identicalBasePath := flags.String("identical-response-base-capsule", "", "optional verified TASK 070 response-bundle capsule directory")
	identicalCapsulePath := flags.String("identical-response-capsule", "", "optional verified TASK 070 outer capsule directory")
	identicalLedgerPath := flags.String("identical-response-ledger", "", "optional canonical TASK 070 claim ledger")
	identicalChallengePath := flags.String("identical-response-challenge-pack", "", "optional canonical TASK 070 challenge pack")
	identicalReproductionPath := flags.String("identical-response-reproduction-report", "", "optional verified TASK 070 reproduction report")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim render:", err)
		return 2
	}
	identicalPaths := identicalResponseEvidencePaths{
		baseCapsule: *identicalBasePath, capsule: *identicalCapsulePath, ledger: *identicalLedgerPath,
		challengePack: *identicalChallengePath, reproductionReport: *identicalReproductionPath,
	}
	if *capsulePath == "" || *ledgerPath == "" || *repositoryRoot == "" || *destination == "" || flags.NArg() != 0 ||
		!validReliancePaths(*relianceCapsulePath, *relianceLedgerPath) || !identicalPaths.valid() {
		fmt.Fprintln(os.Stderr, "claim render: --capsule, --ledger, --repository-root, and --destination are required; reliance paths must be supplied together; all five identical-response paths must be supplied together; positional arguments are forbidden")
		return 2
	}
	report, err := buildClaimReportModelWithEvidence(context.Background(), *capsulePath, *ledgerPath, *repositoryRoot,
		*relianceCapsulePath, *relianceLedgerPath, *profilePath, identicalPaths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim render:", err)
		return 1
	}
	html, metadata, err := explorer.RenderHTML(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim render:", err)
		return 1
	}
	if err := writeNewPublicCommandFile(*destination, html); err != nil {
		fmt.Fprintln(os.Stderr, "claim render:", err)
		return 1
	}
	result := claimRenderResult{
		SchemaVersion: claimRenderResultSchemaVersion, Destination: *destination,
		Bytes: metadata.Bytes, HTMLSHA256: metadata.HTMLSHA256, RendererDigest: metadata.RendererDigest,
		ReportDigest: metadata.ReportDigest, ReportPayloadSHA256: metadata.ReportPayloadSHA256,
		NetworkRequired: false, ProviderCalls: 0,
	}
	return writeCanonicalCommandOutput("claim render", result)
}
