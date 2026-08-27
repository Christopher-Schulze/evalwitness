package calibration

import "testing"

func TestEvaluateFiniteSampleRejectsClusteredTasks(t *testing.T) {
	won := true
	lost := false
	clustered := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.9, Won: &won},
		{ID: "2", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.8, Won: &lost},
		{ID: "3", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.7, Won: &won},
	}
	control := EvaluateFiniteSample(clustered, 0.5, func(o Observation) float64 { return o.Predicted })
	if control.Status != FiniteSampleUnsupported || control.RepeatedTasks != 1 {
		t.Fatalf("clustered control = %+v", control)
	}
}

func TestEvaluateFiniteSampleBoundOnUniqueTasks(t *testing.T) {
	won := true
	lost := false
	unique := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.9, Won: &won},
		{ID: "2", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.8, Won: &lost},
		{ID: "3", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.7, Won: &won},
	}
	control := EvaluateFiniteSample(unique, 0.5, func(o Observation) float64 { return o.Predicted })
	if control.Status != FiniteSampleEvaluated || control.UpperBound == nil || *control.UpperBound < 1.0/3.0 {
		t.Fatalf("unique control = %+v", control)
	}
}

func TestEvaluateDeploymentSurfacesUnsupportedFiniteSample(t *testing.T) {
	won := true
	lost := false
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.95, Won: &won},
		{ID: "2", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.92, Won: &won},
		{ID: "3", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.91, Won: &won},
		{ID: "4", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.90, Won: &lost},
		{ID: "5", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.88, Won: &won},
		{ID: "6", TaskID: "t3", SplitRole: RoleTest, Predicted: 0.87, Won: &won},
	}
	evaluation, err := EvaluateDeployment(obs, 0.5, 0.9, 0.1, 42)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.FiniteSample.Status != FiniteSampleUnsupported {
		t.Fatalf("finite sample = %+v", evaluation.FiniteSample)
	}
}
