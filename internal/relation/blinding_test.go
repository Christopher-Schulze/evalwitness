package relation

import (
	"bytes"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
)

func TestRelationBlindingKeyDecoderIsStrict(t *testing.T) {
	encoded := bytes.Repeat([]byte("ab"), 32)
	key, err := DecodeBlindingKey(bytes.NewReader(append(encoded, '\n')))
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("decoded relation blinding key has %d bytes", len(key))
	}
	for _, invalid := range [][]byte{bytes.ToUpper(encoded), encoded[:62], append(encoded, '0', '0')} {
		if _, err := DecodeBlindingKey(bytes.NewReader(invalid)); err == nil {
			t.Fatal("relation blinding key decoder accepted a noncanonical key")
		}
	}
}

func TestRelationCandidatePacketSeparatesEveryRandomizationDomain(t *testing.T) {
	plan, material, context := relationCandidateBlindingFixture(t)
	key := bytes.Repeat([]byte{0x21}, 32)
	packet, mapping, err := buildBlindedPacket(plan, material, context, "relation-test-key", key)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Objective != ReviewObjectiveControlledRelation || mapping.Objective != ReviewObjectiveControlledRelation || len(mapping.RandomizationDomains) != 7 {
		t.Fatal("relation blinding lost its objective or complete domain-separation declaration")
	}
	if packet.PacketOrderCommitment != digestText(mapping.PacketOrderKey) || packet.ReviewerOrderCommitment != digestText(mapping.ReviewerOrderKey) {
		t.Fatal("relation public order commitments do not bind the owner-only sort keys")
	}
	left := packet.Sides[0].Evidence
	right := packet.Sides[1].Evidence
	if !slices.Equal([]string{left[0].ContentDigest, left[1].ContentDigest}, []string{right[1].ContentDigest, right[0].ContentDigest}) {
		t.Fatal("relation candidate packet is not a preserved pair-of-pairs reversal")
	}
	identities := []string{packet.PacketID, packet.TaskAlias, packet.Sides[0].SideAlias, packet.Sides[1].SideAlias, mapping.PacketOrderKey, mapping.ReviewerOrderKey}
	for _, side := range packet.Sides {
		for _, evidence := range side.Evidence {
			identities = append(identities, evidence.SlotID, evidence.CandidateLabel)
		}
	}
	if hasDuplicate(identities) {
		t.Fatal("relation blinding reused an identity across independent domains")
	}

	rebuiltPacket, rebuiltMapping, err := buildBlindedPacket(plan, material, context, "relation-test-key", key)
	if err != nil {
		t.Fatal(err)
	}
	if rebuiltPacket.Digest != packet.Digest || rebuiltMapping.Digest != mapping.Digest {
		t.Fatal("relation blinding is not deterministic for the same objective, material, and key")
	}
	rekeyedPacket, rekeyedMapping, err := buildBlindedPacket(plan, material, context, "relation-test-key-2", bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if rekeyedPacket.PacketID == packet.PacketID || rekeyedPacket.TaskAlias == packet.TaskAlias || rekeyedPacket.Sides[0].SideAlias == packet.Sides[0].SideAlias ||
		rekeyedPacket.Sides[0].Evidence[0].SlotID == packet.Sides[0].Evidence[0].SlotID || rekeyedPacket.Sides[0].Evidence[0].CandidateLabel == packet.Sides[0].Evidence[0].CandidateLabel ||
		rekeyedMapping.PacketOrderKey == mapping.PacketOrderKey || rekeyedMapping.ReviewerOrderKey == mapping.ReviewerOrderKey {
		t.Fatal("relation rekeying did not independently replace every blinded identity and order key")
	}
}

func TestRelationPacketAndMappingRejectTamperingAndOutcomeSubstitution(t *testing.T) {
	plan, material, context := relationCandidateBlindingFixture(t)
	packet, mapping, err := buildBlindedPacket(plan, material, context, "relation-test-key", bytes.Repeat([]byte{0x21}, 32))
	if err != nil {
		t.Fatal(err)
	}
	tamperedPacket := packet
	tamperedPacket.Objective = "single_trajectory_outcome"
	if err := tamperedPacket.Validate(); err == nil {
		t.Fatal("relation packet accepted an outcome objective")
	}
	tamperedMapping := mapping
	tamperedMapping.EvidenceMappings = append([]PrivateEvidenceMapping(nil), mapping.EvidenceMappings...)
	tamperedMapping.EvidenceMappings[0].LogicalSide = LogicalTransformed
	if err := tamperedMapping.Validate(); err == nil {
		t.Fatal("relation private mapping accepted a direction mutation")
	}

	encodedRelationPacket, err := EncodeIndented(packet)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outcome.DecodeBlindPacket(bytes.NewReader(encodedRelationPacket)); err == nil {
		t.Fatal("outcome decoder accepted a controlled-relation packet")
	}
	outcomePacket, err := outcome.SealBlindPacket(outcome.BlindPacket{
		PlanDigest: digestText("outcome-plan"), TaskAlias: "taskref-" + digestText("outcome-task"),
		Evidence:        []outcome.PacketEvidence{{Slot: "slot-" + digestText("outcome-slot"), Kind: "trajectory", Content: "evidence", ContentDigest: digestText("evidence"), License: "fixture", Limitation: "fixture-only"}},
		RubricQuestions: []string{"Is the task solved?"}, PrivacyClass: "restricted", PublicReleasable: false,
	}, "packet-"+digestText("outcome-packet"))
	if err != nil {
		t.Fatal(err)
	}
	encodedOutcomePacket, err := outcome.EncodeIndented(outcomePacket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBlindPacket(bytes.NewReader(encodedOutcomePacket)); err == nil {
		t.Fatal("relation decoder accepted a single-trajectory outcome packet")
	}
}

func TestRelationMaterialAndPacketRejectSourceContextRedistributionAndPairCollapse(t *testing.T) {
	plan, material, context := relationCandidateBlindingFixture(t)

	missingContext := material
	missingContext.TaskRequirement = ""
	missingContext.TaskRequirementDigest = digestText("")
	if _, err := SealCaseMaterial(missingContext); err == nil {
		t.Fatal("relation material accepted missing task context")
	}

	redistributed := material
	redistributed.Original = append([]EvidenceExcerpt(nil), material.Original...)
	redistributed.Original[0].Redistribution = "redistributable"
	if _, err := SealCaseMaterial(redistributed); err == nil {
		t.Fatal("relation material upgraded reference-only evidence")
	}

	sourceMismatch := material
	sourceMismatch.Original = append([]EvidenceExcerpt(nil), material.Original...)
	sourceMismatch.Original[0].SourceTrajectoryDigest = digestText("wrong-source")
	if err := validateMaterialSourceBindings(sourceMismatch, context.caseRecord, context.sources); err == nil {
		t.Fatal("relation material accepted a source-digest mismatch")
	}

	packet, _, err := buildBlindedPacket(plan, material, context, "relation-adversarial-key", bytes.Repeat([]byte{0x29}, 32))
	if err != nil {
		t.Fatal(err)
	}
	collapsed := packet
	collapsed.Sides = append([]PacketSide(nil), packet.Sides...)
	collapsed.Sides[1].Evidence = append([]PacketEvidence(nil), packet.Sides[1].Evidence...)
	collapsed.Sides[1].Evidence[0].ContentDigest = collapsed.Sides[0].Evidence[0].ContentDigest
	if _, err := SealBlindPacket(collapsed); err == nil {
		t.Fatal("relation candidate packet accepted a collapsed pair ordering")
	}
}

func relationCandidateBlindingFixture(t *testing.T) (Plan, CaseMaterial, packetContext) {
	t.Helper()
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	excerpt := func(name string) EvidenceExcerpt {
		content := "redacted evidence " + name
		return EvidenceExcerpt{
			SourceTrajectoryDigest: digestText("source-" + name), RetainedTrajectoryDigest: digestText("retained-" + name),
			SourceEvents: 4, RetainedEvents: 3, OmittedEvents: 1, RedactionHits: 1,
			EvidenceBudgetTokens: RelationEvidenceBudgetTokens, EvidenceSelector: RelationEvidenceSelectorVersion,
			RequiredEventIDs: []string{"event-" + name}, RetainedLineageDigest: digestText("lineage-" + name),
			Content: content, ContentDigest: digestText(content), LicenseSPDX: "LicenseRef-fixture",
			SourceURL: "https://example.invalid/" + name, SourceRevision: "revision-" + name,
			Redistribution: "reference_only", Visibility: restrictedReferenceVisibility, PublicReleasable: false,
		}
	}
	first, second := excerpt("first"), excerpt("second")
	material, err := SealCaseMaterial(CaseMaterial{
		ProtocolVersion: ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest,
		SourceCorpusDigest: plan.SourceCorpusDigest, CaseID: "mutation-" + digestText("case"), Family: mutation.FamilyCandidateOrderReversal,
		Unit: UnitCandidatePairOrders, TaskRequirement: "Compare the two candidate solutions.", TaskRequirementDigest: digestText("Compare the two candidate solutions."),
		Original: []EvidenceExcerpt{first, second}, Transformed: []EvidenceExcerpt{second, first}, AlignmentDigest: digestText("alignment"),
		ReplayReceiptDigest: digestText("replay"), Limitations: []string{"A limitation.", "B limitation.", "C limitation."},
		ExternalActionStatus: ExternalActionNotAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	caseID := material.CaseID
	context := packetContext{
		caseRecord: mutation.CorpusCase{
			ID: caseID, SourceIDs: []string{"source-" + digestText("first"), "source-" + digestText("second")},
			Family: mutation.FamilyCandidateOrderReversal, Split: "development", Control: "relation",
			Manifest: mutation.Manifest{
				MutationID: caseID, ExpectedRelation: mutation.RelationQualityEqual, SplitGroupID: "fixture-task-group",
				Witness: mutation.Witness{Digest: digestText("witness")}, Digest: digestText("manifest"),
			},
			BlindPacket: mutation.BlindReviewPacket{Digest: digestText("mutation-packet")}, RegenerationKey: digestText("regeneration"),
		},
		sources: []mutation.CorpusSource{
			{ID: "source-" + digestText("first"), TaskID: "task-first", RepositoryID: "repository-first", SourceFamily: "fixture-first", SourceLocation: "private/first", SourceRevision: "revision-first"},
			{ID: "source-" + digestText("second"), TaskID: "task-second", RepositoryID: "repository-second", SourceFamily: "fixture-second", SourceLocation: "private/second", SourceRevision: "revision-second"},
		},
	}
	return plan, material, context
}

func relationCandidateBlindingFixtureV2(t *testing.T) (Plan, CaseMaterial, packetContext) {
	t.Helper()
	_, legacyMaterial, context := relationCandidateBlindingFixture(t)
	plan := readGovernedPlan(t, "relation-audit-plan-v2.json")
	firewall := mutation.ConstructFirewallReport{
		SchemaVersion: mutation.ConstructFirewallSchemaVersion, CanonicalPolicy: mutation.CanonicalPolicy,
		ProgramVersion: mutation.MutationProgramVersionV2, Family: mutation.FamilyCandidateOrderReversal, Status: mutation.ConstructApplied,
		SourceTrajectoryDigest: digestText("v2-fixture-source"), MutatedTrajectoryDigest: digestText("v2-fixture-transformed"),
		TargetEventIDs: []string{}, ProofEventIDs: []string{}, SemanticRole: "candidate_pair_presentation",
		Checks:           []mutation.Check{{Name: "unordered_pair_identity", Expected: "preserved", Observed: "preserved", Passed: true}},
		RejectionReasons: []mutation.ConstructRejectionReason{},
	}
	var err error
	firewall.Digest, err = digestJSON(firewall)
	if err != nil {
		t.Fatal(err)
	}
	if err := firewall.Validate(); err != nil {
		t.Fatal(err)
	}
	material, err := SealCaseMaterial(CaseMaterial{
		ProtocolVersion: ProtocolVersionV2, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest,
		SourceCorpusDigest: plan.SourceCorpusDigest, SourceCorpusSpecDigest: plan.SourceCorpusSpecDigest,
		SourceMutationProgramDigest: plan.SourceMutationProgramDigest, SourceConstructAuditDigest: plan.SourceConstructAuditDigest,
		RelationContractVersion: mutation.RelationContractVersionV2, EvidenceBoundaryVersion: mutation.EvidenceBoundaryVersionV2,
		ConstructFirewallDigest: firewall.Digest, CaseID: legacyMaterial.CaseID, Family: legacyMaterial.Family, Unit: legacyMaterial.Unit,
		TaskRequirement: legacyMaterial.TaskRequirement, TaskRequirementDigest: legacyMaterial.TaskRequirementDigest,
		Original: legacyMaterial.Original, Transformed: legacyMaterial.Transformed, AlignmentDigest: legacyMaterial.AlignmentDigest,
		ReplayReceiptDigest: legacyMaterial.ReplayReceiptDigest, Limitations: legacyMaterial.Limitations,
		ExternalActionStatus: ExternalActionNotAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	context.caseRecord.ConstructFirewall = &firewall
	context.caseRecord.Manifest.ExpectedRelation = mutation.RelationQualityEqual
	return plan, material, context
}

func relationCandidateBlindingFixtureV3(t *testing.T) (Plan, CaseMaterial, packetContext) {
	t.Helper()
	_, legacyMaterial, context := relationCandidateBlindingFixture(t)
	governed := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	plan, err := ReviewPlanV3(governed)
	if err != nil {
		t.Fatal(err)
	}
	material, err := SealCaseMaterial(CaseMaterial{
		ProtocolVersion: ProtocolVersionV3, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest,
		SourceCorpusDigest: plan.SourceCorpusDigest, SourceCorpusPlanDigest: plan.SourceCorpusPlanDigest,
		SourceMutationProgramDigest: plan.SourceMutationProgramDigest, SourceConstructAuditDigest: plan.SourceConstructAuditDigest,
		RelationContractVersion: mutation.RelationContractVersionV3, EvidenceBoundaryVersion: mutation.EvidenceBoundaryVersionV3,
		ConstructFirewallDigest: digestText("v3-fixture-firewall"), CaseID: legacyMaterial.CaseID, Family: legacyMaterial.Family, Unit: legacyMaterial.Unit,
		TaskRequirement: legacyMaterial.TaskRequirement, TaskRequirementDigest: legacyMaterial.TaskRequirementDigest,
		Original: legacyMaterial.Original, Transformed: legacyMaterial.Transformed, AlignmentDigest: legacyMaterial.AlignmentDigest,
		ReplayReceiptDigest: legacyMaterial.ReplayReceiptDigest, Limitations: legacyMaterial.Limitations,
		ExternalActionStatus: ExternalActionNotAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan, material, context
}
