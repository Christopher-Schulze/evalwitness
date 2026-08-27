package lineage

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultPlanMatchesCheckedInArtifact(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.Digest != LockedPlanDigest {
		t.Fatalf("verification-lineage plan digest = %s", plan.Digest)
	}
	encoded, err := EncodeIndented(plan)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := os.ReadFile("../../eval/governance/verification-lineage-plan-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, artifact) {
		t.Fatal("checked-in verification-lineage plan differs from the canonical default plan")
	}
	decoded, err := DecodePlan(bytes.NewReader(artifact))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, plan) {
		t.Fatal("decoded verification-lineage plan differs from the sealed plan")
	}
}

func TestPlanSchemaIsClosedVersionedAndTyped(t *testing.T) {
	schema, err := Schema("plan")
	if err != nil {
		t.Fatal(err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["additionalProperties"] != false {
		t.Fatal("verification-lineage plan schema is not closed Draft 2020-12")
	}
	properties := schema["properties"].(map[string]any)
	version := properties["schema_version"].(JSONSchema)
	if version["const"] != PlanSchemaVersion {
		t.Fatal("verification-lineage plan schema does not bind its version")
	}
	digest := properties["digest"].(JSONSchema)
	if digest["const"] != LockedPlanDigest {
		t.Fatal("verification-lineage plan schema does not bind the locked preregistration")
	}
	roles := properties["roles"].(JSONSchema)["items"].(JSONSchema)
	roleProperties := roles["properties"].(map[string]any)
	roleEnum := roleProperties["role"].(JSONSchema)["enum"].([]string)
	if !reflect.DeepEqual(roleEnum, []string{"adapter_development", "adversarial_challenge", "capture_calibration", "locked_test"}) {
		t.Fatalf("verification-lineage role enum = %v", roleEnum)
	}
	for _, spec := range documentSpecs() {
		documentSchema, schemaErr := Schema(spec.Name)
		if schemaErr != nil {
			t.Fatalf("schema %q: %v", spec.Name, schemaErr)
		}
		if documentSchema["additionalProperties"] != false {
			t.Fatalf("schema %q is not closed", spec.Name)
		}
		if spec.Name != "plan" {
			documentProperties := documentSchema["properties"].(map[string]any)
			header := documentProperties["header"].(JSONSchema)
			headerProperties := header["properties"].(map[string]any)
			if headerProperties["schema_version"].(JSONSchema)["const"] != spec.SchemaVersion {
				t.Fatalf("schema %q does not bind its version", spec.Name)
			}
		}
	}
	if _, err := Schema("unknown"); err == nil {
		t.Fatal("unknown lineage schema type was accepted")
	}
}

func TestSchemaInventoryFreezesTenObjectParentDAG(t *testing.T) {
	inventory, err := DefaultSchemaInventory()
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Documents) != 10 || inventory.Documents[0].DocumentType != "assessment" || inventory.Documents[9].DocumentType != "source" {
		t.Fatalf("unexpected schema inventory: %+v", inventory.Documents)
	}
	mutated := inventory
	mutated.Documents = append([]SchemaInventoryEntry(nil), inventory.Documents...)
	mutated.Documents[0].SchemaDigest = strings.Repeat("f", 64)
	mutated.Digest = ""
	mutated.Digest, err = inventoryDigest(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if err := mutated.Validate(); err == nil {
		t.Fatal("schema body substitution was accepted by the inventory")
	}
	mutated = inventory
	mutated.Documents = append([]SchemaInventoryEntry(nil), inventory.Documents...)
	mutated.Documents[0].ParentRequirements = append([]ParentRequirement(nil), inventory.Documents[0].ParentRequirements...)
	mutated.Documents[0].ParentRequirements[0].SameTaskGroup = false
	mutated.Digest, err = inventoryDigest(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if err := mutated.Validate(); err == nil {
		t.Fatal("parent-DAG mutation was accepted after resealing")
	}
}

func TestArtifactHeaderRejectsMissingCrossTaskAndUnexpectedParents(t *testing.T) {
	requirements := []ParentRequirement{{Relation: "source", SchemaVersions: []string{SourceSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}}
	header := ArtifactHeader{
		SchemaVersion: WitnessSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
		ObjectID: "witness-1", TaskID: "TASK-069", TaskGroupID: "group-1", DataRole: RoleAdapterDevelopment,
		PlanDigest: LockedPlanDigest, Digest: strings.Repeat("1", 64),
		Parents: []ParentRef{{Relation: "source", SchemaVersion: SourceSchemaVersion, ObjectID: "source-1", TaskID: "TASK-069", TaskGroupID: "group-1", Digest: strings.Repeat("2", 64)}},
	}
	if err := validateHeader(header, WitnessSchemaVersion, requirements); err != nil {
		t.Fatal(err)
	}
	crossTask := header
	crossTask.Parents = append([]ParentRef(nil), header.Parents...)
	crossTask.Parents[0].TaskID = "TASK-068"
	if err := validateHeader(crossTask, WitnessSchemaVersion, requirements); err == nil {
		t.Fatal("cross-task parent was accepted")
	}
	missingParent := header
	missingParent.Parents = nil
	if err := validateHeader(missingParent, WitnessSchemaVersion, requirements); err == nil {
		t.Fatal("missing required parent was accepted")
	}
	unexpected := header
	unexpected.Parents = append(append([]ParentRef(nil), header.Parents...), ParentRef{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "plan", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest})
	if err := validateHeader(unexpected, WitnessSchemaVersion, requirements); err == nil {
		t.Fatal("unexpected parent relation was accepted")
	}
}

func TestPlanRejectsResearcherDegreesOfFreedom(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*VerificationLineagePlan)
	}{
		{"outcome-dependent stopping", func(plan *VerificationLineagePlan) { plan.Stopping.OutcomeDependentStopping = true }},
		{"counts inspected before lock", func(plan *VerificationLineagePlan) { plan.Acquisition.CountInspectionState = "inspected" }},
		{"provider use", func(plan *VerificationLineagePlan) { plan.Acquisition.ProviderCallsAllowed = 1 }},
		{"agent launch", func(plan *VerificationLineagePlan) { plan.Acquisition.LaboratoryMayLaunchAgents = true }},
		{"test parser tuning", func(plan *VerificationLineagePlan) { plan.Roles[2].ParserChangesPermitted = true }},
		{"weakened test minimum", func(plan *VerificationLineagePlan) { plan.MinimumSupport.TestTaskGroups = 19 }},
		{"terminal precedence change", func(plan *VerificationLineagePlan) {
			plan.Missingness.TerminalStates[1], plan.Missingness.TerminalStates[2] = plan.Missingness.TerminalStates[2], plan.Missingness.TerminalStates[1]
		}},
		{"forbidden claim removed", func(plan *VerificationLineagePlan) { plan.Claims.Forbidden = plan.Claims.Forbidden[1:] }},
		{"research question drift", func(plan *VerificationLineagePlan) { plan.ResearchQuestions[0].Question += " changed" }},
		{"allowed claim scope drift", func(plan *VerificationLineagePlan) { plan.Claims.Allowed[0].MaximumScope += " changed" }},
		{"source provider permission", func(plan *VerificationLineagePlan) { plan.SourceClasses[0].LiveProviderUsePermitted = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := DefaultPlan()
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&plan)
			if _, err := SealPlan(plan); err == nil {
				t.Fatal("mutated verification-lineage plan was accepted")
			}
		})
	}
}

func TestPlanRejectsUnknownFieldsTrailingValuesAndDigestMutation(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(plan)
	if err != nil {
		t.Fatal(err)
	}
	unknown := bytes.Replace(encoded, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1)
	if _, err := DecodePlan(bytes.NewReader(unknown)); err == nil {
		t.Fatal("unknown verification-lineage plan field was accepted")
	}
	if _, err := DecodePlan(strings.NewReader(string(encoded) + "{}")); err == nil {
		t.Fatal("trailing verification-lineage JSON value was accepted")
	}
	plan.Digest = strings.Repeat("0", 64)
	if err := plan.Validate(); err == nil {
		t.Fatal("verification-lineage plan digest mutation was accepted")
	}
}
