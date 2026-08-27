package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// fileConfig matches the on-disk schema. All fields optional. Provider-specific
// keys (api_key, base_url) live under provider-named tables.
type fileConfig struct {
	Provider               string         `toml:"provider" json:"provider"`
	WireFormat             string         `toml:"wire_format" json:"wire_format"`
	Model                  string         `toml:"model" json:"model"`
	BaseURL                string         `toml:"base_url" json:"base_url"`
	APIKey                 string         `toml:"api_key" json:"api_key"`
	CAFile                 string         `toml:"ca_file" json:"ca_file"`
	CacheDir               string         `toml:"cache_dir" json:"cache_dir"`
	LegacyCacheDir         string         `toml:"legacy_cache_dir" json:"legacy_cache_dir"`
	MaxWorkers             int            `toml:"max_workers" json:"max_workers"`
	MaxRetries             int            `toml:"max_retries" json:"max_retries"`
	TimeoutSec             int            `toml:"timeout_sec" json:"timeout_sec"`
	LogLevel               string         `toml:"log_level" json:"log_level"`
	DefaultReps            int            `toml:"default_reps" json:"default_reps"`
	Epsilon                float64        `toml:"epsilon" json:"epsilon"`
	BiasMitigation         string         `toml:"bias_mitigation" json:"bias_mitigation"`
	InconsistencyPolicy    string         `toml:"inconsistency_policy" json:"inconsistency_policy"`
	MaxPairCalls           int            `toml:"max_pair_calls" json:"max_pair_calls"`
	PairConfidence         float64        `toml:"pair_confidence" json:"pair_confidence"`
	PairCalibrationSigma   float64        `toml:"pair_calibration_sigma" json:"pair_calibration_sigma"`
	ExpectedEscalationRate float64        `toml:"expected_escalation_rate" json:"expected_escalation_rate"`
	MultiCriterionBundle   *bool          `toml:"multi_criterion_bundle" json:"multi_criterion_bundle"`
	CritiqueThenScore      *bool          `toml:"critique_then_score" json:"critique_then_score"`
	SingleElim             *bool          `toml:"single_elim" json:"single_elim"`
	Offline                *bool          `toml:"offline" json:"offline"`
	AllowInsecure          *bool          `toml:"allow_insecure" json:"allow_insecure"`
	BillingModel           string         `toml:"billing_model" json:"billing_model"`
	NoRedact               *bool          `toml:"no_redact" json:"no_redact"`
	NoCache                *bool          `toml:"no_cache" json:"no_cache"`
	MaxCostUSDPerCall      float64        `toml:"max_cost_usd_per_call" json:"max_cost_usd_per_call"`
	AuditLog               string         `toml:"audit_log" json:"audit_log"`
	InputUSDPerM           float64        `toml:"input_usd_per_m" json:"input_usd_per_m"`
	CachedUSDPerM          float64        `toml:"cached_usd_per_m" json:"cached_usd_per_m"`
	OutputUSDPerM          float64        `toml:"output_usd_per_m" json:"output_usd_per_m"`
	ContextLimit           int            `toml:"context_limit" json:"context_limit"`
	EvidenceTokens         int            `toml:"evidence_tokens" json:"evidence_tokens"`
	ThinkingMode           string         `toml:"thinking_mode" json:"thinking_mode"`
	Stream                 *bool          `toml:"stream" json:"stream"`
	MaxTokens              int            `toml:"max_tokens" json:"max_tokens"`
	ReplayFrom             string         `toml:"replay_from" json:"replay_from"`
	ReplayTo               string         `toml:"replay_to" json:"replay_to"`
	ReplayOverwrite        *bool          `toml:"replay_overwrite" json:"replay_overwrite"`
	SPRT                   *fileSPRT      `toml:"sprt" json:"sprt"`
	ProviderKeys           map[string]any `toml:"providers" json:"providers"`
}

type fileSPRT struct {
	Enabled *bool   `toml:"enabled" json:"enabled"`
	Alpha   float64 `toml:"alpha" json:"alpha"`
	Beta    float64 `toml:"beta" json:"beta"`
	MaxReps int     `toml:"max_reps" json:"max_reps"`
	MinReps int     `toml:"min_reps" json:"min_reps"`
	Sigma   float64 `toml:"sigma" json:"sigma"`
}

type configFileCandidate struct {
	Path   string
	Legacy bool
}

// configFileSearchPaths returns paths to try in order; first existing wins.
// Legacy names are fallback-only and are never used as write destinations.
func configFileSearchPaths(resolver *environmentResolver) []configFileCandidate {
	var out []configFileCandidate
	var legacyExplicit []configFileCandidate
	if value, ok, source := resolver.lookupValue(EnvConfigFile); ok && value != "" {
		candidate := configFileCandidate{Path: value, Legacy: source == environmentLegacy}
		if candidate.Legacy {
			legacyExplicit = append(legacyExplicit, candidate)
		} else {
			out = append(out, candidate)
		}
	}
	var legacyHome []configFileCandidate
	if home, err := os.UserHomeDir(); err == nil {
		canonicalBase := filepath.Join(home, ".config", "evalwitness")
		legacyBase := filepath.Join(home, ".config", "logprobe")
		out = append(out,
			configFileCandidate{Path: filepath.Join(canonicalBase, "config.toml")},
			configFileCandidate{Path: filepath.Join(canonicalBase, "config.json")},
		)
		legacyHome = append(legacyHome,
			configFileCandidate{Path: filepath.Join(legacyBase, "config.toml"), Legacy: true},
			configFileCandidate{Path: filepath.Join(legacyBase, "config.json"), Legacy: true},
		)
	}
	out = append(out,
		configFileCandidate{Path: "evalwitness.toml"},
		configFileCandidate{Path: "evalwitness.json"},
	)
	out = append(out, legacyExplicit...)
	out = append(out, legacyHome...)
	out = append(out,
		configFileCandidate{Path: "logprobe.toml", Legacy: true},
		configFileCandidate{Path: "logprobe.json", Legacy: true},
	)
	return out
}

func loadConfigFile(c *Config, resolver *environmentResolver) error {
	for _, candidate := range configFileSearchPaths(resolver) {
		data, err := os.ReadFile(candidate.Path)
		if err != nil {
			continue
		}
		fc, err := parseConfigFile(candidate.Path, data)
		if err != nil {
			return fmt.Errorf("config file %s: %w", candidate.Path, err)
		}
		if candidate.Legacy && fc.CacheDir != "" {
			c.LegacyCacheDir = fc.CacheDir
			fc.CacheDir = ""
		}
		if err := applyFileConfig(c, fc); err != nil {
			return fmt.Errorf("apply config file %s: %w", candidate.Path, err)
		}
		return nil
	}
	return nil
}

func parseConfigFile(path string, data []byte) (fileConfig, error) {
	var fc fileConfig
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &fc); err != nil {
			return fc, err
		}
	case ".toml", "":
		if err := toml.Unmarshal(data, &fc); err != nil {
			return fc, err
		}
	default:
		return fc, errors.New("unsupported config file extension (use .toml or .json)")
	}
	return fc, nil
}

func applyFileConfig(c *Config, fc fileConfig) error {
	setStr(&c.Provider, fc.Provider)
	setStr(&c.WireFormat, fc.WireFormat)
	setStr(&c.Model, fc.Model)
	setStr(&c.BaseURL, fc.BaseURL)
	setStr(&c.APIKey, fc.APIKey)
	setStr(&c.CAFile, fc.CAFile)
	setStr(&c.CacheDir, fc.CacheDir)
	setStr(&c.LegacyCacheDir, fc.LegacyCacheDir)
	setInt(&c.MaxWorkers, fc.MaxWorkers)
	setInt(&c.MaxRetries, fc.MaxRetries)
	setInt(&c.TimeoutSec, fc.TimeoutSec)
	setStr(&c.LogLevel, fc.LogLevel)
	setInt(&c.DefaultReps, fc.DefaultReps)
	setFloat(&c.Epsilon, fc.Epsilon)
	setStr(&c.BiasMitigation, fc.BiasMitigation)
	setStr(&c.InconsistencyPolicy, fc.InconsistencyPolicy)
	setInt(&c.MaxPairCalls, fc.MaxPairCalls)
	setFloat(&c.PairConfidence, fc.PairConfidence)
	setFloat(&c.PairCalibrationSigma, fc.PairCalibrationSigma)
	setFloat(&c.ExpectedEscalationRate, fc.ExpectedEscalationRate)
	setBoolPtr(&c.MultiCriterionBundle, fc.MultiCriterionBundle)
	setBoolPtr(&c.CritiqueThenScore, fc.CritiqueThenScore)
	setBoolPtr(&c.SingleElim, fc.SingleElim)
	setBoolPtr(&c.Offline, fc.Offline)
	setBoolPtr(&c.AllowInsecure, fc.AllowInsecure)
	setStr(&c.BillingModel, fc.BillingModel)
	setBoolPtr(&c.NoRedact, fc.NoRedact)
	setBoolPtr(&c.NoCache, fc.NoCache)
	setFloat(&c.MaxCostUSDPerCall, fc.MaxCostUSDPerCall)
	setStr(&c.AuditLog, fc.AuditLog)
	setFloat(&c.InputUSDPerM, fc.InputUSDPerM)
	setFloat(&c.CachedUSDPerM, fc.CachedUSDPerM)
	setFloat(&c.OutputUSDPerM, fc.OutputUSDPerM)
	setInt(&c.ContextLimit, fc.ContextLimit)
	setInt(&c.EvidenceTokens, fc.EvidenceTokens)
	setStr(&c.ThinkingMode, fc.ThinkingMode)
	if fc.ThinkingMode != "" {
		c.thinkingModeExplicit = true
	}
	setBoolPtr(&c.Stream, fc.Stream)
	setInt(&c.MaxTokens, fc.MaxTokens)
	setStr(&c.ReplayFrom, fc.ReplayFrom)
	setStr(&c.ReplayTo, fc.ReplayTo)
	setBoolPtr(&c.ReplayOverwrite, fc.ReplayOverwrite)
	if fc.SPRT != nil {
		setBoolPtr(&c.SPRT.Enabled, fc.SPRT.Enabled)
		setFloat(&c.SPRT.Alpha, fc.SPRT.Alpha)
		setFloat(&c.SPRT.Beta, fc.SPRT.Beta)
		setInt(&c.SPRT.MaxReps, fc.SPRT.MaxReps)
		setInt(&c.SPRT.MinReps, fc.SPRT.MinReps)
		setFloat(&c.SPRT.Sigma, fc.SPRT.Sigma)
	}
	for name, raw := range fc.ProviderKeys {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		key, _ := m["api_key"].(string)
		if key == "" {
			continue
		}
		envName := strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_API_KEY"
		if _, exists := os.LookupEnv(envName); !exists {
			if err := os.Setenv(envName, key); err != nil {
				return fmt.Errorf("set provider key environment %s: %w", envName, err)
			}
		}
	}
	return nil
}

func setStr(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}

func setInt(dst *int, v int) {
	if v != 0 {
		*dst = v
	}
}

func setFloat(dst *float64, v float64) {
	if v != 0 {
		*dst = v
	}
}

func setBoolPtr(dst *bool, v *bool) {
	if v != nil {
		*dst = *v
	}
}
