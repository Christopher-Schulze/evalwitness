package registry

import "testing"

func TestIndexOwnerInspectionAttestationKeepsReadinessCeiling(t *testing.T) {
	record, err := IndexOwnerInspectionAttestation("../../eval/results/relation-owner-inspection-attestation.json")
	if err != nil {
		t.Fatal(err)
	}
	if record.RequiredAssessments != 66 || record.DimensionCount != 16 {
		t.Fatalf("denominators = %+v", record)
	}
	if record.Rankable || record.IndependentlyReproduced || record.CommunityValidated || record.HumanSupported {
		t.Fatalf("promotion leaked: %+v", record)
	}
	if record.HumanStudyStatus != "not_run" || record.ExternalActionStatus != "not_authorized" {
		t.Fatalf("boundaries = %+v", record)
	}
	if record.OverallStatus == "" || record.Digest == "" {
		t.Fatalf("record = %+v", record)
	}
}
