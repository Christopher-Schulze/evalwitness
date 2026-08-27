package relation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

type lineageBinding struct {
	RepositoryID     string `json:"repository_id"`
	TaskID           string `json:"task_id"`
	SplitGroupID     string `json:"split_group_id"`
	NearDuplicateID  string `json:"near_duplicate_id"`
	LineageClusterID string `json:"lineage_cluster_id"`
	PatchDigest      string `json:"patch_digest"`
}

func BuildPrimarySample(plan Plan, release mutation.CorpusRelease) (PrimarySample, error) {
	if err := plan.Validate(); err != nil {
		return PrimarySample{}, err
	}
	if err := release.Validate(); err != nil {
		return PrimarySample{}, err
	}
	if release.Digest != plan.SourceCorpusDigest || release.CorpusVersion != plan.SourceCorpusVersion {
		return PrimarySample{}, errors.New("relation plan and controlled-corruption release identity differ")
	}
	if plan.SchemaVersion == PlanSchemaVersionV2 && (release.SpecDigest != plan.SourceCorpusSpecDigest ||
		release.MutationProgramDigest != plan.SourceMutationProgramDigest ||
		release.Spec.DevelopmentAudit.ConstructAuditDigest != plan.SourceConstructAuditDigest) {
		return PrimarySample{}, errors.New("v2 relation plan and corpus spec, mutation program, or construct audit bindings differ")
	}
	sourceByID := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, source := range release.Sources {
		sourceByID[source.ID] = source
	}
	selectedCases, err := selectPrimaryCases(plan, release, sourceByID)
	if err != nil {
		return PrimarySample{}, err
	}
	rows := map[string][]string{"cases": {}, "sources": {}, "programs": {}, "manifests": {}, "witnesses": {}, "licenses": {}, "privacy": {}, "lineage": {}, "packets": {}, "regeneration": {}, "construct_firewalls": {}}
	families, splits, controls, sourceFormats := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	selectedIdentities := make([]string, 0, plan.PrimarySampleSize)
	uniqueSources, uniqueGroups, uniqueLineages := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	candidateOrders := 0
	for _, item := range selectedCases {
		linked := make([]mutation.CorpusSource, 0, len(item.SourceIDs))
		lineage := make([]lineageBinding, 0, len(item.SourceIDs))
		for _, sourceID := range item.SourceIDs {
			source, exists := sourceByID[sourceID]
			if !exists {
				return PrimarySample{}, fmt.Errorf("relation sample case %q references unknown source %q", item.ID, sourceID)
			}
			linked = append(linked, source)
			if _, exists := uniqueSources[source.ID]; !exists {
				sourceFormats[string(source.SourceFormat)]++
			}
			uniqueSources[source.ID] = struct{}{}
			uniqueLineages[source.LineageClusterID] = struct{}{}
			lineage = append(lineage, lineageBinding{RepositoryID: source.RepositoryID, TaskID: source.TaskID, SplitGroupID: source.SplitGroupID, NearDuplicateID: source.NearDuplicateID, LineageClusterID: source.LineageClusterID, PatchDigest: source.PatchDigest})
		}
		bindingDigests, err := deepCaseBindingDigests(item, linked, lineage)
		if err != nil {
			return PrimarySample{}, err
		}
		for name, digest := range bindingDigests {
			if name == "construct_firewalls" && plan.SchemaVersion != PlanSchemaVersionV2 {
				continue
			}
			rows[name] = append(rows[name], item.ID+"\x00"+digest)
		}
		selectedIdentities = append(selectedIdentities, item.ID+"\x00"+item.BlindPacket.Digest)
		families[string(item.Family)]++
		splits[string(item.Split)]++
		controls[item.Control]++
		uniqueGroups[item.Manifest.SplitGroupID] = struct{}{}
		if item.Family == mutation.FamilyCandidateOrderReversal {
			candidateOrders++
		}
	}
	if len(selectedIdentities) != plan.PrimarySampleSize {
		return PrimarySample{}, fmt.Errorf("relation primary sample has %d cases, plan requires %d", len(selectedIdentities), plan.PrimarySampleSize)
	}
	for _, contract := range plan.Families {
		if families[string(contract.Family)] == 0 {
			return PrimarySample{}, fmt.Errorf("relation primary sample omits family %q", contract.Family)
		}
	}
	for _, values := range rows {
		sort.Strings(values)
	}
	sort.Strings(selectedIdentities)
	sample := PrimarySample{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest, SourceCorpusDigest: release.Digest,
		SelectionRule: plan.PrimarySampleRule, SelectedCases: len(selectedIdentities), UniqueSourceIDs: len(uniqueSources), UniqueTaskGroups: len(uniqueGroups),
		TrajectoryPairUnits: len(selectedIdentities) - candidateOrders, CandidateOrderUnits: candidateOrders,
		SelectionDigest: digestText(strings.Join(selectedIdentities, "\x00")), FamilyCounts: counts(families), SplitCounts: counts(splits), ControlCounts: counts(controls),
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
		sample.UniqueLineageClusters = len(uniqueLineages)
		sample.SourceFormatCounts = counts(sourceFormats)
		sample.Bindings.ConstructFirewalls = aggregate(rows["construct_firewalls"])
		return SealPrimarySampleV2(sample)
	}
	return SealPrimarySample(sample)
}

func SealPrimarySample(sample PrimarySample) (PrimarySample, error) {
	sample.SchemaVersion, sample.CanonicalPolicy, sample.Digest = PrimarySampleSchemaVersionV1, CanonicalPolicy, ""
	digest, err := primarySampleDigest(sample)
	if err != nil {
		return PrimarySample{}, err
	}
	sample.Digest = digest
	return sample, sample.Validate()
}

func SealPrimarySampleV2(sample PrimarySample) (PrimarySample, error) {
	sample.SchemaVersion, sample.CanonicalPolicy, sample.Digest = PrimarySampleSchemaVersionV2, CanonicalPolicy, ""
	digest, err := primarySampleDigest(sample)
	if err != nil {
		return PrimarySample{}, err
	}
	sample.Digest = digest
	return sample, sample.Validate()
}

func (sample PrimarySample) Validate() error {
	if sample.CanonicalPolicy != CanonicalPolicy || sample.Objective != ReviewObjectiveControlledRelation || !validDigest(sample.PlanDigest) ||
		!validDigest(sample.SourceCorpusDigest) || sample.UniqueSourceIDs < 1 || sample.UniqueTaskGroups < 1 || sample.TrajectoryPairUnits+sample.CandidateOrderUnits != sample.SelectedCases ||
		sample.CandidateOrderUnits < 1 || !validDigest(sample.SelectionDigest) || sample.ExternalActionStatus != ExternalActionNotAuthorized {
		return errors.New("relation primary sample identity, objective, selection, units, or authorization boundary is invalid")
	}
	switch sample.SchemaVersion {
	case PrimarySampleSchemaVersionV1:
		if sample.ProtocolVersion != ProtocolVersionV1 || sample.SelectionRule != PrimarySampleRuleV1 || sample.SelectedCases != 31 ||
			sample.SourceCorpusSpecDigest != "" || sample.SourceMutationProgramDigest != "" || sample.SourceConstructAuditDigest != "" ||
			sample.UniqueLineageClusters != 0 || len(sample.SourceFormatCounts) != 0 || sample.Bindings.ConstructFirewalls != "" {
			return errors.New("v1 relation primary sample identity or historical selection contract is invalid")
		}
	case PrimarySampleSchemaVersionV2:
		if sample.ProtocolVersion != ProtocolVersionV2 || sample.SelectionRule != PrimarySampleRuleV2 || sample.SelectedCases != 32 ||
			sample.UniqueTaskGroups != 32 || sample.UniqueLineageClusters < 1 || !validDigest(sample.SourceCorpusSpecDigest) ||
			!validDigest(sample.SourceMutationProgramDigest) || !validDigest(sample.SourceConstructAuditDigest) || !validDigest(sample.Bindings.ConstructFirewalls) {
			return errors.New("v2 relation primary sample identity, balance, independence, or construct binding is invalid")
		}
	default:
		return errors.New("unknown relation primary sample schema version")
	}
	for name, values := range map[string][]Count{"family": sample.FamilyCounts, "split": sample.SplitCounts, "control": sample.ControlCounts} {
		if len(values) == 0 {
			return fmt.Errorf("relation primary sample %s counts are empty", name)
		}
		total := 0
		for index, value := range values {
			if strings.TrimSpace(value.ID) == "" || value.Count < 1 || index > 0 && values[index-1].ID >= value.ID {
				return fmt.Errorf("relation primary sample %s counts must be positive, unique, and sorted", name)
			}
			total += value.Count
		}
		if total != sample.SelectedCases {
			return fmt.Errorf("relation primary sample %s denominator is incomplete", name)
		}
	}
	if len(sample.FamilyCounts) != 8 || sample.CandidateOrderUnits != countFor(sample.FamilyCounts, string(mutation.FamilyCandidateOrderReversal)) {
		return errors.New("relation primary sample family or unit coverage is incomplete")
	}
	if sample.SchemaVersion == PrimarySampleSchemaVersionV2 {
		for _, value := range sample.FamilyCounts {
			if value.Count != 4 {
				return errors.New("v2 relation primary sample must contain exactly four cases per family")
			}
		}
		if len(sample.SplitCounts) != 2 || countFor(sample.SplitCounts, string(study.RoleCalibration)) != 16 || countFor(sample.SplitCounts, string(study.RoleTest)) != 16 {
			return errors.New("v2 relation primary sample must contain sixteen calibration and sixteen test cases")
		}
		if err := validateCounts("source format", sample.SourceFormatCounts, sample.UniqueSourceIDs); err != nil {
			return err
		}
	}
	for name, digest := range map[string]string{
		"cases": sample.Bindings.Cases, "sources": sample.Bindings.Sources, "programs": sample.Bindings.Programs, "manifests": sample.Bindings.Manifests,
		"witnesses": sample.Bindings.Witnesses, "licenses": sample.Bindings.Licenses, "privacy": sample.Bindings.Privacy, "lineage": sample.Bindings.Lineage,
		"packets": sample.Bindings.Packets, "regeneration": sample.Bindings.Regeneration,
	} {
		if !validDigest(digest) {
			return fmt.Errorf("relation primary sample %s binding is invalid", name)
		}
	}
	expected, err := primarySampleDigest(sample)
	if err != nil || sample.Digest != expected {
		return errors.New("relation primary sample digest is invalid")
	}
	return nil
}

type primarySelectionBucket struct {
	Family     mutation.Family
	Split      study.DataRole
	Candidates []mutation.CorpusCase
}

func selectPrimaryCases(plan Plan, release mutation.CorpusRelease, sources map[string]mutation.CorpusSource) ([]mutation.CorpusCase, error) {
	if plan.SchemaVersion == PlanSchemaVersionV1 {
		selected := make([]mutation.CorpusCase, 0, plan.PrimarySampleSize)
		for _, item := range release.Cases {
			if item.Manifest.Review.Required {
				selected = append(selected, item)
			}
		}
		return selected, nil
	}
	if plan.SchemaVersion != PlanSchemaVersionV2 {
		return nil, errors.New("unknown relation plan schema version")
	}
	buckets := make([]primarySelectionBucket, 0, len(plan.Families)*2)
	for _, contract := range plan.Families {
		for _, split := range []study.DataRole{study.RoleCalibration, study.RoleTest} {
			bucket := primarySelectionBucket{Family: contract.Family, Split: split}
			for _, item := range release.Cases {
				if item.Family == contract.Family && item.Split == split {
					bucket.Candidates = append(bucket.Candidates, item)
				}
			}
			slices.SortFunc(bucket.Candidates, func(left, right mutation.CorpusCase) int { return strings.Compare(left.ID, right.ID) })
			if len(bucket.Candidates) < 2 {
				return nil, fmt.Errorf("v2 relation primary sample lacks two %s cases for family %q", split, contract.Family)
			}
			buckets = append(buckets, bucket)
		}
	}
	selected, ok := selectPrimaryBuckets(plan, release, sources, buckets, 0, nil, map[string]struct{}{})
	if !ok {
		return nil, errors.New("v2 relation sample cannot satisfy balanced primary and non-overlapping pilot contracts jointly")
	}
	return selected, nil
}

func selectPrimaryBuckets(plan Plan, release mutation.CorpusRelease, sources map[string]mutation.CorpusSource, buckets []primarySelectionBucket, index int, selected []mutation.CorpusCase, usedGroups map[string]struct{}) ([]mutation.CorpusCase, bool) {
	if index == len(buckets) {
		if primarySelectionPreservesPilot(plan, release, sources, selected) {
			return append([]mutation.CorpusCase(nil), selected...), true
		}
		return nil, false
	}
	candidates := buckets[index].Candidates
	for left := 0; left < len(candidates)-1; left++ {
		leftGroup := candidates[left].Manifest.SplitGroupID
		if _, used := usedGroups[leftGroup]; used {
			continue
		}
		usedGroups[leftGroup] = struct{}{}
		for right := left + 1; right < len(candidates); right++ {
			rightGroup := candidates[right].Manifest.SplitGroupID
			if _, used := usedGroups[rightGroup]; used {
				continue
			}
			usedGroups[rightGroup] = struct{}{}
			selected = append(selected, candidates[left], candidates[right])
			if result, ok := selectPrimaryBuckets(plan, release, sources, buckets, index+1, selected, usedGroups); ok {
				return result, true
			}
			selected = selected[:len(selected)-2]
			delete(usedGroups, rightGroup)
		}
		delete(usedGroups, leftGroup)
	}
	return nil, false
}

func primarySelectionPreservesPilot(plan Plan, release mutation.CorpusRelease, sources map[string]mutation.CorpusSource, selected []mutation.CorpusCase) bool {
	primarySources, primaryGroups, primaryLineages := selectionIdentities(selected, sources)
	candidates := make(map[mutation.Family][]mutation.CorpusCase, len(plan.Families))
	for _, item := range release.Cases {
		if item.Split != study.RoleDevelopment || intersectsPrimary(item, sources, primarySources, primaryGroups, primaryLineages) {
			continue
		}
		candidates[item.Family] = append(candidates[item.Family], item)
	}
	for family := range candidates {
		slices.SortFunc(candidates[family], func(left, right mutation.CorpusCase) int { return strings.Compare(left.ID, right.ID) })
	}
	pilot, ok := selectPilotCases(plan.Families, candidates, sources, 0, map[string]struct{}{}, map[string]struct{}{})
	return ok && len(pilot) == plan.PilotSampleSize
}

func selectionIdentities(selected []mutation.CorpusCase, sources map[string]mutation.CorpusSource) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	selectedSources, selectedGroups, selectedLineages := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, item := range selected {
		selectedGroups[item.Manifest.SplitGroupID] = struct{}{}
		for _, sourceID := range item.SourceIDs {
			selectedSources[sourceID] = struct{}{}
			selectedLineages[sources[sourceID].LineageClusterID] = struct{}{}
		}
	}
	return selectedSources, selectedGroups, selectedLineages
}

func validateCounts(name string, values []Count, expected int) error {
	if len(values) == 0 {
		return fmt.Errorf("relation %s counts are empty", name)
	}
	total := 0
	for index, value := range values {
		if strings.TrimSpace(value.ID) == "" || value.Count < 1 || index > 0 && values[index-1].ID >= value.ID {
			return fmt.Errorf("relation %s counts must be positive, unique, and sorted", name)
		}
		total += value.Count
	}
	if total != expected {
		return fmt.Errorf("relation %s denominator is incomplete", name)
	}
	return nil
}

func counts(values map[string]int) []Count {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]Count, 0, len(keys))
	for _, key := range keys {
		result = append(result, Count{ID: key, Count: values[key]})
	}
	return result
}

func countFor(values []Count, id string) int {
	for _, value := range values {
		if value.ID == id {
			return value.Count
		}
	}
	return 0
}

func aggregate(rows []string) string {
	return digestText(strings.Join(rows, "\x00"))
}

func primarySampleDigest(sample PrimarySample) (string, error) {
	sample.Digest = ""
	return digestJSON(sample)
}

func deepCaseBindingDigests(item mutation.CorpusCase, sources []mutation.CorpusSource, lineage []lineageBinding) (map[string]string, error) {
	caseBinding := struct {
		ID               string            `json:"id"`
		Family           mutation.Family   `json:"family"`
		Split            string            `json:"split"`
		Control          string            `json:"control"`
		ExpectedRelation mutation.Relation `json:"expected_relation"`
		SourceIDs        []string          `json:"source_ids"`
		ManifestDigest   string            `json:"manifest_digest"`
		PacketDigest     string            `json:"packet_digest"`
		RegenerationKey  string            `json:"regeneration_key"`
	}{item.ID, item.Family, string(item.Split), item.Control, item.Manifest.ExpectedRelation, item.SourceIDs, item.Manifest.Digest, item.BlindPacket.Digest, item.RegenerationKey}
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
