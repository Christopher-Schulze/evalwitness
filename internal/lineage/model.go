package lineage

import "time"

const (
	PlanSchemaVersion                = "evalwitness.verification-lineage-plan.v1"
	CanonicalPolicy                  = "evalwitness.verification-lineage-canonical-json.v1"
	ProtocolVersion                  = "evalwitness.verification-lineage-protocol.v1"
	StudyGovernancePolicy            = "evalwitness.study-governance.v1"
	LockedPlanDigest                 = "5f56a357721a4ec2b650660a4efb7b4d9b67d342ada388a5d28f6e607fd9189c"
	PrimaryUnit                      = "task_clustered_claim_check_attempt"
	ClusterUnit                      = "task_group"
	ExternalActionNotAuthorized      = "not_authorized"
	AcquisitionNotStarted            = "not_started"
	CountsNotInspectedBeforePlanLock = "not_inspected_before_plan_lock"
)

type DataRole string

const (
	RoleAdapterDevelopment   DataRole = "adapter_development"
	RoleCaptureCalibration   DataRole = "capture_calibration"
	RoleLockedTest           DataRole = "locked_test"
	RoleAdversarialChallenge DataRole = "adversarial_challenge"
)

type TerminalState string

const (
	StateInvalidCapture                   TerminalState = "invalid_capture"
	StateBehaviorAbsent                   TerminalState = "behavior_absent"
	StateExportObservabilityAbsent        TerminalState = "export_observability_absent"
	StateAdapterMappingLoss               TerminalState = "adapter_mapping_loss"
	StateUnsupportedShell                 TerminalState = "unsupported_shell"
	StateAmbiguousLineage                 TerminalState = "ambiguous_lineage"
	StateNonFailableVerification          TerminalState = "non_failable_verification"
	StateClaimSpecificEvidenceNotWeakened TerminalState = "claim_specific_evidence_not_weakened"
	StateFreshnessUnresolved              TerminalState = "freshness_unresolved"
	StateDirectVerificationInvocation     TerminalState = "direct_verification_invocation"
)

type StateDisposition string

const (
	DispositionExcluded   StateDisposition = "excluded"
	DispositionLoss       StateDisposition = "terminal_loss"
	DispositionIneligible StateDisposition = "terminal_ineligible"
	DispositionEligible   StateDisposition = "eligible"
)

type HoldoutKind string

const (
	HoldoutFormat       HoldoutKind = "format_holdout"
	HoldoutSyntaxFamily HoldoutKind = "syntax_family_holdout"
)

type VerificationLineagePlan struct {
	SchemaVersion                string             `json:"schema_version"`
	CanonicalPolicy              string             `json:"canonical_policy"`
	ProtocolVersion              string             `json:"protocol_version"`
	StudyGovernancePolicy        string             `json:"study_governance_policy"`
	TraceMappingPolicy           string             `json:"trace_mapping_policy"`
	VerificationEvidenceContract string             `json:"verification_evidence_contract"`
	Identity                     PlanIdentity       `json:"identity"`
	ResearchQuestions            []ResearchQuestion `json:"research_questions"`
	Unit                         UnitPlan           `json:"unit"`
	SourceClasses                []SourceClassPlan  `json:"source_classes"`
	Clusters                     ClusterPlan        `json:"clusters"`
	Roles                        []RolePlan         `json:"roles"`
	Missingness                  MissingnessPlan    `json:"missingness"`
	Exclusions                   []ExclusionRule    `json:"exclusions"`
	Replacement                  ReplacementPlan    `json:"replacement"`
	Stopping                     StoppingPlan       `json:"stopping"`
	MinimumSupport               MinimumSupportPlan `json:"minimum_support"`
	Uncertainty                  UncertaintyPlan    `json:"uncertainty"`
	Holdouts                     []HoldoutPlan      `json:"holdouts"`
	Claims                       ClaimPlan          `json:"claims"`
	Acquisition                  AcquisitionPlan    `json:"acquisition"`
	Digest                       string             `json:"digest"`
}

type PlanIdentity struct {
	PlanID    string    `json:"plan_id"`
	TaskID    string    `json:"task_id"`
	Title     string    `json:"title"`
	Authors   []string  `json:"authors"`
	CreatedAt time.Time `json:"created_at"`
	LockedAt  time.Time `json:"locked_at"`
}

type ResearchQuestion struct {
	ID                      string `json:"id"`
	Question                string `json:"question"`
	PrimaryObservable       string `json:"primary_observable"`
	ForbiddenInterpretation string `json:"forbidden_interpretation"`
}

type UnitPlan struct {
	PrimaryUnit       string   `json:"primary_unit"`
	ClusterUnit       string   `json:"cluster_unit"`
	NestedUnits       []string `json:"nested_units"`
	RetryPolicy       string   `json:"retry_policy"`
	DenominatorPolicy string   `json:"denominator_policy"`
}

type SourceClassPlan struct {
	ID                       string     `json:"id"`
	Basis                    string     `json:"basis"`
	PermittedRoles           []DataRole `json:"permitted_roles"`
	SourceManifestRequired   bool       `json:"source_manifest_required"`
	ExplicitConsentRequired  bool       `json:"explicit_consent_required"`
	RedistributionRequired   bool       `json:"redistribution_required"`
	LiveProviderUsePermitted bool       `json:"live_provider_use_permitted"`
	PreviouslyInspectedUse   string     `json:"previously_inspected_use"`
}

type ClusterPlan struct {
	Dimensions        []string   `json:"dimensions"`
	IsolatedRoles     []DataRole `json:"isolated_roles"`
	CrossRolePolicy   string     `json:"cross_role_policy"`
	PairedViewPolicy  string     `json:"paired_view_policy"`
	NearDuplicateRule string     `json:"near_duplicate_rule"`
}

type RolePlan struct {
	Role                      DataRole `json:"role"`
	Purpose                   string   `json:"purpose"`
	MinimumEligibleTaskGroups int      `json:"minimum_eligible_task_groups"`
	ParserChangesPermitted    bool     `json:"parser_changes_permitted"`
	ConfirmatoryClaims        bool     `json:"confirmatory_claims"`
}

type MissingnessPlan struct {
	TerminalStates     []TerminalStateRule `json:"terminal_states"`
	ClassificationRule string              `json:"classification_rule"`
	ConservationRule   string              `json:"conservation_rule"`
}

type TerminalStateRule struct {
	State            TerminalState    `json:"state"`
	Precedence       int              `json:"precedence"`
	Disposition      StateDisposition `json:"disposition"`
	ProofRequirement string           `json:"proof_requirement"`
}

type ExclusionRule struct {
	ID        string `json:"id"`
	Stage     string `json:"stage"`
	Rule      string `json:"rule"`
	Treatment string `json:"treatment"`
}

type ReplacementPlan struct {
	PostOutcomeReplacementPermitted bool   `json:"post_outcome_replacement_permitted"`
	PermittedStage                  string `json:"permitted_stage"`
	MatchingRule                    string `json:"matching_rule"`
	LineageRule                     string `json:"lineage_rule"`
	ShortfallRule                   string `json:"shortfall_rule"`
}

type StoppingPlan struct {
	MaximumAdmittedTaskGroups int    `json:"maximum_admitted_task_groups"`
	MaximumSourceSessions     int    `json:"maximum_source_sessions"`
	MaximumAcquisitionDays    int    `json:"maximum_acquisition_days"`
	OutcomeDependentStopping  bool   `json:"outcome_dependent_stopping"`
	StopRule                  string `json:"stop_rule"`
}

type MinimumSupportPlan struct {
	NativeTraceFormats             int  `json:"native_trace_formats"`
	IndependentAgentEcosystems     int  `json:"independent_agent_ecosystems"`
	TaskGroupsPerInferentialFormat int  `json:"task_groups_per_inferential_format"`
	CalibrationTaskGroups          int  `json:"calibration_task_groups"`
	TestTaskGroups                 int  `json:"test_task_groups"`
	PairedWitnessRequired          bool `json:"paired_witness_required"`
}

type UncertaintyPlan struct {
	ConfidenceLevel          float64  `json:"confidence_level"`
	ProportionInterval       string   `json:"proportion_interval"`
	PairedDifferenceInterval string   `json:"paired_difference_interval"`
	BootstrapReplicates      int      `json:"bootstrap_replicates"`
	BootstrapSeed            int64    `json:"bootstrap_seed"`
	MultiplicityPolicy       string   `json:"multiplicity_policy"`
	PrimaryEstimands         []string `json:"primary_estimands"`
	RawEventCountsSecondary  bool     `json:"raw_event_counts_secondary"`
}

type HoldoutPlan struct {
	ID                  string      `json:"id"`
	Kind                HoldoutKind `json:"kind"`
	SelectionSeed       string      `json:"selection_seed"`
	SelectionRule       string      `json:"selection_rule"`
	MinimumHeldOutUnits int         `json:"minimum_held_out_units"`
	FrozenSurface       string      `json:"frozen_surface"`
	PostResultRecovery  string      `json:"post_result_recovery"`
}

type ClaimPlan struct {
	Allowed                []ClaimCeiling `json:"allowed"`
	Forbidden              []string       `json:"forbidden"`
	CompositeScoreAllowed  bool           `json:"composite_score_allowed"`
	ProviderRankingAllowed bool           `json:"provider_ranking_allowed"`
}

type ClaimCeiling struct {
	ID               string `json:"id"`
	MaximumScope     string `json:"maximum_scope"`
	RequiredEvidence string `json:"required_evidence"`
}

type AcquisitionPlan struct {
	State                      string   `json:"state"`
	CountInspectionState       string   `json:"count_inspection_state"`
	ExternalActionStatus       string   `json:"external_action_status"`
	ProviderCallsAllowed       int      `json:"provider_calls_allowed"`
	LaboratoryMayLaunchAgents  bool     `json:"laboratory_may_launch_agents"`
	OwnerAuthorizationRequired bool     `json:"owner_authorization_required"`
	PreAcquisitionRequirements []string `json:"pre_acquisition_requirements"`
	LiveCaptureCommandStatus   string   `json:"live_capture_command_status"`
}
