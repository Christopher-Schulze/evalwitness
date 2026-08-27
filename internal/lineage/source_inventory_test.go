package lineage

import (
	"bytes"
	"os"
	"testing"
)

func TestDefaultSourceInventoryMatchesCheckedInArtifact(t *testing.T) {
	inventory, err := DefaultSourceInventory()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(inventory)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := os.ReadFile("../../eval/governance/verification-lineage-source-inventory-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, artifact) {
		t.Fatal("checked-in source inventory differs from the canonical inventory")
	}
}

func TestSourceInventoryRejectsHistoricalGoldensAsResearchData(t *testing.T) {
	inventory, err := DefaultSourceInventory()
	if err != nil {
		t.Fatal(err)
	}
	goldens := inventory.Candidates[2]
	if goldens.ID != "checked_in_vendor_goldens" || goldens.AdmissionStatus != "rejected_for_corpus_adapter_development_only" ||
		goldens.LicenseStatus != "unresolved_per_source" || goldens.AuthoritativeCaptureStatus != "absent" {
		t.Fatal("historical vendor goldens crossed the TASK-069 source-admission boundary")
	}
	if inventory.Candidates[3].Materialized || inventory.Candidates[4].Materialized || inventory.TaskGroupCountsInspected {
		t.Fatal("source inventory inspected or materialized an unauthorized acquisition source")
	}
}

func TestSourceInventoryRejectsResealedAdmissionDrift(t *testing.T) {
	inventory, err := DefaultSourceInventory()
	if err != nil {
		t.Fatal(err)
	}
	inventory.Candidates[2].AdmissionStatus = "admitted"
	inventory.Digest, err = sourceInventoryDigest(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if err := inventory.Validate(); err == nil {
		t.Fatal("resealed source-admission drift was accepted")
	}
}
