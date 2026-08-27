package stress

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestHeldOutCampaignPlanLocksExactWorkloadWithoutExecutionAuthority(t *testing.T) {
	_, lock, design, armPlan, registry, replayed, _ := currentHeldOutReadinessRefusal(t)
	value, err := BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	if value.ProviderDependentArms != 3 || value.LiveProviderArms != 2 || value.SealedReplayArms != 1 || value.ZeroCostArms != 7 ||
		value.ProviderDependentSupportedTestCells != 342 || value.LiveProviderSupportedTestCells != 228 ||
		value.SealedReplaySupportedTestCells != 114 || value.ZeroCostSupportedTestCells != 98 ||
		value.ProviderDependentVerificationInputs != 684 || value.LiveProviderVerificationInputs != 456 ||
		value.SealedReplayVerificationInputs != 228 || value.PlannedProviderDependentSideRepetitions != 2052 ||
		value.PlannedLiveProviderSideRepetitions != 1368 || value.PlannedSealedReplaySideRepetitions != 684 ||
		value.PlannedZeroCostRepetitions != 294 ||
		value.FixedRepetitions != 3 || value.RunAuthorized || value.ExecutionPermitIssued ||
		value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired {
		t.Fatalf("held-out campaign plan = %+v", value)
	}
	classes := map[string]HeldOutCampaignExecutionClass{}
	for _, arm := range value.Arms {
		classes[arm.ArmID] = arm.ExecutionClass
		if arm.ProviderDependent {
			if arm.TestCells != 114 || arm.SupportedTestCells != 114 || arm.UnsupportedTestCells != 0 ||
				arm.ProviderVerificationInputs != 228 || arm.PlannedProviderSideRepetitions != 684 {
				t.Fatalf("provider campaign arm = %+v", arm)
			}
			continue
		}
		if arm.TestCells != 114 || arm.SupportedTestCells != 14 || arm.UnsupportedTestCells != 100 ||
			arm.ProviderVerificationInputs != 0 || arm.PlannedZeroCostRepetitions != 42 {
			t.Fatalf("zero-cost campaign arm = %+v", arm)
		}
	}
	if classes["explicit-text-judge"] != HeldOutExecutionLiveProvider ||
		classes["score-token-verifier"] != HeldOutExecutionLiveProvider ||
		classes["external-protocol-adapter"] != HeldOutExecutionSealedProviderReplay ||
		classes["zero-cost-first-listed"] != HeldOutExecutionDeterministicLocal {
		t.Fatalf("held-out campaign execution classes = %+v", classes)
	}
	if err := value.ValidateAgainst(lock, design, armPlan, registry, replayed); err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHeldOutCampaignPlan(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatal("held-out campaign plan changed across strict JSON decoding")
	}
	rendered, err := RenderHeldOutCampaignPlanMarkdown(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"Live-provider workload: `2` arms", "Sealed-replay workload: `1` arm", "`684` verification inputs", "`2052` registered side repetitions", "Execution permit issued: `false`", "provider request count"} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("held-out campaign Markdown lacks %q", fragment)
		}
	}

	t.Run("rejects live-binding promotion", func(t *testing.T) {
		promoted := value
		promoted.LiveBindings.LiveProviderRequestPlansBound = true
		promoted.Digest = ""
		promoted.Digest, err = heldOutCampaignPlanDigest(promoted)
		if err != nil {
			t.Fatal(err)
		}
		if err := promoted.Validate(); err == nil {
			t.Fatal("held-out campaign topology promoted a missing provider request plan")
		}
	})

	t.Run("rejects protocol adapter live promotion", func(t *testing.T) {
		promoted := value
		promoted.Arms = append([]HeldOutCampaignArm(nil), value.Arms...)
		for index := range promoted.Arms {
			if promoted.Arms[index].ArmID == "external-protocol-adapter" {
				promoted.Arms[index].ExecutionClass = HeldOutExecutionLiveProvider
			}
		}
		promoted.Digest = ""
		promoted.Digest, err = heldOutCampaignPlanDigest(promoted)
		if err != nil {
			t.Fatal(err)
		}
		if err := promoted.Validate(); err == nil {
			t.Fatal("held-out campaign promoted the offline protocol adapter to live provider execution")
		}
	})

	t.Run("rejects resealed cell-set substitution", func(t *testing.T) {
		tampered := value
		tampered.Arms = append([]HeldOutCampaignArm(nil), value.Arms...)
		tampered.Arms[0].SupportedTestCellSetDigest = digestText("foreign-cell-set")
		tampered.Digest = ""
		tampered.Digest, err = heldOutCampaignPlanDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if err := tampered.ValidateAgainst(lock, design, armPlan, registry, replayed); err == nil {
			t.Fatal("held-out campaign plan accepted a resealed foreign cell set")
		}
	})
}

func TestHeldOutCampaignPlanSchemaIsClosed(t *testing.T) {
	schema, err := Schema("held-out-campaign-plan")
	if err != nil {
		t.Fatal(err)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("held-out campaign plan schema permits unknown root properties")
	}
	properties := schema["properties"].(map[string]any)
	version := properties["schema_version"].(JSONSchema)
	if version["const"] != HeldOutCampaignPlanSchemaVersion {
		t.Fatalf("held-out campaign plan schema version = %v", version["const"])
	}
}
