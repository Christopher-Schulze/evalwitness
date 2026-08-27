package stress

import (
	"fmt"
	"strings"
)

func RenderHeldOutRunReadinessRefusalMarkdown(value HeldOutRunReadinessRefusal) (string, error) {
	if err := value.Validate(); err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString("# Held-out run readiness refusal\n\n")
	builder.WriteString("> Provider-free preflight evidence. This receipt does not authorize execution and is not a held-out result.\n\n")
	fmt.Fprintf(&builder, "- Status: `%s`\n", value.Status)
	fmt.Fprintf(&builder, "- Run authorized: `%t`\n", value.RunAuthorized)
	fmt.Fprintf(&builder, "- Execution permit issued: `%t`\n", value.ExecutionPermitIssued)
	fmt.Fprintf(&builder, "- Provider calls: `%d`\n", value.ProviderCalls)
	fmt.Fprintf(&builder, "- Empirical units: `%d`\n", value.EmpiricalUnits)
	fmt.Fprintf(&builder, "- Partition: `%d` cases, `%d` cells, `%d` supported, `%d` structural unsupported\n", value.TestCases, value.TestCells, value.SupportedTestCells, value.UnsupportedTestCells)
	fmt.Fprintf(&builder, "- Receipt digest: `%s`\n\n", value.Digest)
	builder.WriteString("## Gate ledger\n\n")
	builder.WriteString("| Gate | Status | Evidence | Reason |\n")
	builder.WriteString("|---|---|---|---|\n")
	for _, gate := range value.Gates {
		evidence := "unavailable"
		if gate.EvidenceDigest != "" {
			evidence = "`" + gate.EvidenceDigest + "`"
		}
		fmt.Fprintf(&builder, "| `%s` | `%s` | %s | `%s` |\n", gate.ID, gate.Status, evidence, gate.Reason)
	}
	builder.WriteString("\n## Claim boundary\n\n")
	fmt.Fprintf(&builder, "Supported: %s.\n\n", value.ClaimBoundary.SupportedClaim)
	builder.WriteString("Unsupported:\n\n")
	for _, claim := range value.ClaimBoundary.UnsupportedClaims {
		fmt.Fprintf(&builder, "- %s\n", claim)
	}
	return builder.String(), nil
}
