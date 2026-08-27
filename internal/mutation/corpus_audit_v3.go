package mutation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

const CorpusDevelopmentAuditSchemaVersionV3 = "evalwitness.corruption-corpus-development-audit.v3"

// CorpusDevelopmentAuditV3 is intentionally separate from the v2 audit wire
// type. The typed construct proofs are not representable by the v2 schema, and
// keeping a distinct type prevents a decoder from silently accepting mixed
// firewall generations.
type CorpusDevelopmentAuditV3 struct {
	SchemaVersion         string                      `json:"schema_version"`
	CanonicalPolicy       string                      `json:"canonical_policy"`
	AuditedAt             string                      `json:"audited_at"`
	PlanDigest            string                      `json:"plan_digest"`
	CorpusVersion         string                      `json:"corpus_version"`
	MutationProgramDigest string                      `json:"mutation_program_digest"`
	SourceSetDigest       string                      `json:"source_set_digest"`
	AttemptSetDigest      string                      `json:"attempt_set_digest"`
	FirewallSetDigest     string                      `json:"firewall_set_digest"`
	SelectedCaseSetDigest string                      `json:"selected_case_set_digest"`
	Sources               []CorpusSource              `json:"sources"`
	Attempts              []ConstructAttempt          `json:"attempts"`
	AppliedFirewalls      []ConstructFirewallReportV2 `json:"applied_firewalls"`
	RejectedFirewalls     []ConstructFirewallReportV2 `json:"rejected_firewalls"`
	Coverage              []ConstructCoverage         `json:"coverage"`
	SelectedCaseIDs       []string                    `json:"selected_case_ids"`
	QuotaShortfalls       []CorpusCount               `json:"quota_shortfalls"`
	SourceTasks           int                         `json:"source_tasks"`
	TotalAttempts         int                         `json:"total_attempts"`
	AppliedAttempts       int                         `json:"applied_attempts"`
	RejectedAttempts      int                         `json:"rejected_attempts"`
	SelectedCases         int                         `json:"selected_cases"`
	QuotasSatisfied       bool                        `json:"quotas_satisfied"`
	Findings              []string                    `json:"findings"`
	Digest                string                      `json:"digest"`
}

func AuditCorpusV3(plan CorpusDevelopmentPlan, candidates []SourceCandidate, auditedAt string) (CorpusDevelopmentAuditV3, error) {
	if err := validateV3CorpusPlan(plan); err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	if _, err := time.Parse(time.DateOnly, auditedAt); err != nil {
		return CorpusDevelopmentAuditV3{}, errors.New("v3 corpus development audit requires YYYY-MM-DD audited_at")
	}
	selected, err := selectCorpusSources(plan.corpusSpecShape(), candidates)
	if err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	lineageSources := make([]CorpusSource, len(selected))
	for index := range selected {
		lineageSources[index] = selected[index].Source
	}
	lineagePlan := corpusLineagePlan(lineageSources, plan.Seed)
	for index := range selected {
		assignment := lineagePlan[selected[index].Source.ID]
		selected[index].Source.LineageClusterID = assignment.ClusterID
		selected[index].Source.Split = assignment.Split
	}

	attempts := make([]ConstructAttempt, 0)
	appliedFirewalls := make([]ConstructFirewallReportV2, 0)
	rejectedFirewalls := make([]ConstructFirewallReportV2, 0)
	for _, family := range plan.PrimaryFamilies {
		definition, _ := DefinitionFor(family)
		var familyAttempts []ConstructAttempt
		var familyApplied, familyRejected []ConstructFirewallReportV2
		if definition.PairLevel {
			familyAttempts, familyApplied, familyRejected, err = auditPairFamilyV3(plan, selected, family)
		} else {
			familyAttempts, familyApplied, familyRejected, err = auditTrajectoryFamilyV3(plan, selected, family)
		}
		if err != nil {
			return CorpusDevelopmentAuditV3{}, err
		}
		attempts = append(attempts, familyAttempts...)
		appliedFirewalls = append(appliedFirewalls, familyApplied...)
		rejectedFirewalls = append(rejectedFirewalls, familyRejected...)
	}

	sources := make([]CorpusSource, len(selected))
	for index := range selected {
		sources[index] = selected[index].Source
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].ID < sources[right].ID })
	sort.Slice(attempts, func(left, right int) bool { return attempts[left].ID < attempts[right].ID })
	sort.Slice(appliedFirewalls, func(left, right int) bool { return appliedFirewalls[left].Digest < appliedFirewalls[right].Digest })
	sort.Slice(rejectedFirewalls, func(left, right int) bool { return rejectedFirewalls[left].Digest < rejectedFirewalls[right].Digest })
	coverage := constructCoverageV3(attempts, appliedFirewalls, rejectedFirewalls)
	selectedCaseIDs := selectedAttemptMutationIDs(attempts)
	shortfalls := quotaShortfalls(plan, attempts)
	findings := constructAuditFindingsV3(shortfalls, len(attempts), len(rejectedFirewalls))

	sourceSetDigest, err := digestJSON(sources)
	if err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	attemptSetDigest, err := digestJSON(attempts)
	if err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	firewallSetDigest, err := digestJSON(struct {
		Applied  []ConstructFirewallReportV2 `json:"applied"`
		Rejected []ConstructFirewallReportV2 `json:"rejected"`
	}{Applied: appliedFirewalls, Rejected: rejectedFirewalls})
	if err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	selectedCaseSetDigest, err := digestJSON(selectedCaseIDs)
	if err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	audit := CorpusDevelopmentAuditV3{
		AuditedAt: auditedAt, PlanDigest: plan.Digest, CorpusVersion: plan.CorpusVersion,
		MutationProgramDigest: plan.MutationProgramDigest, SourceSetDigest: sourceSetDigest,
		AttemptSetDigest: attemptSetDigest, FirewallSetDigest: firewallSetDigest, SelectedCaseSetDigest: selectedCaseSetDigest,
		Sources: sources, Attempts: attempts, AppliedFirewalls: appliedFirewalls, RejectedFirewalls: rejectedFirewalls,
		Coverage: coverage, SelectedCaseIDs: selectedCaseIDs, QuotaShortfalls: shortfalls,
		SourceTasks: uniqueSourceTasks(sources), TotalAttempts: len(attempts), AppliedAttempts: len(appliedFirewalls),
		RejectedAttempts: len(rejectedFirewalls), SelectedCases: len(selectedCaseIDs), QuotasSatisfied: len(shortfalls) == 0,
		Findings: findings,
	}
	return SealCorpusDevelopmentAuditV3(plan, audit)
}

func auditTrajectoryFamilyV3(plan CorpusDevelopmentPlan, candidates []SourceCandidate, family Family) ([]ConstructAttempt, []ConstructFirewallReportV2, []ConstructFirewallReportV2, error) {
	ordered := append([]SourceCandidate(nil), candidates...)
	sort.Slice(ordered, func(left, right int) bool {
		return digestText(plan.Seed+"\x00"+string(family)+"\x00"+ordered[left].Source.ID) < digestText(plan.Seed+"\x00"+string(family)+"\x00"+ordered[right].Source.ID)
	})
	resolvedGroups := make(map[string]struct{})
	selectedCases := 0
	attempts := make([]ConstructAttempt, 0)
	applied := make([]ConstructFirewallReportV2, 0)
	rejected := make([]ConstructFirewallReportV2, 0)
	for _, candidate := range ordered {
		if _, resolved := resolvedGroups[candidate.Source.SplitGroupID]; resolved {
			continue
		}
		outcome, err := ApplyV3(candidate.Trajectory, corpusApplyRequestV3(plan, family, candidate.Source))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("audit v3 family %q source %q: %w", family, candidate.Source.ID, err)
		}
		attempt := ConstructAttempt{
			Family: family, SourceFormat: candidate.Source.SourceFormat, SourceIDs: []string{candidate.Source.ID},
			SplitGroupID: candidate.Source.SplitGroupID, Status: outcome.Status, FirewallDigest: outcome.Firewall.Digest,
		}
		switch outcome.Status {
		case ConstructApplied:
			if outcome.Applied == nil {
				return nil, nil, nil, fmt.Errorf("audit v3 family %q source %q produced no applied result", family, candidate.Source.ID)
			}
			if _, err := ReduceChangedRegions(outcome.Applied.Manifest, candidate.Trajectory, outcome.Applied.Mutated); err != nil {
				return nil, nil, nil, fmt.Errorf("audit v3 family %q source %q reduction: %w", family, candidate.Source.ID, err)
			}
			resolvedGroups[candidate.Source.SplitGroupID] = struct{}{}
			attempt.MutationID = outcome.Applied.Manifest.MutationID
			if selectedCases < plan.CasesPerFamily {
				attempt.Selected = true
				selectedCases++
			}
			applied = append(applied, outcome.Firewall)
		case ConstructRejected:
			rejected = append(rejected, outcome.Firewall)
		default:
			return nil, nil, nil, fmt.Errorf("audit v3 family %q source %q returned invalid status %q", family, candidate.Source.ID, outcome.Status)
		}
		attempt, err = sealConstructAttempt(attempt)
		if err != nil {
			return nil, nil, nil, err
		}
		attempts = append(attempts, attempt)
	}
	return attempts, applied, rejected, nil
}

func auditPairFamilyV3(plan CorpusDevelopmentPlan, candidates []SourceCandidate, family Family) ([]ConstructAttempt, []ConstructFirewallReportV2, []ConstructFirewallReportV2, error) {
	groups := make(map[string][]SourceCandidate)
	for _, candidate := range candidates {
		groups[candidate.Source.SplitGroupID] = append(groups[candidate.Source.SplitGroupID], candidate)
	}
	groupIDs := rankedGroups(plan.Seed+"\x00"+string(family), groups, func(values []SourceCandidate) bool { return len(values) >= 2 })
	attempts := make([]ConstructAttempt, 0, len(groupIDs))
	applied := make([]ConstructFirewallReportV2, 0, len(groupIDs))
	selectedCases := 0
	for _, groupID := range groupIDs {
		values := append([]SourceCandidate(nil), groups[groupID]...)
		sort.Slice(values, func(left, right int) bool {
			return digestText(plan.Seed+"\x00"+string(family)+"\x00"+values[left].Source.ID) < digestText(plan.Seed+"\x00"+string(family)+"\x00"+values[right].Source.ID)
		})
		left, right := values[0], values[1]
		request := corpusApplyRequestV3(plan, family, left.Source)
		request.SourceFamily = "paired/" + left.Source.SourceFamily + "+" + right.Source.SourceFamily
		request.SourceLocation = "pair/" + groupID + "/" + digestText(left.Source.ID+"\x00"+right.Source.ID)
		request.SourceRevision = left.Source.SourceRevision + "+" + right.Source.SourceRevision
		request.Outcome = SourceOutcome{
			Kind: "paired_benchmark_rewards", Value: left.Source.Outcome.Value + "," + right.Source.Outcome.Value,
			WitnessDigest: digestText(left.Source.Outcome.WitnessDigest + "\x00" + right.Source.Outcome.WitnessDigest),
		}
		outcome, err := ApplyCandidateOrderReversalV3(left.Trajectory, right.Trajectory, request)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("audit v3 pair family %q group %q: %w", family, groupID, err)
		}
		if outcome.Status != ConstructApplied || outcome.Applied == nil {
			return nil, nil, nil, fmt.Errorf("audit v3 pair family %q group %q was not applied", family, groupID)
		}
		attempt := ConstructAttempt{
			Family: family, SourceFormat: left.Source.SourceFormat, SourceIDs: []string{left.Source.ID, right.Source.ID},
			SplitGroupID: groupID, Status: ConstructApplied, FirewallDigest: outcome.Firewall.Digest,
			MutationID: outcome.Applied.Manifest.MutationID, Selected: selectedCases < plan.CasesPerFamily,
		}
		if attempt.Selected {
			selectedCases++
		}
		attempt, err = sealConstructAttempt(attempt)
		if err != nil {
			return nil, nil, nil, err
		}
		attempts = append(attempts, attempt)
		applied = append(applied, outcome.Firewall)
	}
	return attempts, applied, []ConstructFirewallReportV2{}, nil
}

func corpusApplyRequestV3(plan CorpusDevelopmentPlan, family Family, source CorpusSource) ApplyRequest {
	definition, _ := DefinitionFor(family)
	validatorKind := ValidationPreservation
	if definition.Class == ClassAdversarialClaim {
		validatorKind = ValidationFormal
	}
	reviewSampled := digestText(plan.Seed + "\x00review\x00" + source.ID + "\x00" + string(family))[:2] <= "1a"
	return ApplyRequest{
		CorpusVersion: plan.CorpusVersion, TaskID: source.TaskID, RepositoryID: source.RepositoryID, SourceFamily: source.SourceFamily,
		SourceLocation: source.SourceLocation, SourceRevision: source.SourceRevision, SplitGroupID: source.SplitGroupID,
		Seed: plan.Seed + "/" + string(family), Family: family, Outcome: source.Outcome,
		Validator: ValidatorSpec{
			ID: "evalwitness.controlled-relation", Version: RelationContractVersionV3, Kind: validatorKind,
			ContractDigest: plan.MutationProgramDigest, TimeoutMillis: 30_000, MaximumOutputBytes: 1024 * 1024,
		},
		License: source.License, Privacy: source.Privacy, ReviewSampled: reviewSampled,
		ReviewSamplingStratum: "automatic-" + corpusControl(family),
	}
}

func constructCoverageV3(attempts []ConstructAttempt, applied, rejected []ConstructFirewallReportV2) []ConstructCoverage {
	reports := make(map[string]ConstructFirewallReportV2, len(applied)+len(rejected))
	for _, report := range applied {
		reports[report.Digest] = report
	}
	for _, report := range rejected {
		reports[report.Digest] = report
	}
	type accumulator struct {
		coverage ConstructCoverage
		groups   map[string]struct{}
		reasons  map[string]int
	}
	byKey := make(map[string]*accumulator)
	for _, attempt := range attempts {
		key := string(attempt.Family) + "\x00" + string(attempt.SourceFormat)
		row := byKey[key]
		if row == nil {
			row = &accumulator{coverage: ConstructCoverage{Family: attempt.Family, SourceFormat: attempt.SourceFormat}, groups: make(map[string]struct{}), reasons: make(map[string]int)}
			byKey[key] = row
		}
		row.coverage.Attempted++
		switch attempt.Status {
		case ConstructApplied:
			row.coverage.Applied++
			row.groups[attempt.SplitGroupID] = struct{}{}
			if attempt.Selected {
				row.coverage.SelectedCases++
			}
		case ConstructRejected:
			row.coverage.Rejected++
			for _, reason := range reports[attempt.FirewallDigest].RejectionReasons {
				row.reasons[string(reason)]++
			}
		}
	}
	rows := make([]ConstructCoverage, 0, len(byKey))
	for _, row := range byKey {
		row.coverage.EligibleTaskGroups = len(row.groups)
		row.coverage.RejectionReasonCounts = countRecords(row.reasons)
		rows = append(rows, row.coverage)
	}
	sort.Slice(rows, func(left, right int) bool {
		leftKey := string(rows[left].Family) + "\x00" + string(rows[left].SourceFormat)
		rightKey := string(rows[right].Family) + "\x00" + string(rows[right].SourceFormat)
		return leftKey < rightKey
	})
	return rows
}

func constructAuditFindingsV3(shortfalls []CorpusCount, attempts, rejections int) []string {
	findings := []string{
		fmt.Sprintf("complete deterministic attempt universe retained: attempts=%d rejections=%d", attempts, rejections),
		"construct predicates were not relaxed to fill family quotas",
		"typed invocation and presentation proofs were evaluated on the frozen natural trajectory corpus",
	}
	if len(shortfalls) == 0 {
		findings = append(findings, "all primary family quotas satisfied under the v3 construct firewall")
	} else {
		for _, shortfall := range shortfalls {
			findings = append(findings, fmt.Sprintf("family %s quota shortfall=%d", shortfall.ID, shortfall.Count))
		}
	}
	sort.Strings(findings)
	return findings
}

func SealCorpusDevelopmentAuditV3(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAuditV3) (CorpusDevelopmentAuditV3, error) {
	audit.SchemaVersion = CorpusDevelopmentAuditSchemaVersionV3
	audit.CanonicalPolicy = CanonicalPolicy
	audit.Digest = ""
	digest, err := audit.digest()
	if err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	audit.Digest = digest
	if err := audit.Validate(plan); err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	return audit, nil
}

func (audit CorpusDevelopmentAuditV3) Validate(plan CorpusDevelopmentPlan) error {
	if err := validateV3CorpusPlan(plan); err != nil {
		return err
	}
	if audit.SchemaVersion != CorpusDevelopmentAuditSchemaVersionV3 || audit.CanonicalPolicy != CanonicalPolicy ||
		audit.PlanDigest != plan.Digest || audit.CorpusVersion != plan.CorpusVersion || audit.MutationProgramDigest != plan.MutationProgramDigest {
		return errors.New("v3 corpus development audit does not bind its plan or program")
	}
	if _, err := time.Parse(time.DateOnly, audit.AuditedAt); err != nil {
		return errors.New("v3 corpus development audit date is invalid")
	}
	if len(audit.Sources) != plan.SourceTasks*plan.TrajectoriesPerTask || audit.SourceTasks != plan.SourceTasks || uniqueSourceTasks(audit.Sources) != plan.SourceTasks {
		return errors.New("v3 corpus development audit source coverage is invalid")
	}
	if err := validateCorpusSources(audit.Sources, plan.Seed); err != nil {
		return err
	}
	for _, digest := range []string{audit.SourceSetDigest, audit.AttemptSetDigest, audit.FirewallSetDigest, audit.SelectedCaseSetDigest} {
		if !validDigest(digest) {
			return errors.New("v3 corpus development audit set digest is invalid")
		}
	}
	if digest, err := digestJSON(audit.Sources); err != nil || digest != audit.SourceSetDigest {
		return errors.New("v3 corpus development source-set digest does not reproduce")
	}
	if digest, err := digestJSON(audit.Attempts); err != nil || digest != audit.AttemptSetDigest {
		return errors.New("v3 corpus development attempt-set digest does not reproduce")
	}
	if digest, err := digestJSON(struct {
		Applied  []ConstructFirewallReportV2 `json:"applied"`
		Rejected []ConstructFirewallReportV2 `json:"rejected"`
	}{Applied: audit.AppliedFirewalls, Rejected: audit.RejectedFirewalls}); err != nil || digest != audit.FirewallSetDigest {
		return errors.New("v3 corpus development firewall-set digest does not reproduce")
	}
	if digest, err := digestJSON(audit.SelectedCaseIDs); err != nil || digest != audit.SelectedCaseSetDigest {
		return errors.New("v3 corpus development selected-case digest does not reproduce")
	}
	if err := validateDevelopmentFirewallsV3(audit.AppliedFirewalls, ConstructApplied); err != nil {
		return err
	}
	if err := validateDevelopmentFirewallsV3(audit.RejectedFirewalls, ConstructRejected); err != nil {
		return err
	}
	if err := validateConstructAttemptsV3(plan, audit); err != nil {
		return err
	}
	expectedCoverage := constructCoverageV3(audit.Attempts, audit.AppliedFirewalls, audit.RejectedFirewalls)
	if equal, err := equalDigest(expectedCoverage, audit.Coverage); err != nil || !equal {
		return errors.New("v3 corpus development coverage does not reproduce from attempts")
	}
	expectedSelected := selectedAttemptMutationIDs(audit.Attempts)
	if !slices.Equal(expectedSelected, audit.SelectedCaseIDs) {
		return errors.New("v3 corpus development selected cases do not reproduce from attempts")
	}
	expectedShortfalls := quotaShortfalls(plan, audit.Attempts)
	if equal, err := equalDigest(expectedShortfalls, audit.QuotaShortfalls); err != nil || !equal {
		return errors.New("v3 corpus development quota shortfalls do not reproduce")
	}
	if audit.TotalAttempts != len(audit.Attempts) || audit.AppliedAttempts != len(audit.AppliedFirewalls) ||
		audit.RejectedAttempts != len(audit.RejectedFirewalls) || audit.TotalAttempts != audit.AppliedAttempts+audit.RejectedAttempts ||
		audit.SelectedCases != len(audit.SelectedCaseIDs) || audit.QuotasSatisfied != (len(expectedShortfalls) == 0) {
		return errors.New("v3 corpus development aggregate counts are inconsistent")
	}
	expectedFindings := constructAuditFindingsV3(expectedShortfalls, audit.TotalAttempts, audit.RejectedAttempts)
	if !slices.Equal(expectedFindings, audit.Findings) {
		return errors.New("v3 corpus development findings do not reproduce")
	}
	expected, err := audit.digest()
	if err != nil {
		return err
	}
	if audit.Digest != expected {
		return errors.New("v3 corpus development audit digest is invalid")
	}
	return nil
}

func validateV3CorpusPlan(plan CorpusDevelopmentPlan) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if plan.SchemaVersion != CorpusDevelopmentPlanSchemaVersionV3 || plan.MutationProgramDigest != mutationProgramDigest(MutationProgramVersionV3) {
		return errors.New("v3 corpus audit requires the frozen v3 development plan")
	}
	return nil
}

func validateDevelopmentFirewallsV3(reports []ConstructFirewallReportV2, status ConstructStatus) error {
	for index, report := range reports {
		if index > 0 && reports[index-1].Digest >= report.Digest {
			return errors.New("v3 corpus development firewalls must be unique and digest-sorted")
		}
		if report.Status != status {
			return fmt.Errorf("v3 corpus development firewall %q has status %q", report.Digest, report.Status)
		}
		if err := report.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateConstructAttemptsV3(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAuditV3) error {
	sourceByID := make(map[string]CorpusSource, len(audit.Sources))
	for _, source := range audit.Sources {
		sourceByID[source.ID] = source
	}
	reportByDigest := make(map[string]ConstructFirewallReportV2, len(audit.AppliedFirewalls)+len(audit.RejectedFirewalls))
	for _, report := range audit.AppliedFirewalls {
		reportByDigest[report.Digest] = report
	}
	for _, report := range audit.RejectedFirewalls {
		reportByDigest[report.Digest] = report
	}
	usedReports := make(map[string]struct{}, len(reportByDigest))
	for index, attempt := range audit.Attempts {
		if index > 0 && audit.Attempts[index-1].ID >= attempt.ID {
			return errors.New("v3 corpus development attempts must be unique and sorted")
		}
		definition, exists := DefinitionFor(attempt.Family)
		if !exists || !slices.Contains(plan.PrimaryFamilies, attempt.Family) {
			return fmt.Errorf("v3 corpus development attempt %q has an invalid family", attempt.ID)
		}
		expectedSourceCount := 1
		if definition.PairLevel {
			expectedSourceCount = 2
		}
		if len(attempt.SourceIDs) != expectedSourceCount || missing(attempt.SplitGroupID, attempt.FirewallDigest) {
			return fmt.Errorf("v3 corpus development attempt %q source binding is incomplete", attempt.ID)
		}
		trajectoryDigests := make([]string, 0, len(attempt.SourceIDs))
		for _, sourceID := range attempt.SourceIDs {
			source, exists := sourceByID[sourceID]
			if !exists || source.SplitGroupID != attempt.SplitGroupID || source.SourceFormat != attempt.SourceFormat {
				return fmt.Errorf("v3 corpus development attempt %q does not bind one source group and format", attempt.ID)
			}
			trajectoryDigests = append(trajectoryDigests, source.TrajectoryDigest)
		}
		report, exists := reportByDigest[attempt.FirewallDigest]
		if !exists || report.Family != attempt.Family || report.Status != attempt.Status {
			return fmt.Errorf("v3 corpus development attempt %q does not bind its firewall", attempt.ID)
		}
		expectedSourceDigest := trajectoryDigests[0]
		if definition.PairLevel {
			var err error
			expectedSourceDigest, err = digestJSON(trajectoryDigests)
			if err != nil {
				return err
			}
		}
		if report.SourceTrajectoryDigest != expectedSourceDigest {
			return fmt.Errorf("v3 corpus development attempt %q firewall source differs", attempt.ID)
		}
		if attempt.Status == ConstructApplied {
			if !strings.HasPrefix(attempt.MutationID, "mutation-") {
				return fmt.Errorf("v3 corpus development attempt %q lacks a mutation identity", attempt.ID)
			}
		} else if attempt.MutationID != "" || attempt.Selected {
			return fmt.Errorf("rejected v3 corpus development attempt %q cannot be selected", attempt.ID)
		}
		expectedAttempt, err := sealConstructAttempt(attempt)
		if err != nil || expectedAttempt.ID != attempt.ID {
			return fmt.Errorf("v3 corpus development attempt %q identity is invalid", attempt.ID)
		}
		if _, duplicate := usedReports[attempt.FirewallDigest]; duplicate {
			return fmt.Errorf("v3 corpus development firewall %q is reused", attempt.FirewallDigest)
		}
		usedReports[attempt.FirewallDigest] = struct{}{}
	}
	if len(usedReports) != len(reportByDigest) {
		return errors.New("v3 corpus development audit contains an orphan firewall")
	}
	return nil
}

func (audit CorpusDevelopmentAuditV3) digest() (string, error) {
	audit.Digest = ""
	return digestJSON(audit)
}
