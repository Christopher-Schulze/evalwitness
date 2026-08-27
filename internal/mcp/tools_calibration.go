package mcp

import (
	"encoding/json"

	"github.com/Christopher-Schulze/evalwitness/internal/calibration"
)

type calibrationEvaluateArgs struct {
	Observations []calibration.Observation `json:"observations"`
	Threshold    *float64                  `json:"threshold"`
	TargetRisk   *float64                  `json:"target_risk"`
	MinCoverage  *float64                  `json:"min_coverage"`
	Seed         uint64                    `json:"seed"`
}

func calibrationEvaluateTool() Tool {
	return Tool{
		Name:        ToolCalibrationEvaluate,
		Description: "Evaluate held-out selective risk/coverage and IsDeployable on a test-split observation array. Offline only. Not a locked TASK 049 study, not a deployable production policy, and not calibrated confidence for pairwise/absolute/delta.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"observations": map[string]any{
					"type":        "array",
					"minItems":    1,
					"description": "calibration.Observation rows; every split_role must be test",
					"items":       map[string]any{"type": "object"},
				},
				"threshold":    map[string]any{"type": "number", "description": "selection threshold on predicted probability"},
				"target_risk":  map[string]any{"type": "number", "description": "maximum selective-risk upper bound"},
				"min_coverage": map[string]any{"type": "number", "description": "minimum coverage"},
				"seed":         map[string]any{"type": "integer", "minimum": 1, "description": "bootstrap seed; default 1"},
			},
			"required": []string{"observations", "threshold", "target_risk", "min_coverage"},
		},
	}
}

func (h *ToolHandler) callCalibrationEvaluate(raw json.RawMessage) (any, error) {
	var args calibrationEvaluateArgs
	if err := decodeToolArguments(raw, &args); err != nil {
		return nil, &ToolError{Code: -32602, Message: "invalid params", Data: err.Error()}
	}
	if args.Threshold == nil || args.TargetRisk == nil || args.MinCoverage == nil || len(args.Observations) == 0 {
		return nil, &ToolError{Code: -32602, Message: "observations, threshold, target_risk, and min_coverage are required"}
	}
	if args.Seed == 0 {
		args.Seed = 1
	}
	evaluation, err := calibration.EvaluateDeployment(args.Observations, *args.Threshold, *args.TargetRisk, *args.MinCoverage, args.Seed)
	if err != nil {
		return nil, &ToolError{Code: -32602, Message: "invalid params", Data: err.Error()}
	}
	return evaluation, nil
}
