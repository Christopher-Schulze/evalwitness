package safety

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const atomicCandidateAttempts = 8

func (r *CacheRoot) PublishSensitive(relative string, data []byte) (returnErr error) {
	if r == nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationWrite, Path: relative}
	}
	clean, err := cleanRelativePath(relative)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(r.path)
	if err != nil {
		return &Error{Kind: ErrorUnownedRoot, Operation: OperationWrite, Path: r.path, Cause: err}
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil && returnErr == nil {
			returnErr = &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: r.path, Cause: closeErr}
		}
	}()
	directory := filepath.Dir(clean)
	if err := root.MkdirAll(directory, SensitiveDirectoryMode); err != nil {
		return &Error{Kind: ErrorContainmentViolation, Operation: OperationWrite, Path: clean, Cause: err}
	}
	if directory != "." {
		if err := root.Chmod(directory, SensitiveDirectoryMode); err != nil {
			return &Error{Kind: ErrorUnsafePermissions, Operation: OperationWrite, Path: directory, Cause: err}
		}
	}
	temporary, temporaryPath, err := createAtomicCandidate(root, clean)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = root.Remove(temporaryPath)
		}
	}()
	if err := writeAndSyncSensitive(temporary, data); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: clean, Cause: err}
	}
	if err := root.Rename(temporaryPath, clean); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: clean, Cause: err}
	}
	committed = true
	if err := syncRootDirectory(root, directory); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: directory, Cause: err}
	}
	return nil
}

// PublishSensitiveExclusive atomically publishes a new owner-only file and
// refuses to replace an existing path. The temporary inode is linked into the
// destination only after its contents and permissions have been synced.
func (r *CacheRoot) PublishSensitiveExclusive(relative string, data []byte) (returnErr error) {
	if r == nil {
		return &Error{Kind: ErrorInvalidInput, Operation: OperationWrite, Path: relative}
	}
	clean, err := cleanRelativePath(relative)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(r.path)
	if err != nil {
		return &Error{Kind: ErrorUnownedRoot, Operation: OperationWrite, Path: r.path, Cause: err}
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil && returnErr == nil {
			returnErr = &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: r.path, Cause: closeErr}
		}
	}()
	directory := filepath.Dir(clean)
	if err := root.MkdirAll(directory, SensitiveDirectoryMode); err != nil {
		return &Error{Kind: ErrorContainmentViolation, Operation: OperationWrite, Path: clean, Cause: err}
	}
	if directory != "." {
		if err := root.Chmod(directory, SensitiveDirectoryMode); err != nil {
			return &Error{Kind: ErrorUnsafePermissions, Operation: OperationWrite, Path: directory, Cause: err}
		}
	}
	temporary, temporaryPath, err := createAtomicCandidate(root, clean)
	if err != nil {
		return err
	}
	defer func() {
		_ = temporary.Close()
		_ = root.Remove(temporaryPath)
	}()
	if err := writeAndSyncSensitive(temporary, data); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: clean, Cause: err}
	}
	if err := root.Link(temporaryPath, clean); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: clean, Cause: err}
	}
	if err := syncRootDirectory(root, directory); err != nil {
		return &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: directory, Cause: err}
	}
	return nil
}

func (r *CacheRoot) ReadSensitive(relative string, maxBytes int64) (data []byte, returnErr error) {
	if r == nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Path: relative}
	}
	clean, err := cleanRelativePath(relative)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Path: clean}
	}
	root, err := os.OpenRoot(r.path)
	if err != nil {
		return nil, &Error{Kind: ErrorUnownedRoot, Operation: OperationRead, Path: r.path, Cause: err}
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil && returnErr == nil {
			returnErr = &Error{Kind: ErrorConcurrentMutation, Operation: OperationRead, Path: r.path, Cause: closeErr}
		}
	}()
	file, err := root.Open(clean)
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Path: clean, Cause: err}
	}
	data, readErr := readBoundedSensitive(file, maxBytes)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, &Error{Kind: ErrorConcurrentMutation, Operation: OperationRead, Path: clean, Cause: closeErr}
	}
	return data, nil
}

func cleanRelativePath(relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", &Error{Kind: ErrorInvalidInput, Operation: OperationValidate, Path: relative}
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || len(clean) > 2 && clean[:3] == ".."+string(filepath.Separator) {
		return "", &Error{Kind: ErrorContainmentViolation, Operation: OperationValidate, Path: relative}
	}
	return clean, nil
}

func createAtomicCandidate(root *os.Root, target string) (*os.File, string, error) {
	directory := filepath.Dir(target)
	base := filepath.Base(target)
	for range atomicCandidateAttempts {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return nil, "", &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: target, Cause: err}
		}
		candidate := filepath.Join(directory, "."+base+".tmp-"+hex.EncodeToString(random))
		file, err := root.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, SensitiveFileMode)
		if err == nil {
			return file, candidate, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: target, Cause: err}
		}
	}
	return nil, "", &Error{Kind: ErrorConcurrentMutation, Operation: OperationWrite, Path: target}
}

func writeAndSyncSensitive(file *os.File, data []byte) error {
	if err := file.Chmod(SensitiveFileMode); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}

func readBoundedSensitive(file *os.File, maxBytes int64) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	if !info.Mode().IsRegular() {
		return nil, &Error{Kind: ErrorUnsupportedFileType, Operation: OperationRead}
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, &Error{Kind: ErrorUnsafePermissions, Operation: OperationRead}
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, &Error{Kind: ErrorInvalidInput, Operation: OperationRead, Cause: err}
	}
	if int64(len(data)) > maxBytes {
		return nil, &Error{Kind: ErrorResourceLimit, Operation: OperationRead}
	}
	return data, nil
}

func syncRootDirectory(root *os.Root, relative string) error {
	directory, err := root.Open(relative)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}
