package relation

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	PilotChangeReceiptSchemaVersionV1 = "evalwitness.relation-pilot-change-receipt.v1"
	PilotChangeReceiptSchemaVersionV2 = "evalwitness.relation-pilot-change-receipt.v2"
	PilotChangeReceiptSchemaVersionV3 = "evalwitness.relation-pilot-change-receipt.v3"
	PilotChangeReceiptSchemaVersion   = PilotChangeReceiptSchemaVersionV1
	PilotChangeReceiptPolicy          = "evalwitness.relation-pilot-change-receipt.v1"
	PilotChangeReceiptCoverage        = "all_non_common_rendered_lines_digest_bound"
	PilotChangeReceiptContent         = "digests_only_no_raw_task_or_trajectory_content"
	PilotChangeReceiptDecision        = "not_recorded"
	PilotChangeReceiptHumanStudy      = "not_run"
)

type PilotChangeSideBinding struct {
	LogicalSide           LogicalSide     `json:"logical_side"`
	VisiblePosition       VisiblePosition `json:"visible_position"`
	ContentDigest         string          `json:"content_digest"`
	RetainedLineageDigest string          `json:"retained_lineage_digest"`
	SourceEvents          int             `json:"source_events"`
	RetainedEvents        int             `json:"retained_events"`
	OmittedEvents         int             `json:"omitted_events"`
}

type PilotTrajectoryChange struct {
	Original                            PilotChangeSideBinding `json:"original"`
	Transformed                         PilotChangeSideBinding `json:"transformed"`
	CommonPrefixLines                   int                    `json:"common_prefix_lines"`
	CommonSuffixLines                   int                    `json:"common_suffix_lines"`
	OriginalChangedLines                int                    `json:"original_changed_lines"`
	TransformedChangedLines             int                    `json:"transformed_changed_lines"`
	OriginalChangedLinesDigest          string                 `json:"original_changed_lines_digest"`
	TransformedChangedLinesDigest       string                 `json:"transformed_changed_lines_digest"`
	PairedLineageEqual                  bool                   `json:"paired_lineage_equal"`
	ManualCausalReferenceReviewRequired bool                   `json:"manual_causal_reference_review_required"`
	FullWorkbookRequiredForContext      bool                   `json:"full_workbook_required_for_omitted_context"`
}

type PilotCandidateReversalBinding struct {
	SourceCandidateIndex            int             `json:"source_candidate_index"`
	OriginalVisiblePosition         VisiblePosition `json:"original_visible_position"`
	OriginalVisibleEvidenceIndex    int             `json:"original_visible_evidence_index"`
	TransformedVisiblePosition      VisiblePosition `json:"transformed_visible_position"`
	TransformedVisibleEvidenceIndex int             `json:"transformed_visible_evidence_index"`
	ContentDigest                   string          `json:"content_digest"`
	ExactContentMatch               bool            `json:"exact_content_match"`
}

type PilotChangeCheck struct {
	PacketID              string                          `json:"packet_id"`
	PacketDigest          string                          `json:"packet_digest"`
	MappingDigest         string                          `json:"mapping_digest"`
	CaseID                string                          `json:"case_id"`
	Family                mutation.Family                 `json:"family"`
	ExpectedRelation      mutation.Relation               `json:"expected_relation"`
	Unit                  UnitType                        `json:"unit"`
	TaskRequirementDigest string                          `json:"task_requirement_digest"`
	TrajectoryChanges     []PilotTrajectoryChange         `json:"trajectory_changes"`
	CandidateReversal     []PilotCandidateReversalBinding `json:"candidate_reversal"`
}

type PilotChangeReceipt struct {
	SchemaVersion                     string               `json:"schema_version"`
	CanonicalPolicy                   string               `json:"canonical_policy"`
	ProtocolVersion                   string               `json:"protocol_version"`
	Objective                         ReviewObjective      `json:"review_objective"`
	ReceiptPolicy                     string               `json:"receipt_policy"`
	ReadinessDigest                   string               `json:"readiness_digest"`
	BundleDigest                      string               `json:"bundle_digest"`
	MappingCommitmentDigest           string               `json:"mapping_commitment_digest"`
	SourceCorpusDigest                string               `json:"source_corpus_digest,omitempty"`
	SourceCorpusSpecDigest            string               `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest            string               `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest       string               `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest        string               `json:"source_construct_audit_digest,omitempty"`
	ConstructFirewallCommitmentDigest string               `json:"construct_firewall_commitment_digest,omitempty"`
	DataRole                          ReviewDataRole       `json:"data_role"`
	Visibility                        ReviewVisibility     `json:"visibility"`
	Packets                           int                  `json:"packets"`
	Checks                            []PilotChangeCheck   `json:"checks"`
	TrajectoryPairs                   int                  `json:"trajectory_pairs"`
	CandidateOrderControls            int                  `json:"candidate_order_controls"`
	ManualCausalReferenceReviews      int                  `json:"manual_causal_reference_reviews"`
	CoverageStatus                    string               `json:"coverage_status"`
	ContentStatus                     string               `json:"content_status"`
	DecisionStatus                    string               `json:"decision_status"`
	HumanStudyStatus                  string               `json:"human_study_status"`
	ExternalActionStatus              ExternalActionStatus `json:"external_action_status"`
	Limitations                       []string             `json:"limitations"`
	Digest                            string               `json:"digest"`
}

func BuildPilotChangeReceipt(readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) (PilotChangeReceipt, error) {
	if err := VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return PilotChangeReceipt{}, err
	}
	mappingByPacket := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByPacket[mapping.PacketID] = mapping
	}
	checks := make([]PilotChangeCheck, len(bundle.Packets))
	trajectoryPairs, candidateControls, causalReviews := 0, 0, 0
	for index, packet := range bundle.Packets {
		mapping := mappingByPacket[packet.PacketID]
		check := PilotChangeCheck{
			PacketID: packet.PacketID, PacketDigest: packet.Digest, MappingDigest: mapping.Digest,
			CaseID: mapping.CaseID, Family: mapping.Family, ExpectedRelation: mapping.ExpectedRelation,
			Unit: mapping.Unit, TaskRequirementDigest: packet.TaskRequirementDigest,
			TrajectoryChanges: []PilotTrajectoryChange{}, CandidateReversal: []PilotCandidateReversalBinding{},
		}
		if packet.Unit == UnitCandidatePairOrders {
			bindings, err := buildPilotCandidateReversalBindings(packet, mapping)
			if err != nil {
				return PilotChangeReceipt{}, err
			}
			check.CandidateReversal = bindings
			candidateControls++
		} else {
			change, err := buildPilotTrajectoryChange(packet, mapping)
			if err != nil {
				return PilotChangeReceipt{}, err
			}
			check.TrajectoryChanges = []PilotTrajectoryChange{change}
			trajectoryPairs++
			if change.ManualCausalReferenceReviewRequired {
				causalReviews++
			}
		}
		checks[index] = check
	}
	sort.Slice(checks, func(left, right int) bool { return checks[left].PacketID < checks[right].PacketID })
	receipt := PilotChangeReceipt{
		ProtocolVersion: readiness.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, ReceiptPolicy: PilotChangeReceiptPolicy,
		ReadinessDigest: readiness.Digest, BundleDigest: bundle.Digest, MappingCommitmentDigest: readiness.MappingCommitmentDigest,
		DataRole: readiness.DataRole, Visibility: readiness.Visibility, Packets: len(checks), Checks: checks,
		TrajectoryPairs: trajectoryPairs, CandidateOrderControls: candidateControls, ManualCausalReferenceReviews: causalReviews,
		CoverageStatus: PilotChangeReceiptCoverage, ContentStatus: PilotChangeReceiptContent, DecisionStatus: PilotChangeReceiptDecision,
		HumanStudyStatus: PilotChangeReceiptHumanStudy, ExternalActionStatus: ExternalActionNotAuthorized,
		Limitations: pilotChangeReceiptLimitations(),
	}
	switch readiness.ProtocolVersion {
	case ProtocolVersionV2:
		receipt.SourceCorpusDigest = readiness.SourceCorpusDigest
		receipt.SourceCorpusSpecDigest = readiness.SourceCorpusSpecDigest
		receipt.SourceMutationProgramDigest = readiness.SourceMutationProgramDigest
		receipt.SourceConstructAuditDigest = readiness.SourceConstructAuditDigest
		receipt.ConstructFirewallCommitmentDigest = readiness.ConstructFirewallCommitmentDigest
	case ProtocolVersionV3:
		receipt.SourceCorpusDigest = readiness.SourceCorpusDigest
		receipt.SourceCorpusPlanDigest = readiness.SourceCorpusPlanDigest
		receipt.SourceMutationProgramDigest = readiness.SourceMutationProgramDigest
		receipt.SourceConstructAuditDigest = readiness.SourceConstructAuditDigest
		receipt.ConstructFirewallCommitmentDigest = readiness.ConstructFirewallCommitmentDigest
	}
	return SealPilotChangeReceipt(receipt)
}

func buildPilotTrajectoryChange(packet BlindPacket, mapping PrivateMapping) (PilotTrajectoryChange, error) {
	originalMapping, transformedMapping, err := trajectoryPairMappings(mapping)
	if err != nil {
		return PilotTrajectoryChange{}, err
	}
	original, err := packetEvidenceForMapping(packet, originalMapping)
	if err != nil {
		return PilotTrajectoryChange{}, err
	}
	transformed, err := packetEvidenceForMapping(packet, transformedMapping)
	if err != nil {
		return PilotTrajectoryChange{}, err
	}
	change := buildPilotLineChange(original.Content, transformed.Content, pilotChangeAtlasContextLines)
	if len(change.OriginalChanged) == 0 && len(change.TransformedChanged) == 0 {
		return PilotTrajectoryChange{}, errors.New("relation pilot change receipt found no rendered change in a trajectory pair")
	}
	originalChangedDigest, err := digestJSON(change.OriginalChanged)
	if err != nil {
		return PilotTrajectoryChange{}, err
	}
	transformedChangedDigest, err := digestJSON(change.TransformedChanged)
	if err != nil {
		return PilotTrajectoryChange{}, err
	}
	manualCausalReview := mapping.Family == mutation.FamilyCausalIndependentReorder && pilotLineChangeContains(change, "[Event reference:")
	return PilotTrajectoryChange{
		Original:          pilotChangeSideBinding(LogicalOriginal, originalMapping, original),
		Transformed:       pilotChangeSideBinding(LogicalTransformed, transformedMapping, transformed),
		CommonPrefixLines: change.CommonPrefixLines, CommonSuffixLines: change.CommonSuffixLines,
		OriginalChangedLines: len(change.OriginalChanged), TransformedChangedLines: len(change.TransformedChanged),
		OriginalChangedLinesDigest: originalChangedDigest, TransformedChangedLinesDigest: transformedChangedDigest,
		PairedLineageEqual:                  originalMapping.RetainedLineageDigest == transformedMapping.RetainedLineageDigest,
		ManualCausalReferenceReviewRequired: manualCausalReview,
		FullWorkbookRequiredForContext:      original.OmittedEvents > 0 || transformed.OmittedEvents > 0,
	}, nil
}

func pilotChangeSideBinding(logicalSide LogicalSide, mapping PrivateEvidenceMapping, evidence PacketEvidence) PilotChangeSideBinding {
	return PilotChangeSideBinding{
		LogicalSide: logicalSide, VisiblePosition: mapping.VisiblePosition, ContentDigest: evidence.ContentDigest,
		RetainedLineageDigest: mapping.RetainedLineageDigest, SourceEvents: evidence.SourceEvents,
		RetainedEvents: evidence.RetainedEvents, OmittedEvents: evidence.OmittedEvents,
	}
}

func buildPilotCandidateReversalBindings(packet BlindPacket, mapping PrivateMapping) ([]PilotCandidateReversalBinding, error) {
	originalMappings, transformedMappings, err := candidateOrderMappings(mapping)
	if err != nil {
		return nil, err
	}
	bindings := make([]PilotCandidateReversalBinding, len(originalMappings))
	for sourceIndex := 1; sourceIndex <= len(originalMappings); sourceIndex++ {
		originalMapping := originalMappings[sourceIndex-1]
		transformedMapping := transformedMappings[len(transformedMappings)-sourceIndex]
		if originalMapping.SourceCandidateIndex != sourceIndex || transformedMapping.SourceCandidateIndex != sourceIndex {
			return nil, errors.New("relation pilot change receipt candidate mappings do not form an exact reversal")
		}
		original, err := packetEvidenceForMapping(packet, originalMapping)
		if err != nil {
			return nil, err
		}
		transformed, err := packetEvidenceForMapping(packet, transformedMapping)
		if err != nil {
			return nil, err
		}
		exact := original.ContentDigest == transformed.ContentDigest && original.Content == transformed.Content
		if !exact {
			return nil, errors.New("relation pilot change receipt candidate reversal changed candidate content")
		}
		bindings[sourceIndex-1] = PilotCandidateReversalBinding{
			SourceCandidateIndex: sourceIndex, OriginalVisiblePosition: originalMapping.VisiblePosition,
			OriginalVisibleEvidenceIndex: originalMapping.VisibleEvidenceIndex, TransformedVisiblePosition: transformedMapping.VisiblePosition,
			TransformedVisibleEvidenceIndex: transformedMapping.VisibleEvidenceIndex, ContentDigest: original.ContentDigest, ExactContentMatch: true,
		}
	}
	return bindings, nil
}

func SealPilotChangeReceipt(receipt PilotChangeReceipt) (PilotChangeReceipt, error) {
	schemaVersion, err := schemaVersionForProtocol(receipt.ProtocolVersion, PilotChangeReceiptSchemaVersionV1, PilotChangeReceiptSchemaVersionV2, PilotChangeReceiptSchemaVersionV3)
	if err != nil {
		return PilotChangeReceipt{}, err
	}
	receipt.SchemaVersion, receipt.CanonicalPolicy, receipt.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := pilotChangeReceiptDigest(receipt)
	if err != nil {
		return PilotChangeReceipt{}, err
	}
	receipt.Digest = digest
	return receipt, receipt.Validate()
}

func (receipt PilotChangeReceipt) Validate() error {
	expectedPackets, expectedTrajectoryPairs := 8, 7
	if receipt.ProtocolVersion == ProtocolVersionV3 {
		expectedPackets, expectedTrajectoryPairs = 7, 6
	}
	if !validVersionedIdentity(receipt.SchemaVersion, receipt.ProtocolVersion, PilotChangeReceiptSchemaVersionV1, PilotChangeReceiptSchemaVersionV2, PilotChangeReceiptSchemaVersionV3) || receipt.CanonicalPolicy != CanonicalPolicy ||
		receipt.Objective != ReviewObjectiveControlledRelation || receipt.ReceiptPolicy != PilotChangeReceiptPolicy || !validDigest(receipt.ReadinessDigest) ||
		!validDigest(receipt.BundleDigest) || !validDigest(receipt.MappingCommitmentDigest) || receipt.DataRole != ReviewDataDevelopmentPilot ||
		receipt.Visibility != ReviewVisibilityRestricted || receipt.Packets != expectedPackets || len(receipt.Checks) != receipt.Packets ||
		receipt.TrajectoryPairs != expectedTrajectoryPairs || receipt.CandidateOrderControls != 1 || receipt.ManualCausalReferenceReviews < 0 ||
		receipt.ManualCausalReferenceReviews > receipt.TrajectoryPairs || receipt.CoverageStatus != PilotChangeReceiptCoverage ||
		receipt.ContentStatus != PilotChangeReceiptContent || receipt.DecisionStatus != PilotChangeReceiptDecision ||
		receipt.HumanStudyStatus != PilotChangeReceiptHumanStudy || receipt.ExternalActionStatus != ExternalActionNotAuthorized ||
		!slices.Equal(receipt.Limitations, pilotChangeReceiptLimitations()) {
		return errors.New("relation pilot change receipt identity, coverage, content, or claim boundary is invalid")
	}
	if receipt.ProtocolVersion == ProtocolVersionV1 {
		if receipt.SourceCorpusDigest != "" || receipt.SourceCorpusSpecDigest != "" || receipt.SourceCorpusPlanDigest != "" || receipt.SourceMutationProgramDigest != "" ||
			receipt.SourceConstructAuditDigest != "" || receipt.ConstructFirewallCommitmentDigest != "" {
			return errors.New("v1 relation pilot change receipt contains v2-only source or construct bindings")
		}
	} else if receipt.ProtocolVersion == ProtocolVersionV2 && (!validDigest(receipt.SourceCorpusDigest) || !validDigest(receipt.SourceCorpusSpecDigest) || receipt.SourceCorpusPlanDigest != "" ||
		!validDigest(receipt.SourceMutationProgramDigest) || !validDigest(receipt.SourceConstructAuditDigest) ||
		!validDigest(receipt.ConstructFirewallCommitmentDigest)) {
		return errors.New("v2 relation pilot change receipt lacks its corpus, program, audit, or construct-firewall commitment")
	} else if receipt.ProtocolVersion == ProtocolVersionV3 && (!validDigest(receipt.SourceCorpusDigest) || receipt.SourceCorpusSpecDigest != "" || !validDigest(receipt.SourceCorpusPlanDigest) ||
		!validDigest(receipt.SourceMutationProgramDigest) || !validDigest(receipt.SourceConstructAuditDigest) || !validDigest(receipt.ConstructFirewallCommitmentDigest)) {
		return errors.New("v3 relation pilot change receipt lacks its corpus plan, program, audit, or typed construct-firewall commitment")
	}
	seenCases, seenFamilies := make(map[string]struct{}, receipt.Packets), make(map[mutation.Family]struct{}, receipt.Packets)
	trajectoryPairs, candidateControls, causalReviews := 0, 0, 0
	for index, check := range receipt.Checks {
		if err := validatePilotChangeCheck(check); err != nil {
			return fmt.Errorf("relation pilot change receipt check %d: %w", index, err)
		}
		if index > 0 && receipt.Checks[index-1].PacketID >= check.PacketID {
			return errors.New("relation pilot change receipt checks must be sorted by packet ID")
		}
		if _, duplicate := seenCases[check.CaseID]; duplicate {
			return errors.New("relation pilot change receipt reuses a case")
		}
		if _, duplicate := seenFamilies[check.Family]; duplicate {
			return errors.New("relation pilot change receipt reuses a family")
		}
		seenCases[check.CaseID], seenFamilies[check.Family] = struct{}{}, struct{}{}
		if check.Unit == UnitCandidatePairOrders {
			candidateControls++
		} else {
			trajectoryPairs++
			if check.TrajectoryChanges[0].ManualCausalReferenceReviewRequired {
				causalReviews++
			}
		}
	}
	if trajectoryPairs != receipt.TrajectoryPairs || candidateControls != receipt.CandidateOrderControls || causalReviews != receipt.ManualCausalReferenceReviews {
		return errors.New("relation pilot change receipt aggregate counts do not reproduce its checks")
	}
	expected, err := pilotChangeReceiptDigest(receipt)
	if err != nil || receipt.Digest != expected {
		return errors.New("relation pilot change receipt digest is invalid")
	}
	return nil
}

func validatePilotChangeCheck(check PilotChangeCheck) error {
	definition, exists := mutation.DefinitionFor(check.Family)
	if !validOpaqueID(check.PacketID, "relation-packet-") || !validDigest(check.PacketDigest) || !validDigest(check.MappingDigest) ||
		!validOpaqueID(check.CaseID, "mutation-") || !exists || check.ExpectedRelation != definition.Relation || !validDigest(check.TaskRequirementDigest) {
		return errors.New("packet, mapping, case, family, relation, or task binding is invalid")
	}
	expectedUnit := UnitTrajectoryPair
	if definition.PairLevel {
		expectedUnit = UnitCandidatePairOrders
	}
	if check.Unit != expectedUnit {
		return errors.New("family and unit disagree")
	}
	if check.Unit == UnitCandidatePairOrders {
		if len(check.TrajectoryChanges) != 0 || len(check.CandidateReversal) != 2 {
			return errors.New("candidate-order check has an invalid evidence shape")
		}
		for index, binding := range check.CandidateReversal {
			if binding.SourceCandidateIndex != index+1 || binding.OriginalVisibleEvidenceIndex != index+1 ||
				binding.TransformedVisibleEvidenceIndex != len(check.CandidateReversal)-index ||
				!validVisiblePosition(binding.OriginalVisiblePosition) || !validVisiblePosition(binding.TransformedVisiblePosition) ||
				binding.OriginalVisiblePosition == binding.TransformedVisiblePosition || !validDigest(binding.ContentDigest) || !binding.ExactContentMatch {
				return errors.New("candidate-order binding is not an exact content-preserving reversal")
			}
		}
		return nil
	}
	if len(check.TrajectoryChanges) != 1 || len(check.CandidateReversal) != 0 {
		return errors.New("trajectory-pair check has an invalid evidence shape")
	}
	change := check.TrajectoryChanges[0]
	if err := validatePilotChangeSideBinding(change.Original, LogicalOriginal); err != nil {
		return err
	}
	if err := validatePilotChangeSideBinding(change.Transformed, LogicalTransformed); err != nil {
		return err
	}
	if change.Original.VisiblePosition == change.Transformed.VisiblePosition || change.Original.ContentDigest == change.Transformed.ContentDigest ||
		change.CommonPrefixLines < 0 || change.CommonSuffixLines < 0 || change.OriginalChangedLines < 0 || change.TransformedChangedLines < 0 ||
		change.OriginalChangedLines+change.TransformedChangedLines == 0 || !validDigest(change.OriginalChangedLinesDigest) ||
		!validDigest(change.TransformedChangedLinesDigest) || !change.PairedLineageEqual ||
		change.FullWorkbookRequiredForContext != (change.Original.OmittedEvents > 0 || change.Transformed.OmittedEvents > 0) ||
		change.ManualCausalReferenceReviewRequired && check.Family != mutation.FamilyCausalIndependentReorder {
		return errors.New("trajectory-pair change coverage or structural flags are invalid")
	}
	return nil
}

func validatePilotChangeSideBinding(binding PilotChangeSideBinding, logicalSide LogicalSide) error {
	if binding.LogicalSide != logicalSide || !validVisiblePosition(binding.VisiblePosition) || !validDigest(binding.ContentDigest) ||
		!validDigest(binding.RetainedLineageDigest) || binding.SourceEvents <= 0 || binding.RetainedEvents <= 0 ||
		binding.RetainedEvents > binding.SourceEvents || binding.OmittedEvents != binding.SourceEvents-binding.RetainedEvents {
		return errors.New("trajectory side binding or event denominator is invalid")
	}
	return nil
}

func validVisiblePosition(position VisiblePosition) bool {
	return position == PositionLeft || position == PositionRight
}

func VerifyPilotChangeReceipt(receipt PilotChangeReceipt, readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) error {
	if err := receipt.Validate(); err != nil {
		return err
	}
	expected, err := BuildPilotChangeReceipt(readiness, bundle, mappings)
	if err != nil {
		return err
	}
	if receipt.Digest != expected.Digest {
		return errors.New("relation pilot change receipt does not reproduce the supplied readiness, bundle, and mappings")
	}
	return nil
}

func pilotChangeReceiptLimitations() []string {
	return []string{
		"The receipt contains hidden relation identities and is restricted to owner custody.",
		"Structural delta bindings and hazard flags do not establish semantic validity or a human decision.",
		"The complete restricted workbook remains required before any inspection decision.",
	}
}

func pilotChangeReceiptDigest(receipt PilotChangeReceipt) (string, error) {
	receipt.Digest = ""
	return digestJSON(receipt)
}
