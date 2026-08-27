package stress

import (
	"math"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestStressAnalysisDesignLocksAdjustedFamiliesAndCompleteDenominators(t *testing.T) {
	corpusPlan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(corpusPlan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	replayed := currentV3Replay(t, corpusPlan, audit, release, registry)
	armPlan, err := BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	design, err := BuildStressAnalysisDesign(armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	if design.PrimaryRateFamilySize != 28 || design.SensitivityRateFamilySize != 28 ||
		design.PrimaryContrastFamilySize != 21 || design.SensitivityContrastFamilySize != 21 ||
		design.GlobalScore || design.PopulationGeneralization {
		t.Fatalf("stress analysis design = %+v", design)
	}
	armReport, err := BuildArmComparisonReport(armPlan, registry, replayed, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildStressAnalysisReport(design, armPlan, armReport, registry, replayed, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.PlannedCells != 5630 || report.Totals.SupportedCells != 2249 ||
		report.Totals.StructuralUnsupported != 3381 || report.Totals.CompletedCells != 0 ||
		report.Totals.NotRunCells != 2249 || report.GlobalScore || len(report.MinimalWitnesses) != 0 {
		t.Fatalf("unexecuted stress analysis totals = %+v", report.Totals)
	}
	for _, summary := range report.Summaries {
		if summary.SupportedCells > 0 && summary.Status != AnalysisNotRun {
			t.Fatalf("supported unexecuted summary was not not-run: %+v", summary)
		}
		if summary.SupportedCells == 0 && summary.Status != AnalysisUnsupported {
			t.Fatalf("structurally unsupported summary changed status: %+v", summary)
		}
	}

	tampered := design
	tampered.PrimaryRateFamilySize--
	tampered.Digest, err = stressAnalysisDesignDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.ValidateAgainst(armPlan, registry, replayed); err == nil {
		t.Fatal("stress analysis design accepted a reduced multiplicity family")
	}

	relation := relationByID(t, registry, catalogRelationID(mutation.FamilyNeutralFormatting, EstimandPrimaryCore))
	cells := make([]ArmComparisonObservation, 40)
	for index := range cells {
		outcome := OutcomeSatisfied
		switch {
		case index < 5:
			outcome = OutcomeViolated
		case index < 8:
			outcome = OutcomeAbstained
		case index < 10:
			outcome = OutcomeProviderFailed
		}
		cells[index] = ArmComparisonObservation{
			ArmID: "score-token-verifier", RelationID: relation.ID, RelationDigest: relation.Digest,
			TaskGroupID: "task-group-" + string(rune('A'+index)), Support: ArmSupported,
			Status: ArmCellExecuted, Outcome: outcome, AdmissionStatus: AdmissionHumanSupported,
		}
	}
	summary, err := summarizeRelationArm(design, relation, AnalysisTest, cells)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != AnalysisAdjustedComplete || summary.SourceTaskClusters != 40 || summary.ViolatedClusters != 5 ||
		summary.FailedClusters != 10 || summary.ViolationRate == nil || math.Abs(*summary.ViolationRate-0.125) > 1e-12 ||
		summary.FailureRate == nil || math.Abs(*summary.FailureRate-0.25) > 1e-12 || summary.AdjustedAlpha == nil ||
		math.Abs(*summary.AdjustedAlpha-stressNominalAlpha/28) > 1e-12 || summary.ViolationInterval == nil || summary.FailureInterval == nil {
		t.Fatalf("adjusted relation-arm summary = %+v", summary)
	}
}

func TestTaskClusterWilsonIntervalRetainsZeroEventUncertainty(t *testing.T) {
	interval := taskClusterWilsonInterval(0, 40, 1-stressNominalAlpha/28, true)
	if interval == nil || interval.Estimate != 0 || interval.Lower != 0 || interval.Upper <= 0 ||
		interval.Confidence != 1-stressNominalAlpha/28 || interval.Clusters != 40 ||
		interval.Method != stressIntervalMethod+"_bonferroni" {
		t.Fatalf("zero-event adjusted interval = %+v", interval)
	}
}

func TestStressArmContrastUsesPairedClusterEffectAndAdjustedInference(t *testing.T) {
	relation := testRelation(t, mutation.FamilyCandidateOrderReversal, EstimandPrimaryCore)
	design := StressAnalysisDesign{
		NominalAlpha: stressNominalAlpha, PrimaryContrastFamilySize: 21, SensitivityContrastFamilySize: 21,
	}
	reference := make(map[string]ArmComparisonObservation, 40)
	comparator := make(map[string]ArmComparisonObservation, 40)
	cases := make(map[string]ReplayedRelationCaseV3, 40)
	for index := 0; index < 40; index++ {
		caseID := "case-" + string(rune('A'+index))
		cases[caseID] = ReplayedRelationCaseV3{CaseID: caseID, TaskGroupID: "task-" + string(rune('A'+index))}
		referenceOutcome := OutcomeSatisfied
		if index == 0 || index == 1 {
			referenceOutcome = OutcomeViolated
		}
		comparatorOutcome := OutcomeSatisfied
		if index == 0 || index == 2 || index == 3 || index == 4 {
			comparatorOutcome = OutcomeViolated
		}
		reference[caseID] = ArmComparisonObservation{Support: ArmSupported, Status: ArmCellExecuted, Outcome: referenceOutcome}
		comparator[caseID] = ArmComparisonObservation{Support: ArmSupported, Status: ArmCellExecuted, Outcome: comparatorOutcome}
	}
	contrast, err := summarizeArmContrast(design, relation, AnalysisTest, "explicit-text-judge", reference, comparator, cases)
	if err != nil {
		t.Fatal(err)
	}
	if contrast.Status != AnalysisAdjustedComplete || contrast.PairedClusters != 40 || contrast.BothFailed != 1 ||
		contrast.ComparatorOnlyFailed != 3 || contrast.ReferenceOnlyFailed != 1 || contrast.NeitherFailed != 35 ||
		contrast.FailureRiskDifference == nil || math.Abs(*contrast.FailureRiskDifference-0.05) > 1e-12 ||
		contrast.Interval == nil || contrast.RawPValue == nil || contrast.AdjustedAlpha == nil ||
		math.Abs(*contrast.AdjustedAlpha-stressNominalAlpha/21) > 1e-12 {
		t.Fatalf("paired adjusted arm contrast = %+v", contrast)
	}
}

func TestArmContrastMissingnessStatusAccountsForEveryCluster(t *testing.T) {
	relation := testRelation(t, mutation.FamilyCandidateOrderReversal, EstimandSensitivity)
	design := StressAnalysisDesign{NominalAlpha: stressNominalAlpha}
	reference := map[string]ArmComparisonObservation{
		"case-a": {Support: ArmSupported, Status: ArmCellNotRun},
		"case-b": {Support: ArmSupported, Status: ArmCellNotRun},
	}
	comparator := map[string]ArmComparisonObservation{
		"case-a": {Support: ArmSupported, Status: ArmCellNotRun},
		"case-b": {Support: ArmSupported, Status: ArmCellExecuted, Outcome: OutcomeSatisfied},
	}
	cases := map[string]ReplayedRelationCaseV3{
		"case-a": {CaseID: "case-a", TaskGroupID: "task-a"},
		"case-b": {CaseID: "case-b", TaskGroupID: "task-b"},
	}
	for iteration := 0; iteration < 256; iteration++ {
		contrast, err := summarizeArmContrast(design, relation, AnalysisTest, "explicit-text-judge", reference, comparator, cases)
		if err != nil {
			t.Fatal(err)
		}
		if contrast.Status != AnalysisIncomplete {
			t.Fatalf("partially executed contrast status = %s on iteration %d, want %s", contrast.Status, iteration, AnalysisIncomplete)
		}
	}
}

func TestMinimalWitnessBindingRequiresCapsuleAndCounterexample(t *testing.T) {
	resultDigest := digestText("violated-result")
	relationDigest := digestText("violated-relation")
	caseID := "violated-case"
	privacyDigest := digestText("privacy")
	originalDigest := digestText("original-input")
	candidateDigest := digestText("candidate-input")
	originalObservation := reductionObservationFixture(t, relationDigest, privacyDigest, true, true, true)
	finalObservation := reductionObservationFixture(t, relationDigest, privacyDigest, true, true, false)
	counterexample, err := SealCounterexample(Counterexample{
		RelationDigest: relationDigest, SourceResultDigest: resultDigest, CaseID: caseID,
		OriginalInputDigest: originalDigest, ReducedInputDigest: originalDigest, PrivacyPolicyDigest: privacyDigest,
		PublicReleaseAllowed: true, Algorithm: deterministicRestartGreedy, Minimality: ReductionOneMinimal,
		OriginalUnits: []ReductionUnit{{Kind: "event", ID: "event-1"}}, FinalUnits: []ReductionUnit{{Kind: "event", ID: "event-1"}},
		OriginalObservation: originalObservation,
		Steps: []ReductionStep{{
			Index: 0, UnitKind: "event", UnitID: "event-1", BeforeDigest: originalDigest,
			CandidateDigest: candidateDigest, AfterDigest: originalDigest, Decision: ReductionRejected, Observation: finalObservation,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	report := ArmComparisonReport{Cells: []ArmComparisonObservation{{
		CellID: digestText("cell"), RelationDigest: relationDigest, CaseID: caseID,
		Status: ArmCellExecuted, Outcome: OutcomeViolated, ResultDigest: resultDigest, CapsuleDigest: digestText("capsule"),
	}}}
	witnesses, err := bindMinimalWitnesses(report, []Counterexample{counterexample})
	if err != nil {
		t.Fatal(err)
	}
	if len(witnesses) != 1 || witnesses[0].Status != WitnessBoundPublic ||
		witnesses[0].CounterexampleDigest != counterexample.Digest || !witnesses[0].PublicReleaseAllowed {
		t.Fatalf("minimal witness binding = %+v", witnesses)
	}
}
