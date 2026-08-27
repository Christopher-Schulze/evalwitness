package relation

import (
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestPilotInspectionDecisionDerivesConservativeDispositionAndReasons(t *testing.T) {
	base := PilotInspectionDecision{
		PacketID: "relation-packet-" + digestText("inspection-packet"), PacketDigest: digestText("packet"), MappingDigest: digestText("mapping"),
		CaseID: "mutation-" + digestText("case"), Family: mutation.FamilyTestEvidenceOmitted, Unit: UnitTrajectoryPair,
		TaskContext: PilotInspectionPassed, EvidenceAlignment: PilotInspectionPassed, TransformationIsolation: PilotInspectionPassed,
		InformationSufficiency: PilotInspectionPassed, BlindingIntegrity: PilotInspectionPassed, RubricApplicability: PilotInspectionPassed,
		RedistributionBoundary: PilotInspectionPassed, CandidateOrder: PilotInspectionNotApplicable,
		Disposition: PilotInspectionAccepted,
	}
	if err := validatePilotInspectionDecision(base); err != nil {
		t.Fatal(err)
	}
	unresolved := base
	unresolved.InformationSufficiency = PilotInspectionIndeterminate
	unresolved.ReasonCodes = []PilotInspectionReason{PilotInspectionReasonInformation}
	unresolved.Disposition = PilotInspectionUnresolved
	if err := validatePilotInspectionDecision(unresolved); err != nil {
		t.Fatal(err)
	}
	missingReason := unresolved
	missingReason.ReasonCodes = nil
	if err := validatePilotInspectionDecision(missingReason); err == nil {
		t.Fatal("pilot inspection accepted an indeterminate dimension without its canonical reason")
	}
	revision := base
	revision.TaskContext = PilotInspectionFailed
	revision.ReasonCodes = []PilotInspectionReason{PilotInspectionReasonTaskContext}
	revision.Disposition = PilotInspectionRevisionRequired
	if err := validatePilotInspectionDecision(revision); err != nil {
		t.Fatal(err)
	}
}

func TestPilotInspectionCandidateOrderApplicabilityIsFamilyBound(t *testing.T) {
	trajectory := PilotInspectionDecision{
		PacketID: "relation-packet-" + digestText("trajectory-packet"), PacketDigest: digestText("packet"), MappingDigest: digestText("mapping"),
		CaseID: "mutation-" + digestText("case"), Family: mutation.FamilyNeutralFormatting, Unit: UnitTrajectoryPair,
		TaskContext: PilotInspectionPassed, EvidenceAlignment: PilotInspectionPassed, TransformationIsolation: PilotInspectionPassed,
		InformationSufficiency: PilotInspectionPassed, BlindingIntegrity: PilotInspectionPassed, RubricApplicability: PilotInspectionPassed,
		RedistributionBoundary: PilotInspectionPassed, CandidateOrder: PilotInspectionPassed, Disposition: PilotInspectionAccepted,
	}
	if err := validatePilotInspectionDecision(trajectory); err == nil {
		t.Fatal("trajectory-pair inspection accepted a candidate-order judgment")
	}
	candidate := trajectory
	candidate.Family = mutation.FamilyCandidateOrderReversal
	candidate.Unit = UnitCandidatePairOrders
	if err := validatePilotInspectionDecision(candidate); err != nil {
		t.Fatal(err)
	}
}
