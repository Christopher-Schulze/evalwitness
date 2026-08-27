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
	PairJudgmentSchemaVersionV1  = "evalwitness.relation-pair-judgment.v1"
	PairJudgmentSchemaVersionV2  = "evalwitness.relation-pair-judgment.v2"
	PairJudgmentSchemaVersionV3  = "evalwitness.relation-pair-judgment.v3"
	PairJudgmentSchemaVersion    = PairJudgmentSchemaVersionV1
	JudgmentBatchSchemaVersionV1 = "evalwitness.relation-judgment-batch.v1"
	JudgmentBatchSchemaVersionV2 = "evalwitness.relation-judgment-batch.v2"
	JudgmentBatchSchemaVersionV3 = "evalwitness.relation-judgment-batch.v3"
	JudgmentBatchSchemaVersion   = JudgmentBatchSchemaVersionV1
)

type PairJudgmentDraft struct {
	PacketID       string                   `json:"packet_id"`
	Observations   []VisibleAxisObservation `json:"observations"`
	ReasonCodes    []ReasonCode             `json:"reason_codes"`
	SubmittedAt    string                   `json:"submitted_at"`
	RevisionReason string                   `json:"revision_reason"`
}

type PairJudgment struct {
	SchemaVersion        string                   `json:"schema_version"`
	CanonicalPolicy      string                   `json:"canonical_policy"`
	ProtocolVersion      string                   `json:"protocol_version"`
	Objective            ReviewObjective          `json:"review_objective"`
	PlanDigest           string                   `json:"plan_digest"`
	BundleDigest         string                   `json:"bundle_digest"`
	AssignmentDigest     string                   `json:"assignment_digest"`
	PacketID             string                   `json:"packet_id"`
	PacketDigest         string                   `json:"packet_digest"`
	ReviewerAlias        string                   `json:"reviewer_alias"`
	ReviewerSlot         int                      `json:"reviewer_slot"`
	QualificationDigest  string                   `json:"qualification_digest"`
	RubricVersion        string                   `json:"rubric_version"`
	Revision             int                      `json:"revision"`
	ParentDigest         string                   `json:"parent_digest,omitempty"`
	RevisionReason       string                   `json:"revision_reason"`
	SubmittedAt          string                   `json:"submitted_at"`
	Observations         []VisibleAxisObservation `json:"observations"`
	ReasonCodes          []ReasonCode             `json:"reason_codes"`
	ExternalActionStatus ExternalActionStatus     `json:"external_action_status"`
	Digest               string                   `json:"digest"`
}

type JudgmentBatch struct {
	SchemaVersion        string               `json:"schema_version"`
	CanonicalPolicy      string               `json:"canonical_policy"`
	ProtocolVersion      string               `json:"protocol_version"`
	Objective            ReviewObjective      `json:"review_objective"`
	PlanDigest           string               `json:"plan_digest"`
	BundleDigest         string               `json:"bundle_digest"`
	AssignmentDigest     string               `json:"assignment_digest"`
	ReviewerAlias        string               `json:"reviewer_alias"`
	ReviewerSlot         int                  `json:"reviewer_slot"`
	QualificationDigest  string               `json:"qualification_digest"`
	RubricVersion        string               `json:"rubric_version"`
	CoverageStatus       string               `json:"coverage_status"`
	CommittedAt          string               `json:"committed_at"`
	Judgments            []PairJudgment       `json:"judgments"`
	ExternalActionStatus ExternalActionStatus `json:"external_action_status"`
	Digest               string               `json:"digest"`
}

func BuildPairJudgment(bundle ReviewBundle, assignment ReviewAssignment, draft PairJudgmentDraft, parent *PairJudgment) (PairJudgment, error) {
	if err := VerifyReviewAssignment(assignment, bundle); err != nil {
		return PairJudgment{}, err
	}
	packet, exists := reviewPacketByID(bundle, draft.PacketID)
	if !exists || !slices.Contains(assignment.PacketIDs, draft.PacketID) {
		return PairJudgment{}, errors.New("relation pair judgment packet is not assigned to this reviewer")
	}
	if err := validatePairJudgmentDraft(draft, assignment.AssignedAt); err != nil {
		return PairJudgment{}, err
	}
	revision, parentDigest := 1, ""
	if parent != nil {
		if err := validatePairJudgmentParent(*parent, bundle, assignment, packet, draft.SubmittedAt); err != nil {
			return PairJudgment{}, err
		}
		revision, parentDigest = parent.Revision+1, parent.Digest
	}
	judgment := PairJudgment{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: bundle.PlanDigest,
		BundleDigest: bundle.Digest, AssignmentDigest: assignment.Digest, PacketID: packet.PacketID, PacketDigest: packet.Digest,
		ReviewerAlias: assignment.Reviewer.ReviewerAlias, ReviewerSlot: assignment.ReviewerSlot,
		QualificationDigest: assignment.Qualification.Digest, RubricVersion: assignment.RubricVersion, Revision: revision,
		ParentDigest: parentDigest, RevisionReason: strings.TrimSpace(draft.RevisionReason), SubmittedAt: draft.SubmittedAt,
		Observations: append([]VisibleAxisObservation(nil), draft.Observations...), ReasonCodes: append([]ReasonCode(nil), draft.ReasonCodes...),
		ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealPairJudgment(judgment)
}

func SealPairJudgment(judgment PairJudgment) (PairJudgment, error) {
	schemaVersion, err := schemaVersionForProtocol(judgment.ProtocolVersion, PairJudgmentSchemaVersionV1, PairJudgmentSchemaVersionV2, PairJudgmentSchemaVersionV3)
	if err != nil {
		return PairJudgment{}, err
	}
	judgment.SchemaVersion, judgment.CanonicalPolicy, judgment.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := pairJudgmentDigest(judgment)
	if err != nil {
		return PairJudgment{}, err
	}
	judgment.Digest = digest
	return judgment, judgment.Validate()
}

func (judgment PairJudgment) Validate() error {
	if !validVersionedIdentity(judgment.SchemaVersion, judgment.ProtocolVersion, PairJudgmentSchemaVersionV1, PairJudgmentSchemaVersionV2, PairJudgmentSchemaVersionV3) || judgment.CanonicalPolicy != CanonicalPolicy ||
		judgment.Objective != ReviewObjectiveControlledRelation || !validDigest(judgment.PlanDigest) || !validDigest(judgment.BundleDigest) ||
		!validDigest(judgment.AssignmentDigest) || !validOpaqueID(judgment.PacketID, "relation-packet-") || !validDigest(judgment.PacketDigest) ||
		strings.TrimSpace(judgment.ReviewerAlias) == "" || judgment.ReviewerSlot < 1 || judgment.ReviewerSlot > 3 || !validDigest(judgment.QualificationDigest) ||
		!validRubricVersion(judgment.ProtocolVersion, judgment.RubricVersion) || judgment.Revision < 1 || strings.TrimSpace(judgment.RevisionReason) == "" ||
		len(judgment.RevisionReason) > 512 || judgment.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation pair judgment identity, objective, reviewer, revision, or authorization boundary is invalid")
	}
	if judgment.Revision == 1 && judgment.ParentDigest != "" || judgment.Revision > 1 && !validDigest(judgment.ParentDigest) {
		return errors.New("relation pair judgment revision chain is invalid")
	}
	if _, err := time.Parse(time.RFC3339, judgment.SubmittedAt); err != nil {
		return errors.New("relation pair judgment submission time must be RFC3339")
	}
	if err := validateVisibleObservations(judgment.Observations); err != nil {
		return err
	}
	if err := validateReasonCodes(judgment.ReasonCodes); err != nil {
		return err
	}
	if err := validateObservationReasonConsistency(judgment.Observations, judgment.ReasonCodes); err != nil {
		return err
	}
	expected, err := pairJudgmentDigest(judgment)
	if err != nil || judgment.Digest != expected {
		return errors.New("relation pair judgment digest is invalid")
	}
	return nil
}

func BuildJudgmentBatch(bundle ReviewBundle, assignment ReviewAssignment, judgments []PairJudgment, committedAt string) (JudgmentBatch, error) {
	if err := VerifyReviewAssignment(assignment, bundle); err != nil {
		return JudgmentBatch{}, err
	}
	judgments = append([]PairJudgment(nil), judgments...)
	sort.Slice(judgments, func(left, right int) bool { return judgments[left].PacketID < judgments[right].PacketID })
	batch := JudgmentBatch{
		ProtocolVersion: bundle.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: bundle.PlanDigest,
		BundleDigest: bundle.Digest, AssignmentDigest: assignment.Digest, ReviewerAlias: assignment.Reviewer.ReviewerAlias,
		ReviewerSlot: assignment.ReviewerSlot, QualificationDigest: assignment.Qualification.Digest, RubricVersion: assignment.RubricVersion,
		CoverageStatus: "complete", CommittedAt: committedAt, Judgments: judgments, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	if err := validateJudgmentBatchAgainstAssignment(batch, assignment, bundle); err != nil {
		return JudgmentBatch{}, err
	}
	return SealJudgmentBatch(batch)
}

func SealJudgmentBatch(batch JudgmentBatch) (JudgmentBatch, error) {
	schemaVersion, err := schemaVersionForProtocol(batch.ProtocolVersion, JudgmentBatchSchemaVersionV1, JudgmentBatchSchemaVersionV2, JudgmentBatchSchemaVersionV3)
	if err != nil {
		return JudgmentBatch{}, err
	}
	batch.SchemaVersion, batch.CanonicalPolicy, batch.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := judgmentBatchDigest(batch)
	if err != nil {
		return JudgmentBatch{}, err
	}
	batch.Digest = digest
	return batch, batch.Validate()
}

func (batch JudgmentBatch) Validate() error {
	if !validVersionedIdentity(batch.SchemaVersion, batch.ProtocolVersion, JudgmentBatchSchemaVersionV1, JudgmentBatchSchemaVersionV2, JudgmentBatchSchemaVersionV3) || batch.CanonicalPolicy != CanonicalPolicy ||
		batch.Objective != ReviewObjectiveControlledRelation || !validDigest(batch.PlanDigest) || !validDigest(batch.BundleDigest) ||
		!validDigest(batch.AssignmentDigest) || strings.TrimSpace(batch.ReviewerAlias) == "" || batch.ReviewerSlot < 1 || batch.ReviewerSlot > 3 ||
		!validDigest(batch.QualificationDigest) || !validRubricVersion(batch.ProtocolVersion, batch.RubricVersion) || batch.CoverageStatus != "complete" ||
		len(batch.Judgments) == 0 || batch.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation judgment batch identity, objective, reviewer, coverage, or authorization boundary is invalid")
	}
	committedAt, err := time.Parse(time.RFC3339, batch.CommittedAt)
	if err != nil {
		return errors.New("relation judgment batch commitment time must be RFC3339")
	}
	for index, judgment := range batch.Judgments {
		if err := judgment.Validate(); err != nil {
			return fmt.Errorf("relation judgment batch item %d: %w", index, err)
		}
		if index > 0 && batch.Judgments[index-1].PacketID >= judgment.PacketID {
			return errors.New("relation judgment batch items must be unique and sorted by packet")
		}
		if judgment.ProtocolVersion != batch.ProtocolVersion || judgment.PlanDigest != batch.PlanDigest || judgment.BundleDigest != batch.BundleDigest || judgment.AssignmentDigest != batch.AssignmentDigest ||
			judgment.ReviewerAlias != batch.ReviewerAlias || judgment.ReviewerSlot != batch.ReviewerSlot || judgment.QualificationDigest != batch.QualificationDigest ||
			judgment.RubricVersion != batch.RubricVersion {
			return errors.New("relation judgment batch item binding is invalid")
		}
		submittedAt, _ := time.Parse(time.RFC3339, judgment.SubmittedAt)
		if !committedAt.After(submittedAt) {
			return errors.New("relation judgment batch must be committed strictly after every submission")
		}
	}
	expected, err := judgmentBatchDigest(batch)
	if err != nil || batch.Digest != expected {
		return errors.New("relation judgment batch digest is invalid")
	}
	return nil
}

func VerifyJudgmentBatch(batch JudgmentBatch, assignment ReviewAssignment, bundle ReviewBundle) error {
	if err := batch.Validate(); err != nil {
		return err
	}
	if err := VerifyReviewAssignment(assignment, bundle); err != nil {
		return err
	}
	return validateJudgmentBatchAgainstAssignment(batch, assignment, bundle)
}

func validatePairJudgmentDraft(draft PairJudgmentDraft, assignedAt string) error {
	if !validOpaqueID(draft.PacketID, "relation-packet-") || strings.TrimSpace(draft.RevisionReason) == "" || len(draft.RevisionReason) > 512 {
		return errors.New("relation pair judgment draft packet or revision reason is invalid")
	}
	if err := validateVisibleObservations(draft.Observations); err != nil {
		return err
	}
	if err := validateReasonCodes(draft.ReasonCodes); err != nil {
		return err
	}
	if err := validateObservationReasonConsistency(draft.Observations, draft.ReasonCodes); err != nil {
		return err
	}
	return requireRelationTimeNotBefore("relation pair judgment submission", draft.SubmittedAt, assignedAt)
}

func validatePairJudgmentParent(parent PairJudgment, bundle ReviewBundle, assignment ReviewAssignment, packet BlindPacket, submittedAt string) error {
	if err := parent.Validate(); err != nil {
		return err
	}
	if parent.ProtocolVersion != bundle.ProtocolVersion || parent.ProtocolVersion != assignment.ProtocolVersion || parent.BundleDigest != bundle.Digest || parent.AssignmentDigest != assignment.Digest || parent.PacketID != packet.PacketID || parent.PacketDigest != packet.Digest ||
		parent.ReviewerAlias != assignment.Reviewer.ReviewerAlias || parent.ReviewerSlot != assignment.ReviewerSlot || parent.QualificationDigest != assignment.Qualification.Digest {
		return errors.New("relation pair judgment revision changes an immutable parent binding")
	}
	return requireRelationTimeAfter("relation pair judgment revision", submittedAt, parent.SubmittedAt)
}

func validateJudgmentBatchAgainstAssignment(batch JudgmentBatch, assignment ReviewAssignment, bundle ReviewBundle) error {
	if batch.ProtocolVersion != bundle.ProtocolVersion || batch.ProtocolVersion != assignment.ProtocolVersion || batch.PlanDigest != bundle.PlanDigest || batch.BundleDigest != bundle.Digest || batch.AssignmentDigest != assignment.Digest ||
		batch.ReviewerAlias != assignment.Reviewer.ReviewerAlias || batch.ReviewerSlot != assignment.ReviewerSlot ||
		batch.QualificationDigest != assignment.Qualification.Digest || batch.RubricVersion != assignment.RubricVersion || len(batch.Judgments) != len(assignment.PacketIDs) {
		return errors.New("relation judgment batch does not bind its exact assignment")
	}
	want := append([]string(nil), assignment.PacketIDs...)
	got := make([]string, len(batch.Judgments))
	for index, judgment := range batch.Judgments {
		packet, exists := reviewPacketByID(bundle, judgment.PacketID)
		if !exists || judgment.PacketDigest != packet.Digest {
			return errors.New("relation judgment batch packet digest differs from the bundle")
		}
		if err := requireRelationTimeNotBefore("relation pair judgment submission", judgment.SubmittedAt, assignment.AssignedAt); err != nil {
			return err
		}
		got[index] = judgment.PacketID
	}
	sort.Strings(want)
	return requireEqualPacketIDs("relation judgment batch", want, got)
}

func requireEqualPacketIDs(name string, left, right []string) error {
	if !slices.Equal(left, right) {
		return fmt.Errorf("%s packet coverage is incomplete or different", name)
	}
	return nil
}

func reviewPacketByID(bundle ReviewBundle, packetID string) (BlindPacket, bool) {
	for _, packet := range bundle.Packets {
		if packet.PacketID == packetID {
			return packet, true
		}
	}
	return BlindPacket{}, false
}

func requireRelationTimeAfter(name, value, boundary string) error {
	current, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fmt.Errorf("%s time must be RFC3339", name)
	}
	previous, err := time.Parse(time.RFC3339, boundary)
	if err != nil || !current.After(previous) {
		return fmt.Errorf("%s must occur strictly after its boundary", name)
	}
	return nil
}

func requireRelationTimeNotBefore(name, value, boundary string) error {
	current, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fmt.Errorf("%s time must be RFC3339", name)
	}
	previous, err := time.Parse(time.RFC3339, boundary)
	if err != nil || current.Before(previous) {
		return fmt.Errorf("%s cannot precede its boundary", name)
	}
	return nil
}

func pairJudgmentDigest(value PairJudgment) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func judgmentBatchDigest(value JudgmentBatch) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
