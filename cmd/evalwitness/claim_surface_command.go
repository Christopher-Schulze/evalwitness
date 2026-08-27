package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
)

type trackedClaimSurface struct {
	surface string
	path    string
	format  string
}

var trackedClaimSurfaces = []trackedClaimSurface{
	{surface: claimledger.SurfaceDocumentation, path: "docs/documentation.md", format: "markdown"},
	{surface: claimledger.SurfaceFindings, path: "docs/findings.md", format: "markdown"},
	{surface: claimledger.SurfaceREADME, path: "README.md", format: "markdown"},
	{surface: claimledger.SurfaceRelease, path: "docs/releasing.md", format: "markdown"},
	{surface: claimledger.SurfaceResult, path: "eval/results/claim-surface-v1.json", format: "json"},
	{surface: claimledger.SurfaceSkill, path: ".agents/skills/evalwitness-audit/SKILL.md", format: "markdown"},
}

type claimSurfaceVerificationReport struct {
	SchemaVersion string `json:"schema_version"`
	Surfaces      int    `json:"surfaces"`
	Claims        int    `json:"claims"`
	ProviderCalls int    `json:"provider_calls"`
	Offline       bool   `json:"offline"`
	Valid         bool   `json:"valid"`
}

func runClaimSurface(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "claim surface: missing subcommand (render|verify)")
		return 2
	}
	switch args[0] {
	case "render":
		return runClaimSurfaceRender(args[1:])
	case "verify":
		return runClaimSurfaceVerify(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "claim surface: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runClaimSurfaceRender(args []string) int {
	flags := flag.NewFlagSet("claim surface render", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	surface := flags.String("surface", "", "closed surface ID")
	format := flags.String("format", "markdown", "markdown or json")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim surface render:", err)
		return 2
	}
	registry, manifest, payloads, ledger, code := loadClaimCommandEvidence(*capsulePath, *ledgerPath, flags.NArg())
	if code != 0 {
		return code
	}
	views, err := claimledger.BuildSurfaceViews(context.Background(), registry, manifest, payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim surface render:", err)
		return 1
	}
	view, found := views[*surface]
	if !found {
		fmt.Fprintf(os.Stderr, "claim surface render: unknown surface %q\n", *surface)
		return 2
	}
	var raw []byte
	switch *format {
	case "json":
		raw, err = claimledger.EncodeSurfaceView(view)
		if err == nil {
			raw = append(raw, '\n')
		}
	case "markdown":
		raw, err = claimledger.RenderSurfaceMarkdown(view)
	default:
		fmt.Fprintf(os.Stderr, "claim surface render: unknown format %q\n", *format)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim surface render:", err)
		return 1
	}
	if err := writeOutput(os.Stdout, raw); err != nil {
		fmt.Fprintln(os.Stderr, "claim surface render:", err)
		return 1
	}
	return 0
}

func runClaimSurfaceVerify(args []string) int {
	flags := flag.NewFlagSet("claim surface verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	capsulePath := flags.String("capsule", "", "capsule directory")
	ledgerPath := flags.String("ledger", "", "canonical claim ledger")
	repositoryRoot := flags.String("repository-root", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "claim surface verify:", err)
		return 2
	}
	registry, manifest, payloads, ledger, code := loadClaimCommandEvidence(*capsulePath, *ledgerPath, flags.NArg())
	if code != 0 {
		return code
	}
	views, err := claimledger.BuildSurfaceViews(context.Background(), registry, manifest, payloads, ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claim surface verify:", err)
		return 1
	}
	for _, specification := range trackedClaimSurfaces {
		view := views[specification.surface]
		path := filepath.Join(*repositoryRoot, filepath.FromSlash(specification.path))
		actual, readErr := readClaimSurfaceFile(path)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "claim surface verify %s: %v\n", specification.surface, readErr)
			return 1
		}
		switch specification.format {
		case "markdown":
			expected, renderErr := claimledger.RenderSurfaceMarkdown(view)
			if renderErr != nil {
				fmt.Fprintln(os.Stderr, "claim surface verify:", renderErr)
				return 1
			}
			block, extractErr := extractClaimSurfaceBlock(actual, specification.surface)
			if extractErr != nil || !bytes.Equal(block, expected) {
				fmt.Fprintf(os.Stderr, "claim surface verify %s: tracked Markdown differs from deterministic ledger projection\n", specification.surface)
				return 1
			}
		case "json":
			expected, encodeErr := claimledger.EncodeSurfaceView(view)
			if encodeErr != nil {
				fmt.Fprintln(os.Stderr, "claim surface verify:", encodeErr)
				return 1
			}
			expected = append(expected, '\n')
			if !bytes.Equal(actual, expected) {
				fmt.Fprintln(os.Stderr, "claim surface verify result: tracked manifest differs from deterministic ledger projection")
				return 1
			}
		default:
			fmt.Fprintln(os.Stderr, "claim surface verify: internal surface format is invalid")
			return 1
		}
	}
	return writeCanonicalCommandOutput("claim surface verify", claimSurfaceVerificationReport{
		SchemaVersion: "evalwitness.claim-surface-verification.v1", Surfaces: len(trackedClaimSurfaces),
		Claims: len(ledger.Claims), ProviderCalls: 0, Offline: true, Valid: true,
	})
}

func readClaimSurfaceFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maximumEvidenceCommandFileBytes {
		return nil, fmt.Errorf("claim surface %q is not a bounded regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil || int64(len(raw)) != info.Size() {
		return nil, errors.Join(err, errors.New("claim surface read was incomplete"))
	}
	return raw, nil
}

func extractClaimSurfaceBlock(raw []byte, surface string) ([]byte, error) {
	begin := []byte(claimledger.SurfaceBeginMarker(surface))
	end := []byte(claimledger.SurfaceEndMarker(surface))
	if bytes.Count(raw, begin) != 1 || bytes.Count(raw, end) != 1 {
		return nil, errors.New("claim surface markers are missing or duplicated")
	}
	start := bytes.Index(raw, begin)
	finishRelative := bytes.Index(raw[start:], end)
	if finishRelative < 0 {
		return nil, errors.New("claim surface end marker precedes its begin marker")
	}
	finish := start + finishRelative + len(end)
	if finish < len(raw) && raw[finish] == '\n' {
		finish++
	}
	return raw[start:finish], nil
}
