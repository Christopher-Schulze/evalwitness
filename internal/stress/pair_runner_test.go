package stress

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

func TestReplayFirstRunnerBuildsGenericPairEvidence(t *testing.T) {
	runner := exactReplayPairRunner(t)
	request := genericReplayPairRequest(t)
	evidence, err := runner.RunPairEvidence(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := evidence.ValidateRequest(request); err != nil {
		t.Fatal(err)
	}
	if evidence.SchemaVersion != ReplayPairEvidenceSchemaVersion || evidence.Digest == "" ||
		evidence.Original.PlanFingerprint == evidence.Transformed.PlanFingerprint ||
		evidence.ReplaySource.SchemaVersion != provider.ExactReplaySourceSchemaVersion {
		t.Fatalf("generic replay pair evidence is incomplete: %+v", evidence)
	}
}

func TestReplayFirstRunnerAllowsStudyControlWithIdenticalTrajectory(t *testing.T) {
	runner := exactReplayPairRunner(t)
	request := genericReplayPairRequest(t)
	request.Transformed.Trajectories = append([]string(nil), request.Original.Trajectories...)
	evidence, err := runner.RunPairEvidence(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.OriginalInputDigest == evidence.TransformedInputDigest ||
		evidence.Original.PlanFingerprint == evidence.Transformed.PlanFingerprint {
		t.Fatal("study variants did not preserve distinct control observations")
	}
}

func TestReplayPairEvidenceRejectsRequestedInputSubstitution(t *testing.T) {
	request := genericReplayPairRequest(t)
	evidence, err := exactReplayPairRunner(t).RunPairEvidence(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Transformed.Trajectories[0] = "substituted after execution"
	if err := evidence.ValidateRequest(request); err == nil || !strings.Contains(err.Error(), "does not bind") {
		t.Fatalf("substituted replay request error = %v", err)
	}
}

func TestReplayFirstRunnerMutationPreservesGenericPairEvidence(t *testing.T) {
	relation := testRelation(t, mutation.FamilyNeutralFormatting, EstimandSensitivity)
	admission := formalAdmission(t, "mutation-pair-parity")
	request := ReplayRunRequest{
		Relation: relation, Admission: admission,
		Original:    stressVerificationInput(relation, admission, "original trajectory"),
		Transformed: stressVerificationInput(relation, admission, "transformed trajectory"),
	}
	runner := exactReplayPairRunner(t)
	pair, err := runner.RunPairEvidence(context.Background(), ReplayPairRequest{request.Original, request.Transformed})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := runner.RunMutation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if pair.BatchRunFingerprint != execution.BatchRunFingerprint || pair.ReplaySource != execution.ReplaySource ||
		!reflect.DeepEqual(comparableReplaySide(pair.Original), comparableReplaySide(execution.Original)) ||
		!reflect.DeepEqual(comparableReplaySide(pair.Transformed), comparableReplaySide(execution.Transformed)) {
		t.Fatal("relation replay no longer preserves the shared generic pair evidence")
	}
}

func TestReplayFirstRunnerRejectsGenericPairControlPlaneMismatch(t *testing.T) {
	runner := exactReplayPairRunner(t)
	request := genericReplayPairRequest(t)
	request.Transformed.Task = "substituted verification task"
	_, err := runner.RunPairEvidence(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "outside trajectories and study variant") {
		t.Fatalf("generic replay control-plane mismatch error = %v", err)
	}
}

func TestReplayFirstRunnerRejectsGenericPairLineageSubstitution(t *testing.T) {
	runner := exactReplayPairRunner(t)
	request := genericReplayPairRequest(t)
	request.Transformed.Lineage.StudyCellID = "substituted-cell"
	_, err := runner.RunPairEvidence(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "outside trajectories and study variant") {
		t.Fatalf("generic replay lineage substitution error = %v", err)
	}
}

func TestReplayFirstRunnerRejectsGenericPairWithoutExactCaptureSource(t *testing.T) {
	runner, err := NewReplayFirstRunner(stressVerificationServiceWithProvider(t, &sourceLessStressReplayProvider{}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.RunPairEvidence(context.Background(), genericReplayPairRequest(t))
	if err == nil || !strings.Contains(err.Error(), "lacks validated capture provenance") {
		t.Fatalf("generic source-less replay error = %v", err)
	}
}

func exactReplayPairRunner(t *testing.T) *ReplayFirstRunner {
	t.Helper()
	runner, err := NewReplayFirstRunner(stressVerificationService(t, provider.ReplayStatusExact))
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func comparableReplaySide(value ReplaySide) ReplaySide {
	value.Result.Budget.ElapsedSeconds = 0
	return value
}

func genericReplayPairRequest(t *testing.T) ReplayPairRequest {
	t.Helper()
	relation := testRelation(t, mutation.FamilyNeutralFormatting, EstimandSensitivity)
	admission := formalAdmission(t, "generic-pair-evidence")
	original := stressVerificationInput(relation, admission, "original trajectory")
	transformed := stressVerificationInput(relation, admission, "transformed trajectory")
	original.StudyManifestDigest = digestText("generic-pair-study")
	transformed.StudyManifestDigest = original.StudyManifestDigest
	original.StudyVariant, transformed.StudyVariant = "original", "transformed"
	original.Lineage.StudyCellID, transformed.Lineage.StudyCellID = "cell-001", "cell-001"
	return ReplayPairRequest{Original: original, Transformed: transformed}
}
