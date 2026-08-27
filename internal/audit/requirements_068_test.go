package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

const scarcityEvidence = "../../eval/results/relation-scarcity-negative-evidence.json"

func TestCheckNegativeEvidencePinsFrozenFunnel(t *testing.T) {
	req := NegativeEvidenceRequirement{AttemptedTarget: 198, AppliedTarget: 3, RejectedTarget: 195}
	if fails := CheckNegativeEvidence(scarcityEvidence, req); len(fails) != 0 {
		t.Fatalf("frozen funnel must pass: %v", fails)
	}
	drifted := NegativeEvidenceRequirement{AttemptedTarget: 199, AppliedTarget: 3, RejectedTarget: 195}
	fails := CheckNegativeEvidence(scarcityEvidence, drifted)
	if len(fails) != 1 || !strings.Contains(fails[0], "attempted 198 != frozen target 199") {
		t.Fatalf("drift must fail: %v", fails)
	}
}

func TestCheckNegativeEvidenceRejectsTestRolePromotion(t *testing.T) {
	raw, err := os.ReadFile(scarcityEvidence)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	roles := doc["study_roles"].(map[string]any)
	roles["test"] = float64(5)
	body, _ := json.Marshal(doc)
	tampered := filepath.Join(t.TempDir(), "tampered.json")
	_ = os.WriteFile(tampered, body, 0o644)
	// Digest enforcement inside the loader rejects tampered files outright:
	// promotion cannot even reach the role check.
	req := NegativeEvidenceRequirement{AttemptedTarget: 198, AppliedTarget: 3, RejectedTarget: 195}
	fails := CheckNegativeEvidence(tampered, req)
	if len(fails) == 0 {
		t.Fatal("tampered panel must fail")
	}
}

// TestCheckNegativeEvidenceDirectTestRolePromotion proves the promotion rule
// is enforced at the relation decode layer: a study_roles.test=5 file cannot
// seal or decode at all (role-boundary error), so a sealed nonzero-test file
// is unconstructible and the audit-side TestZero loop is redundant defense.
func TestCheckNegativeEvidenceDirectTestRolePromotion(t *testing.T) {
	raw, err := os.ReadFile(scarcityEvidence)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	roles := doc["study_roles"].(map[string]any)
	roles["test"] = float64(5)
	body, _ := json.Marshal(doc)
	promoted := filepath.Join(t.TempDir(), "promoted.json")
	_ = os.WriteFile(promoted, body, 0o644)
	// The loader (relation.DecodeScarcityPublicEvidence) must reject the
	// promotion at the study-role boundary before any audit role check runs.
	req := NegativeEvidenceRequirement{AttemptedTarget: 198, AppliedTarget: 3, RejectedTarget: 195}
	fails := CheckNegativeEvidence(promoted, req)
	if len(fails) != 1 || !strings.Contains(fails[0], "study-role boundary is invalid") {
		t.Fatalf("promotion must fail at the relation decode layer: %v", fails)
	}
}

func TestCheckOwnerInspectionRequirementStates(t *testing.T) {
	write := func(status string) string {
		path := filepath.Join(t.TempDir(), "attestation.json")
		doc := map[string]any{"schema_version": "evalwitness.relation-owner-inspection-public-attestation.v1", "status": status}
		body, _ := json.Marshal(doc)
		_ = os.WriteFile(path, body, 0o644)
		return path
	}
	status, fails := CheckOwnerInspectionRequirement(OwnerInspectionRequirement{AttestationPath: write("passed")})
	if status != "passed" || len(fails) != 0 {
		t.Fatalf("passed without requirement: %s %v", status, fails)
	}
	status, fails = CheckOwnerInspectionRequirement(OwnerInspectionRequirement{AttestationPath: write("revision_required")})
	if status != "revision_required" || len(fails) != 1 || !strings.Contains(fails[0], "revision required") {
		t.Fatalf("revision_required must fail: %s %v", status, fails)
	}
	_, fails = CheckOwnerInspectionRequirement(OwnerInspectionRequirement{AttestationPath: write("unresolved")})
	if len(fails) != 1 || !strings.Contains(fails[0], "unresolved") {
		t.Fatalf("unresolved must fail: %v", fails)
	}
	_, fails = CheckOwnerInspectionRequirement(OwnerInspectionRequirement{AttestationPath: write("absent"), RequirePassed: true})
	if len(fails) != 1 || !strings.Contains(fails[0], "policy requires passed readiness") {
		t.Fatalf("require-passed with absent must fail: %v", fails)
	}
}

func TestCheckMethodLineageLadderShape(t *testing.T) {
	steps := []MethodLineageStep{
		{From: "v1", To: "v2", Reason: "three frozen v1 acceptances are rejected by v2 under closed reasons", Denominator: 3, NonClaims: []string{"no held-out validity"}},
		{From: "v2", To: "v3", Reason: "five frozen v2 false acceptances are rejected by v3", Denominator: 5, NonClaims: []string{"no population inference"}},
		{From: "v3", To: "frozen", Reason: "scarcity frozen as current state", Denominator: 198, NonClaims: []string{"zero test-role scarcity cases forbid held-out validity claims"}},
	}
	if fails := CheckMethodLineage(steps, MethodLineageRequirement{Current: "v3"}); len(fails) != 0 {
		t.Fatalf("valid ladder must pass: %v", fails)
	}
	broken := []MethodLineageStep{
		{From: "v2", To: "v3", Reason: "", Denominator: 0},
	}
	fails := CheckMethodLineage(broken, MethodLineageRequirement{Current: "v3"})
	if len(fails) != 1 {
		t.Fatalf("broken ladder must fail on step count: %v", fails)
	}
}

// Integration proof: profile.StepsFromAutopsyView output satisfies
// CheckMethodLineage (the real producer/consumer pair).
func TestCheckMethodLineageAcceptsProfileStepsOutput(t *testing.T) {
	viewSteps, err := profile.StepsFromAutopsyView(autopsyViewFixture())
	if err != nil {
		t.Fatalf("view steps %v", err)
	}
	steps := make([]MethodLineageStep, len(viewSteps))
	for i, s := range viewSteps {
		steps[i] = MethodLineageStep{From: s.From, To: s.To, Reason: s.Reason, Denominator: s.Denominator, NonClaims: s.NonClaims}
	}
	if fails := CheckMethodLineage(steps, MethodLineageRequirement{Current: "v3"}); len(fails) != 0 {
		t.Fatalf("real lineage must pass: %v", fails)
	}
}

func autopsyViewFixture() profile.AutopsyView {
	lane := profile.MethodIntegrityView{
		Generations: []profile.MethodGenerationView{
			{
				Generation:         "v1",
				FrozenDenominators: []profile.AutopsyCountView{{Name: "corrected_rejections", Value: 3}},
				NonClaims:          []string{"no held-out validity for v1"},
			},
			{
				Generation:         "v2",
				FrozenDenominators: []profile.AutopsyCountView{{Name: "v2_false_acceptances", Value: 5}},
				NonClaims:          []string{"no population inference from v2"},
			},
			{
				Generation:         "v3",
				FrozenDenominators: []profile.AutopsyCountView{{Name: "scarcity_attempted", Value: 198}},
				Evidence:           []profile.AutopsyEvidenceRefView{{ComponentID: "scarcity"}},
				NonClaims:          []string{"zero test-role scarcity cases forbid held-out validity claims"},
			},
		},
		Transitions: []profile.MethodTransitionView{
			{From: "v1", To: "v2", Reason: "three frozen v1 acceptances are rejected by v2 under closed reasons"},
			{From: "v2", To: "v3", Reason: "five frozen v2 false acceptances are rejected by v3 while six positive controls and three shared guards are preserved"},
		},
		Current:  "v3",
		Boundary: "v3 is admitted only as provider-free development evidence; the 280-case inferential core and 3-of-40 scarcity result remain separate and zero test-role scarcity cases forbid held-out validity claims",
	}
	return profile.AutopsyView{MethodIntegrity: lane}
}
