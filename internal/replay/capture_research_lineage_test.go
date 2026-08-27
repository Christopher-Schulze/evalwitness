package replay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

func TestStampCaptureResearchLineageCompletesIncompleteCapture(t *testing.T) {
	source := buildResponseBundleCapture(t)
	destination := filepath.Join(t.TempDir(), "stamped.jsonl")
	stamp := researchLineageStampForTest()
	report, err := StampCaptureResearchLineage(source, destination, stamp)
	if err != nil {
		t.Fatal(err)
	}
	if report.StampedEntries < 1 || report.Admission != CaptureResearchAdmissionAdmitted {
		t.Fatalf("stamp report = %+v", report)
	}
	attestation, err := SealCaptureRunAttestation(destination, report.SourceEntries)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.Status != CaptureRunStatusComplete || !attestation.ResearchLineageComplete {
		t.Fatalf("stamped capture-run = %+v", attestation)
	}
	if _, err := StampCaptureResearchLineage(source, source, stamp); err == nil {
		t.Fatal("in-place stamp was accepted")
	}
}

func TestAdmitCaptureResearchLineageRejectsIncompleteCapture(t *testing.T) {
	path := buildResponseBundleCapture(t)
	admission, err := AdmitCaptureResearchLineage(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if admission.Admission != CaptureResearchAdmissionRejected ||
		admission.RequiredAction != CaptureResearchRequiredRecapture ||
		admission.CompleteResearchEntries != 0 {
		t.Fatalf("admission = %+v", admission)
	}
	complete := buildResponseBundleResearchCapture(t)
	admitted, err := AdmitCaptureResearchLineage(complete, 1)
	if err != nil {
		t.Fatal(err)
	}
	if admitted.Admission != CaptureResearchAdmissionAdmitted {
		t.Fatalf("research capture admission = %+v", admitted)
	}
}

func TestStampCaptureResearchLineageRefusesExistingDestination(t *testing.T) {
	source := buildResponseBundleCapture(t)
	destination := filepath.Join(t.TempDir(), "exists.jsonl")
	if err := os.WriteFile(destination, []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StampCaptureResearchLineage(source, destination, researchLineageStampForTest()); err == nil {
		t.Fatal("existing destination was overwritten")
	}
}

func researchLineageStampForTest() CaptureResearchLineageStamp {
	return CaptureResearchLineageStamp{
		Lineage: provider.RequestLineage{
			CriterionID: "criterion", SamplingSlot: "criterion@r0", Entrypoint: "verify",
			AuditCaseID: "case-001", SourceTraceHash: strings.Repeat("a", 64),
			TraceMapHash: strings.Repeat("b", 64), MutationID: "unmodified",
			StudyCellID: "exact-replay", PolicyHash: strings.Repeat("c", 64),
		},
		EvidenceBindings: []provider.EvidenceBinding{{
			InputSlot: "trajectory_0", SourceDigest: strings.Repeat("d", 64),
			CanonicalDigest: strings.Repeat("e", 64), IngestionDigest: strings.Repeat("f", 64),
			TraceEnvelopeDigest: strings.Repeat("1", 64), MappingReportDigest: strings.Repeat("2", 64),
			MappingPolicyVersion: "evalwitness.trace-mapping.v1",
		}},
		CapabilityAttestationID: "att-" + strings.Repeat("3", 64),
		ServedModel:             "bundle-model",
		CheckpointAssertion:     "served-alias-only",
	}
}
