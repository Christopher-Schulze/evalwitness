package audit

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// PolicyResult is offline CI result without network.
type PolicyResult struct {
	Pass   bool     `json:"pass"`
	Fails  []string `json:"fails,omitempty"`
	Policy string   `json:"policy_digest"`
}

// OfflineAudit checks profile against policy offline; no provider call.
func OfflineAudit(p profile.Profile, pol profile.Policy) (PolicyResult, error) {
	if err := profile.Verify(p); err != nil {
		return PolicyResult{}, fmt.Errorf("audit: profile verify: %w", err)
	}
	ok, fails := profile.Evaluate(p, pol)
	return PolicyResult{Pass: ok, Fails: fails, Policy: pol.Digest}, nil
}
