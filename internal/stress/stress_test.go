package stress

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
)

func TestRelationSealIsDeterministicAndRejectsScarcityAsPrimary(t *testing.T) {
	relation := testRelation(t, mutation.FamilyNeutralFormatting, EstimandSensitivity)
	resealed, err := SealRelation(relation)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(relation, resealed) {
		t.Fatal("sealing an already canonical relation changed its identity")
	}

	invalid := testRelationDraft(t, mutation.FamilyTestEvidenceOmitted, EstimandPrimaryCore)
	if _, err := SealRelation(invalid); err == nil {
		t.Fatal("omitted-evidence scarcity sentinel entered the primary-core estimand")
	}
}

func TestRelationRejectsStageExpectationsThatContradictItsTransform(t *testing.T) {
	value := testRelationDraft(t, mutation.FamilyNeutralFormatting, EstimandSensitivity)
	value.StageExpectations[0].Expectation = StageMustMatch
	if _, err := SealRelation(value); err == nil {
		t.Fatal("relation accepted a must-match expectation at its declared changed layer")
	}
}

func TestStageComparisonLocalizesObservedAndUnexpectedDivergence(t *testing.T) {
	relation := testRelation(t, mutation.FamilyNeutralFormatting, EstimandSensitivity)
	relation.StageExpectations = []StageExpectation{
		{Stage: StageIngestion, Expectation: StageMustDiffer},
		{Stage: StageRequestConstruction, Expectation: StageMayDiffer},
		{Stage: StageProviderResponse, Expectation: StageMustMatch},
		{Stage: StageScoreExtraction, Expectation: StageMustMatch},
		{Stage: StageDecisionPolicy, Expectation: StageMustMatch},
		{Stage: StageRendering, Expectation: StageMustMatch},
	}
	relation, err := SealRelation(relation)
	if err != nil {
		t.Fatal(err)
	}
	left := stageTraceFixture(t, "original", map[Stage]string{})
	right := stageTraceFixture(t, "transformed", map[Stage]string{
		StageIngestion:        "changed-ingestion",
		StageProviderResponse: "unexpected-provider-response",
	})
	comparison, err := CompareStageTraces(relation, left, right)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.EarliestDivergentStage != StageIngestion || comparison.EarliestUnexpectedStage != StageProviderResponse {
		t.Fatalf("earliest divergence = %q/%q", comparison.EarliestDivergentStage, comparison.EarliestUnexpectedStage)
	}
	if comparison.CausalityClaim != stageCausalityBoundary {
		t.Fatal("stage comparison expanded a digest difference into a causal claim")
	}

	tampered := comparison
	tampered.EarliestUnexpectedStage = StageScoreExtraction
	if err := tampered.Validate(); err == nil {
		t.Fatal("stage comparison accepted a tampered earliest-divergence summary")
	}

	differentUnit := right
	differentUnit.Records = append([]StageRecord(nil), right.Records...)
	replacement, err := NewStageRecord(StageIngestion, "different-canonical-unit", []byte("changed-ingestion"))
	if err != nil {
		t.Fatal(err)
	}
	differentUnit.Records[0] = replacement
	differentUnit.Digest, err = stageTraceDigest(differentUnit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompareStageTraces(relation, left, differentUnit); err == nil {
		t.Fatal("stage comparison accepted different canonical units")
	}
}

func TestCounterexampleValidatesEveryReductionDecisionAndDigestTransition(t *testing.T) {
	original := digestText("original")
	candidateRejected := digestText("candidate-rejected")
	candidateAccepted := digestText("candidate-accepted")
	finalCandidateRejected := digestText("final-candidate-rejected")
	relationDigest, privacyDigest := digestText("relation"), digestText("privacy")
	originalObservation := reductionObservationFixture(t, relationDigest, privacyDigest, true, true, true)
	rejectedObservation := reductionObservationFixture(t, relationDigest, privacyDigest, false, true, true)
	acceptedObservation := reductionObservationFixture(t, relationDigest, privacyDigest, true, true, true)
	finalObservation := reductionObservationFixture(t, relationDigest, privacyDigest, true, true, false)
	value := Counterexample{
		RelationDigest: relationDigest, SourceResultDigest: digestText("result"), CaseID: "mutation-example",
		OriginalInputDigest: original, ReducedInputDigest: candidateAccepted, PrivacyPolicyDigest: privacyDigest, PublicReleaseAllowed: true,
		Algorithm: deterministicRestartGreedy, Minimality: ReductionOneMinimal, OriginalUnits: []ReductionUnit{{Kind: "event", ID: "event-1"}, {Kind: "event", ID: "event-2"}},
		FinalUnits: []ReductionUnit{{Kind: "event", ID: "event-1"}}, OriginalObservation: originalObservation,
		Steps: []ReductionStep{
			{Index: 0, UnitKind: "event", UnitID: "event-1", BeforeDigest: original, CandidateDigest: candidateRejected, AfterDigest: original, Decision: ReductionRejected, Observation: rejectedObservation},
			{Index: 1, UnitKind: "event", UnitID: "event-2", BeforeDigest: original, CandidateDigest: candidateAccepted, AfterDigest: candidateAccepted, Decision: ReductionAccepted, Observation: acceptedObservation},
			{Index: 2, UnitKind: "event", UnitID: "event-1", BeforeDigest: candidateAccepted, CandidateDigest: finalCandidateRejected, AfterDigest: candidateAccepted, Decision: ReductionRejected, Observation: finalObservation},
		},
	}
	sealed, err := SealCounterexample(value)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.AcceptedReductions != 1 {
		t.Fatalf("accepted reductions = %d, want 1", sealed.AcceptedReductions)
	}

	tampered := sealed
	tampered.Steps = append([]ReductionStep(nil), sealed.Steps...)
	tampered.Steps[1].Observation.PrivacyRevalidated = false
	if err := tampered.Validate(); err == nil {
		t.Fatal("counterexample accepted a reduction without privacy revalidation")
	}

	misclassified := sealed
	misclassified.Steps = append([]ReductionStep(nil), sealed.Steps...)
	misclassified.Steps[0].Observation = acceptedObservation
	misclassified.Digest, err = counterexampleDigest(misclassified)
	if err != nil {
		t.Fatal(err)
	}
	if err := misclassified.Validate(); err == nil {
		t.Fatal("counterexample accepted a rejected step whose removal preserved every invariant")
	}
}

func reductionObservationFixture(t *testing.T, relationDigest, privacyDigest string, relationValid, privacyValid, violationPreserved bool) ReductionObservation {
	t.Helper()
	value, err := SealReductionObservation(ReductionObservation{
		RelationDigest: relationDigest, PrivacyPolicyDigest: privacyDigest,
		RelationRevalidated: relationValid, PrivacyRevalidated: privacyValid, ViolationPreserved: violationPreserved,
		RelationProofDigest: digestText("relation-proof"), PrivacyProofDigest: digestText("privacy-proof"), ReplayResultDigest: digestText("replay-result"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestConstraintAndResultEvidenceCannotContradictTheirObservations(t *testing.T) {
	relation := testRelation(t, mutation.FamilyNeutralFormatting, EstimandSensitivity)
	constraint := relation.Constraints[0]
	observed, err := EvaluateConstraint(constraint, nil, nil, "candidate-a", "candidate-a")
	if err != nil {
		t.Fatal(err)
	}
	admission := formalAdmission(t, "mutation-example")
	result, err := SealResult(relation, Result{
		CaseID: "mutation-example", TaskGroupID: "task-group-1", Admission: &admission, Outcome: OutcomeSatisfied,
		ConstraintResults: []ConstraintResult{observed}, DistributionComparisons: []TaggedDistributionComparison{},
		PlannedRepetitions: 1, CompletedRepetitions: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	tampered := result
	tampered.ConstraintResults = append([]ConstraintResult(nil), result.ConstraintResults...)
	tampered.ConstraintResults[0].Status = ConstraintViolated
	if err := tampered.ValidateAgainst(relation); err == nil {
		t.Fatal("stress result accepted a constraint status that contradicted its observations")
	}

	invalid, err := SealResult(relation, Result{
		CaseID: "mutation-invalid", TaskGroupID: "task-group-1", Outcome: OutcomeInvalid, InvalidState: InvalidCustody,
		ConstraintResults: []ConstraintResult{}, DistributionComparisons: []TaggedDistributionComparison{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invalid.ProviderCalls != 0 || invalid.Admission != nil {
		t.Fatal("pre-execution invalid result carried provider work or fabricated admission")
	}

	unsupported := Result{
		CaseID: "mutation-unsupported", TaskGroupID: "task-group-1", Admission: &admission, Outcome: OutcomeUnsupported,
		ConstraintResults: []ConstraintResult{}, DistributionComparisons: []TaggedDistributionComparison{},
	}
	if _, err := SealResult(relation, unsupported); err == nil {
		t.Fatal("unsupported stress result accepted fabricated construct admission")
	}

	contradicted := admission
	contradicted.Status = AdmissionHumanContradicted
	contradicted.SensitivityEligible = false
	contradicted.TerminalLedgerDigest = digestText("terminal-ledger")
	contradicted.HumanResolutionDigest = digestText("human-resolution")
	contradicted.Reason = "formal-human test contradiction"
	contradicted.Digest = ""
	contradictedDigest, err := constructAdmissionDigest(contradicted)
	if err != nil {
		t.Fatal(err)
	}
	contradicted.Digest = contradictedDigest
	contradictedResult, err := SealResult(relation, Result{
		CaseID: admission.CaseID, TaskGroupID: "task-group-1", Admission: &contradicted,
		Outcome: OutcomeInvalid, InvalidState: InvalidHumanContradicted,
		ConstraintResults: []ConstraintResult{}, DistributionComparisons: []TaggedDistributionComparison{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if contradictedResult.ProviderCalls != 0 || contradictedResult.Admission.Status != AdmissionHumanContradicted {
		t.Fatal("human contradiction was not retained as a zero-provider invalid result")
	}
}

func TestReplayAdmissionDenominatorsFailClosedBeforePlanning(t *testing.T) {
	formal := formalAdmission(t, "mutation-example")
	supported := admissionWithStatus(t, formal, AdmissionHumanSupported, true, true)
	unresolved := admissionWithStatus(t, formal, AdmissionHumanUnresolved, false, true)
	contradicted := admissionWithStatus(t, formal, AdmissionHumanContradicted, false, false)

	primaryDraft := testRelationDraft(t, mutation.FamilyNeutralFormatting, EstimandPrimaryCore)
	primaryDraft.Applicability.Requirements = append(primaryDraft.Applicability.Requirements, SourceRequirement{
		Kind: RequirementTerminalLedger, Value: relationevidence.TerminalRelationLedgerSchemaVersionV3,
	})
	primary, err := SealRelation(primaryDraft)
	if err != nil {
		t.Fatal(err)
	}
	sensitivity := testRelation(t, mutation.FamilyNeutralFormatting, EstimandSensitivity)

	tests := []struct {
		name      string
		relation  Relation
		admission ConstructAdmission
		wantError bool
	}{
		{name: "primary rejects formal only", relation: primary, admission: formal, wantError: true},
		{name: "primary rejects unresolved", relation: primary, admission: unresolved, wantError: true},
		{name: "primary accepts human supported", relation: primary, admission: supported},
		{name: "sensitivity accepts formal only", relation: sensitivity, admission: formal},
		{name: "sensitivity accepts unresolved", relation: sensitivity, admission: unresolved},
		{name: "sensitivity rejects contradicted", relation: sensitivity, admission: contradicted, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := stressVerificationInput(test.relation, test.admission, "original trajectory")
			transformed := stressVerificationInput(test.relation, test.admission, "transformed trajectory")
			err := validateReplayRunRequest(ReplayRunRequest{
				Relation: test.relation, Admission: test.admission, Original: original, Transformed: transformed,
			})
			if test.wantError && err == nil {
				t.Fatal("ineligible admission reached replay planning")
			}
			if !test.wantError && err != nil {
				t.Fatalf("eligible admission was rejected: %v", err)
			}
		})
	}
}

func TestPairComparisonConstraintUsesItsObservedLevel(t *testing.T) {
	target := 0.8
	constraint := ExpectedConstraint{
		ID: "support-overlap", Metric: MetricSupportJaccard, Operator: OperatorGreaterOrEqual,
		TargetValue: &target, AbsoluteTolerance: 0.01, Required: true,
	}
	satisfied, err := EvaluateComparisonConstraint(constraint, 0.81)
	if err != nil {
		t.Fatal(err)
	}
	violated, err := EvaluateComparisonConstraint(constraint, 0.78)
	if err != nil {
		t.Fatal(err)
	}
	if satisfied.Status != ConstraintSatisfied || violated.Status != ConstraintViolated || satisfied.ComparisonValue == nil || *satisfied.ComparisonValue != 0.81 {
		t.Fatalf("pair-comparison threshold result = %+v / %+v", satisfied, violated)
	}
	if _, err := EvaluateConstraint(constraint, &target, &target, "", ""); err == nil {
		t.Fatal("pair-comparison metric was accepted as an artificial side movement")
	}
}

func TestMovementConstraintsRejectImpossibleMetricDomains(t *testing.T) {
	tests := []struct {
		name, id string
		metric   Metric
		valid    float64
		invalid  float64
	}{
		{name: "rank below one", id: "rank", metric: MetricRank, valid: 1, invalid: 0},
		{name: "rank fractional", id: "rank-fractional", metric: MetricRank, valid: 2, invalid: 1.5},
		{name: "conditional score above one", id: "score", metric: MetricConditionalScore, valid: 0.5, invalid: 1.01},
		{name: "conditional variance above maximum", id: "variance", metric: MetricConditionalVariance, valid: 0.1, invalid: 0.251},
		{name: "visible mass below zero", id: "visible-mass", metric: MetricVisibleMass, valid: 0.5, invalid: -0.01},
		{name: "valid mass above one", id: "valid-mass", metric: MetricValidMass, valid: 0.5, invalid: 1.01},
		{name: "unobserved mass above one", id: "unobserved-mass", metric: MetricUnobservedMass, valid: 0.5, invalid: 1.01},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			constraint := ExpectedConstraint{ID: test.id, Metric: test.metric, Operator: OperatorEqual, Required: true}
			if _, err := EvaluateConstraint(constraint, &test.valid, &test.valid, "", ""); err != nil {
				t.Fatalf("valid metric observation was rejected: %v", err)
			}
			if _, err := EvaluateConstraint(constraint, &test.valid, &test.invalid, "", ""); err == nil {
				t.Fatal("impossible metric observation was accepted")
			}
		})
	}
}

func TestRankPreferenceUsesLowerRankAsBetter(t *testing.T) {
	originalPreferred := ExpectedConstraint{
		ID: "original-rank", Metric: MetricRank, Operator: OperatorOriginalPreferred,
		MinimumEffect: 1, TargetState: "", Required: true,
	}
	originalRank, transformedRank := 1.0, 2.0
	result, err := EvaluateConstraint(originalPreferred, &originalRank, &transformedRank, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ConstraintSatisfied {
		t.Fatalf("lower original rank was not preferred: %+v", result)
	}
	reversed, err := EvaluateConstraint(originalPreferred, &transformedRank, &originalRank, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if reversed.Status != ConstraintViolated {
		t.Fatalf("higher original rank was preferred: %+v", reversed)
	}

	transformedPreferred := originalPreferred
	transformedPreferred.ID = "transformed-rank"
	transformedPreferred.Operator = OperatorTransformedPreferred
	result, err = EvaluateConstraint(transformedPreferred, &transformedRank, &originalRank, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ConstraintSatisfied {
		t.Fatalf("lower transformed rank was not preferred: %+v", result)
	}
}

func TestCurrentOwnerAttestationFailsClosedAndPassedOwnerStillIsNotHumanAdmission(t *testing.T) {
	item, owner := currentMutationCaseAndOwner(t)
	spec := testRelation(t, item.Family, EstimandSensitivity)
	_, err := AdmitMutationCase(spec, item, owner, owner.PackageInventoryDigest, nil)
	var admissionErr *AdmissionError
	if !errors.As(err, &admissionErr) || admissionErr.State != InvalidCustody {
		t.Fatalf("current revision-required owner attestation error = %v", err)
	}

	passed := owner
	for index := range passed.Dimensions {
		passed.Dimensions[index].Passed = passed.Dimensions[index].Applicable
		passed.Dimensions[index].Failed = 0
		passed.Dimensions[index].Indeterminate = 0
	}
	passed.Outcomes = relationevidence.OwnerInspectionPublicOutcomes{
		Core:             relationevidence.OwnerInspectionPublicDispositionCounts{Accepted: passed.Assessments.CoreCases},
		ScarcityCases:    relationevidence.OwnerInspectionPublicDispositionCounts{Accepted: passed.Assessments.ScarcityCaseCount},
		ScarcityBoundary: relationevidence.PilotInspectionAccepted, CoreStatus: relationevidence.PilotInspectionOverallPassed,
		ScarcityStatus: relationevidence.PilotInspectionOverallPassed, OverallStatus: relationevidence.PilotInspectionOverallPassed,
	}
	passed, err = relationevidence.SealOwnerInspectionPublicAttestation(passed)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := AdmitMutationCase(spec, item, passed, passed.PackageInventoryDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if admission.Status != AdmissionFormalOnly || admission.PrimaryEligible || !admission.SensitivityEligible {
		t.Fatalf("passed owner inspection substituted for blinded human admission: %+v", admission)
	}
}

func TestV3AdmissionRejectsEarlierTerminalLedgerBeforeEntryLookup(t *testing.T) {
	_, err := verifiedLedgerEntry(relationevidence.TerminalRelationLedger{
		SchemaVersion:   relationevidence.TerminalRelationLedgerSchemaVersionV1,
		ProtocolVersion: relationevidence.ProtocolVersionV1,
	}, mutation.CorpusCaseV3{})
	var admissionErr *AdmissionError
	if !errors.As(err, &admissionErr) || admissionErr.State != InvalidCrossVersion {
		t.Fatalf("cross-version terminal-ledger error = %v", err)
	}
}

func TestSealedHistoricalConstructCasesAndV3ChallengeFailClosed(t *testing.T) {
	root := filepath.Join("..", "..")
	repairFile := openFixture(t, filepath.Join(root, "eval", "governance", "construct-repair-evidence-v1.json"))
	repair, err := mutation.DecodeConstructRepairEvidence(repairFile)
	if err != nil {
		t.Fatal(err)
	}
	rejections, err := HistoricalConstructRejections(repair)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejections) != repair.Summary.Fixtures {
		t.Fatalf("historical rejection count = %d, want %d", len(rejections), repair.Summary.Fixtures)
	}
	for _, rejection := range rejections {
		spec := testRelation(t, rejection.Family, EstimandSensitivity)
		_, err := AdmitHistoricalConstructCase(spec, repair, rejection.CaseID)
		var admissionErr *AdmissionError
		if !errors.As(err, &admissionErr) || admissionErr.State != InvalidCrossVersion {
			t.Fatalf("historical case %q admission error = %v", rejection.CaseID, err)
		}
	}

	challengeFile := openFixture(t, filepath.Join(root, "eval", "governance", "construct-firewall-challenge-v1.json"))
	challenge, err := mutation.DecodeConstructChallengeEvidence(challengeFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyV3ConstructChallenge(challenge); err != nil {
		t.Fatal(err)
	}
}

func TestStressSchemasCloseUnknownPropertiesAndPinVersions(t *testing.T) {
	documents := []struct {
		name, version string
	}{
		{"relation", RelationSchemaVersion}, {"result", ResultSchemaVersion}, {"stage-trace", StageTraceSchemaVersion},
		{"stage-comparison", StageComparisonSchemaVersion}, {"counterexample", CounterexampleSchemaVersion}, {"construct-admission", AdmissionSchemaVersion},
		{"reduction-observation", ReductionObservationSchemaVersion}, {"relation-registry", RelationRegistrySchemaVersion},
		{"replay-execution", ReplayExecutionSchemaVersion}, {"arm-comparison-plan", ArmComparisonPlanSchemaVersion},
		{"arm-comparison-report", ArmComparisonReportSchemaVersion},
		{"analysis-design", StressAnalysisDesignSchemaVersion}, {"analysis-report", StressAnalysisReportSchemaVersion},
		{"zero-cost-execution", ZeroCostExecutionSchemaVersion}, {"protocol-adapter-proof", ProtocolAdapterProofSchemaVersion},
		{"arm-replay-evidence", ArmReplayEvidenceSchemaVersion},
		{"held-out-partition-lock", HeldOutPartitionLockSchemaVersion}, {"held-out-campaign-plan", HeldOutCampaignPlanSchemaVersion},
		{"held-out-campaign-batch-binding", HeldOutCampaignBatchBindingSchemaVersion},
		{"held-out-admission-plan", HeldOutAdmissionPlanSchemaVersion},
		{"held-out-execution-batch-binding", HeldOutExecutionBatchBindingSchemaVersion},
		{"held-out-preflight-evidence", HeldOutPreflightEvidenceSchemaVersion},
		{"held-out-preflight-custody", HeldOutPreflightCustodySchemaVersion},
		{"held-out-execution-permit", HeldOutExecutionPermitSchemaVersion},
		{"held-out-execution-reservation", HeldOutExecutionReservationSchemaVersion},
		{"held-out-live-batch-evidence", HeldOutLiveBatchEvidenceSchemaVersion},
		{"held-out-live-replay-verification", HeldOutLiveReplayVerificationSchemaVersion},
		{"held-out-execution-ledger", HeldOutExecutionLedgerSchemaVersion},
		{"held-out-run-seal", HeldOutRunSealSchemaVersion},
		{"held-out-run-seal-v2", HeldOutRunSealV2SchemaVersion},
		{"held-out-run-readiness-refusal", HeldOutRunReadinessRefusalSchemaVersion},
		{"next-version-discovery-ledger", NextVersionDiscoverySchemaVersion},
		{"development-case-study", DevelopmentCaseStudySchemaVersion},
		{"development-challenge", DevelopmentChallengeSchemaVersion},
		{"development-challenge-receipt", DevelopmentChallengeReceiptSchemaVersion},
	}
	if len(documents) != 35 {
		t.Fatalf("stress schema inventory=%d want 35", len(documents))
	}
	for _, document := range documents {
		t.Run(document.name, func(t *testing.T) {
			schema, err := Schema(document.name)
			if err != nil {
				t.Fatal(err)
			}
			if schema["additionalProperties"] != false {
				t.Fatal("stress schema permits unknown root properties")
			}
			properties := schema["properties"].(map[string]any)
			version := properties["schema_version"].(JSONSchema)
			if version["const"] != document.version {
				t.Fatalf("schema version = %v", version["const"])
			}
		})
	}
}

func FuzzStageRecordDigestBindsCanonicalMaterial(f *testing.F) {
	f.Add([]byte("left"), []byte("right"))
	f.Fuzz(func(t *testing.T, left, right []byte) {
		if len(left) == 0 || len(right) == 0 || string(left) == string(right) {
			t.Skip()
		}
		leftRecord, err := NewStageRecord(StageIngestion, "canonical-trajectory", left)
		if err != nil {
			t.Fatal(err)
		}
		rightRecord, err := NewStageRecord(StageIngestion, "canonical-trajectory", right)
		if err != nil {
			t.Fatal(err)
		}
		if leftRecord.Digest == rightRecord.Digest {
			t.Fatal("distinct canonical stage materials produced one digest")
		}
	})
}

func testRelation(t *testing.T, family mutation.Family, estimand Estimand) Relation {
	t.Helper()
	value, err := SealRelation(testRelationDraft(t, family, estimand))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func testRelationDraft(t *testing.T, family mutation.Family, estimand Estimand) Relation {
	t.Helper()
	definition, exists := mutation.DefinitionFor(family)
	if !exists {
		t.Fatalf("unknown mutation family %q", family)
	}
	unit, minimum, maximum := UnitTrajectory, 1, 1
	if definition.PairLevel {
		unit, minimum, maximum = UnitCandidatePair, 2, 2
	}
	multiplicity := "holm"
	denominator := DenominatorSensitivityStratified
	if estimand == EstimandPrimaryCore {
		denominator = DenominatorPrimaryHumanSupported
	}
	if estimand == EstimandScarcitySentinel {
		multiplicity = "none_descriptive"
		denominator = DenominatorScarcityAvailability
	}
	kind := KindSensitivity
	if definition.Relation == mutation.RelationQualityEqual || definition.Relation == mutation.RelationNoControlEffect {
		kind = KindInvariance
	}
	return Relation{
		ID: "test-" + string(family), Revision: 1, Kind: kind,
		Applicability: Applicability{
			Unit: unit, MinimumTrajectories: minimum, MaximumTrajectories: maximum,
			RequiredSourceFormats: []preprocess.SourceFormat{preprocess.SourceTerminalBench},
			Requirements: []SourceRequirement{
				{Kind: RequirementV3Manifest, Value: mutation.ManifestSchemaVersion},
				{Kind: RequirementV3ConstructFirewall, Value: mutation.ConstructFirewallSchemaVersionV2},
				{Kind: RequirementFormalWitness, Value: mutation.WitnessSchemaVersion},
				{Kind: RequirementExactReplay, Value: "required"},
				{Kind: RequirementOwnerAttestation, Value: relationevidence.OwnerInspectionPublicAttestationSchemaVersion},
			},
		},
		Transform: Transform{
			Kind: TransformMutation, Identifier: definition.Operator, Version: mutation.MutationProgramVersionV3, MutationFamily: family,
			InterventionClass: definition.Class, ExpectedFormalRelation: definition.Relation, DeclaredChangedLayer: StageIngestion,
		},
		Constraints:   []ExpectedConstraint{{ID: "decision-stability", Metric: MetricDecision, Operator: OperatorEqual, Required: true}},
		InvalidStates: []InvalidState{InvalidPrivacy, InvalidCustody, InvalidFormalWitness, InvalidConstructRejected},
		Repeat:        RepeatPolicy{Kind: RepeatFixed, MinimumRepetitions: 1, MaximumRepetitions: 1, StopRule: "fixed_repetitions"},
		StatisticalFamily: StatisticalFamily{
			ID: "test-family", Estimand: estimand, ClusterUnit: "source_task_group", MultiplicityMethod: multiplicity,
			DenominatorPolicy: denominator, FailurePolicy: canonicalFailurePolicy(),
		},
		StageExpectations: []StageExpectation{
			{Stage: StageIngestion, Expectation: StageMustDiffer}, {Stage: StageRequestConstruction, Expectation: StageMayDiffer},
			{Stage: StageProviderResponse, Expectation: StageMayDiffer}, {Stage: StageScoreExtraction, Expectation: StageMayDiffer},
			{Stage: StageDecisionPolicy, Expectation: StageMayDiffer}, {Stage: StageRendering, Expectation: StageMayDiffer},
		},
	}
}

func stageTraceFixture(t *testing.T, side string, replacements map[Stage]string) StageTrace {
	t.Helper()
	records := make([]StageRecord, 0, len(orderedStages()))
	for _, stage := range orderedStages() {
		material := "stable-" + string(stage)
		if replacement, exists := replacements[stage]; exists {
			material = replacement
		}
		record, err := NewStageRecord(stage, "canonical-"+string(stage), []byte(material))
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	trace, err := SealStageTrace(side, records)
	if err != nil {
		t.Fatal(err)
	}
	return trace
}

func formalAdmission(t *testing.T, caseID string) ConstructAdmission {
	t.Helper()
	value := ConstructAdmission{
		SchemaVersion: AdmissionSchemaVersion, CanonicalPolicy: CanonicalPolicy, CaseID: caseID,
		FormalWitnessDigest: digestText("witness"), ConstructFirewallDigest: digestText("firewall"), OwnerAttestationDigest: digestText("owner"),
		Status: AdmissionFormalOnly, SensitivityEligible: true, Reason: "formal test admission",
	}
	digest, err := constructAdmissionDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	return value
}

func admissionWithStatus(t *testing.T, base ConstructAdmission, status AdmissionStatus, primaryEligible, sensitivityEligible bool) ConstructAdmission {
	t.Helper()
	base.Status = status
	base.PrimaryEligible = primaryEligible
	base.SensitivityEligible = sensitivityEligible
	base.TerminalLedgerDigest = digestText("terminal-ledger-" + string(status))
	base.HumanResolutionDigest = digestText("human-resolution-" + string(status))
	base.Reason = "formal-human test " + string(status)
	base.Digest = ""
	digest, err := constructAdmissionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	base.Digest = digest
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	return base
}

func currentMutationCaseAndOwner(t *testing.T) (mutation.CorpusCaseV3, relationevidence.OwnerInspectionPublicAttestation) {
	t.Helper()
	root := filepath.Join("..", "..")
	planFile := openFixture(t, filepath.Join(root, "eval", "governance", "controlled-corruption-v3-plan.json"))
	plan, err := mutation.DecodeCorpusDevelopmentPlan(planFile)
	if err != nil {
		t.Fatal(err)
	}
	auditFile := openFixture(t, filepath.Join(root, "eval", "governance", "controlled-corruption-v3-natural-audit.json"))
	audit, err := mutation.DecodeCorpusDevelopmentAuditV3(auditFile, plan)
	if err != nil {
		t.Fatal(err)
	}
	releaseFile := openFixture(t, filepath.Join(root, "eval", "governance", "controlled-corruption-v3-release.json"))
	release, err := mutation.DecodeCorpusReleaseV3(releaseFile, plan, audit)
	if err != nil {
		t.Fatal(err)
	}
	if len(release.Cases) == 0 {
		t.Fatal("v3 controlled-corruption release has no cases")
	}
	ownerFile := openFixture(t, filepath.Join(root, "eval", "results", "relation-owner-inspection-attestation.json"))
	owner, err := relationevidence.DecodeOwnerInspectionPublicAttestation(ownerFile)
	if err != nil {
		t.Fatal(err)
	}
	return release.Cases[0], owner
}

func openFixture(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close fixture %s: %v", path, err)
		}
	})
	return file
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
