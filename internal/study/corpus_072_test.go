package study

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestIdenticalResponseDatasetManifestValidates runs the shipped 049
// validateDataset rules over the committed 072 dataset manifest by embedding
// it in a minimal manifest. This is the failable gate: any drift in the
// artifact (bad digests, forbidden role combinations, missing identity)
// breaks this test.
func TestIdenticalResponseDatasetManifestValidates(t *testing.T) {
	raw, err := os.ReadFile("../../eval/governance/identical-response-dataset-manifest-v1.json")
	if err != nil {
		t.Skipf("dataset manifest not present: %v", err)
	}
	var artifact struct {
		Dataset DatasetManifest `json:"dataset"`
	}
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("decode dataset manifest: %v", err)
	}
	ds := artifact.Dataset
	if ds.ID != "identical-response-corpus-v1" {
		t.Fatalf("unexpected dataset id %q", ds.ID)
	}
	if ds.PreviouslyAccessed && (hasRole(ds.PermittedRoles, RoleTest) || hasRole(ds.PermittedRoles, RoleExternalReplication)) {
		t.Fatal("previously accessed corpus must not permit confirmatory test or external-replication roles")
	}
	for _, field := range []struct{ name, digest string }{
		{"dataset", ds.DatasetDigest}, {"task IDs", ds.TaskIDsDigest},
		{"outcome labels", ds.OutcomeLabelsDigest}, {"trajectory set", ds.TrajectorySetDigest},
	} {
		if len(field.digest) != 64 {
			t.Fatalf("%s digest is not SHA-256: %q", field.name, field.digest[:min(8, len(field.digest))])
		}
	}
	if !ds.AcquiredAt.After(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("acquisition time missing: %v", ds.AcquiredAt)
	}
}

func hasRole(roles []DataRole, want DataRole) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}

// TestIdenticalResponseFrozenSplitRecordsFrozenAssignment pins the 072 split
// record: roles come from the frozen controlled-corruption-v3 assignment
// (development/calibration only after the TASK 049 previously-accessed
// demotion), every group keeps its near-duplicate contamination marker, and
// no lineage cluster crosses data roles.
func TestIdenticalResponseFrozenSplitRecordsFrozenAssignment(t *testing.T) {
	rawInventory, err := os.ReadFile("../../eval/governance/identical-response-eligible-inventory-v1.json")
	if err != nil {
		t.Skipf("eligible inventory not present: %v", err)
	}
	rawSplit, err := os.ReadFile("../../eval/governance/identical-response-frozen-split-v1.json")
	if err != nil {
		t.Skipf("frozen split not present: %v", err)
	}
	var inventory struct {
		Groups []struct {
			TaskGroupID       string   `json:"task_group_id"`
			DataRole          string   `json:"data_role"`
			NearDuplicateID   string   `json:"near_duplicate_id"`
			LineageClusterID  string   `json:"lineage_cluster_id"`
			TrajectoryDigests []string `json:"trajectory_digests"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(rawInventory, &inventory); err != nil {
		t.Fatal(err)
	}
	frozenRole := map[string]string{}
	nearDup := map[string]string{}
	lineageRoles := map[string]map[string]bool{}
	for _, g := range inventory.Groups {
		frozenRole[g.TaskGroupID] = g.DataRole
		nearDup[g.NearDuplicateID] = g.TaskGroupID
		if lineageRoles[g.LineageClusterID] == nil {
			lineageRoles[g.LineageClusterID] = map[string]bool{}
		}
		lineageRoles[g.LineageClusterID][g.DataRole] = true
	}
	var split struct {
		SchemaVersion string `json:"schema_version"`
		Assignments   []struct {
			GroupID           string   `json:"group_id"`
			Split             string   `json:"split"`
			CloneFamilyIDs    []string `json:"clone_family_ids"`
			TrajectoryDigests []string `json:"trajectory_digests"`
		} `json:"assignments"`
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(rawSplit, &split); err != nil {
		t.Fatal(err)
	}
	if split.SchemaVersion != "evalwitness.identical-response-frozen-split.v1" {
		t.Fatalf("unexpected split schema %q", split.SchemaVersion)
	}
	if len(split.Assignments) != len(inventory.Groups) {
		t.Fatalf("%d assignments for %d groups", len(split.Assignments), len(inventory.Groups))
	}
	seenNearDup := map[string]bool{}
	for _, a := range split.Assignments {
		want := frozenRole[a.GroupID]
		if a.Split == string(RoleTest) {
			t.Fatalf("group %s kept confirmatory test role despite previously-accessed demotion", a.GroupID)
		}
		if want == string(RoleTest) && a.Split != string(RoleCalibration) {
			t.Fatalf("group %s was test in frozen inventory but became %q", a.GroupID, a.Split)
		}
		if want != string(RoleTest) && a.Split != want {
			t.Fatalf("group %s drifted from frozen role %q to %q", a.GroupID, want, a.Split)
		}
		for _, cf := range a.CloneFamilyIDs {
			if seenNearDup[cf] {
				t.Fatalf("clone family %q appears in multiple assignments", cf)
			}
			seenNearDup[cf] = true
		}
	}
	for cluster, roles := range lineageRoles {
		if len(roles) > 1 {
			t.Fatalf("lineage cluster %s crosses data roles: %v", cluster[:20], roles)
		}
	}
}
