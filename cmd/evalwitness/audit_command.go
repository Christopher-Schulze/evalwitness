package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/audit"
	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// runAudit implements the offline CI surface: `audit --policy FILE --profile
// FILE [--format json|junit|markdown]`. It validates a canonical policy,
// resolves declared local inputs, prints the plan, evaluates TASK 058
// requirements offline and returns stable exit codes:
// 0 pass, 1 policy failed, 2 invalid input, 3 internal error.
func runAudit(args []string) int {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	policyPath := fs.String("policy", "", "policy JSON file")
	profilePath := fs.String("profile", "", "profile JSON file")
	format := fs.String("format", "json", "json|junit|markdown|sarif")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "audit:", err)
		return audit.ExitInvalidInput
	}
	if *policyPath == "" || *profilePath == "" {
		fmt.Fprintln(os.Stderr, "audit: --policy and --profile required")
		return audit.ExitInvalidInput
	}
	polb, err := os.ReadFile(*policyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit:", err)
		return audit.ExitInvalidInput
	}
	var pol profile.Policy
	if err := decodeStrictJSON(polb, &pol); err != nil {
		fmt.Fprintln(os.Stderr, "audit policy:", err)
		return audit.ExitInvalidInput
	}
	p, err := loadVerifiedAuditProfile(*profilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit profile:", err)
		return audit.ExitInvalidInput
	}
	steps, err := audit.Plan(pol, p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit plan:", err)
		return audit.ExitInternalError
	}
	for _, step := range steps {
		fmt.Fprintf(os.Stderr, "plan: %s policy=%s\n", step.ID, step.Digest)
	}
	res, err := audit.OfflineAudit(p, pol)
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit:", err)
		return audit.ExitInternalError
	}
	digest, err := pol.DigestValue()
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit digest:", err)
		return audit.ExitInternalError
	}
	result := audit.Result{
		SchemaVersion: audit.SchemaVersion,
		Pass:          res.Pass,
		Offline:       true,
		PolicyDigest:  digest,
		ProfileDigest: p.Digest,
		Fails:         res.Fails,
	}
	if !res.Pass {
		result.Explanations = audit.ExplainFailures(p, pol, res.Fails)
	}
	switch *format {
	case "json":
		b, err := audit.EncodeCanonical(result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "audit encode:", err)
			return audit.ExitInternalError
		}
		fmt.Println(string(b))
	case "junit":
		b, err := audit.EncodeJUnit(result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "audit junit:", err)
			return audit.ExitInternalError
		}
		fmt.Print(string(b))
	case "markdown":
		b, err := audit.EncodeMarkdown(result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "audit markdown:", err)
			return audit.ExitInternalError
		}
		fmt.Print(string(b))
	case "sarif":
		b, err := audit.EncodeSARIF(result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "audit sarif:", err)
			return audit.ExitInternalError
		}
		fmt.Println(string(b))
	default:
		fmt.Fprintf(os.Stderr, "audit: unknown format %q\n", *format)
		return audit.ExitInvalidInput
	}
	if result.Pass {
		return audit.ExitPass
	}
	return audit.ExitPolicyFailed
}

// loadVerifiedAuditProfile decodes a profile strictly (DisallowUnknownFields),
// validates it and enforces digest equality; identical semantics to the claim
// verify claimcheck gate.
func loadVerifiedAuditProfile(path string) (profile.Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, err
	}
	var p profile.Profile
	if err := decodeStrictJSON(raw, &p); err != nil {
		return profile.Profile{}, err
	}
	if err := p.Validate(); err != nil {
		return profile.Profile{}, err
	}
	digest, err := p.DigestValue()
	if err != nil {
		return profile.Profile{}, err
	}
	if digest != p.Digest {
		return profile.Profile{}, fmt.Errorf("profile digest mismatch: %s vs %s", p.Digest, digest)
	}
	return p, nil
}
