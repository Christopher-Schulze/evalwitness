package reliance

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

const (
	preflightTargetPower            = 0.80
	preflightMonteCarloReplications = 256
	preflightContinuousEffect       = 0.05
	preflightBinaryLogOddsEffect    = 0.75
	preflightContinuousScenario     = "continuous_distribution_sesoi"
	preflightBinaryScenario         = "binary_transition_sesoi"
	preflightMainSentinel           = "command_exit"
	preflightInteractionSentinel    = "error_output_x_prompt_injection"
)

const reliancePreflightCacheCapacity = 8

type reliancePreflightCacheKey struct {
	preregistrationDigest string
	codeDigest            string
}

type reliancePreflightCacheState struct {
	mu      sync.Mutex
	entries map[reliancePreflightCacheKey]ReliancePreflight
	order   []reliancePreflightCacheKey
}

var reliancePreflightCache = reliancePreflightCacheState{
	entries: make(map[reliancePreflightCacheKey]ReliancePreflight, reliancePreflightCacheCapacity),
}

var preflightCandidateSourceTasks = []int{8, 12, 16, 20, 24, 32, 40}

func BuildReliancePreflight(preregistration Preregistration, codeDigest string) (ReliancePreflight, error) {
	if err := validateFrozenPreregistration(preregistration); err != nil {
		return ReliancePreflight{}, err
	}
	return constructReliancePreflight(preregistration, codeDigest)
}

func (value ReliancePreflight) Validate(preregistration Preregistration) error {
	if err := validateFrozenPreregistration(preregistration); err != nil {
		return err
	}
	if value.SchemaVersion != PreflightSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.Algorithm != PreflightAlgorithm || value.PreregistrationDigest != preregistration.Digest ||
		value.LiveAuthorized || value.EmpiricalAssumptions {
		return errors.New("reliance preflight identity or non-authorizing assumption boundary is invalid")
	}
	digest, err := reliancePreflightDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance preflight digest is invalid")
	}
	expected, err := constructReliancePreflight(preregistration, value.CodeDigest)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("reliance preflight differs from the frozen power search")
	}
	return nil
}

func constructReliancePreflight(preregistration Preregistration, codeDigest string) (ReliancePreflight, error) {
	cacheKey := reliancePreflightCacheKey{
		preregistrationDigest: preregistration.Digest,
		codeDigest:            codeDigest,
	}
	if cached, found, err := loadReliancePreflightCache(cacheKey); err != nil {
		return ReliancePreflight{}, err
	} else if found {
		return cached, nil
	}
	aliasAudit, err := auditReferenceWalshDesign(preregistration)
	if err != nil {
		return ReliancePreflight{}, err
	}
	if !aliasAudit.MainEffectsClearOfTwoFactorTerms || !aliasAudit.DeclaredInteractionsUnique {
		return ReliancePreflight{}, errors.New("selected Walsh design does not satisfy the frozen alias boundary")
	}
	resourceModel := frozenRelianceResourceModel()
	hardBudget := relianceResourceBudget(preflightCandidateSourceTasks[len(preflightCandidateSourceTasks)-1], resourceModel)
	candidates := make([]PreflightCandidate, 0, len(preflightCandidateSourceTasks))
	var selectedReports []PreflightScenarioReport
	selectedSourceTasks := 0
	selectedCandidateIndex := -1
	selectedBudget := RelianceResourceBudget{}
	for index, sourceTasks := range preflightCandidateSourceTasks {
		reports, simulationErr := simulatePreflightCandidate(preregistration, codeDigest, sourceTasks, index, hardBudget, resourceModel)
		if simulationErr != nil {
			return ReliancePreflight{}, simulationErr
		}
		checks, resolved, checkErr := preflightPowerChecks(preregistration, reports)
		if checkErr != nil {
			return ReliancePreflight{}, checkErr
		}
		budget := relianceResourceBudget(sourceTasks, resourceModel)
		candidates = append(candidates, PreflightCandidate{SourceTasks: sourceTasks, Budget: budget, Checks: checks, Resolved: resolved})
		if selectedSourceTasks == 0 && resolved {
			selectedSourceTasks = sourceTasks
			selectedCandidateIndex = index
			selectedBudget = budget
			selectedReports = reports
			break
		}
	}
	status := "resolved"
	var selectedMDEs []PreflightMDE
	if selectedSourceTasks == 0 {
		status = "inconclusive_under_hard_ceiling"
	} else {
		selectedMDEs, err = preflightMDEs(
			preregistration, codeDigest, selectedSourceTasks, selectedCandidateIndex,
			selectedReports, hardBudget, resourceModel,
		)
		if err != nil {
			return ReliancePreflight{}, err
		}
	}
	value := ReliancePreflight{
		SchemaVersion: PreflightSchemaVersion, CanonicalPolicy: CanonicalPolicy, Algorithm: PreflightAlgorithm,
		PreregistrationDigest: preregistration.Digest, CodeDigest: codeDigest, TargetPower: preflightTargetPower,
		MonteCarloReplications: preflightMonteCarloReplications,
		CandidateSourceTasks:   slices.Clone(preflightCandidateSourceTasks), AliasAudit: aliasAudit,
		ResourceModel: resourceModel, HardBudget: hardBudget, Candidates: candidates,
		SelectedSourceTasks: selectedSourceTasks, SelectedBudget: selectedBudget, SelectedScenarios: selectedReports,
		SelectedMDEs: selectedMDEs, Status: status, LiveAuthorized: false, EmpiricalAssumptions: false,
	}
	value.Digest, err = reliancePreflightDigest(value)
	if err != nil {
		return ReliancePreflight{}, err
	}
	storeReliancePreflightCache(cacheKey, value)
	return value, nil
}

func loadReliancePreflightCache(key reliancePreflightCacheKey) (ReliancePreflight, bool, error) {
	reliancePreflightCache.mu.Lock()
	value, found := reliancePreflightCache.entries[key]
	reliancePreflightCache.mu.Unlock()
	if !found {
		return ReliancePreflight{}, false, nil
	}
	return cloneReliancePreflight(value), true, nil
}

func storeReliancePreflightCache(key reliancePreflightCacheKey, value ReliancePreflight) {
	reliancePreflightCache.mu.Lock()
	defer reliancePreflightCache.mu.Unlock()
	if _, exists := reliancePreflightCache.entries[key]; exists {
		reliancePreflightCache.entries[key] = cloneReliancePreflight(value)
		return
	}
	if len(reliancePreflightCache.order) == reliancePreflightCacheCapacity {
		delete(reliancePreflightCache.entries, reliancePreflightCache.order[0])
		reliancePreflightCache.order = reliancePreflightCache.order[1:]
	}
	reliancePreflightCache.entries[key] = cloneReliancePreflight(value)
	reliancePreflightCache.order = append(reliancePreflightCache.order, key)
}

func cloneReliancePreflight(value ReliancePreflight) ReliancePreflight {
	cloned := value
	cloned.CandidateSourceTasks = slices.Clone(value.CandidateSourceTasks)
	cloned.AliasAudit.Candidates = slices.Clone(value.AliasAudit.Candidates)
	cloned.Candidates = slices.Clone(value.Candidates)
	for index := range cloned.Candidates {
		cloned.Candidates[index].Checks = clonePreflightPowerChecks(value.Candidates[index].Checks)
	}
	cloned.SelectedScenarios = slices.Clone(value.SelectedScenarios)
	for index := range cloned.SelectedScenarios {
		cloned.SelectedScenarios[index].Report = cloneSimulationReport(value.SelectedScenarios[index].Report)
	}
	cloned.SelectedMDEs = slices.Clone(value.SelectedMDEs)
	for index := range cloned.SelectedMDEs {
		cloned.SelectedMDEs[index].EffectGrid = slices.Clone(value.SelectedMDEs[index].EffectGrid)
		cloned.SelectedMDEs[index].Points = slices.Clone(value.SelectedMDEs[index].Points)
		for pointIndex := range cloned.SelectedMDEs[index].Points {
			cloned.SelectedMDEs[index].Points[pointIndex].Checks = clonePreflightPowerChecks(
				value.SelectedMDEs[index].Points[pointIndex].Checks,
			)
		}
		if value.SelectedMDEs[index].MinimumDetectableEffect != nil {
			minimum := *value.SelectedMDEs[index].MinimumDetectableEffect
			cloned.SelectedMDEs[index].MinimumDetectableEffect = &minimum
		}
	}
	return cloned
}

func clonePreflightPowerChecks(values []PreflightPowerCheck) []PreflightPowerCheck {
	cloned := slices.Clone(values)
	for index := range cloned {
		cloned[index].AppliesTo = slices.Clone(values[index].AppliesTo)
	}
	return cloned
}

func cloneSimulationReport(value stats.ClusterSimulationReport) stats.ClusterSimulationReport {
	cloned := value
	cloned.Aliasing.AliasedTerms = slices.Clone(value.Aliasing.AliasedTerms)
	cloned.Assumptions.Factors = slices.Clone(value.Assumptions.Factors)
	cloned.Assumptions.Interactions = slices.Clone(value.Assumptions.Interactions)
	for index := range cloned.Assumptions.Interactions {
		cloned.Assumptions.Interactions[index].Factors = slices.Clone(value.Assumptions.Interactions[index].Factors)
	}
	if value.Assumptions.CodedDesign != nil {
		design := *value.Assumptions.CodedDesign
		design.FactorMasks = slices.Clone(value.Assumptions.CodedDesign.FactorMasks)
		cloned.Assumptions.CodedDesign = &design
	}
	cloned.OperatingCharacteristics = slices.Clone(value.OperatingCharacteristics)
	return cloned
}

func simulatePreflightCandidate(
	preregistration Preregistration,
	codeDigest string,
	sourceTasks int,
	candidateIndex int,
	hardBudget RelianceResourceBudget,
	resourceModel RelianceResourceModel,
) ([]PreflightScenarioReport, error) {
	reports := make([]PreflightScenarioReport, 0, 2)
	for _, scenarioID := range []string{preflightContinuousScenario, preflightBinaryScenario} {
		spec := preflightSimulationSpec(preregistration, codeDigest, sourceTasks, candidateIndex, scenarioID, resourceModel, hardBudget)
		report, err := stats.SimulateClusteredFactorial(spec)
		if err != nil {
			return nil, err
		}
		reports = append(reports, PreflightScenarioReport{ScenarioID: scenarioID, Report: report})
	}
	return reports, nil
}

func preflightSimulationSpec(
	preregistration Preregistration,
	codeDigest string,
	sourceTasks int,
	candidateIndex int,
	scenarioID string,
	resourceModel RelianceResourceModel,
	hardBudget RelianceResourceBudget,
) stats.ClusterSimulationSpec {
	factors := make([]stats.FactorEffect, 0, len(preregistration.MainEffects)+1)
	for _, factorID := range preregistration.MainEffects {
		effect := 0.0
		if string(factorID) == preflightMainSentinel {
			effect = preflightContinuousEffect
		}
		factors = append(factors, stats.FactorEffect{ID: string(factorID), Effect: effect})
	}
	factors = append(factors, stats.FactorEffect{ID: PresentationOrderTerm, Effect: 0})
	slices.SortFunc(factors, func(left, right stats.FactorEffect) int {
		if left.ID < right.ID {
			return -1
		}
		if left.ID > right.ID {
			return 1
		}
		return 0
	})
	interactions := make([]stats.InteractionEffect, len(preregistration.Interactions))
	for index, interaction := range preregistration.Interactions {
		effect := 0.0
		if interaction.InteractionID == preflightInteractionSentinel {
			effect = preflightContinuousEffect
		}
		interactions[index] = stats.InteractionEffect{ID: interaction.InteractionID, Factors: slices.Clone(interaction.Terms), Effect: effect}
	}
	endpoint := stats.EndpointContinuousDistribution
	baseline := 0.5
	residualSD := 0.15
	seedOffset := int64(0)
	if scenarioID == preflightBinaryScenario {
		endpoint = stats.EndpointBinaryDecision
		baseline = 0.20
		residualSD = 0
		seedOffset = 500_009
		for index := range factors {
			if factors[index].ID == preflightMainSentinel {
				factors[index].Effect = preflightBinaryLogOddsEffect
			}
		}
		for index := range interactions {
			if interactions[index].ID == preflightInteractionSentinel {
				interactions[index].Effect = preflightBinaryLogOddsEffect
			}
		}
	}
	masks := canonicalReferenceMasks()
	codedMasks := make([]stats.CodedFactorMask, len(masks))
	for index, factor := range masks {
		codedMasks[index] = stats.CodedFactorMask{FactorID: factor.FactorID, Mask: factor.Mask}
	}
	inputTokensPerCall := resourceModel.EvidenceTokensPerCall + resourceModel.PromptTokensPerCall
	return stats.ClusterSimulationSpec{
		SourceTasks: sourceTasks, MutationsPerCell: 1, Replications: preflightMonteCarloReplications,
		Seed: 20260814 + int64(candidateIndex)*1_000_003 + seedOffset, CodeDigest: codeDigest,
		Endpoint: endpoint, Baseline: baseline, ResidualSD: residualSD, IntraclusterCorrelation: 0.25,
		Factors: factors, Interactions: interactions,
		CodedDesign:        &stats.CodedFactorialDesign{Algorithm: stats.CodedFactorialAlgorithm, Runs: ReferenceCellsPerTask, FactorMasks: codedMasks},
		SparseCellFraction: 1, InvalidRate: 0.05, MissingRate: 0.03, AbstentionRate: 0.05, RouteFailureRate: 0.02,
		Alpha: 0.05, FamilySize: preregistration.Multiplicity.FamilySize,
		CallsPerSourceTask: resourceModel.BaselineCallsPerTask, CallsPerObservation: resourceModel.CallsPerCell,
		InputTokensPerSourceTask:  inputTokensPerCall * resourceModel.BaselineCallsPerTask,
		InputTokensPerObservation: inputTokensPerCall * resourceModel.CallsPerCell,
		HardCalls:                 hardBudget.LogicalCalls, HardInputTokens: hardBudget.HardInputTokens,
	}
}

func preflightPowerChecks(preregistration Preregistration, reports []PreflightScenarioReport) ([]PreflightPowerCheck, bool, error) {
	checks := make([]PreflightPowerCheck, 0, 4)
	for _, scenario := range reports {
		for _, termID := range []string{preflightMainSentinel, preflightInteractionSentinel} {
			characteristic, found := preflightOperatingCharacteristic(scenario.Report, termID)
			if !found || !characteristic.Estimable || characteristic.ValidRuns != preflightMonteCarloReplications {
				return nil, false, errors.New("preflight simulation omitted a valid sentinel operating characteristic")
			}
			lower := wilsonLower95(characteristic.Power, characteristic.ValidRuns)
			appliesTo := make([]string, 0)
			if termID == preflightMainSentinel {
				for _, factorID := range preregistration.MainEffects {
					appliesTo = append(appliesTo, string(factorID))
				}
			} else {
				for _, interaction := range preregistration.Interactions {
					appliesTo = append(appliesTo, interaction.InteractionID)
				}
			}
			checks = append(checks, PreflightPowerCheck{
				ScenarioID: scenario.ScenarioID, TermID: termID, AppliesTo: appliesTo, DeclaredEffect: characteristic.DeclaredEffect,
				MeanEstimate: characteristic.MeanEstimate, Power: characteristic.Power,
				MonteCarloSE: characteristic.MonteCarloSE, PowerLower95: lower,
			})
		}
	}
	resolved := true
	for _, check := range checks {
		if check.PowerLower95 < preflightTargetPower {
			resolved = false
		}
	}
	return checks, resolved, nil
}

func preflightMDEs(
	preregistration Preregistration,
	codeDigest string,
	sourceTasks int,
	candidateIndex int,
	selectedReports []PreflightScenarioReport,
	hardBudget RelianceResourceBudget,
	resourceModel RelianceResourceModel,
) ([]PreflightMDE, error) {
	grids := map[string][]float64{
		preflightContinuousScenario: {0.02, 0.03, 0.04, 0.05},
		preflightBinaryScenario:     {0.25, 0.50, 0.75},
	}
	values := make([]PreflightMDE, 0, len(selectedReports))
	for _, selected := range selectedReports {
		grid := grids[selected.ScenarioID]
		points := make([]PreflightMDEPoint, 0, len(grid))
		var minimum *float64
		declaredScale := ""
		estimateScale := ""
		for _, effect := range grid {
			report := selected.Report
			selectedEffect := preflightContinuousEffect
			if selected.ScenarioID == preflightBinaryScenario {
				selectedEffect = preflightBinaryLogOddsEffect
			}
			if effect != selectedEffect {
				spec := preflightSimulationSpec(
					preregistration, codeDigest, sourceTasks, candidateIndex,
					selected.ScenarioID, resourceModel, hardBudget,
				)
				setPreflightSentinelEffects(&spec, effect)
				var err error
				report, err = stats.SimulateClusteredFactorial(spec)
				if err != nil {
					return nil, err
				}
			}
			checks, resolved, err := preflightPowerChecks(
				preregistration,
				[]PreflightScenarioReport{{ScenarioID: selected.ScenarioID, Report: report}},
			)
			if err != nil {
				return nil, err
			}
			if len(checks) != 2 {
				return nil, errors.New("preflight MDE grid omitted a sentinel contrast class")
			}
			characteristic, found := preflightOperatingCharacteristic(report, preflightMainSentinel)
			if !found {
				return nil, errors.New("preflight MDE report omitted its main-effect sentinel")
			}
			declaredScale = characteristic.DeclaredEffectScale
			estimateScale = characteristic.EstimateScale
			points = append(points, PreflightMDEPoint{DeclaredEffect: effect, Checks: checks, Resolved: resolved})
			if resolved && minimum == nil {
				value := effect
				minimum = &value
			}
		}
		values = append(values, PreflightMDE{
			ScenarioID: selected.ScenarioID, DeclaredEffectScale: declaredScale, EstimateScale: estimateScale,
			EffectGrid: slices.Clone(grid), Points: points, MinimumDetectableEffect: minimum,
		})
	}
	return values, nil
}

func setPreflightSentinelEffects(spec *stats.ClusterSimulationSpec, effect float64) {
	for index := range spec.Factors {
		if spec.Factors[index].ID == preflightMainSentinel {
			spec.Factors[index].Effect = effect
		}
	}
	for index := range spec.Interactions {
		if spec.Interactions[index].ID == preflightInteractionSentinel {
			spec.Interactions[index].Effect = effect
		}
	}
}

func wilsonLower95(proportion float64, sampleSize int) float64 {
	const z = 1.959963984540054
	if sampleSize <= 0 {
		return 0
	}
	n := float64(sampleSize)
	zSquared := z * z
	denominator := 1 + zSquared/n
	center := (proportion + zSquared/(2*n)) / denominator
	margin := z * math.Sqrt(proportion*(1-proportion)/n+zSquared/(4*n*n)) / denominator
	return math.Max(0, center-margin)
}

func preflightOperatingCharacteristic(
	report stats.ClusterSimulationReport,
	termID string,
) (stats.MonteCarloOperatingCharacteristic, bool) {
	for _, characteristic := range report.OperatingCharacteristics {
		if characteristic.Term == termID {
			return characteristic, true
		}
	}
	return stats.MonteCarloOperatingCharacteristic{}, false
}

func frozenRelianceResourceModel() RelianceResourceModel {
	return RelianceResourceModel{
		BillingModel: "subscription", BaselineCallsPerTask: 1, CallsPerCell: 1,
		EvidenceTokensPerCall: 32_000, PromptTokensPerCall: 4_000, MaximumOutputTokens: 4_096,
		MaximumRetries: 5, RequestTimeoutSeconds: 120, Concurrency: 8, MarginalCostUSDPerCall: 0,
	}
}

func relianceResourceBudget(sourceTasks int, model RelianceResourceModel) RelianceResourceBudget {
	calls := sourceTasks * (model.BaselineCallsPerTask + ReferenceCellsPerTask*model.CallsPerCell)
	attempts := calls * (model.MaximumRetries + 1)
	inputTokens := attempts * (model.EvidenceTokensPerCall + model.PromptTokensPerCall)
	outputTokens := attempts * model.MaximumOutputTokens
	waves := (attempts + model.Concurrency - 1) / model.Concurrency
	return RelianceResourceBudget{
		LogicalCalls: calls, HardAttempts: attempts, HardInputTokens: inputTokens, HardOutputTokens: outputTokens,
		HardDurationSeconds: waves * model.RequestTimeoutSeconds, HardConcurrency: model.Concurrency,
		HardCostUSD: float64(attempts) * model.MarginalCostUSDPerCall,
	}
}

func reliancePreflightDigest(value ReliancePreflight) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
