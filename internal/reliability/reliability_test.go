package reliability

import (
	"math"
	"reflect"
	"testing"
)

func obs(p float64, won bool, n int) []Observation {
	out := make([]Observation, n)
	for i := range out {
		out[i] = Observation{Predicted: p, Won: won}
	}
	return out
}

func TestPerfectCalibrationScoresZeroError(t *testing.T) {
	// Ten observations in the 0.7-0.8 bin, seven of them won: predicted mean
	// 0.75, observed 0.7, so ECE is exactly 0.05 and nothing else contributes.
	var o []Observation
	o = append(o, obs(0.75, true, 7)...)
	o = append(o, obs(0.75, false, 3)...)
	r := Analyze(o)
	if len(r.Bins) != 1 {
		t.Fatalf("bins = %d, want 1", len(r.Bins))
	}
	if math.Abs(r.ECE-0.05) > 1e-12 || math.Abs(r.MCE-0.05) > 1e-12 {
		t.Fatalf("ECE %v MCE %v, want 0.05 each", r.ECE, r.MCE)
	}
	// Brier: 7*(0.75-1)^2 + 3*(0.75-0)^2 all over 10 = (0.4375 + 1.6875)/10
	if want := 0.21250; math.Abs(r.Brier-want) > 1e-12 {
		t.Fatalf("Brier = %v, want %v", r.Brier, want)
	}
	// Every prediction points the same way and seven of ten were right.
	if math.Abs(r.Accuracy-0.7) > 1e-12 {
		t.Fatalf("accuracy = %v, want 0.7", r.Accuracy)
	}
}

func TestFoldedConfidenceWouldHideDirectionErrors(t *testing.T) {
	// A signal that is confidently wrong in both directions. Folded to
	// confidence it looks like a well-populated 0.9 bin; on the raw
	// probability it is two bins that are each maximally miscalibrated, which
	// is what ECE must report.
	var o []Observation
	o = append(o, obs(0.95, false, 20)...)
	o = append(o, obs(0.05, true, 20)...)
	r := Analyze(o)
	if r.ECE < 0.9 {
		t.Fatalf("ECE = %v, want near 0.95; a folded metric would report ~0", r.ECE)
	}
	if r.Accuracy != 0 {
		t.Fatalf("accuracy = %v, want 0 for a fully inverted signal", r.Accuracy)
	}
	if r.AUC != 0 {
		t.Fatalf("AUC = %v, want 0 for a perfectly inverted ranking", r.AUC)
	}
}

func TestMonotoneDetectsAnObservedBinReversal(t *testing.T) {
	rising := []Observation{}
	rising = append(rising, obs(0.15, false, 9)...)
	rising = append(rising, obs(0.15, true, 1)...)
	rising = append(rising, obs(0.85, true, 9)...)
	rising = append(rising, obs(0.85, false, 1)...)
	if !Analyze(rising).Monotone {
		t.Fatal("a rising curve reported as non-monotone")
	}

	// The shape measured in the development artifacts: the middle band lands
	// below the low band. This invalidates the raw gate for that sample without
	// making a universal claim about held-out recalibration.
	inverted := []Observation{}
	inverted = append(inverted, obs(0.55, true, 5)...)
	inverted = append(inverted, obs(0.55, false, 5)...)
	inverted = append(inverted, obs(0.75, true, 3)...)
	inverted = append(inverted, obs(0.75, false, 7)...)
	if Analyze(inverted).Monotone {
		t.Fatal("a curve whose middle band falls below the low band reported as monotone")
	}
}

func TestAUCSeparatesRankingFromCalibration(t *testing.T) {
	// Perfectly ranked and wildly overconfident: AUC 1, ECE large. Reporting
	// only one of the two would call this signal either flawless or useless.
	var o []Observation
	o = append(o, obs(1.0, true, 10)...)
	o = append(o, obs(0.9, false, 10)...)
	r := Analyze(o)
	if math.Abs(r.AUC-1) > 1e-12 {
		t.Fatalf("AUC = %v, want 1 for a perfect ranking", r.AUC)
	}
	if r.ECE < 0.4 {
		t.Fatalf("ECE = %v, want large for a badly calibrated signal", r.ECE)
	}
}

func TestAUCCountsTiesAsHalf(t *testing.T) {
	// Every prediction identical: ranking carries no information at all.
	var o []Observation
	o = append(o, obs(0.6, true, 10)...)
	o = append(o, obs(0.6, false, 10)...)
	if got := Analyze(o).AUC; math.Abs(got-0.5) > 1e-12 {
		t.Fatalf("AUC = %v, want 0.5 when every prediction ties", got)
	}
}

func TestAUCReportsChanceWhenOneOutcomeIsAbsent(t *testing.T) {
	if got := Analyze(obs(0.9, true, 10)).AUC; got != 0.5 {
		t.Fatalf("AUC = %v with no losses, want 0.5 rather than a perfect score", got)
	}
}

func TestExactHalfCountsAsHalfCorrect(t *testing.T) {
	// The judge arm ties constantly and every tie lands here. Scoring these as
	// wins or as losses would move the judge's accuracy by tens of points.
	r := Analyze(obs(0.5, true, 10))
	if math.Abs(r.Accuracy-0.5) > 1e-12 {
		t.Fatalf("accuracy = %v, want 0.5 for predictions of exactly 0.5", r.Accuracy)
	}
	if math.Abs(r.Brier-0.25) > 1e-12 {
		t.Fatalf("Brier = %v, want 0.25", r.Brier)
	}
}

func TestEmptyBinsAreSkippedRatherThanCountedAsZero(t *testing.T) {
	var o []Observation
	o = append(o, obs(0.05, false, 10)...)
	o = append(o, obs(0.95, true, 10)...)
	r := Analyze(o)
	if len(r.Bins) != 2 {
		t.Fatalf("bins = %d, want 2; the eight empty bins must not appear", len(r.Bins))
	}
	// Both bins are near-perfectly calibrated, so an empty bin treated as a
	// zero-observed gap would push MCE to 1.
	if r.MCE > 0.06 {
		t.Fatalf("MCE = %v, want small; empty bins are leaking in", r.MCE)
	}
}

func TestLowSampleIsMarkedNotSuppressed(t *testing.T) {
	r := Analyze(obs(0.8, true, 5))
	if !r.LowSample {
		t.Fatal("five observations not marked low sample")
	}
	if r.Count != 5 || len(r.Bins) == 0 {
		t.Fatal("low-sample report suppressed its own numbers instead of labelling them")
	}
	if Analyze(obs(0.8, true, 30)).LowSample {
		t.Fatal("thirty observations marked low sample")
	}
}

func TestAnalyzeHandlesNoObservations(t *testing.T) {
	r := Analyze(nil)
	if r.Count != 0 || len(r.Bins) != 0 || r.ECE != 0 {
		t.Fatalf("empty analysis = %+v, want a zero report", r)
	}
	if r.SchemaVersion != SchemaVersion || r.DataRole != DevelopmentDataRole {
		t.Fatalf("empty schema/role = %q/%q", r.SchemaVersion, r.DataRole)
	}
	if !r.Metrics.AUC.LowSample || r.Metrics.AUC.Count != 0 {
		t.Fatalf("empty metric evidence = %+v", r.Metrics.AUC)
	}
}

func TestSpearmanRanksWithTies(t *testing.T) {
	cases := []struct {
		name              string
		predicted, actual []float64
		want              float64
	}{
		{"perfect agreement", []float64{1, 2, 3, 4}, []float64{10, 20, 30, 40}, 1},
		{"perfect inversion", []float64{1, 2, 3, 4}, []float64{40, 30, 20, 10}, -1},
		{"monotone but not linear", []float64{1, 2, 3, 4}, []float64{1, 4, 9, 16}, 1},
		{"a constant side has no detectable relationship", []float64{1, 1, 1, 1}, []float64{1, 2, 3, 4}, 0},
		{"mismatched lengths", []float64{1, 2}, []float64{1}, 0},
	}
	for _, tc := range cases {
		if got := Spearman(tc.predicted, tc.actual); math.Abs(got-tc.want) > 1e-12 {
			t.Fatalf("%s: Spearman = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAnalyzeBindsTaskClusterIntervalsAndSourceRows(t *testing.T) {
	observations := []Observation{
		{ID: "a:0", TaskID: "a", ExtractionMode: "verifier", Predicted: 0.9, Won: true, MeanDifference: 0.4},
		{ID: "a:1", TaskID: "a", ExtractionMode: "verifier", Predicted: 0.8, Won: true, MeanDifference: 0.3},
		{ID: "b:0", TaskID: "b", ExtractionMode: "verifier", Predicted: 0.2, Won: false, MeanDifference: -0.3},
		{ID: "b:1", TaskID: "b", ExtractionMode: "verifier", Predicted: 0.1, Won: false, MeanDifference: -0.4},
	}
	first := Analyze(observations)
	second := Analyze(observations)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("task-cluster bootstrap is not deterministic")
	}
	if first.SchemaVersion != SchemaVersion || first.DataRole != DevelopmentDataRole || first.ExtractionMode != "verifier" {
		t.Fatalf("schema identity = %#v", first)
	}
	if first.ClusterCount != 2 || len(first.SourceRows) != 4 {
		t.Fatalf("clusters/source rows = %d/%d, want 2/4", first.ClusterCount, len(first.SourceRows))
	}
	for name, metric := range map[string]Metric{
		"ece": first.Metrics.ECE, "mce": first.Metrics.MCE, "brier": first.Metrics.Brier,
		"auc": first.Metrics.AUC, "accuracy": first.Metrics.Accuracy,
	} {
		if metric.Count != 4 || !metric.LowSample || metric.Interval == nil {
			t.Fatalf("%s metric evidence = %+v", name, metric)
		}
		if metric.Interval.Clusters != 2 || metric.Interval.Replicates != bootstrapReplicates || metric.Interval.EffectiveReplicates != bootstrapReplicates {
			t.Fatalf("%s interval = %+v", name, metric.Interval)
		}
	}
}

func TestErrorDecompositionSeparatesFixedDifferenceBands(t *testing.T) {
	observations := []Observation{
		{TaskID: "near", Predicted: 0.9, Won: false, MeanDifference: 0.01},
		{TaskID: "moderate", Predicted: 0.9, Won: false, MeanDifference: 0.10},
		{TaskID: "large", Predicted: 0.9, Won: false, MeanDifference: 0.40},
		{TaskID: "correct", Predicted: 0.9, Won: true, MeanDifference: 0.40},
		{TaskID: "tie", Predicted: 0.5, Won: false},
	}
	report := Analyze(observations)
	if report.Errors.TotalWrong != 3 || report.Errors.TotalWrongTasks != 3 {
		t.Fatalf("wrong decisions/tasks = %d/%d, want 3/3", report.Errors.TotalWrong, report.Errors.TotalWrongTasks)
	}
	if len(report.Errors.Strata) != 3 {
		t.Fatalf("strata = %d, want 3", len(report.Errors.Strata))
	}
	for _, stratum := range report.Errors.Strata {
		if stratum.Count != 1 || stratum.TaskCount != 1 || math.Abs(stratum.ShareOfErrors.Value-1.0/3.0) > 1e-12 {
			t.Fatalf("stratum %s = %+v", stratum.ID, stratum)
		}
		if stratum.ShareOfErrors.Interval == nil || stratum.ShareOfErrors.Interval.EffectiveReplicates == 0 {
			t.Fatalf("stratum %s has no cluster interval", stratum.ID)
		}
	}
}

func TestAnalyzeWithAbsoluteKeepsSpearmanSeparate(t *testing.T) {
	ranks := []RankObservation{
		{ID: "a:0", TaskID: "a", ExtractionMode: "judge", Predicted: 0.1, Actual: 0},
		{ID: "a:1", TaskID: "a", ExtractionMode: "judge", Predicted: 0.2, Actual: 1},
		{ID: "b:0", TaskID: "b", ExtractionMode: "judge", Predicted: 0.8, Actual: 2},
		{ID: "b:1", TaskID: "b", ExtractionMode: "judge", Predicted: 0.9, Actual: 3},
	}
	report := AnalyzeWithAbsolute(nil, ranks)
	if report.Count != 0 || report.Absolute == nil {
		t.Fatalf("pairwise/absolute report = %d/%#v", report.Count, report.Absolute)
	}
	if report.Absolute.Spearman.Value != 1 || report.Absolute.Spearman.Count != 4 || !report.Absolute.Spearman.LowSample {
		t.Fatalf("Spearman evidence = %+v", report.Absolute.Spearman)
	}
	if report.Absolute.Spearman.Interval == nil || report.Absolute.Spearman.Interval.Clusters != 2 {
		t.Fatalf("Spearman interval = %+v", report.Absolute.Spearman.Interval)
	}
	if len(report.Absolute.Rows) != 4 || report.ExtractionMode != "judge" {
		t.Fatalf("rank rows/mode = %d/%q", len(report.Absolute.Rows), report.ExtractionMode)
	}
}
