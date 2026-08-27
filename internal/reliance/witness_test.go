package reliance

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type relianceWitnessOracleFixture struct {
	required                   stress.ReductionUnit
	relationDigest             string
	privacyDigest              string
	originalInputDigest        string
	originalReplayDigest       string
	originalBatchFingerprint   string
	replayCaptureDigest        string
	originalInterventionDigest string
	outcomeIdentityDigest      string
	relianceResultDigest       string
	inconsistent               bool
}

func TestRelianceWitnessPreservesInterventionOutcomeAndResult(t *testing.T) {
	witness, request, execution := buildRelianceWitnessFixture(t, false)
	if err := witness.Validate(request.ExecutionRequest, execution); err != nil {
		t.Fatal(err)
	}
	if len(witness.FinalUnits) != 1 || witness.FinalUnits[0].ID != "executable-evidence" ||
		len(witness.Evaluations) != len(witness.Counterexample.Steps)+1 || witness.NetworkRequired {
		t.Fatalf("reliance witness identity = %+v", witness)
	}
	final := witnessEvaluationByDigest(t, witness.Evaluations, witness.FinalEvaluationDigest)
	if final.Status != RelianceWitnessPreserved || !final.InterventionValid ||
		final.OutcomeIdentityDigest != witness.OutcomeIdentityDigest ||
		final.RelianceResultDigest != witness.OriginalRelianceResultDigest {
		t.Fatalf("reliance witness final evaluation = %+v", final)
	}
	assertRelianceWitnessDecisionStatuses(t, witness)
}

func TestRelianceWitnessPublicationFixtureIsDeterministic(t *testing.T) {
	first, firstRequest, firstExecution := buildRelianceWitnessFixture(t, false)
	second, secondRequest, secondExecution := buildRelianceWitnessFixture(t, false)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(firstRequest.ExecutionRequest, secondRequest.ExecutionRequest) ||
		!reflect.DeepEqual(firstExecution, secondExecution) {
		t.Fatal("reliance witness publication fixture changed across identical offline executions")
	}
}

func TestRelianceWitnessRejectsPreservationProofTampering(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*RelianceWitnessEvaluation)
	}{
		{"intervention validity", func(value *RelianceWitnessEvaluation) { value.InterventionValid = false }},
		{"outcome identity", func(value *RelianceWitnessEvaluation) {
			value.OutcomeIdentityDigest = analysisDigest("foreign-outcome")
		}},
		{"reliance result", func(value *RelianceWitnessEvaluation) { value.RelianceResultDigest = analysisDigest("foreign-result") }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			witness, request, execution := buildRelianceWitnessFixture(t, false)
			index := witnessEvaluationIndex(t, witness.Evaluations, witness.FinalEvaluationDigest)
			mutation.mutate(&witness.Evaluations[index])
			resealRelianceWitnessTestValue(t, &witness, index)
			if err := witness.Validate(request.ExecutionRequest, execution); err == nil {
				t.Fatalf("resealed %s tampering was accepted", mutation.name)
			}
		})
	}
}

func TestRelianceWitnessBindsOriginalReplayBatchFingerprint(t *testing.T) {
	witness, request, execution := buildRelianceWitnessFixture(t, false)
	witness.Evaluations[0].ReplayBatchFingerprint = analysisDigest("foreign-batch")
	resealRelianceWitnessTestValue(t, &witness, 0)
	if err := witness.Validate(request.ExecutionRequest, execution); err == nil {
		t.Fatal("resealed original replay-batch substitution was accepted")
	}
}

func TestRelianceWitnessRejectsSourceReplayReuseForReducedCandidate(t *testing.T) {
	witness, request, execution := buildRelianceWitnessFixture(t, false)
	witness.Evaluations[1].ReplayBatchFingerprint = witness.SourceBatchRunFingerprint
	resealRelianceWitnessTestValue(t, &witness, 1)
	if err := witness.Validate(request.ExecutionRequest, execution); err == nil {
		t.Fatal("reduced candidate reused the original replay-batch fingerprint")
	}
}

func TestRelianceWitnessRejectsInconsistentOraclePreservation(t *testing.T) {
	_, request, _ := relianceWitnessFixture(t, true)
	if _, err := ReduceRelianceWitness(context.Background(), request); err == nil {
		t.Fatal("reliance witness accepted an oracle whose result identity contradicted its preservation flag")
	}
}

func buildRelianceWitnessFixture(
	t *testing.T,
	inconsistent bool,
) (RelianceWitness, RelianceWitnessReductionRequest, RelationExecutionResult) {
	t.Helper()
	executionRequest := relationExecutionRequestFixture(t)
	execution, err := RunRelationBackedIntervention(context.Background(),
		newRelianceReplayRunner(t, newRelianceReplayProvider(t)), executionRequest)
	if err != nil {
		t.Fatal(err)
	}
	execution = normalizeRelianceWitnessFixtureTiming(t, executionRequest, execution)
	request := newRelianceWitnessReductionRequest(t, executionRequest, execution, inconsistent)
	witness, err := ReduceRelianceWitness(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return witness, request, execution
}

func relianceWitnessFixture(
	t *testing.T,
	inconsistent bool,
) (RelationExecutionResult, RelianceWitnessReductionRequest, *relianceWitnessOracleFixture) {
	t.Helper()
	executionRequest := relationExecutionRequestFixture(t)
	execution, err := RunRelationBackedIntervention(context.Background(),
		newRelianceReplayRunner(t, newRelianceReplayProvider(t)), executionRequest)
	if err != nil {
		t.Fatal(err)
	}
	execution = normalizeRelianceWitnessFixtureTiming(t, executionRequest, execution)
	request := newRelianceWitnessReductionRequest(t, executionRequest, execution, inconsistent)
	oracle, ok := request.Oracle.(*relianceWitnessOracleFixture)
	if !ok {
		t.Fatal("reliance witness fixture oracle has the wrong type")
	}
	return execution, request, oracle
}

func normalizeRelianceWitnessFixtureTiming(
	t *testing.T,
	request RelationExecutionRequest,
	value RelationExecutionResult,
) RelationExecutionResult {
	t.Helper()
	value.Replay.Original.Result.Budget.ElapsedSeconds = 0
	value.Replay.Transformed.Result.Budget.ElapsedSeconds = 0
	value.Replay.Digest = ""
	raw, err := json.Marshal(value.Replay)
	if err != nil {
		t.Fatal(err)
	}
	value.Replay.Digest = protocolkit.DigestBytes(raw)
	value.RelationResult, err = stress.SealReplayResult(request.AdmissionInputs.Relation,
		request.AdmissionInputs.ConstructAdmission, request.AdmissionInputs.ReplayedRelationCase.TaskGroupID, value.Replay)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.Validate(request); err != nil {
		t.Fatal(err)
	}
	return value
}

func newRelianceWitnessReductionRequest(
	t *testing.T,
	executionRequest RelationExecutionRequest,
	execution RelationExecutionResult,
	inconsistent bool,
) RelianceWitnessReductionRequest {
	t.Helper()
	input := relianceReductionFixture()
	inputDigest, err := relianceWitnessInputDigest(input)
	if err != nil {
		t.Fatal(err)
	}
	resultDigest, err := RelianceResultIdentityDigest(executionRequest.AdmissionInputs.Relation, execution.RelationResult)
	if err != nil {
		t.Fatal(err)
	}
	privacyDigest := analysisDigest("reliance-witness-privacy")
	oracle := &relianceWitnessOracleFixture{
		required: input.Retained[0], relationDigest: executionRequest.Admission.RelationDigest,
		privacyDigest: privacyDigest, originalInputDigest: inputDigest, originalReplayDigest: execution.Replay.Digest,
		originalBatchFingerprint:   execution.Replay.BatchRunFingerprint,
		replayCaptureDigest:        execution.Replay.ReplaySource.CaptureSHA256,
		originalInterventionDigest: executionRequest.Admission.InterventionDigest,
		outcomeIdentityDigest:      executionRequest.AdmissionInputs.ReplayedRelationCase.OutcomeEvidenceDigest,
		relianceResultDigest:       resultDigest, inconsistent: inconsistent,
	}
	return RelianceWitnessReductionRequest{
		ExecutionRequest: executionRequest, Execution: execution, PrivacyPolicyDigest: privacyDigest,
		PublicReleaseAllowed: true, Input: input, Oracle: oracle, MaximumEvaluations: 6,
	}
}

func (oracle *relianceWitnessOracleFixture) EvaluateReliance(
	_ context.Context,
	candidate stress.ReducibleInput,
) (RelianceWitnessOracleObservation, error) {
	input, ok := candidate.(relianceReductionInput)
	if !ok {
		return RelianceWitnessOracleObservation{}, errors.New("unexpected reliance witness input")
	}
	encoded, err := input.CanonicalBytes()
	if err != nil {
		return RelianceWitnessOracleObservation{}, err
	}
	inputDigest := protocolkit.DigestBytes(encoded)
	preserved := slices.Contains(input.Retained, oracle.required)
	outcomeDigest, resultDigest := oracle.outcomeIdentityDigest, oracle.relianceResultDigest
	if !preserved {
		outcomeDigest, resultDigest = analysisDigest("changed-outcome", inputDigest), analysisDigest("changed-result", inputDigest)
	}
	observation, err := oracle.reductionObservation(encoded, inputDigest, preserved)
	if err != nil {
		return RelianceWitnessOracleObservation{}, err
	}
	return RelianceWitnessOracleObservation{
		ReductionObservation: observation, ReplayBatchFingerprint: oracle.batchFingerprint(inputDigest),
		ReplayCaptureDigest: oracle.replayCaptureDigest, InterventionValidityDigest: oracle.interventionDigest(encoded, inputDigest),
		InterventionValid: preserved, OutcomeIdentityDigest: outcomeDigest, RelianceResultDigest: resultDigest,
	}, nil
}

func (oracle *relianceWitnessOracleFixture) reductionObservation(
	encoded []byte,
	inputDigest string,
	preserved bool,
) (stress.ReductionObservation, error) {
	replayDigest := analysisDigest("candidate-replay", inputDigest)
	if inputDigest == oracle.originalInputDigest {
		replayDigest = oracle.originalReplayDigest
	}
	violationPreserved := preserved
	if oracle.inconsistent && !preserved {
		violationPreserved = true
	}
	return stress.SealReductionObservation(stress.ReductionObservation{
		RelationDigest: oracle.relationDigest, PrivacyPolicyDigest: oracle.privacyDigest,
		RelationRevalidated: true, PrivacyRevalidated: true, ViolationPreserved: violationPreserved,
		RelationProofDigest: analysisDigest("witness-relation", string(encoded)),
		PrivacyProofDigest:  analysisDigest("witness-privacy", string(encoded)), ReplayResultDigest: replayDigest,
	})
}

func (oracle *relianceWitnessOracleFixture) batchFingerprint(inputDigest string) string {
	if inputDigest == oracle.originalInputDigest {
		return oracle.originalBatchFingerprint
	}
	return analysisDigest("candidate-batch", inputDigest)
}

func (oracle *relianceWitnessOracleFixture) interventionDigest(encoded []byte, inputDigest string) string {
	if inputDigest == oracle.originalInputDigest {
		return oracle.originalInterventionDigest
	}
	return analysisDigest("candidate-intervention", string(encoded))
}

func assertRelianceWitnessDecisionStatuses(t *testing.T, witness RelianceWitness) {
	t.Helper()
	for index, step := range witness.Counterexample.Steps {
		status := witness.Evaluations[index+1].Status
		if step.Decision == stress.ReductionAccepted && status != RelianceWitnessPreserved {
			t.Fatalf("accepted step %d status = %s", index, status)
		}
		if step.Decision == stress.ReductionRejected && status != RelianceWitnessUnresolved {
			t.Fatalf("rejected step %d status = %s", index, status)
		}
	}
}

func witnessEvaluationByDigest(
	t *testing.T,
	values []RelianceWitnessEvaluation,
	digest string,
) RelianceWitnessEvaluation {
	t.Helper()
	return values[witnessEvaluationIndex(t, values, digest)]
}

func witnessEvaluationIndex(t *testing.T, values []RelianceWitnessEvaluation, digest string) int {
	t.Helper()
	for index, value := range values {
		if value.Digest == digest {
			return index
		}
	}
	t.Fatalf("reliance witness evaluation %s is absent", digest)
	return -1
}

func resealRelianceWitnessTestValue(t *testing.T, value *RelianceWitness, evaluationIndex int) {
	t.Helper()
	oldDigest := value.Evaluations[evaluationIndex].Digest
	digest, err := relianceWitnessEvaluationDigest(value.Evaluations[evaluationIndex])
	if err != nil {
		t.Fatal(err)
	}
	value.Evaluations[evaluationIndex].Digest = digest
	if value.FinalEvaluationDigest == oldDigest {
		value.FinalEvaluationDigest = digest
	}
	digest, err = relianceWitnessDigest(*value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
}
