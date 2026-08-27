package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

// TestRunChallengeGateFailsClosedOnEmptyCapsule proves the gate never passes
// without a real verified capsule and receipts: an empty registry fails closed
// instead of reporting a vacuous pass.
func TestRunChallengeGateFailsClosedOnEmptyCapsule(t *testing.T) {
	pack := capsule.ReferencePackage{}
	result, err := RunChallengeGate(context.Background(), pack, claimledger.Ledger{})
	if err != nil {
		// ChallengeAll failing on the empty package is the fail-closed path.
		t.Logf("fail-closed via error: %v", err)
		return
	}
	if result.Pass {
		t.Fatal("empty capsule must not pass the challenge gate")
	}
	if len(result.Fails) == 0 {
		t.Fatalf("expected explicit fail reason: %+v", result)
	}
}

func TestEvaluateChallengeGateTable(t *testing.T) {
	spec := func(id string) claimledger.ChallengeSpec {
		class := claimledger.ChallengeDigestSubstitution
		return claimledger.ChallengeSpec{
			ChallengeID:   strings.ToLower(id) + "." + string(class),
			Class:         class,
			Applicability: claimledger.ChallengeApplied,
			Mutation:      "digest-swap",
			ExpectedGuard: claimledger.GuardEvidenceDigestMismatch,
		}
	}
	receiptFor := func(spec claimledger.ChallengeSpec, passed bool) claimledger.ChallengeReceipt {
		const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		claimID := "CLM-" + strings.ToUpper(strings.SplitN(spec.ChallengeID, ".", 2)[0])[4:]
		receipt := claimledger.ChallengeReceipt{
			SchemaVersion:           claimledger.ChallengeReceiptSchemaVersion,
			Verifier:                claimledger.VerifierVersion,
			ClaimID:                 claimID,
			ChallengeID:             spec.ChallengeID,
			Class:                   spec.Class,
			Mutation:                spec.Mutation,
			ExpectedGuard:           spec.ExpectedGuard,
			ObservedGuard:           spec.ExpectedGuard,
			Passed:                  true,
			BeforeState:             "before",
			AfterState:              "after",
			SourceCapsuleID:         digest,
			SourceManifestDigest:    digest,
			SourceLedgerDigest:      digest,
			SealedSourceDigest:      digest,
			AfterSealedSourceDigest: digest,
		}
		computed, err := protocol.Digest(receipt)
		if err != nil {
			t.Fatalf("digest receipt: %v", err)
		}
		receipt.Digest = computed
		return receipt
	}
	appliedLedger := func(ids ...string) claimledger.Ledger {
		var claims []claimledger.Claim
		for _, id := range ids {
			c := claimledger.Claim{ClaimID: id}
			c.Challenges = append(c.Challenges, spec(id))
			claims = append(claims, c)
		}
		return claimledger.Ledger{Claims: claims}
	}
	t.Run("missing receipt fails", func(t *testing.T) {
		ledger := appliedLedger("CLM-001", "CLM-002")
		result, err := EvaluateChallengeGate([]claimledger.ChallengeReceipt{receiptFor(spec("CLM-001"), true)}, ledger)
		if err != nil {
			t.Fatal(err)
		}
		if result.Pass || len(result.Fails) != 1 || !contains(result.Fails, "declared guard clm-002.digest-substitution never ran: no receipt") {
			t.Fatalf("missing receipt must fail: %+v", result)
		}
	})
	t.Run("zero applied specs fail vacuous pass", func(t *testing.T) {
		result, err := EvaluateChallengeGate(nil, claimledger.Ledger{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Pass || len(result.Fails) != 1 || !contains(result.Fails, "challenge pack declares no applied negative controls: the gate must never pass vacuously") {
			t.Fatalf("vacuum pass must fail: %+v", result)
		}
	})
	t.Run("full match passes with summaries", func(t *testing.T) {
		ledger := appliedLedger("CLM-001", "CLM-002")
		receipts := []claimledger.ChallengeReceipt{receiptFor(spec("CLM-001"), true), receiptFor(spec("CLM-002"), true)}
		result, err := EvaluateChallengeGate(receipts, ledger)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Pass || len(result.GuardSummaries) != 2 || len(result.Fails) != 0 {
			t.Fatalf("full match must pass: %+v", result)
		}
	})
}
