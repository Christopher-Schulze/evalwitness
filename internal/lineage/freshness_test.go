package lineage

import (
	"strings"
	"testing"
	"time"
)

func TestFreshnessIntervalCurrentClosedAndUnresolved(t *testing.T) {
	started := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	state := strings.Repeat("1", 64)
	current := FreshnessInterval{State: FreshnessCurrent, ObservedStateDigest: state, ValidFrom: started, EvaluatedAt: started.Add(time.Minute)}
	if err := current.Validate(); err != nil {
		t.Fatal(err)
	}
	closedAt := started.Add(30 * time.Second)
	closed := FreshnessInterval{
		State: FreshnessClosed, ObservedStateDigest: state, ValidFrom: started, ValidUntil: closedAt, EvaluatedAt: started.Add(time.Minute),
		InvalidationEdges: []InvalidationEdge{{
			EdgeID: "edge-1", Kind: "file_change", SubjectAlias: "tracked-file-1", BeforeStateDigest: state,
			AfterStateDigest: strings.Repeat("2", 64), ObservedAt: closedAt,
		}},
	}
	if err := closed.Validate(); err != nil {
		t.Fatal(err)
	}
	unresolved := FreshnessInterval{State: FreshnessUnresolved, EvaluatedAt: started}
	if err := unresolved.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFreshnessIntervalRejectsStaleCurrentAndFalseClosure(t *testing.T) {
	started := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	state := strings.Repeat("1", 64)
	tests := []FreshnessInterval{
		{State: FreshnessCurrent, ObservedStateDigest: state, ValidFrom: started, ValidUntil: started.Add(time.Second), EvaluatedAt: started.Add(time.Minute)},
		{State: FreshnessUnresolved, ObservedStateDigest: state, EvaluatedAt: started},
		{State: FreshnessClosed, ObservedStateDigest: state, ValidFrom: started, ValidUntil: started.Add(time.Second), EvaluatedAt: started.Add(time.Minute)},
		{
			State: FreshnessClosed, ObservedStateDigest: state, ValidFrom: started, ValidUntil: started.Add(time.Second), EvaluatedAt: started.Add(time.Minute),
			InvalidationEdges: []InvalidationEdge{{EdgeID: "edge-1", Kind: "file_change", SubjectAlias: "file-1", BeforeStateDigest: strings.Repeat("3", 64), AfterStateDigest: strings.Repeat("2", 64), ObservedAt: started.Add(time.Second)}},
		},
	}
	for index, interval := range tests {
		if err := interval.Validate(); err == nil {
			t.Fatalf("invalid freshness interval %d was accepted", index)
		}
	}
}
