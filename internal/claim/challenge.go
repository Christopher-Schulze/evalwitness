package claim

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

type ChallengeReceipt struct {
	SchemaVersion           string         `json:"schema_version"`
	Verifier                string         `json:"verifier"`
	ClaimID                 string         `json:"claim_id"`
	SourceCapsuleID         string         `json:"source_capsule_id"`
	SourceManifestDigest    string         `json:"source_manifest_digest"`
	SourceLedgerDigest      string         `json:"source_ledger_digest"`
	ChallengeID             string         `json:"challenge_id"`
	Class                   ChallengeClass `json:"class"`
	Mutation                string         `json:"mutation"`
	ExpectedGuard           Guard          `json:"expected_guard"`
	ObservedGuard           Guard          `json:"observed_guard"`
	BeforeState             string         `json:"before_state"`
	AfterState              string         `json:"after_state"`
	SealedSourceDigest      string         `json:"sealed_source_digest"`
	AfterSealedSourceDigest string         `json:"after_sealed_source_digest"`
	Passed                  bool           `json:"passed"`
	Digest                  string         `json:"digest"`
}

func (receipt ChallengeReceipt) Validate() error {
	if receipt.SchemaVersion != ChallengeReceiptSchemaVersion || receipt.Verifier != VerifierVersion ||
		!validClaimID(receipt.ClaimID) || !validDigest(receipt.SourceCapsuleID) || !validDigest(receipt.SourceManifestDigest) ||
		!validDigest(receipt.SourceLedgerDigest) || !validToken(receipt.ChallengeID) ||
		!validText(receipt.Mutation) || !receipt.ExpectedGuard.Valid() || !receipt.ObservedGuard.Valid() ||
		!validText(receipt.BeforeState) || !validText(receipt.AfterState) ||
		!validDigest(receipt.SealedSourceDigest) || receipt.AfterSealedSourceDigest != receipt.SealedSourceDigest ||
		!receipt.Passed || receipt.ExpectedGuard != receipt.ObservedGuard || !validDigest(receipt.Digest) {
		return errors.New("claim challenge receipt identity or result is invalid")
	}
	if receipt.ChallengeID != strings.ToLower(receipt.ClaimID)+"."+string(receipt.Class) || expectedGuard(receipt.Class) != receipt.ExpectedGuard {
		return errors.New("claim challenge receipt class binding is invalid")
	}
	digest, err := challengeReceiptDigest(receipt)
	if err != nil || digest != receipt.Digest {
		return errors.New("claim challenge receipt digest is invalid")
	}
	return nil
}

func Challenge(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger, claimID, challengeID string) (ChallengeReceipt, error) {
	if err := verifyLedgerPackage(ctx, registry, manifest, payloads, ledger); err != nil {
		return ChallengeReceipt{}, err
	}
	item, err := Lookup(ledger, claimID)
	if err != nil {
		return ChallengeReceipt{}, err
	}
	specification, err := lookupChallenge(item, challengeID)
	if err != nil {
		return ChallengeReceipt{}, err
	}
	if specification.Applicability != ChallengeApplied {
		return ChallengeReceipt{}, fmt.Errorf("challenge %q is inapplicable: %s", challengeID, specification.InapplicableReason)
	}
	evidence := newEvidenceSet(manifest, payloads)
	sealedDigest, err := sealedPackageDigest(registry, manifest, payloads)
	if err != nil {
		return ChallengeReceipt{}, err
	}
	return challengeVerified(ctx, registry, manifest, payloads, ledger, item, specification, evidence, sealedDigest)
}

func challengeVerified(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger, item Claim, specification ChallengeSpec, evidence evidenceSet, sealedDigest string) (ChallengeReceipt, error) {
	before, err := evaluateClaimState(item, evidence, initialVerificationState(item))
	if err != nil {
		return ChallengeReceipt{}, fmt.Errorf("challenge source claim is not normally verifiable: %w", err)
	}
	observed, err := applyChallenge(ctx, registry, manifest, payloads, item, evidence, specification)
	if err != nil {
		return ChallengeReceipt{}, err
	}
	if observed != specification.ExpectedGuard {
		return ChallengeReceipt{}, fmt.Errorf("challenge %q reached guard %q, want %q", specification.ChallengeID, observed, specification.ExpectedGuard)
	}
	receipt := ChallengeReceipt{
		SchemaVersion: ChallengeReceiptSchemaVersion, Verifier: VerifierVersion,
		ClaimID: item.ClaimID, SourceCapsuleID: manifest.CapsuleID,
		SourceManifestDigest: manifest.ManifestDigest, SourceLedgerDigest: ledger.Digest,
		ChallengeID: specification.ChallengeID, Class: specification.Class, Mutation: specification.Mutation,
		ExpectedGuard: specification.ExpectedGuard, ObservedGuard: observed,
		BeforeState: verificationStateLabel(before), AfterState: "rejected:" + string(observed),
		SealedSourceDigest: sealedDigest, AfterSealedSourceDigest: sealedDigest, Passed: true,
	}
	receipt.Digest, err = challengeReceiptDigest(receipt)
	if err != nil {
		return ChallengeReceipt{}, err
	}
	if err := receipt.Validate(); err != nil {
		return ChallengeReceipt{}, err
	}
	return receipt, nil
}

func ChallengeAll(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) ([]ChallengeReceipt, error) {
	if err := verifyLedgerPackage(ctx, registry, manifest, payloads, ledger); err != nil {
		return nil, err
	}
	return challengeAllVerified(ctx, registry, manifest, payloads, ledger)
}

func challengeAllVerified(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) ([]ChallengeReceipt, error) {
	evidence := newEvidenceSet(manifest, payloads)
	sealedDigest, err := sealedPackageDigest(registry, manifest, payloads)
	if err != nil {
		return nil, err
	}
	receipts := make([]ChallengeReceipt, 0, len(ledger.Claims)*len(ChallengeClasses()))
	for _, item := range ledger.Claims {
		for _, specification := range item.Challenges {
			if specification.Applicability != ChallengeApplied {
				continue
			}
			receipt, err := challengeVerified(ctx, registry, manifest, payloads, ledger, item, specification, evidence, sealedDigest)
			if err != nil {
				return nil, err
			}
			receipts = append(receipts, receipt)
		}
	}
	return receipts, nil
}

func applyChallenge(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, item Claim, evidence evidenceSet, specification ChallengeSpec) (Guard, error) {
	if specification.Class == ChallengeDigestSubstitution {
		return applyDigestSubstitution(ctx, registry, manifest, payloads, item)
	}
	state := initialVerificationState(item)
	switch specification.Class {
	case ChallengeParentRemoval:
		state.Available[item.EvidenceComponents[0]] = false
	case ChallengeDenominatorDeletion:
		state.DenominatorPresent = false
	case ChallengeScopeWidening:
		state.ScopeWidened = true
	case ChallengeStaleAttestation:
		state.Attestation = AttestationStale
	case ChallengeCaveatRemoval:
		state.Caveats[item.EvidenceCeiling.RequiredCaveats[0]] = false
	case ChallengeSupersededGeneration:
		state.EvidenceGeneration = "challenge.superseded-generation"
	case ChallengeVisibilityLeak:
		state.VisibilityLeak = true
	default:
		return "", fmt.Errorf("unsupported challenge class %q", specification.Class)
	}
	_, err := evaluateClaimState(item, evidence, state)
	if err == nil {
		return "", errors.New("claim challenge did not trigger a guard")
	}
	var guarded GuardError
	if !errors.As(err, &guarded) {
		return "", fmt.Errorf("claim challenge reached an unintended verifier error: %w", err)
	}
	return guarded.Guard, nil
}

func applyDigestSubstitution(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, item Claim) (Guard, error) {
	cloned := make(map[string][]byte, len(payloads))
	for digest, raw := range payloads {
		cloned[digest] = slices.Clone(raw)
	}
	evidence := newEvidenceSet(manifest, cloned)
	record, found := evidence.records[item.EvidenceComponents[0]]
	if !found {
		return "", errors.New("digest challenge evidence component is absent")
	}
	raw := cloned[record.Payload.Digest]
	if len(raw) == 0 {
		return "", errors.New("digest challenge evidence payload is empty")
	}
	raw[0] ^= 0x01
	_, err := capsule.VerifyPackage(ctx, registry, manifest, cloned, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err == nil {
		return "", errors.New("digest substitution challenge did not invalidate the capsule")
	}
	var mismatch capsule.PayloadIdentityError
	if !errors.As(err, &mismatch) || mismatch.ComponentName != record.Name || mismatch.PayloadDigest != record.Payload.Digest {
		return "", fmt.Errorf("digest substitution reached an unintended capsule error: %w", err)
	}
	return GuardEvidenceDigestMismatch, nil
}

func lookupChallenge(item Claim, challengeID string) (ChallengeSpec, error) {
	for _, specification := range item.Challenges {
		if specification.ChallengeID == challengeID {
			return specification, nil
		}
	}
	return ChallengeSpec{}, fmt.Errorf("challenge %q is absent from claim %q", challengeID, item.ClaimID)
}

func verificationStateLabel(verification ClaimVerification) string {
	if verification.Assertable {
		return "verified:assertable"
	}
	return "verified:historical-or-unsupported"
}

func sealedPackageDigest(registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte) (string, error) {
	type payloadEntry struct {
		DeclaredDigest string `json:"declared_digest"`
		ActualDigest   string `json:"actual_digest"`
		Bytes          int64  `json:"bytes"`
	}
	type sealedPackage struct {
		RegistryDigest string         `json:"registry_digest"`
		ManifestDigest string         `json:"manifest_digest"`
		Payloads       []payloadEntry `json:"payloads"`
	}
	entries := make([]payloadEntry, 0, len(payloads))
	for digest, raw := range payloads {
		entries = append(entries, payloadEntry{DeclaredDigest: digest, ActualDigest: protocol.DigestBytes(raw), Bytes: int64(len(raw))})
	}
	slices.SortFunc(entries, func(left, right payloadEntry) int { return strings.Compare(left.DeclaredDigest, right.DeclaredDigest) })
	return protocol.Digest(sealedPackage{RegistryDigest: registry.Digest(), ManifestDigest: manifest.ManifestDigest, Payloads: entries})
}

func challengeReceiptDigest(receipt ChallengeReceipt) (string, error) {
	receipt.Digest = ""
	return protocol.Digest(receipt)
}
