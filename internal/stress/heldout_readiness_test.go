package stress

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
)

func TestHeldOutReadinessRefusalBindsCurrentBlockersWithoutExecutionAuthority(t *testing.T) {
	value, lock, design, armPlan, registry, replayed, owner := currentHeldOutReadinessRefusal(t)
	if value.Status != heldOutReadinessStatus || value.RunAuthorized || value.ExecutionPermitIssued ||
		value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired ||
		value.PassedGates != 1 || value.BlockedGates != 1 || value.MissingGates != 6 {
		t.Fatalf("held-out readiness refusal = %+v", value)
	}
	if value.Gates[1].ID != "owner_inspection" || value.Gates[1].Status != HeldOutReadinessBlocked ||
		value.Gates[1].Reason != "owner_inspection_overall_status_revision_required" {
		t.Fatalf("owner readiness gate = %+v", value.Gates[1])
	}
	if err := value.ValidateAgainst(lock, design, armPlan, registry, replayed, owner, owner.PackageInventoryDigest); err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHeldOutRunReadinessRefusal(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatal("held-out readiness refusal changed across strict JSON decoding")
	}
	rendered, err := RenderHeldOutRunReadinessRefusalMarkdown(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"Execution permit issued: `false`", "Provider calls: `0`", "`owner_inspection_overall_status_revision_required`", "Unsupported:"} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("held-out readiness Markdown lacks %q", fragment)
		}
	}

	t.Run("rejects resealed permit promotion", func(t *testing.T) {
		promoted := value
		promoted.RunAuthorized = true
		promoted.ExecutionPermitIssued = true
		promoted.Digest = ""
		promoted.Digest, err = heldOutRunReadinessRefusalDigest(promoted)
		if err != nil {
			t.Fatal(err)
		}
		if err := promoted.Validate(); err == nil {
			t.Fatal("resealed held-out readiness refusal promoted execution authority")
		}
	})

	t.Run("blocks cross-package owner projection", func(t *testing.T) {
		mismatched, err := BuildHeldOutRunReadinessRefusal(
			lock, design, armPlan, registry, replayed, owner, digestText("foreign-owner-package"),
		)
		if err != nil {
			t.Fatal(err)
		}
		if mismatched.Gates[1].Status != HeldOutReadinessBlocked || mismatched.ExpectedOwnerPackageDigest != digestText("foreign-owner-package") {
			t.Fatalf("cross-package owner gate = %+v", mismatched.Gates[1])
		}
	})
}

func TestHeldOutReadinessRefusalSchemaIsClosed(t *testing.T) {
	schema, err := Schema("held-out-run-readiness-refusal")
	if err != nil {
		t.Fatal(err)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("held-out readiness refusal schema permits unknown root properties")
	}
	properties := schema["properties"].(map[string]any)
	version := properties["schema_version"].(JSONSchema)
	if version["const"] != HeldOutRunReadinessRefusalSchemaVersion {
		t.Fatalf("held-out readiness refusal schema version = %v", version["const"])
	}
}

func currentHeldOutReadinessRefusal(t *testing.T) (
	HeldOutRunReadinessRefusal,
	HeldOutPartitionLock,
	StressAnalysisDesign,
	ArmComparisonPlan,
	RelationRegistry,
	[]ReplayedRelationCaseV3,
	relationevidence.OwnerInspectionPublicAttestation,
) {
	t.Helper()
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
	ownerFile := openFixture(t, filepath.Join("..", "..", defaultOwnerAttestationPathForTest))
	owner, err := relationevidence.DecodeOwnerInspectionPublicAttestation(ownerFile)
	if err != nil {
		t.Fatal(err)
	}
	value, err := BuildHeldOutRunReadinessRefusal(lock, design, armPlan, registry, replayed, owner, owner.PackageInventoryDigest)
	if err != nil {
		t.Fatal(err)
	}
	return value, lock, design, armPlan, registry, replayed, owner
}

const defaultOwnerAttestationPathForTest = "eval/results/relation-owner-inspection-attestation.json"
