package capsule

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestDSSEStatementVerificationWithExplicitTrustRoot(t *testing.T) {
	registry, manifest, _ := testPublicPackage(t)
	statement, err := BuildStatement(manifest, registry)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := deterministicPrivateKey(1)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	root, err := SealTrustRoot([]ed25519.PublicKey{publicKey})
	if err != nil {
		t.Fatal(err)
	}
	policy := SignaturePolicy{AllowedKeyIDs: []string{Ed25519KeyID(publicKey)}, MinimumSignatures: 1}
	envelope, err := SignStatement(statement, manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	report, err := VerifyEnvelope(envelope, manifest, root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || report.CapsuleID != manifest.CapsuleID || report.TrustMode != ExplicitEd25519TrustMode ||
		!slices.Equal(report.VerifiedKeyIDs, policy.AllowedKeyIDs) {
		t.Fatalf("DSSE verification report = %+v", report)
	}

	tampered := envelope
	tampered.Signatures = slices.Clone(envelope.Signatures)
	signature, err := base64.StdEncoding.DecodeString(tampered.Signatures[0].Sig)
	if err != nil {
		t.Fatal(err)
	}
	signature[0] ^= 0xff
	tampered.Signatures[0].Sig = base64.StdEncoding.EncodeToString(signature)
	if _, err := VerifyEnvelope(tampered, manifest, root, policy); err == nil {
		t.Fatal("DSSE verifier accepted a modified signature")
	}
}

func TestDSSERejectsWrongTrustAndPayloadType(t *testing.T) {
	registry, manifest, _ := testPublicPackage(t)
	statement, err := BuildStatement(manifest, registry)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := deterministicPrivateKey(2)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	envelope, err := SignStatement(statement, manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey := deterministicPrivateKey(3).Public().(ed25519.PublicKey)
	wrongRoot, err := SealTrustRoot([]ed25519.PublicKey{wrongKey})
	if err != nil {
		t.Fatal(err)
	}
	wrongPolicy := SignaturePolicy{AllowedKeyIDs: []string{Ed25519KeyID(wrongKey)}, MinimumSignatures: 1}
	if _, err := VerifyEnvelope(envelope, manifest, wrongRoot, wrongPolicy); err == nil {
		t.Fatal("DSSE verifier accepted an untrusted signer")
	}

	root, err := SealTrustRoot([]ed25519.PublicKey{publicKey})
	if err != nil {
		t.Fatal(err)
	}
	policy := SignaturePolicy{AllowedKeyIDs: []string{Ed25519KeyID(publicKey)}, MinimumSignatures: 1}
	wrongType := envelope
	wrongType.PayloadType = "application/json"
	if _, err := VerifyEnvelope(wrongType, manifest, root, policy); err == nil {
		t.Fatal("DSSE verifier accepted a substituted payload type")
	}
}

func TestSigstoreBundleExplicitKeyPathRequiresV03AndOneSignature(t *testing.T) {
	registry, manifest, _ := testPublicPackage(t)
	statement, err := BuildStatement(manifest, registry)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := deterministicPrivateKey(4)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	root, err := SealTrustRoot([]ed25519.PublicKey{publicKey})
	if err != nil {
		t.Fatal(err)
	}
	policy := SignaturePolicy{AllowedKeyIDs: []string{Ed25519KeyID(publicKey)}, MinimumSignatures: 1}
	envelope, err := SignStatement(statement, manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	bundle := SigstoreBundle{
		MediaType:            SigstoreBundleMediaTypeV03,
		VerificationMaterial: json.RawMessage(`{"publicKey":{"hint":"` + Ed25519KeyID(publicKey) + `"}}`),
		DSSEEnvelope:         envelope,
	}
	if _, err := VerifySigstoreBundleWithExplicitKeys(bundle, manifest, root, policy); err != nil {
		t.Fatal(err)
	}
	bundle.DSSEEnvelope.Signatures = append(bundle.DSSEEnvelope.Signatures, envelope.Signatures[0])
	if _, err := VerifySigstoreBundleWithExplicitKeys(bundle, manifest, root, policy); err == nil {
		t.Fatal("Sigstore v0.3 DSSE bundle accepted more than one signature")
	}

	bundle.DSSEEnvelope = envelope
	bundle.VerificationMaterial = json.RawMessage(`{"publicKey":{"hint":"sha256:` + strings.Repeat("0", 64) + `"}}`)
	if _, err := VerifySigstoreBundleWithExplicitKeys(bundle, manifest, root, policy); err == nil {
		t.Fatal("Sigstore v0.3 DSSE bundle accepted a public-key hint mismatch")
	}
	bundle.VerificationMaterial = json.RawMessage(`{"publicKey":{"hint":"` + Ed25519KeyID(publicKey) + `"},"tlogEntries":[{}]}`)
	if _, err := VerifySigstoreBundleWithExplicitKeys(bundle, manifest, root, policy); err == nil {
		t.Fatal("explicit-key verification silently ignored transparency-log material")
	}
}

func TestDSSEPAEUsesByteLengths(t *testing.T) {
	if got := string(DSSEPAE("foo", []byte("bar"))); got != "DSSEv1 3 foo 3 bar" {
		t.Fatalf("DSSE PAE = %q", got)
	}
	if got := string(DSSEPAE("é", []byte("ß"))); got != "DSSEv1 2 é 2 ß" {
		t.Fatalf("DSSE PAE UTF-8 lengths = %q", got)
	}
}

func TestGenericDSSEPayloadRoundTripAndSubstitutionRejection(t *testing.T) {
	privateKey := deterministicPrivateKey(5)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	root, err := SealTrustRoot([]ed25519.PublicKey{publicKey})
	if err != nil {
		t.Fatal(err)
	}
	policy := SignaturePolicy{AllowedKeyIDs: []string{Ed25519KeyID(publicKey)}, MinimumSignatures: 1}
	payload := []byte(`{"schema_version":"example.v1"}`)
	envelope, err := SignPayload("application/example+json", payload, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyPayload(envelope, "application/example+json", payload, root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(verified, policy.AllowedKeyIDs) {
		t.Fatalf("verified keys = %v", verified)
	}
	if _, err := VerifyPayload(envelope, "application/example+json", []byte(`{}`), root, policy); err == nil {
		t.Fatal("generic DSSE verifier accepted substituted payload bytes")
	}
	if _, err := VerifyPayload(envelope, "application/other+json", payload, root, policy); err == nil {
		t.Fatal("generic DSSE verifier accepted substituted payload type")
	}
}

func deterministicPrivateKey(fill byte) ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = fill
	}
	return ed25519.NewKeyFromSeed(seed)
}
