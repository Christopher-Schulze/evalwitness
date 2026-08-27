package audit

import (
	"fmt"

	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// ClaimTransition classifies one scope-aware claim delta per the TASK 059
// claim-delta boundary: typed warrant, scope, evidence level, caveats, status,
// and parents — never a text diff, never a global pass percentage.
type ClaimTransition string

const (
	TransitionAdded        ClaimTransition = "added"
	TransitionWithdrawn    ClaimTransition = "withdrawn"
	TransitionStrengthened ClaimTransition = "strengthened"
	TransitionWeakened     ClaimTransition = "weakened"
	TransitionNarrowed     ClaimTransition = "narrowed"
	TransitionWidened      ClaimTransition = "widened"
	TransitionUnchanged    ClaimTransition = "unchanged"
	TransitionIncomparable ClaimTransition = "incomparable"
)

// ClaimDelta is one classified transition with its evidence-parent difference.
type ClaimDelta struct {
	ClaimID        string          `json:"claim_id"`
	Transition     ClaimTransition `json:"transition"`
	BaselineDigest string          `json:"baseline_digest"`
	CurrentDigest  string          `json:"current_digest"`
	Reason         string          `json:"reason"`
}

// EvidenceLevelRank orders evidence levels for strengthen/weaken classification.
var evidenceLevelRank = map[claimledger.EvidenceLevel]int{
	"E1": 1,
	"E2": 2,
	"E3": 3,
	"E4": 4,
}

// ClaimDeltas compares baseline and current ledgers claim-by-claim. Identities
// that exist on one side only are added/withdrawn; matching IDs are classified
// by evidence level, scope, caveats, and status. Incomparable covers claims
// whose estimand identity (generation or expression) diverged.
func ClaimDeltas(baseline, current claimledger.Ledger) []ClaimDelta {
	base := make(map[string]claimledger.Claim, len(baseline.Claims))
	for _, c := range baseline.Claims {
		base[c.ClaimID] = c
	}
	var deltas []ClaimDelta
	seen := make(map[string]struct{}, len(current.Claims))
	for _, cur := range current.Claims {
		seen[cur.ClaimID] = struct{}{}
		old, ok := base[cur.ClaimID]
		if !ok {
			deltas = append(deltas, ClaimDelta{ClaimID: cur.ClaimID, Transition: TransitionAdded, CurrentDigest: generationIdentity(cur), Reason: "claim is new in the current ledger"})
			continue
		}
		deltas = append(deltas, classifyDelta(old, cur))
	}
	for _, old := range baseline.Claims {
		if _, ok := seen[old.ClaimID]; !ok {
			deltas = append(deltas, ClaimDelta{ClaimID: old.ClaimID, Transition: TransitionWithdrawn, BaselineDigest: generationIdentity(old), Reason: "claim absent from the current ledger"})
		}
	}
	return deltas
}

func classifyDelta(old, cur claimledger.Claim) ClaimDelta {
	if old.Generation != cur.Generation || expressionIdentity(old.Expression) != expressionIdentity(cur.Expression) {
		return ClaimDelta{
			ClaimID: cur.ClaimID, Transition: TransitionIncomparable,
			BaselineDigest: generationIdentity(old), CurrentDigest: generationIdentity(cur),
			Reason: "estimand identity diverged (generation or expression)",
		}
	}
	reason := ""
	transition := TransitionUnchanged
	switch {
	case evidenceLevelRank[cur.EvidenceLevel] > evidenceLevelRank[old.EvidenceLevel]:
		transition, reason = TransitionStrengthened, fmt.Sprintf("evidence level %s -> %s", old.EvidenceLevel, cur.EvidenceLevel)
	case evidenceLevelRank[cur.EvidenceLevel] < evidenceLevelRank[old.EvidenceLevel]:
		transition, reason = TransitionWeakened, fmt.Sprintf("evidence level %s -> %s", old.EvidenceLevel, cur.EvidenceLevel)
	}
	if len(cur.Caveats) > len(old.Caveats) {
		if transition == TransitionUnchanged {
			transition, reason = TransitionNarrowed, fmt.Sprintf("caveats grew %d -> %d", len(old.Caveats), len(cur.Caveats))
		}
	} else if len(cur.Caveats) < len(old.Caveats) {
		if transition == TransitionUnchanged {
			transition, reason = TransitionWidened, fmt.Sprintf("caveats shrank %d -> %d", len(old.Caveats), len(cur.Caveats))
		}
	}
	if old.Status != cur.Status && transition == TransitionUnchanged {
		transition, reason = TransitionIncomparable, fmt.Sprintf("status %s -> %s without identity change is a ledger-state diff, not a claim transition", old.Status, cur.Status)
	}
	return ClaimDelta{
		ClaimID: cur.ClaimID, Transition: transition,
		BaselineDigest: generationIdentity(old), CurrentDigest: generationIdentity(cur),
		Reason: reason,
	}
}

// ProfileRegressions compares a baseline profile against the current one and
// returns failures when the current profile drops a dimension, narrows scope,
// or changes route — an improved headline number cannot hide lost evidence.
func ProfileRegressions(baseline, current profile.Profile) []string {
	var fails []string
	if baseline.ProtocolVersion != current.ProtocolVersion {
		fails = append(fails, fmt.Sprintf("profile protocol changed %s -> %s", baseline.ProtocolVersion, current.ProtocolVersion))
	}
	if baseline.RouteScope != current.RouteScope {
		fails = append(fails, fmt.Sprintf("profile route changed %s -> %s", baseline.RouteScope, current.RouteScope))
	}
	base := make(map[string]profile.Dimension, len(baseline.Dimensions))
	for _, d := range baseline.Dimensions {
		base[d.ID] = d
	}
	for _, d := range baseline.Dimensions {
		if _, ok := lookupDimension(current, d.ID); !ok {
			fails = append(fails, fmt.Sprintf("profile dropped dimension %s", d.ID))
		}
	}
	for _, d := range current.Dimensions {
		old, ok := base[d.ID]
		if !ok {
			continue
		}
		if d.Scope != old.Scope {
			fails = append(fails, fmt.Sprintf("profile dimension %s scope changed %q -> %q", d.ID, old.Scope, d.Scope))
		}
		if d.EvidenceLevel != old.EvidenceLevel {
			fails = append(fails, fmt.Sprintf("profile dimension %s evidence level changed %s -> %s", d.ID, old.EvidenceLevel, d.EvidenceLevel))
		}
	}
	return fails
}

func lookupDimension(p profile.Profile, id string) (profile.Dimension, bool) {
	for _, d := range p.Dimensions {
		if d.ID == id {
			return d, true
		}
	}
	return profile.Dimension{}, false
}

func generationIdentity(c claimledger.Claim) string {
	return c.Generation.Lane + "/" + c.Generation.Evidence + "/" + c.Generation.Current
}

func expressionIdentity(e claimledger.Expression) string {
	return fmt.Sprint(e)
}
