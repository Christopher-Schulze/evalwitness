package mutation

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func BuildCorpus(spec CorpusSpec, candidates []SourceCandidate) (CorpusRelease, error) {
	if err := spec.Validate(); err != nil {
		return CorpusRelease{}, err
	}
	selected, err := selectCorpusSources(spec, candidates)
	if err != nil {
		return CorpusRelease{}, err
	}
	lineageSources := make([]CorpusSource, len(selected))
	for index := range selected {
		lineageSources[index] = selected[index].Source
	}
	lineagePlan := corpusLineagePlan(lineageSources, spec.Seed)
	for index := range selected {
		assignment := lineagePlan[selected[index].Source.ID]
		selected[index].Source.LineageClusterID = assignment.ClusterID
		selected[index].Source.Split = assignment.Split
	}
	cases := make([]CorpusCase, 0, len(spec.PrimaryFamilies)*spec.CasesPerFamily)
	constructRejections := make([]ConstructFirewallReport, 0)
	for _, family := range spec.PrimaryFamilies {
		definition, _ := DefinitionFor(family)
		var familyCases []CorpusCase
		var familyRejections []ConstructFirewallReport
		if definition.PairLevel {
			familyCases, familyRejections, err = buildPairCases(spec, selected, family)
		} else {
			familyCases, familyRejections, err = buildTrajectoryCases(spec, selected, family)
		}
		if err != nil {
			return CorpusRelease{}, err
		}
		cases = append(cases, familyCases...)
		constructRejections = append(constructRejections, familyRejections...)
	}
	sort.Slice(cases, func(left, right int) bool { return cases[left].ID < cases[right].ID })
	sort.Slice(constructRejections, func(left, right int) bool {
		return constructRejections[left].Digest < constructRejections[right].Digest
	})
	sources := make([]CorpusSource, len(selected))
	for index, candidate := range selected {
		sources[index] = candidate.Source
	}
	sort.Slice(sources, func(left, right int) bool { return sources[left].ID < sources[right].ID })
	specDigest, err := corpusSpecDigest(spec)
	if err != nil {
		return CorpusRelease{}, err
	}
	positiveControl, err := FormalPositiveControl()
	if err != nil {
		return CorpusRelease{}, err
	}
	release := CorpusRelease{
		CorpusVersion: spec.CorpusVersion, SpecDigest: specDigest, MutationProgramDigest: spec.MutationProgramDigest,
		Spec: spec, Sources: sources, Cases: cases, ConstructRejections: constructRejections, Controls: []CorpusControl{positiveControl}, TaskCount: spec.SourceTasks,
	}
	sourceFamilies, mutationFamilies, splits, controls := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, source := range sources {
		sourceFamilies[source.SourceFamily]++
		splits[string(source.Split)]++
	}
	for _, item := range cases {
		mutationFamilies[string(item.Family)]++
		controls[item.Control]++
	}
	for _, control := range release.Controls {
		controls[control.Kind]++
	}
	release.SourceFamilyCounts = countRecords(sourceFamilies)
	release.MutationFamilyCounts = countRecords(mutationFamilies)
	release.SplitCounts = countRecords(splits)
	release.PositiveControls = controls["positive"]
	release.NegativeControls = controls["negative"]
	release.DecoyControls = controls["decoy"]
	release.AmbiguousCases = controls["ambiguous"]
	return SealCorpusRelease(release)
}

type corpusLineageAssignment struct {
	ClusterID string
	Split     study.DataRole
}

func corpusLineagePlan(sources []CorpusSource, seed string) map[string]corpusLineageAssignment {
	parents := make([]int, len(sources))
	for index := range parents {
		parents[index] = index
	}
	var find func(int) int
	find = func(index int) int {
		if parents[index] != index {
			parents[index] = find(parents[index])
		}
		return parents[index]
	}
	union := func(left, right int) {
		leftRoot, rightRoot := find(left), find(right)
		if leftRoot != rightRoot {
			parents[rightRoot] = leftRoot
		}
	}
	owner := make(map[string]int)
	for index, source := range sources {
		keys := []string{
			"repository:" + source.RepositoryID,
			"task:" + source.RepositoryID + "\x00" + source.TaskID,
			"group:" + source.SplitGroupID,
			"near_duplicate:" + source.NearDuplicateID,
			"trajectory:" + source.TrajectoryDigest,
		}
		if source.PatchDigest != "" {
			keys = append(keys, "patch:"+source.PatchDigest)
		}
		for _, key := range keys {
			if previous, exists := owner[key]; exists {
				union(index, previous)
			} else {
				owner[key] = index
			}
		}
	}
	type lineageCluster struct {
		sourceIDs []string
		taskIDs   map[string]struct{}
	}
	clusters := make(map[int]*lineageCluster)
	for index, source := range sources {
		root := find(index)
		if clusters[root] == nil {
			clusters[root] = &lineageCluster{taskIDs: make(map[string]struct{})}
		}
		clusters[root].sourceIDs = append(clusters[root].sourceIDs, source.ID)
		clusters[root].taskIDs[source.RepositoryID+"\x00"+source.TaskID] = struct{}{}
	}
	ordered := make([]*lineageCluster, 0, len(clusters))
	for _, cluster := range clusters {
		sort.Strings(cluster.sourceIDs)
		ordered = append(ordered, cluster)
	}
	sort.Slice(ordered, func(left, right int) bool {
		leftTasks, rightTasks := len(ordered[left].taskIDs), len(ordered[right].taskIDs)
		if leftTasks != rightTasks {
			return leftTasks > rightTasks
		}
		return digestText(seed+"\x00"+strings.Join(ordered[left].sourceIDs, "\x00")) < digestText(seed+"\x00"+strings.Join(ordered[right].sourceIDs, "\x00"))
	})
	totalTasks := 0
	for _, cluster := range ordered {
		totalTasks += len(cluster.taskIDs)
	}
	targets := map[study.DataRole]int{
		study.RoleDevelopment: totalTasks * 60 / 100,
		study.RoleCalibration: totalTasks * 20 / 100,
	}
	targets[study.RoleTest] = totalTasks - targets[study.RoleDevelopment] - targets[study.RoleCalibration]
	assigned := map[study.DataRole]int{}
	roles := []study.DataRole{study.RoleDevelopment, study.RoleCalibration, study.RoleTest}
	result := make(map[string]corpusLineageAssignment, len(sources))
	for _, cluster := range ordered {
		clusterID := "lineage-" + digestText(strings.Join(cluster.sourceIDs, "\x00"))
		weight := len(cluster.taskIDs)
		role := chooseLineageRole(seed, clusterID, weight, targets, assigned, roles)
		assigned[role] += weight
		assignment := corpusLineageAssignment{ClusterID: clusterID, Split: role}
		for _, sourceID := range cluster.sourceIDs {
			result[sourceID] = assignment
		}
	}
	return result
}

func chooseLineageRole(seed, clusterID string, weight int, targets, assigned map[study.DataRole]int, roles []study.DataRole) study.DataRole {
	best := roles[0]
	bestFits := false
	bestNeed := -1.0
	bestTie := ""
	for _, role := range roles {
		remaining := targets[role] - assigned[role]
		fits := remaining >= weight
		need := -1_000_000.0
		if targets[role] > 0 {
			need = float64(remaining) / float64(targets[role])
		}
		tie := digestText(seed + "\x00" + clusterID + "\x00" + string(role))
		if fits && !bestFits || fits == bestFits && (need > bestNeed || need == bestNeed && tie < bestTie) {
			best, bestFits, bestNeed, bestTie = role, fits, need, tie
		}
	}
	return best
}

func selectCorpusSources(spec CorpusSpec, candidates []SourceCandidate) ([]SourceCandidate, error) {
	groups := make(map[string][]SourceCandidate)
	for _, candidate := range candidates {
		groups[candidate.Source.SplitGroupID] = append(groups[candidate.Source.SplitGroupID], candidate)
	}
	terminalGroups := rankedGroups(spec.Seed+"\x00terminal", groups, func(values []SourceCandidate) bool {
		count := 0
		for _, value := range values {
			if strings.HasPrefix(value.Source.SourceFamily, "terminal-bench-2/") {
				count++
			}
		}
		return count >= spec.TrajectoriesPerTask
	})
	sweGroups := rankedGroups(spec.Seed+"\x00swe", groups, func(values []SourceCandidate) bool {
		families := make(map[string]struct{})
		for _, value := range values {
			if strings.HasPrefix(value.Source.SourceFamily, "swe-bench/") {
				families[value.Source.SourceFamily] = struct{}{}
			}
		}
		return len(families) >= spec.TrajectoriesPerTask
	})
	if len(terminalGroups) < spec.TerminalTasks || len(sweGroups) < spec.SWETasks {
		return nil, fmt.Errorf("corruption source inventory has %d/%d eligible Terminal and %d/%d eligible SWE task groups", len(terminalGroups), spec.TerminalTasks, len(sweGroups), spec.SWETasks)
	}
	selected := make([]SourceCandidate, 0, spec.SourceTasks*spec.TrajectoriesPerTask)
	for _, groupID := range terminalGroups[:spec.TerminalTasks] {
		values := filterRankedCandidates(spec.Seed, groups[groupID], "terminal-bench-2/")
		selected = append(selected, values[:spec.TrajectoriesPerTask]...)
	}
	for _, groupID := range sweGroups[:spec.SWETasks] {
		values := onePerRankedFamily(spec.Seed, groups[groupID], "swe-bench/")
		if len(values) < spec.TrajectoriesPerTask {
			return nil, fmt.Errorf("SWE corruption group %q lost source-family diversity", groupID)
		}
		selected = append(selected, values[:spec.TrajectoriesPerTask]...)
	}
	return selected, nil
}

func rankedGroups(seed string, groups map[string][]SourceCandidate, eligible func([]SourceCandidate) bool) []string {
	var ids []string
	for id, values := range groups {
		if eligible(values) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(left, right int) bool {
		return digestText(seed+"\x00"+ids[left]) < digestText(seed+"\x00"+ids[right])
	})
	return ids
}

func filterRankedCandidates(seed string, candidates []SourceCandidate, familyPrefix string) []SourceCandidate {
	var result []SourceCandidate
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate.Source.SourceFamily, familyPrefix) {
			result = append(result, candidate)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return digestText(seed+"\x00"+result[left].Source.ID) < digestText(seed+"\x00"+result[right].Source.ID)
	})
	return result
}

func onePerRankedFamily(seed string, candidates []SourceCandidate, familyPrefix string) []SourceCandidate {
	byFamily := make(map[string][]SourceCandidate)
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate.Source.SourceFamily, familyPrefix) {
			byFamily[candidate.Source.SourceFamily] = append(byFamily[candidate.Source.SourceFamily], candidate)
		}
	}
	var families []string
	for family := range byFamily {
		families = append(families, family)
	}
	sort.Slice(families, func(left, right int) bool {
		return digestText(seed+"\x00"+families[left]) < digestText(seed+"\x00"+families[right])
	})
	result := make([]SourceCandidate, 0, len(families))
	for _, family := range families {
		values := filterRankedCandidates(seed, byFamily[family], family)
		result = append(result, values[0])
	}
	return result
}

func buildTrajectoryCases(spec CorpusSpec, candidates []SourceCandidate, family Family) ([]CorpusCase, []ConstructFirewallReport, error) {
	ordered := append([]SourceCandidate(nil), candidates...)
	sort.Slice(ordered, func(left, right int) bool {
		return digestText(spec.Seed+"\x00"+string(family)+"\x00"+ordered[left].Source.ID) < digestText(spec.Seed+"\x00"+string(family)+"\x00"+ordered[right].Source.ID)
	})
	result := make([]CorpusCase, 0, spec.CasesPerFamily)
	rejections := make([]ConstructFirewallReport, 0)
	usedGroups := make(map[string]struct{}, spec.CasesPerFamily)
	programVersion, _ := mutationProgramVersion(spec.MutationProgramDigest)
	for _, candidate := range ordered {
		if _, duplicateTask := usedGroups[candidate.Source.SplitGroupID]; duplicateTask {
			continue
		}
		request := corpusApplyRequest(spec, family, candidate.Source)
		var applied ApplyResult
		var firewall *ConstructFirewallReport
		if programVersion == MutationProgramVersionV2 {
			outcome, applyErr := ApplyV2(candidate.Trajectory, request)
			if applyErr != nil {
				return nil, rejections, fmt.Errorf("apply v2 mutation family %q to source %q: %w", family, candidate.Source.ID, applyErr)
			}
			if outcome.Status == ConstructRejected {
				rejections = append(rejections, outcome.Firewall)
				continue
			}
			applied = *outcome.Applied
			firewall = &outcome.Firewall
		} else {
			var applyErr error
			applied, applyErr = Apply(candidate.Trajectory, request)
			if applyErr != nil {
				continue
			}
		}
		reduction, err := ReduceChangedRegions(applied.Manifest, candidate.Trajectory, applied.Mutated)
		if err != nil {
			return nil, nil, err
		}
		usedGroups[candidate.Source.SplitGroupID] = struct{}{}
		if len(result) < spec.CasesPerFamily {
			result = append(result, CorpusCase{
				ID: applied.Manifest.MutationID, SourceIDs: []string{candidate.Source.ID}, Family: family, Split: candidate.Source.Split,
				Control: corpusControl(family), Manifest: applied.Manifest, BlindPacket: applied.Packet, Reduction: &reduction,
				ConstructFirewall: firewall,
				RegenerationKey:   digestText(spec.MutationProgramDigest + "\x00" + candidate.Source.ID + "\x00" + string(family)),
			})
		}
		if programVersion == MutationProgramVersionV1 && len(result) == spec.CasesPerFamily {
			return result, rejections, nil
		}
	}
	if len(result) == spec.CasesPerFamily {
		return result, rejections, nil
	}
	return nil, rejections, fmt.Errorf("mutation family %q produced %d/%d validated corpus cases after %d construct rejections", family, len(result), spec.CasesPerFamily, len(rejections))
}

func buildPairCases(spec CorpusSpec, candidates []SourceCandidate, family Family) ([]CorpusCase, []ConstructFirewallReport, error) {
	groups := make(map[string][]SourceCandidate)
	for _, candidate := range candidates {
		groups[candidate.Source.SplitGroupID] = append(groups[candidate.Source.SplitGroupID], candidate)
	}
	groupIDs := rankedGroups(spec.Seed+"\x00"+string(family), groups, func(values []SourceCandidate) bool { return len(values) >= 2 })
	result := make([]CorpusCase, 0, spec.CasesPerFamily)
	programVersion, _ := mutationProgramVersion(spec.MutationProgramDigest)
	for _, groupID := range groupIDs {
		values := append([]SourceCandidate(nil), groups[groupID]...)
		sort.Slice(values, func(left, right int) bool {
			return digestText(spec.Seed+"\x00"+string(family)+"\x00"+values[left].Source.ID) < digestText(spec.Seed+"\x00"+string(family)+"\x00"+values[right].Source.ID)
		})
		left, right := values[0], values[1]
		request := corpusApplyRequest(spec, family, left.Source)
		request.SourceFamily = "paired/" + left.Source.SourceFamily + "+" + right.Source.SourceFamily
		request.SourceLocation = "pair/" + groupID + "/" + digestText(left.Source.ID+"\x00"+right.Source.ID)
		request.SourceRevision = left.Source.SourceRevision + "+" + right.Source.SourceRevision
		request.Outcome = SourceOutcome{
			Kind: "paired_benchmark_rewards", Value: left.Source.Outcome.Value + "," + right.Source.Outcome.Value,
			WitnessDigest: digestText(left.Source.Outcome.WitnessDigest + "\x00" + right.Source.Outcome.WitnessDigest),
		}
		var applied ApplyResult
		var firewall *ConstructFirewallReport
		if programVersion == MutationProgramVersionV2 {
			outcome, applyErr := ApplyCandidateOrderReversalV2(left.Trajectory, right.Trajectory, request)
			if applyErr != nil {
				return nil, nil, fmt.Errorf("apply v2 pair mutation family %q to group %q: %w", family, groupID, applyErr)
			}
			if outcome.Status != ConstructApplied || outcome.Applied == nil {
				return nil, nil, fmt.Errorf("v2 pair mutation family %q did not produce an applied result", family)
			}
			applied = *outcome.Applied
			firewall = &outcome.Firewall
		} else {
			var applyErr error
			applied, applyErr = ApplyCandidateOrderReversal(left.Trajectory, right.Trajectory, request)
			if applyErr != nil {
				continue
			}
		}
		result = append(result, CorpusCase{
			ID: applied.Manifest.MutationID, SourceIDs: []string{left.Source.ID, right.Source.ID}, Family: family, Split: left.Source.Split,
			Control: corpusControl(family), Manifest: applied.Manifest, BlindPacket: applied.Packet,
			ConstructFirewall: firewall,
			RegenerationKey:   digestText(spec.MutationProgramDigest + "\x00" + left.Source.ID + "\x00" + right.Source.ID + "\x00" + string(family)),
		})
		if len(result) == spec.CasesPerFamily {
			return result, nil, nil
		}
	}
	return nil, nil, fmt.Errorf("pair mutation family %q produced %d/%d validated corpus cases", family, len(result), spec.CasesPerFamily)
}

func corpusApplyRequest(spec CorpusSpec, family Family, source CorpusSource) ApplyRequest {
	definition, _ := DefinitionFor(family)
	validatorKind := ValidationPreservation
	if definition.Class == ClassAdversarialClaim {
		validatorKind = ValidationFormal
	}
	programVersion, _ := mutationProgramVersion(spec.MutationProgramDigest)
	relationContract := RelationContractVersionV1
	if programVersion == MutationProgramVersionV2 {
		relationContract = RelationContractVersionV2
	}
	validator := ValidatorSpec{
		ID: "evalwitness.controlled-relation", Version: relationContract, Kind: validatorKind,
		ContractDigest: spec.MutationProgramDigest, TimeoutMillis: 30_000, MaximumOutputBytes: 1024 * 1024,
	}
	reviewSampled := strings.Compare(digestText(spec.Seed + "\x00review\x00" + source.ID + "\x00" + string(family))[:2], "1a") <= 0
	return ApplyRequest{
		CorpusVersion: spec.CorpusVersion, TaskID: source.TaskID, RepositoryID: source.RepositoryID, SourceFamily: source.SourceFamily,
		SourceLocation: source.SourceLocation, SourceRevision: source.SourceRevision, SplitGroupID: source.SplitGroupID,
		Seed: spec.Seed + "/" + string(family), Family: family, Outcome: source.Outcome, Validator: validator,
		License: source.License, Privacy: source.Privacy, ReviewSampled: reviewSampled,
		ReviewSamplingStratum: "automatic-" + corpusControl(family),
	}
}

func VerifyNoCorpusLeakage(release CorpusRelease) error {
	if err := release.Validate(); err != nil {
		return err
	}
	if release.TaskCount < 40 || len(release.SourceFamilyCounts) < 3 || len(release.MutationFamilyCounts) < 8 {
		return errors.New("corruption corpus is below governed release thresholds")
	}
	return nil
}

func CorpusSplitSummary(release CorpusRelease) map[study.DataRole]int {
	result := make(map[study.DataRole]int)
	for _, source := range release.Sources {
		result[source.Split]++
	}
	return result
}
