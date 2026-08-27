package preprocess

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type selectionUnit struct {
	eventIndexes []int
	score        int
	firstOrder   int
}

type pairedSelectionUnit struct {
	key      string
	left     selectionUnit
	right    selectionUnit
	score    int
	required bool
}

func ApplyEvidenceBudget(source Trajectory, tokenBudget int) (Trajectory, error) {
	return applyEvidenceBudget(source, tokenBudget, nil)
}

// ApplyEvidenceBudgetWithRequiredEvents applies the canonical evidence policy
// while retaining every selection unit containing a required event. Required
// call/result pairs remain atomic. The operation fails when the required units
// cannot fit the hard budget; it never silently drops an anchor.
func ApplyEvidenceBudgetWithRequiredEvents(source Trajectory, tokenBudget int, requiredEventIDs []string) (Trajectory, error) {
	return applyEvidenceBudget(source, tokenBudget, requiredEventIDs)
}

// ApplyPairedEvidenceBudgetWithRequiredEvents selects the same immutable event
// lineages from both trajectories while enforcing the hard budget on each side.
// It is intended for controlled transformations whose event IDs may change but
// whose source lineage remains stable. Required call/result pairs remain atomic.
func ApplyPairedEvidenceBudgetWithRequiredEvents(left, right Trajectory, tokenBudget int, leftRequiredEventIDs, rightRequiredEventIDs []string) (Trajectory, Trajectory, error) {
	if tokenBudget < 0 {
		return Trajectory{}, Trajectory{}, errors.New("paired evidence token budget cannot be negative")
	}
	if len(leftRequiredEventIDs) == 0 || len(rightRequiredEventIDs) == 0 {
		return Trajectory{}, Trajectory{}, errors.New("paired evidence selection requires anchors on both sides")
	}
	leftRequired, err := requiredEventIndexes(left.Events, leftRequiredEventIDs)
	if err != nil {
		return Trajectory{}, Trajectory{}, fmt.Errorf("left paired evidence: %w", err)
	}
	rightRequired, err := requiredEventIndexes(right.Events, rightRequiredEventIDs)
	if err != nil {
		return Trajectory{}, Trajectory{}, fmt.Errorf("right paired evidence: %w", err)
	}
	leftUnits, leftScores := buildSelectionUnits(left)
	rightUnits, rightScores := buildSelectionUnits(right)
	paired, err := pairSelectionUnits(left.Events, right.Events, leftUnits, rightUnits, leftRequired, rightRequired)
	if err != nil {
		return Trajectory{}, Trajectory{}, err
	}
	leftSelected, rightSelected, err := selectPairedUnits(left.Events, right.Events, paired, tokenBudget)
	if err != nil {
		return Trajectory{}, Trajectory{}, err
	}
	leftRetained, err := materializePairedSelection(left, leftSelected, leftScores, tokenBudget)
	if err != nil {
		return Trajectory{}, Trajectory{}, fmt.Errorf("left paired evidence: %w", err)
	}
	rightRetained, err := materializePairedSelection(right, rightSelected, rightScores, tokenBudget)
	if err != nil {
		return Trajectory{}, Trajectory{}, fmt.Errorf("right paired evidence: %w", err)
	}
	return leftRetained, rightRetained, nil
}

func pairSelectionUnits(leftEvents, rightEvents []Event, leftUnits, rightUnits []selectionUnit, leftRequired, rightRequired map[int]bool) ([]pairedSelectionUnit, error) {
	leftByKey, err := selectionUnitsByLineage(leftEvents, leftUnits)
	if err != nil {
		return nil, fmt.Errorf("left paired evidence: %w", err)
	}
	rightByKey, err := selectionUnitsByLineage(rightEvents, rightUnits)
	if err != nil {
		return nil, fmt.Errorf("right paired evidence: %w", err)
	}
	if len(leftByKey) != len(rightByKey) {
		return nil, errors.New("paired evidence trajectories have different selection-unit lineages")
	}
	paired := make([]pairedSelectionUnit, 0, len(leftByKey))
	for key, leftUnit := range leftByKey {
		rightUnit, exists := rightByKey[key]
		if !exists {
			return nil, errors.New("paired evidence trajectories have different selection-unit lineages")
		}
		leftUnitRequired, rightUnitRequired := false, false
		for _, index := range leftUnit.eventIndexes {
			leftUnitRequired = leftUnitRequired || leftRequired[index]
		}
		for _, index := range rightUnit.eventIndexes {
			rightUnitRequired = rightUnitRequired || rightRequired[index]
		}
		if leftUnitRequired != rightUnitRequired {
			return nil, errors.New("paired evidence anchors identify different immutable lineages")
		}
		unit := pairedSelectionUnit{key: key, left: leftUnit, right: rightUnit, score: max(leftUnit.score, rightUnit.score), required: leftUnitRequired}
		paired = append(paired, unit)
	}
	sort.Slice(paired, func(i, j int) bool {
		if paired[i].score == paired[j].score {
			return paired[i].key < paired[j].key
		}
		return paired[i].score > paired[j].score
	})
	return paired, nil
}

func selectionUnitsByLineage(events []Event, units []selectionUnit) (map[string]selectionUnit, error) {
	result := make(map[string]selectionUnit, len(units))
	for _, unit := range units {
		keys := make([]string, len(unit.eventIndexes))
		for index, eventIndex := range unit.eventIndexes {
			keys[index] = eventLineageKey(events[eventIndex])
		}
		sort.Strings(keys)
		key := strings.Join(keys, "\x00")
		if _, duplicate := result[key]; duplicate {
			return nil, errors.New("paired evidence trajectory has duplicate selection-unit lineage")
		}
		result[key] = unit
	}
	return result, nil
}

func eventLineageKey(event Event) string {
	return digestBytes([]byte(eventSourceIdentity(event)))
}

func selectPairedUnits(leftEvents, rightEvents []Event, units []pairedSelectionUnit, tokenBudget int) (map[int]bool, map[int]bool, error) {
	leftSelected, rightSelected := make(map[int]bool), make(map[int]bool)
	leftBytes, rightBytes, limitBytes := 0, 0, tokenBudget*4
	for _, mandatory := range []bool{true, false} {
		for _, unit := range units {
			if unit.required != mandatory {
				continue
			}
			leftAdditional := selectionUnitBytes(leftEvents, unit.left, len(leftSelected) > 0)
			rightAdditional := selectionUnitBytes(rightEvents, unit.right, len(rightSelected) > 0)
			if tokenBudget != 0 && (leftBytes+leftAdditional > limitBytes || rightBytes+rightAdditional > limitBytes) {
				if mandatory {
					return nil, nil, errors.New("required paired evidence units exceed the hard token budget")
				}
				continue
			}
			for _, index := range unit.left.eventIndexes {
				leftSelected[index] = true
			}
			for _, index := range unit.right.eventIndexes {
				rightSelected[index] = true
			}
			leftBytes += leftAdditional
			rightBytes += rightAdditional
		}
	}
	return leftSelected, rightSelected, nil
}

func selectionUnitBytes(events []Event, unit selectionUnit, hasPrevious bool) int {
	result := 0
	for _, index := range unit.eventIndexes {
		result += len(renderEvent(events[index]))
	}
	if hasPrevious {
		result += 2 * len(unit.eventIndexes)
	} else if len(unit.eventIndexes) > 1 {
		result += 2 * (len(unit.eventIndexes) - 1)
	}
	return result
}

func materializePairedSelection(source Trajectory, selected map[int]bool, scores []int, tokenBudget int) (Trajectory, error) {
	retained, truncatedID, err := materializeSelection(source, selected, scores, tokenBudget, "evalwitness.evidence-selector.paired.v1")
	if err != nil {
		return Trajectory{}, err
	}
	retainedTokens := estimateTokensForBytes(len(RenderTrajectory(retained)))
	if tokenBudget != 0 && retainedTokens > tokenBudget {
		return Trajectory{}, fmt.Errorf("paired evidence selector retained %d tokens over %d-token budget", retainedTokens, tokenBudget)
	}
	retained.Report.Truncation = TruncationBoundary{
		Applied: len(retained.Events) != len(source.Events), BudgetTokens: tokenBudget, RetainedTokens: retainedTokens,
		OriginalDigest: source.Digest, RetainedDigest: retained.Digest,
	}
	if retained.Report.Truncation.Applied {
		retained.Report.Truncation.EventID = truncatedID
		retained.Report.Truncation.Reason = "paired canonical evidence exceeded configured budget"
	}
	if err := recomputeTrajectoryDigest(&retained); err != nil {
		return Trajectory{}, err
	}
	retained.Report.Truncation.RetainedDigest = retained.Digest
	return retained, nil
}

func applyEvidenceBudget(source Trajectory, tokenBudget int, requiredEventIDs []string) (Trajectory, error) {
	if tokenBudget < 0 {
		return Trajectory{}, errors.New("evidence token budget cannot be negative")
	}
	required, err := requiredEventIndexes(source.Events, requiredEventIDs)
	if err != nil {
		return Trajectory{}, err
	}
	fullText := RenderTrajectory(source)
	fullTokens := estimateTokensForBytes(len(fullText))
	if tokenBudget == 0 || fullTokens <= tokenBudget {
		result := source
		result.Report.Truncation = TruncationBoundary{
			Applied: false, BudgetTokens: tokenBudget, RetainedTokens: fullTokens,
			OriginalDigest: source.Digest, RetainedDigest: source.Digest,
		}
		return result, nil
	}
	if len(source.Events) == 0 {
		return Trajectory{}, errors.New("cannot apply evidence budget to an empty trajectory")
	}
	units, scores := buildSelectionUnits(source)
	selected, err := selectWholeUnits(source.Events, units, tokenBudget, required)
	if err != nil {
		return Trajectory{}, err
	}
	selectorVersion := "evalwitness.evidence-selector.v1"
	if len(required) > 0 {
		selectorVersion = "evalwitness.evidence-selector.v2"
	}
	retained, truncatedID, err := materializeSelection(source, selected, scores, tokenBudget, selectorVersion)
	if err != nil {
		return Trajectory{}, err
	}
	retainedText := RenderTrajectory(retained)
	retainedTokens := estimateTokensForBytes(len(retainedText))
	if retainedTokens > tokenBudget {
		return Trajectory{}, fmt.Errorf("evidence selector retained %d tokens over %d-token budget", retainedTokens, tokenBudget)
	}
	retained.Report.Truncation = TruncationBoundary{
		Applied: true, BudgetTokens: tokenBudget, RetainedTokens: retainedTokens,
		EventID: truncatedID, Reason: "canonical evidence exceeded configured budget",
		OriginalDigest: source.Digest, RetainedDigest: retained.Digest,
	}
	if err := recomputeTrajectoryDigest(&retained); err != nil {
		return Trajectory{}, err
	}
	retained.Report.Truncation.RetainedDigest = retained.Digest
	return retained, nil
}

func buildSelectionUnits(trajectory Trajectory) ([]selectionUnit, []int) {
	count := len(trajectory.Events)
	parent := make([]int, count)
	indexByID := make(map[string]int, count)
	scores := make([]int, count)
	for index, event := range trajectory.Events {
		parent[index] = index
		indexByID[event.ID] = index
		scores[index] = evidenceEventScore(event, index, count)
	}
	for _, link := range trajectory.Links {
		if link.Kind != LinkCallResult {
			continue
		}
		from, fromOK := indexByID[link.FromID]
		to, toOK := indexByID[link.ToID]
		if fromOK && toOK {
			unionSelection(parent, from, to)
		}
	}
	byRoot := make(map[int]*selectionUnit)
	for index, event := range trajectory.Events {
		root := findSelectionRoot(parent, index)
		unit := byRoot[root]
		if unit == nil {
			unit = &selectionUnit{firstOrder: event.Order}
			byRoot[root] = unit
		}
		unit.eventIndexes = append(unit.eventIndexes, index)
		unit.score += scores[index]
	}
	units := make([]selectionUnit, 0, len(byRoot))
	for _, unit := range byRoot {
		if len(unit.eventIndexes) > 1 {
			unit.score += 8
		}
		units = append(units, *unit)
	}
	sort.SliceStable(units, func(i, j int) bool {
		if units[i].score == units[j].score {
			return units[i].firstOrder < units[j].firstOrder
		}
		return units[i].score > units[j].score
	})
	return units, scores
}

func findSelectionRoot(parent []int, index int) int {
	for parent[index] != index {
		parent[index] = parent[parent[index]]
		index = parent[index]
	}
	return index
}

func unionSelection(parent []int, left, right int) {
	leftRoot := findSelectionRoot(parent, left)
	rightRoot := findSelectionRoot(parent, right)
	if leftRoot != rightRoot {
		parent[rightRoot] = leftRoot
	}
}

func requiredEventIndexes(events []Event, requiredEventIDs []string) (map[int]bool, error) {
	byID := make(map[string]int, len(events))
	for index, event := range events {
		byID[event.ID] = index
	}
	required := make(map[int]bool, len(requiredEventIDs))
	for _, eventID := range requiredEventIDs {
		index, exists := byID[eventID]
		if !exists {
			return nil, fmt.Errorf("required evidence event %q is absent", eventID)
		}
		required[index] = true
	}
	return required, nil
}

func selectWholeUnits(events []Event, units []selectionUnit, tokenBudget int, required map[int]bool) (map[int]bool, error) {
	selected := make(map[int]bool)
	usedBytes := 0
	limitBytes := tokenBudget * 4
	selectUnit := func(unit selectionUnit, mandatory bool) error {
		additional := 0
		for _, index := range unit.eventIndexes {
			additional += len(renderEvent(events[index]))
		}
		if len(selected) > 0 {
			additional += 2 * len(unit.eventIndexes)
		} else if len(unit.eventIndexes) > 1 {
			additional += 2 * (len(unit.eventIndexes) - 1)
		}
		if usedBytes+additional > limitBytes {
			if mandatory {
				return errors.New("required evidence units exceed the hard token budget")
			}
			return nil
		}
		for _, index := range unit.eventIndexes {
			selected[index] = true
		}
		usedBytes += additional
		return nil
	}
	for _, mandatory := range []bool{true, false} {
		for _, unit := range units {
			unitRequired := false
			for _, index := range unit.eventIndexes {
				unitRequired = unitRequired || required[index]
			}
			if unitRequired != mandatory {
				continue
			}
			if err := selectUnit(unit, mandatory); err != nil {
				return nil, err
			}
		}
	}
	return selected, nil
}

func materializeSelection(source Trajectory, selected map[int]bool, scores []int, tokenBudget int, selectorVersion string) (Trajectory, string, error) {
	result, err := cloneTrajectory(source)
	if err != nil {
		return Trajectory{}, "", err
	}
	firstChanged := ""
	if len(selected) == 0 {
		index := highestScoreIndex(scores)
		truncated, ok, err := truncateEventToBudget(source.Events[index], tokenBudget)
		if err != nil {
			return Trajectory{}, "", err
		}
		if !ok {
			return Trajectory{}, "", fmt.Errorf("no canonical event fits %d-token evidence budget", tokenBudget)
		}
		result.Events = []Event{truncated}
		selected[index] = true
		firstChanged = source.Events[index].ID
	} else {
		result.Events = selectedEvents(source.Events, selected)
	}
	result.Links = selectedLinks(source.Links, result.Events)
	result.Derivation = &Derivation{
		ParentDigest: source.Digest, Relation: "evidence_slice", Validator: selectorVersion,
		ChangedEventIDs: droppedEventIDs(source.Events, selected),
	}
	if firstChanged != "" {
		result.Derivation.ChangedEventIDs = append(result.Derivation.ChangedEventIDs, firstChanged)
	}
	result.Report.Selection = selectionReport(source, result, selected, scores, firstChanged)
	result.Report.Categories = retainedCategories(source.Events, result.Events)
	result.Report.RetainedBytes = retainedEventBytes(result.Events)
	result.Report.CanonicalEvents = len(source.Events)
	if firstChanged == "" {
		firstChanged = firstDroppedEvent(source.Events, selected)
	}
	if err := recomputeTrajectoryDigest(&result); err != nil {
		return Trajectory{}, "", err
	}
	return result, firstChanged, nil
}

func highestScoreIndex(scores []int) int {
	best := 0
	for index := 1; index < len(scores); index++ {
		if scores[index] > scores[best] {
			best = index
		}
	}
	return best
}

func truncateEventToBudget(source Event, tokenBudget int) (Event, bool, error) {
	if tokenBudget <= 0 {
		return Event{}, false, nil
	}
	totalMutable := mutableEventBytes(source)
	if totalMutable == 0 {
		return Event{}, false, nil
	}
	low, high := 0, totalMutable
	var best Event
	found := false
	for low <= high {
		mid := low + (high-low)/2
		candidate, err := boundEventContent(source, mid)
		if err != nil {
			return Event{}, false, err
		}
		if estimateTokensForBytes(len(renderEvent(candidate))) <= tokenBudget {
			best = candidate
			found = true
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if !found {
		return Event{}, false, nil
	}
	encoded, err := json.Marshal(eventPayloadMaterial(best))
	if err != nil {
		return Event{}, false, fmt.Errorf("encode truncated event payload: %w", err)
	}
	best.ContentDigest = digestBytes(encoded)
	best.RetainedBytes = mutableEventBytes(best)
	best.EstimatedTokens = estimateTokensForBytes(best.RetainedBytes)
	return best, true, nil
}

func boundEventContent(source Event, maximum int) (Event, error) {
	event, err := cloneEvent(source)
	if err != nil {
		return Event{}, err
	}
	total := mutableEventBytes(event)
	if total <= maximum {
		return event, nil
	}
	remainingSource := total
	remainingBudget := maximum
	apply := func(value *string) {
		if value == nil || *value == "" {
			return
		}
		allocation := 0
		if remainingSource > 0 {
			allocation = remainingBudget * len(*value) / remainingSource
		}
		if allocation > remainingBudget {
			allocation = remainingBudget
		}
		original := len(*value)
		*value = truncateUTF8(*value, allocation)
		remainingSource -= original
		remainingBudget -= len(*value)
	}
	visitEventStrings(&event, apply)
	return event, nil
}

func mutableEventBytes(event Event) int {
	total := 0
	visitEventStrings(&event, func(value *string) { total += len(*value) })
	return total
}

func visitEventStrings(event *Event, visit func(*string)) {
	if event.Message != nil {
		for index := range event.Message.Parts {
			visit(&event.Message.Parts[index].Text)
		}
	}
	if event.ToolCall != nil {
		visit(&event.ToolCall.Arguments)
	}
	if event.ToolResult != nil {
		for index := range event.ToolResult.Stdout {
			visit(&event.ToolResult.Stdout[index].Text)
		}
		for index := range event.ToolResult.Stderr {
			visit(&event.ToolResult.Stderr[index].Text)
		}
		for index := range event.ToolResult.Output {
			visit(&event.ToolResult.Output[index].Text)
		}
	}
	if event.Command != nil {
		visit(&event.Command.Display)
	}
	if event.Output != nil {
		visit(&event.Output.Text)
	}
	if event.FileChange != nil {
		visit(&event.FileChange.Diff)
	}
	if event.Error != nil {
		visit(&event.Error.SafeMessage)
	}
	if event.Metadata != nil {
		visit(&event.Metadata.Value)
	}
}

func truncateUTF8(value string, maximum int) string {
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "�")
	}
	if maximum <= 0 {
		return ""
	}
	if len(value) <= maximum {
		return value
	}
	marker := "\n[... field truncated ...]\n"
	if maximum <= len(marker) {
		return validUTF8Prefix(value, maximum)
	}
	available := maximum - len(marker)
	headBytes := available / 3
	tailBytes := available - headBytes
	head := validUTF8Prefix(value, headBytes)
	tail := validUTF8Suffix(value, tailBytes)
	return head + marker + tail
}

func validUTF8Prefix(value string, maximum int) string {
	if maximum >= len(value) {
		return value
	}
	for maximum > 0 && !utf8.ValidString(value[:maximum]) {
		maximum--
	}
	return value[:maximum]
}

func validUTF8Suffix(value string, maximum int) string {
	if maximum >= len(value) {
		return value
	}
	start := len(value) - maximum
	for start < len(value) && !utf8.RuneStart(value[start]) {
		start++
	}
	return value[start:]
}

func evidenceEventScore(event Event, index, total int) int {
	score := 0
	switch event.Kind {
	case EventError:
		score = 30
	case EventFileChange:
		score = 24
	case EventToolResult:
		score = 20
	case EventCommand:
		score = 18
	case EventOutput:
		score = 16
	case EventToolCall:
		score = 14
	case EventMessage:
		score = 8
	case EventAttachment:
		score = 4
	case EventReasoning:
		score = 1
	case EventMetadata:
		score = 0
	}
	text := strings.ToLower(renderEvent(event))
	if containsAny(text, "error", "failed", "fatal", "panic", "exception", "traceback", "exit=1") {
		score += 16
	}
	if containsAny(text, "passed", "go test", "cargo test", "pytest", "vitest", "jest") {
		score += 10
	}
	if index < 2 || index >= total-4 {
		score += 4
	}
	return score
}

func selectedEvents(events []Event, selected map[int]bool) []Event {
	result := make([]Event, 0, len(selected))
	for index, event := range events {
		if selected[index] {
			result = append(result, event)
		}
	}
	return result
}

func selectedLinks(links []Link, events []Event) []Link {
	ids := make(map[string]struct{}, len(events))
	for _, event := range events {
		ids[event.ID] = struct{}{}
	}
	result := make([]Link, 0, len(links))
	for _, link := range links {
		_, from := ids[link.FromID]
		_, to := ids[link.ToID]
		if from && to {
			result = append(result, link)
		}
	}
	return result
}

func droppedEventIDs(events []Event, selected map[int]bool) []string {
	result := make([]string, 0, len(events)-len(selected))
	for index, event := range events {
		if !selected[index] {
			result = append(result, event.ID)
		}
	}
	return result
}

func firstDroppedEvent(events []Event, selected map[int]bool) string {
	for index, event := range events {
		if !selected[index] {
			return event.ID
		}
	}
	return ""
}

func selectionReport(source, retained Trajectory, selected map[int]bool, scores []int, truncatedID string) []EventSelection {
	retainedByID := make(map[string]Event, len(retained.Events))
	for _, event := range retained.Events {
		retainedByID[event.ID] = event
	}
	linked := callResultMembership(source.Links)
	report := make([]EventSelection, 0, len(source.Events))
	for index, event := range source.Events {
		entry := EventSelection{EventID: event.ID, Score: scores[index], OriginalTokens: event.EstimatedTokens}
		if kept, ok := retainedByID[event.ID]; ok {
			entry.RetainedTokens = estimateTokensForBytes(len(renderEvent(kept)))
			entry.Disposition = DispositionRepresented
			entry.Reason = "selected by deterministic evidence policy"
			if event.ID == truncatedID {
				entry.Disposition = DispositionTruncated
				entry.Reason = "highest-ranked event was field-truncated to fit the hard budget"
			}
		} else {
			entry.Disposition = DispositionTruncated
			entry.Reason = "dropped by hard evidence budget"
		}
		if peer := linked[event.ID]; peer != "" {
			_, eventKept := retainedByID[event.ID]
			_, peerKept := retainedByID[peer]
			switch {
			case eventKept && peerKept:
				entry.LinkedConstraint = "call_result_pair_retained"
			case !eventKept && !peerKept:
				entry.LinkedConstraint = "call_result_pair_dropped"
			default:
				entry.LinkedConstraint = "call_result_pair_partial"
			}
		}
		_ = selected
		report = append(report, entry)
	}
	return report
}

func callResultMembership(links []Link) map[string]string {
	result := make(map[string]string)
	for _, link := range links {
		if link.Kind == LinkCallResult {
			result[link.FromID] = link.ToID
			result[link.ToID] = link.FromID
		}
	}
	return result
}

func retainedCategories(original, retained []Event) []CategoryRetention {
	retainedByID := make(map[string]Event, len(retained))
	for _, event := range retained {
		retainedByID[event.ID] = event
	}
	byKind := make(map[EventKind]*CategoryRetention)
	for _, event := range original {
		entry := byKind[event.Kind]
		if entry == nil {
			entry = &CategoryRetention{Kind: event.Kind}
			byKind[event.Kind] = entry
		}
		entry.OriginalEvents++
		entry.OriginalBytes += event.ContentBytes
		if kept, ok := retainedByID[event.ID]; ok {
			entry.RetainedEvents++
			entry.RetainedBytes += kept.RetainedBytes
		}
	}
	kinds := make([]string, 0, len(byKind))
	for kind := range byKind {
		kinds = append(kinds, string(kind))
	}
	sort.Strings(kinds)
	result := make([]CategoryRetention, 0, len(kinds))
	for _, kind := range kinds {
		result = append(result, *byKind[EventKind(kind)])
	}
	return result
}

func retainedEventBytes(events []Event) int {
	total := 0
	for _, event := range events {
		total += event.RetainedBytes
	}
	return total
}

func cloneTrajectory(source Trajectory) (Trajectory, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return Trajectory{}, fmt.Errorf("clone trajectory: %w", err)
	}
	var result Trajectory
	if err := json.Unmarshal(encoded, &result); err != nil {
		return Trajectory{}, fmt.Errorf("clone trajectory: %w", err)
	}
	return result, nil
}

func cloneEvent(source Event) (Event, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return Event{}, fmt.Errorf("clone event: %w", err)
	}
	var result Event
	if err := json.Unmarshal(encoded, &result); err != nil {
		return Event{}, fmt.Errorf("clone event: %w", err)
	}
	return result, nil
}

func recomputeTrajectoryDigest(trajectory *Trajectory) error {
	encoded, err := json.Marshal(trajectoryDigestMaterial(*trajectory))
	if err != nil {
		return fmt.Errorf("encode canonical trajectory: %w", err)
	}
	trajectory.Digest = digestBytes(encoded)
	return trajectory.Validate()
}
