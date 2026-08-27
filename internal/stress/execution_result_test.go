package stress

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type firstPositionVerifierProvider struct{}

func (*firstPositionVerifierProvider) Name() string { return "first-position-verifier" }

func (*firstPositionVerifierProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: verifier.MinimumVerifierTopK, MaxConcurrent: 2}
}

func (*firstPositionVerifierProvider) ExactReplaySource() (provider.ExactReplaySource, bool) {
	source, err := stressExactReplaySource("first-position-verifier")
	return source, err == nil
}

func (*firstPositionVerifierProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	requestFingerprint, err := request.Fingerprint()
	if err != nil {
		return provider.ResponseRecord{}, err
	}
	var response strings.Builder
	tokens := make([]provider.TokenEvidence, 0, len(request.ScoreTags)*3)
	for index, tag := range request.ScoreTags {
		letter := "A"
		if index == 0 {
			letter = "T"
		}
		response.WriteString(tag)
		response.WriteString(letter)
		response.WriteString("</")
		response.WriteString(strings.TrimPrefix(tag, "<"))
		response.WriteByte('\n')
		tokens = append(tokens,
			provider.TokenEvidence{Position: len(tokens), Token: tag, Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			provider.TokenEvidence{Position: len(tokens) + 1, Token: letter, Logprob: probabilityLogString(0.6), TopAlternatives: strictAlternatives(letter)},
			provider.TokenEvidence{Position: len(tokens) + 2, Token: "</" + strings.TrimPrefix(tag, "<"), Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
		)
	}
	raw := response.String()
	return provider.FinalizeResponse(request, provider.ResponseRecord{
		RawText: raw, NormalizedBody: []byte(raw), ReplayStatus: provider.ReplayStatusExact,
		ProviderRequestID: "first-position-request-" + string(requestFingerprint), ReceivedAt: 1,
		HasLogprobs: true, ObservedTopLogprobs: verifier.MinimumVerifierTopK, OrderedTokenEvidence: tokens,
	})
}

func strictAlternatives(chosen string) []provider.TokenAlternative {
	result := make([]provider.TokenAlternative, 0, verifier.MinimumVerifierTopK)
	for letter := 'A'; letter <= 'T'; letter++ {
		probability := 0.001
		if string(letter) == chosen {
			probability = 0.6
		}
		result = append(result, provider.TokenAlternative{Token: string(letter), Logprob: probabilityLogString(probability)})
	}
	return result
}

func probabilityLogString(probability float64) string {
	return strconv.FormatFloat(math.Log(probability), 'g', -1, 64)
}

func TestCatalogScoreMarginsAreExplicitAndDirectional(t *testing.T) {
	equalDefinition, found := mutation.DefinitionFor(mutation.FamilyNeutralFormatting)
	if !found {
		t.Fatal("neutral-formatting mutation definition is missing")
	}
	equal, err := catalogConstraint(equalDefinition)
	if err != nil {
		t.Fatal(err)
	}
	original, within, outside := 0.50, 0.55, 0.551
	result, err := EvaluateConstraint(equal, &original, &within, "", "")
	if err != nil || result.Status != ConstraintSatisfied {
		t.Fatalf("equality boundary result = %+v, error = %v", result, err)
	}
	result, err = EvaluateConstraint(equal, &original, &outside, "", "")
	if err != nil || result.Status != ConstraintViolated {
		t.Fatalf("equality outside result = %+v, error = %v", result, err)
	}

	qualityDefinition, found := mutation.DefinitionFor(mutation.FamilyPatchHunkRemoval)
	if !found {
		t.Fatal("patch-hunk-removal mutation definition is missing")
	}
	quality, err := catalogConstraint(qualityDefinition)
	if err != nil {
		t.Fatal(err)
	}
	transformedAtBoundary, transformedInside := 0.45, 0.451
	result, err = EvaluateConstraint(quality, &original, &transformedAtBoundary, "", "")
	if err != nil || result.Status != ConstraintSatisfied {
		t.Fatalf("quality-effect boundary result = %+v, error = %v", result, err)
	}
	result, err = EvaluateConstraint(quality, &original, &transformedInside, "", "")
	if err != nil || result.Status != ConstraintViolated {
		t.Fatalf("quality-effect inside result = %+v, error = %v", result, err)
	}
}

func TestReplayResultUsesAbsoluteScoreMovementForSingleTrajectory(t *testing.T) {
	relation, err := sealV3CatalogRelation(mutation.FamilyNeutralFormatting, EstimandSensitivity, []preprocess.SourceFormat{preprocess.SourcePlainText})
	if err != nil {
		t.Fatal(err)
	}
	admission := formalAdmission(t, "mutation-score-result")
	runner, err := NewReplayFirstRunner(stressVerificationService(t, provider.ReplayStatusExact))
	if err != nil {
		t.Fatal(err)
	}
	original := stressVerificationInput(relation, admission, "original trajectory")
	transformed := stressVerificationInput(relation, admission, "transformed trajectory")
	original.Policy.NReps = catalogRepetitions
	transformed.Policy.NReps = catalogRepetitions
	execution, err := runner.RunMutation(context.Background(), ReplayRunRequest{
		Relation: relation, Admission: admission, Original: original, Transformed: transformed,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := SealReplayResult(relation, admission, "task-group-score", execution)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeSatisfied || len(result.ConstraintResults) != 1 ||
		result.ConstraintResults[0].Metric != MetricConditionalScore || result.CompletedRepetitions != catalogRepetitions ||
		result.ProviderCalls != 2*catalogRepetitions {
		t.Fatalf("single-trajectory replay result = %+v", result)
	}

	tampered := execution
	tampered.EvidencePolicy = verification.EvidenceStrictVerifier
	if _, err := SealReplayResult(relation, admission, "task-group-score", tampered); err == nil {
		t.Fatal("replay result accepted an evidence policy that contradicted extracted evidence")
	}
	tampered = execution
	tampered.Original.Observations = cloneScoreObservations(execution.Original.Observations)
	tampered.Original.Observations[0].ReplaySource.CaptureSHA256 = digestText("foreign-capture")
	if _, err := SealReplayResult(relation, admission, "task-group-score", tampered); err == nil {
		t.Fatal("replay result accepted an observation rebound to another capture source")
	}
	if execution.Original.Observations[0].ReplaySource.CaptureSHA256 == digestText("foreign-capture") {
		t.Fatal("score-observation cloning retained shared replay-source ownership")
	}
}

func TestReplayResultComparesSelectedTrajectoryIdentityAcrossCandidateReversal(t *testing.T) {
	relation, err := sealV3CatalogRelation(mutation.FamilyCandidateOrderReversal, EstimandSensitivity, []preprocess.SourceFormat{preprocess.SourcePlainText})
	if err != nil {
		t.Fatal(err)
	}
	admission := formalAdmission(t, "mutation-order-result")
	runner, err := NewReplayFirstRunner(firstPositionVerificationService(t))
	if err != nil {
		t.Fatal(err)
	}
	original := pairVerificationInput(relation, admission, []string{"candidate alpha", "candidate beta"})
	transformed := pairVerificationInput(relation, admission, []string{"candidate beta", "candidate alpha"})
	execution, err := runner.RunMutation(context.Background(), ReplayRunRequest{
		Relation: relation, Admission: admission, Original: original, Transformed: transformed,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := SealReplayResult(relation, admission, "task-group-order", execution)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeViolated || result.ConstraintResults[0].Metric != MetricDecision ||
		result.ConstraintResults[0].OriginalState == result.ConstraintResults[0].TransformedState {
		t.Fatalf("candidate-order replay result = %+v", result)
	}
}

func firstPositionVerificationService(t *testing.T) *verification.Service {
	t.Helper()
	service, err := verification.NewService(verification.Config{
		PreprocessBudget: 0,
		RequestProfile: verification.RequestProfile{
			ProviderID: "first-position-verifier", BaseURL: "https://replay.invalid/v1", RequestedModel: "fixture",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK,
		},
		BudgetProfile: verification.BudgetProfile{MaxWorkers: 2, RequestTimeout: 10 * time.Second},
		Offline:       true,
	}, func(_ context.Context, plan verification.Plan) (verification.Runtime, error) {
		return verification.Runtime{Runner: &mode.Runner{
			Provider: &firstPositionVerifierProvider{}, Budget: mode.NewRunBudget(plan.Input.Limits),
			Cfg: mode.RunnerConfig{
				Model: "fixture", BaseURL: "https://replay.invalid/v1", ThinkingMode: "disabled", Entrypoint: plan.Input.Entrypoint,
				Temperature: 1, MaxTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK, MaxWorkers: 2,
			},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func pairVerificationInput(relation Relation, admission ConstructAdmission, trajectories []string) verification.Input {
	return verification.Input{
		Entrypoint: "stress-replay", Mode: verification.ModePairwise, Task: "compare candidate trajectories",
		Trajectories: append([]string(nil), trajectories...),
		Criteria:     []verifier.Criterion{{ID: "correctness", Name: "Correctness", Description: "Assess correctness."}},
		Policy: verification.Policy{
			Evidence: verification.EvidenceStrictVerifier, NReps: relation.Repeat.MaximumRepetitions, Epsilon: 0.02,
			BiasMitigation: "disabled", InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise",
			MaxWorkers: 2, MaxPairCalls: 4, ConfidenceThreshold: 0.6,
		},
		DisableCache: true,
		Lineage:      verification.LineageReferences{AuditCaseID: admission.CaseID, TransformationID: relation.ID},
	}
}
