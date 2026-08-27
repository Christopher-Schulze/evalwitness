package lineage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"time"
)

const SyntheticWitnessFixtureSchemaVersion = "evalwitness.synthetic-execution-witness-fixtures.v1"

type SyntheticFixtureBehavior string

const (
	SyntheticDirect            SyntheticFixtureBehavior = "direct"
	SyntheticStateChange       SyntheticFixtureBehavior = "state_change"
	SyntheticWrapperPropagates SyntheticFixtureBehavior = "wrapper_propagates_failure"
	SyntheticWrapperMasks      SyntheticFixtureBehavior = "wrapper_masks_failure"
)

type SyntheticFixtureCommandSpec struct {
	CaseID           string                   `json:"case_id"`
	Behavior         SyntheticFixtureBehavior `json:"behavior"`
	InnerCaseID      string                   `json:"inner_case_id"`
	Stdout           []byte                   `json:"-"`
	Stderr           []byte                   `json:"-"`
	ExitStatus       int                      `json:"exit_status"`
	StateBefore      []byte                   `json:"-"`
	StateAfter       []byte                   `json:"-"`
	FailureCondition string                   `json:"failure_condition"`
}

type SyntheticExecutionObservation struct {
	CaseID                string
	Stdout                []byte
	Stderr                []byte
	ExitStatus            int
	RepositoryStateBefore []byte
	RepositoryStateAfter  []byte
}

type SyntheticWitnessHarnessPolicy struct {
	HarnessID                  string   `json:"harness_id"`
	ExecutableAlias            string   `json:"executable_alias"`
	FixedCaseIDs               []string `json:"fixed_case_ids"`
	ArbitraryExecutableAllowed bool     `json:"arbitrary_executable_allowed"`
	ShellUsed                  bool     `json:"shell_used"`
	ProviderCallsAllowed       int      `json:"provider_calls_allowed"`
	AgentLaunchAllowed         bool     `json:"agent_launch_allowed"`
	TimeoutMillis              int      `json:"timeout_millis"`
	StreamRetentionBytes       int      `json:"stream_retention_bytes"`
	ClockProjection            string   `json:"clock_projection"`
}

type SyntheticWitnessFixture struct {
	CaseID                      string                    `json:"case_id"`
	Behavior                    SyntheticFixtureBehavior  `json:"behavior"`
	FailureCondition            string                    `json:"failure_condition"`
	RepositoryStateBeforeDigest string                    `json:"repository_state_before_digest"`
	RepositoryStateAfterDigest  string                    `json:"repository_state_after_digest"`
	RepositoryStateChanged      bool                      `json:"repository_state_changed"`
	Source                      VerificationLineageSource `json:"source"`
	Witness                     ExecutionWitness          `json:"witness"`
}

type SyntheticWitnessFixtureSet struct {
	SchemaVersion   string                        `json:"schema_version"`
	CanonicalPolicy string                        `json:"canonical_policy"`
	PlanDigest      string                        `json:"plan_digest"`
	Harness         SyntheticWitnessHarnessPolicy `json:"harness"`
	Fixtures        []SyntheticWitnessFixture     `json:"fixtures"`
	Digest          string                        `json:"digest"`
}

func SyntheticFixtureCommandSpecs() []SyntheticFixtureCommandSpec {
	return cloneSyntheticFixtureSpecs(syntheticFixtureCommandSpecs())
}

func BuildSyntheticWitnessFixtureSet(observations []SyntheticExecutionObservation) (SyntheticWitnessFixtureSet, error) {
	expectedSpecs := syntheticFixtureCommandSpecs()
	if len(observations) != len(expectedSpecs) {
		return SyntheticWitnessFixtureSet{}, errors.New("synthetic witness fixture observations are incomplete")
	}
	for index, observation := range observations {
		spec := expectedSpecs[index]
		if observation.CaseID != spec.CaseID || observation.ExitStatus != spec.ExitStatus ||
			!reflect.DeepEqual(observation.Stdout, spec.Stdout) || !reflect.DeepEqual(observation.Stderr, spec.Stderr) ||
			!reflect.DeepEqual(observation.RepositoryStateBefore, spec.StateBefore) ||
			!reflect.DeepEqual(observation.RepositoryStateAfter, spec.StateAfter) {
			return SyntheticWitnessFixtureSet{}, fmt.Errorf("synthetic witness observation %q differs from its fixed command contract", spec.CaseID)
		}
	}
	set, err := buildSyntheticWitnessFixtureSet(expectedSpecs)
	if err != nil {
		return SyntheticWitnessFixtureSet{}, err
	}
	return set, set.Validate()
}

func (set SyntheticWitnessFixtureSet) Validate() error {
	expected, err := buildSyntheticWitnessFixtureSet(syntheticFixtureCommandSpecs())
	if err != nil {
		return err
	}
	actualWithoutDigest := set
	actualWithoutDigest.Digest = ""
	expectedWithoutDigest := expected
	expectedWithoutDigest.Digest = ""
	if !reflect.DeepEqual(actualWithoutDigest, expectedWithoutDigest) {
		return errors.New("synthetic witness fixture set differs from the fixed harness contract")
	}
	if !validDigest(set.Digest) || set.Digest != expected.Digest {
		return errors.New("synthetic witness fixture set digest is invalid")
	}
	for _, fixture := range set.Fixtures {
		if err := fixture.Source.Validate(); err != nil {
			return fmt.Errorf("synthetic fixture %q source: %w", fixture.CaseID, err)
		}
		if err := fixture.Witness.Validate(); err != nil {
			return fmt.Errorf("synthetic fixture %q witness: %w", fixture.CaseID, err)
		}
	}
	return nil
}

func buildSyntheticWitnessFixtureSet(specs []SyntheticFixtureCommandSpec) (SyntheticWitnessFixtureSet, error) {
	caseIDs := make([]string, len(specs))
	for index, spec := range specs {
		caseIDs[index] = spec.CaseID
	}
	set := SyntheticWitnessFixtureSet{
		SchemaVersion:   SyntheticWitnessFixtureSchemaVersion,
		CanonicalPolicy: CanonicalPolicy,
		PlanDigest:      LockedPlanDigest,
		Harness: SyntheticWitnessHarnessPolicy{
			HarnessID: "evalwitness-fixed-process-fixture-v1", ExecutableAlias: "@evalwitness",
			FixedCaseIDs: caseIDs, TimeoutMillis: 5000, StreamRetentionBytes: 128,
			ClockProjection: "deterministic_fixture_coordinates_not_wall_clock_measurement",
		},
	}
	for index, spec := range specs {
		fixture, err := buildSyntheticWitnessFixture(spec, index+1, set.Harness)
		if err != nil {
			return SyntheticWitnessFixtureSet{}, err
		}
		set.Fixtures = append(set.Fixtures, fixture)
	}
	digest, err := syntheticWitnessFixtureSetDigest(set)
	if err != nil {
		return SyntheticWitnessFixtureSet{}, err
	}
	set.Digest = digest
	return set, nil
}

func buildSyntheticWitnessFixture(spec SyntheticFixtureCommandSpec, sequence int, harness SyntheticWitnessHarnessPolicy) (SyntheticWitnessFixture, error) {
	groupID := "synthetic-" + spec.CaseID
	argv := []string{harness.ExecutableAlias, "trace", "lineage", "fixture-child", spec.CaseID}
	beforeDigest := digestBytes(spec.StateBefore)
	afterDigest := digestBytes(spec.StateAfter)
	source := VerificationLineageSource{
		Header: ArtifactHeader{
			SchemaVersion: SourceSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "source-" + spec.CaseID, TaskID: "TASK-069", TaskGroupID: groupID, DataRole: RoleAdapterDevelopment,
			PlanDigest: LockedPlanDigest,
			Parents:    []ParentRef{{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-verification-lineage-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest}},
		},
		SourceClass: "checked_in_controls", AgentEcosystem: "evalwitness_fixture_harness", RuntimeIdentityClass: "fixed_local_subprocess",
		ProviderMetadata: "none", ExportFormat: "evalwitness_synthetic_process_observation", ExportVersion: "v1", CaptureMode: CapturePaired,
		SourceSessionID: "fixture-session-" + spec.CaseID, LineageID: "fixture-lineage-" + spec.CaseID,
		NearDuplicateID: "fixture-near-duplicate-" + spec.CaseID, RepositoryID: "evalwitness-repository", RepositoryAlias: "repository",
		TaskAlias: spec.CaseID, License: "MIT", RedistributionPermission: "public_mit", PrivacyClass: "synthetic_public",
		RedactionPolicy: "none", AuthoritativeSurface: false, RawRecordCount: 1,
	}
	rawDigest, err := digestJSON(struct {
		CaseID            string                   `json:"case_id"`
		Behavior          SyntheticFixtureBehavior `json:"behavior"`
		InnerCaseID       string                   `json:"inner_case_id"`
		ExitStatus        int                      `json:"exit_status"`
		StdoutDigest      string                   `json:"stdout_digest"`
		StderrDigest      string                   `json:"stderr_digest"`
		StateBeforeDigest string                   `json:"state_before_digest"`
		StateAfterDigest  string                   `json:"state_after_digest"`
	}{spec.CaseID, spec.Behavior, spec.InnerCaseID, spec.ExitStatus, digestBytes(spec.Stdout), digestBytes(spec.Stderr), beforeDigest, afterDigest})
	if err != nil {
		return SyntheticWitnessFixture{}, err
	}
	source.RawRecordDigest = rawDigest
	source.CanonicalTrajectoryDigest, err = digestJSON(struct {
		Argv         []string `json:"argv"`
		ExitStatus   int      `json:"exit_status"`
		StdoutDigest string   `json:"stdout_digest"`
		StderrDigest string   `json:"stderr_digest"`
	}{argv, spec.ExitStatus, digestBytes(spec.Stdout), digestBytes(spec.Stderr)})
	if err != nil {
		return SyntheticWitnessFixture{}, err
	}
	source.FieldAccountingDigest, err = digestJSON([]string{"argv", "exit_status", "repository_state", "stderr", "stdout"})
	if err != nil {
		return SyntheticWitnessFixture{}, err
	}
	source.Header.Digest, err = artifactDigest(source)
	if err != nil {
		return SyntheticWitnessFixture{}, err
	}

	policy := CapturePolicy{
		PolicyID: "synthetic-process-boundary-v1", Purpose: CaptureSyntheticFixtureGeneration,
		Boundary: "process_spawn_and_wait", ClockSource: "host_monotonic_plus_utc_observation",
		ClockOriginID: "deterministic-fixture-clock-v1", ClockResolutionNanos: 1_000_000,
		WorkingDirectoryPolicy: "repository_relative_alias", EnvironmentPolicy: "allowlisted_names_no_values",
		OutputPolicy: "separate_stream_digest_before_redaction", ChildProcessCoverage: "direct_child_only",
		LaboratoryExecutesCommands: true,
	}
	policy.PolicyDigest, err = capturePolicyDigest(policy)
	if err != nil {
		return SyntheticWitnessFixture{}, err
	}
	threats := syntheticFixtureThreats()
	started := time.Date(2026, time.August, 10, 15, 0, sequence, 0, time.UTC)
	startTick := uint64(sequence * 1000)
	commandDigest, err := digestJSON(argv)
	if err != nil {
		return SyntheticWitnessFixture{}, err
	}
	witness := ExecutionWitness{
		Header: ArtifactHeader{
			SchemaVersion: WitnessSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "witness-" + spec.CaseID, TaskID: "TASK-069", TaskGroupID: groupID, DataRole: RoleAdapterDevelopment,
			PlanDigest: LockedPlanDigest,
			Parents:    []ParentRef{{Relation: "source", SchemaVersion: SourceSchemaVersion, ObjectID: source.Header.ObjectID, TaskID: "TASK-069", TaskGroupID: groupID, Digest: source.Header.Digest}},
		},
		CapturePolicy: policy, Threats: threats, CaptureSequence: uint64(sequence),
		CaptureWindowStartTick: startTick, CaptureWindowEndTick: startTick + 100,
		CaptureWindowStartedAt: started, CaptureWindowEndedAt: started.Add(100 * time.Millisecond), CaptureCompleteness: "complete",
		InvocationPresent: true, InvocationID: "invocation-" + spec.CaseID, Argv: argv, CommandOperandsDigest: commandDigest,
		WorkingDirectoryAlias: "repository", StartedTick: startTick + 10, EndedTick: startTick + 90,
		StartedAt: started.Add(10 * time.Millisecond), EndedAt: started.Add(90 * time.Millisecond),
		ExitStatusObserved: true, ExitStatus: spec.ExitStatus,
		Stdout: capturedFixtureStream(spec.Stdout, harness.StreamRetentionBytes), Stderr: capturedFixtureStream(spec.Stderr, harness.StreamRetentionBytes),
		RepositoryStateObserved: true, RepositoryStateDigest: afterDigest,
	}
	witness.Header.Digest, err = artifactDigest(witness)
	if err != nil {
		return SyntheticWitnessFixture{}, err
	}
	return SyntheticWitnessFixture{
		CaseID: spec.CaseID, Behavior: spec.Behavior, FailureCondition: spec.FailureCondition,
		RepositoryStateBeforeDigest: beforeDigest, RepositoryStateAfterDigest: afterDigest,
		RepositoryStateChanged: beforeDigest != afterDigest, Source: source, Witness: witness,
	}, nil
}

func syntheticFixtureCommandSpecs() []SyntheticFixtureCommandSpec {
	unchanged := []byte("fixture-state-v1\n")
	return []SyntheticFixtureCommandSpec{
		{CaseID: "controlled_failure", Behavior: SyntheticDirect, Stderr: []byte("verification failed\n"), ExitStatus: 7, StateBefore: unchanged, StateAfter: unchanged, FailureCondition: "nonzero exit and decisive stderr prove controlled failure"},
		{CaseID: "mixed_stream_failure", Behavior: SyntheticDirect, Stdout: []byte("partial evidence\n"), Stderr: []byte("decisive failure\n"), ExitStatus: 3, StateBefore: unchanged, StateAfter: unchanged, FailureCondition: "nonzero exit remains decisive when stdout and stderr both exist"},
		{CaseID: "pipeline_masked_failure", Behavior: SyntheticWrapperMasks, InnerCaseID: "controlled_failure", Stdout: []byte("pipeline reported success\n"), Stderr: []byte("verification failed\n"), ExitStatus: 0, StateBefore: unchanged, StateAfter: unchanged, FailureCondition: "inner nonzero exit is masked by a zero-status wrapper"},
		{CaseID: "state_change_success", Behavior: SyntheticStateChange, Stdout: []byte("state updated\n"), ExitStatus: 0, StateBefore: []byte("fixture-state-before\n"), StateAfter: []byte("fixture-state-after\n"), FailureCondition: "state digest changes despite successful exit"},
		{CaseID: "stdout_success", Behavior: SyntheticDirect, Stdout: []byte("verification passed\n"), ExitStatus: 0, StateBefore: unchanged, StateAfter: unchanged, FailureCondition: "nonzero exit would falsify successful verification"},
		{CaseID: "truncated_output_success", Behavior: SyntheticDirect, Stdout: repeatedBytes('A', 4096), ExitStatus: 0, StateBefore: unchanged, StateAfter: unchanged, FailureCondition: "full output identity must survive bounded retention"},
		{CaseID: "wrapper_failure_propagation", Behavior: SyntheticWrapperPropagates, InnerCaseID: "controlled_failure", Stderr: []byte("verification failed\n"), ExitStatus: 7, StateBefore: unchanged, StateAfter: unchanged, FailureCondition: "wrapper must preserve the inner nonzero exit"},
	}
}

func syntheticFixtureThreats() []CaptureThreat {
	return []CaptureThreat{
		{ThreatID: "agent_narration_spoofing", Mitigation: "fixed child process emits all evidence", ResidualState: "mitigated"},
		{ThreatID: "clock_skew", Mitigation: "fixture coordinates are deterministic and make no duration claim", ResidualState: "unsupported"},
		{ThreatID: "command_display_spoofing", Mitigation: "fixed case ID selects a sealed argv alias", ResidualState: "mitigated"},
		{ThreatID: "dropped_child_process", Mitigation: "direct child is captured; wrapper internals are represented by fixed case semantics", ResidualState: "unsupported"},
		{ThreatID: "environment_mutation", Mitigation: "fixture child receives no captured environment values", ResidualState: "mitigated"},
		{ThreatID: "output_truncation", Mitigation: "full and retained stream digests are computed separately", ResidualState: "mitigated"},
		{ThreatID: "shell_wrapper_ambiguity", Mitigation: "no shell is used and wrapper behavior is a closed enum", ResidualState: "mitigated"},
		{ThreatID: "state_drift", Mitigation: "fixture state is hashed before and after execution", ResidualState: "mitigated"},
	}
}

func capturedFixtureStream(content []byte, retentionLimit int) CapturedStream {
	if len(content) == 0 {
		return CapturedStream{State: StreamAbsent}
	}
	if len(content) <= retentionLimit {
		return CapturedStream{State: StreamCaptured, ObservedBytes: int64(len(content)), RetainedBytes: int64(len(content)), ContentDigest: digestBytes(content)}
	}
	prefixLength := retentionLimit / 2
	suffixLength := retentionLimit - prefixLength
	return CapturedStream{
		State: StreamTruncated, ObservedBytes: int64(len(content)), RetainedBytes: int64(retentionLimit), ContentDigest: digestBytes(content),
		PrefixDigest: digestBytes(content[:prefixLength]), SuffixDigest: digestBytes(content[len(content)-suffixLength:]),
	}
}

func capturePolicyDigest(policy CapturePolicy) (string, error) {
	policy.PolicyDigest = ""
	return digestJSON(policy)
}

func syntheticWitnessFixtureSetDigest(set SyntheticWitnessFixtureSet) (string, error) {
	set.Digest = ""
	return digestJSON(set)
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func repeatedBytes(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func cloneSyntheticFixtureSpecs(specs []SyntheticFixtureCommandSpec) []SyntheticFixtureCommandSpec {
	cloned := make([]SyntheticFixtureCommandSpec, len(specs))
	for index, spec := range specs {
		cloned[index] = spec
		cloned[index].Stdout = append([]byte(nil), spec.Stdout...)
		cloned[index].Stderr = append([]byte(nil), spec.Stderr...)
		cloned[index].StateBefore = append([]byte(nil), spec.StateBefore...)
		cloned[index].StateAfter = append([]byte(nil), spec.StateAfter...)
	}
	return cloned
}
