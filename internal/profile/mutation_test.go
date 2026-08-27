package profile

import (
	"testing"
)

func measuredDim(id, scope, expr string) Dimension {
	metric := "1"
	return Dimension{ID: id, Status: StatusMeasured, Metric: &metric, Scope: scope, EvidenceLevel: "E1", CapsuleExpr: expr, Denominator: 10, SampleUnit: "task"}
}

func TestProfileMutationScopeWideningDetected(t *testing.T) {
	narrow, _ := Build("a", "v1", "r1", []Dimension{measuredDim("x", "terminal", "c")})
	wide, _ := Build("a", "v1", "r1", []Dimension{measuredDim("x", "terminal,web", "c")})
	d := Diff(narrow, wide)
	if !d.Compatible {
		t.Fatalf("same protocol/route must stay compatible: %+v", d)
	}
	if len(d.Changed) != 1 {
		t.Fatalf("scope widening must surface as changed, got %+v", d)
	}
	pol, _ := NewPolicy("v1", map[string]string{"x": "measured"})
	okNarrow, failsNarrow := Evaluate(narrow, pol)
	okWide, failsWide := Evaluate(wide, pol)
	if !okNarrow || !okWide || len(failsNarrow) != 0 || len(failsWide) != 0 {
		t.Fatalf("policy is status-only; diff must carry the scope signal: %v %v %v %v", okNarrow, failsNarrow, okWide, failsWide)
	}
	if d.Changed[0] == "" {
		t.Fatal("changed entry empty")
	}
}

func TestProfileMutationDenominatorDetected(t *testing.T) {
	base := measuredDim("x", "s", "c")
	other := base
	other.Denominator = 20
	a, _ := Build("a", "v1", "r1", []Dimension{base})
	b, _ := Build("b", "v1", "r1", []Dimension{other})
	d := Diff(a, b)
	if len(d.Changed) != 1 {
		t.Fatalf("denominator mutation must surface as changed, got %+v", d)
	}
}

func TestProfileMutationStaleCapsuleFailsVerify(t *testing.T) {
	a, _ := Build("a", "v1", "r1", []Dimension{measuredDim("x", "s", "capsule:v1")})
	if err := Verify(a); err != nil {
		t.Fatalf("base verify %v", err)
	}
	stale := a
	stale.CapsuleParents = []string{"sha256-0000000000000000000000000000000000000000000000000000000000000000"}
	// Old copied digest no longer matches mutated content: verify must fail.
	if err := Verify(stale); err == nil {
		t.Fatal("stale capsule parent must fail digest verify")
	}
	// Capsule parents are part of identity: recomputed digest differs.
	d, err := stale.DigestValue()
	if err != nil {
		t.Fatalf("digest %v", err)
	}
	if d == a.Digest {
		t.Fatal("digests must differ")
	}
	stale.Digest = d
	if err := Verify(stale); err != nil {
		t.Fatalf("rebound stale profile must verify against its own digest: %v", err)
	}
}

func TestProfileMutationIncompatibleDiffRefused(t *testing.T) {
	a, _ := Build("a", "v1", "r1", []Dimension{measuredDim("x", "s", "c")})
	b, _ := Build("b", "v2", "r1", []Dimension{measuredDim("x", "s", "c")})
	if d := Diff(a, b); d.Compatible {
		t.Fatal("protocol change must refuse diff")
	}
	c, _ := Build("c", "v1", "other-route", []Dimension{measuredDim("x", "s", "c")})
	if d := Diff(a, c); d.Compatible {
		t.Fatal("route change must refuse diff")
	}
}

func TestProfileMutationPolicyGamingFails(t *testing.T) {
	p, _ := Build("id", "v1", "r", []Dimension{measuredDim("x", "s", "c")})
	// Gaming attempt 1: permit an unknown dimension without naming it -> still fails.
	polUnknown, _ := NewPolicy("v1", map[string]string{"x": "measured", "ghost": "not_measured"})
	ok, fails := Evaluate(p, polUnknown)
	if ok || len(fails) != 1 {
		t.Fatalf("unnamed unknown must fail, got ok=%v fails=%v", ok, fails)
	}
	// Gaming attempt 2: policy digest mismatch must be detectable.
	polReal, _ := NewPolicy("v1", map[string]string{"x": "measured"})
	forged := polReal
	forged.Digest = "deadbeef"
	if forged.Digest == polReal.Digest {
		t.Fatal("forged digest must differ")
	}
	// Gaming attempt 3: flipping a failed dimension to measured in-place breaks Verify.
	tampered := p
	tampered.Dimensions[0].Status = StatusFailed
	if err := Verify(tampered); err == nil {
		t.Fatal("in-place status flip must fail digest verify")
	}
}
