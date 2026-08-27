package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const SchemaVersion = 3
const maxEntryBytes = 16 * 1024 * 1024

const (
	SourceEvalWitness = "evalwitness"
	SourceLegacy      = "logprobe"
)

type Entry struct {
	SchemaVersion      int                               `json:"schema_version"`
	RequestFingerprint provider.Fingerprint              `json:"request_fingerprint"`
	SamplingSlot       string                            `json:"sampling_slot"`
	Response           provider.ResponseRecord           `json:"response"`
	ScoreEvidence      map[string]verifier.ScoreEvidence `json:"score_evidence"`
	CreatedAt          int64                             `json:"created_at"`
	// SourceNamespace records which cache root supplied the entry without
	// changing the immutable on-disk schema.
	SourceNamespace string `json:"-"`
}

type LegacyEntry struct {
	SchemaVersion int                           `json:"schema_version"`
	Distribution  map[string]map[string]float64 `json:"distribution"`
	RawText       string                        `json:"raw_text"`
	InputTokens   int                           `json:"input_tokens"`
	OutputTokens  int                           `json:"output_tokens"`
	CachedTokens  int                           `json:"cached_tokens"`
	ServedModel   string                        `json:"served_model,omitempty"`
	CreatedAt     int64                         `json:"created_at"`
}

type Cache struct {
	dir         string
	legacyDir   string
	enabled     bool
	rootMu      sync.Mutex
	operationMu sync.RWMutex
	root        *safety.CacheRoot
}

func New(dir string, enabled bool) *Cache {
	return &Cache{dir: dir, enabled: enabled}
}

// NewWithLegacyImport adds one read-only legacy root. Get may read an exact
// key from it after a primary miss; Set, Stats, and Clear always target dir.
func NewWithLegacyImport(dir, legacyDir string, enabled bool) *Cache {
	return &Cache{dir: dir, legacyDir: legacyDir, enabled: enabled}
}

func (c *Cache) Enabled() bool { return c != nil && c.enabled }

func requestStorageHash(request provider.RequestEnvelope) (string, error) {
	fingerprint, err := request.Fingerprint()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(fingerprint))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(request.Lineage.SamplingSlot))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *Cache) requestToPath(request provider.RequestEnvelope) string {
	relative, err := requestToRelativePath(request)
	if err != nil {
		return ""
	}
	return filepath.Join(c.dir, relative)
}

func requestToRelativePath(request provider.RequestEnvelope) (string, error) {
	namespace, err := safety.NewRouteNamespace(request.ProviderID, request.RequestedModel)
	if err != nil {
		return "", err
	}
	hash, err := requestStorageHash(request)
	if err != nil {
		return "", err
	}
	return filepath.Join(namespace.Directory(), "responses", hash[:2], hash+".json"), nil
}

func legacyRequestToPathIn(dir string, request provider.RequestEnvelope) (string, bool) {
	if request.ProviderID == "" || request.ProviderID == "." || request.ProviderID == ".." ||
		filepath.IsAbs(request.ProviderID) || filepath.Base(request.ProviderID) != request.ProviderID || strings.ContainsAny(request.ProviderID, `/\`) {
		return "", false
	}
	prompt, ok := request.Prompt()
	if !ok {
		return "", false
	}
	hash := legacyHash(request.ProviderID, request.RequestedModel, request.Lineage.SamplingSlot, prompt, request.TopLogprobs)
	return filepath.Join(dir, request.ProviderID, hash[:2], hash+".json"), true
}

func (c *Cache) Get(request provider.RequestEnvelope) (Entry, bool) {
	if !c.Enabled() {
		return Entry{}, false
	}
	c.operationMu.RLock()
	defer c.operationMu.RUnlock()
	if relative, err := requestToRelativePath(request); err == nil {
		if root, rootErr := c.ownedRoot(false); rootErr == nil {
			if raw, readErr := root.ReadSensitive(relative, maxEntryBytes); readErr == nil {
				if entry, ok := decodeEntry(raw, SourceEvalWitness, request); ok {
					return entry, true
				}
			}
		}
	}
	return Entry{}, false
}

func (c *Cache) GetLegacy(request provider.RequestEnvelope) (LegacyEntry, string, bool) {
	if !c.Enabled() {
		return LegacyEntry{}, "", false
	}
	if entry, ok := readLegacyEntry(c.dir, request); ok {
		return entry, SourceEvalWitness, true
	}
	if c.legacyDir == "" || c.legacyDir == c.dir {
		return LegacyEntry{}, "", false
	}
	entry, ok := readLegacyEntry(c.legacyDir, request)
	return entry, SourceLegacy, ok
}

func readLegacyEntry(dir string, request provider.RequestEnvelope) (LegacyEntry, bool) {
	path, ok := legacyRequestToPathIn(dir, request)
	if !ok {
		return LegacyEntry{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return LegacyEntry{}, false
	}
	if len(data) > maxEntryBytes {
		return LegacyEntry{}, false
	}
	var entry LegacyEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.SchemaVersion != 1 {
		return LegacyEntry{}, false
	}
	return entry, true
}

func decodeEntry(data []byte, sourceNamespace string, request provider.RequestEnvelope) (Entry, bool) {
	if len(data) > maxEntryBytes {
		return Entry{}, false
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, false
	}
	fingerprint, err := request.Fingerprint()
	mode := verifier.ExtractionModeJudge
	if request.Logprobs {
		mode = verifier.ExtractionModeVerifier
	}
	expectedEvidence := verifier.ExtractAllScoreEvidence(request, e.Response, mode)
	if err != nil || e.SchemaVersion != SchemaVersion || e.RequestFingerprint != fingerprint ||
		e.SamplingSlot != request.Lineage.SamplingSlot || e.Response.ValidateExact(request) != nil {
		return Entry{}, false
	}
	if !reflect.DeepEqual(e.ScoreEvidence, expectedEvidence) {
		return Entry{}, false
	}
	if err := validateScoreEvidence(request, e.ScoreEvidence); err != nil {
		return Entry{}, false
	}
	e.SourceNamespace = sourceNamespace
	return e, true
}

func (c *Cache) Set(request provider.RequestEnvelope, response provider.ResponseRecord) error {
	if !c.Enabled() {
		return nil
	}
	c.operationMu.RLock()
	defer c.operationMu.RUnlock()
	if err := response.ValidateExact(request); err != nil {
		return fmt.Errorf("cache response: %w", err)
	}
	fingerprint, err := request.Fingerprint()
	if err != nil {
		return err
	}
	evidence := verifier.ExtractAllScoreEvidence(request, response, cacheExtractionMode(request))
	if err := validateScoreEvidence(request, evidence); err != nil {
		return fmt.Errorf("cache score evidence: %w", err)
	}
	e := Entry{
		SchemaVersion:      SchemaVersion,
		RequestFingerprint: fingerprint,
		SamplingSlot:       request.Lineage.SamplingSlot,
		Response:           response,
		ScoreEvidence:      evidence,
		CreatedAt:          time.Now().Unix(),
	}
	relative, err := requestToRelativePath(request)
	if err != nil {
		return err
	}
	root, err := c.ownedRoot(true)
	if err != nil {
		return err
	}
	namespace, err := safety.NewRouteNamespace(request.ProviderID, request.RequestedModel)
	if err != nil {
		return err
	}
	if err := root.EnsureRouteIdentity(namespace); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return root.PublishSensitive(relative, data)
}

func cacheExtractionMode(request provider.RequestEnvelope) verifier.ExtractionMode {
	if request.Logprobs {
		return verifier.ExtractionModeVerifier
	}
	return verifier.ExtractionModeJudge
}

func validateScoreEvidence(request provider.RequestEnvelope, evidence map[string]verifier.ScoreEvidence) error {
	if len(request.ScoreTags) == 0 {
		return nil
	}
	if request.Logprobs {
		return verifier.ValidateStrictEvidence(evidence)
	}
	return verifier.ValidateJudgeEvidence(evidence)
}

func legacyHash(providerID, model, samplingSlot, prompt string, topLogprobs int) string {
	h := sha256.New()
	separator := []byte{0}
	_, _ = h.Write([]byte(providerID))
	_, _ = h.Write(separator)
	_, _ = h.Write([]byte(model))
	_, _ = h.Write(separator)
	_, _ = h.Write([]byte(samplingSlot))
	_, _ = h.Write(separator)
	_, _ = h.Write([]byte(prompt))
	_, _ = h.Write(separator)
	_, _ = h.Write(fmt.Appendf(nil, "%d", topLogprobs))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Cache) ownedRoot(create bool) (*safety.CacheRoot, error) {
	if c == nil {
		return nil, &safety.Error{Kind: safety.ErrorInvalidInput, Operation: safety.OperationValidate}
	}
	c.rootMu.Lock()
	defer c.rootMu.Unlock()
	if c.root != nil {
		return c.root, nil
	}
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return nil, err
	}
	if create {
		c.root, err = safety.CreateCacheRoot(policy, c.dir)
	} else {
		c.root, err = safety.OpenCacheRoot(policy, c.dir)
	}
	return c.root, err
}

type Stats struct {
	Entries   int64
	SizeBytes int64
}

func (c *Cache) Stats() (Stats, error) {
	c.operationMu.RLock()
	defer c.operationMu.RUnlock()
	var s Stats
	if _, err := os.Stat(c.dir); errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	root, err := c.ownedRoot(false)
	if err != nil {
		return s, err
	}
	routes := filepath.Join(root.Path(), "routes")
	err = filepath.Walk(routes, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if isResponseEntryPath(routes, path) {
			s.Entries++
			s.SizeBytes += info.Size()
		}
		return nil
	})
	return s, err
}

func isResponseEntryPath(routes, path string) bool {
	relative, err := filepath.Rel(routes, path)
	if err != nil || filepath.Ext(relative) != ".json" {
		return false
	}
	parts := strings.Split(relative, string(filepath.Separator))
	return len(parts) == 4 && safety.IsSafeNamespaceID(parts[0]) && parts[1] == "responses" && len(parts[2]) == 2
}
