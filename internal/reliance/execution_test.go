package reliance

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type relianceReplayProvider struct {
	source provider.ExactReplaySource
	calls  atomic.Int64
}

type relianceReductionInput struct {
	Retained []stress.ReductionUnit `json:"retained"`
}

type relianceReductionOracle struct {
	required             stress.ReductionUnit
	relationDigest       string
	privacyDigest        string
	originalInputDigest  string
	originalReplayDigest string
	evaluations          atomic.Int64
}

func TestRelationBackedExecutionUsesSharedReplayRunner(t *testing.T) {
	request := relationExecutionRequestFixture(t)
	replayProvider := newRelianceReplayProvider(t)
	runner := newRelianceReplayRunner(t, replayProvider)
	result, err := RunRelationBackedIntervention(context.Background(), runner, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := result.Validate(request); err != nil {
		t.Fatal(err)
	}
	if result.Replay.RelationDigest != request.Admission.RelationDigest ||
		result.Replay.CaseID != request.Admission.RelationCaseID || result.RelationResult.Digest == "" ||
		result.RelationResult.Admission == nil || replayProvider.calls.Load() == 0 {
		t.Fatalf("relation-backed shared-runner result is incomplete: %+v", result)
	}
}

func TestRelationBackedExecutionRejectsTrajectorySubstitutionBeforeReplay(t *testing.T) {
	request := relationExecutionRequestFixture(t)
	replayProvider := newRelianceReplayProvider(t)
	runner := newRelianceReplayRunner(t, replayProvider)
	request.Transformed.Trajectories = slices.Clone(request.Original.Trajectories)
	_, err := RunRelationBackedIntervention(context.Background(), runner, request)
	if err == nil || replayProvider.calls.Load() != 0 {
		t.Fatalf("substituted trajectory error=%v replay_calls=%d", err, replayProvider.calls.Load())
	}
}

func TestRelationBackedReductionUsesSharedCounterexampleReducer(t *testing.T) {
	request := relationExecutionRequestFixture(t)
	execution, err := RunRelationBackedIntervention(context.Background(),
		newRelianceReplayRunner(t, newRelianceReplayProvider(t)), request)
	if err != nil {
		t.Fatal(err)
	}
	input := relianceReductionFixture()
	privacyDigest := protocolkit.DigestBytes([]byte("reliance-reduction-privacy"))
	oracle := newRelianceReductionOracle(t, input, request.Admission.RelationDigest, privacyDigest, execution.Replay.Digest)
	witness, err := ReduceRelationBackedIntervention(context.Background(), RelationReductionRequest{
		ExecutionRequest: request, Execution: execution, PrivacyPolicyDigest: privacyDigest,
		Input: input, Oracle: oracle, MaximumEvaluations: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(witness.FinalUnits) != 1 || witness.FinalUnits[0] != oracle.required ||
		witness.SourceResultDigest != execution.Replay.Digest || witness.OriginalObservation.ReplayResultDigest != execution.Replay.Digest {
		t.Fatalf("relation-backed reduction witness is incomplete: %+v", witness)
	}
}

func TestCrossVersionAdmissionCannotEnterSharedReducer(t *testing.T) {
	request := relationExecutionRequestFixture(t)
	execution, err := RunRelationBackedIntervention(context.Background(),
		newRelianceReplayRunner(t, newRelianceReplayProvider(t)), request)
	if err != nil {
		t.Fatal(err)
	}
	request.AdmissionInputs.ReplayedRelationCase.MutationProgramVersion = mutation.MutationProgramVersionV1
	oracle := &relianceReductionOracle{}
	_, err = ReduceRelationBackedIntervention(context.Background(), RelationReductionRequest{
		ExecutionRequest: request, Execution: execution, PrivacyPolicyDigest: protocolkit.DigestBytes([]byte("privacy")),
		Input: relianceReductionFixture(), Oracle: oracle, MaximumEvaluations: 6,
	})
	if err == nil || oracle.evaluations.Load() != 0 {
		t.Fatalf("cross-version reduction error=%v evaluations=%d", err, oracle.evaluations.Load())
	}
}

func relationExecutionRequestFixture(t *testing.T) RelationExecutionRequest {
	t.Helper()
	base := relationAdmissionFixture(t, EstimandEvidenceOnly, true)
	parents := relationInputsWithStatus(t, base, stress.EstimandSensitivity, stress.AdmissionHumanSupported)
	admission, err := BindRelationBackedIntervention(parents)
	if err != nil {
		t.Fatal(err)
	}
	original := relationVerificationInput(parents, RelationExecutionOriginalVariant, parents.ReplayedRelationCase.Original)
	transformed := relationVerificationInput(parents, RelationExecutionTransformedVariant, parents.ReplayedRelationCase.Transformed)
	return RelationExecutionRequest{AdmissionInputs: parents, Admission: admission, Original: original, Transformed: transformed}
}

func relationVerificationInput(parents RelationAdmissionInputs, variant string, trajectories []preprocess.Trajectory) verification.Input {
	return verification.Input{
		Entrypoint: "reliance-replay", Mode: verification.ModeAbsolute, Task: "verify the admitted evidence intervention",
		Trajectories: renderRelationExecutionTrajectories(trajectories),
		Criteria:     []verifier.Criterion{{ID: "correctness", Name: "Correctness", Description: "Assess correctness."}},
		Policy: verification.Policy{
			Evidence: verification.EvidenceExplicitJudge, NReps: 1, Epsilon: 0.02, BiasMitigation: "disabled",
			InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise", MaxWorkers: 2, MaxPairCalls: 4, ConfidenceThreshold: 0.8,
		},
		StudyManifestDigest: protocolkit.DigestBytes([]byte("reliance-study")), StudyVariant: variant, DisableCache: true,
		Lineage: verification.LineageReferences{
			AuditCaseID: parents.ConstructAdmission.CaseID, TransformationID: parents.Relation.ID,
			OutcomeEvidenceDigest: parents.ReplayedRelationCase.OutcomeEvidenceDigest,
			StudyCellID:           parents.Intervention.Intervention.InterventionID,
		},
	}
}

func newRelianceReplayRunner(t *testing.T, replayProvider provider.Provider) *stress.ReplayFirstRunner {
	t.Helper()
	service, err := verification.NewService(relianceVerificationConfig(), func(_ context.Context, plan verification.Plan) (verification.Runtime, error) {
		return verification.Runtime{Runner: &mode.Runner{
			Provider: replayProvider, Budget: mode.NewRunBudget(plan.Input.Limits),
			Cfg: mode.RunnerConfig{
				Model: "fixture", BaseURL: "https://replay.invalid/v1", ThinkingMode: "disabled", Entrypoint: plan.Input.Entrypoint,
				Temperature: 1, MaxTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK, MaxWorkers: 2, JudgeMode: true,
			},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := stress.NewReplayFirstRunner(service)
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func relianceVerificationConfig() verification.Config {
	return verification.Config{
		PreprocessBudget: 0,
		RequestProfile: verification.RequestProfile{
			ProviderID: "reliance-replay", BaseURL: "https://replay.invalid/v1", RequestedModel: "fixture",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK,
		},
		BudgetProfile: verification.BudgetProfile{MaxWorkers: 2, RequestTimeout: 10 * time.Second}, Offline: true,
	}
}

func newRelianceReplayProvider(t *testing.T) *relianceReplayProvider {
	t.Helper()
	request, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID: "reliance-replay", BaseURL: "https://replay.invalid/v1", RequestedModel: "fixture",
		Messages: []provider.Message{{Role: "user", Content: "source identity"}}, MaxOutputTokens: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &relianceReplayProvider{source: provider.ExactReplaySource{
		SchemaVersion: provider.ExactReplaySourceSchemaVersion, CaptureSHA256: protocolkit.DigestBytes([]byte("capture")),
		Bytes: 1_000, Records: 1_000, CaptureSchemaVersion: provider.CaptureSchemaVersion,
		RequestSchemaVersion: provider.RequestSchemaVersion, ParserContractVersion: provider.ParserContractVersion,
		ProviderID: "reliance-replay", RouteID: request.RouteID(), RequestedModel: "fixture",
		RequestSetDigest: protocolkit.DigestBytes([]byte("requests")), LineageSetDigest: protocolkit.DigestBytes([]byte("lineages")),
		ResponseBodySetDigest: protocolkit.DigestBytes([]byte("bodies")), EvidenceSetDigest: protocolkit.DigestBytes([]byte("evidence")),
		RecordSetDigest: protocolkit.DigestBytes([]byte("records")),
	}}
}

func (*relianceReplayProvider) Name() string { return "reliance-replay" }

func (*relianceReplayProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: verifier.MinimumVerifierTopK, MaxConcurrent: 2}
}

func (fixture *relianceReplayProvider) ExactReplaySource() (provider.ExactReplaySource, bool) {
	return fixture.source, true
}

func (fixture *relianceReplayProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	fixture.calls.Add(1)
	var raw strings.Builder
	for _, tag := range request.ScoreTags {
		raw.WriteString(tag)
		raw.WriteString("M</")
		raw.WriteString(strings.TrimPrefix(tag, "<"))
		raw.WriteByte('\n')
	}
	response := provider.ResponseRecord{
		RawText: raw.String(), NormalizedBody: []byte(raw.String()), ReplayStatus: provider.ReplayStatusExact,
		ProviderRequestID: "reliance-replay", ReceivedAt: 1,
	}
	return provider.FinalizeResponse(request, response)
}

func (input relianceReductionInput) CanonicalBytes() ([]byte, error) { return json.Marshal(input) }

func (input relianceReductionInput) Units() []stress.ReductionUnit {
	return slices.Clone(input.Retained)
}

func (input relianceReductionInput) Remove(removed stress.ReductionUnit) (stress.ReducibleInput, error) {
	if !slices.Contains(input.Retained, removed) {
		return nil, errors.New("reliance reduction unit is absent")
	}
	retained := slices.DeleteFunc(slices.Clone(input.Retained), func(value stress.ReductionUnit) bool { return value == removed })
	return relianceReductionInput{Retained: retained}, nil
}

func relianceReductionFixture() relianceReductionInput {
	return relianceReductionInput{Retained: []stress.ReductionUnit{
		{Kind: "evidence_factor", ID: "executable-evidence"},
		{Kind: "evidence_factor", ID: "irrelevant-metadata"},
		{Kind: "evidence_factor", ID: "success-prose"},
	}}
}

func newRelianceReductionOracle(t *testing.T, input relianceReductionInput, relationDigest, privacyDigest, replayDigest string) *relianceReductionOracle {
	t.Helper()
	encoded, err := input.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	return &relianceReductionOracle{
		required: input.Retained[0], relationDigest: relationDigest, privacyDigest: privacyDigest,
		originalInputDigest: protocolkit.DigestBytes(encoded), originalReplayDigest: replayDigest,
	}
}

func (oracle *relianceReductionOracle) Evaluate(_ context.Context, candidate stress.ReducibleInput) (stress.ReductionObservation, error) {
	oracle.evaluations.Add(1)
	input, ok := candidate.(relianceReductionInput)
	if !ok {
		return stress.ReductionObservation{}, errors.New("unexpected reliance reduction input")
	}
	encoded, err := input.CanonicalBytes()
	if err != nil {
		return stress.ReductionObservation{}, err
	}
	inputDigest := protocolkit.DigestBytes(encoded)
	replayDigest := protocolkit.DigestBytes(append([]byte("candidate-replay:"), encoded...))
	if inputDigest == oracle.originalInputDigest {
		replayDigest = oracle.originalReplayDigest
	}
	return stress.SealReductionObservation(stress.ReductionObservation{
		RelationDigest: oracle.relationDigest, PrivacyPolicyDigest: oracle.privacyDigest,
		RelationRevalidated: true, PrivacyRevalidated: true, ViolationPreserved: slices.Contains(input.Retained, oracle.required),
		RelationProofDigest: protocolkit.DigestBytes(append([]byte("relation:"), encoded...)),
		PrivacyProofDigest:  protocolkit.DigestBytes(append([]byte("privacy:"), encoded...)), ReplayResultDigest: replayDigest,
	})
}
