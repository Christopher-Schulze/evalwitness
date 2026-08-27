package registry

import "testing"

func TestIndexEmpiricalValidityStaysNotRun(t *testing.T) {
	index, err := IndexEmpiricalValidity("../../eval/results/relation-owner-inspection-attestation.json")
	if err != nil {
		t.Fatal(err)
	}
	if index.OutcomeStatus != "not_run" || index.OutcomeLedgerPresent || index.Empirical || index.Rankable {
		t.Fatalf("index = %+v", index)
	}
	if index.RelationValidityStatus != "revision_required" || index.HumanStudyStatus != "not_run" {
		t.Fatalf("068 projection = %+v", index)
	}
}
