package mutation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

const (
	CorpusReleaseSchemaVersionV3 = "evalwitness.corruption-corpus-release.v3"
	CorpusReleasePolicyVersionV3 = "evalwitness.seven-family-core-plus-scarcity-sentinel.v1"
	CorpusReleaseClaimBoundaryV3 = "the seven quota-satisfied families form the controlled inferential corpus; omitted_test_evidence is an exhaustive three-case corpus-specific scarcity sentinel with no test cases and is excluded from the primary estimand"
)

type CorpusReleasePolicyV3 struct {
	Version                       string        `json:"version"`
	InferentialCoreFamilies       []Family      `json:"inferential_core_families"`
	CoreCasesPerFamily            int           `json:"core_cases_per_family"`
	CoreCases                     int           `json:"core_cases"`
	ScarcitySentinelFamily        Family        `json:"scarcity_sentinel_family"`
	ScarcitySentinelCases         int           `json:"scarcity_sentinel_cases"`
	ScarcitySentinelSplitCounts   []CorpusCount `json:"scarcity_sentinel_split_counts"`
	ScarcitySentinelExhaustive    bool          `json:"scarcity_sentinel_exhaustive"`
	SentinelInPrimaryEstimand     bool          `json:"sentinel_in_primary_estimand"`
	HeldOutSentinelClaimAvailable bool          `json:"held_out_sentinel_claim_available"`
	BalancedEightFamilyAvailable  bool          `json:"balanced_eight_family_available"`
	ClaimBoundary                 string        `json:"claim_boundary"`
}

type CorpusCaseV3 struct {
	ID                string                    `json:"id"`
	SourceIDs         []string                  `json:"source_ids"`
	Family            Family                    `json:"family"`
	Split             study.DataRole            `json:"split"`
	Control           string                    `json:"control"`
	Manifest          Manifest                  `json:"manifest"`
	BlindPacket       BlindReviewPacket         `json:"blind_review_packet"`
	Reduction         *ReductionWitness         `json:"reduction,omitempty"`
	RegenerationKey   string                    `json:"regeneration_key"`
	ConstructFirewall ConstructFirewallReportV2 `json:"construct_firewall"`
}

// CorpusReleaseV3 is additive by design. The historical rectangular v1/v2
// release type cannot represent a seven-family inferential core and a typed
// scarcity sentinel without weakening its invariants or changing old bytes.
type CorpusReleaseV3 struct {
	SchemaVersion         string                      `json:"schema_version"`
	CanonicalPolicy       string                      `json:"canonical_policy"`
	CorpusVersion         string                      `json:"corpus_version"`
	PlanDigest            string                      `json:"plan_digest"`
	AuditDigest           string                      `json:"audit_digest"`
	MutationProgramDigest string                      `json:"mutation_program_digest"`
	SplitAlgorithm        string                      `json:"split_algorithm"`
	Policy                CorpusReleasePolicyV3       `json:"policy"`
	Sources               []CorpusSource              `json:"sources"`
	Cases                 []CorpusCaseV3              `json:"cases"`
	ConstructRejections   []ConstructFirewallReportV2 `json:"construct_rejections"`
	SourceFamilyCounts    []CorpusCount               `json:"source_family_counts"`
	MutationFamilyCounts  []CorpusCount               `json:"mutation_family_counts"`
	SplitCounts           []CorpusCount               `json:"split_counts"`
	TaskCount             int                         `json:"task_count"`
	SelectedCases         int                         `json:"selected_cases"`
	AppliedAttempts       int                         `json:"applied_attempts"`
	RejectedAttempts      int                         `json:"rejected_attempts"`
	Digest                string                      `json:"digest"`
}

func BuildCorpusReleaseV3(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAuditV3, candidates []SourceCandidate) (CorpusReleaseV3, error) {
	if err := audit.Validate(plan); err != nil {
		return CorpusReleaseV3{}, err
	}
	selected, err := selectCorpusSources(plan.corpusSpecShape(), candidates)
	if err != nil {
		return CorpusReleaseV3{}, err
	}
	lineageSources := make([]CorpusSource, len(selected))
	for index := range selected {
		lineageSources[index] = selected[index].Source
	}
	lineagePlan := corpusLineagePlan(lineageSources, plan.Seed)
	candidateByID := make(map[string]SourceCandidate, len(selected))
	for index := range selected {
		assignment := lineagePlan[selected[index].Source.ID]
		selected[index].Source.LineageClusterID = assignment.ClusterID
		selected[index].Source.Split = assignment.Split
		candidateByID[selected[index].Source.ID] = selected[index]
	}
	sources := make([]CorpusSource, 0, len(selected))
	for _, candidate := range selected {
		sources = append(sources, candidate.Source)
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].ID < sources[right].ID })
	if equal, compareErr := equalDigest(sources, audit.Sources); compareErr != nil || !equal {
		return CorpusReleaseV3{}, errors.New("v3 corpus release sources do not reproduce the audited source set")
	}
	reportByDigest := make(map[string]ConstructFirewallReportV2, len(audit.AppliedFirewalls))
	for _, report := range audit.AppliedFirewalls {
		reportByDigest[report.Digest] = report
	}
	cases := make([]CorpusCaseV3, 0, audit.SelectedCases)
	for _, attempt := range audit.Attempts {
		if !attempt.Selected {
			continue
		}
		item, buildErr := buildCorpusCaseV3(plan, attempt, candidateByID)
		if buildErr != nil {
			return CorpusReleaseV3{}, buildErr
		}
		if item.ID != attempt.MutationID || item.ConstructFirewall.Digest != attempt.FirewallDigest {
			return CorpusReleaseV3{}, fmt.Errorf("v3 corpus case for attempt %q differs from its frozen audit", attempt.ID)
		}
		audited, exists := reportByDigest[item.ConstructFirewall.Digest]
		equal, compareErr := equalDigest(audited, item.ConstructFirewall)
		if !exists || compareErr != nil || !equal {
			return CorpusReleaseV3{}, fmt.Errorf("v3 corpus case %q does not reproduce its audited firewall", item.ID)
		}
		cases = append(cases, item)
	}
	sort.Slice(cases, func(left, right int) bool { return cases[left].ID < cases[right].ID })
	policy, err := corpusReleasePolicyV3(cases)
	if err != nil {
		return CorpusReleaseV3{}, err
	}
	sourceFamilies, mutationFamilies, splits := map[string]int{}, map[string]int{}, map[string]int{}
	for _, source := range sources {
		sourceFamilies[source.SourceFamily]++
		splits[string(source.Split)]++
	}
	for _, item := range cases {
		mutationFamilies[string(item.Family)]++
	}
	release := CorpusReleaseV3{
		CorpusVersion: plan.CorpusVersion, PlanDigest: plan.Digest, AuditDigest: audit.Digest,
		MutationProgramDigest: plan.MutationProgramDigest, Policy: policy, Sources: sources, Cases: cases,
		ConstructRejections: append([]ConstructFirewallReportV2(nil), audit.RejectedFirewalls...),
		SourceFamilyCounts:  countRecords(sourceFamilies), MutationFamilyCounts: countRecords(mutationFamilies), SplitCounts: countRecords(splits),
		TaskCount: plan.SourceTasks, SelectedCases: len(cases), AppliedAttempts: audit.AppliedAttempts, RejectedAttempts: audit.RejectedAttempts,
	}
	return SealCorpusReleaseV3(plan, audit, release)
}

func buildCorpusCaseV3(plan CorpusDevelopmentPlan, attempt ConstructAttempt, candidates map[string]SourceCandidate) (CorpusCaseV3, error) {
	definition, exists := DefinitionFor(attempt.Family)
	if !exists {
		return CorpusCaseV3{}, fmt.Errorf("v3 corpus attempt %q has unknown family", attempt.ID)
	}
	linked := make([]SourceCandidate, 0, len(attempt.SourceIDs))
	for _, sourceID := range attempt.SourceIDs {
		candidate, found := candidates[sourceID]
		if !found {
			return CorpusCaseV3{}, fmt.Errorf("v3 corpus attempt %q source %q is unavailable", attempt.ID, sourceID)
		}
		linked = append(linked, candidate)
	}
	request := corpusApplyRequestV3(plan, attempt.Family, linked[0].Source)
	var outcome ApplyV3Outcome
	var err error
	if definition.PairLevel {
		if len(linked) != 2 {
			return CorpusCaseV3{}, fmt.Errorf("v3 pair attempt %q source count is invalid", attempt.ID)
		}
		request.SourceFamily = "paired/" + linked[0].Source.SourceFamily + "+" + linked[1].Source.SourceFamily
		request.SourceLocation = "pair/" + attempt.SplitGroupID + "/" + digestText(linked[0].Source.ID+"\x00"+linked[1].Source.ID)
		request.SourceRevision = linked[0].Source.SourceRevision + "+" + linked[1].Source.SourceRevision
		request.Outcome = SourceOutcome{
			Kind: "paired_benchmark_rewards", Value: linked[0].Source.Outcome.Value + "," + linked[1].Source.Outcome.Value,
			WitnessDigest: digestText(linked[0].Source.Outcome.WitnessDigest + "\x00" + linked[1].Source.Outcome.WitnessDigest),
		}
		outcome, err = ApplyCandidateOrderReversalV3(linked[0].Trajectory, linked[1].Trajectory, request)
	} else {
		if len(linked) != 1 {
			return CorpusCaseV3{}, fmt.Errorf("v3 trajectory attempt %q source count is invalid", attempt.ID)
		}
		outcome, err = ApplyV3(linked[0].Trajectory, request)
	}
	if err != nil {
		return CorpusCaseV3{}, fmt.Errorf("reproduce v3 corpus attempt %q: %w", attempt.ID, err)
	}
	if outcome.Status != ConstructApplied || outcome.Applied == nil {
		return CorpusCaseV3{}, fmt.Errorf("selected v3 corpus attempt %q did not reproduce as applied", attempt.ID)
	}
	var reduction *ReductionWitness
	if !definition.PairLevel {
		value, reduceErr := ReduceChangedRegions(outcome.Applied.Manifest, linked[0].Trajectory, outcome.Applied.Mutated)
		if reduceErr != nil {
			return CorpusCaseV3{}, fmt.Errorf("reduce v3 corpus attempt %q: %w", attempt.ID, reduceErr)
		}
		reduction = &value
	}
	regenerationParts := []string{plan.MutationProgramDigest}
	regenerationParts = append(regenerationParts, attempt.SourceIDs...)
	regenerationParts = append(regenerationParts, string(attempt.Family))
	return CorpusCaseV3{
		ID: outcome.Applied.Manifest.MutationID, SourceIDs: append([]string(nil), attempt.SourceIDs...), Family: attempt.Family,
		Split: linked[0].Source.Split, Control: corpusControl(attempt.Family), Manifest: outcome.Applied.Manifest,
		BlindPacket: outcome.Applied.Packet, Reduction: reduction, RegenerationKey: digestText(strings.Join(regenerationParts, "\x00")),
		ConstructFirewall: outcome.Firewall,
	}, nil
}

func corpusReleasePolicyV3(cases []CorpusCaseV3) (CorpusReleasePolicyV3, error) {
	counts := make(map[Family]int)
	sentinelSplits := make(map[string]int)
	for _, item := range cases {
		counts[item.Family]++
		if item.Family == FamilyTestEvidenceOmitted {
			sentinelSplits[string(item.Split)]++
		}
	}
	coreFamilies := make([]Family, 0, 7)
	for family, count := range counts {
		if family == FamilyTestEvidenceOmitted {
			continue
		}
		if count != 40 {
			return CorpusReleasePolicyV3{}, fmt.Errorf("v3 inferential-core family %q has %d/40 cases", family, count)
		}
		coreFamilies = append(coreFamilies, family)
	}
	slices.Sort(coreFamilies)
	if len(coreFamilies) != 7 || counts[FamilyTestEvidenceOmitted] != 3 || sentinelSplits[string(study.RoleDevelopment)] != 2 || sentinelSplits[string(study.RoleCalibration)] != 1 || sentinelSplits[string(study.RoleTest)] != 0 {
		return CorpusReleasePolicyV3{}, errors.New("v3 corpus does not reproduce the seven-family core and 2/1/0 scarcity sentinel")
	}
	return CorpusReleasePolicyV3{
		Version: CorpusReleasePolicyVersionV3, InferentialCoreFamilies: coreFamilies, CoreCasesPerFamily: 40, CoreCases: 280,
		ScarcitySentinelFamily: FamilyTestEvidenceOmitted, ScarcitySentinelCases: 3,
		ScarcitySentinelSplitCounts: countRecords(sentinelSplits), ScarcitySentinelExhaustive: true,
		SentinelInPrimaryEstimand: false, HeldOutSentinelClaimAvailable: false, BalancedEightFamilyAvailable: false,
		ClaimBoundary: CorpusReleaseClaimBoundaryV3,
	}, nil
}

func SealCorpusReleaseV3(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAuditV3, release CorpusReleaseV3) (CorpusReleaseV3, error) {
	release.SchemaVersion, release.CanonicalPolicy, release.SplitAlgorithm, release.Digest = CorpusReleaseSchemaVersionV3, CanonicalPolicy, CorpusSplitAlgorithm, ""
	digest, err := corpusReleaseDigestV3(release)
	if err != nil {
		return CorpusReleaseV3{}, err
	}
	release.Digest = digest
	if err := release.Validate(plan, audit); err != nil {
		return CorpusReleaseV3{}, err
	}
	return release, nil
}

func (release CorpusReleaseV3) Validate(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAuditV3) error {
	if err := audit.Validate(plan); err != nil {
		return err
	}
	if release.SchemaVersion != CorpusReleaseSchemaVersionV3 || release.CanonicalPolicy != CanonicalPolicy || release.SplitAlgorithm != CorpusSplitAlgorithm ||
		release.CorpusVersion != plan.CorpusVersion || release.PlanDigest != plan.Digest || release.AuditDigest != audit.Digest || release.MutationProgramDigest != plan.MutationProgramDigest {
		return errors.New("v3 corpus release identity or governance binding is invalid")
	}
	if len(release.Sources) != len(audit.Sources) || release.TaskCount != plan.SourceTasks || release.SelectedCases != audit.SelectedCases || len(release.Cases) != audit.SelectedCases ||
		release.AppliedAttempts != audit.AppliedAttempts || release.RejectedAttempts != audit.RejectedAttempts || len(release.ConstructRejections) != audit.RejectedAttempts {
		return errors.New("v3 corpus release source, case, or attempt counts are invalid")
	}
	if equal, err := equalDigest(release.Sources, audit.Sources); err != nil || !equal {
		return errors.New("v3 corpus release source set differs from the audited source set")
	}
	if equal, err := equalDigest(release.ConstructRejections, audit.RejectedFirewalls); err != nil || !equal {
		return errors.New("v3 corpus release rejection set differs from the audited rejection set")
	}
	if err := validateCorpusSources(release.Sources, plan.Seed); err != nil {
		return err
	}
	if err := validateCorpusCasesV3(release.Cases, release.Sources, plan, audit); err != nil {
		return err
	}
	expectedPolicy, err := corpusReleasePolicyV3(release.Cases)
	if err != nil {
		return err
	}
	if equal, err := equalDigest(release.Policy, expectedPolicy); err != nil || !equal {
		return errors.New("v3 corpus release core and scarcity policy does not reproduce")
	}
	if err := validateCorpusReleaseCountsV3(release); err != nil {
		return err
	}
	expected, err := corpusReleaseDigestV3(release)
	if err != nil || release.Digest != expected {
		return errors.New("v3 corpus release digest is invalid")
	}
	return nil
}

func validateCorpusCasesV3(cases []CorpusCaseV3, sources []CorpusSource, plan CorpusDevelopmentPlan, audit CorpusDevelopmentAuditV3) error {
	sourceByID := make(map[string]CorpusSource, len(sources))
	for _, source := range sources {
		sourceByID[source.ID] = source
	}
	selectedByID := make(map[string]ConstructAttempt, audit.SelectedCases)
	for _, attempt := range audit.Attempts {
		if attempt.Selected {
			selectedByID[attempt.MutationID] = attempt
		}
	}
	seen := make(map[string]struct{}, len(cases))
	for index, item := range cases {
		if index > 0 && cases[index-1].ID >= item.ID {
			return errors.New("v3 corpus cases must be unique and identity-sorted")
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return fmt.Errorf("duplicate v3 corpus case %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		attempt, exists := selectedByID[item.ID]
		if !exists || attempt.Family != item.Family || !slices.Equal(attempt.SourceIDs, item.SourceIDs) || attempt.FirewallDigest != item.ConstructFirewall.Digest {
			return fmt.Errorf("v3 corpus case %q does not bind one selected audit attempt", item.ID)
		}
		if item.ID != item.Manifest.MutationID || item.Family != item.Manifest.Program.Family || item.Manifest.CorpusVersion != plan.CorpusVersion ||
			item.Manifest.Program.Version != MutationProgramVersionV3 || item.Manifest.Validator.ContractDigest != plan.MutationProgramDigest ||
			item.Manifest.ConstructFirewallDigest != item.ConstructFirewall.Digest || item.ConstructFirewall.Status != ConstructApplied {
			return fmt.Errorf("v3 corpus case %q identity, program, or firewall is invalid", item.ID)
		}
		if err := item.Manifest.Validate(); err != nil {
			return fmt.Errorf("v3 corpus case %q manifest: %w", item.ID, err)
		}
		if err := item.BlindPacket.Validate(); err != nil {
			return fmt.Errorf("v3 corpus case %q packet: %w", item.ID, err)
		}
		if err := item.ConstructFirewall.Validate(); err != nil {
			return fmt.Errorf("v3 corpus case %q firewall: %w", item.ID, err)
		}
		if item.Control != corpusControl(item.Family) || item.BlindPacket.Digest != item.Manifest.Review.BlindPacketDigest ||
			item.BlindPacket.OriginalDigest != item.Manifest.OriginalTrajectoryDigest || item.BlindPacket.MutatedDigest != item.Manifest.MutatedTrajectoryDigest {
			return fmt.Errorf("v3 corpus case %q control or packet binding is invalid", item.ID)
		}
		linkedSources := make([]CorpusSource, 0, len(item.SourceIDs))
		for _, sourceID := range item.SourceIDs {
			source, found := sourceByID[sourceID]
			if !found || source.Split != item.Split || source.SplitGroupID != item.Manifest.SplitGroupID {
				return fmt.Errorf("v3 corpus case %q crosses source lineage or split", item.ID)
			}
			linkedSources = append(linkedSources, source)
		}
		legacyShape := CorpusCase{ID: item.ID, SourceIDs: item.SourceIDs, Family: item.Family, Split: item.Split, Control: item.Control, Manifest: item.Manifest, BlindPacket: item.BlindPacket, Reduction: item.Reduction, RegenerationKey: item.RegenerationKey}
		definition, _ := DefinitionFor(item.Family)
		if err := validateCaseSourceBinding(legacyShape, linkedSources, definition, plan.MutationProgramDigest); err != nil {
			return err
		}
		if definition.PairLevel && item.Reduction != nil || !definition.PairLevel && item.Reduction == nil {
			return fmt.Errorf("v3 corpus case %q reduction presence is invalid", item.ID)
		}
		if item.Reduction != nil {
			if err := item.Reduction.Validate(); err != nil || item.Reduction.MutationDigest != item.Manifest.Digest {
				return fmt.Errorf("v3 corpus case %q reduction is invalid", item.ID)
			}
		}
	}
	if len(seen) != len(selectedByID) {
		return errors.New("v3 corpus release does not materialize every selected audit attempt")
	}
	return nil
}

func validateCorpusReleaseCountsV3(release CorpusReleaseV3) error {
	sourceFamilies, mutationFamilies, splits := map[string]int{}, map[string]int{}, map[string]int{}
	tasks := make(map[string]struct{})
	for _, source := range release.Sources {
		sourceFamilies[source.SourceFamily]++
		splits[string(source.Split)]++
		tasks[source.SplitGroupID] = struct{}{}
	}
	for _, item := range release.Cases {
		mutationFamilies[string(item.Family)]++
	}
	if !equalCounts(release.SourceFamilyCounts, sourceFamilies) || !equalCounts(release.MutationFamilyCounts, mutationFamilies) || !equalCounts(release.SplitCounts, splits) || release.TaskCount != len(tasks) {
		return errors.New("v3 corpus release declared counts do not match its contents")
	}
	return nil
}

func corpusReleaseDigestV3(release CorpusReleaseV3) (string, error) {
	release.Digest = ""
	return digestJSON(release)
}
