package outcome

const (
	PlanSchemaVersion                = "evalwitness.outcome-adjudication-plan.v1"
	OutcomeSchemaVersion             = "evalwitness.outcome-record.v1"
	PacketSchemaVersion              = "evalwitness.outcome-blind-packet.v1"
	MappingSchemaVersion             = "evalwitness.outcome-private-mapping.v1"
	LabelSchemaVersion               = "evalwitness.outcome-blinded-label.v1"
	ResolutionSchemaVersion          = "evalwitness.outcome-resolution.v1"
	AgreementSchemaVersion           = "evalwitness.outcome-agreement.v1"
	PreservationSchemaVersion        = "evalwitness.outcome-preservation.v1"
	BlindBuildSchemaVersion          = "evalwitness.outcome-blind-build-request.v1"
	QualificationSchemaVersion       = "evalwitness.outcome-qualification-set.v1"
	QualificationReportSchemaVersion = "evalwitness.outcome-qualification-report.v1"
	CanonicalPolicy                  = "evalwitness.outcome-canonical-json.v1"
	MaximumDocumentSize              = 16 * 1024 * 1024
)

type State string

const (
	StateSolved          State = "solved"
	StateUnsolved        State = "unsolved"
	StateIndeterminate   State = "indeterminate"
	StateInvalidTask     State = "invalid_task"
	StateEnvironmentFail State = "environment_failed"
	StateNotAdjudicated  State = "not_adjudicated"
)

type EvidenceKind string

const (
	EvidenceClaimedTest     EvidenceKind = "claimed_test_output"
	EvidenceBenchmarkReward EvidenceKind = "benchmark_reward"
	EvidenceIndependentRun  EvidenceKind = "independent_test_rerun"
	EvidenceFormalRelation  EvidenceKind = "formal_mutation_relation"
	EvidenceHumanLabel      EvidenceKind = "human_adjudication"
)

type AxisRating string

const (
	RatingSufficient    AxisRating = "sufficient"
	RatingInsufficient  AxisRating = "insufficient"
	RatingUnclear       AxisRating = "unclear"
	RatingNotApplicable AxisRating = "not_applicable"
)

type ReasonCode string

const (
	ReasonTaskSatisfied          ReasonCode = "task_satisfied"
	ReasonTaskUnsatisfied        ReasonCode = "task_unsatisfied"
	ReasonTechnicalDefect        ReasonCode = "technical_defect"
	ReasonVerificationComplete   ReasonCode = "verification_complete"
	ReasonVerificationIncomplete ReasonCode = "verification_incomplete"
	ReasonHarmfulSideEffect      ReasonCode = "harmful_side_effect"
	ReasonEvidenceConsistent     ReasonCode = "evidence_consistent"
	ReasonEvidenceConflict       ReasonCode = "evidence_conflict"
	ReasonEvidenceInsufficient   ReasonCode = "evidence_insufficient"
	ReasonIndependentTestsPass   ReasonCode = "independent_tests_pass"
	ReasonIndependentTestsFail   ReasonCode = "independent_tests_fail"
	ReasonEnvironmentFailure     ReasonCode = "environment_failure"
	ReasonInvalidTask            ReasonCode = "invalid_task"
	ReasonClaimedOnly            ReasonCode = "claimed_only"
	ReasonFormalRelationSupports ReasonCode = "formal_relation_supports"
)

type Plan struct {
	SchemaVersion              string   `json:"schema_version"`
	CanonicalPolicy            string   `json:"canonical_policy"`
	ProtocolVersion            string   `json:"protocol_version"`
	SourceCorpusDigest         string   `json:"source_corpus_digest"`
	MutationSampleSize         int      `json:"mutation_sample_size"`
	NaturalSampleSize          int      `json:"natural_sample_size"`
	RequiredStrata             []string `json:"required_strata"`
	MutationFamilies           []string `json:"mutation_families"`
	SamplingRule               string   `json:"sampling_rule"`
	ReplacementRule            string   `json:"replacement_rule"`
	PrimaryAdjudicators        int      `json:"primary_adjudicators"`
	TieBreakAdjudicators       int      `json:"tie_break_adjudicators"`
	BlindingRule               string   `json:"blinding_rule"`
	RubricVersion              string   `json:"rubric_version"`
	AgreementMetrics           []string `json:"agreement_metrics"`
	BootstrapIterations        int      `json:"bootstrap_iterations"`
	BootstrapSeed              string   `json:"bootstrap_seed"`
	ConflictRule               string   `json:"conflict_rule"`
	OutcomeResolutionRule      string   `json:"outcome_resolution_rule"`
	SensitivityAnalysis        string   `json:"sensitivity_analysis"`
	PublicPacketPolicy         string   `json:"public_packet_policy"`
	PrivateMappingPolicy       string   `json:"private_mapping_policy"`
	RecruitmentRequiresConsent bool     `json:"recruitment_requires_consent"`
	Digest                     string   `json:"digest"`
}

type Evidence struct {
	ID             string       `json:"id"`
	Kind           EvidenceKind `json:"kind"`
	State          State        `json:"state"`
	ArtifactDigest string       `json:"artifact_digest"`
	ValidatorID    string       `json:"validator_id,omitempty"`
	ObservedAt     string       `json:"observed_at"`
	Independent    bool         `json:"independent"`
	Public         bool         `json:"public"`
	Limitation     string       `json:"limitation"`
	ParentDigests  []string     `json:"parent_digests"`
	Digest         string       `json:"digest"`
}

type EvidenceDraft struct {
	ID             string       `json:"id"`
	Kind           EvidenceKind `json:"kind"`
	State          State        `json:"state"`
	ArtifactDigest string       `json:"artifact_digest"`
	ValidatorID    string       `json:"validator_id,omitempty"`
	ObservedAt     string       `json:"observed_at"`
	Independent    bool         `json:"independent"`
	Public         bool         `json:"public"`
	Limitation     string       `json:"limitation"`
	ParentDigests  []string     `json:"parent_digests"`
}

type Record struct {
	SchemaVersion   string     `json:"schema_version"`
	CanonicalPolicy string     `json:"canonical_policy"`
	RecordID        string     `json:"record_id"`
	TaskAlias       string     `json:"task_alias"`
	Revision        int        `json:"revision"`
	ParentDigest    string     `json:"parent_digest,omitempty"`
	Evidence        []Evidence `json:"evidence"`
	Resolution      State      `json:"resolution"`
	ResolutionBasis []string   `json:"resolution_basis"`
	Limitations     []string   `json:"limitations"`
	AuthorID        string     `json:"author_id"`
	RevisionReason  string     `json:"revision_reason"`
	Digest          string     `json:"digest"`
}

type RecordDraft struct {
	TaskAlias       string     `json:"task_alias"`
	Revision        int        `json:"revision"`
	ParentDigest    string     `json:"parent_digest,omitempty"`
	Evidence        []Evidence `json:"evidence"`
	Resolution      State      `json:"resolution"`
	ResolutionBasis []string   `json:"resolution_basis"`
	Limitations     []string   `json:"limitations"`
	AuthorID        string     `json:"author_id"`
	RevisionReason  string     `json:"revision_reason"`
}

type PacketEvidence struct {
	Slot          string `json:"slot"`
	Kind          string `json:"kind"`
	Content       string `json:"content,omitempty"`
	ContentDigest string `json:"content_digest"`
	License       string `json:"license"`
	Limitation    string `json:"limitation"`
}

type BlindPacket struct {
	SchemaVersion    string           `json:"schema_version"`
	CanonicalPolicy  string           `json:"canonical_policy"`
	PacketID         string           `json:"packet_id"`
	PlanDigest       string           `json:"plan_digest"`
	TaskAlias        string           `json:"task_alias"`
	Evidence         []PacketEvidence `json:"evidence"`
	RubricQuestions  []string         `json:"rubric_questions"`
	PrivacyClass     string           `json:"privacy_class"`
	PublicReleasable bool             `json:"public_releasable"`
	Digest           string           `json:"digest"`
}

type SlotMapping struct {
	Slot         string `json:"slot"`
	SourceDigest string `json:"source_digest"`
}

type BlindBuildRequest struct {
	SchemaVersion    string           `json:"schema_version"`
	PlanDigest       string           `json:"plan_digest"`
	TaskAlias        string           `json:"task_alias"`
	Evidence         []PacketEvidence `json:"evidence"`
	RubricQuestions  []string         `json:"rubric_questions"`
	PrivacyClass     string           `json:"privacy_class"`
	PublicReleasable bool             `json:"public_releasable"`
	SourceCaseDigest string           `json:"source_case_digest"`
	Condition        string           `json:"condition"`
	ExpectedRelation string           `json:"expected_relation"`
	SlotMappings     []SlotMapping    `json:"slot_mappings"`
	BlindingKeyID    string           `json:"blinding_key_id"`
	ForbiddenValues  []string         `json:"forbidden_values"`
}

type PrivateMapping struct {
	SchemaVersion    string        `json:"schema_version"`
	CanonicalPolicy  string        `json:"canonical_policy"`
	PacketID         string        `json:"packet_id"`
	SourceTaskAlias  string        `json:"source_task_alias"`
	SourceCaseDigest string        `json:"source_case_digest"`
	Condition        string        `json:"condition"`
	ExpectedRelation string        `json:"expected_relation"`
	SlotMappings     []SlotMapping `json:"slot_mappings"`
	BlindingKeyID    string        `json:"blinding_key_id"`
	Digest           string        `json:"digest"`
}

type Label struct {
	SchemaVersion        string       `json:"schema_version"`
	CanonicalPolicy      string       `json:"canonical_policy"`
	LabelID              string       `json:"label_id"`
	PacketID             string       `json:"packet_id"`
	AdjudicatorAlias     string       `json:"adjudicator_alias"`
	ReviewerSlot         int          `json:"reviewer_slot"`
	PrimaryOutcome       State        `json:"primary_outcome"`
	TaskSatisfaction     AxisRating   `json:"task_satisfaction"`
	TechnicalCorrectness AxisRating   `json:"technical_correctness"`
	VerificationQuality  AxisRating   `json:"verification_quality"`
	HarmfulSideEffects   AxisRating   `json:"harmful_side_effects"`
	EvidenceSufficiency  AxisRating   `json:"evidence_sufficiency"`
	ReasonCodes          []ReasonCode `json:"reason_codes"`
	SubmittedAt          string       `json:"submitted_at"`
	RubricVersion        string       `json:"rubric_version"`
	QualificationDigest  string       `json:"qualification_digest"`
	ConflictsOfInterest  []string     `json:"conflicts_of_interest"`
	Digest               string       `json:"digest"`
}

type LabelDraft struct {
	PacketID             string       `json:"packet_id"`
	AdjudicatorAlias     string       `json:"adjudicator_alias"`
	ReviewerSlot         int          `json:"reviewer_slot"`
	PrimaryOutcome       State        `json:"primary_outcome"`
	TaskSatisfaction     AxisRating   `json:"task_satisfaction"`
	TechnicalCorrectness AxisRating   `json:"technical_correctness"`
	VerificationQuality  AxisRating   `json:"verification_quality"`
	HarmfulSideEffects   AxisRating   `json:"harmful_side_effects"`
	EvidenceSufficiency  AxisRating   `json:"evidence_sufficiency"`
	ReasonCodes          []ReasonCode `json:"reason_codes"`
	SubmittedAt          string       `json:"submitted_at"`
	RubricVersion        string       `json:"rubric_version"`
	QualificationDigest  string       `json:"qualification_digest"`
	ConflictsOfInterest  []string     `json:"conflicts_of_interest"`
}

type Resolution struct {
	SchemaVersion       string   `json:"schema_version"`
	CanonicalPolicy     string   `json:"canonical_policy"`
	ResolutionID        string   `json:"resolution_id"`
	PacketID            string   `json:"packet_id"`
	PrimaryLabelDigests []string `json:"primary_label_digests"`
	TieBreakLabelDigest string   `json:"tie_break_label_digest,omitempty"`
	State               State    `json:"state"`
	AgreementState      string   `json:"agreement_state"`
	ResolvedAt          string   `json:"resolved_at"`
	Rule                string   `json:"rule"`
	Digest              string   `json:"digest"`
}

type Preservation struct {
	SchemaVersion           string   `json:"schema_version"`
	CanonicalPolicy         string   `json:"canonical_policy"`
	SourceOutcomeDigest     string   `json:"source_outcome_digest"`
	IntervenedOutcomeDigest string   `json:"intervened_outcome_digest"`
	SourceState             State    `json:"source_state"`
	IntervenedState         State    `json:"intervened_state"`
	Mechanism               string   `json:"mechanism"`
	Admissible              bool     `json:"admissible"`
	EvaluatorBlind          bool     `json:"evaluator_blind"`
	InadmissibilityReasons  []string `json:"inadmissibility_reasons"`
	Digest                  string   `json:"digest"`
}
