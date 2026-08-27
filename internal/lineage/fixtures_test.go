package lineage

import (
	"bytes"
	"os"
	"testing"
)

func TestSyntheticWitnessFixtureSetMatchesCheckedInArtifact(t *testing.T) {
	set, err := BuildSyntheticWitnessFixtureSet(expectedSyntheticObservations())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(set)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := os.ReadFile("../../eval/governance/synthetic-execution-witness-fixtures-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, artifact) {
		t.Fatal("checked-in synthetic witness fixtures differ from the fixed harness contract")
	}
}

func TestSyntheticWitnessFixturesCoverFailureStreamsWrappersStateAndTruncation(t *testing.T) {
	set, err := BuildSyntheticWitnessFixtureSet(expectedSyntheticObservations())
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Fixtures) != 7 || set.Harness.ArbitraryExecutableAllowed || set.Harness.ShellUsed ||
		set.Harness.ProviderCallsAllowed != 0 || set.Harness.AgentLaunchAllowed {
		t.Fatal("synthetic fixture harness boundary drifted")
	}
	byID := make(map[string]SyntheticWitnessFixture, len(set.Fixtures))
	for _, fixture := range set.Fixtures {
		byID[fixture.CaseID] = fixture
		if fixture.Witness.CapturePolicy.Purpose != CaptureSyntheticFixtureGeneration ||
			!fixture.Witness.CapturePolicy.LaboratoryExecutesCommands || fixture.Witness.AuthoritativeSurface {
			t.Fatalf("fixture %q has an invalid synthetic capture boundary", fixture.CaseID)
		}
	}
	if byID["controlled_failure"].Witness.ExitStatus != 7 || byID["controlled_failure"].Witness.Stderr.State != StreamCaptured {
		t.Fatal("controlled failure lost exit or stderr evidence")
	}
	if byID["mixed_stream_failure"].Witness.Stdout.State != StreamCaptured || byID["mixed_stream_failure"].Witness.Stderr.State != StreamCaptured {
		t.Fatal("mixed-stream fixture lost stream separation")
	}
	if byID["pipeline_masked_failure"].Witness.ExitStatus != 0 || byID["pipeline_masked_failure"].Behavior != SyntheticWrapperMasks {
		t.Fatal("masked pipeline fixture stopped demonstrating false success")
	}
	if byID["wrapper_failure_propagation"].Witness.ExitStatus != 7 || byID["wrapper_failure_propagation"].Behavior != SyntheticWrapperPropagates {
		t.Fatal("wrapper fixture stopped propagating failure")
	}
	if !byID["state_change_success"].RepositoryStateChanged {
		t.Fatal("state-change fixture no longer binds distinct states")
	}
	truncated := byID["truncated_output_success"].Witness.Stdout
	if truncated.State != StreamTruncated || truncated.ObservedBytes != 4096 || truncated.RetainedBytes != 128 || truncated.PrefixDigest == "" || truncated.SuffixDigest == "" {
		t.Fatal("truncation fixture lost full and retained output identity")
	}
}

func TestSyntheticWitnessFixtureSetRejectsObservationAndResealedArtifactDrift(t *testing.T) {
	observations := expectedSyntheticObservations()
	observations[0].ExitStatus = 0
	if _, err := BuildSyntheticWitnessFixtureSet(observations); err == nil {
		t.Fatal("false-success observation was accepted for controlled failure")
	}
	set, err := BuildSyntheticWitnessFixtureSet(expectedSyntheticObservations())
	if err != nil {
		t.Fatal(err)
	}
	set.Fixtures[0].FailureCondition = "weakened"
	set.Digest, err = syntheticWitnessFixtureSetDigest(set)
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Validate(); err == nil {
		t.Fatal("resealed fixture-contract drift was accepted")
	}
}

func TestSyntheticFixtureCommandSpecsReturnIndependentBytes(t *testing.T) {
	first := SyntheticFixtureCommandSpecs()
	first[0].Stderr[0] = 'X'
	second := SyntheticFixtureCommandSpecs()
	if second[0].Stderr[0] == 'X' {
		t.Fatal("fixture command specs leaked mutable bytes")
	}
}

func expectedSyntheticObservations() []SyntheticExecutionObservation {
	specs := SyntheticFixtureCommandSpecs()
	observations := make([]SyntheticExecutionObservation, len(specs))
	for index, spec := range specs {
		observations[index] = SyntheticExecutionObservation{
			CaseID: spec.CaseID, Stdout: spec.Stdout, Stderr: spec.Stderr, ExitStatus: spec.ExitStatus,
			RepositoryStateBefore: spec.StateBefore, RepositoryStateAfter: spec.StateAfter,
		}
	}
	return observations
}
