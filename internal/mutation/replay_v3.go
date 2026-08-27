package mutation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type ReplayedCorpusCaseV3 struct {
	Case        CorpusCaseV3
	Original    []preprocess.Trajectory
	Transformed []preprocess.Trajectory
}

func ReplayCorpusCaseV3(plan CorpusDevelopmentPlan, expected CorpusCaseV3, sources []CorpusSource, candidates []SourceCandidate) (ReplayedCorpusCaseV3, error) {
	if err := plan.Validate(); err != nil {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("validate v3 replay corpus plan: %w", err)
	}
	if err := expected.Manifest.Validate(); err != nil {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("validate v3 replay manifest: %w", err)
	}
	if err := expected.BlindPacket.Validate(); err != nil {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("validate v3 replay blind packet: %w", err)
	}
	if err := expected.ConstructFirewall.Validate(); err != nil {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("validate v3 replay construct firewall: %w", err)
	}
	definition, exists := DefinitionFor(expected.Family)
	if !exists {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("v3 replay case has unknown family %q", expected.Family)
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
	trajectories := make([]preprocess.Trajectory, 0, len(expected.SourceIDs))
	for _, sourceID := range expected.SourceIDs {
		source, sourceExists := sourceByID[sourceID]
		candidate, candidateExists := candidateByID[sourceID]
		if !sourceExists || !candidateExists {
			return ReplayedCorpusCaseV3{}, fmt.Errorf("v3 replay case %q cannot resolve source %q", expected.ID, sourceID)
		}
		if err := validateReplayCandidate(source, candidate); err != nil {
			return ReplayedCorpusCaseV3{}, fmt.Errorf("v3 replay case %q source %q: %w", expected.ID, sourceID, err)
		}
		resolvedSources = append(resolvedSources, source)
		trajectories = append(trajectories, candidate.Trajectory)
	}
	if definition.PairLevel {
		return replayPairCaseV3(plan, expected, resolvedSources, trajectories)
	}
	return replayTrajectoryCaseV3(plan, expected, resolvedSources, trajectories)
}

func replayTrajectoryCaseV3(plan CorpusDevelopmentPlan, expected CorpusCaseV3, sources []CorpusSource, trajectories []preprocess.Trajectory) (ReplayedCorpusCaseV3, error) {
	if len(sources) != 1 || len(trajectories) != 1 {
		return ReplayedCorpusCaseV3{}, errors.New("v3 trajectory replay requires exactly one source")
	}
	request := corpusApplyRequestV3(plan, expected.Family, sources[0])
	outcome, err := ApplyV3(trajectories[0], request)
	if err != nil {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("reapply v3 trajectory mutation: %w", err)
	}
	if outcome.Status != ConstructApplied || outcome.Applied == nil {
		return ReplayedCorpusCaseV3{}, errors.New("reapply v3 trajectory mutation did not produce an applied result")
	}
	reduction, err := ReduceChangedRegions(outcome.Applied.Manifest, trajectories[0], outcome.Applied.Mutated)
	if err != nil {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("reproduce v3 changed-region reduction: %w", err)
	}
	regenerated := CorpusCaseV3{
		ID: outcome.Applied.Manifest.MutationID, SourceIDs: append([]string(nil), expected.SourceIDs...), Family: expected.Family,
		Split: sources[0].Split, Control: corpusControl(expected.Family), Manifest: outcome.Applied.Manifest, BlindPacket: outcome.Applied.Packet,
		Reduction: &reduction, RegenerationKey: digestText(strings.Join([]string{plan.MutationProgramDigest, sources[0].ID, string(expected.Family)}, "\x00")),
		ConstructFirewall: outcome.Firewall,
	}
	if !reflect.DeepEqual(regenerated, expected) {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("v3 replay case %q differs from its frozen corpus case", expected.ID)
	}
	return ReplayedCorpusCaseV3{Case: regenerated, Original: []preprocess.Trajectory{trajectories[0]}, Transformed: []preprocess.Trajectory{outcome.Applied.Mutated}}, nil
}

func replayPairCaseV3(plan CorpusDevelopmentPlan, expected CorpusCaseV3, sources []CorpusSource, trajectories []preprocess.Trajectory) (ReplayedCorpusCaseV3, error) {
	if len(sources) != 2 || len(trajectories) != 2 {
		return ReplayedCorpusCaseV3{}, errors.New("v3 candidate-order replay requires exactly two ordered sources")
	}
	left, right := sources[0], sources[1]
	request := corpusApplyRequestV3(plan, expected.Family, left)
	request.SourceFamily = "paired/" + left.SourceFamily + "+" + right.SourceFamily
	request.SourceLocation = "pair/" + left.SplitGroupID + "/" + digestText(left.ID+"\x00"+right.ID)
	request.SourceRevision = left.SourceRevision + "+" + right.SourceRevision
	request.Outcome = SourceOutcome{
		Kind: "paired_benchmark_rewards", Value: left.Outcome.Value + "," + right.Outcome.Value,
		WitnessDigest: digestText(left.Outcome.WitnessDigest + "\x00" + right.Outcome.WitnessDigest),
	}
	outcome, err := ApplyCandidateOrderReversalV3(trajectories[0], trajectories[1], request)
	if err != nil {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("reapply v3 candidate-order mutation: %w", err)
	}
	if outcome.Status != ConstructApplied || outcome.Applied == nil {
		return ReplayedCorpusCaseV3{}, errors.New("reapply v3 candidate-order mutation did not produce an applied result")
	}
	regenerated := CorpusCaseV3{
		ID: outcome.Applied.Manifest.MutationID, SourceIDs: append([]string(nil), expected.SourceIDs...), Family: expected.Family,
		Split: left.Split, Control: corpusControl(expected.Family), Manifest: outcome.Applied.Manifest, BlindPacket: outcome.Applied.Packet,
		RegenerationKey:   digestText(strings.Join([]string{plan.MutationProgramDigest, left.ID, right.ID, string(expected.Family)}, "\x00")),
		ConstructFirewall: outcome.Firewall,
	}
	if !reflect.DeepEqual(regenerated, expected) {
		return ReplayedCorpusCaseV3{}, fmt.Errorf("v3 replay case %q differs from its frozen corpus case", expected.ID)
	}
	return ReplayedCorpusCaseV3{
		Case: regenerated, Original: []preprocess.Trajectory{trajectories[0], trajectories[1]},
		Transformed: []preprocess.Trajectory{trajectories[1], trajectories[0]},
	}, nil
}
