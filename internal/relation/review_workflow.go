package relation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	ReviewBundleSchemaVersionV1     = "evalwitness.relation-review-bundle.v1"
	ReviewBundleSchemaVersionV2     = "evalwitness.relation-review-bundle.v2"
	ReviewBundleSchemaVersionV3     = "evalwitness.relation-review-bundle.v3"
	ReviewBundleSchemaVersion       = ReviewBundleSchemaVersionV1
	ReviewerRecordSchemaVersionV1   = "evalwitness.relation-reviewer-record.v1"
	ReviewerRecordSchemaVersionV2   = "evalwitness.relation-reviewer-record.v2"
	ReviewerRecordSchemaVersionV3   = "evalwitness.relation-reviewer-record.v3"
	ReviewerRecordSchemaVersion     = ReviewerRecordSchemaVersionV1
	ReviewAssignmentSchemaVersionV1 = "evalwitness.relation-review-assignment.v1"
	ReviewAssignmentSchemaVersionV2 = "evalwitness.relation-review-assignment.v2"
	ReviewAssignmentSchemaVersionV3 = "evalwitness.relation-review-assignment.v3"
	ReviewAssignmentSchemaVersion   = ReviewAssignmentSchemaVersionV1
	BundleOrderingProtocol          = "evalwitness.relation-private-packet-order.v1"
	AssignmentOrderingProtocol      = "evalwitness.relation-reviewer-assignment-order.v1"
	domainAssignmentPacketOrder     = "evalwitness.relation.assignment-packet-order.v1"
)

type ReviewDataRole string

const (
	ReviewDataDevelopmentPilot ReviewDataRole = "development_pilot"
	ReviewDataPrimaryAudit     ReviewDataRole = "primary_audit"
)

type ReviewVisibility string

const ReviewVisibilityRestricted ReviewVisibility = "restricted_reference_only"

type ReviewerRole string

const (
	ReviewerRolePrimary  ReviewerRole = "primary"
	ReviewerRoleTieBreak ReviewerRole = "tie_break"
)

type AssignmentPurpose string

const (
	AssignmentPurposePrimary  AssignmentPurpose = "primary"
	AssignmentPurposeTieBreak AssignmentPurpose = "tie_break"
)

type ReviewBundle struct {
	SchemaVersion          string               `json:"schema_version"`
	CanonicalPolicy        string               `json:"canonical_policy"`
	ProtocolVersion        string               `json:"protocol_version"`
	Objective              ReviewObjective      `json:"review_objective"`
	PlanDigest             string               `json:"plan_digest"`
	SampleDigest           string               `json:"sample_digest"`
	RubricVersion          string               `json:"rubric_version"`
	QualificationSetDigest string               `json:"qualification_set_digest"`
	HandbookVersion        string               `json:"handbook_version"`
	HandbookDigest         string               `json:"handbook_digest"`
	DataRole               ReviewDataRole       `json:"data_role"`
	Visibility             ReviewVisibility     `json:"visibility"`
	OrderingProtocol       string               `json:"ordering_protocol"`
	CreatedAt              string               `json:"created_at"`
	Packets                []BlindPacket        `json:"packets"`
	ExternalActionStatus   ExternalActionStatus `json:"external_action_status"`
	Digest                 string               `json:"digest"`
}

type ReviewerRecord struct {
	SchemaVersion               string               `json:"schema_version"`
	CanonicalPolicy             string               `json:"canonical_policy"`
	ProtocolVersion             string               `json:"protocol_version"`
	Objective                   ReviewObjective      `json:"review_objective"`
	ReviewerAlias               string               `json:"reviewer_alias"`
	Role                        ReviewerRole         `json:"role"`
	ConsentedAt                 string               `json:"consented_at"`
	IndependenceAttested        bool                 `json:"independence_attested"`
	AuthorshipPolicyAccepted    bool                 `json:"authorship_policy_accepted"`
	ContactDetailsHeldPrivately bool                 `json:"contact_details_held_privately"`
	ConflictsOfInterest         []string             `json:"conflicts_of_interest"`
	ExternalActionStatus        ExternalActionStatus `json:"external_action_status"`
	Digest                      string               `json:"digest"`
}

type ReviewAssignment struct {
	SchemaVersion          string               `json:"schema_version"`
	CanonicalPolicy        string               `json:"canonical_policy"`
	ProtocolVersion        string               `json:"protocol_version"`
	Objective              ReviewObjective      `json:"review_objective"`
	BundleDigest           string               `json:"bundle_digest"`
	QualificationSetDigest string               `json:"qualification_set_digest"`
	RubricVersion          string               `json:"rubric_version"`
	Purpose                AssignmentPurpose    `json:"purpose"`
	ReviewerSlot           int                  `json:"reviewer_slot"`
	Reviewer               ReviewerRecord       `json:"reviewer"`
	Qualification          QualificationReport  `json:"qualification"`
	OrderingProtocol       string               `json:"ordering_protocol"`
	OrderingSeedDigest     string               `json:"ordering_seed_digest"`
	AssignedAt             string               `json:"assigned_at"`
	DistributionStatus     string               `json:"distribution_status"`
	PacketIDs              []string             `json:"packet_ids"`
	ExternalActionStatus   ExternalActionStatus `json:"external_action_status"`
	Digest                 string               `json:"digest"`
}

func BuildReviewBundle(plan Plan, sampleDigest string, dataRole ReviewDataRole, packets []BlindPacket, mappings []PrivateMapping, qualification QualificationSet, handbook ReviewerHandbook, createdAt string) (ReviewBundle, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return ReviewBundle{}, err
	}
	if !validDigest(sampleDigest) || !slices.Contains([]ReviewDataRole{ReviewDataDevelopmentPilot, ReviewDataPrimaryAudit}, dataRole) || len(packets) == 0 || len(packets) != len(mappings) {
		return ReviewBundle{}, errors.New("relation review bundle requires a sample, data role, and one mapping per packet")
	}
	if err := qualification.Validate(); err != nil {
		return ReviewBundle{}, err
	}
	if err := VerifyReviewerHandbook(handbook, plan, qualification); err != nil {
		return ReviewBundle{}, err
	}
	if _, err := time.Parse(time.RFC3339, createdAt); err != nil {
		return ReviewBundle{}, errors.New("relation review bundle creation time must be RFC3339")
	}
	packetByID := make(map[string]BlindPacket, len(packets))
	mappingByID := make(map[string]PrivateMapping, len(mappings))
	for _, packet := range packets {
		if err := packet.Validate(); err != nil {
			return ReviewBundle{}, err
		}
		if packet.ProtocolVersion != plan.ProtocolVersion || packet.PlanDigest != plan.Digest {
			return ReviewBundle{}, errors.New("relation review bundle packet does not bind its plan")
		}
		if _, exists := packetByID[packet.PacketID]; exists {
			return ReviewBundle{}, errors.New("relation review bundle contains a duplicate packet")
		}
		packetByID[packet.PacketID] = packet
	}
	for _, mapping := range mappings {
		if err := mapping.Validate(); err != nil {
			return ReviewBundle{}, err
		}
		packet, exists := packetByID[mapping.PacketID]
		if !exists || mapping.ProtocolVersion != plan.ProtocolVersion || mapping.PacketDigest != packet.Digest || mapping.PlanDigest != plan.Digest {
			return ReviewBundle{}, errors.New("relation review bundle mapping does not bind one exact packet")
		}
		if _, exists := mappingByID[mapping.PacketID]; exists {
			return ReviewBundle{}, errors.New("relation review bundle contains a duplicate mapping")
		}
		mappingByID[mapping.PacketID] = mapping
	}
	ordered := append([]BlindPacket(nil), packets...)
	sort.Slice(ordered, func(left, right int) bool {
		leftKey := mappingByID[ordered[left].PacketID].PacketOrderKey
		rightKey := mappingByID[ordered[right].PacketID].PacketOrderKey
		if leftKey == rightKey {
			return ordered[left].PacketID < ordered[right].PacketID
		}
		return leftKey < rightKey
	})
	bundle := ReviewBundle{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest, SampleDigest: sampleDigest,
		RubricVersion: plan.RubricVersion, QualificationSetDigest: qualification.Digest, HandbookVersion: handbook.HandbookVersion,
		HandbookDigest: handbook.Digest, DataRole: dataRole, Visibility: ReviewVisibilityRestricted, OrderingProtocol: BundleOrderingProtocol,
		CreatedAt: createdAt, Packets: ordered, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealReviewBundle(bundle)
}

func SealReviewBundle(bundle ReviewBundle) (ReviewBundle, error) {
	schemaVersion, err := schemaVersionForProtocol(bundle.ProtocolVersion, ReviewBundleSchemaVersionV1, ReviewBundleSchemaVersionV2, ReviewBundleSchemaVersionV3)
	if err != nil {
		return ReviewBundle{}, err
	}
	bundle.SchemaVersion, bundle.CanonicalPolicy, bundle.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := reviewBundleDigest(bundle)
	if err != nil {
		return ReviewBundle{}, err
	}
	bundle.Digest = digest
	return bundle, bundle.Validate()
}

func (bundle ReviewBundle) Validate() error {
	expectedHandbookVersion := ReviewerHandbookVersionV1
	switch bundle.ProtocolVersion {
	case ProtocolVersionV2:
		expectedHandbookVersion = ReviewerHandbookVersionV2
	case ProtocolVersionV3:
		expectedHandbookVersion = ReviewerHandbookVersionV3
	}
	if !validVersionedIdentity(bundle.SchemaVersion, bundle.ProtocolVersion, ReviewBundleSchemaVersionV1, ReviewBundleSchemaVersionV2, ReviewBundleSchemaVersionV3) || bundle.CanonicalPolicy != CanonicalPolicy ||
		bundle.Objective != ReviewObjectiveControlledRelation || !validDigest(bundle.PlanDigest) || !validDigest(bundle.SampleDigest) ||
		!validRubricVersion(bundle.ProtocolVersion, bundle.RubricVersion) || !validDigest(bundle.QualificationSetDigest) || bundle.HandbookVersion != expectedHandbookVersion ||
		!validDigest(bundle.HandbookDigest) || !slices.Contains([]ReviewDataRole{ReviewDataDevelopmentPilot, ReviewDataPrimaryAudit}, bundle.DataRole) ||
		bundle.Visibility != ReviewVisibilityRestricted || bundle.OrderingProtocol != BundleOrderingProtocol || len(bundle.Packets) == 0 ||
		bundle.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation review bundle identity, objective, custody, or authorization boundary is invalid")
	}
	if _, err := time.Parse(time.RFC3339, bundle.CreatedAt); err != nil {
		return errors.New("relation review bundle creation time must be RFC3339")
	}
	seen := make(map[string]struct{}, len(bundle.Packets))
	for _, packet := range bundle.Packets {
		if err := packet.Validate(); err != nil {
			return err
		}
		if packet.ProtocolVersion != bundle.ProtocolVersion || packet.PlanDigest != bundle.PlanDigest || packet.RubricVersion != bundle.RubricVersion {
			return errors.New("relation review bundle packet plan or rubric binding is invalid")
		}
		if _, exists := seen[packet.PacketID]; exists {
			return errors.New("relation review bundle packet identities are not unique")
		}
		seen[packet.PacketID] = struct{}{}
	}
	expected, err := reviewBundleDigest(bundle)
	if err != nil || bundle.Digest != expected {
		return errors.New("relation review bundle digest is invalid")
	}
	return nil
}

func VerifyReviewBundle(bundle ReviewBundle, mappings []PrivateMapping) error {
	if err := bundle.Validate(); err != nil {
		return err
	}
	if len(mappings) != len(bundle.Packets) {
		return errors.New("relation review bundle verification requires every private mapping")
	}
	mappingByID := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		if err := mapping.Validate(); err != nil {
			return err
		}
		if mapping.ProtocolVersion != bundle.ProtocolVersion {
			return errors.New("relation review bundle mapping protocol differs from the bundle")
		}
		if _, exists := mappingByID[mapping.PacketID]; exists {
			return errors.New("relation review bundle verification received a duplicate private mapping")
		}
		mappingByID[mapping.PacketID] = mapping
	}
	previousKey, previousPacketID := "", ""
	for _, packet := range bundle.Packets {
		mapping, exists := mappingByID[packet.PacketID]
		if !exists || mapping.PacketDigest != packet.Digest || mapping.PlanDigest != bundle.PlanDigest || mapping.PacketOrderKey < previousKey ||
			mapping.PacketOrderKey == previousKey && packet.PacketID <= previousPacketID {
			return errors.New("relation review bundle packet order or private binding is invalid")
		}
		previousKey, previousPacketID = mapping.PacketOrderKey, packet.PacketID
	}
	return nil
}

func NewReviewerRecord(alias string, role ReviewerRole, consentedAt string, independenceAttested, authorshipAccepted, contactHeldPrivately bool, conflicts []string) (ReviewerRecord, error) {
	return NewReviewerRecordForProtocol(ProtocolVersionV1, alias, role, consentedAt, independenceAttested, authorshipAccepted, contactHeldPrivately, conflicts)
}

func NewReviewerRecordForProtocol(protocolVersion, alias string, role ReviewerRole, consentedAt string, independenceAttested, authorshipAccepted, contactHeldPrivately bool, conflicts []string) (ReviewerRecord, error) {
	conflicts = append([]string(nil), conflicts...)
	sort.Strings(conflicts)
	reviewer := ReviewerRecord{
		ProtocolVersion: protocolVersion, Objective: ReviewObjectiveControlledRelation, ReviewerAlias: strings.TrimSpace(alias), Role: role,
		ConsentedAt: consentedAt, IndependenceAttested: independenceAttested, AuthorshipPolicyAccepted: authorshipAccepted,
		ContactDetailsHeldPrivately: contactHeldPrivately, ConflictsOfInterest: conflicts, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealReviewerRecord(reviewer)
}

func SealReviewerRecord(reviewer ReviewerRecord) (ReviewerRecord, error) {
	schemaVersion, err := schemaVersionForProtocol(reviewer.ProtocolVersion, ReviewerRecordSchemaVersionV1, ReviewerRecordSchemaVersionV2, ReviewerRecordSchemaVersionV3)
	if err != nil {
		return ReviewerRecord{}, err
	}
	reviewer.SchemaVersion, reviewer.CanonicalPolicy, reviewer.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := reviewerRecordDigest(reviewer)
	if err != nil {
		return ReviewerRecord{}, err
	}
	reviewer.Digest = digest
	return reviewer, reviewer.Validate()
}

func (reviewer ReviewerRecord) Validate() error {
	if !validVersionedIdentity(reviewer.SchemaVersion, reviewer.ProtocolVersion, ReviewerRecordSchemaVersionV1, ReviewerRecordSchemaVersionV2, ReviewerRecordSchemaVersionV3) || reviewer.CanonicalPolicy != CanonicalPolicy ||
		reviewer.Objective != ReviewObjectiveControlledRelation || strings.TrimSpace(reviewer.ReviewerAlias) == "" || len(reviewer.ReviewerAlias) > 128 ||
		!slices.Contains([]ReviewerRole{ReviewerRolePrimary, ReviewerRoleTieBreak}, reviewer.Role) || !reviewer.IndependenceAttested ||
		!reviewer.AuthorshipPolicyAccepted || !reviewer.ContactDetailsHeldPrivately || !slices.IsSorted(reviewer.ConflictsOfInterest) || hasDuplicate(reviewer.ConflictsOfInterest) ||
		reviewer.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation reviewer record identity, role, consent, independence, custody, or authorization boundary is invalid")
	}
	if _, err := time.Parse(time.RFC3339, reviewer.ConsentedAt); err != nil {
		return errors.New("relation reviewer consent time must be RFC3339")
	}
	expected, err := reviewerRecordDigest(reviewer)
	if err != nil || reviewer.Digest != expected {
		return errors.New("relation reviewer record digest is invalid")
	}
	return nil
}

func BuildPrimaryAssignment(bundle ReviewBundle, mappings []PrivateMapping, reviewer ReviewerRecord, qualification QualificationReport, reviewerSlot int, seed []byte, assignedAt string) (ReviewAssignment, error) {
	packetIDs := make([]string, len(bundle.Packets))
	for index, packet := range bundle.Packets {
		packetIDs[index] = packet.PacketID
	}
	return buildReviewAssignment(bundle, mappings, reviewer, qualification, AssignmentPurposePrimary, reviewerSlot, packetIDs, seed, assignedAt)
}

func buildReviewAssignment(bundle ReviewBundle, mappings []PrivateMapping, reviewer ReviewerRecord, qualification QualificationReport, purpose AssignmentPurpose, reviewerSlot int, packetIDs []string, seed []byte, assignedAt string) (ReviewAssignment, error) {
	if err := VerifyReviewBundle(bundle, mappings); err != nil {
		return ReviewAssignment{}, err
	}
	if err := reviewer.Validate(); err != nil {
		return ReviewAssignment{}, err
	}
	if err := qualification.Validate(); err != nil {
		return ReviewAssignment{}, err
	}
	if reviewer.ProtocolVersion != bundle.ProtocolVersion || qualification.ProtocolVersion != bundle.ProtocolVersion {
		return ReviewAssignment{}, errors.New("relation assignment reviewer or qualification protocol differs from the bundle")
	}
	expectedRole, expectedSlot := ReviewerRolePrimary, reviewerSlot >= 1 && reviewerSlot <= 2
	if purpose == AssignmentPurposeTieBreak {
		expectedRole, expectedSlot = ReviewerRoleTieBreak, reviewerSlot == 3
	}
	if !slices.Contains([]AssignmentPurpose{AssignmentPurposePrimary, AssignmentPurposeTieBreak}, purpose) || reviewer.Role != expectedRole || !expectedSlot ||
		len(reviewer.ConflictsOfInterest) != 0 || !qualification.Qualified || qualification.ReviewerAlias != reviewer.ReviewerAlias ||
		qualification.QualificationSetDigest != bundle.QualificationSetDigest || qualification.RubricVersion != bundle.RubricVersion || len(seed) < 32 {
		return ReviewAssignment{}, errors.New("relation assignment requires a conflict-free qualified role-matching reviewer, valid slot, and at least 32 seed bytes")
	}
	assignedTime, err := time.Parse(time.RFC3339, assignedAt)
	if err != nil {
		return ReviewAssignment{}, errors.New("relation assignment time must be RFC3339")
	}
	consentedTime, _ := time.Parse(time.RFC3339, reviewer.ConsentedAt)
	qualifiedTime, _ := time.Parse(time.RFC3339, qualification.QualifiedAt)
	if assignedTime.Before(consentedTime) || assignedTime.Before(qualifiedTime) {
		return ReviewAssignment{}, errors.New("relation assignment cannot precede consent or qualification")
	}
	mappingByID := make(map[string]PrivateMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByID[mapping.PacketID] = mapping
	}
	if len(packetIDs) == 0 || hasDuplicate(packetIDs) {
		return ReviewAssignment{}, errors.New("relation assignment requires a nonempty unique packet set")
	}
	bundlePacketIDs := make(map[string]struct{}, len(bundle.Packets))
	for _, packet := range bundle.Packets {
		bundlePacketIDs[packet.PacketID] = struct{}{}
	}
	for _, packetID := range packetIDs {
		if _, exists := bundlePacketIDs[packetID]; !exists {
			return ReviewAssignment{}, errors.New("relation assignment packet is absent from the bundle")
		}
	}
	packetIDs = append([]string(nil), packetIDs...)
	sort.Slice(packetIDs, func(left, right int) bool {
		leftMapping := mappingByID[packetIDs[left]]
		rightMapping := mappingByID[packetIDs[right]]
		leftKey := relationKeyedDigest(seed, domainAssignmentPacketOrder, bundle.Digest, reviewer.Digest, fmt.Sprint(reviewerSlot), leftMapping.ReviewerOrderKey)
		rightKey := relationKeyedDigest(seed, domainAssignmentPacketOrder, bundle.Digest, reviewer.Digest, fmt.Sprint(reviewerSlot), rightMapping.ReviewerOrderKey)
		if leftKey == rightKey {
			return packetIDs[left] < packetIDs[right]
		}
		return leftKey < rightKey
	})
	assignment := ReviewAssignment{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, BundleDigest: bundle.Digest,
		QualificationSetDigest: bundle.QualificationSetDigest, RubricVersion: bundle.RubricVersion, Purpose: purpose,
		ReviewerSlot: reviewerSlot, Reviewer: reviewer, Qualification: qualification, OrderingProtocol: AssignmentOrderingProtocol,
		OrderingSeedDigest: digestText(string(seed)), AssignedAt: assignedAt, DistributionStatus: "planned_not_shared",
		PacketIDs: packetIDs, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealReviewAssignment(assignment)
}

func SealReviewAssignment(assignment ReviewAssignment) (ReviewAssignment, error) {
	schemaVersion, err := schemaVersionForProtocol(assignment.ProtocolVersion, ReviewAssignmentSchemaVersionV1, ReviewAssignmentSchemaVersionV2, ReviewAssignmentSchemaVersionV3)
	if err != nil {
		return ReviewAssignment{}, err
	}
	assignment.SchemaVersion, assignment.CanonicalPolicy, assignment.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := reviewAssignmentDigest(assignment)
	if err != nil {
		return ReviewAssignment{}, err
	}
	assignment.Digest = digest
	return assignment, assignment.Validate()
}

func (assignment ReviewAssignment) Validate() error {
	if !validVersionedIdentity(assignment.SchemaVersion, assignment.ProtocolVersion, ReviewAssignmentSchemaVersionV1, ReviewAssignmentSchemaVersionV2, ReviewAssignmentSchemaVersionV3) || assignment.CanonicalPolicy != CanonicalPolicy ||
		assignment.Objective != ReviewObjectiveControlledRelation || !validDigest(assignment.BundleDigest) || !validDigest(assignment.QualificationSetDigest) ||
		!validRubricVersion(assignment.ProtocolVersion, assignment.RubricVersion) || !slices.Contains([]AssignmentPurpose{AssignmentPurposePrimary, AssignmentPurposeTieBreak}, assignment.Purpose) ||
		assignment.OrderingProtocol != AssignmentOrderingProtocol || !validDigest(assignment.OrderingSeedDigest) || assignment.DistributionStatus != "planned_not_shared" ||
		len(assignment.PacketIDs) == 0 || hasDuplicate(assignment.PacketIDs) || assignment.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation review assignment identity, objective, reviewer slot, order, distribution, or authorization boundary is invalid")
	}
	if err := assignment.Reviewer.Validate(); err != nil {
		return err
	}
	if err := assignment.Qualification.Validate(); err != nil {
		return err
	}
	expectedRole, validSlot := ReviewerRolePrimary, assignment.ReviewerSlot >= 1 && assignment.ReviewerSlot <= 2
	if assignment.Purpose == AssignmentPurposeTieBreak {
		expectedRole, validSlot = ReviewerRoleTieBreak, assignment.ReviewerSlot == 3
	}
	if !validSlot || assignment.Reviewer.ProtocolVersion != assignment.ProtocolVersion || assignment.Qualification.ProtocolVersion != assignment.ProtocolVersion ||
		assignment.Reviewer.Role != expectedRole || len(assignment.Reviewer.ConflictsOfInterest) != 0 || !assignment.Qualification.Qualified ||
		assignment.Reviewer.ReviewerAlias != assignment.Qualification.ReviewerAlias || assignment.Qualification.QualificationSetDigest != assignment.QualificationSetDigest ||
		assignment.Qualification.RubricVersion != assignment.RubricVersion {
		return errors.New("relation review assignment reviewer or qualification binding is invalid")
	}
	assignedTime, err := time.Parse(time.RFC3339, assignment.AssignedAt)
	if err != nil {
		return errors.New("relation assignment time must be RFC3339")
	}
	consentedTime, _ := time.Parse(time.RFC3339, assignment.Reviewer.ConsentedAt)
	qualifiedTime, _ := time.Parse(time.RFC3339, assignment.Qualification.QualifiedAt)
	if assignedTime.Before(consentedTime) || assignedTime.Before(qualifiedTime) {
		return errors.New("relation assignment precedes consent or qualification")
	}
	for _, packetID := range assignment.PacketIDs {
		if !validOpaqueID(packetID, "relation-packet-") {
			return errors.New("relation review assignment contains an invalid packet identity")
		}
	}
	expected, err := reviewAssignmentDigest(assignment)
	if err != nil || assignment.Digest != expected {
		return errors.New("relation review assignment digest is invalid")
	}
	return nil
}

func VerifyReviewAssignment(assignment ReviewAssignment, bundle ReviewBundle) error {
	if err := assignment.Validate(); err != nil {
		return err
	}
	if err := bundle.Validate(); err != nil {
		return err
	}
	if assignment.ProtocolVersion != bundle.ProtocolVersion || assignment.BundleDigest != bundle.Digest || assignment.QualificationSetDigest != bundle.QualificationSetDigest || assignment.RubricVersion != bundle.RubricVersion {
		return errors.New("relation review assignment does not bind the exact bundle")
	}
	bundleIDs := make(map[string]struct{}, len(bundle.Packets))
	for _, packet := range bundle.Packets {
		bundleIDs[packet.PacketID] = struct{}{}
	}
	for _, packetID := range assignment.PacketIDs {
		if _, exists := bundleIDs[packetID]; !exists {
			return errors.New("relation review assignment contains a packet outside the bundle")
		}
	}
	if assignment.Purpose == AssignmentPurposePrimary && len(assignment.PacketIDs) != len(bundle.Packets) {
		return errors.New("relation primary assignment packet coverage is incomplete")
	}
	return nil
}

func reviewBundleDigest(value ReviewBundle) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func reviewerRecordDigest(value ReviewerRecord) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func reviewAssignmentDigest(value ReviewAssignment) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
