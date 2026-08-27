package capsule

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	DSSEPayloadType            = "application/vnd.in-toto+json"
	TrustRootSchemaVersion     = "evalwitness.dsse-trusted-keys.v1"
	DSSEVerificationVersion    = "evalwitness.dsse-verification.v1"
	SigstoreBundleMediaTypeV03 = "application/vnd.dev.sigstore.bundle.v0.3+json"
	ExplicitEd25519TrustMode   = "explicit_ed25519_keys"
)

type DSSESignature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

type DSSEEnvelope struct {
	PayloadType string          `json:"payloadType"`
	Payload     string          `json:"payload"`
	Signatures  []DSSESignature `json:"signatures"`
}

type TrustedKey struct {
	KeyID     string `json:"key_id"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
}

type TrustRoot struct {
	SchemaVersion string       `json:"schema_version"`
	Keys          []TrustedKey `json:"keys"`
	Digest        string       `json:"digest"`
}

type SignaturePolicy struct {
	AllowedKeyIDs     []string `json:"allowed_key_ids"`
	MinimumSignatures int      `json:"minimum_signatures"`
}

type DSSEVerificationReport struct {
	SchemaVersion   string   `json:"schema_version"`
	CapsuleID       string   `json:"capsule_id"`
	PayloadDigest   string   `json:"payload_digest"`
	TrustRootDigest string   `json:"trust_root_digest"`
	TrustMode       string   `json:"trust_mode"`
	VerifiedKeyIDs  []string `json:"verified_key_ids"`
	Valid           bool     `json:"valid"`
}

type SigstoreBundle struct {
	MediaType            string          `json:"mediaType"`
	VerificationMaterial json.RawMessage `json:"verificationMaterial"`
	DSSEEnvelope         DSSEEnvelope    `json:"dsseEnvelope"`
}

func SealTrustRoot(publicKeys []ed25519.PublicKey) (TrustRoot, error) {
	root := TrustRoot{SchemaVersion: TrustRootSchemaVersion}
	for _, publicKey := range publicKeys {
		if len(publicKey) != ed25519.PublicKeySize {
			return TrustRoot{}, errors.New("invalid Ed25519 public key")
		}
		root.Keys = append(root.Keys, TrustedKey{
			KeyID: Ed25519KeyID(publicKey), Algorithm: "ed25519",
			PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		})
	}
	slices.SortFunc(root.Keys, func(left, right TrustedKey) int { return strings.Compare(left.KeyID, right.KeyID) })
	digest, err := trustRootDigest(root)
	if err != nil {
		return TrustRoot{}, err
	}
	root.Digest = digest
	if err := root.Validate(); err != nil {
		return TrustRoot{}, err
	}
	return root, nil
}

func (r TrustRoot) Validate() error {
	if r.SchemaVersion != TrustRootSchemaVersion || !validDigest(r.Digest) || len(r.Keys) == 0 {
		return errors.New("DSSE trust root identity is invalid")
	}
	previous := ""
	for _, key := range r.Keys {
		publicKey, err := decodeTrustedKey(key)
		if err != nil || key.KeyID <= previous || key.KeyID != Ed25519KeyID(publicKey) {
			return errors.New("DSSE trust root keys are invalid, duplicated, or unsorted")
		}
		previous = key.KeyID
	}
	digest, err := trustRootDigest(r)
	if err != nil || digest != r.Digest {
		return errors.New("DSSE trust root digest is invalid")
	}
	return nil
}

func SignStatement(statement InTotoStatement, manifest Manifest, privateKey ed25519.PrivateKey) (DSSEEnvelope, error) {
	if err := statement.Validate(manifest); err != nil {
		return DSSEEnvelope{}, err
	}
	payload, err := EncodeStatement(statement)
	if err != nil {
		return DSSEEnvelope{}, err
	}
	return SignPayload(DSSEPayloadType, payload, privateKey)
}

func SignPayload(payloadType string, payload []byte, privateKey ed25519.PrivateKey) (DSSEEnvelope, error) {
	if payloadType == "" || len(payload) == 0 {
		return DSSEEnvelope{}, errors.New("DSSE payload type and bytes are required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return DSSEEnvelope{}, errors.New("invalid Ed25519 private key")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	signature := ed25519.Sign(privateKey, DSSEPAE(payloadType, payload))
	return DSSEEnvelope{
		PayloadType: payloadType, Payload: base64.StdEncoding.EncodeToString(payload),
		Signatures: []DSSESignature{{KeyID: Ed25519KeyID(publicKey), Sig: base64.StdEncoding.EncodeToString(signature)}},
	}, nil
}

func VerifyEnvelope(envelope DSSEEnvelope, manifest Manifest, root TrustRoot, policy SignaturePolicy) (DSSEVerificationReport, error) {
	if envelope.PayloadType != DSSEPayloadType || len(envelope.Signatures) == 0 {
		return DSSEVerificationReport{}, errors.New("DSSE envelope identity is invalid")
	}
	if err := root.Validate(); err != nil {
		return DSSEVerificationReport{}, err
	}
	if err := policy.Validate(root); err != nil {
		return DSSEVerificationReport{}, err
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(envelope.Payload)
	if err != nil {
		return DSSEVerificationReport{}, errors.New("DSSE payload is not strict base64")
	}
	if _, err := DecodeStatement(payload, manifest); err != nil {
		return DSSEVerificationReport{}, err
	}
	verified, err := VerifyPayload(envelope, DSSEPayloadType, payload, root, policy)
	if err != nil {
		return DSSEVerificationReport{}, err
	}
	return DSSEVerificationReport{
		SchemaVersion: DSSEVerificationVersion, CapsuleID: manifest.CapsuleID,
		PayloadDigest: protocol.DigestBytes(payload), TrustRootDigest: root.Digest,
		TrustMode: ExplicitEd25519TrustMode, VerifiedKeyIDs: verified, Valid: true,
	}, nil
}

func VerifyPayload(envelope DSSEEnvelope, expectedPayloadType string, expectedPayload []byte, root TrustRoot, policy SignaturePolicy) ([]string, error) {
	if expectedPayloadType == "" || len(expectedPayload) == 0 || envelope.PayloadType != expectedPayloadType || len(envelope.Signatures) == 0 {
		return nil, errors.New("DSSE envelope identity is invalid")
	}
	if err := root.Validate(); err != nil {
		return nil, err
	}
	if err := policy.Validate(root); err != nil {
		return nil, err
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(envelope.Payload)
	if err != nil {
		return nil, errors.New("DSSE payload is not strict base64")
	}
	if !bytes.Equal(payload, expectedPayload) {
		return nil, errors.New("DSSE payload differs from the expected bytes")
	}
	return verifySignatures(envelope, payload, root, policy)
}

func VerifySigstoreBundleWithExplicitKeys(bundle SigstoreBundle, manifest Manifest, root TrustRoot, policy SignaturePolicy) (DSSEVerificationReport, error) {
	if bundle.MediaType != SigstoreBundleMediaTypeV03 || len(bundle.DSSEEnvelope.Signatures) != 1 {
		return DSSEVerificationReport{}, errors.New("sigstore DSSE bundle v0.3 identity is invalid")
	}
	hint, err := explicitSigstoreKeyHint(bundle.VerificationMaterial)
	if err != nil {
		return DSSEVerificationReport{}, err
	}
	if hint != bundle.DSSEEnvelope.Signatures[0].KeyID {
		return DSSEVerificationReport{}, errors.New("sigstore public-key hint differs from the DSSE signature key ID")
	}
	return VerifyEnvelope(bundle.DSSEEnvelope, manifest, root, policy)
}

func explicitSigstoreKeyHint(raw json.RawMessage) (string, error) {
	type publicKeyIdentifier struct {
		Hint string `json:"hint"`
	}
	var material struct {
		PublicKey                 *publicKeyIdentifier `json:"publicKey,omitempty"`
		X509CertificateChain      json.RawMessage      `json:"x509CertificateChain,omitempty"`
		Certificate               json.RawMessage      `json:"certificate,omitempty"`
		TlogEntries               []json.RawMessage    `json:"tlogEntries,omitempty"`
		TimestampVerificationData json.RawMessage      `json:"timestampVerificationData,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&material); err != nil {
		return "", errors.New("sigstore verification material is invalid")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err == nil {
		return "", errors.New("sigstore verification material contains trailing JSON")
	} else if !errors.Is(err, io.EOF) {
		return "", errors.New("sigstore verification material is invalid")
	}
	if material.PublicKey == nil || !validKeyID(material.PublicKey.Hint) || len(material.X509CertificateChain) != 0 ||
		len(material.Certificate) != 0 || len(material.TlogEntries) != 0 || len(material.TimestampVerificationData) != 0 {
		return "", errors.New("explicit-key Sigstore verification requires only one out-of-band public-key hint")
	}
	return material.PublicKey.Hint, nil
}

func (p SignaturePolicy) Validate(root TrustRoot) error {
	if p.MinimumSignatures < 1 || p.MinimumSignatures > len(p.AllowedKeyIDs) {
		return errors.New("DSSE signature policy threshold is invalid")
	}
	trusted := make(map[string]struct{}, len(root.Keys))
	for _, key := range root.Keys {
		trusted[key.KeyID] = struct{}{}
	}
	previous := ""
	for _, keyID := range p.AllowedKeyIDs {
		if !validKeyID(keyID) || keyID <= previous {
			return errors.New("DSSE policy key IDs must be valid, unique, and sorted")
		}
		if _, found := trusted[keyID]; !found {
			return fmt.Errorf("DSSE policy key %q is not in the trust root", keyID)
		}
		previous = keyID
	}
	return nil
}

func verifySignatures(envelope DSSEEnvelope, payload []byte, root TrustRoot, policy SignaturePolicy) ([]string, error) {
	trusted := trustedKeys(root)
	allowed := make(map[string]struct{}, len(policy.AllowedKeyIDs))
	for _, keyID := range policy.AllowedKeyIDs {
		allowed[keyID] = struct{}{}
	}
	verified := make([]string, 0, len(envelope.Signatures))
	seen := make(map[string]struct{}, len(envelope.Signatures))
	pae := DSSEPAE(envelope.PayloadType, payload)
	for _, signature := range envelope.Signatures {
		if _, duplicate := seen[signature.KeyID]; duplicate {
			return nil, fmt.Errorf("duplicate DSSE signature key %q", signature.KeyID)
		}
		seen[signature.KeyID] = struct{}{}
		publicKey, trustedKey := trusted[signature.KeyID]
		_, allowedKey := allowed[signature.KeyID]
		raw, err := base64.StdEncoding.Strict().DecodeString(signature.Sig)
		if !trustedKey || !allowedKey || err != nil || len(raw) != ed25519.SignatureSize || !ed25519.Verify(publicKey, pae, raw) {
			return nil, fmt.Errorf("DSSE signature for key %q is invalid or untrusted", signature.KeyID)
		}
		verified = append(verified, signature.KeyID)
	}
	slices.Sort(verified)
	if len(verified) < policy.MinimumSignatures {
		return nil, errors.New("DSSE signature threshold was not met")
	}
	return verified, nil
}

func trustedKeys(root TrustRoot) map[string]ed25519.PublicKey {
	keys := make(map[string]ed25519.PublicKey, len(root.Keys))
	for _, key := range root.Keys {
		publicKey, err := decodeTrustedKey(key)
		if err == nil {
			keys[key.KeyID] = publicKey
		}
	}
	return keys
}

func decodeTrustedKey(key TrustedKey) (ed25519.PublicKey, error) {
	if key.Algorithm != "ed25519" || !validKeyID(key.KeyID) {
		return nil, errors.New("trusted key identity is invalid")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(key.PublicKey)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("trusted Ed25519 key is invalid")
	}
	return ed25519.PublicKey(raw), nil
}

func trustRootDigest(root TrustRoot) (string, error) {
	copy := root
	copy.Digest = ""
	return protocol.Digest(copy)
}

func Ed25519KeyID(publicKey ed25519.PublicKey) string {
	return DigestAlgorithm + ":" + protocol.DigestBytes(publicKey)
}

func validKeyID(value string) bool {
	return strings.HasPrefix(value, DigestAlgorithm+":") && validDigest(strings.TrimPrefix(value, DigestAlgorithm+":"))
}

func DSSEPAE(payloadType string, payload []byte) []byte {
	var output bytes.Buffer
	output.WriteString("DSSEv1 ")
	output.WriteString(strconv.Itoa(len(payloadType)))
	output.WriteByte(' ')
	output.WriteString(payloadType)
	output.WriteByte(' ')
	output.WriteString(strconv.Itoa(len(payload)))
	output.WriteByte(' ')
	output.Write(payload)
	return output.Bytes()
}
