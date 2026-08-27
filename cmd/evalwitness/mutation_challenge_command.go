package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func runMutationConstructChallenge(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "mutation construct-challenge: usage: evalwitness mutation construct-challenge <build|validate>")
		return 2
	}
	switch args[0] {
	case "build":
		return runMutationConstructChallengeBuild(args[1:])
	case "validate":
		return runMutationConstructChallengeValidate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "mutation construct-challenge: unknown command %q\n", args[0])
		return 2
	}
}

func runMutationConstructChallengeBuild(args []string) int {
	flags := flag.NewFlagSet("mutation construct-challenge build", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "mutation construct-challenge build: positional arguments are not supported")
		return 2
	}
	evidence, err := mutation.BuildConstructChallengeEvidence()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-challenge build:", err)
		return 1
	}
	return printMutationJSON(evidence)
}

func runMutationConstructChallengeValidate(args []string) int {
	flags := flag.NewFlagSet("mutation construct-challenge validate", flag.ContinueOnError)
	evidencePath := flags.String("evidence", "", "construct-firewall challenge evidence (@file)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	reader, closeEvidence, err := openStudyDocument(*evidencePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-challenge validate:", err)
		return 2
	}
	defer closeEvidence()
	evidence, err := mutation.DecodeConstructChallengeEvidence(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-challenge validate:", err)
		return 2
	}
	if err := mutation.VerifyConstructChallengeEvidence(evidence); err != nil {
		fmt.Fprintln(os.Stderr, "mutation construct-challenge validate:", err)
		return 1
	}
	return printMutationJSON(struct {
		Valid          bool   `json:"valid"`
		EvidenceDigest string `json:"evidence_digest"`
		Cases          int    `json:"cases"`
	}{Valid: true, EvidenceDigest: evidence.Digest, Cases: len(evidence.Cases)})
}
