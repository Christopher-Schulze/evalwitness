package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func runMutationVerificationEvidence(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "mutation verification-evidence: usage: evalwitness mutation verification-evidence <build-challenge|validate-challenge>")
		return 2
	}
	switch args[0] {
	case "build-challenge":
		return runMutationVerificationEvidenceBuildChallenge(args[1:])
	case "validate-challenge":
		return runMutationVerificationEvidenceValidateChallenge(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "mutation verification-evidence: unknown command %q\n", args[0])
		return 2
	}
}

func runMutationVerificationEvidenceBuildChallenge(args []string) int {
	flags := flag.NewFlagSet("mutation verification-evidence build-challenge", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "mutation verification-evidence build-challenge: positional arguments are not supported")
		return 2
	}
	challenge, err := mutation.BuildVerificationEvidenceChallenge()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation verification-evidence build-challenge:", err)
		return 1
	}
	return printMutationJSON(challenge)
}

func runMutationVerificationEvidenceValidateChallenge(args []string) int {
	flags := flag.NewFlagSet("mutation verification-evidence validate-challenge", flag.ContinueOnError)
	challengePath := flags.String("challenge", "", "verification-evidence challenge (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeChallenge, err := openStudyDocument(*challengePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation verification-evidence validate-challenge:", err)
		return 2
	}
	defer closeChallenge()
	challenge, err := mutation.DecodeVerificationEvidenceChallenge(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation verification-evidence validate-challenge:", err)
		return 2
	}
	if err := mutation.VerifyVerificationEvidenceChallenge(challenge); err != nil {
		fmt.Fprintln(os.Stderr, "mutation verification-evidence validate-challenge:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid    bool   `json:"valid"`
		Digest   string `json:"digest"`
		Cases    int    `json:"cases"`
		Eligible int    `json:"eligible"`
		Rejected int    `json:"rejected"`
	}{
		Valid: true, Digest: challenge.Digest, Cases: challenge.Summary.Cases,
		Eligible: challenge.Summary.Eligible, Rejected: challenge.Summary.Rejected,
	})
}
