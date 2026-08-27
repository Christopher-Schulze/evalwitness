package lineage

import (
	"errors"
	"reflect"
	"slices"
	"time"
)

const SourceSpecificationRegistryVersion = "evalwitness.trace-source-specification-registry.v1"

type SpecificationEvidence struct {
	URL            string `json:"url"`
	UpstreamCommit string `json:"upstream_commit"`
	ContentDigest  string `json:"content_digest"`
	Mutable        bool   `json:"mutable"`
}

type ExpectedCapability struct {
	Field            string          `json:"field"`
	Representability CapabilityState `json:"representability"`
	Evidence         string          `json:"evidence"`
}

type TraceSourceSpecification struct {
	ID                   string                  `json:"id"`
	Format               string                  `json:"format"`
	FormatVersion        string                  `json:"format_version"`
	UpstreamRepository   string                  `json:"upstream_repository"`
	UpstreamCommit       string                  `json:"upstream_commit"`
	License              string                  `json:"license"`
	LicenseEvidence      SpecificationEvidence   `json:"license_evidence"`
	SpecificationStatus  string                  `json:"specification_status"`
	SchemaURL            string                  `json:"schema_url"`
	Admission            string                  `json:"admission"`
	RedistributionPolicy string                  `json:"redistribution_policy"`
	NormativeSources     []SpecificationEvidence `json:"normative_sources"`
	CapabilityBasis      string                  `json:"capability_basis"`
	ExpectedCapabilities []ExpectedCapability    `json:"expected_capabilities"`
	Limitations          []string                `json:"limitations"`
}

type TraceSourceSpecificationRegistry struct {
	SchemaVersion             string                     `json:"schema_version"`
	CanonicalPolicy           string                     `json:"canonical_policy"`
	PlanDigest                string                     `json:"plan_digest"`
	ResearchedAt              time.Time                  `json:"researched_at"`
	ProviderCallsAllowed      int                        `json:"provider_calls_allowed"`
	LaboratoryMayLaunchAgents bool                       `json:"laboratory_may_launch_agents"`
	Specifications            []TraceSourceSpecification `json:"specifications"`
	Digest                    string                     `json:"digest"`
}

func DefaultTraceSourceSpecificationRegistry() (TraceSourceSpecificationRegistry, error) {
	registry := TraceSourceSpecificationRegistry{
		SchemaVersion:   SourceSpecificationRegistryVersion,
		CanonicalPolicy: CanonicalPolicy,
		PlanDigest:      LockedPlanDigest,
		ResearchedAt:    time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
		Specifications:  sourceSpecifications(),
	}
	digest, err := sourceSpecificationRegistryDigest(registry)
	if err != nil {
		return TraceSourceSpecificationRegistry{}, err
	}
	registry.Digest = digest
	return registry, registry.Validate()
}

func (registry TraceSourceSpecificationRegistry) Validate() error {
	expected := TraceSourceSpecificationRegistry{
		SchemaVersion:   SourceSpecificationRegistryVersion,
		CanonicalPolicy: CanonicalPolicy,
		PlanDigest:      LockedPlanDigest,
		ResearchedAt:    time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
		Specifications:  sourceSpecifications(),
	}
	if registry.SchemaVersion != expected.SchemaVersion || registry.CanonicalPolicy != expected.CanonicalPolicy ||
		registry.PlanDigest != expected.PlanDigest || !registry.ResearchedAt.Equal(expected.ResearchedAt) ||
		registry.ProviderCallsAllowed != 0 || registry.LaboratoryMayLaunchAgents || !validDigest(registry.Digest) {
		return errors.New("trace source specification registry identity is invalid")
	}
	if !reflect.DeepEqual(registry.Specifications, expected.Specifications) {
		return errors.New("trace source specification registry differs from the pinned source contract")
	}
	for _, specification := range registry.Specifications {
		if err := validateSourceSpecification(specification); err != nil {
			return err
		}
	}
	digest, err := sourceSpecificationRegistryDigest(registry)
	if err != nil {
		return err
	}
	if registry.Digest != digest {
		return errors.New("trace source specification registry digest is invalid")
	}
	return nil
}

func validateSourceSpecification(specification TraceSourceSpecification) error {
	if missing(specification.ID, specification.Format, specification.FormatVersion, specification.UpstreamRepository,
		specification.UpstreamCommit, specification.License, specification.SpecificationStatus, specification.Admission,
		specification.RedistributionPolicy, specification.CapabilityBasis) || len(specification.NormativeSources) == 0 ||
		len(specification.Limitations) == 0 {
		return errors.New("trace source specification is incomplete")
	}
	if err := validateSpecificationEvidence(specification.LicenseEvidence); err != nil {
		return err
	}
	for _, evidence := range specification.NormativeSources {
		if err := validateSpecificationEvidence(evidence); err != nil {
			return err
		}
	}
	fields := capabilityFieldNames()
	if len(specification.ExpectedCapabilities) != len(fields) {
		return errors.New("trace source specification capability surface is incomplete")
	}
	states := []CapabilityState{CapabilityRequired, CapabilityOptional, CapabilityUnsupported, CapabilityUnspecified}
	for index, capability := range specification.ExpectedCapabilities {
		if capability.Field != fields[index] || !slices.Contains(states, capability.Representability) || capability.Evidence == "" {
			return errors.New("trace source specification capability is invalid or out of order")
		}
	}
	return validateSortedUnique("trace source specification limitations", specification.Limitations, 1)
}

func validateSpecificationEvidence(evidence SpecificationEvidence) error {
	if evidence.URL == "" {
		return errors.New("trace source specification evidence URL is required")
	}
	if evidence.Mutable {
		if evidence.UpstreamCommit != "" || evidence.ContentDigest != "" {
			return errors.New("mutable specification evidence cannot claim immutable identity")
		}
		return nil
	}
	if evidence.UpstreamCommit == "" || !validDigest(evidence.ContentDigest) {
		return errors.New("immutable specification evidence requires commit and content digest")
	}
	return nil
}

func sourceSpecificationRegistryDigest(registry TraceSourceSpecificationRegistry) (string, error) {
	registry.Digest = ""
	return digestJSON(registry)
}

func capabilityFieldNames() []string {
	return []string{"call_id", "command_display", "exit_status", "redaction_provenance", "result", "role", "source_record", "timestamps", "tool_call", "transaction_boundaries"}
}

func expectedCapabilities(states []CapabilityState, evidence string) []ExpectedCapability {
	fields := capabilityFieldNames()
	capabilities := make([]ExpectedCapability, len(fields))
	for index := range fields {
		capabilities[index] = ExpectedCapability{Field: fields[index], Representability: states[index], Evidence: evidence}
	}
	return capabilities
}

func immutableEvidence(url, commit, digest string) SpecificationEvidence {
	return SpecificationEvidence{URL: url, UpstreamCommit: commit, ContentDigest: digest}
}

func sourceSpecifications() []TraceSourceSpecification {
	optional := CapabilityOptional
	unsupported := CapabilityUnsupported
	unspecified := CapabilityUnspecified
	return []TraceSourceSpecification{
		{
			ID: "agent_trace_json", Format: "agent_trace_json", FormatVersion: "0.1.0",
			UpstreamRepository: "https://github.com/cursor/agent-trace", UpstreamCommit: "2754f077f3e50c1fb5088183f5c9362077cc8ca1",
			License: "CC-BY-4.0", LicenseEvidence: SpecificationEvidence{URL: "https://github.com/cursor/agent-trace/blob/2754f077f3e50c1fb5088183f5c9362077cc8ca1/README.md#license", UpstreamCommit: "2754f077f3e50c1fb5088183f5c9362077cc8ca1", ContentDigest: "bd9f6c10a3e5253ae6260a7bba4d9cbabcca168d2bfb771470432b9b7cf655df"},
			SpecificationStatus: "versioned_rfc", SchemaURL: "https://agent-trace.dev/schemas/v1/trace-record.json", Admission: "attribution_control_only",
			RedistributionPolicy: "redistributable_with_attribution", NormativeSources: []SpecificationEvidence{immutableEvidence("https://github.com/cursor/agent-trace/blob/2754f077f3e50c1fb5088183f5c9362077cc8ca1/README.md", "2754f077f3e50c1fb5088183f5c9362077cc8ca1", "bd9f6c10a3e5253ae6260a7bba4d9cbabcca168d2bfb771470432b9b7cf655df")},
			CapabilityBasis:      "versioned RFC trace-record schema",
			ExpectedCapabilities: expectedCapabilities([]CapabilityState{unsupported, unsupported, unsupported, unsupported, unsupported, optional, CapabilityRequired, CapabilityRequired, unsupported, unsupported}, "Agent Trace 0.1.0 records attribution rather than runtime execution"),
			Limitations:          []string{"quality_assessment_is_an_explicit_non_goal", "runtime_command_and_result_lineage_is_not_defined"},
		},
		{
			ID: "claude_code_jsonl", Format: "claude_code_jsonl", FormatVersion: "unversioned-export",
			UpstreamRepository: "https://github.com/anthropics/claude-code", UpstreamCommit: "2bb60696142b493eafaeacfe00eac51d16c50c4f",
			License: "Anthropic-commercial-terms", LicenseEvidence: immutableEvidence("https://github.com/anthropics/claude-code/blob/2bb60696142b493eafaeacfe00eac51d16c50c4f/LICENSE.md", "2bb60696142b493eafaeacfe00eac51d16c50c4f", "728158fd1037143fad6907e8fa34804177e598b7326519503fe83cafdef849e6"),
			SpecificationStatus: "unversioned_documented_storage", Admission: "golden_vectors_required_before_confirmatory_use",
			RedistributionPolicy: "owner_authorization_required", NormativeSources: []SpecificationEvidence{{URL: "https://code.claude.com/docs/en/claude-directory", Mutable: true}},
			CapabilityBasis:      "official storage documentation without a stable field schema",
			ExpectedCapabilities: expectedCapabilities([]CapabilityState{unspecified, unspecified, unspecified, unspecified, unspecified, unspecified, unspecified, unspecified, unspecified, unspecified}, "official documentation describes JSONL record classes but no stable field contract"),
			Limitations:          []string{"exports_can_contain_secrets_and_private_reasoning", "no_stable_versioned_field_schema", "redistribution_is_not_granted_by_the_product_license"},
		},
		{
			ID: "codex_rollout_jsonl", Format: "codex_rollout_jsonl", FormatVersion: "unversioned-export",
			UpstreamRepository: "https://github.com/openai/codex", UpstreamCommit: "1c042dd4d823b451ae44029abaf0e13b7cef8904",
			License: "Apache-2.0", LicenseEvidence: immutableEvidence("https://github.com/openai/codex/blob/1c042dd4d823b451ae44029abaf0e13b7cef8904/LICENSE", "1c042dd4d823b451ae44029abaf0e13b7cef8904", "d17f227e4df5da1600391338865ce0f3055211760a36688f816941d58232d8dc"),
			SpecificationStatus: "commit_pinned_implementation_contract", Admission: "golden_vectors_required_before_confirmatory_use",
			RedistributionPolicy: "source_redistributable_capture_content_requires_separate_authority", NormativeSources: []SpecificationEvidence{immutableEvidence("https://github.com/openai/codex/blob/1c042dd4d823b451ae44029abaf0e13b7cef8904/codex-rs/protocol/src/protocol.rs", "1c042dd4d823b451ae44029abaf0e13b7cef8904", "03c395d104f2609793061c929145bd9fdde8250b1a282ac11a1bc49a414f1f03")},
			CapabilityBasis:      "commit-pinned Rust serialization types, not a separately versioned rollout schema",
			ExpectedCapabilities: expectedCapabilities([]CapabilityState{optional, optional, optional, unsupported, optional, optional, CapabilityRequired, CapabilityRequired, optional, optional}, "RolloutLine and RolloutItem source types at the pinned commit"),
			Limitations:          []string{"record_shape_can_change_without_a_rollout_schema_version", "sensitive_capture_content_has_separate_release_authority"},
		},
		{
			ID: "opencode_export_json", Format: "opencode_export_json", FormatVersion: "unversioned-export",
			UpstreamRepository: "https://github.com/anomalyco/opencode", UpstreamCommit: "550d1ffd24718454925c4636e937878f0274de48",
			License: "MIT", LicenseEvidence: immutableEvidence("https://github.com/anomalyco/opencode/blob/550d1ffd24718454925c4636e937878f0274de48/LICENSE", "550d1ffd24718454925c4636e937878f0274de48", "625f0f619133f89bbbb2abe37369613dfa1885eba1e50d02170deb62bb42cb6b"),
			SpecificationStatus: "commit_pinned_implementation_contract", Admission: "golden_vectors_required_before_confirmatory_use",
			RedistributionPolicy: "source_redistributable_capture_content_requires_separate_authority", NormativeSources: []SpecificationEvidence{immutableEvidence("https://github.com/anomalyco/opencode/blob/550d1ffd24718454925c4636e937878f0274de48/packages/opencode/src/cli/cmd/export.ts", "550d1ffd24718454925c4636e937878f0274de48", "ce20051b0ee3241459eb5566ac6ea83fbc6d701b13db866b8c9e7f5acc1bd328"), immutableEvidence("https://github.com/anomalyco/opencode/blob/550d1ffd24718454925c4636e937878f0274de48/packages/opencode/src/session/message-v2.ts", "550d1ffd24718454925c4636e937878f0274de48", "828789508afda97a1570b96413dd40ee9d9119dd25d057b5f0a985154378e619")},
			CapabilityBasis:      "commit-pinned export implementation and message-part types, not a separately versioned export schema",
			ExpectedCapabilities: expectedCapabilities([]CapabilityState{optional, optional, unspecified, optional, optional, CapabilityRequired, CapabilityRequired, optional, optional, optional}, "export and message-v2 source types at the pinned commit"),
			Limitations:          []string{"exit_status_semantics_are_not_a_stable_export_contract", "record_shape_can_change_without_an_export_schema_version"},
		},
		{
			ID: "otel_genai_development", Format: "otel_genai_development", FormatVersion: "development@46d43c8949afb53765a202e89f4534eeb75ca3fa",
			UpstreamRepository: "https://github.com/open-telemetry/semantic-conventions-genai", UpstreamCommit: "46d43c8949afb53765a202e89f4534eeb75ca3fa",
			License: "Apache-2.0", LicenseEvidence: immutableEvidence("https://github.com/open-telemetry/semantic-conventions-genai/blob/46d43c8949afb53765a202e89f4534eeb75ca3fa/LICENSE", "46d43c8949afb53765a202e89f4534eeb75ca3fa", "c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4"),
			SpecificationStatus: "development_no_schema_url", Admission: "rejected_until_immutable_schema_url",
			RedistributionPolicy: "source_redistributable_under_apache_2_0", NormativeSources: []SpecificationEvidence{immutableEvidence("https://github.com/open-telemetry/semantic-conventions-genai/blob/46d43c8949afb53765a202e89f4534eeb75ca3fa/README.md", "46d43c8949afb53765a202e89f4534eeb75ca3fa", "8ab882b40b743b3474de8d651671e0421e82dd0a8a48967d1bd0d7b06380b46d"), immutableEvidence("https://github.com/open-telemetry/semantic-conventions-genai/blob/46d43c8949afb53765a202e89f4534eeb75ca3fa/model/gen-ai/registry.yaml", "46d43c8949afb53765a202e89f4534eeb75ca3fa", "e9bb6ace35e86823b57cb6e3e0e4ed5596888e59d219b1fb26fdf93006f6cfff")},
			CapabilityBasis:      "development source snapshot; capability is descriptive and not an admitted interchange contract",
			ExpectedCapabilities: expectedCapabilities([]CapabilityState{optional, unsupported, unsupported, unsupported, optional, optional, CapabilityRequired, optional, optional, optional}, "development registry contains tool call ID, arguments, result, roles, and execute_tool operation"),
			Limitations:          []string{"official_schema_url_is_todo", "source_snapshot_is_not_an_admitted_stable_interchange_version"},
		},
		{
			ID: "otlp_json_genai", Format: "otlp_json_genai", FormatVersion: "otlp-json-1.8.0+semconv-1.41.0",
			UpstreamRepository: "https://github.com/open-telemetry/opentelemetry-proto", UpstreamCommit: "c0a98a1847d3124ac5f9ecd02d0e2d2732bbb590",
			License: "Apache-2.0", LicenseEvidence: immutableEvidence("https://github.com/open-telemetry/opentelemetry-proto/blob/c0a98a1847d3124ac5f9ecd02d0e2d2732bbb590/LICENSE", "c0a98a1847d3124ac5f9ecd02d0e2d2732bbb590", "c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4"),
			SpecificationStatus: "versioned_release", SchemaURL: "https://opentelemetry.io/schemas/1.41.0", Admission: "supported_pinned_interchange",
			RedistributionPolicy: "source_redistributable_under_apache_2_0", NormativeSources: []SpecificationEvidence{immutableEvidence("https://github.com/open-telemetry/opentelemetry-proto/blob/c0a98a1847d3124ac5f9ecd02d0e2d2732bbb590/opentelemetry/proto/trace/v1/trace.proto", "c0a98a1847d3124ac5f9ecd02d0e2d2732bbb590", "be3014cf9a3aedc61d045621106a1643f2c9df5d4d13e2ba8c274ccc9d59ee36"), immutableEvidence("https://github.com/open-telemetry/semantic-conventions/blob/e018fe6f91862f5ed63c082f87697cddac596784/model/gen-ai/registry.yaml", "e018fe6f91862f5ed63c082f87697cddac596784", "55ec5102aaa0fbdc1afcef0cebcca5676b6a549507f781425558e2c25d4d137f"), immutableEvidence("https://github.com/open-telemetry/semantic-conventions/blob/e018fe6f91862f5ed63c082f87697cddac596784/model/gen-ai/spans.yaml", "e018fe6f91862f5ed63c082f87697cddac596784", "ff7f1823dd0a3723455bdb2729b489093b3d687b0af680643e5a01455011d558")},
			CapabilityBasis:      "released OTLP trace structure plus immutable GenAI semantic-conventions schema",
			ExpectedCapabilities: expectedCapabilities([]CapabilityState{optional, unsupported, unsupported, unsupported, optional, optional, CapabilityRequired, CapabilityRequired, optional, optional}, "OTLP span structure and GenAI 1.41.0 attributes at pinned release commits"),
			Limitations:          []string{"command_display_has_no_genai_1_41_0_semantic_field", "process_exit_status_has_no_genai_1_41_0_semantic_field", "redaction_provenance_has_no_genai_1_41_0_semantic_field"},
		},
	}
}
