package relation

import (
	"sort"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestPilotLaunchDossierRejectsWeakenedGovernance(t *testing.T) {
	t.Parallel()

	dossier := validPilotLaunchDossierForTest(t)
	tests := []struct {
		name   string
		mutate func(*PilotLaunchDossier)
	}{
		{name: "launch status", mutate: func(value *PilotLaunchDossier) { value.LaunchStatus = "launchable" }},
		{name: "external authorization", mutate: func(value *PilotLaunchDossier) { value.ExternalActions[0].Status = ExternalActionStatus("authorized") }},
		{name: "workload deletion", mutate: func(value *PilotLaunchDossier) { value.ReviewerWorkload.MaximumTotalReviewActions-- }},
		{name: "public packet", mutate: func(value *PilotLaunchDossier) { value.PacketDisclosures[0].PublicReleasable = true }},
		{name: "family substitution", mutate: func(value *PilotLaunchDossier) {
			value.PacketDisclosures[0].Family = mutation.FamilyAmbiguousSemanticEdit
			value.PacketDisclosures[0].Unit = UnitTrajectoryPair
			value.PacketDisclosures[0].EvidenceSlots = 2
		}},
		{name: "owner decision deletion", mutate: func(value *PilotLaunchDossier) { value.GovernanceDecisions = value.GovernanceDecisions[1:] }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := dossier
			changed.PacketDisclosures = append([]PilotPacketDisclosure(nil), dossier.PacketDisclosures...)
			changed.GovernanceDecisions = append([]PilotGovernanceDecision(nil), dossier.GovernanceDecisions...)
			changed.ExternalActions = append([]PilotExternalAction(nil), dossier.ExternalActions...)
			test.mutate(&changed)
			if _, err := SealPilotLaunchDossier(changed); err == nil {
				t.Fatal("SealPilotLaunchDossier() accepted weakened governance")
			}
		})
	}
}

func validPilotLaunchDossierForTest(t *testing.T) PilotLaunchDossier {
	t.Helper()

	families := pilotLaunchFamilies()
	disclosures := make([]PilotPacketDisclosure, len(families))
	for index, family := range families {
		definition, exists := mutation.DefinitionFor(family)
		if !exists {
			t.Fatalf("missing mutation definition for %s", family)
		}
		unit, slots := UnitTrajectoryPair, 2
		if definition.PairLevel {
			unit, slots = UnitCandidatePairOrders, 4
		}
		disclosures[index] = PilotPacketDisclosure{
			PacketID: "relation-packet-" + digestText("packet-id-"+string(family)), PacketDigest: digestText("packet-" + string(family)),
			Family: family, Unit: unit, EvidenceSlots: slots, TaskRequirementDigest: digestText("task-" + string(family)),
			PrivacyClass: "restricted", RedistributionStatus: "restricted_reference_only", LeakageScanStatus: "passed",
			StructuralStatus: "ready_for_owner_semantic_review",
		}
	}
	sort.Slice(disclosures, func(left, right int) bool { return disclosures[left].PacketID < disclosures[right].PacketID })
	dossier := PilotLaunchDossier{
		ProtocolVersion: ProtocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: digestText("plan"),
		PilotSampleDigest: digestText("pilot"), BundleDigest: digestText("bundle"), ReadinessDigest: digestText("readiness"),
		QualificationSetDigest: digestText("qualification"), HandbookDigest: digestText("handbook"), MappingCommitmentDigest: digestText("mappings"),
		DataRole: ReviewDataDevelopmentPilot, Visibility: ReviewVisibilityRestricted,
		ReviewerWorkload: PilotReviewerWorkload{
			RequiredReviewerSlots: 3, PrimaryReviewerSlots: 2, TieBreakReviewerSlots: 1, QualificationCasesPerReviewer: 8,
			RequiredQualificationResponses: 24, RequiredPrimaryJudgments: 16, MaximumTieBreakJudgments: 8,
			RequiredPostLabelProbes: 16, MaximumTotalReviewActions: 64,
		},
		PacketDisclosures: disclosures, GovernanceDecisions: pilotGovernanceDecisions(), ExternalActions: pilotExternalActions(),
		LaunchStatus: PilotLaunchStatus, OwnerInspectionStatus: PilotLaunchOwnerInspection, HumanStudyStatus: PilotLaunchHumanStudy,
		ExternalActionStatus: ExternalActionNotAuthorized, PreparedAt: "2026-08-09T19:02:00Z", Limitations: pilotLaunchDossierLimitations(),
	}
	sealed, err := SealPilotLaunchDossier(dossier)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}
