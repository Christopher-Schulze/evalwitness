package relation

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	BlindPacketSchemaVersionV1    = "evalwitness.relation-blind-packet.v1"
	BlindPacketSchemaVersionV2    = "evalwitness.relation-blind-packet.v2"
	BlindPacketSchemaVersionV3    = "evalwitness.relation-blind-packet.v3"
	PrivateMappingSchemaVersionV1 = "evalwitness.relation-private-mapping.v1"
	PrivateMappingSchemaVersionV2 = "evalwitness.relation-private-mapping.v2"
	PrivateMappingSchemaVersionV3 = "evalwitness.relation-private-mapping.v3"
	BlindPacketSchemaVersion      = BlindPacketSchemaVersionV1
	PrivateMappingSchemaVersion   = PrivateMappingSchemaVersionV1
	BlindingProtocolVersion       = "evalwitness.relation-hmac-sha256.v1"
	maximumBlindingKeyFileBytes   = 256

	domainCandidateLabel          = "evalwitness.relation.candidate-label.v1"
	domainEvidenceSlot            = "evalwitness.relation.evidence-slot.v1"
	domainPacketID                = "evalwitness.relation.packet-id.v1"
	domainPacketOrder             = "evalwitness.relation.packet-order.v1"
	domainReviewerAssignmentOrder = "evalwitness.relation.reviewer-assignment-order.v1"
	domainTaskAlias               = "evalwitness.relation.task-alias.v1"
	domainVisibleSide             = "evalwitness.relation.visible-side-identity.v1"
)

type VisiblePosition string

const (
	PositionLeft  VisiblePosition = "left"
	PositionRight VisiblePosition = "right"
)

type LogicalSide string

const (
	LogicalOriginal    LogicalSide = "original"
	LogicalTransformed LogicalSide = "transformed"
)

type RandomizationDomain struct {
	Purpose string `json:"purpose"`
	Domain  string `json:"domain"`
}

type PacketEvidence struct {
	SlotID               string `json:"slot_id"`
	CandidateLabel       string `json:"candidate_label,omitempty"`
	Content              string `json:"content"`
	ContentDigest        string `json:"content_digest"`
	EvidenceSelector     string `json:"evidence_selector"`
	SourceEvents         int    `json:"source_events"`
	RetainedEvents       int    `json:"retained_events"`
	OmittedEvents        int    `json:"omitted_events"`
	RedactionHits        int    `json:"redaction_hits"`
	EvidenceBudgetTokens int    `json:"evidence_budget_tokens"`
	LicenseSPDX          string `json:"license_spdx"`
	Redistribution       string `json:"redistribution"`
	Visibility           string `json:"visibility"`
	PublicReleasable     bool   `json:"public_releasable"`
}

type PacketSide struct {
	Position  VisiblePosition  `json:"position"`
	SideAlias string           `json:"side_alias"`
	Evidence  []PacketEvidence `json:"evidence"`
}

type BlindPacket struct {
	SchemaVersion           string               `json:"schema_version"`
	CanonicalPolicy         string               `json:"canonical_policy"`
	ProtocolVersion         string               `json:"protocol_version"`
	Objective               ReviewObjective      `json:"review_objective"`
	BlindingProtocol        string               `json:"blinding_protocol"`
	PacketID                string               `json:"packet_id"`
	PlanDigest              string               `json:"plan_digest"`
	TaskAlias               string               `json:"task_alias"`
	Unit                    UnitType             `json:"unit"`
	TaskRequirement         string               `json:"task_requirement"`
	TaskRequirementDigest   string               `json:"task_requirement_digest"`
	Sides                   []PacketSide         `json:"sides"`
	RubricVersion           string               `json:"rubric_version"`
	RubricQuestions         []AxisDefinition     `json:"rubric_questions"`
	PrivacyClass            string               `json:"privacy_class"`
	PublicReleasable        bool                 `json:"public_releasable"`
	Limitations             []string             `json:"limitations"`
	PacketOrderCommitment   string               `json:"packet_order_commitment"`
	ReviewerOrderCommitment string               `json:"reviewer_order_commitment"`
	ExternalActionStatus    ExternalActionStatus `json:"external_action_status"`
	Digest                  string               `json:"digest"`
}

type PrivateEvidenceMapping struct {
	SlotID                   string          `json:"slot_id"`
	CandidateLabel           string          `json:"candidate_label,omitempty"`
	VisiblePosition          VisiblePosition `json:"visible_position"`
	SideAlias                string          `json:"side_alias"`
	LogicalSide              LogicalSide     `json:"logical_side"`
	VisibleEvidenceIndex     int             `json:"visible_evidence_index"`
	SourceCandidateIndex     int             `json:"source_candidate_index"`
	SourceID                 string          `json:"source_id"`
	SourceTrajectoryDigest   string          `json:"source_trajectory_digest"`
	RetainedTrajectoryDigest string          `json:"retained_trajectory_digest"`
	RequiredEventIDs         []string        `json:"required_event_ids"`
	RetainedLineageDigest    string          `json:"retained_lineage_digest"`
	ContentDigest            string          `json:"content_digest"`
	SourceURL                string          `json:"source_url"`
	SourceRevision           string          `json:"source_revision"`
}

type PrivateMapping struct {
	SchemaVersion               string                   `json:"schema_version"`
	CanonicalPolicy             string                   `json:"canonical_policy"`
	ProtocolVersion             string                   `json:"protocol_version"`
	Objective                   ReviewObjective          `json:"review_objective"`
	BlindingProtocol            string                   `json:"blinding_protocol"`
	PacketID                    string                   `json:"packet_id"`
	PacketDigest                string                   `json:"packet_digest"`
	PlanDigest                  string                   `json:"plan_digest"`
	SourceCorpusDigest          string                   `json:"source_corpus_digest"`
	SourceCorpusSpecDigest      string                   `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest      string                   `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest string                   `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest  string                   `json:"source_construct_audit_digest,omitempty"`
	RelationContractVersion     string                   `json:"relation_contract_version,omitempty"`
	EvidenceBoundaryVersion     string                   `json:"evidence_boundary_version,omitempty"`
	ConstructFirewallDigest     string                   `json:"construct_firewall_digest,omitempty"`
	CaseMaterialDigest          string                   `json:"case_material_digest"`
	CaseID                      string                   `json:"case_id"`
	TaskAlias                   string                   `json:"task_alias"`
	SourceTaskGroupID           string                   `json:"source_task_group_id"`
	Family                      mutation.Family          `json:"family"`
	ExpectedRelation            mutation.Relation        `json:"expected_relation"`
	Unit                        UnitType                 `json:"unit"`
	SplitRole                   string                   `json:"split_role"`
	Control                     string                   `json:"control"`
	SourceIDs                   []string                 `json:"source_ids"`
	ManifestDigest              string                   `json:"manifest_digest"`
	WitnessDigest               string                   `json:"witness_digest"`
	MutationPacketDigest        string                   `json:"mutation_packet_digest"`
	RegenerationKey             string                   `json:"regeneration_key"`
	AlignmentDigest             string                   `json:"alignment_digest"`
	ReplayReceiptDigest         string                   `json:"replay_receipt_digest"`
	EvidenceMappings            []PrivateEvidenceMapping `json:"evidence_mappings"`
	RandomizationDomains        []RandomizationDomain    `json:"randomization_domains"`
	PacketOrderKey              string                   `json:"packet_order_key"`
	ReviewerOrderKey            string                   `json:"reviewer_order_key"`
	BlindingKeyID               string                   `json:"blinding_key_id"`
	ExternalActionStatus        ExternalActionStatus     `json:"external_action_status"`
	Digest                      string                   `json:"digest"`
}

type packetContext struct {
	caseRecord mutation.CorpusCase
	sources    []mutation.CorpusSource
}

type logicalPacketSide struct {
	role          LogicalSide
	alias         string
	excerpts      []EvidenceExcerpt
	sourceIndexes []int
}

func DecodeBlindingKey(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumBlindingKeyFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read relation blinding key: %w", err)
	}
	if len(raw) > maximumBlindingKeyFileBytes {
		return nil, errors.New("relation blinding key file exceeds 256-byte limit")
	}
	encoded := bytes.TrimSpace(raw)
	if len(encoded) != 64 || !bytes.Equal(encoded, bytes.ToLower(encoded)) {
		return nil, errors.New("relation blinding key must be exactly 64 lowercase hexadecimal characters")
	}
	key := make([]byte, 32)
	if _, err := hex.Decode(key, encoded); err != nil {
		return nil, errors.New("relation blinding key must be exactly 64 lowercase hexadecimal characters")
	}
	return key, nil
}

func BuildBlindedPacket(plan Plan, release mutation.CorpusRelease, material CaseMaterial, blindingKeyID string, key []byte) (BlindPacket, PrivateMapping, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	if err := release.Validate(); err != nil {
		return BlindPacket{}, PrivateMapping{}, fmt.Errorf("validate relation packet release: %w", err)
	}
	if err := material.Validate(); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	context, err := resolvePacketContext(plan, release, material)
	if err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	return buildBlindedPacket(plan, material, context, blindingKeyID, key)
}

func BuildBlindedPacketV3(plan Plan, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, material CaseMaterial, blindingKeyID string, key []byte) (BlindPacket, PrivateMapping, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	if plan.ProtocolVersion != ProtocolVersionV3 || plan.SchemaVersion != PlanSchemaVersionV3Adapter {
		return BlindPacket{}, PrivateMapping{}, errors.New("v3 relation packet requires the governed v3 review-plan adapter")
	}
	if err := release.Validate(corpusPlan, audit); err != nil {
		return BlindPacket{}, PrivateMapping{}, fmt.Errorf("validate v3 relation packet release: %w", err)
	}
	if err := material.Validate(); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	caseRecord, sources, err := materialCaseSourcesV3(release, material.CaseID)
	if err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	definition, exists := mutation.DefinitionFor(caseRecord.Family)
	if !exists || material.PlanDigest != plan.Digest || material.SourceCorpusDigest != release.Digest || material.SourceCorpusPlanDigest != corpusPlan.Digest ||
		material.SourceMutationProgramDigest != release.MutationProgramDigest || material.SourceConstructAuditDigest != audit.Digest ||
		material.Family != caseRecord.Family || material.Unit != unitForDefinition(definition) || material.ConstructFirewallDigest != caseRecord.ConstructFirewall.Digest {
		return BlindPacket{}, PrivateMapping{}, errors.New("v3 relation packet material disagrees with its governed corpus case or typed firewall")
	}
	legacyCase := legacyCorpusCaseV3(caseRecord)
	if err := validateMaterialSourceBindings(material, legacyCase, sources); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	context := packetContext{caseRecord: legacyCase, sources: sources}
	return buildBlindedPacket(plan, material, context, blindingKeyID, key)
}

func buildBlindedPacket(plan Plan, material CaseMaterial, context packetContext, blindingKeyID string, key []byte) (BlindPacket, PrivateMapping, error) {
	if len(key) < sha256.Size || strings.TrimSpace(blindingKeyID) == "" || len(blindingKeyID) > 128 {
		return BlindPacket{}, PrivateMapping{}, errors.New("relation packet requires a key ID and at least 32 blinding-key bytes")
	}
	caseRecord := context.caseRecord
	base := []string{plan.Digest, material.SourceCorpusDigest, material.Digest, caseRecord.ID, blindingKeyID}
	packetID := "relation-packet-" + relationKeyedDigest(key, domainPacketID, base...)
	taskAlias := "relation-task-" + relationKeyedDigest(key, domainTaskAlias, plan.Digest, material.SourceCorpusDigest, caseRecord.Manifest.SplitGroupID, blindingKeyID)
	packetOrderKey := relationKeyedDigest(key, domainPacketOrder, base...)
	reviewerOrderKey := relationKeyedDigest(key, domainReviewerAssignmentOrder, base...)

	logicalSides := []logicalPacketSide{
		{role: LogicalOriginal, excerpts: append([]EvidenceExcerpt(nil), material.Original...), sourceIndexes: sourceIndexes(material.Unit, LogicalOriginal, len(material.Original))},
		{role: LogicalTransformed, excerpts: append([]EvidenceExcerpt(nil), material.Transformed...), sourceIndexes: sourceIndexes(material.Unit, LogicalTransformed, len(material.Transformed))},
	}
	for index := range logicalSides {
		logicalSides[index].alias = "relation-side-" + relationKeyedDigest(key, domainVisibleSide, append(base, string(logicalSides[index].role))...)
	}
	sort.Slice(logicalSides, func(left, right int) bool { return logicalSides[left].alias < logicalSides[right].alias })
	if logicalSides[0].alias == logicalSides[1].alias {
		return BlindPacket{}, PrivateMapping{}, errors.New("relation side randomization produced a duplicate identity")
	}

	packetSides := make([]PacketSide, 0, 2)
	evidenceMappings := make([]PrivateEvidenceMapping, 0, len(material.Original)+len(material.Transformed))
	positions := []VisiblePosition{PositionLeft, PositionRight}
	for sideIndex, side := range logicalSides {
		position := positions[sideIndex]
		packetEvidence := make([]PacketEvidence, len(side.excerpts))
		for evidenceIndex, excerpt := range side.excerpts {
			sourceIndex := side.sourceIndexes[evidenceIndex]
			source := context.sources[sourceIndex]
			slotID := "relation-slot-" + relationKeyedDigest(key, domainEvidenceSlot, append(base, string(side.role), fmt.Sprint(evidenceIndex+1), excerpt.ContentDigest)...)
			candidateLabel := ""
			if material.Unit == UnitCandidatePairOrders {
				candidateLabel = "relation-candidate-" + relationKeyedDigest(key, domainCandidateLabel, append(base, string(side.role), fmt.Sprint(evidenceIndex+1), fmt.Sprint(sourceIndex+1), excerpt.ContentDigest)...)
			}
			packetEvidence[evidenceIndex] = PacketEvidence{
				SlotID: slotID, CandidateLabel: candidateLabel, Content: excerpt.Content, ContentDigest: excerpt.ContentDigest,
				EvidenceSelector: excerpt.EvidenceSelector, SourceEvents: excerpt.SourceEvents, RetainedEvents: excerpt.RetainedEvents,
				OmittedEvents: excerpt.OmittedEvents, RedactionHits: excerpt.RedactionHits, EvidenceBudgetTokens: excerpt.EvidenceBudgetTokens,
				LicenseSPDX: excerpt.LicenseSPDX, Redistribution: excerpt.Redistribution, Visibility: excerpt.Visibility,
				PublicReleasable: excerpt.PublicReleasable,
			}
			evidenceMappings = append(evidenceMappings, PrivateEvidenceMapping{
				SlotID: slotID, CandidateLabel: candidateLabel, VisiblePosition: position, SideAlias: side.alias, LogicalSide: side.role,
				VisibleEvidenceIndex: evidenceIndex + 1, SourceCandidateIndex: sourceIndex + 1, SourceID: source.ID,
				SourceTrajectoryDigest: excerpt.SourceTrajectoryDigest, RetainedTrajectoryDigest: excerpt.RetainedTrajectoryDigest,
				RequiredEventIDs: append([]string(nil), excerpt.RequiredEventIDs...), RetainedLineageDigest: excerpt.RetainedLineageDigest,
				ContentDigest: excerpt.ContentDigest, SourceURL: excerpt.SourceURL, SourceRevision: excerpt.SourceRevision,
			})
		}
		packetSides = append(packetSides, PacketSide{Position: position, SideAlias: side.alias, Evidence: packetEvidence})
	}
	packet := BlindPacket{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, BlindingProtocol: BlindingProtocolVersion,
		PacketID: packetID, PlanDigest: plan.Digest, TaskAlias: taskAlias, Unit: material.Unit,
		TaskRequirement: material.TaskRequirement, TaskRequirementDigest: material.TaskRequirementDigest, Sides: packetSides,
		RubricVersion: plan.RubricVersion, RubricQuestions: cloneAxisDefinitions(plan.Axes), PrivacyClass: restrictedReferenceVisibility,
		PublicReleasable: false, Limitations: append([]string(nil), material.Limitations...),
		PacketOrderCommitment: digestText(packetOrderKey), ReviewerOrderCommitment: digestText(reviewerOrderKey),
		ExternalActionStatus: ExternalActionNotAuthorized,
	}
	sealedPacket, err := SealBlindPacket(packet)
	if err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	forbidden := packetForbiddenValues(context, blindingKeyID)
	if err := ValidateBlindPacketLeakage(sealedPacket, forbidden); err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	mapping := PrivateMapping{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, BlindingProtocol: BlindingProtocolVersion,
		PacketID: sealedPacket.PacketID, PacketDigest: sealedPacket.Digest, PlanDigest: plan.Digest,
		SourceCorpusDigest: material.SourceCorpusDigest, CaseMaterialDigest: material.Digest, CaseID: caseRecord.ID,
		TaskAlias: taskAlias, SourceTaskGroupID: caseRecord.Manifest.SplitGroupID, Family: caseRecord.Family,
		ExpectedRelation: caseRecord.Manifest.ExpectedRelation, Unit: material.Unit, SplitRole: string(caseRecord.Split), Control: caseRecord.Control,
		SourceIDs: append([]string(nil), caseRecord.SourceIDs...), ManifestDigest: caseRecord.Manifest.Digest,
		WitnessDigest: caseRecord.Manifest.Witness.Digest, MutationPacketDigest: caseRecord.BlindPacket.Digest,
		RegenerationKey: caseRecord.RegenerationKey, AlignmentDigest: material.AlignmentDigest, ReplayReceiptDigest: material.ReplayReceiptDigest,
		EvidenceMappings: evidenceMappings, RandomizationDomains: relationRandomizationDomains(),
		PacketOrderKey: packetOrderKey, ReviewerOrderKey: reviewerOrderKey, BlindingKeyID: blindingKeyID,
		ExternalActionStatus: ExternalActionNotAuthorized,
	}
	switch plan.ProtocolVersion {
	case ProtocolVersionV2:
		mapping.SourceCorpusSpecDigest = material.SourceCorpusSpecDigest
		mapping.SourceMutationProgramDigest = material.SourceMutationProgramDigest
		mapping.SourceConstructAuditDigest = material.SourceConstructAuditDigest
		mapping.RelationContractVersion = material.RelationContractVersion
		mapping.EvidenceBoundaryVersion = material.EvidenceBoundaryVersion
		mapping.ConstructFirewallDigest = material.ConstructFirewallDigest
	case ProtocolVersionV3:
		mapping.SourceCorpusPlanDigest = material.SourceCorpusPlanDigest
		mapping.SourceMutationProgramDigest = material.SourceMutationProgramDigest
		mapping.SourceConstructAuditDigest = material.SourceConstructAuditDigest
		mapping.RelationContractVersion = material.RelationContractVersion
		mapping.EvidenceBoundaryVersion = material.EvidenceBoundaryVersion
		mapping.ConstructFirewallDigest = material.ConstructFirewallDigest
	}
	sealedMapping, err := SealPrivateMapping(mapping)
	if err != nil {
		return BlindPacket{}, PrivateMapping{}, err
	}
	return sealedPacket, sealedMapping, nil
}

func SealBlindPacket(packet BlindPacket) (BlindPacket, error) {
	schemaVersion, err := schemaVersionForProtocol(packet.ProtocolVersion, BlindPacketSchemaVersionV1, BlindPacketSchemaVersionV2, BlindPacketSchemaVersionV3)
	if err != nil {
		return BlindPacket{}, err
	}
	packet.SchemaVersion, packet.CanonicalPolicy, packet.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := blindPacketDigest(packet)
	if err != nil {
		return BlindPacket{}, err
	}
	packet.Digest = digest
	return packet, packet.Validate()
}

func (packet BlindPacket) Validate() error {
	if !validVersionedIdentity(packet.SchemaVersion, packet.ProtocolVersion, BlindPacketSchemaVersionV1, BlindPacketSchemaVersionV2, BlindPacketSchemaVersionV3) || packet.CanonicalPolicy != CanonicalPolicy ||
		packet.Objective != ReviewObjectiveControlledRelation || packet.BlindingProtocol != BlindingProtocolVersion || !validOpaqueID(packet.PacketID, "relation-packet-") ||
		!validDigest(packet.PlanDigest) || !validOpaqueID(packet.TaskAlias, "relation-task-") || !slices.Contains([]UnitType{UnitTrajectoryPair, UnitCandidatePairOrders}, packet.Unit) ||
		strings.TrimSpace(packet.TaskRequirement) == "" || packet.TaskRequirementDigest != digestText(packet.TaskRequirement) || len(packet.Sides) != 2 ||
		!validRubricVersion(packet.ProtocolVersion, packet.RubricVersion) || !axisDefinitionsEqual(packet.RubricQuestions, defaultAxes()) ||
		packet.PrivacyClass != restrictedReferenceVisibility || packet.PublicReleasable || len(packet.Limitations) != 3 || !slices.IsSorted(packet.Limitations) ||
		!validDigest(packet.PacketOrderCommitment) || !validDigest(packet.ReviewerOrderCommitment) || packet.PacketOrderCommitment == packet.ReviewerOrderCommitment ||
		packet.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation blind packet identity, objective, task, rubric, privacy, randomization, or authorization boundary is invalid")
	}
	wantPositions := []VisiblePosition{PositionLeft, PositionRight}
	seenAliases := make(map[string]struct{}, 2)
	seenSlots := make(map[string]struct{})
	seenCandidates := make(map[string]struct{})
	expectedArity := 1
	if packet.Unit == UnitCandidatePairOrders {
		expectedArity = 2
	}
	for sideIndex, side := range packet.Sides {
		if side.Position != wantPositions[sideIndex] || !validOpaqueID(side.SideAlias, "relation-side-") || len(side.Evidence) != expectedArity {
			return errors.New("relation blind packet side identity, order, or arity is invalid")
		}
		if _, exists := seenAliases[side.SideAlias]; exists {
			return errors.New("relation blind packet side aliases are not unique")
		}
		seenAliases[side.SideAlias] = struct{}{}
		for _, evidence := range side.Evidence {
			if !validOpaqueID(evidence.SlotID, "relation-slot-") || strings.TrimSpace(evidence.Content) == "" || evidence.ContentDigest != digestText(evidence.Content) ||
				evidence.EvidenceSelector != RelationEvidenceSelectorVersion || evidence.SourceEvents < 1 || evidence.RetainedEvents < 1 ||
				evidence.RetainedEvents > evidence.SourceEvents || evidence.OmittedEvents != evidence.SourceEvents-evidence.RetainedEvents || evidence.RedactionHits < 0 ||
				evidence.EvidenceBudgetTokens != RelationEvidenceBudgetTokens || strings.TrimSpace(evidence.LicenseSPDX) == "" || evidence.Redistribution != "reference_only" ||
				evidence.Visibility != restrictedReferenceVisibility || evidence.PublicReleasable {
				return errors.New("relation blind packet evidence content, accounting, license, or visibility is invalid")
			}
			if _, exists := seenSlots[evidence.SlotID]; exists {
				return errors.New("relation blind packet evidence slots are not unique")
			}
			seenSlots[evidence.SlotID] = struct{}{}
			if packet.Unit == UnitTrajectoryPair && evidence.CandidateLabel != "" || packet.Unit == UnitCandidatePairOrders && !validOpaqueID(evidence.CandidateLabel, "relation-candidate-") {
				return errors.New("relation blind packet candidate label does not match its unit")
			}
			if evidence.CandidateLabel != "" {
				if _, exists := seenCandidates[evidence.CandidateLabel]; exists {
					return errors.New("relation blind packet candidate labels are not independently randomized")
				}
				seenCandidates[evidence.CandidateLabel] = struct{}{}
			}
		}
	}
	expected, err := blindPacketDigest(packet)
	if err != nil || packet.Digest != expected {
		return errors.New("relation blind packet digest is invalid")
	}
	return validateBlindPacketKeys(packet)
}

func SealPrivateMapping(mapping PrivateMapping) (PrivateMapping, error) {
	schemaVersion, err := schemaVersionForProtocol(mapping.ProtocolVersion, PrivateMappingSchemaVersionV1, PrivateMappingSchemaVersionV2, PrivateMappingSchemaVersionV3)
	if err != nil {
		return PrivateMapping{}, err
	}
	mapping.SchemaVersion, mapping.CanonicalPolicy, mapping.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := privateMappingDigest(mapping)
	if err != nil {
		return PrivateMapping{}, err
	}
	mapping.Digest = digest
	return mapping, mapping.Validate()
}

func (mapping PrivateMapping) Validate() error {
	definition, familyExists := mutation.DefinitionFor(mapping.Family)
	if !validVersionedIdentity(mapping.SchemaVersion, mapping.ProtocolVersion, PrivateMappingSchemaVersionV1, PrivateMappingSchemaVersionV2, PrivateMappingSchemaVersionV3) || mapping.CanonicalPolicy != CanonicalPolicy ||
		mapping.Objective != ReviewObjectiveControlledRelation || mapping.BlindingProtocol != BlindingProtocolVersion || !validOpaqueID(mapping.PacketID, "relation-packet-") ||
		!validDigest(mapping.PacketDigest) || !validDigest(mapping.PlanDigest) || !validDigest(mapping.SourceCorpusDigest) || !validDigest(mapping.CaseMaterialDigest) ||
		strings.TrimSpace(mapping.CaseID) == "" || !validOpaqueID(mapping.TaskAlias, "relation-task-") || strings.TrimSpace(mapping.SourceTaskGroupID) == "" ||
		!familyExists || mapping.ExpectedRelation != definition.Relation || !slices.Contains([]UnitType{UnitTrajectoryPair, UnitCandidatePairOrders}, mapping.Unit) ||
		strings.TrimSpace(mapping.SplitRole) == "" || strings.TrimSpace(mapping.Control) == "" || len(mapping.SourceIDs) == 0 ||
		!validDigest(mapping.ManifestDigest) || !validDigest(mapping.WitnessDigest) || !validDigest(mapping.MutationPacketDigest) || !validDigest(mapping.RegenerationKey) ||
		!validDigest(mapping.AlignmentDigest) || !validDigest(mapping.ReplayReceiptDigest) || len(mapping.EvidenceMappings) == 0 ||
		!slices.Equal(mapping.RandomizationDomains, relationRandomizationDomains()) || !validDigest(mapping.PacketOrderKey) || !validDigest(mapping.ReviewerOrderKey) ||
		mapping.PacketOrderKey == mapping.ReviewerOrderKey || strings.TrimSpace(mapping.BlindingKeyID) == "" || mapping.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation private mapping identity, hidden relation, custody, randomization, or authorization boundary is invalid")
	}
	if mapping.ProtocolVersion == ProtocolVersionV1 {
		if mapping.SourceCorpusSpecDigest != "" || mapping.SourceCorpusPlanDigest != "" || mapping.SourceMutationProgramDigest != "" || mapping.SourceConstructAuditDigest != "" ||
			mapping.RelationContractVersion != "" || mapping.EvidenceBoundaryVersion != "" || mapping.ConstructFirewallDigest != "" {
			return errors.New("v1 relation private mapping contains v2-only source or construct bindings")
		}
	} else if mapping.ProtocolVersion == ProtocolVersionV2 && (!validDigest(mapping.SourceCorpusSpecDigest) || mapping.SourceCorpusPlanDigest != "" || !validDigest(mapping.SourceMutationProgramDigest) || !validDigest(mapping.SourceConstructAuditDigest) ||
		mapping.RelationContractVersion != mutation.RelationContractVersionV2 || mapping.EvidenceBoundaryVersion != mutation.EvidenceBoundaryVersionV2 ||
		!validDigest(mapping.ConstructFirewallDigest)) {
		return errors.New("v2 relation private mapping lacks its corpus, program, contract, evidence-boundary, or construct-firewall binding")
	} else if mapping.ProtocolVersion == ProtocolVersionV3 && (mapping.SourceCorpusSpecDigest != "" || !validDigest(mapping.SourceCorpusPlanDigest) ||
		!validDigest(mapping.SourceMutationProgramDigest) || !validDigest(mapping.SourceConstructAuditDigest) || mapping.RelationContractVersion != mutation.RelationContractVersionV3 ||
		mapping.EvidenceBoundaryVersion != mutation.EvidenceBoundaryVersionV3 || !validDigest(mapping.ConstructFirewallDigest)) {
		return errors.New("v3 relation private mapping lacks its corpus plan, program, contract, evidence-boundary, or typed construct-firewall binding")
	}
	expectedUnit := UnitTrajectoryPair
	expectedSources, expectedEvidence := 1, 2
	if definition.PairLevel {
		expectedUnit, expectedSources, expectedEvidence = UnitCandidatePairOrders, 2, 4
	}
	if mapping.Unit != expectedUnit || len(mapping.SourceIDs) != expectedSources || len(mapping.EvidenceMappings) != expectedEvidence || hasDuplicate(mapping.SourceIDs) {
		return errors.New("relation private mapping unit, source, or evidence arity is invalid")
	}
	seenSlots := make(map[string]struct{}, len(mapping.EvidenceMappings))
	seenCandidates := make(map[string]struct{}, len(mapping.EvidenceMappings))
	for index, evidence := range mapping.EvidenceMappings {
		if !validOpaqueID(evidence.SlotID, "relation-slot-") || !validOpaqueID(evidence.SideAlias, "relation-side-") ||
			!slices.Contains([]VisiblePosition{PositionLeft, PositionRight}, evidence.VisiblePosition) || !slices.Contains([]LogicalSide{LogicalOriginal, LogicalTransformed}, evidence.LogicalSide) ||
			evidence.VisibleEvidenceIndex < 1 || evidence.VisibleEvidenceIndex > expectedSources || evidence.SourceCandidateIndex < 1 || evidence.SourceCandidateIndex > expectedSources ||
			!slices.Contains(mapping.SourceIDs, evidence.SourceID) || !validDigest(evidence.SourceTrajectoryDigest) || !validDigest(evidence.RetainedTrajectoryDigest) ||
			len(evidence.RequiredEventIDs) == 0 || !slices.IsSorted(evidence.RequiredEventIDs) || hasDuplicate(evidence.RequiredEventIDs) ||
			!validDigest(evidence.RetainedLineageDigest) || !validDigest(evidence.ContentDigest) || strings.TrimSpace(evidence.SourceURL) == "" || strings.TrimSpace(evidence.SourceRevision) == "" {
			return fmt.Errorf("relation private mapping evidence %d is invalid", index)
		}
		if _, exists := seenSlots[evidence.SlotID]; exists {
			return errors.New("relation private mapping evidence slots are not unique")
		}
		seenSlots[evidence.SlotID] = struct{}{}
		if mapping.Unit == UnitTrajectoryPair && evidence.CandidateLabel != "" || mapping.Unit == UnitCandidatePairOrders && !validOpaqueID(evidence.CandidateLabel, "relation-candidate-") {
			return errors.New("relation private mapping candidate label does not match its unit")
		}
		if evidence.CandidateLabel != "" {
			if _, exists := seenCandidates[evidence.CandidateLabel]; exists {
				return errors.New("relation private mapping candidate labels are not independently randomized")
			}
			seenCandidates[evidence.CandidateLabel] = struct{}{}
		}
	}
	if _, err := leftLogicalSide(mapping); err != nil {
		return err
	}
	expected, err := privateMappingDigest(mapping)
	if err != nil || mapping.Digest != expected {
		return errors.New("relation private mapping digest is invalid")
	}
	return nil
}

func ValidateBlindPacketLeakage(packet BlindPacket, forbiddenValues []string) error {
	if err := packet.Validate(); err != nil {
		return err
	}
	encoded, err := EncodeIndented(packet)
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(encoded, &document); err != nil {
		return err
	}
	forbiddenKeys := map[string]struct{}{
		"case_id": {}, "control": {}, "expected_relation": {}, "family": {}, "manifest_digest": {}, "mapping": {},
		"mutation_operator": {}, "provider_identity": {}, "regeneration_key": {}, "reviewer_order_key": {}, "source_ids": {},
		"source_revision": {}, "source_trajectory_digest": {}, "source_url": {}, "split": {}, "split_role": {}, "validator": {},
		"verifier_confidence": {}, "verifier_decision": {}, "verifier_score": {}, "witness_digest": {},
	}
	if key := firstForbiddenJSONKey(document, forbiddenKeys); key != "" {
		return fmt.Errorf("relation blind packet contains forbidden field %q", key)
	}
	haystack := strings.ToLower(string(encoded))
	for _, value := range forbiddenValues {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && strings.Contains(haystack, value) {
			return fmt.Errorf("relation blind packet contains forbidden private value %q", value)
		}
	}
	return nil
}

func resolvePacketContext(plan Plan, release mutation.CorpusRelease, material CaseMaterial) (packetContext, error) {
	if release.Digest != plan.SourceCorpusDigest || material.PlanDigest != plan.Digest || material.SourceCorpusDigest != release.Digest {
		return packetContext{}, errors.New("relation packet plan, corpus, and material bindings disagree")
	}
	caseRecord, sources, err := materialCaseSources(release, material.CaseID)
	if err != nil {
		return packetContext{}, err
	}
	definition, exists := mutation.DefinitionFor(caseRecord.Family)
	if !exists || material.Family != caseRecord.Family || material.Unit != unitForDefinition(definition) || caseRecord.Manifest.ExpectedRelation != definition.Relation {
		return packetContext{}, errors.New("relation packet material disagrees with its frozen family, unit, or expected relation")
	}
	if material.ProtocolVersion != plan.ProtocolVersion {
		return packetContext{}, errors.New("relation packet material protocol differs from its plan")
	}
	if plan.ProtocolVersion == ProtocolVersionV2 && (caseRecord.ConstructFirewall == nil || material.SourceCorpusSpecDigest != plan.SourceCorpusSpecDigest ||
		material.SourceMutationProgramDigest != plan.SourceMutationProgramDigest || material.SourceConstructAuditDigest != plan.SourceConstructAuditDigest ||
		material.ConstructFirewallDigest != caseRecord.ConstructFirewall.Digest) {
		return packetContext{}, errors.New("v2 relation packet material source or construct binding disagrees with the governed case")
	}
	if err := validateMaterialSourceBindings(material, caseRecord, sources); err != nil {
		return packetContext{}, err
	}
	return packetContext{caseRecord: caseRecord, sources: sources}, nil
}

func validateMaterialSourceBindings(material CaseMaterial, caseRecord mutation.CorpusCase, sources []mutation.CorpusSource) error {
	originalDigests := make([]string, len(material.Original))
	transformedDigests := make([]string, len(material.Transformed))
	for index := range material.Original {
		originalDigests[index] = material.Original[index].SourceTrajectoryDigest
		transformedDigests[index] = material.Transformed[index].SourceTrajectoryDigest
	}
	if material.Unit == UnitTrajectoryPair {
		if len(sources) != 1 || originalDigests[0] != sources[0].TrajectoryDigest || originalDigests[0] != caseRecord.Manifest.OriginalTrajectoryDigest ||
			transformedDigests[0] != caseRecord.Manifest.MutatedTrajectoryDigest || !excerptLicenseMatches(material.Original[0], sources[0]) || !excerptLicenseMatches(material.Transformed[0], sources[0]) {
			return errors.New("relation trajectory packet material does not match its frozen source, transformation, or license")
		}
		return nil
	}
	if len(sources) != 2 || !slices.Equal(originalDigests, []string{sources[0].TrajectoryDigest, sources[1].TrajectoryDigest}) ||
		!slices.Equal(transformedDigests, []string{sources[1].TrajectoryDigest, sources[0].TrajectoryDigest}) {
		return errors.New("relation candidate-order packet material does not match its frozen source ordering")
	}
	originalDigest, err := digestJSON(originalDigests)
	if err != nil || originalDigest != caseRecord.Manifest.OriginalTrajectoryDigest {
		return errors.New("relation candidate-order packet original material digest is invalid")
	}
	transformedDigest, err := digestJSON(transformedDigests)
	if err != nil || transformedDigest != caseRecord.Manifest.MutatedTrajectoryDigest {
		return errors.New("relation candidate-order packet transformed material digest is invalid")
	}
	for index := range sources {
		if !excerptLicenseMatches(material.Original[index], sources[index]) || !excerptLicenseMatches(material.Transformed[1-index], sources[index]) {
			return errors.New("relation candidate-order packet license binding is invalid")
		}
	}
	return nil
}

func excerptLicenseMatches(excerpt EvidenceExcerpt, source mutation.CorpusSource) bool {
	return excerpt.LicenseSPDX == source.License.SPDX && excerpt.SourceURL == source.License.SourceURL &&
		excerpt.SourceRevision == source.License.SourceRevision && excerpt.Redistribution == source.License.Redistribution
}

func sourceIndexes(unit UnitType, side LogicalSide, arity int) []int {
	result := make([]int, arity)
	for index := range result {
		result[index] = index
	}
	if unit == UnitCandidatePairOrders && side == LogicalTransformed {
		slices.Reverse(result)
	}
	return result
}

func packetForbiddenValues(context packetContext, blindingKeyID string) []string {
	values := []string{
		context.caseRecord.ID, string(context.caseRecord.Family), string(context.caseRecord.Manifest.ExpectedRelation),
		context.caseRecord.Manifest.MutationID, context.caseRecord.Manifest.Digest, context.caseRecord.BlindPacket.Digest,
		context.caseRecord.RegenerationKey, blindingKeyID,
	}
	for _, source := range context.sources {
		values = append(values, source.ID, source.TaskID, source.RepositoryID, source.SourceFamily, source.SourceLocation, source.SourceRevision)
	}
	return uniqueSorted(values)
}

func relationRandomizationDomains() []RandomizationDomain {
	return []RandomizationDomain{
		{Purpose: "candidate_label", Domain: domainCandidateLabel},
		{Purpose: "evidence_slot_identity", Domain: domainEvidenceSlot},
		{Purpose: "packet_id", Domain: domainPacketID},
		{Purpose: "packet_order", Domain: domainPacketOrder},
		{Purpose: "reviewer_assignment_order", Domain: domainReviewerAssignmentOrder},
		{Purpose: "task_alias", Domain: domainTaskAlias},
		{Purpose: "visible_side_identity", Domain: domainVisibleSide},
	}
}

func relationKeyedDigest(key []byte, domain string, values ...string) string {
	mac := hmac.New(sha256.New, key)
	writeLengthPrefixed(mac, domain)
	for _, value := range values {
		writeLengthPrefixed(mac, value)
	}
	return hex.EncodeToString(mac.Sum(nil))
}

func writeLengthPrefixed(writer io.Writer, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(value))
}

func validOpaqueID(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && validDigest(strings.TrimPrefix(value, prefix))
}

func cloneAxisDefinitions(values []AxisDefinition) []AxisDefinition {
	result := make([]AxisDefinition, len(values))
	for index := range values {
		result[index] = values[index]
		result[index].AllowedRatings = append([]Rating(nil), values[index].AllowedRatings...)
	}
	return result
}

func axisDefinitionsEqual(left, right []AxisDefinition) bool {
	return slices.EqualFunc(left, right, func(a, b AxisDefinition) bool {
		return a.ID == b.ID && a.Question == b.Question && slices.Equal(a.AllowedRatings, b.AllowedRatings)
	})
}

func validateBlindPacketKeys(packet BlindPacket) error {
	if packet.Unit == UnitCandidatePairOrders {
		left := packet.Sides[0].Evidence
		right := packet.Sides[1].Evidence
		leftDigests := []string{left[0].ContentDigest, left[1].ContentDigest}
		rightDigests := []string{right[0].ContentDigest, right[1].ContentDigest}
		if !slices.Equal(leftDigests, []string{rightDigests[1], rightDigests[0]}) {
			return errors.New("relation candidate-order packet collapsed or changed its pair-of-pairs reversal")
		}
	}
	return nil
}

func firstForbiddenJSONKey(value any, forbidden map[string]struct{}) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, exists := forbidden[key]; exists {
				return key
			}
			if found := firstForbiddenJSONKey(child, forbidden); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := firstForbiddenJSONKey(child, forbidden); found != "" {
				return found
			}
		}
	}
	return ""
}

func unitForDefinition(definition mutation.Definition) UnitType {
	if definition.PairLevel {
		return UnitCandidatePairOrders
	}
	return UnitTrajectoryPair
}

func blindPacketDigest(packet BlindPacket) (string, error) {
	packet.Digest = ""
	return digestJSON(packet)
}

func privateMappingDigest(mapping PrivateMapping) (string, error) {
	mapping.Digest = ""
	return digestJSON(mapping)
}
