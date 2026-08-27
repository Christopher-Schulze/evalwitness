package calibration

import "testing"

func TestEvaluateDeploymentUsesHeldOutIsDeployable(t *testing.T) {
	trueVal := true
	falseVal := false
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.95, Won: &trueVal},
		{ID: "2", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.92, Won: &trueVal},
		{ID: "3", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.91, Won: &trueVal},
		{ID: "4", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.90, Won: &falseVal},
		{ID: "5", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.88, Won: &trueVal},
		{ID: "6", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.87, Won: &trueVal},
	}
	evaluation, err := EvaluateDeployment(obs, 0.5, 0.9, 0.1, 42)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Digest == "" || evaluation.Selective.Coverage == 0 {
		t.Fatalf("evaluation = %+v", evaluation)
	}
	if evaluation.Deployable != IsDeployable(evaluation.Selective, 0.9, 0.1) {
		t.Fatalf("deployable drifted from IsDeployable: %+v", evaluation)
	}
	if _, err := EvaluateDeployment(obs, 0.5, 0.9, 0.1, 42); err != nil {
		t.Fatal(err)
	}
	scoped, err := EvaluateDeploymentScoped(obs, 0.5, 0.9, 0.1, 42, ModelArtifact{
		RouteScope: "route-a", DomainScope: "terminal",
	}, "route-b", "terminal")
	if err != nil {
		t.Fatal(err)
	}
	if scoped.Deployable || scoped.Status != DeploymentStatusUnsupported || scoped.Applicability == nil || scoped.Applicability.Applicable {
		t.Fatalf("scope mismatch must be unsupported: %+v", scoped)
	}
}
