package verification

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type judgeProvider struct {
	requests []provider.RequestEnvelope
}

func (*judgeProvider) Name() string { return "fixture" }

func (*judgeProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{MaxConcurrent: 1, ContextLimit: 32_768}
}

func (providerFixture *judgeProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	providerFixture.requests = append(providerFixture.requests, request)
	raw := "<score>T</score>"
	if len(request.ScoreTags) == 2 {
		raw = "<score_A>T</score_A>\n<score_B>A</score_B>"
	}
	return provider.FinalizeResponse(request, provider.ResponseRecord{
		RawText: raw, NormalizedBody: []byte(raw), ServedModel: "fixture-model",
	})
}

type retryingPositionProvider struct{}

func (*retryingPositionProvider) Name() string { return "fixture" }

func (*retryingPositionProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{MaxConcurrent: 1, ContextLimit: 32_768}
}

type failingProvider struct {
	requests int
}

func (*failingProvider) Name() string { return "fixture" }

func (*failingProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{MaxConcurrent: 1, ContextLimit: 32_768}
}

func (fixture *failingProvider) Score(context.Context, provider.RequestEnvelope) (provider.ResponseRecord, error) {
	fixture.requests++
	return provider.ResponseRecord{}, errors.New("fixture dispatch failed")
}

func (*retryingPositionProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	if request.BeforeAttempt != nil {
		if err := request.BeforeAttempt(); err != nil {
			return provider.ResponseRecord{}, err
		}
	}
	if request.AfterAttempt != nil {
		if err := request.AfterAttempt(provider.AttemptResult{Err: errors.New("transient fixture failure")}); err != nil {
			return provider.ResponseRecord{}, err
		}
	}
	if request.BeforeAttempt != nil {
		if err := request.BeforeAttempt(); err != nil {
			return provider.ResponseRecord{}, err
		}
	}
	raw := "<score_A>T</score_A>\n<score_B>A</score_B>"
	response, err := provider.FinalizeResponse(request, provider.ResponseRecord{
		RawText: raw, NormalizedBody: []byte(raw), ServedModel: "fixture-model",
	})
	if request.AfterAttempt != nil {
		if afterErr := request.AfterAttempt(provider.AttemptResult{}); afterErr != nil {
			return provider.ResponseRecord{}, errors.Join(err, afterErr)
		}
	}
	return response, err
}

func validPolicy(evidence EvidencePolicy) Policy {
	return Policy{
		Evidence: evidence, NReps: 1, Epsilon: 0.02, BiasMitigation: "both",
		InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise", MaxWorkers: 1, MaxPairCalls: 2,
		ConfidenceThreshold: 0.6, CalibrationSigma: 0.05,
	}
}

func testServiceConfig() Config {
	return Config{
		Redact: true, PreprocessBudget: 1024,
		RequestProfile: RequestProfile{
			ProviderID: "fixture", BaseURL: "https://fixture.invalid/v1", RequestedModel: "fixture-model",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: 20,
		},
		BudgetProfile: BudgetProfile{MaxRetries: 1, MaxWorkers: 1, RequestTimeout: time.Second},
		Offline:       true,
	}
}

func TestPlanPreservesCriteriaOrderAndSharesFingerprintAcrossEntrypoints(t *testing.T) {
	service, err := NewService(testServiceConfig(), func(context.Context, Plan) (Runtime, error) {
		return Runtime{}, errors.New("not executed")
	})
	if err != nil {
		t.Fatal(err)
	}
	input := Input{
		Entrypoint: "cli.verify", Mode: ModeDelta, Task: "repair defect",
		Trajectories: []string{"first", "second"},
		Criteria: []verifier.Criterion{
			verifier.BuiltinCriteria["verification"],
			verifier.BuiltinCriteria["code_review"],
		},
		Policy: validPolicy(EvidenceStrictVerifier),
	}
	cliPlan, err := service.Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Entrypoint = "mcp.delta"
	mcpPlan, err := service.Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	if cliPlan.RunFingerprint != mcpPlan.RunFingerprint {
		t.Fatalf("equivalent entrypoints differ: %s != %s", cliPlan.RunFingerprint, mcpPlan.RunFingerprint)
	}
	if got := cliPlan.Input.Criteria[0].ID; got != "verification" {
		t.Fatalf("first criterion = %q, want caller order to preserve prompt parity", got)
	}
	if cliPlan.TrajectoryEvidence[0].TrajectoryDigest == "" || cliPlan.TrajectoryEvidence[0].TraceEnvelopeDigest == "" {
		t.Fatal("canonical trajectory and trace-envelope evidence were not bound")
	}
}

func TestJointAbsolutePlanBindsAllCandidatesIntoOneRequest(t *testing.T) {
	service, err := NewService(testServiceConfig(), func(context.Context, Plan) (Runtime, error) {
		return Runtime{}, errors.New("not executed")
	})
	if err != nil {
		t.Fatal(err)
	}
	policy := validPolicy(EvidenceStrictVerifier)
	policy.SelectionStrategy = "joint_absolute"
	policy.BiasMitigation = "adaptive"
	plan, err := service.Plan(Input{
		Entrypoint: "eval-terminal", Mode: ModePairwise, Task: "repair defect",
		Trajectories: []string{"one", "two", "three", "four", "five"},
		Criteria:     []verifier.Criterion{verifier.BuiltinCriteria["code_review"]}, Policy: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Requests.Requests) != 1 || plan.Requests.WorstLogicalCalls != 1 {
		t.Fatalf("joint request plan = templates:%d calls:%d", len(plan.Requests.Requests), plan.Requests.WorstLogicalCalls)
	}
	request := plan.Requests.Requests[0]
	if len(request.ScoreTags) != 5 || len(request.EvidenceBindings) != 5 {
		t.Fatalf("joint request = tags:%d bindings:%d", len(request.ScoreTags), len(request.EvidenceBindings))
	}
}

func TestCanonicalFixtureProducesByteStableDecisionAcrossEveryApplicationEntrypoint(t *testing.T) {
	providerFixture := &judgeProvider{}
	service, err := NewService(testServiceConfig(), func(_ context.Context, plan Plan) (Runtime, error) {
		return Runtime{Runner: &mode.Runner{
			Provider: providerFixture, Budget: mode.NewRunBudget(plan.Input.Limits),
			Cfg: mode.RunnerConfig{
				Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", Entrypoint: plan.Input.Entrypoint,
				ThinkingMode: "disabled", Temperature: 1, JudgeMode: true, MaxWorkers: 1, MaxTokens: 128,
			},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	entrypoints := []string{"cli.verify", "mcp.delta", "eval-terminal", "eval-swebench", "best-of-n", "protocol.application"}
	var canonicalRun, canonicalRequestSet string
	var canonicalDecision []byte
	for _, entrypoint := range entrypoints {
		requestOffset := len(providerFixture.requests)
		plan, planErr := service.Plan(Input{
			Entrypoint: entrypoint, Mode: ModeDelta, Task: "repair", Trajectories: []string{"first", "second"},
			Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
		})
		if planErr != nil {
			t.Fatalf("%s plan: %v", entrypoint, planErr)
		}
		result, runErr := service.Execute(context.Background(), plan)
		if runErr != nil {
			t.Fatalf("%s execute: %v", entrypoint, runErr)
		}
		decision, marshalErr := json.Marshal(result.Delta)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if canonicalRun == "" {
			canonicalRun, canonicalRequestSet, canonicalDecision = plan.RunFingerprint, plan.Requests.SetFingerprint, decision
		} else if plan.RunFingerprint != canonicalRun || plan.Requests.SetFingerprint != canonicalRequestSet || string(decision) != string(canonicalDecision) {
			t.Fatalf("%s diverged: run=%s request_set=%s decision=%s", entrypoint, plan.RunFingerprint, plan.Requests.SetFingerprint, decision)
		}
		actualSet := make(map[string]struct{})
		for _, request := range providerFixture.requests[requestOffset:] {
			fingerprint, fingerprintErr := request.Fingerprint()
			if fingerprintErr != nil {
				t.Fatal(fingerprintErr)
			}
			actualSet[string(fingerprint)] = struct{}{}
		}
		if actual := digestOrderedStrings(sortedKeys(actualSet)); actual != canonicalRequestSet {
			t.Fatalf("%s dispatched request set %s, want %s; actual=%v planned=%v", entrypoint, actual, canonicalRequestSet, sortedKeys(actualSet), plan.Requests.Fingerprints)
		}
	}
}

func TestTraceRoundTripConvergesOrExposesMappingDifferenceBeforeScoring(t *testing.T) {
	raw, err := os.ReadFile("../preprocess/testdata/trace/agent-trace-0.1.0.json")
	if err != nil {
		t.Fatal(err)
	}
	options := preprocess.DefaultTraceImportOptions()
	options.Privacy = preprocess.PrivacyFull
	imported, err := preprocess.ImportTraceBytes(raw, options)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := preprocess.ExportOTLPJSON(imported, preprocess.PrivacyFull)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(testServiceConfig(), func(context.Context, Plan) (Runtime, error) {
		return Runtime{}, errors.New("scoring must not run in a trace-boundary conformance test")
	})
	if err != nil {
		t.Fatal(err)
	}
	planFor := func(trajectory string) Plan {
		plan, planErr := service.Plan(Input{
			Entrypoint: "trace.conformance", Mode: ModeAbsolute, Task: "inspect", Trajectories: []string{trajectory},
			Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceStrictVerifier),
		})
		if planErr != nil {
			t.Fatal(planErr)
		}
		return plan
	}
	direct := planFor(string(raw))
	roundTrip := planFor(string(exported.Bytes))
	if direct.PreparedTextDigests[0] != roundTrip.PreparedTextDigests[0] &&
		direct.TrajectoryEvidence[0].TraceMappingDigest == roundTrip.TrajectoryEvidence[0].TraceMappingDigest {
		t.Fatal("non-equivalent mapped traces reached planning without an explicit mapping-report difference")
	}
}

func TestPlanBindsValidatedCallerLineageWithoutAllowingGeneratedEvidenceOverrides(t *testing.T) {
	service, err := NewService(testServiceConfig(), func(context.Context, Plan) (Runtime, error) {
		return Runtime{}, errors.New("not executed")
	})
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	input := Input{
		Entrypoint: "protocol.application", Mode: ModeAbsolute, Task: "audit", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceStrictVerifier),
		Lineage: LineageReferences{
			AuditCaseID: "case.fixture", TransformationID: "transform.fixture", OutcomeEvidenceDigest: digest,
			ProfilePolicyDigest: digest, CapsuleDigest: digest, StudyCellID: "cell.fixture",
		},
	}
	plan, err := service.Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	lineage := plan.Requests.Requests[0].Lineage
	if lineage.AuditCaseID != input.Lineage.AuditCaseID || lineage.MutationID != input.Lineage.TransformationID ||
		lineage.StudyCellID != input.Lineage.StudyCellID || lineage.SourceTraceHash == "" || lineage.TraceMapHash == "" || lineage.PolicyHash == "" {
		t.Fatalf("planned request lineage = %+v", lineage)
	}
	originalRun := plan.RunFingerprint
	originalRequestSet := plan.Requests.SetFingerprint
	input.Lineage.AuditCaseID = "case.changed"
	changed, err := service.Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	if changed.RunFingerprint == originalRun || changed.Requests.SetFingerprint != originalRequestSet {
		t.Fatalf("lineage must bind run identity but not semantic request identity: original=%s changed=%s request=%s/%s", originalRun, changed.RunFingerprint, originalRequestSet, changed.Requests.SetFingerprint)
	}
	input.Lineage.OutcomeEvidenceDigest = strings.Repeat("A", 64)
	if _, err := service.Plan(input); err == nil || !strings.Contains(err.Error(), "lowercase sha256") {
		t.Fatalf("invalid caller lineage digest error = %v", err)
	}
}

func TestRunOwnsRuntimeDispatchAndCleanup(t *testing.T) {
	closed := false
	providerFixture := &judgeProvider{}
	service, err := NewService(testServiceConfig(), func(_ context.Context, plan Plan) (Runtime, error) {
		return Runtime{
			Runner: &mode.Runner{
				Provider: providerFixture,
				Cfg: mode.RunnerConfig{
					Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", Entrypoint: plan.Input.Entrypoint,
					ThinkingMode: "disabled", Temperature: 1, JudgeMode: true, MaxWorkers: 1, MaxTokens: 128,
				},
			},
			Close: func() error { closed = true; return nil },
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Plan(Input{
		Entrypoint: "test", Mode: ModeAbsolute, Task: "judge the result", Trajectories: []string{"completed"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
		Lineage: LineageReferences{AuditCaseID: "case.runtime", TransformationID: "transform.runtime", StudyCellID: "cell.runtime"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != verifier.DecisionSelected || result.Absolute == nil {
		t.Fatalf("result = %+v", result)
	}
	if !closed || result.Lifecycle.RuntimeOpen != LifecycleComplete || result.Lifecycle.Execution != LifecycleComplete || result.Lifecycle.Cleanup != LifecycleComplete {
		t.Fatalf("lifecycle = %+v closed=%t", result.Lifecycle, closed)
	}
	if len(providerFixture.requests) != 1 || len(plan.Requests.Fingerprints) != 1 {
		t.Fatalf("actual requests=%d planned fingerprints=%d", len(providerFixture.requests), len(plan.Requests.Fingerprints))
	}
	actualFingerprint, err := providerFixture.requests[0].Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if string(actualFingerprint) != plan.Requests.Fingerprints[0] {
		t.Fatalf("actual fingerprint %s != planned %s", actualFingerprint, plan.Requests.Fingerprints[0])
	}
	actualLineage := providerFixture.requests[0].Lineage
	if actualLineage.AuditCaseID != "case.runtime" || actualLineage.MutationID != "transform.runtime" ||
		actualLineage.StudyCellID != "cell.runtime" || actualLineage.SourceTraceHash == "" || actualLineage.TraceMapHash == "" || actualLineage.PolicyHash == "" {
		t.Fatalf("actual request lineage = %+v", actualLineage)
	}
}

func TestStrictPolicyRejectsJudgeBackedSelection(t *testing.T) {
	result := Result{
		State: verifier.DecisionSelected,
		Absolute: &mode.Score{
			State: verifier.DecisionSelected,
			EvidenceStrength: verifier.EvidenceStrength{
				ExtractionMode: verifier.ExtractionModeJudge, Observations: 1, ExtractedObservations: 1,
			},
			Usage: mode.UsageSummary{Calls: 1, JudgeTextCalls: 1, ExtractionMode: "judge"},
		},
	}
	if err := validateEvidencePolicy(EvidenceStrictVerifier, result); err == nil {
		t.Fatal("strict verifier accepted judge-backed selection")
	}
}

func TestExecuteRejectsModifiedPlanBeforeOpeningRuntime(t *testing.T) {
	opened := false
	service, err := NewService(testServiceConfig(), func(context.Context, Plan) (Runtime, error) {
		opened = true
		return Runtime{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.Plan(Input{
		Entrypoint: "test", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceStrictVerifier),
	})
	if err != nil {
		t.Fatal(err)
	}
	plan.Input.Task = "changed"
	if _, err := service.Execute(context.Background(), plan); err == nil {
		t.Fatal("modified plan was accepted")
	}
	if opened {
		t.Fatal("runtime opened before plan integrity validation")
	}
	plan, err = service.Plan(Input{
		Entrypoint: "test", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceStrictVerifier),
	})
	if err != nil {
		t.Fatal(err)
	}
	plan.PreparedTrajectories[0].Text = "mutated"
	if _, err := service.Execute(context.Background(), plan); err == nil {
		t.Fatal("modified prepared trajectory was accepted")
	}
	plan, err = service.Plan(Input{
		Entrypoint: "test", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceStrictVerifier),
	})
	if err != nil {
		t.Fatal(err)
	}
	plan.Requests.Requests[0].RequestedModel = "substituted"
	if _, err := service.Execute(context.Background(), plan); err == nil {
		t.Fatal("modified request plan was accepted")
	}
	plan, err = service.Plan(Input{
		Entrypoint: "test", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceStrictVerifier),
	})
	if err != nil {
		t.Fatal(err)
	}
	plan.Input.Limits.MaxDuration++
	if _, err := service.Execute(context.Background(), plan); err == nil {
		t.Fatal("modified duration limit was accepted")
	}
}

func TestExecuteBatchSharesOneRuntimeAndBudget(t *testing.T) {
	opened := 0
	closed := 0
	providerFixture := &judgeProvider{}
	service, err := NewService(testServiceConfig(), func(_ context.Context, plan Plan) (Runtime, error) {
		opened++
		return Runtime{
			Runner: &mode.Runner{
				Provider: providerFixture,
				Budget:   mode.NewRunBudget(plan.Input.Limits),
				Cfg: mode.RunnerConfig{
					Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", Entrypoint: plan.Input.Entrypoint,
					ThinkingMode: "disabled", Temperature: 1, JudgeMode: true, MaxWorkers: 1, MaxTokens: 128,
				},
			},
			Close: func() error { closed++; return nil },
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []Input{
		{
			Entrypoint: "eval.fixture", Mode: ModeAbsolute, Task: "first", Trajectories: []string{"alpha"},
			Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
		},
		{
			Entrypoint: "eval.fixture", Mode: ModeAbsolute, Task: "second", Trajectories: []string{"beta"},
			Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
		},
	}
	batch, err := service.PlanBatch(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Plans[0].Input.Limits.MaxCalls != 2 || batch.Requests.WorstLogicalCalls != 2 {
		t.Fatalf("batch calls: limits=%d planned=%d", batch.Plans[0].Input.Limits.MaxCalls, batch.Requests.WorstLogicalCalls)
	}
	result, err := service.ExecuteBatch(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if opened != 1 || closed != 1 {
		t.Fatalf("runtime opened=%d closed=%d, want one shared lifecycle", opened, closed)
	}
	if len(result.Results) != 2 || len(providerFixture.requests) != 2 || result.Budget.Calls != 2 {
		t.Fatalf("results=%d requests=%d budget=%+v", len(result.Results), len(providerFixture.requests), result.Budget)
	}
	for index, item := range result.Results {
		if item.State != verifier.DecisionSelected || item.Lifecycle.Execution != LifecycleComplete || item.Lifecycle.Cleanup != LifecycleComplete {
			t.Fatalf("result %d = %+v", index, item)
		}
	}
}

func TestExecuteBatchCancellationDoesNotReserveUndispatchedCalls(t *testing.T) {
	providerFixture := &failingProvider{}
	service, err := NewService(testServiceConfig(), func(_ context.Context, plan Plan) (Runtime, error) {
		return Runtime{Runner: &mode.Runner{
			Provider: providerFixture, Budget: mode.NewRunBudget(plan.Input.Limits),
			Cfg: mode.RunnerConfig{
				Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", Entrypoint: plan.Input.Entrypoint,
				ThinkingMode: "disabled", Temperature: 1, JudgeMode: true, MaxWorkers: 1, MaxTokens: 128,
			},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]Input, 3)
	for index := range inputs {
		inputs[index] = Input{
			Entrypoint: "eval.fixture", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trajectory"},
			Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
		}
	}
	batch, err := service.PlanBatch(inputs)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ExecuteBatch(context.Background(), batch)
	if err == nil || !strings.Contains(err.Error(), "fixture dispatch failed") {
		t.Fatalf("batch error = %v", err)
	}
	if providerFixture.requests != 1 || result.Budget.Calls != 1 {
		t.Fatalf("provider requests = %d, reserved calls = %d; want 1/1", providerFixture.requests, result.Budget.Calls)
	}
}

func TestExecuteBatchRejectsChangedAggregateBeforeOpeningRuntime(t *testing.T) {
	opened := false
	service, err := NewService(testServiceConfig(), func(context.Context, Plan) (Runtime, error) {
		opened = true
		return Runtime{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := service.PlanBatch([]Input{{
		Entrypoint: "eval.fixture", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceStrictVerifier),
	}})
	if err != nil {
		t.Fatal(err)
	}
	batch.Requests.WorstLogicalCalls++
	if _, err := service.ExecuteBatch(context.Background(), batch); err == nil {
		t.Fatal("changed aggregate request plan was accepted")
	}
	if opened {
		t.Fatal("runtime opened before batch integrity validation")
	}
}

func TestServiceWritesProviderAndRunLevelAuditEvidence(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	providerFixture := &judgeProvider{}
	service, err := NewService(testServiceConfig(), func(_ context.Context, plan Plan) (Runtime, error) {
		logger, openErr := audit.New(auditPath)
		if openErr != nil {
			return Runtime{}, openErr
		}
		return Runtime{
			Runner: &mode.Runner{
				Provider: providerFixture, Audit: logger, Budget: mode.NewRunBudget(plan.Input.Limits),
				Cfg: mode.RunnerConfig{
					Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", Entrypoint: plan.Input.Entrypoint,
					ThinkingMode: "disabled", Temperature: 1, JudgeMode: true, MaxWorkers: 1, MaxTokens: 128,
				},
			},
			CloseAudit: logger.Close,
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Run(context.Background(), Input{
		Entrypoint: "audit.fixture", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
		Lineage: LineageReferences{
			AuditCaseID: "case.audit", TransformationID: "transform.audit",
			OutcomeEvidenceDigest: strings.Repeat("a", 64), ProfilePolicyDigest: strings.Repeat("b", 64),
			CapsuleDigest: strings.Repeat("c", 64), StudyCellID: "cell.audit",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("audit rows = %d, want provider call plus run record", len(lines))
	}
	var entry audit.Entry
	if err := json.Unmarshal([]byte(lines[1]), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.EventType != "verification_run" || entry.RunFingerprint != result.RunFingerprint ||
		entry.RequestSetFingerprint == "" || entry.DecisionPolicyDigest == "" || entry.DecisionState != string(verifier.DecisionSelected) ||
		entry.ProviderAttempts != result.Budget.Attempts || entry.OutputStatus != "decision_complete" || entry.Lifecycle == nil ||
		entry.Lifecycle.Cleanup != string(LifecycleComplete) || entry.Lifecycle.Audit != string(LifecycleComplete) || entry.RunLineage == nil ||
		entry.RunLineage.AuditCaseID != "case.audit" || entry.RunLineage.TransformationID != "transform.audit" ||
		len(entry.RunLineage.SourceTraceDigests) != 1 || len(entry.RunLineage.TraceMapDigests) != 1 ||
		entry.CalibrationPolicy == nil || entry.CalibrationPolicy.Status != CalibrationPolicyUnsupported ||
		entry.Fallback == nil || entry.Fallback.Kind != "none" || entry.Fallback.Charged {
		t.Fatalf("run audit = %+v", entry)
	}
}

func TestServiceAuditsProviderRetriesAndInconsistentPairs(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	service, err := NewService(testServiceConfig(), func(_ context.Context, plan Plan) (Runtime, error) {
		logger, openErr := audit.New(auditPath)
		if openErr != nil {
			return Runtime{}, openErr
		}
		return Runtime{
			Runner: &mode.Runner{
				Provider: &retryingPositionProvider{}, Audit: logger, Budget: mode.NewRunBudget(plan.Input.Limits),
				Cfg: mode.RunnerConfig{
					Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", Entrypoint: plan.Input.Entrypoint,
					ThinkingMode: "disabled", Temperature: 1, JudgeMode: true, MaxWorkers: 1, MaxTokens: 128,
				},
			},
			CloseAudit: logger.Close,
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Run(context.Background(), Input{
		Entrypoint: "audit.retry", Mode: ModePairwise, Task: "select", Trajectories: []string{"first", "second"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Selection == nil || len(result.Selection.InconsistentPairs) != 1 || result.Budget.Attempts != 4 {
		t.Fatalf("selection or attempts = %+v budget=%+v", result.Selection, result.Budget)
	}
	entries := readAuditEntries(t, auditPath)
	attempts := make([]audit.Entry, 0, 4)
	var run audit.Entry
	for _, entry := range entries {
		switch entry.EventType {
		case "provider_attempt":
			attempts = append(attempts, entry)
		case "verification_run":
			run = entry
		}
	}
	if len(attempts) != 4 {
		t.Fatalf("provider-attempt audit rows = %d, want 4", len(attempts))
	}
	for index, entry := range attempts {
		wantAttempt := index%2 + 1
		wantStatus := "failed"
		if wantAttempt == 2 {
			wantStatus = "complete"
		}
		if entry.ProviderAttempt != wantAttempt || entry.AttemptStatus != wantStatus || entry.RequestFingerprint == "" {
			t.Fatalf("provider attempt %d = %+v", index, entry)
		}
	}
	if run.ProviderAttempts != 4 || len(run.InconsistentPairs) != 1 || run.InconsistentPairs[0] != [2]int{0, 1} {
		t.Fatalf("run audit retry/inconsistency evidence = %+v", run)
	}
}

func TestServicePersistsCleanupFailureInRunAudit(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	providerFixture := &judgeProvider{}
	service, err := NewService(testServiceConfig(), func(_ context.Context, plan Plan) (Runtime, error) {
		logger, openErr := audit.New(auditPath)
		if openErr != nil {
			return Runtime{}, openErr
		}
		return Runtime{
			Runner: &mode.Runner{
				Provider: providerFixture, Audit: logger, Budget: mode.NewRunBudget(plan.Input.Limits),
				Cfg: mode.RunnerConfig{
					Model: "fixture-model", BaseURL: "https://fixture.invalid/v1", Entrypoint: plan.Input.Entrypoint,
					ThinkingMode: "disabled", Temperature: 1, JudgeMode: true, MaxWorkers: 1, MaxTokens: 128,
				},
			},
			Close: func() error { return errors.New("fixture cleanup failed") }, CloseAudit: logger.Close,
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Run(context.Background(), Input{
		Entrypoint: "audit.cleanup", Mode: ModeAbsolute, Task: "task", Trajectories: []string{"trace"},
		Criteria: []verifier.Criterion{verifier.BuiltinCriteria["generic"]}, Policy: validPolicy(EvidenceExplicitJudge),
	})
	if err == nil || !strings.Contains(err.Error(), "fixture cleanup failed") || result.Lifecycle.Cleanup != LifecycleFailed {
		t.Fatalf("cleanup result=%+v err=%v", result.Lifecycle, err)
	}
	entries := readAuditEntries(t, auditPath)
	run := entries[len(entries)-1]
	if run.EventType != "verification_run" || run.OutputStatus != "cleanup_failed" || run.Lifecycle == nil ||
		run.Lifecycle.Cleanup != string(LifecycleFailed) || !strings.Contains(run.Error, "fixture cleanup failed") {
		t.Fatalf("cleanup audit = %+v", run)
	}
}

func TestRunAuditPreservesExecutionAndCleanupErrorsTogether(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	logger, err := audit.New(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	runner := &mode.Runner{Audit: logger}
	plan := Plan{
		RunFingerprint: strings.Repeat("a", 64),
		Input:          Input{Mode: ModeAbsolute, Policy: validPolicy(EvidenceExplicitJudge)},
		Requests:       RequestPlan{SetFingerprint: strings.Repeat("b", 64)},
	}
	result := Result{Lifecycle: Lifecycle{
		RuntimeOpen: LifecycleComplete, Execution: LifecycleFailed, Cleanup: LifecycleFailed, Audit: LifecycleFailed,
	}}
	if err := writeRunAudit(runner, plan, result, errors.New("execution broke"), errors.New("cleanup broke")); err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	entries := readAuditEntries(t, auditPath)
	if len(entries) != 1 || !strings.Contains(entries[0].Error, "execution broke") || !strings.Contains(entries[0].Error, "cleanup broke") {
		t.Fatalf("combined failure audit = %+v", entries)
	}
}

func readAuditEntries(t *testing.T, path string) []audit.Entry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := make([]audit.Entry, len(lines))
	for index, line := range lines {
		if err := json.Unmarshal([]byte(line), &entries[index]); err != nil {
			t.Fatalf("decode audit row %d: %v", index, err)
		}
	}
	return entries
}
