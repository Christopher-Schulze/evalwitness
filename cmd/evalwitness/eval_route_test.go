package main

import (
	"encoding/json"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

// The route block is what makes a published artifact attributable. Two runs in
// this repository had to be moved out of the published set because theirs was
// missing and nobody could say which endpoint produced them. Nothing tested that
// the block is complete or that it records what actually served the run.

func TestRouteBlockCarriesEverythingNeededToAttributeAnArtifact(t *testing.T) {
	cfg := config.Config{
		Provider: "opencode-go-cn",
		Model:    "deepseek-v4-flash",
		BaseURL:  "https://opencode.ai/zen/go/v1",
	}
	route := buildEvalRoute(cfg, "deepseek-v4-flash-0731")
	if route == nil {
		t.Fatal("no route block produced")
	}

	raw, err := json.Marshal(route)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"provider", "model", "base_url", "evaluated_at", "evalwitness_version"} {
		v, ok := got[field]
		if !ok || v == "" {
			t.Fatalf("route block has no %s; an artifact carrying it could not be attributed", field)
		}
	}
	if got["served_model"] != "deepseek-v4-flash-0731" {
		t.Fatalf("served model = %v, want what the endpoint reported rather than what was configured", got["served_model"])
	}
	if got["model"] != "deepseek-v4-flash" {
		t.Fatalf("model = %v, want the configured id preserved alongside the served one", got["model"])
	}
}

func TestRouteTimestampIsUTCAndParsable(t *testing.T) {
	// The timestamp is compared across machines and stored in committed
	// artifacts. A local-time value would make two runs look hours apart.
	route := buildEvalRoute(config.Config{Provider: "p", Model: "m", BaseURL: "https://x"}, "")
	ts, err := time.Parse(time.RFC3339, route.EvaluatedAt)
	if err != nil {
		t.Fatalf("evaluated_at %q is not RFC3339: %v", route.EvaluatedAt, err)
	}
	if _, offset := ts.Zone(); offset != 0 {
		t.Fatalf("evaluated_at carries a %d second offset, want UTC", offset)
	}
}

func TestRouteOmitsServedModelWhenTheEndpointDidNotReportOne(t *testing.T) {
	// An empty served model must not appear as an empty string, which would read
	// as "the endpoint said its name is nothing" rather than "it said nothing".
	route := buildEvalRoute(config.Config{Provider: "p", Model: "m", BaseURL: "https://x"}, "")
	raw, err := json.Marshal(route)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "served_model") {
		t.Fatalf("route block = %s, want served_model omitted when unknown", raw)
	}
}

// limitFlags builds the flag set the eval commands register, so the tests
// exercise the same wiring the binary does rather than a copy of it.
func limitFlags(t *testing.T, args ...string) evalLimitFlags {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	flags := addEvalLimitFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return flags
}

func TestNegativeLimitsAreRejectedByName(t *testing.T) {
	// A negative limit is a typo, and silently treating it as unlimited would
	// remove the bound the user was trying to tighten.
	cases := map[string]string{
		"--max-calls":         "--max-calls=-1",
		"--max-attempts":      "--max-attempts=-1",
		"--max-input-tokens":  "--max-input-tokens=-1",
		"--max-output-tokens": "--max-output-tokens=-1",
		"--max-concurrent":    "--max-concurrent=-1",
		"--max-cost-usd":      "--max-cost-usd=-0.5",
		"--max-duration":      "--max-duration=-1s",
	}
	for name, arg := range cases {
		err := validateEvalLimitFlags(limitFlags(t, arg))
		if err == nil {
			t.Fatalf("%s accepted a negative value", name)
		}
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not name the offending flag %s", err, name)
		}
	}
	if err := validateEvalLimitFlags(limitFlags(t)); err != nil {
		t.Fatalf("default flags rejected: %v", err)
	}
}

func TestUnsetLimitsSeparateLogicalCallsFromRetryAttempts(t *testing.T) {
	plan := evalPlan{
		Calls:                evalEstimateInt{Best: 10, Expected: 50, Worst: 100},
		EstimatedInputTokens: evalEstimateInt{Best: 1000, Expected: 5000, Worst: 10000},
		EstimatedCostUSD:     evalEstimateFloat{Worst: 1.25},
		EstimatedDurationSec: evalEstimateInt{Worst: 60},
	}
	cfg := config.Default()
	applyEvalLimits(&plan, limitFlags(t), cfg)

	if plan.Limits.MaxCalls != plan.Calls.Worst {
		t.Fatalf("logical call limit %d != worst case %d", plan.Limits.MaxCalls, plan.Calls.Worst)
	}
	if plan.Limits.MaxAttempts != plan.Calls.Worst*(cfg.MaxRetries+1) {
		t.Fatalf("attempt limit = %d", plan.Limits.MaxAttempts)
	}
	if plan.Limits.MaxEstimatedInputTokens != plan.EstimatedInputTokens.Worst*(cfg.MaxRetries+1) {
		t.Fatalf("input limit = %d", plan.Limits.MaxEstimatedInputTokens)
	}
	if plan.Limits.MaxReservedOutputTokens <= 0 || plan.Limits.MaxConcurrent != cfg.MaxWorkers {
		t.Fatalf("complete limits = %+v", plan.Limits)
	}
	if plan.HardDurationSeconds < 60 {
		t.Fatalf("hard duration %d s is below the estimated worst case", plan.HardDurationSeconds)
	}
}

func TestExplicitLimitsOverrideTheDerivedOnes(t *testing.T) {
	// A user tightening a bound must get exactly that bound, with no headroom
	// added on top of a number they chose deliberately.
	plan := evalPlan{
		Calls:                evalEstimateInt{Worst: 1000},
		EstimatedInputTokens: evalEstimateInt{Worst: 100000},
		EstimatedCostUSD:     evalEstimateFloat{Worst: 5},
		EstimatedDurationSec: evalEstimateInt{Worst: 600},
	}
	applyEvalLimits(&plan, limitFlags(t,
		"--max-calls=7", "--max-attempts=8", "--max-input-tokens=99", "--max-output-tokens=101", "--max-concurrent=2", "--max-cost-usd=0.5", "--max-duration=30s"), config.Default())

	if plan.Limits.MaxCalls != 7 {
		t.Fatalf("call limit = %d, want the explicit 7", plan.Limits.MaxCalls)
	}
	if plan.Limits.MaxEstimatedInputTokens != 99 {
		t.Fatalf("token limit = %d, want the explicit 99", plan.Limits.MaxEstimatedInputTokens)
	}
	if plan.Limits.MaxAttempts != 8 || plan.Limits.MaxReservedOutputTokens != 101 || plan.Limits.MaxConcurrent != 2 {
		t.Fatalf("explicit attempt/output/concurrency limits = %+v", plan.Limits)
	}
	if plan.Limits.MaxCostUSD != 0.5 {
		t.Fatalf("cost limit = %v, want the explicit 0.5", plan.Limits.MaxCostUSD)
	}
	if plan.Limits.MaxDuration != 30*time.Second {
		t.Fatalf("duration limit = %v, want the explicit 30s", plan.Limits.MaxDuration)
	}
	if plan.HardDurationSeconds != 30 {
		t.Fatalf("hard duration = %d, want 30", plan.HardDurationSeconds)
	}
}

func TestADurationLimitIsNeverZeroSeconds(t *testing.T) {
	// A dry-run-sized plan estimates zero seconds. Rounding that to a zero
	// budget would abort the run before its first call.
	plan := evalPlan{EstimatedDurationSec: evalEstimateInt{Worst: 0}}
	applyEvalLimits(&plan, limitFlags(t), config.Default())
	if plan.Limits.MaxDuration <= 0 || plan.HardDurationSeconds <= 0 {
		t.Fatalf("duration limit = %v (%d s), want a positive bound",
			plan.Limits.MaxDuration, plan.HardDurationSeconds)
	}
}

func TestBudgetSnapshotRoundTripsThroughJSON(t *testing.T) {
	// Limits are published inside every artifact. A shape that does not marshal
	// would drop the bounds a reader needs to judge whether a run was capped.
	plan := evalPlan{
		Calls:                evalEstimateInt{Worst: 100},
		EstimatedInputTokens: evalEstimateInt{Worst: 10000},
		EstimatedCostUSD:     evalEstimateFloat{Worst: 1},
		EstimatedDurationSec: evalEstimateInt{Worst: 60},
	}
	applyEvalLimits(&plan, limitFlags(t), config.Default())
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Limits mode.BudgetLimits `json:"limits"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Limits.MaxCalls != plan.Limits.MaxCalls {
		t.Fatalf("limits did not survive marshalling: %d != %d", got.Limits.MaxCalls, plan.Limits.MaxCalls)
	}
}

func TestDigestStringAloneCannotAuthorizeAStudy(t *testing.T) {
	cfg := config.Default()
	cfg.MaxWorkers = 1
	criteria := []verifier.Criterion{verifier.BuiltinCriteria["code_review"]}
	flags := limitFlags(t, "--study-manifest-digest="+strings.Repeat("a", 64))
	plan := evalPlan{
		Calls: evalEstimateInt{Expected: 1, Worst: 1},
		Limits: mode.BudgetLimits{
			MaxCalls: 1, MaxAttempts: 1, MaxEstimatedInputTokens: 1024,
			MaxReservedOutputTokens: 4096, MaxConcurrent: 1, MaxDuration: time.Minute,
		},
	}
	authorized, err := authorizeEvalPlan(&plan, cfg, criteria, flags, "eval-terminal")
	if err == nil || authorized || plan.Authorization != nil || !strings.Contains(err.Error(), "--study-record") {
		t.Fatalf("digest-only authorization = authorized:%t err:%v", authorized, err)
	}
}
