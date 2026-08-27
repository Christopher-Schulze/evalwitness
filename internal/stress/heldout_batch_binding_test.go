package stress

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestHeldOutCampaignBatchBindingLocksExactProviderWorkWithoutExecutionAuthority(t *testing.T) {
	_, lock, design, armPlan, registry, replayed, _ := currentHeldOutReadinessRefusal(t)
	campaign, err := BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := digestText("held-out-study-manifest")
	bindings := heldOutBatchBindings(t, campaign, lock, design, armPlan, registry, replayed, manifestDigest, "binding-model")
	value, err := BuildHeldOutCampaignBatchBinding(campaign, lock, design, armPlan, registry, replayed, bindings)
	if err != nil {
		t.Fatal(err)
	}
	if value.StudyManifestDigest != manifestDigest || value.LiveBatchBindings != 2 || value.SealedReplayBatchBindings != 1 ||
		value.VerificationInputs != 684 || value.LiveVerificationInputs != 456 || value.SealedReplayVerificationInputs != 228 ||
		len(value.RequiredAuthorizationDigests) != 2 || value.ExecutionPermitIssued || value.RunAuthorized ||
		value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired {
		t.Fatalf("held-out campaign batch binding = %+v", value)
	}
	source, sourceOK := heldOutArmBatchBindingByID(value.Arms, heldOutReplaySourceArmID)
	target, targetOK := heldOutArmBatchBindingByID(value.Arms, heldOutReplayTargetArmID)
	if !sourceOK || !targetOK || source.Batch.Offline || !target.Batch.Offline ||
		source.Batch.RequestSetFingerprint != target.Batch.RequestSetFingerprint ||
		source.Batch.RequestContractDigest != target.Batch.RequestContractDigest ||
		source.Batch.RouteID != target.Batch.RouteID || source.Batch.Budget != target.Batch.Budget {
		t.Fatalf("sealed replay source/target = %+v / %+v", source, target)
	}
	if err := value.ValidateAgainst(campaign, lock, design, armPlan, registry, replayed, bindings); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHeldOutCampaignBatchBinding(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatal("held-out campaign batch binding changed across strict JSON decoding")
	}
	schema, err := Schema("held-out-campaign-batch-binding")
	if err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	version := properties["schema_version"].(JSONSchema)
	if schema["additionalProperties"] != false || version["const"] != HeldOutCampaignBatchBindingSchemaVersion {
		t.Fatal("held-out campaign batch binding schema is open or unpinned")
	}

	t.Run("rejects validly replanned corpus substitution", func(t *testing.T) {
		candidate := heldOutBatchCandidate(t, campaign, armPlan, replayed, manifestDigest, "binding-model", "score-token-verifier")
		inputs := heldOutBatchInputs(candidate.Batch)
		inputs[0].Trajectories = append([]string(nil), inputs[0].Trajectories...)
		inputs[0].Trajectories[0] += "\nforeign trajectory content"
		candidate.Batch, err = candidate.Service.PlanBatch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildHeldOutCampaignArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, candidate); err == nil {
			t.Fatal("held-out batch binding accepted a validly replanned corpus substitution")
		}
	})

	t.Run("rejects missing capsule lineage", func(t *testing.T) {
		candidate := heldOutBatchCandidate(t, campaign, armPlan, replayed, manifestDigest, "binding-model", "explicit-text-judge")
		inputs := heldOutBatchInputs(candidate.Batch)
		inputs[0].Lineage.CapsuleDigest = ""
		candidate.Batch, err = candidate.Service.PlanBatch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildHeldOutCampaignArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, candidate); err == nil {
			t.Fatal("held-out batch binding accepted missing capsule lineage")
		}
	})

	t.Run("rejects replay route substitution", func(t *testing.T) {
		tampered := append([]HeldOutCampaignArmBatchBinding(nil), bindings...)
		index := heldOutArmBatchBindingIndex(t, tampered, heldOutReplayTargetArmID)
		candidate := heldOutBatchCandidate(t, campaign, armPlan, replayed, manifestDigest, "foreign-model", heldOutReplayTargetArmID)
		tampered[index], err = BuildHeldOutCampaignArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, candidate)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildHeldOutCampaignBatchBinding(campaign, lock, design, armPlan, registry, replayed, tampered); err == nil {
			t.Fatal("held-out batch binding accepted a replay route substitution")
		}
	})

	t.Run("rejects shared input contract drift", func(t *testing.T) {
		tamperedBindings := append([]HeldOutCampaignArmBatchBinding(nil), bindings...)
		candidate := heldOutBatchCandidate(t, campaign, armPlan, replayed, manifestDigest, "binding-model", "explicit-text-judge")
		inputs := heldOutBatchInputs(candidate.Batch)
		inputs[0].Criteria = append([]verifier.Criterion(nil), inputs[0].Criteria...)
		inputs[0].Criteria[0].Description = "A different scientific target."
		candidate.Batch, err = candidate.Service.PlanBatch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		tampered, err := BuildHeldOutCampaignArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, candidate)
		if err != nil {
			t.Fatal(err)
		}
		tamperedBindings[heldOutArmBatchBindingIndex(t, tamperedBindings, "explicit-text-judge")] = tampered
		if _, err := BuildHeldOutCampaignBatchBinding(campaign, lock, design, armPlan, registry, replayed, tamperedBindings); err == nil {
			t.Fatal("held-out batch binding accepted cross-arm criteria drift")
		}
	})

	t.Run("rejects shared live budget state", func(t *testing.T) {
		tamperedBindings := append([]HeldOutCampaignArmBatchBinding(nil), bindings...)
		candidate := heldOutBatchCandidate(t, campaign, armPlan, replayed, manifestDigest, "binding-model", "explicit-text-judge")
		inputs := heldOutBatchInputs(candidate.Batch)
		for index := range inputs {
			inputs[index].BudgetStatePath = "eval/runtime/held-out-score-token-verifier.json"
		}
		candidate.Batch, err = candidate.Service.PlanBatch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		tampered, err := BuildHeldOutCampaignArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, candidate)
		if err != nil {
			t.Fatal(err)
		}
		tamperedBindings[heldOutArmBatchBindingIndex(t, tamperedBindings, "explicit-text-judge")] = tampered
		if _, err := BuildHeldOutCampaignBatchBinding(campaign, lock, design, armPlan, registry, replayed, tamperedBindings); err == nil {
			t.Fatal("held-out batch binding accepted a shared live budget-state path")
		}
	})

	t.Run("rejects corpus digest tampering", func(t *testing.T) {
		tampered := append([]HeldOutCampaignArmBatchBinding(nil), bindings...)
		tampered[0].Batch.RawTrajectoryDigests = append([][]string(nil), tampered[0].Batch.RawTrajectoryDigests...)
		tampered[0].Batch.RawTrajectoryDigests[0] = append([]string(nil), tampered[0].Batch.RawTrajectoryDigests[0]...)
		tampered[0].Batch.RawTrajectoryDigests[0][0] = digestText("foreign-trajectory")
		if _, err := BuildHeldOutCampaignBatchBinding(campaign, lock, design, armPlan, registry, replayed, tampered); err == nil {
			t.Fatal("held-out batch binding accepted a foreign corpus digest")
		}
	})

	t.Run("rejects resealed authority promotion", func(t *testing.T) {
		promoted := value
		promoted.RunAuthorized = true
		promoted.ExecutionPermitIssued = true
		promoted.Digest = ""
		promoted.Digest, err = heldOutCampaignBatchBindingDigest(promoted)
		if err != nil {
			t.Fatal(err)
		}
		if err := promoted.Validate(); err == nil {
			t.Fatal("held-out batch binding promoted request previews into execution authority")
		}
	})
}

func heldOutBatchBindings(
	t *testing.T,
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	manifestDigest string,
	model string,
) []HeldOutCampaignArmBatchBinding {
	t.Helper()
	result := make([]HeldOutCampaignArmBatchBinding, 0, campaign.ProviderDependentArms)
	for _, campaignArm := range heldOutProviderCampaignArms(campaign) {
		candidate := heldOutBatchCandidate(t, campaign, armPlan, replayed, manifestDigest, model, campaignArm.ArmID)
		binding, err := BuildHeldOutCampaignArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, candidate)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, binding)
		runtime.GC()
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ArmID > result[right].ArmID })
	return result
}

func heldOutBatchCandidate(
	t *testing.T,
	campaign HeldOutCampaignPlan,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	manifestDigest string,
	model string,
	armID string,
) HeldOutCampaignBatchCandidate {
	t.Helper()
	campaignArm, exists := heldOutCampaignArmByID(campaign.Arms, armID)
	if !exists {
		t.Fatalf("held-out campaign arm %q not found", armID)
	}
	offline := campaignArm.ExecutionClass == HeldOutExecutionSealedProviderReplay
	service := heldOutBatchService(t, offline, model)
	expected, err := heldOutExpectedBatchInputs(campaignArm, armPlan, replayed)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]verification.Input, len(expected))
	for index, item := range expected {
		inputs[index] = verification.Input{
			Entrypoint: campaignArm.Entrypoint, Mode: heldOutVerificationMode(len(item.Trajectories)),
			Task:         "audit the correctness of this controlled coding-agent trajectory relation side",
			Trajectories: append([]string(nil), item.Trajectories...),
			Criteria: []verifier.Criterion{{
				ID: "correctness", Name: "Correctness", Description: "Assess the observed implementation and verification evidence for correctness.",
			}},
			Policy: verification.Policy{
				Evidence: campaignArm.EvidencePolicy, NReps: campaign.FixedRepetitions, Epsilon: 0.02,
				BiasMitigation: "disabled", InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise",
				MaxWorkers: 2, MaxPairCalls: 4, ConfidenceThreshold: 0.8,
			},
			StudyManifestDigest: manifestDigest, StudyVariant: item.Variant, DisableCache: true,
			Lineage: verification.LineageReferences{
				AuditCaseID: item.Cell.CaseID, TransformationID: item.Cell.RelationID, StudyCellID: item.Cell.CellID,
				OutcomeEvidenceDigest: item.OutcomeEvidenceDigest,
				ProfilePolicyDigest:   digestText("profile-policy"),
				CapsuleDigest:         digestText("capsule:" + campaignArm.ArmID + ":" + item.Cell.CellID + ":" + item.Variant),
			},
		}
		if !offline {
			inputs[index].BudgetStatePath = "eval/runtime/held-out-" + campaignArm.ArmID + ".json"
		}
	}
	batch, err := service.PlanBatch(inputs)
	if err != nil {
		t.Fatal(err)
	}
	return HeldOutCampaignBatchCandidate{ArmID: campaignArm.ArmID, Service: service, Batch: batch}
}

func heldOutBatchService(t *testing.T, offline bool, model string) *verification.Service {
	t.Helper()
	service, err := verification.NewService(verification.Config{
		PreprocessBudget: 0,
		RequestProfile: verification.RequestProfile{
			ProviderID: "held-out-provider", BaseURL: "https://held-out.invalid/v1", RequestedModel: model,
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK,
		},
		BudgetProfile: verification.BudgetProfile{MaxRetries: 1, MaxWorkers: 2, RequestTimeout: time.Second},
		Offline:       offline,
	}, func(context.Context, verification.Plan) (verification.Runtime, error) {
		return verification.Runtime{}, errors.New("held-out batch preflight test must not open a runtime")
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func heldOutBatchInputs(batch verification.BatchPlan) []verification.Input {
	result := make([]verification.Input, len(batch.Plans))
	for index, plan := range batch.Plans {
		result[index] = plan.Input
		result[index].Trajectories = append([]string(nil), plan.Input.Trajectories...)
		result[index].Criteria = append([]verifier.Criterion(nil), plan.Input.Criteria...)
	}
	return result
}

func heldOutArmBatchBindingIndex(t *testing.T, values []HeldOutCampaignArmBatchBinding, armID string) int {
	t.Helper()
	index := -1
	for bindingIndex, binding := range values {
		if binding.ArmID == armID {
			index = bindingIndex
		}
	}
	if index < 0 {
		t.Fatalf("held-out arm batch binding %q not found", armID)
	}
	return index
}
