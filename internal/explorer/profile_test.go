package explorer

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

func TestAddProfileAttachesVerifiedProfileToReport(t *testing.T) {
	report, _, _, _, _ := referenceReport(t)
	metric := "0.12"
	p, err := profile.Build("test-profile", "evalwitness.protocol.v1", "routeA", []profile.Dimension{
		{ID: "calibration", Status: profile.StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "calibration.metrics.ece", Denominator: 100, SampleUnit: "task"},
	})
	if err != nil {
		t.Fatalf("build %v", err)
	}
	sealed, err := AddProfile(report, p)
	if err != nil {
		t.Fatalf("add %v", err)
	}
	if sealed.Profile == nil {
		t.Fatal("profile not attached")
	}
	if sealed.Profile.Report.Digest != p.Digest || sealed.Profile.Markdown == "" || sealed.Profile.Report.Text == "" {
		t.Fatalf("view %+v", sealed.Profile)
	}
	if err := sealed.Validate(); err != nil {
		t.Fatalf("sealed validate %v", err)
	}
	// Double attach must fail
	if _, err := AddProfile(sealed, p); err == nil {
		t.Fatal("double attach should fail")
	}
	// Tampered digest must fail
	bad := p
	bad.Digest = "deadbeef"
	if _, err := AddProfile(report, bad); err == nil {
		t.Fatal("tampered digest should fail")
	}
}
