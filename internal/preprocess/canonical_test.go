package preprocess

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCanonicalAdaptersAccountEverySourceRecord(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		format SourceFormat
	}{
		{name: "claude", raw: claudeCodeFixture, format: SourceClaudeCode},
		{name: "codex", raw: codexFixture, format: SourceCodexRollout},
		{name: "opencode", raw: openCodeFixture, format: SourceOpenCode},
		{name: "terminal-bench", raw: `{"trial_id":"trial","trajectory":{"schema_version":"1","session_id":"session","steps":[{"step_id":1,"source":"system","message":"policy","timestamp":"t"},{"step_id":2,"source":"user","message":"task","timestamp":"t"},{"step_id":3,"source":"agent","message":"run","timestamp":"t","tool_calls":[{"tool_call_id":"call","function_name":"terminal","arguments":{"keystrokes":"go test ./..."}}],"observation":{"results":[{"content":"ok"}]}}]}}`, format: SourceTerminalBench},
		{name: "swe-bench", raw: `{"instance_id":"repo__1","trajectory_id":"trajectory","model_name":"model","num_steps":1,"messages":"[{\"role\":\"system\",\"content\":\"policy\"},{\"role\":\"user\",\"content\":\"fix\"},{\"role\":\"assistant\",\"content\":\"run\",\"tool_calls\":[{\"id\":\"call\",\"type\":\"function\",\"function\":{\"name\":\"shell\",\"arguments\":\"{\\\"command\\\":\\\"go test ./...\\\"}\"}}]},{\"role\":\"tool\",\"name\":\"shell\",\"tool_call_id\":\"call\",\"content\":\"ok\"}]","output_patch":"diff --git a/a.go b/a.go"}`, format: SourceSWEbench},
		{name: "plain", raw: "plain transcript", format: SourcePlainText},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trajectory, err := IngestString(test.raw, DefaultIngestOptions())
			if err != nil {
				t.Fatal(err)
			}
			if trajectory.SourceFormat != test.format {
				t.Fatalf("format = %q, want %q", trajectory.SourceFormat, test.format)
			}
			if trajectory.Report.SourceRecords == 0 || trajectory.Report.AccountedRecords != trajectory.Report.SourceRecords {
				t.Fatalf("invalid source accounting: %+v", trajectory.Report)
			}
			if len(trajectory.Events) == 0 {
				t.Fatal("canonical adapter emitted no events")
			}
			if err := trajectory.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCanonicalPipelinePreservesUserMessagesAndOmitsReasoningContent(t *testing.T) {
	raw := `{"type":"user","uuid":"u1","message":{"role":"user","content":"keep this follow-up"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"thinking","thinking":"private chain"},{"type":"text","text":"public answer"}]}}`
	result, err := CanonicalPipeline(raw, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "keep this follow-up") || !strings.Contains(result.Text, "public answer") {
		t.Fatalf("canonical render lost visible messages: %s", result.Text)
	}
	if strings.Contains(result.Text, "private chain") {
		t.Fatal("reasoning content leaked into canonical evidence")
	}
	if !strings.Contains(result.Text, "content omitted by policy") {
		t.Fatal("reasoning omission was not represented")
	}
}

func TestCanonicalPipelineHasStableIdentityForIdenticalInput(t *testing.T) {
	left, err := CanonicalPipeline("identical", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	right, err := CanonicalPipeline("identical", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if left.Hash != right.Hash || left.Text != right.Text {
		t.Fatalf("identical input changed identity: %q != %q\nleft=%s\nright=%s", left.Hash, right.Hash, left.Text, right.Text)
	}
}

func TestEvidenceBudgetBypassesExactFitByteForByte(t *testing.T) {
	trajectory, err := IngestString("short canonical input", DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	before := RenderTrajectory(trajectory)
	budget := estimateTokensForBytes(len(before))
	retained, err := ApplyEvidenceBudget(trajectory, budget)
	if err != nil {
		t.Fatal(err)
	}
	if got := RenderTrajectory(retained); got != before {
		t.Fatalf("exact-fit trajectory changed\nbefore: %q\nafter:  %q", before, got)
	}
	if retained.Report.Truncation.Applied {
		t.Fatal("exact-fit trajectory reported truncation")
	}
}

func TestEvidenceBudgetIsHardForOversizedFirstEventAndUnicode(t *testing.T) {
	raw := strings.Repeat("🧪 evidence line\n", 5000) + "FINAL: tests passed"
	trajectory, err := IngestString(raw, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := ApplyEvidenceBudget(trajectory, 100)
	if err != nil {
		t.Fatal(err)
	}
	text := RenderTrajectory(retained)
	if tokens := estimateTokensForBytes(len(text)); tokens > 100 {
		t.Fatalf("retained %d tokens over 100-token budget", tokens)
	}
	if !utf8.ValidString(text) {
		t.Fatal("Unicode boundary was split")
	}
	if !retained.Report.Truncation.Applied || retained.Report.Truncation.EventID == "" {
		t.Fatalf("missing structured truncation boundary: %+v", retained.Report.Truncation)
	}
}

func TestEvidenceBudgetRejectsWhenNoCanonicalUnitFits(t *testing.T) {
	trajectory, err := IngestString("content", DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyEvidenceBudget(trajectory, 1); err == nil {
		t.Fatal("one-token budget must fail when even the event envelope cannot fit")
	}
}

func TestEvidenceBudgetRetainsRequiredNarrativeAnchor(t *testing.T) {
	raw := `{"instance_id":"repo__1","trajectory_id":"trajectory","model_name":"model","num_steps":2,"messages":"[{\"role\":\"user\",\"content\":\"fix the bug\"},{\"role\":\"assistant\",\"content\":\"investigate\",\"tool_calls\":[{\"id\":\"call\",\"type\":\"function\",\"function\":{\"name\":\"shell\",\"arguments\":\"{\\\"command\\\":\\\"go test ./...\\\"}\"}}]},{\"role\":\"tool\",\"name\":\"shell\",\"tool_call_id\":\"call\",\"content\":\"` + strings.Repeat("failure details ", 2000) + `\"},{\"role\":\"assistant\",\"content\":\"final outcome summary\"}]","output_patch":""}`
	trajectory, err := IngestString(raw, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	anchorID := ""
	for _, event := range trajectory.Events {
		if event.Message != nil && strings.Contains(RenderTrajectory(Trajectory{SourceFormat: trajectory.SourceFormat, Events: []Event{event}}), "final outcome summary") {
			anchorID = event.ID
		}
	}
	if anchorID == "" {
		t.Fatal("fixture did not produce a narrative anchor")
	}
	retained, err := ApplyEvidenceBudgetWithRequiredEvents(trajectory, 100, []string{anchorID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(RenderTrajectory(retained), "final outcome summary") {
		t.Fatal("required narrative anchor was dropped")
	}
	if retained.Derivation == nil || retained.Derivation.Validator != "evalwitness.evidence-selector.v2" {
		t.Fatalf("anchored selection did not record its policy: %+v", retained.Derivation)
	}
	if _, err := ApplyEvidenceBudgetWithRequiredEvents(trajectory, 100, []string{"missing-event"}); err == nil {
		t.Fatal("missing required event was accepted")
	}
	oversizedID := ""
	for _, event := range trajectory.Events {
		if event.Kind == EventToolResult {
			oversizedID = event.ID
		}
	}
	if oversizedID == "" {
		t.Fatal("fixture did not produce an oversized linked result")
	}
	if _, err := ApplyEvidenceBudgetWithRequiredEvents(trajectory, 100, []string{oversizedID}); err == nil {
		t.Fatal("required unit larger than the hard budget was silently truncated or dropped")
	}
}

func TestPairedEvidenceBudgetRetainsIdenticalLineagesAcrossSizeChangingMutation(t *testing.T) {
	raw := `{"instance_id":"repo__1","trajectory_id":"trajectory","model_name":"model","num_steps":2,"messages":"[{\"role\":\"user\",\"content\":\"fix the bug\"},{\"role\":\"assistant\",\"content\":\"investigate\",\"tool_calls\":[{\"id\":\"call\",\"type\":\"function\",\"function\":{\"name\":\"shell\",\"arguments\":\"{\\\"command\\\":\\\"go test ./...\\\"}\"}}]},{\"role\":\"tool\",\"name\":\"shell\",\"tool_call_id\":\"call\",\"content\":\"` + strings.Repeat("failure details ", 5000) + `\"},{\"role\":\"assistant\",\"content\":\"final outcome summary\"}]","output_patch":"diff --git a/a.go b/a.go"}`
	original, err := IngestString(raw, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	events := append([]Event(nil), original.Events...)
	links := append([]Link(nil), original.Links...)
	targetIndex := -1
	for index, event := range events {
		if event.ToolResult != nil {
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		t.Fatal("fixture did not produce a tool result")
	}
	oldID := events[targetIndex].ID
	events[targetIndex], err = cloneEvent(events[targetIndex])
	if err != nil {
		t.Fatal(err)
	}
	events[targetIndex].ToolResult.Output[0].Text = events[targetIndex].ToolResult.Output[0].Text[:len(events[targetIndex].ToolResult.Output[0].Text)/2]
	events[targetIndex], err = RebuildDerivedEvent(original.SourceFormat, events[targetIndex])
	if err != nil {
		t.Fatal(err)
	}
	for index := range links {
		if links[index].FromID == oldID {
			links[index].FromID = events[targetIndex].ID
		}
		if links[index].ToID == oldID {
			links[index].ToID = events[targetIndex].ID
		}
	}
	transformed, err := DeriveTrajectory(original, events, links, DerivationSpec{
		Relation: "controlled_truncation", Validator: "fixture.validator.v1", ChangedEventIDs: []string{oldID},
		ChangedFieldPaths: []FieldPath{FieldPath("/events/" + oldID + "/tool_result/output/0/text")},
	})
	if err != nil {
		t.Fatal(err)
	}
	leftAnchor := original.Events[len(original.Events)-1].ID
	rightAnchor := transformed.Events[len(transformed.Events)-1].ID
	left, right, err := ApplyPairedEvidenceBudgetWithRequiredEvents(original, transformed, 1000, []string{leftAnchor}, []string{rightAnchor})
	if err != nil {
		t.Fatal(err)
	}
	lineages := func(trajectory Trajectory) string {
		keys := make([]string, len(trajectory.Events))
		for index, event := range trajectory.Events {
			keys[index] = eventLineageKey(event)
		}
		sort.Strings(keys)
		return strings.Join(keys, "\n")
	}
	if lineages(left) != lineages(right) {
		t.Fatal("paired evidence selector retained different event lineages")
	}
	for _, retained := range []Trajectory{left, right} {
		if retained.Derivation == nil || retained.Derivation.Validator != "evalwitness.evidence-selector.paired.v1" {
			t.Fatalf("paired selector identity is missing: %+v", retained.Derivation)
		}
		if estimateTokensForBytes(len(RenderTrajectory(retained))) > 1000 {
			t.Fatal("paired selector exceeded the hard budget")
		}
	}
	if _, _, err := ApplyPairedEvidenceBudgetWithRequiredEvents(original, transformed, 1000, []string{leftAnchor}, []string{events[targetIndex].ID}); err == nil {
		t.Fatal("paired selector accepted anchors from different immutable lineages")
	}
}

func TestEvidenceBudgetCoversZeroOneOverAndEnormousOutput(t *testing.T) {
	raw := `{"trial_id":"trial","trajectory":{"steps":[{"step_id":1,"source":"agent","message":"run","tool_calls":[{"tool_call_id":"call","function_name":"terminal","arguments":{"keystrokes":"go test ./..."}}],"observation":{"results":[{"content":"` + strings.Repeat("output 🧪 ", 10000) + `"}]}}]}}`
	trajectory, err := IngestString(raw, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	fullTokens := estimateTokensForBytes(len(RenderTrajectory(trajectory)))
	unchanged, err := ApplyEvidenceBudget(trajectory, 0)
	if err != nil || RenderTrajectory(unchanged) != RenderTrajectory(trajectory) {
		t.Fatalf("zero budget must mean unbounded: err=%v", err)
	}
	exact, err := ApplyEvidenceBudget(trajectory, fullTokens)
	if err != nil || exact.Report.Truncation.Applied {
		t.Fatalf("exact boundary changed input: err=%v truncation=%+v", err, exact.Report.Truncation)
	}
	oneOver, err := ApplyEvidenceBudget(trajectory, fullTokens-1)
	if err != nil {
		t.Fatal(err)
	}
	if !oneOver.Report.Truncation.Applied || estimateTokensForBytes(len(RenderTrajectory(oneOver))) > fullTokens-1 {
		t.Fatal("one-over boundary was not enforced and accounted")
	}
}

func TestFidelityAuditComparesProviderUsageAndCanonicalEstimate(t *testing.T) {
	raw := `{"type":"assistant","uuid":"assistant-1","message":{"role":"assistant","content":"done","usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}`
	trajectory, err := IngestString(raw, DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	report, err := AuditFidelity(trajectory, []int{65536, 16384, 32768})
	if err != nil {
		t.Fatal(err)
	}
	if report.TokenComparison == nil || report.TokenComparison.ObservationCount != 1 {
		t.Fatalf("missing provider usage comparison: %+v", report.TokenComparison)
	}
	if len(report.Budgets) != 3 || report.Budgets[0].BudgetTokens != 16384 || report.Budgets[2].BudgetTokens != 65536 {
		t.Fatalf("fidelity budgets are not deterministic: %+v", report.Budgets)
	}
}

func TestStrictIngestionRejectsUnknownAndMalformedRecords(t *testing.T) {
	unknown := `{"type":"future-record","payload":{}}`
	if _, err := IngestString(unknown, DefaultIngestOptions()); err == nil {
		t.Fatal("strict ingestion accepted an unknown structured record")
	}
	malformed := `{"type":"session_meta","payload":{}}
{"type":`
	if _, err := IngestString(malformed, DefaultIngestOptions()); err == nil {
		t.Fatal("strict ingestion accepted malformed JSONL")
	}
}

func TestCompatibilityIngestionClassifiesUnknownClaudePart(t *testing.T) {
	options := DefaultIngestOptions()
	options.Mode = IngestCompatibility
	raw := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"future_part","value":"x"}]}}`
	trajectory, err := IngestString(raw, options)
	if err != nil {
		t.Fatal(err)
	}
	if trajectory.Report.UnsupportedRecords == 0 {
		t.Fatal("compatibility mode did not account unsupported content")
	}
}

func TestCausalGraphRejectsCyclesAndDuplicateIdentities(t *testing.T) {
	trajectory, err := IngestString("one", DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	duplicate := trajectory.Events[0]
	trajectory.Events = append(trajectory.Events, duplicate)
	if err := trajectory.Validate(); err == nil {
		t.Fatal("duplicate event identity was accepted")
	}
	trajectory.Events[1].ID = "evt_distinct"
	trajectory.Events[1].Order = 1
	trajectory.Links = []Link{
		{Kind: LinkParent, FromID: trajectory.Events[0].ID, ToID: trajectory.Events[1].ID},
		{Kind: LinkParent, FromID: trajectory.Events[1].ID, ToID: trajectory.Events[0].ID},
	}
	if err := trajectory.Validate(); err == nil {
		t.Fatal("causal cycle was accepted")
	}
}

func TestDerivationPreservesImmutableSourceLineage(t *testing.T) {
	parent, err := IngestString("parent evidence", DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	event, err := cloneEvent(parent.Events[0])
	if err != nil {
		t.Fatal(err)
	}
	events := []Event{event}
	events[0].Message.Parts[0].Text = "controlled intervention"
	events[0].RetainedBytes = len("controlled intervention")
	events[0].ContentBytes = events[0].RetainedBytes
	events[0].EstimatedTokens = estimateTokensForBytes(events[0].RetainedBytes)
	encoded, err := json.Marshal(eventPayloadMaterial(events[0]))
	if err != nil {
		t.Fatal(err)
	}
	events[0].ContentDigest = digestBytes(encoded)
	oldID := events[0].ID
	events[0].ID = stableEventID(parent.SourceFormat, events[0].Source, events[0].Kind, events[0].ContentDigest)
	field := FieldPath("/events/" + oldID + "/message/parts/0/text")
	child, err := DeriveTrajectory(parent, events, nil, DerivationSpec{
		Relation: "evidence_intervention", Validator: "fixture.validator.v1",
		ChangedEventIDs: []string{oldID}, ChangedFieldPaths: []FieldPath{field},
	})
	if err != nil {
		t.Fatal(err)
	}
	if child.SourceDigest != parent.SourceDigest || child.Derivation.ParentDigest != parent.Digest || child.Digest == parent.Digest {
		t.Fatalf("invalid derivation lineage: parent=%+v child=%+v", parent, child)
	}
	bad := child
	bad.Derivation.ChangedFieldPaths = []FieldPath{"/message/text"}
	if err := bad.Validate(); err == nil {
		t.Fatal("untyped derivation field path was accepted")
	}
}

func TestBoundedReaderRejectsSourceAndRecordOverflow(t *testing.T) {
	options := DefaultIngestOptions()
	options.MaxSourceBytes = 8
	if _, err := IngestString("more than eight bytes", options); err == nil {
		t.Fatal("source byte bound was not enforced")
	}
	options = DefaultIngestOptions()
	options.MaxRecordBytes = 16
	if _, err := IngestString(`{"type":"session_meta","payload":{"long":"record"}}`, options); err == nil {
		t.Fatal("JSONL record byte bound was not enforced")
	}
}
