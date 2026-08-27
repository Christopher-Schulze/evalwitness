package reliance

import "github.com/Christopher-Schulze/evalwitness/internal/stats"

const (
	PreflightSchemaVersion = "evalwitness.reliance-preflight.v1"
	PreflightAlgorithm     = "evalwitness.reliance-walsh-power-search.v1"
)

type WalshCandidateAudit struct {
	Runs                         int    `json:"runs"`
	Status                       string `json:"status"`
	Reason                       string `json:"reason"`
	SumFreeSetsAudited           int    `json:"sum_free_sets_audited"`
	QualifyingInteractionLayouts int    `json:"qualifying_interaction_layouts"`
}

type WalshAliasAudit struct {
	Factors                          int                   `json:"factors"`
	DeclaredInteractions             int                   `json:"declared_interactions"`
	SelectedRuns                     int                   `json:"selected_runs"`
	MainEffectsClearOfTwoFactorTerms bool                  `json:"main_effects_clear_of_two_factor_terms"`
	DeclaredInteractionsUnique       bool                  `json:"declared_interactions_unique"`
	Candidates                       []WalshCandidateAudit `json:"candidates"`
}

type RelianceResourceModel struct {
	BillingModel           string  `json:"billing_model"`
	BaselineCallsPerTask   int     `json:"baseline_calls_per_task"`
	CallsPerCell           int     `json:"calls_per_cell"`
	EvidenceTokensPerCall  int     `json:"evidence_tokens_per_call"`
	PromptTokensPerCall    int     `json:"prompt_tokens_per_call"`
	MaximumOutputTokens    int     `json:"maximum_output_tokens"`
	MaximumRetries         int     `json:"maximum_retries"`
	RequestTimeoutSeconds  int     `json:"request_timeout_seconds"`
	Concurrency            int     `json:"concurrency"`
	MarginalCostUSDPerCall float64 `json:"marginal_cost_usd_per_call"`
}

type RelianceResourceBudget struct {
	LogicalCalls        int     `json:"logical_calls"`
	HardAttempts        int     `json:"hard_attempts"`
	HardInputTokens     int     `json:"hard_input_tokens"`
	HardOutputTokens    int     `json:"hard_output_tokens"`
	HardDurationSeconds int     `json:"hard_duration_seconds"`
	HardConcurrency     int     `json:"hard_concurrency"`
	HardCostUSD         float64 `json:"hard_cost_usd"`
}

type PreflightPowerCheck struct {
	ScenarioID     string   `json:"scenario_id"`
	TermID         string   `json:"term_id"`
	AppliesTo      []string `json:"applies_to"`
	DeclaredEffect float64  `json:"declared_effect"`
	MeanEstimate   float64  `json:"mean_estimate"`
	Power          float64  `json:"power"`
	MonteCarloSE   float64  `json:"monte_carlo_se"`
	PowerLower95   float64  `json:"power_lower_95"`
}

type PreflightCandidate struct {
	SourceTasks int                    `json:"source_tasks"`
	Budget      RelianceResourceBudget `json:"budget"`
	Checks      []PreflightPowerCheck  `json:"checks"`
	Resolved    bool                   `json:"resolved"`
}

type PreflightScenarioReport struct {
	ScenarioID string                        `json:"scenario_id"`
	Report     stats.ClusterSimulationReport `json:"report"`
}

type PreflightMDEPoint struct {
	DeclaredEffect float64               `json:"declared_effect"`
	Checks         []PreflightPowerCheck `json:"checks"`
	Resolved       bool                  `json:"resolved"`
}

type PreflightMDE struct {
	ScenarioID              string              `json:"scenario_id"`
	DeclaredEffectScale     string              `json:"declared_effect_scale"`
	EstimateScale           string              `json:"estimate_scale"`
	EffectGrid              []float64           `json:"effect_grid"`
	Points                  []PreflightMDEPoint `json:"points"`
	MinimumDetectableEffect *float64            `json:"minimum_detectable_effect"`
}

type ReliancePreflight struct {
	SchemaVersion          string                    `json:"schema_version"`
	CanonicalPolicy        string                    `json:"canonical_policy"`
	Algorithm              string                    `json:"algorithm"`
	PreregistrationDigest  string                    `json:"preregistration_digest"`
	CodeDigest             string                    `json:"code_digest"`
	TargetPower            float64                   `json:"target_power"`
	MonteCarloReplications int                       `json:"monte_carlo_replications"`
	CandidateSourceTasks   []int                     `json:"candidate_source_tasks"`
	AliasAudit             WalshAliasAudit           `json:"alias_audit"`
	ResourceModel          RelianceResourceModel     `json:"resource_model"`
	HardBudget             RelianceResourceBudget    `json:"hard_budget"`
	Candidates             []PreflightCandidate      `json:"candidates"`
	SelectedSourceTasks    int                       `json:"selected_source_tasks"`
	SelectedBudget         RelianceResourceBudget    `json:"selected_budget"`
	SelectedScenarios      []PreflightScenarioReport `json:"selected_scenarios,omitempty"`
	SelectedMDEs           []PreflightMDE            `json:"selected_mdes,omitempty"`
	Status                 string                    `json:"status"`
	LiveAuthorized         bool                      `json:"live_authorized"`
	EmpiricalAssumptions   bool                      `json:"empirical_assumptions"`
	Digest                 string                    `json:"digest"`
}
