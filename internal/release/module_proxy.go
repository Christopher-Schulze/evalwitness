package release

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/Christopher-Schulze/evalwitness/protocol"
	"golang.org/x/mod/module"
)

const (
	GoModuleProxySchemaVersion = "evalwitness.go-module-proxy.v1"
	goModuleProxyPrefix        = "source/go-proxy/"
	goModuleProxyIndexPath     = goModuleProxyPrefix + "index.json"
)

type GoModuleProxyFile struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type GoModuleProxyModule struct {
	Path     string   `json:"path"`
	Version  string   `json:"version"`
	Sum      string   `json:"sum"`
	GoModSum string   `json:"go_mod_sum"`
	Files    []string `json:"files"`
}

type GoModuleProxyIndex struct {
	SchemaVersion string                `json:"schema_version"`
	ModuleCount   int                   `json:"module_count"`
	FileCount     int                   `json:"file_count"`
	Modules       []GoModuleProxyModule `json:"modules"`
	Files         []GoModuleProxyFile   `json:"files"`
}

func VerifyGoModuleProxy(assetRoot string, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	indexRaw, err := readManifestAsset(assetRoot, manifest, goModuleProxyIndexPath)
	if err != nil {
		return err
	}
	index, err := DecodeGoModuleProxyIndex(indexRaw)
	if err != nil {
		return err
	}
	proxyAssets := make(map[string]Asset, len(index.Files))
	for _, asset := range manifest.Assets {
		if strings.HasPrefix(asset.Path, goModuleProxyPrefix) && asset.Path != goModuleProxyIndexPath {
			proxyAssets[strings.TrimPrefix(asset.Path, goModuleProxyPrefix)] = asset
		}
	}
	if len(proxyAssets) != len(index.Files) {
		return errors.New("offline Go module proxy asset count differs from its index")
	}
	for _, file := range index.Files {
		asset, found := proxyAssets[file.Path]
		if !found || asset.Bytes != file.Bytes || asset.SHA256 != file.SHA256 {
			return fmt.Errorf("offline Go module proxy file %q differs from its release asset", file.Path)
		}
	}
	versionsByList := make(map[string][]string)
	for _, declared := range index.Modules {
		escapedPath, err := module.EscapePath(declared.Path)
		if err != nil {
			return fmt.Errorf("escape offline Go module path %q: %w", declared.Path, err)
		}
		listPath := escapedPath + "/@v/list"
		versionsByList[listPath] = append(versionsByList[listPath], declared.Version)
	}
	for listPath, versions := range versionsByList {
		slices.Sort(versions)
		expected := []byte(strings.Join(versions, "\n") + "\n")
		actual, err := readManifestAsset(assetRoot, manifest, goModuleProxyPrefix+listPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("offline Go module proxy list %q differs from its indexed versions", listPath)
		}
	}
	return nil
}

func DecodeGoModuleProxyIndex(raw []byte) (GoModuleProxyIndex, error) {
	var index GoModuleProxyIndex
	if err := protocol.DecodeStrict(raw, &index); err != nil {
		return GoModuleProxyIndex{}, fmt.Errorf("decode offline Go module proxy index: %w", err)
	}
	if err := index.Validate(); err != nil {
		return GoModuleProxyIndex{}, err
	}
	canonical, err := protocol.CanonicalMarshal(index)
	if err != nil || !bytes.Equal(canonical, raw) {
		return GoModuleProxyIndex{}, errors.New("offline Go module proxy index is not canonical JSON")
	}
	return index, nil
}

func (i GoModuleProxyIndex) Validate() error {
	if i.SchemaVersion != GoModuleProxySchemaVersion || i.ModuleCount != len(i.Modules) || i.FileCount != len(i.Files) ||
		len(i.Modules) < 1 || len(i.Modules) > 256 || len(i.Files) < 4 || len(i.Files) > 1024 {
		return errors.New("offline Go module proxy index identity or counts are invalid")
	}
	filesByPath := make(map[string]GoModuleProxyFile, len(i.Files))
	previousFile := ""
	listFiles := 0
	for _, file := range i.Files {
		if file.Path <= previousFile || !validProxyPath(file.Path) || file.Bytes < 1 || file.Bytes > maximumAssetBytes || !validSHA256(file.SHA256) {
			return errors.New("offline Go module proxy files are invalid, duplicated, or unsorted")
		}
		filesByPath[file.Path] = file
		if strings.HasSuffix(file.Path, "/@v/list") {
			listFiles++
		}
		previousFile = file.Path
	}
	moduleFiles := make(map[string]struct{}, len(i.Modules)*3)
	previousModulePath := ""
	previousModuleVersion := ""
	for _, declared := range i.Modules {
		identity := declared.Path + "@" + declared.Version
		if declared.Path < previousModulePath || declared.Path == previousModulePath && declared.Version <= previousModuleVersion ||
			!validModulePath(declared.Path) || !validModuleVersion(declared.Version) ||
			!validGoSum(declared.Sum) || !validGoSum(declared.GoModSum) || len(declared.Files) != 3 {
			return errors.New("offline Go module proxy modules are invalid, duplicated, or unsorted")
		}
		escapedPath, err := module.EscapePath(declared.Path)
		if err != nil {
			return fmt.Errorf("offline Go module %q has an invalid path: %w", identity, err)
		}
		escapedVersion, err := module.EscapeVersion(declared.Version)
		if err != nil {
			return fmt.Errorf("offline Go module %q has an invalid version: %w", identity, err)
		}
		expectedDirectory := escapedPath + "/@v"
		expectedSuffixes := []string{".info", ".mod", ".zip"}
		for index, filePath := range declared.Files {
			expectedPath := expectedDirectory + "/" + escapedVersion + expectedSuffixes[index]
			if filePath != expectedPath || !validProxyPath(filePath) {
				return fmt.Errorf("offline Go module %q has an invalid file set", identity)
			}
			if _, found := filesByPath[filePath]; !found {
				return fmt.Errorf("offline Go module %q references an unindexed file", identity)
			}
			if _, duplicate := moduleFiles[filePath]; duplicate {
				return fmt.Errorf("offline Go module file %q has multiple owners", filePath)
			}
			moduleFiles[filePath] = struct{}{}
		}
		if _, found := filesByPath[expectedDirectory+"/list"]; !found {
			return fmt.Errorf("offline Go module %q has no version list", identity)
		}
		previousModulePath = declared.Path
		previousModuleVersion = declared.Version
	}
	if len(moduleFiles)+listFiles != len(i.Files) {
		return errors.New("offline Go module proxy contains unowned or unexpected files")
	}
	return nil
}

func readManifestAsset(root string, manifest Manifest, assetPath string) ([]byte, error) {
	index := slices.IndexFunc(manifest.Assets, func(asset Asset) bool { return asset.Path == assetPath })
	if index < 0 {
		return nil, fmt.Errorf("release manifest has no %q asset", assetPath)
	}
	asset := manifest.Assets[index]
	raw, err := readStableReleaseFile(root, asset)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) != asset.Bytes || protocol.DigestBytes(raw) != asset.SHA256 {
		return nil, fmt.Errorf("release asset %q differs from its manifest record", assetPath)
	}
	return raw, nil
}

func validProxyPath(value string) bool {
	return validAssetPath("proxy/"+value) && !strings.Contains(value, "//") && !strings.HasPrefix(value, "/")
}

func validModulePath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "//") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, character := range part {
			if unicode.IsSpace(character) || unicode.IsControl(character) {
				return false
			}
		}
	}
	return true
}

func validModuleVersion(value string) bool {
	if len(value) < 2 || value[0] != 'v' || strings.ContainsAny(value, "/\\") {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validGoSum(value string) bool {
	if !strings.HasPrefix(value, "h1:") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "h1:"))
	return err == nil && len(decoded) == 32
}
