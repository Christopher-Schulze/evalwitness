package outcome

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

const ReviewerKitSchemaVersion = "evalwitness.outcome-reviewer-kit.v1"

type ReviewerKit struct {
	SchemaVersion   string           `json:"schema_version"`
	CanonicalPolicy string           `json:"canonical_policy"`
	BundleDigest    string           `json:"bundle_digest"`
	PlanDigest      string           `json:"plan_digest"`
	DataRole        ReviewDataRole   `json:"data_role"`
	Visibility      ReviewVisibility `json:"visibility"`
	Handbook        ReviewerHandbook `json:"handbook"`
	Assignment      ReviewAssignment `json:"assignment"`
	GeneratedAt     string           `json:"generated_at"`
	Packets         []BlindPacket    `json:"packets"`
	Digest          string           `json:"digest"`
}

func BuildReviewerKit(bundle ReviewBundle, assignment ReviewAssignment, handbook ReviewerHandbook, generatedAt string) (ReviewerKit, error) {
	if err := bundle.Validate(); err != nil {
		return ReviewerKit{}, err
	}
	if err := assignment.Validate(); err != nil {
		return ReviewerKit{}, err
	}
	if err := handbook.Validate(); err != nil {
		return ReviewerKit{}, err
	}
	if assignment.BundleDigest != bundle.Digest || handbook.Digest != bundle.HandbookDigest || handbook.HandbookVersion != bundle.HandbookVersion ||
		handbook.QualificationSetDigest != bundle.QualificationSetDigest || handbook.RubricVersion != bundle.RubricVersion {
		return ReviewerKit{}, errors.New("reviewer kit inputs do not bind the same bundle, handbook, qualification, or rubric")
	}
	packetsByID := make(map[string]BlindPacket, len(bundle.Items))
	for _, item := range bundle.Items {
		packetsByID[item.Packet.PacketID] = item.Packet
	}
	packets := make([]BlindPacket, 0, len(assignment.PacketIDs))
	for _, packetID := range assignment.PacketIDs {
		packet, exists := packetsByID[packetID]
		if !exists {
			return ReviewerKit{}, errors.New("reviewer assignment contains a packet absent from its bundle")
		}
		packets = append(packets, packet)
	}
	return SealReviewerKit(ReviewerKit{
		BundleDigest: bundle.Digest, PlanDigest: bundle.PlanDigest, DataRole: bundle.DataRole, Visibility: bundle.Visibility,
		Handbook: handbook, Assignment: assignment, GeneratedAt: generatedAt, Packets: packets,
	})
}

func SealReviewerKit(kit ReviewerKit) (ReviewerKit, error) {
	kit.SchemaVersion = ReviewerKitSchemaVersion
	kit.CanonicalPolicy = CanonicalPolicy
	kit.Digest = ""
	digest, err := reviewerKitDigest(kit)
	if err != nil {
		return ReviewerKit{}, err
	}
	kit.Digest = digest
	return kit, kit.Validate()
}

func (kit ReviewerKit) Validate() error {
	if kit.SchemaVersion != ReviewerKitSchemaVersion || kit.CanonicalPolicy != CanonicalPolicy || !validDigest(kit.BundleDigest) ||
		!validDigest(kit.PlanDigest) || !validReviewDataRole(kit.DataRole) || !validReviewVisibility(kit.Visibility) || missing(kit.GeneratedAt) || len(kit.Packets) == 0 {
		return errors.New("reviewer kit identity, data role, visibility, time, or packets are invalid")
	}
	if err := kit.Handbook.Validate(); err != nil {
		return err
	}
	if err := kit.Assignment.Validate(); err != nil {
		return err
	}
	if kit.Assignment.BundleDigest != kit.BundleDigest || kit.Assignment.RubricVersion != kit.Handbook.RubricVersion ||
		kit.Assignment.QualificationSetDigest != kit.Handbook.QualificationSetDigest {
		return errors.New("reviewer kit assignment does not bind its bundle, handbook, or qualification")
	}
	if err := requireNotBefore("reviewer kit generation", kit.GeneratedAt, kit.Assignment.AssignedAt); err != nil {
		return err
	}
	if len(kit.Packets) != len(kit.Assignment.PacketIDs) {
		return errors.New("reviewer kit packet count does not match its assignment")
	}
	for index, packet := range kit.Packets {
		if err := packet.Validate(); err != nil {
			return fmt.Errorf("reviewer kit packet %d: %w", index, err)
		}
		if packet.PacketID != kit.Assignment.PacketIDs[index] || packet.PlanDigest != kit.PlanDigest {
			return errors.New("reviewer kit packets do not reproduce the assigned order and plan")
		}
		if kit.Visibility == ReviewVisibilityPublic && !packet.PublicReleasable {
			return errors.New("public reviewer kit contains a restricted packet")
		}
	}
	expected, err := reviewerKitDigest(kit)
	if err != nil || kit.Digest != expected {
		return errors.New("reviewer kit digest is invalid")
	}
	return nil
}

func VerifyReviewerKit(kit ReviewerKit, bundle ReviewBundle) error {
	if err := kit.Validate(); err != nil {
		return err
	}
	if err := bundle.Validate(); err != nil {
		return err
	}
	if kit.BundleDigest != bundle.Digest || kit.PlanDigest != bundle.PlanDigest || kit.DataRole != bundle.DataRole || kit.Visibility != bundle.Visibility ||
		kit.Handbook.Digest != bundle.HandbookDigest || kit.Handbook.HandbookVersion != bundle.HandbookVersion {
		return errors.New("reviewer kit does not reproduce its review bundle")
	}
	itemsByID := make(map[string]BlindPacket, len(bundle.Items))
	for _, item := range bundle.Items {
		itemsByID[item.Packet.PacketID] = item.Packet
	}
	for _, packet := range kit.Packets {
		bundlePacket, exists := itemsByID[packet.PacketID]
		if !exists || bundlePacket.Digest != packet.Digest {
			return errors.New("reviewer kit contains a packet not bound by the review bundle")
		}
	}
	return nil
}

func DecodeReviewerKit(reader io.Reader) (ReviewerKit, error) {
	var value ReviewerKit
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewerKit{}, fmt.Errorf("decode reviewer kit: %w", err)
	}
	return value, value.Validate()
}

func RenderReviewerKitMarkdown(kit ReviewerKit) (string, error) {
	if err := kit.Validate(); err != nil {
		return "", err
	}
	var output strings.Builder
	fmt.Fprintln(&output, "# EvalWitness Blinded Review Kit")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "Kit digest: `%s`\n\n", kit.Digest)
	fmt.Fprintf(&output, "Assignment: `%s`, reviewer `%s`, slot `%d`, purpose `%s`\n\n", kit.Assignment.Digest, markdownInline(kit.Assignment.Reviewer.AdjudicatorAlias), kit.Assignment.ReviewerSlot, kit.Assignment.Purpose)
	fmt.Fprintf(&output, "Data role: `%s`; visibility: `%s`; rubric: `%s`; generated: `%s`\n\n", kit.DataRole, kit.Visibility, kit.Handbook.RubricVersion, kit.GeneratedAt)
	fmt.Fprintln(&output, kit.Handbook.Purpose)
	renderNumberedMarkdown(&output, "Decision procedure", kit.Handbook.DecisionProcedure)
	renderBulletMarkdown(&output, "Evidence rules", kit.Handbook.EvidenceRules)
	fmt.Fprintln(&output, "## Outcome definitions")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| outcome | meaning | required evidence |")
	fmt.Fprintln(&output, "|---|---|---|")
	for _, definition := range kit.Handbook.OutcomeDefinitions {
		fmt.Fprintf(&output, "| `%s` | %s | %s |\n", definition.State, markdownCell(definition.Meaning), markdownCell(definition.RequiredEvidence))
	}
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Rubric axes")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| axis | meaning |")
	fmt.Fprintln(&output, "|---|---|")
	for _, definition := range kit.Handbook.AxisDefinitions {
		fmt.Fprintf(&output, "| `%s` | %s |\n", definition.Name, markdownCell(definition.Meaning))
	}
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Reason codes")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| code | meaning |")
	fmt.Fprintln(&output, "|---|---|")
	for _, definition := range kit.Handbook.ReasonDefinitions {
		fmt.Fprintf(&output, "| `%s` | %s |\n", definition.Code, markdownCell(definition.Meaning))
	}
	fmt.Fprintln(&output)
	renderBulletMarkdown(&output, "Conflict policy", kit.Handbook.ConflictPolicy)
	renderBulletMarkdown(&output, "Blinding policy", kit.Handbook.BlindingPolicy)
	renderDatasetStatement(&output, kit.Handbook.DatasetStatement)
	fmt.Fprintln(&output, "## Assigned packets")
	fmt.Fprintln(&output)
	for packetIndex, packet := range kit.Packets {
		fmt.Fprintf(&output, "### %d. `%s`\n\n", packetIndex+1, packet.PacketID)
		fmt.Fprintf(&output, "Task alias: `%s`\n\n", markdownInline(packet.TaskAlias))
		for evidenceIndex, evidence := range packet.Evidence {
			fmt.Fprintf(&output, "#### Evidence %d: `%s`\n\n", evidenceIndex+1, evidence.Slot)
			fmt.Fprintf(&output, "Kind: `%s`; license: `%s`; limitation: %s; content digest: `%s`\n\n", markdownInline(evidence.Kind), markdownInline(evidence.License), markdownInline(evidence.Limitation), evidence.ContentDigest)
			if evidence.Content == "" {
				fmt.Fprintln(&output, "Content is unavailable under this packet's release boundary. Judge only the supplied metadata and record evidence sufficiency accordingly.")
				fmt.Fprintln(&output)
			} else {
				renderIndentedMarkdown(&output, evidence.Content)
			}
		}
		fmt.Fprintln(&output, "Rubric questions:")
		fmt.Fprintln(&output)
		for _, question := range packet.RubricQuestions {
			fmt.Fprintf(&output, "- %s\n", markdownText(question))
		}
		fmt.Fprintln(&output)
		fmt.Fprintln(&output, "Record one primary outcome, all five axis ratings, every applicable reason code, the RFC3339 submission time, and any newly discovered conflict. Do not include free-form source guesses.")
		fmt.Fprintln(&output)
	}
	renderBulletMarkdown(&output, "Submission checklist", kit.Handbook.SubmissionChecklist)
	fmt.Fprintln(&output, "Seal each label with `evalwitness outcome label`, then commit the exact complete array with `evalwitness outcome review label-batch`. Do not inspect mappings, seeds, another reviewer's labels, or tie-break material before commitment.")
	return output.String(), nil
}

func renderNumberedMarkdown(output *strings.Builder, heading string, values []string) {
	fmt.Fprintf(output, "## %s\n\n", heading)
	for index, value := range values {
		fmt.Fprintf(output, "%d. %s\n", index+1, value)
	}
	fmt.Fprintln(output)
}

func renderBulletMarkdown(output *strings.Builder, heading string, values []string) {
	fmt.Fprintf(output, "## %s\n\n", heading)
	for _, value := range values {
		fmt.Fprintf(output, "- %s\n", value)
	}
	fmt.Fprintln(output)
}

func renderDatasetStatement(output *strings.Builder, statement DatasetStatement) {
	fmt.Fprintln(output, "## Dataset statement")
	fmt.Fprintln(output)
	fmt.Fprintf(output, "Unit of review: %s\n\n", statement.UnitOfReview)
	fmt.Fprintln(output, "Sources:")
	fmt.Fprintln(output)
	for _, source := range statement.Sources {
		fmt.Fprintf(output, "- %s\n", source)
	}
	fmt.Fprintln(output)
	fmt.Fprintf(output, "Data roles: %s\n\nSampling: %s\n\n", statement.DataRolePolicy, statement.SamplingDisclosure)
	fmt.Fprintln(output, "Known coverage gaps:")
	fmt.Fprintln(output)
	for _, gap := range statement.KnownCoverageGaps {
		fmt.Fprintf(output, "- %s\n", gap)
	}
	fmt.Fprintln(output)
	fmt.Fprintf(output, "Redistribution: %s\n\nPrivacy: %s\n\nHuman data: %s\n\nGeneralization: %s\n\n", statement.RedistributionRule, statement.PrivacyRule, statement.HumanDataRule, statement.GeneralizationLimit)
}

func renderIndentedMarkdown(output *strings.Builder, value string) {
	for _, line := range strings.Split(value, "\n") {
		fmt.Fprintf(output, "    %s\n", line)
	}
	fmt.Fprintln(output)
}

func markdownCell(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "|", "\\|"), "\n", " ")
}

func markdownInline(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "`", "'"), "\n", " ")
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func markdownText(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.NewReplacer(
		"\\", "\\\\", "&", "&amp;", "<", "&lt;", ">", "&gt;", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "#", "\\#", "|", "\\|",
	).Replace(value)
}

func reviewerKitDigest(value ReviewerKit) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
