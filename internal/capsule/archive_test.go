package capsule

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestDeterministicArchiveRoundTrip(t *testing.T) {
	registry, manifest, payloads := testPublicPackage(t)
	source := filepath.Join(t.TempDir(), "capsule")
	if err := WriteDirectory(context.Background(), source, registry, manifest, payloads); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(t.TempDir(), "first.tar.gz")
	secondPath := filepath.Join(t.TempDir(), "second.tar.gz")
	options := VerificationOptions{MaximumVisibility: VisibilityPublic}
	first, err := CreateArchive(context.Background(), source, firstPath, registry, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateArchive(context.Background(), source, secondPath, registry, options)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) != string(secondBytes) || first.SHA256 != second.SHA256 || first.Bytes != second.Bytes {
		t.Fatal("canonical capsule archives are not byte-identical")
	}
	if first.CapsuleID != manifest.CapsuleID || !first.Deterministic || first.Files != 7 {
		t.Fatalf("archive report = %+v", first)
	}

	extracted := filepath.Join(t.TempDir(), "extracted")
	if _, err := safety.ExtractTarGzip(context.Background(), safety.ArchiveExtractRequest{
		Sources: []string{firstPath}, Destination: extracted, ExpectedRoots: []string{first.ArchiveRoot},
		Limits: safety.DefaultArchiveLimits(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(context.Background(), filepath.Join(extracted, first.ArchiveRoot), registry, options); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveRefusesOverwrite(t *testing.T) {
	registry, manifest, payloads := testPublicPackage(t)
	source := filepath.Join(t.TempDir(), "capsule")
	if err := WriteDirectory(context.Background(), source, registry, manifest, payloads); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "capsule.tar.gz")
	if _, err := CreateArchive(context.Background(), source, destination, registry, VerificationOptions{}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateArchive(context.Background(), source, destination, registry, VerificationOptions{}); err == nil {
		t.Fatal("archive creation overwrote an existing target")
	}
	after, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed archive overwrite changed the existing target")
	}
}

func TestPrivateArchiveUsesPrivateMemberModes(t *testing.T) {
	registry := testRegistry(t)
	secret, raw := buildTestComponent(t, registry, ComponentInput{
		Name: "secret", TypeID: testSecretType, Visibility: VisibilityPrivate, Payload: []byte("private bytes"),
	})
	manifest := buildTestManifest(t, registry, []ComponentRecord{secret}, secret.ComponentID, "")
	source := filepath.Join(t.TempDir(), "capsule")
	if err := WriteDirectory(context.Background(), source, registry, manifest, map[string][]byte{secret.Payload.Digest: raw}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "private.tar.gz")
	if _, err := CreateArchive(context.Background(), source, destination, registry, VerificationOptions{MaximumVisibility: VisibilityPrivate}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(gzipReader)
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		want := int64(safety.SensitiveFileMode)
		if header.Typeflag == tar.TypeDir {
			want = int64(safety.SensitiveDirectoryMode)
		}
		if header.Mode != want {
			t.Fatalf("archive member %q mode = %o, want %o", header.Name, header.Mode, want)
		}
	}
	if err := gzipReader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
