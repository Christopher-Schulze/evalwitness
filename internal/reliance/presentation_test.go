package reliance

import (
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestCellPresentationOrderBuildsTwoDependencyValidExtremes(t *testing.T) {
	fixture := factorialCellFixture(t)
	request := factorialCellRequest(t, fixture.preregistration, nil, -1)
	cell := applyFactorialCellFixture(t, fixture, request)
	plan, err := BuildCellPresentationOrder(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request, cell)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != PresentationOrderAvailable || len(plan.NarrativeEventIDs) != 1 ||
		plan.NarrativeFirstEventIDs[0] != plan.NarrativeEventIDs[0] ||
		plan.NarrativeLastEventIDs[len(plan.NarrativeLastEventIDs)-1] != plan.NarrativeEventIDs[0] ||
		plan.NarrativeFirstTextDigest == plan.NarrativeLastTextDigest {
		t.Fatalf("presentation-order extremes are invalid: %+v", plan)
	}
	rendered, err := RenderCellPresentationOrder(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request, cell, plan)
	if err != nil || preprocess.Hash(rendered) != plan.NarrativeFirstTextDigest ||
		strings.Index(rendered, "all checks passed") > strings.Index(rendered, "go test ./...") {
		t.Fatalf("negative presentation level did not render narrative first: %v", err)
	}
}

func TestCellPresentationOrderRetainsUnsupportedDependencyStates(t *testing.T) {
	chain := factorialCellFixtureWithLinks(t, false)
	request := factorialCellRequest(t, chain.preregistration, nil, 1)
	cell := applyFactorialCellFixture(t, chain, request)
	plan, err := BuildCellPresentationOrder(chain.ontology, chain.estimands, chain.assignments,
		chain.preregistration, chain.parent, chain.plan, request, cell)
	if err != nil || plan.Status != PresentationOrderUnsupported || plan.Reason != PresentationReasonNoOrderContrast {
		t.Fatalf("total-order presentation state = %+v error=%v", plan, err)
	}
	cycle := factorialCellFixtureWithLinks(t, true)
	cell = applyFactorialCellFixture(t, cycle, request)
	plan, err = BuildCellPresentationOrder(cycle.ontology, cycle.estimands, cycle.assignments,
		cycle.preregistration, cycle.parent, cycle.plan, request, cell)
	if err != nil || plan.Status != PresentationOrderUnsupported || plan.Reason != PresentationReasonDependencyCycle {
		t.Fatalf("reference-cycle presentation state = %+v error=%v", plan, err)
	}
}

func applyFactorialCellFixture(t *testing.T, fixture cellFixtureState, request EvidenceInterventionCellRequest) EvidenceInterventionCellResult {
	t.Helper()
	result, err := ApplyEvidenceInterventionCell(fixture.ontology, fixture.estimands, fixture.assignments,
		fixture.preregistration, fixture.parent, fixture.plan, request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func factorialCellFixtureWithLinks(t *testing.T, referenceCycle bool) cellFixtureState {
	t.Helper()
	fixture := factorialCellFixture(t)
	fixture.parent.Links = factorialPresentationLinks(fixture.parent.Events, referenceCycle)
	fixture.parent.Digest = protocolkit.DigestBytes([]byte("factorial-linked-trajectory"))
	assignments := factorialAssignments(t, fixture.ontology, fixture.parent)
	plan, err := SealFactorTreatmentPlan(fixture.ontology, fixture.estimands, assignments, fixture.parent,
		EstimandEvidenceOnly, factorialTreatments(t, fixture.parent, assignments))
	if err != nil {
		t.Fatal(err)
	}
	fixture.assignments, fixture.plan = assignments, plan
	return fixture
}

func factorialPresentationLinks(events []preprocess.Event, referenceCycle bool) []preprocess.Link {
	if referenceCycle {
		return []preprocess.Link{
			{Kind: preprocess.LinkReference, FromID: events[0].ID, ToID: events[1].ID},
			{Kind: preprocess.LinkReference, FromID: events[1].ID, ToID: events[0].ID},
		}
	}
	result := make([]preprocess.Link, 0, len(events)-1)
	for index := 1; index < len(events); index++ {
		result = append(result, preprocess.Link{Kind: preprocess.LinkParent, FromID: events[index-1].ID, ToID: events[index].ID})
	}
	return result
}
