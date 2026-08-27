package reliance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/outcome"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/stress"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestRelationBackedAdmissionSeparatesPrimaryAndSensitivityStrata(t *testing.T) {
	base := relationAdmissionFixture(t, EstimandEvidenceOnly, true)
	tests := []struct {
		name        string
		status      stress.AdmissionStatus
		wantStatus  InterventionAdmissibility
		wantReason  InterventionFailureCode
		wantPrimary bool
		wantSens    bool
	}{
		{"formal only", stress.AdmissionFormalOnly, InterventionUnresolved, FailureRelationFormalOnly, false, true},
		{"human supported", stress.AdmissionHumanSupported, InterventionAdmissible, "", false, true},
		{"human unresolved", stress.AdmissionHumanUnresolved, InterventionUnresolved, FailureRelationHumanUnresolved, false, true},
		{"human contradicted", stress.AdmissionHumanContradicted, InterventionInadmissible, FailureRelationHumanContradicted, false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs := relationInputsWithStatus(t, base, stress.EstimandSensitivity, test.status)
			assertRelationAdmission(t, inputs, test.wantStatus, test.wantReason, test.wantPrimary, test.wantSens)
		})
	}
	primary := relationInputsWithStatus(t, base, stress.EstimandPrimaryCore, stress.AdmissionHumanSupported)
	assertRelationAdmission(t, primary, InterventionAdmissible, "", true, false)
}

func TestRelationBackedAdmissionCanResolveOnlyDeclaredQualityChange(t *testing.T) {
	quality := relationAdmissionFixture(t, EstimandQualityChanging, true)
	qualityInputs := relationInputsWithStatus(t, quality, stress.EstimandPrimaryCore, stress.AdmissionHumanSupported)
	assertRelationAdmission(t, qualityInputs, InterventionAdmissible, "", true, false)

	missingOutcome := relationAdmissionFixture(t, EstimandEvidenceOnly, false)
	missingInputs := relationInputsWithStatus(t, missingOutcome, stress.EstimandPrimaryCore, stress.AdmissionHumanSupported)
	assertRelationAdmission(t, missingInputs, InterventionUnresolved, FailureOutcomePreservationMissing, false, false)
}

func TestRelationBackedAdmissionBindsFrozenParentsAndRejectsTampering(t *testing.T) {
	base := relationAdmissionFixture(t, EstimandEvidenceOnly, true)
	inputs := relationInputsWithStatus(t, base, stress.EstimandSensitivity, stress.AdmissionHumanSupported)
	assignmentsBefore := inputs.Assignments
	interventionBefore := inputs.Intervention
	value, err := BindRelationBackedIntervention(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(assignmentsBefore, inputs.Assignments) || !reflect.DeepEqual(interventionBefore, inputs.Intervention) {
		t.Fatal("relation admission rewrote a frozen assignment or intervention")
	}
	tampered := value
	tampered.AssignmentSetDigest = protocolkit.DigestBytes([]byte("foreign-assignment"))
	tampered.Digest, err = relationBackedAdmissionDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.Validate(inputs); err == nil {
		t.Fatal("digest-recomputed relation admission tampering was accepted")
	}
	inputs.ReplayedRelationCase.Transformed[0] = inputs.Parent
	_, err = BindRelationBackedIntervention(inputs)
	assertInterventionErrorCode(t, err, FailureRelationEvidenceInvalid)
}

type relationAdmissionFixtureState struct {
	ontology    FactorOntology
	estimands   EstimandCatalog
	assignments FactorAssignmentSet
	parent      preprocess.Trajectory
	result      EvidenceInterventionResult
}

func relationAdmissionFixture(t *testing.T, family EstimandFamily, withOutcome bool) relationAdmissionFixtureState {
	t.Helper()
	ontology, estimands, parent := interventionContracts(t)
	toolResult := eventOfKind(t, parent, preprocess.EventToolResult)
	assignments := interventionAssignments(t, ontology, parent, FactorToolOutput, toolResult.ID, PathToolResultStdout)
	replacement := []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "truncated output"}}
	request := EvidenceInterventionRequest{
		FactorID: FactorToolOutput, Operator: OperatorControlledReplacement, EstimandFamily: family,
		Targets: []InterventionTargetRequest{{
			EventID: toolResult.ID, FieldPath: PathToolResultStdout,
			Replacement: &InterventionValue{ContentParts: &replacement},
		}},
	}
	if withOutcome {
		request.SourceOutcome, request.IntervenedOutcome = relationAdmissionOutcomes(t, family)
	}
	result, err := ApplyEvidenceIntervention(ontology, estimands, assignments, parent, request)
	if err != nil {
		t.Fatal(err)
	}
	return relationAdmissionFixtureState{ontology: ontology, estimands: estimands, assignments: assignments, parent: parent, result: result}
}

func relationAdmissionOutcomes(t *testing.T, family EstimandFamily) (*outcome.Record, *outcome.Record) {
	t.Helper()
	source := interventionOutcomeRecord(t, "relation-task", "relation-source", outcome.StateSolved)
	state := outcome.StateSolved
	if family == EstimandQualityChanging {
		state = outcome.StateUnsolved
	}
	intervened := interventionOutcomeRecord(t, "relation-task", "relation-intervened", state)
	return &source, &intervened
}

func relationInputsWithStatus(
	t *testing.T,
	base relationAdmissionFixtureState,
	estimand stress.Estimand,
	status stress.AdmissionStatus,
) RelationAdmissionInputs {
	t.Helper()
	spec := relationAdmissionStressRelation(t, estimand)
	admission := relationAdmissionConstruct(t, status)
	return RelationAdmissionInputs{
		Ontology: base.ontology, Estimands: base.estimands, Assignments: base.assignments,
		Parent: base.parent, Intervention: base.result, Relation: spec, ConstructAdmission: admission,
		ReplayedRelationCase: stress.ReplayedRelationCaseV3{
			CaseID: admission.CaseID, TaskGroupID: "relation-task", Family: mutation.FamilyToolOutputIncomplete,
			ManifestDigest:          protocolkit.DigestBytes([]byte("relation-manifest")),
			MutationProgramVersion:  mutation.MutationProgramVersionV3,
			RelationContractVersion: mutation.RelationContractVersionV3,
			FormalWitnessDigest:     admission.FormalWitnessDigest, ConstructFirewallDigest: admission.ConstructFirewallDigest,
			OutcomeEvidenceDigest: protocolkit.DigestBytes([]byte("relation-outcome")), RelationIDs: []string{spec.ID},
			Original: []preprocess.Trajectory{base.parent}, Transformed: []preprocess.Trajectory{base.result.Trajectory},
		},
	}
}

func relationAdmissionStressRelation(t *testing.T, estimand stress.Estimand) stress.Relation {
	return relationAdmissionStressRelationForFamily(t, estimand, mutation.FamilyToolOutputIncomplete)
}

func relationAdmissionStressRelationForFamily(
	t *testing.T,
	estimand stress.Estimand,
	family mutation.Family,
) stress.Relation {
	t.Helper()
	definition, found := mutation.DefinitionFor(family)
	if !found {
		t.Fatalf("mutation definition %q is missing", family)
	}
	kind := stress.KindSensitivity
	if definition.Relation == mutation.RelationQualityEqual || definition.Relation == mutation.RelationNoControlEffect {
		kind = stress.KindInvariance
	}
	requirements := relationAdmissionRequirements(estimand)
	draft := stress.Relation{
		ID: "reliance-" + string(estimand) + "-" + string(family), Revision: 1, Kind: kind,
		Applicability: stress.Applicability{
			Unit: stress.UnitTrajectory, MinimumTrajectories: 1, MaximumTrajectories: 1,
			RequiredSourceFormats: []preprocess.SourceFormat{preprocess.SourcePlainText}, Requirements: requirements,
		},
		Transform: stress.Transform{
			Kind: stress.TransformMutation, Identifier: definition.Operator, Version: mutation.MutationProgramVersionV3,
			MutationFamily: definition.Family, InterventionClass: definition.Class,
			ExpectedFormalRelation: definition.Relation, DeclaredChangedLayer: stress.StageIngestion,
		},
		Constraints: []stress.ExpectedConstraint{{
			ID: "score-movement", Metric: stress.MetricConditionalScore, Operator: stress.OperatorGreaterOrEqual, Required: true,
		}},
		InvalidStates:     []stress.InvalidState{stress.InvalidConstructRejected, stress.InvalidCustody, stress.InvalidFormalWitness, stress.InvalidPrivacy},
		Repeat:            stress.RepeatPolicy{Kind: stress.RepeatFixed, MinimumRepetitions: 1, MaximumRepetitions: 1, StopRule: "fixed_repetitions"},
		StatisticalFamily: relationAdmissionStatisticalFamily(estimand), StageExpectations: relationAdmissionStages(),
	}
	value, err := stress.SealRelation(draft)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func relationAdmissionRequirements(estimand stress.Estimand) []stress.SourceRequirement {
	values := []stress.SourceRequirement{
		{Kind: stress.RequirementV3Manifest, Value: mutation.ManifestSchemaVersion},
		{Kind: stress.RequirementV3ConstructFirewall, Value: mutation.ConstructFirewallSchemaVersionV2},
		{Kind: stress.RequirementFormalWitness, Value: mutation.WitnessSchemaVersion},
		{Kind: stress.RequirementExactReplay, Value: "required"},
		{Kind: stress.RequirementOwnerAttestation, Value: relationevidence.OwnerInspectionPublicAttestationSchemaVersion},
	}
	if estimand == stress.EstimandPrimaryCore {
		values = append(values, stress.SourceRequirement{
			Kind: stress.RequirementTerminalLedger, Value: relationevidence.TerminalRelationLedgerSchemaVersionV3,
		})
	}
	return values
}

func relationAdmissionStatisticalFamily(estimand stress.Estimand) stress.StatisticalFamily {
	denominator := stress.DenominatorSensitivityStratified
	if estimand == stress.EstimandPrimaryCore {
		denominator = stress.DenominatorPrimaryHumanSupported
	}
	return stress.StatisticalFamily{
		ID: "reliance-relation-family", Estimand: estimand, ClusterUnit: "source_task",
		MultiplicityMethod: "bonferroni", DenominatorPolicy: denominator,
		FailurePolicy: stress.FailurePolicy{
			Invalid: stress.OutcomeInvalid, MissingScore: stress.OutcomeAbstained, ProviderFailure: stress.OutcomeProviderFailed,
			RouteFailure: stress.OutcomeProviderFailed, Timeout: stress.OutcomeProviderFailed, Abstention: stress.OutcomeAbstained,
			BudgetExhaustion: stress.OutcomeInconclusive, RetryExhaustion: stress.OutcomeProviderFailed,
			IncompleteCell: stress.OutcomeInconclusive, Unsupported: stress.OutcomeUnsupported,
			DenominatorPolicy: stress.FailureDenominatorTreatment,
		},
	}
}

func relationAdmissionStages() []stress.StageExpectation {
	return []stress.StageExpectation{
		{Stage: stress.StageIngestion, Expectation: stress.StageMustDiffer},
		{Stage: stress.StageRequestConstruction, Expectation: stress.StageMayDiffer},
		{Stage: stress.StageProviderResponse, Expectation: stress.StageMayDiffer},
		{Stage: stress.StageScoreExtraction, Expectation: stress.StageMayDiffer},
		{Stage: stress.StageDecisionPolicy, Expectation: stress.StageMayDiffer},
		{Stage: stress.StageRendering, Expectation: stress.StageMayDiffer},
	}
}

func relationAdmissionConstruct(t *testing.T, status stress.AdmissionStatus) stress.ConstructAdmission {
	t.Helper()
	value := stress.ConstructAdmission{
		SchemaVersion: stress.AdmissionSchemaVersion, CanonicalPolicy: stress.CanonicalPolicy,
		CaseID: "relation-case", FormalWitnessDigest: protocolkit.DigestBytes([]byte("relation-witness")),
		ConstructFirewallDigest: protocolkit.DigestBytes([]byte("relation-firewall")),
		OwnerAttestationDigest:  protocolkit.DigestBytes([]byte("relation-owner")), Status: status,
		Reason: "relation admission fixture",
	}
	switch status {
	case stress.AdmissionFormalOnly:
		value.SensitivityEligible = true
	case stress.AdmissionHumanSupported:
		value.PrimaryEligible, value.SensitivityEligible = true, true
	case stress.AdmissionHumanUnresolved:
		value.SensitivityEligible = true
	case stress.AdmissionHumanContradicted:
	default:
		t.Fatalf("unsupported admission status %q", status)
	}
	if status != stress.AdmissionFormalOnly {
		value.TerminalLedgerDigest = protocolkit.DigestBytes([]byte("relation-ledger-" + string(status)))
		value.HumanResolutionDigest = protocolkit.DigestBytes([]byte("relation-resolution-" + string(status)))
	}
	value.Digest = stressAdmissionDigest(t, value)
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	return value
}

func stressAdmissionDigest(t *testing.T, value stress.ConstructAdmission) string {
	t.Helper()
	value.Digest = ""
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func assertRelationAdmission(
	t *testing.T,
	inputs RelationAdmissionInputs,
	wantStatus InterventionAdmissibility,
	wantReason InterventionFailureCode,
	wantPrimary, wantSensitivity bool,
) {
	t.Helper()
	value, err := BindRelationBackedIntervention(inputs)
	if err != nil {
		t.Fatal(err)
	}
	wantReasons := []InterventionFailureCode{}
	if wantReason != "" {
		wantReasons = []InterventionFailureCode{wantReason}
	}
	if value.Admissibility != wantStatus || !slices.Equal(value.AdmissibilityReasons, wantReasons) ||
		value.PrimaryEligible != wantPrimary || value.SensitivityEligible != wantSensitivity ||
		value.ProviderCalls != 0 || value.NetworkRequired {
		t.Fatalf("relation-backed admission state is invalid: %+v", value)
	}
	if err := value.Validate(inputs); err != nil {
		t.Fatal(err)
	}
}

func TestRelationBackedAdmissionRejectsFormalOnlyPrimaryRelation(t *testing.T) {
	base := relationAdmissionFixture(t, EstimandEvidenceOnly, true)
	inputs := relationInputsWithStatus(t, base, stress.EstimandPrimaryCore, stress.AdmissionFormalOnly)
	_, err := BindRelationBackedIntervention(inputs)
	var interventionErr *InterventionError
	if !errors.As(err, &interventionErr) || interventionErr.Code != FailureRelationEvidenceInvalid {
		t.Fatalf("formal-only primary relation error = %v", err)
	}
}
