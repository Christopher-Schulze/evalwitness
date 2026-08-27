package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/explorer"
)

func TestBuildClaimReportLoadsVerifiedDiskEvidence(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err = filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := capsule.BuildReferencePackage(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := claimledger.DefaultLedger(reference.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	ledgerRaw, err := claimledger.EncodeLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	capsulePath := filepath.Join(root, "capsule")
	ledgerPath := filepath.Join(root, "claims.json")
	if err := capsule.WriteDirectory(
		context.Background(), capsulePath, reference.Registry, reference.Manifest, reference.Payloads,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath, ledgerRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := buildClaimReportModel(context.Background(), capsulePath, ledgerPath, repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	reportRaw, err := explorer.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := explorer.DecodeReport(reportRaw)
	if err != nil {
		t.Fatal(err)
	}
	if report.Capsule.CapsuleID != reference.Manifest.CapsuleID ||
		decoded.Capsule.LedgerDigest != ledger.Digest || report.Release.FilesVerified != 20 {
		t.Fatal("disk-backed claim report is detached from its verified inputs")
	}
	html, metadata, err := explorer.RenderHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(html) != metadata.Bytes || metadata.ReportDigest != report.Digest {
		t.Fatal("disk-backed claim render metadata differs from its verified report")
	}
}

func TestClaimRenderDestinationRefusesOverwrite(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "explorer.html")
	first := []byte("first immutable render")
	if err := writeNewPublicCommandFile(destination, first); err != nil {
		t.Fatal(err)
	}
	if err := writeNewPublicCommandFile(destination, []byte("replacement")); err == nil {
		t.Fatal("existing claim-render destination was overwritten")
	}
	observed, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(observed) != string(first) {
		t.Fatalf("existing claim-render destination changed to %q", observed)
	}
}
