package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/calibration"
)

func TestCalibrationEvaluateToolUsesTestSplit(t *testing.T) {
	trueVal := true
	falseVal := false
	raw, err := json.Marshal(map[string]any{
		"threshold":    0.5,
		"target_risk":  0.9,
		"min_coverage": 0.1,
		"seed":         42,
		"observations": []map[string]any{
			{"id": "1", "task_id": "t1", "split_role": "test", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.95, "won": trueVal},
			{"id": "2", "task_id": "t1", "split_role": "test", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.92, "won": trueVal},
			{"id": "3", "task_id": "t2", "split_role": "test", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.91, "won": trueVal},
			{"id": "4", "task_id": "t2", "split_role": "test", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.90, "won": falseVal},
			{"id": "5", "task_id": "t3", "split_role": "test", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.88, "won": trueVal},
			{"id": "6", "task_id": "t3", "split_role": "test", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.87, "won": trueVal},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := (&ToolHandler{}).Call(context.Background(), ToolCalibrationEvaluate, raw)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, ok := got.(calibration.DeploymentEvaluation)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if evaluation.SchemaVersion != calibration.DeploymentEvaluationSchemaVersion || evaluation.Digest == "" {
		t.Fatalf("evaluation = %+v", evaluation)
	}
}

func TestCalibrationEvaluateToolRejectsNonTestSplit(t *testing.T) {
	trueVal := true
	raw, err := json.Marshal(map[string]any{
		"threshold":    0.5,
		"target_risk":  0.9,
		"min_coverage": 0.1,
		"observations": []map[string]any{
			{"id": "1", "task_id": "t1", "split_role": "calibration", "conditional_diff": 0, "min_valid_mass": 0, "mean_valid_mass": 0, "visible_mass": 0, "missing_mass": 0, "conditional_variance": 0, "order_effect": 0, "repeat_dispersion": 0, "support_count": 0, "top_k": 20, "evidence_budget": 0, "retention": 0, "predicted": 0.9, "won": trueVal},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&ToolHandler{}).Call(context.Background(), ToolCalibrationEvaluate, raw); err == nil {
		t.Fatal("calibration-split observations accepted")
	}
}
