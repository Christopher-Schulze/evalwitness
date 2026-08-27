package replay

import (
	"testing"
)

func TestCaptureRunAttestationMarksIncompleteWithoutResearchLineage(t *testing.T) {
	path := buildResponseBundleCapture(t)
	attestation, err := SealCaptureRunAttestation(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.InspectionMode != CaptureRunInspectionExactReplay {
		t.Fatalf("inspection mode = %q", attestation.InspectionMode)
	}
	if !attestation.AttemptLedgerReconciled || attestation.ResearchLineageComplete {
		t.Fatalf("reconcile/lineage = %v/%v", attestation.AttemptLedgerReconciled, attestation.ResearchLineageComplete)
	}
	if attestation.Status != CaptureRunStatusIncomplete {
		t.Fatalf("status = %q", attestation.Status)
	}
	if err := VerifyCaptureRunAttestation(path, attestation); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRunAttestationCompletesResearchCapture(t *testing.T) {
	path := buildResponseBundleResearchCapture(t)
	attestation, err := SealCaptureRunAttestation(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.Status != CaptureRunStatusComplete || !attestation.ResearchLineageComplete ||
		!attestation.AttemptLedgerReconciled || attestation.InspectionMode != CaptureRunInspectionExactReplay {
		t.Fatalf("attestation = %+v", attestation)
	}
	if err := VerifyCaptureRunAttestation(path, attestation); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRunAttestationRejectsCallBudgetMismatchAndTamperedDigest(t *testing.T) {
	path := buildResponseBundleCapture(t)
	attestation, err := SealCaptureRunAttestation(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	mismatch, err := SealCaptureRunAttestation(path, 99)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch.AttemptLedgerReconciled || mismatch.Status != CaptureRunStatusIncomplete {
		t.Fatalf("mismatch attestation = %+v", mismatch)
	}
	attestation.Digest = "00" + attestation.Digest[2:]
	if err := VerifyCaptureRunAttestation(path, attestation); err == nil {
		t.Fatal("tampered digest verified")
	}
}
