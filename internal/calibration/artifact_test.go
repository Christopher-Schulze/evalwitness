package calibration

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func testTrainingDigest() string {
	sum := sha256.Sum256([]byte("training-manifest"))
	return hex.EncodeToString(sum[:])
}

func TestSealModelArtifactRejectsLeakageFeatures(t *testing.T) {
	_, err := SealModelArtifact(ModelArtifact{
		ModelType:      "platt",
		FeatureSchema:  FeatureSchema{Version: "v1", Keys: []string{"conditional_diff", "task_id"}},
		TrainingDigest: testTrainingDigest(),
		RouteScope:     "route-a",
		DomainScope:    "terminal",
	}, PlattParams{A: 0, B: 1})
	if err == nil {
		t.Fatal("task_id feature accepted")
	}
}

func TestSealAndVerifyModelArtifactDetectsCorruption(t *testing.T) {
	artifact, err := SealModelArtifact(ModelArtifact{
		ModelType:      "platt",
		FeatureSchema:  FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}},
		TrainingDigest: testTrainingDigest(),
		RouteScope:     "route-a",
		DomainScope:    "terminal",
	}, PlattParams{A: -0.2, B: 1.1})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyModelArtifact(artifact, PlattParams{A: -0.2, B: 1.1}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyModelArtifact(artifact, PlattParams{A: 0, B: 1}); err == nil {
		t.Fatal("corrupted calibrator accepted")
	}
	raw := []byte(`{"a":-0.2,"b":1.1}`)
	sealedBytes, err := SealModelArtifactBytes(ModelArtifact{
		ModelType:      "platt",
		FeatureSchema:  FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}},
		TrainingDigest: testTrainingDigest(),
		RouteScope:     "route-a",
		DomainScope:    "terminal",
	}, raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyModelArtifactBytes(sealedBytes, raw); err != nil {
		t.Fatal(err)
	}
	if err := VerifyModelArtifactBytes(sealedBytes, []byte(`{"a":0,"b":1}`)); err == nil {
		t.Fatal("byte calibrator drift accepted")
	}
}

func TestMatchApplicabilityRejectsRouteAndDomainMismatch(t *testing.T) {
	artifact := ModelArtifact{RouteScope: "route-a", DomainScope: "terminal"}
	if got := MatchApplicability(artifact, "route-b", "terminal"); got.Applicable || got.Reason != "route_mismatch" {
		t.Fatalf("route = %+v", got)
	}
	if got := MatchApplicability(artifact, "route-a", "swebench"); got.Applicable || got.Reason != "domain_mismatch" {
		t.Fatalf("domain = %+v", got)
	}
	if got := MatchApplicability(artifact, "route-a", "terminal"); !got.Applicable {
		t.Fatalf("match = %+v", got)
	}
}

func boolPtr(v bool) *bool { return &v }

func TestFitPlattLabelPermutationChangesChecksum(t *testing.T) {
	obs := []Observation{
		{ID: "1", TaskID: "t1", SplitRole: RoleCalibration, ConditionalDiff: -2, Predicted: 0.1, Won: boolPtr(false)},
		{ID: "2", TaskID: "t1", SplitRole: RoleCalibration, ConditionalDiff: -1, Predicted: 0.2, Won: boolPtr(false)},
		{ID: "3", TaskID: "t2", SplitRole: RoleCalibration, ConditionalDiff: 0, Predicted: 0.5, Won: boolPtr(true)},
		{ID: "4", TaskID: "t2", SplitRole: RoleCalibration, ConditionalDiff: 1, Predicted: 0.8, Won: boolPtr(true)},
		{ID: "5", TaskID: "t3", SplitRole: RoleCalibration, ConditionalDiff: 2, Predicted: 0.9, Won: boolPtr(true)},
		{ID: "6", TaskID: "t3", SplitRole: RoleCalibration, ConditionalDiff: -0.5, Predicted: 0.3, Won: boolPtr(false)},
		{ID: "7", TaskID: "t4", SplitRole: RoleCalibration, ConditionalDiff: 0.5, Predicted: 0.6, Won: boolPtr(true)},
		{ID: "8", TaskID: "t4", SplitRole: RoleCalibration, ConditionalDiff: -1.5, Predicted: 0.15, Won: boolPtr(false)},
		{ID: "9", TaskID: "t5", SplitRole: RoleCalibration, ConditionalDiff: 1.5, Predicted: 0.85, Won: boolPtr(true)},
		{ID: "10", TaskID: "t5", SplitRole: RoleCalibration, ConditionalDiff: 0.2, Predicted: 0.55, Won: boolPtr(true)},
		{ID: "11", TaskID: "t6", SplitRole: RoleCalibration, ConditionalDiff: -0.2, Predicted: 0.45, Won: boolPtr(false)},
	}
	original, err := FitPlatt(obs)
	if err != nil {
		t.Fatal(err)
	}
	permuted := append([]Observation(nil), obs...)
	for i := range permuted {
		if permuted[i].Won == nil {
			continue
		}
		flipped := !*permuted[i].Won
		permuted[i].Won = &flipped
	}
	changed, err := FitPlatt(permuted)
	if err != nil {
		t.Fatal(err)
	}
	left, err := SealModelArtifact(ModelArtifact{
		ModelType: "platt", FeatureSchema: FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}},
		TrainingDigest: testTrainingDigest(), RouteScope: "route-a", DomainScope: "terminal",
	}, original)
	if err != nil {
		t.Fatal(err)
	}
	right, err := SealModelArtifact(ModelArtifact{
		ModelType: "platt", FeatureSchema: FeatureSchema{Version: "v1", Keys: []string{"conditional_diff"}},
		TrainingDigest: testTrainingDigest(), RouteScope: "route-a", DomainScope: "terminal",
	}, changed)
	if err != nil {
		t.Fatal(err)
	}
	if left.CalibratorDigest == right.CalibratorDigest {
		t.Fatal("label permutation produced the same calibrator checksum")
	}
}
