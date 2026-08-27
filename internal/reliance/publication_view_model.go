package reliance

const (
	RelianceProfileProjectionSchemaVersion = "evalwitness.reliance-profile-projection.v1"
	ReliancePaperProjectionSchemaVersion   = "evalwitness.reliance-paper-projection.v1"
)

type RelianceProfileStatusCount struct {
	Status     string `json:"status"`
	Dimensions int    `json:"dimensions"`
}

type RelianceProfileProjection struct {
	SchemaVersion         string                       `json:"schema_version"`
	CapsuleID             string                       `json:"capsule_id"`
	ManifestDigest        string                       `json:"manifest_digest"`
	MapComponentID        string                       `json:"map_component_id"`
	MapDigest             string                       `json:"map_digest"`
	LedgerDigest          string                       `json:"ledger_digest"`
	Scope                 ReliancePublicationScope     `json:"scope"`
	Dimensions            []RelianceProfileDimension   `json:"dimensions"`
	StatusCounts          []RelianceProfileStatusCount `json:"status_counts"`
	GlobalScoreProhibited bool                         `json:"global_score_prohibited"`
	ProviderCalls         int                          `json:"provider_calls"`
	NetworkRequired       bool                         `json:"network_required"`
	Digest                string                       `json:"digest"`
}

type ReliancePaperProjection struct {
	SchemaVersion       string                   `json:"schema_version"`
	CapsuleID           string                   `json:"capsule_id"`
	ManifestDigest      string                   `json:"manifest_digest"`
	MapComponentID      string                   `json:"map_component_id"`
	MapDigest           string                   `json:"map_digest"`
	LedgerDigest        string                   `json:"ledger_digest"`
	Scope               ReliancePublicationScope `json:"scope"`
	Rows                []ReliancePaperRow       `json:"rows"`
	Limitations         []string                 `json:"limitations"`
	CurrentClaimIDs     []string                 `json:"current_claim_ids"`
	UnsupportedClaimIDs []string                 `json:"unsupported_claim_ids"`
	ProviderCalls       int                      `json:"provider_calls"`
	NetworkRequired     bool                     `json:"network_required"`
	Digest              string                   `json:"digest"`
}
