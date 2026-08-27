package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
)

const armEvidenceNotProvided = "supported_cell_evidence_not_provided"

type ArmCellStatus string

const (
	ArmCellExecuted    ArmCellStatus = "executed"
	ArmCellNotRun      ArmCellStatus = "not_run"
	ArmCellUnsupported ArmCellStatus = "unsupported"
)

type ArmEvidenceKind string

const (
	ArmEvidenceReplay   ArmEvidenceKind = "replay_execution"
	ArmEvidenceZeroCost ArmEvidenceKind = "zero_cost_execution"
)

type ArmComparisonObservation struct {
	CellID               string          `json:"cell_id"`
	ArmID                string          `json:"arm_id"`
	RelationID           string          `json:"relation_id"`
	RelationDigest       string          `json:"relation_digest"`
	CaseID               string          `json:"case_id"`
	TaskGroupID          string          `json:"task_group_id"`
	ManifestDigest       string          `json:"manifest_digest"`
	Support              ArmSupport      `json:"support"`
	Status               ArmCellStatus   `json:"status"`
	Outcome              Outcome         `json:"outcome,omitempty"`
	AdmissionStatus      AdmissionStatus `json:"admission_status,omitempty"`
	EvidenceKind         ArmEvidenceKind `json:"evidence_kind,omitempty"`
	ResultDigest         string          `json:"result_digest,omitempty"`
	EvidenceDigest       string          `json:"evidence_digest,omitempty"`
	ProtocolProofDigest  string          `json:"protocol_proof_digest,omitempty"`
	CapsuleDigest        string          `json:"capsule_digest,omitempty"`
	CompletedRepetitions int             `json:"completed_repetitions"`
	ProviderCalls        int             `json:"provider_calls"`
	Reason               string          `json:"reason,omitempty"`
}

type ArmComparisonReport struct {
	SchemaVersion    string                     `json:"schema_version"`
	CanonicalPolicy  string                     `json:"canonical_policy"`
	PlanDigest       string                     `json:"plan_digest"`
	Cells            []ArmComparisonObservation `json:"cells"`
	PlannedCells     int                        `json:"planned_cells"`
	ExecutedCells    int                        `json:"executed_cells"`
	NotRunCells      int                        `json:"not_run_cells"`
	UnsupportedCells int                        `json:"unsupported_cells"`
	GlobalScore      bool                       `json:"global_score"`
	Digest           string                     `json:"digest"`
}

func BuildArmComparisonReport(
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	replayEvidence []ArmReplayEvidence,
	zeroCostEvidence []ZeroCostExecution,
	protocolProof *ProtocolAdapterProof,
) (ArmComparisonReport, error) {
	value, err := buildArmComparisonReportUnchecked(plan, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof)
	if err != nil {
		return ArmComparisonReport{}, err
	}
	value.Digest, err = armComparisonReportDigest(value)
	if err != nil {
		return ArmComparisonReport{}, err
	}
	if err := value.ValidateAgainst(plan, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof); err != nil {
		return ArmComparisonReport{}, err
	}
	return value, nil
}

func (value ArmComparisonReport) Validate() error {
	if value.SchemaVersion != ArmComparisonReportSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.PlanDigest) || value.GlobalScore || value.PlannedCells != len(value.Cells) ||
		value.ExecutedCells < 0 || value.NotRunCells < 0 || value.UnsupportedCells < 0 ||
		value.ExecutedCells+value.NotRunCells+value.UnsupportedCells != value.PlannedCells {
		return errors.New("stress arm-comparison report identity, counts, or global-score boundary is invalid")
	}
	executed, notRun, unsupported := 0, 0, 0
	previousID := ""
	for index, cell := range value.Cells {
		if index > 0 && cell.CellID <= previousID {
			return errors.New("stress arm-comparison report cells are duplicated or not canonically ordered")
		}
		previousID = cell.CellID
		if err := validateArmComparisonObservation(cell); err != nil {
			return fmt.Errorf("validate stress arm-comparison cell %q: %w", cell.CellID, err)
		}
		switch cell.Status {
		case ArmCellExecuted:
			executed++
		case ArmCellNotRun:
			notRun++
		case ArmCellUnsupported:
			unsupported++
		}
	}
	if executed != value.ExecutedCells || notRun != value.NotRunCells || unsupported != value.UnsupportedCells {
		return errors.New("stress arm-comparison report status totals differ from its cells")
	}
	expectedDigest, err := armComparisonReportDigest(value)
	if err != nil || value.Digest != expectedDigest {
		return errors.New("stress arm-comparison report digest is invalid")
	}
	return nil
}

func (value ArmComparisonReport) ValidateAgainst(
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	replayEvidence []ArmReplayEvidence,
	zeroCostEvidence []ZeroCostExecution,
	protocolProof *ProtocolAdapterProof,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	want, err := buildArmComparisonReportUnchecked(plan, registry, replayed, replayEvidence, zeroCostEvidence, protocolProof)
	if err != nil {
		return err
	}
	want.Digest, err = armComparisonReportDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress arm-comparison report differs from its plan or supplied evidence")
	}
	return nil
}

func buildArmComparisonReportUnchecked(
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	replayEvidence []ArmReplayEvidence,
	zeroCostEvidence []ZeroCostExecution,
	protocolProof *ProtocolAdapterProof,
) (ArmComparisonReport, error) {
	if err := plan.ValidateAgainst(registry, replayed); err != nil {
		return ArmComparisonReport{}, err
	}
	if protocolProof != nil {
		if err := protocolProof.Validate(); err != nil {
			return ArmComparisonReport{}, err
		}
	}
	cells := make(map[string]ArmComparisonCell, len(plan.Cells))
	for _, cell := range plan.Cells {
		cells[cell.CellID] = cell
	}
	relations := make(map[string]Relation, len(registry.Relations))
	for _, relation := range registry.Relations {
		relations[relation.ID] = relation
	}
	items := make(map[string]ReplayedRelationCaseV3, len(replayed))
	for _, item := range replayed {
		items[item.CaseID] = item
	}
	observed := make(map[string]ArmComparisonObservation, len(replayEvidence)+len(zeroCostEvidence))
	for _, evidence := range replayEvidence {
		cell, exists := cells[evidence.CellID]
		if !exists || cell.Support != ArmSupported {
			return ArmComparisonReport{}, fmt.Errorf("stress arm replay evidence targets unknown or unsupported cell %q", evidence.CellID)
		}
		arm, exists := canonicalArmByID(cell.ArmID)
		if !exists || arm.Kind == ArmZeroCostControl {
			return ArmComparisonReport{}, fmt.Errorf("stress arm replay evidence targets non-replay cell %q", evidence.CellID)
		}
		spec, relationExists := relations[cell.RelationID]
		item, itemExists := items[cell.CaseID]
		if !relationExists || !itemExists || evidence.Result.Admission == nil {
			return ArmComparisonReport{}, fmt.Errorf("stress arm replay evidence cell %q lacks relation, case, or admission", evidence.CellID)
		}
		var requiredProof *ProtocolAdapterProof
		if arm.Kind == ArmProtocolAdapter {
			requiredProof = protocolProof
		}
		if err := evidence.ValidateAgainst(plan, spec, *evidence.Result.Admission, item, requiredProof); err != nil {
			return ArmComparisonReport{}, fmt.Errorf("validate stress arm replay evidence %q: %w", evidence.CellID, err)
		}
		observation := executedArmObservation(cell, evidence.Result, ArmEvidenceReplay, evidence.Digest, evidence.ProtocolProofDigest)
		if err := registerArmObservation(observed, observation); err != nil {
			return ArmComparisonReport{}, err
		}
	}
	for _, evidence := range zeroCostEvidence {
		arm, exists := canonicalArmByID(evidence.ArmID)
		if !exists || arm.Kind != ArmZeroCostControl || evidence.Result.Admission == nil {
			return ArmComparisonReport{}, fmt.Errorf("stress zero-cost evidence for case %q has no canonical arm or admission", evidence.CaseID)
		}
		item, exists := items[evidence.CaseID]
		if !exists {
			return ArmComparisonReport{}, fmt.Errorf("stress zero-cost evidence case %q is outside the locked corpus", evidence.CaseID)
		}
		var spec Relation
		foundRelation := false
		for _, candidate := range registry.Relations {
			if candidate.Digest == evidence.RelationDigest {
				spec, foundRelation = candidate, true
				break
			}
		}
		if !foundRelation {
			return ArmComparisonReport{}, fmt.Errorf("stress zero-cost evidence case %q has an unknown relation", evidence.CaseID)
		}
		cellID, err := comparisonCellID(evidence.ArmID, spec.ID, evidence.CaseID)
		if err != nil {
			return ArmComparisonReport{}, err
		}
		cell, exists := cells[cellID]
		if !exists || cell.Support != ArmSupported {
			return ArmComparisonReport{}, fmt.Errorf("stress zero-cost evidence targets unknown or unsupported cell %q", cellID)
		}
		if err := evidence.ValidateAgainst(spec, *evidence.Result.Admission, item); err != nil {
			return ArmComparisonReport{}, fmt.Errorf("validate stress zero-cost evidence %q: %w", cellID, err)
		}
		observation := executedArmObservation(cell, evidence.Result, ArmEvidenceZeroCost, evidence.Digest, "")
		if err := registerArmObservation(observed, observation); err != nil {
			return ArmComparisonReport{}, err
		}
	}
	value := ArmComparisonReport{
		SchemaVersion: ArmComparisonReportSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PlanDigest: plan.Digest, PlannedCells: len(plan.Cells), GlobalScore: false,
		Cells: make([]ArmComparisonObservation, 0, len(plan.Cells)),
	}
	for _, cell := range plan.Cells {
		if observation, exists := observed[cell.CellID]; exists {
			value.Cells = append(value.Cells, observation)
			value.ExecutedCells++
			continue
		}
		observation := plannedArmObservation(cell)
		value.Cells = append(value.Cells, observation)
		if observation.Status == ArmCellUnsupported {
			value.UnsupportedCells++
		} else {
			value.NotRunCells++
		}
	}
	return value, nil
}

func executedArmObservation(cell ArmComparisonCell, result Result, kind ArmEvidenceKind, evidenceDigest, protocolProofDigest string) ArmComparisonObservation {
	return ArmComparisonObservation{
		CellID: cell.CellID, ArmID: cell.ArmID, RelationID: cell.RelationID, RelationDigest: cell.RelationDigest,
		CaseID: cell.CaseID, TaskGroupID: cell.TaskGroupID, ManifestDigest: cell.ManifestDigest, Support: cell.Support,
		Status: ArmCellExecuted, Outcome: result.Outcome, EvidenceKind: kind, ResultDigest: result.Digest,
		EvidenceDigest: evidenceDigest, ProtocolProofDigest: protocolProofDigest,
		AdmissionStatus: result.Admission.Status, CapsuleDigest: result.CapsuleDigest,
		CompletedRepetitions: result.CompletedRepetitions, ProviderCalls: result.ProviderCalls,
	}
}

func plannedArmObservation(cell ArmComparisonCell) ArmComparisonObservation {
	status, reason := ArmCellNotRun, armEvidenceNotProvided
	if cell.Support == ArmUnsupported {
		status, reason = ArmCellUnsupported, cell.Reason
	}
	return ArmComparisonObservation{
		CellID: cell.CellID, ArmID: cell.ArmID, RelationID: cell.RelationID, RelationDigest: cell.RelationDigest,
		CaseID: cell.CaseID, TaskGroupID: cell.TaskGroupID, ManifestDigest: cell.ManifestDigest,
		Support: cell.Support, Status: status, Reason: reason,
	}
}

func registerArmObservation(observed map[string]ArmComparisonObservation, value ArmComparisonObservation) error {
	if _, exists := observed[value.CellID]; exists {
		return fmt.Errorf("stress arm-comparison cell %q has duplicate evidence", value.CellID)
	}
	observed[value.CellID] = value
	return nil
}

func validateArmComparisonObservation(value ArmComparisonObservation) error {
	if !identifierPattern.MatchString(value.CellID) || value.ArmID == "" || value.RelationID == "" || !validDigest(value.RelationDigest) ||
		value.CaseID == "" || value.TaskGroupID == "" || !validDigest(value.ManifestDigest) || value.ProviderCalls < 0 || value.CompletedRepetitions < 0 {
		return errors.New("cell identity or non-negative execution accounting is invalid")
	}
	switch value.Status {
	case ArmCellExecuted:
		if value.Support != ArmSupported || !slices.Contains([]Outcome{
			OutcomeSatisfied, OutcomeViolated, OutcomeAbstained, OutcomeInvalid, OutcomeUnsupported,
			OutcomeProviderFailed, OutcomeInconclusive,
		}, value.Outcome) || !slices.Contains([]AdmissionStatus{AdmissionFormalOnly, AdmissionHumanSupported, AdmissionHumanUnresolved}, value.AdmissionStatus) ||
			!slices.Contains([]ArmEvidenceKind{ArmEvidenceReplay, ArmEvidenceZeroCost}, value.EvidenceKind) ||
			!validDigest(value.ResultDigest) || !validDigest(value.EvidenceDigest) || value.CompletedRepetitions == 0 || value.Reason != "" {
			return errors.New("executed cell omits evidence, outcome, or repetition accounting")
		}
		if value.ProtocolProofDigest != "" && !validDigest(value.ProtocolProofDigest) || value.CapsuleDigest != "" && !validDigest(value.CapsuleDigest) {
			return errors.New("executed cell protocol-proof or capsule digest is invalid")
		}
	case ArmCellNotRun:
		if value.Support != ArmSupported || value.Reason != armEvidenceNotProvided || hasArmExecutionMaterial(value) {
			return errors.New("not-run cell contains execution material or an unregistered reason")
		}
	case ArmCellUnsupported:
		if value.Support != ArmUnsupported || value.Reason == "" || hasArmExecutionMaterial(value) {
			return errors.New("unsupported cell contains execution material or omits its reason")
		}
	default:
		return errors.New("cell status is unsupported")
	}
	return nil
}

func hasArmExecutionMaterial(value ArmComparisonObservation) bool {
	return value.Outcome != "" || value.AdmissionStatus != "" || value.EvidenceKind != "" || value.ResultDigest != "" || value.EvidenceDigest != "" ||
		value.ProtocolProofDigest != "" || value.CapsuleDigest != "" || value.CompletedRepetitions != 0 || value.ProviderCalls != 0
}

func armComparisonReportDigest(value ArmComparisonReport) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
