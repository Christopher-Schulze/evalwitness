package capsule

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	referenceTraceSourceType       = "evalwitness.trace-source.codex-rollout-jsonl.v1"
	referenceTraceBindingValidator = "evalwitness.validator.reference-trace-bindings.v1"
)

func referenceTraceTypes() []ComponentType {
	public := []Visibility{VisibilityPublic}
	exact := func(typeID string, role Role, binding string, mediaType string, rules ...ParentRule) ComponentType {
		return ComponentType{
			TypeID: typeID, SchemaID: typeID, Role: role, AllowedVisibilities: public, MediaType: mediaType,
			PayloadProfile: PayloadExactBytes, ValidatorID: referenceTraceValidatorID(typeID),
			BindingValidatorID: binding, ParentRules: rules,
		}
	}
	return []ComponentType{
		exact(lineage.SourceSpecificationRegistryVersion, RoleGovernance, "", "application/json"),
		exact(referenceTraceSourceType, RoleObservation, referenceTraceBindingValidator, "application/x-ndjson",
			parentRule(EdgeGovernedBy, lineage.SourceSpecificationRegistryVersion, 1, 1)),
		exact(preprocess.TraceEnvelopeSchema, RoleObservation, referenceTraceBindingValidator, "application/json",
			parentRule(EdgeGovernedBy, lineage.SourceSpecificationRegistryVersion, 1, 1),
			parentRule(EdgeObservedFrom, referenceTraceSourceType, 1, 1)),
		exact(preprocess.TraceMappingReportSchema, RoleDerivation, referenceTraceBindingValidator, "application/json",
			parentRule(EdgeDerivedFrom, referenceTraceSourceType, 1, 1),
			parentRule(EdgeGovernedBy, lineage.SourceSpecificationRegistryVersion, 1, 1)),
		exact(preprocess.CanonicalTrajectorySchema, RoleDerivation, referenceTraceBindingValidator, "application/json",
			parentRule(EdgeDerivedFrom, referenceTraceSourceType, 1, 1),
			parentRule(EdgeDerivedFrom, preprocess.TraceEnvelopeSchema, 1, 1),
			parentRule(EdgeDerivedFrom, preprocess.TraceMappingReportSchema, 1, 1),
			parentRule(EdgeGovernedBy, lineage.SourceSpecificationRegistryVersion, 1, 1)),
	}
}

func referenceTracePayloadValidators() map[string]PayloadValidator {
	return map[string]PayloadValidator{
		referenceTraceValidatorID(lineage.SourceSpecificationRegistryVersion): func(payload []byte) error {
			var registry lineage.TraceSourceSpecificationRegistry
			if err := decodeReferenceJSON(payload, &registry); err != nil {
				return err
			}
			return registry.Validate()
		},
		referenceTraceValidatorID(referenceTraceSourceType): func(payload []byte) error {
			result, err := preprocess.ImportTraceBytes(payload, preprocess.DefaultTraceImportOptions())
			if err != nil {
				return err
			}
			if result.Envelope.Source.Format != preprocess.SourceCodexRollout || result.Envelope.PrivacyClass != preprocess.PrivacyMetadataOnly {
				return errors.New("reference trace source is not the metadata-only Codex fixture contract")
			}
			return nil
		},
		referenceTraceValidatorID(preprocess.TraceEnvelopeSchema): func(payload []byte) error {
			var envelope preprocess.TraceEnvelope
			if err := decodeReferenceJSON(payload, &envelope); err != nil {
				return err
			}
			return preprocess.ValidateTraceEnvelope(envelope)
		},
		referenceTraceValidatorID(preprocess.TraceMappingReportSchema): func(payload []byte) error {
			var report preprocess.MappingReport
			if err := decodeReferenceJSON(payload, &report); err != nil {
				return err
			}
			return preprocess.ValidateTraceMappingReport(report)
		},
		referenceTraceValidatorID(preprocess.CanonicalTrajectorySchema): func(payload []byte) error {
			var trajectory preprocess.Trajectory
			if err := decodeReferenceJSON(payload, &trajectory); err != nil {
				return err
			}
			return trajectory.Validate()
		},
	}
}

func referenceTraceBindingValidators() map[string]BindingValidator {
	return map[string]BindingValidator{referenceTraceBindingValidator: validateReferenceTraceBindings}
}

func addReferenceTraceEvidence(repositoryRoot string, registry *Registry, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	specification, err := addReferenceFile(repositoryRoot, registry, "trace.source-specifications", lineage.SourceSpecificationRegistryVersion, VisibilityPublic, "eval/governance/trace-source-specifications-v1.json", nil, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	sourceRaw, err := readReferenceFile(repositoryRoot, "internal/preprocess/testdata/golden/codex-rollout.jsonl")
	if err != nil {
		return ComponentRecord{}, err
	}
	source, err := addReferencePayload(registry, ComponentInput{
		Name: "trace.codex-source", TypeID: referenceTraceSourceType, Visibility: VisibilityPublic,
		Payload: sourceRaw, Parents: []ParentRef{internalParentRef(EdgeGovernedBy, specification)},
	}, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	result, err := preprocess.ImportTraceBytes(sourceRaw, preprocess.DefaultTraceImportOptions())
	if err != nil {
		return ComponentRecord{}, err
	}
	envelopeRaw, err := encodeReferenceIndented(result.Envelope)
	if err != nil {
		return ComponentRecord{}, err
	}
	envelope, err := addReferencePayload(registry, ComponentInput{
		Name: "trace.codex-envelope", TypeID: preprocess.TraceEnvelopeSchema, Visibility: VisibilityPublic,
		Payload: envelopeRaw, Parents: []ParentRef{
			internalParentRef(EdgeGovernedBy, specification), internalParentRef(EdgeObservedFrom, source),
		},
	}, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	mappingRaw, err := encodeReferenceIndented(result.Mapping)
	if err != nil {
		return ComponentRecord{}, err
	}
	mapping, err := addReferencePayload(registry, ComponentInput{
		Name: "trace.codex-mapping", TypeID: preprocess.TraceMappingReportSchema, Visibility: VisibilityPublic,
		Payload: mappingRaw, Parents: []ParentRef{
			internalParentRef(EdgeGovernedBy, specification), internalParentRef(EdgeDerivedFrom, source),
		},
	}, records, payloads)
	if err != nil {
		return ComponentRecord{}, err
	}
	trajectoryRaw, err := encodeReferenceIndented(result.Trajectory)
	if err != nil {
		return ComponentRecord{}, err
	}
	return addReferencePayload(registry, ComponentInput{
		Name: "trace.codex-trajectory", TypeID: preprocess.CanonicalTrajectorySchema, Visibility: VisibilityPublic,
		Payload: trajectoryRaw, Parents: []ParentRef{
			internalParentRef(EdgeGovernedBy, specification), internalParentRef(EdgeDerivedFrom, source),
			internalParentRef(EdgeDerivedFrom, envelope), internalParentRef(EdgeDerivedFrom, mapping),
		},
	}, records, payloads)
}

func validateReferenceTraceBindings(context BindingContext) error {
	if _, err := uniqueReferenceParent(context, lineage.SourceSpecificationRegistryVersion); err != nil {
		return err
	}
	if context.Component.TypeID == referenceTraceSourceType {
		_, err := preprocess.ImportTraceBytes(context.Payload, preprocess.DefaultTraceImportOptions())
		return err
	}
	source, err := uniqueReferenceParent(context, referenceTraceSourceType)
	if err != nil {
		return err
	}
	expected, err := preprocess.ImportTraceBytes(source.Payload, preprocess.DefaultTraceImportOptions())
	if err != nil {
		return err
	}
	switch context.Component.TypeID {
	case preprocess.TraceEnvelopeSchema:
		var actual preprocess.TraceEnvelope
		if err := decodeReferenceJSON(context.Payload, &actual); err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, expected.Envelope) {
			return errors.New("trace envelope differs from its exact source import")
		}
	case preprocess.TraceMappingReportSchema:
		var actual preprocess.MappingReport
		if err := decodeReferenceJSON(context.Payload, &actual); err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, expected.Mapping) {
			return errors.New("trace mapping differs from its exact source import")
		}
	case preprocess.CanonicalTrajectorySchema:
		var actual preprocess.Trajectory
		if err := decodeReferenceJSON(context.Payload, &actual); err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, expected.Trajectory) {
			return errors.New("canonical trajectory differs from its exact source import")
		}
		envelope, err := uniqueReferenceParent(context, preprocess.TraceEnvelopeSchema)
		if err != nil {
			return err
		}
		mapping, err := uniqueReferenceParent(context, preprocess.TraceMappingReportSchema)
		if err != nil {
			return err
		}
		var parentEnvelope preprocess.TraceEnvelope
		var parentMapping preprocess.MappingReport
		if err := decodeReferenceJSON(envelope.Payload, &parentEnvelope); err != nil {
			return err
		}
		if err := decodeReferenceJSON(mapping.Payload, &parentMapping); err != nil {
			return err
		}
		if !reflect.DeepEqual(parentEnvelope, expected.Envelope) || !reflect.DeepEqual(parentMapping, expected.Mapping) {
			return errors.New("canonical trajectory parents differ from the exact source import")
		}
	default:
		return errors.New("unsupported reference trace binding type")
	}
	return nil
}

func encodeReferenceIndented(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func referenceTraceValidatorID(typeID string) string {
	return "evalwitness.validator.reference." + strings.TrimPrefix(typeID, "evalwitness.")
}
