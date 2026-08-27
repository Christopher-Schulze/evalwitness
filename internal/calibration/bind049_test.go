package calibration

import "testing"

func TestBind049LifecyclePinsCommittedSplitAndRejectsTestRole(t *testing.T) {
	schema := FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}}
	lifecycle, index, err := Bind049Lifecycle(
		"../../eval/governance/identical-response-frozen-split-v1.json",
		"../../eval/governance/identical-response-study-record-v5.json",
		schema,
	)
	if err != nil {
		t.Fatal(err)
	}
	if lifecycle.SplitDigest == "" || lifecycle.StudyDigest == "" || lifecycle.SplitDigest != index.SplitDigest {
		t.Fatalf("lifecycle = %+v index = %+v", lifecycle, index)
	}
	if index.HasTestRole {
		t.Fatal("frozen split must not contain a confirmatory test role")
	}
	if err := index.ValidateObservations([]Observation{{
		ID: "x", TaskID: "missing-task", SplitRole: RoleCalibration,
	}}, RoleCalibration); err == nil {
		t.Fatal("unknown task accepted")
	}
	if err := index.ValidateObservations([]Observation{{
		ID: "x", TaskID: "django__django-17087", SplitRole: RoleTest, Predicted: 0.9,
	}}, RoleTest); err == nil {
		t.Fatal("test-role evaluation accepted against a development/calibration freeze")
	}
	if err := index.ValidateObservations([]Observation{{
		ID: "cal", TaskID: "fix-git", SplitRole: RoleCalibration,
	}}, RoleCalibration); err != nil {
		t.Fatal(err)
	}
}
