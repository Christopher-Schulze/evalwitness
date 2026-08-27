// Package reliability measures whether a predicted probability means what it
// says.
//
// The pipeline takes three separate decisions from one number: whether a pair
// is settled after one call, whether to spend a second, and whether to evaluate
// the reverse order. All three read `win_probability`, and until this package
// existed nothing checked that the number tracks correctness. On the shipped
// route the development data do not support using it as a calibrated gate.
// This package measures the signal without claiming that calibration caused
// each downstream result.
//
// Ground truth exists only where a benchmark supplies rewards. Calibration is
// therefore an eval-only output on purpose: a figure computed against the
// verifier's own score would be circular and worse than none.
package reliability

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// binCount fixes the reliability histogram at ten equal-width bins over [0, 1].
// Fixed rather than configurable, because a bin count chosen after seeing the
// curve is a free parameter and ECE is sensitive to it.
const binCount = 10

// lowSampleFloor is the count below which every metric is marked unreliable.
// Not a threshold for hiding numbers, a threshold for labelling them.
const lowSampleFloor = 30

const (
	// SchemaVersion identifies the descriptive reliability record. It is not a
	// fitted calibration policy and must never be consumed as one.
	SchemaVersion = "logprobe.reliability.v1"
	// DevelopmentDataRole prevents already inspected benchmark rows from being
	// relabelled as held-out calibration evidence by a downstream renderer.
	DevelopmentDataRole = "development"
	bootstrapReplicates = 2000
	intervalLevel       = 0.95
)

// Observation pairs a predicted probability with what actually happened.
// Predicted is the probability the first slot of a comparison wins; Won records
// whether it did. Observations where no correct answer exists must be excluded
// by the caller, not passed in with an invented label.
type Observation struct {
	ID             string  `json:"id,omitempty"`
	TaskID         string  `json:"task_id,omitempty"`
	Pair           [2]int  `json:"pair,omitempty"`
	ExtractionMode string  `json:"extraction_mode,omitempty"`
	Predicted      float64 `json:"predicted"`
	Won            bool    `json:"won"`
	MeanDifference float64 `json:"mean_difference"`
	ScoreMass      float64 `json:"score_mass"`
	Calls          int     `json:"calls"`
	PairCallLimit  int     `json:"pair_call_limit"`
	OrderPolicy    string  `json:"order_policy,omitempty"`
	FirstOrder     string  `json:"first_order,omitempty"`
	OutcomeID      string  `json:"outcome_id,omitempty"`
	Inconsistent   bool    `json:"inconsistent,omitempty"`
}

// RankObservation joins an absolute trajectory score to an independently
// supplied reward. It is separate from pairwise probability observations so a
// rank correlation can never be mistaken for probability calibration.
type RankObservation struct {
	ID             string  `json:"id,omitempty"`
	TaskID         string  `json:"task_id,omitempty"`
	Trajectory     int     `json:"trajectory"`
	ExtractionMode string  `json:"extraction_mode,omitempty"`
	Predicted      float64 `json:"predicted"`
	Actual         float64 `json:"actual"`
	OutcomeID      string  `json:"outcome_id,omitempty"`
}

// Interval is a deterministic percentile interval from task-cluster bootstrap
// resampling. EffectiveReplicates can be lower than Replicates for a conditional
// error-share metric when a resample contains no wrong decisions.
type Interval struct {
	Lower               float64 `json:"lower"`
	Upper               float64 `json:"upper"`
	Level               float64 `json:"level"`
	Method              string  `json:"method"`
	Replicates          int     `json:"replicates"`
	EffectiveReplicates int     `json:"effective_replicates"`
	Clusters            int     `json:"clusters"`
}

// Metric binds a point estimate to its actual observation count, sample warning,
// and task-cluster uncertainty. It is the versioned field downstream consumers
// should use instead of treating a legacy naked scalar as calibrated confidence.
type Metric struct {
	Value     float64   `json:"value"`
	Count     int       `json:"count"`
	LowSample bool      `json:"low_sample"`
	Interval  *Interval `json:"interval,omitempty"`
}

// Metrics is the typed evidence for each descriptive pairwise quantity.
type Metrics struct {
	ECE      Metric `json:"ece"`
	MCE      Metric `json:"mce"`
	Brier    Metric `json:"brier"`
	AUC      Metric `json:"auc"`
	Accuracy Metric `json:"accuracy"`
}

// ErrorStratum separates ambiguous small score differences from confidently
// wrong-direction evidence using thresholds fixed before the bootstrap.
type ErrorStratum struct {
	ID            string `json:"id"`
	Range         string `json:"range"`
	Count         int    `json:"count"`
	TaskCount     int    `json:"task_count"`
	ShareOfErrors Metric `json:"share_of_errors"`
}

// ErrorDecomposition describes only directional errors. Exact 0.5 predictions
// are unresolved rather than silently classified as wrong.
type ErrorDecomposition struct {
	TotalWrong      int            `json:"total_wrong"`
	TotalWrongTasks int            `json:"total_wrong_tasks"`
	Strata          []ErrorStratum `json:"strata"`
}

// AbsoluteReport keeps rank discrimination for absolute-mode outputs separate
// from pairwise calibration.
type AbsoluteReport struct {
	Spearman Metric            `json:"spearman"`
	Rows     []RankObservation `json:"source_rows"`
}

// Bin is one row of the reliability diagram.
type Bin struct {
	Lower         float64 `json:"lower"`
	Upper         float64 `json:"upper"`
	Count         int     `json:"count"`
	MeanPredicted float64 `json:"mean_predicted"`
	MeanObserved  float64 `json:"mean_observed"`
}

// Report is the full calibration and discrimination picture for one arm.
type Report struct {
	SchemaVersion  string             `json:"schema_version"`
	DataRole       string             `json:"data_role"`
	ExtractionMode string             `json:"extraction_mode,omitempty"`
	ClusterCount   int                `json:"cluster_count"`
	Metrics        Metrics            `json:"metrics"`
	Errors         ErrorDecomposition `json:"error_decomposition"`
	SourceRows     []Observation      `json:"source_rows,omitempty"`
	Absolute       *AbsoluteReport    `json:"absolute,omitempty"`

	Count int   `json:"count"`
	Bins  []Bin `json:"bins"`

	// ECE weights each bin's gap by its share of observations; MCE reports the
	// worst bin. Both are computed on the raw predicted probability rather than
	// on folded confidence, because folding first hides direction errors, which
	// is the failure this package was written to catch.
	ECE   float64 `json:"ece"`
	MCE   float64 `json:"mce"`
	Brier float64 `json:"brier"`

	// AUC is discrimination rather than calibration: whether the signal ranks
	// correct above incorrect at all, independent of whether its magnitudes
	// mean anything. A perfectly ranked but wildly overconfident signal scores
	// 1.0 here and badly on ECE, and the two are worth telling apart.
	AUC float64 `json:"auc"`

	// Accuracy is the share of observations where the signal pointed the right
	// way, taking 0.5 as no answer and scoring it half.
	Accuracy float64 `json:"accuracy"`

	// Monotone reports whether observed frequency rises with predicted
	// probability across non-empty bins in this sample. Non-monotonicity
	// invalidates the current raw-probability gate but does not prove that a
	// held-out isotonic or parametric calibration policy cannot generalize.
	Monotone bool `json:"monotone"`

	// LowSample marks a report computed from too few observations to carry
	// weight. The numbers are still emitted, because suppressing them would
	// hide the sample size rather than communicate it.
	LowSample bool `json:"low_sample"`
}

// MarshalJSON omits pairwise-only quantities for an absolute-only run. The Go
// fields remain available for source compatibility, while the versioned wire
// schema cannot turn a zero-valued pairwise report into an apparent metric.
func (r Report) MarshalJSON() ([]byte, error) {
	type reportAlias Report
	if r.Count > 0 {
		return json.Marshal(reportAlias(r))
	}
	return json.Marshal(struct {
		SchemaVersion  string          `json:"schema_version"`
		DataRole       string          `json:"data_role"`
		ExtractionMode string          `json:"extraction_mode,omitempty"`
		Absolute       *AbsoluteReport `json:"absolute,omitempty"`
	}{
		SchemaVersion:  r.SchemaVersion,
		DataRole:       r.DataRole,
		ExtractionMode: r.ExtractionMode,
		Absolute:       r.Absolute,
	})
}

// Analyze computes the full report. An empty input yields a zero report rather
// than an error: an arm that produced no decidable observation is a fact about
// the run, not a failure of the analysis.
func Analyze(observations []Observation) Report {
	return AnalyzeWithAbsolute(observations, nil)
}

// AnalyzeWithAbsolute computes pairwise calibration and optional absolute-mode
// rank discrimination under one versioned descriptive schema.
func AnalyzeWithAbsolute(observations []Observation, ranks []RankObservation) Report {
	r := analyzePoint(observations)
	r.SchemaVersion = SchemaVersion
	r.DataRole = DevelopmentDataRole
	r.ExtractionMode = extractionMode(observations, ranks)
	r.ClusterCount = observationClusterCount(observations)
	r.SourceRows = append([]Observation(nil), observations...)
	r.Errors = decomposeErrors(observations)
	applyBootstrap(&r, observations)
	if len(ranks) > 0 {
		r.Absolute = analyzeAbsolute(ranks)
	}
	return r
}

func analyzePoint(observations []Observation) Report {
	r := Report{Count: len(observations), LowSample: len(observations) < lowSampleFloor}
	if len(observations) == 0 {
		r.Metrics = Metrics{
			ECE:      metric(0, 0, nil),
			MCE:      metric(0, 0, nil),
			Brier:    metric(0, 0, nil),
			AUC:      metric(0, 0, nil),
			Accuracy: metric(0, 0, nil),
		}
		return r
	}

	sums := make([]float64, binCount)
	wins := make([]float64, binCount)
	counts := make([]int, binCount)
	brier, correct := 0.0, 0.0

	for _, o := range observations {
		p := clamp01(o.Predicted)
		idx := int(p * binCount)
		if idx >= binCount {
			idx = binCount - 1
		}
		counts[idx]++
		sums[idx] += p
		y := 0.0
		if o.Won {
			y = 1
		}
		wins[idx] += y
		brier += (p - y) * (p - y)
		switch {
		case p == 0.5:
			correct += 0.5
		case (p > 0.5) == o.Won:
			correct++
		}
	}

	r.Brier = brier / float64(len(observations))
	r.Accuracy = correct / float64(len(observations))

	for i := range counts {
		if counts[i] == 0 {
			continue
		}
		n := float64(counts[i])
		bin := Bin{
			Lower:         float64(i) / binCount,
			Upper:         float64(i+1) / binCount,
			Count:         counts[i],
			MeanPredicted: sums[i] / n,
			MeanObserved:  wins[i] / n,
		}
		gap := math.Abs(bin.MeanObserved - bin.MeanPredicted)
		r.ECE += n / float64(len(observations)) * gap
		if gap > r.MCE {
			r.MCE = gap
		}
		r.Bins = append(r.Bins, bin)
	}

	r.AUC = auc(observations)
	r.Monotone = monotone(r.Bins)
	r.Metrics = Metrics{
		ECE:      metric(r.ECE, len(observations), nil),
		MCE:      metric(r.MCE, len(observations), nil),
		Brier:    metric(r.Brier, len(observations), nil),
		AUC:      metric(r.AUC, len(observations), nil),
		Accuracy: metric(r.Accuracy, len(observations), nil),
	}
	return r
}

func metric(value float64, count int, interval *Interval) Metric {
	return Metric{Value: value, Count: count, LowSample: count < lowSampleFloor, Interval: interval}
}

func extractionMode(observations []Observation, ranks []RankObservation) string {
	modes := map[string]struct{}{}
	for _, observation := range observations {
		if observation.ExtractionMode != "" {
			modes[observation.ExtractionMode] = struct{}{}
		}
	}
	for _, observation := range ranks {
		if observation.ExtractionMode != "" {
			modes[observation.ExtractionMode] = struct{}{}
		}
	}
	if len(modes) == 1 {
		for mode := range modes {
			return mode
		}
	}
	if len(modes) > 1 {
		return "mixed"
	}
	return ""
}

func observationClusterCount(observations []Observation) int {
	return len(groupObservations(observations))
}

func groupObservations(observations []Observation) map[string][]Observation {
	groups := make(map[string][]Observation)
	for i, observation := range observations {
		key := observation.TaskID
		if key == "" {
			key = fmt.Sprintf("observation:%08d", i)
		}
		groups[key] = append(groups[key], observation)
	}
	return groups
}

func decomposeErrors(observations []Observation) ErrorDecomposition {
	type band struct {
		id, label string
		matches   func(float64) bool
	}
	bands := []band{
		{"near_zero", "[0.00,0.05]", func(v float64) bool { return v <= 0.05 }},
		{"moderate", "(0.05,0.20]", func(v float64) bool { return v > 0.05 && v <= 0.20 }},
		{"large", "(0.20,1.00]", func(v float64) bool { return v > 0.20 }},
	}
	result := ErrorDecomposition{Strata: make([]ErrorStratum, len(bands))}
	tasks := map[string]struct{}{}
	stratumTasks := make([]map[string]struct{}, len(bands))
	for i, b := range bands {
		result.Strata[i] = ErrorStratum{ID: b.id, Range: b.label}
		stratumTasks[i] = map[string]struct{}{}
	}
	for index, observation := range observations {
		p := clamp01(observation.Predicted)
		if p == 0.5 || (p > 0.5) == observation.Won {
			continue
		}
		result.TotalWrong++
		taskID := observation.TaskID
		if taskID == "" {
			taskID = fmt.Sprintf("observation:%08d", index)
		}
		tasks[taskID] = struct{}{}
		difference := math.Abs(observation.MeanDifference)
		for i, b := range bands {
			if b.matches(difference) {
				result.Strata[i].Count++
				stratumTasks[i][taskID] = struct{}{}
				break
			}
		}
	}
	result.TotalWrongTasks = len(tasks)
	for i := range result.Strata {
		result.Strata[i].TaskCount = len(stratumTasks[i])
		share := 0.0
		if result.TotalWrong > 0 {
			share = float64(result.Strata[i].Count) / float64(result.TotalWrong)
		}
		result.Strata[i].ShareOfErrors = metric(share, result.TotalWrong, nil)
	}
	return result
}

func applyBootstrap(report *Report, observations []Observation) {
	groups := groupObservations(observations)
	if len(groups) == 0 {
		return
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := map[string][]float64{
		"ece": {}, "mce": {}, "brier": {}, "auc": {}, "accuracy": {},
	}
	errorShares := make([][]float64, len(report.Errors.Strata))
	for replicate := 0; replicate < bootstrapReplicates; replicate++ {
		sample := make([]Observation, 0, len(observations))
		for draw := range keys {
			key := keys[deterministicClusterIndex(replicate, draw, len(keys))]
			sample = append(sample, groups[key]...)
		}
		point := analyzePoint(sample)
		values["ece"] = append(values["ece"], point.ECE)
		values["mce"] = append(values["mce"], point.MCE)
		values["brier"] = append(values["brier"], point.Brier)
		values["auc"] = append(values["auc"], point.AUC)
		values["accuracy"] = append(values["accuracy"], point.Accuracy)
		errors := decomposeErrors(sample)
		if errors.TotalWrong == 0 {
			continue
		}
		for i, stratum := range errors.Strata {
			errorShares[i] = append(errorShares[i], stratum.ShareOfErrors.Value)
		}
	}
	clusters := len(keys)
	report.Metrics.ECE.Interval = percentileInterval(values["ece"], clusters)
	report.Metrics.MCE.Interval = percentileInterval(values["mce"], clusters)
	report.Metrics.Brier.Interval = percentileInterval(values["brier"], clusters)
	report.Metrics.AUC.Interval = percentileInterval(values["auc"], clusters)
	report.Metrics.Accuracy.Interval = percentileInterval(values["accuracy"], clusters)
	for i := range report.Errors.Strata {
		report.Errors.Strata[i].ShareOfErrors.Interval = percentileInterval(errorShares[i], clusters)
	}
}

func deterministicClusterIndex(replicate, draw, clusters int) int {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", SchemaVersion, replicate, draw)))
	return int(binary.BigEndian.Uint64(digest[:8]) % uint64(clusters))
}

func percentileInterval(values []float64, clusters int) *Interval {
	if len(values) == 0 {
		return nil
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	lowerIndex := int(math.Floor(0.025 * float64(len(ordered)-1)))
	upperIndex := int(math.Ceil(0.975 * float64(len(ordered)-1)))
	return &Interval{
		Lower:               ordered[lowerIndex],
		Upper:               ordered[upperIndex],
		Level:               intervalLevel,
		Method:              "task_cluster_percentile_bootstrap",
		Replicates:          bootstrapReplicates,
		EffectiveReplicates: len(ordered),
		Clusters:            clusters,
	}
}

func analyzeAbsolute(observations []RankObservation) *AbsoluteReport {
	predicted := make([]float64, len(observations))
	actual := make([]float64, len(observations))
	for i, observation := range observations {
		predicted[i] = observation.Predicted
		actual[i] = observation.Actual
	}
	value := Spearman(predicted, actual)
	report := &AbsoluteReport{
		Spearman: metric(value, len(observations), nil),
		Rows:     append([]RankObservation(nil), observations...),
	}
	groups := groupRankObservations(observations)
	if len(groups) == 0 {
		return report
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]float64, 0, bootstrapReplicates)
	for replicate := 0; replicate < bootstrapReplicates; replicate++ {
		var samplePredicted, sampleActual []float64
		for draw := range keys {
			key := keys[deterministicClusterIndex(replicate, draw, len(keys))]
			for _, observation := range groups[key] {
				samplePredicted = append(samplePredicted, observation.Predicted)
				sampleActual = append(sampleActual, observation.Actual)
			}
		}
		values = append(values, Spearman(samplePredicted, sampleActual))
	}
	report.Spearman.Interval = percentileInterval(values, len(keys))
	return report
}

func groupRankObservations(observations []RankObservation) map[string][]RankObservation {
	groups := make(map[string][]RankObservation)
	for i, observation := range observations {
		key := observation.TaskID
		if key == "" {
			key = fmt.Sprintf("rank-observation:%08d", i)
		}
		groups[key] = append(groups[key], observation)
	}
	return groups
}

// auc is the probability that a randomly chosen won observation carries a
// higher predicted probability than a randomly chosen lost one, computed as the
// rank-sum statistic with ties contributing one half.
func auc(observations []Observation) float64 {
	pos, neg := 0, 0
	for _, o := range observations {
		if o.Won {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		// One class is absent, so ranking cannot be assessed. Report chance
		// rather than a perfect or degenerate score.
		return 0.5
	}

	sorted := make([]Observation, len(observations))
	copy(sorted, observations)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Predicted < sorted[j].Predicted })

	// Mid-ranks so that tied predictions cannot inflate or deflate the score.
	ranks := make([]float64, len(sorted))
	for i := 0; i < len(sorted); {
		j := i
		for j < len(sorted) && sorted[j].Predicted == sorted[i].Predicted {
			j++
		}
		mid := float64(i+j+1) / 2
		for k := i; k < j; k++ {
			ranks[k] = mid
		}
		i = j
	}

	sum := 0.0
	for i, o := range sorted {
		if o.Won {
			sum += ranks[i]
		}
	}
	return (sum - float64(pos)*float64(pos+1)/2) / (float64(pos) * float64(neg))
}

// monotone reports whether observed frequency is non-decreasing across bins
// ordered by predicted probability. Bins with no observations are skipped
// rather than treated as zero.
func monotone(bins []Bin) bool {
	for i := 1; i < len(bins); i++ {
		if bins[i].MeanObserved < bins[i-1].MeanObserved {
			return false
		}
	}
	return true
}

// Spearman is the rank correlation between a predicted score and a realised
// reward, for arms that score trajectories in isolation and produce no pairwise
// comparison at all. It returns 0 when either side is constant, since no
// monotone relationship can be detected there.
func Spearman(predicted, actual []float64) float64 {
	if len(predicted) != len(actual) || len(predicted) < 2 {
		return 0
	}
	rp, rq := midRanks(predicted), midRanks(actual)
	return pearson(rp, rq)
}

func midRanks(values []float64) []float64 {
	type indexed struct {
		value float64
		index int
	}
	order := make([]indexed, len(values))
	for i, v := range values {
		order[i] = indexed{v, i}
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].value < order[j].value })
	ranks := make([]float64, len(values))
	for i := 0; i < len(order); {
		j := i
		for j < len(order) && order[j].value == order[i].value {
			j++
		}
		mid := float64(i+j+1) / 2
		for k := i; k < j; k++ {
			ranks[order[k].index] = mid
		}
		i = j
	}
	return ranks
}

func pearson(a, b []float64) float64 {
	n := float64(len(a))
	var ma, mb float64
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma /= n
	mb /= n
	var cov, va, vb float64
	for i := range a {
		da, db := a[i]-ma, b[i]-mb
		cov += da * db
		va += da * da
		vb += db * db
	}
	if va == 0 || vb == 0 {
		return 0
	}
	return cov / math.Sqrt(va*vb)
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
