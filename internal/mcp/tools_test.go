package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

// The MCP tools are the surface a coding agent calls. Their schemas are the only
// contract the agent sees: a field the schema advertises but the handler ignores
// is dropped silently, and a field the handler requires but the schema omits
// produces an error the agent cannot anticipate. Nothing checked either
// direction, and none of the three handlers was executed by a test.

// lettersProvider answers every score request with a fixed letter, so a tool
// call completes without a network and its result shape can be asserted.
type lettersProvider struct{ letter string }

func (p *lettersProvider) Name() string { return "mock" }
func (p *lettersProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: 20}
}

func (p *lettersProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	var b strings.Builder
	dist := map[string]map[string]float64{}
	tokenEvidence := []provider.TokenEvidence{}
	for _, tag := range req.ScoreTags {
		closing := strings.Replace(tag, "<", "</", 1)
		fmt.Fprintf(&b, "%s%s%s\n", tag, p.letter, closing)
		dist[tag] = map[string]float64{p.letter: 1}
		tokenEvidence = append(tokenEvidence,
			provider.TokenEvidence{Position: len(tokenEvidence), Token: tag, Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			provider.TokenEvidence{Position: len(tokenEvidence) + 1, Token: p.letter, Logprob: strconv.FormatFloat(math.Log(0.9), 'g', -1, 64), TopAlternatives: mcpScoreAlternatives(p.letter)},
			provider.TokenEvidence{Position: len(tokenEvidence) + 2, Token: closing + "\n", Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
		)
	}
	rawText := b.String()
	return provider.FinalizeResponse(req, provider.ResponseRecord{
		RawText:              rawText,
		NormalizedBody:       []byte(rawText),
		Distributions:        dist,
		HasLogprobs:          true,
		ObservedTopLogprobs:  20,
		OrderedTokenEvidence: tokenEvidence,
	})
}

func mcpScoreAlternatives(chosen string) []provider.TokenAlternative {
	alternatives := make([]provider.TokenAlternative, 0, 20)
	alternatives = append(alternatives, provider.TokenAlternative{Token: chosen, Logprob: strconv.FormatFloat(math.Log(0.9), 'g', -1, 64)})
	for index := 1; index < 20; index++ {
		alternatives = append(alternatives, provider.TokenAlternative{Token: "#" + strconv.Itoa(index), Logprob: strconv.FormatFloat(math.Log(0.1/19), 'g', -1, 64)})
	}
	return alternatives
}

func testHandler(t *testing.T) *ToolHandler {
	t.Helper()
	serviceConfig := verification.Config{
		Redact: true, PreprocessBudget: 1024, Offline: true,
		RequestProfile: verification.RequestProfile{
			ProviderID: "mock", BaseURL: "https://mock.invalid/v1", RequestedModel: "m",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: 20,
		},
		BudgetProfile: verification.BudgetProfile{MaxRetries: 1, MaxWorkers: 1, RequestTimeout: time.Second},
	}
	service, err := verification.NewService(serviceConfig, func(_ context.Context, plan verification.Plan) (verification.Runtime, error) {
		return verification.Runtime{Runner: &mode.Runner{
			Provider: &lettersProvider{letter: "A"},
			Cfg: mode.RunnerConfig{
				Model: "m", BaseURL: "https://mock.invalid/v1", ThinkingMode: "disabled", Temperature: 1,
				Entrypoint: plan.Input.Entrypoint, MaxTokens: 128, TopLogprobs: 20, MaxWorkers: 1,
			},
			Budget: mode.NewRunBudget(plan.Input.Limits),
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return &ToolHandler{
		Service: service,
		Policy: verification.Policy{
			Evidence: verification.EvidenceStrictVerifier, NReps: 1, Epsilon: 0.02,
			BiasMitigation: "adaptive", InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise",
			MaxWorkers: 1, MaxPairCalls: 2, ConfidenceThreshold: 0.6, CalibrationSigma: 0.05,
		},
	}
}

func TestToolsAdvertiseCanonicalAndDeprecatedAliasSchemas(t *testing.T) {
	tools := (&ToolHandler{}).Tools()
	if len(tools) != 7 {
		t.Fatalf("%d tools, want 7", len(tools))
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name %q", tool.Name)
		}
		seen[tool.Name] = true
		if tool.Description == "" {
			t.Fatalf("%s has no description; an agent picks tools by description", tool.Name)
		}
		if tool.InputSchema["type"] != "object" {
			t.Fatalf("%s schema type = %v, want object", tool.Name, tool.InputSchema["type"])
		}
		// The schema crosses the wire as JSON. A value that cannot marshal makes
		// the whole tools/list response fail rather than one tool.
		if _, err := json.Marshal(tool.InputSchema); err != nil {
			t.Fatalf("%s schema does not marshal: %v", tool.Name, err)
		}
	}
	for _, want := range []string{ToolPairwise, ToolAbsolute, ToolDelta, legacyToolPairwise, legacyToolAbsolute, legacyToolDelta, ToolCalibrationEvaluate} {
		if !seen[want] {
			t.Fatalf("missing tool %s", want)
		}
	}
	for _, tool := range tools[3:6] {
		if !strings.Contains(tool.Description, "DEPRECATED") || !strings.Contains(tool.Description, canonicalToolName(tool.Name)) {
			t.Fatalf("legacy tool %s has no canonical deprecation pointer: %q", tool.Name, tool.Description)
		}
	}
	if tools[6].Name != ToolCalibrationEvaluate {
		t.Fatalf("calibration tool = %q", tools[6].Name)
	}
}

func TestLegacyToolsUseCanonicalSchemasAndHandlers(t *testing.T) {
	h := testHandler(t)
	tools := h.Tools()
	for index := 0; index < 3; index++ {
		canonical := tools[index]
		legacy := tools[index+3]
		if !reflect.DeepEqual(canonical.InputSchema, legacy.InputSchema) {
			t.Fatalf("%s and %s have different schemas", canonical.Name, legacy.Name)
		}
		var args json.RawMessage
		switch canonical.Name {
		case ToolPairwise:
			args = json.RawMessage(`{"task":"t","trajectories":["a","b"]}`)
		case ToolAbsolute:
			args = json.RawMessage(`{"task":"t","trajectory":"a"}`)
		case ToolDelta:
			args = json.RawMessage(`{"task":"t","trajectory_a":"a","trajectory_b":"b"}`)
		}
		canonicalResult, err := h.Call(context.Background(), canonical.Name, args)
		if err != nil {
			t.Fatal(err)
		}
		legacyResult, err := h.Call(context.Background(), legacy.Name, args)
		if err != nil {
			t.Fatal(err)
		}
		canonicalJSON, err := json.Marshal(canonicalResult)
		if err != nil {
			t.Fatal(err)
		}
		legacyJSON, err := json.Marshal(legacyResult)
		if err != nil {
			t.Fatal(err)
		}
		if string(canonicalJSON) != string(legacyJSON) {
			t.Fatalf("%s and %s returned different results\ncanonical: %s\nlegacy: %s", canonical.Name, legacy.Name, canonicalJSON, legacyJSON)
		}
	}
}

func TestSchemaRequiredFieldsMatchWhatHandlersEnforce(t *testing.T) {
	// The contract that would otherwise rot silently: every field the schema
	// marks required must actually be rejected when absent, and every property
	// the schema advertises must be a field the argument struct decodes.
	h := testHandler(t)
	cases := []struct {
		tool    string
		full    map[string]any
		argType any
	}{
		{"evalwitness_pairwise", map[string]any{"task": "t", "trajectories": []string{"a", "b"}}, &pairwiseArgs{}},
		{"evalwitness_absolute", map[string]any{"task": "t", "trajectory": "a"}, &absoluteArgs{}},
		{"evalwitness_delta", map[string]any{"task": "t", "trajectory_a": "a", "trajectory_b": "b"}, &deltaArgs{}},
		{ToolCalibrationEvaluate, map[string]any{
			"observations": []map[string]any{{"id": "1", "task_id": "t1", "split_role": "test", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.9, "won": true}},
			"threshold":    0.5,
			"target_risk":  0.9,
			"min_coverage": 0.1,
		}, &calibrationEvaluateArgs{}},
	}
	schemas := map[string]map[string]any{}
	for _, tool := range h.Tools() {
		schemas[tool.Name] = tool.InputSchema
	}

	for _, tc := range cases {
		schema := schemas[tc.tool]
		required, _ := schema["required"].([]string)
		if len(required) == 0 {
			t.Fatalf("%s declares no required fields", tc.tool)
		}
		for _, field := range required {
			args := map[string]any{}
			for k, v := range tc.full {
				if k != field {
					args[k] = v
				}
			}
			raw, _ := json.Marshal(args)
			if _, err := h.Call(context.Background(), tc.tool, raw); err == nil {
				t.Fatalf("%s accepted a call missing required field %q", tc.tool, field)
			}
		}

		// Every advertised property must decode into the argument struct,
		// otherwise an agent sets it and nothing happens.
		props, _ := schema["properties"].(map[string]any)
		fields := map[string]bool{}
		for _, tag := range jsonFieldNames(tc.argType) {
			fields[tag] = true
		}
		for name := range props {
			if !fields[name] {
				t.Fatalf("%s advertises %q but the handler decodes no such field", tc.tool, name)
			}
		}
	}
}

// jsonFieldNames lists the json tags an argument struct decodes.
func jsonFieldNames(v any) []string {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	// Fields tagged omitempty vanish from a zero value, so add them explicitly.
	out := []string{"criteria", "n_reps", "authorization_digest"}
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestUnknownToolIsMethodNotFound(t *testing.T) {
	_, err := testHandler(t).Call(context.Background(), "evalwitness_nonexistent", json.RawMessage(`{}`))
	var te *ToolError
	if !errors.As(err, &te) {
		t.Fatalf("error is %T, want *ToolError", err)
	}
	if te.Code != -32601 {
		t.Fatalf("code = %d, want -32601 method not found", te.Code)
	}
}

func TestMalformedArgumentsAreInvalidParams(t *testing.T) {
	for _, tool := range []string{"evalwitness_pairwise", "evalwitness_absolute", "evalwitness_delta"} {
		_, err := testHandler(t).Call(context.Background(), tool, json.RawMessage(`{"task": 12345}`))
		var te *ToolError
		if !errors.As(err, &te) {
			t.Fatalf("%s: error is %T, want *ToolError", tool, err)
		}
		if te.Code != -32602 {
			t.Fatalf("%s: code = %d, want -32602 invalid params", tool, te.Code)
		}
	}
}

func TestPairwiseRejectsFewerThanTwoTrajectories(t *testing.T) {
	// A one-trajectory pairwise call has no comparison to make. Accepting it
	// would return a winner index of 0 with a confidence that means nothing.
	raw := json.RawMessage(`{"task":"t","trajectories":["only one"]}`)
	_, err := testHandler(t).Call(context.Background(), "evalwitness_pairwise", raw)
	var te *ToolError
	if !errors.As(err, &te) || te.Code != -32602 {
		t.Fatalf("err = %v, want invalid params", err)
	}
}

func TestUnknownCriterionIsRejectedWithItsOwnCode(t *testing.T) {
	// -32006 rather than a generic invalid-params, so an agent can tell a typo
	// in a criterion id from a malformed call and retry differently.
	raw := json.RawMessage(`{"task":"t","trajectories":["a","b"],"criteria":["not_a_criterion"]}`)
	_, err := testHandler(t).Call(context.Background(), "evalwitness_pairwise", raw)
	var te *ToolError
	if !errors.As(err, &te) {
		t.Fatalf("error is %T, want *ToolError", err)
	}
	if te.Code != -32006 {
		t.Fatalf("code = %d, want -32006", te.Code)
	}
}

func TestStrictEvidenceFailureHasDedicatedMCPCode(t *testing.T) {
	evidence := verifier.ScoreEvidence{
		SchemaVersion:   verifier.ScoreEvidenceSchemaVersion,
		PolicyVersion:   verifier.StrictPolicyVersion,
		Tag:             "<score>",
		ExtractionMode:  verifier.ExtractionModeVerifier,
		AlignmentStatus: verifier.AlignmentMissing,
		Degradations:    []verifier.Degradation{{Code: verifier.DegradationMissingLogprobs}},
	}
	err := translateError(&verifier.EvidenceError{Tag: evidence.Tag, Evidence: evidence})
	var toolError *ToolError
	if !errors.As(err, &toolError) || toolError.Code != -32010 || !reflect.DeepEqual(toolError.Data, evidence) {
		t.Fatalf("translated error = %#v", err)
	}
}

func TestMCPAuthorizationPreviewStopsBeforeRunnerExecution(t *testing.T) {
	called := false
	serviceConfig := verification.Config{
		Redact: true, PreprocessBudget: 1024,
		RequestProfile: verification.RequestProfile{
			ProviderID: "mock", BaseURL: "https://mock.invalid/v1", RequestedModel: "m",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: 20,
		},
		BudgetProfile: verification.BudgetProfile{MaxRetries: 1, MaxWorkers: 1, RequestTimeout: time.Second},
	}
	service, err := verification.NewService(serviceConfig, func(context.Context, verification.Plan) (verification.Runtime, error) {
		called = true
		return verification.Runtime{}, errors.New("runtime must not open")
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := &ToolHandler{Service: service, Policy: testHandler(t).Policy}
	_, err = handler.Call(context.Background(), ToolPairwise, json.RawMessage(`{"task":"t","trajectories":["a","b"]}`))
	var toolError *ToolError
	if !errors.As(err, &toolError) || toolError.Code != -32011 {
		t.Fatalf("authorization preview error = %#v", err)
	}
	plan, ok := toolError.Data.(mode.AuthorizationPlan)
	if !ok || plan.AuthorizationDigest == "" {
		t.Fatalf("authorization preview data = %#v", toolError.Data)
	}
	if called {
		t.Fatal("runtime opened before authorization")
	}
}

func TestSchemaCriterionEnumMatchesTheResolvableSet(t *testing.T) {
	// An agent picks criterion ids from the enum. One that the resolver rejects
	// would be advertised and unusable.
	h := testHandler(t)
	var enum []string
	for _, tool := range h.Tools() {
		props := tool.InputSchema["properties"].(map[string]any)
		crit := props["criteria"].(map[string]any)
		items := crit["items"].(map[string]any)
		enum, _ = items["enum"].([]string)
		break
	}
	if len(enum) == 0 {
		t.Fatal("criteria enum is empty; an agent has nothing to choose from")
	}
	for _, name := range enum {
		if _, err := verifier.ResolveCriteria([]string{name}); err != nil {
			t.Fatalf("schema advertises criterion %q that does not resolve: %v", name, err)
		}
	}
}

func TestPairwiseCallReturnsASelection(t *testing.T) {
	raw := json.RawMessage(`{"task":"reverse a string","trajectories":["attempt one","attempt two"]}`)
	got, err := testHandler(t).Call(context.Background(), "evalwitness_pairwise", raw)
	if err != nil {
		t.Fatal(err)
	}
	sel, ok := got.(mode.Selection)
	if !ok {
		t.Fatalf("result is %T, want mode.Selection", got)
	}
	if sel.State != verifier.DecisionTied || sel.BestIndex != -1 {
		t.Fatalf("equal evidence was upgraded to a winner: %+v", sel)
	}
	if len(sel.Scores) != 2 {
		t.Fatalf("%d scores for 2 trajectories", len(sel.Scores))
	}
	// The result crosses the wire as JSON; a shape that cannot marshal reaches
	// the agent as a protocol error rather than a result.
	if _, err := json.Marshal(sel); err != nil {
		t.Fatalf("selection does not marshal: %v", err)
	}
}

func TestAbsoluteAndDeltaCallsReturnMarshalableResults(t *testing.T) {
	h := testHandler(t)
	cases := map[string]json.RawMessage{
		"evalwitness_absolute": json.RawMessage(`{"task":"t","trajectory":"one attempt"}`),
		"evalwitness_delta":    json.RawMessage(`{"task":"t","trajectory_a":"a","trajectory_b":"b"}`),
	}
	for tool, raw := range cases {
		got, err := h.Call(context.Background(), tool, raw)
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		if _, err := json.Marshal(got); err != nil {
			t.Fatalf("%s result does not marshal: %v", tool, err)
		}
	}
}

func TestNRepsDefaultsOnlyWhenAbsentAndRejectsOutOfSchemaValues(t *testing.T) {
	h := testHandler(t)
	got, err := h.Call(context.Background(), "evalwitness_pairwise", json.RawMessage(`{"task":"t","trajectories":["a","b"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if selection := got.(mode.Selection); selection.Usage.Calls == 0 {
		t.Fatal("absent n_reps did not use the configured default")
	}
	for _, value := range []int{0, -3, verification.MaxRepetitions + 1} {
		body := fmt.Sprintf(`{"task":"t","trajectories":["a","b"],"n_reps":%d}`, value)
		_, err := h.Call(context.Background(), "evalwitness_pairwise", json.RawMessage(body))
		var toolError *ToolError
		if !errors.As(err, &toolError) || toolError.Code != -32602 {
			t.Fatalf("n_reps=%d error = %#v", value, err)
		}
	}
}

func TestServiceContractBoundaryErrorsAreInvalidParams(t *testing.T) {
	trajectories := make([]string, verification.MaxTrajectories+1)
	for index := range trajectories {
		trajectories[index] = fmt.Sprintf("trajectory-%d", index)
	}
	payload, err := json.Marshal(map[string]any{"task": "t", "trajectories": trajectories})
	if err != nil {
		t.Fatal(err)
	}
	_, err = testHandler(t).Call(context.Background(), ToolPairwise, payload)
	var toolError *ToolError
	if !errors.As(err, &toolError) || toolError.Code != -32602 {
		t.Fatalf("service contract error = %#v", err)
	}
}
