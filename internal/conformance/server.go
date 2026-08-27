package conformance

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
)

type Scenario string

const (
	ScenarioValid             Scenario = "valid"
	ScenarioMissingTopK       Scenario = "missing_top_k"
	ScenarioLowScoreMass      Scenario = "low_score_mass"
	ScenarioMalformedSSE      Scenario = "malformed_sse"
	ScenarioMissingUsage      Scenario = "missing_usage"
	ScenarioDelayedRetryAfter Scenario = "delayed_retry_after"
	ScenarioAliasMismatch     Scenario = "alias_mismatch"
	ScenarioTruncation        Scenario = "truncation"
	ScenarioDegenerate        Scenario = "degenerate_distribution"
)

type DeterministicServer struct {
	scenario Scenario
	requests atomic.Int64
}

type conformanceUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type conformanceAlternative struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

type conformanceToken struct {
	Token       string                   `json:"token"`
	Logprob     float64                  `json:"logprob"`
	TopLogprobs []conformanceAlternative `json:"top_logprobs"`
}

type conformanceLogprobs struct {
	Content []conformanceToken `json:"content"`
}

type conformanceMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type conformanceChoice struct {
	Index        int                 `json:"index"`
	Message      conformanceMessage  `json:"message"`
	FinishReason string              `json:"finish_reason"`
	Logprobs     conformanceLogprobs `json:"logprobs"`
}

type conformanceResponse struct {
	ID      string              `json:"id"`
	Model   string              `json:"model"`
	Choices []conformanceChoice `json:"choices"`
	Usage   *conformanceUsage   `json:"usage,omitempty"`
}

type conformanceDelta struct {
	Content string `json:"content"`
}

type conformanceStreamChoice struct {
	Index        int                 `json:"index"`
	Delta        conformanceDelta    `json:"delta"`
	FinishReason string              `json:"finish_reason"`
	Logprobs     conformanceLogprobs `json:"logprobs"`
}

type conformanceStreamChunk struct {
	ID      string                    `json:"id"`
	Model   string                    `json:"model"`
	Choices []conformanceStreamChoice `json:"choices"`
	Usage   *conformanceUsage         `json:"usage,omitempty"`
}

func NewDeterministicServer(scenario Scenario) (*DeterministicServer, error) {
	if !scenario.Valid() {
		return nil, fmt.Errorf("unsupported conformance scenario %q", scenario)
	}
	return &DeterministicServer{scenario: scenario}, nil
}

func (s Scenario) Valid() bool {
	switch s {
	case ScenarioValid, ScenarioMissingTopK, ScenarioLowScoreMass, ScenarioMalformedSSE,
		ScenarioMissingUsage, ScenarioDelayedRetryAfter, ScenarioAliasMismatch,
		ScenarioTruncation, ScenarioDegenerate:
		return true
	default:
		return false
	}
}

func (s *DeterministicServer) Requests() int64 {
	if s == nil {
		return 0
	}
	return s.requests.Load()
}

func (s *DeterministicServer) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if s == nil {
		http.Error(response, "server is not configured", http.StatusInternalServerError)
		return
	}
	s.requests.Add(1)
	if request.Method != http.MethodPost {
		http.Error(response, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Stream bool `json:"stream"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		http.Error(response, "invalid request JSON", http.StatusBadRequest)
		return
	}
	if s.scenario == ScenarioDelayedRetryAfter {
		response.Header().Set("Retry-After", "120")
		response.WriteHeader(http.StatusTooManyRequests)
		_, _ = response.Write([]byte(`{"error":{"message":"rate limited by deterministic conformance scenario"}}`))
		return
	}
	if s.scenario == ScenarioMalformedSSE {
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(response, "data: {malformed\n\n")
		return
	}
	if body.Stream {
		s.writeStream(response)
		return
	}
	s.writeJSON(response)
}

func (s *DeterministicServer) writeJSON(response http.ResponseWriter) {
	content, finishReason, tokens := s.payload()
	servedModel := "requested-alias"
	if s.scenario == ScenarioAliasMismatch {
		servedModel = "different-served-alias"
	}
	payload := conformanceResponse{
		ID: "conformance-request-1", Model: servedModel,
		Choices: []conformanceChoice{{
			Index: 0, Message: conformanceMessage{Role: "assistant", Content: content},
			FinishReason: finishReason, Logprobs: conformanceLogprobs{Content: tokens},
		}},
	}
	if s.scenario != ScenarioMissingUsage {
		payload.Usage = &conformanceUsage{PromptTokens: 21, CompletionTokens: 3}
	}
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(payload)
}

func (s *DeterministicServer) writeStream(response http.ResponseWriter) {
	content, finishReason, tokens := s.payload()
	servedModel := "requested-alias"
	if s.scenario == ScenarioAliasMismatch {
		servedModel = "different-served-alias"
	}
	chunk := conformanceStreamChunk{
		ID: "conformance-request-1", Model: servedModel,
		Choices: []conformanceStreamChoice{{
			Index: 0, Delta: conformanceDelta{Content: content}, FinishReason: finishReason,
			Logprobs: conformanceLogprobs{Content: tokens},
		}},
	}
	encoded, _ := json.Marshal(chunk)
	response.Header().Set("Content-Type", "text/event-stream")
	_, _ = fmt.Fprintf(response, "data: %s\n\n", encoded)
	if s.scenario != ScenarioMissingUsage {
		usage, _ := json.Marshal(conformanceStreamChunk{
			ID: "conformance-request-1", Model: servedModel, Choices: []conformanceStreamChoice{},
			Usage: &conformanceUsage{PromptTokens: 21, CompletionTokens: 3},
		})
		_, _ = fmt.Fprintf(response, "data: %s\n\n", usage)
	}
	_, _ = fmt.Fprint(response, "data: [DONE]\n\n")
}

func (s *DeterministicServer) payload() (string, string, []conformanceToken) {
	if s.scenario == ScenarioTruncation {
		return "<score_A>", "length", []conformanceToken{tokenRecord("<score_A>", 0, ordinaryAlternatives(20))}
	}
	topK := 20
	if s.scenario == ScenarioMissingTopK {
		topK = 5
	}
	alternatives := ordinaryAlternatives(topK)
	if s.scenario == ScenarioLowScoreMass {
		alternatives = lowMassAlternatives(topK)
	}
	if s.scenario == ScenarioDegenerate {
		return "<score_A>A</score_A>", "stop", []conformanceToken{
			tokenRecord("<score_A>", 0, degenerateAlternatives(topK, "<score_A>")),
			tokenRecord("A", 0, degenerateAlternatives(topK, "A")),
			tokenRecord("</score_A>", 0, []conformanceAlternative{}),
		}
	}
	chosenLogprob := math.Log(0.6)
	if s.scenario == ScenarioLowScoreMass {
		chosenLogprob = math.Log(0.049)
	}
	return "<score_A>A</score_A>", "stop", []conformanceToken{
		tokenRecord("<score_A>", 0, []conformanceAlternative{}),
		tokenRecord("A", chosenLogprob, alternatives),
		tokenRecord("</score_A>", 0, []conformanceAlternative{}),
	}
}

func tokenRecord(token string, logprob float64, alternatives []conformanceAlternative) conformanceToken {
	return conformanceToken{Token: token, Logprob: logprob, TopLogprobs: alternatives}
}

func ordinaryAlternatives(topK int) []conformanceAlternative {
	values := []conformanceAlternative{
		{Token: "A", Logprob: math.Log(0.6)},
		{Token: "B", Logprob: math.Log(0.2)},
	}
	return fillAlternatives(values, topK, -1000)
}

func lowMassAlternatives(topK int) []conformanceAlternative {
	values := []conformanceAlternative{
		{Token: "A", Logprob: math.Log(0.049)},
		{Token: "!", Logprob: math.Log(0.90)},
	}
	return fillAlternatives(values, topK, -1000)
}

func degenerateAlternatives(topK int, chosen string) []conformanceAlternative {
	values := []conformanceAlternative{{Token: chosen, Logprob: 0}}
	return fillAlternatives(values, topK, -9999)
}

func fillAlternatives(values []conformanceAlternative, topK int, fillerLogprob float64) []conformanceAlternative {
	for len(values) < topK {
		values = append(values, conformanceAlternative{Token: "#" + strconv.Itoa(len(values)), Logprob: fillerLogprob})
	}
	if len(values) > topK {
		values = values[:topK]
	}
	return values
}
