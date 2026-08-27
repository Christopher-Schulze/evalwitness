package reliance

import (
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

const (
	ReliancePanelRegistrationSchemaVersion  = "evalwitness.reliance-panel-registration.v1"
	RelianceCellFailureReceiptSchemaVersion = "evalwitness.reliance-cell-failure-receipt.v1"
	RelianceAnalysisCorpusSchemaVersion     = "evalwitness.reliance-analysis-corpus.v1"
	EvidenceRelianceAnalysisSchemaVersion   = "evalwitness.evidence-reliance-analysis.v1"
	ReliancePanelAssignmentAlgorithm        = "evalwitness.reliance-walsh64-panel.v1"
	RelianceRegistrationFreezeStage         = "before_verifier_output"
	RelianceRegistrationChronologyStatus    = "declared_not_timestamp_proven"
	RelianceSelectedSourceTasks             = 24
)

type RelianceCellStatus string

const (
	RelianceCellMeasured            RelianceCellStatus = "measured"
	RelianceCellAbstained           RelianceCellStatus = "abstained"
	RelianceCellBudgetExhausted     RelianceCellStatus = "budget_exhausted"
	RelianceCellIncompletePair      RelianceCellStatus = "incomplete_pair"
	RelianceCellInterventionInvalid RelianceCellStatus = "intervention_invalid"
	RelianceCellMissingScore        RelianceCellStatus = "missing_score"
	RelianceCellOutcomeAmbiguous    RelianceCellStatus = "outcome_ambiguous"
	RelianceCellProviderFailed      RelianceCellStatus = "provider_failed"
	RelianceCellRelationUnresolved  RelianceCellStatus = "relation_unresolved"
	RelianceCellRouteFailed         RelianceCellStatus = "route_failed"
	RelianceCellUnsupported         RelianceCellStatus = "unsupported_cell"
)

type RelianceAnalysisArm struct {
	Entrypoint     string                      `json:"entrypoint"`
	CriterionID    string                      `json:"criterion_id"`
	ScoreTag       string                      `json:"score_tag"`
	EvidencePolicy verification.EvidencePolicy `json:"evidence_policy"`
	ProviderID     string                      `json:"provider_id"`
	RouteID        string                      `json:"route_id"`
	RequestedModel string                      `json:"requested_model"`
}

type RelianceSourceTaskRegistration struct {
	SourceTaskID             string `json:"source_task_id"`
	SourceTrajectoryDigest   string `json:"source_trajectory_digest"`
	AssignmentSetDigest      string `json:"assignment_set_digest"`
	TreatmentPlanDigest      string `json:"treatment_plan_digest"`
	OutcomeEvidenceSetDigest string `json:"outcome_evidence_set_digest"`
}

type ReliancePanelRegistration struct {
	SchemaVersion          string                           `json:"schema_version"`
	CanonicalPolicy        string                           `json:"canonical_policy"`
	StudyManifestDigest    string                           `json:"study_manifest_digest"`
	PreregistrationDigest  string                           `json:"preregistration_digest"`
	PreflightDigest        string                           `json:"preflight_digest"`
	Arm                    RelianceAnalysisArm              `json:"arm"`
	AssignmentAlgorithm    string                           `json:"assignment_algorithm"`
	FreezeStage            string                           `json:"freeze_stage"`
	ChronologyStatus       string                           `json:"chronology_status"`
	SourceTasks            []RelianceSourceTaskRegistration `json:"source_tasks"`
	SourceTaskCount        int                              `json:"source_task_count"`
	CellsPerTask           int                              `json:"cells_per_task"`
	RegisteredCells        int                              `json:"registered_cells"`
	PlannedLogicalCalls    int                              `json:"planned_logical_calls"`
	SealingProviderCalls   int                              `json:"sealing_provider_calls"`
	SealingNetworkRequired bool                             `json:"sealing_network_required"`
	Digest                 string                           `json:"digest"`
}

type RelianceCellFailureReceipt struct {
	SchemaVersion          string                 `json:"schema_version"`
	CanonicalPolicy        string                 `json:"canonical_policy"`
	RegistrationDigest     string                 `json:"registration_digest"`
	StudyManifestDigest    string                 `json:"study_manifest_digest"`
	PreregistrationDigest  string                 `json:"preregistration_digest"`
	SourceTaskID           string                 `json:"source_task_id"`
	CellIndex              int                    `json:"cell_index"`
	Levels                 []stats.FactorialLevel `json:"levels"`
	Status                 RelianceCellStatus     `json:"status"`
	EvidenceSchemaVersion  string                 `json:"evidence_schema_version"`
	EvidenceDigest         string                 `json:"evidence_digest"`
	AttributedLogicalCalls int                    `json:"attributed_logical_calls"`
	Digest                 string                 `json:"digest"`
}

type RelianceOutcomeValue struct {
	OutcomeID OutcomeID `json:"outcome_id"`
	Value     float64   `json:"value"`
}

type RelianceAnalysisCell struct {
	ObservationID          string                 `json:"observation_id"`
	SourceTaskID           string                 `json:"source_task_id"`
	CellIndex              int                    `json:"cell_index"`
	Levels                 []stats.FactorialLevel `json:"levels"`
	Status                 RelianceCellStatus     `json:"status"`
	PanelExecutionDigest   string                 `json:"panel_execution_digest,omitempty"`
	InterventionCellDigest string                 `json:"intervention_cell_digest,omitempty"`
	PresentationDigest     string                 `json:"presentation_digest,omitempty"`
	ReplayEvidenceDigest   string                 `json:"replay_evidence_digest,omitempty"`
	FailureReceiptDigest   string                 `json:"failure_receipt_digest,omitempty"`
	OutcomeValues          []RelianceOutcomeValue `json:"outcome_values"`
}

type RelianceCellStatusCount struct {
	Status RelianceCellStatus `json:"status"`
	Cells  int                `json:"cells"`
}

type RelianceAnalysisCorpus struct {
	SchemaVersion                 string                    `json:"schema_version"`
	CanonicalPolicy               string                    `json:"canonical_policy"`
	RegistrationDigest            string                    `json:"registration_digest"`
	StudyManifestDigest           string                    `json:"study_manifest_digest"`
	PreregistrationDigest         string                    `json:"preregistration_digest"`
	PreflightDigest               string                    `json:"preflight_digest"`
	Arm                           RelianceAnalysisArm       `json:"arm"`
	SourceTasks                   int                       `json:"source_tasks"`
	RegisteredCells               int                       `json:"registered_cells"`
	OutcomeBearingCells           int                       `json:"outcome_bearing_cells"`
	PanelExecutions               int                       `json:"panel_executions"`
	FailureReceipts               int                       `json:"failure_receipts"`
	CompletedPanelLogicalCalls    int                       `json:"completed_panel_logical_calls"`
	FailureAttributedLogicalCalls int                       `json:"failure_attributed_logical_calls"`
	DenominatorPolicy             string                    `json:"denominator_policy"`
	Imputation                    string                    `json:"imputation"`
	StatusCounts                  []RelianceCellStatusCount `json:"status_counts"`
	Cells                         []RelianceAnalysisCell    `json:"cells"`
	Digest                        string                    `json:"digest"`
}

type RelianceFitStatus string

const (
	RelianceFitMeasured     RelianceFitStatus = "measured"
	RelianceFitInconclusive RelianceFitStatus = "inconclusive"
)

type EvidenceRelianceOutcomeFit struct {
	OutcomeID            OutcomeID                    `json:"outcome_id"`
	Status               RelianceFitStatus            `json:"status"`
	RegisteredCells      int                          `json:"registered_cells"`
	EligibleObservations int                          `json:"eligible_observations"`
	ExcludedFromFit      int                          `json:"excluded_from_fit"`
	Reason               string                       `json:"reason,omitempty"`
	Fit                  *stats.ClusteredFactorialFit `json:"fit,omitempty"`
}

type EvidenceRelianceAnalysis struct {
	SchemaVersion          string                       `json:"schema_version"`
	CanonicalPolicy        string                       `json:"canonical_policy"`
	RegistrationDigest     string                       `json:"registration_digest"`
	CorpusDigest           string                       `json:"corpus_digest"`
	PreregistrationDigest  string                       `json:"preregistration_digest"`
	MultiplicityMethod     string                       `json:"multiplicity_method"`
	MultiplicityFamilySize int                          `json:"multiplicity_family_size"`
	OutcomeFits            []EvidenceRelianceOutcomeFit `json:"outcome_fits"`
	Digest                 string                       `json:"digest"`
}
