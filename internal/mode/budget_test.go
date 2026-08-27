package mode

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestRunBudgetEnforcesCallsTokensAndCost(t *testing.T) {
	tests := []struct {
		name         string
		limits       BudgetLimits
		firstTokens  int
		firstCost    float64
		secondTokens int
		secondCost   float64
		metric       string
	}{
		{name: "calls", limits: BudgetLimits{MaxCalls: 1}, metric: "calls"},
		{name: "tokens", limits: BudgetLimits{MaxEstimatedInputTokens: 10}, firstTokens: 6, secondTokens: 5, metric: "estimated_input_tokens"},
		{name: "cost", limits: BudgetLimits{MaxCostUSD: 0.1}, firstCost: 0.06, secondCost: 0.05, metric: "estimated_cost_usd"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			budget := NewRunBudget(test.limits)
			if err := budget.Reserve(test.firstTokens, test.firstCost); err != nil {
				t.Fatal(err)
			}
			err := budget.Reserve(test.secondTokens, test.secondCost)
			var exceeded *BudgetExceededError
			if !errors.As(err, &exceeded) {
				t.Fatalf("error = %T %v, want BudgetExceededError", err, err)
			}
			if exceeded.Metric != test.metric {
				t.Fatalf("metric = %q, want %q", exceeded.Metric, test.metric)
			}
		})
	}
}

func TestRunBudgetDeadline(t *testing.T) {
	budget := NewRunBudget(BudgetLimits{MaxDuration: time.Nanosecond})
	time.Sleep(time.Millisecond)
	err := budget.Reserve(1, 0)
	var exceeded *BudgetExceededError
	if !errors.As(err, &exceeded) || exceeded.Metric != "duration_seconds" {
		t.Fatalf("error = %T %v, want duration budget error", err, err)
	}
}

func TestPersistentRunBudgetResumesUsage(t *testing.T) {
	limits := BudgetLimits{MaxCalls: 3, MaxEstimatedInputTokens: 100, MaxCostUSD: 1, MaxDuration: time.Hour}
	statePath := filepath.Join(t.TempDir(), "budget.json")
	first, err := NewPersistentRunBudget(limits, statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Reserve(20, 0.1); err != nil {
		t.Fatal(err)
	}
	resumed, err := NewPersistentRunBudget(limits, statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := resumed.Reserve(30, 0.2); err != nil {
		t.Fatal(err)
	}
	snapshot := resumed.Snapshot()
	if snapshot.Calls != 2 || snapshot.EstimatedInputTokens != 50 || math.Abs(snapshot.EstimatedCostUSD-0.3) > 1e-12 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestPersistentRunBudgetCreatesSensitiveParentAndFile(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, "nested", "budget.json")
	if _, err := NewPersistentRunBudget(BudgetLimits{MaxCalls: 1}, statePath); err != nil {
		t.Fatal(err)
	}
	parentInfo, err := os.Stat(filepath.Dir(statePath))
	if err != nil {
		t.Fatal(err)
	}
	if parentInfo.Mode().Perm() != safety.SensitiveDirectoryMode {
		t.Fatalf("parent mode = %o", parentInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != safety.SensitiveFileMode {
		t.Fatalf("file mode = %o", fileInfo.Mode().Perm())
	}
}

func TestPersistentRunBudgetRejectsChangedLimits(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "budget.json")
	if _, err := NewPersistentRunBudget(BudgetLimits{MaxCalls: 2}, statePath); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPersistentRunBudget(BudgetLimits{MaxCalls: 3}, statePath); err == nil {
		t.Fatal("expected changed-limit error")
	}
}

func TestPersistentRunBudgetSerializesConcurrentReservations(t *testing.T) {
	limits := BudgetLimits{MaxCalls: 32, MaxEstimatedInputTokens: 320, MaxDuration: time.Hour}
	statePath := filepath.Join(t.TempDir(), "budget.json")
	budget, err := NewPersistentRunBudget(limits, statePath)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errorsFound := make(chan error, 16)
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			errorsFound <- budget.Reserve(10, 0)
		}()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	resumed, err := NewPersistentRunBudget(limits, statePath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := resumed.Snapshot()
	if snapshot.Calls != 16 || snapshot.EstimatedInputTokens != 160 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestRunBudgetSeparatesLogicalCallsFromHTTPAttempts(t *testing.T) {
	budget := NewRunBudget(BudgetLimits{
		MaxCalls: 1, MaxAttempts: 2, MaxEstimatedInputTokens: 20,
		MaxReservedOutputTokens: 16, MaxConcurrent: 1,
	})
	if err := budget.ReserveCall(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		release, err := budget.AcquireAttempt(context.Background(), AttemptReservation{
			EstimatedInputTokens: 10, MaxOutputTokens: 8,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := release(AttemptActual{}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := budget.Snapshot()
	if snapshot.Calls != 1 || snapshot.Attempts != 2 || snapshot.EstimatedInputTokens != 20 || snapshot.ReservedOutputTokens != 16 || snapshot.PeakConcurrent != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if _, err := budget.AcquireAttempt(context.Background(), AttemptReservation{}); err == nil {
		t.Fatal("third HTTP attempt exceeded no limit")
	}
}

func TestRunBudgetRejectsWorstAttemptBeforeDispatch(t *testing.T) {
	budget := NewRunBudget(BudgetLimits{MaxAttempts: 2, MaxReservedOutputTokens: 10, MaxConcurrent: 1})
	release, err := budget.AcquireAttempt(context.Background(), AttemptReservation{MaxOutputTokens: 6})
	if err != nil {
		t.Fatal(err)
	}
	if err := release(AttemptActual{}); err != nil {
		t.Fatal(err)
	}
	_, err = budget.AcquireAttempt(context.Background(), AttemptReservation{MaxOutputTokens: 5})
	var exceeded *BudgetExceededError
	if !errors.As(err, &exceeded) || exceeded.Metric != "reserved_output_tokens" {
		t.Fatalf("error = %T %v", err, err)
	}
	snapshot := budget.Snapshot()
	if snapshot.Attempts != 1 || snapshot.ReservedOutputTokens != 6 {
		t.Fatalf("failed reservation changed usage: %+v", snapshot)
	}
}

func TestRunBudgetConcurrencyWaitHonorsCallerCancellation(t *testing.T) {
	budget := NewRunBudget(BudgetLimits{MaxAttempts: 2, MaxConcurrent: 1})
	release, err := budget.AcquireAttempt(context.Background(), AttemptReservation{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := budget.AcquireAttempt(ctx, AttemptReservation{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked reservation error = %v", err)
	}
	if err := release(AttemptActual{}); err != nil {
		t.Fatal(err)
	}
	if snapshot := budget.Snapshot(); snapshot.Attempts != 1 || snapshot.PeakConcurrent != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestRunBudgetConcurrentAttemptsCannotOversubscribe(t *testing.T) {
	const limit = 16
	budget := NewRunBudget(BudgetLimits{MaxAttempts: limit, MaxReservedOutputTokens: limit, MaxConcurrent: limit})
	var group sync.WaitGroup
	results := make(chan error, limit*2)
	for range limit * 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			release, err := budget.AcquireAttempt(context.Background(), AttemptReservation{MaxOutputTokens: 1})
			if err == nil {
				err = release(AttemptActual{})
			}
			results <- err
		}()
	}
	group.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
			continue
		}
		var exceeded *BudgetExceededError
		if !errors.As(err, &exceeded) {
			t.Fatalf("unexpected error = %T %v", err, err)
		}
	}
	if succeeded != limit {
		t.Fatalf("successful reservations = %d, want %d", succeeded, limit)
	}
	if snapshot := budget.Snapshot(); snapshot.Attempts != limit || snapshot.ReservedOutputTokens != limit || snapshot.PeakConcurrent > limit {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestRunBudgetReconcilesReservationToObservedUsage(t *testing.T) {
	budget := NewRunBudget(BudgetLimits{
		MaxAttempts: 2, MaxEstimatedInputTokens: 20, MaxReservedOutputTokens: 16,
		MaxCostUSD: 1, MaxConcurrent: 1,
	})
	release, err := budget.AcquireAttempt(context.Background(), AttemptReservation{
		EstimatedInputTokens: 10, MaxOutputTokens: 8, EstimatedCostUSD: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	actualCost := 0.2
	if err := release(AttemptActual{InputTokens: 6, OutputTokens: 3, CostUSD: &actualCost, UsageObserved: true}); err != nil {
		t.Fatal(err)
	}
	snapshot := budget.Snapshot()
	if snapshot.EstimatedInputTokens != 6 || snapshot.ReservedOutputTokens != 3 || math.Abs(snapshot.EstimatedCostUSD-0.2) > 1e-12 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestRunBudgetRecordsAndRejectsObservedOverrun(t *testing.T) {
	budget := NewRunBudget(BudgetLimits{
		MaxAttempts: 1, MaxEstimatedInputTokens: 10, MaxReservedOutputTokens: 8,
		MaxCostUSD: 0.5, MaxConcurrent: 1,
	})
	release, err := budget.AcquireAttempt(context.Background(), AttemptReservation{
		EstimatedInputTokens: 5, MaxOutputTokens: 4, EstimatedCostUSD: 0.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	actualCost := 0.7
	err = release(AttemptActual{InputTokens: 11, OutputTokens: 9, CostUSD: &actualCost, UsageObserved: true})
	var exceeded *BudgetExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("error = %T %v, want BudgetExceededError", err, err)
	}
	snapshot := budget.Snapshot()
	if snapshot.EstimatedInputTokens != 11 || snapshot.ReservedOutputTokens != 9 || math.Abs(snapshot.EstimatedCostUSD-0.7) > 1e-12 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
