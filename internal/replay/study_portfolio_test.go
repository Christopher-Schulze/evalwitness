package replay

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildStudyPortfolioStaysIncompleteWithoutExplorer(t *testing.T) {
	capture := buildResponseBundleCapture(t)
	attestation, err := SealCaptureRunAttestation(capture, 2)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := AdmitCaptureResearchLineage(capture, 2)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	attestationPath := filepath.Join(dir, "attestation.json")
	admissionPath := filepath.Join(dir, "admission.json")
	ledgerPath := filepath.Join(dir, "ledger.json")
	bindPath := filepath.Join(dir, "bind.json")
	writeCanonicalFile(t, attestationPath, attestation)
	writeCanonicalFile(t, admissionPath, admission)
	if err := os.WriteFile(ledgerPath, []byte(`{"schema_version":"evalwitness.claim-ledger.v1","claims":[{"claim_id":"CLM-070-002","title":"paired","status":"unsupported"}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	certificate, err := BindStudyEvidence(StudyBindInput{
		CapturePath:     capture,
		AuthorizedCalls: 2,
		AttestationPath: attestationPath,
		AdmissionPath:   admissionPath,
		ClaimLedgerPath: ledgerPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCanonicalFile(t, bindPath, certificate)
	portfolio, err := BuildStudyPortfolio(bindPath, ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if portfolio.BindStatus != StudyBindStatusIncomplete || portfolio.ExplorerPresent || portfolio.Digest == "" {
		t.Fatalf("portfolio = %+v", portfolio)
	}
	if portfolio.ObservedEstimand != EvalTerminalNotLockedEstimand {
		t.Fatalf("observed estimand = %q", portfolio.ObservedEstimand)
	}
	if len(portfolio.Claims) != 1 || portfolio.Claims[0].Status != "unsupported" {
		t.Fatalf("claims = %+v", portfolio.Claims)
	}
}

func TestBuildStudyPortfolioRejectsLedgerSwap(t *testing.T) {
	capture := buildResponseBundleCapture(t)
	attestation, err := SealCaptureRunAttestation(capture, 2)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := AdmitCaptureResearchLineage(capture, 2)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	attestationPath := filepath.Join(dir, "attestation.json")
	admissionPath := filepath.Join(dir, "admission.json")
	ledgerPath := filepath.Join(dir, "ledger.json")
	otherLedger := filepath.Join(dir, "other.json")
	bindPath := filepath.Join(dir, "bind.json")
	writeCanonicalFile(t, attestationPath, attestation)
	writeCanonicalFile(t, admissionPath, admission)
	if err := os.WriteFile(ledgerPath, []byte(`{"schema_version":"evalwitness.claim-ledger.v1","claims":[]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherLedger, []byte(`{"schema_version":"evalwitness.claim-ledger.v1","claims":[{"claim_id":"X","title":"x","status":"supported"}]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	certificate, err := BindStudyEvidence(StudyBindInput{
		CapturePath:     capture,
		AuthorizedCalls: 2,
		AttestationPath: attestationPath,
		AdmissionPath:   admissionPath,
		ClaimLedgerPath: ledgerPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCanonicalFile(t, bindPath, certificate)
	if _, err := BuildStudyPortfolio(bindPath, otherLedger); err == nil {
		t.Fatal("swapped ledger accepted")
	}
}
