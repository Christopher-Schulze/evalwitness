package mode

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/sprt"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type AbsoluteConfig struct {
	NReps      int
	UseSPRT    bool
	SPRTParams sprt.Params
}

type AbsoluteInput struct {
	Task       string
	Trajectory string
	Criteria   []verifier.Criterion
	Cfg        AbsoluteConfig
	// PreparedTrajectory reuses preprocessing the caller already did. Benchmarks
	// slice and redact every trajectory once up front, and re-running that per
	// call is pure waste.
	PreparedTrajectory *preprocess.Result
}

type Score struct {
	SchemaVersion      string                              `json:"schema_version"`
	State              verifier.DecisionState              `json:"state"`
	Value              float64                             `json:"conditional_score"`
	Confidence         float64                             `json:"-"`
	PerCriterion       map[string]float64                  `json:"per_criterion_conditional_scores"`
	CriterionEvidence  map[string][]verifier.ScoreEvidence `json:"criterion_evidence"`
	EvidenceStrength   verifier.EvidenceStrength           `json:"evidence_strength"`
	TrajectoryEvidence []preprocess.AccountingSummary      `json:"trajectory_evidence"`
	Usage              UsageSummary                        `json:"usage"`
}

type AbsoluteTrajectoryEvidence struct {
	TrajectoryIndex   int                                 `json:"trajectory_index"`
	ConditionalScore  float64                             `json:"conditional_score"`
	CriterionEvidence map[string][]verifier.ScoreEvidence `json:"criterion_evidence"`
	EvidenceStrength  verifier.EvidenceStrength           `json:"evidence_strength"`
}

func RunAbsolute(ctx context.Context, r *Runner, in AbsoluteInput) (Score, error) {
	if in.Cfg.NReps <= 0 {
		in.Cfg.NReps = 2
	}
	if (in.Cfg.SPRTParams == sprt.Params{}) {
		in.Cfg.SPRTParams = sprt.DefaultParams()
	}

	var prep preprocess.Result
	if in.PreparedTrajectory != nil {
		prep = *in.PreparedTrajectory
	} else {
		var err error
		prep, err = r.PrepareTrajectory(in.Trajectory)
		if err != nil {
			return Score{}, fmt.Errorf("prepare trajectory: %w", err)
		}
	}
	ctx = ContextWithAuditMeta(ctx, auditMetaFor(prep))
	out := Score{
		SchemaVersion:      "evalwitness.absolute-score.v2",
		State:              verifier.DecisionSelected,
		PerCriterion:       map[string]float64{},
		CriterionEvidence:  map[string][]verifier.ScoreEvidence{},
		TrajectoryEvidence: preprocess.AccountingSummaries(prep),
	}
	allEvidence := []verifier.ScoreEvidence{}

	maxReps := in.Cfg.NReps
	if in.Cfg.UseSPRT && in.Cfg.SPRTParams.MaxReps > maxReps {
		maxReps = in.Cfg.SPRTParams.MaxReps
	}
	minReps := in.Cfg.NReps
	if in.Cfg.UseSPRT && in.Cfg.SPRTParams.MinReps > 0 && in.Cfg.SPRTParams.MinReps < minReps {
		minReps = in.Cfg.SPRTParams.MinReps
	}

	perCritScores := map[string][]float64{}
	for _, c := range in.Criteria {
		perCritScores[c.ID] = nil
	}

	for rep := 0; rep < maxReps; rep++ {
		scores, used, err := r.ScoreSingleEvidence(ctx, in.Task, prep.Text, in.Criteria, rep)
		if err != nil {
			return out, fmt.Errorf("rep %d: %w", rep, err)
		}
		AddUsage(&out.Usage, used)
		for cid, s := range scores {
			value := evidenceScore(s)
			perCritScores[cid] = append(perCritScores[cid], value)
			out.CriterionEvidence[cid] = append(out.CriterionEvidence[cid], s)
			allEvidence = append(allEvidence, s)
		}

		if in.Cfg.UseSPRT && rep+1 >= minReps {
			allCurrent := []float64{}
			for _, scs := range perCritScores {
				allCurrent = append(allCurrent, scs...)
			}
			if sprt.AbsoluteVarianceDecision(allCurrent, in.Cfg.SPRTParams) {
				break
			}
		} else if !in.Cfg.UseSPRT && rep+1 >= in.Cfg.NReps {
			break
		}
	}

	cnt := 0
	sumValue := 0.0
	for cid, scs := range perCritScores {
		critMean := mean(scs)
		out.PerCriterion[cid] = critMean
		sumValue += critMean
		cnt++
	}
	if cnt > 0 {
		out.Value = sumValue / float64(cnt)
	}
	out.EvidenceStrength = verifier.SummarizeEvidenceStrength(allEvidence)
	out.Usage.finalizeExtraction()
	return out, nil
}

// AbsoluteSelectionInput drives selection by scoring each trajectory once
// instead of comparing them in pairs.
type AbsoluteSelectionInput struct {
	Task                 string
	Trajectories         []string
	PreparedTrajectories []preprocess.Result
	Criteria             []verifier.Criterion
	Cfg                  PairwiseConfig
}

// RunAbsoluteSelection scores every trajectory once and picks the highest.
//
// It is the cheapest selection strategy by input volume: a pairwise call carries
// two trajectories, an absolute call carries one, so n calls of half the size
// replace n-1 calls of full size. Whether that costs accuracy is a measurement,
// not an assumption, which is why it is opt-in rather than the default.
func RunAbsoluteSelection(ctx context.Context, r *Runner, in AbsoluteSelectionInput) (Selection, error) {
	n := len(in.Trajectories)
	if n < 2 {
		return Selection{}, errors.New("absolute selection requires at least 2 trajectories")
	}
	setPairwiseDefaults(&in.Cfg)

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

	scores := make([]float64, n)
	absoluteEvidence := make([]AbsoluteTrajectoryEvidence, n)
	usages := make([]UsageSummary, n)
	var mu sync.Mutex

	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	label := func(i int) string { return fmt.Sprintf("absolute trajectory %d", i) }
	evaluate := func(runCtx context.Context, i int) error {
		prep := preps[i]
		score, err := RunAbsolute(runCtx, r, AbsoluteInput{
			Task:               in.Task,
			Trajectory:         in.Trajectories[i],
			PreparedTrajectory: &prep,
			Criteria:           in.Criteria,
			Cfg:                AbsoluteConfig{NReps: 1},
		})
		if err != nil {
			return err
		}
		mu.Lock()
		scores[i] = score.Value
		absoluteEvidence[i] = AbsoluteTrajectoryEvidence{
			TrajectoryIndex:   i,
			ConditionalScore:  score.Value,
			CriterionEvidence: score.CriterionEvidence,
			EvidenceStrength:  score.EvidenceStrength,
		}
		usages[i] = score.Usage
		mu.Unlock()
		return nil
	}

	failed, fatal := runPairSweep(ctx, in.Cfg.MaxWorkers, indices, label, evaluate)
	if fatal != nil {
		return Selection{}, fatal
	}
	failed, fatal = recoverFailedPairs(ctx, in.Cfg.MaxWorkers, failed, n, label, evaluate)
	if fatal != nil {
		return Selection{}, fatal
	}
	if err := reportPairFailures(failed, n, n-len(failed)); err != nil {
		return Selection{}, err
	}

	sel := Selection{SchemaVersion: selectionSchemaVersion, State: verifier.DecisionSelected, Scores: scores, Wins: make([]float64, n), AbsoluteEvidence: absoluteEvidence, TrajectoryEvidence: preprocess.AccountingSummaries(preps...)}
	for _, u := range usages {
		AddUsage(&sel.Usage, u)
	}
	best := 0
	for i := 1; i < n; i++ {
		if scores[i] > scores[best] {
			best = i
		}
	}
	runnerUp := 0.0
	for i, v := range scores {
		if i != best && v > runnerUp {
			runnerUp = v
		}
	}
	margin := scores[best] - runnerUp
	if !selectionMarginExceedsEpsilon(margin, in.Cfg.Epsilon) {
		sel.State = verifier.DecisionTied
		sel.BestIndex = -1
	} else {
		sel.BestIndex = best
		sel.Wins[best] = 1
		sel.Confidence = clamp01(margin)
	}
	allEvidence := []verifier.ScoreEvidence{}
	for _, trajectory := range absoluteEvidence {
		for _, criterion := range trajectory.CriterionEvidence {
			allEvidence = append(allEvidence, criterion...)
		}
	}
	sel.EvidenceStrength = verifier.SummarizeEvidenceStrength(allEvidence)
	sel.Usage.finalizeExtraction()
	return sel, nil
}

// RunJointAbsoluteSelection scores the complete, ordered candidate set in one
// response per repetition. It is intended for fixed-graph studies where both
// analysis arms must consume exactly the same response bytes.
func RunJointAbsoluteSelection(ctx context.Context, r *Runner, in AbsoluteSelectionInput) (Selection, error) {
	n := len(in.Trajectories)
	if n < 2 {
		return Selection{}, errors.New("joint-absolute selection requires at least 2 trajectories")
	}
	setPairwiseDefaults(&in.Cfg)
	if in.Cfg.UseSPRT {
		return Selection{}, errors.New("joint-absolute selection requires fixed repetitions")
	}

	preps := in.PreparedTrajectories
	if len(preps) > 0 && len(preps) != n {
		return Selection{}, fmt.Errorf("prepared trajectory count %d does not match trajectory count %d", len(preps), n)
	}
	if len(preps) == 0 {
		preps = make([]preprocess.Result, n)
		for index, trajectory := range in.Trajectories {
			var err error
			preps[index], err = r.PrepareTrajectory(trajectory)
			if err != nil {
				return Selection{}, fmt.Errorf("prepare trajectory %d: %w", index, err)
			}
		}
	}
	ctx = ContextWithAuditMeta(ctx, auditMetaFor(preps...))

	evidence := make([]AbsoluteTrajectoryEvidence, n)
	criterionScores := make([]map[string][]float64, n)
	for index := range evidence {
		evidence[index] = AbsoluteTrajectoryEvidence{
			TrajectoryIndex: index, CriterionEvidence: map[string][]verifier.ScoreEvidence{},
		}
		criterionScores[index] = map[string][]float64{}
	}
	selection := Selection{
		SchemaVersion: selectionSchemaVersion, State: verifier.DecisionSelected,
		Scores: make([]float64, n), Wins: make([]float64, n),
		AbsoluteEvidence: evidence, TrajectoryEvidence: preprocess.AccountingSummaries(preps...),
	}
	for rep := 0; rep < in.Cfg.NReps; rep++ {
		repEvidence, usage, err := r.ScoreJointAbsoluteEvidence(ctx, in.Task, in.Trajectories, in.Criteria, rep)
		if err != nil {
			return selection, fmt.Errorf("joint-absolute rep %d: %w", rep, err)
		}
		AddUsage(&selection.Usage, usage)
		for candidateIndex, candidateEvidence := range repEvidence {
			for criterionID, scoreEvidence := range candidateEvidence {
				criterionScores[candidateIndex][criterionID] = append(criterionScores[candidateIndex][criterionID], evidenceScore(scoreEvidence))
				selection.AbsoluteEvidence[candidateIndex].CriterionEvidence[criterionID] = append(selection.AbsoluteEvidence[candidateIndex].CriterionEvidence[criterionID], scoreEvidence)
			}
		}
	}

	allEvidence := make([]verifier.ScoreEvidence, 0, n*len(in.Criteria)*in.Cfg.NReps)
	for candidateIndex := range selection.Scores {
		criterionTotal := 0.0
		for _, scores := range criterionScores[candidateIndex] {
			criterionTotal += mean(scores)
		}
		if len(criterionScores[candidateIndex]) > 0 {
			selection.Scores[candidateIndex] = criterionTotal / float64(len(criterionScores[candidateIndex]))
		}
		selection.AbsoluteEvidence[candidateIndex].ConditionalScore = selection.Scores[candidateIndex]
		for _, criterionEvidence := range selection.AbsoluteEvidence[candidateIndex].CriterionEvidence {
			allEvidence = append(allEvidence, criterionEvidence...)
		}
	}
	best := 0
	runnerUp := math.Inf(-1)
	for index := 1; index < n; index++ {
		if selection.Scores[index] > selection.Scores[best] {
			best = index
		}
	}
	for index, score := range selection.Scores {
		if index != best && score > runnerUp {
			runnerUp = score
		}
	}
	margin := selection.Scores[best] - runnerUp
	if !selectionMarginExceedsEpsilon(margin, in.Cfg.Epsilon) {
		selection.State = verifier.DecisionTied
		selection.BestIndex = -1
	} else {
		selection.BestIndex = best
		selection.Wins[best] = 1
		selection.Confidence = clamp01(margin)
	}
	selection.EvidenceStrength = verifier.SummarizeEvidenceStrength(allEvidence)
	selection.Usage.finalizeExtraction()
	return selection, nil
}

func selectionMarginExceedsEpsilon(margin, epsilon float64) bool {
	return margin > epsilon+1e-12
}
