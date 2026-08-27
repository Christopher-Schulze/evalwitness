package verification

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
)

func TestPolicyFromConfigNormalizesDefaultPairwiseStrategy(t *testing.T) {
	cfg := config.Default()
	policy := PolicyFromConfig(cfg, 1, cfg.Epsilon, cfg.BiasMitigation, false, cfg.Selection)
	if policy.SelectionStrategy != "pairwise" {
		t.Fatalf("selection strategy = %q, want pairwise", policy.SelectionStrategy)
	}
}
