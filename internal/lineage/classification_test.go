package lineage

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestEarliestLossClassificationCoversClosedTerminalContract(t *testing.T) {
	contract := terminalStateContract()
	units := make([]ClassifiedLineageUnit, 0, len(contract))
	for index, rule := range contract {
		input := classificationInputForState(index, rule.State)
		unit, err := ClassifyEarliestLoss(input)
		if err != nil {
			t.Fatalf("classify %s: %v", rule.State, err)
		}
		if unit.TerminalState != rule.State || unit.StateDisposition != rule.Disposition || unit.PrecedenceApplied != rule.Precedence {
			t.Fatalf("classification differs from contract for %s: %#v", rule.State, unit)
		}
		units = append(units, unit)
	}

	summary, err := SummarizeClassifications(units)
	if err != nil {
		t.Fatal(err)
	}
	if err := summary.Validate(); err != nil {
		t.Fatal(err)
	}
	if summary.ConsideredTaskGroups != 10 || summary.IncludedTaskGroups != 9 || summary.ExcludedTaskGroups != 1 || summary.UnresolvedTaskGroups != 3 || len(summary.Formats) != 1 {
		t.Fatalf("unexpected classification totals: %#v", summary)
	}
	format := summary.Formats[0]
	if format.DirectInvocations != 8 || format.CandidateEvidenceEvents != 8 || format.SourceSessions != 10 {
		t.Fatalf("unexpected format evidence totals: %#v", format)
	}
	wantFlows := []LayerFlow{
		{FromLayer: "runtime_witness", ToLayer: "native_export", Entered: 9, Survived: 7, Lost: 2},
		{FromLayer: "native_export", ToLayer: "canonical_graph", Entered: 7, Survived: 4, Lost: 3},
		{FromLayer: "canonical_graph", ToLayer: "retained_bundle", Entered: 4, Survived: 3, Lost: 1},
		{FromLayer: "retained_bundle", ToLayer: "verifier_request", Entered: 3, Survived: 1, Lost: 2},
	}
	if !slices.Equal(format.Flows, wantFlows) {
		t.Fatalf("unexpected exact flows: got %#v want %#v", format.Flows, wantFlows)
	}
}

func TestEarliestLossWinsWhenLaterConditionsAreAlsoProven(t *testing.T) {
	input := classificationInputForState(2, StateExportObservabilityAbsent)
	input.Findings[9] = TerminalFinding{State: StateDirectVerificationInvocation, Status: FindingProven, ProofRecordIDs: []string{"proof-direct"}}
	unit, err := ClassifyEarliestLoss(input)
	if err != nil {
		t.Fatal(err)
	}
	if unit.TerminalState != StateExportObservabilityAbsent || unit.PrecedenceApplied != 3 {
		t.Fatalf("later condition displaced earliest proven loss: %#v", unit)
	}
}

func TestEarliestLossClassificationRejectsEvidenceGapsAndInvocationContradictions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ClassificationInput)
	}{
		{name: "missing state", mutate: func(input *ClassificationInput) { input.Findings = input.Findings[:9] }},
		{name: "out of order", mutate: func(input *ClassificationInput) { input.Findings[0].State = StateBehaviorAbsent }},
		{name: "skipped precedence", mutate: func(input *ClassificationInput) {
			input.Findings[0] = TerminalFinding{State: StateInvalidCapture, Status: FindingNotEvaluated}
		}},
		{name: "unproved direct state", mutate: func(input *ClassificationInput) { input.Findings[9].Status = FindingDisproven }},
		{name: "missing proof", mutate: func(input *ClassificationInput) { input.Findings[9].ProofRecordIDs = nil }},
		{name: "invocation contradiction", mutate: func(input *ClassificationInput) { input.DirectInvocationObserved = false }},
		{name: "event contradiction", mutate: func(input *ClassificationInput) { input.CandidateEvidenceEvents = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := classificationInputForState(9, StateDirectVerificationInvocation)
			test.mutate(&input)
			if _, err := ClassifyEarliestLoss(input); err == nil {
				t.Fatal("invalid classification evidence was accepted")
			}
		})
	}
}

func TestClassificationSummaryRejectsDuplicateAndTamperedUnits(t *testing.T) {
	first, err := ClassifyEarliestLoss(classificationInputForState(9, StateDirectVerificationInvocation))
	if err != nil {
		t.Fatal(err)
	}
	duplicate := first
	duplicate.UnitID = "different-unit"
	if _, err := SummarizeClassifications([]ClassifiedLineageUnit{first, duplicate}); err == nil {
		t.Fatal("duplicate format/task-group terminal classification was accepted")
	}

	tampered := first
	tampered.LayerSurvival[4] = false
	if _, err := SummarizeClassifications([]ClassifiedLineageUnit{tampered}); err == nil {
		t.Fatal("tampered layer survival was accepted")
	}
}

func TestClassificationSummaryPreservesAnAllExcludedFormatWithoutInventingADenominator(t *testing.T) {
	unit, err := ClassifyEarliestLoss(classificationInputForState(0, StateInvalidCapture))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := SummarizeClassifications([]ClassifiedLineageUnit{unit})
	if err != nil {
		t.Fatal(err)
	}
	if err := summary.Validate(); err != nil {
		t.Fatal(err)
	}
	if summary.ConsideredTaskGroups != 1 || summary.IncludedTaskGroups != 0 || summary.ExcludedTaskGroups != 1 || summary.Formats[0].Flows[0].Entered != 0 {
		t.Fatalf("all-excluded format invented an analytic denominator: %#v", summary)
	}
}

func TestClassificationSummarySupportsPairedFormatViewsWithoutMergingThem(t *testing.T) {
	first, err := ClassifyEarliestLoss(classificationInputForState(9, StateDirectVerificationInvocation))
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.UnitID = "unit-second-format"
	second.Format = "opencode_export"
	summary, err := SummarizeClassifications([]ClassifiedLineageUnit{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if summary.ConsideredTaskGroups != 2 || summary.IncludedTaskGroups != 2 || len(summary.Formats) != 2 || summary.Formats[0].Format != "codex_rollout" || summary.Formats[1].Format != "opencode_export" {
		t.Fatalf("paired format views were merged or unsorted: %#v", summary)
	}
}

func TestLineageAuditIsBuiltFromTheSameExclusiveClassificationsAndFlows(t *testing.T) {
	contract := terminalStateContract()
	units := make([]ClassifiedLineageUnit, 0, len(contract))
	parents := []ParentRef{{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-plan-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest}}
	for index, rule := range contract {
		unit, err := ClassifyEarliestLoss(classificationInputForState(index, rule.State))
		if err != nil {
			t.Fatal(err)
		}
		units = append(units, unit)
		parents = append(parents, ParentRef{
			Relation: "assessment", SchemaVersion: AssessmentSchemaVersion, ObjectID: fmt.Sprintf("assessment-%02d", index+1),
			TaskID: "TASK-069", TaskGroupID: unit.TaskGroupID, Digest: fmt.Sprintf("%064x", index+1),
		})
	}
	parents = append(parents, ParentRef{
		Relation: "capability", SchemaVersion: CapabilitySchemaVersion, ObjectID: "capability-codex",
		TaskID: "TASK-069", TaskGroupID: "study", Digest: strings.Repeat("a", 64),
	})
	header := ArtifactHeader{
		SchemaVersion: AuditSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
		ObjectID: "audit-classification-test", TaskID: "TASK-069", TaskGroupID: "study", DataRole: RoleAdapterDevelopment,
		PlanDigest: LockedPlanDigest, Parents: parents,
	}
	audit, err := BuildLineageAudit(
		header, "audit-classification-test", strings.Repeat("b", 64), strings.Repeat("c", 64),
		[]string{"captured_formats_only", "no_provider_ranking"}, units,
	)
	if err != nil {
		t.Fatal(err)
	}
	if audit.ConsideredTaskGroups != 10 || audit.IncludedTaskGroups != 9 || audit.ExcludedTaskGroups != 1 || audit.UnresolvedTaskGroups != 3 || !audit.ConservationPassed {
		t.Fatalf("audit differs from classification summary: %#v", audit)
	}
	mutated := audit
	mutated.ExcludedTaskGroups++
	mutated.Header.Digest = ""
	mutated.Header.Digest = sealArtifactForTest(t, mutated)
	if err := mutated.Validate(); err == nil {
		t.Fatal("audit with non-conserving excluded total was accepted")
	}
}

func classificationInputForState(selected int, state TerminalState) ClassificationInput {
	contract := terminalStateContract()
	findings := make([]TerminalFinding, len(contract))
	for index, rule := range contract {
		status := FindingNotEvaluated
		proofs := []string(nil)
		if index < selected {
			status = FindingDisproven
			proofs = []string{fmt.Sprintf("proof-%02d-disproven", index+1)}
		}
		if index == selected {
			status = FindingProven
			proofs = []string{fmt.Sprintf("proof-%02d-proven", index+1)}
		}
		findings[index] = TerminalFinding{State: rule.State, Status: status, ProofRecordIDs: proofs}
	}
	direct := stateRequiresObservedInvocation(state)
	candidates := 0
	if direct {
		candidates = 1
	}
	return ClassificationInput{
		UnitID: fmt.Sprintf("unit-%02d", selected+1), TaskGroupID: fmt.Sprintf("group-%02d", selected+1),
		Format: "codex_rollout", SourceSessionID: fmt.Sprintf("session-%02d", selected+1),
		CandidateEvidenceEvents: candidates, DirectInvocationObserved: direct, Findings: findings,
	}
}
