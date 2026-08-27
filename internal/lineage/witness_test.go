package lineage

import (
	"strings"
	"testing"
	"time"
)

func TestAuthoritativeNoInvocationWitnessRequiresClosedProcessBoundary(t *testing.T) {
	witness := validNoInvocationWitnessForTest(t)
	tests := []struct {
		name   string
		mutate func(*ExecutionWitness)
	}{
		{"non-authoritative surface", func(value *ExecutionWitness) { value.AuthoritativeSurface = false }},
		{"incomplete capture", func(value *ExecutionWitness) { value.CaptureCompleteness = "incomplete" }},
		{"execution field present", func(value *ExecutionWitness) { value.InvocationID = "spoofed" }},
		{"stream content present", func(value *ExecutionWitness) {
			value.Stdout = CapturedStream{State: StreamCaptured, ContentDigest: strings.Repeat("8", 64)}
		}},
		{"invalid capture window", func(value *ExecutionWitness) { value.CaptureWindowEndTick = value.CaptureWindowStartTick - 1 }},
		{"policy digest mutation", func(value *ExecutionWitness) { value.CapturePolicy.PolicyDigest = strings.Repeat("9", 64) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := witness
			test.mutate(&mutated)
			mutated.Header.Digest = sealWitnessForTest(t, mutated)
			if err := mutated.Validate(); err == nil {
				t.Fatal("invalid behavior-absence witness was accepted")
			}
		})
	}
}

func TestCapturePolicyRejectsIncompleteDescendantCoverageForAbsence(t *testing.T) {
	witness := validNoInvocationWitnessForTest(t)
	witness.CapturePolicy.ChildProcessCoverage = "direct_child_only"
	witness.CapturePolicy.PolicyDigest = sealCapturePolicyForTest(t, witness.CapturePolicy)
	witness.Header.Digest = sealWitnessForTest(t, witness)
	if err := witness.Validate(); err == nil {
		t.Fatal("behavior-absence policy without descendant coverage was accepted")
	}
}

func TestCapturedStreamTruncationContract(t *testing.T) {
	valid := CapturedStream{
		State: StreamTruncated, ObservedBytes: 100, RetainedBytes: 20, ContentDigest: strings.Repeat("1", 64),
		PrefixDigest: strings.Repeat("2", 64), SuffixDigest: strings.Repeat("3", 64),
	}
	if err := validateCapturedStream("stdout", valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.ObservedBytes = invalid.RetainedBytes
	if err := validateCapturedStream("stdout", invalid); err == nil {
		t.Fatal("non-lossy stream was accepted as truncated")
	}
}

func TestInvocationWitnessBindsSyntaxExitStreamsAndMonotonicInterval(t *testing.T) {
	witness := validNoInvocationWitnessForTest(t)
	witness.InvocationPresent = true
	witness.InvocationID = "invocation-1"
	witness.Argv = []string{"go", "test", "./..."}
	witness.CommandOperandsDigest = strings.Repeat("4", 64)
	witness.StartedTick = 120
	witness.EndedTick = 180
	witness.StartedAt = witness.CaptureWindowStartedAt.Add(100 * time.Millisecond)
	witness.EndedAt = witness.CaptureWindowStartedAt.Add(900 * time.Millisecond)
	witness.ExitStatusObserved = true
	witness.Stdout = CapturedStream{State: StreamCaptured, ContentDigest: strings.Repeat("5", 64)}
	witness.Header.Digest = sealWitnessForTest(t, witness)
	if err := witness.Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*ExecutionWitness)
	}{
		{"dual syntax", func(value *ExecutionWitness) { value.UnsupportedShellText = "go test ./..." }},
		{"missing exit", func(value *ExecutionWitness) { value.ExitStatusObserved = false }},
		{"tick outside capture", func(value *ExecutionWitness) { value.EndedTick = value.CaptureWindowEndTick + 1 }},
		{"echo marker trusted", func(value *ExecutionWitness) { value.EchoedSuccessMarkerTrusted = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := witness
			test.mutate(&mutated)
			mutated.Header.Digest = sealWitnessForTest(t, mutated)
			if err := mutated.Validate(); err == nil {
				t.Fatal("invalid invocation witness was accepted")
			}
		})
	}
}

func validNoInvocationWitnessForTest(t *testing.T) ExecutionWitness {
	t.Helper()
	source := validSourceForTest(t)
	policy := CapturePolicy{
		PolicyID: "process-boundary-v1", Purpose: CaptureExternalObservation, Boundary: "process_spawn_and_wait", ClockSource: "host_monotonic_plus_utc_observation",
		ClockOriginID: "boot-session-1", ClockResolutionNanos: 1, WorkingDirectoryPolicy: "repository_relative_alias",
		EnvironmentPolicy: "allowlisted_names_no_values", OutputPolicy: "separate_stream_digest_before_redaction",
		ChildProcessCoverage: "complete_descendant_tree", AuthoritativeForAbsence: true,
	}
	policy.PolicyDigest = sealCapturePolicyForTest(t, policy)
	threatIDs := []string{"agent_narration_spoofing", "clock_skew", "command_display_spoofing", "dropped_child_process", "environment_mutation", "output_truncation", "shell_wrapper_ambiguity", "state_drift"}
	threats := make([]CaptureThreat, 0, len(threatIDs))
	for _, threatID := range threatIDs {
		threats = append(threats, CaptureThreat{ThreatID: threatID, Mitigation: "process-boundary evidence", ResidualState: "mitigated"})
	}
	started := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	witness := ExecutionWitness{
		Header: ArtifactHeader{
			SchemaVersion: WitnessSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "witness-1", TaskID: "TASK-069", TaskGroupID: "group-1", DataRole: RoleAdapterDevelopment,
			PlanDigest: LockedPlanDigest,
			Parents:    []ParentRef{{Relation: "source", SchemaVersion: SourceSchemaVersion, ObjectID: source.Header.ObjectID, TaskID: "TASK-069", TaskGroupID: "group-1", Digest: source.Header.Digest}},
		},
		CapturePolicy: policy, Threats: threats, CaptureSequence: 1, CaptureWindowStartTick: 100, CaptureWindowEndTick: 200,
		CaptureWindowStartedAt: started, CaptureWindowEndedAt: started.Add(time.Second), CaptureCompleteness: "complete",
		AuthoritativeSurface: true, WorkingDirectoryAlias: "repository-1", Stdout: CapturedStream{State: StreamAbsent},
		Stderr: CapturedStream{State: StreamAbsent}, RepositoryStateObserved: true, RepositoryStateDigest: strings.Repeat("7", 64),
	}
	witness.Header.Digest = sealWitnessForTest(t, witness)
	if err := witness.Validate(); err != nil {
		t.Fatal(err)
	}
	return witness
}

func TestCapturePolicySeparatesExternalObservationFromSyntheticFixtureExecution(t *testing.T) {
	witness := validNoInvocationWitnessForTest(t)
	external := witness.CapturePolicy
	external.LaboratoryExecutesCommands = true
	external.PolicyDigest = sealCapturePolicyForTest(t, external)
	if err := validateCapturePolicy(external); err == nil {
		t.Fatal("external observation was allowed to execute captured commands")
	}
	synthetic := witness.CapturePolicy
	synthetic.Purpose = CaptureSyntheticFixtureGeneration
	synthetic.AuthoritativeForAbsence = false
	synthetic.ChildProcessCoverage = "direct_child_only"
	synthetic.LaboratoryExecutesCommands = true
	synthetic.PolicyDigest = sealCapturePolicyForTest(t, synthetic)
	if err := validateCapturePolicy(synthetic); err != nil {
		t.Fatal(err)
	}
	synthetic.LaboratoryExecutesCommands = false
	synthetic.PolicyDigest = sealCapturePolicyForTest(t, synthetic)
	if err := validateCapturePolicy(synthetic); err == nil {
		t.Fatal("synthetic fixture policy without fixed local execution was accepted")
	}
}

func sealCapturePolicyForTest(t *testing.T, policy CapturePolicy) string {
	t.Helper()
	policy.PolicyDigest = ""
	digest, err := digestJSON(policy)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func sealArtifactForTest(t *testing.T, value any) string {
	t.Helper()
	digest, err := artifactDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func sealWitnessForTest(t *testing.T, witness ExecutionWitness) string {
	t.Helper()
	witness.Header.Digest = ""
	return sealArtifactForTest(t, witness)
}
