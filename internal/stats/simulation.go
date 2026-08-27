package stats

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"sort"
)

const (
	ClusterSimulationAlgorithm = "evalwitness.cluster-factorial.v1"
	CodedFactorialAlgorithm    = "evalwitness.walsh-coded-factorial.v1"
)

type SimulationEndpoint string

const (
	EndpointBinaryDecision         SimulationEndpoint = "binary_decision"
	EndpointContinuousDistribution SimulationEndpoint = "continuous_distribution"
)

type FactorEffect struct {
	ID     string  `json:"id"`
	Effect float64 `json:"effect"`
}

type InteractionEffect struct {
	ID      string   `json:"id"`
	Factors []string `json:"factors"`
	Effect  float64  `json:"effect"`
}

type CodedFactorMask struct {
	FactorID string `json:"factor_id"`
	Mask     uint64 `json:"mask"`
}

type CodedFactorialDesign struct {
	Algorithm   string            `json:"algorithm"`
	Runs        int               `json:"runs"`
	FactorMasks []CodedFactorMask `json:"factor_masks"`
}

type ClusterSimulationSpec struct {
	SourceTasks               int                   `json:"source_tasks"`
	MutationsPerCell          int                   `json:"mutations_per_cell"`
	Replications              int                   `json:"replications"`
	Seed                      int64                 `json:"seed"`
	CodeDigest                string                `json:"code_digest"`
	Endpoint                  SimulationEndpoint    `json:"endpoint"`
	Baseline                  float64               `json:"baseline"`
	ResidualSD                float64               `json:"residual_sd"`
	IntraclusterCorrelation   float64               `json:"intracluster_correlation"`
	Factors                   []FactorEffect        `json:"factors"`
	Interactions              []InteractionEffect   `json:"interactions,omitempty"`
	CodedDesign               *CodedFactorialDesign `json:"coded_design,omitempty"`
	SparseCellFraction        float64               `json:"sparse_cell_fraction"`
	InvalidRate               float64               `json:"invalid_rate"`
	MissingRate               float64               `json:"missing_rate"`
	AbstentionRate            float64               `json:"abstention_rate"`
	RouteFailureRate          float64               `json:"route_failure_rate"`
	Alpha                     float64               `json:"alpha"`
	FamilySize                int                   `json:"family_size"`
	CallsPerSourceTask        int                   `json:"calls_per_source_task"`
	CallsPerObservation       int                   `json:"calls_per_observation"`
	InputTokensPerSourceTask  int                   `json:"input_tokens_per_source_task"`
	InputTokensPerObservation int                   `json:"input_tokens_per_observation"`
	HardCalls                 int                   `json:"hard_calls"`
	HardInputTokens           int                   `json:"hard_input_tokens"`
}

type SimulationBudget struct {
	PlannedCells        int `json:"planned_cells"`
	PlannedObservations int `json:"planned_observations"`
	RequiredCalls       int `json:"required_calls"`
	RequiredInputTokens int `json:"required_input_tokens"`
	HardCalls           int `json:"hard_calls"`
	HardInputTokens     int `json:"hard_input_tokens"`
}

type DesignAliasing struct {
	Parameters   int      `json:"parameters"`
	Rank         int      `json:"rank"`
	AliasedTerms []string `json:"aliased_terms,omitempty"`
}

type MonteCarloOperatingCharacteristic struct {
	Term                string  `json:"term"`
	DeclaredEffect      float64 `json:"declared_effect"`
	DeclaredEffectScale string  `json:"declared_effect_scale"`
	EstimateScale       string  `json:"estimate_scale"`
	Estimable           bool    `json:"estimable"`
	MeanEstimate        float64 `json:"mean_estimate"`
	Power               float64 `json:"power"`
	MonteCarloSE        float64 `json:"monte_carlo_se"`
	ValidRuns           int     `json:"valid_runs"`
}

type SimulationDenominators struct {
	Planned      int `json:"planned"`
	Observed     int `json:"observed"`
	Invalid      int `json:"invalid"`
	Missing      int `json:"missing"`
	Abstained    int `json:"abstained"`
	RouteFailure int `json:"route_failure"`
}

type SimulationAssumptions struct {
	MutationsPerCell         int                   `json:"mutations_per_cell"`
	Replications             int                   `json:"replications"`
	SparseCellFraction       float64               `json:"sparse_cell_fraction"`
	InvalidRate              float64               `json:"invalid_rate"`
	MissingRate              float64               `json:"missing_rate"`
	AbstentionRate           float64               `json:"abstention_rate"`
	RouteFailureRate         float64               `json:"route_failure_rate"`
	CallsPerSourceTask       int                   `json:"calls_per_source_task"`
	InputTokensPerSourceTask int                   `json:"input_tokens_per_source_task"`
	Factors                  []FactorEffect        `json:"factors"`
	Interactions             []InteractionEffect   `json:"interactions,omitempty"`
	CodedDesign              *CodedFactorialDesign `json:"coded_design,omitempty"`
}

type ClusterSimulationReport struct {
	Algorithm                string                              `json:"algorithm"`
	AlgorithmDigest          string                              `json:"algorithm_digest"`
	CodeDigest               string                              `json:"code_digest"`
	DesignDigest             string                              `json:"design_digest"`
	Seed                     int64                               `json:"seed"`
	Endpoint                 SimulationEndpoint                  `json:"endpoint"`
	DataGeneratingModel      string                              `json:"data_generating_model"`
	SourceTasks              int                                 `json:"source_tasks"`
	EffectiveSourceTasks     float64                             `json:"effective_source_tasks"`
	IntraclusterCorrelation  float64                             `json:"intracluster_correlation"`
	InvalidAllowance         float64                             `json:"invalid_allowance"`
	FamilyAdjustedAlpha      float64                             `json:"family_adjusted_alpha"`
	Aliasing                 DesignAliasing                      `json:"aliasing"`
	Budget                   SimulationBudget                    `json:"budget"`
	Denominators             SimulationDenominators              `json:"denominators"`
	Assumptions              SimulationAssumptions               `json:"assumptions"`
	OperatingCharacteristics []MonteCarloOperatingCharacteristic `json:"operating_characteristics"`
}

type SimulationSensitivity struct {
	IntraclusterCorrelation float64 `json:"intracluster_correlation"`
	InvalidRate             float64 `json:"invalid_rate"`
	MissingRate             float64 `json:"missing_rate"`
	AbstentionRate          float64 `json:"abstention_rate"`
	RouteFailureRate        float64 `json:"route_failure_rate"`
}

type simulationTerm struct {
	id      string
	columns []int
	effect  float64
}

type plannedObservation struct {
	cluster int
	x       []float64
}

type simulationObservation struct {
	cluster int
	x       []float64
	y       float64
}

// SimulateClusteredFactorial estimates power and failure characteristics for a
// frozen sparse factorial panel. Random intercepts preserve source-task
// clustering; inference uses a cluster-robust sandwich covariance estimator.
func SimulateClusteredFactorial(spec ClusterSimulationSpec) (ClusterSimulationReport, error) {
	terms, model, declaredScale, estimateScale, err := validateSimulationSpec(spec)
	if err != nil {
		return ClusterSimulationReport{}, err
	}
	planned, budget := planFactorialObservations(spec, terms)
	if budget.HardCalls > 0 && budget.RequiredCalls > budget.HardCalls {
		return ClusterSimulationReport{}, fmt.Errorf("factorial design requires %d calls, exceeding hard limit %d", budget.RequiredCalls, budget.HardCalls)
	}
	if budget.HardInputTokens > 0 && budget.RequiredInputTokens > budget.HardInputTokens {
		return ClusterSimulationReport{}, fmt.Errorf("factorial design requires %d input tokens, exceeding hard limit %d", budget.RequiredInputTokens, budget.HardInputTokens)
	}
	parameterNames := append([]string{"intercept"}, termIDs(terms)...)
	designMatrix := make([][]float64, len(planned))
	for index, observation := range planned {
		designMatrix[index] = observation.x
	}
	rank, aliased := matrixRankAndAliased(designMatrix, parameterNames)
	report := ClusterSimulationReport{
		Algorithm: ClusterSimulationAlgorithm, AlgorithmDigest: simulationAlgorithmDigest(), CodeDigest: spec.CodeDigest,
		Seed: spec.Seed, Endpoint: spec.Endpoint, DataGeneratingModel: model, SourceTasks: spec.SourceTasks,
		IntraclusterCorrelation: spec.IntraclusterCorrelation, InvalidAllowance: spec.InvalidRate,
		FamilyAdjustedAlpha: spec.Alpha / float64(spec.FamilySize),
		Aliasing:            DesignAliasing{Parameters: len(parameterNames), Rank: rank, AliasedTerms: aliased}, Budget: budget,
		Assumptions: SimulationAssumptions{
			MutationsPerCell: spec.MutationsPerCell, Replications: spec.Replications,
			SparseCellFraction: spec.SparseCellFraction, InvalidRate: spec.InvalidRate, MissingRate: spec.MissingRate,
			AbstentionRate: spec.AbstentionRate, RouteFailureRate: spec.RouteFailureRate,
			CallsPerSourceTask: spec.CallsPerSourceTask, InputTokensPerSourceTask: spec.InputTokensPerSourceTask,
			Factors: append([]FactorEffect(nil), spec.Factors...), Interactions: cloneInteractionEffects(spec.Interactions),
			CodedDesign: cloneCodedFactorialDesign(spec.CodedDesign),
		},
	}
	report.DesignDigest, err = CanonicalSimulationDigest(spec)
	if err != nil {
		return ClusterSimulationReport{}, fmt.Errorf("digest simulation design: %w", err)
	}
	estimable := make(map[string]bool, len(terms))
	for _, term := range terms {
		estimable[term.id] = !containsString(aliased, term.id)
	}
	rejections := make([]int, len(terms))
	validRuns := make([]int, len(terms))
	estimateSums := make([]float64, len(terms))
	rng := rand.New(rand.NewSource(spec.Seed))
	effectiveSourceTaskSum := 0
	for range spec.Replications {
		observations, denominators := generateSimulationObservations(spec, terms, planned, rng)
		addSimulationDenominators(&report.Denominators, denominators)
		clustersObserved := make([]bool, spec.SourceTasks)
		for _, observation := range observations {
			clustersObserved[observation.cluster] = true
		}
		for _, observed := range clustersObserved {
			if observed {
				effectiveSourceTaskSum++
			}
		}
		fit, fitErr := clusterRobustOLS(observations, len(parameterNames), spec.SourceTasks)
		if fitErr != nil {
			continue
		}
		critical := math.Sqrt2 * math.Erfinv(1-report.FamilyAdjustedAlpha)
		for index, term := range terms {
			if !estimable[term.id] || index+1 >= len(fit.coefficients) || fit.standardErrors[index+1] <= 0 {
				continue
			}
			validRuns[index]++
			estimate := 2 * fit.coefficients[index+1]
			estimateSums[index] += estimate
			if math.Abs(fit.coefficients[index+1]/fit.standardErrors[index+1]) > critical {
				rejections[index]++
			}
		}
	}
	report.EffectiveSourceTasks = float64(effectiveSourceTaskSum) / float64(spec.Replications)
	for index, term := range terms {
		characteristic := MonteCarloOperatingCharacteristic{
			Term: term.id, DeclaredEffect: term.effect, DeclaredEffectScale: declaredScale,
			EstimateScale: estimateScale, Estimable: estimable[term.id], ValidRuns: validRuns[index],
		}
		if validRuns[index] > 0 {
			characteristic.MeanEstimate = estimateSums[index] / float64(validRuns[index])
			characteristic.Power = float64(rejections[index]) / float64(validRuns[index])
			characteristic.MonteCarloSE = math.Sqrt(characteristic.Power * (1 - characteristic.Power) / float64(validRuns[index]))
		}
		report.OperatingCharacteristics = append(report.OperatingCharacteristics, characteristic)
	}
	return report, nil
}

// SimulateClusteredSensitivity executes a frozen design over declared ICC and
// failure scenarios. Each cell derives a stable seed from the base seed and
// scenario index, so repeated preflights are byte-reproducible.
func SimulateClusteredSensitivity(base ClusterSimulationSpec, scenarios []SimulationSensitivity) ([]ClusterSimulationReport, error) {
	reports := make([]ClusterSimulationReport, 0, len(scenarios))
	for index, scenario := range scenarios {
		spec := base
		spec.Seed = base.Seed + int64(index)*1_000_003
		spec.IntraclusterCorrelation = scenario.IntraclusterCorrelation
		spec.InvalidRate = scenario.InvalidRate
		spec.MissingRate = scenario.MissingRate
		spec.AbstentionRate = scenario.AbstentionRate
		spec.RouteFailureRate = scenario.RouteFailureRate
		report, err := SimulateClusteredFactorial(spec)
		if err != nil {
			return nil, fmt.Errorf("sensitivity scenario %d: %w", index, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// MinimumDetectableFactorEffect evaluates a prespecified ascending effect grid
// and returns the first effect whose simulated power reaches targetPower.
func MinimumDetectableFactorEffect(spec ClusterSimulationSpec, termID string, effects []float64, targetPower float64) (*float64, []MonteCarloOperatingCharacteristic, error) {
	if !(targetPower > 0 && targetPower < 1) {
		return nil, nil, errors.New("target power must be between zero and one")
	}
	if len(effects) == 0 {
		return nil, nil, errors.New("effect grid must not be empty")
	}
	grid := append([]float64(nil), effects...)
	sort.Float64s(grid)
	results := make([]MonteCarloOperatingCharacteristic, 0, len(grid))
	for _, effect := range grid {
		candidate, found := setSimulationTermEffect(spec, termID, effect)
		if !found {
			return nil, nil, fmt.Errorf("unknown simulation term %q", termID)
		}
		report, err := SimulateClusteredFactorial(candidate)
		if err != nil {
			return nil, nil, err
		}
		characteristic, found := findOperatingCharacteristic(report.OperatingCharacteristics, termID)
		if !found {
			return nil, nil, fmt.Errorf("simulation omitted term %q", termID)
		}
		results = append(results, characteristic)
		if characteristic.Estimable && characteristic.Power >= targetPower {
			value := effect
			return &value, results, nil
		}
	}
	return nil, results, nil
}

func validateSimulationSpec(spec ClusterSimulationSpec) ([]simulationTerm, string, string, string, error) {
	fail := func(err error) ([]simulationTerm, string, string, string, error) {
		return nil, "", "", "", err
	}
	if spec.SourceTasks < 2 || spec.MutationsPerCell < 1 || spec.Replications < 1 {
		return fail(errors.New("source tasks >= 2, mutations per cell >= 1, and replications >= 1 are required"))
	}
	if spec.CodeDigest == "" {
		return fail(errors.New("code digest is required"))
	}
	if len(spec.CodeDigest) != sha256.Size*2 {
		return fail(errors.New("code digest must be a SHA-256 hex digest"))
	}
	if _, err := hex.DecodeString(spec.CodeDigest); err != nil {
		return fail(errors.New("code digest must be a SHA-256 hex digest"))
	}
	if spec.CallsPerSourceTask < 0 || spec.CallsPerObservation < 0 || spec.InputTokensPerSourceTask < 0 ||
		spec.InputTokensPerObservation < 0 || spec.HardCalls < 0 || spec.HardInputTokens < 0 {
		return fail(errors.New("resource coefficients and hard limits must be non-negative"))
	}
	if !(spec.IntraclusterCorrelation >= 0 && spec.IntraclusterCorrelation < 1) {
		return fail(errors.New("intracluster correlation must be in [0, 1)"))
	}
	if !(spec.SparseCellFraction > 0 && spec.SparseCellFraction <= 1) {
		return fail(errors.New("sparse cell fraction must be in (0, 1]"))
	}
	for _, rate := range []float64{spec.InvalidRate, spec.MissingRate, spec.AbstentionRate, spec.RouteFailureRate} {
		if !(rate >= 0 && rate < 1) {
			return fail(errors.New("invalid, missing, abstention, and route failure rates must be in [0, 1)"))
		}
	}
	if spec.InvalidRate+spec.MissingRate+spec.AbstentionRate+spec.RouteFailureRate >= 1 {
		return fail(errors.New("combined failure rates must be below one"))
	}
	if !(spec.Alpha > 0 && spec.Alpha < 0.5) || spec.FamilySize < 1 {
		return fail(errors.New("alpha must be in (0, 0.5) and family size must be positive"))
	}
	if len(spec.Factors) < 1 || (spec.CodedDesign == nil && len(spec.Factors) > 10) || len(spec.Factors) > 63 {
		return fail(errors.New("between one and ten factors are required for enumerated designs; coded designs support up to 63"))
	}
	model := "Gaussian random intercept with cluster-robust linear model"
	declaredScale := "mean_difference"
	estimateScale := "mean_difference"
	switch spec.Endpoint {
	case EndpointBinaryDecision:
		if !(spec.Baseline > 0 && spec.Baseline < 1) {
			return fail(errors.New("binary baseline probability must be in (0, 1)"))
		}
		model = "logistic random intercept with cluster-robust linear-probability analysis"
		declaredScale = "log_odds_contrast"
		estimateScale = "probability_difference"
	case EndpointContinuousDistribution:
		if math.IsNaN(spec.Baseline) || math.IsInf(spec.Baseline, 0) || !(spec.ResidualSD > 0) || math.IsInf(spec.ResidualSD, 0) {
			return fail(errors.New("continuous endpoint requires a finite baseline and finite residual_sd > 0"))
		}
	default:
		return fail(fmt.Errorf("unsupported simulation endpoint %q", spec.Endpoint))
	}
	factorIndexes := make(map[string]int, len(spec.Factors))
	terms := make([]simulationTerm, 0, len(spec.Factors)+len(spec.Interactions))
	for index, factor := range spec.Factors {
		if factor.ID == "" {
			return fail(errors.New("factor ID is required"))
		}
		if math.IsNaN(factor.Effect) || math.IsInf(factor.Effect, 0) {
			return fail(fmt.Errorf("factor %q effect must be finite", factor.ID))
		}
		if _, exists := factorIndexes[factor.ID]; exists {
			return fail(fmt.Errorf("duplicate factor %q", factor.ID))
		}
		factorIndexes[factor.ID] = index
		terms = append(terms, simulationTerm{id: factor.ID, columns: []int{index}, effect: factor.Effect})
	}
	termIDsSeen := make(map[string]struct{}, len(terms)+len(spec.Interactions))
	for _, term := range terms {
		termIDsSeen[term.id] = struct{}{}
	}
	for _, interaction := range spec.Interactions {
		if interaction.ID == "" || len(interaction.Factors) < 2 {
			return fail(errors.New("interaction ID and at least two factors are required"))
		}
		if _, exists := termIDsSeen[interaction.ID]; exists {
			return fail(fmt.Errorf("duplicate term %q", interaction.ID))
		}
		if math.IsNaN(interaction.Effect) || math.IsInf(interaction.Effect, 0) {
			return fail(fmt.Errorf("interaction %q effect must be finite", interaction.ID))
		}
		columns := make([]int, len(interaction.Factors))
		seenColumns := make(map[int]struct{}, len(columns))
		for index, factorID := range interaction.Factors {
			column, found := factorIndexes[factorID]
			if !found {
				return fail(fmt.Errorf("interaction %q references unknown factor %q", interaction.ID, factorID))
			}
			if _, duplicate := seenColumns[column]; duplicate {
				return fail(fmt.Errorf("interaction %q repeats factor %q", interaction.ID, factorID))
			}
			seenColumns[column] = struct{}{}
			columns[index] = column
		}
		termIDsSeen[interaction.ID] = struct{}{}
		terms = append(terms, simulationTerm{id: interaction.ID, columns: columns, effect: interaction.Effect})
	}
	if err := validateCodedFactorialDesign(spec); err != nil {
		return fail(err)
	}
	return terms, model, declaredScale, estimateScale, nil
}
func planFactorialObservations(spec ClusterSimulationSpec, terms []simulationTerm) ([]plannedObservation, SimulationBudget) {
	if spec.CodedDesign != nil {
		return planCodedFactorialObservations(spec, terms)
	}
	rng := rand.New(rand.NewSource(spec.Seed ^ 0x5eed5eed))
	cells := 1 << len(spec.Factors)
	planned := make([]plannedObservation, 0, spec.SourceTasks*cells*spec.MutationsPerCell)
	for cluster := 0; cluster < spec.SourceTasks; cluster++ {
		included := 0
		for cell := 0; cell < cells; cell++ {
			if spec.SparseCellFraction < 1 && rng.Float64() > spec.SparseCellFraction {
				continue
			}
			included++
			x := factorialRow(cell, len(spec.Factors), terms)
			for range spec.MutationsPerCell {
				planned = append(planned, plannedObservation{cluster: cluster, x: x})
			}
		}
		if included == 0 {
			x := factorialRow(cluster%cells, len(spec.Factors), terms)
			for range spec.MutationsPerCell {
				planned = append(planned, plannedObservation{cluster: cluster, x: x})
			}
		}
	}
	budget := SimulationBudget{
		PlannedCells: len(planned) / spec.MutationsPerCell, PlannedObservations: len(planned),
		RequiredCalls:       spec.SourceTasks*spec.CallsPerSourceTask + len(planned)*spec.CallsPerObservation,
		RequiredInputTokens: spec.SourceTasks*spec.InputTokensPerSourceTask + len(planned)*spec.InputTokensPerObservation,
		HardCalls:           spec.HardCalls, HardInputTokens: spec.HardInputTokens,
	}
	return planned, budget
}

func validateCodedFactorialDesign(spec ClusterSimulationSpec) error {
	if spec.CodedDesign == nil {
		return nil
	}
	design := spec.CodedDesign
	if design.Algorithm != CodedFactorialAlgorithm {
		return fmt.Errorf("coded factorial algorithm must be %q", CodedFactorialAlgorithm)
	}
	if design.Runs < 2 || design.Runs > 1<<20 || design.Runs&(design.Runs-1) != 0 {
		return errors.New("coded factorial runs must be a power of two between 2 and 1048576")
	}
	if spec.SparseCellFraction != 1 {
		return errors.New("coded factorial designs require sparse_cell_fraction=1 because the coded rows are already exact")
	}
	if len(design.FactorMasks) != len(spec.Factors) {
		return errors.New("coded factorial design must provide exactly one mask per factor")
	}
	factors := make(map[string]struct{}, len(spec.Factors))
	for _, factor := range spec.Factors {
		factors[factor.ID] = struct{}{}
	}
	seenFactors := make(map[string]struct{}, len(design.FactorMasks))
	seenMasks := make(map[uint64]struct{}, len(design.FactorMasks))
	for _, factor := range design.FactorMasks {
		if _, exists := factors[factor.FactorID]; !exists {
			return fmt.Errorf("coded factorial mask references unknown factor %q", factor.FactorID)
		}
		if _, duplicate := seenFactors[factor.FactorID]; duplicate {
			return fmt.Errorf("coded factorial repeats factor %q", factor.FactorID)
		}
		if factor.Mask == 0 || factor.Mask >= uint64(design.Runs) {
			return fmt.Errorf("coded factorial mask for %q must be in [1, %d)", factor.FactorID, design.Runs)
		}
		if _, duplicate := seenMasks[factor.Mask]; duplicate {
			return fmt.Errorf("coded factorial repeats mask %d", factor.Mask)
		}
		seenFactors[factor.FactorID] = struct{}{}
		seenMasks[factor.Mask] = struct{}{}
	}
	return nil
}

func planCodedFactorialObservations(spec ClusterSimulationSpec, terms []simulationTerm) ([]plannedObservation, SimulationBudget) {
	masks := make(map[string]uint64, len(spec.CodedDesign.FactorMasks))
	for _, factor := range spec.CodedDesign.FactorMasks {
		masks[factor.FactorID] = factor.Mask
	}
	planned := make([]plannedObservation, 0, spec.SourceTasks*spec.CodedDesign.Runs*spec.MutationsPerCell)
	for cluster := 0; cluster < spec.SourceTasks; cluster++ {
		for run := 0; run < spec.CodedDesign.Runs; run++ {
			factors := make([]float64, len(spec.Factors))
			for index, factor := range spec.Factors {
				factors[index] = -1
				if bits.OnesCount64(uint64(run)&masks[factor.ID])%2 == 1 {
					factors[index] = 1
				}
			}
			x := factorialRowFromLevels(factors, terms)
			for range spec.MutationsPerCell {
				planned = append(planned, plannedObservation{cluster: cluster, x: x})
			}
		}
	}
	budget := SimulationBudget{
		PlannedCells: len(planned) / spec.MutationsPerCell, PlannedObservations: len(planned),
		RequiredCalls:       spec.SourceTasks*spec.CallsPerSourceTask + len(planned)*spec.CallsPerObservation,
		RequiredInputTokens: spec.SourceTasks*spec.InputTokensPerSourceTask + len(planned)*spec.InputTokensPerObservation,
		HardCalls:           spec.HardCalls, HardInputTokens: spec.HardInputTokens,
	}
	return planned, budget
}

func factorialRow(cell, factorCount int, terms []simulationTerm) []float64 {
	factors := make([]float64, factorCount)
	for index := range factors {
		factors[index] = -1
		if cell&(1<<index) != 0 {
			factors[index] = 1
		}
	}
	return factorialRowFromLevels(factors, terms)
}

func factorialRowFromLevels(factors []float64, terms []simulationTerm) []float64 {
	row := make([]float64, len(terms)+1)
	row[0] = 1
	for index, term := range terms {
		value := 1.0
		for _, column := range term.columns {
			value *= factors[column]
		}
		row[index+1] = value
	}
	return row
}

func generateSimulationObservations(spec ClusterSimulationSpec, terms []simulationTerm, planned []plannedObservation, rng *rand.Rand) ([]simulationObservation, SimulationDenominators) {
	denominators := SimulationDenominators{Planned: len(planned)}
	clusterEffects := make([]float64, spec.SourceTasks)
	for cluster := range clusterEffects {
		clusterEffects[cluster] = rng.NormFloat64()
	}
	observations := make([]simulationObservation, 0, len(planned))
	for _, item := range planned {
		draw := rng.Float64()
		switch {
		case draw < spec.InvalidRate:
			denominators.Invalid++
			continue
		case draw < spec.InvalidRate+spec.MissingRate:
			denominators.Missing++
			continue
		case draw < spec.InvalidRate+spec.MissingRate+spec.AbstentionRate:
			denominators.Abstained++
			continue
		case draw < spec.InvalidRate+spec.MissingRate+spec.AbstentionRate+spec.RouteFailureRate:
			denominators.RouteFailure++
			continue
		}
		linear := 0.0
		for termIndex, term := range terms {
			linear += item.x[termIndex+1] * term.effect / 2
		}
		var outcome float64
		switch spec.Endpoint {
		case EndpointBinaryDecision:
			sigma := math.Sqrt(spec.IntraclusterCorrelation * math.Pi * math.Pi / (3 * (1 - spec.IntraclusterCorrelation)))
			logit := math.Log(spec.Baseline/(1-spec.Baseline)) + sigma*clusterEffects[item.cluster] + linear
			probability := 1 / (1 + math.Exp(-logit))
			if rng.Float64() < probability {
				outcome = 1
			}
		case EndpointContinuousDistribution:
			clusterSD := spec.ResidualSD * math.Sqrt(spec.IntraclusterCorrelation)
			observationSD := spec.ResidualSD * math.Sqrt(1-spec.IntraclusterCorrelation)
			outcome = spec.Baseline + clusterSD*clusterEffects[item.cluster] + linear + observationSD*rng.NormFloat64()
		}
		observations = append(observations, simulationObservation{cluster: item.cluster, x: item.x, y: outcome})
		denominators.Observed++
	}
	return observations, denominators
}

type olsFit struct {
	coefficients   []float64
	standardErrors []float64
}

func clusterRobustOLS(observations []simulationObservation, parameters, clusters int) (olsFit, error) {
	if len(observations) <= parameters || clusters < 2 {
		return olsFit{}, errors.New("insufficient observations for clustered model")
	}
	xTx := makeMatrix(parameters, parameters)
	xTy := make([]float64, parameters)
	for _, observation := range observations {
		for row := 0; row < parameters; row++ {
			xTy[row] += observation.x[row] * observation.y
			for column := 0; column < parameters; column++ {
				xTx[row][column] += observation.x[row] * observation.x[column]
			}
		}
	}
	inverse, ok := invertMatrix(xTx)
	if !ok {
		return olsFit{}, errors.New("factorial design matrix is singular")
	}
	coefficients := multiplyMatrixVector(inverse, xTy)
	scores := make([][]float64, clusters)
	for cluster := range scores {
		scores[cluster] = make([]float64, parameters)
	}
	observedClusters := make([]bool, clusters)
	observedClusterCount := 0
	for _, observation := range observations {
		residual := observation.y - dot(observation.x, coefficients)
		if !observedClusters[observation.cluster] {
			observedClusters[observation.cluster] = true
			observedClusterCount++
		}
		for parameter := 0; parameter < parameters; parameter++ {
			scores[observation.cluster][parameter] += observation.x[parameter] * residual
		}
	}
	if observedClusterCount < 2 {
		return olsFit{}, errors.New("fewer than two observed clusters")
	}
	meat := makeMatrix(parameters, parameters)
	for cluster, observed := range observedClusters {
		if !observed {
			continue
		}
		for row := 0; row < parameters; row++ {
			for column := 0; column < parameters; column++ {
				meat[row][column] += scores[cluster][row] * scores[cluster][column]
			}
		}
	}
	covariance := multiplyMatrices(multiplyMatrices(inverse, meat), inverse)
	correction := float64(observedClusterCount) / float64(observedClusterCount-1)
	correction *= float64(len(observations)-1) / float64(len(observations)-parameters)
	standardErrors := make([]float64, parameters)
	for index := range standardErrors {
		standardErrors[index] = math.Sqrt(math.Max(0, covariance[index][index]*correction))
	}
	return olsFit{coefficients: coefficients, standardErrors: standardErrors}, nil
}

func matrixRankAndAliased(matrix [][]float64, names []string) (int, []string) {
	if len(matrix) == 0 {
		return 0, append([]string(nil), names...)
	}
	copyMatrix := makeMatrix(len(matrix), len(names))
	for row := range matrix {
		copy(copyMatrix[row], matrix[row])
	}
	pivotColumns := make(map[int]struct{}, len(names))
	pivotRow := 0
	for column := 0; column < len(names) && pivotRow < len(copyMatrix); column++ {
		best := pivotRow
		for row := pivotRow + 1; row < len(copyMatrix); row++ {
			if math.Abs(copyMatrix[row][column]) > math.Abs(copyMatrix[best][column]) {
				best = row
			}
		}
		if math.Abs(copyMatrix[best][column]) < 1e-10 {
			continue
		}
		copyMatrix[pivotRow], copyMatrix[best] = copyMatrix[best], copyMatrix[pivotRow]
		pivot := copyMatrix[pivotRow][column]
		for valueColumn := column; valueColumn < len(names); valueColumn++ {
			copyMatrix[pivotRow][valueColumn] /= pivot
		}
		for row := 0; row < len(copyMatrix); row++ {
			if row == pivotRow {
				continue
			}
			factor := copyMatrix[row][column]
			for valueColumn := column; valueColumn < len(names); valueColumn++ {
				copyMatrix[row][valueColumn] -= factor * copyMatrix[pivotRow][valueColumn]
			}
		}
		pivotColumns[column] = struct{}{}
		pivotRow++
	}
	aliased := make([]string, 0)
	for column, name := range names {
		if _, pivot := pivotColumns[column]; !pivot {
			aliased = append(aliased, name)
		}
	}
	return pivotRow, aliased
}

func invertMatrix(matrix [][]float64) ([][]float64, bool) {
	n := len(matrix)
	augmented := makeMatrix(n, 2*n)
	for row := 0; row < n; row++ {
		copy(augmented[row], matrix[row])
		augmented[row][n+row] = 1
	}
	for column := 0; column < n; column++ {
		pivotRow := column
		for row := column + 1; row < n; row++ {
			if math.Abs(augmented[row][column]) > math.Abs(augmented[pivotRow][column]) {
				pivotRow = row
			}
		}
		if math.Abs(augmented[pivotRow][column]) < 1e-12 {
			return nil, false
		}
		augmented[column], augmented[pivotRow] = augmented[pivotRow], augmented[column]
		pivot := augmented[column][column]
		for valueColumn := 0; valueColumn < 2*n; valueColumn++ {
			augmented[column][valueColumn] /= pivot
		}
		for row := 0; row < n; row++ {
			if row == column {
				continue
			}
			factor := augmented[row][column]
			for valueColumn := 0; valueColumn < 2*n; valueColumn++ {
				augmented[row][valueColumn] -= factor * augmented[column][valueColumn]
			}
		}
	}
	inverse := makeMatrix(n, n)
	for row := 0; row < n; row++ {
		copy(inverse[row], augmented[row][n:])
	}
	return inverse, true
}

func makeMatrix(rows, columns int) [][]float64 {
	matrix := make([][]float64, rows)
	for row := range matrix {
		matrix[row] = make([]float64, columns)
	}
	return matrix
}

func multiplyMatrices(left, right [][]float64) [][]float64 {
	result := makeMatrix(len(left), len(right[0]))
	for row := range left {
		for column := range right[0] {
			for inner := range right {
				result[row][column] += left[row][inner] * right[inner][column]
			}
		}
	}
	return result
}

func multiplyMatrixVector(matrix [][]float64, vector []float64) []float64 {
	result := make([]float64, len(matrix))
	for row := range matrix {
		result[row] = dot(matrix[row], vector)
	}
	return result
}

func dot(left, right []float64) float64 {
	result := 0.0
	for index := range left {
		result += left[index] * right[index]
	}
	return result
}

func addSimulationDenominators(total *SimulationDenominators, next SimulationDenominators) {
	total.Planned += next.Planned
	total.Observed += next.Observed
	total.Invalid += next.Invalid
	total.Missing += next.Missing
	total.Abstained += next.Abstained
	total.RouteFailure += next.RouteFailure
}

func termIDs(terms []simulationTerm) []string {
	ids := make([]string, len(terms))
	for index, term := range terms {
		ids[index] = term.id
	}
	return ids
}

func simulationAlgorithmDigest() string {
	digest := sha256.Sum256([]byte(ClusterSimulationAlgorithm))
	return hex.EncodeToString(digest[:])
}

func setSimulationTermEffect(spec ClusterSimulationSpec, termID string, effect float64) (ClusterSimulationSpec, bool) {
	copySpec := spec
	copySpec.Factors = append([]FactorEffect(nil), spec.Factors...)
	copySpec.Interactions = cloneInteractionEffects(spec.Interactions)
	for index := range copySpec.Factors {
		if copySpec.Factors[index].ID == termID {
			copySpec.Factors[index].Effect = effect
			return copySpec, true
		}
	}
	for index := range copySpec.Interactions {
		if copySpec.Interactions[index].ID == termID {
			copySpec.Interactions[index].Effect = effect
			return copySpec, true
		}
	}
	return ClusterSimulationSpec{}, false
}

func cloneInteractionEffects(interactions []InteractionEffect) []InteractionEffect {
	cloned := make([]InteractionEffect, len(interactions))
	for index, interaction := range interactions {
		cloned[index] = interaction
		cloned[index].Factors = append([]string(nil), interaction.Factors...)
	}
	return cloned
}

func cloneCodedFactorialDesign(design *CodedFactorialDesign) *CodedFactorialDesign {
	if design == nil {
		return nil
	}
	cloned := *design
	cloned.FactorMasks = append([]CodedFactorMask(nil), design.FactorMasks...)
	return &cloned
}

func findOperatingCharacteristic(values []MonteCarloOperatingCharacteristic, termID string) (MonteCarloOperatingCharacteristic, bool) {
	for _, value := range values {
		if value.Term == termID {
			return value, true
		}
	}
	return MonteCarloOperatingCharacteristic{}, false
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// CanonicalSimulationDigest binds the exact assumptions used for a simulation
// preflight without depending on map ordering.
func CanonicalSimulationDigest(spec ClusterSimulationSpec) (string, error) {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
