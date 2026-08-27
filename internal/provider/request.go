package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const RequestSchemaVersion = 2

const (
	EndpointOpenAIChatCompletions = "openai.chat.completions"
	PromptBuilderVersion          = "evalwitness.verifier.prompt.v1"
	ResponseFormatText            = "text"
)

type Fingerprint string

type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type RequestLineage struct {
	CriterionID     string `json:"criterion_id,omitempty"`
	SamplingSlot    string `json:"sampling_slot"`
	Entrypoint      string `json:"entrypoint,omitempty"`
	AuditCaseID     string `json:"audit_case_id,omitempty"`
	SourceTraceHash string `json:"source_trace_hash,omitempty"`
	TraceMapHash    string `json:"trace_map_hash,omitempty"`
	MutationID      string `json:"mutation_id,omitempty"`
	StudyCellID     string `json:"study_cell_id,omitempty"`
	PolicyHash      string `json:"policy_hash,omitempty"`
	LegacySource    string `json:"legacy_source,omitempty"`
}

type EvidenceBinding struct {
	InputSlot            string `json:"input_slot"`
	SourceDigest         string `json:"source_digest"`
	CanonicalDigest      string `json:"canonical_digest"`
	IngestionDigest      string `json:"ingestion_digest"`
	TraceEnvelopeDigest  string `json:"trace_envelope_digest"`
	MappingReportDigest  string `json:"mapping_report_digest"`
	MappingPolicyVersion string `json:"mapping_policy_version"`
}

func (l RequestLineage) ValidateResearch() error {
	required := []struct {
		name  string
		value string
	}{
		{"criterion_id", l.CriterionID},
		{"sampling_slot", l.SamplingSlot},
		{"entrypoint", l.Entrypoint},
		{"audit_case_id", l.AuditCaseID},
		{"source_trace_hash", l.SourceTraceHash},
		{"trace_map_hash", l.TraceMapHash},
		{"mutation_id", l.MutationID},
		{"study_cell_id", l.StudyCellID},
		{"policy_hash", l.PolicyHash},
	}
	missing := make([]string, 0, len(required))
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("research request lineage is incomplete: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

type RequestEnvelope struct {
	SchemaVersion        int               `json:"schema_version"`
	ProviderID           string            `json:"provider_id"`
	BaseURLOrigin        string            `json:"base_url_origin"`
	EndpointPath         string            `json:"endpoint_path"`
	EndpointKind         string            `json:"endpoint_kind"`
	RequestedModel       string            `json:"requested_model"`
	ThinkingMode         string            `json:"thinking_mode"`
	Messages             []Message         `json:"messages"`
	Temperature          float64           `json:"temperature"`
	Seed                 *int              `json:"seed"`
	MaxOutputTokens      int               `json:"max_output_tokens"`
	Logprobs             bool              `json:"logprobs"`
	TopLogprobs          int               `json:"top_logprobs"`
	Stop                 []string          `json:"stop"`
	ScoreTags            []string          `json:"score_tags"`
	ResponseFormat       string            `json:"response_format"`
	Stream               bool              `json:"stream"`
	PromptBuilderVersion string            `json:"prompt_builder_version"`
	LogitBias            map[string]int    `json:"logit_bias"`
	EvidenceBindings     []EvidenceBinding `json:"evidence_bindings"`
	Lineage              RequestLineage    `json:"lineage"`

	BeforeAttempt func() error              `json:"-"`
	AfterAttempt  func(AttemptResult) error `json:"-"`
}

type RequestOptions struct {
	ProviderID       string
	BaseURL          string
	RequestedModel   string
	ThinkingMode     string
	Messages         []Message
	Temperature      float64
	Seed             *int
	MaxOutputTokens  int
	Logprobs         bool
	TopLogprobs      int
	Stop             []string
	ScoreTags        []string
	ResponseFormat   string
	Stream           bool
	LogitBias        map[string]int
	EvidenceBindings []EvidenceBinding
	Lineage          RequestLineage
	BeforeAttempt    func() error
	AfterAttempt     func(AttemptResult) error
}

func NewRequestEnvelope(options RequestOptions) (RequestEnvelope, error) {
	origin, endpointPath, err := canonicalEndpoint(options.BaseURL)
	if err != nil {
		return RequestEnvelope{}, err
	}
	thinkingMode := options.ThinkingMode
	if thinkingMode == "" {
		thinkingMode = "default"
	}
	responseFormat := options.ResponseFormat
	if responseFormat == "" {
		responseFormat = ResponseFormatText
	}
	envelope := RequestEnvelope{
		SchemaVersion:        RequestSchemaVersion,
		ProviderID:           options.ProviderID,
		BaseURLOrigin:        origin,
		EndpointPath:         endpointPath,
		EndpointKind:         EndpointOpenAIChatCompletions,
		RequestedModel:       options.RequestedModel,
		ThinkingMode:         thinkingMode,
		Messages:             append([]Message(nil), options.Messages...),
		Temperature:          normalizeZero(options.Temperature),
		Seed:                 cloneInt(options.Seed),
		MaxOutputTokens:      options.MaxOutputTokens,
		Logprobs:             options.Logprobs,
		TopLogprobs:          options.TopLogprobs,
		Stop:                 normalizeStrings(options.Stop),
		ScoreTags:            normalizeStrings(options.ScoreTags),
		ResponseFormat:       responseFormat,
		Stream:               options.Stream,
		PromptBuilderVersion: PromptBuilderVersion,
		LogitBias:            cloneIntMap(options.LogitBias),
		EvidenceBindings:     append([]EvidenceBinding(nil), options.EvidenceBindings...),
		Lineage:              options.Lineage,
		BeforeAttempt:        options.BeforeAttempt,
		AfterAttempt:         options.AfterAttempt,
	}
	if envelope.Lineage.SamplingSlot == "" {
		envelope.Lineage.SamplingSlot = "default"
	}
	if err := envelope.Validate(); err != nil {
		return RequestEnvelope{}, err
	}
	return envelope, nil
}

func (r RequestEnvelope) Validate() error {
	switch {
	case r.SchemaVersion != RequestSchemaVersion:
		return fmt.Errorf("request schema version %d is unsupported", r.SchemaVersion)
	case strings.TrimSpace(r.ProviderID) == "":
		return errors.New("request provider_id is required")
	case strings.TrimSpace(r.BaseURLOrigin) == "":
		return errors.New("request base_url_origin is required")
	case !strings.HasPrefix(r.EndpointPath, "/"):
		return errors.New("request endpoint_path must be absolute within its origin")
	case r.EndpointKind != EndpointOpenAIChatCompletions:
		return fmt.Errorf("request endpoint_kind %q is unsupported", r.EndpointKind)
	case strings.TrimSpace(r.RequestedModel) == "":
		return errors.New("request requested_model is required")
	case strings.TrimSpace(r.ThinkingMode) == "":
		return errors.New("request thinking_mode is required")
	case len(r.Messages) == 0:
		return errors.New("request messages are required")
	case math.IsNaN(r.Temperature) || math.IsInf(r.Temperature, 0):
		return errors.New("request temperature must be finite")
	case r.Temperature < 0:
		return errors.New("request temperature must be non-negative")
	case r.MaxOutputTokens <= 0:
		return errors.New("request max_output_tokens must be positive")
	case !r.Logprobs && r.TopLogprobs != 0:
		return errors.New("request top_logprobs requires logprobs")
	case r.Logprobs && r.TopLogprobs <= 0:
		return errors.New("request logprobs requires positive top_logprobs")
	case r.ResponseFormat == "":
		return errors.New("request response_format is required")
	case r.PromptBuilderVersion == "":
		return errors.New("request prompt_builder_version is required")
	case r.Lineage.SamplingSlot == "":
		return errors.New("request lineage sampling_slot is required")
	}
	if err := validateCanonicalRoute(r.BaseURLOrigin, r.EndpointPath); err != nil {
		return err
	}
	if err := validateRequestUTF8(r); err != nil {
		return err
	}
	if err := validateEvidenceBindings(r.EvidenceBindings); err != nil {
		return err
	}
	for index, message := range r.Messages {
		if message.Role == "" {
			return fmt.Errorf("request messages[%d].role is required", index)
		}
	}
	return nil
}

func (r RequestEnvelope) CanonicalBytes() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	b := make([]byte, 0, 1024)
	b = append(b, '{')
	b = appendStringField(b, "schema_version", strconv.Itoa(r.SchemaVersion), false, true)
	b = appendStringField(b, "provider_id", r.ProviderID, true, false)
	b = appendStringField(b, "base_url_origin", r.BaseURLOrigin, true, false)
	b = appendStringField(b, "endpoint_path", r.EndpointPath, true, false)
	b = appendStringField(b, "endpoint_kind", r.EndpointKind, true, false)
	b = appendStringField(b, "requested_model", r.RequestedModel, true, false)
	b = appendStringField(b, "thinking_mode", r.ThinkingMode, true, false)
	b = append(b, `,"messages":`...)
	b = appendMessages(b, r.Messages)
	b = appendStringField(b, "temperature_f64", fmt.Sprintf("%016x", math.Float64bits(normalizeZero(r.Temperature))), true, false)
	b = append(b, `,"seed":`...)
	if r.Seed == nil {
		b = append(b, "null"...)
	} else {
		b = strconv.AppendInt(b, int64(*r.Seed), 10)
	}
	b = appendStringField(b, "max_output_tokens", strconv.Itoa(r.MaxOutputTokens), true, true)
	b = appendBoolField(b, "logprobs", r.Logprobs)
	b = appendStringField(b, "top_logprobs", strconv.Itoa(r.TopLogprobs), true, true)
	b = append(b, `,"stop":`...)
	b = appendStringArray(b, r.Stop)
	b = append(b, `,"score_tags":`...)
	b = appendStringArray(b, r.ScoreTags)
	b = appendStringField(b, "response_format", r.ResponseFormat, true, false)
	b = appendBoolField(b, "stream", r.Stream)
	b = appendStringField(b, "prompt_builder_version", r.PromptBuilderVersion, true, false)
	b = append(b, `,"logit_bias":`...)
	b = appendSortedIntMap(b, r.LogitBias)
	b = append(b, `,"evidence_bindings":`...)
	b = appendEvidenceBindings(b, r.EvidenceBindings)
	b = append(b, '}')
	return b, nil
}

func (r RequestEnvelope) Fingerprint() (Fingerprint, error) {
	canonical, err := r.CanonicalBytes()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return Fingerprint(hex.EncodeToString(digest[:])), nil
}

func (r RequestEnvelope) Prompt() (string, bool) {
	if len(r.Messages) != 1 || r.Messages[0].Role != "user" || r.Messages[0].Name != "" || r.Messages[0].ToolCallID != "" {
		return "", false
	}
	return r.Messages[0].Content, true
}

func (r RequestEnvelope) RouteID() string {
	h := sha256.New()
	writeLengthPrefixed(h, r.ProviderID)
	writeLengthPrefixed(h, r.BaseURLOrigin)
	writeLengthPrefixed(h, r.EndpointPath)
	writeLengthPrefixed(h, r.RequestedModel)
	return "route-" + hex.EncodeToString(h.Sum(nil))
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeLengthPrefixed(writer byteWriter, value string) {
	_, _ = writer.Write([]byte(strconv.Itoa(len(value))))
	_, _ = writer.Write([]byte{':'})
	_, _ = writer.Write([]byte(value))
}

func canonicalEndpoint(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("request base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", fmt.Errorf("request base URL scheme %q is unsupported", u.Scheme)
	}
	if u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", errors.New("request base URL must have a host and no userinfo, query, or fragment")
	}
	scheme := strings.ToLower(u.Scheme)
	hostname := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	basePath := path.Clean("/" + strings.TrimPrefix(u.EscapedPath(), "/"))
	if basePath == "/." {
		basePath = "/"
	}
	endpointPath := path.Join(basePath, "chat/completions")
	return scheme + "://" + host, endpointPath, nil
}

func appendStringField(dst []byte, key, value string, comma, raw bool) []byte {
	if comma {
		dst = append(dst, ',')
	}
	dst = appendJSONString(dst, key)
	dst = append(dst, ':')
	if raw {
		return append(dst, value...)
	}
	return appendJSONString(dst, value)
}

func appendBoolField(dst []byte, key string, value bool) []byte {
	dst = append(dst, ',')
	dst = appendJSONString(dst, key)
	dst = append(dst, ':')
	return strconv.AppendBool(dst, value)
}

func appendMessages(dst []byte, values []Message) []byte {
	dst = append(dst, '[')
	for index, value := range values {
		if index > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, '{')
		dst = appendStringField(dst, "role", value.Role, false, false)
		dst = appendStringField(dst, "content", value.Content, true, false)
		dst = appendStringField(dst, "name", value.Name, true, false)
		dst = appendStringField(dst, "tool_call_id", value.ToolCallID, true, false)
		dst = append(dst, '}')
	}
	return append(dst, ']')
}

func appendStringArray(dst []byte, values []string) []byte {
	dst = append(dst, '[')
	for index, value := range values {
		if index > 0 {
			dst = append(dst, ',')
		}
		dst = appendJSONString(dst, value)
	}
	return append(dst, ']')
}

func appendSortedIntMap(dst []byte, values map[string]int) []byte {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	dst = append(dst, '{')
	for index, key := range keys {
		if index > 0 {
			dst = append(dst, ',')
		}
		dst = appendJSONString(dst, key)
		dst = append(dst, ':')
		dst = strconv.AppendInt(dst, int64(values[key]), 10)
	}
	return append(dst, '}')
}

func appendEvidenceBindings(dst []byte, values []EvidenceBinding) []byte {
	dst = append(dst, '[')
	for index, value := range values {
		if index > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, '{')
		dst = appendStringField(dst, "input_slot", value.InputSlot, false, false)
		dst = appendStringField(dst, "source_digest", value.SourceDigest, true, false)
		dst = appendStringField(dst, "canonical_digest", value.CanonicalDigest, true, false)
		dst = appendStringField(dst, "ingestion_digest", value.IngestionDigest, true, false)
		dst = appendStringField(dst, "trace_envelope_digest", value.TraceEnvelopeDigest, true, false)
		dst = appendStringField(dst, "mapping_report_digest", value.MappingReportDigest, true, false)
		dst = appendStringField(dst, "mapping_policy_version", value.MappingPolicyVersion, true, false)
		dst = append(dst, '}')
	}
	return append(dst, ']')
}

func appendJSONString(dst []byte, value string) []byte {
	dst = append(dst, '"')
	for _, current := range []byte(value) {
		switch current {
		case '"', '\\':
			dst = append(dst, '\\', current)
		case '\b':
			dst = append(dst, `\b`...)
		case '\f':
			dst = append(dst, `\f`...)
		case '\n':
			dst = append(dst, `\n`...)
		case '\r':
			dst = append(dst, `\r`...)
		case '\t':
			dst = append(dst, `\t`...)
		default:
			if current < 0x20 {
				dst = append(dst, `\u00`...)
				dst = append(dst, "0123456789abcdef"[current>>4], "0123456789abcdef"[current&0x0f])
				continue
			}
			dst = append(dst, current)
		}
	}
	return append(dst, '"')
}

func validateCanonicalRoute(origin, endpointPath string) error {
	canonicalOrigin, _, err := canonicalEndpoint(origin)
	if err != nil {
		return fmt.Errorf("request base_url_origin: %w", err)
	}
	if canonicalOrigin != origin {
		return fmt.Errorf("request base_url_origin %q is not canonical", origin)
	}
	if path.Clean(endpointPath) != endpointPath || strings.Contains(endpointPath, "\\") {
		return fmt.Errorf("request endpoint_path %q is not canonical", endpointPath)
	}
	return nil
}

func validateRequestUTF8(request RequestEnvelope) error {
	values := []string{
		request.ProviderID, request.BaseURLOrigin, request.EndpointPath,
		request.EndpointKind, request.RequestedModel, request.ThinkingMode,
		request.ResponseFormat, request.PromptBuilderVersion,
		request.Lineage.CriterionID, request.Lineage.SamplingSlot,
		request.Lineage.Entrypoint, request.Lineage.AuditCaseID,
		request.Lineage.SourceTraceHash, request.Lineage.TraceMapHash,
		request.Lineage.MutationID, request.Lineage.StudyCellID,
		request.Lineage.PolicyHash, request.Lineage.LegacySource,
	}
	for _, binding := range request.EvidenceBindings {
		values = append(values, binding.InputSlot, binding.SourceDigest, binding.CanonicalDigest,
			binding.IngestionDigest, binding.TraceEnvelopeDigest, binding.MappingReportDigest, binding.MappingPolicyVersion)
	}
	for _, message := range request.Messages {
		values = append(values, message.Role, message.Content, message.Name, message.ToolCallID)
	}
	values = append(values, request.Stop...)
	values = append(values, request.ScoreTags...)
	for key := range request.LogitBias {
		values = append(values, key)
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return errors.New("request contains invalid UTF-8")
		}
	}
	return nil
}

func validateEvidenceBindings(bindings []EvidenceBinding) error {
	seen := make(map[string]struct{}, len(bindings))
	for index, binding := range bindings {
		if strings.TrimSpace(binding.InputSlot) == "" {
			return fmt.Errorf("request evidence_bindings[%d].input_slot is required", index)
		}
		if _, exists := seen[binding.InputSlot]; exists {
			return fmt.Errorf("request evidence binding repeats input_slot %q", binding.InputSlot)
		}
		seen[binding.InputSlot] = struct{}{}
		for name, value := range map[string]string{
			"source_digest": binding.SourceDigest, "canonical_digest": binding.CanonicalDigest,
			"ingestion_digest": binding.IngestionDigest, "trace_envelope_digest": binding.TraceEnvelopeDigest,
			"mapping_report_digest": binding.MappingReportDigest,
		} {
			decoded, err := hex.DecodeString(value)
			if err != nil || len(decoded) != sha256.Size {
				return fmt.Errorf("request evidence_bindings[%d].%s must be a sha256 digest", index, name)
			}
		}
		if strings.TrimSpace(binding.MappingPolicyVersion) == "" {
			return fmt.Errorf("request evidence_bindings[%d].mapping_policy_version is required", index)
		}
	}
	return nil
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneIntMap(values map[string]int) map[string]int {
	if len(values) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func normalizeZero(value float64) float64 {
	if value == 0 {
		return 0
	}
	return value
}
