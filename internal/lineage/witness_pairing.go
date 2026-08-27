package lineage

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const WitnessNativePairingVersion = "evalwitness.execution-witness-native-pairing.v1"

type WitnessPairingPolicy struct {
	MaximumClockSkewMillis int64 `json:"maximum_clock_skew_millis"`
}

type WitnessNativePairing struct {
	Version                   string `json:"version"`
	SourceDigest              string `json:"source_digest"`
	WitnessDigest             string `json:"witness_digest"`
	NativeRawDigest           string `json:"native_raw_digest"`
	CanonicalTrajectoryDigest string `json:"canonical_trajectory_digest"`
	FieldAccountingDigest     string `json:"field_accounting_digest"`
	InvocationID              string `json:"invocation_id"`
	ToolCallEventID           string `json:"tool_call_event_id"`
	CommandEventID            string `json:"command_event_id"`
	ToolResultEventID         string `json:"tool_result_event_id"`
	CommandOperandsDigest     string `json:"command_operands_digest"`
	RepositoryStateDigest     string `json:"repository_state_digest"`
	CallIDMatched             bool   `json:"call_id_matched"`
	ParentEdgesMatched        bool   `json:"parent_edges_matched"`
	CommandDigestMatched      bool   `json:"command_digest_matched"`
	ExitStatusMatched         bool   `json:"exit_status_matched"`
	StdoutMatched             bool   `json:"stdout_matched"`
	StderrMatched             bool   `json:"stderr_matched"`
	RepositoryBindingMatched  bool   `json:"repository_binding_matched"`
	TemporalWindowMatched     bool   `json:"temporal_window_matched"`
	TimestampOnlyCausality    bool   `json:"timestamp_only_causality"`
	Digest                    string `json:"digest"`
}

func PairExecutionWitness(source VerificationLineageSource, witness ExecutionWitness, nativeRaw []byte, policy WitnessPairingPolicy) (WitnessNativePairing, error) {
	if err := source.Validate(); err != nil {
		return WitnessNativePairing{}, fmt.Errorf("validate pairing source: %w", err)
	}
	if err := witness.Validate(); err != nil {
		return WitnessNativePairing{}, fmt.Errorf("validate pairing witness: %w", err)
	}
	if !witness.InvocationPresent || !witness.ExitStatusObserved || !witness.RepositoryStateObserved {
		return WitnessNativePairing{}, errors.New("native pairing requires invocation, exit, and repository-state observations")
	}
	if policy.MaximumClockSkewMillis < 0 || policy.MaximumClockSkewMillis > 60_000 {
		return WitnessNativePairing{}, errors.New("pairing clock skew must be between zero and 60000 milliseconds")
	}
	parent, found := parentReference(witness.Header, "source")
	if !found || parent.ObjectID != source.Header.ObjectID || parent.Digest != source.Header.Digest || parent.TaskGroupID != source.Header.TaskGroupID {
		return WitnessNativePairing{}, errors.New("witness source parent does not match the supplied source")
	}
	result, err := preprocess.ImportTraceBytes(nativeRaw, preprocess.DefaultTraceImportOptions())
	if err != nil {
		return WitnessNativePairing{}, fmt.Errorf("strict native witness import: %w", err)
	}
	accountingDigest, err := digestJSON(result.Trajectory.Report)
	if err != nil {
		return WitnessNativePairing{}, err
	}
	if source.RawRecordDigest != digestBytes(nativeRaw) || source.RawRecordCount != result.Trajectory.Report.SourceRecords ||
		source.CanonicalTrajectoryDigest != result.Trajectory.Digest || source.FieldAccountingDigest != accountingDigest ||
		source.ExportFormat != string(result.Trajectory.SourceFormat) {
		return WitnessNativePairing{}, errors.New("native bytes, canonical trajectory, or accounting differ from the source binding")
	}
	if source.RepositoryAlias != witness.WorkingDirectoryAlias {
		return WitnessNativePairing{}, errors.New("witness working directory does not match the source repository alias")
	}
	commandDigest, err := witnessCommandDigest(witness)
	if err != nil || commandDigest != witness.CommandOperandsDigest {
		return WitnessNativePairing{}, errors.New("witness command syntax differs from its operand digest")
	}
	call, command, toolResult, err := exactNativeInvocation(result.Trajectory, witness.InvocationID, witness.CommandOperandsDigest)
	if err != nil {
		return WitnessNativePairing{}, err
	}
	if toolResult.ExitCode == nil || *toolResult.ExitCode != witness.ExitStatus {
		return WitnessNativePairing{}, errors.New("native result exit status does not match the witness")
	}
	if !capturedStreamMatches(witness.Stdout, toolResult.Stdout) || !capturedStreamMatches(witness.Stderr, toolResult.Stderr) {
		return WitnessNativePairing{}, errors.New("native result streams do not match the witness")
	}
	if command.WorkingDirectoryAlias != "" && command.WorkingDirectoryAlias != witness.WorkingDirectoryAlias {
		return WitnessNativePairing{}, errors.New("native command working directory does not match the witness")
	}
	if err := nativeIntervalMatches(call.Timestamp, toolResultTimestamp(result.Trajectory, witness.InvocationID), witness, policy); err != nil {
		return WitnessNativePairing{}, err
	}
	pairing := WitnessNativePairing{
		Version: WitnessNativePairingVersion, SourceDigest: source.Header.Digest, WitnessDigest: witness.Header.Digest,
		NativeRawDigest: source.RawRecordDigest, CanonicalTrajectoryDigest: result.Trajectory.Digest,
		FieldAccountingDigest: accountingDigest, InvocationID: witness.InvocationID,
		ToolCallEventID: call.ID, CommandEventID: commandEventID(result.Trajectory, call.ID, command.OperandsDigest),
		ToolResultEventID:     toolResultEventID(result.Trajectory, call.ID, witness.InvocationID),
		CommandOperandsDigest: witness.CommandOperandsDigest, RepositoryStateDigest: witness.RepositoryStateDigest,
		CallIDMatched: true, ParentEdgesMatched: true, CommandDigestMatched: true, ExitStatusMatched: true,
		StdoutMatched: true, StderrMatched: true, RepositoryBindingMatched: true, TemporalWindowMatched: true,
		TimestampOnlyCausality: false,
	}
	pairing.Digest, err = witnessNativePairingDigest(pairing)
	if err != nil {
		return WitnessNativePairing{}, err
	}
	return pairing, pairing.Validate()
}

func (pairing WitnessNativePairing) Validate() error {
	if pairing.Version != WitnessNativePairingVersion || missing(pairing.InvocationID, pairing.ToolCallEventID, pairing.CommandEventID, pairing.ToolResultEventID) ||
		!validDigest(pairing.SourceDigest) || !validDigest(pairing.WitnessDigest) || !validDigest(pairing.NativeRawDigest) ||
		!validDigest(pairing.CanonicalTrajectoryDigest) || !validDigest(pairing.FieldAccountingDigest) ||
		!validDigest(pairing.CommandOperandsDigest) || !validDigest(pairing.RepositoryStateDigest) ||
		!pairing.CallIDMatched || !pairing.ParentEdgesMatched || !pairing.CommandDigestMatched || !pairing.ExitStatusMatched ||
		!pairing.StdoutMatched || !pairing.StderrMatched || !pairing.RepositoryBindingMatched || !pairing.TemporalWindowMatched || pairing.TimestampOnlyCausality {
		return errors.New("execution-witness native pairing is incomplete or invalid")
	}
	expected, err := witnessNativePairingDigest(pairing)
	if err != nil {
		return err
	}
	if pairing.Digest != expected {
		return errors.New("execution-witness native pairing digest is invalid")
	}
	return nil
}

func witnessCommandDigest(witness ExecutionWitness) (string, error) {
	if len(witness.Argv) > 0 && witness.UnsupportedShellText == "" {
		return digestJSON(witness.Argv)
	}
	if len(witness.Argv) == 0 && witness.UnsupportedShellText != "" {
		return digestJSON(witness.UnsupportedShellText)
	}
	return "", errors.New("witness command syntax is not exclusive")
}

func exactNativeInvocation(trajectory preprocess.Trajectory, invocationID, commandDigest string) (preprocess.Event, *preprocess.CommandPayload, *preprocess.ToolResultPayload, error) {
	calls := make([]preprocess.Event, 0, 1)
	for _, event := range trajectory.Events {
		if event.ToolCall != nil && event.ToolCall.CallID == invocationID {
			calls = append(calls, event)
		}
	}
	if len(calls) != 1 {
		return preprocess.Event{}, nil, nil, errors.New("native trajectory requires exactly one matching tool call")
	}
	call := calls[0]
	var command *preprocess.CommandPayload
	var result *preprocess.ToolResultPayload
	for _, link := range trajectory.Links {
		if link.FromID != call.ID {
			continue
		}
		event, found := eventByID(trajectory.Events, link.ToID)
		if !found {
			continue
		}
		if link.Kind == preprocess.LinkParent && event.Command != nil && event.Command.OperandsDigest == commandDigest {
			if command != nil {
				return preprocess.Event{}, nil, nil, errors.New("native trajectory has multiple matching command events")
			}
			command = event.Command
		}
		if link.Kind == preprocess.LinkCallResult && event.ToolResult != nil && event.ToolResult.CallID == invocationID {
			if result != nil {
				return preprocess.Event{}, nil, nil, errors.New("native trajectory has multiple matching result events")
			}
			result = event.ToolResult
		}
	}
	if command == nil || result == nil {
		return preprocess.Event{}, nil, nil, errors.New("native command/result causal binding is incomplete")
	}
	return call, command, result, nil
}

func capturedStreamMatches(stream CapturedStream, parts []preprocess.ContentPart) bool {
	if stream.State == StreamAbsent {
		return len(parts) == 0
	}
	content := ""
	for _, part := range parts {
		if part.Kind != preprocess.ContentText {
			return false
		}
		content += part.Text
	}
	if stream.State == StreamCaptured {
		return int64(len(content)) == stream.ObservedBytes && digestBytes([]byte(content)) == stream.ContentDigest
	}
	return int64(len(content)) == stream.ObservedBytes && digestBytes([]byte(content)) == stream.ContentDigest && stream.State == StreamTruncated
}

func nativeIntervalMatches(callTimestamp, resultTimestamp string, witness ExecutionWitness, policy WitnessPairingPolicy) error {
	callTime, err := time.Parse(time.RFC3339Nano, callTimestamp)
	if err != nil {
		return errors.New("native call timestamp is absent or invalid")
	}
	resultTime, err := time.Parse(time.RFC3339Nano, resultTimestamp)
	if err != nil {
		return errors.New("native result timestamp is absent or invalid")
	}
	skew := time.Duration(policy.MaximumClockSkewMillis) * time.Millisecond
	if callTime.Before(witness.StartedAt.Add(-skew)) || callTime.After(witness.EndedAt.Add(skew)) ||
		resultTime.Before(callTime) || resultTime.After(witness.EndedAt.Add(skew)) {
		return errors.New("native call/result timestamps fall outside the bounded witness interval")
	}
	return nil
}

func eventByID(events []preprocess.Event, eventID string) (preprocess.Event, bool) {
	index := slices.IndexFunc(events, func(event preprocess.Event) bool { return event.ID == eventID })
	if index < 0 {
		return preprocess.Event{}, false
	}
	return events[index], true
}

func commandEventID(trajectory preprocess.Trajectory, callID, digest string) string {
	for _, link := range trajectory.Links {
		if link.FromID == callID && link.Kind == preprocess.LinkParent {
			if event, found := eventByID(trajectory.Events, link.ToID); found && event.Command != nil && event.Command.OperandsDigest == digest {
				return event.ID
			}
		}
	}
	return ""
}

func toolResultEventID(trajectory preprocess.Trajectory, callID, invocationID string) string {
	for _, link := range trajectory.Links {
		if link.FromID == callID && link.Kind == preprocess.LinkCallResult {
			if event, found := eventByID(trajectory.Events, link.ToID); found && event.ToolResult != nil && event.ToolResult.CallID == invocationID {
				return event.ID
			}
		}
	}
	return ""
}

func toolResultTimestamp(trajectory preprocess.Trajectory, invocationID string) string {
	for _, event := range trajectory.Events {
		if event.ToolResult != nil && event.ToolResult.CallID == invocationID {
			return event.Timestamp
		}
	}
	return ""
}

func witnessNativePairingDigest(pairing WitnessNativePairing) (string, error) {
	pairing.Digest = ""
	return digestJSON(pairing)
}
