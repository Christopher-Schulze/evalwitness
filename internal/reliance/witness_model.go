package reliance

import (
	"context"

	"github.com/Christopher-Schulze/evalwitness/internal/stress"
)

const (
	RelianceWitnessSchemaVersion           = "evalwitness.reliance-witness.v1"
	RelianceWitnessEvaluationSchemaVersion = "evalwitness.reliance-witness-evaluation.v1"
	RelianceWitnessEvaluationPolicy        = "evalwitness.reliance-witness-preservation.v1"
)

type RelianceWitnessEvaluationStatus string

const (
	RelianceWitnessPreserved  RelianceWitnessEvaluationStatus = "preserved"
	RelianceWitnessUnresolved RelianceWitnessEvaluationStatus = "unresolved"
)

type RelianceWitnessUnresolvedReason string

const (
	WitnessInterventionInvalid RelianceWitnessUnresolvedReason = "intervention_invalid"
	WitnessOutcomeChanged      RelianceWitnessUnresolvedReason = "outcome_identity_changed"
	WitnessPrivacyUnresolved   RelianceWitnessUnresolvedReason = "privacy_not_revalidated"
	WitnessRelationUnresolved  RelianceWitnessUnresolvedReason = "relation_not_revalidated"
	WitnessResultChanged       RelianceWitnessUnresolvedReason = "reliance_result_changed"
)

type RelianceWitnessOracleObservation struct {
	ReductionObservation       stress.ReductionObservation
	ReplayBatchFingerprint     string
	ReplayCaptureDigest        string
	InterventionValidityDigest string
	InterventionValid          bool
	OutcomeIdentityDigest      string
	RelianceResultDigest       string
	NetworkRequired            bool
}

type RelianceWitnessOracle interface {
	EvaluateReliance(context.Context, stress.ReducibleInput) (RelianceWitnessOracleObservation, error)
}

type RelianceWitnessReductionRequest struct {
	ExecutionRequest     RelationExecutionRequest
	Execution            RelationExecutionResult
	PrivacyPolicyDigest  string
	PublicReleaseAllowed bool
	Input                stress.ReducibleInput
	Oracle               RelianceWitnessOracle
	MaximumEvaluations   int
}

type RelianceWitnessEvaluation struct {
	SchemaVersion              string                            `json:"schema_version"`
	CanonicalPolicy            string                            `json:"canonical_policy"`
	EvaluationPolicy           string                            `json:"evaluation_policy"`
	InputDigest                string                            `json:"input_digest"`
	ReplayBatchFingerprint     string                            `json:"replay_batch_fingerprint"`
	ReplayCaptureDigest        string                            `json:"replay_capture_digest"`
	InterventionValidityDigest string                            `json:"intervention_validity_digest"`
	InterventionValid          bool                              `json:"intervention_valid"`
	OutcomeIdentityDigest      string                            `json:"outcome_identity_digest"`
	RelianceResultDigest       string                            `json:"reliance_result_digest"`
	Status                     RelianceWitnessEvaluationStatus   `json:"status"`
	UnresolvedReasons          []RelianceWitnessUnresolvedReason `json:"unresolved_reasons"`
	ReductionObservation       stress.ReductionObservation       `json:"reduction_observation"`
	NetworkRequired            bool                              `json:"network_required"`
	Digest                     string                            `json:"digest"`
}

type RelianceWitness struct {
	SchemaVersion                string                      `json:"schema_version"`
	CanonicalPolicy              string                      `json:"canonical_policy"`
	EvaluationPolicy             string                      `json:"evaluation_policy"`
	AdmissionDigest              string                      `json:"admission_digest"`
	RelationDigest               string                      `json:"relation_digest"`
	CaseID                       string                      `json:"case_id"`
	SourceReplayDigest           string                      `json:"source_replay_digest"`
	SourceBatchRunFingerprint    string                      `json:"source_batch_run_fingerprint"`
	SourceReplayCaptureDigest    string                      `json:"source_replay_capture_digest"`
	OutcomeIdentityDigest        string                      `json:"outcome_identity_digest"`
	OriginalRelianceResultDigest string                      `json:"original_reliance_result_digest"`
	Counterexample               stress.Counterexample       `json:"counterexample"`
	Evaluations                  []RelianceWitnessEvaluation `json:"evaluations"`
	FinalEvaluationDigest        string                      `json:"final_evaluation_digest"`
	FinalUnits                   []stress.ReductionUnit      `json:"final_units"`
	PublicReleaseAllowed         bool                        `json:"public_release_allowed"`
	NetworkRequired              bool                        `json:"network_required"`
	Digest                       string                      `json:"digest"`
}
