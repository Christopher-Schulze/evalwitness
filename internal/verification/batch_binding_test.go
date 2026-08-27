package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestBatchPlanBindingLocksStudyRouteRequestsBudgetAndAuthorizationWithoutExecution(t *testing.T) {
	manifestDigest := strings.Repeat("a", 64)
	service := batchBindingService(t, false)
	inputs := []Input{
		batchBindingInput("cell-1", "case-1", "original", manifestDigest),
		batchBindingInput("cell-2", "case-2", "transformed", manifestDigest),
	}
	batch, err := service.PlanBatch(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Authorization == nil || batch.Authorization.StudyManifestDigest != manifestDigest || batch.AuthorizationDigest != "" {
		t.Fatalf("live batch authorization preview = %+v", batch.Authorization)
	}
	binding, err := service.BindBatchPlan(batch)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Offline || binding.InputCount != 2 || binding.StudyManifestDigest != manifestDigest ||
		binding.RequiredAuthorizationDigest != batch.Authorization.AuthorizationDigest || binding.RouteID == "" ||
		binding.RequestTemplates != len(batch.Requests.Requests) || binding.WorstLogicalCalls != batch.Requests.WorstLogicalCalls ||
		binding.Budget.MaxCalls != batch.Plans[0].Input.Limits.MaxCalls || !binding.DisableCache ||
		len(binding.RawTrajectoryDigests) != 2 || len(binding.RawTrajectoryDigests[0]) != 1 ||
		len(binding.TaskDigests) != 2 || len(binding.CriteriaDigests) != 2 || len(binding.BasePolicyDigests) != 2 ||
		len(binding.Repetitions) != 2 || binding.Repetitions[0] != 3 || len(binding.AdaptiveRepetitions) != 2 {
		t.Fatalf("live batch binding = %+v", binding)
	}
	if err := binding.Validate(); err != nil {
		t.Fatal(err)
	}

	t.Run("rejects presented approval", func(t *testing.T) {
		approved := batch
		approved.AuthorizationDigest = batch.Authorization.AuthorizationDigest
		for index := range approved.Plans {
			approved.Plans[index].Input.AuthorizationDigest = approved.AuthorizationDigest
		}
		if _, err := service.BindBatchPlan(approved); err == nil {
			t.Fatal("batch binding accepted an already-approved execution plan")
		}
	})

	t.Run("rejects study substitution", func(t *testing.T) {
		tampered := batch
		tampered.Plans = append([]Plan(nil), batch.Plans...)
		tampered.Plans[1].Input.StudyManifestDigest = strings.Repeat("b", 64)
		if _, err := service.BindBatchPlan(tampered); err == nil {
			t.Fatal("batch binding accepted a substituted study manifest")
		}
	})

	t.Run("rejects resealed binding substitution", func(t *testing.T) {
		tampered := binding
		tampered.BatchRequestContractDigests = append([]string(nil), binding.BatchRequestContractDigests...)
		tampered.BatchRequestContractDigests[0] = strings.Repeat("b", 64)
		tampered.Digest = ""
		tampered.Digest, err = batchPlanBindingDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if err := tampered.Validate(); err == nil {
			t.Fatal("batch binding accepted a resealed route substitution")
		}
	})
}

func TestBatchPlanBindingSeparatesOfflineReplayFromLiveAuthority(t *testing.T) {
	service := batchBindingService(t, true)
	input := batchBindingInput("cell-replay", "case-replay", "relation-replay", strings.Repeat("c", 64))
	batch, err := service.PlanBatch([]Input{input})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := service.BindBatchPlan(batch)
	if err != nil {
		t.Fatal(err)
	}
	if !binding.Offline || binding.RequiredAuthorizationDigest != "" || batch.Authorization != nil {
		t.Fatalf("offline batch binding = %+v", binding)
	}

	missingStudy := input
	missingStudy.StudyManifestDigest = ""
	missingBatch, err := service.PlanBatch([]Input{missingStudy})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindBatchPlan(missingBatch); err == nil {
		t.Fatal("batch binding accepted replay without a study manifest")
	}
}

func batchBindingService(t *testing.T, offline bool) *Service {
	t.Helper()
	service, err := NewService(Config{
		PreprocessBudget: 0,
		RequestProfile: RequestProfile{
			ProviderID: "binding-provider", BaseURL: "https://binding.invalid/v1", RequestedModel: "binding-model",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK,
		},
		BudgetProfile: BudgetProfile{MaxRetries: 1, MaxWorkers: 2, RequestTimeout: time.Second},
		Offline:       offline,
	}, func(context.Context, Plan) (Runtime, error) {
		return Runtime{}, errors.New("batch binding test must not open a runtime")
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func batchBindingInput(cellID, caseID, transformationID, manifestDigest string) Input {
	return Input{
		Entrypoint: "stress.held-out", Mode: ModeAbsolute, Task: "audit one controlled relation side",
		Trajectories: []string{"agent trajectory\n"},
		Criteria:     []verifier.Criterion{{ID: "correctness", Name: "Correctness", Description: "Assess correctness."}},
		Policy: Policy{
			Evidence: EvidenceStrictVerifier, NReps: 3, Epsilon: 0.02, BiasMitigation: "disabled",
			InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise", MaxWorkers: 2, MaxPairCalls: 4,
			ConfidenceThreshold: 0.8,
		},
		StudyManifestDigest: manifestDigest, StudyVariant: transformationID, DisableCache: true,
		Lineage: LineageReferences{AuditCaseID: caseID, TransformationID: transformationID, StudyCellID: cellID},
	}
}
