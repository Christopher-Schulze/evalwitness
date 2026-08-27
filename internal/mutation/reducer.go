package mutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const ReductionSchemaVersion = "evalwitness.mutation-reduction.v1"

type ReducedRegion struct {
	EventID              string               `json:"event_id"`
	FieldPath            preprocess.FieldPath `json:"field_path"`
	BeforeStart          int                  `json:"before_start"`
	BeforeEnd            int                  `json:"before_end"`
	AfterStart           int                  `json:"after_start"`
	AfterEnd             int                  `json:"after_end"`
	BeforeFragmentDigest string               `json:"before_fragment_digest"`
	AfterFragmentDigest  string               `json:"after_fragment_digest"`
}

type ReductionWitness struct {
	SchemaVersion  string          `json:"schema_version"`
	MutationDigest string          `json:"mutation_digest"`
	Relation       Relation        `json:"relation"`
	Regions        []ReducedRegion `json:"regions"`
	Digest         string          `json:"digest"`
}

func ReduceChangedRegions(manifest Manifest, original, mutated preprocess.Trajectory) (ReductionWitness, error) {
	if err := ValidateFormalRelation(manifest, original, mutated); err != nil {
		return ReductionWitness{}, err
	}
	regions := make([]ReducedRegion, 0, len(manifest.Affected.FieldPaths))
	for _, fieldPath := range manifest.Affected.FieldPaths {
		eventID, suffix, err := splitEventFieldPath(fieldPath)
		if err != nil {
			return ReductionWitness{}, err
		}
		beforeEvent, exists := eventByID(original.Events, eventID)
		if !exists {
			return ReductionWitness{}, fmt.Errorf("reduction source event %q is missing", eventID)
		}
		afterEvent, exists := eventBySource(mutated.Events, beforeEvent)
		if !exists {
			return ReductionWitness{}, fmt.Errorf("reduction target for event %q is missing", eventID)
		}
		before, err := fieldMaterial(beforeEvent, suffix)
		if err != nil {
			return ReductionWitness{}, err
		}
		after, err := fieldMaterial(afterEvent, suffix)
		if err != nil {
			return ReductionWitness{}, err
		}
		beforeStart, beforeEnd, afterStart, afterEnd := smallestChangedSpan(before, after)
		if beforeStart == beforeEnd && afterStart == afterEnd {
			return ReductionWitness{}, fmt.Errorf("affected field %q has no changed region", fieldPath)
		}
		regions = append(regions, ReducedRegion{
			EventID: eventID, FieldPath: fieldPath,
			BeforeStart: beforeStart, BeforeEnd: beforeEnd, AfterStart: afterStart, AfterEnd: afterEnd,
			BeforeFragmentDigest: digestText(string(before[beforeStart:beforeEnd])),
			AfterFragmentDigest:  digestText(string(after[afterStart:afterEnd])),
		})
	}
	sort.Slice(regions, func(left, right int) bool { return regions[left].FieldPath < regions[right].FieldPath })
	witness := ReductionWitness{SchemaVersion: ReductionSchemaVersion, MutationDigest: manifest.Digest, Relation: manifest.ExpectedRelation, Regions: regions}
	digest, err := reductionDigest(witness)
	if err != nil {
		return ReductionWitness{}, err
	}
	witness.Digest = digest
	return witness, witness.Validate()
}

func (witness ReductionWitness) Validate() error {
	if witness.SchemaVersion != ReductionSchemaVersion || !validDigest(witness.MutationDigest) || !validRelation(witness.Relation) || len(witness.Regions) == 0 {
		return errors.New("mutation reduction identity, relation, or regions are invalid")
	}
	paths := make([]string, len(witness.Regions))
	for index, region := range witness.Regions {
		paths[index] = string(region.FieldPath)
		if missing(region.EventID) || !validDigest(region.BeforeFragmentDigest) || !validDigest(region.AfterFragmentDigest) ||
			region.BeforeStart < 0 || region.BeforeEnd < region.BeforeStart || region.AfterStart < 0 || region.AfterEnd < region.AfterStart ||
			region.BeforeStart == region.BeforeEnd && region.AfterStart == region.AfterEnd {
			return fmt.Errorf("mutation reduction region %d is invalid", index)
		}
	}
	if err := validateUniqueSortedStrings("mutation reduction fields", paths); err != nil {
		return err
	}
	expected, err := reductionDigest(witness)
	if err != nil {
		return err
	}
	if witness.Digest != expected {
		return errors.New("mutation reduction digest is invalid")
	}
	return nil
}

func reductionDigest(witness ReductionWitness) (string, error) {
	witness.Digest = ""
	return digestJSON(witness)
}

func splitEventFieldPath(path preprocess.FieldPath) (string, string, error) {
	remainder := strings.TrimPrefix(string(path), "/events/")
	eventID, suffix, found := strings.Cut(remainder, "/")
	if remainder == string(path) || !found || eventID == "" || suffix == "" {
		return "", "", fmt.Errorf("invalid event field path %q", path)
	}
	return eventID, "/" + suffix, nil
}

func eventByID(events []preprocess.Event, id string) (preprocess.Event, bool) {
	for _, event := range events {
		if event.ID == id {
			return event, true
		}
	}
	return preprocess.Event{}, false
}

func eventBySource(events []preprocess.Event, source preprocess.Event) (preprocess.Event, bool) {
	key := eventSourceKey(source)
	for _, event := range events {
		if eventSourceKey(event) == key {
			return event, true
		}
	}
	return preprocess.Event{}, false
}

func fieldMaterial(event preprocess.Event, suffix string) ([]byte, error) {
	var value any
	switch suffix {
	case "/text":
		value = eventText(event)
	case "/evidence":
		value = eventEvidenceText(event)
	case "/file_change/diff":
		if event.FileChange == nil {
			return nil, errors.New("affected diff field has no file-change payload")
		}
		value = event.FileChange.Diff
	case "/file_change/path_alias":
		if event.FileChange == nil {
			return nil, errors.New("affected path alias has no file-change payload")
		}
		value = event.FileChange.PathAlias
	case "/command/working_directory_alias":
		if event.Command == nil {
			return nil, errors.New("affected working directory has no command payload")
		}
		value = event.Command.WorkingDirectoryAlias
	case "/command/exit_code":
		if event.Command == nil {
			return nil, errors.New("affected exit code has no command payload")
		}
		value = event.Command.ExitCode
	case "/order":
		value = event.Order
	default:
		return nil, fmt.Errorf("unsupported reducible mutation field %q", suffix)
	}
	return json.Marshal(value)
}

func smallestChangedSpan(before, after []byte) (int, int, int, int) {
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	beforeSuffix, afterSuffix := len(before), len(after)
	for beforeSuffix > prefix && afterSuffix > prefix && before[beforeSuffix-1] == after[afterSuffix-1] {
		beforeSuffix--
		afterSuffix--
	}
	return prefix, beforeSuffix, prefix, afterSuffix
}
