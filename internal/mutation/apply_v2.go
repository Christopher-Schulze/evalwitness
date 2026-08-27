package mutation

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func ApplyV2(parent preprocess.Trajectory, request ApplyRequest) (ApplyV2Outcome, error) {
	if err := parent.Validate(); err != nil {
		return ApplyV2Outcome{}, fmt.Errorf("validate mutation source: %w", err)
	}
	definition, exists := DefinitionFor(request.Family)
	if !exists {
		return ApplyV2Outcome{}, fmt.Errorf("unsupported mutation family %q", request.Family)
	}
	if definition.PairLevel {
		return ApplyV2Outcome{}, errors.New("candidate-order reversal requires its pair-level v2 operator")
	}
	if missing(request.CorpusVersion, request.TaskID, request.RepositoryID, request.SourceFamily, request.SourceLocation,
		request.SourceRevision, request.SplitGroupID, request.Seed) {
		return ApplyV2Outcome{}, errors.New("mutation request source, corpus, split, and seed are required")
	}
	events, err := cloneEvents(parent.Events)
	if err != nil {
		return ApplyV2Outcome{}, err
	}
	change, err := applyTrajectoryMutationV2(parent, events, request, definition)
	if err != nil {
		return rejectedV2Outcome(parent, request, err)
	}
	child, err := preprocess.DeriveTrajectory(parent, change.events, change.links, preprocess.DerivationSpec{
		Relation: string(definition.Relation), Validator: request.Validator.ID,
		ChangedEventIDs: change.eventIDs, ChangedFieldPaths: change.fieldPaths,
	})
	if err != nil {
		return ApplyV2Outcome{}, fmt.Errorf("derive v2 mutated trajectory: %w", err)
	}
	preservation, preservationChecks, err := buildPreservationV2(parent, child, definition)
	if err != nil {
		return ApplyV2Outcome{}, err
	}
	checks := append(append([]Check{}, change.checks...), preservationChecks...)
	label := LabelProven
	relation := definition.Relation
	if relation == RelationAmbiguous {
		label = LabelAmbiguous
		preservation.AmbiguityReasons = append(preservation.AmbiguityReasons, "prespecified_ambiguity")
		sort.Strings(preservation.AmbiguityReasons)
	}
	for _, check := range checks {
		if !check.Passed {
			return rejectedV2Outcome(parent, request, constructRejection{
				reasons: []ConstructRejectionReason{RejectionPreservationFailure}, eventIDs: change.eventIDs,
				role: constructRole(change), checks: checks,
			})
		}
	}
	firewall, err := sealConstructFirewall(ConstructFirewallReport{
		Family: request.Family, Status: ConstructApplied, SourceTrajectoryDigest: parent.Digest,
		MutatedTrajectoryDigest: child.Digest, TargetEventIDs: change.eventIDs,
		ProofEventIDs: constructProofEventIDs(change),
		SemanticRole:  constructRole(change), Checks: checks, RejectionReasons: []ConstructRejectionReason{},
	})
	if err != nil {
		return ApplyV2Outcome{}, err
	}
	witness, err := SealWitness(Witness{
		ValidatorID: request.Validator.ID, ValidatorVersion: request.Validator.Version,
		Relation: relation, LabelState: label, Checks: checks,
	})
	if err != nil {
		return ApplyV2Outcome{}, err
	}
	packet, err := SealBlindReviewPacket(BlindReviewPacket{
		MutationMaterialDigest: digestText(parent.Digest + "\x00" + child.Digest + "\x00" + string(request.Family) + "\x00" + firewall.Digest),
		TaskAlias:              "task-" + digestText(request.TaskID)[:16], SourceFormat: parent.SourceFormat,
		OriginalDigest: parent.Digest, MutatedDigest: child.Digest, AffectedEventCount: len(change.eventIDs),
		ReviewQuestions: []string{
			"Did the transformation alter task-level semantic quality?",
			"Does the construct-firewall proof match the visible change?",
			"Is either trajectory ambiguous without hidden source information?",
		},
	})
	if err != nil {
		return ApplyV2Outcome{}, err
	}
	parameters := append([]NamedValue(nil), change.parameters...)
	sort.Slice(parameters, func(left, right int) bool { return parameters[left].Name < parameters[right].Name })
	manifest, err := SealManifestV2(Manifest{
		CorpusVersion: request.CorpusVersion,
		Source: SourceRef{
			TaskID: request.TaskID, RepositoryID: request.RepositoryID, SourceFamily: request.SourceFamily,
			SourceFormat: parent.SourceFormat, SourceLocation: request.SourceLocation, SourceRevision: request.SourceRevision,
			SourceDigest: parent.SourceDigest, TrajectoryDigest: parent.Digest, Outcome: request.Outcome,
		},
		Program: Program{
			Family: request.Family, Seed: request.Seed, Operator: operatorForProgram(MutationProgramVersionV2, definition),
			Parameters: parameters,
		},
		Class: definition.Class, ExpectedRelation: relation,
		Affected: AffectedSurface{
			EventIDs: sortedStrings(change.eventIDs), FieldPaths: sortedFieldPaths(change.fieldPaths), FileAliases: sortedStrings(change.fileAliases),
		},
		Validator: request.Validator, Preservation: preservation, Witness: witness,
		OutcomeProof: request.OutcomeProof,
		License:      request.License, Privacy: request.Privacy, SplitGroupID: request.SplitGroupID,
		OriginalTrajectoryDigest: parent.Digest, MutatedTrajectoryDigest: child.Digest,
		Review: ReviewState{
			Required: label == LabelAmbiguous || request.ReviewSampled, SamplingStratum: request.ReviewSamplingStratum, BlindPacketDigest: packet.Digest,
		},
		ConstructFirewallDigest: firewall.Digest,
	})
	if err != nil {
		return ApplyV2Outcome{}, err
	}
	result := ApplyResult{Manifest: manifest, Mutated: child, Packet: packet}
	return ApplyV2Outcome{Status: ConstructApplied, Applied: &result, Firewall: firewall}, nil
}

func rejectedV2Outcome(parent preprocess.Trajectory, request ApplyRequest, cause error) (ApplyV2Outcome, error) {
	var rejection constructRejection
	if !errors.As(cause, &rejection) {
		if !strings.Contains(cause.Error(), "no applicable target event") && !strings.Contains(cause.Error(), "no causally independent") {
			return ApplyV2Outcome{}, cause
		}
		rejection = constructRejection{
			reasons: []ConstructRejectionReason{RejectionNoApplicableTarget},
			checks:  []Check{{Name: "applicable_target", Expected: "present", Observed: "absent", Passed: false}},
		}
	}
	if len(rejection.checks) == 0 {
		rejection.checks = []Check{{Name: "construct_eligibility", Expected: "eligible", Observed: "rejected", Passed: false}}
	}
	firewall, err := sealConstructFirewall(ConstructFirewallReport{
		Family: request.Family, Status: ConstructRejected, SourceTrajectoryDigest: parent.Digest,
		TargetEventIDs: rejection.eventIDs, SemanticRole: rejection.role,
		ProofEventIDs: rejection.eventIDs,
		Checks:        rejection.checks, RejectionReasons: rejection.reasons,
	})
	if err != nil {
		return ApplyV2Outcome{}, err
	}
	return ApplyV2Outcome{Status: ConstructRejected, Firewall: firewall}, nil
}

func applyTrajectoryMutationV2(parent preprocess.Trajectory, events []preprocess.Event, request ApplyRequest, definition Definition) (mutationChange, error) {
	switch request.Family {
	case FamilyTestEvidenceOmitted:
		return mutateVerifiedEvidenceV2(parent, events, request)
	case FamilyNeutralFormatting:
		return mutateNaturalFormattingV2(parent, events, request)
	case FamilyCausalIndependentReorder:
		return mutateIndependentOrderV2(parent, events, request)
	default:
		change, err := applyTrajectoryMutation(parent, events, request, definition)
		if err == nil {
			change.parameters = append(change.parameters, NamedValue{Name: "construct_role", Value: "registered_operator"})
			change.checks = append(change.checks, Check{Name: "registered_v2_operator", Expected: "registered", Observed: "registered", Passed: true})
		}
		return change, err
	}
}

func constructRole(change mutationChange) string {
	for _, parameter := range change.parameters {
		if parameter.Name == "construct_role" {
			return parameter.Value
		}
	}
	return "registered_operator"
}

func constructProofEventIDs(change mutationChange) []string {
	if len(change.proofEventIDs) != 0 {
		return change.proofEventIDs
	}
	return change.eventIDs
}

func mutateVerifiedEvidenceV2(parent preprocess.Trajectory, events []preprocess.Event, request ApplyRequest) (mutationChange, error) {
	type candidate struct {
		index         int
		role          string
		proofEventIDs []string
	}
	var candidates []candidate
	var evidenceIndices []int
	for index, event := range events {
		if request.TargetEventID != "" && event.ID != request.TargetEventID {
			continue
		}
		if eventEvidenceText(event) == "" {
			continue
		}
		evidenceIndices = append(evidenceIndices, index)
		if role, proofEventIDs := verifiedExecutionRole(parent, event); role != "" {
			candidates = append(candidates, candidate{index: index, role: role, proofEventIDs: proofEventIDs})
		}
	}
	if len(candidates) == 0 {
		targets := make([]string, 0, len(evidenceIndices))
		for _, index := range evidenceIndices {
			targets = append(targets, events[index].ID)
		}
		return mutationChange{}, rejectConstruct(RejectionUnverifiedEvidenceRole, targets, "", Check{
			Name: "verified_execution_lineage", Expected: "test, check, build, or outcome_probe", Observed: "unverified", Passed: false,
		})
	}
	indices := make([]int, len(candidates))
	roleByIndex := make(map[int]string, len(candidates))
	proofByIndex := make(map[int][]string, len(candidates))
	for index, candidate := range candidates {
		indices[index] = candidate.index
		roleByIndex[candidate.index] = candidate.role
		proofByIndex[candidate.index] = candidate.proofEventIDs
	}
	selected, err := deterministicIndex(events, indices, request)
	if err != nil {
		return mutationChange{}, err
	}
	originalID := events[selected].ID
	before := eventEvidenceText(events[selected])
	role := roleByIndex[selected]
	setEventEvidenceText(&events[selected], "")
	change, err := rebuildMutationEvent(parent, events, selected, originalID, "/evidence", []NamedValue{
		{Name: "construct_role", Value: role}, {Name: "operation", Value: "omit_verified_execution_evidence"},
	}, []Check{
		{Name: "verified_execution_lineage", Expected: "test, check, build, or outcome_probe", Observed: role, Passed: true},
		{Name: "evidence_nonempty_before", Expected: "nonempty", Observed: fmt.Sprintf("bytes=%d", len(before)), Passed: before != ""},
		{Name: "evidence_empty_after", Expected: "empty", Observed: fmt.Sprintf("bytes=%d", len(eventEvidenceText(events[selected]))), Passed: eventEvidenceText(events[selected]) == ""},
	})
	change.proofEventIDs = proofByIndex[selected]
	return change, err
}

func verifiedExecutionRole(trajectory preprocess.Trajectory, evidence preprocess.Event) (string, []string) {
	type linkedCommand struct {
		text    string
		eventID string
	}
	commands := make([]linkedCommand, 0)
	for _, event := range trajectory.Events {
		linked := false
		if evidence.ToolResult != nil && event.ToolCall != nil && evidence.ToolResult.CallID != "" && evidence.ToolResult.CallID == event.ToolCall.CallID {
			linked = true
		}
		for _, link := range trajectory.Links {
			if link.ToID == evidence.ID && link.FromID == event.ID && (link.Kind == preprocess.LinkCallResult || link.Kind == preprocess.LinkParent || link.Kind == preprocess.LinkReference) {
				linked = true
			}
		}
		if !linked {
			continue
		}
		if event.ToolCall != nil {
			commands = append(commands, linkedCommand{text: event.ToolCall.ToolName + " " + event.ToolCall.Arguments, eventID: event.ID})
		}
		if event.Command != nil {
			commands = append(commands, linkedCommand{text: event.Command.Display, eventID: event.ID})
		}
	}
	for _, command := range commands {
		if role := classifyVerificationCommand(command.text); role != "" {
			return role, sortedStrings([]string{command.eventID, evidence.ID})
		}
	}
	return "", nil
}

func classifyVerificationCommand(command string) string {
	normalized := strings.ToLower(command)
	normalized = strings.NewReplacer("\"", " ", "'", " ", "{", " ", "}", " ", "[", " ", "]", " ", ":", " ", ",", " ", "=", " ").Replace(normalized)
	fields := strings.Fields(normalized)
	joined := " " + strings.Join(fields, " ") + " "
	testPatterns := []string{" go test ", " cargo test ", " pytest ", " py.test ", " python -m pytest ", " python3 -m pytest ", " python -m unittest ", " python3 -m unittest ", " bun test ", " npm test ", " npm run test ", " pnpm test ", " yarn test ", " ctest ", " make test ", " mvn test ", " gradle test ", " ./gradlew test "}
	for _, pattern := range testPatterns {
		if strings.Contains(joined, pattern) {
			return "test"
		}
	}
	for index, field := range fields {
		if index == 0 || (fields[index-1] != "python" && fields[index-1] != "python3") {
			continue
		}
		base := filepath.Base(field)
		if strings.HasSuffix(base, ".py") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(strings.TrimSuffix(base, ".py"), "_test")) {
			return "test"
		}
	}
	checkPatterns := []string{" go vet ", " cargo check ", " staticcheck ", " golangci-lint ", " ruff check ", " eslint ", " tsc --noemit "}
	for _, pattern := range checkPatterns {
		if strings.Contains(joined, pattern) {
			return "check"
		}
	}
	buildPatterns := []string{" go build ", " cargo build ", " bun run build ", " npm run build ", " pnpm build ", " yarn build ", " make build "}
	for _, pattern := range buildPatterns {
		if strings.Contains(joined, pattern) {
			return "build"
		}
	}
	probePatterns := []string{" diff ", " cmp ", " sha256sum ", " shasum ", " assert "}
	for _, pattern := range probePatterns {
		if strings.Contains(joined, pattern) {
			return "outcome_probe"
		}
	}
	return ""
}

func mutateNaturalFormattingV2(parent preprocess.Trajectory, events []preprocess.Event, request ApplyRequest) (mutationChange, error) {
	afterByEventID := make(map[string]string)
	indices := candidateIndices(events, request.TargetEventID, func(event preprocess.Event) bool {
		after, eligible := naturalFormatting(event)
		if eligible {
			afterByEventID[event.ID] = after
		}
		return eligible
	})
	selected, err := deterministicIndex(events, indices, request)
	if err != nil {
		return mutationChange{}, rejectConstruct(RejectionUnnaturalFormatting, nil, "assistant_prose", Check{
			Name: "natural_prose_target", Expected: "wrappable prose", Observed: "absent", Passed: false,
		})
	}
	originalID := events[selected].ID
	before := eventText(events[selected])
	after := afterByEventID[events[selected].ID]
	beforeTokens := strings.Fields(before)
	afterTokens := strings.Fields(after)
	if strings.Join(beforeTokens, "\x00") != strings.Join(afterTokens, "\x00") {
		return mutationChange{}, rejectConstruct(RejectionTokenSequenceChanged, []string{originalID}, "assistant_prose", Check{
			Name: "token_sequence_preserved", Expected: "equal", Observed: "different", Passed: false,
		})
	}
	setEventText(&events[selected], after)
	return rebuildMutationEvent(parent, events, selected, originalID, "/text", []NamedValue{
		{Name: "construct_role", Value: "assistant_prose"}, {Name: "operation", Value: "natural_line_wrap"}, {Name: "wrap_width", Value: "72"},
	}, []Check{
		{Name: "natural_prose_target", Expected: "wrappable prose", Observed: "wrappable prose", Passed: true},
		{Name: "token_sequence_preserved", Expected: "equal", Observed: "equal", Passed: true},
		{Name: "single_token_paragraphs_absent", Expected: "absent", Observed: fmt.Sprintf("present=%t", hasSingleTokenLine(after)), Passed: !hasSingleTokenLine(after)},
		{Name: "formatting_changed", Expected: "changed", Observed: fmt.Sprintf("changed=%t", before != after), Passed: before != after},
	})
}

func naturalFormatting(event preprocess.Event) (string, bool) {
	if event.Message == nil {
		return "", false
	}
	textParts := 0
	before := ""
	for _, part := range event.Message.Parts {
		if part.Kind == preprocess.ContentText {
			textParts++
			before = part.Text
		}
	}
	trimmed := strings.TrimSpace(before)
	if textParts != 1 || len(strings.Fields(trimmed)) < 12 || len(trimmed) <= 72 || strings.Contains(trimmed, "```") || strings.HasPrefix(trimmed, "Execute [") || strings.HasPrefix(trimmed, "{") {
		return "", false
	}
	if !strings.ContainsAny(trimmed, ".,;:!?") {
		return "", false
	}
	after := wrapTokens(strings.Fields(trimmed), 72)
	if after == before || hasSingleTokenLine(after) {
		return "", false
	}
	return after, true
}

func wrapTokens(tokens []string, width int) string {
	lines := make([][]string, 0)
	current := make([]string, 0)
	length := 0
	for _, token := range tokens {
		nextLength := len(token)
		if len(current) > 0 {
			nextLength += length + 1
		}
		if len(current) > 0 && nextLength > width {
			lines = append(lines, current)
			current = []string{token}
			length = len(token)
			continue
		}
		current = append(current, token)
		length = nextLength
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}
	if len(lines) > 1 && len(lines[len(lines)-1]) == 1 && len(lines[len(lines)-2]) > 2 {
		previous := lines[len(lines)-2]
		last := lines[len(lines)-1]
		lines[len(lines)-2] = previous[:len(previous)-1]
		lines[len(lines)-1] = append([]string{previous[len(previous)-1]}, last...)
	}
	rendered := make([]string, len(lines))
	for index, line := range lines {
		rendered[index] = strings.Join(line, " ")
	}
	return strings.Join(rendered, "\n")
}

func hasSingleTokenLine(value string) bool {
	for _, line := range strings.Split(value, "\n") {
		if len(strings.Fields(line)) == 1 {
			return true
		}
	}
	return false
}

func mutateIndependentOrderV2(parent preprocess.Trajectory, events []preprocess.Event, request ApplyRequest) (mutationChange, error) {
	left, right, checks, rejection := transactionIndependentAdjacentPair(events, parent.Links, request.TargetEventID, request.Seed)
	if left < 0 {
		return mutationChange{}, rejectConstruct(rejection, nil, "transaction_independent_events", checks...)
	}
	leftID, rightID := events[left].ID, events[right].ID
	leftOrder, rightOrder := events[left].Order, events[right].Order
	events[left], events[right] = events[right], events[left]
	events[left].Order = leftOrder
	events[right].Order = rightOrder
	ids := sortedStrings([]string{leftID, rightID})
	return mutationChange{
		events: events, links: append([]preprocess.Link(nil), parent.Links...), eventIDs: ids,
		fieldPaths: sortedFieldPaths([]preprocess.FieldPath{
			preprocess.FieldPath("/events/" + leftID + "/order"), preprocess.FieldPath("/events/" + rightID + "/order"),
		}),
		parameters: []NamedValue{
			{Name: "construct_role", Value: "transaction_independent_events"}, {Name: "left_event", Value: leftID}, {Name: "right_event", Value: rightID},
		},
		checks: checks, proofEventIDs: ids,
	}, nil
}

func transactionIndependentAdjacentPair(events []preprocess.Event, links []preprocess.Link, targetEventID, seed string) (int, int, []Check, ConstructRejectionReason) {
	if len(events) < 2 {
		return -1, -1, []Check{{Name: "adjacent_pair_available", Expected: "present", Observed: "absent", Passed: false}}, RejectionNoApplicableTarget
	}
	components := dependencyComponents(events, links)
	type pair struct {
		left   int
		right  int
		hash   string
		checks []Check
	}
	var pairs []pair
	sawTransactionDependency := false
	sawTemporalDependency := false
	for index := 0; index+1 < len(events); index++ {
		left, right := events[index], events[index+1]
		if targetEventID != "" && left.ID != targetEventID && right.ID != targetEventID {
			continue
		}
		if components[left.ID] == components[right.ID] {
			sawTransactionDependency = true
			continue
		}
		if left.Timestamp != "" && right.Timestamp != "" && left.Timestamp != right.Timestamp {
			sawTemporalDependency = true
			continue
		}
		pairs = append(pairs, pair{
			left: index, right: index + 1, hash: digestText(seed + "\x00" + left.ID + "\x00" + right.ID),
			checks: []Check{
				{Name: "dependency_components_distinct", Expected: "distinct", Observed: "distinct", Passed: true},
				{Name: "raw_source_records_distinct", Expected: "distinct", Observed: "distinct", Passed: left.Source.Record != right.Source.Record},
				{Name: "shared_call_id_absent", Expected: "absent", Observed: fmt.Sprintf("present=%t", sharedCallID(left, right)), Passed: !sharedCallID(left, right)},
				{Name: "temporal_dependency_absent", Expected: "absent", Observed: "absent", Passed: true},
			},
		})
	}
	if len(pairs) == 0 {
		switch {
		case sawTransactionDependency:
			return -1, -1, []Check{{Name: "dependency_components_distinct", Expected: "distinct", Observed: "shared", Passed: false}}, RejectionTransactionDependency
		case sawTemporalDependency:
			return -1, -1, []Check{{Name: "temporal_dependency_absent", Expected: "absent", Observed: "ordered timestamps", Passed: false}}, RejectionTemporalDependency
		default:
			return -1, -1, []Check{{Name: "adjacent_pair_available", Expected: "present", Observed: "absent", Passed: false}}, RejectionNoApplicableTarget
		}
	}
	sort.Slice(pairs, func(left, right int) bool { return pairs[left].hash < pairs[right].hash })
	return pairs[0].left, pairs[0].right, pairs[0].checks, ""
}

func dependencyComponents(events []preprocess.Event, links []preprocess.Link) map[string]string {
	parent := make(map[string]string, len(events))
	byRecord := make(map[int]string)
	bySourceEvent := make(map[string]string)
	byCallID := make(map[string]string)
	for _, event := range events {
		parent[event.ID] = event.ID
	}
	var find func(string) string
	find = func(value string) string {
		root, exists := parent[value]
		if !exists {
			return ""
		}
		if root != value {
			parent[value] = find(root)
		}
		return parent[value]
	}
	union := func(left, right string) {
		leftRoot, rightRoot := find(left), find(right)
		if leftRoot == "" || rightRoot == "" || leftRoot == rightRoot {
			return
		}
		if leftRoot < rightRoot {
			parent[rightRoot] = leftRoot
		} else {
			parent[leftRoot] = rightRoot
		}
	}
	for _, link := range links {
		union(link.FromID, link.ToID)
	}
	for _, event := range events {
		if first, exists := byRecord[event.Source.Record]; exists {
			union(first, event.ID)
		} else {
			byRecord[event.Source.Record] = event.ID
		}
		if event.SourceEventID != "" {
			if first, exists := bySourceEvent[event.SourceEventID]; exists {
				union(first, event.ID)
			} else {
				bySourceEvent[event.SourceEventID] = event.ID
			}
		}
		callID := ""
		if event.ToolCall != nil {
			callID = event.ToolCall.CallID
		} else if event.ToolResult != nil {
			callID = event.ToolResult.CallID
		}
		if callID != "" {
			if first, exists := byCallID[callID]; exists {
				union(first, event.ID)
			} else {
				byCallID[callID] = event.ID
			}
		}
		if event.Message != nil {
			for _, part := range event.Message.Parts {
				if part.Kind == preprocess.ContentEventReference && part.EventID != "" {
					union(event.ID, part.EventID)
				}
			}
		}
	}
	components := make(map[string]string, len(events))
	for _, event := range events {
		components[event.ID] = find(event.ID)
	}
	return components
}

func sharedCallID(left, right preprocess.Event) bool {
	callID := func(event preprocess.Event) string {
		if event.ToolCall != nil {
			return event.ToolCall.CallID
		}
		if event.ToolResult != nil {
			return event.ToolResult.CallID
		}
		return ""
	}
	leftID, rightID := callID(left), callID(right)
	return leftID != "" && leftID == rightID
}
