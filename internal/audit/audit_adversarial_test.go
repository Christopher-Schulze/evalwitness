package audit

import (
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

func TestPlanRejectsTamperedPolicyDigest(t *testing.T) {
	metric := "1"
	p, err := profile.Build("p", "v1", "r", []profile.Dimension{{ID: "x", Status: profile.StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	if err != nil {
		t.Fatal(err)
	}
	pol, err := profile.NewPolicy("v1", map[string]string{"x": "measured"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := Plan(pol, p)
	if err != nil || len(steps) != 1 || steps[0].Digest != pol.Digest {
		t.Fatalf("plan %v %+v", err, steps)
	}
	forged := pol
	forged.Digest = "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := Plan(forged, p); err == nil {
		t.Fatal("declared digest mismatch must fail")
	}
}

func TestEncodeJUnitKeepsFailuresAggregateWithoutFakeLocations(t *testing.T) {
	r := Result{
		SchemaVersion: SchemaVersion,
		Pass:          false,
		PolicyDigest:  "abc123",
		Fails:         []string{"selective_risk want <= 0.25 got 0.41 (statistical route failure, no source location)"},
	}
	b, err := EncodeJUnit(r)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	if !strings.Contains(out, "<failure") || !strings.Contains(out, xmlEscape(r.Fails[0])) {
		t.Fatalf("junit missing failure body: %s", out)
	}
	if strings.Contains(out, "file=") || strings.Contains(out, "<location") {
		t.Fatalf("statistical failure must not gain a fake source location: %s", out)
	}
}

func TestHasSARIFFindingsOnlyForFileLocalFindings(t *testing.T) {
	statistical := []string{"coverage below minimum (aggregate)"}
	fileLocal := []string{"unsupported config file=config/verifier.json line=12"}
	if HasSARIFFindings(statistical) {
		t.Fatal("aggregate statistical failures must not produce SARIF")
	}
	if !HasSARIFFindings(fileLocal) {
		t.Fatal("file-local findings must produce SARIF")
	}
}

func TestEncodeMarkdownRendersFailsAndPasses(t *testing.T) {
	pass := Result{SchemaVersion: SchemaVersion, Pass: true, Offline: true, PolicyDigest: "d1", ProfileDigest: "d2"}
	b, err := EncodeMarkdown(pass)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "PASS") {
		t.Fatalf("markdown pass %s", b)
	}
	fail := Result{SchemaVersion: SchemaVersion, Pass: false, PolicyDigest: "d1", Fails: []string{"calibration want measured got failed"}}
	bf, err := EncodeMarkdown(fail)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bf), "FAIL") || !strings.Contains(string(bf), "want measured got failed") {
		t.Fatalf("markdown fail %s", bf)
	}
}

func TestEncodeCanonicalRejectsWrongSchema(t *testing.T) {
	if _, err := EncodeCanonical(Result{SchemaVersion: "bogus"}); err == nil {
		t.Fatal("wrong schema must fail")
	}
}

func TestEncodeSARIFOnlyFileLocalFindings(t *testing.T) {
	r := Result{
		SchemaVersion: SchemaVersion,
		Pass:          false,
		PolicyDigest:  "abc123",
		Fails: []string{
			"selective_risk want <= 0.25 got 0.41 (statistical route failure)",
			"unsupported config file=config/verifier.json line=12 check failed",
		},
	}
	b, err := EncodeSARIF(r)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	if !strings.Contains(out, `"version":"2.1.0"`) || !strings.Contains(out, "config/verifier.json") || !strings.Contains(out, `"startLine":12`) {
		t.Fatalf("sarif missing file-local finding: %s", out)
	}
	if strings.Contains(out, "selective_risk") {
		t.Fatalf("statistical failure must not become SARIF result: %s", out)
	}
	if !HasSARIFFindings(r.Fails) {
		t.Fatal("expected file-local finding present")
	}
	if HasSARIFFindings([]string{r.Fails[0]}) {
		t.Fatal("statistical-only failures must not produce SARIF")
	}
	empty, err := EncodeSARIF(Result{SchemaVersion: SchemaVersion, Pass: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"results":[]`) {
		t.Fatalf("passing run must emit empty results: %s", empty)
	}
	if _, err := EncodeSARIF(Result{SchemaVersion: "bogus"}); err == nil {
		t.Fatal("wrong schema must fail")
	}
}

func TestEncodeMarkdownRendersExplanations(t *testing.T) {
	r := Result{
		SchemaVersion: SchemaVersion,
		Pass:          false,
		PolicyDigest:  "d1",
		Fails:         []string{"calibration want failed got measured"},
		Explanations: []Explanation{{
			Check:       "policy v1 requires calibration",
			Why:         "release gate",
			Evidence:    "dimension calibration status measured",
			Remediation: "reproduce offline",
		}},
	}
	b, err := EncodeMarkdown(r)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, want := range []string{"## Explanations and reproduction", "### policy v1 requires calibration", "- why: release gate", "- evidence: dimension calibration status measured", "- remediation: reproduce offline"} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown missing %q: %s", want, out)
		}
	}
}
