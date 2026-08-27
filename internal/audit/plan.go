package audit

import (
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// SchemaVersion is the versioned CI audit contract.
const SchemaVersion = "evalwitness.audit-result.v1"

// Exit codes are stable and documented in docs/spec.md.
const (
	ExitPass          = 0
	ExitPolicyFailed  = 1
	ExitInvalidInput  = 2
	ExitInternalError = 3
)

// PlanStep is one resolved offline step the executor will run.
type PlanStep struct {
	ID      string   `json:"id"`
	Command []string `json:"command"`
	Digest  string   `json:"digest"`
}

// Explanation carries why a check exists, what failed, and how to reproduce.
type Explanation struct {
	Check       string `json:"check"`
	Why         string `json:"why"`
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation"`
}

// Result is the canonical offline audit result emitted as JSON.
type Result struct {
	SchemaVersion string                  `json:"schema_version"`
	Pass          bool                    `json:"pass"`
	Offline       bool                    `json:"offline"`
	PolicyDigest  string                  `json:"policy_digest"`
	ProfileDigest string                  `json:"profile_digest"`
	Fails         []string                `json:"fails,omitempty"`
	Explanations  []Explanation           `json:"explanations,omitempty"`
	ProfileReport *profile.EvidenceReport `json:"profile_report,omitempty"`
}

// Plan resolves declared local inputs into ordered offline steps and prints
// them before execution. It never upgrades to live: credentials never change
// the plan. Policy identity comes from the recomputed content digest; a
// declared Digest field is accepted only when it matches (tamper evidence).
func Plan(pol profile.Policy, p profile.Profile) ([]PlanStep, error) {
	digest, err := pol.DigestValue()
	if err != nil {
		return nil, fmt.Errorf("audit plan: policy incomplete: %w", err)
	}
	if pol.Digest != "" && pol.Digest != digest {
		return nil, fmt.Errorf("audit plan: declared policy digest mismatch: %s vs %s", pol.Digest, digest)
	}
	if err := profile.Verify(p); err != nil {
		return nil, fmt.Errorf("audit plan: profile verify: %w", err)
	}
	return []PlanStep{
		{ID: "profile-policy", Digest: digest},
	}, nil
}
