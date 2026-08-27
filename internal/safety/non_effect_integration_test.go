package safety_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/cache"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

type protectedSnapshot struct {
	mode    os.FileMode
	entries []string
}

func TestRejectedDestructiveCommandsLeaveProtectedTargetsUnchanged(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repository := findTestRepositoryRoot(t, workingDirectory)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	targets := []string{string(filepath.Separator), home, repository, workingDirectory}
	snapshots := make(map[string]protectedSnapshot, len(targets))
	for _, target := range targets {
		snapshots[target] = snapshotProtectedTarget(t, target)
	}
	sentinel := filepath.Join(repository, "go.mod")
	sentinelBefore := hashFile(t, sentinel)

	for _, target := range targets {
		for _, scope := range []cache.ClearScope{cache.ClearResponses, cache.ClearCapabilities, cache.ClearAll} {
			if _, err := cache.New(target, true).Clear(scope); !safety.IsKind(err, safety.ErrorProtectedPath) {
				t.Fatalf("cache clear target=%q scope=%q error=%T %v", target, scope, err, err)
			}
			assertProtectedSnapshot(t, target, snapshots[target])
			assertFileHash(t, sentinel, sentinelBefore)
		}
	}

	archive := createSafeArchive(t)
	for _, target := range targets {
		_, err := safety.ExtractTarGzip(context.Background(), safety.ArchiveExtractRequest{
			Sources: []string{archive}, Destination: target, ExpectedRoots: []string{"root"}, Limits: safety.DefaultArchiveLimits(),
		})
		if !safety.IsKind(err, safety.ErrorProtectedPath) {
			t.Fatalf("archive extract target=%q error=%T %v", target, err, err)
		}
		assertProtectedSnapshot(t, target, snapshots[target])
		assertFileHash(t, sentinel, sentinelBefore)
	}
}

func findTestRepositoryRoot(t *testing.T, start string) string {
	t.Helper()
	if portableRoot := os.Getenv("EVALWITNESS_REPOSITORY_ROOT"); portableRoot != "" {
		resolved, err := filepath.Abs(portableRoot)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(resolved)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("portable repository root is not a real directory")
		}
		return resolved
	}
	current := filepath.Clean(start)
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatal("repository root not found")
		}
		current = parent
	}
}

func snapshotProtectedTarget(t *testing.T, path string) protectedSnapshot {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name()
	}
	sort.Strings(names)
	return protectedSnapshot{mode: info.Mode(), entries: names}
}

func assertProtectedSnapshot(t *testing.T, path string, want protectedSnapshot) {
	t.Helper()
	got := snapshotProtectedTarget(t, path)
	if got.mode != want.mode || !reflect.DeepEqual(got.entries, want.entries) {
		t.Fatalf("protected target changed: %q", path)
	}
}

func hashFile(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(raw)
}

func assertFileHash(t *testing.T, path string, want [sha256.Size]byte) {
	t.Helper()
	if got := hashFile(t, path); got != want {
		t.Fatalf("sentinel changed: %q", path)
	}
}

func createSafeArchive(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "safe.tar.gz")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, safety.SensitiveFileMode)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	payload := []byte("safe")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "root/file.txt", Mode: 0o600, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
