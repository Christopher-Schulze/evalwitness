package replay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const goldenFixturePath = "../../scripts/tests/golden-delta-replay.jsonl"

type goldenProvider struct{}

func (*goldenProvider) Name() string { return "replay" }
func (*goldenProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20, Streaming: true}
}
func (*goldenProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	rawText := "<critique_A>Trajectory A writes addition and verifies the required output.</critique_A>" +
		"<critique_B>Trajectory B implements subtraction and observes the wrong output.</critique_B>" +
		"<score_A>A</score_A><score_B>T</score_B>"
	distributions := map[string]map[string]float64{
		"<score_A>": {"A": 0.95, "B": 0.05},
		"<score_B>": {"S": 0.1, "T": 0.9},
	}
	return provider.FinalizeResponse(request, provider.ResponseRecord{
		ServedModel:          "golden-delta",
		ProviderRequestID:    "golden-delta-1",
		ReceivedAt:           1700000000000,
		Usage:                provider.TokenUsage{Input: 900, Output: 32},
		NormalizedBody:       []byte(rawText),
		RawText:              rawText,
		HasLogprobs:          true,
		ObservedTopLogprobs:  20,
		Distributions:        distributions,
		OrderedTokenEvidence: replayTokenEvidence(request.ScoreTags, rawText, distributions),
	})
}

func TestGoldenDeltaFixtureMatchesCanonicalRuntimeRequest(t *testing.T) {
	want, err := os.ReadFile(goldenFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(t.TempDir(), "golden.jsonl")
	capture, err := WrapCapture(&goldenProvider{}, "golden-delta", fixturePath, false)
	if err != nil {
		t.Fatal(err)
	}
	task := readGoldenInput(t, "../../scripts/tests/sample-task.txt")
	trajectoryA := readGoldenInput(t, "../../scripts/tests/sample-traj-a.txt")
	trajectoryB := readGoldenInput(t, "../../scripts/tests/sample-traj-b.txt")
	runner := &mode.Runner{
		Provider: capture,
		Cfg: mode.RunnerConfig{
			Model:                "golden-delta",
			BaseURL:              "https://replay.invalid/v1",
			ThinkingMode:         "disabled",
			Stream:               true,
			Entrypoint:           "cli.verify",
			PreprocessBudget:     32_000,
			CritiqueThenScore:    true,
			MultiCriterionBundle: true,
			Temperature:          1,
			TopLogprobs:          20,
		},
	}
	_, err = mode.RunDelta(context.Background(), runner, mode.DeltaInput{
		Task:        task,
		TrajectoryA: trajectoryA,
		TrajectoryB: trajectoryB,
		Criteria:    []verifier.Criterion{verifier.BuiltinCriteria["generic"]},
		Cfg:         mode.DeltaConfig{NReps: 1, BiasMitigation: "single"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("EVALWITNESS_WRITE_GOLDEN_CANDIDATE") == "1" {
		candidate := goldenFixturePath + ".candidate"
		if _, statErr := os.Stat(candidate); !os.IsNotExist(statErr) {
			t.Fatalf("refusing to overwrite golden candidate %s", candidate)
		}
		if err := os.WriteFile(candidate, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var wantEntry, gotEntry fixtureEntry
	if err := json.Unmarshal(want, &wantEntry); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &gotEntry); err != nil {
		t.Fatal(err)
	}
	if wantEntry.CaptureSchemaVersion != gotEntry.CaptureSchemaVersion ||
		!reflect.DeepEqual(wantEntry.Request, gotEntry.Request) ||
		!reflect.DeepEqual(wantEntry.Response, gotEntry.Response) ||
		!replayEvidenceEquivalent(wantEntry.ScoreEvidence, gotEntry.ScoreEvidence) {
		t.Fatal("golden fixture differs from the canonical runtime request or response evidence")
	}
	for _, entry := range []fixtureEntry{wantEntry, gotEntry} {
		if _, err := validateFixtureEntry(entry, "replay", "golden-delta"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReplayEvidenceEquivalentRejectsMaterialNumericDrift(t *testing.T) {
	raw, err := os.ReadFile(goldenFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var base fixtureEntry
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		delta float64
		want  bool
	}{
		{name: "exact", want: true},
		{name: "platform rounding", delta: 1e-15, want: true},
		{name: "material drift", delta: 1e-6, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var candidate fixtureEntry
			if err := json.Unmarshal(raw, &candidate); err != nil {
				t.Fatal(err)
			}
			item := candidate.ScoreEvidence["<score_A>"]
			item.Support[1].Probability += test.delta
			candidate.ScoreEvidence["<score_A>"] = item
			if got := replayEvidenceEquivalent(base.ScoreEvidence, candidate.ScoreEvidence); got != test.want {
				t.Fatalf("equivalent = %v, want %v", got, test.want)
			}
		})
	}
}

func readGoldenInput(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
