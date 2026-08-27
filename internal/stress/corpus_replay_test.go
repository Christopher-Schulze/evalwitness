package stress

import (
	"context"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestV3RelationCorpusReplaysEveryCaseAndSharedEntrypoint(t *testing.T) {
	plan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(plan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	replayed := currentV3Replay(t, plan, audit, release, registry)
	if len(replayed) != 283 {
		t.Fatalf("replayed v3 cases = %d, want 283", len(replayed))
	}
	caseByRelation := make(map[string]ReplayedRelationCaseV3, len(registry.Relations))
	coverage := make(map[string]int, len(registry.Relations))
	for _, item := range replayed {
		if len(item.Original) == 0 || len(item.Original) != len(item.Transformed) || item.ManifestDigest == "" {
			t.Fatalf("replayed case %q has incomplete material", item.CaseID)
		}
		for _, relationID := range item.RelationIDs {
			coverage[relationID]++
			if _, exists := caseByRelation[relationID]; !exists {
				caseByRelation[relationID] = item
			}
		}
	}
	for _, relation := range registry.Relations {
		want := 40
		if relation.StatisticalFamily.Estimand == EstimandScarcitySentinel {
			want = 3
		}
		if coverage[relation.ID] != want {
			t.Fatalf("relation %q replay coverage = %d, want %d", relation.ID, coverage[relation.ID], want)
		}
	}

	runner, err := NewReplayFirstRunner(stressVerificationService(t, provider.ReplayStatusExact))
	if err != nil {
		t.Fatal(err)
	}
	entrypoints := []string{"cli.verify", "mcp.delta", "eval-terminal", "eval-swebench", "best-of-n", "protocol.application"}
	baselines := make(map[string]ReplayExecution, len(registry.Relations))
	for _, entrypoint := range entrypoints {
		for _, relation := range registry.Relations {
			item := caseByRelation[relation.ID]
			admission := formalAdmission(t, item.CaseID)
			if relation.StatisticalFamily.Estimand == EstimandPrimaryCore {
				admission = admissionWithStatus(t, admission, AdmissionHumanSupported, true, true)
			}
			execution, runErr := runner.RunMutation(context.Background(), ReplayRunRequest{
				Relation: relation, Admission: admission,
				Original:    corpusVerificationInput(relation, admission, entrypoint, item.Original),
				Transformed: corpusVerificationInput(relation, admission, entrypoint, item.Transformed),
			})
			if runErr != nil {
				t.Fatalf("%s %s: %v", entrypoint, relation.ID, runErr)
			}
			baseline, exists := baselines[relation.ID]
			if !exists {
				baselines[relation.ID] = execution
				continue
			}
			assertCrossEntrypointReplay(t, entrypoint, relation.ID, execution, baseline)
		}
	}
}

func assertCrossEntrypointReplay(t *testing.T, entrypoint, relationID string, got, want ReplayExecution) {
	t.Helper()
	if got.BatchRunFingerprint != want.BatchRunFingerprint {
		t.Fatalf("%s changed batch fingerprint for relation %q", entrypoint, relationID)
	}
	for _, sides := range [][2]ReplaySide{{got.Original, want.Original}, {got.Transformed, want.Transformed}} {
		for index, stage := range orderedStages() {
			gotRecord, wantRecord := sides[0].StageTrace.Records[index], sides[1].StageTrace.Records[index]
			if stage == StageProviderResponse {
				if gotRecord.Digest == wantRecord.Digest {
					t.Fatalf("%s erased provider-response entrypoint provenance for relation %q", entrypoint, relationID)
				}
				continue
			}
			if gotRecord != wantRecord {
				t.Fatalf("%s changed canonical %s evidence for relation %q", entrypoint, stage, relationID)
			}
		}
	}
	if got.StageComparison.EarliestDivergentStage != want.StageComparison.EarliestDivergentStage ||
		got.StageComparison.EarliestUnexpectedStage != want.StageComparison.EarliestUnexpectedStage ||
		len(got.StageComparison.Differences) != len(want.StageComparison.Differences) {
		t.Fatalf("%s changed stage-comparison semantics for relation %q", entrypoint, relationID)
	}
	for index := range got.StageComparison.Differences {
		left, right := got.StageComparison.Differences[index], want.StageComparison.Differences[index]
		if left.Stage != right.Stage || left.Expectation != right.Expectation || left.Divergent != right.Divergent || left.Unexpected != right.Unexpected {
			t.Fatalf("%s changed %s comparison semantics for relation %q", entrypoint, left.Stage, relationID)
		}
	}
}

func corpusVerificationInput(relation Relation, admission ConstructAdmission, entrypoint string, trajectories []preprocess.Trajectory) verification.Input {
	texts := make([]string, len(trajectories))
	for index, trajectory := range trajectories {
		texts[index] = preprocess.RenderTrajectory(trajectory)
	}
	verificationMode := verification.ModeAbsolute
	if len(texts) == 2 {
		verificationMode = verification.ModePairwise
	}
	return verification.Input{
		Entrypoint: entrypoint, Mode: verificationMode, Task: "audit controlled coding-agent trajectory relation",
		Trajectories: texts, Criteria: []verifier.Criterion{{ID: "correctness", Name: "Correctness", Description: "Assess correctness."}},
		Policy: verification.Policy{
			Evidence: verification.EvidenceExplicitJudge, NReps: relation.Repeat.MaximumRepetitions, Epsilon: 0.02,
			BiasMitigation: "disabled", InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise",
			MaxWorkers: 2, MaxPairCalls: 4, ConfidenceThreshold: 0.8,
		},
		DisableCache: true,
		Lineage:      verification.LineageReferences{AuditCaseID: admission.CaseID, TransformationID: relation.ID},
	}
}
