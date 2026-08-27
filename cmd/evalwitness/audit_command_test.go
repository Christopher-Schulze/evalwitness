package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

func TestRunAuditExitContractAndFormats(t *testing.T) {
	dir := t.TempDir()
	metric := "0.12"
	p, err := profile.Build("audit-profile", "evalwitness.protocol.v1", "routeA", []profile.Dimension{
		{ID: "calibration", Status: profile.StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "calibration.metrics.ece", Denominator: 100, SampleUnit: "task"},
	})
	if err != nil {
		t.Fatalf("build %v", err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(profilePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(dir, "pass-policy.json")
	passPolicy := `{"version":"v1","requirements":{"calibration":"measured"}}`
	if err := os.WriteFile(policyPath, []byte(passPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runCapture(t, func() int { return runAudit([]string{"--policy", policyPath, "--profile", profilePath}) })
	pol, errPol := profile.NewPolicy("v1", map[string]string{"calibration": "measured"})
	if errPol != nil {
		t.Fatal(errPol)
	}
	if out.code != audit.ExitPass || !contains(out.stdout, `"schema_version":"evalwitness.audit-result.v1"`) || !contains(out.stdout, `"pass":true`) || !contains(out.stdout, pol.Digest) {
		t.Fatalf("pass exit %d stdout %s stderr %s", out.code, out.stdout, out.stderr)
	}
	failPolicyPath := filepath.Join(dir, "fail-policy.json")
	failPolicy := `{"version":"v1","requirements":{"calibration":"failed"}}`
	if err := os.WriteFile(failPolicyPath, []byte(failPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	outFail := runCapture(t, func() int { return runAudit([]string{"--policy", failPolicyPath, "--profile", profilePath}) })
	if outFail.code != audit.ExitPolicyFailed || !contains(outFail.stdout, `"pass":false`) {
		t.Fatalf("fail exit %d stdout %s stderr %s", outFail.code, outFail.stdout, outFail.stderr)
	}
	junitOut := runCapture(t, func() int {
		return runAudit([]string{"--policy", failPolicyPath, "--profile", profilePath, "--format", "junit"})
	})
	if junitOut.code != audit.ExitPolicyFailed || !contains(junitOut.stdout, "<failure") {
		t.Fatalf("junit exit %d stdout %s", junitOut.code, junitOut.stdout)
	}
	mdOut := runCapture(t, func() int {
		return runAudit([]string{"--policy", failPolicyPath, "--profile", profilePath, "--format", "markdown"})
	})
	if mdOut.code != audit.ExitPolicyFailed || !contains(mdOut.stdout, "# EvalWitness offline audit: FAIL") {
		t.Fatalf("markdown exit %d stdout %s", mdOut.code, mdOut.stdout)
	}
	// Tampered profile digest must be rejected with invalid input
	tampered := p
	tampered.Digest = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	tamperedRaw, _ := json.Marshal(tampered)
	tamperedPath := filepath.Join(dir, "tampered.json")
	_ = os.WriteFile(tamperedPath, tamperedRaw, 0o644)
	outTamper := runCapture(t, func() int { return runAudit([]string{"--policy", policyPath, "--profile", tamperedPath}) })
	if outTamper.code != audit.ExitInvalidInput {
		t.Fatalf("tampered exit %d want %d stderr %s", outTamper.code, audit.ExitInvalidInput, outTamper.stderr)
	}
	// Unknown field in policy must be rejected
	unknownPath := filepath.Join(dir, "unknown.json")
	unknown := `{"version":"v1","requirements":{"calibration":"measured"},"bogus":true}`
	_ = os.WriteFile(unknownPath, []byte(unknown), 0o644)
	outUnknown := runCapture(t, func() int { return runAudit([]string{"--policy", unknownPath, "--profile", profilePath}) })
	if outUnknown.code != audit.ExitInvalidInput {
		t.Fatalf("unknown field exit %d want %d stderr %s", outUnknown.code, audit.ExitInvalidInput, outUnknown.stderr)
	}
}

func TestRunAuditSARIFFormat(t *testing.T) {
	dir := t.TempDir()
	metric := "0.12"
	p, err := profile.Build("audit-profile", "evalwitness.protocol.v1", "routeA", []profile.Dimension{
		{ID: "calibration", Status: profile.StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "calibration.metrics.ece", Denominator: 100, SampleUnit: "task"},
	})
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(dir, "profile.json")
	raw, _ := json.Marshal(p)
	if err := os.WriteFile(profilePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	failPolicyPath := filepath.Join(dir, "fail.json")
	if err := os.WriteFile(failPolicyPath, []byte(`{"version":"v1","requirements":{"calibration":"failed"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runCapture(t, func() int {
		return runAudit([]string{"--policy", failPolicyPath, "--profile", profilePath, "--format", "sarif"})
	})
	if out.code != audit.ExitPolicyFailed {
		t.Fatalf("sarif exit %d stdout %s stderr %s", out.code, out.stdout, out.stderr)
	}
	if !contains(out.stdout, `"version":"2.1.0"`) || !contains(out.stdout, `"results":[]`) {
		t.Fatalf("statistical-only failure must emit empty SARIF results: %s", out.stdout)
	}
}

func TestRunAuditExplainPath(t *testing.T) {
	dir := t.TempDir()
	metric := "0.12"
	p, err := profile.Build("audit-profile", "evalwitness.protocol.v1", "routeA", []profile.Dimension{
		{ID: "calibration", Status: profile.StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "calibration.metrics.ece", Denominator: 100, SampleUnit: "task"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	profilePath := filepath.Join(dir, "profile.json")
	_ = os.WriteFile(profilePath, raw, 0o644)
	failPolicyPath := filepath.Join(dir, "fail.json")
	_ = os.WriteFile(failPolicyPath, []byte(`{"version":"v1","requirements":{"calibration":"failed"}}`), 0o644)
	out := runCapture(t, func() int { return runAudit([]string{"--policy", failPolicyPath, "--profile", profilePath}) })
	if out.code != audit.ExitPolicyFailed {
		t.Fatalf("exit %d", out.code)
	}
	for _, want := range []string{
		`"check":"policy v1 requires calibration"`,
		"dimension calibration status measured",
		"capsule expression calibration.metrics.ece",
		"reproduce offline: evalwitness audit --policy",
	} {
		if !contains(out.stdout, want) {
			t.Fatalf("explanation missing %q in %s", want, out.stdout)
		}
	}
}
