package relation

import (
	"errors"
	"reflect"
)

const OwnerInspectionCustodyGateSchemaVersion = "evalwitness.relation-owner-inspection-custody-gate.v1"

type OwnerInspectionCustodyGate struct {
	SchemaVersion                     string                       `json:"schema_version"`
	CanonicalPolicy                   string                       `json:"canonical_policy"`
	PublicAttestationDigest           string                       `json:"public_attestation_digest"`
	PackageInventoryDigest            string                       `json:"package_inventory_digest"`
	SessionDigest                     string                       `json:"session_digest"`
	InspectionRecordDigest            string                       `json:"inspection_record_digest"`
	CompletionDigest                  string                       `json:"completion_digest"`
	RequiredAssessments               int                          `json:"required_assessments"`
	CompletedAssessments              int                          `json:"completed_assessments"`
	JournalEvents                     int                          `json:"journal_events"`
	Corrections                       int                          `json:"corrections"`
	Dimensions                        int                          `json:"dimensions"`
	OverallStatus                     PilotInspectionOverallStatus `json:"overall_status"`
	HumanStudyStatus                  string                       `json:"human_study_status"`
	ExternalActionStatus              ExternalActionStatus         `json:"external_action_status"`
	PrivateChainVerified              bool                         `json:"private_chain_verified"`
	ClaimBoundaryVerified             bool                         `json:"claim_boundary_verified"`
	FormalHumanLedgerPresent          bool                         `json:"formal_human_ledger_present"`
	PrimaryAdmissionAuthorized        bool                         `json:"primary_admission_authorized"`
	ExecutionAuthorized               bool                         `json:"execution_authorized"`
	ProviderCalls                     int                          `json:"provider_calls"`
	NetworkRequired                   bool                         `json:"network_required"`
	ContainsPrivateEvidenceIdentities bool                         `json:"contains_private_evidence_identities"`
	Digest                            string                       `json:"digest"`
}

func BuildOwnerInspectionCustodyGate(
	attestation OwnerInspectionPublicAttestation,
	chain OwnerInspectionPrivateChain,
) (OwnerInspectionCustodyGate, error) {
	value, err := buildOwnerInspectionCustodyGate(attestation, chain)
	if err != nil {
		return OwnerInspectionCustodyGate{}, err
	}
	value.Digest, err = ownerInspectionCustodyGateDigest(value)
	if err != nil {
		return OwnerInspectionCustodyGate{}, err
	}
	if err := value.Validate(attestation, chain); err != nil {
		return OwnerInspectionCustodyGate{}, err
	}
	return value, nil
}

func buildOwnerInspectionCustodyGate(
	attestation OwnerInspectionPublicAttestation,
	chain OwnerInspectionPrivateChain,
) (OwnerInspectionCustodyGate, error) {
	if err := validateOwnerInspectionCustodyParents(attestation, chain); err != nil {
		return OwnerInspectionCustodyGate{}, err
	}
	return OwnerInspectionCustodyGate{
		SchemaVersion: OwnerInspectionCustodyGateSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PublicAttestationDigest: attestation.Digest, PackageInventoryDigest: attestation.PackageInventoryDigest,
		SessionDigest: chain.Session.Digest, InspectionRecordDigest: chain.Record.Digest, CompletionDigest: chain.Completion.Digest,
		RequiredAssessments: attestation.Assessments.Required, CompletedAssessments: attestation.Assessments.Completed,
		JournalEvents: attestation.Assessments.JournalEvents, Corrections: attestation.Assessments.Corrections,
		Dimensions: len(attestation.Dimensions), OverallStatus: attestation.Outcomes.OverallStatus,
		HumanStudyStatus: attestation.HumanStudyStatus, ExternalActionStatus: attestation.ExternalActionStatus,
		PrivateChainVerified: true, ClaimBoundaryVerified: true,
		FormalHumanLedgerPresent: false, PrimaryAdmissionAuthorized: false, ExecutionAuthorized: false,
		ProviderCalls: 0, NetworkRequired: false, ContainsPrivateEvidenceIdentities: true,
	}, nil
}

func validateOwnerInspectionCustodyParents(
	attestation OwnerInspectionPublicAttestation,
	chain OwnerInspectionPrivateChain,
) error {
	if err := VerifyOwnerInspectionPublicAttestation(attestation, chain); err != nil {
		return err
	}
	if err := validatePassedOwnerInspection(attestation); err != nil {
		return err
	}
	if err := validateOwnerInspectionCustodyClaims(attestation.Claims); err != nil {
		return err
	}
	if attestation.PackageInventoryDigest != chain.Completion.PackageInventoryDigest ||
		attestation.PackageInventoryDigest != chain.Session.Package.PackageInventoryDigest {
		return errors.New("owner-inspection custody package identity is cross-bound")
	}
	return nil
}

func validatePassedOwnerInspection(attestation OwnerInspectionPublicAttestation) error {
	counts := attestation.Assessments
	if counts.Required != PilotInspectionRequiredAssessments || counts.Completed != PilotInspectionRequiredAssessments ||
		len(attestation.Dimensions) != len(PilotInspectionDimensions()) ||
		attestation.Outcomes.Core != (OwnerInspectionPublicDispositionCounts{Accepted: counts.CoreCases}) ||
		attestation.Outcomes.ScarcityCases != (OwnerInspectionPublicDispositionCounts{Accepted: counts.ScarcityCaseCount}) ||
		attestation.Outcomes.ScarcityBoundary != PilotInspectionAccepted ||
		attestation.Outcomes.CoreStatus != PilotInspectionOverallPassed ||
		attestation.Outcomes.ScarcityStatus != PilotInspectionOverallPassed ||
		attestation.Outcomes.OverallStatus != PilotInspectionOverallPassed {
		return errors.New("owner-inspection custody requires one fully passed 66-assessment projection")
	}
	for _, dimension := range attestation.Dimensions {
		if dimension.Passed != dimension.Applicable || dimension.Failed != 0 || dimension.Indeterminate != 0 {
			return errors.New("owner-inspection custody contains a failed or unresolved dimension")
		}
	}
	return nil
}

func validateOwnerInspectionCustodyClaims(claims []OwnerInspectionPublicClaim) error {
	expected := map[string]OwnerInspectionPublicClaimStatus{
		"public_document_integrity":        OwnerInspectionPublicClaimSupported,
		"private_owner_inspection":         OwnerInspectionPublicClaimOwnerAttested,
		"core_construct_status":            OwnerInspectionPublicClaimOwnerAttested,
		"scarcity_construct_status":        OwnerInspectionPublicClaimOwnerAttested,
		"formal_human_study":               OwnerInspectionPublicClaimNotRun,
		"provider_or_verifier_performance": OwnerInspectionPublicClaimNotRun,
		"external_action":                  OwnerInspectionPublicClaimNotAuthorized,
		"population_or_held_out_validity":  OwnerInspectionPublicClaimUnsupported,
		"public_source_reproduction":       OwnerInspectionPublicClaimNotPubliclyReproducible,
		"corrected_corpus_feasibility":     OwnerInspectionPublicClaimUnsupported,
	}
	if len(claims) != len(expected) {
		return errors.New("owner-inspection custody claim inventory is incomplete")
	}
	seen := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		status, found := expected[claim.ID]
		_, duplicated := seen[claim.ID]
		if !found || duplicated || claim.Status != status {
			return errors.New("owner-inspection custody claim boundary was promoted or changed")
		}
		seen[claim.ID] = struct{}{}
	}
	return nil
}

func (value OwnerInspectionCustodyGate) Validate(
	attestation OwnerInspectionPublicAttestation,
	chain OwnerInspectionPrivateChain,
) error {
	expected, err := buildOwnerInspectionCustodyGate(attestation, chain)
	if err != nil {
		return err
	}
	expected.Digest, err = ownerInspectionCustodyGateDigest(expected)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("owner-inspection custody gate does not reproduce from its private and public parents")
	}
	return nil
}

func ownerInspectionCustodyGateDigest(value OwnerInspectionCustodyGate) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
