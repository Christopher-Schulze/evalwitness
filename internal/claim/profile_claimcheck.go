package claim

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// CheckProfile feeds profile expressions and statuses into TASK 050 claimcheck.
// It validates the profile and returns its claimcheck expressions; every
// dimension's status must be explicitly declared and its capsule expression
// must be non-empty. Failures are visible rather than silent passes.
func CheckProfile(p profile.Profile) ([]string, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("profile claimcheck: validate: %w", err)
	}
	exprs := profile.ClaimCheckExpr(p)
	statuses := profile.ClaimCheckStatuses(p)
	if len(exprs) != len(statuses) {
		return nil, fmt.Errorf("profile claimcheck: expr/status count mismatch %d vs %d", len(exprs), len(statuses))
	}
	for id, st := range statuses {
		if st == "" {
			return nil, fmt.Errorf("profile claimcheck: dimension %s has empty status", id)
		}
	}
	for i, e := range exprs {
		if e == "" {
			return nil, fmt.Errorf("profile claimcheck: dimension %d has empty capsule expression", i)
		}
	}
	return exprs, nil
}

// ProfileStepsView builds a profile AutopsyView from a claim.Autopsy for
// lineage consumption without creating an import cycle in the opposite direction.
// Callers in claim can project profile lineage via profile.StepsFromAutopsyView.
func ProfileStepsView(a Autopsy) (profile.AutopsyView, error) {
	view := profile.AutopsyView{
		MethodIntegrity: profile.MethodIntegrityView{
			Current:  a.MethodIntegrity.Current,
			Boundary: a.MethodIntegrity.Boundary,
		},
	}
	for _, g := range a.MethodIntegrity.Generations {
		evs := make([]profile.AutopsyEvidenceRefView, 0, len(g.Evidence))
		for _, e := range g.Evidence {
			evs = append(evs, profile.AutopsyEvidenceRefView{ComponentID: e.ComponentID})
		}
		counts := make([]profile.AutopsyCountView, 0, len(g.FrozenDenominators))
		for _, c := range g.FrozenDenominators {
			counts = append(counts, profile.AutopsyCountView{Name: c.Name, Value: c.Value})
		}
		view.MethodIntegrity.Generations = append(view.MethodIntegrity.Generations, profile.MethodGenerationView{
			Generation:         g.Generation,
			FrozenDenominators: counts,
			NonClaims:          append([]string(nil), g.NonClaims...),
			Evidence:           evs,
		})
	}
	for _, t := range a.MethodIntegrity.Transitions {
		view.MethodIntegrity.Transitions = append(view.MethodIntegrity.Transitions, profile.MethodTransitionView{
			From:                 t.From,
			To:                   t.To,
			Reason:               t.Reason,
			EvidenceComponentIDs: append([]string(nil), t.EvidenceComponentIDs...),
			GuardingTest:         t.GuardingTest,
		})
	}
	return view, nil
}
