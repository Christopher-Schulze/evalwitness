package preprocess

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readTraceFixture(t testing.TB, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/trace/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func importTraceFixture(t *testing.T, name string, privacy PrivacyClass) TraceImportResult {
	t.Helper()
	options := DefaultTraceImportOptions()
	options.Privacy = privacy
	result, err := ImportTraceBytes(readTraceFixture(t, name), options)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAgentTracePrivacyAndAttributionRoundTrip(t *testing.T) {
	metadata := importTraceFixture(t, "agent-trace-0.1.0.json", PrivacyMetadataOnly)
	contribution := firstContribution(t, metadata.Trajectory)
	if contribution.Path != "" || contribution.PathAlias == "" || contribution.ConversationURL != "" || contribution.ConversationDigest == "" {
		t.Fatalf("metadata-only contribution leaked attribution: %+v", contribution)
	}
	if metadata.Mapping.Totals.Redacted < 2 || metadata.Mapping.Lossless {
		t.Fatalf("metadata-only mapping = %+v", metadata.Mapping)
	}
	if _, err := ExportAgentTrace(metadata, nil); err == nil || !strings.Contains(err.Error(), "requires attribution_authorized") {
		t.Fatalf("metadata-only Agent Trace export error = %v", err)
	}

	attributed := importTraceFixture(t, "agent-trace-0.1.0.json", PrivacyAttribution)
	contribution = firstContribution(t, attributed.Trajectory)
	if contribution.Path != "internal/example.go" || contribution.ConversationURL == "" || contribution.StartLine != 4 || contribution.EndLine != 9 {
		t.Fatalf("attribution contribution = %+v", contribution)
	}
	exported, err := ExportAgentTrace(attributed, &VerifierEvidenceReference{ProtocolRunDigest: strings.Repeat("a", 64), AuditCaseID: "case.fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(exported.Bytes, []byte(`"quality"`)) || !bytes.Contains(exported.Bytes, []byte(`"org.evalwitness.verifier_evidence.v1"`)) {
		t.Fatalf("Agent Trace evidence boundary violated: %s", exported.Bytes)
	}
	if _, err := ExportAgentTrace(attributed, &VerifierEvidenceReference{ProtocolRunDigest: "not-a-digest"}); err == nil || !strings.Contains(err.Error(), "lowercase sha256") {
		t.Fatalf("invalid verifier evidence digest error = %v", err)
	}
	options := DefaultTraceImportOptions()
	options.Privacy = PrivacyAttribution
	roundTrip, err := ImportTraceBytes(exported.Bytes, options)
	if err != nil {
		t.Fatal(err)
	}
	got := firstContribution(t, roundTrip.Trajectory)
	if got.Path != contribution.Path || got.StartLine != contribution.StartLine || got.EndLine != contribution.EndLine || got.ContributorType != contribution.ContributorType {
		t.Fatalf("Agent Trace semantic round trip changed attribution: got %+v want %+v", got, contribution)
	}
}

func TestOTLPMetadataPrivacyCausalityAndSemanticRoundTrip(t *testing.T) {
	metadata := importTraceFixture(t, "otlp-genai-1.41.0.json", PrivacyMetadataOnly)
	if metadata.Envelope.Source.Format != SourceOTLPJSON || metadata.Envelope.Source.SchemaVersion != sourceVersionFor(SourceOTLPJSON) {
		t.Fatalf("OTLP source identity = %+v", metadata.Envelope.Source)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private fixture prompt", "go test ./..."} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("metadata-only import retained %q", secret)
		}
	}
	if metadata.Mapping.Totals.Redacted < 3 || metadata.Mapping.Lossless {
		t.Fatalf("metadata-only OTLP mapping = %+v", metadata.Mapping)
	}
	if len(metadata.Trajectory.Report.ProviderUsage) != 1 || metadata.Trajectory.Report.ProviderUsage[0].InputTokens != 12 || metadata.Trajectory.Report.ProviderUsage[0].OutputTokens != 7 {
		t.Fatalf("OTLP usage observations = %+v", metadata.Trajectory.Report.ProviderUsage)
	}
	inspection, err := InspectTrace(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.RootEvents != 1 || inspection.MaximumDepth < 2 || inspection.CausalLinks < 2 {
		t.Fatalf("OTLP hierarchy = roots %d depth %d links %d", inspection.RootEvents, inspection.MaximumDepth, inspection.CausalLinks)
	}
	for _, event := range metadata.Trajectory.Events {
		if event.External != nil && event.External.OperationName != "" && strings.Contains(event.External.OperationName, "conversation") {
			t.Fatalf("conversation ID was fabricated: %+v", event.External)
		}
	}

	content := importTraceFixture(t, "otlp-genai-1.41.0.json", PrivacyContent)
	if !trajectoryContains(content.Trajectory, "private fixture prompt") || !trajectoryContains(content.Trajectory, "go test ./...") || !trajectoryContains(content.Trajectory, "ok") {
		t.Fatal("content-authorized import did not retain all opt-in content")
	}
	exported, err := ExportOTLPJSON(content, PrivacyContent)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(exported.Bytes, []byte("gen_ai.usage.reasoning.output_tokens")) || bytes.Contains(exported.Bytes, []byte("gen_ai.usage.cache_read.input_tokens")) {
		t.Fatalf("OTLP export converted missing usage fields to observed zero: %s", exported.Bytes)
	}
	roundTrip, err := ImportTraceBytes(exported.Bytes, TraceImportOptions{Ingest: DefaultIngestOptions(), Privacy: PrivacyContent})
	if err != nil {
		t.Fatal(err)
	}
	if !trajectoryContains(roundTrip.Trajectory, "go test ./...") || !trajectoryContains(roundTrip.Trajectory, "ok") {
		t.Fatalf("OTLP semantic round trip lost tool semantics: %s", exported.Bytes)
	}
	if len(roundTrip.Trajectory.Report.ProviderUsage) != 1 || roundTrip.Trajectory.Report.ProviderUsage[0].InputTokens != 12 || roundTrip.Trajectory.Report.ProviderUsage[0].OutputTokens != 7 {
		t.Fatalf("OTLP semantic round trip lost usage: %+v", roundTrip.Trajectory.Report.ProviderUsage)
	}
	if bytes.Equal(exported.Bytes, readTraceFixture(t, "otlp-genai-1.41.0.json")) {
		t.Fatal("semantic round trip was incorrectly asserted as byte equality")
	}
}

func TestTraceImportRejectsSchemaDriftAmbiguityAndHostileJSON(t *testing.T) {
	otlp := readTraceFixture(t, "otlp-genai-1.41.0.json")
	drifted := bytes.ReplaceAll(otlp, []byte(OTelSchemaURL), []byte("https://opentelemetry.io/schemas/1.42.0"))
	if _, err := ImportTraceBytes(drifted, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "unsupported OpenTelemetry schema URL") {
		t.Fatalf("schema drift error = %v", err)
	}
	unknownField := bytes.Replace(otlp, []byte(`"traceId": "0123456789abcdef0123456789abcdef",`), []byte(`"unknownFutureField": true, "traceId": "0123456789abcdef0123456789abcdef",`), 1)
	if _, err := ImportTraceBytes(unknownField, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown OTLP field error = %v", err)
	}
	repeated := bytes.Replace(otlp, []byte(`"fedcba9876543210"`), []byte(`"0123456789abcdef"`), 1)
	if _, err := ImportTraceBytes(repeated, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "repeated OTLP span identity") {
		t.Fatalf("repeated identity error = %v", err)
	}
	deep := []byte(`{"resourceSpans":` + strings.Repeat("[", 65) + strings.Repeat("]", 65) + `}`)
	if _, err := ImportTraceBytes(deep, DefaultTraceImportOptions()); err == nil {
		t.Fatal("deep JSON was accepted")
	}
	jaeger := []byte(`{"data":[{"spans":[]}]}`)
	if _, err := ImportTraceBytes(jaeger, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "intentionally unsupported") {
		t.Fatalf("Jaeger boundary error = %v", err)
	}
	agent := readTraceFixture(t, "agent-trace-0.1.0.json")
	agentDrift := bytes.Replace(agent, []byte(`"version": "0.1.0",`), []byte(`"version": "0.1.0", "unknown_standard_field": true,`), 1)
	if _, err := ImportTraceBytes(agentDrift, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Agent Trace schema drift error = %v", err)
	}
}

func TestOTLPUnsupportedSpanEventAndResourceBounds(t *testing.T) {
	otlp := readTraceFixture(t, "otlp-genai-1.41.0.json")
	withEvent := bytes.Replace(otlp, []byte(`"status": {"code": 1}`), []byte(`"events": [{"timeUnixNano": "1767323045000000000", "name": "gen_ai.evaluation.result", "attributes": []}], "status": {"code": 1}`), 1)
	result, err := ImportTraceBytes(withEvent, DefaultTraceImportOptions())
	if err != nil {
		t.Fatal(err)
	}
	if result.Mapping.Totals.Unsupported == 0 {
		t.Fatal("OTLP span event was not reported as unsupported")
	}
	for _, event := range result.Trajectory.Events {
		if event.Kind == EventEvaluation {
			t.Fatal("OTLP span event was misrepresented as an OTel GenAI evaluation log event")
		}
	}

	unresolved := bytes.Replace(otlp, []byte(`"parentSpanId": "0123456789abcdef"`), []byte(`"parentSpanId": "aaaaaaaaaaaaaaaa"`), 1)
	result, err = ImportTraceBytes(unresolved, DefaultTraceImportOptions())
	if err != nil {
		t.Fatal(err)
	}
	if result.Mapping.Totals.Ambiguous == 0 {
		t.Fatal("missing OTLP parent was not reported as ambiguous")
	}

	invalidTimestamp := bytes.Replace(otlp, []byte(`"startTimeUnixNano": "1767323045000000000"`), []byte(`"startTimeUnixNano": "99999999999999999999"`), 1)
	if _, err := ImportTraceBytes(invalidTimestamp, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "startTimeUnixNano") {
		t.Fatalf("invalid timestamp error = %v", err)
	}
	invalidUsage := bytes.Replace(otlp, []byte(`{"intValue": "12"}`), []byte(`{"intValue": "-1"}`), 1)
	if _, err := ImportTraceBytes(invalidUsage, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "must be a non-negative") {
		t.Fatalf("invalid usage error = %v", err)
	}

	attributes := make([]otlpAttribute, 129)
	for index := range attributes {
		attributes[index] = otlpAttribute{Key: "resource." + strings.Repeat("x", index+1), Value: json.RawMessage(`{"stringValue":"x"}`)}
	}
	resource := otlpResourceSpans{}
	resource.Resource.Attributes = attributes
	resourceRaw, err := json.Marshal(resource)
	if err != nil {
		t.Fatal(err)
	}
	overLimit, err := json.Marshal(otlpTracesData{ResourceSpans: []json.RawMessage{resourceRaw}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportTraceBytes(overLimit, DefaultTraceImportOptions()); err == nil || !strings.Contains(err.Error(), "attribute count 129 exceeds 128") {
		t.Fatalf("resource attribute bound error = %v", err)
	}

	options := DefaultTraceImportOptions()
	options.Ingest.MaxSourceBytes = int64(len(otlp) - 1)
	if _, err := ImportTraceReader(bytes.NewReader(otlp), options); err == nil || !strings.Contains(err.Error(), "source exceeds") {
		t.Fatalf("source byte bound error = %v", err)
	}
}

func TestOTLPReferenceLinksDoNotInventCausality(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(readTraceFixture(t, "otlp-genai-1.41.0.json"), &document); err != nil {
		t.Fatal(err)
	}
	resources := document["resourceSpans"].([]any)
	resource := resources[0].(map[string]any)
	scopes := resource["scopeSpans"].([]any)
	scope := scopes[0].(map[string]any)
	spans := scope["spans"].([]any)
	first := spans[0].(map[string]any)
	second := spans[1].(map[string]any)
	traceID := first["traceId"].(string)
	firstID := first["spanId"].(string)
	secondID := second["spanId"].(string)
	first["links"] = []any{map[string]any{"traceId": traceID, "spanId": secondID}}
	second["links"] = []any{map[string]any{"traceId": traceID, "spanId": firstID}}
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ImportTraceBytes(raw, DefaultTraceImportOptions())
	if err != nil {
		t.Fatal(err)
	}
	references := 0
	for _, link := range result.Trajectory.Links {
		if link.Kind == LinkReference {
			references++
		}
	}
	if references != 2 {
		t.Fatalf("reference links = %d, want 2", references)
	}
	if _, err := InspectTrace(result); err != nil {
		t.Fatalf("non-causal reference cycle rejected: %v", err)
	}
}

func TestTraceImportAllocationCeiling(t *testing.T) {
	raw := readTraceFixture(t, "otlp-genai-1.41.0.json")
	var importErr error
	allocations := testing.AllocsPerRun(10, func() {
		_, importErr = ImportTraceBytes(raw, DefaultTraceImportOptions())
	})
	if importErr != nil {
		t.Fatal(importErr)
	}
	if allocations > 10_000 {
		t.Fatalf("fixture import allocations %.0f exceed ceiling 10000", allocations)
	}
}

func TestTraceFixtureManifestPinsSchemasLicensesAndCommits(t *testing.T) {
	var manifest struct {
		SchemaVersion string `json:"schema_version"`
		Fixtures      []struct {
			Path           string `json:"path"`
			Format         string `json:"format"`
			Version        string `json:"version"`
			UpstreamCommit string `json:"upstream_commit"`
			License        string `json:"license"`
			Derivation     string `json:"derivation"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(readTraceFixture(t, "manifest.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "evalwitness.trace-fixture-manifest.v1" || len(manifest.Fixtures) != 2 {
		t.Fatalf("fixture manifest = %+v", manifest)
	}
	for _, fixture := range manifest.Fixtures {
		if fixture.Path == "" || fixture.Format == "" || fixture.Version == "" || len(fixture.UpstreamCommit) != 40 || fixture.License == "" || fixture.Derivation == "" {
			t.Fatalf("fixture provenance is incomplete: %+v", fixture)
		}
		if _, err := os.Stat("testdata/trace/" + fixture.Path); err != nil {
			t.Fatal(err)
		}
	}
}

func FuzzImportTraceNeverPanics(f *testing.F) {
	f.Add(readTraceFixture(f, "agent-trace-0.1.0.json"))
	f.Add(readTraceFixture(f, "otlp-genai-1.41.0.json"))
	f.Add([]byte(`{"data":[{"spans":[]}]}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		options := DefaultTraceImportOptions()
		options.Ingest.MaxSourceBytes = 1 << 20
		result, err := ImportTraceBytes(raw, options)
		if err == nil && (len(result.Trajectory.Events) > 100_000 || len(result.Mapping.Entries) > 200_000) {
			t.Fatal("bounded import produced an unbounded result")
		}
	})
}

func firstContribution(t *testing.T, trajectory Trajectory) ContributionPayload {
	t.Helper()
	for _, event := range trajectory.Events {
		if event.Contribution != nil {
			return *event.Contribution
		}
	}
	t.Fatal("trajectory has no contribution event")
	return ContributionPayload{}
}

func trajectoryContains(trajectory Trajectory, value string) bool {
	raw, err := json.Marshal(trajectory)
	return err == nil && bytes.Contains(raw, []byte(value))
}
