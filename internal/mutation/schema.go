package mutation

import (
	"errors"
	"reflect"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

type JSONSchema map[string]any

func Schema(document string) (JSONSchema, error) {
	var root reflect.Type
	var identifier string
	switch document {
	case "manifest":
		root = reflect.TypeOf(Manifest{})
		identifier = "https://evalwitness.dev/schemas/mutation-manifest.v1.json"
	case "witness":
		root = reflect.TypeOf(Witness{})
		identifier = "https://evalwitness.dev/schemas/mutation-witness.v1.json"
	case "blind-review-packet":
		root = reflect.TypeOf(BlindReviewPacket{})
		identifier = "https://evalwitness.dev/schemas/blind-review-packet.v1.json"
	case "construct-firewall":
		root = reflect.TypeOf(ConstructFirewallReport{})
		identifier = "https://evalwitness.dev/schemas/construct-firewall.v1.json"
	case "construct-firewall-v2":
		root = reflect.TypeOf(ConstructFirewallReportV2{})
		identifier = "https://evalwitness.dev/schemas/construct-firewall.v2.json"
	case "construct-repair-evidence":
		root = reflect.TypeOf(ConstructRepairEvidence{})
		identifier = "https://evalwitness.dev/schemas/construct-repair-evidence.v1.json"
	case "construct-firewall-challenge":
		root = reflect.TypeOf(ConstructChallengeEvidence{})
		identifier = "https://evalwitness.dev/schemas/construct-firewall-challenge.v1.json"
	case "verification-evidence-assessment":
		root = reflect.TypeOf(VerificationEvidenceAssessment{})
		identifier = "https://evalwitness.dev/schemas/verification-evidence-assessment.v1.json"
	case "verification-evidence-challenge":
		root = reflect.TypeOf(VerificationEvidenceChallenge{})
		identifier = "https://evalwitness.dev/schemas/verification-evidence-challenge.v1.json"
	case "corpus-spec":
		root = reflect.TypeOf(CorpusSpec{})
		identifier = "https://evalwitness.dev/schemas/corruption-corpus-spec.v1.json"
	case "corpus-development-plan":
		root = reflect.TypeOf(CorpusDevelopmentPlan{})
		identifier = "https://evalwitness.dev/schemas/corruption-corpus-development-plan.v2.json"
	case "corpus-development-audit":
		root = reflect.TypeOf(CorpusDevelopmentAudit{})
		identifier = "https://evalwitness.dev/schemas/corruption-corpus-development-audit.v2.json"
	case "corpus-development-audit-v3":
		root = reflect.TypeOf(CorpusDevelopmentAuditV3{})
		identifier = "https://evalwitness.dev/schemas/corruption-corpus-development-audit.v3.json"
	case "corpus-release":
		root = reflect.TypeOf(CorpusRelease{})
		identifier = "https://evalwitness.dev/schemas/corruption-corpus-release.v1.json"
	case "corpus-release-v3":
		root = reflect.TypeOf(CorpusReleaseV3{})
		identifier = "https://evalwitness.dev/schemas/corruption-corpus-release.v3.json"
	case "reduction-witness":
		root = reflect.TypeOf(ReductionWitness{})
		identifier = "https://evalwitness.dev/schemas/mutation-reduction.v1.json"
	case "formal-control":
		root = reflect.TypeOf(FormalControlProgram{})
		identifier = "https://evalwitness.dev/schemas/formal-arithmetic-control.v1.json"
	default:
		return nil, errors.New("mutation schema type must be manifest, witness, blind-review-packet, construct-firewall, construct-firewall-v2, construct-repair-evidence, construct-firewall-challenge, verification-evidence-assessment, verification-evidence-challenge, corpus-spec, corpus-development-plan, corpus-development-audit, corpus-development-audit-v3, corpus-release, corpus-release-v3, reduction-witness, or formal-control")
	}
	schema := mutationSchemaForType(root)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = identifier
	properties := schema["properties"].(map[string]any)
	switch document {
	case "manifest":
		properties["schema_version"] = JSONSchema{"const": ManifestSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["relation_contract_version"] = JSONSchema{"enum": []string{RelationContractVersionV1, RelationContractVersionV2, RelationContractVersionV3}}
	case "witness":
		properties["schema_version"] = JSONSchema{"const": WitnessSchemaVersion}
	case "blind-review-packet":
		properties["schema_version"] = JSONSchema{"const": BlindPacketSchemaVersion}
	case "construct-firewall":
		properties["schema_version"] = JSONSchema{"const": ConstructFirewallSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["program_version"] = JSONSchema{"const": MutationProgramVersionV2}
	case "construct-firewall-v2":
		properties["schema_version"] = JSONSchema{"const": ConstructFirewallSchemaVersionV2}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["program_version"] = JSONSchema{"const": MutationProgramVersionV3}
	case "construct-repair-evidence":
		properties["schema_version"] = JSONSchema{"const": ConstructRepairEvidenceSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	case "construct-firewall-challenge":
		properties["schema_version"] = JSONSchema{"const": ConstructChallengeSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	case "verification-evidence-assessment":
		properties["schema_version"] = JSONSchema{"const": VerificationEvidenceAssessmentSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["classifier_version"] = JSONSchema{"const": VerificationEvidenceClassifierVersion}
	case "verification-evidence-challenge":
		properties["schema_version"] = JSONSchema{"const": VerificationEvidenceChallengeSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["classifier_version"] = JSONSchema{"const": VerificationEvidenceClassifierVersion}
	case "corpus-spec":
		properties["schema_version"] = JSONSchema{"const": CorpusSpecSchemaVersion}
	case "corpus-development-plan":
		properties["schema_version"] = JSONSchema{"enum": []string{CorpusDevelopmentPlanSchemaVersion, CorpusDevelopmentPlanSchemaVersionV3}}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	case "corpus-development-audit":
		properties["schema_version"] = JSONSchema{"const": CorpusDevelopmentAuditSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	case "corpus-development-audit-v3":
		properties["schema_version"] = JSONSchema{"const": CorpusDevelopmentAuditSchemaVersionV3}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	case "corpus-release":
		properties["schema_version"] = JSONSchema{"const": CorpusReleaseSchemaVersion}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["split_algorithm"] = JSONSchema{"const": CorpusSplitAlgorithm}
	case "corpus-release-v3":
		properties["schema_version"] = JSONSchema{"const": CorpusReleaseSchemaVersionV3}
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		properties["split_algorithm"] = JSONSchema{"const": CorpusSplitAlgorithm}
	case "reduction-witness":
		properties["schema_version"] = JSONSchema{"const": ReductionSchemaVersion}
	case "formal-control":
		properties["schema_version"] = JSONSchema{"const": FormalControlSchemaVersion}
	}
	return schema, nil
}

func mutationSchemaForType(value reflect.Type) JSONSchema {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if enum := mutationEnumValues(value); len(enum) > 0 {
		return JSONSchema{"type": "string", "enum": enum}
	}
	switch value.Kind() {
	case reflect.Struct:
		properties := make(map[string]any)
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
			properties[name] = mutationSchemaForType(field.Type)
			if !containsSchemaTag(parts[1:], "omitempty") {
				required = append(required, name)
			}
		}
		return JSONSchema{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
	case reflect.Slice, reflect.Array:
		return JSONSchema{"type": "array", "items": mutationSchemaForType(value.Elem())}
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

func mutationEnumValues(value reflect.Type) []string {
	switch value {
	case reflect.TypeOf(Family("")):
		result := make([]string, 0, len(definitions))
		for _, definition := range definitions {
			result = append(result, string(definition.Family))
		}
		return result
	case reflect.TypeOf(InterventionClass("")):
		return []string{string(ClassSemanticQuality), string(ClassPresentation), string(ClassEvidenceAvailability), string(ClassAdversarialClaim), string(ClassParserOnly)}
	case reflect.TypeOf(Relation("")):
		return []string{string(RelationOriginalBetter), string(RelationQualityEqual), string(RelationQualityEqualEvidenceLow), string(RelationVerifiedOutcomeWins), string(RelationNoControlEffect), string(RelationAmbiguous)}
	case reflect.TypeOf(LabelState("")):
		return []string{string(LabelProven), string(LabelAmbiguous), string(LabelInvalid)}
	case reflect.TypeOf(ValidationKind("")):
		return []string{string(ValidationFormal), string(ValidationHermetic), string(ValidationPreservation)}
	case reflect.TypeOf(ConstructStatus("")):
		return []string{string(ConstructApplied), string(ConstructRejected)}
	case reflect.TypeOf(ConstructRejectionReason("")):
		return []string{string(RejectionNoApplicableTarget), string(RejectionPreservationFailure), string(RejectionTemporalDependency), string(RejectionTokenSequenceChanged), string(RejectionTransactionDependency), string(RejectionUnnaturalFormatting), string(RejectionUnverifiedEvidenceRole)}
	case reflect.TypeOf(InvocationParseStatus("")):
		return []string{string(InvocationParsed), string(InvocationRejected)}
	case reflect.TypeOf(PresentationContentKind("")):
		return []string{string(PresentationAssistantProse), string(PresentationTerminalCommand), string(PresentationTerminalTranscript), string(PresentationCode), string(PresentationStructuredData), string(PresentationNonAssistantRole), string(PresentationUnknown)}
	case reflect.TypeOf(VerificationEvidenceStatus("")):
		return []string{string(VerificationEvidenceEligible), string(VerificationEvidenceRejected)}
	case reflect.TypeOf(VerificationEvidenceRejectionReason("")):
		return []string{string(VerificationEvidenceNoTarget), string(VerificationEvidenceInvocationUnverified), string(VerificationEvidenceProvenanceUnbound), string(VerificationEvidenceNonFailable), string(VerificationEvidenceNotWeakened)}
	case reflect.TypeOf(VerificationEvidenceContentKind("")):
		return []string{string(VerificationContentExecutionOutput), string(VerificationContentCommandMarker), string(VerificationContentMixedNarration), string(VerificationContentUnbound), string(VerificationContentAbsent)}
	case reflect.TypeOf(preprocess.SourceFormat("")):
		return []string{string(preprocess.SourcePlainText), string(preprocess.SourceClaudeCode), string(preprocess.SourceCodexRollout), string(preprocess.SourceOpenCode), string(preprocess.SourceTerminalBench), string(preprocess.SourceSWEbench), string(preprocess.SourceOTLPJSON), string(preprocess.SourceAgentTrace)}
	case reflect.TypeOf(study.DataRole("")):
		return []string{string(study.RoleDevelopment), string(study.RoleCalibration), string(study.RoleTest)}
	default:
		return nil
	}
}

func containsSchemaTag(tags []string, wanted string) bool {
	for _, tag := range tags {
		if tag == wanted {
			return true
		}
	}
	return false
}
