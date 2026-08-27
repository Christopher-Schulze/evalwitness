package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

// conformanceInput is the single canonical input every driver and the capture
// run share: same task, trajectories, criteria, policy. Only the entrypoint
// differs, which is exactly what the fingerprint invariant covers.
func conformanceInput(entrypoint string, verifyMode verification.Mode) verification.Input {
	return verification.Input{
		Entrypoint:   entrypoint,
		Mode:         verifyMode,
		Task:         "reverse the string abcdef and print the result",
		Trajectories: []string{"abcdef -> fedcba and prints it", "ABCDEF -> FEDCBA via upper"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Policy: verification.Policy{
			Evidence: verification.EvidenceStrictVerifier, NReps: 1, Epsilon: 0.02,
			BiasMitigation: "adaptive", InconsistencyPolicy: "flag-only",
			SelectionStrategy: "pairwise",
			MaxWorkers:        1, MaxPairCalls: 2, ConfidenceThreshold: 0.6, CalibrationSigma: 0.05,
		},
	}
}

// crossEntrypointDriverCase drives one shipped entrypoint through the real
// verification.Service against an exact replay record and reports the request
// fingerprint plus the decision.
type crossEntrypointDriverResult struct {
	fingerprint string
	winner      string
}

// captureReplayFixtureForConformance runs the FIRST entrypoint (cli.verify)
// through the real service with WrapCapture around its provider. The captured
// file is the exact replay record every subsequent entrypoint driver consumes:
// fingerprint, sampling slot, response bytes, and parsed score evidence are
// bound by the shipped capture path, not by a hand-built fixture.
func captureReplayFixtureForConformance(t *testing.T, dir string, entrypoint string) string {
	t.Helper()
	capturePath := filepath.Join(dir, "capture.jsonl")
	serviceConfig := verification.Config{
		Redact: true, PreprocessBudget: 1024, Offline: true,
		RequestProfile: verification.RequestProfile{
			ProviderID: "replay", BaseURL: "https://replay.invalid/v1", RequestedModel: "golden-delta",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: 20,
		},
		BudgetProfile: verification.BudgetProfile{MaxRetries: 1, MaxWorkers: 1, RequestTimeout: 5 * time.Second},
	}
	var capturing *replay.CapturingProvider
	service, err := verification.NewService(serviceConfig, func(_ context.Context, plan verification.Plan) (verification.Runtime, error) {
		inner := &conformanceFixtureProvider{response: "A"}
		var p provider.Provider = inner
		if plan.Input.Entrypoint == entrypoint {
			wrapped, wrapErr := replay.WrapCapture(inner, "golden-delta", capturePath, true)
			if wrapErr != nil {
				t.Fatalf("wrap capture: %v", wrapErr)
			}
			p = wrapped
			capturing = wrapped
		}
		return verification.Runtime{Runner: &mode.Runner{
			Provider: p,
			Cfg: mode.RunnerConfig{
				Model: "golden-delta", BaseURL: "https://replay.invalid/v1", ThinkingMode: "disabled", Temperature: 1,
				Entrypoint: plan.Input.Entrypoint, MaxTokens: 128, TopLogprobs: 20, MaxWorkers: 1,
			},
			Budget: mode.NewRunBudget(plan.Input.Limits),
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	input := conformanceInput(entrypoint, verification.ModePairwise)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	planResult, err := service.Plan(input)
	if err != nil {
		t.Fatalf("capture plan: %v", err)
	}
	result, execErr := service.Execute(ctx, planResult)
	// Close publishes the capture file; do it before any size assertion.
	var closeErr error
	if capturing != nil {
		closeErr = capturing.Close()
	}
	if execErr != nil {
		t.Fatalf("capture execute: %v", execErr)
	}
	if result.Lifecycle.Execution != verification.LifecycleComplete {
		t.Fatalf("capture execution state %q", result.Lifecycle.Execution)
	}
	if closeErr != nil {
		t.Fatalf("capture close: %v", closeErr)
	}
	info, err := os.Stat(capturePath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("capture produced no replay record")
	}
	return capturePath
}

// conformanceFixtureProvider answers with deterministic strict-verifier score
// evidence naming trajectory A as winner, through FinalizeResponse so the
// captured record carries the exact TASK 043 identity fields.
type conformanceFixtureProvider struct {
	response string
}

func (p *conformanceFixtureProvider) Name() string { return "replay" }

func (p *conformanceFixtureProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}

func (p *conformanceFixtureProvider) Score(ctx context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	var b []byte
	for _, tag := range req.ScoreTags {
		closing := "</" + tag[1:]
		b = append(b, []byte(tag+p.response+closing+"\n")...)
	}
	rawText := string(b)
	dist := map[string]map[string]float64{}
	tokenEvidence := make([]provider.TokenEvidence, 0, len(req.ScoreTags)*3)
	for _, tag := range req.ScoreTags {
		dist[tag] = map[string]float64{p.response: 0.9, alternateLetter(tag, p.response): 0.1}
		tokenEvidence = append(tokenEvidence,
			provider.TokenEvidence{Position: len(tokenEvidence), Token: tag, Logprob: "-0.01"},
			provider.TokenEvidence{Position: len(tokenEvidence) + 1, Token: p.response, Logprob: "-0.105", TopAlternatives: conformanceTopAlternatives(p.response)},
			provider.TokenEvidence{Position: len(tokenEvidence) + 2, Token: "end", Logprob: "-0.02"},
		)
	}
	record, err := provider.FinalizeResponse(req, provider.ResponseRecord{
		RawText: rawText, NormalizedBody: []byte(rawText),
		Distributions:        dist,
		HasLogprobs:          true,
		ObservedTopLogprobs:  20,
		OrderedTokenEvidence: tokenEvidence,
	})
	if err != nil {
		return provider.ResponseRecord{}, err
	}
	_ = ctx
	return record, nil
}

// conformanceTopAlternatives satisfies the strict verifier's minimum top-k:
// 20 alternatives with descending probabilities around the chosen letter.
func conformanceTopAlternatives(chosen string) []provider.TokenAlternative {
	alts := make([]provider.TokenAlternative, 0, verifier.MinimumVerifierTopK)
	remaining := 0.1
	for i := 0; i < verifier.MinimumVerifierTopK; i++ {
		p := remaining / float64(verifier.MinimumVerifierTopK-i)
		remaining -= p
		letter := string(rune('B' + i))
		if letter == chosen {
			letter = "Z"
		}
		alts = append(alts, provider.TokenAlternative{Token: letter, Logprob: fmt.Sprintf("%.4f", -3-float64(i))})
	}
	return alts
}

func alternateLetter(tag, chosen string) string {
	if chosen == "A" {
		return "B"
	}
	return "A"
}

// TestCrossEntrypointDriversConsumeExactReplayRecords is the TASK 039
// adapter-driver core: benchmark-shaped tasks driven through cli.verify,
// mcp.pairwise, and best-of-n consume the SAME exact replay records with no
// live provider calls, and the TASK 043 fingerprints must be equal before any
// decision comparison is meaningful.
func TestCrossEntrypointDriversConsumeExactReplayRecords(t *testing.T) {
	dir := t.TempDir()
	// The capture run itself is cli.verify in delta mode; its exact response
	// records then feed all three entrypoint drivers.
	replayPath := captureReplayFixtureForConformance(t, dir, "cli.verify")

	entrypoints := []struct {
		name string
		mode verification.Mode
	}{
		// All three run the same pairwise comparison over the same canonical
		// input. RunFingerprint includes the mode, so mode must be held
		// constant across surfaces for the fingerprint equality assertion to
		// be the TASK 043 invariant and not a mode contrast.
		{name: "cli.verify", mode: verification.ModePairwise},
		{name: "mcp.pairwise", mode: verification.ModePairwise},
		{name: "best-of-n", mode: verification.ModePairwise},
	}

	results := make(map[string]crossEntrypointDriverResult, len(entrypoints))
	for _, ep := range entrypoints {
		ep := ep
		t.Run(ep.name, func(t *testing.T) {
			caps := provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
			replayProvider, err := replay.LoadReplay(replayPath, "replay", "golden-delta", caps)
			if err != nil {
				t.Fatalf("load replay: %v", err)
			}
			result := driveEntrypointAgainstReplay(t, ep.name, ep.mode, replayProvider)
			results[ep.name] = result
		})
	}

	base := results["cli.verify"].fingerprint
	for name, got := range results {
		if got.fingerprint != base {
			t.Errorf("run fingerprint mismatch on %s: %s vs cli.verify %s — surfaces are not semantically equivalent", name, got.fingerprint, base)
		}
	}
	if results["mcp.pairwise"].winner != results["best-of-n"].winner {
		t.Errorf("selection mismatch mcp.pairwise=%q best-of-n=%q on identical replay evidence",
			results["mcp.pairwise"].winner, results["best-of-n"].winner)
	}
}

func driveEntrypointAgainstReplay(t *testing.T, entrypoint string, verifyMode verification.Mode, replayProvider provider.Provider) crossEntrypointDriverResult {
	t.Helper()
	serviceConfig := verification.Config{
		Redact: true, PreprocessBudget: 1024, Offline: true,
		RequestProfile: verification.RequestProfile{
			ProviderID: "replay", BaseURL: "https://replay.invalid/v1", RequestedModel: "golden-delta",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: 20,
		},
		BudgetProfile: verification.BudgetProfile{MaxRetries: 1, MaxWorkers: 1, RequestTimeout: 5 * time.Second},
	}
	service, err := verification.NewService(serviceConfig, func(_ context.Context, plan verification.Plan) (verification.Runtime, error) {
		return verification.Runtime{Runner: &mode.Runner{
			Provider: replayProvider,
			Cfg: mode.RunnerConfig{
				Model: "golden-delta", BaseURL: "https://replay.invalid/v1", ThinkingMode: "disabled", Temperature: 1,
				Entrypoint: plan.Input.Entrypoint, MaxTokens: 128, TopLogprobs: 20, MaxWorkers: 1,
			},
			Budget: mode.NewRunBudget(plan.Input.Limits),
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	input := conformanceInput(entrypoint, verifyMode)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	planResult, err := service.Plan(input)
	if err != nil {
		t.Fatalf("%s plan: %v", entrypoint, err)
	}
	if len(planResult.Requests.Fingerprints) == 0 {
		t.Fatalf("%s produced no request fingerprints", entrypoint)
	}
	result, err := service.Execute(ctx, planResult)
	if err != nil {
		t.Fatalf("%s execute: %v", entrypoint, err)
	}
	if result.Lifecycle.Execution != verification.LifecycleComplete {
		t.Fatalf("%s execution state %q", entrypoint, result.Lifecycle.Execution)
	}
	out := crossEntrypointDriverResult{fingerprint: planResult.RunFingerprint}
	switch {
	case result.Selection != nil && len(result.Selection.Scores) > 0:
		out.winner = selectionWinner(result.Selection.BestIndex)
	case result.Delta != nil:
		out.winner = result.Delta.Winner
	}
	return out
}

// selectionWinner names the trajectory a pairwise best_index points at, so the
// decision-conformance assertion compares semantic winners across surfaces.
func selectionWinner(bestIndex int) string {
	switch bestIndex {
	case 0:
		return "A"
	case 1:
		return "B"
	default:
		return ""
	}
}
