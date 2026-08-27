package outcome

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const OutcomePilotInspectionSchemaVersion = "evalwitness.outcome-pilot-inspection.v1"

type OutcomePilotReviewabilityStatus string

const OutcomePilotStructurallyReady OutcomePilotReviewabilityStatus = "structurally_ready"

type OutcomePilotSemanticStatus string

const OutcomePilotRequiresHumanPilot OutcomePilotSemanticStatus = "requires_human_pilot"

type OutcomePilotBindingReference struct {
	PacketID            string `json:"packet_id"`
	SourceBindingDigest string `json:"source_binding_digest"`
}

type OutcomePilotPacketInspection struct {
	PacketID                 string                          `json:"packet_id"`
	TaskGroupID              string                          `json:"task_group_id"`
	SourceBindingDigest      string                          `json:"source_binding_digest"`
	DecisionAnchorKind       string                          `json:"decision_anchor_kind"`
	DecisionAnchorDigest     string                          `json:"decision_anchor_digest"`
	SourceEvents             int                             `json:"source_events"`
	RetainedEvents           int                             `json:"retained_events"`
	OmittedEvents            int                             `json:"omitted_events"`
	RetentionRatio           float64                         `json:"retention_ratio"`
	RetainedMessages         int                             `json:"retained_messages"`
	RetainedActions          int                             `json:"retained_actions"`
	RetainedResults          int                             `json:"retained_results"`
	EvidenceBudgetTokens     int                             `json:"evidence_budget_tokens"`
	RenderedEvidenceBytes    int                             `json:"rendered_evidence_bytes"`
	EstimatedEvidenceTokens  int                             `json:"estimated_evidence_tokens"`
	TrajectoryEvidenceDigest string                          `json:"trajectory_evidence_digest"`
	ReviewabilityStatus      OutcomePilotReviewabilityStatus `json:"reviewability_status"`
}

type OutcomePilotInspection struct {
	SchemaVersion                 string                          `json:"schema_version"`
	CanonicalPolicy               string                          `json:"canonical_policy"`
	PilotSampleDigest             string                          `json:"pilot_sample_digest"`
	BundleDigest                  string                          `json:"bundle_digest"`
	PrivateMaterialsDigest        string                          `json:"private_materials_digest"`
	Objective                     ReviewObjective                 `json:"objective"`
	Packets                       int                             `json:"packets"`
	BindingReferences             []OutcomePilotBindingReference  `json:"binding_references"`
	SourceBindingCommitmentDigest string                          `json:"source_binding_commitment_digest"`
	MappingCommitmentDigest       string                          `json:"mapping_commitment_digest"`
	Items                         []OutcomePilotPacketInspection  `json:"items"`
	TotalSourceEvents             int                             `json:"total_source_events"`
	TotalRetainedEvents           int                             `json:"total_retained_events"`
	TotalOmittedEvents            int                             `json:"total_omitted_events"`
	MinimumRetentionRatio         float64                         `json:"minimum_retention_ratio"`
	PacketsWithMessages           int                             `json:"packets_with_messages"`
	PacketsWithoutMessages        int                             `json:"packets_without_messages"`
	PatchAnchors                  int                             `json:"patch_anchors"`
	NarrativeAnchors              int                             `json:"narrative_anchors"`
	ReviewabilityStatus           OutcomePilotReviewabilityStatus `json:"reviewability_status"`
	SemanticStatus                OutcomePilotSemanticStatus      `json:"semantic_status"`
	Limitations                   []string                        `json:"limitations"`
	Digest                        string                          `json:"digest"`
}

func BuildOutcomePilotInspection(pilot OutcomePilotSampleCommitment, bundle ReviewBundle, privateMaterials OutcomePilotPrivateMaterials) (OutcomePilotInspection, error) {
	if err := pilot.Validate(); err != nil {
		return OutcomePilotInspection{}, err
	}
	if err := bundle.Validate(); err != nil {
		return OutcomePilotInspection{}, err
	}
	if err := privateMaterials.Validate(); err != nil {
		return OutcomePilotInspection{}, err
	}
	mappings, bindings := privateMaterials.Mappings, privateMaterials.SourceBindings
	if bundle.PlanDigest != pilot.PlanDigest || bundle.DataRole != pilot.DataRole || bundle.Visibility != ReviewVisibilityRestricted || len(bundle.Items) != pilot.SelectedCases {
		return OutcomePilotInspection{}, errors.New("outcome pilot inspection inputs do not share the frozen restricted pilot")
	}
	mappingReferences, err := mappingReferences(bundle, mappings)
	if err != nil {
		return OutcomePilotInspection{}, errors.New("outcome pilot inspection mappings do not reproduce the review bundle")
	}
	if err := validateOutcomePilotBundleCoverage(pilot, bundle, mappings); err != nil {
		return OutcomePilotInspection{}, err
	}
	if len(bindings) != len(bundle.Items) {
		return OutcomePilotInspection{}, errors.New("outcome pilot inspection requires one source binding per packet")
	}
	pilotBySource := make(map[string]OutcomePilotCaseReference, len(pilot.Cases))
	for _, pilotCase := range pilot.Cases {
		pilotBySource[pilotCase.SourceCaseDigest] = pilotCase
	}
	mappingByPacket := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByPacket[mapping.PacketID] = mapping
	}
	itemByPacket := make(map[string]ReviewItem, len(bundle.Items))
	for _, item := range bundle.Items {
		itemByPacket[item.Packet.PacketID] = item
	}
	inspection := OutcomePilotInspection{
		PilotSampleDigest: pilot.Digest, BundleDigest: bundle.Digest, PrivateMaterialsDigest: privateMaterials.Digest, Objective: ReviewObjectiveOutcome,
		Packets: len(bundle.Items), ReviewabilityStatus: OutcomePilotStructurallyReady,
		SemanticStatus: OutcomePilotRequiresHumanPilot, Limitations: outcomePilotInspectionLimitations(),
	}
	inspection.MinimumRetentionRatio = 1
	for _, binding := range bindings {
		if err := binding.Validate(); err != nil {
			return OutcomePilotInspection{}, err
		}
		mapping, mappingExists := mappingByPacket[binding.PacketID]
		item, itemExists := itemByPacket[binding.PacketID]
		pilotCase, caseExists := pilotBySource[mapping.SourceCaseDigest]
		if !mappingExists || !itemExists || !caseExists || binding.PilotSampleDigest != pilot.Digest ||
			binding.NaturalInventoryDigest != pilot.NaturalInventoryDigest || binding.MappingDigest != mapping.Digest ||
			binding.Stratum != pilotCase.Stratum || item.TaskGroupID != pilotCase.TaskGroupID {
			return OutcomePilotInspection{}, errors.New("outcome pilot source binding does not reproduce its packet, mapping, stratum, inventory, and task group")
		}
		trajectoryEvidence, err := outcomePilotTrajectoryEvidence(item.Packet)
		if err != nil {
			return OutcomePilotInspection{}, err
		}
		entry := OutcomePilotPacketInspection{
			PacketID: binding.PacketID, TaskGroupID: item.TaskGroupID, SourceBindingDigest: binding.Digest,
			DecisionAnchorKind: binding.DecisionAnchorKind, DecisionAnchorDigest: binding.DecisionAnchorDigest,
			SourceEvents: binding.SourceEvents, RetainedEvents: binding.RetainedEvents, OmittedEvents: binding.SourceEvents - binding.RetainedEvents,
			RetentionRatio: float64(binding.RetainedEvents) / float64(binding.SourceEvents), RetainedMessages: binding.RetainedMessages,
			RetainedActions: binding.RetainedActions, RetainedResults: binding.RetainedResults, EvidenceBudgetTokens: binding.EvidenceBudgetTokens,
			RenderedEvidenceBytes: len(trajectoryEvidence.Content), EstimatedEvidenceTokens: preprocess.EstimateTokens(trajectoryEvidence.Content),
			TrajectoryEvidenceDigest: trajectoryEvidence.ContentDigest, ReviewabilityStatus: OutcomePilotStructurallyReady,
		}
		inspection.Items = append(inspection.Items, entry)
		inspection.BindingReferences = append(inspection.BindingReferences, OutcomePilotBindingReference{PacketID: binding.PacketID, SourceBindingDigest: binding.Digest})
		inspection.TotalSourceEvents += entry.SourceEvents
		inspection.TotalRetainedEvents += entry.RetainedEvents
		inspection.TotalOmittedEvents += entry.OmittedEvents
		if entry.RetentionRatio < inspection.MinimumRetentionRatio {
			inspection.MinimumRetentionRatio = entry.RetentionRatio
		}
		if entry.RetainedMessages > 0 {
			inspection.PacketsWithMessages++
		} else {
			inspection.PacketsWithoutMessages++
		}
		switch entry.DecisionAnchorKind {
		case string(preprocess.EventFileChange):
			inspection.PatchAnchors++
		case string(preprocess.EventMessage):
			inspection.NarrativeAnchors++
		default:
			return OutcomePilotInspection{}, errors.New("outcome pilot inspection encountered an unsupported decision anchor")
		}
	}
	sort.Slice(inspection.Items, func(left, right int) bool { return inspection.Items[left].PacketID < inspection.Items[right].PacketID })
	sort.Slice(inspection.BindingReferences, func(left, right int) bool {
		return inspection.BindingReferences[left].PacketID < inspection.BindingReferences[right].PacketID
	})
	inspection.SourceBindingCommitmentDigest, err = digestJSON(inspection.BindingReferences)
	if err != nil {
		return OutcomePilotInspection{}, err
	}
	inspection.MappingCommitmentDigest, err = digestJSON(mappingReferences)
	if err != nil {
		return OutcomePilotInspection{}, err
	}
	return SealOutcomePilotInspection(inspection)
}

func SealOutcomePilotInspection(inspection OutcomePilotInspection) (OutcomePilotInspection, error) {
	inspection.SchemaVersion, inspection.CanonicalPolicy, inspection.Digest = OutcomePilotInspectionSchemaVersion, CanonicalPolicy, ""
	digest, err := outcomePilotInspectionDigest(inspection)
	if err != nil {
		return OutcomePilotInspection{}, err
	}
	inspection.Digest = digest
	return inspection, inspection.Validate()
}

func (inspection OutcomePilotInspection) Validate() error {
	if inspection.SchemaVersion != OutcomePilotInspectionSchemaVersion || inspection.CanonicalPolicy != CanonicalPolicy || inspection.Objective != ReviewObjectiveOutcome ||
		!validDigest(inspection.PilotSampleDigest) || !validDigest(inspection.BundleDigest) || !validDigest(inspection.PrivateMaterialsDigest) || inspection.Packets < 1 ||
		len(inspection.Items) != inspection.Packets || len(inspection.BindingReferences) != inspection.Packets ||
		!validDigest(inspection.SourceBindingCommitmentDigest) || !validDigest(inspection.MappingCommitmentDigest) ||
		inspection.ReviewabilityStatus != OutcomePilotStructurallyReady || inspection.SemanticStatus != OutcomePilotRequiresHumanPilot ||
		inspection.TotalSourceEvents < inspection.Packets || inspection.TotalRetainedEvents < inspection.Packets || inspection.TotalOmittedEvents < 0 ||
		inspection.TotalSourceEvents != inspection.TotalRetainedEvents+inspection.TotalOmittedEvents ||
		inspection.MinimumRetentionRatio <= 0 || inspection.MinimumRetentionRatio > 1 ||
		inspection.PacketsWithMessages < 0 || inspection.PacketsWithoutMessages < 0 || inspection.PacketsWithMessages+inspection.PacketsWithoutMessages != inspection.Packets ||
		inspection.PatchAnchors < 0 || inspection.NarrativeAnchors < 0 || inspection.PatchAnchors+inspection.NarrativeAnchors != inspection.Packets {
		return errors.New("outcome pilot inspection identity, coverage, status, or aggregate accounting is invalid")
	}
	if !equalStrings(inspection.Limitations, outcomePilotInspectionLimitations()) {
		return errors.New("outcome pilot inspection claim boundary is invalid")
	}
	if err := validateOutcomePilotInspectionItems(inspection); err != nil {
		return err
	}
	expectedBindings, err := digestJSON(inspection.BindingReferences)
	if err != nil || inspection.SourceBindingCommitmentDigest != expectedBindings {
		return errors.New("outcome pilot source-binding commitment is invalid")
	}
	expected, err := outcomePilotInspectionDigest(inspection)
	if err != nil || inspection.Digest != expected {
		return errors.New("outcome pilot inspection digest is invalid")
	}
	return nil
}

func DecodeOutcomePilotInspection(reader io.Reader) (OutcomePilotInspection, error) {
	var value OutcomePilotInspection
	if err := decodeStrict(reader, &value); err != nil {
		return OutcomePilotInspection{}, fmt.Errorf("decode outcome pilot inspection: %w", err)
	}
	return value, value.Validate()
}

func validateOutcomePilotInspectionItems(inspection OutcomePilotInspection) error {
	if len(inspection.Items) != len(inspection.BindingReferences) {
		return errors.New("outcome pilot inspection item and binding-reference counts differ")
	}
	totalSource, totalRetained, totalOmitted := 0, 0, 0
	withMessages, patchAnchors, narrativeAnchors := 0, 0, 0
	minimumRatio := 1.0
	for index, item := range inspection.Items {
		reference := inspection.BindingReferences[index]
		if !validOpaquePacketID(item.PacketID) || !validTaskGroupID(item.TaskGroupID) || !validDigest(item.SourceBindingDigest) ||
			item.PacketID != reference.PacketID || item.SourceBindingDigest != reference.SourceBindingDigest ||
			!validDigest(item.DecisionAnchorDigest) || !validDigest(item.TrajectoryEvidenceDigest) ||
			item.SourceEvents < 1 || item.RetainedEvents < 1 || item.RetainedEvents > item.SourceEvents || item.OmittedEvents != item.SourceEvents-item.RetainedEvents ||
			item.RetentionRatio != float64(item.RetainedEvents)/float64(item.SourceEvents) || item.RetainedMessages < 0 || item.RetainedActions < 1 || item.RetainedResults < 1 ||
			item.EvidenceBudgetTokens != OutcomePilotEvidenceBudgetTokens || item.RenderedEvidenceBytes < 1 || item.EstimatedEvidenceTokens < 1 ||
			item.EstimatedEvidenceTokens > item.EvidenceBudgetTokens || item.ReviewabilityStatus != OutcomePilotStructurallyReady ||
			index > 0 && inspection.Items[index-1].PacketID >= item.PacketID {
			return errors.New("outcome pilot packet inspection identity, ordering, retention, evidence budget, or structural coverage is invalid")
		}
		switch item.DecisionAnchorKind {
		case string(preprocess.EventFileChange):
			patchAnchors++
		case string(preprocess.EventMessage):
			narrativeAnchors++
		default:
			return errors.New("outcome pilot packet inspection decision anchor is invalid")
		}
		if item.RetainedMessages > 0 {
			withMessages++
		}
		totalSource += item.SourceEvents
		totalRetained += item.RetainedEvents
		totalOmitted += item.OmittedEvents
		if item.RetentionRatio < minimumRatio {
			minimumRatio = item.RetentionRatio
		}
	}
	expectedWithoutMessages := inspection.Packets - withMessages
	if totalSource != inspection.TotalSourceEvents || totalRetained != inspection.TotalRetainedEvents || totalOmitted != inspection.TotalOmittedEvents ||
		withMessages != inspection.PacketsWithMessages || inspection.PacketsWithoutMessages != expectedWithoutMessages ||
		patchAnchors != inspection.PatchAnchors || narrativeAnchors != inspection.NarrativeAnchors || minimumRatio != inspection.MinimumRetentionRatio {
		return errors.New("outcome pilot inspection aggregate accounting does not reproduce its packet rows")
	}
	return nil
}

func outcomePilotTrajectoryEvidence(packet BlindPacket) (PacketEvidence, error) {
	for _, evidence := range packet.Evidence {
		if evidence.Kind == "trajectory_evidence" {
			return evidence, nil
		}
	}
	return PacketEvidence{}, errors.New("outcome pilot packet has no trajectory evidence")
}

func outcomePilotInspectionLimitations() []string {
	return []string{
		"reference-only source material remains restricted and is not made public by this metadata inspection",
		"reviewability inspection verifies deterministic structural coverage and lineage, not semantic sufficiency or rubric validity",
		"semantic acceptance requires the committed independent human development pilot",
	}
}

func outcomePilotInspectionDigest(value OutcomePilotInspection) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
