package safety

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testPathPolicy(t *testing.T) (*PathPolicy, string, string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repository := filepath.Join(home, "code", "project")
	working := filepath.Join(repository, "internal")
	for _, path := range []string{home, repository, working} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := NewPathPolicy(PathPolicyOptions{UserHome: home, RepositoryRoot: repository, WorkingDir: working})
	if err != nil {
		t.Fatal(err)
	}
	return policy, home, repository, working
}

func TestPathPolicyRejectsProtectedLocationsAndTheirParents(t *testing.T) {
	policy, home, repository, working := testPathPolicy(t)
	root := filepath.Dir(home)
	tests := []string{
		string(filepath.Separator),
		root,
		home,
		repository,
		working,
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			_, err := policy.ValidateCacheRoot(path)
			if !IsKind(err, ErrorProtectedPath) {
				t.Fatalf("error = %T %v, want protected path", err, err)
			}
		})
	}
}

func TestPathPolicyAllowsStrictDescendantCacheRoots(t *testing.T) {
	policy, home, repository, _ := testPathPolicy(t)
	for _, path := range []string{
		filepath.Join(home, ".cache", ProductID),
		filepath.Join(repository, ".cache", ProductID),
	} {
		got, err := policy.ValidateCacheRoot(path)
		if err != nil {
			t.Fatalf("validate %q: %v", path, err)
		}
		if !filepath.IsAbs(got) || filepath.Base(got) != ProductID {
			t.Fatalf("canonical path = %q", got)
		}
	}
}

func TestPathPolicyResolvesExistingParentSymlinks(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	linkParent := filepath.Join(filepath.Dir(home), "home-link")
	if err := os.Symlink(home, linkParent); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Fatal(err)
	}

	got, err := policy.ValidateCacheRoot(filepath.Join(linkParent, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	want, err := resolveWithExistingParents(filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("canonical path = %q, want %q", got, want)
	}
	if _, err := policy.ValidateCacheRoot(linkParent); !IsKind(err, ErrorProtectedPath) {
		t.Fatalf("symlink to home error = %T %v", err, err)
	}
}

func TestPathPolicyRejectsInvalidInputs(t *testing.T) {
	policy, _, _, _ := testPathPolicy(t)
	if _, err := policy.ValidateCacheRoot(""); !IsKind(err, ErrorInvalidInput) {
		t.Fatalf("empty path error = %T %v", err, err)
	}
	var nilPolicy *PathPolicy
	if _, err := nilPolicy.ValidateCacheRoot("cache"); !IsKind(err, ErrorInvalidInput) {
		t.Fatalf("nil policy error = %T %v", err, err)
	}
	if _, err := NewPathPolicy(PathPolicyOptions{}); !IsKind(err, ErrorInvalidInput) {
		t.Fatalf("empty options error = %T %v", err, err)
	}
}

func TestContainsOrEqualUsesComponentBoundaries(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "root")
	if !containsOrEqual(root, root) || !containsOrEqual(root, filepath.Join(root, "child")) {
		t.Fatal("equal or child path was not contained")
	}
	if containsOrEqual(root, root+"-other") || containsOrEqual(root, filepath.Dir(root)) {
		t.Fatal("sibling or parent path was treated as contained")
	}
}

func TestDarwinVolumeRootsAreProtected(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin volume-root convention")
	}
	if !isVolumeRoot("/Volumes/External") {
		t.Fatal("direct volume mount was not recognized")
	}
	if isVolumeRoot("/Volumes/External/cache") {
		t.Fatal("volume descendant was classified as the volume root")
	}
}

func TestResolveWithExistingParentsReturnsUnderlyingErrors(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := resolveWithExistingParents(filepath.Join(file, "child"))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-directory parent error = %v", err)
	}
}
