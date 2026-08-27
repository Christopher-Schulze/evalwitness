package stress

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type heldOutLiveTestProvider struct {
	status          provider.ReplayStatus
	providerRequest string
	strict          bool
	replaySource    *provider.ExactReplaySource
}

func (*heldOutLiveTestProvider) Name() string { return "held-out-provider" }

func (*heldOutLiveTestProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{Logprobs: true, TopLogprobsMax: verifier.MinimumVerifierTopK, MaxConcurrent: 2}
}

func (fixture *heldOutLiveTestProvider) ExactReplaySource() (provider.ExactReplaySource, bool) {
	if fixture.status != provider.ReplayStatusExact || fixture.replaySource == nil {
		return provider.ExactReplaySource{}, false
	}
	return *fixture.replaySource, true
}

func (fixture *heldOutLiveTestProvider) Score(_ context.Context, request provider.RequestEnvelope) (provider.ResponseRecord, error) {
	if request.BeforeAttempt != nil {
		if err := request.BeforeAttempt(); err != nil {
			return provider.ResponseRecord{}, err
		}
	}
	var raw strings.Builder
	for index, tag := range request.ScoreTags {
		letter := "M"
		if fixture.strict {
			letter = "A"
			if index == 0 {
				letter = "T"
			}
		}
		raw.WriteString(tag)
		raw.WriteString(letter)
		raw.WriteString("</")
		raw.WriteString(strings.TrimPrefix(tag, "<"))
		raw.WriteByte('\n')
	}
	providerRequestID := fixture.providerRequest
	if providerRequestID == "" {
		fingerprint, err := request.Fingerprint()
		if err != nil {
			return provider.ResponseRecord{}, err
		}
		providerRequestID = "held-out-request-" + string(fingerprint)
	}
	response := provider.ResponseRecord{
		RawText: raw.String(), NormalizedBody: []byte(raw.String()), ReplayStatus: fixture.status,
		ProviderRequestID: providerRequestID, ServedModel: "fixture-live-model", ReceivedAt: 1,
	}
	if fixture.strict {
		response.HasLogprobs = true
		response.ObservedTopLogprobs = verifier.MinimumVerifierTopK
		for index, tag := range request.ScoreTags {
			letter := "A"
			if index == 0 {
				letter = "T"
			}
			response.OrderedTokenEvidence = append(response.OrderedTokenEvidence,
				provider.TokenEvidence{Position: len(response.OrderedTokenEvidence), Token: tag, Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
				provider.TokenEvidence{Position: len(response.OrderedTokenEvidence) + 1, Token: letter, Logprob: probabilityLogString(0.6), TopAlternatives: strictAlternatives(letter)},
				provider.TokenEvidence{Position: len(response.OrderedTokenEvidence) + 2, Token: "</" + strings.TrimPrefix(tag, "<"), Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			)
		}
	}
	finalized, err := provider.FinalizeResponse(request, response)
	if request.AfterAttempt != nil {
		err = errors.Join(err, request.AfterAttempt(provider.AttemptResult{}))
	}
	return finalized, err
}

func TestHeldOutExecutionLedgerClosesAdmissionFilteredDenominatorsByEvidenceAuthority(t *testing.T) {
	serveStressProtocolAdapterHelper()
	fixture := heldOutPreflightFixtureForTest(t, relationevidence.PilotInspectionOverallPassed)
	_, lock, design, armPlan, registry, replayed, _ := currentHeldOutReadinessRefusal(t)
	campaign, err := BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	preflight, _, err := BuildHeldOutPreflightCapsule(
		context.Background(), fixture.privateRelation, fixture.admission, fixture.execution, fixture.evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	issuedAt := fixture.verifiedAt.Add(time.Minute)
	reservedAt := issuedAt.Add(time.Minute)
	store := heldOutReservationStoreForTest(t, reservedAt)
	authorizations := append([]string(nil), fixture.execution.RequiredAuthorizationDigests...)
	permit, err := BuildHeldOutExecutionPermit(
		context.Background(), preflight, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, authorizations, store.Authority(), issuedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := store.Reserve(
		context.Background(), permit, preflight, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, authorizations,
	)
	if err != nil {
		t.Fatal(err)
	}
	proof, _, _, _ := protocolAdapterProofFixture(t)
	replayEvidence, zeroCostEvidence, liveEvidence := executeAdmissionFilteredHeldOutFixture(
		t, armPlan, registry, replayed, fixture.admission, fixture.execution, permit, reservation, proof, reservedAt.Add(time.Second),
	)
	liveReplayVerifications := make([]HeldOutLiveReplayVerification, len(liveEvidence))
	for index, live := range liveEvidence {
		liveReplayVerifications[index], err = BuildHeldOutLiveReplayVerification(
			armPlan, registry, replayed, fixture.admission, live, replayEvidence,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	armReport, err := BuildArmComparisonReport(armPlan, registry, replayed, replayEvidence, zeroCostEvidence, &proof)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := BuildStressAnalysisReport(
		design, armPlan, armReport, registry, replayed, replayEvidence, zeroCostEvidence, &proof, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := BuildHeldOutExecutionLedger(
		lock, campaign, fixture.admission, fixture.execution, permit, reservation, liveEvidence, liveReplayVerifications, armReport, analysis,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.TestCells != 1140 || ledger.StructurallySupportedCells != 440 || ledger.StructuralUnsupportedCells != 700 ||
		ledger.ExecutionEligibleCells != 276 || ledger.ExecutedCells != 276 || ledger.PreExecutionIneligibleCells != 164 ||
		ledger.ExecutedEvidenceUnits != 276 || ledger.ProviderCalls <= 0 || !ledger.LiveResponseEvidenceObserved || len(ledger.Arms) != 10 {
		t.Fatalf("held-out execution ledger = %+v", ledger)
	}
	for _, arm := range ledger.Arms {
		switch arm.EvidenceAuthority {
		case HeldOutEvidenceLiveProviderReservation:
			if arm.PermitDigest != permit.Digest || arm.ReservationDigest != reservation.Digest || arm.ProviderCalls <= 0 ||
				!validDigest(arm.LiveBatchEvidenceDigest) || !validDigest(arm.LiveReplayVerificationDigest) || !validDigest(arm.ReplayEvidenceSetDigest) {
				t.Fatalf("live held-out arm ledger = %+v", arm)
			}
		case HeldOutEvidenceSealedProviderReplay:
			if arm.ReplaySourceArmID != heldOutReplaySourceArmID || !validDigest(arm.ReplaySourceCellSetDigest) || arm.ProviderCalls != 0 {
				t.Fatalf("replay held-out arm ledger = %+v", arm)
			}
		case HeldOutEvidenceDeterministicLocal:
			if arm.ProviderCalls != 0 || arm.PermitDigest != "" || arm.ReservationDigest != "" || arm.ReplaySourceArmID != "" {
				t.Fatalf("local held-out arm ledger = %+v", arm)
			}
		default:
			t.Fatalf("unknown held-out execution authority %q", arm.EvidenceAuthority)
		}
	}
	encoded, err := EncodeIndented(ledger)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeHeldOutExecutionLedger(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, ledger) {
		t.Fatal("held-out execution ledger changed across strict JSON decoding")
	}
	liveReplay := liveReplayVerifications[0]
	if liveReplay.ExactReplayCells != liveEvidence[0].EligibleCells ||
		liveReplay.ResponseEvidenceMatches != liveEvidence[0].ProviderCalls {
		t.Fatalf("live-to-exact replay verification = %+v", liveReplay)
	}
	sealStore, err := NewHeldOutRunSealStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	sealParents := HeldOutRunSealV2Parents{
		Lock: lock, Campaign: campaign, Admission: fixture.admission, Execution: fixture.execution,
		Permit: permit, Reservation: reservation, LiveEvidence: liveEvidence,
		LiveReplayVerifications: liveReplayVerifications, ExecutionLedger: ledger,
		Design: design, Plan: armPlan, ArmReport: armReport, Analysis: analysis,
		Registry: registry, Replayed: replayed, ReplayEvidence: replayEvidence,
		ZeroCostEvidence: zeroCostEvidence, ProtocolProof: &proof,
	}
	var seal HeldOutRunSealV2
	var sealMu sync.Mutex
	sealErrors := make(chan error, 2)
	var sealGroup sync.WaitGroup
	for range 2 {
		sealGroup.Add(1)
		go func() {
			defer sealGroup.Done()
			candidate, sealErr := sealStore.Seal(sealParents)
			if sealErr == nil {
				sealMu.Lock()
				seal = candidate
				sealMu.Unlock()
			}
			sealErrors <- sealErr
		}()
	}
	sealGroup.Wait()
	close(sealErrors)
	sealSuccesses, sealRefusals := 0, 0
	for sealErr := range sealErrors {
		if sealErr == nil {
			sealSuccesses++
			continue
		}
		if containsError(sealErr, "already sealed") {
			sealRefusals++
			continue
		}
		t.Fatalf("concurrent held-out run seal v2 error = %v", sealErr)
	}
	if sealSuccesses != 1 || sealRefusals != 1 {
		t.Fatalf("concurrent held-out run seal v2 successes=%d refusals=%d", sealSuccesses, sealRefusals)
	}
	if seal.RunCount != 1 || seal.Reopened || !seal.Completed || seal.TestCells != 1140 ||
		seal.StructurallySupportedCells != 440 || seal.StructuralUnsupportedCells != 700 ||
		seal.ExecutionEligibleCells != 276 || seal.ExecutedCells != 276 || seal.PreExecutionIneligibleCells != 164 ||
		seal.ExecutedEvidenceUnits != 276 || seal.ProviderCalls != ledger.ProviderCalls || !seal.LiveResponseEvidenceObserved ||
		seal.ReplayCaptureSources != 2 || seal.AnalysisCompletionStatus != heldOutRunSealV2AnalysisState ||
		seal.ConfirmatoryInferenceComplete || seal.PopulationGeneralization ||
		seal.WitnessesRequired != seal.ViolatedExecutedCells || seal.WitnessesBound+seal.WitnessesMissing != seal.WitnessesRequired {
		t.Fatalf("held-out run seal v2 = %+v", seal)
	}
	loadedSeal, err := sealStore.Load(lock)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loadedSeal, seal) {
		t.Fatal("held-out run seal v2 changed after owner-only store reload")
	}
	encodedSeal, err := EncodeIndented(seal)
	if err != nil {
		t.Fatal(err)
	}
	decodedSeal, err := DecodeHeldOutRunSealV2(bytes.NewReader(encodedSeal))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedSeal, seal) {
		t.Fatal("held-out run seal v2 changed across strict JSON decoding")
	}
	unknownSeal := bytes.Replace(encodedSeal, []byte("{"), []byte(`{"unknown":true,`), 1)
	if _, err := DecodeHeldOutRunSealV2(bytes.NewReader(unknownSeal)); err == nil {
		t.Fatal("held-out run seal v2 accepted an unknown JSON field")
	}
	if _, err := DecodeHeldOutRunSealV2(bytes.NewReader(append(encodedSeal, []byte("\n{}")...))); err == nil {
		t.Fatal("held-out run seal v2 accepted trailing JSON")
	}
	violated := heldOutViolatedCell(t, lock, armReport)
	source := relationByID(t, registry, violated.RelationID)
	candidate := source
	candidate.ID = source.ID + ".admission-filtered-next-catalog"
	candidate.Revision++
	candidate.Digest = ""
	candidate, err = SealRelation(candidate)
	if err != nil {
		t.Fatal(err)
	}
	discoveryLedger, err := RouteHeldOutDiscoveriesV2(seal, sealParents, []RelationDiscovery{{
		SourceCellID: violated.CellID, DiscoveryEvidenceDigest: digestText("admission-filtered-discovery-evidence"), Candidate: candidate,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := discoveryLedger.ValidateAgainstV2(seal, sealParents); err != nil {
		t.Fatal(err)
	}
	if discoveryLedger.DiscoveryCount != 1 || discoveryLedger.HeldOutRunSealDigest != seal.Digest ||
		discoveryLedger.TargetCatalogVersion != lock.NextCatalogVersion || !discoveryLedger.CurrentCatalogFrozen ||
		discoveryLedger.TestPartitionRetrofitted || discoveryLedger.Discoveries[0].SourceResultDigest != violated.ResultDigest {
		t.Fatalf("admission-filtered next-version discovery ledger = %+v", discoveryLedger)
	}

	t.Run("rejects second seal for the same frozen partition", func(t *testing.T) {
		if _, err := sealStore.Seal(sealParents); err == nil || !containsError(err, "already sealed") {
			t.Fatalf("second held-out run seal v2 error = %v", err)
		}
	})

	t.Run("rejects discovery-ledger seal substitution", func(t *testing.T) {
		tampered := discoveryLedger
		tampered.HeldOutRunSealDigest = digestText("foreign-admission-filtered-seal")
		tampered.Digest, err = nextVersionDiscoveryLedgerDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if err := tampered.ValidateAgainstV2(seal, sealParents); err == nil {
			t.Fatal("next-version discovery ledger accepted a substituted admission-filtered run seal")
		}
	})

	t.Run("rejects admission-ineligible execution promotion", func(t *testing.T) {
		tampered := seal
		tampered.ExecutedCells++
		tampered.ExecutionEligibleCells++
		tampered.ExecutedEvidenceUnits++
		tampered.Digest, err = heldOutRunSealV2Digest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if err := tampered.ValidateAgainst(sealParents); err == nil {
			t.Fatal("held-out run seal v2 accepted execution promotion beyond the admission-eligible denominator")
		}
	})

	t.Run("rejects complete-inference promotion", func(t *testing.T) {
		tampered := seal
		tampered.ConfirmatoryInferenceComplete = true
		tampered.Digest, err = heldOutRunSealV2Digest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if err := tampered.Validate(); err == nil {
			t.Fatal("held-out run seal v2 promoted filtered evidence to complete confirmatory inference")
		}
	})

	t.Run("rejects execution-ledger substitution", func(t *testing.T) {
		tampered := sealParents
		tampered.ExecutionLedger = ledger
		tampered.ExecutionLedger.Digest = digestText("foreign-execution-ledger")
		if err := seal.ValidateAgainst(tampered); err == nil {
			t.Fatal("held-out run seal v2 accepted a substituted execution ledger")
		}
	})

	t.Run("rejects replay-verification substitution", func(t *testing.T) {
		tampered := sealParents
		tampered.LiveReplayVerifications = append([]HeldOutLiveReplayVerification(nil), liveReplayVerifications...)
		tampered.LiveReplayVerifications[0].Digest = digestText("foreign-live-replay-verification")
		if err := seal.ValidateAgainst(tampered); err == nil {
			t.Fatal("held-out run seal v2 accepted a substituted exact-replay verification")
		}
	})

	t.Run("rejects a different owner-only seal authority", func(t *testing.T) {
		foreignReservationStore := heldOutReservationStoreForTest(t, reservedAt)
		foreignSealStore, err := NewHeldOutRunSealStore(foreignReservationStore.root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := foreignSealStore.Seal(sealParents); err == nil {
			t.Fatal("held-out run seal v2 accepted a store outside the permit-bound authority")
		}
	})

	t.Run("rejects replay status masquerading as live", func(t *testing.T) {
		binding, found := heldOutExecutionArmBindingByID(fixture.execution.Arms, liveEvidence[0].ArmID)
		if !found {
			t.Fatalf("live evidence references unknown arm %q", liveEvidence[0].ArmID)
		}
		tampered := liveEvidence[0]
		tampered.Cells = append([]HeldOutLiveCellEvidence(nil), liveEvidence[0].Cells...)
		tampered.Cells[0].Original.Observations = append([]mode.ScoreObservation(nil), tampered.Cells[0].Original.Observations...)
		tampered.Cells[0].Original.Observations[0].ReplayStatus = provider.ReplayStatusExact
		tampered.Digest = ""
		if _, err := BuildHeldOutLiveBatchEvidence(
			binding, permit, reservation, liveBatchResultFromEvidence(tampered, binding.Batch), liveObservationsFromEvidence(tampered), reservedAt.Add(time.Second),
		); err == nil {
			t.Fatal("held-out live evidence accepted exact replay as live execution")
		}
	})

	t.Run("rejects missing provider request identity", func(t *testing.T) {
		binding, found := heldOutExecutionArmBindingByID(fixture.execution.Arms, liveEvidence[0].ArmID)
		if !found {
			t.Fatalf("live evidence references unknown arm %q", liveEvidence[0].ArmID)
		}
		tampered := liveEvidence[0]
		tampered.Cells = append([]HeldOutLiveCellEvidence(nil), liveEvidence[0].Cells...)
		tampered.Cells[0].Original.Observations = append([]mode.ScoreObservation(nil), tampered.Cells[0].Original.Observations...)
		tampered.Cells[0].Original.Observations[0].ProviderRequestID = ""
		if _, err := BuildHeldOutLiveBatchEvidence(
			binding, permit, reservation, liveBatchResultFromEvidence(tampered, binding.Batch), liveObservationsFromEvidence(tampered), reservedAt.Add(time.Second),
		); err == nil {
			t.Fatal("held-out live evidence accepted a live response without provider request identity")
		}
	})

	t.Run("rejects missing independent exact replay verification", func(t *testing.T) {
		if _, err := BuildHeldOutExecutionLedger(
			lock, campaign, fixture.admission, fixture.execution, permit, reservation, liveEvidence, liveReplayVerifications[:1], armReport, analysis,
		); err == nil {
			t.Fatal("held-out execution ledger accepted live evidence without one independent exact replay verification per live arm")
		}
	})

	t.Run("rejects fabricated exact replay response equality", func(t *testing.T) {
		tamperedReplay := append([]ArmReplayEvidence(nil), replayEvidence...)
		for index := range tamperedReplay {
			if tamperedReplay[index].ArmID != liveEvidence[0].ArmID {
				continue
			}
			tamperedReplay[index].Execution.Original.Observations = append(
				[]mode.ScoreObservation(nil), tamperedReplay[index].Execution.Original.Observations...,
			)
			tamperedReplay[index].Execution.Original.Observations[0].ResponseEvidenceDigest = digestText("foreign-replay-response")
			break
		}
		if _, err := BuildHeldOutLiveReplayVerification(
			armPlan, registry, replayed, fixture.admission, liveEvidence[0], tamperedReplay,
		); err == nil {
			t.Fatal("held-out live replay verification accepted a substituted exact replay response")
		}
	})

	t.Run("rejects exact replay capture from another live route", func(t *testing.T) {
		tampered := append([]HeldOutLiveReplayVerification(nil), liveReplayVerifications...)
		foreign, sourceErr := stressExactReplaySource("foreign-provider")
		if sourceErr != nil {
			t.Fatal(sourceErr)
		}
		tampered[0].ReplaySource = foreign
		tampered[0].Digest, err = heldOutLiveReplayVerificationDigest(tampered[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildHeldOutExecutionLedger(
			lock, campaign, fixture.admission, fixture.execution, permit, reservation, liveEvidence, tampered, armReport, analysis,
		); err == nil {
			t.Fatal("held-out execution ledger accepted an exact replay capture from another live route")
		}
	})

	t.Run("rejects exact replay capture smaller than the reproduced call set", func(t *testing.T) {
		tampered := liveReplayVerifications[0]
		tampered.ReplaySource.Bytes = 1
		tampered.ReplaySource.Records = 1
		tampered.Digest, err = heldOutLiveReplayVerificationDigest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if err := tampered.Validate(); err == nil {
			t.Fatal("held-out live replay verification accepted fewer capture records than reproduced calls")
		}
	})

	t.Run("rejects executed ineligible cell", func(t *testing.T) {
		tampered := armReport
		tampered.Cells = append([]ArmComparisonObservation(nil), armReport.Cells...)
		for index := range tampered.Cells {
			if _, ineligible := stringSet(fixture.admission.PreExecutionIneligibleCellIDs)[tampered.Cells[index].CellID]; ineligible {
				tampered.Cells[index] = firstExecutedHeldOutObservation(t, armReport, tampered.Cells[index].ArmID)
				tampered.Cells[index].CellID = fixture.admission.PreExecutionIneligibleCellIDs[0]
				break
			}
		}
		tampered.Digest = armReport.Digest
		if _, err := BuildHeldOutExecutionLedger(
			lock, campaign, fixture.admission, fixture.execution, permit, reservation, liveEvidence, liveReplayVerifications, tampered, analysis,
		); err == nil {
			t.Fatal("held-out execution ledger accepted execution evidence for a pre-execution-ineligible cell")
		}
	})

	t.Run("rejects missing eligible cell", func(t *testing.T) {
		tampered := armReport
		tampered.Cells = append([]ArmComparisonObservation(nil), armReport.Cells...)
		for index := range tampered.Cells {
			if _, eligible := stringSet(fixture.admission.ExecutionEligibleCellIDs)[tampered.Cells[index].CellID]; eligible {
				tampered.Cells[index] = plannedArmObservation(ArmComparisonCell{
					CellID: tampered.Cells[index].CellID, ArmID: tampered.Cells[index].ArmID,
					RelationID: tampered.Cells[index].RelationID, RelationDigest: tampered.Cells[index].RelationDigest,
					CaseID: tampered.Cells[index].CaseID, TaskGroupID: tampered.Cells[index].TaskGroupID,
					ManifestDigest: tampered.Cells[index].ManifestDigest, Support: ArmSupported,
				})
				break
			}
		}
		tampered.Digest = armReport.Digest
		if _, err := BuildHeldOutExecutionLedger(
			lock, campaign, fixture.admission, fixture.execution, permit, reservation, liveEvidence, liveReplayVerifications, tampered, analysis,
		); err == nil {
			t.Fatal("held-out execution ledger accepted a missing admission-eligible execution")
		}
	})

	t.Run("rejects receipt substitution", func(t *testing.T) {
		foreign := reservation
		foreign.Digest = digestText("foreign-held-out-reservation")
		if _, err := BuildHeldOutExecutionLedger(
			lock, campaign, fixture.admission, fixture.execution, permit, foreign, liveEvidence, liveReplayVerifications, armReport, analysis,
		); err == nil {
			t.Fatal("held-out execution ledger accepted a foreign reservation")
		}
	})
}

func TestExecuteHeldOutLiveBatchUsesExactReservedPreviewWithoutReplanning(t *testing.T) {
	fixture := heldOutPreflightFixtureForTest(t, relationevidence.PilotInspectionOverallPassed)
	_, lock, design, armPlan, registry, replayed, _ := currentHeldOutReadinessRefusal(t)
	campaign, err := BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	preflight, _, err := BuildHeldOutPreflightCapsule(
		context.Background(), fixture.privateRelation, fixture.admission, fixture.execution, fixture.evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	issuedAt := fixture.verifiedAt.Add(time.Minute)
	reservedAt := issuedAt.Add(time.Minute)
	store := heldOutReservationStoreForTest(t, reservedAt)
	authorizations := append([]string(nil), fixture.execution.RequiredAuthorizationDigests...)
	permit, err := BuildHeldOutExecutionPermit(
		context.Background(), preflight, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, authorizations, store.Authority(), issuedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := store.Reserve(
		context.Background(), permit, preflight, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, authorizations,
	)
	if err != nil {
		t.Fatal(err)
	}
	const armID = "score-token-verifier"
	binding, exists := heldOutExecutionArmBindingByID(fixture.execution.Arms, armID)
	if !exists {
		t.Fatalf("missing held-out live binding %q", armID)
	}
	previewCandidate := heldOutEligibleBatchCandidateForCapsule(
		t, campaign, armPlan, replayed, fixture.admission, fixture.execution.StudyManifestDigest,
		"preflight-model", armID, fixture.privateRelation.Manifest.CapsuleID,
	)
	service := heldOutLiveVerificationService(t, binding, true, provider.ReplayStatusLive, "fixture-upstream-request")
	times := []time.Time{reservedAt.Add(time.Second), reservedAt.Add(2 * time.Second)}
	clockIndex := 0
	execution, err := executeHeldOutLiveBatchAt(
		context.Background(), service, binding, previewCandidate.Batch, permit, reservation,
		func() time.Time {
			value := times[clockIndex]
			clockIndex++
			return value
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if execution.Evidence.ArmID != armID || execution.Evidence.ProviderCalls <= 0 ||
		execution.Evidence.LiveResponses != execution.Evidence.ProviderCalls ||
		execution.Evidence.BatchRunFingerprint != previewCandidate.Batch.RunFingerprint ||
		execution.Evidence.BatchPlanBindingDigest != binding.Batch.Digest {
		t.Fatalf("held-out live batch execution = %+v", execution.Evidence)
	}
	if previewCandidate.Batch.AuthorizationDigest != "" {
		t.Fatal("held-out live executor mutated its unapproved caller preview")
	}

	t.Run("rejects changed bound preview before executor invocation", func(t *testing.T) {
		tampered := previewCandidate.Batch
		tampered.RunFingerprint = digestText("foreign-live-preview")
		called := false
		executor := heldOutLiveBatchExecutorFunc(func(context.Context, verification.BatchPlan) (verification.BatchResult, error) {
			called = true
			return verification.BatchResult{}, nil
		})
		if _, err := executeHeldOutLiveBatchAt(
			context.Background(), executor, binding, tampered, permit, reservation,
			func() time.Time { return reservedAt.Add(time.Second) },
		); err == nil || called {
			t.Fatal("held-out live executor accepted a changed preview or invoked the executor before validation")
		}
	})

	t.Run("rejects completion outside permit window", func(t *testing.T) {
		expiresAt, parseErr := parseHeldOutExecutionPermitTime(permit.ExpiresAt)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		executor := heldOutLiveBatchExecutorFunc(func(context.Context, verification.BatchPlan) (verification.BatchResult, error) {
			return execution.Result, nil
		})
		times := []time.Time{reservedAt.Add(time.Second), expiresAt}
		index := 0
		if _, err := executeHeldOutLiveBatchAt(
			context.Background(), executor, binding, previewCandidate.Batch, permit, reservation,
			func() time.Time {
				value := times[index]
				index++
				return value
			},
		); err == nil {
			t.Fatal("held-out live executor accepted a batch that completed after permit expiry")
		}
	})
}

type heldOutLiveBatchExecutorFunc func(context.Context, verification.BatchPlan) (verification.BatchResult, error)

func (function heldOutLiveBatchExecutorFunc) ExecuteBatch(ctx context.Context, batch verification.BatchPlan) (verification.BatchResult, error) {
	return function(ctx, batch)
}

func heldOutLiveVerificationService(
	t *testing.T,
	binding HeldOutExecutionArmBatchBinding,
	strict bool,
	status provider.ReplayStatus,
	providerRequestID string,
) *verification.Service {
	t.Helper()
	service, err := verification.NewService(verification.Config{
		PreprocessBudget: 0,
		RequestProfile: verification.RequestProfile{
			ProviderID: "held-out-provider",
			BaseURL:    "https://held-out.invalid/v1", RequestedModel: "preflight-model",
			ThinkingMode: "disabled", Temperature: 1, MaxOutputTokens: 128, TopLogprobs: verifier.MinimumVerifierTopK,
		},
		BudgetProfile: verification.BudgetProfile{MaxRetries: 1, MaxWorkers: 2, RequestTimeout: time.Second},
		Offline:       status == provider.ReplayStatusExact,
	}, func(_ context.Context, plan verification.Plan) (verification.Runtime, error) {
		var replaySource *provider.ExactReplaySource
		if status == provider.ReplayStatusExact {
			source, sourceErr := stressExactReplaySourceForRoute(
				binding.ArmID, "held-out-provider", "https://held-out.invalid/v1", "preflight-model",
			)
			if sourceErr != nil {
				return verification.Runtime{}, sourceErr
			}
			replaySource = &source
		}
		return verification.Runtime{Runner: &mode.Runner{
			Provider: &heldOutLiveTestProvider{
				status: status, providerRequest: providerRequestID, strict: strict, replaySource: replaySource,
			},
			Budget: mode.NewRunBudget(plan.Input.Limits),
			Cfg: mode.RunnerConfig{
				Model: "preflight-model", BaseURL: "https://held-out.invalid/v1", ThinkingMode: "disabled",
				Entrypoint: plan.Input.Entrypoint, Temperature: 1, MaxTokens: 128,
				TopLogprobs: verifier.MinimumVerifierTopK, MaxWorkers: 2, JudgeMode: !strict,
			},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func liveBatchResultFromEvidence(value HeldOutLiveBatchEvidence, binding verification.BatchPlanBinding) verification.BatchResult {
	result := verification.BatchResult{
		SchemaVersion: verification.BatchSchemaVersion, RunFingerprint: value.BatchRunFingerprint,
		ServedModel: value.ServedModel, Budget: value.Budget, Lifecycle: value.Lifecycle,
		Results: make([]verification.Result, binding.InputCount),
	}
	byFingerprint := make(map[string]verification.Result, binding.InputCount)
	for _, cell := range value.Cells {
		byFingerprint[cell.Original.PlanFingerprint] = cell.Original.Result
		byFingerprint[cell.Transformed.PlanFingerprint] = cell.Transformed.Result
	}
	for index, fingerprint := range binding.PlanFingerprints {
		result.Results[index] = byFingerprint[fingerprint]
	}
	return result
}

func liveObservationsFromEvidence(value HeldOutLiveBatchEvidence) []mode.ScoreObservation {
	var result []mode.ScoreObservation
	for _, cell := range value.Cells {
		result = append(result, cell.Original.Observations...)
		result = append(result, cell.Transformed.Observations...)
	}
	return result
}

func executeAdmissionFilteredHeldOutFixture(
	t *testing.T,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	admission HeldOutAdmissionPlan,
	executionBinding HeldOutExecutionBatchBinding,
	permit HeldOutExecutionPermit,
	reservation HeldOutExecutionReservation,
	proof ProtocolAdapterProof,
	executedAt time.Time,
) ([]ArmReplayEvidence, []ZeroCostExecution, []HeldOutLiveBatchEvidence) {
	t.Helper()
	judgeBinding, judgeExists := heldOutExecutionArmBindingByID(executionBinding.Arms, "explicit-text-judge")
	verifierBinding, verifierExists := heldOutExecutionArmBindingByID(executionBinding.Arms, "score-token-verifier")
	if !judgeExists || !verifierExists {
		t.Fatal("held-out fixture lacks the two provider route bindings")
	}
	judgeRunner, err := NewReplayFirstRunner(heldOutLiveVerificationService(
		t, judgeBinding, false, provider.ReplayStatusExact, "",
	))
	if err != nil {
		t.Fatal(err)
	}
	verifierRunner, err := NewReplayFirstRunner(heldOutLiveVerificationService(
		t, verifierBinding, true, provider.ReplayStatusExact, "",
	))
	if err != nil {
		t.Fatal(err)
	}
	protocolRunner, err := NewReplayFirstRunner(heldOutLiveVerificationService(
		t, verifierBinding, true, provider.ReplayStatusExact, "",
	))
	if err != nil {
		t.Fatal(err)
	}
	runners := map[string]*ReplayFirstRunner{
		"explicit-text-judge": judgeRunner, "score-token-verifier": verifierRunner, "external-protocol-adapter": protocolRunner,
	}
	admissionByCell := make(map[string]ConstructAdmission, len(admission.ExecutionEligibleCellIDs))
	for _, entry := range admission.Entries {
		if !entry.ExecutionEligible {
			continue
		}
		for _, cellID := range entry.StructurallySupportedCellIDs {
			admissionByCell[cellID] = entry.Admission
		}
	}
	var replayEvidence []ArmReplayEvidence
	var zeroCostEvidence []ZeroCostExecution
	var liveEvidence []HeldOutLiveBatchEvidence
	for _, item := range replayed {
		for _, relationID := range item.RelationIDs {
			relation := relationByID(t, registry, relationID)
			for _, arm := range plan.Arms {
				cellID, cellErr := comparisonCellID(arm.ID, relation.ID, item.CaseID)
				if cellErr != nil {
					t.Fatal(cellErr)
				}
				constructAdmission, eligible := admissionByCell[cellID]
				if !eligible {
					continue
				}
				if arm.Kind == ArmZeroCostControl {
					execution, runErr := RunZeroCostArm(relation, constructAdmission, item, arm.ID)
					if runErr != nil {
						t.Fatalf("zero-cost admitted cell %s: %v", cellID, runErr)
					}
					zeroCostEvidence = append(zeroCostEvidence, execution)
					continue
				}
				runner := runners[arm.ID]
				original := corpusVerificationInput(relation, constructAdmission, arm.Entrypoint, item.Original)
				transformed := corpusVerificationInput(relation, constructAdmission, arm.Entrypoint, item.Transformed)
				original.Policy.Evidence = arm.EvidencePolicy
				transformed.Policy.Evidence = arm.EvidencePolicy
				execution, runErr := runner.RunMutation(context.Background(), ReplayRunRequest{
					Relation: relation, Admission: constructAdmission, Original: original, Transformed: transformed,
				})
				if runErr != nil {
					t.Fatalf("provider admitted cell %s: %v", cellID, runErr)
				}
				var requiredProof *ProtocolAdapterProof
				if arm.Kind == ArmProtocolAdapter {
					requiredProof = &proof
				}
				sealed, runErr := SealArmReplayEvidence(plan, relation, constructAdmission, item, execution, requiredProof)
				if runErr != nil {
					t.Fatalf("seal admitted cell %s: %v", cellID, runErr)
				}
				replayEvidence = append(replayEvidence, sealed)
			}
		}
	}
	for _, armID := range []string{"explicit-text-judge", "score-token-verifier"} {
		binding, exists := heldOutExecutionArmBindingByID(executionBinding.Arms, armID)
		if !exists {
			t.Fatalf("missing live binding %q", armID)
		}
		byCell := make(map[string]ArmReplayEvidence, binding.EligibleTestCells)
		for _, evidence := range replayEvidence {
			if evidence.ArmID == armID {
				byCell[evidence.CellID] = evidence
			}
		}
		batchResult := verification.BatchResult{
			SchemaVersion: verification.BatchSchemaVersion, RunFingerprint: binding.Batch.RunFingerprint,
			Results: make([]verification.Result, binding.VerificationInputs), ServedModel: "fixture-live-model",
			Lifecycle: verification.Lifecycle{
				RuntimeOpen: verification.LifecycleComplete, Execution: verification.LifecycleComplete,
				Cleanup: verification.LifecycleComplete, Audit: verification.LifecycleComplete,
			},
		}
		var observations []mode.ScoreObservation
		for index, cellID := range binding.Batch.StudyCellIDs {
			evidence, found := byCell[cellID]
			if !found {
				t.Fatalf("missing live fixture cell %q", cellID)
			}
			side := evidence.Execution.Original
			if binding.Batch.StudyVariants[index] == "transformed" {
				side = evidence.Execution.Transformed
			}
			item := side.Result
			item.RunFingerprint = binding.Batch.PlanFingerprints[index]
			batchResult.Results[index] = item
			usage, err := verificationResultUsage(item)
			if err != nil {
				t.Fatal(err)
			}
			batchResult.Budget.Calls += usage.Calls
			batchResult.Budget.Attempts += usage.Calls
			for _, observation := range side.Observations {
				observation.Scope = item.RunFingerprint
				observation.ReplayStatus = provider.ReplayStatusLive
				observation.ReplaySource = nil
				observations = append(observations, observation)
			}
		}
		sealed, err := BuildHeldOutLiveBatchEvidence(binding, permit, reservation, batchResult, observations, executedAt)
		if err != nil {
			t.Fatal(err)
		}
		liveEvidence = append(liveEvidence, sealed)
	}
	return replayEvidence, zeroCostEvidence, liveEvidence
}

func firstExecutedHeldOutObservation(t *testing.T, report ArmComparisonReport, armID string) ArmComparisonObservation {
	t.Helper()
	for _, cell := range report.Cells {
		if cell.ArmID == armID && cell.Status == ArmCellExecuted {
			return cell
		}
	}
	t.Fatalf("no executed held-out observation for arm %q", armID)
	return ArmComparisonObservation{}
}
