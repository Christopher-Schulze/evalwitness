package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const applicationAdapterTestHelperArgument = "--evalwitness-application-adapter-test-helper"

func TestApplicationProtocolAdapterMatchesServiceAtCanonicalTraceBoundary(t *testing.T) {
	service, err := verification.NewService(verification.Config{
		Redact: true, PreprocessBudget: 4096, Offline: true,
		RequestProfile: verification.RequestProfile{
			ProviderID: "fixture", BaseURL: "https://fixture.invalid/v1", RequestedModel: "fixture",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK,
		},
		BudgetProfile: verification.BudgetProfile{MaxWorkers: 1, RequestTimeout: time.Second},
	}, func(_ context.Context, plan verification.Plan) (verification.Runtime, error) {
		return verification.Runtime{Runner: &mode.Runner{
			Budget: mode.NewRunBudget(plan.Input.Limits),
			Cfg:    mode.RunnerConfig{Entrypoint: plan.Input.Entrypoint, MaxWorkers: 1},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if applicationAdapterTestHelperRequested() {
		if err := protocolkit.ServeAdapter(os.Stdin, os.Stdout, applicationProtocolEvaluator{service: service}); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	raw, err := os.ReadFile("../../internal/preprocess/testdata/trace/agent-trace-0.1.0.json")
	if err != nil {
		t.Fatal(err)
	}
	canonicalSource, err := protocolkit.CanonicalizeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	criterion := verifier.BuiltinCriteria["generic"]
	policy := verification.Policy{
		Evidence: verification.EvidenceStrictVerifier, NReps: 1, Epsilon: 0.02,
		BiasMitigation: "both", InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise",
		MaxWorkers: 1, MaxPairCalls: 2, ConfidenceThreshold: 0.6, CalibrationSigma: 0.05,
	}
	directInput := verification.Input{
		Entrypoint: "cli.verify", Mode: verification.ModePairwise, Task: "compare traces",
		Trajectories: []string{string(canonicalSource), string(canonicalSource)}, Criteria: []verifier.Criterion{criterion}, Policy: policy,
		DisableCache: true,
	}
	directPlan, err := service.Plan(directInput)
	if err != nil {
		t.Fatal(err)
	}
	directResult, err := service.Execute(context.Background(), directPlan)
	if err != nil {
		t.Fatal(err)
	}
	references := make([]protocolkit.TrajectoryRef, len(directPlan.TrajectoryEvidence))
	for index, evidence := range directPlan.TrajectoryEvidence {
		references[index] = protocolkit.TrajectoryRef{
			CanonicalSchema: "evalwitness.trajectory.v1", SourceFormat: string(evidence.SourceFormat),
			SourceDigest: evidence.SourceDigest, TrajectoryDigest: evidence.TrajectoryDigest,
			AccountingDigest: evidence.IngestionDigest, TraceEnvelopeDigest: evidence.TraceEnvelopeDigest,
			MappingReportDigest: evidence.TraceMappingDigest, MappingPolicyVersion: evidence.TraceMappingPolicy,
			Inline: append(json.RawMessage(nil), canonicalSource...),
		}
	}
	request := protocolkit.ApplicationInvocation{
		SchemaVersion: protocolkit.ApplicationInvocationSchema, Mode: "pairwise", Task: "compare traces",
		Criteria: []protocolkit.ApplicationCriterion{{ID: criterion.ID, Name: criterion.Name, Description: criterion.Description}},
		Policy: protocolkit.ApplicationPolicy{
			Evidence: string(policy.Evidence), NReps: policy.NReps, Epsilon: "0.02",
			BiasMitigation: policy.BiasMitigation, InconsistencyPolicy: policy.InconsistencyPolicy,
			SelectionStrategy: policy.SelectionStrategy, MaxWorkers: policy.MaxWorkers,
			MaxPairCalls: policy.MaxPairCalls, ConfidenceThreshold: "0.6",
			CalibrationSigma: "0.05",
			SPRT:             protocolkit.ApplicationSPRT{Alpha: "0", Beta: "0", Sigma: "0", Epsilon: "0"},
		},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	auditCase := protocolkit.AuditCase{
		SchemaVersion: protocolkit.CaseSchema, ProtocolVersion: protocolkit.CurrentVersion,
		CaseID: "application.trace-equivalence", Level: protocolkit.LevelDeterministicReplay,
		Kind: protocolkit.CaseExtension, Description: "application adapter trace boundary",
		RequiredCapabilities: []string{"application_service"},
		Invocation: protocolkit.AuditInvocation{
			SchemaVersion: protocolkit.InvocationSchema, InvocationID: "application.trace-equivalence",
			Operation: protocolkit.CaseExtension, Offline: true, TimeoutMillis: 1000, MaxInputBytes: 4 << 20,
			TrajectoryRefs: references,
			Extensions: []protocolkit.Extension{{
				Namespace: protocolkit.ApplicationExtensionNamespace, Schema: protocolkit.ApplicationInvocationSchema,
				Required: true, Payload: payload,
			}},
		},
		Expected: protocolkit.ExpectedOutcome{Status: protocolkit.ExpectationAccepted}, Extensions: []protocolkit.Extension{},
	}
	evaluator := applicationProtocolEvaluator{service: service}
	result := evaluator.Evaluate(auditCase)
	if err := protocolkit.ValidateInvocationResult(result, auditCase.Invocation.InvocationID); err != nil {
		t.Fatal(err)
	}
	if result.Status != protocolkit.InvocationAccepted || len(result.Extensions) != 1 {
		t.Fatalf("application result = %+v", result)
	}
	var applicationResult protocolkit.ApplicationResult
	if err := protocolkit.DecodeStrict(result.Extensions[0].Payload, &applicationResult); err != nil {
		t.Fatal(err)
	}
	directDecision, err := json.Marshal(directResult.Selection)
	if err != nil {
		t.Fatal(err)
	}
	if applicationResult.RunFingerprint != directPlan.RunFingerprint ||
		applicationResult.RequestSetFingerprint != directPlan.Requests.SetFingerprint ||
		applicationResult.DecisionUTF8Hex != hex.EncodeToString(directDecision) {
		t.Fatalf("application=%+v direct_plan=%+v direct_result=%+v", applicationResult, directPlan, directResult)
	}
	caseRaw, err := protocolkit.CanonicalMarshal(auditCase)
	if err != nil {
		t.Fatal(err)
	}
	corpusDigest, err := protocolkit.Digest([]json.RawMessage{caseRaw})
	if err != nil {
		t.Fatal(err)
	}
	schemaRaw, err := protocolkit.ReadSchemaArtifact("audit-case.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	corpus := protocolkit.NormativeCorpus{
		Vectors: []protocolkit.CaseVector{{
			CaseID: auditCase.CaseID, Level: auditCase.Level, Kind: auditCase.Kind,
			Description: auditCase.Description, Expected: auditCase.Expected, Raw: caseRaw,
		}},
		Digest: corpusDigest, SchemaArtifactDigest: protocolkit.DigestBytes(schemaRaw),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	external, err := protocolkit.RunAdapterCorpus(ctx, protocolkit.AdapterCommand{
		Path: os.Args[0], Arguments: []string{"-test.run=TestApplicationProtocolAdapterMatchesServiceAtCanonicalTraceBoundary", "--", applicationAdapterTestHelperArgument},
	}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if len(external.Results) != 1 || external.Results[0].Outcome != protocolkit.OutcomePassed ||
		external.Results[0].ResultDigest != result.EvidenceDigest || external.Results[0].ObservedDigest != directPlan.RunFingerprint {
		t.Fatalf("external application adapter result=%+v direct=%+v", external.Results, result)
	}
	nonCanonical := auditCase
	nonCanonical.Invocation.TrajectoryRefs = append([]protocolkit.TrajectoryRef(nil), auditCase.Invocation.TrajectoryRefs...)
	nonCanonical.Invocation.TrajectoryRefs[0].Inline = append(json.RawMessage(nil), raw...)
	if rejected := evaluator.Evaluate(nonCanonical); rejected.Status != protocolkit.InvocationRejected {
		t.Fatalf("non-canonical inline source status = %s", rejected.Status)
	}

	auditCase.Invocation.TrajectoryRefs[0].MappingReportDigest = digestApplicationBytes([]byte("changed"))
	rejected := evaluator.Evaluate(auditCase)
	if rejected.Status != protocolkit.InvocationRejected {
		t.Fatalf("mapping-loss mismatch status = %s", rejected.Status)
	}
}

func applicationAdapterTestHelperRequested() bool {
	for _, argument := range os.Args {
		if argument == applicationAdapterTestHelperArgument {
			return true
		}
	}
	return false
}

func TestApplicationDecimalRejectsAmbiguousWireForms(t *testing.T) {
	for _, value := range []string{"", "00", "01", "-0", "0.0", "1.20", ".5", "1.", "1e-3", "NaN", "+1"} {
		if _, err := applicationDecimal(value, "fixture"); err == nil {
			t.Fatalf("application decimal %q was accepted", value)
		}
	}
	for _, value := range []string{"0", "1", "10", "0.5", "1.25"} {
		if _, err := applicationDecimal(value, "fixture"); err != nil {
			t.Fatalf("canonical application decimal %q: %v", value, err)
		}
	}
}
