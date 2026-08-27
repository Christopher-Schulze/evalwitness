package stress

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

type countingReplayExecutor struct {
	VerificationExecutor
	plans      atomic.Int64
	executions atomic.Int64
}

func (executor *countingReplayExecutor) PlanBatch(inputs []verification.Input) (verification.BatchPlan, error) {
	executor.plans.Add(1)
	return executor.VerificationExecutor.PlanBatch(inputs)
}

func (executor *countingReplayExecutor) ExecuteBatch(ctx context.Context, plan verification.BatchPlan) (verification.BatchResult, error) {
	executor.executions.Add(1)
	return executor.VerificationExecutor.ExecuteBatch(ctx, plan)
}

func TestReplayFirstRunnerExecutesOneSharedEvidenceBatch(t *testing.T) {
	request := genericReplayBatchRequest(t)
	executor := &countingReplayExecutor{VerificationExecutor: stressVerificationService(t, provider.ReplayStatusExact)}
	runner, err := NewReplayFirstRunner(executor)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := runner.RunBatchEvidence(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := evidence.ValidateRequest(request); err != nil {
		t.Fatal(err)
	}
	if len(evidence.Items) != 3 || executor.plans.Load() != 1 || executor.executions.Load() != 1 {
		t.Fatalf("shared replay batch items=%d plans=%d executions=%d", len(evidence.Items), executor.plans.Load(), executor.executions.Load())
	}
}

func TestReplayFirstRunnerRejectsBatchControlPlaneSubstitution(t *testing.T) {
	request := genericReplayBatchRequest(t)
	executor := &countingReplayExecutor{VerificationExecutor: stressVerificationService(t, provider.ReplayStatusExact)}
	runner, err := NewReplayFirstRunner(executor)
	if err != nil {
		t.Fatal(err)
	}
	request.Items[2].Input.Lineage.TransformationID = "foreign-treatment-plan"
	_, err = runner.RunBatchEvidence(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "outside trajectory") || executor.plans.Load() != 0 {
		t.Fatalf("batch control-plane substitution error=%v plan_calls=%d", err, executor.plans.Load())
	}
}

func TestReplayBatchEvidenceRejectsRequestedItemSubstitution(t *testing.T) {
	request := genericReplayBatchRequest(t)
	evidence, err := exactReplayPairRunner(t).RunBatchEvidence(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Items[1].Input.Trajectories[0] = "substituted after execution"
	if err := evidence.ValidateRequest(request); err == nil || !strings.Contains(err.Error(), "does not bind") {
		t.Fatalf("substituted replay batch request error = %v", err)
	}
}

func genericReplayBatchRequest(t *testing.T) ReplayBatchRequest {
	t.Helper()
	pair := genericReplayPairRequest(t)
	baseline := pair.Original
	baseline.StudyVariant, baseline.Lineage.StudyCellID = "baseline", "baseline-cell"
	first := pair.Transformed
	first.StudyVariant, first.Lineage.StudyCellID = "cell-001", "cell-001"
	second := pair.Transformed
	second.Trajectories = []string{"second transformed trajectory"}
	second.StudyVariant, second.Lineage.StudyCellID = "cell-002", "cell-002"
	return ReplayBatchRequest{Items: []ReplayBatchItemRequest{
		{Label: "baseline", Input: baseline},
		{Label: "cell-001", Input: first},
		{Label: "cell-002", Input: second},
	}}
}
