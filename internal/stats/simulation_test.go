package stats

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestClusteredFactorialSimulationIsDeterministicAndBudgeted(t *testing.T) {
	spec := referenceSimulationSpec()
	first, err := SimulateClusteredFactorial(spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SimulateClusteredFactorial(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical seed and assumptions produced different reports")
	}
	wantCalls := spec.SourceTasks*spec.CallsPerSourceTask + first.Budget.PlannedObservations*spec.CallsPerObservation
	if first.Budget.RequiredCalls != wantCalls {
		t.Fatalf("required calls = %d, planned observations = %d", first.Budget.RequiredCalls, first.Budget.PlannedObservations)
	}
	if first.Denominators.Planned != first.Budget.PlannedObservations*spec.Replications {
		t.Fatalf("denominator planned = %d, want %d", first.Denominators.Planned, first.Budget.PlannedObservations*spec.Replications)
	}
	if first.Aliasing.Rank != first.Aliasing.Parameters || len(first.Aliasing.AliasedTerms) != 0 {
		t.Fatalf("full factorial aliasing = %#v", first.Aliasing)
	}
}

func TestClusteredFactorialSimulationRecoversPlantedContinuousEffects(t *testing.T) {
	spec := referenceSimulationSpec()
	report, err := SimulateClusteredFactorial(spec)
	if err != nil {
		t.Fatal(err)
	}
	main, found := findOperatingCharacteristic(report.OperatingCharacteristics, "failure_evidence")
	if !found || !main.Estimable {
		t.Fatalf("main effect missing: %#v", report.OperatingCharacteristics)
	}
	if main.MeanEstimate < 0.6 || main.MeanEstimate > 1.0 {
		t.Fatalf("mean planted effect = %.4f, want near .8", main.MeanEstimate)
	}
	if main.Power < 0.8 {
		t.Fatalf("planted effect power = %.3f, want >= .8", main.Power)
	}
	null, found := findOperatingCharacteristic(report.OperatingCharacteristics, "formatting")
	if !found || null.Power > 0.15 {
		t.Fatalf("null factor false-positive rate = %.3f, want <= .15", null.Power)
	}
}

func TestClusteredFactorialSimulationFailsClosedOnHardBudget(t *testing.T) {
	spec := referenceSimulationSpec()
	spec.HardCalls = 1
	if _, err := SimulateClusteredFactorial(spec); err == nil {
		t.Fatal("over-budget design was accepted")
	}
}

func TestClusteredFactorialSimulationRejectsUnboundCodeAndNegativeResources(t *testing.T) {
	spec := referenceSimulationSpec()
	spec.CodeDigest = "label-not-a-digest"
	if _, err := SimulateClusteredFactorial(spec); err == nil {
		t.Fatal("non-digest code identity was accepted")
	}
	spec = referenceSimulationSpec()
	spec.CallsPerObservation = -1
	if _, err := SimulateClusteredFactorial(spec); err == nil {
		t.Fatal("negative call coefficient was accepted")
	}
	spec = referenceSimulationSpec()
	spec.InputTokensPerSourceTask = -1
	if _, err := SimulateClusteredFactorial(spec); err == nil {
		t.Fatal("negative source-task resource coefficient was accepted")
	}
	spec = referenceSimulationSpec()
	spec.InvalidRate = math.NaN()
	if _, err := SimulateClusteredFactorial(spec); err == nil {
		t.Fatal("non-finite failure rate was accepted")
	}
	spec = referenceSimulationSpec()
	spec.ResidualSD = math.NaN()
	if _, err := SimulateClusteredFactorial(spec); err == nil {
		t.Fatal("non-finite residual standard deviation was accepted")
	}
}

func TestSparseFactorialReportsAliasing(t *testing.T) {
	spec := referenceSimulationSpec()
	spec.SourceTasks = 2
	spec.SparseCellFraction = 0.01
	report, err := SimulateClusteredFactorial(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.Aliasing.Rank >= report.Aliasing.Parameters || len(report.Aliasing.AliasedTerms) == 0 {
		t.Fatalf("sparse design did not report aliasing: %#v", report.Aliasing)
	}
}

func TestCodedFactorialSupportsElevenFactorsWithoutModeledAliasing(t *testing.T) {
	spec := codedSimulationSpec()
	report, err := SimulateClusteredFactorial(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.Budget.PlannedCells != spec.SourceTasks*spec.CodedDesign.Runs {
		t.Fatalf("planned cells = %d, want %d", report.Budget.PlannedCells, spec.SourceTasks*spec.CodedDesign.Runs)
	}
	if report.Aliasing.Rank != report.Aliasing.Parameters || report.Aliasing.Parameters != 16 {
		t.Fatalf("coded design aliasing = %#v, want rank 16/16", report.Aliasing)
	}
	planned, budget := planFactorialObservations(spec, mustSimulationTerms(t, spec))
	if budget != report.Budget {
		t.Fatalf("planned budget = %+v, report budget = %+v", budget, report.Budget)
	}
	for column := 1; column <= len(spec.Factors); column++ {
		sum := 0.0
		for _, observation := range planned[:spec.CodedDesign.Runs] {
			sum += observation.x[column]
		}
		if sum != 0 {
			t.Fatalf("coded factor column %d is not balanced: sum=%v", column, sum)
		}
	}
}

func TestCodedFactorialRejectsAmbiguousOrInexactDesigns(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ClusterSimulationSpec)
	}{
		{name: "algorithm", mutate: func(spec *ClusterSimulationSpec) { spec.CodedDesign.Algorithm = "unknown" }},
		{name: "runs", mutate: func(spec *ClusterSimulationSpec) { spec.CodedDesign.Runs = 63 }},
		{name: "fraction", mutate: func(spec *ClusterSimulationSpec) { spec.SparseCellFraction = 0.5 }},
		{name: "missing factor", mutate: func(spec *ClusterSimulationSpec) { spec.CodedDesign.FactorMasks = spec.CodedDesign.FactorMasks[:10] }},
		{name: "duplicate factor", mutate: func(spec *ClusterSimulationSpec) {
			spec.CodedDesign.FactorMasks[1].FactorID = spec.CodedDesign.FactorMasks[0].FactorID
		}},
		{name: "duplicate mask", mutate: func(spec *ClusterSimulationSpec) {
			spec.CodedDesign.FactorMasks[1].Mask = spec.CodedDesign.FactorMasks[0].Mask
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := codedSimulationSpec()
			test.mutate(&spec)
			if _, err := SimulateClusteredFactorial(spec); err == nil {
				t.Fatal("invalid coded design was accepted")
			}
		})
	}
}

func TestMinimumDetectableFactorEffectUsesPrespecifiedGrid(t *testing.T) {
	spec := referenceSimulationSpec()
	spec.Replications = 80
	detectable, results, err := MinimumDetectableFactorEffect(spec, "failure_evidence", []float64{0, 0.4, 0.8, 1.2}, 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("effect grid results = %d, want at least 2", len(results))
	}
	if detectable == nil || *detectable <= 0 {
		t.Fatalf("detectable effect = %v", detectable)
	}
}

func referenceSimulationSpec() ClusterSimulationSpec {
	return ClusterSimulationSpec{
		SourceTasks: 48, MutationsPerCell: 2, Replications: 120, Seed: 20260809, CodeDigest: strings.Repeat("a", 64),
		Endpoint: EndpointContinuousDistribution, Baseline: 0, ResidualSD: 1, IntraclusterCorrelation: 0.25,
		Factors:            []FactorEffect{{ID: "failure_evidence", Effect: 0.8}, {ID: "formatting", Effect: 0}},
		Interactions:       []InteractionEffect{{ID: "failure_x_formatting", Factors: []string{"failure_evidence", "formatting"}, Effect: 0.35}},
		SparseCellFraction: 1, InvalidRate: 0.02, MissingRate: 0.02, AbstentionRate: 0.01, RouteFailureRate: 0.01,
		Alpha: 0.05, FamilySize: 3, CallsPerSourceTask: 1, CallsPerObservation: 1,
		InputTokensPerSourceTask: 1000, InputTokensPerObservation: 1000,
		HardCalls: 1000, HardInputTokens: 1_000_000,
	}
}

func codedSimulationSpec() ClusterSimulationSpec {
	factors := []FactorEffect{
		{ID: "command_exit", Effect: 0.05}, {ID: "error_output", Effect: 0.05},
		{ID: "executable_outcome", Effect: 0.05}, {ID: "irrelevant_verbosity", Effect: 0},
		{ID: "metadata", Effect: 0}, {ID: "patch_edit", Effect: 0.05},
		{ID: "presentation_order", Effect: 0}, {ID: "prompt_injection", Effect: -0.05},
		{ID: "success_failure_prose", Effect: 0.05}, {ID: "test_result", Effect: 0.05},
		{ID: "tool_output", Effect: 0.05},
	}
	masks := []CodedFactorMask{
		{FactorID: "command_exit", Mask: 48}, {FactorID: "error_output", Mask: 43},
		{FactorID: "executable_outcome", Mask: 51}, {FactorID: "irrelevant_verbosity", Mask: 50},
		{FactorID: "metadata", Mask: 56}, {FactorID: "patch_edit", Mask: 58},
		{FactorID: "presentation_order", Mask: 52}, {FactorID: "prompt_injection", Mask: 55},
		{FactorID: "success_failure_prose", Mask: 33}, {FactorID: "test_result", Mask: 53},
		{FactorID: "tool_output", Mask: 60},
	}
	return ClusterSimulationSpec{
		SourceTasks: 4, MutationsPerCell: 1, Replications: 2, Seed: 20260814,
		CodeDigest: strings.Repeat("b", 64), Endpoint: EndpointContinuousDistribution,
		Baseline: 0.5, ResidualSD: 0.15, IntraclusterCorrelation: 0.25, Factors: factors,
		Interactions: []InteractionEffect{
			{ID: "error_output_x_prompt_injection", Factors: []string{"error_output", "prompt_injection"}, Effect: -0.05},
			{ID: "executable_outcome_x_success_failure_prose", Factors: []string{"executable_outcome", "success_failure_prose"}, Effect: -0.05},
			{ID: "success_failure_prose_x_presentation_order", Factors: []string{"success_failure_prose", "presentation_order"}, Effect: 0.05},
			{ID: "test_result_x_success_failure_prose", Factors: []string{"test_result", "success_failure_prose"}, Effect: -0.05},
		},
		CodedDesign:        &CodedFactorialDesign{Algorithm: CodedFactorialAlgorithm, Runs: 64, FactorMasks: masks},
		SparseCellFraction: 1, Alpha: 0.05, FamilySize: 98,
	}
}

func mustSimulationTerms(t *testing.T, spec ClusterSimulationSpec) []simulationTerm {
	t.Helper()
	terms, _, _, _, err := validateSimulationSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	return terms
}
