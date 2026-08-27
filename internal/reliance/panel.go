package reliance

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func RunEvidenceTaskPanel(
	ctx context.Context,
	runner *stress.ReplayFirstRunner,
	parents EvidenceTaskPanelParents,
	request EvidenceTaskPanelRequest,
) (EvidenceTaskPanelExecutionResult, error) {
	if ctx == nil || runner == nil {
		return EvidenceTaskPanelExecutionResult{}, errors.New("evidence task panel requires a context and shared replay runner")
	}
	batchRequest, outcomeSetDigest, err := buildEvidenceTaskPanelBatchRequest(parents, request)
	if err != nil {
		return EvidenceTaskPanelExecutionResult{}, err
	}
	replay, err := runner.RunBatchEvidence(ctx, batchRequest)
	if err != nil {
		return EvidenceTaskPanelExecutionResult{}, fmt.Errorf("run evidence task panel: %w", err)
	}
	execution, err := constructEvidenceTaskPanelExecution(parents, request, batchRequest, outcomeSetDigest, replay)
	if err != nil {
		return EvidenceTaskPanelExecutionResult{}, err
	}
	result := EvidenceTaskPanelExecutionResult{Execution: execution, Replay: replay}
	return result, result.Validate(parents, request)
}

func (value EvidenceTaskPanelExecutionResult) Validate(parents EvidenceTaskPanelParents, request EvidenceTaskPanelRequest) error {
	batchRequest, outcomeSetDigest, err := buildEvidenceTaskPanelBatchRequest(parents, request)
	if err != nil {
		return err
	}
	if err := value.Replay.ValidateRequest(batchRequest); err != nil {
		return err
	}
	expected, err := constructEvidenceTaskPanelExecution(parents, request, batchRequest, outcomeSetDigest, value.Replay)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value.Execution, expected) {
		return errors.New("evidence task panel execution differs from its frozen cells and exact replay")
	}
	return nil
}

func buildEvidenceTaskPanelBatchRequest(parents EvidenceTaskPanelParents, request EvidenceTaskPanelRequest) (stress.ReplayBatchRequest, string, error) {
	if err := validateEvidenceTaskPanelParents(parents); err != nil {
		return stress.ReplayBatchRequest{}, "", err
	}
	if !validPanelIdentifier(request.SourceTaskID) || len(request.Cells) != ReferenceCellsPerTask {
		return stress.ReplayBatchRequest{}, "", errors.New("evidence task panel source task or cell count is invalid")
	}
	outcomeSetDigest, err := panelOutcomeEvidenceSetDigest(request)
	if err != nil {
		return stress.ReplayBatchRequest{}, "", err
	}
	items := make([]stress.ReplayBatchItemRequest, 1, len(request.Cells)+1)
	if err := validatePanelVerificationInput(request.Baseline, parents, request.SourceTaskID,
		PanelBaselineVariant, PanelBaselineLabel, outcomeSetDigest, preprocess.RenderTrajectory(parents.Parent)); err != nil {
		return stress.ReplayBatchRequest{}, "", err
	}
	items[0] = stress.ReplayBatchItemRequest{Label: PanelBaselineLabel, Input: request.Baseline}
	cellItems, err := buildEvidenceTaskPanelCells(parents, request, outcomeSetDigest)
	if err != nil {
		return stress.ReplayBatchRequest{}, "", err
	}
	items = append(items, cellItems...)
	return stress.ReplayBatchRequest{Items: items}, outcomeSetDigest, nil
}

func validateEvidenceTaskPanelParents(parents EvidenceTaskPanelParents) error {
	if err := parents.Preregistration.Validate(parents.Ontology, parents.Estimands); err != nil {
		return err
	}
	if err := parents.TreatmentPlan.Validate(parents.Ontology, parents.Estimands, parents.Assignments, parents.Parent); err != nil {
		return err
	}
	if parents.TreatmentPlan.EstimandFamily != EstimandEvidenceOnly {
		return errors.New("evidence task panel requires the evidence-only estimand")
	}
	return nil
}

func buildEvidenceTaskPanelCells(parents EvidenceTaskPanelParents, request EvidenceTaskPanelRequest, outcomeSetDigest string) ([]stress.ReplayBatchItemRequest, error) {
	items := make([]stress.ReplayBatchItemRequest, len(request.Cells))
	seenCellIDs := make(map[string]struct{}, len(request.Cells))
	for index, item := range request.Cells {
		rendered, err := validateEvidenceTaskPanelCell(parents, request.SourceTaskID, index, item)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seenCellIDs[item.Cell.Cell.CellID]; duplicate {
			return nil, fmt.Errorf("evidence task panel cell ID %q is duplicated", item.Cell.Cell.CellID)
		}
		seenCellIDs[item.Cell.Cell.CellID] = struct{}{}
		if err := validatePanelVerificationInput(item.Input, parents, request.SourceTaskID,
			PanelCellVariant, item.Cell.Cell.CellID, outcomeSetDigest, rendered); err != nil {
			return nil, err
		}
		items[index] = stress.ReplayBatchItemRequest{Label: panelCellLabel(index), Input: item.Input}
	}
	return items, nil
}

func validateEvidenceTaskPanelCell(parents EvidenceTaskPanelParents, sourceTaskID string, index int, item EvidenceTaskPanelCellRequest) (string, error) {
	if item.CellIndex != index || item.Request.CellID != item.Cell.Cell.CellID || !validPanelIdentifier(item.Cell.Cell.CellID) {
		return "", fmt.Errorf("evidence task panel cell %d has an invalid index or identity", index)
	}
	if item.Request.SourceOutcome == nil || item.Request.IntervenedOutcome == nil ||
		item.Request.SourceOutcome.TaskAlias != sourceTaskID || item.Request.IntervenedOutcome.TaskAlias != sourceTaskID {
		return "", fmt.Errorf("evidence task panel cell %d does not bind the source task outcome", index)
	}
	expected, err := constructEvidenceInterventionCellFromValidatedParents(
		parents.Ontology, parents.Estimands, parents.Assignments, parents.Preregistration,
		parents.Parent, parents.TreatmentPlan, item.Request,
	)
	if err != nil || !reflect.DeepEqual(item.Cell, expected) {
		return "", errors.New("evidence task panel cell differs from its frozen parents and level assignment")
	}
	if !item.Cell.Cell.DenominatorEligible || item.Cell.Cell.OutcomePreservation == nil {
		return "", fmt.Errorf("evidence task panel cell %d is not denominator eligible", index)
	}
	if item.Presentation.Status != PresentationOrderAvailable || !reflect.DeepEqual(item.Cell.Cell.Levels, referenceLevels(index, canonicalReferenceMasks())) {
		return "", fmt.Errorf("evidence task panel cell %d does not match the frozen Walsh row or presentation support", index)
	}
	return validateAndRenderPresentationFromValidatedCell(parents.Parent, parents.Assignments, item.Cell, item.Presentation)
}

func panelOutcomeEvidenceSetDigest(request EvidenceTaskPanelRequest) (string, error) {
	type outcomeBinding struct {
		CellID             string `json:"cell_id"`
		PreservationDigest string `json:"preservation_digest"`
	}
	bindings := make([]outcomeBinding, len(request.Cells))
	sourceDigest := ""
	for index, item := range request.Cells {
		if item.Cell.Cell.OutcomePreservation == nil {
			return "", fmt.Errorf("evidence task panel cell %d lacks outcome preservation", index)
		}
		preservation := item.Cell.Cell.OutcomePreservation
		if sourceDigest == "" {
			sourceDigest = preservation.SourceOutcomeDigest
		}
		if preservation.SourceOutcomeDigest != sourceDigest {
			return "", errors.New("evidence task panel cells do not share one source outcome")
		}
		bindings[index] = outcomeBinding{item.Cell.Cell.CellID, preservation.Digest}
	}
	return referenceJSONDigest(struct {
		SourceTaskID        string           `json:"source_task_id"`
		SourceOutcomeDigest string           `json:"source_outcome_digest"`
		Cells               []outcomeBinding `json:"cells"`
	}{request.SourceTaskID, sourceDigest, bindings})
}

func validatePanelVerificationInput(input verification.Input, parents EvidenceTaskPanelParents, sourceTaskID, variant, cellID, outcomeSetDigest, trajectory string) error {
	lineage := input.Lineage
	if input.Mode != verification.ModeAbsolute || len(input.Trajectories) != 1 || input.Trajectories[0] != trajectory ||
		len(input.Criteria) != 1 || input.Policy.NReps != 1 || input.Policy.UseSPRT {
		return fmt.Errorf("evidence task panel %s input violates the frozen one-call absolute design", cellID)
	}
	if input.StudyManifestDigest == "" || input.StudyVariant != variant || !input.DisableCache ||
		input.AuthorizationDigest != "" || input.BudgetStatePath != "" {
		return fmt.Errorf("evidence task panel %s input lacks its study or offline boundary", cellID)
	}
	if lineage.AuditCaseID != sourceTaskID || lineage.TransformationID != parents.TreatmentPlan.PlanID ||
		lineage.OutcomeEvidenceDigest != outcomeSetDigest || lineage.StudyCellID != cellID {
		return fmt.Errorf("evidence task panel %s input lineage is invalid", cellID)
	}
	return nil
}

func constructEvidenceTaskPanelExecution(parents EvidenceTaskPanelParents, request EvidenceTaskPanelRequest, batchRequest stress.ReplayBatchRequest, outcomeSetDigest string, replay stress.ReplayBatchEvidence) (EvidenceTaskPanelExecution, error) {
	if err := replay.ValidateRequest(batchRequest); err != nil {
		return EvidenceTaskPanelExecution{}, err
	}
	baseline, err := buildPanelBaselineEvidence(request.Baseline, replay.Items[0])
	if err != nil {
		return EvidenceTaskPanelExecution{}, err
	}
	baselineReplay, err := panelReplayReference(replay.Items[0])
	if err != nil {
		return EvidenceTaskPanelExecution{}, err
	}
	cells, err := buildPanelExecutionCells(request, replay, baseline)
	if err != nil {
		return EvidenceTaskPanelExecution{}, err
	}
	value := EvidenceTaskPanelExecution{
		SchemaVersion: EvidenceTaskPanelSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		SourceTaskID: request.SourceTaskID, PreregistrationDigest: parents.Preregistration.Digest,
		TreatmentPlanDigest: parents.TreatmentPlan.Digest, AssignmentSetDigest: parents.Assignments.Digest,
		SourceTrajectoryDigest: parents.Parent.Digest, StudyManifestDigest: request.Baseline.StudyManifestDigest,
		OutcomeEvidenceSetDigest: outcomeSetDigest, Entrypoint: replay.Entrypoint, EvidencePolicy: replay.EvidencePolicy,
		BatchRunFingerprint: replay.BatchRunFingerprint,
		ReplaySource:        replay.ReplaySource, BaselineReplay: baselineReplay,
		BaselineState: replay.Items[0].Replay.Result.State, BaselineEvidence: baseline,
		Cells: cells, LogicalCalls: replay.Items[0].Replay.Result.Budget.Calls, NetworkRequired: false,
	}
	if value.LogicalCalls != len(replay.Items) {
		return EvidenceTaskPanelExecution{}, fmt.Errorf("evidence task panel used %d logical calls, want %d", value.LogicalCalls, len(replay.Items))
	}
	value.Digest, err = evidenceTaskPanelDigest(value)
	return value, err
}

func buildPanelBaselineEvidence(input verification.Input, item stress.ReplayBatchItemEvidence) ([]PanelScoreEvidence, error) {
	evidence, err := absoluteCriterionEvidence(item.Replay, input.Criteria[0].ID)
	if err != nil {
		return nil, err
	}
	result := make([]PanelScoreEvidence, len(evidence))
	for repetition, value := range evidence {
		if err := validatePanelScoreEvidence(input.Policy.Evidence, value); err != nil {
			return nil, err
		}
		digest, err := referenceJSONDigest(value)
		if err != nil {
			return nil, err
		}
		result[repetition] = PanelScoreEvidence{input.Criteria[0].ID, repetition, value, digest}
	}
	return result, nil
}

func buildPanelExecutionCells(request EvidenceTaskPanelRequest, replay stress.ReplayBatchEvidence, baseline []PanelScoreEvidence) ([]EvidenceTaskPanelCell, error) {
	result := make([]EvidenceTaskPanelCell, len(request.Cells))
	for index, requested := range request.Cells {
		item := replay.Items[index+1]
		contrasts, err := buildPanelCriterionContrasts(requested.Input, item, baseline)
		if err != nil {
			return nil, err
		}
		replayReference, err := panelReplayReference(item)
		if err != nil {
			return nil, err
		}
		baselineState := replay.Items[0].Replay.Result.State
		interventionState := item.Replay.Result.State
		result[index] = EvidenceTaskPanelCell{
			CellIndex: index, CellID: requested.Cell.Cell.CellID, CellDigest: requested.Cell.Cell.Digest,
			PresentationDigest: requested.Presentation.Digest, Levels: slices.Clone(requested.Cell.Cell.Levels),
			Replay: replayReference, BaselineState: baselineState, InterventionState: interventionState,
			DecisionFlip:         baselineState != interventionState,
			AbstentionTransition: (baselineState == verifier.DecisionAbstained) != (interventionState == verifier.DecisionAbstained),
			CriterionContrasts:   contrasts,
		}
	}
	return result, nil
}

func buildPanelCriterionContrasts(input verification.Input, item stress.ReplayBatchItemEvidence, baseline []PanelScoreEvidence) ([]PanelCriterionContrast, error) {
	evidence, err := absoluteCriterionEvidence(item.Replay, input.Criteria[0].ID)
	if err != nil || len(evidence) != len(baseline) {
		return nil, errors.New("evidence task panel baseline and cell repetition counts differ")
	}
	result := make([]PanelCriterionContrast, len(evidence))
	for repetition, value := range evidence {
		if err := validatePanelScoreEvidence(input.Policy.Evidence, value); err != nil {
			return nil, err
		}
		digest, err := referenceJSONDigest(value)
		if err != nil {
			return nil, err
		}
		comparison := verifier.CompareScoreEvidence(baseline[repetition].Evidence, value)
		if err := verifier.ValidateScoreEvidenceComparison(comparison); err != nil {
			return nil, err
		}
		result[repetition] = PanelCriterionContrast{
			CriterionID: input.Criteria[0].ID, Repetition: repetition,
			BaselineEvidenceDigest: baseline[repetition].EvidenceDigest,
			InterventionEvidence:   value, InterventionEvidenceDigest: digest, Comparison: comparison,
		}
	}
	return result, nil
}

func absoluteCriterionEvidence(side stress.ReplaySide, criterionID string) ([]verifier.ScoreEvidence, error) {
	if side.Result.Absolute == nil || side.Result.Delta != nil || side.Result.Selection != nil {
		return nil, errors.New("evidence task panel replay is not an absolute result")
	}
	evidence, found := side.Result.Absolute.CriterionEvidence[criterionID]
	if !found || len(evidence) != 1 {
		return nil, fmt.Errorf("evidence task panel criterion %q lacks its one frozen repetition", criterionID)
	}
	return evidence, nil
}

func validatePanelScoreEvidence(policy verification.EvidencePolicy, value verifier.ScoreEvidence) error {
	evidence := map[string]verifier.ScoreEvidence{value.Tag: value}
	switch policy {
	case verification.EvidenceStrictVerifier:
		return verifier.ValidateStrictEvidence(evidence)
	case verification.EvidenceExplicitJudge:
		return verifier.ValidateJudgeEvidence(evidence)
	default:
		return fmt.Errorf("evidence task panel has unsupported evidence policy %q", policy)
	}
}

func panelReplayReference(item stress.ReplayBatchItemEvidence) (PanelReplayReference, error) {
	observationDigest, err := referenceJSONDigest(item.Replay.Observations)
	if err != nil {
		return PanelReplayReference{}, err
	}
	return PanelReplayReference{
		InputDigest: item.InputDigest, PlanFingerprint: item.Replay.PlanFingerprint,
		ObservationSetDigest: observationDigest, StageTraceDigest: item.Replay.StageTrace.Digest,
	}, nil
}

func panelCellLabel(index int) string { return fmt.Sprintf("cell-%02d", index) }

func validPanelIdentifier(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= 128 &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

func evidenceTaskPanelDigest(value EvidenceTaskPanelExecution) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
