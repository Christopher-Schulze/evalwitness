package capsule

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectBuildProvenanceBindsDirtyState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := t.TempDir()
	writeFixtureFile(t, filepath.Join(repository, "go.mod"), "module example.com/buildfixture\n\ngo 1.23\n")
	mainPath := filepath.Join(repository, "main.go")
	writeFixtureFile(t, mainPath, "package main\n\nfunc main() {}\n")
	runFixtureCommand(t, ctx, repository, "git", "init", "--quiet")
	runFixtureCommand(t, ctx, repository, "git", "config", "user.name", "EvalWitness Test")
	runFixtureCommand(t, ctx, repository, "git", "config", "user.email", "evalwitness-test@example.invalid")
	runFixtureCommand(t, ctx, repository, "git", "add", "go.mod", "main.go")
	runFixtureCommand(t, ctx, repository, "git", "commit", "--quiet", "-m", "fixture")

	cleanBinary := filepath.Join(t.TempDir(), "clean-fixture")
	runFixtureCommand(t, ctx, repository, "go", "build", "-o", cleanBinary, ".")
	writeFixtureFile(t, mainPath, "package main\n\nvar dirty = true\n\nfunc main() { _ = dirty }\n")
	_, _, err := CollectBuildProvenance(ctx, repository, cleanBinary, "test-binary")
	if err == nil || !strings.Contains(err.Error(), "modified state differs") {
		t.Fatalf("collect clean binary over dirty source error = %v, want modified-state rejection", err)
	}

	dirtyBinary := filepath.Join(t.TempDir(), "dirty-fixture")
	runFixtureCommand(t, ctx, repository, "go", "build", "-o", dirtyBinary, ".")
	source, build, err := CollectBuildProvenance(ctx, repository, dirtyBinary, "test-binary")
	if err != nil {
		t.Fatalf("collect dirty build provenance: %v", err)
	}
	if !source.Dirty || !build.Dirty || build.EmbeddedVCSModified != "true" || build.SourceMatchStatus != "matched" {
		t.Fatalf("dirty provenance = source:%v build:%v embedded:%q match:%q", source.Dirty, build.Dirty, build.EmbeddedVCSModified, build.SourceMatchStatus)
	}
	if build.SchemaVersion != BuildProvenanceSchemaVersion || build.ProductVersion == "" {
		t.Fatalf("build product identity = schema:%q version:%q", build.SchemaVersion, build.ProductVersion)
	}

	build.Dirty = false
	build.Digest, err = buildProvenanceDigest(build)
	if err != nil {
		t.Fatalf("digest mismatched provenance: %v", err)
	}
	if err := build.Validate(); err == nil || !strings.Contains(err.Error(), "modified state disagrees") {
		t.Fatalf("validate mismatched dirty provenance error = %v, want modified-state rejection", err)
	}
}

func TestSameDirectoryAcceptsCaseVariantOnCaseInsensitiveFilesystem(t *testing.T) {
	parent := t.TempDir()
	canonical := filepath.Join(parent, "CaseRoot")
	if err := os.Mkdir(canonical, 0o700); err != nil {
		t.Fatal(err)
	}
	variant := filepath.Join(parent, "caseroot")
	if _, err := os.Stat(variant); err != nil {
		t.Skip("filesystem is case-sensitive")
	}
	same, err := sameDirectory(canonical, variant)
	if err != nil || !same {
		t.Fatalf("sameDirectory(%q, %q) = %t, %v", canonical, variant, same, err)
	}
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func runFixtureCommand(t *testing.T, ctx context.Context, directory, name string, args ...string) {
	t.Helper()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = directory
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run %s %v: %v: %s", name, args, err, output)
	}
}
