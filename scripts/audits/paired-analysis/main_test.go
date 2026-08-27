package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func TestLoadArtifactRejectsDuplicateTaskIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "duplicate.json")
	raw := []byte(`{"details":[{"task_name":"task-1","rewards":[0,1]},{"task_name":"task-1","rewards":[0,1]}]}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadArtifact(path); err == nil {
		t.Fatal("duplicate task IDs were accepted")
	}
}

func TestLoadArtifactPreservesTypedPairedInputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "details.json")
	raw := []byte(`{"usage":{"extraction_mode":"verifier"},"details":[{"instance_id":"task-1","rewards":[0,1],"selected_index":1,"selected_reward":1}]}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	row := loaded.Rows["task-1"]
	if loaded.Mode != "verifier" || !stats.DecidableBinary(row.Rewards) || row.SelectedReward == nil || *row.SelectedReward != 1 {
		t.Fatalf("loaded artifact = %#v", loaded)
	}
}

func TestLoadArtifactRejectsInconsistentSelection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inconsistent.json")
	raw := []byte(`{"details":[{"task_name":"task-1","rewards":[0,1],"selected_index":0,"selected_reward":1}]}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadArtifact(path); err == nil {
		t.Fatal("inconsistent selected reward was accepted")
	}
}
