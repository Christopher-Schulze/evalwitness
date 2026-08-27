package reliance

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

type selectorFactorRetention struct {
	assignments  int
	minimumScore int
	maximumScore int
	budgets      map[int]*SelectorFactorBudgetAudit
}

type selectorCategoryKey struct {
	budget int
	kind   preprocess.EventKind
}

type selectorRetention struct {
	factors    map[FactorID]*selectorFactorRetention
	categories []SelectorCategoryBudgetAudit
}

func collectSelectorRetention(
	sources []EvidenceSelectorAuditSource,
	budgets []int,
) (selectorRetention, error) {
	result := selectorRetention{factors: make(map[FactorID]*selectorFactorRetention)}
	categories := make(map[selectorCategoryKey]*SelectorCategoryBudgetAudit)
	for _, source := range sources {
		scores, err := selectorEventScores(source.Trajectory)
		if err != nil {
			return selectorRetention{}, err
		}
		recordSelectorAssignments(result.factors, source, scores, budgets)
		for _, budget := range budgets {
			retained, err := preprocess.ApplyEvidenceBudget(source.Trajectory, budget)
			if err != nil {
				return selectorRetention{}, fmt.Errorf("selector audit task %q budget %d: %w", source.SourceTaskID, budget, err)
			}
			recordSelectorCategoryRetention(categories, source.Trajectory, retained, budget)
			if err := recordSelectorTargetRetention(result.factors, source, retained, budget); err != nil {
				return selectorRetention{}, err
			}
		}
	}
	result.categories = orderedSelectorCategories(categories)
	return result, nil
}

func selectorEventScores(trajectory preprocess.Trajectory) (map[string]int, error) {
	inspections, err := preprocess.InspectEvidenceEventScores(trajectory)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(inspections))
	for _, inspection := range inspections {
		result[inspection.EventID] = inspection.Score
	}
	return result, nil
}

func recordSelectorAssignments(
	factors map[FactorID]*selectorFactorRetention,
	source EvidenceSelectorAuditSource,
	scores map[string]int,
	budgets []int,
) {
	events := indexEvents(source.Trajectory.Events)
	seenEvents := make(map[string]struct{})
	for _, assignment := range source.Assignments.Assignments {
		factor := selectorFactorRetentionFor(factors, assignment.FactorID, budgets)
		factor.assignments++
		score := scores[assignment.EventID]
		if factor.assignments == 1 || score < factor.minimumScore {
			factor.minimumScore = score
		}
		if factor.assignments == 1 || score > factor.maximumScore {
			factor.maximumScore = score
		}
		key := string(assignment.FactorID) + "\x00" + assignment.EventID
		if _, duplicate := seenEvents[key]; duplicate {
			continue
		}
		seenEvents[key] = struct{}{}
		for _, budget := range budgets {
			factor.budgets[budget].AssignedEventBytes += events[assignment.EventID].RetainedBytes
			factor.budgets[budget].AssignedRenderedBytes += len(preprocess.RenderCanonicalEvidenceEvent(events[assignment.EventID]))
		}
	}
}

func selectorFactorRetentionFor(
	factors map[FactorID]*selectorFactorRetention,
	factorID FactorID,
	budgets []int,
) *selectorFactorRetention {
	value := factors[factorID]
	if value != nil {
		return value
	}
	value = &selectorFactorRetention{budgets: make(map[int]*SelectorFactorBudgetAudit, len(budgets))}
	for _, budget := range budgets {
		value.budgets[budget] = &SelectorFactorBudgetAudit{BudgetTokens: budget, RiskFlags: []SelectorRiskFlag{}}
	}
	factors[factorID] = value
	return value
}

func recordSelectorTargetRetention(
	factors map[FactorID]*selectorFactorRetention,
	source EvidenceSelectorAuditSource,
	retained preprocess.Trajectory,
	budget int,
) error {
	retainedEvents := indexEvents(retained.Events)
	seenEvents := make(map[string]struct{})
	for _, assignment := range source.Assignments.Assignments {
		factor := factors[assignment.FactorID]
		result := factor.budgets[budget]
		result.AssignmentTargets++
		retainedEvent, found := retainedEvents[assignment.EventID]
		if !found {
			result.DroppedTargets++
			continue
		}
		rendered, err := selectorTargetRendered(retainedEvent, assignment.FieldPath)
		if err != nil {
			return err
		}
		if !rendered {
			result.UnrenderedTargets++
		} else {
			recordSelectorTargetFidelity(result, retainedEvent, assignment)
		}
		key := string(assignment.FactorID) + "\x00" + assignment.EventID
		if _, duplicate := seenEvents[key]; !duplicate {
			seenEvents[key] = struct{}{}
			result.RetainedAssignedEventBytes += retainedEvent.RetainedBytes
			result.RetainedAssignedRenderedBytes += len(preprocess.RenderCanonicalEvidenceEvent(retainedEvent))
		}
	}
	return nil
}

func selectorTargetRendered(event preprocess.Event, path preprocess.FieldPath) (bool, error) {
	target, found := canonicalTarget(event.Kind, path)
	if !found {
		return false, fmt.Errorf("selector visibility probe has unknown target %s%s", event.Kind, path)
	}
	original := preprocess.RenderCanonicalEvidenceEvent(event)
	probes := selectorVisibilityProbes(target.ValueKind)
	if len(probes) == 0 {
		return false, fmt.Errorf("selector visibility probe has unknown value kind %q", target.ValueKind)
	}
	for _, probe := range probes {
		candidate, err := cloneSelectorProbeEvent(event)
		if err != nil {
			return false, err
		}
		if err := replaceInterventionField(&candidate, path, probe); err != nil {
			return false, err
		}
		if preprocess.RenderCanonicalEvidenceEvent(candidate) != original {
			return true, nil
		}
	}
	return false, nil
}

func cloneSelectorProbeEvent(event preprocess.Event) (preprocess.Event, error) {
	result := event
	switch event.Kind {
	case preprocess.EventCommand:
		payload := *event.Command
		result.Command = &payload
	case preprocess.EventError:
		payload := *event.Error
		result.Error = &payload
	case preprocess.EventEvaluation:
		payload := *event.Evaluation
		result.Evaluation = &payload
	case preprocess.EventFileChange:
		payload := *event.FileChange
		result.FileChange = &payload
	case preprocess.EventMessage:
		payload := *event.Message
		payload.Parts = slices.Clone(payload.Parts)
		result.Message = &payload
	case preprocess.EventMetadata:
		payload := *event.Metadata
		result.Metadata = &payload
	case preprocess.EventOutput:
		payload := *event.Output
		result.Output = &payload
	case preprocess.EventToolCall:
		payload := *event.ToolCall
		result.ToolCall = &payload
	case preprocess.EventToolResult:
		payload := *event.ToolResult
		payload.Stdout, payload.Stderr, payload.Output = slices.Clone(payload.Stdout), slices.Clone(payload.Stderr), slices.Clone(payload.Output)
		result.ToolResult = &payload
	default:
		return preprocess.Event{}, fmt.Errorf("selector visibility probe cannot clone event kind %q", event.Kind)
	}
	return result, nil
}

func selectorVisibilityProbes(kind FieldValueKind) []InterventionValue {
	firstText, secondText := "evalwitness-render-probe-a", "evalwitness-render-probe-b"
	firstInteger, secondInteger := 1_000_000_007, -1_000_000_007
	firstBoolean, secondBoolean := true, false
	firstParts := []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: firstText}}
	secondParts := []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: secondText}}
	switch kind {
	case ValueText:
		return []InterventionValue{{Text: &firstText}, {Text: &secondText}}
	case ValueInteger:
		return []InterventionValue{{Integer: &firstInteger}, {Integer: &secondInteger}}
	case ValueBoolean:
		return []InterventionValue{{Boolean: &firstBoolean}, {Boolean: &secondBoolean}}
	case ValueContentParts:
		return []InterventionValue{{ContentParts: &firstParts}, {ContentParts: &secondParts}}
	default:
		return nil
	}
}

func recordSelectorTargetFidelity(
	result *SelectorFactorBudgetAudit,
	event preprocess.Event,
	assignment EvidenceAssignment,
) {
	_, digest, err := eventFieldMaterial(event, assignment.FieldPath)
	if err != nil || digest != assignment.ValueDigest {
		result.ChangedTargets++
		return
	}
	result.ExactTargets++
}

func recordSelectorCategoryRetention(
	categories map[selectorCategoryKey]*SelectorCategoryBudgetAudit,
	original preprocess.Trajectory,
	retained preprocess.Trajectory,
	budget int,
) {
	for _, event := range original.Events {
		value := selectorCategoryRetentionFor(categories, budget, event.Kind)
		value.OriginalEvents++
		value.OriginalBytes += event.RetainedBytes
	}
	for _, event := range retained.Events {
		value := selectorCategoryRetentionFor(categories, budget, event.Kind)
		value.RetainedEvents++
		value.RetainedBytes += event.RetainedBytes
	}
}

func selectorCategoryRetentionFor(
	categories map[selectorCategoryKey]*SelectorCategoryBudgetAudit,
	budget int,
	kind preprocess.EventKind,
) *SelectorCategoryBudgetAudit {
	key := selectorCategoryKey{budget, kind}
	value := categories[key]
	if value == nil {
		value = &SelectorCategoryBudgetAudit{BudgetTokens: budget, EventKind: kind}
		categories[key] = value
	}
	return value
}

func orderedSelectorCategories(
	categories map[selectorCategoryKey]*SelectorCategoryBudgetAudit,
) []SelectorCategoryBudgetAudit {
	keys := make([]selectorCategoryKey, 0, len(categories))
	for key := range categories {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool {
		if keys[left].budget != keys[right].budget {
			return keys[left].budget < keys[right].budget
		}
		return keys[left].kind < keys[right].kind
	})
	result := make([]SelectorCategoryBudgetAudit, len(keys))
	for index, key := range keys {
		result[index] = *categories[key]
	}
	return result
}

func selectorFactorBudgets(
	observed *selectorFactorRetention,
	effectStatus SelectorEffectStatus,
) []SelectorFactorBudgetAudit {
	budgets := selectorAuditBudgets()
	result := make([]SelectorFactorBudgetAudit, len(budgets))
	for index, budget := range budgets {
		value := *observed.budgets[budget]
		switch {
		case effectStatus == SelectorEffectInconclusive:
			value.RiskFlags = []SelectorRiskFlag{SelectorRiskInconclusive}
		case effectStatus == SelectorEffectDetected && value.ChangedTargets+value.UnrenderedTargets+value.DroppedTargets > 0:
			value.RiskFlags = []SelectorRiskFlag{SelectorRiskDetectedEffectNonexact}
		case effectStatus == SelectorEffectNotDetected && value.ExactTargets+value.ChangedTargets > 0:
			value.RiskFlags = []SelectorRiskFlag{SelectorRiskUndetectedEffectRetained}
		}
		result[index] = value
	}
	return result
}
