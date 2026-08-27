package capsule

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestWriteAndVerifyDirectoryDeterministically(t *testing.T) {
	registry, manifest, payloads := testPublicPackage(t)
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	if err := WriteDirectory(context.Background(), first, registry, manifest, payloads); err != nil {
		t.Fatal(err)
	}
	if err := WriteDirectory(context.Background(), second, registry, manifest, payloads); err != nil {
		t.Fatal(err)
	}
	assertDirectoryEqual(t, first, second)
	report, err := VerifyDirectory(context.Background(), first, registry, VerificationOptions{MaximumVisibility: VisibilityPublic})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || !report.Offline || report.Components != 4 || report.ScientificComponents != 3 || report.PresentationComponents != 1 {
		t.Fatalf("verification report = %+v", report)
	}
	if report.CapsuleID != manifest.CapsuleID || report.ManifestDigest != manifest.ManifestDigest {
		t.Fatal("verification report lost capsule identity")
	}
	assertTreeMode(t, first, safety.PublicDirectoryMode, safety.PublicFileMode)
}

func TestWriteDirectoryRefusesOverwrite(t *testing.T) {
	registry, manifest, payloads := testPublicPackage(t)
	destination := filepath.Join(t.TempDir(), "capsule")
	if err := WriteDirectory(context.Background(), destination, registry, manifest, payloads); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(destination, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteDirectory(context.Background(), destination, registry, manifest, payloads); err == nil {
		t.Fatal("capsule writer replaced an existing destination")
	}
	after, err := os.ReadFile(filepath.Join(destination, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed overwrite attempt changed the existing capsule")
	}
}

func TestRenameDirectoryNoReplaceRejectsExistingEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.Mkdir(source, safety.PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "source.txt"), []byte("source"), safety.PublicFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination, safety.PublicDirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := RenameDirectoryNoReplace(source, destination); err == nil {
		t.Fatal("exclusive directory publication replaced an existing empty directory")
	}
	if raw, err := os.ReadFile(filepath.Join(source, "source.txt")); err != nil || string(raw) != "source" {
		t.Fatalf("failed exclusive publication changed its source: %q, %v", raw, err)
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed exclusive publication changed its destination: %v, %v", entries, err)
	}
}

func TestVerifyDirectoryRejectsCorruptionAndUnexpectedContent(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, root string, manifest Manifest)
	}{
		{
			name: "payload byte",
			mutate: func(t *testing.T, root string, manifest Manifest) {
				t.Helper()
				path := filepath.Join(root, payloadRelativePath(manifest.Components[0].Payload.Digest))
				if err := os.WriteFile(path, []byte("corrupt"), safety.PublicFileMode); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "extra file",
			mutate: func(t *testing.T, root string, _ Manifest) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "undeclared.txt"), []byte("extra"), safety.PublicFileMode); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "extra directory",
			mutate: func(t *testing.T, root string, _ Manifest) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(root, "undeclared"), safety.PublicDirectoryMode); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlink",
			mutate: func(t *testing.T, root string, _ Manifest) {
				t.Helper()
				if err := os.Symlink(manifestName, filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry, manifest, payloads := testPublicPackage(t)
			root := filepath.Join(t.TempDir(), "capsule")
			if err := WriteDirectory(context.Background(), root, registry, manifest, payloads); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, root, manifest)
			if _, err := VerifyDirectory(context.Background(), root, registry, VerificationOptions{MaximumVisibility: VisibilityPublic}); err == nil {
				t.Fatal("corrupted capsule passed verification")
			}
		})
	}
}

func TestPrivateCapsuleRequiresExplicitVisibility(t *testing.T) {
	registry := testRegistry(t)
	secret, secretBytes := buildTestComponent(t, registry, ComponentInput{
		Name: "secret", TypeID: testSecretType, Visibility: VisibilityPrivate, Payload: []byte("private bytes"),
	})
	commitment, commitmentBytes := buildTestComponent(t, registry, ComponentInput{
		Name: "commitment", TypeID: testCommitmentType, Visibility: VisibilityPublic,
		Payload: []byte(`{"omission":"restricted-source-bytes"}`), Parents: []ParentRef{internalParent(EdgeCommitsTo, secret)},
	})
	manifest := buildTestManifest(t, registry, []ComponentRecord{secret, commitment}, commitment.ComponentID, "")
	payloads := map[string][]byte{secret.Payload.Digest: secretBytes, commitment.Payload.Digest: commitmentBytes}
	root := filepath.Join(t.TempDir(), "private-capsule")
	if err := WriteDirectory(context.Background(), root, registry, manifest, payloads); err != nil {
		t.Fatal(err)
	}
	assertTreeMode(t, root, safety.SensitiveDirectoryMode, safety.SensitiveFileMode)
	if _, err := VerifyDirectory(context.Background(), root, registry, VerificationOptions{MaximumVisibility: VisibilityPublic}); err == nil {
		t.Fatal("public verifier accepted a private component")
	}
	if _, err := VerifyDirectory(context.Background(), root, registry, VerificationOptions{MaximumVisibility: VisibilityPrivate}); err != nil {
		t.Fatal(err)
	}
}

func testPublicPackage(t *testing.T) (*Registry, Manifest, map[string][]byte) {
	t.Helper()
	registry := testRegistry(t)
	plan, planBytes := buildTestComponent(t, registry, ComponentInput{Name: "plan", TypeID: testPlanType, Visibility: VisibilityPublic, Payload: []byte(`{"value":1}`)})
	observation, observationBytes := buildTestComponent(t, registry, ComponentInput{
		Name: "observation", TypeID: testObservationType, Visibility: VisibilityPublic,
		Payload: []byte(`{"value":0.5}`), Parents: []ParentRef{internalParent(EdgeGovernedBy, plan)},
	})
	analysis, analysisBytes := buildTestComponent(t, registry, ComponentInput{
		Name: "analysis", TypeID: testAnalysisType, Visibility: VisibilityPublic,
		Payload: []byte(`{"count":1}`), Parents: []ParentRef{internalParent(EdgeGovernedBy, plan), internalParent(EdgeDerivedFrom, observation)},
	})
	presentation, presentationBytes := buildTestComponent(t, registry, ComponentInput{
		Name: "report", TypeID: testPresentationType, Visibility: VisibilityPublic,
		Payload: []byte("report\n"), Parents: []ParentRef{internalParent(EdgeRenders, analysis)},
	})
	manifest := buildTestManifest(t, registry, []ComponentRecord{plan, observation, analysis, presentation}, analysis.ComponentID, presentation.ComponentID)
	payloads := map[string][]byte{
		plan.Payload.Digest: planBytes, observation.Payload.Digest: observationBytes,
		analysis.Payload.Digest: analysisBytes, presentation.Payload.Digest: presentationBytes,
	}
	return registry, manifest, payloads
}

func assertDirectoryEqual(t *testing.T, left, right string) {
	t.Helper()
	err := fs.WalkDir(os.DirFS(left), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		leftBytes, err := os.ReadFile(filepath.Join(left, path))
		if err != nil {
			return err
		}
		rightBytes, err := os.ReadFile(filepath.Join(right, path))
		if err != nil {
			return err
		}
		if string(leftBytes) != string(rightBytes) {
			return errors.New("capsule directories differ")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertTreeMode(t *testing.T, root string, directoryMode, fileMode fs.FileMode) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		want := fileMode
		if entry.IsDir() {
			want = directoryMode
		}
		if info.Mode().Perm() != want {
			return errors.New("capsule permissions differ from policy")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
