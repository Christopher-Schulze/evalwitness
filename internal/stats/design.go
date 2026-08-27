package stats

import (
	"errors"
	"fmt"
	"math"
)

type InferenceQuestion string

const (
	QuestionSuperiority    InferenceQuestion = "superiority"
	QuestionNonInferiority InferenceQuestion = "non_inferiority"
	QuestionEquivalence    InferenceQuestion = "equivalence"
)

type RejectionRegion struct {
	Discordant int     `json:"discordant"`
	Alpha      float64 `json:"alpha"`
	RejectedA  []int   `json:"rejected_a_wins"`
	LowerMax   *int    `json:"lower_tail_max,omitempty"`
	UpperMin   *int    `json:"upper_tail_min,omitempty"`
}

type PairedInterval struct {
	Method     string  `json:"method"`
	Confidence float64 `json:"confidence"`
	Estimate   float64 `json:"estimate"`
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
}

type PairedInference struct {
	Question    InferenceQuestion `json:"question"`
	Margin      float64           `json:"margin"`
	Alpha       float64           `json:"alpha"`
	PValue      float64           `json:"p_value"`
	Interval    PairedInterval    `json:"interval"`
	Established bool              `json:"established"`
	Conclusion  string            `json:"conclusion"`
}

type DetectableEffect struct {
	TotalTasks                int      `json:"total_tasks"`
	DisagreementRate          float64  `json:"disagreement_rate"`
	ExpectedDiscordant        float64  `json:"expected_discordant"`
	Alpha                     float64  `json:"alpha"`
	TargetPower               float64  `json:"target_power"`
	MinimumPairedEffect       *float64 `json:"minimum_paired_effect,omitempty"`
	DiscordantWinProbability  *float64 `json:"discordant_win_probability,omitempty"`
	PowerAtCompleteSeparation float64  `json:"power_at_complete_separation"`
}

type ExactBinomialDesign struct {
	Tasks                  int     `json:"tasks"`
	NullProbability        float64 `json:"null_probability"`
	AlternativeProbability float64 `json:"alternative_probability"`
	Alpha                  float64 `json:"alpha"`
	CriticalSuccesses      int     `json:"critical_successes"`
	ActualNullTail         float64 `json:"actual_null_tail"`
	Power                  float64 `json:"power"`
}

// PairedInferenceConfidence returns the minimum two-sided interval confidence
// that supports the declared question at alpha. Superiority uses a two-sided
// 1-alpha interval; non-inferiority and equivalence use the 1-2*alpha interval
// corresponding to one-sided alpha tests.
func PairedInferenceConfidence(question InferenceQuestion, alpha float64) (float64, error) {
	if !(alpha > 0 && alpha < 0.5) {
		return 0, errors.New("alpha must be between zero and one half")
	}
	switch question {
	case QuestionSuperiority:
		return 1 - alpha, nil
	case QuestionNonInferiority, QuestionEquivalence:
		return 1 - 2*alpha, nil
	default:
		return 0, fmt.Errorf("unknown inference question %q", question)
	}
}

// ExactMcNemarRejectionRegion enumerates every A-only win count rejected by the
// repository's two-sided exact conditional McNemar test. The test uses p < alpha
// consistently with McNemarExact and SmallestSignificantSplit.
func ExactMcNemarRejectionRegion(discordant int, alpha float64) (RejectionRegion, error) {
	if discordant < 0 {
		return RejectionRegion{}, errors.New("discordant count must be non-negative")
	}
	if !(alpha > 0 && alpha < 1) {
		return RejectionRegion{}, errors.New("alpha must be between zero and one")
	}
	region := RejectionRegion{Discordant: discordant, Alpha: alpha}
	lowerBoundary := -1
	logTail := math.Inf(-1)
	logDenominator := float64(discordant) * math.Ln2
	for aWins := 0; aWins <= discordant/2; aWins++ {
		logTail = logSumExp(logTail, logBinomial(discordant, aWins))
		pValue := math.Min(1, 2*math.Exp(logTail-logDenominator))
		if pValue < alpha {
			lowerBoundary = aWins
		}
	}
	if lowerBoundary < 0 {
		return region, nil
	}
	lower := lowerBoundary
	upper := discordant - lowerBoundary
	region.LowerMax = &lower
	region.UpperMin = &upper
	for aWins := 0; aWins <= lowerBoundary; aWins++ {
		region.RejectedA = append(region.RejectedA, aWins)
	}
	for aWins := upper; aWins <= discordant; aWins++ {
		region.RejectedA = append(region.RejectedA, aWins)
	}
	return region, nil
}

// ExactMcNemarPower sums the declared alternative Binomial(discordant, q)
// distribution over the exact rejection region. It is a design calculation,
// not a transformation of an observed p-value.
func ExactMcNemarPower(discordant int, discordantWinProbability, alpha float64) (float64, error) {
	if !(discordantWinProbability >= 0 && discordantWinProbability <= 1) {
		return 0, errors.New("discordant win probability must be between zero and one")
	}
	region, err := ExactMcNemarRejectionRegion(discordant, alpha)
	if err != nil {
		return 0, err
	}
	power := 0.0
	for _, aWins := range region.RejectedA {
		power += binomialPMF(discordant, aWins, discordantWinProbability)
	}
	return clampProbability(power), nil
}

// ExactMcNemarUnconditionalPower averages conditional exact power over the
// prespecified Binomial(totalTasks, disagreementRate) distribution of the
// discordant count.
func ExactMcNemarUnconditionalPower(totalTasks int, disagreementRate, discordantWinProbability, alpha float64) (float64, error) {
	if totalTasks < 0 {
		return 0, errors.New("total task count must be non-negative")
	}
	if !(disagreementRate >= 0 && disagreementRate <= 1) {
		return 0, errors.New("disagreement rate must be between zero and one")
	}
	if !(discordantWinProbability >= 0 && discordantWinProbability <= 1) {
		return 0, errors.New("discordant win probability must be between zero and one")
	}
	if !(alpha > 0 && alpha < 1) {
		return 0, errors.New("alpha must be between zero and one")
	}
	power := 0.0
	for discordant := 0; discordant <= totalTasks; discordant++ {
		conditional, err := ExactMcNemarPower(discordant, discordantWinProbability, alpha)
		if err != nil {
			return 0, err
		}
		power += binomialPMF(totalTasks, discordant, disagreementRate) * conditional
	}
	return clampProbability(power), nil
}

// MinimumDetectablePairedEffect finds the smallest absolute task-level success
// rate difference whose unconditional exact power reaches targetPower. The
// paired effect is disagreementRate*(2*q-1), with q >= 0.5 declared as A's win
// probability among discordant tasks.
func MinimumDetectablePairedEffect(totalTasks int, disagreementRate, alpha, targetPower float64) (DetectableEffect, error) {
	row := DetectableEffect{
		TotalTasks: totalTasks, DisagreementRate: disagreementRate,
		ExpectedDiscordant: float64(totalTasks) * disagreementRate,
		Alpha:              alpha, TargetPower: targetPower,
	}
	if totalTasks < 0 {
		return row, errors.New("total task count must be non-negative")
	}
	if !(disagreementRate >= 0 && disagreementRate <= 1) {
		return row, errors.New("disagreement rate must be between zero and one")
	}
	if !(targetPower > 0 && targetPower < 1) {
		return row, errors.New("target power must be between zero and one")
	}
	complete, err := ExactMcNemarUnconditionalPower(totalTasks, disagreementRate, 1, alpha)
	if err != nil {
		return row, err
	}
	row.PowerAtCompleteSeparation = complete
	if disagreementRate == 0 || complete < targetPower {
		return row, nil
	}
	low, high := 0.5, 1.0
	for range 60 {
		mid := (low + high) / 2
		power, powerErr := ExactMcNemarUnconditionalPower(totalTasks, disagreementRate, mid, alpha)
		if powerErr != nil {
			return row, powerErr
		}
		if power >= targetPower {
			high = mid
		} else {
			low = mid
		}
	}
	effect := disagreementRate * (2*high - 1)
	row.MinimumPairedEffect = &effect
	row.DiscordantWinProbability = &high
	return row, nil
}

// RequiredDiscordantPairs returns the smallest fixed informative sample whose
// exact conditional power reaches the target under the declared q.
func RequiredDiscordantPairs(discordantWinProbability, alpha, targetPower float64, maximum int) (int, error) {
	if maximum < 1 {
		return 0, errors.New("maximum must be positive")
	}
	for discordant := 1; discordant <= maximum; discordant++ {
		power, err := ExactMcNemarPower(discordant, discordantWinProbability, alpha)
		if err != nil {
			return 0, err
		}
		if power >= targetPower {
			return discordant, nil
		}
	}
	return 0, fmt.Errorf("target power is not reached within %d discordant pairs", maximum)
}

// ExactBinomialUpperDesign computes the smallest upper-tail rejection boundary
// whose exact null probability is below alpha and its power under a prespecified
// alternative. Independent source tasks, not generated descendants, are the
// sampling units.
func ExactBinomialUpperDesign(tasks int, nullProbability, alternativeProbability, alpha float64) (ExactBinomialDesign, error) {
	result := ExactBinomialDesign{
		Tasks: tasks, NullProbability: nullProbability, AlternativeProbability: alternativeProbability, Alpha: alpha,
		CriticalSuccesses: tasks + 1,
	}
	if tasks < 1 {
		return result, errors.New("exact binomial design requires at least one task")
	}
	if !(nullProbability >= 0 && nullProbability < 1) || !(alternativeProbability > nullProbability && alternativeProbability <= 1) {
		return result, errors.New("exact binomial design requires 0 <= null < alternative <= 1")
	}
	if !(alpha > 0 && alpha < 1) {
		return result, errors.New("exact binomial design alpha must be between zero and one")
	}
	for successes := 0; successes <= tasks; successes++ {
		nullTail := binomialUpperTail(tasks, successes, nullProbability)
		if nullTail < alpha {
			result.CriticalSuccesses = successes
			result.ActualNullTail = nullTail
			result.Power = binomialUpperTail(tasks, successes, alternativeProbability)
			break
		}
	}
	return result, nil
}

// PairedBinaryScoreInterval implements Newcombe's paired score interval
// (method 10) for the marginal success-rate difference A-B.
func PairedBinaryScoreInterval(both, aOnly, bOnly, neither int, confidence float64) (PairedInterval, error) {
	counts := []int{both, aOnly, bOnly, neither}
	total := 0
	for _, count := range counts {
		if count < 0 {
			return PairedInterval{}, errors.New("paired table counts must be non-negative")
		}
		total += count
	}
	if total == 0 {
		return PairedInterval{}, errors.New("paired table must contain at least one task")
	}
	if !(confidence > 0 && confidence < 1) {
		return PairedInterval{}, errors.New("confidence must be between zero and one")
	}
	n := float64(total)
	pA := float64(both+aOnly) / n
	pB := float64(both+bOnly) / n
	estimate := pA - pB
	z := math.Sqrt2 * math.Erfinv(confidence)
	lowerA, upperA := wilsonInterval(both+aOnly, total, z)
	lowerB, upperB := wilsonInterval(both+bOnly, total, z)
	rho := 0.0
	denominator := math.Sqrt(pA * (1 - pA) * pB * (1 - pB))
	if denominator > 0 {
		p11 := float64(both) / n
		p10 := float64(aOnly) / n
		p01 := float64(bOnly) / n
		p00 := float64(neither) / n
		rho = (p11*p00 - p10*p01) / denominator
		rho = math.Max(-1, math.Min(1, rho))
	}
	lowerRadicand := square(pA-lowerA) + square(upperB-pB) - 2*rho*(pA-lowerA)*(upperB-pB)
	upperRadicand := square(upperA-pA) + square(pB-lowerB) - 2*rho*(upperA-pA)*(pB-lowerB)
	lower := estimate - math.Sqrt(math.Max(0, lowerRadicand))
	upper := estimate + math.Sqrt(math.Max(0, upperRadicand))
	return PairedInterval{
		Method: "newcombe_paired_score_method_10", Confidence: confidence, Estimate: estimate,
		Lower: math.Max(-1, lower), Upper: math.Min(1, upper),
	}, nil
}

// EvaluatePairedQuestion keeps superiority, non-inferiority, and equivalence
// conclusions typed. In particular, a non-significant superiority result can
// never become equivalence.
func EvaluatePairedQuestion(question InferenceQuestion, margin, alpha float64, interval PairedInterval, pValue float64) (PairedInference, error) {
	requiredConfidence, err := PairedInferenceConfidence(question, alpha)
	if err != nil {
		return PairedInference{}, err
	}
	if interval.Confidence+1e-12 < requiredConfidence {
		return PairedInference{}, fmt.Errorf("interval confidence %.6f is below the %.6f required for %s at alpha %.6f", interval.Confidence, requiredConfidence, question, alpha)
	}
	if margin < 0 || margin >= 1 {
		return PairedInference{}, errors.New("margin must be in [0, 1)")
	}
	result := PairedInference{Question: question, Margin: margin, Alpha: alpha, PValue: pValue, Interval: interval}
	switch question {
	case QuestionSuperiority:
		result.Established = pValue < alpha && interval.Lower > 0
		result.Conclusion = "superiority_not_established"
		if result.Established {
			result.Conclusion = "superiority_established"
		}
	case QuestionNonInferiority:
		if margin == 0 {
			return PairedInference{}, errors.New("non-inferiority requires a positive margin")
		}
		result.Established = interval.Lower > -margin
		result.Conclusion = "non_inferiority_not_established"
		if result.Established {
			result.Conclusion = "non_inferiority_established"
		}
	case QuestionEquivalence:
		if margin == 0 {
			return PairedInference{}, errors.New("equivalence requires a positive margin")
		}
		result.Established = interval.Lower > -margin && interval.Upper < margin
		result.Conclusion = "equivalence_not_established"
		if result.Established {
			result.Conclusion = "equivalence_established"
		}
	default:
		return PairedInference{}, fmt.Errorf("unknown inference question %q", question)
	}
	return result, nil
}

// PoissonBinomialUpperTail returns P(X >= observed) for independent Bernoulli
// variables with non-identical probabilities using exact convolution.
func PoissonBinomialUpperTail(probabilities []float64, observed int) (float64, error) {
	if observed < 0 || observed > len(probabilities) {
		return 0, errors.New("observed successes must be between zero and the number of probabilities")
	}
	distribution := []float64{1}
	for _, probability := range probabilities {
		if !(probability >= 0 && probability <= 1) {
			return 0, errors.New("poisson-binomial probabilities must be between zero and one")
		}
		next := make([]float64, len(distribution)+1)
		for successes, mass := range distribution {
			next[successes] += mass * (1 - probability)
			next[successes+1] += mass * probability
		}
		distribution = next
	}
	tail := 0.0
	for _, mass := range distribution[observed:] {
		tail += mass
	}
	return clampProbability(tail), nil
}

func wilsonInterval(successes, total int, z float64) (float64, float64) {
	n := float64(total)
	p := float64(successes) / n
	z2 := z * z
	center := (p + z2/(2*n)) / (1 + z2/n)
	half := z / (1 + z2/n) * math.Sqrt(p*(1-p)/n+z2/(4*n*n))
	return math.Max(0, center-half), math.Min(1, center+half)
}

func binomialPMF(n, k int, probability float64) float64 {
	if k < 0 || k > n {
		return 0
	}
	if probability == 0 {
		if k == 0 {
			return 1
		}
		return 0
	}
	if probability == 1 {
		if k == n {
			return 1
		}
		return 0
	}
	logMass := logBinomial(n, k) + float64(k)*math.Log(probability) + float64(n-k)*math.Log1p(-probability)
	return math.Exp(logMass)
}

func binomialUpperTail(n, observed int, probability float64) float64 {
	tail := 0.0
	for successes := observed; successes <= n; successes++ {
		tail += binomialPMF(n, successes, probability)
	}
	return clampProbability(tail)
}

func clampProbability(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func square(value float64) float64 {
	return value * value
}
