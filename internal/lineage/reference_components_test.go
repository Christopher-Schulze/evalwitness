package lineage

import (
	"path/filepath"
	"testing"
)

func TestVerificationLineageReferenceComponentsBindTenExactArtifacts(t *testing.T) {
	components, err := BuildVerificationLineageReferenceComponents(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := components.Validate(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"plan":         "5f56a357721a4ec2b650660a4efb7b4d9b67d342ada388a5d28f6e607fd9189c",
		"source":       "1045664f1f03d211a26eb6488f5f132859a41f3023402e79e80081730154d65a",
		"witness":      "d41a8c35255b7176a3721f3481e45e6c5da9be198cbca1f6bf31bbf9cad848c9",
		"candidate":    "4be901a26a698894613830f0f009f535a825be4b1bde897d547ad685cf30239d",
		"assessment":   "d74a7bcd38296518993203cfcc4ec6cc36b659e65270d48284926aa858018aff",
		"capability":   "513ce19684ca845c4300e40ff4a88ef7b0cb995fbd688f698844d085e520de37",
		"audit":        "f2a1b543986c7fad5cb665855c958869137ec9e6eae342adef1e09fa236be745",
		"bom":          "c1d15d54d2108d19a3261fe292a72fecd1993409094139d4332c8589c016e4c0",
		"dataset_card": "1a50d635eba0926f2954dee9dbd03bb2221b229669cb14967b7a11766225a1df",
		"release":      "a63eaf5b7a3d3db966b01df87818044469a837ff4a6b4de0020982fd3d403a5d",
	}
	got := map[string]string{
		"plan": components.Plan.Digest, "source": components.Source.Header.Digest,
		"witness": components.Witness.Header.Digest, "candidate": components.Candidate.Header.Digest,
		"assessment": components.Assessment.Header.Digest,
		"audit":      components.Audit.Header.Digest, "bom": components.BOM.Header.Digest,
		"dataset_card": components.DatasetCard.Header.Digest, "release": components.Release.Header.Digest,
	}
	for _, capability := range components.Capabilities {
		if capability.Header.ObjectID == "capability-codex_rollout_jsonl" {
			got["capability"] = capability.Header.Digest
		}
	}
	for name, digest := range want {
		if got[name] != digest {
			t.Fatalf("%s digest = %s, want %s", name, got[name], digest)
		}
	}
}
