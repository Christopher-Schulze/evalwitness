package relation

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	PilotLaunchDossierSchemaVersionV1 = "evalwitness.relation-pilot-launch-dossier.v1"
	PilotLaunchDossierSchemaVersionV2 = "evalwitness.relation-pilot-launch-dossier.v2"
	PilotLaunchDossierSchemaVersionV3 = "evalwitness.relation-pilot-launch-dossier.v3"
	PilotLaunchDossierSchemaVersion   = PilotLaunchDossierSchemaVersionV1
	PilotLaunchStatus                 = "not_launchable_pending_owner_inspection_and_authorization"
	PilotLaunchOwnerInspection        = "not_completed"
	PilotLaunchHumanStudy             = "not_run"
	PilotLaunchDecisionRequired       = "owner_decision_required"
)

type PilotReviewerWorkload struct {
	RequiredReviewerSlots          int `json:"required_reviewer_slots"`
	PrimaryReviewerSlots           int `json:"primary_reviewer_slots"`
	TieBreakReviewerSlots          int `json:"tie_break_reviewer_slots"`
	QualificationCasesPerReviewer  int `json:"qualification_cases_per_reviewer"`
	RequiredQualificationResponses int `json:"required_qualification_responses"`
	RequiredPrimaryJudgments       int `json:"required_primary_judgments"`
	MaximumTieBreakJudgments       int `json:"maximum_tie_break_judgments"`
	RequiredPostLabelProbes        int `json:"required_post_label_probes"`
	MaximumTotalReviewActions      int `json:"maximum_total_review_actions"`
}

type PilotPacketDisclosure struct {
	PacketID              string          `json:"packet_id"`
	PacketDigest          string          `json:"packet_digest"`
	Family                mutation.Family `json:"family"`
	Unit                  UnitType        `json:"unit"`
	EvidenceSlots         int             `json:"evidence_slots"`
	TaskRequirementDigest string          `json:"task_requirement_digest"`
	PrivacyClass          string          `json:"privacy_class"`
	PublicReleasable      bool            `json:"public_releasable"`
	RedistributionStatus  string          `json:"redistribution_status"`
	LeakageScanStatus     string          `json:"leakage_scan_status"`
	StructuralStatus      string          `json:"structural_status"`
}

type PilotGovernanceDecision struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	RequiredBefore string `json:"required_before"`
	Rule           string `json:"rule"`
}

type PilotExternalAction struct {
	Action string               `json:"action"`
	Status ExternalActionStatus `json:"status"`
}

type PilotLaunchDossier struct {
	SchemaVersion                     string                    `json:"schema_version"`
	CanonicalPolicy                   string                    `json:"canonical_policy"`
	ProtocolVersion                   string                    `json:"protocol_version"`
	Objective                         ReviewObjective           `json:"review_objective"`
	PlanDigest                        string                    `json:"plan_digest"`
	SourceCorpusDigest                string                    `json:"source_corpus_digest,omitempty"`
	SourceCorpusSpecDigest            string                    `json:"source_corpus_spec_digest,omitempty"`
	SourceCorpusPlanDigest            string                    `json:"source_corpus_plan_digest,omitempty"`
	SourceMutationProgramDigest       string                    `json:"source_mutation_program_digest,omitempty"`
	SourceConstructAuditDigest        string                    `json:"source_construct_audit_digest,omitempty"`
	ConstructFirewallCommitmentDigest string                    `json:"construct_firewall_commitment_digest,omitempty"`
	PilotSampleDigest                 string                    `json:"pilot_sample_digest"`
	BundleDigest                      string                    `json:"bundle_digest"`
	ReadinessDigest                   string                    `json:"readiness_digest"`
	QualificationSetDigest            string                    `json:"qualification_set_digest"`
	HandbookDigest                    string                    `json:"handbook_digest"`
	MappingCommitmentDigest           string                    `json:"mapping_commitment_digest"`
	DataRole                          ReviewDataRole            `json:"data_role"`
	Visibility                        ReviewVisibility          `json:"visibility"`
	ReviewerWorkload                  PilotReviewerWorkload     `json:"reviewer_workload"`
	PacketDisclosures                 []PilotPacketDisclosure   `json:"packet_disclosures"`
	GovernanceDecisions               []PilotGovernanceDecision `json:"governance_decisions"`
	ExternalActions                   []PilotExternalAction     `json:"external_actions"`
	LaunchStatus                      string                    `json:"launch_status"`
	OwnerInspectionStatus             string                    `json:"owner_inspection_status"`
	HumanStudyStatus                  string                    `json:"human_study_status"`
	ExternalActionStatus              ExternalActionStatus      `json:"external_action_status"`
	PreparedAt                        string                    `json:"prepared_at"`
	Limitations                       []string                  `json:"limitations"`
	Digest                            string                    `json:"digest"`
}

func BuildPilotLaunchDossier(plan Plan, pilot PilotSample, bundle ReviewBundle, mappings []PrivateMapping, qualification QualificationSet, handbook ReviewerHandbook, readiness RelationPilotReadiness, preparedAt string) (PilotLaunchDossier, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return PilotLaunchDossier{}, err
	}
	if err := pilot.Validate(); err != nil {
		return PilotLaunchDossier{}, err
	}
	if err := qualification.Validate(); err != nil {
		return PilotLaunchDossier{}, err
	}
	if err := VerifyReviewerHandbook(handbook, plan, qualification); err != nil {
		return PilotLaunchDossier{}, err
	}
	if err := VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return PilotLaunchDossier{}, err
	}
	if pilot.Digest != readiness.PilotSampleDigest || plan.Digest != readiness.PlanDigest || qualification.Digest != readiness.QualificationSetDigest ||
		handbook.Digest != readiness.HandbookDigest || bundle.Digest != readiness.BundleDigest {
		return PilotLaunchDossier{}, errors.New("relation pilot launch dossier inputs do not share one governed readiness state")
	}
	if plan.ProtocolVersion == ProtocolVersionV2 && (readiness.SourceCorpusDigest != plan.SourceCorpusDigest ||
		readiness.SourceCorpusSpecDigest != plan.SourceCorpusSpecDigest || readiness.SourceMutationProgramDigest != plan.SourceMutationProgramDigest ||
		readiness.SourceConstructAuditDigest != plan.SourceConstructAuditDigest) {
		return PilotLaunchDossier{}, errors.New("v2 relation pilot launch dossier source governance differs from its plan")
	} else if plan.ProtocolVersion == ProtocolVersionV3 && (readiness.SourceCorpusDigest != plan.SourceCorpusDigest ||
		readiness.SourceCorpusPlanDigest != plan.SourceCorpusPlanDigest || readiness.SourceMutationProgramDigest != plan.SourceMutationProgramDigest ||
		readiness.SourceConstructAuditDigest != plan.SourceConstructAuditDigest) {
		return PilotLaunchDossier{}, errors.New("v3 relation pilot launch dossier source governance differs from its plan")
	}
	if err := requireRelationTimeAfter("relation pilot launch dossier", preparedAt, readiness.PreparedAt); err != nil {
		return PilotLaunchDossier{}, err
	}
	packetByID := make(map[string]BlindPacket, len(bundle.Packets))
	for _, packet := range bundle.Packets {
		packetByID[packet.PacketID] = packet
	}
	disclosures := make([]PilotPacketDisclosure, len(readiness.PacketChecks))
	for index, check := range readiness.PacketChecks {
		packet, exists := packetByID[check.PacketID]
		if !exists {
			return PilotLaunchDossier{}, errors.New("relation pilot launch dossier readiness references an absent packet")
		}
		disclosures[index] = PilotPacketDisclosure{
			PacketID: check.PacketID, PacketDigest: check.PacketDigest, Family: check.Family, Unit: check.Unit,
			EvidenceSlots: check.EvidenceSlots, TaskRequirementDigest: check.TaskRequirementDigest, PrivacyClass: packet.PrivacyClass,
			PublicReleasable: packet.PublicReleasable, RedistributionStatus: check.RedistributionStatus,
			LeakageScanStatus: check.LeakageScanStatus, StructuralStatus: check.StructuralStatus,
		}
	}
	workload := PilotReviewerWorkload{
		RequiredReviewerSlots: plan.PrimaryReviewers + plan.TieBreakReviewers, PrimaryReviewerSlots: plan.PrimaryReviewers,
		TieBreakReviewerSlots: plan.TieBreakReviewers, QualificationCasesPerReviewer: len(qualification.Cases),
		RequiredQualificationResponses: len(qualification.Cases) * (plan.PrimaryReviewers + plan.TieBreakReviewers),
		RequiredPrimaryJudgments:       pilot.RequiredPrimaryLabels, MaximumTieBreakJudgments: pilot.MaximumTieBreakLabels,
		RequiredPostLabelProbes:   pilot.RequiredPostLabelProbes,
		MaximumTotalReviewActions: len(qualification.Cases)*(plan.PrimaryReviewers+plan.TieBreakReviewers) + pilot.RequiredPrimaryLabels + pilot.MaximumTieBreakLabels + pilot.RequiredPostLabelProbes,
	}
	dossier := PilotLaunchDossier{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: plan.Digest,
		PilotSampleDigest: pilot.Digest, BundleDigest: bundle.Digest, ReadinessDigest: readiness.Digest,
		QualificationSetDigest: qualification.Digest, HandbookDigest: handbook.Digest,
		MappingCommitmentDigest: readiness.MappingCommitmentDigest, DataRole: readiness.DataRole, Visibility: readiness.Visibility,
		ReviewerWorkload: workload, PacketDisclosures: disclosures, GovernanceDecisions: pilotGovernanceDecisions(),
		ExternalActions: pilotExternalActions(), LaunchStatus: PilotLaunchStatus, OwnerInspectionStatus: PilotLaunchOwnerInspection,
		HumanStudyStatus: PilotLaunchHumanStudy, ExternalActionStatus: ExternalActionNotAuthorized,
		PreparedAt: preparedAt, Limitations: pilotLaunchDossierLimitations(),
	}
	switch plan.ProtocolVersion {
	case ProtocolVersionV2:
		dossier.SourceCorpusDigest = readiness.SourceCorpusDigest
		dossier.SourceCorpusSpecDigest = readiness.SourceCorpusSpecDigest
		dossier.SourceMutationProgramDigest = readiness.SourceMutationProgramDigest
		dossier.SourceConstructAuditDigest = readiness.SourceConstructAuditDigest
		dossier.ConstructFirewallCommitmentDigest = readiness.ConstructFirewallCommitmentDigest
	case ProtocolVersionV3:
		dossier.SourceCorpusDigest = readiness.SourceCorpusDigest
		dossier.SourceCorpusPlanDigest = readiness.SourceCorpusPlanDigest
		dossier.SourceMutationProgramDigest = readiness.SourceMutationProgramDigest
		dossier.SourceConstructAuditDigest = readiness.SourceConstructAuditDigest
		dossier.ConstructFirewallCommitmentDigest = readiness.ConstructFirewallCommitmentDigest
	}
	return SealPilotLaunchDossier(dossier)
}

func SealPilotLaunchDossier(dossier PilotLaunchDossier) (PilotLaunchDossier, error) {
	schemaVersion, err := schemaVersionForProtocol(dossier.ProtocolVersion, PilotLaunchDossierSchemaVersionV1, PilotLaunchDossierSchemaVersionV2, PilotLaunchDossierSchemaVersionV3)
	if err != nil {
		return PilotLaunchDossier{}, err
	}
	dossier.SchemaVersion, dossier.CanonicalPolicy, dossier.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := pilotLaunchDossierDigest(dossier)
	if err != nil {
		return PilotLaunchDossier{}, err
	}
	dossier.Digest = digest
	return dossier, dossier.Validate()
}

func (dossier PilotLaunchDossier) Validate() error {
	if !validVersionedIdentity(dossier.SchemaVersion, dossier.ProtocolVersion, PilotLaunchDossierSchemaVersionV1, PilotLaunchDossierSchemaVersionV2, PilotLaunchDossierSchemaVersionV3) || dossier.CanonicalPolicy != CanonicalPolicy ||
		dossier.Objective != ReviewObjectiveControlledRelation || !validDigest(dossier.PlanDigest) || !validDigest(dossier.PilotSampleDigest) ||
		!validDigest(dossier.BundleDigest) || !validDigest(dossier.ReadinessDigest) || !validDigest(dossier.QualificationSetDigest) ||
		!validDigest(dossier.HandbookDigest) || !validDigest(dossier.MappingCommitmentDigest) || dossier.DataRole != ReviewDataDevelopmentPilot ||
		dossier.Visibility != ReviewVisibilityRestricted || dossier.LaunchStatus != PilotLaunchStatus || dossier.OwnerInspectionStatus != PilotLaunchOwnerInspection ||
		dossier.HumanStudyStatus != PilotLaunchHumanStudy || dossier.ExternalActionStatus != ExternalActionNotAuthorized ||
		!slices.Equal(dossier.GovernanceDecisions, pilotGovernanceDecisions()) || !slices.Equal(dossier.ExternalActions, pilotExternalActions()) ||
		!slices.Equal(dossier.Limitations, pilotLaunchDossierLimitations()) {
		return errors.New("relation pilot launch dossier identity, custody, or claim boundary is invalid")
	}
	if dossier.ProtocolVersion == ProtocolVersionV1 {
		if dossier.SourceCorpusDigest != "" || dossier.SourceCorpusSpecDigest != "" || dossier.SourceCorpusPlanDigest != "" || dossier.SourceMutationProgramDigest != "" ||
			dossier.SourceConstructAuditDigest != "" || dossier.ConstructFirewallCommitmentDigest != "" {
			return errors.New("v1 relation pilot launch dossier contains v2-only source or construct bindings")
		}
	} else if dossier.ProtocolVersion == ProtocolVersionV2 && (!validDigest(dossier.SourceCorpusDigest) || !validDigest(dossier.SourceCorpusSpecDigest) || dossier.SourceCorpusPlanDigest != "" ||
		!validDigest(dossier.SourceMutationProgramDigest) || !validDigest(dossier.SourceConstructAuditDigest) ||
		!validDigest(dossier.ConstructFirewallCommitmentDigest)) {
		return errors.New("v2 relation pilot launch dossier lacks its corpus, program, audit, or construct-firewall commitment")
	} else if dossier.ProtocolVersion == ProtocolVersionV3 && (!validDigest(dossier.SourceCorpusDigest) || dossier.SourceCorpusSpecDigest != "" || !validDigest(dossier.SourceCorpusPlanDigest) ||
		!validDigest(dossier.SourceMutationProgramDigest) || !validDigest(dossier.SourceConstructAuditDigest) || !validDigest(dossier.ConstructFirewallCommitmentDigest)) {
		return errors.New("v3 relation pilot launch dossier lacks its corpus plan, program, audit, or typed construct-firewall commitment")
	}
	if _, err := time.Parse(time.RFC3339, dossier.PreparedAt); err != nil {
		return errors.New("relation pilot launch dossier time must be RFC3339")
	}
	expectedWorkload := PilotReviewerWorkload{
		RequiredReviewerSlots: 3, PrimaryReviewerSlots: 2, TieBreakReviewerSlots: 1, QualificationCasesPerReviewer: 8,
		RequiredQualificationResponses: 24, RequiredPrimaryJudgments: 16, MaximumTieBreakJudgments: 8,
		RequiredPostLabelProbes: 16, MaximumTotalReviewActions: 64,
	}
	expectedPackets := 8
	if dossier.ProtocolVersion == ProtocolVersionV3 {
		expectedPackets = 7
		expectedWorkload.RequiredPrimaryJudgments = 14
		expectedWorkload.MaximumTieBreakJudgments = 7
		expectedWorkload.RequiredPostLabelProbes = 14
		expectedWorkload.MaximumTotalReviewActions = 59
	}
	if dossier.ReviewerWorkload != expectedWorkload || len(dossier.PacketDisclosures) != expectedPackets {
		return errors.New("relation pilot launch dossier workload or packet disclosure count is invalid")
	}
	seenFamilies := make(map[mutation.Family]struct{}, len(dossier.PacketDisclosures))
	for index, disclosure := range dossier.PacketDisclosures {
		definition, exists := mutation.DefinitionFor(disclosure.Family)
		expectedUnit, expectedSlots := UnitTrajectoryPair, 2
		if exists && definition.PairLevel {
			expectedUnit, expectedSlots = UnitCandidatePairOrders, 4
		}
		if !validOpaqueID(disclosure.PacketID, "relation-packet-") || !validDigest(disclosure.PacketDigest) || !exists ||
			disclosure.Unit != expectedUnit || disclosure.EvidenceSlots != expectedSlots || !validDigest(disclosure.TaskRequirementDigest) ||
			strings.TrimSpace(disclosure.PrivacyClass) == "" || disclosure.PublicReleasable || disclosure.RedistributionStatus != "restricted_reference_only" ||
			disclosure.LeakageScanStatus != "passed" || disclosure.StructuralStatus != "ready_for_owner_semantic_review" ||
			index > 0 && dossier.PacketDisclosures[index-1].PacketID >= disclosure.PacketID {
			return fmt.Errorf("relation pilot launch dossier packet disclosure %d is invalid", index)
		}
		if _, duplicate := seenFamilies[disclosure.Family]; duplicate {
			return errors.New("relation pilot launch dossier reuses a family")
		}
		seenFamilies[disclosure.Family] = struct{}{}
	}
	observedFamilies := make([]mutation.Family, 0, len(seenFamilies))
	for family := range seenFamilies {
		observedFamilies = append(observedFamilies, family)
	}
	slices.Sort(observedFamilies)
	expectedFamilies := pilotLaunchFamilies()
	if dossier.ProtocolVersion == ProtocolVersionV3 {
		expectedFamilies = pilotLaunchFamiliesV3()
	}
	if !slices.Equal(observedFamilies, expectedFamilies) {
		return errors.New("relation pilot launch dossier does not cover the exact governed family set")
	}
	expected, err := pilotLaunchDossierDigest(dossier)
	if err != nil || dossier.Digest != expected {
		return errors.New("relation pilot launch dossier digest is invalid")
	}
	return nil
}

func pilotGovernanceDecisions() []PilotGovernanceDecision {
	return []PilotGovernanceDecision{
		{ID: "authorship_and_labor_credit", Status: PilotLaunchDecisionRequired, RequiredBefore: "reviewer_contact", Rule: "Approve authorship, acknowledgement, contribution credit, and labor-credit terms before contacting any reviewer."},
		{ID: "compensation_or_volunteer_terms", Status: PilotLaunchDecisionRequired, RequiredBefore: "reviewer_contact", Rule: "Approve compensation amount and method or an explicit volunteer arrangement before contacting any reviewer."},
		{ID: "consent_and_withdrawal_terms", Status: PilotLaunchDecisionRequired, RequiredBefore: "packet_sharing", Rule: "Approve informed-consent, withdrawal, and downstream aggregate-publication terms before sharing any packet."},
		{ID: "contact_and_scheduling_method", Status: PilotLaunchDecisionRequired, RequiredBefore: "reviewer_contact", Rule: "Approve the contact channel, scheduling process, and responsible owner before contacting any reviewer."},
		{ID: "human_data_retention_and_deletion", Status: PilotLaunchDecisionRequired, RequiredBefore: "packet_sharing", Rule: "Approve retention, access, deletion, and incident-response rules for consent and reviewer records before sharing any packet."},
		{ID: "reviewer_population_and_independence", Status: PilotLaunchDecisionRequired, RequiredBefore: "recruitment", Rule: "Approve reviewer eligibility, technical background, independence, conflict, and exclusion criteria before recruitment."},
	}
}

func pilotLaunchFamilies() []mutation.Family {
	return []mutation.Family{
		mutation.FamilyCandidateOrderReversal,
		mutation.FamilyCausalIndependentReorder,
		mutation.FamilyTestEvidenceFalsified,
		mutation.FamilyToolOutputIncomplete,
		mutation.FamilyIrrelevantVerbosity,
		mutation.FamilyNeutralFormatting,
		mutation.FamilyTestEvidenceOmitted,
		mutation.FamilyUntrustedScoreInjection,
	}
}

func pilotLaunchFamiliesV3() []mutation.Family {
	return []mutation.Family{
		mutation.FamilyCandidateOrderReversal,
		mutation.FamilyCausalIndependentReorder,
		mutation.FamilyTestEvidenceFalsified,
		mutation.FamilyToolOutputIncomplete,
		mutation.FamilyIrrelevantVerbosity,
		mutation.FamilyNeutralFormatting,
		mutation.FamilyUntrustedScoreInjection,
	}
}

func pilotExternalActions() []PilotExternalAction {
	return []PilotExternalAction{
		{Action: "assignment", Status: ExternalActionNotAuthorized},
		{Action: "compensation", Status: ExternalActionNotAuthorized},
		{Action: "contact", Status: ExternalActionNotAuthorized},
		{Action: "packet_sharing", Status: ExternalActionNotAuthorized},
		{Action: "publication", Status: ExternalActionNotAuthorized},
		{Action: "recruitment", Status: ExternalActionNotAuthorized},
		{Action: "scheduling", Status: ExternalActionNotAuthorized},
	}
}

func pilotLaunchDossierLimitations() []string {
	return []string{
		"This dossier is an owner decision surface, not consent, ethics approval, recruitment authority, or a human-study result.",
		"Workload counts are exact protocol maxima; reviewer duration and burden remain empirical and unmeasured.",
		"Packet disclosures contain structural metadata only and do not make restricted reference-only evidence public.",
		"A passing owner semantic inspection still requires separate explicit authorization for every external action.",
	}
}

func pilotLaunchDossierDigest(dossier PilotLaunchDossier) (string, error) {
	dossier.Digest = ""
	return digestJSON(dossier)
}
