package profile

import (
	"os"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/calibration"
	"github.com/Christopher-Schulze/evalwitness/internal/reliability"
)

func TestProfileValidate(t *testing.T) {
	metric := "0.12"
	p := Profile{
		SchemaVersion:   SchemaVersion,
		Identity:        "test-profile",
		ProtocolVersion: "evalwitness.protocol.v1",
		RouteScope:      "opencode-go-cn:deepseek-v4-flash",
		Dimensions: []Dimension{
			{ID: "calibration", Status: StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "calibration.metrics.ece", Denominator: 100, SampleUnit: "task"},
			{ID: "transfer", Status: StatusNotMeasured, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "unknown", Denominator: 0, SampleUnit: "task"},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid: %v", err)
	}
	d1, _ := p.DigestValue()
	d2, _ := p.DigestValue()
	if d1 != d2 {
		t.Fatalf("not deterministic %s vs %s", d1, d2)
	}
	bad := p
	bad.Dimensions[0].Metric = nil
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error on missing metric")
	}
	bad2 := p
	bad2.Dimensions[0].Status = "unknown"
	if err := bad2.Validate(); err == nil {
		t.Fatal("expected error on unknown status")
	}
	if p.Identity == "" {
		t.Fatal("identity empty")
	}
}

func TestProfileNoGlobalScore(t *testing.T) {
	p := Profile{SchemaVersion: SchemaVersion, Identity: "x", ProtocolVersion: "v1", RouteScope: "r", Dimensions: []Dimension{{ID: "a", Status: StatusMeasured, Metric: new("1"), Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}}}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate %v", err)
	}
}

func TestProfileBuildVerify(t *testing.T) {
	metric := "0.5"
	p, err := Build("id1", "evalwitness.protocol.v1", "routeA", []Dimension{{ID: "calibration", Status: StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 10, SampleUnit: "task"}})
	if err != nil {
		t.Fatalf("build %v", err)
	}
	if p.Digest == "" {
		t.Fatal("digest empty")
	}
	if err := Verify(p); err != nil {
		t.Fatalf("verify %v", err)
	}
	p.Digest = "bad"
	if err := Verify(p); err == nil {
		t.Fatal("expected digest mismatch")
	}
}

func TestProfileDiff(t *testing.T) {
	metric := "1"
	a, _ := Build("a", "v1", "r1", []Dimension{{ID: "x", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	b, _ := Build("b", "v2", "r1", []Dimension{{ID: "x", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	d := Diff(a, b)
	if d.Compatible {
		t.Fatal("expected incompatible protocol")
	}
	c, _ := Build("c", "v1", "r1", []Dimension{{ID: "x", Status: StatusFailed, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	d2 := Diff(a, c)
	if !d2.Compatible || len(d2.Changed) == 0 {
		t.Fatalf("expected change %+v", d2)
	}
}

func TestProfilePolicy(t *testing.T) {
	metric := "1"
	p, _ := Build("id", "v1", "r", []Dimension{{ID: "calibration", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	pol, _ := NewPolicy("v1", map[string]string{"calibration": "measured"})
	ok, fails := Evaluate(p, pol)
	if !ok || len(fails) != 0 {
		t.Fatalf("expected pass %v", fails)
	}
	pol2, _ := NewPolicy("v1", map[string]string{"calibration": "failed"})
	ok2, fails2 := Evaluate(p, pol2)
	if ok2 || len(fails2) == 0 {
		t.Fatal("expected fail")
	}
}

func TestCalibrationWire(t *testing.T) {
	metric := "0.1"
	p, _ := Build("id", "v1", "r", []Dimension{{ID: "x", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	_ = p
	schema := calibration.FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}}
	lc, _ := calibration.NewLifecycle(schema, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	obs := []calibration.Observation{{ID: "1", TaskID: "t1", SplitRole: calibration.RoleTest, Predicted: 0.9, Won: new(true)}, {ID: "2", TaskID: "t2", SplitRole: calibration.RoleTest, Predicted: 0.2, Won: new(false)}}
	est := func(o calibration.Observation) float64 { return o.Predicted }
	rep, err := calibration.BuildReport("platt", schema, lc, obs, 0.5, est, 42)
	if err != nil {
		t.Fatalf("build report %v", err)
	}
	rel := reliability.Analyze([]reliability.Observation{{TaskID: "t1", Predicted: 0.9, Won: true}, {TaskID: "t2", Predicted: 0.2, Won: false}})
	dims, err := ToDimensions(CalibrationInput{Report: rep, Reliability: rel})
	if err != nil || len(dims) != 2 {
		t.Fatalf("wire %v %d", err, len(dims))
	}
}

func TestProfileVerifyEmptyDigest(t *testing.T) {
	metric := "1"
	p := Profile{SchemaVersion: SchemaVersion, Identity: "x", ProtocolVersion: "v1", RouteScope: "r", Dimensions: []Dimension{{ID: "a", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}}}
	if err := Verify(p); err == nil {
		t.Fatal("expected empty digest error")
	}
	p2, _ := Build("id", "v1", "r", []Dimension{{ID: "a", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	if err := Verify(p2); err != nil {
		t.Fatalf("verify built %v", err)
	}
}

func TestProfileDiffDeterministic(t *testing.T) {
	metric := "1"
	a, _ := Build("a", "v1", "r1", []Dimension{{ID: "b", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}, {ID: "a", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	b, _ := Build("b", "v1", "r1", []Dimension{{ID: "c", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	d1 := Diff(a, b)
	d2 := Diff(a, b)
	if len(d1.Added) != len(d2.Added) || len(d1.Removed) != len(d2.Removed) {
		t.Fatalf("not deterministic %+v vs %+v", d1, d2)
	}
	for i := 1; i < len(d1.Added); i++ {
		if d1.Added[i-1] > d1.Added[i] {
			t.Fatalf("Added not sorted %v", d1.Added)
		}
	}
}

func TestProfileMutationDeletionFails(t *testing.T) {
	metric := "1"
	p, _ := Build("id", "v1", "r", []Dimension{{ID: "calibration", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}, {ID: "transfer", Status: StatusNotMeasured, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 0, SampleUnit: "task"}})
	p2, _ := Build("id", "v1", "r", []Dimension{{ID: "calibration", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	d := Diff(p, p2)
	if len(d.Removed) == 0 {
		t.Fatalf("expected removed %+v", d)
	}
	pol, _ := NewPolicy("v1", map[string]string{"transfer": "not_measured"})
	ok, _ := Evaluate(p2, pol)
	if ok {
		t.Fatal("expected policy fail on missing dimension")
	}
}
func TestHelpersClaimCheckAndOTelAndReport(t *testing.T) {
	metric := "1"
	p, _ := Build("id", "v1", "r", []Dimension{{ID: "a", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	exprs := ClaimCheckExpr(p)
	if len(exprs) != 1 || exprs[0] == "" {
		t.Fatalf("claimcheck empty %v", exprs)
	}
	st := ClaimCheckStatuses(p)
	if st["a"] != StatusMeasured {
		t.Fatalf("status %v", st)
	}
	ev, err := ToOTel(p)
	if err != nil || ev.ProfileDigest != p.Digest || ev.TraceID != p.Identity {
		t.Fatalf("otel %v %v", ev, err)
	}
	pBad := p
	pBad.Digest = ""
	if _, err := ToOTel(pBad); err == nil {
		t.Fatal("expected otel fail on empty digest")
	}
	txt := TextReport(p)
	if txt == "" || MarkdownReport(p) == "" {
		t.Fatal("report empty")
	}
	rep, err := BuildEvidenceReport(p)
	if err != nil || rep.Text == "" || rep.Markdown == "" || len(rep.ClaimCheckExprs) != 1 || rep.OTel.ProfileDigest != p.Digest || len(rep.DimensionsJSON) == 0 {
		t.Fatalf("evidence report %v %+v", err, rep)
	}
}

func TestDocsAndSpecFlushed(t *testing.T) {
	doc, err := os.ReadFile("../../docs/documentation.md")
	if err != nil {
		t.Skip("no doc")
	}
	if !IsDocsFlushed(string(doc)) {
		t.Fatalf("docs not flushed: missing Reliability profile / internal/profile marker")
	}
	spec, err := os.ReadFile("../../docs/spec.md")
	if err != nil {
		t.Skip("no spec")
	}
	if !IsSpecFlushed(string(spec)) {
		t.Fatalf("spec not flushed: missing VerifierReliabilityProfile / evalwitness.profile.v1.schema")
	}
}

func TestStepsFromAutopsy(t *testing.T) {
	lane := MethodIntegrityView{
		Generations: []MethodGenerationView{
			{Generation: "v1", FrozenDenominators: []AutopsyCountView{{Name: "corrected_rejections", Value: 3}, {Name: "fixtures", Value: 3}}, NonClaims: []string{"no held-out validity for v1"}},
			{Generation: "v2", FrozenDenominators: []AutopsyCountView{{Name: "v2_false_acceptances", Value: 5}}, NonClaims: []string{"no population inference from v2"}},
			{Generation: "v3", FrozenDenominators: []AutopsyCountView{{Name: "scarcity_attempted", Value: 198}, {Name: "scarcity_admitted", Value: 3}}, Evidence: []AutopsyEvidenceRefView{{ComponentID: "scarcity"}}, NonClaims: []string{"zero test-role scarcity cases forbid held-out validity claims"}},
		},
		Transitions: []MethodTransitionView{
			{From: "v1", To: "v2", Reason: "three frozen v1 acceptances are rejected by v2 under closed reasons", EvidenceComponentIDs: []string{"repair"}, GuardingTest: "TestConstructRepairCasesReproduceLegacyAcceptanceAndCorrectedRejection"},
			{From: "v2", To: "v3", Reason: "five frozen v2 false acceptances are rejected by v3 while six positive controls and three shared guards are preserved", EvidenceComponentIDs: []string{"challenge"}, GuardingTest: "TestConstructChallengeReproducesV2FalsificationAndV3Repair"},
		},
		Current: "v3", Boundary: "v3 is admitted only as provider-free development evidence; the 280-case inferential core and 3-of-40 scarcity result remain separate and zero test-role scarcity cases forbid held-out validity claims",
	}
	a := AutopsyView{MethodIntegrity: lane}
	steps, err := StepsFromAutopsyView(a)
	if err != nil {
		t.Fatalf("steps %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("steps len %d", len(steps))
	}
	if steps[0].Denominator != 3 || steps[1].Denominator != 5 || steps[2].Denominator != 198 {
		t.Fatalf("denominators %v", steps)
	}
	if steps[0].DenominatorName != "corrected_rejections" || steps[1].DenominatorName != "v2_false_acceptances" || steps[2].DenominatorName != "scarcity_attempted" {
		t.Fatalf("names %v", steps)
	}
	if len(steps[0].NonClaims) == 0 || len(steps[2].NonClaims) == 0 {
		t.Fatalf("non-claims missing %v", steps)
	}
	failCases := []func(a AutopsyView) AutopsyView{
		func(a AutopsyView) AutopsyView {
			a.MethodIntegrity.Generations = a.MethodIntegrity.Generations[:2]
			return a
		},
		func(a AutopsyView) AutopsyView {
			a.MethodIntegrity.Transitions = a.MethodIntegrity.Transitions[:1]
			return a
		},
		func(a AutopsyView) AutopsyView { a.MethodIntegrity.Current = "v2"; return a },
		func(a AutopsyView) AutopsyView { a.MethodIntegrity.Boundary = ""; return a },
		func(a AutopsyView) AutopsyView { a.MethodIntegrity.Generations[0].NonClaims = nil; return a },
		func(a AutopsyView) AutopsyView { a.MethodIntegrity.Generations[0].FrozenDenominators = nil; return a },
		func(a AutopsyView) AutopsyView { a.MethodIntegrity.Transitions[0].Reason = ""; return a },
		func(a AutopsyView) AutopsyView { a.MethodIntegrity.Transitions[0].To = "v3"; return a },
	}
	for i, mutate := range failCases {
		if _, err := StepsFromAutopsyView(mutate(a)); err == nil {
			t.Fatalf("expected fail on mutation case %d", i)
		}
	}
}
