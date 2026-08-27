package audit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EncodeCanonical renders the result as canonical JSON for CI artifacts.
func EncodeCanonical(r Result) ([]byte, error) {
	if r.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("audit encode: schema %q", r.SchemaVersion)
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("audit encode: %w", err)
	}
	return b, nil
}

// EncodeJUnit renders suite-level results as JUnit XML. Statistical route
// failures stay aggregate cases; they never claim source locations.
func EncodeJUnit(r Result) ([]byte, error) {
	if r.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("audit junit: schema %q", r.SchemaVersion)
	}
	var b strings.Builder
	failureCount := len(r.Fails)
	status := "passed"
	if !r.Pass {
		status = "failed"
	}
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&b, "<testsuite name=\"evalwitness-audit\" tests=\"%d\" failures=\"%d\">\n", 1, failureCount)
	fmt.Fprintf(&b, "  <testcase name=\"policy-%s\" status=\"%s\"", shortDigest(r.PolicyDigest), status)
	if r.Pass {
		fmt.Fprintf(&b, " />\n")
	} else {
		fmt.Fprintf(&b, ">\n    <failure message=\"policy requirements failed\">\n")
		for _, fail := range r.Fails {
			fmt.Fprintf(&b, "      %s\n", xmlEscape(fail))
		}
		fmt.Fprintf(&b, "    </failure>\n  </testcase>\n")
	}
	fmt.Fprintf(&b, "</testsuite>\n")
	return []byte(b.String()), nil
}

// EncodeMarkdown renders the human job summary.
func EncodeMarkdown(r Result) ([]byte, error) {
	var b strings.Builder
	state := "PASS"
	if !r.Pass {
		state = "FAIL"
	}
	fmt.Fprintf(&b, "# EvalWitness offline audit: %s\n\n", state)
	fmt.Fprintf(&b, "- offline: %t\n- policy digest: `%s`\n- profile digest: `%s`\n\n", r.Offline, r.PolicyDigest, r.ProfileDigest)
	if len(r.Fails) == 0 {
		b.WriteString("All declared policy requirements passed.\n")
		return []byte(b.String()), nil
	}
	b.WriteString("## Failed requirements\n\n")
	for _, fail := range r.Fails {
		fmt.Fprintf(&b, "- %s\n", fail)
	}
	if len(r.Explanations) > 0 {
		b.WriteString("\n## Explanations and reproduction\n\n")
		for _, explanation := range r.Explanations {
			fmt.Fprintf(&b, "### %s\n\n", explanation.Check)
			fmt.Fprintf(&b, "- why: %s\n", explanation.Why)
			fmt.Fprintf(&b, "- evidence: %s\n", explanation.Evidence)
			fmt.Fprintf(&b, "- remediation: %s\n", explanation.Remediation)
		}
	}
	return []byte(b.String()), nil
}

// SARIFFinding is one actionable file-local finding.
type SARIFFinding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
	CheckID string `json:"check_id"`
}

// sarifSchema is the pinned SARIF 2.1.0 schema URI.
const sarifSchema = "https://json.schemastore.org/sarif-2.1.0.json"
const sarifVersion = "2.1.0"
const sarifDriver = "evalwitness"

// ParseSARIFFinding extracts one file-local finding from a failure string of
// the shape `... file=PATH line=N ...`. Statistical failures without both
// markers return ok=false and must never become SARIF entries.
func ParseSARIFFinding(fail string) (SARIFFinding, bool) {
	fileIdx := strings.Index(fail, "file=")
	lineIdx := strings.Index(fail, "line=")
	if fileIdx < 0 || lineIdx < 0 || lineIdx < fileIdx {
		return SARIFFinding{}, false
	}
	rest := fail[fileIdx+len("file="):]
	spaceFile := strings.IndexByte(rest, ' ')
	if spaceFile < 0 {
		return SARIFFinding{}, false
	}
	file := rest[:spaceFile]
	restLine := fail[lineIdx+len("line="):]
	spaceLine := strings.IndexByte(restLine, ' ')
	lineToken := restLine
	if spaceLine >= 0 {
		lineToken = restLine[:spaceLine]
	}
	line := 0
	for _, ch := range lineToken {
		if ch < '0' || ch > '9' {
			return SARIFFinding{}, false
		}
		line = line*10 + int(ch-'0')
	}
	if line <= 0 || file == "" || !strings.ContainsAny(file, "/\\") && !strings.Contains(file, ".") {
		return SARIFFinding{}, false
	}
	checkID := "evalwitness.policy"
	message := strings.TrimSpace(strings.Join(strings.Fields(fail), " "))
	return SARIFFinding{File: file, Line: line, Message: message, CheckID: checkID}, true
}

// CollectSARIFFindings returns every parseable file-local finding; statistical
// failures are skipped, never fabricated into locations.
func CollectSARIFFindings(fails []string) []SARIFFinding {
	var out []SARIFFinding
	for _, fail := range fails {
		if finding, ok := ParseSARIFFinding(fail); ok {
			out = append(out, finding)
		}
	}
	return out
}

// HasSARIFFindings reports whether any failure carries a file-local location.
func HasSARIFFindings(fails []string) bool {
	return len(CollectSARIFFindings(fails)) > 0
}

// EncodeSARIF renders file-local findings as a SARIF 2.1.0 run. Results with no
// file-local findings produce an empty run rather than fabricated locations.
func EncodeSARIF(r Result) ([]byte, error) {
	if r.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("audit sarif: schema %q", r.SchemaVersion)
	}
	findings := CollectSARIFFindings(r.Fails)
	results := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		results = append(results, map[string]any{
			"ruleId":  f.CheckID,
			"level":   "error",
			"message": map[string]any{"text": f.Message},
			"locations": []map[string]any{{
				"physicalLocation": map[string]any{
					"artifactLocation": map[string]any{"uri": f.File},
					"region":           map[string]any{"startLine": f.Line},
				},
			}},
		})
	}
	doc := map[string]any{
		"$schema": sarifSchema,
		"version": sarifVersion,
		"runs": []map[string]any{{
			"tool": map[string]any{
				"driver": map[string]any{
					"name":           sarifDriver,
					"informationUri": "https://github.com/Christopher-Schulze/evalwitness",
				},
			},
			"results": results,
		}},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("audit sarif: %w", err)
	}
	return b, nil
}

func shortDigest(digest string) string {
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&#34;", "'", "&#39;")
	return replacer.Replace(value)
}
