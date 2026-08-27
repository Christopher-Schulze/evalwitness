package stress

import (
	"bytes"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestHeldOutExecutionBatchBindingContainsOnlyAdmissionEligibleProviderWork(t *testing.T) {
	_, lock, design, armPlan, registry, replayed, owner := currentHeldOutReadinessRefusal(t)
	campaign, err := BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	corpusPlan, corpusAudit, corpusRelease := currentCorpusV3(t)
	relationPlan, primarySample := currentRelationGovernanceV3(t)
	owner = passedHeldOutOwner(t, owner)
	ledger := supportedPrimaryLedgerV3(t, primarySample, corpusRelease)
	admission, err := BuildHeldOutAdmissionPlan(
		campaign, lock, design, armPlan, registry, replayed,
		corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample,
		owner, owner.PackageInventoryDigest, ledger,
	)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := digestText("held-out-study-manifest")
	bindings := heldOutExecutionBindings(t, campaign, lock, design, armPlan, registry, replayed, admission, manifestDigest, "binding-model")
	value, err := BuildHeldOutExecutionBatchBinding(campaign, lock, design, armPlan, registry, replayed, admission, bindings)
	if err != nil {
		t.Fatal(err)
	}
	if value.AdmissionPlanDigest != admission.Digest || value.ProviderEligibleCells != 213 ||
		value.LiveProviderEligibleCells != 142 || value.SealedReplayEligibleCells != 71 ||
		value.VerificationInputs != 426 || value.LiveVerificationInputs != 284 || value.SealedReplayVerificationInputs != 142 ||
		len(value.RequiredAuthorizationDigests) != 2 || value.ExecutionPermitIssued || value.RunAuthorized ||
		value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired {
		t.Fatalf("held-out execution binding = %+v", value)
	}
	eligible := stringSet(admission.ExecutionEligibleCellIDs)
	for _, cellID := range value.ProviderEligibleCellIDs {
		if _, admitted := eligible[cellID]; !admitted {
			t.Fatalf("execution binding contains pre-execution-ineligible cell %q", cellID)
		}
	}
	source, sourceOK := heldOutExecutionArmBindingByID(value.Arms, heldOutReplaySourceArmID)
	target, targetOK := heldOutExecutionArmBindingByID(value.Arms, heldOutReplayTargetArmID)
	if !sourceOK || !targetOK || source.Batch.Offline || !target.Batch.Offline ||
		source.Batch.RequestSetFingerprint != target.Batch.RequestSetFingerprint ||
		source.Batch.RequestContractDigest != target.Batch.RequestContractDigest ||
		source.Batch.RouteID != target.Batch.RouteID || source.Batch.Budget != target.Batch.Budget {
		t.Fatalf("admission-filtered sealed replay source/target = %+v / %+v", source, target)
	}
	if err := value.ValidateAgainst(campaign, lock, design, armPlan, registry, replayed, admission, bindings); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHeldOutExecutionBatchBinding(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatal("held-out execution binding changed across strict JSON decoding")
	}
	schema, err := Schema("held-out-execution-batch-binding")
	if err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	if schema["additionalProperties"] != false || properties["schema_version"].(JSONSchema)["const"] != HeldOutExecutionBatchBindingSchemaVersion {
		t.Fatal("held-out execution binding schema is open or unpinned")
	}

	t.Run("rejects the unfiltered structural workload", func(t *testing.T) {
		candidate := heldOutBatchCandidate(t, campaign, armPlan, replayed, manifestDigest, "binding-model", heldOutReplaySourceArmID)
		if _, err := BuildHeldOutExecutionArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, admission, candidate); err == nil {
			t.Fatal("held-out execution binding accepted all structurally supported inputs without admission filtering")
		}
	})

	t.Run("rejects pre-execution-ineligible cell substitution", func(t *testing.T) {
		candidate := heldOutEligibleBatchCandidate(t, campaign, armPlan, replayed, admission, manifestDigest, "binding-model", heldOutReplaySourceArmID)
		inputs := heldOutBatchInputs(candidate.Batch)
		ineligible := admission.PreExecutionIneligibleCellIDs[0]
		inputs[0].Lineage.StudyCellID = ineligible
		candidate.Batch, err = candidate.Service.PlanBatch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildHeldOutExecutionArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, admission, candidate); err == nil {
			t.Fatal("held-out execution binding accepted an ineligible study-cell substitution")
		}
	})

	t.Run("rejects shared live budget state", func(t *testing.T) {
		tampered := append([]HeldOutExecutionArmBatchBinding(nil), bindings...)
		candidate := heldOutEligibleBatchCandidate(t, campaign, armPlan, replayed, admission, manifestDigest, "binding-model", "explicit-text-judge")
		inputs := heldOutBatchInputs(candidate.Batch)
		for index := range inputs {
			inputs[index].BudgetStatePath = "eval/runtime/held-out-score-token-verifier.json"
		}
		candidate.Batch, err = candidate.Service.PlanBatch(inputs)
		if err != nil {
			t.Fatal(err)
		}
		tamperedBinding, err := BuildHeldOutExecutionArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, admission, candidate)
		if err != nil {
			t.Fatal(err)
		}
		tampered[heldOutExecutionBindingIndex(t, tampered, "explicit-text-judge")] = tamperedBinding
		if _, err := BuildHeldOutExecutionBatchBinding(campaign, lock, design, armPlan, registry, replayed, admission, tampered); err == nil {
			t.Fatal("held-out execution binding accepted shared live budget state")
		}
	})

	t.Run("rejects resealed authority promotion", func(t *testing.T) {
		promoted := value
		promoted.RunAuthorized = true
		promoted.ExecutionPermitIssued = true
		promoted.Digest = ""
		promoted.Digest, err = heldOutExecutionBatchBindingDigest(promoted)
		if err != nil {
			t.Fatal(err)
		}
		if err := promoted.Validate(); err == nil {
			t.Fatal("held-out execution binding promoted request previews into execution authority")
		}
	})
}

func heldOutExecutionBindings(
	t *testing.T,
	campaign HeldOutCampaignPlan,
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	armPlan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	manifestDigest string,
	model string,
) []HeldOutExecutionArmBatchBinding {
	t.Helper()
	result := make([]HeldOutExecutionArmBatchBinding, 0, campaign.ProviderDependentArms)
	for _, campaignArm := range heldOutProviderCampaignArms(campaign) {
		candidate := heldOutEligibleBatchCandidate(t, campaign, armPlan, replayed, admission, manifestDigest, model, campaignArm.ArmID)
		binding, err := BuildHeldOutExecutionArmBatchBinding(campaign, lock, design, armPlan, registry, replayed, admission, candidate)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, binding)
		runtime.GC()
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ArmID > result[right].ArmID })
	return result
}

func heldOutEligibleBatchCandidate(
	t *testing.T,
	campaign HeldOutCampaignPlan,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	manifestDigest string,
	model string,
	armID string,
) HeldOutCampaignBatchCandidate {
	return heldOutEligibleBatchCandidateForCapsule(t, campaign, armPlan, replayed, admission, manifestDigest, model, armID, "")
}

func heldOutEligibleBatchCandidateForCapsule(
	t *testing.T,
	campaign HeldOutCampaignPlan,
	armPlan ArmComparisonPlan,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	manifestDigest string,
	model string,
	armID string,
	capsuleDigest string,
) HeldOutCampaignBatchCandidate {
	t.Helper()
	projected, _, expected, err := heldOutEligibleProviderArm(campaign, armPlan, replayed, admission, armID)
	if err != nil {
		t.Fatal(err)
	}
	offline := projected.ExecutionClass == HeldOutExecutionSealedProviderReplay
	service := heldOutBatchService(t, offline, model)
	inputs := make([]verification.Input, len(expected))
	for index, item := range expected {
		boundCapsuleDigest := capsuleDigest
		if boundCapsuleDigest == "" {
			boundCapsuleDigest = digestText("capsule:" + projected.ArmID + ":" + item.Cell.CellID + ":" + item.Variant)
		}
		inputs[index] = verification.Input{
			Entrypoint: projected.Entrypoint, Mode: heldOutVerificationMode(len(item.Trajectories)),
			Task:         "audit the correctness of this controlled coding-agent trajectory relation side",
			Trajectories: append([]string(nil), item.Trajectories...),
			Criteria: []verifier.Criterion{{
				ID: "correctness", Name: "Correctness", Description: "Assess the observed implementation and verification evidence for correctness.",
			}},
			Policy: verification.Policy{
				Evidence: projected.EvidencePolicy, NReps: campaign.FixedRepetitions, Epsilon: 0.02,
				BiasMitigation: "disabled", InconsistencyPolicy: "flag-only", SelectionStrategy: "pairwise",
				MaxWorkers: 2, MaxPairCalls: 4, ConfidenceThreshold: 0.8,
			},
			StudyManifestDigest: manifestDigest, StudyVariant: item.Variant, DisableCache: true,
			Lineage: verification.LineageReferences{
				AuditCaseID: item.Cell.CaseID, TransformationID: item.Cell.RelationID, StudyCellID: item.Cell.CellID,
				OutcomeEvidenceDigest: item.OutcomeEvidenceDigest,
				ProfilePolicyDigest:   digestText("profile-policy"),
				CapsuleDigest:         boundCapsuleDigest,
			},
		}
		if !offline {
			inputs[index].BudgetStatePath = "eval/runtime/held-out-" + projected.ArmID + ".json"
		}
	}
	batch, err := service.PlanBatch(inputs)
	if err != nil {
		t.Fatal(err)
	}
	return HeldOutCampaignBatchCandidate{ArmID: projected.ArmID, Service: service, Batch: batch}
}

func heldOutExecutionBindingIndex(t *testing.T, values []HeldOutExecutionArmBatchBinding, armID string) int {
	t.Helper()
	for index, binding := range values {
		if binding.ArmID == armID {
			return index
		}
	}
	t.Fatalf("held-out execution arm binding %q not found", armID)
	return -1
}
