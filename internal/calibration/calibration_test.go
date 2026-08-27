package calibration

import (
	"math"
	"testing"
)

func TestPlattConvergenceAndDeterminism(t *testing.T) {
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleCalibration, ConditionalDiff: -2, Predicted: 0.1, Won: new(false)},
		{ID: "2", TaskID: "t1", SplitRole: RoleCalibration, ConditionalDiff: -1, Predicted: 0.2, Won: new(false)},
		{ID: "3", TaskID: "t2", SplitRole: RoleCalibration, ConditionalDiff: 0, Predicted: 0.5, Won: new(true)},
		{ID: "4", TaskID: "t2", SplitRole: RoleCalibration, ConditionalDiff: 1, Predicted: 0.8, Won: new(true)},
		{ID: "5", TaskID: "t3", SplitRole: RoleCalibration, ConditionalDiff: 2, Predicted: 0.9, Won: new(true)},
		{ID: "6", TaskID: "t3", SplitRole: RoleCalibration, ConditionalDiff: -0.5, Predicted: 0.3, Won: new(false)},
		{ID: "7", TaskID: "t4", SplitRole: RoleCalibration, ConditionalDiff: 0.5, Predicted: 0.6, Won: new(true)},
		{ID: "8", TaskID: "t4", SplitRole: RoleCalibration, ConditionalDiff: -1.5, Predicted: 0.15, Won: new(false)},
		{ID: "9", TaskID: "t5", SplitRole: RoleCalibration, ConditionalDiff: 1.5, Predicted: 0.85, Won: new(true)},
		{ID: "10", TaskID: "t5", SplitRole: RoleCalibration, ConditionalDiff: 0.2, Predicted: 0.55, Won: new(true)},
		{ID: "11", TaskID: "t6", SplitRole: RoleCalibration, ConditionalDiff: -0.2, Predicted: 0.45, Won: new(false)},
	}
	p1, err := FitPlatt(obs)
	if err != nil {
		t.Fatalf("FitPlatt error: %v", err)
	}
	p2, _ := FitPlatt(obs)
	if p1.A != p2.A || p1.B != p2.B {
		t.Fatalf("not deterministic: %+v vs %+v", p1, p2)
	}
	if p1.Calibrate(-2) >= p1.Calibrate(2) {
		t.Fatalf("Platt not monotonic: calibrate(-2)=%f calibrate(2)=%f params %+v", p1.Calibrate(-2), p1.Calibrate(2), p1)
	}
	if math.IsNaN(p1.A) || math.IsNaN(p1.B) || math.IsInf(p1.A, 0) || math.IsInf(p1.B, 0) {
		t.Fatalf("non-finite params %+v", p1)
	}
}

func TestIsotonicMonotonicityAndTies(t *testing.T) {
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleCalibration, Predicted: 0.1, Won: new(false)},
		{ID: "2", TaskID: "t1", SplitRole: RoleCalibration, Predicted: 0.1, Won: new(true)},
		{ID: "3", TaskID: "t2", SplitRole: RoleCalibration, Predicted: 0.4, Won: new(false)},
		{ID: "4", TaskID: "t2", SplitRole: RoleCalibration, Predicted: 0.6, Won: new(true)},
		{ID: "5", TaskID: "t3", SplitRole: RoleCalibration, Predicted: 0.8, Won: new(true)},
		{ID: "6", TaskID: "t3", SplitRole: RoleCalibration, Predicted: 0.9, Won: new(true)},
	}
	m := FitIsotonic(obs)
	if len(m.Blocks) == 0 {
		t.Fatal("empty blocks")
	}
	for i := 1; i < len(m.Blocks); i++ {
		if m.Blocks[i].Y < m.Blocks[i-1].Y-1e-9 {
			t.Fatalf("not monotone block %d %f < %d %f", i, m.Blocks[i].Y, i-1, m.Blocks[i-1].Y)
		}
	}
	if m.Calibrate(0.1) > m.Calibrate(0.9)+1e-9 {
		t.Fatalf("calibrate not monotone 0.1=%f 0.9=%f", m.Calibrate(0.1), m.Calibrate(0.9))
	}
}

func TestSelectiveRiskCoverageAURC(t *testing.T) {
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.9, Won: new(true)},
		{ID: "2", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.8, Won: new(false)},
		{ID: "3", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.3, Won: new(true)},
		{ID: "4", TaskID: "t4", SplitRole: RoleTest, Predicted: 0.2, Won: new(false)},
	}
	est := func(o Observation) float64 { return o.Predicted }
	m := EvaluateSelective(obs, 0.5, est)
	if math.Abs(m.Coverage-0.5) > 1e-9 {
		t.Fatalf("coverage %f want 0.5", m.Coverage)
	}
	if math.Abs(m.SelectiveRisk-0.5) > 1e-9 {
		t.Fatalf("selectiveRisk %f want 0.5", m.SelectiveRisk)
	}
	if m.AURC <= 0 || m.AURC >= 1 {
		t.Fatalf("AURC out of range %f", m.AURC)
	}
	m2 := EvaluateSelective(obs, 0.95, est)
	if !math.IsNaN(m2.SelectiveRisk) && m2.Coverage == 0 {
		// NaN is correct when no selected; if not NaN, must not be 0 coverage case
		t.Fatalf("expected NaN risk at coverage 0, got %f", m2.SelectiveRisk)
	}
}

func TestBootstrapReproducibility(t *testing.T) {
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleCalibration, Predicted: 0.1, Won: new(false)},
		{ID: "2", TaskID: "t1", SplitRole: RoleCalibration, Predicted: 0.11, Won: new(false)},
		{ID: "3", TaskID: "t2", SplitRole: RoleCalibration, Predicted: 0.8, Won: new(true)},
		{ID: "4", TaskID: "t2", SplitRole: RoleCalibration, Predicted: 0.81, Won: new(true)},
		{ID: "5", TaskID: "t3", SplitRole: RoleCalibration, Predicted: 0.5, Won: new(false)},
		{ID: "6", TaskID: "t3", SplitRole: RoleCalibration, Predicted: 0.51, Won: new(true)},
	}
	fn := func(rows []Observation) float64 {
		sum := 0.0
		for _, r := range rows {
			sum += r.Predicted
		}
		return sum / float64(len(rows))
	}
	a := clusterBootstrapInterval(obs, fn, 42)
	b := clusterBootstrapInterval(obs, fn, 42)
	if a == nil || b == nil {
		t.Fatal("nil interval")
	}
	if a.Lower != b.Lower || a.Upper != b.Upper {
		t.Fatalf("not reproducible %v vs %v", a, b)
	}
	c := clusterBootstrapInterval(obs, fn, 99)
	if c == nil {
		t.Fatal("nil interval for seed 99")
	}
	if c.Lower > c.Upper {
		t.Fatalf("invalid interval %v", c)
	}
}

func TestLeakageRejected(t *testing.T) {
	cal := []Observation{
		{ID: "c1", TaskID: "t1", SplitRole: RoleCalibration, Predicted: 0.5, Won: new(true)},
	}
	test := []Observation{
		{ID: "t1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.5, Won: new(true)},
	}
	if err := ValidateSplit(test, RoleCalibration); err == nil {
		t.Fatal("expected leakage error for test split validated as calibration")
	}
	if err := ValidateSplit(cal, RoleCalibration); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if err := ValidateSplit(test, RoleTest); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if err := ValidateSplit(nil, RoleTest); err == nil {
		t.Fatal("expected error on empty")
	}
	bad := []Observation{{ID: "x", SplitRole: RoleTest, Predicted: 0.5, Won: new(true)}}
	if err := ValidateSplit(bad, RoleTest); err == nil {
		t.Fatal("expected error on missing task_id")
	}
}

func TestBuildReportDeterministic(t *testing.T) {
	schema := FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}}
	lc, err := NewLifecycle(schema, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("lifecycle %v", err)
	}
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.9, Won: new(true)},
		{ID: "2", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.2, Won: new(false)},
	}
	est := func(o Observation) float64 { return o.Predicted }
	a, err := BuildReport("platt", schema, lc, obs, 0.5, est, 42)
	if err != nil {
		t.Fatalf("build %v", err)
	}
	b, _ := BuildReport("platt", schema, lc, obs, 0.5, est, 42)
	if a.Digest != b.Digest {
		t.Fatalf("digest not deterministic %s vs %s", a.Digest, b.Digest)
	}
	if a.TaskCount != 2 || a.ObservationCount != 2 {
		t.Fatalf("counts %+v", a)
	}
}

func TestNewLifecycleFromFile(t *testing.T) {
	schema := FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}}
	lc, err := NewLifecycleFromFile("testdata/lifecycle.json", schema)
	if err != nil {
		t.Fatalf("load %v", err)
	}
	if lc.SplitDigest != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("split digest %s", lc.SplitDigest)
	}
	if _, err := NewLifecycleFromFile("testdata/missing.json", schema); err == nil {
		t.Fatal("expected error on missing")
	}
}
func TestSelectiveWithIntervals(t *testing.T) {
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.9, Won: new(true)},
		{ID: "2", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.8, Won: new(false)},
		{ID: "3", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.7, Won: new(true)},
		{ID: "4", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.6, Won: new(false)},
		{ID: "5", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.5, Won: new(true)},
		{ID: "6", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.4, Won: new(false)},
	}
	est := func(o Observation) float64 { return o.Predicted }
	m := EvaluateSelectiveWithIntervals(obs, 0.5, est, 42)
	if m.IntervalCoverage == nil || m.IntervalRisk == nil {
		t.Fatalf("intervals nil %+v", m)
	}
	if m.IntervalCoverage.Lower > m.IntervalCoverage.Upper {
		t.Fatalf("coverage interval inverted %+v", m.IntervalCoverage)
	}
}

func TestFallbackSelect(t *testing.T) {
	app := Applicability{Applicable: true, RouteScope: "opencode-go-cn", DomainScope: "terminal"}
	d := SelectWithFallback(SelectiveMetrics{}, 0.7, 0.8, app, FallbackPolicy{Kind: FallbackJudge})
	if !d.Select || d.AbstainReason != "" {
		t.Fatalf("expected select %+v", d)
	}
	d2 := SelectWithFallback(SelectiveMetrics{}, 0.7, 0.6, app, FallbackPolicy{Kind: FallbackHuman, CostCalls: 1, CostTokens: 32})
	if d2.Select || d2.AbstainReason != "below_threshold" || d2.Fallback.Kind != FallbackHuman || d2.Fallback.CostCalls != 1 {
		t.Fatalf("expected abstain %+v", d2)
	}
	app2 := Applicability{Applicable: false, Reason: "route_mismatch"}
	d3 := SelectWithFallback(SelectiveMetrics{}, 0.7, 0.9, app2, FallbackPolicy{Kind: FallbackNoAction})
	if d3.Select || d3.AbstainReason != "route_mismatch" {
		t.Fatalf("expected route mismatch abstain %+v", d3)
	}
}

type stubBudget struct {
	calls  int
	tokens int
}

func (s *stubBudget) Reserve(estimatedInputTokens int, _ float64) error {
	s.calls++
	s.tokens += estimatedInputTokens
	return nil
}

func TestChargeFallbackReservesRunBudget(t *testing.T) {
	budget := &stubBudget{}
	decision := SelectWithFallback(SelectiveMetrics{}, 0.7, 0.1, Applicability{Applicable: true}, FallbackPolicy{
		Kind: FallbackJudge, CostCalls: 2, CostTokens: 10,
	})
	if err := AccountFallback(budget, &decision); err != nil {
		t.Fatal(err)
	}
	if !decision.FallbackAccounted {
		t.Fatal("fallback not marked accounted")
	}
	if budget.calls != 2 || budget.tokens != 10 {
		t.Fatalf("budget = %+v", budget)
	}
	if err := ChargeFallback(&stubBudget{}, FallbackPolicy{Kind: FallbackJudge}); err == nil {
		t.Fatal("zero-cost judge accepted")
	}
}

func TestSelectiveIntervalsHighThresholdNaN(t *testing.T) {
	// Threshold 0.95 is above all but one cluster mean — many bootstrap samples draw no selected,
	// producing NaN risk replicates that must be filtered, not corrupt sorting.
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.1, Won: new(false)},
		{ID: "2", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.12, Won: new(false)},
		{ID: "3", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.2, Won: new(false)},
		{ID: "4", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.21, Won: new(true)},
		{ID: "5", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.96, Won: new(true)},
		{ID: "6", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.97, Won: new(false)},
	}
	est := func(o Observation) float64 { return o.Predicted }
	m := EvaluateSelectiveWithIntervals(obs, 0.95, est, 42)
	if m.IntervalCoverage == nil {
		t.Fatalf("coverage interval nil")
	}
	// Risk interval may be nil if too many NaNs, but must not be corrupted
	if m.IntervalRisk != nil && (m.IntervalRisk.Lower > m.IntervalRisk.Upper || math.IsNaN(m.IntervalRisk.Lower) || math.IsNaN(m.IntervalRisk.Upper)) {
		t.Fatalf("corrupted risk interval %+v", m.IntervalRisk)
	}
	// IsDeployable must not panic and must respect nil risk -> falls back to point risk
	_ = IsDeployable(m, 0.5, 0.1)
}
