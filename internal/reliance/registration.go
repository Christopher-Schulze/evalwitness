package reliance

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

func SealReliancePanelRegistration(
	preregistration Preregistration,
	preflight ReliancePreflight,
	studyManifestDigest string,
	arm RelianceAnalysisArm,
	sourceTasks []RelianceSourceTaskRegistration,
) (ReliancePanelRegistration, error) {
	if err := preflight.Validate(preregistration); err != nil {
		return ReliancePanelRegistration{}, err
	}
	if preflight.Status != "resolved" || preflight.SelectedSourceTasks != RelianceSelectedSourceTasks {
		return ReliancePanelRegistration{}, errors.New("reliance panel registration requires the resolved frozen 24-task preflight")
	}
	ordered, err := canonicalRegistrationSourceTasks(sourceTasks)
	if err != nil {
		return ReliancePanelRegistration{}, err
	}
	value := ReliancePanelRegistration{
		SchemaVersion: ReliancePanelRegistrationSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		StudyManifestDigest: studyManifestDigest, PreregistrationDigest: preregistration.Digest,
		PreflightDigest: preflight.Digest, Arm: arm, AssignmentAlgorithm: ReliancePanelAssignmentAlgorithm,
		FreezeStage: RelianceRegistrationFreezeStage, ChronologyStatus: RelianceRegistrationChronologyStatus,
		SourceTasks: ordered, SourceTaskCount: len(ordered), CellsPerTask: ReferenceCellsPerTask,
		RegisteredCells:      len(ordered) * ReferenceCellsPerTask,
		PlannedLogicalCalls:  preflight.SelectedBudget.LogicalCalls,
		SealingProviderCalls: 0, SealingNetworkRequired: false,
	}
	value.Digest, err = reliancePanelRegistrationDigest(value)
	if err != nil {
		return ReliancePanelRegistration{}, err
	}
	return value, value.Validate()
}

func (value ReliancePanelRegistration) Validate() error {
	if value.SchemaVersion != ReliancePanelRegistrationSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.AssignmentAlgorithm != ReliancePanelAssignmentAlgorithm || value.FreezeStage != RelianceRegistrationFreezeStage ||
		value.ChronologyStatus != RelianceRegistrationChronologyStatus ||
		!validRelianceDigest(value.StudyManifestDigest) || !validRelianceDigest(value.PreregistrationDigest) ||
		!validRelianceDigest(value.PreflightDigest) || value.SourceTaskCount != RelianceSelectedSourceTasks ||
		value.CellsPerTask != ReferenceCellsPerTask || value.RegisteredCells != value.SourceTaskCount*value.CellsPerTask ||
		value.PlannedLogicalCalls != value.SourceTaskCount*(value.CellsPerTask+1) ||
		value.SealingProviderCalls != 0 || value.SealingNetworkRequired {
		return errors.New("reliance panel registration identity, dimensions, or freeze boundary is invalid")
	}
	if err := value.Arm.Validate(); err != nil {
		return err
	}
	ordered, err := canonicalRegistrationSourceTasks(value.SourceTasks)
	if err != nil || !slices.Equal(value.SourceTasks, ordered) {
		return errors.New("reliance panel registration source tasks are invalid or noncanonical")
	}
	digest, err := reliancePanelRegistrationDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance panel registration digest is invalid")
	}
	return nil
}

func (value RelianceAnalysisArm) Validate() error {
	validPolicy := value.EvidencePolicy == verification.EvidenceStrictVerifier ||
		value.EvidencePolicy == verification.EvidenceExplicitJudge
	routeDigest := strings.TrimPrefix(value.RouteID, "route-")
	if strings.TrimSpace(value.Entrypoint) == "" || value.Entrypoint != strings.TrimSpace(value.Entrypoint) ||
		strings.TrimSpace(value.CriterionID) == "" || value.CriterionID != strings.TrimSpace(value.CriterionID) ||
		strings.TrimSpace(value.ScoreTag) == "" || value.ScoreTag != strings.TrimSpace(value.ScoreTag) ||
		!validPolicy || strings.TrimSpace(value.ProviderID) == "" || value.ProviderID != strings.TrimSpace(value.ProviderID) ||
		!strings.HasPrefix(value.RouteID, "route-") || !validRelianceDigest(routeDigest) ||
		strings.TrimSpace(value.RequestedModel) == "" || value.RequestedModel != strings.TrimSpace(value.RequestedModel) {
		return errors.New("reliance analysis arm is invalid")
	}
	return nil
}

func canonicalRegistrationSourceTasks(
	sourceTasks []RelianceSourceTaskRegistration,
) ([]RelianceSourceTaskRegistration, error) {
	if len(sourceTasks) != RelianceSelectedSourceTasks {
		return nil, fmt.Errorf("reliance panel registration requires %d source tasks", RelianceSelectedSourceTasks)
	}
	ordered := slices.Clone(sourceTasks)
	for _, sourceTask := range ordered {
		if !validRelianceSourceTaskRegistration(sourceTask) {
			return nil, fmt.Errorf("reliance panel registration source task %q is invalid", sourceTask.SourceTaskID)
		}
	}
	slices.SortFunc(ordered, func(left, right RelianceSourceTaskRegistration) int {
		return strings.Compare(left.SourceTaskID, right.SourceTaskID)
	})
	for index := 1; index < len(ordered); index++ {
		if ordered[index].SourceTaskID == ordered[index-1].SourceTaskID {
			return nil, fmt.Errorf("reliance panel registration repeats source task %q", ordered[index].SourceTaskID)
		}
	}
	seenDigests := make(map[string]string, len(ordered)*4)
	for _, sourceTask := range ordered {
		if err := reserveSourceTaskDigests(seenDigests, sourceTask); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func validRelianceSourceTaskRegistration(value RelianceSourceTaskRegistration) bool {
	return validPanelIdentifier(value.SourceTaskID) && validRelianceDigest(value.SourceTrajectoryDigest) &&
		validRelianceDigest(value.AssignmentSetDigest) && validRelianceDigest(value.TreatmentPlanDigest) &&
		validRelianceDigest(value.OutcomeEvidenceSetDigest)
}

func reserveSourceTaskDigests(seen map[string]string, value RelianceSourceTaskRegistration) error {
	fields := []struct {
		name   string
		digest string
	}{
		{"source trajectory", value.SourceTrajectoryDigest}, {"assignment set", value.AssignmentSetDigest},
		{"treatment plan", value.TreatmentPlanDigest}, {"outcome evidence set", value.OutcomeEvidenceSetDigest},
	}
	for _, field := range fields {
		if prior, duplicate := seen[field.digest]; duplicate {
			return fmt.Errorf("reliance source tasks %q and %q repeat a %s digest", prior, value.SourceTaskID, field.name)
		}
		seen[field.digest] = value.SourceTaskID
	}
	return nil
}

func (value ReliancePanelRegistration) SourceTaskIDs() []string {
	result := make([]string, len(value.SourceTasks))
	for index, sourceTask := range value.SourceTasks {
		result[index] = sourceTask.SourceTaskID
	}
	return result
}

func (value ReliancePanelRegistration) sourceTask(sourceTaskID string) (RelianceSourceTaskRegistration, bool) {
	for _, sourceTask := range value.SourceTasks {
		if sourceTask.SourceTaskID == sourceTaskID {
			return sourceTask, true
		}
	}
	return RelianceSourceTaskRegistration{}, false
}

func SealRelianceCellFailureReceipt(
	registration ReliancePanelRegistration,
	preregistration Preregistration,
	sourceTaskID string,
	cellIndex int,
	status RelianceCellStatus,
	evidenceSchemaVersion string,
	evidenceDigest string,
	attributedLogicalCalls int,
) (RelianceCellFailureReceipt, error) {
	if err := validateRelianceAnalysisParents(registration, preregistration); err != nil {
		return RelianceCellFailureReceipt{}, err
	}
	value := RelianceCellFailureReceipt{
		SchemaVersion: RelianceCellFailureReceiptSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistrationDigest: registration.Digest, StudyManifestDigest: registration.StudyManifestDigest,
		PreregistrationDigest: preregistration.Digest, SourceTaskID: sourceTaskID, CellIndex: cellIndex,
		Levels: referenceLevels(cellIndex, canonicalReferenceMasks()), Status: status,
		EvidenceSchemaVersion: evidenceSchemaVersion, EvidenceDigest: evidenceDigest,
		AttributedLogicalCalls: attributedLogicalCalls,
	}
	digest, err := relianceCellFailureReceiptDigest(value)
	if err != nil {
		return RelianceCellFailureReceipt{}, err
	}
	value.Digest = digest
	return value, value.Validate(registration, preregistration)
}

func (value RelianceCellFailureReceipt) Validate(
	registration ReliancePanelRegistration,
	preregistration Preregistration,
) error {
	if err := validateRelianceAnalysisParents(registration, preregistration); err != nil {
		return err
	}
	if value.SchemaVersion != RelianceCellFailureReceiptSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.RegistrationDigest != registration.Digest || value.StudyManifestDigest != registration.StudyManifestDigest ||
		value.PreregistrationDigest != preregistration.Digest || !slices.Contains(registration.SourceTaskIDs(), value.SourceTaskID) ||
		value.CellIndex < 0 || value.CellIndex >= ReferenceCellsPerTask ||
		!slices.EqualFunc(value.Levels, referenceLevels(value.CellIndex, canonicalReferenceMasks()), equalFactorialLevel) ||
		!validRelianceFailureStatus(value.Status, preregistration) ||
		strings.TrimSpace(value.EvidenceSchemaVersion) == "" || value.EvidenceSchemaVersion != strings.TrimSpace(value.EvidenceSchemaVersion) ||
		!validRelianceDigest(value.EvidenceDigest) ||
		value.AttributedLogicalCalls < 0 || value.AttributedLogicalCalls > 1 {
		return errors.New("reliance cell failure receipt identity, cell, status, or evidence is invalid")
	}
	digest, err := relianceCellFailureReceiptDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance cell failure receipt digest is invalid")
	}
	return nil
}

func validateRelianceAnalysisParents(registration ReliancePanelRegistration, preregistration Preregistration) error {
	if err := validateFrozenPreregistration(preregistration); err != nil {
		return err
	}
	if err := registration.Validate(); err != nil {
		return err
	}
	if registration.PreregistrationDigest != preregistration.Digest {
		return errors.New("reliance analysis registration does not bind the frozen preregistration")
	}
	return nil
}

func validRelianceFailureStatus(status RelianceCellStatus, preregistration Preregistration) bool {
	if status == RelianceCellMeasured || status == RelianceCellAbstained {
		return false
	}
	return slices.Contains(preregistration.RetainedPostRandomization, string(status))
}

func equalFactorialLevel(left, right stats.FactorialLevel) bool {
	return left.FactorID == right.FactorID && left.Level == right.Level
}

func reliancePanelRegistrationDigest(value ReliancePanelRegistration) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}

func relianceCellFailureReceiptDigest(value RelianceCellFailureReceipt) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}

func relianceObservationID(sourceTaskID string, cellIndex int) string {
	return fmt.Sprintf("%s-cell-%02d", strings.TrimSpace(sourceTaskID), cellIndex)
}
