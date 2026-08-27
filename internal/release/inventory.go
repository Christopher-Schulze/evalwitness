package release

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

func BuildManifest(assetRoot, productVersion, gitCommit, sourceArchiveSHA256, createdAt string) (Manifest, error) {
	assets, err := inventoryAssets(assetRoot)
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion, Product: ProductName, ProductVersion: productVersion,
		GitCommit: gitCommit, SourceArchiveSHA256: sourceArchiveSHA256, CreatedAt: createdAt,
		Assets: assets, AssetCount: len(assets), Truth: DefaultTruthStatus(),
	}
	for _, asset := range assets {
		manifest.TotalBytes += asset.Bytes
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	if err := VerifyGoModuleProxy(assetRoot, manifest); err != nil {
		return Manifest{}, err
	}
	if err := VerifySourceArchiveReport(assetRoot, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func VerifyManifestAssets(assetRoot string, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	actual, err := inventoryAssets(assetRoot)
	if err != nil {
		return err
	}
	if !slices.Equal(actual, manifest.Assets) {
		return errors.New("release assets differ from the manifest")
	}
	if err := VerifyGoModuleProxy(assetRoot, manifest); err != nil {
		return err
	}
	return VerifySourceArchiveReport(assetRoot, manifest)
}

func inventoryAssets(root string) ([]Asset, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("release asset root %q is not a real directory", root)
	}
	assets := make([]Asset, 0, len(canonicalRoles))
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("release asset path %q is a symlink", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("release asset path %q is not a regular file", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		canonicalPath := filepath.ToSlash(relative)
		if !validAssetPath(canonicalPath) || !slices.Contains(canonicalRoles, assetRole(canonicalPath)) {
			return fmt.Errorf("release asset path %q has no canonical role", canonicalPath)
		}
		size, digest, err := digestStableRegularFile(path)
		if err != nil {
			return err
		}
		assets = append(assets, Asset{Path: canonicalPath, Role: assetRole(canonicalPath), Bytes: size, SHA256: digest})
		if len(assets) > maximumAssetCount {
			return errors.New("release asset count exceeds the hard limit")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(assets, func(left, right Asset) int {
		if left.Path < right.Path {
			return -1
		}
		if left.Path > right.Path {
			return 1
		}
		return 0
	})
	return assets, nil
}

func digestStableRegularFile(path string) (size int64, digest string, returnErr error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > maximumAssetBytes {
		return 0, "", fmt.Errorf("release asset %q is not a bounded regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close release asset %q: %w", path, closeErr)
		}
	}()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return 0, "", fmt.Errorf("release asset %q changed before capture", path)
	}
	hasher := sha256.New()
	written, err := io.Copy(hasher, io.LimitReader(file, maximumAssetBytes+1))
	if err != nil || written != before.Size() {
		return 0, "", fmt.Errorf("release asset %q changed during capture", path)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(opened, after) || after.Size() != written || after.ModTime() != before.ModTime() {
		return 0, "", fmt.Errorf("release asset %q changed after capture", path)
	}
	return written, hex.EncodeToString(hasher.Sum(nil)), nil
}

func readStableReleaseFile(root string, asset Asset) ([]byte, error) {
	filePath := filepath.Join(root, filepath.FromSlash(asset.Path))
	before, err := os.Lstat(filePath)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() != asset.Bytes {
		return nil, fmt.Errorf("release asset %q is not its declared regular file", asset.Path)
	}
	raw, err := os.ReadFile(filePath)
	if err != nil || int64(len(raw)) != before.Size() {
		return nil, fmt.Errorf("release asset %q could not be read completely", asset.Path)
	}
	after, err := os.Lstat(filePath)
	if err != nil || !os.SameFile(before, after) || after.Size() != before.Size() || after.ModTime() != before.ModTime() {
		return nil, fmt.Errorf("release asset %q changed during read", asset.Path)
	}
	return raw, nil
}
