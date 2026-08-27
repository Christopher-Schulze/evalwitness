package relation

import (
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestScarcityInspectionBindsEveryGovernedCaseWithoutCreatingAResult(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)
	materials := make([]CaseMaterial, len(sentinel.Cases))
	for index, reference := range sentinel.Cases {
		materials[index] = testScarcityMaterial(t, plan, reference)
	}

	rendered, err := RenderScarcityInspectionMarkdown(plan, primary, sentinel, materials)
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range sentinel.Cases {
		if !strings.Contains(rendered, reference.CaseID) || !strings.Contains(rendered, reference.CaseBindingDigest) {
			t.Fatalf("rendered scarcity inspection omitted governed case %q", reference.CaseID)
		}
	}
	for _, required := range []string{
		ScarcityInspectionPolicy,
		"descriptive_only_excluded_from_primary_estimand",
		"It is not a pilot packet",
		"Held-out sentinel claim available | `false`",
		"Empirical status: `not_run`",
		"External action: `not_authorized`",
	} {
		if !strings.Contains(rendered, required) {
			t.Fatalf("rendered scarcity inspection omitted boundary %q", required)
		}
	}
	if strings.Contains(rendered, "overall_status") || strings.Contains(rendered, "human_study_status=run") {
		t.Fatal("scarcity inspection rendered a fabricated result state")
	}
}

func TestScarcityInspectionRejectsMissingDuplicateAndCrossBoundMaterial(t *testing.T) {
	plan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", plan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", plan, primary)
	materials := make([]CaseMaterial, len(sentinel.Cases))
	for index, reference := range sentinel.Cases {
		materials[index] = testScarcityMaterial(t, plan, reference)
	}

	if err := VerifyScarcityInspectionMaterials(plan, primary, sentinel, materials[:2]); err == nil {
		t.Fatal("scarcity inspection accepted a missing governed case")
	}
	duplicate := append([]CaseMaterial(nil), materials...)
	duplicate[2] = duplicate[1]
	if err := VerifyScarcityInspectionMaterials(plan, primary, sentinel, duplicate); err == nil {
		t.Fatal("scarcity inspection accepted duplicate material")
	}
	crossBound := append([]CaseMaterial(nil), materials...)
	crossBound[0].ConstructFirewallDigest = digestText("different-firewall")
	resealed, err := SealCaseMaterial(crossBound[0])
	if err != nil {
		t.Fatal(err)
	}
	crossBound[0] = resealed
	if err := VerifyScarcityInspectionMaterials(plan, primary, sentinel, crossBound); err == nil {
		t.Fatal("scarcity inspection accepted material bound to a different construct firewall")
	}
}

func testScarcityMaterial(t *testing.T, plan RelationPlanV3, reference GovernedCaseReferenceV3) CaseMaterial {
	t.Helper()
	lineage := digestText("scarcity-lineage-" + reference.CaseID)
	excerpt := func(label string) EvidenceExcerpt {
		content := label + " controlled evidence for " + reference.CaseID
		return EvidenceExcerpt{
			SourceTrajectoryDigest:   digestText("source-" + reference.CaseID),
			RetainedTrajectoryDigest: digestText("retained-" + label + reference.CaseID),
			SourceEvents:             2, RetainedEvents: 2, OmittedEvents: 0,
			EvidenceBudgetTokens: RelationEvidenceBudgetTokens, EvidenceSelector: RelationEvidenceSelectorVersion,
			RequiredEventIDs: []string{"event-1"}, RetainedLineageDigest: lineage,
			Content: content, ContentDigest: digestText(content), LicenseSPDX: "MIT",
			SourceURL: "https://example.invalid/source", SourceRevision: "frozen-revision",
			Redistribution: "reference_only", Visibility: restrictedReferenceVisibility, PublicReleasable: false,
		}
	}
	requirement := "Inspect omitted test evidence for governed case " + reference.CaseID
	material, err := SealCaseMaterial(CaseMaterial{
		ProtocolVersion: ProtocolVersionV3, Objective: ReviewObjectiveControlledRelation,
		PlanDigest: plan.Digest, SourceCorpusDigest: plan.SourceCorpusDigest, SourceCorpusPlanDigest: plan.SourceCorpusPlanDigest,
		SourceMutationProgramDigest: plan.SourceMutationProgramDigest, SourceConstructAuditDigest: plan.SourceConstructAuditDigest,
		RelationContractVersion: mutation.RelationContractVersionV3, EvidenceBoundaryVersion: mutation.EvidenceBoundaryVersionV3,
		ConstructFirewallDigest: reference.ConstructFirewallDigest, CaseID: reference.CaseID,
		Family: reference.Family, Unit: reference.Unit, TaskRequirement: requirement, TaskRequirementDigest: digestText(requirement),
		Original: []EvidenceExcerpt{excerpt("original")}, Transformed: []EvidenceExcerpt{excerpt("transformed")},
		AlignmentDigest: lineage, ReplayReceiptDigest: digestText("replay-" + reference.CaseID),
		Limitations: []string{
			"Deterministic paired slicing may omit judgment-relevant evidence; omitted event counts remain explicit.",
			"Materialization proves reproducibility, alignment, redaction, and accounting, not human semantic sufficiency.",
			"Restricted reference-only excerpts are not public-release artifacts.",
		},
		ExternalActionStatus: ExternalActionNotAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	return material
}
