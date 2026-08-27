package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestExecuteArtifactScanUsesEnvironmentSecretsWithoutDisclosingThem(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, safety.PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	const secret = "opaque-environment-value-747d"
	path := filepath.Join(root, "result.json")
	if err := os.WriteFile(path, []byte(`{"value":"`+secret+`"}`), safety.PublicFileMode); err != nil {
		t.Fatal(err)
	}
	limits := safety.DefaultArtifactScanLimits()
	report, err := executeArtifactScan([]string{root}, safety.ArtifactPublic, &limits, []string{"PROVIDER_API_KEY=" + secret})
	if !safety.IsKind(err, safety.ErrorSecretDetected) {
		t.Fatalf("error = %T %v, want secret detected", err, err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != "registered_secret" {
		t.Fatalf("findings = %+v", report.Findings)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(report.Findings[0].Path, secret) {
		t.Fatal("secret disclosed by scan result")
	}
}

func TestExecuteArtifactScanRejectsNilLimits(t *testing.T) {
	_, err := executeArtifactScan([]string{"."}, safety.ArtifactPublic, nil, nil)
	if !safety.IsKind(err, safety.ErrorInvalidInput) {
		t.Fatalf("error = %T %v, want invalid input", err, err)
	}
}
