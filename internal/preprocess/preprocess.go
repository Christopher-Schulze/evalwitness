package preprocess

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type redactPattern struct {
	rx          *regexp.Regexp
	replacement string
}

var redactPatterns = []redactPattern{
	{regexp.MustCompile(`(?i)sk-[a-zA-Z0-9_-]{20,}`), "[REDACTED_KEY]"},
	{regexp.MustCompile(`Bearer\s+[A-Za-z0-9._\-]+`), "Bearer [REDACTED]"},
	{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "[REDACTED_AWS]"},
	{regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`), "[REDACTED_GH_TOKEN]"},
	{regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`), "[REDACTED_SLACK]"},
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`), "[REDACTED_JWT]"},
	{regexp.MustCompile(`(?i)(["']?password["']?\s*[:=]\s*["'])[^"']+(["'])`), "${1}[REDACTED]${2}"},
}

var (
	customMu       sync.RWMutex
	customPatterns []redactPattern
)

// CustomPattern is one user-supplied redaction rule loaded from
// EVALWITNESS_REDACT_PATTERNS (JSON array of {pattern, replacement}).
type CustomPattern struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
}

// LoadCustomPatterns reads a JSON pattern file and installs the rules for all
// subsequent Redact calls. Invalid regexes fail the whole load so a typo does
// not silently drop a redaction rule.
func LoadCustomPatterns(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read redact patterns: %w", err)
	}
	var raw []CustomPattern
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse redact patterns: %w", err)
	}
	compiled := make([]redactPattern, 0, len(raw))
	for i, p := range raw {
		rx, err := regexp.Compile(p.Pattern)
		if err != nil {
			return fmt.Errorf("redact pattern %d (%q): %w", i, p.Pattern, err)
		}
		compiled = append(compiled, redactPattern{rx: rx, replacement: p.Replacement})
	}
	customMu.Lock()
	customPatterns = compiled
	customMu.Unlock()
	return nil
}

// Redact applies the secret blocklist. Returns (cleaned, hits).
func Redact(s string, enabled bool) (string, int) {
	if !enabled {
		return s, 0
	}
	out := s
	hits := 0
	for _, p := range redactPatterns {
		matches := p.rx.FindAllStringIndex(out, -1)
		hits += len(matches)
		out = p.rx.ReplaceAllString(out, p.replacement)
	}
	customMu.RLock()
	custom := customPatterns
	customMu.RUnlock()
	for _, p := range custom {
		matches := p.rx.FindAllStringIndex(out, -1)
		hits += len(matches)
		out = p.rx.ReplaceAllString(out, p.replacement)
	}
	return out, hits
}

// Hash returns a stable SHA-256 hex of the input.
func Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// EstimateTokens returns a fast char/4 token count estimate.
func EstimateTokens(s string) int {
	return len(s) / 4
}

// Step is one logical agent action extracted from a JSON trajectory.
type Step struct {
	StepID  string
	Source  string
	Message string
	Command string
	Output  string
	Raw     string // full text representation if no JSON parse possible
}

// importanceScore weights signals from the spec table.
func (s Step) importanceScore(idx, total int, prevHash string) int {
	score := 0
	text := s.Message + " " + s.Command + " " + s.Output
	lower := strings.ToLower(text)

	if s.Command != "" && (containsAny(lower, "edit ", "write ", " > ", " >> ", "tee ", "cat >") || strings.Contains(s.Command, "exec")) {
		score += 3
	}
	if s.Output != "" {
		score += 2
	}
	if containsAny(lower, "error", "exception", "traceback", "fatal", "command not found", "no such file", "segmentation fault") {
		score += 5
	}
	if matchExitCode(lower) {
		score += 5
	}
	if containsAny(lower, "pytest", "jest", "go test", "cargo test", "npm test", "mocha", "vitest") {
		score += 3
	}
	if total > 0 {
		if idx >= total-5 {
			score += 2
		}
		if idx < 3 {
			score += 1
		}
	}
	if s.Command == "" && s.Output == "" && s.Message != "" {
		score -= 1
	}
	hash := Hash(strings.TrimSpace(s.Message + s.Command + s.Output))
	if hash == prevHash && hash != "" {
		score -= 2
	}
	return score
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

var exitCodeRegex = regexp.MustCompile(`exit\s*(?:code|status)?\s*[:=]?\s*([1-9][0-9]*)`)

func matchExitCode(s string) bool {
	return exitCodeRegex.MatchString(s)
}

// terminalBenchTrajectory matches the JSON shape in eval/trajectories/terminal_trajs.
type terminalBenchTrajectory struct {
	Trajectory struct {
		Steps []terminalBenchStep `json:"steps"`
	} `json:"trajectory"`
}

type terminalBenchStep struct {
	StepID      any                     `json:"step_id"`
	Source      string                  `json:"source"`
	Message     string                  `json:"message"`
	ToolCalls   []terminalBenchToolCall `json:"tool_calls"`
	Observation struct {
		Results []struct {
			Content string `json:"content"`
		} `json:"results"`
	} `json:"observation"`
}

type terminalBenchToolCall struct {
	Arguments struct {
		Keystrokes string `json:"keystrokes"`
	} `json:"arguments"`
}

// ParseJSONTrajectory attempts to interpret s as a known JSON trajectory shape.
// Returns parsed steps and true if successful, else (nil, false).
func ParseJSONTrajectory(s string) ([]Step, bool) {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}
	var t terminalBenchTrajectory
	if err := json.Unmarshal([]byte(trimmed), &t); err != nil {
		return nil, false
	}
	if len(t.Trajectory.Steps) == 0 {
		return nil, false
	}
	out := make([]Step, 0, len(t.Trajectory.Steps))
	for _, raw := range t.Trajectory.Steps {
		if raw.Source == "system" || raw.Source == "user" {
			continue
		}
		var cmd, output string
		for _, tc := range raw.ToolCalls {
			if tc.Arguments.Keystrokes != "" {
				cmd = strings.TrimRight(tc.Arguments.Keystrokes, "\r\n")
				break
			}
		}
		for _, r := range raw.Observation.Results {
			if r.Content != "" {
				output = r.Content
				break
			}
		}
		out = append(out, Step{
			StepID:  fmt.Sprintf("%v", raw.StepID),
			Source:  raw.Source,
			Message: raw.Message,
			Command: cmd,
			Output:  output,
		})
	}
	return out, true
}

// FormatSteps renders parsed steps to the canonical text trajectory format.
func FormatSteps(steps []Step) string {
	var b strings.Builder
	for i, s := range steps {
		fmt.Fprintf(&b, "--- Agent Step %s ---\n", s.StepID)
		if s.Message != "" {
			b.WriteString(s.Message + "\n")
		}
		if s.Command != "" {
			fmt.Fprintf(&b, "[Command] %s\n", s.Command)
		}
		if s.Output != "" {
			fmt.Fprintf(&b, "[Output]\n%s\n", s.Output)
		}
		if i < len(steps)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ImportanceTruncate keeps highest-ranked steps until tokenBudget is reached.
// Returns (text, originalSteps, keptSteps, evidenceSliced).
func ImportanceTruncate(steps []Step, tokenBudget int) (string, int, int, bool) {
	if len(steps) == 0 {
		return "", 0, 0, false
	}
	steps, evidenceSliced := EvidenceSlice(steps, tokenBudget)
	type scored struct {
		idx   int
		score int
		step  Step
	}
	scoredSteps := make([]scored, len(steps))
	prevHash := ""
	for i, s := range steps {
		scoredSteps[i] = scored{
			idx:   i,
			score: s.importanceScore(i, len(steps), prevHash),
			step:  s,
		}
		prevHash = Hash(strings.TrimSpace(s.Message + s.Command + s.Output))
	}
	sort.SliceStable(scoredSteps, func(i, j int) bool {
		return scoredSteps[i].score > scoredSteps[j].score
	})
	keptIdx := map[int]bool{}
	used := 0
	for _, sc := range scoredSteps {
		txt := formatSingle(sc.step)
		t := EstimateTokens(txt)
		if used+t > tokenBudget && len(keptIdx) > 0 {
			continue
		}
		keptIdx[sc.idx] = true
		used += t
		if used >= tokenBudget {
			break
		}
	}
	var b strings.Builder
	prevKeptIdx := -1
	for i, s := range steps {
		if !keptIdx[i] {
			continue
		}
		if prevKeptIdx >= 0 && i-prevKeptIdx > 1 {
			fmt.Fprintf(&b, "[... %d steps elided ...]\n\n", i-prevKeptIdx-1)
		}
		b.WriteString(formatSingle(s))
		b.WriteString("\n")
		prevKeptIdx = i
	}
	if prevKeptIdx < len(steps)-1 && prevKeptIdx >= 0 {
		fmt.Fprintf(&b, "[... %d trailing steps elided ...]\n", len(steps)-1-prevKeptIdx)
	}
	return strings.TrimRight(b.String(), "\n"), len(steps), len(keptIdx), evidenceSliced
}

// EvidenceSlice bounds oversized individual steps before whole-step ranking.
// It preserves code-change coordinates, test outcomes, failures, exit status,
// commands, and final-state lines instead of spending the context budget on
// repeated terminal noise.
func EvidenceSlice(steps []Step, tokenBudget int) ([]Step, bool) {
	if tokenBudget <= 0 || len(steps) == 0 {
		return append([]Step(nil), steps...), false
	}
	divisor := len(steps)
	if divisor > 8 {
		divisor = 8
	}
	perStepTokens := tokenBudget / divisor
	if perStepTokens < 256 {
		perStepTokens = 256
	}
	perStepChars := perStepTokens * 4
	out := make([]Step, len(steps))
	sliced := false
	for index, step := range steps {
		out[index] = step
		messageLimit := perStepChars / 5
		if messageLimit < 512 {
			messageLimit = 512
		}
		out[index].Message = sliceEvidenceText(step.Message, messageLimit)
		commandLimit := perStepChars / 5
		if commandLimit < 512 {
			commandLimit = 512
		}
		out[index].Command = sliceEvidenceText(step.Command, commandLimit)
		reserved := len(out[index].Message) + len(out[index].Command) + 256
		outputLimit := perStepChars - reserved
		if outputLimit < 1024 {
			outputLimit = 1024
		}
		out[index].Output = sliceEvidenceText(step.Output, outputLimit)
		if out[index].Message != step.Message || out[index].Command != step.Command || out[index].Output != step.Output {
			sliced = true
		}
	}
	return out, sliced
}

type evidenceLine struct {
	index int
	score int
	size  int
}

func sliceEvidenceText(text string, maxChars int) string {
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	lines := strings.Split(text, "\n")
	candidates := make([]evidenceLine, 0, len(lines))
	for index, line := range lines {
		score := evidenceLineScore(line)
		if index < 5 {
			score += 2
		}
		if index >= len(lines)-20 {
			score += 4
		}
		candidates = append(candidates, evidenceLine{index: index, score: score, size: len(line) + 1})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].index > candidates[j].index
		}
		return candidates[i].score > candidates[j].score
	})
	selected := make(map[int]bool)
	used := 0
	for _, candidate := range candidates {
		if candidate.score <= 0 || used+candidate.size > maxChars-128 {
			continue
		}
		selected[candidate.index] = true
		used += candidate.size
	}
	indices := make([]int, 0, len(selected))
	for index := range selected {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	var output strings.Builder
	previous := -1
	for _, index := range indices {
		if previous >= 0 && index-previous > 1 {
			fmt.Fprintf(&output, "[... %d terminal lines elided ...]\n", index-previous-1)
		}
		output.WriteString(lines[index])
		output.WriteByte('\n')
		previous = index
	}
	result := strings.TrimRight(output.String(), "\n")
	if result == "" {
		return naiveTruncate(text, maxChars/4)
	}
	if len(result) > maxChars {
		return naiveTruncate(result, maxChars/4)
	}
	return result
}

func evidenceLineScore(line string) int {
	lower := strings.ToLower(strings.TrimSpace(line))
	score := 0
	if containsAny(lower,
		"error", "failed", "failure", "fatal", "panic", "exception", "traceback",
		"segmentation fault", "command not found", "no such file", "exit code", "exit status",
	) {
		score += 12
	}
	if containsAny(lower,
		"passed", "tests pass", "test result:", "go test", "cargo test", "pytest", "vitest", "jest",
	) {
		score += 10
	}
	if strings.HasPrefix(lower, "diff --git ") || strings.HasPrefix(lower, "@@ ") || strings.HasPrefix(lower, "+++ ") || strings.HasPrefix(lower, "--- ") {
		score += 9
	}
	if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
		score += 7
	}
	if containsAny(lower, "final", "summary", "modified:", "created:", "deleted:", "git status", "git diff") {
		score += 5
	}
	return score
}

func formatSingle(s Step) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- Agent Step %s ---\n", s.StepID)
	if s.Message != "" {
		b.WriteString(s.Message + "\n")
	}
	if s.Command != "" {
		fmt.Fprintf(&b, "[Command] %s\n", s.Command)
	}
	if s.Output != "" {
		fmt.Fprintf(&b, "[Output]\n%s\n", s.Output)
	}
	return b.String()
}

// Result captures the full preprocessing pipeline output.
type Result struct {
	Text            string
	Hash            string
	RedactionHits   int
	Truncated       bool
	OriginalSteps   int
	KeptSteps       int
	SourceFormat    SourceFormat
	Trajectory      Trajectory
	IngestionReport IngestionReport
	IngestionDigest string
	TraceEnvelope   *TraceEnvelope
	TraceMapping    *MappingReport
}

func (r Result) AccountingSummary() AccountingSummary {
	summary := AccountingSummary{
		SchemaVersion: "evalwitness.evidence-accounting.v1", SourceFormat: r.SourceFormat,
		SourceDigest: r.Trajectory.SourceDigest, TrajectoryDigest: r.Trajectory.Digest, IngestionDigest: r.IngestionDigest,
		SourceRecords: r.IngestionReport.SourceRecords, CanonicalEvents: r.IngestionReport.CanonicalEvents,
		RetainedEvents: len(r.Trajectory.Events), OriginalBytes: r.IngestionReport.OriginalBytes,
		RetainedBytes: r.IngestionReport.RetainedBytes, UnsupportedRecords: r.IngestionReport.UnsupportedRecords,
		ParseErrors: r.IngestionReport.ParseErrors, UnpairedToolCalls: r.IngestionReport.UnpairedToolCalls,
		UnpairedToolResults: r.IngestionReport.UnpairedToolResults, RedactionHits: r.RedactionHits,
		Truncation: r.IngestionReport.Truncation,
	}
	if r.TraceEnvelope != nil {
		summary.TraceEnvelopeDigest = r.TraceEnvelope.Digest
		summary.TraceMappingPolicy = r.TraceEnvelope.MappingPolicyVersion
	}
	if r.TraceMapping != nil {
		summary.TraceMappingDigest = r.TraceMapping.Digest
	}
	return summary
}

func AccountingSummaries(results ...Result) []AccountingSummary {
	summaries := make([]AccountingSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.AccountingSummary())
	}
	return summaries
}

func AggregateAccounting(results ...Result) AccountingAggregate {
	aggregate := AccountingAggregate{SchemaVersion: "evalwitness.evidence-accounting-aggregate.v1"}
	formats := make(map[SourceFormat]int)
	for _, result := range results {
		summary := result.AccountingSummary()
		aggregate.Trajectories++
		formats[summary.SourceFormat]++
		aggregate.SourceRecords += summary.SourceRecords
		aggregate.CanonicalEvents += summary.CanonicalEvents
		aggregate.RetainedEvents += summary.RetainedEvents
		aggregate.OriginalBytes += summary.OriginalBytes
		aggregate.RetainedBytes += summary.RetainedBytes
		aggregate.UnsupportedRecords += summary.UnsupportedRecords
		aggregate.UnpairedToolCalls += summary.UnpairedToolCalls
		aggregate.UnpairedToolResults += summary.UnpairedToolResults
		aggregate.RedactionHits += summary.RedactionHits
		if summary.Truncation.Applied {
			aggregate.TruncatedTrajectories++
		}
	}
	formatNames := make([]string, 0, len(formats))
	for format := range formats {
		formatNames = append(formatNames, string(format))
	}
	sort.Strings(formatNames)
	for _, name := range formatNames {
		format := SourceFormat(name)
		aggregate.Formats = append(aggregate.Formats, FormatAccounting{SourceFormat: format, Trajectories: formats[format]})
	}
	return aggregate
}

// CanonicalPipeline is the strict research ingestion path. It parses every
// supported source into the versioned event model, redacts before selection,
// applies a hard evidence budget, and returns the exact loss report used to
// render verifier evidence.
func CanonicalPipeline(raw string, redact bool, tokenBudget int) (Result, error) {
	options := DefaultIngestOptions()
	options.Redact = redact
	privacy := PrivacyMetadataOnly
	if !redact {
		privacy = PrivacyFull
	}
	traceResult, err := ImportTraceBytes([]byte(raw), TraceImportOptions{Ingest: options, Privacy: privacy})
	if err != nil {
		return Result{}, err
	}
	trajectory := traceResult.Trajectory
	originalEvents := len(trajectory.Events)
	retained, err := ApplyEvidenceBudget(trajectory, tokenBudget)
	if err != nil {
		return Result{}, err
	}
	result, err := canonicalResult(retained, originalEvents)
	if err != nil {
		return Result{}, err
	}
	result.TraceEnvelope = &traceResult.Envelope
	result.TraceMapping = &traceResult.Mapping
	if retained.SourceFormat == SourcePlainText {
		return result, nil
	}
	result.Text = accountingHeader(result.AccountingSummary()) + "\n\n" + result.Text
	result.Hash = Hash(result.Text)
	if tokenBudget <= 0 || estimateTokensForBytes(len(result.Text)) <= tokenBudget {
		return result, nil
	}
	headerTokens := estimateTokensForBytes(len(accountingHeader(result.AccountingSummary()) + "\n\n"))
	if headerTokens >= tokenBudget {
		return Result{}, fmt.Errorf("evidence accounting header requires %d tokens over %d-token budget", headerTokens, tokenBudget)
	}
	retained, err = ApplyEvidenceBudget(trajectory, tokenBudget-headerTokens)
	if err != nil {
		return Result{}, err
	}
	result, err = canonicalResult(retained, originalEvents)
	if err != nil {
		return Result{}, err
	}
	result.TraceEnvelope = &traceResult.Envelope
	result.TraceMapping = &traceResult.Mapping
	result.Text = accountingHeader(result.AccountingSummary()) + "\n\n" + result.Text
	result.Hash = Hash(result.Text)
	if estimateTokensForBytes(len(result.Text)) > tokenBudget {
		return Result{}, fmt.Errorf("accounted evidence exceeds %d-token budget", tokenBudget)
	}
	return result, nil
}

func canonicalResult(retained Trajectory, originalEvents int) (Result, error) {
	text := RenderTrajectory(retained)
	reportJSON, err := json.Marshal(retained.Report)
	if err != nil {
		return Result{}, fmt.Errorf("encode ingestion report: %w", err)
	}
	return Result{
		Text:            text,
		Hash:            Hash(text),
		RedactionHits:   retained.Report.RedactionHits,
		Truncated:       retained.Report.Truncation.Applied,
		OriginalSteps:   originalEvents,
		KeptSteps:       len(retained.Events),
		SourceFormat:    retained.SourceFormat,
		Trajectory:      retained,
		IngestionReport: retained.Report,
		IngestionDigest: Hash(string(reportJSON)),
	}, nil
}

func accountingHeader(summary AccountingSummary) string {
	return fmt.Sprintf(
		"[Evidence accounting schema=%s source=%s trajectory=%s report=%s records=%d events=%d/%d bytes=%d/%d redactions=%d unsupported=%d unpaired_calls=%d unpaired_results=%d truncated=%t budget=%d retained_tokens=%d]",
		summary.SchemaVersion, summary.SourceFormat, summary.TrajectoryDigest, summary.IngestionDigest,
		summary.SourceRecords, summary.RetainedEvents, summary.CanonicalEvents,
		summary.RetainedBytes, summary.OriginalBytes, summary.RedactionHits, summary.UnsupportedRecords,
		summary.UnpairedToolCalls, summary.UnpairedToolResults, summary.Truncation.Applied,
		summary.Truncation.BudgetTokens, summary.Truncation.RetainedTokens,
	)
}

// Pipeline applies redact -> hash on a raw text trajectory; if a known JSON
// trajectory or agent-session transcript is detected (terminal-bench JSON,
// Claude Code / Codex session JSONL, OpenCode export), it is formatted to the
// canonical step layout; importance-truncation applies when a budget is given.
func Pipeline(raw string, redact bool, tokenBudget int) Result {
	steps, ok := ParseJSONTrajectory(raw)
	if !ok {
		steps, ok = ParseAgentSession(raw)
	}
	if ok {
		if tokenBudget > 0 {
			text, total, kept, evidenceSliced := ImportanceTruncate(steps, tokenBudget)
			text, hits := Redact(text, redact)
			return Result{
				Text:          text,
				Hash:          Hash(text),
				RedactionHits: hits,
				Truncated:     kept < total || evidenceSliced,
				OriginalSteps: total,
				KeptSteps:     kept,
			}
		}
		text := FormatSteps(steps)
		text, hits := Redact(text, redact)
		return Result{
			Text:          text,
			Hash:          Hash(text),
			RedactionHits: hits,
			OriginalSteps: len(steps),
			KeptSteps:     len(steps),
		}
	}
	text, hits := Redact(raw, redact)
	if tokenBudget > 0 && EstimateTokens(text) > tokenBudget {
		text = naiveTruncate(text, tokenBudget)
	}
	return Result{
		Text:          text,
		Hash:          Hash(text),
		RedactionHits: hits,
	}
}

func naiveTruncate(s string, tokenBudget int) string {
	limit := tokenBudget * 4
	if len(s) <= limit {
		return s
	}
	headEnd := limit / 5
	tailStart := len(s) - (3 * limit / 5)
	if tailStart <= headEnd {
		return s[:limit]
	}
	return s[:headEnd] + "\n\n[... text elided to fit context ...]\n\n" + s[tailStart:]
}
