package outcome

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestSelectPilotCasesUsesEveryFamilyAndAvailableNaturalStratumWithoutTaskReuse(t *testing.T) {
	plan := validPlan(t)
	plan.MutationFamilies = []string{"family-a", "family-b"}
	plan.RequiredStrata = []string{"stratum-a", "stratum-b"}
	releaseCases := []mutation.CorpusCase{
		pilotMutationCase("mutation-a", "family-a", "group-"+digestText("shared"), true),
		pilotMutationCase("mutation-b", "family-b", "group-"+digestText("shared"), true),
		pilotMutationCase("mutation-c", "family-b", "group-"+digestText("unique"), true),
		pilotMutationCase("mutation-d", "family-a", "group-"+digestText("not-reviewed"), false),
	}
	natural := []NaturalSelection{{Stratum: "stratum-a", Rank: 1, TaskGroupDigest: digestText("natural-group"), CaseDigest: digestText("natural-case")}}

	cases, selected, unavailable, err := selectPilotCases(plan, releaseCases, natural)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 || !equalStrings(selected, []string{"stratum-a"}) || !equalStrings(unavailable, []string{"stratum-b"}) {
		t.Fatalf("pilot selection = %#v, selected = %#v, unavailable = %#v", cases, selected, unavailable)
	}
	byDimension := make(map[string]PilotCaseReference, len(cases))
	for _, item := range cases {
		byDimension[item.Dimension] = item
	}
	if byDimension["family-a"].TaskGroupID == byDimension["family-b"].TaskGroupID || byDimension["family-b"].TaskGroupID != "group-"+digestText("unique") {
		t.Fatalf("pilot mutation task-group allocation = %#v", cases)
	}
}

func TestPilotSampleCommitmentFailsClosedOnDuplicateTaskGroup(t *testing.T) {
	pilot := validPilotSample(t)
	pilot.Cases[1].TaskGroupID = pilot.Cases[0].TaskGroupID
	sort.Slice(pilot.Cases, func(left, right int) bool {
		return pilotCaseIdentity(pilot.Cases[left]) < pilotCaseIdentity(pilot.Cases[right])
	})
	pilot.SelectionDigest = pilotSelectionDigest(pilot.Cases)
	if _, err := SealPilotSampleCommitment(pilot); err == nil {
		t.Fatal("accepted a pilot sample with repeated task groups")
	}
}

func TestPilotReadinessBindsEveryCommittedCaseWithoutAuthorizingExternalAction(t *testing.T) {
	plan := validPlan(t)
	qualification, err := DefaultQualificationSet()
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(qualification)
	if err != nil {
		t.Fatal(err)
	}
	items, mappings, pilot := outcomePilotReviewFixture(t, plan)
	bundle, err := BuildReviewBundle(plan, qualification, handbook, items, ReviewDataDevelopment, ReviewVisibilityRestricted, "2026-08-09T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	protocol, err := BuildBlindingProtocol(bundle, mappings, "2026-08-09T13:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	bindings := outcomePilotSourceBindingFixture(t, pilot, items, mappings)
	privateMaterials, err := SealOutcomePilotPrivateMaterials(pilot.Digest, mappings, bindings)
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := BuildOutcomePilotInspection(pilot, bundle, privateMaterials)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.ReviewabilityStatus != OutcomePilotStructurallyReady || inspection.SemanticStatus != OutcomePilotRequiresHumanPilot ||
		inspection.Packets != 3 || inspection.PatchAnchors != 2 || inspection.NarrativeAnchors != 1 || inspection.PacketsWithoutMessages != 2 {
		t.Fatalf("pilot inspection = %#v", inspection)
	}
	tamperedBindings := append([]OutcomePilotSourceBinding(nil), bindings...)
	tamperedBindings[0].MappingDigest = digestText("different-mapping")
	tamperedBindings[0], err = SealOutcomePilotSourceBinding(tamperedBindings[0])
	if err != nil {
		t.Fatal(err)
	}
	tamperedPrivate, err := SealOutcomePilotPrivateMaterials(pilot.Digest, mappings, tamperedBindings)
	if err == nil {
		t.Fatal("private pilot materials accepted a source binding for a different mapping")
	}
	if _, err := BuildOutcomePilotInspection(pilot, bundle, tamperedPrivate); err == nil {
		t.Fatal("pilot inspection accepted a source binding for a different private mapping")
	}
	wrongInspection := inspection
	wrongInspection.BundleDigest = digestText("different-bundle")
	wrongInspection, err = SealOutcomePilotInspection(wrongInspection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOutcomePilotReadiness(pilot, bundle, qualification, handbook, protocol, wrongInspection, mappings, "2026-08-09T13:31:00Z"); err == nil {
		t.Fatal("pilot readiness accepted an inspection for a different bundle")
	}

	readiness, err := BuildOutcomePilotReadiness(pilot, bundle, qualification, handbook, protocol, inspection, mappings, "2026-08-09T13:31:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if readiness.TechnicalStatus != PilotTechnicalReady || readiness.ExternalActionStatus != PilotExternalActionNotAuthorized ||
		readiness.Packets != 3 || readiness.RequiredPrimaryLabels != 6 || readiness.MaximumTieBreakLabels != 3 || readiness.RequiredSourceProbes != 6 {
		t.Fatalf("pilot readiness = %#v", readiness)
	}
	wrongSequence := readiness
	wrongSequence.ProtocolCreatedAt = "2026-08-09T11:59:59Z"
	if _, err := SealOutcomePilotReadiness(wrongSequence); err == nil {
		t.Fatal("accepted a blinding protocol frozen before its pilot bundle")
	}
	brokenInspection := inspection
	brokenInspection.TotalRetainedEvents--
	if _, err := SealOutcomePilotInspection(brokenInspection); err == nil {
		t.Fatal("pilot inspection accepted aggregate denominator deletion")
	}

	broken := pilot
	broken.Cases = append([]OutcomePilotCaseReference(nil), pilot.Cases...)
	broken.Cases[0].TaskGroupID = "group-" + digestText("wrong-group")
	sort.Slice(broken.Cases, func(left, right int) bool {
		return outcomePilotCaseIdentity(broken.Cases[left]) < outcomePilotCaseIdentity(broken.Cases[right])
	})
	broken.SelectionDigest = outcomePilotSelectionDigest(broken.Cases)
	broken, err = SealOutcomePilotSampleCommitment(broken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOutcomePilotReadiness(broken, bundle, qualification, handbook, protocol, inspection, mappings, "2026-08-09T13:31:00Z"); err == nil {
		t.Fatal("accepted readiness for a bundle with the wrong committed task group")
	}
	relationMappings := append([]PrivateMapping(nil), mappings...)
	relationMappings[0].ExpectedRelation = "quality_equal"
	relationMappings[0], err = SealPrivateMapping(relationMappings[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOutcomePilotReadiness(pilot, bundle, qualification, handbook, protocol, inspection, relationMappings, "2026-08-09T13:31:00Z"); err == nil {
		t.Fatal("accepted a controlled-relation mapping as an outcome pilot unit")
	}
	relationItems := append([]ReviewItem(nil), items...)
	relationItems[0].Packet.Evidence = append([]PacketEvidence(nil), relationItems[0].Packet.Evidence...)
	relationItems[0].Packet.Evidence[0].Kind = "relation_pair"
	relationItems[0].Packet, err = SealBlindPacket(relationItems[0].Packet, relationItems[0].Packet.PacketID)
	if err != nil {
		t.Fatal(err)
	}
	relationBundle, err := BuildReviewBundle(plan, qualification, handbook, relationItems, ReviewDataDevelopment, ReviewVisibilityRestricted, "2026-08-09T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	relationProtocol, err := BuildBlindingProtocol(relationBundle, mappings, "2026-08-09T13:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOutcomePilotReadiness(pilot, relationBundle, qualification, handbook, relationProtocol, inspection, mappings, "2026-08-09T13:31:00Z"); err == nil {
		t.Fatal("accepted pair-level relation evidence in an outcome pilot bundle")
	}
	if _, err := BuildPilotReadiness(validPilotSample(t), bundle, qualification, handbook, protocol, mappings, "2026-08-09T13:31:00Z"); err == nil {
		t.Fatal("mixed pilot v1 remained launchable")
	}
}

func TestGovernedPilotSampleIsSealedAndBounded(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "eval", "governance", "outcome-pilot-sample-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close outcome pilot sample: %v", err)
		}
	}()
	pilot, err := DecodePilotSampleCommitment(file)
	if err != nil {
		t.Fatal(err)
	}
	if pilot.SelectedCases != 14 || len(pilot.RequiredMutationFamilies) != 8 || len(pilot.SelectedNaturalStrata) != 6 ||
		!equalStrings(pilot.UnavailableNaturalStrata, []string{"abstention", "provider_failure"}) || pilot.RequiredPrimaryLabels != 28 || pilot.RequiredSourceProbes != 28 {
		t.Fatalf("governed pilot sample = %#v", pilot)
	}
}

func TestGovernedOutcomePilotV2IsNaturalOnlyAndExactlyBounded(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "eval", "governance", "outcome-pilot-sample-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close outcome pilot v2 sample: %v", err)
		}
	}()
	pilot, err := DecodeOutcomePilotSampleCommitment(file)
	if err != nil {
		t.Fatal(err)
	}
	if pilot.Objective != ReviewObjectiveOutcome || pilot.SelectedCases != 6 || len(pilot.SelectedNaturalStrata) != 6 ||
		!equalStrings(pilot.UnavailableNaturalStrata, []string{"abstention", "provider_failure"}) || pilot.RequiredPrimaryLabels != 12 ||
		pilot.MaximumTieBreakLabels != 6 || pilot.RequiredSourceProbes != 12 {
		t.Fatalf("governed outcome pilot v2 = %#v", pilot)
	}
	for _, item := range pilot.Cases {
		if item.Objective != ReviewObjectiveOutcome {
			t.Fatalf("governed outcome pilot contains a non-outcome unit: %#v", item)
		}
	}
}

func TestOutcomePilotRouteTokensArePseudonymized(t *testing.T) {
	raw := "source=call_abc__thought__secret call_id=call_abc__thought__secret source=forge_3"
	redacted := pseudonymizeOutcomePilotRouteTokens(redactOutcomePilotIdentities(raw, []string{"forge"}))
	for _, forbidden := range []string{"__thought__", "secret", "forge", "source=call_", "call_id=call_abc"} {
		if strings.Contains(strings.ToLower(redacted), forbidden) {
			t.Fatalf("route token %q survived pseudonymization: %q", forbidden, redacted)
		}
	}
	if !strings.Contains(redacted, "source=source-ref-") || !strings.Contains(redacted, "call_id=call-ref-") {
		t.Fatalf("route tokens lost their typed pseudonymous linkage: %q", redacted)
	}
}

func validPilotSample(t *testing.T) PilotSampleCommitment {
	t.Helper()
	cases := []PilotCaseReference{
		{Source: PilotSourceMutation, Dimension: "family-a", TaskGroupID: "group-" + digestText("group-a"), SourceCaseDigest: digestText("case-a"), SelectionEvidenceDigest: digestText("evidence-a")},
		{Source: PilotSourceMutation, Dimension: "family-b", TaskGroupID: "group-" + digestText("group-b"), SourceCaseDigest: digestText("case-b"), SelectionEvidenceDigest: digestText("evidence-b")},
		{Source: PilotSourceNatural, Dimension: "stratum-a", TaskGroupID: "group-" + digestText("group-c"), SourceCaseDigest: digestText("case-c"), SelectionEvidenceDigest: digestText("case-c")},
	}
	commitment, err := SealPilotSampleCommitment(PilotSampleCommitment{
		PlanDigest: digestText("plan"), MutationSampleDigest: digestText("sample"), MutationReleaseDigest: digestText("release"), NaturalInventoryDigest: digestText("inventory"),
		DataRole: ReviewDataDevelopment, SelectionRule: PilotSelectionRule, RequiredMutationFamilies: []string{"family-a", "family-b"},
		SelectedNaturalStrata: []string{"stratum-a"}, UnavailableNaturalStrata: []string{"stratum-b"}, SelectedCases: len(cases),
		RequiredPrimaryReviewers: 2, RequiredTieBreakReviewers: 1, RequiredPrimaryLabels: 6, MaximumTieBreakLabels: 3, RequiredSourceProbes: 6,
		Cases: cases, SelectionDigest: pilotSelectionDigest(cases), Limitations: pilotSampleLimitations(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return commitment
}

func outcomePilotReviewFixture(t *testing.T, plan Plan) ([]ReviewItem, []PrivateMapping, OutcomePilotSampleCommitment) {
	t.Helper()
	var items []ReviewItem
	var mappings []PrivateMapping
	var cases []OutcomePilotCaseReference
	for index, stratum := range []string{"stratum-a", "stratum-b", "stratum-c"} {
		taskContent := "task requirement " + stratum
		trajectoryContent := "trajectory evidence " + stratum
		sourceCaseDigest := digestText("outcome-case-" + stratum)
		request := BlindBuildRequest{
			SchemaVersion: BlindBuildSchemaVersion, PlanDigest: plan.Digest, TaskAlias: "source-task-" + stratum,
			Evidence: []PacketEvidence{
				{Slot: "task", Kind: "task_requirement", Content: taskContent, ContentDigest: digestText(taskContent), License: "reference_only", Limitation: "test fixture"},
				{Slot: "trajectory", Kind: "trajectory_evidence", Content: trajectoryContent, ContentDigest: digestText(trajectoryContent), License: "reference_only", Limitation: "test fixture"},
			},
			RubricQuestions: []string{"Does the trajectory satisfy the task?"}, PrivacyClass: "restricted_reference_only", PublicReleasable: false,
			SourceCaseDigest: sourceCaseDigest, Condition: "natural." + stratum, ExpectedRelation: OutcomePilotExpectedRelation,
			SlotMappings:  []SlotMapping{{Slot: "task", SourceDigest: digestText("task-source-" + stratum)}, {Slot: "trajectory", SourceDigest: digestText("trajectory-source-" + stratum)}},
			BlindingKeyID: "outcome-pilot-test-key", ForbiddenValues: []string{"natural." + stratum, OutcomePilotExpectedRelation, "source-task-" + stratum},
		}
		packet, mapping, err := BuildBlindedPacketFromRequest(request, reviewSeed(byte('a'+index)))
		if err != nil {
			t.Fatal(err)
		}
		taskGroupID := "group-" + digestText("outcome-group-"+stratum)
		items = append(items, ReviewItem{TaskGroupID: taskGroupID, Packet: packet})
		mappings = append(mappings, mapping)
		cases = append(cases, OutcomePilotCaseReference{Objective: ReviewObjectiveOutcome, Stratum: stratum, TaskGroupID: taskGroupID, SourceCaseDigest: sourceCaseDigest, SelectionEvidenceDigest: sourceCaseDigest})
	}
	sort.Slice(cases, func(left, right int) bool {
		return outcomePilotCaseIdentity(cases[left]) < outcomePilotCaseIdentity(cases[right])
	})
	pilot, err := SealOutcomePilotSampleCommitment(OutcomePilotSampleCommitment{
		PlanDigest: plan.Digest, NaturalInventoryDigest: digestText("natural-inventory"), Objective: ReviewObjectiveOutcome, DataRole: ReviewDataDevelopment,
		SelectionRule: OutcomePilotSelectionRule, SelectedNaturalStrata: []string{"stratum-a", "stratum-b", "stratum-c"}, UnavailableNaturalStrata: []string{},
		SelectedCases: 3, RequiredPrimaryReviewers: 2, RequiredTieBreakReviewers: 1, RequiredPrimaryLabels: 6, MaximumTieBreakLabels: 3, RequiredSourceProbes: 6,
		Cases: cases, SelectionDigest: outcomePilotSelectionDigest(cases), Limitations: outcomePilotSampleLimitations(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return items, mappings, pilot
}

func outcomePilotSourceBindingFixture(t *testing.T, pilot OutcomePilotSampleCommitment, items []ReviewItem, mappings []PrivateMapping) []OutcomePilotSourceBinding {
	t.Helper()
	itemByPacket := make(map[string]ReviewItem, len(items))
	for _, item := range items {
		itemByPacket[item.Packet.PacketID] = item
	}
	pilotBySource := make(map[string]OutcomePilotCaseReference, len(pilot.Cases))
	for _, pilotCase := range pilot.Cases {
		pilotBySource[pilotCase.SourceCaseDigest] = pilotCase
	}
	bindings := make([]OutcomePilotSourceBinding, 0, len(mappings))
	for index, mapping := range mappings {
		pilotCase := pilotBySource[mapping.SourceCaseDigest]
		item := itemByPacket[mapping.PacketID]
		anchorKind := string(preprocess.EventFileChange)
		messages := 0
		if index == len(mappings)-1 {
			anchorKind = string(preprocess.EventMessage)
			messages = 1
		}
		binding, err := SealOutcomePilotSourceBinding(OutcomePilotSourceBinding{
			PilotSampleDigest: pilot.Digest, NaturalInventoryDigest: pilot.NaturalInventoryDigest, Objective: ReviewObjectiveOutcome,
			Stratum: pilotCase.Stratum, Suite: "fixture", TaskID: "fixture-task-" + pilotCase.Stratum, SelectedIndex: index, SelectedReward: index % 2,
			SourceLocation: "fixture/source.json#/" + pilotCase.Stratum, SourceRevision: "fixture-revision", SourceDigest: digestText("source-" + pilotCase.Stratum),
			TrajectoryDigest: digestText("trajectory-" + pilotCase.Stratum), RetainedDigest: digestText("retained-" + pilotCase.Stratum),
			SourceEvents: 10 + index, RetainedEvents: 5 + index, RedactionHits: 1, EvidenceBudgetTokens: OutcomePilotEvidenceBudgetTokens,
			EvidenceSelector: OutcomePilotEvidenceSelectorVersion, DecisionAnchorKind: anchorKind, DecisionAnchorDigest: digestText("anchor-" + pilotCase.Stratum),
			RetainedMessages: messages, RetainedActions: 2, RetainedResults: 2, LicenseSPDX: "MIT", SourceURL: "https://example.invalid/source",
			Redistribution: "reference_only", PacketID: item.Packet.PacketID, MappingDigest: mapping.Digest,
		})
		if err != nil {
			t.Fatal(err)
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func pilotMutationCase(id, family, group string, reviewRequired bool) mutation.CorpusCase {
	return mutation.CorpusCase{
		ID: id, Family: mutation.Family(family), Split: study.RoleDevelopment,
		Manifest: mutation.Manifest{
			SplitGroupID: group, Digest: digestText(id), Review: mutation.ReviewState{Required: reviewRequired},
		},
		BlindPacket: mutation.BlindReviewPacket{Digest: digestText(id + "-packet")},
	}
}
