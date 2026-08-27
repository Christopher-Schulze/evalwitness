package relation

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const ScarcityPublicBriefPolicy = "evalwitness.relation-scarcity-public-brief.v1"

type scarcityFunnel struct {
	Attempted        int
	Applied          int
	Rejected         int
	Selected         int
	RejectionReasons []mutation.CorpusCount
	Coverage         []mutation.ConstructCoverage
}

// VerifyScarcityPublicEvidence proves that the public governance chain supports
// one exact corpus-scarcity projection. It does not inspect restricted material
// and cannot create a human, provider, held-out, or verifier result.
func VerifyScarcityPublicEvidence(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3) error {
	if err := release.Validate(corpusPlan, audit); err != nil {
		return err
	}
	if err := sentinel.Validate(plan, primary); err != nil {
		return err
	}
	if plan.SourceCorpusDigest != release.Digest || plan.SourceCorpusPlanDigest != corpusPlan.Digest ||
		plan.SourceConstructAuditDigest != audit.Digest || plan.SourceMutationProgramDigest != release.MutationProgramDigest {
		return errors.New("scarcity public evidence does not bind the frozen corpus chain")
	}
	policy := release.Policy
	if policy.ScarcitySentinelFamily != sentinel.Family || policy.ScarcitySentinelCases != sentinel.SelectedCases ||
		!policy.ScarcitySentinelExhaustive || policy.SentinelInPrimaryEstimand || policy.HeldOutSentinelClaimAvailable ||
		policy.BalancedEightFamilyAvailable || policy.CoreCases != policy.CoreCasesPerFamily*len(policy.InferentialCoreFamilies) {
		return errors.New("scarcity public evidence release policy weakens the sentinel boundary")
	}
	if !sameSplitCounts(policy.ScarcitySentinelSplitCounts, sentinel.SplitCounts) {
		return errors.New("scarcity public evidence split roles disagree")
	}
	funnel, err := buildScarcityFunnel(audit, sentinel.Family)
	if err != nil {
		return err
	}
	if funnel.Applied != sentinel.SelectedCases || funnel.Selected != sentinel.SelectedCases ||
		policy.CoreCasesPerFamily-funnel.Selected != auditShortfall(audit, string(sentinel.Family)) {
		return errors.New("scarcity public evidence funnel does not reproduce the frozen shortfall")
	}
	return nil
}

// RenderScarcityPublicBriefMarkdown emits the deterministic human view of one
// validated public negative-evidence contract.
func RenderScarcityPublicBriefMarkdown(evidence ScarcityPublicEvidence) (string, error) {
	if err := evidence.Validate(); err != nil {
		return "", err
	}
	var builder strings.Builder
	renderScarcityPublicSummary(&builder, evidence)
	renderScarcityPublicFunnel(&builder, evidence)
	renderScarcityPublicRoles(&builder, evidence)
	renderScarcityPublicCommitments(&builder, evidence.Cases)
	renderScarcityPublicChain(&builder, evidence.Parents)
	renderScarcityPublicClaims(&builder, evidence.Claims)
	renderScarcityPublicCommand(&builder)
	return builder.String(), nil
}

func renderScarcityPublicSummary(builder *strings.Builder, evidence ScarcityPublicEvidence) {
	availability, roles := evidence.Availability, evidence.StudyRoles
	fmt.Fprintln(builder, "# EvalWitness Negative-Evidence Brief: Omitted Test Evidence")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "> This provider-free artifact reports what the frozen corpus could not support. It contains no restricted trajectory content, owner decision, human judgment, verifier result, provider result, held-out result, or authorization.")
	fmt.Fprintln(builder)
	fmt.Fprintf(builder, "Evidence contract: `%s`\n\nEvidence digest: `%s`\n\nBrief policy: `%s`\n\n", evidence.SchemaVersion, evidence.Digest, evidence.BriefPolicy)
	fmt.Fprintln(builder, "## Result at a glance")
	fmt.Fprintln(builder)
	fmt.Fprintf(builder, "The frozen construct firewall evaluated **%d** `%s` attempts. It admitted **%d**, rejected **%d**, and therefore left a **%d-case shortfall** against the descriptive %d-case availability target. The three admitted cases are exhaustive within this release, but their roles are two development, one calibration, and zero test.\n\n",
		availability.Attempted, evidence.ConstructFamily, availability.Admitted, availability.Rejected, availability.Shortfall, availability.Target)
	fmt.Fprintln(builder, "```text")
	fmt.Fprintf(builder, "%d attempted -> %d admitted -> %d selected -> %d development + %d calibration + %d test\n",
		availability.Attempted, availability.Admitted, availability.Selected, roles.Development, roles.Calibration, roles.Test)
	fmt.Fprintln(builder, "```")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "This is a corpus-specific construct-availability result. The shortfall was preserved instead of relaxing the eligibility predicate, fabricating a test split, or treating the sentinel as an eighth balanced family.")
}

func renderScarcityPublicFunnel(builder *strings.Builder, evidence ScarcityPublicEvidence) {
	fmt.Fprintln(builder, "\n## Eligibility funnel")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Source format | Attempted | Admitted | Rejected | Eligible task groups | Selected |")
	fmt.Fprintln(builder, "|---|---:|---:|---:|---:|---:|")
	for _, row := range evidence.Coverage {
		fmt.Fprintf(builder, "| `%s` | %d | %d | %d | %d | %d |\n", row.SourceFormat, row.Attempted, row.Admitted, row.Rejected, row.EligibleTaskGroups, row.Selected)
	}
	availability := evidence.Availability
	fmt.Fprintf(builder, "| **Total** | **%d** | **%d** | **%d** |  | **%d** |\n", availability.Attempted, availability.Admitted, availability.Rejected, availability.Selected)
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Closed rejection reason | Count |")
	fmt.Fprintln(builder, "|---|---:|")
	for _, reason := range evidence.RejectionReasons {
		fmt.Fprintf(builder, "| `%s` | %d |\n", markdownInline(reason.ID), reason.Count)
	}
}

func renderScarcityPublicRoles(builder *strings.Builder, evidence ScarcityPublicEvidence) {
	fmt.Fprintln(builder, "\n## Study-role boundary")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Role or use | Cases | Status |")
	fmt.Fprintln(builder, "|---|---:|---|")
	roles, core, primary := evidence.StudyRoles, evidence.InferentialCore, evidence.PrimarySample
	fmt.Fprintf(builder, "| development | %d | descriptive only |\n", roles.Development)
	fmt.Fprintf(builder, "| calibration | %d | descriptive only |\n", roles.Calibration)
	fmt.Fprintf(builder, "| test | %d | unavailable; no held-out claim |\n", roles.Test)
	fmt.Fprintf(builder, "| primary-estimand overlap | %d | excluded |\n", roles.PrimaryEstimandOverlap)
	fmt.Fprintf(builder, "| balanced inferential core | %d | %d separate families, %d cases each |\n", core.Cases, core.Families, core.CasesPerFamily)
	fmt.Fprintf(builder, "| locked relation primary sample | %d | %d task groups and %d lineage clusters |\n", primary.Cases, primary.TaskGroups, primary.LineageClusters)
}

func renderScarcityPublicCommitments(builder *strings.Builder, cases []ScarcityPublicCase) {
	fmt.Fprintln(builder, "\n## Public case commitments")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "The commitments below identify the three governed cases without publishing task text, source paths, trajectory excerpts, or owner notes.")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Case | Data role | Unit | Case binding | Construct firewall |")
	fmt.Fprintln(builder, "|---:|---|---|---|---|")
	for _, item := range cases {
		fmt.Fprintf(builder, "| %d | `%s` | `%s` | `%s` | `%s` |\n", item.Index, item.DataRole, item.Unit, item.CaseBindingDigest, item.ConstructFirewallDigest)
	}
}

func renderScarcityPublicChain(builder *strings.Builder, parents []ScarcityPublicParent) {
	fmt.Fprintln(builder, "\n## Evidence chain")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Artifact | Digest |")
	fmt.Fprintln(builder, "|---|---|")
	for _, parent := range parents {
		fmt.Fprintf(builder, "| %s | `%s` |\n", scarcityParentLabel(parent.ID), parent.Digest)
	}
}

func renderScarcityPublicClaims(builder *strings.Builder, claims []ScarcityPublicClaim) {
	fmt.Fprintln(builder, "\n## Claim boundary")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Claim | Evidence status |")
	fmt.Fprintln(builder, "|---|---|")
	for _, claim := range claims {
		fmt.Fprintf(builder, "| %s | %s |\n", markdownInline(claim.Claim), markdownInline(claim.Evidence))
	}
}

func renderScarcityPublicCommand(builder *strings.Builder) {
	fmt.Fprintln(builder, "\n## Reproduce offline")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "```bash")
	fmt.Fprintln(builder, "evalwitness relation render-scarcity-public-brief \\")
	fmt.Fprintln(builder, "  --format markdown \\")
	fmt.Fprintln(builder, "  --plan @eval/governance/relation-audit-plan-v3.json \\")
	fmt.Fprintln(builder, "  --primary-sample @eval/governance/relation-primary-sample-v3.json \\")
	fmt.Fprintln(builder, "  --scarcity-sentinel @eval/governance/relation-scarcity-sentinel-v3.json \\")
	fmt.Fprintln(builder, "  --corpus-plan @eval/governance/controlled-corruption-v3-plan.json \\")
	fmt.Fprintln(builder, "  --corpus-audit @eval/governance/controlled-corruption-v3-natural-audit.json \\")
	fmt.Fprintln(builder, "  --release @eval/governance/controlled-corruption-v3-release.json")
	fmt.Fprintln(builder, "```")
}

func scarcityParentLabel(id string) string {
	switch id {
	case "corpus_development_plan":
		return "corpus development plan"
	case "natural_corpus_audit":
		return "939-attempt natural audit"
	case "controlled_corpus_release":
		return "typed 283-case release"
	case "relation_governance_plan":
		return "relation governance plan"
	case "relation_primary_sample":
		return "balanced primary sample"
	case "relation_scarcity_sentinel":
		return "exhaustive scarcity sentinel"
	default:
		return "unknown parent"
	}
}

func buildScarcityFunnel(audit mutation.CorpusDevelopmentAuditV3, family mutation.Family) (scarcityFunnel, error) {
	reasons := make(map[string]int)
	funnel := scarcityFunnel{}
	for _, row := range audit.Coverage {
		if row.Family != family {
			continue
		}
		funnel.Coverage = append(funnel.Coverage, row)
		funnel.Attempted += row.Attempted
		funnel.Applied += row.Applied
		funnel.Rejected += row.Rejected
		funnel.Selected += row.SelectedCases
		for _, reason := range row.RejectionReasonCounts {
			reasons[reason.ID] += reason.Count
		}
	}
	if len(funnel.Coverage) == 0 || funnel.Attempted != funnel.Applied+funnel.Rejected {
		return scarcityFunnel{}, errors.New("scarcity public evidence has no complete family funnel")
	}
	for id, count := range reasons {
		funnel.RejectionReasons = append(funnel.RejectionReasons, mutation.CorpusCount{ID: id, Count: count})
	}
	sort.Slice(funnel.RejectionReasons, func(left, right int) bool {
		return funnel.RejectionReasons[left].ID < funnel.RejectionReasons[right].ID
	})
	return funnel, nil
}

func sameSplitCounts(left []mutation.CorpusCount, right []Count) bool {
	if len(left) != len(right) {
		return false
	}
	for _, item := range left {
		if relationSplitCount(right, item.ID) != item.Count {
			return false
		}
	}
	return true
}

func auditShortfall(audit mutation.CorpusDevelopmentAuditV3, id string) int {
	for _, item := range audit.QuotaShortfalls {
		if item.ID == id {
			return item.Count
		}
	}
	return 0
}

func relationSplitCount(values []Count, id string) int {
	for _, item := range values {
		if item.ID == id {
			return item.Count
		}
	}
	return 0
}
