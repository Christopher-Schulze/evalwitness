package stress

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type JSONSchema map[string]any

func Schema(document string) (JSONSchema, error) {
	var root reflect.Type
	var identifier, schemaVersion string
	switch document {
	case "relation":
		root, identifier, schemaVersion = reflect.TypeOf(Relation{}), "https://evalwitness.dev/schemas/stress-relation.v1.json", RelationSchemaVersion
	case "result":
		root, identifier, schemaVersion = reflect.TypeOf(Result{}), "https://evalwitness.dev/schemas/stress-result.v1.json", ResultSchemaVersion
	case "stage-trace":
		root, identifier, schemaVersion = reflect.TypeOf(StageTrace{}), "https://evalwitness.dev/schemas/stress-stage-trace.v1.json", StageTraceSchemaVersion
	case "stage-comparison":
		root, identifier, schemaVersion = reflect.TypeOf(StageComparison{}), "https://evalwitness.dev/schemas/stress-stage-comparison.v1.json", StageComparisonSchemaVersion
	case "counterexample":
		root, identifier, schemaVersion = reflect.TypeOf(Counterexample{}), "https://evalwitness.dev/schemas/stress-counterexample.v1.json", CounterexampleSchemaVersion
	case "construct-admission":
		root, identifier, schemaVersion = reflect.TypeOf(ConstructAdmission{}), "https://evalwitness.dev/schemas/stress-construct-admission.v1.json", AdmissionSchemaVersion
	case "reduction-observation":
		root, identifier, schemaVersion = reflect.TypeOf(ReductionObservation{}), "https://evalwitness.dev/schemas/stress-reduction-observation.v1.json", ReductionObservationSchemaVersion
	case "relation-registry":
		root, identifier, schemaVersion = reflect.TypeOf(RelationRegistry{}), "https://evalwitness.dev/schemas/stress-relation-registry.v1.json", RelationRegistrySchemaVersion
	case "replay-execution":
		root, identifier, schemaVersion = reflect.TypeOf(ReplayExecution{}), "https://evalwitness.dev/schemas/stress-replay-execution.v1.json", ReplayExecutionSchemaVersion
	case "arm-comparison-plan":
		root, identifier, schemaVersion = reflect.TypeOf(ArmComparisonPlan{}), "https://evalwitness.dev/schemas/stress-arm-comparison-plan.v1.json", ArmComparisonPlanSchemaVersion
	case "arm-comparison-report":
		root, identifier, schemaVersion = reflect.TypeOf(ArmComparisonReport{}), "https://evalwitness.dev/schemas/stress-arm-comparison-report.v1.json", ArmComparisonReportSchemaVersion
	case "analysis-design":
		root, identifier, schemaVersion = reflect.TypeOf(StressAnalysisDesign{}), "https://evalwitness.dev/schemas/stress-analysis-design.v1.json", StressAnalysisDesignSchemaVersion
	case "analysis-report":
		root, identifier, schemaVersion = reflect.TypeOf(StressAnalysisReport{}), "https://evalwitness.dev/schemas/stress-analysis-report.v1.json", StressAnalysisReportSchemaVersion
	case "zero-cost-execution":
		root, identifier, schemaVersion = reflect.TypeOf(ZeroCostExecution{}), "https://evalwitness.dev/schemas/stress-zero-cost-execution.v1.json", ZeroCostExecutionSchemaVersion
	case "protocol-adapter-proof":
		root, identifier, schemaVersion = reflect.TypeOf(ProtocolAdapterProof{}), "https://evalwitness.dev/schemas/stress-protocol-adapter-proof.v1.json", ProtocolAdapterProofSchemaVersion
	case "arm-replay-evidence":
		root, identifier, schemaVersion = reflect.TypeOf(ArmReplayEvidence{}), "https://evalwitness.dev/schemas/stress-arm-replay-evidence.v1.json", ArmReplayEvidenceSchemaVersion
	case "held-out-partition-lock":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutPartitionLock{}), "https://evalwitness.dev/schemas/stress-held-out-partition-lock.v1.json", HeldOutPartitionLockSchemaVersion
	case "held-out-campaign-plan":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutCampaignPlan{}), "https://evalwitness.dev/schemas/stress-held-out-campaign-plan.v1.json", HeldOutCampaignPlanSchemaVersion
	case "held-out-campaign-batch-binding":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutCampaignBatchBinding{}), "https://evalwitness.dev/schemas/stress-held-out-campaign-batch-binding.v1.json", HeldOutCampaignBatchBindingSchemaVersion
	case "held-out-admission-plan":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutAdmissionPlan{}), "https://evalwitness.dev/schemas/stress-held-out-admission-plan.v1.json", HeldOutAdmissionPlanSchemaVersion
	case "held-out-execution-batch-binding":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutExecutionBatchBinding{}), "https://evalwitness.dev/schemas/stress-held-out-execution-batch-binding.v1.json", HeldOutExecutionBatchBindingSchemaVersion
	case "held-out-preflight-evidence":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutPreflightEvidence{}), "https://evalwitness.dev/schemas/stress-held-out-preflight-evidence.v1.json", HeldOutPreflightEvidenceSchemaVersion
	case "held-out-preflight-custody":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutPreflightCustody{}), "https://evalwitness.dev/schemas/stress-held-out-preflight-custody.v1.json", HeldOutPreflightCustodySchemaVersion
	case "held-out-execution-permit":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutExecutionPermit{}), "https://evalwitness.dev/schemas/stress-held-out-execution-permit.v2.json", HeldOutExecutionPermitSchemaVersion
	case "held-out-execution-reservation":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutExecutionReservation{}), "https://evalwitness.dev/schemas/stress-held-out-execution-reservation.v1.json", HeldOutExecutionReservationSchemaVersion
	case "held-out-live-batch-evidence":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutLiveBatchEvidence{}), "https://evalwitness.dev/schemas/stress-held-out-live-batch-evidence.v1.json", HeldOutLiveBatchEvidenceSchemaVersion
	case "held-out-live-replay-verification":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutLiveReplayVerification{}), "https://evalwitness.dev/schemas/stress-held-out-live-replay-verification.v1.json", HeldOutLiveReplayVerificationSchemaVersion
	case "held-out-execution-ledger":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutExecutionLedger{}), "https://evalwitness.dev/schemas/stress-held-out-execution-ledger.v1.json", HeldOutExecutionLedgerSchemaVersion
	case "held-out-run-seal":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutRunSeal{}), "https://evalwitness.dev/schemas/stress-held-out-run-seal.v1.json", HeldOutRunSealSchemaVersion
	case "held-out-run-seal-v2":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutRunSealV2{}), "https://evalwitness.dev/schemas/stress-held-out-run-seal.v2.json", HeldOutRunSealV2SchemaVersion
	case "held-out-run-readiness-refusal":
		root, identifier, schemaVersion = reflect.TypeOf(HeldOutRunReadinessRefusal{}), "https://evalwitness.dev/schemas/stress-held-out-run-readiness-refusal.v1.json", HeldOutRunReadinessRefusalSchemaVersion
	case "next-version-discovery-ledger":
		root, identifier, schemaVersion = reflect.TypeOf(NextVersionDiscoveryLedger{}), "https://evalwitness.dev/schemas/stress-next-version-discovery-ledger.v1.json", NextVersionDiscoverySchemaVersion
	case "development-case-study":
		root, identifier, schemaVersion = reflect.TypeOf(DevelopmentCaseStudy{}), "https://evalwitness.dev/schemas/stress-development-case-study.v1.json", DevelopmentCaseStudySchemaVersion
	case "development-challenge":
		root, identifier, schemaVersion = reflect.TypeOf(DevelopmentChallenge{}), "https://evalwitness.dev/schemas/stress-development-challenge.v1.json", DevelopmentChallengeSchemaVersion
	case "development-challenge-receipt":
		root, identifier, schemaVersion = reflect.TypeOf(DevelopmentChallengeReceipt{}), "https://evalwitness.dev/schemas/stress-development-challenge-receipt.v1.json", DevelopmentChallengeReceiptSchemaVersion
	default:
		return nil, errors.New("stress schema type must be relation, result, stage-trace, stage-comparison, counterexample, construct-admission, reduction-observation, relation-registry, replay-execution, arm-comparison-plan, arm-comparison-report, analysis-design, analysis-report, zero-cost-execution, protocol-adapter-proof, arm-replay-evidence, held-out-partition-lock, held-out-campaign-plan, held-out-campaign-batch-binding, held-out-admission-plan, held-out-execution-batch-binding, held-out-preflight-evidence, held-out-preflight-custody, held-out-execution-permit, held-out-execution-reservation, held-out-live-batch-evidence, held-out-live-replay-verification, held-out-execution-ledger, held-out-run-seal, held-out-run-seal-v2, held-out-run-readiness-refusal, next-version-discovery-ledger, development-case-study, development-challenge, or development-challenge-receipt")
	}
	schema := stressSchemaForType(root)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = identifier
	properties := schema["properties"].(map[string]any)
	properties["schema_version"] = JSONSchema{"const": schemaVersion}
	properties["canonical_policy"] = JSONSchema{"const": CanonicalPolicy}
	return schema, nil
}

func stressSchemaForType(value reflect.Type) JSONSchema {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value == reflect.TypeOf(time.Time{}) {
		return JSONSchema{"type": "string", "format": "date-time"}
	}
	if enum := stressEnumValues(value); len(enum) > 0 {
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
			properties[name] = stressSchemaForType(field.Type)
			if !slices.Contains(parts[1:], "omitempty") {
				required = append(required, name)
			}
		}
		return JSONSchema{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
	case reflect.Slice, reflect.Array:
		return JSONSchema{"type": "array", "items": stressSchemaForType(value.Elem())}
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

func stressEnumValues(value reflect.Type) []string {
	switch value {
	case reflect.TypeOf(RelationKind("")):
		return stringsOf(KindInvariance, KindSensitivity, KindDifferential)
	case reflect.TypeOf(Unit("")):
		return stringsOf(UnitTrajectory, UnitCandidatePair, UnitTraceMapping, UnitEntrypoint, UnitProviderRoute, UnitExtractionPolicy)
	case reflect.TypeOf(TransformKind("")):
		return stringsOf(TransformMutation, TransformTraceMapping, TransformEntrypoint, TransformProviderRoute, TransformExtractionMode)
	case reflect.TypeOf(SourceRequirementKind("")):
		return stringsOf(RequirementV3Manifest, RequirementV3ConstructFirewall, RequirementFormalWitness, RequirementExactReplay, RequirementOwnerAttestation, RequirementTerminalLedger, RequirementOutcomeProof, RequirementPublicFixture, RequirementLiveAuthorization, RequirementCapsule)
	case reflect.TypeOf(InvalidState("")):
		return stringsOf(InvalidNotApplicable, InvalidSourceUnavailable, InvalidFormalWitness, InvalidConstructRejected, InvalidCustody, InvalidHumanContradicted, InvalidTransform, InvalidReplayMismatch, InvalidPrivacy, InvalidCrossVersion, InvalidLockedPartitionUsed)
	case reflect.TypeOf(Metric("")):
		return stringsOf(MetricDecision, MetricRank, MetricConditionalScore, MetricConditionalVariance, MetricSupportJaccard, MetricProbabilityOverlap, MetricCommonSupportDivergence, MetricVisibleMass, MetricValidMass, MetricUnobservedMass)
	case reflect.TypeOf(Operator("")):
		return stringsOf(OperatorEqual, OperatorNotEqual, OperatorLessOrEqual, OperatorGreaterOrEqual, OperatorOriginalPreferred, OperatorTransformedPreferred)
	case reflect.TypeOf(RepeatKind("")):
		return stringsOf(RepeatFixed, RepeatRegisteredAdaptive)
	case reflect.TypeOf(Estimand("")):
		return stringsOf(EstimandPrimaryCore, EstimandScarcitySentinel, EstimandSensitivity, EstimandDiagnostic)
	case reflect.TypeOf(Stage("")):
		return stringsOf(StageIngestion, StageRequestConstruction, StageProviderResponse, StageScoreExtraction, StageDecisionPolicy, StageRendering)
	case reflect.TypeOf(StageExpectationKind("")):
		return stringsOf(StageMustMatch, StageMustDiffer, StageMayDiffer)
	case reflect.TypeOf(Outcome("")):
		return stringsOf(OutcomeSatisfied, OutcomeViolated, OutcomeAbstained, OutcomeInvalid, OutcomeUnsupported, OutcomeProviderFailed, OutcomeInconclusive)
	case reflect.TypeOf(ConstraintStatus("")):
		return stringsOf(ConstraintSatisfied, ConstraintViolated, ConstraintAbstained, ConstraintUnsupported, ConstraintInconclusive)
	case reflect.TypeOf(AdmissionStatus("")):
		return stringsOf(AdmissionFormalOnly, AdmissionHumanSupported, AdmissionHumanContradicted, AdmissionHumanUnresolved)
	case reflect.TypeOf(ArmKind("")):
		return stringsOf(ArmScoreTokenVerifier, ArmExplicitTextJudge, ArmZeroCostControl, ArmProtocolAdapter)
	case reflect.TypeOf(ArmSupport("")):
		return stringsOf(ArmSupported, ArmUnsupported)
	case reflect.TypeOf(HeldOutCampaignExecutionClass("")):
		return stringsOf(HeldOutExecutionLiveProvider, HeldOutExecutionSealedProviderReplay, HeldOutExecutionDeterministicLocal)
	case reflect.TypeOf(HeldOutExecutionEvidenceAuthority("")):
		return stringsOf(HeldOutEvidenceLiveProviderReservation, HeldOutEvidenceSealedProviderReplay, HeldOutEvidenceDeterministicLocal)
	case reflect.TypeOf(HeldOutAdmissionEligibility("")):
		return stringsOf(HeldOutAdmissionEligible, HeldOutAdmissionMissingHumanResolution, HeldOutAdmissionHumanContradicted, HeldOutAdmissionHumanUnresolvedPrimaryCore)
	case reflect.TypeOf(ArmCellStatus("")):
		return stringsOf(ArmCellExecuted, ArmCellNotRun, ArmCellUnsupported)
	case reflect.TypeOf(ArmEvidenceKind("")):
		return stringsOf(ArmEvidenceReplay, ArmEvidenceZeroCost)
	case reflect.TypeOf(HeldOutReadinessGateStatus("")):
		return stringsOf(HeldOutReadinessPassed, HeldOutReadinessBlocked, HeldOutReadinessMissing)
	case reflect.TypeOf(AnalysisSplit("")):
		return stringsOf(AnalysisDevelopment, AnalysisCalibration, AnalysisTest)
	case reflect.TypeOf(AnalysisStatus("")):
		return stringsOf(AnalysisAdjustedComplete, AnalysisDescriptive, AnalysisIncomplete, AnalysisNotRun, AnalysisUnsupported)
	case reflect.TypeOf(WitnessBindingStatus("")):
		return stringsOf(WitnessBoundPrivate, WitnessBoundPublic, WitnessMissingCapsule, WitnessMissingCounterexample, WitnessMissingCapsuleAndCounterexample)
	case reflect.TypeOf(ReductionDecision("")):
		return stringsOf(ReductionAccepted, ReductionRejected)
	case reflect.TypeOf(ReductionMinimality("")):
		return stringsOf(ReductionOneMinimal)
	case reflect.TypeOf(preprocess.SourceFormat("")):
		return stringsOf(preprocess.SourcePlainText, preprocess.SourceClaudeCode, preprocess.SourceCodexRollout, preprocess.SourceOpenCode, preprocess.SourceTerminalBench, preprocess.SourceSWEbench, preprocess.SourceOTLPJSON, preprocess.SourceAgentTrace)
	case reflect.TypeOf(mutation.Family("")):
		values := make([]string, 0, len(mutation.Definitions()))
		for _, definition := range mutation.Definitions() {
			values = append(values, string(definition.Family))
		}
		slices.Sort(values)
		return values
	case reflect.TypeOf(mutation.InterventionClass("")):
		return stringsOf(mutation.ClassSemanticQuality, mutation.ClassPresentation, mutation.ClassEvidenceAvailability, mutation.ClassAdversarialClaim, mutation.ClassParserOnly)
	case reflect.TypeOf(mutation.Relation("")):
		return stringsOf(mutation.RelationOriginalBetter, mutation.RelationQualityEqual, mutation.RelationQualityEqualEvidenceLow, mutation.RelationVerifiedOutcomeWins, mutation.RelationNoControlEffect, mutation.RelationAmbiguous)
	default:
		return nil
	}
}

func stringsOf[T ~string](values ...T) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}
