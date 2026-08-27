package reliance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func (value EvidenceTaskPanelExecution) Validate() error {
	if value.SchemaVersion != EvidenceTaskPanelSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validPanelIdentifier(value.SourceTaskID) || strings.TrimSpace(value.Entrypoint) == "" || value.LogicalCalls != ReferenceCellsPerTask+1 ||
		value.NetworkRequired || len(value.Cells) != ReferenceCellsPerTask || len(value.BaselineEvidence) != 1 {
		return errors.New("evidence task panel identity, dimensions, or execution boundary is invalid")
	}
	for _, digest := range []string{
		value.PreregistrationDigest, value.TreatmentPlanDigest, value.AssignmentSetDigest,
		value.SourceTrajectoryDigest, value.StudyManifestDigest, value.OutcomeEvidenceSetDigest,
		value.BatchRunFingerprint,
	} {
		if !validRelianceDigest(digest) {
			return errors.New("evidence task panel contains an invalid parent or execution digest")
		}
	}
	if err := value.ReplaySource.Validate(); err != nil {
		return err
	}
	if err := validatePanelBaselineArtifact(value); err != nil {
		return err
	}
	if err := validatePanelCellArtifacts(value); err != nil {
		return err
	}
	digest, err := evidenceTaskPanelDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("evidence task panel digest is invalid")
	}
	return nil
}

func validatePanelBaselineArtifact(value EvidenceTaskPanelExecution) error {
	if !validPanelDecisionState(value.BaselineState) || validatePanelReplayReference(value.BaselineReplay) != nil {
		return errors.New("evidence task panel baseline state or replay reference is invalid")
	}
	evidence := value.BaselineEvidence[0]
	if strings.TrimSpace(evidence.CriterionID) == "" || evidence.Repetition != 0 || !validRelianceDigest(evidence.EvidenceDigest) {
		return errors.New("evidence task panel baseline evidence identity is invalid")
	}
	if err := validatePanelScoreEvidence(value.EvidencePolicy, evidence.Evidence); err != nil {
		return err
	}
	digest, err := referenceJSONDigest(evidence.Evidence)
	if err != nil || evidence.EvidenceDigest != digest {
		return errors.New("evidence task panel baseline evidence digest is invalid")
	}
	return nil
}

func validatePanelCellArtifacts(value EvidenceTaskPanelExecution) error {
	seen := make(map[string]struct{}, len(value.Cells))
	baseline := value.BaselineEvidence[0]
	for index, cell := range value.Cells {
		if cell.CellIndex != index || !validPanelIdentifier(cell.CellID) || !validRelianceDigest(cell.CellDigest) ||
			!validRelianceDigest(cell.PresentationDigest) || !reflect.DeepEqual(cell.Levels, referenceLevels(index, canonicalReferenceMasks())) ||
			cell.BaselineState != value.BaselineState || !validPanelDecisionState(cell.InterventionState) || len(cell.CriterionContrasts) != 1 {
			return fmt.Errorf("evidence task panel cell %d identity, levels, state, or contrast count is invalid", index)
		}
		if _, duplicate := seen[cell.CellID]; duplicate {
			return fmt.Errorf("evidence task panel cell ID %q is duplicated", cell.CellID)
		}
		seen[cell.CellID] = struct{}{}
		if err := validatePanelCellState(cell); err != nil {
			return err
		}
		if err := validatePanelReplayReference(cell.Replay); err != nil {
			return err
		}
		if err := validatePanelCriterionContrast(value.EvidencePolicy, baseline, cell.CriterionContrasts[0]); err != nil {
			return err
		}
	}
	return nil
}

func validatePanelCellState(cell EvidenceTaskPanelCell) error {
	wantFlip := cell.BaselineState != cell.InterventionState
	wantAbstention := (cell.BaselineState == verifier.DecisionAbstained) !=
		(cell.InterventionState == verifier.DecisionAbstained)
	if cell.DecisionFlip != wantFlip || cell.AbstentionTransition != wantAbstention {
		return fmt.Errorf("evidence task panel cell %d transition flags are invalid", cell.CellIndex)
	}
	return nil
}

func validatePanelCriterionContrast(policy verification.EvidencePolicy, baseline PanelScoreEvidence, contrast PanelCriterionContrast) error {
	if contrast.CriterionID != baseline.CriterionID || contrast.Repetition != baseline.Repetition ||
		contrast.BaselineEvidenceDigest != baseline.EvidenceDigest || !validRelianceDigest(contrast.InterventionEvidenceDigest) {
		return errors.New("evidence task panel criterion contrast identity is invalid")
	}
	if err := validatePanelScoreEvidence(policy, contrast.InterventionEvidence); err != nil {
		return err
	}
	digest, err := referenceJSONDigest(contrast.InterventionEvidence)
	if err != nil || contrast.InterventionEvidenceDigest != digest {
		return errors.New("evidence task panel intervention evidence digest is invalid")
	}
	expected := verifier.CompareScoreEvidence(baseline.Evidence, contrast.InterventionEvidence)
	if !reflect.DeepEqual(contrast.Comparison, expected) || verifier.ValidateScoreEvidenceComparison(contrast.Comparison) != nil {
		return errors.New("evidence task panel score-evidence comparison is invalid")
	}
	return nil
}

func validatePanelReplayReference(value PanelReplayReference) error {
	for _, digest := range []string{value.InputDigest, value.PlanFingerprint, value.ObservationSetDigest, value.StageTraceDigest} {
		if !validRelianceDigest(digest) {
			return errors.New("evidence task panel replay reference digest is invalid")
		}
	}
	return nil
}

func validPanelDecisionState(value verifier.DecisionState) bool {
	return value == verifier.DecisionSelected || value == verifier.DecisionTied || value == verifier.DecisionAbstained
}

func validRelianceDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}
