package release

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	InTotoStatementType     = "https://in-toto.io/Statement/v1"
	ReleasePredicateType    = "https://evalwitness.dev/attestation/release/v1"
	ReleasePredicateVersion = "evalwitness.in-toto-release-predicate.v1"
	ReleaseManifestSubject  = "release-manifest.json"
	ReleaseSBOMSubject      = "evalwitness.spdx.json"
)

type ReleasePredicate struct {
	SchemaVersion       string      `json:"schema_version"`
	Product             string      `json:"product"`
	ProductVersion      string      `json:"product_version"`
	GitCommit           string      `json:"git_commit"`
	SourceArchiveSHA256 string      `json:"source_archive_sha256"`
	CreatedAt           string      `json:"created_at"`
	ManifestDigest      string      `json:"manifest_digest"`
	SBOMDigest          string      `json:"sbom_digest"`
	AssetCount          int         `json:"asset_count"`
	TotalBytes          int64       `json:"total_bytes"`
	Truth               TruthStatus `json:"truth"`
}

type Statement struct {
	Type          string                  `json:"_type"`
	Subject       []capsule.InTotoSubject `json:"subject"`
	PredicateType string                  `json:"predicateType"`
	Predicate     ReleasePredicate        `json:"predicate"`
}

func BuildStatement(manifestRaw, sbomRaw []byte) (Statement, error) {
	manifest, err := DecodeManifest(manifestRaw)
	if err != nil {
		return Statement{}, err
	}
	if _, err := DecodeSBOM(sbomRaw, manifest); err != nil {
		return Statement{}, err
	}
	statement := Statement{
		Type: InTotoStatementType,
		Subject: []capsule.InTotoSubject{
			{Name: ReleaseManifestSubject, Digest: map[string]string{"sha256": protocol.DigestBytes(manifestRaw)}},
			{Name: ReleaseSBOMSubject, Digest: map[string]string{"sha256": protocol.DigestBytes(sbomRaw)}},
		},
		PredicateType: ReleasePredicateType,
		Predicate: ReleasePredicate{
			SchemaVersion: ReleasePredicateVersion, Product: manifest.Product, ProductVersion: manifest.ProductVersion,
			GitCommit: manifest.GitCommit, SourceArchiveSHA256: manifest.SourceArchiveSHA256, CreatedAt: manifest.CreatedAt,
			ManifestDigest: protocol.DigestBytes(manifestRaw), SBOMDigest: protocol.DigestBytes(sbomRaw),
			AssetCount: manifest.AssetCount, TotalBytes: manifest.TotalBytes, Truth: manifest.Truth,
		},
	}
	if err := statement.Validate(manifestRaw, sbomRaw); err != nil {
		return Statement{}, err
	}
	return statement, nil
}

func (s Statement) Validate(manifestRaw, sbomRaw []byte) error {
	manifest, err := DecodeManifest(manifestRaw)
	if err != nil {
		return err
	}
	if _, err := DecodeSBOM(sbomRaw, manifest); err != nil {
		return err
	}
	expectedSubjects := []capsule.InTotoSubject{
		{Name: ReleaseManifestSubject, Digest: map[string]string{"sha256": protocol.DigestBytes(manifestRaw)}},
		{Name: ReleaseSBOMSubject, Digest: map[string]string{"sha256": protocol.DigestBytes(sbomRaw)}},
	}
	if s.Type != InTotoStatementType || s.PredicateType != ReleasePredicateType || !slices.EqualFunc(s.Subject, expectedSubjects, equalSubject) {
		return errors.New("in-toto release statement identity is invalid")
	}
	predicate := s.Predicate
	if predicate.SchemaVersion != ReleasePredicateVersion || predicate.Product != manifest.Product || predicate.ProductVersion != manifest.ProductVersion ||
		predicate.GitCommit != manifest.GitCommit || predicate.SourceArchiveSHA256 != manifest.SourceArchiveSHA256 || predicate.CreatedAt != manifest.CreatedAt ||
		predicate.ManifestDigest != protocol.DigestBytes(manifestRaw) || predicate.SBOMDigest != protocol.DigestBytes(sbomRaw) ||
		predicate.AssetCount != manifest.AssetCount || predicate.TotalBytes != manifest.TotalBytes || predicate.Truth != manifest.Truth {
		return errors.New("in-toto release predicate does not match the manifest and SBOM")
	}
	return nil
}

func EncodeStatement(statement Statement, manifestRaw, sbomRaw []byte) ([]byte, error) {
	if err := statement.Validate(manifestRaw, sbomRaw); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(statement)
}

func DecodeStatement(raw, manifestRaw, sbomRaw []byte) (Statement, error) {
	var statement Statement
	if err := protocol.DecodeStrict(raw, &statement); err != nil {
		return Statement{}, fmt.Errorf("decode in-toto release statement: %w", err)
	}
	canonical, err := EncodeStatement(statement, manifestRaw, sbomRaw)
	if err != nil || !bytes.Equal(canonical, raw) {
		return Statement{}, errors.New("in-toto release statement is not canonical JSON")
	}
	return statement, nil
}

func equalSubject(left, right capsule.InTotoSubject) bool {
	if left.Name != right.Name || len(left.Digest) != len(right.Digest) {
		return false
	}
	for algorithm, digest := range left.Digest {
		if right.Digest[algorithm] != digest {
			return false
		}
	}
	return true
}
