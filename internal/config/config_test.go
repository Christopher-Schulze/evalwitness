package config

import (
	"os"
	"testing"
)

func TestDefaultRouteIsTheMeasuredOne(t *testing.T) {
	// The default is the route the project now works on: the live-probed free
	// b.ai DeepSeek V4 Flash. A run with no configuration reproduces it.
	t.Setenv("BAI_API_KEY", "test-bai-key")

	cfg := Default()
	if cfg.Provider != "bai" {
		t.Fatalf("provider = %q, want bai", cfg.Provider)
	}
	if cfg.WireFormat != "openai" {
		t.Fatalf("wire format = %q, want openai", cfg.WireFormat)
	}
	if cfg.BaseURL != "https://api.b.ai/v1" {
		t.Fatalf("base url = %q", cfg.BaseURL)
	}
	if cfg.Model != "deepseek-v4-flash" {
		t.Fatalf("model = %q, want deepseek-v4-flash", cfg.Model)
	}
	if resolveAPIKey(cfg.Provider, newEnvironmentResolver(os.LookupEnv)) != "test-bai-key" {
		t.Fatalf("api key did not resolve from BAI_API_KEY")
	}
	if cfg.BillingModel != "free-tier" {
		t.Fatalf("billing model = %q, want free-tier", cfg.BillingModel)
	}
	if cfg.ContextLimit != 1_000_000 {
		t.Fatalf("context limit = %d, want 1000000", cfg.ContextLimit)
	}
	if cfg.ThinkingMode != "disabled" {
		t.Fatalf("thinking mode = %q, want disabled", cfg.ThinkingMode)
	}
}

func TestPresetKeepsProviderLabelSeparateFromWireFormat(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "test-deepseek-key")

	cfg := Default()
	if !applyPreset(&cfg, "deepseek-v4-flash") {
		t.Fatalf("preset did not apply")
	}
	cfg.normalizeWireFormat()
	if cfg.Provider != "deepseek" {
		t.Fatalf("provider = %q, want deepseek", cfg.Provider)
	}
	if cfg.WireFormat != "openai" {
		t.Fatalf("wire format = %q, want openai", cfg.WireFormat)
	}
	if cfg.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("base url = %q", cfg.BaseURL)
	}
	if cfg.InputUSDPerM != 0 || cfg.CachedUSDPerM != 0 || cfg.OutputUSDPerM != 0 {
		t.Fatalf("preset embedded drift-prone rates: input %v cached %v output %v",
			cfg.InputUSDPerM, cfg.CachedUSDPerM, cfg.OutputUSDPerM)
	}
	if cfg.APIKey != "test-deepseek-key" {
		t.Fatalf("api key did not resolve from DEEPSEEK_API_KEY")
	}
	if cfg.ThinkingMode != "disabled" {
		t.Fatalf("thinking mode = %q, want disabled", cfg.ThinkingMode)
	}
}

func TestPresetSummariesExposeDefaultAndKeyNames(t *testing.T) {
	summaries := PresetSummaries()
	if len(summaries) == 0 {
		t.Fatalf("expected preset summaries")
	}
	defaults := 0
	foundDeepSeek := false
	foundMeasured := false
	foundFireworks := false
	foundOpenRouterMorph := false
	foundOpenRouterAmbient := false
	for _, summary := range summaries {
		if summary.Default {
			defaults++
			if summary.Name != "bai-deepseek-v4-flash" {
				t.Fatalf("default preset = %q, want the live-probed free route", summary.Name)
			}
		}
		if summary.Name == "deepseek-v4-pro" {
			foundDeepSeek = true
			if summary.CapabilityState != "configured" {
				t.Fatalf("deepseek pro preset state = %q, want configured", summary.CapabilityState)
			}
			if len(summary.KeyEnvNames) != 1 || summary.KeyEnvNames[0] != "DEEPSEEK_API_KEY" {
				t.Fatalf("deepseek key env names = %#v", summary.KeyEnvNames)
			}
		}
		if summary.Name == "opencode-go-deepseek-v4-flash-0731" {
			foundMeasured = true
			if summary.CapabilityState != "configured" {
				t.Fatalf("preset self-qualified with state %q", summary.CapabilityState)
			}
			if len(summary.KeyEnvNames) != 1 || summary.KeyEnvNames[0] != "OPENCODE_GO_API_KEY" {
				t.Fatalf("measured route key env names = %#v", summary.KeyEnvNames)
			}
		}
		if summary.Name == "fireworks-deepseek-v4-flash-0731" {
			foundFireworks = true
			if summary.Provider != "fireworks" || summary.Model != "deepseek-v4-flash-0731" {
				t.Fatalf("fireworks route = %s/%s", summary.Provider, summary.Model)
			}
			if len(summary.KeyEnvNames) != 1 || summary.KeyEnvNames[0] != "FIREWORKS_API_KEY" {
				t.Fatalf("fireworks key env names = %#v", summary.KeyEnvNames)
			}
			if summary.ContextLimit != 1_048_576 || summary.EvidenceTokens != 1_048_576 {
				t.Fatalf("fireworks token limits = context %d evidence %d", summary.ContextLimit, summary.EvidenceTokens)
			}
		}
		if summary.Name == "openrouter-morph-deepseek-v4-flash-0731" {
			foundOpenRouterMorph = true
			if summary.Provider != "openrouter-morph" || summary.Model != "deepseek/deepseek-v4-flash-0731" {
				t.Fatalf("OpenRouter Morph route = %s/%s", summary.Provider, summary.Model)
			}
			if len(summary.KeyEnvNames) != 1 || summary.KeyEnvNames[0] != "OPENROUTER_API_KEY" {
				t.Fatalf("OpenRouter key env names = %#v", summary.KeyEnvNames)
			}
			if summary.ContextLimit != 1_048_576 || summary.EvidenceTokens != 1_048_576 {
				t.Fatalf("OpenRouter token limits = context %d evidence %d", summary.ContextLimit, summary.EvidenceTokens)
			}
			if summary.UpstreamProvider != "morph" {
				t.Fatalf("OpenRouter upstream provider = %q, want morph", summary.UpstreamProvider)
			}
		}
		if summary.Name == "openrouter-ambient-deepseek-v4-flash-0731" {
			foundOpenRouterAmbient = true
			if summary.Provider != "openrouter-ambient" || summary.Model != "deepseek/deepseek-v4-flash-0731" || summary.UpstreamProvider != "ambient" {
				t.Fatalf("OpenRouter Ambient route = %s/%s upstream=%s", summary.Provider, summary.Model, summary.UpstreamProvider)
			}
			if summary.ContextLimit != 1_048_576 || summary.EvidenceTokens != 1_048_576 {
				t.Fatalf("OpenRouter Ambient token limits = context %d evidence %d", summary.ContextLimit, summary.EvidenceTokens)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("default preset count = %d, want 1", defaults)
	}
	if !foundDeepSeek {
		t.Fatalf("missing deepseek pro preset summary")
	}
	if !foundMeasured {
		t.Fatalf("missing summary for the measured route")
	}
	if !foundFireworks {
		t.Fatalf("missing summary for the Fireworks route")
	}
	if !foundOpenRouterMorph {
		t.Fatalf("missing summary for the OpenRouter Morph route")
	}
	if !foundOpenRouterAmbient {
		t.Fatalf("missing summary for the OpenRouter Ambient route")
	}
}

func TestOpenRouterMorphPresetBindsRouteAndCredential(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")

	cfg := Default()
	if !applyPreset(&cfg, "openrouter-morph-deepseek-v4-flash-0731") {
		t.Fatal("preset did not apply")
	}
	if cfg.Provider != "openrouter-morph" || cfg.UpstreamProvider != "morph" {
		t.Fatalf("route binding = %q/%q", cfg.Provider, cfg.UpstreamProvider)
	}
	if cfg.APIKey != "test-openrouter-key" {
		t.Fatal("OpenRouter credential was not resolved")
	}
}

func TestAnthropicWireFormatRejected(t *testing.T) {
	t.Setenv("EVALWITNESS_WIRE_FORMAT", "anthropic")

	cfg := Default()
	cfg.applyEnv(newEnvironmentResolver(os.LookupEnv))
	if err := cfg.validate(); err == nil {
		t.Fatalf("validate succeeded with anthropic wire format")
	}
}

func TestInvalidThinkingModeRejected(t *testing.T) {
	cfg := Default()
	cfg.APIKey = "test-key"
	cfg.ThinkingMode = "max"
	if err := cfg.validate(); err == nil {
		t.Fatalf("validate succeeded with invalid thinking mode")
	}
}

func TestMaxRetriesEnv(t *testing.T) {
	t.Setenv("EVALWITNESS_MAX_RETRIES", "10")

	cfg := Default()
	cfg.applyEnv(newEnvironmentResolver(os.LookupEnv))
	if cfg.MaxRetries != 10 {
		t.Fatalf("max retries = %d, want 10", cfg.MaxRetries)
	}
}

func TestCAFileEnv(t *testing.T) {
	t.Setenv("EVALWITNESS_CA_FILE", "/tmp/test-ca.pem")
	cfg := Default()
	cfg.applyEnv(newEnvironmentResolver(os.LookupEnv))
	if cfg.CAFile != "/tmp/test-ca.pem" {
		t.Fatalf("CA file = %q", cfg.CAFile)
	}
}

func TestNonPositiveMaxRetriesRejected(t *testing.T) {
	cfg := Default()
	cfg.APIKey = "test-key"
	cfg.MaxRetries = 0
	if err := cfg.validate(); err == nil {
		t.Fatalf("validate succeeded with non-positive max retries")
	}
}

func TestNonDeepSeekModelClearsImplicitThinkingMode(t *testing.T) {
	cfg := Default()
	cfg.Provider = "custom"
	cfg.BaseURL = "https://example.com/v1"
	cfg.Model = "some-other-model"
	cfg.normalizeThinkingMode()
	if cfg.ThinkingMode != "" {
		t.Fatalf("thinking mode = %q, want cleared", cfg.ThinkingMode)
	}
}

func TestGatewayHostedDeepSeekModelKeepsThinkingMode(t *testing.T) {
	// The thinking switch follows the model family: a DeepSeek model behind a
	// non-DeepSeek gateway still needs
	// thinking=disabled for score-token logprobs.
	cfg := Default()
	cfg.Provider = "opencode-go"
	cfg.BaseURL = "https://opencode.ai/zen/v1"
	cfg.Model = "deepseek-v4-flash-free"
	cfg.normalizeThinkingMode()
	if cfg.ThinkingMode != "disabled" {
		t.Fatalf("thinking mode = %q, want disabled", cfg.ThinkingMode)
	}
}

func TestPresetDeclaredThinkingModeSurvivesNormalization(t *testing.T) {
	// A preset that declares thinking=disabled must keep it through
	// normalization. Where a reasoning model is left enabled it spends the whole
	// output budget on reasoning tokens and returns an empty logprobs array,
	// which destroys a verifier run silently.
	for _, name := range []string{"opencode-go-deepseek-v4-flash-0731", "deepseek-v4-pro"} {
		cfg := Default()
		if !applyPreset(&cfg, name) {
			t.Fatalf("%s: preset did not apply", name)
		}
		cfg.normalizeThinkingMode()
		if cfg.ThinkingMode != "disabled" {
			t.Fatalf("%s: thinking mode = %q, want disabled", name, cfg.ThinkingMode)
		}
	}
}

func TestPresetWithoutThinkingModeStaysImplicit(t *testing.T) {
	// Every shipped preset currently declares a thinking mode, so this exercises
	// the mechanism directly rather than through whichever preset happens not to.
	// A preset that stays silent must leave the mode inherited, so the model
	// family heuristic in normalizeThinkingMode still applies to it.
	cfg := Default()
	applyPresetValue(&cfg, Preset{
		Name:       "test-silent-thinking",
		Provider:   "example",
		WireFormat: "openai",
		Model:      "some-model",
		BaseURL:    "https://example.invalid/v1",
		KeyEnvName: "EXAMPLE_API_KEY",
	})
	if cfg.thinkingModeExplicit {
		t.Fatalf("preset without ThinkingMode must not mark the mode explicit")
	}
	cfg.normalizeThinkingMode()
	if cfg.ThinkingMode != "" {
		t.Fatalf("thinking mode = %q, want empty for a non-DeepSeek model", cfg.ThinkingMode)
	}
}

func TestExplicitThinkingModeEnvSurvivesNormalization(t *testing.T) {
	t.Setenv("EVALWITNESS_THINKING_MODE", "disabled")

	cfg := Default()
	cfg.Provider = "custom"
	cfg.BaseURL = "https://example.com/v1"
	cfg.Model = "some-other-model"
	cfg.applyEnv(newEnvironmentResolver(os.LookupEnv))
	cfg.normalizeThinkingMode()
	if cfg.ThinkingMode != "disabled" {
		t.Fatalf("thinking mode = %q, want disabled", cfg.ThinkingMode)
	}
}

func TestBoundedAdaptiveDefaults(t *testing.T) {
	cfg := Default()
	if cfg.DefaultReps != 1 || cfg.BiasMitigation != "adaptive" || cfg.SPRT.Enabled {
		t.Fatalf("default sampling = reps %d bias %q sprt %t", cfg.DefaultReps, cfg.BiasMitigation, cfg.SPRT.Enabled)
	}
	// Swept offline over both benchmark caches on 2026-08-07. A two-call ceiling
	// holds both scores (78/89, 385/500) at 97 and 206 calls; the third call the
	// earlier sweep kept changed no Terminal-Bench decision and split 1-1 on
	// SWE-bench for 20% and 8% more calls, so it is opt-in.
	if cfg.MaxPairCalls != 2 || cfg.PairConfidence != 0.6 || cfg.PairCalibrationSigma != 0.05 {
		t.Fatalf("pair bounds = calls %d confidence %v sigma %v", cfg.MaxPairCalls, cfg.PairConfidence, cfg.PairCalibrationSigma)
	}
	// Measured on both benchmarks by replaying the accepted all-pairs caches:
	// Terminal-Bench 78/89 at 97 calls against 234, SWE-bench 385/500 at 206
	// against 321. Identical score on either, at well under half the calls.
	if !cfg.SingleElim {
		t.Fatal("the dynamic tournament is the production default")
	}
	if cfg.EvidenceTokens != 32_000 {
		t.Fatalf("evidence tokens = %d, want 32000", cfg.EvidenceTokens)
	}
}
