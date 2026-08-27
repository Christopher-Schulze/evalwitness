package cache

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

// The cache decides which stored answer is served for which call. A key that
// collides serves one model's response under another's identity, and every
// number downstream is then wrong without anything failing. This file pins the
// key's isolation properties, because a defect here is invisible by
// construction: the run completes, the scores look ordinary, and they belong to
// something else.

func baseRequest() provider.RequestEnvelope {
	request, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID:      "opencode-go-cn",
		BaseURL:         "https://opencode.ai/zen/go/v1",
		RequestedModel:  "deepseek-v4-flash",
		Messages:        []provider.Message{{Role: "user", Content: "score this"}},
		Temperature:     1,
		MaxOutputTokens: 4096,
		Logprobs:        true,
		TopLogprobs:     20,
		ResponseFormat:  provider.ResponseFormatText,
		Lineage:         provider.RequestLineage{CriterionID: "generic", SamplingSlot: "generic", Entrypoint: "test"},
	})
	if err != nil {
		panic(err)
	}
	return request
}

func requestHashForTest(t *testing.T, request provider.RequestEnvelope) string {
	t.Helper()
	hash, err := requestStorageHash(request)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func responseForTest(t *testing.T, request provider.RequestEnvelope, rawText string) provider.ResponseRecord {
	t.Helper()
	response, err := provider.FinalizeResponse(request, provider.ResponseRecord{
		RawText:        rawText,
		NormalizedBody: []byte(rawText),
		Status:         provider.ResponseStatusComplete,
	})
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func scoreResponseForTest(t *testing.T, request provider.RequestEnvelope) provider.ResponseRecord {
	t.Helper()
	alternatives := make([]provider.TokenAlternative, 0, verifier.MinimumVerifierTopK)
	alternatives = append(alternatives, provider.TokenAlternative{Token: "A", Logprob: strconv.FormatFloat(math.Log(0.9), 'g', -1, 64)})
	for index := 1; index < verifier.MinimumVerifierTopK; index++ {
		alternatives = append(alternatives, provider.TokenAlternative{Token: "#" + strconv.Itoa(index), Logprob: strconv.FormatFloat(math.Log(0.1/19), 'g', -1, 64)})
	}
	rawText := "<score>A</score>"
	response, err := provider.FinalizeResponse(request, provider.ResponseRecord{
		RawText:             rawText,
		NormalizedBody:      []byte(rawText),
		HasLogprobs:         true,
		ObservedTopLogprobs: 20,
		OrderedTokenEvidence: []provider.TokenEvidence{
			{Position: 0, Token: "<score>", Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			{Position: 1, Token: "A", Logprob: alternatives[0].Logprob, TopAlternatives: alternatives},
			{Position: 2, Token: "</score>", Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestEveryKeyFieldChangesTheHash(t *testing.T) {
	// The storage hash is the semantic fingerprint plus the sampling slot. Each
	// variant differs in one of those dimensions and must remain isolated.
	base := baseRequest()
	providerVariant := base
	providerVariant.ProviderID = "other"
	modelVariant := base
	modelVariant.RequestedModel = "other"
	promptVariant := base
	promptVariant.Messages = []provider.Message{{Role: "user", Content: "other"}}
	topLogprobsVariant := base
	topLogprobsVariant.TopLogprobs = 10
	samplingVariant := base
	samplingVariant.Lineage.SamplingSlot = "generic@r1"
	variants := map[string]provider.RequestEnvelope{
		"provider":      providerVariant,
		"model":         modelVariant,
		"prompt":        promptVariant,
		"top_logprobs":  topLogprobsVariant,
		"sampling_slot": samplingVariant,
	}
	baseHash := requestHashForTest(t, base)
	seen := map[string]string{baseHash: "base"}
	for name, request := range variants {
		h := requestHashForTest(t, request)
		if h == baseHash {
			t.Fatalf("changing %s did not change the hash; that field is not part of the key", name)
		}
		if prev, dup := seen[h]; dup {
			t.Fatalf("%s and %s hash identically", name, prev)
		}
		seen[h] = name
	}
}

func TestTopLogprobsSeparatesVerifierFromJudgeEntries(t *testing.T) {
	// Judge mode requests no logprobs and keys on TopLogprobs 0. Sharing a key
	// with the verifier would let a judge run serve a distribution-backed answer
	// and report itself as judge, which is the comparison this repository is
	// built on.
	verifierRequest := baseRequest()
	judgeRequest := baseRequest()
	judgeRequest.Logprobs = false
	judgeRequest.TopLogprobs = 0
	if requestHashForTest(t, verifierRequest) == requestHashForTest(t, judgeRequest) {
		t.Fatal("verifier and judge entries share a cache key")
	}
}

func TestFieldBoundariesCannotBeShiftedIntoACollision(t *testing.T) {
	base := baseRequest()
	one := base
	one.ProviderID, one.RequestedModel = "ab", "c"
	two := base
	two.ProviderID, two.RequestedModel = "a", "bc"
	three := base
	three.Messages = []provider.Message{{Role: "user", Content: "xy"}}
	three.Lineage.SamplingSlot = "generic"
	four := base
	four.Messages = []provider.Message{{Role: "user", Content: "y"}}
	four.Lineage.SamplingSlot = "genericx"
	pairs := [][2]provider.RequestEnvelope{{one, two}, {three, four}}
	for i, pair := range pairs {
		if requestHashForTest(t, pair[0]) == requestHashForTest(t, pair[1]) {
			t.Fatalf("pair %d collides: %+v and %+v share a key", i, pair[0], pair[1])
		}
	}
}

func TestHashIsStableAcrossCalls(t *testing.T) {
	// The hash is a filename. An unstable one would miss every entry it wrote.
	request := baseRequest()
	first := requestHashForTest(t, request)
	for range 100 {
		if requestHashForTest(t, request) != first {
			t.Fatal("hash is not deterministic")
		}
	}
}

func TestEntriesAreScopedToHashedRouteDirectories(t *testing.T) {
	// Provider-scoped paths are why a mid-run route switch cannot serve another
	// route's answers. The property is load-bearing and easy to lose in a
	// refactor of keyToPath.
	dir := t.TempDir()
	c := New(dir, true)
	a := baseRequest()
	b := baseRequest()
	b.ProviderID = "second-provider"

	if err := c.Set(a, responseForTest(t, a, "from A")); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(b, responseForTest(t, b, "from B")); err != nil {
		t.Fatal(err)
	}

	for _, request := range []provider.RequestEnvelope{a, b} {
		namespace, err := safety.NewRouteNamespace(request.ProviderID, request.RequestedModel)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, namespace.Directory())); err != nil {
			t.Fatalf("no directory for route %q/%q: %v", request.ProviderID, request.RequestedModel, err)
		}
		if _, err := os.Stat(filepath.Join(dir, request.ProviderID)); !os.IsNotExist(err) {
			t.Fatalf("raw provider path exists for %q: %v", request.ProviderID, err)
		}
	}
	gotA, ok := c.Get(a)
	if !ok || gotA.Response.RawText != "from A" {
		t.Fatalf("provider A read back %q (found %t)", gotA.Response.RawText, ok)
	}
	gotB, _ := c.Get(b)
	if gotB.Response.RawText != "from B" {
		t.Fatalf("provider B read %q; it was served another provider's entry", gotB.Response.RawText)
	}
}

func TestSchemaVersionMismatchReadsAsAMiss(t *testing.T) {
	// An entry written by an older layout must not be served under the current
	// one. Treating it as a miss re-fetches; trusting it would mix formats
	// silently.
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	if err := c.Set(request, responseForTest(t, request, "current")); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get(request); !ok {
		t.Fatal("a freshly written entry was not readable")
	}

	path := c.requestToPath(request)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e.SchemaVersion != SchemaVersion {
		t.Fatalf("Set stored schema version %d, want %d", e.SchemaVersion, SchemaVersion)
	}
	e.SchemaVersion = SchemaVersion + 1
	patched, _ := json.Marshal(e)
	if err := os.WriteFile(path, patched, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get(request); ok {
		t.Fatal("an entry from a different schema version was served")
	}
}

func TestScoreEvidenceCorruptionReadsAsAMiss(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	request.ScoreTags = []string{"<score>"}
	if err := c.Set(request, scoreResponseForTest(t, request)); err != nil {
		t.Fatal(err)
	}
	path := c.requestToPath(request)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	evidence := entry.ScoreEvidence["<score>"]
	if !evidence.Extracted || evidence.ConditionalExpectedScore == nil {
		t.Fatalf("stored score evidence = %+v", evidence)
	}
	tampered := 0.123
	evidence.ConditionalExpectedScore = &tampered
	entry.ScoreEvidence["<score>"] = evidence
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, safety.SensitiveFileMode); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get(request); ok {
		t.Fatal("cache served score evidence that no longer matches the captured token stream")
	}
}

func TestSetRejectsInvalidScoreEvidence(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	request.ScoreTags = []string{"<score>"}
	response := responseForTest(t, request, "<score>A</score>")
	if err := c.Set(request, response); err == nil || !strings.Contains(err.Error(), "score evidence") {
		t.Fatalf("invalid score evidence write error = %v", err)
	}
	if _, err := os.Stat(c.requestToPath(request)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid score evidence was published: %v", err)
	}
}

func TestCorruptEntryReadsAsAMissRatherThanFailing(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	if err := c.Set(request, responseForTest(t, request, "ok")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.requestToPath(request), []byte("{ this is not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get(request); ok {
		t.Fatal("a corrupt entry was served as a hit")
	}
}

func TestSetIsAtomicAndLeavesNoTemporaryBehind(t *testing.T) {
	// Set writes to a temporary and renames. A partially written file that Get
	// could observe would be a corrupt entry served as a hit.
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	response := responseForTest(t, request, "value")
	response.Usage.Input = 10
	response, err := provider.FinalizeResponse(request, response)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set(request, response); err != nil {
		t.Fatal(err)
	}
	var temps []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".tmp") {
			temps = append(temps, path)
		}
		return nil
	})
	if len(temps) > 0 {
		t.Fatalf("temporary files left behind: %v", temps)
	}
	info, err := os.Stat(c.requestToPath(request))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("entry mode = %o, want 600", perm)
	}
}

func TestOverwriteReplacesRatherThanAppends(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	if err := c.Set(request, responseForTest(t, request, "first")); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(request, responseForTest(t, request, "second")); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get(request)
	if !ok || got.Response.RawText != "second" {
		t.Fatalf("read back %q, want the second write", got.Response.RawText)
	}
}

func TestConcurrentSetPublishesOneCompleteEntry(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	responses := []provider.ResponseRecord{
		responseForTest(t, request, strings.Repeat("a", 64*1024)),
		responseForTest(t, request, strings.Repeat("b", 64*1024)),
		responseForTest(t, request, strings.Repeat("c", 64*1024)),
		responseForTest(t, request, strings.Repeat("d", 64*1024)),
	}
	var group sync.WaitGroup
	errorsFound := make(chan error, len(responses))
	for _, response := range responses {
		group.Add(1)
		go func() {
			defer group.Done()
			errorsFound <- c.Set(request, response)
		}()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, ok := c.Get(request)
	if !ok {
		t.Fatal("concurrent writes left no readable entry")
	}
	matched := false
	for _, response := range responses {
		matched = matched || got.Response.RawText == response.RawText
	}
	if !matched {
		t.Fatal("cache entry is a partial or mixed writer payload")
	}
}

func TestDisabledCacheNeitherReadsNorWrites(t *testing.T) {
	dir := t.TempDir()
	enabled := New(dir, true)
	request := baseRequest()
	if err := enabled.Set(request, responseForTest(t, request, "present")); err != nil {
		t.Fatal(err)
	}

	disabled := New(dir, false)
	if disabled.Enabled() {
		t.Fatal("a cache constructed disabled reports itself enabled")
	}
	if _, ok := disabled.Get(request); ok {
		t.Fatal("a disabled cache served an entry; --no-cache would not mean no cache")
	}
	other := baseRequest()
	other.Messages = []provider.Message{{Role: "user", Content: "written while disabled"}}
	if err := disabled.Set(other, provider.ResponseRecord{}); err != nil {
		t.Fatalf("Set on a disabled cache returned %v, want nil", err)
	}
	if _, ok := enabled.Get(other); ok {
		t.Fatal("a disabled cache wrote an entry")
	}
}

func TestNilCacheIsSafe(t *testing.T) {
	var c *Cache
	if c.Enabled() {
		t.Fatal("nil cache reports itself enabled")
	}
	if _, ok := c.Get(baseRequest()); ok {
		t.Fatal("nil cache reported a hit")
	}
	if err := c.Set(baseRequest(), provider.ResponseRecord{}); err != nil {
		t.Fatalf("Set on a nil cache returned %v", err)
	}
}

func TestStatsCountsOnlyEntries(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	for i, prompt := range []string{"one", "two", "three"} {
		request := baseRequest()
		request.Messages = []provider.Message{{Role: "user", Content: prompt}}
		if err := c.Set(request, responseForTest(t, request, strings.Repeat("x", 100*(i+1)))); err != nil {
			t.Fatal(err)
		}
	}
	// A stray non-entry file must not be counted as an entry.
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("not an entry"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := c.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if s.Entries != 3 {
		t.Fatalf("entries = %d, want 3", s.Entries)
	}
	if s.SizeBytes <= 300 {
		t.Fatalf("size = %d bytes, want the entry payloads counted", s.SizeBytes)
	}
}

func TestStatsOnAMissingDirectoryIsNotAnError(t *testing.T) {
	// `evalwitness cache stats` before the first run must report zero rather than
	// fail.
	c := New(filepath.Join(t.TempDir(), "never-created"), true)
	s, err := c.Stats()
	if err != nil {
		t.Fatalf("Stats on a missing directory returned %v", err)
	}
	if s.Entries != 0 {
		t.Fatalf("entries = %d, want 0", s.Entries)
	}
}

func TestClearRemovesOnlyExplicitOwnedScopes(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true)
	request := baseRequest()
	if err := c.Set(request, responseForTest(t, request, "value")); err != nil {
		t.Fatal(err)
	}
	namespace, err := safety.NewRouteNamespace(request.ProviderID, request.RequestedModel)
	if err != nil {
		t.Fatal(err)
	}
	root, err := c.ownedRoot(false)
	if err != nil {
		t.Fatal(err)
	}
	capabilities := filepath.Join(namespace.Directory(), "capabilities.json")
	if err := root.PublishSensitive(capabilities, []byte(`{"logprobs":true}`)); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(root.Path(), "do-not-delete")
	if err := os.WriteFile(unknown, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := c.Clear(ClearResponses)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scope != ClearResponses || result.FilesRemoved != 1 || result.BytesRemoved == 0 {
		t.Fatalf("response clear result = %+v", result)
	}
	if _, ok := c.Get(request); ok {
		t.Fatal("response survived response clear")
	}
	if _, err := os.Stat(filepath.Join(root.Path(), capabilities)); err != nil {
		t.Fatalf("capabilities removed by response clear: %v", err)
	}
	for _, retained := range []string{
		root.Path(),
		filepath.Join(root.Path(), safety.CacheRootMarkerName),
		filepath.Join(root.Path(), namespace.IdentityPath()),
		unknown,
	} {
		if _, err := os.Stat(retained); err != nil {
			t.Fatalf("clear removed retained path %q: %v", retained, err)
		}
	}

	result, err = c.Clear(ClearCapabilities)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesRemoved != 1 {
		t.Fatalf("capability clear result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root.Path(), capabilities)); !os.IsNotExist(err) {
		t.Fatalf("capabilities survived clear: %v", err)
	}
}

func TestClearRejectsMissingScopeUnownedRootAndProtectedPath(t *testing.T) {
	if _, err := New(t.TempDir(), true).Clear(""); !safety.IsKind(err, safety.ErrorInvalidInput) {
		t.Fatalf("missing scope error = %T %v", err, err)
	}

	unowned := t.TempDir()
	sentinel := filepath.Join(unowned, "sentinel")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(unowned, true).Clear(ClearAll); !safety.IsKind(err, safety.ErrorUnownedRoot) && !safety.IsKind(err, safety.ErrorUnsafePermissions) {
		t.Fatalf("unowned root error = %T %v", err, err)
	}
	if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "unchanged" {
		t.Fatalf("unowned sentinel changed: %q %v", raw, err)
	}

	if _, err := New(string(filepath.Separator), true).Clear(ClearAll); !safety.IsKind(err, safety.ErrorProtectedPath) {
		t.Fatalf("filesystem root error = %T %v", err, err)
	}
}
