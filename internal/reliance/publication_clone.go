package reliance

import (
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
)

func cloneRelianceArmComparison(value RelianceArmComparison) RelianceArmComparison {
	result := value
	result.Arms = slices.Clone(value.Arms)
	result.Contrasts = make([]RelianceArmContrast, len(value.Contrasts))
	for index, contrast := range value.Contrasts {
		result.Contrasts[index] = cloneRelianceArmContrast(contrast)
	}
	return result
}

func cloneRelianceArmContrast(value RelianceArmContrast) RelianceArmContrast {
	result := value
	result.ChangedDimensions = slices.Clone(value.ChangedDimensions)
	result.PairingStatusCounts = slices.Clone(value.PairingStatusCounts)
	result.OutcomeFits = make([]RelianceArmContrastOutcomeFit, len(value.OutcomeFits))
	for index, outcome := range value.OutcomeFits {
		result.OutcomeFits[index] = outcome
		result.OutcomeFits[index].Fit = cloneClusteredFactorialFit(outcome.Fit)
	}
	return result
}

func cloneClusteredFactorialFit(value *stats.ClusteredFactorialFit) *stats.ClusteredFactorialFit {
	if value == nil {
		return nil
	}
	result := *value
	result.Estimates = slices.Clone(value.Estimates)
	return &result
}

func cloneRelianceWitness(value RelianceWitness) RelianceWitness {
	result := value
	result.Counterexample = cloneRelianceCounterexample(value.Counterexample)
	result.Evaluations = cloneRelianceWitnessEvaluations(value.Evaluations)
	result.FinalUnits = slices.Clone(value.FinalUnits)
	return result
}

func cloneRelianceCounterexample(value stress.Counterexample) stress.Counterexample {
	result := value
	result.OriginalUnits = slices.Clone(value.OriginalUnits)
	result.FinalUnits = slices.Clone(value.FinalUnits)
	result.Steps = slices.Clone(value.Steps)
	return result
}
