package safety

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

func TestClosedVocabularyRejectsUnknownValues(t *testing.T) {
	operations := []Operation{
		OperationValidate, OperationCreate, OperationRead, OperationWrite,
		OperationDelete, OperationExtract, OperationPublish, OperationScan,
	}
	for _, operation := range operations {
		if !operation.Valid() {
			t.Fatalf("operation %q is not valid", operation)
		}
	}
	if Operation("execute").Valid() {
		t.Fatal("untrusted execute operation entered the safety vocabulary")
	}

	kinds := []ErrorKind{
		ErrorInvalidInput, ErrorProtectedPath, ErrorContainmentViolation,
		ErrorUnownedRoot, ErrorUnsafePermissions, ErrorSymlinkRace,
		ErrorUnsupportedFileType, ErrorNameCollision, ErrorResourceLimit,
		ErrorInsufficientSpace, ErrorSecretDetected, ErrorArtifactPolicyViolation,
		ErrorUntrustedControl, ErrorConcurrentMutation,
	}
	for _, kind := range kinds {
		if !kind.Valid() {
			t.Fatalf("error kind %q is not valid", kind)
		}
	}
	if ErrorKind("best_effort").Valid() {
		t.Fatal("best-effort behavior entered the fail-closed error taxonomy")
	}

	locations := []ProtectedLocation{
		ProtectedFilesystemRoot, ProtectedVolumeRoot, ProtectedUserHome,
		ProtectedRepositoryRoot, ProtectedWorkingDir,
	}
	for _, location := range locations {
		if !location.Valid() {
			t.Fatalf("protected location %q is not valid", location)
		}
	}
	if ProtectedLocation("cache").Valid() {
		t.Fatal("an owned cache root was classified as inherently protected")
	}
}

func TestSafetyErrorIsTypedWithoutEchoingCause(t *testing.T) {
	const secret = "credential-value-that-must-not-be-printed"
	cause := fmt.Errorf("provider returned %s", secret)
	err := &Error{
		Kind:      ErrorSecretDetected,
		Operation: OperationScan,
		Path:      "/tmp/report.json",
		Cause:     cause,
	}

	if !IsKind(err, ErrorSecretDetected) {
		t.Fatalf("IsKind did not recognize %T", err)
	}
	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause is unavailable to in-process callers")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error disclosed its wrapped cause: %s", err)
	}
	for _, want := range []string{"scan", "secret_detected", "/tmp/report.json"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q omits %q", err, want)
		}
	}
}

func TestArtifactClassesRequireExplicitPublicClassification(t *testing.T) {
	tests := []struct {
		class         ArtifactClass
		wantFile      fs.FileMode
		wantDirectory fs.FileMode
	}{
		{ArtifactSensitive, 0o600, 0o700},
		{ArtifactPublic, 0o644, 0o755},
	}
	for _, test := range tests {
		t.Run(string(test.class), func(t *testing.T) {
			fileMode, err := test.class.FileMode()
			if err != nil {
				t.Fatal(err)
			}
			directoryMode, err := test.class.DirectoryMode()
			if err != nil {
				t.Fatal(err)
			}
			if fileMode != test.wantFile || directoryMode != test.wantDirectory {
				t.Fatalf("modes = %o/%o, want %o/%o", fileMode, directoryMode, test.wantFile, test.wantDirectory)
			}
		})
	}

	unknown := ArtifactClass("")
	if unknown.Valid() {
		t.Fatal("empty artifact class is valid")
	}
	if _, err := unknown.FileMode(); !IsKind(err, ErrorInvalidInput) {
		t.Fatalf("unknown file class error = %T %v", err, err)
	}
	if _, err := unknown.DirectoryMode(); !IsKind(err, ErrorInvalidInput) {
		t.Fatalf("unknown directory class error = %T %v", err, err)
	}
}

func TestCacheRootMarkerRequiresCanonicalIdentityAndRandomID(t *testing.T) {
	valid := CacheRootMarker{SchemaVersion: CacheMarkerSchema, Product: ProductID, RootID: "1f25fa29d1a24ba91f25fa29d1a24ba9"}
	if !valid.Valid() {
		t.Fatal("canonical marker is invalid")
	}

	invalid := []CacheRootMarker{
		{},
		{SchemaVersion: CacheMarkerSchema, Product: ProductID},
		{SchemaVersion: CacheMarkerSchema + 1, Product: ProductID, RootID: valid.RootID},
		{SchemaVersion: CacheMarkerSchema, Product: "logprobe", RootID: valid.RootID},
	}
	for _, marker := range invalid {
		if marker.Valid() {
			t.Fatalf("invalid marker passed: %+v", marker)
		}
	}
}
