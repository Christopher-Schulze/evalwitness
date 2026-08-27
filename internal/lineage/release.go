package lineage

import "errors"

type ReleaseFile struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Bytes  int64  `json:"bytes"`
	Digest string `json:"digest"`
}

type VerificationLineageRelease struct {
	Header                     ArtifactHeader `json:"header"`
	ReleaseID                  string         `json:"release_id"`
	Version                    string         `json:"version"`
	Files                      []ReleaseFile  `json:"files"`
	SchemaInventoryDigest      string         `json:"schema_inventory_digest"`
	AuditDigest                string         `json:"audit_digest"`
	DatasetCardDigest          string         `json:"dataset_card_digest"`
	LimitationsDigest          string         `json:"limitations_digest"`
	LineageGraphJSONDigest     string         `json:"lineage_graph_json_digest"`
	LineageGraphSVGDigest      string         `json:"lineage_graph_svg_digest"`
	ReproductionCommand        string         `json:"reproduction_command"`
	ProviderCallsRequired      int            `json:"provider_calls_required"`
	AllFilesVerified           bool           `json:"all_files_verified"`
	PublicProjection           bool           `json:"public_projection"`
	RestrictedMaterialExcluded bool           `json:"restricted_material_excluded"`
}

func (release VerificationLineageRelease) Validate() error {
	if err := validateHeader(release.Header, ReleaseSchemaVersion, []ParentRequirement{
		{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
		{Relation: "audit", SchemaVersions: []string{AuditSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
		{Relation: "bom", SchemaVersions: []string{BOMSchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true},
		{Relation: "dataset_card", SchemaVersions: []string{DatasetCardSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
	}); err != nil {
		return err
	}
	if missing(release.ReleaseID, release.Version, release.ReproductionCommand) || release.ProviderCallsRequired != 0 || !release.AllFilesVerified ||
		!release.PublicProjection || !release.RestrictedMaterialExcluded || len(release.Files) == 0 {
		return errors.New("verification-lineage release identity or offline verification boundary is invalid")
	}
	for _, digest := range []string{release.SchemaInventoryDigest, release.AuditDigest, release.DatasetCardDigest, release.LimitationsDigest, release.LineageGraphJSONDigest, release.LineageGraphSVGDigest} {
		if !validDigest(digest) {
			return errors.New("verification-lineage release contains an invalid artifact digest")
		}
	}
	auditDigest, auditFound := parentDigest(release.Header, "audit")
	cardDigest, cardFound := parentDigest(release.Header, "dataset_card")
	if !auditFound || !cardFound || release.AuditDigest != auditDigest || release.DatasetCardDigest != cardDigest {
		return errors.New("verification-lineage release parent digests do not match its manifest bindings")
	}
	previous := ""
	for _, file := range release.Files {
		if !validReleasePath(file.Path) || missing(file.Role) || file.Path <= previous || file.Bytes < 1 || !validDigest(file.Digest) {
			return errors.New("release files must be safe, non-empty, unique, and sorted")
		}
		previous = file.Path
	}
	copy := release
	copy.Header.Digest = ""
	return validateArtifactDigest(release.Header.Digest, copy)
}
