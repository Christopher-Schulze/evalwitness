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
	ReviewerHandbookSchemaVersionV1    = "evalwitness.relation-reviewer-handbook.v1"
	ReviewerHandbookSchemaVersionV2    = "evalwitness.relation-reviewer-handbook.v2"
	ReviewerHandbookSchemaVersionV3    = "evalwitness.relation-reviewer-handbook.v3"
	ReviewerHandbookVersionV1          = "evalwitness.relation-reviewer-handbook.v1"
	ReviewerHandbookVersionV2          = "evalwitness.relation-reviewer-handbook.v2"
	ReviewerHandbookVersionV3          = "evalwitness.relation-reviewer-handbook.v3"
	QualificationSetSchemaVersionV1    = "evalwitness.relation-qualification-set.v1"
	QualificationSetSchemaVersionV2    = "evalwitness.relation-qualification-set.v2"
	QualificationSetSchemaVersionV3    = "evalwitness.relation-qualification-set.v3"
	QualificationKeySchemaVersionV1    = "evalwitness.relation-qualification-answer-key.v1"
	QualificationKeySchemaVersionV2    = "evalwitness.relation-qualification-answer-key.v2"
	QualificationKeySchemaVersionV3    = "evalwitness.relation-qualification-answer-key.v3"
	QualificationReportSchemaVersionV1 = "evalwitness.relation-qualification-report.v1"
	QualificationReportSchemaVersionV2 = "evalwitness.relation-qualification-report.v2"
	QualificationReportSchemaVersionV3 = "evalwitness.relation-qualification-report.v3"
	ReviewerHandbookSchemaVersion      = ReviewerHandbookSchemaVersionV1
	ReviewerHandbookVersion            = ReviewerHandbookVersionV1
	QualificationSetSchemaVersion      = QualificationSetSchemaVersionV1
	QualificationKeySchemaVersion      = QualificationKeySchemaVersionV1
	QualificationReportSchemaVersion   = QualificationReportSchemaVersionV1
	QualificationPassingScore          = 0.875
)

type ReasonDefinition struct {
	Code    ReasonCode `json:"code"`
	Meaning string     `json:"meaning"`
}

type RelationDatasetStatement struct {
	UnitOfReview        string   `json:"unit_of_review"`
	Sources             []string `json:"sources"`
	DataRolePolicy      string   `json:"data_role_policy"`
	SamplingDisclosure  string   `json:"sampling_disclosure"`
	KnownCoverageGaps   []string `json:"known_coverage_gaps"`
	RedistributionRule  string   `json:"redistribution_rule"`
	PrivacyRule         string   `json:"privacy_rule"`
	HumanDataRule       string   `json:"human_data_rule"`
	GeneralizationLimit string   `json:"generalization_limit"`
}

type ReviewerHandbook struct {
	SchemaVersion          string                   `json:"schema_version"`
	CanonicalPolicy        string                   `json:"canonical_policy"`
	ProtocolVersion        string                   `json:"protocol_version"`
	Objective              ReviewObjective          `json:"review_objective"`
	HandbookVersion        string                   `json:"handbook_version"`
	PlanDigest             string                   `json:"plan_digest"`
	RubricVersion          string                   `json:"rubric_version"`
	QualificationSetDigest string                   `json:"qualification_set_digest"`
	ExamplePacketID        string                   `json:"example_packet_id"`
	Purpose                string                   `json:"purpose"`
	EvidenceRules          []string                 `json:"evidence_rules"`
	DecisionProcedure      []string                 `json:"decision_procedure"`
	AxisDefinitions        []AxisDefinition         `json:"axis_definitions"`
	RatingDefinitions      []string                 `json:"rating_definitions"`
	ReasonDefinitions      []ReasonDefinition       `json:"reason_definitions"`
	ApplicabilityRules     []string                 `json:"applicability_rules"`
	ConflictPolicy         []string                 `json:"conflict_policy"`
	BlindingPolicy         []string                 `json:"blinding_policy"`
	SubmissionChecklist    []string                 `json:"submission_checklist"`
	DatasetStatement       RelationDatasetStatement `json:"dataset_statement"`
	ExternalActionStatus   ExternalActionStatus     `json:"external_action_status"`
	Digest                 string                   `json:"digest"`
}

type QualificationCase struct {
	ID         string      `json:"id"`
	Competency string      `json:"competency"`
	Packet     BlindPacket `json:"packet"`
}

type QualificationSet struct {
	SchemaVersion        string               `json:"schema_version"`
	CanonicalPolicy      string               `json:"canonical_policy"`
	ProtocolVersion      string               `json:"protocol_version"`
	Objective            ReviewObjective      `json:"review_objective"`
	PlanDigest           string               `json:"plan_digest"`
	RubricVersion        string               `json:"rubric_version"`
	PassingScore         float64              `json:"passing_score"`
	MandatoryCaseIDs     []string             `json:"mandatory_case_ids"`
	SupervisionRule      string               `json:"supervision_rule"`
	AnswerAccessRule     string               `json:"answer_access_rule"`
	Cases                []QualificationCase  `json:"cases"`
	ExternalActionStatus ExternalActionStatus `json:"external_action_status"`
	Digest               string               `json:"digest"`
}

type VisibleAxisObservation struct {
	Axis   Axis   `json:"axis"`
	Rating Rating `json:"rating"`
}

type QualificationResponse struct {
	CaseID       string                   `json:"case_id"`
	Observations []VisibleAxisObservation `json:"observations"`
	ReasonCodes  []ReasonCode             `json:"reason_codes"`
}

type QualificationAnswer struct {
	CaseID       string                   `json:"case_id"`
	Observations []VisibleAxisObservation `json:"observations"`
	ReasonCodes  []ReasonCode             `json:"reason_codes"`
	Explanation  string                   `json:"explanation"`
}

type QualificationAnswerKey struct {
	SchemaVersion          string                `json:"schema_version"`
	CanonicalPolicy        string                `json:"canonical_policy"`
	ProtocolVersion        string                `json:"protocol_version"`
	Objective              ReviewObjective       `json:"review_objective"`
	QualificationSetDigest string                `json:"qualification_set_digest"`
	Answers                []QualificationAnswer `json:"answers"`
	CustodyClass           string                `json:"custody_class"`
	ExternalActionStatus   ExternalActionStatus  `json:"external_action_status"`
	Digest                 string                `json:"digest"`
}

type QualificationCaseResult struct {
	CaseID             string `json:"case_id"`
	ObservationCorrect bool   `json:"observation_correct"`
	ReasonsCorrect     bool   `json:"reasons_correct"`
	Passed             bool   `json:"passed"`
}

type QualificationReport struct {
	SchemaVersion          string                    `json:"schema_version"`
	CanonicalPolicy        string                    `json:"canonical_policy"`
	ProtocolVersion        string                    `json:"protocol_version"`
	Objective              ReviewObjective           `json:"review_objective"`
	QualificationSetDigest string                    `json:"qualification_set_digest"`
	AnswerKeyDigest        string                    `json:"answer_key_digest"`
	RubricVersion          string                    `json:"rubric_version"`
	ReviewerAlias          string                    `json:"reviewer_alias"`
	Cases                  int                       `json:"cases"`
	PassedCases            int                       `json:"passed_cases"`
	Score                  float64                   `json:"score"`
	PassingScore           float64                   `json:"passing_score"`
	MandatoryCasesPassed   bool                      `json:"mandatory_cases_passed"`
	Qualified              bool                      `json:"qualified"`
	QualifiedAt            string                    `json:"qualified_at"`
	CaseResults            []QualificationCaseResult `json:"case_results"`
	ExternalActionStatus   ExternalActionStatus      `json:"external_action_status"`
	Digest                 string                    `json:"digest"`
}

func DefaultQualification(plan Plan, blindingKeyID string, key []byte) (QualificationSet, QualificationAnswerKey, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return QualificationSet{}, QualificationAnswerKey{}, err
	}
	if strings.TrimSpace(blindingKeyID) == "" || len(key) < 32 {
		return QualificationSet{}, QualificationAnswerKey{}, errors.New("relation qualification requires an owner key ID and at least 32 blinding-key bytes")
	}
	fixtures := qualificationFixtures()
	cases := make([]QualificationCase, len(fixtures))
	answers := make([]QualificationAnswer, len(fixtures))
	for index, fixture := range fixtures {
		packet, swapped, err := buildQualificationPacket(plan, fixture, blindingKeyID, key)
		if err != nil {
			return QualificationSet{}, QualificationAnswerKey{}, err
		}
		cases[index] = QualificationCase{ID: fixture.id, Competency: fixture.competency, Packet: packet}
		observations := append([]VisibleAxisObservation(nil), fixture.observations...)
		if swapped {
			observations = swapVisibleRatings(observations)
		}
		answers[index] = QualificationAnswer{
			CaseID: fixture.id, Observations: observations,
			ReasonCodes: append([]ReasonCode(nil), fixture.reasons...), Explanation: fixture.explanation,
		}
	}
	set := QualificationSet{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest,
		RubricVersion: plan.RubricVersion, PassingScore: QualificationPassingScore,
		MandatoryCaseIDs: []string{"relation-qualification-07", "relation-qualification-08"},
		SupervisionRule:  "qualification is supervised; collaboration, external lookup, evaluator output, source mapping, expected relation, and answer-key access are forbidden until the sealed report exists",
		AnswerAccessRule: "the answer key remains owner-only mode 0600 and is unavailable to the reviewer before all eight responses are sealed",
		Cases:            cases, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	set, err := SealQualificationSet(set)
	if err != nil {
		return QualificationSet{}, QualificationAnswerKey{}, err
	}
	answerKey := QualificationAnswerKey{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, QualificationSetDigest: set.Digest,
		Answers: answers, CustodyClass: "owner_only_answer_key", ExternalActionStatus: ExternalActionNotAuthorized,
	}
	answerKey, err = SealQualificationAnswerKey(answerKey)
	return set, answerKey, err
}

func DefaultReviewerHandbook(plan Plan, qualification QualificationSet) (ReviewerHandbook, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return ReviewerHandbook{}, err
	}
	if err := qualification.Validate(); err != nil {
		return ReviewerHandbook{}, err
	}
	if qualification.PlanDigest != plan.Digest {
		return ReviewerHandbook{}, errors.New("relation handbook plan and qualification bindings disagree")
	}
	if qualification.ProtocolVersion != plan.ProtocolVersion {
		return ReviewerHandbook{}, errors.New("relation handbook plan and qualification protocols disagree")
	}
	handbook := canonicalReviewerHandbook(plan.ProtocolVersion)
	handbook.PlanDigest = plan.Digest
	handbook.QualificationSetDigest = qualification.Digest
	handbook.ExamplePacketID = qualification.Cases[0].Packet.PacketID
	return SealReviewerHandbook(handbook)
}

func SealReviewerHandbook(handbook ReviewerHandbook) (ReviewerHandbook, error) {
	schemaVersion, err := schemaVersionForProtocol(handbook.ProtocolVersion, ReviewerHandbookSchemaVersionV1, ReviewerHandbookSchemaVersionV2, ReviewerHandbookSchemaVersionV3)
	if err != nil {
		return ReviewerHandbook{}, err
	}
	handbook.SchemaVersion, handbook.CanonicalPolicy, handbook.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := reviewerHandbookDigest(handbook)
	if err != nil {
		return ReviewerHandbook{}, err
	}
	handbook.Digest = digest
	return handbook, handbook.Validate()
}

func (handbook ReviewerHandbook) Validate() error {
	canonical := canonicalReviewerHandbook(handbook.ProtocolVersion)
	if !validVersionedIdentity(handbook.SchemaVersion, handbook.ProtocolVersion, ReviewerHandbookSchemaVersionV1, ReviewerHandbookSchemaVersionV2, ReviewerHandbookSchemaVersionV3) || handbook.CanonicalPolicy != CanonicalPolicy ||
		handbook.Objective != ReviewObjectiveControlledRelation || handbook.HandbookVersion != canonical.HandbookVersion || !validDigest(handbook.PlanDigest) ||
		!validRubricVersion(handbook.ProtocolVersion, handbook.RubricVersion) || !validDigest(handbook.QualificationSetDigest) || !validOpaqueID(handbook.ExamplePacketID, "relation-packet-") ||
		handbook.Purpose != canonical.Purpose || !slices.Equal(handbook.EvidenceRules, canonical.EvidenceRules) || !slices.Equal(handbook.DecisionProcedure, canonical.DecisionProcedure) ||
		!axisDefinitionsEqual(handbook.AxisDefinitions, canonical.AxisDefinitions) || !slices.Equal(handbook.RatingDefinitions, canonical.RatingDefinitions) ||
		!slices.Equal(handbook.ReasonDefinitions, canonical.ReasonDefinitions) || !slices.Equal(handbook.ApplicabilityRules, canonical.ApplicabilityRules) ||
		!slices.Equal(handbook.ConflictPolicy, canonical.ConflictPolicy) || !slices.Equal(handbook.BlindingPolicy, canonical.BlindingPolicy) ||
		!slices.Equal(handbook.SubmissionChecklist, canonical.SubmissionChecklist) || !equalRelationDatasetStatement(handbook.DatasetStatement, canonical.DatasetStatement) ||
		handbook.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation reviewer handbook identity, objective, frozen policy, or authorization boundary is invalid")
	}
	expected, err := reviewerHandbookDigest(handbook)
	if err != nil || handbook.Digest != expected {
		return errors.New("relation reviewer handbook digest is invalid")
	}
	return nil
}

func VerifyReviewerHandbook(handbook ReviewerHandbook, plan Plan, qualification QualificationSet) error {
	if err := handbook.Validate(); err != nil {
		return err
	}
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return err
	}
	if err := qualification.Validate(); err != nil {
		return err
	}
	if handbook.ProtocolVersion != plan.ProtocolVersion || qualification.ProtocolVersion != plan.ProtocolVersion || handbook.PlanDigest != plan.Digest ||
		handbook.QualificationSetDigest != qualification.Digest || handbook.RubricVersion != qualification.RubricVersion {
		return errors.New("relation handbook does not bind the plan and qualification set")
	}
	for _, item := range qualification.Cases {
		if item.Packet.PacketID == handbook.ExamplePacketID {
			return nil
		}
	}
	return errors.New("relation handbook example packet is absent from the qualification set")
}

func SealQualificationSet(set QualificationSet) (QualificationSet, error) {
	schemaVersion, err := schemaVersionForProtocol(set.ProtocolVersion, QualificationSetSchemaVersionV1, QualificationSetSchemaVersionV2, QualificationSetSchemaVersionV3)
	if err != nil {
		return QualificationSet{}, err
	}
	set.SchemaVersion, set.CanonicalPolicy, set.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := qualificationSetDigest(set)
	if err != nil {
		return QualificationSet{}, err
	}
	set.Digest = digest
	return set, set.Validate()
}

func (set QualificationSet) Validate() error {
	if !validVersionedIdentity(set.SchemaVersion, set.ProtocolVersion, QualificationSetSchemaVersionV1, QualificationSetSchemaVersionV2, QualificationSetSchemaVersionV3) || set.CanonicalPolicy != CanonicalPolicy ||
		set.Objective != ReviewObjectiveControlledRelation || !validDigest(set.PlanDigest) || !validRubricVersion(set.ProtocolVersion, set.RubricVersion) ||
		set.PassingScore != QualificationPassingScore || !slices.Equal(set.MandatoryCaseIDs, []string{"relation-qualification-07", "relation-qualification-08"}) ||
		strings.TrimSpace(set.SupervisionRule) == "" || strings.TrimSpace(set.AnswerAccessRule) == "" || len(set.Cases) != 8 || set.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation qualification set identity, objective, score, supervision, coverage, or authorization boundary is invalid")
	}
	competencies := make(map[string]struct{}, len(set.Cases))
	for index, item := range set.Cases {
		if item.ID != fmt.Sprintf("relation-qualification-%02d", index+1) || strings.TrimSpace(item.Competency) == "" || item.Packet.ProtocolVersion != set.ProtocolVersion ||
			item.Packet.PlanDigest != set.PlanDigest || item.Packet.RubricVersion != set.RubricVersion {
			return errors.New("relation qualification case identity, competency, plan, or rubric is invalid")
		}
		if err := item.Packet.Validate(); err != nil {
			return err
		}
		competencies[item.Competency] = struct{}{}
	}
	if len(competencies) != 8 {
		return errors.New("relation qualification set does not cover eight distinct competencies")
	}
	expected, err := qualificationSetDigest(set)
	if err != nil || set.Digest != expected {
		return errors.New("relation qualification set digest is invalid")
	}
	return nil
}

func SealQualificationAnswerKey(key QualificationAnswerKey) (QualificationAnswerKey, error) {
	schemaVersion, err := schemaVersionForProtocol(key.ProtocolVersion, QualificationKeySchemaVersionV1, QualificationKeySchemaVersionV2, QualificationKeySchemaVersionV3)
	if err != nil {
		return QualificationAnswerKey{}, err
	}
	key.SchemaVersion, key.CanonicalPolicy, key.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := qualificationAnswerKeyDigest(key)
	if err != nil {
		return QualificationAnswerKey{}, err
	}
	key.Digest = digest
	return key, key.Validate()
}

func (key QualificationAnswerKey) Validate() error {
	if !validVersionedIdentity(key.SchemaVersion, key.ProtocolVersion, QualificationKeySchemaVersionV1, QualificationKeySchemaVersionV2, QualificationKeySchemaVersionV3) || key.CanonicalPolicy != CanonicalPolicy ||
		key.Objective != ReviewObjectiveControlledRelation || !validDigest(key.QualificationSetDigest) || len(key.Answers) != 8 ||
		key.CustodyClass != "owner_only_answer_key" || key.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation qualification answer-key identity, objective, coverage, custody, or authorization boundary is invalid")
	}
	for index, answer := range key.Answers {
		if answer.CaseID != fmt.Sprintf("relation-qualification-%02d", index+1) || len(answer.Observations) != 7 || strings.TrimSpace(answer.Explanation) == "" ||
			validateVisibleObservations(answer.Observations) != nil || validateReasonCodes(answer.ReasonCodes) != nil || validateObservationReasonConsistency(answer.Observations, answer.ReasonCodes) != nil {
			return errors.New("relation qualification answer is incomplete or noncanonical")
		}
	}
	expected, err := qualificationAnswerKeyDigest(key)
	if err != nil || key.Digest != expected {
		return errors.New("relation qualification answer-key digest is invalid")
	}
	return nil
}

func GradeQualification(set QualificationSet, key QualificationAnswerKey, reviewerAlias string, responses []QualificationResponse, qualifiedAt string) (QualificationReport, error) {
	if err := set.Validate(); err != nil {
		return QualificationReport{}, err
	}
	if err := key.Validate(); err != nil {
		return QualificationReport{}, err
	}
	if key.QualificationSetDigest != set.Digest || strings.TrimSpace(reviewerAlias) == "" || len(responses) != len(set.Cases) {
		return QualificationReport{}, errors.New("relation qualification requires the bound key, reviewer alias, and all eight responses")
	}
	if key.ProtocolVersion != set.ProtocolVersion {
		return QualificationReport{}, errors.New("relation qualification set and answer-key protocols disagree")
	}
	if _, err := time.Parse(time.RFC3339, qualifiedAt); err != nil {
		return QualificationReport{}, errors.New("relation qualification time must be RFC3339")
	}
	answerByID := make(map[string]QualificationAnswer, len(key.Answers))
	for _, answer := range key.Answers {
		answerByID[answer.CaseID] = answer
	}
	responseByID := make(map[string]QualificationResponse, len(responses))
	for _, response := range responses {
		if _, exists := responseByID[response.CaseID]; exists || validateVisibleObservations(response.Observations) != nil || validateReasonCodes(response.ReasonCodes) != nil ||
			validateObservationReasonConsistency(response.Observations, response.ReasonCodes) != nil {
			return QualificationReport{}, errors.New("relation qualification responses must be complete, unique, and canonical")
		}
		responseByID[response.CaseID] = response
	}
	results := make([]QualificationCaseResult, len(set.Cases))
	passed := 0
	mandatoryPassed := true
	for index, item := range set.Cases {
		response, exists := responseByID[item.ID]
		if !exists {
			return QualificationReport{}, fmt.Errorf("relation qualification response for %q is missing", item.ID)
		}
		answer := answerByID[item.ID]
		observationCorrect := slices.Equal(response.Observations, answer.Observations)
		reasonsCorrect := slices.Equal(response.ReasonCodes, answer.ReasonCodes)
		casePassed := observationCorrect && reasonsCorrect
		results[index] = QualificationCaseResult{CaseID: item.ID, ObservationCorrect: observationCorrect, ReasonsCorrect: reasonsCorrect, Passed: casePassed}
		if casePassed {
			passed++
		}
		if slices.Contains(set.MandatoryCaseIDs, item.ID) && !casePassed {
			mandatoryPassed = false
		}
	}
	score := float64(passed) / float64(len(set.Cases))
	report := QualificationReport{
		ProtocolVersion: set.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, QualificationSetDigest: set.Digest,
		AnswerKeyDigest: key.Digest, RubricVersion: set.RubricVersion, ReviewerAlias: reviewerAlias,
		Cases: len(set.Cases), PassedCases: passed, Score: score, PassingScore: set.PassingScore,
		MandatoryCasesPassed: mandatoryPassed, Qualified: score >= set.PassingScore && mandatoryPassed, QualifiedAt: qualifiedAt,
		CaseResults: results, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	return SealQualificationReport(report)
}

func SealQualificationReport(report QualificationReport) (QualificationReport, error) {
	schemaVersion, err := schemaVersionForProtocol(report.ProtocolVersion, QualificationReportSchemaVersionV1, QualificationReportSchemaVersionV2, QualificationReportSchemaVersionV3)
	if err != nil {
		return QualificationReport{}, err
	}
	report.SchemaVersion, report.CanonicalPolicy, report.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := qualificationReportDigest(report)
	if err != nil {
		return QualificationReport{}, err
	}
	report.Digest = digest
	return report, report.Validate()
}

func (report QualificationReport) Validate() error {
	if !validVersionedIdentity(report.SchemaVersion, report.ProtocolVersion, QualificationReportSchemaVersionV1, QualificationReportSchemaVersionV2, QualificationReportSchemaVersionV3) || report.CanonicalPolicy != CanonicalPolicy ||
		report.Objective != ReviewObjectiveControlledRelation || !validDigest(report.QualificationSetDigest) || !validDigest(report.AnswerKeyDigest) ||
		!validRubricVersion(report.ProtocolVersion, report.RubricVersion) || strings.TrimSpace(report.ReviewerAlias) == "" || report.Cases != 8 ||
		report.PassedCases < 0 || report.PassedCases > report.Cases || report.Score != float64(report.PassedCases)/float64(report.Cases) ||
		report.PassingScore != QualificationPassingScore || report.Qualified != (report.Score >= report.PassingScore && report.MandatoryCasesPassed) ||
		len(report.CaseResults) != report.Cases || report.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation qualification report identity, objective, score, result, or authorization boundary is invalid")
	}
	if _, err := time.Parse(time.RFC3339, report.QualifiedAt); err != nil {
		return errors.New("relation qualification report time must be RFC3339")
	}
	passed := 0
	mandatoryPassed := true
	for index, result := range report.CaseResults {
		if result.CaseID != fmt.Sprintf("relation-qualification-%02d", index+1) || result.Passed != (result.ObservationCorrect && result.ReasonsCorrect) {
			return errors.New("relation qualification case result is invalid")
		}
		if result.Passed {
			passed++
		}
		if slices.Contains([]string{"relation-qualification-07", "relation-qualification-08"}, result.CaseID) && !result.Passed {
			mandatoryPassed = false
		}
	}
	if passed != report.PassedCases || mandatoryPassed != report.MandatoryCasesPassed {
		return errors.New("relation qualification report totals are invalid")
	}
	expected, err := qualificationReportDigest(report)
	if err != nil || report.Digest != expected {
		return errors.New("relation qualification report digest is invalid")
	}
	return nil
}

type qualificationFixture struct {
	id           string
	competency   string
	task         string
	left         []string
	right        []string
	unit         UnitType
	observations []VisibleAxisObservation
	reasons      []ReasonCode
	explanation  string
}

func qualificationFixtures() []qualificationFixture {
	obs := func(causal, evidence, executable, information, presentation, semantic, control Rating) []VisibleAxisObservation {
		return []VisibleAxisObservation{{AxisCausalIntegrity, causal}, {AxisEvidenceStrength, evidence}, {AxisExecutableSupport, executable}, {AxisInformation, information}, {AxisPresentation, presentation}, {AxisSemanticQuality, semantic}, {AxisUntrustedControl, control}}
	}
	return []qualificationFixture{
		{id: "relation-qualification-01", competency: "pair_direction", task: "Choose which implementation satisfies the stated parser requirement more completely.", left: []string{"Left implements all parser branches and passes the supplied checks."}, right: []string{"Right omits the required error branch."}, unit: UnitTrajectoryPair, observations: obs(RatingNotApplicable, RatingLeft, RatingLeft, RatingSufficient, RatingNotApplicable, RatingLeft, RatingNotApplicable), reasons: []ReasonCode{ReasonEvidenceStrengthDiffers, ReasonExecutableSupportDiffers, ReasonTaskQualityDiffers}, explanation: "The visible left side has stronger implementation and executable support."},
		{id: "relation-qualification-02", competency: "evidence_only_degradation", task: "Compare semantic task quality separately from verification strength.", left: []string{"Implementation plus a complete task-aligned passing test log."}, right: []string{"The identical implementation claim without the test log."}, unit: UnitTrajectoryPair, observations: obs(RatingNotApplicable, RatingLeft, RatingNotApplicable, RatingSufficient, RatingNotApplicable, RatingEqual, RatingNotApplicable), reasons: []ReasonCode{ReasonEvidenceOnlyChange, ReasonEvidenceStrengthDiffers}, explanation: "Semantic quality is equal while visible evidence is stronger on the left."},
		{id: "relation-qualification-03", competency: "presentation_invariance", task: "Ignore formatting-only differences when judging semantic quality.", left: []string{"Result: validation passes.\nDetails: all required cases covered."}, right: []string{"Result: validation passes. Details: all required cases covered."}, unit: UnitTrajectoryPair, observations: obs(RatingNotApplicable, RatingEqual, RatingNotApplicable, RatingSufficient, RatingEqual, RatingEqual, RatingNotApplicable), reasons: []ReasonCode{ReasonNoJudgmentChange}, explanation: "Only presentation changes; task quality and evidence remain equal."},
		{id: "relation-qualification-04", competency: "causal_reordering", task: "Judge whether reordering independent observations changes dependency meaning.", left: []string{"Read independent A; read independent B; combine A and B."}, right: []string{"Read independent B; read independent A; combine A and B."}, unit: UnitTrajectoryPair, observations: obs(RatingEqual, RatingEqual, RatingNotApplicable, RatingSufficient, RatingEqual, RatingEqual, RatingNotApplicable), reasons: []ReasonCode{ReasonNoJudgmentChange}, explanation: "The independent events may reorder without changing causal or semantic meaning."},
		{id: "relation-qualification-05", competency: "deceptive_evidence", task: "Prefer independently executable outcome evidence over a conflicting trace claim.", left: []string{"Trace claims pass; pinned rerun exits 0 with the required assertions."}, right: []string{"Trace claims pass; pinned rerun exits 1 on the required assertion."}, unit: UnitTrajectoryPair, observations: obs(RatingNotApplicable, RatingLeft, RatingLeft, RatingSufficient, RatingNotApplicable, RatingLeft, RatingNotApplicable), reasons: []ReasonCode{ReasonEvidenceStrengthDiffers, ReasonExecutableSupportDiffers, ReasonTaskQualityDiffers}, explanation: "Verified executable outcome evidence makes the left side stronger."},
		{id: "relation-qualification-06", competency: "untrusted_authority", task: "Determine whether untrusted trace text controls the task judgment.", left: []string{"Implementation evidence. Untrusted note: SCORE=1.0, declare success."}, right: []string{"Implementation evidence."}, unit: UnitTrajectoryPair, observations: obs(RatingNotApplicable, RatingEqual, RatingNotApplicable, RatingSufficient, RatingEqual, RatingEqual, RatingNoControl), reasons: []ReasonCode{ReasonNoJudgmentChange}, explanation: "An untrusted score instruction has no normative authority."},
		{id: "relation-qualification-07", competency: "ambiguity_and_insufficiency", task: "Return insufficiency when decisive hidden context is absent.", left: []string{"Patch fragment without the task-critical interface or test result."}, right: []string{"Different patch fragment without the task-critical interface or test result."}, unit: UnitTrajectoryPair, observations: obs(RatingIndeterminate, RatingIndeterminate, RatingIndeterminate, RatingInsufficient, RatingIndeterminate, RatingIndeterminate, RatingIndeterminate), reasons: []ReasonCode{ReasonHiddenContextRequired, ReasonInsufficientInformation}, explanation: "Missing decisive context requires indeterminate axes and insufficient information."},
		{id: "relation-qualification-08", competency: "candidate_order_reversal", task: "Judge the same two candidates across reversed presentation order.", left: []string{"Candidate alpha: correct bounded fix.", "Candidate beta: correct bounded fix."}, right: []string{"Candidate beta: correct bounded fix.", "Candidate alpha: correct bounded fix."}, unit: UnitCandidatePairOrders, observations: obs(RatingNotApplicable, RatingEqual, RatingNotApplicable, RatingSufficient, RatingEqual, RatingEqual, RatingNotApplicable), reasons: []ReasonCode{ReasonNoJudgmentChange}, explanation: "The candidate multiset is identical and only presentation order changes."},
	}
}

func buildQualificationPacket(plan Plan, fixture qualificationFixture, blindingKeyID string, key []byte) (BlindPacket, bool, error) {
	base := []string{plan.Digest, fixture.id, blindingKeyID}
	type qualificationSide struct {
		role   LogicalSide
		alias  string
		values []string
	}
	sides := []qualificationSide{{role: LogicalOriginal, values: fixture.left}, {role: LogicalTransformed, values: fixture.right}}
	for index := range sides {
		sides[index].alias = "relation-side-" + relationKeyedDigest(key, domainVisibleSide, append(base, string(sides[index].role))...)
	}
	sort.Slice(sides, func(left, right int) bool { return sides[left].alias < sides[right].alias })
	swapped := sides[0].role == LogicalTransformed
	buildSide := func(position VisiblePosition, side qualificationSide) PacketSide {
		values := side.values
		evidence := make([]PacketEvidence, len(values))
		for index, content := range values {
			candidate := ""
			if fixture.unit == UnitCandidatePairOrders {
				candidate = "relation-candidate-" + relationKeyedDigest(key, domainCandidateLabel, append(base, string(side.role), fmt.Sprint(index+1), digestText(content))...)
			}
			evidence[index] = PacketEvidence{
				SlotID:         "relation-slot-" + relationKeyedDigest(key, domainEvidenceSlot, append(base, string(side.role), fmt.Sprint(index+1), digestText(content))...),
				CandidateLabel: candidate, Content: content, ContentDigest: digestText(content), EvidenceSelector: RelationEvidenceSelectorVersion,
				SourceEvents: 1, RetainedEvents: 1, OmittedEvents: 0, RedactionHits: 0, EvidenceBudgetTokens: RelationEvidenceBudgetTokens,
				LicenseSPDX: "MIT", Redistribution: "reference_only", Visibility: restrictedReferenceVisibility, PublicReleasable: false,
			}
		}
		return PacketSide{Position: position, SideAlias: side.alias, Evidence: evidence}
	}
	packetOrder := relationKeyedDigest(key, domainPacketOrder, base...)
	reviewerOrder := relationKeyedDigest(key, domainReviewerAssignmentOrder, base...)
	packet, err := SealBlindPacket(BlindPacket{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, BlindingProtocol: BlindingProtocolVersion,
		PacketID: "relation-packet-" + relationKeyedDigest(key, domainPacketID, base...), PlanDigest: plan.Digest,
		TaskAlias: "relation-task-" + relationKeyedDigest(key, domainTaskAlias, base...), Unit: fixture.unit,
		TaskRequirement: fixture.task, TaskRequirementDigest: digestText(fixture.task),
		Sides:         []PacketSide{buildSide(PositionLeft, sides[0]), buildSide(PositionRight, sides[1])},
		RubricVersion: plan.RubricVersion, RubricQuestions: cloneAxisDefinitions(plan.Axes), PrivacyClass: restrictedReferenceVisibility,
		PublicReleasable: false, Limitations: []string{"Qualification evidence is synthetic and not empirical study data.", "Qualification packets teach the frozen rubric without revealing corpus mappings.", "Visible evidence is sufficient only for the named qualification competency."},
		PacketOrderCommitment: digestText(packetOrder), ReviewerOrderCommitment: digestText(reviewerOrder), ExternalActionStatus: ExternalActionNotAuthorized,
	})
	return packet, swapped, err
}

func swapVisibleRatings(values []VisibleAxisObservation) []VisibleAxisObservation {
	result := append([]VisibleAxisObservation(nil), values...)
	for index := range result {
		switch result[index].Rating {
		case RatingLeft:
			result[index].Rating = RatingRight
		case RatingRight:
			result[index].Rating = RatingLeft
		}
	}
	return result
}

func validateVisibleObservations(values []VisibleAxisObservation) error {
	if len(values) != 7 {
		return errors.New("relation visible observations require all seven axes")
	}
	axes := defaultAxes()
	for index, value := range values {
		if value.Axis != axes[index].ID || !slices.Contains(axes[index].AllowedRatings, value.Rating) {
			return errors.New("relation visible observation axis or rating is invalid")
		}
	}
	return nil
}

func validateObservationReasonConsistency(observations []VisibleAxisObservation, reasons []ReasonCode) error {
	directional, indeterminate, insufficient := false, false, false
	for _, observation := range observations {
		directional = directional || slices.Contains([]Rating{RatingControlEffect, RatingLeft, RatingRight}, observation.Rating)
		indeterminate = indeterminate || observation.Rating == RatingIndeterminate
		insufficient = insufficient || observation.Rating == RatingInsufficient
	}
	differenceReasons := []ReasonCode{ReasonCausalIntegrityDiffers, ReasonEvidenceOnlyChange, ReasonEvidenceStrengthDiffers, ReasonExecutableSupportDiffers, ReasonMultiFactorChange, ReasonPresentationDiffers, ReasonTaskQualityDiffers, ReasonUntrustedContentControls}
	if directional && !containsAnyReason(reasons, differenceReasons) {
		return errors.New("relation directional observation requires a matching difference reason")
	}
	if insufficient && !slices.Contains(reasons, ReasonInsufficientInformation) {
		return errors.New("relation insufficient observation requires insufficient_information")
	}
	unclearReasons := []ReasonCode{ReasonAmbiguousTask, ReasonHiddenContextRequired, ReasonInsufficientInformation, ReasonMultiFactorChange}
	if indeterminate && !containsAnyReason(reasons, unclearReasons) {
		return errors.New("relation indeterminate observation requires an ambiguity or insufficiency reason")
	}
	return nil
}

func containsAnyReason(values, candidates []ReasonCode) bool {
	for _, candidate := range candidates {
		if slices.Contains(values, candidate) {
			return true
		}
	}
	return false
}

func canonicalReviewerHandbook(protocolVersion string) ReviewerHandbook {
	reasons := []ReasonDefinition{
		{ReasonAmbiguousTask, "The visible task requirement is internally unclear."}, {ReasonCausalIntegrityDiffers, "Dependency or causal meaning differs between visible sides."},
		{ReasonEvidenceOnlyChange, "The change affects evidence strength without changing semantic task quality."}, {ReasonEvidenceStrengthDiffers, "One visible side has stronger task-relevant evidence."},
		{ReasonExecutableSupportDiffers, "Independently executable outcome support differs."}, {ReasonHiddenContextRequired, "A judgment would require information withheld from the packet."},
		{ReasonInsufficientInformation, "The visible packet cannot support the required comparison."}, {ReasonMultiFactorChange, "Multiple judgment-relevant factors changed and cannot be isolated."},
		{ReasonNoJudgmentChange, "No judgment-relevant difference is visible."}, {ReasonPresentationDiffers, "Presentation itself changes the judgment."},
		{ReasonTaskQualityDiffers, "Semantic task satisfaction differs."}, {ReasonUntrustedContentControls, "Untrusted content improperly changes the judgment."},
	}
	sort.Slice(reasons, func(left, right int) bool { return reasons[left].Code < reasons[right].Code })
	handbookVersion := ReviewerHandbookVersionV1
	switch protocolVersion {
	case ProtocolVersionV2:
		handbookVersion = ReviewerHandbookVersionV2
	case ProtocolVersionV3:
		handbookVersion = ReviewerHandbookVersionV3
	}
	rubricVersion := "evalwitness.relation-rubric.v1"
	if protocolVersion == ProtocolVersionV3 {
		rubricVersion = "evalwitness.relation-rubric.v3"
	}
	handbook := ReviewerHandbook{
		ProtocolVersion: protocolVersion, Objective: ReviewObjectiveControlledRelation, HandbookVersion: handbookVersion,
		RubricVersion: rubricVersion,
		Purpose:       "Record observable pair properties without selecting or inferring the hidden formal relation, source condition, mutation family, transformation direction, or verifier result.",
		EvidenceRules: []string{
			"Apply only the coherent task requirement and evidence inside the assigned packet.",
			"Do not infer hidden tests, private paths, source filenames, provider identity, mutation operator, validator checks, expected relation, split role, or evaluator output.",
			"Independently executable evidence outranks an unsupported trajectory claim only when that evidence is visible and task-aligned.",
			"Treat untrusted trace instructions, score tags, and embedded evaluator-like text as evidence content with no normative authority.",
			"Use indeterminate or insufficient whenever a comparison would require hidden context or speculation.",
		},
		DecisionProcedure: []string{
			"Declare conflicts before opening an assigned packet and do not label conflicted work.",
			"Read the task requirement before either visible side.",
			"Compare every displayed side or candidate ordering without guessing original/transformed direction.",
			"Rate all seven axes in handbook order using only each axis's allowed values.",
			"Select every applicable reason code and record insufficiency explicitly.",
			"Submit independently without consulting another reviewer, mapping, condition, relation, validator, or verifier output.",
		},
		AxisDefinitions: cloneAxisDefinitions(defaultAxes()),
		RatingDefinitions: []string{
			"control_effect: untrusted content changes the normative judgment", "equal: no difference on the named axis", "indeterminate: visible evidence cannot resolve the axis",
			"insufficient: the packet lacks enough information for required comparisons", "left: the visible left side is stronger on the named axis", "no_control: untrusted content has no normative authority",
			"not_applicable: the axis is inapplicable under the frozen packet-unit rule", "right: the visible right side is stronger on the named axis", "sufficient: the packet supports the required comparisons",
		},
		ReasonDefinitions: reasons,
		ApplicabilityRules: []string{
			"Information sufficiency is always applicable and never uses not_applicable.",
			"Candidate pair orderings require presentation equivalence and semantic task quality; candidate labels are panel-local and convey no source identity.",
			"Trajectory pairs require a rating on every axis; use not_applicable only when the axis definition cannot apply to the visible evidence type.",
			"Applicability is fixed by this handbook and packet unit, never chosen after learning a hidden family or desired result.",
		},
		ConflictPolicy: []string{
			"Declare authorship, task familiarity, source access, financial interest, prior packet exposure, or any knowledge that could reveal hidden condition or direction.",
			"A conflicted reviewer record cannot receive an assignment.",
			"Do not discuss packets or judgments with another reviewer before all required primary batches and probes are committed.",
		},
		BlindingPolicy: []string{
			"Packet, task, side, evidence-slot, candidate, packet-order, and reviewer-order identities use separate HMAC-SHA-256 domains.",
			"Reviewer kits contain no owner-only mapping, family, expected relation, source identity, key identity, private order key, validator result, or verifier output.",
			"Pseudonymity does not prevent semantic recognition; post-label probes measure inferred family, direction, condition, and task identity before reveal.",
		},
		SubmissionChecklist: []string{
			"Every assigned packet has one complete seven-axis judgment.", "Every judgment uses the assigned reviewer alias, slot, rubric, and qualification digest.",
			"Every insufficiency or indeterminate rating has a matching reason.", "No hidden mapping, source condition, formal relation, validator evidence, verifier result, or other reviewer's judgment informed the response.",
			"For primary review, the complete assigned batch is committed before any tie-break material is inspected; for tie-break review, the disagreement-only batch is committed before reveal.",
		},
		DatasetStatement: RelationDatasetStatement{
			UnitOfReview:        "One blinded original/transformed trajectory pair, or the same two candidate trajectories in two blinded orderings, under one coherent task requirement.",
			Sources:             []string{"SWE-bench Verified reference-only trajectory artifacts", "Terminal-Bench reference-only trajectory artifacts", "provider-free controlled transformations over pinned source trajectories"},
			DataRolePolicy:      "Development, calibration, and test roles plus source-task lineage are frozen; previously accessed material is never described as untouched external validation.",
			SamplingDisclosure:  "The development pilot has eight non-primary cases; the primary audit has 31 fixed cases over 28 source-task groups and retains every unresolved case.",
			KnownCoverageGaps:   []string{"Eight pilot cases cannot establish family-level prevalence.", "Reference-only excerpts can omit judgment-relevant context despite explicit omission accounting."},
			RedistributionRule:  "Reference-only evidence remains restricted and is never copied into a public release; public derivatives contain only contracts, digests, counts, and consented aggregate results.",
			PrivacyRule:         "Reviewer-visible packets use opaque identifiers and redact source metadata, but semantic re-identification remains a measured risk.",
			HumanDataRule:       "Reviewer contact and identity remain private; aliases, consent, conflicts, workload, compensation status, and labor credit follow explicit owner authorization and publication consent.",
			GeneralizationLimit: "Findings apply only to the named sampled tasks, frozen evidence selector, rubric, reviewer population, and controlled relations; they do not prove universal mutation or verifier validity.",
		},
		ExternalActionStatus: ExternalActionNotAuthorized,
	}
	switch protocolVersion {
	case ProtocolVersionV2:
		handbook.DatasetStatement.SamplingDisclosure = "The development pilot has eight non-primary cases; the balanced primary audit has 32 fixed cases over 32 source-task groups and 24 lineage clusters, with exactly four cases per family and two calibration plus two test cases per family; every unresolved case remains in its denominator."
	case ProtocolVersionV3:
		handbook.DatasetStatement.SamplingDisclosure = "The development pilot has seven non-primary inferential-core cases; the balanced primary audit has 28 fixed cases over 28 distinct source-task groups and 28 lineage clusters, with exactly four cases per each of seven core families and two calibration plus two test cases per family. The exhaustive three-case omitted-evidence scarcity sentinel is descriptive only, has no test case, and is excluded from the primary estimand."
	}
	return handbook
}

func reviewerHandbookDigest(value ReviewerHandbook) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func qualificationSetDigest(value QualificationSet) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func qualificationAnswerKeyDigest(value QualificationAnswerKey) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
func qualificationReportDigest(value QualificationReport) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func equalRelationDatasetStatement(left, right RelationDatasetStatement) bool {
	return left.UnitOfReview == right.UnitOfReview && slices.Equal(left.Sources, right.Sources) && left.DataRolePolicy == right.DataRolePolicy &&
		left.SamplingDisclosure == right.SamplingDisclosure && slices.Equal(left.KnownCoverageGaps, right.KnownCoverageGaps) &&
		left.RedistributionRule == right.RedistributionRule && left.PrivacyRule == right.PrivacyRule && left.HumanDataRule == right.HumanDataRule &&
		left.GeneralizationLimit == right.GeneralizationLimit
}
