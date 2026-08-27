package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/registry"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

func runRegistry(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "registry: missing subcommand (validate-intake|preflight|template|refresh|review-checklist|render-matrix|render-reliance|index-scarcity|index-owner-inspection|render-method-lineage|index-empirical|inventory|public-derivative)")
		return 2
	}
	switch args[0] {
	case "validate-intake":
		return runRegistryValidateIntake(args[1:])
	case "preflight":
		return runRegistryPreflight(args[1:])
	case "template":
		return runRegistryTemplate(args[1:])
	case "refresh":
		return runRegistryRefresh(args[1:])
	case "review-checklist":
		return runRegistryReviewChecklist(args[1:])
	case "render-matrix":
		return runRegistryRenderMatrix(args[1:])
	case "render-reliance":
		return runRegistryRenderReliance(args[1:])
	case "index-scarcity":
		return runRegistryIndexScarcity(args[1:])
	case "index-owner-inspection":
		return runRegistryIndexOwnerInspection(args[1:])
	case "render-method-lineage":
		return runRegistryRenderMethodLineage(args[1:])
	case "index-empirical":
		return runRegistryIndexEmpirical(args[1:])
	case "inventory":
		return runRegistryInventory(args[1:])
	case "public-derivative":
		return runRegistryPublicDerivative(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "registry: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runRegistryValidateIntake(args []string) int {
	flags := flag.NewFlagSet("registry validate-intake", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	entryPath := flags.String("entry", "", "intake entry JSON")
	catalogPath := flags.String("catalog", "", "optional JSON array of existing intake entries")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *entryPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry validate-intake: --entry is required; positional arguments are forbidden")
		return 2
	}
	raw, err := readRegistryPayload(*entryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry validate-intake:", err)
		return 1
	}
	var entry registry.IntakeEntry
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &entry); err != nil {
		fmt.Fprintln(os.Stderr, "registry validate-intake:", err)
		return 1
	}
	var catalog []registry.IntakeEntry
	if *catalogPath != "" {
		catalogRaw, catalogErr := readRegistryPayload(*catalogPath)
		if catalogErr != nil {
			fmt.Fprintln(os.Stderr, "registry validate-intake:", catalogErr)
			return 1
		}
		if err := protocol.DecodeStrict(bytes.TrimSpace(catalogRaw), &catalog); err != nil {
			fmt.Fprintln(os.Stderr, "registry validate-intake catalog:", err)
			return 1
		}
	}
	report, err := registry.ValidateIntakeAgainstCatalog(entry, catalog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry validate-intake:", err)
		return 1
	}
	code := writeCanonicalCommandOutput("registry validate-intake", report)
	if !report.Valid {
		return 1
	}
	return code
}

func runRegistryRenderMatrix(args []string) int {
	flags := flag.NewFlagSet("registry render-matrix", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	catalogPath := flags.String("catalog", "", "JSON array of intake entries")
	historyPath := flags.String("history", "", "optional previous contract matrix JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *catalogPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry render-matrix: --catalog is required; positional arguments are forbidden")
		return 2
	}
	raw, err := readRegistryPayload(*catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry render-matrix:", err)
		return 1
	}
	var catalog []registry.IntakeEntry
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &catalog); err != nil {
		fmt.Fprintln(os.Stderr, "registry render-matrix:", err)
		return 1
	}
	var previous registry.ContractMatrix
	if *historyPath != "" {
		historyRaw, historyErr := readRegistryPayload(*historyPath)
		if historyErr != nil {
			fmt.Fprintln(os.Stderr, "registry render-matrix:", historyErr)
			return 1
		}
		if err := protocol.DecodeStrict(bytes.TrimSpace(historyRaw), &previous); err != nil {
			fmt.Fprintln(os.Stderr, "registry render-matrix history:", err)
			return 1
		}
	}
	matrix, err := registry.MergeContractMatrix(previous, catalog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry render-matrix:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry render-matrix", matrix)
}

func runRegistryPreflight(args []string) int {
	flags := flag.NewFlagSet("registry preflight", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	entryPath := flags.String("entry", "", "intake entry JSON")
	catalogPath := flags.String("catalog", "", "optional JSON array of existing intake entries")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *entryPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry preflight: --entry is required; positional arguments are forbidden")
		return 2
	}
	raw, err := readRegistryPayload(*entryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry preflight:", err)
		return 1
	}
	var entry registry.IntakeEntry
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &entry); err != nil {
		fmt.Fprintln(os.Stderr, "registry preflight:", err)
		return 1
	}
	var catalog []registry.IntakeEntry
	if *catalogPath != "" {
		catalogRaw, catalogErr := readRegistryPayload(*catalogPath)
		if catalogErr != nil {
			fmt.Fprintln(os.Stderr, "registry preflight:", catalogErr)
			return 1
		}
		if err := protocol.DecodeStrict(bytes.TrimSpace(catalogRaw), &catalog); err != nil {
			fmt.Fprintln(os.Stderr, "registry preflight catalog:", err)
			return 1
		}
	}
	report, err := registry.PreflightIntake(entry, catalog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry preflight:", err)
		return 1
	}
	code := writeCanonicalCommandOutput("registry preflight", report)
	if !report.Valid {
		return 1
	}
	return code
}

func runRegistryTemplate(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "registry template: positional arguments are forbidden")
		return 2
	}
	if _, err := os.Stdout.Write(registry.IntakeTemplate()); err != nil {
		fmt.Fprintln(os.Stderr, "registry template:", err)
		return 1
	}
	return 0
}

func runRegistryRefresh(args []string) int {
	flags := flag.NewFlagSet("registry refresh", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	catalogPath := flags.String("catalog", "", "JSON array of intake entries")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *catalogPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry refresh: --catalog is required; positional arguments are forbidden")
		return 2
	}
	raw, err := readRegistryPayload(*catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry refresh:", err)
		return 1
	}
	var catalog []registry.IntakeEntry
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &catalog); err != nil {
		fmt.Fprintln(os.Stderr, "registry refresh:", err)
		return 1
	}
	report, err := registry.RefreshCatalog(catalog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry refresh:", err)
		return 1
	}
	code := writeCanonicalCommandOutput("registry refresh", report)
	if report.Rejected > 0 {
		return 1
	}
	return code
}

func runRegistryRenderReliance(args []string) int {
	flags := flag.NewFlagSet("registry render-reliance", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	catalogPath := flags.String("catalog", "", "JSON array of intake entries")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *catalogPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry render-reliance: --catalog is required; positional arguments are forbidden")
		return 2
	}
	raw, err := readRegistryPayload(*catalogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry render-reliance:", err)
		return 1
	}
	var catalog []registry.IntakeEntry
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &catalog); err != nil {
		fmt.Fprintln(os.Stderr, "registry render-reliance:", err)
		return 1
	}
	index, err := registry.RenderRelianceIndex(catalog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry render-reliance:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry render-reliance", index)
}

func runRegistryIndexScarcity(args []string) int {
	flags := flag.NewFlagSet("registry index-scarcity", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	evidencePath := flags.String("evidence", "", "committed scarcity negative-evidence JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *evidencePath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry index-scarcity: --evidence is required; positional arguments are forbidden")
		return 2
	}
	record, err := registry.IndexScarcityNegativeEvidence(*evidencePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry index-scarcity:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry index-scarcity", record)
}

func runRegistryIndexOwnerInspection(args []string) int {
	flags := flag.NewFlagSet("registry index-owner-inspection", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	attestationPath := flags.String("attestation", "", "committed public owner-inspection attestation JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *attestationPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry index-owner-inspection: --attestation is required; positional arguments are forbidden")
		return 2
	}
	record, err := registry.IndexOwnerInspectionAttestation(*attestationPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry index-owner-inspection:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry index-owner-inspection", record)
}

func runRegistryRenderMethodLineage(args []string) int {
	flags := flag.NewFlagSet("registry render-method-lineage", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	autopsyPath := flags.String("autopsy", "", "AutopsyView JSON with method_integrity")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *autopsyPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry render-method-lineage: --autopsy is required; positional arguments are forbidden")
		return 2
	}
	view, err := registry.LoadMethodLineageView(*autopsyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry render-method-lineage:", err)
		return 1
	}
	record, err := registry.RenderMethodLineage(view)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry render-method-lineage:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry render-method-lineage", record)
}

func runRegistryIndexEmpirical(args []string) int {
	flags := flag.NewFlagSet("registry index-empirical", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	attestationPath := flags.String("attestation", "", "committed public owner-inspection attestation JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *attestationPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry index-empirical: --attestation is required; positional arguments are forbidden")
		return 2
	}
	index, err := registry.IndexEmpiricalValidity(*attestationPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry index-empirical:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry index-empirical", index)
}

func runRegistryInventory(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "registry inventory: positional arguments are forbidden")
		return 2
	}
	inventory, err := registry.ProviderEvidenceInventory()
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry inventory:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry inventory", inventory)
}

func runRegistryPublicDerivative(args []string) int {
	flags := flag.NewFlagSet("registry public-derivative", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	entryPath := flags.String("entry", "", "intake entry JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *entryPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "registry public-derivative: --entry is required; positional arguments are forbidden")
		return 2
	}
	raw, err := readRegistryPayload(*entryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry public-derivative:", err)
		return 1
	}
	var entry registry.IntakeEntry
	if err := protocol.DecodeStrict(bytes.TrimSpace(raw), &entry); err != nil {
		fmt.Fprintln(os.Stderr, "registry public-derivative:", err)
		return 1
	}
	derivative, err := registry.RenderPublicDerivative(entry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry public-derivative:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry public-derivative", derivative)
}

func runRegistryReviewChecklist(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "registry review-checklist: positional arguments are forbidden")
		return 2
	}
	checklist, err := registry.MaintainerReviewChecklist()
	if err != nil {
		fmt.Fprintln(os.Stderr, "registry review-checklist:", err)
		return 1
	}
	return writeCanonicalCommandOutput("registry review-checklist", checklist)
}

func readRegistryPayload(path string) ([]byte, error) {
	raw, err := readBoundedCommandFile(path)
	if err != nil {
		return nil, err
	}
	if err := registry.RejectUnboundedAllocation(len(raw)); err != nil {
		return nil, err
	}
	if err := registry.RejectArchivePayload(path, raw); err != nil {
		return nil, err
	}
	return raw, nil
}
