package config

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

func TestEnvironmentResolverReportsAbsentSetting(t *testing.T) {
	resolver := newEnvironmentResolver(func(string) (string, bool) { return "", false })

	value, ok, source := resolver.lookupValue(EnvModel)
	if ok || value != "" || source != environmentAbsent {
		t.Fatalf("lookup = %q, %t, %d; want absent", value, ok, source)
	}
	if names := resolver.legacySettings(); len(names) != 0 {
		t.Fatalf("absent setting recorded legacy names: %v", names)
	}
}

func TestEnvironmentResolverUsesCanonicalSettingAlone(t *testing.T) {
	resolver := newEnvironmentResolver(func(name string) (string, bool) {
		return map[string]string{"EVALWITNESS_MODEL": "canonical"}[name], name == "EVALWITNESS_MODEL"
	})

	value, ok, source := resolver.lookupValue(EnvModel)
	if !ok || value != "canonical" || source != environmentCanonical {
		t.Fatalf("lookup = %q, %t, %d; want canonical", value, ok, source)
	}
}

func TestEnvironmentResolverPrecedence(t *testing.T) {
	values := map[string]string{
		"EVALWITNESS_MODEL": "canonical",
		"LOGPROBE_MODEL":    "legacy",
	}
	resolver := newEnvironmentResolver(func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	})

	value, ok, source := resolver.lookupValue(EnvModel)
	if !ok || value != "canonical" || source != environmentCanonical {
		t.Fatalf("lookup = %q, %t, %d; want canonical", value, ok, source)
	}
	if names := resolver.legacySettings(); len(names) != 0 {
		t.Fatalf("ignored legacy setting was reported as consumed: %v", names)
	}
}

func TestEnvironmentResolverUsesAndRecordsLegacyFallback(t *testing.T) {
	values := map[string]string{"LOGPROBE_MODEL": "legacy-value"}
	resolver := newEnvironmentResolver(func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	})

	value, ok, source := resolver.lookupValue(EnvModel)
	if !ok || value != "legacy-value" || source != environmentLegacy {
		t.Fatalf("lookup = %q, %t, %d; want legacy fallback", value, ok, source)
	}
	names := resolver.legacySettings()
	if len(names) != 1 || names[0] != "LOGPROBE_MODEL" {
		t.Fatalf("legacy names = %v", names)
	}
}

func TestLegacyWarningIsSortedOnceAndRedacted(t *testing.T) {
	previousWriter := legacyWarningWriter
	previousOnce := legacyWarningOnce
	var output bytes.Buffer
	legacyWarningWriter = &output
	legacyWarningOnce = &sync.Once{}
	t.Cleanup(func() {
		legacyWarningWriter = previousWriter
		legacyWarningOnce = previousOnce
	})

	if err := warnLegacySettings([]string{"LOGPROBE_MODEL", "LOGPROBE_API_KEY"}); err != nil {
		t.Fatal(err)
	}
	if err := warnLegacySettings([]string{"LOGPROBE_PRESET"}); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	want := "warning: legacy LOGPROBE_* settings consumed: LOGPROBE_API_KEY, LOGPROBE_MODEL; migrate to EVALWITNESS_*; values were not logged\n"
	if got != want {
		t.Fatalf("warning = %q, want %q", got, want)
	}
	for _, secret := range []string{"legacy-value", "secret", "api-key-value"} {
		if strings.Contains(got, secret) {
			t.Fatalf("warning exposed a value marker %q: %s", secret, got)
		}
	}
}

func TestRuntimeEnvironmentRegistryHasCanonicalLegacyPairs(t *testing.T) {
	seenCanonical := make(map[string]bool, len(runtimeEnvironmentKeys))
	seenLegacy := make(map[string]bool, len(runtimeEnvironmentKeys))
	for _, key := range runtimeEnvironmentKeys {
		canonical := string(key)
		legacy := legacyEnvironmentName(key)
		if !strings.HasPrefix(canonical, CanonicalEnvironmentPrefix) {
			t.Fatalf("non-canonical key %q", canonical)
		}
		if !strings.HasPrefix(legacy, LegacyEnvironmentPrefix) {
			t.Fatalf("non-legacy counterpart %q", legacy)
		}
		if seenCanonical[canonical] || seenLegacy[legacy] {
			t.Fatalf("duplicate environment mapping %q -> %q", canonical, legacy)
		}
		seenCanonical[canonical] = true
		seenLegacy[legacy] = true
	}
}

func TestCanonicalAndLegacySettingsProduceTheSameRequestFingerprint(t *testing.T) {
	canonicalValues := map[string]string{
		"EVALWITNESS_PROVIDER":      "route",
		"EVALWITNESS_BASE_URL":      "https://gateway.example/v1",
		"EVALWITNESS_MODEL":         "model",
		"EVALWITNESS_THINKING_MODE": "disabled",
		"EVALWITNESS_TEMPERATURE":   "0.25",
		"EVALWITNESS_MAX_TOKENS":    "256",
		"EVALWITNESS_STREAM":        "true",
	}
	legacyValues := map[string]string{}
	for name, value := range canonicalValues {
		legacyValues[strings.Replace(name, "EVALWITNESS_", "LOGPROBE_", 1)] = value
	}
	load := func(values map[string]string) Config {
		cfg := Default()
		resolver := newEnvironmentResolver(func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		})
		cfg.applyEnv(resolver)
		cfg.normalizeThinkingMode()
		return cfg
	}
	canonicalConfig := load(canonicalValues)
	legacyConfig := load(legacyValues)
	request := func(cfg Config) provider.RequestEnvelope {
		envelope, err := provider.NewRequestEnvelope(provider.RequestOptions{
			ProviderID:      cfg.Provider,
			BaseURL:         cfg.BaseURL,
			RequestedModel:  cfg.Model,
			ThinkingMode:    cfg.ThinkingMode,
			Messages:        []provider.Message{{Role: "user", Content: "same prompt"}},
			Temperature:     cfg.Temperature,
			MaxOutputTokens: cfg.MaxTokens,
			Logprobs:        true,
			TopLogprobs:     20,
			Stream:          cfg.Stream,
			Lineage:         provider.RequestLineage{SamplingSlot: "same"},
		})
		if err != nil {
			t.Fatal(err)
		}
		return envelope
	}
	canonicalFingerprint, err := request(canonicalConfig).Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	legacyFingerprint, err := request(legacyConfig).Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if canonicalFingerprint != legacyFingerprint {
		t.Fatalf("canonical fingerprint %s != legacy compatibility fingerprint %s", canonicalFingerprint, legacyFingerprint)
	}
}
