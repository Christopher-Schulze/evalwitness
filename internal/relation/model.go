package relation

import "github.com/Christopher-Schulze/evalwitness/internal/mutation"

const (
	PlanSchemaVersionV1               = "evalwitness.relation-audit-plan.v1"
	PlanSchemaVersionV2               = "evalwitness.relation-audit-plan.v2"
	PlanSchemaVersionV3Adapter        = "evalwitness.relation-review-plan-adapter.v3"
	PilotSampleSchemaVersionV1        = "evalwitness.relation-pilot-sample.v1"
	PilotSampleSchemaVersionV2        = "evalwitness.relation-pilot-sample.v2"
	PilotSampleSchemaVersionV3Adapter = "evalwitness.relation-pilot-sample-adapter.v3"
	PrimarySampleSchemaVersionV1      = "evalwitness.relation-primary-sample.v1"
	PrimarySampleSchemaVersionV2      = "evalwitness.relation-primary-sample.v2"
	StudyAmendmentSchemaVersionV1     = "evalwitness.relation-study-amendment.v1"
	StudyAmendmentSchemaVersionV2     = "evalwitness.relation-study-amendment.v2"
	PlanSchemaVersion                 = PlanSchemaVersionV1
	PilotSampleSchemaVersion          = PilotSampleSchemaVersionV1
	PrimarySampleSchemaVersion        = PrimarySampleSchemaVersionV1
	StudyAmendmentSchemaVersion       = StudyAmendmentSchemaVersionV1
	ReplayReceiptSchemaVersionV1      = "evalwitness.relation-replay-receipt.v1"
	ReplayReceiptSchemaVersionV2      = "evalwitness.relation-replay-receipt.v2"
	ReplayReceiptSchemaVersionV3      = "evalwitness.relation-replay-receipt.v3"
	ReplayReceiptSchemaVersion        = ReplayReceiptSchemaVersionV1
	CaseMaterialSchemaVersionV1       = "evalwitness.relation-case-material.v1"
	CaseMaterialSchemaVersionV2       = "evalwitness.relation-case-material.v2"
	CaseMaterialSchemaVersionV3       = "evalwitness.relation-case-material.v3"
	CaseMaterialSchemaVersion         = CaseMaterialSchemaVersionV1
	CanonicalPolicy                   = "evalwitness.relation-canonical-json.v1"
	ProtocolVersionV1                 = "evalwitness.controlled-relation-review.v1"
	ProtocolVersionV2                 = "evalwitness.controlled-relation-review.v2"
	ProtocolVersionV3                 = GovernanceProtocolVersionV3
	ProtocolVersion                   = ProtocolVersionV1
	MaximumDocumentSize               = 16 * 1024 * 1024
	PrimarySampleRuleV1               = "all and only frozen corpus cases whose mutation manifest has review.required=true; case bindings include exact source, program, manifest, witness, license, privacy, lineage, packet, and regeneration commitments"
	PrimarySampleRuleV2               = "exactly two calibration and two test cases per governed family, selected lexicographically with unique source task groups and jointly with the development pilot to preserve zero source, task-group, or lineage overlap; case bindings include exact source, program, manifest, witness, construct-firewall, license, privacy, lineage, packet, and regeneration commitments"
	PilotSampleRuleV1                 = "one lexicographically deterministic non-review development case per governed family, solved jointly for zero primary source, task-group, or lineage overlap and unique pilot task-group and lineage assignments"
	PilotSampleRuleV2                 = "one lexicographically deterministic development case per governed family, selected jointly with the balanced primary sample for zero primary source, task-group, or lineage overlap and unique pilot task-group and lineage assignments"
	PrimarySampleRule                 = PrimarySampleRuleV1
	PilotSampleRule                   = PilotSampleRuleV1
	UnresolvedRule                    = "resolve unresolved when any required axis is indeterminate, information sufficiency is insufficient, both support and contradiction match, or neither deterministic rule matches"
)

type ReviewObjective string

const ReviewObjectiveControlledRelation ReviewObjective = "controlled_relation"

type UnitType string

const (
	UnitTrajectoryPair      UnitType = "trajectory_pair"
	UnitCandidatePairOrders UnitType = "candidate_pair_orderings"
)

type Axis string

const (
	AxisCausalIntegrity   Axis = "causal_integrity_preservation"
	AxisEvidenceStrength  Axis = "evidence_strength"
	AxisExecutableSupport Axis = "executable_outcome_support"
	AxisInformation       Axis = "information_sufficiency"
	AxisPresentation      Axis = "presentation_equivalence"
	AxisSemanticQuality   Axis = "semantic_task_quality"
	AxisUntrustedControl  Axis = "untrusted_content_authority"
)

type Rating string

const (
	RatingControlEffect Rating = "control_effect"
	RatingEqual         Rating = "equal"
	RatingIndeterminate Rating = "indeterminate"
	RatingInsufficient  Rating = "insufficient"
	RatingLeft          Rating = "left"
	RatingNoControl     Rating = "no_control"
	RatingNotApplicable Rating = "not_applicable"
	RatingRight         Rating = "right"
	RatingSufficient    Rating = "sufficient"
)

type NormalizedRating string

const (
	NormalizedControlEffect NormalizedRating = "control_effect"
	NormalizedEqual         NormalizedRating = "equal"
	NormalizedIndeterminate NormalizedRating = "indeterminate"
	NormalizedInsufficient  NormalizedRating = "insufficient"
	NormalizedNoControl     NormalizedRating = "no_control"
	NormalizedNotApplicable NormalizedRating = "not_applicable"
	NormalizedOriginal      NormalizedRating = "original"
	NormalizedSufficient    NormalizedRating = "sufficient"
	NormalizedTransformed   NormalizedRating = "transformed"
)

type ExternalActionStatus string

const ExternalActionNotAuthorized ExternalActionStatus = "not_authorized"

type TranslationState string

const (
	TranslationSupports    TranslationState = "supports"
	TranslationContradicts TranslationState = "contradicts"
	TranslationUnresolved  TranslationState = "unresolved"
)

type ReasonCode string

const (
	ReasonAmbiguousTask            ReasonCode = "ambiguous_task"
	ReasonCausalIntegrityDiffers   ReasonCode = "causal_integrity_differs"
	ReasonEvidenceOnlyChange       ReasonCode = "evidence_only_change"
	ReasonEvidenceStrengthDiffers  ReasonCode = "evidence_strength_differs"
	ReasonExecutableSupportDiffers ReasonCode = "executable_support_differs"
	ReasonHiddenContextRequired    ReasonCode = "hidden_context_required"
	ReasonInsufficientInformation  ReasonCode = "insufficient_information"
	ReasonMultiFactorChange        ReasonCode = "multi_factor_change"
	ReasonNoJudgmentChange         ReasonCode = "no_judgment_relevant_change"
	ReasonPresentationDiffers      ReasonCode = "presentation_differs"
	ReasonTaskQualityDiffers       ReasonCode = "task_quality_differs"
	ReasonUntrustedContentControls ReasonCode = "untrusted_content_controls"
)

type AxisDefinition struct {
	ID             Axis     `json:"id"`
	Question       string   `json:"question"`
	AllowedRatings []Rating `json:"allowed_ratings"`
}

type TranslationCondition struct {
	Axis    Axis               `json:"axis"`
	Ratings []NormalizedRating `json:"ratings"`
}

type FamilyContract struct {
	Family           mutation.Family        `json:"family"`
	Unit             UnitType               `json:"unit"`
	ExpectedRelation mutation.Relation      `json:"expected_relation"`
	RequiredAxes     []Axis                 `json:"required_axes"`
	SupportAll       []TranslationCondition `json:"support_all"`
	ContradictAny    []TranslationCondition `json:"contradict_any"`
}

type Plan struct {
	SchemaVersion               string               `json:"schema_version"`
	CanonicalPolicy             string               `json:"canonical_policy"`
	ProtocolVersion             string               `json:"protocol_version"`
	Objective                   ReviewObjective      `json:"review_objective"`
	SourceCorpusDigest          string               `json:"source_corpus_digest"`
	SourceCorpusVersion         string               `json:"source_corpus_version"`
	SourceCorpusSpecDigest      string               `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest      string               `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest string               `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest  string               `json:"source_construct_audit_digest,omitempty"`
	PrimarySampleRule           string               `json:"primary_sample_rule"`
	PrimarySampleSize           int                  `json:"primary_sample_size"`
	PilotSampleSize             int                  `json:"pilot_sample_size"`
	PrimaryReviewers            int                  `json:"primary_reviewers"`
	TieBreakReviewers           int                  `json:"tie_break_reviewers"`
	RubricVersion               string               `json:"rubric_version"`
	CommitRevealRule            string               `json:"commit_reveal_rule"`
	UnresolvedRule              string               `json:"unresolved_rule"`
	ReviewerForbiddenInputs     []string             `json:"reviewer_forbidden_inputs"`
	ReasonCodes                 []ReasonCode         `json:"reason_codes"`
	Axes                        []AxisDefinition     `json:"axes"`
	Families                    []FamilyContract     `json:"families"`
	ExternalActionStatus        ExternalActionStatus `json:"external_action_status"`
	RequiredExternalAction      string               `json:"required_external_action"`
	Digest                      string               `json:"digest"`
}

type AxisObservation struct {
	Axis   Axis             `json:"axis"`
	Rating NormalizedRating `json:"rating"`
}

type TranslationResult struct {
	SchemaVersion         string            `json:"schema_version"`
	CanonicalPolicy       string            `json:"canonical_policy"`
	ProtocolVersion       string            `json:"protocol_version"`
	Objective             ReviewObjective   `json:"review_objective"`
	PlanDigest            string            `json:"plan_digest"`
	Family                mutation.Family   `json:"family"`
	ExpectedRelation      mutation.Relation `json:"expected_relation"`
	Observations          []AxisObservation `json:"observations"`
	MatchedSupportAxes    []Axis            `json:"matched_support_axes"`
	MatchedContradictAxes []Axis            `json:"matched_contradict_axes"`
	State                 TranslationState  `json:"state"`
	ReasonCodes           []ReasonCode      `json:"reason_codes"`
	Digest                string            `json:"digest"`
}

type ReplayReceipt struct {
	SchemaVersion                string               `json:"schema_version"`
	CanonicalPolicy              string               `json:"canonical_policy"`
	ProtocolVersion              string               `json:"protocol_version"`
	Objective                    ReviewObjective      `json:"review_objective"`
	SourceCorpusDigest           string               `json:"source_corpus_digest"`
	CaseID                       string               `json:"case_id"`
	Family                       mutation.Family      `json:"family"`
	Unit                         UnitType             `json:"unit"`
	SourceIDs                    []string             `json:"source_ids"`
	OriginalTrajectoryDigests    []string             `json:"original_trajectory_digests"`
	TransformedTrajectoryDigests []string             `json:"transformed_trajectory_digests"`
	OriginalMaterialDigest       string               `json:"original_material_digest"`
	TransformedMaterialDigest    string               `json:"transformed_material_digest"`
	ManifestDigest               string               `json:"manifest_digest"`
	BlindPacketDigest            string               `json:"blind_packet_digest"`
	RegenerationKey              string               `json:"regeneration_key"`
	ReplayStatus                 string               `json:"replay_status"`
	ExternalActionStatus         ExternalActionStatus `json:"external_action_status"`
	Digest                       string               `json:"digest"`
}

type EvidenceExcerpt struct {
	SourceTrajectoryDigest   string   `json:"source_trajectory_digest"`
	RetainedTrajectoryDigest string   `json:"retained_trajectory_digest"`
	SourceEvents             int      `json:"source_events"`
	RetainedEvents           int      `json:"retained_events"`
	OmittedEvents            int      `json:"omitted_events"`
	RedactionHits            int      `json:"redaction_hits"`
	EvidenceBudgetTokens     int      `json:"evidence_budget_tokens"`
	EvidenceSelector         string   `json:"evidence_selector"`
	RequiredEventIDs         []string `json:"required_event_ids"`
	RetainedLineageDigest    string   `json:"retained_lineage_digest"`
	Content                  string   `json:"content"`
	ContentDigest            string   `json:"content_digest"`
	LicenseSPDX              string   `json:"license_spdx"`
	SourceURL                string   `json:"source_url"`
	SourceRevision           string   `json:"source_revision"`
	Redistribution           string   `json:"redistribution"`
	Visibility               string   `json:"visibility"`
	PublicReleasable         bool     `json:"public_releasable"`
}

type CaseMaterial struct {
	SchemaVersion               string               `json:"schema_version"`
	CanonicalPolicy             string               `json:"canonical_policy"`
	ProtocolVersion             string               `json:"protocol_version"`
	Objective                   ReviewObjective      `json:"review_objective"`
	PlanDigest                  string               `json:"plan_digest"`
	SourceCorpusDigest          string               `json:"source_corpus_digest"`
	SourceCorpusSpecDigest      string               `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest      string               `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest string               `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest  string               `json:"source_construct_audit_digest,omitempty"`
	RelationContractVersion     string               `json:"relation_contract_version,omitempty"`
	EvidenceBoundaryVersion     string               `json:"evidence_boundary_version,omitempty"`
	ConstructFirewallDigest     string               `json:"construct_firewall_digest,omitempty"`
	CaseID                      string               `json:"case_id"`
	Family                      mutation.Family      `json:"family"`
	Unit                        UnitType             `json:"unit"`
	TaskRequirement             string               `json:"task_requirement"`
	TaskRequirementDigest       string               `json:"task_requirement_digest"`
	Original                    []EvidenceExcerpt    `json:"original"`
	Transformed                 []EvidenceExcerpt    `json:"transformed"`
	AlignmentDigest             string               `json:"alignment_digest"`
	ReplayReceiptDigest         string               `json:"replay_receipt_digest"`
	Limitations                 []string             `json:"limitations"`
	ExternalActionStatus        ExternalActionStatus `json:"external_action_status"`
	Digest                      string               `json:"digest"`
}

type Count struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type BindingCommitments struct {
	Cases              string `json:"cases"`
	Sources            string `json:"sources"`
	Programs           string `json:"programs"`
	Manifests          string `json:"manifests"`
	Witnesses          string `json:"witnesses"`
	Licenses           string `json:"licenses"`
	Privacy            string `json:"privacy"`
	Lineage            string `json:"lineage"`
	Packets            string `json:"packets"`
	Regeneration       string `json:"regeneration"`
	ConstructFirewalls string `json:"construct_firewalls,omitempty"`
}

type PrimarySample struct {
	SchemaVersion               string               `json:"schema_version"`
	CanonicalPolicy             string               `json:"canonical_policy"`
	ProtocolVersion             string               `json:"protocol_version"`
	Objective                   ReviewObjective      `json:"review_objective"`
	PlanDigest                  string               `json:"plan_digest"`
	SourceCorpusDigest          string               `json:"source_corpus_digest"`
	SourceCorpusSpecDigest      string               `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest      string               `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest string               `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest  string               `json:"source_construct_audit_digest,omitempty"`
	SelectionRule               string               `json:"selection_rule"`
	SelectedCases               int                  `json:"selected_cases"`
	UniqueSourceIDs             int                  `json:"unique_source_ids"`
	UniqueTaskGroups            int                  `json:"unique_task_groups"`
	UniqueLineageClusters       int                  `json:"unique_lineage_clusters,omitempty"`
	TrajectoryPairUnits         int                  `json:"trajectory_pair_units"`
	CandidateOrderUnits         int                  `json:"candidate_order_units"`
	SelectionDigest             string               `json:"selection_digest"`
	FamilyCounts                []Count              `json:"family_counts"`
	SplitCounts                 []Count              `json:"split_counts"`
	ControlCounts               []Count              `json:"control_counts"`
	SourceFormatCounts          []Count              `json:"source_format_counts,omitempty"`
	Bindings                    BindingCommitments   `json:"bindings"`
	ExternalActionStatus        ExternalActionStatus `json:"external_action_status"`
	Digest                      string               `json:"digest"`
}

type PilotCaseReference struct {
	Family            mutation.Family `json:"family"`
	CaseID            string          `json:"case_id"`
	Unit              UnitType        `json:"unit"`
	TaskGroupID       string          `json:"task_group_id"`
	SourceIDs         []string        `json:"source_ids"`
	LineageClusterIDs []string        `json:"lineage_cluster_ids"`
	CaseBindingDigest string          `json:"case_binding_digest"`
}

type PilotSample struct {
	SchemaVersion               string               `json:"schema_version"`
	CanonicalPolicy             string               `json:"canonical_policy"`
	ProtocolVersion             string               `json:"protocol_version"`
	Objective                   ReviewObjective      `json:"review_objective"`
	PlanDigest                  string               `json:"plan_digest"`
	PrimarySampleDigest         string               `json:"primary_sample_digest"`
	SourceCorpusDigest          string               `json:"source_corpus_digest"`
	SourceCorpusSpecDigest      string               `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest      string               `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest string               `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest  string               `json:"source_construct_audit_digest,omitempty"`
	DataRole                    string               `json:"data_role"`
	SelectionRule               string               `json:"selection_rule"`
	SelectedCases               int                  `json:"selected_cases"`
	UniqueSourceIDs             int                  `json:"unique_source_ids"`
	UniqueTaskGroups            int                  `json:"unique_task_groups"`
	UniqueLineageClusters       int                  `json:"unique_lineage_clusters"`
	PrimaryOverlap              int                  `json:"primary_overlap"`
	ScarcitySentinelDigest      string               `json:"scarcity_sentinel_digest,omitempty"`
	ScarcitySentinelOverlap     int                  `json:"scarcity_sentinel_overlap,omitempty"`
	RequiredPrimaryLabels       int                  `json:"required_primary_labels"`
	MaximumTieBreakLabels       int                  `json:"maximum_tie_break_labels"`
	RequiredPostLabelProbes     int                  `json:"required_post_label_probes"`
	Cases                       []PilotCaseReference `json:"cases"`
	Bindings                    BindingCommitments   `json:"bindings"`
	ExternalActionStatus        ExternalActionStatus `json:"external_action_status"`
	EmpiricalStatus             string               `json:"empirical_status,omitempty"`
	Digest                      string               `json:"digest"`
}

type DetectionScenario struct {
	ContradictionRate    float64 `json:"contradiction_rate"`
	DetectionProbability float64 `json:"detection_probability"`
}

type PilotDesign struct {
	Cases                 int    `json:"cases"`
	PrimaryLabels         int    `json:"primary_labels"`
	MaximumTieBreakLabels int    `json:"maximum_tie_break_labels"`
	PostLabelProbes       int    `json:"post_label_probes"`
	DataRole              string `json:"data_role"`
	PrimaryOverlap        int    `json:"primary_overlap"`
	Purpose               string `json:"purpose"`
	PrimaryAnalysisUse    string `json:"primary_analysis_use"`
}

type PrimaryDesign struct {
	Cases                 int    `json:"cases"`
	EffectiveTaskGroups   int    `json:"effective_task_groups"`
	PrimaryLabels         int    `json:"primary_labels"`
	MaximumTieBreakLabels int    `json:"maximum_tie_break_labels"`
	PostLabelProbes       int    `json:"post_label_probes"`
	ClusterUnit           string `json:"cluster_unit"`
	AggregationRule       string `json:"aggregation_rule"`
	ReplacementRule       string `json:"replacement_rule"`
	MissingnessRule       string `json:"missingness_rule"`
	StoppingRule          string `json:"stopping_rule"`
}

type RelationInference struct {
	PrimaryEstimand             string              `json:"primary_estimand"`
	NominalAlpha                float64             `json:"nominal_alpha"`
	IntervalMethod              string              `json:"interval_method"`
	MultiplicityMethod          string              `json:"multiplicity_method"`
	PrimaryMultiplicityFamily   []string            `json:"primary_multiplicity_family"`
	FamilyAnalysisRole          string              `json:"family_analysis_role"`
	ZeroContradictionUpperBound float64             `json:"zero_contradiction_upper_bound"`
	DetectionScenarios          []DetectionScenario `json:"detection_scenarios"`
	UnresolvedDenominatorRule   string              `json:"unresolved_denominator_rule"`
	IndependenceLimitation      string              `json:"independence_limitation"`
}

type StudyAmendment struct {
	SchemaVersion               string               `json:"schema_version"`
	CanonicalPolicy             string               `json:"canonical_policy"`
	ProtocolVersion             string               `json:"protocol_version"`
	Objective                   ReviewObjective      `json:"review_objective"`
	IssuedAt                    string               `json:"issued_at"`
	PlanDigest                  string               `json:"plan_digest"`
	PilotSampleDigest           string               `json:"pilot_sample_digest"`
	PrimarySampleDigest         string               `json:"primary_sample_digest"`
	SourceCorpusDigest          string               `json:"source_corpus_digest"`
	SourceCorpusSpecDigest      string               `json:"source_corpus_spec_digest,omitempty"`
	SourceMutationProgramDigest string               `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest  string               `json:"source_construct_audit_digest,omitempty"`
	Pilot                       PilotDesign          `json:"pilot"`
	Primary                     PrimaryDesign        `json:"primary"`
	Inference                   RelationInference    `json:"inference"`
	ClaimBoundary               string               `json:"claim_boundary"`
	EmpiricalStatus             string               `json:"empirical_status"`
	ExternalActionStatus        ExternalActionStatus `json:"external_action_status"`
	Digest                      string               `json:"digest"`
}
