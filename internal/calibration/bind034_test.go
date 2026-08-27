package calibration

import "testing"

func TestBind034DevelopmentFreezesHistoricalTasks(t *testing.T) {
	report, err := Bind034Development("../..", "../../eval/governance/development-inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.TaskCount != 589 || report.ConfirmationPermitted || report.Role != RoleDevelopment || report.Digest == "" {
		t.Fatalf("report = %+v", report)
	}
	if err := report.RejectConfirmatory([]Observation{{
		ID: "dev", TaskID: "adaptive-rejection-sampler", SplitRole: RoleDevelopment,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := report.RejectConfirmatory([]Observation{{
		ID: "leak", TaskID: "adaptive-rejection-sampler", SplitRole: RoleTest,
	}}); err == nil {
		t.Fatal("034 task accepted as confirmatory test")
	}
}

func TestGuard034DevelopmentBlocksEvaluateLeakage(t *testing.T) {
	obs := []Observation{{ID: "1", TaskID: "adaptive-rejection-sampler", SplitRole: RoleTest}}
	if err := Guard034Development("../..", "../../eval/governance/development-inventory.json", obs); err == nil {
		t.Fatal("evaluate leakage of a TASK 034 task was accepted")
	}
	if err := Guard034Development("../..", "", obs); err != nil {
		t.Fatal(err)
	}
}
