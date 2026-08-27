package conformance

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const (
	PublicAttestationSchemaVersion = "evalwitness.public-attestation.v1"
	MaximumChallengeLifetime       = 15 * time.Minute
)

type FreshnessChallenge struct {
	Purpose   string    `json:"purpose"`
	Nonce     string    `json:"nonce"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PublicIdentity struct {
	ProviderID                string `json:"provider_id"`
	RouteID                   string `json:"route_id"`
	EndpointDigest            string `json:"endpoint_digest"`
	DisclosedEndpoint         string `json:"disclosed_endpoint,omitempty"`
	RequestedModel            string `json:"requested_model"`
	ServedModel               string `json:"served_model,omitempty"`
	CheckpointAssertion       string `json:"checkpoint_assertion,omitempty"`
	CheckpointAssertionSource string `json:"checkpoint_assertion_source,omitempty"`
	IdentityAuthority         string `json:"identity_authority"`
}

type PublicObservation struct {
	EvidenceStrength   verifier.EvidenceStrength `json:"evidence_strength"`
	HasLogprobs        bool                      `json:"has_logprobs"`
	DegenerateLogprobs bool                      `json:"degenerate_logprobs"`
	ObservedTopK       int                       `json:"observed_top_k"`
	StreamingObserved  bool                      `json:"streaming_observed"`
	UsageObserved      bool                      `json:"usage_observed"`
	FinishReason       string                    `json:"finish_reason,omitempty"`
}

type PublicAttestation struct {
	SchemaVersion       string             `json:"schema_version"`
	SourceAttestationID string             `json:"source_attestation_id"`
	SourceDigest        string             `json:"source_digest"`
	State               RouteState         `json:"state"`
	EvidenceCeiling     RouteState         `json:"evidence_ceiling"`
	Identity            PublicIdentity     `json:"identity"`
	Contract            RequestContract    `json:"contract"`
	Observation         PublicObservation  `json:"observation"`
	ObservedAt          time.Time          `json:"observed_at"`
	ExpiresAt           time.Time          `json:"expires_at"`
	Limitations         []string           `json:"limitations"`
	Challenge           FreshnessChallenge `json:"challenge"`
	SignerKeyID         string             `json:"signer_key_id"`
	SignerPublicKey     string             `json:"signer_public_key"`
	Signature           string             `json:"signature"`
}

type PublicOptions struct {
	DisclosedEndpoint string
	Challenge         FreshnessChallenge
}

type PublicExpectation struct {
	RouteID          string
	SourceDigest     string
	MinimumState     RouteState
	ChallengePurpose string
}

func DerivePublic(attestation Attestation, options PublicOptions, privateKey ed25519.PrivateKey) (PublicAttestation, error) {
	if err := attestation.ValidateIntegrity(); err != nil {
		return PublicAttestation{}, err
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return PublicAttestation{}, errors.New("Ed25519 private key is invalid")
	}
	if err := validateChallenge(options.Challenge, options.Challenge.IssuedAt); err != nil {
		return PublicAttestation{}, err
	}
	endpointHash := sha256.Sum256([]byte(attestation.Identity.BaseOrigin + attestation.Identity.EndpointPath))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	keyHash := sha256.Sum256(publicKey)
	result := PublicAttestation{
		SchemaVersion:       PublicAttestationSchemaVersion,
		SourceAttestationID: attestation.AttestationID,
		SourceDigest:        attestation.AttestationDigest,
		State:               attestation.State,
		EvidenceCeiling:     attestation.State,
		Identity: PublicIdentity{
			ProviderID:                attestation.Identity.ProviderID,
			RouteID:                   attestation.Identity.RouteID,
			EndpointDigest:            hex.EncodeToString(endpointHash[:]),
			DisclosedEndpoint:         strings.TrimSpace(options.DisclosedEndpoint),
			RequestedModel:            attestation.Identity.RequestedModel,
			ServedModel:               attestation.Identity.ServedModel,
			CheckpointAssertion:       attestation.Identity.CheckpointAssertion,
			CheckpointAssertionSource: attestation.Identity.CheckpointAssertionSource,
			IdentityAuthority:         "independent_signer_claim",
		},
		Contract: attestation.Contract,
		Observation: PublicObservation{
			EvidenceStrength:   attestation.Observation.EvidenceStrength,
			HasLogprobs:        attestation.Observation.HasLogprobs,
			DegenerateLogprobs: attestation.Observation.DegenerateLogprobs,
			ObservedTopK:       attestation.Observation.ObservedTopK,
			StreamingObserved:  attestation.Observation.StreamingObserved,
			UsageObserved:      attestation.Observation.UsageObserved,
			FinishReason:       attestation.Observation.FinishReason,
		},
		ObservedAt:      attestation.ObservedAt,
		ExpiresAt:       attestation.ExpiresAt,
		Limitations:     append([]string(nil), attestation.Limitations...),
		Challenge:       options.Challenge,
		SignerKeyID:     "ed25519-" + hex.EncodeToString(keyHash[:]),
		SignerPublicKey: hex.EncodeToString(publicKey),
	}
	payload, err := result.signingBytes()
	if err != nil {
		return PublicAttestation{}, err
	}
	result.Signature = hex.EncodeToString(ed25519.Sign(privateKey, payload))
	return result, nil
}

func (p PublicAttestation) Verify(now time.Time, expected PublicExpectation) error {
	if p.SchemaVersion != PublicAttestationSchemaVersion || !p.State.Valid() || !p.EvidenceCeiling.Valid() {
		return errors.New("public attestation schema or state is invalid")
	}
	if p.State.EvidenceRank() > p.EvidenceCeiling.EvidenceRank() {
		return errors.New("public attestation exceeds its evidence ceiling")
	}
	if expected.RouteID != "" && p.Identity.RouteID != expected.RouteID {
		return errors.New("public attestation route mismatch")
	}
	if expected.SourceDigest != "" && p.SourceDigest != expected.SourceDigest {
		return errors.New("public attestation source mismatch")
	}
	if expected.MinimumState.Valid() && p.State.EvidenceRank() < expected.MinimumState.EvidenceRank() {
		return fmt.Errorf("public attestation state %s is below required %s", p.State, expected.MinimumState)
	}
	if expected.ChallengePurpose != "" && p.Challenge.Purpose != expected.ChallengePurpose {
		return errors.New("public attestation challenge purpose mismatch")
	}
	if err := validateChallenge(p.Challenge, now.UTC()); err != nil {
		return err
	}
	if !p.ExpiresAt.IsZero() && !now.UTC().Before(p.ExpiresAt) {
		return errors.New("source attestation is expired")
	}
	publicKey, err := hex.DecodeString(p.SignerPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return errors.New("public attestation signer key is invalid")
	}
	keyHash := sha256.Sum256(publicKey)
	if p.SignerKeyID != "ed25519-"+hex.EncodeToString(keyHash[:]) {
		return errors.New("public attestation signer key ID mismatch")
	}
	signature, err := hex.DecodeString(p.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("public attestation signature encoding is invalid")
	}
	payload, err := p.signingBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), payload, signature) {
		return errors.New("public attestation signature is invalid")
	}
	return nil
}

func validateChallenge(challenge FreshnessChallenge, now time.Time) error {
	switch {
	case strings.TrimSpace(challenge.Purpose) == "" || strings.TrimSpace(challenge.Nonce) == "":
		return errors.New("freshness challenge purpose and nonce are required")
	case challenge.IssuedAt.IsZero() || challenge.ExpiresAt.IsZero() || !challenge.ExpiresAt.After(challenge.IssuedAt):
		return errors.New("freshness challenge interval is invalid")
	case challenge.ExpiresAt.Sub(challenge.IssuedAt) > MaximumChallengeLifetime:
		return errors.New("freshness challenge lifetime exceeds the maximum")
	case now.Before(challenge.IssuedAt):
		return errors.New("freshness challenge is not active yet")
	case !now.Before(challenge.ExpiresAt):
		return errors.New("freshness challenge is expired")
	}
	return nil
}

func (p PublicAttestation) signingBytes() ([]byte, error) {
	p.Signature = ""
	encoded, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("encode public attestation: %w", err)
	}
	return encoded, nil
}
