package reliance

import (
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

type presentationGraph struct {
	events    map[string]preprocess.Event
	adjacency map[string][]string
	indegree  map[string]int
}

func BuildCellPresentationOrder(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	treatmentPlan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
	cell EvidenceInterventionCellResult,
) (PresentationOrderPlan, error) {
	if err := cell.Validate(ontology, estimands, assignments, preregistration, parent, treatmentPlan, request); err != nil {
		return PresentationOrderPlan{}, err
	}
	narrativeIDs, err := narrativeChildEventIDs(parent, cell.Trajectory, assignments)
	if err != nil {
		return PresentationOrderPlan{}, err
	}
	return constructPresentationOrderPlan(cell, assignments, narrativeIDs)
}

func (value PresentationOrderPlan) Validate(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	treatmentPlan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
	cell EvidenceInterventionCellResult,
) error {
	expected, err := BuildCellPresentationOrder(ontology, estimands, assignments, preregistration, parent, treatmentPlan, request, cell)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("presentation-order plan differs from its validated factorial cell")
	}
	return nil
}

func RenderCellPresentationOrder(
	ontology FactorOntology,
	estimands EstimandCatalog,
	assignments FactorAssignmentSet,
	preregistration Preregistration,
	parent preprocess.Trajectory,
	treatmentPlan FactorTreatmentPlan,
	request EvidenceInterventionCellRequest,
	cell EvidenceInterventionCellResult,
	plan PresentationOrderPlan,
) (string, error) {
	if err := plan.Validate(ontology, estimands, assignments, preregistration, parent, treatmentPlan, request, cell); err != nil {
		return "", err
	}
	if plan.Status != PresentationOrderAvailable {
		return "", errors.New("factorial cell has no dependency-valid presentation-order contrast")
	}
	eventIDs := plan.NarrativeLastEventIDs
	if cellLevelMap(cell.Cell.Levels)[PresentationOrderTerm] < 0 {
		eventIDs = plan.NarrativeFirstEventIDs
	}
	return preprocess.RenderTrajectoryInOrder(cell.Trajectory, eventIDs)
}

func validateAndRenderPresentationFromValidatedCell(
	parent preprocess.Trajectory,
	assignments FactorAssignmentSet,
	cell EvidenceInterventionCellResult,
	plan PresentationOrderPlan,
) (string, error) {
	narrativeIDs, err := narrativeChildEventIDs(parent, cell.Trajectory, assignments)
	if err != nil {
		return "", err
	}
	expected, err := constructPresentationOrderPlan(cell, assignments, narrativeIDs)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(plan, expected) || plan.Status != PresentationOrderAvailable {
		return "", errors.New("presentation-order plan differs from its validated factorial cell or is unsupported")
	}
	eventIDs := plan.NarrativeLastEventIDs
	if cellLevelMap(cell.Cell.Levels)[PresentationOrderTerm] < 0 {
		eventIDs = plan.NarrativeFirstEventIDs
	}
	return preprocess.RenderTrajectoryInOrder(cell.Trajectory, eventIDs)
}

func constructPresentationOrderPlan(
	cell EvidenceInterventionCellResult,
	assignments FactorAssignmentSet,
	narrativeIDs []string,
) (PresentationOrderPlan, error) {
	value := PresentationOrderPlan{
		SchemaVersion: PresentationOrderPlanSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PolicyVersion: PresentationOrderPolicyVersion, CellDigest: cell.Cell.Digest,
		AssignmentSetDigest: assignments.Digest, TrajectoryDigest: cell.Trajectory.Digest,
		NarrativeEventIDs: slices.Clone(narrativeIDs), ProviderCalls: 0, NetworkRequired: false,
	}
	if len(narrativeIDs) == 0 {
		value.Status, value.Reason = PresentationOrderUnsupported, PresentationReasonNarrativeAbsent
		return sealPresentationOrderPlan(value)
	}
	narrative := stringSet(narrativeIDs)
	first, firstOK := dependencyValidPresentationOrder(cell.Trajectory, narrative, true)
	last, lastOK := dependencyValidPresentationOrder(cell.Trajectory, narrative, false)
	if !firstOK || !lastOK {
		value.Status, value.Reason = PresentationOrderUnsupported, PresentationReasonDependencyCycle
		return sealPresentationOrderPlan(value)
	}
	if slices.Equal(first, last) {
		value.Status, value.Reason = PresentationOrderUnsupported, PresentationReasonNoOrderContrast
		return sealPresentationOrderPlan(value)
	}
	return completePresentationOrderPlan(value, cell.Trajectory, first, last)
}

func completePresentationOrderPlan(
	value PresentationOrderPlan,
	trajectory preprocess.Trajectory,
	first, last []string,
) (PresentationOrderPlan, error) {
	firstText, err := preprocess.RenderTrajectoryInOrder(trajectory, first)
	if err != nil {
		return PresentationOrderPlan{}, err
	}
	lastText, err := preprocess.RenderTrajectoryInOrder(trajectory, last)
	if err != nil {
		return PresentationOrderPlan{}, err
	}
	value.NarrativeFirstEventIDs, value.NarrativeLastEventIDs = first, last
	value.NarrativeFirstTextDigest, value.NarrativeLastTextDigest = preprocess.Hash(firstText), preprocess.Hash(lastText)
	value.Status = PresentationOrderAvailable
	return sealPresentationOrderPlan(value)
}

func narrativeChildEventIDs(
	parent, child preprocess.Trajectory,
	assignments FactorAssignmentSet,
) ([]string, error) {
	parentIndexes := make(map[string]int, len(parent.Events))
	for index, event := range parent.Events {
		parentIndexes[event.ID] = index
	}
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, assignment := range assignments.Assignments {
		if assignment.FactorID != FactorSuccessFailureProse {
			continue
		}
		index, found := parentIndexes[assignment.EventID]
		if !found || index >= len(child.Events) || !sameInterventionEventLineage(parent.Events[index], child.Events[index]) {
			return nil, errors.New("narrative factor assignment does not map to the factorial child")
		}
		if _, duplicate := seen[child.Events[index].ID]; !duplicate {
			seen[child.Events[index].ID] = struct{}{}
			result = append(result, child.Events[index].ID)
		}
	}
	slices.SortFunc(result, func(left, right string) int { return comparePresentationEventIDs(child, left, right) })
	return result, nil
}

func comparePresentationEventIDs(trajectory preprocess.Trajectory, left, right string) int {
	indexes := make(map[string]preprocess.Event, len(trajectory.Events))
	for _, event := range trajectory.Events {
		indexes[event.ID] = event
	}
	leftEvent, rightEvent := indexes[left], indexes[right]
	if leftEvent.Order != rightEvent.Order {
		return leftEvent.Order - rightEvent.Order
	}
	return strings.Compare(left, right)
}

func dependencyValidPresentationOrder(
	trajectory preprocess.Trajectory,
	narrative map[string]struct{},
	narrativeFirst bool,
) ([]string, bool) {
	graph := buildPresentationGraph(trajectory)
	ready := make([]string, 0)
	for eventID, degree := range graph.indegree {
		if degree == 0 {
			ready = append(ready, eventID)
		}
	}
	result := make([]string, 0, len(graph.events))
	for len(ready) > 0 {
		index := selectPresentationReady(ready, graph.events, narrative, narrativeFirst)
		eventID := ready[index]
		ready = slices.Delete(ready, index, index+1)
		result = append(result, eventID)
		for _, next := range graph.adjacency[eventID] {
			graph.indegree[next]--
			if graph.indegree[next] == 0 {
				ready = append(ready, next)
			}
		}
	}
	return result, len(result) == len(graph.events)
}

func buildPresentationGraph(trajectory preprocess.Trajectory) presentationGraph {
	graph := presentationGraph{
		events:    make(map[string]preprocess.Event, len(trajectory.Events)),
		adjacency: make(map[string][]string), indegree: make(map[string]int, len(trajectory.Events)),
	}
	for _, event := range trajectory.Events {
		graph.events[event.ID], graph.indegree[event.ID] = event, 0
	}
	seenEdges := make(map[string]struct{})
	for _, link := range trajectory.Links {
		key := link.FromID + "\x00" + link.ToID
		if _, duplicate := seenEdges[key]; duplicate {
			continue
		}
		seenEdges[key] = struct{}{}
		graph.adjacency[link.FromID] = append(graph.adjacency[link.FromID], link.ToID)
		graph.indegree[link.ToID]++
	}
	for eventID := range graph.adjacency {
		slices.Sort(graph.adjacency[eventID])
	}
	return graph
}

func selectPresentationReady(
	ready []string,
	events map[string]preprocess.Event,
	narrative map[string]struct{},
	narrativeFirst bool,
) int {
	best := 0
	for index := 1; index < len(ready); index++ {
		if presentationEventBefore(ready[index], ready[best], events, narrative, narrativeFirst) {
			best = index
		}
	}
	return best
}

func presentationEventBefore(
	left, right string,
	events map[string]preprocess.Event,
	narrative map[string]struct{},
	narrativeFirst bool,
) bool {
	_, leftNarrative := narrative[left]
	_, rightNarrative := narrative[right]
	if leftNarrative != rightNarrative {
		return leftNarrative == narrativeFirst
	}
	if events[left].Order != events[right].Order {
		return events[left].Order < events[right].Order
	}
	return left < right
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func sealPresentationOrderPlan(value PresentationOrderPlan) (PresentationOrderPlan, error) {
	digest, err := presentationOrderPlanDigest(value)
	if err != nil {
		return PresentationOrderPlan{}, err
	}
	value.Digest = digest
	return value, nil
}

func presentationOrderPlanDigest(value PresentationOrderPlan) (string, error) {
	value.Digest = ""
	return protocolkit.Digest(value)
}
