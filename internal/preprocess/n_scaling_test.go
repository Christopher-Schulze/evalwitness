package preprocess

import "testing"

func TestMeasureSliceRetention(t *testing.T) {
	traj := Trajectory{
		SourceFormat: SourceClaudeCode,
		Events: []Event{
			{ID: "e1", Kind: EventMessage, Message: &MessagePayload{Role: "user", Parts: []ContentPart{{Kind: ContentText, Text: "hello"}}}, ContentBytes: 5, EstimatedTokens: 1},
			{ID: "e2", Kind: EventMessage, Message: &MessagePayload{Role: "assistant", Parts: []ContentPart{{Kind: ContentText, Text: "world"}}}, ContentBytes: 5, EstimatedTokens: 1},
			{ID: "e3", Kind: EventMessage, Message: &MessagePayload{Role: "user", Parts: []ContentPart{{Kind: ContentText, Text: "foo"}}}, ContentBytes: 3, EstimatedTokens: 1},
			{ID: "e4", Kind: EventMessage, Message: &MessagePayload{Role: "assistant", Parts: []ContentPart{{Kind: ContentText, Text: "bar"}}}, ContentBytes: 3, EstimatedTokens: 1},
		},
	}
	rets := MeasureSliceRetention(traj, []int{1, 10, 100000})
	if len(rets) != 3 {
		t.Fatalf("expected 3, got %d", len(rets))
	}
	if rets[0].Format != string(SourceClaudeCode) {
		t.Fatalf("format %s", rets[0].Format)
	}
	if rets[2].Retained < rets[0].Retained-1e-9 {
		t.Fatalf("retention not monotonic %v", rets)
	}
}

func TestMeasureNScaling(t *testing.T) {
	ns := []int{2, 3, 5, 8}
	rets := MeasureNScaling(ns, 10)
	if len(rets) != 4 {
		t.Fatalf("expected 4, got %d", len(rets))
	}
	for i, r := range rets {
		want := ns[i] - 1
		if r.Calls != want {
			t.Fatalf("n=%d calls %d want %d", r.N, r.Calls, want)
		}
		if r.Coverage != 1.0 {
			t.Fatalf("n=%d coverage %f want 1", r.N, r.Coverage)
		}
	}
	rets2 := MeasureNScaling([]int{10}, 5)
	if rets2[0].Coverage != 0.5 {
		t.Fatalf("coverage %f want 0.5", rets2[0].Coverage)
	}
}
