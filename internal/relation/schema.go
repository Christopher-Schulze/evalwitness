package relation

import (
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

type JSONSchema map[string]any

var relationSchemaTypes = map[string]reflect.Type{
	"blind-packet":                        reflect.TypeOf(BlindPacket{}),
	"blind-packet-v2":                     reflect.TypeOf(BlindPacket{}),
	"blind-packet-v3":                     reflect.TypeOf(BlindPacket{}),
	"case-material":                       reflect.TypeOf(CaseMaterial{}),
	"case-material-v2":                    reflect.TypeOf(CaseMaterial{}),
	"case-material-v3":                    reflect.TypeOf(CaseMaterial{}),
	"condition-probe":                     reflect.TypeOf(ConditionProbe{}),
	"condition-probe-v2":                  reflect.TypeOf(ConditionProbe{}),
	"condition-probe-v3":                  reflect.TypeOf(ConditionProbe{}),
	"condition-probe-batch":               reflect.TypeOf(ConditionProbeBatch{}),
	"condition-probe-batch-v2":            reflect.TypeOf(ConditionProbeBatch{}),
	"condition-probe-batch-v3":            reflect.TypeOf(ConditionProbeBatch{}),
	"formal-human-comparison":             reflect.TypeOf(FormalHumanComparison{}),
	"formal-human-comparison-v2":          reflect.TypeOf(FormalHumanComparison{}),
	"formal-human-comparison-v3":          reflect.TypeOf(FormalHumanComparison{}),
	"normalized-observations":             reflect.TypeOf([]AxisObservation{}),
	"mapping-reveal":                      reflect.TypeOf(MappingReveal{}),
	"mapping-reveal-v2":                   reflect.TypeOf(MappingReveal{}),
	"mapping-reveal-v3":                   reflect.TypeOf(MappingReveal{}),
	"owner-inspection-public-attestation": reflect.TypeOf(OwnerInspectionPublicAttestation{}),
	"pair-judgment":                       reflect.TypeOf(PairJudgment{}),
	"pair-judgment-v2":                    reflect.TypeOf(PairJudgment{}),
	"pair-judgment-v3":                    reflect.TypeOf(PairJudgment{}),
	"plan":                                reflect.TypeOf(Plan{}),
	"plan-v2":                             reflect.TypeOf(Plan{}),
	"plan-v3":                             reflect.TypeOf(RelationPlanV3{}),
	"pilot-sample":                        reflect.TypeOf(PilotSample{}),
	"pilot-sample-v2":                     reflect.TypeOf(PilotSample{}),
	"pilot-sample-v3":                     reflect.TypeOf(PilotSampleV3{}),
	"pilot-readiness":                     reflect.TypeOf(RelationPilotReadiness{}),
	"pilot-readiness-v2":                  reflect.TypeOf(RelationPilotReadiness{}),
	"pilot-readiness-v3":                  reflect.TypeOf(RelationPilotReadiness{}),
	"pilot-change-receipt":                reflect.TypeOf(PilotChangeReceipt{}),
	"pilot-change-receipt-v2":             reflect.TypeOf(PilotChangeReceipt{}),
	"pilot-change-receipt-v3":             reflect.TypeOf(PilotChangeReceipt{}),
	"pilot-inspection":                    reflect.TypeOf(PilotInspectionRecord{}),
	"pilot-inspection-v2":                 reflect.TypeOf(PilotInspectionRecord{}),
	"pilot-inspection-v3":                 reflect.TypeOf(PilotInspectionRecord{}),
	"pilot-inspection-session":            reflect.TypeOf(PilotInspectionSession{}),
	"pilot-inspection-event":              reflect.TypeOf(PilotInspectionEvent{}),
	"pilot-inspection-completion":         reflect.TypeOf(PilotInspectionCompletion{}),
	"pilot-launch-dossier":                reflect.TypeOf(PilotLaunchDossier{}),
	"pilot-launch-dossier-v2":             reflect.TypeOf(PilotLaunchDossier{}),
	"pilot-launch-dossier-v3":             reflect.TypeOf(PilotLaunchDossier{}),
	"primary-sample":                      reflect.TypeOf(PrimarySample{}),
	"primary-sample-v2":                   reflect.TypeOf(PrimarySample{}),
	"primary-sample-v3":                   reflect.TypeOf(PrimarySampleV3{}),
	"scarcity-public-evidence":            reflect.TypeOf(ScarcityPublicEvidence{}),
	"scarcity-sentinel-v3":                reflect.TypeOf(ScarcitySentinelV3{}),
	"private-mapping":                     reflect.TypeOf(PrivateMapping{}),
	"private-mapping-v2":                  reflect.TypeOf(PrivateMapping{}),
	"private-mapping-v3":                  reflect.TypeOf(PrivateMapping{}),
	"relation-resolution":                 reflect.TypeOf(RelationResolution{}),
	"relation-resolution-v2":              reflect.TypeOf(RelationResolution{}),
	"relation-resolution-v3":              reflect.TypeOf(RelationResolution{}),
	"qualification-answer-key":            reflect.TypeOf(QualificationAnswerKey{}),
	"qualification-answer-key-v2":         reflect.TypeOf(QualificationAnswerKey{}),
	"qualification-answer-key-v3":         reflect.TypeOf(QualificationAnswerKey{}),
	"qualification-report":                reflect.TypeOf(QualificationReport{}),
	"qualification-report-v2":             reflect.TypeOf(QualificationReport{}),
	"qualification-report-v3":             reflect.TypeOf(QualificationReport{}),
	"qualification-set":                   reflect.TypeOf(QualificationSet{}),
	"qualification-set-v2":                reflect.TypeOf(QualificationSet{}),
	"qualification-set-v3":                reflect.TypeOf(QualificationSet{}),
	"replay-receipt":                      reflect.TypeOf(ReplayReceipt{}),
	"replay-receipt-v2":                   reflect.TypeOf(ReplayReceipt{}),
	"replay-receipt-v3":                   reflect.TypeOf(ReplayReceipt{}),
	"review-assignment":                   reflect.TypeOf(ReviewAssignment{}),
	"review-assignment-v2":                reflect.TypeOf(ReviewAssignment{}),
	"review-assignment-v3":                reflect.TypeOf(ReviewAssignment{}),
	"review-bundle":                       reflect.TypeOf(ReviewBundle{}),
	"review-bundle-v2":                    reflect.TypeOf(ReviewBundle{}),
	"review-bundle-v3":                    reflect.TypeOf(ReviewBundle{}),
	"reviewer-handbook":                   reflect.TypeOf(ReviewerHandbook{}),
	"reviewer-handbook-v2":                reflect.TypeOf(ReviewerHandbook{}),
	"reviewer-handbook-v3":                reflect.TypeOf(ReviewerHandbook{}),
	"reviewer-kit":                        reflect.TypeOf(ReviewerKit{}),
	"reviewer-kit-v2":                     reflect.TypeOf(ReviewerKit{}),
	"reviewer-kit-v3":                     reflect.TypeOf(ReviewerKit{}),
	"reviewer-record":                     reflect.TypeOf(ReviewerRecord{}),
	"reviewer-record-v2":                  reflect.TypeOf(ReviewerRecord{}),
	"reviewer-record-v3":                  reflect.TypeOf(ReviewerRecord{}),
	"judgment-batch":                      reflect.TypeOf(JudgmentBatch{}),
	"judgment-batch-v2":                   reflect.TypeOf(JudgmentBatch{}),
	"judgment-batch-v3":                   reflect.TypeOf(JudgmentBatch{}),
	"prereveal-ambiguity":                 reflect.TypeOf(RelationAmbiguityAnalysis{}),
	"prereveal-ambiguity-v2":              reflect.TypeOf(RelationAmbiguityAnalysis{}),
	"prereveal-ambiguity-v3":              reflect.TypeOf(RelationAmbiguityAnalysis{}),
	"study-amendment":                     reflect.TypeOf(StudyAmendment{}),
	"study-amendment-v2":                  reflect.TypeOf(StudyAmendment{}),
	"study-amendment-v3":                  reflect.TypeOf(StudyAmendmentV3{}),
	"terminal-ledger":                     reflect.TypeOf(TerminalRelationLedger{}),
	"terminal-ledger-v2":                  reflect.TypeOf(TerminalRelationLedger{}),
	"terminal-ledger-v3":                  reflect.TypeOf(TerminalRelationLedger{}),
	"translation-result":                  reflect.TypeOf(TranslationResult{}),
	"translation-result-v2":               reflect.TypeOf(TranslationResult{}),
	"translation-result-v3":               reflect.TypeOf(TranslationResult{}),
}

var relationSchemaVersionsV3 = map[string]string{
	"blind-packet-v3":             BlindPacketSchemaVersionV3,
	"case-material-v3":            CaseMaterialSchemaVersionV3,
	"condition-probe-v3":          ConditionProbeSchemaVersionV3,
	"condition-probe-batch-v3":    ConditionProbeBatchSchemaVersionV3,
	"formal-human-comparison-v3":  FormalHumanComparisonSchemaVersionV3,
	"mapping-reveal-v3":           MappingRevealSchemaVersionV3,
	"pair-judgment-v3":            PairJudgmentSchemaVersionV3,
	"pilot-readiness-v3":          RelationPilotReadinessSchemaVersionV3,
	"pilot-change-receipt-v3":     PilotChangeReceiptSchemaVersionV3,
	"pilot-inspection-v3":         PilotInspectionSchemaVersionV3,
	"pilot-launch-dossier-v3":     PilotLaunchDossierSchemaVersionV3,
	"private-mapping-v3":          PrivateMappingSchemaVersionV3,
	"relation-resolution-v3":      RelationResolutionSchemaVersionV3,
	"qualification-answer-key-v3": QualificationKeySchemaVersionV3,
	"qualification-report-v3":     QualificationReportSchemaVersionV3,
	"qualification-set-v3":        QualificationSetSchemaVersionV3,
	"replay-receipt-v3":           ReplayReceiptSchemaVersionV3,
	"review-assignment-v3":        ReviewAssignmentSchemaVersionV3,
	"review-bundle-v3":            ReviewBundleSchemaVersionV3,
	"reviewer-handbook-v3":        ReviewerHandbookSchemaVersionV3,
	"reviewer-kit-v3":             ReviewerKitSchemaVersionV3,
	"reviewer-record-v3":          ReviewerRecordSchemaVersionV3,
	"judgment-batch-v3":           JudgmentBatchSchemaVersionV3,
	"prereveal-ambiguity-v3":      RelationAmbiguitySchemaVersionV3,
	"terminal-ledger-v3":          TerminalRelationLedgerSchemaVersionV3,
	"translation-result-v3":       TranslationResultSchemaVersionV3,
}

func Schema(document string) (JSONSchema, error) {
	root, exists := relationSchemaTypes[document]
	if !exists {
		return nil, errors.New("unknown relation schema type")
	}
	schema := schemaForType(root)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schemaDocument, schemaVersion := document, "v1"
	if strings.HasSuffix(document, "-v2") {
		schemaDocument, schemaVersion = strings.TrimSuffix(document, "-v2"), "v2"
	} else if strings.HasSuffix(document, "-v3") {
		schemaDocument, schemaVersion = strings.TrimSuffix(document, "-v3"), "v3"
	}
	schema["$id"] = "https://evalwitness.dev/schemas/relation-" + schemaDocument + "." + schemaVersion + ".json"
	if properties, ok := schema["properties"].(map[string]any); ok {
		if _, exists := properties["canonical_policy"]; exists {
			properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
		}
		switch document {
		case "blind-packet":
			properties["schema_version"] = JSONSchema{"const": BlindPacketSchemaVersionV1}
		case "blind-packet-v2":
			properties["schema_version"] = JSONSchema{"const": BlindPacketSchemaVersionV2}
		case "case-material":
			properties["schema_version"] = JSONSchema{"const": CaseMaterialSchemaVersionV1}
		case "case-material-v2":
			properties["schema_version"] = JSONSchema{"const": CaseMaterialSchemaVersionV2}
		case "condition-probe":
			properties["schema_version"] = JSONSchema{"const": ConditionProbeSchemaVersionV1}
		case "condition-probe-v2":
			properties["schema_version"] = JSONSchema{"const": ConditionProbeSchemaVersionV2}
		case "condition-probe-batch":
			properties["schema_version"] = JSONSchema{"const": ConditionProbeBatchSchemaVersionV1}
		case "condition-probe-batch-v2":
			properties["schema_version"] = JSONSchema{"const": ConditionProbeBatchSchemaVersionV2}
		case "formal-human-comparison":
			properties["schema_version"] = JSONSchema{"const": FormalHumanComparisonSchemaVersionV1}
		case "formal-human-comparison-v2":
			properties["schema_version"] = JSONSchema{"const": FormalHumanComparisonSchemaVersionV2}
		case "mapping-reveal":
			properties["schema_version"] = JSONSchema{"const": MappingRevealSchemaVersionV1}
		case "mapping-reveal-v2":
			properties["schema_version"] = JSONSchema{"const": MappingRevealSchemaVersionV2}
		case "plan":
			properties["schema_version"] = JSONSchema{"const": PlanSchemaVersionV1}
		case "plan-v2":
			properties["schema_version"] = JSONSchema{"const": PlanSchemaVersionV2}
		case "plan-v3":
			properties["schema_version"] = JSONSchema{"const": PlanSchemaVersionV3}
		case "pilot-sample":
			properties["schema_version"] = JSONSchema{"const": PilotSampleSchemaVersionV1}
		case "pilot-sample-v2":
			properties["schema_version"] = JSONSchema{"const": PilotSampleSchemaVersionV2}
		case "pilot-sample-v3":
			properties["schema_version"] = JSONSchema{"const": PilotSampleSchemaVersionV3}
		case "pilot-readiness":
			properties["schema_version"] = JSONSchema{"const": RelationPilotReadinessSchemaVersionV1}
		case "pilot-readiness-v2":
			properties["schema_version"] = JSONSchema{"const": RelationPilotReadinessSchemaVersionV2}
		case "pilot-change-receipt":
			properties["schema_version"] = JSONSchema{"const": PilotChangeReceiptSchemaVersionV1}
		case "pilot-change-receipt-v2":
			properties["schema_version"] = JSONSchema{"const": PilotChangeReceiptSchemaVersionV2}
		case "pilot-inspection":
			properties["schema_version"] = JSONSchema{"const": PilotInspectionSchemaVersionV1}
		case "pilot-inspection-v2":
			properties["schema_version"] = JSONSchema{"const": PilotInspectionSchemaVersionV2}
		case "pilot-inspection-session":
			properties["schema_version"] = JSONSchema{"const": PilotInspectionSessionSchemaVersion}
			properties["journal_protocol"] = JSONSchema{"const": PilotInspectionJournalProtocol}
			properties["relation_protocol"] = JSONSchema{"const": ProtocolVersionV3}
			properties["human_study_status"] = JSONSchema{"const": PilotInspectionJournalHumanStudyStatus}
			properties["packets"] = schemaArrayWithBounds(properties["packets"], 7)
			properties["scarcity_cases"] = schemaArrayWithBounds(properties["scarcity_cases"], 3)
			properties["core_dimensions"] = pilotInspectionDimensionArraySchema(PilotInspectionCoreDimensions())
			properties["scarcity_case_dimensions"] = pilotInspectionDimensionArraySchema(PilotInspectionScarcityCaseDimensions())
			properties["scarcity_boundary_dimensions"] = pilotInspectionDimensionArraySchema(PilotInspectionScarcityBoundaryDimensions())
			if packageBinding, ok := properties["package"].(JSONSchema); ok {
				if packageProperties, ok := packageBinding["properties"].(map[string]any); ok {
					packageProperties["package_format"] = JSONSchema{"const": PilotPackageFormatV5}
				}
			}
		case "pilot-inspection-event":
			properties["schema_version"] = JSONSchema{"const": PilotInspectionEventSchemaVersion}
			properties["journal_protocol"] = JSONSchema{"const": PilotInspectionJournalProtocol}
			properties["owner_confirmation"] = JSONSchema{"const": PilotInspectionOwnerConfirmation}
			properties["subject_kind"] = JSONSchema{"type": "string", "enum": []string{
				string(PilotInspectionSubjectCorePacket), string(PilotInspectionSubjectScarcityCase), string(PilotInspectionSubjectScarcityBoundary),
			}}
			properties["dimension"] = JSONSchema{"type": "string", "enum": pilotInspectionDimensionStrings(PilotInspectionDimensions())}
		case "pilot-inspection-completion":
			properties["schema_version"] = JSONSchema{"const": PilotInspectionCompletionSchemaVersion}
			properties["journal_protocol"] = JSONSchema{"const": PilotInspectionJournalProtocol}
			properties["owner_confirmation"] = JSONSchema{"const": PilotInspectionCompletionConfirmation}
			properties["human_study_status"] = JSONSchema{"const": PilotInspectionJournalHumanStudyStatus}
			properties["required_assessments"] = JSONSchema{"const": PilotInspectionRequiredAssessments}
			properties["decision_summaries"] = schemaArrayWithBounds(properties["decision_summaries"], 7)
			properties["scarcity_summaries"] = schemaArrayWithBounds(properties["scarcity_summaries"], 3)
		case "pilot-launch-dossier":
			properties["schema_version"] = JSONSchema{"const": PilotLaunchDossierSchemaVersionV1}
		case "pilot-launch-dossier-v2":
			properties["schema_version"] = JSONSchema{"const": PilotLaunchDossierSchemaVersionV2}
		case "primary-sample":
			properties["schema_version"] = JSONSchema{"const": PrimarySampleSchemaVersionV1}
		case "primary-sample-v2":
			properties["schema_version"] = JSONSchema{"const": PrimarySampleSchemaVersionV2}
		case "primary-sample-v3":
			properties["schema_version"] = JSONSchema{"const": PrimarySampleSchemaVersionV3}
		case "scarcity-sentinel-v3":
			properties["schema_version"] = JSONSchema{"const": ScarcitySentinelSchemaVersionV3}
		case "scarcity-public-evidence":
			properties["schema_version"] = JSONSchema{"const": ScarcityPublicEvidenceSchemaVersion}
		case "owner-inspection-public-attestation":
			properties["schema_version"] = JSONSchema{"const": OwnerInspectionPublicAttestationSchemaVersion}
			properties["evidence_kind"] = JSONSchema{"const": OwnerInspectionPublicAttestationEvidenceKind}
			properties["inspection_mode"] = JSONSchema{"const": OwnerInspectionPublicAttestationMode}
			properties["human_study_status"] = JSONSchema{"const": PilotInspectionJournalHumanStudyStatus}
			properties["external_action_status"] = JSONSchema{"const": PilotInspectionJournalExternalAction}
			properties["dimensions"] = schemaArrayWithBounds(properties["dimensions"], len(PilotInspectionDimensions()))
			properties["claims"] = schemaArrayWithBounds(properties["claims"], 10)
			if disclosure, ok := properties["disclosure"].(JSONSchema); ok {
				if disclosureProperties, ok := disclosure["properties"].(map[string]any); ok {
					disclosureProperties["private_chain_verified"] = JSONSchema{"const": true}
					disclosureProperties["private_journal_identities_disclosed"] = JSONSchema{"const": false}
					disclosureProperties["restricted_evidence_disclosed"] = JSONSchema{"const": false}
					disclosureProperties["public_validation_scope"] = JSONSchema{"const": OwnerInspectionPublicValidationScope}
					disclosureProperties["source_reproduction"] = JSONSchema{"const": OwnerInspectionPublicSourceReproduction}
					disclosureProperties["signature_status"] = JSONSchema{"const": OwnerInspectionPublicSignatureStatus}
					disclosureProperties["capsule_status"] = JSONSchema{"const": OwnerInspectionPublicCapsuleStatus}
				}
			}
		case "private-mapping":
			properties["schema_version"] = JSONSchema{"const": PrivateMappingSchemaVersionV1}
		case "private-mapping-v2":
			properties["schema_version"] = JSONSchema{"const": PrivateMappingSchemaVersionV2}
		case "relation-resolution":
			properties["schema_version"] = JSONSchema{"const": RelationResolutionSchemaVersionV1}
		case "relation-resolution-v2":
			properties["schema_version"] = JSONSchema{"const": RelationResolutionSchemaVersionV2}
		case "pair-judgment":
			properties["schema_version"] = JSONSchema{"const": PairJudgmentSchemaVersionV1}
		case "pair-judgment-v2":
			properties["schema_version"] = JSONSchema{"const": PairJudgmentSchemaVersionV2}
		case "qualification-answer-key":
			properties["schema_version"] = JSONSchema{"const": QualificationKeySchemaVersionV1}
		case "qualification-answer-key-v2":
			properties["schema_version"] = JSONSchema{"const": QualificationKeySchemaVersionV2}
		case "qualification-report":
			properties["schema_version"] = JSONSchema{"const": QualificationReportSchemaVersionV1}
		case "qualification-report-v2":
			properties["schema_version"] = JSONSchema{"const": QualificationReportSchemaVersionV2}
		case "qualification-set":
			properties["schema_version"] = JSONSchema{"const": QualificationSetSchemaVersionV1}
		case "qualification-set-v2":
			properties["schema_version"] = JSONSchema{"const": QualificationSetSchemaVersionV2}
		case "replay-receipt":
			properties["schema_version"] = JSONSchema{"const": ReplayReceiptSchemaVersionV1}
		case "replay-receipt-v2":
			properties["schema_version"] = JSONSchema{"const": ReplayReceiptSchemaVersionV2}
		case "review-assignment":
			properties["schema_version"] = JSONSchema{"const": ReviewAssignmentSchemaVersionV1}
		case "review-assignment-v2":
			properties["schema_version"] = JSONSchema{"const": ReviewAssignmentSchemaVersionV2}
		case "review-bundle":
			properties["schema_version"] = JSONSchema{"const": ReviewBundleSchemaVersionV1}
		case "review-bundle-v2":
			properties["schema_version"] = JSONSchema{"const": ReviewBundleSchemaVersionV2}
		case "reviewer-handbook":
			properties["schema_version"] = JSONSchema{"const": ReviewerHandbookSchemaVersionV1}
		case "reviewer-handbook-v2":
			properties["schema_version"] = JSONSchema{"const": ReviewerHandbookSchemaVersionV2}
		case "reviewer-kit":
			properties["schema_version"] = JSONSchema{"const": ReviewerKitSchemaVersionV1}
		case "reviewer-kit-v2":
			properties["schema_version"] = JSONSchema{"const": ReviewerKitSchemaVersionV2}
		case "reviewer-record":
			properties["schema_version"] = JSONSchema{"const": ReviewerRecordSchemaVersionV1}
		case "reviewer-record-v2":
			properties["schema_version"] = JSONSchema{"const": ReviewerRecordSchemaVersionV2}
		case "judgment-batch":
			properties["schema_version"] = JSONSchema{"const": JudgmentBatchSchemaVersionV1}
		case "judgment-batch-v2":
			properties["schema_version"] = JSONSchema{"const": JudgmentBatchSchemaVersionV2}
		case "prereveal-ambiguity":
			properties["schema_version"] = JSONSchema{"const": RelationAmbiguitySchemaVersionV1}
		case "prereveal-ambiguity-v2":
			properties["schema_version"] = JSONSchema{"const": RelationAmbiguitySchemaVersionV2}
		case "study-amendment":
			properties["schema_version"] = JSONSchema{"const": StudyAmendmentSchemaVersionV1}
		case "study-amendment-v2":
			properties["schema_version"] = JSONSchema{"const": StudyAmendmentSchemaVersionV2}
		case "study-amendment-v3":
			properties["schema_version"] = JSONSchema{"const": StudyAmendmentSchemaVersionV3}
		case "terminal-ledger":
			properties["schema_version"] = JSONSchema{"const": TerminalRelationLedgerSchemaVersionV1}
		case "terminal-ledger-v2":
			properties["schema_version"] = JSONSchema{"const": TerminalRelationLedgerSchemaVersionV2}
		case "translation-result":
			properties["schema_version"] = JSONSchema{"const": TranslationResultSchemaVersionV1}
		case "translation-result-v2":
			properties["schema_version"] = JSONSchema{"const": TranslationResultSchemaVersionV2}
		}
		if v3Identity, exists := relationSchemaVersionsV3[document]; exists {
			properties["schema_version"] = JSONSchema{"const": v3Identity}
		}
	}
	if err := configureGovernanceSchema(document, schema); err != nil {
		return nil, err
	}
	return schema, nil
}

func pilotInspectionDimensionArraySchema(dimensions []PilotInspectionDimension) JSONSchema {
	return JSONSchema{
		"type": "array", "minItems": len(dimensions), "maxItems": len(dimensions),
		"items": JSONSchema{"type": "string", "enum": pilotInspectionDimensionStrings(dimensions)},
	}
}

func pilotInspectionDimensionStrings(dimensions []PilotInspectionDimension) []string {
	values := make([]string, len(dimensions))
	for index, dimension := range dimensions {
		values[index] = string(dimension)
	}
	return values
}

func schemaArrayWithBounds(value any, exact int) JSONSchema {
	schema, ok := value.(JSONSchema)
	if !ok {
		schema = JSONSchema{"type": "array"}
	}
	schema["minItems"] = exact
	schema["maxItems"] = exact
	return schema
}

func configureGovernanceSchema(document string, schema JSONSchema) error {
	sourceBindings := []string{"source_corpus_spec_digest", "source_mutation_program_digest", "source_construct_audit_digest"}
	v2ChainBindings := append(append([]string(nil), sourceBindings...), "source_corpus_digest", "construct_firewall_commitment_digest")
	materialBindings := append(append([]string(nil), sourceBindings...), "relation_contract_version", "evidence_boundary_version", "construct_firewall_digest")
	v3SourceBindings := []string{"source_corpus_plan_digest", "source_mutation_program_digest", "source_construct_audit_digest"}
	v3ChainBindings := append(append([]string(nil), v3SourceBindings...), "source_corpus_digest", "construct_firewall_commitment_digest")
	v3MaterialBindings := append(append([]string(nil), v3SourceBindings...), "relation_contract_version", "evidence_boundary_version", "construct_firewall_digest")
	if properties, ok := schema["properties"].(map[string]any); ok && strings.HasSuffix(document, "-v2") {
		if _, exists := properties["protocol_version"]; exists {
			properties["protocol_version"] = JSONSchema{"const": ProtocolVersionV2}
		}
	}
	if properties, ok := schema["properties"].(map[string]any); ok && strings.HasSuffix(document, "-v3") {
		if _, exists := properties["protocol_version"]; exists {
			properties["protocol_version"] = JSONSchema{"const": GovernanceProtocolVersionV3}
		}
	}
	switch document {
	case "plan":
		removeSchemaProperties(schema, sourceBindings...)
	case "plan-v2":
		return requireSchemaProperties(schema, sourceBindings...)
	case "primary-sample":
		removeSchemaProperties(schema, append(sourceBindings, "unique_lineage_clusters", "source_format_counts")...)
		return configureBindingSchema(schema, false)
	case "primary-sample-v2":
		if err := requireSchemaProperties(schema, append(sourceBindings, "unique_lineage_clusters", "source_format_counts")...); err != nil {
			return err
		}
		return configureBindingSchema(schema, true)
	case "pilot-sample":
		removeSchemaProperties(schema, sourceBindings...)
		return configureBindingSchema(schema, false)
	case "pilot-sample-v2":
		if err := requireSchemaProperties(schema, sourceBindings...); err != nil {
			return err
		}
		return configureBindingSchema(schema, true)
	case "study-amendment":
		removeSchemaProperties(schema, sourceBindings...)
	case "study-amendment-v2":
		return requireSchemaProperties(schema, sourceBindings...)
	case "case-material", "private-mapping":
		removeSchemaProperties(schema, materialBindings...)
	case "case-material-v2", "private-mapping-v2":
		return requireSchemaProperties(schema, materialBindings...)
	case "case-material-v3", "private-mapping-v3":
		removeSchemaProperties(schema, "source_corpus_spec_digest")
		return requireSchemaProperties(schema, v3MaterialBindings...)
	case "pilot-readiness", "pilot-change-receipt", "pilot-launch-dossier":
		removeSchemaProperties(schema, v2ChainBindings...)
	case "pilot-readiness-v2", "pilot-change-receipt-v2", "pilot-launch-dossier-v2":
		return requireSchemaProperties(schema, v2ChainBindings...)
	case "pilot-readiness-v3", "pilot-change-receipt-v3", "pilot-launch-dossier-v3":
		removeSchemaProperties(schema, "source_corpus_spec_digest")
		return requireSchemaProperties(schema, v3ChainBindings...)
	}
	return nil
}

func configureBindingSchema(schema JSONSchema, required bool) error {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return errors.New("relation schema properties are invalid")
	}
	bindings, ok := properties["bindings"].(JSONSchema)
	if !ok {
		return errors.New("relation binding schema is invalid")
	}
	if required {
		return requireSchemaProperties(bindings, "construct_firewalls")
	}
	removeSchemaProperties(bindings, "construct_firewalls")
	return nil
}

func requireSchemaProperties(schema JSONSchema, names ...string) error {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return errors.New("relation schema properties are invalid")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		return errors.New("relation schema required list is invalid")
	}
	for _, name := range names {
		if _, exists := properties[name]; !exists {
			return errors.New("relation schema required property is missing")
		}
		if !slices.Contains(required, name) {
			required = append(required, name)
		}
	}
	schema["required"] = required
	return nil
}

func removeSchemaProperties(schema JSONSchema, names ...string) {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	for _, name := range names {
		delete(properties, name)
	}
}

func schemaForType(value reflect.Type) JSONSchema {
	if enum := enumValues(value); len(enum) > 0 {
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
			properties[name] = schemaForType(field.Type)
			if len(parts) == 1 || parts[1] != "omitempty" {
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
	default:
		return JSONSchema{}
	}
}

func enumValues(value reflect.Type) []string {
	switch value {
	case reflect.TypeOf(ReviewObjective("")):
		return []string{string(ReviewObjectiveControlledRelation)}
	case reflect.TypeOf(VisiblePosition("")):
		return []string{string(PositionLeft), string(PositionRight)}
	case reflect.TypeOf(LogicalSide("")):
		return []string{string(LogicalOriginal), string(LogicalTransformed)}
	case reflect.TypeOf(ReviewDataRole("")):
		return []string{string(ReviewDataDevelopmentPilot), string(ReviewDataPrimaryAudit)}
	case reflect.TypeOf(ReviewVisibility("")):
		return []string{string(ReviewVisibilityRestricted)}
	case reflect.TypeOf(ReviewerRole("")):
		return []string{string(ReviewerRolePrimary), string(ReviewerRoleTieBreak)}
	case reflect.TypeOf(AssignmentPurpose("")):
		return []string{string(AssignmentPurposePrimary), string(AssignmentPurposeTieBreak)}
	case reflect.TypeOf(PilotInspectionAssessment("")):
		return []string{string(PilotInspectionFailed), string(PilotInspectionIndeterminate), string(PilotInspectionNotApplicable), string(PilotInspectionPassed)}
	case reflect.TypeOf(PilotInspectionDisposition("")):
		return []string{string(PilotInspectionAccepted), string(PilotInspectionRevisionRequired), string(PilotInspectionUnresolved)}
	case reflect.TypeOf(PilotInspectionOverallStatus("")):
		return []string{string(PilotInspectionOverallPassed), string(PilotInspectionOverallRevisionRequired), string(PilotInspectionOverallUnresolved)}
	case reflect.TypeOf(PilotInspectionReason("")):
		return []string{string(PilotInspectionReasonAlignment), string(PilotInspectionReasonBlinding), string(PilotInspectionReasonCandidateOrder), string(PilotInspectionReasonInformation), string(PilotInspectionReasonRedistribution), string(PilotInspectionReasonRubric), string(PilotInspectionReasonTaskContext), string(PilotInspectionReasonTransformation)}
	case reflect.TypeOf(PilotInspectionDimension("")):
		values := PilotInspectionDimensions()
		result := make([]string, len(values))
		for index, value := range values {
			result[index] = string(value)
		}
		return result
	case reflect.TypeOf(DirectionGuess("")):
		return []string{string(DirectionLeftOriginal), string(DirectionRightOriginal), string(DirectionUnknown)}
	case reflect.TypeOf(RecognitionBasis("")):
		return []string{string(RecognitionNone), string(RecognitionPriorExposure), string(RecognitionRepositoryContext), string(RecognitionTaskText), string(RecognitionTraceContent)}
	case reflect.TypeOf(UnitType("")):
		return []string{string(UnitCandidatePairOrders), string(UnitTrajectoryPair)}
	case reflect.TypeOf(Axis("")):
		return []string{string(AxisCausalIntegrity), string(AxisEvidenceStrength), string(AxisExecutableSupport), string(AxisInformation), string(AxisPresentation), string(AxisSemanticQuality), string(AxisUntrustedControl)}
	case reflect.TypeOf(Rating("")):
		return []string{string(RatingControlEffect), string(RatingEqual), string(RatingIndeterminate), string(RatingInsufficient), string(RatingLeft), string(RatingNoControl), string(RatingNotApplicable), string(RatingRight), string(RatingSufficient)}
	case reflect.TypeOf(NormalizedRating("")):
		return []string{string(NormalizedControlEffect), string(NormalizedEqual), string(NormalizedIndeterminate), string(NormalizedInsufficient), string(NormalizedNoControl), string(NormalizedNotApplicable), string(NormalizedOriginal), string(NormalizedSufficient), string(NormalizedTransformed)}
	case reflect.TypeOf(ExternalActionStatus("")):
		return []string{string(ExternalActionNotAuthorized)}
	case reflect.TypeOf(ScarcityPublicClaimStatus("")):
		return []string{
			string(ScarcityPublicClaimSupported), string(ScarcityPublicClaimUnsupported), string(ScarcityPublicClaimNotRun),
			string(ScarcityPublicClaimNotMeasured), string(ScarcityPublicClaimNotAuthorized),
		}
	case reflect.TypeOf(OwnerInspectionPublicClaimStatus("")):
		return []string{
			string(OwnerInspectionPublicClaimSupported), string(OwnerInspectionPublicClaimOwnerAttested),
			string(OwnerInspectionPublicClaimNotRun), string(OwnerInspectionPublicClaimNotAuthorized),
			string(OwnerInspectionPublicClaimUnsupported), string(OwnerInspectionPublicClaimNotPubliclyReproducible),
		}
	case reflect.TypeOf(TranslationState("")):
		return []string{string(TranslationContradicts), string(TranslationSupports), string(TranslationUnresolved)}
	case reflect.TypeOf(ReasonCode("")):
		return []string{string(ReasonAmbiguousTask), string(ReasonCausalIntegrityDiffers), string(ReasonEvidenceOnlyChange), string(ReasonEvidenceStrengthDiffers), string(ReasonExecutableSupportDiffers), string(ReasonHiddenContextRequired), string(ReasonInsufficientInformation), string(ReasonMultiFactorChange), string(ReasonNoJudgmentChange), string(ReasonPresentationDiffers), string(ReasonTaskQualityDiffers), string(ReasonUntrustedContentControls)}
	case reflect.TypeOf(mutation.Family("")):
		return []string{string(mutation.FamilyCandidateOrderReversal), string(mutation.FamilyCausalIndependentReorder), string(mutation.FamilyTestEvidenceFalsified), string(mutation.FamilyToolOutputIncomplete), string(mutation.FamilyIrrelevantVerbosity), string(mutation.FamilyNeutralFormatting), string(mutation.FamilyTestEvidenceOmitted), string(mutation.FamilyUntrustedScoreInjection)}
	case reflect.TypeOf(mutation.InterventionClass("")):
		return []string{string(mutation.ClassAdversarialClaim), string(mutation.ClassEvidenceAvailability), string(mutation.ClassPresentation), string(mutation.ClassSemanticQuality)}
	case reflect.TypeOf(mutation.Relation("")):
		return []string{string(mutation.RelationNoControlEffect), string(mutation.RelationQualityEqual), string(mutation.RelationQualityEqualEvidenceLow), string(mutation.RelationVerifiedOutcomeWins)}
	default:
		return nil
	}
}
