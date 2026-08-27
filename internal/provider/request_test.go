package provider

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func requestForTest(t *testing.T, value Provider, prompt string, maxOutputTokens int) RequestEnvelope {
	t.Helper()
	openAI, ok := value.(*openaiProvider)
	if !ok {
		t.Fatal("requestForTest requires the OpenAI-compatible provider")
	}
	request, err := NewRequestEnvelope(RequestOptions{
		ProviderID:      openAI.cfg.Name,
		BaseURL:         openAI.cfg.BaseURL,
		RequestedModel:  openAI.cfg.Model,
		ThinkingMode:    openAI.cfg.Thinking,
		Messages:        []Message{{Role: "user", Content: prompt}},
		Temperature:     1,
		MaxOutputTokens: maxOutputTokens,
		ResponseFormat:  ResponseFormatText,
		Lineage:         RequestLineage{CriterionID: "test", SamplingSlot: "test", Entrypoint: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func streamRequestForTest(t *testing.T, value Provider, prompt string, maxOutputTokens int, scoreTags ...string) RequestEnvelope {
	t.Helper()
	request := requestForTest(t, value, prompt, maxOutputTokens)
	request.Logprobs = true
	request.TopLogprobs = 20
	request.ScoreTags = append([]string(nil), scoreTags...)
	request.Stream = true
	return request
}

func canonicalRequestForTest(t *testing.T) RequestEnvelope {
	t.Helper()
	seed := 0
	request, err := NewRequestEnvelope(RequestOptions{
		ProviderID:      "provider-a",
		BaseURL:         "HTTPS://Gateway.Example:443/api/v1/",
		RequestedModel:  "model-a",
		ThinkingMode:    "disabled",
		Messages:        []Message{{Role: "system", Content: "Be exact."}, {Role: "user", Content: "score \"this\"\ntrajectory ☃"}},
		Temperature:     0.25,
		Seed:            &seed,
		MaxOutputTokens: 256,
		Logprobs:        true,
		TopLogprobs:     20,
		Stop:            []string{"</score_A>", "stop\u0001"},
		ScoreTags:       []string{"<score_A>", "<score_B>"},
		ResponseFormat:  "text",
		Stream:          true,
		LogitBias:       map[string]int{"42": -10, "7": 4},
		EvidenceBindings: []EvidenceBinding{{
			InputSlot:            "trajectory_0",
			SourceDigest:         strings.Repeat("a", 64),
			CanonicalDigest:      strings.Repeat("b", 64),
			IngestionDigest:      strings.Repeat("c", 64),
			TraceEnvelopeDigest:  strings.Repeat("d", 64),
			MappingReportDigest:  strings.Repeat("e", 64),
			MappingPolicyVersion: "evalwitness.trace-mapping.v1",
		}},
		Lineage: RequestLineage{
			CriterionID:     "correctness",
			SamplingSlot:    "correctness@r0",
			Entrypoint:      "cli",
			AuditCaseID:     "case-1",
			SourceTraceHash: "trace-1",
			TraceMapHash:    "map-1",
			MutationID:      "mutation-1",
			StudyCellID:     "cell-1",
			PolicyHash:      "policy-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestCanonicalRequestNormalizesEquivalentConfigurationSources(t *testing.T) {
	first := canonicalRequestForTest(t)
	second := canonicalRequestForTest(t)
	second.LogitBias = map[string]int{"7": 4, "42": -10}
	second.Stop = append([]string(nil), second.Stop...)
	second.ScoreTags = append([]string(nil), second.ScoreTags...)
	second.Temperature = math.Copysign(0, -1)
	first.Temperature = 0

	firstBytes, err := first.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := second.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("equivalent requests differ:\n%s\n%s", firstBytes, secondBytes)
	}
	if !strings.Contains(string(firstBytes), `"logit_bias":{"42":-10,"7":4}`) {
		t.Fatalf("map keys are not canonical: %s", firstBytes)
	}
}

func TestEverySemanticRequestFieldChangesTheFingerprint(t *testing.T) {
	base := canonicalRequestForTest(t)
	baseFingerprint, err := base.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	seed := 1
	cases := []struct {
		name   string
		mutate func(*RequestEnvelope)
	}{
		{"provider_id", func(r *RequestEnvelope) { r.ProviderID = "provider-b" }},
		{"base_url_origin", func(r *RequestEnvelope) { r.BaseURLOrigin = "https://other.example" }},
		{"endpoint_path", func(r *RequestEnvelope) { r.EndpointPath = "/api/v2/chat/completions" }},
		{"requested_model", func(r *RequestEnvelope) { r.RequestedModel = "model-b" }},
		{"thinking_mode", func(r *RequestEnvelope) { r.ThinkingMode = "enabled" }},
		{"messages", func(r *RequestEnvelope) { r.Messages[1].Content += "!" }},
		{"temperature", func(r *RequestEnvelope) { r.Temperature = 0.5 }},
		{"seed", func(r *RequestEnvelope) { r.Seed = &seed }},
		{"max_output_tokens", func(r *RequestEnvelope) { r.MaxOutputTokens++ }},
		{"logprob_contract", func(r *RequestEnvelope) { r.Logprobs = false; r.TopLogprobs = 0 }},
		{"top_logprobs", func(r *RequestEnvelope) { r.TopLogprobs-- }},
		{"stop", func(r *RequestEnvelope) { r.Stop = append(r.Stop, "later") }},
		{"score_tags", func(r *RequestEnvelope) { r.ScoreTags = []string{"<score_A>"} }},
		{"response_format", func(r *RequestEnvelope) { r.ResponseFormat = "json" }},
		{"stream", func(r *RequestEnvelope) { r.Stream = false }},
		{"prompt_builder_version", func(r *RequestEnvelope) { r.PromptBuilderVersion = "evalwitness.verifier.prompt.v2" }},
		{"logit_bias", func(r *RequestEnvelope) { r.LogitBias["42"] = -9 }},
		{"evidence_bindings", func(r *RequestEnvelope) { r.EvidenceBindings[0].MappingReportDigest = strings.Repeat("f", 64) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneRequest(base)
			test.mutate(&candidate)
			fingerprint, err := candidate.Fingerprint()
			if err != nil {
				t.Fatal(err)
			}
			if fingerprint == baseFingerprint {
				t.Fatalf("%s did not change fingerprint %s", test.name, fingerprint)
			}
		})
	}
}

func TestLineageAndRuntimeCallbacksDoNotChangeSemanticIdentity(t *testing.T) {
	base := canonicalRequestForTest(t)
	candidate := cloneRequest(base)
	candidate.Lineage = RequestLineage{
		CriterionID:     "different",
		SamplingSlot:    "different@r9",
		Entrypoint:      "mcp",
		AuditCaseID:     "case-2",
		SourceTraceHash: "trace-2",
		TraceMapHash:    "map-2",
		MutationID:      "mutation-2",
		StudyCellID:     "cell-2",
		PolicyHash:      "policy-2",
		LegacySource:    "legacy",
	}
	candidate.BeforeAttempt = func() error { return nil }
	baseFingerprint, err := base.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	candidateFingerprint, err := candidate.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if candidateFingerprint != baseFingerprint {
		t.Fatalf("provenance polluted semantic identity: %s != %s", candidateFingerprint, baseFingerprint)
	}
}

func TestResearchLineageValidationRequiresEveryScientificParent(t *testing.T) {
	request := canonicalRequestForTest(t)
	if err := request.Lineage.ValidateResearch(); err != nil {
		t.Fatal(err)
	}
	request.Lineage.TraceMapHash = ""
	err := request.Lineage.ValidateResearch()
	if err == nil || !strings.Contains(err.Error(), "trace_map_hash") {
		t.Fatalf("lineage error = %v", err)
	}
}

func TestCanonicalRequestUsesUnambiguousFieldBoundariesAndValidJSONEscapes(t *testing.T) {
	first := canonicalRequestForTest(t)
	first.Messages = []Message{{Role: "user", Content: "ab"}, {Role: "user", Content: "c"}}
	second := cloneRequest(first)
	second.Messages = []Message{{Role: "user", Content: "a"}, {Role: "user", Content: "bc"}}
	firstFingerprint, err := first.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	secondFingerprint, err := second.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if firstFingerprint == secondFingerprint {
		t.Fatal("message field boundaries collided")
	}
	first.Messages[0].Content = "control:\x00\x1f\x7f emoji:🧪"
	canonical, err := first.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(canonical, []byte(`\x`)) || !bytes.Contains(canonical, []byte(`\u0000\u001f`)) {
		t.Fatalf("canonical bytes are not portable JSON escaping: %s", canonical)
	}
}

func TestCanonicalRequestRejectsInvalidUTF8AndNoncanonicalRoutes(t *testing.T) {
	invalidUTF8 := canonicalRequestForTest(t)
	invalidUTF8.Messages[0].Content = string([]byte{0xff})
	if _, err := invalidUTF8.Fingerprint(); err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("invalid UTF-8 error = %v", err)
	}
	noncanonicalRoute := canonicalRequestForTest(t)
	noncanonicalRoute.BaseURLOrigin = "HTTPS://Gateway.Example:443/"
	if _, err := noncanonicalRoute.Fingerprint(); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("route error = %v", err)
	}
}

func FuzzRequestFingerprintDeterministic(f *testing.F) {
	f.Add("trajectory", "tag", "42", 7)
	f.Add("\x00🧪", "<score_A>", "α", -3)
	f.Fuzz(func(t *testing.T, prompt, tag, biasKey string, bias int) {
		if !utf8.ValidString(prompt) || !utf8.ValidString(tag) || !utf8.ValidString(biasKey) {
			t.Skip()
		}
		request := canonicalRequestForTest(t)
		request.Messages[1].Content = prompt
		request.ScoreTags = []string{tag}
		request.LogitBias = map[string]int{biasKey: bias, "stable": 1}
		copy := cloneRequest(request)
		copy.LogitBias = map[string]int{"stable": 1, biasKey: bias}
		first, err := request.Fingerprint()
		if err != nil {
			t.Fatal(err)
		}
		second, err := copy.Fingerprint()
		if err != nil {
			t.Fatal(err)
		}
		if first != second {
			t.Fatalf("fingerprints differ: %s != %s", first, second)
		}
	})
}

func cloneRequest(request RequestEnvelope) RequestEnvelope {
	request.Messages = append([]Message(nil), request.Messages...)
	request.Stop = append([]string(nil), request.Stop...)
	request.ScoreTags = append([]string(nil), request.ScoreTags...)
	request.Seed = cloneInt(request.Seed)
	request.LogitBias = cloneIntMap(request.LogitBias)
	request.EvidenceBindings = append([]EvidenceBinding(nil), request.EvidenceBindings...)
	return request
}

func TestRequestFingerprintVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/request-fingerprint-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		CorpusSchema         string `json:"corpus_schema"`
		RequestSchemaVersion int    `json:"request_schema_version"`
		Digest               string `json:"digest"`
		NumericContract      string `json:"numeric_contract"`
		Cases                []struct {
			Name              string          `json:"name"`
			Request           RequestEnvelope `json:"request"`
			CanonicalUTF8Hex  string          `json:"canonical_utf8_hex"`
			FingerprintSHA256 Fingerprint     `json:"fingerprint_sha256"`
		} `json:"cases"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.CorpusSchema != "evalwitness.request-fingerprint-corpus.v2" || corpus.RequestSchemaVersion != RequestSchemaVersion || corpus.Digest != "sha256" || corpus.NumericContract == "" {
		t.Fatalf("unsupported corpus contract %q/%d", corpus.CorpusSchema, corpus.RequestSchemaVersion)
	}
	if len(corpus.Cases) < 4 {
		t.Fatalf("vector corpus has %d cases, want at least 4", len(corpus.Cases))
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			wantCanonical, err := hex.DecodeString(test.CanonicalUTF8Hex)
			if err != nil {
				t.Fatal(err)
			}
			gotCanonical, err := test.Request.CanonicalBytes()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotCanonical, wantCanonical) {
				t.Fatalf("canonical bytes differ:\ngot  %x\nwant %x", gotCanonical, wantCanonical)
			}
			fingerprint, err := test.Request.Fingerprint()
			if err != nil {
				t.Fatal(err)
			}
			if fingerprint != test.FingerprintSHA256 {
				t.Fatalf("fingerprint = %s, want %s", fingerprint, test.FingerprintSHA256)
			}
		})
	}
}
