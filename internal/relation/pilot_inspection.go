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
	PilotInspectionSchemaVersionV1 = "evalwitness.relation-pilot-inspection.v1"
	PilotInspectionSchemaVersionV2 = "evalwitness.relation-pilot-inspection.v2"
	PilotInspectionSchemaVersionV3 = "evalwitness.relation-pilot-inspection.v3"
	PilotInspectionProtocolV1      = "evalwitness.relation-owner-semantic-inspection.v1"
	PilotInspectionProtocolV2      = "evalwitness.relation-owner-semantic-inspection.v2"
	PilotInspectionProtocolV3      = "evalwitness.relation-owner-semantic-inspection.v3"
	PilotInspectionSchemaVersion   = PilotInspectionSchemaVersionV1
	PilotInspectionProtocol        = PilotInspectionProtocolV1
	PilotInspectionEvidenceScope   = "owner_prelaunch_semantic_inspection_only"
	PilotInspectionHumanNotRun     = "not_run"
)

type PilotInspectionAssessment string

const (
	PilotInspectionPassed        PilotInspectionAssessment = "passed"
	PilotInspectionFailed        PilotInspectionAssessment = "failed"
	PilotInspectionIndeterminate PilotInspectionAssessment = "indeterminate"
	PilotInspectionNotApplicable PilotInspectionAssessment = "not_applicable"
)

type PilotInspectionDisposition string

const (
	PilotInspectionAccepted         PilotInspectionDisposition = "accepted"
	PilotInspectionRevisionRequired PilotInspectionDisposition = "revision_required"
	PilotInspectionUnresolved       PilotInspectionDisposition = "unresolved"
)

type PilotInspectionOverallStatus string

const (
	PilotInspectionOverallPassed           PilotInspectionOverallStatus = "passed"
	PilotInspectionOverallRevisionRequired PilotInspectionOverallStatus = "revision_required"
	PilotInspectionOverallUnresolved       PilotInspectionOverallStatus = "unresolved"
)

type PilotInspectionReason string

const (
	PilotInspectionReasonAlignment      PilotInspectionReason = "evidence_alignment_gap"
	PilotInspectionReasonBlinding       PilotInspectionReason = "blinding_or_identity_leakage"
	PilotInspectionReasonCandidateOrder PilotInspectionReason = "candidate_order_gap"
	PilotInspectionReasonInformation    PilotInspectionReason = "insufficient_information"
	PilotInspectionReasonRedistribution PilotInspectionReason = "license_or_redistribution_gap"
	PilotInspectionReasonRubric         PilotInspectionReason = "rubric_applicability_gap"
	PilotInspectionReasonTaskContext    PilotInspectionReason = "task_context_gap"
	PilotInspectionReasonTransformation PilotInspectionReason = "multi_factor_or_unisolated_change"
)

type PilotInspectionDecisionDraft struct {
	PacketID                string                    `json:"packet_id"`
	TaskContext             PilotInspectionAssessment `json:"task_context"`
	EvidenceAlignment       PilotInspectionAssessment `json:"evidence_alignment"`
	TransformationIsolation PilotInspectionAssessment `json:"transformation_isolation"`
	InformationSufficiency  PilotInspectionAssessment `json:"information_sufficiency"`
	BlindingIntegrity       PilotInspectionAssessment `json:"blinding_integrity"`
	RubricApplicability     PilotInspectionAssessment `json:"rubric_applicability"`
	RedistributionBoundary  PilotInspectionAssessment `json:"redistribution_boundary"`
	CandidateOrder          PilotInspectionAssessment `json:"candidate_order"`
	ReasonCodes             []PilotInspectionReason   `json:"reason_codes"`
}

type PilotInspectionDecision struct {
	PacketID                string                     `json:"packet_id"`
	PacketDigest            string                     `json:"packet_digest"`
	MappingDigest           string                     `json:"mapping_digest"`
	CaseID                  string                     `json:"case_id"`
	Family                  mutation.Family            `json:"family"`
	Unit                    UnitType                   `json:"unit"`
	TaskContext             PilotInspectionAssessment  `json:"task_context"`
	EvidenceAlignment       PilotInspectionAssessment  `json:"evidence_alignment"`
	TransformationIsolation PilotInspectionAssessment  `json:"transformation_isolation"`
	InformationSufficiency  PilotInspectionAssessment  `json:"information_sufficiency"`
	BlindingIntegrity       PilotInspectionAssessment  `json:"blinding_integrity"`
	RubricApplicability     PilotInspectionAssessment  `json:"rubric_applicability"`
	RedistributionBoundary  PilotInspectionAssessment  `json:"redistribution_boundary"`
	CandidateOrder          PilotInspectionAssessment  `json:"candidate_order"`
	ReasonCodes             []PilotInspectionReason    `json:"reason_codes"`
	Disposition             PilotInspectionDisposition `json:"disposition"`
}

type PilotInspectionRecord struct {
	SchemaVersion           string                       `json:"schema_version"`
	CanonicalPolicy         string                       `json:"canonical_policy"`
	ProtocolVersion         string                       `json:"protocol_version"`
	Objective               ReviewObjective              `json:"review_objective"`
	InspectionProtocol      string                       `json:"inspection_protocol"`
	ReadinessDigest         string                       `json:"readiness_digest"`
	PlanDigest              string                       `json:"plan_digest"`
	PilotSampleDigest       string                       `json:"pilot_sample_digest"`
	BundleDigest            string                       `json:"bundle_digest"`
	MappingCommitmentDigest string                       `json:"mapping_commitment_digest"`
	HandbookDigest          string                       `json:"handbook_digest"`
	DataRole                ReviewDataRole               `json:"data_role"`
	Visibility              ReviewVisibility             `json:"visibility"`
	InspectorAlias          string                       `json:"inspector_alias"`
	Packets                 int                          `json:"packets"`
	Decisions               []PilotInspectionDecision    `json:"decisions"`
	Accepted                int                          `json:"accepted"`
	RevisionRequired        int                          `json:"revision_required"`
	Unresolved              int                          `json:"unresolved"`
	OverallStatus           PilotInspectionOverallStatus `json:"overall_status"`
	EvidenceScope           string                       `json:"evidence_scope"`
	HumanStudyStatus        string                       `json:"human_study_status"`
	InspectedAt             string                       `json:"inspected_at"`
	ExternalActionStatus    ExternalActionStatus         `json:"external_action_status"`
	Limitations             []string                     `json:"limitations"`
	Digest                  string                       `json:"digest"`
}

func BuildPilotInspectionRecord(readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping, drafts []PilotInspectionDecisionDraft, inspectorAlias, inspectedAt string) (PilotInspectionRecord, error) {
	if err := VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return PilotInspectionRecord{}, err
	}
	if strings.TrimSpace(inspectorAlias) == "" {
		return PilotInspectionRecord{}, errors.New("relation pilot inspection requires a pseudonymous owner inspector alias")
	}
	if err := requireRelationTimeAfter("relation pilot inspection", inspectedAt, readiness.PreparedAt); err != nil {
		return PilotInspectionRecord{}, err
	}
	if len(drafts) != readiness.Packets {
		return PilotInspectionRecord{}, errors.New("relation pilot inspection requires one decision per readiness packet")
	}
	checkByPacket := make(map[string]RelationPacketReadiness, len(readiness.PacketChecks))
	for _, check := range readiness.PacketChecks {
		checkByPacket[check.PacketID] = check
	}
	decisions := make([]PilotInspectionDecision, len(drafts))
	seen := make(map[string]struct{}, len(drafts))
	for index, draft := range drafts {
		check, exists := checkByPacket[draft.PacketID]
		if !exists {
			return PilotInspectionRecord{}, errors.New("relation pilot inspection draft references a packet outside readiness")
		}
		if _, duplicate := seen[draft.PacketID]; duplicate {
			return PilotInspectionRecord{}, errors.New("relation pilot inspection contains a duplicate packet decision")
		}
		seen[draft.PacketID] = struct{}{}
		decision, err := buildPilotInspectionDecision(check, draft)
		if err != nil {
			return PilotInspectionRecord{}, fmt.Errorf("relation pilot inspection packet %s: %w", draft.PacketID, err)
		}
		decisions[index] = decision
	}
	sort.Slice(decisions, func(left, right int) bool { return decisions[left].PacketID < decisions[right].PacketID })
	accepted, revisionRequired, unresolved := pilotInspectionCounts(decisions)
	record := PilotInspectionRecord{
		ProtocolVersion: readiness.ProtocolVersion, Objective: ReviewObjectiveControlledRelation,
		InspectionProtocol: pilotInspectionProtocolForVersion(readiness.ProtocolVersion),
		ReadinessDigest:    readiness.Digest, PlanDigest: readiness.PlanDigest, PilotSampleDigest: readiness.PilotSampleDigest,
		BundleDigest: readiness.BundleDigest, MappingCommitmentDigest: readiness.MappingCommitmentDigest, HandbookDigest: readiness.HandbookDigest,
		DataRole: readiness.DataRole, Visibility: readiness.Visibility, InspectorAlias: strings.TrimSpace(inspectorAlias), Packets: readiness.Packets,
		Decisions: decisions, Accepted: accepted, RevisionRequired: revisionRequired, Unresolved: unresolved,
		OverallStatus: pilotInspectionOverallStatus(revisionRequired, unresolved), EvidenceScope: PilotInspectionEvidenceScope,
		HumanStudyStatus: PilotInspectionHumanNotRun, InspectedAt: inspectedAt, ExternalActionStatus: ExternalActionNotAuthorized,
		Limitations: pilotInspectionLimitations(),
	}
	return SealPilotInspectionRecord(record)
}

func buildPilotInspectionDecision(check RelationPacketReadiness, draft PilotInspectionDecisionDraft) (PilotInspectionDecision, error) {
	assessments := []PilotInspectionAssessment{
		draft.TaskContext, draft.EvidenceAlignment, draft.TransformationIsolation, draft.InformationSufficiency,
		draft.BlindingIntegrity, draft.RubricApplicability, draft.RedistributionBoundary,
	}
	for _, assessment := range assessments {
		if !slices.Contains([]PilotInspectionAssessment{PilotInspectionPassed, PilotInspectionFailed, PilotInspectionIndeterminate}, assessment) {
			return PilotInspectionDecision{}, errors.New("inspection dimensions require passed, failed, or indeterminate")
		}
	}
	expectedCandidateOrder := PilotInspectionNotApplicable
	if check.Unit == UnitCandidatePairOrders {
		expectedCandidateOrder = PilotInspectionPassed
	}
	if check.Unit == UnitCandidatePairOrders {
		if !slices.Contains([]PilotInspectionAssessment{PilotInspectionPassed, PilotInspectionFailed, PilotInspectionIndeterminate}, draft.CandidateOrder) {
			return PilotInspectionDecision{}, errors.New("candidate-order inspection requires passed, failed, or indeterminate")
		}
	} else if draft.CandidateOrder != expectedCandidateOrder {
		return PilotInspectionDecision{}, errors.New("trajectory-pair inspection requires candidate_order=not_applicable")
	}
	decision := PilotInspectionDecision{
		PacketID: check.PacketID, PacketDigest: check.PacketDigest, MappingDigest: check.MappingDigest, CaseID: check.CaseID,
		Family: check.Family, Unit: check.Unit, TaskContext: draft.TaskContext, EvidenceAlignment: draft.EvidenceAlignment,
		TransformationIsolation: draft.TransformationIsolation, InformationSufficiency: draft.InformationSufficiency,
		BlindingIntegrity: draft.BlindingIntegrity, RubricApplicability: draft.RubricApplicability,
		RedistributionBoundary: draft.RedistributionBoundary, CandidateOrder: draft.CandidateOrder,
		ReasonCodes: append([]PilotInspectionReason(nil), draft.ReasonCodes...),
	}
	decision.Disposition = pilotInspectionDisposition(decision)
	expectedReasons := pilotInspectionReasons(decision)
	if !slices.Equal(decision.ReasonCodes, expectedReasons) {
		return PilotInspectionDecision{}, errors.New("inspection reason codes must exactly reproduce non-passing dimensions in canonical order")
	}
	return decision, nil
}

func validatePilotInspectionDecision(decision PilotInspectionDecision) error {
	definition, exists := mutation.DefinitionFor(decision.Family)
	if !exists {
		return errors.New("inspection decision uses an unknown relation family")
	}
	expectedUnit := UnitTrajectoryPair
	if definition.PairLevel {
		expectedUnit = UnitCandidatePairOrders
	}
	if decision.Unit != expectedUnit {
		return errors.New("inspection decision family and unit disagree")
	}
	assessments := []PilotInspectionAssessment{
		decision.TaskContext, decision.EvidenceAlignment, decision.TransformationIsolation, decision.InformationSufficiency,
		decision.BlindingIntegrity, decision.RubricApplicability, decision.RedistributionBoundary,
	}
	for _, assessment := range assessments {
		if !slices.Contains([]PilotInspectionAssessment{PilotInspectionPassed, PilotInspectionFailed, PilotInspectionIndeterminate}, assessment) {
			return errors.New("inspection decision contains an invalid assessment")
		}
	}
	if decision.Unit == UnitCandidatePairOrders {
		if !slices.Contains([]PilotInspectionAssessment{PilotInspectionPassed, PilotInspectionFailed, PilotInspectionIndeterminate}, decision.CandidateOrder) {
			return errors.New("candidate-order inspection contains an invalid assessment")
		}
	} else if decision.CandidateOrder != PilotInspectionNotApplicable {
		return errors.New("trajectory-pair inspection must mark candidate order not applicable")
	}
	if decision.Disposition != pilotInspectionDisposition(decision) || !slices.Equal(decision.ReasonCodes, pilotInspectionReasons(decision)) {
		return errors.New("inspection decision disposition or reasons do not reproduce its assessments")
	}
	return nil
}

func pilotInspectionDisposition(decision PilotInspectionDecision) PilotInspectionDisposition {
	assessments := []PilotInspectionAssessment{
		decision.TaskContext, decision.EvidenceAlignment, decision.TransformationIsolation, decision.InformationSufficiency,
		decision.BlindingIntegrity, decision.RubricApplicability, decision.RedistributionBoundary, decision.CandidateOrder,
	}
	if slices.Contains(assessments, PilotInspectionFailed) {
		return PilotInspectionRevisionRequired
	}
	if slices.Contains(assessments, PilotInspectionIndeterminate) {
		return PilotInspectionUnresolved
	}
	return PilotInspectionAccepted
}

func pilotInspectionReasons(decision PilotInspectionDecision) []PilotInspectionReason {
	var reasons []PilotInspectionReason
	appendWhenNotPassed := func(assessment PilotInspectionAssessment, reason PilotInspectionReason) {
		if assessment != PilotInspectionPassed && assessment != PilotInspectionNotApplicable {
			reasons = append(reasons, reason)
		}
	}
	appendWhenNotPassed(decision.EvidenceAlignment, PilotInspectionReasonAlignment)
	appendWhenNotPassed(decision.BlindingIntegrity, PilotInspectionReasonBlinding)
	appendWhenNotPassed(decision.CandidateOrder, PilotInspectionReasonCandidateOrder)
	appendWhenNotPassed(decision.InformationSufficiency, PilotInspectionReasonInformation)
	appendWhenNotPassed(decision.RedistributionBoundary, PilotInspectionReasonRedistribution)
	appendWhenNotPassed(decision.RubricApplicability, PilotInspectionReasonRubric)
	appendWhenNotPassed(decision.TaskContext, PilotInspectionReasonTaskContext)
	appendWhenNotPassed(decision.TransformationIsolation, PilotInspectionReasonTransformation)
	sort.Slice(reasons, func(left, right int) bool { return reasons[left] < reasons[right] })
	return reasons
}

func pilotInspectionCounts(decisions []PilotInspectionDecision) (int, int, int) {
	accepted, revisionRequired, unresolved := 0, 0, 0
	for _, decision := range decisions {
		switch decision.Disposition {
		case PilotInspectionAccepted:
			accepted++
		case PilotInspectionRevisionRequired:
			revisionRequired++
		case PilotInspectionUnresolved:
			unresolved++
		}
	}
	return accepted, revisionRequired, unresolved
}

func pilotInspectionOverallStatus(revisionRequired, unresolved int) PilotInspectionOverallStatus {
	if revisionRequired > 0 {
		return PilotInspectionOverallRevisionRequired
	}
	if unresolved > 0 {
		return PilotInspectionOverallUnresolved
	}
	return PilotInspectionOverallPassed
}

func SealPilotInspectionRecord(record PilotInspectionRecord) (PilotInspectionRecord, error) {
	schemaVersion, err := schemaVersionForProtocol(record.ProtocolVersion, PilotInspectionSchemaVersionV1, PilotInspectionSchemaVersionV2, PilotInspectionSchemaVersionV3)
	if err != nil {
		return PilotInspectionRecord{}, err
	}
	record.SchemaVersion, record.CanonicalPolicy, record.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := pilotInspectionRecordDigest(record)
	if err != nil {
		return PilotInspectionRecord{}, err
	}
	record.Digest = digest
	return record, record.Validate()
}

func (record PilotInspectionRecord) Validate() error {
	expectedPackets := 8
	if record.ProtocolVersion == ProtocolVersionV3 {
		expectedPackets = 7
	}
	if !validVersionedIdentity(record.SchemaVersion, record.ProtocolVersion, PilotInspectionSchemaVersionV1, PilotInspectionSchemaVersionV2, PilotInspectionSchemaVersionV3) || record.CanonicalPolicy != CanonicalPolicy ||
		record.Objective != ReviewObjectiveControlledRelation || record.InspectionProtocol != pilotInspectionProtocolForVersion(record.ProtocolVersion) || !validDigest(record.ReadinessDigest) ||
		!validDigest(record.PlanDigest) || !validDigest(record.PilotSampleDigest) || !validDigest(record.BundleDigest) || !validDigest(record.MappingCommitmentDigest) ||
		!validDigest(record.HandbookDigest) || record.DataRole != ReviewDataDevelopmentPilot || record.Visibility != ReviewVisibilityRestricted ||
		strings.TrimSpace(record.InspectorAlias) == "" || record.Packets != expectedPackets || len(record.Decisions) != record.Packets ||
		record.EvidenceScope != PilotInspectionEvidenceScope || record.HumanStudyStatus != PilotInspectionHumanNotRun ||
		record.ExternalActionStatus != ExternalActionNotAuthorized || !slices.Equal(record.Limitations, pilotInspectionLimitations()) {
		return errors.New("relation pilot inspection identity, scope, custody, or claim boundary is invalid")
	}
	if _, err := time.Parse(time.RFC3339, record.InspectedAt); err != nil {
		return errors.New("relation pilot inspection time must be RFC3339")
	}
	seenCases, seenFamilies := make(map[string]struct{}, record.Packets), make(map[mutation.Family]struct{}, record.Packets)
	for index, decision := range record.Decisions {
		if !validOpaqueID(decision.PacketID, "relation-packet-") || !validDigest(decision.PacketDigest) || !validDigest(decision.MappingDigest) ||
			strings.TrimSpace(decision.CaseID) == "" || index > 0 && record.Decisions[index-1].PacketID >= decision.PacketID || validatePilotInspectionDecision(decision) != nil {
			return fmt.Errorf("relation pilot inspection decision %d is invalid", index)
		}
		if _, duplicate := seenCases[decision.CaseID]; duplicate {
			return errors.New("relation pilot inspection reuses a case")
		}
		if _, duplicate := seenFamilies[decision.Family]; duplicate {
			return errors.New("relation pilot inspection reuses a family")
		}
		seenCases[decision.CaseID], seenFamilies[decision.Family] = struct{}{}, struct{}{}
	}
	accepted, revisionRequired, unresolved := pilotInspectionCounts(record.Decisions)
	if accepted != record.Accepted || revisionRequired != record.RevisionRequired || unresolved != record.Unresolved ||
		accepted+revisionRequired+unresolved != record.Packets || record.OverallStatus != pilotInspectionOverallStatus(revisionRequired, unresolved) {
		return errors.New("relation pilot inspection counts or overall status are invalid")
	}
	expected, err := pilotInspectionRecordDigest(record)
	if err != nil || record.Digest != expected {
		return errors.New("relation pilot inspection digest is invalid")
	}
	return nil
}

func VerifyRelationPilotReadiness(readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) error {
	if err := readiness.Validate(); err != nil {
		return err
	}
	if err := VerifyReviewBundle(bundle, mappings); err != nil {
		return err
	}
	if readiness.ProtocolVersion != bundle.ProtocolVersion || readiness.PlanDigest != bundle.PlanDigest || readiness.PilotSampleDigest != bundle.SampleDigest || readiness.BundleDigest != bundle.Digest ||
		readiness.QualificationSetDigest != bundle.QualificationSetDigest || readiness.HandbookDigest != bundle.HandbookDigest ||
		readiness.DataRole != bundle.DataRole || readiness.Visibility != bundle.Visibility || readiness.Packets != len(bundle.Packets) {
		return errors.New("relation pilot readiness does not bind the supplied restricted bundle")
	}
	references, mappingByPacket, err := relationMappingReferences(bundle, mappings)
	if err != nil {
		return err
	}
	if !slices.Equal(readiness.MappingReferences, references) {
		return errors.New("relation pilot readiness does not bind the supplied private mappings")
	}
	if readiness.ProtocolVersion == ProtocolVersionV2 || readiness.ProtocolVersion == ProtocolVersionV3 {
		commitment, commitmentErr := relationConstructFirewallCommitment(bundle, mappingByPacket)
		if commitmentErr != nil {
			return commitmentErr
		}
		if readiness.ConstructFirewallCommitmentDigest != commitment {
			return errors.New("relation pilot readiness does not bind the supplied construct firewalls")
		}
	}
	checkByPacket := make(map[string]RelationPacketReadiness, len(readiness.PacketChecks))
	for _, check := range readiness.PacketChecks {
		checkByPacket[check.PacketID] = check
	}
	for _, packet := range bundle.Packets {
		expected, buildErr := buildRelationPacketReadiness(packet, mappingByPacket[packet.PacketID])
		if buildErr != nil {
			return buildErr
		}
		if checkByPacket[packet.PacketID] != expected {
			return errors.New("relation pilot readiness packet check does not reproduce the bundle and mapping")
		}
	}
	return nil
}

func VerifyPilotInspectionRecord(record PilotInspectionRecord, readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if err := VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return err
	}
	if record.ProtocolVersion != readiness.ProtocolVersion || record.ReadinessDigest != readiness.Digest || record.PlanDigest != readiness.PlanDigest || record.PilotSampleDigest != readiness.PilotSampleDigest ||
		record.BundleDigest != readiness.BundleDigest || record.MappingCommitmentDigest != readiness.MappingCommitmentDigest || record.HandbookDigest != readiness.HandbookDigest {
		return errors.New("relation pilot inspection does not bind the supplied readiness evidence")
	}
	checkByPacket := make(map[string]RelationPacketReadiness, len(readiness.PacketChecks))
	for _, check := range readiness.PacketChecks {
		checkByPacket[check.PacketID] = check
	}
	for _, decision := range record.Decisions {
		check, exists := checkByPacket[decision.PacketID]
		if !exists || decision.PacketDigest != check.PacketDigest || decision.MappingDigest != check.MappingDigest || decision.CaseID != check.CaseID ||
			decision.Family != check.Family || decision.Unit != check.Unit {
			return errors.New("relation pilot inspection decision does not bind an exact readiness packet")
		}
	}
	return nil
}

func RenderPilotInspectionMarkdown(readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping, handbook ReviewerHandbook) (string, error) {
	if err := VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return "", err
	}
	if err := handbook.Validate(); err != nil {
		return "", err
	}
	if handbook.Digest != readiness.HandbookDigest || handbook.Digest != bundle.HandbookDigest {
		return "", errors.New("relation pilot inspection workbook handbook does not bind readiness")
	}
	mappingByPacket := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByPacket[mapping.PacketID] = mapping
	}
	var builder strings.Builder
	fmt.Fprintln(&builder, "# EvalWitness Owner-Only Relation Pilot Inspection")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "- Readiness digest: `%s`\n", readiness.Digest)
	fmt.Fprintf(&builder, "- Pilot sample digest: `%s`\n", readiness.PilotSampleDigest)
	fmt.Fprintf(&builder, "- Bundle digest: `%s`\n", readiness.BundleDigest)
	fmt.Fprintf(&builder, "- Mapping commitment: `%s`\n", readiness.MappingCommitmentDigest)
	fmt.Fprintf(&builder, "- Technical status: `%s`\n", readiness.TechnicalStatus)
	fmt.Fprintf(&builder, "- Semantic status: `%s`\n", readiness.SemanticInspectionStatus)
	fmt.Fprintf(&builder, "- External action: `%s`\n\n", readiness.ExternalActionStatus)
	fmt.Fprintln(&builder, "> OWNER-ONLY RESTRICTED SURFACE. This workbook contains hidden relation identities and reference-only evidence. It is not a reviewer kit, human-study result, or authorization to contact or distribute.")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "Required next action: %s\n\n", readiness.RequiredExternalAction)
	fmt.Fprintln(&builder, "## Frozen rubric")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "| Axis | Question | Allowed ratings |")
	fmt.Fprintln(&builder, "|---|---|---|")
	for _, axis := range handbook.AxisDefinitions {
		ratings := make([]string, len(axis.AllowedRatings))
		for index, rating := range axis.AllowedRatings {
			ratings[index] = string(rating)
		}
		fmt.Fprintf(&builder, "| `%s` | %s | `%s` |\n", axis.ID, markdownTable(axis.Question), strings.Join(ratings, "`, `"))
	}
	for index, packet := range bundle.Packets {
		mapping := mappingByPacket[packet.PacketID]
		fmt.Fprintf(&builder, "\n## Packet %d of %d\n\n", index+1, len(bundle.Packets))
		fmt.Fprintf(&builder, "- Packet ID: `%s`\n", packet.PacketID)
		fmt.Fprintf(&builder, "- Packet digest: `%s`\n", packet.Digest)
		fmt.Fprintf(&builder, "- Mapping digest: `%s`\n", mapping.Digest)
		fmt.Fprintf(&builder, "- Hidden case: `%s`\n", markdownInline(mapping.CaseID))
		fmt.Fprintf(&builder, "- Hidden family: `%s`\n", mapping.Family)
		fmt.Fprintf(&builder, "- Hidden expected relation: `%s`\n", mapping.ExpectedRelation)
		fmt.Fprintf(&builder, "- Hidden task group: `%s`\n", markdownInline(mapping.SourceTaskGroupID))
		fmt.Fprintf(&builder, "- Unit: `%s`\n\n", packet.Unit)
		fmt.Fprintln(&builder, "### Hidden mapping")
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, "| Visible side | Side alias | Slot | Candidate | Logical side | Source candidate | Source ID | Lineage digest |")
		fmt.Fprintln(&builder, "|---|---|---|---|---|---:|---|---|")
		for _, evidence := range mapping.EvidenceMappings {
			candidate := evidence.CandidateLabel
			if candidate == "" {
				candidate = "n/a"
			}
			fmt.Fprintf(&builder, "| `%s` | `%s` | `%s` | `%s` | `%s` | %d | `%s` | `%s` |\n",
				evidence.VisiblePosition, markdownInline(evidence.SideAlias), markdownInline(evidence.SlotID), markdownInline(candidate),
				evidence.LogicalSide, evidence.SourceCandidateIndex, markdownInline(evidence.SourceID), evidence.RetainedLineageDigest)
		}
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, "### Reviewer-visible task requirement")
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, fencedUntrusted(packet.TaskRequirement))
		for _, side := range packet.Sides {
			fmt.Fprintf(&builder, "\n### Reviewer-visible side %s\n\n", side.Position)
			fmt.Fprintf(&builder, "Side alias: `%s`\n\n", markdownInline(side.SideAlias))
			for evidenceIndex, evidence := range side.Evidence {
				fmt.Fprintf(&builder, "#### Evidence %d\n\n", evidenceIndex+1)
				fmt.Fprintf(&builder, "Slot `%s`; retained %d/%d events; omitted %d; license `%s`; visibility `%s`.\n\n",
					markdownInline(evidence.SlotID), evidence.RetainedEvents, evidence.SourceEvents, evidence.OmittedEvents,
					markdownInline(evidence.LicenseSPDX), evidence.Visibility)
				fmt.Fprintln(&builder, fencedUntrusted(evidence.Content))
			}
		}
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, "### Owner inspection matrix")
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, "For each dimension, select exactly one status. Any failed or indeterminate dimension requires its canonical reason code in the sealed decision input.")
		fmt.Fprintln(&builder)
		for _, dimension := range []string{"task_context", "evidence_alignment", "transformation_isolation", "information_sufficiency", "blinding_integrity", "rubric_applicability", "redistribution_boundary"} {
			fmt.Fprintf(&builder, "- `%s`: [ ] passed  [ ] failed  [ ] indeterminate\n", dimension)
		}
		if packet.Unit == UnitCandidatePairOrders {
			fmt.Fprintln(&builder, "- `candidate_order`: [ ] passed  [ ] failed  [ ] indeterminate")
		} else {
			fmt.Fprintln(&builder, "- `candidate_order`: `not_applicable`")
		}
		fmt.Fprintln(&builder, "- Canonical reason codes: [ ] evidence_alignment_gap [ ] blinding_or_identity_leakage [ ] candidate_order_gap [ ] insufficient_information [ ] license_or_redistribution_gap [ ] rubric_applicability_gap [ ] task_context_gap [ ] multi_factor_or_unisolated_change")
	}
	fmt.Fprintln(&builder, "\n## Owner completion gate")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "- [ ] Every packet was inspected against its hidden mapping and reviewer-visible rendering.")
	fmt.Fprintln(&builder, "- [ ] Every dimension has exactly one assessment and exact canonical reasons.")
	fmt.Fprintln(&builder, "- [ ] Any revision-required or unresolved packet caused a new packet/rubric/translation version before reviewer qualification.")
	fmt.Fprintln(&builder, "- [ ] No reviewer was contacted and no artifact was distributed based on this workbook alone.")
	return builder.String(), nil
}

func pilotInspectionLimitations() []string {
	return []string{
		"Owner inspection is not independent reviewer evidence or a construct-validity result.",
		"Passing inspection does not authorize reviewer contact, packet distribution, publication, or a provider call.",
		"Restricted evidence and hidden mappings remain owner-only and non-public.",
	}
}

func pilotInspectionRecordDigest(record PilotInspectionRecord) (string, error) {
	record.Digest = ""
	return digestJSON(record)
}

func pilotInspectionProtocolForVersion(protocolVersion string) string {
	switch protocolVersion {
	case ProtocolVersionV2:
		return PilotInspectionProtocolV2
	case ProtocolVersionV3:
		return PilotInspectionProtocolV3
	}
	return PilotInspectionProtocolV1
}
