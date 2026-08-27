package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/product"
)

func TestCreateSourceArchiveIsDeterministicStrictUSTAR(t *testing.T) {
	repository, commit := createSourceArchiveRepository(t)
	firstPath := filepath.Join(t.TempDir(), ProductName+"-"+product.Version+"-source.tar.gz")
	secondPath := filepath.Join(t.TempDir(), ProductName+"-"+product.Version+"-source.tar.gz")
	first, err := CreateSourceArchive(context.Background(), repository, commit, firstPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateSourceArchive(context.Background(), repository, commit, secondPath)
	if err != nil {
		t.Fatal(err)
	}
	firstRaw, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondRaw, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstRaw, secondRaw) || first != second {
		t.Fatal("identical committed trees did not produce byte-identical archives and reports")
	}
	if first.SchemaVersion != SourceArchiveReportSchemaVersion || first.Format != sourceArchiveFormat || !first.Deterministic || first.Files != 3 || first.Directories != 3 {
		t.Fatalf("unexpected source archive report: %+v", first)
	}

	archive, err := gzip.NewReader(bytes.NewReader(firstRaw))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := archive.Close(); err != nil {
			t.Errorf("close source archive gzip: %v", err)
		}
	}()
	reader := tar.NewReader(archive)
	expected := map[string]struct {
		body     string
		mode     int64
		typeflag byte
	}{
		"evalwitness-" + product.Version + "/":                 {mode: 0o755, typeflag: tar.TypeDir},
		"evalwitness-" + product.Version + "/cmd/":             {mode: 0o755, typeflag: tar.TypeDir},
		"evalwitness-" + product.Version + "/nested/":          {mode: 0o755, typeflag: tar.TypeDir},
		"evalwitness-" + product.Version + "/README.md":        {body: "hello\n", mode: 0o644, typeflag: tar.TypeReg},
		"evalwitness-" + product.Version + "/cmd/run.sh":       {body: "#!/bin/sh\nexit 0\n", mode: 0o755, typeflag: tar.TypeReg},
		"evalwitness-" + product.Version + "/nested/data.json": {body: "{}\n", mode: 0o644, typeflag: tar.TypeReg},
	}
	seen := make(map[string]struct{}, len(expected))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		want, found := expected[header.Name]
		if !found {
			t.Fatalf("unexpected archive entry %q", header.Name)
		}
		if header.Format != tar.FormatUSTAR || len(header.PAXRecords) != 0 || header.Linkname != "" || header.Typeflag != want.typeflag || header.Mode != want.mode || !header.ModTime.Equal(time.Unix(0, 0)) {
			t.Fatalf("archive entry %q is not canonical USTAR: %+v", header.Name, header)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want.body {
			t.Fatalf("archive entry %q body differs", header.Name)
		}
		seen[header.Name] = struct{}{}
	}
	if len(seen) != len(expected) {
		t.Fatalf("archive entry count = %d, want %d", len(seen), len(expected))
	}
	if _, err := CreateSourceArchive(context.Background(), repository, commit, firstPath); err == nil {
		t.Fatal("source archive builder overwrote an existing destination")
	}
}

func TestCreateSourceArchiveRejectsUnboundRepositoryState(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string, string) string
	}{
		{
			name: "dirty worktree",
			setup: func(t *testing.T, repository, commit string) string {
				t.Helper()
				if err := os.WriteFile(filepath.Join(repository, "untracked.txt"), []byte("dirty\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return commit
			},
		},
		{
			name: "commit mismatch",
			setup: func(t *testing.T, repository, commit string) string {
				t.Helper()
				return "0000000000000000000000000000000000000000"
			},
		},
		{
			name: "tracked symlink",
			setup: func(t *testing.T, repository, commit string) string {
				t.Helper()
				if runtime.GOOS == "windows" {
					t.Skip("Git symlink mode is not portable on Windows")
				}
				if err := os.Symlink("README.md", filepath.Join(repository, "linked-readme")); err != nil {
					t.Fatal(err)
				}
				runSourceArchiveGit(t, repository, "add", "linked-readme")
				runSourceArchiveGit(t, repository, "commit", "-m", "add symlink")
				return sourceArchiveGitOutput(t, repository, "rev-parse", "HEAD")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository, commit := createSourceArchiveRepository(t)
			commit = test.setup(t, repository, commit)
			destination := filepath.Join(t.TempDir(), ProductName+"-"+product.Version+"-source.tar.gz")
			if _, err := CreateSourceArchive(context.Background(), repository, commit, destination); err == nil {
				t.Fatal("source archive builder accepted an unbound repository state")
			}
			if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("rejected source archive request published an output")
			}
		})
	}
}

func TestCreateSourceArchiveAcceptsCaseVariantOfGitTopLevel(t *testing.T) {
	repository, commit := createSourceArchiveRepository(t)
	variant := strings.ToUpper(repository)
	if variant == repository {
		t.Skip("temporary repository path has no lowercase characters")
	}
	if _, err := os.Stat(variant); err != nil {
		t.Skip("filesystem is case-sensitive")
	}
	destination := filepath.Join(t.TempDir(), ProductName+"-"+product.Version+"-source.tar.gz")
	report, err := CreateSourceArchive(context.Background(), variant, commit, destination)
	if err != nil {
		t.Fatalf("create source archive through case-variant top level: %v", err)
	}
	if report.GitCommit != commit {
		t.Fatalf("source archive commit = %q, want %q", report.GitCommit, commit)
	}
}

func createSourceArchiveRepository(t *testing.T) (string, string) {
	t.Helper()
	repository := t.TempDir()
	runSourceArchiveGit(t, repository, "init", "--quiet")
	runSourceArchiveGit(t, repository, "config", "user.name", "EvalWitness Test")
	runSourceArchiveGit(t, repository, "config", "user.email", "test@evalwitness.invalid")
	files := map[string]struct {
		body string
		mode os.FileMode
	}{
		"README.md":        {body: "hello\n", mode: 0o644},
		"cmd/run.sh":       {body: "#!/bin/sh\nexit 0\n", mode: 0o755},
		"nested/data.json": {body: "{}\n", mode: 0o644},
	}
	for name, file := range files {
		path := filepath.Join(repository, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file.body), file.mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, file.mode); err != nil {
			t.Fatal(err)
		}
	}
	runSourceArchiveGit(t, repository, "add", ".")
	runSourceArchiveGit(t, repository, "commit", "--quiet", "-m", "fixture")
	return repository, sourceArchiveGitOutput(t, repository, "rev-parse", "HEAD")
}

func runSourceArchiveGit(t *testing.T, repository string, arguments ...string) {
	t.Helper()
	command := exec.CommandContext(context.Background(), "git", append([]string{"-C", repository}, arguments...)...)
	command.Env = append(os.Environ(), "LC_ALL=C", "LANG=C", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}

func sourceArchiveGitOutput(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	command := exec.CommandContext(context.Background(), "git", append([]string{"-C", repository}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace(output))
}
