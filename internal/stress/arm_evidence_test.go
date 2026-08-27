package stress

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const protocolAdapterHelperArgument = "--evalwitness-stress-protocol-adapter-helper"

func TestProtocolAdapterProofBindsDirectSubprocessAndMappingConformance(t *testing.T) {
	serveStressProtocolAdapterHelper()
	proof, reference, external, mapping := protocolAdapterProofFixture(t)
	if proof.ProviderCalls != 0 || proof.EmpiricalReliability || !proof.ResultParity ||
		proof.ReferenceRunDigest != reference.RunDigest || proof.ExternalProcessRunDigest != external.RunDigest ||
		proof.LineageAdapterConformanceDigest != mapping.Digest {
		t.Fatalf("protocol adapter proof = %+v", proof)
	}
	tampered := proof
	tampered.EmpiricalReliability = true
	digest, err := protocolAdapterProofDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	tampered.Digest = digest
	if err := tampered.ValidateAgainst(reference, external, mapping); err == nil {
		t.Fatal("provider-free protocol conformance was promoted to empirical reliability")
	}
}

func TestModelAndProtocolArmsShareOneRelationCellContract(t *testing.T) {
	serveStressProtocolAdapterHelper()
	plan, audit, release := currentCorpusV3(t)
	registry, err := BuildV3RelationRegistry(plan, audit, release)
	if err != nil {
		t.Fatal(err)
	}
	replayed := currentV3Replay(t, plan, audit, release, registry)
	comparisonPlan, err := BuildArmComparisonPlan(registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	relationID := catalogRelationID(mutation.FamilyNeutralFormatting, EstimandSensitivity)
	relation := relationByID(t, registry, relationID)
	item := replayedCaseByRelation(t, replayed, relationID)
	admission := formalAdmission(t, item.CaseID)
	proof, _, _, _ := protocolAdapterProofFixture(t)

	tests := []struct {
		name       string
		entrypoint string
		policy     verification.EvidencePolicy
		service    VerificationExecutor
		proof      *ProtocolAdapterProof
	}{
		{name: "explicit text judge", entrypoint: "cli.verify", policy: verification.EvidenceExplicitJudge, service: stressVerificationService(t, provider.ReplayStatusExact)},
		{name: "score-token verifier", entrypoint: "cli.verify", policy: verification.EvidenceStrictVerifier, service: firstPositionVerificationService(t)},
		{name: "protocol adapter", entrypoint: "protocol.application", policy: verification.EvidenceStrictVerifier, service: firstPositionVerificationService(t), proof: &proof},
	}
	evidence := make([]ArmReplayEvidence, 0, len(tests))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner, runErr := NewReplayFirstRunner(test.service)
			if runErr != nil {
				t.Fatal(runErr)
			}
			original := corpusVerificationInput(relation, admission, test.entrypoint, item.Original)
			transformed := corpusVerificationInput(relation, admission, test.entrypoint, item.Transformed)
			original.Policy.Evidence, transformed.Policy.Evidence = test.policy, test.policy
			execution, runErr := runner.RunMutation(context.Background(), ReplayRunRequest{
				Relation: relation, Admission: admission, Original: original, Transformed: transformed,
			})
			if runErr != nil {
				t.Fatal(runErr)
			}
			sealed, runErr := SealArmReplayEvidence(comparisonPlan, relation, admission, item, execution, test.proof)
			if runErr != nil {
				t.Fatal(runErr)
			}
			if sealed.Result.Outcome != OutcomeSatisfied || sealed.Result.CompletedRepetitions != catalogRepetitions {
				t.Fatalf("arm replay evidence = %+v", sealed)
			}
			evidence = append(evidence, sealed)
		})
	}
	if len(evidence) != 3 || evidence[0].CellID == evidence[1].CellID || evidence[1].CellID == evidence[2].CellID ||
		evidence[0].Result.RelationDigest != evidence[1].Result.RelationDigest || evidence[1].Result.RelationDigest != evidence[2].Result.RelationDigest ||
		evidence[0].Result.CaseID != evidence[1].Result.CaseID || evidence[1].Result.CaseID != evidence[2].Result.CaseID {
		t.Fatalf("arm evidence does not share one relation cell contract: %+v", evidence)
	}
	if !reflect.DeepEqual(evidence[1].Result.ConstraintResults, evidence[2].Result.ConstraintResults) {
		t.Fatal("strict verifier and protocol application arm changed relation semantics")
	}
	report, err := BuildArmComparisonReport(comparisonPlan, registry, replayed, evidence, nil, &proof)
	if err != nil {
		t.Fatal(err)
	}
	if report.PlannedCells != 5630 || report.ExecutedCells != 3 || report.NotRunCells != 2246 ||
		report.UnsupportedCells != 3381 || report.GlobalScore {
		t.Fatalf("model and protocol arm report accounting = %+v", report)
	}
	duplicate := append(append([]ArmReplayEvidence(nil), evidence...), evidence[0])
	if _, err := BuildArmComparisonReport(comparisonPlan, registry, replayed, duplicate, nil, &proof); err == nil {
		t.Fatal("arm-comparison report accepted duplicate cell evidence")
	}

	if _, err := SealArmReplayEvidence(comparisonPlan, relation, admission, item, evidence[1].Execution, &proof); err == nil {
		t.Fatal("non-protocol arm accepted protocol-only evidence")
	}
}

func protocolAdapterProofFixture(t *testing.T) (ProtocolAdapterProof, protocolkit.AuditRun, protocolkit.AuditRun, lineage.AdapterConformanceReport) {
	t.Helper()
	requestCorpus, err := os.ReadFile("../provider/testdata/request-fingerprint-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := protocolkit.LoadNormativeCorpus(requestCorpus)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := protocolkit.RunReferenceCorpus(corpus)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	external, err := protocolkit.RunAdapterCorpus(ctx, protocolkit.AdapterCommand{
		Path: os.Args[0], Arguments: []string{"-test.run=" + t.Name(), "--", protocolAdapterHelperArgument},
	}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := lineage.BuildAdapterConformanceReport()
	if err != nil {
		t.Fatal(err)
	}
	proof, err := BuildProtocolAdapterProof(reference, external, mapping)
	if err != nil {
		t.Fatal(err)
	}
	return proof, reference, external, mapping
}

func serveStressProtocolAdapterHelper() {
	for _, argument := range os.Args {
		if argument != protocolAdapterHelperArgument {
			continue
		}
		if err := protocolkit.ServeReferenceAdapter(os.Stdin, os.Stdout); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
}
