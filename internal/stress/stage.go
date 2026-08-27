package stress

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
)

const stageCausalityBoundary = "earliest observed digest divergence only; no causal attribution beyond the declared controlled transform"

func NewStageRecord(stage Stage, unit string, canonical []byte) (StageRecord, error) {
	if !validStage(stage) || !identifierPattern.MatchString(unit) || len(canonical) == 0 {
		return StageRecord{}, errors.New("stage record requires a valid stage, unit, and non-empty canonical material")
	}
	material := appendStageMaterial(nil, []byte(stage))
	material = appendStageMaterial(material, []byte(unit))
	material = appendStageMaterial(material, canonical)
	digest := sha256.Sum256(material)
	return StageRecord{Stage: stage, Unit: unit, Digest: hex.EncodeToString(digest[:])}, nil
}

func SealStageTrace(side string, records []StageRecord) (StageTrace, error) {
	value := StageTrace{
		SchemaVersion: StageTraceSchemaVersion, CanonicalPolicy: CanonicalPolicy, Side: side,
		Records: append([]StageRecord(nil), records...),
	}
	sortStageRecords(value.Records)
	digest, err := stageTraceDigest(value)
	if err != nil {
		return StageTrace{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return StageTrace{}, err
	}
	return value, nil
}

func (value StageTrace) Validate() error {
	if value.SchemaVersion != StageTraceSchemaVersion || value.CanonicalPolicy != CanonicalPolicy || !identifierPattern.MatchString(value.Side) || len(value.Records) == 0 {
		return errors.New("stress stage trace identity or records are invalid")
	}
	for index, record := range value.Records {
		if !validStage(record.Stage) || !identifierPattern.MatchString(record.Unit) || !validDigest(record.Digest) || stageIndex(record.Stage) != index {
			return errors.New("stress stage trace must be a unique ordered prefix beginning at ingestion")
		}
	}
	expected, err := stageTraceDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress stage trace digest is invalid")
	}
	return nil
}

func CompareStageTraces(relation Relation, left, right StageTrace) (StageComparison, error) {
	if err := relation.Validate(); err != nil {
		return StageComparison{}, fmt.Errorf("validate relation before stage comparison: %w", err)
	}
	if err := left.Validate(); err != nil {
		return StageComparison{}, fmt.Errorf("validate left stage trace: %w", err)
	}
	if err := right.Validate(); err != nil {
		return StageComparison{}, fmt.Errorf("validate right stage trace: %w", err)
	}
	if left.Digest == right.Digest || left.Side == right.Side {
		return StageComparison{}, errors.New("stage comparison requires distinct trace sides and identities")
	}
	expectations := make(map[Stage]StageExpectationKind, len(relation.StageExpectations))
	for _, expectation := range relation.StageExpectations {
		expectations[expectation.Stage] = expectation.Expectation
	}
	leftByStage := stageRecordsByStage(left.Records)
	rightByStage := stageRecordsByStage(right.Records)
	comparison := StageComparison{
		SchemaVersion: StageComparisonSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RelationDigest: relation.Digest, LeftTraceDigest: left.Digest, RightTraceDigest: right.Digest,
		Differences: []StageDifference{}, CausalityClaim: stageCausalityBoundary,
	}
	for _, stage := range orderedStages() {
		leftRecord, hasLeft := leftByStage[stage]
		rightRecord, hasRight := rightByStage[stage]
		if !hasLeft && !hasRight {
			continue
		}
		if hasLeft && hasRight && leftRecord.Unit != rightRecord.Unit {
			return StageComparison{}, fmt.Errorf("stress stage %q compares different canonical units", stage)
		}
		divergent := !hasLeft || !hasRight || leftRecord.Digest != rightRecord.Digest
		expectation := expectations[stage]
		unexpected := expectation == StageMustMatch && divergent || expectation == StageMustDiffer && hasLeft && hasRight && !divergent
		difference := StageDifference{Stage: stage, Expectation: expectation, Divergent: divergent, Unexpected: unexpected}
		if hasLeft {
			difference.LeftDigest = leftRecord.Digest
		}
		if hasRight {
			difference.RightDigest = rightRecord.Digest
		}
		comparison.Differences = append(comparison.Differences, difference)
		if divergent && comparison.EarliestDivergentStage == "" {
			comparison.EarliestDivergentStage = stage
		}
		if unexpected && comparison.EarliestUnexpectedStage == "" {
			comparison.EarliestUnexpectedStage = stage
		}
	}
	digest, err := stageComparisonDigest(comparison)
	if err != nil {
		return StageComparison{}, err
	}
	comparison.Digest = digest
	if err := comparison.Validate(); err != nil {
		return StageComparison{}, err
	}
	return comparison, nil
}

func (value StageComparison) Validate() error {
	if value.SchemaVersion != StageComparisonSchemaVersion || value.CanonicalPolicy != CanonicalPolicy || !validDigest(value.RelationDigest) ||
		!validDigest(value.LeftTraceDigest) || !validDigest(value.RightTraceDigest) || value.LeftTraceDigest == value.RightTraceDigest ||
		len(value.Differences) == 0 || value.CausalityClaim != stageCausalityBoundary {
		return errors.New("stress stage comparison identity or boundary is invalid")
	}
	earliestDivergent, earliestUnexpected := Stage(""), Stage("")
	previous := -1
	for _, difference := range value.Differences {
		index := stageIndex(difference.Stage)
		if index <= previous || !slices.Contains([]StageExpectationKind{StageMustMatch, StageMustDiffer, StageMayDiffer}, difference.Expectation) ||
			(difference.LeftDigest != "" && !validDigest(difference.LeftDigest)) || (difference.RightDigest != "" && !validDigest(difference.RightDigest)) ||
			difference.LeftDigest == "" && difference.RightDigest == "" || difference.Divergent != (difference.LeftDigest == "" || difference.RightDigest == "" || difference.LeftDigest != difference.RightDigest) ||
			difference.Unexpected != (difference.Expectation == StageMustMatch && difference.Divergent || difference.Expectation == StageMustDiffer && difference.LeftDigest != "" && difference.RightDigest != "" && !difference.Divergent) {
			return errors.New("stress stage comparison rows are invalid or out of order")
		}
		previous = index
		if difference.Divergent && earliestDivergent == "" {
			earliestDivergent = difference.Stage
		}
		if difference.Unexpected && earliestUnexpected == "" {
			earliestUnexpected = difference.Stage
		}
	}
	if value.EarliestDivergentStage != earliestDivergent || value.EarliestUnexpectedStage != earliestUnexpected {
		return errors.New("stress stage comparison earliest-divergence summary is invalid")
	}
	expected, err := stageComparisonDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress stage comparison digest is invalid")
	}
	return nil
}

func appendStageMaterial(target, value []byte) []byte {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	target = append(target, length[:]...)
	return append(target, value...)
}

func sortStageRecords(records []StageRecord) {
	slices.SortFunc(records, func(left, right StageRecord) int { return stageIndex(left.Stage) - stageIndex(right.Stage) })
}

func stageRecordsByStage(records []StageRecord) map[Stage]StageRecord {
	result := make(map[Stage]StageRecord, len(records))
	for _, record := range records {
		result[record.Stage] = record
	}
	return result
}

func stageTraceDigest(value StageTrace) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func stageComparisonDigest(value StageComparison) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
