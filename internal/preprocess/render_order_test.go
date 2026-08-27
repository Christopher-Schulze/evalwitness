package preprocess

import (
	"strings"
	"testing"
)

func TestRenderTrajectoryInOrderRequiresCompleteDependencyValidPermutation(t *testing.T) {
	trajectory := renderOrderTrajectory(t)
	first, second, third := trajectory.Events[0].ID, trajectory.Events[1].ID, trajectory.Events[2].ID
	rendered, err := RenderTrajectoryInOrder(trajectory, []string{second, first, third})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(rendered, "second") > strings.Index(rendered, "first") ||
		strings.Index(rendered, "first") > strings.Index(rendered, "third") {
		t.Fatalf("presentation order was not honored: %s", rendered)
	}
	for _, order := range [][]string{{third, first, second}, {first, first, third}, {first, second}, {first, second, "unknown"}} {
		if _, err := RenderTrajectoryInOrder(trajectory, order); err == nil {
			t.Fatalf("invalid presentation order was accepted: %v", order)
		}
	}
}

func renderOrderTrajectory(t *testing.T) Trajectory {
	t.Helper()
	events := []Event{
		renderOrderEvent(0, 1, "first"), renderOrderEvent(1, 2, "second"), renderOrderEvent(2, 3, "third"),
	}
	for index := range events {
		var err error
		events[index], err = RebuildDerivedEvent(SourcePlainText, events[index])
		if err != nil {
			t.Fatal(err)
		}
	}
	trajectory := Trajectory{
		SchemaVersion: CanonicalTrajectorySchema, SourceFormat: SourcePlainText,
		SourceDigest: Hash("render-order-source"), Digest: Hash("render-order-trajectory"), Events: events,
		Links: []Link{{Kind: LinkParent, FromID: events[0].ID, ToID: events[2].ID}},
		Report: IngestionReport{SchemaVersion: CanonicalTrajectorySchema, SourceRecords: 1, AccountedRecords: 1,
			CanonicalEvents: 3, Records: []RecordAccounting{{Source: SourceLocation{Record: 0}, Disposition: DispositionRepresented}}},
	}
	if err := trajectory.Validate(); err != nil {
		t.Fatal(err)
	}
	return trajectory
}

func renderOrderEvent(order, line int, text string) Event {
	return Event{
		Kind: EventMessage, Order: order, Source: SourceLocation{Record: 0, Line: line}, Sensitivity: SensitivityPublic,
		Message: &MessagePayload{Role: "assistant", Parts: []ContentPart{{Kind: ContentText, Text: text}}},
	}
}
