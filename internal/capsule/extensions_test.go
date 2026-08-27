package capsule

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestExtensionConformancePackageFreezesFiveOwnerContracts(t *testing.T) {
	base := buildReferencePackage(t)
	baseDigest := base.Registry.Digest()
	extension, err := BuildExtensionConformancePackage(base)
	if err != nil {
		t.Fatal(err)
	}
	if base.Registry.Digest() != baseDigest {
		t.Fatal("extension construction mutated the sealed base registry")
	}
	if len(extension.Registry.Document().Types) != 56 || len(extension.Manifest.Components) != 5 || len(extension.Payloads) != 5 {
		t.Fatalf("extension package has %d types, %d components, and %d payloads", len(extension.Registry.Document().Types), len(extension.Manifest.Components), len(extension.Payloads))
	}
	if len(extension.Manifest.ParentCapsules) != 1 || extension.Manifest.ParentCapsules[0].CapsuleID != base.Manifest.CapsuleID {
		t.Fatalf("extension parent capsules = %+v", extension.Manifest.ParentCapsules)
	}
	for _, record := range extension.Manifest.Components {
		if len(record.Parents) == 0 {
			t.Fatalf("extension component %q has no evidence parent", record.Name)
		}
		for _, parent := range record.Parents {
			if parent.Resolution != ParentExternal || parent.CapsuleID != base.Manifest.CapsuleID {
				t.Fatalf("extension parent for %q = %+v", record.Name, parent)
			}
		}
	}
	if _, err := VerifyPackage(context.Background(), extension.Registry, extension.Manifest, extension.Payloads, VerificationOptions{MaximumVisibility: VisibilityPublic}); err != nil {
		t.Fatal(err)
	}
}

func TestExtensionFixturesRejectEmpiricalPromotionAndCrossCapsuleSubstitution(t *testing.T) {
	base := buildReferencePackage(t)
	extension, err := BuildExtensionConformancePackage(base)
	if err != nil {
		t.Fatal(err)
	}
	record := extension.Manifest.Components[0]
	raw := extension.Payloads[record.Payload.Digest]
	var fixture ExtensionConformanceFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	fixture.ProviderCallsRequired = 1
	mutated, err := protocol.CanonicalMarshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := BuildComponent(extension.Registry, ComponentInput{
		Name: record.Name, TypeID: record.TypeID, Visibility: record.Visibility,
		Payload: mutated, Parents: record.Parents,
	}); err == nil {
		t.Fatal("extension fixture accepted invented provider work")
	}

	parents := append([]ParentRef(nil), record.Parents...)
	for index := range parents {
		parents[index].CapsuleID = strings.Repeat("f", 64)
	}
	misbound, _, err := BuildComponent(extension.Registry, ComponentInput{
		Name: record.Name, TypeID: record.TypeID, Visibility: record.Visibility,
		Payload: raw, Parents: parents,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildManifest(extension.Registry, ManifestInput{
		StudyID: "task-050-extension-conformance", CellID: "cross-capsule-substitution",
		ParentCapsules:  []CapsuleRef{{Relation: "extends", CapsuleID: base.Manifest.CapsuleID}},
		ScientificRoots: []string{misbound.ComponentID}, PresentationRoots: []string{}, Components: []ComponentRecord{misbound},
	}); err == nil {
		t.Fatal("extension manifest accepted an undeclared external parent capsule")
	}
}

func TestExtensionRegistryCannotRedefineBaseOrReferenceUnknownParent(t *testing.T) {
	base := buildReferencePackage(t)
	baseType := base.Registry.Document().Types[0]
	redefinition, err := SealRegistry("evalwitness.invalid-redefinition.v1", base.Registry.Digest(), []ComponentType{baseType})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExtendRegistry(base.Registry, redefinition, map[string]PayloadValidator{}); err == nil {
		t.Fatal("extension registry redefined a sealed base type")
	}

	const validatorID = "evalwitness.validator.unknown-parent-fixture.v1"
	unknownType := ComponentType{
		TypeID: "evalwitness.extension.unknown-parent.v1", SchemaID: "evalwitness.extension.unknown-parent.v1",
		Role: RoleDerivation, AllowedVisibilities: []Visibility{VisibilityPublic},
		MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON, ValidatorID: validatorID,
		ParentRules: []ParentRule{{
			Kind: EdgeDerivedFrom, ParentType: "evalwitness.missing-parent.v1", Minimum: 1, Maximum: 1,
			Resolutions: []ParentResolution{ParentExternal},
		}},
	}
	unknown, err := SealRegistry("evalwitness.invalid-unknown-parent.v1", base.Registry.Digest(), []ComponentType{unknownType})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExtendRegistry(base.Registry, unknown, map[string]PayloadValidator{validatorID: func([]byte) error { return nil }}); err == nil {
		t.Fatal("extension registry accepted an unknown parent type")
	}
}
