package reliance

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/stress"
)

func (value RelianceWitness) Validate(
	request RelationExecutionRequest,
	execution RelationExecutionResult,
) error {
	if err := execution.Validate(request); err != nil {
		return err
	}
	if err := value.Counterexample.Validate(); err != nil {
		return err
	}
	outcomeDigest := request.AdmissionInputs.ReplayedRelationCase.OutcomeEvidenceDigest
	resultDigest, err := RelianceResultIdentityDigest(request.AdmissionInputs.Relation, execution.RelationResult)
	if err != nil {
		return err
	}
	if err := validateRelianceWitnessIdentity(value, request, execution, outcomeDigest, resultDigest); err != nil {
		return err
	}
	if err := validateRelianceWitnessTrace(value, outcomeDigest, resultDigest,
		request.Admission.InterventionDigest, execution.Replay.ReplaySource.CaptureSHA256); err != nil {
		return err
	}
	digest, err := relianceWitnessDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance witness digest is invalid")
	}
	return nil
}

func validateRelianceWitnessIdentity(
	value RelianceWitness,
	request RelationExecutionRequest,
	execution RelationExecutionResult,
	outcomeDigest string,
	resultDigest string,
) error {
	admission := request.Admission
	counterexample := value.Counterexample
	if value.SchemaVersion != RelianceWitnessSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.EvaluationPolicy != RelianceWitnessEvaluationPolicy || value.AdmissionDigest != admission.Digest ||
		value.RelationDigest != admission.RelationDigest || value.CaseID != admission.RelationCaseID ||
		value.SourceReplayDigest != execution.Replay.Digest || value.SourceBatchRunFingerprint != execution.Replay.BatchRunFingerprint ||
		value.SourceReplayCaptureDigest != execution.Replay.ReplaySource.CaptureSHA256 ||
		value.OutcomeIdentityDigest != outcomeDigest || value.OriginalRelianceResultDigest != resultDigest || value.NetworkRequired {
		return errors.New("reliance witness source identity, result, or execution boundary is invalid")
	}
	if counterexample.RelationDigest != value.RelationDigest || counterexample.CaseID != value.CaseID ||
		counterexample.SourceResultDigest != value.SourceReplayDigest ||
		counterexample.OriginalObservation.ReplayResultDigest != value.SourceReplayDigest ||
		counterexample.PublicReleaseAllowed != value.PublicReleaseAllowed ||
		!slices.Equal(value.FinalUnits, counterexample.FinalUnits) {
		return errors.New("reliance witness counterexample binding is invalid")
	}
	return nil
}

func validateRelianceWitnessTrace(
	value RelianceWitness,
	expectedOutcome string,
	expectedResult string,
	originalInterventionDigest string,
	expectedCapture string,
) error {
	if len(value.Evaluations) != len(value.Counterexample.Steps)+1 {
		return errors.New("reliance witness evaluation trace length is invalid")
	}
	for index, evaluation := range value.Evaluations {
		if err := validateRelianceWitnessEvaluation(evaluation, expectedOutcome, expectedResult, expectedCapture); err != nil {
			return fmt.Errorf("validate reliance witness evaluation %d: %w", index, err)
		}
	}
	if err := validateRelianceWitnessCandidateReplays(value); err != nil {
		return err
	}
	original := value.Evaluations[0]
	if original.InputDigest != value.Counterexample.OriginalInputDigest || original.Status != RelianceWitnessPreserved ||
		original.ReplayBatchFingerprint != value.SourceBatchRunFingerprint ||
		original.InterventionValidityDigest != originalInterventionDigest ||
		!reflect.DeepEqual(original.ReductionObservation, value.Counterexample.OriginalObservation) {
		return errors.New("reliance witness original evaluation binding is invalid")
	}
	final, err := traceFinalRelianceWitnessEvaluation(value.Counterexample, value.Evaluations)
	if err != nil {
		return err
	}
	if final.Status != RelianceWitnessPreserved || final.InputDigest != value.Counterexample.ReducedInputDigest ||
		value.FinalEvaluationDigest != final.Digest {
		return errors.New("reliance witness final evaluation is not preserved or bound")
	}
	return nil
}

func validateRelianceWitnessCandidateReplays(value RelianceWitness) error {
	for index, evaluation := range value.Evaluations[1:] {
		if evaluation.InputDigest == value.Counterexample.OriginalInputDigest ||
			evaluation.ReplayBatchFingerprint == value.SourceBatchRunFingerprint ||
			evaluation.ReductionObservation.ReplayResultDigest == value.SourceReplayDigest {
			return fmt.Errorf("reliance witness candidate evaluation %d reused source input or replay identity", index+1)
		}
	}
	return nil
}

func validateRelianceWitnessEvaluation(
	value RelianceWitnessEvaluation,
	expectedOutcome string,
	expectedResult string,
	expectedCapture string,
) error {
	if value.SchemaVersion != RelianceWitnessEvaluationSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.EvaluationPolicy != RelianceWitnessEvaluationPolicy || !validRelianceDigest(value.InputDigest) ||
		!validRelianceDigest(value.ReplayBatchFingerprint) || value.ReplayCaptureDigest != expectedCapture ||
		!validRelianceDigest(value.InterventionValidityDigest) ||
		!validRelianceDigest(value.OutcomeIdentityDigest) || !validRelianceDigest(value.RelianceResultDigest) || value.NetworkRequired {
		return errors.New("reliance witness evaluation identity or execution boundary is invalid")
	}
	if err := value.ReductionObservation.Validate(); err != nil {
		return err
	}
	wantReasons := relianceWitnessReasons(value, expectedOutcome, expectedResult)
	wantStatus := RelianceWitnessPreserved
	if len(wantReasons) > 0 {
		wantStatus = RelianceWitnessUnresolved
	}
	if value.Status != wantStatus || !slices.Equal(value.UnresolvedReasons, wantReasons) ||
		value.ReductionObservation.ViolationPreserved != (wantStatus == RelianceWitnessPreserved) {
		return errors.New("reliance witness evaluation status or preservation proof is invalid")
	}
	digest, err := relianceWitnessEvaluationDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance witness evaluation digest is invalid")
	}
	return nil
}

func traceFinalRelianceWitnessEvaluation(
	counterexample stress.Counterexample,
	evaluations []RelianceWitnessEvaluation,
) (RelianceWitnessEvaluation, error) {
	final := evaluations[0]
	for index, step := range counterexample.Steps {
		evaluation := evaluations[index+1]
		if evaluation.InputDigest != step.CandidateDigest ||
			!reflect.DeepEqual(evaluation.ReductionObservation, step.Observation) {
			return RelianceWitnessEvaluation{}, fmt.Errorf("reliance witness step %d evaluation binding is invalid", index)
		}
		if step.Decision == stress.ReductionAccepted {
			if evaluation.Status != RelianceWitnessPreserved {
				return RelianceWitnessEvaluation{}, fmt.Errorf("reliance witness accepted step %d is unresolved", index)
			}
			final = evaluation
		} else if evaluation.Status != RelianceWitnessUnresolved {
			return RelianceWitnessEvaluation{}, fmt.Errorf("reliance witness rejected step %d still preserves the result", index)
		}
	}
	return final, nil
}

func finalRelianceWitnessEvaluation(
	counterexample stress.Counterexample,
	evaluations []RelianceWitnessEvaluation,
) (RelianceWitnessEvaluation, error) {
	if len(evaluations) != len(counterexample.Steps)+1 {
		return RelianceWitnessEvaluation{}, errors.New("reliance witness reducer and evaluation trace differ")
	}
	return traceFinalRelianceWitnessEvaluation(counterexample, evaluations)
}

func cloneRelianceWitnessEvaluations(values []RelianceWitnessEvaluation) []RelianceWitnessEvaluation {
	result := slices.Clone(values)
	for index := range result {
		result[index].UnresolvedReasons = slices.Clone(result[index].UnresolvedReasons)
	}
	return result
}

func relianceWitnessEvaluationDigest(value RelianceWitnessEvaluation) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}

func relianceWitnessDigest(value RelianceWitness) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
