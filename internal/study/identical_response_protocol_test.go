package study

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIdenticalResponseProtocol(t *testing.T) {
	root := identicalResponseRepositoryRoot(t)
	protocol, err := BuildIdenticalResponseProtocol(root)
	if err != nil {
		t.Fatalf("build identical-response protocol: %v", err)
	}
	if err := protocol.Validate(); err != nil {
		t.Fatalf("validate identical-response protocol: %v", err)
	}
	if protocol.Counterfactual != "distribution_aware_vs_chosen_token" {
		t.Fatalf("counterfactual = %q, want distribution_aware_vs_chosen_token", protocol.Counterfactual)
	}
	if protocol.MinimumTaskGroups != IdenticalResponseMinimumTaskGroups || protocol.EligibleTaskGroups < protocol.MinimumTaskGroups {
		t.Fatalf("task-group counts = %d/%d, want minimum %d", protocol.EligibleTaskGroups, protocol.MinimumTaskGroups, IdenticalResponseMinimumTaskGroups)
	}
	if !sameStringSlice(protocol.MultiplicityFamily, []string{"paired_task_group_disagreement"}) {
		t.Fatalf("multiplicity family drifted: %v", protocol.MultiplicityFamily)
	}
	for _, digest := range []string{protocol.DesignSpecDigest, protocol.DesignReportDigest, protocol.InventoryDigest, protocol.RedistributionRightDigest} {
		if !validDigest(digest) {
			t.Fatalf("binding digest %q is not a valid SHA-256", digest)
		}
	}
}

func TestIdenticalResponseProtocolRejectsTampering(t *testing.T) {
	root := identicalResponseRepositoryRoot(t)
	protocol, err := BuildIdenticalResponseProtocol(root)
	if err != nil {
		t.Fatalf("build identical-response protocol: %v", err)
	}

	tampered := protocol
	tampered.Counterfactual = "verifier_vs_judge"
	if err := tampered.Validate(); err == nil {
		t.Fatalf("renaming the counterfactual must be rejected")
	}

	tampered = protocol
	tampered.MultiplicityFamily = append([]string{}, protocol.MultiplicityFamily...)
	tampered.MultiplicityFamily[0] = "multiple_endpoints"
	if err := tampered.Validate(); err == nil {
		t.Fatalf("broadened multiplicity family must be rejected")
	}

	tampered = protocol
	tampered.UnsupportedClaims[0] = "correctness or accuracy is now supported"
	if err := tampered.Validate(); err == nil {
		t.Fatalf("broadened claim boundary must be rejected")
	}

	tampered = protocol
	tampered.MinimumTaskGroups = 10
	if err := tampered.Validate(); err == nil {
		t.Fatalf("weakened minimum task-group count must be rejected")
	}
}

func TestIdenticalResponseProtocolBindingDriftDetected(t *testing.T) {
	root := identicalResponseRepositoryRoot(t)
	protocol, err := BuildIdenticalResponseProtocol(root)
	if err != nil {
		t.Fatalf("build identical-response protocol: %v", err)
	}

	tampered := protocol
	tampered.InventoryDigest = "0" + tampered.InventoryDigest[1:]
	if err := tampered.Validate(); err == nil {
		t.Fatalf("stale inventory binding digest must be rejected")
	}

	// A missing bound artifact must fail the build outright.
	empty, err := os.MkdirTemp("", "identical-response-protocol-empty-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(empty); err != nil {
			t.Errorf("remove empty protocol fixture: %v", err)
		}
	}()
	if err := os.MkdirAll(filepath.Join(empty, "eval", "governance"), 0o755); err != nil {
		t.Fatalf("create governance dir: %v", err)
	}
	if _, err := BuildIdenticalResponseProtocol(empty); err == nil {
		t.Fatalf("missing bound artifact must fail the build")
	}
}
