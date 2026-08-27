package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

func TestCheckProfileForClaimVerifyStrictAndSurfaced(t *testing.T) {
	metric := "0.12"
	p, err := profile.Build("test-profile", "evalwitness.protocol.v1", "routeA", []profile.Dimension{
		{ID: "calibration", Status: profile.StatusMeasured, Metric: &metric, Scope: "terminal", EvidenceLevel: "E1", CapsuleExpr: "calibration.metrics.ece", Denominator: 100, SampleUnit: "task"},
	})
	if err != nil {
		t.Fatalf("build %v", err)
	}
	dir := t.TempDir()
	validPath := filepath.Join(dir, "valid.json")
	raw, _ := json.Marshal(p)
	if err := os.WriteFile(validPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	exprs, statuses, err := checkProfileForClaimVerify(validPath)
	if err != nil {
		t.Fatalf("valid profile should pass strict check: %v", err)
	}
	if len(exprs) != 1 || exprs[0] == "" || len(statuses) != 1 || statuses["calibration"] != profile.StatusMeasured {
		t.Fatalf("valid surfaced exprs/statuses wrong: %v %v", exprs, statuses)
	}
	// Tampered digest should fail
	var tampered profile.Profile
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.Digest = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	tamperedRaw, _ := json.Marshal(tampered)
	tamperedPath := filepath.Join(dir, "tampered.json")
	_ = os.WriteFile(tamperedPath, tamperedRaw, 0o644)
	if _, _, err := checkProfileForClaimVerify(tamperedPath); err == nil {
		t.Fatal("tampered digest should fail")
	}
	// Unknown field should fail via DisallowUnknownFields
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["unknown_field"] = "oops"
	unknownRaw, _ := json.Marshal(m)
	unknownPath := filepath.Join(dir, "unknown.json")
	_ = os.WriteFile(unknownPath, unknownRaw, 0o644)
	if _, _, err := checkProfileForClaimVerify(unknownPath); err == nil {
		t.Fatal("unknown field should fail DisallowUnknownFields")
	}
}
