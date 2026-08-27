package reliance

import (
	"reflect"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type selectorAuditFixture struct {
	parents EvidenceSelectorAuditParents
	audit   EvidenceSelectorAudit
}

var selectorAuditFixtureCache struct {
	once  sync.Once
	value selectorAuditFixture
}

func TestEvidenceSelectorAuditMapsAdjustedEffectsToProductionRetention(t *testing.T) {
	fixture := cachedSelectorAuditFixture(t)
	parents, audit := fixture.parents, fixture.audit
	if err := audit.Validate(parents); err != nil {
		t.Fatal(err)
	}
	assertSelectorAuditIdentity(t, audit)
	for _, factor := range audit.Factors {
		assertSelectorFactorCoverage(t, factor)
	}
	assertSelectorReferenceRisks(t, audit)
}

func TestEvidenceSelectorAuditRejectsResealedTamperingAndForeignSources(t *testing.T) {
	fixture := cachedSelectorAuditFixture(t)
	parents, audit := fixture.parents, fixture.audit
	tampered := audit
	tampered.Factors = append([]SelectorFactorAudit(nil), audit.Factors...)
	tampered.Factors[0].Budgets = append([]SelectorFactorBudgetAudit(nil), audit.Factors[0].Budgets...)
	tampered.Factors[0].Budgets[0].ExactTargets--
	var err error
	tampered.Digest, err = evidenceSelectorAuditDigest(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.Validate(parents); err == nil {
		t.Fatal("resealed selector-retention tampering was accepted")
	}
	foreign := parents
	foreign.Sources = append([]EvidenceSelectorAuditSource(nil), parents.Sources...)
	foreign.Sources[0].Trajectory.Digest = analysisDigest("foreign-selector-source")
	if _, err := BuildEvidenceSelectorAudit(foreign); err == nil {
		t.Fatal("selector audit accepted a source trajectory outside the registration")
	}
}

func TestSelectorVisibilityProbeDistinguishesZeroValueFromUnrenderedField(t *testing.T) {
	toolResult := preprocess.Event{Kind: preprocess.EventToolResult, ToolResult: &preprocess.ToolResultPayload{Error: false}}
	rendered, err := selectorTargetRendered(toolResult, PathToolResultError)
	if err != nil || !rendered {
		t.Fatalf("rendered false boolean visibility = %t, %v", rendered, err)
	}
	evaluation := preprocess.Event{Kind: preprocess.EventEvaluation, Evaluation: &preprocess.EvaluationPayload{Explanation: "hidden"}}
	rendered, err = selectorTargetRendered(evaluation, PathEvaluationExplanation)
	if err != nil || rendered {
		t.Fatalf("unrendered explanation visibility = %t, %v", rendered, err)
	}
}

func TestSelectorRiskPolicyPreservesNondetectionAndInconclusiveBoundaries(t *testing.T) {
	retention := selectorFactorRetentionFor(map[FactorID]*selectorFactorRetention{}, FactorMetadata, selectorAuditBudgets())
	for _, budget := range selectorAuditBudgets() {
		retention.budgets[budget].AssignmentTargets = 1
		retention.budgets[budget].ExactTargets = 1
	}
	notDetected := selectorFactorBudgets(retention, SelectorEffectNotDetected)
	inconclusive := selectorFactorBudgets(retention, SelectorEffectInconclusive)
	for index := range notDetected {
		if !reflect.DeepEqual(notDetected[index].RiskFlags, []SelectorRiskFlag{SelectorRiskUndetectedEffectRetained}) ||
			!reflect.DeepEqual(inconclusive[index].RiskFlags, []SelectorRiskFlag{SelectorRiskInconclusive}) {
			t.Fatalf("selector risk policy at budget %d = %v / %v", notDetected[index].BudgetTokens,
				notDetected[index].RiskFlags, inconclusive[index].RiskFlags)
		}
	}
}

func cachedSelectorAuditFixture(t *testing.T) selectorAuditFixture {
	t.Helper()
	selectorAuditFixtureCache.once.Do(func() {
		analysis := relianceAnalysisFixture(t)
		parents := reliancePublicationSelectorParents(analysis)
		audit, err := BuildEvidenceSelectorAudit(parents)
		if err != nil {
			t.Fatal(err)
		}
		selectorAuditFixtureCache.value = selectorAuditFixture{parents: parents, audit: audit}
	})
	return selectorAuditFixtureCache.value
}

func assertSelectorAuditIdentity(t *testing.T, audit EvidenceSelectorAudit) {
	t.Helper()
	wantPolicy := preprocess.InspectEvidenceSelectionPolicies()
	if audit.SchemaVersion != EvidenceSelectorAuditSchemaVersion || audit.PolicyVersion != EvidenceSelectorAuditPolicyVersion ||
		audit.EffectDetectionRule != SelectorEffectDetectionRule || audit.AdjustedEffectAlpha != referenceNominalAlpha ||
		audit.SourceTasks != 24 || audit.AssignmentTargets != 240 || len(audit.Factors) != 10 || len(audit.Categories) != 24 ||
		audit.ProviderCalls != 0 || audit.NetworkRequired || !audit.EventBytesAreNonAdditive ||
		!reflect.DeepEqual(audit.Budgets, []int{16_384, 32_768, 65_536}) || audit.ProductionPolicy != wantPolicy {
		t.Fatalf("selector audit identity = %+v", audit)
	}
	if audit.LegacyLinePolicy.Status != LegacyLineSelectorStatus ||
		audit.LegacyLinePolicy.Selector != preprocess.LegacyEvidenceSelector || len(audit.LegacyLinePolicy.Probes) != 6 {
		t.Fatalf("legacy selector boundary = %+v", audit.LegacyLinePolicy)
	}
}

func assertSelectorFactorCoverage(t *testing.T, factor SelectorFactorAudit) {
	t.Helper()
	if factor.AssignmentTargets != 24 || len(factor.Budgets) != 3 || len(factor.TermOutcomes) == 0 {
		t.Fatalf("selector factor coverage %s = %+v", factor.FactorID, factor)
	}
	for _, budget := range factor.Budgets {
		covered := budget.ExactTargets + budget.ChangedTargets + budget.UnrenderedTargets + budget.DroppedTargets
		if budget.AssignmentTargets != 24 || covered != 24 {
			t.Fatalf("selector factor budget coverage %s = %+v", factor.FactorID, budget)
		}
	}
}

func assertSelectorReferenceRisks(t *testing.T, audit EvidenceSelectorAudit) {
	t.Helper()
	irrelevant := selectorFactorAudit(t, audit, FactorIrrelevantVerbosity)
	metadata := selectorFactorAudit(t, audit, FactorMetadata)
	testResult := selectorFactorAudit(t, audit, FactorTestResult)
	if irrelevant.EffectStatus != SelectorEffectDetected || irrelevant.Budgets[0].DroppedTargets != 24 ||
		!reflect.DeepEqual(irrelevant.Budgets[0].RiskFlags, []SelectorRiskFlag{SelectorRiskDetectedEffectNonexact}) {
		t.Fatalf("irrelevant-verbosity selector result = %+v", irrelevant)
	}
	if metadata.EffectStatus != SelectorEffectDetected || metadata.Budgets[0].ExactTargets != 24 ||
		len(metadata.Budgets[0].RiskFlags) != 0 {
		t.Fatalf("metadata selector result = %+v", metadata)
	}
	if testResult.EffectStatus != SelectorEffectDetected || testResult.Budgets[0].UnrenderedTargets != 24 ||
		!reflect.DeepEqual(testResult.Budgets[0].RiskFlags, []SelectorRiskFlag{SelectorRiskDetectedEffectNonexact}) ||
		testResult.Budgets[2].UnrenderedTargets != 24 || testResult.Budgets[0].RetainedAssignedRenderedBytes == 0 {
		t.Fatalf("test-result selector result = %+v", testResult)
	}
}

func selectorFactorAudit(t *testing.T, audit EvidenceSelectorAudit, factorID FactorID) SelectorFactorAudit {
	t.Helper()
	for _, factor := range audit.Factors {
		if factor.FactorID == factorID {
			return factor
		}
	}
	t.Fatalf("selector audit lacks factor %q", factorID)
	return SelectorFactorAudit{}
}
