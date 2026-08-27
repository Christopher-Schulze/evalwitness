package registry

import "testing"

func TestIndexScarcityNegativeEvidencePinsCommittedFunnel(t *testing.T) {
	record, err := IndexScarcityNegativeEvidence("../../eval/results/relation-scarcity-negative-evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	if record.Attempted != 198 || record.Admitted != 3 || record.Rejected != 195 {
		t.Fatalf("funnel = %d/%d/%d", record.Attempted, record.Admitted, record.Rejected)
	}
	if record.Development != 2 || record.Calibration != 1 || record.Test != 0 {
		t.Fatalf("roles = %d/%d/%d", record.Development, record.Calibration, record.Test)
	}
	if record.Rankable || record.VerifierScore || record.Digest == "" {
		t.Fatalf("record = %+v", record)
	}
	if record.PackageFormat != "evalwitness.relation-pilot-package.v5" || len(record.ParentDigests) != 6 {
		t.Fatalf("package-format-v5 chain = %+v", record)
	}
}
