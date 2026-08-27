package safety

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxCacheMarkerBytes = 4096

type CacheRoot struct {
	path   string
	rootID string
}

func (r *CacheRoot) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

func (r *CacheRoot) RootID() string {
	if r == nil {
		return ""
	}
	return r.rootID
}

func CreateCacheRoot(policy *PathPolicy, path string) (*CacheRoot, error) {
	canonical, err := policy.ValidateCacheRoot(path)
	if err != nil {
		return nil, err
	}
	_, err = ensureSensitiveDirectory(canonical)
	if err != nil {
		return nil, err
	}
	if root, openErr := OpenCacheRoot(policy, canonical); openErr == nil {
		return root, nil
	} else if !IsKind(openErr, ErrorUnownedRoot) {
		return nil, openErr
	}
	allowed, initializing, err := cacheRootInitializationState(canonical)
	if err != nil {
		return nil, &Error{Kind: ErrorUnownedRoot, Operation: OperationCreate, Path: canonical, Cause: err}
	}
	if !allowed {
		if root, openErr := OpenCacheRoot(policy, canonical); openErr == nil {
			return root, nil
		}
		return nil, &Error{Kind: ErrorUnownedRoot, Operation: OperationCreate, Path: canonical}
	}
	if initializing {
		return nil, &Error{Kind: ErrorConcurrentMutation, Operation: OperationCreate, Path: canonical}
	}
	marker, err := newCacheRootMarker()
	if err != nil {
		return nil, &Error{Kind: ErrorConcurrentMutation, Operation: OperationCreate, Path: canonical, Cause: err}
	}
	marker, err = publishCacheRootMarker(canonical, marker)
	if err != nil {
		return nil, err
	}
	return &CacheRoot{path: canonical, rootID: marker.RootID}, nil
}

func OpenCacheRoot(policy *PathPolicy, path string) (*CacheRoot, error) {
	canonical, err := policy.ValidateCacheRoot(path)
	if err != nil {
		return nil, err
	}
	if err := validateSensitiveDirectory(canonical); err != nil {
		return nil, err
	}
	marker, err := readCacheRootMarker(canonical)
	if err != nil {
		return nil, err
	}
	return &CacheRoot{path: canonical, rootID: marker.RootID}, nil
}

func (r *CacheRoot) Resolve(relative string) (string, error) {
	if r == nil || relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) == "." {
		return "", &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: relative}
	}
	candidate, err := resolveWithExistingParents(filepath.Join(r.path, relative))
	if err != nil {
		return "", &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: relative, Cause: err}
	}
	if candidate == r.path || !containsOrEqual(r.path, candidate) {
		return "", &Error{Kind: ErrorContainmentViolation, Operation: OperationValidate, Path: candidate}
	}
	return candidate, nil
}

func ensureSensitiveDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, SensitiveDirectoryMode); err != nil {
			return false, &Error{Kind: ErrorInvalidInput, Operation: OperationCreate, Path: path, Cause: err}
		}
		if err := os.Chmod(path, SensitiveDirectoryMode); err != nil {
			return false, &Error{Kind: ErrorUnsafePermissions, Operation: OperationCreate, Path: path, Cause: err}
		}
		return true, nil
	}
	if err != nil {
		return false, &Error{Kind: ErrorInvalidInput, Operation: OperationCreate, Path: path, Cause: err}
	}
	if !info.IsDir() {
		return false, &Error{Kind: ErrorUnsupportedFileType, Operation: OperationCreate, Path: path}
	}
	if info.Mode().Perm()&0o077 != 0 {
		empty, emptyErr := directoryContainsOnly(path, "")
		if emptyErr != nil || !empty {
			return false, &Error{Kind: ErrorUnsafePermissions, Operation: OperationCreate, Path: path, Cause: emptyErr}
		}
		if err := os.Chmod(path, SensitiveDirectoryMode); err != nil {
			return false, &Error{Kind: ErrorUnsafePermissions, Operation: OperationCreate, Path: path, Cause: err}
		}
	}
	return false, nil
}

func validateSensitiveDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return &Error{Kind: ErrorUnownedRoot, Operation: OperationRead, Path: path, Cause: err}
	}
	if !info.IsDir() {
		return &Error{Kind: ErrorUnsupportedFileType, Operation: OperationRead, Path: path}
	}
	if info.Mode().Perm()&0o077 != 0 {
		return &Error{Kind: ErrorUnsafePermissions, Operation: OperationRead, Path: path}
	}
	return nil
}

func directoryContainsOnly(path, allowedName string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Name() != allowedName {
			return false, nil
		}
	}
	return true, nil
}

func cacheRootInitializationState(path string) (bool, bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, false, err
	}
	lockName := CacheRootMarkerName + ".lock"
	temporaryPrefix := "." + CacheRootMarkerName + ".tmp-"
	initializing := false
	for _, entry := range entries {
		if entry.Name() == lockName || strings.HasPrefix(entry.Name(), temporaryPrefix) {
			initializing = true
			continue
		}
		return false, initializing, nil
	}
	return true, initializing, nil
}

func newCacheRootMarker() (CacheRootMarker, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return CacheRootMarker{}, err
	}
	return CacheRootMarker{
		SchemaVersion: CacheMarkerSchema,
		Product:       ProductID,
		RootID:        hex.EncodeToString(random),
	}, nil
}

func publishCacheRootMarker(root string, marker CacheRootMarker) (result CacheRootMarker, returnErr error) {
	lockPath := filepath.Join(root, CacheRootMarkerName+".lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, SensitiveFileMode)
	if err != nil {
		return CacheRootMarker{}, &Error{Kind: ErrorConcurrentMutation, Operation: OperationCreate, Path: root, Cause: err}
	}
	if closeErr := lock.Close(); closeErr != nil {
		_ = os.Remove(lockPath)
		return CacheRootMarker{}, &Error{Kind: ErrorConcurrentMutation, Operation: OperationCreate, Path: root, Cause: closeErr}
	}
	defer func() {
		if removeErr := os.Remove(lockPath); removeErr != nil && returnErr == nil {
			returnErr = &Error{Kind: ErrorConcurrentMutation, Operation: OperationCreate, Path: root, Cause: removeErr}
		}
	}()
	if existing, readErr := readCacheRootMarker(root); readErr == nil {
		return existing, nil
	} else if _, statErr := os.Lstat(filepath.Join(root, CacheRootMarkerName)); statErr == nil {
		return CacheRootMarker{}, readErr
	}
	allowed, err := directoryContainsOnly(root, filepath.Base(lockPath))
	if err != nil || !allowed {
		return CacheRootMarker{}, &Error{Kind: ErrorUnownedRoot, Operation: OperationCreate, Path: root, Cause: err}
	}

	raw, err := json.Marshal(marker)
	if err != nil {
		return CacheRootMarker{}, &Error{Kind: ErrorInvalidInput, Operation: OperationCreate, Path: root, Cause: err}
	}
	markerPath := filepath.Join(root, CacheRootMarkerName)
	if err := writeAtomic(markerPath, raw, SensitiveFileMode); err != nil {
		return CacheRootMarker{}, &Error{Kind: ErrorConcurrentMutation, Operation: OperationCreate, Path: markerPath, Cause: err}
	}
	return marker, nil
}

func readCacheRootMarker(root string) (CacheRootMarker, error) {
	path := filepath.Join(root, CacheRootMarkerName)
	info, err := os.Lstat(path)
	if err != nil {
		return CacheRootMarker{}, &Error{Kind: ErrorUnownedRoot, Operation: OperationRead, Path: root, Cause: err}
	}
	if !info.Mode().IsRegular() {
		return CacheRootMarker{}, &Error{Kind: ErrorUnsupportedFileType, Operation: OperationRead, Path: path}
	}
	if info.Mode().Perm()&0o077 != 0 {
		return CacheRootMarker{}, &Error{Kind: ErrorUnsafePermissions, Operation: OperationRead, Path: path}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return CacheRootMarker{}, &Error{Kind: ErrorUnownedRoot, Operation: OperationRead, Path: root, Cause: err}
	}
	if len(raw) > maxCacheMarkerBytes {
		return CacheRootMarker{}, &Error{Kind: ErrorResourceLimit, Operation: OperationRead, Path: path}
	}
	var marker CacheRootMarker
	if err := json.Unmarshal(raw, &marker); err != nil || !marker.Valid() {
		return CacheRootMarker{}, &Error{Kind: ErrorUnownedRoot, Operation: OperationRead, Path: root, Cause: err}
	}
	return marker, nil
}

func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) (returnErr error) {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil && returnErr == nil {
			returnErr = closeErr
		}
	}()
	return directory.Sync()
}
