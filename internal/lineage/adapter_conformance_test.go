package lineage

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAdapterConformanceReproducesExactNormativeSurface(t *testing.T) {
	report, err := BuildAdapterConformanceReport()
	if err != nil {
		t.Fatal(err)
	}
	want := AdapterConformanceSummary{
		Vectors: 63, Conformant: 49, KnownMappingGaps: 0,
		ExpectedRejections: 11, NotApplicable: 3,
		Checks: 504, PassedChecks: 504, FailedChecks: 0,
	}
	if report.Summary != want {
		t.Fatalf("adapter-conformance summary = %#v, want %#v", report.Summary, want)
	}
	if report.ClaimBoundary.ResearchOutcome || report.ClaimBoundary.FormatWideClaim || report.ClaimBoundary.ProviderCalls != 0 || report.ClaimBoundary.AgentLaunch {
		t.Fatalf("adapter-conformance claim boundary widened: %#v", report.ClaimBoundary)
	}
}

func TestAdapterConformanceKnownGapsHaveFailedNormativeChecks(t *testing.T) {
	report, err := BuildAdapterConformanceReport()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range report.Cases {
		switch item.Status {
		case AdapterKnownGap:
			if item.KnownMappingGap == "" || failedAdapterChecks(item.Checks) == 0 {
				t.Fatalf("known gap %q lacks a failed normative check: %#v", item.VectorID, item)
			}
		case AdapterConformant:
			if item.KnownMappingGap != "" || failedAdapterChecks(item.Checks) != 0 {
				t.Fatalf("conformant case %q hides a gap: %#v", item.VectorID, item)
			}
		case AdapterExpectedRejection, AdapterNotApplicable:
			if failedAdapterChecks(item.Checks) != 0 {
				t.Fatalf("boundary case %q failed its own expected check", item.VectorID)
			}
		default:
			t.Fatalf("case %q has unknown status %q", item.VectorID, item.Status)
		}
	}
}

func TestAdapterConformanceRoundTripAndTamperRejection(t *testing.T) {
	report, err := BuildAdapterConformanceReport()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded AdapterConformanceReport
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	tampered := decoded
	tampered.Cases = append([]AdapterConformanceCase(nil), decoded.Cases...)
	tampered.Cases[0].Checks = append([]AdapterConformanceCheck(nil), decoded.Cases[0].Checks...)
	tampered.Cases[0].Checks[0].Passed = false
	if err := tampered.Validate(); err == nil {
		t.Fatal("tampered adapter-conformance report was accepted")
	}
	repeated, err := BuildAdapterConformanceReport()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report, repeated) {
		t.Fatal("adapter-conformance generation is not deterministic")
	}
	for _, forbidden := range []string{"fixture-token-value", "Bearer ", "/Users/", "private chain", "chain of thought"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("adapter-conformance report contains forbidden material %q", forbidden)
		}
	}
}
