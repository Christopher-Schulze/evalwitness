package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRelianceCapsulePublicationBuildsAndVerifiesFrozenParent(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	results := filepath.Join(repositoryRoot, "eval", "results")
	root := t.TempDir()
	buildInputs := relianceCapsuleBuildInputs{
		baseCapsulePath: filepath.Join(results, "evidence-reliance-base-capsule-v1"),
		mapPath:         filepath.Join(results, "evidence-reliance-map-v1.json"),
		destination:     filepath.Join(root, "reliance-capsule"),
	}
	buildReport, err := buildRelianceCapsulePublication(context.Background(), buildInputs)
	if err != nil {
		t.Fatal(err)
	}
	if !buildReport.Offline || buildReport.NetworkRequired || buildReport.ProviderCalls != 0 ||
		buildReport.BaseCapsuleID == buildReport.CapsuleID {
		t.Fatalf("reliance capsule build boundary = %+v", buildReport)
	}
	verifyInputs := relianceCapsuleVerifyInputs{
		baseCapsulePath: buildInputs.baseCapsulePath, source: buildInputs.destination,
		mapPath: buildInputs.mapPath, ledgerPath: filepath.Join(results, "evidence-reliance-claims-v1.json"),
		profilePath:  filepath.Join(results, "evidence-reliance-profile-v1.json"),
		paperPath:    filepath.Join(results, "evidence-reliance-paper-v1.json"),
		explorerPath: filepath.Join(results, "evidence-reliance-explorer-v1.json"),
	}
	verifyReport, err := verifyRelianceCapsulePublication(context.Background(), verifyInputs)
	if err != nil {
		t.Fatal(err)
	}
	assertRelianceVerifyReport(t, verifyReport)
	assertRelianceBuildLedgerMatchesPublication(t, buildReport.LedgerPath, verifyInputs.ledgerPath)
	if _, err := buildRelianceCapsulePublication(context.Background(), buildInputs); err == nil {
		t.Fatal("reliance capsule build overwrote an existing publication target")
	}
}

func assertRelianceVerifyReport(t *testing.T, report relianceCapsuleVerifyReport) {
	t.Helper()
	if report.ArtifactsVerified != 5 || report.Dimensions != 98 ||
		report.ClaimsSupported != 8 || report.ClaimsUnsupported != 3 || !report.Offline ||
		report.NetworkRequired || report.ProviderCalls != 0 {
		t.Fatalf("reliance capsule verification boundary = %+v", report)
	}
}

func TestRelianceCapsuleCommandsRejectIncompletePaths(t *testing.T) {
	if code := runRelianceCapsuleBuild([]string{"--map", "map.json"}); code != 2 {
		t.Fatalf("incomplete reliance capsule build exit code = %d", code)
	}
	if code := runRelianceCapsuleVerify([]string{"--source", "capsule"}); code != 2 {
		t.Fatalf("incomplete reliance capsule verify exit code = %d", code)
	}
}

func assertRelianceBuildLedgerMatchesPublication(t *testing.T, builtPath string, publishedPath string) {
	t.Helper()
	built, err := os.ReadFile(builtPath)
	if err != nil {
		t.Fatal(err)
	}
	published, err := os.ReadFile(publishedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(built, published) {
		t.Fatal("built reliance ledger differs from the canonical public ledger")
	}
}
