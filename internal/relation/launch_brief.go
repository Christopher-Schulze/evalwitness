package relation

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const PilotLaunchBriefVersion = "evalwitness.relation-pilot-launch-brief.v1"

func RenderPilotLaunchBriefMarkdown(dossier PilotLaunchDossier) (string, error) {
	if err := dossier.Validate(); err != nil {
		return "", err
	}

	disclosures := slices.Clone(dossier.PacketDisclosures)
	slices.SortFunc(disclosures, func(left, right PilotPacketDisclosure) int {
		return strings.Compare(string(left.Family), string(right.Family))
	})

	var builder strings.Builder
	fmt.Fprintln(&builder, "# EvalWitness Controlled-Relation Pilot Launch Brief")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "- Brief policy: `%s`\n", PilotLaunchBriefVersion)
	fmt.Fprintf(&builder, "- Review objective: `%s`\n", dossier.Objective)
	fmt.Fprintf(&builder, "- Data role: `%s`\n", dossier.DataRole)
	fmt.Fprintf(&builder, "- Launch status: `%s`\n", dossier.LaunchStatus)
	fmt.Fprintf(&builder, "- Owner inspection: `%s`\n", dossier.OwnerInspectionStatus)
	fmt.Fprintf(&builder, "- Human study: `%s`\n", dossier.HumanStudyStatus)
	fmt.Fprintf(&builder, "- External action: `%s`\n\n", dossier.ExternalActionStatus)
	fmt.Fprintln(&builder, "> PUBLIC-SAFE DECISION SURFACE. This brief contains structural protocol facts only. It contains no packet identity, digest, task text, trajectory evidence, source identity, private mapping, human result, or launch authorization.")

	fmt.Fprintln(&builder, "\n## Exact reviewer workload")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "| Work item | Exact requirement |")
	fmt.Fprintln(&builder, "|---|---:|")
	for _, row := range pilotLaunchWorkloadRows(dossier.ReviewerWorkload) {
		fmt.Fprintf(&builder, "| %s | %d |\n", row.Label, row.Value)
	}
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "The 64-action figure is the protocol maximum, not a measured duration or burden estimate. Tie-break work occurs only for prespecified disagreements.")

	fmt.Fprintln(&builder, "\n## Structural packet examples")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "| Controlled family | Review unit | Evidence slots | Observable audit question |")
	fmt.Fprintln(&builder, "|---|---|---:|---|")
	for _, disclosure := range disclosures {
		question, err := pilotLaunchBriefQuestion(disclosure.Family)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&builder, "| `%s` | `%s` | %d | %s |\n", disclosure.Family, disclosure.Unit, disclosure.EvidenceSlots, question)
	}
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "These are protocol-shape examples, not reviewer packets. They disclose no randomized side, packet order, source, task requirement, or evidence content.")

	fmt.Fprintln(&builder, "\n## Privacy and license boundary")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "- All eight pilot packets are non-public and restricted reference-only.")
	fmt.Fprintln(&builder, "- Zero pilot packets are approved for public redistribution.")
	fmt.Fprintln(&builder, "- Reviewer-visible material may be shared only after owner inspection, consent terms, recipient authorization, and packet-sharing authorization are separately complete.")
	fmt.Fprintln(&builder, "- Raw task requirements, evidence, source identities, workstation paths, packet identities, cryptographic commitments, and private mappings are excluded from this brief.")
	fmt.Fprintln(&builder, "- Public reporting may use aggregate protocol facts and consented aggregate results only; it must not reconstruct or quote restricted source material.")

	fmt.Fprintln(&builder, "\n## Proposed governance defaults")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "Every row is a non-binding proposal. Status remains `owner_decision_required` until the owner records an explicit decision.")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "| Decision | Current status | Proposed default | Required before |")
	fmt.Fprintln(&builder, "|---|---|---|---|")
	for _, decision := range dossier.GovernanceDecisions {
		proposal, err := pilotLaunchGovernanceProposal(decision.ID)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&builder, "| `%s` | `%s` | %s | `%s` |\n", decision.ID, decision.Status, proposal, decision.RequiredBefore)
	}

	fmt.Fprintln(&builder, "\n## External-action authorization")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "| Action | Current status |")
	fmt.Fprintln(&builder, "|---|---|")
	for _, action := range dossier.ExternalActions {
		fmt.Fprintf(&builder, "| `%s` | `%s` |\n", action.Action, action.Status)
	}
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "Authorization is action-specific. Approval of one row never authorizes another row, and technical readiness never changes any row automatically.")

	fmt.Fprintln(&builder, "\n## Scientific claim boundary")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "This brief establishes only a deterministic, structurally validated prelaunch decision surface with exact protocol workload and explicit custody boundaries.")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "It does not establish:")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "- completed owner semantic inspection;")
	fmt.Fprintln(&builder, "- reviewer qualification, independence, consent, agreement, timing, or burden;")
	fmt.Fprintln(&builder, "- human support for any controlled relation or construct-validity conclusion;")
	fmt.Fprintln(&builder, "- ethics, recruitment, contact, compensation, packet-sharing, publication, or scheduling approval;")
	fmt.Fprintln(&builder, "- verifier robustness, model quality, provider capability, or paper-result reproduction.")

	fmt.Fprintln(&builder, "\n## Owner authorization gate")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "- [ ] Complete and seal the eight-packet owner semantic inspection with no unresolved or revision-required packet.")
	fmt.Fprintln(&builder, "- [ ] Record an explicit decision for every governance row above.")
	fmt.Fprintln(&builder, "- [ ] Authorize each intended external action separately; leave every unused action `not_authorized`.")
	fmt.Fprintln(&builder, "- [ ] Rebuild and independently verify the immutable package after any governed packet, rubric, translation, or policy change.")
	fmt.Fprintln(&builder, "- [ ] Preserve `human_study_status=not_run` until real qualified reviewers complete the governed pilot.")
	return builder.String(), nil
}

type pilotLaunchWorkloadRow struct {
	Label string
	Value int
}

func pilotLaunchWorkloadRows(workload PilotReviewerWorkload) []pilotLaunchWorkloadRow {
	return []pilotLaunchWorkloadRow{
		{Label: "Reviewer slots", Value: workload.RequiredReviewerSlots},
		{Label: "Primary reviewer slots", Value: workload.PrimaryReviewerSlots},
		{Label: "Disagreement-only tie-break slots", Value: workload.TieBreakReviewerSlots},
		{Label: "Qualification cases per reviewer", Value: workload.QualificationCasesPerReviewer},
		{Label: "Qualification responses", Value: workload.RequiredQualificationResponses},
		{Label: "Primary judgments", Value: workload.RequiredPrimaryJudgments},
		{Label: "Maximum tie-break judgments", Value: workload.MaximumTieBreakJudgments},
		{Label: "Post-label condition probes", Value: workload.RequiredPostLabelProbes},
		{Label: "Maximum total review actions", Value: workload.MaximumTotalReviewActions},
	}
}

func pilotLaunchBriefQuestion(family mutation.Family) (string, error) {
	switch family {
	case mutation.FamilyCandidateOrderReversal:
		return "Does reversing the same candidate pair preserve the substantive comparison?", nil
	case mutation.FamilyCausalIndependentReorder:
		return "Does reordering causally independent events preserve task meaning and evidence?", nil
	case mutation.FamilyTestEvidenceFalsified:
		return "Does independently supported execution evidence dominate a falsified visible claim?", nil
	case mutation.FamilyToolOutputIncomplete:
		return "Does truncation weaken evidence without changing underlying task quality?", nil
	case mutation.FamilyIrrelevantVerbosity:
		return "Does irrelevant added text leave the substantive task judgment unchanged?", nil
	case mutation.FamilyNeutralFormatting:
		return "Does neutral formatting leave all judgment-relevant content unchanged?", nil
	case mutation.FamilyTestEvidenceOmitted:
		return "Does removing test evidence weaken support without changing underlying task quality?", nil
	case mutation.FamilyUntrustedScoreInjection:
		return "Does an untrusted score tag have zero normative authority over the task judgment?", nil
	default:
		return "", errors.New("relation pilot launch brief encountered an unsupported family")
	}
}

func pilotLaunchGovernanceProposal(decisionID string) (string, error) {
	switch decisionID {
	case "authorship_and_labor_credit":
		return "Guarantee labor acknowledgement and a contribution record; grant authorship only for qualifying intellectual contributions under a rule approved before publication, never as a recruitment inducement.", nil
	case "compensation_or_volunteer_terms":
		return "Use owner-approved flat compensation based on measured pilot burden, never outcomes or agreement; any volunteer path requires explicit opt-in with no penalty. Amount and payment method remain owner decisions.", nil
	case "consent_and_withdrawal_terms":
		return "Obtain informed consent before packet sharing; state withdrawal, aggregate-publication, and post-lock limits in advance; retain no unapproved secondary-use permission.", nil
	case "contact_and_scheduling_method":
		return "Use one owner-managed private channel, keep direct identifiers outside study artifacts, and schedule only after recruitment authorization.", nil
	case "human_data_retention_and_deletion":
		return "Separate contact data from pseudonymous study records, restrict access, and approve exact retention, deletion, backup, and incident-response dates before sharing packets.", nil
	case "reviewer_population_and_independence":
		return "Recruit three technically capable, conflict-screened reviewers with no prior packet exposure; require two independent primaries and a distinct disagreement-only tie-break reviewer.", nil
	default:
		return "", errors.New("relation pilot launch brief encountered an unsupported governance decision")
	}
}
