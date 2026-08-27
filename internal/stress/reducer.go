package stress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"slices"
)

const MaximumReductionUnits = 4096

type ReducibleInput interface {
	CanonicalBytes() ([]byte, error)
	Units() []ReductionUnit
	Remove(ReductionUnit) (ReducibleInput, error)
}

type ReductionOracle interface {
	Evaluate(context.Context, ReducibleInput) (ReductionObservation, error)
}

type ReductionRequest struct {
	RelationDigest       string
	SourceResultDigest   string
	CaseID               string
	PrivacyPolicyDigest  string
	PublicReleaseAllowed bool
	Input                ReducibleInput
	Oracle               ReductionOracle
	MaximumEvaluations   int
}

func SealReductionObservation(value ReductionObservation) (ReductionObservation, error) {
	value.SchemaVersion = ReductionObservationSchemaVersion
	value.CanonicalPolicy = CanonicalPolicy
	value.Digest = ""
	digest, err := reductionObservationDigest(value)
	if err != nil {
		return ReductionObservation{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return ReductionObservation{}, err
	}
	return value, nil
}

func (value ReductionObservation) Validate() error {
	if value.SchemaVersion != ReductionObservationSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.RelationDigest) || !validDigest(value.PrivacyPolicyDigest) || !validDigest(value.RelationProofDigest) ||
		!validDigest(value.PrivacyProofDigest) || !validDigest(value.ReplayResultDigest) {
		return errors.New("stress reduction observation identity or proof digests are invalid")
	}
	expected, err := reductionObservationDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress reduction observation digest is invalid")
	}
	return nil
}

func ReduceCounterexample(ctx context.Context, request ReductionRequest) (Counterexample, error) {
	units, err := validateReductionRequest(request)
	if err != nil {
		return Counterexample{}, err
	}
	originalDigest, err := reducibleInputDigest(request.Input)
	if err != nil {
		return Counterexample{}, err
	}
	if err := ctx.Err(); err != nil {
		return Counterexample{}, err
	}
	originalObservation, err := request.Oracle.Evaluate(ctx, request.Input)
	if err != nil {
		return Counterexample{}, fmt.Errorf("evaluate original counterexample: %w", err)
	}
	originalAfterOracle, err := reducibleInputDigest(request.Input)
	if err != nil {
		return Counterexample{}, err
	}
	if originalAfterOracle != originalDigest {
		return Counterexample{}, errors.New("stress reduction oracle mutated the original input")
	}
	if err := validatePreservingObservation(request, originalObservation); err != nil {
		return Counterexample{}, fmt.Errorf("original input is not an admissible relation violation: %w", err)
	}

	current, currentDigest := request.Input, originalDigest
	steps := []ReductionStep{}
	evaluations := 0
	for {
		accepted := false
		currentUnits := canonicalReductionUnits(current.Units())
		for _, unit := range currentUnits {
			if err := ctx.Err(); err != nil {
				return Counterexample{}, err
			}
			if evaluations >= request.MaximumEvaluations {
				return Counterexample{}, errors.New("stress reduction exhausted its prevalidated complete evaluation budget")
			}
			candidate, removeErr := current.Remove(unit)
			if removeErr != nil {
				return Counterexample{}, fmt.Errorf("remove stress reduction unit %s/%s: %w", unit.Kind, unit.ID, removeErr)
			}
			if isNilInterface(candidate) {
				return Counterexample{}, fmt.Errorf("remove stress reduction unit %s/%s returned no candidate", unit.Kind, unit.ID)
			}
			retainedDigest, retainedErr := reducibleInputDigest(current)
			if retainedErr != nil {
				return Counterexample{}, retainedErr
			}
			if retainedDigest != currentDigest {
				return Counterexample{}, errors.New("stress reducer mutated the retained input while constructing a candidate")
			}
			if err := validateSingleRemoval(currentUnits, unit, candidate.Units()); err != nil {
				return Counterexample{}, err
			}
			candidateDigest, digestErr := reducibleInputDigest(candidate)
			if digestErr != nil {
				return Counterexample{}, digestErr
			}
			if candidateDigest == currentDigest {
				return Counterexample{}, fmt.Errorf("removing stress reduction unit %s/%s did not change canonical input", unit.Kind, unit.ID)
			}
			observation, observeErr := request.Oracle.Evaluate(ctx, candidate)
			if observeErr != nil {
				return Counterexample{}, fmt.Errorf("evaluate stress reduction unit %s/%s: %w", unit.Kind, unit.ID, observeErr)
			}
			retainedAfterOracle, retainedErr := reducibleInputDigest(current)
			if retainedErr != nil {
				return Counterexample{}, retainedErr
			}
			candidateAfterOracle, candidateErr := reducibleInputDigest(candidate)
			if candidateErr != nil {
				return Counterexample{}, candidateErr
			}
			if retainedAfterOracle != currentDigest || candidateAfterOracle != candidateDigest {
				return Counterexample{}, errors.New("stress reduction oracle mutated a retained or candidate input")
			}
			if err := observation.Validate(); err != nil {
				return Counterexample{}, err
			}
			if observation.RelationDigest != request.RelationDigest || observation.PrivacyPolicyDigest != request.PrivacyPolicyDigest {
				return Counterexample{}, errors.New("stress reduction observation belongs to another relation or privacy policy")
			}
			evaluations++
			preserved := observation.RelationRevalidated && observation.PrivacyRevalidated && observation.ViolationPreserved
			step := ReductionStep{
				Index: len(steps), UnitKind: unit.Kind, UnitID: unit.ID, BeforeDigest: currentDigest,
				CandidateDigest: candidateDigest, AfterDigest: currentDigest,
				Decision: ReductionRejected, Observation: observation,
			}
			if preserved {
				step.AfterDigest, step.Decision = candidateDigest, ReductionAccepted
				current, currentDigest, accepted = candidate, candidateDigest, true
			}
			steps = append(steps, step)
			if accepted {
				break
			}
		}
		if !accepted {
			break
		}
	}
	return SealCounterexample(Counterexample{
		RelationDigest: request.RelationDigest, SourceResultDigest: request.SourceResultDigest, CaseID: request.CaseID,
		OriginalInputDigest: originalDigest, ReducedInputDigest: currentDigest, PrivacyPolicyDigest: request.PrivacyPolicyDigest,
		PublicReleaseAllowed: request.PublicReleaseAllowed, Algorithm: deterministicRestartGreedy, Minimality: ReductionOneMinimal,
		OriginalUnits: units, FinalUnits: canonicalReductionUnits(current.Units()), OriginalObservation: originalObservation,
		Steps: steps,
	})
}

func validateReductionRequest(request ReductionRequest) ([]ReductionUnit, error) {
	if !validDigest(request.RelationDigest) || !validDigest(request.SourceResultDigest) || !identifierPattern.MatchString(request.CaseID) ||
		!validDigest(request.PrivacyPolicyDigest) || isNilInterface(request.Input) || isNilInterface(request.Oracle) {
		return nil, errors.New("stress reduction request identity, input, or oracle is invalid")
	}
	units := canonicalReductionUnits(request.Input.Units())
	if err := validateReductionUnits(units, true); err != nil {
		return nil, err
	}
	if len(units) > MaximumReductionUnits {
		return nil, fmt.Errorf("stress reduction units %d exceed maximum %d", len(units), MaximumReductionUnits)
	}
	worstEvaluations := len(units) * (len(units) + 1) / 2
	if request.MaximumEvaluations < worstEvaluations {
		return nil, fmt.Errorf("stress reduction maximum evaluations %d cannot guarantee the complete one-minimal search requiring %d", request.MaximumEvaluations, worstEvaluations)
	}
	return units, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func validatePreservingObservation(request ReductionRequest, value ReductionObservation) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if value.RelationDigest != request.RelationDigest || value.PrivacyPolicyDigest != request.PrivacyPolicyDigest {
		return errors.New("observation belongs to another relation or privacy policy")
	}
	if !value.RelationRevalidated || !value.PrivacyRevalidated || !value.ViolationPreserved {
		return errors.New("relation, privacy, and violation must all hold")
	}
	return nil
}

func validateSingleRemoval(before []ReductionUnit, removed ReductionUnit, after []ReductionUnit) error {
	canonicalAfter := canonicalReductionUnits(after)
	if err := validateReductionUnits(canonicalAfter, false); err != nil {
		return err
	}
	if len(canonicalAfter) != len(before)-1 || slices.Contains(canonicalAfter, removed) {
		return errors.New("stress reducer did not remove exactly the selected unit")
	}
	expected := slices.DeleteFunc(slices.Clone(before), func(value ReductionUnit) bool { return value == removed })
	if !slices.Equal(expected, canonicalAfter) {
		return errors.New("stress reducer changed units outside the selected removal")
	}
	return nil
}

func canonicalReductionUnits(values []ReductionUnit) []ReductionUnit {
	result := slices.Clone(values)
	slices.SortFunc(result, compareReductionUnits)
	return result
}

func reducibleInputDigest(value ReducibleInput) (string, error) {
	encoded, err := value.CanonicalBytes()
	if err != nil {
		return "", fmt.Errorf("encode reducible stress input: %w", err)
	}
	if len(encoded) == 0 {
		return "", errors.New("reducible stress input canonical bytes are empty")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func reductionObservationDigest(value ReductionObservation) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
