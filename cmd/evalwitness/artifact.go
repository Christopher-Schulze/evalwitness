package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

func runArtifact(args []string) int {
	if len(args) == 0 || args[0] != "scan" {
		fmt.Fprintln(os.Stderr, "artifact: expected scan")
		return 2
	}
	flags := flag.NewFlagSet("artifact scan", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var paths stringSlice
	flags.Var(&paths, "path", "file or directory to scan (repeatable)")
	class := flags.String("class", "", "artifact class (public or sensitive)")
	reviewManifestPath := flags.String("reviewed-findings", "", "exact-content reviewed-findings manifest")
	reviewCandidatePath := flags.String("write-review-candidate", "", "write a new review candidate without approving it")
	artifactID := flags.String("artifact-id", "", "stable artifact ID for a review candidate")
	limits := addArtifactScanLimitFlags(flags)
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if len(paths) == 0 || !safety.ArtifactClass(*class).Valid() {
		fmt.Fprintln(os.Stderr, "artifact scan: --class public|sensitive and at least one --path are required")
		return 2
	}
	if *reviewCandidatePath != "" && (*artifactID == "" || *reviewManifestPath != "") {
		fmt.Fprintln(os.Stderr, "artifact scan: --write-review-candidate requires --artifact-id and cannot be combined with --reviewed-findings")
		return 2
	}
	report, scanErr := executeArtifactScan(paths, safety.ArtifactClass(*class), limits, os.Environ())
	if *reviewManifestPath != "" && (scanErr == nil || safety.IsKind(scanErr, safety.ErrorSecretDetected) || safety.IsKind(scanErr, safety.ErrorArtifactPolicyViolation)) {
		manifest, err := safety.LoadArtifactReviewManifest(*reviewManifestPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "artifact review:", err)
			return 1
		}
		report, scanErr = safety.ApplyArtifactReviewManifest(report, manifest)
	}
	if *reviewCandidatePath != "" {
		manifest, err := safety.ReviewManifestFromReport(*artifactID, report)
		if err == nil {
			err = safety.WriteArtifactReviewCandidate(*reviewCandidatePath, manifest)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "artifact review candidate:", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, "artifact review candidate written; review every finding before use")
	}
	raw, marshalErr := json.MarshalIndent(report, "", "  ")
	if marshalErr != nil {
		fmt.Fprintln(os.Stderr, "artifact scan output:", marshalErr)
		return 1
	}
	fmt.Println(string(raw))
	if scanErr != nil {
		fmt.Fprintln(os.Stderr, "artifact scan:", scanErr)
		return 1
	}
	return 0
}

func addArtifactScanLimitFlags(flags *flag.FlagSet) *safety.ArtifactScanLimits {
	limits := safety.DefaultArtifactScanLimits()
	flags.IntVar(&limits.MaxFiles, "max-files", limits.MaxFiles, "maximum files")
	flags.Int64Var(&limits.MaxFileBytes, "max-file-bytes", limits.MaxFileBytes, "maximum bytes in one file")
	flags.Int64Var(&limits.MaxTotalBytes, "max-total-bytes", limits.MaxTotalBytes, "maximum total bytes")
	flags.IntVar(&limits.MaxDepth, "max-depth", limits.MaxDepth, "maximum path depth")
	flags.IntVar(&limits.MaxPathBytes, "max-path-bytes", limits.MaxPathBytes, "maximum path bytes")
	flags.IntVar(&limits.MaxFindings, "max-findings", limits.MaxFindings, "maximum reported findings")
	return &limits
}

func executeArtifactScan(paths []string, class safety.ArtifactClass, limits *safety.ArtifactScanLimits, environment []string) (safety.ArtifactScanReport, error) {
	if limits == nil {
		return safety.ArtifactScanReport{Class: class}, &safety.Error{Kind: safety.ErrorInvalidInput, Operation: safety.OperationScan}
	}
	return safety.ScanArtifacts(safety.ArtifactScanRequest{
		Roots:        paths,
		Class:        class,
		KnownSecrets: safety.SecretsFromEnvironment(environment),
		Limits:       *limits,
	})
}
