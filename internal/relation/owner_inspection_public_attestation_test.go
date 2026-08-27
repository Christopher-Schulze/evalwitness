package relation

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestOwnerInspectionPublicAttestationProjectsExactResultWithoutPrivateIdentities(t *testing.T) {
	fixture := newPilotInspectionJournalFixture(t)
	chain := ownerInspectionPublicChainFixture(t, fixture)
	attestation, err := BuildOwnerInspectionPublicAttestation(chain)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.Assessments.Required != 66 || attestation.Assessments.JournalEvents != 66 || attestation.Assessments.Corrections != 0 ||
		attestation.Outcomes.Core != (OwnerInspectionPublicDispositionCounts{Accepted: 7}) ||
		attestation.Outcomes.ScarcityCases != (OwnerInspectionPublicDispositionCounts{Accepted: 1, RevisionRequired: 2}) ||
		attestation.Outcomes.CoreStatus != PilotInspectionOverallPassed || attestation.Outcomes.ScarcityStatus != PilotInspectionOverallRevisionRequired ||
		attestation.Outcomes.OverallStatus != PilotInspectionOverallRevisionRequired {
		t.Fatalf("unexpected public owner-inspection projection: %#v", attestation)
	}
	targetOmission := ownerInspectionPublicDimensionByID(t, attestation.Dimensions, PilotInspectionDimensionScarcityTargetOmission)
	if targetOmission.Applicable != 3 || targetOmission.Passed != 1 || targetOmission.Failed != 2 || targetOmission.Indeterminate != 0 {
		t.Fatalf("unexpected scarcity target-omission aggregate: %#v", targetOmission)
	}
	encoded, err := EncodeIndented(attestation)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		fixture.session.Digest, fixture.session.InspectorAlias, chain.Events[0].Digest,
		chain.Record.Digest, chain.Completion.Digest, fixture.session.Packets[0].PacketID,
		fixture.session.Packets[0].MappingDigest, fixture.session.ScarcityCases[0].CaseID,
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("public owner-inspection attestation disclosed private identity %q", forbidden)
		}
	}
	if err := VerifyOwnerInspectionPublicAttestation(attestation, chain); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerInspectionPublicAttestationCodecSchemaAndPrivateParentVerification(t *testing.T) {
	fixture := newPilotInspectionJournalFixture(t)
	chain := ownerInspectionPublicChainFixture(t, fixture)
	attestation, err := BuildOwnerInspectionPublicAttestation(chain)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(attestation)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeOwnerInspectionPublicAttestation(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != attestation.Digest {
		t.Fatalf("owner-inspection public attestation round trip failed: %v", err)
	}
	schema, err := Schema("owner-inspection-public-attestation")
	if err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	claims := properties["claims"].(JSONSchema)
	claimProperties := claims["items"].(JSONSchema)["properties"].(map[string]any)
	statuses := claimProperties["status"].(JSONSchema)["enum"].([]string)
	if len(statuses) != 6 || !slices.Contains(statuses, string(OwnerInspectionPublicClaimNotPubliclyReproducible)) {
		t.Fatal("public owner-inspection schema does not close claim status vocabulary")
	}

	tampered := attestation
	tampered.Outcomes.ScarcityCases = OwnerInspectionPublicDispositionCounts{Accepted: 2, RevisionRequired: 1}
	tampered, err = SealOwnerInspectionPublicAttestation(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyOwnerInspectionPublicAttestation(tampered, chain); err == nil {
		t.Fatal("resealed public attestation accepted a result that differs from the private projection")
	}
	promoted := attestation
	promoted.Outcomes.ScarcityCases = OwnerInspectionPublicDispositionCounts{Accepted: 3}
	promoted.Outcomes.ScarcityStatus = PilotInspectionOverallPassed
	promoted.Outcomes.OverallStatus = PilotInspectionOverallPassed
	promoted, sealErr := SealOwnerInspectionPublicAttestation(promoted)
	if sealErr == nil && VerifyOwnerInspectionPublicAttestation(promoted, chain) == nil {
		t.Fatal("public attestation accepted a resealed status promotion over revision-required private evidence")
	}
	falseReproduction := attestation
	falseReproduction.Disclosure.SourceReproduction = "complete_public_source_reproduction"
	falseReproduction.Digest = ""
	falseReproduction.Digest, err = falseReproduction.digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := falseReproduction.Validate(); err == nil {
		t.Fatal("public attestation accepted a false independent-reproduction projection")
	}
	invalidChain := chain
	invalidChain.PackageBinding.PackageInventoryDigest = digestText("different-package")
	if _, err := BuildOwnerInspectionPublicAttestation(invalidChain); err == nil {
		t.Fatal("public attestation accepted a private chain with a changed package binding")
	}
	incompleteChain := chain
	incompleteChain.Events = incompleteChain.Events[:len(incompleteChain.Events)-1]
	if _, err := BuildOwnerInspectionPublicAttestation(incompleteChain); err == nil {
		t.Fatal("public attestation accepted an incomplete private event chain")
	}
	unknown := strings.Replace(string(encoded), `"digest": "`, `"unknown": true, "digest": "`, 1)
	if _, err := DecodeOwnerInspectionPublicAttestation(strings.NewReader(unknown)); err == nil {
		t.Fatal("owner-inspection public attestation accepted an unknown field")
	}
	var generic map[string]any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatal(err)
	}
	generic["package_inventory_digest"] = digestText("different-package")
	raw, err := json.Marshal(generic)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeOwnerInspectionPublicAttestation(strings.NewReader(string(raw))); err == nil {
		t.Fatal("owner-inspection public attestation accepted a changed package commitment")
	}
}

func ownerInspectionPublicChainFixture(t *testing.T, fixture pilotInspectionJournalFixture) OwnerInspectionPrivateChain {
	t.Helper()
	events := make([]PilotInspectionEvent, 0, PilotInspectionRequiredAssessments)
	nextTime := mustJournalTime(t, fixture.session.CreatedAt)
	for _, packet := range fixture.session.Packets {
		for _, dimension := range fixture.session.CoreDimensions {
			if !coreDimensionApplies(packet, dimension) {
				continue
			}
			event, err := BuildPilotInspectionEvent(
				fixture.session, events, PilotInspectionSubjectCorePacket, packet.PacketID, dimension, PilotInspectionPassed, nextTime(), false,
			)
			if err != nil {
				t.Fatal(err)
			}
			events = append(events, event)
		}
	}
	for caseIndex, item := range fixture.session.ScarcityCases {
		for _, dimension := range fixture.session.ScarcityCaseDimensions {
			assessment := PilotInspectionPassed
			if caseIndex < 2 && dimension == PilotInspectionDimensionScarcityTargetOmission {
				assessment = PilotInspectionFailed
			}
			event, err := BuildPilotInspectionEvent(
				fixture.session, events, PilotInspectionSubjectScarcityCase, item.CaseID, dimension, assessment, nextTime(), false,
			)
			if err != nil {
				t.Fatal(err)
			}
			events = append(events, event)
		}
	}
	for _, dimension := range fixture.session.ScarcityBoundaryDimensions {
		event, err := BuildPilotInspectionEvent(
			fixture.session, events, PilotInspectionSubjectScarcityBoundary, PilotInspectionScarcityBoundaryID, dimension, PilotInspectionPassed, nextTime(), false,
		)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
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

func ownerInspectionPublicDimensionByID(t *testing.T, values []OwnerInspectionPublicDimensionCounts, dimension PilotInspectionDimension) OwnerInspectionPublicDimensionCounts {
	t.Helper()
	index := slices.IndexFunc(values, func(value OwnerInspectionPublicDimensionCounts) bool {
		return value.Dimension == dimension
	})
	if index < 0 {
		t.Fatalf("public owner-inspection dimensions omit %q", dimension)
	}
	return values[index]
}
