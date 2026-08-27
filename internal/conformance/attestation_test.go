package conformance

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func qualificationFixture(t *testing.T) (provider.RequestEnvelope, provider.ResponseRecord, map[string]verifier.ScoreEvidence, QualificationContext) {
	t.Helper()
	request, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID:      "deterministic-conformance",
		BaseURL:         "https://conformance.invalid/v1",
		RequestedModel:  "alias-model",
		ThinkingMode:    "disabled",
		Messages:        []provider.Message{{Role: "user", Content: "bounded real score task"}},
		Temperature:     0.7,
		MaxOutputTokens: 64,
		Logprobs:        true,
		TopLogprobs:     verifier.MinimumVerifierTopK,
		ScoreTags:       []string{"<score_A>"},
		Stream:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	alternatives := make([]provider.TokenAlternative, 0, verifier.MinimumVerifierTopK)
	alternatives = append(alternatives,
		provider.TokenAlternative{Token: "A", Logprob: probabilityLog(0.60)},
		provider.TokenAlternative{Token: "B", Logprob: probabilityLog(0.20)},
	)
	for index := len(alternatives); index < verifier.MinimumVerifierTopK; index++ {
		alternatives = append(alternatives, provider.TokenAlternative{Token: "#" + string(rune('a'+index)), Logprob: probabilityLog(0.001)})
	}
	response, err := provider.FinalizeResponse(request, provider.ResponseRecord{
		ServedModel:         "served-alias-model",
		CheckpointAssertion: "checkpoint-operator-claim",
		ProviderRequestID:   "request-1",
		FinishReason:        "stop",
		Usage:               provider.TokenUsage{Input: 21, Output: 4},
		NormalizedBody:      []byte(`{"fixture":"private raw body"}`),
		RawText:             "<score_A>A</score_A>",
		HasLogprobs:         true,
		ObservedTopLogprobs: verifier.MinimumVerifierTopK,
		OrderedTokenEvidence: []provider.TokenEvidence{
			{Position: 0, Token: "<score_A>", Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			{Position: 1, Token: "A", Logprob: probabilityLog(0.60), TopAlternatives: alternatives},
			{Position: 2, Token: "</score_A>", Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence := verifier.ExtractAllScoreEvidence(request, response, verifier.ExtractionModeVerifier)
	if err := verifier.ValidateStrictEvidence(evidence); err != nil {
		t.Fatalf("fixture evidence is not strict: %v", err)
	}
	observedAt := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	context := QualificationContext{
		ObservedAt:                observedAt,
		ExpiresAt:                 observedAt.Add(24 * time.Hour),
		Latency:                   250 * time.Millisecond,
		HTTPAttempts:              1,
		StreamingObserved:         true,
		UsageObserved:             true,
		CheckpointAssertionSource: "operator configuration",
		Build: BuildIdentity{
			Commit:       "0123456789abcdef",
			BinarySHA256: strings.Repeat("a", 64),
		},
	}
	return request, response, evidence, context
}

func probabilityLog(probability float64) string {
	return strconv.FormatFloat(math.Log(probability), 'g', -1, 64)
}

func TestRouteStateTransitionsAreExplicit(t *testing.T) {
	allowed := [][2]RouteState{
		{StateUnconfigured, StateConfigured},
		{StateConfigured, StateProbeCompatible},
		{StateConfigured, StateBoundedQualified},
		{StateProbeCompatible, StateBoundedQualified},
		{StateBoundedQualified, StateStudyQualified},
		{StateStudyQualified, StateExpired},
		{StateFailed, StateConfigured},
	}
	for _, transition := range allowed {
		if err := ValidateTransition(transition[0], transition[1]); err != nil {
			t.Fatalf("allowed transition %s -> %s failed: %v", transition[0], transition[1], err)
		}
	}
	for _, transition := range [][2]RouteState{{StateUnconfigured, StateStudyQualified}, {StateProbeCompatible, StateStudyQualified}, {StateExpired, StateStudyQualified}, {StateFailed, StateBoundedQualified}} {
		if err := ValidateTransition(transition[0], transition[1]); err == nil {
			t.Fatalf("forbidden transition %s -> %s passed", transition[0], transition[1])
		}
	}
}

func TestBoundedQualificationRequiresTheRealScoreContract(t *testing.T) {
	request, response, evidence, context := qualificationFixture(t)
	attestation, err := EvaluateBounded(request, response, evidence, context)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.State != StateBoundedQualified {
		t.Fatalf("state = %s", attestation.State)
	}
	if err := attestation.ValidateIntegrity(); err != nil {
		t.Fatal(err)
	}
	if attestation.Identity.ServedModel != "served-alias-model" || attestation.Identity.CheckpointAssertionSource != "operator configuration" {
		t.Fatalf("identity boundaries collapsed: %+v", attestation.Identity)
	}

	zeroTemperature := request
	zeroTemperature.Temperature = 0
	failed, err := EvaluateBounded(zeroTemperature, response, evidence, context)
	if err == nil || failed.State != StateFailed {
		t.Fatalf("temperature-zero probe qualified: state=%s err=%v", failed.State, err)
	}

	noTags := request
	noTags.ScoreTags = nil
	failed, err = EvaluateBounded(noTags, response, evidence, context)
	if err == nil || failed.State != StateFailed {
		t.Fatalf("no-tag probe qualified: state=%s err=%v", failed.State, err)
	}
}

func TestProbeQualificationBindsNonLogprobRouteContractAndFreshness(t *testing.T) {
	request, response, _, context := qualificationFixture(t)
	request.Logprobs = false
	request.TopLogprobs = 0
	request.ScoreTags = nil
	response.RequestFingerprint = ""
	response.ResponseBodyDigest = ""
	response.ParsedPayloadDigest = ""
	response.EvidenceDigest = ""
	response.HasLogprobs = false
	response.ObservedTopLogprobs = 0
	response.OrderedTokenEvidence = nil
	response, err := provider.FinalizeResponse(request, response)
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := EvaluateProbe(request, response, context)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.State != StateProbeCompatible || attestation.Observation.HasLogprobs {
		t.Fatalf("probe attestation = %+v", attestation)
	}
	if state, reason := attestation.EffectiveState(context.ObservedAt, attestation.Identity.RouteConfigDigest, attestation.Contract.ContractDigest, ""); state != StateProbeCompatible || reason != "" {
		t.Fatalf("probe state=%s reason=%s", state, reason)
	}
	if state, reason := attestation.EffectiveState(context.ExpiresAt, attestation.Identity.RouteConfigDigest, attestation.Contract.ContractDigest, ""); state != StateExpired || reason != ExpirationFreshness {
		t.Fatalf("expired probe state=%s reason=%s", state, reason)
	}
	if _, err := EvaluateBounded(request, response, nil, context); err == nil {
		t.Fatal("non-logprob probe route passed bounded qualification")
	}
}

func TestAttestationExpiresOnFreshnessRouteOrContractDrift(t *testing.T) {
	request, response, evidence, context := qualificationFixture(t)
	attestation, err := EvaluateBounded(request, response, evidence, context)
	if err != nil {
		t.Fatal(err)
	}
	if state, reason := attestation.EffectiveState(context.ObservedAt, attestation.Identity.RouteConfigDigest, attestation.Contract.ContractDigest, ""); state != StateBoundedQualified || reason != "" {
		t.Fatalf("fresh state=%s reason=%s", state, reason)
	}
	checks := []struct {
		now            time.Time
		routeDigest    string
		contractDigest string
		want           ExpirationReason
	}{
		{context.ExpiresAt, attestation.Identity.RouteConfigDigest, attestation.Contract.ContractDigest, ExpirationFreshness},
		{context.ObservedAt, "foreign-route", attestation.Contract.ContractDigest, ExpirationRouteConfig},
		{context.ObservedAt, attestation.Identity.RouteConfigDigest, "foreign-contract", ExpirationRequestContract},
	}
	for _, check := range checks {
		if state, reason := attestation.EffectiveState(check.now, check.routeDigest, check.contractDigest, ""); state != StateExpired || reason != check.want {
			t.Fatalf("state=%s reason=%s want expired/%s", state, reason, check.want)
		}
	}
}

func TestAttestationStoreKeysExactRouteAndRequestContract(t *testing.T) {
	request, response, evidence, context := qualificationFixture(t)
	attestation, err := EvaluateBounded(request, response, evidence, context)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(attestation); err != nil {
		t.Fatal(err)
	}
	loaded, state, reason, err := store.Load(RouteConfigDigest(request), CapabilityContractDigest(request), context.ObservedAt, "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AttestationDigest != attestation.AttestationDigest || state != StateBoundedQualified || reason != "" {
		t.Fatalf("loaded digest=%s state=%s reason=%s", loaded.AttestationDigest, state, reason)
	}

	changed := request
	changed.ThinkingMode = "enabled"
	if _, _, _, err := store.Load(RouteConfigDigest(changed), CapabilityContractDigest(request), context.ObservedAt, ""); !errors.Is(err, ErrAttestationNotFound) {
		t.Fatalf("foreign route lookup error = %v", err)
	}
	changed = request
	changed.Temperature = 0.8
	if _, _, _, err := store.Load(RouteConfigDigest(request), CapabilityContractDigest(changed), context.ObservedAt, ""); !errors.Is(err, ErrAttestationNotFound) {
		t.Fatalf("foreign contract lookup error = %v", err)
	}
}

func TestPublicDerivativeIsRedactedSignedAndChallengeBound(t *testing.T) {
	request, response, evidence, context := qualificationFixture(t)
	attestation, err := EvaluateBounded(request, response, evidence, context)
	if err != nil {
		t.Fatal(err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	challenge := FreshnessChallenge{
		Purpose:   "registry-submission",
		Nonce:     "server-issued-nonce",
		IssuedAt:  context.ObservedAt,
		ExpiresAt: context.ObservedAt.Add(5 * time.Minute),
	}
	public, err := DerivePublic(attestation, PublicOptions{Challenge: challenge}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	expectation := PublicExpectation{RouteID: request.RouteID(), SourceDigest: attestation.AttestationDigest, MinimumState: StateBoundedQualified, ChallengePurpose: "registry-submission"}
	if err := public.Verify(context.ObservedAt.Add(time.Minute), expectation); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"https://conformance.invalid", "private raw body", "bounded real score task"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("public derivative leaked %q", secret)
		}
	}

	tampered := public
	tampered.State = StateStudyQualified
	if err := tampered.Verify(context.ObservedAt.Add(time.Minute), expectation); err == nil {
		t.Fatal("evidence ceiling tampering passed")
	}
	if err := public.Verify(challenge.ExpiresAt, expectation); err == nil {
		t.Fatal("replayed stale challenge passed")
	}
	wrongRoute := expectation
	wrongRoute.RouteID = "route-foreign"
	if err := public.Verify(context.ObservedAt.Add(time.Minute), wrongRoute); err == nil {
		t.Fatal("route mismatch passed")
	}
}
