package relation

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type ReplayedMaterial struct {
	Receipt     ReplayReceipt
	Original    []preprocess.Trajectory
	Transformed []preprocess.Trajectory
}

func ReplayCase(root string, release mutation.CorpusRelease, caseID string) (ReplayedMaterial, error) {
	if err := release.Validate(); err != nil {
		return ReplayedMaterial{}, fmt.Errorf("validate relation replay release: %w", err)
	}
	var selected *mutation.CorpusCase
	for index := range release.Cases {
		if release.Cases[index].ID == caseID {
			selected = &release.Cases[index]
			break
		}
	}
	if selected == nil {
		return ReplayedMaterial{}, fmt.Errorf("relation replay case %q is absent from the release", caseID)
	}
	sourceByID := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, source := range release.Sources {
		sourceByID[source.ID] = source
	}
	selectedSources := make([]mutation.CorpusSource, 0, len(selected.SourceIDs))
	for _, sourceID := range selected.SourceIDs {
		source, exists := sourceByID[sourceID]
		if !exists {
			return ReplayedMaterial{}, fmt.Errorf("relation replay case %q references missing source %q", caseID, sourceID)
		}
		selectedSources = append(selectedSources, source)
	}
	candidates, err := mutation.ResolveCorpusSources(root, selectedSources)
	if err != nil {
		return ReplayedMaterial{}, fmt.Errorf("discover relation replay sources: %w", err)
	}
	replayed, err := mutation.ReplayCorpusCase(release.Spec, *selected, release.Sources, candidates)
	if err != nil {
		return ReplayedMaterial{}, err
	}
	protocolVersion := ProtocolVersionV1
	if release.CorpusVersion == controlledCorruptionCorpusVersionV2 {
		protocolVersion = ProtocolVersionV2
	}
	receipt, err := buildReplayReceipt(protocolVersion, release.Digest, replayed)
	if err != nil {
		return ReplayedMaterial{}, err
	}
	return ReplayedMaterial{Receipt: receipt, Original: replayed.Original, Transformed: replayed.Transformed}, nil
}

func ReplayCaseV3(root string, plan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, caseID string) (ReplayedMaterial, error) {
	if err := release.Validate(plan, audit); err != nil {
		return ReplayedMaterial{}, fmt.Errorf("validate v3 relation replay release: %w", err)
	}
	var selected *mutation.CorpusCaseV3
	for index := range release.Cases {
		if release.Cases[index].ID == caseID {
			selected = &release.Cases[index]
			break
		}
	}
	if selected == nil {
		return ReplayedMaterial{}, fmt.Errorf("v3 relation replay case %q is absent from the release", caseID)
	}
	sourceByID := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, source := range release.Sources {
		sourceByID[source.ID] = source
	}
	selectedSources := make([]mutation.CorpusSource, 0, len(selected.SourceIDs))
	for _, sourceID := range selected.SourceIDs {
		source, exists := sourceByID[sourceID]
		if !exists {
			return ReplayedMaterial{}, fmt.Errorf("v3 relation replay case %q references missing source %q", caseID, sourceID)
		}
		selectedSources = append(selectedSources, source)
	}
	candidates, err := mutation.ResolveCorpusSources(root, selectedSources)
	if err != nil {
		return ReplayedMaterial{}, fmt.Errorf("discover v3 relation replay sources: %w", err)
	}
	replayed, err := mutation.ReplayCorpusCaseV3(plan, *selected, release.Sources, candidates)
	if err != nil {
		return ReplayedMaterial{}, err
	}
	receipt, err := buildReplayReceiptV3(release.Digest, replayed)
	if err != nil {
		return ReplayedMaterial{}, err
	}
	return ReplayedMaterial{Receipt: receipt, Original: replayed.Original, Transformed: replayed.Transformed}, nil
}

func buildReplayReceipt(protocolVersion, corpusDigest string, replayed mutation.ReplayedCorpusCase) (ReplayReceipt, error) {
	originalDigests := trajectoryDigests(replayed.Original)
	transformedDigests := trajectoryDigests(replayed.Transformed)
	unit := UnitTrajectoryPair
	if replayed.Case.Family == mutation.FamilyCandidateOrderReversal {
		unit = UnitCandidatePairOrders
	}
	receipt := ReplayReceipt{
		ProtocolVersion: protocolVersion, Objective: ReviewObjectiveControlledRelation, SourceCorpusDigest: corpusDigest,
		CaseID: replayed.Case.ID, Family: replayed.Case.Family, Unit: unit, SourceIDs: append([]string(nil), replayed.Case.SourceIDs...),
		OriginalTrajectoryDigests: originalDigests, TransformedTrajectoryDigests: transformedDigests,
		OriginalMaterialDigest: replayed.Case.Manifest.OriginalTrajectoryDigest, TransformedMaterialDigest: replayed.Case.Manifest.MutatedTrajectoryDigest,
		ManifestDigest: replayed.Case.Manifest.Digest, BlindPacketDigest: replayed.Case.BlindPacket.Digest,
		RegenerationKey: replayed.Case.RegenerationKey, ReplayStatus: "exact", ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealReplayReceipt(receipt)
}

func buildReplayReceiptV3(corpusDigest string, replayed mutation.ReplayedCorpusCaseV3) (ReplayReceipt, error) {
	originalDigests := trajectoryDigests(replayed.Original)
	transformedDigests := trajectoryDigests(replayed.Transformed)
	unit := UnitTrajectoryPair
	if replayed.Case.Family == mutation.FamilyCandidateOrderReversal {
		unit = UnitCandidatePairOrders
	}
	receipt := ReplayReceipt{
		ProtocolVersion: ProtocolVersionV3, Objective: ReviewObjectiveControlledRelation, SourceCorpusDigest: corpusDigest,
		CaseID: replayed.Case.ID, Family: replayed.Case.Family, Unit: unit, SourceIDs: append([]string(nil), replayed.Case.SourceIDs...),
		OriginalTrajectoryDigests: originalDigests, TransformedTrajectoryDigests: transformedDigests,
		OriginalMaterialDigest: replayed.Case.Manifest.OriginalTrajectoryDigest, TransformedMaterialDigest: replayed.Case.Manifest.MutatedTrajectoryDigest,
		ManifestDigest: replayed.Case.Manifest.Digest, BlindPacketDigest: replayed.Case.BlindPacket.Digest,
		RegenerationKey: replayed.Case.RegenerationKey, ReplayStatus: "exact", ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealReplayReceipt(receipt)
}

func SealReplayReceipt(receipt ReplayReceipt) (ReplayReceipt, error) {
	schemaVersion, err := schemaVersionForProtocol(receipt.ProtocolVersion, ReplayReceiptSchemaVersionV1, ReplayReceiptSchemaVersionV2, ReplayReceiptSchemaVersionV3)
	if err != nil {
		return ReplayReceipt{}, err
	}
	receipt.SchemaVersion, receipt.CanonicalPolicy, receipt.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := replayReceiptDigest(receipt)
	if err != nil {
		return ReplayReceipt{}, err
	}
	receipt.Digest = digest
	return receipt, receipt.Validate()
}

func (receipt ReplayReceipt) Validate() error {
	definition, exists := mutation.DefinitionFor(receipt.Family)
	expectedUnit := UnitTrajectoryPair
	expectedArity := 1
	if exists && definition.PairLevel {
		expectedUnit, expectedArity = UnitCandidatePairOrders, 2
	}
	if !validVersionedIdentity(receipt.SchemaVersion, receipt.ProtocolVersion, ReplayReceiptSchemaVersionV1, ReplayReceiptSchemaVersionV2, ReplayReceiptSchemaVersionV3) || receipt.CanonicalPolicy != CanonicalPolicy ||
		receipt.Objective != ReviewObjectiveControlledRelation || !validDigest(receipt.SourceCorpusDigest) || !exists || receipt.Unit != expectedUnit ||
		len(receipt.SourceIDs) != expectedArity || len(receipt.OriginalTrajectoryDigests) != expectedArity || len(receipt.TransformedTrajectoryDigests) != expectedArity ||
		!validDigest(receipt.OriginalMaterialDigest) || !validDigest(receipt.TransformedMaterialDigest) || !validDigest(receipt.ManifestDigest) ||
		!validDigest(receipt.BlindPacketDigest) || !validDigest(receipt.RegenerationKey) || receipt.ReplayStatus != "exact" ||
		receipt.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation replay receipt identity, unit, material, status, or authorization boundary is invalid")
	}
	for _, values := range [][]string{receipt.SourceIDs, receipt.OriginalTrajectoryDigests, receipt.TransformedTrajectoryDigests} {
		if len(values) == 0 || slices.Contains(values, "") {
			return errors.New("relation replay receipt has an empty source or trajectory identity")
		}
	}
	if expectedArity == 1 {
		if receipt.OriginalMaterialDigest != receipt.OriginalTrajectoryDigests[0] || receipt.TransformedMaterialDigest != receipt.TransformedTrajectoryDigests[0] {
			return errors.New("relation trajectory replay material digest does not match its trajectory")
		}
	} else {
		original, err := digestJSON(receipt.OriginalTrajectoryDigests)
		if err != nil || original != receipt.OriginalMaterialDigest {
			return errors.New("relation pair replay original material digest is invalid")
		}
		transformed, err := digestJSON(receipt.TransformedTrajectoryDigests)
		if err != nil || transformed != receipt.TransformedMaterialDigest || !slices.Equal(receipt.OriginalTrajectoryDigests, []string{receipt.TransformedTrajectoryDigests[1], receipt.TransformedTrajectoryDigests[0]}) {
			return errors.New("relation pair replay transformed material is not the exact reverse ordering")
		}
	}
	expected, err := replayReceiptDigest(receipt)
	if err != nil || expected != receipt.Digest {
		return errors.New("relation replay receipt digest is invalid")
	}
	return nil
}

func trajectoryDigests(values []preprocess.Trajectory) []string {
	result := make([]string, len(values))
	for index := range values {
		result[index] = values[index].Digest
	}
	return result
}

func replayReceiptDigest(receipt ReplayReceipt) (string, error) {
	receipt.Digest = ""
	return digestJSON(receipt)
}
