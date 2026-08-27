package mutation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	CorpusDevelopmentPlanSchemaVersion   = "evalwitness.corruption-corpus-development-plan.v2"
	CorpusDevelopmentAuditSchemaVersion  = "evalwitness.corruption-corpus-development-audit.v2"
	CorpusDevelopmentPlanSchemaVersionV3 = "evalwitness.corruption-corpus-development-plan.v3"
)

type CorpusDevelopmentPlan struct {
	SchemaVersion         string         `json:"schema_version"`
	CanonicalPolicy       string         `json:"canonical_policy"`
	CorpusVersion         string         `json:"corpus_version"`
	Seed                  string         `json:"seed"`
	SourceTasks           int            `json:"source_tasks"`
	TerminalTasks         int            `json:"terminal_tasks"`
	SWETasks              int            `json:"swe_tasks"`
	TrajectoriesPerTask   int            `json:"trajectories_per_task"`
	CasesPerFamily        int            `json:"cases_per_family"`
	PrimaryFamilies       []Family       `json:"primary_families"`
	MutationProgramDigest string         `json:"mutation_program_digest"`
	DesignEvidenceDigest  string         `json:"design_evidence_digest"`
	Design                RelationDesign `json:"design"`
	Digest                string         `json:"digest"`
}

type ConstructAttempt struct {
	ID             string                  `json:"id"`
	Family         Family                  `json:"family"`
	SourceFormat   preprocess.SourceFormat `json:"source_format"`
	SourceIDs      []string                `json:"source_ids"`
	SplitGroupID   string                  `json:"split_group_id"`
	Status         ConstructStatus         `json:"status"`
	FirewallDigest string                  `json:"firewall_digest"`
	MutationID     string                  `json:"mutation_id,omitempty"`
	Selected       bool                    `json:"selected"`
}

type ConstructCoverage struct {
	Family                Family                  `json:"family"`
	SourceFormat          preprocess.SourceFormat `json:"source_format"`
	Attempted             int                     `json:"attempted"`
	Applied               int                     `json:"applied"`
	Rejected              int                     `json:"rejected"`
	EligibleTaskGroups    int                     `json:"eligible_task_groups"`
	SelectedCases         int                     `json:"selected_cases"`
	RejectionReasonCounts []CorpusCount           `json:"rejection_reason_counts"`
}

type CorpusDevelopmentAudit struct {
	SchemaVersion         string                    `json:"schema_version"`
	CanonicalPolicy       string                    `json:"canonical_policy"`
	AuditedAt             string                    `json:"audited_at"`
	PlanDigest            string                    `json:"plan_digest"`
	CorpusVersion         string                    `json:"corpus_version"`
	MutationProgramDigest string                    `json:"mutation_program_digest"`
	SourceSetDigest       string                    `json:"source_set_digest"`
	AttemptSetDigest      string                    `json:"attempt_set_digest"`
	FirewallSetDigest     string                    `json:"firewall_set_digest"`
	SelectedCaseSetDigest string                    `json:"selected_case_set_digest"`
	Sources               []CorpusSource            `json:"sources"`
	Attempts              []ConstructAttempt        `json:"attempts"`
	AppliedFirewalls      []ConstructFirewallReport `json:"applied_firewalls"`
	RejectedFirewalls     []ConstructFirewallReport `json:"rejected_firewalls"`
	Coverage              []ConstructCoverage       `json:"coverage"`
	SelectedCaseIDs       []string                  `json:"selected_case_ids"`
	QuotaShortfalls       []CorpusCount             `json:"quota_shortfalls"`
	SourceTasks           int                       `json:"source_tasks"`
	TotalAttempts         int                       `json:"total_attempts"`
	AppliedAttempts       int                       `json:"applied_attempts"`
	RejectedAttempts      int                       `json:"rejected_attempts"`
	SelectedCases         int                       `json:"selected_cases"`
	QuotasSatisfied       bool                      `json:"quotas_satisfied"`
	Findings              []string                  `json:"findings"`
	Digest                string                    `json:"digest"`
}

func DefaultCorpusDevelopmentPlanV2() (CorpusDevelopmentPlan, error) {
	legacy, err := DefaultCorpusSpec()
	if err != nil {
		return CorpusDevelopmentPlan{}, err
	}
	return SealCorpusDevelopmentPlan(CorpusDevelopmentPlan{
		CorpusVersion: "evalwitness-controlled-corruption.v2", Seed: "evalwitness-controlled-corruption-v2-frozen-seed",
		SourceTasks: legacy.SourceTasks, TerminalTasks: legacy.TerminalTasks, SWETasks: legacy.SWETasks,
		TrajectoriesPerTask: legacy.TrajectoriesPerTask, CasesPerFamily: legacy.CasesPerFamily,
		PrimaryFamilies: append([]Family(nil), legacy.PrimaryFamilies...), MutationProgramDigest: mutationProgramDigest(MutationProgramVersionV2),
		DesignEvidenceDigest: legacy.DesignEvidenceDigest, Design: legacy.Design,
	})
}

func DefaultCorpusDevelopmentPlanV3() (CorpusDevelopmentPlan, error) {
	legacy, err := DefaultCorpusSpec()
	if err != nil {
		return CorpusDevelopmentPlan{}, err
	}
	return SealCorpusDevelopmentPlanV3(CorpusDevelopmentPlan{
		CorpusVersion: "evalwitness-controlled-corruption.v3", Seed: "evalwitness-controlled-corruption-v3-frozen-seed",
		SourceTasks: legacy.SourceTasks, TerminalTasks: legacy.TerminalTasks, SWETasks: legacy.SWETasks,
		TrajectoriesPerTask: legacy.TrajectoriesPerTask, CasesPerFamily: legacy.CasesPerFamily,
		PrimaryFamilies: append([]Family(nil), legacy.PrimaryFamilies...), MutationProgramDigest: mutationProgramDigest(MutationProgramVersionV3),
		DesignEvidenceDigest: legacy.DesignEvidenceDigest, Design: legacy.Design,
	})
}

func SealCorpusDevelopmentPlan(plan CorpusDevelopmentPlan) (CorpusDevelopmentPlan, error) {
	return sealCorpusDevelopmentPlan(plan, CorpusDevelopmentPlanSchemaVersion)
}

func SealCorpusDevelopmentPlanV3(plan CorpusDevelopmentPlan) (CorpusDevelopmentPlan, error) {
	return sealCorpusDevelopmentPlan(plan, CorpusDevelopmentPlanSchemaVersionV3)
}

func sealCorpusDevelopmentPlan(plan CorpusDevelopmentPlan, schemaVersion string) (CorpusDevelopmentPlan, error) {
	plan.SchemaVersion = schemaVersion
	plan.CanonicalPolicy = CanonicalPolicy
	plan.PrimaryFamilies = append([]Family(nil), plan.PrimaryFamilies...)
	sort.Slice(plan.PrimaryFamilies, func(left, right int) bool { return plan.PrimaryFamilies[left] < plan.PrimaryFamilies[right] })
	plan.Digest = ""
	digest, err := plan.digest()
	if err != nil {
		return CorpusDevelopmentPlan{}, err
	}
	plan.Digest = digest
	if err := plan.Validate(); err != nil {
		return CorpusDevelopmentPlan{}, err
	}
	return plan, nil
}

func (plan CorpusDevelopmentPlan) Validate() error {
	programVersion := ""
	switch plan.SchemaVersion {
	case CorpusDevelopmentPlanSchemaVersion:
		programVersion = MutationProgramVersionV2
	case CorpusDevelopmentPlanSchemaVersionV3:
		programVersion = MutationProgramVersionV3
	default:
		return errors.New("corpus development plan schema is unsupported")
	}
	if plan.CanonicalPolicy != CanonicalPolicy ||
		missing(plan.CorpusVersion, plan.Seed) || plan.SourceTasks < 40 || plan.TerminalTasks < 1 || plan.SWETasks < 1 ||
		plan.TerminalTasks+plan.SWETasks != plan.SourceTasks || plan.TrajectoriesPerTask != 2 || plan.CasesPerFamily < 1 ||
		len(plan.PrimaryFamilies) < 8 || plan.MutationProgramDigest != mutationProgramDigest(programVersion) ||
		!validDigest(plan.DesignEvidenceDigest) {
		return fmt.Errorf("%s corpus development plan identity, source design, or mutation program is invalid", programVersion)
	}
	if !slices.IsSorted(plan.PrimaryFamilies) {
		return fmt.Errorf("%s corpus development families must be sorted", programVersion)
	}
	seen := make(map[Family]struct{}, len(plan.PrimaryFamilies))
	for _, family := range plan.PrimaryFamilies {
		definition, exists := DefinitionFor(family)
		if !exists || definition.RequiresGoldProof || family == FamilyAmbiguousSemanticEdit {
			return fmt.Errorf("%s corpus development family %q is ineligible for the primary program", programVersion, family)
		}
		if _, exists := seen[family]; exists {
			return fmt.Errorf("duplicate %s corpus development family %q", programVersion, family)
		}
		seen[family] = struct{}{}
	}
	if err := plan.Design.Validate(plan.CasesPerFamily, len(plan.PrimaryFamilies)); err != nil {
		return err
	}
	designDigest, err := digestJSON(plan.Design)
	if err != nil || designDigest != plan.DesignEvidenceDigest {
		return fmt.Errorf("%s corpus development design digest is invalid", programVersion)
	}
	expected, err := plan.digest()
	if err != nil {
		return err
	}
	if plan.Digest != expected {
		return fmt.Errorf("%s corpus development plan digest is invalid", programVersion)
	}
	return nil
}

func (plan CorpusDevelopmentPlan) digest() (string, error) {
	plan.Digest = ""
	return digestJSON(plan)
}

func (plan CorpusDevelopmentPlan) corpusSpecShape() CorpusSpec {
	return CorpusSpec{
		CorpusVersion: plan.CorpusVersion, Seed: plan.Seed, SourceTasks: plan.SourceTasks,
		TerminalTasks: plan.TerminalTasks, SWETasks: plan.SWETasks, TrajectoriesPerTask: plan.TrajectoriesPerTask,
		CasesPerFamily: plan.CasesPerFamily, PrimaryFamilies: append([]Family(nil), plan.PrimaryFamilies...),
		MutationProgramDigest: plan.MutationProgramDigest, DesignEvidenceDigest: plan.DesignEvidenceDigest, Design: plan.Design,
	}
}

func AuditCorpusV2(plan CorpusDevelopmentPlan, candidates []SourceCandidate, auditedAt string) (CorpusDevelopmentAudit, error) {
	if err := plan.Validate(); err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	if _, err := time.Parse(time.DateOnly, auditedAt); err != nil {
		return CorpusDevelopmentAudit{}, errors.New("v2 corpus development audit requires YYYY-MM-DD audited_at")
	}
	selected, err := selectCorpusSources(plan.corpusSpecShape(), candidates)
	if err != nil {
		return CorpusDevelopmentAudit{}, err
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
	appliedFirewalls := make([]ConstructFirewallReport, 0)
	rejectedFirewalls := make([]ConstructFirewallReport, 0)
	for _, family := range plan.PrimaryFamilies {
		definition, _ := DefinitionFor(family)
		var familyAttempts []ConstructAttempt
		var familyApplied []ConstructFirewallReport
		var familyRejected []ConstructFirewallReport
		if definition.PairLevel {
			familyAttempts, familyApplied, familyRejected, err = auditPairFamilyV2(plan, selected, family)
		} else {
			familyAttempts, familyApplied, familyRejected, err = auditTrajectoryFamilyV2(plan, selected, family)
		}
		if err != nil {
			return CorpusDevelopmentAudit{}, err
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
	coverage := constructCoverage(attempts, appliedFirewalls, rejectedFirewalls)
	selectedCaseIDs := selectedAttemptMutationIDs(attempts)
	shortfalls := quotaShortfalls(plan, attempts)
	findings := constructAuditFindings(shortfalls, len(attempts), len(rejectedFirewalls))

	sourceSetDigest, err := digestJSON(sources)
	if err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	attemptSetDigest, err := digestJSON(attempts)
	if err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	firewallSetDigest, err := digestJSON(struct {
		Applied  []ConstructFirewallReport `json:"applied"`
		Rejected []ConstructFirewallReport `json:"rejected"`
	}{Applied: appliedFirewalls, Rejected: rejectedFirewalls})
	if err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	selectedCaseSetDigest, err := digestJSON(selectedCaseIDs)
	if err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	audit := CorpusDevelopmentAudit{
		AuditedAt: auditedAt, PlanDigest: plan.Digest, CorpusVersion: plan.CorpusVersion,
		MutationProgramDigest: plan.MutationProgramDigest, SourceSetDigest: sourceSetDigest,
		AttemptSetDigest: attemptSetDigest, FirewallSetDigest: firewallSetDigest, SelectedCaseSetDigest: selectedCaseSetDigest,
		Sources: sources, Attempts: attempts, AppliedFirewalls: appliedFirewalls, RejectedFirewalls: rejectedFirewalls,
		Coverage: coverage, SelectedCaseIDs: selectedCaseIDs, QuotaShortfalls: shortfalls,
		SourceTasks: uniqueSourceTasks(sources), TotalAttempts: len(attempts), AppliedAttempts: len(appliedFirewalls),
		RejectedAttempts: len(rejectedFirewalls), SelectedCases: len(selectedCaseIDs), QuotasSatisfied: len(shortfalls) == 0,
		Findings: findings,
	}
	return SealCorpusDevelopmentAudit(plan, audit)
}

func auditTrajectoryFamilyV2(plan CorpusDevelopmentPlan, candidates []SourceCandidate, family Family) ([]ConstructAttempt, []ConstructFirewallReport, []ConstructFirewallReport, error) {
	ordered := append([]SourceCandidate(nil), candidates...)
	sort.Slice(ordered, func(left, right int) bool {
		return digestText(plan.Seed+"\x00"+string(family)+"\x00"+ordered[left].Source.ID) < digestText(plan.Seed+"\x00"+string(family)+"\x00"+ordered[right].Source.ID)
	})
	resolvedGroups := make(map[string]struct{})
	selectedCases := 0
	attempts := make([]ConstructAttempt, 0)
	applied := make([]ConstructFirewallReport, 0)
	rejected := make([]ConstructFirewallReport, 0)
	requestSpec := plan.corpusSpecShape()
	for _, candidate := range ordered {
		if _, resolved := resolvedGroups[candidate.Source.SplitGroupID]; resolved {
			continue
		}
		outcome, err := ApplyV2(candidate.Trajectory, corpusApplyRequest(requestSpec, family, candidate.Source))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("audit v2 family %q source %q: %w", family, candidate.Source.ID, err)
		}
		attempt := ConstructAttempt{
			Family: family, SourceFormat: candidate.Source.SourceFormat, SourceIDs: []string{candidate.Source.ID},
			SplitGroupID: candidate.Source.SplitGroupID, Status: outcome.Status, FirewallDigest: outcome.Firewall.Digest,
		}
		switch outcome.Status {
		case ConstructApplied:
			if outcome.Applied == nil {
				return nil, nil, nil, fmt.Errorf("audit v2 family %q source %q produced no applied result", family, candidate.Source.ID)
			}
			if _, err := ReduceChangedRegions(outcome.Applied.Manifest, candidate.Trajectory, outcome.Applied.Mutated); err != nil {
				return nil, nil, nil, fmt.Errorf("audit v2 family %q source %q reduction: %w", family, candidate.Source.ID, err)
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
			return nil, nil, nil, fmt.Errorf("audit v2 family %q source %q returned invalid status %q", family, candidate.Source.ID, outcome.Status)
		}
		attempt, err = sealConstructAttempt(attempt)
		if err != nil {
			return nil, nil, nil, err
		}
		attempts = append(attempts, attempt)
	}
	return attempts, applied, rejected, nil
}

func auditPairFamilyV2(plan CorpusDevelopmentPlan, candidates []SourceCandidate, family Family) ([]ConstructAttempt, []ConstructFirewallReport, []ConstructFirewallReport, error) {
	groups := make(map[string][]SourceCandidate)
	for _, candidate := range candidates {
		groups[candidate.Source.SplitGroupID] = append(groups[candidate.Source.SplitGroupID], candidate)
	}
	groupIDs := rankedGroups(plan.Seed+"\x00"+string(family), groups, func(values []SourceCandidate) bool { return len(values) >= 2 })
	attempts := make([]ConstructAttempt, 0, len(groupIDs))
	applied := make([]ConstructFirewallReport, 0, len(groupIDs))
	selectedCases := 0
	requestSpec := plan.corpusSpecShape()
	for _, groupID := range groupIDs {
		values := append([]SourceCandidate(nil), groups[groupID]...)
		sort.Slice(values, func(left, right int) bool {
			return digestText(plan.Seed+"\x00"+string(family)+"\x00"+values[left].Source.ID) < digestText(plan.Seed+"\x00"+string(family)+"\x00"+values[right].Source.ID)
		})
		left, right := values[0], values[1]
		request := corpusApplyRequest(requestSpec, family, left.Source)
		request.SourceFamily = "paired/" + left.Source.SourceFamily + "+" + right.Source.SourceFamily
		request.SourceLocation = "pair/" + groupID + "/" + digestText(left.Source.ID+"\x00"+right.Source.ID)
		request.SourceRevision = left.Source.SourceRevision + "+" + right.Source.SourceRevision
		request.Outcome = SourceOutcome{
			Kind: "paired_benchmark_rewards", Value: left.Source.Outcome.Value + "," + right.Source.Outcome.Value,
			WitnessDigest: digestText(left.Source.Outcome.WitnessDigest + "\x00" + right.Source.Outcome.WitnessDigest),
		}
		outcome, err := ApplyCandidateOrderReversalV2(left.Trajectory, right.Trajectory, request)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("audit v2 pair family %q group %q: %w", family, groupID, err)
		}
		if outcome.Status != ConstructApplied || outcome.Applied == nil {
			return nil, nil, nil, fmt.Errorf("audit v2 pair family %q group %q was not applied", family, groupID)
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
	return attempts, applied, []ConstructFirewallReport{}, nil
}

func sealConstructAttempt(attempt ConstructAttempt) (ConstructAttempt, error) {
	attempt.ID = ""
	digest, err := digestJSON(attempt)
	if err != nil {
		return ConstructAttempt{}, err
	}
	attempt.ID = "attempt-" + digest
	return attempt, nil
}

func constructCoverage(attempts []ConstructAttempt, applied, rejected []ConstructFirewallReport) []ConstructCoverage {
	reports := make(map[string]ConstructFirewallReport, len(applied)+len(rejected))
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

func selectedAttemptMutationIDs(attempts []ConstructAttempt) []string {
	result := make([]string, 0)
	for _, attempt := range attempts {
		if attempt.Selected {
			result = append(result, attempt.MutationID)
		}
	}
	sort.Strings(result)
	return result
}

func quotaShortfalls(plan CorpusDevelopmentPlan, attempts []ConstructAttempt) []CorpusCount {
	selected := make(map[Family]int)
	for _, attempt := range attempts {
		if attempt.Selected {
			selected[attempt.Family]++
		}
	}
	shortfalls := make([]CorpusCount, 0)
	for _, family := range plan.PrimaryFamilies {
		if missingCases := plan.CasesPerFamily - selected[family]; missingCases > 0 {
			shortfalls = append(shortfalls, CorpusCount{ID: string(family), Count: missingCases})
		}
	}
	return shortfalls
}

func constructAuditFindings(shortfalls []CorpusCount, attempts, rejections int) []string {
	findings := []string{
		fmt.Sprintf("complete deterministic attempt universe retained: attempts=%d rejections=%d", attempts, rejections),
		"construct predicates were not relaxed to fill family quotas",
	}
	if len(shortfalls) == 0 {
		findings = append(findings, "all primary family quotas satisfied under the v2 construct firewall")
	} else {
		for _, shortfall := range shortfalls {
			findings = append(findings, fmt.Sprintf("family %s quota shortfall=%d", shortfall.ID, shortfall.Count))
		}
	}
	sort.Strings(findings)
	return findings
}

func uniqueSourceTasks(sources []CorpusSource) int {
	groups := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		groups[source.SplitGroupID] = struct{}{}
	}
	return len(groups)
}

func SealCorpusDevelopmentAudit(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAudit) (CorpusDevelopmentAudit, error) {
	audit.SchemaVersion = CorpusDevelopmentAuditSchemaVersion
	audit.CanonicalPolicy = CanonicalPolicy
	audit.Digest = ""
	digest, err := audit.digest()
	if err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	audit.Digest = digest
	if err := audit.Validate(plan); err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	return audit, nil
}

func (audit CorpusDevelopmentAudit) Validate(plan CorpusDevelopmentPlan) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if audit.SchemaVersion != CorpusDevelopmentAuditSchemaVersion || audit.CanonicalPolicy != CanonicalPolicy ||
		audit.PlanDigest != plan.Digest || audit.CorpusVersion != plan.CorpusVersion || audit.MutationProgramDigest != plan.MutationProgramDigest {
		return errors.New("v2 corpus development audit does not bind its plan or program")
	}
	if _, err := time.Parse(time.DateOnly, audit.AuditedAt); err != nil {
		return errors.New("v2 corpus development audit date is invalid")
	}
	if len(audit.Sources) != plan.SourceTasks*plan.TrajectoriesPerTask || audit.SourceTasks != plan.SourceTasks || uniqueSourceTasks(audit.Sources) != plan.SourceTasks {
		return errors.New("v2 corpus development audit source coverage is invalid")
	}
	if err := validateCorpusSources(audit.Sources, plan.Seed); err != nil {
		return err
	}
	for _, digest := range []string{audit.SourceSetDigest, audit.AttemptSetDigest, audit.FirewallSetDigest, audit.SelectedCaseSetDigest} {
		if !validDigest(digest) {
			return errors.New("v2 corpus development audit set digest is invalid")
		}
	}
	if digest, err := digestJSON(audit.Sources); err != nil || digest != audit.SourceSetDigest {
		return errors.New("v2 corpus development source-set digest does not reproduce")
	}
	if digest, err := digestJSON(audit.Attempts); err != nil || digest != audit.AttemptSetDigest {
		return errors.New("v2 corpus development attempt-set digest does not reproduce")
	}
	if digest, err := digestJSON(struct {
		Applied  []ConstructFirewallReport `json:"applied"`
		Rejected []ConstructFirewallReport `json:"rejected"`
	}{Applied: audit.AppliedFirewalls, Rejected: audit.RejectedFirewalls}); err != nil || digest != audit.FirewallSetDigest {
		return errors.New("v2 corpus development firewall-set digest does not reproduce")
	}
	if digest, err := digestJSON(audit.SelectedCaseIDs); err != nil || digest != audit.SelectedCaseSetDigest {
		return errors.New("v2 corpus development selected-case digest does not reproduce")
	}
	if err := validateDevelopmentFirewalls(audit.AppliedFirewalls, ConstructApplied); err != nil {
		return err
	}
	if err := validateDevelopmentFirewalls(audit.RejectedFirewalls, ConstructRejected); err != nil {
		return err
	}
	if err := validateConstructAttempts(plan, audit); err != nil {
		return err
	}
	expectedCoverage := constructCoverage(audit.Attempts, audit.AppliedFirewalls, audit.RejectedFirewalls)
	if equal, err := equalDigest(expectedCoverage, audit.Coverage); err != nil || !equal {
		return errors.New("v2 corpus development coverage does not reproduce from attempts")
	}
	expectedSelected := selectedAttemptMutationIDs(audit.Attempts)
	if !slices.Equal(expectedSelected, audit.SelectedCaseIDs) {
		return errors.New("v2 corpus development selected cases do not reproduce from attempts")
	}
	expectedShortfalls := quotaShortfalls(plan, audit.Attempts)
	if equal, err := equalDigest(expectedShortfalls, audit.QuotaShortfalls); err != nil || !equal {
		return errors.New("v2 corpus development quota shortfalls do not reproduce")
	}
	if audit.TotalAttempts != len(audit.Attempts) || audit.AppliedAttempts != len(audit.AppliedFirewalls) ||
		audit.RejectedAttempts != len(audit.RejectedFirewalls) || audit.TotalAttempts != audit.AppliedAttempts+audit.RejectedAttempts ||
		audit.SelectedCases != len(audit.SelectedCaseIDs) || audit.QuotasSatisfied != (len(expectedShortfalls) == 0) {
		return errors.New("v2 corpus development aggregate counts are inconsistent")
	}
	expectedFindings := constructAuditFindings(expectedShortfalls, audit.TotalAttempts, audit.RejectedAttempts)
	if !slices.Equal(expectedFindings, audit.Findings) {
		return errors.New("v2 corpus development findings do not reproduce")
	}
	expected, err := audit.digest()
	if err != nil {
		return err
	}
	if audit.Digest != expected {
		return errors.New("v2 corpus development audit digest is invalid")
	}
	return nil
}

func validateDevelopmentFirewalls(reports []ConstructFirewallReport, status ConstructStatus) error {
	for index, report := range reports {
		if index > 0 && reports[index-1].Digest >= report.Digest {
			return errors.New("v2 corpus development firewalls must be unique and digest-sorted")
		}
		if report.Status != status {
			return fmt.Errorf("v2 corpus development firewall %q has status %q", report.Digest, report.Status)
		}
		if err := report.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateConstructAttempts(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAudit) error {
	sourceByID := make(map[string]CorpusSource, len(audit.Sources))
	for _, source := range audit.Sources {
		sourceByID[source.ID] = source
	}
	reportByDigest := make(map[string]ConstructFirewallReport, len(audit.AppliedFirewalls)+len(audit.RejectedFirewalls))
	for _, report := range audit.AppliedFirewalls {
		reportByDigest[report.Digest] = report
	}
	for _, report := range audit.RejectedFirewalls {
		reportByDigest[report.Digest] = report
	}
	usedReports := make(map[string]struct{}, len(reportByDigest))
	for index, attempt := range audit.Attempts {
		if index > 0 && audit.Attempts[index-1].ID >= attempt.ID {
			return errors.New("v2 corpus development attempts must be unique and sorted")
		}
		definition, exists := DefinitionFor(attempt.Family)
		if !exists || !slices.Contains(plan.PrimaryFamilies, attempt.Family) {
			return fmt.Errorf("v2 corpus development attempt %q has an invalid family", attempt.ID)
		}
		expectedSourceCount := 1
		if definition.PairLevel {
			expectedSourceCount = 2
		}
		if len(attempt.SourceIDs) != expectedSourceCount || missing(attempt.SplitGroupID, attempt.FirewallDigest) {
			return fmt.Errorf("v2 corpus development attempt %q source binding is incomplete", attempt.ID)
		}
		trajectoryDigests := make([]string, 0, len(attempt.SourceIDs))
		for _, sourceID := range attempt.SourceIDs {
			source, exists := sourceByID[sourceID]
			if !exists || source.SplitGroupID != attempt.SplitGroupID || source.SourceFormat != attempt.SourceFormat {
				return fmt.Errorf("v2 corpus development attempt %q does not bind one source group and format", attempt.ID)
			}
			trajectoryDigests = append(trajectoryDigests, source.TrajectoryDigest)
		}
		report, exists := reportByDigest[attempt.FirewallDigest]
		if !exists || report.Family != attempt.Family || report.Status != attempt.Status {
			return fmt.Errorf("v2 corpus development attempt %q does not bind its firewall", attempt.ID)
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
			return fmt.Errorf("v2 corpus development attempt %q firewall source differs", attempt.ID)
		}
		if attempt.Status == ConstructApplied {
			if !strings.HasPrefix(attempt.MutationID, "mutation-") {
				return fmt.Errorf("v2 corpus development attempt %q lacks a mutation identity", attempt.ID)
			}
		} else if attempt.MutationID != "" || attempt.Selected {
			return fmt.Errorf("rejected v2 corpus development attempt %q cannot be selected", attempt.ID)
		}
		expectedAttempt, err := sealConstructAttempt(attempt)
		if err != nil || expectedAttempt.ID != attempt.ID {
			return fmt.Errorf("v2 corpus development attempt %q identity is invalid", attempt.ID)
		}
		if _, duplicate := usedReports[attempt.FirewallDigest]; duplicate {
			return fmt.Errorf("v2 corpus development firewall %q is reused", attempt.FirewallDigest)
		}
		usedReports[attempt.FirewallDigest] = struct{}{}
	}
	if len(usedReports) != len(reportByDigest) {
		return errors.New("v2 corpus development audit contains an orphan firewall")
	}
	return nil
}

func equalDigest(left, right any) (bool, error) {
	leftDigest, err := digestJSON(left)
	if err != nil {
		return false, err
	}
	rightDigest, err := digestJSON(right)
	if err != nil {
		return false, err
	}
	return leftDigest == rightDigest, nil
}

func (audit CorpusDevelopmentAudit) digest() (string, error) {
	audit.Digest = ""
	return digestJSON(audit)
}

func FreezeCorpusSpecV2(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAudit) (CorpusSpec, error) {
	if err := plan.Validate(); err != nil {
		return CorpusSpec{}, err
	}
	if err := audit.Validate(plan); err != nil {
		return CorpusSpec{}, err
	}
	if !audit.QuotasSatisfied {
		return CorpusSpec{}, errors.New("cannot freeze v2 corpus specification with construct quota shortfalls")
	}
	attemptedFamilies := make([]Family, 0, len(definitions))
	excludedFamilies := make([]Family, 0)
	for _, definition := range definitions {
		attemptedFamilies = append(attemptedFamilies, definition.Family)
		if !slices.Contains(plan.PrimaryFamilies, definition.Family) {
			excludedFamilies = append(excludedFamilies, definition.Family)
		}
	}
	sort.Slice(attemptedFamilies, func(left, right int) bool { return attemptedFamilies[left] < attemptedFamilies[right] })
	sort.Slice(excludedFamilies, func(left, right int) bool { return excludedFamilies[left] < excludedFamilies[right] })
	developmentAudit := DevelopmentAudit{
		Method: "provider_free_full_corpus_build_construct_firewall_audit.v2", AuditedAt: audit.AuditedAt,
		ObservedSources: len(audit.Sources), ObservedSourceTasks: audit.SourceTasks, ObservedCases: audit.SelectedCases,
		ObservedPositiveControls: 0, AttemptedFamilies: attemptedFamilies, ExcludedFamilies: excludedFamilies,
		Findings: append([]string(nil), audit.Findings...), MutationProgramDigest: plan.MutationProgramDigest,
		ConstructAuditDigest: audit.Digest,
	}
	developmentAuditDigest, err := digestJSON(developmentAudit)
	if err != nil {
		return CorpusSpec{}, err
	}
	spec := CorpusSpec{
		SchemaVersion: CorpusSpecSchemaVersion, CorpusVersion: plan.CorpusVersion, Seed: plan.Seed,
		SourceTasks: plan.SourceTasks, TerminalTasks: plan.TerminalTasks, SWETasks: plan.SWETasks,
		TrajectoriesPerTask: plan.TrajectoriesPerTask, CasesPerFamily: plan.CasesPerFamily,
		PrimaryFamilies: append([]Family(nil), plan.PrimaryFamilies...), MutationProgramDigest: plan.MutationProgramDigest,
		DevelopmentAuditDigest: developmentAuditDigest, DesignEvidenceDigest: plan.DesignEvidenceDigest,
		MutatorsFrozen: true, Design: plan.Design, DevelopmentAudit: developmentAudit,
	}
	if err := spec.Validate(); err != nil {
		return CorpusSpec{}, err
	}
	return spec, nil
}

func VerifyCorpusV2AgainstAudit(release CorpusRelease, plan CorpusDevelopmentPlan, audit CorpusDevelopmentAudit) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if err := audit.Validate(plan); err != nil {
		return err
	}
	if err := release.Validate(); err != nil {
		return err
	}
	if !audit.QuotasSatisfied || release.MutationProgramDigest != plan.MutationProgramDigest ||
		release.Spec.DevelopmentAudit.ConstructAuditDigest != audit.Digest {
		return errors.New("v2 corpus release does not bind the satisfied development audit")
	}
	if equal, err := equalDigest(release.Sources, audit.Sources); err != nil || !equal {
		return errors.New("v2 corpus release source set differs from the development audit")
	}
	releaseCaseIDs := make([]string, len(release.Cases))
	caseByID := make(map[string]CorpusCase, len(release.Cases))
	for index, item := range release.Cases {
		releaseCaseIDs[index] = item.ID
		caseByID[item.ID] = item
	}
	sort.Strings(releaseCaseIDs)
	if !slices.Equal(releaseCaseIDs, audit.SelectedCaseIDs) {
		return errors.New("v2 corpus release cases differ from the audit-selected cases")
	}
	reportByDigest := make(map[string]ConstructFirewallReport, len(audit.AppliedFirewalls))
	for _, report := range audit.AppliedFirewalls {
		reportByDigest[report.Digest] = report
	}
	for _, attempt := range audit.Attempts {
		if !attempt.Selected {
			continue
		}
		item, exists := caseByID[attempt.MutationID]
		if !exists || item.ConstructFirewall == nil || item.ConstructFirewall.Digest != attempt.FirewallDigest {
			return fmt.Errorf("v2 corpus release case %q differs from its selected construct attempt", attempt.MutationID)
		}
		if expected, exists := reportByDigest[attempt.FirewallDigest]; !exists || item.ConstructFirewall.Digest != expected.Digest {
			return fmt.Errorf("v2 corpus release case %q does not bind its audited firewall", attempt.MutationID)
		}
	}
	if equal, err := equalDigest(release.ConstructRejections, audit.RejectedFirewalls); err != nil || !equal {
		return errors.New("v2 corpus release construct rejections differ from the complete audit")
	}
	return nil
}
