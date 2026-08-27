package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sealPanel mirrors the reliance package's sealing: marshal the struct with
// Digest cleared, sha256 it, set Digest — the exact contract the loader
// enforces.
func sealPanel(t *testing.T, panel reliancePanel) reliancePanel {
	t.Helper()
	panel.Digest = ""
	encoded, err := json.Marshal(panel)
	if err != nil {
		t.Fatal(err)
	}
	panel.Digest = sha256Hex(encoded)
	return panel
}

func writePanel(t *testing.T, panel reliancePanel) string {
	t.Helper()
	raw, err := json.Marshal(panel)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "panel.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckRelianceRequirementsAgainstFrozenPanel(t *testing.T) {
	panelPath := writePanel(t, sealPanel(t, reliancePanel{
		SchemaVersion:         reliancePanelSchemaVersion,
		GlobalScoreProhibited: true,
		Dimensions: []reliancePanelDimension{
			{MapTermID: "command_exit", TermKind: "main_effect", Status: "measured", EvidenceLevel: "E1"},
			{MapTermID: "narrative_injection", TermKind: "main_effect", Status: "not_measured", EvidenceLevel: "E1"},
		},
	}))
	panel, err := LoadReliancePanel(panelPath)
	if err != nil {
		t.Fatalf("load %v", err)
	}
	passing := []RelianceRequirement{{MapTermID: "command_exit", Status: "measured", EvidenceLevel: "E1"}}
	if fails := CheckRelianceRequirements(panel, passing); len(fails) != 0 {
		t.Fatalf("satisfied requirement must pass: %v", fails)
	}
	unmeasured := []RelianceRequirement{{MapTermID: "narrative_injection", Status: "measured", EvidenceLevel: "E1"}}
	fails := CheckRelianceRequirements(panel, unmeasured)
	if len(fails) != 1 || !strings.Contains(fails[0], "term_kind any term kind") || !strings.Contains(fails[0], "slicing scope mismatch") {
		t.Fatalf("unmeasured factor must fail with panel-incompatible terms: %v", fails)
	}
	wrongLevel := []RelianceRequirement{{MapTermID: "command_exit", TermKind: "main_effect", Status: "measured", EvidenceLevel: "E4"}}
	fails = CheckRelianceRequirements(panel, wrongLevel)
	if len(fails) != 1 || !strings.Contains(fails[0], "evidence level E4") || !strings.Contains(fails[0], "term_kind main_effect") {
		t.Fatalf("wrong evidence level must fail: %v", fails)
	}
}

func TestLoadReliancePanelStrictAndTamperEvident(t *testing.T) {
	sealed := sealPanel(t, reliancePanel{
		SchemaVersion:         reliancePanelSchemaVersion,
		GlobalScoreProhibited: true,
		Dimensions:            []reliancePanelDimension{{MapTermID: "x", Status: "measured", EvidenceLevel: "E1"}},
	})
	if _, err := LoadReliancePanel(writePanel(t, sealed)); err != nil {
		t.Fatalf("valid sealed panel must load: %v", err)
	}
	// Tamper the content but keep the digest: mismatch must fail.
	tampered := sealed
	tampered.Dimensions[0].Status = "failed"
	if _, err := LoadReliancePanel(writePanel(t, tampered)); err == nil {
		t.Fatal("tampered content must fail digest recompute")
	}
	// Unknown field fails DisallowUnknownFields.
	raw, _ := json.Marshal(sealed)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["bogus"] = true
	unknownRaw, _ := json.Marshal(m)
	unknown := filepath.Join(t.TempDir(), "unknown.json")
	_ = os.WriteFile(unknown, unknownRaw, 0o644)
	if _, err := LoadReliancePanel(unknown); err == nil {
		t.Fatal("unknown field must fail DisallowUnknownFields")
	}
	// Malformed raw bytes fail at the digest probe instead of degrading.
	malformed := filepath.Join(t.TempDir(), "malformed.json")
	if err := os.WriteFile(malformed, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReliancePanel(malformed); err == nil {
		t.Fatalf("malformed file must surface probe error: %v", err)
	}
	// Wrong schema fails.
	wrongSchema := strings.Replace(string(raw), reliancePanelSchemaVersion, "bogus.v9", 1)
	wrongPath := filepath.Join(t.TempDir(), "schema.json")
	_ = os.WriteFile(wrongPath, []byte(wrongSchema), 0o644)
	if _, err := LoadReliancePanel(wrongPath); err == nil {
		t.Fatal("wrong schema must fail")
	}
}

// TestLoadReliancePanelRealFrozenFile proves the local mirror structs are
// byte-identical to internal/reliance's declaration order: the real frozen
// eval/results file must decode and recompute to its own sealed digest.
func TestLoadReliancePanelRealFrozenFile(t *testing.T) {
	const real = "../../eval/results/evidence-reliance-profile-v1.json"
	if _, err := os.Stat(real); err != nil {
		t.Skipf("frozen panel not present: %v", err)
	}
	panel, err := LoadReliancePanel(real)
	if err != nil {
		t.Fatalf("real frozen panel must load and verify its digest: %v", err)
	}
	if len(panel.Dimensions) != 98 || !panel.GlobalScoreProhibited {
		t.Fatalf("panel shape drifted: dims=%d prohibited=%t", len(panel.Dimensions), panel.GlobalScoreProhibited)
	}
}
