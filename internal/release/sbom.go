package release

import (
	"bytes"
	"debug/buildinfo"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	SPDXVersion       = "SPDX-2.3"
	SPDXDataLicense   = "CC0-1.0"
	SPDXDocumentID    = "SPDXRef-DOCUMENT"
	SPDXRootPackageID = "SPDXRef-Package-EvalWitness"
)

type SPDXCreationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

type SPDXChecksum struct {
	Algorithm     string `json:"algorithm"`
	ChecksumValue string `json:"checksumValue"`
}

type SPDXExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceLocator  string `json:"referenceLocator"`
	ReferenceType     string `json:"referenceType"`
}

type SPDXPackage struct {
	Name             string            `json:"name"`
	SPDXID           string            `json:"SPDXID"`
	VersionInfo      string            `json:"versionInfo"`
	DownloadLocation string            `json:"downloadLocation"`
	FilesAnalyzed    bool              `json:"filesAnalyzed"`
	LicenseConcluded string            `json:"licenseConcluded"`
	LicenseDeclared  string            `json:"licenseDeclared"`
	CopyrightText    string            `json:"copyrightText"`
	ExternalRefs     []SPDXExternalRef `json:"externalRefs,omitempty"`
}

type SPDXFile struct {
	FileName         string         `json:"fileName"`
	SPDXID           string         `json:"SPDXID"`
	Checksums        []SPDXChecksum `json:"checksums"`
	FileTypes        []string       `json:"fileTypes"`
	LicenseConcluded string         `json:"licenseConcluded"`
	CopyrightText    string         `json:"copyrightText"`
}

type SPDXRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

type SPDXDocument struct {
	SPDXVersion       string             `json:"spdxVersion"`
	DataLicense       string             `json:"dataLicense"`
	SPDXID            string             `json:"SPDXID"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      SPDXCreationInfo   `json:"creationInfo"`
	DocumentDescribes []string           `json:"documentDescribes"`
	Packages          []SPDXPackage      `json:"packages"`
	Files             []SPDXFile         `json:"files"`
	Relationships     []SPDXRelationship `json:"relationships"`
}

type binaryEvidence struct {
	Asset Asset
	Info  debug.BuildInfo
}

func BuildSBOM(assetRoot string, manifest Manifest) (SPDXDocument, error) {
	if err := VerifyManifestAssets(assetRoot, manifest); err != nil {
		return SPDXDocument{}, err
	}
	evidence := make([]binaryEvidence, 0, 5)
	for _, asset := range manifest.Assets {
		if asset.Role != "binary" {
			continue
		}
		info, err := buildinfo.ReadFile(filepath.Join(assetRoot, filepath.FromSlash(asset.Path)))
		if err != nil {
			return SPDXDocument{}, fmt.Errorf("read Go build information for %q: %w", asset.Path, err)
		}
		evidence = append(evidence, binaryEvidence{Asset: asset, Info: *info})
	}
	return buildSBOM(manifest, evidence)
}

func VerifySBOM(assetRoot string, manifest Manifest, raw []byte) (SPDXDocument, error) {
	document, err := DecodeSBOM(raw, manifest)
	if err != nil {
		return SPDXDocument{}, err
	}
	expected, err := BuildSBOM(assetRoot, manifest)
	if err != nil {
		return SPDXDocument{}, err
	}
	canonical, err := EncodeSBOM(expected, manifest)
	if err != nil || !bytes.Equal(canonical, raw) {
		return SPDXDocument{}, errors.New("SPDX SBOM differs from the release binaries")
	}
	return document, nil
}

func EncodeSBOM(document SPDXDocument, manifest Manifest) ([]byte, error) {
	if err := document.Validate(manifest); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(document)
}

func DecodeSBOM(raw []byte, manifest Manifest) (SPDXDocument, error) {
	var document SPDXDocument
	if err := protocol.DecodeStrict(raw, &document); err != nil {
		return SPDXDocument{}, fmt.Errorf("decode SPDX SBOM: %w", err)
	}
	canonical, err := EncodeSBOM(document, manifest)
	if err != nil || !bytes.Equal(canonical, raw) {
		return SPDXDocument{}, errors.New("SPDX SBOM is not canonical JSON")
	}
	return document, nil
}

func buildSBOM(manifest Manifest, binaries []binaryEvidence) (SPDXDocument, error) {
	if err := manifest.Validate(); err != nil {
		return SPDXDocument{}, err
	}
	expectedBinaryCount := 0
	for _, asset := range manifest.Assets {
		if asset.Role == "binary" {
			expectedBinaryCount++
		}
	}
	if len(binaries) != expectedBinaryCount || len(binaries) == 0 {
		return SPDXDocument{}, errors.New("SPDX input does not cover every release binary")
	}
	slices.SortFunc(binaries, func(left, right binaryEvidence) int { return strings.Compare(left.Asset.Path, right.Asset.Path) })
	dependencies := make(map[string]debug.Module)
	files := make([]SPDXFile, 0, len(binaries))
	for _, binary := range binaries {
		if err := validateBinaryBuildInfo(binary, manifest); err != nil {
			return SPDXDocument{}, err
		}
		files = append(files, SPDXFile{
			FileName: "./assets/" + binary.Asset.Path, SPDXID: spdxFileID(binary.Asset.Path),
			Checksums: []SPDXChecksum{{Algorithm: "SHA256", ChecksumValue: binary.Asset.SHA256}},
			FileTypes: []string{"BINARY"}, LicenseConcluded: "NOASSERTION", CopyrightText: "NOASSERTION",
		})
		for _, dependency := range binary.Info.Deps {
			if dependency == nil {
				return SPDXDocument{}, fmt.Errorf("binary %q has an empty dependency record", binary.Asset.Path)
			}
			effective := *dependency
			if dependency.Replace != nil {
				effective = *dependency.Replace
			}
			if effective.Path == "" || effective.Version == "" || effective.Version == "(devel)" || effective.Sum == "" {
				return SPDXDocument{}, fmt.Errorf("binary %q has an unpinned dependency %q", binary.Asset.Path, dependency.Path)
			}
			key := effective.Path + "@" + effective.Version
			if existing, found := dependencies[key]; found && existing.Sum != effective.Sum {
				return SPDXDocument{}, fmt.Errorf("dependency %q has conflicting checksums", key)
			}
			dependencies[key] = effective
		}
	}
	packages := []SPDXPackage{{
		Name: ProductName, SPDXID: SPDXRootPackageID, VersionInfo: manifest.ProductVersion,
		DownloadLocation: "NOASSERTION", FilesAnalyzed: true, LicenseConcluded: "MIT",
		LicenseDeclared: "MIT", CopyrightText: "Copyright (c) 2026 Christopher",
		ExternalRefs: []SPDXExternalRef{{ReferenceCategory: "PACKAGE-MANAGER", ReferenceType: "purl", ReferenceLocator: "pkg:golang/github.com/Christopher-Schulze/evalwitness@v" + manifest.ProductVersion}},
	}}
	dependencyKeys := make([]string, 0, len(dependencies))
	for key := range dependencies {
		dependencyKeys = append(dependencyKeys, key)
	}
	slices.Sort(dependencyKeys)
	for _, key := range dependencyKeys {
		dependency := dependencies[key]
		packages = append(packages, SPDXPackage{
			Name: dependency.Path, SPDXID: spdxDependencyID(key), VersionInfo: dependency.Version,
			DownloadLocation: "NOASSERTION", FilesAnalyzed: false, LicenseConcluded: "NOASSERTION",
			LicenseDeclared: "NOASSERTION", CopyrightText: "NOASSERTION",
			ExternalRefs: []SPDXExternalRef{{ReferenceCategory: "PACKAGE-MANAGER", ReferenceType: "purl", ReferenceLocator: goPackageURL(dependency)}},
		})
	}
	relationships := make([]SPDXRelationship, 0, len(files)+len(dependencyKeys))
	for _, file := range files {
		relationships = append(relationships, SPDXRelationship{SPDXElementID: SPDXRootPackageID, RelationshipType: "CONTAINS", RelatedSPDXElement: file.SPDXID})
	}
	for _, key := range dependencyKeys {
		relationships = append(relationships, SPDXRelationship{SPDXElementID: SPDXRootPackageID, RelationshipType: "DEPENDS_ON", RelatedSPDXElement: spdxDependencyID(key)})
	}
	document := SPDXDocument{
		SPDXVersion: SPDXVersion, DataLicense: SPDXDataLicense, SPDXID: SPDXDocumentID,
		Name: ProductName + "-" + manifest.ProductVersion, DocumentNamespace: "https://evalwitness.dev/spdx/" + manifest.ProductVersion + "/" + manifest.GitCommit,
		CreationInfo:      SPDXCreationInfo{Created: manifest.CreatedAt, Creators: []string{"Tool: evalwitness-" + manifest.ProductVersion}},
		DocumentDescribes: []string{SPDXRootPackageID}, Packages: packages, Files: files, Relationships: relationships,
	}
	if err := document.Validate(manifest); err != nil {
		return SPDXDocument{}, err
	}
	return document, nil
}

func (d SPDXDocument) Validate(manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if d.SPDXVersion != SPDXVersion || d.DataLicense != SPDXDataLicense || d.SPDXID != SPDXDocumentID ||
		d.Name != ProductName+"-"+manifest.ProductVersion ||
		d.DocumentNamespace != "https://evalwitness.dev/spdx/"+manifest.ProductVersion+"/"+manifest.GitCommit ||
		d.CreationInfo.Created != manifest.CreatedAt || !slices.Equal(d.CreationInfo.Creators, []string{"Tool: evalwitness-" + manifest.ProductVersion}) ||
		!slices.Equal(d.DocumentDescribes, []string{SPDXRootPackageID}) {
		return errors.New("SPDX document identity is invalid")
	}
	if len(d.Packages) == 0 || d.Packages[0].SPDXID != SPDXRootPackageID || len(d.Files) == 0 {
		return errors.New("SPDX package or file inventory is incomplete")
	}
	rootPackage := d.Packages[0]
	if rootPackage.Name != ProductName || rootPackage.VersionInfo != manifest.ProductVersion || !rootPackage.FilesAnalyzed ||
		rootPackage.LicenseConcluded != "MIT" || rootPackage.LicenseDeclared != "MIT" || rootPackage.CopyrightText != "Copyright (c) 2026 Christopher" {
		return errors.New("SPDX root package identity is invalid")
	}
	seenIDs := map[string]struct{}{SPDXDocumentID: {}}
	previousPackage := ""
	for index, pkg := range d.Packages {
		packageIdentity := pkg.Name + "@" + pkg.VersionInfo
		if pkg.SPDXID == "" || pkg.Name == "" || pkg.VersionInfo == "" || pkg.DownloadLocation != "NOASSERTION" || len(pkg.ExternalRefs) != 1 ||
			(index > 0 && packageIdentity <= previousPackage) {
			return errors.New("SPDX packages are invalid, duplicated, or unsorted")
		}
		if _, duplicate := seenIDs[pkg.SPDXID]; duplicate {
			return errors.New("SPDX element ID is duplicated")
		}
		seenIDs[pkg.SPDXID] = struct{}{}
		if index > 0 {
			previousPackage = packageIdentity
		}
	}
	binaryAssets := make([]Asset, 0, len(d.Files))
	for _, asset := range manifest.Assets {
		if asset.Role == "binary" {
			binaryAssets = append(binaryAssets, asset)
		}
	}
	if len(d.Files) != len(binaryAssets) {
		return errors.New("SPDX file inventory does not cover every release binary")
	}
	previousFileName := ""
	for index, file := range d.Files {
		expectedAsset := binaryAssets[index]
		if file.FileName <= previousFileName || file.FileName != "./assets/"+expectedAsset.Path || file.SPDXID != spdxFileID(expectedAsset.Path) ||
			len(file.Checksums) != 1 || file.Checksums[0].Algorithm != "SHA256" || file.Checksums[0].ChecksumValue != expectedAsset.SHA256 ||
			!slices.Equal(file.FileTypes, []string{"BINARY"}) || file.LicenseConcluded != "NOASSERTION" || file.CopyrightText != "NOASSERTION" {
			return errors.New("SPDX files are invalid, duplicated, or unsorted")
		}
		if _, duplicate := seenIDs[file.SPDXID]; duplicate {
			return errors.New("SPDX element ID is duplicated")
		}
		seenIDs[file.SPDXID] = struct{}{}
		previousFileName = file.FileName
	}
	if len(d.Relationships) != len(d.Files)+len(d.Packages)-1 {
		return errors.New("SPDX relationship inventory is incomplete")
	}
	for index, relationship := range d.Relationships {
		if relationship.SPDXElementID != SPDXRootPackageID || (relationship.RelationshipType != "CONTAINS" && relationship.RelationshipType != "DEPENDS_ON") {
			return errors.New("SPDX relationship is invalid")
		}
		if _, found := seenIDs[relationship.RelatedSPDXElement]; !found {
			return errors.New("SPDX relationship references an unknown element")
		}
		if index < len(d.Files) {
			if relationship.RelationshipType != "CONTAINS" || relationship.RelatedSPDXElement != d.Files[index].SPDXID {
				return errors.New("SPDX binary containment relationships are not canonical")
			}
			continue
		}
		packageIndex := index - len(d.Files) + 1
		if relationship.RelationshipType != "DEPENDS_ON" || relationship.RelatedSPDXElement != d.Packages[packageIndex].SPDXID {
			return errors.New("SPDX dependency relationships are not canonical")
		}
	}
	return nil
}

func validateBinaryBuildInfo(binary binaryEvidence, manifest Manifest) error {
	if binary.Asset.Role != "binary" || binary.Info.Path != "github.com/Christopher-Schulze/evalwitness/cmd/evalwitness" ||
		binary.Info.Main.Path != "github.com/Christopher-Schulze/evalwitness" || binary.Info.GoVersion == "" {
		return fmt.Errorf("binary %q has unexpected Go build identity", binary.Asset.Path)
	}
	target, found := canonicalBinaryTargets[binary.Asset.Path]
	if !found {
		return fmt.Errorf("binary %q is not in the canonical release set", binary.Asset.Path)
	}
	settings := make(map[string]string, len(binary.Info.Settings))
	for _, setting := range binary.Info.Settings {
		if _, duplicate := settings[setting.Key]; duplicate {
			return fmt.Errorf("binary %q has duplicate build setting %q", binary.Asset.Path, setting.Key)
		}
		settings[setting.Key] = setting.Value
	}
	for _, forbidden := range []string{"vcs", "vcs.revision", "vcs.modified", "vcs.time"} {
		if _, found := settings[forbidden]; found {
			return fmt.Errorf("binary %q embeds VCS metadata that prevents archive-only reproduction", binary.Asset.Path)
		}
	}
	if settings["-trimpath"] != "true" || settings["CGO_ENABLED"] != "0" || settings["GOOS"] != target[0] || settings["GOARCH"] != target[1] {
		return fmt.Errorf("binary %q is not a clean, pinned, cross-platform release build", binary.Asset.Path)
	}
	return nil
}

func spdxFileID(path string) string {
	return "SPDXRef-File-" + protocol.DigestBytes([]byte(path))[:20]
}

func spdxDependencyID(key string) string {
	return "SPDXRef-Dependency-" + protocol.DigestBytes([]byte(key))[:20]
}

func goPackageURL(module debug.Module) string {
	return "pkg:golang/" + module.Path + "@" + url.PathEscape(module.Version) + "?checksum=" + url.QueryEscape(module.Sum)
}
