package mutation

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type ReplayedCorpusCase struct {
	Case        CorpusCase
	Original    []preprocess.Trajectory
	Transformed []preprocess.Trajectory
}

func ReplayCorpusCase(spec CorpusSpec, expected CorpusCase, sources []CorpusSource, candidates []SourceCandidate) (ReplayedCorpusCase, error) {
	if err := spec.Validate(); err != nil {
		return ReplayedCorpusCase{}, fmt.Errorf("validate replay corpus spec: %w", err)
	}
	if err := expected.Manifest.Validate(); err != nil {
		return ReplayedCorpusCase{}, fmt.Errorf("validate replay manifest: %w", err)
	}
	if err := expected.BlindPacket.Validate(); err != nil {
		return ReplayedCorpusCase{}, fmt.Errorf("validate replay blind packet: %w", err)
	}
	definition, exists := DefinitionFor(expected.Family)
	if !exists {
		return ReplayedCorpusCase{}, fmt.Errorf("replay case has unknown family %q", expected.Family)
	}
	if expected.Reduction != nil {
		if err := expected.Reduction.Validate(); err != nil {
			return ReplayedCorpusCase{}, fmt.Errorf("validate replay reduction: %w", err)
		}
	}

	sourceByID := make(map[string]CorpusSource, len(sources))
	for _, source := range sources {
		sourceByID[source.ID] = source
	}
	candidateByID := make(map[string]SourceCandidate, len(candidates))
	for _, candidate := range candidates {
		candidateByID[candidate.Source.ID] = candidate
	}
	resolvedSources := make([]CorpusSource, 0, len(expected.SourceIDs))
	resolvedTrajectories := make([]preprocess.Trajectory, 0, len(expected.SourceIDs))
	for _, sourceID := range expected.SourceIDs {
		source, sourceExists := sourceByID[sourceID]
		candidate, candidateExists := candidateByID[sourceID]
		if !sourceExists || !candidateExists {
			return ReplayedCorpusCase{}, fmt.Errorf("replay case %q cannot resolve source %q", expected.ID, sourceID)
		}
		if err := validateReplayCandidate(source, candidate); err != nil {
			return ReplayedCorpusCase{}, fmt.Errorf("replay case %q source %q: %w", expected.ID, sourceID, err)
		}
		resolvedSources = append(resolvedSources, source)
		resolvedTrajectories = append(resolvedTrajectories, candidate.Trajectory)
	}
	if err := validateCaseSourceBinding(expected, resolvedSources, definition, spec.MutationProgramDigest); err != nil {
		return ReplayedCorpusCase{}, fmt.Errorf("validate replay source binding: %w", err)
	}

	if definition.PairLevel {
		return replayPairCase(spec, expected, resolvedSources, resolvedTrajectories)
	}
	return replayTrajectoryCase(spec, expected, resolvedSources, resolvedTrajectories)
}

func replayTrajectoryCase(spec CorpusSpec, expected CorpusCase, sources []CorpusSource, trajectories []preprocess.Trajectory) (ReplayedCorpusCase, error) {
	if len(sources) != 1 || len(trajectories) != 1 {
		return ReplayedCorpusCase{}, errors.New("trajectory replay requires exactly one source")
	}
	request := corpusApplyRequest(spec, expected.Family, sources[0])
	programVersion, _ := mutationProgramVersion(spec.MutationProgramDigest)
	var applied ApplyResult
	var firewall *ConstructFirewallReport
	if programVersion == MutationProgramVersionV2 {
		outcome, applyErr := ApplyV2(trajectories[0], request)
		if applyErr != nil {
			return ReplayedCorpusCase{}, fmt.Errorf("reapply v2 trajectory mutation: %w", applyErr)
		}
		if outcome.Status != ConstructApplied || outcome.Applied == nil {
			return ReplayedCorpusCase{}, fmt.Errorf("reapply v2 trajectory mutation: construct rejected with %v", outcome.Firewall.RejectionReasons)
		}
		applied = *outcome.Applied
		firewall = &outcome.Firewall
	} else {
		var applyErr error
		applied, applyErr = Apply(trajectories[0], request)
		if applyErr != nil {
			return ReplayedCorpusCase{}, fmt.Errorf("reapply trajectory mutation: %w", applyErr)
		}
	}
	reduction, err := ReduceChangedRegions(applied.Manifest, trajectories[0], applied.Mutated)
	if err != nil {
		return ReplayedCorpusCase{}, fmt.Errorf("reproduce changed-region reduction: %w", err)
	}
	regenerated := CorpusCase{
		ID: applied.Manifest.MutationID, SourceIDs: append([]string(nil), expected.SourceIDs...), Family: expected.Family,
		Split: sources[0].Split, Control: corpusControl(expected.Family), Manifest: applied.Manifest, BlindPacket: applied.Packet,
		Reduction: &reduction, RegenerationKey: digestText(spec.MutationProgramDigest + "\x00" + sources[0].ID + "\x00" + string(expected.Family)), ConstructFirewall: firewall,
	}
	if !reflect.DeepEqual(regenerated, expected) {
		return ReplayedCorpusCase{}, fmt.Errorf("replay case %q differs from its frozen corpus case", expected.ID)
	}
	return ReplayedCorpusCase{Case: regenerated, Original: []preprocess.Trajectory{trajectories[0]}, Transformed: []preprocess.Trajectory{applied.Mutated}}, nil
}

func replayPairCase(spec CorpusSpec, expected CorpusCase, sources []CorpusSource, trajectories []preprocess.Trajectory) (ReplayedCorpusCase, error) {
	if len(sources) != 2 || len(trajectories) != 2 {
		return ReplayedCorpusCase{}, errors.New("candidate-order replay requires exactly two ordered sources")
	}
	left, right := sources[0], sources[1]
	request := corpusApplyRequest(spec, expected.Family, left)
	request.SourceFamily = "paired/" + left.SourceFamily + "+" + right.SourceFamily
	request.SourceLocation = "pair/" + left.SplitGroupID + "/" + digestText(left.ID+"\x00"+right.ID)
	request.SourceRevision = left.SourceRevision + "+" + right.SourceRevision
	request.Outcome = SourceOutcome{
		Kind: "paired_benchmark_rewards", Value: left.Outcome.Value + "," + right.Outcome.Value,
		WitnessDigest: digestText(left.Outcome.WitnessDigest + "\x00" + right.Outcome.WitnessDigest),
	}
	programVersion, _ := mutationProgramVersion(spec.MutationProgramDigest)
	var applied ApplyResult
	var firewall *ConstructFirewallReport
	if programVersion == MutationProgramVersionV2 {
		outcome, applyErr := ApplyCandidateOrderReversalV2(trajectories[0], trajectories[1], request)
		if applyErr != nil {
			return ReplayedCorpusCase{}, fmt.Errorf("reapply v2 candidate-order mutation: %w", applyErr)
		}
		if outcome.Status != ConstructApplied || outcome.Applied == nil {
			return ReplayedCorpusCase{}, errors.New("reapply v2 candidate-order mutation did not produce an applied result")
		}
		applied = *outcome.Applied
		firewall = &outcome.Firewall
	} else {
		var applyErr error
		applied, applyErr = ApplyCandidateOrderReversal(trajectories[0], trajectories[1], request)
		if applyErr != nil {
			return ReplayedCorpusCase{}, fmt.Errorf("reapply candidate-order mutation: %w", applyErr)
		}
	}
	regenerated := CorpusCase{
		ID: applied.Manifest.MutationID, SourceIDs: append([]string(nil), expected.SourceIDs...), Family: expected.Family,
		Split: left.Split, Control: corpusControl(expected.Family), Manifest: applied.Manifest, BlindPacket: applied.Packet,
		RegenerationKey: digestText(spec.MutationProgramDigest + "\x00" + left.ID + "\x00" + right.ID + "\x00" + string(expected.Family)), ConstructFirewall: firewall,
	}
	if !reflect.DeepEqual(regenerated, expected) {
		return ReplayedCorpusCase{}, fmt.Errorf("replay case %q differs from its frozen corpus case", expected.ID)
	}
	return ReplayedCorpusCase{
		Case: regenerated, Original: []preprocess.Trajectory{trajectories[0], trajectories[1]},
		Transformed: []preprocess.Trajectory{trajectories[1], trajectories[0]},
	}, nil
}

func validateReplayCandidate(source CorpusSource, candidate SourceCandidate) error {
	if err := candidate.Trajectory.Validate(); err != nil {
		return err
	}
	actual := candidate.Source
	if actual.ID != source.ID || actual.TaskID != source.TaskID || actual.RepositoryID != source.RepositoryID ||
		actual.SourceFamily != source.SourceFamily || actual.SourceFormat != source.SourceFormat || actual.SourceLocation != source.SourceLocation ||
		actual.SourceRevision != source.SourceRevision || actual.SourceDigest != source.SourceDigest || actual.TrajectoryDigest != source.TrajectoryDigest ||
		actual.PatchDigest != source.PatchDigest || actual.Outcome != source.Outcome || actual.License != source.License || actual.Privacy != source.Privacy ||
		candidate.Trajectory.SourceDigest != source.SourceDigest || candidate.Trajectory.Digest != source.TrajectoryDigest {
		return errors.New("discovered source identity differs from the frozen release")
	}
	return nil
}
