package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/reliance"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func runDesign(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "design: usage: evalwitness design <simulate|reliance-preflight|identical-response>")
		return 2
	}
	if args[0] == "reliance-preflight" {
		return runReliancePreflightDesign(args[1:])
	}
	if args[0] == "identical-response" {
		return runIdenticalResponseDesign(args[1:])
	}
	if args[0] != "simulate" {
		fmt.Fprintln(os.Stderr, "design: usage: evalwitness design <simulate|reliance-preflight|identical-response>")
		return 2
	}
	fs := flag.NewFlagSet("design simulate", flag.ContinueOnError)
	specPath := fs.String("spec", "", "clustered factorial simulation spec (@file or - for stdin)")
	output := fs.String("output", "json", "json/text")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "design simulate:", err)
		return 2
	}
	if strings.TrimSpace(*specPath) == "" {
		fmt.Fprintln(os.Stderr, "design simulate: --spec is required")
		return 2
	}
	reader, closeReader, err := openDesignSpec(*specPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "design simulate:", err)
		return 2
	}
	defer closeReader()
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec stats.ClusterSimulationSpec
	if err := decoder.Decode(&spec); err != nil {
		fmt.Fprintln(os.Stderr, "design simulate: decode spec:", err)
		return 2
	}
	if err := requireJSONEOF(decoder); err != nil {
		fmt.Fprintln(os.Stderr, "design simulate:", err)
		return 2
	}
	report, err := stats.SimulateClusteredFactorial(spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "design simulate:", err)
		return 2
	}
	switch *output {
	case "json":
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "design simulate: encode report:", err)
			return 1
		}
		fmt.Println(string(encoded))
	case "text":
		printClusterSimulationReport(report)
	default:
		fmt.Fprintln(os.Stderr, "design simulate: --output must be json or text")
		return 2
	}
	return 0
}

func runIdenticalResponseDesign(args []string) int {
	fs := flag.NewFlagSet("design identical-response", flag.ContinueOnError)
	specPath := fs.String("spec", "", "identical-response design spec (@file or - for stdin)")
	output := fs.String("output", "json", "json/text")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "design identical-response:", err)
		return 2
	}
	if strings.TrimSpace(*specPath) == "" {
		fmt.Fprintln(os.Stderr, "design identical-response: --spec is required")
		return 2
	}
	reader, closeReader, err := openDesignSpec(*specPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "design identical-response:", err)
		return 2
	}
	defer closeReader()
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec stats.IdenticalResponseDesignSpec
	if err := decoder.Decode(&spec); err != nil {
		fmt.Fprintln(os.Stderr, "design identical-response: decode spec:", err)
		return 2
	}
	if err := requireJSONEOF(decoder); err != nil {
		fmt.Fprintln(os.Stderr, "design identical-response:", err)
		return 2
	}
	report, err := stats.ComputeIdenticalResponseDesign(spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "design identical-response:", err)
		return 2
	}
	switch *output {
	case "json":
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "design identical-response: encode report:", err)
			return 1
		}
		fmt.Println(string(encoded))
	case "text":
		printIdenticalResponseDesignReport(report)
	default:
		fmt.Fprintln(os.Stderr, "design identical-response: --output must be json or text")
		return 2
	}
	return 0
}

func runReliancePreflightDesign(args []string) int {
	fs := flag.NewFlagSet("design reliance-preflight", flag.ContinueOnError)
	codeDigest := fs.String("code-digest", "", "SHA-256 digest of the analysis binary or source bundle")
	output := fs.String("output", "json", "json/text")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "design reliance-preflight:", err)
		return 2
	}
	if strings.TrimSpace(*codeDigest) == "" {
		fmt.Fprintln(os.Stderr, "design reliance-preflight: --code-digest is required")
		return 2
	}
	ontology, err := reliance.FrozenOntology()
	if err != nil {
		fmt.Fprintln(os.Stderr, "design reliance-preflight:", err)
		return 1
	}
	estimands, err := reliance.FrozenEstimands()
	if err != nil {
		fmt.Fprintln(os.Stderr, "design reliance-preflight:", err)
		return 1
	}
	preregistration, err := reliance.FrozenPreregistration(ontology, estimands)
	if err != nil {
		fmt.Fprintln(os.Stderr, "design reliance-preflight:", err)
		return 1
	}
	preflight, err := reliance.BuildReliancePreflight(preregistration, *codeDigest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "design reliance-preflight:", err)
		return 2
	}
	switch *output {
	case "json":
		encoded, err := json.MarshalIndent(preflight, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "design reliance-preflight: encode report:", err)
			return 1
		}
		fmt.Println(string(encoded))
	case "text":
		printReliancePreflightReport(preflight)
	default:
		fmt.Fprintln(os.Stderr, "design reliance-preflight: --output must be json or text")
		return 2
	}
	return 0
}

func openDesignSpec(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	path = strings.TrimPrefix(path, "@")
	file, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return file, func() { _ = file.Close() }, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing input: %w", err)
	}
	return fmt.Errorf("simulation spec contains more than one JSON value")
}

func printIdenticalResponseDesignReport(report stats.IdenticalResponseDesignReport) {
	fmt.Printf("Identical-response paired design: counterfactual=%s source_task_groups=%d effective=%d combined_loss=%.2f seed-free\n",
		report.Counterfactual, report.SourceTaskGroups, report.EffectiveTaskGroups, report.CombinedLoss)
	fmt.Printf("Design alpha: nominal=%.4f adjusted=%.4f family=%d target_power=%.2f discordant_q=%.3f\n",
		report.Alpha, report.AdjustedAlpha, report.FamilySize, report.TargetPower, report.DiscordantWinProbability)
	for _, row := range report.Rows {
		fmt.Printf("  disagreement=%5.1f%% expected_discordant=%6.2f power_q=%.3f/%.3f separation=%.3f/%.3f MDE=%s/%s nominal/adjusted\n",
			row.DisagreementRate*100, row.ExpectedDiscordant, row.PowerAtDeclaredQNominal, row.PowerAtDeclaredQAdjusted,
			row.PowerAtSeparationNominal, row.PowerAtSeparationAdjusted, optionalFloat(row.MDENominal), optionalFloat(row.MDEAdjusted))
	}
	for _, row := range report.FailureSensitivity {
		fmt.Printf("  failure %s: effective=%d disagreement=%5.1f%% MDE=%s/%s nominal/adjusted\n",
			row.Scenario, row.EffectiveTaskGroups, row.DisagreementRate*100, optionalFloat(row.MDENominal), optionalFloat(row.MDEAdjusted))
	}
	fmt.Printf("Reproducibility: algorithm=%s design_digest=%s\n", report.Algorithm, report.DesignDigest)
}

func printClusterSimulationReport(report stats.ClusterSimulationReport) {
	fmt.Printf("Clustered factorial design: endpoint=%s source_tasks=%d effective=%.1f ICC=%.3f seed=%d\n",
		report.Endpoint, report.SourceTasks, report.EffectiveSourceTasks, report.IntraclusterCorrelation, report.Seed)
	fmt.Printf("Model: %s\n", report.DataGeneratingModel)
	fmt.Printf("Design rank: %d/%d", report.Aliasing.Rank, report.Aliasing.Parameters)
	if len(report.Aliasing.AliasedTerms) > 0 {
		fmt.Printf(" aliased=%s", strings.Join(report.Aliasing.AliasedTerms, ","))
	}
	fmt.Println()
	fmt.Printf("Budget: observations=%d calls=%d/%d input_tokens=%d/%d\n",
		report.Budget.PlannedObservations, report.Budget.RequiredCalls, report.Budget.HardCalls,
		report.Budget.RequiredInputTokens, report.Budget.HardInputTokens)
	fmt.Printf("Denominators: planned=%d observed=%d invalid=%d missing=%d abstained=%d route_failed=%d\n",
		report.Denominators.Planned, report.Denominators.Observed, report.Denominators.Invalid,
		report.Denominators.Missing, report.Denominators.Abstained, report.Denominators.RouteFailure)
	for _, result := range report.OperatingCharacteristics {
		fmt.Printf("  %s: effect=%+.4f (%s) estimate=%+.4f (%s) power=%.3f MCSE=%.4f valid=%d estimable=%t\n",
			result.Term, result.DeclaredEffect, result.DeclaredEffectScale, result.MeanEstimate,
			result.EstimateScale, result.Power, result.MonteCarloSE, result.ValidRuns, result.Estimable)
	}
	fmt.Printf("Reproducibility: algorithm=%s algorithm_digest=%s design_digest=%s code_digest=%s\n", report.Algorithm, report.AlgorithmDigest, report.DesignDigest, report.CodeDigest)
}

func printReliancePreflightReport(report reliance.ReliancePreflight) {
	fmt.Printf("Evidence-reliance preflight: status=%s selected_source_tasks=%d target_power=%.2f replications=%d\n",
		report.Status, report.SelectedSourceTasks, report.TargetPower, report.MonteCarloReplications)
	fmt.Printf("Walsh design: runs=%d factors=%d interactions=%d main_clear=%t interactions_unique=%t\n",
		report.AliasAudit.SelectedRuns, report.AliasAudit.Factors, report.AliasAudit.DeclaredInteractions,
		report.AliasAudit.MainEffectsClearOfTwoFactorTerms, report.AliasAudit.DeclaredInteractionsUnique)
	for _, candidate := range report.AliasAudit.Candidates {
		fmt.Printf("  alias candidate runs=%d status=%s audited=%d qualifying=%d reason=%s\n",
			candidate.Runs, candidate.Status, candidate.SumFreeSetsAudited,
			candidate.QualifyingInteractionLayouts, candidate.Reason)
	}
	for _, candidate := range report.Candidates {
		fmt.Printf("  power candidate source_tasks=%d resolved=%t calls=%d hard_attempts=%d\n",
			candidate.SourceTasks, candidate.Resolved, candidate.Budget.LogicalCalls, candidate.Budget.HardAttempts)
		for _, check := range candidate.Checks {
			fmt.Printf("    %s/%s effect=%+.4f estimate=%+.4f power=%.3f lower95=%.3f MCSE=%.4f\n",
				check.ScenarioID, check.TermID, check.DeclaredEffect, check.MeanEstimate,
				check.Power, check.PowerLower95, check.MonteCarloSE)
		}
	}
	for _, mde := range report.SelectedMDEs {
		if mde.MinimumDetectableEffect == nil {
			fmt.Printf("  MDE %s: unresolved on grid=%v\n", mde.ScenarioID, mde.EffectGrid)
			continue
		}
		fmt.Printf("  MDE %s: %.4f declared_scale=%s estimate_scale=%s grid=%v\n",
			mde.ScenarioID, *mde.MinimumDetectableEffect, mde.DeclaredEffectScale, mde.EstimateScale, mde.EffectGrid)
	}
	fmt.Printf("Selected hard budget: calls=%d attempts=%d input_tokens=%d output_tokens=%d duration_seconds=%d concurrency=%d cost_usd=%.4f\n",
		report.SelectedBudget.LogicalCalls, report.SelectedBudget.HardAttempts,
		report.SelectedBudget.HardInputTokens, report.SelectedBudget.HardOutputTokens,
		report.SelectedBudget.HardDurationSeconds, report.SelectedBudget.HardConcurrency,
		report.SelectedBudget.HardCostUSD)
	fmt.Printf("Boundary: live_authorized=%t empirical_assumptions=%t digest=%s\n",
		report.LiveAuthorized, report.EmpiricalAssumptions, report.Digest)
}
