package lineage

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

type FindingStatus string

const (
	FindingProven       FindingStatus = "proven"
	FindingDisproven    FindingStatus = "disproven"
	FindingNotEvaluated FindingStatus = "not_evaluated"
)

type TerminalFinding struct {
	State          TerminalState `json:"state"`
	Status         FindingStatus `json:"status"`
	ProofRecordIDs []string      `json:"proof_record_ids"`
}

type ClassificationInput struct {
	UnitID                   string            `json:"unit_id"`
	TaskGroupID              string            `json:"task_group_id"`
	Format                   string            `json:"format"`
	SourceSessionID          string            `json:"source_session_id"`
	CandidateEvidenceEvents  int               `json:"candidate_evidence_events"`
	DirectInvocationObserved bool              `json:"direct_invocation_observed"`
	Findings                 []TerminalFinding `json:"findings"`
}

type ClassifiedLineageUnit struct {
	UnitID                   string           `json:"unit_id"`
	TaskGroupID              string           `json:"task_group_id"`
	Format                   string           `json:"format"`
	SourceSessionID          string           `json:"source_session_id"`
	CandidateEvidenceEvents  int              `json:"candidate_evidence_events"`
	DirectInvocationObserved bool             `json:"direct_invocation_observed"`
	TerminalState            TerminalState    `json:"terminal_state"`
	StateDisposition         StateDisposition `json:"state_disposition"`
	PrecedenceApplied        int              `json:"precedence_applied"`
	ProofRecordIDs           []string         `json:"proof_record_ids"`
	LayerSurvival            []bool           `json:"layer_survival"`
}

type ClassificationSummary struct {
	ConsideredTaskGroups int            `json:"considered_task_groups"`
	IncludedTaskGroups   int            `json:"included_task_groups"`
	ExcludedTaskGroups   int            `json:"excluded_task_groups"`
	UnresolvedTaskGroups int            `json:"unresolved_task_groups"`
	Formats              []FormatResult `json:"formats"`
}

func BuildLineageAudit(header ArtifactHeader, auditID, attemptLedgerDigest, holdoutResultsDigest string, claimBoundary []string, units []ClassifiedLineageUnit) (LineageAudit, error) {
	if header.Digest != "" {
		return LineageAudit{}, errors.New("lineage audit builder requires an unsealed header")
	}
	assessmentParents := 0
	for _, parent := range header.Parents {
		if parent.Relation == "assessment" {
			assessmentParents++
		}
	}
	if assessmentParents != len(units) {
		return LineageAudit{}, errors.New("lineage audit requires exactly one assessment parent per classified unit")
	}
	summary, err := SummarizeClassifications(units)
	if err != nil {
		return LineageAudit{}, err
	}
	if err := summary.Validate(); err != nil {
		return LineageAudit{}, err
	}
	boundary := append([]string(nil), claimBoundary...)
	sort.Strings(boundary)
	audit := LineageAudit{
		Header: header, AuditID: auditID, AttemptLedgerDigest: attemptLedgerDigest, HoldoutResultsDigest: holdoutResultsDigest,
		ClaimBoundary: boundary, ConsideredTaskGroups: summary.ConsideredTaskGroups, IncludedTaskGroups: summary.IncludedTaskGroups, ExcludedTaskGroups: summary.ExcludedTaskGroups,
		UnresolvedTaskGroups: summary.UnresolvedTaskGroups, Formats: summary.Formats,
		BootstrapSeed: 20260810, BootstrapReplicates: 10000, ConservationPassed: true,
	}
	audit.Header.Digest, err = artifactDigest(audit)
	if err != nil {
		return LineageAudit{}, err
	}
	return audit, audit.Validate()
}

func ClassifyEarliestLoss(input ClassificationInput) (ClassifiedLineageUnit, error) {
	if missing(input.UnitID, input.TaskGroupID, input.Format, input.SourceSessionID) || input.CandidateEvidenceEvents < 0 {
		return ClassifiedLineageUnit{}, errors.New("classification unit identity or event count is invalid")
	}
	contract := terminalStateContract()
	if len(input.Findings) != len(contract) {
		return ClassifiedLineageUnit{}, errors.New("classification requires the complete terminal-state evidence vector")
	}
	selected := -1
	for index, finding := range input.Findings {
		if finding.State != contract[index].State || !slices.Contains([]FindingStatus{FindingProven, FindingDisproven, FindingNotEvaluated}, finding.Status) {
			return ClassifiedLineageUnit{}, errors.New("classification finding is invalid or out of precedence order")
		}
		if finding.Status == FindingNotEvaluated {
			if len(finding.ProofRecordIDs) != 0 {
				return ClassifiedLineageUnit{}, errors.New("unevaluated classification finding cannot carry proof records")
			}
			if selected < 0 {
				return ClassifiedLineageUnit{}, errors.New("classification cannot skip an earlier terminal state")
			}
			continue
		}
		if err := validateSortedUnique("classification proof record IDs", finding.ProofRecordIDs, 1); err != nil {
			return ClassifiedLineageUnit{}, err
		}
		if finding.Status == FindingProven && selected < 0 {
			selected = index
		}
	}
	if selected < 0 {
		return ClassifiedLineageUnit{}, errors.New("classification has no proven terminal state")
	}
	for index := 0; index < selected; index++ {
		if input.Findings[index].Status != FindingDisproven {
			return ClassifiedLineageUnit{}, errors.New("classification did not disprove every higher-precedence state")
		}
	}
	state := contract[selected].State
	if input.DirectInvocationObserved != stateRequiresObservedInvocation(state) {
		return ClassifiedLineageUnit{}, errors.New("classification invocation observation conflicts with its terminal state")
	}
	if input.DirectInvocationObserved && input.CandidateEvidenceEvents < 1 {
		return ClassifiedLineageUnit{}, errors.New("observed invocation requires at least one candidate evidence event")
	}
	return ClassifiedLineageUnit{
		UnitID: input.UnitID, TaskGroupID: input.TaskGroupID, Format: input.Format, SourceSessionID: input.SourceSessionID,
		CandidateEvidenceEvents: input.CandidateEvidenceEvents, DirectInvocationObserved: input.DirectInvocationObserved,
		TerminalState: state, StateDisposition: contract[selected].Disposition, PrecedenceApplied: selected + 1,
		ProofRecordIDs: append([]string(nil), input.Findings[selected].ProofRecordIDs...), LayerSurvival: terminalLayerSurvival(state),
	}, nil
}

func SummarizeClassifications(units []ClassifiedLineageUnit) (ClassificationSummary, error) {
	if len(units) == 0 {
		return ClassificationSummary{}, errors.New("classification summary requires at least one unit")
	}
	ordered := append([]ClassifiedLineageUnit(nil), units...)
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].Format != ordered[right].Format {
			return ordered[left].Format < ordered[right].Format
		}
		if ordered[left].TaskGroupID != ordered[right].TaskGroupID {
			return ordered[left].TaskGroupID < ordered[right].TaskGroupID
		}
		return ordered[left].UnitID < ordered[right].UnitID
	})
	seenUnits := make(map[string]struct{}, len(ordered))
	seenFormatGroups := make(map[string]struct{}, len(ordered))
	for _, unit := range ordered {
		if err := validateClassifiedUnit(unit); err != nil {
			return ClassificationSummary{}, err
		}
		if _, found := seenUnits[unit.UnitID]; found {
			return ClassificationSummary{}, errors.New("classification unit appears more than once")
		}
		seenUnits[unit.UnitID] = struct{}{}
		key := unit.Format + "\x00" + unit.TaskGroupID
		if _, found := seenFormatGroups[key]; found {
			return ClassificationSummary{}, errors.New("format/task group has more than one terminal classification")
		}
		seenFormatGroups[key] = struct{}{}
	}

	summary := ClassificationSummary{ConsideredTaskGroups: len(ordered)}
	for start := 0; start < len(ordered); {
		end := start + 1
		for end < len(ordered) && ordered[end].Format == ordered[start].Format {
			end++
		}
		format, excluded, unresolved := summarizeFormat(ordered[start:end])
		summary.Formats = append(summary.Formats, format)
		summary.IncludedTaskGroups += format.UniqueTaskGroups
		summary.ExcludedTaskGroups += excluded
		summary.UnresolvedTaskGroups += unresolved
		start = end
	}
	return summary, nil
}

func validateClassifiedUnit(unit ClassifiedLineageUnit) error {
	if missing(unit.UnitID, unit.TaskGroupID, unit.Format, unit.SourceSessionID) || unit.CandidateEvidenceEvents < 0 ||
		len(unit.ProofRecordIDs) == 0 || len(unit.LayerSurvival) != 5 {
		return errors.New("classified lineage unit is incomplete")
	}
	contract := terminalStateContract()
	index := slices.IndexFunc(contract, func(rule TerminalStateRule) bool { return rule.State == unit.TerminalState })
	if index < 0 || unit.PrecedenceApplied != index+1 || unit.StateDisposition != contract[index].Disposition ||
		unit.DirectInvocationObserved != stateRequiresObservedInvocation(unit.TerminalState) ||
		!slices.Equal(unit.LayerSurvival, terminalLayerSurvival(unit.TerminalState)) {
		return errors.New("classified lineage unit conflicts with the terminal-state contract")
	}
	if unit.DirectInvocationObserved && unit.CandidateEvidenceEvents < 1 {
		return errors.New("classified invocation requires at least one candidate evidence event")
	}
	return validateSortedUnique("classified unit proof record IDs", unit.ProofRecordIDs, 1)
}

func summarizeFormat(units []ClassifiedLineageUnit) (FormatResult, int, int) {
	contract := terminalStateContract()
	result := FormatResult{Format: units[0].Format, ConsideredTaskGroups: len(units), TerminalCounts: make([]StateCount, len(contract))}
	for index, rule := range contract {
		result.TerminalCounts[index].State = rule.State
	}
	sessions := make(map[string]struct{})
	excluded := 0
	unresolved := 0
	for _, unit := range units {
		sessions[unit.SourceSessionID] = struct{}{}
		result.CandidateEvidenceEvents += unit.CandidateEvidenceEvents
		if unit.DirectInvocationObserved {
			result.DirectInvocations++
		}
		index := slices.IndexFunc(contract, func(rule TerminalStateRule) bool { return rule.State == unit.TerminalState })
		result.TerminalCounts[index].Count++
		if unit.TerminalState == StateInvalidCapture {
			excluded++
		}
		if slices.Contains([]TerminalState{StateUnsupportedShell, StateAmbiguousLineage, StateFreshnessUnresolved}, unit.TerminalState) {
			unresolved++
		}
	}
	result.SourceSessions = len(sessions)
	result.ExcludedTaskGroups = excluded
	result.UniqueTaskGroups = result.ConsideredTaskGroups - result.ExcludedTaskGroups
	layers := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	entered := result.UniqueTaskGroups
	for transition := 0; transition < len(layers)-1; transition++ {
		survived := 0
		for _, unit := range units {
			if unit.TerminalState != StateInvalidCapture && unit.LayerSurvival[transition+1] {
				survived++
			}
		}
		result.Flows = append(result.Flows, LayerFlow{FromLayer: layers[transition], ToLayer: layers[transition+1], Entered: entered, Survived: survived, Lost: entered - survived})
		entered = survived
	}
	result.Inferential = result.UniqueTaskGroups >= 20
	return result, excluded, unresolved
}

func stateRequiresObservedInvocation(state TerminalState) bool {
	return state != StateInvalidCapture && state != StateBehaviorAbsent
}

func terminalLayerSurvival(state TerminalState) []bool {
	survivesThrough := map[TerminalState]int{
		StateInvalidCapture: 0, StateBehaviorAbsent: 0, StateExportObservabilityAbsent: 0,
		StateAdapterMappingLoss: 1, StateUnsupportedShell: 1, StateAmbiguousLineage: 1,
		StateNonFailableVerification:          2,
		StateClaimSpecificEvidenceNotWeakened: 3, StateFreshnessUnresolved: 3,
		StateDirectVerificationInvocation: 4,
	}[state]
	layers := make([]bool, 5)
	for index := 0; index <= survivesThrough; index++ {
		layers[index] = true
	}
	if state == StateInvalidCapture || state == StateBehaviorAbsent || state == StateExportObservabilityAbsent {
		layers[0] = state != StateInvalidCapture
	}
	return layers
}

func (summary ClassificationSummary) Validate() error {
	if summary.ConsideredTaskGroups < 1 || summary.IncludedTaskGroups < 0 || summary.ExcludedTaskGroups < 0 || summary.UnresolvedTaskGroups < 0 ||
		summary.ConsideredTaskGroups != summary.IncludedTaskGroups+summary.ExcludedTaskGroups || len(summary.Formats) == 0 {
		return errors.New("classification summary identity is invalid")
	}
	total := 0
	considered := 0
	excluded := 0
	unresolved := 0
	previous := ""
	for _, format := range summary.Formats {
		if format.Format <= previous {
			return errors.New("classification summary formats are not strictly sorted")
		}
		if err := validateFormatConservation(format); err != nil {
			return fmt.Errorf("format %s: %w", format.Format, err)
		}
		total += format.UniqueTaskGroups
		considered += format.ConsideredTaskGroups
		excluded += format.TerminalCounts[0].Count
		unresolved += format.TerminalCounts[4].Count + format.TerminalCounts[5].Count + format.TerminalCounts[8].Count
		previous = format.Format
	}
	if considered != summary.ConsideredTaskGroups || total != summary.IncludedTaskGroups || excluded != summary.ExcludedTaskGroups || unresolved != summary.UnresolvedTaskGroups {
		return errors.New("classification summary totals do not conserve terminal states")
	}
	return nil
}

func validateFormatConservation(format FormatResult) error {
	if missing(format.Format) || format.ConsideredTaskGroups < 1 || format.UniqueTaskGroups < 0 || format.ExcludedTaskGroups < 0 ||
		format.ConsideredTaskGroups != format.UniqueTaskGroups+format.ExcludedTaskGroups || format.SourceSessions < 1 || format.CandidateEvidenceEvents < 0 ||
		format.DirectInvocations < 0 || format.DirectInvocations > format.CandidateEvidenceEvents || len(format.TerminalCounts) != len(terminalStateContract()) || len(format.Flows) != 4 {
		return errors.New("format result is incomplete")
	}
	total := 0
	for index, count := range format.TerminalCounts {
		if count.State != terminalStateContract()[index].State || count.Count < 0 {
			return errors.New("terminal counts violate the closed state order")
		}
		total += count.Count
	}
	if total != format.ConsideredTaskGroups || format.TerminalCounts[0].Count != format.ExcludedTaskGroups {
		return errors.New("terminal counts do not conserve task groups")
	}
	expectedFlows := [][2]string{{"runtime_witness", "native_export"}, {"native_export", "canonical_graph"}, {"canonical_graph", "retained_bundle"}, {"retained_bundle", "verifier_request"}}
	entered := format.UniqueTaskGroups
	for index, flow := range format.Flows {
		if flow.FromLayer != expectedFlows[index][0] || flow.ToLayer != expectedFlows[index][1] || flow.Entered != entered ||
			flow.Survived < 0 || flow.Lost < 0 || flow.Survived+flow.Lost != flow.Entered {
			return errors.New("layer flow violates exact conservation")
		}
		entered = flow.Survived
	}
	return nil
}
