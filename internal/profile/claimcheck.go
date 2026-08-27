package profile

import "fmt"

// ClaimCheckExpr returns claimcheck expressions for profile with status scope.
func ClaimCheckExpr(p Profile) []string {
	var out []string
	for _, d := range p.Dimensions {
		out = append(out, fmt.Sprintf("%s:%s:%s", d.ID, d.Status, d.CapsuleExpr))
	}
	return out
}

// ClaimCheckStatuses maps dimension ID to its evidence status for TASK 050 gate.
func ClaimCheckStatuses(p Profile) map[string]Status {
	m := make(map[string]Status, len(p.Dimensions))
	for _, d := range p.Dimensions {
		m[d.ID] = d.Status
	}
	return m
}
