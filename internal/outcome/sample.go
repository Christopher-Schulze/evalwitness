package outcome

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const SampleCommitmentSchemaVersion = "evalwitness.outcome-sample-commitment.v1"

type SampleCount struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type SampleCommitment struct {
	SchemaVersion      string        `json:"schema_version"`
	CanonicalPolicy    string        `json:"canonical_policy"`
	PlanDigest         string        `json:"plan_digest"`
	SourceCorpusDigest string        `json:"source_corpus_digest"`
	SelectionRule      string        `json:"selection_rule"`
	SelectedCases      int           `json:"selected_cases"`
	SelectionDigest    string        `json:"selection_digest"`
	FamilyCounts       []SampleCount `json:"family_counts"`
	SplitCounts        []SampleCount `json:"split_counts"`
	ControlCounts      []SampleCount `json:"control_counts"`
	Digest             string        `json:"digest"`
}

func BuildMutationSampleCommitment(plan Plan, release mutation.CorpusRelease) (SampleCommitment, error) {
	if err := plan.Validate(); err != nil {
		return SampleCommitment{}, err
	}
	if err := release.Validate(); err != nil {
		return SampleCommitment{}, err
	}
	if release.Digest != plan.SourceCorpusDigest {
		return SampleCommitment{}, errors.New("outcome plan and controlled-corruption release digests differ")
	}
	var identities []string
	families, splits, controls := map[string]int{}, map[string]int{}, map[string]int{}
	for _, item := range release.Cases {
		if !item.Manifest.Review.Required {
			continue
		}
		identities = append(identities, item.ID+"\x00"+item.BlindPacket.Digest)
		families[string(item.Family)]++
		splits[string(item.Split)]++
		controls[item.Control]++
	}
	sort.Strings(identities)
	if len(identities) != plan.MutationSampleSize {
		return SampleCommitment{}, fmt.Errorf("controlled-corruption review sample has %d cases, plan requires %d", len(identities), plan.MutationSampleSize)
	}
	for _, family := range plan.MutationFamilies {
		if families[family] == 0 {
			return SampleCommitment{}, fmt.Errorf("controlled-corruption review sample omits family %q", family)
		}
	}
	commitment := SampleCommitment{
		PlanDigest: plan.Digest, SourceCorpusDigest: release.Digest,
		SelectionRule: "all cases whose frozen mutation manifest has review.required=true; mapping remains private until labels are sealed",
		SelectedCases: len(identities), SelectionDigest: digestText(joinWithNUL(identities)),
		FamilyCounts: sampleCounts(families), SplitCounts: sampleCounts(splits), ControlCounts: sampleCounts(controls),
	}
	return SealSampleCommitment(commitment)
}

func SealSampleCommitment(commitment SampleCommitment) (SampleCommitment, error) {
	commitment.SchemaVersion = SampleCommitmentSchemaVersion
	commitment.CanonicalPolicy = CanonicalPolicy
	commitment.Digest = ""
	digest, err := sampleCommitmentDigest(commitment)
	if err != nil {
		return SampleCommitment{}, err
	}
	commitment.Digest = digest
	return commitment, commitment.Validate()
}

func (commitment SampleCommitment) Validate() error {
	if commitment.SchemaVersion != SampleCommitmentSchemaVersion || commitment.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(commitment.PlanDigest) || !validDigest(commitment.SourceCorpusDigest) || !validDigest(commitment.SelectionDigest) ||
		missing(commitment.SelectionRule) || commitment.SelectedCases < 1 {
		return errors.New("outcome sample commitment identity, selection, or count is invalid")
	}
	for name, counts := range map[string][]SampleCount{"family": commitment.FamilyCounts, "split": commitment.SplitCounts, "control": commitment.ControlCounts} {
		total := 0
		for index, count := range counts {
			if missing(count.ID) || count.Count < 1 || index > 0 && counts[index-1].ID >= count.ID {
				return fmt.Errorf("outcome sample %s counts must be positive, unique, and sorted", name)
			}
			total += count.Count
		}
		if total != commitment.SelectedCases {
			return fmt.Errorf("outcome sample %s count denominator does not match selected cases", name)
		}
	}
	if len(commitment.FamilyCounts) < 8 || !slices.ContainsFunc(commitment.ControlCounts, func(count SampleCount) bool { return count.ID == "negative" }) ||
		!slices.ContainsFunc(commitment.ControlCounts, func(count SampleCount) bool { return count.ID == "decoy" }) {
		return errors.New("outcome mutation sample lacks family coverage, negative controls, or decoys")
	}
	expected, err := sampleCommitmentDigest(commitment)
	if err != nil || commitment.Digest != expected {
		return errors.New("outcome sample commitment digest is invalid")
	}
	return nil
}

func sampleCounts(values map[string]int) []SampleCount {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]SampleCount, 0, len(keys))
	for _, key := range keys {
		result = append(result, SampleCount{ID: key, Count: values[key]})
	}
	return result
}

func joinWithNUL(values []string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += "\x00"
		}
		result += value
	}
	return result
}

func sampleCommitmentDigest(value SampleCommitment) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
