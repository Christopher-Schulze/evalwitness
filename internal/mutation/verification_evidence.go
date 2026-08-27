package mutation

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	VerificationEvidenceAssessmentSchemaVersion = "evalwitness.verification-evidence-assessment.v1"
	VerificationEvidenceClassifierVersion       = "evalwitness.verification-evidence-classifier.v1"
)

type VerificationEvidenceStatus string

const (
	VerificationEvidenceEligible VerificationEvidenceStatus = "eligible"
	VerificationEvidenceRejected VerificationEvidenceStatus = "rejected"
)

type VerificationEvidenceRejectionReason string

const (
	VerificationEvidenceNoTarget             VerificationEvidenceRejectionReason = "no_applicable_target"
	VerificationEvidenceInvocationUnverified VerificationEvidenceRejectionReason = "verification_invocation_unverified"
	VerificationEvidenceProvenanceUnbound    VerificationEvidenceRejectionReason = "result_provenance_unbound"
	VerificationEvidenceNonFailable          VerificationEvidenceRejectionReason = "non_failable_verification"
	VerificationEvidenceNotWeakened          VerificationEvidenceRejectionReason = "claim_specific_evidence_not_weakened"
)

type VerificationEvidenceContentKind string

const (
	VerificationContentExecutionOutput VerificationEvidenceContentKind = "verification_execution_output"
	VerificationContentCommandMarker   VerificationEvidenceContentKind = "command_emitted_success_marker"
	VerificationContentMixedNarration  VerificationEvidenceContentKind = "mixed_agent_narration"
	VerificationContentUnbound         VerificationEvidenceContentKind = "unbound_result_text"
	VerificationContentAbsent          VerificationEvidenceContentKind = "absent"
)

type VerificationEvidenceAssessment struct {
	SchemaVersion             string                                `json:"schema_version"`
	CanonicalPolicy           string                                `json:"canonical_policy"`
	ClassifierVersion         string                                `json:"classifier_version"`
	SourceFormat              preprocess.SourceFormat               `json:"source_format"`
	SourceTrajectoryDigest    string                                `json:"source_trajectory_digest"`
	TargetEventID             string                                `json:"target_event_id"`
	Status                    VerificationEvidenceStatus            `json:"status"`
	SemanticRole              string                                `json:"semantic_role,omitempty"`
	Invocation                InvocationProof                       `json:"invocation"`
	ResultChannel             string                                `json:"result_channel"`
	ResultStatus              string                                `json:"result_status,omitempty"`
	ResultError               bool                                  `json:"result_error"`
	ContentKind               VerificationEvidenceContentKind       `json:"content_kind"`
	ContentDigest             string                                `json:"content_digest"`
	ContentBytes              int                                   `json:"content_bytes"`
	ProvenanceBound           bool                                  `json:"provenance_bound"`
	VerificationFailable      bool                                  `json:"verification_failable"`
	RemainingDecisiveChannels []string                              `json:"remaining_decisive_channels"`
	EvidenceWeakened          bool                                  `json:"evidence_weakened"`
	Checks                    []Check                               `json:"checks"`
	RejectionReasons          []VerificationEvidenceRejectionReason `json:"rejection_reasons"`
	Digest                    string                                `json:"digest"`
}

func AssessVerificationEvidence(trajectory preprocess.Trajectory, targetEventID string) (VerificationEvidenceAssessment, error) {
	if err := trajectory.Validate(); err != nil {
		return VerificationEvidenceAssessment{}, fmt.Errorf("validate verification-evidence source: %w", err)
	}
	target, exists := trajectoryEventByID(trajectory, targetEventID)
	if !exists || strings.TrimSpace(eventEvidenceText(target)) == "" {
		proof := InvocationProof{
			ParserVersion: InvocationParserVersion, EvidenceEventID: targetEventID, SegmentIndex: 0,
			Executable: "<missing-evidence-target>", Arguments: []string{}, WrapperChain: []string{},
			ParseStatus: InvocationRejected, Decision: "missing_evidence_target",
		}
		return sealVerificationEvidenceAssessment(VerificationEvidenceAssessment{
			SourceFormat: trajectory.SourceFormat, SourceTrajectoryDigest: trajectory.Digest,
			TargetEventID: targetEventID, Invocation: proof, ResultChannel: "none",
			ContentKind: VerificationContentAbsent, ContentDigest: digestText(""),
			RejectionReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNoTarget},
		})
	}

	content := eventEvidenceText(target)
	proof, role, direct := verifiedExecutionInvocation(trajectory, target)
	failable := direct && verificationInvocationFailable(proof)
	contentKind, provenanceBound := classifyVerificationContent(trajectory, target, proof, role, content)
	remaining := remainingVerificationChannels(trajectory, target, proof)
	evidenceWeakened := direct && failable && provenanceBound && len(remaining) == 0

	return sealVerificationEvidenceAssessment(VerificationEvidenceAssessment{
		SourceFormat: trajectory.SourceFormat, SourceTrajectoryDigest: trajectory.Digest,
		TargetEventID: targetEventID, SemanticRole: role, Invocation: proof,
		ResultChannel: verificationResultChannel(target), ResultStatus: verificationResultStatus(target),
		ResultError: verificationResultError(target), ContentKind: contentKind,
		ContentDigest: digestText(content), ContentBytes: len(content), ProvenanceBound: provenanceBound,
		VerificationFailable: failable, RemainingDecisiveChannels: remaining,
		EvidenceWeakened: evidenceWeakened,
	})
}

func sealVerificationEvidenceAssessment(assessment VerificationEvidenceAssessment) (VerificationEvidenceAssessment, error) {
	assessment.SchemaVersion = VerificationEvidenceAssessmentSchemaVersion
	assessment.CanonicalPolicy = CanonicalPolicy
	assessment.ClassifierVersion = VerificationEvidenceClassifierVersion
	assessment.RemainingDecisiveChannels = sortedStrings(assessment.RemainingDecisiveChannels)
	assessment.RejectionReasons = expectedVerificationEvidenceReasons(assessment)
	sort.Slice(assessment.RejectionReasons, func(left, right int) bool {
		return assessment.RejectionReasons[left] < assessment.RejectionReasons[right]
	})
	assessment.Status = VerificationEvidenceEligible
	if len(assessment.RejectionReasons) > 0 {
		assessment.Status = VerificationEvidenceRejected
	}
	assessment.Checks = verificationEvidenceChecks(assessment)
	assessment.Digest = ""
	digest, err := assessment.digest()
	if err != nil {
		return VerificationEvidenceAssessment{}, err
	}
	assessment.Digest = digest
	if err := assessment.Validate(); err != nil {
		return VerificationEvidenceAssessment{}, err
	}
	return assessment, nil
}

func (assessment VerificationEvidenceAssessment) Validate() error {
	if assessment.SchemaVersion != VerificationEvidenceAssessmentSchemaVersion || assessment.CanonicalPolicy != CanonicalPolicy || assessment.ClassifierVersion != VerificationEvidenceClassifierVersion {
		return errors.New("verification-evidence assessment identity is invalid")
	}
	if !validSourceFormat(assessment.SourceFormat) || !validDigest(assessment.SourceTrajectoryDigest) || strings.TrimSpace(assessment.TargetEventID) == "" || !validDigest(assessment.ContentDigest) || assessment.ContentBytes < 0 {
		return errors.New("verification-evidence assessment source, target, or content binding is invalid")
	}
	if !slices.Contains([]VerificationEvidenceStatus{VerificationEvidenceEligible, VerificationEvidenceRejected}, assessment.Status) {
		return errors.New("verification-evidence assessment status is invalid")
	}
	if !slices.Contains([]VerificationEvidenceContentKind{VerificationContentExecutionOutput, VerificationContentCommandMarker, VerificationContentMixedNarration, VerificationContentUnbound, VerificationContentAbsent}, assessment.ContentKind) {
		return errors.New("verification-evidence content kind is invalid")
	}
	if assessment.Invocation.ParserVersion != InvocationParserVersion || assessment.Invocation.EvidenceEventID != assessment.TargetEventID || assessment.Invocation.SegmentIndex < 0 || strings.TrimSpace(assessment.Invocation.Executable) == "" || strings.TrimSpace(assessment.Invocation.Decision) == "" {
		return errors.New("verification-evidence invocation proof is invalid")
	}
	if err := validateUniqueSortedStrings("remaining decisive channels", assessment.RemainingDecisiveChannels); err != nil {
		return err
	}
	if err := validateVerificationEvidenceReasons(assessment.RejectionReasons); err != nil {
		return err
	}
	if !slices.Equal(assessment.RejectionReasons, expectedVerificationEvidenceReasons(assessment)) {
		return errors.New("verification-evidence rejection reasons do not match the typed assessment")
	}
	if !slices.Equal(assessment.Checks, verificationEvidenceChecks(assessment)) {
		return errors.New("verification-evidence checks do not match the typed assessment")
	}
	if assessment.Status == VerificationEvidenceEligible {
		if len(assessment.RejectionReasons) != 0 || !assessment.Invocation.DirectInvocation || !assessment.ProvenanceBound || !assessment.VerificationFailable || !assessment.EvidenceWeakened {
			return errors.New("eligible verification evidence is not fully proven")
		}
	} else if len(assessment.RejectionReasons) == 0 {
		return errors.New("rejected verification evidence lacks a closed reason")
	}
	expected, err := assessment.digest()
	if err != nil {
		return err
	}
	if assessment.Digest != expected {
		return errors.New("verification-evidence assessment digest is invalid")
	}
	return nil
}

func (assessment VerificationEvidenceAssessment) digest() (string, error) {
	assessment.Digest = ""
	return digestJSON(assessment)
}

func verificationEvidenceChecks(assessment VerificationEvidenceAssessment) []Check {
	return []Check{
		{Name: "direct_verification_invocation", Expected: "proven", Observed: describeInvocation(assessment.Invocation), Passed: assessment.Invocation.DirectInvocation},
		{Name: "result_provenance_bound", Expected: "bound", Observed: string(assessment.ContentKind), Passed: assessment.ProvenanceBound},
		{Name: "verification_failable", Expected: "failable", Observed: fmt.Sprintf("failable=%t", assessment.VerificationFailable), Passed: assessment.VerificationFailable},
		{Name: "claim_specific_evidence_loss", Expected: "weakened with no equivalent decisive channel", Observed: fmt.Sprintf("weakened=%t remaining=%s", assessment.EvidenceWeakened, strings.Join(assessment.RemainingDecisiveChannels, ",")), Passed: assessment.EvidenceWeakened},
	}
}

func validateVerificationEvidenceReasons(reasons []VerificationEvidenceRejectionReason) error {
	if !slices.IsSorted(reasons) {
		return errors.New("verification-evidence rejection reasons must be sorted")
	}
	allowed := []VerificationEvidenceRejectionReason{
		VerificationEvidenceNoTarget,
		VerificationEvidenceInvocationUnverified,
		VerificationEvidenceProvenanceUnbound,
		VerificationEvidenceNonFailable,
		VerificationEvidenceNotWeakened,
	}
	seen := make(map[VerificationEvidenceRejectionReason]struct{}, len(reasons))
	for _, reason := range reasons {
		if !slices.Contains(allowed, reason) {
			return fmt.Errorf("unsupported verification-evidence rejection reason %q", reason)
		}
		if _, duplicate := seen[reason]; duplicate {
			return fmt.Errorf("duplicate verification-evidence rejection reason %q", reason)
		}
		seen[reason] = struct{}{}
	}
	return nil
}

func expectedVerificationEvidenceReasons(assessment VerificationEvidenceAssessment) []VerificationEvidenceRejectionReason {
	if assessment.Invocation.Decision == "missing_evidence_target" {
		return []VerificationEvidenceRejectionReason{VerificationEvidenceNoTarget}
	}
	reasons := make([]VerificationEvidenceRejectionReason, 0, 4)
	if !assessment.Invocation.DirectInvocation {
		reasons = append(reasons, VerificationEvidenceInvocationUnverified)
	}
	if !assessment.ProvenanceBound {
		reasons = append(reasons, VerificationEvidenceProvenanceUnbound)
	}
	if assessment.Invocation.DirectInvocation && !assessment.VerificationFailable {
		reasons = append(reasons, VerificationEvidenceNonFailable)
	}
	if !assessment.EvidenceWeakened {
		reasons = append(reasons, VerificationEvidenceNotWeakened)
	}
	sort.Slice(reasons, func(left, right int) bool {
		return reasons[left] < reasons[right]
	})
	return reasons
}

func verificationInvocationFailable(proof InvocationProof) bool {
	if !proof.DirectInvocation {
		return false
	}
	switch proof.Executable {
	case "diff", "cmp":
		operands, valid := comparisonOperands(proof.Executable, proof.Arguments)
		return valid && filepath.Clean(operands[0]) != filepath.Clean(operands[1])
	case "sha256sum", "shasum":
		return checksumCheckInput(proof.Arguments)
	case "assert":
		return false
	default:
		return proof.SemanticRole == "test" || proof.SemanticRole == "check" || proof.SemanticRole == "build"
	}
}

func comparisonOperands(executable string, arguments []string) ([2]string, bool) {
	var result [2]string
	operands := make([]string, 0, 2)
	consumeNext := false
	optionsDone := false
	for _, argument := range arguments {
		if consumeNext {
			consumeNext = false
			continue
		}
		if !optionsDone && argument == "--" {
			optionsDone = true
			continue
		}
		if !optionsDone && strings.HasPrefix(argument, "-") && argument != "-" {
			if comparisonOptionConsumesValue(executable, argument) {
				consumeNext = true
			}
			continue
		}
		operands = append(operands, argument)
	}
	if consumeNext || len(operands) != 2 || strings.TrimSpace(operands[0]) == "" || strings.TrimSpace(operands[1]) == "" {
		return result, false
	}
	copy(result[:], operands)
	return result, true
}

func comparisonOptionConsumesValue(executable, argument string) bool {
	if strings.Contains(argument, "=") {
		return false
	}
	if executable == "diff" {
		return slices.Contains([]string{"-C", "-D", "-I", "-L", "-U", "--context", "--ifdef", "--ignore-matching-lines", "--label", "--unified"}, argument)
	}
	return slices.Contains([]string{"-i", "-n", "--bytes", "--ignore-initial"}, argument)
}

func checksumCheckInput(arguments []string) bool {
	checkMode := false
	input := false
	for _, argument := range arguments {
		if argument == "-c" || argument == "--check" {
			checkMode = true
			continue
		}
		if !strings.HasPrefix(argument, "-") || argument == "-" {
			input = true
		}
	}
	return checkMode && input
}

func classifyVerificationContent(trajectory preprocess.Trajectory, target preprocess.Event, proof InvocationProof, role, content string) (VerificationEvidenceContentKind, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return VerificationContentAbsent, false
	}
	if containsAgentNarration(trimmed) {
		return VerificationContentMixedNarration, false
	}
	command, exists := trajectoryEventByID(trajectory, proof.CommandEventID)
	if exists && command.Command != nil {
		if marker, found := commandSuccessMarker(command.Command.Display, proof.SegmentIndex); found && strings.TrimSpace(marker) == trimmed {
			return VerificationContentCommandMarker, true
		}
	}
	if recognizedVerificationOutput(proof.Executable, role, trimmed) {
		return VerificationContentExecutionOutput, true
	}
	return VerificationContentUnbound, false
}

func containsAgentNarration(content string) bool {
	lower := strings.ToLower(content)
	markers := []string{
		"\ni need ", "\ni should ", "\ni want ", "\ni'll ", "\ni will ", "\ni’m ", "\ni'm ",
		"i need to ", "i should ", "i want to ", "it seems like ", "before i can ", "time to ",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func recognizedVerificationOutput(executable, role, content string) bool {
	lower := strings.ToLower(content)
	switch executable {
	case "go":
		return hasLinePrefix(content, "ok ", "?   ", "FAIL", "PASS", "=== RUN", "--- PASS", "--- FAIL")
	case "cargo":
		return strings.Contains(lower, "test result:") || hasLinePrefix(content, "Finished ", "Checking ", "Compiling ", "running ")
	case "pytest", "py.test", "python", "python3":
		return strings.Contains(lower, " passed") || strings.Contains(lower, " failed") || strings.Contains(lower, "collected ") || strings.Contains(content, "Ran ")
	case "sha256sum", "shasum":
		return strings.Contains(content, ": OK") || strings.Contains(content, ": FAILED")
	}
	if role == "build" || role == "check" {
		return hasLinePrefix(content, "success:", "error:", "warning:")
	}
	return false
}

func hasLinePrefix(content string, prefixes ...string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) {
				return true
			}
		}
	}
	return false
}

func remainingVerificationChannels(trajectory preprocess.Trajectory, target preprocess.Event, proof InvocationProof) []string {
	channels := make([]string, 0, 2)
	if command, exists := trajectoryEventByID(trajectory, proof.CommandEventID); exists && command.Command != nil && command.Command.ExitCode != nil {
		channels = append(channels, "command_exit_code")
	}
	if target.ToolResult != nil {
		if target.ToolResult.Error {
			channels = append(channels, "tool_result_error")
		}
		if slices.Contains([]string{"failed", "failure", "ok", "passed", "success"}, strings.ToLower(strings.TrimSpace(target.ToolResult.Status))) {
			channels = append(channels, "tool_result_status")
		}
	}
	if target.Output != nil && slices.Contains([]string{"failed", "failure", "ok", "passed", "success"}, strings.ToLower(strings.TrimSpace(target.Output.Status))) {
		channels = append(channels, "output_status")
	}
	return sortedStrings(channels)
}

func verificationResultChannel(event preprocess.Event) string {
	switch {
	case event.ToolResult != nil:
		return "tool_result_output"
	case event.Output != nil:
		return "command_output"
	default:
		return "none"
	}
}

func verificationResultStatus(event preprocess.Event) string {
	if event.ToolResult != nil {
		return event.ToolResult.Status
	}
	if event.Output != nil {
		return event.Output.Status
	}
	return ""
}

func verificationResultError(event preprocess.Event) bool {
	return event.ToolResult != nil && event.ToolResult.Error
}

func trajectoryEventByID(trajectory preprocess.Trajectory, eventID string) (preprocess.Event, bool) {
	for _, event := range trajectory.Events {
		if event.ID == eventID {
			return event, true
		}
	}
	return preprocess.Event{}, false
}

type shellCommandPart struct {
	text      string
	connector string
}

func commandSuccessMarker(command string, verificationSegment int) (string, bool) {
	parts, err := splitShellCommandParts(command)
	if err != nil || verificationSegment < 0 || verificationSegment+1 >= len(parts) || parts[verificationSegment].connector != "&&" {
		return "", false
	}
	segments, err := lexShellSegments(parts[verificationSegment+1].text)
	if err != nil || len(segments) != 1 || len(segments[0]) < 2 {
		return "", false
	}
	executable := filepath.Base(strings.ToLower(segments[0][0]))
	arguments := segments[0][1:]
	switch executable {
	case "echo":
		return strings.Join(arguments, " ") + "\n", true
	case "printf":
		if len(arguments) != 1 || strings.Contains(arguments[0], "%") {
			return "", false
		}
		return decodeShellPrintfLiteral(arguments[0])
	default:
		return "", false
	}
}

func splitShellCommandParts(command string) ([]shellCommandPart, error) {
	var parts []shellCommandPart
	var segment strings.Builder
	quote := rune(0)
	escaped := false
	flush := func(connector string) {
		text := strings.TrimSpace(segment.String())
		segment.Reset()
		if text == "" {
			return
		}
		parts = append(parts, shellCommandPart{text: text, connector: connector})
	}
	runes := []rune(command)
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		if escaped {
			segment.WriteRune(current)
			escaped = false
			continue
		}
		if quote != 0 {
			segment.WriteRune(current)
			if current == quote {
				quote = 0
			} else if current == '\\' && quote == '"' {
				escaped = true
			}
			continue
		}
		switch current {
		case '\\':
			segment.WriteRune(current)
			escaped = true
		case '\'', '"':
			quote = current
			segment.WriteRune(current)
		case '`':
			return nil, errors.New("command substitution is unsupported")
		case '$':
			if index+1 < len(runes) && runes[index+1] == '(' {
				return nil, errors.New("command substitution is unsupported")
			}
			segment.WriteRune(current)
		case '&', '|':
			connector := string(current)
			if index+1 < len(runes) && runes[index+1] == current {
				connector += string(current)
				index++
			}
			flush(connector)
		case ';':
			flush(";")
		case '\n':
			flush("newline")
		default:
			segment.WriteRune(current)
		}
	}
	if escaped || quote != 0 {
		return nil, errors.New("unterminated shell token")
	}
	flush("")
	return parts, nil
}

func decodeShellPrintfLiteral(value string) (string, bool) {
	var decoded strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' {
			decoded.WriteByte(value[index])
			continue
		}
		if index+1 >= len(value) {
			return "", false
		}
		index++
		switch value[index] {
		case 'n':
			decoded.WriteByte('\n')
		case 't':
			decoded.WriteByte('\t')
		case 'r':
			decoded.WriteByte('\r')
		case '\\':
			decoded.WriteByte('\\')
		default:
			return "", false
		}
	}
	if !utf8GraphicOrWhitespace(decoded.String()) {
		return "", false
	}
	return decoded.String(), true
}

func utf8GraphicOrWhitespace(value string) bool {
	for _, current := range value {
		if !unicode.IsGraphic(current) && !unicode.IsSpace(current) {
			return false
		}
	}
	return true
}
