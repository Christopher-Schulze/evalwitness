package lineage

import (
	"errors"
	"reflect"
)

const SourceInventoryVersion = "evalwitness.verification-lineage-source-inventory.v1"

type SourceInventoryCandidate struct {
	ID                         string     `json:"id"`
	SourceClass                string     `json:"source_class"`
	Materialized               bool       `json:"materialized"`
	Paths                      []string   `json:"paths"`
	ManifestDigest             string     `json:"manifest_digest"`
	Formats                    []string   `json:"formats"`
	PermittedRoles             []DataRole `json:"permitted_roles"`
	LicenseStatus              string     `json:"license_status"`
	ConsentStatus              string     `json:"consent_status"`
	PrivacyStatus              string     `json:"privacy_status"`
	RedistributionStatus       string     `json:"redistribution_status"`
	ExportVersionStatus        string     `json:"export_version_status"`
	TaskLineageStatus          string     `json:"task_lineage_status"`
	NearDuplicateStatus        string     `json:"near_duplicate_status"`
	AuthoritativeCaptureStatus string     `json:"authoritative_capture_status"`
	AdmissionStatus            string     `json:"admission_status"`
	Reasons                    []string   `json:"reasons"`
}

type SourceInventory struct {
	SchemaVersion             string                     `json:"schema_version"`
	CanonicalPolicy           string                     `json:"canonical_policy"`
	PlanDigest                string                     `json:"plan_digest"`
	InventoryState            string                     `json:"inventory_state"`
	TaskGroupCountsInspected  bool                       `json:"task_group_counts_inspected"`
	ExternalActionStatus      string                     `json:"external_action_status"`
	ProviderCallsAllowed      int                        `json:"provider_calls_allowed"`
	LaboratoryMayLaunchAgents bool                       `json:"laboratory_may_launch_agents"`
	Candidates                []SourceInventoryCandidate `json:"candidates"`
	Digest                    string                     `json:"digest"`
}

func DefaultSourceInventory() (SourceInventory, error) {
	inventory := SourceInventory{
		SchemaVersion: SourceInventoryVersion, CanonicalPolicy: CanonicalPolicy, PlanDigest: LockedPlanDigest,
		InventoryState: "pre_acquisition_candidate_inventory", ExternalActionStatus: ExternalActionNotAuthorized,
		Candidates: sourceInventoryCandidates(),
	}
	digest, err := sourceInventoryDigest(inventory)
	if err != nil {
		return SourceInventory{}, err
	}
	inventory.Digest = digest
	return inventory, inventory.Validate()
}

func (inventory SourceInventory) Validate() error {
	expected := SourceInventory{
		SchemaVersion: SourceInventoryVersion, CanonicalPolicy: CanonicalPolicy, PlanDigest: LockedPlanDigest,
		InventoryState: "pre_acquisition_candidate_inventory", ExternalActionStatus: ExternalActionNotAuthorized,
		Candidates: sourceInventoryCandidates(),
	}
	if inventory.SchemaVersion != expected.SchemaVersion || inventory.CanonicalPolicy != expected.CanonicalPolicy ||
		inventory.PlanDigest != expected.PlanDigest || inventory.InventoryState != expected.InventoryState ||
		inventory.TaskGroupCountsInspected || inventory.ExternalActionStatus != ExternalActionNotAuthorized ||
		inventory.ProviderCallsAllowed != 0 || inventory.LaboratoryMayLaunchAgents || !validDigest(inventory.Digest) {
		return errors.New("verification-lineage source inventory identity is invalid")
	}
	if !reflect.DeepEqual(inventory.Candidates, expected.Candidates) {
		return errors.New("verification-lineage source inventory differs from the sealed candidate contract")
	}
	for _, candidate := range inventory.Candidates {
		if err := validateSourceInventoryCandidate(candidate); err != nil {
			return err
		}
	}
	digest, err := sourceInventoryDigest(inventory)
	if err != nil {
		return err
	}
	if inventory.Digest != digest {
		return errors.New("verification-lineage source inventory digest is invalid")
	}
	return nil
}

func validateSourceInventoryCandidate(candidate SourceInventoryCandidate) error {
	if missing(candidate.ID, candidate.SourceClass, candidate.LicenseStatus, candidate.ConsentStatus,
		candidate.PrivacyStatus, candidate.RedistributionStatus, candidate.ExportVersionStatus,
		candidate.TaskLineageStatus, candidate.NearDuplicateStatus, candidate.AuthoritativeCaptureStatus,
		candidate.AdmissionStatus) || len(candidate.Formats) == 0 || len(candidate.PermittedRoles) == 0 || len(candidate.Reasons) == 0 {
		return errors.New("verification-lineage source inventory candidate is incomplete")
	}
	if candidate.Materialized != (len(candidate.Paths) > 0) || candidate.Materialized != validDigest(candidate.ManifestDigest) {
		return errors.New("verification-lineage source inventory materialization identity is invalid")
	}
	if err := validateSortedUnique("source inventory paths", candidate.Paths, 0); err != nil {
		return err
	}
	if err := validateSortedUnique("source inventory formats", candidate.Formats, 1); err != nil {
		return err
	}
	if err := validateSortedUnique("source inventory reasons", candidate.Reasons, 1); err != nil {
		return err
	}
	return nil
}

func sourceInventoryDigest(inventory SourceInventory) (string, error) {
	inventory.Digest = ""
	return digestJSON(inventory)
}

func sourceInventoryCandidates() []SourceInventoryCandidate {
	development := []DataRole{RoleAdapterDevelopment}
	return []SourceInventoryCandidate{
		{
			ID: "checked_in_agent_trace_synthetic", SourceClass: "checked_in_controls", Materialized: true,
			Paths: []string{"internal/preprocess/testdata/trace/agent-trace-0.1.0.json", "internal/preprocess/testdata/trace/manifest.json"}, ManifestDigest: "7342988b349710d4b2668e83405513fdf78118b2e62374e8fde9727a8d7a8aed",
			Formats: []string{"agent_trace_json"}, PermittedRoles: development,
			LicenseStatus: "verified_cc_by_4_0", ConsentStatus: "not_applicable_synthetic", PrivacyStatus: "synthetic_public",
			RedistributionStatus: "verified_with_attribution", ExportVersionStatus: "pinned_0_1_0", TaskLineageStatus: "synthetic_fixture_only",
			NearDuplicateStatus: "isolated_control", AuthoritativeCaptureStatus: "absent_synthetic_attribution_record", AdmissionStatus: "admitted_adapter_development_only",
			Reasons: []string{"not_a_native_agent_execution", "runtime_verification_lineage_is_not_representable"},
		},
		{
			ID: "checked_in_otlp_synthetic", SourceClass: "checked_in_controls", Materialized: true,
			Paths: []string{"internal/preprocess/testdata/trace/manifest.json", "internal/preprocess/testdata/trace/otlp-genai-1.41.0.json"}, ManifestDigest: "7342988b349710d4b2668e83405513fdf78118b2e62374e8fde9727a8d7a8aed",
			Formats: []string{"otlp_json_genai"}, PermittedRoles: development,
			LicenseStatus: "verified_apache_2_0", ConsentStatus: "not_applicable_synthetic", PrivacyStatus: "synthetic_public",
			RedistributionStatus: "verified_apache_2_0", ExportVersionStatus: "pinned_otlp_1_8_0_semconv_1_41_0", TaskLineageStatus: "synthetic_fixture_only",
			NearDuplicateStatus: "isolated_control", AuthoritativeCaptureStatus: "absent_synthetic_trace", AdmissionStatus: "admitted_adapter_development_only",
			Reasons: []string{"no_independent_execution_witness", "not_a_native_agent_execution"},
		},
		{
			ID: "checked_in_vendor_goldens", SourceClass: "checked_in_controls", Materialized: true,
			Paths: []string{"internal/preprocess/testdata/golden/claude-code.jsonl", "internal/preprocess/testdata/golden/codex-rollout.jsonl", "internal/preprocess/testdata/golden/manifest.json", "internal/preprocess/testdata/golden/opencode-export.json"}, ManifestDigest: "45cec90046d02209c1acf6f740f724b07bec5322b5d44d8be77461a1e41ceddd",
			Formats: []string{"claude_code_jsonl", "codex_rollout_jsonl", "opencode_export_json"}, PermittedRoles: development,
			LicenseStatus: "unresolved_per_source", ConsentStatus: "unresolved_for_task_069", PrivacyStatus: "historically_sanitized_not_task_069_attested",
			RedistributionStatus: "unresolved_per_source", ExportVersionStatus: "unversioned_historical_shapes", TaskLineageStatus: "missing_task_069_source_manifest",
			NearDuplicateStatus: "unresolved", AuthoritativeCaptureStatus: "absent", AdmissionStatus: "rejected_for_corpus_adapter_development_only",
			Reasons: []string{"authoritative_execution_witness_absent", "license_consent_and_task_lineage_not_sealed", "manifest_proves_conversion_counts_not_research_admissibility"},
		},
		{
			ID: "explicitly_authorized_owner_captures", SourceClass: "explicitly_authorized_owner_captures",
			Formats: []string{"claude_code_jsonl", "codex_rollout_jsonl", "opencode_export_json"}, PermittedRoles: []DataRole{RoleAdapterDevelopment, RoleCaptureCalibration, RoleLockedTest},
			LicenseStatus: "requires_owner_authority", ConsentStatus: "required_not_obtained", PrivacyStatus: "requires_pre_capture_policy",
			RedistributionStatus: "not_authorized", ExportVersionStatus: "requires_exact_cli_and_export_identity", TaskLineageStatus: "not_assigned",
			NearDuplicateStatus: "not_assessed", AuthoritativeCaptureStatus: "requires_paired_capture", AdmissionStatus: "not_admitted_external_action_not_authorized",
			Reasons: []string{"capture_command_and_spend_boundary_not_sealed", "owner_authorization_not_obtained", "source_bytes_not_materialized"},
		},
		{
			ID: "public_licensed_native_exports", SourceClass: "public_licensed_native_exports",
			Formats: []string{"claude_code_jsonl", "codex_rollout_jsonl", "opencode_export_json"}, PermittedRoles: []DataRole{RoleAdapterDevelopment, RoleCaptureCalibration, RoleLockedTest},
			LicenseStatus: "requires_per_source_verification", ConsentStatus: "requires_publication_basis", PrivacyStatus: "requires_per_source_review",
			RedistributionStatus: "requires_per_source_verification", ExportVersionStatus: "requires_exact_native_export_identity", TaskLineageStatus: "not_assigned",
			NearDuplicateStatus: "not_assessed", AuthoritativeCaptureStatus: "requires_paired_capture_or_typed_absence", AdmissionStatus: "not_admitted_no_source_selected",
			Reasons: []string{"no_candidate_source_selected", "source_bytes_not_materialized", "task_group_counts_not_inspected"},
		},
	}
}
