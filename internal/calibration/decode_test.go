package calibration

import (
	"strings"
	"testing"
)

func TestDecodeObservationsAcceptsPredictedFloats(t *testing.T) {
	raw := []byte(`[{"id":"1","task_id":"t1","split_role":"test","predicted":0.95,"won":true}]`)
	observations, err := DecodeObservations(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 || observations[0].Predicted != 0.95 || observations[0].Won == nil || !*observations[0].Won {
		t.Fatalf("observations = %+v", observations)
	}
}

func TestDecodeObservationsRejectsUnknownFieldsAndTrailing(t *testing.T) {
	if _, err := DecodeObservations([]byte(`[{"id":"1","task_id":"t1","split_role":"test","predicted":0.9,"leak":1}]`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := DecodeObservations([]byte(`[{"id":"1","task_id":"t1","split_role":"test","predicted":0.9}] {}`)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestDecodeModelArtifactAcceptsMetricFloats(t *testing.T) {
	raw := []byte(`{"schema_version":"evalwitness.calibration.v1","model_type":"platt","feature_schema":{"version":"v1","keys":["conditional_diff"]},"training_manifest_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","calibrator_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","route_scope":"route-a","domain_scope":"terminal","metrics":{"ece":{"value":0.195},"mce":{"value":0.4},"brier":{"value":0.2},"auc":{"value":0.8},"accuracy":{"value":0.7}}}`)
	artifact, err := DecodeModelArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Metrics == nil || artifact.Metrics.ECE.Value != 0.195 {
		t.Fatalf("artifact = %+v", artifact)
	}
	encoded, err := EncodeModelArtifact(artifact)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := DecodeModelArtifact(encoded)
	if err != nil || roundTrip.Metrics == nil || roundTrip.Metrics.ECE.Value != 0.195 {
		t.Fatalf("round-trip = %+v err=%v", roundTrip, err)
	}
	if _, err := DecodeModelArtifact([]byte(`{"route_scope":"x","unknown":1}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestEncodeDeploymentEvaluationAllowsFloats(t *testing.T) {
	trueVal := true
	evaluation, err := EvaluateDeployment([]Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleTest, Predicted: 0.95, Won: &trueVal},
		{ID: "2", TaskID: "t2", SplitRole: RoleTest, Predicted: 0.2, Won: &trueVal},
	}, 0.5, 0.9, 0.1, 1)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeDeploymentEvaluation(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"predicted"`) && !strings.Contains(string(raw), "selective") {
		t.Fatalf("encoded = %s", raw)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Fatal("missing trailing newline")
	}
}
