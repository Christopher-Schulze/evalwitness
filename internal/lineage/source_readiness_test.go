package lineage

import "testing"

func TestSourceReadinessAuditPreservesZeroResearchDenominator(t *testing.T) {
	audit, err := BuildVerificationLineageSourceReadinessAudit("../..")
	if err != nil {
		t.Fatal(err)
	}
	if audit.Summary.ResearchAdmittedCandidates != 0 || audit.Summary.CalibrationEligibleCandidates != 0 ||
		audit.Summary.LockedTestEligibleCandidates != 0 || audit.Summary.AdmittedTaskGroups != 0 || audit.EmpiricalDenominatorsAvailable {
		t.Fatalf("source readiness invented an empirical denominator: %#v", audit)
	}
	if err := VerifyVerificationLineageSourceReadinessAudit("../..", audit); err != nil {
		t.Fatal(err)
	}
}

func TestSourceReadinessAuditRejectsResealedAdmissionPromotion(t *testing.T) {
	audit, err := BuildVerificationLineageSourceReadinessAudit("../..")
	if err != nil {
		t.Fatal(err)
	}
	audit.Candidates[3].ResearchAdmitted = true
	audit.Summary.ResearchAdmittedCandidates = 1
	audit.Digest, err = sourceReadinessAuditDigest(audit)
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.Validate(); err == nil {
		t.Fatal("resealed unauthorized source promotion was accepted")
	}
}

func TestSourceReadinessAuditRejectsInventedTaskGroupCounts(t *testing.T) {
	audit, err := BuildVerificationLineageSourceReadinessAudit("../..")
	if err != nil {
		t.Fatal(err)
	}
	audit.Formats[1].CalibrationTaskGroups = 20
	audit.Summary.AdmittedTaskGroups = 20
	audit.Digest, err = sourceReadinessAuditDigest(audit)
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.Validate(); err == nil {
		t.Fatal("invented calibration task-group denominator was accepted")
	}
}
