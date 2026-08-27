package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	RetryPolicyVersionV1 = "evalwitness.provider-retry.v1"
	RetryPolicyVersionV2 = "evalwitness.provider-retry.v2"
	RetryPolicyVersion   = "evalwitness.provider-retry.v3"
)

const ExactReplaySourceSchemaVersion = "evalwitness.exact-replay-source.v1"

type Provider interface {
	Name() string
	Score(ctx context.Context, req RequestEnvelope) (ResponseRecord, error)
	Capabilities() Capabilities
}

// SupportedRetryPolicyVersion accepts historical policy identities for replay
// while RetryPolicyVersion remains the only identity for new live bindings.
func SupportedRetryPolicyVersion(version string) bool {
	return version == RetryPolicyVersionV1 || version == RetryPolicyVersionV2 || version == RetryPolicyVersion
}

// ExactReplaySource binds replayed responses to one validated capture byte
// stream and its content-addressed request, lineage, response, and record sets.
type ExactReplaySource struct {
	SchemaVersion         string `json:"schema_version"`
	CaptureSHA256         string `json:"capture_sha256"`
	Bytes                 int64  `json:"bytes"`
	Records               int    `json:"records"`
	CaptureSchemaVersion  int    `json:"capture_schema_version"`
	RequestSchemaVersion  int    `json:"request_schema_version"`
	ParserContractVersion string `json:"parser_contract_version"`
	ProviderID            string `json:"provider_id"`
	RouteID               string `json:"route_id"`
	RequestedModel        string `json:"requested_model"`
	RequestSetDigest      string `json:"request_set_digest"`
	LineageSetDigest      string `json:"lineage_set_digest"`
	ResponseBodySetDigest string `json:"response_body_set_digest"`
	EvidenceSetDigest     string `json:"evidence_set_digest"`
	RecordSetDigest       string `json:"record_set_digest"`
}

func (source ExactReplaySource) Validate() error {
	if source.SchemaVersion != ExactReplaySourceSchemaVersion || !validExactReplayDigest(source.CaptureSHA256) ||
		source.Bytes < 1 || source.Records < 1 || source.Bytes < int64(source.Records) || strings.TrimSpace(source.ProviderID) == "" ||
		source.CaptureSchemaVersion != CaptureSchemaVersion || source.RequestSchemaVersion != RequestSchemaVersion ||
		source.ParserContractVersion != ParserContractVersion ||
		!strings.HasPrefix(source.RouteID, "route-") || !validExactReplayDigest(strings.TrimPrefix(source.RouteID, "route-")) ||
		strings.TrimSpace(source.RequestedModel) == "" || !validExactReplayDigest(source.RequestSetDigest) ||
		!validExactReplayDigest(source.LineageSetDigest) || !validExactReplayDigest(source.ResponseBodySetDigest) ||
		!validExactReplayDigest(source.EvidenceSetDigest) || !validExactReplayDigest(source.RecordSetDigest) {
		return errors.New("exact replay source identity or content digest is invalid")
	}
	return nil
}

// ExactReplaySourceProvider is implemented only when a provider can bind an
// exact response to the validated capture bytes from which it was loaded.
type ExactReplaySourceProvider interface {
	ExactReplaySource() (ExactReplaySource, bool)
}

func validExactReplayDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

type Capabilities struct {
	Logprobs       bool
	TopLogprobsMax int
	LogitBias      bool
	PromptCache    bool
	Streaming      bool
	ContextLimit   int
	MaxConcurrent  int
}

type AttemptResult struct {
	Usage         TokenUsage
	UsageObserved bool
	Err           error
}

type Config struct {
	Name             string
	WireFormat       string
	BaseURL          string
	APIKey           string
	CAFile           string
	Model            string
	UpstreamProvider string
	Caps             Capabilities
	Thinking         string
	Timeout          int
	Offline          bool
	LiveIntent       bool
	MaxRetries       int
	UserAgent        string
}

type ErrorClass int

const (
	ClassUnknown ErrorClass = iota
	ClassRetryable
	ClassRateLimited
	ClassPaymentRequired
	ClassContextOverflow
	ClassBadConfig
	ClassAuthFailed
	ClassCapabilityMissing
	ClassMalformedResponse
)

type ProviderError struct {
	Provider   string
	Status     int
	Body       string
	Class      ErrorClass
	RetryAfter int
	RequestID  string
	Cause      error
}

func (e *ProviderError) Error() string {
	body := e.Body
	if len(body) > 256 {
		body = body[:256] + "..."
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("provider %s http %d retry_after=%s (resumable %s): %s",
			e.Provider, e.Status, formatRetryAfter(e.RetryAfter), retryAfterClock(e.RetryAfter), body)
	}
	if e.Status == 0 {
		return fmt.Sprintf("provider %s malformed response: %s", e.Provider, body)
	}
	return fmt.Sprintf("provider %s http %d: %s", e.Provider, e.Status, body)
}

func (e *ProviderError) Unwrap() error { return e.Cause }

var ErrOffline = errors.New("offline mode: refused to make network call")
var ErrLiveIntentRequired = errors.New("live intent required: provider network dispatch was not explicitly authorized")
var ErrMalformedResponse = errors.New("provider response is malformed")

type Constructor func(cfg Config) (Provider, error)

var registry = map[string]Constructor{}

func Register(name string, fn Constructor) {
	registry[name] = fn
}

func New(cfg Config) (Provider, error) {
	wireFormat := cfg.WireFormat
	if wireFormat == "" {
		wireFormat = cfg.Name
	}
	fn, ok := registry[wireFormat]
	if !ok {
		return nil, fmt.Errorf("unknown wire format %q for provider %q", wireFormat, cfg.Name)
	}
	return fn(cfg)
}

func Names() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

var ErrLogprobsUnsupported = errors.New("provider does not support logprobs")

// formatRetryAfter renders a Retry-After the way an operator reads it: seconds
// stay seconds, longer waits become minutes or hours.
func formatRetryAfter(seconds int) string {
	d := time.Duration(seconds) * time.Second
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%.1fh", d.Hours())
	case d >= time.Minute:
		return fmt.Sprintf("%.0fm", d.Minutes())
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// retryAfterClock is the local wall-clock time at which the wait expires.
func retryAfterClock(seconds int) string {
	return nowFunc().Add(time.Duration(seconds) * time.Second).Format("2006-01-02 15:04")
}

// nowFunc is a seam so the message stays assertable in tests.
var nowFunc = time.Now

// IdleConnectionCloser is implemented by providers whose HTTP transport pools
// connections. Retrying a request that failed for a response-shape reason over
// the same pooled connection repeats it rather than retrying it: gateways that
// pin an upstream per connection will answer from the same upstream. Dropping
// idle connections first makes the next attempt open a fresh one.
type IdleConnectionCloser interface {
	CloseIdleConnections()
}
