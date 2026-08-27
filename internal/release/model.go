package release

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/product"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	ManifestSchemaVersion     = "evalwitness.release-manifest.v1"
	VerificationSchemaVersion = "evalwitness.release-verification.v1"
	ProductName               = "evalwitness"
	maximumAssetCount         = 4096
	maximumAssetBytes         = int64(512 << 20)
	maximumReleaseBytes       = int64(2 << 30)
)

var canonicalRoles = []string{
	"binary",
	"capsule",
	"documentation",
	"evidence",
	"protocol",
	"replication",
	"source",
}

var canonicalBinaryPaths = []string{
	"binary/evalwitness-darwin-amd64",
	"binary/evalwitness-darwin-arm64",
	"binary/evalwitness-linux-amd64",
	"binary/evalwitness-linux-arm64",
	"binary/evalwitness-windows-amd64.exe",
}

var canonicalBinaryTargets = map[string][2]string{
	"binary/evalwitness-darwin-amd64":      {"darwin", "amd64"},
	"binary/evalwitness-darwin-arm64":      {"darwin", "arm64"},
	"binary/evalwitness-linux-amd64":       {"linux", "amd64"},
	"binary/evalwitness-linux-arm64":       {"linux", "arm64"},
	"binary/evalwitness-windows-amd64.exe": {"windows", "amd64"},
}

type TruthStatus struct {
	ProviderCalls          int    `json:"provider_calls"`
	EmpiricalStudy         string `json:"empirical_study"`
	HumanStudy             string `json:"human_study"`
	IndependentReplication string `json:"independent_replication"`
	ExternalPublication    string `json:"external_publication"`
}

type Asset struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion       string      `json:"schema_version"`
	Product             string      `json:"product"`
	ProductVersion      string      `json:"product_version"`
	GitCommit           string      `json:"git_commit"`
	SourceArchiveSHA256 string      `json:"source_archive_sha256"`
	CreatedAt           string      `json:"created_at"`
	Assets              []Asset     `json:"assets"`
	AssetCount          int         `json:"asset_count"`
	TotalBytes          int64       `json:"total_bytes"`
	Truth               TruthStatus `json:"truth"`
}

type VerificationReport struct {
	SchemaVersion   string   `json:"schema_version"`
	Product         string   `json:"product"`
	ProductVersion  string   `json:"product_version"`
	GitCommit       string   `json:"git_commit"`
	ManifestDigest  string   `json:"manifest_digest"`
	SBOMDigest      string   `json:"sbom_digest"`
	StatementDigest string   `json:"statement_digest"`
	AssetCount      int      `json:"asset_count"`
	TotalBytes      int64    `json:"total_bytes"`
	Signed          bool     `json:"signed"`
	VerifiedKeyIDs  []string `json:"verified_key_ids"`
	Valid           bool     `json:"valid"`
}

func DefaultTruthStatus() TruthStatus {
	return TruthStatus{
		ProviderCalls: 0, EmpiricalStudy: "not_run", HumanStudy: "not_run",
		IndependentReplication: "not_run", ExternalPublication: "not_authorized",
	}
}

func (t TruthStatus) Validate() error {
	expected := DefaultTruthStatus()
	if t.ProviderCalls != expected.ProviderCalls || t.EmpiricalStudy != expected.EmpiricalStudy ||
		t.HumanStudy != expected.HumanStudy || t.IndependentReplication != expected.IndependentReplication ||
		(t.ExternalPublication != expected.ExternalPublication && t.ExternalPublication != "authorized_by_tag") {
		return errors.New("release truth status overstates the authorized empirical or publication state")
	}
	return nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion || m.Product != ProductName || !product.ValidVersion(m.ProductVersion) {
		return errors.New("release manifest identity is invalid")
	}
	if !validGitCommit(m.GitCommit) || !validSHA256(m.SourceArchiveSHA256) || !validCreatedAt(m.CreatedAt) {
		return errors.New("release source identity is invalid")
	}
	if err := m.Truth.Validate(); err != nil {
		return err
	}
	if len(m.Assets) < len(canonicalRoles) || len(m.Assets) > maximumAssetCount || m.AssetCount != len(m.Assets) {
		return errors.New("release asset count is invalid")
	}
	roleCounts := make(map[string]int, len(canonicalRoles))
	binaryPaths := make([]string, 0, len(canonicalBinaryPaths))
	sourceArchivePath := "source/evalwitness-" + m.ProductVersion + "-source.tar.gz"
	sourceArchiveBound := false
	moduleProxyIndexPresent := false
	sourceTreeProvenancePresent := false
	var total int64
	previous := ""
	for _, asset := range m.Assets {
		if asset.Path <= previous || !validAssetPath(asset.Path) || asset.Role != assetRole(asset.Path) || !slices.Contains(canonicalRoles, asset.Role) {
			return errors.New("release assets are invalid, duplicated, or unsorted")
		}
		if asset.Bytes < 1 || asset.Bytes > maximumAssetBytes || !validSHA256(asset.SHA256) || total > maximumReleaseBytes-asset.Bytes {
			return fmt.Errorf("release asset %q has invalid size or digest", asset.Path)
		}
		total += asset.Bytes
		roleCounts[asset.Role]++
		if asset.Role == "binary" {
			binaryPaths = append(binaryPaths, asset.Path)
		}
		if asset.Path == sourceArchivePath {
			if asset.SHA256 != m.SourceArchiveSHA256 {
				return errors.New("release source archive digest differs from its manifest asset")
			}
			sourceArchiveBound = true
		}
		if asset.Path == "source/go-proxy/index.json" {
			moduleProxyIndexPresent = true
		}
		if asset.Path == sourceTreeProvenancePath {
			sourceTreeProvenancePresent = true
		}
		if asset.Role == "source" && asset.Path != sourceArchivePath && asset.Path != sourceTreeProvenancePath && !strings.HasPrefix(asset.Path, goModuleProxyPrefix) {
			return fmt.Errorf("release source asset %q is outside the closed source graph", asset.Path)
		}
		previous = asset.Path
	}
	if !slices.Equal(binaryPaths, canonicalBinaryPaths) {
		return errors.New("release binary inventory is not the canonical five-platform set")
	}
	for _, role := range canonicalRoles {
		if roleCounts[role] == 0 {
			return fmt.Errorf("release has no %q asset", role)
		}
	}
	if !sourceArchiveBound || !moduleProxyIndexPresent || !sourceTreeProvenancePresent {
		return errors.New("release source archive, source-tree provenance, or offline Go module proxy is missing")
	}
	if total != m.TotalBytes || total > maximumReleaseBytes {
		return errors.New("release total byte count is invalid")
	}
	return nil
}

func EncodeManifest(manifest Manifest) ([]byte, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(manifest)
}

func DecodeManifest(raw []byte) (Manifest, error) {
	var manifest Manifest
	if err := protocol.DecodeStrict(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode release manifest: %w", err)
	}
	canonical, err := EncodeManifest(manifest)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Manifest{}, errors.New("release manifest is not canonical JSON")
	}
	return manifest, nil
}

func validCreatedAt(value string) bool {
	parsed, err := time.Parse(time.RFC3339, value)
	return err == nil && parsed.UTC().Format(time.RFC3339) == value
}

func validGitCommit(value string) bool {
	return (len(value) == 40 || len(value) == 64) && validLowerHex(value)
}

func validSHA256(value string) bool {
	return len(value) == 64 && validLowerHex(value)
}

func validLowerHex(value string) bool {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func validAssetPath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return false
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func assetRole(path string) string {
	role, _, _ := strings.Cut(path, "/")
	return role
}
