package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// captureTraceExport runs runTraceExport with stdout redirected and returns
// the sha256 of the emitted artifact plus the raw bytes. It pins TASK 054
// export determinism: same input trace -> byte-identical mapping output.
func captureTraceExport(t *testing.T, target, source string) (string, []byte) {
	t.Helper()
	stdout := os.Stdout
	tmp, err := os.CreateTemp(t.TempDir(), "trace-export-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmp.Name()); err != nil {
			t.Errorf("remove trace export temporary file: %v", err)
		}
	}()
	os.Stdout = tmp
	code := runTraceExport([]string{"--source", source, "--target", target, "--privacy-class", "content_and_attribution_authorized", "--output", "json"})
	os.Stdout = stdout
	if code != 0 {
		t.Fatalf("trace export %s exit %d", target, code)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw
}

// TestTraceExportDeterministicAcrossTargets drives the three shipped TASK 054
// mapping targets over one fixed source trace twice each. Any byte difference
// between runs breaks the conformance harness invariant that equivalent
// canonical inputs produce identical canonical digests.
func TestTraceExportDeterministicAcrossTargets(t *testing.T) {
	source := filepath.Join("..", "..", "internal", "preprocess", "testdata", "trace", "agent-trace-0.1.0.json")
	if _, err := os.Stat(source); err != nil {
		t.Skipf("trace fixture missing: %v", err)
	}
	for _, target := range []string{"canonical", "otlp", "agent-trace"} {
		target := target
		t.Run(target, func(t *testing.T) {
			firstDigest, firstBytes := captureTraceExport(t, target, source)
			if len(firstBytes) == 0 {
				t.Fatal("empty export bytes")
			}
			secondDigest, _ := captureTraceExport(t, target, source)
			if firstDigest != secondDigest {
				t.Fatalf("%s export not deterministic: %s vs %s", target, firstDigest[:16], secondDigest[:16])
			}
			t.Logf("%s export: %d bytes, digest %s", target, len(firstBytes), firstDigest[:16])
		})
	}
}
