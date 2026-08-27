package lineage

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const GoldenVectorFixtureSchemaVersion = "evalwitness.verification-lineage-golden-vectors.v1"

type GoldenVectorPolarity string

const (
	GoldenPositive GoldenVectorPolarity = "positive_control"
	GoldenNegative GoldenVectorPolarity = "negative_control"
	GoldenBoundary GoldenVectorPolarity = "capability_boundary"
)

type GoldenVectorImportOutcome string

const (
	GoldenImportAccepted      GoldenVectorImportOutcome = "accepted"
	GoldenImportRejected      GoldenVectorImportOutcome = "rejected"
	GoldenImportNotApplicable GoldenVectorImportOutcome = "not_applicable"
)

type GoldenVectorExpectation struct {
	RepresentableByFormat  CapabilityState           `json:"representable_by_format"`
	ImportOutcome          GoldenVectorImportOutcome `json:"import_outcome"`
	TerminalState          TerminalState             `json:"terminal_state"`
	CommandDisplay         string                    `json:"command_display"`
	MinimumToolCalls       int                       `json:"minimum_tool_calls"`
	MinimumToolResults     int                       `json:"minimum_tool_results"`
	MinimumCallResultLinks int                       `json:"minimum_call_result_links"`
	RedactionHits          int                       `json:"redaction_hits"`
	TruncationRequired     bool                      `json:"truncation_required"`
	ExitStatusRequired     bool                      `json:"exit_status_required"`
	Invariant              string                    `json:"invariant"`
}

type GoldenVectorObservation struct {
	ImportOutcome            GoldenVectorImportOutcome `json:"import_outcome"`
	SourceDigest             string                    `json:"source_digest"`
	TrajectoryDigest         string                    `json:"trajectory_digest"`
	RetainedTrajectoryDigest string                    `json:"retained_trajectory_digest"`
	SourceRecords            int                       `json:"source_records"`
	CanonicalEvents          int                       `json:"canonical_events"`
	RetainedCanonicalEvents  int                       `json:"retained_canonical_events"`
	ToolCalls                int                       `json:"tool_calls"`
	ToolResults              int                       `json:"tool_results"`
	Commands                 int                       `json:"commands"`
	CallResultLinks          int                       `json:"call_result_links"`
	RetainedToolCalls        int                       `json:"retained_tool_calls"`
	RetainedToolResults      int                       `json:"retained_tool_results"`
	RetainedCommands         int                       `json:"retained_commands"`
	RetainedCallResultLinks  int                       `json:"retained_call_result_links"`
	UnpairedToolCalls        int                       `json:"unpaired_tool_calls"`
	UnpairedToolResults      int                       `json:"unpaired_tool_results"`
	RedactionHits            int                       `json:"redaction_hits"`
	TruncationApplied        bool                      `json:"truncation_applied"`
	ExitStatusObservations   int                       `json:"exit_status_observations"`
	CommandDisplays          []string                  `json:"command_displays"`
	ErrorClass               string                    `json:"error_class"`
}

type VerificationLineageGoldenVector struct {
	VectorID          string                  `json:"vector_id"`
	CaseID            string                  `json:"case_id"`
	Format            preprocess.SourceFormat `json:"format"`
	Polarity          GoldenVectorPolarity    `json:"polarity"`
	DataRole          DataRole                `json:"data_role"`
	NativeInputDigest string                  `json:"native_input_digest"`
	PublicExcerpt     string                  `json:"public_excerpt"`
	Expectation       GoldenVectorExpectation `json:"expectation"`
	Observation       GoldenVectorObservation `json:"observation"`
	KnownMappingGap   string                  `json:"known_mapping_gap"`
}

type GoldenVectorFixturePolicy struct {
	FixtureSource             string `json:"fixture_source"`
	ProviderCallsAllowed      int    `json:"provider_calls_allowed"`
	AgentLaunchAllowed        bool   `json:"agent_launch_allowed"`
	ResearchDenominatorUse    bool   `json:"research_denominator_use"`
	FormatCapabilityInference bool   `json:"format_capability_inference"`
	RawSecretsPublished       bool   `json:"raw_secrets_published"`
}

type GoldenVectorFixtureSummary struct {
	Vectors              int `json:"vectors"`
	Formats              int `json:"formats"`
	SemanticCases        int `json:"semantic_cases"`
	PositiveControls     int `json:"positive_controls"`
	NegativeControls     int `json:"negative_controls"`
	CapabilityBoundaries int `json:"capability_boundaries"`
	Accepted             int `json:"accepted"`
	Rejected             int `json:"rejected"`
	NotApplicable        int `json:"not_applicable"`
	KnownMappingGaps     int `json:"known_mapping_gaps"`
}

type GoldenVectorFixtureSet struct {
	SchemaVersion   string                            `json:"schema_version"`
	CanonicalPolicy string                            `json:"canonical_policy"`
	PlanDigest      string                            `json:"plan_digest"`
	Policy          GoldenVectorFixturePolicy         `json:"policy"`
	Vectors         []VerificationLineageGoldenVector `json:"vectors"`
	Summary         GoldenVectorFixtureSummary        `json:"summary"`
	Digest          string                            `json:"digest"`
}

type goldenCaseDefinition struct {
	id               string
	polarity         GoldenVectorPolarity
	command          string
	result           string
	terminal         TerminalState
	invariant        string
	redaction        bool
	truncate         bool
	exitStatus       bool
	unsupported      bool
	missingCommand   bool
	missingCallID    bool
	duplicateCallID  bool
	outOfOrderResult bool
	structuredOnly   bool
}

func BuildGoldenVectorFixtureSet() (GoldenVectorFixtureSet, error) {
	set, err := buildGoldenVectorFixtureSetUnchecked()
	if err != nil {
		return GoldenVectorFixtureSet{}, err
	}
	return set, validateGoldenVectorFixtureSetIntegrity(set)
}

func (set GoldenVectorFixtureSet) Validate() error {
	if err := validateGoldenVectorFixtureSetIntegrity(set); err != nil {
		return err
	}
	expected, err := buildGoldenVectorFixtureSetUnchecked()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(set, expected) {
		return errors.New("golden-vector fixture set differs from the sealed native-vector contract")
	}
	return nil
}

func validateGoldenVectorFixtureSetIntegrity(set GoldenVectorFixtureSet) error {
	if set.SchemaVersion != GoldenVectorFixtureSchemaVersion || set.CanonicalPolicy != CanonicalPolicy || set.PlanDigest != LockedPlanDigest {
		return errors.New("golden-vector fixture identity is invalid")
	}
	if set.Policy != expectedGoldenVectorFixturePolicy() {
		return errors.New("golden-vector fixture policy is invalid")
	}
	if set.Summary != summarizeGoldenVectors(set.Vectors) {
		return errors.New("golden-vector fixture summary is invalid")
	}
	digest, err := digestGoldenVectorFixtureSet(set)
	if err != nil {
		return err
	}
	if set.Digest == "" || set.Digest != digest {
		return errors.New("golden-vector fixture digest is invalid")
	}
	return nil
}

func expectedGoldenVectorFixturePolicy() GoldenVectorFixturePolicy {
	return GoldenVectorFixturePolicy{FixtureSource: "typed_synthetic_native_documents"}
}

func buildGoldenVectorFixtureSetUnchecked() (GoldenVectorFixtureSet, error) {
	definitions := goldenCaseDefinitions()
	if err := validateGoldenCaseDefinitions(definitions); err != nil {
		return GoldenVectorFixtureSet{}, err
	}
	formats := []preprocess.SourceFormat{preprocess.SourceClaudeCode, preprocess.SourceCodexRollout, preprocess.SourceOpenCode}
	set := GoldenVectorFixtureSet{
		SchemaVersion: GoldenVectorFixtureSchemaVersion, CanonicalPolicy: CanonicalPolicy, PlanDigest: LockedPlanDigest,
		Policy: expectedGoldenVectorFixturePolicy(),
	}
	for _, format := range formats {
		for _, definition := range definitions {
			vector, err := buildGoldenVector(format, definition)
			if err != nil {
				return GoldenVectorFixtureSet{}, err
			}
			set.Vectors = append(set.Vectors, vector)
		}
	}
	set.Summary = summarizeGoldenVectors(set.Vectors)
	digest, err := digestGoldenVectorFixtureSet(set)
	if err != nil {
		return GoldenVectorFixtureSet{}, err
	}
	set.Digest = digest
	return set, nil
}

func buildGoldenVector(format preprocess.SourceFormat, definition goldenCaseDefinition) (VerificationLineageGoldenVector, error) {
	representable := CapabilityRequired
	if format == preprocess.SourceClaudeCode {
		representable = CapabilityUnspecified
	}
	if definition.outOfOrderResult && format == preprocess.SourceOpenCode {
		representable = CapabilityUnsupported
	}
	if definition.exitStatus && format != preprocess.SourceCodexRollout {
		representable = CapabilityUnspecified
	}
	expectation := GoldenVectorExpectation{
		RepresentableByFormat: representable, ImportOutcome: GoldenImportAccepted,
		TerminalState: definition.terminal, CommandDisplay: definition.command,
		MinimumToolCalls: 1, MinimumToolResults: 1, MinimumCallResultLinks: 1,
		Invariant: definition.invariant,
	}
	if definition.unsupported {
		expectation.ImportOutcome = GoldenImportRejected
		expectation.TerminalState = StateAdapterMappingLoss
		expectation.CommandDisplay = ""
		expectation.MinimumToolCalls = 0
		expectation.MinimumToolResults = 0
		expectation.MinimumCallResultLinks = 0
	}
	if definition.missingCommand {
		expectation.CommandDisplay = ""
		expectation.TerminalState = StateExportObservabilityAbsent
	}
	if definition.missingCallID || definition.duplicateCallID || definition.outOfOrderResult {
		expectation.TerminalState = StateAmbiguousLineage
		if representable != CapabilityUnsupported {
			expectation.ImportOutcome = GoldenImportRejected
			expectation.CommandDisplay = ""
			expectation.MinimumToolCalls = 0
			expectation.MinimumToolResults = 0
			expectation.MinimumCallResultLinks = 0
		}
	}
	if definition.redaction {
		expectation.RedactionHits = 1
	}
	if definition.truncate {
		expectation.TruncationRequired = true
	}
	if definition.exitStatus {
		expectation.ExitStatusRequired = format == preprocess.SourceCodexRollout
	}
	if representable == CapabilityUnsupported {
		expectation.ImportOutcome = GoldenImportNotApplicable
		expectation.CommandDisplay = ""
		expectation.MinimumToolCalls = 0
		expectation.MinimumToolResults = 0
		expectation.MinimumCallResultLinks = 0
	}
	if definition.exitStatus && format != preprocess.SourceCodexRollout {
		expectation.ImportOutcome = GoldenImportNotApplicable
		expectation.CommandDisplay = ""
		expectation.MinimumToolCalls = 0
		expectation.MinimumToolResults = 0
		expectation.MinimumCallResultLinks = 0
	}
	raw, excerpt, err := goldenNativeDocument(format, definition)
	if err != nil {
		return VerificationLineageGoldenVector{}, err
	}
	observation, err := observeGoldenVector(raw, definition, expectation)
	if err != nil {
		return VerificationLineageGoldenVector{}, err
	}
	gap := goldenKnownMappingGap(format, definition, observation)
	nativeInputDigest := ""
	if raw != "" {
		nativeInputDigest = digestBytes([]byte(raw))
	}
	return VerificationLineageGoldenVector{
		VectorID: string(format) + "/" + definition.id, CaseID: definition.id, Format: format,
		Polarity: definition.polarity, DataRole: RoleAdapterDevelopment,
		NativeInputDigest: nativeInputDigest, PublicExcerpt: excerpt,
		Expectation: expectation, Observation: observation, KnownMappingGap: gap,
	}, nil
}

func observeGoldenVector(raw string, definition goldenCaseDefinition, expectation GoldenVectorExpectation) (GoldenVectorObservation, error) {
	if expectation.ImportOutcome == GoldenImportNotApplicable {
		return GoldenVectorObservation{ImportOutcome: GoldenImportNotApplicable}, nil
	}
	trajectory, err := preprocess.IngestString(raw, preprocess.DefaultIngestOptions())
	if err != nil {
		if expectation.ImportOutcome != GoldenImportRejected {
			return GoldenVectorObservation{}, err
		}
		return GoldenVectorObservation{ImportOutcome: GoldenImportRejected, SourceDigest: digestBytes([]byte(raw)), ErrorClass: "strict_unsupported_native_shape"}, nil
	}
	if expectation.ImportOutcome == GoldenImportRejected {
		return GoldenVectorObservation{}, errors.New("strict adapter accepted a vector that must be rejected")
	}
	retained := trajectory
	if definition.truncate {
		retained, err = preprocess.ApplyEvidenceBudget(trajectory, 256)
		if err != nil {
			return GoldenVectorObservation{}, fmt.Errorf("apply golden-vector evidence budget: %w", err)
		}
	}
	observation := GoldenVectorObservation{
		ImportOutcome: GoldenImportAccepted, SourceDigest: trajectory.SourceDigest, TrajectoryDigest: trajectory.Digest,
		RetainedTrajectoryDigest: retained.Digest,
		SourceRecords:            trajectory.Report.SourceRecords, CanonicalEvents: len(trajectory.Events), RetainedCanonicalEvents: len(retained.Events),
		UnpairedToolCalls: trajectory.Report.UnpairedToolCalls, UnpairedToolResults: trajectory.Report.UnpairedToolResults,
		RedactionHits: trajectory.Report.RedactionHits, TruncationApplied: retained.Report.Truncation.Applied,
	}
	countGoldenTrajectory(trajectory, &observation, false)
	countGoldenTrajectory(retained, &observation, true)
	return observation, nil
}

func countGoldenTrajectory(trajectory preprocess.Trajectory, observation *GoldenVectorObservation, retained bool) {
	for _, event := range trajectory.Events {
		switch event.Kind {
		case preprocess.EventToolCall:
			if retained {
				observation.RetainedToolCalls++
			} else {
				observation.ToolCalls++
			}
		case preprocess.EventToolResult:
			if retained {
				observation.RetainedToolResults++
			} else {
				observation.ToolResults++
				if event.ToolResult.ExitCode != nil {
					observation.ExitStatusObservations++
				}
			}
		case preprocess.EventCommand:
			if retained {
				observation.RetainedCommands++
			} else {
				observation.Commands++
				observation.CommandDisplays = append(observation.CommandDisplays, event.Command.Display)
				if event.Command.ExitCode != nil {
					observation.ExitStatusObservations++
				}
			}
		}
	}
	for _, link := range trajectory.Links {
		if link.Kind == preprocess.LinkCallResult {
			if retained {
				observation.RetainedCallResultLinks++
			} else {
				observation.CallResultLinks++
			}
		}
	}
}

func goldenNativeDocument(format preprocess.SourceFormat, definition goldenCaseDefinition) (string, string, error) {
	if definition.outOfOrderResult && format == preprocess.SourceOpenCode {
		return "", "not representable: OpenCode colocates tool call and terminal state", nil
	}
	if definition.exitStatus && format != preprocess.SourceCodexRollout {
		return "", "unspecified: pinned format contract does not establish native exit-status representation", nil
	}
	switch format {
	case preprocess.SourceClaudeCode:
		return claudeGoldenDocument(definition)
	case preprocess.SourceCodexRollout:
		return codexGoldenDocument(definition)
	case preprocess.SourceOpenCode:
		return openCodeGoldenDocument(definition)
	default:
		return "", "", fmt.Errorf("unsupported golden-vector format %q", format)
	}
}

func claudeGoldenDocument(definition goldenCaseDefinition) (string, string, error) {
	if definition.unsupported {
		raw := `{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"future_verification_part"}]}}`
		return raw, "unknown content part: future_verification_part", nil
	}
	callID := "call-1"
	if definition.missingCallID {
		callID = ""
	}
	input := map[string]any{"command": definition.command}
	if definition.missingCommand {
		input = map[string]any{"description": "verification command display omitted"}
	}
	call := map[string]any{"type": "tool_use", "id": callID, "name": "Bash", "input": input}
	resultContent := definition.result
	if definition.redaction {
		resultContent = "Bearer fixture-token-value"
	}
	result := map[string]any{"type": "tool_result", "tool_use_id": callID, "content": resultContent}
	if definition.structuredOnly {
		result["content"] = ""
		result["is_error"] = false
	}
	records := []any{
		map[string]any{"type": "assistant", "uuid": "a1", "message": map[string]any{"role": "assistant", "content": []any{call}}},
		map[string]any{"type": "user", "uuid": "u1", "message": map[string]any{"role": "user", "content": []any{result}}},
	}
	if definition.duplicateCallID {
		second := map[string]any{
			"type": "assistant", "uuid": "a2",
			"message": map[string]any{
				"role": "assistant",
				"content": []any{map[string]any{
					"type": "tool_use", "id": callID, "name": "Bash",
					"input": map[string]any{"command": "go vet ./..."},
				}},
			},
		}
		records = []any{records[0], second, records[1]}
	}
	if definition.outOfOrderResult {
		records[0], records[1] = records[1], records[0]
	}
	raw, err := marshalJSONL(records)
	return raw, publicGoldenExcerpt(definition), err
}

func codexGoldenDocument(definition goldenCaseDefinition) (string, string, error) {
	if definition.unsupported {
		raw := `{"timestamp":"t","type":"response_item","payload":{"type":"future_verification_item"}}`
		return raw, "unknown response item: future_verification_item", nil
	}
	callID := "call-1"
	if definition.missingCallID {
		callID = ""
	}
	if definition.exitStatus {
		records := []any{
			codexEventRecord("exec_command_begin", map[string]any{"call_id": callID, "command": []string{"go", "test", "./..."}, "cwd": "/repo"}),
			codexEventRecord("exec_command_end", map[string]any{"call_id": callID, "exit_code": 0, "status": "completed", "stdout": "ok example/project"}),
		}
		raw, err := marshalJSONL(records)
		return raw, publicGoldenExcerpt(definition), err
	}
	arguments := map[string]any{"cmd": definition.command}
	if definition.missingCommand {
		arguments = map[string]any{"description": "verification command display omitted"}
	}
	encodedArguments, err := json.Marshal(arguments)
	if err != nil {
		return "", "", err
	}
	call := codexResponseRecord("function_call", map[string]any{"id": "fc-1", "name": "exec_command", "call_id": callID, "arguments": string(encodedArguments), "status": "completed"})
	resultText := definition.result
	if definition.redaction {
		resultText = "Bearer fixture-token-value"
	}
	resultPayload := map[string]any{"id": "fr-1", "call_id": callID, "output": resultText}
	if definition.structuredOnly {
		resultPayload["output"] = ""
		resultPayload["status"] = "completed"
	}
	result := codexResponseRecord("function_call_output", resultPayload)
	records := []any{call, result}
	if definition.duplicateCallID {
		secondArguments, marshalErr := json.Marshal(map[string]any{"cmd": "go vet ./..."})
		if marshalErr != nil {
			return "", "", marshalErr
		}
		records = []any{call, codexResponseRecord("function_call", map[string]any{"id": "fc-2", "name": "exec_command", "call_id": callID, "arguments": string(secondArguments)}), result}
	}
	if definition.outOfOrderResult {
		records[0], records[1] = records[1], records[0]
	}
	raw, err := marshalJSONL(records)
	return raw, publicGoldenExcerpt(definition), err
}

func openCodeGoldenDocument(definition goldenCaseDefinition) (string, string, error) {
	if definition.unsupported {
		raw := `{"info":{"id":"session-1","version":"fixture-v1"},"messages":[{"info":{"id":"message-1","role":"assistant"},"parts":[{"type":"future_verification_part"}]}]}`
		return raw, "unknown part: future_verification_part", nil
	}
	callID := "call-1"
	if definition.missingCallID {
		callID = ""
	}
	input := map[string]any{"command": definition.command}
	if definition.missingCommand {
		input = map[string]any{"description": "verification command display omitted"}
	}
	output := definition.result
	if definition.redaction {
		output = "Bearer fixture-token-value"
	}
	if definition.structuredOnly {
		output = ""
	}
	parts := []any{map[string]any{
		"id": "part-1", "type": "tool", "tool": "bash", "callID": callID,
		"state": map[string]any{"status": "completed", "input": input, "output": output},
	}}
	if definition.duplicateCallID {
		parts = append(parts, map[string]any{
			"id": "part-2", "type": "tool", "tool": "bash", "callID": callID,
			"state": map[string]any{"status": "completed", "input": map[string]any{"command": "go vet ./..."}, "output": "ok"},
		})
	}
	document := map[string]any{
		"info":     map[string]any{"id": "session-1", "version": "fixture-v1"},
		"messages": []any{map[string]any{"info": map[string]any{"id": "message-1", "role": "assistant", "time": map[string]any{"created": 1, "completed": 2}}, "parts": parts}},
	}
	raw, err := json.Marshal(document)
	return string(raw), publicGoldenExcerpt(definition), err
}

func codexResponseRecord(payloadType string, fields map[string]any) map[string]any {
	payload := map[string]any{"type": payloadType}
	for key, value := range fields {
		payload[key] = value
	}
	return map[string]any{"timestamp": "t", "type": "response_item", "payload": payload}
}

func codexEventRecord(payloadType string, fields map[string]any) map[string]any {
	payload := map[string]any{"type": payloadType}
	for key, value := range fields {
		payload[key] = value
	}
	return map[string]any{"timestamp": "t", "type": "event_msg", "payload": payload}
}

func marshalJSONL(records []any) (string, error) {
	lines := make([]string, len(records))
	for index, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			return "", err
		}
		lines[index] = string(encoded)
	}
	return strings.Join(lines, "\n"), nil
}

func publicGoldenExcerpt(definition goldenCaseDefinition) string {
	if definition.redaction {
		return "result contains one synthetic credential shape; published observation contains only its redaction accounting"
	}
	if definition.missingCommand {
		return "tool invocation intentionally omits command display"
	}
	if definition.structuredOnly {
		return definition.command + " -> empty text with structured completed status"
	}
	if definition.truncate {
		return fmt.Sprintf("%s -> %d-byte synthetic result constrained by a 256-token evidence budget", definition.command, len(definition.result))
	}
	return definition.command + " -> " + definition.result
}

func goldenKnownMappingGap(format preprocess.SourceFormat, definition goldenCaseDefinition, observation GoldenVectorObservation) string {
	if observation.ImportOutcome != GoldenImportAccepted {
		return ""
	}
	switch {
	case definition.exitStatus && format == preprocess.SourceCodexRollout && observation.ExitStatusObservations == 0:
		return "native exit status is reduced to an error bit instead of retained on the command event"
	default:
		return ""
	}
}

func summarizeGoldenVectors(vectors []VerificationLineageGoldenVector) GoldenVectorFixtureSummary {
	summary := GoldenVectorFixtureSummary{Vectors: len(vectors), Formats: 3, SemanticCases: len(goldenCaseDefinitions())}
	for _, vector := range vectors {
		switch vector.Polarity {
		case GoldenPositive:
			summary.PositiveControls++
		case GoldenNegative:
			summary.NegativeControls++
		case GoldenBoundary:
			summary.CapabilityBoundaries++
		}
		switch vector.Observation.ImportOutcome {
		case GoldenImportAccepted:
			summary.Accepted++
		case GoldenImportRejected:
			summary.Rejected++
		case GoldenImportNotApplicable:
			summary.NotApplicable++
		}
		if vector.KnownMappingGap != "" {
			summary.KnownMappingGaps++
		}
	}
	return summary
}

func digestGoldenVectorFixtureSet(set GoldenVectorFixtureSet) (string, error) {
	set.Digest = ""
	return digestJSON(set)
}

func goldenCaseDefinitions() []goldenCaseDefinition {
	longOutput := strings.Repeat("verification-output-", 1024)
	return []goldenCaseDefinition{
		{id: "direct_test_invocation", polarity: GoldenPositive, command: "go test ./...", result: "ok example/project", terminal: StateDirectVerificationInvocation, invariant: "a direct failable test invocation remains identifiable"},
		{id: "direct_check_invocation", polarity: GoldenPositive, command: "cargo check --workspace", result: "Finished dev profile", terminal: StateDirectVerificationInvocation, invariant: "a direct failable check invocation remains identifiable"},
		{id: "direct_build_invocation", polarity: GoldenPositive, command: "go build ./cmd/evalwitness", result: "", terminal: StateDirectVerificationInvocation, invariant: "a direct failable build invocation remains identifiable"},
		{id: "direct_outcome_probe", polarity: GoldenPositive, command: "test -s report.json", result: "", terminal: StateDirectVerificationInvocation, invariant: "a direct failable outcome probe remains identifiable"},
		{id: "printed_verification_name", polarity: GoldenNegative, command: "printf 'go test ./...\\n'", result: "go test ./...", terminal: StateNonFailableVerification, invariant: "printing a verification name is not an invocation"},
		{id: "searched_verification_name", polarity: GoldenNegative, command: "rg 'cargo test' docs", result: "docs/guide.md:cargo test", terminal: StateNonFailableVerification, invariant: "searching for a verification name is not an invocation"},
		{id: "wrapper_chain", polarity: GoldenPositive, command: "env CI=1 sh -c 'go test ./...'", result: "ok example/project", terminal: StateDirectVerificationInvocation, invariant: "a wrapper chain must preserve the invoked verification and its decisive status"},
		{id: "compound_command", polarity: GoldenPositive, command: "go test ./... && go vet ./...", result: "ok example/project", terminal: StateDirectVerificationInvocation, invariant: "compound segments must be parsed without treating printed text as execution"},
		{id: "missing_command_display", polarity: GoldenBoundary, result: "ok example/project", terminal: StateExportObservabilityAbsent, missingCommand: true, invariant: "absence of native command display is an export-observability loss, not behavior absence"},
		{id: "missing_call_id", polarity: GoldenNegative, command: "go test ./...", result: "ok example/project", terminal: StateAmbiguousLineage, missingCallID: true, invariant: "a result without native call identity cannot prove cross-record lineage"},
		{id: "duplicate_call_id", polarity: GoldenNegative, command: "go test ./...", result: "ok example/project", terminal: StateAmbiguousLineage, duplicateCallID: true, invariant: "duplicate native call identities are ambiguous even when one result is present"},
		{id: "out_of_order_result", polarity: GoldenNegative, command: "go test ./...", result: "ok example/project", terminal: StateAmbiguousLineage, outOfOrderResult: true, invariant: "a result observed before its call is not repaired by timestamp-free guessing"},
		{id: "bounded_truncation", polarity: GoldenBoundary, command: "go test ./...", result: longOutput, terminal: StateClaimSpecificEvidenceNotWeakened, truncate: true, invariant: "budget truncation is explicit and cannot masquerade as complete evidence"},
		{id: "secret_redaction", polarity: GoldenBoundary, command: "go test ./...", terminal: StateClaimSpecificEvidenceNotWeakened, redaction: true, invariant: "redaction loss is counted without publishing synthetic credential material"},
		{id: "unsupported_native_syntax", polarity: GoldenBoundary, terminal: StateAdapterMappingLoss, unsupported: true, invariant: "strict ingestion rejects unknown native syntax"},
		{id: "agent_prose_attached_to_result", polarity: GoldenNegative, command: "cmp -s /tmp/before /tmp/after", result: "I should run another check before finalizing.", terminal: StateClaimSpecificEvidenceNotWeakened, invariant: "agent narration sharing a result body is not subprocess evidence"},
		{id: "same_path_comparison_false_target", polarity: GoldenNegative, command: "cmp -s /repo/file /repo/file && printf 'legacy_unchanged\\n'", result: "legacy_unchanged", terminal: StateNonFailableVerification, invariant: "a same-path comparison cannot falsify unchanged-file provenance"},
		{id: "structured_success_after_text_omission", polarity: GoldenNegative, command: "go test ./...", terminal: StateClaimSpecificEvidenceNotWeakened, structuredOnly: true, invariant: "omitting result text does not weaken a claim when structured success survives"},
		{id: "distinct_operand_comparison", polarity: GoldenPositive, command: "cmp -s /tmp/before /tmp/after && printf 'files_equal\\n'", result: "files_equal", terminal: StateDirectVerificationInvocation, invariant: "a comparison of distinct operands has a real failure condition"},
		{id: "planning_prose_false_target", polarity: GoldenNegative, command: "diff -u /tmp/before.err /tmp/after.err", result: "I need to invoke the verification specialist before completion.", terminal: StateClaimSpecificEvidenceNotWeakened, invariant: "removing planning prose cannot erase the empty-success channel of a successful diff"},
		{id: "native_exit_status", polarity: GoldenBoundary, command: "go test ./...", result: "ok example/project", terminal: StateDirectVerificationInvocation, exitStatus: true, invariant: "native exit status is preserved when the admitted export shape carries it"},
	}
}

func validateGoldenCaseDefinitions(definitions []goldenCaseDefinition) error {
	if len(definitions) != 21 {
		return errors.New("golden-vector taxonomy requires exactly 21 semantic cases")
	}
	ids := make([]string, len(definitions))
	for index, definition := range definitions {
		ids[index] = definition.id
		if definition.id == "" || definition.invariant == "" || !slices.Contains([]GoldenVectorPolarity{GoldenPositive, GoldenNegative, GoldenBoundary}, definition.polarity) {
			return errors.New("golden-vector case definition is incomplete")
		}
	}
	sorted := append([]string(nil), ids...)
	slices.Sort(sorted)
	for index := 1; index < len(sorted); index++ {
		if sorted[index] == sorted[index-1] {
			return errors.New("golden-vector case identities must be unique")
		}
	}
	return nil
}
