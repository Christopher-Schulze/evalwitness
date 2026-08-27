package release

import (
	"crypto/ed25519"
	"encoding/hex"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
)

func TestReleaseStatementAndExplicitSignatureBindExactInputs(t *testing.T) {
	manifest, manifestRaw, sbomRaw := releaseStatementFixture(t)
	statement, err := BuildStatement(manifestRaw, sbomRaw)
	if err != nil {
		t.Fatal(err)
	}
	statementRaw, err := EncodeStatement(statement, manifestRaw, sbomRaw)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := releaseTestPrivateKey(7)
	signed, err := SealStatement(manifestRaw, sbomRaw, statementRaw, []byte(hex.EncodeToString(privateKey)))
	if err != nil {
		t.Fatal(err)
	}
	envelopeRaw, err := EncodeEnvelope(signed.Envelope)
	if err != nil {
		t.Fatal(err)
	}
	rootRaw, err := EncodeTrustRoot(signed.TrustRoot)
	if err != nil {
		t.Fatal(err)
	}
	policyRaw, err := EncodeSignaturePolicy(signed.Policy, signed.TrustRoot)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifySignedStatement(manifestRaw, sbomRaw, statementRaw, envelopeRaw, rootRaw, policyRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(verified, signed.Policy.AllowedKeyIDs) {
		t.Fatalf("verified keys = %v", verified)
	}

	manifest.CreatedAt = "2026-08-12T10:00:01Z"
	substitutedManifestRaw, err := EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeStatement(statementRaw, substitutedManifestRaw, sbomRaw); err == nil {
		t.Fatal("release statement accepted a substituted manifest")
	}
}

func TestReleasePrivateKeyParserRejectsNoncanonicalAndInconsistentKeys(t *testing.T) {
	privateKey := releaseTestPrivateKey(8)
	canonical := []byte(hex.EncodeToString(privateKey) + "\n")
	if _, err := ParsePrivateKey(canonical); err != nil {
		t.Fatal(err)
	}
	uppercase := []byte(strings.ToUpper(hex.EncodeToString(privateKey)))
	if _, err := ParsePrivateKey(uppercase); err == nil {
		t.Fatal("private-key parser accepted uppercase hexadecimal")
	}
	inconsistent := slices.Clone(privateKey)
	inconsistent[len(inconsistent)-1] ^= 0xff
	if _, err := ParsePrivateKey([]byte(hex.EncodeToString(inconsistent))); err == nil {
		t.Fatal("private-key parser accepted inconsistent seed and public bytes")
	}
}

func releaseStatementFixture(t *testing.T) (Manifest, []byte, []byte) {
	t.Helper()
	root := releaseAssetFixture(t)
	manifest, err := BuildManifest(root, "0.2.0", strings.Repeat("1", 40), releaseFixtureSourceSHA256(t), "2026-08-12T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	evidence := make([]binaryEvidence, 0, len(canonicalBinaryPaths))
	for _, binary := range manifest.Assets[:len(canonicalBinaryPaths)] {
		evidence = append(evidence, binaryEvidence{Asset: binary, Info: releaseTestBuildInfo(manifest, binary)})
	}
	document, err := buildSBOM(manifest, evidence)
	if err != nil {
		t.Fatal(err)
	}
	sbomRaw, err := EncodeSBOM(document, manifest)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, manifestRaw, sbomRaw
}

func releaseTestBuildInfo(manifest Manifest, binary Asset) debug.BuildInfo {
	target := canonicalBinaryTargets[binary.Path]
	return debug.BuildInfo{
		GoVersion: "go1.26.5", Path: "github.com/Christopher-Schulze/evalwitness/cmd/evalwitness",
		Main:     debug.Module{Path: "github.com/Christopher-Schulze/evalwitness", Version: "(devel)"},
		Deps:     []*debug.Module{{Path: "golang.org/x/sys", Version: "v0.47.0", Sum: "h1:example"}},
		Settings: []debug.BuildSetting{{Key: "-trimpath", Value: "true"}, {Key: "CGO_ENABLED", Value: "0"}, {Key: "GOOS", Value: target[0]}, {Key: "GOARCH", Value: target[1]}},
	}
}

func releaseTestPrivateKey(fill byte) ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = fill
	}
	return ed25519.NewKeyFromSeed(seed)
}
