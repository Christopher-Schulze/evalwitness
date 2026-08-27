package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// runProfile implements the offline `profile build|verify|diff|policy|render` surface.
func runProfile(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "profile: subcommand required: build|verify|diff|policy|render")
		return 2
	}
	switch args[0] {
	case "build":
		return runProfileBuild(args[1:])
	case "verify":
		return runProfileVerify(args[1:])
	case "diff":
		return runProfileDiff(args[1:])
	case "policy":
		return runProfilePolicy(args[1:])
	case "render":
		return runProfileRender(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "profile: unknown subcommand %q\n", args[0])
		return 2
	}
}

// parseDim parses ID:STATUS:METRIC:SCOPE:LEVEL:EXPR:DENOM:UNIT where EXPR may
// itself contain colons (e.g. capsule:sha256-...); DENOM:UNIT is split from the right.
func parseDim(spec string) (profile.Dimension, error) {
	last := strings.LastIndex(spec, ":")
	if last < 0 {
		return profile.Dimension{}, fmt.Errorf("dim spec needs ID:STATUS:METRIC:SCOPE:LEVEL:EXPR:DENOM:UNIT")
	}
	second := strings.LastIndex(spec[:last], ":")
	if second < 0 {
		return profile.Dimension{}, fmt.Errorf("dim spec needs ID:STATUS:METRIC:SCOPE:LEVEL:EXPR:DENOM:UNIT")
	}
	head := spec[:second]
	denomUnit := spec[second+1:]
	parts := strings.SplitN(head, ":", 6)
	if len(parts) != 6 {
		return profile.Dimension{}, fmt.Errorf("dim spec prefix needs 6 fields before DENOM:UNIT, got %d in %q", len(parts), head)
	}
	du := strings.SplitN(denomUnit, ":", 2)
	if len(du) != 2 {
		return profile.Dimension{}, fmt.Errorf("dim spec tail needs DENOM:UNIT, got %q", denomUnit)
	}
	d := profile.Dimension{ID: parts[0], Status: profile.Status(parts[1]), Metric: &parts[2], Scope: parts[3], EvidenceLevel: parts[4], CapsuleExpr: parts[5], SampleUnit: du[1]}
	if _, err := fmt.Sscanf(du[0], "%d", &d.Denominator); err != nil {
		return profile.Dimension{}, fmt.Errorf("denominator %q: %w", du[0], err)
	}
	if parts[2] == "-" {
		d.Metric = nil
	}
	return d, nil
}

func runProfileBuild(args []string) int {
	fs := flag.NewFlagSet("profile build", flag.ContinueOnError)
	identity := fs.String("identity", "", "profile identity")
	protocol := fs.String("protocol", "evalwitness.protocol.v1", "protocol version")
	route := fs.String("route", "", "route scope")
	var dimSpecs multiFlag
	fs.Var(&dimSpecs, "dim", "dimension spec ID:STATUS:METRIC:SCOPE:LEVEL:EXPR:DENOM:UNIT (repeatable); METRIC - means absent")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "profile build:", err)
		return 2
	}
	if *identity == "" || *route == "" || len(dimSpecs) == 0 {
		fmt.Fprintln(os.Stderr, "profile build: --identity, --route, and at least one --dim required")
		return 2
	}
	dims := make([]profile.Dimension, 0, len(dimSpecs))
	for _, spec := range dimSpecs {
		d, err := parseDim(spec)
		if err != nil {
			fmt.Fprintln(os.Stderr, "profile build:", err)
			return 2
		}
		dims = append(dims, d)
	}
	p, err := profile.CLIBuild(*identity, *protocol, *route, dims)
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile build:", err)
		return 1
	}
	return printJSON(p)
}

func runProfileVerify(args []string) int {
	p, code := readProfileFlag(args, "profile verify")
	if code != 0 {
		return code
	}
	if err := profile.CLIVerify(p); err != nil {
		fmt.Fprintln(os.Stderr, "profile verify:", err)
		return 1
	}
	fmt.Printf("verified %s digest %s\n", p.Identity, p.Digest)
	return 0
}

func runProfileDiff(args []string) int {
	fs := flag.NewFlagSet("profile diff", flag.ContinueOnError)
	aPath := fs.String("a", "", "profile A JSON")
	bPath := fs.String("b", "", "profile B JSON")
	format := fs.String("format", "json", "json|text")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "profile diff:", err)
		return 2
	}
	if *aPath == "" || *bPath == "" {
		fmt.Fprintln(os.Stderr, "profile diff: --a and --b required")
		return 2
	}
	a, err := readProfileFile(*aPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile diff:", err)
		return 1
	}
	b, err := readProfileFile(*bPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile diff:", err)
		return 1
	}
	d := profile.Diff(a, b)
	if *format == "text" {
		fmt.Printf("compatible=%v added=%v removed=%v changed=%v\n", d.Compatible, d.Added, d.Removed, d.Changed)
		return 0
	}
	return printJSON(d)
}

func runProfilePolicy(args []string) int {
	fs := flag.NewFlagSet("profile policy", flag.ContinueOnError)
	policyPath := fs.String("policy", "", "policy JSON file")
	profilePath := fs.String("in", "", "profile JSON file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "profile policy:", err)
		return 2
	}
	if *policyPath == "" || *profilePath == "" {
		fmt.Fprintln(os.Stderr, "profile policy: --policy and --in required")
		return 2
	}
	p, err := readProfileFile(*profilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile policy:", err)
		return 1
	}
	pb, err := os.ReadFile(*policyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile policy:", err)
		return 1
	}
	var pol profile.Policy
	if err := json.Unmarshal(pb, &pol); err != nil {
		fmt.Fprintln(os.Stderr, "profile policy:", err)
		return 1
	}
	ok, fails := profile.Evaluate(p, pol)
	out := struct {
		Pass   bool     `json:"pass"`
		Fails  []string `json:"fails"`
		Policy string   `json:"policy_digest"`
	}{Pass: ok, Fails: fails, Policy: pol.Digest}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
	if ok {
		return 0
	}
	return 1
}

func runProfileRender(args []string) int {
	fs := flag.NewFlagSet("profile render", flag.ContinueOnError)
	in := fs.String("in", "", "profile JSON file")
	format := fs.String("format", "text", "text|markdown|report")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "profile render:", err)
		return 2
	}
	if *in == "" {
		fmt.Fprintln(os.Stderr, "profile render: --in required")
		return 2
	}
	p, err := readProfileFile(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile render:", err)
		return 1
	}
	switch *format {
	case "text":
		fmt.Println(profile.TextReport(p))
	case "markdown":
		fmt.Print(profile.MarkdownReport(p))
	case "report":
		rep, err := profile.BuildEvidenceReport(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "profile render:", err)
			return 1
		}
		return printJSON(rep)
	default:
		fmt.Fprintf(os.Stderr, "profile render: unknown format %q\n", *format)
		return 2
	}
	return 0
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func readProfileFile(path string) (profile.Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, err
	}
	var p profile.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return profile.Profile{}, err
	}
	return p, nil
}

// readProfileFlag reads a single --in flag profile; returns non-zero code on failure.
func readProfileFlag(args []string, name string) (profile.Profile, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	in := fs.String("in", "", "profile JSON file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, name+":", err)
		return profile.Profile{}, 2
	}
	if *in == "" {
		fmt.Fprintln(os.Stderr, name+": --in required")
		return profile.Profile{}, 2
	}
	p, err := readProfileFile(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, name+":", err)
		return profile.Profile{}, 1
	}
	return p, 0
}

func printJSON(v any) int {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "json:", err)
		return 1
	}
	fmt.Println(string(b))
	return 0
}
