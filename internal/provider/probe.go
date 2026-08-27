package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/log"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

const (
	probeSchemaVersion = 1
	probeMaxAge        = 30 * 24 * time.Hour
	probePromptText    = "Reply with exactly one capital letter from A through T. Output only the letter, nothing else."
	maxProbeRecordSize = 16 * 1024
)

type CapabilityCacheSource string

const (
	CapabilityCacheEvalWitness CapabilityCacheSource = "evalwitness"
	CapabilityCacheLegacy      CapabilityCacheSource = "logprobe"
)

// DefaultContextLimit is the assumed model context window when no override is
// configured. Callers can pass an explicit limit via the cacheDir-paired
// caller (currently main.go reads EVALWITNESS_CONTEXT_LIMIT and passes it through).
const DefaultContextLimit = 128_000

type probeRecord struct {
	SchemaVersion   int    `json:"schema_version"`
	TS              int64  `json:"ts"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Logprobs        bool   `json:"logprobs"`
	TopLogprobsMax  int    `json:"top_logprobs_max"`
	LogitBias       bool   `json:"logit_bias"`
	PromptCache     bool   `json:"prompt_cache"`
	Streaming       bool   `json:"streaming"`
	ContextLimit    int    `json:"context_limit"`
	MaxConcurrent   int    `json:"max_concurrent"`
	ResponseExcerpt string `json:"probe_response_excerpt"`
}

func capsCachePath(cacheDir, provider, model string) string {
	namespace, err := safety.NewRouteNamespace(provider, model)
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, namespace.Directory(), "capabilities.json")
}

func legacyCapsCachePath(cacheDir, provider, model string) (string, bool) {
	if provider == "" || provider == "." || provider == ".." || filepath.IsAbs(provider) ||
		filepath.Base(provider) != provider || strings.ContainsAny(provider, `/\`) {
		return "", false
	}
	return filepath.Join(cacheDir, "caps", provider+"-"+sanitizeLegacyModel(model)+"-caps.json"), true
}

func sanitizeLegacyModel(s string) string {
	out := []rune(s)
	for i, r := range out {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			out[i] = '_'
		}
	}
	return string(out)
}

func LoadCachedCaps(cacheDir, provider, model string) (Capabilities, bool) {
	caps, _, ok := LoadCachedCapsWithLegacy(cacheDir, "", provider, model)
	return caps, ok
}

// LoadCachedCapsWithLegacy reads the canonical cache first and an explicitly
// configured legacy cache second. Probe always writes only to cacheDir.
func LoadCachedCapsWithLegacy(cacheDir, legacyCacheDir, provider, model string) (Capabilities, CapabilityCacheSource, bool) {
	if caps, ok := loadCachedCaps(cacheDir, provider, model); ok {
		return caps, CapabilityCacheEvalWitness, true
	}
	if legacyCacheDir == "" || legacyCacheDir == cacheDir {
		return Capabilities{}, "", false
	}
	if caps, ok := loadCachedCaps(legacyCacheDir, provider, model); ok {
		return caps, CapabilityCacheLegacy, true
	}
	return Capabilities{}, "", false
}

func loadCachedCaps(cacheDir, provider, model string) (Capabilities, bool) {
	namespace, err := safety.NewRouteNamespace(provider, model)
	if err == nil {
		policy, policyErr := safety.CurrentPathPolicy()
		if policyErr == nil {
			if root, rootErr := safety.OpenCacheRoot(policy, cacheDir); rootErr == nil {
				if identityErr := root.VerifyRouteIdentity(namespace); identityErr == nil {
					relative := filepath.Join(namespace.Directory(), "capabilities.json")
					if data, readErr := root.ReadSensitive(relative, maxProbeRecordSize); readErr == nil {
						if caps, ok := decodeCachedCaps(data, provider, model); ok {
							return caps, true
						}
					}
				}
			}
		}
	}
	path, ok := legacyCapsCachePath(cacheDir, provider, model)
	if !ok {
		return Capabilities{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > maxProbeRecordSize {
		return Capabilities{}, false
	}
	return decodeCachedCaps(data, provider, model)
}

func decodeCachedCaps(data []byte, provider, model string) (Capabilities, bool) {
	var pr probeRecord
	if err := json.Unmarshal(data, &pr); err != nil {
		return Capabilities{}, false
	}
	if pr.SchemaVersion != probeSchemaVersion || pr.Provider != provider || pr.Model != model {
		return Capabilities{}, false
	}
	if time.Now().Unix()-pr.TS > int64(probeMaxAge.Seconds()) {
		return Capabilities{}, false
	}
	return Capabilities{
		Logprobs:       pr.Logprobs,
		TopLogprobsMax: pr.TopLogprobsMax,
		LogitBias:      pr.LogitBias,
		PromptCache:    pr.PromptCache,
		Streaming:      pr.Streaming,
		ContextLimit:   pr.ContextLimit,
		MaxConcurrent:  pr.MaxConcurrent,
	}, true
}

// probeMaxTokens keeps the probe cheap while leaving room for a reasoning
// model that burns output tokens before the answer. Such a model returns empty
// choices when the cap is too tight for its thinking prelude, which reads as a
// capability failure rather than as the budget being too small.
const probeMaxTokens = 512

func Probe(ctx context.Context, p Provider, cacheDir, baseURL, model, thinkingMode string) (Capabilities, error) {
	req, err := NewProbeRequest(p.Name(), baseURL, model, thinkingMode)
	if err != nil {
		return Capabilities{}, err
	}
	return ProbeWithRequest(ctx, p, cacheDir, req)
}

func NewProbeRequest(providerID, baseURL, model, thinkingMode string) (RequestEnvelope, error) {
	req, err := NewRequestEnvelope(RequestOptions{
		ProviderID:      providerID,
		BaseURL:         baseURL,
		RequestedModel:  model,
		ThinkingMode:    thinkingMode,
		Messages:        []Message{{Role: "user", Content: probePromptText}},
		Temperature:     0,
		MaxOutputTokens: probeMaxTokens,
		Logprobs:        true,
		TopLogprobs:     20,
		ResponseFormat:  ResponseFormatText,
		Lineage:         RequestLineage{CriterionID: "capability-probe", SamplingSlot: "capability-probe", Entrypoint: "probe"},
	})
	if err != nil {
		return RequestEnvelope{}, fmt.Errorf("build probe request: %w", err)
	}
	return req, nil
}

func ProbeWithRequest(ctx context.Context, p Provider, cacheDir string, req RequestEnvelope) (Capabilities, error) {
	if err := req.Validate(); err != nil {
		return Capabilities{}, fmt.Errorf("probe request: %w", err)
	}
	if req.ProviderID != p.Name() || req.Lineage.Entrypoint != "probe" || req.Temperature != 0 || len(req.ScoreTags) != 0 {
		return Capabilities{}, errors.New("probe request does not match the weak capability-probe contract")
	}
	resp, err := p.Score(ctx, req)
	if err != nil {
		var pe *ProviderError
		if errors.As(err, &pe) && pe.Class == ClassCapabilityMissing {
			// Endpoint rejects logprobs with a 4xx: confirm the route itself
			// works without logprobs, then persist Logprobs=false instead of
			// failing the probe.
			retry := req
			retry.Logprobs = false
			retry.TopLogprobs = 0
			resp, err = p.Score(ctx, retry)
			if err != nil {
				return Capabilities{}, fmt.Errorf("probe call failed: %w", err)
			}
			resp.HasLogprobs = false
			resp.ObservedTopLogprobs = 0
		} else {
			return Capabilities{}, fmt.Errorf("probe call failed: %w", err)
		}
	}
	caps := Capabilities{
		Logprobs:       resp.HasLogprobs,
		TopLogprobsMax: resp.ObservedTopLogprobs,
		PromptCache:    resp.Usage.Cached > 0,
		ContextLimit:   DefaultContextLimit,
		MaxConcurrent:  8,
	}
	pr := probeRecord{
		SchemaVersion:   probeSchemaVersion,
		TS:              time.Now().Unix(),
		Provider:        p.Name(),
		Model:           req.RequestedModel,
		Logprobs:        caps.Logprobs,
		TopLogprobsMax:  caps.TopLogprobsMax,
		LogitBias:       caps.LogitBias,
		PromptCache:     caps.PromptCache,
		Streaming:       caps.Streaming,
		ContextLimit:    caps.ContextLimit,
		MaxConcurrent:   caps.MaxConcurrent,
		ResponseExcerpt: log.Redact(truncate(resp.RawText, 256)),
	}
	if err := persistCaps(cacheDir, pr); err != nil {
		return caps, fmt.Errorf("persist caps: %w", err)
	}
	return caps, nil
}

func persistCaps(cacheDir string, pr probeRecord) error {
	namespace, err := safety.NewRouteNamespace(pr.Provider, pr.Model)
	if err != nil {
		return err
	}
	policy, err := safety.CurrentPathPolicy()
	if err != nil {
		return err
	}
	root, err := safety.CreateCacheRoot(policy, cacheDir)
	if err != nil {
		return err
	}
	if err := root.EnsureRouteIdentity(namespace); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return err
	}
	return root.PublishSensitive(filepath.Join(namespace.Directory(), "capabilities.json"), data)
}
