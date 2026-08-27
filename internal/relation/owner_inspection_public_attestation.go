package relation

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

const (
	OwnerInspectionPublicAttestationSchemaVersion = "evalwitness.relation-owner-inspection-public-attestation.v1"
	OwnerInspectionPublicAttestationEvidenceKind  = "owner_authorized_agent_assisted_construct_inspection"
	OwnerInspectionPublicAttestationMode          = "private_chain_verified_public_aggregate"
	OwnerInspectionPublicValidationScope          = "schema_digest_denominators_statuses_and_claim_boundary"
	OwnerInspectionPublicSourceReproduction       = "requires_withheld_restricted_owner_inputs"
	OwnerInspectionPublicSignatureStatus          = "unsigned_content_addressed"
	OwnerInspectionPublicCapsuleStatus            = "not_yet_capsule_bound"
)

type OwnerInspectionPublicClaimStatus string

const (
	OwnerInspectionPublicClaimSupported               OwnerInspectionPublicClaimStatus = "supported"
	OwnerInspectionPublicClaimOwnerAttested           OwnerInspectionPublicClaimStatus = "owner_attested"
	OwnerInspectionPublicClaimNotRun                  OwnerInspectionPublicClaimStatus = "not_run"
	OwnerInspectionPublicClaimNotAuthorized           OwnerInspectionPublicClaimStatus = "not_authorized"
	OwnerInspectionPublicClaimUnsupported             OwnerInspectionPublicClaimStatus = "unsupported"
	OwnerInspectionPublicClaimNotPubliclyReproducible OwnerInspectionPublicClaimStatus = "not_publicly_reproducible"
)

type OwnerInspectionPublicAssessmentCounts struct {
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

type OwnerInspectionPublicDimensionCounts struct {
	Scope         string                   `json:"scope"`
	Dimension     PilotInspectionDimension `json:"dimension"`
	Applicable    int                      `json:"applicable"`
	Passed        int                      `json:"passed"`
	Failed        int                      `json:"failed"`
	Indeterminate int                      `json:"indeterminate"`
}

type OwnerInspectionPublicDispositionCounts struct {
	Accepted         int `json:"accepted"`
	RevisionRequired int `json:"revision_required"`
	Unresolved       int `json:"unresolved"`
}

type OwnerInspectionPublicOutcomes struct {
	Core             OwnerInspectionPublicDispositionCounts `json:"core"`
	ScarcityCases    OwnerInspectionPublicDispositionCounts `json:"scarcity_cases"`
	ScarcityBoundary PilotInspectionDisposition             `json:"scarcity_boundary"`
	CoreStatus       PilotInspectionOverallStatus           `json:"core_status"`
	ScarcityStatus   PilotInspectionOverallStatus           `json:"scarcity_status"`
	OverallStatus    PilotInspectionOverallStatus           `json:"overall_status"`
}

type OwnerInspectionPublicDisclosure struct {
	PrivateChainVerified              bool   `json:"private_chain_verified"`
	PrivateJournalIdentitiesDisclosed bool   `json:"private_journal_identities_disclosed"`
	RestrictedEvidenceDisclosed       bool   `json:"restricted_evidence_disclosed"`
	PublicValidationScope             string `json:"public_validation_scope"`
	SourceReproduction                string `json:"source_reproduction"`
	SignatureStatus                   string `json:"signature_status"`
	CapsuleStatus                     string `json:"capsule_status"`
}

type OwnerInspectionPublicClaim struct {
	ID       string                           `json:"id"`
	Claim    string                           `json:"claim"`
	Status   OwnerInspectionPublicClaimStatus `json:"status"`
	Evidence string                           `json:"evidence"`
}

type OwnerInspectionPublicAttestation struct {
	SchemaVersion          string                                 `json:"schema_version"`
	CanonicalPolicy        string                                 `json:"canonical_policy"`
	EvidenceKind           string                                 `json:"evidence_kind"`
	InspectionMode         string                                 `json:"inspection_mode"`
	InspectionDate         string                                 `json:"inspection_date"`
	PackageInventoryDigest string                                 `json:"package_inventory_digest"`
	Assessments            OwnerInspectionPublicAssessmentCounts  `json:"assessments"`
	Dimensions             []OwnerInspectionPublicDimensionCounts `json:"dimensions"`
	Outcomes               OwnerInspectionPublicOutcomes          `json:"outcomes"`
	HumanStudyStatus       string                                 `json:"human_study_status"`
	ExternalActionStatus   ExternalActionStatus                   `json:"external_action_status"`
	Disclosure             OwnerInspectionPublicDisclosure        `json:"disclosure"`
	Claims                 []OwnerInspectionPublicClaim           `json:"claims"`
	Digest                 string                                 `json:"digest"`
}

type OwnerInspectionPrivateChain struct {
	Completion        PilotInspectionCompletion
	Record            PilotInspectionRecord
	Session           PilotInspectionSession
	Events            []PilotInspectionEvent
	Readiness         RelationPilotReadiness
	Bundle            ReviewBundle
	Mappings          []PrivateMapping
	Plan              RelationPlanV3
	Primary           PrimarySampleV3
	Sentinel          ScarcitySentinelV3
	Pilot             PilotSampleV3
	ScarcityMaterials []CaseMaterial
	PackageBinding    PilotInspectionPackageBinding
}

func BuildOwnerInspectionPublicAttestation(chain OwnerInspectionPrivateChain) (OwnerInspectionPublicAttestation, error) {
	if err := verifyOwnerInspectionPrivateChain(chain); err != nil {
		return OwnerInspectionPublicAttestation{}, err
	}
	inspectedAt, _ := time.Parse(time.RFC3339, chain.Completion.InspectedAt)
	attestation := OwnerInspectionPublicAttestation{
		InspectionDate:         inspectedAt.UTC().Format(time.DateOnly),
		PackageInventoryDigest: chain.Completion.PackageInventoryDigest,
		Assessments: OwnerInspectionPublicAssessmentCounts{
			Required: PilotInspectionRequiredAssessments, JournalEvents: chain.Completion.EventCount,
			Corrections: chain.Completion.EventCount - chain.Completion.RequiredAssessments,
			Core:        PilotInspectionCoreAssessments, ScarcityCases: PilotInspectionScarcityAssessments,
			ScarcityBoundary: PilotInspectionBoundaryAssessments, Completed: chain.Completion.RequiredAssessments,
			CoreCases: len(chain.Record.Decisions), ScarcityCaseCount: len(chain.Completion.ScarcitySummaries),
			ScarcityTestCases: ownerInspectionScarcityRoleCount(chain.Completion.ScarcitySummaries, "test"),
		},
		Dimensions:           ownerInspectionPublicDimensions(chain.Record, chain.Completion),
		Outcomes:             ownerInspectionPublicOutcomes(chain.Record, chain.Completion),
		HumanStudyStatus:     chain.Completion.HumanStudyStatus,
		ExternalActionStatus: chain.Completion.ExternalActionStatus,
	}
	return SealOwnerInspectionPublicAttestation(attestation)
}

func VerifyOwnerInspectionPublicAttestation(attestation OwnerInspectionPublicAttestation, chain OwnerInspectionPrivateChain) error {
	if err := attestation.Validate(); err != nil {
		return err
	}
	expected, err := BuildOwnerInspectionPublicAttestation(chain)
	if err != nil {
		return err
	}
	if attestation.Digest != expected.Digest {
		return errors.New("owner-inspection public attestation differs from its verified private projection")
	}
	return nil
}

func SealOwnerInspectionPublicAttestation(attestation OwnerInspectionPublicAttestation) (OwnerInspectionPublicAttestation, error) {
	attestation.SchemaVersion = OwnerInspectionPublicAttestationSchemaVersion
	attestation.CanonicalPolicy = CanonicalPolicy
	attestation.EvidenceKind = OwnerInspectionPublicAttestationEvidenceKind
	attestation.InspectionMode = OwnerInspectionPublicAttestationMode
	attestation.Disclosure = expectedOwnerInspectionPublicDisclosure()
	attestation.Claims = expectedOwnerInspectionPublicClaims(attestation)
	attestation.Digest = ""
	digest, err := attestation.digest()
	if err != nil {
		return OwnerInspectionPublicAttestation{}, err
	}
	attestation.Digest = digest
	return attestation, attestation.Validate()
}

func (attestation OwnerInspectionPublicAttestation) Validate() error {
	if attestation.SchemaVersion != OwnerInspectionPublicAttestationSchemaVersion || attestation.CanonicalPolicy != CanonicalPolicy ||
		attestation.EvidenceKind != OwnerInspectionPublicAttestationEvidenceKind || attestation.InspectionMode != OwnerInspectionPublicAttestationMode ||
		!validDigest(attestation.PackageInventoryDigest) || attestation.HumanStudyStatus != PilotInspectionJournalHumanStudyStatus ||
		attestation.ExternalActionStatus != PilotInspectionJournalExternalAction {
		return errors.New("owner-inspection public attestation identity or claim boundary is invalid")
	}
	if parsed, err := time.Parse(time.DateOnly, attestation.InspectionDate); err != nil || parsed.UTC().Format(time.DateOnly) != attestation.InspectionDate {
		return errors.New("owner-inspection public attestation date is invalid")
	}
	if err := validateOwnerInspectionPublicAssessments(attestation.Assessments); err != nil {
		return err
	}
	if err := validateOwnerInspectionPublicDimensions(attestation.Dimensions); err != nil {
		return err
	}
	if err := validateOwnerInspectionPublicOutcomes(attestation.Outcomes, attestation.Assessments); err != nil {
		return err
	}
	if attestation.Disclosure != expectedOwnerInspectionPublicDisclosure() {
		return errors.New("owner-inspection public disclosure boundary is invalid")
	}
	if !slices.Equal(attestation.Claims, expectedOwnerInspectionPublicClaims(attestation)) {
		return errors.New("owner-inspection public claim boundary is invalid")
	}
	expected, err := attestation.digest()
	if err != nil || attestation.Digest != expected {
		return errors.New("owner-inspection public attestation digest is invalid")
	}
	return nil
}

func verifyOwnerInspectionPrivateChain(chain OwnerInspectionPrivateChain) error {
	if err := VerifyPilotInspectionSession(
		chain.Session, chain.Readiness, chain.Bundle, chain.Mappings, chain.Plan, chain.Primary,
		chain.Sentinel, chain.Pilot, chain.ScarcityMaterials, chain.PackageBinding,
	); err != nil {
		return err
	}
	if err := VerifyPilotInspectionCompletion(
		chain.Completion, chain.Session, chain.Events, chain.Record, chain.Readiness, chain.Bundle, chain.Mappings,
	); err != nil {
		return err
	}
	if chain.Completion.SessionDigest != chain.Session.Digest || chain.Completion.PackageInventoryDigest != chain.Session.Package.PackageInventoryDigest ||
		chain.Completion.InspectionRecordDigest != chain.Record.Digest || chain.Completion.InspectedAt != chain.Record.InspectedAt ||
		chain.Completion.CoreStatus != chain.Record.OverallStatus || chain.Completion.HumanStudyStatus != chain.Record.HumanStudyStatus ||
		chain.Completion.ExternalActionStatus != chain.Record.ExternalActionStatus || chain.Record.PlanDigest != chain.Plan.Digest ||
		chain.Record.PilotSampleDigest != chain.Pilot.Digest || chain.Session.ScarcitySentinelDigest != chain.Sentinel.Digest {
		return errors.New("owner-inspection public projection parents are cross-bound")
	}
	if len(chain.Completion.DecisionSummaries) != len(chain.Record.Decisions) || len(chain.Session.Packets) != len(chain.Record.Decisions) || len(chain.Session.ScarcityCases) != len(chain.Completion.ScarcitySummaries) {
		return errors.New("owner-inspection public projection parent denominators disagree")
	}
	for index, summary := range chain.Completion.DecisionSummaries {
		if summary.PacketID != chain.Record.Decisions[index].PacketID || summary.Disposition != chain.Record.Decisions[index].Disposition ||
			!slices.Equal(summary.ReasonCodes, chain.Record.Decisions[index].ReasonCodes) {
			return errors.New("owner-inspection public projection core summaries disagree")
		}
	}
	for index, summary := range chain.Completion.ScarcitySummaries {
		if summary.CaseID != chain.Sentinel.Cases[index].CaseID || summary.DataRole != string(chain.Sentinel.Cases[index].DataRole) {
			return errors.New("owner-inspection public projection scarcity summaries disagree with governance")
		}
	}
	return nil
}

func ownerInspectionPublicDimensions(record PilotInspectionRecord, completion PilotInspectionCompletion) []OwnerInspectionPublicDimensionCounts {
	rows := make([]OwnerInspectionPublicDimensionCounts, 0, len(PilotInspectionDimensions()))
	coreValues := []struct {
		dimension PilotInspectionDimension
		values    func(PilotInspectionDecision) PilotInspectionAssessment
	}{
		{PilotInspectionDimensionTaskContext, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.TaskContext }},
		{PilotInspectionDimensionEvidenceAlignment, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.EvidenceAlignment }},
		{PilotInspectionDimensionTransformationIsolation, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.TransformationIsolation }},
		{PilotInspectionDimensionInformationSufficiency, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.InformationSufficiency }},
		{PilotInspectionDimensionBlindingIntegrity, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.BlindingIntegrity }},
		{PilotInspectionDimensionRubricApplicability, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.RubricApplicability }},
		{PilotInspectionDimensionRedistributionBoundary, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.RedistributionBoundary }},
		{PilotInspectionDimensionCandidateOrder, func(value PilotInspectionDecision) PilotInspectionAssessment { return value.CandidateOrder }},
	}
	for _, item := range coreValues {
		assessments := make([]PilotInspectionAssessment, 0, len(record.Decisions))
		for _, decision := range record.Decisions {
			assessment := item.values(decision)
			if assessment != PilotInspectionNotApplicable {
				assessments = append(assessments, assessment)
			}
		}
		rows = append(rows, ownerInspectionPublicDimension("core", item.dimension, assessments))
	}
	scarcityValues := []struct {
		dimension PilotInspectionDimension
		values    func(PilotInspectionScarcitySummary) PilotInspectionAssessment
	}{
		{PilotInspectionDimensionScarcityOriginalEvidence, func(value PilotInspectionScarcitySummary) PilotInspectionAssessment { return value.OriginalEvidence }},
		{PilotInspectionDimensionScarcityTargetOmission, func(value PilotInspectionScarcitySummary) PilotInspectionAssessment { return value.TargetOmission }},
		{PilotInspectionDimensionScarcityRelationPreservation, func(value PilotInspectionScarcitySummary) PilotInspectionAssessment {
			return value.RelationPreservation
		}},
		{PilotInspectionDimensionScarcityInformationSufficiency, func(value PilotInspectionScarcitySummary) PilotInspectionAssessment {
			return value.InformationSufficiency
		}},
	}
	for _, item := range scarcityValues {
		assessments := make([]PilotInspectionAssessment, len(completion.ScarcitySummaries))
		for index, summary := range completion.ScarcitySummaries {
			assessments[index] = item.values(summary)
		}
		rows = append(rows, ownerInspectionPublicDimension("scarcity_case", item.dimension, assessments))
	}
	boundary := completion.ScarcityBoundary
	rows = append(rows,
		ownerInspectionPublicDimension("scarcity_boundary", PilotInspectionDimensionScarcityExhaustiveScope, []PilotInspectionAssessment{boundary.ExhaustiveScope}),
		ownerInspectionPublicDimension("scarcity_boundary", PilotInspectionDimensionScarcityRoleIntegrity, []PilotInspectionAssessment{boundary.RoleIntegrity}),
		ownerInspectionPublicDimension("scarcity_boundary", PilotInspectionDimensionScarcityEstimandSeparation, []PilotInspectionAssessment{boundary.EstimandSeparation}),
		ownerInspectionPublicDimension("scarcity_boundary", PilotInspectionDimensionScarcityNonAuthorization, []PilotInspectionAssessment{boundary.NonAuthorization}),
	)
	return rows
}

func ownerInspectionPublicDimension(scope string, dimension PilotInspectionDimension, assessments []PilotInspectionAssessment) OwnerInspectionPublicDimensionCounts {
	row := OwnerInspectionPublicDimensionCounts{Scope: scope, Dimension: dimension, Applicable: len(assessments)}
	for _, assessment := range assessments {
		switch assessment {
		case PilotInspectionPassed:
			row.Passed++
		case PilotInspectionFailed:
			row.Failed++
		case PilotInspectionIndeterminate:
			row.Indeterminate++
		}
	}
	return row
}

func ownerInspectionPublicOutcomes(record PilotInspectionRecord, completion PilotInspectionCompletion) OwnerInspectionPublicOutcomes {
	return OwnerInspectionPublicOutcomes{
		Core: OwnerInspectionPublicDispositionCounts{
			Accepted: record.Accepted, RevisionRequired: record.RevisionRequired, Unresolved: record.Unresolved,
		},
		ScarcityCases:    ownerInspectionPublicScarcityDispositionCounts(completion.ScarcitySummaries),
		ScarcityBoundary: completion.ScarcityBoundary.Disposition,
		CoreStatus:       completion.CoreStatus,
		ScarcityStatus:   completion.ScarcityStatus,
		OverallStatus:    completion.OverallStatus,
	}
}

func ownerInspectionPublicScarcityDispositionCounts(summaries []PilotInspectionScarcitySummary) OwnerInspectionPublicDispositionCounts {
	counts := OwnerInspectionPublicDispositionCounts{}
	for _, summary := range summaries {
		switch summary.Disposition {
		case PilotInspectionAccepted:
			counts.Accepted++
		case PilotInspectionRevisionRequired:
			counts.RevisionRequired++
		case PilotInspectionUnresolved:
			counts.Unresolved++
		}
	}
	return counts
}

func ownerInspectionScarcityRoleCount(summaries []PilotInspectionScarcitySummary, role string) int {
	count := 0
	for _, summary := range summaries {
		if summary.DataRole == role {
			count++
		}
	}
	return count
}

func validateOwnerInspectionPublicAssessments(value OwnerInspectionPublicAssessmentCounts) error {
	if value.Required != PilotInspectionRequiredAssessments || value.JournalEvents < value.Required || value.Corrections != value.JournalEvents-value.Required ||
		value.Core != PilotInspectionCoreAssessments || value.ScarcityCases != PilotInspectionScarcityAssessments ||
		value.ScarcityBoundary != PilotInspectionBoundaryAssessments || value.Completed != value.Required ||
		value.Core+value.ScarcityCases+value.ScarcityBoundary != value.Required || value.CoreCases != 7 ||
		value.ScarcityCaseCount != 3 || value.ScarcityTestCases != 0 {
		return errors.New("owner-inspection public assessment denominators are invalid")
	}
	return nil
}

func validateOwnerInspectionPublicDimensions(values []OwnerInspectionPublicDimensionCounts) error {
	expected := append([]PilotInspectionDimension{}, PilotInspectionCoreDimensions()...)
	expected = append(expected, PilotInspectionScarcityCaseDimensions()...)
	expected = append(expected, PilotInspectionScarcityBoundaryDimensions()...)
	if len(values) != len(expected) {
		return errors.New("owner-inspection public dimension coverage is incomplete")
	}
	totals := map[string]int{"core": 0, "scarcity_case": 0, "scarcity_boundary": 0}
	for index, row := range values {
		scope := "core"
		applicable := 7
		if index == len(PilotInspectionCoreDimensions())-1 {
			applicable = 1
		}
		if index >= len(PilotInspectionCoreDimensions()) && index < len(PilotInspectionCoreDimensions())+len(PilotInspectionScarcityCaseDimensions()) {
			scope, applicable = "scarcity_case", 3
		} else if index >= len(PilotInspectionCoreDimensions())+len(PilotInspectionScarcityCaseDimensions()) {
			scope, applicable = "scarcity_boundary", 1
		}
		if row.Scope != scope || row.Dimension != expected[index] || row.Applicable != applicable || row.Passed < 0 || row.Failed < 0 || row.Indeterminate < 0 ||
			row.Passed+row.Failed+row.Indeterminate != row.Applicable {
			return fmt.Errorf("owner-inspection public dimension %d is invalid", index)
		}
		totals[scope] += row.Applicable
	}
	if totals["core"] != PilotInspectionCoreAssessments || totals["scarcity_case"] != PilotInspectionScarcityAssessments || totals["scarcity_boundary"] != PilotInspectionBoundaryAssessments {
		return errors.New("owner-inspection public dimensions do not reproduce the assessment denominators")
	}
	return nil
}

func validateOwnerInspectionPublicOutcomes(value OwnerInspectionPublicOutcomes, assessments OwnerInspectionPublicAssessmentCounts) error {
	validCounts := func(counts OwnerInspectionPublicDispositionCounts, cases int) bool {
		return counts.Accepted >= 0 && counts.RevisionRequired >= 0 && counts.Unresolved >= 0 && counts.Accepted+counts.RevisionRequired+counts.Unresolved == cases
	}
	if !validCounts(value.Core, assessments.CoreCases) || !validCounts(value.ScarcityCases, assessments.ScarcityCaseCount) ||
		!slices.Contains([]PilotInspectionDisposition{PilotInspectionAccepted, PilotInspectionRevisionRequired, PilotInspectionUnresolved}, value.ScarcityBoundary) ||
		value.CoreStatus != pilotInspectionOverallStatus(value.Core.RevisionRequired, value.Core.Unresolved) {
		return errors.New("owner-inspection public outcome counts or core status are invalid")
	}
	scarcityRevision := value.ScarcityCases.RevisionRequired
	scarcityUnresolved := value.ScarcityCases.Unresolved
	switch value.ScarcityBoundary {
	case PilotInspectionRevisionRequired:
		scarcityRevision++
	case PilotInspectionUnresolved:
		scarcityUnresolved++
	}
	if value.ScarcityStatus != pilotInspectionOverallStatus(scarcityRevision, scarcityUnresolved) || value.OverallStatus != combinePilotInspectionStatuses(value.CoreStatus, value.ScarcityStatus) {
		return errors.New("owner-inspection public scarcity or combined status is invalid")
	}
	return nil
}

func expectedOwnerInspectionPublicDisclosure() OwnerInspectionPublicDisclosure {
	return OwnerInspectionPublicDisclosure{
		PrivateChainVerified: true, PrivateJournalIdentitiesDisclosed: false, RestrictedEvidenceDisclosed: false,
		PublicValidationScope: OwnerInspectionPublicValidationScope, SourceReproduction: OwnerInspectionPublicSourceReproduction,
		SignatureStatus: OwnerInspectionPublicSignatureStatus, CapsuleStatus: OwnerInspectionPublicCapsuleStatus,
	}
}

func expectedOwnerInspectionPublicClaims(attestation OwnerInspectionPublicAttestation) []OwnerInspectionPublicClaim {
	return []OwnerInspectionPublicClaim{
		{ID: "public_document_integrity", Claim: "Public schema, digest, denominators, aggregate statuses, and claim boundary", Status: OwnerInspectionPublicClaimSupported, Evidence: OwnerInspectionPublicValidationScope},
		{ID: "private_owner_inspection", Claim: "Owner-authorized agent-assisted inspection completed over all 66 required assessments", Status: OwnerInspectionPublicClaimOwnerAttested, Evidence: fmt.Sprintf("private chain verified during projection; %d journal events and %d corrections", attestation.Assessments.JournalEvents, attestation.Assessments.Corrections)},
		{ID: "core_construct_status", Claim: "Seven core pilot constructs", Status: OwnerInspectionPublicClaimOwnerAttested, Evidence: fmt.Sprintf("accepted=%d revision_required=%d unresolved=%d status=%s", attestation.Outcomes.Core.Accepted, attestation.Outcomes.Core.RevisionRequired, attestation.Outcomes.Core.Unresolved, attestation.Outcomes.CoreStatus)},
		{ID: "scarcity_construct_status", Claim: "Three separately reported scarcity constructs", Status: OwnerInspectionPublicClaimOwnerAttested, Evidence: fmt.Sprintf("accepted=%d revision_required=%d unresolved=%d status=%s", attestation.Outcomes.ScarcityCases.Accepted, attestation.Outcomes.ScarcityCases.RevisionRequired, attestation.Outcomes.ScarcityCases.Unresolved, attestation.Outcomes.ScarcityStatus)},
		{ID: "formal_human_study", Claim: "Independent formal human study or inter-rater agreement", Status: OwnerInspectionPublicClaimNotRun, Evidence: "not run"},
		{ID: "provider_or_verifier_performance", Claim: "Provider or verifier performance", Status: OwnerInspectionPublicClaimNotRun, Evidence: "not run; no provider evidence enters this attestation"},
		{ID: "external_action", Claim: "Reviewer contact, packet sharing, recruitment, publication authority, or study launch", Status: OwnerInspectionPublicClaimNotAuthorized, Evidence: "not authorized"},
		{ID: "population_or_held_out_validity", Claim: "Population generalization or held-out omitted-evidence validity", Status: OwnerInspectionPublicClaimUnsupported, Evidence: "unsupported; the scarcity surface has zero test-role cases"},
		{ID: "public_source_reproduction", Claim: "Public reproduction of the private assessment chain", Status: OwnerInspectionPublicClaimNotPubliclyReproducible, Evidence: OwnerInspectionPublicSourceReproduction},
		{ID: "corrected_corpus_feasibility", Claim: "Prospective corrected corpus or v4 feasibility", Status: OwnerInspectionPublicClaimUnsupported, Evidence: "unsupported; owned by the prospective trace-native source audit"},
	}
}

func (attestation OwnerInspectionPublicAttestation) digest() (string, error) {
	attestation.Digest = ""
	return digestJSON(attestation)
}
