package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func TestKeyHashStable(t *testing.T) {
	a := baseRequest()
	b := baseRequest()
	if requestHashForTest(t, a) != requestHashForTest(t, b) {
		t.Errorf("equal keys hashed differently")
	}
	changed := baseRequest()
	changed.Messages = []provider.Message{{Role: "user", Content: "y"}}
	if requestHashForTest(t, a) == requestHashForTest(t, changed) {
		t.Errorf("different prompts hashed identically")
	}
}

func TestCacheRoundtrip(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	if _, ok := c.Get(request); ok {
		t.Fatalf("expected miss before set")
	}
	response := responseForTest(t, request, "raw")
	response.Usage = provider.TokenUsage{Input: 100, Output: 5}
	response, err := provider.FinalizeResponse(request, response)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set(request, response); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok := c.Get(request)
	if !ok {
		t.Fatalf("expected hit after set")
	}
	if got.Response.RawText != "raw" || got.Response.Usage.Input != 100 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("schema version not set: %d", got.SchemaVersion)
	}
	if got.CreatedAt == 0 {
		t.Error("cache creation time was not recorded")
	}
	hash := requestHashForTest(t, request)
	namespace, err := safety.NewRouteNamespace(request.ProviderID, request.RequestedModel)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, namespace.Directory(), "responses", hash[:2], hash+".json")
	if c.requestToPath(request) != expected {
		t.Errorf("path = %s, want %s", c.requestToPath(request), expected)
	}
}

func TestCacheDisabled(t *testing.T) {
	c := New(t.TempDir(), false)
	request := baseRequest()
	if c.Enabled() {
		t.Fatalf("disabled cache reports enabled")
	}
	if err := c.Set(request, provider.ResponseRecord{}); err != nil {
		t.Fatalf("disabled set returned error: %v", err)
	}
	if _, ok := c.Get(request); ok {
		t.Fatalf("disabled cache returned hit")
	}
}

func TestLegacyImportIsReadOnlyAndRequestExact(t *testing.T) {
	primaryDir := t.TempDir()
	legacyDir := t.TempDir()
	request := baseRequest()
	writeLegacyEntryForTest(t, legacyDir, request, LegacyEntry{RawText: "legacy"})

	combined := NewWithLegacyImport(primaryDir, legacyDir, true)
	if _, ok := combined.Get(request); ok {
		t.Fatal("legacy cache entry was silently upgraded to an exact hit")
	}
	got, source, ok := combined.GetLegacy(request)
	if !ok || got.RawText != "legacy" || source != SourceLegacy {
		t.Fatalf("legacy read = %+v, found %t", got, ok)
	}
	changed := request
	changed.Messages = []provider.Message{{Role: "user", Content: "different request"}}
	if _, _, ok := combined.GetLegacy(changed); ok {
		t.Fatal("legacy import served an entry for a different request fingerprint")
	}

	if err := combined.Set(request, responseForTest(t, request, "canonical")); err != nil {
		t.Fatal(err)
	}
	exact, ok := combined.Get(request)
	if !ok || exact.Response.RawText != "canonical" || exact.SourceNamespace != SourceEvalWitness {
		t.Fatalf("primary read = %+v, found %t", exact, ok)
	}
	if legacyGot, ok := readLegacyEntry(legacyDir, request); !ok || legacyGot.RawText != "legacy" {
		t.Fatalf("legacy writer was mutated: %+v, found %t", legacyGot, ok)
	}
}

func writeLegacyEntryForTest(t *testing.T, dir string, request provider.RequestEnvelope, entry LegacyEntry) {
	t.Helper()
	entry.SchemaVersion = 1
	path, ok := legacyRequestToPathIn(dir, request)
	if !ok {
		t.Fatalf("test request has no valid legacy path: %+v", request)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
