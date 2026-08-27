// Package stress defines the executable contracts for metamorphic and
// differential verifier audits. It binds existing mutation, construct, and
// score-evidence identities for shared verification entrypoints; it does not
// introduce another judge.
package stress

import (
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const (
	CanonicalPolicy                          = "evalwitness.stress-canonical-json.v1"
	RelationSchemaVersion                    = "evalwitness.stress-relation.v1"
	ResultSchemaVersion                      = "evalwitness.stress-result.v1"
	StageTraceSchemaVersion                  = "evalwitness.stress-stage-trace.v1"
	StageComparisonSchemaVersion             = "evalwitness.stress-stage-comparison.v1"
	CounterexampleSchemaVersion              = "evalwitness.stress-counterexample.v1"
	AdmissionSchemaVersion                   = "evalwitness.stress-construct-admission.v1"
	ReductionObservationSchemaVersion        = "evalwitness.stress-reduction-observation.v1"
	RelationRegistrySchemaVersion            = "evalwitness.stress-relation-registry.v1"
	ReplayBatchEvidenceSchemaVersion         = "evalwitness.stress-replay-batch-evidence.v1"
	ReplayPairEvidenceSchemaVersion          = "evalwitness.stress-replay-pair-evidence.v1"
	ReplayExecutionSchemaVersion             = "evalwitness.stress-replay-execution.v1"
	ArmComparisonPlanSchemaVersion           = "evalwitness.stress-arm-comparison-plan.v1"
	ZeroCostExecutionSchemaVersion           = "evalwitness.stress-zero-cost-execution.v1"
	ArmComparisonReportSchemaVersion         = "evalwitness.stress-arm-comparison-report.v1"
	ProtocolAdapterProofSchemaVersion        = "evalwitness.stress-protocol-adapter-proof.v1"
	ArmReplayEvidenceSchemaVersion           = "evalwitness.stress-arm-replay-evidence.v1"
	DevelopmentCaseStudySchemaVersion        = "evalwitness.stress-development-case-study.v1"
	DevelopmentChallengeSchemaVersion        = "evalwitness.stress-development-challenge.v1"
	DevelopmentChallengeReceiptSchemaVersion = "evalwitness.stress-development-challenge-receipt.v1"
	RelationCatalogVersionV3                 = "evalwitness.controlled-corruption-stress-catalog.v1"
)

type RelationKind string

const (
	KindInvariance   RelationKind = "invariance"
	KindSensitivity  RelationKind = "sensitivity"
	KindDifferential RelationKind = "differential"
)

type Unit string

const (
	UnitTrajectory       Unit = "trajectory"
	UnitCandidatePair    Unit = "candidate_pair"
	UnitTraceMapping     Unit = "trace_mapping"
	UnitEntrypoint       Unit = "entrypoint"
	UnitProviderRoute    Unit = "provider_route"
	UnitExtractionPolicy Unit = "extraction_policy"
)

type TransformKind string

const (
	TransformMutation       TransformKind = "mutation"
	TransformTraceMapping   TransformKind = "trace_mapping"
	TransformEntrypoint     TransformKind = "entrypoint"
	TransformProviderRoute  TransformKind = "provider_route"
	TransformExtractionMode TransformKind = "extraction_mode"
)

type SourceRequirementKind string

const (
	RequirementV3Manifest          SourceRequirementKind = "v3_mutation_manifest"
	RequirementV3ConstructFirewall SourceRequirementKind = "v3_construct_firewall"
	RequirementFormalWitness       SourceRequirementKind = "formal_witness"
	RequirementExactReplay         SourceRequirementKind = "exact_replay"
	RequirementOwnerAttestation    SourceRequirementKind = "owner_attestation"
	RequirementTerminalLedger      SourceRequirementKind = "terminal_human_ledger"
	RequirementOutcomeProof        SourceRequirementKind = "outcome_proof"
	RequirementPublicFixture       SourceRequirementKind = "public_release_fixture"
	RequirementLiveAuthorization   SourceRequirementKind = "live_authorization"
	RequirementCapsule             SourceRequirementKind = "experiment_capsule"
)

type InvalidState string

const (
	InvalidNotApplicable       InvalidState = "not_applicable"
	InvalidSourceUnavailable   InvalidState = "source_unavailable"
	InvalidFormalWitness       InvalidState = "formal_witness_invalid"
	InvalidConstructRejected   InvalidState = "construct_rejected"
	InvalidCustody             InvalidState = "custody_invalid"
	InvalidHumanContradicted   InvalidState = "human_contradicted"
	InvalidTransform           InvalidState = "transform_invalid"
	InvalidReplayMismatch      InvalidState = "replay_mismatch"
	InvalidPrivacy             InvalidState = "privacy_invalid"
	InvalidCrossVersion        InvalidState = "cross_version_substitution"
	InvalidLockedPartitionUsed InvalidState = "locked_partition_already_used"
)

type Metric string

const (
	MetricDecision                Metric = "decision"
	MetricRank                    Metric = "rank"
	MetricConditionalScore        Metric = "conditional_expected_score"
	MetricConditionalVariance     Metric = "conditional_variance"
	MetricSupportJaccard          Metric = "support_jaccard"
	MetricProbabilityOverlap      Metric = "probability_overlap"
	MetricCommonSupportDivergence Metric = "common_support_divergence"
	MetricVisibleMass             Metric = "visible_probability_mass"
	MetricValidMass               Metric = "valid_score_mass"
	MetricUnobservedMass          Metric = "unobserved_probability_mass"
)

type Operator string

const (
	OperatorEqual                Operator = "equal"
	OperatorNotEqual             Operator = "not_equal"
	OperatorLessOrEqual          Operator = "less_or_equal"
	OperatorGreaterOrEqual       Operator = "greater_or_equal"
	OperatorOriginalPreferred    Operator = "original_preferred"
	OperatorTransformedPreferred Operator = "transformed_preferred"
)

type RepeatKind string

const (
	RepeatFixed              RepeatKind = "fixed"
	RepeatRegisteredAdaptive RepeatKind = "registered_adaptive"
)

type Estimand string

const (
	EstimandPrimaryCore      Estimand = "primary_core"
	EstimandScarcitySentinel Estimand = "scarcity_sentinel"
	EstimandSensitivity      Estimand = "sensitivity"
	EstimandDiagnostic       Estimand = "diagnostic"
)

type DenominatorPolicy string

const (
	DenominatorPrimaryHumanSupported DenominatorPolicy = "human_supported_source_task_relations_with_all_post_admission_outcomes"
	DenominatorSensitivityStratified DenominatorPolicy = "eligible_source_task_relations_stratified_by_admission_with_all_post_admission_outcomes"
	DenominatorScarcityAvailability  DenominatorPolicy = "available_sentinel_relations_over_40_target_source_tasks_with_all_post_admission_outcomes"
)

const FailureDenominatorTreatment = "retain_every_registered_case_by_admission_and_outcome_without_complete_case_deletion"

type Stage string

const (
	StageIngestion           Stage = "ingestion"
	StageRequestConstruction Stage = "request_construction"
	StageProviderResponse    Stage = "provider_response"
	StageScoreExtraction     Stage = "score_extraction"
	StageDecisionPolicy      Stage = "decision_policy"
	StageRendering           Stage = "rendering"
)

type StageExpectationKind string

const (
	StageMustMatch  StageExpectationKind = "must_match"
	StageMustDiffer StageExpectationKind = "must_differ"
	StageMayDiffer  StageExpectationKind = "may_differ"
)

type Outcome string

const (
	OutcomeSatisfied      Outcome = "satisfied"
	OutcomeViolated       Outcome = "violated"
	OutcomeAbstained      Outcome = "abstained"
	OutcomeInvalid        Outcome = "invalid"
	OutcomeUnsupported    Outcome = "unsupported"
	OutcomeProviderFailed Outcome = "provider_failed"
	OutcomeInconclusive   Outcome = "inconclusive"
)

type ConstraintStatus string

const (
	ConstraintSatisfied    ConstraintStatus = "satisfied"
	ConstraintViolated     ConstraintStatus = "violated"
	ConstraintAbstained    ConstraintStatus = "abstained"
	ConstraintUnsupported  ConstraintStatus = "unsupported"
	ConstraintInconclusive ConstraintStatus = "inconclusive"
)

type AdmissionStatus string

const (
	AdmissionFormalOnly        AdmissionStatus = "formal_only"
	AdmissionHumanSupported    AdmissionStatus = "human_supported"
	AdmissionHumanContradicted AdmissionStatus = "human_contradicted"
	AdmissionHumanUnresolved   AdmissionStatus = "human_unresolved"
)

type SourceRequirement struct {
	Kind  SourceRequirementKind `json:"kind"`
	Value string                `json:"value"`
}

type Applicability struct {
	Unit                  Unit                      `json:"unit"`
	MinimumTrajectories   int                       `json:"minimum_trajectories"`
	MaximumTrajectories   int                       `json:"maximum_trajectories"`
	RequiredSourceFormats []preprocess.SourceFormat `json:"required_source_formats"`
	Requirements          []SourceRequirement       `json:"requirements"`
}

type Transform struct {
	Kind                   TransformKind              `json:"kind"`
	Identifier             string                     `json:"identifier"`
	Version                string                     `json:"version"`
	MutationFamily         mutation.Family            `json:"mutation_family,omitempty"`
	InterventionClass      mutation.InterventionClass `json:"intervention_class,omitempty"`
	ExpectedFormalRelation mutation.Relation          `json:"expected_formal_relation,omitempty"`
	DeclaredChangedLayer   Stage                      `json:"declared_changed_layer"`
}

type ExpectedConstraint struct {
	ID                string   `json:"id"`
	Metric            Metric   `json:"metric"`
	Operator          Operator `json:"operator"`
	TargetValue       *float64 `json:"target_value,omitempty"`
	AbsoluteTolerance float64  `json:"absolute_tolerance"`
	MinimumEffect     float64  `json:"minimum_effect"`
	TargetState       string   `json:"target_state,omitempty"`
	Required          bool     `json:"required"`
}

type RepeatPolicy struct {
	Kind               RepeatKind `json:"kind"`
	MinimumRepetitions int        `json:"minimum_repetitions"`
	MaximumRepetitions int        `json:"maximum_repetitions"`
	StopRule           string     `json:"stop_rule"`
}

type FailurePolicy struct {
	Invalid           Outcome `json:"invalid"`
	MissingScore      Outcome `json:"missing_score"`
	ProviderFailure   Outcome `json:"provider_failure"`
	RouteFailure      Outcome `json:"route_failure"`
	Timeout           Outcome `json:"timeout"`
	Abstention        Outcome `json:"abstention"`
	BudgetExhaustion  Outcome `json:"budget_exhaustion"`
	RetryExhaustion   Outcome `json:"retry_exhaustion"`
	IncompleteCell    Outcome `json:"incomplete_cell"`
	Unsupported       Outcome `json:"unsupported"`
	DenominatorPolicy string  `json:"denominator_policy"`
}

type StatisticalFamily struct {
	ID                 string            `json:"id"`
	Estimand           Estimand          `json:"estimand"`
	ClusterUnit        string            `json:"cluster_unit"`
	MultiplicityMethod string            `json:"multiplicity_method"`
	DenominatorPolicy  DenominatorPolicy `json:"denominator_policy"`
	FailurePolicy      FailurePolicy     `json:"failure_policy"`
}

type StageExpectation struct {
	Stage       Stage                `json:"stage"`
	Expectation StageExpectationKind `json:"expectation"`
}

type Relation struct {
	SchemaVersion     string               `json:"schema_version"`
	CanonicalPolicy   string               `json:"canonical_policy"`
	ID                string               `json:"id"`
	Revision          int                  `json:"revision"`
	Kind              RelationKind         `json:"kind"`
	Applicability     Applicability        `json:"applicability"`
	Transform         Transform            `json:"transform"`
	Constraints       []ExpectedConstraint `json:"constraints"`
	InvalidStates     []InvalidState       `json:"invalid_states"`
	Repeat            RepeatPolicy         `json:"repeat_policy"`
	StatisticalFamily StatisticalFamily    `json:"statistical_family"`
	StageExpectations []StageExpectation   `json:"stage_expectations"`
	Digest            string               `json:"digest"`
}

type StageRecord struct {
	Stage  Stage  `json:"stage"`
	Unit   string `json:"unit"`
	Digest string `json:"digest"`
}

type StageTrace struct {
	SchemaVersion   string        `json:"schema_version"`
	CanonicalPolicy string        `json:"canonical_policy"`
	Side            string        `json:"side"`
	Records         []StageRecord `json:"records"`
	Digest          string        `json:"digest"`
}

type StageDifference struct {
	Stage       Stage                `json:"stage"`
	Expectation StageExpectationKind `json:"expectation"`
	LeftDigest  string               `json:"left_digest,omitempty"`
	RightDigest string               `json:"right_digest,omitempty"`
	Divergent   bool                 `json:"divergent"`
	Unexpected  bool                 `json:"unexpected"`
}

type StageComparison struct {
	SchemaVersion           string            `json:"schema_version"`
	CanonicalPolicy         string            `json:"canonical_policy"`
	RelationDigest          string            `json:"relation_digest"`
	LeftTraceDigest         string            `json:"left_trace_digest"`
	RightTraceDigest        string            `json:"right_trace_digest"`
	Differences             []StageDifference `json:"differences"`
	EarliestDivergentStage  Stage             `json:"earliest_divergent_stage,omitempty"`
	EarliestUnexpectedStage Stage             `json:"earliest_unexpected_stage,omitempty"`
	CausalityClaim          string            `json:"causality_claim"`
	Digest                  string            `json:"digest"`
}

type ConstructAdmission struct {
	SchemaVersion           string          `json:"schema_version"`
	CanonicalPolicy         string          `json:"canonical_policy"`
	CaseID                  string          `json:"case_id"`
	FormalWitnessDigest     string          `json:"formal_witness_digest"`
	ConstructFirewallDigest string          `json:"construct_firewall_digest"`
	OwnerAttestationDigest  string          `json:"owner_attestation_digest"`
	TerminalLedgerDigest    string          `json:"terminal_ledger_digest,omitempty"`
	HumanResolutionDigest   string          `json:"human_resolution_digest,omitempty"`
	Status                  AdmissionStatus `json:"status"`
	PrimaryEligible         bool            `json:"primary_eligible"`
	SensitivityEligible     bool            `json:"sensitivity_eligible"`
	Reason                  string          `json:"reason"`
	Digest                  string          `json:"digest"`
}

type ConstraintResult struct {
	ConstraintID       string           `json:"constraint_id"`
	Metric             Metric           `json:"metric"`
	Operator           Operator         `json:"operator"`
	Status             ConstraintStatus `json:"status"`
	OriginalValue      *float64         `json:"original_value,omitempty"`
	TransformedValue   *float64         `json:"transformed_value,omitempty"`
	ComparisonValue    *float64         `json:"comparison_value,omitempty"`
	OriginalState      string           `json:"original_state,omitempty"`
	TransformedState   string           `json:"transformed_state,omitempty"`
	ObservedDifference *float64         `json:"observed_difference,omitempty"`
}

type TaggedDistributionComparison struct {
	Tag        string                           `json:"tag"`
	Comparison verifier.ScoreEvidenceComparison `json:"comparison"`
}

type Result struct {
	SchemaVersion           string                         `json:"schema_version"`
	CanonicalPolicy         string                         `json:"canonical_policy"`
	RelationDigest          string                         `json:"relation_digest"`
	CaseID                  string                         `json:"case_id"`
	TaskGroupID             string                         `json:"task_group_id"`
	Admission               *ConstructAdmission            `json:"construct_admission,omitempty"`
	Outcome                 Outcome                        `json:"outcome"`
	InvalidState            InvalidState                   `json:"invalid_state,omitempty"`
	ConstraintResults       []ConstraintResult             `json:"constraint_results"`
	DistributionComparisons []TaggedDistributionComparison `json:"distribution_comparisons"`
	StageComparison         *StageComparison               `json:"stage_comparison,omitempty"`
	PlannedRepetitions      int                            `json:"planned_repetitions"`
	CompletedRepetitions    int                            `json:"completed_repetitions"`
	ProviderCalls           int                            `json:"provider_calls"`
	CapsuleDigest           string                         `json:"capsule_digest,omitempty"`
	Digest                  string                         `json:"digest"`
}

type ReductionDecision string

const (
	ReductionAccepted ReductionDecision = "accepted"
	ReductionRejected ReductionDecision = "rejected"
)

type ReductionMinimality string

const (
	ReductionOneMinimal ReductionMinimality = "one_minimal"
)

type ReductionUnit struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type ReductionObservation struct {
	SchemaVersion       string `json:"schema_version"`
	CanonicalPolicy     string `json:"canonical_policy"`
	RelationDigest      string `json:"relation_digest"`
	PrivacyPolicyDigest string `json:"privacy_policy_digest"`
	RelationRevalidated bool   `json:"relation_revalidated"`
	PrivacyRevalidated  bool   `json:"privacy_revalidated"`
	ViolationPreserved  bool   `json:"violation_preserved"`
	RelationProofDigest string `json:"relation_proof_digest"`
	PrivacyProofDigest  string `json:"privacy_proof_digest"`
	ReplayResultDigest  string `json:"replay_result_digest"`
	Digest              string `json:"digest"`
}

type ReductionStep struct {
	Index           int                  `json:"index"`
	UnitKind        string               `json:"unit_kind"`
	UnitID          string               `json:"unit_id"`
	BeforeDigest    string               `json:"before_digest"`
	CandidateDigest string               `json:"candidate_digest"`
	AfterDigest     string               `json:"after_digest"`
	Decision        ReductionDecision    `json:"decision"`
	Observation     ReductionObservation `json:"observation"`
}

type Counterexample struct {
	SchemaVersion        string               `json:"schema_version"`
	CanonicalPolicy      string               `json:"canonical_policy"`
	RelationDigest       string               `json:"relation_digest"`
	SourceResultDigest   string               `json:"source_result_digest"`
	CaseID               string               `json:"case_id"`
	OriginalInputDigest  string               `json:"original_input_digest"`
	ReducedInputDigest   string               `json:"reduced_input_digest"`
	PrivacyPolicyDigest  string               `json:"privacy_policy_digest"`
	PublicReleaseAllowed bool                 `json:"public_release_allowed"`
	Algorithm            string               `json:"algorithm"`
	Minimality           ReductionMinimality  `json:"minimality"`
	OriginalUnits        []ReductionUnit      `json:"original_units"`
	FinalUnits           []ReductionUnit      `json:"final_units"`
	OriginalObservation  ReductionObservation `json:"original_observation"`
	Steps                []ReductionStep      `json:"steps"`
	AcceptedReductions   int                  `json:"accepted_reductions"`
	Digest               string               `json:"digest"`
}
