package outcome

import (
	"errors"
	"reflect"
	"strings"
)

type JSONSchema map[string]any

func Schema(document string) (JSONSchema, error) {
	types := map[string]reflect.Type{
		"plan": reflect.TypeOf(Plan{}), "record": reflect.TypeOf(Record{}), "record-draft": reflect.TypeOf(RecordDraft{}), "evidence": reflect.TypeOf(Evidence{}), "evidence-draft": reflect.TypeOf(EvidenceDraft{}),
		"blind-packet": reflect.TypeOf(BlindPacket{}), "private-mapping": reflect.TypeOf(PrivateMapping{}), "label": reflect.TypeOf(Label{}),
		"blind-build-request": reflect.TypeOf(BlindBuildRequest{}),
		"resolution":          reflect.TypeOf(Resolution{}), "agreement": reflect.TypeOf(AgreementReport{}), "preservation": reflect.TypeOf(Preservation{}),
		"sample-commitment":         reflect.TypeOf(SampleCommitment{}),
		"pilot-sample-v1":           reflect.TypeOf(PilotSampleCommitment{}),
		"pilot-readiness-v1":        reflect.TypeOf(PilotReadiness{}),
		"pilot-sample":              reflect.TypeOf(OutcomePilotSampleCommitment{}),
		"pilot-readiness":           reflect.TypeOf(OutcomePilotReadiness{}),
		"pilot-source-binding":      reflect.TypeOf(OutcomePilotSourceBinding{}),
		"pilot-private-materials":   reflect.TypeOf(OutcomePilotPrivateMaterials{}),
		"pilot-inspection":          reflect.TypeOf(OutcomePilotInspection{}),
		"natural-inventory-request": reflect.TypeOf(NaturalInventoryRequest{}), "natural-inventory": reflect.TypeOf(NaturalInventory{}),
		"executable-log":    reflect.TypeOf(ExecutionLog{}),
		"qualification-set": reflect.TypeOf(QualificationSet{}), "qualification-report": reflect.TypeOf(QualificationReport{}),
		"label-draft":               reflect.TypeOf(LabelDraft{}),
		"review-bundle":             reflect.TypeOf(ReviewBundle{}),
		"reviewer-record":           reflect.TypeOf(ReviewerRecord{}),
		"review-assignment":         reflect.TypeOf(ReviewAssignment{}),
		"label-batch":               reflect.TypeOf(LabelBatch{}),
		"mapping-reveal":            reflect.TypeOf(MappingReveal{}),
		"adjudication-ledger":       reflect.TypeOf(AdjudicationLedger{}),
		"reviewer-handbook":         reflect.TypeOf(ReviewerHandbook{}),
		"reviewer-kit":              reflect.TypeOf(ReviewerKit{}),
		"blinding-protocol":         reflect.TypeOf(BlindingProtocol{}),
		"blinding-probe":            reflect.TypeOf(BlindingProbe{}),
		"blinding-probe-batch":      reflect.TypeOf(BlindingProbeBatch{}),
		"blinding-analysis":         reflect.TypeOf(BlindingAnalysis{}),
		"rubric-ambiguity-analysis": reflect.TypeOf(RubricAmbiguityAnalysis{}),
		"source-audit":              reflect.TypeOf(OutcomeSourceAudit{}),
	}
	root, exists := types[document]
	if !exists {
		return nil, errors.New("outcome schema type must be plan, record, record-draft, evidence, evidence-draft, blind-build-request, blind-packet, private-mapping, label, label-draft, resolution, agreement, preservation, sample-commitment, pilot-sample-v1, pilot-readiness-v1, pilot-sample, pilot-readiness, pilot-source-binding, pilot-private-materials, pilot-inspection, natural-inventory-request, natural-inventory, executable-log, qualification-set, qualification-report, review-bundle, reviewer-record, review-assignment, label-batch, mapping-reveal, adjudication-ledger, reviewer-handbook, reviewer-kit, blinding-protocol, blinding-probe, blinding-probe-batch, blinding-analysis, rubric-ambiguity-analysis, or source-audit")
	}
	schema := schemaForType(root)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = schemaID(document)
	properties := schema["properties"].(map[string]any)
	if _, exists := properties["canonical_policy"]; exists {
		properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	}
	if _, exists := properties["schema_version"]; exists {
		properties["schema_version"] = JSONSchema{"const": schemaVersion(document)}
	}
	return schema, nil
}

func schemaForType(value reflect.Type) JSONSchema {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if enum := enumValues(value); len(enum) > 0 {
		return JSONSchema{"type": "string", "enum": enum}
	}
	switch value.Kind() {
	case reflect.Struct:
		properties := make(map[string]any)
		var required []string
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
			if !contains(parts[1:], "omitempty") {
				required = append(required, name)
			}
		}
		return JSONSchema{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
	case reflect.Slice, reflect.Array:
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

func enumValues(value reflect.Type) []string {
	switch value {
	case reflect.TypeOf(State("")):
		return []string{string(StateSolved), string(StateUnsolved), string(StateIndeterminate), string(StateInvalidTask), string(StateEnvironmentFail), string(StateNotAdjudicated)}
	case reflect.TypeOf(EvidenceKind("")):
		return []string{string(EvidenceClaimedTest), string(EvidenceBenchmarkReward), string(EvidenceIndependentRun), string(EvidenceFormalRelation), string(EvidenceHumanLabel)}
	case reflect.TypeOf(AxisRating("")):
		return []string{string(RatingSufficient), string(RatingInsufficient), string(RatingUnclear), string(RatingNotApplicable)}
	case reflect.TypeOf(ReasonCode("")):
		return []string{
			string(ReasonClaimedOnly), string(ReasonEnvironmentFailure), string(ReasonEvidenceConflict), string(ReasonEvidenceConsistent),
			string(ReasonEvidenceInsufficient), string(ReasonFormalRelationSupports), string(ReasonHarmfulSideEffect),
			string(ReasonIndependentTestsFail), string(ReasonIndependentTestsPass), string(ReasonInvalidTask), string(ReasonTaskSatisfied),
			string(ReasonTaskUnsatisfied), string(ReasonTechnicalDefect), string(ReasonVerificationComplete), string(ReasonVerificationIncomplete),
		}
	case reflect.TypeOf(ReviewerRole("")):
		return []string{string(ReviewerRolePrimary), string(ReviewerRoleTieBreak)}
	case reflect.TypeOf(AssignmentPurpose("")):
		return []string{string(AssignmentPrimary), string(AssignmentTieBreak)}
	case reflect.TypeOf(ReviewVisibility("")):
		return []string{string(ReviewVisibilityPublic), string(ReviewVisibilityRestricted)}
	case reflect.TypeOf(ReviewDataRole("")):
		return []string{string(ReviewDataDevelopment), string(ReviewDataCalibration), string(ReviewDataTest)}
	case reflect.TypeOf(AdjudicationStatus("")):
		return []string{string(AdjudicationComplete), string(AdjudicationUnresolved)}
	case reflect.TypeOf(RecognitionBasis("")):
		return []string{string(RecognitionNone), string(RecognitionTaskText), string(RecognitionTrajectoryContent), string(RecognitionRepositoryFamiliarity), string(RecognitionPriorExposure), string(RecognitionOther)}
	case reflect.TypeOf(RubricAxis("")):
		return []string{string(RubricAxisTaskSatisfaction), string(RubricAxisTechnicalCorrectness), string(RubricAxisVerificationQuality), string(RubricAxisHarmfulSideEffects), string(RubricAxisEvidenceSufficiency)}
	case reflect.TypeOf(PilotSource("")):
		return []string{string(PilotSourceMutation), string(PilotSourceNatural)}
	case reflect.TypeOf(ReviewObjective("")):
		return []string{string(ReviewObjectiveOutcome)}
	case reflect.TypeOf(PilotTechnicalStatus("")):
		return []string{string(PilotTechnicalReady)}
	case reflect.TypeOf(PilotExternalActionStatus("")):
		return []string{string(PilotExternalActionNotAuthorized)}
	case reflect.TypeOf(OutcomePilotReviewabilityStatus("")):
		return []string{string(OutcomePilotStructurallyReady)}
	case reflect.TypeOf(OutcomePilotSemanticStatus("")):
		return []string{string(OutcomePilotRequiresHumanPilot)}
	default:
		return nil
	}
}

func schemaVersion(document string) string {
	return map[string]string{
		"plan": PlanSchemaVersion, "record": OutcomeSchemaVersion, "blind-packet": PacketSchemaVersion,
		"blind-build-request": BlindBuildSchemaVersion,
		"private-mapping":     MappingSchemaVersion, "label": LabelSchemaVersion, "resolution": ResolutionSchemaVersion,
		"agreement": AgreementSchemaVersion, "preservation": PreservationSchemaVersion,
		"sample-commitment":         SampleCommitmentSchemaVersion,
		"pilot-sample-v1":           PilotSampleSchemaVersion,
		"pilot-readiness-v1":        PilotReadinessSchemaVersion,
		"pilot-sample":              OutcomePilotSampleSchemaVersion,
		"pilot-readiness":           OutcomePilotReadinessSchemaVersion,
		"pilot-source-binding":      OutcomePilotSourceBindingSchemaVersion,
		"pilot-private-materials":   OutcomePilotPrivateMaterialsSchemaVersion,
		"pilot-inspection":          OutcomePilotInspectionSchemaVersion,
		"natural-inventory-request": NaturalRequestSchemaVersion, "natural-inventory": NaturalInventorySchemaVersion,
		"executable-log":    ExecutionSchemaVersion,
		"qualification-set": QualificationSchemaVersion, "qualification-report": QualificationReportSchemaVersion,
		"review-bundle":             ReviewBundleSchemaVersion,
		"reviewer-record":           ReviewerRecordSchemaVersion,
		"review-assignment":         ReviewAssignmentSchemaVersion,
		"label-batch":               LabelBatchSchemaVersion,
		"mapping-reveal":            MappingRevealSchemaVersion,
		"adjudication-ledger":       AdjudicationLedgerSchemaVersion,
		"reviewer-handbook":         ReviewerHandbookSchemaVersion,
		"reviewer-kit":              ReviewerKitSchemaVersion,
		"blinding-protocol":         BlindingProtocolSchemaVersion,
		"blinding-probe":            BlindingProbeSchemaVersion,
		"blinding-probe-batch":      BlindingProbeBatchSchemaVersion,
		"blinding-analysis":         BlindingAnalysisSchemaVersion,
		"rubric-ambiguity-analysis": RubricAmbiguitySchemaVersion,
		"source-audit":              OutcomeSourceAuditSchemaVersion,
	}[document]
}

func schemaID(document string) string {
	name := document
	if document == "pilot-sample-v1" {
		name = "pilot-sample"
	}
	if document == "pilot-readiness-v1" {
		name = "pilot-readiness"
	}
	version := schemaVersion(document)
	versionIndex := strings.LastIndex(version, ".v")
	if versionIndex < 0 {
		return ""
	}
	suffix := version[versionIndex:]
	return "https://evalwitness.dev/schemas/outcome-" + name + suffix + ".json"
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
