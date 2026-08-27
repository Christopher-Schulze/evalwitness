package relation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const ScarcityInspectionPolicy = "evalwitness.relation-scarcity-owner-inspection.v1"

// VerifyScarcityInspectionMaterials verifies that the restricted materials are
// an exact, one-per-case rendering surface for the governed v3 scarcity
// sentinel. It does not create a judgment, result, or authorization state.
func VerifyScarcityInspectionMaterials(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, materials []CaseMaterial) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if err := primary.Validate(plan); err != nil {
		return err
	}
	if err := sentinel.Validate(plan, primary); err != nil {
		return err
	}
	if len(materials) != sentinel.SelectedCases || len(materials) != len(sentinel.Cases) {
		return errors.New("scarcity inspection requires exactly one restricted material per sentinel case")
	}

	materialByCase := make(map[string]CaseMaterial, len(materials))
	for _, material := range materials {
		if err := material.Validate(); err != nil {
			return fmt.Errorf("scarcity inspection material %q: %w", material.CaseID, err)
		}
		if _, duplicate := materialByCase[material.CaseID]; duplicate {
			return fmt.Errorf("scarcity inspection contains duplicate material for case %q", material.CaseID)
		}
		materialByCase[material.CaseID] = material
	}

	for _, reference := range sentinel.Cases {
		material, exists := materialByCase[reference.CaseID]
		if !exists {
			return fmt.Errorf("scarcity inspection lacks material for governed case %q", reference.CaseID)
		}
		if material.ProtocolVersion != ProtocolVersionV3 || material.Objective != ReviewObjectiveControlledRelation ||
			material.PlanDigest != plan.Digest || material.SourceCorpusDigest != sentinel.SourceCorpusDigest ||
			material.SourceCorpusPlanDigest != sentinel.SourceCorpusPlanDigest ||
			material.SourceConstructAuditDigest != sentinel.SourceConstructAuditDigest ||
			material.SourceMutationProgramDigest != sentinel.SourceMutationProgramDigest ||
			material.CaseID != reference.CaseID || material.Family != reference.Family || material.Unit != reference.Unit ||
			material.ConstructFirewallDigest != reference.ConstructFirewallDigest {
			return fmt.Errorf("scarcity inspection material %q does not bind its governed sentinel reference", reference.CaseID)
		}
	}
	return nil
}

// VerifyScarcityInspectionReplay reconstructs every sentinel material from the
// frozen source chain and requires byte-identity through its canonical digest.
func VerifyScarcityInspectionReplay(root string, plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3, materials []CaseMaterial) error {
	if err := VerifyScarcityInspectionMaterials(plan, primary, sentinel, materials); err != nil {
		return err
	}
	reviewPlan, err := ReviewPlanV3(plan)
	if err != nil {
		return err
	}
	materialByCase := make(map[string]CaseMaterial, len(materials))
	for _, material := range materials {
		materialByCase[material.CaseID] = material
	}
	for _, reference := range sentinel.Cases {
		reproduced, err := MaterializeCaseV3(root, reviewPlan, corpusPlan, audit, release, reference.CaseID, RelationEvidenceBudgetTokens)
		if err != nil {
			return fmt.Errorf("reproduce scarcity inspection material %q: %w", reference.CaseID, err)
		}
		if reproduced.Digest != materialByCase[reference.CaseID].Digest {
			return fmt.Errorf("scarcity inspection material %q does not reproduce from the frozen source chain", reference.CaseID)
		}
	}
	return nil
}

// RenderScarcityInspectionMarkdown renders a deterministic owner-only surface
// for inspecting corpus scarcity and construct availability. The output
// intentionally has no machine decision fields and cannot enter the primary
// estimand or reviewer bundle.
func RenderScarcityInspectionMarkdown(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, materials []CaseMaterial) (string, error) {
	if err := VerifyScarcityInspectionMaterials(plan, primary, sentinel, materials); err != nil {
		return "", err
	}
	materialByCase := make(map[string]CaseMaterial, len(materials))
	for _, material := range materials {
		materialByCase[material.CaseID] = material
	}

	var builder strings.Builder
	fmt.Fprintln(&builder, "# EvalWitness Owner-Only Scarcity Sentinel Inspection")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "- Inspection policy: `%s`\n", ScarcityInspectionPolicy)
	fmt.Fprintf(&builder, "- Governance plan digest: `%s`\n", plan.Digest)
	fmt.Fprintf(&builder, "- Primary sample digest: `%s`\n", primary.Digest)
	fmt.Fprintf(&builder, "- Scarcity sentinel digest: `%s`\n", sentinel.Digest)
	fmt.Fprintf(&builder, "- Source corpus digest: `%s`\n", sentinel.SourceCorpusDigest)
	fmt.Fprintf(&builder, "- Source corpus plan digest: `%s`\n", sentinel.SourceCorpusPlanDigest)
	fmt.Fprintf(&builder, "- Source construct audit digest: `%s`\n", sentinel.SourceConstructAuditDigest)
	fmt.Fprintf(&builder, "- Selected cases: %d exhaustive natural cases\n", sentinel.SelectedCases)
	fmt.Fprintf(&builder, "- Analysis use: `%s`\n", sentinel.AnalysisUse)
	fmt.Fprintf(&builder, "- Empirical status: `%s`\n", sentinel.EmpiricalStatus)
	fmt.Fprintf(&builder, "- External action: `%s`\n\n", sentinel.ExternalActionStatus)
	fmt.Fprintln(&builder, "> OWNER-ONLY RESTRICTED SURFACE. This appendix contains reference-only task and trajectory evidence. It is not a pilot packet, reviewer kit, human judgment, provider result, held-out test, primary-estimand input, or authorization to contact or distribute.")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "## Frozen scarcity boundary")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "Selection rule: %s\n\n", sentinel.SelectionRule)
	fmt.Fprintln(&builder, "| Property | Frozen value |")
	fmt.Fprintln(&builder, "|---|---:|")
	fmt.Fprintf(&builder, "| Exhaustive within frozen release | `%t` |\n", sentinel.Exhaustive)
	fmt.Fprintf(&builder, "| Primary overlap | %d |\n", sentinel.PrimaryOverlap)
	fmt.Fprintf(&builder, "| Test cases | %d |\n", sentinel.TestCases)
	fmt.Fprintf(&builder, "| Held-out sentinel claim available | `%t` |\n", sentinel.HeldOutClaimAvailable)
	for _, count := range sentinel.SplitCounts {
		fmt.Fprintf(&builder, "| Data role `%s` | %d |\n", markdownInline(count.ID), count.Count)
	}
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "Interpretation boundary: the artifact establishes the exact availability and reviewability of this construct in the frozen corpus. It cannot establish population prevalence, held-out performance, reviewer agreement, verifier robustness, provider behavior, or human ground truth.")

	for index, reference := range sentinel.Cases {
		material := materialByCase[reference.CaseID]
		fmt.Fprintf(&builder, "\n## Sentinel case %d of %d\n\n", index+1, len(sentinel.Cases))
		fmt.Fprintf(&builder, "- Case ID: `%s`\n", markdownInline(reference.CaseID))
		fmt.Fprintf(&builder, "- Family: `%s`\n", reference.Family)
		fmt.Fprintf(&builder, "- Data role: `%s`\n", reference.DataRole)
		fmt.Fprintf(&builder, "- Unit: `%s`\n", reference.Unit)
		fmt.Fprintf(&builder, "- Task group: `%s`\n", markdownInline(reference.TaskGroupID))
		fmt.Fprintf(&builder, "- Source IDs: %s\n", scarcityInlineValues(reference.SourceIDs))
		fmt.Fprintf(&builder, "- Lineage clusters: %s\n", scarcityInlineValues(reference.LineageClusterIDs))
		fmt.Fprintf(&builder, "- Case binding digest: `%s`\n", reference.CaseBindingDigest)
		fmt.Fprintf(&builder, "- Construct firewall digest: `%s`\n", reference.ConstructFirewallDigest)
		fmt.Fprintf(&builder, "- Material digest: `%s`\n", material.Digest)
		fmt.Fprintf(&builder, "- Replay receipt digest: `%s`\n", material.ReplayReceiptDigest)
		fmt.Fprintf(&builder, "- Alignment digest: `%s`\n\n", material.AlignmentDigest)
		fmt.Fprintln(&builder, "### Task requirement")
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, fencedUntrusted(material.TaskRequirement))
		renderScarcityEvidence(&builder, "Original evidence", material.Original)
		renderScarcityEvidence(&builder, "Transformed evidence", material.Transformed)
		fmt.Fprintln(&builder, "### Owner observation prompts")
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, "These prompts are inspection notes only. They do not constitute a sealed judgment or empirical result.")
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, "- [ ] The original surface contains task-relevant test evidence.")
		fmt.Fprintln(&builder, "- [ ] The transformed surface omits the targeted test evidence while preserving the rest of the trajectory relation.")
		fmt.Fprintln(&builder, "- [ ] The paired rendering exposes enough local and outcome context to inspect the omission.")
		fmt.Fprintln(&builder, "- [ ] Any ambiguity, additional change, or evidence-budget loss is recorded outside this immutable package.")
		fmt.Fprintln(&builder, "- Notes: ________________________________________________________________________________")
	}

	fmt.Fprintln(&builder, "\n## Owner completion gate")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "- [ ] All three exhaustive sentinel cases were inspected, including the two development cases and one calibration case.")
	fmt.Fprintln(&builder, "- [ ] The absence of a test-role case was retained as a scarcity finding, not relabeled as held-out evidence.")
	fmt.Fprintln(&builder, "- [ ] Sentinel observations were kept separate from the seven-family pilot and 28-case primary estimand.")
	fmt.Fprintln(&builder, "- [ ] No reviewer, provider, publication, or distribution action was inferred or authorized from this appendix.")
	return builder.String(), nil
}

func renderScarcityEvidence(builder *strings.Builder, title string, excerpts []EvidenceExcerpt) {
	fmt.Fprintf(builder, "\n### %s\n\n", title)
	for index, excerpt := range excerpts {
		fmt.Fprintf(builder, "#### Evidence %d\n\n", index+1)
		fmt.Fprintf(builder, "Source `%s`; retained `%s`; content `%s`; lineage `%s`; retained %d/%d events; omitted %d; selector `%s`; license `%s`; visibility `%s`.\n\n",
			excerpt.SourceTrajectoryDigest, excerpt.RetainedTrajectoryDigest, excerpt.ContentDigest, excerpt.RetainedLineageDigest,
			excerpt.RetainedEvents, excerpt.SourceEvents, excerpt.OmittedEvents, excerpt.EvidenceSelector,
			markdownInline(excerpt.LicenseSPDX), excerpt.Visibility)
		fmt.Fprintln(builder, fencedUntrusted(excerpt.Content))
	}
}

func scarcityInlineValues(values []string) string {
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = "`" + markdownInline(value) + "`"
	}
	return strings.Join(encoded, ", ")
}
