package replay

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

// Replay is how this repository's reproducibility claim is kept: a fixture
// captured once lets anyone re-run the verifier path with no key and no network
// and get the identical result. That only holds if capture and replay agree on
// the key, and if a miss is an error rather than a quiet empty response - an
// empty response would score as a 0.5 fallback and reproduce nothing while
// appearing to succeed.

type scriptedInner struct {
	mu    sync.Mutex
	calls int
	reply func(req provider.RequestEnvelope) (provider.ResponseRecord, error)
}

func (s *scriptedInner) Name() string { return "inner-provider" }
func (s *scriptedInner) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20, Streaming: true}
}

func (s *scriptedInner) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.reply != nil {
		return s.reply(req)
	}
	return exactResponse(req)
}

func exactRequest(t *testing.T, providerID, model, prompt string) provider.RequestEnvelope {
	t.Helper()
	request, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID:      providerID,
		BaseURL:         "https://mock.invalid/v1",
		RequestedModel:  model,
		Messages:        []provider.Message{{Role: "user", Content: prompt}},
		Temperature:     1,
		MaxOutputTokens: 128,
		Logprobs:        true,
		TopLogprobs:     20,
		ScoreTags:       []string{"<score_A>"},
		Stream:          true,
		Lineage:         provider.RequestLineage{CriterionID: "criterion", SamplingSlot: "criterion@r0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func exactResponse(request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	rawText := "<score_A>B</score_A>"
	distributions := map[string]map[string]float64{"<score_A>": {"B": 0.9, "C": 0.1}}
	return provider.FinalizeResponse(request, provider.ResponseRecord{
		RawText:              rawText,
		NormalizedBody:       []byte(rawText),
		Distributions:        distributions,
		HasLogprobs:          true,
		ObservedTopLogprobs:  20,
		Usage:                provider.TokenUsage{Input: 120, Output: 5},
		ServedModel:          "served-name",
		OrderedTokenEvidence: replayTokenEvidence(request.ScoreTags, rawText, distributions),
	})
}

func TestCaptureThenReplayReproducesTheResponseWithoutTheProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	inner := &scriptedInner{}
	cap, err := WrapCapture(inner, "model-under-test", path, false)
	if err != nil {
		t.Fatal(err)
	}

	req := exactRequest(t, inner.Name(), "model-under-test", "score this trajectory")
	live, err := cap.Score(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := cap.Close(); err != nil {
		t.Fatal(err)
	}

	rp, err := LoadReplay(path, inner.Name(), "model-under-test", inner.Capabilities())
	if err != nil {
		t.Fatal(err)
	}
	source, available := rp.ExactReplaySource()
	if !available || source.Validate() != nil || source.ProviderID != inner.Name() || source.RequestedModel != "model-under-test" ||
		source.Records != 1 || source.Bytes < 1 || source.CaptureSHA256 == "" || source.RecordSetDigest == "" {
		t.Fatalf("exact replay source evidence is incomplete: available=%t source=%+v", available, source)
	}
	captureBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if source.CaptureSHA256 != protocol.DigestBytes(captureBytes) || source.Bytes != int64(len(captureBytes)) {
		t.Fatalf("exact replay source does not bind capture bytes: source=%+v", source)
	}
	replayed, err := rp.Score(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if replayed.RawText != live.RawText {
		t.Fatalf("raw text %q != captured %q", replayed.RawText, live.RawText)
	}
	if replayed.Usage != live.Usage {
		t.Fatalf("usage did not survive the round trip: %+v vs %+v", replayed, live)
	}
	if replayed.ServedModel != live.ServedModel {
		t.Fatalf("served model %q lost; a replayed artifact could not say which model produced it", replayed.ServedModel)
	}
	if !replayed.HasLogprobs || len(replayed.Distributions["<score_A>"]) != 2 {
		t.Fatalf("distribution did not survive: %+v", replayed.Distributions)
	}
	if inner.calls != 1 {
		t.Fatalf("inner provider called %d times; replay reached the network", inner.calls)
	}
}

func TestCaptureRejectsInvalidScoreEvidenceBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	inner := &scriptedInner{reply: func(request provider.RequestEnvelope) (provider.ResponseRecord, error) {
		rawText := "<score_A>B</score_A>"
		return provider.FinalizeResponse(request, provider.ResponseRecord{
			RawText:             rawText,
			NormalizedBody:      []byte(rawText),
			HasLogprobs:         true,
			ObservedTopLogprobs: 1,
			OrderedTokenEvidence: []provider.TokenEvidence{
				{Position: 0, Token: "<score_A>", Logprob: "0"},
				{Position: 1, Token: "B", Logprob: "0", TopAlternatives: []provider.TokenAlternative{{Token: "B", Logprob: "0"}}},
				{Position: 2, Token: "</score_A>", Logprob: "0"},
			},
		})
	}}
	capture, err := WrapCapture(inner, "model-under-test", path, false)
	if err != nil {
		t.Fatal(err)
	}
	request := exactRequest(t, inner.Name(), "model-under-test", "invalid evidence")
	if _, err := capture.Score(context.Background(), request); err == nil || !strings.Contains(err.Error(), "score evidence") {
		t.Fatalf("invalid capture error = %v", err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(path); err != nil || len(raw) != 0 {
		t.Fatalf("invalid score evidence was captured: bytes=%d err=%v", len(raw), err)
	}
}

func TestCaptureSecuresNewParentsAndExistingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "capture.jsonl")
	provider, err := WrapCapture(&scriptedInner{}, "model", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if parentInfo.Mode().Perm() != safety.SensitiveDirectoryMode {
		t.Fatalf("parent mode = %o", parentInfo.Mode().Perm())
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	provider, err = WrapCapture(&scriptedInner{}, "model", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != safety.SensitiveFileMode {
		t.Fatalf("file mode = %o", fileInfo.Mode().Perm())
	}
}

func TestReplayIdentityAndCapabilitiesAreTheOnesGiven(t *testing.T) {
	// A replayed run has to report the identity it is replaying, not the
	// original provider's, or an artifact would claim a route it never used.
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	caps := provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
	rp, err := LoadReplay(path, "replay-name", "replay-model", caps)
	if err != nil {
		t.Fatal(err)
	}
	if rp.Name() != "replay-name" {
		t.Fatalf("name = %q", rp.Name())
	}
	if rp.Capabilities() != caps {
		t.Fatalf("capabilities = %+v, want the ones supplied", rp.Capabilities())
	}
}

func TestReplayMissIsAnErrorRatherThanAnEmptyResponse(t *testing.T) {
	// The failure that matters. An empty response would extract nothing, fall
	// back to 0.5, and produce a run that looks reproduced and is not.
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	rp, err := LoadReplay(path, "r", "m", provider.Capabilities{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = rp.Score(context.Background(), exactRequest(t, "r", "m", "never captured"))
	if err == nil {
		t.Fatal("a replay miss returned no error")
	}
	if !strings.Contains(err.Error(), "replay miss") {
		t.Fatalf("error %q does not identify itself as a replay miss", err)
	}
}

func TestPromptChangeMissesRatherThanServingTheWrongEntry(t *testing.T) {
	// The fixture is keyed on the prompt. A one-character change is a different
	// call and must not be answered from the old one.
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	cap, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	request := exactRequest(t, cap.Name(), "m", "prompt one")
	if _, err := cap.Score(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := cap.Close(); err != nil {
		t.Fatal(err)
	}

	rp, err := LoadReplay(path, cap.Name(), "m", provider.Capabilities{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rp.Score(context.Background(), request); err != nil {
		t.Fatalf("the captured prompt did not replay: %v", err)
	}
	if _, err := rp.Score(context.Background(), exactRequest(t, cap.Name(), "m", "prompt onx")); err == nil {
		t.Fatal("a different prompt was served from the fixture")
	}
}

func TestCaptureDoesNotRecordFailedCalls(t *testing.T) {
	// A fixture containing an error's empty response would replay as a
	// successful empty answer, which is worse than having no fixture.
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	inner := &scriptedInner{reply: func(provider.RequestEnvelope) (provider.ResponseRecord, error) {
		return provider.ResponseRecord{}, errors.New("upstream refused")
	}}
	cap, err := WrapCapture(inner, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cap.Score(context.Background(), exactRequest(t, cap.Name(), "m", "p")); err == nil {
		t.Fatal("the capturing wrapper swallowed the inner error")
	}
	if err := cap.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(raw))) != 0 {
		t.Fatalf("fixture recorded a failed call: %q", raw)
	}
}

func TestCaptureAppendsByDefaultAndTruncatesOnOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")

	first, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Score(context.Background(), exactRequest(t, first.Name(), "m", "one")); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	appended, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appended.Score(context.Background(), exactRequest(t, appended.Name(), "m", "two")); err != nil {
		t.Fatal(err)
	}
	if err := appended.Close(); err != nil {
		t.Fatal(err)
	}

	if got := countLines(t, path); got != 2 {
		t.Fatalf("%d entries after two appending runs, want 2", got)
	}

	overwritten, err := WrapCapture(&scriptedInner{}, "m", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := overwritten.Score(context.Background(), exactRequest(t, overwritten.Name(), "m", "three")); err != nil {
		t.Fatal(err)
	}
	if err := overwritten.Close(); err != nil {
		t.Fatal(err)
	}

	if got := countLines(t, path); got != 1 {
		t.Fatalf("%d entries after an overwriting run, want 1", got)
	}
}

func TestCaptureDelegatesIdentityToTheInnerProvider(t *testing.T) {
	// A captured run is still a live run against the real route, so its recorded
	// identity has to be the inner provider's rather than the wrapper's.
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	inner := &scriptedInner{}
	cap, err := WrapCapture(inner, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			if err := cap.Close(); err != nil {
				t.Errorf("close capture: %v", err)
			}
		}
	}()
	if cap.Name() != inner.Name() {
		t.Fatalf("name = %q, want the inner provider's", cap.Name())
	}
	if cap.Capabilities() != inner.Capabilities() {
		t.Fatal("capabilities do not match the inner provider's")
	}

	if _, err := cap.Score(context.Background(), exactRequest(t, cap.Name(), "m", "p")); err != nil {
		t.Fatal(err)
	}
	if err := cap.Close(); err != nil {
		closed = true
		t.Fatal(err)
	}
	closed = true
	var e fixtureEntry
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &e); err != nil {
		t.Fatal(err)
	}
	if e.Request.ProviderID != inner.Name() || e.Request.RequestedModel != "m" {
		t.Fatalf("entry records %s/%s, want the inner provider and the given model", e.Request.ProviderID, e.Request.RequestedModel)
	}
}

func TestWrapCaptureRequiresAPath(t *testing.T) {
	if _, err := WrapCapture(&scriptedInner{}, "m", "", false); err == nil {
		t.Fatal("capture with no path was accepted")
	}
}

func TestLoadReplayRejectsAMalformedFixture(t *testing.T) {
	// Silently skipping a bad line would turn a corrupt fixture into a set of
	// replay misses, which reads as a coverage problem rather than a broken file.
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	if err := os.WriteFile(path, []byte("{not json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(path, "r", "m", provider.Capabilities{}); err == nil {
		t.Fatal("a malformed fixture loaded without error")
	}
}

func TestLoadReplayReportsAMissingFixture(t *testing.T) {
	if _, err := LoadReplay(filepath.Join(t.TempDir(), "absent.jsonl"), "r", "m", provider.Capabilities{}); err == nil {
		t.Fatal("a missing fixture loaded without error")
	}
}

func TestConcurrentCaptureWritesCompleteLines(t *testing.T) {
	// Pair evaluation is concurrent, so capture is too. A torn line would make
	// the fixture unloadable and only show up on the next replay.
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	cap, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	requests := make([]provider.RequestEnvelope, 16)
	for i := range requests {
		requests[i] = exactRequest(t, cap.Name(), "m", strings.Repeat("p", i+1))
	}
	var wg sync.WaitGroup
	for i, request := range requests {
		wg.Go(func() {
			if _, err := cap.Score(context.Background(), request); err != nil {
				t.Errorf("capture %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	if err := cap.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(path, cap.Name(), "m", provider.Capabilities{}); err != nil {
		t.Fatalf("concurrently captured fixture does not load: %v", err)
	}
	if got := countLines(t, path); got != 16 {
		t.Fatalf("%d entries, want 16", got)
	}
}

func TestCapturePublishesOnlyAfterCheckedClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), exactRequest(t, capture.Name(), "m", "pending")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target became visible before checked close: %v", err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("published fixture: %v", err)
	}
}

func TestCaptureRefusesToOverwriteConcurrentTargetChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	original := []byte("concurrent owner\n")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), exactRequest(t, capture.Name(), "m", "pending")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	err = capture.Close()
	if err == nil || !strings.Contains(err.Error(), "changed concurrently") {
		t.Fatalf("close error = %v, want concurrent-change rejection", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("concurrent target was overwritten: %q", got)
	}
	if _, statErr := os.Stat(capture.candidatePath); statErr != nil {
		t.Fatalf("failed candidate was not retained: %v", statErr)
	}
}

func TestLoadReplayRejectsRouteNarrativeMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), exactRequest(t, capture.Name(), "m", "route")); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(path, "different-provider", "m", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "route mismatch") {
		t.Fatalf("provider mismatch error = %v", err)
	}
	if _, err := LoadReplay(path, capture.Name(), "different-model", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "route mismatch") {
		t.Fatalf("model mismatch error = %v", err)
	}
}

func TestLoadReplayRejectsBodyCorruptionAndDuplicateIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	request := exactRequest(t, capture.Name(), "m", "integrity")
	if _, err := capture.Score(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	duplicatePath := filepath.Join(t.TempDir(), "duplicate.jsonl")
	if err := os.WriteFile(duplicatePath, append(append([]byte(nil), raw...), raw...), safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(duplicatePath, capture.Name(), "m", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "duplicate request") {
		t.Fatalf("duplicate error = %v", err)
	}

	var entry fixtureEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	entry.Response.NormalizedBody = []byte("tampered")
	tampered, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	corruptPath := filepath.Join(t.TempDir(), "corrupt.jsonl")
	if err := os.WriteFile(corruptPath, append(tampered, '\n'), safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(corruptPath, capture.Name(), "m", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "body checksum mismatch") {
		t.Fatalf("corruption error = %v", err)
	}
}

func TestLoadReplayRejectsResponseMetadataCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), exactRequest(t, capture.Name(), "m", "metadata")); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry fixtureEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	entry.Response.Usage.Input++
	tampered, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(tampered, '\n'), safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(path, capture.Name(), "m", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "evidence checksum mismatch") {
		t.Fatalf("metadata corruption error = %v", err)
	}
}

func TestLoadReplayRejectsScoreEvidenceCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), exactRequest(t, capture.Name(), "m", "score evidence")); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry fixtureEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	for tag, evidence := range entry.ScoreEvidence {
		evidence.ValidScoreMass /= 2
		entry.ScoreEvidence[tag] = evidence
		break
	}
	tampered, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(tampered, '\n'), safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(path, capture.Name(), "m", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "score evidence does not match") {
		t.Fatalf("score-evidence corruption error = %v", err)
	}
}

func TestLoadReplayRejectsRequestLineageCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Score(context.Background(), exactRequest(t, capture.Name(), "m", "lineage")); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry fixtureEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	entry.Request.Lineage.Entrypoint = "tampered"
	tampered, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(tampered, '\n'), safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(path, capture.Name(), "m", provider.Capabilities{}); err == nil || !strings.Contains(err.Error(), "record checksum mismatch") {
		t.Fatalf("lineage corruption error = %v", err)
	}
}

func TestReplayReturnsTypedExactAndMissStatuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.jsonl")
	capture, err := WrapCapture(&scriptedInner{}, "m", path, false)
	if err != nil {
		t.Fatal(err)
	}
	request := exactRequest(t, capture.Name(), "m", "status")
	if _, err := capture.Score(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	replayProvider, err := LoadReplay(path, capture.Name(), "m", provider.Capabilities{})
	if err != nil {
		t.Fatal(err)
	}
	response, err := replayProvider.Score(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.ReplayStatus != provider.ReplayStatusExact || response.ReplayReason == "" {
		t.Fatalf("replay provenance = %q/%q", response.ReplayStatus, response.ReplayReason)
	}
	_, err = replayProvider.Score(context.Background(), exactRequest(t, capture.Name(), "m", "missing"))
	var lookupError *provider.ReplayLookupError
	if !errors.As(err, &lookupError) || lookupError.Status != provider.ReplayStatusMiss {
		t.Fatalf("miss error = %T %v", err, err)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}
