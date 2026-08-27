package reliance

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

const (
	RelationExecutionOriginalVariant    = "original"
	RelationExecutionTransformedVariant = "transformed"
)

type RelationExecutionRequest struct {
	AdmissionInputs RelationAdmissionInputs
	Admission       RelationBackedInterventionAdmission
	Original        verification.Input
	Transformed     verification.Input
}

type RelationExecutionResult struct {
	Replay         stress.ReplayExecution
	RelationResult stress.Result
}

type RelationReductionRequest struct {
	ExecutionRequest     RelationExecutionRequest
	Execution            RelationExecutionResult
	PrivacyPolicyDigest  string
	PublicReleaseAllowed bool
	Input                stress.ReducibleInput
	Oracle               stress.ReductionOracle
	MaximumEvaluations   int
}

func RunRelationBackedIntervention(ctx context.Context, runner *stress.ReplayFirstRunner, request RelationExecutionRequest) (RelationExecutionResult, error) {
	if ctx == nil || runner == nil {
		return RelationExecutionResult{}, errors.New("relation-backed reliance execution requires a context and shared stress runner")
	}
	if err := validateRelationExecutionRequest(request); err != nil {
		return RelationExecutionResult{}, err
	}
	replay, err := runner.RunMutation(ctx, stress.ReplayRunRequest{
		Relation: request.AdmissionInputs.Relation, Admission: request.AdmissionInputs.ConstructAdmission,
		Original: request.Original, Transformed: request.Transformed,
	})
	if err != nil {
		return RelationExecutionResult{}, fmt.Errorf("run relation-backed reliance intervention: %w", err)
	}
	relationResult, err := stress.SealReplayResult(request.AdmissionInputs.Relation,
		request.AdmissionInputs.ConstructAdmission, request.AdmissionInputs.ReplayedRelationCase.TaskGroupID, replay)
	if err != nil {
		return RelationExecutionResult{}, fmt.Errorf("seal relation-backed reliance result: %w", err)
	}
	result := RelationExecutionResult{Replay: replay, RelationResult: relationResult}
	return result, result.Validate(request)
}

func (value RelationExecutionResult) Validate(request RelationExecutionRequest) error {
	if err := validateRelationExecutionRequest(request); err != nil {
		return err
	}
	if err := value.Replay.ValidateAgainst(request.AdmissionInputs.Relation); err != nil {
		return err
	}
	expected, err := stress.SealReplayResult(request.AdmissionInputs.Relation,
		request.AdmissionInputs.ConstructAdmission, request.AdmissionInputs.ReplayedRelationCase.TaskGroupID, value.Replay)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value.RelationResult, expected) {
		return errors.New("relation-backed reliance result differs from its exact replay")
	}
	return nil
}

func ReduceRelationBackedIntervention(ctx context.Context, request RelationReductionRequest) (stress.Counterexample, error) {
	if ctx == nil {
		return stress.Counterexample{}, errors.New("relation-backed reliance reduction requires a context")
	}
	if err := request.Execution.Validate(request.ExecutionRequest); err != nil {
		return stress.Counterexample{}, fmt.Errorf("validate relation-backed reliance execution before reduction: %w", err)
	}
	admission := request.ExecutionRequest.Admission
	value, err := stress.ReduceCounterexample(ctx, stress.ReductionRequest{
		RelationDigest: admission.RelationDigest, SourceResultDigest: request.Execution.Replay.Digest,
		CaseID: admission.RelationCaseID, PrivacyPolicyDigest: request.PrivacyPolicyDigest,
		PublicReleaseAllowed: request.PublicReleaseAllowed, Input: request.Input,
		Oracle: request.Oracle, MaximumEvaluations: request.MaximumEvaluations,
	})
	if err != nil {
		return stress.Counterexample{}, fmt.Errorf("reduce relation-backed reliance intervention: %w", err)
	}
	if value.SourceResultDigest != request.Execution.Replay.Digest ||
		value.OriginalObservation.ReplayResultDigest != request.Execution.Replay.Digest {
		return stress.Counterexample{}, errors.New("relation-backed reliance reduction does not bind its source replay")
	}
	return value, nil
}

func validateRelationExecutionRequest(request RelationExecutionRequest) error {
	if err := request.Admission.Validate(request.AdmissionInputs); err != nil {
		return err
	}
	if !request.Admission.PrimaryEligible && !request.Admission.SensitivityEligible {
		return errors.New("relation-backed reliance admission is ineligible for execution")
	}
	replayed := request.AdmissionInputs.ReplayedRelationCase
	if err := validateRelationExecutionSide(request.Original, replayed.Original,
		RelationExecutionOriginalVariant, request.AdmissionInputs); err != nil {
		return err
	}
	if err := validateRelationExecutionSide(request.Transformed, replayed.Transformed,
		RelationExecutionTransformedVariant, request.AdmissionInputs); err != nil {
		return err
	}
	return validateSharedRelationExecutionInput(request.Original, request.Transformed)
}

func validateRelationExecutionSide(input verification.Input, trajectories []preprocess.Trajectory, variant string, parents RelationAdmissionInputs) error {
	expected := renderRelationExecutionTrajectories(trajectories)
	wantMode := verification.ModePairwise
	if len(expected) == 1 {
		wantMode = verification.ModeAbsolute
	}
	lineage := input.Lineage
	interventionID := parents.Intervention.Intervention.InterventionID
	if input.Mode != wantMode || !slices.Equal(input.Trajectories, expected) || input.StudyVariant != variant ||
		lineage.AuditCaseID != parents.ConstructAdmission.CaseID || lineage.TransformationID != parents.Relation.ID ||
		lineage.OutcomeEvidenceDigest != parents.ReplayedRelationCase.OutcomeEvidenceDigest || lineage.StudyCellID != interventionID {
		return fmt.Errorf("relation-backed reliance %s input differs from its exact replay, side, or lineage", variant)
	}
	if input.StudyManifestDigest == "" || !input.DisableCache || input.AuthorizationDigest != "" || input.BudgetStatePath != "" {
		return fmt.Errorf("relation-backed reliance %s input lacks a study binding or offline boundary", variant)
	}
	return nil
}

func validateSharedRelationExecutionInput(original, transformed verification.Input) error {
	left, right := original, transformed
	left.Trajectories, right.Trajectories = nil, nil
	left.StudyVariant, right.StudyVariant = "", ""
	if !reflect.DeepEqual(left, right) {
		return errors.New("relation-backed reliance execution sides differ outside trajectory and study variant")
	}
	return nil
}

func renderRelationExecutionTrajectories(values []preprocess.Trajectory) []string {
	result := make([]string, len(values))
	for index, trajectory := range values {
		result[index] = preprocess.RenderTrajectory(trajectory)
	}
	return result
}
