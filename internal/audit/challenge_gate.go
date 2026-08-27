package audit

import (
	"context"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
)

// ChallengeGateResult reports the TASK 050 challenge-pack gate: every declared
// negative control ran in an ephemeral context and produced a valid receipt.
type ChallengeGateResult struct {
	Pass           bool     `json:"pass"`
	Receipts       int      `json:"receipts"`
	GuardSummaries []string `json:"guard_summaries,omitempty"`
	Fails          []string `json:"fails,omitempty"`
}

// RunChallengeGate executes the declared TASK 050 negative controls via
// claim.ChallengeAll against the verified capsule and fails closed when any
// guard does not fire or fires for the wrong reason. Green normal verification
// without green challenge receipts is insufficient for release policies that
// enable this requirement.
func RunChallengeGate(ctx context.Context, pack capsule.ReferencePackage, ledger claimledger.Ledger) (ChallengeGateResult, error) {
	receipts, err := claimledger.ChallengeAll(ctx, pack.Registry, pack.Manifest, pack.Payloads, ledger)
	if err != nil {
		return ChallengeGateResult{}, fmt.Errorf("challenge gate: %w", err)
	}
	return EvaluateChallengeGate(receipts, ledger)
}

// EvaluateChallengeGate is the pure decision core of the gate so the
// declared-vs-received diff and vacuum rule are directly testable.
func EvaluateChallengeGate(receipts []claimledger.ChallengeReceipt, ledger claimledger.Ledger) (ChallengeGateResult, error) {
	// Declared-vs-received diff: every applied challenge spec in the ledger
	// must have produced exactly one receipt. A ledger declaring N guards with
	// fewer receipts means guards silently did not run. Zero applied specs is
	// itself a failure: the gate must never pass vacuously.
	received := make(map[string]claimledger.ChallengeReceipt, len(receipts))
	for _, receipt := range receipts {
		if err := receipt.Validate(); err != nil {
			return ChallengeGateResult{}, fmt.Errorf("challenge gate: receipt %s: %w", receipt.ChallengeID, err)
		}
		received[receipt.ChallengeID] = receipt
	}
	result := ChallengeGateResult{Pass: true, Receipts: len(receipts)}
	var fails []string
	applied := 0
	for _, item := range ledger.Claims {
		for _, specification := range item.Challenges {
			if specification.Applicability != claimledger.ChallengeApplied {
				continue
			}
			applied++
			receipt, ok := received[specification.ChallengeID]
			if !ok {
				fails = append(fails, fmt.Sprintf("declared guard %s never ran: no receipt", specification.ChallengeID))
				continue
			}
			if !receipt.Passed {
				fails = append(fails, fmt.Sprintf("guard %s did not fire correctly (observed=%s)", receipt.ChallengeID, receipt.ObservedGuard))
				continue
			}
			if receipt.ObservedGuard != receipt.ExpectedGuard {
				fails = append(fails, fmt.Sprintf("guard %s fired for the wrong reason: expected %s observed %s", receipt.ChallengeID, receipt.ExpectedGuard, receipt.ObservedGuard))
				continue
			}
			if receipt.SealedSourceDigest != receipt.AfterSealedSourceDigest {
				fails = append(fails, fmt.Sprintf("guard %s mutated the source capsule", receipt.ChallengeID))
				continue
			}
			result.GuardSummaries = append(result.GuardSummaries, fmt.Sprintf("%s guard=%s passed=%t", receipt.ChallengeID, receipt.ExpectedGuard, receipt.Passed))
		}
	}
	if applied == 0 {
		fails = append(fails, "challenge pack declares no applied negative controls: the gate must never pass vacuously")
	}
	if len(fails) > 0 {
		result.Pass = false
		result.Fails = fails
	}
	return result, nil
}
