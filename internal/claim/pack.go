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

const ChallengePackSchemaVersion = "evalwitness.claim-challenge-pack.v1"

type ChallengePack struct {
	SchemaVersion   string             `json:"schema_version"`
	SourceCapsuleID string             `json:"source_capsule_id"`
	LedgerDigest    string             `json:"ledger_digest"`
	Receipts        []ChallengeReceipt `json:"receipts"`
	ClassCounts     map[string]int     `json:"class_counts"`
	Digest          string             `json:"digest"`
}

func BuildChallengePack(ctx context.Context, registry *capsule.Registry, manifest capsule.Manifest, payloads map[string][]byte, ledger Ledger) (ChallengePack, error) {
	receipts, err := ChallengeAll(ctx, registry, manifest, payloads, ledger)
	if err != nil {
		return ChallengePack{}, err
	}
	return buildChallengePack(manifest, ledger, receipts)
}

func buildChallengePack(manifest capsule.Manifest, ledger Ledger, receipts []ChallengeReceipt) (ChallengePack, error) {
	pack := ChallengePack{
		SchemaVersion: ChallengePackSchemaVersion, SourceCapsuleID: manifest.CapsuleID,
		LedgerDigest: ledger.Digest, Receipts: slices.Clone(receipts), ClassCounts: map[string]int{},
	}
	slices.SortFunc(pack.Receipts, func(left, right ChallengeReceipt) int {
		return strings.Compare(left.ClaimID+"\x00"+left.ChallengeID, right.ClaimID+"\x00"+right.ChallengeID)
	})
	for _, receipt := range pack.Receipts {
		pack.ClassCounts[string(receipt.Class)]++
	}
	digest, err := challengePackDigest(pack)
	if err != nil {
		return ChallengePack{}, err
	}
	pack.Digest = digest
	if err := pack.Validate(); err != nil {
		return ChallengePack{}, err
	}
	return pack, nil
}

func (pack ChallengePack) Validate() error {
	if pack.SchemaVersion != ChallengePackSchemaVersion || !validDigest(pack.SourceCapsuleID) ||
		!validDigest(pack.LedgerDigest) || len(pack.Receipts) == 0 || pack.ClassCounts == nil || !validDigest(pack.Digest) {
		return errors.New("claim challenge pack identity is invalid")
	}
	counts := make(map[string]int)
	previous := ""
	for _, receipt := range pack.Receipts {
		key := receipt.ClaimID + "\x00" + receipt.ChallengeID
		if key <= previous || receipt.SourceCapsuleID != pack.SourceCapsuleID || receipt.SourceLedgerDigest != pack.LedgerDigest {
			return fmt.Errorf("claim challenge receipt %q is duplicated, unsorted, or misbound", receipt.ChallengeID)
		}
		if err := receipt.Validate(); err != nil {
			return err
		}
		counts[string(receipt.Class)]++
		previous = key
	}
	if len(counts) != len(pack.ClassCounts) {
		return errors.New("claim challenge class counts are incomplete")
	}
	for class, count := range counts {
		if pack.ClassCounts[class] != count {
			return errors.New("claim challenge class count differs from receipts")
		}
	}
	digest, err := challengePackDigest(pack)
	if err != nil || digest != pack.Digest {
		return errors.New("claim challenge pack digest is invalid")
	}
	return nil
}

func challengePackDigest(pack ChallengePack) (string, error) {
	pack.Digest = ""
	return protocol.Digest(pack)
}
