package stress

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

type ReplayedRelationCaseV3 struct {
	CaseID                  string
	TaskGroupID             string
	Family                  mutation.Family
	Split                   study.DataRole
	ManifestDigest          string
	MutationProgramVersion  string
	RelationContractVersion string
	FormalWitnessDigest     string
	ConstructFirewallDigest string
	OutcomeEvidenceDigest   string
	RelationIDs             []string
	Original                []preprocess.Trajectory
	Transformed             []preprocess.Trajectory
}

func (item ReplayedRelationCaseV3) ValidateConstructBinding(spec Relation, admission ConstructAdmission) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if err := admission.Validate(); err != nil {
		return err
	}
	if item.CaseID != admission.CaseID || item.Family != spec.Transform.MutationFamily ||
		!slices.Contains(item.RelationIDs, spec.ID) || strings.TrimSpace(item.TaskGroupID) == "" ||
		item.TaskGroupID != strings.TrimSpace(item.TaskGroupID) || !validDigest(item.ManifestDigest) ||
		!validDigest(item.OutcomeEvidenceDigest) || item.MutationProgramVersion != mutation.MutationProgramVersionV3 ||
		item.RelationContractVersion != mutation.RelationContractVersionV3 ||
		item.FormalWitnessDigest != admission.FormalWitnessDigest ||
		item.ConstructFirewallDigest != admission.ConstructFirewallDigest {
		return errors.New("replayed relation case differs from its v3 construct admission")
	}
	if !sortedUniqueReplayRelationIDs(item.RelationIDs) || len(item.Original) != len(item.Transformed) ||
		len(item.Original) < spec.Applicability.MinimumTrajectories || len(item.Original) > spec.Applicability.MaximumTrajectories {
		return errors.New("replayed relation case has invalid relation or trajectory cardinality")
	}
	for index := range item.Original {
		if err := item.Original[index].Validate(); err != nil {
			return err
		}
		if err := item.Transformed[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func sortedUniqueReplayRelationIDs(values []string) bool {
	if len(values) == 0 || !slices.IsSorted(values) {
		return false
	}
	for index := 1; index < len(values); index++ {
		if values[index-1] == values[index] {
			return false
		}
	}
	return true
}

func ReplayV3RelationCorpus(root string, plan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, registry RelationRegistry) ([]ReplayedRelationCaseV3, error) {
	if err := registry.ValidateAgainst(plan, audit, release); err != nil {
		return nil, fmt.Errorf("validate stress registry before v3 corpus replay: %w", err)
	}
	candidates, err := mutation.ResolveCorpusSources(root, release.Sources)
	if err != nil {
		return nil, fmt.Errorf("resolve v3 stress corpus sources: %w", err)
	}
	relationsByFamily, err := replayRelationsByFamily(registry.Relations)
	if err != nil {
		return nil, err
	}
	result := make([]ReplayedRelationCaseV3, 0, len(release.Cases))
	coverage := make(map[string]int, len(registry.Relations))
	for _, item := range release.Cases {
		replayed, replayErr := replayV3RelationCase(plan, item, release.Sources, candidates, relationsByFamily, coverage)
		if replayErr != nil {
			return nil, replayErr
		}
		result = append(result, replayed)
	}
	if err := validateReplayRelationCoverage(registry, coverage); err != nil {
		return nil, err
	}
	return result, nil
}

func replayRelationsByFamily(relations []Relation) (map[mutation.Family][]Relation, error) {
	relationsByFamily := make(map[mutation.Family][]Relation, len(relations))
	for _, relation := range relations {
		sealed, sealErr := SealRelation(relation)
		if sealErr != nil {
			return nil, fmt.Errorf("snapshot stress relation %q: %w", relation.ID, sealErr)
		}
		relationsByFamily[sealed.Transform.MutationFamily] = append(relationsByFamily[sealed.Transform.MutationFamily], sealed)
	}
	return relationsByFamily, nil
}

func replayV3RelationCase(plan mutation.CorpusDevelopmentPlan, item mutation.CorpusCaseV3, sources []mutation.CorpusSource, candidates []mutation.SourceCandidate, relationsByFamily map[mutation.Family][]Relation, coverage map[string]int) (ReplayedRelationCaseV3, error) {
	replayed, err := mutation.ReplayCorpusCaseV3(plan, item, sources, candidates)
	if err != nil {
		return ReplayedRelationCaseV3{}, fmt.Errorf("replay v3 stress case %q: %w", item.ID, err)
	}
	relations := relationsByFamily[item.Family]
	if len(relations) == 0 {
		return ReplayedRelationCaseV3{}, fmt.Errorf("v3 stress case %q has no registered relation", item.ID)
	}
	relationIDs := make([]string, len(relations))
	for index, relation := range relations {
		relationIDs[index] = relation.ID
		coverage[relation.ID]++
	}
	slices.Sort(relationIDs)
	return ReplayedRelationCaseV3{
		CaseID: item.ID, TaskGroupID: item.Manifest.SplitGroupID, Family: item.Family, Split: item.Split,
		ManifestDigest: item.Manifest.Digest, MutationProgramVersion: item.Manifest.Program.Version,
		RelationContractVersion: item.Manifest.RelationContractVersion, FormalWitnessDigest: item.Manifest.Witness.Digest,
		ConstructFirewallDigest: item.ConstructFirewall.Digest, OutcomeEvidenceDigest: item.Manifest.Source.Outcome.WitnessDigest,
		RelationIDs: relationIDs, Original: replayed.Original, Transformed: replayed.Transformed,
	}, nil
}

func validateReplayRelationCoverage(registry RelationRegistry, coverage map[string]int) error {
	for _, relation := range registry.Relations {
		expected := registry.CoreCasesPerFamily
		if relation.StatisticalFamily.Estimand == EstimandScarcitySentinel {
			expected = registry.SentinelCases
		}
		if coverage[relation.ID] != expected {
			return fmt.Errorf("v3 stress relation %q replayed %d cases, want %d", relation.ID, coverage[relation.ID], expected)
		}
	}
	return nil
}
