package relation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

const (
	GovernanceProtocolVersionV3     = "evalwitness.controlled-relation-governance.v3"
	PlanSchemaVersionV3             = "evalwitness.relation-audit-plan.v3"
	PrimarySampleSchemaVersionV3    = "evalwitness.relation-primary-sample.v3"
	PilotSampleSchemaVersionV3      = "evalwitness.relation-pilot-sample.v3"
	ScarcitySentinelSchemaVersionV3 = "evalwitness.relation-scarcity-sentinel.v3"
	StudyAmendmentSchemaVersionV3   = "evalwitness.relation-study-amendment.v3"
	PrimarySampleRuleV3             = "exactly two calibration and two test cases per inferential-core family, selected lexicographically with globally unique sources, source task groups, and lineage clusters while preserving a disjoint seven-family development pilot and all three exhaustive scarcity-sentinel cases"
	PilotSampleRuleV3               = "one lexicographically deterministic development case per inferential-core family with globally unique sources, source task groups, and lineage clusters and zero overlap with the primary sample or exhaustive scarcity sentinel"
	ScarcitySentinelRuleV3          = "all and only three naturally eligible omitted_test_evidence cases from the frozen v3 release, retained descriptively outside the primary estimand with their observed 2 development, 1 calibration, and 0 test roles"
	primaryReplacementRuleV3        = "none; the exact balanced 28-case inferential-core commitment is immutable and every unavailable case remains visible"
	primaryStoppingRuleV3           = "fixed sample; stop only after all 28 inferential-core cases reach committed dual review plus required tie-break or explicit unresolved state; no sequential efficacy stopping"
	independenceLimitationV3        = "exact binomial resolution assumes independent sampled task groups; the governed corpus is not a probability sample, but the 28 committed core cases occupy 28 distinct source task groups and 28 distinct lineage clusters, so the bound remains a corpus-specific design diagnostic rather than a population guarantee"
	claimBoundaryV3                 = "report blinded reviewers' support, contradiction, and unresolved states for the frozen 28-case seven-family inferential core; report the exhaustive three-case omitted-evidence sentinel separately as corpus-specific scarcity; do not claim held-out sentinel validity, per-family prevalence, universal construct validity, verifier robustness, or human ground truth"
	ScarcitySentinelAnalysisUseV3   = "descriptive_only_excluded_from_primary_estimand"
	EmpiricalStatusNotRun           = "not_run"
)

type RelationPlanV3 struct {
	SchemaVersion               string               `json:"schema_version"`
	CanonicalPolicy             string               `json:"canonical_policy"`
	ProtocolVersion             string               `json:"protocol_version"`
	Objective                   ReviewObjective      `json:"review_objective"`
	SourceCorpusDigest          string               `json:"source_corpus_digest"`
	SourceCorpusVersion         string               `json:"source_corpus_version"`
	SourceCorpusPlanDigest      string               `json:"source_corpus_plan_digest"`
	SourceConstructAuditDigest  string               `json:"source_construct_audit_digest"`
	SourceMutationProgramDigest string               `json:"source_mutation_program_digest"`
	PrimarySampleRule           string               `json:"primary_sample_rule"`
	PilotSampleRule             string               `json:"pilot_sample_rule"`
	ScarcitySentinelRule        string               `json:"scarcity_sentinel_rule"`
	PrimarySampleSize           int                  `json:"primary_sample_size"`
	PilotSampleSize             int                  `json:"pilot_sample_size"`
	ScarcitySentinelSize        int                  `json:"scarcity_sentinel_size"`
	PrimaryReviewers            int                  `json:"primary_reviewers"`
	TieBreakReviewers           int                  `json:"tie_break_reviewers"`
	RubricVersion               string               `json:"rubric_version"`
	CommitRevealRule            string               `json:"commit_reveal_rule"`
	UnresolvedRule              string               `json:"unresolved_rule"`
	ReviewerForbiddenInputs     []string             `json:"reviewer_forbidden_inputs"`
	ReasonCodes                 []ReasonCode         `json:"reason_codes"`
	Axes                        []AxisDefinition     `json:"axes"`
	CoreFamilies                []FamilyContract     `json:"core_families"`
	ScarcitySentinel            FamilyContract       `json:"scarcity_sentinel"`
	SentinelInPrimaryEstimand   bool                 `json:"sentinel_in_primary_estimand"`
	HeldOutSentinelAvailable    bool                 `json:"held_out_sentinel_available"`
	EmpiricalStatus             string               `json:"empirical_status"`
	ExternalActionStatus        ExternalActionStatus `json:"external_action_status"`
	RequiredExternalAction      string               `json:"required_external_action"`
	Digest                      string               `json:"digest"`
}

type GovernedCaseReferenceV3 struct {
	Family                  mutation.Family `json:"family"`
	CaseID                  string          `json:"case_id"`
	DataRole                study.DataRole  `json:"data_role"`
	Unit                    UnitType        `json:"unit"`
	TaskGroupID             string          `json:"task_group_id"`
	SourceIDs               []string        `json:"source_ids"`
	LineageClusterIDs       []string        `json:"lineage_cluster_ids"`
	CaseBindingDigest       string          `json:"case_binding_digest"`
	ConstructFirewallDigest string          `json:"construct_firewall_digest"`
}

type PrimarySampleV3 struct {
	SchemaVersion               string                    `json:"schema_version"`
	CanonicalPolicy             string                    `json:"canonical_policy"`
	ProtocolVersion             string                    `json:"protocol_version"`
	Objective                   ReviewObjective           `json:"review_objective"`
	PlanDigest                  string                    `json:"plan_digest"`
	SourceCorpusDigest          string                    `json:"source_corpus_digest"`
	SourceCorpusPlanDigest      string                    `json:"source_corpus_plan_digest"`
	SourceConstructAuditDigest  string                    `json:"source_construct_audit_digest"`
	SourceMutationProgramDigest string                    `json:"source_mutation_program_digest"`
	SelectionRule               string                    `json:"selection_rule"`
	SelectedCases               int                       `json:"selected_cases"`
	UniqueSourceIDs             int                       `json:"unique_source_ids"`
	UniqueTaskGroups            int                       `json:"unique_task_groups"`
	UniqueLineageClusters       int                       `json:"unique_lineage_clusters"`
	TrajectoryPairUnits         int                       `json:"trajectory_pair_units"`
	CandidateOrderUnits         int                       `json:"candidate_order_units"`
	SelectionDigest             string                    `json:"selection_digest"`
	FamilyCounts                []Count                   `json:"family_counts"`
	SplitCounts                 []Count                   `json:"split_counts"`
	ControlCounts               []Count                   `json:"control_counts"`
	SourceFormatCounts          []Count                   `json:"source_format_counts"`
	Cases                       []GovernedCaseReferenceV3 `json:"cases"`
	Bindings                    BindingCommitments        `json:"bindings"`
	EmpiricalStatus             string                    `json:"empirical_status"`
	ExternalActionStatus        ExternalActionStatus      `json:"external_action_status"`
	Digest                      string                    `json:"digest"`
}

type PilotSampleV3 struct {
	SchemaVersion               string                    `json:"schema_version"`
	CanonicalPolicy             string                    `json:"canonical_policy"`
	ProtocolVersion             string                    `json:"protocol_version"`
	Objective                   ReviewObjective           `json:"review_objective"`
	PlanDigest                  string                    `json:"plan_digest"`
	PrimarySampleDigest         string                    `json:"primary_sample_digest"`
	ScarcitySentinelDigest      string                    `json:"scarcity_sentinel_digest"`
	SourceCorpusDigest          string                    `json:"source_corpus_digest"`
	SourceCorpusPlanDigest      string                    `json:"source_corpus_plan_digest"`
	SourceConstructAuditDigest  string                    `json:"source_construct_audit_digest"`
	SourceMutationProgramDigest string                    `json:"source_mutation_program_digest"`
	DataRole                    string                    `json:"data_role"`
	SelectionRule               string                    `json:"selection_rule"`
	SelectedCases               int                       `json:"selected_cases"`
	UniqueSourceIDs             int                       `json:"unique_source_ids"`
	UniqueTaskGroups            int                       `json:"unique_task_groups"`
	UniqueLineageClusters       int                       `json:"unique_lineage_clusters"`
	PrimaryOverlap              int                       `json:"primary_overlap"`
	ScarcitySentinelOverlap     int                       `json:"scarcity_sentinel_overlap"`
	RequiredPrimaryLabels       int                       `json:"required_primary_labels"`
	MaximumTieBreakLabels       int                       `json:"maximum_tie_break_labels"`
	RequiredPostLabelProbes     int                       `json:"required_post_label_probes"`
	Cases                       []GovernedCaseReferenceV3 `json:"cases"`
	Bindings                    BindingCommitments        `json:"bindings"`
	EmpiricalStatus             string                    `json:"empirical_status"`
	ExternalActionStatus        ExternalActionStatus      `json:"external_action_status"`
	Digest                      string                    `json:"digest"`
}

type ScarcitySentinelV3 struct {
	SchemaVersion               string                    `json:"schema_version"`
	CanonicalPolicy             string                    `json:"canonical_policy"`
	ProtocolVersion             string                    `json:"protocol_version"`
	Objective                   ReviewObjective           `json:"review_objective"`
	PlanDigest                  string                    `json:"plan_digest"`
	PrimarySampleDigest         string                    `json:"primary_sample_digest"`
	SourceCorpusDigest          string                    `json:"source_corpus_digest"`
	SourceCorpusPlanDigest      string                    `json:"source_corpus_plan_digest"`
	SourceConstructAuditDigest  string                    `json:"source_construct_audit_digest"`
	SourceMutationProgramDigest string                    `json:"source_mutation_program_digest"`
	SelectionRule               string                    `json:"selection_rule"`
	Family                      mutation.Family           `json:"family"`
	SelectedCases               int                       `json:"selected_cases"`
	SplitCounts                 []Count                   `json:"split_counts"`
	TestCases                   int                       `json:"test_cases"`
	PrimaryOverlap              int                       `json:"primary_overlap"`
	Exhaustive                  bool                      `json:"exhaustive"`
	AnalysisUse                 string                    `json:"analysis_use"`
	HeldOutClaimAvailable       bool                      `json:"held_out_claim_available"`
	Cases                       []GovernedCaseReferenceV3 `json:"cases"`
	Bindings                    BindingCommitments        `json:"bindings"`
	EmpiricalStatus             string                    `json:"empirical_status"`
	ExternalActionStatus        ExternalActionStatus      `json:"external_action_status"`
	Digest                      string                    `json:"digest"`
}

type ScarcitySentinelDesignV3 struct {
	Cases                 int     `json:"cases"`
	SplitCounts           []Count `json:"split_counts"`
	TestCases             int     `json:"test_cases"`
	PrimaryOverlap        int     `json:"primary_overlap"`
	Exhaustive            bool    `json:"exhaustive"`
	PrimaryAnalysisUse    string  `json:"primary_analysis_use"`
	HeldOutClaimAvailable bool    `json:"held_out_claim_available"`
}

type StudyAmendmentV3 struct {
	SchemaVersion               string                   `json:"schema_version"`
	CanonicalPolicy             string                   `json:"canonical_policy"`
	ProtocolVersion             string                   `json:"protocol_version"`
	Objective                   ReviewObjective          `json:"review_objective"`
	IssuedAt                    string                   `json:"issued_at"`
	PlanDigest                  string                   `json:"plan_digest"`
	PilotSampleDigest           string                   `json:"pilot_sample_digest"`
	PrimarySampleDigest         string                   `json:"primary_sample_digest"`
	ScarcitySentinelDigest      string                   `json:"scarcity_sentinel_digest"`
	SourceCorpusDigest          string                   `json:"source_corpus_digest"`
	SourceCorpusPlanDigest      string                   `json:"source_corpus_plan_digest"`
	SourceConstructAuditDigest  string                   `json:"source_construct_audit_digest"`
	SourceMutationProgramDigest string                   `json:"source_mutation_program_digest"`
	Pilot                       PilotDesign              `json:"pilot"`
	Primary                     PrimaryDesign            `json:"primary"`
	ScarcitySentinel            ScarcitySentinelDesignV3 `json:"scarcity_sentinel"`
	Inference                   RelationInference        `json:"inference"`
	ClaimBoundary               string                   `json:"claim_boundary"`
	EmpiricalStatus             string                   `json:"empirical_status"`
	ExternalActionStatus        ExternalActionStatus     `json:"external_action_status"`
	Digest                      string                   `json:"digest"`
}

func BuildPlanV3(corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3) (RelationPlanV3, error) {
	if err := release.Validate(corpusPlan, audit); err != nil {
		return RelationPlanV3{}, err
	}
	allContracts := defaultFamilyContracts()
	coreContracts := make([]FamilyContract, 0, len(release.Policy.InferentialCoreFamilies))
	var sentinel FamilyContract
	for _, contract := range allContracts {
		if contract.Family == mutation.FamilyTestEvidenceOmitted {
			sentinel = contract
			continue
		}
		if slices.Contains(release.Policy.InferentialCoreFamilies, contract.Family) {
			coreContracts = append(coreContracts, contract)
		}
	}
	plan := RelationPlanV3{
		ProtocolVersion: GovernanceProtocolVersionV3, Objective: ReviewObjectiveControlledRelation,
		SourceCorpusDigest: release.Digest, SourceCorpusVersion: release.CorpusVersion, SourceCorpusPlanDigest: corpusPlan.Digest,
		SourceConstructAuditDigest: audit.Digest, SourceMutationProgramDigest: release.MutationProgramDigest,
		PrimarySampleRule: PrimarySampleRuleV3, PilotSampleRule: PilotSampleRuleV3, ScarcitySentinelRule: ScarcitySentinelRuleV3,
		PrimarySampleSize: 28, PilotSampleSize: 7, ScarcitySentinelSize: 3, PrimaryReviewers: 2, TieBreakReviewers: 1,
		RubricVersion:    "evalwitness.relation-rubric.v3",
		CommitRevealRule: "both complete primary judgment batches and post-label condition probes must be committed before any family, direction, source-condition, sentinel status, or mapping reveal",
		UnresolvedRule:   UnresolvedRule,
		ReviewerForbiddenInputs: []string{
			"candidate_order", "desired_conclusion", "expected_relation", "formal_validator_result", "mutation_family", "mutation_operator",
			"provider_identity", "scarcity_sentinel_status", "source_condition", "verifier_confidence", "verifier_decision", "verifier_score",
		},
		ReasonCodes: canonicalReasonCodes(), Axes: defaultAxes(), CoreFamilies: coreContracts, ScarcitySentinel: sentinel,
		SentinelInPrimaryEstimand: false, HeldOutSentinelAvailable: false, EmpiricalStatus: EmpiricalStatusNotRun,
		ExternalActionStatus:   ExternalActionNotAuthorized,
		RequiredExternalAction: "obtain explicit owner authorization before reviewer recruitment, contact, scheduling, assignment, packet sharing, compensation, or publication of consented human artifacts",
	}
	return sealRelationPlanV3(plan)
}

// ReviewPlanV3 projects the frozen v3 governance document into the internal
// plan view consumed by the reviewer workflow. The adapter is never a public
// schema: every descendant binds the exact RelationPlanV3 digest.
func ReviewPlanV3(plan RelationPlanV3) (Plan, error) {
	if err := plan.Validate(); err != nil {
		return Plan{}, err
	}
	adapted := Plan{
		SchemaVersion: PlanSchemaVersionV3Adapter, CanonicalPolicy: CanonicalPolicy,
		ProtocolVersion: ProtocolVersionV3, Objective: plan.Objective, SourceCorpusDigest: plan.SourceCorpusDigest,
		SourceCorpusVersion: plan.SourceCorpusVersion, SourceCorpusPlanDigest: plan.SourceCorpusPlanDigest,
		SourceMutationProgramDigest: plan.SourceMutationProgramDigest, SourceConstructAuditDigest: plan.SourceConstructAuditDigest,
		PrimarySampleRule: plan.PrimarySampleRule, PrimarySampleSize: plan.PrimarySampleSize, PilotSampleSize: plan.PilotSampleSize,
		PrimaryReviewers: plan.PrimaryReviewers, TieBreakReviewers: plan.TieBreakReviewers, RubricVersion: plan.RubricVersion,
		CommitRevealRule: plan.CommitRevealRule, UnresolvedRule: plan.UnresolvedRule,
		ReviewerForbiddenInputs: append([]string(nil), plan.ReviewerForbiddenInputs...), ReasonCodes: append([]ReasonCode(nil), plan.ReasonCodes...),
		Axes: append([]AxisDefinition(nil), plan.Axes...), Families: append([]FamilyContract(nil), plan.CoreFamilies...),
		ExternalActionStatus: plan.ExternalActionStatus, RequiredExternalAction: plan.RequiredExternalAction, Digest: plan.Digest,
	}
	return adapted, adapted.Validate()
}

// ReviewPilotSampleV3 projects the governed v3 pilot into the internal shape
// used by readiness and package builders while preserving its exact digest.
func ReviewPilotSampleV3(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, pilot PilotSampleV3) (PilotSample, error) {
	if err := pilot.Validate(plan, primary, sentinel); err != nil {
		return PilotSample{}, err
	}
	cases := make([]PilotCaseReference, len(pilot.Cases))
	for index, item := range pilot.Cases {
		cases[index] = PilotCaseReference{
			Family: item.Family, CaseID: item.CaseID, Unit: item.Unit, TaskGroupID: item.TaskGroupID,
			SourceIDs: append([]string(nil), item.SourceIDs...), LineageClusterIDs: append([]string(nil), item.LineageClusterIDs...),
			CaseBindingDigest: item.CaseBindingDigest,
		}
	}
	adapted := PilotSample{
		SchemaVersion: PilotSampleSchemaVersionV3Adapter, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersionV3,
		Objective: pilot.Objective, PlanDigest: pilot.PlanDigest, PrimarySampleDigest: pilot.PrimarySampleDigest,
		SourceCorpusDigest: pilot.SourceCorpusDigest, SourceCorpusPlanDigest: pilot.SourceCorpusPlanDigest,
		SourceMutationProgramDigest: pilot.SourceMutationProgramDigest, SourceConstructAuditDigest: pilot.SourceConstructAuditDigest,
		DataRole: pilot.DataRole, SelectionRule: pilot.SelectionRule, SelectedCases: pilot.SelectedCases,
		UniqueSourceIDs: pilot.UniqueSourceIDs, UniqueTaskGroups: pilot.UniqueTaskGroups, UniqueLineageClusters: pilot.UniqueLineageClusters,
		PrimaryOverlap: pilot.PrimaryOverlap, ScarcitySentinelDigest: pilot.ScarcitySentinelDigest, ScarcitySentinelOverlap: pilot.ScarcitySentinelOverlap,
		RequiredPrimaryLabels: pilot.RequiredPrimaryLabels, MaximumTieBreakLabels: pilot.MaximumTieBreakLabels,
		RequiredPostLabelProbes: pilot.RequiredPostLabelProbes, Cases: cases, Bindings: pilot.Bindings,
		ExternalActionStatus: pilot.ExternalActionStatus, EmpiricalStatus: pilot.EmpiricalStatus, Digest: pilot.Digest,
	}
	return adapted, adapted.Validate()
}

func sealRelationPlanV3(plan RelationPlanV3) (RelationPlanV3, error) {
	plan.SchemaVersion, plan.CanonicalPolicy, plan.Digest = PlanSchemaVersionV3, CanonicalPolicy, ""
	digest, err := digestJSON(plan)
	if err != nil {
		return RelationPlanV3{}, err
	}
	plan.Digest = digest
	return plan, plan.Validate()
}

func (plan RelationPlanV3) Validate() error {
	if plan.SchemaVersion != PlanSchemaVersionV3 || plan.CanonicalPolicy != CanonicalPolicy || plan.ProtocolVersion != GovernanceProtocolVersionV3 ||
		plan.Objective != ReviewObjectiveControlledRelation || !validDigest(plan.SourceCorpusDigest) || !validDigest(plan.SourceCorpusPlanDigest) ||
		!validDigest(plan.SourceConstructAuditDigest) || !validDigest(plan.SourceMutationProgramDigest) || strings.TrimSpace(plan.SourceCorpusVersion) == "" ||
		plan.PrimarySampleRule != PrimarySampleRuleV3 || plan.PilotSampleRule != PilotSampleRuleV3 || plan.ScarcitySentinelRule != ScarcitySentinelRuleV3 ||
		plan.PrimarySampleSize != 28 || plan.PilotSampleSize != 7 || plan.ScarcitySentinelSize != 3 || plan.PrimaryReviewers != 2 || plan.TieBreakReviewers != 1 ||
		plan.RubricVersion != "evalwitness.relation-rubric.v3" || plan.UnresolvedRule != UnresolvedRule || plan.SentinelInPrimaryEstimand || plan.HeldOutSentinelAvailable ||
		plan.EmpiricalStatus != EmpiricalStatusNotRun || plan.ExternalActionStatus != ExternalActionNotAuthorized || strings.TrimSpace(plan.RequiredExternalAction) == "" {
		return errors.New("v3 relation plan identity, sample, sentinel, empirical, or authorization contract is invalid")
	}
	if err := uniqueSortedStrings("v3 relation reviewer forbidden inputs", plan.ReviewerForbiddenInputs); err != nil {
		return err
	}
	if !slices.Equal(plan.ReasonCodes, canonicalReasonCodes()) || len(plan.Axes) != 7 || len(plan.CoreFamilies) != 7 || plan.ScarcitySentinel.Family != mutation.FamilyTestEvidenceOmitted {
		return errors.New("v3 relation plan lacks its complete axis, core-family, or scarcity-sentinel surface")
	}
	expectedCore := []mutation.Family{
		mutation.FamilyCandidateOrderReversal, mutation.FamilyCausalIndependentReorder, mutation.FamilyTestEvidenceFalsified,
		mutation.FamilyToolOutputIncomplete, mutation.FamilyIrrelevantVerbosity, mutation.FamilyNeutralFormatting, mutation.FamilyUntrustedScoreInjection,
	}
	for index, contract := range plan.CoreFamilies {
		if contract.Family != expectedCore[index] {
			return fmt.Errorf("v3 core family contract %q is out of order", contract.Family)
		}
		if err := validateFamilyContractV3(contract, plan.Axes); err != nil {
			return fmt.Errorf("v3 core family contract %q is invalid: %w", contract.Family, err)
		}
	}
	if err := validateFamilyContractV3(plan.ScarcitySentinel, plan.Axes); err != nil {
		return fmt.Errorf("v3 scarcity-sentinel contract is invalid: %w", err)
	}
	expected, err := relationPlanV3Digest(plan)
	if err != nil || plan.Digest != expected {
		return errors.New("v3 relation plan digest is invalid")
	}
	return nil
}

func validateFamilyContractV3(contract FamilyContract, axes []AxisDefinition) error {
	definition, exists := mutation.DefinitionFor(contract.Family)
	if !exists || contract.ExpectedRelation != definition.Relation || len(contract.RequiredAxes) == 0 || !slices.IsSorted(contract.RequiredAxes) || hasDuplicate(contract.RequiredAxes) || len(contract.SupportAll) == 0 || len(contract.ContradictAny) == 0 {
		return errors.New("contract contradicts the frozen mutation ontology")
	}
	expectedUnit := UnitTrajectoryPair
	if definition.PairLevel {
		expectedUnit = UnitCandidatePairOrders
	}
	if contract.Unit != expectedUnit {
		return errors.New("contract uses the wrong review unit")
	}
	axisSet := make(map[Axis]struct{}, len(axes))
	for _, axis := range axes {
		axisSet[axis.ID] = struct{}{}
	}
	for _, axis := range contract.RequiredAxes {
		if _, found := axisSet[axis]; !found {
			return fmt.Errorf("contract references unknown axis %q", axis)
		}
	}
	for _, conditions := range [][]TranslationCondition{contract.SupportAll, contract.ContradictAny} {
		if err := validateConditions(contract, conditions, axisSet); err != nil {
			return err
		}
	}
	return nil
}

func relationPlanV3Digest(plan RelationPlanV3) (string, error) {
	plan.Digest = ""
	return digestJSON(plan)
}

type governanceSelectionV3 struct {
	primary  []mutation.CorpusCaseV3
	pilot    []mutation.CorpusCaseV3
	sentinel []mutation.CorpusCaseV3
	sources  map[string]mutation.CorpusSource
}

type governanceBucketV3 struct {
	candidates []mutation.CorpusCaseV3
}

func selectGovernanceV3(plan RelationPlanV3, release mutation.CorpusReleaseV3) (governanceSelectionV3, error) {
	sources := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, source := range release.Sources {
		sources[source.ID] = source
	}
	sentinel := make([]mutation.CorpusCaseV3, 0, plan.ScarcitySentinelSize)
	for _, item := range release.Cases {
		if item.Family == mutation.FamilyTestEvidenceOmitted {
			sentinel = append(sentinel, item)
		}
	}
	sort.Slice(sentinel, func(left, right int) bool { return sentinel[left].ID < sentinel[right].ID })
	if len(sentinel) != plan.ScarcitySentinelSize {
		return governanceSelectionV3{}, errors.New("v3 relation governance cannot reproduce the exhaustive scarcity sentinel")
	}
	sentinelSources, sentinelGroups, sentinelLineages := caseIdentitiesV3(sentinel, sources)
	buckets := make([]governanceBucketV3, 0, len(plan.CoreFamilies)*2)
	for _, contract := range plan.CoreFamilies {
		for _, role := range []study.DataRole{study.RoleCalibration, study.RoleTest} {
			value := governanceBucketV3{}
			for _, item := range release.Cases {
				if item.Family == contract.Family && item.Split == role {
					value.candidates = append(value.candidates, item)
				}
			}
			sort.Slice(value.candidates, func(left, right int) bool { return value.candidates[left].ID < value.candidates[right].ID })
			if len(value.candidates) < 2 {
				return governanceSelectionV3{}, fmt.Errorf("v3 primary bucket %s/%s has fewer than two cases", contract.Family, role)
			}
			buckets = append(buckets, value)
		}
	}
	primary, pilot, ok := selectPrimaryBucketsV3(plan, release, sources, buckets, sentinelSources, sentinelGroups, sentinelLineages, 0, nil, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{})
	if !ok {
		return governanceSelectionV3{}, errors.New("v3 relation governance cannot jointly satisfy primary, pilot, and scarcity-sentinel independence")
	}
	return governanceSelectionV3{primary: primary, pilot: pilot, sentinel: sentinel, sources: sources}, nil
}

func selectPrimaryBucketsV3(plan RelationPlanV3, release mutation.CorpusReleaseV3, sources map[string]mutation.CorpusSource, buckets []governanceBucketV3, sentinelSources, sentinelGroups, sentinelLineages map[string]struct{}, index int, selected []mutation.CorpusCaseV3, usedSources, usedGroups, usedLineages map[string]struct{}) ([]mutation.CorpusCaseV3, []mutation.CorpusCaseV3, bool) {
	if index == len(buckets) {
		pilot, ok := selectPilotV3(plan, release, sources, usedSources, usedGroups, usedLineages, sentinelSources, sentinelGroups, sentinelLineages)
		if !ok {
			return nil, nil, false
		}
		return append([]mutation.CorpusCaseV3(nil), selected...), pilot, true
	}
	candidates := buckets[index].candidates
	for left := 0; left < len(candidates)-1; left++ {
		leftSources, leftGroups, leftLineages := caseIdentitiesV3([]mutation.CorpusCaseV3{candidates[left]}, sources)
		if identityOverlapV3(leftSources, leftGroups, leftLineages, usedSources, usedGroups, usedLineages) || identityOverlapV3(leftSources, leftGroups, leftLineages, sentinelSources, sentinelGroups, sentinelLineages) {
			continue
		}
		for right := left + 1; right < len(candidates); right++ {
			rightSources, rightGroups, rightLineages := caseIdentitiesV3([]mutation.CorpusCaseV3{candidates[right]}, sources)
			if identityOverlapV3(rightSources, rightGroups, rightLineages, usedSources, usedGroups, usedLineages) || identityOverlapV3(rightSources, rightGroups, rightLineages, sentinelSources, sentinelGroups, sentinelLineages) || identityOverlapV3(rightSources, rightGroups, rightLineages, leftSources, leftGroups, leftLineages) {
				continue
			}
			addIdentitiesV3(usedSources, leftSources, rightSources)
			addIdentitiesV3(usedGroups, leftGroups, rightGroups)
			addIdentitiesV3(usedLineages, leftLineages, rightLineages)
			selected = append(selected, candidates[left], candidates[right])
			if primary, pilot, ok := selectPrimaryBucketsV3(plan, release, sources, buckets, sentinelSources, sentinelGroups, sentinelLineages, index+1, selected, usedSources, usedGroups, usedLineages); ok {
				return primary, pilot, true
			}
			selected = selected[:len(selected)-2]
			removeIdentitiesV3(usedSources, leftSources, rightSources)
			removeIdentitiesV3(usedGroups, leftGroups, rightGroups)
			removeIdentitiesV3(usedLineages, leftLineages, rightLineages)
		}
	}
	return nil, nil, false
}

func selectPilotV3(plan RelationPlanV3, release mutation.CorpusReleaseV3, sources map[string]mutation.CorpusSource, primarySources, primaryGroups, primaryLineages, sentinelSources, sentinelGroups, sentinelLineages map[string]struct{}) ([]mutation.CorpusCaseV3, bool) {
	candidates := make(map[mutation.Family][]mutation.CorpusCaseV3, len(plan.CoreFamilies))
	for _, item := range release.Cases {
		if item.Split != study.RoleDevelopment || item.Family == mutation.FamilyTestEvidenceOmitted {
			continue
		}
		itemSources, itemGroups, itemLineages := caseIdentitiesV3([]mutation.CorpusCaseV3{item}, sources)
		if identityOverlapV3(itemSources, itemGroups, itemLineages, primarySources, primaryGroups, primaryLineages) || identityOverlapV3(itemSources, itemGroups, itemLineages, sentinelSources, sentinelGroups, sentinelLineages) {
			continue
		}
		candidates[item.Family] = append(candidates[item.Family], item)
	}
	for family := range candidates {
		sort.Slice(candidates[family], func(left, right int) bool { return candidates[family][left].ID < candidates[family][right].ID })
	}
	return selectPilotFamiliesV3(plan.CoreFamilies, candidates, sources, 0, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{})
}

func selectPilotFamiliesV3(contracts []FamilyContract, candidates map[mutation.Family][]mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource, index int, usedSources, usedGroups, usedLineages map[string]struct{}) ([]mutation.CorpusCaseV3, bool) {
	if index == len(contracts) {
		return []mutation.CorpusCaseV3{}, true
	}
	for _, item := range candidates[contracts[index].Family] {
		itemSources, itemGroups, itemLineages := caseIdentitiesV3([]mutation.CorpusCaseV3{item}, sources)
		if identityOverlapV3(itemSources, itemGroups, itemLineages, usedSources, usedGroups, usedLineages) {
			continue
		}
		addIdentitiesV3(usedSources, itemSources)
		addIdentitiesV3(usedGroups, itemGroups)
		addIdentitiesV3(usedLineages, itemLineages)
		rest, ok := selectPilotFamiliesV3(contracts, candidates, sources, index+1, usedSources, usedGroups, usedLineages)
		if ok {
			return append([]mutation.CorpusCaseV3{item}, rest...), true
		}
		removeIdentitiesV3(usedSources, itemSources)
		removeIdentitiesV3(usedGroups, itemGroups)
		removeIdentitiesV3(usedLineages, itemLineages)
	}
	return nil, false
}

func caseIdentitiesV3(cases []mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	sourceIDs, groups, lineages := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, item := range cases {
		groups[item.Manifest.SplitGroupID] = struct{}{}
		for _, sourceID := range item.SourceIDs {
			sourceIDs[sourceID] = struct{}{}
			lineages[sources[sourceID].LineageClusterID] = struct{}{}
		}
	}
	return sourceIDs, groups, lineages
}

func identityOverlapV3(sourceIDs, groups, lineages, usedSources, usedGroups, usedLineages map[string]struct{}) bool {
	return setOverlapsV3(sourceIDs, usedSources) || setOverlapsV3(groups, usedGroups) || setOverlapsV3(lineages, usedLineages)
}

func setOverlapsV3(left, right map[string]struct{}) bool {
	for value := range left {
		if _, found := right[value]; found {
			return true
		}
	}
	return false
}

func addIdentitiesV3(target map[string]struct{}, values ...map[string]struct{}) {
	for _, set := range values {
		for value := range set {
			target[value] = struct{}{}
		}
	}
}

func removeIdentitiesV3(target map[string]struct{}, values ...map[string]struct{}) {
	for _, set := range values {
		for value := range set {
			delete(target, value)
		}
	}
}

func BuildPrimarySampleV3(plan RelationPlanV3, release mutation.CorpusReleaseV3) (PrimarySampleV3, error) {
	if err := plan.Validate(); err != nil {
		return PrimarySampleV3{}, err
	}
	selection, err := selectGovernanceV3(plan, release)
	if err != nil {
		return PrimarySampleV3{}, err
	}
	references, bindings, countsByFamily, countsBySplit, countsByControl, countsByFormat, err := summarizeCasesV3(selection.primary, selection.sources)
	if err != nil {
		return PrimarySampleV3{}, err
	}
	caseIDs := make([]string, 0, len(references))
	trajectoryUnits, candidateUnits := 0, 0
	for _, item := range references {
		caseIDs = append(caseIDs, item.CaseID)
		if item.Unit == UnitCandidatePairOrders {
			candidateUnits++
		} else {
			trajectoryUnits++
		}
	}
	selectionDigest, err := digestJSON(caseIDs)
	if err != nil {
		return PrimarySampleV3{}, err
	}
	sourceIDs, groups, lineages := caseIdentitiesV3(selection.primary, selection.sources)
	sample := PrimarySampleV3{
		ProtocolVersion: plan.ProtocolVersion, Objective: plan.Objective, PlanDigest: plan.Digest,
		SourceCorpusDigest: plan.SourceCorpusDigest, SourceCorpusPlanDigest: plan.SourceCorpusPlanDigest,
		SourceConstructAuditDigest: plan.SourceConstructAuditDigest, SourceMutationProgramDigest: plan.SourceMutationProgramDigest,
		SelectionRule: plan.PrimarySampleRule, SelectedCases: len(selection.primary), UniqueSourceIDs: len(sourceIDs), UniqueTaskGroups: len(groups), UniqueLineageClusters: len(lineages),
		TrajectoryPairUnits: trajectoryUnits, CandidateOrderUnits: candidateUnits, SelectionDigest: selectionDigest,
		FamilyCounts: countsByFamily, SplitCounts: countsBySplit, ControlCounts: countsByControl, SourceFormatCounts: countsByFormat,
		Cases: references, Bindings: bindings, EmpiricalStatus: EmpiricalStatusNotRun, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	sealed, err := sealPrimarySampleV3(sample)
	if err != nil {
		return PrimarySampleV3{}, err
	}
	return sealed, sealed.Validate(plan)
}

func BuildScarcitySentinelV3(plan RelationPlanV3, primary PrimarySampleV3, release mutation.CorpusReleaseV3) (ScarcitySentinelV3, error) {
	if err := primary.Validate(plan); err != nil {
		return ScarcitySentinelV3{}, err
	}
	selection, err := selectGovernanceV3(plan, release)
	if err != nil {
		return ScarcitySentinelV3{}, err
	}
	expectedPrimary, err := BuildPrimarySampleV3(plan, release)
	if err != nil || expectedPrimary.Digest != primary.Digest {
		return ScarcitySentinelV3{}, errors.New("v3 scarcity sentinel primary sample does not reproduce")
	}
	references, bindings, _, splitCounts, _, _, err := summarizeCasesV3(selection.sentinel, selection.sources)
	if err != nil {
		return ScarcitySentinelV3{}, err
	}
	primarySources, primaryGroups, primaryLineages := caseIdentitiesV3(selection.primary, selection.sources)
	sentinelSources, sentinelGroups, sentinelLineages := caseIdentitiesV3(selection.sentinel, selection.sources)
	overlap := 0
	if identityOverlapV3(sentinelSources, sentinelGroups, sentinelLineages, primarySources, primaryGroups, primaryLineages) {
		overlap = 1
	}
	sentinel := ScarcitySentinelV3{
		ProtocolVersion: plan.ProtocolVersion, Objective: plan.Objective, PlanDigest: plan.Digest, PrimarySampleDigest: primary.Digest,
		SourceCorpusDigest: plan.SourceCorpusDigest, SourceCorpusPlanDigest: plan.SourceCorpusPlanDigest,
		SourceConstructAuditDigest: plan.SourceConstructAuditDigest, SourceMutationProgramDigest: plan.SourceMutationProgramDigest,
		SelectionRule: plan.ScarcitySentinelRule, Family: mutation.FamilyTestEvidenceOmitted, SelectedCases: len(selection.sentinel), SplitCounts: splitCounts,
		TestCases: countFor(splitCounts, string(study.RoleTest)), PrimaryOverlap: overlap, Exhaustive: true,
		AnalysisUse: ScarcitySentinelAnalysisUseV3, HeldOutClaimAvailable: false, Cases: references, Bindings: bindings,
		EmpiricalStatus: EmpiricalStatusNotRun, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	sealed, err := sealScarcitySentinelV3(sentinel)
	if err != nil {
		return ScarcitySentinelV3{}, err
	}
	return sealed, sealed.Validate(plan, primary)
}

func BuildPilotSampleV3(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, release mutation.CorpusReleaseV3) (PilotSampleV3, error) {
	if err := primary.Validate(plan); err != nil {
		return PilotSampleV3{}, err
	}
	if err := sentinel.Validate(plan, primary); err != nil {
		return PilotSampleV3{}, err
	}
	selection, err := selectGovernanceV3(plan, release)
	if err != nil {
		return PilotSampleV3{}, err
	}
	expectedPrimary, err := BuildPrimarySampleV3(plan, release)
	if err != nil || expectedPrimary.Digest != primary.Digest {
		return PilotSampleV3{}, errors.New("v3 pilot primary sample does not reproduce")
	}
	expectedSentinel, err := BuildScarcitySentinelV3(plan, primary, release)
	if err != nil || expectedSentinel.Digest != sentinel.Digest {
		return PilotSampleV3{}, errors.New("v3 pilot scarcity sentinel does not reproduce")
	}
	references, bindings, _, _, _, _, err := summarizeCasesV3(selection.pilot, selection.sources)
	if err != nil {
		return PilotSampleV3{}, err
	}
	sourceIDs, groups, lineages := caseIdentitiesV3(selection.pilot, selection.sources)
	pilot := PilotSampleV3{
		ProtocolVersion: plan.ProtocolVersion, Objective: plan.Objective, PlanDigest: plan.Digest, PrimarySampleDigest: primary.Digest, ScarcitySentinelDigest: sentinel.Digest,
		SourceCorpusDigest: plan.SourceCorpusDigest, SourceCorpusPlanDigest: plan.SourceCorpusPlanDigest,
		SourceConstructAuditDigest: plan.SourceConstructAuditDigest, SourceMutationProgramDigest: plan.SourceMutationProgramDigest,
		DataRole: string(study.RoleDevelopment), SelectionRule: plan.PilotSampleRule, SelectedCases: len(selection.pilot),
		UniqueSourceIDs: len(sourceIDs), UniqueTaskGroups: len(groups), UniqueLineageClusters: len(lineages), PrimaryOverlap: 0, ScarcitySentinelOverlap: 0,
		RequiredPrimaryLabels: len(selection.pilot) * plan.PrimaryReviewers, MaximumTieBreakLabels: len(selection.pilot) * plan.TieBreakReviewers,
		RequiredPostLabelProbes: len(selection.pilot) * plan.PrimaryReviewers, Cases: references, Bindings: bindings,
		EmpiricalStatus: EmpiricalStatusNotRun, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	sealed, err := sealPilotSampleV3(pilot)
	if err != nil {
		return PilotSampleV3{}, err
	}
	return sealed, sealed.Validate(plan, primary, sentinel)
}

func summarizeCasesV3(cases []mutation.CorpusCaseV3, sources map[string]mutation.CorpusSource) ([]GovernedCaseReferenceV3, BindingCommitments, []Count, []Count, []Count, []Count, error) {
	ordered := append([]mutation.CorpusCaseV3(nil), cases...)
	sort.Slice(ordered, func(left, right int) bool {
		leftKey := string(ordered[left].Family) + "\x00" + string(ordered[left].Split) + "\x00" + ordered[left].ID
		rightKey := string(ordered[right].Family) + "\x00" + string(ordered[right].Split) + "\x00" + ordered[right].ID
		return leftKey < rightKey
	})
	rows := map[string][]string{"cases": {}, "sources": {}, "programs": {}, "manifests": {}, "witnesses": {}, "licenses": {}, "privacy": {}, "lineage": {}, "packets": {}, "regeneration": {}, "construct_firewalls": {}}
	familyCounts, splitCounts, controlCounts, sourceFormatCounts := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	references := make([]GovernedCaseReferenceV3, 0, len(ordered))
	for _, item := range ordered {
		linked := make([]mutation.CorpusSource, 0, len(item.SourceIDs))
		lineageIDs := make([]string, 0, len(item.SourceIDs))
		lineageRows := make([]lineageBinding, 0, len(item.SourceIDs))
		for _, sourceID := range item.SourceIDs {
			source := sources[sourceID]
			linked = append(linked, source)
			lineageIDs = append(lineageIDs, source.LineageClusterID)
			lineageRows = append(lineageRows, lineageBinding{RepositoryID: source.RepositoryID, TaskID: source.TaskID, SplitGroupID: source.SplitGroupID, NearDuplicateID: source.NearDuplicateID, LineageClusterID: source.LineageClusterID, PatchDigest: source.PatchDigest})
			sourceFormatCounts[string(source.SourceFormat)]++
		}
		lineageIDs = uniqueSorted(lineageIDs)
		digests, err := deepCaseBindingDigestsV3(item, linked, lineageRows)
		if err != nil {
			return nil, BindingCommitments{}, nil, nil, nil, nil, err
		}
		for name, digest := range digests {
			rows[name] = append(rows[name], item.ID+"\x00"+digest)
		}
		definition, _ := mutation.DefinitionFor(item.Family)
		unit := UnitTrajectoryPair
		if definition.PairLevel {
			unit = UnitCandidatePairOrders
		}
		familyCounts[string(item.Family)]++
		splitCounts[string(item.Split)]++
		controlCounts[item.Control]++
		references = append(references, GovernedCaseReferenceV3{
			Family: item.Family, CaseID: item.ID, DataRole: item.Split, Unit: unit, TaskGroupID: item.Manifest.SplitGroupID,
			SourceIDs: append([]string(nil), item.SourceIDs...), LineageClusterIDs: lineageIDs, CaseBindingDigest: digests["cases"], ConstructFirewallDigest: item.ConstructFirewall.Digest,
		})
	}
	bindings := BindingCommitments{
		Cases: aggregate(rows["cases"]), Sources: aggregate(rows["sources"]), Programs: aggregate(rows["programs"]), Manifests: aggregate(rows["manifests"]),
		Witnesses: aggregate(rows["witnesses"]), Licenses: aggregate(rows["licenses"]), Privacy: aggregate(rows["privacy"]), Lineage: aggregate(rows["lineage"]),
		Packets: aggregate(rows["packets"]), Regeneration: aggregate(rows["regeneration"]), ConstructFirewalls: aggregate(rows["construct_firewalls"]),
	}
	return references, bindings, counts(familyCounts), counts(splitCounts), counts(controlCounts), counts(sourceFormatCounts), nil
}

func deepCaseBindingDigestsV3(item mutation.CorpusCaseV3, sources []mutation.CorpusSource, lineage []lineageBinding) (map[string]string, error) {
	caseBinding := struct {
		ID                      string            `json:"id"`
		Family                  mutation.Family   `json:"family"`
		Split                   string            `json:"split"`
		Control                 string            `json:"control"`
		ExpectedRelation        mutation.Relation `json:"expected_relation"`
		SourceIDs               []string          `json:"source_ids"`
		ManifestDigest          string            `json:"manifest_digest"`
		PacketDigest            string            `json:"packet_digest"`
		RegenerationKey         string            `json:"regeneration_key"`
		ConstructFirewallDigest string            `json:"construct_firewall_digest"`
	}{item.ID, item.Family, string(item.Split), item.Control, item.Manifest.ExpectedRelation, item.SourceIDs, item.Manifest.Digest, item.BlindPacket.Digest, item.RegenerationKey, item.ConstructFirewall.Digest}
	values := map[string]any{
		"cases": caseBinding, "sources": sources, "programs": item.Manifest.Program, "manifests": item.Manifest.Digest,
		"witnesses": item.Manifest.Witness.Digest, "licenses": item.Manifest.License, "privacy": item.Manifest.Privacy,
		"lineage": lineage, "packets": item.BlindPacket.Digest, "regeneration": item.RegenerationKey, "construct_firewalls": item.ConstructFirewall,
	}
	result := make(map[string]string, len(values))
	for name, value := range values {
		digest, err := digestJSON(value)
		if err != nil {
			return nil, err
		}
		result[name] = digest
	}
	return result, nil
}

func sealPrimarySampleV3(sample PrimarySampleV3) (PrimarySampleV3, error) {
	sample.SchemaVersion, sample.CanonicalPolicy, sample.Digest = PrimarySampleSchemaVersionV3, CanonicalPolicy, ""
	digest, err := digestJSON(sample)
	if err != nil {
		return PrimarySampleV3{}, err
	}
	sample.Digest = digest
	return sample, nil
}

func (sample PrimarySampleV3) Validate(plan RelationPlanV3) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if sample.SchemaVersion != PrimarySampleSchemaVersionV3 || sample.CanonicalPolicy != CanonicalPolicy || sample.ProtocolVersion != plan.ProtocolVersion || sample.Objective != plan.Objective ||
		sample.PlanDigest != plan.Digest || sample.SourceCorpusDigest != plan.SourceCorpusDigest || sample.SourceCorpusPlanDigest != plan.SourceCorpusPlanDigest ||
		sample.SourceConstructAuditDigest != plan.SourceConstructAuditDigest || sample.SourceMutationProgramDigest != plan.SourceMutationProgramDigest ||
		sample.SelectionRule != PrimarySampleRuleV3 || sample.SelectedCases != 28 || sample.UniqueTaskGroups != 28 || sample.UniqueLineageClusters != 28 ||
		sample.TrajectoryPairUnits != 24 || sample.CandidateOrderUnits != 4 || len(sample.Cases) != 28 || sample.EmpiricalStatus != EmpiricalStatusNotRun || sample.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("v3 primary sample identity, independence, unit, or empirical boundary is invalid")
	}
	if len(sample.FamilyCounts) != 7 || len(sample.SplitCounts) != 2 || countFor(sample.SplitCounts, string(study.RoleCalibration)) != 14 || countFor(sample.SplitCounts, string(study.RoleTest)) != 14 {
		return errors.New("v3 primary sample balance is invalid")
	}
	for _, row := range sample.FamilyCounts {
		if row.Count != 4 || row.ID == string(mutation.FamilyTestEvidenceOmitted) {
			return errors.New("v3 primary sample contains an unbalanced or scarcity-sentinel family")
		}
	}
	if err := validateCaseReferencesV3(sample.Cases, sample.SelectedCases, true); err != nil {
		return err
	}
	caseIDs, familyCounts, splitCounts, sourceIDs, groups, lineages := summarizeReferencesV3(sample.Cases)
	selectionDigest, err := digestJSON(caseIDs)
	if err != nil || selectionDigest != sample.SelectionDigest || !slices.Equal(familyCounts, sample.FamilyCounts) || !slices.Equal(splitCounts, sample.SplitCounts) ||
		len(sourceIDs) != sample.UniqueSourceIDs || len(groups) != sample.UniqueTaskGroups || len(lineages) != sample.UniqueLineageClusters {
		return errors.New("v3 primary sample selection digest, counts, or independence denominators do not reproduce")
	}
	if err := validateBindingCommitmentsV3(sample.Bindings); err != nil {
		return err
	}
	expected, err := primarySampleV3Digest(sample)
	if err != nil || sample.Digest != expected {
		return errors.New("v3 primary sample digest is invalid")
	}
	return nil
}

func sealPilotSampleV3(sample PilotSampleV3) (PilotSampleV3, error) {
	sample.SchemaVersion, sample.CanonicalPolicy, sample.Digest = PilotSampleSchemaVersionV3, CanonicalPolicy, ""
	digest, err := digestJSON(sample)
	if err != nil {
		return PilotSampleV3{}, err
	}
	sample.Digest = digest
	return sample, nil
}

func (sample PilotSampleV3) Validate(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3) error {
	if err := primary.Validate(plan); err != nil {
		return err
	}
	if err := sentinel.Validate(plan, primary); err != nil {
		return err
	}
	if sample.SchemaVersion != PilotSampleSchemaVersionV3 || sample.CanonicalPolicy != CanonicalPolicy || sample.ProtocolVersion != plan.ProtocolVersion || sample.Objective != plan.Objective ||
		sample.PlanDigest != plan.Digest || sample.PrimarySampleDigest != primary.Digest || sample.ScarcitySentinelDigest != sentinel.Digest || sample.SourceCorpusDigest != plan.SourceCorpusDigest ||
		sample.SourceCorpusPlanDigest != plan.SourceCorpusPlanDigest || sample.SourceConstructAuditDigest != plan.SourceConstructAuditDigest || sample.SourceMutationProgramDigest != plan.SourceMutationProgramDigest ||
		sample.DataRole != string(study.RoleDevelopment) || sample.SelectionRule != PilotSampleRuleV3 || sample.SelectedCases != 7 || sample.UniqueTaskGroups != 7 || sample.UniqueLineageClusters != 7 ||
		sample.PrimaryOverlap != 0 || sample.ScarcitySentinelOverlap != 0 || sample.RequiredPrimaryLabels != 14 || sample.MaximumTieBreakLabels != 7 || sample.RequiredPostLabelProbes != 14 ||
		len(sample.Cases) != 7 || sample.EmpiricalStatus != EmpiricalStatusNotRun || sample.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("v3 pilot identity, independence, workload, or empirical boundary is invalid")
	}
	if err := validateCaseReferencesV3(sample.Cases, sample.SelectedCases, true); err != nil {
		return err
	}
	_, _, _, pilotSources, pilotGroups, pilotLineages := summarizeReferencesV3(sample.Cases)
	_, _, _, primarySources, primaryGroups, primaryLineages := summarizeReferencesV3(primary.Cases)
	_, _, _, sentinelSources, sentinelGroups, sentinelLineages := summarizeReferencesV3(sentinel.Cases)
	if len(pilotSources) != sample.UniqueSourceIDs || len(pilotGroups) != sample.UniqueTaskGroups || len(pilotLineages) != sample.UniqueLineageClusters ||
		identityOverlapV3(pilotSources, pilotGroups, pilotLineages, primarySources, primaryGroups, primaryLineages) ||
		identityOverlapV3(pilotSources, pilotGroups, pilotLineages, sentinelSources, sentinelGroups, sentinelLineages) {
		return errors.New("v3 pilot sample overlap or independence denominators do not reproduce")
	}
	if err := validateBindingCommitmentsV3(sample.Bindings); err != nil {
		return err
	}
	expected, err := pilotSampleV3Digest(sample)
	if err != nil || sample.Digest != expected {
		return errors.New("v3 pilot sample digest is invalid")
	}
	return nil
}

func sealScarcitySentinelV3(sentinel ScarcitySentinelV3) (ScarcitySentinelV3, error) {
	sentinel.SchemaVersion, sentinel.CanonicalPolicy, sentinel.Digest = ScarcitySentinelSchemaVersionV3, CanonicalPolicy, ""
	digest, err := digestJSON(sentinel)
	if err != nil {
		return ScarcitySentinelV3{}, err
	}
	sentinel.Digest = digest
	return sentinel, nil
}

func (sentinel ScarcitySentinelV3) Validate(plan RelationPlanV3, primary PrimarySampleV3) error {
	if err := primary.Validate(plan); err != nil {
		return err
	}
	if sentinel.SchemaVersion != ScarcitySentinelSchemaVersionV3 || sentinel.CanonicalPolicy != CanonicalPolicy || sentinel.ProtocolVersion != plan.ProtocolVersion || sentinel.Objective != plan.Objective ||
		sentinel.PlanDigest != plan.Digest || sentinel.PrimarySampleDigest != primary.Digest || sentinel.SourceCorpusDigest != plan.SourceCorpusDigest || sentinel.SourceCorpusPlanDigest != plan.SourceCorpusPlanDigest ||
		sentinel.SourceConstructAuditDigest != plan.SourceConstructAuditDigest || sentinel.SourceMutationProgramDigest != plan.SourceMutationProgramDigest || sentinel.SelectionRule != ScarcitySentinelRuleV3 ||
		sentinel.Family != mutation.FamilyTestEvidenceOmitted || sentinel.SelectedCases != 3 || sentinel.TestCases != 0 || sentinel.PrimaryOverlap != 0 || !sentinel.Exhaustive ||
		sentinel.AnalysisUse != ScarcitySentinelAnalysisUseV3 || sentinel.HeldOutClaimAvailable || len(sentinel.Cases) != 3 || sentinel.EmpiricalStatus != EmpiricalStatusNotRun || sentinel.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("v3 scarcity sentinel identity, exhaustiveness, split, or claim boundary is invalid")
	}
	if len(sentinel.SplitCounts) != 2 || countFor(sentinel.SplitCounts, string(study.RoleDevelopment)) != 2 || countFor(sentinel.SplitCounts, string(study.RoleCalibration)) != 1 || countFor(sentinel.SplitCounts, string(study.RoleTest)) != 0 {
		return errors.New("v3 scarcity sentinel roles do not reproduce 2 development, 1 calibration, and 0 test")
	}
	if err := validateCaseReferencesV3(sentinel.Cases, sentinel.SelectedCases, true); err != nil {
		return err
	}
	for _, item := range sentinel.Cases {
		if item.Family != mutation.FamilyTestEvidenceOmitted {
			return errors.New("v3 scarcity sentinel contains a core-family case")
		}
	}
	_, _, splitCounts, sentinelSources, sentinelGroups, sentinelLineages := summarizeReferencesV3(sentinel.Cases)
	_, _, _, primarySources, primaryGroups, primaryLineages := summarizeReferencesV3(primary.Cases)
	if !slices.Equal(splitCounts, sentinel.SplitCounts) || identityOverlapV3(sentinelSources, sentinelGroups, sentinelLineages, primarySources, primaryGroups, primaryLineages) {
		return errors.New("v3 scarcity sentinel split counts or primary independence do not reproduce")
	}
	if err := validateBindingCommitmentsV3(sentinel.Bindings); err != nil {
		return err
	}
	expected, err := scarcitySentinelV3Digest(sentinel)
	if err != nil || sentinel.Digest != expected {
		return errors.New("v3 scarcity sentinel digest is invalid")
	}
	return nil
}

func validateCaseReferencesV3(values []GovernedCaseReferenceV3, expected int, requireUniqueLineage bool) error {
	if len(values) != expected {
		return errors.New("v3 governed case reference count is invalid")
	}
	seenCases, seenSources, seenGroups, seenLineages := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	previous := ""
	for _, item := range values {
		key := string(item.Family) + "\x00" + string(item.DataRole) + "\x00" + item.CaseID
		if key <= previous || strings.TrimSpace(item.CaseID) == "" || strings.TrimSpace(item.TaskGroupID) == "" || len(item.SourceIDs) == 0 || len(item.LineageClusterIDs) == 0 ||
			!validDigest(item.CaseBindingDigest) || !validDigest(item.ConstructFirewallDigest) {
			return errors.New("v3 governed case references are incomplete or not canonically sorted")
		}
		previous = key
		if _, duplicate := seenCases[item.CaseID]; duplicate {
			return errors.New("v3 governed case references reuse a case")
		}
		if _, duplicate := seenGroups[item.TaskGroupID]; duplicate {
			return errors.New("v3 governed case references reuse a task group")
		}
		seenCases[item.CaseID], seenGroups[item.TaskGroupID] = struct{}{}, struct{}{}
		for _, sourceID := range item.SourceIDs {
			if _, duplicate := seenSources[sourceID]; duplicate {
				return errors.New("v3 governed case references reuse a source")
			}
			seenSources[sourceID] = struct{}{}
		}
		for _, lineageID := range item.LineageClusterIDs {
			if _, duplicate := seenLineages[lineageID]; duplicate && requireUniqueLineage {
				return errors.New("v3 governed case references reuse a lineage cluster")
			}
			seenLineages[lineageID] = struct{}{}
		}
	}
	return nil
}

func summarizeReferencesV3(values []GovernedCaseReferenceV3) ([]string, []Count, []Count, map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	caseIDs := make([]string, 0, len(values))
	families, splits := map[string]int{}, map[string]int{}
	sources, groups, lineages := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, item := range values {
		caseIDs = append(caseIDs, item.CaseID)
		families[string(item.Family)]++
		splits[string(item.DataRole)]++
		groups[item.TaskGroupID] = struct{}{}
		for _, sourceID := range item.SourceIDs {
			sources[sourceID] = struct{}{}
		}
		for _, lineageID := range item.LineageClusterIDs {
			lineages[lineageID] = struct{}{}
		}
	}
	return caseIDs, counts(families), counts(splits), sources, groups, lineages
}

func validateBindingCommitmentsV3(bindings BindingCommitments) error {
	for _, digest := range []string{bindings.Cases, bindings.Sources, bindings.Programs, bindings.Manifests, bindings.Witnesses, bindings.Licenses, bindings.Privacy, bindings.Lineage, bindings.Packets, bindings.Regeneration, bindings.ConstructFirewalls} {
		if !validDigest(digest) {
			return errors.New("v3 governance contains an invalid deep binding commitment")
		}
	}
	return nil
}

func primarySampleV3Digest(sample PrimarySampleV3) (string, error) {
	sample.Digest = ""
	return digestJSON(sample)
}

func pilotSampleV3Digest(sample PilotSampleV3) (string, error) {
	sample.Digest = ""
	return digestJSON(sample)
}

func scarcitySentinelV3Digest(sentinel ScarcitySentinelV3) (string, error) {
	sentinel.Digest = ""
	return digestJSON(sentinel)
}

func BuildStudyAmendmentV3(plan RelationPlanV3, pilot PilotSampleV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, issuedAt string) (StudyAmendmentV3, error) {
	if err := pilot.Validate(plan, primary, sentinel); err != nil {
		return StudyAmendmentV3{}, err
	}
	parsed, err := time.Parse(time.RFC3339, issuedAt)
	if err != nil || parsed.Location() != time.UTC {
		return StudyAmendmentV3{}, errors.New("v3 relation amendment issued_at must be UTC RFC3339")
	}
	alpha := 0.05
	amendment := StudyAmendmentV3{
		ProtocolVersion: plan.ProtocolVersion, Objective: plan.Objective, IssuedAt: issuedAt, PlanDigest: plan.Digest,
		PilotSampleDigest: pilot.Digest, PrimarySampleDigest: primary.Digest, ScarcitySentinelDigest: sentinel.Digest,
		SourceCorpusDigest: plan.SourceCorpusDigest, SourceCorpusPlanDigest: plan.SourceCorpusPlanDigest,
		SourceConstructAuditDigest: plan.SourceConstructAuditDigest, SourceMutationProgramDigest: plan.SourceMutationProgramDigest,
		Pilot: PilotDesign{
			Cases: 7, PrimaryLabels: 14, MaximumTieBreakLabels: 7, PostLabelProbes: 14, DataRole: string(study.RoleDevelopment), PrimaryOverlap: 0,
			Purpose: "materialization, comprehension, timing, qualification, ambiguity, and leakage only", PrimaryAnalysisUse: "forbidden",
		},
		Primary: PrimaryDesign{
			Cases: 28, EffectiveTaskGroups: 28, PrimaryLabels: 56, MaximumTieBreakLabels: 28, PostLabelProbes: 56,
			ClusterUnit: "source_task_group", AggregationRule: primaryAggregationRule, ReplacementRule: primaryReplacementRuleV3,
			MissingnessRule: primaryMissingnessRule, StoppingRule: primaryStoppingRuleV3,
		},
		ScarcitySentinel: ScarcitySentinelDesignV3{
			Cases: 3, SplitCounts: sentinel.SplitCounts, TestCases: 0, PrimaryOverlap: 0, Exhaustive: true,
			PrimaryAnalysisUse: "forbidden", HeldOutClaimAvailable: false,
		},
		Inference: RelationInference{
			PrimaryEstimand: primaryEstimand, NominalAlpha: alpha, IntervalMethod: intervalMethod, MultiplicityMethod: multiplicityMethod,
			PrimaryMultiplicityFamily: []string{"cluster_contradiction_prevalence"}, FamilyAnalysisRole: familyAnalysisRole,
			ZeroContradictionUpperBound: zeroContradictionUpperBound(28, alpha),
			DetectionScenarios: []DetectionScenario{
				{ContradictionRate: 0.05, DetectionProbability: detectionProbability(28, 0.05)},
				{ContradictionRate: 0.10, DetectionProbability: detectionProbability(28, 0.10)},
				{ContradictionRate: 0.20, DetectionProbability: detectionProbability(28, 0.20)},
			},
			UnresolvedDenominatorRule: unresolvedDenominatorRule, IndependenceLimitation: independenceLimitationV3,
		},
		ClaimBoundary: claimBoundaryV3, EmpiricalStatus: EmpiricalStatusNotRun, ExternalActionStatus: ExternalActionNotAuthorized,
	}
	sealed, err := sealStudyAmendmentV3(amendment)
	if err != nil {
		return StudyAmendmentV3{}, err
	}
	return sealed, sealed.Validate(plan, pilot, primary, sentinel)
}

func sealStudyAmendmentV3(amendment StudyAmendmentV3) (StudyAmendmentV3, error) {
	amendment.SchemaVersion, amendment.CanonicalPolicy, amendment.Digest = StudyAmendmentSchemaVersionV3, CanonicalPolicy, ""
	digest, err := digestJSON(amendment)
	if err != nil {
		return StudyAmendmentV3{}, err
	}
	amendment.Digest = digest
	return amendment, nil
}

func (amendment StudyAmendmentV3) Validate(plan RelationPlanV3, pilot PilotSampleV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3) error {
	if err := pilot.Validate(plan, primary, sentinel); err != nil {
		return err
	}
	issuedAt, issuedErr := time.Parse(time.RFC3339, amendment.IssuedAt)
	if amendment.SchemaVersion != StudyAmendmentSchemaVersionV3 || amendment.CanonicalPolicy != CanonicalPolicy || amendment.ProtocolVersion != plan.ProtocolVersion || amendment.Objective != plan.Objective ||
		issuedErr != nil || issuedAt.Location() != time.UTC || amendment.PlanDigest != plan.Digest || amendment.PilotSampleDigest != pilot.Digest || amendment.PrimarySampleDigest != primary.Digest ||
		amendment.ScarcitySentinelDigest != sentinel.Digest || amendment.SourceCorpusDigest != plan.SourceCorpusDigest || amendment.SourceCorpusPlanDigest != plan.SourceCorpusPlanDigest ||
		amendment.SourceConstructAuditDigest != plan.SourceConstructAuditDigest || amendment.SourceMutationProgramDigest != plan.SourceMutationProgramDigest ||
		amendment.ClaimBoundary != claimBoundaryV3 || amendment.EmpiricalStatus != EmpiricalStatusNotRun || amendment.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("v3 relation amendment identity, timing, claim, empirical, or authorization boundary is invalid")
	}
	if amendment.Pilot.Cases != 7 || amendment.Pilot.PrimaryLabels != 14 || amendment.Pilot.MaximumTieBreakLabels != 7 || amendment.Pilot.PostLabelProbes != 14 || amendment.Pilot.DataRole != string(study.RoleDevelopment) || amendment.Pilot.PrimaryOverlap != 0 || amendment.Pilot.PrimaryAnalysisUse != "forbidden" {
		return errors.New("v3 relation amendment pilot design is invalid")
	}
	if amendment.Primary.Cases != 28 || amendment.Primary.EffectiveTaskGroups != 28 || amendment.Primary.PrimaryLabels != 56 || amendment.Primary.MaximumTieBreakLabels != 28 || amendment.Primary.PostLabelProbes != 56 ||
		amendment.Primary.ClusterUnit != "source_task_group" || amendment.Primary.AggregationRule != primaryAggregationRule || amendment.Primary.ReplacementRule != primaryReplacementRuleV3 || amendment.Primary.MissingnessRule != primaryMissingnessRule || amendment.Primary.StoppingRule != primaryStoppingRuleV3 {
		return errors.New("v3 relation amendment primary design is invalid")
	}
	if amendment.ScarcitySentinel.Cases != 3 || amendment.ScarcitySentinel.TestCases != 0 || amendment.ScarcitySentinel.PrimaryOverlap != 0 || !amendment.ScarcitySentinel.Exhaustive || amendment.ScarcitySentinel.PrimaryAnalysisUse != "forbidden" || amendment.ScarcitySentinel.HeldOutClaimAvailable || !slices.Equal(amendment.ScarcitySentinel.SplitCounts, sentinel.SplitCounts) {
		return errors.New("v3 relation amendment scarcity-sentinel design is invalid")
	}
	inference := amendment.Inference
	if inference.PrimaryEstimand != primaryEstimand || inference.NominalAlpha != 0.05 || inference.IntervalMethod != intervalMethod || inference.MultiplicityMethod != multiplicityMethod ||
		!slices.Equal(inference.PrimaryMultiplicityFamily, []string{"cluster_contradiction_prevalence"}) || inference.FamilyAnalysisRole != familyAnalysisRole ||
		inference.UnresolvedDenominatorRule != unresolvedDenominatorRule || inference.IndependenceLimitation != independenceLimitationV3 ||
		inference.ZeroContradictionUpperBound != zeroContradictionUpperBound(28, 0.05) || len(inference.DetectionScenarios) != 3 {
		return errors.New("v3 relation amendment inference design is invalid")
	}
	expectedRates := []float64{0.05, 0.10, 0.20}
	for index, scenario := range inference.DetectionScenarios {
		if scenario.ContradictionRate != expectedRates[index] || scenario.DetectionProbability != detectionProbability(28, scenario.ContradictionRate) {
			return errors.New("v3 relation amendment detection scenario does not reproduce")
		}
	}
	expected, err := studyAmendmentV3Digest(amendment)
	if err != nil || amendment.Digest != expected {
		return errors.New("v3 relation amendment digest is invalid")
	}
	return nil
}

func studyAmendmentV3Digest(amendment StudyAmendmentV3) (string, error) {
	amendment.Digest = ""
	return digestJSON(amendment)
}
