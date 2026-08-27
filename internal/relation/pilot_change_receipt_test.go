package relation

import (
	"sort"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestPilotChangeReceiptRejectsWeakenedEvidenceAndClaimBoundaries(t *testing.T) {
	t.Parallel()

	receipt := validPilotChangeReceiptForTest(t)
	tests := []struct {
		name   string
		mutate func(*PilotChangeReceipt)
	}{
		{name: "changed-line digest", mutate: func(value *PilotChangeReceipt) {
			for index := range value.Checks {
				if value.Checks[index].Unit == UnitTrajectoryPair {
					value.Checks[index].TrajectoryChanges[0].OriginalChangedLinesDigest = "invalid"
					return
				}
			}
		}},
		{name: "line coverage", mutate: func(value *PilotChangeReceipt) { value.CoverageStatus = "sampled" }},
		{name: "decision status", mutate: func(value *PilotChangeReceipt) { value.DecisionStatus = "passed" }},
		{name: "external authorization", mutate: func(value *PilotChangeReceipt) { value.ExternalActionStatus = ExternalActionStatus("authorized") }},
		{name: "candidate identity", mutate: func(value *PilotChangeReceipt) {
			for index := range value.Checks {
				if value.Checks[index].Unit == UnitCandidatePairOrders {
					value.Checks[index].CandidateReversal[0].ExactContentMatch = false
					return
				}
			}
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := receipt
			changed.Checks = clonePilotChangeChecks(receipt.Checks)
			test.mutate(&changed)
			if _, err := SealPilotChangeReceipt(changed); err == nil {
				t.Fatal("SealPilotChangeReceipt() accepted a weakened receipt")
			}
		})
	}
}

func validPilotChangeReceiptForTest(t *testing.T) PilotChangeReceipt {
	t.Helper()

	families := pilotLaunchFamilies()
	checks := make([]PilotChangeCheck, len(families))
	causalReviews := 0
	for index, family := range families {
		definition, exists := mutation.DefinitionFor(family)
		if !exists {
			t.Fatalf("missing mutation definition for %s", family)
		}
		check := PilotChangeCheck{
			PacketID: "relation-packet-" + digestText("receipt-packet-id-"+string(family)), PacketDigest: digestText("receipt-packet-" + string(family)),
			MappingDigest: digestText("receipt-mapping-" + string(family)), CaseID: "mutation-" + digestText("receipt-case-"+string(family)),
			Family: family, ExpectedRelation: definition.Relation, Unit: UnitTrajectoryPair,
			TaskRequirementDigest: digestText("receipt-task-" + string(family)), TrajectoryChanges: []PilotTrajectoryChange{},
			CandidateReversal: []PilotCandidateReversalBinding{},
		}
		if definition.PairLevel {
			check.Unit = UnitCandidatePairOrders
			check.CandidateReversal = []PilotCandidateReversalBinding{
				{SourceCandidateIndex: 1, OriginalVisiblePosition: PositionLeft, OriginalVisibleEvidenceIndex: 1, TransformedVisiblePosition: PositionRight, TransformedVisibleEvidenceIndex: 2, ContentDigest: digestText("candidate-1"), ExactContentMatch: true},
				{SourceCandidateIndex: 2, OriginalVisiblePosition: PositionLeft, OriginalVisibleEvidenceIndex: 2, TransformedVisiblePosition: PositionRight, TransformedVisibleEvidenceIndex: 1, ContentDigest: digestText("candidate-2"), ExactContentMatch: true},
			}
		} else {
			manualReview := family == mutation.FamilyCausalIndependentReorder
			if manualReview {
				causalReviews++
			}
			check.TrajectoryChanges = []PilotTrajectoryChange{{
				Original:          PilotChangeSideBinding{LogicalSide: LogicalOriginal, VisiblePosition: PositionLeft, ContentDigest: digestText("original-" + string(family)), RetainedLineageDigest: digestText("lineage-" + string(family)), SourceEvents: 5, RetainedEvents: 5},
				Transformed:       PilotChangeSideBinding{LogicalSide: LogicalTransformed, VisiblePosition: PositionRight, ContentDigest: digestText("transformed-" + string(family)), RetainedLineageDigest: digestText("lineage-" + string(family)), SourceEvents: 5, RetainedEvents: 5},
				CommonPrefixLines: 2, CommonSuffixLines: 1, OriginalChangedLines: 1, TransformedChangedLines: 1,
				OriginalChangedLinesDigest: digestText("original-lines-" + string(family)), TransformedChangedLinesDigest: digestText("transformed-lines-" + string(family)),
				PairedLineageEqual: true, ManualCausalReferenceReviewRequired: manualReview,
			}}
		}
		checks[index] = check
	}
	sort.Slice(checks, func(left, right int) bool { return checks[left].PacketID < checks[right].PacketID })
	receipt := PilotChangeReceipt{
		ProtocolVersion: ProtocolVersion, Objective: ReviewObjectiveControlledRelation, ReceiptPolicy: PilotChangeReceiptPolicy,
		ReadinessDigest: digestText("receipt-readiness"), BundleDigest: digestText("receipt-bundle"), MappingCommitmentDigest: digestText("receipt-mappings"),
		DataRole: ReviewDataDevelopmentPilot, Visibility: ReviewVisibilityRestricted, Packets: 8, Checks: checks,
		TrajectoryPairs: 7, CandidateOrderControls: 1, ManualCausalReferenceReviews: causalReviews,
		CoverageStatus: PilotChangeReceiptCoverage, ContentStatus: PilotChangeReceiptContent, DecisionStatus: PilotChangeReceiptDecision,
		HumanStudyStatus: PilotChangeReceiptHumanStudy, ExternalActionStatus: ExternalActionNotAuthorized, Limitations: pilotChangeReceiptLimitations(),
	}
	sealed, err := SealPilotChangeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func clonePilotChangeChecks(checks []PilotChangeCheck) []PilotChangeCheck {
	cloned := append([]PilotChangeCheck(nil), checks...)
	for index := range cloned {
		cloned[index].TrajectoryChanges = append([]PilotTrajectoryChange(nil), checks[index].TrajectoryChanges...)
		cloned[index].CandidateReversal = append([]PilotCandidateReversalBinding(nil), checks[index].CandidateReversal...)
	}
	return cloned
}
