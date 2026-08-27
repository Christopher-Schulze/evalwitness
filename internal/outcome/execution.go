package outcome

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const ExecutionSchemaVersion = "evalwitness.outcome-executable-log.v1"

type ExecutionLog struct {
	SchemaVersion       string                   `json:"schema_version"`
	CanonicalPolicy     string                   `json:"canonical_policy"`
	TaskAlias           string                   `json:"task_alias"`
	EnvironmentID       string                   `json:"environment_id"`
	EnvironmentRevision string                   `json:"environment_revision"`
	ValidatorID         string                   `json:"validator_id"`
	ValidatorVersion    string                   `json:"validator_version"`
	ContractDigest      string                   `json:"contract_digest"`
	ObservedAt          string                   `json:"observed_at"`
	ExitCode            int                      `json:"exit_code"`
	OutputDigest        string                   `json:"output_digest"`
	OutputBytes         int                      `json:"output_bytes"`
	TimedOut            bool                     `json:"timed_out"`
	Failure             mutation.HermeticFailure `json:"failure,omitempty"`
	FailureDetailDigest string                   `json:"failure_detail_digest,omitempty"`
	Outcome             State                    `json:"outcome"`
	Limitation          string                   `json:"limitation"`
	Digest              string                   `json:"digest"`
}

func RunIndependentOutcome(ctx context.Context, registry *mutation.HermeticRegistry, taskAlias string, validator mutation.ValidatorSpec, environment mutation.TaskEnvironment, observedAt, limitation string) (ExecutionLog, error) {
	if missing(taskAlias, observedAt, limitation) {
		return ExecutionLog{}, errors.New("independent outcome execution requires task alias, observation time, and limitation")
	}
	execution, runErr := registry.Execute(ctx, validator, environment)
	if runErr != nil && execution.Failure == "" {
		return ExecutionLog{}, runErr
	}
	state := StateUnsolved
	if execution.Failure != "" {
		state = StateEnvironmentFail
	} else if execution.Passed {
		state = StateSolved
	}
	return SealExecutionLog(ExecutionLog{
		TaskAlias: taskAlias, EnvironmentID: environment.ID, EnvironmentRevision: environment.Revision,
		ValidatorID: validator.ID, ValidatorVersion: validator.Version, ContractDigest: validator.ContractDigest,
		ObservedAt: observedAt, ExitCode: execution.ExitCode, OutputDigest: execution.OutputDigest, OutputBytes: execution.OutputBytes,
		TimedOut: execution.TimedOut, Failure: execution.Failure, FailureDetailDigest: execution.FailureDetailDigest,
		Outcome: state, Limitation: limitation,
	})
}

func SealExecutionLog(log ExecutionLog) (ExecutionLog, error) {
	log.SchemaVersion = ExecutionSchemaVersion
	log.CanonicalPolicy = CanonicalPolicy
	log.Digest = ""
	digest, err := executionLogDigest(log)
	if err != nil {
		return ExecutionLog{}, err
	}
	log.Digest = digest
	return log, log.Validate()
}

func (log ExecutionLog) Validate() error {
	if log.SchemaVersion != ExecutionSchemaVersion || log.CanonicalPolicy != CanonicalPolicy ||
		missing(log.TaskAlias, log.EnvironmentID, log.EnvironmentRevision, log.ValidatorID, log.ValidatorVersion, log.ObservedAt, log.Limitation) ||
		!validDigest(log.ContractDigest) || !validDigest(log.OutputDigest) || log.OutputBytes < 0 || !validState(log.Outcome) || log.Outcome == StateNotAdjudicated {
		return errors.New("executable outcome log identity, environment, validator, output, outcome, or limitation is invalid")
	}
	if _, err := time.Parse(time.RFC3339, log.ObservedAt); err != nil {
		return errors.New("executable outcome timestamp must be RFC3339")
	}
	validFailures := []mutation.HermeticFailure{
		mutation.HermeticFailureTimeout, mutation.HermeticFailureOutputLimit,
		mutation.HermeticFailureExecution, mutation.HermeticFailureCleanup,
	}
	if log.Failure == "" {
		if log.FailureDetailDigest != "" || log.TimedOut || !slices.Contains([]State{StateSolved, StateUnsolved}, log.Outcome) {
			return errors.New("successful executable outcome contains infrastructure-failure state")
		}
	} else if !slices.Contains(validFailures, log.Failure) || !validDigest(log.FailureDetailDigest) || log.Outcome != StateEnvironmentFail || log.TimedOut != (log.Failure == mutation.HermeticFailureTimeout) {
		return errors.New("executable outcome infrastructure failure is inconsistent")
	}
	expected, err := executionLogDigest(log)
	if err != nil || log.Digest != expected {
		return errors.New("executable outcome log digest is invalid")
	}
	return nil
}

func EvidenceFromExecution(log ExecutionLog, evidenceID string, public bool) (Evidence, error) {
	if err := log.Validate(); err != nil {
		return Evidence{}, err
	}
	if missing(evidenceID) {
		return Evidence{}, fmt.Errorf("executable outcome evidence ID is required")
	}
	return SealEvidence(Evidence{
		ID: evidenceID, Kind: EvidenceIndependentRun, State: log.Outcome, ArtifactDigest: log.Digest,
		ValidatorID: log.ValidatorID + "@" + log.ValidatorVersion, ObservedAt: log.ObservedAt,
		Independent: true, Public: public, Limitation: log.Limitation, ParentDigests: []string{},
	})
}

func DecodeExecutionLog(reader io.Reader) (ExecutionLog, error) {
	var value ExecutionLog
	if err := decodeStrict(reader, &value); err != nil {
		return ExecutionLog{}, fmt.Errorf("decode executable outcome log: %w", err)
	}
	return value, value.Validate()
}

func executionLogDigest(value ExecutionLog) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
