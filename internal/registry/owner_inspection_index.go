package registry

import (
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const OwnerInspectionIndexSchemaVersion = "evalwitness.registry-owner-inspection-index.v1"

type OwnerInspectionRegistryRecord struct {
	SchemaVersion           string   `json:"schema_version"`
	AttestationSchema       string   `json:"attestation_schema"`
	AttestationDigest       string   `json:"attestation_digest"`
	PackageInventoryDigest  string   `json:"package_inventory_digest"`
	InspectionDate          string   `json:"inspection_date"`
	RequiredAssessments     int      `json:"required_assessments"`
	DimensionCount          int      `json:"dimension_count"`
	CoreStatus              string   `json:"core_status"`
	ScarcityStatus          string   `json:"scarcity_status"`
	OverallStatus           string   `json:"overall_status"`
	HumanStudyStatus        string   `json:"human_study_status"`
	ExternalActionStatus    string   `json:"external_action_status"`
	SourceReproduction      string   `json:"source_reproduction"`
	Rankable                bool     `json:"rankable"`
	IndependentlyReproduced bool     `json:"independently_reproduced"`
	CommunityValidated      bool     `json:"community_validated"`
	HumanSupported          bool     `json:"human_supported"`
	Limitations             []string `json:"limitations"`
	Digest                  string   `json:"digest"`
}

func IndexOwnerInspectionAttestation(path string) (record OwnerInspectionRegistryRecord, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return OwnerInspectionRegistryRecord{}, fmt.Errorf("owner-inspection index: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("owner-inspection index: close source: %w", closeErr)
		}
	}()
	attestation, err := relation.DecodeOwnerInspectionPublicAttestation(file)
	if err != nil {
		return OwnerInspectionRegistryRecord{}, fmt.Errorf("owner-inspection index: %w", err)
	}
	if attestation.Assessments.Required != 66 {
		return OwnerInspectionRegistryRecord{}, fmt.Errorf("owner-inspection index: required assessments must be 66")
	}
	if attestation.Disclosure.PrivateJournalIdentitiesDisclosed || attestation.Disclosure.RestrictedEvidenceDisclosed {
		return OwnerInspectionRegistryRecord{}, fmt.Errorf("owner-inspection index: private payloads are forbidden")
	}
	if attestation.HumanStudyStatus != "not_run" || string(attestation.ExternalActionStatus) != "not_authorized" {
		return OwnerInspectionRegistryRecord{}, fmt.Errorf("owner-inspection index: human/external boundaries drifted")
	}
	record = OwnerInspectionRegistryRecord{
		SchemaVersion:           OwnerInspectionIndexSchemaVersion,
		AttestationSchema:       attestation.SchemaVersion,
		AttestationDigest:       attestation.Digest,
		PackageInventoryDigest:  attestation.PackageInventoryDigest,
		InspectionDate:          attestation.InspectionDate,
		RequiredAssessments:     attestation.Assessments.Required,
		DimensionCount:          len(attestation.Dimensions),
		CoreStatus:              string(attestation.Outcomes.CoreStatus),
		ScarcityStatus:          string(attestation.Outcomes.ScarcityStatus),
		OverallStatus:           string(attestation.Outcomes.OverallStatus),
		HumanStudyStatus:        attestation.HumanStudyStatus,
		ExternalActionStatus:    string(attestation.ExternalActionStatus),
		SourceReproduction:      attestation.Disclosure.SourceReproduction,
		Rankable:                false,
		IndependentlyReproduced: false,
		CommunityValidated:      false,
		HumanSupported:          false,
		Limitations: []string{
			"readiness record only; owner-pass never upgrades independently_reproduced, community_validated, or human_supported",
			"private journal, packet, case, mapping, and path identities stay omitted",
			"not a provider-tested or held-out validity cell",
		},
	}
	digest, err := protocol.Digest(unsignedOwnerInspectionRecord(record))
	if err != nil {
		return OwnerInspectionRegistryRecord{}, err
	}
	record.Digest = digest
	return record, nil
}

func unsignedOwnerInspectionRecord(record OwnerInspectionRegistryRecord) OwnerInspectionRegistryRecord {
	record.Digest = ""
	return record
}
