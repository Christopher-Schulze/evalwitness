package lineage

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"time"
)

type JSONSchema map[string]any

func Schema(document string) (JSONSchema, error) {
	spec, found := documentSpecByName(document)
	if !found {
		return nil, errors.New("unsupported lineage schema type")
	}
	schema := schemaForType(spec.GoType)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = schemaID(spec.SchemaVersion)
	properties := schema["properties"].(map[string]any)
	if document == "plan" {
		properties["schema_version"] = JSONSchema{"const": PlanSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["protocol_version"] = JSONSchema{"const": ProtocolVersion}
		properties["study_governance_policy"] = JSONSchema{"const": StudyGovernancePolicy}
		properties["digest"] = JSONSchema{"const": LockedPlanDigest}
	} else {
		header := properties["header"].(JSONSchema)
		headerProperties := header["properties"].(map[string]any)
		headerProperties["schema_version"] = JSONSchema{"const": spec.SchemaVersion}
		headerProperties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		headerProperties["protocol_version"] = JSONSchema{"const": ProtocolVersion}
	}
	return schema, nil
}

func schemaID(schemaVersion string) string {
	return "https://evalwitness.dev/schemas/" + strings.TrimPrefix(schemaVersion, "evalwitness.") + ".json"
}

type documentSpec struct {
	Name          string
	SchemaVersion string
	GoType        reflect.Type
	Parents       []ParentRequirement
}

func documentSpecs() []documentSpec {
	return []documentSpec{
		{Name: "assessment", SchemaVersion: AssessmentSchemaVersion, GoType: reflect.TypeOf(LineageAssessment{}), Parents: []ParentRequirement{{Relation: "candidate", SchemaVersions: []string{CandidateSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}, {Relation: "witness", SchemaVersions: []string{WitnessSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}}},
		{Name: "audit", SchemaVersion: AuditSchemaVersion, GoType: reflect.TypeOf(LineageAudit{}), Parents: []ParentRequirement{{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}, {Relation: "assessment", SchemaVersions: []string{AssessmentSchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true}, {Relation: "capability", SchemaVersions: []string{CapabilitySchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true}}},
		{Name: "bom", SchemaVersion: BOMSchemaVersion, GoType: reflect.TypeOf(VerificationEvidenceBOM{}), Parents: []ParentRequirement{{Relation: "source", SchemaVersions: []string{SourceSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}, {Relation: "witness", SchemaVersions: []string{WitnessSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}, {Relation: "candidate", SchemaVersions: []string{CandidateSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}, {Relation: "assessment", SchemaVersions: []string{AssessmentSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}, {Relation: "audit", SchemaVersions: []string{AuditSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}}},
		{Name: "candidate", SchemaVersion: CandidateSchemaVersion, GoType: reflect.TypeOf(LineageCandidate{}), Parents: []ParentRequirement{{Relation: "source", SchemaVersions: []string{SourceSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}, {Relation: "witness", SchemaVersions: []string{WitnessSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}}},
		{Name: "capability-vector", SchemaVersion: CapabilitySchemaVersion, GoType: reflect.TypeOf(TraceCapabilityVector{}), Parents: []ParentRequirement{{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}}},
		{Name: "dataset-card", SchemaVersion: DatasetCardSchemaVersion, GoType: reflect.TypeOf(VerificationLineageDatasetCard{}), Parents: []ParentRequirement{{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}, {Relation: "audit", SchemaVersions: []string{AuditSchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true}, {Relation: "capability", SchemaVersions: []string{CapabilitySchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true}}},
		{Name: "execution-witness", SchemaVersion: WitnessSchemaVersion, GoType: reflect.TypeOf(ExecutionWitness{}), Parents: []ParentRequirement{{Relation: "source", SchemaVersions: []string{SourceSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true, SameTaskGroup: true}}},
		{Name: "plan", SchemaVersion: PlanSchemaVersion, GoType: reflect.TypeOf(VerificationLineagePlan{})},
		{Name: "release", SchemaVersion: ReleaseSchemaVersion, GoType: reflect.TypeOf(VerificationLineageRelease{}), Parents: []ParentRequirement{{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}, {Relation: "audit", SchemaVersions: []string{AuditSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}, {Relation: "bom", SchemaVersions: []string{BOMSchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true}, {Relation: "dataset_card", SchemaVersions: []string{DatasetCardSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}}},
		{Name: "source", SchemaVersion: SourceSchemaVersion, GoType: reflect.TypeOf(VerificationLineageSource{}), Parents: []ParentRequirement{{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true}}},
	}
}

func documentSpecByName(name string) (documentSpec, bool) {
	specs := documentSpecs()
	index := slices.IndexFunc(specs, func(spec documentSpec) bool { return spec.Name == name })
	if index < 0 {
		return documentSpec{}, false
	}
	return specs[index], true
}

func schemaForType(value reflect.Type) JSONSchema {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value == reflect.TypeOf(time.Time{}) {
		return JSONSchema{"type": "string", "format": "date-time"}
	}
	if values := enumValues(value); len(values) > 0 {
		return JSONSchema{"type": "string", "enum": values}
	}
	switch value.Kind() {
	case reflect.Struct:
		return schemaForStruct(value)
	case reflect.Slice:
		return JSONSchema{"type": "array", "items": schemaForType(value.Elem())}
	case reflect.String:
		return JSONSchema{"type": "string"}
	case reflect.Bool:
		return JSONSchema{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return JSONSchema{"type": "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return JSONSchema{"type": "integer", "minimum": 0}
	case reflect.Float32, reflect.Float64:
		return JSONSchema{"type": "number"}
	default:
		return JSONSchema{}
	}
}

func schemaForStruct(value reflect.Type) JSONSchema {
	properties := make(map[string]any, value.NumField())
	required := make([]string, 0, value.NumField())
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		parts := strings.Split(field.Tag.Get("json"), ",")
		name := parts[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		properties[name] = schemaForType(field.Type)
		if !slicesContain(parts[1:], "omitempty") {
			required = append(required, name)
		}
	}
	return JSONSchema{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}

func enumValues(value reflect.Type) []string {
	switch value {
	case reflect.TypeOf(DataRole("")):
		return []string{string(RoleAdapterDevelopment), string(RoleAdversarialChallenge), string(RoleCaptureCalibration), string(RoleLockedTest)}
	case reflect.TypeOf(TerminalState("")):
		return []string{
			string(StateInvalidCapture), string(StateBehaviorAbsent), string(StateExportObservabilityAbsent),
			string(StateAdapterMappingLoss), string(StateUnsupportedShell), string(StateAmbiguousLineage),
			string(StateNonFailableVerification), string(StateClaimSpecificEvidenceNotWeakened),
			string(StateFreshnessUnresolved), string(StateDirectVerificationInvocation),
		}
	case reflect.TypeOf(StateDisposition("")):
		return []string{string(DispositionExcluded), string(DispositionLoss), string(DispositionIneligible), string(DispositionEligible)}
	case reflect.TypeOf(HoldoutKind("")):
		return []string{string(HoldoutFormat), string(HoldoutSyntaxFamily)}
	case reflect.TypeOf(CaptureMode("")):
		return []string{string(CaptureNativeExport), string(CapturePaired)}
	case reflect.TypeOf(StreamState("")):
		return []string{string(StreamAbsent), string(StreamCaptured), string(StreamTruncated)}
	case reflect.TypeOf(CapturePurpose("")):
		return []string{string(CaptureExternalObservation), string(CaptureSyntheticFixtureGeneration)}
	case reflect.TypeOf(ProvenanceClass("")):
		return []string{string(ProvenanceNarration), string(ProvenanceSynthesized), string(ProvenanceExitStatus), string(ProvenanceStderr), string(ProvenanceStdout), string(ProvenanceStructured)}
	case reflect.TypeOf(CapabilityState("")):
		return []string{string(CapabilityAbsent), string(CapabilityMalformed), string(CapabilityNotObserved), string(CapabilityObserved), string(CapabilityOptional), string(CapabilityRedacted), string(CapabilityRequired), string(CapabilityUnspecified), string(CapabilityUnsupported)}
	case reflect.TypeOf(FreshnessState("")):
		return []string{string(FreshnessClosed), string(FreshnessCurrent), string(FreshnessUnresolved)}
	default:
		return nil
	}
}

func slicesContain(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
