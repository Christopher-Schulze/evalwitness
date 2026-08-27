package reliance

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

var evidencePanelFixtureCache struct {
	once    sync.Once
	parents EvidenceTaskPanelParents
	request EvidenceTaskPanelRequest
}

func TestEvidenceTaskPanelExecutesOneBaselineAndEveryWalshCell(t *testing.T) {
	parents, request := evidenceTaskPanelFixture(t)
	provider := newRelianceReplayProvider(t)
	result, err := RunEvidenceTaskPanel(context.Background(), newRelianceReplayRunner(t, provider), parents, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := result.Validate(parents, request); err != nil {
		t.Fatal(err)
	}
	if err := result.Execution.Validate(); err != nil {
		t.Fatal(err)
	}
	if result.Execution.LogicalCalls != ReferenceCellsPerTask+1 || provider.calls.Load() != ReferenceCellsPerTask+1 ||
		len(result.Execution.Cells) != ReferenceCellsPerTask || len(result.Replay.Items) != ReferenceCellsPerTask+1 ||
		len(result.Execution.BaselineEvidence) != 1 || result.Execution.Digest == "" {
		t.Fatalf("evidence task panel is incomplete: calls=%d provider=%d cells=%d replay=%d",
			result.Execution.LogicalCalls, provider.calls.Load(), len(result.Execution.Cells), len(result.Replay.Items))
	}
	for _, cell := range result.Execution.Cells {
		if len(cell.CriterionContrasts) != 1 || cell.Replay.ObservationSetDigest == "" || cell.PresentationDigest == "" {
			t.Fatalf("evidence task panel cell is incomplete: %+v", cell)
		}
	}
}

func TestEvidenceTaskPanelRejectsIncompleteWalshPanelBeforeReplay(t *testing.T) {
	parents, request := evidenceTaskPanelFixture(t)
	provider := newRelianceReplayProvider(t)
	request.Cells = request.Cells[:len(request.Cells)-1]
	_, err := RunEvidenceTaskPanel(context.Background(), newRelianceReplayRunner(t, provider), parents, request)
	if err == nil || provider.calls.Load() != 0 {
		t.Fatalf("incomplete panel error=%v replay_calls=%d", err, provider.calls.Load())
	}
}

func TestEvidenceTaskPanelRejectsCellInputSubstitutionBeforeReplay(t *testing.T) {
	parents, request := evidenceTaskPanelFixture(t)
	provider := newRelianceReplayProvider(t)
	request.Cells[7].Input.Trajectories[0] = "substituted cell trajectory"
	_, err := RunEvidenceTaskPanel(context.Background(), newRelianceReplayRunner(t, provider), parents, request)
	if err == nil || provider.calls.Load() != 0 {
		t.Fatalf("substituted panel error=%v replay_calls=%d", err, provider.calls.Load())
	}
}

func TestEvidenceTaskPanelRejectsResealedExecutionTampering(t *testing.T) {
	parents, request := evidenceTaskPanelFixture(t)
	result, err := RunEvidenceTaskPanel(context.Background(),
		newRelianceReplayRunner(t, newRelianceReplayProvider(t)), parents, request)
	if err != nil {
		t.Fatal(err)
	}
	policyTampered := result.Execution
	policyTampered.EvidencePolicy = "unknown"
	policyTampered.Digest, err = evidenceTaskPanelDigest(policyTampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := policyTampered.Validate(); err == nil {
		t.Fatal("standalone evidence task panel accepted an unknown evidence policy")
	}
	result.Execution.Cells[0].CriterionContrasts[0].Comparison.VisibleMassMovement += 0.01
	result.Execution.Digest, err = evidenceTaskPanelDigest(result.Execution)
	if err != nil {
		t.Fatal(err)
	}
	if err := result.Execution.Validate(); err == nil {
		t.Fatal("standalone evidence task panel accepted a resealed comparison")
	}
	if err := result.Validate(parents, request); err == nil {
		t.Fatal("resealed evidence task panel tampering was accepted")
	}
}

func TestEvidenceTaskPanelExactReplayIsDeterministic(t *testing.T) {
	parents, request := evidenceTaskPanelFixture(t)
	runner := newRelianceReplayRunner(t, newRelianceReplayProvider(t))
	first, err := RunEvidenceTaskPanel(context.Background(), runner, parents, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunEvidenceTaskPanel(context.Background(), runner, parents, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Execution, second.Execution) {
		t.Fatal("exact replay changed the deterministic evidence task panel artifact")
	}
}

func evidenceTaskPanelFixture(t *testing.T) (EvidenceTaskPanelParents, EvidenceTaskPanelRequest) {
	t.Helper()
	evidencePanelFixtureCache.once.Do(func() {
		evidencePanelFixtureCache.parents, evidencePanelFixtureCache.request = buildEvidenceTaskPanelFixture(t)
	})
	encoded, err := json.Marshal(struct {
		Parents EvidenceTaskPanelParents
		Request EvidenceTaskPanelRequest
	}{evidencePanelFixtureCache.parents, evidencePanelFixtureCache.request})
	if err != nil {
		t.Fatal(err)
	}
	var cloned struct {
		Parents EvidenceTaskPanelParents
		Request EvidenceTaskPanelRequest
	}
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned.Parents, cloned.Request
}

func buildEvidenceTaskPanelFixture(t *testing.T) (EvidenceTaskPanelParents, EvidenceTaskPanelRequest) {
	t.Helper()
	fixture := factorialCellFixture(t)
	parents := EvidenceTaskPanelParents{
		Ontology: fixture.ontology, Estimands: fixture.estimands, Assignments: fixture.assignments,
		Preregistration: fixture.preregistration, Parent: fixture.parent, TreatmentPlan: fixture.plan,
	}
	request := EvidenceTaskPanelRequest{SourceTaskID: "factorial-task", Cells: evidenceTaskPanelCells(t, fixture)}
	outcomeSetDigest, err := panelOutcomeEvidenceSetDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Baseline = evidenceTaskPanelInput(parents, request.SourceTaskID, PanelBaselineVariant,
		PanelBaselineLabel, outcomeSetDigest, preprocess.RenderTrajectory(parents.Parent))
	for index := range request.Cells {
		rendered, err := RenderCellPresentationOrder(parents.Ontology, parents.Estimands, parents.Assignments,
			parents.Preregistration, parents.Parent, parents.TreatmentPlan, request.Cells[index].Request,
			request.Cells[index].Cell, request.Cells[index].Presentation)
		if err != nil {
			t.Fatal(err)
		}
		cellID := request.Cells[index].Cell.Cell.CellID
		request.Cells[index].Input = evidenceTaskPanelInput(parents, request.SourceTaskID,
			PanelCellVariant, cellID, outcomeSetDigest, rendered)
	}
	return parents, request
}

func evidenceTaskPanelCells(t *testing.T, fixture cellFixtureState) []EvidenceTaskPanelCellRequest {
	t.Helper()
	result := make([]EvidenceTaskPanelCellRequest, ReferenceCellsPerTask)
	for index := range result {
		request := factorialCellRequest(t, fixture.preregistration, nil, 1)
		request.CellID = fmt.Sprintf("factorial-cell-%02d", index)
		request.Levels = referenceLevels(index, canonicalReferenceMasks())
		cell, err := ApplyEvidenceInterventionCell(fixture.ontology, fixture.estimands, fixture.assignments,
			fixture.preregistration, fixture.parent, fixture.plan, request)
		if err != nil {
			t.Fatal(err)
		}
		presentation, err := BuildCellPresentationOrder(fixture.ontology, fixture.estimands, fixture.assignments,
			fixture.preregistration, fixture.parent, fixture.plan, request, cell)
		if err != nil {
			t.Fatal(err)
		}
		result[index] = EvidenceTaskPanelCellRequest{CellIndex: index, Request: request, Cell: cell, Presentation: presentation}
	}
	return result
}

func evidenceTaskPanelInput(parents EvidenceTaskPanelParents, sourceTaskID, variant, cellID, outcomeSetDigest, trajectory string) verification.Input {
	return verification.Input{
		Entrypoint: "reliance-panel", Mode: verification.ModeAbsolute, Task: "verify the frozen evidence cell",
		Trajectories: []string{trajectory},
		Criteria:     []verifier.Criterion{{ID: "correctness", Name: "Correctness", Description: "Assess correctness."}},
		Policy: verification.Policy{
			Evidence: verification.EvidenceExplicitJudge, NReps: 1, Epsilon: 0.02,
			BiasMitigation: "disabled", InconsistencyPolicy: "flag-only", SelectionStrategy: "absolute",
			MaxWorkers: 2, MaxPairCalls: 4, ConfidenceThreshold: 0.8,
		},
		StudyManifestDigest: protocolkit.DigestBytes([]byte("evidence-task-panel-study")),
		StudyVariant:        variant, DisableCache: true,
		Lineage: verification.LineageReferences{
			AuditCaseID: sourceTaskID, TransformationID: parents.TreatmentPlan.PlanID,
			OutcomeEvidenceDigest: outcomeSetDigest, StudyCellID: cellID,
		},
	}
}
