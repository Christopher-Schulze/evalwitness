package reliance

import (
	"context"
	"errors"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/claim"
)

func BuildRelianceProfileProjection(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claim.Ledger,
) (RelianceProfileProjection, error) {
	value, component, err := verifiedRelianceProjectionSources(ctx, base, child, ledger)
	if err != nil {
		return RelianceProfileProjection{}, err
	}
	return newRelianceProfileProjection(value, component, child.Manifest, ledger)
}

func newRelianceProfileProjection(
	value EvidenceRelianceMap,
	component capsule.ComponentRecord,
	manifest capsule.Manifest,
	ledger claim.Ledger,
) (RelianceProfileProjection, error) {
	projection := RelianceProfileProjection{
		SchemaVersion: RelianceProfileProjectionSchemaVersion,
		CapsuleID:     manifest.CapsuleID, ManifestDigest: manifest.ManifestDigest,
		MapComponentID: component.ComponentID, MapDigest: value.Digest, LedgerDigest: ledger.Digest,
		Scope: value.Scope, Dimensions: cloneRelianceProfileDimensions(value.ProfileDimensions),
		StatusCounts: relianceProfileStatusCounts(value.ProfileDimensions), GlobalScoreProhibited: true,
		ProviderCalls: 0, NetworkRequired: false,
	}
	digest, err := relianceProfileProjectionDigest(projection)
	if err != nil {
		return RelianceProfileProjection{}, err
	}
	projection.Digest = digest
	return projection, nil
}

func (value RelianceProfileProjection) Validate(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claim.Ledger,
) error {
	expected, err := BuildRelianceProfileProjection(ctx, base, child, ledger)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("reliance profile projection differs from its capsule and claim parents")
	}
	return nil
}

func BuildReliancePaperProjection(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claim.Ledger,
) (ReliancePaperProjection, error) {
	value, component, err := verifiedRelianceProjectionSources(ctx, base, child, ledger)
	if err != nil {
		return ReliancePaperProjection{}, err
	}
	return newReliancePaperProjection(value, component, child.Manifest, ledger)
}

func newReliancePaperProjection(
	value EvidenceRelianceMap,
	component capsule.ComponentRecord,
	manifest capsule.Manifest,
	ledger claim.Ledger,
) (ReliancePaperProjection, error) {
	current, unsupported := relianceProjectionClaimIDs(ledger)
	projection := ReliancePaperProjection{
		SchemaVersion: ReliancePaperProjectionSchemaVersion,
		CapsuleID:     manifest.CapsuleID, ManifestDigest: manifest.ManifestDigest,
		MapComponentID: component.ComponentID, MapDigest: value.Digest, LedgerDigest: ledger.Digest,
		Scope: value.Scope, Rows: cloneReliancePaperRows(value.PaperRows),
		Limitations: slices.Clone(value.PaperLimitations), CurrentClaimIDs: current,
		UnsupportedClaimIDs: unsupported, ProviderCalls: 0, NetworkRequired: false,
	}
	digest, err := reliancePaperProjectionDigest(projection)
	if err != nil {
		return ReliancePaperProjection{}, err
	}
	projection.Digest = digest
	return projection, nil
}

func (value ReliancePaperProjection) Validate(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claim.Ledger,
) error {
	expected, err := BuildReliancePaperProjection(ctx, base, child, ledger)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("reliance paper projection differs from its capsule and claim parents")
	}
	return nil
}

func verifiedRelianceProjectionSources(
	ctx context.Context,
	base capsule.ReferencePackage,
	child capsule.ReferencePackage,
	ledger claim.Ledger,
) (EvidenceRelianceMap, capsule.ComponentRecord, error) {
	value, err := verifyEvidenceReliancePackageFamily(ctx, base, child)
	if err != nil {
		return EvidenceRelianceMap{}, capsule.ComponentRecord{}, err
	}
	wantLedger, err := BuildEvidenceRelianceLedger(ctx, base, child)
	if err != nil || !reflect.DeepEqual(ledger, wantLedger) {
		return EvidenceRelianceMap{}, capsule.ComponentRecord{}, errors.New("reliance projection ledger differs from the canonical capsule-derived ledger")
	}
	component, err := uniqueRelianceCapsuleComponent(child.Manifest)
	return value, component, err
}

func relianceProjectionClaimIDs(ledger claim.Ledger) ([]string, []string) {
	current := make([]string, 0, len(ledger.Claims))
	unsupported := make([]string, 0, len(ledger.Claims))
	for _, item := range ledger.Claims {
		if item.Status.Assertable() {
			current = append(current, item.ClaimID)
		} else {
			unsupported = append(unsupported, item.ClaimID)
		}
	}
	return current, unsupported
}

func cloneRelianceProfileDimensions(values []RelianceProfileDimension) []RelianceProfileDimension {
	result := slices.Clone(values)
	for index := range result {
		result[index].Factors = slices.Clone(values[index].Factors)
		result[index].Estimate = cloneRelianceMapEstimate(values[index].Estimate)
		result[index].Caveats = slices.Clone(values[index].Caveats)
	}
	return result
}

func cloneReliancePaperRows(values []ReliancePaperRow) []ReliancePaperRow {
	result := slices.Clone(values)
	for index := range result {
		result[index].Factors = slices.Clone(values[index].Factors)
		result[index].Estimate = cloneRelianceMapEstimate(values[index].Estimate)
	}
	return result
}

func relianceProfileStatusCounts(values []RelianceProfileDimension) []RelianceProfileStatusCount {
	statuses := []string{"failed", "measured", "not_applicable", "not_measured", "unsupported"}
	counts := make(map[string]int, len(statuses))
	for _, value := range values {
		counts[value.Status]++
	}
	result := make([]RelianceProfileStatusCount, len(statuses))
	for index, status := range statuses {
		result[index] = RelianceProfileStatusCount{Status: status, Dimensions: counts[status]}
	}
	return result
}

func relianceProfileProjectionDigest(value RelianceProfileProjection) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}

func reliancePaperProjectionDigest(value ReliancePaperProjection) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
