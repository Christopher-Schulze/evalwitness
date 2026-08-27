package stress

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const deterministicRestartGreedy = "evalwitness.deterministic-restart-greedy-reducer.v1"

func SealCounterexample(value Counterexample) (Counterexample, error) {
	value.SchemaVersion = CounterexampleSchemaVersion
	value.CanonicalPolicy = CanonicalPolicy
	value.Steps = append([]ReductionStep(nil), value.Steps...)
	value.OriginalUnits = append([]ReductionUnit(nil), value.OriginalUnits...)
	value.FinalUnits = append([]ReductionUnit(nil), value.FinalUnits...)
	slices.SortFunc(value.OriginalUnits, compareReductionUnits)
	slices.SortFunc(value.FinalUnits, compareReductionUnits)
	value.AcceptedReductions = 0
	for _, step := range value.Steps {
		if step.Decision == ReductionAccepted {
			value.AcceptedReductions++
		}
	}
	value.Digest = ""
	digest, err := counterexampleDigest(value)
	if err != nil {
		return Counterexample{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return Counterexample{}, err
	}
	return value, nil
}

func (value Counterexample) Validate() error {
	if value.SchemaVersion != CounterexampleSchemaVersion || value.CanonicalPolicy != CanonicalPolicy || !validDigest(value.RelationDigest) ||
		!validDigest(value.SourceResultDigest) || !identifierPattern.MatchString(value.CaseID) || !validDigest(value.OriginalInputDigest) ||
		!validDigest(value.ReducedInputDigest) || !validDigest(value.PrivacyPolicyDigest) || value.Algorithm != deterministicRestartGreedy ||
		value.Minimality != ReductionOneMinimal || len(value.Steps) == 0 || value.AcceptedReductions < 0 {
		return errors.New("stress counterexample identity, privacy, or reduction trace is invalid")
	}
	if err := validateBoundReductionObservation(value.OriginalObservation, value.RelationDigest, value.PrivacyPolicyDigest); err != nil ||
		!value.OriginalObservation.RelationRevalidated || !value.OriginalObservation.PrivacyRevalidated || !value.OriginalObservation.ViolationPreserved {
		return errors.New("stress counterexample original observation does not prove an admissible relation violation")
	}
	if err := validateReductionUnits(value.OriginalUnits, true); err != nil {
		return err
	}
	if err := validateReductionUnits(value.FinalUnits, false); err != nil {
		return err
	}
	current := value.OriginalInputDigest
	accepted := 0
	retained := make(map[ReductionUnit]struct{}, len(value.OriginalUnits))
	for _, unit := range value.OriginalUnits {
		retained[unit] = struct{}{}
	}
	for index, step := range value.Steps {
		if step.Index != index || strings.TrimSpace(step.UnitKind) == "" || step.UnitKind != strings.TrimSpace(step.UnitKind) ||
			strings.TrimSpace(step.UnitID) == "" || step.UnitID != strings.TrimSpace(step.UnitID) || step.BeforeDigest != current ||
			!validDigest(step.CandidateDigest) || step.CandidateDigest == step.BeforeDigest || !validDigest(step.AfterDigest) {
			return fmt.Errorf("stress counterexample reduction step %d identity or digest chain is invalid", index)
		}
		if err := validateBoundReductionObservation(step.Observation, value.RelationDigest, value.PrivacyPolicyDigest); err != nil {
			return fmt.Errorf("stress counterexample reduction step %d observation is invalid: %w", index, err)
		}
		unit := ReductionUnit{Kind: step.UnitKind, ID: step.UnitID}
		if _, exists := retained[unit]; !exists {
			return fmt.Errorf("stress counterexample reduction step %d targets a non-retained unit", index)
		}
		switch step.Decision {
		case ReductionAccepted:
			if !step.Observation.RelationRevalidated || !step.Observation.PrivacyRevalidated || !step.Observation.ViolationPreserved || step.AfterDigest != step.CandidateDigest {
				return fmt.Errorf("accepted reduction step %d lacks relation, privacy, or violation proof", index)
			}
			accepted++
			delete(retained, unit)
		case ReductionRejected:
			if step.AfterDigest != step.BeforeDigest {
				return fmt.Errorf("rejected reduction step %d changed the retained witness", index)
			}
			if step.Observation.RelationRevalidated && step.Observation.PrivacyRevalidated && step.Observation.ViolationPreserved {
				return fmt.Errorf("rejected reduction step %d preserved every required invariant", index)
			}
		default:
			return fmt.Errorf("stress counterexample reduction step %d has an invalid decision", index)
		}
		current = step.AfterDigest
	}
	if current != value.ReducedInputDigest || accepted != value.AcceptedReductions || accepted == 0 && value.ReducedInputDigest != value.OriginalInputDigest ||
		len(value.OriginalUnits)-accepted != len(value.FinalUnits) || !sameReductionUnitSet(retained, value.FinalUnits) {
		return errors.New("stress counterexample reduction summary does not reproduce its trace")
	}
	if err := validateOneMinimalTail(value); err != nil {
		return err
	}
	expected, err := counterexampleDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress counterexample digest is invalid")
	}
	return nil
}

func validateBoundReductionObservation(value ReductionObservation, relationDigest, privacyPolicyDigest string) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if value.RelationDigest != relationDigest || value.PrivacyPolicyDigest != privacyPolicyDigest {
		return errors.New("stress reduction observation belongs to another relation or privacy policy")
	}
	return nil
}

func sameReductionUnitSet(left map[ReductionUnit]struct{}, right []ReductionUnit) bool {
	if len(left) != len(right) {
		return false
	}
	for _, value := range right {
		if _, exists := left[value]; !exists {
			return false
		}
	}
	return true
}

func validateOneMinimalTail(value Counterexample) error {
	lastAccepted := -1
	for index, step := range value.Steps {
		if step.Decision == ReductionAccepted {
			lastAccepted = index
		}
	}
	tail := value.Steps[lastAccepted+1:]
	if len(tail) != len(value.FinalUnits) {
		return errors.New("one-minimal counterexample lacks one final rejected attempt per retained unit")
	}
	for index, unit := range value.FinalUnits {
		step := tail[index]
		if step.Decision != ReductionRejected || step.UnitKind != unit.Kind || step.UnitID != unit.ID ||
			step.BeforeDigest != value.ReducedInputDigest || step.AfterDigest != value.ReducedInputDigest {
			return errors.New("one-minimal counterexample final rejection pass is incomplete or out of order")
		}
	}
	return nil
}

func validateReductionUnits(values []ReductionUnit, requireNonEmpty bool) error {
	if requireNonEmpty && len(values) == 0 {
		return errors.New("stress reduction requires at least one retained unit")
	}
	for index, value := range values {
		if !identifierPattern.MatchString(value.Kind) || !identifierPattern.MatchString(value.ID) || index > 0 && compareReductionUnits(values[index-1], value) >= 0 {
			return errors.New("stress reduction units must be valid, unique, and sorted")
		}
	}
	return nil
}

func compareReductionUnits(left, right ReductionUnit) int {
	if left.Kind != right.Kind {
		return strings.Compare(left.Kind, right.Kind)
	}
	return strings.Compare(left.ID, right.ID)
}

func counterexampleDigest(value Counterexample) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
