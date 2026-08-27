package audit

import (
	"testing"

	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

func claimFor(id string, level claimledger.EvidenceLevel, gen string, caveats int) claimledger.Claim {
	caveatList := make([]string, caveats)
	for i := range caveatList {
		caveatList[i] = "caveat"
	}
	return claimledger.Claim{
		ClaimID: id, Status: claimledger.StatusSupported, EvidenceLevel: level,
		Caveats:    caveatList,
		Generation: claimledger.Generation{Lane: "development", Evidence: "capsule", Current: gen},
	}
}

func TestClaimDeltasClassifyTransitions(t *testing.T) {
	base := claimledger.Ledger{Claims: []claimledger.Claim{
		claimFor("CLM-001", "E1", "v3", 1),
		claimFor("CLM-002", "E2", "v3", 1),
		claimFor("CLM-003", "E2", "v3", 2),
	}}
	cur := claimledger.Ledger{Claims: []claimledger.Claim{
		claimFor("CLM-001", "E2", "v3", 1), // strengthened
		claimFor("CLM-002", "E1", "v3", 1), // weakened
		claimFor("CLM-003", "E2", "v3", 3), // narrowed (caveats grew)
		claimFor("CLM-004", "E1", "v3", 0), // added
	}}
	deltas := ClaimDeltas(base, cur)
	byID := make(map[string]ClaimDelta, len(deltas))
	for _, d := range deltas {
		byID[d.ClaimID] = d
	}
	if byID["CLM-001"].Transition != TransitionStrengthened {
		t.Fatalf("CLM-001 %+v", byID["CLM-001"])
	}
	if byID["CLM-002"].Transition != TransitionWeakened {
		t.Fatalf("CLM-002 %+v", byID["CLM-002"])
	}
	if byID["CLM-003"].Transition != TransitionNarrowed {
		t.Fatalf("CLM-003 %+v", byID["CLM-003"])
	}
	if byID["CLM-004"].Transition != TransitionAdded {
		t.Fatalf("CLM-004 %+v", byID["CLM-004"])
	}
	withdrawn := claimledger.Ledger{Claims: []claimledger.Claim{
		claimFor("CLM-001", "E1", "v3", 1),
		claimFor("CLM-002", "E2", "v3", 1),
	}}
	deltas = ClaimDeltas(base, withdrawn)
	found := false
	for _, d := range deltas {
		if d.Transition == TransitionWithdrawn && d.ClaimID == "CLM-003" {
			found = true
		}
	}
	if !found {
		t.Fatalf("withdrawn missing %v", deltas)
	}
	// Incomparable on generation divergence
	diverged := claimledger.Ledger{Claims: []claimledger.Claim{
		func() claimledger.Claim { c := claimFor("CLM-001", "E1", "v4", 1); return c }(),
	}}
	deltas = ClaimDeltas(base, diverged)
	if deltas[0].Transition != TransitionIncomparable {
		t.Fatalf("diverged %+v", deltas[0])
	}
}

func TestProfileRegressionsDetectDropsAndScopeChanges(t *testing.T) {
	metric := "0.12"
	build := func(scope, level string, dims ...string) profile.Profile {
		var ds []profile.Dimension
		for _, id := range dims {
			m := metric
			ds = append(ds, profile.Dimension{ID: id, Status: profile.StatusMeasured, Metric: &m, Scope: scope, EvidenceLevel: level, CapsuleExpr: "c", Denominator: 10, SampleUnit: "task"})
		}
		p, err := profile.Build("regression", "v1", "routeA", ds)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	base := build("terminal", "E1", "calibration", "transfer")
	same := build("terminal", "E1", "calibration", "transfer")
	if fails := ProfileRegressions(base, same); len(fails) != 0 {
		t.Fatalf("identical profile must pass: %v", fails)
	}
	dropped := build("terminal", "E1", "calibration")
	if fails := ProfileRegressions(base, dropped); len(fails) == 0 || !contains([]string{fails[0]}, "dropped dimension transfer") {
		t.Fatalf("drop must fail: %v", fails)
	}
	scoped := build("terminal,web", "E1", "calibration", "transfer")
	if fails := ProfileRegressions(base, scoped); len(fails) == 0 || !contains([]string{fails[0]}, "scope changed") {
		t.Fatalf("scope change must fail: %v", fails)
	}
	routed := profile.Profile{}
	routed.ProtocolVersion = "v9"
	if fails := ProfileRegressions(base, routed); len(fails) == 0 || !contains([]string{fails[0]}, "protocol changed") {
		t.Fatalf("protocol change must fail: %v", fails)
	}
}

func contains(list []string, sub string) bool {
	for _, s := range list {
		if len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
