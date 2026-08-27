package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

type claimExplainReport struct {
	SchemaVersion string                        `json:"schema_version"`
	Claim         claimledger.Claim             `json:"claim"`
	Verification  claimledger.ClaimVerification `json:"verification"`
}

type scopedChallengeReport struct {
	SchemaVersion string                         `json:"schema_version"`
	ClaimID       string                         `json:"claim_id"`
	CapsuleID     string                         `json:"capsule_id"`
	LedgerDigest  string                         `json:"ledger_digest"`
	Receipts      []claimledger.ChallengeReceipt `json:"receipts"`
}

func runClaim(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "claim: missing subcommand (verify|explain|challenge|autopsy|report|render|surface)")
		return 2
	}
	switch args[0] {
	case "verify":
		return runClaimVerify(args[1:])
	case "explain":
		return runClaimExplain(args[1:])
	case "challenge":
		return runClaimChallenge(args[1:])
	case "autopsy":
		return runClaimAutopsy(args[1:])
	case "report":
		return runClaimReport(args[1:])
	case "render":
		return runClaimRender(args[1:])
	case "surface":
		return runClaimSurface(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "claim: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runClaimAutopsy(args []string) int {
	flags := flag.NewFlagSet("claim autopsy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim autopsy:", err)
		return 2
	}
	registry, manifest, payloads, ledger, code := loadClaimCommandEvidence(*capsulePath, *ledgerPath, flags.NArg())
	if code != 0 {
		return code
	}
	autopsy, err := claimledger.BuildAutopsy(context.Background(), registry, manifest, payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim autopsy:", err)
		return 1
	}
	return writeCanonicalCommandOutput("claim autopsy", autopsy)
}

func runClaimVerify(args []string) int {
	flags := flag.NewFlagSet("claim verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	claimID := flags.String("claim", "", "optional CLM-NNN")
	profilePath := flags.String("profile", "", "optional profile JSON to feed claimcheck")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim verify:", err)
		return 2
	}
	registry, manifest, payloads, ledger, code := loadClaimCommandEvidence(*capsulePath, *ledgerPath, flags.NArg())
	if code != 0 {
		return code
	}
	var profileExprs []string
	var profileStatuses map[string]profile.Status
	if *profilePath != "" {
		exprs, statuses, err := checkProfileForClaimVerify(*profilePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "claim verify profile:", err)
			return 1
		}
		profileExprs = exprs
		profileStatuses = statuses
	}
	if *claimID != "" {
		verification, err := claimledger.VerifyClaim(context.Background(), registry, manifest, payloads, ledger, *claimID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "claim verify:", err)
			return 1
		}
		if profileExprs != nil {
			out := struct {
				Verification      claimledger.ClaimVerification `json:"verification"`
				ProfileClaimCheck []string                      `json:"profile_claimcheck"`
				ProfileStatuses   map[string]profile.Status     `json:"profile_statuses"`
			}{Verification: verification, ProfileClaimCheck: profileExprs, ProfileStatuses: profileStatuses}
			return writeCanonicalCommandOutput("claim verify", out)
		}
		return writeCanonicalCommandOutput("claim verify", verification)
	}
	report, err := claimledger.VerifyLedger(context.Background(), registry, manifest, payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim verify:", err)
		return 1
	}
	if profileExprs != nil {
		out := struct {
			Verification      claimledger.VerificationReport `json:"verification"`
			ProfileClaimCheck []string                       `json:"profile_claimcheck"`
			ProfileStatuses   map[string]profile.Status      `json:"profile_statuses"`
		}{Verification: report, ProfileClaimCheck: profileExprs, ProfileStatuses: profileStatuses}
		return writeCanonicalCommandOutput("claim verify", out)
	}
	return writeCanonicalCommandOutput("claim verify", report)
}

func checkProfileForClaimVerify(path string) ([]string, map[string]profile.Status, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var p profile.Profile
	if err := dec.Decode(&p); err != nil {
		return nil, nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, nil, err
	}
	dig, err := p.DigestValue()
	if err != nil {
		return nil, nil, err
	}
	if dig != p.Digest {
		return nil, nil, fmt.Errorf("profile digest mismatch: %s vs %s", p.Digest, dig)
	}
	exprs, err := claimledger.CheckProfile(p)
	if err != nil {
		return nil, nil, err
	}
	statuses := profile.ClaimCheckStatuses(p)
	return exprs, statuses, nil
}
func runClaimExplain(args []string) int {
	flags := flag.NewFlagSet("claim explain", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	claimID := flags.String("claim", "", "required CLM-NNN")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim explain:", err)
		return 2
	}
	if *claimID == "" {
		fmt.Fprintln(os.Stderr, "claim explain: --claim is required")
		return 2
	}
	registry, manifest, payloads, ledger, code := loadClaimCommandEvidence(*capsulePath, *ledgerPath, flags.NArg())
	if code != 0 {
		return code
	}
	item, err := claimledger.Lookup(ledger, *claimID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim explain:", err)
		return 1
	}
	verification, err := claimledger.VerifyClaim(context.Background(), registry, manifest, payloads, ledger, *claimID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim explain:", err)
		return 1
	}
	report := claimExplainReport{SchemaVersion: "evalwitness.claim-explanation.v1", Claim: item, Verification: verification}
	return writeCanonicalCommandOutput("claim explain", report)
}

func runClaimChallenge(args []string) int {
	flags := flag.NewFlagSet("claim challenge", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	claimID := flags.String("claim", "", "CLM-NNN; optional only with --all")
	challengeID := flags.String("challenge", "", "closed challenge ID")
	all := flags.Bool("all", false, "run every applicable challenge")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim challenge:", err)
		return 2
	}
	if *all && *challengeID != "" || !*all && (*claimID == "" || *challengeID == "") {
		fmt.Fprintln(os.Stderr, "claim challenge: use --claim and --challenge, or use --all with optional --claim")
		return 2
	}
	registry, manifest, payloads, ledger, code := loadClaimCommandEvidence(*capsulePath, *ledgerPath, flags.NArg())
	if code != 0 {
		return code
	}
	ctx := context.Background()
	if !*all {
		receipt, err := claimledger.Challenge(ctx, registry, manifest, payloads, ledger, *claimID, *challengeID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "claim challenge:", err)
			return 1
		}
		return writeCanonicalCommandOutput("claim challenge", receipt)
	}
	if *claimID == "" {
		pack, err := claimledger.BuildChallengePack(ctx, registry, manifest, payloads, ledger)
		if err != nil {
			fmt.Fprintln(os.Stderr, "claim challenge:", err)
			return 1
		}
		return writeCanonicalCommandOutput("claim challenge", pack)
	}
	item, err := claimledger.Lookup(ledger, *claimID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim challenge:", err)
		return 1
	}
	receipts := make([]claimledger.ChallengeReceipt, 0, len(item.Challenges))
	for _, specification := range item.Challenges {
		if specification.Applicability != claimledger.ChallengeApplied {
			continue
		}
		receipt, challengeErr := claimledger.Challenge(ctx, registry, manifest, payloads, ledger, item.ClaimID, specification.ChallengeID)
		if challengeErr != nil {
			fmt.Fprintln(os.Stderr, "claim challenge:", challengeErr)
			return 1
		}
		receipts = append(receipts, receipt)
	}
	report := scopedChallengeReport{
		SchemaVersion: "evalwitness.claim-scoped-challenge-report.v1", ClaimID: item.ClaimID,
		CapsuleID: manifest.CapsuleID, LedgerDigest: ledger.Digest, Receipts: receipts,
	}
	return writeCanonicalCommandOutput("claim challenge", report)
}

func loadClaimCommandEvidence(capsulePath, ledgerPath string, positionalArguments int) (*capsule.Registry, capsule.Manifest, map[string][]byte, claimledger.Ledger, int) {
	if capsulePath == "" || ledgerPath == "" || positionalArguments != 0 {
		fmt.Fprintln(os.Stderr, "claim: --capsule and --ledger are required and positional arguments are forbidden")
		return nil, capsule.Manifest{}, nil, claimledger.Ledger{}, 2
	}
	registry, err := capsule.ReferenceRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim registry:", err)
		return nil, capsule.Manifest{}, nil, claimledger.Ledger{}, 1
	}
	manifest, payloads, err := capsule.LoadDirectory(
		context.Background(), capsulePath, registry,
		capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim capsule:", err)
		return nil, capsule.Manifest{}, nil, claimledger.Ledger{}, 1
	}
	ledger, err := readClaimLedger(ledgerPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim ledger:", err)
		return nil, capsule.Manifest{}, nil, claimledger.Ledger{}, 1
	}
	return registry, manifest, payloads, ledger, 0
}
