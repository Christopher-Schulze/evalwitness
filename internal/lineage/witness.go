package lineage

import (
	"errors"
	"slices"
	"time"
)

type StreamState string

type CapturePurpose string

const (
	StreamCaptured  StreamState = "captured"
	StreamTruncated StreamState = "truncated"
	StreamAbsent    StreamState = "absent"
)

const (
	CaptureExternalObservation        CapturePurpose = "external_observation"
	CaptureSyntheticFixtureGeneration CapturePurpose = "synthetic_fixture_generation"
)

type CapturePolicy struct {
	PolicyID                   string         `json:"policy_id"`
	PolicyDigest               string         `json:"policy_digest"`
	Purpose                    CapturePurpose `json:"purpose"`
	Boundary                   string         `json:"boundary"`
	ClockSource                string         `json:"clock_source"`
	ClockOriginID              string         `json:"clock_origin_id"`
	ClockResolutionNanos       int64          `json:"clock_resolution_nanos"`
	WorkingDirectoryPolicy     string         `json:"working_directory_policy"`
	EnvironmentPolicy          string         `json:"environment_policy"`
	OutputPolicy               string         `json:"output_policy"`
	ChildProcessCoverage       string         `json:"child_process_coverage"`
	AuthoritativeForAbsence    bool           `json:"authoritative_for_absence"`
	LaboratoryExecutesCommands bool           `json:"laboratory_executes_commands"`
	ProviderCallsAllowed       int            `json:"provider_calls_allowed"`
}

type CaptureThreat struct {
	ThreatID      string `json:"threat_id"`
	Mitigation    string `json:"mitigation"`
	ResidualState string `json:"residual_state"`
}

type CapturedStream struct {
	State         StreamState `json:"state"`
	ObservedBytes int64       `json:"observed_bytes"`
	RetainedBytes int64       `json:"retained_bytes"`
	ContentDigest string      `json:"content_digest"`
	PrefixDigest  string      `json:"prefix_digest"`
	SuffixDigest  string      `json:"suffix_digest"`
}

type ExecutionWitness struct {
	Header                     ArtifactHeader  `json:"header"`
	CapturePolicy              CapturePolicy   `json:"capture_policy"`
	Threats                    []CaptureThreat `json:"threats"`
	CaptureSequence            uint64          `json:"capture_sequence"`
	CaptureWindowStartTick     uint64          `json:"capture_window_start_tick"`
	CaptureWindowEndTick       uint64          `json:"capture_window_end_tick"`
	CaptureWindowStartedAt     time.Time       `json:"capture_window_started_at"`
	CaptureWindowEndedAt       time.Time       `json:"capture_window_ended_at"`
	CaptureCompleteness        string          `json:"capture_completeness"`
	AuthoritativeSurface       bool            `json:"authoritative_surface"`
	InvocationPresent          bool            `json:"invocation_present"`
	InvocationID               string          `json:"invocation_id"`
	ParentInvocationID         string          `json:"parent_invocation_id"`
	Argv                       []string        `json:"argv"`
	UnsupportedShellText       string          `json:"unsupported_shell_text"`
	CommandOperandsDigest      string          `json:"command_operands_digest"`
	WorkingDirectoryAlias      string          `json:"working_directory_alias"`
	StartedTick                uint64          `json:"started_tick"`
	EndedTick                  uint64          `json:"ended_tick"`
	StartedAt                  time.Time       `json:"started_at"`
	EndedAt                    time.Time       `json:"ended_at"`
	ExitStatusObserved         bool            `json:"exit_status_observed"`
	ExitStatus                 int             `json:"exit_status"`
	Stdout                     CapturedStream  `json:"stdout"`
	Stderr                     CapturedStream  `json:"stderr"`
	RepositoryStateObserved    bool            `json:"repository_state_observed"`
	RepositoryStateDigest      string          `json:"repository_state_digest"`
	EchoedSuccessMarkerTrusted bool            `json:"echoed_success_marker_trusted"`
}

func (witness ExecutionWitness) Validate() error {
	if err := validateHeader(witness.Header, WitnessSchemaVersion, []ParentRequirement{
		{Relation: "source", SchemaVersions: []string{SourceSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true},
	}); err != nil {
		return err
	}
	if err := validateCapturePolicy(witness.CapturePolicy); err != nil {
		return err
	}
	if err := validateCaptureThreats(witness.Threats); err != nil {
		return err
	}
	if missing(witness.WorkingDirectoryAlias, witness.CaptureCompleteness) || witness.CaptureSequence == 0 ||
		witness.CaptureWindowStartTick == 0 || witness.CaptureWindowEndTick < witness.CaptureWindowStartTick || witness.EchoedSuccessMarkerTrusted ||
		witness.CaptureWindowStartedAt.IsZero() || witness.CaptureWindowEndedAt.Before(witness.CaptureWindowStartedAt) ||
		!slices.Contains([]string{"complete", "incomplete", "interrupted"}, witness.CaptureCompleteness) {
		return errors.New("execution witness capture contract is invalid")
	}
	if !witness.InvocationPresent {
		if !witness.AuthoritativeSurface || !witness.CapturePolicy.AuthoritativeForAbsence || witness.CaptureCompleteness != "complete" ||
			witness.InvocationID != "" || witness.ParentInvocationID != "" || len(witness.Argv) != 0 || witness.UnsupportedShellText != "" ||
			witness.CommandOperandsDigest != "" || witness.StartedTick != 0 || witness.EndedTick != 0 || !witness.StartedAt.IsZero() ||
			!witness.EndedAt.IsZero() || witness.ExitStatusObserved || witness.ExitStatus != 0 || witness.Stdout.State != StreamAbsent || witness.Stderr.State != StreamAbsent {
			return errors.New("no-invocation witness requires a closed authoritative surface and no execution fields")
		}
	} else {
		if missing(witness.InvocationID) || (len(witness.Argv) == 0) == (witness.UnsupportedShellText == "") ||
			!validDigest(witness.CommandOperandsDigest) || witness.StartedTick < witness.CaptureWindowStartTick || witness.EndedTick < witness.StartedTick ||
			witness.EndedTick > witness.CaptureWindowEndTick || witness.StartedAt.Before(witness.CaptureWindowStartedAt) ||
			witness.EndedAt.Before(witness.StartedAt) || witness.EndedAt.After(witness.CaptureWindowEndedAt) {
			return errors.New("execution witness invocation identity, syntax, or interval is invalid")
		}
		if witness.CaptureCompleteness == "complete" && !witness.ExitStatusObserved {
			return errors.New("complete invocation capture requires exit status")
		}
	}
	if !witness.ExitStatusObserved && witness.ExitStatus != 0 {
		return errors.New("unobserved exit status cannot carry a value")
	}
	if witness.AuthoritativeSurface != (witness.CapturePolicy.AuthoritativeForAbsence && witness.CaptureCompleteness == "complete") {
		return errors.New("authoritative-surface claim disagrees with capture policy or completeness")
	}
	if err := validateCapturedStream("stdout", witness.Stdout); err != nil {
		return err
	}
	if err := validateCapturedStream("stderr", witness.Stderr); err != nil {
		return err
	}
	if witness.RepositoryStateObserved != (witness.RepositoryStateDigest != "") ||
		(witness.RepositoryStateObserved && !validDigest(witness.RepositoryStateDigest)) {
		return errors.New("repository state identity is invalid")
	}
	copy := witness
	copy.Header.Digest = ""
	return validateArtifactDigest(witness.Header.Digest, copy)
}

func validateCapturePolicy(policy CapturePolicy) error {
	if missing(policy.PolicyID, policy.Boundary, policy.ClockSource, policy.ClockOriginID, policy.WorkingDirectoryPolicy, policy.EnvironmentPolicy,
		policy.OutputPolicy, policy.ChildProcessCoverage) || !validDigest(policy.PolicyDigest) || policy.ClockResolutionNanos < 1 ||
		policy.Boundary != "process_spawn_and_wait" || policy.ClockSource != "host_monotonic_plus_utc_observation" ||
		policy.WorkingDirectoryPolicy != "repository_relative_alias" || policy.EnvironmentPolicy != "allowlisted_names_no_values" ||
		policy.OutputPolicy != "separate_stream_digest_before_redaction" ||
		!slices.Contains([]string{"complete_descendant_tree", "direct_child_only", "unsupported"}, policy.ChildProcessCoverage) ||
		!slices.Contains([]CapturePurpose{CaptureExternalObservation, CaptureSyntheticFixtureGeneration}, policy.Purpose) ||
		policy.ProviderCallsAllowed != 0 {
		return errors.New("execution-witness capture policy is invalid")
	}
	if policy.Purpose == CaptureExternalObservation && policy.LaboratoryExecutesCommands {
		return errors.New("external observation cannot execute captured commands during analysis")
	}
	if policy.Purpose == CaptureSyntheticFixtureGeneration && (!policy.LaboratoryExecutesCommands || policy.AuthoritativeForAbsence) {
		return errors.New("synthetic fixture generation requires fixed local execution and cannot prove behavior absence")
	}
	if policy.AuthoritativeForAbsence && policy.ChildProcessCoverage != "complete_descendant_tree" {
		return errors.New("behavior-absence authority requires complete descendant coverage")
	}
	copy := policy
	copy.PolicyDigest = ""
	expected, err := digestJSON(copy)
	if err != nil {
		return err
	}
	if policy.PolicyDigest != expected {
		return errors.New("execution-witness capture policy digest is invalid")
	}
	return nil
}

func validateCaptureThreats(threats []CaptureThreat) error {
	expected := []string{"agent_narration_spoofing", "clock_skew", "command_display_spoofing", "dropped_child_process", "environment_mutation", "output_truncation", "shell_wrapper_ambiguity", "state_drift"}
	if len(threats) != len(expected) {
		return errors.New("execution witness requires the complete threat model")
	}
	for index, threat := range threats {
		if threat.ThreatID != expected[index] || missing(threat.Mitigation) ||
			!slices.Contains([]string{"mitigated", "unresolved", "unsupported"}, threat.ResidualState) {
			return errors.New("execution-witness threat model is incomplete or out of order")
		}
	}
	return nil
}

func validateCapturedStream(name string, stream CapturedStream) error {
	if !slices.Contains([]StreamState{StreamCaptured, StreamTruncated, StreamAbsent}, stream.State) || stream.ObservedBytes < 0 || stream.RetainedBytes < 0 {
		return errors.New(name + " capture state is invalid")
	}
	switch stream.State {
	case StreamAbsent:
		if stream.ObservedBytes != 0 || stream.RetainedBytes != 0 || stream.ContentDigest != "" || stream.PrefixDigest != "" || stream.SuffixDigest != "" {
			return errors.New(name + " absent state carries content")
		}
	case StreamCaptured:
		if stream.ObservedBytes != stream.RetainedBytes || !validDigest(stream.ContentDigest) || stream.PrefixDigest != "" || stream.SuffixDigest != "" {
			return errors.New(name + " complete capture identity is invalid")
		}
	case StreamTruncated:
		if stream.ObservedBytes <= stream.RetainedBytes || stream.RetainedBytes < 1 || !validDigest(stream.ContentDigest) ||
			!validDigest(stream.PrefixDigest) || !validDigest(stream.SuffixDigest) {
			return errors.New(name + " truncation evidence is invalid")
		}
	}
	return nil
}
