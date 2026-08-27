package lineage

import "errors"

type DatasetCardCount struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type VerificationLineageDatasetCard struct {
	Header                ArtifactHeader     `json:"header"`
	DatasetID             string             `json:"dataset_id"`
	Title                 string             `json:"title"`
	Purpose               string             `json:"purpose"`
	SourcePopulations     []string           `json:"source_populations"`
	Formats               []string           `json:"formats"`
	AgentEcosystems       []string           `json:"agent_ecosystems"`
	Roles                 []DataRole         `json:"roles"`
	ClusterDimensions     []string           `json:"cluster_dimensions"`
	InclusionCriteria     []string           `json:"inclusion_criteria"`
	ExclusionCriteria     []string           `json:"exclusion_criteria"`
	Counts                []DatasetCardCount `json:"counts"`
	License               string             `json:"license"`
	PrivacyProjection     string             `json:"privacy_projection"`
	RedactionLossPolicy   string             `json:"redaction_loss_policy"`
	IntendedUses          []string           `json:"intended_uses"`
	OutOfScopeUses        []string           `json:"out_of_scope_uses"`
	KnownLimitations      []string           `json:"known_limitations"`
	ReproductionCommand   string             `json:"reproduction_command"`
	ProviderCallsRequired int                `json:"provider_calls_required"`
}

func (card VerificationLineageDatasetCard) Validate() error {
	if err := validateHeader(card.Header, DatasetCardSchemaVersion, []ParentRequirement{
		{Relation: "plan", SchemaVersions: []string{PlanSchemaVersion}, Minimum: 1, Maximum: 1, SameTask: true},
		{Relation: "audit", SchemaVersions: []string{AuditSchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true},
		{Relation: "capability", SchemaVersions: []string{CapabilitySchemaVersion}, Minimum: 1, Maximum: -1, SameTask: true},
	}); err != nil {
		return err
	}
	if missing(card.DatasetID, card.Title, card.Purpose, card.License, card.PrivacyProjection, card.RedactionLossPolicy, card.ReproductionCommand) ||
		card.ProviderCallsRequired != 0 || len(card.Counts) == 0 {
		return errors.New("verification-lineage dataset card identity or offline boundary is invalid")
	}
	for name, values := range map[string][]string{
		"dataset source populations": card.SourcePopulations,
		"dataset formats":            card.Formats,
		"dataset agent ecosystems":   card.AgentEcosystems,
		"dataset cluster dimensions": card.ClusterDimensions,
		"dataset inclusion criteria": card.InclusionCriteria,
		"dataset exclusion criteria": card.ExclusionCriteria,
		"dataset intended uses":      card.IntendedUses,
		"dataset out-of-scope uses":  card.OutOfScopeUses,
		"dataset known limitations":  card.KnownLimitations,
	} {
		if err := validateSortedUnique(name, values, 1); err != nil {
			return err
		}
	}
	if err := validateRoles(card.Roles, true); err != nil || len(card.Roles) == 0 {
		return errors.New("dataset roles must be supported, non-empty, unique, and sorted")
	}
	previous := ""
	for _, count := range card.Counts {
		if missing(count.Name) || count.Name <= previous || count.Value < 0 {
			return errors.New("dataset-card counts must be non-negative, unique, and sorted")
		}
		previous = count.Name
	}
	copy := card
	copy.Header.Digest = ""
	return validateArtifactDigest(card.Header.Digest, copy)
}
