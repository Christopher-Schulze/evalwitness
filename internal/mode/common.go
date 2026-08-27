package mode

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/cache"
	"github.com/Christopher-Schulze/evalwitness/internal/cost"
	"github.com/Christopher-Schulze/evalwitness/internal/log"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type Runner struct {
	Provider provider.Provider
	Cache    *cache.Cache
	Cost     *cost.Calculator
	Audit    *audit.Logger
	Cfg      RunnerConfig
	Budget   *RunBudget

	noLogprobsWarning sync.Once
	degenerateWarning sync.Once
	limiterOnce       sync.Once
	providerLimiter   chan struct{}

	servedModelMu sync.Mutex
	servedModel   string
}

// ServedModel is the model identifier the endpoint reported for this run, which
// can differ from the requested one when a gateway sits in between. Empty until
// the first response or cache hit that carries it.
func (r *Runner) ServedModel() string {
	r.servedModelMu.Lock()
	defer r.servedModelMu.Unlock()
	return r.servedModel
}

func (r *Runner) recordServedModel(name string) {
	if name == "" {
		return
	}
	r.servedModelMu.Lock()
	if r.servedModel == "" {
		r.servedModel = name
	}
	r.servedModelMu.Unlock()
}

type RunnerConfig struct {
	Model                string
	BaseURL              string
	ThinkingMode         string
	Stream               bool
	Entrypoint           string
	NoRedact             bool
	PreprocessBudget     int
	CritiqueThenScore    bool
	MultiCriterionBundle bool
	Temperature          float64
	MaxTokens            int
	TopLogprobs          int
	MaxCostPerCall       float64
	MaxWorkers           int
	Seed                 *int
	RequireLogprobs      bool
	// JudgeMode skips logprob requests and forces raw-text extraction
	// (LLM-as-a-Judge instead of LLM-as-a-Verifier).
	JudgeMode bool
	Lineage   provider.RequestLineage
}

// CostCapError is returned before dispatch when the estimated call cost
// exceeds EVALWITNESS_MAX_COST_USD_PER_CALL. MCP maps it to error code -32008.
type CostCapError struct {
	EstCostUSD float64
	CapUSD     float64
}

func (e *CostCapError) Error() string {
	return fmt.Sprintf("estimated cost $%.4f exceeds EVALWITNESS_MAX_COST_USD_PER_CALL=$%.4f", e.EstCostUSD, e.CapUSD)
}

const (
	promptOverheadTokens = 2048
	safetyMarginTokens   = 1024
	// fullVerifierExtractionRetries bounds the retries for a response that
	// carried no extractable distribution. Each retry drops idle connections
	// first, so the attempts differ instead of repeating; see the retry site.
	// A ~3100-call parity run dies on a single unextractable pair, and a fresh
	// process cleared pairs that same-connection retries could not.
	fullVerifierExtractionRetries = 3
	// minTruncationBudget guarantees truncation still leaves usable trace
	// content when the context window is very small.
	minTruncationBudget = 512
)

// DerivePreprocessBudget computes the per-trajectory token budget from the
// provider context limit: contextLimit - prompt overhead - 2x output reserve -
// safety margin, halved because pairwise prompts carry two trajectories.
// Returns 0 (no truncation) when contextLimit is unknown.
func DerivePreprocessBudget(contextLimit, maxOutputTokens int) int {
	if contextLimit <= 0 {
		return 0
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 256
	}
	total := contextLimit - promptOverheadTokens - 2*maxOutputTokens - safetyMarginTokens
	if total < 2*minTruncationBudget {
		return minTruncationBudget
	}
	return total / 2
}

var biasWarnOnce sync.Once

// warnBiasNotBoth surfaces the spec-required startup warning when order-bias
// mitigation is not running both orders.
func warnBiasNotBoth(modeName, biasMitigation string) {
	if biasMitigation == "both" || biasMitigation == "adaptive" {
		return
	}
	biasWarnOnce.Do(func() {
		log.L().Warn("order-bias mitigation is not evaluating both orders; single-order scoring inherits position bias",
			"mode", modeName,
			"bias_mitigation", biasMitigation,
		)
	})
}

type ScoreOutcome struct {
	Distribution       map[string]map[string]float64
	Evidence           map[string]verifier.ScoreEvidence
	RawText            string
	InputTokens        int
	OutputTokens       int
	CachedTokens       int
	FromCache          bool
	CacheNamespace     string
	HasLogprobs        bool
	RequestFingerprint provider.Fingerprint
	ResponseDigest     string
	ServedModel        string
	ReplayStatus       provider.ReplayStatus
	ReplayReason       string
	FinishReason       string
	// EstimatedInput marks an outcome whose input size the provider did not
	// report and which fell back to the local estimate.
	EstimatedInput bool
}

type UsageSummary struct {
	Calls               int `json:"calls"`
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CachedTokens        int `json:"cached_tokens"`
	CacheHitCalls       int `json:"cache_hit_calls"`
	LegacyCacheHitCalls int `json:"legacy_cache_hit_calls,omitempty"`
	LogprobCalls        int `json:"logprob_calls"`
	JudgeTextCalls      int `json:"judge_text_calls"`
	UnextractedScores   int `json:"unextracted_scores"`
	// EstimatedInputCalls counts calls whose input size the provider did not
	// report and which fall back to the local bytes/4 estimate, so an artifact
	// says how much of its input figure is measured rather than estimated.
	EstimatedInputCalls int     `json:"estimated_input_calls,omitempty"`
	ExtractionMode      string  `json:"extraction_mode,omitempty"`
	EstCostUSD          float64 `json:"est_cost_usd,omitempty"`
}

// finalizeExtraction labels the run verifier / judge / mixed based on whether
// score-token logprobs actually backed the extraction.
func (u *UsageSummary) finalizeExtraction() {
	u.updateExtractionMode()
	if u.UnextractedScores > 0 {
		log.L().Warn("score extraction invariant violated; strict runtime paths must reject this result",
			"unextracted_scores", u.UnextractedScores,
			"calls", u.Calls,
		)
	}
}

func (u *UsageSummary) updateExtractionMode() {
	switch {
	case u.LogprobCalls > 0 && u.JudgeTextCalls == 0:
		u.ExtractionMode = "verifier"
	case u.LogprobCalls == 0 && u.JudgeTextCalls > 0:
		u.ExtractionMode = "judge"
	case u.LogprobCalls > 0 && u.JudgeTextCalls > 0:
		u.ExtractionMode = "mixed"
	}
}

func (r *Runner) PrepareTrajectory(raw string) (preprocess.Result, error) {
	return preprocess.CanonicalPipeline(raw, !r.Cfg.NoRedact, r.Cfg.PreprocessBudget)
}

type auditMetaKey struct{}
type requestLineageKey struct{}

type AuditMeta struct {
	RedactionHits      int
	TrajectoryEvidence []preprocess.AccountingSummary
}

func auditMetaFor(results ...preprocess.Result) AuditMeta {
	meta := AuditMeta{TrajectoryEvidence: preprocess.AccountingSummaries(results...)}
	for _, result := range results {
		meta.RedactionHits += result.RedactionHits
	}
	return meta
}

// ContextWithAuditMeta attaches per-call audit metadata that the Runner will
// pick up when writing audit log entries (redaction_hits primarily).
func ContextWithAuditMeta(ctx context.Context, m AuditMeta) context.Context {
	return context.WithValue(ctx, auditMetaKey{}, m)
}

func auditMetaFromContext(ctx context.Context) AuditMeta {
	if v, ok := ctx.Value(auditMetaKey{}).(AuditMeta); ok {
		return v
	}
	return AuditMeta{}
}

// ContextWithRequestLineage attaches validated run-level references. Score
// fills service-owned criterion, sampling, source, mapping, and entrypoint fields.
func ContextWithRequestLineage(ctx context.Context, lineage provider.RequestLineage) context.Context {
	return context.WithValue(ctx, requestLineageKey{}, lineage)
}

func requestLineageFromContext(ctx context.Context, fallback provider.RequestLineage) provider.RequestLineage {
	if value, ok := ctx.Value(requestLineageKey{}).(provider.RequestLineage); ok {
		return value
	}
	return fallback
}

func evidenceBindingsFromContext(ctx context.Context) []provider.EvidenceBinding {
	return EvidenceBindings(auditMetaFromContext(ctx).TrajectoryEvidence)
}

// EvidenceBindings converts canonical trajectory accounting into the exact
// ordered request evidence contract shared by planning and provider dispatch.
func EvidenceBindings(summaries []preprocess.AccountingSummary) []provider.EvidenceBinding {
	bindings := make([]provider.EvidenceBinding, 0, len(summaries))
	for index, summary := range summaries {
		bindings = append(bindings, provider.EvidenceBinding{
			InputSlot: fmt.Sprintf("trajectory_%d", index), SourceDigest: summary.SourceDigest,
			CanonicalDigest: summary.TrajectoryDigest, IngestionDigest: summary.IngestionDigest,
			TraceEnvelopeDigest: summary.TraceEnvelopeDigest, MappingReportDigest: summary.TraceMappingDigest,
			MappingPolicyVersion: summary.TraceMappingPolicy,
		})
	}
	return bindings
}

func (r *Runner) Score(ctx context.Context, criterionID, prompt string, scoreTags []string, maxTokens int) (ScoreOutcome, error) {
	verifierTopLogprobs := effectiveTopLogprobs(r.Cfg.TopLogprobs)
	if maximum := r.Provider.Capabilities().TopLogprobsMax; maximum > 0 && verifierTopLogprobs > maximum {
		verifierTopLogprobs = maximum
	}
	topLogprobs := verifierTopLogprobs
	if r.Cfg.JudgeMode {
		topLogprobs = 0
	}
	request, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID:       r.Provider.Name(),
		BaseURL:          r.Cfg.BaseURL,
		RequestedModel:   r.Cfg.Model,
		ThinkingMode:     r.Cfg.ThinkingMode,
		Messages:         []provider.Message{{Role: "user", Content: prompt}},
		Temperature:      effectiveTemperature(r.Cfg.Temperature),
		Seed:             r.Cfg.Seed,
		MaxOutputTokens:  maxTokens,
		Logprobs:         topLogprobs > 0,
		TopLogprobs:      topLogprobs,
		ScoreTags:        scoreTags,
		ResponseFormat:   provider.ResponseFormatText,
		Stream:           r.Cfg.Stream && len(scoreTags) > 0,
		EvidenceBindings: evidenceBindingsFromContext(ctx),
		Lineage: CompleteRequestLineage(
			requestLineageFromContext(ctx, r.Cfg.Lineage), baseCriterionID(criterionID), criterionID, r.Cfg.Entrypoint,
			auditMetaFromContext(ctx).TrajectoryEvidence,
		),
	})
	if err != nil {
		return ScoreOutcome{}, fmt.Errorf("canonical request: %w", err)
	}
	if r.Cache != nil {
		lookups := []provider.RequestEnvelope{request}
		for _, candidate := range lookups {
			e, ok := r.Cache.Get(candidate)
			if !ok {
				continue
			}
			outc := ScoreOutcome{
				Distribution:       e.Response.Distributions,
				Evidence:           verifier.ExtractAllScoreEvidence(candidate, e.Response, r.extractionMode()),
				RawText:            e.Response.RawText,
				InputTokens:        e.Response.Usage.Input,
				OutputTokens:       e.Response.Usage.Output,
				CachedTokens:       e.Response.Usage.Cached,
				FromCache:          true,
				CacheNamespace:     e.SourceNamespace,
				HasLogprobs:        e.Response.HasLogprobs,
				RequestFingerprint: e.Response.RequestFingerprint,
				ResponseDigest:     e.Response.EvidenceDigest,
				ServedModel:        e.Response.ServedModel,
				ReplayStatus:       provider.ReplayStatusExact,
				ReplayReason:       "exact canonical cache entry matched",
				FinishReason:       e.Response.FinishReason,
			}
			if outc.InputTokens == 0 {
				// Entries written before the streaming usage fix carry no input size,
				// so a replay of them would report zero. Substitute the same estimate
				// the budget uses and count it, rather than publish a figure that is
				// plainly false.
				outc.InputTokens = preprocess.EstimateTokens(prompt)
				outc.EstimatedInput = true
			}
			r.recordServedModel(e.Response.ServedModel)
			if r.Cfg.JudgeMode {
				// Present it exactly as a judge would have received it, so usage
				// accounting cannot report a judge run as logprob-backed.
				outc.Distribution = nil
				outc.HasLogprobs = false
			}
			if scoresExtractable(outc, scoreTags) {
				if err := observeScore(ctx, criterionID, candidate, e.Response, outc.Evidence, outc.ReplayStatus, nil, nil); err != nil {
					return ScoreOutcome{}, err
				}
				if err := r.audit(ctx, criterionID, candidate, e.Response, outc, ""); err != nil {
					return ScoreOutcome{}, err
				}
				return outc, nil
			}
			// Legacy or incomplete entries cannot back the active extraction
			// mode. Treat them as misses so a provider response can replace the
			// generated cache artifact.
		}
	}

	promptTokens := preprocess.EstimateTokens(prompt)
	estimatedCost := 0.0
	if r.Cost != nil {
		if est := r.Cost.EstimatePromptCost(promptTokens, maxTokens); est != nil {
			estimatedCost = *est
			if r.Cfg.MaxCostPerCall > 0 && *est > r.Cfg.MaxCostPerCall {
				return ScoreOutcome{}, &CostCapError{EstCostUSD: *est, CapUSD: r.Cfg.MaxCostPerCall}
			}
		}
	}
	callCtx := ctx
	if deadline, ok := r.Budget.Deadline(); ok {
		if current, hasCurrent := ctx.Deadline(); !hasCurrent || deadline.Before(current) {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithDeadline(ctx, deadline)
			defer cancel()
		}
	}
	estimatedInput := false
	providerAttempt := 0
	for extractionAttempt := 0; ; extractionAttempt++ {
		if err := r.Budget.ReserveCall(); err != nil {
			auditErr := r.audit(ctx, criterionID, request, provider.ResponseRecord{}, ScoreOutcome{}, err.Error())
			return ScoreOutcome{}, errors.Join(err, auditErr)
		}
		release, err := r.acquireProviderSlot(callCtx)
		if err != nil {
			auditErr := r.audit(ctx, criterionID, request, provider.ResponseRecord{}, ScoreOutcome{}, err.Error())
			return ScoreOutcome{}, errors.Join(err, auditErr)
		}
		var releaseAttempt func(AttemptActual) error
		request.BeforeAttempt = func() error {
			var reserveErr error
			releaseAttempt, reserveErr = r.Budget.AcquireAttempt(callCtx, AttemptReservation{
				EstimatedInputTokens: promptTokens,
				MaxOutputTokens:      maxTokens,
				EstimatedCostUSD:     estimatedCost,
			})
			return reserveErr
		}
		request.AfterAttempt = func(result provider.AttemptResult) error {
			if releaseAttempt != nil {
				providerAttempt++
				actual := AttemptActual{
					InputTokens:   result.Usage.Input,
					OutputTokens:  result.Usage.Output,
					UsageObserved: result.UsageObserved,
				}
				if result.UsageObserved && r.Cost != nil {
					actual.CostUSD = r.Cost.Estimate(result.Usage.Input, result.Usage.Output, result.Usage.Cached)
				}
				releaseErr := releaseAttempt(actual)
				releaseAttempt = nil
				auditErr := r.auditProviderAttempt(ctx, criterionID, request, providerAttempt, result)
				return errors.Join(releaseErr, auditErr)
			}
			return nil
		}
		resp, err := r.Provider.Score(callCtx, request)
		fallbackReleaseErr := request.AfterAttempt(provider.AttemptResult{})
		release()
		if fallbackReleaseErr != nil {
			return ScoreOutcome{}, fallbackReleaseErr
		}
		var captureEvidenceErr *verifier.EvidenceError
		if err != nil && !errors.As(err, &captureEvidenceErr) {
			outcome := ScoreOutcome{}
			var replayError *provider.ReplayLookupError
			if errors.As(err, &replayError) {
				outcome.ReplayStatus = replayError.Status
				outcome.ReplayReason = replayError.Reason
			}
			auditErr := r.audit(ctx, criterionID, request, provider.ResponseRecord{}, outcome, err.Error())
			return ScoreOutcome{}, errors.Join(err, auditErr)
		}
		// Capture validation runs before this extraction gate. An evidence error
		// means the exact response was intentionally not written; validate and
		// route it through the same bounded fresh-connection retry as any other
		// unextractable provider response.
		if err := resp.ValidateExact(request); err != nil {
			return ScoreOutcome{}, fmt.Errorf("provider returned invalid response record: %w", err)
		}
		replaySource, err := exactReplaySourceForObservation(r.Provider, resp.ReplayStatus)
		if err != nil {
			return ScoreOutcome{}, err
		}
		r.recordServedModel(resp.ServedModel)
		inputTokens := resp.Usage.Input
		if inputTokens == 0 {
			// Some providers report usage only in the trailing stream chunk, which a
			// cut stream never delivers. Falling back to the same estimate the hard
			// budget already reserves keeps an artifact from publishing an input
			// figure of zero; estimated_input_calls records how many it applies to.
			inputTokens = promptTokens
			estimatedInput = true
		}
		effectiveHasLogprobs := resp.HasLogprobs
		effectiveDistributions := resp.Distributions
		if r.Cfg.JudgeMode {
			effectiveHasLogprobs = false
			effectiveDistributions = nil
		}
		if resp.DegenerateLogprobs && len(scoreTags) > 0 && !r.Cfg.JudgeMode {
			// Structurally valid, informationally empty: every alternative sits at
			// the sentinel floor, so the "distribution" is a one-hot argmax with
			// mass 1 and variance 0. That passes every downstream gate and would
			// publish a judge as a verifier, so it is rejected as if no logprobs
			// had been returned at all.
			r.degenerateWarning.Do(func() {
				log.L().Warn("provider returned degenerate logprobs; every alternative is at the sentinel floor",
					"provider", r.Provider.Name(),
					"model", r.Cfg.Model,
					"hint", "a sampling temperature of 0 collapses the softmax to argmax on some stacks",
				)
			})
			effectiveHasLogprobs = false
			effectiveDistributions = nil
		}
		if !effectiveHasLogprobs && len(scoreTags) > 0 && !r.Cfg.JudgeMode {
			r.noLogprobsWarning.Do(func() {
				log.L().Warn("provider returned no logprobs; strict verifier extraction rejects text fallback",
					"provider", r.Provider.Name(),
					"model", r.Cfg.Model,
				)
			})
		}
		out := ScoreOutcome{
			Distribution:       effectiveDistributions,
			Evidence:           verifier.ExtractAllScoreEvidence(request, resp, r.extractionMode()),
			RawText:            resp.RawText,
			InputTokens:        inputTokens,
			OutputTokens:       resp.Usage.Output,
			CachedTokens:       resp.Usage.Cached,
			EstimatedInput:     estimatedInput,
			HasLogprobs:        effectiveHasLogprobs,
			RequestFingerprint: resp.RequestFingerprint,
			ResponseDigest:     resp.EvidenceDigest,
			ServedModel:        resp.ServedModel,
			ReplayStatus:       resp.ReplayStatus,
			ReplayReason:       resp.ReplayReason,
			FinishReason:       resp.FinishReason,
		}
		var extractionErr error
		if len(scoreTags) > 0 {
			if r.Cfg.JudgeMode {
				extractionErr = verifier.ValidateJudgeEvidence(out.Evidence)
			} else {
				extractionErr = verifier.ValidateStrictEvidence(out.Evidence)
			}
		}
		if extractionErr != nil {
			modeName := "judge"
			if !r.Cfg.JudgeMode {
				modeName = fmt.Sprintf("full verifier at Top-%d", topLogprobs)
			}
			extractionErr = fmt.Errorf("%s extraction required from provider %s: %w", modeName, r.Provider.Name(), extractionErr)
			if err := observeScore(ctx, criterionID, request, resp, out.Evidence, out.ReplayStatus, replaySource, extractionErr); err != nil {
				return ScoreOutcome{}, err
			}
			if extractionAttempt < fullVerifierExtractionRetries {
				if auditErr := r.audit(ctx, criterionID, request, resp, out, extractionErr.Error()+"; retrying"); auditErr != nil {
					return ScoreOutcome{}, auditErr
				}
				// Retrying over the pooled connection would reach the same
				// upstream that just answered without an extractable
				// distribution. Drop idle connections so the next attempt
				// opens a fresh one and can land on a different one.
				if closer, ok := r.Provider.(provider.IdleConnectionCloser); ok {
					closer.CloseIdleConnections()
				}
				log.L().Warn("score extraction incomplete; retrying on a fresh connection",
					"provider", r.Provider.Name(),
					"model", r.Cfg.Model,
					"attempt", extractionAttempt+1,
					"of", fullVerifierExtractionRetries,
				)
				continue
			}
			auditErr := r.audit(ctx, criterionID, request, resp, out, extractionErr.Error())
			return ScoreOutcome{}, errors.Join(extractionErr, auditErr)
		}
		if err := observeScore(ctx, criterionID, request, resp, out.Evidence, out.ReplayStatus, replaySource, nil); err != nil {
			return ScoreOutcome{}, err
		}
		if r.Cache != nil && scoresExtractable(out, scoreTags) {
			if err := r.Cache.Set(request, resp); err != nil {
				return ScoreOutcome{}, fmt.Errorf("persist exact response cache: %w", err)
			}
		}
		if err := r.audit(ctx, criterionID, request, resp, out, ""); err != nil {
			return ScoreOutcome{}, err
		}
		return out, nil
	}
}

func exactReplaySourceForObservation(value provider.Provider, status provider.ReplayStatus) (*provider.ExactReplaySource, error) {
	if status != provider.ReplayStatusExact {
		return nil, nil
	}
	sourceProvider, ok := value.(provider.ExactReplaySourceProvider)
	if !ok {
		return nil, nil
	}
	source, available := sourceProvider.ExactReplaySource()
	if !available {
		return nil, nil
	}
	if err := source.Validate(); err != nil {
		return nil, fmt.Errorf("validate exact replay source: %w", err)
	}
	return &source, nil
}

// CompleteRequestLineage combines caller references with service-derived
// source, mapping, criterion, sampling, and policy evidence. Callers never
// provide the generated source or mapping digests.
func CompleteRequestLineage(base provider.RequestLineage, criterionID, samplingSlot, entrypoint string, summaries []preprocess.AccountingSummary) provider.RequestLineage {
	base.CriterionID = criterionID
	base.SamplingSlot = samplingSlot
	base.Entrypoint = entrypoint
	if base.PolicyHash == "" {
		return base
	}
	sourceDigests := make([]string, len(summaries))
	mappingDigests := make([]string, len(summaries))
	for index, summary := range summaries {
		sourceDigests[index] = summary.SourceDigest
		mappingDigests[index] = summary.TraceMappingDigest
	}
	base.SourceTraceHash = preprocess.Hash(strings.Join(sourceDigests, "\x00"))
	base.TraceMapHash = preprocess.Hash(strings.Join(mappingDigests, "\x00"))
	return base
}

func (r *Runner) acquireProviderSlot(ctx context.Context) (func(), error) {
	r.limiterOnce.Do(func() {
		workers := r.Cfg.MaxWorkers
		if workers <= 0 {
			workers = 8
		}
		r.providerLimiter = make(chan struct{}, workers)
	})
	select {
	case r.providerLimiter <- struct{}{}:
		return func() { <-r.providerLimiter }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// scoresExtractable reports whether every requested score tag can be
// extracted through the active mode. Full-verifier entries require score-token
// distributions; judge-mode entries require closing tags in the raw text.
func scoresExtractable(out ScoreOutcome, scoreTags []string) bool {
	for _, tag := range scoreTags {
		if !out.Evidence[tag].Extracted {
			return false
		}
	}
	return true
}

func (r *Runner) extractionMode() verifier.ExtractionMode {
	if r.Cfg.JudgeMode {
		return verifier.ExtractionModeJudge
	}
	return verifier.ExtractionModeVerifier
}

func (r *Runner) audit(ctx context.Context, criterionID string, request provider.RequestEnvelope, response provider.ResponseRecord, out ScoreOutcome, errMsg string) error {
	if r.Audit == nil {
		return nil
	}
	var estCost *float64
	if r.Cost != nil {
		estCost = r.Cost.Estimate(out.InputTokens, out.OutputTokens, out.CachedTokens)
	}
	meta := auditMetaFromContext(ctx)
	if err := r.Audit.Write(audit.Entry{
		EventType:             "provider_call",
		Provider:              r.Provider.Name(),
		Model:                 r.Cfg.Model,
		CriterionID:           criterionID,
		RequestFingerprint:    string(out.RequestFingerprint),
		ResponseDigest:        out.ResponseDigest,
		ServedModel:           out.ServedModel,
		ProviderRequestID:     response.ProviderRequestID,
		ReplayStatus:          out.ReplayStatus,
		ReplayReason:          out.ReplayReason,
		ParserContractVersion: response.ParserContractVersion,
		Lineage:               request.Lineage,
		InputTokens:           out.InputTokens,
		OutputTokens:          out.OutputTokens,
		CachedTokens:          out.CachedTokens,
		EstCostUSD:            estCost,
		CacheHit:              out.FromCache,
		CacheNamespace:        out.CacheNamespace,
		Logprobs:              out.HasLogprobs && !r.Cfg.JudgeMode,
		ScoreEvidence:         out.Evidence,
		RedactionHits:         meta.RedactionHits,
		TrajectoryEvidence:    meta.TrajectoryEvidence,
		Error:                 log.Redact(errMsg),
	}); err != nil {
		return fmt.Errorf("write score audit: %w", err)
	}
	return nil
}

func (r *Runner) auditProviderAttempt(ctx context.Context, criterionID string, request provider.RequestEnvelope, attempt int, result provider.AttemptResult) error {
	if r.Audit == nil {
		return nil
	}
	fingerprint, err := request.Fingerprint()
	if err != nil {
		return err
	}
	status := "complete"
	errorMessage := ""
	if result.Err != nil {
		status = "failed"
		errorMessage = log.Redact(result.Err.Error())
	}
	meta := auditMetaFromContext(ctx)
	if err := r.Audit.Write(audit.Entry{
		EventType: "provider_attempt", Provider: r.Provider.Name(), Model: r.Cfg.Model,
		CriterionID: criterionID, RequestFingerprint: string(fingerprint), Lineage: request.Lineage,
		ProviderAttempt: attempt, AttemptStatus: status, InputTokens: result.Usage.Input,
		OutputTokens: result.Usage.Output, CachedTokens: result.Usage.Cached,
		RedactionHits: meta.RedactionHits, TrajectoryEvidence: meta.TrajectoryEvidence, Error: errorMessage,
	}); err != nil {
		return fmt.Errorf("write provider-attempt audit: %w", err)
	}
	return nil
}

func baseCriterionID(value string) string {
	if index := strings.LastIndex(value, "@r"); index > 0 {
		if _, err := strconv.Atoi(value[index+2:]); err == nil {
			return value[:index]
		}
	}
	return value
}

// PairScores maps criterion id to (scoreA, scoreB) for a single pair-rep.
type PairScores map[string][2]float64

// PairCriterionEvidence preserves the distribution statistics that decide
// whether a pair needs another order/sample. Scores alone cannot distinguish
// a sharp verdict from a broad Top-20 distribution with the same mean.
type PairCriterionEvidence struct {
	A verifier.ScoreEvidence `json:"a"`
	B verifier.ScoreEvidence `json:"b"`
}

type PairEvidenceScores map[string]PairCriterionEvidence

// AbsoluteScores maps criterion id to score for a single trajectory-rep.
type AbsoluteScores map[string]float64

type AbsoluteEvidenceScores map[string]verifier.ScoreEvidence

// JointAbsoluteEvidenceScores is indexed by candidate, then criterion ID.
// Every entry comes from the same immutable provider response for one rep.
type JointAbsoluteEvidenceScores []AbsoluteEvidenceScores

// repCacheID suffixes the cache criterion id with the sample index so
// repeated verifications (temperature > 0) are independent samples instead of
// disk-cache replays of rep 0. Rep 0 keeps the plain id, so existing cache
// entries stay valid.
func repCacheID(criterionID string, rep int) string {
	if rep <= 0 {
		return criterionID
	}
	return fmt.Sprintf("%s@r%d", criterionID, rep)
}

// ScorePair scores one pair of trajectories on all given criteria for one
// verification rep. When bundling is enabled and there are 2+ criteria, a
// single bundled prompt is used; otherwise N separate calls.
func (r *Runner) ScorePair(ctx context.Context, task, traceA, traceB string, criteria []verifier.Criterion, rep int) (PairScores, UsageSummary, error) {
	evidence, usage, err := r.ScorePairEvidence(ctx, task, traceA, traceB, criteria, rep)
	scores := make(PairScores, len(evidence))
	for criterionID, score := range evidence {
		scores[criterionID] = [2]float64{evidenceScore(score.A), evidenceScore(score.B)}
	}
	return scores, usage, err
}

// ScorePairEvidence scores one pair while retaining expectation, variance,
// score-token mass, and extraction provenance for every criterion.
func (r *Runner) ScorePairEvidence(ctx context.Context, task, traceA, traceB string, criteria []verifier.Criterion, rep int) (PairEvidenceScores, UsageSummary, error) {
	usage := UsageSummary{}
	scores := PairEvidenceScores{}
	if r.Cfg.MultiCriterionBundle && len(criteria) > 1 {
		prompt, critTags := verifier.PromptPairwiseBundled(task, traceA, traceB, criteria, verifier.PromptOptions{CritiqueThenScore: r.Cfg.CritiqueThenScore})
		key := repCacheID(bundledKey(criteria, "pair"), rep)
		out, err := r.Score(ctx, key, prompt, verifier.AllTags(critTags), r.maxTokensFor(len(criteria)))
		if err != nil {
			return scores, usage, err
		}
		r.mergeUsage(&usage, out)
		for _, ct := range critTags {
			a := out.Evidence[ct.Tags[0]]
			b := out.Evidence[ct.Tags[1]]
			countUnextracted(&usage, a, b)
			scores[ct.CriterionID] = PairCriterionEvidence{A: a, B: b}
		}
		return scores, usage, nil
	}
	for _, c := range criteria {
		prompt, tags := verifier.PromptPairwise(task, traceA, traceB, c, verifier.PromptOptions{CritiqueThenScore: r.Cfg.CritiqueThenScore})
		out, err := r.Score(ctx, repCacheID(c.ID, rep), prompt, tags, r.maxTokensFor(1))
		if err != nil {
			return scores, usage, err
		}
		r.mergeUsage(&usage, out)
		a := out.Evidence["<score_A>"]
		b := out.Evidence["<score_B>"]
		countUnextracted(&usage, a, b)
		scores[c.ID] = PairCriterionEvidence{A: a, B: b}
	}
	return scores, usage, nil
}

// ScoreSingle scores one trajectory on all given criteria for one rep.
func (r *Runner) ScoreSingle(ctx context.Context, task, trace string, criteria []verifier.Criterion, rep int) (AbsoluteScores, UsageSummary, error) {
	evidence, usage, err := r.ScoreSingleEvidence(ctx, task, trace, criteria, rep)
	scores := make(AbsoluteScores, len(evidence))
	for criterionID, score := range evidence {
		scores[criterionID] = evidenceScore(score)
	}
	return scores, usage, err
}

func (r *Runner) ScoreSingleEvidence(ctx context.Context, task, trace string, criteria []verifier.Criterion, rep int) (AbsoluteEvidenceScores, UsageSummary, error) {
	usage := UsageSummary{}
	scores := AbsoluteEvidenceScores{}
	if r.Cfg.MultiCriterionBundle && len(criteria) > 1 {
		prompt, critTags := verifier.PromptAbsoluteBundled(task, trace, criteria, verifier.PromptOptions{CritiqueThenScore: r.Cfg.CritiqueThenScore})
		key := repCacheID(bundledKey(criteria, "abs"), rep)
		out, err := r.Score(ctx, key, prompt, verifier.AllTags(critTags), r.maxTokensFor(len(criteria)))
		if err != nil {
			return scores, usage, err
		}
		r.mergeUsage(&usage, out)
		for _, ct := range critTags {
			s := out.Evidence[ct.Tags[0]]
			countUnextracted(&usage, s)
			scores[ct.CriterionID] = s
		}
		return scores, usage, nil
	}
	for _, c := range criteria {
		prompt, tags := verifier.PromptAbsolute(task, trace, c, verifier.PromptOptions{CritiqueThenScore: r.Cfg.CritiqueThenScore})
		out, err := r.Score(ctx, repCacheID(c.ID+"_abs", rep), prompt, tags, r.maxTokensFor(1))
		if err != nil {
			return scores, usage, err
		}
		r.mergeUsage(&usage, out)
		s := out.Evidence["<score>"]
		countUnextracted(&usage, s)
		scores[c.ID] = s
	}
	return scores, usage, nil
}

// JointAbsoluteCriterionID is the stable lineage/cache identity shared by
// request planning and execution for one joint candidate-set observation.
func JointAbsoluteCriterionID(criteria []verifier.Criterion) string {
	return bundledKey(criteria, "joint_abs")
}

func (r *Runner) ScoreJointAbsoluteEvidence(ctx context.Context, task string, traces []string, criteria []verifier.Criterion, rep int) (JointAbsoluteEvidenceScores, UsageSummary, error) {
	usage := UsageSummary{}
	scores := make(JointAbsoluteEvidenceScores, len(traces))
	for index := range scores {
		scores[index] = AbsoluteEvidenceScores{}
	}
	prompt, criterionTags := verifier.PromptJointAbsolute(task, traces, criteria, verifier.PromptOptions{CritiqueThenScore: r.Cfg.CritiqueThenScore})
	key := repCacheID(JointAbsoluteCriterionID(criteria), rep)
	out, err := r.Score(ctx, key, prompt, verifier.AllTags(criterionTags), r.maxTokensFor(len(traces)*len(criteria)))
	if err != nil {
		return scores, usage, err
	}
	r.mergeUsage(&usage, out)
	for _, criterion := range criterionTags {
		if len(criterion.Tags) != len(traces) {
			return scores, usage, fmt.Errorf("joint-absolute criterion %q has %d tags for %d trajectories", criterion.CriterionID, len(criterion.Tags), len(traces))
		}
		for index, tag := range criterion.Tags {
			evidence := out.Evidence[tag]
			countUnextracted(&usage, evidence)
			scores[index][criterion.CriterionID] = evidence
		}
	}
	return scores, usage, nil
}

// countUnextracted is a defensive invariant counter. Strict runtime paths
// reject before reaching aggregation, so any non-zero result is a defect.
func countUnextracted(u *UsageSummary, results ...verifier.ScoreEvidence) {
	for _, res := range results {
		if !res.Extracted {
			u.UnextractedScores++
		}
	}
}

func evidenceScore(evidence verifier.ScoreEvidence) float64 {
	if evidence.ConditionalExpectedScore == nil {
		return 0.5
	}
	return *evidence.ConditionalExpectedScore
}

func evidenceVariance(evidence verifier.ScoreEvidence) float64 {
	if evidence.ConditionalVariance == nil {
		return 0
	}
	return *evidence.ConditionalVariance
}

func bundledKey(criteria []verifier.Criterion, kind string) string {
	ids := make([]string, len(criteria))
	for i, c := range criteria {
		ids[i] = c.ID
	}
	sort.Strings(ids)
	return "_bundled_" + kind + "_" + strings.Join(ids, ",")
}

func (r *Runner) addCost(usage *UsageSummary, out ScoreOutcome) {
	if r.Cost == nil {
		return
	}
	if est := r.Cost.Estimate(out.InputTokens, out.OutputTokens, out.CachedTokens); est != nil {
		usage.EstCostUSD += *est
	}
}

// DefaultMaxTokens matches the reference implementation (verifier_core.py
// max_output_tokens=4096). The prompt asks the model to analyze BEFORE
// emitting score tags, so tight caps truncate the answer mid-analysis and
// strict extraction fails. Once all tags appear, streaming freezes semantic
// evidence, performs a bounded drain for terminal usage, and then cancels.
const DefaultMaxTokens = 4096

// maxTokensFor returns the per-call output budget. An explicit
// EVALWITNESS_MAX_TOKENS always wins.
func (r *Runner) maxTokensFor(_ int) int {
	if r.Cfg.MaxTokens > 0 {
		return r.Cfg.MaxTokens
	}
	return DefaultMaxTokens
}

func effectiveTemperature(v float64) float64 {
	if v == 0 {
		return 1.0
	}
	return v
}

func effectiveTopLogprobs(v int) int {
	if v <= 0 {
		return 20
	}
	return v
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0.5
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func clamp01(f float64) float64 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	}
	return f
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func signOf(f float64) int {
	switch {
	case f > 0:
		return 1
	case f < 0:
		return -1
	}
	return 0
}

func (r *Runner) mergeUsage(dst *UsageSummary, src ScoreOutcome) {
	dst.Calls++
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CachedTokens += src.CachedTokens
	if src.EstimatedInput {
		dst.EstimatedInputCalls++
	}
	if src.FromCache {
		dst.CacheHitCalls++
	}
	if src.CacheNamespace == cache.SourceLegacy {
		dst.LegacyCacheHitCalls++
	}
	if src.HasLogprobs && !r.Cfg.JudgeMode {
		dst.LogprobCalls++
	} else {
		dst.JudgeTextCalls++
	}
	r.addCost(dst, src)
}

func AddUsage(dst *UsageSummary, src UsageSummary) {
	dst.Calls += src.Calls
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CachedTokens += src.CachedTokens
	dst.CacheHitCalls += src.CacheHitCalls
	dst.LegacyCacheHitCalls += src.LegacyCacheHitCalls
	dst.LogprobCalls += src.LogprobCalls
	dst.JudgeTextCalls += src.JudgeTextCalls
	dst.UnextractedScores += src.UnextractedScores
	dst.EstimatedInputCalls += src.EstimatedInputCalls
	dst.EstCostUSD += src.EstCostUSD
	dst.updateExtractionMode()
}
