package outcome

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"
)

const (
	ReviewBundleSchemaVersion        = "evalwitness.outcome-review-bundle.v1"
	ReviewerRecordSchemaVersion      = "evalwitness.outcome-reviewer-record.v1"
	ReviewAssignmentSchemaVersion    = "evalwitness.outcome-review-assignment.v1"
	LabelBatchSchemaVersion          = "evalwitness.outcome-label-batch.v1"
	MappingRevealSchemaVersion       = "evalwitness.outcome-mapping-reveal.v1"
	AdjudicationLedgerSchemaVersion  = "evalwitness.outcome-adjudication-ledger.v1"
	reviewAssignmentOrderingProtocol = "hmac-sha256-review-order.v1"
)

type ReviewerRole string

const (
	ReviewerRolePrimary  ReviewerRole = "primary"
	ReviewerRoleTieBreak ReviewerRole = "tie_break"
)

type AssignmentPurpose string

const (
	AssignmentPrimary  AssignmentPurpose = "primary"
	AssignmentTieBreak AssignmentPurpose = "tie_break"
)

type ReviewVisibility string

const (
	ReviewVisibilityPublic     ReviewVisibility = "public"
	ReviewVisibilityRestricted ReviewVisibility = "restricted"
)

type ReviewDataRole string

const (
	ReviewDataDevelopment ReviewDataRole = "development"
	ReviewDataCalibration ReviewDataRole = "calibration"
	ReviewDataTest        ReviewDataRole = "test"
)

type AdjudicationStatus string

const (
	AdjudicationComplete   AdjudicationStatus = "complete"
	AdjudicationUnresolved AdjudicationStatus = "unresolved"
)

type ReviewItem struct {
	TaskGroupID string      `json:"task_group_id"`
	Packet      BlindPacket `json:"packet"`
}

type ReviewBundle struct {
	SchemaVersion          string           `json:"schema_version"`
	CanonicalPolicy        string           `json:"canonical_policy"`
	PlanDigest             string           `json:"plan_digest"`
	RubricVersion          string           `json:"rubric_version"`
	QualificationSetDigest string           `json:"qualification_set_digest"`
	HandbookVersion        string           `json:"handbook_version"`
	HandbookDigest         string           `json:"handbook_digest"`
	DataRole               ReviewDataRole   `json:"data_role"`
	Visibility             ReviewVisibility `json:"visibility"`
	CreatedAt              string           `json:"created_at"`
	Items                  []ReviewItem     `json:"items"`
	Digest                 string           `json:"digest"`
}

type ReviewerRecord struct {
	SchemaVersion               string       `json:"schema_version"`
	CanonicalPolicy             string       `json:"canonical_policy"`
	AdjudicatorAlias            string       `json:"adjudicator_alias"`
	Role                        ReviewerRole `json:"role"`
	ConsentedAt                 string       `json:"consented_at"`
	IndependenceAttested        bool         `json:"independence_attested"`
	AuthorshipPolicyAccepted    bool         `json:"authorship_policy_accepted"`
	ContactDetailsHeldPrivately bool         `json:"contact_details_held_privately"`
	ConflictsOfInterest         []string     `json:"conflicts_of_interest"`
	Digest                      string       `json:"digest"`
}

type ReviewAssignment struct {
	SchemaVersion          string              `json:"schema_version"`
	CanonicalPolicy        string              `json:"canonical_policy"`
	BundleDigest           string              `json:"bundle_digest"`
	QualificationSetDigest string              `json:"qualification_set_digest"`
	RubricVersion          string              `json:"rubric_version"`
	Purpose                AssignmentPurpose   `json:"purpose"`
	ReviewerSlot           int                 `json:"reviewer_slot"`
	Reviewer               ReviewerRecord      `json:"reviewer"`
	Qualification          QualificationReport `json:"qualification"`
	OrderingProtocol       string              `json:"ordering_protocol"`
	OrderingSeedDigest     string              `json:"ordering_seed_digest"`
	AssignedAt             string              `json:"assigned_at"`
	PacketIDs              []string            `json:"packet_ids"`
	Digest                 string              `json:"digest"`
}

type LabelBatch struct {
	SchemaVersion       string  `json:"schema_version"`
	CanonicalPolicy     string  `json:"canonical_policy"`
	BundleDigest        string  `json:"bundle_digest"`
	AssignmentDigest    string  `json:"assignment_digest"`
	AdjudicatorAlias    string  `json:"adjudicator_alias"`
	ReviewerSlot        int     `json:"reviewer_slot"`
	QualificationDigest string  `json:"qualification_digest"`
	CommittedAt         string  `json:"committed_at"`
	Labels              []Label `json:"labels"`
	Digest              string  `json:"digest"`
}

type MappingReference struct {
	PacketID      string `json:"packet_id"`
	MappingDigest string `json:"mapping_digest"`
}

type ReviewCommitmentReference struct {
	ReviewerSlot     int    `json:"reviewer_slot"`
	AssignmentDigest string `json:"assignment_digest"`
	BatchDigest      string `json:"batch_digest"`
}

type OrderingSeedReveal struct {
	AssignmentDigest string `json:"assignment_digest"`
	Seed             string `json:"seed"`
}

type AssignmentSeed struct {
	AssignmentDigest string
	Seed             []byte
}

type MappingReveal struct {
	SchemaVersion            string                      `json:"schema_version"`
	CanonicalPolicy          string                      `json:"canonical_policy"`
	BundleDigest             string                      `json:"bundle_digest"`
	PrimaryCommitments       []ReviewCommitmentReference `json:"primary_commitments"`
	TieBreakAssignmentDigest string                      `json:"tie_break_assignment_digest,omitempty"`
	TieBreakBatchDigest      string                      `json:"tie_break_batch_digest,omitempty"`
	OrderingSeeds            []OrderingSeedReveal        `json:"ordering_seeds"`
	Mappings                 []MappingReference          `json:"mappings"`
	RevealedAt               string                      `json:"revealed_at"`
	RevealedBy               string                      `json:"revealed_by"`
	Digest                   string                      `json:"digest"`
}

type AdjudicationLedger struct {
	SchemaVersion            string                      `json:"schema_version"`
	CanonicalPolicy          string                      `json:"canonical_policy"`
	BundleDigest             string                      `json:"bundle_digest"`
	PrimaryCommitments       []ReviewCommitmentReference `json:"primary_commitments"`
	TieBreakAssignmentDigest string                      `json:"tie_break_assignment_digest,omitempty"`
	TieBreakBatchDigest      string                      `json:"tie_break_batch_digest,omitempty"`
	MappingRevealDigest      string                      `json:"mapping_reveal_digest"`
	RubricAmbiguityDigest    string                      `json:"rubric_ambiguity_digest"`
	BlindingAnalysisDigest   string                      `json:"blinding_analysis_digest"`
	ResolutionDigests        []string                    `json:"resolution_digests"`
	AgreementDigest          string                      `json:"agreement_digest"`
	UnresolvedPacketIDs      []string                    `json:"unresolved_packet_ids"`
	Status                   AdjudicationStatus          `json:"status"`
	CompletedAt              string                      `json:"completed_at"`
	Digest                   string                      `json:"digest"`
}

type AdjudicationResult struct {
	Ledger      AdjudicationLedger `json:"ledger"`
	Agreement   AgreementReport    `json:"agreement"`
	Resolutions []Resolution       `json:"resolutions"`
}

func BuildReviewBundle(plan Plan, qualification QualificationSet, handbook ReviewerHandbook, items []ReviewItem, dataRole ReviewDataRole, visibility ReviewVisibility, createdAt string) (ReviewBundle, error) {
	if err := plan.Validate(); err != nil {
		return ReviewBundle{}, err
	}
	if err := qualification.Validate(); err != nil {
		return ReviewBundle{}, err
	}
	if err := VerifyReviewerHandbook(handbook, qualification); err != nil {
		return ReviewBundle{}, err
	}
	if plan.RubricVersion != handbook.RubricVersion {
		return ReviewBundle{}, errors.New("review bundle plan and handbook rubric versions differ")
	}
	items = append([]ReviewItem(nil), items...)
	sort.Slice(items, func(left, right int) bool { return items[left].Packet.PacketID < items[right].Packet.PacketID })
	return SealReviewBundle(ReviewBundle{
		PlanDigest: plan.Digest, RubricVersion: qualification.RubricVersion, QualificationSetDigest: qualification.Digest,
		HandbookVersion: handbook.HandbookVersion, HandbookDigest: handbook.Digest, DataRole: dataRole, Visibility: visibility, CreatedAt: createdAt, Items: items,
	})
}

func SealReviewBundle(bundle ReviewBundle) (ReviewBundle, error) {
	bundle.SchemaVersion = ReviewBundleSchemaVersion
	bundle.CanonicalPolicy = CanonicalPolicy
	bundle.Digest = ""
	digest, err := reviewBundleDigest(bundle)
	if err != nil {
		return ReviewBundle{}, err
	}
	bundle.Digest = digest
	return bundle, bundle.Validate()
}

func (bundle ReviewBundle) Validate() error {
	if bundle.SchemaVersion != ReviewBundleSchemaVersion || bundle.CanonicalPolicy != CanonicalPolicy || !validDigest(bundle.PlanDigest) ||
		!validDigest(bundle.QualificationSetDigest) || !validDigest(bundle.HandbookDigest) || bundle.HandbookVersion != ReviewerHandbookVersion || missing(bundle.RubricVersion, bundle.CreatedAt) || len(bundle.Items) == 0 ||
		!validReviewDataRole(bundle.DataRole) || !validReviewVisibility(bundle.Visibility) {
		return errors.New("review bundle identity, governance, role, visibility, or items are invalid")
	}
	if _, err := time.Parse(time.RFC3339, bundle.CreatedAt); err != nil {
		return errors.New("review bundle timestamp must be RFC3339")
	}
	for index, item := range bundle.Items {
		if err := validateReviewItem(item, bundle); err != nil {
			return fmt.Errorf("review bundle item %d: %w", index, err)
		}
		if index > 0 && bundle.Items[index-1].Packet.PacketID >= item.Packet.PacketID {
			return errors.New("review bundle packets must be unique and sorted")
		}
	}
	expected, err := reviewBundleDigest(bundle)
	if err != nil || bundle.Digest != expected {
		return errors.New("review bundle digest is invalid")
	}
	return nil
}

func NewReviewerRecord(alias string, role ReviewerRole, consentedAt string, independence, authorship, privateContact bool, conflicts []string) (ReviewerRecord, error) {
	conflicts = append([]string(nil), conflicts...)
	sort.Strings(conflicts)
	return SealReviewerRecord(ReviewerRecord{
		AdjudicatorAlias: alias, Role: role, ConsentedAt: consentedAt, IndependenceAttested: independence,
		AuthorshipPolicyAccepted: authorship, ContactDetailsHeldPrivately: privateContact, ConflictsOfInterest: conflicts,
	})
}

func SealReviewerRecord(record ReviewerRecord) (ReviewerRecord, error) {
	record.SchemaVersion = ReviewerRecordSchemaVersion
	record.CanonicalPolicy = CanonicalPolicy
	record.Digest = ""
	digest, err := reviewerRecordDigest(record)
	if err != nil {
		return ReviewerRecord{}, err
	}
	record.Digest = digest
	return record, record.Validate()
}

func (record ReviewerRecord) Validate() error {
	if record.SchemaVersion != ReviewerRecordSchemaVersion || record.CanonicalPolicy != CanonicalPolicy || missing(record.AdjudicatorAlias, record.ConsentedAt) ||
		!validReviewerRole(record.Role) || !record.IndependenceAttested || !record.AuthorshipPolicyAccepted || !record.ContactDetailsHeldPrivately {
		return errors.New("reviewer consent, independence, authorship, contact, or role is invalid")
	}
	if _, err := time.Parse(time.RFC3339, record.ConsentedAt); err != nil {
		return errors.New("reviewer consent timestamp must be RFC3339")
	}
	if err := uniqueSorted("reviewer conflicts of interest", record.ConflictsOfInterest); err != nil {
		return err
	}
	expected, err := reviewerRecordDigest(record)
	if err != nil || record.Digest != expected {
		return errors.New("reviewer record digest is invalid")
	}
	return nil
}

func BuildPrimaryAssignment(bundle ReviewBundle, reviewer ReviewerRecord, qualification QualificationReport, slot int, seed []byte, assignedAt string) (ReviewAssignment, error) {
	if slot != 1 && slot != 2 {
		return ReviewAssignment{}, errors.New("primary review assignment requires reviewer slot one or two")
	}
	packetIDs := reviewBundlePacketIDs(bundle)
	return buildReviewAssignment(bundle, reviewer, qualification, AssignmentPrimary, slot, packetIDs, seed, assignedAt)
}

func BuildTieBreakAssignment(bundle ReviewBundle, reviewer ReviewerRecord, qualification QualificationReport, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, seed []byte, assignedAt string) (ReviewAssignment, error) {
	if err := validatePrimaryReviewPair(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch); err != nil {
		return ReviewAssignment{}, err
	}
	packetIDs := disagreementPacketIDs(leftBatch, rightBatch)
	if len(packetIDs) == 0 {
		return ReviewAssignment{}, errors.New("tie-break assignment requires at least one primary disagreement")
	}
	latestCommit, err := laterTimestamp(leftBatch.CommittedAt, rightBatch.CommittedAt)
	if err != nil {
		return ReviewAssignment{}, err
	}
	if err := requireStrictlyAfter("tie-break assignment", assignedAt, latestCommit); err != nil {
		return ReviewAssignment{}, err
	}
	return buildReviewAssignment(bundle, reviewer, qualification, AssignmentTieBreak, 3, packetIDs, seed, assignedAt)
}

func buildReviewAssignment(bundle ReviewBundle, reviewer ReviewerRecord, qualification QualificationReport, purpose AssignmentPurpose, slot int, packetIDs []string, seed []byte, assignedAt string) (ReviewAssignment, error) {
	if err := validateAssignmentInputs(bundle, reviewer, qualification, purpose, slot, seed, assignedAt); err != nil {
		return ReviewAssignment{}, err
	}
	packetIDs = orderPacketIDs(packetIDs, seed, bundle.Digest, reviewer.Digest)
	assignment := ReviewAssignment{
		BundleDigest: bundle.Digest, QualificationSetDigest: bundle.QualificationSetDigest, RubricVersion: bundle.RubricVersion, Purpose: purpose, ReviewerSlot: slot,
		Reviewer: reviewer, Qualification: qualification, OrderingProtocol: reviewAssignmentOrderingProtocol,
		OrderingSeedDigest: digestText(string(seed)), AssignedAt: assignedAt, PacketIDs: packetIDs,
	}
	return SealReviewAssignment(assignment)
}

func SealReviewAssignment(assignment ReviewAssignment) (ReviewAssignment, error) {
	assignment.SchemaVersion = ReviewAssignmentSchemaVersion
	assignment.CanonicalPolicy = CanonicalPolicy
	assignment.Digest = ""
	digest, err := reviewAssignmentDigest(assignment)
	if err != nil {
		return ReviewAssignment{}, err
	}
	assignment.Digest = digest
	return assignment, assignment.Validate()
}

func (assignment ReviewAssignment) Validate() error {
	if assignment.SchemaVersion != ReviewAssignmentSchemaVersion || assignment.CanonicalPolicy != CanonicalPolicy || !validDigest(assignment.BundleDigest) ||
		!validDigest(assignment.QualificationSetDigest) || missing(assignment.RubricVersion) || assignment.OrderingProtocol != reviewAssignmentOrderingProtocol || !validDigest(assignment.OrderingSeedDigest) ||
		missing(assignment.AssignedAt) || len(assignment.PacketIDs) == 0 || !validAssignmentPurpose(assignment.Purpose) {
		return errors.New("review assignment identity, ordering, purpose, or packet set is invalid")
	}
	if err := assignment.Reviewer.Validate(); err != nil {
		return err
	}
	if err := assignment.Qualification.Validate(); err != nil {
		return err
	}
	if err := validateAssignmentRole(assignment); err != nil {
		return err
	}
	if err := validateAssignmentQualification(assignment); err != nil {
		return err
	}
	if err := uniqueOpaquePacketIDs("review assignment packet IDs", assignment.PacketIDs); err != nil {
		return err
	}
	if err := requireNotBefore("review assignment", assignment.AssignedAt, assignment.Reviewer.ConsentedAt, assignment.Qualification.QualifiedAt); err != nil {
		return err
	}
	expected, err := reviewAssignmentDigest(assignment)
	if err != nil || assignment.Digest != expected {
		return errors.New("review assignment digest is invalid")
	}
	return nil
}

func BuildLabelBatch(assignment ReviewAssignment, labels []Label, committedAt string) (LabelBatch, error) {
	if err := assignment.Validate(); err != nil {
		return LabelBatch{}, err
	}
	labels = append([]Label(nil), labels...)
	sort.Slice(labels, func(left, right int) bool { return labels[left].PacketID < labels[right].PacketID })
	batch := LabelBatch{
		BundleDigest: assignment.BundleDigest, AssignmentDigest: assignment.Digest,
		AdjudicatorAlias: assignment.Reviewer.AdjudicatorAlias, ReviewerSlot: assignment.ReviewerSlot,
		QualificationDigest: assignment.Qualification.Digest, CommittedAt: committedAt, Labels: labels,
	}
	if err := validateLabelBatchAgainstAssignment(batch, assignment); err != nil {
		return LabelBatch{}, err
	}
	return SealLabelBatch(batch)
}

func SealLabelBatch(batch LabelBatch) (LabelBatch, error) {
	batch.SchemaVersion = LabelBatchSchemaVersion
	batch.CanonicalPolicy = CanonicalPolicy
	batch.Digest = ""
	digest, err := labelBatchDigest(batch)
	if err != nil {
		return LabelBatch{}, err
	}
	batch.Digest = digest
	return batch, batch.Validate()
}

func (batch LabelBatch) Validate() error {
	if batch.SchemaVersion != LabelBatchSchemaVersion || batch.CanonicalPolicy != CanonicalPolicy || !validDigest(batch.BundleDigest) ||
		!validDigest(batch.AssignmentDigest) || !validDigest(batch.QualificationDigest) || missing(batch.AdjudicatorAlias, batch.CommittedAt) ||
		batch.ReviewerSlot < 1 || batch.ReviewerSlot > 3 || len(batch.Labels) == 0 {
		return errors.New("label batch identity, reviewer, qualification, time, or labels are invalid")
	}
	committedAt, err := time.Parse(time.RFC3339, batch.CommittedAt)
	if err != nil {
		return errors.New("label batch commitment timestamp must be RFC3339")
	}
	for index, label := range batch.Labels {
		if err := validateBatchLabel(label, batch, committedAt); err != nil {
			return fmt.Errorf("label batch item %d: %w", index, err)
		}
		if index > 0 && batch.Labels[index-1].PacketID >= label.PacketID {
			return errors.New("label batch labels must be unique and sorted by packet")
		}
	}
	expected, err := labelBatchDigest(batch)
	if err != nil || batch.Digest != expected {
		return errors.New("label batch digest is invalid")
	}
	return nil
}

func BuildMappingReveal(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, tieAssignment *ReviewAssignment, tieBatch *LabelBatch, mappings []PrivateMapping, seeds []AssignmentSeed, revealedAt, revealedBy string) (MappingReveal, error) {
	if err := validateReviewSequence(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, tieAssignment, tieBatch); err != nil {
		return MappingReveal{}, err
	}
	references, err := mappingReferences(bundle, mappings)
	if err != nil {
		return MappingReveal{}, err
	}
	latestCommit, err := latestBatchCommit(leftBatch, rightBatch, tieBatch)
	if err != nil {
		return MappingReveal{}, err
	}
	if missing(revealedBy) {
		return MappingReveal{}, errors.New("mapping reveal requires an actor")
	}
	if err := requireStrictlyAfter("mapping reveal", revealedAt, latestCommit); err != nil {
		return MappingReveal{}, err
	}
	seedReveals, err := buildOrderingSeedReveals(bundle, leftAssignment, rightAssignment, tieAssignment, seeds)
	if err != nil {
		return MappingReveal{}, err
	}
	reveal := MappingReveal{
		BundleDigest: bundle.Digest, PrimaryCommitments: primaryCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch),
		OrderingSeeds: seedReveals, Mappings: references, RevealedAt: revealedAt, RevealedBy: revealedBy,
	}
	if tieAssignment != nil {
		reveal.TieBreakAssignmentDigest, reveal.TieBreakBatchDigest = tieAssignment.Digest, tieBatch.Digest
	}
	return SealMappingReveal(reveal)
}

func SealMappingReveal(reveal MappingReveal) (MappingReveal, error) {
	reveal.SchemaVersion = MappingRevealSchemaVersion
	reveal.CanonicalPolicy = CanonicalPolicy
	reveal.Digest = ""
	digest, err := mappingRevealDigest(reveal)
	if err != nil {
		return MappingReveal{}, err
	}
	reveal.Digest = digest
	return reveal, reveal.Validate()
}

func (reveal MappingReveal) Validate() error {
	if reveal.SchemaVersion != MappingRevealSchemaVersion || reveal.CanonicalPolicy != CanonicalPolicy || !validDigest(reveal.BundleDigest) ||
		missing(reveal.RevealedAt, reveal.RevealedBy) || len(reveal.Mappings) == 0 {
		return errors.New("mapping reveal identity, actor, time, or mappings are invalid")
	}
	if err := validateReviewCommitments(reveal.PrimaryCommitments); err != nil {
		return err
	}
	if (reveal.TieBreakAssignmentDigest == "") != (reveal.TieBreakBatchDigest == "") || reveal.TieBreakAssignmentDigest != "" &&
		(!validDigest(reveal.TieBreakAssignmentDigest) || !validDigest(reveal.TieBreakBatchDigest)) {
		return errors.New("mapping reveal tie-break references are inconsistent")
	}
	if _, err := time.Parse(time.RFC3339, reveal.RevealedAt); err != nil {
		return errors.New("mapping reveal timestamp must be RFC3339")
	}
	if err := validateMappingReferences(reveal.Mappings); err != nil {
		return err
	}
	if err := validateOrderingSeedReveals(reveal.OrderingSeeds, reveal.TieBreakAssignmentDigest != ""); err != nil {
		return err
	}
	expected, err := mappingRevealDigest(reveal)
	if err != nil || reveal.Digest != expected {
		return errors.New("mapping reveal digest is invalid")
	}
	return nil
}

func BuildAdjudicationLedger(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, tieAssignment *ReviewAssignment, tieBatch *LabelBatch, reveal MappingReveal, rubricAmbiguity RubricAmbiguityAnalysis, blinding BlindingAnalysis, completedAt, rule string, bootstrapIterations int, bootstrapSeed string) (AdjudicationResult, error) {
	if err := validateReviewSequence(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, tieAssignment, tieBatch); err != nil {
		return AdjudicationResult{}, err
	}
	if err := validateRevealAgainstSequence(reveal, bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, tieAssignment, tieBatch); err != nil {
		return AdjudicationResult{}, err
	}
	if err := validateRubricAmbiguityForLedger(rubricAmbiguity, bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, reveal); err != nil {
		return AdjudicationResult{}, err
	}
	if err := validateBlindingAnalysisForLedger(blinding, bundle, reveal); err != nil {
		return AdjudicationResult{}, err
	}
	if missing(rule) {
		return AdjudicationResult{}, errors.New("adjudication ledger requires the frozen resolution rule")
	}
	if err := requireNotBefore("adjudication completion", completedAt, reveal.RevealedAt, blinding.AnalyzedAt); err != nil {
		return AdjudicationResult{}, err
	}
	agreement, err := agreementFromBatches(bundle, leftBatch, rightBatch, bootstrapIterations, bootstrapSeed)
	if err != nil {
		return AdjudicationResult{}, err
	}
	resolutions, unresolved, err := resolveReviewBundle(bundle, leftBatch, rightBatch, tieBatch, completedAt, rule)
	if err != nil {
		return AdjudicationResult{}, err
	}
	ledger, err := buildLedger(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch, tieAssignment, tieBatch, reveal, rubricAmbiguity, blinding, agreement, resolutions, unresolved, completedAt)
	if err != nil {
		return AdjudicationResult{}, err
	}
	return AdjudicationResult{Ledger: ledger, Agreement: agreement, Resolutions: resolutions}, nil
}

func SealAdjudicationLedger(ledger AdjudicationLedger) (AdjudicationLedger, error) {
	ledger.SchemaVersion = AdjudicationLedgerSchemaVersion
	ledger.CanonicalPolicy = CanonicalPolicy
	ledger.Digest = ""
	digest, err := adjudicationLedgerDigest(ledger)
	if err != nil {
		return AdjudicationLedger{}, err
	}
	ledger.Digest = digest
	return ledger, ledger.Validate()
}

func (ledger AdjudicationLedger) Validate() error {
	if ledger.SchemaVersion != AdjudicationLedgerSchemaVersion || ledger.CanonicalPolicy != CanonicalPolicy || !validDigest(ledger.BundleDigest) ||
		!validDigest(ledger.MappingRevealDigest) || !validDigest(ledger.RubricAmbiguityDigest) || !validDigest(ledger.BlindingAnalysisDigest) || !validDigest(ledger.AgreementDigest) || missing(ledger.CompletedAt) ||
		!validAdjudicationStatus(ledger.Status) || len(ledger.ResolutionDigests) == 0 {
		return errors.New("adjudication ledger identity, evidence, status, or completion time is invalid")
	}
	if err := validateLedgerDigests(ledger); err != nil {
		return err
	}
	if err := uniqueSorted("adjudication unresolved packets", ledger.UnresolvedPacketIDs); err != nil {
		return err
	}
	if ledger.Status == AdjudicationComplete && len(ledger.UnresolvedPacketIDs) != 0 || ledger.Status == AdjudicationUnresolved && len(ledger.UnresolvedPacketIDs) == 0 {
		return errors.New("adjudication ledger status contradicts unresolved packets")
	}
	if _, err := time.Parse(time.RFC3339, ledger.CompletedAt); err != nil {
		return errors.New("adjudication ledger timestamp must be RFC3339")
	}
	expected, err := adjudicationLedgerDigest(ledger)
	if err != nil || ledger.Digest != expected {
		return errors.New("adjudication ledger digest is invalid")
	}
	return nil
}

func DecodeReviewItems(reader io.Reader) ([]ReviewItem, error) {
	var values []ReviewItem
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode review items: %w", err)
	}
	return values, nil
}

func DecodeReviewBundle(reader io.Reader) (ReviewBundle, error) {
	var value ReviewBundle
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewBundle{}, fmt.Errorf("decode review bundle: %w", err)
	}
	return value, value.Validate()
}

func DecodeReviewerRecord(reader io.Reader) (ReviewerRecord, error) {
	var value ReviewerRecord
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewerRecord{}, fmt.Errorf("decode reviewer record: %w", err)
	}
	return value, value.Validate()
}

func DecodeReviewAssignment(reader io.Reader) (ReviewAssignment, error) {
	var value ReviewAssignment
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewAssignment{}, fmt.Errorf("decode review assignment: %w", err)
	}
	return value, value.Validate()
}

func DecodeLabels(reader io.Reader) ([]Label, error) {
	var values []Label
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode outcome labels: %w", err)
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return nil, fmt.Errorf("outcome label %d: %w", index, err)
		}
	}
	return values, nil
}

func DecodeLabelBatch(reader io.Reader) (LabelBatch, error) {
	var value LabelBatch
	if err := decodeStrict(reader, &value); err != nil {
		return LabelBatch{}, fmt.Errorf("decode label batch: %w", err)
	}
	return value, value.Validate()
}

func DecodePrivateMappings(reader io.Reader) ([]PrivateMapping, error) {
	var values []PrivateMapping
	if err := decodeStrict(reader, &values); err != nil {
		return nil, fmt.Errorf("decode private outcome mappings: %w", err)
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return nil, fmt.Errorf("private outcome mapping %d: %w", index, err)
		}
	}
	return values, nil
}

func DecodeMappingReveal(reader io.Reader) (MappingReveal, error) {
	var value MappingReveal
	if err := decodeStrict(reader, &value); err != nil {
		return MappingReveal{}, fmt.Errorf("decode mapping reveal: %w", err)
	}
	return value, value.Validate()
}

func DecodeAdjudicationLedger(reader io.Reader) (AdjudicationLedger, error) {
	var value AdjudicationLedger
	if err := decodeStrict(reader, &value); err != nil {
		return AdjudicationLedger{}, fmt.Errorf("decode adjudication ledger: %w", err)
	}
	return value, value.Validate()
}

func validateReviewItem(item ReviewItem, bundle ReviewBundle) error {
	if !validTaskGroupID(item.TaskGroupID) {
		return errors.New("review item task group must be opaque")
	}
	if err := item.Packet.Validate(); err != nil {
		return err
	}
	if item.Packet.PlanDigest != bundle.PlanDigest {
		return errors.New("review packet does not bind the bundle plan")
	}
	if bundle.Visibility == ReviewVisibilityPublic && !item.Packet.PublicReleasable {
		return errors.New("public review bundle contains a restricted packet")
	}
	return nil
}

func validateAssignmentInputs(bundle ReviewBundle, reviewer ReviewerRecord, qualification QualificationReport, purpose AssignmentPurpose, slot int, seed []byte, assignedAt string) error {
	if err := bundle.Validate(); err != nil {
		return err
	}
	if err := reviewer.Validate(); err != nil {
		return err
	}
	if err := qualification.Validate(); err != nil {
		return err
	}
	if len(seed) != 32 || missing(assignedAt) {
		return errors.New("review assignment requires a 32-byte secret seed and assignment time")
	}
	assignment := ReviewAssignment{Purpose: purpose, ReviewerSlot: slot, Reviewer: reviewer}
	if err := validateAssignmentRole(assignment); err != nil {
		return err
	}
	if !qualification.Qualified || qualification.AdjudicatorAlias != reviewer.AdjudicatorAlias || qualification.QualificationSet != bundle.QualificationSetDigest {
		return errors.New("review assignment requires the bundle qualification and a passing reviewer-specific report")
	}
	if len(reviewer.ConflictsOfInterest) != 0 {
		return errors.New("review assignment requires a reviewer with no declared conflict of interest")
	}
	return requireNotBefore("review assignment", assignedAt, reviewer.ConsentedAt, qualification.QualifiedAt)
}

func validateAssignmentRole(assignment ReviewAssignment) error {
	if assignment.Purpose == AssignmentPrimary && (assignment.Reviewer.Role != ReviewerRolePrimary || assignment.ReviewerSlot != 1 && assignment.ReviewerSlot != 2) {
		return errors.New("primary assignment requires a primary reviewer in slot one or two")
	}
	if assignment.Purpose == AssignmentTieBreak && (assignment.Reviewer.Role != ReviewerRoleTieBreak || assignment.ReviewerSlot != 3) {
		return errors.New("tie-break assignment requires a tie-break reviewer in slot three")
	}
	return nil
}

func validateAssignmentQualification(assignment ReviewAssignment) error {
	if !assignment.Qualification.Qualified || assignment.Qualification.AdjudicatorAlias != assignment.Reviewer.AdjudicatorAlias ||
		assignment.Qualification.QualificationSet != assignment.QualificationSetDigest || assignment.Qualification.RubricVersion != assignment.RubricVersion {
		return errors.New("review assignment qualification does not bind its reviewer or governed set")
	}
	return nil
}

func validateLabelBatchAgainstAssignment(batch LabelBatch, assignment ReviewAssignment) error {
	if batch.BundleDigest != assignment.BundleDigest || batch.AssignmentDigest != assignment.Digest || batch.AdjudicatorAlias != assignment.Reviewer.AdjudicatorAlias ||
		batch.ReviewerSlot != assignment.ReviewerSlot || batch.QualificationDigest != assignment.Qualification.Digest {
		return errors.New("label batch does not bind its assignment, reviewer, or qualification")
	}
	if err := exactPacketCoverage(assignment.PacketIDs, labelPacketIDs(batch.Labels)); err != nil {
		return err
	}
	for _, label := range batch.Labels {
		if label.RubricVersion != assignment.RubricVersion || len(label.ConflictsOfInterest) != 0 {
			return errors.New("label batch contains a rubric mismatch or reviewer conflict")
		}
		if err := requireNotBefore("label submission", label.SubmittedAt, assignment.AssignedAt); err != nil {
			return err
		}
	}
	return requireNotBefore("label batch commitment", batch.CommittedAt, assignment.AssignedAt)
}

func validateBatchLabel(label Label, batch LabelBatch, committedAt time.Time) error {
	if err := label.Validate(); err != nil {
		return err
	}
	if label.AdjudicatorAlias != batch.AdjudicatorAlias || label.ReviewerSlot != batch.ReviewerSlot || label.QualificationDigest != batch.QualificationDigest {
		return errors.New("label does not bind the batch reviewer, slot, or qualification")
	}
	submittedAt, err := time.Parse(time.RFC3339, label.SubmittedAt)
	if err != nil || submittedAt.After(committedAt) {
		return errors.New("label submission occurs after its batch commitment")
	}
	return nil
}

func validatePrimaryReviewPair(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch) error {
	if err := bundle.Validate(); err != nil {
		return err
	}
	if err := validateAssignmentBatch(bundle, leftAssignment, leftBatch); err != nil {
		return err
	}
	if err := validateAssignmentBatch(bundle, rightAssignment, rightBatch); err != nil {
		return err
	}
	if leftAssignment.Purpose != AssignmentPrimary || rightAssignment.Purpose != AssignmentPrimary || leftAssignment.ReviewerSlot == rightAssignment.ReviewerSlot ||
		leftAssignment.Reviewer.AdjudicatorAlias == rightAssignment.Reviewer.AdjudicatorAlias {
		return errors.New("primary review requires independent reviewers in distinct slots")
	}
	if err := exactPacketCoverage(reviewBundlePacketIDs(bundle), leftAssignment.PacketIDs); err != nil {
		return err
	}
	return exactPacketCoverage(reviewBundlePacketIDs(bundle), rightAssignment.PacketIDs)
}

func validateAssignmentBatch(bundle ReviewBundle, assignment ReviewAssignment, batch LabelBatch) error {
	if err := assignment.Validate(); err != nil {
		return err
	}
	if err := batch.Validate(); err != nil {
		return err
	}
	if assignment.BundleDigest != bundle.Digest {
		return errors.New("review assignment does not bind the bundle")
	}
	return validateLabelBatchAgainstAssignment(batch, assignment)
}

func validateReviewSequence(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, tieAssignment *ReviewAssignment, tieBatch *LabelBatch) error {
	if err := validatePrimaryReviewPair(bundle, leftAssignment, leftBatch, rightAssignment, rightBatch); err != nil {
		return err
	}
	if (tieAssignment == nil) != (tieBatch == nil) {
		return errors.New("tie-break assignment and batch must be supplied together")
	}
	if tieAssignment == nil {
		return nil
	}
	if err := validateAssignmentBatch(bundle, *tieAssignment, *tieBatch); err != nil {
		return err
	}
	if tieAssignment.Purpose != AssignmentTieBreak || tieAssignment.ReviewerSlot != 3 ||
		tieAssignment.Reviewer.AdjudicatorAlias == leftAssignment.Reviewer.AdjudicatorAlias || tieAssignment.Reviewer.AdjudicatorAlias == rightAssignment.Reviewer.AdjudicatorAlias {
		return errors.New("tie-break review is not independent from primary reviewers")
	}
	return exactPacketCoverage(disagreementPacketIDs(leftBatch, rightBatch), tieAssignment.PacketIDs)
}

func mappingReferences(bundle ReviewBundle, mappings []PrivateMapping) ([]MappingReference, error) {
	references := make([]MappingReference, 0, len(mappings))
	for _, mapping := range mappings {
		if err := mapping.Validate(); err != nil {
			return nil, err
		}
		references = append(references, MappingReference{PacketID: mapping.PacketID, MappingDigest: mapping.Digest})
	}
	sort.Slice(references, func(left, right int) bool { return references[left].PacketID < references[right].PacketID })
	if err := exactPacketCoverage(reviewBundlePacketIDs(bundle), mappingReferencePacketIDs(references)); err != nil {
		return nil, err
	}
	return references, nil
}

func validateMappingReferences(references []MappingReference) error {
	for index, reference := range references {
		if !validOpaquePacketID(reference.PacketID) || !validDigest(reference.MappingDigest) || index > 0 && references[index-1].PacketID >= reference.PacketID {
			return errors.New("mapping references must be valid, unique, and sorted")
		}
	}
	return nil
}

func buildOrderingSeedReveals(bundle ReviewBundle, left, right ReviewAssignment, tie *ReviewAssignment, seeds []AssignmentSeed) ([]OrderingSeedReveal, error) {
	assignments := []ReviewAssignment{left, right}
	if tie != nil {
		assignments = append(assignments, *tie)
	}
	if len(seeds) != len(assignments) {
		return nil, errors.New("mapping reveal requires exactly one ordering seed per assignment")
	}
	seedByAssignment := make(map[string][]byte, len(seeds))
	for _, item := range seeds {
		if _, duplicate := seedByAssignment[item.AssignmentDigest]; duplicate || len(item.Seed) != 32 {
			return nil, errors.New("mapping reveal ordering seeds must be unique 32-byte assignment bindings")
		}
		seedByAssignment[item.AssignmentDigest] = item.Seed
	}
	reveals := make([]OrderingSeedReveal, 0, len(assignments))
	for _, assignment := range assignments {
		seed, exists := seedByAssignment[assignment.Digest]
		if !exists || assignment.OrderingSeedDigest != digestText(string(seed)) ||
			!equalStrings(assignment.PacketIDs, orderPacketIDs(assignment.PacketIDs, seed, bundle.Digest, assignment.Reviewer.Digest)) {
			return nil, errors.New("mapping reveal ordering seed does not reproduce its assignment")
		}
		reveals = append(reveals, OrderingSeedReveal{AssignmentDigest: assignment.Digest, Seed: hex.EncodeToString(seed)})
	}
	sort.Slice(reveals, func(left, right int) bool { return reveals[left].AssignmentDigest < reveals[right].AssignmentDigest })
	return reveals, nil
}

func validateOrderingSeedReveals(reveals []OrderingSeedReveal, hasTieBreak bool) error {
	expected := 2
	if hasTieBreak {
		expected = 3
	}
	if len(reveals) != expected {
		return errors.New("mapping reveal ordering seed count does not match its assignments")
	}
	for index, reveal := range reveals {
		seed, err := hex.DecodeString(reveal.Seed)
		if err != nil || len(seed) != 32 || !validDigest(reveal.AssignmentDigest) || index > 0 && reveals[index-1].AssignmentDigest >= reveal.AssignmentDigest {
			return errors.New("mapping reveal ordering seeds must be valid, unique, and sorted")
		}
	}
	return nil
}

func verifyOrderingSeedReveals(bundle ReviewBundle, reveals []OrderingSeedReveal, left, right ReviewAssignment, tie *ReviewAssignment) error {
	seeds := make([]AssignmentSeed, 0, len(reveals))
	for _, reveal := range reveals {
		seed, err := hex.DecodeString(reveal.Seed)
		if err != nil {
			return errors.New("mapping reveal contains an invalid ordering seed")
		}
		seeds = append(seeds, AssignmentSeed{AssignmentDigest: reveal.AssignmentDigest, Seed: seed})
	}
	expected, err := buildOrderingSeedReveals(bundle, left, right, tie, seeds)
	if err != nil || !equalOrderingSeedReveals(reveals, expected) {
		return errors.New("mapping reveal does not reproduce assignment randomization")
	}
	return nil
}

func primaryCommitments(leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch) []ReviewCommitmentReference {
	values := []ReviewCommitmentReference{
		{ReviewerSlot: leftAssignment.ReviewerSlot, AssignmentDigest: leftAssignment.Digest, BatchDigest: leftBatch.Digest},
		{ReviewerSlot: rightAssignment.ReviewerSlot, AssignmentDigest: rightAssignment.Digest, BatchDigest: rightBatch.Digest},
	}
	sort.Slice(values, func(left, right int) bool { return values[left].ReviewerSlot < values[right].ReviewerSlot })
	return values
}

func validateReviewCommitments(values []ReviewCommitmentReference) error {
	if len(values) != 2 {
		return errors.New("review evidence requires exactly two primary commitments")
	}
	for index, value := range values {
		if value.ReviewerSlot != index+1 || !validDigest(value.AssignmentDigest) || !validDigest(value.BatchDigest) {
			return errors.New("primary review commitments must bind slots one and two in order")
		}
	}
	if values[0].AssignmentDigest == values[1].AssignmentDigest || values[0].BatchDigest == values[1].BatchDigest {
		return errors.New("primary review commitments must be distinct")
	}
	return nil
}

func equalReviewCommitments(left, right []ReviewCommitmentReference) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalOrderingSeedReveals(left, right []OrderingSeedReveal) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateRevealAgainstSequence(reveal MappingReveal, bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, tieAssignment *ReviewAssignment, tieBatch *LabelBatch) error {
	if err := reveal.Validate(); err != nil {
		return err
	}
	if reveal.BundleDigest != bundle.Digest || !equalReviewCommitments(reveal.PrimaryCommitments, primaryCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch)) {
		return errors.New("mapping reveal does not bind the review sequence")
	}
	if err := exactPacketCoverage(reviewBundlePacketIDs(bundle), mappingReferencePacketIDs(reveal.Mappings)); err != nil {
		return err
	}
	if err := verifyOrderingSeedReveals(bundle, reveal.OrderingSeeds, leftAssignment, rightAssignment, tieAssignment); err != nil {
		return err
	}
	if tieAssignment == nil {
		if reveal.TieBreakAssignmentDigest != "" || reveal.TieBreakBatchDigest != "" {
			return errors.New("mapping reveal contains an unexpected tie-break")
		}
		return nil
	}
	if reveal.TieBreakAssignmentDigest != tieAssignment.Digest || reveal.TieBreakBatchDigest != tieBatch.Digest {
		return errors.New("mapping reveal does not bind the tie-break sequence")
	}
	return nil
}

func agreementFromBatches(bundle ReviewBundle, left, right LabelBatch, iterations int, seed string) (AgreementReport, error) {
	leftLabels, rightLabels := labelsByPacket(left.Labels), labelsByPacket(right.Labels)
	pairs := make([]AgreementPair, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		pairs = append(pairs, AgreementPair{PacketID: item.Packet.PacketID, TaskGroupID: item.TaskGroupID, Left: leftLabels[item.Packet.PacketID].PrimaryOutcome, Right: rightLabels[item.Packet.PacketID].PrimaryOutcome})
	}
	return ComputeAgreement(pairs, iterations, seed)
}

func resolveReviewBundle(bundle ReviewBundle, left, right LabelBatch, tie *LabelBatch, resolvedAt, rule string) ([]Resolution, []string, error) {
	leftLabels, rightLabels := labelsByPacket(left.Labels), labelsByPacket(right.Labels)
	tieLabels := map[string]Label{}
	if tie != nil {
		tieLabels = labelsByPacket(tie.Labels)
	}
	resolutions := make([]Resolution, 0, len(bundle.Items))
	var unresolved []string
	for _, item := range bundle.Items {
		packetID := item.Packet.PacketID
		var tieLabel *Label
		if label, exists := tieLabels[packetID]; exists {
			tieLabel = &label
		}
		resolution, err := ResolveLabels([]Label{leftLabels[packetID], rightLabels[packetID]}, tieLabel, resolvedAt, rule)
		if err != nil {
			return nil, nil, err
		}
		if resolution.AgreementState == "unresolved_disagreement" {
			unresolved = append(unresolved, packetID)
		}
		resolutions = append(resolutions, resolution)
	}
	sort.Slice(resolutions, func(left, right int) bool { return resolutions[left].PacketID < resolutions[right].PacketID })
	sort.Strings(unresolved)
	return resolutions, unresolved, nil
}

func buildLedger(bundle ReviewBundle, leftAssignment ReviewAssignment, leftBatch LabelBatch, rightAssignment ReviewAssignment, rightBatch LabelBatch, tieAssignment *ReviewAssignment, tieBatch *LabelBatch, reveal MappingReveal, rubricAmbiguity RubricAmbiguityAnalysis, blinding BlindingAnalysis, agreement AgreementReport, resolutions []Resolution, unresolved []string, completedAt string) (AdjudicationLedger, error) {
	resolutionDigests := make([]string, 0, len(resolutions))
	for _, resolution := range resolutions {
		resolutionDigests = append(resolutionDigests, resolution.Digest)
	}
	sort.Strings(resolutionDigests)
	status := AdjudicationComplete
	if len(unresolved) > 0 {
		status = AdjudicationUnresolved
	}
	ledger := AdjudicationLedger{
		BundleDigest: bundle.Digest, PrimaryCommitments: primaryCommitments(leftAssignment, leftBatch, rightAssignment, rightBatch),
		MappingRevealDigest: reveal.Digest, RubricAmbiguityDigest: rubricAmbiguity.Digest, BlindingAnalysisDigest: blinding.Digest,
		ResolutionDigests: resolutionDigests, AgreementDigest: agreement.Digest,
		UnresolvedPacketIDs: unresolved, Status: status, CompletedAt: completedAt,
	}
	if tieAssignment != nil {
		ledger.TieBreakAssignmentDigest, ledger.TieBreakBatchDigest = tieAssignment.Digest, tieBatch.Digest
	}
	return SealAdjudicationLedger(ledger)
}

func validateLedgerDigests(ledger AdjudicationLedger) error {
	if err := validateReviewCommitments(ledger.PrimaryCommitments); err != nil {
		return err
	}
	if err := uniqueSortedDigests("adjudication resolutions", ledger.ResolutionDigests); err != nil {
		return err
	}
	if (ledger.TieBreakAssignmentDigest == "") != (ledger.TieBreakBatchDigest == "") || ledger.TieBreakAssignmentDigest != "" &&
		(!validDigest(ledger.TieBreakAssignmentDigest) || !validDigest(ledger.TieBreakBatchDigest)) {
		return errors.New("adjudication ledger tie-break references are inconsistent")
	}
	return nil
}

func orderPacketIDs(packetIDs []string, seed []byte, bundleDigest, reviewerDigest string) []string {
	packetIDs = append([]string(nil), packetIDs...)
	sort.Slice(packetIDs, func(left, right int) bool {
		leftRank := keyedDigest(seed, reviewAssignmentOrderingProtocol, bundleDigest, reviewerDigest, packetIDs[left])
		rightRank := keyedDigest(seed, reviewAssignmentOrderingProtocol, bundleDigest, reviewerDigest, packetIDs[right])
		if leftRank == rightRank {
			return packetIDs[left] < packetIDs[right]
		}
		return leftRank < rightRank
	})
	return packetIDs
}

func reviewBundlePacketIDs(bundle ReviewBundle) []string {
	values := make([]string, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		values = append(values, item.Packet.PacketID)
	}
	return values
}

func labelPacketIDs(labels []Label) []string {
	values := make([]string, 0, len(labels))
	for _, label := range labels {
		values = append(values, label.PacketID)
	}
	return values
}

func mappingReferencePacketIDs(references []MappingReference) []string {
	values := make([]string, 0, len(references))
	for _, reference := range references {
		values = append(values, reference.PacketID)
	}
	return values
}

func labelsByPacket(labels []Label) map[string]Label {
	values := make(map[string]Label, len(labels))
	for _, label := range labels {
		values[label.PacketID] = label
	}
	return values
}

func disagreementPacketIDs(left, right LabelBatch) []string {
	leftLabels, rightLabels := labelsByPacket(left.Labels), labelsByPacket(right.Labels)
	var packetIDs []string
	for packetID, leftLabel := range leftLabels {
		if rightLabel, exists := rightLabels[packetID]; exists && leftLabel.PrimaryOutcome != rightLabel.PrimaryOutcome {
			packetIDs = append(packetIDs, packetID)
		}
	}
	sort.Strings(packetIDs)
	return packetIDs
}

func exactPacketCoverage(expected, actual []string) error {
	expected, actual = append([]string(nil), expected...), append([]string(nil), actual...)
	sort.Strings(expected)
	sort.Strings(actual)
	if !equalStrings(expected, actual) {
		return errors.New("review packet coverage is incomplete or contains unexpected packets")
	}
	return nil
}

func uniqueOpaquePacketIDs(name string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaquePacketID(value) {
			return fmt.Errorf("%s contains an invalid packet ID", name)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s contains a duplicate packet ID", name)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func latestBatchCommit(left, right LabelBatch, tie *LabelBatch) (string, error) {
	latest, err := laterTimestamp(left.CommittedAt, right.CommittedAt)
	if err != nil || tie == nil {
		return latest, err
	}
	return laterTimestamp(latest, tie.CommittedAt)
}

func laterTimestamp(left, right string) (string, error) {
	leftTime, err := time.Parse(time.RFC3339, left)
	if err != nil {
		return "", err
	}
	rightTime, err := time.Parse(time.RFC3339, right)
	if err != nil {
		return "", err
	}
	if rightTime.After(leftTime) {
		return right, nil
	}
	return left, nil
}

func requireNotBefore(name, value string, predecessors ...string) error {
	current, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fmt.Errorf("%s timestamp must be RFC3339", name)
	}
	for _, predecessor := range predecessors {
		previous, parseErr := time.Parse(time.RFC3339, predecessor)
		if parseErr != nil || current.Before(previous) {
			return fmt.Errorf("%s occurs before its required predecessor", name)
		}
	}
	return nil
}

func requireStrictlyAfter(name, value, predecessor string) error {
	current, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fmt.Errorf("%s timestamp must be RFC3339", name)
	}
	previous, err := time.Parse(time.RFC3339, predecessor)
	if err != nil || !current.After(previous) {
		return fmt.Errorf("%s must occur after its required predecessor", name)
	}
	return nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validReviewerRole(value ReviewerRole) bool {
	return value == ReviewerRolePrimary || value == ReviewerRoleTieBreak
}

func validAssignmentPurpose(value AssignmentPurpose) bool {
	return value == AssignmentPrimary || value == AssignmentTieBreak
}

func validReviewVisibility(value ReviewVisibility) bool {
	return value == ReviewVisibilityPublic || value == ReviewVisibilityRestricted
}

func validReviewDataRole(value ReviewDataRole) bool {
	return value == ReviewDataDevelopment || value == ReviewDataCalibration || value == ReviewDataTest
}

func validAdjudicationStatus(value AdjudicationStatus) bool {
	return value == AdjudicationComplete || value == AdjudicationUnresolved
}

func validTaskGroupID(value string) bool {
	return len(value) == len("group-")+64 && value[:len("group-")] == "group-" && validDigest(value[len("group-"):])
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

func labelBatchDigest(value LabelBatch) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func mappingRevealDigest(value MappingReveal) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func adjudicationLedgerDigest(value AdjudicationLedger) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
