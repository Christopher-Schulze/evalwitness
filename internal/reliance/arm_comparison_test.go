package reliance

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestRelianceArmComparisonFitsOnlyPrespecifiedPairedDimensions(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	routeArm := fixture.registration.Arm
	routeArm.RouteID = "route-" + analysisDigest("comparison-route")
	route := comparisonArmEvidence(t, fixture, "arm-route", "reference-family", routeArm)
	entrypointArm := fixture.registration.Arm
	entrypointArm.Entrypoint = "reliance-comparison-entrypoint"
	entrypoint := comparisonArmEvidence(t, fixture, "arm-entrypoint", "reference-family", entrypointArm)
	specs := []RelianceArmContrastSpec{
		{"route-contrast", RelianceContrastRoute, reference.ArmID, route.ArmID},
		{"entrypoint-contrast", RelianceContrastEntrypoint, reference.ArmID, entrypoint.ArmID},
	}
	comparison, err := BuildRelianceArmComparison(
		fixture.preregistration, fixture.preflight, []RelianceArmEvidence{route, reference, entrypoint}, specs,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := comparison.Validate(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{route, reference, entrypoint}, specs); err != nil {
		t.Fatal(err)
	}
	assertRelianceArmComparison(t, comparison, 196)
}

func TestRelianceArmComparisonRetainsUnpairedCellsInDenominator(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	routeArm := fixture.registration.Arm
	routeArm.RouteID = "route-" + analysisDigest("comparison-missing-route")
	comparator := comparisonArmEvidence(t, fixture, "arm-route", "reference-family", routeArm)
	failedTask := comparator.Registration.SourceTaskIDs()[23]
	comparator.Executions = comparator.Executions[:23]
	comparator.Failures = comparisonTaskFailures(t, comparator.Registration, fixture.preregistration, failedTask)
	spec := RelianceArmContrastSpec{"route-contrast", RelianceContrastRoute, reference.ArmID, comparator.ArmID}
	comparison, err := BuildRelianceArmComparison(
		fixture.preregistration, fixture.preflight, []RelianceArmEvidence{reference, comparator}, []RelianceArmContrastSpec{spec},
	)
	if err != nil {
		t.Fatal(err)
	}
	contrast := comparison.Contrasts[0]
	if contrast.RegisteredPairs != 1_536 || contrast.EligiblePairs != 1_472 ||
		pairingStatusCells(contrast.PairingStatusCounts, ReliancePairReferenceOnly) != 64 {
		t.Fatalf("paired denominator = %+v", contrast)
	}
	for _, outcome := range contrast.OutcomeFits {
		if outcome.RegisteredPairs != 1_536 || outcome.EligiblePairs != 1_472 || outcome.ExcludedFromFit != 64 {
			t.Fatalf("paired outcome denominator = %+v", outcome)
		}
	}
}

func TestRelianceArmComparisonMarksConfoundedContrastUnsupported(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	mixedArm := fixture.registration.Arm
	mixedArm.RouteID = "route-" + analysisDigest("comparison-mixed-route")
	mixedArm.ProviderID = "foreign-provider"
	mixedArm.RequestedModel = "foreign-model"
	mixed := comparisonArmEvidence(t, fixture, "arm-mixed", "foreign-family", mixedArm)
	spec := RelianceArmContrastSpec{"route-contrast", RelianceContrastRoute, reference.ArmID, mixed.ArmID}
	comparison, err := BuildRelianceArmComparison(
		fixture.preregistration, fixture.preflight, []RelianceArmEvidence{reference, mixed}, []RelianceArmContrastSpec{spec},
	)
	if err != nil {
		t.Fatal(err)
	}
	contrast := comparison.Contrasts[0]
	if contrast.Support != RelianceArmContrastUnsupported || contrast.Reason == "" || len(contrast.OutcomeFits) != 0 ||
		!reflect.DeepEqual(contrast.ChangedDimensions, []string{"model_family", "provider", "requested_model", "route"}) {
		t.Fatalf("confounded route contrast = %+v", contrast)
	}
}

func TestRelianceArmComparisonSupportsProviderContrast(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	providerArm := fixture.registration.Arm
	providerArm.ProviderID = "second-provider"
	providerArm.RouteID = "route-" + analysisDigest("comparison-provider-route")
	comparator := comparisonArmEvidence(t, fixture, "arm-provider", "reference-family", providerArm)
	spec := RelianceArmContrastSpec{"provider-contrast", RelianceContrastProvider, reference.ArmID, comparator.ArmID}
	comparison, err := BuildRelianceArmComparison(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{reference, comparator}, []RelianceArmContrastSpec{spec})
	if err != nil {
		t.Fatal(err)
	}
	contrast := comparison.Contrasts[0]
	if contrast.Support != RelianceArmContrastSupported ||
		!reflect.DeepEqual(contrast.ChangedDimensions, []string{"provider", "route"}) {
		t.Fatalf("provider contrast = %+v", contrast)
	}
}

func TestRelianceArmComparisonRequiresBoundNamedFamilyEvidence(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	modelArm := fixture.registration.Arm
	modelArm.RouteID = "route-" + analysisDigest("comparison-model-route")
	modelArm.RequestedModel = "second-reference-model"
	comparator := comparisonArmEvidence(t, fixture, "arm-model", "second-family", modelArm)
	spec := RelianceArmContrastSpec{"model-family-contrast", RelianceContrastModelFamily, reference.ArmID, comparator.ArmID}
	comparison, err := BuildRelianceArmComparison(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{reference, comparator}, []RelianceArmContrastSpec{spec})
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Contrasts[0].Support != RelianceArmContrastSupported {
		t.Fatalf("named model-family contrast = %+v", comparison.Contrasts[0])
	}
	comparator.ModelIdentityStatus = RelianceModelIdentityAliasOnly
	comparison, err = BuildRelianceArmComparison(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{reference, comparator}, []RelianceArmContrastSpec{spec})
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Contrasts[0].Support != RelianceArmContrastUnsupported ||
		comparison.Contrasts[0].Reason != "model_family_identity_evidence_not_bound" {
		t.Fatalf("alias-only model-family contrast = %+v", comparison.Contrasts[0])
	}
}

func TestRelianceArmComparisonSupportsExplicitJudgePolicy(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-verifier", "reference-family", fixture.registration.Arm)
	judgeArm := fixture.registration.Arm
	judgeArm.EvidencePolicy = verification.EvidenceExplicitJudge
	judge := comparisonJudgeArmEvidence(t, fixture, "arm-judge", "reference-family", judgeArm)
	spec := RelianceArmContrastSpec{"evidence-policy-contrast", RelianceContrastEvidencePolicy, reference.ArmID, judge.ArmID}
	arms := []RelianceArmEvidence{reference, judge}
	specs := []RelianceArmContrastSpec{spec}
	comparison, err := BuildRelianceArmComparison(fixture.preregistration, fixture.preflight, arms, specs)
	if err != nil {
		t.Fatal(err)
	}
	if err := comparison.Validate(fixture.preregistration, fixture.preflight, arms, specs); err != nil {
		t.Fatal(err)
	}
	contrast := comparison.Contrasts[0]
	if contrast.Support != RelianceArmContrastSupported ||
		!reflect.DeepEqual(contrast.ChangedDimensions, []string{"evidence_policy"}) || contrast.EligiblePairs != 1_536 {
		t.Fatalf("explicit-judge evidence-policy contrast = %+v", contrast)
	}
}

func TestRelianceArmComparisonRejectsJudgeFromDifferentResponseRecords(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-verifier", "reference-family", fixture.registration.Arm)
	judgeArm := fixture.registration.Arm
	judgeArm.EvidencePolicy = verification.EvidenceExplicitJudge
	judge := comparisonJudgeArmEvidence(t, fixture, "arm-judge", "reference-family", judgeArm)
	judge.Executions[0].ReplaySource.ResponseBodySetDigest = analysisDigest("different-response-records")
	sealComparisonPanel(t, &judge.Executions[0])
	spec := RelianceArmContrastSpec{"evidence-policy-contrast", RelianceContrastEvidencePolicy, reference.ArmID, judge.ArmID}
	comparison, err := BuildRelianceArmComparison(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{reference, judge}, []RelianceArmContrastSpec{spec})
	if err != nil {
		t.Fatal(err)
	}
	contrast := comparison.Contrasts[0]
	if contrast.Support != RelianceArmContrastUnsupported || contrast.Reason != "evidence_policy_response_records_mismatch" {
		t.Fatalf("different-response judge contrast = %+v", contrast)
	}
}

func TestRelianceArmComparisonRejectsResealedResultTampering(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	routeArm := fixture.registration.Arm
	routeArm.RouteID = "route-" + analysisDigest("comparison-tamper-route")
	comparator := comparisonArmEvidence(t, fixture, "arm-route", "reference-family", routeArm)
	specs := []RelianceArmContrastSpec{{"route-contrast", RelianceContrastRoute, reference.ArmID, comparator.ArmID}}
	comparison, err := BuildRelianceArmComparison(
		fixture.preregistration, fixture.preflight, []RelianceArmEvidence{reference, comparator}, specs,
	)
	if err != nil {
		t.Fatal(err)
	}
	comparison.Contrasts[0].OutcomeFits[0].Fit.Estimates[0].Estimate = 0.5
	comparison.Digest, err = relianceArmComparisonDigest(comparison)
	if err != nil {
		t.Fatal(err)
	}
	if err := comparison.Validate(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{reference, comparator}, specs); err == nil {
		t.Fatal("resealed paired arm estimate tampering was accepted")
	}
}

func TestRelianceArmComparisonRejectsDifferentPresentationCells(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	routeArm := fixture.registration.Arm
	routeArm.RouteID = "route-" + analysisDigest("comparison-presentation-route")
	comparator := comparisonArmEvidence(t, fixture, "arm-route", "reference-family", routeArm)
	comparator.Executions[0].Cells = append([]EvidenceTaskPanelCell(nil), comparator.Executions[0].Cells...)
	comparator.Executions[0].Cells[0].PresentationDigest = analysisDigest("foreign-presentation-cell")
	digest, err := evidenceTaskPanelDigest(comparator.Executions[0])
	if err != nil {
		t.Fatal(err)
	}
	comparator.Executions[0].Digest = digest
	spec := RelianceArmContrastSpec{"route-contrast", RelianceContrastRoute, reference.ArmID, comparator.ArmID}
	if _, err := BuildRelianceArmComparison(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{reference, comparator}, []RelianceArmContrastSpec{spec}); err == nil {
		t.Fatal("paired arm comparison accepted different presentation cells")
	}
}

func TestRelianceArmComparisonRejectsDuplicatePrespecifiedContrast(t *testing.T) {
	fixture := relianceAnalysisFixture(t)
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	routeArm := fixture.registration.Arm
	routeArm.RouteID = "route-" + analysisDigest("comparison-duplicate-route")
	comparator := comparisonArmEvidence(t, fixture, "arm-route", "reference-family", routeArm)
	specs := []RelianceArmContrastSpec{
		{"route-contrast-a", RelianceContrastRoute, reference.ArmID, comparator.ArmID},
		{"route-contrast-b", RelianceContrastRoute, reference.ArmID, comparator.ArmID},
	}
	if _, err := BuildRelianceArmComparison(fixture.preregistration, fixture.preflight,
		[]RelianceArmEvidence{reference, comparator}, specs); err == nil {
		t.Fatal("duplicate prespecified contrast dimensions were accepted")
	}
}

func comparisonArmEvidence(
	t *testing.T,
	fixture relianceAnalysisFixtureState,
	armID string,
	modelFamilyID string,
	analysisArm RelianceAnalysisArm,
) RelianceArmEvidence {
	t.Helper()
	registration := comparisonRegistration(t, fixture.registration, analysisArm)
	executions := make([]EvidenceTaskPanelExecution, len(fixture.executions))
	for index, source := range fixture.executions {
		executions[index] = comparisonPanelExecution(t, source, analysisArm)
	}
	return RelianceArmEvidence{
		ArmID: armID, ModelFamilyID: modelFamilyID, ModelIdentityStatus: RelianceModelIdentityNamedFamilyEvidenceBound,
		RouteAttestationDigest: analysisDigest("route-attestation", armID),
		Registration:           registration, Executions: executions,
	}
}

func comparisonRegistration(
	t *testing.T,
	source ReliancePanelRegistration,
	arm RelianceAnalysisArm,
) ReliancePanelRegistration {
	t.Helper()
	value := source
	value.Arm = arm
	digest, err := reliancePanelRegistrationDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	return value
}

func comparisonJudgeArmEvidence(
	t *testing.T,
	fixture relianceAnalysisFixtureState,
	armID string,
	modelFamilyID string,
	analysisArm RelianceAnalysisArm,
) RelianceArmEvidence {
	t.Helper()
	registration := comparisonRegistration(t, fixture.registration, analysisArm)
	executions := make([]EvidenceTaskPanelExecution, len(fixture.executions))
	for index, source := range fixture.executions {
		executions[index] = comparisonJudgePanelExecution(t, source, analysisArm)
	}
	return RelianceArmEvidence{
		ArmID: armID, ModelFamilyID: modelFamilyID, ModelIdentityStatus: RelianceModelIdentityNamedFamilyEvidenceBound,
		RouteAttestationDigest: analysisDigest("route-attestation", armID), Registration: registration, Executions: executions,
	}
}

func comparisonPanelExecution(
	t *testing.T,
	source EvidenceTaskPanelExecution,
	arm RelianceAnalysisArm,
) EvidenceTaskPanelExecution {
	t.Helper()
	value := source
	value.Entrypoint, value.EvidencePolicy = arm.Entrypoint, arm.EvidencePolicy
	value.ReplaySource.ProviderID = arm.ProviderID
	value.ReplaySource.RouteID = arm.RouteID
	value.ReplaySource.RequestedModel = arm.RequestedModel
	digest, err := evidenceTaskPanelDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	return value
}

func comparisonJudgePanelExecution(
	t *testing.T,
	source EvidenceTaskPanelExecution,
	arm RelianceAnalysisArm,
) EvidenceTaskPanelExecution {
	t.Helper()
	value := source
	value.Entrypoint, value.EvidencePolicy = arm.Entrypoint, arm.EvidencePolicy
	value.ReplaySource.ProviderID, value.ReplaySource.RouteID = arm.ProviderID, arm.RouteID
	value.ReplaySource.RequestedModel = arm.RequestedModel
	baseline := judgePanelScoreEvidence(t, source.BaselineEvidence[0])
	value.BaselineState = verifier.DecisionSelected
	value.BaselineEvidence = []PanelScoreEvidence{baseline}
	value.Cells = make([]EvidenceTaskPanelCell, len(source.Cells))
	for index, cell := range source.Cells {
		value.Cells[index] = comparisonJudgePanelCell(t, cell, baseline)
	}
	sealComparisonPanel(t, &value)
	return value
}

func comparisonJudgePanelCell(
	t *testing.T,
	source EvidenceTaskPanelCell,
	baseline PanelScoreEvidence,
) EvidenceTaskPanelCell {
	t.Helper()
	value := source
	value.BaselineState, value.InterventionState = verifier.DecisionSelected, verifier.DecisionSelected
	value.DecisionFlip, value.AbstentionTransition = false, false
	contrast := source.CriterionContrasts[0]
	intervention := judgeScoreEvidence(t, contrast.InterventionEvidence)
	interventionDigest, err := referenceJSONDigest(intervention)
	if err != nil {
		t.Fatal(err)
	}
	contrast.BaselineEvidenceDigest = baseline.EvidenceDigest
	contrast.InterventionEvidence, contrast.InterventionEvidenceDigest = intervention, interventionDigest
	contrast.Comparison = verifier.CompareScoreEvidence(baseline.Evidence, intervention)
	value.CriterionContrasts = []PanelCriterionContrast{contrast}
	return value
}

func judgePanelScoreEvidence(t *testing.T, source PanelScoreEvidence) PanelScoreEvidence {
	t.Helper()
	value := source
	value.Evidence = judgeScoreEvidence(t, source.Evidence)
	digest, err := referenceJSONDigest(value.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	value.EvidenceDigest = digest
	return value
}

func judgeScoreEvidence(t *testing.T, source verifier.ScoreEvidence) verifier.ScoreEvidence {
	t.Helper()
	letter := chosenScoreLetter(t, source)
	tagName := strings.Trim(source.Tag, "<>")
	rawText := "<" + tagName + ">" + letter + "</" + tagName + ">"
	value := verifier.ExtractScoreEvidence(0, provider.ResponseRecord{RawText: rawText}, source.Tag, verifier.ExtractionModeJudge)
	if err := verifier.ValidateJudgeEvidence(map[string]verifier.ScoreEvidence{source.Tag: value}); err != nil {
		t.Fatal(err)
	}
	return value
}

func chosenScoreLetter(t *testing.T, source verifier.ScoreEvidence) string {
	t.Helper()
	letter := ""
	for _, alternative := range source.Alternatives {
		if !alternative.Chosen || alternative.CanonicalLetter == "" {
			continue
		}
		if letter != "" {
			t.Fatal("judge fixture source has multiple chosen score letters")
		}
		letter = alternative.CanonicalLetter
	}
	if letter == "" {
		t.Fatal("judge fixture source has no chosen score letter")
	}
	return letter
}

func sealComparisonPanel(t *testing.T, value *EvidenceTaskPanelExecution) {
	t.Helper()
	digest, err := evidenceTaskPanelDigest(*value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
}

func comparisonTaskFailures(
	t *testing.T,
	registration ReliancePanelRegistration,
	preregistration Preregistration,
	sourceTaskID string,
) []RelianceCellFailureReceipt {
	t.Helper()
	result := make([]RelianceCellFailureReceipt, ReferenceCellsPerTask)
	for cellIndex := range result {
		receipt, err := SealRelianceCellFailureReceipt(registration, preregistration, sourceTaskID, cellIndex,
			RelianceCellProviderFailed, "evalwitness.comparison-test-failure.v1",
			analysisDigest("comparison-failure", sourceTaskID, fmt.Sprint(cellIndex)), 1)
		if err != nil {
			t.Fatal(err)
		}
		result[cellIndex] = receipt
	}
	return result
}

func assertRelianceArmComparison(t *testing.T, comparison RelianceArmComparison, familySize int) {
	t.Helper()
	if comparison.MultiplicityFamilySize != familySize || len(comparison.Arms) != 3 || len(comparison.Contrasts) != 2 ||
		comparison.ProviderCalls != 0 || comparison.NetworkRequired {
		t.Fatalf("reliance arm comparison identity = %+v", comparison)
	}
	for _, contrast := range comparison.Contrasts {
		if contrast.Support != RelianceArmContrastSupported || contrast.RegisteredPairs != 1_536 ||
			contrast.EligiblePairs != 1_536 || len(contrast.OutcomeFits) != 7 {
			t.Fatalf("supported arm contrast = %+v", contrast)
		}
		for _, outcome := range contrast.OutcomeFits {
			if outcome.Status != RelianceFitMeasured || outcome.Fit == nil || outcome.Fit.FamilySize != familySize {
				t.Fatalf("paired arm outcome = %+v", outcome)
			}
			for _, estimate := range outcome.Fit.Estimates {
				if math.Abs(estimate.Estimate) > 1e-15 {
					t.Fatalf("identical paired arms produced estimate %+v", estimate)
				}
			}
		}
	}
}
