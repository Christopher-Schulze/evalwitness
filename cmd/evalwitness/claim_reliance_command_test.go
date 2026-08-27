package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/explorer"
	"github.com/Christopher-Schulze/evalwitness/internal/reliance"
)

type claimRelianceFixture struct {
	repositoryRoot   string
	baseCapsulePath  string
	baseLedgerPath   string
	childCapsulePath string
	childLedgerPath  string
}

func TestBuildClaimReportBindsVerifiedRelianceEvidence(t *testing.T) {
	fixture := buildClaimRelianceFixture(t)
	report, err := buildClaimReportModelWithReliance(context.Background(), fixture.baseCapsulePath,
		fixture.baseLedgerPath, fixture.repositoryRoot, fixture.childCapsulePath, fixture.childLedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if report.Reliance == nil || len(report.Reliance.Outcomes) != 98 || report.Reliance.ProviderCalls != 0 ||
		report.Reliance.NetworkRequired || !report.Reliance.GlobalScoreProhibited {
		t.Fatalf("claim report reliance boundary = %+v", report.Reliance)
	}
	raw, err := explorer.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := explorer.DecodeReport(raw)
	if err != nil || decoded.Reliance == nil || decoded.Reliance.Digest != report.Reliance.Digest {
		t.Fatalf("claim report reliance round trip = %+v / %v", decoded.Reliance, err)
	}
	html, metadata, err := explorer.RenderHTML(decoded)
	if err != nil || len(html) != metadata.Bytes || metadata.ReportDigest != decoded.Digest ||
		!bytes.Contains(html, []byte(base64.StdEncoding.EncodeToString(raw))) {
		t.Fatalf("claim report reliance render metadata = %+v / %v", metadata, err)
	}
}

func TestValidReliancePathsRequiresAnExactPair(t *testing.T) {
	for _, test := range []struct {
		name    string
		capsule string
		ledger  string
		want    bool
	}{
		{"absent", "", "", true},
		{"paired", "capsule", "ledger", true},
		{"capsule-only", "capsule", "", false},
		{"ledger-only", "", "ledger", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validReliancePaths(test.capsule, test.ledger); got != test.want {
				t.Fatalf("validReliancePaths(%q, %q) = %t, want %t", test.capsule, test.ledger, got, test.want)
			}
		})
	}
}

func buildClaimRelianceFixture(t *testing.T) claimRelianceFixture {
	t.Helper()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err = filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	base, baseCapsulePath, baseLedgerPath := loadBaseClaimEvidence(t, repositoryRoot)
	childCapsulePath, childLedgerPath := writeRelianceClaimEvidence(t, repositoryRoot, root, base)
	return claimRelianceFixture{
		repositoryRoot: repositoryRoot, baseCapsulePath: baseCapsulePath, baseLedgerPath: baseLedgerPath,
		childCapsulePath: childCapsulePath, childLedgerPath: childLedgerPath,
	}
}

func loadBaseClaimEvidence(
	t *testing.T,
	repositoryRoot string,
) (capsule.ReferencePackage, string, string) {
	t.Helper()
	capsulePath := filepath.Join(repositoryRoot, "eval", "results", "evidence-reliance-base-capsule-v1")
	base, err := loadRelianceBaseCapsule(context.Background(), capsulePath)
	if err != nil {
		t.Fatal(err)
	}
	ledgerPath := filepath.Join(repositoryRoot, "eval", "results", "evidence-reliance-base-claims-v1.json")
	return base, capsulePath, ledgerPath
}

func writeRelianceClaimEvidence(
	t *testing.T,
	repositoryRoot string,
	root string,
	base capsule.ReferencePackage,
) (string, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repositoryRoot, "eval", "results", "evidence-reliance-map-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	value, err := reliance.DecodeEvidenceRelianceMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	child, err := reliance.BuildEvidenceRelianceCapsule(base, value)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := reliance.BuildEvidenceRelianceLedger(context.Background(), base, child)
	if err != nil {
		t.Fatal(err)
	}
	capsulePath, ledgerPath := filepath.Join(root, "reliance-capsule"), filepath.Join(root, "reliance-claims.json")
	writeClaimEvidence(t, capsulePath, ledgerPath, child, ledger)
	return capsulePath, ledgerPath
}

func writeClaimEvidence(
	t *testing.T,
	capsulePath string,
	ledgerPath string,
	value capsule.ReferencePackage,
	ledger claimledger.Ledger,
) {
	t.Helper()
	if err := capsule.WriteDirectory(context.Background(), capsulePath, value.Registry, value.Manifest, value.Payloads); err != nil {
		t.Fatal(err)
	}
	raw, err := claimledger.EncodeLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
