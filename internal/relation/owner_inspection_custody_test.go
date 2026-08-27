package relation

import (
	"slices"
	"testing"
)

func TestOwnerInspectionCustodyGateRequiresPassedPrivateChainAndPreservesClaimBoundary(t *testing.T) {
	chain := passedOwnerInspectionPrivateChain(t)
	attestation, err := BuildOwnerInspectionPublicAttestation(chain)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := BuildOwnerInspectionCustodyGate(attestation, chain)
	if err != nil {
		t.Fatal(err)
	}
	if gate.RequiredAssessments != 66 || gate.CompletedAssessments != 66 || gate.Dimensions != 16 ||
		gate.OverallStatus != PilotInspectionOverallPassed || !gate.PrivateChainVerified || !gate.ClaimBoundaryVerified ||
		gate.FormalHumanLedgerPresent || gate.PrimaryAdmissionAuthorized || gate.ExecutionAuthorized ||
		gate.ProviderCalls != 0 || gate.NetworkRequired || !gate.ContainsPrivateEvidenceIdentities {
		t.Fatalf("owner custody gate boundary is invalid: %+v", gate)
	}
	if err := gate.Validate(attestation, chain); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerInspectionCustodyGateRejectsRevisionPromotion(t *testing.T) {
	fixture := newPilotInspectionJournalFixture(t)
	revisionChain := ownerInspectionPublicChainFixture(t, fixture)
	revisionAttestation, err := BuildOwnerInspectionPublicAttestation(revisionChain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOwnerInspectionCustodyGate(revisionAttestation, revisionChain); err == nil {
		t.Fatal("revision-required owner projection entered the custody gate")
	}
	promotedStatus := revisionAttestation
	promotedStatus.Outcomes.ScarcityCases = OwnerInspectionPublicDispositionCounts{Accepted: promotedStatus.Assessments.ScarcityCaseCount}
	promotedStatus.Outcomes.ScarcityStatus = PilotInspectionOverallPassed
	promotedStatus.Outcomes.OverallStatus = PilotInspectionOverallPassed
	promotedStatus, err = SealOwnerInspectionPublicAttestation(promotedStatus)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOwnerInspectionCustodyGate(promotedStatus, revisionChain); err == nil {
		t.Fatal("resealed status promotion over private evidence entered the custody gate")
	}
}

func TestOwnerInspectionCustodyGateRejectsClaimPromotion(t *testing.T) {
	passedChain := passedOwnerInspectionPrivateChain(t)
	passed, err := BuildOwnerInspectionPublicAttestation(passedChain)
	if err != nil {
		t.Fatal(err)
	}
	promoted := passed
	index := slices.IndexFunc(promoted.Claims, func(claim OwnerInspectionPublicClaim) bool {
		return claim.ID == "formal_human_study"
	})
	if index < 0 {
		t.Fatal("formal-human claim is missing")
	}
	promoted.Claims[index].Status = OwnerInspectionPublicClaimSupported
	promoted.Digest = ""
	promoted.Digest, err = promoted.digest()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOwnerInspectionCustodyGate(promoted, passedChain); err == nil {
		t.Fatal("claim-promoted owner projection entered the custody gate")
	}
}

func TestOwnerInspectionCustodyGateRejectsPrivateChainDriftAndArtifactTampering(t *testing.T) {
	chain := passedOwnerInspectionPrivateChain(t)
	attestation, err := BuildOwnerInspectionPublicAttestation(chain)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := BuildOwnerInspectionCustodyGate(attestation, chain)
	if err != nil {
		t.Fatal(err)
	}
	tamperedChain := chain
	tamperedChain.PackageBinding.PackageInventoryDigest = digestText("foreign-package")
	if _, err := BuildOwnerInspectionCustodyGate(attestation, tamperedChain); err == nil {
		t.Fatal("private-chain drift entered the custody gate")
	}
	tamperedGate := gate
	tamperedGate.PrimaryAdmissionAuthorized = true
	tamperedGate.Digest, err = ownerInspectionCustodyGateDigest(tamperedGate)
	if err != nil {
		t.Fatal(err)
	}
	if err := tamperedGate.Validate(attestation, chain); err == nil {
		t.Fatal("digest-recomputed primary authorization entered the custody gate")
	}
}

func passedOwnerInspectionPrivateChain(t *testing.T) OwnerInspectionPrivateChain {
	t.Helper()
	fixture := newPilotInspectionJournalFixture(t)
	events := make([]PilotInspectionEvent, 0, PilotInspectionRequiredAssessments)
	nextTime := mustJournalTime(t, fixture.session.CreatedAt)
	for _, packet := range fixture.session.Packets {
		for _, dimension := range fixture.session.CoreDimensions {
			if !coreDimensionApplies(packet, dimension) {
				continue
			}
			events = append(events, passedOwnerInspectionEvent(t, fixture.session, events,
				PilotInspectionSubjectCorePacket, packet.PacketID, dimension, nextTime()))
		}
	}
	for _, item := range fixture.session.ScarcityCases {
		for _, dimension := range fixture.session.ScarcityCaseDimensions {
			events = append(events, passedOwnerInspectionEvent(t, fixture.session, events,
				PilotInspectionSubjectScarcityCase, item.CaseID, dimension, nextTime()))
		}
	}
	for _, dimension := range fixture.session.ScarcityBoundaryDimensions {
		events = append(events, passedOwnerInspectionEvent(t, fixture.session, events,
			PilotInspectionSubjectScarcityBoundary, PilotInspectionScarcityBoundaryID, dimension, nextTime()))
	}
	record, completion, err := BuildPilotInspectionCompletion(
		fixture.session, events, fixture.readiness, fixture.bundle, fixture.mappings, nextTime(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return OwnerInspectionPrivateChain{
		Completion: completion, Record: record, Session: fixture.session, Events: events,
		Readiness: fixture.readiness, Bundle: fixture.bundle, Mappings: fixture.mappings,
		Plan: fixture.plan, Primary: fixture.primary, Sentinel: fixture.sentinel, Pilot: fixture.pilot,
		ScarcityMaterials: fixture.scarcityMaterials, PackageBinding: fixture.binding,
	}
}

func passedOwnerInspectionEvent(
	t *testing.T,
	session PilotInspectionSession,
	events []PilotInspectionEvent,
	subject PilotInspectionSubjectKind,
	subjectID string,
	dimension PilotInspectionDimension,
	observedAt string,
) PilotInspectionEvent {
	t.Helper()
	event, err := BuildPilotInspectionEvent(
		session, events, subject, subjectID, dimension, PilotInspectionPassed, observedAt, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	return event
}
