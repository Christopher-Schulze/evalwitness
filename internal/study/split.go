package study

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

const splitAlgorithm = "evalwitness.task-group-stratified-hash.v1"

type SplitWeight struct {
	Role   DataRole `json:"role"`
	Weight int      `json:"weight"`
}

type SplitSpec struct {
	Seed                    string        `json:"seed"`
	StratificationVariables []string      `json:"stratification_variables"`
	Weights                 []SplitWeight `json:"weights"`
}

type SplitGroup struct {
	DatasetID                 string       `json:"dataset_id"`
	GroupID                   string       `json:"group_id"`
	Stratum                   []NamedValue `json:"stratum"`
	TaskIDs                   []string     `json:"task_ids"`
	RepositoryIDs             []string     `json:"repository_ids"`
	CloneFamilyIDs            []string     `json:"clone_family_ids"`
	TrajectoryDigests         []string     `json:"trajectory_digests"`
	PairObservationIDs        []string     `json:"pair_observation_ids"`
	MutationDescendantDigests []string     `json:"mutation_descendant_digests"`
	EvidenceDescendantDigests []string     `json:"evidence_descendant_digests"`
	AdjudicationPacketDigests []string     `json:"adjudication_packet_digests"`
	CounterexampleDigests     []string     `json:"counterexample_digests"`
	CorpusVersionIDs          []string     `json:"corpus_version_ids"`
}

func GenerateSplit(spec SplitSpec, groups []SplitGroup) (SplitManifest, error) {
	if strings.TrimSpace(spec.Seed) == "" || len(groups) == 0 || len(spec.Weights) < 2 {
		return SplitManifest{}, errors.New("split seed, groups, and at least two weighted roles are required")
	}
	if err := validateUniqueText("stratification variables", spec.StratificationVariables); err != nil {
		return SplitManifest{}, err
	}
	totalWeight := 0
	maximumInt := int(^uint(0) >> 1)
	roles := make(map[DataRole]struct{}, len(spec.Weights))
	for _, weight := range spec.Weights {
		if !validRole(weight.Role) || weight.Role == RoleUnavailable || weight.Weight <= 0 {
			return SplitManifest{}, errors.New("split weights require unique usable roles and positive integers")
		}
		if _, duplicate := roles[weight.Role]; duplicate {
			return SplitManifest{}, fmt.Errorf("duplicate split role %q", weight.Role)
		}
		if totalWeight > maximumInt-weight.Weight || len(groups) > maximumInt/weight.Weight {
			return SplitManifest{}, errors.New("split weights exceed safe integer arithmetic")
		}
		roles[weight.Role] = struct{}{}
		totalWeight += weight.Weight
	}
	groupIDs := make(map[string]struct{}, len(groups))
	buckets := make(map[string][]SplitGroup)
	for index, group := range groups {
		if err := validateSplitGroup(group, spec.StratificationVariables); err != nil {
			return SplitManifest{}, fmt.Errorf("group %d: %w", index, err)
		}
		if _, duplicate := groupIDs[group.GroupID]; duplicate {
			return SplitManifest{}, fmt.Errorf("duplicate split group %q", group.GroupID)
		}
		groupIDs[group.GroupID] = struct{}{}
		bucketKey := string(canonicalIdentityBytes(group.DatasetID, stratumKey(group.Stratum)))
		buckets[bucketKey] = append(buckets[bucketKey], group)
	}
	bucketKeys := make([]string, 0, len(buckets))
	for key := range buckets {
		bucketKeys = append(bucketKeys, key)
	}
	sort.Strings(bucketKeys)
	manifest := SplitManifest{
		SchemaVersion: SplitSchemaVersion, Algorithm: splitAlgorithm, Seed: spec.Seed,
		StratificationVariables: append([]string{}, spec.StratificationVariables...),
	}
	for _, key := range bucketKeys {
		bucket := buckets[key]
		sort.Slice(bucket, func(left, right int) bool {
			leftHash := splitHash(spec.Seed, key, bucket[left].GroupID)
			rightHash := splitHash(spec.Seed, key, bucket[right].GroupID)
			if leftHash == rightHash {
				return bucket[left].GroupID < bucket[right].GroupID
			}
			return leftHash < rightHash
		})
		counts := allocateSplitCounts(len(bucket), spec.Weights, totalWeight)
		cursor := 0
		for weightIndex, weight := range spec.Weights {
			for _, group := range bucket[cursor : cursor+counts[weightIndex]] {
				manifest.Assignments = append(manifest.Assignments, assignmentFromGroup(group, weight.Role))
			}
			cursor += counts[weightIndex]
		}
	}
	sort.Slice(manifest.Assignments, func(left, right int) bool {
		return manifest.Assignments[left].GroupID < manifest.Assignments[right].GroupID
	})
	digest, err := manifest.digest()
	if err != nil {
		return SplitManifest{}, err
	}
	manifest.Digest = digest
	if err := manifest.Validate(); err != nil {
		return SplitManifest{}, err
	}
	return manifest, nil
}

func (manifest SplitManifest) Validate() error {
	if manifest.SchemaVersion != SplitSchemaVersion || manifest.Algorithm != splitAlgorithm || strings.TrimSpace(manifest.Seed) == "" {
		return errors.New("split schema, algorithm, or seed is invalid")
	}
	if len(manifest.Assignments) == 0 {
		return errors.New("split has no assignments")
	}
	if err := validateUniqueText("stratification variables", manifest.StratificationVariables); err != nil {
		return err
	}
	expected, err := manifest.digest()
	if err != nil {
		return err
	}
	if manifest.Digest != expected {
		return errors.New("split digest is invalid")
	}
	groupIDs := make(map[string]DataRole, len(manifest.Assignments))
	identities := make(map[string]struct {
		role  DataRole
		group string
	})
	for index, assignment := range manifest.Assignments {
		if !validRole(assignment.Split) || assignment.Split == RoleUnavailable || missing(assignment.DatasetID, assignment.GroupID) || len(assignment.TaskIDs) == 0 {
			return fmt.Errorf("assignment %d has invalid dataset, group, role, or task set", index)
		}
		if index > 0 && manifest.Assignments[index-1].GroupID >= assignment.GroupID {
			return errors.New("split assignments must be strictly sorted by group ID")
		}
		if err := validateStratum(assignment.Stratum, manifest.StratificationVariables); err != nil {
			return fmt.Errorf("assignment %q: %w", assignment.GroupID, err)
		}
		if prior, duplicate := groupIDs[assignment.GroupID]; duplicate {
			return fmt.Errorf("group %q appears in both %s and %s", assignment.GroupID, prior, assignment.Split)
		}
		groupIDs[assignment.GroupID] = assignment.Split
		collections := []struct {
			name   string
			values []string
		}{
			{name: "task", values: assignment.TaskIDs},
			{name: "repository", values: assignment.RepositoryIDs},
			{name: "clone family", values: assignment.CloneFamilyIDs},
			{name: "trajectory", values: assignment.TrajectoryDigests},
			{name: "pair observation", values: assignment.PairObservationIDs},
			{name: "mutation descendant", values: assignment.MutationDescendantDigests},
			{name: "evidence descendant", values: assignment.EvidenceDescendantDigests},
			{name: "adjudication packet", values: assignment.AdjudicationPacketDigests},
			{name: "counterexample", values: assignment.CounterexampleDigests},
			{name: "corpus version", values: assignment.CorpusVersionIDs},
		}
		for _, collection := range collections {
			if err := validateUniqueText(collection.name+" identities", collection.values); err != nil {
				return fmt.Errorf("group %q: %w", assignment.GroupID, err)
			}
			if !slices.IsSorted(collection.values) {
				return fmt.Errorf("group %q %s identities are not canonically sorted", assignment.GroupID, collection.name)
			}
			for _, value := range collection.values {
				key := collection.name + "\x00" + value
				if prior, exists := identities[key]; exists && prior.group != assignment.GroupID {
					return fmt.Errorf("%s %q leaks across group %q (%s) and %q (%s)", collection.name, value, prior.group, prior.role, assignment.GroupID, assignment.Split)
				}
				identities[key] = struct {
					role  DataRole
					group string
				}{role: assignment.Split, group: assignment.GroupID}
			}
		}
		for _, collection := range []struct {
			name   string
			values []string
		}{
			{name: "trajectory", values: assignment.TrajectoryDigests},
			{name: "mutation descendant", values: assignment.MutationDescendantDigests},
			{name: "evidence descendant", values: assignment.EvidenceDescendantDigests},
			{name: "adjudication packet", values: assignment.AdjudicationPacketDigests},
			{name: "counterexample", values: assignment.CounterexampleDigests},
		} {
			for _, value := range collection.values {
				if !validDigest(value) {
					return fmt.Errorf("group %q %s digest %q is not SHA-256", assignment.GroupID, collection.name, value)
				}
			}
		}
	}
	return nil
}

func validateSplitGroup(group SplitGroup, variables []string) error {
	if missing(group.DatasetID, group.GroupID) || len(group.TaskIDs) == 0 {
		return errors.New("dataset ID, group ID, and task IDs are required")
	}
	if err := validateStratum(group.Stratum, variables); err != nil {
		return err
	}
	collections := [][]string{
		group.TaskIDs, group.RepositoryIDs, group.CloneFamilyIDs, group.TrajectoryDigests, group.PairObservationIDs,
		group.MutationDescendantDigests, group.EvidenceDescendantDigests,
		group.AdjudicationPacketDigests, group.CounterexampleDigests, group.CorpusVersionIDs,
	}
	for _, values := range collections {
		if err := validateUniqueText("split group", values); err != nil {
			return err
		}
	}
	return nil
}

func validateStratum(stratum []NamedValue, variables []string) error {
	if len(stratum) != len(variables) {
		return errors.New("group stratum does not match declared variables")
	}
	for index, variable := range variables {
		if stratum[index].Name != variable || strings.TrimSpace(stratum[index].Value) == "" {
			return errors.New("group stratum must use declared variable order with non-empty values")
		}
	}
	return nil
}

func allocateSplitCounts(size int, weights []SplitWeight, totalWeight int) []int {
	counts := make([]int, len(weights))
	type remainder struct {
		index int
		value int
	}
	remainders := make([]remainder, len(weights))
	allocated := 0
	for index, weight := range weights {
		product := size * weight.Weight
		counts[index] = product / totalWeight
		allocated += counts[index]
		remainders[index] = remainder{index: index, value: product % totalWeight}
	}
	sort.SliceStable(remainders, func(left, right int) bool {
		return remainders[left].value > remainders[right].value
	})
	for index := 0; index < size-allocated; index++ {
		counts[remainders[index].index]++
	}
	return counts
}

func assignmentFromGroup(group SplitGroup, role DataRole) SplitAssignment {
	return SplitAssignment{
		DatasetID: group.DatasetID, GroupID: group.GroupID, Split: role, Stratum: append([]NamedValue(nil), group.Stratum...),
		TaskIDs: sortedCopy(group.TaskIDs), RepositoryIDs: sortedCopy(group.RepositoryIDs),
		CloneFamilyIDs: sortedCopy(group.CloneFamilyIDs), TrajectoryDigests: sortedCopy(group.TrajectoryDigests),
		PairObservationIDs:        sortedCopy(group.PairObservationIDs),
		MutationDescendantDigests: sortedCopy(group.MutationDescendantDigests),
		EvidenceDescendantDigests: sortedCopy(group.EvidenceDescendantDigests),
		AdjudicationPacketDigests: sortedCopy(group.AdjudicationPacketDigests),
		CounterexampleDigests:     sortedCopy(group.CounterexampleDigests), CorpusVersionIDs: sortedCopy(group.CorpusVersionIDs),
	}
}

func (manifest SplitManifest) digest() (string, error) {
	manifest.Digest = ""
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode split manifest: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func stratumKey(values []NamedValue) string {
	parts := make([]string, 0, len(values)*2)
	for _, value := range values {
		parts = append(parts, value.Name, value.Value)
	}
	return string(canonicalIdentityBytes(parts...))
}

func splitHash(seed, stratum, groupID string) uint64 {
	digest := sha256.Sum256(canonicalIdentityBytes(seed, stratum, groupID))
	return binary.BigEndian.Uint64(digest[:8])
}

func canonicalIdentityBytes(values ...string) []byte {
	var encoded bytes.Buffer
	var length [8]byte
	for _, value := range values {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		encoded.Write(length[:])
		encoded.WriteString(value)
	}
	return encoded.Bytes()
}

func sortedCopy(values []string) []string {
	copyValues := append([]string{}, values...)
	sort.Strings(copyValues)
	return copyValues
}
