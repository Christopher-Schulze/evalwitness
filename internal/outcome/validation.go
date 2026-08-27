package outcome

import (
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

func SealPlan(plan Plan) (Plan, error) {
	plan.SchemaVersion = PlanSchemaVersion
	plan.CanonicalPolicy = CanonicalPolicy
	plan.Digest = ""
	digest, err := planDigest(plan)
	if err != nil {
		return Plan{}, err
	}
	plan.Digest = digest
	return plan, plan.Validate()
}

func (plan Plan) Validate() error {
	if plan.SchemaVersion != PlanSchemaVersion || plan.CanonicalPolicy != CanonicalPolicy || missing(plan.ProtocolVersion, plan.SamplingRule,
		plan.ReplacementRule, plan.BlindingRule, plan.RubricVersion, plan.BootstrapSeed, plan.ConflictRule, plan.OutcomeResolutionRule,
		plan.SensitivityAnalysis, plan.PublicPacketPolicy, plan.PrivateMappingPolicy) || !validDigest(plan.SourceCorpusDigest) {
		return errors.New("outcome adjudication plan identity or rules are incomplete")
	}
	if plan.MutationSampleSize < len(plan.MutationFamilies) || plan.NaturalSampleSize < 1 || plan.PrimaryAdjudicators != 2 ||
		plan.TieBreakAdjudicators != 1 || plan.BootstrapIterations < 10_000 || !plan.RecruitmentRequiresConsent {
		return errors.New("outcome adjudication sample, reviewer, bootstrap, or consent design is invalid")
	}
	if err := uniqueSorted("outcome strata", plan.RequiredStrata); err != nil {
		return err
	}
	if err := uniqueSorted("outcome mutation families", plan.MutationFamilies); err != nil {
		return err
	}
	if err := uniqueSorted("outcome agreement metrics", plan.AgreementMetrics); err != nil {
		return err
	}
	for _, required := range []string{"raw_agreement", "cohen_kappa", "label_prevalence"} {
		if !slices.Contains(plan.AgreementMetrics, required) {
			return fmt.Errorf("outcome adjudication plan omits required agreement metric %q", required)
		}
	}
	expected, err := planDigest(plan)
	if err != nil || plan.Digest != expected {
		return errors.New("outcome adjudication plan digest is invalid")
	}
	return nil
}

func SealEvidence(evidence Evidence) (Evidence, error) {
	evidence.Digest = ""
	digest, err := evidenceDigest(evidence)
	if err != nil {
		return Evidence{}, err
	}
	evidence.Digest = digest
	return evidence, evidence.Validate()
}

func (evidence Evidence) Validate() error {
	if missing(evidence.ID, evidence.ObservedAt, evidence.Limitation) || !validEvidenceKind(evidence.Kind) || !validState(evidence.State) ||
		!validDigest(evidence.ArtifactDigest) || !validDigest(evidence.Digest) {
		return errors.New("outcome evidence identity, kind, state, artifact, time, or limitation is invalid")
	}
	if _, err := time.Parse(time.RFC3339, evidence.ObservedAt); err != nil {
		return errors.New("outcome evidence timestamp must be RFC3339")
	}
	if evidence.Kind == EvidenceIndependentRun || evidence.Kind == EvidenceFormalRelation {
		if !evidence.Independent || missing(evidence.ValidatorID) {
			return errors.New("executable and formal outcome evidence must be independent and name its validator")
		}
	}
	if err := uniqueSortedDigests("outcome evidence parents", evidence.ParentDigests); err != nil {
		return err
	}
	expected, err := evidenceDigest(evidence)
	if err != nil || evidence.Digest != expected {
		return errors.New("outcome evidence digest is invalid")
	}
	return nil
}

func SealRecord(record Record) (Record, error) {
	record.SchemaVersion = OutcomeSchemaVersion
	record.CanonicalPolicy = CanonicalPolicy
	record.RecordID = ""
	record.Digest = ""
	digest, err := recordDigest(record)
	if err != nil {
		return Record{}, err
	}
	record.Digest = digest
	record.RecordID = "outcome-" + digest
	return record, record.Validate()
}

func (record Record) Validate() error {
	if record.SchemaVersion != OutcomeSchemaVersion || record.CanonicalPolicy != CanonicalPolicy || missing(record.RecordID, record.TaskAlias,
		record.AuthorID, record.RevisionReason) || record.Revision < 1 || !validState(record.Resolution) || len(record.Evidence) == 0 {
		return errors.New("outcome record identity, revision, resolution, author, or evidence is invalid")
	}
	if record.Revision == 1 && record.ParentDigest != "" || record.Revision > 1 && !validDigest(record.ParentDigest) {
		return errors.New("outcome revision parent does not match revision number")
	}
	ids := make(map[string]struct{}, len(record.Evidence))
	for index, evidence := range record.Evidence {
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("outcome evidence %d: %w", index, err)
		}
		if index > 0 && record.Evidence[index-1].ID >= evidence.ID {
			return errors.New("outcome evidence must be unique and sorted by ID")
		}
		ids[evidence.ID] = struct{}{}
	}
	if record.Resolution != StateNotAdjudicated && len(record.ResolutionBasis) == 0 {
		return errors.New("resolved outcome requires evidence basis")
	}
	if record.Resolution == StateNotAdjudicated && len(record.ResolutionBasis) != 0 {
		return errors.New("not-adjudicated outcome cannot claim an evidence basis")
	}
	if err := uniqueSorted("outcome resolution basis", record.ResolutionBasis); err != nil {
		return err
	}
	for _, id := range record.ResolutionBasis {
		if _, exists := ids[id]; !exists {
			return fmt.Errorf("outcome resolution basis %q is not an evidence record", id)
		}
	}
	if record.Resolution != StateNotAdjudicated {
		basisStates := make(map[State]struct{}, len(record.ResolutionBasis))
		for _, evidence := range record.Evidence {
			if slices.Contains(record.ResolutionBasis, evidence.ID) {
				basisStates[evidence.State] = struct{}{}
			}
		}
		_, matchingBasis := basisStates[record.Resolution]
		if record.Resolution != StateIndeterminate && !matchingBasis {
			return errors.New("decisive outcome resolution requires matching basis evidence")
		}
		if record.Resolution == StateIndeterminate && !matchingBasis && len(basisStates) < 2 {
			return errors.New("indeterminate outcome requires indeterminate or conflicting basis evidence")
		}
	}
	if err := uniqueSorted("outcome limitations", record.Limitations); err != nil {
		return err
	}
	expected, err := recordDigest(record)
	if err != nil || record.Digest != expected || record.RecordID != "outcome-"+expected {
		return errors.New("outcome record identity is invalid")
	}
	return nil
}

func SealBlindPacket(packet BlindPacket, opaquePacketID string) (BlindPacket, error) {
	packet.SchemaVersion = PacketSchemaVersion
	packet.CanonicalPolicy = CanonicalPolicy
	packet.PacketID = opaquePacketID
	packet.Digest = ""
	digest, err := packetDigest(packet)
	if err != nil {
		return BlindPacket{}, err
	}
	packet.Digest = digest
	return packet, packet.Validate()
}

func (packet BlindPacket) Validate() error {
	if packet.SchemaVersion != PacketSchemaVersion || packet.CanonicalPolicy != CanonicalPolicy || !validOpaquePacketID(packet.PacketID) || !validOpaqueTaskAlias(packet.TaskAlias) || missing(
		packet.PrivacyClass) || !validDigest(packet.PlanDigest) || len(packet.Evidence) == 0 || len(packet.RubricQuestions) == 0 {
		return errors.New("outcome blind packet identity, plan, evidence, rubric, or privacy is invalid")
	}
	for index, item := range packet.Evidence {
		if !validOpaqueSlot(item.Slot) || missing(item.Kind, item.License, item.Limitation) || !validDigest(item.ContentDigest) || item.Content == "" && packet.PublicReleasable {
			return fmt.Errorf("outcome packet evidence %d is incomplete", index)
		}
		if index > 0 && packet.Evidence[index-1].Slot >= item.Slot {
			return errors.New("outcome packet evidence must have unique sorted slots")
		}
		if item.Content != "" && digestText(item.Content) != item.ContentDigest {
			return fmt.Errorf("outcome packet evidence %d content digest is invalid", index)
		}
	}
	if err := uniqueSorted("outcome packet rubric questions", packet.RubricQuestions); err != nil {
		return err
	}
	expected, err := packetDigest(packet)
	if err != nil || packet.Digest != expected {
		return errors.New("outcome blind packet identity is invalid")
	}
	return nil
}

func ValidatePacketLeakage(packet BlindPacket, forbidden []string) error {
	if err := packet.Validate(); err != nil {
		return err
	}
	encoded, err := EncodeIndented(packet)
	if err != nil {
		return err
	}
	haystack := strings.ToLower(string(encoded))
	for _, value := range forbidden {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && strings.Contains(haystack, value) {
			return fmt.Errorf("outcome packet contains forbidden blinded value %q", value)
		}
	}
	return nil
}

func SealLabel(label Label) (Label, error) {
	label.SchemaVersion = LabelSchemaVersion
	label.CanonicalPolicy = CanonicalPolicy
	label.LabelID = ""
	label.Digest = ""
	digest, err := labelDigest(label)
	if err != nil {
		return Label{}, err
	}
	label.Digest = digest
	label.LabelID = "label-" + digest
	return label, label.Validate()
}

func (label Label) Validate() error {
	if label.SchemaVersion != LabelSchemaVersion || label.CanonicalPolicy != CanonicalPolicy || missing(label.LabelID,
		label.AdjudicatorAlias, label.SubmittedAt, label.RubricVersion) || !validOpaquePacketID(label.PacketID) || label.ReviewerSlot < 1 || label.ReviewerSlot > 3 ||
		!adjudicatableState(label.PrimaryOutcome) || !validDigest(label.QualificationDigest) {
		return errors.New("blinded outcome label identity, reviewer, round, outcome, rubric, time, or qualification is invalid")
	}
	if _, err := time.Parse(time.RFC3339, label.SubmittedAt); err != nil {
		return errors.New("blinded outcome label timestamp must be RFC3339")
	}
	for _, rating := range []AxisRating{label.TaskSatisfaction, label.TechnicalCorrectness, label.VerificationQuality, label.HarmfulSideEffects, label.EvidenceSufficiency} {
		if !validRating(rating) {
			return errors.New("blinded outcome label contains an invalid rubric rating")
		}
	}
	if err := validReasonCodes(label.ReasonCodes); err != nil {
		return err
	}
	if err := uniqueSorted("outcome label conflicts", label.ConflictsOfInterest); err != nil {
		return err
	}
	expected, err := labelDigest(label)
	if err != nil || label.Digest != expected || label.LabelID != "label-"+expected {
		return errors.New("blinded outcome label identity is invalid")
	}
	return nil
}

func planDigest(plan Plan) (string, error)          { plan.Digest = ""; return digestJSON(plan) }
func evidenceDigest(value Evidence) (string, error) { value.Digest = ""; return digestJSON(value) }
func recordDigest(value Record) (string, error) {
	value.RecordID, value.Digest = "", ""
	return digestJSON(value)
}
func packetDigest(value BlindPacket) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func labelDigest(value Label) (string, error) {
	value.LabelID, value.Digest = "", ""
	return digestJSON(value)
}

func validState(state State) bool {
	return slices.Contains([]State{StateSolved, StateUnsolved, StateIndeterminate, StateInvalidTask, StateEnvironmentFail, StateNotAdjudicated}, state)
}

func adjudicatableState(state State) bool { return validState(state) && state != StateNotAdjudicated }

func validEvidenceKind(kind EvidenceKind) bool {
	return slices.Contains([]EvidenceKind{EvidenceClaimedTest, EvidenceBenchmarkReward, EvidenceIndependentRun, EvidenceFormalRelation, EvidenceHumanLabel}, kind)
}

func validRating(rating AxisRating) bool {
	return slices.Contains([]AxisRating{RatingSufficient, RatingInsufficient, RatingUnclear, RatingNotApplicable}, rating)
}

func validReasonCodes(values []ReasonCode) error {
	if len(values) == 0 {
		return errors.New("outcome label requires at least one reason code")
	}
	allowed := []ReasonCode{
		ReasonClaimedOnly, ReasonEnvironmentFailure, ReasonEvidenceConflict, ReasonEvidenceConsistent,
		ReasonEvidenceInsufficient, ReasonFormalRelationSupports, ReasonHarmfulSideEffect,
		ReasonIndependentTestsFail, ReasonIndependentTestsPass, ReasonInvalidTask, ReasonTaskSatisfied,
		ReasonTaskUnsatisfied, ReasonTechnicalDefect, ReasonVerificationComplete, ReasonVerificationIncomplete,
	}
	if !slices.IsSorted(values) {
		return errors.New("outcome label reason codes must be sorted")
	}
	for index, value := range values {
		if !slices.Contains(allowed, value) || index > 0 && values[index-1] == value {
			return errors.New("outcome label reason codes contain an unknown or duplicate value")
		}
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validOpaquePacketID(value string) bool {
	return strings.HasPrefix(value, "packet-") && validDigest(strings.TrimPrefix(value, "packet-"))
}

func validOpaqueSlot(value string) bool {
	return strings.HasPrefix(value, "slot-") && validDigest(strings.TrimPrefix(value, "slot-"))
}

func validOpaqueTaskAlias(value string) bool {
	return strings.HasPrefix(value, "taskref-") && validDigest(strings.TrimPrefix(value, "taskref-"))
}

func uniqueSorted(name string, values []string) error {
	if !sort.StringsAreSorted(values) {
		return fmt.Errorf("%s must be sorted", name)
	}
	for index, value := range values {
		if strings.TrimSpace(value) == "" || index > 0 && values[index-1] == value {
			return fmt.Errorf("%s contains an empty or duplicate value", name)
		}
	}
	return nil
}

func uniqueSortedDigests(name string, values []string) error {
	if err := uniqueSorted(name, values); err != nil {
		return err
	}
	for _, value := range values {
		if !validDigest(value) {
			return fmt.Errorf("%s contains an invalid digest", name)
		}
	}
	return nil
}

func missing(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}
