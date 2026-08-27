package mode

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

// scriptedProvider returns canned scores driven by a per-prompt-substring map.
// Allows simulating different verdicts for different trajectory pairs.
type scriptedProvider struct {
	caps  provider.Capabilities
	calls int32

	// map: substring of trajectory text -> score letter (A=20, T=1)
	letterFor func(prompt string) (scoreA, scoreB string)
}

type requestRecordingProvider struct {
	mu       sync.Mutex
	requests []provider.RequestEnvelope
}

func (p *requestRecordingProvider) Name() string { return "mock" }
func (p *requestRecordingProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}
func (p *requestRecordingProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	p.mu.Lock()
	p.requests = append(p.requests, request)
	p.mu.Unlock()
	return finalizeTestResponse(request, provider.ResponseRecord{RawText: "ok"})
}

func finalizeTestResponse(request provider.RequestEnvelope, response provider.ResponseRecord) (provider.ResponseRecord, error) {
	if response.ReplayStatus == "" || response.ReplayStatus == provider.ReplayStatusLive {
		response.ProviderRequestID = "test-provider-request"
	}
	if response.NormalizedBody == nil {
		response.NormalizedBody = []byte(response.RawText)
	}
	if (request.Logprobs || response.HasLogprobs) && len(response.OrderedTokenEvidence) == 0 {
		response.OrderedTokenEvidence = scoreTokenEvidenceForTest(request.ScoreTags, response.RawText, response.Distributions)
		if len(response.OrderedTokenEvidence) > 0 {
			response.HasLogprobs = true
			response.ObservedTopLogprobs = 20
		}
	}
	return provider.FinalizeResponse(request, response)
}

func scoreTokenEvidenceForTest(tags []string, rawText string, distributions map[string]map[string]float64) []provider.TokenEvidence {
	var evidence []provider.TokenEvidence
	cursor := 0
	appendToken := func(token, logprob string, alternatives []provider.TokenAlternative) {
		if token == "" {
			return
		}
		evidence = append(evidence, provider.TokenEvidence{Position: len(evidence), Token: token, Logprob: logprob, TopAlternatives: alternatives})
	}
	for _, tag := range tags {
		relative := strings.Index(rawText[cursor:], tag)
		if relative < 0 {
			continue
		}
		tagStart := cursor + relative
		scoreStart := tagStart + len(tag)
		for scoreStart < len(rawText) && strings.ContainsRune(" \t\n\r", rune(rawText[scoreStart])) {
			scoreStart++
		}
		if scoreStart >= len(rawText) {
			continue
		}
		if _, ok := verifier.TokenValue(string(rawText[scoreStart])); !ok {
			continue
		}
		appendToken(rawText[cursor:scoreStart], "0", nil)
		chosen := string(rawText[scoreStart])
		alternatives, chosenLogprob := scoreAlternativesForTest(chosen, distributions[tag])
		appendToken(chosen, chosenLogprob, alternatives)
		cursor = scoreStart + 1
	}
	appendToken(rawText[cursor:], "0", nil)
	return evidence
}

func scoreAlternativesForTest(chosen string, distribution map[string]float64) ([]provider.TokenAlternative, string) {
	probabilities := map[string]float64{}
	for token, probability := range distribution {
		if _, ok := verifier.TokenValue(token); ok && probability >= 0 {
			probabilities[token] = probability
		}
	}
	if len(probabilities) == 0 {
		probabilities[chosen] = 0.9
	}
	if _, ok := probabilities[chosen]; !ok {
		probabilities[chosen] = 0
	}
	ordered := []string{chosen}
	for letter := 'A'; letter <= 'T'; letter++ {
		token := string(letter)
		if token == chosen {
			continue
		}
		if _, ok := probabilities[token]; ok {
			ordered = append(ordered, token)
		}
	}
	visible := 0.0
	for _, probability := range probabilities {
		visible += probability
	}
	remaining := math.Max(0, 1-visible)
	for len(ordered) < 20 {
		ordered = append(ordered, "#"+strconv.Itoa(len(ordered)))
	}
	invalidCount := 20 - len(probabilities)
	invalidProbability := 0.0
	if invalidCount > 0 {
		invalidProbability = remaining / float64(invalidCount)
	}
	alternatives := make([]provider.TokenAlternative, 0, 20)
	chosenLogprob := "-1000"
	for _, token := range ordered[:20] {
		probability, ok := probabilities[token]
		if !ok {
			probability = invalidProbability
		}
		logprob := "-1000"
		if probability > 0 {
			logprob = strconv.FormatFloat(math.Log(probability), 'g', -1, 64)
		}
		if token == chosen {
			chosenLogprob = logprob
		}
		alternatives = append(alternatives, provider.TokenAlternative{Token: token, Logprob: logprob})
	}
	return alternatives, chosenLogprob
}

func (s *scriptedProvider) Name() string                        { return "mock" }
func (s *scriptedProvider) Capabilities() provider.Capabilities { return s.caps }

func (s *scriptedProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	atomic.AddInt32(&s.calls, 1)
	prompt, _ := req.Prompt()
	a, b := s.letterFor(prompt)
	rt := ""
	if len(req.ScoreTags) == 1 && req.ScoreTags[0] == "<score>" {
		if a == "" {
			a = "M"
		}
		rt = "<score>" + a + "</score>"
	} else if a != "" {
		rt += "<score_A>" + a + "</score_A>\n"
	}
	if b != "" {
		rt += "<score_B>" + b + "</score_B>\n"
	}
	return finalizeTestResponse(req, provider.ResponseRecord{
		RawText: rt,
	})
}

func TestEquivalentEntrypointsShareOneSemanticRequestFingerprint(t *testing.T) {
	entrypoints := []string{"cli", "mcp", "eval", "best-of-n"}
	recorder := &requestRecordingProvider{}
	fingerprints := make([]provider.Fingerprint, 0, len(entrypoints))
	for _, entrypoint := range entrypoints {
		runner := &Runner{
			Provider: recorder,
			Cfg: RunnerConfig{
				Model:        "m",
				BaseURL:      "https://mock.invalid/v1",
				ThinkingMode: "disabled",
				Stream:       true,
				Entrypoint:   entrypoint,
				Temperature:  1,
				TopLogprobs:  20,
			},
		}
		outcome, err := runner.Score(context.Background(), "generic@r0", "same canonical prompt", nil, 128)
		if err != nil {
			t.Fatalf("%s: %v", entrypoint, err)
		}
		fingerprints = append(fingerprints, outcome.RequestFingerprint)
	}
	for index := 1; index < len(fingerprints); index++ {
		if fingerprints[index] != fingerprints[0] {
			t.Fatalf("%s fingerprint %s != CLI %s", entrypoints[index], fingerprints[index], fingerprints[0])
		}
	}
	if len(recorder.requests) != len(entrypoints) {
		t.Fatalf("recorded %d requests, want %d", len(recorder.requests), len(entrypoints))
	}
	for index, request := range recorder.requests {
		if request.Lineage.Entrypoint != entrypoints[index] {
			t.Fatalf("lineage entrypoint %q, want %q", request.Lineage.Entrypoint, entrypoints[index])
		}
	}
}

func TestRunDelta_AWins(t *testing.T) {
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
	p := &scriptedProvider{
		letterFor: func(prompt string) (string, string) {
			aText := extractTraj(prompt, "**Trajectory A:**")
			bText := extractTraj(prompt, "**Trajectory B:**")
			scoreA := "T"
			scoreB := "T"
			if strings.Contains(aText, "FLAG_GOOD") {
				scoreA = "A"
			}
			if strings.Contains(bText, "FLAG_GOOD") {
				scoreB = "A"
			}
			return scoreA, scoreB
		},
	}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	v, err := RunDelta(context.Background(), r, DeltaInput{
		Task:        "test",
		TrajectoryA: "FLAG_GOOD trajectory",
		TrajectoryB: "bad trajectory",
		Criteria:    []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:         DeltaConfig{NReps: 1, BiasMitigation: "both"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Winner != "A" {
		t.Errorf("winner = %s, want A; verdict %+v", v.Winner, v)
	}
	if v.ScoreA <= v.ScoreB {
		t.Errorf("ScoreA (%v) should beat ScoreB (%v)", v.ScoreA, v.ScoreB)
	}
}

func TestRunDelta_BiasMitigationCalls(t *testing.T) {
	p := &scriptedProvider{
		letterFor: func(_ string) (string, string) { return "M", "M" },
	}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	_, err := RunDelta(context.Background(), r, DeltaInput{
		Task:        "test",
		TrajectoryA: "a",
		TrajectoryB: "b",
		Criteria:    []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:         DeltaConfig{NReps: 2, BiasMitigation: "both"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := atomic.LoadInt32(&p.calls)
	want := int32(4) // 2 reps x 2 orders = 4 calls
	if got != want {
		t.Errorf("calls = %d, want %d (bias-mit doubles call count)", got, want)
	}
}

func TestRunDelta_SingleOrderHalvesCalls(t *testing.T) {
	p := &scriptedProvider{
		letterFor: func(_ string) (string, string) { return "M", "M" },
	}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	_, err := RunDelta(context.Background(), r, DeltaInput{
		Task:        "test",
		TrajectoryA: "a",
		TrajectoryB: "b",
		Criteria:    []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:         DeltaConfig{NReps: 2, BiasMitigation: "single"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&p.calls); got != 2 {
		t.Errorf("calls = %d, want 2 (single-order)", got)
	}
}

func TestRunAbsolute_BasicAggregation(t *testing.T) {
	p := &scriptedProvider{
		letterFor: func(_ string) (string, string) { return "", "" },
	}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	out, err := RunAbsolute(context.Background(), r, AbsoluteInput{
		Task:       "test",
		Trajectory: "x",
		Criteria:   []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:        AbsoluteConfig{NReps: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Always returns "M" -> value 8 -> normalized (8-1)/19 = 7/19 ≈ 0.368
	if out.Value < 0.3 || out.Value > 0.45 {
		t.Errorf("score = %v, want ~0.368", out.Value)
	}
}

func TestRunPairwise_BestWins(t *testing.T) {
	// Extract just the trajectory text section, not the surrounding prompt.
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
	p := &scriptedProvider{
		letterFor: func(prompt string) (string, string) {
			aTraj := extractTraj(prompt, "**Trajectory A:**")
			bTraj := extractTraj(prompt, "**Trajectory B:**")
			score := func(text string) string {
				if strings.Contains(text, "FLAG_TOP") {
					return "A"
				}
				if strings.Contains(text, "FLAG_MID") {
					return "K"
				}
				return "T"
			}
			return score(aTraj), score(bTraj)
		},
	}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	out, err := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "test",
		Trajectories: []string{"FLAG_TOP solution", "FLAG_MID attempt", "FLAG_LOW broken"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, BiasMitigation: "single", MaxWorkers: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.BestIndex != 0 {
		t.Errorf("best_index = %d, want 0; result %+v", out.BestIndex, out)
	}
}

func TestRunPairwise_PreSkipIdentical(t *testing.T) {
	p := &scriptedProvider{
		letterFor: func(_ string) (string, string) { return "M", "M" },
	}
	r := &Runner{Provider: p, Cfg: RunnerConfig{Model: "m", BaseURL: "https://mock.invalid/v1"}}
	_, err := RunPairwise(context.Background(), r, PairwiseInput{
		Task:         "test",
		Trajectories: []string{"identical", "identical", "different"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:          PairwiseConfig{NReps: 1, BiasMitigation: "single", MaxWorkers: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	// With 3 trajectories where A==B (identical hashes), pair (0,1) is pre-skipped.
	// Pairs (0,2) and (1,2) require a single call each -> 2 calls total.
	got := atomic.LoadInt32(&p.calls)
	if got > 2 {
		t.Errorf("expected <=2 calls (identical pair pre-skipped), got %d", got)
	}
}
