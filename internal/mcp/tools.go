package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type ToolHandler struct {
	Service verificationService
	Policy  verification.Policy
}

type verificationService interface {
	Plan(verification.Input) (verification.Plan, error)
	Execute(context.Context, verification.Plan) (verification.Result, error)
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

const (
	ToolPairwise            = "evalwitness_pairwise"
	ToolAbsolute            = "evalwitness_absolute"
	ToolDelta               = "evalwitness_delta"
	ToolCalibrationEvaluate = "evalwitness_calibration_evaluate"

	legacyToolPairwise = "logprobe_pairwise"
	legacyToolAbsolute = "logprobe_absolute"
	legacyToolDelta    = "logprobe_delta"
)

type pairwiseArgs struct {
	Task                string   `json:"task"`
	Trajectories        []string `json:"trajectories"`
	Criteria            []string `json:"criteria,omitempty"`
	NReps               *int     `json:"n_reps,omitempty"`
	AuthorizationDigest string   `json:"authorization_digest,omitempty"`
}

type absoluteArgs struct {
	Task                string   `json:"task"`
	Trajectory          string   `json:"trajectory"`
	Criteria            []string `json:"criteria,omitempty"`
	NReps               *int     `json:"n_reps,omitempty"`
	AuthorizationDigest string   `json:"authorization_digest,omitempty"`
}

type deltaArgs struct {
	Task                string   `json:"task"`
	TrajectoryA         string   `json:"trajectory_a"`
	TrajectoryB         string   `json:"trajectory_b"`
	Criteria            []string `json:"criteria,omitempty"`
	NReps               *int     `json:"n_reps,omitempty"`
	AuthorizationDigest string   `json:"authorization_digest,omitempty"`
}

func (h *ToolHandler) Tools() []Tool {
	canonical := []Tool{
		{
			Name:        ToolPairwise,
			Description: "Select or abstain among 2-10 agent trajectories using strict Top-20 score evidence and order-bias mitigation. Returns decision state, non-calibrated decision strength, conditional scores, uncertainty components, and raw score evidence.",
			InputSchema: pairwiseSchema(),
		},
		{
			Name:        ToolAbsolute,
			Description: "Score a single trajectory given a task description and one or more criteria. Returns a conditional score, structured evidence strength, and per-observation score evidence; it does not claim calibrated confidence.",
			InputSchema: absoluteSchema(),
		},
		{
			Name:        ToolDelta,
			Description: "Compare trajectory A vs trajectory B and select, tie, or abstain. Returns conditional scores, order evidence, uncertainty components, evidence strength, and explicit inconsistency diagnostics.",
			InputSchema: deltaSchema(),
		},
	}
	return append(canonical,
		deprecatedToolAlias(canonical[0], legacyToolPairwise),
		deprecatedToolAlias(canonical[1], legacyToolAbsolute),
		deprecatedToolAlias(canonical[2], legacyToolDelta),
		calibrationEvaluateTool(),
	)
}

func deprecatedToolAlias(canonical Tool, legacyName string) Tool {
	canonical.Name = legacyName
	canonical.Description = "DEPRECATED: use " + canonicalToolName(legacyName) + ". " + canonical.Description
	return canonical
}

func canonicalToolName(name string) string {
	switch name {
	case legacyToolPairwise:
		return ToolPairwise
	case legacyToolAbsolute:
		return ToolAbsolute
	case legacyToolDelta:
		return ToolDelta
	default:
		return name
	}
}

func (h *ToolHandler) Call(ctx context.Context, name string, args json.RawMessage) (any, error) {
	var (
		result any
		err    error
	)
	switch canonicalToolName(name) {
	case ToolPairwise:
		result, err = h.callPairwise(ctx, args)
	case ToolAbsolute:
		result, err = h.callAbsolute(ctx, args)
	case ToolDelta:
		result, err = h.callDelta(ctx, args)
	case ToolCalibrationEvaluate:
		result, err = h.callCalibrationEvaluate(args)
	default:
		return nil, &ToolError{Code: -32601, Message: "unknown tool", Data: name}
	}
	if err != nil {
		return nil, translateError(err)
	}
	return result, nil
}

// translateError maps domain errors onto the documented MCP error taxonomy so
// clients see -32001..-32008 instead of a generic internal error.
func translateError(err error) error {
	var te *ToolError
	if errors.As(err, &te) {
		return te
	}
	var cce *mode.CostCapError
	if errors.As(err, &cce) {
		return &ToolError{Code: -32008, Message: "cost cap exceeded", Data: map[string]any{
			"est_cost_usd": cce.EstCostUSD,
			"cap_usd":      cce.CapUSD,
		}}
	}
	var budgetError *mode.BudgetExceededError
	if errors.As(err, &budgetError) {
		return &ToolError{Code: -32009, Message: "run budget exceeded", Data: map[string]any{
			"metric":    budgetError.Metric,
			"used":      budgetError.Used,
			"requested": budgetError.Requested,
			"limit":     budgetError.Limit,
		}}
	}
	var authorizationError *verification.AuthorizationRequiredError
	if errors.As(err, &authorizationError) {
		return &ToolError{Code: -32011, Message: "live authorization required or changed", Data: authorizationError.Plan}
	}
	var evidenceError *verifier.EvidenceError
	if errors.As(err, &evidenceError) {
		return &ToolError{Code: -32010, Message: "score evidence rejected", Data: evidenceError.Evidence}
	}
	var pe *provider.ProviderError
	if errors.As(err, &pe) {
		data := map[string]any{
			"provider":    pe.Provider,
			"status_code": pe.Status,
			"request_id":  pe.RequestID,
		}
		switch pe.Class {
		case provider.ClassRateLimited:
			data["retry_after_sec"] = pe.RetryAfter
			return &ToolError{Code: -32002, Message: "provider rate limited", Data: data}
		case provider.ClassAuthFailed:
			return &ToolError{Code: -32007, Message: "provider auth failed", Data: data}
		case provider.ClassContextOverflow:
			return &ToolError{Code: -32005, Message: "trajectory too large for provider context", Data: data}
		case provider.ClassCapabilityMissing:
			data["missing"] = "logprobs"
			return &ToolError{Code: -32003, Message: "provider capability missing", Data: data}
		default:
			data["body"] = pe.Body
			return &ToolError{Code: -32001, Message: "provider error", Data: data}
		}
	}
	if errors.Is(err, provider.ErrOffline) {
		return &ToolError{Code: -32001, Message: "offline mode refused network call"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &ToolError{Code: -32004, Message: "request timed out"}
	}
	return err
}

func (h *ToolHandler) callPairwise(ctx context.Context, raw json.RawMessage) (any, error) {
	var a pairwiseArgs
	if err := decodeToolArguments(raw, &a); err != nil {
		return nil, &ToolError{Code: -32602, Message: "invalid params", Data: err.Error()}
	}
	if a.Task == "" {
		return nil, &ToolError{Code: -32602, Message: "task required"}
	}
	if len(a.Trajectories) < 2 {
		return nil, &ToolError{Code: -32602, Message: "trajectories must have at least 2 items"}
	}
	crits, err := h.resolveCriteria(a.Criteria)
	if err != nil {
		return nil, err
	}
	result, err := h.execute(ctx, verification.ModePairwise, a.Task, a.Trajectories, crits, a.NReps, a.AuthorizationDigest)
	if err != nil {
		return nil, err
	}
	return *result.Selection, nil
}

func (h *ToolHandler) callAbsolute(ctx context.Context, raw json.RawMessage) (any, error) {
	var a absoluteArgs
	if err := decodeToolArguments(raw, &a); err != nil {
		return nil, &ToolError{Code: -32602, Message: "invalid params", Data: err.Error()}
	}
	if a.Task == "" {
		return nil, &ToolError{Code: -32602, Message: "task required"}
	}
	if a.Trajectory == "" {
		return nil, &ToolError{Code: -32602, Message: "trajectory required"}
	}
	crits, err := h.resolveCriteria(a.Criteria)
	if err != nil {
		return nil, err
	}
	result, err := h.execute(ctx, verification.ModeAbsolute, a.Task, []string{a.Trajectory}, crits, a.NReps, a.AuthorizationDigest)
	if err != nil {
		return nil, err
	}
	return *result.Absolute, nil
}

func (h *ToolHandler) callDelta(ctx context.Context, raw json.RawMessage) (any, error) {
	var a deltaArgs
	if err := decodeToolArguments(raw, &a); err != nil {
		return nil, &ToolError{Code: -32602, Message: "invalid params", Data: err.Error()}
	}
	if a.Task == "" {
		return nil, &ToolError{Code: -32602, Message: "task required"}
	}
	if a.TrajectoryA == "" || a.TrajectoryB == "" {
		return nil, &ToolError{Code: -32602, Message: "trajectory_a and trajectory_b required"}
	}
	crits, err := h.resolveCriteria(a.Criteria)
	if err != nil {
		return nil, err
	}
	result, err := h.execute(ctx, verification.ModeDelta, a.Task, []string{a.TrajectoryA, a.TrajectoryB}, crits, a.NReps, a.AuthorizationDigest)
	if err != nil {
		return nil, err
	}
	return *result.Delta, nil
}

func (h *ToolHandler) execute(ctx context.Context, modeName verification.Mode, task string, trajectories []string, criteria []verifier.Criterion, nReps *int, authorizationDigest string) (verification.Result, error) {
	if h.Service == nil {
		return verification.Result{}, errors.New("MCP verification service is not configured")
	}
	policy := h.Policy
	if nReps != nil {
		if *nReps < 1 || *nReps > verification.MaxRepetitions {
			return verification.Result{}, &ToolError{Code: -32602, Message: "invalid params", Data: fmt.Sprintf("n_reps must be between 1 and %d", verification.MaxRepetitions)}
		}
		policy.NReps = *nReps
	}
	plan, err := h.Service.Plan(verification.Input{
		Entrypoint: "mcp." + string(modeName), Mode: modeName, Task: task, Trajectories: trajectories,
		Criteria: criteria, Policy: policy, AuthorizationDigest: authorizationDigest,
	})
	if err != nil {
		return verification.Result{}, &ToolError{Code: -32602, Message: "invalid params", Data: err.Error()}
	}
	return h.Service.Execute(ctx, plan)
}

func decodeToolArguments(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return fmt.Errorf("trailing JSON value: %w", err)
	}
	return nil
}

func (h *ToolHandler) resolveCriteria(names []string) ([]verifier.Criterion, error) {
	crits, err := verifier.ResolveCriteria(names)
	if err != nil {
		return nil, &ToolError{Code: -32006, Message: err.Error()}
	}
	return crits, nil
}

func pairwiseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task":                 map[string]any{"type": "string", "minLength": 1, "maxLength": verification.MaxTaskBytes, "description": "Task description or problem statement"},
			"trajectories":         map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1, "maxLength": verification.MaxTrajectoryBytes}, "minItems": 2, "maxItems": verification.MaxTrajectories, "description": "2 to 10 agent trace texts"},
			"criteria":             map[string]any{"type": "array", "maxItems": verification.MaxCriteria, "uniqueItems": true, "items": map[string]any{"type": "string", "enum": criterionNames()}, "description": "preset criterion ids; default [generic]"},
			"n_reps":               map[string]any{"type": "integer", "minimum": 1, "maximum": verification.MaxRepetitions, "description": "reps per pair-criterion"},
			"authorization_digest": map[string]any{"type": "string", "description": "digest returned by the authorization-required preview"},
		},
		"required": []string{"task", "trajectories"},
	}
}

func absoluteSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task":                 map[string]any{"type": "string", "minLength": 1, "maxLength": verification.MaxTaskBytes},
			"trajectory":           map[string]any{"type": "string", "minLength": 1, "maxLength": verification.MaxTrajectoryBytes},
			"criteria":             map[string]any{"type": "array", "maxItems": verification.MaxCriteria, "uniqueItems": true, "items": map[string]any{"type": "string", "enum": criterionNames()}},
			"n_reps":               map[string]any{"type": "integer", "minimum": 1, "maximum": verification.MaxRepetitions},
			"authorization_digest": map[string]any{"type": "string"},
		},
		"required": []string{"task", "trajectory"},
	}
}

func deltaSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task":                 map[string]any{"type": "string", "minLength": 1, "maxLength": verification.MaxTaskBytes},
			"trajectory_a":         map[string]any{"type": "string", "minLength": 1, "maxLength": verification.MaxTrajectoryBytes},
			"trajectory_b":         map[string]any{"type": "string", "minLength": 1, "maxLength": verification.MaxTrajectoryBytes},
			"criteria":             map[string]any{"type": "array", "maxItems": verification.MaxCriteria, "uniqueItems": true, "items": map[string]any{"type": "string", "enum": criterionNames()}},
			"n_reps":               map[string]any{"type": "integer", "minimum": 1, "maximum": verification.MaxRepetitions},
			"authorization_digest": map[string]any{"type": "string"},
		},
		"required": []string{"task", "trajectory_a", "trajectory_b"},
	}
}

func criterionNames() []string {
	out := make([]string, 0, len(verifier.BuiltinCriteria))
	for k := range verifier.BuiltinCriteria {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
