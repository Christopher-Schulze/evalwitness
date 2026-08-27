package provider

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/log"
)

type openaiProvider struct {
	cfg  Config
	http *http.Client
}

// CloseIdleConnections drops pooled connections so the next request opens a new
// one. Used by the extraction-retry path; see IdleConnectionCloser.
func (p *openaiProvider) CloseIdleConnections() {
	if p.http != nil {
		p.http.CloseIdleConnections()
	}
}

func newOpenAI(cfg Config) (Provider, error) {
	if cfg.APIKey == "" && cfg.Name != "ollama" {
		return nil, fmt.Errorf("api key required for provider %q", cfg.Name)
	}
	if cfg.UpstreamProvider != "" {
		upstreamProvider := strings.TrimSpace(cfg.UpstreamProvider)
		if upstreamProvider == "" || cfg.UpstreamProvider != upstreamProvider {
			return nil, errors.New("upstream provider must be non-empty and whitespace-free")
		}
		if cfg.Name != "openrouter-"+upstreamProvider {
			return nil, fmt.Errorf("provider label %q must bind pinned upstream %q as %q", cfg.Name, upstreamProvider, "openrouter-"+upstreamProvider)
		}
	}
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	httpClient, err := newOpenAIHTTPClient(timeout, cfg.CAFile)
	if err != nil {
		return nil, err
	}
	return &openaiProvider{
		cfg:  cfg,
		http: httpClient,
	}, nil
}

func newOpenAIHTTPClient(timeout time.Duration, caFile string) (*http.Client, error) {
	client := &http.Client{Timeout: timeout}
	if caFile == "" {
		return client, nil
	}
	pemData, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read EVALWITNESS_CA_FILE: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("EVALWITNESS_CA_FILE contains no PEM certificates: %s", caFile)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
	}
	client.Transport = transport
	return client, nil
}

func (p *openaiProvider) Name() string               { return p.cfg.Name }
func (p *openaiProvider) Capabilities() Capabilities { return p.cfg.Caps }

type openaiChatRequest struct {
	Model       string                 `json:"model"`
	Messages    []openaiMessage        `json:"messages"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature"`
	TopP        float64                `json:"top_p,omitempty"`
	Logprobs    bool                   `json:"logprobs,omitempty"`
	TopLogprobs int                    `json:"top_logprobs,omitempty"`
	Stop        []string               `json:"stop,omitempty"`
	Stream      bool                   `json:"stream"`
	LogitBias   map[string]int         `json:"logit_bias,omitempty"`
	Thinking    *openaiThinking        `json:"thinking,omitempty"`
	Provider    *openaiProviderRouting `json:"provider,omitempty"`
	Seed        *int                   `json:"seed,omitempty"`
	// StreamOptions requests the usage chunk in SSE streams (OpenAI-compat
	// providers omit usage from streams unless asked).
	StreamOptions *openaiStreamOptions `json:"stream_options,omitempty"`
}

type openaiProviderRouting struct {
	Only              []string `json:"only"`
	RequireParameters bool     `json:"require_parameters"`
	AllowFallbacks    bool     `json:"allow_fallbacks"`
}

type openaiStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiThinking struct {
	Type string `json:"type"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiChatResponse struct {
	Model   string         `json:"model"`
	ID      string         `json:"id"`
	Choices []openaiChoice `json:"choices"`
	Usage   *openaiUsage   `json:"usage"`
	Error   *openaiError   `json:"error,omitempty"`
}

type openaiChoice struct {
	Index        int             `json:"index"`
	Message      openaiMessage   `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     *openaiLogprobs `json:"logprobs"`
}

type openaiLogprobs struct {
	Content []openaiLogprobToken `json:"content"`
}

type openaiLogprobToken struct {
	Token       string                     `json:"token"`
	Logprob     float64                    `json:"logprob"`
	TopLogprobs []openaiLogprobAlternative `json:"top_logprobs"`
}

type openaiLogprobAlternative struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

type logprobTokenSpan struct {
	start int
	end   int
	token openaiLogprobToken
}

type openaiUsage struct {
	PromptTokens         int                 `json:"prompt_tokens"`
	CompletionTokens     int                 `json:"completion_tokens"`
	PromptCacheHitTokens int                 `json:"prompt_cache_hit_tokens,omitempty"`
	CacheReadInputTokens int                 `json:"cache_read_input_tokens,omitempty"`
	PromptTokensDetails  *promptTokenDetails `json:"prompt_tokens_details,omitempty"`
}

type promptTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type openaiError struct {
	Message string          `json:"message"`
	Type    string          `json:"type"`
	Code    json.RawMessage `json:"code"`
}

func (p *openaiProvider) Score(ctx context.Context, req RequestEnvelope) (ResponseRecord, error) {
	if p.cfg.Offline {
		return ResponseRecord{}, ErrOffline
	}
	if !p.cfg.LiveIntent {
		return ResponseRecord{}, ErrLiveIntentRequired
	}
	if err := p.validateRequest(req); err != nil {
		return ResponseRecord{}, err
	}

	maxRetries := p.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 5
	}

	doOnce := p.doScore
	if req.Stream {
		doOnce = p.doScoreStream
	}

	var lastErr error
	var lastResp ResponseRecord
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if req.BeforeAttempt != nil {
			if err := req.BeforeAttempt(); err != nil {
				return lastResp, err
			}
		}
		resp, err := doOnce(ctx, req)
		if req.AfterAttempt != nil {
			if completionErr := req.AfterAttempt(AttemptResult{Usage: resp.Usage, UsageObserved: resp.UsageObserved, Err: err}); completionErr != nil {
				return resp, errors.Join(err, completionErr)
			}
		}
		if err == nil {
			return FinalizeResponse(req, resp)
		}
		lastErr = err
		lastResp = resp

		var pe *ProviderError
		if !errors.As(err, &pe) {
			if errors.Is(err, ErrMalformedResponse) {
				return resp, err
			}
			if isContextErr(err) {
				// Only bail when the CALLER's context ended (cancel/deadline).
				// A per-request http.Client timeout also surfaces as a
				// context error but is a transient provider condition
				// (server-side queuing) and must be retried.
				if ctx.Err() != nil {
					return resp, err
				}
			}
			if attempt < 3 {
				if waitErr := waitBackoff(ctx, attempt, 2*time.Second, 60*time.Second, 0); waitErr != nil {
					return resp, waitErr
				}
				continue
			}
			return resp, err
		}

		switch pe.Class {
		case ClassRetryable, ClassRateLimited:
			retryAfter := time.Duration(pe.RetryAfter) * time.Second
			if attempt >= maxRetries {
				return resp, err
			}
			if isBAITransientLogprobsError(p.cfg.Name, pe.Status, pe.Body) {
				p.CloseIdleConnections()
			}
			if pe.Class == ClassRateLimited {
				log.L().Warn("provider rate limited; waiting before retry",
					"provider", p.cfg.Name,
					"status", pe.Status,
					"retry_after_seconds", pe.RetryAfter,
					"attempt", attempt+1,
					"max_attempts", maxRetries+1,
				)
			}
			if waitErr := waitBackoff(ctx, attempt, 1*time.Second, 60*time.Second, retryAfter); waitErr != nil {
				return resp, waitErr
			}
			continue
		default:
			return resp, err
		}
	}
	return lastResp, lastErr
}

func (p *openaiProvider) doScore(ctx context.Context, req RequestEnvelope) (record ResponseRecord, returnErr error) {
	messages := make([]openaiMessage, len(req.Messages))
	for index, message := range req.Messages {
		messages[index] = openaiMessage{Role: message.Role, Content: message.Content}
	}
	body := openaiChatRequest{
		Model:       req.RequestedModel,
		Messages:    messages,
		MaxTokens:   req.MaxOutputTokens,
		Temperature: req.Temperature,
		Stop:        req.Stop,
		Stream:      false,
		Seed:        req.Seed,
	}
	if req.TopLogprobs > 0 {
		body.Logprobs = true
		body.TopLogprobs = clampTopLogprobs(req.TopLogprobs, p.cfg.Caps.TopLogprobsMax)
	}
	if p.cfg.Caps.LogitBias && len(req.LogitBias) > 0 {
		body.LogitBias = req.LogitBias
	}
	applyThinkingMode(&body, req.ThinkingMode)
	applyUpstreamProvider(&body, p.cfg.UpstreamProvider)

	buf, err := json.Marshal(body)
	if err != nil {
		return ResponseRecord{}, err
	}
	requestURL := req.BaseURLOrigin + req.EndpointPath
	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(buf))
	if err != nil {
		return ResponseRecord{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if p.cfg.UserAgent != "" {
		httpReq.Header.Set("User-Agent", p.cfg.UserAgent)
	}
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	resp, err := p.http.Do(httpReq)
	if err != nil {
		return ResponseRecord{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close provider response body: %w", closeErr)
		}
	}()
	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return ResponseRecord{}, err
	}
	if resp.StatusCode >= 400 {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		return ResponseRecord{}, &ProviderError{
			Provider:   p.cfg.Name,
			Status:     resp.StatusCode,
			Body:       string(rb),
			Class:      classifyProviderError(p.cfg.Name, resp.StatusCode, string(rb)),
			RetryAfter: retryAfter,
		}
	}
	var parsed openaiChatResponse
	if err := json.Unmarshal(rb, &parsed); err != nil {
		return ResponseRecord{}, malformedResponseError(p.cfg.Name, fmt.Sprintf("parse JSON: %v (body: %s)", err, truncate(string(rb), 512)))
	}
	if parsed.Error != nil {
		status := openAIErrorStatus(parsed.Error.Code, resp.StatusCode)
		return ResponseRecord{}, &ProviderError{
			Provider: p.cfg.Name,
			Status:   status,
			Body:     parsed.Error.Message,
			Class:    classifyProviderError(p.cfg.Name, status, parsed.Error.Message),
		}
	}
	if len(parsed.Choices) == 0 {
		return ResponseRecord{}, malformedResponseError(p.cfg.Name, "empty choices")
	}
	choice := parsed.Choices[0]
	usage := openaiUsage{}
	if parsed.Usage != nil {
		usage = *parsed.Usage
	}
	cached := usage.PromptCacheHitTokens
	if usage.CacheReadInputTokens > cached {
		cached = usage.CacheReadInputTokens
	}
	if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > cached {
		cached = usage.PromptTokensDetails.CachedTokens
	}
	out := ResponseRecord{
		RawText:           choice.Message.Content,
		Usage:             TokenUsage{Input: usage.PromptTokens, Output: usage.CompletionTokens, Cached: cached},
		UsageObserved:     parsed.Usage != nil,
		ProviderRequestID: parsed.ID,
		ServedModel:       parsed.Model,
		FinishReason:      choice.FinishReason,
		NormalizedBody:    append([]byte(nil), rb...),
		Distributions:     map[string]map[string]float64{},
	}
	if choice.Logprobs != nil && len(choice.Logprobs.Content) > 0 {
		out.HasLogprobs = true
		maxTL := 0
		for _, tok := range choice.Logprobs.Content {
			if len(tok.TopLogprobs) > maxTL {
				maxTL = len(tok.TopLogprobs)
			}
		}
		out.ObservedTopLogprobs = maxTL
		out.DegenerateLogprobs = logprobsAreDegenerate(choice.Logprobs.Content)
		out.OrderedTokenEvidence = tokenEvidence(choice.Logprobs.Content)
		if len(req.ScoreTags) > 0 {
			out.Distributions = extractDistributions(choice.Message.Content, choice.Logprobs.Content, req.ScoreTags)
		}
	}
	return out, nil
}

func openAIErrorStatus(code json.RawMessage, fallback int) int {
	var numeric int
	if json.Unmarshal(code, &numeric) == nil && numeric >= 400 && numeric <= 599 {
		return numeric
	}
	var text string
	if json.Unmarshal(code, &text) == nil {
		if numeric, err := strconv.Atoi(text); err == nil && numeric >= 400 && numeric <= 599 {
			return numeric
		}
	}
	return fallback
}

func classifyError(status int, body string) ErrorClass {
	bl := strings.ToLower(body)
	switch {
	case status == http.StatusTooManyRequests:
		return ClassRateLimited
	case status == http.StatusPaymentRequired:
		return ClassPaymentRequired
	case strings.Contains(bl, "context") || strings.Contains(bl, "prompt tokens limit") || strings.Contains(bl, "length") || strings.Contains(bl, "too long"):
		return ClassContextOverflow
	case strings.Contains(bl, "rate") || strings.Contains(bl, "limit") || strings.Contains(bl, "quota") || strings.Contains(bl, "tpm") || strings.Contains(bl, "rpm"):
		return ClassRateLimited
	case strings.Contains(bl, "logprobs") || strings.Contains(bl, "top_logprobs"):
		return ClassCapabilityMissing
	case strings.Contains(bl, "auth") || strings.Contains(bl, "unauthorized") || strings.Contains(bl, "invalid api key") || strings.Contains(bl, "forbidden"):
		return ClassAuthFailed
	case strings.Contains(bl, "model") && (strings.Contains(bl, "not found") || strings.Contains(bl, "does not exist")):
		return ClassBadConfig
	}
	if status == 408 || status == 429 || status >= 500 {
		return ClassRetryable
	}
	return ClassUnknown
}

func classifyProviderError(providerName string, status int, body string) ErrorClass {
	if isBAITransientLogprobsError(providerName, status, body) {
		return ClassRetryable
	}
	return classifyError(status, body)
}

func isBAITransientLogprobsError(providerName string, status int, body string) bool {
	if providerName != "bai" || status != http.StatusBadRequest {
		return false
	}
	body = strings.ToLower(body)
	return strings.Contains(body, "invalidparameter") &&
		strings.Contains(body, "`logprobs`") &&
		strings.Contains(body, "is not supported")
}

func waitBackoff(ctx context.Context, attempt int, base, ceiling, retryAfter time.Duration) error {
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if delay > ceiling {
		delay = ceiling
	}
	jitter := time.Duration(rand.Int63n(int64(time.Second)))
	delay += jitter
	if retryAfter > ceiling {
		retryAfter = ceiling
	}
	if retryAfter > delay {
		delay = retryAfter
	}
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func parseRetryAfter(value string, now time.Time) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds > 0 {
			return seconds
		}
		return 0
	}
	deadline, err := http.ParseTime(value)
	if err != nil || !deadline.After(now) {
		return 0
	}
	return int(math.Ceil(deadline.Sub(now).Seconds()))
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func extractDistributions(rawText string, tokens []openaiLogprobToken, tags []string) map[string]map[string]float64 {
	// Prefer coordinates in the provider's own token stream. Some gateways'
	// DeepSeek upstream omits markup fragments such as "<" and "</" from
	// logprobs.content, so raw-text greedy alignment can drift across repeated
	// words in long analyses even though the score token itself is present.
	out := extractTokenStreamDistributions(rawText, tokens, tags)
	if len(out) == len(tags) {
		debugDistributionMasses(out, tags)
		return out
	}
	for tag, dist := range extractAlignedDistributions(rawText, tokens, tags) {
		if len(out[tag]) == 0 {
			out[tag] = dist
		}
	}
	debugDistributionMasses(out, tags)
	return out
}

func debugDistributionMasses(distributions map[string]map[string]float64, tags []string) {
	for _, tag := range tags {
		mass := 0.0
		for token, probability := range distributions[tag] {
			if len(token) == 1 && isScoreLetter(token[0]) {
				mass += probability
			}
		}
		log.L().Debug("score-token distribution extracted",
			"tag", tag,
			"score_mass", mass,
			"alternatives", len(distributions[tag]),
		)
	}
}

// extractAlignedDistributions maps logprob token records back onto the raw
// response. Some OpenAI-compatible gateways omit markup tokens such as "<"
// and "</" from logprobs.content even though they remain in message.content.
// Sequential alignment preserves the score-token offset across those gaps.
func extractAlignedDistributions(rawText string, tokens []openaiLogprobToken, tags []string) map[string]map[string]float64 {
	type span struct {
		start int
		end   int
		token openaiLogprobToken
	}
	spans := make([]span, 0, len(tokens))
	cursor := 0
	for _, token := range tokens {
		if token.Token == "" || cursor >= len(rawText) {
			continue
		}
		relative := strings.Index(rawText[cursor:], token.Token)
		if relative < 0 {
			continue
		}
		start := cursor + relative
		end := start + len(token.Token)
		spans = append(spans, span{start: start, end: end, token: token})
		cursor = end
	}

	out := map[string]map[string]float64{}
	for _, tag := range tags {
		tagStart := strings.Index(rawText, tag)
		if tagStart < 0 {
			continue
		}
		scoreOffset := tagStart + len(tag)
		for _, current := range spans {
			if scoreOffset < current.start || scoreOffset >= current.end {
				continue
			}
			offset := scoreOffset - current.start
			dist := map[string]float64{}
			for _, alternative := range current.token.TopLogprobs {
				if score, ok := scoreLetterAtOffset(alternative.Token, offset); ok {
					setMaxScoreProbability(dist, score, math.Exp(alternative.Logprob))
				}
			}
			if score, ok := scoreLetterAtOffset(current.token.Token, offset); ok {
				setMaxScoreProbability(dist, score, math.Exp(current.token.Logprob))
			}
			if len(dist) > 0 {
				out[tag] = dist
			}
			break
		}
	}
	return out
}

func extractTokenStreamDistributions(rawText string, tokens []openaiLogprobToken, tags []string) map[string]map[string]float64 {
	var stream strings.Builder
	spans := make([]logprobTokenSpan, 0, len(tokens))
	for _, token := range tokens {
		start := stream.Len()
		stream.WriteString(token.Token)
		spans = append(spans, logprobTokenSpan{start: start, end: stream.Len(), token: token})
	}
	content := stream.String()
	out := map[string]map[string]float64{}
	for _, tag := range tags {
		expectedScore, hasExpectedScore := lastRawScoreForTag(rawText, tag)
		variants := []string{tag}
		if stripped := strings.TrimPrefix(tag, "<"); stripped != tag {
			variants = append(variants, stripped)
		}
		bestStart := -1
		bestVariant := ""
		var bestDistribution map[string]float64
		for _, variant := range variants {
			searchFrom := 0
			for searchFrom < len(content) {
				relative := strings.Index(content[searchFrom:], variant)
				if relative < 0 {
					break
				}
				tagStart := searchFrom + relative
				scoreOffset := tagStart + len(variant)
				if dist := distributionAtStreamOffset(spans, scoreOffset, expectedScore, hasExpectedScore); len(dist) > 0 && tagStart > bestStart {
					bestStart = tagStart
					bestVariant = variant
					bestDistribution = dist
				}
				searchFrom = tagStart + 1
			}
		}
		if len(bestDistribution) > 0 {
			out[tag] = bestDistribution
		}
		log.L().Debug("score-token coordinate selected",
			"tag", tag,
			"expected_score", string([]byte{expectedScore}),
			"expected_score_found", hasExpectedScore,
			"stream_offset", bestStart,
			"variant", bestVariant,
		)
	}
	return out
}

func lastRawScoreForTag(rawText, tag string) (byte, bool) {
	tagName := strings.Trim(tag, "<>")
	closingTag := "</" + tagName + ">"
	searchEnd := len(rawText)
	for searchEnd > 0 {
		tagStart := strings.LastIndex(rawText[:searchEnd], tag)
		if tagStart < 0 {
			return 0, false
		}
		scoreStart := tagStart + len(tag)
		scoreText := strings.TrimLeft(rawText[scoreStart:], " \t\n\r")
		if len(scoreText) > 0 && isScoreLetter(scoreText[0]) {
			afterScore := strings.TrimLeft(scoreText[1:], " \t\n\r")
			if strings.HasPrefix(afterScore, closingTag) {
				return scoreText[0], true
			}
		}
		searchEnd = tagStart
	}
	return 0, false
}

func distributionAtStreamOffset(spans []logprobTokenSpan, scoreOffset int, expectedScore byte, requireExpected bool) map[string]float64 {
	for _, current := range spans {
		if scoreOffset < current.start || scoreOffset >= current.end {
			continue
		}
		offset := scoreOffset - current.start
		chosen, ok := scoreLetterAtOffset(current.token.Token, offset)
		if !ok {
			return nil
		}
		if requireExpected && !strings.EqualFold(chosen, string([]byte{expectedScore})) {
			return nil
		}
		log.L().Debug("score-token span matched",
			"chosen", chosen,
			"token", current.token.Token,
			"token_offset", offset,
			"chosen_logprob", current.token.Logprob,
		)
		dist := map[string]float64{}
		for _, alternative := range current.token.TopLogprobs {
			if score, ok := scoreLetterAtOffset(alternative.Token, offset); ok {
				setMaxScoreProbability(dist, score, math.Exp(alternative.Logprob))
			}
		}
		if score, ok := scoreLetterAtOffset(current.token.Token, offset); ok {
			setMaxScoreProbability(dist, score, math.Exp(current.token.Logprob))
		}
		return dist
	}
	return nil
}

// degenerateAlternativeFloor sits far below any log probability a real softmax
// produces; observed sentinels are around -9999.
const degenerateAlternativeFloor = -100.0

// maxUsageDrainChunks bounds how long a stream is kept open after the score tags
// are complete, purely to collect the trailing usage chunk.
const maxUsageDrainChunks = 64

// logprobsAreDegenerate reports whether a whole response carries alternatives
// that are all pinned to the sentinel floor. It requires every token that has
// alternatives to look that way, so a single genuinely certain token in an
// otherwise informative response never trips it.
func logprobsAreDegenerate(tokens []openaiLogprobToken) bool {
	considered := 0
	for _, token := range tokens {
		if len(token.TopLogprobs) < 2 {
			continue
		}
		considered++
		informative := 0
		for _, alternative := range token.TopLogprobs {
			if alternative.Logprob > degenerateAlternativeFloor {
				informative++
			}
		}
		if informative > 1 {
			return false
		}
	}
	return considered >= 2
}

func setMaxScoreProbability(distribution map[string]float64, score string, probability float64) {
	if existing, found := distribution[score]; !found || probability > existing {
		distribution[score] = probability
	}
}

func scoreLetterAtOffset(token string, offset int) (string, bool) {
	if offset < 0 || offset >= len(token) {
		return "", false
	}
	c := token[offset]
	if isScoreLetter(c) {
		return string([]byte{c}), true
	}
	return "", false
}

func isScoreLetter(c byte) bool {
	return (c >= 'A' && c <= 'T') || (c >= 'a' && c <= 't')
}

func clampTopLogprobs(want, max int) int {
	if want <= 0 {
		want = 20
	}
	if max > 0 && want > max {
		want = max
	}
	return want
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

type streamChunk struct {
	Model   string         `json:"model"`
	ID      string         `json:"id"`
	Choices []streamChoice `json:"choices"`
	Usage   *openaiUsage   `json:"usage"`
}

type streamChoice struct {
	Index        int             `json:"index"`
	Delta        streamDelta     `json:"delta"`
	Logprobs     *openaiLogprobs `json:"logprobs"`
	FinishReason string          `json:"finish_reason"`
}

type streamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content"`
}

func (p *openaiProvider) doScoreStream(ctx context.Context, req RequestEnvelope) (record ResponseRecord, returnErr error) {
	messages := make([]openaiMessage, len(req.Messages))
	for index, message := range req.Messages {
		messages[index] = openaiMessage{Role: message.Role, Content: message.Content}
	}
	body := openaiChatRequest{
		Model:         req.RequestedModel,
		Messages:      messages,
		MaxTokens:     req.MaxOutputTokens,
		Temperature:   req.Temperature,
		Stop:          req.Stop,
		Stream:        true,
		Seed:          req.Seed,
		StreamOptions: &openaiStreamOptions{IncludeUsage: true},
	}
	if req.TopLogprobs > 0 {
		body.Logprobs = true
		body.TopLogprobs = clampTopLogprobs(req.TopLogprobs, p.cfg.Caps.TopLogprobsMax)
	}
	if p.cfg.Caps.LogitBias && len(req.LogitBias) > 0 {
		body.LogitBias = req.LogitBias
	}
	applyThinkingMode(&body, req.ThinkingMode)
	applyUpstreamProvider(&body, p.cfg.UpstreamProvider)
	buf, err := json.Marshal(body)
	if err != nil {
		return ResponseRecord{}, err
	}
	requestURL := req.BaseURLOrigin + req.EndpointPath
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(streamCtx, "POST", requestURL, bytes.NewReader(buf))
	if err != nil {
		return ResponseRecord{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.cfg.UserAgent != "" {
		httpReq.Header.Set("User-Agent", p.cfg.UserAgent)
	}
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	resp, err := p.http.Do(httpReq)
	if err != nil {
		return ResponseRecord{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close provider stream response body: %w", closeErr)
		}
	}()
	if resp.StatusCode >= 400 {
		rb, _ := io.ReadAll(resp.Body)
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		return ResponseRecord{}, &ProviderError{
			Provider:   p.cfg.Name,
			Status:     resp.StatusCode,
			Body:       string(rb),
			Class:      classifyProviderError(p.cfg.Name, resp.StatusCode, string(rb)),
			RetryAfter: retryAfter,
		}
	}

	var (
		contentB       strings.Builder
		allTokens      []openaiLogprobToken
		idResp         string
		servedModel    string
		usage          openaiUsage
		usageObserved  bool
		finishReason   string
		normalizedBody bytes.Buffer
	)
	tagsComplete := false
	drained := 0
	pendingClose := map[string]bool{}
	for _, t := range req.ScoreTags {
		if !strings.HasPrefix(t, "<") || !strings.HasSuffix(t, ">") {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(t, "<"), ">")
		pendingClose["</"+inner+">"] = true
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		normalizedBody.WriteString(data)
		normalizedBody.WriteByte('\n')
		var ch streamChunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return ResponseRecord{}, malformedResponseError(p.cfg.Name, fmt.Sprintf("parse stream chunk: %v", err))
		}
		if ch.ID != "" {
			idResp = ch.ID
		}
		if ch.Model != "" {
			servedModel = ch.Model
		}
		if ch.Usage != nil {
			usage = *ch.Usage
			usageObserved = true
		}
		for _, c := range ch.Choices {
			if c.FinishReason != "" {
				finishReason = c.FinishReason
			}
			if tagsComplete {
				continue
			}
			if c.Delta.Content != "" {
				contentB.WriteString(c.Delta.Content)
			}
			if c.Logprobs != nil {
				allTokens = append(allTokens, c.Logprobs.Content...)
			}
		}
		if !tagsComplete {
			text := contentB.String()
			for tag := range pendingClose {
				if strings.Contains(text, tag) {
					delete(pendingClose, tag)
				}
			}
			if len(pendingClose) == 0 && len(req.ScoreTags) > 0 {
				// The score tags are complete, but providers emit the usage chunk last.
				// Freeze semantic content and drain a bounded number of frames to pick
				// usage up without letting a rambling model alter the score evidence.
				tagsComplete = true
			}
		}
		if tagsComplete {
			if usage.PromptTokens > 0 || drained >= maxUsageDrainChunks {
				break
			}
			drained++
		}
	}
	if err := scanner.Err(); err != nil {
		return ResponseRecord{}, malformedResponseError(p.cfg.Name, fmt.Sprintf("read stream: %v", err))
	}

	cached := usage.PromptCacheHitTokens
	if usage.CacheReadInputTokens > cached {
		cached = usage.CacheReadInputTokens
	}
	if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > cached {
		cached = usage.PromptTokensDetails.CachedTokens
	}
	out := ResponseRecord{
		RawText:           contentB.String(),
		Usage:             TokenUsage{Input: usage.PromptTokens, Output: usage.CompletionTokens, Cached: cached},
		UsageObserved:     usageObserved,
		ProviderRequestID: idResp,
		ServedModel:       servedModel,
		FinishReason:      finishReason,
		NormalizedBody:    append([]byte(nil), normalizedBody.Bytes()...),
		Distributions:     map[string]map[string]float64{},
	}
	if len(allTokens) > 0 {
		out.HasLogprobs = true
		maxTL := 0
		for _, t := range allTokens {
			if len(t.TopLogprobs) > maxTL {
				maxTL = len(t.TopLogprobs)
			}
		}
		out.ObservedTopLogprobs = maxTL
		out.DegenerateLogprobs = logprobsAreDegenerate(allTokens)
		out.OrderedTokenEvidence = tokenEvidence(allTokens)
		out.Distributions = extractDistributions(out.RawText, allTokens, req.ScoreTags)
	}
	return out, nil
}

func applyThinkingMode(body *openaiChatRequest, thinkingMode string) {
	switch thinkingMode {
	case "enabled", "disabled":
		body.Thinking = &openaiThinking{Type: thinkingMode}
	}
}

func applyUpstreamProvider(body *openaiChatRequest, upstreamProvider string) {
	if upstreamProvider == "" {
		return
	}
	body.Provider = &openaiProviderRouting{
		Only:              []string{upstreamProvider},
		RequireParameters: true,
		AllowFallbacks:    false,
	}
}

func (p *openaiProvider) validateRequest(request RequestEnvelope) error {
	if err := request.Validate(); err != nil {
		return err
	}
	expectedOrigin, expectedPath, err := canonicalEndpoint(p.cfg.BaseURL)
	if err != nil {
		return err
	}
	if request.ProviderID != p.cfg.Name || request.RequestedModel != p.cfg.Model ||
		request.BaseURLOrigin != expectedOrigin || request.EndpointPath != expectedPath {
		return errors.New("canonical request route does not match configured provider")
	}
	if request.Stream && !p.cfg.Caps.Streaming {
		return errors.New("canonical request asks for unsupported streaming")
	}
	if request.ResponseFormat != ResponseFormatText {
		return fmt.Errorf("canonical request response format %q is not supported by the OpenAI-compatible adapter", request.ResponseFormat)
	}
	if p.cfg.Caps.TopLogprobsMax > 0 && request.TopLogprobs > p.cfg.Caps.TopLogprobsMax {
		return fmt.Errorf("canonical request top_logprobs %d exceeds the configured route maximum %d", request.TopLogprobs, p.cfg.Caps.TopLogprobsMax)
	}
	if len(request.LogitBias) > 0 && !p.cfg.Caps.LogitBias {
		return errors.New("canonical request logit_bias is not supported by the configured route")
	}
	for index, message := range request.Messages {
		if message.Name != "" || message.ToolCallID != "" {
			return fmt.Errorf("canonical request messages[%d] uses unsupported name or tool_call_id fields", index)
		}
	}
	return nil
}

func tokenEvidence(tokens []openaiLogprobToken) []TokenEvidence {
	out := make([]TokenEvidence, len(tokens))
	for index, token := range tokens {
		alternatives := make([]TokenAlternative, len(token.TopLogprobs))
		for alternativeIndex, alternative := range token.TopLogprobs {
			alternatives[alternativeIndex] = TokenAlternative{
				Token:   alternative.Token,
				Logprob: strconv.FormatFloat(alternative.Logprob, 'g', -1, 64),
			}
		}
		out[index] = TokenEvidence{
			Position:        index,
			Token:           token.Token,
			Logprob:         strconv.FormatFloat(token.Logprob, 'g', -1, 64),
			TopAlternatives: alternatives,
		}
	}
	return out
}

func malformedResponseError(providerName, detail string) error {
	return &ProviderError{
		Provider: providerName,
		Body:     detail,
		Class:    ClassMalformedResponse,
		Cause:    ErrMalformedResponse,
	}
}

func init() {
	Register("openai", newOpenAI)
}
