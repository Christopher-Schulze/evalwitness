package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalibrationEvaluatePropagatesScopedEvaluationErrors(t *testing.T) {
	root := t.TempDir()
	observationsPath := filepath.Join(root, "observations.json")
	artifactPath := filepath.Join(root, "artifact.json")
	if err := os.WriteFile(observationsPath, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := runCapture(t, func() int {
		return runCalibrationEvaluate([]string{
			"--observations", observationsPath,
			"--artifact", artifactPath,
			"--route", "route-test",
			"--domain", "test-domain",
		})
	})
	if result.code != 1 || !contains(result.stderr, "calibration: empty test split") {
		t.Fatalf("scoped evaluation result = code %d stdout %q stderr %q", result.code, result.stdout, result.stderr)
	}
	if result.stdout != "" {
		t.Fatalf("failed scoped evaluation emitted output %q", result.stdout)
	}
}
