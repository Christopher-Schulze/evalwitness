package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestCapsuleArtifactGroupPublishesWithoutOverwriteOrResidue(t *testing.T) {
	pack, err := capsule.BuildReferencePackage(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	destination := filepath.Join(root, "capsule")
	archive := filepath.Join(root, "capsule.tar.gz")
	sidecar := filepath.Join(root, "capsule.claims.json")
	report, err := publishCapsuleArtifacts(
		context.Background(), destination, archive, pack.Registry, pack.Manifest, pack.Payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
		[]capsuleSidecar{{Destination: sidecar, Payload: []byte("{}")}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Deterministic || report.Files != 74 {
		t.Fatalf("capsule archive report = %+v", report)
	}
	if _, err := capsule.VerifyDirectory(
		context.Background(), destination, pack.Registry,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
	); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{archive, sidecar} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != safety.PublicFileMode {
			t.Fatalf("published file %q mode or identity is invalid: %v, %v", path, info, err)
		}
	}
	residue, err := filepath.Glob(filepath.Join(root, ".*.transaction-*"))
	if err != nil || len(residue) != 0 {
		t.Fatalf("capsule transaction residue = %v, %v", residue, err)
	}

	if _, err := publishCapsuleArtifacts(
		context.Background(), destination, archive, pack.Registry, pack.Manifest, pack.Payloads,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}, nil,
	); err == nil {
		t.Fatal("capsule artifact group overwrote existing targets")
	}
	if _, err := capsule.VerifyDirectory(
		context.Background(), destination, pack.Registry,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
	); err != nil {
		t.Fatalf("failed no-overwrite publication changed the existing capsule: %v", err)
	}
}

func TestCapsuleArtifactGroupRejectsIncompleteSidecarBeforePublication(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "capsule")
	archive := filepath.Join(root, "capsule.tar.gz")
	_, err := publishCapsuleArtifacts(
		context.Background(), destination, archive, &capsule.Registry{}, capsule.Manifest{}, nil,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
		[]capsuleSidecar{{Destination: filepath.Join(root, "empty.json")}},
	)
	if err == nil {
		t.Fatal("capsule artifact group accepted an incomplete publication")
	}
	for _, path := range []string{destination, archive, filepath.Join(root, "empty.json")} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("failed capsule publication left target %q", path)
		}
	}
}
