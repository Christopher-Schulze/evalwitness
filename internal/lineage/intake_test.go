package lineage

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestTraceIntakeStopsBeforeUnsupportedLineageClaims(t *testing.T) {
	definition := goldenCaseDefinitions()[0]
	raw, _, err := goldenNativeDocument(preprocess.SourceCodexRollout, definition)
	if err != nil {
		t.Fatal(err)
	}
	result, err := preprocess.ImportTraceBytes([]byte(raw), preprocess.DefaultTraceImportOptions())
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildVerificationLineageTraceIntakeReport(result)
	if err != nil {
		t.Fatal(err)
	}
	if report.CapabilityPlan.Status != "development_vector_available" || report.ConformancePlan.DevelopmentVectors != 21 ||
		report.LossPlan.CanClassifyTerminalState || report.LossPlan.CanBuildBOM || report.ClaimBoundary.AnalysisPerformed {
		t.Fatalf("trace intake crossed its missing-witness boundary: %#v", report)
	}
}

func TestTraceIntakeRejectsResealedTerminalPromotion(t *testing.T) {
	definition := goldenCaseDefinitions()[0]
	raw, _, err := goldenNativeDocument(preprocess.SourceCodexRollout, definition)
	if err != nil {
		t.Fatal(err)
	}
	result, err := preprocess.ImportTraceBytes([]byte(raw), preprocess.DefaultTraceImportOptions())
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildVerificationLineageTraceIntakeReport(result)
	if err != nil {
		t.Fatal(err)
	}
	report.LossPlan.CanClassifyTerminalState = true
	report.LossPlan.CanBuildBOM = true
	report.Digest, err = traceIntakeReportDigest(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(); err == nil {
		t.Fatal("resealed intake report invented a terminal result and BOM")
	}
}

func TestTraceIntakeRejectsContentBearingPrivacyClass(t *testing.T) {
	definition := goldenCaseDefinitions()[0]
	raw, _, err := goldenNativeDocument(preprocess.SourceCodexRollout, definition)
	if err != nil {
		t.Fatal(err)
	}
	options := preprocess.DefaultTraceImportOptions()
	options.Privacy = preprocess.PrivacyContent
	result, err := preprocess.ImportTraceBytes([]byte(raw), options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildVerificationLineageTraceIntakeReport(result); err == nil {
		t.Fatal("lineage intake accepted a content-bearing privacy class")
	}
}
