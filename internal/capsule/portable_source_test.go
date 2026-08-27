package capsule

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableSourceTreeReproducesCleanGitProvenanceWithoutGitHistory(t *testing.T) {
	repository, portableRoot, indexPath, expected := createPortableSourceFixture(t)
	if repository == portableRoot {
		t.Fatal("portable fixture unexpectedly reused the Git repository")
	}
	t.Setenv(PortableSourceTreeEnvironment, indexPath)
	t.Setenv(PortableSourceRootEnvironment, portableRoot)
	actual, err := CollectSourceTreeProvenance(context.Background(), portableRoot)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Digest != expected.Digest || actual.Commit != expected.Commit || actual.Files != expected.Files || actual.Bytes != expected.Bytes {
		t.Fatalf("portable source provenance = %+v, want identity %+v", actual, expected)
	}
	runFixtureCommand(t, context.Background(), portableRoot, "git", "init", "--quiet")
	if _, err := CollectSourceTreeProvenance(context.Background(), portableRoot); err != nil {
		t.Fatalf("ephemeral Git harness changed portable source identity: %v", err)
	}
}

func TestPortableSourceTreeRejectsContentAndInventoryDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{
			name: "changed content",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				writeFixtureFile(t, filepath.Join(root, "README.md"), "changed\n")
			},
		},
		{
			name: "additional file",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				writeFixtureFile(t, filepath.Join(root, "extra.txt"), "extra\n")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, portableRoot, indexPath, _ := createPortableSourceFixture(t)
			test.mutate(t, portableRoot)
			if _, err := LoadPortableSourceTreeProvenance(portableRoot, indexPath); err == nil {
				t.Fatal("portable source-tree verifier accepted drift")
			}
		})
	}
}

func createPortableSourceFixture(t *testing.T) (string, string, string, SourceTreeProvenance) {
	t.Helper()
	ctx := context.Background()
	repository := t.TempDir()
	files := map[string]string{
		"README.md":      "portable source\n",
		"scripts/run.sh": "#!/bin/sh\nexit 0\n",
	}
	for name, content := range files {
		filePath := filepath.Join(repository, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFixtureFile(t, filePath, content)
	}
	if err := os.Chmod(filepath.Join(repository, "scripts", "run.sh"), 0o700); err != nil {
		t.Fatal(err)
	}
	runFixtureCommand(t, ctx, repository, "git", "init", "--quiet")
	runFixtureCommand(t, ctx, repository, "git", "config", "user.name", "EvalWitness Portable Test")
	runFixtureCommand(t, ctx, repository, "git", "config", "user.email", "portable-test@evalwitness.invalid")
	runFixtureCommand(t, ctx, repository, "git", "add", ".")
	runFixtureCommand(t, ctx, repository, "git", "commit", "--quiet", "-m", "portable fixture")
	source, err := CollectGitSourceTreeProvenance(ctx, repository)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeSourceTreeProvenance(source)
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(t.TempDir(), "source-tree-provenance.json")
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	portableRoot := t.TempDir()
	for name, content := range files {
		filePath := filepath.Join(portableRoot, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return repository, portableRoot, indexPath, source
}
