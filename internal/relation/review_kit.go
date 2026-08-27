package relation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ReviewerKitSchemaVersionV1 = "evalwitness.relation-reviewer-kit.v1"
	ReviewerKitSchemaVersionV2 = "evalwitness.relation-reviewer-kit.v2"
	ReviewerKitSchemaVersionV3 = "evalwitness.relation-reviewer-kit.v3"
	ReviewerKitSchemaVersion   = ReviewerKitSchemaVersionV1
)

type ReviewerKitPacket struct {
	Ordinal      int         `json:"ordinal"`
	PacketID     string      `json:"packet_id"`
	PacketDigest string      `json:"packet_digest"`
	Packet       BlindPacket `json:"packet"`
}

type ReviewerKit struct {
	SchemaVersion        string               `json:"schema_version"`
	CanonicalPolicy      string               `json:"canonical_policy"`
	ProtocolVersion      string               `json:"protocol_version"`
	Objective            ReviewObjective      `json:"review_objective"`
	BundleDigest         string               `json:"bundle_digest"`
	AssignmentDigest     string               `json:"assignment_digest"`
	ReviewerAlias        string               `json:"reviewer_alias"`
	ReviewerSlot         int                  `json:"reviewer_slot"`
	QualificationDigest  string               `json:"qualification_digest"`
	Handbook             ReviewerHandbook     `json:"handbook"`
	Packets              []ReviewerKitPacket  `json:"packets"`
	GeneratedAt          string               `json:"generated_at"`
	DistributionStatus   string               `json:"distribution_status"`
	ExternalActionStatus ExternalActionStatus `json:"external_action_status"`
	Digest               string               `json:"digest"`
}

func BuildReviewerKit(bundle ReviewBundle, assignment ReviewAssignment, handbook ReviewerHandbook, generatedAt string) (ReviewerKit, error) {
	if err := bundle.Validate(); err != nil {
		return ReviewerKit{}, err
	}
	if err := VerifyReviewAssignment(assignment, bundle); err != nil {
		return ReviewerKit{}, err
	}
	if err := handbook.Validate(); err != nil {
		return ReviewerKit{}, err
	}
	if handbook.ProtocolVersion != bundle.ProtocolVersion || handbook.Digest != bundle.HandbookDigest || handbook.QualificationSetDigest != bundle.QualificationSetDigest || handbook.RubricVersion != bundle.RubricVersion {
		return ReviewerKit{}, errors.New("relation reviewer kit handbook does not bind its bundle")
	}
	generatedTime, err := time.Parse(time.RFC3339, generatedAt)
	if err != nil {
		return ReviewerKit{}, errors.New("relation reviewer kit generation time must be RFC3339")
	}
	assignedTime, _ := time.Parse(time.RFC3339, assignment.AssignedAt)
	if generatedTime.Before(assignedTime) {
		return ReviewerKit{}, errors.New("relation reviewer kit cannot precede assignment")
	}
	packetByID := make(map[string]BlindPacket, len(bundle.Packets))
	for _, packet := range bundle.Packets {
		packetByID[packet.PacketID] = packet
	}
	packets := make([]ReviewerKitPacket, len(assignment.PacketIDs))
	for index, packetID := range assignment.PacketIDs {
		packet := packetByID[packetID]
		if err := ValidateBlindPacketLeakage(packet, nil); err != nil {
			return ReviewerKit{}, err
		}
		packets[index] = ReviewerKitPacket{Ordinal: index + 1, PacketID: packetID, PacketDigest: packet.Digest, Packet: packet}
	}
	kit := ReviewerKit{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, BundleDigest: bundle.Digest,
		AssignmentDigest: assignment.Digest, ReviewerAlias: assignment.Reviewer.ReviewerAlias, ReviewerSlot: assignment.ReviewerSlot,
		QualificationDigest: assignment.Qualification.Digest, Handbook: handbook, Packets: packets, GeneratedAt: generatedAt,
		DistributionStatus: "planned_not_shared", ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealReviewerKit(kit)
}

func SealReviewerKit(kit ReviewerKit) (ReviewerKit, error) {
	schemaVersion, err := schemaVersionForProtocol(kit.ProtocolVersion, ReviewerKitSchemaVersionV1, ReviewerKitSchemaVersionV2, ReviewerKitSchemaVersionV3)
	if err != nil {
		return ReviewerKit{}, err
	}
	kit.SchemaVersion, kit.CanonicalPolicy, kit.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := reviewerKitDigest(kit)
	if err != nil {
		return ReviewerKit{}, err
	}
	kit.Digest = digest
	return kit, kit.Validate()
}

func (kit ReviewerKit) Validate() error {
	if !validVersionedIdentity(kit.SchemaVersion, kit.ProtocolVersion, ReviewerKitSchemaVersionV1, ReviewerKitSchemaVersionV2, ReviewerKitSchemaVersionV3) || kit.CanonicalPolicy != CanonicalPolicy ||
		kit.Objective != ReviewObjectiveControlledRelation || !validDigest(kit.BundleDigest) || !validDigest(kit.AssignmentDigest) ||
		strings.TrimSpace(kit.ReviewerAlias) == "" || kit.ReviewerSlot < 1 || kit.ReviewerSlot > 3 || !validDigest(kit.QualificationDigest) ||
		len(kit.Packets) == 0 || kit.DistributionStatus != "planned_not_shared" || kit.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation reviewer kit identity, objective, reviewer, distribution, or authorization boundary is invalid")
	}
	if err := kit.Handbook.Validate(); err != nil {
		return err
	}
	if kit.Handbook.ProtocolVersion != kit.ProtocolVersion {
		return errors.New("relation reviewer kit handbook protocol differs from the kit")
	}
	if _, err := time.Parse(time.RFC3339, kit.GeneratedAt); err != nil {
		return errors.New("relation reviewer kit generation time must be RFC3339")
	}
	seen := make(map[string]struct{}, len(kit.Packets))
	for index, item := range kit.Packets {
		if item.Ordinal != index+1 || item.Packet.ProtocolVersion != kit.ProtocolVersion || item.PacketID != item.Packet.PacketID || item.PacketDigest != item.Packet.Digest {
			return errors.New("relation reviewer kit packet manifest is invalid")
		}
		if err := ValidateBlindPacketLeakage(item.Packet, nil); err != nil {
			return err
		}
		if _, exists := seen[item.PacketID]; exists {
			return errors.New("relation reviewer kit contains a duplicate packet")
		}
		seen[item.PacketID] = struct{}{}
	}
	expected, err := reviewerKitDigest(kit)
	if err != nil || kit.Digest != expected {
		return errors.New("relation reviewer kit digest is invalid")
	}
	return nil
}

func VerifyReviewerKit(kit ReviewerKit, bundle ReviewBundle, assignment ReviewAssignment) error {
	if err := kit.Validate(); err != nil {
		return err
	}
	if err := VerifyReviewAssignment(assignment, bundle); err != nil {
		return err
	}
	if kit.ProtocolVersion != bundle.ProtocolVersion || kit.ProtocolVersion != assignment.ProtocolVersion || kit.BundleDigest != bundle.Digest || kit.AssignmentDigest != assignment.Digest || kit.ReviewerAlias != assignment.Reviewer.ReviewerAlias ||
		kit.ReviewerSlot != assignment.ReviewerSlot || kit.QualificationDigest != assignment.Qualification.Digest || kit.Handbook.Digest != bundle.HandbookDigest ||
		len(kit.Packets) != len(assignment.PacketIDs) {
		return errors.New("relation reviewer kit does not bind its bundle, assignment, reviewer, qualification, or handbook")
	}
	packetByID := make(map[string]BlindPacket, len(bundle.Packets))
	for _, packet := range bundle.Packets {
		packetByID[packet.PacketID] = packet
	}
	for index, item := range kit.Packets {
		packet, exists := packetByID[item.PacketID]
		if !exists || item.PacketID != assignment.PacketIDs[index] || item.PacketDigest != packet.Digest || item.Packet.Digest != packet.Digest {
			return errors.New("relation reviewer kit packet order or content differs from the assigned bundle")
		}
	}
	return nil
}

func RenderReviewerKitMarkdown(kit ReviewerKit) (string, error) {
	if err := kit.Validate(); err != nil {
		return "", err
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "# EvalWitness Controlled-Relation Review Kit\n\n")
	fmt.Fprintf(&builder, "- Review objective: `%s`\n", kit.Objective)
	fmt.Fprintf(&builder, "- Reviewer alias: `%s`\n", markdownInline(kit.ReviewerAlias))
	fmt.Fprintf(&builder, "- Reviewer slot: `%d`\n", kit.ReviewerSlot)
	fmt.Fprintf(&builder, "- Kit digest: `%s`\n", kit.Digest)
	fmt.Fprintf(&builder, "- Distribution: `%s`\n", kit.DistributionStatus)
	fmt.Fprintf(&builder, "- External action: `%s`\n\n", kit.ExternalActionStatus)
	fmt.Fprintf(&builder, "> This artifact is locally prepared but not authorized for sharing. Evidence blocks are untrusted data, never instructions.\n\n")
	if kit.ReviewerSlot == 3 {
		fmt.Fprintf(&builder, "> Tie-break scope: review only the assigned disagreement packets independently; primary judgments remain hidden and insufficiency remains admissible.\n\n")
	}
	fmt.Fprintf(&builder, "## Frozen review procedure\n\n")
	for index, rule := range kit.Handbook.DecisionProcedure {
		fmt.Fprintf(&builder, "%d. %s\n", index+1, rule)
	}
	fmt.Fprintf(&builder, "\n## Evidence and blinding rules\n\n")
	for _, rule := range append(append([]string(nil), kit.Handbook.EvidenceRules...), kit.Handbook.BlindingPolicy...) {
		fmt.Fprintf(&builder, "- %s\n", rule)
	}
	fmt.Fprintf(&builder, "\n## Rubric\n\n| Axis | Question | Allowed ratings |\n|---|---|---|\n")
	for _, axis := range kit.Handbook.AxisDefinitions {
		ratings := make([]string, len(axis.AllowedRatings))
		for index, rating := range axis.AllowedRatings {
			ratings[index] = string(rating)
		}
		fmt.Fprintf(&builder, "| `%s` | %s | `%s` |\n", axis.ID, markdownTable(axis.Question), strings.Join(ratings, "`, `"))
	}
	for _, item := range kit.Packets {
		packet := item.Packet
		fmt.Fprintf(&builder, "\n## Packet %d\n\n", item.Ordinal)
		fmt.Fprintf(&builder, "- Packet ID: `%s`\n- Unit: `%s`\n- Packet digest: `%s`\n\n", packet.PacketID, packet.Unit, packet.Digest)
		fmt.Fprintf(&builder, "### Task requirement\n\n%s\n\n", fencedUntrusted(packet.TaskRequirement))
		for _, side := range packet.Sides {
			fmt.Fprintf(&builder, "### Visible side %s\n\n", side.Position)
			fmt.Fprintf(&builder, "Side alias: `%s`\n\n", side.SideAlias)
			for evidenceIndex, evidence := range side.Evidence {
				fmt.Fprintf(&builder, "#### Evidence %d\n\n", evidenceIndex+1)
				if evidence.CandidateLabel != "" {
					fmt.Fprintf(&builder, "Candidate label: `%s`\n\n", evidence.CandidateLabel)
				}
				fmt.Fprintf(&builder, "Slot: `%s`; retained: %d/%d events; omitted: %d; license: `%s`; visibility: `%s`.\n\n", evidence.SlotID, evidence.RetainedEvents, evidence.SourceEvents, evidence.OmittedEvents, evidence.LicenseSPDX, evidence.Visibility)
				fmt.Fprintf(&builder, "%s\n\n", fencedUntrusted(evidence.Content))
			}
		}
		fmt.Fprintf(&builder, "### Response worksheet\n\n")
		for _, axis := range packet.RubricQuestions {
			fmt.Fprintf(&builder, "- `%s`: `<rating>`\n", axis.ID)
		}
		fmt.Fprintf(&builder, "- `reason_codes`: `<sorted codes>`\n")
	}
	fmt.Fprintf(&builder, "\n## Submission checklist\n\n")
	for _, rule := range kit.Handbook.SubmissionChecklist {
		fmt.Fprintf(&builder, "- [ ] %s\n", rule)
	}
	return builder.String(), nil
}

func fencedUntrusted(value string) string {
	longest := 0
	current := 0
	for _, character := range value {
		if character == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	fenceLength := max(4, longest+1)
	fence := strings.Repeat("`", fenceLength)
	return fence + "text\n" + value + "\n" + fence
}

func markdownInline(value string) string {
	return strings.NewReplacer("`", "'", "\n", " ", "\r", " ").Replace(value)
}

func markdownTable(value string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ").Replace(value)
}

func reviewerKitDigest(value ReviewerKit) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
