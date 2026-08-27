package mutation

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestReplayCorpusCaseReproducesTrajectoryAndPairUnits(t *testing.T) {
	spec, err := DefaultCorpusSpec()
	if err != nil {
		t.Fatal(err)
	}
	base := ingestFixture(t, "claude-code.jsonl")
	left := replayFixtureSource(base, "left")

	trajectoryRequest := corpusApplyRequest(spec, FamilyTestEvidenceOmitted, left)
	trajectoryApplied, err := Apply(base, trajectoryRequest)
	if err != nil {
		t.Fatal(err)
	}
	reduction, err := ReduceChangedRegions(trajectoryApplied.Manifest, base, trajectoryApplied.Mutated)
	if err != nil {
		t.Fatal(err)
	}
	trajectoryCase := CorpusCase{
		ID: trajectoryApplied.Manifest.MutationID, SourceIDs: []string{left.ID}, Family: FamilyTestEvidenceOmitted,
		Split: left.Split, Control: corpusControl(FamilyTestEvidenceOmitted), Manifest: trajectoryApplied.Manifest,
		BlindPacket: trajectoryApplied.Packet, Reduction: &reduction,
		RegenerationKey: digestText(spec.MutationProgramDigest + "\x00" + left.ID + "\x00" + string(FamilyTestEvidenceOmitted)),
	}
	replayedTrajectory, err := ReplayCorpusCase(spec, trajectoryCase, []CorpusSource{left}, []SourceCandidate{{Source: left, Trajectory: base}})
	if err != nil {
		t.Fatal(err)
	}
	if len(replayedTrajectory.Original) != 1 || len(replayedTrajectory.Transformed) != 1 || replayedTrajectory.Transformed[0].Digest != trajectoryApplied.Mutated.Digest {
		t.Fatalf("trajectory replay material = %#v", replayedTrajectory)
	}

	peer, err := preprocess.DeriveTrajectory(base, base.Events, base.Links, preprocess.DerivationSpec{Relation: "fixture_peer", Validator: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	right := replayFixtureSource(peer, "right")
	pairRequest := corpusApplyRequest(spec, FamilyCandidateOrderReversal, left)
	pairRequest.SourceFamily = "paired/" + left.SourceFamily + "+" + right.SourceFamily
	pairRequest.SourceLocation = "pair/" + left.SplitGroupID + "/" + digestText(left.ID+"\x00"+right.ID)
	pairRequest.SourceRevision = left.SourceRevision + "+" + right.SourceRevision
	pairRequest.Outcome = SourceOutcome{
		Kind: "paired_benchmark_rewards", Value: left.Outcome.Value + "," + right.Outcome.Value,
		WitnessDigest: digestText(left.Outcome.WitnessDigest + "\x00" + right.Outcome.WitnessDigest),
	}
	pairApplied, err := ApplyCandidateOrderReversal(base, peer, pairRequest)
	if err != nil {
		t.Fatal(err)
	}
	pairCase := CorpusCase{
		ID: pairApplied.Manifest.MutationID, SourceIDs: []string{left.ID, right.ID}, Family: FamilyCandidateOrderReversal,
		Split: left.Split, Control: corpusControl(FamilyCandidateOrderReversal), Manifest: pairApplied.Manifest, BlindPacket: pairApplied.Packet,
		RegenerationKey: digestText(spec.MutationProgramDigest + "\x00" + left.ID + "\x00" + right.ID + "\x00" + string(FamilyCandidateOrderReversal)),
	}
	replayedPair, err := ReplayCorpusCase(spec, pairCase, []CorpusSource{left, right}, []SourceCandidate{{Source: left, Trajectory: base}, {Source: right, Trajectory: peer}})
	if err != nil {
		t.Fatal(err)
	}
	if len(replayedPair.Original) != 2 || len(replayedPair.Transformed) != 2 || replayedPair.Transformed[0].Digest != peer.Digest || replayedPair.Transformed[1].Digest != base.Digest {
		t.Fatalf("pair replay did not return the exact reverse ordering: %#v", replayedPair)
	}

	tampered := left
	tampered.SourceRevision = "wrong-revision"
	if _, err := ReplayCorpusCase(spec, trajectoryCase, []CorpusSource{left}, []SourceCandidate{{Source: tampered, Trajectory: base}}); err == nil {
		t.Fatal("corpus replay accepted a source revision mismatch")
	}
}

func replayFixtureSource(trajectory preprocess.Trajectory, suffix string) CorpusSource {
	license := LicenseMetadata{SPDX: "MIT", SourceURL: "https://github.com/example/repository", SourceRevision: "fixture-v1", Redistribution: "permitted", Attribution: "EvalWitness fixtures"}
	privacy := PrivacyMetadata{Classification: "public", RedactionPolicyDigest: digestText("redaction"), PublicReleaseAllowed: true}
	return CorpusSource{
		ID: "source-" + trajectory.Digest, TaskID: "fixture-task", RepositoryID: "fixture-repository", SourceFamily: "evalwitness-golden",
		SourceFormat: trajectory.SourceFormat, SourceLocation: "internal/preprocess/testdata/golden/" + suffix, SourceRevision: "fixture-v1",
		SourceDigest: trajectory.SourceDigest, TrajectoryDigest: trajectory.Digest, PatchDigest: trajectoryPatchDigest(trajectory),
		SplitGroupID: "fixture-task", NearDuplicateID: "near-" + digestText("fixture-task"), LineageClusterID: "lineage-" + digestText("fixture-task"), Split: study.RoleDevelopment,
		Outcome: SourceOutcome{Kind: "formal_fixture", Value: "pass", WitnessDigest: digestText("outcome-" + suffix)}, License: license, Privacy: privacy,
	}
}
