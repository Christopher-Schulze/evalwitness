package study

import "time"

const (
	ManifestSchemaVersion = "evalwitness.study-manifest.v1"
	LockedSchemaVersion   = "evalwitness.locked-study.v1"
	RecordSchemaVersion   = "evalwitness.study-record.v1"
	SplitSchemaVersion    = "evalwitness.study-split.v1"
	CanonicalPolicy       = "evalwitness.study-canonical-json.v1"
)

type State string

const (
	StateDraft      State = "draft"
	StateLocked     State = "locked"
	StateAuthorized State = "authorized"
	StateRunning    State = "running"
	StateComplete   State = "complete"
	StateFailed     State = "failed"
	StateWithdrawn  State = "withdrawn"
)

type DataRole string

const (
	RoleDevelopment         DataRole = "development"
	RoleCalibration         DataRole = "calibration"
	RoleTest                DataRole = "test"
	RoleExternalReplication DataRole = "external_replication"
	RoleUnavailable         DataRole = "unavailable_for_confirmation"
)

type StudyKind string

const (
	KindBenchmark          StudyKind = "benchmark"
	KindCalibration        StudyKind = "calibration"
	KindControlledRelation StudyKind = "controlled_relation"
	KindEvidenceReliance   StudyKind = "evidence_reliance"
	KindRealAgentCorpus    StudyKind = "real_agent_corpus"
	KindTransfer           StudyKind = "transfer"
	KindDrift              StudyKind = "drift"
)

type Manifest struct {
	SchemaVersion    string                `json:"schema_version"`
	CanonicalPolicy  string                `json:"canonical_policy"`
	Identity         Identity              `json:"identity"`
	Hypotheses       Hypotheses            `json:"hypotheses"`
	Data             DataPlan              `json:"data"`
	Arms             []Arm                 `json:"arms"`
	Outcomes         OutcomePlan           `json:"outcomes"`
	Inference        InferencePlan         `json:"inference"`
	Failures         FailurePlan           `json:"failures"`
	Controls         ControlPlan           `json:"controls"`
	Providers        []ProviderPlan        `json:"providers"`
	Budget           BudgetPlan            `json:"budget"`
	Execution        ExecutionPlan         `json:"execution"`
	Publication      PublicationPlan       `json:"publication"`
	Reliability      ReliabilityContracts  `json:"reliability_contracts"`
	RealAgentCorpus  *RealAgentCorpusPlan  `json:"real_agent_corpus,omitempty"`
	Relations        *ControlledRelations  `json:"controlled_relations,omitempty"`
	EvidenceReliance *EvidenceReliancePlan `json:"evidence_reliance,omitempty"`
	Adjudication     AdjudicationPlan      `json:"adjudication"`
}

type Identity struct {
	Title            string    `json:"title"`
	ResearchQuestion string    `json:"research_question"`
	Kind             StudyKind `json:"kind"`
	Authors          []string  `json:"authors"`
	CreatedAt        time.Time `json:"created_at"`
	LockedAt         time.Time `json:"locked_at"`
}

type Hypotheses struct {
	PrimaryNull        string   `json:"primary_null"`
	PrimaryAlternative string   `json:"primary_alternative"`
	Secondary          []string `json:"secondary"`
	Exploratory        []string `json:"exploratory"`
}

type DataPlan struct {
	PrimaryUnit string            `json:"primary_unit"`
	Datasets    []DatasetManifest `json:"datasets"`
	Split       SplitManifest     `json:"split"`
}

type DatasetManifest struct {
	ID                  string      `json:"id"`
	Source              string      `json:"source"`
	Version             string      `json:"version"`
	License             string      `json:"license"`
	AcquiredAt          time.Time   `json:"acquired_at"`
	DatasetDigest       string      `json:"dataset_digest"`
	TaskIDsDigest       string      `json:"task_ids_digest"`
	OutcomeLabelsDigest string      `json:"outcome_labels_digest"`
	TrajectorySetDigest string      `json:"trajectory_set_digest"`
	TaskCount           int         `json:"task_count"`
	PermittedRoles      []DataRole  `json:"permitted_roles"`
	PreviouslyAccessed  bool        `json:"previously_accessed"`
	Exclusions          []Exclusion `json:"exclusions"`
}

type Exclusion struct {
	ID        string `json:"id"`
	Rule      string `json:"rule"`
	Stage     string `json:"stage"`
	Treatment string `json:"treatment"`
}

type SplitManifest struct {
	SchemaVersion           string            `json:"schema_version"`
	Algorithm               string            `json:"algorithm"`
	Seed                    string            `json:"seed"`
	StratificationVariables []string          `json:"stratification_variables"`
	Assignments             []SplitAssignment `json:"assignments"`
	Digest                  string            `json:"digest"`
}

type SplitAssignment struct {
	DatasetID                 string       `json:"dataset_id"`
	GroupID                   string       `json:"group_id"`
	Split                     DataRole     `json:"split"`
	Stratum                   []NamedValue `json:"stratum"`
	TaskIDs                   []string     `json:"task_ids"`
	RepositoryIDs             []string     `json:"repository_ids"`
	CloneFamilyIDs            []string     `json:"clone_family_ids"`
	TrajectoryDigests         []string     `json:"trajectory_digests"`
	PairObservationIDs        []string     `json:"pair_observation_ids"`
	MutationDescendantDigests []string     `json:"mutation_descendant_digests"`
	EvidenceDescendantDigests []string     `json:"evidence_descendant_digests"`
	AdjudicationPacketDigests []string     `json:"adjudication_packet_digests"`
	CounterexampleDigests     []string     `json:"counterexample_digests"`
	CorpusVersionIDs          []string     `json:"corpus_version_ids"`
}

type NamedValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Arm struct {
	ID                    string `json:"id"`
	Entrypoint            string `json:"entrypoint"`
	RouteID               string `json:"route_id"`
	ProviderID            string `json:"provider_id"`
	RequestedModel        string `json:"requested_model"`
	PromptDigest          string `json:"prompt_digest"`
	RequestContractDigest string `json:"request_contract_digest"`
	ScorePolicyVersion    string `json:"score_policy_version"`
	CalibrationDigest     string `json:"calibration_digest"`
	SelectionMode         string `json:"selection_mode"`
	Candidates            int    `json:"candidates"`
	Repetitions           int    `json:"repetitions"`
	AttestationDigest     string `json:"attestation_digest"`
}

type OutcomePlan struct {
	Primary     Endpoint   `json:"primary"`
	Secondary   []Endpoint `json:"secondary"`
	Exploratory []Endpoint `json:"exploratory"`
}

type Endpoint struct {
	ID                 string  `json:"id"`
	Metric             string  `json:"metric"`
	Direction          string  `json:"direction"`
	Question           string  `json:"question"`
	Margin             float64 `json:"margin"`
	RiskTarget         float64 `json:"risk_target"`
	MinimumCoverage    float64 `json:"minimum_coverage"`
	FailureDenominator string  `json:"failure_denominator"`
}

type InferencePlan struct {
	Test                     string         `json:"test"`
	IntervalMethod           string         `json:"interval_method"`
	DesignMethod             string         `json:"design_method"`
	DesignEvidenceDigest     string         `json:"design_evidence_digest"`
	ClusterUnit              string         `json:"cluster_unit"`
	NominalAlpha             float64        `json:"nominal_alpha"`
	TargetPower              float64        `json:"target_power"`
	MinimumEffect            float64        `json:"minimum_effect"`
	DisagreementRate         float64        `json:"disagreement_rate"`
	DiscordantWinProbability float64        `json:"discordant_win_probability"`
	PowerAtMinimumEffect     float64        `json:"power_at_minimum_effect"`
	DecidableTasks           int            `json:"decidable_tasks"`
	MultiplicityMethod       string         `json:"multiplicity_method"`
	PrimaryFamily            []string       `json:"primary_family"`
	SecondaryFamilies        [][]string     `json:"secondary_families"`
	Sequential               SequentialPlan `json:"sequential"`
}

type SequentialPlan struct {
	Enabled        bool   `json:"enabled"`
	Method         string `json:"method"`
	BoundaryDigest string `json:"boundary_digest"`
	MaximumLooks   int    `json:"maximum_looks"`
}

type FailurePlan struct {
	MissingScore      string `json:"missing_score"`
	ProviderFailure   string `json:"provider_failure"`
	RouteFailure      string `json:"route_failure"`
	Timeout           string `json:"timeout"`
	Abstention        string `json:"abstention"`
	BudgetExhaustion  string `json:"budget_exhaustion"`
	RetryExhaustion   string `json:"retry_exhaustion"`
	IncompleteCell    string `json:"incomplete_cell"`
	DenominatorPolicy string `json:"denominator_policy"`
}

type ControlPlan struct {
	RandomSelectionID       string   `json:"random_selection_id"`
	TaskIndependentSelector string   `json:"task_independent_selector"`
	PositiveControl         string   `json:"positive_control"`
	PositiveControlSource   DataRole `json:"positive_control_source"`
}

type ProviderPlan struct {
	ArmID                             string    `json:"arm_id"`
	AttestationObservedAt             time.Time `json:"attestation_observed_at"`
	AttestationExpiresAt              time.Time `json:"attestation_expires_at"`
	ServedIdentityPolicy              string    `json:"served_identity_policy"`
	ExpectedServedModel               string    `json:"expected_served_model"`
	ExpectedServedModels              []string  `json:"expected_served_models,omitempty"`
	CheckpointAssertionPolicy         string    `json:"checkpoint_assertion_policy"`
	ExpectedCheckpointAssertion       string    `json:"expected_checkpoint_assertion"`
	ExpectedCheckpointAssertionSource string    `json:"expected_checkpoint_assertion_source"`
	RetryPolicyVersion                string    `json:"retry_policy_version"`
	MaxRetries                        int       `json:"max_retries"`
	RequestTimeoutSeconds             int       `json:"request_timeout_seconds"`
	MinDispatchIntervalSeconds        int       `json:"min_dispatch_interval_seconds,omitempty"`
}

type BudgetPlan struct {
	ExpectedCalls       int     `json:"expected_calls"`
	HardCalls           int     `json:"hard_calls"`
	HardAttempts        int     `json:"hard_attempts"`
	HardInputTokens     int     `json:"hard_input_tokens"`
	HardOutputTokens    int     `json:"hard_output_tokens"`
	HardDurationSeconds int64   `json:"hard_duration_seconds"`
	HardConcurrent      int     `json:"hard_concurrent"`
	HardCostUSD         float64 `json:"hard_cost_usd"`
}

type ExecutionPlan struct {
	Commit               string   `json:"commit"`
	Dirty                bool     `json:"dirty"`
	BinaryDigest         string   `json:"binary_digest"`
	Platform             string   `json:"platform"`
	AnalysisCommand      []string `json:"analysis_command"`
	AnalysisVersion      string   `json:"analysis_version"`
	AnalysisDigest       string   `json:"analysis_digest"`
	DeclaredInputPaths   []string `json:"declared_input_paths"`
	DeclaredInputDigests []string `json:"declared_input_digests"`
	DeclaredRouteIDs     []string `json:"declared_route_ids"`
}

type PublicationPlan struct {
	CapsuleVisibility           string    `json:"capsule_visibility"`
	AllowedClaimIDs             []string  `json:"allowed_claim_ids"`
	RequiredCaveats             []string  `json:"required_caveats"`
	IndependentReproductionGate bool      `json:"independent_reproduction_gate"`
	RegisteredReportTimestamp   time.Time `json:"registered_report_timestamp"`
}

type ReliabilityContracts struct {
	ProtocolVersion             string `json:"protocol_version"`
	ProtocolCorpusDigest        string `json:"protocol_corpus_digest"`
	ProtocolRequestCorpusDigest string `json:"protocol_request_corpus_digest"`
	ProtocolSchemaDigest        string `json:"protocol_schema_digest"`
	TraceMappingPolicy          string `json:"trace_mapping_policy"`
	RelationCorpusDigest        string `json:"relation_corpus_digest,omitempty"`
	ValidatorContractDigest     string `json:"validator_contract_digest,omitempty"`
	OutcomeContractDigest       string `json:"outcome_contract_digest"`
	AdjudicationContractDigest  string `json:"adjudication_contract_digest"`
	EvidenceFactorDigest        string `json:"evidence_factor_digest,omitempty"`
	InterventionContractDigest  string `json:"intervention_contract_digest,omitempty"`
	ProfileProjectionDigest     string `json:"profile_projection_digest"`
}

type RealAgentCorpusPlan struct {
	SourceBasis           string   `json:"source_basis"`
	ConsentPolicy         string   `json:"consent_policy"`
	LicensePolicy         string   `json:"license_policy"`
	PrivacyClass          string   `json:"privacy_class"`
	RedactionPolicyDigest string   `json:"redaction_policy_digest"`
	TaskLabelContract     string   `json:"task_label_contract"`
	ContaminationChecks   []string `json:"contamination_checks"`
	FormatTargets         []string `json:"format_targets"`
	ReleaseabilityRule    string   `json:"releaseability_rule"`
}

type ControlledRelations struct {
	CorpusVersion           string   `json:"corpus_version"`
	RelationContractVersion string   `json:"relation_contract_version"`
	MutationFamilies        []string `json:"mutation_families"`
	ExpectedRelations       []string `json:"expected_relations"`
	ValidatorDigests        []string `json:"validator_digests"`
	AmbiguityPolicy         string   `json:"ambiguity_policy"`
	PrimaryDenominator      string   `json:"primary_denominator"`
	ClusterUnit             string   `json:"cluster_unit"`
	ReductionPolicy         string   `json:"reduction_policy"`
	ClaimType               string   `json:"claim_type"`
}

type EvidenceReliancePlan struct {
	FactorOntologyDigest      string   `json:"factor_ontology_digest"`
	AllowedFieldPaths         []string `json:"allowed_field_paths"`
	InterventionOperators     []string `json:"intervention_operators"`
	EvidenceOnlyFamilies      []string `json:"evidence_only_families"`
	QualityChangingFamilies   []string `json:"quality_changing_families"`
	IdentificationAssumptions []string `json:"identification_assumptions"`
	Randomization             string   `json:"randomization"`
	MainEffects               []string `json:"main_effects"`
	Interactions              []string `json:"interactions"`
	Estimators                []string `json:"estimators"`
	MultiplicityMethod        string   `json:"multiplicity_method"`
	MultiplicityFamily        []string `json:"multiplicity_family"`
	InvalidCaseHandling       string   `json:"invalid_case_handling"`
	ReductionPolicy           string   `json:"reduction_policy"`
	RelianceWitness           string   `json:"reliance_witness"`
}

type AdjudicationPlan struct {
	SampleStrata        []string `json:"sample_strata"`
	Blinding            string   `json:"blinding"`
	AgreementMetric     string   `json:"agreement_metric"`
	ConflictResolution  string   `json:"conflict_resolution"`
	LabelRevision       string   `json:"label_revision"`
	SensitivityAnalysis string   `json:"sensitivity_analysis"`
}

type LockedStudy struct {
	SchemaVersion  string   `json:"schema_version"`
	StudyID        string   `json:"study_id"`
	Manifest       Manifest `json:"manifest"`
	ManifestDigest string   `json:"manifest_digest"`
}

type Event struct {
	From                 State     `json:"from,omitempty"`
	To                   State     `json:"to"`
	At                   time.Time `json:"at"`
	Actor                string    `json:"actor"`
	Reason               string    `json:"reason"`
	AttestationDigests   []string  `json:"attestation_digests,omitempty"`
	PreviousRecordDigest string    `json:"previous_record_digest,omitempty"`
}

type Record struct {
	SchemaVersion string      `json:"schema_version"`
	Study         LockedStudy `json:"study"`
	State         State       `json:"state"`
	Events        []Event     `json:"events"`
	RecordDigest  string      `json:"record_digest"`
}
