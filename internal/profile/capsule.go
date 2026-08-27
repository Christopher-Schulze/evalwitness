package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// CapsuleRef is a minimal TASK 050 capsule reference for profile building.
type CapsuleRef struct {
	Digest string `json:"digest"`
	Path   string `json:"path"`
}

// EvaluateExpression reads capsule JSON and extracts metric via simple path lookup.
// Path is dot-separated JSON path, offline only, no network. No map[string]any used.
func EvaluateExpression(capsulePath string, expr string) (string, error) {
	b, err := os.ReadFile(capsulePath)
	if err != nil {
		return "", fmt.Errorf("profile: read capsule: %w", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return "", fmt.Errorf("profile: decode capsule: %w", err)
	}
	parts := splitDot(expr)
	if len(parts) == 0 {
		return "", fmt.Errorf("profile: expr %q empty", expr)
	}
	curMap := top
	var curRaw json.RawMessage
	for i, part := range parts {
		raw, ok := curMap[part]
		if !ok {
			return "", fmt.Errorf("profile: expr %q missing %q", expr, part)
		}
		if i == len(parts)-1 {
			curRaw = raw
			break
		}
		var next map[string]json.RawMessage
		if err := json.Unmarshal(raw, &next); err != nil {
			return "", fmt.Errorf("profile: expr %q not found", expr)
		}
		curMap = next
	}
	// Try string
	var s string
	if err := json.Unmarshal(curRaw, &s); err == nil {
		return s, nil
	}
	var f float64
	if err := json.Unmarshal(curRaw, &f); err == nil {
		return fmt.Sprintf("%g", f), nil
	}
	// Fallback: return raw JSON
	return string(curRaw), nil
}

func splitDot(s string) []string {
	var out []string
	start := 0
	for i, c := range s {
		if c == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// BuildFromCapsule builds a profile from capsule expressions; fails if any expression missing.
// Dimensions are produced in sorted key order for determinism.
func BuildFromCapsule(identity, protocolVersion, routeScope string, capsulePath string, exprs map[string]string) (Profile, error) {
	keys := make([]string, 0, len(exprs))
	for k := range exprs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var dims []Dimension
	for _, id := range keys {
		expr := exprs[id]
		val, err := EvaluateExpression(capsulePath, expr)
		if err != nil {
			return Profile{}, err
		}
		metric := val
		dims = append(dims, Dimension{
			ID:            id,
			Status:        StatusMeasured,
			Metric:        &metric,
			Scope:         routeScope,
			EvidenceLevel: "E1",
			CapsuleExpr:   expr,
			Denominator:   1,
			SampleUnit:    "task",
		})
	}
	return Build(identity, protocolVersion, routeScope, dims)
}
