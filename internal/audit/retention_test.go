package audit

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestMeasureRetentionFractions(t *testing.T) {
	trajectory := preprocess.Trajectory{
		SchemaVersion: "evalwitness.trajectory.v1",
		Digest:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Events: []preprocess.Event{
			{ID: "e1", Kind: preprocess.EventMessage, ContentDigest: "d1"},
			{ID: "e2", Kind: preprocess.EventMessage, ContentDigest: "d2"},
			{ID: "e3", Kind: preprocess.EventToolCall, ContentDigest: "d3"},
			{ID: "e4", Kind: preprocess.EventToolResult, ContentDigest: "d4"},
		},
	}
	points, err := MeasureRetention(trajectory, []int{16384, 32768, 65536})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) == 0 {
		t.Fatal("no points")
	}
	for _, point := range points {
		if point.Fraction < 0 || point.Fraction > 1 {
			t.Fatalf("fraction out of range %+v", point)
		}
		if point.Retained > point.Original {
			t.Fatalf("retained > original %+v", point)
		}
	}
	// Empty digest must fail
	trajectory.Digest = ""
	if _, err := MeasureRetention(trajectory, []int{16384}); err == nil {
		t.Fatal("empty digest must fail")
	}
}
