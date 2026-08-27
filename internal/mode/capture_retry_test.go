package mode

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type recoveringCaptureProvider struct {
	calls    atomic.Int32
	closures atomic.Int32
}

func (p *recoveringCaptureProvider) Name() string { return "mock" }

func (p *recoveringCaptureProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}

func (p *recoveringCaptureProvider) CloseIdleConnections() {
	p.closures.Add(1)
}

func (p *recoveringCaptureProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	if p.calls.Add(1) == 1 {
		rawText := "<score_A>A</score_A>\n<score_B>T</score_B>"
		return provider.FinalizeResponse(request, provider.ResponseRecord{
			RawText: rawText, NormalizedBody: []byte(rawText), ProviderRequestID: "missing-logprobs",
		})
	}
	return finalizeTestResponse(request, provider.ResponseRecord{
		RawText: "<score_A>A</score_A>\n<score_B>T</score_B>",
		Distributions: map[string]map[string]float64{
			"<score_A>": {"A": 0.9},
			"<score_B>": {"T": 0.9},
		},
	})
}

func TestCaptureDefersMissingLogprobsToBoundedExtractionRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jsonl")
	inner := &recoveringCaptureProvider{}
	capture, err := replay.WrapCapture(inner, "model", path, false)
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{
		Provider: capture,
		Cfg: RunnerConfig{
			Model: "model", BaseURL: "https://mock.invalid/v1", RequireLogprobs: true,
		},
	}

	criteria := []verifier.Criterion{verifier.BuiltinCriteria["generic"]}
	if _, _, err := runner.ScorePair(context.Background(), "task", "left", "right", criteria, 0); err != nil {
		t.Fatalf("capture retry failed: %v", err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	if got := inner.calls.Load(); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
	if got := inner.closures.Load(); got != 1 {
		t.Fatalf("connection closures = %d, want 1", got)
	}
	inspection, err := replay.InspectCaptureFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Entries != 1 {
		t.Fatalf("captured entries = %d, want only the valid retry", inspection.Entries)
	}
}
