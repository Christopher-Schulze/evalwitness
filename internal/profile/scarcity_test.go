package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNegativeEvidenceAndRelationScarcityFromCanonicalJSON(t *testing.T) {
	proj, err := LoadNegativeEvidence("../../" + ScarcityEvidencePath)
	if err != nil {
		t.Fatalf("load negative %v", err)
	}
	if proj.Attempts != 198 || proj.Applied != 3 || proj.Rejected != 195 || proj.Target != 40 || proj.Shortfall != 37 {
		t.Fatalf("fields %+v", proj)
	}
	if proj.Attempts != proj.Applied+proj.Rejected {
		t.Fatalf("attempted invariant violated")
	}
	dims, err := LoadRelationScarcity("../../" + ScarcityEvidencePath)
	if err != nil {
		t.Fatalf("load scarcity %v", err)
	}
	if len(dims) != 1 || dims[0].Family != "omitted_test_evidence" || dims[0].Attempted != 198 || !dims[0].TestZero || dims[0].CoreCases != 280 || dims[0].CoreFamilies != 7 {
		t.Fatalf("dims %+v", dims)
	}
	// Sugar wrappers must also succeed from repo root (tests run from package dir, so use relative path via envelope)
	if _, err := NegativeEvidence(); err != nil {
		// When running from package dir, repo-root relative probe fails; try envelope absolute via canonical path file presence
		if _, stat := os.Stat("../../" + ScarcityEvidencePath); stat != nil {
			t.Skipf("NegativeEvidence() repo-root probe not present in this cwd: %v", err)
		} else {
			t.Fatalf("NegativeEvidence %v", err)
		}
	}
	if _, err := RelationScarcityDimensions(); err != nil {
		if _, stat := os.Stat("../../" + ScarcityEvidencePath); stat != nil {
			t.Skipf("RelationScarcityDimensions repo-root probe not present: %v", err)
		} else {
			t.Fatalf("RelationScarcityDimensions %v", err)
		}
	}
}

func TestScarcityTamperedJSONFails(t *testing.T) {
	raw, err := os.ReadFile("../../" + ScarcityEvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	avail := doc["availability"].(map[string]any)
	avail["attempted"] = float64(999)
	body, _ := json.Marshal(doc)
	dir := t.TempDir()
	tampered := filepath.Join(dir, "tampered.json")
	if err := os.WriteFile(tampered, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNegativeEvidence(tampered); err == nil {
		t.Fatal("tampered attempted should fail digest + invariant check")
	}
	// Unknown-field variant
	var doc2 map[string]any
	_ = json.Unmarshal(raw, &doc2)
	doc2["unknown_field"] = "oops"
	body2, _ := json.Marshal(doc2)
	tampered2 := filepath.Join(dir, "tampered2.json")
	_ = os.WriteFile(tampered2, body2, 0o644)
	if _, err := LoadNegativeEvidence(tampered2); err == nil {
		t.Fatal("unknown field should fail DisallowUnknownFields")
	}
	var buf bytes.Buffer
	buf.Write(raw)
	_ = buf
}
