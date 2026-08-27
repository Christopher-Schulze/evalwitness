package outcome

import (
	"errors"
	"fmt"
	"io"
	"slices"
)

const (
	ReviewerHandbookSchemaVersion = "evalwitness.outcome-reviewer-handbook.v1"
	ReviewerHandbookVersion       = "evalwitness.reviewer-handbook.v1"
)

type OutcomeDefinition struct {
	State            State  `json:"state"`
	Meaning          string `json:"meaning"`
	RequiredEvidence string `json:"required_evidence"`
}

type AxisDefinition struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}

type ReasonDefinition struct {
	Code    ReasonCode `json:"code"`
	Meaning string     `json:"meaning"`
}

type DatasetStatement struct {
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
	SchemaVersion          string              `json:"schema_version"`
	CanonicalPolicy        string              `json:"canonical_policy"`
	HandbookVersion        string              `json:"handbook_version"`
	RubricVersion          string              `json:"rubric_version"`
	QualificationSetDigest string              `json:"qualification_set_digest"`
	ExamplePacketID        string              `json:"example_packet_id"`
	Purpose                string              `json:"purpose"`
	EvidenceRules          []string            `json:"evidence_rules"`
	DecisionProcedure      []string            `json:"decision_procedure"`
	OutcomeDefinitions     []OutcomeDefinition `json:"outcome_definitions"`
	AxisDefinitions        []AxisDefinition    `json:"axis_definitions"`
	ReasonDefinitions      []ReasonDefinition  `json:"reason_definitions"`
	ConflictPolicy         []string            `json:"conflict_policy"`
	BlindingPolicy         []string            `json:"blinding_policy"`
	SubmissionChecklist    []string            `json:"submission_checklist"`
	DatasetStatement       DatasetStatement    `json:"dataset_statement"`
	Digest                 string              `json:"digest"`
}

func DefaultReviewerHandbook(qualification QualificationSet) (ReviewerHandbook, error) {
	if err := qualification.Validate(); err != nil {
		return ReviewerHandbook{}, err
	}
	policy := canonicalReviewerHandbookPolicy()
	policy.QualificationSetDigest = qualification.Digest
	policy.ExamplePacketID = qualification.Cases[0].Packet.PacketID
	return SealReviewerHandbook(policy)
}

func SealReviewerHandbook(handbook ReviewerHandbook) (ReviewerHandbook, error) {
	handbook.SchemaVersion = ReviewerHandbookSchemaVersion
	handbook.CanonicalPolicy = CanonicalPolicy
	handbook.Digest = ""
	digest, err := reviewerHandbookDigest(handbook)
	if err != nil {
		return ReviewerHandbook{}, err
	}
	handbook.Digest = digest
	return handbook, handbook.Validate()
}

func (handbook ReviewerHandbook) Validate() error {
	if handbook.SchemaVersion != ReviewerHandbookSchemaVersion || handbook.CanonicalPolicy != CanonicalPolicy ||
		handbook.HandbookVersion != ReviewerHandbookVersion || handbook.RubricVersion != "evalwitness.outcome-rubric.v1" ||
		!validDigest(handbook.QualificationSetDigest) || !validOpaquePacketID(handbook.ExamplePacketID) || missing(handbook.Purpose) {
		return errors.New("reviewer handbook identity, rubric, qualification, example, or purpose is invalid")
	}
	canonical := canonicalReviewerHandbookPolicy()
	if handbook.Purpose != canonical.Purpose || !slices.Equal(handbook.EvidenceRules, canonical.EvidenceRules) ||
		!slices.Equal(handbook.DecisionProcedure, canonical.DecisionProcedure) || !slices.Equal(handbook.OutcomeDefinitions, canonical.OutcomeDefinitions) ||
		!slices.Equal(handbook.AxisDefinitions, canonical.AxisDefinitions) || !slices.Equal(handbook.ReasonDefinitions, canonical.ReasonDefinitions) ||
		!slices.Equal(handbook.ConflictPolicy, canonical.ConflictPolicy) || !slices.Equal(handbook.BlindingPolicy, canonical.BlindingPolicy) ||
		!slices.Equal(handbook.SubmissionChecklist, canonical.SubmissionChecklist) || !equalDatasetStatement(handbook.DatasetStatement, canonical.DatasetStatement) {
		return errors.New("reviewer handbook policy does not match its frozen version")
	}
	expected, err := reviewerHandbookDigest(handbook)
	if err != nil || handbook.Digest != expected {
		return errors.New("reviewer handbook digest is invalid")
	}
	return nil
}

func VerifyReviewerHandbook(handbook ReviewerHandbook, qualification QualificationSet) error {
	if err := handbook.Validate(); err != nil {
		return err
	}
	if err := qualification.Validate(); err != nil {
		return err
	}
	if handbook.QualificationSetDigest != qualification.Digest || handbook.RubricVersion != qualification.RubricVersion {
		return errors.New("reviewer handbook does not bind the governed qualification set")
	}
	for _, item := range qualification.Cases {
		if item.Packet.PacketID == handbook.ExamplePacketID {
			return nil
		}
	}
	return errors.New("reviewer handbook example packet is absent from the governed qualification set")
}

func DecodeReviewerHandbook(reader io.Reader) (ReviewerHandbook, error) {
	var value ReviewerHandbook
	if err := decodeStrict(reader, &value); err != nil {
		return ReviewerHandbook{}, fmt.Errorf("decode reviewer handbook: %w", err)
	}
	return value, value.Validate()
}

func canonicalReviewerHandbookPolicy() ReviewerHandbook {
	return ReviewerHandbook{
		HandbookVersion: ReviewerHandbookVersion,
		RubricVersion:   "evalwitness.outcome-rubric.v1",
		Purpose:         "Determine the task outcome supported by the supplied blinded evidence without consulting evaluator outputs, source mappings, study conditions, or expected relations.",
		EvidenceRules: []string{
			"Apply only the task requirement and evidence contained in the assigned blinded packet.",
			"Treat trajectory claims and benchmark rewards as assertions unless independent evidence in the packet supports them.",
			"Do not infer missing tests, hidden requirements, provider identity, source condition, or expected answer.",
			"Use indeterminate when the available evidence is conflicting, incomplete, or insufficient for a decisive task outcome.",
		},
		DecisionProcedure: []string{
			"Declare any conflict of interest before examining the packet and stop labeling if a conflict exists.",
			"Decide whether the task specification is coherent and answerable from the supplied requirements.",
			"Separate ordinary task failure from timeout, launch, output-limit, cleanup, or other environment failure.",
			"Rate task satisfaction, technical correctness, verification quality, harmful side effects, and evidence sufficiency independently.",
			"Select exactly one supported primary outcome and every applicable reason code.",
			"Submit the label without consulting another reviewer, the private mapping, evaluator output, or study hypothesis.",
		},
		OutcomeDefinitions: []OutcomeDefinition{
			{State: StateSolved, Meaning: "The available evidence supports that the implementation satisfies the coherent task requirements without a task-defeating defect.", RequiredEvidence: "Task-aligned implementation evidence with sufficient verification or an equivalent decisive independent basis."},
			{State: StateUnsolved, Meaning: "The available evidence supports a task-relevant implementation failure or unmet coherent requirement.", RequiredEvidence: "A task-linked defect, failed independent check, or other decisive contradiction that is not merely an infrastructure failure."},
			{State: StateIndeterminate, Meaning: "The supplied evidence cannot support either solved or unsolved without speculation.", RequiredEvidence: "Conflicting, incomplete, unverifiable, or insufficient evidence after applying the full rubric."},
			{State: StateInvalidTask, Meaning: "The task requirements are internally inconsistent, undefined, or impossible to adjudicate as written.", RequiredEvidence: "A concrete contradiction or missing requirement that prevents a coherent outcome judgment."},
			{State: StateEnvironmentFail, Meaning: "The adjudication evidence failed because the execution environment or bounded runner failed, not because the task was disproved.", RequiredEvidence: "A recorded timeout, launch, output-limit, cleanup, or equivalent infrastructure failure."},
		},
		AxisDefinitions: []AxisDefinition{
			{Name: "task_satisfaction", Meaning: "Whether the implementation satisfies every coherent task requirement visible in the packet."},
			{Name: "technical_correctness", Meaning: "Whether the implementation behavior and reasoning are technically sound for the stated task."},
			{Name: "verification_quality", Meaning: "Whether the supplied checks are task-aligned, independently credible, and complete enough for the claim."},
			{Name: "harmful_side_effects", Meaning: "Whether the implementation introduces behavior that defeats the task despite local success."},
			{Name: "evidence_sufficiency", Meaning: "Whether the packet contains enough trustworthy evidence for the selected primary outcome."},
		},
		ReasonDefinitions: []ReasonDefinition{
			{Code: ReasonClaimedOnly, Meaning: "Success or failure is asserted but not independently supported."},
			{Code: ReasonEnvironmentFailure, Meaning: "Infrastructure prevented a task-level conclusion."},
			{Code: ReasonEvidenceConflict, Meaning: "Material evidence sources disagree."},
			{Code: ReasonEvidenceConsistent, Meaning: "Independent evidence sources support the same conclusion."},
			{Code: ReasonEvidenceInsufficient, Meaning: "The packet lacks enough trustworthy evidence for a decisive outcome."},
			{Code: ReasonFormalRelationSupports, Meaning: "A scoped formal relation supports the conclusion."},
			{Code: ReasonHarmfulSideEffect, Meaning: "A side effect defeats or materially compromises the task."},
			{Code: ReasonIndependentTestsFail, Meaning: "Task-aligned independent checks fail."},
			{Code: ReasonIndependentTestsPass, Meaning: "Task-aligned independent checks pass."},
			{Code: ReasonInvalidTask, Meaning: "The task specification cannot support a coherent judgment."},
			{Code: ReasonTaskSatisfied, Meaning: "The coherent task requirements are satisfied."},
			{Code: ReasonTaskUnsatisfied, Meaning: "At least one coherent task requirement is unmet."},
			{Code: ReasonTechnicalDefect, Meaning: "A task-relevant technical defect is present."},
			{Code: ReasonVerificationComplete, Meaning: "Verification adequately covers the decisive requirements."},
			{Code: ReasonVerificationIncomplete, Meaning: "Verification omits a decisive requirement or failure mode."},
		},
		ConflictPolicy: []string{
			"Declare authorship, task familiarity, source access, financial interest, or prior exposure that could reveal the condition or expected answer.",
			"Do not label an assignment after declaring a conflict; the study owner must replace the reviewer or packet assignment.",
			"Do not discuss assigned packets or labels with another reviewer before all required label batches are committed.",
			"Record newly discovered conflicts before submission; a conflict-free batch cannot contain a conflicted label.",
		},
		BlindingPolicy: []string{
			"Reviewer packets omit evaluator identity, evaluator output, confidence, route, source condition, expected relation, and private source mapping.",
			"Packet and evidence-slot identities are domain-separated HMAC values; pseudonymity is not anonymity against prior knowledge of the task universe.",
			"Assignment ordering seeds and private mapping digests remain concealed until every required label batch is committed.",
			"Semantic leakage remains a measured validity risk even when lexical leakage checks pass.",
		},
		SubmissionChecklist: []string{
			"Every assigned packet has exactly one sealed label.",
			"Every label uses the assignment rubric, reviewer alias, slot, and qualification digest.",
			"Every axis is rated and every applicable reason code is included.",
			"No label was informed by another reviewer, evaluator output, mapping, condition, relation, or expected answer.",
			"The complete batch is committed before any reveal or tie-break result is inspected.",
		},
		DatasetStatement: DatasetStatement{
			UnitOfReview:        "One blinded coding-agent trajectory evidence packet under one coherent task requirement.",
			Sources:             []string{"SWE-bench Verified trajectory artifacts previously accessed for development", "Terminal-Bench trajectory artifacts previously accessed for development", "provider-free controlled corruption cases derived from licensed public trajectory artifacts"},
			DataRolePolicy:      "Development, calibration, and test roles are explicit and immutable; previously accessed artifacts are never described as untouched external validation.",
			SamplingDisclosure:  "The governed design uses a 31-case controlled-corruption audit sample and a 48-case natural target with global task-group deduplication and no synthetic or cross-stratum replacement.",
			KnownCoverageGaps:   []string{"No explicit abstention rows exist in the current natural development inventory.", "No explicit provider-failure rows exist in the current natural development inventory."},
			RedistributionRule:  "Only public-releasable licensed evidence or redacted excerpts with content digests may enter a public bundle; restricted evidence remains outside public artifacts.",
			PrivacyRule:         "Public packets use opaque HMAC-derived identifiers and exclude private mappings, but source inference from semantic cues remains possible and must be measured.",
			HumanDataRule:       "Reviewer identity and contact details remain private; public artifacts use consented pseudonymous aliases and credit real research labor under the authorship policy.",
			GeneralizationLimit: "Adjudication supports only the named tasks, supplied evidence, frozen rubric, sampled strata, and recorded reviewer population; it does not establish universal coding-agent or verifier quality.",
		},
	}
}

func equalDatasetStatement(left, right DatasetStatement) bool {
	return left.UnitOfReview == right.UnitOfReview && slices.Equal(left.Sources, right.Sources) && left.DataRolePolicy == right.DataRolePolicy &&
		left.SamplingDisclosure == right.SamplingDisclosure && slices.Equal(left.KnownCoverageGaps, right.KnownCoverageGaps) &&
		left.RedistributionRule == right.RedistributionRule && left.PrivacyRule == right.PrivacyRule && left.HumanDataRule == right.HumanDataRule &&
		left.GeneralizationLimit == right.GeneralizationLimit
}

func reviewerHandbookDigest(value ReviewerHandbook) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
