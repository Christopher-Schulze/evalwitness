package study

import (
	"errors"
	"reflect"
	"strings"
	"time"
)

type JSONSchema map[string]any

func Schema(document string) (JSONSchema, error) {
	var root reflect.Type
	var identifier string
	switch document {
	case "manifest":
		root = reflect.TypeOf(Manifest{})
		identifier = "https://evalwitness.dev/schemas/study-manifest.v1.json"
	case "record":
		root = reflect.TypeOf(Record{})
		identifier = "https://evalwitness.dev/schemas/study-record.v1.json"
	case "split":
		root = reflect.TypeOf(SplitManifest{})
		identifier = "https://evalwitness.dev/schemas/study-split.v1.json"
	default:
		return nil, errors.New("schema type must be manifest, record, or split")
	}
	schema := schemaForType(root)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = identifier
	applySchemaConstants(document, schema)
	return schema, nil
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
		properties := make(map[string]any)
		required := make([]string, 0, value.NumField())
		for index := 0; index < value.NumField(); index++ {
			field := value.Field(index)
			tag := field.Tag.Get("json")
			if tag == "-" {
				continue
			}
			parts := strings.Split(tag, ",")
			name := parts[0]
			if name == "" {
				name = field.Name
			}
			properties[name] = schemaForType(field.Type)
			if !containsTag(parts[1:], "omitempty") {
				required = append(required, name)
			}
		}
		return JSONSchema{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
	case reflect.Slice:
		return JSONSchema{"type": "array", "items": schemaForType(value.Elem())}
	case reflect.String:
		return JSONSchema{"type": "string"}
	case reflect.Bool:
		return JSONSchema{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return JSONSchema{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return JSONSchema{"type": "number"}
	default:
		return JSONSchema{}
	}
}

func applySchemaConstants(document string, schema JSONSchema) {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	switch document {
	case "manifest":
		properties["schema_version"] = JSONSchema{"const": ManifestSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	case "record":
		properties["schema_version"] = JSONSchema{"const": RecordSchemaVersion}
	case "split":
		properties["schema_version"] = JSONSchema{"const": SplitSchemaVersion}
		properties["algorithm"] = JSONSchema{"const": splitAlgorithm}
	}
}

func enumValues(value reflect.Type) []string {
	switch value {
	case reflect.TypeOf(State("")):
		return []string{string(StateDraft), string(StateLocked), string(StateAuthorized), string(StateRunning), string(StateComplete), string(StateFailed), string(StateWithdrawn)}
	case reflect.TypeOf(DataRole("")):
		return []string{string(RoleDevelopment), string(RoleCalibration), string(RoleTest), string(RoleExternalReplication), string(RoleUnavailable)}
	case reflect.TypeOf(StudyKind("")):
		return []string{string(KindBenchmark), string(KindCalibration), string(KindControlledRelation), string(KindEvidenceReliance), string(KindRealAgentCorpus), string(KindTransfer), string(KindDrift)}
	default:
		return nil
	}
}

func containsTag(tags []string, wanted string) bool {
	for _, tag := range tags {
		if tag == wanted {
			return true
		}
	}
	return false
}
