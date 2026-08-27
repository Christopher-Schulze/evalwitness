package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func runArchive(args []string) int {
	if len(args) == 0 || args[0] != "inspect" && args[0] != "extract" {
		fmt.Fprintln(os.Stderr, "archive: expected inspect or extract")
		return 2
	}
	command := args[0]
	flags := flag.NewFlagSet("archive "+command, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var sources stringSlice
	var expectedRoots stringSlice
	flags.Var(&sources, "source", "tar.gz source path (repeatable)")
	flags.Var(&expectedRoots, "expected-root", "allowed top-level archive root (repeatable)")
	destination := flags.String("destination", "", "atomic extraction destination")
	limits := addArchiveLimitFlags(flags)
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if len(sources) == 0 || command == "extract" && *destination == "" {
		fmt.Fprintln(os.Stderr, "archive: --source is required; extract also requires --destination")
		return 2
	}
	result, err := executeArchiveCommand(command, sources, expectedRoots, *destination, limits)
	if err != nil {
		fmt.Fprintln(os.Stderr, "archive:", err)
		return 1
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "archive output:", err)
		return 1
	}
	fmt.Println(string(raw))
	return 0
}

func addArchiveLimitFlags(flags *flag.FlagSet) *safety.ArchiveLimits {
	limits := safety.DefaultArchiveLimits()
	flags.IntVar(&limits.MaxEntries, "max-entries", limits.MaxEntries, "maximum archive entries")
	flags.Int64Var(&limits.MaxExpandedBytes, "max-expanded-bytes", limits.MaxExpandedBytes, "maximum total expanded bytes")
	flags.Int64Var(&limits.MaxEntryBytes, "max-entry-bytes", limits.MaxEntryBytes, "maximum bytes in one file")
	flags.Float64Var(&limits.MaxCompressionRatio, "max-compression-ratio", limits.MaxCompressionRatio, "maximum expanded/compressed ratio")
	flags.IntVar(&limits.MaxDepth, "max-depth", limits.MaxDepth, "maximum archive path depth")
	flags.IntVar(&limits.MaxPathBytes, "max-path-bytes", limits.MaxPathBytes, "maximum archive path bytes")
	flags.Int64Var(&limits.ReservationHeadroomBytes, "reservation-headroom-bytes", limits.ReservationHeadroomBytes, "disk bytes reserved beyond expanded content")
	return &limits
}

func executeArchiveCommand(command string, sources, expectedRoots []string, destination string, limits *safety.ArchiveLimits) (safety.ArchiveExtractResult, error) {
	if limits == nil {
		return safety.ArchiveExtractResult{}, &safety.Error{Kind: safety.ErrorInvalidInput, Operation: safety.OperationValidate}
	}
	if command == "inspect" {
		return safety.InspectTarGzip(context.Background(), safety.ArchiveInspectRequest{
			Sources: sources, ExpectedRoots: expectedRoots, Limits: *limits,
		})
	}
	return safety.ExtractTarGzip(context.Background(), safety.ArchiveExtractRequest{
		Sources: sources, Destination: destination, ExpectedRoots: expectedRoots, Limits: *limits,
	})
}
