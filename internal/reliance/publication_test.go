package reliance

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

type reliancePublicationFixture struct {
	request EvidenceRelianceMapRequest
}

var reliancePublicationFixtureCache struct {
	once    sync.Once
	fixture reliancePublicationFixture
	value   EvidenceRelianceMap
}

func TestEvidenceRelianceMapPublishesCanonicalBoundedProjections(t *testing.T) {
	_, value := cachedReliancePublicationFixture(t)
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(value.Terms) != 14 || len(value.ProfileDimensions) != 98 || len(value.PaperRows) != 98 ||
		value.RegisteredCells != 1_536 || value.SourceTasks != 24 || value.Scope.Empirical ||
		value.ProjectionProviderCalls != 0 || value.NetworkRequired {
		t.Fatalf("evidence reliance map dimensions or execution boundary = %+v", value)
	}
	if len(value.ArmComparisons) != 1 || len(value.ArmComparisons[0].Comparison.Contrasts) != 5 ||
		len(value.Witnesses) != 1 || value.Witnesses[0].BindingStatus != RelianceWitnessRelationBinding ||
		value.Witnesses[0].SourceAnalysisDigest != value.AnalysisDigest {
		t.Fatalf("evidence reliance publication bindings = %+v / %+v", value.ArmComparisons, value.Witnesses)
	}
	for _, contrast := range value.ArmComparisons[0].Comparison.Contrasts {
		if contrast.Support != RelianceArmContrastUnsupported || contrast.Reason == "" {
			t.Fatalf("confounded publication contrast was not explicit = %+v", contrast)
		}
	}
}

func TestEvidenceRelianceMapRejectsResealedProjectionTampering(t *testing.T) {
	_, value := cachedReliancePublicationFixture(t)
	mutations := []struct {
		name   string
		mutate func(*EvidenceRelianceMap)
	}{
		{"paper row", func(candidate *EvidenceRelianceMap) { candidate.PaperRows[0].RegisteredCells-- }},
		{"profile source", func(candidate *EvidenceRelianceMap) {
			candidate.ProfileDimensions[0].SourceAnalysisDigest = analysisDigest("foreign")
		}},
		{"term denominator", func(candidate *EvidenceRelianceMap) { candidate.Terms[0].Outcomes[0].ExcludedFromFit++ }},
		{"witness attribution", func(candidate *EvidenceRelianceMap) {
			candidate.Witnesses[0].BindingStatus = "factorial_cell_attributed"
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := cloneRelianceMapTestValue(t, value)
			mutation.mutate(&candidate)
			resealRelianceMapTestValue(t, &candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("resealed %s tampering was accepted", mutation.name)
			}
		})
	}
}

func TestEvidenceRelianceMapRequiresEveryPrespecifiedArmFamily(t *testing.T) {
	_, value := cachedReliancePublicationFixture(t)
	candidate := cloneRelianceMapTestValue(t, value)
	comparison := &candidate.ArmComparisons[0].Comparison
	comparison.Contrasts = comparison.Contrasts[:len(comparison.Contrasts)-1]
	comparison.MultiplicityFamilySize = len(comparison.Contrasts) * len(referenceFactorialTerms()) * len(reliancePublicationOutcomeIDs())
	digest, err := relianceArmComparisonDigest(*comparison)
	if err != nil {
		t.Fatal(err)
	}
	comparison.Digest = digest
	resealRelianceMapTestValue(t, &candidate)
	if err := candidate.Validate(); err == nil {
		t.Fatal("evidence reliance map accepted incomplete arm-contrast coverage")
	}
}

func cachedReliancePublicationFixture(t *testing.T) (reliancePublicationFixture, EvidenceRelianceMap) {
	t.Helper()
	reliancePublicationFixtureCache.once.Do(func() {
		fixture := buildReliancePublicationFixture(t)
		value, err := BuildEvidenceRelianceMap(fixture.request)
		if err != nil {
			t.Fatal(err)
		}
		reliancePublicationFixtureCache.fixture = fixture
		reliancePublicationFixtureCache.value = value
	})
	return reliancePublicationFixtureCache.fixture, reliancePublicationFixtureCache.value
}

func buildReliancePublicationFixture(t *testing.T) reliancePublicationFixture {
	t.Helper()
	analysis := relianceAnalysisFixture(t)
	selector := cachedSelectorAuditFixture(t)
	arms, specs := reliancePublicationArms(t, analysis)
	comparison, err := BuildRelianceArmComparison(analysis.preregistration, analysis.preflight, arms, specs)
	if err != nil {
		t.Fatal(err)
	}
	witness, witnessRequest, execution := buildRelianceWitnessFixture(t, false)
	request := EvidenceRelianceMapRequest{
		SelectorParents: selector.parents,
		SelectorAudit:   selector.audit,
		ArmComparisons: []RelianceArmComparisonEvidence{{
			Comparison: comparison, Arms: arms, Specs: specs,
		}},
		Witnesses: []RelianceWitnessPublicationEvidence{{
			Witness: witness, Request: witnessRequest.ExecutionRequest, Execution: execution,
		}},
	}
	return reliancePublicationFixture{request: request}
}

func reliancePublicationSelectorParents(value relianceAnalysisFixtureState) EvidenceSelectorAuditParents {
	return EvidenceSelectorAuditParents{
		Ontology: value.ontology, Estimands: value.estimands,
		Preregistration: value.preregistration, Preflight: value.preflight,
		Registration: value.registration, Executions: value.executions, Sources: value.sources,
	}
}

func reliancePublicationArms(
	t *testing.T,
	fixture relianceAnalysisFixtureState,
) ([]RelianceArmEvidence, []RelianceArmContrastSpec) {
	t.Helper()
	reference := comparisonArmEvidence(t, fixture, "arm-reference", "reference-family", fixture.registration.Arm)
	comparatorArm := fixture.registration.Arm
	comparatorArm.EvidencePolicy = verification.EvidenceExplicitJudge
	comparatorArm.Entrypoint = "comparison-entrypoint"
	comparatorArm.ProviderID = "second-provider"
	comparatorArm.RouteID = "route-" + analysisDigest("publication-mixed-arm")
	comparatorArm.RequestedModel = "second-model"
	comparator := comparisonJudgeArmEvidence(t, fixture, "arm-comparator", "second-family", comparatorArm)
	arms := []RelianceArmEvidence{reference, comparator}
	return arms, reliancePublicationContrastSpecs(reference, comparator)
}

func reliancePublicationContrastSpecs(
	reference RelianceArmEvidence,
	comparator RelianceArmEvidence,
) []RelianceArmContrastSpec {
	return []RelianceArmContrastSpec{
		{"evidence-policy", RelianceContrastEvidencePolicy, reference.ArmID, comparator.ArmID},
		{"entrypoint", RelianceContrastEntrypoint, reference.ArmID, comparator.ArmID},
		{"model-family", RelianceContrastModelFamily, reference.ArmID, comparator.ArmID},
		{"provider", RelianceContrastProvider, reference.ArmID, comparator.ArmID},
		{"route", RelianceContrastRoute, reference.ArmID, comparator.ArmID},
	}
}

func cloneRelianceMapTestValue(t *testing.T, value EvidenceRelianceMap) EvidenceRelianceMap {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result EvidenceRelianceMap
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func resealRelianceMapTestValue(t *testing.T, value *EvidenceRelianceMap) {
	t.Helper()
	digest, err := evidenceRelianceMapDigest(*value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
}
