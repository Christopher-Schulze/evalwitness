// Package safety defines the fail-closed vocabulary shared by filesystem,
// archive, logging, and artifact-publication boundaries.
package safety

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
)

const (
	ProductID              = "evalwitness"
	CacheRootMarkerName    = ".evalwitness-cache-root.json"
	CacheMarkerSchema      = 1
	SensitiveFileMode      = fs.FileMode(0o600)
	SensitiveDirectoryMode = fs.FileMode(0o700)
	PublicFileMode         = fs.FileMode(0o644)
	PublicDirectoryMode    = fs.FileMode(0o755)
)

type Operation string

const (
	OperationValidate Operation = "validate"
	OperationCreate   Operation = "create"
	OperationRead     Operation = "read"
	OperationWrite    Operation = "write"
	OperationDelete   Operation = "delete"
	OperationExtract  Operation = "extract"
	OperationPublish  Operation = "publish"
	OperationScan     Operation = "scan"
)

func (o Operation) Valid() bool {
	switch o {
	case OperationValidate, OperationCreate, OperationRead, OperationWrite,
		OperationDelete, OperationExtract, OperationPublish, OperationScan:
		return true
	default:
		return false
	}
}

type ErrorKind string

const (
	ErrorInvalidInput            ErrorKind = "invalid_input"
	ErrorProtectedPath           ErrorKind = "protected_path"
	ErrorContainmentViolation    ErrorKind = "containment_violation"
	ErrorUnownedRoot             ErrorKind = "unowned_root"
	ErrorUnsafePermissions       ErrorKind = "unsafe_permissions"
	ErrorSymlinkRace             ErrorKind = "symlink_race"
	ErrorUnsupportedFileType     ErrorKind = "unsupported_file_type"
	ErrorNameCollision           ErrorKind = "name_collision"
	ErrorResourceLimit           ErrorKind = "resource_limit"
	ErrorInsufficientSpace       ErrorKind = "insufficient_space"
	ErrorSecretDetected          ErrorKind = "secret_detected"
	ErrorArtifactPolicyViolation ErrorKind = "artifact_policy_violation"
	ErrorUntrustedControl        ErrorKind = "untrusted_control"
	ErrorConcurrentMutation      ErrorKind = "concurrent_mutation"
)

func (k ErrorKind) Valid() bool {
	switch k {
	case ErrorInvalidInput, ErrorProtectedPath, ErrorContainmentViolation,
		ErrorUnownedRoot, ErrorUnsafePermissions, ErrorSymlinkRace,
		ErrorUnsupportedFileType, ErrorNameCollision, ErrorResourceLimit,
		ErrorInsufficientSpace, ErrorSecretDetected, ErrorArtifactPolicyViolation,
		ErrorUntrustedControl, ErrorConcurrentMutation:
		return true
	default:
		return false
	}
}

type Error struct {
	Kind      ErrorKind
	Operation Operation
	Path      string
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := fmt.Sprintf("safety %s failed: %s", e.Operation, e.Kind)
	if e.Path != "" {
		message += fmt.Sprintf(" at %q", e.Path)
	}
	return message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsKind(err error, kind ErrorKind) bool {
	var safetyError *Error
	return errors.As(err, &safetyError) && safetyError.Kind == kind
}

type ProtectedLocation string

const (
	ProtectedFilesystemRoot ProtectedLocation = "filesystem_root"
	ProtectedVolumeRoot     ProtectedLocation = "volume_root"
	ProtectedUserHome       ProtectedLocation = "user_home"
	ProtectedRepositoryRoot ProtectedLocation = "repository_root"
	ProtectedWorkingDir     ProtectedLocation = "working_directory"
)

func (p ProtectedLocation) Valid() bool {
	switch p {
	case ProtectedFilesystemRoot, ProtectedVolumeRoot, ProtectedUserHome,
		ProtectedRepositoryRoot, ProtectedWorkingDir:
		return true
	default:
		return false
	}
}

type ProtectedPath struct {
	Location ProtectedLocation
	Path     string
}

type ArtifactClass string

const (
	ArtifactSensitive ArtifactClass = "sensitive"
	ArtifactPublic    ArtifactClass = "public"
)

func (c ArtifactClass) Valid() bool {
	return c == ArtifactSensitive || c == ArtifactPublic
}

func (c ArtifactClass) FileMode() (fs.FileMode, error) {
	switch c {
	case ArtifactSensitive:
		return SensitiveFileMode, nil
	case ArtifactPublic:
		return PublicFileMode, nil
	default:
		return 0, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
}

func (c ArtifactClass) DirectoryMode() (fs.FileMode, error) {
	switch c {
	case ArtifactSensitive:
		return SensitiveDirectoryMode, nil
	case ArtifactPublic:
		return PublicDirectoryMode, nil
	default:
		return 0, &Error{Kind: ErrorInvalidInput, Operation: OperationValidate}
	}
}

type CacheRootMarker struct {
	SchemaVersion int    `json:"schema_version"`
	Product       string `json:"product"`
	RootID        string `json:"root_id"`
}

func (m CacheRootMarker) Valid() bool {
	if m.SchemaVersion != CacheMarkerSchema || m.Product != ProductID || len(m.RootID) != 32 {
		return false
	}
	_, err := hex.DecodeString(m.RootID)
	return err == nil
}
