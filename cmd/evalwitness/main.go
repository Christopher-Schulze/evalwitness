package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/cache"
	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/log"
	"github.com/Christopher-Schulze/evalwitness/internal/mcp"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/product"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/replay"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const version = product.Version

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version)
	case "probe":
		os.Exit(runProbe(os.Args[2:]))
	case "attest":
		os.Exit(runAttest(os.Args[2:]))
	case "verify":
		os.Exit(runVerify(os.Args[2:]))
	case "doctor":
		os.Exit(runDoctor(os.Args[2:]))
	case "presets":
		os.Exit(runPresets(os.Args[2:]))
	case "mcp-serve":
		os.Exit(runMCP(os.Args[2:]))
	case "cache":
		os.Exit(runCache(os.Args[2:]))
	case "archive":
		os.Exit(runArchive(os.Args[2:]))
	case "artifact":
		os.Exit(runArtifact(os.Args[2:]))
	case "replay":
		os.Exit(runReplay(os.Args[2:]))
	case "eval-terminal":
		os.Exit(runEvalTerminal(os.Args[2:]))
	case "eval-swebench":
		os.Exit(runEvalSWEbench(os.Args[2:]))
	case "bon":
		os.Exit(runBon(os.Args[2:]))
	case "fidelity":
		os.Exit(runFidelity(os.Args[2:]))
	case "protocol":
		os.Exit(runProtocol(os.Args[2:]))
	case "trace":
		os.Exit(runTrace(os.Args[2:]))
	case "design":
		os.Exit(runDesign(os.Args[2:]))
	case "study":
		os.Exit(runStudy(os.Args[2:]))
	case "stress":
		os.Exit(runStress(os.Args[2:]))
	case "mutation":
		os.Exit(runMutation(os.Args[2:]))
	case "relation":
		os.Exit(runRelation(os.Args[2:]))
	case "outcome":
		os.Exit(runOutcome(os.Args[2:]))
	case "agent-study":
		os.Exit(runAgentStudy(os.Args[2:]))
	case "capsule":
		os.Exit(runCapsule(os.Args[2:]))
	case "claim":
		os.Exit(runClaim(os.Args[2:]))
	case "profile":
		os.Exit(runProfile(os.Args[2:]))
	case "calibration":
		os.Exit(runCalibration(os.Args[2:]))
	case "registry":
		os.Exit(runRegistry(os.Args[2:]))
	case "audit":
		os.Exit(runAudit(os.Args[2:]))
	case "release":
		os.Exit(runRelease(os.Args[2:]))
	case "help", "--help", "-h":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w io.Writer) {
	if _, err := fmt.Fprintln(w, `evalwitness - reproducible verifier audit lab for coding-agent trajectories

usage:
  evalwitness version
  evalwitness doctor [--live] [--output json|text]
  evalwitness presets [--output json|text]
  evalwitness probe [--provider X] [--wire-format openai] [--model Y]
  evalwitness attest [hard-limit flags] [--authorize DIGEST]
  evalwitness verify --mode <delta|absolute|pairwise> --task <inline|@file> [mode-specific flags]
  evalwitness mcp-serve
  evalwitness cache <stats|clear --scope responses|capabilities|all>
  evalwitness archive <inspect|extract> --source file.tar.gz [--source ...] [--destination path]
  evalwitness artifact scan --class <public|sensitive> --path PATH [--reviewed-findings MANIFEST]
  evalwitness replay bundle seal-policy --source draft.json --repository-root PATH --producer-binary PATH --redistribution-evidence PATH --capture name=exact.jsonl [--capture ...]
  evalwitness replay bundle build --policy policy.json --repository-root PATH --producer-binary PATH --redistribution-evidence PATH --destination PATH --capture name=exact.jsonl [--capture ...] [--archive PATH] [--reviewed-findings PATH]
  evalwitness replay bundle verify --source PATH [--reviewed-findings PATH]
  evalwitness replay capture-run attest --capture exact.jsonl --authorized-calls N [--output PATH]
  evalwitness replay capture-run verify --capture exact.jsonl --attestation PATH
  evalwitness replay capture-run stamp --capture exact.jsonl --destination stamped.jsonl --stamp stamp.json [--output PATH]
  evalwitness replay capture-run admit --capture exact.jsonl --authorized-calls N [--output PATH]
  evalwitness replay study bind --capture exact.jsonl --authorized-calls N --attestation PATH --admission PATH --claim-ledger PATH [--bundle-policy PATH --study-record PATH --offline-analysis PATH --output PATH]
  evalwitness replay study portfolio --bind PATH --claim-ledger PATH [--output PATH]
  evalwitness replay study capsule build --base-capsule PATH --study-manifest PATH --study-record PATH --live-authorization PATH --route-attestation PATH --capture-run-attestation PATH --admission PATH --offline-analysis PATH --destination PATH [--archive PATH --claim-ledger PATH --challenge-pack PATH]
  evalwitness replay study capsule verify --base-capsule PATH --source PATH --claim-ledger PATH --challenge-pack PATH
  evalwitness calibration evaluate --observations FILE --threshold F --target-risk F --min-coverage F [--seed N] [--artifact FILE --route SCOPE --domain SCOPE] [--inventory FILE --root DIR]
  evalwitness calibration seal --artifact FILE --calibrator FILE
  evalwitness calibration verify --artifact FILE --calibrator FILE
  evalwitness calibration apply --artifact FILE --route SCOPE --domain SCOPE [--calibrator FILE]
  evalwitness calibration bind-049 --split FILE --study FILE
  evalwitness calibration bind-034 --inventory FILE [--root DIR]
  evalwitness registry validate-intake --entry FILE [--catalog FILE]
  evalwitness registry preflight --entry FILE [--catalog FILE]
  evalwitness registry template
  evalwitness registry refresh --catalog FILE
  evalwitness registry review-checklist
  evalwitness registry render-matrix --catalog FILE [--history FILE]
  evalwitness registry render-reliance --catalog FILE
  evalwitness registry index-scarcity --evidence FILE
  evalwitness registry index-owner-inspection --attestation FILE
  evalwitness registry render-method-lineage --autopsy FILE
  evalwitness registry index-empirical --attestation FILE
  evalwitness registry inventory
  evalwitness registry public-derivative --entry FILE
  evalwitness replay census-legacy-cache --source PATH --published-provider ID
  evalwitness replay migrate --source legacy.jsonl [--candidate legacy.jsonl.candidate]
  evalwitness eval-terminal [--trajs forge_gpt54] [--limit N] [--dry-run]
  evalwitness eval-swebench [--limit N] [--dry-run]
  evalwitness bon -n <N> --task <inline|@file> [--apply] [--keep] -- <agent command...>
  evalwitness fidelity --source PATH|- [--source PATH...] [--budgets 16384,32768,65536]
  evalwitness protocol <run|cases|schema|reference-adapter|application-adapter>
  evalwitness trace <inspect|export>
  evalwitness design <simulate --spec @design.json|reliance-preflight --code-digest SHA256|identical-response --spec @design.json> [--output json|text]
  evalwitness study <validate|lock|transition|report|split|schema|inventory|identical-response-inventory|identical-response-redistribution-right|identical-response-protocol|verify-execution>
  evalwitness stress <catalog|arm-plan|analysis-design|held-out-lock|held-out-campaign|held-out-readiness|development-case-study|development-challenge|verify-development-challenge|verify-development-challenge-receipt|validate|schema>
  evalwitness mutation <validate|schema|construct-repair build|construct-repair validate|construct-challenge build|construct-challenge validate|verification-evidence build-challenge|verification-evidence validate-challenge|control validate|corpus spec|corpus build|corpus validate>
  evalwitness agent-study <build|validate|schema>
  evalwitness relation <analyze-ambiguity|assign-primary|assign-tie|bundle|compare|handbook|judgment|judgment-batch|kit|materialize|materialize-v3|packet|packet-v3|pilot-change-receipt|pilot-inspection|pilot-inspection-session-finalize|pilot-inspection-session-guide|pilot-inspection-session-record|pilot-inspection-session-start|pilot-inspection-session-status|pilot-launch-dossier|plan|plan-v2|plan-v3|pilot-readiness|pilot-sample|pilot-sample-v3|primary-sample|primary-sample-v3|scarcity-sentinel-v3|probe-batch|qualification|qualify|render-kit|render-owner-inspection-public-attestation|render-pilot-change-atlas|render-pilot-inspection|render-pilot-launch-brief|render-scarcity-inspection|render-scarcity-public-brief|replay|replay-v3|reveal|reviewer|study-amendment|study-amendment-v3|terminal-ledger|translate|validate|verify-kit|verify-pilot-inspection|schema>
  evalwitness claim <verify|explain|challenge|autopsy|report|render|surface>
  evalwitness profile <build|verify|diff|policy|render> [--identity X --route R --dim ID:STATUS:METRIC:SCOPE:LEVEL:EXPR:DENOM:UNIT ... | --in FILE | --a A --b B | --policy FILE]
  evalwitness release <source-archive|source-index|manifest|sbom|statement|sign|verify>

mode flags:
  delta:     --trajectory <inline|@file> --trajectory <inline|@file>
  absolute:  --trajectory <inline|@file>
  pairwise:  --trajectory <inline|@file> --trajectory <inline|@file> ... (2 or more)

common flags:
  --criteria <id,id,...>  preset criterion ids (default: generic)
  --n-reps <int>          override EVALWITNESS_DEFAULT_REPS
  --no-bias-mit           disable order-bias mitigation
  --no-critique           disable critique-then-score
  --no-sprt               disable SPRT adaptive reps
  --paper-parity          reference-parity pipeline (no critique/bundling, single order, 4 reps)
  --no-cache              disable disk cache
  --output <json|text>    output format (default: json)

env: EVALWITNESS_PROVIDER, EVALWITNESS_MODEL, EVALWITNESS_BASE_URL, <PROVIDER>_API_KEY, EVALWITNESS_DEFAULT_REPS,
     EVALWITNESS_WIRE_FORMAT=openai,
     EVALWITNESS_BIAS_MITIGATION, EVALWITNESS_CRITIQUE_THEN_SCORE, EVALWITNESS_EPSILON, EVALWITNESS_MAX_TOKENS,
     EVALWITNESS_CACHE_DIR, EVALWITNESS_NO_CACHE, EVALWITNESS_NO_REDACT, EVALWITNESS_SPRT, ...`); err != nil {
		return
	}
}

type doctorReport struct {
	Version           string                  `json:"version"`
	Provider          string                  `json:"provider"`
	WireFormat        string                  `json:"wire_format"`
	BaseURL           string                  `json:"base_url"`
	Model             string                  `json:"model"`
	Preset            string                  `json:"preset,omitempty"`
	ThinkingMode      string                  `json:"thinking_mode"`
	KeyEnv            string                  `json:"key_env"`
	KeyPresent        bool                    `json:"key_present"`
	CacheDir          string                  `json:"cache_dir"`
	LegacyCacheDir    string                  `json:"legacy_cache_dir,omitempty"`
	Binary            string                  `json:"binary"`
	CapabilityCache   string                  `json:"capability_cache_namespace,omitempty"`
	CapabilityStatus  string                  `json:"capability_status"`
	FullVerifier      bool                    `json:"full_verifier"`
	AttestationID     string                  `json:"attestation_id,omitempty"`
	AttestationState  conformance.RouteState  `json:"attestation_state,omitempty"`
	AttestationExpiry string                  `json:"attestation_expires_at,omitempty"`
	JudgeModeAllowed  bool                    `json:"judge_mode_allowed,omitempty"`
	CachedCaps        *provider.Capabilities  `json:"cached_capabilities,omitempty"`
	LiveCaps          *provider.Capabilities  `json:"live_capabilities,omitempty"`
	AuthorizationPlan *mode.AuthorizationPlan `json:"authorization_plan,omitempty"`
	NextCommand       string                  `json:"next_command,omitempty"`
	Problems          []string                `json:"problems,omitempty"`
	Recommendations   []string                `json:"recommendations,omitempty"`
}

type runtimeBundle struct {
	Cfg      config.Config
	Provider provider.Provider
	Cache    *cache.Cache
	Cost     *cost.Calculator
	Audit    *audit.Logger
}

type runtimeLoadOptions struct {
	LiveIntent    bool
	Qualification bool
}

func (r *runtimeBundle) Close() error {
	if r == nil {
		return nil
	}
	var closeErrors []error
	if closer, ok := r.Provider.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("provider close: %w", err))
		}
	}
	if r.Audit != nil {
		if err := r.Audit.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("audit close: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}

func closeRuntime(rt *runtimeBundle, exitCode *int) {
	if err := rt.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "finalize:", err)
		*exitCode = 1
	}
}

func loadRuntime(options runtimeLoadOptions) (*runtimeBundle, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	log.Init(cfg.LogLevel)
	log.RegisterSecret(cfg.APIKey)

	if cfg.RedactPatternsFile != "" {
		if err := preprocess.LoadCustomPatterns(cfg.RedactPatternsFile); err != nil {
			return nil, fmt.Errorf("EVALWITNESS_REDACT_PATTERNS: %w", err)
		}
	}

	caps, _, ok := provider.LoadCachedCapsWithLegacy(cfg.CacheDir, cfg.LegacyCacheDir, cfg.Provider, cfg.Model)
	ctxLimit := cfg.ContextLimit
	if ctxLimit <= 0 {
		ctxLimit = provider.DefaultContextLimit
	}
	if !ok || options.Qualification {
		caps = provider.Capabilities{
			Logprobs:       true,
			TopLogprobsMax: 20,
			MaxConcurrent:  cfg.MaxWorkers,
			ContextLimit:   ctxLimit,
		}
	} else if cfg.ContextLimit > 0 {
		caps.ContextLimit = ctxLimit
	}
	caps.Streaming = cfg.Stream

	pcfg := provider.Config{
		Name:             cfg.Provider,
		WireFormat:       cfg.WireFormat,
		BaseURL:          cfg.BaseURL,
		APIKey:           cfg.APIKey,
		CAFile:           cfg.CAFile,
		Model:            cfg.Model,
		UpstreamProvider: cfg.UpstreamProvider,
		Timeout:          cfg.TimeoutSec,
		Caps:             caps,
		Thinking:         cfg.ThinkingMode,
		Offline:          cfg.Offline,
		LiveIntent:       options.LiveIntent,
		MaxRetries:       cfg.MaxRetries,
		UserAgent:        "evalwitness/" + version,
	}

	var p provider.Provider
	if cfg.ReplayFrom != "" {
		p, err = replay.LoadReplay(cfg.ReplayFrom, cfg.Provider, cfg.Model, caps)
		if err != nil {
			return nil, fmt.Errorf("replay load: %w", err)
		}
	} else {
		p, err = provider.New(pcfg)
		if err != nil {
			return nil, err
		}
		if cfg.ReplayTo != "" {
			cap, err := replay.WrapCapture(p, cfg.Model, cfg.ReplayTo, cfg.ReplayOverwrite)
			if err != nil {
				return nil, fmt.Errorf("replay capture: %w", err)
			}
			p = cap
		}
	}

	c := cache.NewWithLegacyImport(cfg.CacheDir, cfg.LegacyCacheDir, !cfg.NoCache)

	calc := cost.New(
		cfg.InputUSDPerM,
		cfg.CachedUSDPerM,
		cfg.OutputUSDPerM,
		cfg.BillingModel == "subscription",
	)

	auditLogger, err := audit.New(cfg.AuditLog)
	if err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}

	return &runtimeBundle{
		Cfg:      cfg,
		Provider: p,
		Cache:    c,
		Cost:     calc,
		Audit:    auditLogger,
	}, nil
}

func runProbe(args []string) (exitCode int) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	providerFlag := fs.String("provider", "", "provider override")
	wireFormatFlag := fs.String("wire-format", "", "wire format override (openai only)")
	modelFlag := fs.String("model", "", "model override")
	authorize := fs.String("authorize", "", "execute only when this digest matches the printed plan")
	maxCalls := fs.Int("max-calls", 1, "hard logical-call limit")
	maxAttempts := fs.Int("max-attempts", 0, "hard HTTP-attempt limit")
	maxInputTokens := fs.Int("max-input-tokens", 0, "hard reserved input-token limit")
	maxOutputTokens := fs.Int("max-output-tokens", 0, "hard reserved output-token limit")
	maxDuration := fs.Duration("max-duration", 0, "hard total deadline")
	maxConcurrent := fs.Int("max-concurrent", 1, "hard concurrent-attempt limit")
	maxCostUSD := fs.Float64("max-cost-usd", 0, "optional hard monetary limit")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		return 2
	}
	if *providerFlag != "" {
		if err := os.Setenv("EVALWITNESS_PROVIDER", *providerFlag); err != nil {
			fmt.Fprintln(os.Stderr, "probe:", err)
			return 2
		}
	}
	if *wireFormatFlag != "" {
		if err := os.Setenv("EVALWITNESS_WIRE_FORMAT", *wireFormatFlag); err != nil {
			fmt.Fprintln(os.Stderr, "probe:", err)
			return 2
		}
	}
	if *modelFlag != "" {
		if err := os.Setenv("EVALWITNESS_MODEL", *modelFlag); err != nil {
			fmt.Fprintln(os.Stderr, "probe:", err)
			return 2
		}
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	request, err := provider.NewProbeRequest(cfg.Provider, cfg.BaseURL, cfg.Model, cfg.ThinkingMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe request:", err)
		return 1
	}
	requestFingerprint, err := request.Fingerprint()
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe fingerprint:", err)
		return 1
	}
	limits := resolveProbeLimits(cfg, request, *maxCalls, *maxAttempts, *maxInputTokens, *maxOutputTokens, *maxDuration, *maxConcurrent, *maxCostUSD)
	plan, err := buildProbeAuthorization(cfg, request, requestFingerprint, limits, "cli.probe")
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe authorization:", err)
		return 2
	}
	if strings.TrimSpace(*authorize) == "" {
		printAuthorizationPlan(plan)
		return 0
	}
	if err := plan.Verify(*authorize); err != nil {
		fmt.Fprintln(os.Stderr, err)
		printAuthorizationPlan(plan)
		return 2
	}
	caps, err := executeProbe(cfg, request, limits)
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe failed:", err)
		return 1
	}
	out, _ := json.MarshalIndent(struct {
		Provider     string                 `json:"provider"`
		WireFormat   string                 `json:"wire_format"`
		Model        string                 `json:"model"`
		State        conformance.RouteState `json:"state"`
		Capabilities provider.Capabilities  `json:"capabilities"`
	}{cfg.Provider, cfg.WireFormat, cfg.Model, conformance.StateProbeCompatible, caps}, "", "  ")
	fmt.Println(string(out))
	return 0
}

func runPresets(args []string) int {
	fs := flag.NewFlagSet("presets", flag.ExitOnError)
	output := fs.String("output", "text", "json/text")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	summaries := config.PresetSummaries()
	switch *output {
	case "json":
		b, _ := json.MarshalIndent(summaries, "", "  ")
		fmt.Println(string(b))
	case "text":
		for _, p := range summaries {
			marker := " "
			if p.Default {
				marker = "*"
			}
			fmt.Printf("%s %s\n", marker, p.Name)
			fmt.Printf("  provider=%s model=%s\n", p.Provider, p.Model)
			fmt.Printf("  base_url=%s key=%s state=%s\n", p.BaseURL, strings.Join(p.KeyEnvNames, "|"), p.CapabilityState)
			if p.Limitations != "" {
				fmt.Printf("  note=%s\n", p.Limitations)
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown --output:", *output)
		return 2
	}
	return 0
}

func runDoctor(args []string) (exitCode int) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	live := fs.Bool("live", false, "run live provider probe")
	output := fs.String("output", "text", "json/text")
	authorize := fs.String("authorize", "", "execute --live only when this digest matches the printed plan")
	maxCalls := fs.Int("max-calls", 1, "hard logical-call limit")
	maxAttempts := fs.Int("max-attempts", 0, "hard HTTP-attempt limit")
	maxInputTokens := fs.Int("max-input-tokens", 0, "hard reserved input-token limit")
	maxOutputTokens := fs.Int("max-output-tokens", 0, "hard reserved output-token limit")
	maxDuration := fs.Duration("max-duration", 0, "hard total deadline")
	maxConcurrent := fs.Int("max-concurrent", 1, "hard concurrent-attempt limit")
	maxCostUSD := fs.Float64("max-cost-usd", 0, "optional hard monetary limit")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "doctor:", err)
		return 2
	}
	cfg, err := config.LoadForDiagnostics()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	keyEnv := providerKeyEnv(cfg.Provider)
	cachedCaps, cacheSource, cached := provider.LoadCachedCapsWithLegacy(cfg.CacheDir, cfg.LegacyCacheDir, cfg.Provider, cfg.Model)
	report := doctorReport{
		Version:          version,
		Provider:         cfg.Provider,
		WireFormat:       cfg.WireFormat,
		BaseURL:          cfg.BaseURL,
		Model:            cfg.Model,
		Preset:           cfg.Preset,
		ThinkingMode:     displayThinkingMode(cfg.ThinkingMode),
		KeyEnv:           keyEnv,
		KeyPresent:       cfg.APIKey != "",
		CacheDir:         cfg.CacheDir,
		LegacyCacheDir:   cfg.LegacyCacheDir,
		Binary:           executablePath(),
		CapabilityCache:  string(cacheSource),
		FullVerifier:     false,
		JudgeModeAllowed: cfg.AllowJudgeMode,
	}
	if cached {
		report.CachedCaps = &cachedCaps
	}
	qualificationRequest, qualificationRequestErr := defaultQualificationRequest(cfg)
	if qualificationRequestErr != nil {
		report.Problems = append(report.Problems, "qualification contract: "+qualificationRequestErr.Error())
	} else if store, storeErr := conformance.OpenExistingStore(cfg.CacheDir); storeErr == nil {
		attestation, state, _, loadErr := store.Load(
			conformance.RouteConfigDigest(qualificationRequest),
			conformance.CapabilityContractDigest(qualificationRequest),
			time.Now().UTC(), "",
		)
		if loadErr == nil {
			report.AttestationID = attestation.AttestationID
			report.AttestationState = state
			report.AttestationExpiry = attestation.ExpiresAt.Format(time.RFC3339)
			report.FullVerifier = state == conformance.StateBoundedQualified || state == conformance.StateStudyQualified
		}
	}
	if cfg.WireFormat != "openai" {
		report.Problems = append(report.Problems, "EVALWITNESS_WIRE_FORMAT must be openai")
	}
	if cfg.APIKey == "" && cfg.Provider != "ollama" {
		report.Problems = append(report.Problems, "missing API key: set "+keyEnv+" or EVALWITNESS_API_KEY")
	}
	if report.AttestationState == "" {
		report.Recommendations = append(report.Recommendations, "run ./evalwitness attest, inspect the authorization plan, then rerun with --authorize DIGEST")
	} else if !report.FullVerifier {
		report.Problems = append(report.Problems, "route has no current bounded qualification; attestation state is "+string(report.AttestationState))
	}
	if cached && (!cachedCaps.Logprobs || cachedCaps.TopLogprobsMax < verifier.MinimumVerifierTopK) {
		if cfg.AllowJudgeMode {
			report.Recommendations = append(report.Recommendations, "weak probe supports judge-mode only; quality delta vs full verifier mode is unmeasured")
		} else {
			report.Problems = append(report.Problems, "weak probe did not observe the minimum logprob shape")
		}
	}
	if *live {
		probeRequest, requestErr := provider.NewProbeRequest(cfg.Provider, cfg.BaseURL, cfg.Model, cfg.ThinkingMode)
		if requestErr != nil {
			report.Problems = append(report.Problems, "live probe request: "+requestErr.Error())
		} else {
			fingerprint, fingerprintErr := probeRequest.Fingerprint()
			limits := resolveProbeLimits(cfg, probeRequest, *maxCalls, *maxAttempts, *maxInputTokens, *maxOutputTokens, *maxDuration, *maxConcurrent, *maxCostUSD)
			plan, planErr := buildProbeAuthorization(cfg, probeRequest, fingerprint, limits, "cli.doctor.live")
			if fingerprintErr != nil || planErr != nil {
				report.Problems = append(report.Problems, "live probe authorization: "+errors.Join(fingerprintErr, planErr).Error())
			} else {
				report.AuthorizationPlan = &plan
				switch {
				case strings.TrimSpace(*authorize) == "":
					report.Recommendations = append(report.Recommendations, "approve the displayed live authorization digest with --authorize DIGEST")
				case plan.Verify(*authorize) != nil:
					report.Problems = append(report.Problems, plan.Verify(*authorize).Error())
				default:
					caps, probeErr := executeProbe(cfg, probeRequest, limits)
					if probeErr != nil {
						report.Problems = append(report.Problems, "live probe failed: "+probeErr.Error())
					} else {
						report.LiveCaps = &caps
					}
				}
			}
		}
	}
	finalizeDoctorReport(&report)
	switch *output {
	case "json":
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
	case "text":
		printDoctorText(report)
	default:
		fmt.Fprintln(os.Stderr, "unknown --output:", *output)
		return 2
	}
	if len(report.Problems) > 0 {
		return 1
	}
	return 0
}

func finalizeDoctorReport(report *doctorReport) {
	switch {
	case !report.KeyPresent:
		report.CapabilityStatus = string(conformance.StateUnconfigured)
	case report.AttestationState == conformance.StateStudyQualified:
		report.CapabilityStatus = string(conformance.StateStudyQualified)
	case report.AttestationState == conformance.StateBoundedQualified:
		report.CapabilityStatus = string(conformance.StateBoundedQualified)
	case report.AttestationState == conformance.StateExpired || report.AttestationState == conformance.StateFailed:
		report.CapabilityStatus = string(report.AttestationState)
	case (report.LiveCaps != nil && report.LiveCaps.Logprobs) || (report.CachedCaps != nil && report.CachedCaps.Logprobs):
		report.CapabilityStatus = string(conformance.StateProbeCompatible)
	case (report.LiveCaps != nil || report.CachedCaps != nil) && report.JudgeModeAllowed:
		report.CapabilityStatus = "judge-mode"
	case report.LiveCaps != nil || report.CachedCaps != nil:
		report.CapabilityStatus = string(conformance.StateConfigured)
	default:
		report.CapabilityStatus = string(conformance.StateConfigured)
	}
	if !report.KeyPresent {
		report.NextCommand = "export " + report.KeyEnv + "=..."
		return
	}
	if report.AuthorizationPlan != nil {
		report.NextCommand = "./evalwitness doctor --live --authorize " + report.AuthorizationPlan.AuthorizationDigest
		return
	}
	if !report.FullVerifier {
		report.NextCommand = "./evalwitness attest"
	}
	if report.CapabilityStatus == "judge-mode" {
		report.NextCommand = "./evalwitness verify --judge-mode ..."
	}
}

func displayThinkingMode(mode string) string {
	if mode == "" {
		return "omitted"
	}
	return mode
}

func runVerify(args []string) (exitCode int) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	modeFlag := fs.String("mode", "delta", "pairwise/absolute/delta")
	taskFlag := fs.String("task", "", "task description (inline or @file)")
	criteriaFlag := fs.String("criteria", "generic", "comma-separated preset criterion ids")
	nReps := fs.Int("n-reps", 0, "reps per criterion (0 = use config default)")
	providerFlag := fs.String("provider", "", "override EVALWITNESS_PROVIDER")
	wireFormatFlag := fs.String("wire-format", "", "override EVALWITNESS_WIRE_FORMAT")
	modelFlag := fs.String("model", "", "override EVALWITNESS_MODEL")
	baseURLFlag := fs.String("base-url", "", "override EVALWITNESS_BASE_URL")
	maxWorkers := fs.Int("max-workers", 0, "override EVALWITNESS_MAX_WORKERS (0 = use config default)")
	epsilonFlag := fs.Float64("epsilon", 0, "tie threshold (0 = use config default)")
	noBoth := fs.Bool("no-bias-mit", false, "disable order-bias mitigation")
	noCrit := fs.Bool("no-critique", false, "disable critique-then-score")
	noSPRT := fs.Bool("no-sprt", false, "disable SPRT adaptive reps")
	paperParity := fs.Bool("paper-parity", false, "reference-parity mode: single-criterion prompts, no critique, no bundling, single order, fixed 4 reps")
	judgeMode := fs.Bool("judge-mode", false, "LLM-as-a-Judge: no logprob requests, raw-text score extraction")
	noCache := fs.Bool("no-cache", false, "disable disk cache for this run")
	verbose := fs.Bool("verbose", false, "debug logging to stderr")
	output := fs.String("output", "json", "json/text")
	liveFlags := addVerifyLiveFlags(fs)

	var trajectories stringSlice
	fs.Var(&trajectories, "trajectory", "trajectory inline or @file (repeat for multiple)")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := validateVerifyLiveFlags(liveFlags); err != nil {
		fmt.Fprintln(os.Stderr, "verify:", err)
		return 2
	}

	if *providerFlag != "" {
		if err := os.Setenv("EVALWITNESS_PROVIDER", *providerFlag); err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			return 2
		}
	}
	if *wireFormatFlag != "" {
		if err := os.Setenv("EVALWITNESS_WIRE_FORMAT", *wireFormatFlag); err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			return 2
		}
	}
	if *modelFlag != "" {
		if err := os.Setenv("EVALWITNESS_MODEL", *modelFlag); err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			return 2
		}
	}
	if *baseURLFlag != "" {
		if err := os.Setenv("EVALWITNESS_BASE_URL", *baseURLFlag); err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			return 2
		}
	}
	if *maxWorkers > 0 {
		if err := os.Setenv("EVALWITNESS_MAX_WORKERS", fmt.Sprintf("%d", *maxWorkers)); err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			return 2
		}
	}
	if *verbose {
		if err := os.Setenv("EVALWITNESS_LOG_LEVEL", "debug"); err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			return 2
		}
	}
	if *judgeMode {
		if err := os.Setenv("EVALWITNESS_JUDGE_MODE", "true"); err != nil {
			fmt.Fprintln(os.Stderr, "verify:", err)
			return 2
		}
	}
	if *taskFlag == "" {
		fmt.Fprintln(os.Stderr, "--task required")
		return 2
	}

	task, err := loadInline(*taskFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "task:", err)
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}

	critNames := splitTrim(*criteriaFlag)
	crits, err := verifier.ResolveCriteria(critNames)
	if err != nil {
		fmt.Fprintln(os.Stderr, "criteria:", err)
		return 2
	}

	biasMit := cfg.BiasMitigation
	if *noBoth {
		biasMit = "single"
	}
	critique := cfg.CritiqueThenScore
	if *noCrit {
		critique = false
	}
	useSPRT := cfg.SPRT.Enabled
	if *noSPRT {
		useSPRT = false
	}
	reps := *nReps
	if reps <= 0 {
		reps = cfg.DefaultReps
	}
	epsilon := *epsilonFlag
	if epsilon == 0 {
		epsilon = cfg.Epsilon
	}
	if *paperParity {
		// Prompt-for-prompt parity with eval/python-reference: one criterion
		// per call, no critique, no bundling, single order, fixed reps.
		critique = false
		biasMit = "single"
		useSPRT = false
		cfg.MultiCriterionBundle = false
		if *nReps <= 0 {
			reps = 4
		}
	}

	loadedTrajectories := make([]string, len(trajectories))
	for index, trajectory := range trajectories {
		loaded, loadErr := loadInline(trajectory)
		if loadErr != nil {
			fmt.Fprintln(os.Stderr, "trajectory", index, ":", loadErr)
			return 2
		}
		loadedTrajectories[index] = loaded
	}
	switch *modeFlag {
	case "delta":
		if len(loadedTrajectories) != 2 {
			fmt.Fprintln(os.Stderr, "delta mode requires exactly 2 --trajectory flags")
			return 2
		}
	case "absolute":
		if len(loadedTrajectories) != 1 {
			fmt.Fprintln(os.Stderr, "absolute mode requires exactly 1 --trajectory flag")
			return 2
		}
	case "pairwise":
		if len(loadedTrajectories) < 2 {
			fmt.Fprintln(os.Stderr, "pairwise mode requires 2+ --trajectory flags")
			return 2
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *modeFlag)
		return 2
	}
	cfg.CritiqueThenScore = critique
	service, err := verification.NewProductionService(cfg, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify init:", err)
		return 1
	}
	input := verification.Input{
		Entrypoint: "cli.verify", Mode: verification.Mode(*modeFlag), Task: task,
		Trajectories: loadedTrajectories, Criteria: crits,
		Policy: verification.PolicyFromConfig(cfg, reps, epsilon, biasMit, useSPRT, "pairwise"),
		Limits: mode.BudgetLimits{
			MaxCalls: *liveFlags.maxCalls, MaxAttempts: *liveFlags.maxAttempts,
			MaxEstimatedInputTokens: *liveFlags.maxInputTokens, MaxReservedOutputTokens: *liveFlags.maxOutputTokens,
			MaxConcurrent: *liveFlags.maxConcurrent, MaxCostUSD: *liveFlags.maxCostUSD, MaxDuration: *liveFlags.maxDuration,
		},
		AuthorizationDigest: *liveFlags.authorize, BudgetStatePath: cfg.RunBudgetState, DisableCache: *noCache,
	}
	plan, err := service.Plan(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify plan:", err)
		return 2
	}
	if plan.Authorization != nil {
		if strings.TrimSpace(*liveFlags.authorize) == "" {
			printAuthorizationPlan(*plan.Authorization)
			return 0
		}
		if err := plan.Authorization.Verify(*liveFlags.authorize); err != nil {
			fmt.Fprintln(os.Stderr, err)
			printAuthorizationPlan(*plan.Authorization)
			return 2
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	runResult, err := service.Execute(ctx, plan)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify:", err)
		return 1
	}
	var result any
	switch runResult.Mode {
	case verification.ModeDelta:
		result = *runResult.Delta
	case verification.ModeAbsolute:
		result = *runResult.Absolute
	case verification.ModePairwise:
		result = *runResult.Selection
	}

	switch *output {
	case "json":
		b, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(b))
	case "text":
		printResultText(*modeFlag, result)
	default:
		fmt.Fprintln(os.Stderr, "unknown --output:", *output)
		return 2
	}
	return 0
}

func runMCP(_ []string) (exitCode int) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	service, err := verification.NewProductionService(cfg, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcp init:", err)
		return 1
	}
	handler := &mcp.ToolHandler{
		Service: service,
		Policy: verification.PolicyFromConfig(
			cfg, cfg.DefaultReps, cfg.Epsilon, cfg.BiasMitigation, cfg.SPRT.Enabled, "pairwise",
		),
	}
	handler.Policy.MaxWorkers = 1
	server := mcp.NewServer("evalwitness", version, handler, os.Stdin, os.Stdout)
	if err := server.Serve(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "mcp serve:", err)
		return 1
	}
	return 0
}

func runCache(args []string) (exitCode int) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "cache: missing subcommand (stats|clear --scope responses|capabilities|all)")
		return 2
	}
	rt, err := loadRuntime(runtimeLoadOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	defer closeRuntime(rt, &exitCode)
	cfg := rt.Cfg
	c := rt.Cache
	switch args[0] {
	case "stats":
		s, err := c.Stats()
		if err != nil {
			fmt.Fprintln(os.Stderr, "stats:", err)
			return 1
		}
		out := map[string]any{
			"cache_dir":  cfg.CacheDir,
			"entries":    s.Entries,
			"size_bytes": s.SizeBytes,
			"enabled":    !cfg.NoCache,
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return 0
	case "clear":
		if len(args) != 3 || args[1] != "--scope" {
			fmt.Fprintln(os.Stderr, "cache clear: required --scope responses|capabilities|all")
			return 2
		}
		result, err := c.Clear(cache.ClearScope(args[2]))
		if err != nil {
			fmt.Fprintln(os.Stderr, "clear:", err)
			return 1
		}
		fmt.Printf("cache cleared: scope=%s files=%d bytes=%d\n", result.Scope, result.FilesRemoved, result.BytesRemoved)
		return 0
	}
	fmt.Fprintf(os.Stderr, "cache: unknown sub %q\n", args[0])
	return 2
}

func printDoctorText(report doctorReport) {
	fmt.Printf("evalwitness %s\n", report.Version)
	fmt.Printf("binary: %s\n", report.Binary)
	if report.Preset != "" {
		fmt.Printf("preset: %s\n", report.Preset)
	}
	fmt.Printf("provider: %s\n", report.Provider)
	fmt.Printf("model: %s\n", report.Model)
	fmt.Printf("base_url: %s\n", report.BaseURL)
	fmt.Printf("wire_format: %s\n", report.WireFormat)
	fmt.Printf("thinking_mode: %s\n", report.ThinkingMode)
	fmt.Printf("key: %s present=%t\n", report.KeyEnv, report.KeyPresent)
	fmt.Printf("cache_dir: %s\n", report.CacheDir)
	if report.LegacyCacheDir != "" {
		fmt.Printf("legacy_cache_dir: %s (read-only)\n", report.LegacyCacheDir)
	}
	if report.CachedCaps != nil {
		fmt.Printf("cached_caps: logprobs=%t top_logprobs=%d streaming=%t\n",
			report.CachedCaps.Logprobs, report.CachedCaps.TopLogprobsMax, report.CachedCaps.Streaming)
		fmt.Printf("capability_cache_namespace: %s\n", report.CapabilityCache)
	} else {
		fmt.Println("cached_caps: missing")
	}
	if report.LiveCaps != nil {
		fmt.Printf("live_caps: logprobs=%t top_logprobs=%d streaming=%t\n",
			report.LiveCaps.Logprobs, report.LiveCaps.TopLogprobsMax, report.LiveCaps.Streaming)
	}
	if report.AttestationState != "" {
		fmt.Printf("attestation: id=%s state=%s expires=%s\n", report.AttestationID, report.AttestationState, report.AttestationExpiry)
	} else {
		fmt.Println("attestation: missing")
	}
	if report.AuthorizationPlan != nil {
		fmt.Printf("authorization_digest: %s\n", report.AuthorizationPlan.AuthorizationDigest)
	}
	fmt.Printf("capability_status: %s\n", report.CapabilityStatus)
	fmt.Printf("full_verifier: %t\n", report.FullVerifier)
	if report.NextCommand != "" {
		fmt.Printf("next: %s\n", report.NextCommand)
	}
	if len(report.Problems) > 0 {
		fmt.Println("problems:")
		for _, problem := range report.Problems {
			fmt.Printf("- %s\n", problem)
		}
	}
	if len(report.Recommendations) > 0 {
		fmt.Println("recommendations:")
		for _, rec := range report.Recommendations {
			fmt.Printf("- %s\n", rec)
		}
	}
}

func providerKeyEnv(providerName string) string {
	return strings.ToUpper(strings.ReplaceAll(providerName, "-", "_")) + "_API_KEY"
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// applyPaperParityEnv flips the env-driven pipeline switches to the reference
// configuration (no critique, no bundling) and defaults reps to the paper's 4.
// Called before loadRuntime so the runner picks the values up.
func applyPaperParityEnv(fs *flag.FlagSet, criteriaFlagName string, nReps *int) error {
	for _, setting := range []struct{ key, value string }{
		{key: "EVALWITNESS_CRITIQUE_THEN_SCORE", value: "false"},
		{key: "EVALWITNESS_MULTI_CRITERION_BUNDLE", value: "false"},
		{key: "EVALWITNESS_SINGLE_ELIM", value: "false"},
	} {
		if err := os.Setenv(setting.key, setting.value); err != nil {
			return err
		}
	}
	explicitCriteria := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == criteriaFlagName {
			explicitCriteria = true
		}
	})
	if explicitCriteria {
		fmt.Fprintln(os.Stderr, "note: --paper-parity overrides --criteria with the paper criteria set")
	}
	if *nReps <= 0 {
		*nReps = 4
	}
	return nil
}

func loadInline(s string) (string, error) {
	if !strings.HasPrefix(s, "@") {
		return s, nil
	}
	path, err := expandPath(s[1:])
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func expandPath(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func printResultText(modeName string, r any) {
	switch v := r.(type) {
	case mode.Verdict:
		fmt.Printf("state: %s\n", v.State)
		fmt.Printf("winner: %s\n", v.Winner)
		fmt.Printf("conditional_margin: %.4f\n", v.Margin)
		fmt.Printf("conditional_score_a: %.4f\n", v.ScoreA)
		fmt.Printf("conditional_score_b: %.4f\n", v.ScoreB)
		fmt.Printf("inconsistent: %t\n", v.Inconsistent)
		if len(v.PerCriterion) > 0 {
			fmt.Println("criteria:")
			for id, sc := range v.PerCriterion {
				fmt.Printf("- %s: A=%.4f B=%.4f\n", id, sc.A, sc.B)
			}
		}
		printUsage(v.Usage)
	case mode.Selection:
		fmt.Printf("state: %s\n", v.State)
		if v.State == verifier.DecisionSelected {
			fmt.Printf("best_index: %d\n", v.BestIndex)
		} else {
			fmt.Println("best_index: none")
		}
		fmt.Printf("decision_strength: %.4f\n", v.Confidence)
		fmt.Printf("conditional_scores: %s\n", formatFloatSlice(v.Scores))
		fmt.Printf("wins: %s\n", formatFloatSlice(v.Wins))
		if len(v.InconsistentPairs) > 0 {
			fmt.Printf("inconsistent_pairs: %v\n", v.InconsistentPairs)
		}
		printUsage(v.Usage)
	case mode.Score:
		fmt.Printf("state: %s\n", v.State)
		fmt.Printf("conditional_score: %.4f\n", v.Value)
		fmt.Printf("evidence_strength: observations=%d extracted=%d min_top_k=%d min_visible_mass=%.4f min_valid_mass=%.4f\n",
			v.EvidenceStrength.Observations, v.EvidenceStrength.ExtractedObservations,
			v.EvidenceStrength.MinimumReturnedTopK, v.EvidenceStrength.MinimumVisibleMass,
			v.EvidenceStrength.MinimumValidMass)
		if len(v.PerCriterion) > 0 {
			fmt.Println("criteria:")
			for id, score := range v.PerCriterion {
				fmt.Printf("- %s: %.4f\n", id, score)
			}
		}
		printUsage(v.Usage)
	default:
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Printf("[%s]\n%s\n", modeName, string(b))
	}
}

func printUsage(u mode.UsageSummary) {
	fmt.Printf("usage: calls=%d cache_hits=%d legacy_cache_hits=%d input_tokens=%d output_tokens=%d cached_tokens=%d\n",
		u.Calls, u.CacheHitCalls, u.LegacyCacheHitCalls, u.InputTokens, u.OutputTokens, u.CachedTokens)
}

func formatFloatSlice(values []float64) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%.4f", value)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}
