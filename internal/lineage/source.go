package lineage

import (
	"errors"
	"slices"
)

type CaptureMode string

const (
	CaptureNativeExport CaptureMode = "native_export"
	CapturePaired       CaptureMode = "paired_native_and_witness"
)

type VerificationLineageSource struct {
	Header                    ArtifactHeader `json:"header"`
	SourceClass               string         `json:"source_class"`
	AgentEcosystem            string         `json:"agent_ecosystem"`
	RuntimeIdentityClass      string         `json:"runtime_identity_class"`
	ProviderMetadata          string         `json:"provider_metadata"`
	ExportFormat              string         `json:"export_format"`
	ExportVersion             string         `json:"export_version"`
	CaptureMode               CaptureMode    `json:"capture_mode"`
	SourceSessionID           string         `json:"source_session_id"`
	LineageID                 string         `json:"lineage_id"`
	NearDuplicateID           string         `json:"near_duplicate_id"`
	RepositoryID              string         `json:"repository_id"`
	RepositoryAlias           string         `json:"repository_alias"`
	TaskAlias                 string         `json:"task_alias"`
	License                   string         `json:"license"`
	RedistributionPermission  string         `json:"redistribution_permission"`
	PrivacyClass              string         `json:"privacy_class"`
	RedactionPolicy           string         `json:"redaction_policy"`
	AuthoritativeSurface      bool           `json:"authoritative_surface"`
	RawRecordCount            int            `json:"raw_record_count"`
	RawRecordDigest           string         `json:"raw_record_digest"`
	CanonicalTrajectoryDigest string         `json:"canonical_trajectory_digest"`
	FieldAccountingDigest     string         `json:"field_accounting_digest"`
}

func (source VerificationLineageSource) Validate() error {
	if err := validateHeader(source.Header, SourceSchemaVersion, []ParentRequirement{
		{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
	}); err != nil {
		return err
	}
	if missing(source.SourceClass, source.AgentEcosystem, source.RuntimeIdentityClass, source.ProviderMetadata,
		source.ExportFormat, source.ExportVersion, source.SourceSessionID, source.LineageID, source.NearDuplicateID,
		source.RepositoryID, source.RepositoryAlias, source.TaskAlias,
		source.License, source.RedistributionPermission, source.PrivacyClass, source.RedactionPolicy) {
		return errors.New("verification-lineage source metadata is incomplete")
	}
	if !slices.Contains([]CaptureMode{CaptureNativeExport, CapturePaired}, source.CaptureMode) || source.RawRecordCount < 1 ||
		!validDigest(source.RawRecordDigest) || !validDigest(source.CanonicalTrajectoryDigest) || !validDigest(source.FieldAccountingDigest) {
		return errors.New("verification-lineage source capture or content identity is invalid")
	}
	copy := source
	copy.Header.Digest = ""
	return validateArtifactDigest(source.Header.Digest, copy)
}
