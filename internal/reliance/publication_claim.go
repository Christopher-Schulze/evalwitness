package reliance

import (
	"context"
	"errors"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/claim"
)

func BuildEvidenceRelianceLedger(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
) (claim.Ledger, error) {
	if _, err := verifyEvidenceReliancePackageFamily(ctx, base, child); err != nil {
		return claim.Ledger{}, err
	}
	ledger, err := claim.BuildRelianceLedger(ctx, child.Registry, child.Manifest, child.Payloads, claim.RelianceLedgerSource{
		ComponentName: EvidenceRelianceComponentName,
		TypeID:        EvidenceRelianceMapSchemaVersion,
	})
	if err != nil {
		return claim.Ledger{}, err
	}
	if ledger.CapsuleID != child.Manifest.CapsuleID {
		return claim.Ledger{}, errors.New("evidence reliance ledger belongs to another capsule")
	}
	return ledger, nil
}
