package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type fidelitySourceReport struct {
	SourceID string                    `json:"source_id"`
	Report   preprocess.FidelityReport `json:"report"`
}

func runFidelity(args []string) int {
	fs := flag.NewFlagSet("fidelity", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	output := fs.String("output", "json", "json/text")
	mode := fs.String("mode", string(preprocess.IngestStrict), "strict/compatibility")
	budgetsFlag := fs.String("budgets", "16384,32768,65536", "comma-separated positive token budgets")
	var sources stringSlice
	fs.Var(&sources, "source", "trajectory file (repeat for multiple)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "fidelity:", err)
		return 2
	}
	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, "fidelity: at least one --source is required")
		return 2
	}
	stdinSources := 0
	for _, source := range sources {
		if source == "-" {
			stdinSources++
		}
	}
	if stdinSources > 1 {
		fmt.Fprintln(os.Stderr, "fidelity: stdin source '-' may appear only once")
		return 2
	}
	budgets, err := parsePositiveIntegers(*budgetsFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fidelity budgets:", err)
		return 2
	}
	options := preprocess.DefaultIngestOptions()
	options.Mode = preprocess.IngestMode(*mode)
	if options.Mode != preprocess.IngestStrict && options.Mode != preprocess.IngestCompatibility {
		fmt.Fprintln(os.Stderr, "fidelity: --mode must be strict or compatibility")
		return 2
	}
	reports := make([]fidelitySourceReport, 0, len(sources))
	for index, source := range sources {
		report, err := auditFidelitySource(source, budgets, options)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fidelity source %d: %v\n", index+1, err)
			return 1
		}
		reports = append(reports, fidelitySourceReport{SourceID: fmt.Sprintf("source-%03d", index+1), Report: report})
	}
	switch *output {
	case "json":
		encoded, err := json.MarshalIndent(reports, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "fidelity output:", err)
			return 1
		}
		fmt.Println(string(encoded))
	case "text":
		printFidelityText(reports)
	default:
		fmt.Fprintln(os.Stderr, "fidelity: --output must be json or text")
		return 2
	}
	return 0
}

func auditFidelitySource(path string, budgets []int, options preprocess.IngestOptions) (report preprocess.FidelityReport, returnErr error) {
	if path == "-" {
		trajectory, err := preprocess.IngestReader(os.Stdin, options)
		if err != nil {
			return preprocess.FidelityReport{}, err
		}
		return preprocess.AuditFidelity(trajectory, budgets)
	}
	file, err := os.Open(path)
	if err != nil {
		return preprocess.FidelityReport{}, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close %s: %w", filepath.Base(path), closeErr)
		}
	}()
	trajectory, err := preprocess.IngestReader(file, options)
	if err != nil {
		return preprocess.FidelityReport{}, err
	}
	return preprocess.AuditFidelity(trajectory, budgets)
}

func parsePositiveIntegers(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		parsed, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("invalid positive integer %q", part)
		}
		values = append(values, parsed)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one budget is required")
	}
	return values, nil
}

func printFidelityText(reports []fidelitySourceReport) {
	for _, source := range reports {
		report := source.Report
		fmt.Printf("%s format=%s records=%d events=%d estimated_tokens=%d\n",
			source.SourceID, report.SourceFormat, report.Ingestion.SourceRecords,
			report.Ingestion.CanonicalEvents, report.EstimatedTokens)
		if report.TokenComparison != nil {
			fmt.Printf("  provider_usage observations=%d reported_total=%d relative_difference=%.6f\n",
				report.TokenComparison.ObservationCount, report.TokenComparison.ProviderReportedTotal,
				report.TokenComparison.RelativeDifference)
		}
		for _, budget := range report.Budgets {
			fmt.Printf("  budget=%d retained_tokens=%d retained_events=%d truncated=%t\n",
				budget.BudgetTokens, budget.RetainedTokens, budget.RetainedEvents, budget.Truncation.Applied)
		}
	}
}
