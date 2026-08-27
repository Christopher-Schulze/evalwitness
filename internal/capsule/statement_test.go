package capsule

import (
	"slices"
	"testing"
)

func TestInTotoStatementBindsScientificCapsule(t *testing.T) {
	registry, manifest, _ := testPublicPackage(t)
	statement, err := BuildStatement(manifest, registry)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeStatement(statement)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeStatement(raw, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Subject[0].Digest[DigestAlgorithm] != manifest.CapsuleID || decoded.Predicate.ManifestDigest != manifest.ManifestDigest {
		t.Fatal("in-toto statement lost scientific or distribution identity")
	}

	tampered := decoded
	tampered.Predicate.ScientificRoots = slices.Clone(tampered.Predicate.ScientificRoots)
	tampered.Predicate.ScientificRoots[0] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := tampered.Validate(manifest); err == nil {
		t.Fatal("in-toto statement accepted a substituted scientific root")
	}
}

func TestInTotoStatementRejectsNonCanonicalEncoding(t *testing.T) {
	registry, manifest, _ := testPublicPackage(t)
	statement, err := BuildStatement(manifest, registry)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeStatement(statement)
	if err != nil {
		t.Fatal(err)
	}
	nonCanonical := append([]byte(" "), raw...)
	if _, err := DecodeStatement(nonCanonical, manifest); err == nil {
		t.Fatal("in-toto statement accepted non-canonical JSON")
	}
}
