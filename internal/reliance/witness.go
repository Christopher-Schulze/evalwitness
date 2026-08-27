package reliance

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type relianceWitnessOracleAdapter struct {
	oracle          RelianceWitnessOracle
	expectedOutcome string
	expectedResult  string
	expectedCapture string
	evaluations     []RelianceWitnessEvaluation
}

type relianceResultIdentity struct {
	RelationDigest          string                                `json:"relation_digest"`
	CaseID                  string                                `json:"case_id"`
	TaskGroupID             string                                `json:"task_group_id"`
	Outcome                 stress.Outcome                        `json:"outcome"`
	InvalidState            stress.InvalidState                   `json:"invalid_state,omitempty"`
	ConstraintResults       []stress.ConstraintResult             `json:"constraint_results"`
	DistributionComparisons []stress.TaggedDistributionComparison `json:"distribution_comparisons"`
	PlannedRepetitions      int                                   `json:"planned_repetitions"`
	CompletedRepetitions    int                                   `json:"completed_repetitions"`
}

func ReduceRelianceWitness(ctx context.Context, request RelianceWitnessReductionRequest) (RelianceWitness, error) {
	outcomeDigest, resultDigest, err := validateRelianceWitnessRequest(ctx, request)
	if err != nil {
		return RelianceWitness{}, err
	}
	adapter := &relianceWitnessOracleAdapter{
		oracle: request.Oracle, expectedOutcome: outcomeDigest, expectedResult: resultDigest,
		expectedCapture: request.Execution.Replay.ReplaySource.CaptureSHA256,
	}
	counterexample, err := ReduceRelationBackedIntervention(ctx, RelationReductionRequest{
		ExecutionRequest: request.ExecutionRequest, Execution: request.Execution,
		PrivacyPolicyDigest: request.PrivacyPolicyDigest, PublicReleaseAllowed: request.PublicReleaseAllowed,
		Input: request.Input, Oracle: adapter, MaximumEvaluations: request.MaximumEvaluations,
	})
	if err != nil {
		return RelianceWitness{}, err
	}
	value, err := buildRelianceWitness(request, counterexample, adapter.evaluations, outcomeDigest, resultDigest)
	if err != nil {
		return RelianceWitness{}, err
	}
	return value, value.Validate(request.ExecutionRequest, request.Execution)
}

func validateRelianceWitnessRequest(
	ctx context.Context,
	request RelianceWitnessReductionRequest,
) (string, string, error) {
	if ctx == nil || isNilRelianceWitnessOracle(request.Oracle) {
		return "", "", errors.New("reliance witness reduction requires a context and oracle")
	}
	if err := request.Execution.Validate(request.ExecutionRequest); err != nil {
		return "", "", fmt.Errorf("validate reliance witness source execution: %w", err)
	}
	outcomeDigest := request.ExecutionRequest.AdmissionInputs.ReplayedRelationCase.OutcomeEvidenceDigest
	if !validRelianceDigest(outcomeDigest) {
		return "", "", errors.New("reliance witness source outcome identity is invalid")
	}
	resultDigest, err := RelianceResultIdentityDigest(
		request.ExecutionRequest.AdmissionInputs.Relation, request.Execution.RelationResult,
	)
	if err != nil {
		return "", "", err
	}
	return outcomeDigest, resultDigest, nil
}

func (adapter *relianceWitnessOracleAdapter) Evaluate(
	ctx context.Context,
	input stress.ReducibleInput,
) (stress.ReductionObservation, error) {
	observation, err := adapter.oracle.EvaluateReliance(ctx, input)
	if err != nil {
		return stress.ReductionObservation{}, err
	}
	inputDigest, err := relianceWitnessInputDigest(input)
	if err != nil {
		return stress.ReductionObservation{}, err
	}
	evaluation, err := sealRelianceWitnessEvaluation(
		inputDigest, observation, adapter.expectedOutcome, adapter.expectedResult, adapter.expectedCapture,
	)
	if err != nil {
		return stress.ReductionObservation{}, err
	}
	adapter.evaluations = append(adapter.evaluations, evaluation)
	return observation.ReductionObservation, nil
}

func sealRelianceWitnessEvaluation(
	inputDigest string,
	observation RelianceWitnessOracleObservation,
	expectedOutcome string,
	expectedResult string,
	expectedCapture string,
) (RelianceWitnessEvaluation, error) {
	value := RelianceWitnessEvaluation{
		SchemaVersion: RelianceWitnessEvaluationSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		EvaluationPolicy: RelianceWitnessEvaluationPolicy, InputDigest: inputDigest,
		ReplayBatchFingerprint:     observation.ReplayBatchFingerprint,
		ReplayCaptureDigest:        observation.ReplayCaptureDigest,
		InterventionValidityDigest: observation.InterventionValidityDigest, InterventionValid: observation.InterventionValid,
		OutcomeIdentityDigest: observation.OutcomeIdentityDigest, RelianceResultDigest: observation.RelianceResultDigest,
		ReductionObservation: observation.ReductionObservation, NetworkRequired: observation.NetworkRequired,
	}
	value.UnresolvedReasons = relianceWitnessReasons(value, expectedOutcome, expectedResult)
	value.Status = RelianceWitnessPreserved
	if len(value.UnresolvedReasons) > 0 {
		value.Status = RelianceWitnessUnresolved
	}
	digest, err := relianceWitnessEvaluationDigest(value)
	if err != nil {
		return RelianceWitnessEvaluation{}, err
	}
	value.Digest = digest
	if err := validateRelianceWitnessEvaluation(value, expectedOutcome, expectedResult, expectedCapture); err != nil {
		return RelianceWitnessEvaluation{}, err
	}
	return value, nil
}

func relianceWitnessReasons(
	value RelianceWitnessEvaluation,
	expectedOutcome string,
	expectedResult string,
) []RelianceWitnessUnresolvedReason {
	reasons := []RelianceWitnessUnresolvedReason{}
	if !value.InterventionValid {
		reasons = append(reasons, WitnessInterventionInvalid)
	}
	if value.OutcomeIdentityDigest != expectedOutcome {
		reasons = append(reasons, WitnessOutcomeChanged)
	}
	if !value.ReductionObservation.PrivacyRevalidated {
		reasons = append(reasons, WitnessPrivacyUnresolved)
	}
	if !value.ReductionObservation.RelationRevalidated {
		reasons = append(reasons, WitnessRelationUnresolved)
	}
	if value.RelianceResultDigest != expectedResult {
		reasons = append(reasons, WitnessResultChanged)
	}
	return reasons
}

func buildRelianceWitness(
	request RelianceWitnessReductionRequest,
	counterexample stress.Counterexample,
	evaluations []RelianceWitnessEvaluation,
	outcomeDigest string,
	resultDigest string,
) (RelianceWitness, error) {
	finalEvaluation, err := finalRelianceWitnessEvaluation(counterexample, evaluations)
	if err != nil {
		return RelianceWitness{}, err
	}
	value := RelianceWitness{
		SchemaVersion: RelianceWitnessSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		EvaluationPolicy: RelianceWitnessEvaluationPolicy, AdmissionDigest: request.ExecutionRequest.Admission.Digest,
		RelationDigest: request.ExecutionRequest.Admission.RelationDigest, CaseID: request.ExecutionRequest.Admission.RelationCaseID,
		SourceReplayDigest: request.Execution.Replay.Digest, SourceBatchRunFingerprint: request.Execution.Replay.BatchRunFingerprint,
		SourceReplayCaptureDigest: request.Execution.Replay.ReplaySource.CaptureSHA256,
		OutcomeIdentityDigest:     outcomeDigest, OriginalRelianceResultDigest: resultDigest,
		Counterexample: counterexample, Evaluations: cloneRelianceWitnessEvaluations(evaluations),
		FinalEvaluationDigest: finalEvaluation.Digest, FinalUnits: slices.Clone(counterexample.FinalUnits),
		PublicReleaseAllowed: counterexample.PublicReleaseAllowed, NetworkRequired: false,
	}
	digest, err := relianceWitnessDigest(value)
	if err != nil {
		return RelianceWitness{}, err
	}
	value.Digest = digest
	return value, nil
}

func RelianceResultIdentityDigest(spec stress.Relation, value stress.Result) (string, error) {
	if err := value.ValidateAgainst(spec); err != nil {
		return "", err
	}
	return referenceJSONDigest(relianceResultIdentity{
		RelationDigest: value.RelationDigest, CaseID: value.CaseID, TaskGroupID: value.TaskGroupID,
		Outcome: value.Outcome, InvalidState: value.InvalidState, ConstraintResults: value.ConstraintResults,
		DistributionComparisons: value.DistributionComparisons,
		PlannedRepetitions:      value.PlannedRepetitions, CompletedRepetitions: value.CompletedRepetitions,
	})
}

func relianceWitnessInputDigest(value stress.ReducibleInput) (string, error) {
	if value == nil {
		return "", errors.New("reliance witness input is nil")
	}
	encoded, err := value.CanonicalBytes()
	if err != nil {
		return "", fmt.Errorf("encode reliance witness input: %w", err)
	}
	if len(encoded) == 0 {
		return "", errors.New("reliance witness input canonical bytes are empty")
	}
	return protocolkit.DigestBytes(encoded), nil
}

func isNilRelianceWitnessOracle(value RelianceWitnessOracle) bool {
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
