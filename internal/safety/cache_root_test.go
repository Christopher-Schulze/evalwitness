package safety

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestCreateAndOpenCacheRoot(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	path := filepath.Join(home, ".cache", ProductID)
	created, err := CreateCacheRoot(policy, path)
	if err != nil {
		t.Fatal(err)
	}
	canonicalPath, err := resolveWithExistingParents(path)
	if err != nil {
		t.Fatal(err)
	}
	if created.Path() != canonicalPath || created.RootID() == "" {
		t.Fatalf("root = path %q id %q", created.Path(), created.RootID())
	}

	rootInfo, err := os.Stat(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if rootInfo.Mode().Perm() != SensitiveDirectoryMode {
		t.Fatalf("root mode = %o", rootInfo.Mode().Perm())
	}
	markerInfo, err := os.Stat(filepath.Join(canonicalPath, CacheRootMarkerName))
	if err != nil {
		t.Fatal(err)
	}
	if markerInfo.Mode().Perm() != SensitiveFileMode {
		t.Fatalf("marker mode = %o", markerInfo.Mode().Perm())
	}

	opened, err := OpenCacheRoot(policy, path)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Path() != created.Path() || opened.RootID() != created.RootID() {
		t.Fatalf("opened root = %+v, created = %+v", opened, created)
	}
}

func TestCreateCacheRootIsIdempotentForOwnedRoot(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	path := filepath.Join(home, ".cache", ProductID)
	first, err := CreateCacheRoot(policy, path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateCacheRoot(policy, path)
	if err != nil {
		t.Fatal(err)
	}
	if first.RootID() != second.RootID() {
		t.Fatalf("idempotent create changed root ID: %q != %q", first.RootID(), second.RootID())
	}
}

func TestCreateCacheRootRefusesUnmarkedNonEmptyDirectory(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	path := filepath.Join(home, "existing")
	if err := os.MkdirAll(path, SensitiveDirectoryMode); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(path, "do-not-delete")
	if err := os.WriteFile(sentinel, []byte("sentinel"), SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCacheRoot(policy, path); !IsKind(err, ErrorUnownedRoot) {
		t.Fatalf("error = %T %v, want unowned root", err, err)
	}
	raw, err := os.ReadFile(sentinel)
	if err != nil || string(raw) != "sentinel" {
		t.Fatalf("sentinel changed: %q, %v", raw, err)
	}
}

func TestCreateCacheRootSecuresEmptyExistingDirectory(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	path := filepath.Join(home, "empty-existing")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCacheRoot(policy, path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != SensitiveDirectoryMode {
		t.Fatalf("secured mode = %o", info.Mode().Perm())
	}
}

func TestOpenCacheRootRejectsMarkerIdentityAndPermissions(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	tests := []struct {
		name   string
		mutate func(t *testing.T, root string)
		kind   ErrorKind
	}{
		{
			name: "legacy product",
			mutate: func(t *testing.T, root string) {
				writeMarkerForTest(t, root, CacheRootMarker{SchemaVersion: CacheMarkerSchema, Product: "logprobe", RootID: "0123456789abcdef0123456789abcdef"})
			},
			kind: ErrorUnownedRoot,
		},
		{
			name: "unsafe root mode",
			mutate: func(t *testing.T, root string) {
				if err := os.Chmod(root, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			kind: ErrorUnsafePermissions,
		},
		{
			name: "unsafe marker mode",
			mutate: func(t *testing.T, root string) {
				if err := os.Chmod(filepath.Join(root, CacheRootMarkerName), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			kind: ErrorUnsafePermissions,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(home, test.name)
			if _, err := CreateCacheRoot(policy, root); err != nil {
				t.Fatal(err)
			}
			test.mutate(t, root)
			if _, err := OpenCacheRoot(policy, root); !IsKind(err, test.kind) {
				t.Fatalf("error = %T %v, want %s", err, err, test.kind)
			}
		})
	}
}

func TestOpenCacheRootRejectsSymlinkMarker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permission model differs")
	}
	policy, home, _, _ := testPathPolicy(t)
	root := filepath.Join(home, "symlink-marker")
	if err := os.MkdirAll(root, SensitiveDirectoryMode); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "marker-target")
	writeMarkerForTest(t, home, CacheRootMarker{SchemaVersion: CacheMarkerSchema, Product: ProductID, RootID: "0123456789abcdef0123456789abcdef"})
	if err := os.Rename(filepath.Join(home, CacheRootMarkerName), target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, CacheRootMarkerName)); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenCacheRoot(policy, root); !IsKind(err, ErrorUnsupportedFileType) {
		t.Fatalf("error = %T %v, want unsupported file type", err, err)
	}
}

func TestCacheRootResolveRejectsEscapesAndSymlinkParents(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root.Path(), "responses", "entry.json")
	got, err := root.Resolve(filepath.Join("responses", "entry.json"))
	if err != nil || got != want {
		t.Fatalf("resolve = %q, %v, want %q", got, err, want)
	}
	if _, err := root.Resolve(filepath.Join("..", "escape")); !IsKind(err, ErrorContainmentViolation) {
		t.Fatalf("parent escape error = %T %v", err, err)
	}

	outside := filepath.Join(home, "outside")
	if err := os.MkdirAll(outside, SensitiveDirectoryMode); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root.Path(), "link")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if _, err := root.Resolve(filepath.Join("link", "entry")); !IsKind(err, ErrorContainmentViolation) {
		t.Fatalf("symlink escape error = %T %v", err, err)
	}
}

func TestConcurrentCacheRootCreationProducesOneIdentity(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	path := filepath.Join(home, ".cache", ProductID)
	const workers = 16
	var group sync.WaitGroup
	results := make(chan *CacheRoot, workers)
	errorsFound := make(chan error, workers)
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			root, err := CreateCacheRoot(policy, path)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- root
		}()
	}
	group.Wait()
	close(results)
	close(errorsFound)

	opened, err := OpenCacheRoot(policy, path)
	if err != nil {
		t.Fatal(err)
	}
	for result := range results {
		if result.RootID() != opened.RootID() {
			t.Fatalf("created root ID %q != committed %q", result.RootID(), opened.RootID())
		}
	}
	for err := range errorsFound {
		if !IsKind(err, ErrorConcurrentMutation) {
			t.Fatalf("unexpected concurrent error: %T %v", err, err)
		}
	}
}

func writeMarkerForTest(t *testing.T, root string, marker CacheRootMarker) {
	t.Helper()
	raw, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, CacheRootMarkerName), raw, SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
}
