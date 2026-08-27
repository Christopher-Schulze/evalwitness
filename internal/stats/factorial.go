package stats

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

const ClusteredFactorialFitSchemaVersion = "evalwitness.clustered-factorial-fit.v1"

var ErrFactorialNotEstimable = errors.New("clustered factorial model is not estimable")

type FactorialTerm struct {
	ID      string   `json:"id"`
	Factors []string `json:"factors"`
}

type FactorialLevel struct {
	FactorID string  `json:"factor_id"`
	Level    float64 `json:"level"`
}

type FactorialObservation struct {
	ObservationID string           `json:"observation_id"`
	ClusterID     string           `json:"cluster_id"`
	Levels        []FactorialLevel `json:"levels"`
	Outcome       float64          `json:"outcome"`
}

type FactorialEstimate struct {
	TermID         string  `json:"term_id"`
	Estimate       float64 `json:"estimate"`
	StandardError  float64 `json:"standard_error"`
	Lower          float64 `json:"lower"`
	Upper          float64 `json:"upper"`
	RawPValue      float64 `json:"raw_p_value"`
	AdjustedPValue float64 `json:"adjusted_p_value"`
	FamilyAdjusted bool    `json:"family_adjusted"`
}

type ClusteredFactorialFit struct {
	SchemaVersion       string              `json:"schema_version"`
	Method              string              `json:"method"`
	Observations        int                 `json:"observations"`
	Clusters            int                 `json:"clusters"`
	Parameters          int                 `json:"parameters"`
	Rank                int                 `json:"rank"`
	NominalAlpha        float64             `json:"nominal_alpha"`
	FamilySize          int                 `json:"family_size"`
	FamilyAdjustedAlpha float64             `json:"family_adjusted_alpha"`
	CriticalValue       float64             `json:"critical_value"`
	Estimates           []FactorialEstimate `json:"estimates"`
}

func FitClusteredFactorial(
	terms []FactorialTerm,
	observations []FactorialObservation,
	nominalAlpha float64,
	familySize int,
) (ClusteredFactorialFit, error) {
	validatedTerms, factors, err := validateFactorialTerms(terms)
	if err != nil {
		return ClusteredFactorialFit{}, err
	}
	if !(nominalAlpha > 0 && nominalAlpha < 0.5) || familySize < len(validatedTerms) {
		return ClusteredFactorialFit{}, errors.New("factorial fit requires alpha in (0,0.5) and a family covering every fitted term")
	}
	ordered, clusters, err := validateFactorialObservations(observations, factors)
	if err != nil {
		return ClusteredFactorialFit{}, err
	}
	parameterNames := append([]string{"intercept"}, factorialTermIDs(validatedTerms)...)
	rows, matrix := factorialRegressionInputs(ordered, validatedTerms, clusters)
	rank, aliased := matrixRankAndAliased(matrix, parameterNames)
	if rank != len(parameterNames) {
		return ClusteredFactorialFit{}, fmt.Errorf(
			"%w: design is rank %d of %d; aliased terms: %s",
			ErrFactorialNotEstimable, rank, len(parameterNames), strings.Join(aliased, ","),
		)
	}
	fit, err := clusterRobustOLS(rows, len(parameterNames), len(clusters))
	if err != nil {
		return ClusteredFactorialFit{}, fmt.Errorf("%w: %v", ErrFactorialNotEstimable, err)
	}
	return clusteredFactorialReport(validatedTerms, len(rows), len(clusters), rank, nominalAlpha, familySize, fit), nil
}

func factorialRegressionInputs(
	ordered []FactorialObservation,
	terms []FactorialTerm,
	clusters map[string]int,
) ([]simulationObservation, [][]float64) {
	rows := make([]simulationObservation, len(ordered))
	matrix := make([][]float64, len(ordered))
	for index, observation := range ordered {
		row := factorialObservationRow(observation.Levels, terms)
		matrix[index] = row
		rows[index] = simulationObservation{cluster: clusters[observation.ClusterID], x: row, y: observation.Outcome}
	}
	return rows, matrix
}

func clusteredFactorialReport(
	terms []FactorialTerm,
	observations int,
	clusters int,
	rank int,
	nominalAlpha float64,
	familySize int,
	fit olsFit,
) ClusteredFactorialFit {
	adjustedAlpha := nominalAlpha / float64(familySize)
	critical := math.Sqrt2 * math.Erfinv(1-adjustedAlpha)
	report := ClusteredFactorialFit{
		SchemaVersion: ClusteredFactorialFitSchemaVersion, Method: "source_task_cluster_sandwich_ols.v1",
		Observations: observations, Clusters: clusters, Parameters: len(terms) + 1, Rank: rank,
		NominalAlpha: nominalAlpha, FamilySize: familySize, FamilyAdjustedAlpha: adjustedAlpha, CriticalValue: critical,
		Estimates: make([]FactorialEstimate, len(terms)),
	}
	for index, term := range terms {
		estimate := 2 * fit.coefficients[index+1]
		standardError := 2 * fit.standardErrors[index+1]
		rawPValue := factorialPValue(estimate, standardError)
		report.Estimates[index] = FactorialEstimate{
			TermID: term.ID, Estimate: estimate, StandardError: standardError,
			Lower: estimate - critical*standardError, Upper: estimate + critical*standardError,
			RawPValue: rawPValue, AdjustedPValue: math.Min(1, rawPValue*float64(familySize)), FamilyAdjusted: true,
		}
	}
	return report
}

func validateFactorialTerms(terms []FactorialTerm) ([]FactorialTerm, []string, error) {
	if len(terms) == 0 {
		return nil, nil, errors.New("factorial fit requires at least one term")
	}
	result := make([]FactorialTerm, len(terms))
	factorSet := make(map[string]struct{})
	termIDs := make(map[string]struct{}, len(terms))
	for index, term := range terms {
		term.ID = strings.TrimSpace(term.ID)
		term.Factors = slices.Clone(term.Factors)
		if term.ID == "" || len(term.Factors) == 0 || !slices.IsSorted(term.Factors) {
			return nil, nil, errors.New("factorial term requires an ID and sorted factors")
		}
		if _, duplicate := termIDs[term.ID]; duplicate {
			return nil, nil, fmt.Errorf("factorial fit repeats term %q", term.ID)
		}
		termIDs[term.ID] = struct{}{}
		seenFactors := make(map[string]struct{}, len(term.Factors))
		for _, factor := range term.Factors {
			if factor == "" || factor != strings.TrimSpace(factor) {
				return nil, nil, fmt.Errorf("factorial term %q has an invalid factor", term.ID)
			}
			if _, duplicate := seenFactors[factor]; duplicate {
				return nil, nil, fmt.Errorf("factorial term %q repeats factor %q", term.ID, factor)
			}
			seenFactors[factor] = struct{}{}
			factorSet[factor] = struct{}{}
		}
		result[index] = term
	}
	slices.SortFunc(result, func(left, right FactorialTerm) int { return strings.Compare(left.ID, right.ID) })
	factors := make([]string, 0, len(factorSet))
	for factor := range factorSet {
		factors = append(factors, factor)
	}
	slices.Sort(factors)
	return result, factors, nil
}

func validateFactorialObservations(
	observations []FactorialObservation,
	factors []string,
) ([]FactorialObservation, map[string]int, error) {
	if len(observations) == 0 {
		return nil, nil, errors.New("factorial fit requires observations")
	}
	ordered := make([]FactorialObservation, len(observations))
	observationIDs := make(map[string]struct{}, len(observations))
	clusterSet := make(map[string]struct{})
	for index, observation := range observations {
		observation.Levels = slices.Clone(observation.Levels)
		if observation.ObservationID == "" || observation.ObservationID != strings.TrimSpace(observation.ObservationID) ||
			observation.ClusterID == "" || observation.ClusterID != strings.TrimSpace(observation.ClusterID) ||
			math.IsNaN(observation.Outcome) || math.IsInf(observation.Outcome, 0) {
			return nil, nil, errors.New("factorial observation identity, cluster, or outcome is invalid")
		}
		if _, duplicate := observationIDs[observation.ObservationID]; duplicate {
			return nil, nil, fmt.Errorf("factorial fit repeats observation %q", observation.ObservationID)
		}
		observationIDs[observation.ObservationID] = struct{}{}
		if err := validateFactorialLevels(observation.Levels, factors); err != nil {
			return nil, nil, fmt.Errorf("observation %q: %w", observation.ObservationID, err)
		}
		clusterSet[observation.ClusterID] = struct{}{}
		ordered[index] = observation
	}
	slices.SortFunc(ordered, func(left, right FactorialObservation) int {
		if value := strings.Compare(left.ClusterID, right.ClusterID); value != 0 {
			return value
		}
		return strings.Compare(left.ObservationID, right.ObservationID)
	})
	return ordered, indexFactorialClusters(clusterSet), nil
}

func indexFactorialClusters(clusterSet map[string]struct{}) map[string]int {
	clusterIDs := make([]string, 0, len(clusterSet))
	for clusterID := range clusterSet {
		clusterIDs = append(clusterIDs, clusterID)
	}
	sort.Strings(clusterIDs)
	clusters := make(map[string]int, len(clusterIDs))
	for index, clusterID := range clusterIDs {
		clusters[clusterID] = index
	}
	return clusters
}

func validateFactorialLevels(levels []FactorialLevel, factors []string) error {
	if len(levels) != len(factors) {
		return errors.New("factorial levels do not cover the exact factor set")
	}
	for index, level := range levels {
		if level.FactorID != factors[index] || level.Level != -1 && level.Level != 1 {
			return errors.New("factorial levels must be sorted, complete, and coded as -1 or +1")
		}
	}
	return nil
}

func factorialObservationRow(levels []FactorialLevel, terms []FactorialTerm) []float64 {
	levelByFactor := make(map[string]float64, len(levels))
	for _, level := range levels {
		levelByFactor[level.FactorID] = level.Level
	}
	row := make([]float64, len(terms)+1)
	row[0] = 1
	for index, term := range terms {
		value := 1.0
		for _, factor := range term.Factors {
			value *= levelByFactor[factor]
		}
		row[index+1] = value
	}
	return row
}

func factorialTermIDs(terms []FactorialTerm) []string {
	result := make([]string, len(terms))
	for index, term := range terms {
		result[index] = term.ID
	}
	return result
}

func factorialPValue(estimate, standardError float64) float64 {
	if standardError > 0 {
		return math.Erfc(math.Abs(estimate/standardError) / math.Sqrt2)
	}
	if math.Abs(estimate) <= 1e-15 {
		return 1
	}
	return 0
}
