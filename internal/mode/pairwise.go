package mode

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/log"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/sprt"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	"golang.org/x/sync/errgroup"
)

const (
	selectionSchemaVersion    = "evalwitness.selection.v2"
	pairDecisionSchemaVersion = "evalwitness.pair-decision.v2"
)

type PairwiseConfig struct {
	NReps                int
	Epsilon              float64
	BiasMitigation       string
	InconsistencyPolicy  string
	SingleElim           bool
	UseSPRT              bool
	SPRTParams           sprt.Params
	MaxWorkers           int
	MaxPairCalls         int
	ConfidenceThreshold  float64
	CalibrationSigma     float64
	ConfidenceEscalation string
}

type PairwiseInput struct {
	Task                 string
	Trajectories         []string
	PreparedTrajectories []preprocess.Result
	Criteria             []verifier.Criterion
	Cfg                  PairwiseConfig
}

type Selection struct {
	SchemaVersion      string                         `json:"schema_version"`
	State              verifier.DecisionState         `json:"state"`
	AbstentionReason   verifier.AbstentionReason      `json:"abstention_reason,omitempty"`
	BestIndex          int                            `json:"best_index"`
	Confidence         float64                        `json:"decision_strength"`
	Scores             []float64                      `json:"conditional_scores"`
	Wins               []float64                      `json:"wins"`
	EvidenceStrength   verifier.EvidenceStrength      `json:"evidence_strength"`
	InconsistentPairs  [][2]int                       `json:"inconsistent_pairs,omitempty"`
	Usage              UsageSummary                   `json:"usage"`
	PairsEvaluated     int                            `json:"pairs_evaluated,omitempty"`
	EscalatedPairs     int                            `json:"escalated_pairs,omitempty"`
	PairDecisions      []PairDecision                 `json:"pair_decisions,omitempty"`
	AbsoluteEvidence   []AbsoluteTrajectoryEvidence   `json:"absolute_evidence,omitempty"`
	TrajectoryEvidence []preprocess.AccountingSummary `json:"trajectory_evidence"`
}

type PairDecision struct {
	SchemaVersion    string                    `json:"schema_version"`
	PolicyVersion    string                    `json:"policy_version"`
	State            verifier.DecisionState    `json:"state"`
	AbstentionReason verifier.AbstentionReason `json:"abstention_reason,omitempty"`
	Pair             [2]int                    `json:"pair"`
	OrderPolicy      string                    `json:"order_policy"`
	FirstOrder       string                    `json:"first_order"`
	Calls            int                       `json:"calls"`
	RepeatCount      int                       `json:"repeat_count"`
	MeanDifference   float64                   `json:"conditional_mean_difference"`
	Variance         float64                   `json:"-"`
	ScoreMass        float64                   `json:"-"`
	WinProbability   float64                   `json:"model_win_probability"`
	Calibrated       bool                      `json:"calibrated"`
	Confidence       float64                   `json:"decision_strength"`
	Winner           int                       `json:"winner"`
	Inconsistent     bool                      `json:"inconsistent,omitempty"`
	Uncertainty      PairUncertainty           `json:"uncertainty"`
	EvidenceStrength verifier.EvidenceStrength `json:"evidence_strength"`
	Observations     []PairObservationRecord   `json:"observations"`
}

type PairUncertainty struct {
	ConditionalTokenVariance  float64 `json:"conditional_token_variance"`
	RepeatedSampleVariance    float64 `json:"repeated_sample_variance"`
	PresentationOrderVariance float64 `json:"presentation_order_variance"`
	PolicyVariance            float64 `json:"policy_variance"`
	TotalVariance             float64 `json:"total_variance"`
}

type PairObservationRecord struct {
	Order                     string             `json:"order"`
	Repeat                    int                `json:"repeat"`
	ConditionalScoreA         float64            `json:"conditional_score_a"`
	ConditionalScoreB         float64            `json:"conditional_score_b"`
	ConditionalMeanDifference float64            `json:"conditional_mean_difference"`
	Criteria                  PairEvidenceScores `json:"criteria"`
}

type pair struct{ i, j int }

type pairResult struct {
	p            pair
	si, sj       float64
	inconsistent bool
	usage        UsageSummary
	decision     PairDecision
	observations []pairObservation
}

type repScore struct {
	i, j        float64
	observation pairObservation
}

type pairState struct {
	p       pair
	mu      sync.Mutex
	xReps   []repScore
	yReps   []repScore
	usage   UsageSummary
	decided bool
}

func RunPairwise(ctx context.Context, r *Runner, in PairwiseInput) (Selection, error) {
	n := len(in.Trajectories)
	if n < 2 {
		return Selection{}, errors.New("pairwise mode requires at least 2 trajectories")
	}
	setPairwiseDefaults(&in.Cfg)
	if in.Cfg.BiasMitigation == "adaptive" && in.Cfg.NReps != 1 {
		return Selection{}, fmt.Errorf("adaptive bias mitigation requires n_reps=1; use both or single for an explicit fixed multi-rep run")
	}
	warnBiasNotBoth("pairwise", in.Cfg.BiasMitigation)

	preps := in.PreparedTrajectories
	if len(preps) > 0 && len(preps) != n {
		return Selection{}, fmt.Errorf("prepared trajectory count %d does not match trajectory count %d", len(preps), n)
	}
	if len(preps) == 0 {
		preps = make([]preprocess.Result, n)
		for i, trajectory := range in.Trajectories {
			var err error
			preps[i], err = r.PrepareTrajectory(trajectory)
			if err != nil {
				return Selection{}, fmt.Errorf("prepare trajectory %d: %w", i, err)
			}
		}
	}
	if in.Cfg.SingleElim {
		return runDynamicTournament(ctx, r, in, preps)
	}

	pairs := generatePairs(n, false)
	if len(pairs) == 0 {
		return Selection{}, errors.New("no pairs generated")
	}

	sel := Selection{TrajectoryEvidence: preprocess.AccountingSummaries(preps...)}
	wins := make([]float64, n)
	scoreSum := make([]float64, n)
	scoreCount := make([]int, n)

	var nonIdentical []pair
	for _, p := range pairs {
		if preps[p.i].Hash == preps[p.j].Hash {
			wins[p.i] += 0.5
			wins[p.j] += 0.5
			continue
		}
		nonIdentical = append(nonIdentical, p)
	}

	if len(nonIdentical) == 0 {
		return finalizeSelection(sel, n, len(pairs), wins, scoreSum, scoreCount), nil
	}

	results := make([]pairResult, len(nonIdentical))
	indices := make([]int, len(nonIdentical))
	for i := range nonIdentical {
		indices[i] = i
	}

	var pairResults map[int]*pairResult
	var err error
	if in.Cfg.BiasMitigation == "adaptive" {
		pairResults, err = adaptivePairsConcurrent(ctx, r, in, preps, nonIdentical, indices)
	} else {
		pairResults, err = stage2PhasedConcurrent(ctx, r, in, preps, nonIdentical, indices)
	}
	if err != nil {
		return sel, err
	}
	for idx, res := range pairResults {
		if res == nil {
			continue
		}
		results[idx].p = res.p
		results[idx].si = res.si
		results[idx].sj = res.sj
		results[idx].inconsistent = res.inconsistent
		results[idx].decision = res.decision
		results[idx].observations = append([]pairObservation(nil), res.observations...)
		AddUsage(&results[idx].usage, res.usage)
		if in.Cfg.BiasMitigation != "adaptive" && results[idx].decision.Calls == 0 {
			results[idx].decision = fixedPairDecision(results[idx].p, results[idx], in)
		}
	}

	for _, res := range results {
		AddUsage(&sel.Usage, res.usage)
		applyPair(res.p, res.si, res.sj, in.Cfg.Epsilon, wins, scoreSum, scoreCount)
		if res.inconsistent {
			sel.InconsistentPairs = append(sel.InconsistentPairs, [2]int{res.p.i, res.p.j})
		}
		if res.decision.Calls > 0 {
			sel.PairDecisions = append(sel.PairDecisions, res.decision)
			sel.PairsEvaluated++
			if res.decision.Calls > pairOrderCallCount(r, in.Criteria) {
				sel.EscalatedPairs++
			}
		}
	}

	return finalizeSelection(sel, n, len(pairs), wins, scoreSum, scoreCount), nil
}

func setPairwiseDefaults(cfg *PairwiseConfig) {
	if cfg.NReps <= 0 {
		cfg.NReps = 1
	}
	if cfg.Epsilon == 0 {
		cfg.Epsilon = 0.02
	}
	if cfg.BiasMitigation == "" {
		cfg.BiasMitigation = "adaptive"
	}
	if cfg.InconsistencyPolicy == "" {
		cfg.InconsistencyPolicy = "flag-only"
	}
	if (cfg.SPRTParams == sprt.Params{}) {
		cfg.SPRTParams = sprt.DefaultParams()
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 8
	}
	// Swept offline over both benchmark caches. Threshold 0.6 with a two-call
	// ceiling holds both benchmark scores; the third call that an earlier sweep
	// kept changed no decision on Terminal-Bench and split 1-1 on SWE-bench
	// while costing 20% and 8% more calls, and a fourth buys nothing at all.
	if cfg.MaxPairCalls <= 0 {
		cfg.MaxPairCalls = 2
	}
	if cfg.ConfidenceThreshold == 0 {
		cfg.ConfidenceThreshold = 0.6
	}
	if cfg.CalibrationSigma == 0 {
		cfg.CalibrationSigma = 0.05
	}
}

type pairObservation struct {
	si, sj             float64
	difference         float64
	variance           float64
	scoreMass          float64
	distributionBacked bool
	reversed           bool
	repeat             int
	evidence           PairEvidenceScores
}

type pairAggregate struct {
	si, sj             float64
	difference         float64
	variance           float64
	scoreMass          float64
	winProbability     float64
	confidence         float64
	distributionBacked bool
	inconsistent       bool
	uncertainty        PairUncertainty
}

// pairFailureBudget bounds how many pair-local failures one bounded run
// tolerates before it stops scheduling further pairs. A single unlucky pair must
// not discard the remaining ones, but a whole worker wave failing means the
// route is down and continuing only burns budget and wall-clock.
const pairFailureBudget = 8

// isFatalPairError separates route-level failures from pair-local ones. Auth,
// configuration, capability and budget failures repeat identically for every
// remaining pair, so there is nothing to gain from attempting them.
func isFatalPairError(err error) bool {
	var budgetErr *BudgetExceededError
	if errors.As(err, &budgetErr) {
		return true
	}
	var providerErr *provider.ProviderError
	if errors.As(err, &providerErr) {
		switch providerErr.Class {
		case provider.ClassAuthFailed, provider.ClassBadConfig, provider.ClassCapabilityMissing:
			return true
		}
	}
	return errors.Is(err, provider.ErrOffline)
}

// runPairSweep runs one concurrent pass over the given pair indices with a
// single failure policy for every selection strategy: a route-level failure
// aborts at once because it repeats for every remaining pair, pair-local
// failures are collected so their siblings still finish and reach the disk
// cache, and a whole worker wave of them stops scheduling because that is a dead
// route rather than bad luck.
func runPairSweep(ctx context.Context, maxWorkers int, indices []int,
	label func(int) string, evaluate func(context.Context, int) error) (map[int]error, error) {
	failed := make(map[int]error)
	var mu sync.Mutex
	var fatal error

	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	for _, idx := range indices {
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-runCtx.Done():
				return
			}
			if runCtx.Err() != nil {
				return
			}
			err := evaluate(runCtx, idx)

			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				delete(failed, idx)
				return
			}
			if cause := runCtx.Err(); cause != nil && errors.Is(err, cause) {
				// Shutdown of this sweep, not a property of the pair.
				return
			}
			wrapped := fmt.Errorf("%s: %w", label(idx), err)
			if isFatalPairError(err) {
				if fatal == nil {
					fatal = wrapped
				}
				stop()
				return
			}
			failed[idx] = wrapped
			if len(failed) >= pairFailureBudget {
				stop()
			}
		}()
	}
	wg.Wait()
	return failed, fatal
}

// recoverFailedPairs gives pairs that failed pair-locally exactly one more
// sweep. Completed calls are already cached, so it re-issues only what is
// missing; a route where everything failed is not retried.
func recoverFailedPairs(ctx context.Context, maxWorkers int, failed map[int]error, total int,
	label func(int) string, evaluate func(context.Context, int) error) (map[int]error, error) {
	if len(failed) == 0 || ctx.Err() != nil {
		return failed, nil
	}
	// A sweep where everything failed is evidence of a dead route only when it was
	// large enough for that to mean something. A two-match bracket round failing
	// twice is bad luck, and refusing to retry it would make the tournament less
	// resilient than all-pairs purely because its rounds are smaller.
	if len(failed) >= total && total >= pairFailureBudget {
		return failed, nil
	}
	retry := sortedKeys(failed)
	log.L().Warn("retrying pairs that returned no extractable distribution",
		"failed_pairs", len(retry), "total_pairs", total)
	stillFailed, fatal := runPairSweep(ctx, maxWorkers, retry, label, evaluate)
	if fatal != nil {
		return failed, fatal
	}
	for idx := range stillFailed {
		failed[idx] = stillFailed[idx]
	}
	for _, idx := range retry {
		if _, still := stillFailed[idx]; !still {
			delete(failed, idx)
		}
	}
	return failed, nil
}

// adaptivePairsConcurrent evaluates every scheduled pair. A pair that fails
// after the provider's own retries no longer aborts its siblings: the remaining
// pairs run to completion so their responses reach the disk cache, and the run
// then fails as a whole. That keeps benchmark results all-or-nothing while
// making every attempt accumulate resumable progress, which is what a restart on
// a rate-limited free route depends on.
func adaptivePairsConcurrent(ctx context.Context, r *Runner, in PairwiseInput, preps []preprocess.Result, allPairs []pair, indices []int) (map[int]*pairResult, error) {
	out := make(map[int]*pairResult, len(indices))
	var mu sync.Mutex

	// Dispatch grouped by the trajectory that ends up in slot A. The prompt is
	// [instructions][task][trajectory A][trajectory B], so consecutive calls in a
	// group share a prefix worth roughly half the request and a provider prompt
	// cache can serve it. Order balancing picks slot A per pair, which otherwise
	// scatters those prefixes across the run: the accepted Run A cached only 7.2%
	// of its input. Ordering costs nothing and also makes dispatch deterministic.
	ordered := append([]int(nil), indices...)
	sort.SliceStable(ordered, func(a, b int) bool {
		return slotATrajectory(in.Task, allPairs[ordered[a]]) < slotATrajectory(in.Task, allPairs[ordered[b]])
	})

	label := func(idx int) string {
		return fmt.Sprintf("adaptive pair (%d,%d)", allPairs[idx].i, allPairs[idx].j)
	}
	evaluate := func(runCtx context.Context, idx int) error {
		res, err := evaluateAdaptivePair(runCtx, r, in, preps, allPairs[idx])
		if err != nil {
			return err
		}
		mu.Lock()
		out[idx] = &res
		mu.Unlock()
		return nil
	}

	failed, fatal := runPairSweep(ctx, in.Cfg.MaxWorkers, ordered, label, evaluate)
	if fatal != nil {
		return nil, fatal
	}
	failed, fatal = recoverFailedPairs(ctx, in.Cfg.MaxWorkers, failed, len(ordered), label, evaluate)
	if fatal != nil {
		return nil, fatal
	}
	if err := reportPairFailures(failed, len(ordered), len(out)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// reportPairFailures turns surviving pair-local failures into one run-level
// error. The run fails as a whole so no partial benchmark result is published,
// while the completed pairs stay cached for the next attempt.
func reportPairFailures(failed map[int]error, total, completed int) error {
	if len(failed) == 0 {
		return nil
	}
	reported := make([]error, 0, 3)
	for _, idx := range sortedKeys(failed) {
		if len(reported) == 3 {
			break
		}
		reported = append(reported, failed[idx])
	}
	return fmt.Errorf("%d of %d pairs failed after provider retries and one recovery sweep, %d completed pairs cached for the next attempt: %w",
		len(failed), total, completed, errors.Join(reported...))
}

func sortedKeys(m map[int]error) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func evaluateAdaptivePair(ctx context.Context, r *Runner, in PairwiseInput, preps []preprocess.Result, p pair) (pairResult, error) {
	perOrderCalls := pairOrderCallCount(r, in.Criteria)
	if perOrderCalls > in.Cfg.MaxPairCalls {
		return pairResult{}, fmt.Errorf(
			"adaptive max pair calls %d cannot fit one order requiring %d provider calls; enable criterion bundling or raise the limit",
			in.Cfg.MaxPairCalls,
			perOrderCalls,
		)
	}
	baseReversed := balancedOrderReversed(in.Task, p)
	orders := []struct {
		reversed bool
		rep      int
	}{
		{reversed: baseReversed, rep: 0},
		{reversed: !baseReversed, rep: 0},
		{reversed: baseReversed, rep: 1},
		{reversed: !baseReversed, rep: 1},
	}

	res := pairResult{p: p}
	observations := make([]pairObservation, 0, len(orders))
	for _, order := range orders {
		if len(observations) > 0 && res.usage.Calls+perOrderCalls > in.Cfg.MaxPairCalls {
			break
		}
		traceA, traceB := preps[p.i].Text, preps[p.j].Text
		leftPrep, rightPrep := preps[p.i], preps[p.j]
		if order.reversed {
			traceA, traceB = traceB, traceA
			leftPrep, rightPrep = rightPrep, leftPrep
		}
		pctx := ContextWithAuditMeta(ctx, auditMetaFor(leftPrep, rightPrep))
		scores, used, err := r.ScorePairEvidence(pctx, in.Task, traceA, traceB, in.Criteria, order.rep)
		if err != nil {
			return res, err
		}
		AddUsage(&res.usage, used)
		observations = append(observations, observationFromEvidence(scores, order.reversed, order.rep))
		aggregate := aggregatePairObservations(observations, in.Cfg.CalibrationSigma)
		if shouldStopAdaptivePair(aggregate, in.Cfg) {
			break
		}
	}

	aggregate := aggregatePairObservations(observations, in.Cfg.CalibrationSigma)
	res.si = aggregate.si
	res.sj = aggregate.sj
	res.inconsistent = aggregate.inconsistent
	res.observations = append([]pairObservation(nil), observations...)
	res.decision = PairDecision{
		SchemaVersion:  pairDecisionSchemaVersion,
		PolicyVersion:  verifier.StrictPolicyVersion,
		Pair:           [2]int{p.i, p.j},
		OrderPolicy:    in.Cfg.BiasMitigation,
		FirstOrder:     pairOrderName(baseReversed),
		Calls:          res.usage.Calls,
		RepeatCount:    countPairRepeats(observations),
		MeanDifference: aggregate.difference,
		Variance:       aggregate.variance,
		ScoreMass:      aggregate.scoreMass,
		WinProbability: aggregate.winProbability,
		Confidence:     aggregate.confidence,
		Winner:         pairWinner(p, aggregate.difference, in.Cfg.Epsilon, in.Task),
		Inconsistent:   aggregate.inconsistent,
		Uncertainty:    aggregate.uncertainty,
		Observations:   pairObservationRecords(observations),
	}
	res.decision.EvidenceStrength = verifier.SummarizeEvidenceStrength(pairEvidenceItems(observations))
	setPairDecisionState(&res.decision, aggregate, in.Cfg)
	return res, nil
}

func pairOrderCallCount(r *Runner, criteria []verifier.Criterion) int {
	if r.Cfg.MultiCriterionBundle && len(criteria) > 1 {
		return 1
	}
	if len(criteria) == 0 {
		return 1
	}
	return len(criteria)
}

// slotATrajectory reports which trajectory the balanced order puts in slot A,
// which is what determines the shared prompt prefix.
func slotATrajectory(task string, p pair) int {
	if balancedOrderReversed(task, p) {
		return p.j
	}
	return p.i
}

func balancedOrderReversed(task string, p pair) bool {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%d", task, p.i, p.j)))
	return sum[0]&1 == 1
}

func pairOrderName(reversed bool) string {
	if reversed {
		return "reversed"
	}
	return "forward"
}

func observationFromEvidence(scores PairEvidenceScores, reversed bool, repeat int) pairObservation {
	obs := pairObservation{scoreMass: 1, distributionBacked: len(scores) > 0, reversed: reversed, repeat: repeat, evidence: scores}
	count := 0
	for _, score := range scores {
		iScore, jScore := score.A, score.B
		if reversed {
			iScore, jScore = jScore, iScore
		}
		obs.si += evidenceScore(iScore)
		obs.sj += evidenceScore(jScore)
		obs.variance += evidenceVariance(iScore) + evidenceVariance(jScore)
		mass := math.Min(iScore.ValidScoreMass, jScore.ValidScoreMass)
		if mass < obs.scoreMass {
			obs.scoreMass = mass
		}
		if iScore.ExtractionMode != verifier.ExtractionModeVerifier || jScore.ExtractionMode != verifier.ExtractionModeVerifier || !iScore.Extracted || !jScore.Extracted {
			obs.distributionBacked = false
		}
		count++
	}
	if count == 0 {
		obs.si = 0.5
		obs.sj = 0.5
		obs.scoreMass = 0
		return obs
	}
	denominator := float64(count)
	obs.si /= denominator
	obs.sj /= denominator
	obs.difference = obs.si - obs.sj
	obs.variance /= denominator * denominator
	if !obs.distributionBacked {
		obs.scoreMass = 0
	}
	return obs
}

func aggregatePairObservations(observations []pairObservation, calibrationSigma float64) pairAggregate {
	if len(observations) == 0 {
		return pairAggregate{si: 0.5, sj: 0.5, winProbability: 0.5}
	}
	aggregate := pairAggregate{scoreMass: 1, distributionBacked: true}
	for _, observation := range observations {
		aggregate.si += observation.si
		aggregate.sj += observation.sj
		aggregate.difference += observation.difference
		aggregate.uncertainty.ConditionalTokenVariance += observation.variance
		if observation.scoreMass < aggregate.scoreMass {
			aggregate.scoreMass = observation.scoreMass
		}
		if !observation.distributionBacked {
			aggregate.distributionBacked = false
		}
	}
	n := float64(len(observations))
	aggregate.si /= n
	aggregate.sj /= n
	aggregate.difference /= n
	aggregate.uncertainty.ConditionalTokenVariance /= n * n
	aggregate.uncertainty.RepeatedSampleVariance = repeatedSampleVariance(observations)
	aggregate.uncertainty.PresentationOrderVariance = presentationOrderVariance(observations)
	if calibrationSigma > 0 {
		aggregate.uncertainty.PolicyVariance = calibrationSigma * calibrationSigma
	}
	aggregate.uncertainty.TotalVariance = aggregate.uncertainty.ConditionalTokenVariance +
		aggregate.uncertainty.RepeatedSampleVariance +
		aggregate.uncertainty.PresentationOrderVariance +
		aggregate.uncertainty.PolicyVariance
	aggregate.variance = aggregate.uncertainty.TotalVariance
	// The model win probability is conditional on the score-token evidence that
	// passed the strict policy. Coverage is never folded into this probability:
	// ScoreEvidence retains valid and unobserved mass separately so downstream
	// audits can reject, stratify, or calibrate without inventing the censored tail.
	aggregate.winProbability = normalWinProbability(aggregate.difference, aggregate.variance)
	if !aggregate.distributionBacked {
		aggregate.scoreMass = 0
	}
	aggregate.confidence = clamp01(2 * math.Abs(aggregate.winProbability-0.5))
	aggregate.inconsistent = observationsInconsistent(observations)
	return aggregate
}

func repeatedSampleVariance(observations []pairObservation) float64 {
	groups := map[bool][]float64{false: {}, true: {}}
	for _, observation := range observations {
		groups[observation.reversed] = append(groups[observation.reversed], observation.difference)
	}
	n := float64(len(observations))
	if n == 0 {
		return 0
	}
	variance := 0.0
	for _, differences := range groups {
		if len(differences) < 2 {
			continue
		}
		groupMean := mean(differences)
		sampleSum := 0.0
		for _, difference := range differences {
			delta := difference - groupMean
			sampleSum += delta * delta
		}
		sampleVariance := sampleSum / float64(len(differences)-1)
		variance += float64(len(differences)) * sampleVariance / (n * n)
	}
	return variance
}

func presentationOrderVariance(observations []pairObservation) float64 {
	forward := []float64{}
	reversed := []float64{}
	for _, observation := range observations {
		if observation.reversed {
			reversed = append(reversed, observation.difference)
		} else {
			forward = append(forward, observation.difference)
		}
	}
	if len(forward) == 0 || len(reversed) == 0 {
		return 0
	}
	difference := mean(forward) - mean(reversed)
	return difference * difference / 4
}

func observationsInconsistent(observations []pairObservation) bool {
	forwardSign := 0
	reverseSign := 0
	for _, observation := range observations {
		sign := signOf(observation.difference)
		if sign == 0 {
			continue
		}
		if observation.reversed {
			reverseSign = sign
		} else {
			forwardSign = sign
		}
	}
	return forwardSign != 0 && reverseSign != 0 && forwardSign != reverseSign
}

func normalWinProbability(difference, variance float64) float64 {
	if variance <= 0 {
		switch {
		case difference > 0:
			return 1
		case difference < 0:
			return 0
		default:
			return 0.5
		}
	}
	return clamp01(0.5 * (1 + math.Erf(difference/math.Sqrt(2*variance))))
}

const (
	ConfidenceEscalationDisabled = "disabled"
	ConfidenceEscalationLegacy   = "legacy"
)

func shouldStopAdaptivePair(aggregate pairAggregate, cfg PairwiseConfig) bool {
	if !aggregate.distributionBacked {
		return true
	}
	if cfg.ConfidenceEscalation != ConfidenceEscalationLegacy {
		return true
	}
	return pairDecisionConfident(aggregate, cfg)
}

func pairDecisionConfident(aggregate pairAggregate, cfg PairwiseConfig) bool {
	if !aggregate.distributionBacked || math.Abs(aggregate.difference) <= cfg.Epsilon {
		return false
	}
	return aggregate.winProbability >= cfg.ConfidenceThreshold || aggregate.winProbability <= 1-cfg.ConfidenceThreshold
}

func pairWinner(p pair, difference, epsilon float64, task string) int {
	switch {
	case difference > epsilon:
		return p.i
	case difference < -epsilon:
		return p.j
	case balancedOrderReversed(task+"\x00tie", p):
		return p.j
	default:
		return p.i
	}
}

func setPairDecisionState(decision *PairDecision, aggregate pairAggregate, cfg PairwiseConfig) {
	switch {
	case aggregate.inconsistent:
		decision.State = verifier.DecisionAbstained
		decision.AbstentionReason = verifier.AbstentionUnstableOrder
		decision.Winner = -1
	case math.Abs(aggregate.difference) <= cfg.Epsilon:
		decision.State = verifier.DecisionTied
		decision.Winner = -1
	case !pairDecisionConfident(aggregate, cfg):
		decision.State = verifier.DecisionAbstained
		decision.AbstentionReason = verifier.AbstentionEvidenceCeiling
		decision.Winner = -1
	default:
		decision.State = verifier.DecisionSelected
	}
}

func countPairRepeats(observations []pairObservation) int {
	repeats := map[int]struct{}{}
	for _, observation := range observations {
		repeats[observation.repeat] = struct{}{}
	}
	return len(repeats)
}

func pairObservationRecords(observations []pairObservation) []PairObservationRecord {
	records := make([]PairObservationRecord, 0, len(observations))
	for _, observation := range observations {
		records = append(records, PairObservationRecord{
			Order:                     pairOrderName(observation.reversed),
			Repeat:                    observation.repeat,
			ConditionalScoreA:         observation.si,
			ConditionalScoreB:         observation.sj,
			ConditionalMeanDifference: observation.difference,
			Criteria:                  observation.evidence,
		})
	}
	return records
}

func pairEvidenceItems(observations []pairObservation) []verifier.ScoreEvidence {
	items := []verifier.ScoreEvidence{}
	for _, observation := range observations {
		for _, criterion := range observation.evidence {
			items = append(items, criterion.A, criterion.B)
		}
	}
	return items
}

func runDynamicTournament(ctx context.Context, r *Runner, in PairwiseInput, preps []preprocess.Result) (Selection, error) {
	n := len(preps)
	active := make([]int, n)
	for i := range active {
		active[i] = i
	}
	sel := Selection{TrajectoryEvidence: preprocess.AccountingSummaries(preps...)}
	wins := make([]float64, n)
	scoreSum := make([]float64, n)
	scoreCount := make([]int, n)
	pathConfidence := make([]float64, n)
	for i := range pathConfidence {
		pathConfidence[i] = 1
	}

	for len(active) > 1 {
		matches := make([]pair, 0, len(active)/2)
		for index := 0; index+1 < len(active); index += 2 {
			matches = append(matches, pair{i: active[index], j: active[index+1]})
		}
		results, err := evaluateTournamentRound(ctx, r, in, preps, matches)
		if err != nil {
			return sel, err
		}
		next := make([]int, 0, (len(active)+1)/2)
		roundUnresolved := false
		roundAbstained := false
		for index, match := range matches {
			res := results[index]
			AddUsage(&sel.Usage, res.usage)
			applyPair(match, res.si, res.sj, in.Cfg.Epsilon, wins, scoreSum, scoreCount)
			if res.inconsistent {
				sel.InconsistentPairs = append(sel.InconsistentPairs, [2]int{match.i, match.j})
			}
			if res.decision.Calls > 0 {
				sel.PairDecisions = append(sel.PairDecisions, res.decision)
				if res.decision.Calls > pairOrderCallCount(r, in.Criteria) {
					sel.EscalatedPairs++
				}
			}
			sel.PairsEvaluated++
			if res.decision.State != verifier.DecisionSelected {
				roundUnresolved = true
				roundAbstained = roundAbstained || res.decision.State == verifier.DecisionAbstained
				continue
			}
			winner := res.decision.Winner
			if winner != match.i && winner != match.j {
				roundUnresolved = true
				continue
			}
			next = append(next, winner)
			pathConfidence[winner] = math.Min(pathConfidence[winner], res.decision.Confidence)
		}
		if roundUnresolved {
			sel = finalizeSelection(sel, n, n-1, wins, scoreSum, scoreCount)
			sel.BestIndex = -1
			sel.Confidence = 0
			if roundAbstained {
				sel.State = verifier.DecisionAbstained
			} else {
				sel.State = verifier.DecisionTied
			}
			return sel, nil
		}
		if len(active)%2 == 1 {
			next = append(next, active[len(active)-1])
		}
		active = next
	}

	sel = finalizeSelection(sel, n, n-1, wins, scoreSum, scoreCount)
	if sel.State == verifier.DecisionSelected {
		sel.BestIndex = active[0]
		sel.Confidence = clamp01(pathConfidence[active[0]])
	}
	return sel, nil
}

// evaluateTournamentRound runs one bracket round under the same failure policy
// as all-pairs. The tournament is the production default, so it must not be the
// less robust path: a single flaky match would otherwise discard the round.
func evaluateTournamentRound(ctx context.Context, r *Runner, in PairwiseInput, preps []preprocess.Result, matches []pair) ([]pairResult, error) {
	results := make([]pairResult, len(matches))
	var mu sync.Mutex

	indices := make([]int, len(matches))
	for index := range matches {
		indices[index] = index
	}
	label := func(index int) string {
		return fmt.Sprintf("tournament pair (%d,%d)", matches[index].i, matches[index].j)
	}
	evaluate := func(runCtx context.Context, index int) error {
		match := matches[index]
		if preps[match.i].Hash == preps[match.j].Hash {
			mu.Lock()
			results[index] = pairResult{p: match, si: 0.5, sj: 0.5}
			mu.Unlock()
			return nil
		}
		res, err := evaluateTournamentPair(runCtx, r, in, preps, match)
		if err != nil {
			return err
		}
		mu.Lock()
		results[index] = res
		mu.Unlock()
		return nil
	}

	failed, fatal := runPairSweep(ctx, in.Cfg.MaxWorkers, indices, label, evaluate)
	if fatal != nil {
		return nil, fatal
	}
	failed, fatal = recoverFailedPairs(ctx, in.Cfg.MaxWorkers, failed, len(indices), label, evaluate)
	if fatal != nil {
		return nil, fatal
	}
	if err := reportPairFailures(failed, len(indices), len(indices)-len(failed)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func evaluateTournamentPair(ctx context.Context, r *Runner, in PairwiseInput, preps []preprocess.Result, p pair) (pairResult, error) {
	if in.Cfg.BiasMitigation == "adaptive" {
		return evaluateAdaptivePair(ctx, r, in, preps, p)
	}

	results, err := stage2PhasedConcurrent(ctx, r, in, preps, []pair{p}, []int{0})
	if err != nil {
		return pairResult{}, err
	}
	res := *results[0]
	res.decision = fixedPairDecision(p, res, in)
	return res, nil
}

func fixedPairDecision(p pair, res pairResult, in PairwiseInput) PairDecision {
	aggregate := aggregatePairObservations(res.observations, in.Cfg.CalibrationSigma)
	difference := aggregate.difference
	if len(res.observations) == 0 {
		difference = res.si - res.sj
		aggregate = pairAggregate{
			si:             res.si,
			sj:             res.sj,
			difference:     difference,
			variance:       in.Cfg.CalibrationSigma * in.Cfg.CalibrationSigma,
			winProbability: normalWinProbability(difference, in.Cfg.CalibrationSigma*in.Cfg.CalibrationSigma),
			uncertainty: PairUncertainty{
				PolicyVariance: in.Cfg.CalibrationSigma * in.Cfg.CalibrationSigma,
				TotalVariance:  in.Cfg.CalibrationSigma * in.Cfg.CalibrationSigma,
			},
		}
	}
	decision := PairDecision{
		SchemaVersion:  pairDecisionSchemaVersion,
		PolicyVersion:  verifier.StrictPolicyVersion,
		Pair:           [2]int{p.i, p.j},
		OrderPolicy:    in.Cfg.BiasMitigation,
		FirstOrder:     "forward",
		Calls:          res.usage.Calls,
		RepeatCount:    countPairRepeats(res.observations),
		MeanDifference: difference,
		Variance:       aggregate.variance,
		ScoreMass:      aggregate.scoreMass,
		WinProbability: aggregate.winProbability,
		Winner:         pairWinner(p, difference, in.Cfg.Epsilon, in.Task),
		Inconsistent:   res.inconsistent,
		Uncertainty:    aggregate.uncertainty,
		Observations:   pairObservationRecords(res.observations),
	}
	decision.Confidence = clamp01(2 * math.Abs(decision.WinProbability-0.5))
	decision.EvidenceStrength = verifier.SummarizeEvidenceStrength(pairEvidenceItems(res.observations))
	setPairDecisionState(&decision, aggregate, in.Cfg)
	return decision
}

// stage2PhasedConcurrent runs full SPRT-aware scoring of stage2 pairs with
// per-rep X-then-Y phase ordering and concurrent fanout across pairs within
// each phase. Returns a slice indexed parallel to the input nonIdentical
// pairs slice; entries for non-stage2 indices are nil.
func stage2PhasedConcurrent(ctx context.Context, r *Runner, in PairwiseInput, preps []preprocess.Result, allPairs []pair, indices []int) (map[int]*pairResult, error) {
	doBoth := in.Cfg.BiasMitigation == "both"
	maxReps := in.Cfg.NReps
	if in.Cfg.UseSPRT && in.Cfg.SPRTParams.MaxReps > maxReps {
		maxReps = in.Cfg.SPRTParams.MaxReps
	}
	minReps := in.Cfg.NReps
	if in.Cfg.UseSPRT && in.Cfg.SPRTParams.MinReps > 0 && in.Cfg.SPRTParams.MinReps < minReps {
		minReps = in.Cfg.SPRTParams.MinReps
	}

	states := map[int]*pairState{}
	for _, idx := range indices {
		states[idx] = &pairState{p: allPairs[idx]}
	}

	// Adaptive inconsistency policy: order-flagged pairs keep collecting
	// rep-pairs (both orders) until the X/Y means agree on a winner or the
	// rep ceiling is reached. flag-only leaves them at NReps and lets the
	// confidence penalty in finalizeSelection surface the disagreement.
	adaptive := doBoth && in.Cfg.InconsistencyPolicy == "adaptive"
	capReps := maxReps
	if adaptive && in.Cfg.SPRTParams.MaxReps > capReps {
		capReps = in.Cfg.SPRTParams.MaxReps
	}

	for rep := 0; rep < capReps; rep++ {
		if err := dispatchPhase(ctx, r, in, preps, states, false, rep); err != nil {
			return nil, err
		}
		if doBoth {
			if err := dispatchPhase(ctx, r, in, preps, states, true, rep); err != nil {
				return nil, err
			}
		}
		if rep+1 < minReps {
			continue
		}
		allDecided := true
		for _, st := range states {
			if st.decided {
				continue
			}
			needMore := false
			if in.Cfg.UseSPRT {
				d, _ := sprt.Decide(st.allDiffs(), in.Cfg.SPRTParams)
				needMore = d == sprt.Continue
			} else {
				needMore = rep+1 < in.Cfg.NReps
			}
			if !needMore && adaptive && rep+1 < capReps && st.currentlyInconsistent() {
				needMore = true
			}
			if needMore {
				allDecided = false
			} else {
				st.decided = true
			}
		}
		if allDecided {
			break
		}
	}

	out := map[int]*pairResult{}
	for idx, st := range states {
		res := st.toResult(doBoth)
		out[idx] = &res
	}
	return out, nil
}

func dispatchPhase(ctx context.Context, r *Runner, in PairwiseInput, preps []preprocess.Result, states map[int]*pairState, isYPhase bool, rep int) error {
	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, in.Cfg.MaxWorkers)
	// Sorted dispatch keeps the documented cache-optimal (i, j) prefix order;
	// map iteration would randomize it per run.
	orderedIdx := make([]int, 0, len(states))
	for idx := range states {
		orderedIdx = append(orderedIdx, idx)
	}
	sort.Ints(orderedIdx)
	for _, idx := range orderedIdx {
		st := states[idx]
		if st.decided {
			continue
		}
		idx, st := idx, st
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			p := st.p
			var traceL, traceR string
			if isYPhase {
				traceL = preps[p.j].Text
				traceR = preps[p.i].Text
			} else {
				traceL = preps[p.i].Text
				traceR = preps[p.j].Text
			}
			leftPrep, rightPrep := preps[p.i], preps[p.j]
			if isYPhase {
				leftPrep, rightPrep = rightPrep, leftPrep
			}
			pctx := ContextWithAuditMeta(gctx, auditMetaFor(leftPrep, rightPrep))
			scores, used, err := r.ScorePairEvidence(pctx, in.Task, traceL, traceR, in.Criteria, rep)
			if err != nil {
				return fmt.Errorf("phase pair (%d,%d): %w", p.i, p.j, err)
			}
			rs := repScoreFromEvidence(scores, isYPhase, rep)
			st.mu.Lock()
			if isYPhase {
				st.yReps = append(st.yReps, rs)
			} else {
				st.xReps = append(st.xReps, rs)
			}
			AddUsage(&st.usage, used)
			st.mu.Unlock()
			_ = idx
			return nil
		})
	}
	return g.Wait()
}

func repScoreFromEvidence(scores PairEvidenceScores, isYPhase bool, repeat int) repScore {
	if len(scores) == 0 {
		return repScore{}
	}
	observation := observationFromEvidence(scores, isYPhase, repeat)
	return repScore{i: observation.si, j: observation.sj, observation: observation}
}

func (s *pairState) allDiffs() []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]float64, 0, len(s.xReps)+len(s.yReps))
	for _, x := range s.xReps {
		out = append(out, x.i-x.j)
	}
	for _, y := range s.yReps {
		out = append(out, y.i-y.j)
	}
	return out
}

// currentlyInconsistent reports whether the X-order and Y-order rep means
// disagree on the winner direction right now.
func (s *pairState) currentlyInconsistent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.xReps) == 0 || len(s.yReps) == 0 {
		return false
	}
	var xMI, xMJ, yMI, yMJ float64
	for _, x := range s.xReps {
		xMI += x.i
		xMJ += x.j
	}
	for _, y := range s.yReps {
		yMI += y.i
		yMJ += y.j
	}
	sX := signOf(xMI/float64(len(s.xReps)) - xMJ/float64(len(s.xReps)))
	sY := signOf(yMI/float64(len(s.yReps)) - yMJ/float64(len(s.yReps)))
	return sX != 0 && sY != 0 && sX != sY
}

func (s *pairState) toResult(doBoth bool) pairResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := pairResult{p: s.p, usage: s.usage}
	cnt := 0
	var sumI, sumJ float64
	for _, x := range s.xReps {
		sumI += x.i
		sumJ += x.j
		cnt++
		res.observations = append(res.observations, x.observation)
	}
	for _, y := range s.yReps {
		sumI += y.i
		sumJ += y.j
		cnt++
		res.observations = append(res.observations, y.observation)
	}
	if cnt == 0 {
		res.si, res.sj = 0.5, 0.5
		return res
	}
	res.si = sumI / float64(cnt)
	res.sj = sumJ / float64(cnt)
	if doBoth && len(s.xReps) > 0 && len(s.yReps) > 0 {
		var xMI, xMJ, yMI, yMJ float64
		for _, x := range s.xReps {
			xMI += x.i
			xMJ += x.j
		}
		for _, y := range s.yReps {
			yMI += y.i
			yMJ += y.j
		}
		xMI /= float64(len(s.xReps))
		xMJ /= float64(len(s.xReps))
		yMI /= float64(len(s.yReps))
		yMJ /= float64(len(s.yReps))
		sX := signOf(xMI - xMJ)
		sY := signOf(yMI - yMJ)
		if sX != 0 && sY != 0 && sX != sY {
			res.inconsistent = true
		}
	}
	return res
}

func generatePairs(n int, singleElim bool) []pair {
	if !singleElim || n <= 4 {
		var out []pair
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				out = append(out, pair{i, j})
			}
		}
		return out
	}
	seeds := make([]int, n)
	for i := range seeds {
		seeds[i] = i
	}
	round := seeds
	var out []pair
	for len(round) > 1 {
		var next []int
		for k := 0; k+1 < len(round); k += 2 {
			out = append(out, pair{round[k], round[k+1]})
			next = append(next, round[k])
		}
		if len(round)%2 == 1 {
			next = append(next, round[len(round)-1])
		}
		round = next
	}
	return out
}

func applyPair(p pair, si, sj, epsilon float64, wins, scoreSum []float64, scoreCount []int) {
	switch {
	case si > sj+epsilon:
		wins[p.i] += 1
	case sj > si+epsilon:
		wins[p.j] += 1
	default:
		wins[p.i] += 0.5
		wins[p.j] += 0.5
	}
	scoreSum[p.i] += si
	scoreSum[p.j] += sj
	scoreCount[p.i]++
	scoreCount[p.j]++
}

func finalizeSelection(sel Selection, n, totalPairs int, wins, scoreSum []float64, scoreCount []int) Selection {
	sel.SchemaVersion = selectionSchemaVersion
	sel.Wins = wins
	sel.Scores = make([]float64, n)
	for i := range scoreSum {
		if scoreCount[i] > 0 {
			sel.Scores[i] = scoreSum[i] / float64(scoreCount[i])
		} else {
			sel.Scores[i] = 0.5
		}
	}
	bestIdx := 0
	for i := 1; i < n; i++ {
		if wins[i] > wins[bestIdx] {
			bestIdx = i
		}
	}
	secondBest := 0.0
	for i, w := range wins {
		if i != bestIdx && w > secondBest {
			secondBest = w
		}
	}
	sel.BestIndex = bestIdx
	sel.State = verifier.DecisionSelected
	confidence := 0.0
	if totalPairs > 0 {
		confidence = (wins[bestIdx] - secondBest) / float64(totalPairs)
	}
	if len(sel.InconsistentPairs) > 0 && totalPairs > 0 {
		ratio := float64(len(sel.InconsistentPairs)) / float64(totalPairs)
		confidence *= 1 - 0.5*ratio
	}
	sel.Confidence = clamp01(confidence)
	allEvidence := []verifier.ScoreEvidence{}
	selectedDecisions := 0
	tiedDecisions := 0
	for _, decision := range sel.PairDecisions {
		for _, observation := range decision.Observations {
			for _, criterion := range observation.Criteria {
				allEvidence = append(allEvidence, criterion.A, criterion.B)
			}
		}
		switch decision.State {
		case verifier.DecisionSelected:
			selectedDecisions++
		case verifier.DecisionTied:
			tiedDecisions++
		case verifier.DecisionAbstained:
			sel.State = verifier.DecisionAbstained
			if sel.AbstentionReason == verifier.AbstentionNone {
				sel.AbstentionReason = decision.AbstentionReason
			}
		}
	}
	if sel.State == verifier.DecisionSelected && (topWinCount(wins) > 1 || selectedDecisions == 0 && tiedDecisions > 0) {
		sel.State = verifier.DecisionTied
	}
	if sel.State != verifier.DecisionSelected {
		sel.BestIndex = -1
		sel.Confidence = 0
	}
	sel.EvidenceStrength = verifier.SummarizeEvidenceStrength(allEvidence)
	sel.Usage.finalizeExtraction()
	return sel
}

func topWinCount(wins []float64) int {
	if len(wins) == 0 {
		return 0
	}
	best := wins[0]
	count := 1
	for _, value := range wins[1:] {
		switch {
		case value > best:
			best = value
			count = 1
		case value == best:
			count++
		}
	}
	return count
}
