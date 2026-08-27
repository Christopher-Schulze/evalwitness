package relation

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestPilotPackageInventoryReproducesSortedKeyDigest(t *testing.T) {
	inventory := PilotPackageInventory{
		SchemaVersion: PilotPackageInventorySchemaVersion,
		PackageFormat: PilotPackageFormatV5,
		HashAlgorithm: PilotPackageInventoryHashAlgorithm,
		Scope:         PilotPackageInventoryScope,
		Directories:   []PilotPackageInventoryDirectory{{Path: "packets", Mode: "0700"}},
		Files: []PilotPackageInventoryFile{{
			Path: "packets/01.json", Bytes: 3, Mode: "0600",
			SHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		}},
		PayloadFiles: 1,
		PayloadBytes: 3,
		Digest:       "0049281da0c3c1942c9251bee090fe50f6135a57e79deff523e11ea19302dc15",
	}
	if err := inventory.Validate(); err != nil {
		t.Fatal(err)
	}
	tampered := inventory
	tampered.PayloadBytes++
	if err := tampered.Validate(); err == nil {
		t.Fatal("package inventory accepted a changed byte denominator")
	}
}

func TestPilotInspectionJournalSchemasCloseExactScope(t *testing.T) {
	sessionSchema, err := Schema("pilot-inspection-session")
	if err != nil {
		t.Fatal(err)
	}
	properties := sessionSchema["properties"].(map[string]any)
	coreDimensions := properties["core_dimensions"].(JSONSchema)
	scarcityCases := properties["scarcity_cases"].(JSONSchema)
	if coreDimensions["minItems"] != 8 || coreDimensions["maxItems"] != 8 || scarcityCases["minItems"] != 3 || scarcityCases["maxItems"] != 3 {
		t.Fatal("guided owner inspection session schema does not close the core and scarcity cardinalities")
	}
	eventSchema, err := Schema("pilot-inspection-event")
	if err != nil {
		t.Fatal(err)
	}
	eventProperties := eventSchema["properties"].(map[string]any)
	subjectKinds := eventProperties["subject_kind"].(JSONSchema)["enum"].([]string)
	if len(subjectKinds) != 3 {
		t.Fatal("guided owner inspection event schema does not close the subject vocabulary")
	}
	completionSchema, err := Schema("pilot-inspection-completion")
	if err != nil {
		t.Fatal(err)
	}
	completionProperties := completionSchema["properties"].(map[string]any)
	if completionProperties["required_assessments"].(JSONSchema)["const"] != PilotInspectionRequiredAssessments {
		t.Fatal("guided owner inspection completion schema does not freeze the 66-assessment denominator")
	}
}

func TestPilotInspectionJournalIsFailClosedAndReproducible(t *testing.T) {
	fixture := newPilotInspectionJournalFixture(t)
	session, readiness, bundle, mappings := fixture.session, fixture.readiness, fixture.bundle, fixture.mappings
	events := make([]PilotInspectionEvent, 0, PilotInspectionRequiredAssessments+1)
	nextTime := mustJournalTime(t, session.CreatedAt)

	firstPacket := session.Packets[0]
	first, err := BuildPilotInspectionEvent(
		session, events, PilotInspectionSubjectCorePacket, firstPacket.PacketID, PilotInspectionDimensionTaskContext,
		PilotInspectionFailed, nextTime(), false,
	)
	if err != nil {
		t.Fatal(err)
	}
	events = append(events, first)
	if _, err := BuildPilotInspectionEvent(
		session, events, PilotInspectionSubjectCorePacket, firstPacket.PacketID, PilotInspectionDimensionTaskContext,
		PilotInspectionPassed, nextTime(), false,
	); err == nil {
		t.Fatal("journal accepted an implicit overwrite without correction intent")
	}
	corrected, err := BuildPilotInspectionEvent(
		session, events, PilotInspectionSubjectCorePacket, firstPacket.PacketID, PilotInspectionDimensionTaskContext,
		PilotInspectionPassed, nextTime(), true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if corrected.SupersedesDigest != first.Digest {
		t.Fatal("correction does not bind the assessment it supersedes")
	}
	events = append(events, corrected)

	for _, packet := range session.Packets {
		for _, dimension := range session.CoreDimensions {
			if !coreDimensionApplies(packet, dimension) || packet.PacketID == firstPacket.PacketID && dimension == PilotInspectionDimensionTaskContext {
				continue
			}
			event, buildErr := BuildPilotInspectionEvent(
				session, events, PilotInspectionSubjectCorePacket, packet.PacketID, dimension, PilotInspectionPassed, nextTime(), false,
			)
			if buildErr != nil {
				t.Fatalf("record %s/%s: %v", packet.PacketID, dimension, buildErr)
			}
			events = append(events, event)
		}
	}
	coreOnlyStatus, err := VerifyPilotInspectionJournal(session, events)
	if err != nil {
		t.Fatal(err)
	}
	if coreOnlyStatus.ReadyToFinalize || coreOnlyStatus.CompletedCoreAssessments != PilotInspectionCoreAssessments ||
		coreOnlyStatus.CompletedScarcityAssessments != 0 || coreOnlyStatus.Next == nil || coreOnlyStatus.Next.SubjectKind != PilotInspectionSubjectScarcityCase {
		t.Fatalf("core-only journal did not stop at the scarcity gate: %+v", coreOnlyStatus)
	}
	if _, _, err := BuildPilotInspectionCompletion(session, events, readiness, bundle, mappings, nextTime()); err == nil {
		t.Fatal("core-only journal finalized without the required scarcity inspection")
	}
	for caseIndex, item := range session.ScarcityCases {
		for dimensionIndex, dimension := range session.ScarcityCaseDimensions {
			assessment := PilotInspectionPassed
			if caseIndex == 0 && dimensionIndex == 0 {
				assessment = PilotInspectionFailed
			}
			event, buildErr := BuildPilotInspectionEvent(
				session, events, PilotInspectionSubjectScarcityCase, item.CaseID, dimension, assessment, nextTime(), false,
			)
			if buildErr != nil {
				t.Fatalf("record scarcity %s/%s: %v", item.CaseID, dimension, buildErr)
			}
			events = append(events, event)
		}
	}
	for _, dimension := range session.ScarcityBoundaryDimensions {
		event, buildErr := BuildPilotInspectionEvent(
			session, events, PilotInspectionSubjectScarcityBoundary, PilotInspectionScarcityBoundaryID, dimension, PilotInspectionPassed, nextTime(), false,
		)
		if buildErr != nil {
			t.Fatalf("record scarcity boundary %s: %v", dimension, buildErr)
		}
		events = append(events, event)
	}
	status, err := VerifyPilotInspectionJournal(session, events)
	if err != nil {
		t.Fatal(err)
	}
	if !status.ReadyToFinalize || status.RequiredAssessments != PilotInspectionRequiredAssessments || status.CompletedAssessments != PilotInspectionRequiredAssessments ||
		status.CompletedCoreAssessments != PilotInspectionCoreAssessments || status.CompletedScarcityAssessments != PilotInspectionScarcityAssessments ||
		status.CompletedBoundaryAssessments != PilotInspectionBoundaryAssessments || status.Events != PilotInspectionRequiredAssessments+1 || status.Corrections != 1 {
		t.Fatalf("unexpected complete journal status: %+v", status)
	}

	inspectedAt := nextTime()
	record, completion, err := BuildPilotInspectionCompletion(session, events, readiness, bundle, mappings, inspectedAt)
	if err != nil {
		t.Fatal(err)
	}
	if record.OverallStatus != PilotInspectionOverallPassed || completion.CoreStatus != PilotInspectionOverallPassed ||
		completion.ScarcityStatus != PilotInspectionOverallRevisionRequired || completion.OverallStatus != PilotInspectionOverallRevisionRequired {
		t.Fatal("scarcity failure did not conservatively determine the combined final status")
	}
	if err := VerifyPilotInspectionCompletion(completion, session, events, record, readiness, bundle, mappings); err != nil {
		t.Fatal(err)
	}

	truncated := append([]PilotInspectionEvent(nil), events[:len(events)-1]...)
	if _, _, err := BuildPilotInspectionCompletion(session, truncated, readiness, bundle, mappings, inspectedAt); err == nil {
		t.Fatal("truncated journal produced final evidence")
	}
	tampered := append([]PilotInspectionEvent(nil), events...)
	tampered[2].Assessment = PilotInspectionFailed
	if _, err := VerifyPilotInspectionJournal(session, tampered); err == nil {
		t.Fatal("journal accepted an edited event")
	}
	reordered := append([]PilotInspectionEvent(nil), events...)
	reordered[2], reordered[3] = reordered[3], reordered[2]
	if _, err := VerifyPilotInspectionJournal(session, reordered); err == nil {
		t.Fatal("journal accepted reordered events")
	}
	crossCase := append([]PilotInspectionEvent(nil), events...)
	last := len(crossCase) - 1
	crossCase[last].SubjectKind = PilotInspectionSubjectScarcityCase
	crossCase[last].SubjectID = session.ScarcityCases[0].CaseID
	crossCase[last].Dimension = PilotInspectionDimensionScarcityOriginalEvidence
	crossCase[last].Digest = ""
	crossCase[last].Digest, err = pilotInspectionEventDigest(crossCase[last])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPilotInspectionJournal(session, crossCase); err == nil {
		t.Fatal("journal accepted a boundary event moved onto a scarcity case")
	}
	crossPackage := fixture.binding
	crossPackage.PackageInventoryDigest = digestText("other-package")
	if err := VerifyPilotInspectionSession(session, readiness, bundle, mappings, fixture.plan, fixture.primary, fixture.sentinel, fixture.pilot, fixture.scarcityMaterials, crossPackage); err == nil {
		t.Fatal("session accepted a different package inventory")
	}
}

type pilotInspectionJournalFixture struct {
	session           PilotInspectionSession
	readiness         RelationPilotReadiness
	bundle            ReviewBundle
	mappings          []PrivateMapping
	plan              RelationPlanV3
	primary           PrimarySampleV3
	sentinel          ScarcitySentinelV3
	pilot             PilotSampleV3
	scarcityMaterials []CaseMaterial
	binding           PilotInspectionPackageBinding
}

func newPilotInspectionJournalFixture(t *testing.T) pilotInspectionJournalFixture {
	t.Helper()
	corpusPlan, audit, release := readGovernedCorpusV3(t)
	governedPlan := readGovernedPlanV3(t, "relation-audit-plan-v3.json")
	primary := readGovernedPrimaryV3(t, "relation-primary-sample-v3.json", governedPlan)
	sentinel := readGovernedSentinelV3(t, "relation-scarcity-sentinel-v3.json", governedPlan, primary)
	pilot := readGovernedPilotV3(t, "relation-pilot-sample-v3.json", governedPlan, primary, sentinel)
	plan, err := ReviewPlanV3(governedPlan)
	if err != nil {
		t.Fatal(err)
	}
	reviewPilot, err := ReviewPilotSampleV3(governedPlan, primary, sentinel, pilot)
	if err != nil {
		t.Fatal(err)
	}
	caseByID, sourceByID := governedV3CasesAndSources(release)
	scarcityMaterials := make([]CaseMaterial, len(sentinel.Cases))
	for index, reference := range sentinel.Cases {
		item := caseByID[reference.CaseID]
		receipt := sealedV3ReplayReceiptFixture(t, release.Digest, item, sourceByID)
		scarcityMaterials[index] = sealedV3CaseMaterialFixture(t, plan, corpusPlan, audit, release, item, sourceByID, receipt)
	}
	packets := make([]BlindPacket, len(pilot.Cases))
	mappings := make([]PrivateMapping, len(pilot.Cases))
	for index, reference := range pilot.Cases {
		item := caseByID[reference.CaseID]
		receipt := sealedV3ReplayReceiptFixture(t, release.Digest, item, sourceByID)
		material := sealedV3CaseMaterialFixture(t, plan, corpusPlan, audit, release, item, sourceByID, receipt)
		packet, mapping, buildErr := BuildBlindedPacketV3(
			plan, corpusPlan, audit, release, material, fmt.Sprintf("journal-key-%d", index+1), bytes.Repeat([]byte{byte(0x71 + index)}, 32),
		)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		packets[index], mappings[index] = packet, mapping
	}
	qualification, _, err := DefaultQualification(plan, "journal-qualification", bytes.Repeat([]byte{0x51}, 32))
	if err != nil {
		t.Fatal(err)
	}
	handbook, err := DefaultReviewerHandbook(plan, qualification)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := BuildReviewBundle(plan, pilot.Digest, ReviewDataDevelopmentPilot, packets, mappings, qualification, handbook, "2026-08-10T08:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := BuildRelationPilotReadiness(plan, reviewPilot, bundle, mappings, qualification, handbook, "2026-08-10T08:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	binding := PilotInspectionPackageBinding{
		PackageFormat: PilotPackageFormatV5, PackageInventoryDigest: digestText("inventory"),
		ReadinessSHA256: digestText("readiness-file"), BundleSHA256: digestText("bundle-file"),
		MappingsSHA256: digestText("mappings-file"), WorkbookSHA256: digestText("workbook"),
		ChangeAtlasSHA256: digestText("atlas"), ScarcitySentinelSHA256: digestText("scarcity-sentinel"),
		ScarcityAppendixSHA256: digestText("scarcity"),
	}
	session, err := BuildPilotInspectionSession(readiness, bundle, mappings, governedPlan, primary, sentinel, pilot, scarcityMaterials, binding, "owner-journal-fixture", "2026-08-10T08:00:02Z")
	if err != nil {
		t.Fatal(err)
	}
	return pilotInspectionJournalFixture{
		session: session, readiness: readiness, bundle: bundle, mappings: mappings,
		plan: governedPlan, primary: primary, sentinel: sentinel, pilot: pilot, scarcityMaterials: scarcityMaterials, binding: binding,
	}
}

func governedV3CasesAndSources(release mutation.CorpusReleaseV3) (map[string]mutation.CorpusCaseV3, map[string]mutation.CorpusSource) {
	cases := make(map[string]mutation.CorpusCaseV3, len(release.Cases))
	sources := make(map[string]mutation.CorpusSource, len(release.Sources))
	for _, item := range release.Cases {
		cases[item.ID] = item
	}
	for _, source := range release.Sources {
		sources[source.ID] = source
	}
	return cases, sources
}

func mustJournalTime(t *testing.T, start string) func() string {
	t.Helper()
	current, err := time.Parse(time.RFC3339, start)
	if err != nil {
		t.Fatal(err)
	}
	return func() string {
		current = current.Add(time.Second)
		return current.Format(time.RFC3339)
	}
}
