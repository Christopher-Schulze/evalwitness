package relation

import (
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func BuildPilotSample(plan Plan, primary PrimarySample, release mutation.CorpusRelease) (PilotSample, error) {
	if err := plan.Validate(); err != nil {
		return PilotSample{}, err
	}
	if err := primary.Validate(); err != nil {
		return PilotSample{}, err
	}
	if err := release.Validate(); err != nil {
		return PilotSample{}, err
	}
	if primary.PlanDigest != plan.Digest || primary.SourceCorpusDigest != release.Digest || release.Digest != plan.SourceCorpusDigest {
		return PilotSample{}, errors.New("relation pilot plan, primary sample, and release bindings differ")
	}
	if primary.ProtocolVersion != plan.ProtocolVersion {
		return PilotSample{}, errors.New("relation pilot plan and primary sample protocol versions differ")
	}
	if plan.SchemaVersion == PlanSchemaVersionV2 && (primary.SourceCorpusSpecDigest != plan.SourceCorpusSpecDigest ||
		primary.SourceMutationProgramDigest != plan.SourceMutationProgramDigest || primary.SourceConstructAuditDigest != plan.SourceConstructAuditDigest ||
		release.SpecDigest != plan.SourceCorpusSpecDigest || release.MutationProgramDigest != plan.SourceMutationProgramDigest ||
		release.Spec.DevelopmentAudit.ConstructAuditDigest != plan.SourceConstructAuditDigest) {
		return PilotSample{}, errors.New("v2 relation pilot corpus spec, mutation program, or construct audit bindings differ")
	}
	sourceByID := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, source := range release.Sources {
		sourceByID[source.ID] = source
	}
	expectedPrimary, err := BuildPrimarySample(plan, release)
	if err != nil {
		return PilotSample{}, err
	}
	if expectedPrimary.Digest != primary.Digest {
		return PilotSample{}, errors.New("relation pilot primary sample does not reproduce from the bound plan and release")
	}
	primaryCases, err := selectPrimaryCases(plan, release, sourceByID)
	if err != nil {
		return PilotSample{}, err
	}
	primarySources, primaryGroups, primaryLineages := selectionIdentities(primaryCases, sourceByID)
	candidates := make(map[mutation.Family][]mutation.CorpusCase, len(plan.Families))
	for _, item := range release.Cases {
		if item.Split != study.RoleDevelopment || plan.SchemaVersion == PlanSchemaVersionV1 && item.Manifest.Review.Required ||
			intersectsPrimary(item, sourceByID, primarySources, primaryGroups, primaryLineages) {
			continue
		}
		candidates[item.Family] = append(candidates[item.Family], item)
	}
	for family := range candidates {
		sort.Slice(candidates[family], func(left, right int) bool { return candidates[family][left].ID < candidates[family][right].ID })
	}
	selected, ok := selectPilotCases(plan.Families, candidates, sourceByID, 0, map[string]struct{}{}, map[string]struct{}{})
	if !ok || len(selected) != plan.PilotSampleSize {
		return PilotSample{}, errors.New("relation pilot cannot select one non-overlapping development case per governed family")
	}
	rows := map[string][]string{"cases": {}, "sources": {}, "programs": {}, "manifests": {}, "witnesses": {}, "licenses": {}, "privacy": {}, "lineage": {}, "packets": {}, "regeneration": {}, "construct_firewalls": {}}
	references := make([]PilotCaseReference, 0, len(selected))
	uniqueSources, uniqueGroups, uniqueLineages := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, item := range selected {
		linked := make([]mutation.CorpusSource, 0, len(item.SourceIDs))
		lineage := make([]lineageBinding, 0, len(item.SourceIDs))
		lineageIDs := make([]string, 0, len(item.SourceIDs))
		for _, sourceID := range item.SourceIDs {
			source := sourceByID[sourceID]
			linked = append(linked, source)
			uniqueSources[source.ID] = struct{}{}
			uniqueLineages[source.LineageClusterID] = struct{}{}
			lineageIDs = append(lineageIDs, source.LineageClusterID)
			lineage = append(lineage, lineageBinding{RepositoryID: source.RepositoryID, TaskID: source.TaskID, SplitGroupID: source.SplitGroupID, NearDuplicateID: source.NearDuplicateID, LineageClusterID: source.LineageClusterID, PatchDigest: source.PatchDigest})
		}
		lineageIDs = uniqueSorted(lineageIDs)
		bindingDigests, err := deepCaseBindingDigests(item, linked, lineage)
		if err != nil {
			return PilotSample{}, err
		}
		for name, digest := range bindingDigests {
			if name == "construct_firewalls" && plan.SchemaVersion != PlanSchemaVersionV2 {
				continue
			}
			rows[name] = append(rows[name], item.ID+"\x00"+digest)
		}
		definition, _ := mutation.DefinitionFor(item.Family)
		unit := UnitTrajectoryPair
		if definition.PairLevel {
			unit = UnitCandidatePairOrders
		}
		uniqueGroups[item.Manifest.SplitGroupID] = struct{}{}
		references = append(references, PilotCaseReference{
			Family: item.Family, CaseID: item.ID, Unit: unit, TaskGroupID: item.Manifest.SplitGroupID,
			SourceIDs: append([]string(nil), item.SourceIDs...), LineageClusterIDs: lineageIDs, CaseBindingDigest: bindingDigests["cases"],
		})
	}
	for _, values := range rows {
		sort.Strings(values)
	}
	sample := PilotSample{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest,
		PrimarySampleDigest: primary.Digest, SourceCorpusDigest: release.Digest, DataRole: "development", SelectionRule: pilotSampleRule(plan.SchemaVersion),
		SelectedCases: len(references), UniqueSourceIDs: len(uniqueSources), UniqueTaskGroups: len(uniqueGroups), UniqueLineageClusters: len(uniqueLineages), PrimaryOverlap: 0,
		RequiredPrimaryLabels: len(references) * plan.PrimaryReviewers, MaximumTieBreakLabels: len(references) * plan.TieBreakReviewers,
		RequiredPostLabelProbes: len(references) * plan.PrimaryReviewers, Cases: references,
		Bindings: BindingCommitments{
			Cases: aggregate(rows["cases"]), Sources: aggregate(rows["sources"]), Programs: aggregate(rows["programs"]), Manifests: aggregate(rows["manifests"]),
			Witnesses: aggregate(rows["witnesses"]), Licenses: aggregate(rows["licenses"]), Privacy: aggregate(rows["privacy"]), Lineage: aggregate(rows["lineage"]),
			Packets: aggregate(rows["packets"]), Regeneration: aggregate(rows["regeneration"]),
		},
		ExternalActionStatus: ExternalActionNotAuthorized,
	}
	if plan.SchemaVersion == PlanSchemaVersionV2 {
		sample.SourceCorpusSpecDigest = plan.SourceCorpusSpecDigest
		sample.SourceMutationProgramDigest = plan.SourceMutationProgramDigest
		sample.SourceConstructAuditDigest = plan.SourceConstructAuditDigest
		sample.Bindings.ConstructFirewalls = aggregate(rows["construct_firewalls"])
		return SealPilotSampleV2(sample)
	}
	return SealPilotSample(sample)
}

func pilotSampleRule(planSchemaVersion string) string {
	if planSchemaVersion == PlanSchemaVersionV2 {
		return PilotSampleRuleV2
	}
	return PilotSampleRuleV1
}

func SealPilotSample(sample PilotSample) (PilotSample, error) {
	sample.SchemaVersion, sample.CanonicalPolicy, sample.Digest = PilotSampleSchemaVersionV1, CanonicalPolicy, ""
	digest, err := pilotSampleDigest(sample)
	if err != nil {
		return PilotSample{}, err
	}
	sample.Digest = digest
	return sample, sample.Validate()
}

func SealPilotSampleV2(sample PilotSample) (PilotSample, error) {
	sample.SchemaVersion, sample.CanonicalPolicy, sample.Digest = PilotSampleSchemaVersionV2, CanonicalPolicy, ""
	digest, err := pilotSampleDigest(sample)
	if err != nil {
		return PilotSample{}, err
	}
	sample.Digest = digest
	return sample, sample.Validate()
}

func (sample PilotSample) Validate() error {
	v3Adapter := sample.SchemaVersion == PilotSampleSchemaVersionV3Adapter
	expectedCases, expectedSources, expectedGroups, expectedLineages := 8, 8, 8, 8
	expectedPrimaryLabels, expectedTieBreakLabels, expectedProbes := 16, 8, 16
	if v3Adapter {
		expectedCases, expectedSources, expectedGroups, expectedLineages = 7, 8, 7, 7
		expectedPrimaryLabels, expectedTieBreakLabels, expectedProbes = 14, 7, 14
	}
	if sample.CanonicalPolicy != CanonicalPolicy || sample.Objective != ReviewObjectiveControlledRelation || !validDigest(sample.PlanDigest) ||
		!validDigest(sample.PrimarySampleDigest) || !validDigest(sample.SourceCorpusDigest) || sample.DataRole != "development" || sample.SelectedCases != expectedCases || sample.SelectedCases != len(sample.Cases) ||
		sample.UniqueSourceIDs < expectedSources || sample.UniqueTaskGroups != expectedGroups || sample.UniqueLineageClusters != expectedLineages || sample.PrimaryOverlap != 0 ||
		sample.RequiredPrimaryLabels != expectedPrimaryLabels || sample.MaximumTieBreakLabels != expectedTieBreakLabels || sample.RequiredPostLabelProbes != expectedProbes || sample.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation pilot identity, objective, selection, overlap, workload, or authorization boundary is invalid")
	}
	switch sample.SchemaVersion {
	case PilotSampleSchemaVersionV1:
		if sample.ProtocolVersion != ProtocolVersionV1 || sample.SelectionRule != PilotSampleRuleV1 || sample.SourceCorpusSpecDigest != "" ||
			sample.SourceCorpusPlanDigest != "" || sample.SourceMutationProgramDigest != "" || sample.SourceConstructAuditDigest != "" || sample.Bindings.ConstructFirewalls != "" ||
			sample.ScarcitySentinelDigest != "" || sample.EmpiricalStatus != "" {
			return errors.New("v1 relation pilot identity or historical selection contract is invalid")
		}
	case PilotSampleSchemaVersionV2:
		if sample.ProtocolVersion != ProtocolVersionV2 || sample.SelectionRule != PilotSampleRuleV2 || !validDigest(sample.SourceCorpusSpecDigest) ||
			sample.SourceCorpusPlanDigest != "" || !validDigest(sample.SourceMutationProgramDigest) || !validDigest(sample.SourceConstructAuditDigest) || !validDigest(sample.Bindings.ConstructFirewalls) ||
			sample.ScarcitySentinelDigest != "" || sample.EmpiricalStatus != "" {
			return errors.New("v2 relation pilot identity, corpus audit, or construct binding is invalid")
		}
	case PilotSampleSchemaVersionV3Adapter:
		if sample.ProtocolVersion != ProtocolVersionV3 || sample.SelectionRule != PilotSampleRuleV3 || sample.SourceCorpusSpecDigest != "" || !validDigest(sample.SourceCorpusPlanDigest) ||
			!validDigest(sample.SourceMutationProgramDigest) || !validDigest(sample.SourceConstructAuditDigest) || !validDigest(sample.Bindings.ConstructFirewalls) ||
			!validDigest(sample.ScarcitySentinelDigest) || sample.ScarcitySentinelOverlap != 0 || sample.EmpiricalStatus != EmpiricalStatusNotRun {
			return errors.New("v3 relation pilot adapter identity, scarcity, corpus audit, or construct binding is invalid")
		}
	default:
		return errors.New("unknown relation pilot sample schema version")
	}
	seenFamilies, seenCases, seenGroups, seenSources, seenLineages := map[mutation.Family]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for index, item := range sample.Cases {
		definition, exists := mutation.DefinitionFor(item.Family)
		expectedUnit := UnitTrajectoryPair
		if exists && definition.PairLevel {
			expectedUnit = UnitCandidatePairOrders
		}
		if !exists || item.Unit != expectedUnit || !validDigest(item.CaseBindingDigest) || strings.TrimSpace(item.CaseID) == "" || strings.TrimSpace(item.TaskGroupID) == "" ||
			len(item.SourceIDs) == 0 || len(item.LineageClusterIDs) == 0 || index > 0 && sample.Cases[index-1].Family >= item.Family {
			return errors.New("relation pilot case references must cover sorted unique governed families and valid units")
		}
		if _, duplicate := seenFamilies[item.Family]; duplicate {
			return errors.New("relation pilot reuses a family")
		}
		if _, duplicate := seenCases[item.CaseID]; duplicate {
			return errors.New("relation pilot reuses a case")
		}
		if _, duplicate := seenGroups[item.TaskGroupID]; duplicate {
			return errors.New("relation pilot reuses a task group")
		}
		for _, sourceID := range item.SourceIDs {
			if _, duplicate := seenSources[sourceID]; duplicate {
				return errors.New("relation pilot reuses a source")
			}
			seenSources[sourceID] = struct{}{}
		}
		for _, lineageID := range item.LineageClusterIDs {
			if _, duplicate := seenLineages[lineageID]; duplicate {
				return errors.New("relation pilot reuses a lineage cluster")
			}
			seenLineages[lineageID] = struct{}{}
		}
		seenFamilies[item.Family], seenCases[item.CaseID], seenGroups[item.TaskGroupID] = struct{}{}, struct{}{}, struct{}{}
	}
	for _, digest := range []string{sample.Bindings.Cases, sample.Bindings.Sources, sample.Bindings.Programs, sample.Bindings.Manifests, sample.Bindings.Witnesses, sample.Bindings.Licenses, sample.Bindings.Privacy, sample.Bindings.Lineage, sample.Bindings.Packets, sample.Bindings.Regeneration} {
		if !validDigest(digest) {
			return errors.New("relation pilot has an invalid deep binding commitment")
		}
	}
	if !v3Adapter {
		expected, err := pilotSampleDigest(sample)
		if err != nil || sample.Digest != expected {
			return errors.New("relation pilot sample digest is invalid")
		}
	} else if !validDigest(sample.Digest) {
		return errors.New("v3 relation pilot adapter must bind the exact governed pilot digest")
	}
	return nil
}

func intersectsPrimary(item mutation.CorpusCase, sources map[string]mutation.CorpusSource, primarySources, primaryGroups, primaryLineages map[string]struct{}) bool {
	if _, exists := primaryGroups[item.Manifest.SplitGroupID]; exists {
		return true
	}
	for _, sourceID := range item.SourceIDs {
		if _, exists := primarySources[sourceID]; exists {
			return true
		}
		if _, exists := primaryLineages[sources[sourceID].LineageClusterID]; exists {
			return true
		}
	}
	return false
}

func selectPilotCases(contracts []FamilyContract, candidates map[mutation.Family][]mutation.CorpusCase, sources map[string]mutation.CorpusSource, index int, usedGroups, usedLineages map[string]struct{}) ([]mutation.CorpusCase, bool) {
	if index == len(contracts) {
		return []mutation.CorpusCase{}, true
	}
	for _, item := range candidates[contracts[index].Family] {
		if _, used := usedGroups[item.Manifest.SplitGroupID]; used {
			continue
		}
		lineages := uniqueSortedLineages(item, sources)
		if slices.ContainsFunc(lineages, func(value string) bool { _, used := usedLineages[value]; return used }) {
			continue
		}
		usedGroups[item.Manifest.SplitGroupID] = struct{}{}
		for _, lineage := range lineages {
			usedLineages[lineage] = struct{}{}
		}
		rest, ok := selectPilotCases(contracts, candidates, sources, index+1, usedGroups, usedLineages)
		if ok {
			return append([]mutation.CorpusCase{item}, rest...), true
		}
		delete(usedGroups, item.Manifest.SplitGroupID)
		for _, lineage := range lineages {
			delete(usedLineages, lineage)
		}
	}
	return nil, false
}

func uniqueSortedLineages(item mutation.CorpusCase, sources map[string]mutation.CorpusSource) []string {
	values := make([]string, 0, len(item.SourceIDs))
	for _, sourceID := range item.SourceIDs {
		values = append(values, sources[sourceID].LineageClusterID)
	}
	return uniqueSorted(values)
}

func uniqueSorted(values []string) []string {
	slices.Sort(values)
	return slices.Compact(values)
}

func pilotSampleDigest(sample PilotSample) (string, error) {
	sample.Digest = ""
	return digestJSON(sample)
}
