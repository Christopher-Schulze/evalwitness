package registry

import "github.com/Christopher-Schulze/evalwitness/protocol"

const (
	PublicDerivativeSchemaVersion = "evalwitness.registry-public-derivative.v1"
	PublicDerivativeCeiling       = "mechanism_conformance"
)

type PublicDerivative struct {
	SchemaVersion      string   `json:"schema_version"`
	EntryID            string   `json:"entry_id"`
	CapsuleDigest      string   `json:"capsule_digest"`
	ProfileDigest      string   `json:"profile_digest"`
	ServedModel        string   `json:"served_model"`
	Status             string   `json:"status"`
	EvidenceCeiling    string   `json:"evidence_ceiling"`
	Omitted            []string `json:"omitted"`
	CommunityValidated bool     `json:"community_validated"`
	Digest             string   `json:"digest"`
}

func RenderPublicDerivative(entry IntakeEntry) (PublicDerivative, error) {
	if err := ValidateIntake(entry); err != nil {
		return PublicDerivative{}, err
	}
	derivative := PublicDerivative{
		SchemaVersion:      PublicDerivativeSchemaVersion,
		EntryID:            entry.EntryID,
		CapsuleDigest:      entry.CapsuleDigest,
		ProfileDigest:      entry.ProfileDigest,
		ServedModel:        entry.ServedModel,
		Status:             entry.Status,
		EvidenceCeiling:    PublicDerivativeCeiling,
		CommunityValidated: false,
		Omitted: []string{
			"provider credentials",
			"local configuration and workstation paths",
			"operational logs and account metadata",
			"private capsule payloads",
		},
	}
	digest, err := protocol.Digest(unsignedPublicDerivative(derivative))
	if err != nil {
		return PublicDerivative{}, err
	}
	derivative.Digest = digest
	return derivative, nil
}

func unsignedPublicDerivative(derivative PublicDerivative) PublicDerivative {
	derivative.Digest = ""
	return derivative
}
