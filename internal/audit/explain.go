package audit

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// ExplainFailures turns every failed policy requirement into a developer-facing
// explanation: which check failed, why the check exists, the exact evidence
// (profile dimension status + capsule expression), and the reproduction
// command pinned to both digests. No secrets, no private paths.
func ExplainFailures(p profile.Profile, pol profile.Policy, fails []string) []Explanation {
	statuses := profile.ClaimCheckStatuses(p)
	exprs := map[string]string{}
	for _, d := range p.Dimensions {
		exprs[d.ID] = d.CapsuleExpr
	}
	explanations := make([]Explanation, 0, len(fails))
	for _, fail := range fails {
		id := requirementID(fail)
		explanation := Explanation{
			Check:    fmt.Sprintf("policy %s requires %s", pol.Version, id),
			Why:      "the checked-in CI policy declares this reliability dimension as a release gate; failing it means the measured evidence does not meet the declared bar",
			Evidence: evidenceFor(id, statuses, exprs),
			Remediation: fmt.Sprintf(
				"reproduce offline: evalwitness audit --policy <policy.json> --profile <profile.json>; pin: policy digest %s, profile digest %s",
				pol.Digest, p.Digest),
		}
		if stringsContains(fail, "missing") {
			explanation.Evidence = fmt.Sprintf("dimension %s is absent from the profile entirely", id)
			explanation.Remediation = "add the missing dimension to the profile build inputs or narrow the policy to the dimensions actually measured"
		}
		explanations = append(explanations, explanation)
	}
	return explanations
}

func requirementID(fail string) string {
	end := stringsIndex(fail, " ")
	if end < 0 {
		return fail
	}
	return fail[:end]
}

func evidenceFor(id string, statuses map[string]profile.Status, exprs map[string]string) string {
	status, ok := statuses[id]
	if !ok {
		return "no measurement exists"
	}
	evidence := fmt.Sprintf("dimension %s status %s", id, status)
	if expr := exprs[id]; expr != "" {
		evidence += fmt.Sprintf(", capsule expression %s", expr)
	}
	return evidence
}

func stringsContains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func stringsIndex(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
