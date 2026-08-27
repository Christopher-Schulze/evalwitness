package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

// Probe decides whether a route counts as a verifier at all. Its answer is
// cached and reused, so a wrong one is not a single bad run: it is every run
// afterwards, silently taking a path the route cannot serve. Nothing here was
// tested, including the persisted record that carries the answer forward.

type probeProvider struct {
	calls     []RequestEnvelope
	responses []ResponseRecord
	errs      []error
}

func (p *probeProvider) Name() string               { return "probe-target" }
func (p *probeProvider) Capabilities() Capabilities { return Capabilities{} }

func (p *probeProvider) Score(_ context.Context, req RequestEnvelope) (ResponseRecord, error) {
	i := len(p.calls)
	p.calls = append(p.calls, req)
	if i < len(p.errs) && p.errs[i] != nil {
		return ResponseRecord{}, p.errs[i]
	}
	if i < len(p.responses) {
		return p.responses[i], nil
	}
	return ResponseRecord{}, nil
}

func TestProbeRecordsWhatTheRouteActuallyReturned(t *testing.T) {
	dir := t.TempDir()
	p := &probeProvider{responses: []ResponseRecord{{
		RawText: "<score>A</score>", HasLogprobs: true, ObservedTopLogprobs: 20, Usage: TokenUsage{Cached: 40},
	}}}

	caps, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "the-model", "default")
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Logprobs || caps.TopLogprobsMax != 20 {
		t.Fatalf("caps = %+v, want logprobs with 20 alternatives", caps)
	}
	if !caps.PromptCache {
		t.Fatal("cached tokens in the response did not register as prompt caching")
	}
	// A probe asks for the full window on purpose: a route that returns fewer
	// alternatives than requested has to be recorded at what it gave, not at
	// what was asked for.
	if got := p.calls[0].TopLogprobs; got != 20 {
		t.Fatalf("probe requested %d alternatives, want 20", got)
	}
}

func TestProbeRecordsFewerAlternativesThanRequested(t *testing.T) {
	// The failure this catches is a route that advertises top_logprobs and caps
	// it below 20. Recording 20 would let it claim full-verifier eligibility.
	dir := t.TempDir()
	p := &probeProvider{responses: []ResponseRecord{{HasLogprobs: true, ObservedTopLogprobs: 5}}}

	caps, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "capped-model", "default")
	if err != nil {
		t.Fatal(err)
	}
	if caps.TopLogprobsMax != 5 {
		t.Fatalf("top logprobs max = %d, want the 5 the route actually returned", caps.TopLogprobsMax)
	}
}

func TestProbeRetriesWithoutLogprobsWhenTheRouteRejectsThem(t *testing.T) {
	// A 4xx specifically about logprobs means the route works and the feature
	// does not. Failing the probe outright would lose a usable judge route.
	dir := t.TempDir()
	p := &probeProvider{
		errs: []error{&ProviderError{Provider: "probe-target", Status: 400, Class: ClassCapabilityMissing,
			Body: "does not support return_logprob yet"}},
		responses: []ResponseRecord{{}, {RawText: "plain answer", HasLogprobs: true, ObservedTopLogprobs: 20}},
	}

	caps, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "no-logprob-model", "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.calls) != 2 {
		t.Fatalf("%d calls, want a retry without logprobs", len(p.calls))
	}
	if p.calls[1].TopLogprobs != 0 {
		t.Fatalf("retry asked for %d alternatives, want none", p.calls[1].TopLogprobs)
	}
	// The retry succeeded and even claimed logprobs; the probe must still record
	// false, because the first attempt is the one that answers the question.
	if caps.Logprobs || caps.TopLogprobsMax != 0 {
		t.Fatalf("caps = %+v, want logprobs false after a capability rejection", caps)
	}
}

func TestProbeFailsOnErrorsThatAreNotAboutCapability(t *testing.T) {
	dir := t.TempDir()
	p := &probeProvider{errs: []error{&ProviderError{Status: 401, Class: ClassAuthFailed, Body: "bad key"}}}

	if _, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "m", "default"); err == nil {
		t.Fatal("an authentication failure passed the probe")
	}
	if len(p.calls) != 1 {
		t.Fatalf("%d calls, want no retry for a non-capability error", len(p.calls))
	}
}

func TestProbeResultRoundTripsThroughTheCapabilityCache(t *testing.T) {
	dir := t.TempDir()
	p := &probeProvider{responses: []ResponseRecord{{HasLogprobs: true, ObservedTopLogprobs: 20}}}
	want, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "round/trip:model", "default")
	if err != nil {
		t.Fatal(err)
	}

	got, ok := LoadCachedCaps(dir, "probe-target", "round/trip:model")
	if !ok {
		t.Fatal("a freshly probed route was not readable from the capability cache")
	}
	if got.Logprobs != want.Logprobs || got.TopLogprobsMax != want.TopLogprobsMax {
		t.Fatalf("cached %+v != probed %+v", got, want)
	}
}

func TestCapabilityCacheIsKeyedByProviderAndModel(t *testing.T) {
	// Two models on one endpoint have different capabilities. Sharing a cache
	// entry would let one qualify the other.
	dir := t.TempDir()
	yes := &probeProvider{responses: []ResponseRecord{{HasLogprobs: true, ObservedTopLogprobs: 20}}}
	no := &probeProvider{responses: []ResponseRecord{{HasLogprobs: false}}}

	if _, err := Probe(context.Background(), yes, dir, "https://probe.invalid/v1", "model-with", "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := Probe(context.Background(), no, dir, "https://probe.invalid/v1", "model-without", "default"); err != nil {
		t.Fatal(err)
	}

	withCaps, ok := LoadCachedCaps(dir, "probe-target", "model-with")
	if !ok || !withCaps.Logprobs {
		t.Fatalf("model-with reads back %+v (found %t)", withCaps, ok)
	}
	withoutCaps, ok := LoadCachedCaps(dir, "probe-target", "model-without")
	if !ok || withoutCaps.Logprobs {
		t.Fatalf("model-without reads back %+v; it inherited the other model's answer", withoutCaps)
	}
}

func TestCapabilityCacheRejectsAStaleOrForeignRecord(t *testing.T) {
	dir := t.TempDir()
	p := &probeProvider{responses: []ResponseRecord{{HasLogprobs: true, ObservedTopLogprobs: 20}}}
	if _, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "m", "default"); err != nil {
		t.Fatal(err)
	}
	path := capsCachePath(dir, "probe-target", "m")

	patch := func(t *testing.T, mutate func(*probeRecord)) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var pr probeRecord
		if err := json.Unmarshal(raw, &pr); err != nil {
			t.Fatal(err)
		}
		mutate(&pr)
		out, _ := json.Marshal(pr)
		if err := os.WriteFile(path, out, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// A record from an older layout must not be trusted under the current one.
	patch(t, func(pr *probeRecord) { pr.SchemaVersion = probeSchemaVersion + 1 })
	if _, ok := LoadCachedCaps(dir, "probe-target", "m"); ok {
		t.Fatal("a record from another schema version was served")
	}

	// An endpoint's capabilities change without notice, so a stale answer has to
	// expire rather than qualify a route indefinitely.
	patch(t, func(pr *probeRecord) {
		pr.SchemaVersion = probeSchemaVersion
		pr.TS = time.Now().Add(-2 * probeMaxAge).Unix()
	})
	if _, ok := LoadCachedCaps(dir, "probe-target", "m"); ok {
		t.Fatal("a record older than the maximum age was served")
	}
}

func TestCapabilityCacheMissesRatherThanFailing(t *testing.T) {
	dir := t.TempDir()
	if _, ok := LoadCachedCaps(dir, "never-probed", "m"); ok {
		t.Fatal("an unprobed route reported cached capabilities")
	}

	p := &probeProvider{responses: []ResponseRecord{{HasLogprobs: true, ObservedTopLogprobs: 20}}}
	if _, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "m", "default"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(capsCachePath(dir, "probe-target", "m"), []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadCachedCaps(dir, "probe-target", "m"); ok {
		t.Fatal("a corrupt capability record was served")
	}
}

func TestCapabilityCacheUsesExplicitLegacyFallbackWithoutWritingIt(t *testing.T) {
	primaryDir := t.TempDir()
	legacyDir := t.TempDir()
	p := &probeProvider{responses: []ResponseRecord{{HasLogprobs: true, ObservedTopLogprobs: 20}}}
	if _, err := Probe(context.Background(), p, legacyDir, "https://probe.invalid/v1", "legacy-model", "default"); err != nil {
		t.Fatal(err)
	}

	caps, source, ok := LoadCachedCapsWithLegacy(primaryDir, legacyDir, "probe-target", "legacy-model")
	if !ok || !caps.Logprobs || source != CapabilityCacheLegacy {
		t.Fatalf("legacy caps = %+v source=%q found=%t", caps, source, ok)
	}
	if _, err := os.Stat(capsCachePath(primaryDir, "probe-target", "legacy-model")); !os.IsNotExist(err) {
		t.Fatalf("legacy read wrote into the canonical cache: %v", err)
	}

	canonical := &probeProvider{responses: []ResponseRecord{{HasLogprobs: false}}}
	if _, err := Probe(context.Background(), canonical, primaryDir, "https://probe.invalid/v1", "legacy-model", "default"); err != nil {
		t.Fatal(err)
	}
	caps, source, ok = LoadCachedCapsWithLegacy(primaryDir, legacyDir, "probe-target", "legacy-model")
	if !ok || caps.Logprobs || source != CapabilityCacheEvalWitness {
		t.Fatalf("canonical caps did not win: %+v source=%q found=%t", caps, source, ok)
	}
}

func TestProbeRedactsSecretsFromTheStoredExcerpt(t *testing.T) {
	// The record keeps a slice of the response so a human can see what the route
	// actually said. A key echoed back by an endpoint would otherwise be written
	// to disk in plain text.
	dir := t.TempDir()
	p := &probeProvider{responses: []ResponseRecord{{
		RawText:             "your key sk-proj-EXAMPLEKEYNOTREAL0000000000 was rejected",
		HasLogprobs:         true,
		ObservedTopLogprobs: 20,
	}}}
	if _, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "m", "default"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(capsCachePath(dir, "probe-target", "m"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sk-proj-EXAMPLEKEY") {
		t.Fatalf("a key-shaped string was written to the capability cache: %s", raw)
	}
	if !strings.Contains(string(raw), "REDACTED") {
		t.Fatalf("excerpt shows no redaction: %s", raw)
	}
}

func TestProbeExcerptIsBounded(t *testing.T) {
	// A model that answers with a whole trajectory must not put it in a cache
	// file that exists to hold a capability answer.
	dir := t.TempDir()
	p := &probeProvider{responses: []ResponseRecord{{
		RawText: strings.Repeat("x", 10_000), HasLogprobs: true, ObservedTopLogprobs: 20,
	}}}
	if _, err := Probe(context.Background(), p, dir, "https://probe.invalid/v1", "m", "default"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(capsCachePath(dir, "probe-target", "m"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 2048 {
		t.Fatalf("capability record is %d bytes; the excerpt is unbounded", info.Size())
	}
}

func TestCapsPathUsesHashedRouteNamespace(t *testing.T) {
	provider := "prov"
	model := "vendor/model:v1 beta"
	namespace, err := safety.NewRouteNamespace(provider, model)
	if err != nil {
		t.Fatal(err)
	}
	got := capsCachePath("/cache", provider, model)
	want := filepath.Join("/cache", namespace.Directory(), "capabilities.json")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if strings.Contains(got, provider) || strings.Contains(got, model) {
		t.Fatalf("path contains raw route identity: %q", got)
	}
}

func TestProviderErrorDescribesItselfCompletely(t *testing.T) {
	// The message is what a user sees when a route refuses. It has to carry the
	// provider, the status and the body, because the body is how a route that
	// passes a probe and fails real work identifies itself.
	pe := &ProviderError{Provider: "p", Status: 400, Body: "does not support return_logprob yet", Class: ClassCapabilityMissing}
	msg := pe.Error()
	for _, want := range []string{"p", "400", "return_logprob"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q omits %q", msg, want)
		}
	}

	// An oversized body is truncated rather than printed whole; some endpoints
	// answer an error with an entire HTML page.
	long := &ProviderError{Provider: "p", Status: 500, Body: strings.Repeat("x", 5000)}
	if len(long.Error()) > 600 {
		t.Fatalf("error message is %d characters; the body is not bounded", len(long.Error()))
	}
}
