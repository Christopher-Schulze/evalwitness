package study

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// IdenticalResponseInventorySchemaVersion is the frozen schema identity for the
// TASK 070 eligible development inventory. It enumerates the unique source task
// groups admitted to the prospective identical-response study before any
// provider response is observed.
const IdenticalResponseInventorySchemaVersion = "evalwitness.identical-response-eligible-inventory.v1"

// governedReleaseSchemaVersion and governedReleaseCorpusVersion pin the exact
// frozen controlled-corruption release the inventory is derived from.
const (
	governedReleaseSchemaVersion = "evalwitness.corruption-corpus-release.v3"
	governedReleaseCorpusVersion = "evalwitness-controlled-corruption.v3"
	governedReleaseRelativePath  = "eval/governance/controlled-corruption-v3-release.json"
)

// IdenticalResponseMinimumTaskGroups mirrors the frozen design floor; the
// inventory is honest about whether the governed corpus reaches it.
const IdenticalResponseMinimumTaskGroups = 40

// IdenticalResponseCount is a deterministic count row for role, license, and
// redistribution-class denominators.
type IdenticalResponseCount struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// IdenticalResponseEligibleGroup is one unique source task group (unit) with
// its frozen license, redistribution, digest, outcome, and contamination
// metadata. All digest lists are sorted for canonical serialization.
type IdenticalResponseEligibleGroup struct {
	TaskGroupID           string   `json:"task_group_id"`
	DataRole              string   `json:"data_role"`
	SourceFamilies        []string `json:"source_families"`
	LicenseSPDX           string   `json:"license_spdx"`
	RedistributionClass   string   `json:"redistribution_class"`
	SourceDigests         []string `json:"source_digests"`
	TrajectoryDigests     []string `json:"trajectory_digests"`
	OutcomeAvailable      bool     `json:"outcome_available"`
	OutcomeWitnessDigests []string `json:"outcome_witness_digests"`
	NearDuplicateID       string   `json:"near_duplicate_id"`
	LineageClusterID      string   `json:"lineage_cluster_id"`
	Exclusions            []string `json:"exclusions"`
}

// IdenticalResponseClaimBoundary records exactly what the frozen inventory may
// and may not be used to claim.
type IdenticalResponseClaimBoundary struct {
	ProviderCalls      int      `json:"provider_calls"`
	AgentLaunches      int      `json:"agent_launches"`
	LiveResponseAccess bool     `json:"live_response_access"`
	SupportedClaim     string   `json:"supported_claim"`
	UnsupportedClaims  []string `json:"unsupported_claims"`
}

// IdenticalResponseEligibleInventory is the frozen pre-response eligible
// development inventory for TASK 070.
type IdenticalResponseEligibleInventory struct {
	SchemaVersion            string                           `json:"schema_version"`
	SourceReleaseVersion     string                           `json:"source_release_version"`
	SourceReleaseDigest      string                           `json:"source_release_digest"`
	MinimumTaskGroups        int                              `json:"minimum_task_groups"`
	EligibleTaskGroups       int                              `json:"eligible_task_groups"`
	TaskGroupsByRole         []IdenticalResponseCount         `json:"task_groups_by_role"`
	Licenses                 []IdenticalResponseCount         `json:"licenses"`
	RedistributionClasses    []IdenticalResponseCount         `json:"redistribution_classes"`
	ExclusionRules           []string                         `json:"exclusion_rules"`
	ContaminationBoundary    string                           `json:"contamination_boundary"`
	RedistributionAuthorized bool                             `json:"redistribution_authorized"`
	ClaimBoundary            IdenticalResponseClaimBoundary   `json:"claim_boundary"`
	Groups                   []IdenticalResponseEligibleGroup `json:"groups"`
	Digest                   string                           `json:"digest"`
}

// identicalResponseReleaseSource is the minimal source projection read from the
// frozen governed release. It avoids importing the mutation package so the
// study package stays free of that dependency edge.
type identicalResponseReleaseSource struct {
	SplitGroupID     string `json:"split_group_id"`
	SourceFamily     string `json:"source_family"`
	SourceDigest     string `json:"source_digest"`
	TrajectoryDigest string `json:"trajectory_digest"`
	Split            string `json:"split"`
	NearDuplicateID  string `json:"near_duplicate_id"`
	LineageClusterID string `json:"lineage_cluster_id"`
	License          struct {
		SPDX           string `json:"spdx"`
		Redistribution string `json:"redistribution"`
	} `json:"license"`
	Outcome struct {
		Kind          string `json:"kind"`
		Value         string `json:"value"`
		WitnessDigest string `json:"witness_digest"`
	} `json:"outcome"`
}

type identicalResponseRelease struct {
	SchemaVersion string                           `json:"schema_version"`
	CorpusVersion string                           `json:"corpus_version"`
	Digest        string                           `json:"digest"`
	TaskCount     int                              `json:"task_count"`
	Sources       []identicalResponseReleaseSource `json:"sources"`
}

// BuildIdenticalResponseEligibleInventory derives the frozen eligible
// development inventory from the committed governed controlled-corruption
// release. It performs zero provider calls and zero agent launches.
func BuildIdenticalResponseEligibleInventory(repositoryRoot string) (IdenticalResponseEligibleInventory, error) {
	release, err := readGovernedRelease(repositoryRoot)
	if err != nil {
		return IdenticalResponseEligibleInventory{}, err
	}
	grouped, err := groupEligibleReleaseSources(release.Sources)
	if err != nil {
		return IdenticalResponseEligibleInventory{}, err
	}
	inventory := IdenticalResponseEligibleInventory{
		SchemaVersion:            IdenticalResponseInventorySchemaVersion,
		SourceReleaseVersion:     release.CorpusVersion,
		SourceReleaseDigest:      release.Digest,
		MinimumTaskGroups:        IdenticalResponseMinimumTaskGroups,
		EligibleTaskGroups:       len(grouped),
		ExclusionRules:           []string{},
		ContaminationBoundary:    "upstream trajectory bodies are reference_only and never re-published; the prospective study re-runs extracted task prompts and captures fresh provider responses whose redistribution basis is a separate verification",
		RedistributionAuthorized: false,
		ClaimBoundary: IdenticalResponseClaimBoundary{
			ProviderCalls:      0,
			AgentLaunches:      0,
			LiveResponseAccess: false,
			SupportedClaim:     "the governed source corpus contributes 100 unique source task groups (60 development, 20 calibration, 20 test) with pinned Apache-2.0/MIT licenses and reference-only redistribution class, so task prompts can be extracted and re-run but upstream trajectory bodies cannot be republished",
			UnsupportedClaims: []string{
				"exact public redistribution of upstream trajectory bodies",
				"provider response redistribution basis",
				"confirmatory use of previously accessed benchmark tasks",
				"live provider response access",
			},
		},
	}
	groupIDs := make([]string, 0, len(grouped))
	for groupID := range grouped {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Strings(groupIDs)
	roleCounts := map[string]int{}
	licenseCounts := map[string]int{}
	redistributionCounts := map[string]int{}
	for _, groupID := range groupIDs {
		group := grouped[groupID]
		roleCounts[group.DataRole]++
		licenseCounts[group.LicenseSPDX]++
		redistributionCounts[group.RedistributionClass]++
		inventory.Groups = append(inventory.Groups, group)
	}
	inventory.TaskGroupsByRole = countRows(roleCounts)
	inventory.Licenses = countRows(licenseCounts)
	inventory.RedistributionClasses = countRows(redistributionCounts)
	inventory.Digest, err = identicalResponseInventoryDigest(inventory)
	if err != nil {
		return IdenticalResponseEligibleInventory{}, err
	}
	if err := inventory.Validate(); err != nil {
		return IdenticalResponseEligibleInventory{}, err
	}
	return inventory, nil
}

func readGovernedRelease(repositoryRoot string) (identicalResponseRelease, error) {
	releasePath := filepath.Join(repositoryRoot, filepath.FromSlash(governedReleaseRelativePath))
	raw, err := os.ReadFile(releasePath)
	if err != nil {
		return identicalResponseRelease{}, fmt.Errorf("read governed release: %w", err)
	}
	var release identicalResponseRelease
	if err := json.Unmarshal(raw, &release); err != nil {
		return identicalResponseRelease{}, fmt.Errorf("decode governed release: %w", err)
	}
	if release.SchemaVersion != governedReleaseSchemaVersion || release.CorpusVersion != governedReleaseCorpusVersion {
		return identicalResponseRelease{}, errors.New("governed release schema or corpus version drifted")
	}
	if release.TaskCount < IdenticalResponseMinimumTaskGroups || len(release.Sources) != release.TaskCount*2 || !validDigest(release.Digest) {
		return identicalResponseRelease{}, errors.New("governed release task count, source cardinality, or digest is invalid")
	}
	return release, nil
}

func groupEligibleReleaseSources(sources []identicalResponseReleaseSource) (map[string]IdenticalResponseEligibleGroup, error) {
	byGroup := make(map[string][]identicalResponseReleaseSource, len(sources)/2)
	for _, source := range sources {
		if missing(source.SplitGroupID, source.SourceFamily, source.SourceDigest, source.TrajectoryDigest, source.Split, source.NearDuplicateID, source.LineageClusterID) ||
			!validDigest(source.SourceDigest) || !validDigest(source.TrajectoryDigest) || !validDigest(source.Outcome.WitnessDigest) ||
			!strings.HasPrefix(source.SplitGroupID, "group-") || !strings.HasPrefix(source.NearDuplicateID, "near-") || !strings.HasPrefix(source.LineageClusterID, "lineage-") {
			return nil, fmt.Errorf("governed source identity or digest is incomplete")
		}
		if !slices.Contains([]string{string(RoleDevelopment), string(RoleCalibration), string(RoleTest)}, source.Split) {
			return nil, fmt.Errorf("governed source %q has an unsupported data role", source.SplitGroupID)
		}
		if source.Outcome.Kind != "benchmark_reward" || missing(source.Outcome.Value) {
			return nil, fmt.Errorf("governed source %q has no available benchmark outcome", source.SplitGroupID)
		}
		byGroup[source.SplitGroupID] = append(byGroup[source.SplitGroupID], source)
	}
	result := make(map[string]IdenticalResponseEligibleGroup, len(byGroup))
	for groupID, members := range byGroup {
		if len(members) != 2 {
			return nil, fmt.Errorf("governed task group %q has %d sources, expected 2", groupID, len(members))
		}
		first := members[0]
		sourceFamilies := map[string]struct{}{}
		sourceDigests := make([]string, 0, len(members))
		trajectoryDigests := make([]string, 0, len(members))
		witnessDigests := make([]string, 0, len(members))
		for _, member := range members {
			if member.Split != first.Split || member.NearDuplicateID != first.NearDuplicateID || member.LineageClusterID != first.LineageClusterID ||
				member.License.SPDX != first.License.SPDX || member.License.Redistribution != first.License.Redistribution {
				return nil, fmt.Errorf("governed task group %q crosses a role, contamination, license, or redistribution boundary", groupID)
			}
			sourceFamilies[member.SourceFamily] = struct{}{}
			sourceDigests = append(sourceDigests, member.SourceDigest)
			trajectoryDigests = append(trajectoryDigests, member.TrajectoryDigest)
			witnessDigests = append(witnessDigests, member.Outcome.WitnessDigest)
		}
		families := make([]string, 0, len(sourceFamilies))
		for family := range sourceFamilies {
			families = append(families, family)
		}
		sort.Strings(families)
		sort.Strings(sourceDigests)
		sort.Strings(trajectoryDigests)
		sort.Strings(witnessDigests)
		result[groupID] = IdenticalResponseEligibleGroup{
			TaskGroupID:           groupID,
			DataRole:              first.Split,
			SourceFamilies:        families,
			LicenseSPDX:           first.License.SPDX,
			RedistributionClass:   first.License.Redistribution,
			SourceDigests:         sourceDigests,
			TrajectoryDigests:     trajectoryDigests,
			OutcomeAvailable:      true,
			OutcomeWitnessDigests: witnessDigests,
			NearDuplicateID:       first.NearDuplicateID,
			LineageClusterID:      first.LineageClusterID,
			Exclusions:            []string{},
		}
	}
	return result, nil
}

// Validate enforces the frozen inventory invariants so the committed artifact
// cannot silently drift from the governed corpus.
func (inventory IdenticalResponseEligibleInventory) Validate() error {
	if inventory.SchemaVersion != IdenticalResponseInventorySchemaVersion || inventory.SourceReleaseVersion != governedReleaseCorpusVersion ||
		!validDigest(inventory.SourceReleaseDigest) || !validDigest(inventory.Digest) {
		return errors.New("identical-response eligible inventory identity or release binding is invalid")
	}
	if inventory.MinimumTaskGroups != IdenticalResponseMinimumTaskGroups || inventory.EligibleTaskGroups < IdenticalResponseMinimumTaskGroups ||
		inventory.EligibleTaskGroups != len(inventory.Groups) {
		return errors.New("identical-response eligible inventory does not satisfy the 40 source-task-group floor")
	}
	if inventory.RedistributionAuthorized {
		return errors.New("identical-response eligible inventory must not authorize redistribution of reference-only upstream sources")
	}
	if !countRowsEqual(inventory.TaskGroupsByRole, map[string]int{"development": 60, "calibration": 20, "test": 20}) {
		return errors.New("identical-response eligible inventory role denominators changed")
	}
	if !countRowsEqual(inventory.Licenses, map[string]int{"Apache-2.0": 60, "MIT": 40}) {
		return errors.New("identical-response eligible inventory license denominators changed")
	}
	if !countRowsEqual(inventory.RedistributionClasses, map[string]int{"reference_only": 100}) {
		return errors.New("identical-response eligible inventory redistribution denominators changed")
	}
	if inventory.ClaimBoundary.ProviderCalls != 0 || inventory.ClaimBoundary.AgentLaunches != 0 || inventory.ClaimBoundary.LiveResponseAccess ||
		missing(inventory.ClaimBoundary.SupportedClaim, inventory.ContaminationBoundary) || len(inventory.ClaimBoundary.UnsupportedClaims) == 0 {
		return errors.New("identical-response eligible inventory claim boundary or contamination boundary is invalid")
	}
	if err := validateUniqueText("identical-response unsupported claims", inventory.ClaimBoundary.UnsupportedClaims); err != nil {
		return err
	}
	if len(inventory.Groups) == 0 {
		return errors.New("identical-response eligible inventory has no task groups")
	}
	previous := ""
	seenGroups := make(map[string]struct{}, len(inventory.Groups))
	for _, group := range inventory.Groups {
		if !strings.HasPrefix(group.TaskGroupID, "group-") || group.TaskGroupID <= previous {
			return errors.New("identical-response eligible groups are invalid or unsorted")
		}
		if _, duplicate := seenGroups[group.TaskGroupID]; duplicate {
			return errors.New("identical-response eligible inventory has a duplicate task group")
		}
		seenGroups[group.TaskGroupID] = struct{}{}
		previous = group.TaskGroupID
		if !slices.Contains([]string{string(RoleDevelopment), string(RoleCalibration), string(RoleTest)}, group.DataRole) {
			return fmt.Errorf("identical-response task group %q has an invalid data role", group.TaskGroupID)
		}
		if len(group.SourceFamilies) < 1 || len(group.SourceDigests) != 2 || len(group.TrajectoryDigests) != 2 || len(group.OutcomeWitnessDigests) != 2 ||
			!group.OutcomeAvailable || !validPrefixedDigest(group.NearDuplicateID, "near-") || !validPrefixedDigest(group.LineageClusterID, "lineage-") ||
			!strings.HasPrefix(group.TaskGroupID, "group-") || !validPrefixedDigest(group.TaskGroupID, "group-") {
			return fmt.Errorf("identical-response task group %q metadata is incomplete", group.TaskGroupID)
		}
		if !slices.Contains([]string{"reference_only"}, group.RedistributionClass) || missing(group.LicenseSPDX) {
			return fmt.Errorf("identical-response task group %q license or redistribution class is invalid", group.TaskGroupID)
		}
		if !sortedUnique(group.SourceFamilies) || !sortedUnique(group.SourceDigests) || !sortedUnique(group.TrajectoryDigests) || !sortedUnique(group.OutcomeWitnessDigests) {
			return fmt.Errorf("identical-response task group %q digest lists are unsorted or duplicated", group.TaskGroupID)
		}
		for _, digest := range append(append(append([]string{}, group.SourceDigests...), group.TrajectoryDigests...), group.OutcomeWitnessDigests...) {
			if !validDigest(digest) {
				return fmt.Errorf("identical-response task group %q has an invalid digest", group.TaskGroupID)
			}
		}
	}
	expected, err := identicalResponseInventoryDigest(inventory)
	if err != nil {
		return err
	}
	if inventory.Digest != expected {
		return errors.New("identical-response eligible inventory digest is invalid")
	}
	return nil
}

func countRows(values map[string]int) []IdenticalResponseCount {
	rows := make([]IdenticalResponseCount, 0, len(values))
	for id, count := range values {
		rows = append(rows, IdenticalResponseCount{ID: id, Count: count})
	}
	sort.Slice(rows, func(left, right int) bool { return rows[left].ID < rows[right].ID })
	return rows
}

func countRowsEqual(rows []IdenticalResponseCount, expected map[string]int) bool {
	if len(rows) != len(expected) {
		return false
	}
	for _, row := range rows {
		if expected[row.ID] != row.Count {
			return false
		}
	}
	return true
}

func sortedUnique(values []string) bool {
	if len(values) == 0 || !slices.IsSorted(values) {
		return false
	}
	for index := 1; index < len(values); index++ {
		if values[index-1] == values[index] {
			return false
		}
	}
	return true
}

func validPrefixedDigest(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && validDigest(strings.TrimPrefix(value, prefix))
}

func identicalResponseInventoryDigest(inventory IdenticalResponseEligibleInventory) (string, error) {
	inventory.Digest = ""
	encoded, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
