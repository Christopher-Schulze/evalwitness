package lineage

import (
	"bytes"
	"os"
	"testing"
)

func TestDefaultTraceSourceSpecificationRegistryMatchesCheckedInArtifact(t *testing.T) {
	registry, err := DefaultTraceSourceSpecificationRegistry()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(registry)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := os.ReadFile("../../eval/governance/trace-source-specifications-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, artifact) {
		t.Fatal("checked-in trace source specification registry differs from the canonical registry")
	}
}

func TestTraceSourceSpecificationRegistryPinsHonestBoundaries(t *testing.T) {
	registry, err := DefaultTraceSourceSpecificationRegistry()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]TraceSourceSpecification, len(registry.Specifications))
	for _, specification := range registry.Specifications {
		byID[specification.ID] = specification
	}
	claude := byID["claude_code_jsonl"]
	for _, capability := range claude.ExpectedCapabilities {
		if capability.Representability != CapabilityUnspecified {
			t.Fatalf("Claude unversioned field %q was promoted to %q", capability.Field, capability.Representability)
		}
	}
	if claude.RedistributionPolicy != "owner_authorization_required" || claude.SpecificationStatus != "unversioned_documented_storage" {
		t.Fatal("Claude source or redistribution boundary drifted")
	}
	agentTrace := byID["agent_trace_json"]
	for _, field := range []int{0, 1, 2, 3, 4, 8, 9} {
		if agentTrace.ExpectedCapabilities[field].Representability != CapabilityUnsupported {
			t.Fatal("Agent Trace attribution control acquired runtime capability")
		}
	}
	development := byID["otel_genai_development"]
	if development.SchemaURL != "" || development.Admission != "rejected_until_immutable_schema_url" {
		t.Fatal("development OpenTelemetry GenAI source was promoted to stable")
	}
	otlp := byID["otlp_json_genai"]
	if otlp.FormatVersion != "otlp-json-1.8.0+semconv-1.41.0" || otlp.SchemaURL != "https://opentelemetry.io/schemas/1.41.0" {
		t.Fatal("pinned OTLP release identity drifted")
	}
}

func TestTraceSourceSpecificationRegistryRejectsResealedDrift(t *testing.T) {
	registry, err := DefaultTraceSourceSpecificationRegistry()
	if err != nil {
		t.Fatal(err)
	}
	registry.Specifications[1].ExpectedCapabilities[0].Representability = CapabilityOptional
	registry.Digest, err = sourceSpecificationRegistryDigest(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Validate(); err == nil {
		t.Fatal("resealed source specification drift was accepted")
	}
}

func TestCapabilitySchemaIncludesUnspecifiedRepresentability(t *testing.T) {
	schema, err := Schema("capability-vector")
	if err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	fields := properties["fields"].(JSONSchema)["items"].(JSONSchema)
	fieldProperties := fields["properties"].(map[string]any)
	states := fieldProperties["representable_by_format"].(JSONSchema)["enum"].([]string)
	found := false
	for _, state := range states {
		found = found || state == string(CapabilityUnspecified)
	}
	if !found {
		t.Fatal("capability schema omits unspecified representability")
	}
}
