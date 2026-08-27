package verification

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
)

func TestDefaultFallbackAccountIsUncharged(t *testing.T) {
	account := DefaultFallbackAccount()
	if account.Kind != "none" || account.Charged || account.Calls != 0 {
		t.Fatalf("fallback = %+v", account)
	}
}

func TestDefaultCalibrationPolicyIsUnsupported(t *testing.T) {
	policy := DefaultCalibrationPolicy()
	if policy.Status != CalibrationPolicyUnsupported || policy.Reason == "" {
		t.Fatalf("policy = %+v", policy)
	}
}

func TestPolicyFromConfigDisablesLegacyConfidenceEscalation(t *testing.T) {
	policy := PolicyFromConfig(config.Default(), 1, 0.02, "adaptive", false, "pairwise")
	if policy.ConfidenceEscalation != mode.ConfidenceEscalationDisabled {
		t.Fatalf("confidence escalation = %q", policy.ConfidenceEscalation)
	}
}
