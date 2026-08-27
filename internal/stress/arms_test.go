package stress

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestArmComparisonPlanUsesOneLockedCorpusAndExplicitUnsupportedCells(t *testing.T) {
	plan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(plan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	replayed := currentV3Replay(t, plan, audit, release, registry)
	comparison, err := BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	if len(comparison.Arms) != 10 || comparison.RelationCases != 563 || len(comparison.Cells) != 5630 ||
		comparison.SupportedCells != 2249 || comparison.UnsupportedCells != 3381 || comparison.GlobalScore {
		t.Fatalf("arm-comparison coverage = arms:%d relation-cases:%d cells:%d supported:%d unsupported:%d global:%t",
			len(comparison.Arms), comparison.RelationCases, len(comparison.Cells), comparison.SupportedCells, comparison.UnsupportedCells, comparison.GlobalScore)
	}
	for _, cell := range comparison.Cells {
		arm, found := canonicalArmByID(cell.ArmID)
		if !found {
			t.Fatalf("comparison cell %q references unknown arm %q", cell.CellID, cell.ArmID)
		}
		if arm.Kind != ArmZeroCostControl && cell.Support != ArmSupported {
			t.Fatalf("model or protocol cell %q was unexpectedly unsupported", cell.CellID)
		}
		if arm.Kind == ArmZeroCostControl && cell.RelationID != catalogRelationID(mutation.FamilyCandidateOrderReversal, EstimandPrimaryCore) &&
			cell.RelationID != catalogRelationID(mutation.FamilyCandidateOrderReversal, EstimandSensitivity) && cell.Support != ArmUnsupported {
			t.Fatalf("zero-cost cell %q fabricated support for %q", cell.CellID, cell.RelationID)
		}
	}

	tampered := comparison
	tampered.Cells = append([]ArmComparisonCell(nil), comparison.Cells...)
	tampered.Cells[0].Support = ArmSupported
	tampered.Digest, err = armComparisonPlanDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.ValidateAgainst(registry, replayed); err == nil {
		t.Fatal("arm-comparison plan accepted fabricated support")
	}

	replayedSplitTampered := append([]ReplayedRelationCaseV3(nil), replayed...)
	if replayedSplitTampered[0].Split == study.RoleTest {
		replayedSplitTampered[0].Split = study.RoleDevelopment
	} else {
		replayedSplitTampered[0].Split = study.RoleTest
	}
	if _, err := BuildArmComparisonPlan(registry, replayedSplitTampered); err == nil {
		t.Fatal("arm-comparison plan accepted a replay split that differed from the committed v3 release")
	}

	replayedTrajectoryTampered := append([]ReplayedRelationCaseV3(nil), replayed...)
	replayedTrajectoryTampered[0].Original = append([]preprocess.Trajectory(nil), replayed[0].Original...)
	replayedTrajectoryTampered[0].Original[0].Digest = digestText("foreign-replay-trajectory")
	if _, err := BuildArmComparisonPlan(registry, replayedTrajectoryTampered); err == nil {
		t.Fatal("arm-comparison plan accepted replay trajectory identity outside the committed v3 release")
	}
}

func TestZeroCostArmsExecuteCandidateOrderCellsWithoutProviderCalls(t *testing.T) {
	plan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(plan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	replayed := currentV3Replay(t, plan, audit, release, registry)
	relationID := catalogRelationID(mutation.FamilyCandidateOrderReversal, EstimandSensitivity)
	relation := relationByID(t, registry, relationID)
	item := replayedCaseByRelation(t, replayed, relationID)
	admission := formalAdmission(t, item.CaseID)
	executed := 0
	executions := make([]ZeroCostExecution, 0, 7)
	for _, arm := range canonicalArmDefinitions() {
		if arm.Kind != ArmZeroCostControl {
			continue
		}
		execution, runErr := RunZeroCostArm(relation, admission, item, arm.ID)
		if runErr != nil {
			t.Fatalf("%s: %v", arm.ID, runErr)
		}
		if execution.Result.ProviderCalls != 0 || execution.CompletedRepetitions != catalogRepetitions ||
			(execution.Result.Outcome != OutcomeSatisfied && execution.Result.Outcome != OutcomeViolated) {
			t.Fatalf("%s execution = %+v", arm.ID, execution)
		}
		executions = append(executions, execution)
		executed++
	}
	if executed != 7 {
		t.Fatalf("executed zero-cost controls = %d, want 7", executed)
	}
	comparison, err := BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildArmComparisonReport(comparison, registry, replayed, nil, executions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.PlannedCells != 5630 || report.ExecutedCells != 7 || report.NotRunCells != 2242 ||
		report.UnsupportedCells != 3381 || report.GlobalScore {
		t.Fatalf("zero-cost arm report accounting = %+v", report)
	}
	design, err := BuildStressAnalysisDesign(comparison, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := BuildStressAnalysisReport(design, comparison, report, registry, replayed, nil, executions, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	violations := 0
	for _, execution := range executions {
		if execution.Result.Outcome == OutcomeViolated {
			violations++
		}
	}
	if analysis.Totals.WitnessesRequired != violations || analysis.Totals.WitnessesMissing != violations ||
		analysis.Totals.WitnessesBound != 0 || len(analysis.MinimalWitnesses) != violations {
		t.Fatalf("zero-cost violation witness accounting = %+v", analysis.Totals)
	}

	singleRelation := relationByID(t, registry, catalogRelationID(mutation.FamilyNeutralFormatting, EstimandSensitivity))
	singleItem := replayedCaseByRelation(t, replayed, singleRelation.ID)
	if _, err := RunZeroCostArm(singleRelation, formalAdmission(t, singleItem.CaseID), singleItem, "zero-cost-first-listed"); err == nil {
		t.Fatal("zero-cost control executed an unsupported single-trajectory relation")
	}
}

func relationByID(t *testing.T, registry RelationRegistry, relationID string) Relation {
	t.Helper()
	for _, relation := range registry.Relations {
		if relation.ID == relationID {
			return relation
		}
	}
	t.Fatalf("relation %q not found", relationID)
	return Relation{}
}

func replayedCaseByRelation(t *testing.T, replayed []ReplayedRelationCaseV3, relationID string) ReplayedRelationCaseV3 {
	t.Helper()
	for _, item := range replayed {
		if slicesContains(item.RelationIDs, relationID) {
			return item
		}
	}
	t.Fatalf("replayed case for %q not found", relationID)
	return ReplayedRelationCaseV3{}
}
