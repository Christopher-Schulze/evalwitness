package safety

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactReviewManifestPinsExactFileContent(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "result.txt")
	if err := os.WriteFile(artifact, []byte("api_key=synthetic-fixture-value\n"), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	report, err := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{root}, Class: ArtifactPublic, Limits: DefaultArtifactScanLimits(),
	})
	if !IsKind(err, ErrorSecretDetected) || len(report.Findings) != 1 {
		t.Fatalf("scan report = %+v, %v", report, err)
	}
	if len(report.Findings[0].FileSHA256) != 64 {
		t.Fatalf("file digest = %q", report.Findings[0].FileSHA256)
	}
	manifest, err := ReviewManifestFromReport("synthetic-corpus", report)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "review.json")
	if err := WriteArtifactReviewCandidate(path, manifest); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadArtifactReviewManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := ApplyArtifactReviewManifest(report, loaded)
	if err != nil || len(accepted.Findings) != 0 || len(accepted.ReviewedFindings) != 1 {
		t.Fatalf("accepted report = %+v, %v", accepted, err)
	}

	if err := os.WriteFile(artifact, []byte("api_key=changed-synthetic-value\n"), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	changed, scanErr := ScanArtifacts(ArtifactScanRequest{
		Roots: []string{root}, Class: ArtifactPublic, Limits: DefaultArtifactScanLimits(),
	})
	if !IsKind(scanErr, ErrorSecretDetected) {
		t.Fatalf("changed scan error = %T %v", scanErr, scanErr)
	}
	if _, err := ApplyArtifactReviewManifest(changed, loaded); !IsKind(err, ErrorArtifactPolicyViolation) {
		t.Fatalf("stale review error = %T %v", err, err)
	}
}

func TestArtifactReviewManifestIsStrictAndCannotOverwrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "review.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1}`), PublicFileMode); err != nil {
		t.Fatal(err)
	}
	manifest := ArtifactReviewManifest{
		SchemaVersion: ArtifactReviewSchema,
		ArtifactID:    "artifact",
		Findings: []ArtifactFinding{{
			Rule: "credential_assignment", Path: "result.json", Line: 1,
			FileSHA256: strings.Repeat("a", 64),
		}},
	}
	if err := WriteArtifactReviewCandidate(path, manifest); !IsKind(err, ErrorConcurrentMutation) {
		t.Fatalf("overwrite error = %T %v", err, err)
	}
	if _, err := LoadArtifactReviewManifest(path); !IsKind(err, ErrorArtifactPolicyViolation) {
		t.Fatalf("strict load error = %T %v", err, err)
	}
	unsafePath := filepath.Join(root, "unsafe.json")
	raw := `{"schema_version":1,"artifact_id":"artifact","reviewed_findings":[{"rule":"credential_assignment","path":"result.json","line":1,"file_sha256":"` + strings.Repeat("a", 64) + `"}]}`
	if err := os.WriteFile(unsafePath, []byte(raw), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unsafePath, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadArtifactReviewManifest(unsafePath); !IsKind(err, ErrorArtifactPolicyViolation) {
		t.Fatalf("unsafe mode error = %T %v", err, err)
	}
}
