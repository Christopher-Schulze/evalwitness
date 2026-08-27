package profile

import "fmt"

// SelfFalsificationStep is one generation transition with provenance.
type SelfFalsificationStep struct {
	From                 string   `json:"from"`
	To                   string   `json:"to"`
	Reason               string   `json:"reason"`
	Denominator          int      `json:"denominator"`
	DenominatorName      string   `json:"denominator_name"`
	EvidenceComponentIDs []string `json:"evidence_component_ids"`
	GuardingTest         string   `json:"guarding_test"`
	NonClaims            []string `json:"non_claims"`
}

// AutopsyView is a cycle-free view of claim.Autopsy needed for the lineage.
type AutopsyView struct {
	MethodIntegrity MethodIntegrityView `json:"method_integrity"`
}

type AutopsyCountView struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type AutopsyEvidenceRefView struct {
	ComponentID string `json:"component_id"`
}

type MethodGenerationView struct {
	Generation         string                   `json:"generation"`
	FrozenDenominators []AutopsyCountView       `json:"frozen_denominators"`
	NonClaims          []string                 `json:"non_claims"`
	Evidence           []AutopsyEvidenceRefView `json:"evidence"`
}

type MethodTransitionView struct {
	From                 string   `json:"from"`
	To                   string   `json:"to"`
	Reason               string   `json:"reason"`
	EvidenceComponentIDs []string `json:"evidence_component_ids"`
	GuardingTest         string   `json:"guarding_test"`
}

type MethodIntegrityView struct {
	Generations []MethodGenerationView `json:"generations"`
	Transitions []MethodTransitionView `json:"transitions"`
	Current     string                 `json:"current"`
	Boundary    string                 `json:"boundary"`
}

// StepsFromAutopsyView projects the self-falsification ladder from an AutopsyView.
// It fails on missing generations, transitions, denominators, supersession reasons,
// current-status edges, or scientific non-claims and forbids hand-entered numbers.
func StepsFromAutopsyView(a AutopsyView) ([]SelfFalsificationStep, error) {
	lane := a.MethodIntegrity
	if len(lane.Generations) != 3 {
		return nil, fmt.Errorf("self-falsification: expected 3 generations, got %d", len(lane.Generations))
	}
	if len(lane.Transitions) != 2 {
		return nil, fmt.Errorf("self-falsification: expected 2 transitions, got %d", len(lane.Transitions))
	}
	if lane.Current != "v3" {
		return nil, fmt.Errorf("self-falsification: lane current must be v3, got %s", lane.Current)
	}
	if lane.Boundary == "" {
		return nil, fmt.Errorf("self-falsification: boundary empty")
	}
	genByID := make(map[string]MethodGenerationView, 3)
	for _, g := range lane.Generations {
		genByID[g.Generation] = g
	}
	v1, ok1 := genByID["v1"]
	v2, ok2 := genByID["v2"]
	v3, ok3 := genByID["v3"]
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("self-falsification: missing v1/v2/v3 generations")
	}
	for _, g := range []MethodGenerationView{v1, v2, v3} {
		if len(g.NonClaims) == 0 {
			return nil, fmt.Errorf("self-falsification: generation %s has no scientific non-claims", g.Generation)
		}
	}
	corrected := countByNameView(v1.FrozenDenominators, "corrected_rejections")
	if corrected == 0 {
		return nil, fmt.Errorf("self-falsification: v1 corrected_rejections is zero or missing")
	}
	v2FA := countByNameView(v2.FrozenDenominators, "v2_false_acceptances")
	if v2FA == 0 {
		return nil, fmt.Errorf("self-falsification: v2 v2_false_acceptances is zero or missing")
	}
	scarcityAttempted := countByNameView(v3.FrozenDenominators, "scarcity_attempted")
	if scarcityAttempted == 0 {
		return nil, fmt.Errorf("self-falsification: v3 scarcity_attempted is zero or missing")
	}
	t01 := lane.Transitions[0]
	t12 := lane.Transitions[1]
	if t01.From != "v1" || t01.To != "v2" || t12.From != "v2" || t12.To != "v3" {
		return nil, fmt.Errorf("self-falsification: transitions must be v1->v2 and v2->v3, got %s->%s and %s->%s", t01.From, t01.To, t12.From, t12.To)
	}
	if t01.Reason == "" || t12.Reason == "" {
		return nil, fmt.Errorf("self-falsification: supersession reason empty")
	}
	return []SelfFalsificationStep{
		{From: t01.From, To: t01.To, Reason: t01.Reason, Denominator: corrected, DenominatorName: "corrected_rejections", EvidenceComponentIDs: append([]string(nil), t01.EvidenceComponentIDs...), GuardingTest: t01.GuardingTest, NonClaims: append([]string(nil), v1.NonClaims...)},
		{From: t12.From, To: t12.To, Reason: t12.Reason, Denominator: v2FA, DenominatorName: "v2_false_acceptances", EvidenceComponentIDs: append([]string(nil), t12.EvidenceComponentIDs...), GuardingTest: t12.GuardingTest, NonClaims: append([]string(nil), v2.NonClaims...)},
		{From: "v3", To: "frozen", Reason: lane.Boundary, Denominator: scarcityAttempted, DenominatorName: "scarcity_attempted", EvidenceComponentIDs: evidenceIDsView(v3.Evidence), GuardingTest: "", NonClaims: append([]string(nil), v3.NonClaims...)},
	}, nil
}

func countByNameView(counts []AutopsyCountView, name string) int {
	for _, c := range counts {
		if c.Name == name {
			return c.Value
		}
	}
	return 0
}

func evidenceIDsView(refs []AutopsyEvidenceRefView) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.ComponentID)
	}
	return out
}
