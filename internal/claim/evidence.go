package claim

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
)

// EvidenceVerification is the atomic result of verifying a public capsule,
// its claim ledger, its deterministic autopsy, and every applicable challenge.
type EvidenceVerification struct {
	Ledger     VerificationReport
	Autopsy    Autopsy
	Challenges ChallengePack
}

// VerifyEvidence establishes one verified package boundary and derives every
// claim-side verification artifact without revalidating the unchanged source.
func VerifyEvidence(
	ctx context.Context,
	registry *capsule.Registry,
	manifest capsule.Manifest,
	payloads map[string][]byte,
	ledger Ledger,
	actualAutopsy Autopsy,
) (EvidenceVerification, error) {
	if err := actualAutopsy.Validate(); err != nil {
		return EvidenceVerification{}, fmt.Errorf("validate evidence autopsy: %w", err)
	}
	if err := verifyLedgerPackage(ctx, registry, manifest, payloads, ledger); err != nil {
		return EvidenceVerification{}, fmt.Errorf("verify evidence package and ledger: %w", err)
	}
	ledgerReport, err := verifyLedgerClaims(manifest, payloads, ledger)
	if err != nil {
		return EvidenceVerification{}, fmt.Errorf("evaluate verified evidence ledger: %w", err)
	}
	expectedAutopsy, err := buildAutopsy(manifest, payloads, ledger)
	if err != nil {
		return EvidenceVerification{}, fmt.Errorf("build verified evidence autopsy: %w", err)
	}
	if !reflect.DeepEqual(actualAutopsy, expectedAutopsy) {
		return EvidenceVerification{}, errors.New("claim autopsy differs from deterministic capsule and ledger recomputation")
	}
	receipts, err := challengeAllVerified(ctx, registry, manifest, payloads, ledger)
	if err != nil {
		return EvidenceVerification{}, fmt.Errorf("challenge verified evidence: %w", err)
	}
	challengePack, err := buildChallengePack(manifest, ledger, receipts)
	if err != nil {
		return EvidenceVerification{}, fmt.Errorf("build verified evidence challenge pack: %w", err)
	}
	return EvidenceVerification{Ledger: ledgerReport, Autopsy: expectedAutopsy, Challenges: challengePack}, nil
}
