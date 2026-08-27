package claim

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
)

const VerificationSchemaVersion = "evalwitness.claim-verification.v1"

type Guard string

const (
	GuardEvidenceParentMissing        Guard = "evidence-parent-missing"
	GuardEvidenceDigestMismatch       Guard = "evidence-digest-mismatch"
	GuardExpressionDenominatorMissing Guard = "expression-denominator-missing"
	GuardScopeExceedsLedger           Guard = "scope-exceeds-ledger"
	GuardAttestationStale             Guard = "attestation-stale"
	GuardRequiredCaveatMissing        Guard = "required-caveat-missing"
	GuardGenerationNotCurrent         Guard = "generation-not-current"
	GuardVisibilityLeak               Guard = "visibility-leak"
)

func (guard Guard) Valid() bool {
	switch guard {
	case GuardEvidenceParentMissing, GuardEvidenceDigestMismatch, GuardExpressionDenominatorMissing,
		GuardScopeExceedsLedger, GuardAttestationStale, GuardRequiredCaveatMissing,
		GuardGenerationNotCurrent, GuardVisibilityLeak:
		return true
	default:
		return false
	}
}

type GuardError struct {
	Guard   Guard
	Message string
}

func (failure GuardError) Error() string {
	return string(failure.Guard) + ": " + failure.Message
}

type ClaimVerification struct {
	ClaimID           string        `json:"claim_id"`
	Status            Status        `json:"status"`
	EvidenceLevel     EvidenceLevel `json:"evidence_level"`
	ExpressionValue   ExactValue    `json:"expression_value"`
	ExpressionMatches bool          `json:"expression_matches"`
	Assertable        bool          `json:"assertable"`
	Valid             bool          `json:"valid"`
}

type VerificationReport struct {
	SchemaVersion string              `json:"schema_version"`
	Verifier      string              `json:"verifier"`
	CapsuleID     string              `json:"capsule_id"`
	LedgerDigest  string              `json:"ledger_digest"`
	Claims        []ClaimVerification `json:"claims"`
	StatusCounts  map[string]int      `json:"status_counts"`
	Valid         bool                `json:"valid"`
	Offline       bool                `json:"offline"`
}

type verificationState struct {
	Available          map[string]bool
	Caveats            map[string]bool
	DenominatorPresent bool
	ScopeWidened       bool
	Attestation        AttestationState
	EvidenceGeneration string
	VisibilityLeak     bool
}

func VerifyLedger(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) (VerificationReport, error) {
	if err := verifyLedgerPackage(ctx, registry, manifest, payloads, ledger); err != nil {
		return VerificationReport{}, err
	}
	return verifyLedgerClaims(manifest, payloads, ledger)
}

func verifyLedgerClaims(manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) (VerificationReport, error) {
	evidence := newEvidenceSet(manifest, payloads)
	report := VerificationReport{
		SchemaVersion: VerificationSchemaVersion, Verifier: VerifierVersion,
		CapsuleID: manifest.CapsuleID, LedgerDigest: ledger.Digest,
		Claims: make([]ClaimVerification, 0, len(ledger.Claims)), StatusCounts: map[string]int{},
		Valid: true, Offline: true,
	}
	for _, item := range ledger.Claims {
		verification, err := evaluateClaimState(item, evidence, initialVerificationState(item))
		if err != nil {
			return VerificationReport{}, fmt.Errorf("verify claim %q: %w", item.ClaimID, err)
		}
		report.Claims = append(report.Claims, verification)
		report.StatusCounts[string(item.Status)]++
	}
	return report, nil
}

func VerifyClaim(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger, claimID string) (ClaimVerification, error) {
	if err := verifyLedgerPackage(ctx, registry, manifest, payloads, ledger); err != nil {
		return ClaimVerification{}, err
	}
	item, err := Lookup(ledger, claimID)
	if err != nil {
		return ClaimVerification{}, err
	}
	return evaluateClaimState(item, newEvidenceSet(manifest, payloads), initialVerificationState(item))
}

func Lookup(ledger Ledger, claimID string) (Claim, error) {
	if !validClaimID(claimID) {
		return Claim{}, errors.New("claim ID is invalid")
	}
	index, found := slices.BinarySearchFunc(ledger.Claims, claimID, func(item Claim, target string) int {
		return strings.Compare(item.ClaimID, target)
	})
	if !found {
		return Claim{}, fmt.Errorf("claim %q is absent from the ledger", claimID)
	}
	return cloneClaims([]Claim{ledger.Claims[index]})[0], nil
}

func verifyLedgerPackage(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) error {
	if ctx == nil || registry == nil {
		return errors.New("claim verification requires context and capsule registry")
	}
	if err := ledger.Validate(); err != nil {
		return err
	}
	if ledger.CapsuleID != manifest.CapsuleID || ledger.ManifestDigest != manifest.ManifestDigest {
		return errors.New("claim ledger does not bind the supplied capsule manifest")
	}
	_, err := capsule.VerifyPackage(ctx, registry, manifest, payloads, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	return err
}

func evaluateClaimState(item Claim, evidence evidenceSet, state verificationState) (ClaimVerification, error) {
	for _, component := range item.EvidenceComponents {
		record, found := evidence.records[component]
		if !state.Available[component] || !found {
			return ClaimVerification{}, GuardError{Guard: GuardEvidenceParentMissing, Message: fmt.Sprintf("required evidence component %q is unavailable", component)}
		}
		if record.Visibility != capsule.VisibilityPublic || state.VisibilityLeak {
			return ClaimVerification{}, GuardError{Guard: GuardVisibilityLeak, Message: fmt.Sprintf("claim evidence component %q is not a public projection", component)}
		}
	}
	if state.ScopeWidened {
		return ClaimVerification{}, GuardError{Guard: GuardScopeExceedsLedger, Message: "requested claim scope exceeds the sealed ledger scope"}
	}
	if item.Expression.Operation == OperationRatio && !state.DenominatorPresent {
		return ClaimVerification{}, GuardError{Guard: GuardExpressionDenominatorMissing, Message: "ratio denominator was removed from the ephemeral verification state"}
	}
	if item.Status.Assertable() && state.Attestation != AttestationNotRequired && state.Attestation != AttestationCurrent {
		return ClaimVerification{}, GuardError{Guard: GuardAttestationStale, Message: "claim requires a current attestation"}
	}
	for _, required := range item.EvidenceCeiling.RequiredCaveats {
		if !state.Caveats[required] {
			return ClaimVerification{}, GuardError{Guard: GuardRequiredCaveatMissing, Message: fmt.Sprintf("required caveat %q is absent", required)}
		}
	}
	if item.Status.Assertable() && state.EvidenceGeneration != item.Generation.Current {
		return ClaimVerification{}, GuardError{Guard: GuardGenerationNotCurrent, Message: fmt.Sprintf("evidence generation %q is not current generation %q", state.EvidenceGeneration, item.Generation.Current)}
	}
	for _, typeID := range item.EvidenceCeiling.RequiredParentTypes {
		found := false
		for _, component := range item.EvidenceComponents {
			record := evidence.records[component]
			if record.TypeID == typeID {
				found = true
				break
			}
		}
		if !found {
			return ClaimVerification{}, GuardError{Guard: GuardEvidenceParentMissing, Message: fmt.Sprintf("required evidence type %q is absent", typeID)}
		}
	}
	actual, err := evaluateExpression(item.Expression, evidence)
	if err != nil {
		return ClaimVerification{}, err
	}
	matches := actual.Equal(item.Expression.Expected)
	if !matches {
		return ClaimVerification{}, fmt.Errorf("claim expression produced %s:%q, want %s:%q", actual.Kind, actual.Value, item.Expression.Expected.Kind, item.Expression.Expected.Value)
	}
	return ClaimVerification{
		ClaimID: item.ClaimID, Status: item.Status, EvidenceLevel: item.EvidenceLevel,
		ExpressionValue: actual, ExpressionMatches: true,
		Assertable: item.Status.Assertable(), Valid: true,
	}, nil
}

func initialVerificationState(item Claim) verificationState {
	available := make(map[string]bool, len(item.EvidenceComponents))
	for _, component := range item.EvidenceComponents {
		available[component] = true
	}
	caveats := make(map[string]bool, len(item.Caveats))
	for _, caveat := range item.Caveats {
		caveats[caveat] = true
	}
	return verificationState{
		Available: available, Caveats: caveats, DenominatorPresent: true,
		Attestation: item.Attestation, EvidenceGeneration: item.Generation.Evidence,
	}
}

func expectedGuard(class ChallengeClass) Guard {
	switch class {
	case ChallengeParentRemoval:
		return GuardEvidenceParentMissing
	case ChallengeDigestSubstitution:
		return GuardEvidenceDigestMismatch
	case ChallengeDenominatorDeletion:
		return GuardExpressionDenominatorMissing
	case ChallengeScopeWidening:
		return GuardScopeExceedsLedger
	case ChallengeStaleAttestation:
		return GuardAttestationStale
	case ChallengeCaveatRemoval:
		return GuardRequiredCaveatMissing
	case ChallengeSupersededGeneration:
		return GuardGenerationNotCurrent
	case ChallengeVisibilityLeak:
		return GuardVisibilityLeak
	default:
		return ""
	}
}
