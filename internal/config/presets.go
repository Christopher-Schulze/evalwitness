package config

import (
	"os"
	"sort"
)

// Preset bundles the provider/model/url/billing/rates/key-source for a known
// configuration. Set EVALWITNESS_PRESET to one of the registered names to apply
// everything at once. Individual env-vars (EVALWITNESS_BASE_URL etc.) still override
// preset values, so partial customization works.
type Preset struct {
	Name             string
	Provider         string
	WireFormat       string
	BaseURL          string
	Model            string
	BillingModel     string
	InputUSDPerM     float64
	CachedUSDPerM    float64
	OutputUSDPerM    float64
	KeyEnvName       string
	KeyEnvNames      []string
	ContextLimit     int
	EvidenceTokens   int
	ThinkingMode     string
	UpstreamProvider string
}

// PresetSummary is the public, non-secret view of a preset.
type PresetSummary struct {
	Name             string   `json:"name"`
	Provider         string   `json:"provider"`
	WireFormat       string   `json:"wire_format"`
	BaseURL          string   `json:"base_url"`
	Model            string   `json:"model"`
	BillingModel     string   `json:"billing_model"`
	KeyEnvNames      []string `json:"key_env_names"`
	ContextLimit     int      `json:"context_limit,omitempty"`
	EvidenceTokens   int      `json:"evidence_tokens,omitempty"`
	UpstreamProvider string   `json:"upstream_provider,omitempty"`
	Default          bool     `json:"default"`
	CapabilityState  string   `json:"capability_state"`
	Limitations      string   `json:"limitations,omitempty"`
}

var presets = map[string]Preset{
	"deepseek-v4-flash": {
		Name:         "deepseek-v4-flash",
		Provider:     "deepseek",
		WireFormat:   "openai",
		BaseURL:      "https://api.deepseek.com",
		Model:        "deepseek-v4-flash",
		BillingModel: "pay-as-you-go",
		KeyEnvName:   "DEEPSEEK_API_KEY",
		ContextLimit: 1_000_000,
		ThinkingMode: "disabled",
	},
	"deepseek-v4-pro": {
		Name:         "deepseek-v4-pro",
		Provider:     "deepseek",
		WireFormat:   "openai",
		BaseURL:      "https://api.deepseek.com",
		Model:        "deepseek-v4-pro",
		BillingModel: "pay-as-you-go",
		KeyEnvName:   "DEEPSEEK_API_KEY",
		ContextLimit: 1_000_000,
		ThinkingMode: "disabled",
	},
	"opencode-go-deepseek-v4-flash-0731": {
		Name: "opencode-go-deepseek-v4-flash-0731",
		// Own provider label on purpose: the cache is keyed by provider, model and
		// prompt, and the historical "opencode-go" + "deepseek-v4-flash" namespace
		// holds entries from an older checkpoint. A separate label guarantees this
		// route can never serve a cached answer that a different model produced.
		Provider:     "opencode-go-cn",
		WireFormat:   "openai",
		BaseURL:      "https://opencode.ai/zen/go/v1",
		Model:        "deepseek-v4-flash",
		BillingModel: "subscription",
		KeyEnvName:   "OPENCODE_GO_API_KEY",
		ContextLimit: 1_000_000,
		ThinkingMode: "disabled",
	},
	"fireworks-deepseek-v4-flash-0731": {
		Name:           "fireworks-deepseek-v4-flash-0731",
		Provider:       "fireworks",
		WireFormat:     "openai",
		BaseURL:        "https://api.fireworks.ai/inference/v1",
		Model:          "deepseek-v4-flash-0731",
		BillingModel:   "pay-as-you-go",
		KeyEnvName:     "FIREWORKS_API_KEY",
		ContextLimit:   1_048_576,
		EvidenceTokens: 1_048_576,
		ThinkingMode:   "disabled",
	},
	"openrouter-morph-deepseek-v4-flash-0731": {
		Name:             "openrouter-morph-deepseek-v4-flash-0731",
		Provider:         "openrouter-morph",
		WireFormat:       "openai",
		BaseURL:          "https://openrouter.ai/api/v1",
		Model:            "deepseek/deepseek-v4-flash-0731",
		BillingModel:     "pay-as-you-go",
		KeyEnvName:       "OPENROUTER_API_KEY",
		ContextLimit:     1_048_576,
		EvidenceTokens:   1_048_576,
		ThinkingMode:     "disabled",
		UpstreamProvider: "morph",
	},
	"openrouter-ambient-deepseek-v4-flash-0731": {
		Name:             "openrouter-ambient-deepseek-v4-flash-0731",
		Provider:         "openrouter-ambient",
		WireFormat:       "openai",
		BaseURL:          "https://openrouter.ai/api/v1",
		Model:            "deepseek/deepseek-v4-flash-0731",
		BillingModel:     "pay-as-you-go",
		KeyEnvName:       "OPENROUTER_API_KEY",
		ContextLimit:     1_048_576,
		EvidenceTokens:   1_048_576,
		ThinkingMode:     "disabled",
		UpstreamProvider: "ambient",
	},
	"bai-deepseek-v4-flash": {
		Name:         "bai-deepseek-v4-flash",
		Provider:     "bai",
		WireFormat:   "openai",
		BaseURL:      "https://api.b.ai/v1",
		Model:        "deepseek-v4-flash",
		BillingModel: "free-tier",
		KeyEnvName:   "BAI_API_KEY",
		ContextLimit: 1_000_000,
		ThinkingMode: "disabled",
	},
}

var presetMeta = map[string]struct {
	Default     bool
	Limitations string
}{
	"bai-deepseek-v4-flash": {
		Default:     true,
		Limitations: "live probe verified logprobs=true top_logprobs=20; free-tier unlimited; 1M context window",
	},
	"deepseek-v4-pro": {
		Limitations: "configured direct DeepSeek route; probe is diagnostic only, and evalwitness attest is required before paper-grade claims",
	},
	"fireworks-deepseek-v4-flash-0731": {
		Limitations: "configured Fireworks 1M-context route; provider pricing must be supplied at run time, and evalwitness attest must verify the strict top-20 score-evidence contract before research use",
	},
	"opencode-go-deepseek-v4-flash-0731": {
		Limitations: "requires the China opt-in on an OpenCode Go subscription. The response model field has historically exposed only deepseek-v4-flash, so 0731 remains an operator assertion until provider-issued identity proves it. Preset membership never qualifies the route; run evalwitness attest for the current request contract",
	},
	"openrouter-morph-deepseek-v4-flash-0731": {
		Limitations: "pins OpenRouter to Morph, requires parameter support, and disables fallbacks; provider pricing must be supplied at run time, and evalwitness attest must qualify the exact current route before research use",
	},
	"openrouter-ambient-deepseek-v4-flash-0731": {
		Limitations: "pins OpenRouter to Ambient, requires parameter support, and disables fallbacks; provider pricing must be supplied at run time, and evalwitness attest must qualify the exact current route before research use",
	},
	"deepseek-v4-flash": {
		Limitations: "configured direct DeepSeek route; probe is diagnostic only, and evalwitness attest is required before paper-grade claims",
	},
}

// PresetNames returns the registered preset identifiers.
func PresetNames() []string {
	out := make([]string, 0, len(presets))
	for k := range presets {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// PresetSummaries returns sorted non-secret preset metadata for CLI/docs agents.
func PresetSummaries() []PresetSummary {
	names := PresetNames()
	out := make([]PresetSummary, 0, len(names))
	for _, name := range names {
		p := presets[name]
		keyEnvNames := make([]string, 0, 1+len(p.KeyEnvNames))
		if p.KeyEnvName != "" {
			keyEnvNames = append(keyEnvNames, p.KeyEnvName)
		}
		keyEnvNames = append(keyEnvNames, p.KeyEnvNames...)
		meta := presetMeta[name]
		out = append(out, PresetSummary{
			Name:             p.Name,
			Provider:         p.Provider,
			WireFormat:       p.WireFormat,
			BaseURL:          p.BaseURL,
			Model:            p.Model,
			BillingModel:     p.BillingModel,
			KeyEnvNames:      keyEnvNames,
			ContextLimit:     p.ContextLimit,
			EvidenceTokens:   p.EvidenceTokens,
			UpstreamProvider: p.UpstreamProvider,
			Default:          meta.Default,
			CapabilityState:  "configured",
			Limitations:      meta.Limitations,
		})
	}
	return out
}

func applyPreset(c *Config, name string) bool {
	p, ok := presets[name]
	if !ok {
		return false
	}
	applyPresetValue(c, p)
	return true
}

// applyPresetValue applies a preset that need not be registered, which lets a
// test exercise the application rules themselves rather than whichever shipped
// preset happens to have the shape it needs.
func applyPresetValue(c *Config, p Preset) {
	c.Provider = p.Provider
	c.WireFormat = p.WireFormat
	c.BaseURL = p.BaseURL
	c.Model = p.Model
	if p.BillingModel != "" {
		c.BillingModel = p.BillingModel
	}
	c.InputUSDPerM = p.InputUSDPerM
	c.CachedUSDPerM = p.CachedUSDPerM
	c.OutputUSDPerM = p.OutputUSDPerM
	if p.ContextLimit > 0 {
		c.ContextLimit = p.ContextLimit
	}
	if p.EvidenceTokens > 0 {
		c.EvidenceTokens = p.EvidenceTokens
	}
	c.ThinkingMode = p.ThinkingMode
	c.thinkingModeExplicit = p.ThinkingMode != ""
	c.UpstreamProvider = p.UpstreamProvider
	if p.KeyEnvName != "" {
		if v := os.Getenv(p.KeyEnvName); v != "" {
			c.APIKey = v
			return
		}
	}
	for _, keyEnvName := range p.KeyEnvNames {
		if v := os.Getenv(keyEnvName); v != "" {
			c.APIKey = v
			break
		}
	}
}
