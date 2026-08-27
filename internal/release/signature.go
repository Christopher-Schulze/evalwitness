package release

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type SignedMaterial struct {
	Envelope  capsule.DSSEEnvelope
	TrustRoot capsule.TrustRoot
	Policy    capsule.SignaturePolicy
}

func SealStatement(manifestRaw, sbomRaw, statementRaw, privateKeyRaw []byte) (SignedMaterial, error) {
	if _, err := DecodeStatement(statementRaw, manifestRaw, sbomRaw); err != nil {
		return SignedMaterial{}, err
	}
	privateKey, err := ParsePrivateKey(privateKeyRaw)
	if err != nil {
		return SignedMaterial{}, err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	root, err := capsule.SealTrustRoot([]ed25519.PublicKey{publicKey})
	if err != nil {
		return SignedMaterial{}, err
	}
	policy := capsule.SignaturePolicy{AllowedKeyIDs: []string{capsule.Ed25519KeyID(publicKey)}, MinimumSignatures: 1}
	envelope, err := capsule.SignPayload(capsule.DSSEPayloadType, statementRaw, privateKey)
	if err != nil {
		return SignedMaterial{}, err
	}
	return SignedMaterial{Envelope: envelope, TrustRoot: root, Policy: policy}, nil
}

func VerifySignedStatement(manifestRaw, sbomRaw, statementRaw, envelopeRaw, trustRootRaw, policyRaw []byte) ([]string, error) {
	if _, err := DecodeStatement(statementRaw, manifestRaw, sbomRaw); err != nil {
		return nil, err
	}
	envelope, err := DecodeEnvelope(envelopeRaw)
	if err != nil {
		return nil, err
	}
	root, err := DecodeTrustRoot(trustRootRaw)
	if err != nil {
		return nil, err
	}
	policy, err := DecodeSignaturePolicy(policyRaw, root)
	if err != nil {
		return nil, err
	}
	return capsule.VerifyPayload(envelope, capsule.DSSEPayloadType, statementRaw, root, policy)
}

func ParsePrivateKey(raw []byte) (ed25519.PrivateKey, error) {
	if len(raw) == ed25519.PrivateKeySize*2+1 && raw[len(raw)-1] == '\n' {
		raw = raw[:len(raw)-1]
	}
	if len(raw) != ed25519.PrivateKeySize*2 || !validLowerHex(string(raw)) {
		return nil, errors.New("Ed25519 private key must be exactly 128 lowercase hexadecimal characters with an optional final newline")
	}
	decoded := make([]byte, ed25519.PrivateKeySize)
	if _, err := hex.Decode(decoded, raw); err != nil {
		return nil, errors.New("Ed25519 private key is invalid")
	}
	derived := ed25519.NewKeyFromSeed(decoded[:ed25519.SeedSize])
	if !bytes.Equal(derived, decoded) {
		return nil, errors.New("Ed25519 private key seed and public key are inconsistent")
	}
	return ed25519.PrivateKey(decoded), nil
}

func EncodeEnvelope(envelope capsule.DSSEEnvelope) ([]byte, error) {
	if envelope.PayloadType != capsule.DSSEPayloadType || envelope.Payload == "" || len(envelope.Signatures) != 1 {
		return nil, errors.New("release DSSE envelope identity is invalid")
	}
	return protocol.CanonicalMarshal(envelope)
}

func DecodeEnvelope(raw []byte) (capsule.DSSEEnvelope, error) {
	var envelope capsule.DSSEEnvelope
	if err := protocol.DecodeStrict(raw, &envelope); err != nil {
		return capsule.DSSEEnvelope{}, fmt.Errorf("decode release DSSE envelope: %w", err)
	}
	canonical, err := EncodeEnvelope(envelope)
	if err != nil || !bytes.Equal(canonical, raw) {
		return capsule.DSSEEnvelope{}, errors.New("release DSSE envelope is not canonical JSON")
	}
	return envelope, nil
}

func EncodeTrustRoot(root capsule.TrustRoot) ([]byte, error) {
	if err := root.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(root)
}

func DecodeTrustRoot(raw []byte) (capsule.TrustRoot, error) {
	var root capsule.TrustRoot
	if err := protocol.DecodeStrict(raw, &root); err != nil {
		return capsule.TrustRoot{}, fmt.Errorf("decode release trust root: %w", err)
	}
	canonical, err := EncodeTrustRoot(root)
	if err != nil || !bytes.Equal(canonical, raw) {
		return capsule.TrustRoot{}, errors.New("release trust root is not canonical JSON")
	}
	return root, nil
}

func EncodeSignaturePolicy(policy capsule.SignaturePolicy, root capsule.TrustRoot) ([]byte, error) {
	if err := policy.Validate(root); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(policy)
}

func DecodeSignaturePolicy(raw []byte, root capsule.TrustRoot) (capsule.SignaturePolicy, error) {
	var policy capsule.SignaturePolicy
	if err := protocol.DecodeStrict(raw, &policy); err != nil {
		return capsule.SignaturePolicy{}, fmt.Errorf("decode release signature policy: %w", err)
	}
	canonical, err := EncodeSignaturePolicy(policy, root)
	if err != nil || !bytes.Equal(canonical, raw) {
		return capsule.SignaturePolicy{}, errors.New("release signature policy is not canonical JSON")
	}
	return policy, nil
}
