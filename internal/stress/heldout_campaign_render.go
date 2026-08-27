package stress

import (
	"fmt"
	"strings"
)

func RenderHeldOutCampaignPlanMarkdown(value HeldOutCampaignPlan) (string, error) {
	if err := value.Validate(); err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString("# Held-out campaign plan\n\n")
	builder.WriteString("> Provider-free topology evidence. This plan fixes workload shape and execution class, not provider requests, replay payloads, budget, authorization, or execution.\n\n")
	fmt.Fprintf(&builder, "- Status: `%s`\n", value.Status)
	fmt.Fprintf(&builder, "- Partition: `%d` cases, `%d` cells, `%d` supported, `%d` structural unsupported\n", value.TestCases, value.TestCells, value.SupportedTestCells, value.StructuralUnsupportedTestCells)
	fmt.Fprintf(&builder, "- Provider-dependent workload: `%d` arms, `%d` supported cells, `%d` verification inputs, `%d` registered side repetitions\n", value.ProviderDependentArms, value.ProviderDependentSupportedTestCells, value.ProviderDependentVerificationInputs, value.PlannedProviderDependentSideRepetitions)
	fmt.Fprintf(&builder, "- Live-provider workload: `%d` arms, `%d` supported cells, `%d` verification inputs, `%d` registered side repetitions\n", value.LiveProviderArms, value.LiveProviderSupportedTestCells, value.LiveProviderVerificationInputs, value.PlannedLiveProviderSideRepetitions)
	fmt.Fprintf(&builder, "- Sealed-replay workload: `%d` arm, `%d` supported cells, `%d` verification inputs, `%d` registered side repetitions\n", value.SealedReplayArms, value.SealedReplaySupportedTestCells, value.SealedReplayVerificationInputs, value.PlannedSealedReplaySideRepetitions)
	fmt.Fprintf(&builder, "- Zero-cost workload: `%d` arms, `%d` supported cells, `%d` deterministic repetitions\n", value.ZeroCostArms, value.ZeroCostSupportedTestCells, value.PlannedZeroCostRepetitions)
	fmt.Fprintf(&builder, "- Fixed repetitions: `%d`\n", value.FixedRepetitions)
	fmt.Fprintf(&builder, "- Run authorized: `%t`\n", value.RunAuthorized)
	fmt.Fprintf(&builder, "- Execution permit issued: `%t`\n", value.ExecutionPermitIssued)
	fmt.Fprintf(&builder, "- Provider calls: `%d`\n", value.ProviderCalls)
	fmt.Fprintf(&builder, "- Empirical units: `%d`\n", value.EmpiricalUnits)
	fmt.Fprintf(&builder, "- Campaign digest: `%s`\n\n", value.Digest)
	builder.WriteString("## Arm topology\n\n")
	builder.WriteString("| Arm | Execution class | Provider-dependent | Test | Supported | Unsupported | Verification inputs | Registered repetitions |\n")
	builder.WriteString("|---|---|---:|---:|---:|---:|---:|---:|\n")
	for _, arm := range value.Arms {
		repetitions := arm.PlannedProviderSideRepetitions + arm.PlannedZeroCostRepetitions
		fmt.Fprintf(&builder, "| `%s` | `%s` | `%t` | %d | %d | %d | %d | %d |\n", arm.ArmID, arm.ExecutionClass, arm.ProviderDependent, arm.TestCells, arm.SupportedTestCells, arm.UnsupportedTestCells, arm.ProviderVerificationInputs, repetitions)
	}
	builder.WriteString("\n## Live-binding boundary\n\n")
	builder.WriteString("Every live-binding flag is `false`: StudyRecord, execution bindings, two live-provider request plans, the sealed-replay plan, provider call counts, budgets, current route attestations, authorization digests, and the private capsule family.\n\n")
	builder.WriteString("## Claim boundary\n\n")
	fmt.Fprintf(&builder, "Supported: %s.\n\n", value.ClaimBoundary.SupportedClaim)
	builder.WriteString("Unsupported:\n\n")
	for _, claim := range value.ClaimBoundary.UnsupportedClaims {
		fmt.Fprintf(&builder, "- %s\n", claim)
	}
	return builder.String(), nil
}
