package lineage

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestExecutionWitnessPairsOnlyThroughExactNativeEvidence(t *testing.T) {
	witness := stdoutSuccessWitnessForPairing(t)
	raw := codexPairingRaw(t, witness, witness.InvocationID, witness.Argv, witness.ExitStatus, "verification passed\n", "", witness.StartedAt, witness.EndedAt)
	source, witness := bindNativePairingSource(t, raw, witness)
	pairing, err := PairExecutionWitness(source, witness, raw, WitnessPairingPolicy{MaximumClockSkewMillis: 0})
	if err != nil {
		t.Fatal(err)
	}
	if err := pairing.Validate(); err != nil {
		t.Fatal(err)
	}
	if pairing.TimestampOnlyCausality || !pairing.CallIDMatched || !pairing.CommandDigestMatched || !pairing.ParentEdgesMatched ||
		!pairing.ExitStatusMatched || !pairing.StdoutMatched || !pairing.StderrMatched || !pairing.RepositoryBindingMatched || !pairing.TemporalWindowMatched {
		t.Fatalf("pairing proof is incomplete: %#v", pairing)
	}
}

func TestExecutionWitnessPairingRejectsSubstitutionAndTimestampOnlyCausality(t *testing.T) {
	base := stdoutSuccessWitnessForPairing(t)
	tests := []struct {
		name    string
		callID  string
		argv    []string
		exit    int
		stdout  string
		stderr  string
		started time.Time
		ended   time.Time
	}{
		{name: "call substitution", callID: "other-call", argv: base.Argv, exit: base.ExitStatus, stdout: "verification passed\n", started: base.StartedAt, ended: base.EndedAt},
		{name: "command substitution", callID: base.InvocationID, argv: []string{"go", "test", "./..."}, exit: base.ExitStatus, stdout: "verification passed\n", started: base.StartedAt, ended: base.EndedAt},
		{name: "exit substitution", callID: base.InvocationID, argv: base.Argv, exit: 7, stdout: "verification passed\n", started: base.StartedAt, ended: base.EndedAt},
		{name: "stdout substitution", callID: base.InvocationID, argv: base.Argv, exit: base.ExitStatus, stdout: "fabricated success\n", started: base.StartedAt, ended: base.EndedAt},
		{name: "stderr substitution", callID: base.InvocationID, argv: base.Argv, exit: base.ExitStatus, stdout: "verification passed\n", stderr: "unexpected\n", started: base.StartedAt, ended: base.EndedAt},
		{name: "timestamp only", callID: base.InvocationID, argv: []string{"go", "test", "./..."}, exit: base.ExitStatus, stdout: "verification passed\n", started: base.StartedAt, ended: base.EndedAt},
		{name: "outside interval", callID: base.InvocationID, argv: base.Argv, exit: base.ExitStatus, stdout: "verification passed\n", started: base.StartedAt.Add(-time.Second), ended: base.EndedAt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := codexPairingRaw(t, base, test.callID, test.argv, test.exit, test.stdout, test.stderr, test.started, test.ended)
			source, witness := bindNativePairingSource(t, raw, base)
			if _, err := PairExecutionWitness(source, witness, raw, WitnessPairingPolicy{}); err == nil {
				t.Fatal("substituted or timestamp-only native evidence was paired")
			}
		})
	}
}

func TestExecutionWitnessPairingRejectsSourceAndRepositoryDrift(t *testing.T) {
	witness := stdoutSuccessWitnessForPairing(t)
	raw := codexPairingRaw(t, witness, witness.InvocationID, witness.Argv, witness.ExitStatus, "verification passed\n", "", witness.StartedAt, witness.EndedAt)
	source, witness := bindNativePairingSource(t, raw, witness)

	driftedRaw := append([]byte(nil), raw...)
	driftedRaw = append(driftedRaw, '\n')
	if _, err := PairExecutionWitness(source, witness, driftedRaw, WitnessPairingPolicy{}); err == nil {
		t.Fatal("native byte drift was accepted")
	}

	source.RepositoryAlias = "different-repository"
	source.Header.Digest = ""
	source.Header.Digest = sealArtifactForTest(t, source)
	witness.Header.Parents[0].Digest = source.Header.Digest
	witness.Header.Digest = ""
	witness.Header.Digest = sealWitnessForTest(t, witness)
	if _, err := PairExecutionWitness(source, witness, raw, WitnessPairingPolicy{}); err == nil {
		t.Fatal("repository binding drift was accepted")
	}
}

func stdoutSuccessWitnessForPairing(t *testing.T) ExecutionWitness {
	t.Helper()
	set, err := buildSyntheticWitnessFixtureSet(syntheticFixtureCommandSpecs())
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range set.Fixtures {
		if fixture.CaseID == "stdout_success" {
			return fixture.Witness
		}
	}
	t.Fatal("stdout-success witness fixture is absent")
	return ExecutionWitness{}
}

func codexPairingRaw(t *testing.T, witness ExecutionWitness, callID string, argv []string, exit int, stdout, stderr string, started, ended time.Time) []byte {
	t.Helper()
	records := []any{
		map[string]any{"timestamp": started.UTC().Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]any{"type": "exec_command_begin", "call_id": callID, "command": argv}},
		map[string]any{"timestamp": ended.UTC().Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]any{"type": "exec_command_end", "call_id": callID, "status": "completed", "exit_code": exit, "stdout": stdout, "stderr": stderr}},
	}
	lines := make([]string, len(records))
	for index, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		lines[index] = string(encoded)
	}
	return []byte(strings.Join(lines, "\n"))
}

func bindNativePairingSource(t *testing.T, raw []byte, witness ExecutionWitness) (VerificationLineageSource, ExecutionWitness) {
	t.Helper()
	imported, err := preprocess.ImportTraceBytes(raw, preprocess.DefaultTraceImportOptions())
	if err != nil {
		t.Fatal(err)
	}
	accountingDigest, err := digestJSON(imported.Trajectory.Report)
	if err != nil {
		t.Fatal(err)
	}
	source := VerificationLineageSource{
		Header: ArtifactHeader{
			SchemaVersion: SourceSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "paired-source-" + witness.InvocationID, TaskID: "TASK-069", TaskGroupID: witness.Header.TaskGroupID,
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest,
			Parents: []ParentRef{{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-verification-lineage-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest}},
		},
		SourceClass: "checked_in_controls", AgentEcosystem: "codex_fixture", RuntimeIdentityClass: "fixed_local_subprocess",
		ProviderMetadata: "none", ExportFormat: string(imported.Trajectory.SourceFormat), ExportVersion: "fixture-v1", CaptureMode: CapturePaired,
		SourceSessionID: "paired-session", LineageID: "paired-lineage", NearDuplicateID: "paired-near-duplicate",
		RepositoryID: "evalwitness-repository", RepositoryAlias: witness.WorkingDirectoryAlias, TaskAlias: "stdout-success-pairing",
		License: "MIT", RedistributionPermission: "public_mit", PrivacyClass: "synthetic_public", RedactionPolicy: "none",
		RawRecordCount: imported.Trajectory.Report.SourceRecords, RawRecordDigest: digestBytes(raw),
		CanonicalTrajectoryDigest: imported.Trajectory.Digest, FieldAccountingDigest: accountingDigest,
	}
	source.Header.Digest = sealArtifactForTest(t, source)
	if err := source.Validate(); err != nil {
		t.Fatal(err)
	}
	witness.Header.ObjectID = fmt.Sprintf("paired-witness-%s", witness.InvocationID)
	witness.Header.Parents = []ParentRef{{Relation: "source", SchemaVersion: SourceSchemaVersion, ObjectID: source.Header.ObjectID, TaskID: "TASK-069", TaskGroupID: source.Header.TaskGroupID, Digest: source.Header.Digest}}
	witness.Header.Digest = ""
	witness.Header.Digest = sealWitnessForTest(t, witness)
	if err := witness.Validate(); err != nil {
		t.Fatal(err)
	}
	return source, witness
}
