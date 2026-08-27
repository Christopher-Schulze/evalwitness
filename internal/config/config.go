package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Preset     string
	Provider   string
	WireFormat string
	Model      string
	BaseURL    string
	APIKey     string
	CAFile     string
	// UpstreamProvider pins an OpenRouter preset to one exact inference
	// provider. It is intentionally preset-owned so the provider label remains
	// the canonical route identity and cannot drift through an environment edit.
	UpstreamProvider string
	CacheDir         string
	// LegacyCacheDir is an explicit read-only import root. EvalWitness never
	// writes to this directory.
	LegacyCacheDir string
	MaxWorkers     int
	MaxRetries     int
	TimeoutSec     int
	// MinDispatchIntervalSec spaces completed batch cells before the next cell
	// starts. Zero disables pacing; positive values are locked into run policy.
	MinDispatchIntervalSec int
	LogLevel               string
	DefaultReps            int
	Epsilon                float64

	BiasMitigation         string
	InconsistencyPolicy    string
	MaxPairCalls           int
	PairConfidence         float64
	PairCalibrationSigma   float64
	ExpectedEscalationRate float64

	MultiCriterionBundle bool
	CritiqueThenScore    bool

	SPRT SPRTConfig

	SingleElim bool
	// Selection picks the strategy that turns per-trajectory judgements into one
	// winner. Empty follows SingleElim; "absolute" scores each trajectory once;
	// "joint_absolute" scores the complete candidate set in one response.
	Selection string

	Offline       bool
	AllowInsecure bool

	BillingModel string

	NoRedact bool
	NoCache  bool

	MaxCostUSDPerCall float64
	AuditLog          string

	InputUSDPerM  float64
	CachedUSDPerM float64
	OutputUSDPerM float64

	ContextLimit   int
	EvidenceTokens int
	ThinkingMode   string

	// thinkingModeExplicit records that ThinkingMode was declared by a preset,
	// a config file, or EVALWITNESS_THINKING_MODE rather than inherited from the
	// defaults. Only the inherited default is subject to the model-family
	// heuristic in normalizeThinkingMode.
	thinkingModeExplicit bool

	Stream bool

	// MaxTokens 0 selects the reference-parity 4096-token output ceiling.
	MaxTokens int

	// Temperature for score calls; 1.0 matches the reference implementation.
	Temperature float64

	// Seed is passed to providers that support it; nil means omitted.
	Seed *int

	// RedactPatternsFile is an optional JSON file with extra redaction rules.
	RedactPatternsFile string

	// JudgeMode forces raw-text score extraction and skips logprob requests
	// entirely (LLM-as-a-Judge). For comparison runs and providers without
	// logprobs.
	JudgeMode bool

	// AllowJudgeMode makes doctor report a logprob-less route as usable
	// judge-mode instead of a problem.
	AllowJudgeMode bool

	ReplayFrom      string
	ReplayTo        string
	ReplayOverwrite bool
	RunBudgetState  string
}

type SPRTConfig struct {
	Enabled bool
	Alpha   float64
	Beta    float64
	MaxReps int
	MinReps int
	Sigma   float64
}

// wireFormatDefaultBaseURL gives a sensible default per supported wire format.
// EvalWitness intentionally supports only OpenAI-compatible Chat Completions because
// the verifier depends on output-token logprobs.
var wireFormatDefaultBaseURL = map[string]string{
	"openai": "https://api.openai.com/v1",
}

func Default() Config {
	home, _ := os.UserHomeDir()
	c := Config{
		Provider:            "bai",
		WireFormat:          "openai",
		Model:               "deepseek-v4-flash",
		BaseURL:             "https://api.b.ai/v1",
		CacheDir:            filepath.Join(home, ".cache", "evalwitness"),
		MaxWorkers:          8,
		MaxRetries:          5,
		TimeoutSec:          120,
		LogLevel:            "info",
		DefaultReps:         1,
		Epsilon:             0.02,
		BiasMitigation:      "adaptive",
		InconsistencyPolicy: "flag-only",
		// A third call on an uncertain pair buys nothing measurable. Swept
		// offline over both benchmark caches, a ceiling of 2 against 3 produced
		// zero disagreements on Terminal-Bench and 1-1 on SWE-bench, for 20% and
		// 8% fewer calls. Raise it with EVALWITNESS_MAX_PAIR_CALLS where a complete
		// escalation ladder matters more than the calls.
		MaxPairCalls:           2,
		PairConfidence:         0.6,
		PairCalibrationSigma:   0.05,
		ExpectedEscalationRate: 0.25,
		MultiCriterionBundle:   true,
		CritiqueThenScore:      true,
		SingleElim:             true,
		// The default route is a free tier, so per-call rates are zero and the
		// cost model reports calls rather than dollars. A custom route sets its
		// own rates through EVALWITNESS_INPUT_USD_PER_M and friends.
		BillingModel:   "free-tier",
		ThinkingMode:   "disabled",
		Stream:         true,
		MaxTokens:      0,
		Temperature:    1.0,
		ContextLimit:   1_000_000,
		EvidenceTokens: 32_000,
	}
	c.SPRT = SPRTConfig{
		Enabled: false,
		Alpha:   0.05,
		Beta:    0.05,
		MaxReps: 4,
		MinReps: 1,
		Sigma:   0.15,
	}
	return c
}

// Load resolves execution configuration. Exact replay requires the complete
// local fixture identity but no provider credential; every other execution
// route requires its provider key.
func Load() (Config, error) {
	return load(true)
}

// LoadForDiagnostics resolves and validates configuration without requiring an
// API key. Diagnostic commands use it so they can report a missing key instead
// of failing before they construct their readiness report.
func LoadForDiagnostics() (Config, error) {
	return load(false)
}

func load(requireAPIKey bool) (cfg Config, returnErr error) {
	resolver := newEnvironmentResolver(os.LookupEnv)
	defer func() {
		if warningErr := warnLegacySettings(resolver.legacySettings()); warningErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("write legacy settings warning: %w", warningErr)
		}
	}()
	for _, path := range envFileSearchPaths(resolver) {
		if loadEnvFile(path) == nil {
			break
		}
	}
	c := Default()
	if presetName, ok, _ := resolver.lookupValue(EnvPreset); ok && presetName != "" {
		c.Preset = presetName
		if !applyPreset(&c, presetName) {
			return c, fmt.Errorf("unknown EVALWITNESS_PRESET %q (available: %v)", presetName, PresetNames())
		}
	}
	if err := loadConfigFile(&c, resolver); err != nil {
		return c, err
	}
	c.applyEnv(resolver)
	c.normalizeWireFormat()
	c.normalizeThinkingMode()
	if c.BaseURL == "" {
		c.BaseURL = wireFormatDefaultBaseURL[c.WireFormat]
	}
	// Environment is the highest configuration layer. Resolve against the final
	// provider even when a preset or config file already supplied a credential;
	// otherwise changing the route can silently retain the previous provider's key.
	if apiKey := resolveAPIKey(c.Provider, resolver); apiKey != "" {
		c.APIKey = apiKey
	}
	if err := c.validateWithoutAPIKey(); err != nil {
		return c, err
	}
	if requireAPIKey && c.ReplayFrom == "" {
		if err := c.validateAPIKey(); err != nil {
			return c, err
		}
	}
	return c, nil
}

func (c *Config) applyEnv(resolver *environmentResolver) {
	c.Provider = envStr(resolver, EnvProvider, c.Provider)
	c.WireFormat = envStr(resolver, EnvWireFormat, c.WireFormat)
	c.Model = envStr(resolver, EnvModel, c.Model)
	c.BaseURL = envStr(resolver, EnvBaseURL, c.BaseURL)
	c.CAFile = envStr(resolver, EnvCAFile, c.CAFile)

	if value, ok, source := resolver.lookupValue(EnvCacheDir); ok {
		if source == environmentLegacy {
			c.LegacyCacheDir = value
		} else {
			c.CacheDir = value
		}
	}
	if value, ok := os.LookupEnv(EnvLegacyCacheDir); ok {
		c.LegacyCacheDir = value
	}
	c.MaxWorkers = envInt(resolver, EnvMaxWorkers, c.MaxWorkers)
	c.MaxRetries = envInt(resolver, EnvMaxRetries, c.MaxRetries)
	c.TimeoutSec = envInt(resolver, EnvTimeoutSec, c.TimeoutSec)
	c.MinDispatchIntervalSec = envInt(resolver, EnvMinDispatchInterval, c.MinDispatchIntervalSec)
	c.LogLevel = envStr(resolver, EnvLogLevel, c.LogLevel)
	c.DefaultReps = envInt(resolver, EnvDefaultReps, c.DefaultReps)
	c.Epsilon = envFloat(resolver, EnvEpsilon, c.Epsilon)

	c.BiasMitigation = envStr(resolver, EnvBiasMitigation, c.BiasMitigation)
	c.InconsistencyPolicy = envStr(resolver, EnvInconsistencyPolicy, c.InconsistencyPolicy)
	c.MaxPairCalls = envInt(resolver, EnvMaxPairCalls, c.MaxPairCalls)
	c.PairConfidence = envFloat(resolver, EnvPairConfidence, c.PairConfidence)
	c.PairCalibrationSigma = envFloat(resolver, EnvPairCalibrationSigma, c.PairCalibrationSigma)
	c.ExpectedEscalationRate = envFloat(resolver, EnvExpectedEscalation, c.ExpectedEscalationRate)

	c.MultiCriterionBundle = envBool(resolver, EnvMultiCriterionBundle, c.MultiCriterionBundle)
	c.CritiqueThenScore = envBool(resolver, EnvCritiqueThenScore, c.CritiqueThenScore)

	c.SPRT.Enabled = envBool(resolver, EnvSPRT, c.SPRT.Enabled)
	c.SPRT.Alpha = envFloat(resolver, EnvSPRTAlpha, c.SPRT.Alpha)
	c.SPRT.Beta = envFloat(resolver, EnvSPRTBeta, c.SPRT.Beta)
	c.SPRT.MaxReps = envInt(resolver, EnvSPRTMaxReps, c.SPRT.MaxReps)
	c.SPRT.MinReps = envInt(resolver, EnvSPRTMinReps, c.SPRT.MinReps)
	c.SPRT.Sigma = envFloat(resolver, EnvSPRTSigma, c.SPRT.Sigma)

	c.SingleElim = envBool(resolver, EnvSingleElim, c.SingleElim)
	c.Selection = envStr(resolver, EnvSelection, c.Selection)
	c.Offline = envBool(resolver, EnvOffline, c.Offline)
	c.AllowInsecure = envBool(resolver, EnvAllowInsecure, c.AllowInsecure)
	c.BillingModel = envStr(resolver, EnvBillingModel, c.BillingModel)
	c.NoRedact = envBool(resolver, EnvNoRedact, c.NoRedact)
	c.NoCache = envBool(resolver, EnvNoCache, c.NoCache)

	c.MaxCostUSDPerCall = envFloat(resolver, EnvMaxCostUSDPerCall, c.MaxCostUSDPerCall)
	c.AuditLog = envStr(resolver, EnvAuditLog, c.AuditLog)
	c.InputUSDPerM = envFloat(resolver, EnvInputUSDPerM, c.InputUSDPerM)
	c.CachedUSDPerM = envFloat(resolver, EnvCachedUSDPerM, c.CachedUSDPerM)
	c.OutputUSDPerM = envFloat(resolver, EnvOutputUSDPerM, c.OutputUSDPerM)

	c.ContextLimit = envInt(resolver, EnvContextLimit, c.ContextLimit)
	c.EvidenceTokens = envInt(resolver, EnvEvidenceTokens, c.EvidenceTokens)
	if value, ok, _ := resolver.lookupValue(EnvThinkingMode); ok && value != "" {
		c.ThinkingMode = value
		c.thinkingModeExplicit = true
	}

	c.Stream = envBool(resolver, EnvStream, c.Stream)
	c.MaxTokens = envInt(resolver, EnvMaxTokens, c.MaxTokens)
	c.Temperature = envFloat(resolver, EnvTemperature, c.Temperature)
	if value, ok, _ := resolver.lookupValue(EnvSeed); ok && value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			c.Seed = &n
		}
	}
	c.RedactPatternsFile = envStr(resolver, EnvRedactPatterns, c.RedactPatternsFile)
	c.JudgeMode = envBool(resolver, EnvJudgeMode, c.JudgeMode)
	c.AllowJudgeMode = envBool(resolver, EnvAllowJudgeMode, c.AllowJudgeMode)
	c.ReplayFrom = envStr(resolver, EnvReplayFrom, c.ReplayFrom)
	c.ReplayTo = envStr(resolver, EnvReplayTo, c.ReplayTo)
	c.ReplayOverwrite = envBool(resolver, EnvReplayOverwrite, c.ReplayOverwrite)
	c.RunBudgetState = envStr(resolver, EnvRunBudgetState, c.RunBudgetState)
}

func (c *Config) normalizeWireFormat() {
	if c.WireFormat != "" {
		return
	}
	c.WireFormat = "openai"
}

func (c *Config) normalizeThinkingMode() {
	if c.ThinkingMode == "default" {
		c.ThinkingMode = ""
		return
	}
	if c.ThinkingMode == "" {
		return
	}
	// A route that declares its thinking mode knows its own model best: presets,
	// config files and EVALWITNESS_THINKING_MODE are never overridden. Reasoning
	// models otherwise spend the entire output
	// budget on reasoning tokens and return no score-token logprobs at all.
	if c.thinkingModeExplicit {
		return
	}
	// Only the inherited default follows the model family, not the hosting
	// gateway: keep it for any DeepSeek model regardless of provider label.
	if c.Provider == "deepseek" || strings.Contains(c.BaseURL, "api.deepseek.com") || strings.Contains(strings.ToLower(c.Model), "deepseek") {
		return
	}
	c.ThinkingMode = ""
}

// resolveAPIKey looks up the key for a provider label, then falls back to
// progressively shorter prefixes of it. A label like "opencode-go-cn" names one
// account's regional opt-in rather than a separate vendor, and its key lives
// under the vendor's name, so OPENCODE_GO_CN_API_KEY and OPENCODE_GO_API_KEY
// both resolve it. EVALWITNESS_API_KEY is the last resort.
func resolveAPIKey(provider string, resolver *environmentResolver) string {
	parts := strings.Split(provider, "-")
	for i := len(parts); i > 0; i-- {
		name := strings.ToUpper(strings.Join(parts[:i], "_")) + "_API_KEY"
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	value, _, _ := resolver.lookupValue(EnvAPIKey)
	return value
}

func (c *Config) validate() error {
	if err := c.validateWithoutAPIKey(); err != nil {
		return err
	}
	return c.validateAPIKey()
}

func (c *Config) validateWithoutAPIKey() error {
	if c.BaseURL == "" {
		if _, ok := wireFormatDefaultBaseURL[c.WireFormat]; !ok {
			return fmt.Errorf("unknown wire format %q and no EVALWITNESS_BASE_URL set", c.WireFormat)
		}
		return errors.New("EVALWITNESS_BASE_URL is empty")
	}
	if _, ok := wireFormatDefaultBaseURL[c.WireFormat]; !ok {
		return fmt.Errorf("EVALWITNESS_WIRE_FORMAT must be openai, got %q", c.WireFormat)
	}
	if c.Model == "" {
		return errors.New("EVALWITNESS_MODEL is required")
	}
	if !strings.HasPrefix(c.BaseURL, "https://") && !strings.HasPrefix(c.BaseURL, "http://") {
		return fmt.Errorf("invalid base URL: %q", c.BaseURL)
	}
	if strings.HasPrefix(c.BaseURL, "http://") && !c.AllowInsecure {
		return fmt.Errorf("plain HTTP base URL requires EVALWITNESS_ALLOW_INSECURE=true (got %q)", c.BaseURL)
	}
	if c.MaxWorkers <= 0 {
		return errors.New("EVALWITNESS_MAX_WORKERS must be > 0")
	}
	if c.MaxRetries <= 0 {
		return errors.New("EVALWITNESS_MAX_RETRIES must be > 0")
	}
	if c.MinDispatchIntervalSec < 0 {
		return errors.New("EVALWITNESS_MIN_DISPATCH_INTERVAL_SEC must be >= 0")
	}
	if c.DefaultReps <= 0 {
		return errors.New("EVALWITNESS_DEFAULT_REPS must be > 0")
	}
	if c.SPRT.Enabled && c.SPRT.MaxReps < c.SPRT.MinReps {
		return errors.New("EVALWITNESS_SPRT_MAX_REPS must be >= EVALWITNESS_SPRT_MIN_REPS")
	}
	if c.MaxPairCalls <= 0 || c.MaxPairCalls > 4 {
		return errors.New("EVALWITNESS_MAX_PAIR_CALLS must be between 1 and 4")
	}
	if c.PairConfidence <= 0.5 || c.PairConfidence >= 1 {
		return errors.New("EVALWITNESS_PAIR_CONFIDENCE must be between 0.5 and 1")
	}
	if c.PairCalibrationSigma < 0 {
		return errors.New("EVALWITNESS_PAIR_CALIBRATION_SIGMA must be >= 0")
	}
	if c.ExpectedEscalationRate < 0 || c.ExpectedEscalationRate > 1 {
		return errors.New("EVALWITNESS_EXPECTED_ESCALATION_RATE must be between 0 and 1")
	}
	if c.Selection != "" && c.Selection != "absolute" && c.Selection != "joint_absolute" {
		return fmt.Errorf("EVALWITNESS_SELECTION must be empty, \"absolute\", or \"joint_absolute\", got %q", c.Selection)
	}
	if c.EvidenceTokens <= 0 {
		return errors.New("EVALWITNESS_EVIDENCE_TOKENS must be > 0")
	}
	switch c.BiasMitigation {
	case "adaptive", "both", "single", "disabled":
	default:
		return fmt.Errorf("EVALWITNESS_BIAS_MITIGATION must be adaptive/both/single/disabled, got %q", c.BiasMitigation)
	}
	switch c.InconsistencyPolicy {
	case "adaptive", "flag-only":
	default:
		return fmt.Errorf("EVALWITNESS_INCONSISTENCY_POLICY must be adaptive/flag-only, got %q", c.InconsistencyPolicy)
	}
	switch c.ThinkingMode {
	case "", "default", "enabled", "disabled":
	default:
		return fmt.Errorf("EVALWITNESS_THINKING_MODE must be default/enabled/disabled, got %q", c.ThinkingMode)
	}
	return nil
}

func (c *Config) validateAPIKey() error {
	if c.Provider != "ollama" && c.APIKey == "" {
		return fmt.Errorf("API key required for provider %q (set %s_API_KEY or EVALWITNESS_API_KEY)",
			c.Provider, strings.ToUpper(strings.ReplaceAll(c.Provider, "-", "_")))
	}
	return nil
}

func envFileSearchPaths(resolver *environmentResolver) []string {
	var paths []string
	if v, ok, _ := resolver.lookupValue(EnvEnvFile); ok && v != "" {
		paths = append(paths, v)
	}
	paths = append(paths, ".env")
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "evalwitness", ".env"))
		paths = append(paths, filepath.Join(home, ".config", "logprobe", ".env"))
	}
	return paths
}

func loadEnvFile(path string) (returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && returnErr == nil {
			returnErr = closeErr
		}
	}()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rawKey, rawVal, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(rawKey)
		val := strings.TrimSpace(rawVal)
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return err
			}
		}
	}
	return s.Err()
}

func envStr(resolver *environmentResolver, key EnvironmentKey, def string) string {
	if v, ok, _ := resolver.lookupValue(key); ok {
		return v
	}
	return def
}

func envInt(resolver *environmentResolver, key EnvironmentKey, def int) int {
	if v, ok, _ := resolver.lookupValue(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(resolver *environmentResolver, key EnvironmentKey, def bool) bool {
	if v, ok, _ := resolver.lookupValue(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envFloat(resolver *environmentResolver, key EnvironmentKey, def float64) float64 {
	if v, ok, _ := resolver.lookupValue(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
