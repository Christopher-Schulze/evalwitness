package registry

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

func TestRenderMethodLineagePinsTwoBreakLadder(t *testing.T) {
	record, err := RenderMethodLineage(methodLineageFixture())
	if err != nil {
		t.Fatal(err)
	}
	if record.Current != "v3" || len(record.Stages) != 3 || record.Rankable || record.Pooled {
		t.Fatalf("record = %+v", record)
	}
	if record.Stages[0].StageID != "v1-v2" || record.Stages[1].StageID != "v2-v3" || record.Stages[2].StageID != "v3-frozen" {
		t.Fatalf("stages = %+v", record.Stages)
	}
}

func TestRenderMethodLineageRejectsBrokenLadder(t *testing.T) {
	view := methodLineageFixture()
	view.MethodIntegrity.Current = "v2"
	if _, err := RenderMethodLineage(view); err == nil {
		t.Fatal("broken current accepted")
	}
}

func methodLineageFixture() profile.AutopsyView {
	return profile.AutopsyView{MethodIntegrity: profile.MethodIntegrityView{
		Generations: []profile.MethodGenerationView{
			{Generation: "v1", FrozenDenominators: []profile.AutopsyCountView{{Name: "corrected_rejections", Value: 3}}, NonClaims: []string{"no held-out validity for v1"}},
			{Generation: "v2", FrozenDenominators: []profile.AutopsyCountView{{Name: "v2_false_acceptances", Value: 5}}, NonClaims: []string{"no population inference from v2"}},
			{Generation: "v3", FrozenDenominators: []profile.AutopsyCountView{{Name: "scarcity_attempted", Value: 198}}, Evidence: []profile.AutopsyEvidenceRefView{{ComponentID: "scarcity"}}, NonClaims: []string{"zero test-role scarcity cases forbid held-out validity claims"}},
		},
		Transitions: []profile.MethodTransitionView{
			{From: "v1", To: "v2", Reason: "three frozen v1 acceptances are rejected by v2 under closed reasons"},
			{From: "v2", To: "v3", Reason: "five frozen v2 false acceptances are rejected by v3 while six positive controls and three shared guards are preserved"},
		},
		Current:  "v3",
		Boundary: "v3 is admitted only as provider-free development evidence; the 280-case inferential core and 3-of-40 scarcity result remain separate and zero test-role scarcity cases forbid held-out validity claims",
	}}
}
