package study

import (
	"os"
	"path/filepath"
	"testing"
)

func identicalResponseRepositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve package directory: %v", err)
	}
	for {
		release := filepath.Join(root, "eval", "governance", "controlled-corruption-v3-release.json")
		if _, statErr := os.Stat(release); statErr == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("could not locate repository root with the governed release")
		}
		root = parent
	}
}

func TestBuildIdenticalResponseEligibleInventory(t *testing.T) {
	root := identicalResponseRepositoryRoot(t)
	inventory, err := BuildIdenticalResponseEligibleInventory(root)
	if err != nil {
		t.Fatalf("build eligible inventory: %v", err)
	}
	if err := inventory.Validate(); err != nil {
		t.Fatalf("validate eligible inventory: %v", err)
	}
	if inventory.EligibleTaskGroups != 100 || len(inventory.Groups) != 100 {
		t.Fatalf("eligible task groups = %d (groups %d), want 100", inventory.EligibleTaskGroups, len(inventory.Groups))
	}
	if inventory.MinimumTaskGroups != IdenticalResponseMinimumTaskGroups {
		t.Fatalf("minimum task groups = %d, want %d", inventory.MinimumTaskGroups, IdenticalResponseMinimumTaskGroups)
	}
	if inventory.RedistributionAuthorized {
		t.Fatalf("reference-only upstream sources must not be marked redistribution-authorized")
	}
	if inventory.ClaimBoundary.ProviderCalls != 0 || inventory.ClaimBoundary.AgentLaunches != 0 || inventory.ClaimBoundary.LiveResponseAccess {
		t.Fatalf("offline inventory must declare zero provider calls, zero agent launches, and no live access")
	}
	roles := map[string]int{}
	for _, row := range inventory.TaskGroupsByRole {
		roles[row.ID] = row.Count
	}
	if roles["development"] != 60 || roles["calibration"] != 20 || roles["test"] != 20 {
		t.Fatalf("task group role denominators = %v, want development=60 calibration=20 test=20", roles)
	}
	for _, group := range inventory.Groups {
		if group.RedistributionClass != "reference_only" {
			t.Fatalf("task group %q redistribution class = %q, want reference_only", group.TaskGroupID, group.RedistributionClass)
		}
		if len(group.SourceDigests) != 2 || len(group.TrajectoryDigests) != 2 || len(group.OutcomeWitnessDigests) != 2 {
			t.Fatalf("task group %q must bind exactly two source/trajectory/evidence digests", group.TaskGroupID)
		}
	}
}

func TestIdenticalResponseEligibleInventoryRejectsTampering(t *testing.T) {
	root := identicalResponseRepositoryRoot(t)
	inventory, err := BuildIdenticalResponseEligibleInventory(root)
	if err != nil {
		t.Fatalf("build eligible inventory: %v", err)
	}

	tampered := inventory
	tampered.RedistributionAuthorized = true
	if err := tampered.Validate(); err == nil {
		t.Fatalf("redistribution authorization tampering must be rejected")
	}

	tampered = inventory
	tampered.EligibleTaskGroups = 39
	if err := tampered.Validate(); err == nil {
		t.Fatalf("sub-40 task-group denominator tampering must be rejected")
	}

	tampered = inventory
	digest := tampered.Groups[0].SourceDigests[0]
	if digest[len(digest)-1] == 'a' {
		tampered.Groups[0].SourceDigests[0] = digest[:len(digest)-1] + "b"
	} else {
		tampered.Groups[0].SourceDigests[0] = digest[:len(digest)-1] + "a"
	}
	if err := tampered.Validate(); err == nil {
		t.Fatalf("source digest tampering must be rejected")
	}

	tampered = inventory
	tampered.Groups = tampered.Groups[:len(tampered.Groups)-1]
	if err := tampered.Validate(); err == nil {
		t.Fatalf("group removal tampering must be rejected")
	}
}
