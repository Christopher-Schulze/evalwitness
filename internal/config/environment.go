package config

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	CanonicalEnvironmentPrefix = "EVALWITNESS_"
	LegacyEnvironmentPrefix    = "LOGPROBE_"
)

// EnvironmentKey is a canonical EvalWitness environment setting. Every key in
// this registry has an automatically derived LOGPROBE_* compatibility name.
type EnvironmentKey string

const (
	EnvAllowInsecure        EnvironmentKey = "EVALWITNESS_ALLOW_INSECURE"
	EnvAllowJudgeMode       EnvironmentKey = "EVALWITNESS_ALLOW_JUDGE_MODE"
	EnvAPIKey               EnvironmentKey = "EVALWITNESS_API_KEY"
	EnvAuditLog             EnvironmentKey = "EVALWITNESS_AUDIT_LOG"
	EnvBaseURL              EnvironmentKey = "EVALWITNESS_BASE_URL"
	EnvBiasMitigation       EnvironmentKey = "EVALWITNESS_BIAS_MITIGATION"
	EnvBillingModel         EnvironmentKey = "EVALWITNESS_BILLING_MODEL"
	EnvCachedUSDPerM        EnvironmentKey = "EVALWITNESS_CACHED_USD_PER_M"
	EnvCacheDir             EnvironmentKey = "EVALWITNESS_CACHE_DIR"
	EnvCAFile               EnvironmentKey = "EVALWITNESS_CA_FILE"
	EnvConfigFile           EnvironmentKey = "EVALWITNESS_CONFIG_FILE"
	EnvContextLimit         EnvironmentKey = "EVALWITNESS_CONTEXT_LIMIT"
	EnvCritiqueThenScore    EnvironmentKey = "EVALWITNESS_CRITIQUE_THEN_SCORE"
	EnvDefaultReps          EnvironmentKey = "EVALWITNESS_DEFAULT_REPS"
	EnvEnvFile              EnvironmentKey = "EVALWITNESS_ENV_FILE"
	EnvEpsilon              EnvironmentKey = "EVALWITNESS_EPSILON"
	EnvEvidenceTokens       EnvironmentKey = "EVALWITNESS_EVIDENCE_TOKENS"
	EnvExpectedEscalation   EnvironmentKey = "EVALWITNESS_EXPECTED_ESCALATION_RATE"
	EnvInconsistencyPolicy  EnvironmentKey = "EVALWITNESS_INCONSISTENCY_POLICY"
	EnvInputUSDPerM         EnvironmentKey = "EVALWITNESS_INPUT_USD_PER_M"
	EnvJudgeMode            EnvironmentKey = "EVALWITNESS_JUDGE_MODE"
	EnvLogLevel             EnvironmentKey = "EVALWITNESS_LOG_LEVEL"
	EnvMinDispatchInterval  EnvironmentKey = "EVALWITNESS_MIN_DISPATCH_INTERVAL_SEC"
	EnvMaxCostUSDPerCall    EnvironmentKey = "EVALWITNESS_MAX_COST_USD_PER_CALL"
	EnvMaxPairCalls         EnvironmentKey = "EVALWITNESS_MAX_PAIR_CALLS"
	EnvMaxRetries           EnvironmentKey = "EVALWITNESS_MAX_RETRIES"
	EnvMaxTokens            EnvironmentKey = "EVALWITNESS_MAX_TOKENS"
	EnvMaxWorkers           EnvironmentKey = "EVALWITNESS_MAX_WORKERS"
	EnvModel                EnvironmentKey = "EVALWITNESS_MODEL"
	EnvMultiCriterionBundle EnvironmentKey = "EVALWITNESS_MULTI_CRITERION_BUNDLE"
	EnvNoCache              EnvironmentKey = "EVALWITNESS_NO_CACHE"
	EnvNoRedact             EnvironmentKey = "EVALWITNESS_NO_REDACT"
	EnvOffline              EnvironmentKey = "EVALWITNESS_OFFLINE"
	EnvOutputUSDPerM        EnvironmentKey = "EVALWITNESS_OUTPUT_USD_PER_M"
	EnvPairCalibrationSigma EnvironmentKey = "EVALWITNESS_PAIR_CALIBRATION_SIGMA"
	EnvPairConfidence       EnvironmentKey = "EVALWITNESS_PAIR_CONFIDENCE"
	EnvPreset               EnvironmentKey = "EVALWITNESS_PRESET"
	EnvProvider             EnvironmentKey = "EVALWITNESS_PROVIDER"
	EnvRedactPatterns       EnvironmentKey = "EVALWITNESS_REDACT_PATTERNS"
	EnvReplayFrom           EnvironmentKey = "EVALWITNESS_REPLAY_FROM"
	EnvReplayOverwrite      EnvironmentKey = "EVALWITNESS_REPLAY_OVERWRITE"
	EnvReplayTo             EnvironmentKey = "EVALWITNESS_REPLAY_TO"
	EnvRunBudgetState       EnvironmentKey = "EVALWITNESS_RUN_BUDGET_STATE"
	EnvSeed                 EnvironmentKey = "EVALWITNESS_SEED"
	EnvSelection            EnvironmentKey = "EVALWITNESS_SELECTION"
	EnvSingleElim           EnvironmentKey = "EVALWITNESS_SINGLE_ELIM"
	EnvSPRT                 EnvironmentKey = "EVALWITNESS_SPRT"
	EnvSPRTAlpha            EnvironmentKey = "EVALWITNESS_SPRT_ALPHA"
	EnvSPRTBeta             EnvironmentKey = "EVALWITNESS_SPRT_BETA"
	EnvSPRTMaxReps          EnvironmentKey = "EVALWITNESS_SPRT_MAX_REPS"
	EnvSPRTMinReps          EnvironmentKey = "EVALWITNESS_SPRT_MIN_REPS"
	EnvSPRTSigma            EnvironmentKey = "EVALWITNESS_SPRT_SIGMA"
	EnvStream               EnvironmentKey = "EVALWITNESS_STREAM"
	EnvTemperature          EnvironmentKey = "EVALWITNESS_TEMPERATURE"
	EnvThinkingMode         EnvironmentKey = "EVALWITNESS_THINKING_MODE"
	EnvTimeoutSec           EnvironmentKey = "EVALWITNESS_TIMEOUT_SEC"
	EnvWireFormat           EnvironmentKey = "EVALWITNESS_WIRE_FORMAT"
)

const EnvLegacyCacheDir = "EVALWITNESS_LEGACY_CACHE_DIR"

var runtimeEnvironmentKeys = []EnvironmentKey{
	EnvAllowInsecure, EnvAllowJudgeMode, EnvAPIKey, EnvAuditLog, EnvBaseURL,
	EnvBiasMitigation, EnvBillingModel, EnvCachedUSDPerM, EnvCacheDir, EnvCAFile,
	EnvConfigFile, EnvContextLimit, EnvCritiqueThenScore, EnvDefaultReps,
	EnvEnvFile, EnvEpsilon, EnvEvidenceTokens, EnvExpectedEscalation,
	EnvInconsistencyPolicy, EnvInputUSDPerM, EnvJudgeMode, EnvLogLevel, EnvMinDispatchInterval,
	EnvMaxCostUSDPerCall, EnvMaxPairCalls, EnvMaxRetries, EnvMaxTokens,
	EnvMaxWorkers, EnvModel, EnvMultiCriterionBundle, EnvNoCache, EnvNoRedact,
	EnvOffline, EnvOutputUSDPerM, EnvPairCalibrationSigma, EnvPairConfidence,
	EnvPreset, EnvProvider, EnvRedactPatterns, EnvReplayFrom, EnvReplayOverwrite,
	EnvReplayTo, EnvRunBudgetState, EnvSeed, EnvSelection, EnvSingleElim, EnvSPRT,
	EnvSPRTAlpha, EnvSPRTBeta, EnvSPRTMaxReps, EnvSPRTMinReps, EnvSPRTSigma,
	EnvStream, EnvTemperature, EnvThinkingMode, EnvTimeoutSec, EnvWireFormat,
}

type environmentSource uint8

const (
	environmentAbsent environmentSource = iota
	environmentCanonical
	environmentLegacy
)

type environmentResolver struct {
	lookup      func(string) (string, bool)
	legacyNames map[string]struct{}
}

func newEnvironmentResolver(lookup func(string) (string, bool)) *environmentResolver {
	return &environmentResolver{lookup: lookup, legacyNames: make(map[string]struct{})}
}

func (r *environmentResolver) lookupValue(key EnvironmentKey) (string, bool, environmentSource) {
	canonical := string(key)
	if value, ok := r.lookup(canonical); ok {
		return value, true, environmentCanonical
	}
	legacy := legacyEnvironmentName(key)
	if value, ok := r.lookup(legacy); ok {
		r.legacyNames[legacy] = struct{}{}
		return value, true, environmentLegacy
	}
	return "", false, environmentAbsent
}

func (r *environmentResolver) legacySettings() []string {
	names := make([]string, 0, len(r.legacyNames))
	for name := range r.legacyNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func legacyEnvironmentName(key EnvironmentKey) string {
	return LegacyEnvironmentPrefix + strings.TrimPrefix(string(key), CanonicalEnvironmentPrefix)
}

var (
	legacyWarningOnce             = &sync.Once{}
	legacyWarningWriter io.Writer = os.Stderr
)

func warnLegacySettings(names []string) error {
	if len(names) == 0 {
		return nil
	}
	var writeErr error
	legacyWarningOnce.Do(func() {
		_, writeErr = fmt.Fprintln(legacyWarningWriter, formatLegacyWarning(names))
	})
	return writeErr
}

func formatLegacyWarning(names []string) string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	return "warning: legacy LOGPROBE_* settings consumed: " + strings.Join(sorted, ", ") +
		"; migrate to EVALWITNESS_*; values were not logged"
}
