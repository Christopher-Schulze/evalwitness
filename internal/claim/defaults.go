package claim

import (
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	componentBuild       = "provenance.build"
	componentClocks      = "provenance.clocks"
	componentLegacyFacts = "legacy.claim-facts"
	componentProtocolRun = "protocol.reference-audit-run"
	componentRoute       = "provenance.route"
	componentSourceTree  = "provenance.source-tree"
	componentStudy       = "provenance.study"
	componentAbsolute    = "legacy.result.terminal-bench-absolute"
	componentSWEVerifier = "legacy.result.swebench-verifier"
	componentSWESize     = "legacy.result.swebench-size-agnostic"
	componentTerminal    = "legacy.result.terminal-bench-verifier"
	componentTerminal16K = "legacy.result.terminal-bench-evidence-16k"
)

func DefaultLedger(manifest capsule.Manifest) (Ledger, error) {
	claims, err := DefaultClaims()
	if err != nil {
		return Ledger{}, err
	}
	return SealLedger(manifest, claims)
}

func DefaultClaims() ([]Claim, error) {
	claims := []Claim{
		defaultClaim("CLM-001", "The public reference capsule binds a Git-visible source-tree snapshot with the declared source-tree algorithm", StatusSupported, EvidenceE1, softwareScope("TASK-047", "TASK-050"), pointer(componentSourceTree, capsule.SourceTreeProvenanceSchemaVersion, "/algorithm", StringValue(capsule.SourceTreeAlgorithm)), "Source binding proves artifact identity, not production safety or interface correctness", "source-interface-v1", AttestationNotRequired),
		defaultClaim("CLM-002", "The sealed protocol audit records an offline reference-evaluator run over the bound conformance corpus", StatusSupported, EvidenceE1, softwareScope("TASK-044", "TASK-053"), pointer(componentProtocolRun, protocolkit.RunSchema, "/offline", BooleanValue(true)), "Protocol conformance does not establish empirical evaluator reliability", "score-evidence-v1", AttestationNotRequired),
		defaultClaim("CLM-003", "The capsule digest-binds an external binary to the recorded source snapshot and build metadata", StatusSupported, EvidenceE1, softwareScope("TASK-042", "TASK-047", "TASK-050"), pointer(componentBuild, capsule.BuildProvenanceSchemaVersion, "/reproducibility", StringValue("digest-bound-external-binary")), "Digest binding does not prove a reproducible build or safety for untrusted use", "build-provenance-v1", AttestationNotRequired),
		defaultClaim("CLM-004", "The project is provider-agnostic or works with any OpenAI-compatible endpoint", StatusUnsupported, EvidenceE0, routeScope(), pointer(componentRoute, capsule.RouteProvenanceSchemaVersion, "/provider_calls_status", StringValue("not_run")), "No universal provider claim is permitted; only explicitly attested routes may be named", "route-reference-v1", AttestationUnavailable),
		defaultClaim("CLM-005", "Nothing else ships this combination of reliability instrumentation", StatusUnsupported, EvidenceE0, governanceScope("TASK-037"), pointer(componentStudy, capsule.StudyProvenanceSchemaVersion, "/confirmation_status", StringValue("not_admitted")), "Novelty remains unconfirmed until the closest-work and release audit is rerun", "positioning-unconfirmed-v1", AttestationNotRequired),
		defaultClaim("CLM-006", "The four frozen request fixtures bind the configured route and preset metadata used by the reference package", StatusSupported, EvidenceE1, routeScope(), count(componentRoute, capsule.RouteProvenanceSchemaVersion, "/requests", "4"), "This is configuration and request-fixture evidence, not live capability evidence", "route-reference-v1", AttestationNotRequired),
		defaultClaim("CLM-007", "The default route is currently full-verifier eligible", StatusUnsupported, EvidenceE2, routeScope(), pointer(componentRoute, capsule.RouteProvenanceSchemaVersion, "/attestation_status", StringValue("unavailable_public_source")), "The 2026-08-06/07 route evidence is stale for a current-capability claim and must be re-attested", "route-attestation-2026-08-07", AttestationStale),
		defaultClaim("CLM-008", "The sealed offline protocol audit contains exactly 188 bound conformance-case results", StatusSupported, EvidenceE1, softwareScope("TASK-053"), count(componentProtocolRun, protocolkit.RunSchema, "/results", "188"), "Conformance-case coverage is not an empirical reproduction of the paper", "protocol-reference-v1", AttestationNotRequired),
		defaultClaim("CLM-009", "The two primary legacy verifier artifacts bind a total of 589 benchmark tasks", StatusSupported, EvidenceE1, legacyScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/benchmark_tasks_total", NumberValue("589")), "Dataset-only legacy scope; incomplete run provenance caps this claim at E1", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-010", "The two primary legacy verifier artifacts record zero unextracted scores", StatusExploratory, EvidenceE1, legacyScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/unextracted_scores_total", NumberValue("0")), "Quote only with exact artifact, route, date, decidable count, and legacy-provenance caveat", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-011", "The recorded Terminal-Bench verifier-to-random score ratio is exactly 15/14", StatusExploratory, EvidenceE1, terminalScope(), ratio(componentTerminal, capsule.LegacyDerivedResultSchemaVersion, "/verifier_score", componentTerminal, capsule.LegacyDerivedResultSchemaVersion, "/pass_at_1_score", "15/14"), "Terminal-Bench development artifact only; no pooled or method-general statement", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-012", "The two primary legacy verifier artifacts record zero pair-decision ties", StatusSupported, EvidenceE1, legacyScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/pair_ties_total", NumberValue("0")), "The 240-decision denominator is governed separately by CLM-017", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-013", "Reading the distribution improves selection accuracy over reading the emitted letter", StatusUnsupported, EvidenceE1, legacyScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/pair_decisions_total", NumberValue("240")), "Current paired tests do not support superiority", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-014", "Verifier and judge arms are exact response-identical counterfactuals", StatusUnsupported, EvidenceE1, routeScope(), pointer(componentRoute, capsule.RouteProvenanceSchemaVersion, "/response_status", StringValue("unavailable_public_source")), "Exact response identity requires bound response bytes and TASK-043 fingerprints, which the public reference lacks", "route-reference-v1", AttestationUnavailable),
		defaultClaim("CLM-015", "Raw win_probability is calibrated confidence or a valid correctness threshold", StatusUnsupported, EvidenceE1, legacyScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/calibration_monotone", BooleanValue(false)), "Development calibration curves invalidate a calibrated-confidence interpretation", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-016", "The recorded Terminal-Bench development AUC is exactly 0.828125 under the committed artifact", StatusSupported, EvidenceE1, terminalScope(), pointer(componentTerminal, capsule.LegacyDerivedResultSchemaVersion, "/calibration/auc", NumberValue("53/64")), "Development-only and task-clustered; other metrics remain separate ledger expressions", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-017", "The two primary legacy verifier artifacts contain exactly 240 pair decisions", StatusSupported, EvidenceE1, legacyScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/pair_decisions_total", NumberValue("240")), "Exact committed artifacts only; no population denominator is implied", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-018", "The committed Terminal-Bench absolute artifact declares the absolute selection strategy", StatusSupported, EvidenceE1, terminalScope(), pointer(componentAbsolute, capsule.LegacyDerivedResultSchemaVersion, "/plan/strategy", StringValue("absolute")), "Association and quality conclusions remain separate and inherit legacy provenance limits", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-019", "The most-patch-hunks baseline descriptively solves seven more decidable SWE-bench tasks than the verifier in the committed artifact", StatusSupported, EvidenceE1, sweScope(), difference(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/swe_hunk_baseline_decidable_solved", componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/swe_verifier_decidable_solved", "7"), "Say outscores in this artifact; do not claim causal superiority", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-020", "The hunk-count heuristic is generally better than the verifier", StatusUnsupported, EvidenceE1, sweScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/swe_hunk_baseline_decidable_solved", NumberValue("56")), "The paired comparison and external validity do not support a general claim", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-021", "Removing size clauses caused the six-task SWE-bench score increase", StatusUnsupported, EvidenceE1, sweScope(), difference(componentSWESize, capsule.LegacyDerivedResultSchemaVersion, "/verifier_score", componentSWEVerifier, capsule.LegacyDerivedResultSchemaVersion, "/verifier_score", "6"), "The six-task crossing is descriptive; causality is unsupported", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-022", "The committed verifier-to-paper-parity call-count ratio is exactly 101/1712", StatusSupported, EvidenceE1, legacyScope(), ratio(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/verifier_calls_total", componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/paper_parity_calls_total", "101/1712"), "Descriptive development comparison only; no empirical equivalence claim", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-023", "A third pair call buys nothing, or two calls are universally sufficient", StatusUnsupported, EvidenceE1, legacyScope(), pointer(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/three_call_pairs_total", NumberValue("0")), "Zero recorded third calls is a bounded sweep observation, not universal sufficiency", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-024", "A 16k evidence budget is harmless or optimal", StatusUnsupported, EvidenceE1, terminalScope(), ratio(componentTerminal16K, capsule.LegacyDerivedResultSchemaVersion, "/verifier_score", componentTerminal, capsule.LegacyDerivedResultSchemaVersion, "/verifier_score", "25/26"), "The available sample cannot exclude harm or establish an optimum", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-025", "The committed Terminal-Bench absolute-to-default call-count ratio is exactly 85/97", StatusSupported, EvidenceE1, terminalScope(), ratio(componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/terminal_absolute_calls", componentLegacyFacts, capsule.LegacyClaimFactsSchemaVersion, "/terminal_verifier_calls", "85/97"), "No universal efficiency or quality claim; the score difference remains separately guarded", "legacy-development-v1", AttestationNotRequired),
		defaultClaim("CLM-026", "Passing a short capability probe qualifies a route", StatusSuperseded, EvidenceE1, routeScope(), pointer(componentRoute, capsule.RouteProvenanceSchemaVersion, "/provider_calls_status", StringValue("not_run")), "Short probes are diagnostics only and cannot qualify a route", "short-probe-v1", AttestationUnavailable),
		defaultClaim("CLM-027", "Three surveyed routes passed short probes and failed production-shaped score extraction", StatusUnsupported, EvidenceE1, governanceScope("TASK-045"), pointer(componentRoute, capsule.RouteProvenanceSchemaVersion, "/attestation_status", StringValue("unavailable_public_source")), "The public capsule lacks the historical response and attestation bytes needed to assert this survey result", "historical-route-survey-v1", AttestationUnavailable),
		defaultClaim("CLM-028", "Dry-run, replay, or a config catalog proves live provider capability", StatusUnsupported, EvidenceE1, routeScope(), pointer(componentRoute, capsule.RouteProvenanceSchemaVersion, "/provider_calls_status", StringValue("not_run")), "Only a bounded live E2 attestation can establish route capability", "route-reference-v1", AttestationUnavailable),
		defaultClaim("CLM-029", "Current filesystem, archive, transcript, logging, and release behavior is safe for untrusted public use", StatusUnsupported, EvidenceE0, softwareScope("TASK-042", "TASK-050", "TASK-052"), pointer(componentBuild, capsule.BuildProvenanceSchemaVersion, "/binary_included", BooleanValue(false)), "The reference proves bounded artifact handling, not blanket safety for untrusted public use", "build-provenance-v1", AttestationNotRequired),
		defaultClaim("CLM-030", "Warm-cache regeneration is a complete scientific reproduction", StatusUnsupported, EvidenceE1, governanceScope("TASK-040", "TASK-050"), pointer(componentClocks, capsule.ClockProvenanceSchemaVersion, "/events/1/status", StringValue("not_run")), "Replay and collection clocks are separate; clean-clone reproduction remains required", "clock-separation-v1", AttestationNotRequired),
		defaultClaim("CLM-031", "The repository reproduces the paper's headline empirical results", StatusUnsupported, EvidenceE1, governanceScope("TASK-040", "TASK-063"), pointer(componentStudy, capsule.StudyProvenanceSchemaVersion, "/provider_study_status", StringValue("not_run")), "Reference conformance and dataset tallies are not headline empirical reproduction", "study-boundary-v1", AttestationNotRequired),
		defaultClaim("CLM-032", "Findings transfer across providers, gateways, model families, checkpoints, or time", StatusUnsupported, EvidenceE1, governanceScope("TASK-036", "TASK-051"), count(componentRoute, capsule.RouteProvenanceSchemaVersion, "/served_identities", "0"), "Transfer requires the prespecified multi-provider and longitudinal studies", "route-reference-v1", AttestationUnavailable),
		defaultClaim("CLM-033", "Evidence-reliance effects establish agent-step causality or model-internal explanation", StatusUnsupported, EvidenceE0, governanceScope("TASK-065"), pointer(componentStudy, capsule.StudyProvenanceSchemaVersion, "/human_study_status", StringValue("not_run")), "Agent-step causality and model-internal explanation are permanently outside claim scope", "study-boundary-v1", AttestationNotRequired),
		defaultClaim("CLM-034", "Downloadable releases currently exist at the README release URL", StatusUnsupported, EvidenceE0, governanceScope("TASK-052"), pointer(componentStudy, capsule.StudyProvenanceSchemaVersion, "/external_action_status", StringValue("not_authorized")), "The local repository has no publication proof; release publication requires explicit authorization", "study-boundary-v1", AttestationNotRequired),
	}
	claims[6].History = []HistoryEntry{{Revision: 1, PriorStatus: StatusExploratory, Reason: "Current-capability wording expired when the time-bound public attestation became unavailable", SupersededBy: ""}}
	claims[13].History = []HistoryEntry{{Revision: 1, PriorStatus: StatusExploratory, Reason: "Exact counterfactual identity is unsupported without bound response bytes", SupersededBy: ""}}
	claims[25].History = []HistoryEntry{{Revision: 1, PriorStatus: StatusSupported, Reason: "Bounded real score-task attestation replaced short-probe qualification", SupersededBy: "CLM-007"}}
	claims[25].Generation.Current = "route-reference-v1"
	for index := range claims {
		normalizeClaim(&claims[index])
		if err := claims[index].Validate(); err != nil {
			return nil, err
		}
	}
	return claims, nil
}

func defaultClaim(id, text string, status Status, level EvidenceLevel, scope Scope, expression Expression, caveat, generation string, attestation AttestationState) Claim {
	components := make([]string, 0, len(expression.Operands))
	parentTypes := make([]string, 0, len(expression.Operands))
	for _, operand := range expression.Operands {
		if !slices.Contains(components, operand.Component) {
			components = append(components, operand.Component)
		}
		if !slices.Contains(parentTypes, operand.TypeID) {
			parentTypes = append(parentTypes, operand.TypeID)
		}
	}
	claim := Claim{
		ClaimID: id, TextTemplate: text, Status: status, EvidenceLevel: level, Scope: scope, Expression: expression,
		Uncertainty:  Uncertainty{Status: uncertaintyStatus(expression, generation), Method: "exact committed artifact field or guarding test", ConfidenceLevel: "", ClusterUnit: "declared claim scope"},
		Multiplicity: Multiplicity{Status: "descriptive_only", Family: "claim-ledger-v1", Method: "no multiplicity promotion"},
		CapsuleIDs:   []string{strings.Repeat("0", 64)}, EvidenceComponents: components, Caveats: []string{caveat},
		EvidenceCeiling: EvidenceCeiling{MaximumLevel: level, StrongestStatement: text, RequiredParentTypes: parentTypes, RequiredCaveats: []string{caveat}},
		Attestation:     attestation, Generation: Generation{Lane: "public-claims", Evidence: generation, Current: generation},
		GuardingTest: "scripts/tests/run-claimcheck.sh#" + id, History: []HistoryEntry{},
	}
	claim.Challenges = defaultChallenges(claim)
	return claim
}

func defaultChallenges(item Claim) []ChallengeSpec {
	challenges := make([]ChallengeSpec, 0, len(ChallengeClasses()))
	for _, class := range ChallengeClasses() {
		specification := ChallengeSpec{ChallengeID: strings.ToLower(item.ClaimID) + "." + string(class), Class: class}
		applicable, reason := challengeApplicability(item, class)
		if applicable {
			specification.Applicability = ChallengeApplied
			specification.Mutation = challengeMutation(class)
			specification.ExpectedGuard = expectedGuard(class)
		} else {
			specification.Applicability = ChallengeInapplicable
			specification.InapplicableReason = reason
		}
		challenges = append(challenges, specification)
	}
	return challenges
}

func challengeMutation(class ChallengeClass) string {
	switch class {
	case ChallengeCaveatRemoval:
		return "remove the first required caveat from the ephemeral claim state"
	case ChallengeDenominatorDeletion:
		return "remove the declared ratio denominator from the ephemeral expression state"
	case ChallengeDigestSubstitution:
		return "flip one byte in the first evidence payload of an ephemeral capsule copy"
	case ChallengeParentRemoval:
		return "remove the first declared evidence component from the ephemeral availability set"
	case ChallengeScopeWidening:
		return "add an undeclared scope bound to the ephemeral verification request"
	case ChallengeStaleAttestation:
		return "replace the ephemeral attestation state with stale"
	case ChallengeSupersededGeneration:
		return "replace the ephemeral evidence generation with a superseded generation"
	case ChallengeVisibilityLeak:
		return "expose a non-public evidence projection in the ephemeral claim state"
	default:
		return ""
	}
}

func uncertaintyStatus(expression Expression, generation string) string {
	if strings.HasPrefix(generation, "legacy-") {
		return "unavailable_legacy"
	}
	if expression.Expected.Kind == ValueNumber {
		return "descriptive_only"
	}
	return "not_applicable"
}

func exists(component, typeID string) Expression {
	return Expression{Operation: OperationComponentExists, Operands: []Operand{{Component: component, TypeID: typeID, Pointer: ""}}, Expected: BooleanValue(true)}
}

func pointer(component, typeID, path string, expected ExactValue) Expression {
	return Expression{Operation: OperationPointerEquals, Operands: []Operand{{Component: component, TypeID: typeID, Pointer: path}}, Expected: expected}
}

func count(component, typeID, path, expected string) Expression {
	return Expression{Operation: OperationCount, Operands: []Operand{{Component: component, TypeID: typeID, Pointer: path}}, Expected: NumberValue(expected)}
}

func ratio(leftComponent, leftType, leftPath, rightComponent, rightType, rightPath, expected string) Expression {
	return Expression{Operation: OperationRatio, Operands: []Operand{
		{Component: leftComponent, TypeID: leftType, Pointer: leftPath},
		{Component: rightComponent, TypeID: rightType, Pointer: rightPath},
	}, Expected: NumberValue(expected)}
}

func difference(leftComponent, leftType, leftPath, rightComponent, rightType, rightPath, expected string) Expression {
	return Expression{Operation: OperationDifference, Operands: []Operand{
		{Component: leftComponent, TypeID: leftType, Pointer: leftPath},
		{Component: rightComponent, TypeID: rightType, Pointer: rightPath},
	}, Expected: NumberValue(expected)}
}

func softwareScope(tasks ...string) Scope {
	return closedScope([]string{}, []string{}, []string{"software-implementation"}, tasks, []string{"tested-interface-only"}, []string{}, []string{"cli", "mcp"})
}

func routeScope() Scope {
	return closedScope([]string{"opencode-go-cn"}, []string{"deepseek-v4-flash"}, []string{"route-conformance"}, []string{"TASK-043", "TASK-045", "TASK-050"}, []string{"bounded-attestation-required"}, []string{"2026-08-06/2026-08-07-historical"}, []string{"cli"})
}

func legacyScope() Scope {
	return closedScope([]string{"opencode-go-cn"}, []string{"deepseek-v4-flash"}, []string{"swebench", "terminal-bench"}, []string{"TASK-041", "TASK-050"}, []string{"development-only", "legacy-provenance"}, []string{"2026-08-07"}, []string{"eval"})
}

func terminalScope() Scope {
	return closedScope([]string{"opencode-go-cn"}, []string{"deepseek-v4-flash"}, []string{"terminal-bench"}, []string{"TASK-041", "TASK-050"}, []string{"development-only", "legacy-provenance"}, []string{"2026-08-07"}, []string{"eval"})
}

func sweScope() Scope {
	return closedScope([]string{"opencode-go-cn"}, []string{"deepseek-v4-flash"}, []string{"swebench"}, []string{"TASK-041", "TASK-050"}, []string{"development-only", "legacy-provenance"}, []string{"2026-08-07"}, []string{"eval"})
}

func governanceScope(tasks ...string) Scope {
	return closedScope([]string{}, []string{}, []string{"research-governance"}, tasks, []string{"claim-ceiling-enforced"}, []string{}, []string{"documentation", "release"})
}

func closedScope(routes, models, domains, tasks, policies, timeBounds, entrypoints []string) Scope {
	scope := Scope{
		Routes: slices.Clone(routes), Models: slices.Clone(models), Domains: slices.Clone(domains),
		Tasks: slices.Clone(tasks), Policies: slices.Clone(policies), TimeBounds: slices.Clone(timeBounds),
		Entrypoints: slices.Clone(entrypoints),
	}
	normalizeScope(&scope)
	return scope
}
