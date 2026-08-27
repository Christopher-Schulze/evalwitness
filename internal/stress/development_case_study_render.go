package stress

import (
	"fmt"
	"html"
	"strings"
)

func RenderDevelopmentCaseStudyMarkdown(value DevelopmentCaseStudy) (string, error) {
	if err := value.Validate(); err != nil {
		return "", err
	}
	var output strings.Builder
	sections := []string{
		"# Candidate-order failure reduced to a two-line witness\n\n",
		"> Provider-free development mechanism evidence. This is not a verifier reliability result.\n\n",
		"## Result\n\n",
		"| Evidence | Observed value |\n|---|---|\n",
		fmt.Sprintf("| Relation | `%s` |\n", value.Relation.ID),
		fmt.Sprintf("| Negative control | `%s` |\n", value.Observation.ControlArmID),
		fmt.Sprintf("| Original selection | `%s` |\n", value.Observation.OriginalSelected),
		fmt.Sprintf("| Reversed selection | `%s` |\n", value.Observation.TransformedSelected),
		fmt.Sprintf("| Relation outcome | `%s` |\n", value.Observation.Outcome),
		fmt.Sprintf("| Reduction | %d fixture lines to %d, %.2f%% removed |\n", value.OriginalLineUnits, value.FinalLineUnits, value.ReductionPercent),
		fmt.Sprintf("| Minimality | `%s` |\n", value.Counterexample.Minimality),
		fmt.Sprintf("| Provider calls | %d |\n", value.ProviderCalls),
		fmt.Sprintf("| Network required | `%t` |\n", value.NetworkRequired),
		fmt.Sprintf("| Empirical units | %d |\n", value.EmpiricalUnits),
		fmt.Sprintf("| Case-study digest | `%s` |\n\n", value.Digest),
		"```mermaid\nflowchart LR\n  A[32 checked-in fixture lines] --> B[Reverse candidate order]\n  B --> C[First-listed selects another trajectory]\n  C --> D[Invariance relation violated]\n  D --> E[Shared restart-greedy reducer]\n  E --> F[Two-line one-minimal witness]\n```\n\n",
		"## Final witness\n\n",
		"| Trajectory | Repository source | Retained content |\n|---|---|---|\n",
	}
	for _, section := range sections {
		if _, err := output.WriteString(section); err != nil {
			return "", err
		}
	}
	for _, line := range value.FinalWitness {
		row := fmt.Sprintf("| `%s` | `%s:%d` | `%s` |\n", line.UnitID, line.FixturePath, line.Line, escapeMarkdownTable(line.Content))
		if _, err := output.WriteString(row); err != nil {
			return "", err
		}
	}
	if _, err := output.WriteString("\nThe task requires output `5`. The retained fixture outputs `5` and `-1`, so the two candidates remain distinct while the first-listed control changes identity after reversal. Every removed line was replayed through the same relation, privacy, and violation oracle. The final rejection pass proves deterministic one-minimality over declared line units, not a global minimum.\n\n## Claim boundary\n\n| Status | Claim |\n|---|---|\n"); err != nil {
		return "", err
	}
	for _, claim := range value.AllowedClaims {
		if _, err := fmt.Fprintf(&output, "| allowed | `%s` |\n", claim); err != nil {
			return "", err
		}
	}
	for _, claim := range value.ForbiddenClaims {
		if _, err := fmt.Fprintf(&output, "| forbidden | `%s` |\n", claim); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprintf(&output, "\n%s\n\n## Reproduce\n\n```bash\n./evalwitness stress development-case-study --repository-root . --format json\n./evalwitness stress development-case-study --repository-root . --format markdown\n./evalwitness stress development-case-study --repository-root . --format svg > /tmp/evalwitness-stress-certificate.svg\n./evalwitness stress validate --type development-case-study --repository-root . --document @eval/results/stress-development-case-study-v1.json\n```\n", value.ClaimBoundary); err != nil {
		return "", err
	}
	return output.String(), nil
}

func RenderDevelopmentCaseStudySVG(value DevelopmentCaseStudy) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	var output strings.Builder
	if err := writeDevelopmentSVGHeader(&output, value); err != nil {
		return nil, err
	}
	if err := writeDevelopmentSVGOrderComparison(&output, value); err != nil {
		return nil, err
	}
	if err := writeDevelopmentSVGReduction(&output, value); err != nil {
		return nil, err
	}
	if err := writeDevelopmentSVGWitness(&output, value); err != nil {
		return nil, err
	}
	if err := appendDevelopmentSVG(&output, fmt.Sprintf(`<text class="boundary" x="64" y="846">%s</text><text class="foot" x="64" y="878">case %s · canonical JSON is the scientific source · this SVG is a deterministic projection</text></svg>`, html.EscapeString(value.ClaimBoundary), value.Digest[:12]), "\n"); err != nil {
		return nil, err
	}
	return []byte(output.String()), nil
}

func writeDevelopmentSVGHeader(output *strings.Builder, value DevelopmentCaseStudy) error {
	return appendDevelopmentSVG(output,
		`<svg xmlns="http://www.w3.org/2000/svg" width="1400" height="900" viewBox="0 0 1400 900" role="img" aria-labelledby="title desc">`,
		`<title id="title">Candidate-order violation reduced to a two-line witness</title>`,
		fmt.Sprintf(`<desc id="desc">A provider-free first-listed negative control selects %s in original order and %s after reversal. A deterministic 53-attempt replay reduces 32 fixture lines to two and proves one-minimality. Zero empirical units.</desc>`, html.EscapeString(value.Observation.OriginalSelected), html.EscapeString(value.Observation.TransformedSelected)),
		`<style>.bg{fill:#0b0f14}.panel{fill:#121820;stroke:#2a3440;stroke-width:1}.panel-strong{fill:#171f29;stroke:#36424f;stroke-width:1}.mono{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}.display{font-family:Inter,ui-sans-serif,system-ui,sans-serif}.eyebrow{font:600 13px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:1.8px;fill:#8f9aa6}.title{font:700 42px Inter,ui-sans-serif,system-ui,sans-serif;fill:#f4f7f9}.subtitle{font:400 17px Inter,ui-sans-serif,system-ui,sans-serif;fill:#9da7b2}.section{font:600 14px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:1.4px;fill:#c8d0d8}.label{font:500 13px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;fill:#8f9aa6}.value{font:650 18px Inter,ui-sans-serif,system-ui,sans-serif;fill:#f4f7f9}.metric{font:750 72px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;fill:#f4f7f9}.metric-red{fill:#ff5a56}.blue{fill:#2f6cf6}.red{fill:#ff5a56}.cold{fill:#f4f7f9}.muted{fill:#8f9aa6}.rule{stroke:#2a3440;stroke-width:1}.boundary{font:500 13px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;fill:#f4a7a4}.foot{font:400 12px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;fill:#6f7a86}</style>`,
		`<rect class="bg" width="1400" height="900"/>`,
		`<text class="eyebrow" x="64" y="54">EVALWITNESS / REDUCTION CERTIFICATE / DEVELOPMENT</text>`,
		`<text class="title" x="64" y="110">Candidate-order violation, reduced</text>`,
		`<text class="subtitle" x="64" y="145">One canonical relation. One replay-preserving reducer. One inspectable witness.</text>`,
		`<rect x="1165" y="43" width="171" height="34" rx="17" fill="#2a171a" stroke="#ff5a56"/>`,
		`<text x="1250" y="65" text-anchor="middle" class="mono red" font-size="12" font-weight="700" letter-spacing="1.2">MECHANISM ONLY</text>`,
	)
}

func writeDevelopmentSVGOrderComparison(output *strings.Builder, value DevelopmentCaseStudy) error {
	originalOrder := html.EscapeString(strings.Join(value.Observation.OriginalOrder, "  /  "))
	reversedOrder := html.EscapeString(strings.Join(value.Observation.TransformedOrder, "  /  "))
	return appendDevelopmentSVG(output,
		`<rect class="panel" x="64" y="190" width="392" height="198" rx="12"/>`,
		`<text class="section" x="88" y="224">ORIGINAL ORDER</text>`,
		fmt.Sprintf(`<text class="label" x="88" y="260">%s</text>`, originalOrder),
		fmt.Sprintf(`<text class="value" x="88" y="306">selected  <tspan class="blue">%s</tspan></text>`, html.EscapeString(value.Observation.OriginalSelected)),
		`<text class="label" x="88" y="354">first-listed control</text>`,
		`<rect class="panel" x="480" y="190" width="392" height="198" rx="12"/>`,
		`<text class="section" x="504" y="224">REVERSED ORDER</text>`,
		fmt.Sprintf(`<text class="label" x="504" y="260">%s</text>`, reversedOrder),
		fmt.Sprintf(`<text class="value" x="504" y="306">selected  <tspan class="red">%s</tspan></text>`, html.EscapeString(value.Observation.TransformedSelected)),
		fmt.Sprintf(`<text class="label" x="504" y="354">relation outcome  <tspan class="red">%s</tspan></text>`, html.EscapeString(string(value.Observation.Outcome))),
		`<rect class="panel-strong" x="896" y="190" width="440" height="198" rx="12"/>`,
		`<text class="section" x="920" y="224">REDUCTION</text>`,
		fmt.Sprintf(`<text class="metric" x="920" y="312">%d <tspan class="metric-red">-&gt;</tspan> %d</text>`, value.OriginalLineUnits, value.FinalLineUnits),
		fmt.Sprintf(`<text class="label" x="920" y="354">fixture line units · %.2f%% removed</text>`, value.ReductionPercent),
	)
}

func writeDevelopmentSVGReduction(output *strings.Builder, value DevelopmentCaseStudy) error {
	rejected := len(value.Counterexample.Steps) - value.AcceptedReductions
	if err := appendDevelopmentSVG(output,
		`<text class="section" x="64" y="448">REPLAY TRACE</text>`,
		fmt.Sprintf(`<text class="label" x="210" y="448">%d attempts · %d accepted removals · %d rejected removals</text>`, len(value.Counterexample.Steps), value.AcceptedReductions, rejected),
		fmt.Sprintf(`<text class="label" x="1336" y="448" text-anchor="end">%s · final rejection pass %d/%d</text>`, html.EscapeString(string(value.Counterexample.Minimality)), len(value.Counterexample.FinalUnits), len(value.Counterexample.FinalUnits)),
		`<line class="rule" x1="64" y1="472" x2="1336" y2="472"/>`,
	); err != nil {
		return err
	}
	for index, step := range value.Counterexample.Steps {
		x := 70 + index*24
		class, y, height := "blue", 493, 27
		if step.Decision == ReductionRejected {
			class, y, height = "red", 483, 37
		}
		mark := fmt.Sprintf(`<rect class="%s" x="%d" y="%d" width="14" height="%d" rx="2"/>`, class, x, y, height)
		if err := appendDevelopmentSVG(output, mark); err != nil {
			return err
		}
	}
	return appendDevelopmentSVG(output,
		`<line class="rule" x1="64" y1="532" x2="1336" y2="532"/>`,
		`<circle class="blue" cx="72" cy="559" r="5"/><text class="label" x="86" y="564">accepted: relation + privacy + violation preserved</text>`,
		`<circle class="red" cx="530" cy="559" r="5"/><text class="label" x="544" y="564">rejected: removing a retained unit breaks the proof</text>`,
	)
}

func writeDevelopmentSVGWitness(output *strings.Builder, value DevelopmentCaseStudy) error {
	left, right := value.FinalWitness[0], value.FinalWitness[1]
	return appendDevelopmentSVG(output,
		`<text class="section" x="64" y="616">FINAL WITNESS</text>`,
		fmt.Sprintf(`<text class="label" x="1336" y="616" text-anchor="end">task-required output  %s</text>`, html.EscapeString(left.Content)),
		`<rect class="panel-strong" x="64" y="640" width="612" height="142" rx="12"/>`,
		fmt.Sprintf(`<text class="metric blue" x="88" y="716">%s</text>`, html.EscapeString(left.Content)),
		fmt.Sprintf(`<text class="value" x="190" y="690">%s</text>`, html.EscapeString(left.UnitID)),
		fmt.Sprintf(`<text class="label" x="190" y="724">%s:%d</text>`, html.EscapeString(left.FixturePath), left.Line),
		`<text class="label" x="190" y="754">retained after final attempted removal</text>`,
		`<rect class="panel-strong" x="700" y="640" width="636" height="142" rx="12"/>`,
		fmt.Sprintf(`<text class="metric red" x="724" y="716">%s</text>`, html.EscapeString(right.Content)),
		fmt.Sprintf(`<text class="value" x="850" y="690">%s</text>`, html.EscapeString(right.UnitID)),
		fmt.Sprintf(`<text class="label" x="850" y="724">%s:%d</text>`, html.EscapeString(right.FixturePath), right.Line),
		`<text class="label" x="850" y="754">retained after final attempted removal</text>`,
		`<line class="rule" x1="64" y1="816" x2="1336" y2="816"/>`,
	)
}

func appendDevelopmentSVG(output *strings.Builder, segments ...string) error {
	for _, segment := range segments {
		if _, err := output.WriteString(segment); err != nil {
			return err
		}
	}
	return nil
}

func escapeMarkdownTable(value string) string {
	return strings.NewReplacer("\\", "\\\\", "|", "\\|", "`", "\\`").Replace(value)
}
