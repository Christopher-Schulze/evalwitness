package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/explorer"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const commandIdenticalResponseBaseRoot = "evalwitness-capsule-3b26fefb5174cc63d03f47f2be5543878c287e978155116b0da6dce85d9ebf19"

func TestBuildClaimReportBindsVerifiedIdenticalResponseEvidence(t *testing.T) {
	fixture := buildClaimRelianceFixture(t)
	baseCapsule := extractCommandIdenticalResponseBase(t, fixture.repositoryRoot)
	governance := filepath.Join(fixture.repositoryRoot, "eval", "governance")
	report, err := buildClaimReportModelWithEvidence(
		context.Background(), fixture.baseCapsulePath, fixture.baseLedgerPath, fixture.repositoryRoot,
		"", "", "", identicalResponseEvidencePaths{
			baseCapsule:        baseCapsule,
			capsule:            filepath.Join(governance, "identical-response-capsule-v5-outer"),
			ledger:             filepath.Join(governance, "identical-response-claim-ledger-v5.json"),
			challengePack:      filepath.Join(governance, "identical-response-claim-challenge-pack-v5.json"),
			reproductionReport: filepath.Join(governance, "identical-response-reproduction-report-v5.json"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.IdenticalResponse == nil || report.IdenticalResponse.Comparison.TaskGroups != 60 ||
		report.IdenticalResponse.Comparison.Disagreements != 7 || report.IdenticalResponse.Challenges.Total != 34 {
		t.Fatalf("identical-response report = %+v", report.IdenticalResponse)
	}
	raw, err := explorer.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := explorer.DecodeReport(raw)
	if err != nil || decoded.IdenticalResponse == nil || decoded.IdenticalResponse.Digest != report.IdenticalResponse.Digest {
		t.Fatalf("identical-response report round trip = %+v / %v", decoded.IdenticalResponse, err)
	}
}

func TestIdenticalResponseEvidencePathsRequireAllOrNone(t *testing.T) {
	complete := identicalResponseEvidencePaths{
		baseCapsule: "base", capsule: "child", ledger: "ledger",
		challengePack: "challenges", reproductionReport: "reproduction",
	}
	if !(identicalResponseEvidencePaths{}).valid() || !complete.valid() {
		t.Fatal("absent or complete identical-response evidence paths were rejected")
	}
	complete.ledger = ""
	if complete.valid() {
		t.Fatal("partial identical-response evidence paths were accepted")
	}
}

func extractCommandIdenticalResponseBase(t *testing.T, repositoryRoot string) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "base")
	_, err := safety.ExtractTarGzip(context.Background(), safety.ArchiveExtractRequest{
		Sources:     []string{filepath.Join(repositoryRoot, "eval", "governance", "identical-response-capsule-v5.tar.gz")},
		Destination: destination, ExpectedRoots: []string{commandIdenticalResponseBaseRoot},
		Limits: safety.DefaultArchiveLimits(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(destination, commandIdenticalResponseBaseRoot)
}
