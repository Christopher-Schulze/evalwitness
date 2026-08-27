package calibration_test

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/calibration"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
)

func TestChargeFallbackReservesPersistentRunBudget(t *testing.T) {
	budget := mode.NewRunBudget(mode.BudgetLimits{MaxCalls: 3, MaxAttempts: 3, MaxEstimatedInputTokens: 40})
	if err := calibration.ChargeFallback(budget, calibration.FallbackPolicy{
		Kind: calibration.FallbackHuman, CostCalls: 1, CostTokens: 8,
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := budget.Snapshot()
	if snapshot.Calls != 1 || snapshot.EstimatedInputTokens != 8 {
		t.Fatalf("budget = %+v", snapshot)
	}
}
