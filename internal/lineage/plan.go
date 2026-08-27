package lineage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func DefaultPlan() (VerificationLineagePlan, error) {
	createdAt := time.Date(2026, 8, 10, 10, 30, 0, 0, time.UTC)
	lockedAt := time.Date(2026, 8, 10, 10, 58, 58, 0, time.UTC)
	plan := VerificationLineagePlan{
		Identity: PlanIdentity{
			PlanID: "task_069-verification-lineage-v1", TaskID: "TASK-069",
			Title: "Trace-native verification-lineage corpus", Authors: []string{"Christopher Schulze"},
			CreatedAt: createdAt, LockedAt: lockedAt,
		},
		ResearchQuestions: defaultResearchQuestions(), Unit: defaultUnitPlan(),
		SourceClasses: defaultSourceClasses(), Clusters: defaultClusterPlan(), Roles: defaultRolePlans(),
		Missingness: defaultMissingnessPlan(), Exclusions: defaultExclusions(), Replacement: defaultReplacementPlan(),
		Stopping: defaultStoppingPlan(), MinimumSupport: defaultMinimumSupportPlan(), Uncertainty: defaultUncertaintyPlan(),
		Holdouts: defaultHoldouts(), Claims: defaultClaimPlan(), Acquisition: defaultAcquisitionPlan(),
	}
	return SealPlan(plan)
}

func SealPlan(plan VerificationLineagePlan) (VerificationLineagePlan, error) {
	plan.SchemaVersion = PlanSchemaVersion
	plan.CanonicalPolicy = CanonicalPolicy
	plan.ProtocolVersion = ProtocolVersion
	plan.StudyGovernancePolicy = StudyGovernancePolicy
	plan.TraceMappingPolicy = preprocess.TraceMappingPolicyVersion
	plan.VerificationEvidenceContract = mutation.VerificationEvidenceAssessmentSchemaVersion
	plan.Digest = ""
	digest, err := digestJSON(plan)
	if err != nil {
		return VerificationLineagePlan{}, err
	}
	plan.Digest = digest
	return plan, plan.Validate()
}

func (plan VerificationLineagePlan) Validate() error {
	if err := validatePlanIdentity(plan); err != nil {
		return err
	}
	validators := []func() error{
		func() error { return validateResearchQuestions(plan.ResearchQuestions) },
		func() error { return validateUnitPlan(plan.Unit) },
		func() error { return validateSourceClasses(plan.SourceClasses) },
		func() error { return validateClusterPlan(plan.Clusters) },
		func() error { return validateRolePlans(plan.Roles) },
		func() error { return validateMissingnessPlan(plan.Missingness) },
		func() error { return validateExclusions(plan.Exclusions) },
		func() error { return validateReplacementPlan(plan.Replacement) },
		func() error { return validateStoppingPlan(plan.Stopping) },
		func() error { return validateMinimumSupport(plan.MinimumSupport, plan.Roles) },
		func() error { return validateUncertaintyPlan(plan.Uncertainty) },
		func() error { return validateHoldouts(plan.Holdouts) },
		func() error { return validateClaimPlan(plan.Claims) },
		func() error { return validateAcquisitionPlan(plan.Acquisition) },
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return validatePlanDigest(plan)
}

func validatePlanIdentity(plan VerificationLineagePlan) error {
	if plan.SchemaVersion != PlanSchemaVersion || plan.CanonicalPolicy != CanonicalPolicy ||
		plan.ProtocolVersion != ProtocolVersion || plan.StudyGovernancePolicy != StudyGovernancePolicy ||
		plan.TraceMappingPolicy != preprocess.TraceMappingPolicyVersion ||
		plan.VerificationEvidenceContract != mutation.VerificationEvidenceAssessmentSchemaVersion {
		return errors.New("verification-lineage plan protocol identity is invalid")
	}
	identity := plan.Identity
	if identity.PlanID == "" || identity.TaskID != "TASK-069" || identity.Title == "" || len(identity.Authors) == 0 ||
		identity.CreatedAt.IsZero() || identity.LockedAt.Before(identity.CreatedAt) {
		return errors.New("verification-lineage plan identity or lock interval is invalid")
	}
	return validateUniqueStrings("plan authors", identity.Authors, false)
}

func validateResearchQuestions(questions []ResearchQuestion) error {
	if len(questions) != 6 {
		return errors.New("verification-lineage plan requires exactly RQ1 through RQ6")
	}
	for index, question := range questions {
		expectedID := fmt.Sprintf("RQ%d", index+1)
		if question.ID != expectedID || missing(question.Question, question.PrimaryObservable, question.ForbiddenInterpretation) {
			return fmt.Errorf("research question %d is incomplete or out of order", index+1)
		}
	}
	return nil
}

func validateUnitPlan(plan UnitPlan) error {
	expectedNested := []string{"call", "result_chunk", "retry", "wrapper_child"}
	if plan.PrimaryUnit != PrimaryUnit || plan.ClusterUnit != ClusterUnit || !slices.Equal(plan.NestedUnits, expectedNested) ||
		plan.RetryPolicy != "nest_retries_under_original_task_group" ||
		plan.DenominatorPolicy != "one_terminal_state_per_included_task_group" {
		return errors.New("verification-lineage unit or denominator policy is invalid")
	}
	return nil
}

func validateSourceClasses(classes []SourceClassPlan) error {
	if len(classes) < 3 {
		return errors.New("verification-lineage plan requires at least three source classes")
	}
	previous := ""
	for _, class := range classes {
		if missing(class.ID, class.Basis, class.PreviouslyInspectedUse) || class.ID <= previous || !class.SourceManifestRequired || len(class.PermittedRoles) == 0 {
			return errors.New("source classes must be complete, unique, and sorted")
		}
		if err := validateRoles(class.PermittedRoles, true); err != nil {
			return fmt.Errorf("source class %q: %w", class.ID, err)
		}
		if class.LiveProviderUsePermitted {
			return fmt.Errorf("source class %q improperly authorizes provider use", class.ID)
		}
		previous = class.ID
	}
	return nil
}

func validateClusterPlan(plan ClusterPlan) error {
	expectedDimensions := []string{"lineage_id", "near_duplicate_id", "repository_id", "source_session_id", "task_id"}
	if !slices.Equal(plan.Dimensions, expectedDimensions) || !slices.Equal(plan.IsolatedRoles, canonicalRoles()) ||
		plan.CrossRolePolicy != "forbid_any_shared_cluster_dimension" ||
		plan.PairedViewPolicy != "paired_format_views_share_one_role_and_task_group" ||
		plan.NearDuplicateRule != "normalized_task_and_repository_similarity_declared_before_role_assignment" {
		return errors.New("verification-lineage cluster isolation contract is invalid")
	}
	return nil
}

func validateRolePlans(plans []RolePlan) error {
	expected := []RolePlan{
		{Role: RoleAdapterDevelopment, MinimumEligibleTaskGroups: 0, ParserChangesPermitted: true, ConfirmatoryClaims: false},
		{Role: RoleCaptureCalibration, MinimumEligibleTaskGroups: 20, ParserChangesPermitted: false, ConfirmatoryClaims: false},
		{Role: RoleLockedTest, MinimumEligibleTaskGroups: 20, ParserChangesPermitted: false, ConfirmatoryClaims: true},
		{Role: RoleAdversarialChallenge, MinimumEligibleTaskGroups: 2, ParserChangesPermitted: false, ConfirmatoryClaims: false},
	}
	if len(plans) != len(expected) {
		return errors.New("verification-lineage plan requires four isolated data roles")
	}
	for index, plan := range plans {
		want := expected[index]
		if plan.Role != want.Role || missing(plan.Purpose) || plan.MinimumEligibleTaskGroups != want.MinimumEligibleTaskGroups ||
			plan.ParserChangesPermitted != want.ParserChangesPermitted || plan.ConfirmatoryClaims != want.ConfirmatoryClaims {
			return fmt.Errorf("data role %d violates its locked use", index)
		}
	}
	return nil
}

func validateMissingnessPlan(plan MissingnessPlan) error {
	if plan.ClassificationRule != "assign_first_proven_terminal_state_by_precedence" ||
		plan.ConservationRule != "included_equals_eligible_plus_all_terminal_losses_and_ineligible_states" {
		return errors.New("missingness classification or conservation rule is invalid")
	}
	expected := terminalStateContract()
	if len(plan.TerminalStates) != len(expected) {
		return errors.New("missingness plan has an incomplete terminal-state surface")
	}
	for index, rule := range plan.TerminalStates {
		want := expected[index]
		if rule.State != want.State || rule.Precedence != index+1 || rule.Disposition != want.Disposition || missing(rule.ProofRequirement) {
			return fmt.Errorf("terminal state %d violates the exclusive precedence contract", index+1)
		}
	}
	return nil
}

func validateExclusions(rules []ExclusionRule) error {
	if len(rules) < 8 {
		return errors.New("verification-lineage exclusion surface is incomplete")
	}
	previous := ""
	for _, rule := range rules {
		if missing(rule.ID, rule.Stage, rule.Rule, rule.Treatment) || rule.ID <= previous || rule.Treatment != "exclude_before_primary_denominator" {
			return errors.New("exclusion rules must be complete, sorted, and pre-denominator")
		}
		previous = rule.ID
	}
	return nil
}

func validateReplacementPlan(plan ReplacementPlan) error {
	if plan.PostOutcomeReplacementPermitted || plan.PermittedStage != "before_content_or_outcome_inspection" ||
		plan.MatchingRule != "same_preregistered_source_class_format_and_role_stratum" ||
		plan.LineageRule != "new_source_identity_and_recorded_replacement_edge" ||
		plan.ShortfallRule != "retain_shortfall_when_no_prespecified_replacement_exists" {
		return errors.New("verification-lineage replacement policy is invalid")
	}
	return nil
}

func validateStoppingPlan(plan StoppingPlan) error {
	if plan.MaximumAdmittedTaskGroups != 240 || plan.MaximumSourceSessions != 120 || plan.MaximumAcquisitionDays != 30 ||
		plan.OutcomeDependentStopping ||
		plan.StopRule != "stop_at_first_hard_limit_or_exhausted_preregistered_inventory_never_on_observed_result" {
		return errors.New("verification-lineage stopping policy is invalid")
	}
	return nil
}

func validateMinimumSupport(plan MinimumSupportPlan, roles []RolePlan) error {
	if plan.NativeTraceFormats != 3 || plan.IndependentAgentEcosystems != 2 || plan.TaskGroupsPerInferentialFormat != 20 ||
		plan.CalibrationTaskGroups != 20 || plan.TestTaskGroups != 20 || !plan.PairedWitnessRequired {
		return errors.New("verification-lineage minimum support contract is invalid")
	}
	if len(roles) != 4 || roles[1].MinimumEligibleTaskGroups != plan.CalibrationTaskGroups || roles[2].MinimumEligibleTaskGroups != plan.TestTaskGroups {
		return errors.New("role and minimum-support task counts disagree")
	}
	return nil
}

func validateUncertaintyPlan(plan UncertaintyPlan) error {
	expectedEstimands := []string{"adjacent_layer_survival_proportion_by_format", "terminal_loss_state_proportion_by_format"}
	if plan.ConfidenceLevel != 0.95 || plan.ProportionInterval != "clopper_pearson_exact_task_cluster" ||
		plan.PairedDifferenceInterval != "task_cluster_percentile_bootstrap" || plan.BootstrapReplicates != 10000 ||
		plan.BootstrapSeed != 20260810 || plan.MultiplicityPolicy != "none_descriptive_primary_estimands" ||
		!slices.Equal(plan.PrimaryEstimands, expectedEstimands) || !plan.RawEventCountsSecondary {
		return errors.New("verification-lineage uncertainty contract is invalid")
	}
	return nil
}

func validateHoldouts(plans []HoldoutPlan) error {
	if len(plans) != 2 || plans[0].ID != "format-holdout-v1" || plans[0].Kind != HoldoutFormat ||
		plans[1].ID != "syntax-family-holdout-v1" || plans[1].Kind != HoldoutSyntaxFamily {
		return errors.New("verification-lineage plan requires format and syntax-family holdouts")
	}
	for _, plan := range plans {
		if missing(plan.SelectionSeed, plan.SelectionRule, plan.FrozenSurface, plan.PostResultRecovery) || plan.MinimumHeldOutUnits < 1 ||
			plan.PostResultRecovery != "forbidden_results_remain_immutable_recovery_requires_protocol_v2" {
			return fmt.Errorf("holdout %q is incomplete", plan.ID)
		}
	}
	return nil
}

func validateClaimPlan(plan ClaimPlan) error {
	if plan.CompositeScoreAllowed || plan.ProviderRankingAllowed || len(plan.Allowed) != 6 {
		return errors.New("verification-lineage claim ceiling permits a score, ranking, or incomplete surface")
	}
	previous := ""
	for _, claim := range plan.Allowed {
		if missing(claim.ID, claim.MaximumScope, claim.RequiredEvidence) || claim.ID <= previous {
			return errors.New("allowed lineage claims must be complete, unique, and sorted")
		}
		previous = claim.ID
	}
	expectedForbidden := []string{
		"agent_did_not_verify_from_missing_trace", "causal_performance_impact", "format_is_lossless", "hidden_reasoning",
		"human_validated", "provider_quality", "universal_verification_frequency",
	}
	if !slices.Equal(plan.Forbidden, expectedForbidden) {
		return errors.New("verification-lineage forbidden-claim surface is incomplete")
	}
	return nil
}

func validateAcquisitionPlan(plan AcquisitionPlan) error {
	expectedRequirements := []string{
		"adapter_and_parser_freeze", "capability_vectors_sealed", "golden_vectors_sealed",
		"source_manifests_sealed", "ten_schema_inventory_sealed",
	}
	if plan.State != AcquisitionNotStarted || plan.CountInspectionState != CountsNotInspectedBeforePlanLock ||
		plan.ExternalActionStatus != ExternalActionNotAuthorized || plan.ProviderCallsAllowed != 0 || plan.LaboratoryMayLaunchAgents ||
		!plan.OwnerAuthorizationRequired || !slices.Equal(plan.PreAcquisitionRequirements, expectedRequirements) ||
		plan.LiveCaptureCommandStatus != "undeclared_until_explicit_owner_authorization" {
		return errors.New("verification-lineage acquisition boundary is invalid")
	}
	return nil
}

func validatePlanDigest(plan VerificationLineagePlan) error {
	if !validDigest(plan.Digest) {
		return errors.New("verification-lineage plan digest must be lowercase SHA-256")
	}
	expected, err := planDigest(plan)
	if err != nil {
		return err
	}
	if plan.Digest != expected {
		return errors.New("verification-lineage plan digest is invalid")
	}
	if plan.Digest != LockedPlanDigest {
		return errors.New("verification-lineage plan differs from the locked preregistration")
	}
	return nil
}

func validateRoles(roles []DataRole, sorted bool) error {
	seen := make(map[DataRole]struct{}, len(roles))
	for _, role := range roles {
		if !slices.Contains(canonicalRoles(), role) {
			return fmt.Errorf("unsupported data role %q", role)
		}
		if _, duplicate := seen[role]; duplicate {
			return fmt.Errorf("duplicate data role %q", role)
		}
		seen[role] = struct{}{}
	}
	if sorted && !slices.IsSorted(roles) {
		return errors.New("data roles must be canonically sorted")
	}
	return nil
}

func validateUniqueStrings(name string, values []string, sorted bool) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s contains duplicate %q", name, value)
		}
		seen[value] = struct{}{}
	}
	if sorted && !slices.IsSorted(values) {
		return fmt.Errorf("%s must be sorted", name)
	}
	return nil
}

func planDigest(plan VerificationLineagePlan) (string, error) {
	plan.Digest = ""
	return digestJSON(plan)
}

func digestJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func missing(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}
