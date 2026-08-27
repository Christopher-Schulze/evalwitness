package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const (
	AttestationSchemaVersion = "evalwitness.capability-attestation.v1"
	ScoreAlphabet            = "ABCDEFGHIJKLMNOPQRST"
)

type ExpirationReason string

const (
	ExpirationFreshness       ExpirationReason = "freshness_window_elapsed"
	ExpirationRequestContract ExpirationReason = "request_contract_changed"
	ExpirationRouteConfig     ExpirationReason = "route_config_changed"
	ExpirationStudyManifest   ExpirationReason = "study_manifest_changed"
)

type RouteIdentity struct {
	ProviderID                string `json:"provider_id"`
	RouteID                   string `json:"route_id"`
	RouteConfigDigest         string `json:"route_config_digest"`
	BaseOrigin                string `json:"base_origin"`
	EndpointPath              string `json:"endpoint_path"`
	RequestedModel            string `json:"requested_model"`
	ServedModel               string `json:"served_model,omitempty"`
	CheckpointAssertion       string `json:"checkpoint_assertion,omitempty"`
	CheckpointAssertionSource string `json:"checkpoint_assertion_source,omitempty"`
}

type RequestContract struct {
	ContractDigest        string               `json:"contract_digest"`
	RequestSchemaVersion  int                  `json:"request_schema_version"`
	PromptBuilderVersion  string               `json:"prompt_builder_version"`
	ParserContractVersion string               `json:"parser_contract_version"`
	ScorePolicyVersion    string               `json:"score_policy_version"`
	ScoreAlphabet         string               `json:"score_alphabet"`
	ScoreTags             []string             `json:"score_tags"`
	RequestedTopK         int                  `json:"requested_top_k"`
	Temperature           float64              `json:"temperature"`
	ThinkingMode          string               `json:"thinking_mode,omitempty"`
	StreamingRequested    bool                 `json:"streaming_requested"`
	RequestFingerprint    provider.Fingerprint `json:"request_fingerprint"`
}

type Observation struct {
	Evidence             map[string]verifier.ScoreEvidence `json:"score_evidence"`
	EvidenceStrength     verifier.EvidenceStrength         `json:"evidence_strength"`
	HasLogprobs          bool                              `json:"has_logprobs"`
	DegenerateLogprobs   bool                              `json:"degenerate_logprobs"`
	ObservedTopK         int                               `json:"observed_top_k"`
	FinishReason         string                            `json:"finish_reason,omitempty"`
	StreamingObserved    bool                              `json:"streaming_observed"`
	UsageObserved        bool                              `json:"usage_observed"`
	Usage                provider.TokenUsage               `json:"usage"`
	ProviderRequestID    string                            `json:"provider_request_id,omitempty"`
	LatencyMilliseconds  int64                             `json:"latency_milliseconds"`
	HTTPAttempts         int                               `json:"http_attempts"`
	ResponseEvidenceHash string                            `json:"response_evidence_digest"`
}

type BuildIdentity struct {
	Commit       string `json:"commit"`
	Dirty        bool   `json:"dirty"`
	BinarySHA256 string `json:"binary_sha256"`
	GoVersion    string `json:"go_version"`
	Target       string `json:"target"`
}

type QualificationContext struct {
	ObservedAt                time.Time
	ExpiresAt                 time.Time
	Latency                   time.Duration
	HTTPAttempts              int
	StreamingObserved         bool
	UsageObserved             bool
	CheckpointAssertionSource string
	Build                     BuildIdentity
}

type Attestation struct {
	SchemaVersion       string           `json:"schema_version"`
	AttestationID       string           `json:"attestation_id"`
	State               RouteState       `json:"state"`
	Identity            RouteIdentity    `json:"identity"`
	Contract            RequestContract  `json:"contract"`
	Observation         Observation      `json:"observation"`
	Build               BuildIdentity    `json:"build"`
	ObservedAt          time.Time        `json:"observed_at"`
	ExpiresAt           time.Time        `json:"expires_at"`
	ExpirationReason    ExpirationReason `json:"expiration_reason,omitempty"`
	StudyManifestDigest string           `json:"study_manifest_digest,omitempty"`
	Limitations         []string         `json:"limitations"`
	AttestationDigest   string           `json:"attestation_digest"`
}

func EvaluateBounded(request provider.RequestEnvelope, response provider.ResponseRecord, evidence map[string]verifier.ScoreEvidence, context QualificationContext) (Attestation, error) {
	attestation, observedAt, err := buildAttestation(request, response, evidence, context)
	if err != nil {
		return Attestation{}, err
	}
	qualificationErr := validateBoundedQualification(request, response, evidence, context, observedAt)
	if qualificationErr == nil {
		attestation.State = StateBoundedQualified
	}
	if err := attestation.seal(); err != nil {
		return Attestation{}, err
	}
	return attestation, qualificationErr
}

func EvaluateProbe(request provider.RequestEnvelope, response provider.ResponseRecord, context QualificationContext) (Attestation, error) {
	attestation, observedAt, err := buildAttestation(request, response, nil, context)
	if err != nil {
		return Attestation{}, err
	}
	qualificationErr := validateProbeCompatibility(request, response, context, observedAt)
	if qualificationErr == nil {
		attestation.State = StateProbeCompatible
	}
	if err := attestation.seal(); err != nil {
		return Attestation{}, err
	}
	return attestation, qualificationErr
}

func buildAttestation(
	request provider.RequestEnvelope,
	response provider.ResponseRecord,
	evidence map[string]verifier.ScoreEvidence,
	context QualificationContext,
) (Attestation, time.Time, error) {
	fingerprint, err := request.Fingerprint()
	if err != nil {
		return Attestation{}, time.Time{}, err
	}
	observedAt := context.ObservedAt.UTC()
	if observedAt.IsZero() && response.ReceivedAt > 0 {
		observedAt = time.UnixMilli(response.ReceivedAt).UTC()
	}
	attestation := Attestation{
		SchemaVersion: AttestationSchemaVersion,
		State:         StateFailed,
		Identity: RouteIdentity{
			ProviderID:                request.ProviderID,
			RouteID:                   request.RouteID(),
			RouteConfigDigest:         RouteConfigDigest(request),
			BaseOrigin:                request.BaseURLOrigin,
			EndpointPath:              request.EndpointPath,
			RequestedModel:            request.RequestedModel,
			ServedModel:               response.ServedModel,
			CheckpointAssertion:       response.CheckpointAssertion,
			CheckpointAssertionSource: strings.TrimSpace(context.CheckpointAssertionSource),
		},
		Contract: RequestContract{
			ContractDigest:        CapabilityContractDigest(request),
			RequestSchemaVersion:  request.SchemaVersion,
			PromptBuilderVersion:  request.PromptBuilderVersion,
			ParserContractVersion: provider.ParserContractVersion,
			ScorePolicyVersion:    verifier.StrictPolicyVersion,
			ScoreAlphabet:         ScoreAlphabet,
			ScoreTags:             append([]string(nil), request.ScoreTags...),
			RequestedTopK:         request.TopLogprobs,
			Temperature:           request.Temperature,
			ThinkingMode:          request.ThinkingMode,
			StreamingRequested:    request.Stream,
			RequestFingerprint:    fingerprint,
		},
		Observation: Observation{
			Evidence:             cloneEvidence(evidence),
			EvidenceStrength:     verifier.SummarizeEvidenceStrength(orderedEvidence(evidence)),
			HasLogprobs:          response.HasLogprobs,
			DegenerateLogprobs:   response.DegenerateLogprobs,
			ObservedTopK:         response.ObservedTopLogprobs,
			FinishReason:         response.FinishReason,
			StreamingObserved:    context.StreamingObserved,
			UsageObserved:        context.UsageObserved,
			Usage:                response.Usage,
			ProviderRequestID:    response.ProviderRequestID,
			LatencyMilliseconds:  context.Latency.Milliseconds(),
			HTTPAttempts:         context.HTTPAttempts,
			ResponseEvidenceHash: response.EvidenceDigest,
		},
		Build:       normalizedBuild(context.Build),
		ObservedAt:  observedAt,
		ExpiresAt:   context.ExpiresAt.UTC(),
		Limitations: limitationsFor(response, context),
	}
	return attestation, observedAt, nil
}

func (a Attestation) ValidateIntegrity() error {
	if a.SchemaVersion != AttestationSchemaVersion || !a.State.Valid() {
		return errors.New("attestation schema or state is invalid")
	}
	expected, err := a.digest()
	if err != nil {
		return err
	}
	if a.AttestationDigest != expected || a.AttestationID != "att-"+expected {
		return errors.New("attestation digest is invalid")
	}
	return nil
}

func (a Attestation) EffectiveState(now time.Time, routeConfigDigest, contractDigest, studyManifestDigest string) (RouteState, ExpirationReason) {
	if a.State != StateProbeCompatible && a.State != StateBoundedQualified && a.State != StateStudyQualified {
		return a.State, ""
	}
	if routeConfigDigest != "" && routeConfigDigest != a.Identity.RouteConfigDigest {
		return StateExpired, ExpirationRouteConfig
	}
	if contractDigest != "" && contractDigest != a.Contract.ContractDigest {
		return StateExpired, ExpirationRequestContract
	}
	if a.State == StateStudyQualified && studyManifestDigest != "" && studyManifestDigest != a.StudyManifestDigest {
		return StateExpired, ExpirationStudyManifest
	}
	if !a.ExpiresAt.IsZero() && !now.UTC().Before(a.ExpiresAt) {
		return StateExpired, ExpirationFreshness
	}
	return a.State, ""
}

func validateBoundedQualification(request provider.RequestEnvelope, response provider.ResponseRecord, evidence map[string]verifier.ScoreEvidence, context QualificationContext, observedAt time.Time) error {
	if err := validateProbeCompatibility(request, response, context, observedAt); err != nil {
		return err
	}
	switch {
	case request.Temperature <= 0:
		return errors.New("bounded qualification requires temperature greater than zero")
	case !request.Logprobs:
		return errors.New("bounded qualification requires logprobs")
	case request.TopLogprobs < verifier.MinimumVerifierTopK:
		return fmt.Errorf("bounded qualification requires top-k >= %d", verifier.MinimumVerifierTopK)
	case len(request.ScoreTags) == 0:
		return errors.New("bounded qualification requires closed score tags")
	}
	if err := verifier.ValidateStrictEvidence(evidence); err != nil {
		return fmt.Errorf("strict score evidence: %w", err)
	}
	if len(evidence) != len(request.ScoreTags) {
		return errors.New("score evidence does not cover every requested tag")
	}
	for _, tag := range request.ScoreTags {
		if _, ok := evidence[tag]; !ok {
			return fmt.Errorf("score evidence is missing tag %s", tag)
		}
	}
	return nil
}

func validateProbeCompatibility(request provider.RequestEnvelope, response provider.ResponseRecord, context QualificationContext, observedAt time.Time) error {
	switch {
	case request.SchemaVersion != provider.RequestSchemaVersion:
		return errors.New("route qualification requires the current request schema")
	case context.HTTPAttempts <= 0:
		return errors.New("route qualification requires a recorded HTTP attempt")
	case observedAt.IsZero() || context.ExpiresAt.IsZero() || !context.ExpiresAt.After(observedAt):
		return errors.New("route qualification requires a valid freshness window")
	case context.Latency < 0:
		return errors.New("route qualification latency cannot be negative")
	case response.CheckpointAssertion != "" && strings.TrimSpace(context.CheckpointAssertionSource) == "":
		return errors.New("checkpoint assertion requires an explicit source")
	}
	if err := response.ValidateExact(request); err != nil {
		return fmt.Errorf("response contract: %w", err)
	}
	return validateBuild(context.Build)
}

func validateBuild(build BuildIdentity) error {
	if strings.TrimSpace(build.Commit) == "" || strings.TrimSpace(build.BinarySHA256) == "" {
		return errors.New("route qualification requires commit and binary digests")
	}
	if len(build.BinarySHA256) != sha256.Size*2 {
		return errors.New("binary digest must be SHA-256")
	}
	if _, err := hex.DecodeString(build.BinarySHA256); err != nil {
		return errors.New("binary digest must be hexadecimal SHA-256")
	}
	return nil
}

func normalizedBuild(build BuildIdentity) BuildIdentity {
	if build.GoVersion == "" {
		build.GoVersion = runtime.Version()
	}
	if build.Target == "" {
		build.Target = runtime.GOOS + "/" + runtime.GOARCH
	}
	return build
}

func limitationsFor(response provider.ResponseRecord, context QualificationContext) []string {
	limitations := []string{"independent observation; not a provider endorsement", "observed behavior is limited to this request contract and freshness window"}
	if response.ServedModel == "" {
		limitations = append(limitations, "provider did not expose a served model identity")
	}
	if response.CheckpointAssertion != "" {
		limitations = append(limitations, "checkpoint is an assertion from "+strings.TrimSpace(context.CheckpointAssertionSource)+", not provider-issued identity")
	}
	if !context.UsageObserved {
		limitations = append(limitations, "provider usage metadata was not observed")
	}
	if !context.StreamingObserved && response.Status == provider.ResponseStatusComplete {
		limitations = append(limitations, "streaming behavior was not observed")
	}
	sort.Strings(limitations)
	return limitations
}

func RouteConfigDigest(request provider.RequestEnvelope) string {
	payload := struct {
		ProviderID     string `json:"provider_id"`
		BaseOrigin     string `json:"base_origin"`
		EndpointPath   string `json:"endpoint_path"`
		RequestedModel string `json:"requested_model"`
		ThinkingMode   string `json:"thinking_mode"`
	}{request.ProviderID, request.BaseURLOrigin, request.EndpointPath, request.RequestedModel, request.ThinkingMode}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func CapabilityContractDigest(request provider.RequestEnvelope) string {
	payload := struct {
		SchemaVersion        int            `json:"schema_version"`
		EndpointKind         string         `json:"endpoint_kind"`
		ThinkingMode         string         `json:"thinking_mode"`
		Temperature          float64        `json:"temperature"`
		MaxOutputTokens      int            `json:"max_output_tokens"`
		Logprobs             bool           `json:"logprobs"`
		TopLogprobs          int            `json:"top_logprobs"`
		Stop                 []string       `json:"stop"`
		ScoreTags            []string       `json:"score_tags"`
		ResponseFormat       string         `json:"response_format"`
		Stream               bool           `json:"stream"`
		PromptBuilderVersion string         `json:"prompt_builder_version"`
		LogitBias            map[string]int `json:"logit_bias"`
		ParserContract       string         `json:"parser_contract"`
		ScorePolicy          string         `json:"score_policy"`
		ScoreAlphabet        string         `json:"score_alphabet"`
	}{
		SchemaVersion: request.SchemaVersion, EndpointKind: request.EndpointKind, ThinkingMode: request.ThinkingMode,
		Temperature: request.Temperature, MaxOutputTokens: request.MaxOutputTokens, Logprobs: request.Logprobs,
		TopLogprobs: request.TopLogprobs, Stop: request.Stop, ScoreTags: request.ScoreTags,
		ResponseFormat: request.ResponseFormat, Stream: request.Stream, PromptBuilderVersion: request.PromptBuilderVersion,
		LogitBias: request.LogitBias, ParserContract: provider.ParserContractVersion,
		ScorePolicy: verifier.StrictPolicyVersion, ScoreAlphabet: ScoreAlphabet,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (a *Attestation) seal() error {
	digest, err := a.digest()
	if err != nil {
		return err
	}
	a.AttestationDigest = digest
	a.AttestationID = "att-" + digest
	return nil
}

func (a Attestation) digest() (string, error) {
	a.AttestationID = ""
	a.AttestationDigest = ""
	encoded, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("encode attestation: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func orderedEvidence(evidence map[string]verifier.ScoreEvidence) []verifier.ScoreEvidence {
	tags := make([]string, 0, len(evidence))
	for tag := range evidence {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	items := make([]verifier.ScoreEvidence, 0, len(tags))
	for _, tag := range tags {
		items = append(items, evidence[tag])
	}
	return items
}

func cloneEvidence(evidence map[string]verifier.ScoreEvidence) map[string]verifier.ScoreEvidence {
	if evidence == nil {
		return map[string]verifier.ScoreEvidence{}
	}
	encoded, _ := json.Marshal(evidence)
	var clone map[string]verifier.ScoreEvidence
	_ = json.Unmarshal(encoded, &clone)
	return clone
}
