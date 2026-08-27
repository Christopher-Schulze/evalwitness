package stress

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
)

func TestRelationRegistryBindsExactV3CoreSentinelAndSourceFormats(t *testing.T) {
	plan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(plan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.CoreFamilies) != 7 || registry.CoreCases != 280 || registry.SentinelCases != 3 ||
		registry.PrimaryRelations != 7 || registry.SensitivityRelations != 7 || registry.SentinelRelations != 1 || len(registry.Relations) != 15 {
		t.Fatalf("unexpected v3 relation registry: %+v", registry)
	}
	wantReplayIdentityDigest, err := releaseReplayCorpusIdentityDigest(release)
	if err != nil {
		t.Fatal(err)
	}
	if registry.ReplayCorpusIdentityDigest != wantReplayIdentityDigest {
		t.Fatalf("replay corpus identity digest = %s, want %s", registry.ReplayCorpusIdentityDigest, wantReplayIdentityDigest)
	}
	if err := registry.ValidateAgainst(plan, audit, release); err != nil {
		t.Fatal(err)
	}
	for _, relation := range registry.Relations {
		definition, exists := mutation.DefinitionFor(relation.Transform.MutationFamily)
		if !exists {
			t.Fatalf("registered relation %q has no mutation definition", relation.ID)
		}
		want, err := catalogConstraint(definition)
		if err != nil {
			t.Fatal(err)
		}
		if len(relation.Constraints) != 1 || !reflect.DeepEqual(relation.Constraints[0], want) {
			t.Fatalf("relation %q metric contract = %+v, want %+v", relation.ID, relation.Constraints, want)
		}
		if definition.PairLevel && relation.Constraints[0].Metric != MetricDecision {
			t.Fatalf("pair relation %q does not compare selected trajectory identity", relation.ID)
		}
		if !definition.PairLevel && relation.Constraints[0].Metric != MetricConditionalScore {
			t.Fatalf("single-trajectory relation %q retained a non-score outcome", relation.ID)
		}
	}
	if _, err := SealRelationRegistry(plan, audit, release, registry.Relations[:len(registry.Relations)-1]); err == nil {
		t.Fatal("relation registry accepted incomplete release-family coverage")
	}
	tampered := registry
	tampered.Relations = append([]Relation(nil), registry.Relations...)
	tampered.Relations[0].StatisticalFamily.ClusterUnit = "observation"
	if err := tampered.ValidateAgainst(plan, audit, release); err == nil {
		t.Fatal("relation registry accepted a changed cluster unit")
	}
	formatTampered := registry
	formatTampered.Relations = append([]Relation(nil), registry.Relations...)
	changed := false
	for index, relation := range formatTampered.Relations {
		if len(relation.Applicability.RequiredSourceFormats) < 2 {
			continue
		}
		relation.Applicability.RequiredSourceFormats = relation.Applicability.RequiredSourceFormats[:len(relation.Applicability.RequiredSourceFormats)-1]
		relation, err = SealRelation(relation)
		if err != nil {
			t.Fatal(err)
		}
		formatTampered.Relations[index] = relation
		changed = true
		break
	}
	if !changed {
		t.Fatal("v3 registry fixture has no multi-format relation for its negative control")
	}
	formatTampered.Digest, err = relationRegistryDigest(formatTampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := formatTampered.ValidateAgainst(plan, audit, release); err == nil {
		t.Fatal("relation registry accepted omitted release source-format coverage")
	}
	metricTampered := registry
	metricTampered.Relations = append([]Relation(nil), registry.Relations...)
	metricTampered.Relations[0].Constraints = cloneExpectedConstraints(metricTampered.Relations[0].Constraints)
	metricTampered.Relations[0].Constraints[0].Operator = OperatorNotEqual
	metricTampered.Relations[0], err = SealRelation(metricTampered.Relations[0])
	if err != nil {
		t.Fatal(err)
	}
	metricTampered.Digest, err = relationRegistryDigest(metricTampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := metricTampered.Validate(); err == nil {
		t.Fatal("relation registry accepted a post-hoc metric change")
	}
	denominatorTampered := registry.Relations[0]
	denominatorTampered.StatisticalFamily.DenominatorPolicy = DenominatorSensitivityStratified
	if _, err := SealRelation(denominatorTampered); err == nil {
		t.Fatal("relation accepted a denominator policy from another estimand")
	}
	failureTampered := registry.Relations[0]
	failureTampered.StatisticalFamily.FailurePolicy.MissingScore = OutcomeUnsupported
	if _, err := SealRelation(failureTampered); err == nil {
		t.Fatal("relation accepted post-hoc failure handling")
	}
}

func TestRelationRegistryRequiresV3TerminalLedgerForPrimaryCore(t *testing.T) {
	value := testRelationDraft(t, mutation.FamilyNeutralFormatting, EstimandPrimaryCore)
	value.StatisticalFamily.ClusterUnit = "source_task"
	relation, err := SealRelation(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRegistryRelation(relation, mutation.FamilyTestEvidenceOmitted); err == nil {
		t.Fatal("primary-core relation was registered without the v3 terminal ledger")
	}
	value.Applicability.Requirements = append(value.Applicability.Requirements, SourceRequirement{
		Kind: RequirementTerminalLedger, Value: relationevidence.TerminalRelationLedgerSchemaVersionV3,
	})
	relation, err = SealRelation(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRegistryRelation(relation, mutation.FamilyTestEvidenceOmitted); err != nil {
		t.Fatal(err)
	}
}

func currentCorpusV3(t *testing.T) (mutation.CorpusDevelopmentPlan, mutation.CorpusDevelopmentAuditV3, mutation.CorpusReleaseV3) {
	t.Helper()
	root := "../.."
	plan, err := mutation.DecodeCorpusDevelopmentPlan(openFixture(t, root+"/eval/governance/controlled-corruption-v3-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	audit, err := mutation.DecodeCorpusDevelopmentAuditV3(openFixture(t, root+"/eval/governance/controlled-corruption-v3-natural-audit.json"), plan)
	if err != nil {
		t.Fatal(err)
	}
	release, err := mutation.DecodeCorpusReleaseV3(openFixture(t, root+"/eval/governance/controlled-corruption-v3-release.json"), plan, audit)
	if err != nil {
		t.Fatal(err)
	}
	return plan, audit, release
}

type currentV3ReplayFixture struct {
	once           sync.Once
	planDigest     string
	auditDigest    string
	releaseDigest  string
	registryDigest string
	replayed       []ReplayedRelationCaseV3
	err            error
}

var currentV3ReplayCache currentV3ReplayFixture

func currentV3Replay(t *testing.T, plan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, registry RelationRegistry) []ReplayedRelationCaseV3 {
	t.Helper()
	requireFetchedCorpusSources(t)
	currentV3ReplayCache.once.Do(func() {
		currentV3ReplayCache.planDigest = plan.Digest
		currentV3ReplayCache.auditDigest = audit.Digest
		currentV3ReplayCache.releaseDigest = release.Digest
		currentV3ReplayCache.registryDigest = registry.Digest
		currentV3ReplayCache.replayed, currentV3ReplayCache.err = ReplayV3RelationCorpus("../..", plan, audit, release, registry)
	})
	if currentV3ReplayCache.err != nil {
		t.Fatal(currentV3ReplayCache.err)
	}
	if currentV3ReplayCache.planDigest != plan.Digest || currentV3ReplayCache.auditDigest != audit.Digest ||
		currentV3ReplayCache.releaseDigest != release.Digest || currentV3ReplayCache.registryDigest != registry.Digest {
		t.Fatal("current v3 replay cache received a different corpus or relation registry")
	}
	return slices.Clone(currentV3ReplayCache.replayed)
}

func requireFetchedCorpusSources(t *testing.T) {
	t.Helper()
	paths := []string{
		filepath.Join("..", "..", "eval", "trajectories", "terminal_trajs", "forge_gpt54"),
		filepath.Join("..", "..", "eval", "trajectories", "swebench_verified_trajs"),
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		switch {
		case err == nil && info.IsDir():
			continue
		case err == nil:
			t.Fatalf("fetched eval corpus source %s is not a directory", path)
		case os.IsNotExist(err):
			t.Skip("fetched eval trajectories are absent; run eval/fetch-eval-data.sh")
		default:
			t.Fatalf("stat fetched eval corpus source %s: %v", path, err)
		}
	}
}
