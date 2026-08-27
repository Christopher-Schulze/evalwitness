package stress

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/Christopher-Schulze/evalwitness/internal/baseline"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type ZeroCostCandidate struct {
	Steps      int `json:"steps"`
	TraceBytes int `json:"trace_bytes"`
	ErrorWords int `json:"error_words"`
}

type ZeroCostExecution struct {
	SchemaVersion             string              `json:"schema_version"`
	CanonicalPolicy           string              `json:"canonical_policy"`
	ArmID                     string              `json:"arm_id"`
	RelationDigest            string              `json:"relation_digest"`
	CaseID                    string              `json:"case_id"`
	TaskGroupID               string              `json:"task_group_id"`
	ManifestDigest            string              `json:"manifest_digest"`
	OriginalCandidates        []ZeroCostCandidate `json:"original_candidates"`
	TransformedCandidates     []ZeroCostCandidate `json:"transformed_candidates"`
	OriginalSelectedIndex     int                 `json:"original_selected_index"`
	TransformedSelectedIndex  int                 `json:"transformed_selected_index"`
	OriginalSelectedDigest    string              `json:"original_selected_digest"`
	TransformedSelectedDigest string              `json:"transformed_selected_digest"`
	CompletedRepetitions      int                 `json:"completed_repetitions"`
	Result                    Result              `json:"result"`
	Digest                    string              `json:"digest"`
}

func RunZeroCostArm(spec Relation, admission ConstructAdmission, item ReplayedRelationCaseV3, armID string) (ZeroCostExecution, error) {
	arm, exists := canonicalArmByID(armID)
	if !exists || arm.Kind != ArmZeroCostControl {
		return ZeroCostExecution{}, errors.New("stress zero-cost execution requires one canonical zero-cost arm")
	}
	control, exists := zeroCostControlByArm(arm)
	if !exists {
		return ZeroCostExecution{}, errors.New("stress zero-cost arm does not bind one canonical baseline")
	}
	if support, reason := armSupportForRelation(arm, spec); support != ArmSupported {
		return ZeroCostExecution{}, fmt.Errorf("stress zero-cost arm is unsupported for this relation: %s", reason)
	}
	if err := validateZeroCostInputs(spec, admission, item); err != nil {
		return ZeroCostExecution{}, err
	}
	originalCandidates := zeroCostCandidates(item.Original)
	transformedCandidates := zeroCostCandidates(item.Transformed)
	originalIndex, transformedIndex := -1, -1
	for repetition := 0; repetition < spec.Repeat.MaximumRepetitions; repetition++ {
		observedOriginal := control.Pick(asBaselineCandidates(originalCandidates))
		observedTransformed := control.Pick(asBaselineCandidates(transformedCandidates))
		if repetition == 0 {
			originalIndex, transformedIndex = observedOriginal, observedTransformed
			continue
		}
		if observedOriginal != originalIndex || observedTransformed != transformedIndex {
			return ZeroCostExecution{}, errors.New("zero-cost control changed across deterministic repetitions")
		}
	}
	originalDigest := item.Original[originalIndex].Digest
	transformedDigest := item.Transformed[transformedIndex].Digest
	constraint, err := EvaluateConstraint(spec.Constraints[0], nil, nil, originalDigest, transformedDigest)
	if err != nil {
		return ZeroCostExecution{}, err
	}
	outcome := outcomeForConstraintResults([]ConstraintResult{constraint})
	result, err := SealResult(spec, Result{
		CaseID: item.CaseID, TaskGroupID: item.TaskGroupID, Admission: &admission, Outcome: outcome,
		ConstraintResults: []ConstraintResult{constraint}, DistributionComparisons: []TaggedDistributionComparison{},
		PlannedRepetitions: spec.Repeat.MaximumRepetitions, CompletedRepetitions: spec.Repeat.MaximumRepetitions,
		ProviderCalls: 0,
	})
	if err != nil {
		return ZeroCostExecution{}, err
	}
	value := ZeroCostExecution{
		SchemaVersion: ZeroCostExecutionSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		ArmID: arm.ID, RelationDigest: spec.Digest, CaseID: item.CaseID, TaskGroupID: item.TaskGroupID, ManifestDigest: item.ManifestDigest,
		OriginalCandidates: originalCandidates, TransformedCandidates: transformedCandidates,
		OriginalSelectedIndex: originalIndex, TransformedSelectedIndex: transformedIndex,
		OriginalSelectedDigest: originalDigest, TransformedSelectedDigest: transformedDigest,
		CompletedRepetitions: spec.Repeat.MaximumRepetitions, Result: result,
	}
	value.Digest, err = zeroCostExecutionDigest(value)
	if err != nil {
		return ZeroCostExecution{}, err
	}
	if err := value.ValidateAgainst(spec, admission, item); err != nil {
		return ZeroCostExecution{}, err
	}
	return value, nil
}

func (value ZeroCostExecution) ValidateAgainst(spec Relation, admission ConstructAdmission, item ReplayedRelationCaseV3) error {
	arm, exists := canonicalArmByID(value.ArmID)
	if !exists || arm.Kind != ArmZeroCostControl || value.SchemaVersion != ZeroCostExecutionSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.RelationDigest != spec.Digest || value.CaseID != item.CaseID || value.TaskGroupID != item.TaskGroupID ||
		value.ManifestDigest != item.ManifestDigest || value.CompletedRepetitions != spec.Repeat.MaximumRepetitions {
		return errors.New("stress zero-cost execution identity or repeat binding is invalid")
	}
	if err := validateZeroCostInputs(spec, admission, item); err != nil {
		return err
	}
	control, exists := zeroCostControlByArm(arm)
	if !exists {
		return errors.New("stress zero-cost execution arm is not canonical")
	}
	originalCandidates := zeroCostCandidates(item.Original)
	transformedCandidates := zeroCostCandidates(item.Transformed)
	if !reflect.DeepEqual(value.OriginalCandidates, originalCandidates) || !reflect.DeepEqual(value.TransformedCandidates, transformedCandidates) {
		return errors.New("stress zero-cost execution candidate features differ from replayed trajectories")
	}
	originalIndex := control.Pick(asBaselineCandidates(originalCandidates))
	transformedIndex := control.Pick(asBaselineCandidates(transformedCandidates))
	if value.OriginalSelectedIndex != originalIndex || value.TransformedSelectedIndex != transformedIndex ||
		value.OriginalSelectedDigest != item.Original[originalIndex].Digest || value.TransformedSelectedDigest != item.Transformed[transformedIndex].Digest {
		return errors.New("stress zero-cost execution selection does not reproduce from its baseline features")
	}
	if err := value.Result.ValidateAgainst(spec); err != nil {
		return err
	}
	if value.Result.CaseID != item.CaseID || value.Result.TaskGroupID != item.TaskGroupID || value.Result.ProviderCalls != 0 ||
		value.Result.CompletedRepetitions != value.CompletedRepetitions || value.Result.StageComparison != nil || len(value.Result.DistributionComparisons) != 0 {
		return errors.New("stress zero-cost result contains model execution or differs from its cell")
	}
	if len(value.Result.ConstraintResults) != 1 || value.Result.ConstraintResults[0].OriginalState != value.OriginalSelectedDigest ||
		value.Result.ConstraintResults[0].TransformedState != value.TransformedSelectedDigest {
		return errors.New("stress zero-cost result does not bind selected trajectory identities")
	}
	expectedDigest, err := zeroCostExecutionDigest(value)
	if err != nil || value.Digest != expectedDigest {
		return errors.New("stress zero-cost execution digest is invalid")
	}
	return nil
}

func validateZeroCostInputs(spec Relation, admission ConstructAdmission, item ReplayedRelationCaseV3) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if err := admission.Validate(); err != nil {
		return err
	}
	if admission.CaseID != item.CaseID || !slicesContains(item.RelationIDs, spec.ID) || item.Family != spec.Transform.MutationFamily ||
		!validDigest(item.ManifestDigest) || len(item.Original) != 2 || len(item.Transformed) != 2 || len(spec.Constraints) != 1 ||
		spec.Constraints[0].Metric != MetricDecision || spec.Constraints[0].Operator != OperatorEqual {
		return errors.New("stress zero-cost execution requires one admitted candidate-order decision cell")
	}
	if item.Original[0].Digest == item.Original[1].Digest || item.Original[0].Digest != item.Transformed[1].Digest ||
		item.Original[1].Digest != item.Transformed[0].Digest {
		return errors.New("stress zero-cost execution trajectories are not one exact candidate reversal")
	}
	if spec.StatisticalFamily.Estimand == EstimandPrimaryCore && !admission.PrimaryEligible ||
		spec.StatisticalFamily.Estimand != EstimandPrimaryCore && !admission.SensitivityEligible {
		return errors.New("stress zero-cost execution admission is ineligible for the relation estimand")
	}
	return nil
}

func zeroCostCandidates(trajectories []preprocess.Trajectory) []ZeroCostCandidate {
	result := make([]ZeroCostCandidate, len(trajectories))
	for index, trajectory := range trajectories {
		rendered := preprocess.RenderTrajectory(trajectory)
		result[index] = ZeroCostCandidate{
			Steps: len(trajectory.Events), TraceBytes: len(rendered), ErrorWords: baseline.CountErrorWords(rendered),
		}
	}
	return result
}

func asBaselineCandidates(values []ZeroCostCandidate) []baseline.Candidate {
	result := make([]baseline.Candidate, len(values))
	for index, value := range values {
		result[index] = baseline.Candidate{Steps: value.Steps, TraceBytes: value.TraceBytes, ErrorWords: value.ErrorWords}
	}
	return result
}

func slicesContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func zeroCostExecutionDigest(value ZeroCostExecution) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
