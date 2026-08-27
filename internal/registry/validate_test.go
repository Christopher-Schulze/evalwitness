package registry

import (
	"bytes"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestValidateIntakeReportAcceptsValidEntry(t *testing.T) {
	report, err := ValidateIntakeReport(validEntry("intake-ok"))
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || report.Digest == "" || report.SchemaVersion != IntakeValidationSchemaVersion {
		t.Fatalf("report = %+v", report)
	}
}

func TestPreflightIntakeRejectsExpiredWithoutCatalog(t *testing.T) {
	entry := validEntry("stale")
	entry.ObservedAt = entry.ObservedAt.Add(-48 * time.Hour)
	entry.ExpiresAt = entry.ObservedAt.Add(time.Hour)
	report, err := PreflightIntake(entry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Valid {
		t.Fatal("expired preflight accepted")
	}
}

func TestIntakeTemplateIsNotSubmittable(t *testing.T) {
	var entry IntakeEntry
	if err := protocol.DecodeStrict(bytes.TrimSpace(IntakeTemplate()), &entry); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIntake(entry); err == nil {
		t.Fatal("intake template accepted as a real submission")
	}
}

func TestValidateIntakeAgainstCatalogRejectsDuplicateEntryID(t *testing.T) {
	entry := validEntry("dup-1")
	report, err := ValidateIntakeAgainstCatalog(entry, []IntakeEntry{entry})
	if err != nil {
		t.Fatal(err)
	}
	if report.Valid {
		t.Fatal("duplicate catalog entry accepted")
	}
	if report.Error == "" {
		t.Fatal("duplicate catalog error missing")
	}
}
