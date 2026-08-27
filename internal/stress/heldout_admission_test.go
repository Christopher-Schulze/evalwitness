package stress

import (
	"bytes"
	"reflect"
	"sort"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestHeldOutAdmissionPlanPartitionsStructuralSupportBeforeExecutionAuthority(t *testing.T) {
	_, lock, design, armPlan, registry, replayed, owner := currentHeldOutReadinessRefusal(t)
	campaign, err := BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	corpusPlan, corpusAudit, corpusRelease := currentCorpusV3(t)
	relationPlan, primarySample := currentRelationGovernanceV3(t)
	owner = passedHeldOutOwner(t, owner)
	ledger := supportedPrimaryLedgerV3(t, primarySample, corpusRelease)

	value, err := BuildHeldOutAdmissionPlan(
		campaign, lock, design, armPlan, registry, replayed,
		corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample,
		owner, owner.PackageInventoryDigest, ledger,
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.HeldOutCases != 57 || value.RelationCases != 114 || value.PrimarySampleCases != 28 || value.PrimarySampleTestCases != 14 ||
		value.TerminalLedgerCases != 28 || value.TerminalLedgerTestCases != 14 || value.HumanSupportedTestCases != 14 ||
		value.StructurallySupportedCells != 440 || value.ExecutionEligibleCells != 276 || value.PreExecutionIneligibleCells != 164 ||
		value.EligibleLiveProviderCells != 142 || value.IneligibleLiveProviderCells != 86 ||
		value.EligibleSealedReplayCells != 71 || value.IneligibleSealedReplayCells != 43 ||
		value.EligibleDeterministicLocalCells != 63 || value.IneligibleDeterministicLocalCells != 35 ||
		value.EligibleProviderVerificationInputs != 426 || value.EligibleLiveVerificationInputs != 284 ||
		value.EligibleSealedReplayVerificationInputs != 142 || value.PlannedEligibleProviderRepetitions != 1278 ||
		value.PlannedEligibleLiveRepetitions != 852 || value.PlannedEligibleSealedReplayRepetitions != 426 ||
		value.PlannedEligibleLocalRepetitions != 189 || value.RunAuthorized || value.ExecutionPermitIssued ||
		value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired {
		t.Fatalf("held-out admission plan = %+v", value)
	}
	if err := value.ValidateAgainst(
		campaign, lock, design, armPlan, registry, replayed,
		corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample,
		owner, owner.PackageInventoryDigest, ledger,
	); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHeldOutAdmissionPlan(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, value) {
		t.Fatal("held-out admission plan changed across strict JSON decoding")
	}
	schema, err := Schema("held-out-admission-plan")
	if err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	if schema["additionalProperties"] != false || properties["schema_version"].(JSONSchema)["const"] != HeldOutAdmissionPlanSchemaVersion {
		t.Fatal("held-out admission plan schema is open or unpinned")
	}

	t.Run("rejects incomplete but internally valid terminal ledger", func(t *testing.T) {
		truncated := ledger
		truncated.Entries = append([]relationevidence.RelationLedgerEntry(nil), ledger.Entries[1:]...)
		truncated.PacketStates.Denominator--
		truncated.PacketStates.Supports--
		truncated, err = relationevidence.SealTerminalRelationLedger(truncated)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildHeldOutAdmissionPlan(
			campaign, lock, design, armPlan, registry, replayed,
			corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample,
			owner, owner.PackageInventoryDigest, truncated,
		); err == nil {
			t.Fatal("held-out admission accepted a terminal ledger that omitted a frozen primary-sample case")
		}
	})

	t.Run("human contradiction excludes both registered estimands before execution", func(t *testing.T) {
		contradicted := terminalLedgerWithTestState(t, ledger, primarySample, relationevidence.TranslationContradicts)
		changed, err := BuildHeldOutAdmissionPlan(
			campaign, lock, design, armPlan, registry, replayed,
			corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample,
			owner, owner.PackageInventoryDigest, contradicted,
		)
		if err != nil {
			t.Fatal(err)
		}
		if changed.HumanContradictedTestCases != 1 || changed.HumanSupportedTestCases != 13 ||
			changed.ExecutionEligibleCells >= value.ExecutionEligibleCells || changed.PreExecutionIneligibleCells <= value.PreExecutionIneligibleCells {
			t.Fatalf("contradicted held-out admission = %+v", changed)
		}
	})

	t.Run("human unresolved excludes primary but retains sensitivity", func(t *testing.T) {
		unresolved := terminalLedgerWithTestState(t, ledger, primarySample, relationevidence.TranslationUnresolved)
		changed, err := BuildHeldOutAdmissionPlan(
			campaign, lock, design, armPlan, registry, replayed,
			corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample,
			owner, owner.PackageInventoryDigest, unresolved,
		)
		if err != nil {
			t.Fatal(err)
		}
		caseID := firstPrimaryTestCaseID(t, primarySample)
		expectedEligible := value.ExecutionEligibleCells
		for _, entry := range value.Entries {
			if entry.CaseID == caseID && entry.Estimand == EstimandPrimaryCore {
				expectedEligible -= len(entry.StructurallySupportedCellIDs)
			}
		}
		if changed.HumanUnresolvedTestCases != 1 || changed.HumanSupportedTestCases != 13 ||
			changed.ExecutionEligibleCells != expectedEligible {
			t.Fatalf("unresolved held-out admission = %+v", changed)
		}
	})

	t.Run("rejects resealed authority promotion", func(t *testing.T) {
		promoted := value
		promoted.RunAuthorized = true
		promoted.ExecutionPermitIssued = true
		promoted.Digest = ""
		promoted.Digest, err = heldOutAdmissionPlanDigest(promoted)
		if err != nil {
			t.Fatal(err)
		}
		if err := promoted.Validate(); err == nil {
			t.Fatal("held-out admission plan promoted eligibility into execution authority")
		}
	})
}

func currentRelationGovernanceV3(t *testing.T) (relationevidence.RelationPlanV3, relationevidence.PrimarySampleV3) {
	t.Helper()
	root := "../.."
	plan, err := relationevidence.DecodePlanV3(openFixture(t, root+"/eval/governance/relation-audit-plan-v3.json"))
	if err != nil {
		t.Fatal(err)
	}
	primary, err := relationevidence.DecodePrimarySampleV3(openFixture(t, root+"/eval/governance/relation-primary-sample-v3.json"), plan)
	if err != nil {
		t.Fatal(err)
	}
	return plan, primary
}

func passedHeldOutOwner(t *testing.T, owner relationevidence.OwnerInspectionPublicAttestation) relationevidence.OwnerInspectionPublicAttestation {
	t.Helper()
	for index := range owner.Dimensions {
		owner.Dimensions[index].Passed = owner.Dimensions[index].Applicable
		owner.Dimensions[index].Failed = 0
		owner.Dimensions[index].Indeterminate = 0
	}
	owner.Outcomes = relationevidence.OwnerInspectionPublicOutcomes{
		Core:             relationevidence.OwnerInspectionPublicDispositionCounts{Accepted: owner.Assessments.CoreCases},
		ScarcityCases:    relationevidence.OwnerInspectionPublicDispositionCounts{Accepted: owner.Assessments.ScarcityCaseCount},
		ScarcityBoundary: relationevidence.PilotInspectionAccepted,
		CoreStatus:       relationevidence.PilotInspectionOverallPassed,
		ScarcityStatus:   relationevidence.PilotInspectionOverallPassed,
		OverallStatus:    relationevidence.PilotInspectionOverallPassed,
	}
	sealed, err := relationevidence.SealOwnerInspectionPublicAttestation(owner)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func supportedPrimaryLedgerV3(
	t *testing.T,
	primary relationevidence.PrimarySampleV3,
	release mutation.CorpusReleaseV3,
) relationevidence.TerminalRelationLedger {
	t.Helper()
	releaseByID := make(map[string]mutation.CorpusCaseV3, len(release.Cases))
	for _, item := range release.Cases {
		releaseByID[item.ID] = item
	}
	entries := make([]relationevidence.RelationLedgerEntry, 0, len(primary.Cases))
	for _, reference := range primary.Cases {
		item, exists := releaseByID[reference.CaseID]
		if !exists {
			t.Fatalf("primary case %q not found in release", reference.CaseID)
		}
		definition, exists := mutation.DefinitionFor(item.Family)
		if !exists {
			t.Fatalf("primary case %q has no mutation definition", reference.CaseID)
		}
		entries = append(entries, relationevidence.RelationLedgerEntry{
			PacketID:               "relation-packet-" + digestText("packet:"+item.ID),
			PacketDigest:           digestText("packet-digest:" + item.ID),
			CaseID:                 item.ID,
			Family:                 item.Family,
			ExpectedRelation:       definition.Relation,
			FormalWitnessDigest:    item.Manifest.Witness.Digest,
			HumanResolutionDigest:  digestText("human-resolution:" + item.ID + ":supports"),
			HumanState:             relationevidence.TranslationSupports,
			AdmissibilityStatus:    relationevidence.RelationAdmissibilityHumanSupported,
			VerifierRelationStatus: relationevidence.VerifierRelationNotConsulted,
		})
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].PacketID < entries[right].PacketID })
	probeDigests := []string{digestText("probe-batch:1"), digestText("probe-batch:2")}
	sort.Strings(probeDigests)
	ledger, err := relationevidence.SealTerminalRelationLedger(relationevidence.TerminalRelationLedger{
		ProtocolVersion:             relationevidence.ProtocolVersionV3,
		Objective:                   relationevidence.ReviewObjectiveControlledRelation,
		PlanDigest:                  primary.PlanDigest,
		BundleDigest:                digestText("primary-review-bundle"),
		SampleDigest:                primary.Digest,
		DataRole:                    relationevidence.ReviewDataPrimaryAudit,
		MappingRevealDigest:         digestText("mapping-reveal"),
		AmbiguityAnalysisDigest:     digestText("ambiguity-analysis"),
		FormalHumanComparisonDigest: digestText("formal-human-comparison"),
		PrimaryCommitments: []relationevidence.JudgmentCommitmentReference{
			{ReviewerSlot: 1, AssignmentDigest: digestText("assignment:1"), BatchDigest: digestText("judgment-batch:1")},
			{ReviewerSlot: 2, AssignmentDigest: digestText("assignment:2"), BatchDigest: digestText("judgment-batch:2")},
		},
		ProbeBatchDigests: probeDigests,
		Entries:           entries,
		PacketStates: relationevidence.StateTally{
			Denominator: len(entries), Supports: len(entries),
		},
		Status: "complete",
		EvidenceLayers: relationevidence.EvidenceLayerBoundary{
			FormalRelationSource:   "frozen_mutation_manifest_and_witness",
			HumanConstructSource:   "post_reveal_resolved_blinded_pair_judgments",
			VerifierRelationStatus: relationevidence.VerifierRelationNotConsulted,
		},
		CompletedAt:          "2026-08-12T12:00:00Z",
		ExternalActionStatus: relationevidence.ExternalActionNotAuthorized,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ledger
}

func terminalLedgerWithTestState(
	t *testing.T,
	ledger relationevidence.TerminalRelationLedger,
	primary relationevidence.PrimarySampleV3,
	state relationevidence.TranslationState,
) relationevidence.TerminalRelationLedger {
	t.Helper()
	caseID := firstPrimaryTestCaseID(t, primary)
	ledger.Entries = append([]relationevidence.RelationLedgerEntry(nil), ledger.Entries...)
	changed := false
	for index := range ledger.Entries {
		if ledger.Entries[index].CaseID != caseID {
			continue
		}
		ledger.Entries[index].HumanState = state
		ledger.Entries[index].HumanResolutionDigest = digestText("human-resolution:" + caseID + ":" + string(state))
		ledger.PacketStates.Supports--
		switch state {
		case relationevidence.TranslationContradicts:
			ledger.Entries[index].AdmissibilityStatus = relationevidence.RelationAdmissibilityHumanContradicted
			ledger.PacketStates.Contradicts++
			ledger.Status = "complete"
		case relationevidence.TranslationUnresolved:
			ledger.Entries[index].AdmissibilityStatus = relationevidence.RelationAdmissibilityHumanUnresolved
			ledger.PacketStates.Unresolved++
			ledger.Status = "complete_with_unresolved"
		default:
			t.Fatalf("unsupported test state %q", state)
		}
		changed = true
		break
	}
	if !changed {
		t.Fatalf("ledger lacks held-out primary case %q", caseID)
	}
	sealed, err := relationevidence.SealTerminalRelationLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func firstPrimaryTestCaseID(t *testing.T, primary relationevidence.PrimarySampleV3) string {
	t.Helper()
	for _, reference := range primary.Cases {
		if reference.DataRole == study.RoleTest {
			return reference.CaseID
		}
	}
	t.Fatal("primary sample has no held-out test case")
	return ""
}
