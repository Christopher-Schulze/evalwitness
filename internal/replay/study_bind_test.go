package replay

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestBindStudyEvidenceStaysIncompleteWithoutResearchLineage(t *testing.T) {
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
	writeCanonicalFile(t, attestationPath, attestation)
	writeCanonicalFile(t, admissionPath, admission)
	if err := os.WriteFile(ledgerPath, []byte(`{"schema_version":"evalwitness.claim-ledger.v1","claims":[]}`+"\n"), 0o600); err != nil {
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
	if certificate.BindStatus != StudyBindStatusIncomplete {
		t.Fatalf("bind_status = %q", certificate.BindStatus)
	}
	if certificate.Admission != CaptureResearchAdmissionRejected {
		t.Fatalf("admission = %q", certificate.Admission)
	}
	if certificate.EvidenceCeiling != StudyBindEvidenceCeiling {
		t.Fatalf("evidence ceiling = %q", certificate.EvidenceCeiling)
	}
	if _, err := BindStudyEvidence(StudyBindInput{
		CapturePath:     capture,
		AuthorizedCalls: 99,
		AttestationPath: attestationPath,
		AdmissionPath:   admissionPath,
		ClaimLedgerPath: ledgerPath,
	}); err == nil {
		t.Fatal("mismatched authorized-calls accepted")
	}
}

func TestBindStudyEvidenceCompletesStampedResearchCapture(t *testing.T) {
	source := buildResponseBundleCapture(t)
	destination := filepath.Join(t.TempDir(), "stamped.jsonl")
	report, err := StampCaptureResearchLineage(source, destination, researchLineageStampForTest())
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := SealCaptureRunAttestation(destination, report.SourceEntries)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.Status != CaptureRunStatusComplete {
		t.Fatalf("stamped capture-run = %+v", attestation)
	}
	admission, err := AdmitCaptureResearchLineage(destination, report.SourceEntries)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	attestationPath := filepath.Join(dir, "attestation.json")
	admissionPath := filepath.Join(dir, "admission.json")
	ledgerPath := filepath.Join(dir, "ledger.json")
	policyPath := filepath.Join(dir, "policy.json")
	studyPath := filepath.Join(dir, "study.json")
	analysisPath := filepath.Join(dir, "analysis.json")
	writeCanonicalFile(t, attestationPath, attestation)
	writeCanonicalFile(t, admissionPath, admission)
	for _, path := range []string{ledgerPath, policyPath, studyPath, analysisPath} {
		if err := os.WriteFile(path, []byte(`{"ok":true}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	certificate, err := BindStudyEvidence(StudyBindInput{
		CapturePath:         destination,
		AuthorizedCalls:     report.SourceEntries,
		AttestationPath:     attestationPath,
		AdmissionPath:       admissionPath,
		ClaimLedgerPath:     ledgerPath,
		BundlePolicyPath:    policyPath,
		StudyRecordPath:     studyPath,
		OfflineAnalysisPath: analysisPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if certificate.BindStatus != StudyBindStatusComplete || certificate.Admission != CaptureResearchAdmissionAdmitted {
		t.Fatalf("complete bind = %+v", certificate)
	}
	if slices.Contains(certificate.Limitations, "live capture remains eval-terminal; it does not answer the locked identical-response estimand") {
		t.Fatalf("complete bind retained stale estimand limitation: %+v", certificate.Limitations)
	}
}

func writeCanonicalFile(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := protocol.CanonicalMarshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
