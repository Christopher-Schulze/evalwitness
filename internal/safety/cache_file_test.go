package safety

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestCacheRootPublishesAndReadsSensitiveFile(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	relative := filepath.Join("routes", "route-id", "identity.json")
	if err := root.PublishSensitive(relative, []byte(`{"provider":"p"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := root.ReadSensitive(relative, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"provider":"p"}` {
		t.Fatalf("read = %q", got)
	}
	info, err := os.Stat(filepath.Join(root.Path(), relative))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != SensitiveFileMode {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestCacheRootPublishIsAtomicUnderConcurrentWriters(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	relative := filepath.Join("routes", "route-id", "response.json")
	payloads := [][]byte{
		bytes.Repeat([]byte("a"), 32*1024),
		bytes.Repeat([]byte("b"), 32*1024),
		bytes.Repeat([]byte("c"), 32*1024),
		bytes.Repeat([]byte("d"), 32*1024),
	}
	var group sync.WaitGroup
	errorsFound := make(chan error, len(payloads))
	for _, payload := range payloads {
		group.Add(1)
		go func() {
			defer group.Done()
			errorsFound <- root.PublishSensitive(relative, payload)
		}()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := root.ReadSensitive(relative, 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, payload := range payloads {
		matched = matched || bytes.Equal(got, payload)
	}
	if !matched {
		t.Fatal("published file is a partial or mixed writer payload")
	}
	entries, err := os.ReadDir(filepath.Dir(filepath.Join(root.Path(), relative)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file remained: %q", entry.Name())
		}
	}
}

func TestCacheRootPublishSensitiveExclusiveNeverOverwrites(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	const relative = "private-mapping.json"
	if err := root.PublishSensitiveExclusive(relative, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := root.PublishSensitiveExclusive(relative, []byte("second")); err == nil {
		t.Fatal("exclusive sensitive publish overwrote an existing file")
	}
	data, err := root.ReadSensitive(relative, 64)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first" {
		t.Fatalf("exclusive sensitive publish changed existing data: %q", data)
	}
}

func TestCacheRootFileOperationsRejectEscapes(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"", ".", "..", filepath.Join("..", "escape"), string(filepath.Separator) + "absolute"} {
		if err := root.PublishSensitive(relative, []byte("x")); err == nil {
			t.Fatalf("publish accepted %q", relative)
		}
		if _, err := root.ReadSensitive(relative, 1); err == nil {
			t.Fatalf("read accepted %q", relative)
		}
	}
}

func TestCacheRootFileOperationsRejectEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permission model differs")
	}
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "outside")
	if err := os.MkdirAll(outside, SensitiveDirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root.Path(), "routes")); err != nil {
		t.Fatal(err)
	}
	err = root.PublishSensitive(filepath.Join("routes", "entry.json"), []byte("x"))
	if !IsKind(err, ErrorContainmentViolation) {
		t.Fatalf("error = %T %v, want containment violation", err, err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "entry.json")); !os.IsNotExist(statErr) {
		t.Fatalf("escaping write reached outside directory: %v", statErr)
	}
}

func TestCacheRootReadEnforcesBoundsAndPermissions(t *testing.T) {
	policy, home, _, _ := testPathPolicy(t)
	root, err := CreateCacheRoot(policy, filepath.Join(home, ".cache", ProductID))
	if err != nil {
		t.Fatal(err)
	}
	relative := filepath.Join("routes", "large.json")
	if err := root.PublishSensitive(relative, bytes.Repeat([]byte("x"), 32)); err != nil {
		t.Fatal(err)
	}
	if _, err := root.ReadSensitive(relative, 16); !IsKind(err, ErrorResourceLimit) {
		t.Fatalf("bound error = %T %v", err, err)
	}
	if err := os.Chmod(filepath.Join(root.Path(), relative), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := root.ReadSensitive(relative, 64); !IsKind(err, ErrorUnsafePermissions) {
		t.Fatalf("permission error = %T %v", err, err)
	}
}
