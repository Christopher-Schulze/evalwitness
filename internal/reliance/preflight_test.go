package reliance

import (
	"reflect"
	"strings"
	"testing"
)

func TestReferenceWalshAuditRejectsSmallerDesignsAndClearsSelectedTerms(t *testing.T) {
	preregistration := referenceTestPreregistration(t)
	setsAudited, qualifyingLayouts := searchThirtyTwoRunLayouts()
	if setsAudited != walsh32SumFreeSetsAudited || qualifyingLayouts != walsh32QualifyingInteractionLayouts {
		t.Fatalf("32-run exhaustive search = %d audited sets, %d qualifying layouts", setsAudited, qualifyingLayouts)
	}
	audit, err := auditReferenceWalshDesign(preregistration)
	if err != nil {
		t.Fatal(err)
	}
	if audit.SelectedRuns != 64 || !audit.MainEffectsClearOfTwoFactorTerms || !audit.DeclaredInteractionsUnique {
		t.Fatalf("selected alias audit = %+v", audit)
	}
	if len(audit.Candidates) != 3 || audit.Candidates[0].Status != "rejected" ||
		audit.Candidates[1].Status != "rejected" || audit.Candidates[2].Status != "selected" {
		t.Fatalf("Walsh candidate decisions = %+v", audit.Candidates)
	}
	if audit.Candidates[1].SumFreeSetsAudited != 135_408 || audit.Candidates[1].QualifyingInteractionLayouts != 0 {
		t.Fatalf("32-run exhaustive audit = %+v", audit.Candidates[1])
	}
}

func TestRelianceResourceBudgetBindsEveryHardDimension(t *testing.T) {
	model := frozenRelianceResourceModel()
	budget := relianceResourceBudget(8, model)
	if budget.LogicalCalls != 520 || budget.HardAttempts != 3_120 || budget.HardInputTokens != 112_320_000 ||
		budget.HardOutputTokens != 12_779_520 || budget.HardDurationSeconds != 46_800 ||
		budget.HardConcurrency != 8 || budget.HardCostUSD != 0 {
		t.Fatalf("resource budget = %+v", budget)
	}
}

func TestReliancePreflightIsDeterministicAndNonAuthorizing(t *testing.T) {
	if testing.Short() {
		t.Fatal("reliance preflight must not be skipped in short mode")
	}
	preregistration := referenceTestPreregistration(t)
	codeDigest := strings.Repeat("c", 64)
	first, err := BuildReliancePreflight(preregistration, codeDigest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildReliancePreflight(preregistration, codeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("reliance preflight is not deterministic")
	}
	if first.LiveAuthorized || first.EmpiricalAssumptions || first.Status == "" {
		t.Fatalf("preflight boundary = live %t empirical %t status %q", first.LiveAuthorized, first.EmpiricalAssumptions, first.Status)
	}
	if first.Status != "resolved" || first.SelectedSourceTasks != 24 || len(first.Candidates) != 5 {
		t.Fatalf("preflight selection = status %s source_tasks %d candidates %d", first.Status, first.SelectedSourceTasks, len(first.Candidates))
	}
	if first.Candidates[3].Resolved || !first.Candidates[4].Resolved {
		t.Fatalf("smallest-design boundary = 20 tasks resolved %t, 24 tasks resolved %t", first.Candidates[3].Resolved, first.Candidates[4].Resolved)
	}
	if len(first.SelectedMDEs) != 2 || first.SelectedMDEs[0].MinimumDetectableEffect == nil ||
		first.SelectedMDEs[1].MinimumDetectableEffect == nil || *first.SelectedMDEs[0].MinimumDetectableEffect != 0.04 ||
		*first.SelectedMDEs[1].MinimumDetectableEffect != 0.75 {
		t.Fatalf("selected MDEs = %+v", first.SelectedMDEs)
	}
	if err := first.Validate(preregistration); err != nil {
		t.Fatal(err)
	}
}
