package study

import "testing"

func TestBuildIdenticalResponseRedistributionRight(t *testing.T) {
	record, err := BuildIdenticalResponseRedistributionRight()
	if err != nil {
		t.Fatalf("build redistribution-right record: %v", err)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("validate redistribution-right record: %v", err)
	}
	if len(record.Evidence) != 2 || record.Summary.Routes != 2 || record.Summary.OutputRightsAssigned != 2 {
		t.Fatalf("expected two routes with assigned output rights, got %d/%d", record.Summary.Routes, record.Summary.OutputRightsAssigned)
	}
	for _, evidence := range record.Evidence {
		if evidence.RetrievalDate != "2026-08-18" {
			t.Fatalf("route %q retrieval date = %q, want frozen 2026-08-18", evidence.RouteID, evidence.RetrievalDate)
		}
		if evidence.SPDXExpression != "NOASSERTION" {
			t.Fatalf("route %q must not fabricate an SPDX expression for a ToS assignment", evidence.RouteID)
		}
	}
}

func TestIdenticalResponseRedistributionRightRejectsTampering(t *testing.T) {
	record, err := BuildIdenticalResponseRedistributionRight()
	if err != nil {
		t.Fatalf("build redistribution-right record: %v", err)
	}

	tampered := record
	tampered.Evidence[0].SPDXExpression = "MIT"
	if err := tampered.Validate(); err == nil {
		t.Fatalf("fabricated SPDX expression must be rejected")
	}

	tampered = record
	tampered.Evidence[0].AssignmentClause = "no assignment language here"
	if err := tampered.Validate(); err == nil {
		t.Fatalf("clause without assignment language must be rejected")
	}

	tampered = record
	tampered.Summary.OutputRightsAssigned = 1
	if err := tampered.Validate(); err == nil {
		t.Fatalf("miscounted output-rights summary must be rejected")
	}

	tampered = record
	tampered.Evidence = tampered.Evidence[:1]
	if err := tampered.Validate(); err == nil {
		t.Fatalf("dropped evidence route must be rejected")
	}
}
