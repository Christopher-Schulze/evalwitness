package relation

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestRelationV3GovernanceToOwnerInspectionChainIsVersionClosed(t *testing.T) {
	corpusPlan, audit, release := readGovernedCorpusV3(t)
	governedPlan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", governedPlan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", governedPlan, primary)
	pilot := readGovernedPilotV3(t, "relation-pilot-sample-v3.json", governedPlan, primary, sentinel)
	amendment := readGovernedAmendmentV3(t, "relation-study-amendment-v3.json", governedPlan, pilot, primary, sentinel)
	reviewPlan, err := ReviewPlanV3(governedPlan)
	if err != nil {
		t.Fatal(err)
	}
	reviewPilot, err := ReviewPilotSampleV3(governedPlan, primary, sentinel, pilot)
	if err != nil {
		t.Fatal(err)
	}

	caseByID := make(map[string]mutation.CorpusCaseV3, len(release.Cases))
	sourceByID := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, item := range release.Cases {
		caseByID[item.ID] = item
	}
	for _, source := range release.Sources {
		sourceByID[source.ID] = source
	}
	packets := make([]BlindPacket, len(pilot.Cases))
	mappings := make([]PrivateMapping, len(pilot.Cases))
	var firstReceipt ReplayReceipt
	var firstMaterial CaseMaterial
	for index, reference := range pilot.Cases {
		item := caseByID[reference.CaseID]
		receipt := sealedV3ReplayReceiptFixture(t, release.Digest, item, sourceByID)
		material := sealedV3CaseMaterialFixture(t, reviewPlan, corpusPlan, audit, release, item, sourceByID, receipt)
		packet, mapping, buildErr := BuildBlindedPacketV3(
			reviewPlan, corpusPlan, audit, release, material, fmt.Sprintf("v3-chain-key-%d", index+1), bytes.Repeat([]byte{byte(0x31 + index)}, 32),
		)
		if buildErr != nil {
			t.Fatalf("build v3 pilot packet %d: %v", index, buildErr)
		}
		packets[index], mappings[index] = packet, mapping
		if index == 0 {
			firstReceipt, firstMaterial = receipt, material
		}
	}
	qualification, answerKey, err := DefaultQualification(reviewPlan, "v3-qualification-key", bytes.Repeat([]byte{0x61}, 32))
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(reviewPlan, qualification)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := BuildReviewBundle(reviewPlan, pilot.Digest, ReviewDataDevelopmentPilot, packets, mappings, qualification, handbook, "2026-08-10T03:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := BuildRelationPilotReadiness(reviewPlan, reviewPilot, bundle, mappings, qualification, handbook, "2026-08-10T03:01:00Z")
	if err != nil {
		t.Fatal(err)
	}
	changeReceipt, err := BuildPilotChangeReceipt(readiness, bundle, mappings)
	if err != nil {
		t.Fatal(err)
	}
	dossier, err := BuildPilotLaunchDossier(reviewPlan, reviewPilot, bundle, mappings, qualification, handbook, readiness, "2026-08-10T03:02:00Z")
	if err != nil {
		t.Fatal(err)
	}
	drafts := make([]PilotInspectionDecisionDraft, len(bundle.Packets))
	for index, packet := range bundle.Packets {
		candidateOrder := PilotInspectionNotApplicable
		if packet.Unit == UnitCandidatePairOrders {
			candidateOrder = PilotInspectionPassed
		}
		drafts[index] = PilotInspectionDecisionDraft{
			PacketID: packet.PacketID, TaskContext: PilotInspectionPassed, EvidenceAlignment: PilotInspectionPassed,
			TransformationIsolation: PilotInspectionPassed, InformationSufficiency: PilotInspectionPassed,
			BlindingIntegrity: PilotInspectionPassed, RubricApplicability: PilotInspectionPassed,
			RedistributionBoundary: PilotInspectionPassed, CandidateOrder: candidateOrder, ReasonCodes: []PilotInspectionReason{},
		}
	}
	inspection, err := BuildPilotInspectionRecord(readiness, bundle, mappings, drafts, "owner-inspector-v3-fixture", "2026-08-10T03:03:00Z")
	if err != nil {
		t.Fatal(err)
	}

	identities := []struct {
		name, protocol, schema, want string
	}{
		{"plan", governedPlan.ProtocolVersion, governedPlan.SchemaVersion, PlanSchemaVersionV3},
		{"primary", primary.ProtocolVersion, primary.SchemaVersion, PrimarySampleSchemaVersionV3},
		{"sentinel", sentinel.ProtocolVersion, sentinel.SchemaVersion, ScarcitySentinelSchemaVersionV3},
		{"pilot", pilot.ProtocolVersion, pilot.SchemaVersion, PilotSampleSchemaVersionV3},
		{"amendment", amendment.ProtocolVersion, amendment.SchemaVersion, StudyAmendmentSchemaVersionV3},
		{"replay receipt", firstReceipt.ProtocolVersion, firstReceipt.SchemaVersion, ReplayReceiptSchemaVersionV3},
		{"case material", firstMaterial.ProtocolVersion, firstMaterial.SchemaVersion, CaseMaterialSchemaVersionV3},
		{"blind packet", packets[0].ProtocolVersion, packets[0].SchemaVersion, BlindPacketSchemaVersionV3},
		{"private mapping", mappings[0].ProtocolVersion, mappings[0].SchemaVersion, PrivateMappingSchemaVersionV3},
		{"qualification set", qualification.ProtocolVersion, qualification.SchemaVersion, QualificationSetSchemaVersionV3},
		{"qualification answer key", answerKey.ProtocolVersion, answerKey.SchemaVersion, QualificationKeySchemaVersionV3},
		{"handbook", handbook.ProtocolVersion, handbook.SchemaVersion, ReviewerHandbookSchemaVersionV3},
		{"bundle", bundle.ProtocolVersion, bundle.SchemaVersion, ReviewBundleSchemaVersionV3},
		{"readiness", readiness.ProtocolVersion, readiness.SchemaVersion, RelationPilotReadinessSchemaVersionV3},
		{"change receipt", changeReceipt.ProtocolVersion, changeReceipt.SchemaVersion, PilotChangeReceiptSchemaVersionV3},
		{"launch dossier", dossier.ProtocolVersion, dossier.SchemaVersion, PilotLaunchDossierSchemaVersionV3},
		{"inspection", inspection.ProtocolVersion, inspection.SchemaVersion, PilotInspectionSchemaVersionV3},
	}
	for _, identity := range identities {
		if identity.protocol != ProtocolVersionV3 || identity.schema != identity.want {
			t.Fatalf("%s escaped protocol v3: protocol=%q schema=%q", identity.name, identity.protocol, identity.schema)
		}
	}
	if readiness.SourceCorpusPlanDigest != corpusPlan.Digest || changeReceipt.SourceCorpusPlanDigest != corpusPlan.Digest || dossier.SourceCorpusPlanDigest != corpusPlan.Digest {
		t.Fatal("v3 owner-inspection chain lost its exact corpus-plan parent")
	}
	tampered := readiness
	tampered.ProtocolVersion = ProtocolVersionV2
	if _, err := SealRelationPilotReadiness(tampered); err == nil {
		t.Fatal("v2 readiness envelope accepted v3 corpus-plan bindings")
	}
}

func sealedV3ReplayReceiptFixture(t *testing.T, corpusDigest string, item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) ReplayReceipt {
	t.Helper()
	original := make([]string, len(item.SourceIDs))
	for index, sourceID := range item.SourceIDs {
		original[index] = sources[sourceID].TrajectoryDigest
	}
	transformed := []string{item.Manifest.MutatedTrajectoryDigest}
	unit := UnitTrajectoryPair
	if item.Family == mutation.FamilyCandidateOrderReversal {
		unit = UnitCandidatePairOrders
		transformed = []string{original[1], original[0]}
	}
	receipt, err := SealReplayReceipt(ReplayReceipt{
		ProtocolVersion: ProtocolVersionV3, Objective: ReviewObjectiveControlledRelation, SourceCorpusDigest: corpusDigest,
		CaseID: item.ID, Family: item.Family, Unit: unit, SourceIDs: append([]string(nil), item.SourceIDs...),
		OriginalTrajectoryDigests: original, TransformedTrajectoryDigests: transformed,
		OriginalMaterialDigest: item.Manifest.OriginalTrajectoryDigest, TransformedMaterialDigest: item.Manifest.MutatedTrajectoryDigest,
		ManifestDigest: item.Manifest.Digest, BlindPacketDigest: item.BlindPacket.Digest, RegenerationKey: item.RegenerationKey,
		ReplayStatus: "exact", ExternalActionStatus: ExternalActionNotAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func sealedV3CaseMaterialFixture(t *testing.T, plan Plan, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, item mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource, receipt ReplayReceipt) CaseMaterial {
	t.Helper()
	original := make([]EvidenceExcerpt, len(item.SourceIDs))
	for index, sourceID := range item.SourceIDs {
		original[index] = v3EvidenceExcerptFixture(sources[sourceID], sources[sourceID].TrajectoryDigest, fmt.Sprintf("original-%d", index+1))
	}
	transformed := make([]EvidenceExcerpt, 1)
	if item.Family == mutation.FamilyCandidateOrderReversal {
		transformed = []EvidenceExcerpt{original[1], original[0]}
	} else {
		transformed[0] = v3EvidenceExcerptFixture(sources[item.SourceIDs[0]], item.Manifest.MutatedTrajectoryDigest, "transformed")
	}
	material, err := SealCaseMaterial(CaseMaterial{
		ProtocolVersion: ProtocolVersionV3, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest,
		SourceCorpusDigest: release.Digest, SourceCorpusPlanDigest: corpusPlan.Digest,
		SourceMutationProgramDigest: release.MutationProgramDigest, SourceConstructAuditDigest: audit.Digest,
		RelationContractVersion: mutation.RelationContractVersionV3, EvidenceBoundaryVersion: mutation.EvidenceBoundaryVersionV3,
		ConstructFirewallDigest: item.ConstructFirewall.Digest, CaseID: item.ID, Family: item.Family, Unit: receipt.Unit,
		TaskRequirement:       "Assess whether the visible evidence satisfies the stated coding task.",
		TaskRequirementDigest: digestText("Assess whether the visible evidence satisfies the stated coding task."),
		Original:              original, Transformed: transformed, AlignmentDigest: digestText("v3-alignment-" + item.ID),
		ReplayReceiptDigest:  receipt.Digest,
		Limitations:          []string{"A deterministic fixture limitation.", "B restricted evidence limitation.", "C no-human-claim limitation."},
		ExternalActionStatus: ExternalActionNotAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	return material
}

func v3EvidenceExcerptFixture(source mutation.CorpusSource, trajectoryDigest, label string) EvidenceExcerpt {
	content := "governed redacted evidence " + label
	return EvidenceExcerpt{
		SourceTrajectoryDigest: trajectoryDigest, RetainedTrajectoryDigest: digestText("retained-" + trajectoryDigest + label),
		SourceEvents: 3, RetainedEvents: 2, OmittedEvents: 1, EvidenceBudgetTokens: RelationEvidenceBudgetTokens,
		EvidenceSelector: RelationEvidenceSelectorVersion, RequiredEventIDs: []string{"event-1"}, RetainedLineageDigest: digestText("lineage-" + source.ID),
		Content: content, ContentDigest: digestText(content), LicenseSPDX: source.License.SPDX, SourceURL: source.License.SourceURL,
		SourceRevision: source.License.SourceRevision, Redistribution: source.License.Redistribution,
		Visibility: restrictedReferenceVisibility, PublicReleasable: false,
	}
}
