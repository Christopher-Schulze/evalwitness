package capsule

import (
	"bytes"
	"errors"
	"reflect"
	"strings"

	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	referenceProtocolSchemaType       = "evalwitness.protocol.schema-artifact.v1"
	referenceRequiredFieldCorpusType  = "evalwitness.protocol.required-field-corpus.v1"
	referenceRequestCorpusType        = "evalwitness.request-fingerprint-corpus.v2"
	referenceProtocolBindingValidator = "evalwitness.validator.reference-protocol-bindings.v1"
)

func referenceProtocolTypes() []ComponentType {
	public := []Visibility{VisibilityPublic}
	exact := func(typeID string, role Role) ComponentType {
		return ComponentType{
			TypeID: typeID, SchemaID: typeID, Role: role, AllowedVisibilities: public,
			MediaType: "application/json", PayloadProfile: PayloadExactBytes, ValidatorID: referenceProtocolValidatorID(typeID),
		}
	}
	return []ComponentType{
		exact(referenceProtocolSchemaType, RoleGovernance),
		exact(protocolkit.VectorCorpusSchema, RoleGovernance),
		exact(referenceRequiredFieldCorpusType, RoleGovernance),
		exact(referenceRequestCorpusType, RoleGovernance),
		{TypeID: protocolkit.DescriptorSchema, SchemaID: protocolkit.DescriptorSchema, Role: RoleObservation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON, ValidatorID: referenceProtocolValidatorID(protocolkit.DescriptorSchema)},
		{TypeID: protocolkit.RunSchema, SchemaID: protocolkit.RunSchema, Role: RoleDerivation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: PayloadCanonicalJSON, ValidatorID: referenceProtocolValidatorID(protocolkit.RunSchema), BindingValidatorID: referenceProtocolBindingValidator, ParentRules: []ParentRule{
			parentRule(EdgeDerivedFrom, referenceProtocolSchemaType, len(protocolSchemaNames()), len(protocolSchemaNames())),
			parentRule(EdgeDerivedFrom, protocolkit.VectorCorpusSchema, 1, 1),
			parentRule(EdgeDerivedFrom, referenceRequiredFieldCorpusType, 1, 1),
			parentRule(EdgeDerivedFrom, referenceRequestCorpusType, 1, 1),
			parentRule(EdgeDerivedFrom, protocolkit.DescriptorSchema, 1, 1),
		}},
	}
}

func referenceProtocolPayloadValidators() map[string]PayloadValidator {
	return map[string]PayloadValidator{
		referenceProtocolValidatorID(referenceProtocolSchemaType): validateEmbeddedProtocolSchema,
		referenceProtocolValidatorID(protocolkit.VectorCorpusSchema): func(payload []byte) error {
			expected, err := protocolkit.ReadVectorArtifact("normative-cases.json")
			if err != nil || !bytes.Equal(payload, expected) {
				return errors.New("protocol normative vector payload differs from the embedded artifact")
			}
			return nil
		},
		referenceProtocolValidatorID(referenceRequiredFieldCorpusType): func(payload []byte) error {
			expected, err := protocolkit.ReadVectorArtifact("required-fields.json")
			if err != nil || !bytes.Equal(payload, expected) {
				return errors.New("protocol required-field payload differs from the embedded artifact")
			}
			return nil
		},
		referenceProtocolValidatorID(referenceRequestCorpusType): func(payload []byte) error {
			_, err := protocolkit.LoadNormativeCorpus(payload)
			return err
		},
		referenceProtocolValidatorID(protocolkit.DescriptorSchema): func(payload []byte) error {
			var descriptor protocolkit.EvaluatorDescriptor
			if err := protocolkit.DecodeStrict(payload, &descriptor); err != nil {
				return err
			}
			return protocolkit.ValidateDescriptor(descriptor)
		},
		referenceProtocolValidatorID(protocolkit.RunSchema): func(payload []byte) error {
			var run protocolkit.AuditRun
			if err := protocolkit.DecodeStrict(payload, &run); err != nil {
				return err
			}
			return protocolkit.ValidateAuditRun(run)
		},
	}
}

func referenceProtocolBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{referenceProtocolBindingValidator: validateReferenceProtocolBindings}
}

func addReferenceProtocolEvidence(repositoryRoot string, registry *Registry, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	parents := make([]ParentRef, 0, len(protocolSchemaNames())+4)
	for _, name := range protocolSchemaNames() {
		component, err := addReferenceFile(repositoryRoot, registry, "protocol.schema."+strings.TrimSuffix(name, ".schema.json"), referenceProtocolSchemaType, VisibilityPublic, "protocol/schemas/"+name, nil, records, payloads)
		if err != nil {
			return ComponentRecord{}, err
		}
		parents = append(parents, internalParentRef(EdgeDerivedFrom, component))
	}
	normative, err := addReferenceFile(repositoryRoot, registry, "protocol.vectors.normative", protocolkit.VectorCorpusSchema, VisibilityPublic, "protocol/vectors/normative-cases.json", nil, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	required, err := addReferenceFile(repositoryRoot, registry, "protocol.vectors.required-fields", referenceRequiredFieldCorpusType, VisibilityPublic, "protocol/vectors/required-fields.json", nil, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	requestRaw, err := readReferenceFile(repositoryRoot, "internal/provider/testdata/request-fingerprint-v2.json")
	if err != nil {
		return ComponentRecord{}, err
	}
	requests, err := addReferencePayload(registry, ComponentInput{
		Name: "protocol.vectors.request-fingerprints", TypeID: referenceRequestCorpusType,
		Visibility: VisibilityPublic, Payload: requestRaw,
	}, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	descriptorValue := (protocolkit.ReferenceEvaluator{}).Descriptor()
	descriptorRaw, err := protocolkit.CanonicalMarshal(descriptorValue)
	if err != nil {
		return ComponentRecord{}, err
	}
	descriptor, err := addReferencePayload(registry, ComponentInput{
		Name: "protocol.reference-evaluator", TypeID: protocolkit.DescriptorSchema,
		Visibility: VisibilityPublic, Payload: descriptorRaw,
	}, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	corpus, err := protocolkit.LoadNormativeCorpus(requestRaw)
	if err != nil {
		return ComponentRecord{}, err
	}
	runValue, err := protocolkit.RunReferenceCorpus(corpus)
	if err != nil {
		return ComponentRecord{}, err
	}
	runRaw, err := protocolkit.CanonicalMarshal(runValue)
	if err != nil {
		return ComponentRecord{}, err
	}
	parents = append(parents,
		internalParentRef(EdgeDerivedFrom, normative), internalParentRef(EdgeDerivedFrom, required),
		internalParentRef(EdgeDerivedFrom, requests), internalParentRef(EdgeDerivedFrom, descriptor),
	)
	return addReferencePayload(registry, ComponentInput{
		Name: "protocol.reference-audit-run", TypeID: protocolkit.RunSchema,
		Visibility: VisibilityPublic, Payload: runRaw, Parents: parents,
	}, records, payloads)
}

func validateReferenceProtocolBindings(context BindingContext) error {
	if err := validateProtocolSchemaParents(context); err != nil {
		return err
	}
	normative, err := uniqueReferenceParent(context, protocolkit.VectorCorpusSchema)
	if err != nil {
		return err
	}
	required, err := uniqueReferenceParent(context, referenceRequiredFieldCorpusType)
	if err != nil {
		return err
	}
	requests, err := uniqueReferenceParent(context, referenceRequestCorpusType)
	if err != nil {
		return err
	}
	descriptorParent, err := uniqueReferenceParent(context, protocolkit.DescriptorSchema)
	if err != nil {
		return err
	}
	expectedNormative, err := protocolkit.ReadVectorArtifact("normative-cases.json")
	if err != nil || !bytes.Equal(normative.Payload, expectedNormative) {
		return errors.New("protocol audit parent has the wrong normative vectors")
	}
	expectedRequired, err := protocolkit.ReadVectorArtifact("required-fields.json")
	if err != nil || !bytes.Equal(required.Payload, expectedRequired) {
		return errors.New("protocol audit parent has the wrong required-field vectors")
	}
	corpus, err := protocolkit.LoadNormativeCorpus(requests.Payload)
	if err != nil {
		return err
	}
	expectedRun, err := protocolkit.RunReferenceCorpus(corpus)
	if err != nil {
		return err
	}
	var actualRun protocolkit.AuditRun
	if err := protocolkit.DecodeStrict(context.Payload, &actualRun); err != nil {
		return err
	}
	if !reflect.DeepEqual(actualRun, expectedRun) {
		return errors.New("protocol audit run differs from its exact corpus replay")
	}
	var descriptor protocolkit.EvaluatorDescriptor
	if err := protocolkit.DecodeStrict(descriptorParent.Payload, &descriptor); err != nil {
		return err
	}
	if !reflect.DeepEqual(descriptor, expectedRun.Evaluator) {
		return errors.New("protocol audit run differs from its evaluator descriptor parent")
	}
	return nil
}

func validateProtocolSchemaParents(context BindingContext) error {
	wanted := make(map[string]string, len(protocolSchemaNames()))
	for _, name := range protocolSchemaNames() {
		wanted["protocol.schema."+strings.TrimSuffix(name, ".schema.json")] = name
	}
	seen := make(map[string]struct{}, len(wanted))
	for _, parent := range context.Parents {
		if parent.Record.TypeID != referenceProtocolSchemaType {
			continue
		}
		name, found := wanted[parent.Record.Name]
		if !found {
			return errors.New("protocol audit has an unknown schema parent")
		}
		expected, err := protocolkit.ReadSchemaArtifact(name)
		if err != nil || !bytes.Equal(parent.Payload, expected) {
			return errors.New("protocol audit schema parent differs from the embedded artifact")
		}
		seen[name] = struct{}{}
	}
	if len(seen) != len(wanted) {
		return errors.New("protocol audit does not bind the complete schema set")
	}
	return nil
}

func validateEmbeddedProtocolSchema(payload []byte) error {
	for _, name := range protocolSchemaNames() {
		expected, err := protocolkit.ReadSchemaArtifact(name)
		if err == nil && bytes.Equal(payload, expected) {
			return nil
		}
	}
	return errors.New("protocol schema payload differs from every embedded schema artifact")
}

func protocolSchemaNames() []string {
	return []string{
		"adapter-message.schema.json",
		"audit-case.schema.json",
		"audit-finding.schema.json",
		"audit-invocation.schema.json",
		"audit-run.schema.json",
		"capability-matrix.schema.json",
		"decision-evidence.schema.json",
		"evaluator-descriptor.schema.json",
		"invocation-result.schema.json",
		"reliability-extension.schema.json",
		"score-evidence.schema.json",
		"shared.schema.json",
	}
}

func referenceProtocolValidatorID(typeID string) string {
	return "evalwitness.validator.reference." + strings.TrimPrefix(typeID, "evalwitness.")
}
