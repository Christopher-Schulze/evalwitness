package stress

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func TestHeldOutPreflightCustodyVerifiesEveryPrerequisiteWithoutExecutionAuthority(t *testing.T) {
	fixture := heldOutPreflightFixtureForTest(t, relationevidence.PilotInspectionOverallPassed)
	value, err := BuildHeldOutPreflightCustody(
		context.Background(), fixture.privateRelation, fixture.admission, fixture.execution, fixture.evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if value.CampaignDigest != fixture.admission.CampaignDigest || value.AdmissionPlanDigest != fixture.admission.Digest ||
		value.ExecutionBatchBindingDigest != fixture.execution.Digest || value.PreflightEvidenceDigest != fixture.evidence.Digest ||
		value.PrivateRelationCapsuleID != fixture.privateRelation.Manifest.CapsuleID || !value.PrivateRelationCapsuleVerified ||
		!value.StudyRecordVerified || !value.ExecutionBindingsVerified || !value.CurrentRoutesAttested ||
		!value.AuthorizationPlansVerified || !value.AdmissionFilteredWorkloadVerified || value.RunAuthorized ||
		value.ExecutionPermitIssued || value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired {
		t.Fatalf("held-out preflight custody = %+v", value)
	}
	if err := value.ValidateAgainst(
		fixture.admission, fixture.execution, fixture.evidence, fixture.privateRelation, fixture.verifiedAt,
	); err != nil {
		t.Fatal(err)
	}
	evidenceJSON, err := EncodeIndented(fixture.evidence)
	if err != nil {
		t.Fatal(err)
	}
	decodedEvidence, err := DecodeHeldOutPreflightEvidence(bytes.NewReader(evidenceJSON))
	if err != nil {
		t.Fatal(err)
	}
	if decodedEvidence.Digest != fixture.evidence.Digest {
		t.Fatalf("decoded preflight evidence digest=%s want %s", decodedEvidence.Digest, fixture.evidence.Digest)
	}
	custodyJSON, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	decodedCustody, err := DecodeHeldOutPreflightCustody(bytes.NewReader(custodyJSON))
	if err != nil {
		t.Fatal(err)
	}
	if decodedCustody.Digest != value.Digest {
		t.Fatalf("decoded preflight custody digest=%s want %s", decodedCustody.Digest, value.Digest)
	}
	for _, document := range []struct {
		name    string
		version string
	}{
		{name: "held-out-preflight-evidence", version: HeldOutPreflightEvidenceSchemaVersion},
		{name: "held-out-preflight-custody", version: HeldOutPreflightCustodySchemaVersion},
	} {
		schema, schemaErr := Schema(document.name)
		if schemaErr != nil {
			t.Fatal(schemaErr)
		}
		properties := schema["properties"].(map[string]any)
		if schema["additionalProperties"] != false || properties["schema_version"].(JSONSchema)["const"] != document.version {
			t.Fatalf("preflight schema %q is open or has the wrong version", document.name)
		}
	}
	preflightCapsule, capsuleCustody, err := BuildHeldOutPreflightCapsule(
		context.Background(), fixture.privateRelation, fixture.admission, fixture.execution, fixture.evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if capsuleCustody.Digest != value.Digest || preflightCapsule.Manifest.ParentCapsules[0].CapsuleID != fixture.privateRelation.Manifest.CapsuleID ||
		len(preflightCapsule.Manifest.Components) != 4 || len(preflightCapsule.Manifest.ScientificRoots) != 1 {
		t.Fatalf("held-out preflight capsule = %+v custody=%+v", preflightCapsule.Manifest, capsuleCustody)
	}
	verifiedCustody, err := VerifyHeldOutPreflightCapsule(
		context.Background(), preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution, fixture.evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if verifiedCustody.Digest != value.Digest {
		t.Fatalf("verified capsule custody digest=%s want %s", verifiedCustody.Digest, value.Digest)
	}
	if _, err := capsule.VerifyPackageFamily(
		context.Background(), preflightCapsule, nil, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPrivate},
	); err == nil {
		t.Fatal("held-out preflight capsule verified without its private owner-custody parent")
	}
	issuedAt := fixture.verifiedAt.Add(time.Minute)
	reservedAt := issuedAt.Add(2 * time.Minute)
	providedAuthorizations := append([]string(nil), fixture.execution.RequiredAuthorizationDigests...)
	reservationStore := heldOutReservationStoreForTest(t, reservedAt)
	reservationAuthority := reservationStore.Authority()
	permit, err := BuildHeldOutExecutionPermit(
		context.Background(), preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, providedAuthorizations, reservationAuthority, issuedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !permit.RunAuthorized || !permit.ExecutionPermitIssued || permit.ExecutionStarted || permit.ProviderCalls != 0 ||
		permit.EmpiricalUnits != 0 || permit.NetworkPerformed || !permit.NetworkRequiredForExecution || len(permit.Arms) != 2 ||
		permit.LiveProviderEligibleCells != fixture.execution.LiveProviderEligibleCells ||
		permit.LiveVerificationInputs != fixture.execution.LiveVerificationInputs {
		t.Fatalf("held-out execution permit = %+v", permit)
	}
	if err := VerifyHeldOutExecutionPermit(
		context.Background(), permit, preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, providedAuthorizations, reservationAuthority, issuedAt.Add(time.Minute),
	); err != nil {
		t.Fatal(err)
	}
	permitJSON, err := EncodeIndented(permit)
	if err != nil {
		t.Fatal(err)
	}
	decodedPermit, err := DecodeHeldOutExecutionPermit(bytes.NewReader(permitJSON))
	if err != nil {
		t.Fatal(err)
	}
	if decodedPermit.Digest != permit.Digest {
		t.Fatalf("decoded execution permit digest=%s want %s", decodedPermit.Digest, permit.Digest)
	}
	permitSchema, err := Schema("held-out-execution-permit")
	if err != nil {
		t.Fatal(err)
	}
	permitProperties := permitSchema["properties"].(map[string]any)
	if permitSchema["additionalProperties"] != false ||
		permitProperties["schema_version"].(JSONSchema)["const"] != HeldOutExecutionPermitSchemaVersion {
		t.Fatal("held-out execution permit schema is open or has the wrong version")
	}
	reservationSchema, err := Schema("held-out-execution-reservation")
	if err != nil {
		t.Fatal(err)
	}
	reservationProperties := reservationSchema["properties"].(map[string]any)
	if reservationSchema["additionalProperties"] != false ||
		reservationProperties["schema_version"].(JSONSchema)["const"] != HeldOutExecutionReservationSchemaVersion {
		t.Fatal("held-out execution reservation schema is open or has the wrong version")
	}
	reservation, err := reservationStore.Reserve(
		context.Background(), permit, preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, providedAuthorizations,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reservation.ExecutionStarted || reservation.ProviderCalls != 0 || reservation.EmpiricalUnits != 0 || reservation.NetworkPerformed {
		t.Fatalf("reservation fabricated execution: %+v", reservation)
	}
	loaded, err := reservationStore.Load(permit)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, reservation) {
		t.Fatal("loaded reservation differs from the atomically published receipt")
	}
	encodedReservation, err := EncodeIndented(reservation)
	if err != nil {
		t.Fatal(err)
	}
	decodedReservation, err := DecodeHeldOutExecutionReservation(bytes.NewReader(encodedReservation))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedReservation, reservation) {
		t.Fatal("reservation changed across strict JSON decoding")
	}
	receiptPath, err := reservationStore.root.Resolve(filepath.FromSlash(reservation.ReservationKey))
	if err != nil {
		t.Fatal(err)
	}
	receiptInfo, err := os.Stat(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if receiptInfo.Mode().Perm() != safety.SensitiveFileMode {
		t.Fatalf("reservation receipt mode=%#o want %#o", receiptInfo.Mode().Perm(), safety.SensitiveFileMode)
	}
	if _, err := reservationStore.Reserve(
		context.Background(), permit, preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
		fixture.evidence, providedAuthorizations,
	); err == nil {
		t.Fatal("held-out execution permit was reserved twice")
	}
	t.Run("rejects receipt substitution and unknown fields", func(t *testing.T) {
		substituted := reservation
		substituted.PermitDigest = digestText("foreign-held-out-permit")
		substituted.ReservationKey = heldOutExecutionReservationKey(substituted.PermitDigest)
		substituted.Digest = ""
		substituted.Digest, err = heldOutExecutionReservationDigest(substituted)
		if err != nil {
			t.Fatal(err)
		}
		if err := substituted.Validate(); err != nil {
			t.Fatalf("resealed receipt substitution should remain structurally valid: %v", err)
		}
		if err := substituted.ValidateAgainst(permit, reservedAt); err == nil {
			t.Fatal("held-out execution reservation accepted a foreign permit digest")
		}

		var document map[string]any
		if err := json.Unmarshal(encodedReservation, &document); err != nil {
			t.Fatal(err)
		}
		document["unknown"] = true
		unknown, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeHeldOutExecutionReservation(bytes.NewReader(unknown)); err == nil {
			t.Fatal("held-out execution reservation accepted an unknown property")
		}
	})

	t.Run("publishes one receipt across concurrent contenders", func(t *testing.T) {
		const contenders = 16
		concurrentStore := heldOutReservationStoreForTest(t, reservedAt)
		concurrentPermit, buildErr := BuildHeldOutExecutionPermit(
			context.Background(), preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
			fixture.evidence, providedAuthorizations, concurrentStore.Authority(), issuedAt,
		)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		var group sync.WaitGroup
		results := make(chan HeldOutExecutionReservation, contenders)
		errorsFound := make(chan error, contenders)
		for range contenders {
			group.Add(1)
			go func() {
				defer group.Done()
				reservation, reserveErr := concurrentStore.reserveVerified(concurrentPermit, reservedAt)
				if reserveErr != nil {
					errorsFound <- reserveErr
					return
				}
				results <- reservation
			}()
		}
		group.Wait()
		close(results)
		close(errorsFound)
		if len(results) != 1 || len(errorsFound) != contenders-1 {
			t.Fatalf("reservation outcomes: success=%d errors=%d", len(results), len(errorsFound))
		}
		concurrentReservation := <-results
		if concurrentReservation.ExecutionStarted || concurrentReservation.ProviderCalls != 0 ||
			concurrentReservation.EmpiricalUnits != 0 || concurrentReservation.NetworkPerformed {
			t.Fatalf("reservation fabricated execution: %+v", concurrentReservation)
		}
		loaded, loadErr := concurrentStore.Load(concurrentPermit)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if !reflect.DeepEqual(loaded, concurrentReservation) {
			t.Fatal("loaded reservation differs from the atomically published receipt")
		}
		encoded, encodeErr := EncodeIndented(concurrentReservation)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		decoded, decodeErr := DecodeHeldOutExecutionReservation(bytes.NewReader(encoded))
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if !reflect.DeepEqual(decoded, concurrentReservation) {
			t.Fatal("reservation changed across strict JSON decoding")
		}
	})

	t.Run("rejects missing or foreign authorization", func(t *testing.T) {
		if _, err := BuildHeldOutExecutionPermit(
			context.Background(), preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
			fixture.evidence, providedAuthorizations[:1], reservationAuthority, issuedAt,
		); err == nil {
			t.Fatal("held-out execution permit accepted one missing authorization")
		}
		foreign := append([]string(nil), providedAuthorizations...)
		foreign[0] = digestText("foreign-held-out-authorization")
		if _, err := BuildHeldOutExecutionPermit(
			context.Background(), preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
			fixture.evidence, foreign, reservationAuthority, issuedAt,
		); err == nil {
			t.Fatal("held-out execution permit accepted a foreign authorization")
		}
	})

	t.Run("rejects expired routes and foreign capsule", func(t *testing.T) {
		expiresAt, parseErr := parseHeldOutExecutionPermitTime(permit.ExpiresAt)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if _, err := BuildHeldOutExecutionPermit(
			context.Background(), preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
			fixture.evidence, providedAuthorizations, reservationAuthority, expiresAt,
		); err == nil {
			t.Fatal("held-out execution permit accepted route evidence at its expiry boundary")
		}
		if _, err := BuildHeldOutExecutionPermit(
			context.Background(), fixture.privateRelation, fixture.privateRelation, fixture.admission, fixture.execution,
			fixture.evidence, providedAuthorizations, reservationAuthority, issuedAt,
		); err == nil {
			t.Fatal("held-out execution permit accepted a foreign preflight capsule")
		}
	})

	t.Run("rejects resealed workload substitution", func(t *testing.T) {
		substituted := permit
		substituted.Arms = append([]HeldOutExecutionPermitArm(nil), permit.Arms...)
		substituted.Arms[0].EligibleTestCellSetDigest = digestText("foreign-held-out-cell-set")
		substituted.Digest = ""
		substituted.Digest, err = heldOutExecutionPermitDigest(substituted)
		if err != nil {
			t.Fatal(err)
		}
		if err := substituted.Validate(); err != nil {
			t.Fatalf("resealed substitution should remain structurally valid: %v", err)
		}
		if err := VerifyHeldOutExecutionPermit(
			context.Background(), substituted, preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
			fixture.evidence, providedAuthorizations, reservationAuthority, issuedAt.Add(time.Minute),
		); err == nil {
			t.Fatal("held-out execution permit accepted a resealed foreign workload")
		}
	})

	t.Run("rejects foreign reservation authority", func(t *testing.T) {
		foreignStore := heldOutReservationStoreForTest(t, reservedAt)
		if _, err := foreignStore.Reserve(
			context.Background(), permit, preflightCapsule, fixture.privateRelation, fixture.admission, fixture.execution,
			fixture.evidence, providedAuthorizations,
		); err == nil {
			t.Fatal("foreign reservation authority consumed the execution permit")
		}
	})

	t.Run("rejects revision-required private owner custody", func(t *testing.T) {
		revision := heldOutPrivateRelationPackageForTest(t, fixture.admission, relationevidence.PilotInspectionOverallRevisionRequired)
		if _, err := BuildHeldOutPreflightCustody(
			context.Background(), revision, fixture.admission, fixture.execution, fixture.evidence,
		); err == nil {
			t.Fatal("held-out preflight accepted revision-required private owner custody")
		}
	})

	t.Run("rejects expired route evidence", func(t *testing.T) {
		expired := fixture.evidence
		expired.VerifiedAt = fixture.evidence.RouteAttestations[0].ExpiresAt.UTC().Format(time.RFC3339)
		expired.Digest = ""
		expired.Digest, err = heldOutPreflightEvidenceDigest(expired)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := BuildHeldOutPreflightCustody(
			context.Background(), fixture.privateRelation, fixture.admission, fixture.execution, expired,
		); err == nil {
			t.Fatal("held-out preflight accepted expired route evidence")
		}
	})

	t.Run("rejects resealed authority promotion", func(t *testing.T) {
		promoted := value
		promoted.RunAuthorized = true
		promoted.ExecutionPermitIssued = true
		promoted.Digest = ""
		promoted.Digest, err = heldOutPreflightCustodyDigest(promoted)
		if err != nil {
			t.Fatal(err)
		}
		if err := promoted.Validate(); err == nil {
			t.Fatal("held-out preflight custody promoted verified prerequisites into execution authority")
		}
	})
}

func heldOutReservationStoreForTest(t *testing.T, now time.Time) *HeldOutExecutionReservationStore {
	t.Helper()
	home := t.TempDir()
	working := t.TempDir()
	policy, err := safety.NewPathPolicy(safety.PathPolicyOptions{UserHome: home, WorkingDir: working})
	if err != nil {
		t.Fatal(err)
	}
	root, err := safety.CreateCacheRoot(policy, filepath.Join(home, ".cache", "evalwitness-held-out-reservations"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewHeldOutExecutionReservationStore(root)
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return now }
	return store
}

type heldOutPreflightFixture struct {
	admission       HeldOutAdmissionPlan
	execution       HeldOutExecutionBatchBinding
	evidence        HeldOutPreflightEvidence
	privateRelation capsule.Package
	verifiedAt      time.Time
}

func heldOutPreflightFixtureForTest(t *testing.T, ownerStatus relationevidence.PilotInspectionOverallStatus) heldOutPreflightFixture {
	t.Helper()
	_, lock, design, armPlan, registry, replayed, owner := currentHeldOutReadinessRefusal(t)
	campaign, err := BuildHeldOutCampaignPlan(lock, design, armPlan, registry, replayed)
	if err != nil {
		t.Fatal(err)
	}
	corpusPlan, corpusAudit, corpusRelease := currentCorpusV3(t)
	relationPlan, primarySample := currentRelationGovernanceV3(t)
	owner = passedHeldOutOwner(t, owner)
	ledger := supportedPrimaryLedgerV3(t, primarySample, corpusRelease)
	admission, err := BuildHeldOutAdmissionPlan(
		campaign, lock, design, armPlan, registry, replayed,
		corpusPlan, corpusAudit, corpusRelease, relationPlan, primarySample,
		owner, owner.PackageInventoryDigest, ledger,
	)
	if err != nil {
		t.Fatal(err)
	}
	privateRelation := heldOutPrivateRelationPackageForTest(t, admission, ownerStatus)
	placeholderManifest := digestText("held-out-preflight-placeholder-study")
	createdAt := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	prototypeBindings := make(map[string]HeldOutExecutionArmBatchBinding, 2)
	attestations := make(map[string][]conformance.Attestation, 2)
	attestationByCapability := make(map[string]conformance.Attestation)
	for _, campaignArm := range heldOutProviderCampaignArms(campaign) {
		if campaignArm.ExecutionClass != HeldOutExecutionLiveProvider {
			continue
		}
		candidate := heldOutEligibleBatchCandidateForCapsule(
			t, campaign, armPlan, replayed, admission, placeholderManifest, "preflight-model", campaignArm.ArmID,
			privateRelation.Manifest.CapsuleID,
		)
		binding, err := BuildHeldOutExecutionArmBatchBinding(
			campaign, lock, design, armPlan, registry, replayed, admission, candidate,
		)
		if err != nil {
			t.Fatal(err)
		}
		prototypeBindings[campaignArm.ArmID] = binding
		attestations[campaignArm.ArmID] = heldOutRouteAttestations(
			t, candidate.Batch.Requests.Requests, campaignArm.ArmID, createdAt.Add(-2*time.Hour), attestationByCapability,
		)
	}
	manifest := heldOutStudyManifestForTest(t, admission, prototypeBindings, attestations, createdAt)
	record, err := study.Lock(manifest, "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	record, err = study.Transition(
		record, study.StateAuthorized, manifest.Identity.LockedAt.Add(time.Minute), "owner@example.test",
		"authorize exact held-out verifier audit", heldOutAttestationDigests(t, attestations),
	)
	if err != nil {
		t.Fatal(err)
	}
	armBindings := make([]HeldOutExecutionArmBatchBinding, 0, campaign.ProviderDependentArms)
	authorizations := make([]mode.AuthorizationPlan, 0, 2)
	for _, campaignArm := range heldOutProviderCampaignArms(campaign) {
		candidate := heldOutEligibleBatchCandidateForCapsule(
			t, campaign, armPlan, replayed, admission, record.Study.ManifestDigest, "preflight-model", campaignArm.ArmID,
			privateRelation.Manifest.CapsuleID,
		)
		binding, err := BuildHeldOutExecutionArmBatchBinding(
			campaign, lock, design, armPlan, registry, replayed, admission, candidate,
		)
		if err != nil {
			t.Fatal(err)
		}
		if prototype, live := prototypeBindings[campaignArm.ArmID]; live &&
			(prototype.Batch.RequestContractDigest != binding.Batch.RequestContractDigest ||
				prototype.Batch.RouteConfigDigest != binding.Batch.RouteConfigDigest) {
			t.Fatalf("study locking changed route or request contract for arm %q", campaignArm.ArmID)
		}
		armBindings = append(armBindings, binding)
		if candidate.Batch.Authorization != nil {
			authorizations = append(authorizations, *candidate.Batch.Authorization)
		}
	}
	execution, err := BuildHeldOutExecutionBatchBinding(
		campaign, lock, design, armPlan, registry, replayed, admission, armBindings,
	)
	if err != nil {
		t.Fatal(err)
	}
	executionBindings := heldOutStudyExecutionBindingsForTest(t, record, execution)
	routes := make([]conformance.Attestation, 0, len(attestationByCapability))
	for _, attestation := range attestationByCapability {
		routes = append(routes, attestation)
	}
	verifiedAt := manifest.Identity.LockedAt.Add(2 * time.Minute)
	evidence, err := BuildHeldOutPreflightEvidence(record, executionBindings, routes, authorizations, verifiedAt)
	if err != nil {
		t.Fatal(err)
	}
	return heldOutPreflightFixture{
		admission: admission, execution: execution, evidence: evidence,
		privateRelation: privateRelation, verifiedAt: verifiedAt,
	}
}

func heldOutPrivateRelationPackageForTest(
	t *testing.T,
	admission HeldOutAdmissionPlan,
	status relationevidence.PilotInspectionOverallStatus,
) capsule.Package {
	t.Helper()
	proof := capsule.PrivateRelationProof{
		SchemaVersion: capsule.PrivateRelationProofSchemaVersion, PackageFormat: relationevidence.PilotPackageFormatV5,
		PackageInventoryDigest: admission.ExpectedOwnerPackageDigest, PackagePayloadFiles: 53,
		SessionDigest: digestText("preflight-private-session"), EventCount: relationevidence.PilotInspectionRequiredAssessments,
		RequiredAssessments:    relationevidence.PilotInspectionRequiredAssessments,
		CompletedAssessments:   relationevidence.PilotInspectionRequiredAssessments,
		InspectionRecordDigest: digestText("preflight-private-inspection"), CompletionDigest: digestText("preflight-private-completion"),
		CoreStatus: status, ScarcityStatus: status, OverallStatus: status,
		HumanStudyStatus:     relationevidence.PilotInspectionJournalHumanStudyStatus,
		ExternalActionStatus: relationevidence.PilotInspectionJournalExternalAction,
		PublicCapsuleID:      digestText("preflight-public-parent"), PublicCommitmentComponentID: digestText("preflight-public-commitment"),
		PublicAttestationComponentID: digestText("preflight-public-attestation-component"),
		PublicAttestationDigest:      admission.OwnerAttestationDigest,
		VerificationSteps: []string{
			"exact_package_inventory", "exact_package_payloads", "full_package_parent_reconstruction", "immutable_session",
			"ordered_append_only_event_chain", "seven_packet_inspection_reproduction", "combined_completion_reproduction",
			"closed_public_projection_reproduction",
		},
	}
	var err error
	proof.Digest, err = protocolkit.Digest(proof)
	if err != nil {
		t.Fatal(err)
	}
	proofRaw, err := protocolkit.CanonicalMarshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	const validatorID = "evalwitness.validator.stress-preflight-private-proof.v1"
	document, err := capsule.SealRegistry(capsule.PrivateRelationRegistryID, digestText("preflight-public-registry"), []capsule.ComponentType{{
		TypeID: capsule.PrivateRelationProofSchemaVersion, SchemaID: capsule.PrivateRelationProofSchemaVersion,
		Role: capsule.RoleDerivation, AllowedVisibilities: []capsule.Visibility{capsule.VisibilityPrivate},
		MediaType: "application/json", PayloadProfile: capsule.PayloadCanonicalJSON, ValidatorID: validatorID,
		ParentRules: []capsule.ParentRule{},
	}})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := capsule.NewRegistry(document, map[string]capsule.PayloadValidator{
		validatorID: func(raw []byte) error {
			var value capsule.PrivateRelationProof
			if err := protocolkit.DecodeStrict(raw, &value); err != nil {
				return err
			}
			return value.Validate()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	record, normalized, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "relation.owner-private-proof", TypeID: capsule.PrivateRelationProofSchemaVersion,
		Visibility: capsule.VisibilityPrivate, Payload: proofRaw,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := capsule.BuildManifest(registry, capsule.ManifestInput{
		StudyID: "task-068-owner-inspection", CellID: "private-owner-inspection-test",
		ParentCapsules:  []capsule.CapsuleRef{{Relation: "extends", CapsuleID: proof.PublicCapsuleID}},
		ScientificRoots: []string{record.ComponentID}, Components: []capsule.ComponentRecord{record},
	})
	if err != nil {
		t.Fatal(err)
	}
	return capsule.Package{Registry: registry, Manifest: manifest, Payloads: map[string][]byte{record.Payload.Digest: normalized}}
}

func heldOutRouteAttestation(
	t *testing.T,
	request provider.RequestEnvelope,
	armID string,
	observedAt time.Time,
) conformance.Attestation {
	t.Helper()
	var raw strings.Builder
	strictRoute := request.Logprobs && request.TopLogprobs >= verifier.MinimumVerifierTopK
	ordered := make([]provider.TokenEvidence, 0, len(request.ScoreTags)*3)
	position := 0
	for _, tag := range request.ScoreTags {
		closing := strings.Replace(tag, "<", "</", 1)
		raw.WriteString(tag)
		raw.WriteString("A")
		raw.WriteString(closing)
		raw.WriteString("\n")
		if strictRoute {
			ordered = append(ordered,
				provider.TokenEvidence{Position: position, Token: tag, Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
				provider.TokenEvidence{Position: position + 1, Token: "A", Logprob: probabilityLogString(0.60), TopAlternatives: strictAlternatives("A")},
				provider.TokenEvidence{Position: position + 2, Token: closing, Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			)
			position += 3
		}
	}
	response, err := provider.FinalizeResponse(request, provider.ResponseRecord{
		ServedModel: "served-preflight-model", ProviderRequestID: "preflight-" + armID, FinishReason: "stop",
		Usage: provider.TokenUsage{Input: 200, Output: len(ordered)}, NormalizedBody: []byte(`{"fixture":"held-out-preflight"}`),
		RawText: raw.String(), HasLogprobs: strictRoute, ObservedTopLogprobs: request.TopLogprobs,
		OrderedTokenEvidence: ordered,
	})
	if err != nil {
		t.Fatal(err)
	}
	qualification := conformance.QualificationContext{
		ObservedAt: observedAt, ExpiresAt: observedAt.Add(24 * time.Hour), Latency: time.Second,
		HTTPAttempts: 1, StreamingObserved: true, UsageObserved: true,
		Build: conformance.BuildIdentity{Commit: "0123456789abcdef", BinarySHA256: digestText("preflight-attestation-binary")},
	}
	if !strictRoute {
		attestation, err := conformance.EvaluateProbe(request, response, qualification)
		if err != nil {
			t.Fatal(err)
		}
		return attestation
	}
	evidence := verifier.ExtractAllScoreEvidence(request, response, verifier.ExtractionModeVerifier)
	if err := verifier.ValidateStrictEvidence(evidence); err != nil {
		t.Fatal(err)
	}
	attestation, err := conformance.EvaluateBounded(request, response, evidence, qualification)
	if err != nil {
		t.Fatal(err)
	}
	return attestation
}

func heldOutRouteAttestations(
	t *testing.T,
	requests []provider.RequestEnvelope,
	armID string,
	observedAt time.Time,
	shared map[string]conformance.Attestation,
) []conformance.Attestation {
	t.Helper()
	result := make([]conformance.Attestation, 0)
	seen := make(map[string]struct{})
	for _, request := range requests {
		key := conformance.RouteConfigDigest(request) + "\x00" + conformance.CapabilityContractDigest(request)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		attestation, exists := shared[key]
		if !exists {
			attestation = heldOutRouteAttestation(t, request, armID, observedAt)
			shared[key] = attestation
		}
		result = append(result, attestation)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].AttestationDigest < result[right].AttestationDigest
	})
	return result
}

func heldOutStudyManifestForTest(
	t *testing.T,
	admission HeldOutAdmissionPlan,
	live map[string]HeldOutExecutionArmBatchBinding,
	attestations map[string][]conformance.Attestation,
	createdAt time.Time,
) study.Manifest {
	t.Helper()
	if len(live) != 2 || len(attestations) != 2 {
		t.Fatal("held-out study fixture requires exactly two live arms")
	}
	split := heldOutStudySplitForTest(t)
	var taskIDs []string
	var trajectoryDigests []string
	for _, assignment := range split.Assignments {
		taskIDs = append(taskIDs, assignment.TaskIDs...)
		trajectoryDigests = append(trajectoryDigests, assignment.TrajectoryDigests...)
	}
	armIDs := make([]string, 0, len(live))
	for armID := range live {
		armIDs = append(armIDs, armID)
	}
	sort.Strings(armIDs)
	first := live[armIDs[0]].Batch
	manifest := study.Manifest{
		SchemaVersion: study.ManifestSchemaVersion, CanonicalPolicy: study.CanonicalPolicy,
		Identity: study.Identity{
			Title: "Held-out verifier trajectory audit", ResearchQuestion: "Do verifier modes preserve registered controlled relations?",
			Kind: study.KindControlledRelation, Authors: []string{"owner@example.test"}, CreatedAt: createdAt, LockedAt: createdAt.Add(time.Hour),
		},
		Hypotheses: study.Hypotheses{PrimaryNull: "registered relation preservation is not improved", PrimaryAlternative: "registered relation preservation is improved"},
		Data: study.DataPlan{PrimaryUnit: "task", Datasets: []study.DatasetManifest{{
			ID: "held-out-controlled-relations", Source: "frozen mutation corpus", Version: "v3", License: "Apache-2.0", AcquiredAt: createdAt.Add(-time.Hour),
			DatasetDigest: admission.CorpusReleaseDigest, TaskIDsDigest: heldOutStudyStringSetDigest(taskIDs),
			OutcomeLabelsDigest: admission.ExpectedOwnerPackageDigest, TrajectorySetDigest: heldOutStudyStringSetDigest(trajectoryDigests),
			TaskCount: len(taskIDs), PermittedRoles: []study.DataRole{study.RoleDevelopment, study.RoleCalibration, study.RoleTest},
		}}, Split: split},
		Outcomes: study.OutcomePlan{Primary: study.Endpoint{
			ID: "relation_preservation", Metric: "paired relation preservation difference", Direction: "higher", Question: "superiority",
			FailureDenominator: "all locked test tasks",
		}},
		Inference: study.InferencePlan{
			Test: "exact_mcnemar", IntervalMethod: "newcombe_paired_score_method_10", DesignMethod: "exact_mcnemar_unconditional",
			DesignEvidenceDigest: admission.AnalysisDesignDigest, ClusterUnit: "task", NominalAlpha: 0.05, TargetPower: 0.80,
			MinimumEffect: 1, DisagreementRate: 1, DiscordantWinProbability: 1, PowerAtMinimumEffect: 1,
			DecidableTasks: len(taskIDs), MultiplicityMethod: "bonferroni", PrimaryFamily: []string{"relation_preservation"},
			Sequential: study.SequentialPlan{Method: "fixed_sample", MaximumLooks: 1},
		},
		Failures: study.FailurePlan{
			MissingScore: "failure", ProviderFailure: "failure", RouteFailure: "failure", Timeout: "failure", Abstention: "failure",
			BudgetExhaustion: "failed study", RetryExhaustion: "failure", IncompleteCell: "retain and report",
			DenominatorPolicy: "all locked test tasks",
		},
		Controls: study.ControlPlan{
			RandomSelectionID: "frozen-hash-selection", TaskIndependentSelector: "registered-cell-order",
			PositiveControl: "known controlled corruption", PositiveControlSource: study.RoleCalibration,
		},
		Budget: study.BudgetPlan{
			ExpectedCalls: first.WorstLogicalCalls, HardCalls: first.Budget.MaxCalls, HardAttempts: first.Budget.MaxAttempts,
			HardInputTokens: first.Budget.MaxEstimatedInputTokens, HardOutputTokens: first.Budget.MaxReservedOutputTokens,
			HardDurationSeconds: int64(time.Duration(first.Budget.MaxDurationNanoseconds) / time.Second),
			HardConcurrent:      first.Budget.MaxConcurrent, HardCostUSD: first.Budget.MaxCostUSD,
		},
		Execution: study.ExecutionPlan{
			Commit: digestText("held-out-preflight-commit"), BinaryDigest: digestText("held-out-preflight-binary"), Platform: "darwin/arm64",
			AnalysisCommand: []string{"evalwitness", "stress", "run-held-out"}, AnalysisVersion: "evalwitness.stress-held-out.v1",
			AnalysisDigest:     admission.AnalysisDesignDigest,
			DeclaredInputPaths: []string{"eval/governance/controlled-corruption-v3-release.json"}, DeclaredInputDigests: []string{admission.CorpusReleaseDigest},
		},
		Publication: study.PublicationPlan{
			CapsuleVisibility: "private", AllowedClaimIDs: []string{"relation_preservation"},
			RequiredCaveats: []string{"owner-authorized held-out execution required"}, IndependentReproductionGate: true,
			RegisteredReportTimestamp: createdAt.Add(30 * time.Minute),
		},
		Reliability: study.ReliabilityContracts{
			ProtocolVersion: protocolkit.CurrentVersion, ProtocolCorpusDigest: admission.CorpusReleaseDigest,
			ProtocolRequestCorpusDigest: admission.ExecutionEligibleCellSetDigestForTest(t), ProtocolSchemaDigest: admission.RelationPlanDigest,
			TraceMappingPolicy: preprocess.TraceMappingPolicyVersion, RelationCorpusDigest: admission.CorpusReleaseDigest,
			ValidatorContractDigest: admission.RegistryDigest, OutcomeContractDigest: admission.ExpectedOwnerPackageDigest,
			AdjudicationContractDigest: admission.TerminalLedgerDigest, ProfileProjectionDigest: first.ProfilePolicyDigests[0],
		},
		Relations: &study.ControlledRelations{
			CorpusVersion: "v3", RelationContractVersion: "evalwitness.stress-relation.v1",
			MutationFamilies: []string{"controlled_corruption"}, ExpectedRelations: []string{"invariance", "sensitivity"},
			ValidatorDigests: []string{admission.RegistryDigest}, AmbiguityPolicy: "exclude_primary_retain_sensitivity",
			PrimaryDenominator: "all admission-eligible locked test cells", ClusterUnit: "source_task",
			ReductionPolicy: "frozen_before_execution", ClaimType: "invariance",
		},
		Adjudication: study.AdjudicationPlan{
			SampleStrata: []string{"human_supported", "formal_only"}, Blinding: "arm and route hidden", AgreementMetric: "Gwet AC1",
			ConflictResolution: "third blinded adjudicator", LabelRevision: "new locked manifest revision",
			SensitivityAnalysis: "primary and sensitivity estimands reported separately",
		},
	}
	for _, armID := range armIDs {
		binding := live[armID].Batch
		armAttestations := attestations[armID]
		if len(armAttestations) == 0 {
			t.Fatalf("held-out study arm %q has no route attestations", armID)
		}
		attestation := armAttestations[0]
		attestationSetDigest, err := heldOutRouteAttestationSetDigest(armAttestations)
		if err != nil {
			t.Fatal(err)
		}
		if binding.Budget != first.Budget || binding.WorstLogicalCalls != first.WorstLogicalCalls {
			t.Fatal("held-out live arms require one shared locked study budget")
		}
		manifest.Arms = append(manifest.Arms, study.Arm{
			ID: armID, Entrypoint: binding.Entrypoint, RouteID: binding.RouteID, ProviderID: attestation.Identity.ProviderID,
			RequestedModel: attestation.Identity.RequestedModel, PromptDigest: binding.PlanFingerprintDigest,
			RequestContractDigest: binding.RequestContractDigest, ScorePolicyVersion: verifier.StrictPolicyVersion,
			CalibrationDigest: binding.ProfilePolicyDigests[0], SelectionMode: "registered_relation_side",
			Candidates: 2, Repetitions: first.Repetitions[0], AttestationDigest: attestationSetDigest,
		})
		manifest.Providers = append(manifest.Providers, study.ProviderPlan{
			ArmID: armID, AttestationObservedAt: attestation.ObservedAt, AttestationExpiresAt: attestation.ExpiresAt,
			ServedIdentityPolicy: "exact_observed", ExpectedServedModel: attestation.Identity.ServedModel,
			CheckpointAssertionPolicy: "exact_observed", RetryPolicyVersion: provider.RetryPolicyVersion,
			MaxRetries: first.Budget.MaxAttempts/first.Budget.MaxCalls - 1, RequestTimeoutSeconds: 1,
		})
		manifest.Execution.DeclaredRouteIDs = append(manifest.Execution.DeclaredRouteIDs, binding.RouteID)
	}
	sort.Strings(manifest.Execution.DeclaredRouteIDs)
	manifest.Execution.DeclaredRouteIDs = compactHeldOutStrings(manifest.Execution.DeclaredRouteIDs)
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func heldOutStudySplitForTest(t *testing.T) study.SplitManifest {
	t.Helper()
	groups := make([]study.SplitGroup, 50)
	for index := range groups {
		id := fmt.Sprintf("%03d", index)
		groups[index] = study.SplitGroup{
			DatasetID: "held-out-controlled-relations", GroupID: "group-" + id,
			TaskIDs:       []string{"task-" + id + "-1", "task-" + id + "-2"},
			RepositoryIDs: []string{"repo-" + id}, CloneFamilyIDs: []string{"clone-" + id},
			TrajectoryDigests: []string{digestText("trajectory:" + id)},
		}
	}
	split, err := study.GenerateSplit(study.SplitSpec{
		Seed: "held-out-preflight-split", Weights: []study.SplitWeight{
			{Role: study.RoleDevelopment, Weight: 1}, {Role: study.RoleCalibration, Weight: 1}, {Role: study.RoleTest, Weight: 1},
		},
	}, groups)
	if err != nil {
		t.Fatal(err)
	}
	return split
}

func heldOutStudyStringSetDigest(values []string) string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	var encoded bytes.Buffer
	var length [8]byte
	for _, value := range values {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		encoded.Write(length[:])
		encoded.WriteString(value)
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes()))
}

func heldOutAttestationDigests(t *testing.T, values map[string][]conformance.Attestation) []string {
	t.Helper()
	result := make([]string, 0, len(values))
	for _, armAttestations := range values {
		digest, err := heldOutRouteAttestationSetDigest(armAttestations)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, digest)
	}
	sort.Strings(result)
	return result
}

func heldOutStudyExecutionBindingsForTest(
	t *testing.T,
	record study.Record,
	execution HeldOutExecutionBatchBinding,
) []study.ExecutionBinding {
	t.Helper()
	result := make([]study.ExecutionBinding, 0, 2)
	for _, arm := range execution.Arms {
		if arm.ExecutionClass != HeldOutExecutionLiveProvider {
			continue
		}
		var providerPlan *study.ProviderPlan
		for index := range record.Study.Manifest.Providers {
			if record.Study.Manifest.Providers[index].ArmID == arm.ArmID {
				providerPlan = &record.Study.Manifest.Providers[index]
				break
			}
		}
		if providerPlan == nil {
			t.Fatalf("provider plan for arm %q not found", arm.ArmID)
		}
		manifest := record.Study.Manifest
		binding := study.ExecutionBinding{
			ArmID: arm.ArmID, Entrypoint: arm.Batch.Entrypoint, RouteID: arm.Batch.RouteID,
			RequestContractDigest: arm.Batch.RequestContractDigest, Commit: manifest.Execution.Commit,
			Dirty: manifest.Execution.Dirty, BinaryDigest: manifest.Execution.BinaryDigest, AnalysisDigest: manifest.Execution.AnalysisDigest,
			AnalysisCommand: append([]string(nil), manifest.Execution.AnalysisCommand...), AnalysisVersion: manifest.Execution.AnalysisVersion,
			InputPaths: append([]string(nil), manifest.Execution.DeclaredInputPaths...), InputDigests: append([]string(nil), manifest.Execution.DeclaredInputDigests...),
			ExpectedCalls: manifest.Budget.ExpectedCalls, HardCalls: manifest.Budget.HardCalls, HardAttempts: manifest.Budget.HardAttempts,
			HardInputTokens: manifest.Budget.HardInputTokens, HardOutputTokens: manifest.Budget.HardOutputTokens,
			HardDurationSeconds: manifest.Budget.HardDurationSeconds, HardConcurrent: manifest.Budget.HardConcurrent,
			HardCostUSD: manifest.Budget.HardCostUSD, DecidableTasks: manifest.Inference.DecidableTasks,
			NominalAlpha: manifest.Inference.NominalAlpha, TargetPower: manifest.Inference.TargetPower,
			MinimumEffect: manifest.Inference.MinimumEffect, DisagreementRate: manifest.Inference.DisagreementRate,
			DiscordantWinProbability: manifest.Inference.DiscordantWinProbability, PowerAtMinimumEffect: manifest.Inference.PowerAtMinimumEffect,
			PrimaryFamilySize:    len(manifest.Inference.PrimaryFamily),
			ServedIdentityPolicy: providerPlan.ServedIdentityPolicy, ExpectedServedModel: providerPlan.ExpectedServedModel,
			ExpectedServedModels: append([]string(nil), providerPlan.ExpectedServedModels...), RetryPolicyVersion: providerPlan.RetryPolicyVersion,
			MaxRetries: providerPlan.MaxRetries, RequestTimeoutSeconds: providerPlan.RequestTimeoutSeconds,
		}
		if err := study.VerifyExecutionBinding(record, binding); err != nil {
			t.Fatal(err)
		}
		result = append(result, binding)
	}
	return result
}

func compactHeldOutStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func (value HeldOutAdmissionPlan) ExecutionEligibleCellSetDigestForTest(t *testing.T) string {
	t.Helper()
	digest, err := digestDocument(value.ExecutionEligibleCellIDs)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
