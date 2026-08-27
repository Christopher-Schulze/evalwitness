package explorer

import (
	"errors"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
)

type OwnerInspectionAssessmentView struct {
	Required          int `json:"required"`
	JournalEvents     int `json:"journal_events"`
	Corrections       int `json:"corrections"`
	Core              int `json:"core"`
	ScarcityCases     int `json:"scarcity_cases"`
	ScarcityBoundary  int `json:"scarcity_boundary"`
	Completed         int `json:"completed"`
	CoreCases         int `json:"core_cases"`
	ScarcityCaseCount int `json:"scarcity_case_count"`
	ScarcityTestCases int `json:"scarcity_test_cases"`
}

type OwnerInspectionDimensionView struct {
	Scope         string `json:"scope"`
	Dimension     string `json:"dimension"`
	Applicable    int    `json:"applicable"`
	Passed        int    `json:"passed"`
	Failed        int    `json:"failed"`
	Indeterminate int    `json:"indeterminate"`
}

type OwnerInspectionDispositionView struct {
	Accepted         int `json:"accepted"`
	RevisionRequired int `json:"revision_required"`
	Unresolved       int `json:"unresolved"`
}

type OwnerInspectionOutcomesView struct {
	Core             OwnerInspectionDispositionView `json:"core"`
	ScarcityCases    OwnerInspectionDispositionView `json:"scarcity_cases"`
	ScarcityBoundary string                         `json:"scarcity_boundary"`
	CoreStatus       string                         `json:"core_status"`
	ScarcityStatus   string                         `json:"scarcity_status"`
	OverallStatus    string                         `json:"overall_status"`
}

type OwnerInspectionDisclosureView struct {
	PrivateChainVerified              bool   `json:"private_chain_verified"`
	PrivateJournalIdentitiesDisclosed bool   `json:"private_journal_identities_disclosed"`
	RestrictedEvidenceDisclosed       bool   `json:"restricted_evidence_disclosed"`
	PublicValidationScope             string `json:"public_validation_scope"`
	SourceReproduction                string `json:"source_reproduction"`
	SignatureStatus                   string `json:"signature_status"`
	CapsuleStatus                     string `json:"capsule_status"`
}

type OwnerInspectionClaimView struct {
	ID       string `json:"id"`
	Claim    string `json:"claim"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

type OwnerInspectionView struct {
	Availability           Availability                   `json:"availability"`
	SchemaVersion          string                         `json:"schema_version"`
	CanonicalPolicy        string                         `json:"canonical_policy"`
	EvidenceKind           string                         `json:"evidence_kind"`
	InspectionMode         string                         `json:"inspection_mode"`
	InspectionDate         string                         `json:"inspection_date"`
	PackageInventoryDigest string                         `json:"package_inventory_digest"`
	Assessments            OwnerInspectionAssessmentView  `json:"assessments"`
	Dimensions             []OwnerInspectionDimensionView `json:"dimensions"`
	Outcomes               OwnerInspectionOutcomesView    `json:"outcomes"`
	HumanStudyStatus       string                         `json:"human_study_status"`
	ExternalActionStatus   string                         `json:"external_action_status"`
	Disclosure             OwnerInspectionDisclosureView  `json:"disclosure"`
	Claims                 []OwnerInspectionClaimView     `json:"claims"`
	Digest                 string                         `json:"digest"`
	Source                 ArtifactRef                    `json:"source"`
}

func buildOwnerInspectionView(
	attestation relation.OwnerInspectionPublicAttestation,
	source ArtifactRef,
) OwnerInspectionView {
	dimensions := make([]OwnerInspectionDimensionView, len(attestation.Dimensions))
	for index, dimension := range attestation.Dimensions {
		dimensions[index] = OwnerInspectionDimensionView{
			Scope: dimension.Scope, Dimension: string(dimension.Dimension), Applicable: dimension.Applicable,
			Passed: dimension.Passed, Failed: dimension.Failed, Indeterminate: dimension.Indeterminate,
		}
	}
	claims := make([]OwnerInspectionClaimView, len(attestation.Claims))
	for index, claim := range attestation.Claims {
		claims[index] = OwnerInspectionClaimView{
			ID: claim.ID, Claim: claim.Claim, Status: string(claim.Status), Evidence: claim.Evidence,
		}
	}
	return OwnerInspectionView{
		Availability: AvailabilityAvailable, SchemaVersion: attestation.SchemaVersion,
		CanonicalPolicy: attestation.CanonicalPolicy, EvidenceKind: attestation.EvidenceKind,
		InspectionMode: attestation.InspectionMode, InspectionDate: attestation.InspectionDate,
		PackageInventoryDigest: attestation.PackageInventoryDigest,
		Assessments: OwnerInspectionAssessmentView{
			Required: attestation.Assessments.Required, JournalEvents: attestation.Assessments.JournalEvents,
			Corrections: attestation.Assessments.Corrections, Core: attestation.Assessments.Core,
			ScarcityCases:    attestation.Assessments.ScarcityCases,
			ScarcityBoundary: attestation.Assessments.ScarcityBoundary, Completed: attestation.Assessments.Completed,
			CoreCases: attestation.Assessments.CoreCases, ScarcityCaseCount: attestation.Assessments.ScarcityCaseCount,
			ScarcityTestCases: attestation.Assessments.ScarcityTestCases,
		},
		Dimensions: dimensions,
		Outcomes: OwnerInspectionOutcomesView{
			Core:             ownerInspectionDispositionView(attestation.Outcomes.Core),
			ScarcityCases:    ownerInspectionDispositionView(attestation.Outcomes.ScarcityCases),
			ScarcityBoundary: string(attestation.Outcomes.ScarcityBoundary),
			CoreStatus:       string(attestation.Outcomes.CoreStatus), ScarcityStatus: string(attestation.Outcomes.ScarcityStatus),
			OverallStatus: string(attestation.Outcomes.OverallStatus),
		},
		HumanStudyStatus: attestation.HumanStudyStatus, ExternalActionStatus: string(attestation.ExternalActionStatus),
		Disclosure: OwnerInspectionDisclosureView{
			PrivateChainVerified:              attestation.Disclosure.PrivateChainVerified,
			PrivateJournalIdentitiesDisclosed: attestation.Disclosure.PrivateJournalIdentitiesDisclosed,
			RestrictedEvidenceDisclosed:       attestation.Disclosure.RestrictedEvidenceDisclosed,
			PublicValidationScope:             attestation.Disclosure.PublicValidationScope,
			SourceReproduction:                attestation.Disclosure.SourceReproduction,
			SignatureStatus:                   attestation.Disclosure.SignatureStatus, CapsuleStatus: attestation.Disclosure.CapsuleStatus,
		},
		Claims: claims, Digest: attestation.Digest, Source: source,
	}
}

func ownerInspectionDispositionView(value relation.OwnerInspectionPublicDispositionCounts) OwnerInspectionDispositionView {
	return OwnerInspectionDispositionView{
		Accepted: value.Accepted, RevisionRequired: value.RevisionRequired, Unresolved: value.Unresolved,
	}
}

func (report Report) validateOwnerInspection() error {
	view := report.OwnerInspection
	if view.Availability != AvailabilityAvailable || validateArtifactRef(view.Source) != nil ||
		view.Source.Kind != ArtifactCapsuleComponent ||
		view.Source.SchemaVersion != relation.OwnerInspectionPublicAttestationSchemaVersion ||
		view.Source.SchemaVersion != view.SchemaVersion || view.Source.ArtifactDigest != view.Digest {
		return errors.New("evidence explorer owner-inspection source is invalid")
	}
	attestation := relation.OwnerInspectionPublicAttestation{
		SchemaVersion: view.SchemaVersion, CanonicalPolicy: view.CanonicalPolicy, EvidenceKind: view.EvidenceKind,
		InspectionMode: view.InspectionMode, InspectionDate: view.InspectionDate,
		PackageInventoryDigest: view.PackageInventoryDigest,
		Assessments: relation.OwnerInspectionPublicAssessmentCounts{
			Required: view.Assessments.Required, JournalEvents: view.Assessments.JournalEvents,
			Corrections: view.Assessments.Corrections, Core: view.Assessments.Core,
			ScarcityCases: view.Assessments.ScarcityCases, ScarcityBoundary: view.Assessments.ScarcityBoundary,
			Completed: view.Assessments.Completed, CoreCases: view.Assessments.CoreCases,
			ScarcityCaseCount: view.Assessments.ScarcityCaseCount, ScarcityTestCases: view.Assessments.ScarcityTestCases,
		},
		Dimensions: ownerInspectionDomainDimensions(view.Dimensions),
		Outcomes: relation.OwnerInspectionPublicOutcomes{
			Core:             ownerInspectionDomainDisposition(view.Outcomes.Core),
			ScarcityCases:    ownerInspectionDomainDisposition(view.Outcomes.ScarcityCases),
			ScarcityBoundary: relation.PilotInspectionDisposition(view.Outcomes.ScarcityBoundary),
			CoreStatus:       relation.PilotInspectionOverallStatus(view.Outcomes.CoreStatus),
			ScarcityStatus:   relation.PilotInspectionOverallStatus(view.Outcomes.ScarcityStatus),
			OverallStatus:    relation.PilotInspectionOverallStatus(view.Outcomes.OverallStatus),
		},
		HumanStudyStatus: view.HumanStudyStatus, ExternalActionStatus: relation.ExternalActionStatus(view.ExternalActionStatus),
		Disclosure: relation.OwnerInspectionPublicDisclosure{
			PrivateChainVerified:              view.Disclosure.PrivateChainVerified,
			PrivateJournalIdentitiesDisclosed: view.Disclosure.PrivateJournalIdentitiesDisclosed,
			RestrictedEvidenceDisclosed:       view.Disclosure.RestrictedEvidenceDisclosed,
			PublicValidationScope:             view.Disclosure.PublicValidationScope,
			SourceReproduction:                view.Disclosure.SourceReproduction, SignatureStatus: view.Disclosure.SignatureStatus,
			CapsuleStatus: view.Disclosure.CapsuleStatus,
		},
		Claims: ownerInspectionDomainClaims(view.Claims), Digest: view.Digest,
	}
	if err := attestation.Validate(); err != nil {
		return errors.New("evidence explorer owner-inspection projection is invalid")
	}
	return nil
}

func ownerInspectionDomainDimensions(source []OwnerInspectionDimensionView) []relation.OwnerInspectionPublicDimensionCounts {
	dimensions := make([]relation.OwnerInspectionPublicDimensionCounts, len(source))
	for index, dimension := range source {
		dimensions[index] = relation.OwnerInspectionPublicDimensionCounts{
			Scope: dimension.Scope, Dimension: relation.PilotInspectionDimension(dimension.Dimension),
			Applicable: dimension.Applicable, Passed: dimension.Passed,
			Failed: dimension.Failed, Indeterminate: dimension.Indeterminate,
		}
	}
	return dimensions
}

func ownerInspectionDomainDisposition(value OwnerInspectionDispositionView) relation.OwnerInspectionPublicDispositionCounts {
	return relation.OwnerInspectionPublicDispositionCounts{
		Accepted: value.Accepted, RevisionRequired: value.RevisionRequired, Unresolved: value.Unresolved,
	}
}

func ownerInspectionDomainClaims(source []OwnerInspectionClaimView) []relation.OwnerInspectionPublicClaim {
	claims := make([]relation.OwnerInspectionPublicClaim, len(source))
	for index, claim := range source {
		claims[index] = relation.OwnerInspectionPublicClaim{
			ID: claim.ID, Claim: claim.Claim, Status: relation.OwnerInspectionPublicClaimStatus(claim.Status),
			Evidence: claim.Evidence,
		}
	}
	return claims
}
