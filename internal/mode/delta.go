package mode

import (
	"context"
	"fmt"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/sprt"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type DeltaConfig struct {
	NReps                int
	Epsilon              float64
	BiasMitigation       string
	UseSPRT              bool
	SPRTParams           sprt.Params
	MaxPairCalls         int
	ConfidenceThreshold  float64
	CalibrationSigma     float64
	ConfidenceEscalation string
}

type DeltaInput struct {
	Task                 string
	TrajectoryA          string
	TrajectoryB          string
	PreparedTrajectories []preprocess.Result
	Criteria             []verifier.Criterion
	Cfg                  DeltaConfig
}

type Verdict struct {
	SchemaVersion      string                         `json:"schema_version"`
	State              verifier.DecisionState         `json:"state"`
	AbstentionReason   verifier.AbstentionReason      `json:"abstention_reason,omitempty"`
	Winner             string                         `json:"winner"`
	Margin             float64                        `json:"conditional_margin"`
	ScoreA             float64                        `json:"conditional_score_a"`
	ScoreB             float64                        `json:"conditional_score_b"`
	PerCriterion       map[string]CriterionScores     `json:"per_criterion_conditional_scores"`
	Inconsistent       bool                           `json:"inconsistent"`
	Decision           *PairDecision                  `json:"decision,omitempty"`
	EvidenceStrength   verifier.EvidenceStrength      `json:"evidence_strength"`
	Observations       []PairObservationRecord        `json:"observations"`
	TrajectoryEvidence []preprocess.AccountingSummary `json:"trajectory_evidence"`
	Usage              UsageSummary                   `json:"usage"`
}

type CriterionScores struct {
	A float64 `json:"conditional_a"`
	B float64 `json:"conditional_b"`
}

type deltaCritAccum struct {
	aSumX, bSumX float64
	aSumY, bSumY float64
	nX, nY       int
}

func RunDelta(ctx context.Context, r *Runner, in DeltaInput) (Verdict, error) {
	if in.Cfg.NReps <= 0 {
		in.Cfg.NReps = 1
	}
	if in.Cfg.Epsilon == 0 {
		in.Cfg.Epsilon = 0.02
	}
	if in.Cfg.BiasMitigation == "" {
		in.Cfg.BiasMitigation = "adaptive"
	}
	if in.Cfg.MaxPairCalls <= 0 {
		in.Cfg.MaxPairCalls = 4
	}
	if in.Cfg.ConfidenceThreshold == 0 {
		in.Cfg.ConfidenceThreshold = 0.8
	}
	if in.Cfg.CalibrationSigma == 0 {
		in.Cfg.CalibrationSigma = 0.05
	}
	if in.Cfg.BiasMitigation == "adaptive" && in.Cfg.NReps != 1 {
		return Verdict{}, fmt.Errorf("adaptive bias mitigation requires n_reps=1; use both or single for an explicit fixed multi-rep run")
	}
	warnBiasNotBoth("delta", in.Cfg.BiasMitigation)
	if (in.Cfg.SPRTParams == sprt.Params{}) {
		in.Cfg.SPRTParams = sprt.DefaultParams()
	}

	var aPrep, bPrep preprocess.Result
	if len(in.PreparedTrajectories) > 0 {
		if len(in.PreparedTrajectories) != 2 {
			return Verdict{}, fmt.Errorf("prepared trajectory count %d does not match delta trajectory count 2", len(in.PreparedTrajectories))
		}
		aPrep, bPrep = in.PreparedTrajectories[0], in.PreparedTrajectories[1]
	} else {
		var err error
		aPrep, err = r.PrepareTrajectory(in.TrajectoryA)
		if err != nil {
			return Verdict{}, fmt.Errorf("prepare trajectory A: %w", err)
		}
		bPrep, err = r.PrepareTrajectory(in.TrajectoryB)
		if err != nil {
			return Verdict{}, fmt.Errorf("prepare trajectory B: %w", err)
		}
	}
	forwardCtx := ContextWithAuditMeta(ctx, auditMetaFor(aPrep, bPrep))
	reverseCtx := ContextWithAuditMeta(ctx, auditMetaFor(bPrep, aPrep))
	if in.Cfg.BiasMitigation == "adaptive" {
		verdict, err := runAdaptiveDelta(ctx, r, in, aPrep, bPrep)
		verdict.TrajectoryEvidence = preprocess.AccountingSummaries(aPrep, bPrep)
		return verdict, err
	}

	doBoth := in.Cfg.BiasMitigation == "both"
	v := Verdict{SchemaVersion: "evalwitness.delta.v2", PerCriterion: map[string]CriterionScores{}, TrajectoryEvidence: preprocess.AccountingSummaries(aPrep, bPrep)}
	observations := []pairObservation{}

	perCrit := map[string]*deltaCritAccum{}
	for _, c := range in.Criteria {
		perCrit[c.ID] = &deltaCritAccum{}
	}

	maxReps := in.Cfg.NReps
	if in.Cfg.UseSPRT && in.Cfg.SPRTParams.MaxReps > maxReps {
		maxReps = in.Cfg.SPRTParams.MaxReps
	}
	minReps := in.Cfg.NReps
	if in.Cfg.UseSPRT && in.Cfg.SPRTParams.MinReps > 0 && in.Cfg.SPRTParams.MinReps < minReps {
		minReps = in.Cfg.SPRTParams.MinReps
	}

	var diffs []float64
	for rep := 0; rep < maxReps; rep++ {
		scoresX, usedX, err := r.ScorePairEvidence(forwardCtx, in.Task, aPrep.Text, bPrep.Text, in.Criteria, rep)
		if err != nil {
			return v, fmt.Errorf("rep %d order X: %w", rep, err)
		}
		AddUsage(&v.Usage, usedX)
		var repAX, repBX float64
		nCrit := 0
		observations = append(observations, observationFromEvidence(scoresX, false, rep))
		for cid, sc := range scoresX {
			a := perCrit[cid]
			a.aSumX += evidenceScore(sc.A)
			a.bSumX += evidenceScore(sc.B)
			a.nX++
			repAX += evidenceScore(sc.A)
			repBX += evidenceScore(sc.B)
			nCrit++
		}
		if nCrit > 0 {
			diffs = append(diffs, (repAX-repBX)/float64(nCrit))
		}
		if doBoth {
			scoresY, usedY, err := r.ScorePairEvidence(reverseCtx, in.Task, bPrep.Text, aPrep.Text, in.Criteria, rep)
			if err != nil {
				return v, fmt.Errorf("rep %d order Y: %w", rep, err)
			}
			AddUsage(&v.Usage, usedY)
			var repAY, repBY float64
			nCritY := 0
			observations = append(observations, observationFromEvidence(scoresY, true, rep))
			for cid, sc := range scoresY {
				a := perCrit[cid]
				a.aSumY += evidenceScore(sc.B)
				a.bSumY += evidenceScore(sc.A)
				a.nY++
				repAY += evidenceScore(sc.B)
				repBY += evidenceScore(sc.A)
				nCritY++
			}
			if nCritY > 0 {
				diffs = append(diffs, (repAY-repBY)/float64(nCritY))
			}
		}

		if in.Cfg.UseSPRT && rep+1 >= minReps {
			if d, _ := sprt.Decide(diffs, in.Cfg.SPRTParams); d != sprt.Continue {
				break
			}
		} else if !in.Cfg.UseSPRT && rep+1 >= in.Cfg.NReps {
			break
		}
	}

	var aTotal, bTotal float64
	cnt := 0
	anyDisagree := false
	for cid, a := range perCrit {
		var critA, critB float64
		if a.nX > 0 {
			critA = a.aSumX / float64(a.nX)
			critB = a.bSumX / float64(a.nX)
		}
		if doBoth && a.nY > 0 {
			critA = (critA + a.aSumY/float64(a.nY)) / 2
			critB = (critB + a.bSumY/float64(a.nY)) / 2
			if a.nX > 0 {
				sX := signOf(a.aSumX/float64(a.nX) - a.bSumX/float64(a.nX))
				sY := signOf(a.aSumY/float64(a.nY) - a.bSumY/float64(a.nY))
				if sX != 0 && sY != 0 && sX != sY {
					anyDisagree = true
				}
			}
		}
		v.PerCriterion[cid] = CriterionScores{A: critA, B: critB}
		aTotal += critA
		bTotal += critB
		cnt++
	}

	if cnt > 0 {
		v.ScoreA = aTotal / float64(cnt)
		v.ScoreB = bTotal / float64(cnt)
	}
	diff := v.ScoreA - v.ScoreB
	switch {
	case diff > in.Cfg.Epsilon:
		v.Winner = "A"
		v.Margin = diff
	case diff < -in.Cfg.Epsilon:
		v.Winner = "B"
		v.Margin = -diff
	default:
		v.Winner = "tie"
		v.Margin = absF(diff)
	}
	v.Inconsistent = anyDisagree && doBoth
	aggregate := aggregatePairObservations(observations, in.Cfg.CalibrationSigma)
	decision := PairDecision{
		SchemaVersion:    pairDecisionSchemaVersion,
		PolicyVersion:    verifier.StrictPolicyVersion,
		Pair:             [2]int{0, 1},
		OrderPolicy:      in.Cfg.BiasMitigation,
		FirstOrder:       "forward",
		Calls:            v.Usage.Calls,
		RepeatCount:      countPairRepeats(observations),
		MeanDifference:   aggregate.difference,
		Variance:         aggregate.variance,
		ScoreMass:        aggregate.scoreMass,
		WinProbability:   aggregate.winProbability,
		Confidence:       aggregate.confidence,
		Winner:           pairWinner(pair{i: 0, j: 1}, aggregate.difference, in.Cfg.Epsilon, in.Task),
		Inconsistent:     aggregate.inconsistent,
		Uncertainty:      aggregate.uncertainty,
		EvidenceStrength: verifier.SummarizeEvidenceStrength(pairEvidenceItems(observations)),
		Observations:     pairObservationRecords(observations),
	}
	setPairDecisionState(&decision, aggregate, PairwiseConfig{Epsilon: in.Cfg.Epsilon, ConfidenceThreshold: in.Cfg.ConfidenceThreshold})
	v.Decision = &decision
	v.State = decision.State
	v.AbstentionReason = decision.AbstentionReason
	if decision.State == verifier.DecisionAbstained {
		v.Winner = "none"
	}
	v.EvidenceStrength = decision.EvidenceStrength
	v.Observations = decision.Observations
	v.Usage.finalizeExtraction()
	return v, nil
}

func runAdaptiveDelta(ctx context.Context, r *Runner, in DeltaInput, aPrep, bPrep preprocess.Result) (Verdict, error) {
	perOrderCalls := pairOrderCallCount(r, in.Criteria)
	if perOrderCalls > in.Cfg.MaxPairCalls {
		return Verdict{}, fmt.Errorf(
			"adaptive max pair calls %d cannot fit one order requiring %d provider calls; enable criterion bundling or raise the limit",
			in.Cfg.MaxPairCalls,
			perOrderCalls,
		)
	}
	p := pair{i: 0, j: 1}
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

	type criterionAccum struct {
		aSum float64
		bSum float64
		n    int
	}
	perCriterion := make(map[string]*criterionAccum, len(in.Criteria))
	for _, criterion := range in.Criteria {
		perCriterion[criterion.ID] = &criterionAccum{}
	}

	verdict := Verdict{SchemaVersion: "evalwitness.delta.v2", PerCriterion: make(map[string]CriterionScores, len(in.Criteria))}
	observations := make([]pairObservation, 0, len(orders))
	for _, order := range orders {
		if len(observations) > 0 && verdict.Usage.Calls+perOrderCalls > in.Cfg.MaxPairCalls {
			break
		}
		left, right := aPrep.Text, bPrep.Text
		orderCtx := ContextWithAuditMeta(ctx, auditMetaFor(aPrep, bPrep))
		if order.reversed {
			left, right = right, left
			orderCtx = ContextWithAuditMeta(ctx, auditMetaFor(bPrep, aPrep))
		}
		scores, usage, err := r.ScorePairEvidence(orderCtx, in.Task, left, right, in.Criteria, order.rep)
		if err != nil {
			return verdict, fmt.Errorf("adaptive delta rep %d reversed=%t: %w", order.rep, order.reversed, err)
		}
		AddUsage(&verdict.Usage, usage)
		observations = append(observations, observationFromEvidence(scores, order.reversed, order.rep))
		for criterionID, score := range scores {
			aScore, bScore := evidenceScore(score.A), evidenceScore(score.B)
			if order.reversed {
				aScore, bScore = bScore, aScore
			}
			accumulator := perCriterion[criterionID]
			accumulator.aSum += aScore
			accumulator.bSum += bScore
			accumulator.n++
		}
		aggregate := aggregatePairObservations(observations, in.Cfg.CalibrationSigma)
		if shouldStopAdaptivePair(aggregate, PairwiseConfig{
			Epsilon:              in.Cfg.Epsilon,
			ConfidenceThreshold:  in.Cfg.ConfidenceThreshold,
			ConfidenceEscalation: in.Cfg.ConfidenceEscalation,
		}) {
			break
		}
	}

	aggregate := aggregatePairObservations(observations, in.Cfg.CalibrationSigma)
	for criterionID, accumulator := range perCriterion {
		if accumulator.n == 0 {
			verdict.PerCriterion[criterionID] = CriterionScores{A: 0.5, B: 0.5}
			continue
		}
		denominator := float64(accumulator.n)
		verdict.PerCriterion[criterionID] = CriterionScores{
			A: accumulator.aSum / denominator,
			B: accumulator.bSum / denominator,
		}
	}
	verdict.ScoreA = aggregate.si
	verdict.ScoreB = aggregate.sj
	verdict.Inconsistent = aggregate.inconsistent
	difference := aggregate.difference
	switch {
	case difference > in.Cfg.Epsilon:
		verdict.Winner = "A"
		verdict.Margin = difference
	case difference < -in.Cfg.Epsilon:
		verdict.Winner = "B"
		verdict.Margin = -difference
	default:
		verdict.Winner = "tie"
		verdict.Margin = absF(difference)
	}
	decision := PairDecision{
		SchemaVersion:    pairDecisionSchemaVersion,
		PolicyVersion:    verifier.StrictPolicyVersion,
		Pair:             [2]int{0, 1},
		OrderPolicy:      in.Cfg.BiasMitigation,
		FirstOrder:       pairOrderName(baseReversed),
		Calls:            verdict.Usage.Calls,
		RepeatCount:      countPairRepeats(observations),
		MeanDifference:   difference,
		Variance:         aggregate.variance,
		ScoreMass:        aggregate.scoreMass,
		WinProbability:   aggregate.winProbability,
		Confidence:       aggregate.confidence,
		Winner:           pairWinner(p, difference, in.Cfg.Epsilon, in.Task),
		Inconsistent:     aggregate.inconsistent,
		Uncertainty:      aggregate.uncertainty,
		EvidenceStrength: verifier.SummarizeEvidenceStrength(pairEvidenceItems(observations)),
		Observations:     pairObservationRecords(observations),
	}
	setPairDecisionState(&decision, aggregate, PairwiseConfig{Epsilon: in.Cfg.Epsilon, ConfidenceThreshold: in.Cfg.ConfidenceThreshold})
	verdict.Decision = &decision
	verdict.State = decision.State
	verdict.AbstentionReason = decision.AbstentionReason
	if decision.State == verifier.DecisionAbstained {
		verdict.Winner = "none"
	}
	verdict.EvidenceStrength = decision.EvidenceStrength
	verdict.Observations = decision.Observations
	verdict.Usage.finalizeExtraction()
	return verdict, nil
}
