package relation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	RelationPilotReadinessSchemaVersionV1 = "evalwitness.relation-pilot-readiness.v1"
	RelationPilotReadinessSchemaVersionV2 = "evalwitness.relation-pilot-readiness.v2"
	RelationPilotReadinessSchemaVersionV3 = "evalwitness.relation-pilot-readiness.v3"
	RelationPilotReadinessSchemaVersion   = RelationPilotReadinessSchemaVersionV1
	RelationPilotTechnicalReady           = "structurally_ready_for_owner_semantic_review"
	RelationPilotSemanticPending          = "requires_owner_manual_inspection"
	RelationPilotRequiredExternalAction   = "Owner must inspect every restricted packet and rendered review surface, approve reviewer population and labor terms, and explicitly authorize contact or distribution; this artifact authorizes none of those actions."
)

type RelationPacketReadiness struct {
	PacketID              string          `json:"packet_id"`
	PacketDigest          string          `json:"packet_digest"`
	MappingDigest         string          `json:"mapping_digest"`
	CaseID                string          `json:"case_id"`
	Family                mutation.Family `json:"family"`
	Unit                  UnitType        `json:"unit"`
	TaskGroupID           string          `json:"task_group_id"`
	TaskRequirementDigest string          `json:"task_requirement_digest"`
	EvidenceSlots         int             `json:"evidence_slots"`
	PairOrderStatus       string          `json:"pair_order_status"`
	TaskContextStatus     string          `json:"task_context_status"`
	RedistributionStatus  string          `json:"redistribution_status"`
	LeakageScanStatus     string          `json:"leakage_scan_status"`
	StructuralStatus      string          `json:"structural_status"`
}

type RelationPilotReadiness struct {
	SchemaVersion                     string                    `json:"schema_version"`
	CanonicalPolicy                   string                    `json:"canonical_policy"`
	ProtocolVersion                   string                    `json:"protocol_version"`
	Objective                         ReviewObjective           `json:"review_objective"`
	PlanDigest                        string                    `json:"plan_digest"`
	SourceCorpusDigest                string                    `json:"source_corpus_digest,omitempty"`
	SourceCorpusSpecDigest            string                    `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest            string                    `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest       string                    `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest        string                    `json:"source_construct_audit_digest,omitempty"`
	ConstructFirewallCommitmentDigest string                    `json:"construct_firewall_commitment_digest,omitempty"`
	PilotSampleDigest                 string                    `json:"pilot_sample_digest"`
	QualificationSetDigest            string                    `json:"qualification_set_digest"`
	HandbookDigest                    string                    `json:"handbook_digest"`
	BundleDigest                      string                    `json:"bundle_digest"`
	DataRole                          ReviewDataRole            `json:"data_role"`
	Visibility                        ReviewVisibility          `json:"visibility"`
	Packets                           int                       `json:"packets"`
	MappingReferences                 []MappingReference        `json:"mapping_references"`
	MappingCommitmentDigest           string                    `json:"mapping_commitment_digest"`
	PacketChecks                      []RelationPacketReadiness `json:"packet_checks"`
	RequiredPrimaryReviewers          int                       `json:"required_primary_reviewers"`
	RequiredTieBreakReviewers         int                       `json:"required_tie_break_reviewers"`
	RequiredPrimaryJudgments          int                       `json:"required_primary_judgments"`
	MaximumTieBreakJudgments          int                       `json:"maximum_tie_break_judgments"`
	RequiredPostLabelProbes           int                       `json:"required_post_label_probes"`
	TechnicalStatus                   string                    `json:"technical_status"`
	SemanticInspectionStatus          string                    `json:"semantic_inspection_status"`
	ExternalActionStatus              ExternalActionStatus      `json:"external_action_status"`
	RequiredExternalAction            string                    `json:"required_external_action"`
	PreparedAt                        string                    `json:"prepared_at"`
	Limitations                       []string                  `json:"limitations"`
	Digest                            string                    `json:"digest"`
}

func BuildRelationPilotReadiness(plan Plan, pilot PilotSample, bundle ReviewBundle, mappings []PrivateMapping, qualification QualificationSet, handbook ReviewerHandbook, preparedAt string) (RelationPilotReadiness, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return RelationPilotReadiness{}, err
	}
	if err := pilot.Validate(); err != nil {
		return RelationPilotReadiness{}, err
	}
	if err := qualification.Validate(); err != nil {
		return RelationPilotReadiness{}, err
	}
	if err := VerifyReviewerHandbook(handbook, plan, qualification); err != nil {
		return RelationPilotReadiness{}, err
	}
	if err := VerifyReviewBundle(bundle, mappings); err != nil {
		return RelationPilotReadiness{}, err
	}
	if pilot.PlanDigest != plan.Digest || bundle.PlanDigest != plan.Digest || bundle.SampleDigest != pilot.Digest || bundle.DataRole != ReviewDataDevelopmentPilot ||
		bundle.Visibility != ReviewVisibilityRestricted || bundle.QualificationSetDigest != qualification.Digest || bundle.HandbookDigest != handbook.Digest || len(bundle.Packets) != pilot.SelectedCases {
		return RelationPilotReadiness{}, errors.New("relation pilot readiness inputs do not share the governed plan, sample, restricted bundle, and review policy")
	}
	if err := requireRelationTimeAfter("relation pilot readiness", preparedAt, bundle.CreatedAt); err != nil {
		return RelationPilotReadiness{}, err
	}
	references, mappingByPacket, err := relationMappingReferences(bundle, mappings)
	if err != nil {
		return RelationPilotReadiness{}, err
	}
	mappingCommitment, err := digestJSON(references)
	if err != nil {
		return RelationPilotReadiness{}, err
	}
	pilotByCase := make(map[string]PilotCaseReference, len(pilot.Cases))
	for _, item := range pilot.Cases {
		pilotByCase[item.CaseID] = item
	}
	checks := make([]RelationPacketReadiness, len(bundle.Packets))
	seenCases := make(map[string]struct{}, len(bundle.Packets))
	for index, packet := range bundle.Packets {
		mapping := mappingByPacket[packet.PacketID]
		pilotCase, exists := pilotByCase[mapping.CaseID]
		if !exists || pilotCase.Family != mapping.Family || pilotCase.Unit != mapping.Unit || pilotCase.TaskGroupID != mapping.SourceTaskGroupID {
			return RelationPilotReadiness{}, errors.New("relation pilot readiness packet mapping differs from its frozen pilot case")
		}
		if _, duplicate := seenCases[mapping.CaseID]; duplicate {
			return RelationPilotReadiness{}, errors.New("relation pilot readiness reuses a pilot case")
		}
		seenCases[mapping.CaseID] = struct{}{}
		check, checkErr := buildRelationPacketReadiness(packet, mapping)
		if checkErr != nil {
			return RelationPilotReadiness{}, checkErr
		}
		checks[index] = check
	}
	if len(seenCases) != len(pilot.Cases) {
		return RelationPilotReadiness{}, errors.New("relation pilot readiness does not cover every frozen pilot case")
	}
	sort.Slice(checks, func(left, right int) bool { return checks[left].PacketID < checks[right].PacketID })
	readiness := RelationPilotReadiness{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest, PilotSampleDigest: pilot.Digest,
		QualificationSetDigest: qualification.Digest, HandbookDigest: handbook.Digest, BundleDigest: bundle.Digest,
		DataRole: bundle.DataRole, Visibility: bundle.Visibility, Packets: len(bundle.Packets), MappingReferences: references,
		MappingCommitmentDigest: mappingCommitment, PacketChecks: checks, RequiredPrimaryReviewers: plan.PrimaryReviewers,
		RequiredTieBreakReviewers: plan.TieBreakReviewers, RequiredPrimaryJudgments: pilot.RequiredPrimaryLabels,
		MaximumTieBreakJudgments: pilot.MaximumTieBreakLabels, RequiredPostLabelProbes: pilot.RequiredPostLabelProbes,
		TechnicalStatus: RelationPilotTechnicalReady, SemanticInspectionStatus: RelationPilotSemanticPending,
		ExternalActionStatus: ExternalActionNotAuthorized, RequiredExternalAction: RelationPilotRequiredExternalAction,
		PreparedAt: preparedAt, Limitations: relationPilotReadinessLimitations(),
	}
	switch plan.ProtocolVersion {
	case ProtocolVersionV2:
		constructCommitment, commitmentErr := relationConstructFirewallCommitment(bundle, mappingByPacket)
		if commitmentErr != nil {
			return RelationPilotReadiness{}, commitmentErr
		}
		readiness.SourceCorpusDigest = plan.SourceCorpusDigest
		readiness.SourceCorpusSpecDigest = plan.SourceCorpusSpecDigest
		readiness.SourceMutationProgramDigest = plan.SourceMutationProgramDigest
		readiness.SourceConstructAuditDigest = plan.SourceConstructAuditDigest
		readiness.ConstructFirewallCommitmentDigest = constructCommitment
	case ProtocolVersionV3:
		constructCommitment, commitmentErr := relationConstructFirewallCommitment(bundle, mappingByPacket)
		if commitmentErr != nil {
			return RelationPilotReadiness{}, commitmentErr
		}
		readiness.SourceCorpusDigest = plan.SourceCorpusDigest
		readiness.SourceCorpusPlanDigest = plan.SourceCorpusPlanDigest
		readiness.SourceMutationProgramDigest = plan.SourceMutationProgramDigest
		readiness.SourceConstructAuditDigest = plan.SourceConstructAuditDigest
		readiness.ConstructFirewallCommitmentDigest = constructCommitment
	}
	return SealRelationPilotReadiness(readiness)
}

func buildRelationPacketReadiness(packet BlindPacket, mapping PrivateMapping) (RelationPacketReadiness, error) {
	if err := ValidateBlindPacketLeakage(packet, nil); err != nil {
		return RelationPacketReadiness{}, err
	}
	if _, err := leftLogicalSide(mapping); err != nil {
		return RelationPacketReadiness{}, err
	}
	evidenceSlots := 0
	for _, side := range packet.Sides {
		for _, evidence := range side.Evidence {
			if evidence.Redistribution != "reference_only" || evidence.Visibility != restrictedReferenceVisibility || evidence.PublicReleasable {
				return RelationPacketReadiness{}, errors.New("relation pilot readiness packet contains redistributable or public evidence")
			}
			evidenceSlots++
		}
	}
	pairStatus := "not_applicable"
	if packet.Unit == UnitCandidatePairOrders {
		pairStatus = "exact_reversal_verified"
	}
	return RelationPacketReadiness{
		PacketID: packet.PacketID, PacketDigest: packet.Digest, MappingDigest: mapping.Digest, CaseID: mapping.CaseID,
		Family: mapping.Family, Unit: mapping.Unit, TaskGroupID: mapping.SourceTaskGroupID, TaskRequirementDigest: packet.TaskRequirementDigest,
		EvidenceSlots: evidenceSlots, PairOrderStatus: pairStatus, TaskContextStatus: "present_digest_bound",
		RedistributionStatus: "restricted_reference_only", LeakageScanStatus: "passed", StructuralStatus: "ready_for_owner_semantic_review",
	}, nil
}

func SealRelationPilotReadiness(readiness RelationPilotReadiness) (RelationPilotReadiness, error) {
	schemaVersion, err := schemaVersionForProtocol(readiness.ProtocolVersion, RelationPilotReadinessSchemaVersionV1, RelationPilotReadinessSchemaVersionV2, RelationPilotReadinessSchemaVersionV3)
	if err != nil {
		return RelationPilotReadiness{}, err
	}
	readiness.SchemaVersion, readiness.CanonicalPolicy, readiness.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := relationPilotReadinessDigest(readiness)
	if err != nil {
		return RelationPilotReadiness{}, err
	}
	readiness.Digest = digest
	return readiness, readiness.Validate()
}

func (readiness RelationPilotReadiness) Validate() error {
	expectedPackets, expectedPrimaryJudgments, expectedTieBreakJudgments, expectedProbes := 8, 16, 8, 16
	if readiness.ProtocolVersion == ProtocolVersionV3 {
		expectedPackets, expectedPrimaryJudgments, expectedTieBreakJudgments, expectedProbes = 7, 14, 7, 14
	}
	if !validVersionedIdentity(readiness.SchemaVersion, readiness.ProtocolVersion, RelationPilotReadinessSchemaVersionV1, RelationPilotReadinessSchemaVersionV2, RelationPilotReadinessSchemaVersionV3) || readiness.CanonicalPolicy != CanonicalPolicy ||
		readiness.Objective != ReviewObjectiveControlledRelation || !validDigest(readiness.PlanDigest) || !validDigest(readiness.PilotSampleDigest) ||
		!validDigest(readiness.QualificationSetDigest) || !validDigest(readiness.HandbookDigest) || !validDigest(readiness.BundleDigest) ||
		readiness.DataRole != ReviewDataDevelopmentPilot || readiness.Visibility != ReviewVisibilityRestricted || readiness.Packets != expectedPackets ||
		len(readiness.MappingReferences) != readiness.Packets || len(readiness.PacketChecks) != readiness.Packets || !validDigest(readiness.MappingCommitmentDigest) ||
		readiness.RequiredPrimaryReviewers != 2 || readiness.RequiredTieBreakReviewers != 1 || readiness.RequiredPrimaryJudgments != expectedPrimaryJudgments ||
		readiness.MaximumTieBreakJudgments != expectedTieBreakJudgments || readiness.RequiredPostLabelProbes != expectedProbes || readiness.TechnicalStatus != RelationPilotTechnicalReady ||
		readiness.SemanticInspectionStatus != RelationPilotSemanticPending || readiness.ExternalActionStatus != ExternalActionNotAuthorized ||
		readiness.RequiredExternalAction != RelationPilotRequiredExternalAction || !slices.Equal(readiness.Limitations, relationPilotReadinessLimitations()) {
		return errors.New("relation pilot readiness identity, governance, workload, inspection, or authorization boundary is invalid")
	}
	if readiness.ProtocolVersion == ProtocolVersionV1 {
		if readiness.SourceCorpusDigest != "" || readiness.SourceCorpusSpecDigest != "" || readiness.SourceCorpusPlanDigest != "" || readiness.SourceMutationProgramDigest != "" ||
			readiness.SourceConstructAuditDigest != "" || readiness.ConstructFirewallCommitmentDigest != "" {
			return errors.New("v1 relation pilot readiness contains v2-only source or construct bindings")
		}
	} else if readiness.ProtocolVersion == ProtocolVersionV2 && (!validDigest(readiness.SourceCorpusDigest) || !validDigest(readiness.SourceCorpusSpecDigest) || readiness.SourceCorpusPlanDigest != "" ||
		!validDigest(readiness.SourceMutationProgramDigest) || !validDigest(readiness.SourceConstructAuditDigest) ||
		!validDigest(readiness.ConstructFirewallCommitmentDigest)) {
		return errors.New("v2 relation pilot readiness lacks its corpus, program, audit, or construct-firewall commitment")
	} else if readiness.ProtocolVersion == ProtocolVersionV3 && (!validDigest(readiness.SourceCorpusDigest) || readiness.SourceCorpusSpecDigest != "" || !validDigest(readiness.SourceCorpusPlanDigest) ||
		!validDigest(readiness.SourceMutationProgramDigest) || !validDigest(readiness.SourceConstructAuditDigest) || !validDigest(readiness.ConstructFirewallCommitmentDigest)) {
		return errors.New("v3 relation pilot readiness lacks its corpus plan, program, audit, or typed construct-firewall commitment")
	}
	if _, err := time.Parse(time.RFC3339, readiness.PreparedAt); err != nil {
		return errors.New("relation pilot readiness time must be RFC3339")
	}
	if err := validateRelationMappingReferences(readiness.MappingReferences); err != nil {
		return err
	}
	mappingCommitment, err := digestJSON(readiness.MappingReferences)
	if err != nil || readiness.MappingCommitmentDigest != mappingCommitment {
		return errors.New("relation pilot readiness mapping commitment is invalid")
	}
	seenCases, seenFamilies, seenGroups := map[string]struct{}{}, map[mutation.Family]struct{}{}, map[string]struct{}{}
	for index, check := range readiness.PacketChecks {
		definition, exists := mutation.DefinitionFor(check.Family)
		expectedUnit := UnitTrajectoryPair
		expectedSlots, expectedPairStatus := 2, "not_applicable"
		if exists && definition.PairLevel {
			expectedUnit, expectedSlots, expectedPairStatus = UnitCandidatePairOrders, 4, "exact_reversal_verified"
		}
		if !validOpaqueID(check.PacketID, "relation-packet-") || !validDigest(check.PacketDigest) || !validDigest(check.MappingDigest) || strings.TrimSpace(check.CaseID) == "" ||
			!exists || check.Unit != expectedUnit || strings.TrimSpace(check.TaskGroupID) == "" || !validDigest(check.TaskRequirementDigest) || check.EvidenceSlots != expectedSlots ||
			check.PairOrderStatus != expectedPairStatus || check.TaskContextStatus != "present_digest_bound" || check.RedistributionStatus != "restricted_reference_only" ||
			check.LeakageScanStatus != "passed" || check.StructuralStatus != "ready_for_owner_semantic_review" || index > 0 && readiness.PacketChecks[index-1].PacketID >= check.PacketID {
			return fmt.Errorf("relation pilot readiness packet check %d is invalid", index)
		}
		if _, duplicate := seenCases[check.CaseID]; duplicate {
			return errors.New("relation pilot readiness packet checks reuse a case")
		}
		if _, duplicate := seenFamilies[check.Family]; duplicate {
			return errors.New("relation pilot readiness packet checks reuse a family")
		}
		if _, duplicate := seenGroups[check.TaskGroupID]; duplicate {
			return errors.New("relation pilot readiness packet checks reuse a task group")
		}
		seenCases[check.CaseID], seenFamilies[check.Family], seenGroups[check.TaskGroupID] = struct{}{}, struct{}{}, struct{}{}
	}
	expected, err := relationPilotReadinessDigest(readiness)
	if err != nil || readiness.Digest != expected {
		return errors.New("relation pilot readiness digest is invalid")
	}
	return nil
}

func validateRelationMappingReferences(references []MappingReference) error {
	if len(references) == 0 {
		return errors.New("relation mapping references are empty")
	}
	for index, reference := range references {
		if !validOpaqueID(reference.PacketID, "relation-packet-") || !validDigest(reference.MappingDigest) || index > 0 && references[index-1].PacketID >= reference.PacketID {
			return errors.New("relation mapping references must be valid, unique, and sorted")
		}
	}
	return nil
}

func relationPilotReadinessLimitations() []string {
	return []string{
		"No reviewer has been contacted and no packet or kit has been distributed.",
		"Structural checks do not establish semantic sufficiency, rubric clarity, reviewer comprehension, or construct validity.",
		"The owner must manually inspect all eight restricted packet and rendered-kit surfaces before any external authorization.",
	}
}

func relationPilotReadinessDigest(value RelationPilotReadiness) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func relationConstructFirewallCommitment(bundle ReviewBundle, mappings map[string]PrivateMapping) (string, error) {
	type binding struct {
		PacketID                string `json:"packet_id"`
		ConstructFirewallDigest string `json:"construct_firewall_digest"`
	}
	bindings := make([]binding, len(bundle.Packets))
	for index, packet := range bundle.Packets {
		mapping, exists := mappings[packet.PacketID]
		if !exists || !validDigest(mapping.ConstructFirewallDigest) {
			return "", errors.New("v2 relation pilot readiness requires every packet construct-firewall binding")
		}
		bindings[index] = binding{PacketID: packet.PacketID, ConstructFirewallDigest: mapping.ConstructFirewallDigest}
	}
	sort.Slice(bindings, func(left, right int) bool { return bindings[left].PacketID < bindings[right].PacketID })
	return digestJSON(bindings)
}
