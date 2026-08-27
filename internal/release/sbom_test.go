package release

import (
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestSPDXSBOMIsDeterministicAndBindsBinaryDependencies(t *testing.T) {
	root := releaseAssetFixture(t)
	manifest, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	evidence := make([]binaryEvidence, 0, len(canonicalBinaryPaths))
	for _, binary := range manifest.Assets[:len(canonicalBinaryPaths)] {
		target := canonicalBinaryTargets[binary.Path]
		evidence = append(evidence, binaryEvidence{
			Asset: binary,
			Info: debug.BuildInfo{
				GoVersion: "go1.26.5", Path: "github.com/Christopher-Schulze/evalwitness/cmd/evalwitness",
				Main: debug.Module{Path: "github.com/Christopher-Schulze/evalwitness", Version: "(devel)"},
				Deps: []*debug.Module{
					{Path: "golang.org/x/sys", Version: "v0.47.0", Sum: "h1:example"},
					{Path: "github.com/pelletier/go-toml/v2", Version: "v2.4.3", Sum: "h1:example2"},
				},
				Settings: []debug.BuildSetting{
					{Key: "-trimpath", Value: "true"}, {Key: "CGO_ENABLED", Value: "0"},
					{Key: "GOOS", Value: target[0]}, {Key: "GOARCH", Value: target[1]},
				},
			},
		})
	}
	first, err := buildSBOM(manifest, slices.Clone(evidence))
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildSBOM(manifest, slices.Clone(evidence))
	if err != nil {
		t.Fatal(err)
	}
	firstRaw, err := EncodeSBOM(first, manifest)
	if err != nil {
		t.Fatal(err)
	}
	secondRaw, err := EncodeSBOM(second, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRaw) != string(secondRaw) || len(first.Packages) != 3 || len(first.Files) != 5 || len(first.Relationships) != 7 {
		t.Fatalf("SPDX output is not deterministic or complete: packages=%d files=%d relationships=%d", len(first.Packages), len(first.Files), len(first.Relationships))
	}
	if _, err := DecodeSBOM(firstRaw, manifest); err != nil {
		t.Fatal(err)
	}
	tampered := first
	tampered.Files = slices.Clone(first.Files)
	tampered.Files[0].Checksums = []SPDXChecksum{{Algorithm: "SHA256", ChecksumValue: strings.Repeat("f", 64)}}
	tamperedRaw, err := protocol.CanonicalMarshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSBOM(tamperedRaw, manifest); err == nil {
		t.Fatal("SPDX decoder accepted a document that no longer binds the manifest")
	}
}

func TestSPDXRejectsDirtyAndUnpinnedBuildInformation(t *testing.T) {
	root := releaseAssetFixture(t)
	manifest, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	binary := manifest.Assets[0]
	target := canonicalBinaryTargets[binary.Path]
	info := debug.BuildInfo{
		GoVersion: "go1.26.5", Path: "github.com/Christopher-Schulze/evalwitness/cmd/evalwitness",
		Main:     debug.Module{Path: "github.com/Christopher-Schulze/evalwitness"},
		Deps:     []*debug.Module{{Path: "example.invalid/local", Version: "(devel)"}},
		Settings: []debug.BuildSetting{{Key: "-trimpath", Value: "true"}, {Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: manifest.GitCommit}, {Key: "vcs.modified", Value: "true"}, {Key: "CGO_ENABLED", Value: "0"}, {Key: "GOOS", Value: target[0]}, {Key: "GOARCH", Value: target[1]}},
	}
	if _, err := buildSBOM(manifest, []binaryEvidence{{Asset: binary, Info: info}}); err == nil {
		t.Fatal("SPDX builder accepted dirty, unpinned build information")
	}
}
