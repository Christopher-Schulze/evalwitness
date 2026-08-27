package stress

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestHeldOutPartitionLockFreezesExactTestCellsAndRejectsNotRunSeal(t *testing.T) {
	corpusPlan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(corpusPlan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	replayed := currentV3Replay(t, corpusPlan, audit, release, registry)
	armPlan, err := BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	design, err := BuildStressAnalysisDesign(armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := BuildHeldOutPartitionLock(design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	if lock.TestCases != 57 || lock.TestCells != 1140 || lock.SupportedTestCells != 440 ||
		lock.UnsupportedTestCells != 700 || lock.SourceCatalogVersion != RelationCatalogVersionV3 ||
		lock.NextCatalogVersion != "evalwitness.controlled-corruption-stress-catalog.v2" {
		t.Fatalf("held-out partition lock = %+v", lock)
	}
	armReport, err := BuildArmComparisonReport(armPlan, registry, replayed, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := BuildStressAnalysisReport(design, armPlan, armReport, registry, replayed, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = SealHeldOutRun(nil, lock, design, armPlan, armReport, analysis, registry, replayed, nil, nil, nil, nil)
	if err == nil || !containsError(err, "was not executed exactly once") {
		t.Fatalf("not-run held-out seal error = %v", err)
	}

	tampered := lock
	tampered.TestCellIDs = append([]string(nil), lock.TestCellIDs[1:]...)
	tampered.TestCells--
	tampered.Digest, err = heldOutPartitionLockDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.ValidateAgainst(design, armPlan, registry, replayed); err == nil {
		t.Fatal("held-out partition lock accepted one omitted test cell")
	}
}

func TestHeldOutRunSealsOnceAndRoutesViolationsOnlyToNextCatalog(t *testing.T) {
	serveStressProtocolAdapterHelper()
	corpusPlan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(corpusPlan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	replayed := currentV3Replay(t, corpusPlan, audit, release, registry)
	armPlan, err := BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	design, err := BuildStressAnalysisDesign(armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := BuildHeldOutPartitionLock(design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	proof, _, _, _ := protocolAdapterProofFixture(t)
	replayEvidence, zeroCostEvidence := executeHeldOutFixture(t, armPlan, registry, replayed, proof)
	armReport, err := BuildArmComparisonReport(armPlan, registry, replayed, replayEvidence, zeroCostEvidence, &proof)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := BuildStressAnalysisReport(
		design, armPlan, armReport, registry, replayed, replayEvidence, zeroCostEvidence, &proof, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	seal, err := SealHeldOutRun(
		nil, lock, design, armPlan, armReport, analysis, registry, replayed,
		replayEvidence, zeroCostEvidence, &proof, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if seal.RunCount != 1 || seal.Reopened || !seal.Completed || seal.ExecutedTestCells != 440 ||
		seal.ExecutedTestCells != seal.SupportedTestCells || seal.ViolatedTestCells == 0 ||
		seal.WitnessesBound+seal.WitnessesMissing != seal.ViolatedTestCells {
		t.Fatalf("held-out run seal = %+v", seal)
	}
	_, err = SealHeldOutRun(
		[]HeldOutRunSeal{seal}, lock, design, armPlan, armReport, analysis, registry, replayed,
		replayEvidence, zeroCostEvidence, &proof, nil,
	)
	var admissionErr *AdmissionError
	if !errors.As(err, &admissionErr) || admissionErr.State != InvalidLockedPartitionUsed {
		t.Fatalf("second held-out run error = %v", err)
	}

	violated := heldOutViolatedCell(t, lock, armReport)
	source := relationByID(t, registry, violated.RelationID)
	candidate := source
	candidate.ID = source.ID + ".next-catalog"
	candidate.Revision++
	candidate.Digest = ""
	candidate, err = SealRelation(candidate)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := RouteHeldOutDiscoveries(lock, seal, registry, armReport, analysis, []RelationDiscovery{{
		SourceCellID: violated.CellID, DiscoveryEvidenceDigest: digestText("held-out-discovery-evidence"), Candidate: candidate,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if ledger.DiscoveryCount != 1 || ledger.TargetCatalogVersion != lock.NextCatalogVersion ||
		!ledger.CurrentCatalogFrozen || ledger.TestPartitionRetrofitted ||
		ledger.Discoveries[0].Candidate.Digest != candidate.Digest || ledger.Discoveries[0].SourceResultDigest != violated.ResultDigest {
		t.Fatalf("next-version discovery ledger = %+v", ledger)
	}
	_, err = RouteHeldOutDiscoveries(lock, seal, registry, armReport, analysis, []RelationDiscovery{{
		SourceCellID: violated.CellID, DiscoveryEvidenceDigest: digestText("retrofit-evidence"), Candidate: source,
	}})
	if err == nil || !containsError(err, "frozen catalog") {
		t.Fatalf("current-catalog retrofit error = %v", err)
	}
}

func executeHeldOutFixture(
	t *testing.T,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	proof ProtocolAdapterProof,
) ([]ArmReplayEvidence, []ZeroCostExecution) {
	t.Helper()
	judgeRunner, err := NewReplayFirstRunner(stressVerificationService(t, provider.ReplayStatusExact))
	if err != nil {
		t.Fatal(err)
	}
	verifierRunner, err := NewReplayFirstRunner(firstPositionVerificationService(t))
	if err != nil {
		t.Fatal(err)
	}
	protocolRunner, err := NewReplayFirstRunner(firstPositionVerificationService(t))
	if err != nil {
		t.Fatal(err)
	}
	runners := map[string]*ReplayFirstRunner{
		"explicit-text-judge":       judgeRunner,
		"score-token-verifier":      verifierRunner,
		"external-protocol-adapter": protocolRunner,
	}
	var replayEvidence []ArmReplayEvidence
	var zeroCostEvidence []ZeroCostExecution
	for _, item := range replayed {
		if item.Split != study.RoleTest {
			continue
		}
		for _, relationID := range item.RelationIDs {
			relation := relationByID(t, registry, relationID)
			admission := formalAdmission(t, item.CaseID)
			if relation.StatisticalFamily.Estimand == EstimandPrimaryCore {
				admission = admissionWithStatus(t, admission, AdmissionHumanSupported, true, true)
			}
			for _, arm := range plan.Arms {
				support, _ := armSupportForRelation(arm, relation)
				if support != ArmSupported {
					continue
				}
				if arm.Kind == ArmZeroCostControl {
					execution, runErr := RunZeroCostArm(relation, admission, item, arm.ID)
					if runErr != nil {
						t.Fatalf("zero-cost held-out cell %s/%s/%s: %v", arm.ID, relation.ID, item.CaseID, runErr)
					}
					zeroCostEvidence = append(zeroCostEvidence, execution)
					continue
				}
				runner := runners[arm.ID]
				if runner == nil {
					t.Fatalf("no held-out fixture runner for arm %q", arm.ID)
				}
				original := corpusVerificationInput(relation, admission, arm.Entrypoint, item.Original)
				transformed := corpusVerificationInput(relation, admission, arm.Entrypoint, item.Transformed)
				original.Policy.Evidence = arm.EvidencePolicy
				transformed.Policy.Evidence = arm.EvidencePolicy
				execution, runErr := runner.RunMutation(context.Background(), ReplayRunRequest{
					Relation: relation, Admission: admission, Original: original, Transformed: transformed,
				})
				if runErr != nil {
					t.Fatalf("model held-out cell %s/%s/%s: %v", arm.ID, relation.ID, item.CaseID, runErr)
				}
				var requiredProof *ProtocolAdapterProof
				if arm.Kind == ArmProtocolAdapter {
					requiredProof = &proof
				}
				sealed, runErr := SealArmReplayEvidence(plan, relation, admission, item, execution, requiredProof)
				if runErr != nil {
					t.Fatalf("seal held-out cell %s/%s/%s: %v", arm.ID, relation.ID, item.CaseID, runErr)
				}
				replayEvidence = append(replayEvidence, sealed)
			}
		}
	}
	return replayEvidence, zeroCostEvidence
}

func heldOutViolatedCell(t *testing.T, lock HeldOutPartitionLock, report ArmComparisonReport) ArmComparisonObservation {
	t.Helper()
	testCells := stringSet(lock.SupportedTestCellIDs)
	for _, cell := range report.Cells {
		if _, exists := testCells[cell.CellID]; exists && cell.Status == ArmCellExecuted && cell.Outcome == OutcomeViolated {
			return cell
		}
	}
	t.Fatal("held-out fixture produced no violated cell")
	return ArmComparisonObservation{}
}

func containsError(err error, fragment string) bool {
	return err != nil && strings.Contains(err.Error(), fragment)
}
