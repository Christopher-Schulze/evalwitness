package study

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion || m.CanonicalPolicy != CanonicalPolicy {
		return errors.New("study manifest schema or canonical policy is unsupported")
	}
	if err := validateIdentity(m.Identity); err != nil {
		return err
	}
	if err := validateHypotheses(m.Hypotheses); err != nil {
		return err
	}
	if m.Data.PrimaryUnit != "task" {
		return errors.New("study primary unit must be task")
	}
	if len(m.Data.Datasets) == 0 {
		return errors.New("study requires at least one dataset")
	}
	datasetIDs := make(map[string]struct{}, len(m.Data.Datasets))
	for index, dataset := range m.Data.Datasets {
		if err := validateDataset(dataset); err != nil {
			return fmt.Errorf("dataset %d: %w", index, err)
		}
		if _, duplicate := datasetIDs[dataset.ID]; duplicate {
			return fmt.Errorf("duplicate dataset ID %q", dataset.ID)
		}
		datasetIDs[dataset.ID] = struct{}{}
	}
	if err := m.Data.Split.Validate(); err != nil {
		return fmt.Errorf("split manifest: %w", err)
	}
	if err := validateDataPlan(m.Data, m.Inference.DecidableTasks); err != nil {
		return err
	}
	if err := validateArms(m.Arms, m.Execution.DeclaredRouteIDs); err != nil {
		return err
	}
	if err := validateOutcomes(m.Outcomes); err != nil {
		return err
	}
	if err := validateInference(m.Inference, m.Outcomes); err != nil {
		return err
	}
	if err := validateFailures(m.Failures); err != nil {
		return err
	}
	if err := validateControls(m.Controls); err != nil {
		return err
	}
	if err := validateProviders(m.Providers, m.Arms, m.Identity.LockedAt); err != nil {
		return err
	}
	if err := validateBudget(m.Budget); err != nil {
		return err
	}
	if err := validateExecution(m.Execution); err != nil {
		return err
	}
	if err := validatePublication(m.Publication, m.Identity, m.Outcomes); err != nil {
		return err
	}
	if err := validateReliability(m.Reliability, m.Identity.Kind); err != nil {
		return err
	}
	if err := validateKindSpecific(m); err != nil {
		return err
	}
	return validateAdjudication(m.Adjudication)
}

func validateIdentity(identity Identity) error {
	if missing(identity.Title, identity.ResearchQuestion) || len(identity.Authors) == 0 {
		return errors.New("study title, research question, and authors are required")
	}
	if !validKind(identity.Kind) {
		return fmt.Errorf("unsupported study kind %q", identity.Kind)
	}
	if identity.CreatedAt.IsZero() || identity.LockedAt.IsZero() || identity.LockedAt.Before(identity.CreatedAt) {
		return errors.New("created_at and non-earlier locked_at are required")
	}
	return validateUniqueText("authors", identity.Authors)
}

func validateHypotheses(h Hypotheses) error {
	if missing(h.PrimaryNull, h.PrimaryAlternative) {
		return errors.New("primary null and alternative hypotheses are required")
	}
	if err := validateUniqueText("secondary hypotheses", h.Secondary); err != nil {
		return err
	}
	return validateUniqueText("exploratory registry", h.Exploratory)
}

func validateDataset(dataset DatasetManifest) error {
	if missing(dataset.ID, dataset.Source, dataset.Version, dataset.License) || dataset.AcquiredAt.IsZero() {
		return errors.New("ID, source, version, license, and acquisition time are required")
	}
	if dataset.TaskCount < 1 || len(dataset.PermittedRoles) == 0 {
		return errors.New("positive task count and at least one permitted data role are required")
	}
	seenRoles := make(map[DataRole]struct{}, len(dataset.PermittedRoles))
	for _, role := range dataset.PermittedRoles {
		if !validRole(role) || role == RoleUnavailable {
			return fmt.Errorf("dataset has invalid permitted role %q", role)
		}
		if _, duplicate := seenRoles[role]; duplicate {
			return fmt.Errorf("dataset has duplicate permitted role %q", role)
		}
		seenRoles[role] = struct{}{}
	}
	for _, field := range []struct{ name, digest string }{
		{name: "dataset", digest: dataset.DatasetDigest}, {name: "task IDs", digest: dataset.TaskIDsDigest},
		{name: "outcome labels", digest: dataset.OutcomeLabelsDigest}, {name: "trajectory set", digest: dataset.TrajectorySetDigest},
	} {
		if !validDigest(field.digest) {
			return fmt.Errorf("%s digest must be SHA-256", field.name)
		}
	}
	if dataset.PreviouslyAccessed && (slices.Contains(dataset.PermittedRoles, RoleTest) || slices.Contains(dataset.PermittedRoles, RoleExternalReplication)) {
		return errors.New("previously accessed data cannot permit confirmatory test or external-replication roles")
	}
	exclusionIDs := make(map[string]struct{}, len(dataset.Exclusions))
	for index, exclusion := range dataset.Exclusions {
		if missing(exclusion.ID, exclusion.Rule, exclusion.Stage, exclusion.Treatment) {
			return fmt.Errorf("exclusion %d is incomplete", index)
		}
		if _, duplicate := exclusionIDs[exclusion.ID]; duplicate {
			return fmt.Errorf("duplicate exclusion ID %q", exclusion.ID)
		}
		exclusionIDs[exclusion.ID] = struct{}{}
	}
	return nil
}

func validateDataPlan(plan DataPlan, decidableTasks int) error {
	type datasetUse struct {
		tasks             int
		roles             map[DataRole]struct{}
		taskIDs           []string
		trajectoryDigests []string
	}
	uses := make(map[string]*datasetUse, len(plan.Datasets))
	taskCount := 0
	for _, assignment := range plan.Split.Assignments {
		taskCount += len(assignment.TaskIDs)
		use := uses[assignment.DatasetID]
		if use == nil {
			use = &datasetUse{roles: make(map[DataRole]struct{})}
			uses[assignment.DatasetID] = use
		}
		use.tasks += len(assignment.TaskIDs)
		use.roles[assignment.Split] = struct{}{}
		use.taskIDs = append(use.taskIDs, assignment.TaskIDs...)
		use.trajectoryDigests = append(use.trajectoryDigests, assignment.TrajectoryDigests...)
	}
	for _, dataset := range plan.Datasets {
		use := uses[dataset.ID]
		if use == nil {
			return fmt.Errorf("dataset %q has no split assignments", dataset.ID)
		}
		if use.tasks != dataset.TaskCount {
			return fmt.Errorf("dataset %q split assigns %d tasks but manifest declares %d", dataset.ID, use.tasks, dataset.TaskCount)
		}
		if canonicalStringSetDigest(use.taskIDs) != dataset.TaskIDsDigest {
			return fmt.Errorf("dataset %q task ID digest differs from its split assignments", dataset.ID)
		}
		if canonicalStringSetDigest(use.trajectoryDigests) != dataset.TrajectorySetDigest {
			return fmt.Errorf("dataset %q trajectory-set digest differs from its split assignments", dataset.ID)
		}
		permittedRoles := make(map[DataRole]struct{}, len(dataset.PermittedRoles))
		for _, role := range dataset.PermittedRoles {
			permittedRoles[role] = struct{}{}
		}
		for role := range use.roles {
			if _, allowed := permittedRoles[role]; !allowed {
				return fmt.Errorf("dataset %q split uses unpermitted role %q", dataset.ID, role)
			}
		}
		delete(uses, dataset.ID)
	}
	if len(uses) != 0 {
		return errors.New("split references an undeclared dataset")
	}
	if decidableTasks > taskCount {
		return fmt.Errorf("decidable task count %d exceeds the %d locked tasks", decidableTasks, taskCount)
	}
	return nil
}

func canonicalStringSetDigest(values []string) string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	digest := sha256.Sum256(canonicalIdentityBytes(values...))
	return hex.EncodeToString(digest[:])
}

func validateArms(arms []Arm, declaredRoutes []string) error {
	if len(arms) < 1 {
		return errors.New("at least one study arm is required")
	}
	seen := make(map[string]struct{}, len(arms))
	seenExecutionIdentity := make(map[string]struct{}, len(arms))
	for index, arm := range arms {
		if missing(arm.ID, arm.Entrypoint, arm.RouteID, arm.ProviderID, arm.RequestedModel, arm.ScorePolicyVersion, arm.SelectionMode) {
			return fmt.Errorf("arm %d has incomplete identity or policy", index)
		}
		if _, duplicate := seen[arm.ID]; duplicate {
			return fmt.Errorf("duplicate arm ID %q", arm.ID)
		}
		seen[arm.ID] = struct{}{}
		executionIdentity := arm.Entrypoint + "\x00" + arm.RouteID + "\x00" + arm.RequestContractDigest
		if _, duplicate := seenExecutionIdentity[executionIdentity]; duplicate {
			return fmt.Errorf("arm %q duplicates an entrypoint, route, and request contract", arm.ID)
		}
		seenExecutionIdentity[executionIdentity] = struct{}{}
		for _, field := range []struct{ name, digest string }{
			{name: "prompt", digest: arm.PromptDigest}, {name: "request contract", digest: arm.RequestContractDigest},
			{name: "calibration", digest: arm.CalibrationDigest}, {name: "attestation", digest: arm.AttestationDigest},
		} {
			if !validDigest(field.digest) {
				return fmt.Errorf("arm %q %s digest must be SHA-256", arm.ID, field.name)
			}
		}
		if arm.Candidates < 1 || arm.Repetitions < 1 {
			return fmt.Errorf("arm %q candidates and repetitions must be positive", arm.ID)
		}
		if !slices.Contains(declaredRoutes, arm.RouteID) {
			return fmt.Errorf("arm %q route %q is not declared for execution", arm.ID, arm.RouteID)
		}
	}
	return validateUniqueText("declared routes", declaredRoutes)
}

func validateOutcomes(outcomes OutcomePlan) error {
	if err := validateEndpoint(outcomes.Primary); err != nil {
		return fmt.Errorf("primary outcome: %w", err)
	}
	seen := map[string]struct{}{outcomes.Primary.ID: {}}
	families := []struct {
		name      string
		endpoints []Endpoint
	}{
		{name: "secondary", endpoints: outcomes.Secondary},
		{name: "exploratory", endpoints: outcomes.Exploratory},
	}
	for _, family := range families {
		for index, endpoint := range family.endpoints {
			if err := validateEndpoint(endpoint); err != nil {
				return fmt.Errorf("%s outcome %d: %w", family.name, index, err)
			}
			if _, duplicate := seen[endpoint.ID]; duplicate {
				return fmt.Errorf("outcome ID %q appears in more than one family", endpoint.ID)
			}
			seen[endpoint.ID] = struct{}{}
		}
	}
	return nil
}

func validateEndpoint(endpoint Endpoint) error {
	if missing(endpoint.ID, endpoint.Metric, endpoint.Direction, endpoint.Question, endpoint.FailureDenominator) {
		return errors.New("ID, metric, direction, question, and failure denominator are required")
	}
	question := stats.InferenceQuestion(endpoint.Question)
	if question != stats.QuestionSuperiority && question != stats.QuestionNonInferiority && question != stats.QuestionEquivalence {
		return fmt.Errorf("unsupported inference question %q", endpoint.Question)
	}
	if (question == stats.QuestionNonInferiority || question == stats.QuestionEquivalence) && !(endpoint.Margin > 0 && endpoint.Margin < 1) {
		return errors.New("non-inferiority and equivalence require a margin in (0,1)")
	}
	if question == stats.QuestionSuperiority && endpoint.Margin != 0 {
		return errors.New("superiority outcomes must not declare an equivalence or non-inferiority margin")
	}
	if question == stats.QuestionEquivalence && endpoint.Direction != "two_sided" {
		return errors.New("equivalence outcomes must use two_sided direction")
	}
	if question == stats.QuestionNonInferiority && endpoint.Direction == "two_sided" {
		return errors.New("non-inferiority outcomes must declare higher or lower direction")
	}
	if !finite(endpoint.Margin, endpoint.RiskTarget, endpoint.MinimumCoverage) || endpoint.RiskTarget < 0 || endpoint.RiskTarget > 1 || endpoint.MinimumCoverage < 0 || endpoint.MinimumCoverage > 1 {
		return errors.New("risk target and minimum coverage must be in [0,1]")
	}
	if endpoint.Direction != "higher" && endpoint.Direction != "lower" && endpoint.Direction != "two_sided" {
		return errors.New("outcome direction must be higher, lower, or two_sided")
	}
	return nil
}

func validateInference(plan InferencePlan, outcomes OutcomePlan) error {
	if missing(plan.Test, plan.IntervalMethod, plan.DesignMethod, plan.ClusterUnit, plan.MultiplicityMethod) || plan.ClusterUnit != "task" {
		return errors.New("test, interval, task cluster unit, and multiplicity method are required")
	}
	if !validDigest(plan.DesignEvidenceDigest) {
		return errors.New("design evidence digest must be SHA-256")
	}
	if !finite(plan.NominalAlpha, plan.TargetPower, plan.MinimumEffect, plan.DisagreementRate, plan.DiscordantWinProbability, plan.PowerAtMinimumEffect) ||
		!(plan.NominalAlpha > 0 && plan.NominalAlpha < 0.5) || !(plan.TargetPower > 0 && plan.TargetPower < 1) {
		return errors.New("alpha must be in (0,0.5) and target power in (0,1)")
	}
	if plan.MultiplicityMethod != "bonferroni" {
		return errors.New("version 1 manifests support only the implemented Bonferroni correction")
	}
	if plan.DecidableTasks < 1 || len(plan.PrimaryFamily) < 1 || !slices.Contains(plan.PrimaryFamily, outcomes.Primary.ID) {
		return errors.New("positive decidable task count and a primary family containing the primary outcome are required")
	}
	if err := validateUniqueText("primary multiplicity family", plan.PrimaryFamily); err != nil {
		return err
	}
	confirmatory := make(map[string]struct{}, 1+len(outcomes.Secondary))
	confirmatory[outcomes.Primary.ID] = struct{}{}
	for _, endpoint := range outcomes.Secondary {
		confirmatory[endpoint.ID] = struct{}{}
	}
	familyMembers := make(map[string]struct{}, len(plan.PrimaryFamily))
	for _, id := range plan.PrimaryFamily {
		if _, exists := confirmatory[id]; !exists {
			return fmt.Errorf("primary family member %q is not a confirmatory outcome", id)
		}
		familyMembers[id] = struct{}{}
	}
	for familyIndex, family := range plan.SecondaryFamilies {
		if len(family) == 0 {
			return fmt.Errorf("secondary multiplicity family %d is empty", familyIndex)
		}
		if err := validateUniqueText(fmt.Sprintf("secondary multiplicity family %d", familyIndex), family); err != nil {
			return err
		}
		for _, id := range family {
			if id == outcomes.Primary.ID {
				return errors.New("primary outcome cannot appear in a secondary family")
			}
			if _, exists := confirmatory[id]; !exists {
				return fmt.Errorf("secondary family member %q is not a confirmatory outcome", id)
			}
			if _, duplicate := familyMembers[id]; duplicate {
				return fmt.Errorf("confirmatory outcome %q appears in more than one multiplicity family", id)
			}
			familyMembers[id] = struct{}{}
		}
	}
	for _, endpoint := range outcomes.Secondary {
		if _, registered := familyMembers[endpoint.ID]; !registered {
			return fmt.Errorf("secondary outcome %q has no multiplicity family", endpoint.ID)
		}
	}
	for _, endpoint := range outcomes.Exploratory {
		if slices.Contains(plan.PrimaryFamily, endpoint.ID) {
			return fmt.Errorf("exploratory outcome %q cannot be promoted into the primary family", endpoint.ID)
		}
	}
	adjustedAlpha := plan.NominalAlpha / float64(len(plan.PrimaryFamily))
	if _, err := stats.PairedInferenceConfidence(stats.InferenceQuestion(outcomes.Primary.Question), adjustedAlpha); err != nil {
		return fmt.Errorf("primary inference: %w", err)
	}
	if !(plan.DisagreementRate >= 0 && plan.DisagreementRate <= 1) || !(plan.DiscordantWinProbability > 0.5 && plan.DiscordantWinProbability <= 1) {
		return errors.New("disagreement rate must be in [0,1] and discordant win probability in (0.5,1]")
	}
	if plan.MinimumEffect <= 0 || plan.MinimumEffect > plan.DisagreementRate {
		return errors.New("minimum effect must be positive and no greater than the disagreement rate")
	}
	question := stats.InferenceQuestion(outcomes.Primary.Question)
	switch question {
	case stats.QuestionSuperiority:
		if plan.DesignMethod != "exact_mcnemar_unconditional" {
			return errors.New("superiority requires exact_mcnemar_unconditional design power")
		}
		q := (1 + plan.MinimumEffect/plan.DisagreementRate) / 2
		power, err := stats.ExactMcNemarUnconditionalPower(plan.DecidableTasks, plan.DisagreementRate, q, adjustedAlpha)
		if err != nil {
			return err
		}
		if math.Abs(power-plan.PowerAtMinimumEffect) > 1e-12 {
			return fmt.Errorf("declared design power %.12f differs from exact power %.12f", plan.PowerAtMinimumEffect, power)
		}
	case stats.QuestionNonInferiority, stats.QuestionEquivalence:
		if plan.DesignMethod != "paired_joint_outcome_simulation" {
			return errors.New("non-inferiority and equivalence require paired_joint_outcome_simulation design evidence")
		}
	}
	if plan.PowerAtMinimumEffect+1e-12 < plan.TargetPower || plan.PowerAtMinimumEffect > 1 {
		return fmt.Errorf("locked design power %.6f is outside the target range [%.6f,1]", plan.PowerAtMinimumEffect, plan.TargetPower)
	}
	if plan.Sequential.Enabled {
		if missing(plan.Sequential.Method, plan.Sequential.BoundaryDigest) || !validDigest(plan.Sequential.BoundaryDigest) || plan.Sequential.MaximumLooks < 2 {
			return errors.New("sequential stopping requires method, boundary digest, and at least two looks")
		}
	} else if plan.Sequential.Method != "fixed_sample" || plan.Sequential.BoundaryDigest != "" || plan.Sequential.MaximumLooks != 1 {
		return errors.New("non-sequential studies must declare fixed_sample with one look and no boundary digest")
	}
	return nil
}

func validateFailures(plan FailurePlan) error {
	if missing(plan.MissingScore, plan.ProviderFailure, plan.RouteFailure, plan.Timeout, plan.Abstention,
		plan.BudgetExhaustion, plan.RetryExhaustion, plan.IncompleteCell, plan.DenominatorPolicy) {
		return errors.New("every missingness, failure, abstention, exhaustion, incomplete-cell, and denominator treatment must be prespecified")
	}
	return nil
}

func validateControls(plan ControlPlan) error {
	if missing(plan.RandomSelectionID, plan.TaskIndependentSelector, plan.PositiveControl) {
		return errors.New("random, task-independent, and positive controls are required")
	}
	if plan.PositiveControlSource != RoleDevelopment && plan.PositiveControlSource != RoleCalibration {
		return errors.New("positive control source must be development or calibration")
	}
	return nil
}

func validateProviders(plans []ProviderPlan, arms []Arm, lockedAt time.Time) error {
	if len(plans) != len(arms) {
		return errors.New("provider plans require exactly one entry per study arm")
	}
	armIDs := make(map[string]struct{}, len(arms))
	for _, arm := range arms {
		armIDs[arm.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(plans))
	for index, plan := range plans {
		if _, exists := armIDs[plan.ArmID]; !exists {
			return fmt.Errorf("provider plan %d references unknown arm %q", index, plan.ArmID)
		}
		if _, duplicate := seen[plan.ArmID]; duplicate {
			return fmt.Errorf("duplicate provider plan for arm %q", plan.ArmID)
		}
		seen[plan.ArmID] = struct{}{}
		if err := validateProvider(plan, lockedAt); err != nil {
			return fmt.Errorf("provider plan for arm %q: %w", plan.ArmID, err)
		}
	}
	return nil
}

func validateProvider(plan ProviderPlan, lockedAt time.Time) error {
	if strings.TrimSpace(plan.ArmID) == "" {
		return errors.New("provider plan arm ID is required")
	}
	if plan.AttestationObservedAt.IsZero() || plan.AttestationExpiresAt.IsZero() || plan.AttestationObservedAt.After(lockedAt) ||
		!plan.AttestationExpiresAt.After(plan.AttestationObservedAt) || !plan.AttestationExpiresAt.After(lockedAt) {
		return errors.New("provider attestation must be observed no later than lock and remain fresh after lock")
	}
	if err := provider.ValidateServedIdentityPolicy(plan.ServedIdentityPolicy, plan.ExpectedServedModel, plan.ExpectedServedModels); err != nil {
		return fmt.Errorf("served identity policy: %w", err)
	}
	if plan.CheckpointAssertionPolicy != provider.ServedIdentityPolicyExactObserved {
		return errors.New("provider checkpoint assertion policy must be exact_observed")
	}
	if plan.ExpectedCheckpointAssertion == "" && plan.ExpectedCheckpointAssertionSource != "" {
		return errors.New("checkpoint assertion source requires an expected assertion")
	}
	if plan.ExpectedCheckpointAssertion != "" && plan.ExpectedCheckpointAssertionSource == "" {
		return errors.New("expected checkpoint assertion requires its source")
	}
	if !provider.SupportedRetryPolicyVersion(plan.RetryPolicyVersion) || plan.MaxRetries < 0 || plan.RequestTimeoutSeconds <= 0 || plan.MinDispatchIntervalSeconds < 0 {
		return errors.New("provider retry policy, maximum retries, request timeout, or dispatch interval is invalid")
	}
	return nil
}

func validateBudget(plan BudgetPlan) error {
	if plan.ExpectedCalls < 0 || plan.HardCalls <= 0 || plan.ExpectedCalls > plan.HardCalls || plan.HardAttempts < plan.HardCalls ||
		plan.HardInputTokens <= 0 || plan.HardOutputTokens <= 0 || plan.HardDurationSeconds <= 0 || plan.HardConcurrent <= 0 ||
		!finite(plan.HardCostUSD) || plan.HardCostUSD < 0 {
		return errors.New("study budget bounds are invalid")
	}
	return nil
}

func validateExecution(plan ExecutionPlan) error {
	if plan.Dirty {
		return errors.New("locked studies prohibit dirty builds")
	}
	if missing(plan.Commit, plan.Platform, plan.AnalysisVersion) || len(plan.AnalysisCommand) == 0 || len(plan.DeclaredInputPaths) == 0 {
		return errors.New("clean commit, platform, analysis command/version, and declared inputs are required")
	}
	for _, field := range []struct{ name, digest string }{{name: "binary", digest: plan.BinaryDigest}, {name: "analysis", digest: plan.AnalysisDigest}} {
		if !validDigest(field.digest) {
			return fmt.Errorf("%s digest must be SHA-256", field.name)
		}
	}
	if len(plan.DeclaredInputPaths) != len(plan.DeclaredInputDigests) {
		return errors.New("declared input paths and digests must have equal length")
	}
	if err := validateUniqueText("declared input paths", plan.DeclaredInputPaths); err != nil {
		return err
	}
	for index, path := range plan.DeclaredInputPaths {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
			return fmt.Errorf("declared input path %q must be a clean repository-relative path", path)
		}
		if !validDigest(plan.DeclaredInputDigests[index]) {
			return fmt.Errorf("declared input %q has an invalid digest", path)
		}
	}
	return nil
}

func validatePublication(plan PublicationPlan, identity Identity, outcomes OutcomePlan) error {
	if missing(plan.CapsuleVisibility) || len(plan.AllowedClaimIDs) == 0 || len(plan.RequiredCaveats) == 0 || plan.RegisteredReportTimestamp.IsZero() {
		return errors.New("publication visibility, allowed claims, caveats, and registered-report timestamp are required")
	}
	if err := validateUniqueText("allowed claims", plan.AllowedClaimIDs); err != nil {
		return err
	}
	if plan.RegisteredReportTimestamp.Before(identity.CreatedAt) || plan.RegisteredReportTimestamp.After(identity.LockedAt) {
		return errors.New("registered-report timestamp must be between manifest creation and lock")
	}
	confirmatory := map[string]struct{}{outcomes.Primary.ID: {}}
	for _, endpoint := range outcomes.Secondary {
		confirmatory[endpoint.ID] = struct{}{}
	}
	for _, claimID := range plan.AllowedClaimIDs {
		if _, registered := confirmatory[claimID]; !registered {
			return fmt.Errorf("allowed claim %q is not a registered confirmatory endpoint", claimID)
		}
	}
	return validateUniqueText("required caveats", plan.RequiredCaveats)
}

func validateReliability(contracts ReliabilityContracts, kind StudyKind) error {
	if contracts.ProtocolVersion != protocol.CurrentVersion || contracts.TraceMappingPolicy != preprocess.TraceMappingPolicyVersion {
		return errors.New("study must pin the current verifier-audit protocol and trace mapping policy")
	}
	for _, field := range []struct{ name, digest string }{
		{name: "protocol corpus", digest: contracts.ProtocolCorpusDigest},
		{name: "protocol request corpus", digest: contracts.ProtocolRequestCorpusDigest},
		{name: "protocol schema", digest: contracts.ProtocolSchemaDigest},
		{name: "outcome contract", digest: contracts.OutcomeContractDigest},
		{name: "adjudication contract", digest: contracts.AdjudicationContractDigest},
		{name: "profile projection", digest: contracts.ProfileProjectionDigest},
	} {
		if !validDigest(field.digest) {
			return fmt.Errorf("%s digest must be SHA-256", field.name)
		}
	}
	if kind == KindControlledRelation && (!validDigest(contracts.RelationCorpusDigest) || !validDigest(contracts.ValidatorContractDigest)) {
		return errors.New("controlled-relation study requires relation corpus and validator contract digests")
	}
	if kind == KindEvidenceReliance && (!validDigest(contracts.EvidenceFactorDigest) || !validDigest(contracts.InterventionContractDigest)) {
		return errors.New("evidence-reliance study requires factor and intervention contract digests")
	}
	return nil
}

func validateKindSpecific(manifest Manifest) error {
	if manifest.Identity.Kind != KindRealAgentCorpus && manifest.RealAgentCorpus != nil {
		return errors.New("real-agent corpus governance is only valid for a real-agent corpus study")
	}
	if manifest.Identity.Kind != KindControlledRelation && manifest.Relations != nil {
		return errors.New("controlled-relation governance is only valid for a controlled-relation study")
	}
	if manifest.Identity.Kind != KindEvidenceReliance && manifest.EvidenceReliance != nil {
		return errors.New("evidence-reliance governance is only valid for an evidence-reliance study")
	}
	switch manifest.Identity.Kind {
	case KindRealAgentCorpus:
		if manifest.RealAgentCorpus == nil {
			return errors.New("real-agent corpus study requires corpus governance")
		}
		plan := manifest.RealAgentCorpus
		if missing(plan.SourceBasis, plan.ConsentPolicy, plan.LicensePolicy, plan.PrivacyClass, plan.TaskLabelContract, plan.ReleaseabilityRule) ||
			!validDigest(plan.RedactionPolicyDigest) || len(plan.ContaminationChecks) == 0 || len(plan.FormatTargets) == 0 {
			return errors.New("real-agent corpus governance is incomplete")
		}
		if err := validateUniqueText("contamination checks", plan.ContaminationChecks); err != nil {
			return err
		}
		if err := validateUniqueText("format targets", plan.FormatTargets); err != nil {
			return err
		}
	case KindControlledRelation:
		if manifest.Relations == nil || len(manifest.Relations.MutationFamilies) == 0 || len(manifest.Relations.ExpectedRelations) == 0 ||
			len(manifest.Relations.ValidatorDigests) == 0 || missing(manifest.Relations.CorpusVersion, manifest.Relations.RelationContractVersion,
			manifest.Relations.AmbiguityPolicy, manifest.Relations.PrimaryDenominator, manifest.Relations.ClusterUnit,
			manifest.Relations.ReductionPolicy, manifest.Relations.ClaimType) {
			return errors.New("controlled-relation governance is incomplete")
		}
		plan := manifest.Relations
		if plan.ClusterUnit != "source_task" {
			return errors.New("controlled-relation cluster unit must be source_task")
		}
		if !slices.Contains([]string{"invariance", "sensitivity", "adversarial_evidence"}, plan.ClaimType) {
			return errors.New("controlled-relation claim type must be invariance, sensitivity, or adversarial_evidence")
		}
		if err := validateUniqueText("mutation families", plan.MutationFamilies); err != nil {
			return err
		}
		if err := validateUniqueText("expected relations", plan.ExpectedRelations); err != nil {
			return err
		}
		if err := validateUniqueText("validator digests", plan.ValidatorDigests); err != nil {
			return err
		}
		for _, digest := range plan.ValidatorDigests {
			if !validDigest(digest) {
				return errors.New("controlled-relation validator digest must be SHA-256")
			}
		}
	case KindEvidenceReliance:
		if manifest.EvidenceReliance == nil {
			return errors.New("evidence-reliance study requires intervention governance")
		}
		plan := manifest.EvidenceReliance
		if !validDigest(plan.FactorOntologyDigest) || len(plan.AllowedFieldPaths) == 0 || len(plan.InterventionOperators) == 0 ||
			len(plan.EvidenceOnlyFamilies) == 0 || len(plan.IdentificationAssumptions) == 0 || len(plan.MainEffects) == 0 ||
			len(plan.Estimators) == 0 || len(plan.MultiplicityFamily) == 0 || missing(plan.Randomization, plan.MultiplicityMethod,
			plan.InvalidCaseHandling, plan.ReductionPolicy, plan.RelianceWitness) {
			return errors.New("evidence-reliance governance is incomplete")
		}
		collections := []struct {
			name   string
			values []string
		}{
			{name: "allowed evidence field paths", values: plan.AllowedFieldPaths},
			{name: "intervention operators", values: plan.InterventionOperators},
			{name: "evidence-only families", values: plan.EvidenceOnlyFamilies},
			{name: "quality-changing families", values: plan.QualityChangingFamilies},
			{name: "identification assumptions", values: plan.IdentificationAssumptions},
			{name: "main effects", values: plan.MainEffects},
			{name: "interactions", values: plan.Interactions},
			{name: "estimators", values: plan.Estimators},
			{name: "evidence-reliance multiplicity family", values: plan.MultiplicityFamily},
		}
		for _, collection := range collections {
			if err := validateUniqueText(collection.name, collection.values); err != nil {
				return err
			}
		}
		declaredEffects := append(append([]string(nil), plan.MainEffects...), plan.Interactions...)
		if err := validateUniqueText("evidence-reliance declared effects", declaredEffects); err != nil {
			return err
		}
		if !sameStringSet(declaredEffects, plan.MultiplicityFamily) {
			return errors.New("evidence-reliance multiplicity family must contain every declared main effect and interaction exactly once")
		}
		qualityFamilies := make(map[string]struct{}, len(plan.QualityChangingFamilies))
		for _, family := range plan.QualityChangingFamilies {
			qualityFamilies[family] = struct{}{}
		}
		for _, family := range plan.EvidenceOnlyFamilies {
			if _, overlap := qualityFamilies[family]; overlap {
				return fmt.Errorf("intervention family %q cannot be both evidence-only and quality-changing", family)
			}
		}
	}
	return nil
}

func validateAdjudication(plan AdjudicationPlan) error {
	if len(plan.SampleStrata) == 0 || missing(plan.Blinding, plan.AgreementMetric, plan.ConflictResolution, plan.LabelRevision, plan.SensitivityAnalysis) {
		return errors.New("adjudication strata, blinding, agreement, conflict, revision, and sensitivity rules are required")
	}
	return validateUniqueText("adjudication strata", plan.SampleStrata)
}

func validKind(kind StudyKind) bool {
	switch kind {
	case KindBenchmark, KindCalibration, KindControlledRelation, KindEvidenceReliance, KindRealAgentCorpus, KindTransfer, KindDrift:
		return true
	default:
		return false
	}
}

func validRole(role DataRole) bool {
	switch role {
	case RoleDevelopment, RoleCalibration, RoleTest, RoleExternalReplication, RoleUnavailable:
		return true
	default:
		return false
	}
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func finite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func missing(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func validateUniqueText(name string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s contains duplicate %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
