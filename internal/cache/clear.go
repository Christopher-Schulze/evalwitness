package cache

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

type ClearScope string

const (
	ClearResponses    ClearScope = "responses"
	ClearCapabilities ClearScope = "capabilities"
	ClearAll          ClearScope = "all"
)

func (s ClearScope) Valid() bool {
	return s == ClearResponses || s == ClearCapabilities || s == ClearAll
}

type ClearResult struct {
	Scope        ClearScope `json:"scope"`
	FilesRemoved int64      `json:"files_removed"`
	BytesRemoved int64      `json:"bytes_removed"`
}

func (c *Cache) Clear(scope ClearScope) (result ClearResult, returnErr error) {
	result = ClearResult{Scope: scope}
	if c == nil || !scope.Valid() {
		return result, &safety.Error{Kind: safety.ErrorInvalidInput, Operation: safety.OperationDelete}
	}
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	root, err := c.ownedRoot(false)
	if err != nil {
		return result, err
	}
	filesystemRoot, err := os.OpenRoot(root.Path())
	if err != nil {
		return result, &safety.Error{Kind: safety.ErrorUnownedRoot, Operation: safety.OperationDelete, Path: root.Path(), Cause: err}
	}
	defer func() {
		if closeErr := filesystemRoot.Close(); closeErr != nil && returnErr == nil {
			returnErr = &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: root.Path(), Cause: closeErr}
		}
	}()
	routes, err := ownedRouteDirectories(root.Path())
	if err != nil {
		return result, err
	}
	for _, routeID := range routes {
		if err := clearRoute(filesystemRoot, routeID, scope, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func ownedRouteDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "routes"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, &safety.Error{Kind: safety.ErrorInvalidInput, Operation: safety.OperationDelete, Path: root, Cause: err}
	}
	routes := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && safety.IsSafeNamespaceID(entry.Name()) {
			routes = append(routes, entry.Name())
		}
	}
	return routes, nil
}

func clearRoute(root *os.Root, routeID string, scope ClearScope, result *ClearResult) error {
	directory := filepath.Join("routes", routeID)
	if scope == ClearResponses || scope == ClearAll {
		if err := removeOwnedPath(root, filepath.Join(directory, "responses"), result); err != nil {
			return err
		}
	}
	if scope == ClearCapabilities || scope == ClearAll {
		if err := removeOwnedPath(root, filepath.Join(directory, "capabilities.json"), result); err != nil {
			return err
		}
	}
	return syncOwnedDirectory(root, directory)
}

func removeOwnedPath(root *os.Root, relative string, result *ClearResult) error {
	tombstone, exists, err := renameToTombstone(root, relative)
	if err != nil || !exists {
		return err
	}
	files, bytes, err := measureOwnedPath(root, tombstone)
	if err != nil {
		return err
	}
	if err := root.RemoveAll(tombstone); err != nil {
		return &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	result.FilesRemoved += files
	result.BytesRemoved += bytes
	return nil
}

func renameToTombstone(root *os.Root, relative string) (string, bool, error) {
	if _, err := root.Lstat(relative); errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	} else if err != nil {
		return "", false, &safety.Error{Kind: safety.ErrorInvalidInput, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", false, &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	tombstone := filepath.Join(filepath.Dir(relative), ".evalwitness-clear-"+hex.EncodeToString(random))
	if err := root.Rename(relative, tombstone); err != nil {
		return "", false, &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	return tombstone, true, nil
}

func measureOwnedPath(root *os.Root, relative string) (int64, int64, error) {
	var files int64
	var bytes int64
	err := fs.WalkDir(root.FS(), relative, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files++
		bytes += info.Size()
		return nil
	})
	if err != nil {
		return 0, 0, &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	return files, bytes, nil
}

func syncOwnedDirectory(root *os.Root, relative string) error {
	directory, err := root.Open(relative)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	if err := directory.Close(); err != nil {
		return &safety.Error{Kind: safety.ErrorConcurrentMutation, Operation: safety.OperationDelete, Path: relative, Cause: err}
	}
	return nil
}
