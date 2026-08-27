package reliance

import (
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const (
	EvidenceTaskPanelSchemaVersion = "evalwitness.evidence-intervention-task-panel.v1"
	PanelBaselineLabel             = "baseline"
	PanelBaselineVariant           = "baseline"
	PanelCellVariant               = "factorial-cell"
)

type EvidenceTaskPanelParents struct {
	Ontology        FactorOntology
	Estimands       EstimandCatalog
	Assignments     FactorAssignmentSet
	Preregistration Preregistration
	Parent          preprocess.Trajectory
	TreatmentPlan   FactorTreatmentPlan
}

type EvidenceTaskPanelCellRequest struct {
	CellIndex    int
	Request      EvidenceInterventionCellRequest
	Cell         EvidenceInterventionCellResult
	Presentation PresentationOrderPlan
	Input        verification.Input
}

type EvidenceTaskPanelRequest struct {
	SourceTaskID string
	Baseline     verification.Input
	Cells        []EvidenceTaskPanelCellRequest
}

type PanelReplayReference struct {
	InputDigest          string `json:"input_digest"`
	PlanFingerprint      string `json:"plan_fingerprint"`
	ObservationSetDigest string `json:"observation_set_digest"`
	StageTraceDigest     string `json:"stage_trace_digest"`
}

type PanelScoreEvidence struct {
	CriterionID    string                 `json:"criterion_id"`
	Repetition     int                    `json:"repetition"`
	Evidence       verifier.ScoreEvidence `json:"evidence"`
	EvidenceDigest string                 `json:"evidence_digest"`
}

type PanelCriterionContrast struct {
	CriterionID                string                           `json:"criterion_id"`
	Repetition                 int                              `json:"repetition"`
	BaselineEvidenceDigest     string                           `json:"baseline_evidence_digest"`
	InterventionEvidence       verifier.ScoreEvidence           `json:"intervention_evidence"`
	InterventionEvidenceDigest string                           `json:"intervention_evidence_digest"`
	Comparison                 verifier.ScoreEvidenceComparison `json:"comparison"`
}

type EvidenceTaskPanelCell struct {
	CellIndex            int                      `json:"cell_index"`
	CellID               string                   `json:"cell_id"`
	CellDigest           string                   `json:"cell_digest"`
	PresentationDigest   string                   `json:"presentation_digest"`
	Levels               []stats.FactorialLevel   `json:"levels"`
	Replay               PanelReplayReference     `json:"replay"`
	BaselineState        verifier.DecisionState   `json:"baseline_state"`
	InterventionState    verifier.DecisionState   `json:"intervention_state"`
	DecisionFlip         bool                     `json:"decision_flip"`
	AbstentionTransition bool                     `json:"abstention_transition"`
	CriterionContrasts   []PanelCriterionContrast `json:"criterion_contrasts"`
}

type EvidenceTaskPanelExecution struct {
	SchemaVersion            string                      `json:"schema_version"`
	CanonicalPolicy          string                      `json:"canonical_policy"`
	SourceTaskID             string                      `json:"source_task_id"`
	PreregistrationDigest    string                      `json:"preregistration_digest"`
	TreatmentPlanDigest      string                      `json:"treatment_plan_digest"`
	AssignmentSetDigest      string                      `json:"assignment_set_digest"`
	SourceTrajectoryDigest   string                      `json:"source_trajectory_digest"`
	StudyManifestDigest      string                      `json:"study_manifest_digest"`
	OutcomeEvidenceSetDigest string                      `json:"outcome_evidence_set_digest"`
	Entrypoint               string                      `json:"entrypoint"`
	EvidencePolicy           verification.EvidencePolicy `json:"evidence_policy"`
	BatchRunFingerprint      string                      `json:"batch_run_fingerprint"`
	ReplaySource             provider.ExactReplaySource  `json:"replay_source"`
	BaselineReplay           PanelReplayReference        `json:"baseline_replay"`
	BaselineState            verifier.DecisionState      `json:"baseline_state"`
	BaselineEvidence         []PanelScoreEvidence        `json:"baseline_evidence"`
	Cells                    []EvidenceTaskPanelCell     `json:"cells"`
	LogicalCalls             int                         `json:"logical_calls"`
	NetworkRequired          bool                        `json:"network_required"`
	Digest                   string                      `json:"digest"`
}

type EvidenceTaskPanelExecutionResult struct {
	Execution EvidenceTaskPanelExecution
	Replay    stress.ReplayBatchEvidence
}
