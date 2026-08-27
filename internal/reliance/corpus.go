package reliance

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type relianceCellKey struct {
	sourceTaskID string
	cellIndex    int
}

type relianceCorpusBuilder struct {
	registration           ReliancePanelRegistration
	preregistration        Preregistration
	cells                  map[relianceCellKey]RelianceAnalysisCell
	panelExecutions        int
	failureReceipts        int
	completedPanelCalls    int
	failureAttributedCalls int
}

func BuildRelianceAnalysisCorpus(
	registration ReliancePanelRegistration,
	preregistration Preregistration,
	executions []EvidenceTaskPanelExecution,
	failures []RelianceCellFailureReceipt,
) (RelianceAnalysisCorpus, error) {
	if err := validateRelianceAnalysisParents(registration, preregistration); err != nil {
		return RelianceAnalysisCorpus{}, err
	}
	builder := relianceCorpusBuilder{
		registration: registration, preregistration: preregistration,
		cells: make(map[relianceCellKey]RelianceAnalysisCell, registration.RegisteredCells),
	}
	if err := builder.addPanelExecutions(executions); err != nil {
		return RelianceAnalysisCorpus{}, err
	}
	if err := builder.addFailureReceipts(failures); err != nil {
		return RelianceAnalysisCorpus{}, err
	}
	return builder.build()
}

func (value RelianceAnalysisCorpus) Validate(
	registration ReliancePanelRegistration,
	preregistration Preregistration,
	executions []EvidenceTaskPanelExecution,
	failures []RelianceCellFailureReceipt,
) error {
	expected, err := BuildRelianceAnalysisCorpus(registration, preregistration, executions, failures)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("reliance analysis corpus differs from its registered panel evidence")
	}
	return nil
}

func (builder *relianceCorpusBuilder) addPanelExecutions(executions []EvidenceTaskPanelExecution) error {
	for _, execution := range executions {
		if err := validateRegisteredPanelExecution(builder.registration, execution); err != nil {
			return err
		}
		for _, cell := range execution.Cells {
			record, err := analysisCellFromPanel(execution, cell, builder.preregistration)
			if err != nil {
				return err
			}
			if err := builder.addCell(record); err != nil {
				return err
			}
		}
		builder.panelExecutions++
		builder.completedPanelCalls += execution.LogicalCalls
	}
	return nil
}

func validateRegisteredPanelExecution(registration ReliancePanelRegistration, execution EvidenceTaskPanelExecution) error {
	if err := execution.Validate(); err != nil {
		return err
	}
	arm := RelianceAnalysisArm{
		Entrypoint: execution.Entrypoint, CriterionID: execution.BaselineEvidence[0].CriterionID,
		ScoreTag:       execution.BaselineEvidence[0].Evidence.Tag,
		EvidencePolicy: execution.EvidencePolicy,
		ProviderID:     execution.ReplaySource.ProviderID, RouteID: execution.ReplaySource.RouteID,
		RequestedModel: execution.ReplaySource.RequestedModel,
	}
	sourceTask, found := registration.sourceTask(execution.SourceTaskID)
	if execution.StudyManifestDigest != registration.StudyManifestDigest ||
		execution.PreregistrationDigest != registration.PreregistrationDigest ||
		!found || !reflect.DeepEqual(arm, registration.Arm) ||
		execution.SourceTrajectoryDigest != sourceTask.SourceTrajectoryDigest ||
		execution.AssignmentSetDigest != sourceTask.AssignmentSetDigest ||
		execution.TreatmentPlanDigest != sourceTask.TreatmentPlanDigest ||
		execution.OutcomeEvidenceSetDigest != sourceTask.OutcomeEvidenceSetDigest {
		return errors.New("evidence task panel does not match the registered study, task, preregistration, or arm")
	}
	return nil
}

func analysisCellFromPanel(
	execution EvidenceTaskPanelExecution,
	cell EvidenceTaskPanelCell,
	preregistration Preregistration,
) (RelianceAnalysisCell, error) {
	status := RelianceCellMeasured
	contrast := cell.CriterionContrasts[0]
	outcomes, complete := panelOutcomeValues(cell, contrast, preregistration)
	if !complete {
		status = RelianceCellMissingScore
		outcomes = []RelianceOutcomeValue{}
	} else if cell.BaselineState == verifier.DecisionAbstained || cell.InterventionState == verifier.DecisionAbstained {
		status = RelianceCellAbstained
	}
	return RelianceAnalysisCell{
		ObservationID: relianceObservationID(execution.SourceTaskID, cell.CellIndex),
		SourceTaskID:  execution.SourceTaskID, CellIndex: cell.CellIndex, Levels: slices.Clone(cell.Levels), Status: status,
		PanelExecutionDigest: execution.Digest, InterventionCellDigest: cell.CellDigest,
		PresentationDigest: cell.PresentationDigest, ReplayEvidenceDigest: cell.Replay.ObservationSetDigest,
		OutcomeValues: outcomes,
	}, nil
}

func panelOutcomeValues(
	cell EvidenceTaskPanelCell,
	contrast PanelCriterionContrast,
	preregistration Preregistration,
) ([]RelianceOutcomeValue, bool) {
	if contrast.Comparison.ConditionalScoreMovement == nil || contrast.Comparison.ConditionalVarianceMovement == nil {
		return nil, false
	}
	values := make([]RelianceOutcomeValue, len(preregistration.PrimaryOutcomes))
	for index, outcome := range preregistration.PrimaryOutcomes {
		value, ok := panelOutcomeValue(cell, contrast.Comparison, outcome.OutcomeID)
		if !ok {
			return nil, false
		}
		values[index] = RelianceOutcomeValue{OutcomeID: outcome.OutcomeID, Value: value}
	}
	return values, true
}

func panelOutcomeValue(
	cell EvidenceTaskPanelCell,
	comparison verifier.ScoreEvidenceComparison,
	outcomeID OutcomeID,
) (float64, bool) {
	switch outcomeID {
	case OutcomeAbstentionTransition:
		return binaryReferenceOutcome(cell.AbstentionTransition), true
	case OutcomeConditionalMean:
		return optionalPanelOutcome(comparison.ConditionalScoreMovement)
	case OutcomeConditionalVariance:
		return optionalPanelOutcome(comparison.ConditionalVarianceMovement)
	case OutcomeDecisionFlip:
		return binaryReferenceOutcome(cell.DecisionFlip), true
	case OutcomeSupportJaccard:
		return comparison.SupportJaccard, true
	case OutcomeValidMass:
		return comparison.ValidScoreMassMovement, true
	case OutcomeVisibleMass:
		return comparison.VisibleMassMovement, true
	default:
		return 0, false
	}
}

func optionalPanelOutcome(value *float64) (float64, bool) {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return 0, false
	}
	return *value, true
}

func (builder *relianceCorpusBuilder) addFailureReceipts(failures []RelianceCellFailureReceipt) error {
	for _, failure := range failures {
		if err := failure.Validate(builder.registration, builder.preregistration); err != nil {
			return err
		}
		record := RelianceAnalysisCell{
			ObservationID: relianceObservationID(failure.SourceTaskID, failure.CellIndex),
			SourceTaskID:  failure.SourceTaskID, CellIndex: failure.CellIndex,
			Levels: slices.Clone(failure.Levels), Status: failure.Status,
			FailureReceiptDigest: failure.Digest, OutcomeValues: []RelianceOutcomeValue{},
		}
		if err := builder.addCell(record); err != nil {
			return err
		}
		builder.failureReceipts++
		builder.failureAttributedCalls += failure.AttributedLogicalCalls
	}
	return nil
}

func (builder *relianceCorpusBuilder) addCell(record RelianceAnalysisCell) error {
	key := relianceCellKey{record.SourceTaskID, record.CellIndex}
	if _, duplicate := builder.cells[key]; duplicate {
		return fmt.Errorf("reliance analysis repeats registered cell %s/%d", record.SourceTaskID, record.CellIndex)
	}
	builder.cells[key] = record
	return nil
}

func (builder *relianceCorpusBuilder) build() (RelianceAnalysisCorpus, error) {
	if len(builder.cells) != builder.registration.RegisteredCells {
		return RelianceAnalysisCorpus{}, fmt.Errorf(
			"reliance analysis covers %d of %d registered cells", len(builder.cells), builder.registration.RegisteredCells,
		)
	}
	cells, outcomeBearing, counts, err := builder.orderedCells()
	if err != nil {
		return RelianceAnalysisCorpus{}, err
	}
	value := RelianceAnalysisCorpus{
		SchemaVersion: RelianceAnalysisCorpusSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistrationDigest: builder.registration.Digest, StudyManifestDigest: builder.registration.StudyManifestDigest,
		PreregistrationDigest: builder.preregistration.Digest, PreflightDigest: builder.registration.PreflightDigest,
		Arm: builder.registration.Arm, SourceTasks: builder.registration.SourceTaskCount,
		RegisteredCells: builder.registration.RegisteredCells, OutcomeBearingCells: outcomeBearing,
		PanelExecutions: builder.panelExecutions, FailureReceipts: builder.failureReceipts,
		CompletedPanelLogicalCalls:    builder.completedPanelCalls,
		FailureAttributedLogicalCalls: builder.failureAttributedCalls,
		DenominatorPolicy:             builder.preregistration.Missingness.DenominatorPolicy,
		Imputation:                    builder.preregistration.Missingness.Imputation, StatusCounts: counts, Cells: cells,
	}
	value.Digest, err = relianceAnalysisCorpusDigest(value)
	if err != nil {
		return RelianceAnalysisCorpus{}, err
	}
	return value, validateRelianceAnalysisCorpusStructure(value, builder.registration, builder.preregistration)
}

func (builder *relianceCorpusBuilder) orderedCells() ([]RelianceAnalysisCell, int, []RelianceCellStatusCount, error) {
	result := make([]RelianceAnalysisCell, 0, builder.registration.RegisteredCells)
	counts := make(map[RelianceCellStatus]int)
	outcomeBearing := 0
	for _, sourceTask := range builder.registration.SourceTasks {
		sourceTaskID := sourceTask.SourceTaskID
		for cellIndex := 0; cellIndex < ReferenceCellsPerTask; cellIndex++ {
			cell, found := builder.cells[relianceCellKey{sourceTaskID, cellIndex}]
			if !found {
				return nil, 0, nil, fmt.Errorf("reliance analysis omits registered cell %s/%d", sourceTaskID, cellIndex)
			}
			result = append(result, cell)
			counts[cell.Status]++
			if len(cell.OutcomeValues) > 0 {
				outcomeBearing++
			}
		}
	}
	return result, outcomeBearing, relianceStatusCounts(counts), nil
}

func relianceStatusCounts(counts map[RelianceCellStatus]int) []RelianceCellStatusCount {
	statuses := canonicalRelianceCellStatuses()
	result := make([]RelianceCellStatusCount, len(statuses))
	for index, status := range statuses {
		result[index] = RelianceCellStatusCount{Status: status, Cells: counts[status]}
	}
	return result
}

func canonicalRelianceCellStatuses() []RelianceCellStatus {
	return []RelianceCellStatus{
		RelianceCellAbstained, RelianceCellBudgetExhausted, RelianceCellIncompletePair,
		RelianceCellInterventionInvalid, RelianceCellMeasured, RelianceCellMissingScore,
		RelianceCellOutcomeAmbiguous, RelianceCellProviderFailed, RelianceCellRelationUnresolved,
		RelianceCellRouteFailed, RelianceCellUnsupported,
	}
}

func validateRelianceAnalysisCorpusStructure(
	value RelianceAnalysisCorpus,
	registration ReliancePanelRegistration,
	preregistration Preregistration,
) error {
	if value.SchemaVersion != RelianceAnalysisCorpusSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.RegistrationDigest != registration.Digest || value.StudyManifestDigest != registration.StudyManifestDigest ||
		value.PreregistrationDigest != preregistration.Digest || value.PreflightDigest != registration.PreflightDigest ||
		!reflect.DeepEqual(value.Arm, registration.Arm) || value.SourceTasks != registration.SourceTaskCount ||
		value.RegisteredCells != registration.RegisteredCells || len(value.Cells) != registration.RegisteredCells ||
		value.DenominatorPolicy != preregistration.Missingness.DenominatorPolicy ||
		value.Imputation != preregistration.Missingness.Imputation {
		return errors.New("reliance analysis corpus identity, denominator, or arm is invalid")
	}
	if err := validateRelianceCorpusCells(value, registration, preregistration); err != nil {
		return err
	}
	digest, err := relianceAnalysisCorpusDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance analysis corpus digest is invalid")
	}
	return nil
}

func validateRelianceCorpusCells(
	value RelianceAnalysisCorpus,
	registration ReliancePanelRegistration,
	preregistration Preregistration,
) error {
	counts := make(map[RelianceCellStatus]int)
	outcomeBearing := 0
	for index, cell := range value.Cells {
		sourceTaskID := registration.SourceTasks[index/ReferenceCellsPerTask].SourceTaskID
		cellIndex := index % ReferenceCellsPerTask
		if err := validateRelianceAnalysisCell(cell, sourceTaskID, cellIndex, preregistration); err != nil {
			return err
		}
		counts[cell.Status]++
		if len(cell.OutcomeValues) > 0 {
			outcomeBearing++
		}
	}
	if outcomeBearing != value.OutcomeBearingCells || !reflect.DeepEqual(value.StatusCounts, relianceStatusCounts(counts)) ||
		value.PanelExecutions < 0 || value.PanelExecutions > registration.SourceTaskCount ||
		value.PanelExecutions*ReferenceCellsPerTask+value.FailureReceipts != registration.RegisteredCells ||
		value.CompletedPanelLogicalCalls != value.PanelExecutions*(ReferenceCellsPerTask+1) ||
		value.FailureReceipts < 0 || value.FailureReceipts > registration.RegisteredCells ||
		value.FailureAttributedLogicalCalls < 0 || value.FailureAttributedLogicalCalls > value.FailureReceipts {
		return errors.New("reliance analysis corpus counts or logical-call accounting is invalid")
	}
	return nil
}

func validateRelianceAnalysisCell(
	cell RelianceAnalysisCell,
	sourceTaskID string,
	cellIndex int,
	preregistration Preregistration,
) error {
	if cell.ObservationID != relianceObservationID(sourceTaskID, cellIndex) || cell.SourceTaskID != sourceTaskID ||
		cell.CellIndex != cellIndex || !reflect.DeepEqual(cell.Levels, referenceLevels(cellIndex, canonicalReferenceMasks())) {
		return fmt.Errorf("reliance analysis cell %s/%d identity or levels are invalid", sourceTaskID, cellIndex)
	}
	panelBacked := validRelianceDigest(cell.PanelExecutionDigest) && validRelianceDigest(cell.InterventionCellDigest) &&
		validRelianceDigest(cell.PresentationDigest) && validRelianceDigest(cell.ReplayEvidenceDigest) &&
		cell.FailureReceiptDigest == ""
	failureBacked := cell.PanelExecutionDigest == "" && cell.InterventionCellDigest == "" &&
		cell.PresentationDigest == "" && cell.ReplayEvidenceDigest == "" && validRelianceDigest(cell.FailureReceiptDigest)
	if panelBacked == failureBacked || !validRelianceCellOutcomes(cell, preregistration, panelBacked) {
		return fmt.Errorf("reliance analysis cell %s/%d evidence binding or outcomes are invalid", sourceTaskID, cellIndex)
	}
	return nil
}

func validRelianceCellOutcomes(
	cell RelianceAnalysisCell,
	preregistration Preregistration,
	panelBacked bool,
) bool {
	measured := cell.Status == RelianceCellMeasured || cell.Status == RelianceCellAbstained
	if measured != (len(cell.OutcomeValues) == len(preregistration.PrimaryOutcomes)) || measured && !panelBacked {
		return false
	}
	if !measured && len(cell.OutcomeValues) != 0 {
		return false
	}
	for index, outcome := range cell.OutcomeValues {
		if outcome.OutcomeID != preregistration.PrimaryOutcomes[index].OutcomeID ||
			math.IsNaN(outcome.Value) || math.IsInf(outcome.Value, 0) {
			return false
		}
	}
	return measured || cell.Status == RelianceCellMissingScore && panelBacked ||
		validRelianceFailureStatus(cell.Status, preregistration)
}

func relianceAnalysisCorpusDigest(value RelianceAnalysisCorpus) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}

func factorialObservationFromCell(cell RelianceAnalysisCell, outcomeIndex int) stats.FactorialObservation {
	return stats.FactorialObservation{
		ObservationID: cell.ObservationID, ClusterID: cell.SourceTaskID,
		Levels: slices.Clone(cell.Levels), Outcome: cell.OutcomeValues[outcomeIndex].Value,
	}
}
