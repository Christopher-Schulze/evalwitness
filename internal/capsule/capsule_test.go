package capsule

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	testPlanType         = "example.plan.v1"
	testObservationType  = "example.observation.v1"
	testAnalysisType     = "example.analysis.v1"
	testPresentationType = "example.presentation.v1"
	testSecretType       = "example.secret.v1"
	testCommitmentType   = "example.commitment.v1"
)

func TestManifestSeparatesScientificAndPresentationIdentity(t *testing.T) {
	registry := testRegistry(t)
	plan, _ := buildTestComponent(t, registry, ComponentInput{Name: "plan", TypeID: testPlanType, Visibility: VisibilityPublic, Payload: []byte(`{"value":1}`)})
	observation, _ := buildTestComponent(t, registry, ComponentInput{
		Name: "observation", TypeID: testObservationType, Visibility: VisibilityPublic, Payload: []byte(`{"value":0.5}`),
		Parents: []ParentRef{internalParent(EdgeGovernedBy, plan)},
	})
	analysis, _ := buildTestComponent(t, registry, ComponentInput{
		Name: "analysis", TypeID: testAnalysisType, Visibility: VisibilityPublic, Payload: []byte(`{"count":1}`),
		Parents: []ParentRef{internalParent(EdgeGovernedBy, plan), internalParent(EdgeDerivedFrom, observation)},
	})
	firstView, _ := buildTestComponent(t, registry, ComponentInput{
		Name: "report", TypeID: testPresentationType, Visibility: VisibilityPublic, Payload: []byte("first view\n"),
		Parents: []ParentRef{internalParent(EdgeRenders, analysis)},
	})
	first := buildTestManifest(t, registry, []ComponentRecord{plan, observation, analysis, firstView}, analysis.ComponentID, firstView.ComponentID)

	secondView, _ := buildTestComponent(t, registry, ComponentInput{
		Name: "report", TypeID: testPresentationType, Visibility: VisibilityPublic, Payload: []byte("second view\n"),
		Parents: []ParentRef{internalParent(EdgeRenders, analysis)},
	})
	second := buildTestManifest(t, registry, []ComponentRecord{plan, observation, analysis, secondView}, analysis.ComponentID, secondView.ComponentID)
	if first.CapsuleID != second.CapsuleID {
		t.Fatalf("presentation changed scientific identity: %s != %s", first.CapsuleID, second.CapsuleID)
	}
	if first.ManifestDigest == second.ManifestDigest {
		t.Fatal("presentation change did not change the distribution manifest")
	}

	changedAnalysis, _ := buildTestComponent(t, registry, ComponentInput{
		Name: "analysis", TypeID: testAnalysisType, Visibility: VisibilityPublic, Payload: []byte(`{"count":2}`),
		Parents: []ParentRef{internalParent(EdgeGovernedBy, plan), internalParent(EdgeDerivedFrom, observation)},
	})
	changed := buildTestManifest(t, registry, []ComponentRecord{plan, observation, changedAnalysis}, changedAnalysis.ComponentID, "")
	if first.CapsuleID == changed.CapsuleID {
		t.Fatal("semantic analysis change preserved scientific identity")
	}
}

func TestPayloadProfilesKeepLegacyFloatsExact(t *testing.T) {
	registry := testRegistry(t)
	plan, canonical := buildTestComponent(t, registry, ComponentInput{
		Name: "plan", TypeID: testPlanType, Visibility: VisibilityPublic, Payload: []byte("{ \"value\" : 1 }"),
	})
	if !bytes.Equal(canonical, []byte(`{"value":1}`)) {
		t.Fatalf("canonical payload = %q", canonical)
	}
	observation, exact := buildTestComponent(t, registry, ComponentInput{
		Name: "observation", TypeID: testObservationType, Visibility: VisibilityPublic,
		Payload: []byte("{ \"value\" : 0.5 }\n"), Parents: []ParentRef{internalParent(EdgeGovernedBy, plan)},
	})
	if !bytes.Equal(exact, []byte("{ \"value\" : 0.5 }\n")) {
		t.Fatalf("exact payload changed: %q", exact)
	}
	if observation.Payload.Digest != protocol.DigestBytes(exact) {
		t.Fatal("exact payload digest does not bind file bytes")
	}
	if _, _, err := BuildComponent(registry, ComponentInput{
		Name: "invalid.plan", TypeID: testPlanType, Visibility: VisibilityPublic, Payload: []byte(`{"value":0.5}`),
	}); err == nil {
		t.Fatal("canonical capsule JSON accepted a floating-point value")
	}
}

func TestRegistryAndGraphFailClosed(t *testing.T) {
	registry := testRegistry(t)
	if _, _, err := BuildComponent(registry, ComponentInput{
		Name: "unknown", TypeID: "example.unknown.v1", Visibility: VisibilityPublic, Payload: []byte("x"),
	}); err == nil {
		t.Fatal("unknown component type was accepted")
	}

	badTypes := testComponentTypes()
	badTypes = append(badTypes, ComponentType{
		TypeID: "example.bad-observation.v1", SchemaID: "example.bad-observation.v1", Role: RoleObservation,
		AllowedVisibilities: []Visibility{VisibilityPublic}, MediaType: "application/json",
		PayloadProfile: PayloadCanonicalJSON, ValidatorID: "test.canonical-json.v1",
		ParentRules: []ParentRule{{
			Kind: EdgeDerivedFrom, ParentType: testAnalysisType, Minimum: 1, Maximum: 1,
			Resolutions: []ParentResolution{ParentInternal},
		}},
	})
	document, err := SealRegistry("example.bad-registry.v1", "", badTypes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewRegistry(document, testValidators()); err == nil {
		t.Fatal("registry accepted a role-inverting parent contract")
	}

	cycleID1 := "1111111111111111111111111111111111111111111111111111111111111111"
	cycleID2 := "2222222222222222222222222222222222222222222222222222222222222222"
	cycle := []ComponentRecord{
		{ComponentID: cycleID1, Parents: []ParentRef{{ComponentID: cycleID2, Resolution: ParentInternal}}},
		{ComponentID: cycleID2, Parents: []ParentRef{{ComponentID: cycleID1, Resolution: ParentInternal}}},
	}
	components := map[string]ComponentRecord{cycleID1: cycle[0], cycleID2: cycle[1]}
	if err := rejectCycles(cycle, components); err == nil {
		t.Fatal("cycle detector accepted a provenance cycle")
	}
}

func TestPublicCommitmentCanDeclareOmittedPrivateParent(t *testing.T) {
	registry := testRegistry(t)
	secret, _ := buildTestComponent(t, registry, ComponentInput{
		Name: "secret", TypeID: testSecretType, Visibility: VisibilityPrivate, Payload: []byte("private bytes"),
	})
	omitted := internalParent(EdgeCommitsTo, secret)
	omitted.Resolution = ParentOmitted
	omitted.OmissionClass = "restricted-source-bytes"
	commitment, _ := buildTestComponent(t, registry, ComponentInput{
		Name: "public.commitment", TypeID: testCommitmentType, Visibility: VisibilityPublic,
		Payload: []byte(`{"omission":"restricted-source-bytes"}`), Parents: []ParentRef{omitted},
	})
	manifest := buildTestManifest(t, registry, []ComponentRecord{commitment}, commitment.ComponentID, "")
	if err := manifest.Validate(registry); err != nil {
		t.Fatal(err)
	}

	invalid := commitment
	invalid.Parents[0].Kind = EdgeDerivedFrom
	invalid.ComponentID, _ = componentDigest(invalid)
	if _, err := BuildManifest(registry, ManifestInput{
		StudyID: "study.fixture", CellID: "cell.fixture", ScientificRoots: []string{invalid.ComponentID},
		Components: []ComponentRecord{invalid},
	}); err == nil {
		t.Fatal("omitted private parent was accepted on a non-projection edge")
	}
}

func TestRegistryExtensionCannotRedefineTypeOrValidator(t *testing.T) {
	base := testRegistry(t)
	extensionType := ComponentType{
		TypeID: "example.extension.v1", SchemaID: "example.extension.v1", Role: RoleGovernance,
		AllowedVisibilities: []Visibility{VisibilityPublic}, MediaType: "application/json",
		PayloadProfile: PayloadCanonicalJSON, ValidatorID: "test.extension-json.v1",
	}
	document, err := SealRegistry("example.extension-registry.v1", base.Digest(), []ComponentType{extensionType})
	if err != nil {
		t.Fatal(err)
	}
	extended, err := ExtendRegistry(base, document, map[string]PayloadValidator{
		"test.extension-json.v1": testValidators()["test.canonical-json.v1"],
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := extended.Lookup(extensionType.TypeID); !found {
		t.Fatal("valid registry extension type is missing")
	}

	redefinition := testComponentTypes()[0]
	redefinitionDocument, err := SealRegistry("example.redefinition.v1", base.Digest(), []ComponentType{redefinition})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExtendRegistry(base, redefinitionDocument, nil); err == nil {
		t.Fatal("registry extension redefined a base type")
	}
}

func TestRegistryValidatorsCannotMutateCallerEvidence(t *testing.T) {
	descriptor := ComponentType{
		TypeID: "example.isolated-validator.v1", SchemaID: "example.isolated-validator.v1", Role: RoleDerivation,
		AllowedVisibilities: []Visibility{VisibilityPublic}, MediaType: "application/octet-stream",
		PayloadProfile: PayloadExactBytes, ValidatorID: "test.mutating-payload.v1", BindingValidatorID: "test.mutating-binding.v1",
	}
	document, err := SealRegistry("example.isolated-registry.v1", "", []ComponentType{descriptor})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistryWithBindings(document, map[string]PayloadValidator{
		"test.mutating-payload.v1": func(payload []byte) error {
			payload[0] = 'x'
			return nil
		},
	}, map[string]BindingValidator{
		"test.mutating-binding.v1": func(context BindingContext) error {
			context.Component.Name = "mutated"
			context.Payload[0] = 'y'
			context.Parents[0].Record.Name = "mutated-parent"
			context.Parents[0].Payload[0] = 'z'
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("payload")
	if err := registry.ValidatePayload(descriptor.TypeID, payload); err != nil {
		t.Fatal(err)
	}
	if string(payload) != "payload" {
		t.Fatal("payload validator mutated caller-owned bytes")
	}
	context := BindingContext{
		Component: ComponentRecord{Name: "child", TypeID: descriptor.TypeID}, Payload: payload,
		Parents: []BoundParent{{Record: ComponentRecord{Name: "parent"}, Payload: []byte("parent-payload")}},
	}
	if err := registry.ValidateBindings(context); err != nil {
		t.Fatal(err)
	}
	if context.Component.Name != "child" || string(context.Payload) != "payload" ||
		context.Parents[0].Record.Name != "parent" || string(context.Parents[0].Payload) != "parent-payload" {
		t.Fatal("binding validator mutated caller-owned evidence")
	}
}

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	document, err := SealRegistry("example.registry.v1", "", testComponentTypes())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(document, testValidators())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func testComponentTypes() []ComponentType {
	public := []Visibility{VisibilityPublic}
	return []ComponentType{
		{TypeID: testPlanType, SchemaID: testPlanType, Role: RoleGovernance, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON, ValidatorID: "test.canonical-json.v1"},
		{TypeID: testObservationType, SchemaID: testObservationType, Role: RoleObservation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadExactBytes, ValidatorID: "test.exact-bytes.v1", ParentRules: []ParentRule{{Kind: EdgeGovernedBy, ParentType: testPlanType, Minimum: 1, Maximum: 1, Resolutions: []ParentResolution{ParentInternal}}}},
		{TypeID: testAnalysisType, SchemaID: testAnalysisType, Role: RoleDerivation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON, ValidatorID: "test.canonical-json.v1", ParentRules: []ParentRule{{Kind: EdgeDerivedFrom, ParentType: testObservationType, Minimum: 1, Maximum: 1, Resolutions: []ParentResolution{ParentInternal}}, {Kind: EdgeGovernedBy, ParentType: testPlanType, Minimum: 1, Maximum: 1, Resolutions: []ParentResolution{ParentInternal}}}},
		{TypeID: testPresentationType, SchemaID: testPresentationType, Role: RolePresentation, AllowedVisibilities: public, MediaType: "text/markdown", PayloadProfile: PayloadExactBytes, ValidatorID: "test.exact-bytes.v1", ParentRules: []ParentRule{{Kind: EdgeRenders, ParentType: testAnalysisType, Minimum: 1, Maximum: 1, Resolutions: []ParentResolution{ParentInternal}}}},
		{TypeID: testSecretType, SchemaID: testSecretType, Role: RoleObservation, AllowedVisibilities: []Visibility{VisibilityPrivate}, MediaType: "application/octet-stream", PayloadProfile: PayloadExactBytes, ValidatorID: "test.exact-bytes.v1"},
		{TypeID: testCommitmentType, SchemaID: testCommitmentType, Role: RoleCommitment, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON, ValidatorID: "test.canonical-json.v1", ParentRules: []ParentRule{{Kind: EdgeCommitsTo, ParentType: testSecretType, Minimum: 1, Maximum: 1, Resolutions: []ParentResolution{ParentInternal, ParentOmitted}}}},
	}
}

func testValidators() map[string]PayloadValidator {
	return map[string]PayloadValidator{
		"test.canonical-json.v1": func(payload []byte) error {
			canonical, err := protocol.CanonicalizeJSON(payload)
			if err != nil {
				return err
			}
			if !bytes.Equal(canonical, payload) {
				return errors.New("payload is not canonical JSON")
			}
			return nil
		},
		"test.exact-bytes.v1": func(payload []byte) error {
			if len(payload) == 0 {
				return errors.New("payload is empty")
			}
			return nil
		},
	}
}

func buildTestComponent(t *testing.T, registry *Registry, input ComponentInput) (ComponentRecord, []byte) {
	t.Helper()
	record, payload, err := BuildComponent(registry, input)
	if err != nil {
		t.Fatal(err)
	}
	return record, payload
}

func buildTestManifest(t *testing.T, registry *Registry, records []ComponentRecord, scientificRoot, presentationRoot string) Manifest {
	t.Helper()
	input := ManifestInput{
		StudyID: "study.fixture", CellID: "cell.fixture", ScientificRoots: []string{scientificRoot}, Components: slices.Clone(records),
	}
	if presentationRoot != "" {
		input.PresentationRoots = []string{presentationRoot}
	}
	manifest, err := BuildManifest(registry, input)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func internalParent(kind EdgeKind, record ComponentRecord) ParentRef {
	return ParentRef{
		Kind: kind, ComponentID: record.ComponentID, TypeID: record.TypeID, Role: record.Role,
		Visibility: record.Visibility, Resolution: ParentInternal,
	}
}
