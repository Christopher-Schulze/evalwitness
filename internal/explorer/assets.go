package explorer

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	assetManifestSchemaVersion = "evalwitness.evidence-explorer-assets.v1"
	maximumExplorerAssetBytes  = 2 << 20
)

//go:embed assets/explorer.css assets/explorer.js assets/manifest.json
var embeddedExplorerAssets embed.FS

type explorerAssetManifest struct {
	Files         []explorerAssetRecord `json:"files"`
	SchemaVersion string                `json:"schema_version"`
}

type explorerAssetRecord struct {
	Bytes  int    `json:"bytes"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type explorerAssetBundle struct {
	stylesheet     []byte
	javascript     []byte
	rendererDigest string
}

func loadExplorerAssets() (explorerAssetBundle, error) {
	manifestRaw, err := embeddedExplorerAssets.ReadFile("assets/manifest.json")
	if err != nil {
		return explorerAssetBundle{}, fmt.Errorf("read explorer asset manifest: %w", err)
	}
	manifest, err := decodeExplorerAssetManifest(manifestRaw)
	if err != nil {
		return explorerAssetBundle{}, err
	}
	assets, err := readVerifiedExplorerAssets(manifest)
	if err != nil {
		return explorerAssetBundle{}, err
	}
	return explorerAssetBundle{
		stylesheet: assets["explorer.css"], javascript: assets["explorer.js"],
		rendererDigest: protocol.DigestBytes(append([]byte(renderTemplateVersion+"\x00"), manifestRaw...)),
	}, nil
}

func decodeExplorerAssetManifest(raw []byte) (explorerAssetManifest, error) {
	var manifest explorerAssetManifest
	if err := protocol.DecodeStrict(raw, &manifest); err != nil {
		return explorerAssetManifest{}, fmt.Errorf("decode explorer asset manifest: %w", err)
	}
	canonical, err := protocol.CanonicalMarshal(manifest)
	if err != nil || !bytes.Equal(append(canonical, '\n'), raw) {
		return explorerAssetManifest{}, errors.New("explorer asset manifest is not canonical JSON")
	}
	if manifest.SchemaVersion != assetManifestSchemaVersion || len(manifest.Files) != 2 ||
		manifest.Files[0].Path != "explorer.css" || manifest.Files[1].Path != "explorer.js" {
		return explorerAssetManifest{}, errors.New("explorer asset manifest inventory is invalid")
	}
	return manifest, nil
}

func readVerifiedExplorerAssets(manifest explorerAssetManifest) (map[string][]byte, error) {
	assets := make(map[string][]byte, len(manifest.Files))
	for _, record := range manifest.Files {
		if record.Bytes < 1 || record.Bytes > maximumExplorerAssetBytes || !validDigest(record.SHA256) {
			return nil, fmt.Errorf("explorer asset %q has invalid bounds or identity", record.Path)
		}
		raw, err := embeddedExplorerAssets.ReadFile("assets/" + record.Path)
		if err != nil || len(raw) != record.Bytes || protocol.DigestBytes(raw) != record.SHA256 {
			return nil, fmt.Errorf("explorer asset %q differs from its manifest", record.Path)
		}
		assets[record.Path] = raw
	}
	if containsClosingElement(assets["explorer.css"], "style") || containsClosingElement(assets["explorer.js"], "script") {
		return nil, errors.New("explorer asset contains an unsafe inline-element terminator")
	}
	return assets, nil
}

func containsClosingElement(raw []byte, element string) bool {
	return strings.Contains(strings.ToLower(string(raw)), "</"+element)
}
