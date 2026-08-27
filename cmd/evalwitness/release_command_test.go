package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseSignatureDirectoryPublishesExactNoOverwriteSet(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "signature")
	envelope := []byte(`{"envelope":true}`)
	root := []byte(`{"root":true}`)
	policy := []byte(`{"policy":true}`)
	if err := writeReleaseSignatureDirectory(destination, envelope, root, policy); err != nil {
		t.Fatal(err)
	}
	actualEnvelope, actualRoot, actualPolicy, err := readReleaseSignatureDirectory(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualEnvelope) != string(envelope) || string(actualRoot) != string(root) || string(actualPolicy) != string(policy) {
		t.Fatal("release signature directory changed material ordering or bytes")
	}
	if err := writeReleaseSignatureDirectory(destination, []byte("replacement"), root, policy); err == nil {
		t.Fatal("release signature directory overwrote an existing destination")
	}
	after, err := os.ReadFile(filepath.Join(destination, "envelope.dsse.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(envelope) {
		t.Fatal("rejected signature publication mutated existing material")
	}
}

func TestReleaseSignatureDirectoryRejectsAdditionalFiles(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "signature")
	if err := writeReleaseSignatureDirectory(destination, []byte("envelope"), []byte("root"), []byte("policy")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "unexpected.json"), []byte("unexpected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readReleaseSignatureDirectory(destination); err == nil {
		t.Fatal("release signature reader accepted an additional file")
	}
}

func TestReleasePrivateKeyReaderRequiresMode0600AndRejectsLinks(t *testing.T) {
	root := t.TempDir()
	keyPath := filepath.Join(root, "key")
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleasePrivateKey(keyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleasePrivateKey(keyPath); err == nil {
		t.Fatal("release private-key reader accepted mode 0644")
	}
	linkPath := filepath.Join(root, "linked-key")
	if err := os.Symlink(keyPath, linkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleasePrivateKey(linkPath); err == nil {
		t.Fatal("release private-key reader accepted a symlink")
	}
}
