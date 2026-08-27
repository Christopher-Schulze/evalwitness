package mode

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/cache"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/sprt"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type jointAbsoluteProvider struct {
	calls int32
}

func (p *jointAbsoluteProvider) Name() string { return "mock" }
func (p *jointAbsoluteProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}
func (p *jointAbsoluteProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	atomic.AddInt32(&p.calls, 1)
	var raw strings.Builder
	for index, tag := range request.ScoreTags {
		letter := "T"
		if index == 2 {
			letter = "A"
		}
		closing := strings.Replace(tag, "<", "</", 1)
		fmt.Fprintf(&raw, "%s%s%s\n", tag, letter, closing)
	}
	return finalizeTestResponse(request, provider.ResponseRecord{RawText: raw.String()})
}

func TestDeltaSPRTStopsEarly(t *testing.T) {
	extractTraj := func(prompt, marker string) string {
		start := strings.Index(prompt, marker)
		if start < 0 {
			return ""
		}
		start += len(marker)
		end := strings.Index(prompt[start:], "\n\n**")
		if end < 0 {
			return prompt[start:]
		}
		return prompt[start : start+end]
	}
	// Content-based scoring: the GOOD trajectory wins in both orders, so the
	// per-rep score diff is consistently large and SPRT can terminate.
	p := &scriptedProvider{
		letterFor: func(prompt string) (string, string) {
			scoreOf := func(text string) string {
				if strings.Contains(text, "GOOD") {
					return "A"
				}
				return "T"
			}
			return scoreOf(extractTraj(prompt, "**Trajectory A:**")), scoreOf(extractTraj(prompt, "**Trajectory B:**"))
		},
	}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	_, err := RunDelta(context.Background(), r, DeltaInput{
		Task:        "t",
		TrajectoryA: "GOOD solution",
		TrajectoryB: "broken attempt",
		Criteria:    []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg: DeltaConfig{
			NReps:          8,
			BiasMitigation: "both",
			UseSPRT:        true,
			SPRTParams:     sprt.Params{Alpha: 0.05, Beta: 0.05, Sigma: 0.15, Epsilon: 0.05, MinReps: 2, MaxReps: 16},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := atomic.LoadInt32(&p.calls)
	// Without SPRT: 8 reps x 2 orders = 16 calls. A clear A-vs-T margin must
	// terminate after MinReps-worth of observations.
	if got >= 16 {
		t.Errorf("SPRT did not stop early: %d calls", got)
	}
	if got < 2 {
		t.Errorf("suspiciously few calls: %d", got)
	}
}

// positionBiasedProvider always favors whichever trajectory sits in slot A,
// producing a permanently inconsistent pair under order-bias mitigation.
type positionBiasedProvider struct{ calls int32 }

func (p *positionBiasedProvider) Name() string                        { return "mock" }
func (p *positionBiasedProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *positionBiasedProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	atomic.AddInt32(&p.calls, 1)
	return finalizeTestResponse(req, provider.ResponseRecord{RawText: "<score_A>A</score_A>\n<score_B>T</score_B>"})
}

type concurrencyBoundProvider struct {
	current atomic.Int32
	maximum atomic.Int32
}

func (p *concurrencyBoundProvider) Name() string { return "concurrency-bound" }
func (p *concurrencyBoundProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}
func (p *concurrencyBoundProvider) Score(ctx context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	active := p.current.Add(1)
	defer p.current.Add(-1)
	for {
		observed := p.maximum.Load()
		if active <= observed || p.maximum.CompareAndSwap(observed, active) {
			break
		}
	}
	select {
	case <-time.After(10 * time.Millisecond):
		return finalizeTestResponse(req, provider.ResponseRecord{RawText: "ok"})
	case <-ctx.Done():
		return provider.ResponseRecord{}, ctx.Err()
	}
}

// poisonedPairProvider fails every prompt that mentions one specific trajectory
// and succeeds for all others, counting both kinds of calls.
type poisonedPairProvider struct {
	marker      string
	failedCalls atomic.Int32
	healthyCall atomic.Int32
	fatal       bool
}

func (p *poisonedPairProvider) Name() string                        { return "poisoned" }
func (p *poisonedPairProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *poisonedPairProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	prompt, _ := req.Prompt()
	if strings.Contains(prompt, p.marker) {
		p.failedCalls.Add(1)
		if p.fatal {
			return provider.ResponseRecord{}, &provider.ProviderError{
				Provider: "poisoned", Status: 401, Body: "unauthorized", Class: provider.ClassAuthFailed,
			}
		}
		return provider.ResponseRecord{}, &provider.ProviderError{
			Provider: "poisoned", Status: 429, Body: "all replicas at capacity", Class: provider.ClassRetryable,
		}
	}
	p.healthyCall.Add(1)
	return finalizeTestResponse(req, provider.ResponseRecord{RawText: "<score_A>A</score_A>\n<score_B>T</score_B>"})
}

func pairwiseWithPoisonedTrajectory(t *testing.T, p *poisonedPairProvider) error {
	t.Helper()
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	_, err := RunPairwise(context.Background(), runner, PairwiseInput{
		Task: "task",
		// Trajectory 0 carries the marker, so the first three scheduled pairs
		// all fail before any healthy pair is reached.
		Trajectories: []string{"POISONED trajectory", "beta trajectory", "gamma trajectory", "delta trajectory"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, BiasMitigation: "adaptive", MaxWorkers: 1},
	})
	return err
}

func TestFailingPairDoesNotDiscardTheRemainingPairs(t *testing.T) {
	// A pair that fails after the provider's own retries must not abort its
	// siblings: the healthy pairs still run so their responses reach the disk
	// cache and a restart resumes with real progress. The run itself still fails.
	p := &poisonedPairProvider{marker: "POISONED"}
	err := pairwiseWithPoisonedTrajectory(t, p)
	if err == nil {
		t.Fatal("run with failing pairs must return an error")
	}
	if p.failedCalls.Load() == 0 {
		t.Fatal("expected the poisoned pairs to be attempted")
	}
	// Pairs (1,2), (1,3) and (2,3) contain no poisoned trajectory and are all
	// scheduled after the three failing ones.
	if got := p.healthyCall.Load(); got < 3 {
		t.Fatalf("healthy pair calls = %d, want at least 3; the failing pairs aborted the run", got)
	}
}

// flakyExtractionProvider fails the first n calls it ever sees and succeeds
// afterwards, reproducing a free route that occasionally answers without any
// extractable distribution.
type flakyExtractionProvider struct {
	remainingFailures atomic.Int32
	calls             atomic.Int32
}

func (p *flakyExtractionProvider) Name() string { return "flaky" }
func (p *flakyExtractionProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}
func (p *flakyExtractionProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	p.calls.Add(1)
	if p.remainingFailures.Add(-1) >= 0 {
		return provider.ResponseRecord{}, &provider.ProviderError{
			Provider: "flaky", Status: 502, Body: "no extractable distribution", Class: provider.ClassRetryable,
		}
	}
	return finalizeTestResponse(req, provider.ResponseRecord{RawText: "<score_A>A</score_A>\n<score_B>T</score_B>"})
}

func TestRecoverySweepRescuesFlakyPairs(t *testing.T) {
	// A single flaky call must not fail a whole benchmark run: the recovery
	// sweep re-issues only the failed pairs, and the run then succeeds.
	p := &flakyExtractionProvider{}
	p.remainingFailures.Store(2)
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	sel, err := RunPairwise(context.Background(), runner, PairwiseInput{
		Task:         "task",
		Trajectories: []string{"alpha trajectory", "beta trajectory", "gamma trajectory", "delta trajectory"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, BiasMitigation: "adaptive", MaxWorkers: 1},
	})
	if err != nil {
		t.Fatalf("recovery sweep did not rescue the flaky pairs: %v", err)
	}
	if len(sel.Scores) != 4 {
		t.Fatalf("scores = %d, want 4", len(sel.Scores))
	}
	if p.calls.Load() < 8 {
		t.Fatalf("provider calls = %d, want at least 8 (6 pairs plus retries)", p.calls.Load())
	}
}

func TestRecoverySweepIsNotRetriedForever(t *testing.T) {
	// A route that never produces an extractable distribution must fail after
	// exactly one recovery sweep rather than looping.
	p := &flakyExtractionProvider{}
	p.remainingFailures.Store(1 << 20)
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	_, err := RunPairwise(context.Background(), runner, PairwiseInput{
		Task:         "task",
		Trajectories: []string{"alpha", "beta", "gamma", "delta"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, BiasMitigation: "adaptive", MaxWorkers: 1},
	})
	if err == nil {
		t.Fatal("a permanently broken route must fail the run")
	}
	if !strings.Contains(err.Error(), "recovery sweep") {
		t.Fatalf("error should name the recovery sweep, got: %v", err)
	}
}

func TestRouteLevelFailureStopsTheRunImmediately(t *testing.T) {
	// An auth failure repeats identically for every remaining pair, so it must
	// abort at once instead of burning the budget on the healthy ones.
	p := &poisonedPairProvider{marker: "POISONED", fatal: true}
	err := pairwiseWithPoisonedTrajectory(t, p)
	if err == nil {
		t.Fatal("route-level failure must return an error")
	}
	// Pairs already holding a worker slot still finish, so the guarantee is an
	// early stop rather than zero calls: the run must not work through all three
	// healthy pairs the way a pair-local failure does.
	if got := p.healthyCall.Load(); got >= 3 {
		t.Fatalf("healthy pair calls = %d, want fewer than 3; the route-level failure did not stop the run", got)
	}
}

func TestRunnerBoundsGlobalProviderConcurrency(t *testing.T) {
	p := &concurrencyBoundProvider{}
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 3}}
	var group sync.WaitGroup
	errorsFound := make(chan error, 12)
	for index := 0; index < 12; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := runner.Score(context.Background(), "criterion", "prompt", nil, 1)
			errorsFound <- err
		}()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	if p.maximum.Load() != 3 {
		t.Fatalf("maximum provider concurrency = %d, want 3", p.maximum.Load())
	}
}

func TestPairwiseRejectsMismatchedPreparedTrajectories(t *testing.T) {
	_, err := RunPairwise(context.Background(), &Runner{}, PairwiseInput{
		Task:                 "task",
		Trajectories:         []string{"first", "second"},
		PreparedTrajectories: []preprocess.Result{{Text: "first"}},
		Criteria:             []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
	})
	if err == nil || !strings.Contains(err.Error(), "prepared trajectory count") {
		t.Fatalf("error = %v, want prepared trajectory count error", err)
	}
}

func TestPairwiseAdaptivePolicyRunsExtraRepsOnInconsistency(t *testing.T) {
	run := func(policy string) (int32, Selection) {
		p := &positionBiasedProvider{}
		r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
		sel, err := RunPairwise(context.Background(), r, PairwiseInput{
			Task:         "t",
			Trajectories: []string{"first", "second"},
			Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
			Cfg: PairwiseConfig{
				NReps:               2,
				BiasMitigation:      "both",
				InconsistencyPolicy: policy,
				MaxWorkers:          2,
				UseSPRT:             false,
				SPRTParams:          sprt.Params{Alpha: 0.05, Beta: 0.05, Sigma: 0.15, Epsilon: 0.05, MinReps: 2, MaxReps: 6},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return atomic.LoadInt32(&p.calls), sel
	}

	flagCalls, flagSel := run("flag-only")
	adaptiveCalls, adaptiveSel := run("adaptive")

	if len(flagSel.InconsistentPairs) != 1 || len(adaptiveSel.InconsistentPairs) != 1 {
		t.Errorf("both runs must flag the inconsistent pair: flag=%v adaptive=%v",
			flagSel.InconsistentPairs, adaptiveSel.InconsistentPairs)
	}
	if adaptiveCalls <= flagCalls {
		t.Errorf("adaptive policy must spend extra reps on inconsistent pairs: adaptive=%d flag-only=%d",
			adaptiveCalls, flagCalls)
	}
}

// tokenCapture records the MaxTokens of each request.
type tokenCapture struct {
	last atomic.Int64
}

func (p *tokenCapture) Name() string                        { return "mock" }
func (p *tokenCapture) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *tokenCapture) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	p.last.Store(int64(req.MaxOutputTokens))
	var raw strings.Builder
	for _, tag := range req.ScoreTags {
		raw.WriteString(tag)
		raw.WriteString("M")
		raw.WriteString(strings.Replace(tag, "<", "</", 1))
		raw.WriteByte('\n')
	}
	return finalizeTestResponse(req, provider.ResponseRecord{RawText: raw.String()})
}

func TestDerivedMaxTokens(t *testing.T) {
	crits := []verifier.Criterion{
		verifier.BuiltinCriteria["generic"],
		verifier.BuiltinCriteria["error_signals"],
		verifier.BuiltinCriteria["specification"],
	}
	cases := []struct {
		name     string
		cfg      RunnerConfig
		criteria []verifier.Criterion
		want     int64
	}{
		// Reference parity: 4096 output budget (verifier_core.py); the prompt
		// asks for analysis before the tags, tight caps truncate to garbage.
		{"default matches reference", RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}, crits[:1], 4096},
		{"bundled default unchanged", RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", CritiqueThenScore: true, MultiCriterionBundle: true}, crits, 4096},
		{"explicit override wins", RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxTokens: 1234}, crits[:1], 1234},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &tokenCapture{}
			r := &Runner{Provider: p, Cfg: tc.cfg}
			if _, _, err := r.ScoreSingle(context.Background(), "t", "x", tc.criteria, 0); err != nil {
				t.Fatal(err)
			}
			if got := p.last.Load(); got != tc.want {
				t.Errorf("MaxTokens = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCostCapReturnsTypedError(t *testing.T) {
	p := &tokenCapture{}
	r := &Runner{
		Provider: p,
		Cost:     cost.New(1000000, 0, 1000000, false), // absurd rates force the cap
		Cfg:      RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxCostPerCall: 0.0001},
	}
	_, err := r.Score(context.Background(), "c", strings.Repeat("x", 4000), []string{"<score>"}, 256)
	if err == nil {
		t.Fatal("expected cost cap error")
	}
	var cce *CostCapError
	if !errors.As(err, &cce) {
		t.Fatalf("expected *CostCapError, got %T: %v", err, err)
	}
	if cce.CapUSD != 0.0001 {
		t.Errorf("cap = %v", cce.CapUSD)
	}
}

func TestDerivePreprocessBudget(t *testing.T) {
	cases := []struct {
		contextLimit, maxOut, want int
	}{
		{0, 256, 0}, // unknown limit: no truncation
		{1_000_000, 256, (1_000_000 - 2048 - 512 - 1024) / 2}, // large window
		{4096, 256, 512}, // tiny window clamps to floor
	}
	for _, tc := range cases {
		if got := DerivePreprocessBudget(tc.contextLimit, tc.maxOut); got != tc.want {
			t.Errorf("DerivePreprocessBudget(%d,%d) = %d, want %d", tc.contextLimit, tc.maxOut, got, tc.want)
		}
	}
}

func TestPrepareTrajectoryTruncatesWithBudget(t *testing.T) {
	var steps []string
	for i := 0; i < 60; i++ {
		steps = append(steps, fmt.Sprintf(`{"step_id":%d,"source":"agent","message":"planning step %d with a fairly long text body to consume budget","tool_calls":[],"observation":{"results":[]}}`, i, i))
	}
	traj := `{"trajectory":{"steps":[` + strings.Join(steps, ",") + `]}}`
	r := &Runner{Provider: &tokenCapture{}, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", PreprocessBudget: 200}}
	res, err := r.PrepareTrajectory(traj)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated {
		t.Fatalf("expected truncation with budget 200 over %d steps (kept=%d of %d)", len(steps), res.KeptSteps, res.OriginalSteps)
	}
	if len(res.IngestionReport.Selection) != res.OriginalSteps {
		t.Fatalf("selection accounting entries = %d, want %d", len(res.IngestionReport.Selection), res.OriginalSteps)
	}
	if tokens := (len(res.Text) + 3) / 4; tokens > 200 {
		t.Fatalf("canonical evidence retained %d tokens over 200-token budget", tokens)
	}
}

// distProvider returns both a distribution and raw text so tests can tell
// which extraction path was used.
type distProvider struct {
	lastTopLogprobs int
}

func (p *distProvider) Name() string                        { return "mock" }
func (p *distProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *distProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	p.lastTopLogprobs = req.TopLogprobs
	resp := provider.ResponseRecord{RawText: "<score>C</score>"}
	if req.TopLogprobs > 0 {
		resp.HasLogprobs = true
		// Distribution says A (1.0), raw text says C (~0.895): divergence
		// exposes which path extracted the score.
		resp.Distributions = map[string]map[string]float64{
			"<score>": {"A": 0.99},
		}
	}
	return finalizeTestResponse(req, resp)
}

func TestJudgeModeForcesTextExtraction(t *testing.T) {
	p := &distProvider{}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", JudgeMode: true}}
	out, err := RunAbsolute(context.Background(), r, AbsoluteInput{
		Task:       "t",
		Trajectory: "x",
		Criteria:   []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:        AbsoluteConfig{NReps: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.lastTopLogprobs != 0 {
		t.Errorf("judge mode must not request logprobs, got top_logprobs=%d", p.lastTopLogprobs)
	}
	// C = value 18 -> (18-1)/19
	want := (18.0 - 1) / 19
	if absF(out.Value-want) > 1e-9 {
		t.Errorf("score = %v, want %v (raw-text C)", out.Value, want)
	}
	if out.Usage.ExtractionMode != "judge" {
		t.Errorf("extraction_mode = %q, want judge", out.Usage.ExtractionMode)
	}
}

func TestVerifierModeUsesLogprobs(t *testing.T) {
	p := &distProvider{}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	out, err := RunAbsolute(context.Background(), r, AbsoluteInput{
		Task:       "t",
		Trajectory: "x",
		Criteria:   []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:        AbsoluteConfig{NReps: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Value < 0.99 {
		t.Errorf("score = %v, want ~1.0 (logprob A)", out.Value)
	}
	if out.Usage.ExtractionMode != "verifier" {
		t.Errorf("extraction_mode = %q, want verifier", out.Usage.ExtractionMode)
	}
}

func TestAddUsageUpdatesAggregateExtractionMode(t *testing.T) {
	var aggregate UsageSummary
	AddUsage(&aggregate, UsageSummary{Calls: 1, LogprobCalls: 1})
	if aggregate.ExtractionMode != "verifier" {
		t.Fatalf("first aggregate mode = %q, want verifier", aggregate.ExtractionMode)
	}
	AddUsage(&aggregate, UsageSummary{Calls: 1, JudgeTextCalls: 1})
	if aggregate.ExtractionMode != "mixed" {
		t.Fatalf("mixed aggregate mode = %q, want mixed", aggregate.ExtractionMode)
	}
}

// cacheCountingProvider counts real provider calls behind a live disk cache.
type cacheCountingProvider struct{ calls int32 }

func (p *cacheCountingProvider) Name() string                        { return "mock" }
func (p *cacheCountingProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *cacheCountingProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	atomic.AddInt32(&p.calls, 1)
	return finalizeTestResponse(req, provider.ResponseRecord{
		RawText:     "<score_A>C</score_A>\n<score_B>K</score_B>",
		HasLogprobs: true,
		Distributions: map[string]map[string]float64{
			"<score_A>": {"C": 1},
			"<score_B>": {"K": 1},
		},
	})
}

func TestRepsAreIndependentSamplesDespiteCache(t *testing.T) {
	p := &cacheCountingProvider{}
	r := &Runner{
		Provider: p,
		Cache:    cache.New(t.TempDir(), true),
		Cfg:      RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"},
	}
	crits := []verifier.Criterion{verifier.BuiltinCriteria["generic"]}
	// 3 reps of the identical prompt must hit the provider 3 times: repeated
	// verification samples the model, it must not replay rep 0 from cache.
	for rep := 0; rep < 3; rep++ {
		if _, _, err := r.ScorePair(context.Background(), "t", "a", "b", crits, rep); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&p.calls); got != 3 {
		t.Fatalf("provider calls = %d, want 3 (reps collapsed into cache)", got)
	}
	// Re-running the same reps must now be fully cache-served.
	for rep := 0; rep < 3; rep++ {
		if _, _, err := r.ScorePair(context.Background(), "t", "a", "b", crits, rep); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&p.calls); got != 3 {
		t.Fatalf("provider calls = %d, want 3 (rerun should be cache hits)", got)
	}
}

func TestAdaptivePairRejectsOrderLargerThanHardCallLimit(t *testing.T) {
	p := &cacheCountingProvider{}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MultiCriterionBundle: false}}
	_, err := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "t",
		Trajectories: []string{"first", "second"},
		Criteria: []verifier.Criterion{
			verifier.BuiltinCriteria["generic"],
			verifier.BuiltinCriteria["specification"],
			verifier.BuiltinCriteria["error_signals"],
		},
		Cfg: PairwiseConfig{
			NReps:          1,
			BiasMitigation: "adaptive",
			MaxPairCalls:   2,
		},
	})
	if err == nil {
		t.Fatal("expected adaptive pair-budget error")
	}
	if got := atomic.LoadInt32(&p.calls); got != 0 {
		t.Fatalf("provider calls = %d, want 0", got)
	}
}

func TestAdaptiveModesRejectFixedMultiRepConfiguration(t *testing.T) {
	r := &Runner{Provider: &cacheCountingProvider{}, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	criteria := []verifier.Criterion{verifier.BuiltinCriteria["generic"]}
	_, pairwiseErr := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "t",
		Trajectories: []string{"first", "second"},
		Criteria:     criteria,
		Cfg:          PairwiseConfig{NReps: 2, BiasMitigation: "adaptive"},
	})
	if pairwiseErr == nil {
		t.Fatal("pairwise accepted adaptive n_reps=2")
	}
	_, deltaErr := RunDelta(context.Background(), r, DeltaInput{
		Task:        "t",
		TrajectoryA: "first",
		TrajectoryB: "second",
		Criteria:    criteria,
		Cfg:         DeltaConfig{NReps: 2, BiasMitigation: "adaptive"},
	})
	if deltaErr == nil {
		t.Fatal("delta accepted adaptive n_reps=2")
	}
}

func TestInvalidCacheEntriesAreRejectedBeforeRuntime(t *testing.T) {
	tests := []struct {
		name        string
		judgeMode   bool
		topLogprobs int
		response    provider.ResponseRecord
	}{
		{
			name:        "verifier requires distributions",
			topLogprobs: 20,
			response:    provider.ResponseRecord{RawText: "<score_A>C</score_A>\n<score_B>K</score_B>"},
		},
		{
			name:        "verifier rejects low score mass",
			topLogprobs: 20,
			response: provider.ResponseRecord{
				RawText:     "<score_A>C</score_A>\n<score_B>K</score_B>",
				HasLogprobs: true,
				Distributions: map[string]map[string]float64{
					"<score_A>": {"C": 0.001},
					"<score_B>": {"K": 0.001},
				},
			},
		},
		{
			name:        "judge requires closing tags",
			judgeMode:   true,
			topLogprobs: 0,
			response:    provider.ResponseRecord{RawText: "analysis truncated before scores"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &cacheCountingProvider{}
			cacheStore := cache.New(t.TempDir(), true)
			request, err := provider.NewRequestEnvelope(provider.RequestOptions{
				ProviderID:      "mock",
				BaseURL:         "https://mock.invalid/v1",
				RequestedModel:  "m",
				Messages:        []provider.Message{{Role: "user", Content: "prompt"}},
				Temperature:     1,
				MaxOutputTokens: 4096,
				Logprobs:        tc.topLogprobs > 0,
				TopLogprobs:     tc.topLogprobs,
				ScoreTags:       []string{"<score_A>", "<score_B>"},
				ResponseFormat:  provider.ResponseFormatText,
				Lineage:         provider.RequestLineage{CriterionID: "criterion", SamplingSlot: "criterion"},
			})
			if err != nil {
				t.Fatal(err)
			}
			var response provider.ResponseRecord
			if tc.name == "verifier requires distributions" {
				response, err = provider.FinalizeResponse(request, tc.response)
			} else {
				response, err = finalizeTestResponse(request, tc.response)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := cacheStore.Set(request, response); err == nil {
				t.Fatal("cache accepted invalid score evidence")
			}
			r := &Runner{
				Provider: p,
				Cache:    cacheStore,
				Cfg:      RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", JudgeMode: tc.judgeMode},
			}
			out, err := r.Score(context.Background(), "criterion", "prompt", []string{"<score_A>", "<score_B>"}, 4096)
			if err != nil {
				t.Fatal(err)
			}
			if out.FromCache {
				t.Fatal("incomplete cache entry was accepted")
			}
			if got := atomic.LoadInt32(&p.calls); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
		})
	}
}

// truncatingProvider simulates output cut off mid-analysis (no score tags).
type truncatingProvider struct{ calls int32 }

func (p *truncatingProvider) Name() string                        { return "mock" }
func (p *truncatingProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *truncatingProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	atomic.AddInt32(&p.calls, 1)
	return finalizeTestResponse(req, provider.ResponseRecord{RawText: "Let me analyze the two trajectories. Trajectory A appears"})
}

func TestTruncatedResponsesFailStrictVerifierAndAreNotCached(t *testing.T) {
	p := &truncatingProvider{}
	r := &Runner{Provider: p, Cache: cache.New(t.TempDir(), true), Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	crits := []verifier.Criterion{verifier.BuiltinCriteria["generic"]}

	_, _, err := r.ScorePair(context.Background(), "t", "a", "b", crits, 0)
	if err == nil || !strings.Contains(err.Error(), "missing_tag") {
		t.Fatalf("strict verifier error = %v, want missing_tag", err)
	}
	// Second identical call must hit the provider again: garbage is not cached.
	if _, _, err := r.ScorePair(context.Background(), "t", "a", "b", crits, 0); err == nil {
		t.Fatal("second truncated response unexpectedly succeeded")
	}
	want := int32(2 * (fullVerifierExtractionRetries + 1))
	if got := atomic.LoadInt32(&p.calls); got != want {
		t.Errorf("provider calls = %d, want %d bounded retries with no cache write", got, want)
	}
}

func TestRequiredVerifierExtractionRetriesBoundedThenFails(t *testing.T) {
	p := &truncatingProvider{}
	r := &Runner{Provider: p, Cache: cache.New(t.TempDir(), true), Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", RequireLogprobs: true}}
	crits := []verifier.Criterion{verifier.BuiltinCriteria["generic"]}

	_, _, err := r.ScorePair(context.Background(), "t", "a", "b", crits, 0)
	if err == nil || !strings.Contains(err.Error(), "full verifier at Top-20 extraction required") {
		t.Fatalf("required verifier extraction error = %v", err)
	}
	want := int32(fullVerifierExtractionRetries + 1)
	if got := atomic.LoadInt32(&p.calls); got != want {
		t.Fatalf("provider calls = %d, want %d bounded attempts", got, want)
	}
}

// connectionCountingProvider never returns an extractable distribution and
// records how often its pooled connections were dropped.
type connectionCountingProvider struct {
	calls    atomic.Int32
	closures atomic.Int32
}

func (p *connectionCountingProvider) Name() string { return "counting" }
func (p *connectionCountingProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}
func (p *connectionCountingProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	p.calls.Add(1)
	return finalizeTestResponse(req, provider.ResponseRecord{RawText: "no score tags here"})
}
func (p *connectionCountingProvider) CloseIdleConnections() { p.closures.Add(1) }

// A retry over the same pooled connection reaches the same upstream, so an
// extraction retry must drop idle connections before every attempt it makes.
func TestExtractionRetryDropsPooledConnections(t *testing.T) {
	p := &connectionCountingProvider{}
	r := &Runner{Provider: p, Cache: cache.New(t.TempDir(), true), Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", RequireLogprobs: true}}
	crits := []verifier.Criterion{verifier.BuiltinCriteria["generic"]}

	if _, _, err := r.ScorePair(context.Background(), "t", "a", "b", crits, 0); err == nil {
		t.Fatal("expected the extraction gate to fail the pair")
	}
	if got, want := p.calls.Load(), int32(fullVerifierExtractionRetries+1); got != want {
		t.Fatalf("provider calls = %d, want %d", got, want)
	}
	if got, want := p.closures.Load(), int32(fullVerifierExtractionRetries); got != want {
		t.Fatalf("connection drops = %d, want one before each of the %d retries", got, want)
	}
}

type recoveringExtractionProvider struct{ calls atomic.Int32 }

func (p *recoveringExtractionProvider) Name() string { return "recovering" }
func (p *recoveringExtractionProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}
func (p *recoveringExtractionProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	if p.calls.Add(1) == 1 {
		return finalizeTestResponse(req, provider.ResponseRecord{RawText: "incomplete first response"})
	}
	return finalizeTestResponse(req, provider.ResponseRecord{
		RawText:     "<score_A>A</score_A>\n<score_B>T</score_B>",
		HasLogprobs: true,
		Distributions: map[string]map[string]float64{
			"<score_A>": {"A": 1},
			"<score_B>": {"T": 1},
		},
	})
}

func TestRequiredVerifierExtractionRecoversOnBoundedRetry(t *testing.T) {
	p := &recoveringExtractionProvider{}
	r := &Runner{Provider: p, Cache: cache.New(t.TempDir(), true), Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", RequireLogprobs: true}}
	criteria := []verifier.Criterion{verifier.BuiltinCriteria["generic"]}
	scores, usage, err := r.ScorePair(context.Background(), "t", "a", "b", criteria, 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.calls.Load() != 2 || usage.LogprobCalls != 1 || usage.JudgeTextCalls != 0 || scores["generic"][0] <= scores["generic"][1] {
		t.Fatalf("calls=%d usage=%+v scores=%v", p.calls.Load(), usage, scores)
	}
}

type uncertaintyProvider struct {
	calls     int32
	uncertain bool
}

func (p *uncertaintyProvider) Name() string { return "mock" }
func (p *uncertaintyProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}
func (p *uncertaintyProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	atomic.AddInt32(&p.calls, 1)
	distributions := map[string]map[string]float64{
		"<score_A>": {"A": 0.99},
		"<score_B>": {"T": 0.99},
	}
	if p.uncertain {
		distributions = map[string]map[string]float64{
			"<score_A>": {"J": 0.5, "K": 0.5},
			"<score_B>": {"J": 0.5, "K": 0.5},
		}
	}
	return finalizeTestResponse(req, provider.ResponseRecord{
		RawText:       "<score_A>A</score_A>\n<score_B>T</score_B>",
		HasLogprobs:   true,
		Distributions: distributions,
	})
}

func TestAdaptivePairUsesOneCallForSharpDistribution(t *testing.T) {
	p := &uncertaintyProvider{}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	selection, err := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "sharp",
		Trajectories: []string{"first", "second"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg: PairwiseConfig{
			NReps:               1,
			BiasMitigation:      "adaptive",
			MaxWorkers:          1,
			MaxPairCalls:        4,
			ConfidenceThreshold: 0.8,
			CalibrationSigma:    0.05,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&p.calls); got != 1 {
		t.Fatalf("provider calls = %d, want 1 for a sharp distribution", got)
	}
	if len(selection.PairDecisions) != 1 || selection.PairDecisions[0].Calls != 1 {
		t.Fatalf("pair decisions = %#v", selection.PairDecisions)
	}
	if selection.EscalatedPairs != 0 {
		t.Fatalf("escalated pairs = %d, want 0", selection.EscalatedPairs)
	}
}

func TestAdaptivePairStopsAtFourCallsForUncertainDistribution(t *testing.T) {
	p := &uncertaintyProvider{uncertain: true}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	selection, err := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "uncertain",
		Trajectories: []string{"first", "second"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg: PairwiseConfig{
			NReps:                1,
			BiasMitigation:       "adaptive",
			MaxWorkers:           1,
			MaxPairCalls:         4,
			ConfidenceThreshold:  0.8,
			CalibrationSigma:     0.05,
			ConfidenceEscalation: ConfidenceEscalationLegacy,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&p.calls); got != 4 {
		t.Fatalf("provider calls = %d, want hard cap 4", got)
	}
	if len(selection.PairDecisions) != 1 || selection.PairDecisions[0].Calls != 4 {
		t.Fatalf("pair decisions = %#v", selection.PairDecisions)
	}
	if selection.EscalatedPairs != 1 {
		t.Fatalf("escalated pairs = %d, want 1", selection.EscalatedPairs)
	}
}

func TestAdaptivePairDefaultDoesNotReadLegacyConfidence(t *testing.T) {
	p := &uncertaintyProvider{uncertain: true}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	selection, err := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "uncertain-default",
		Trajectories: []string{"first", "second"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg: PairwiseConfig{
			NReps:               1,
			BiasMitigation:      "adaptive",
			MaxWorkers:          1,
			MaxPairCalls:        4,
			ConfidenceThreshold: 0.8,
			CalibrationSigma:    0.05,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&p.calls); got != 1 {
		t.Fatalf("provider calls = %d, want 1 when confidence escalation is disabled", got)
	}
	if selection.EscalatedPairs != 0 {
		t.Fatalf("escalated pairs = %d, want 0", selection.EscalatedPairs)
	}
}

func TestAdaptiveDeltaUsesDistributionAndHardCallCeiling(t *testing.T) {
	tests := []struct {
		name      string
		uncertain bool
		wantCalls int32
	}{
		{name: "sharp", wantCalls: 1},
		{name: "uncertain", uncertain: true, wantCalls: 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := &uncertaintyProvider{uncertain: test.uncertain}
			r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
			verdict, err := RunDelta(context.Background(), r, DeltaInput{
				Task:        test.name,
				TrajectoryA: "first",
				TrajectoryB: "second",
				Criteria:    []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
				Cfg: DeltaConfig{
					NReps:                1,
					BiasMitigation:       "adaptive",
					MaxPairCalls:         4,
					ConfidenceThreshold:  0.8,
					CalibrationSigma:     0.05,
					ConfidenceEscalation: ConfidenceEscalationLegacy,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := atomic.LoadInt32(&p.calls); got != test.wantCalls {
				t.Fatalf("provider calls = %d, want %d", got, test.wantCalls)
			}
			if verdict.Decision == nil || verdict.Decision.Calls != int(test.wantCalls) {
				t.Fatalf("decision = %#v", verdict.Decision)
			}
		})
	}
}

func TestDynamicTournamentAdvancesActualWinner(t *testing.T) {
	extract := func(prompt, marker string) string {
		start := strings.Index(prompt, marker)
		if start < 0 {
			return ""
		}
		start += len(marker)
		end := strings.Index(prompt[start:], "\n\n**")
		if end < 0 {
			return prompt[start:]
		}
		return prompt[start : start+end]
	}
	letter := func(trajectory string) string {
		for rank, value := range []string{"T", "P", "K", "F", "A"} {
			if strings.Contains(trajectory, fmt.Sprintf("RANK_%d", rank)) {
				return value
			}
		}
		return "T"
	}
	p := &scriptedProvider{letterFor: func(prompt string) (string, string) {
		return letter(extract(prompt, "**Trajectory A:**")), letter(extract(prompt, "**Trajectory B:**"))
	}}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	selection, err := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "dynamic bracket",
		Trajectories: []string{"RANK_0", "RANK_1", "RANK_2", "RANK_3", "RANK_4"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg: PairwiseConfig{
			NReps:          1,
			BiasMitigation: "single",
			SingleElim:     true,
			MaxWorkers:     2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selection.BestIndex != 4 {
		t.Fatalf("best index = %d, want actual bracket winner 4", selection.BestIndex)
	}
	if selection.PairsEvaluated != 4 {
		t.Fatalf("pairs evaluated = %d, want n-1 = 4", selection.PairsEvaluated)
	}
	if got := atomic.LoadInt32(&p.calls); got != 4 {
		t.Fatalf("provider calls = %d, want 4", got)
	}
}

// distributionProvider answers like a real full verifier: raw text plus a
// score-token distribution, so its responses actually reach the disk cache.
type distributionProvider struct{ calls atomic.Int32 }

func (p *distributionProvider) Name() string                        { return "dist" }
func (p *distributionProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (p *distributionProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	p.calls.Add(1)
	return finalizeTestResponse(req, provider.ResponseRecord{
		RawText:     "<score_A>A</score_A>\n<score_B>T</score_B>",
		HasLogprobs: true,
		Distributions: map[string]map[string]float64{
			"<score_A>": {"A": 0.7, "B": 0.2, "C": 0.05},
			"<score_B>": {"T": 0.6, "S": 0.3, "R": 0.05},
		},
	})
}

func TestJudgeModeDoesNotReuseVerifierCacheEntries(t *testing.T) {
	// Judge and verifier requests are distinct experimental treatments. Reusing
	// one mode's response under the other's cache identity would hide the actual
	// request and sampling provenance.
	dir := t.TempDir()
	shared := cache.New(dir, true)
	provider := &distributionProvider{}

	verifier := &Runner{Provider: provider, Cache: shared, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	if _, err := verifier.Score(context.Background(), "generic", "prompt", []string{"<score_A>", "<score_B>"}, 256); err != nil {
		t.Fatal(err)
	}
	writes := provider.calls.Load()
	if writes != 1 {
		t.Fatalf("verifier run made %d calls, want 1", writes)
	}

	judge := &Runner{Provider: provider, Cache: shared, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1, JudgeMode: true}}
	out, err := judge.Score(context.Background(), "generic", "prompt", []string{"<score_A>", "<score_B>"}, 256)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != writes+1 {
		t.Fatalf("judge run made %d total calls, want an independent second call", provider.calls.Load())
	}
	if out.FromCache {
		t.Fatal("judge outcome reused a verifier cache entry")
	}
	if out.HasLogprobs || len(out.Distribution) != 0 {
		t.Fatal("a judge outcome must not carry a distribution, or usage would report it as logprob-backed")
	}
	if !strings.Contains(out.RawText, "</score_A>") {
		t.Fatalf("judge outcome lost the raw text it scores from: %q", out.RawText)
	}
}

func TestLegacyCacheIsNeverServedAsExactEvidence(t *testing.T) {
	legacyDir := t.TempDir()
	primaryDir := t.TempDir()
	const prompt = "legacy prompt"

	p := &countingProvider{}
	r := &Runner{Provider: p, Cache: cache.NewWithLegacyImport(primaryDir, legacyDir, true), Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	out, err := r.Score(context.Background(), "generic", prompt, []string{"<score>"}, 64)
	if err != nil {
		t.Fatal(err)
	}
	if out.FromCache || out.CacheNamespace == cache.SourceLegacy {
		t.Fatalf("legacy cache surfaced as exact evidence: %+v", out)
	}
	var usage UsageSummary
	r.mergeUsage(&usage, out)
	if usage.CacheHitCalls != 0 || usage.LegacyCacheHitCalls != 0 {
		t.Fatalf("cache usage = %+v", usage)
	}
	if p.calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want one exact replacement", p.calls.Load())
	}
}

func TestTournamentRoundAlsoRecoversFlakyMatches(t *testing.T) {
	// The tournament is the production default, so it must carry the same failure
	// policy as all-pairs. Before this was shared, a single flaky match discarded
	// the whole round through errgroup and no recovery sweep existed.
	p := &flakyExtractionProvider{}
	p.remainingFailures.Store(2)
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	sel, err := RunPairwise(context.Background(), runner, PairwiseInput{
		Task:         "task",
		Trajectories: []string{"alpha", "beta", "gamma", "delta"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, BiasMitigation: "adaptive", MaxWorkers: 1, SingleElim: true},
	})
	if err != nil {
		t.Fatalf("tournament did not recover a flaky match: %v", err)
	}
	if len(sel.Scores) != 4 {
		t.Fatalf("scores = %d, want 4", len(sel.Scores))
	}
	// Four trajectories need exactly three matches to produce one winner.
	if sel.PairsEvaluated != 3 {
		t.Fatalf("pairs evaluated = %d, want exactly n-1 = 3", sel.PairsEvaluated)
	}
}

func TestTournamentStopsOnARouteLevelFailure(t *testing.T) {
	// An auth failure repeats for every match, so the round must abort rather
	// than work through the bracket.
	p := &poisonedPairProvider{marker: "", fatal: true}
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	_, err := RunPairwise(context.Background(), runner, PairwiseInput{
		Task:         "task",
		Trajectories: []string{"alpha", "beta", "gamma", "delta"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, BiasMitigation: "adaptive", MaxWorkers: 1, SingleElim: true},
	})
	if err == nil {
		t.Fatal("a route-level failure must fail the tournament")
	}
}

func TestAbsoluteSelectionPicksTheHighestScore(t *testing.T) {
	// One call per trajectory, not one per pair, and the winner is the highest
	// scorer rather than the survivor of a bracket.
	p := &scriptedProvider{
		letterFor: func(prompt string) (string, string) {
			if strings.Contains(prompt, "gamma") {
				return "A", ""
			}
			return "T", ""
		},
	}
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 2}}
	sel, err := RunAbsoluteSelection(context.Background(), runner, AbsoluteSelectionInput{
		Task:         "task",
		Trajectories: []string{"alpha", "beta", "gamma", "delta"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, MaxWorkers: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Scores) != 4 {
		t.Fatalf("scores = %d, want one per trajectory", len(sel.Scores))
	}
	// Four trajectories cost exactly four calls; a tournament would need three
	// pairwise calls carrying two trajectories each.
	if got := atomic.LoadInt32(&p.calls); got != 4 {
		t.Fatalf("provider calls = %d, want exactly one per trajectory", got)
	}
	if sel.State != verifier.DecisionSelected || sel.BestIndex != 2 || sel.Wins[sel.BestIndex] != 1 {
		t.Fatal("the selected trajectory must carry the win")
	}
}

func TestAbsoluteSelectionDoesNotBreakEqualScoreTies(t *testing.T) {
	p := &scriptedProvider{letterFor: func(string) (string, string) { return "M", "" }}
	runner := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 2}}
	sel, err := RunAbsoluteSelection(context.Background(), runner, AbsoluteSelectionInput{
		Task:         "task",
		Trajectories: []string{"alpha", "beta", "gamma"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, MaxWorkers: 2, Epsilon: 0.02},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sel.State != verifier.DecisionTied || sel.BestIndex != -1 || sel.Confidence != 0 {
		t.Fatalf("equal absolute scores produced a winner: %+v", sel)
	}
}

func TestJointAbsoluteSelectionUsesOneImmutableResponse(t *testing.T) {
	provider := &jointAbsoluteProvider{}
	runner := &Runner{Provider: provider, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", MaxWorkers: 1}}
	selection, err := RunJointAbsoluteSelection(context.Background(), runner, AbsoluteSelectionInput{
		Task:         "task",
		Trajectories: []string{"alpha", "beta", "gamma", "delta", "epsilon"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, MaxWorkers: 1, Epsilon: 0.02},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 1 {
		t.Fatalf("provider calls = %d, want one", got)
	}
	if selection.State != verifier.DecisionSelected || selection.BestIndex != 2 || len(selection.AbsoluteEvidence) != 5 {
		t.Fatalf("joint selection = %+v", selection)
	}
	if selection.EvidenceStrength.Observations != 5 || selection.Usage.LogprobCalls != 1 || selection.Usage.UnextractedScores != 0 {
		t.Fatalf("joint evidence/usage = %+v %+v", selection.EvidenceStrength, selection.Usage)
	}
}

// Cache identity includes extraction mode, so a judge can only consume the
// judge-native response even when an otherwise matching verifier response exists.
func TestJudgeModeUsesOnlyItsOwnCacheIdentity(t *testing.T) {
	dir := t.TempDir()
	c := cache.New(dir, true)
	const prompt = "score this"
	criterion := verifier.BuiltinCriteria["generic"]

	verifierRequest, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID:      "mock",
		BaseURL:         "https://mock.invalid/v1",
		RequestedModel:  "m",
		Messages:        []provider.Message{{Role: "user", Content: prompt}},
		Temperature:     1,
		MaxOutputTokens: 64,
		Logprobs:        true,
		TopLogprobs:     effectiveTopLogprobs(0),
		ScoreTags:       []string{"<score>"},
		ResponseFormat:  provider.ResponseFormatText,
		Lineage:         provider.RequestLineage{CriterionID: criterion.ID, SamplingSlot: criterion.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	judgeRequest := verifierRequest
	judgeRequest.Logprobs = false
	judgeRequest.TopLogprobs = 0
	verifierResponse, err := finalizeTestResponse(verifierRequest, provider.ResponseRecord{RawText: "<score>A</score>"})
	if err != nil {
		t.Fatal(err)
	}
	judgeResponse, err := finalizeTestResponse(judgeRequest, provider.ResponseRecord{RawText: "<score>T</score>"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set(verifierRequest, verifierResponse); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(judgeRequest, judgeResponse); err != nil {
		t.Fatal(err)
	}

	p := &countingProvider{}
	r := &Runner{Provider: p, Cache: c, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1", JudgeMode: true}}
	out, err := r.Score(context.Background(), criterion.ID, prompt, []string{"<score>"}, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.RawText, "T") || strings.Contains(out.RawText, "A") {
		t.Fatalf("judge read %q, want its judge-native entry that scored T", out.RawText)
	}
	if p.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want the comparison served entirely from cache", p.calls.Load())
	}
}

type countingProvider struct{ calls atomic.Int32 }

func (p *countingProvider) Name() string { return "mock" }
func (p *countingProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}
func (p *countingProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	p.calls.Add(1)
	return finalizeTestResponse(req, provider.ResponseRecord{RawText: "<score>M</score>"})
}
