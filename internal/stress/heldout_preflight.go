package stress

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	HeldOutPreflightEvidenceSchemaVersion = "evalwitness.stress-held-out-preflight-evidence.v1"
	HeldOutPreflightCustodySchemaVersion  = "evalwitness.stress-held-out-preflight-custody.v1"
	heldOutPreflightStatus                = "all_preflight_evidence_verified_execution_not_authorized"
)

type HeldOutPreflightEvidence struct {
	SchemaVersion      string                    `json:"schema_version"`
	CanonicalPolicy    string                    `json:"canonical_policy"`
	StudyRecord        study.Record              `json:"study_record"`
	ExecutionBindings  []study.ExecutionBinding  `json:"execution_bindings"`
	RouteAttestations  []conformance.Attestation `json:"route_attestations"`
	AuthorizationPlans []mode.AuthorizationPlan  `json:"authorization_plans"`
	VerifiedAt         string                    `json:"verified_at"`
	Digest             string                    `json:"digest"`
}

type HeldOutPreflightCustody struct {
	SchemaVersion                     string                       `json:"schema_version"`
	CanonicalPolicy                   string                       `json:"canonical_policy"`
	CampaignDigest                    string                       `json:"campaign_digest"`
	AdmissionPlanDigest               string                       `json:"admission_plan_digest"`
	ExecutionBatchBindingDigest       string                       `json:"execution_batch_binding_digest"`
	PreflightEvidenceDigest           string                       `json:"preflight_evidence_digest"`
	PrivateRelationCapsuleID          string                       `json:"private_relation_capsule_id"`
	PrivateRelationManifestDigest     string                       `json:"private_relation_manifest_digest"`
	PrivateRelationProofDigest        string                       `json:"private_relation_proof_digest"`
	OwnerAttestationDigest            string                       `json:"owner_attestation_digest"`
	StudyRecordDigest                 string                       `json:"study_record_digest"`
	StudyManifestDigest               string                       `json:"study_manifest_digest"`
	ExecutionBindingDigests           []string                     `json:"execution_binding_digests"`
	RouteAttestationDigests           []string                     `json:"route_attestation_digests"`
	AuthorizationPlanDigests          []string                     `json:"authorization_plan_digests"`
	ProfilePolicyDigest               string                       `json:"profile_policy_digest"`
	VerifiedAt                        string                       `json:"verified_at"`
	RouteAttestationsExpireAt         string                       `json:"route_attestations_expire_at"`
	PrivateRelationCapsuleVerified    bool                         `json:"private_relation_capsule_verified"`
	StudyRecordVerified               bool                         `json:"study_record_verified"`
	ExecutionBindingsVerified         bool                         `json:"execution_bindings_verified"`
	CurrentRoutesAttested             bool                         `json:"current_routes_attested"`
	AuthorizationPlansVerified        bool                         `json:"authorization_plans_verified"`
	AdmissionFilteredWorkloadVerified bool                         `json:"admission_filtered_workload_verified"`
	Status                            string                       `json:"status"`
	ExternalActionStatus              string                       `json:"external_action_status"`
	RunAuthorized                     bool                         `json:"run_authorized"`
	ExecutionPermitIssued             bool                         `json:"execution_permit_issued"`
	ProviderCalls                     int                          `json:"provider_calls"`
	EmpiricalUnits                    int                          `json:"empirical_units"`
	NetworkRequired                   bool                         `json:"network_required"`
	ClaimBoundary                     HeldOutCampaignClaimBoundary `json:"claim_boundary"`
	Digest                            string                       `json:"digest"`
}

func BuildHeldOutPreflightEvidence(
	record study.Record,
	executionBindings []study.ExecutionBinding,
	routeAttestations []conformance.Attestation,
	authorizationPlans []mode.AuthorizationPlan,
	verifiedAt time.Time,
) (HeldOutPreflightEvidence, error) {
	if verifiedAt.IsZero() {
		return HeldOutPreflightEvidence{}, errors.New("stress held-out preflight evidence requires a verification time")
	}
	value := HeldOutPreflightEvidence{
		SchemaVersion: HeldOutPreflightEvidenceSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		StudyRecord: record, ExecutionBindings: slices.Clone(executionBindings), RouteAttestations: slices.Clone(routeAttestations),
		AuthorizationPlans: slices.Clone(authorizationPlans), VerifiedAt: verifiedAt.UTC().Format(time.RFC3339),
	}
	sort.Slice(value.ExecutionBindings, func(left, right int) bool {
		return value.ExecutionBindings[left].ArmID < value.ExecutionBindings[right].ArmID
	})
	sort.Slice(value.RouteAttestations, func(left, right int) bool {
		return value.RouteAttestations[left].AttestationDigest < value.RouteAttestations[right].AttestationDigest
	})
	sort.Slice(value.AuthorizationPlans, func(left, right int) bool {
		return value.AuthorizationPlans[left].AuthorizationDigest < value.AuthorizationPlans[right].AuthorizationDigest
	})
	digest, err := heldOutPreflightEvidenceDigest(value)
	if err != nil {
		return HeldOutPreflightEvidence{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return HeldOutPreflightEvidence{}, err
	}
	return value, nil
}

func (value HeldOutPreflightEvidence) Validate() error {
	if value.SchemaVersion != HeldOutPreflightEvidenceSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		len(value.ExecutionBindings) != 2 || len(value.RouteAttestations) < 2 || len(value.AuthorizationPlans) != 2 {
		return errors.New("stress held-out preflight evidence identity or live-arm evidence counts are invalid")
	}
	verifiedAt, err := time.Parse(time.RFC3339, value.VerifiedAt)
	if err != nil || verifiedAt.IsZero() || value.VerifiedAt != verifiedAt.UTC().Format(time.RFC3339) {
		return errors.New("stress held-out preflight evidence verification time is invalid")
	}
	if err := value.StudyRecord.Validate(); err != nil || value.StudyRecord.State != study.StateAuthorized {
		return errors.New("stress held-out preflight evidence requires an authorized, valid study record")
	}
	previousArm := ""
	for _, binding := range value.ExecutionBindings {
		if binding.ArmID == "" || binding.ArmID <= previousArm {
			return errors.New("stress held-out preflight execution bindings must be unique and arm-sorted")
		}
		previousArm = binding.ArmID
	}
	previousAttestation := ""
	for _, attestation := range value.RouteAttestations {
		if err := attestation.ValidateIntegrity(); err != nil || attestation.AttestationDigest <= previousAttestation {
			return errors.New("stress held-out preflight route attestations are invalid, duplicated, or unsorted")
		}
		previousAttestation = attestation.AttestationDigest
	}
	previousAuthorization := ""
	for _, plan := range value.AuthorizationPlans {
		if plan.AuthorizationDigest <= previousAuthorization || plan.Verify(plan.AuthorizationDigest) != nil {
			return errors.New("stress held-out preflight authorization plans are invalid, duplicated, or unsorted")
		}
		previousAuthorization = plan.AuthorizationDigest
	}
	expected, err := heldOutPreflightEvidenceDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out preflight evidence digest is invalid")
	}
	return nil
}

func BuildHeldOutPreflightCustody(
	ctx context.Context,
	privateRelation capsule.Package,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
) (HeldOutPreflightCustody, error) {
	if ctx == nil {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight custody requires context")
	}
	if err := admission.Validate(); err != nil {
		return HeldOutPreflightCustody{}, err
	}
	if err := execution.Validate(); err != nil {
		return HeldOutPreflightCustody{}, err
	}
	if execution.AdmissionPlanDigest != admission.Digest || execution.CampaignDigest != admission.CampaignDigest {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight execution binding differs from the exact admission plan")
	}
	if err := evidence.Validate(); err != nil {
		return HeldOutPreflightCustody{}, err
	}
	proof, err := verifyHeldOutPrivateRelationParent(ctx, privateRelation, admission)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	verifiedAt, err := time.Parse(time.RFC3339, evidence.VerifiedAt)
	if err != nil {
		return HeldOutPreflightCustody{}, fmt.Errorf("parse stress held-out preflight verification time: %w", err)
	}
	if err := verifyHeldOutPreflightLiveEvidence(admission, execution, evidence, privateRelation.Manifest.CapsuleID, verifiedAt); err != nil {
		return HeldOutPreflightCustody{}, err
	}
	value := HeldOutPreflightCustody{
		SchemaVersion: HeldOutPreflightCustodySchemaVersion, CanonicalPolicy: CanonicalPolicy,
		CampaignDigest: admission.CampaignDigest, AdmissionPlanDigest: admission.Digest,
		ExecutionBatchBindingDigest: execution.Digest, PreflightEvidenceDigest: evidence.Digest,
		PrivateRelationCapsuleID: privateRelation.Manifest.CapsuleID, PrivateRelationManifestDigest: privateRelation.Manifest.ManifestDigest,
		PrivateRelationProofDigest: proof.Digest, OwnerAttestationDigest: admission.OwnerAttestationDigest,
		StudyRecordDigest: evidence.StudyRecord.RecordDigest, StudyManifestDigest: evidence.StudyRecord.Study.ManifestDigest,
		ProfilePolicyDigest: execution.ProfilePolicyDigest, VerifiedAt: evidence.VerifiedAt,
		PrivateRelationCapsuleVerified: true, StudyRecordVerified: true, ExecutionBindingsVerified: true,
		CurrentRoutesAttested: true, AuthorizationPlansVerified: true, AdmissionFilteredWorkloadVerified: true,
		Status: heldOutPreflightStatus, ExternalActionStatus: heldOutBatchBindingExternalStatus,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim: heldOutPreflightSupportedClaim, UnsupportedClaims: slices.Clone(heldOutPreflightUnsupportedClaims),
		},
	}
	for _, binding := range evidence.ExecutionBindings {
		digest, digestErr := digestDocument(binding)
		if digestErr != nil {
			return HeldOutPreflightCustody{}, digestErr
		}
		value.ExecutionBindingDigests = append(value.ExecutionBindingDigests, digest)
	}
	for _, attestation := range evidence.RouteAttestations {
		value.RouteAttestationDigests = append(value.RouteAttestationDigests, attestation.AttestationDigest)
	}
	for _, plan := range evidence.AuthorizationPlans {
		value.AuthorizationPlanDigests = append(value.AuthorizationPlanDigests, plan.AuthorizationDigest)
	}
	sort.Strings(value.ExecutionBindingDigests)
	sort.Strings(value.RouteAttestationDigests)
	sort.Strings(value.AuthorizationPlanDigests)
	expiresAt := earliestHeldOutRouteExpiry(evidence.RouteAttestations)
	value.RouteAttestationsExpireAt = expiresAt.UTC().Format(time.RFC3339)
	value.Digest, err = heldOutPreflightCustodyDigest(value)
	if err != nil {
		return HeldOutPreflightCustody{}, err
	}
	if err := value.ValidateAgainst(admission, execution, evidence, privateRelation, verifiedAt); err != nil {
		return HeldOutPreflightCustody{}, err
	}
	return value, nil
}

func (value HeldOutPreflightCustody) Validate() error {
	if value.SchemaVersion != HeldOutPreflightCustodySchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(value.CampaignDigest) || !validDigest(value.AdmissionPlanDigest) || !validDigest(value.ExecutionBatchBindingDigest) ||
		!validDigest(value.PreflightEvidenceDigest) || !validDigest(value.PrivateRelationCapsuleID) ||
		!validDigest(value.PrivateRelationManifestDigest) || !validDigest(value.PrivateRelationProofDigest) ||
		!validDigest(value.OwnerAttestationDigest) || !validDigest(value.StudyRecordDigest) || !validDigest(value.StudyManifestDigest) ||
		!validDigest(value.ProfilePolicyDigest) || len(value.ExecutionBindingDigests) != 2 || len(value.RouteAttestationDigests) < 2 ||
		len(value.AuthorizationPlanDigests) != 2 {
		return errors.New("stress held-out preflight custody identity, lineage, or exact live-arm evidence counts are invalid")
	}
	for _, values := range [][]string{value.ExecutionBindingDigests, value.RouteAttestationDigests, value.AuthorizationPlanDigests} {
		if !slices.IsSorted(values) || !uniqueSortedStrings(values) {
			return errors.New("stress held-out preflight custody digest sets must be valid, unique, and sorted")
		}
	}
	verifiedAt, verifiedErr := time.Parse(time.RFC3339, value.VerifiedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339, value.RouteAttestationsExpireAt)
	if verifiedErr != nil || expiresErr != nil || !verifiedAt.Before(expiresAt) ||
		value.VerifiedAt != verifiedAt.UTC().Format(time.RFC3339) || value.RouteAttestationsExpireAt != expiresAt.UTC().Format(time.RFC3339) {
		return errors.New("stress held-out preflight custody verification or route-expiry time is invalid")
	}
	if !value.PrivateRelationCapsuleVerified || !value.StudyRecordVerified || !value.ExecutionBindingsVerified ||
		!value.CurrentRoutesAttested || !value.AuthorizationPlansVerified || !value.AdmissionFilteredWorkloadVerified ||
		value.Status != heldOutPreflightStatus || value.ExternalActionStatus != heldOutBatchBindingExternalStatus ||
		value.RunAuthorized || value.ExecutionPermitIssued || value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired ||
		value.ClaimBoundary.SupportedClaim != heldOutPreflightSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutPreflightUnsupportedClaims) {
		return errors.New("stress held-out preflight custody promoted verified prerequisites into execution authority, action, observation, or claim")
	}
	expected, err := heldOutPreflightCustodyDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out preflight custody digest is invalid")
	}
	return nil
}

func (value HeldOutPreflightCustody) ValidateAgainst(
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
	privateRelation capsule.Package,
	verifiedAt time.Time,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if verifiedAt.IsZero() || verifiedAt.UTC().Format(time.RFC3339) != value.VerifiedAt || evidence.VerifiedAt != value.VerifiedAt {
		return errors.New("stress held-out preflight custody verification time differs from its evidence")
	}
	if value.AdmissionPlanDigest != admission.Digest || value.CampaignDigest != admission.CampaignDigest ||
		value.ExecutionBatchBindingDigest != execution.Digest || value.PreflightEvidenceDigest != evidence.Digest ||
		value.PrivateRelationCapsuleID != privateRelation.Manifest.CapsuleID ||
		value.PrivateRelationManifestDigest != privateRelation.Manifest.ManifestDigest ||
		value.OwnerAttestationDigest != admission.OwnerAttestationDigest || value.StudyRecordDigest != evidence.StudyRecord.RecordDigest ||
		value.StudyManifestDigest != evidence.StudyRecord.Study.ManifestDigest || value.ProfilePolicyDigest != execution.ProfilePolicyDigest {
		return errors.New("stress held-out preflight custody differs from its exact capsule, admission, execution, study, or profile parents")
	}
	wantExecution := make([]string, 0, len(evidence.ExecutionBindings))
	for _, binding := range evidence.ExecutionBindings {
		digest, err := digestDocument(binding)
		if err != nil {
			return err
		}
		wantExecution = append(wantExecution, digest)
	}
	wantAttestations := make([]string, 0, len(evidence.RouteAttestations))
	for _, attestation := range evidence.RouteAttestations {
		wantAttestations = append(wantAttestations, attestation.AttestationDigest)
	}
	wantAuthorizations := make([]string, 0, len(evidence.AuthorizationPlans))
	for _, plan := range evidence.AuthorizationPlans {
		wantAuthorizations = append(wantAuthorizations, plan.AuthorizationDigest)
	}
	sort.Strings(wantExecution)
	sort.Strings(wantAttestations)
	sort.Strings(wantAuthorizations)
	if !slices.Equal(value.ExecutionBindingDigests, wantExecution) || !slices.Equal(value.RouteAttestationDigests, wantAttestations) ||
		!slices.Equal(value.AuthorizationPlanDigests, wantAuthorizations) ||
		value.RouteAttestationsExpireAt != earliestHeldOutRouteExpiry(evidence.RouteAttestations).UTC().Format(time.RFC3339) {
		return errors.New("stress held-out preflight custody digest or freshness projections differ from the exact evidence")
	}
	return nil
}

func verifyHeldOutPrivateRelationParent(
	ctx context.Context,
	privateRelation capsule.Package,
	admission HeldOutAdmissionPlan,
) (capsule.PrivateRelationProof, error) {
	if privateRelation.Registry == nil {
		return capsule.PrivateRelationProof{}, errors.New("stress held-out preflight lacks a private relation capsule registry")
	}
	registry := privateRelation.Registry.Document()
	if registry.RegistryID != capsule.PrivateRelationRegistryID || !validDigest(registry.BaseRegistryDigest) ||
		len(privateRelation.Manifest.ParentCapsules) != 1 || privateRelation.Manifest.ParentCapsules[0].Relation != "extends" {
		return capsule.PrivateRelationProof{}, errors.New("stress held-out preflight requires the registered private relation capsule shape")
	}
	if _, err := capsule.VerifyPackage(ctx, privateRelation.Registry, privateRelation.Manifest, privateRelation.Payloads, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPrivate}); err != nil {
		return capsule.PrivateRelationProof{}, fmt.Errorf("verify stress held-out private relation capsule: %w", err)
	}
	record, err := heldOutPrivateRelationProofComponent(privateRelation.Manifest)
	if err != nil {
		return capsule.PrivateRelationProof{}, err
	}
	raw, exists := privateRelation.Payloads[record.Payload.Digest]
	if !exists {
		return capsule.PrivateRelationProof{}, errors.New("stress held-out private relation proof payload is unavailable")
	}
	var proof capsule.PrivateRelationProof
	if err := protocolkit.DecodeStrict(raw, &proof); err != nil {
		return capsule.PrivateRelationProof{}, err
	}
	if err := proof.Validate(); err != nil {
		return capsule.PrivateRelationProof{}, err
	}
	if proof.OverallStatus != relationevidence.PilotInspectionOverallPassed ||
		proof.CoreStatus != relationevidence.PilotInspectionOverallPassed || proof.ScarcityStatus != relationevidence.PilotInspectionOverallPassed ||
		proof.PackageInventoryDigest != admission.ExpectedOwnerPackageDigest || proof.PublicAttestationDigest != admission.OwnerAttestationDigest ||
		proof.PublicCapsuleID != privateRelation.Manifest.ParentCapsules[0].CapsuleID {
		return capsule.PrivateRelationProof{}, errors.New("stress held-out private relation capsule is not the exact passed owner-custody parent")
	}
	return proof, nil
}

func heldOutPrivateRelationProofComponent(manifest capsule.Manifest) (capsule.ComponentRecord, error) {
	record, exists := heldOutCapsuleComponentByType(manifest.Components, capsule.PrivateRelationProofSchemaVersion)
	if !exists || !slices.Contains(manifest.ScientificRoots, record.ComponentID) {
		return capsule.ComponentRecord{}, errors.New("stress held-out private relation proof is absent, repeated, or not a scientific root")
	}
	return record, nil
}

func verifyHeldOutPreflightLiveEvidence(
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
	privateCapsuleID string,
	verifiedAt time.Time,
) error {
	if evidence.StudyRecord.Study.ManifestDigest != execution.StudyManifestDigest || evidence.StudyRecord.State != study.StateAuthorized {
		return errors.New("stress held-out preflight study does not authorize the exact admission-filtered batch manifest")
	}
	liveArms := make(map[string]HeldOutExecutionArmBatchBinding, execution.LiveBatchBindings)
	for _, arm := range execution.Arms {
		if arm.ExecutionClass == HeldOutExecutionLiveProvider {
			liveArms[arm.ArmID] = arm
		}
		for _, digest := range arm.Batch.CapsuleDigests {
			if digest != privateCapsuleID {
				return errors.New("stress held-out preflight input lineage does not bind the verified private relation capsule")
			}
		}
	}
	if len(liveArms) != 2 || len(evidence.StudyRecord.Study.Manifest.Arms) != 2 {
		return errors.New("stress held-out preflight study must contain exactly the two live provider arms")
	}
	studyArms := make(map[string]study.Arm, 2)
	for _, arm := range evidence.StudyRecord.Study.Manifest.Arms {
		studyArms[arm.ID] = arm
	}
	executionBindings := make(map[string]study.ExecutionBinding, len(evidence.ExecutionBindings))
	for _, binding := range evidence.ExecutionBindings {
		executionBindings[binding.ArmID] = binding
	}
	authorizations := make(map[string]mode.AuthorizationPlan, len(evidence.AuthorizationPlans))
	for _, plan := range evidence.AuthorizationPlans {
		authorizations[plan.AuthorizationDigest] = plan
	}
	attestations := make(map[string]conformance.Attestation, len(evidence.RouteAttestations))
	for _, attestation := range evidence.RouteAttestations {
		key := attestation.Identity.RouteConfigDigest + "\x00" + attestation.Contract.ContractDigest
		if _, duplicate := attestations[key]; duplicate {
			return errors.New("stress held-out preflight repeats one route capability contract")
		}
		attestations[key] = attestation
	}
	requiredAttestations := make(map[string]struct{}, len(attestations))
	for armID, arm := range liveArms {
		studyArm, exists := studyArms[armID]
		if !exists || studyArm.Entrypoint != arm.Batch.Entrypoint || studyArm.RouteID != arm.Batch.RouteID ||
			studyArm.RequestContractDigest != arm.Batch.RequestContractDigest {
			return fmt.Errorf("stress held-out preflight study arm %q differs from its exact request binding", armID)
		}
		binding, exists := executionBindings[armID]
		if !exists {
			return fmt.Errorf("stress held-out preflight lacks execution binding for arm %q", armID)
		}
		if err := study.VerifyExecutionBinding(evidence.StudyRecord, binding); err != nil {
			return fmt.Errorf("verify stress held-out execution binding %q: %w", armID, err)
		}
		if binding.ExpectedCalls != arm.Batch.WorstLogicalCalls || binding.RequestContractDigest != arm.Batch.RequestContractDigest ||
			binding.RouteID != arm.Batch.RouteID || binding.Entrypoint != arm.Batch.Entrypoint {
			return fmt.Errorf("stress held-out preflight execution binding %q differs from its exact batch workload", armID)
		}
		authorization, exists := authorizations[arm.Batch.RequiredAuthorizationDigest]
		if !exists || !heldOutAuthorizationMatchesBatch(authorization, arm.Batch) {
			return fmt.Errorf("stress held-out preflight authorization plan for arm %q differs from its exact batch preview", armID)
		}
		armAttestations := make([]conformance.Attestation, 0, len(arm.Batch.CapabilityContractDigests))
		for _, contractDigest := range arm.Batch.CapabilityContractDigests {
			key := arm.Batch.RouteConfigDigest + "\x00" + contractDigest
			requiredAttestations[key] = struct{}{}
			attestation, exists := attestations[key]
			if !exists || attestation.Identity.RouteID != arm.Batch.RouteID || attestation.Identity.ProviderID != studyArm.ProviderID ||
				attestation.Identity.RequestedModel != studyArm.RequestedModel || attestation.StudyManifestDigest != "" {
				return fmt.Errorf("stress held-out preflight route attestation for arm %q differs from its exact route, contract, or study", armID)
			}
			state, reason := attestation.EffectiveState(verifiedAt, arm.Batch.RouteConfigDigest, contractDigest, "")
			minimumState := conformance.StateProbeCompatible
			if arm.Batch.EvidencePolicy == verification.EvidenceStrictVerifier {
				minimumState = conformance.StateBoundedQualified
			}
			if state.EvidenceRank() < minimumState.EvidenceRank() || reason != "" {
				return fmt.Errorf("stress held-out preflight route attestation for arm %q is below its required evidence capability", armID)
			}
			armAttestations = append(armAttestations, attestation)
		}
		attestationSetDigest, err := heldOutRouteAttestationSetDigest(armAttestations)
		if err != nil || studyArm.AttestationDigest != attestationSetDigest {
			return fmt.Errorf("stress held-out preflight study arm %q does not lock its exact route-attestation set", armID)
		}
	}
	if len(executionBindings) != len(liveArms) || len(authorizations) != len(liveArms) || len(attestations) != len(requiredAttestations) ||
		execution.AdmissionPlanDigest != admission.Digest {
		return errors.New("stress held-out preflight live evidence contains extra, missing, or cross-admission material")
	}
	return nil
}

func heldOutAuthorizationMatchesBatch(plan mode.AuthorizationPlan, binding verification.BatchPlanBinding) bool {
	if plan.Verify(plan.AuthorizationDigest) != nil || plan.AuthorizationDigest != binding.RequiredAuthorizationDigest ||
		plan.Entrypoint != binding.Entrypoint || plan.RouteID != binding.RouteID || string(plan.RequestFingerprint) != binding.RequestSetFingerprint ||
		plan.RequestContractDigest != binding.RequestContractDigest || plan.MaxWorkers != binding.MaxWorkers ||
		plan.MaxOutputTokens != binding.MaximumOutputTokensPerRequest || plan.ExpectedCalls != binding.WorstLogicalCalls ||
		plan.WorstCalls != binding.WorstLogicalCalls || plan.StudyManifestDigest != binding.StudyManifestDigest ||
		plan.Limits.MaxCalls != binding.Budget.MaxCalls || plan.Limits.MaxAttempts != binding.Budget.MaxAttempts ||
		plan.Limits.MaxEstimatedInputTokens != binding.Budget.MaxEstimatedInputTokens ||
		plan.Limits.MaxReservedOutputTokens != binding.Budget.MaxReservedOutputTokens ||
		plan.Limits.MaxConcurrent != binding.Budget.MaxConcurrent || plan.Limits.MaxCostUSD != binding.Budget.MaxCostUSD {
		return false
	}
	duration := time.Duration(binding.Budget.MaxDurationNanoseconds)
	return duration%time.Second == 0 && plan.Limits.MaxDurationSeconds == int64(duration/time.Second)
}

func earliestHeldOutRouteExpiry(values []conformance.Attestation) time.Time {
	earliest := values[0].ExpiresAt
	for _, value := range values[1:] {
		if value.ExpiresAt.Before(earliest) {
			earliest = value.ExpiresAt
		}
	}
	return earliest
}

func heldOutRouteAttestationSetDigest(values []conformance.Attestation) (string, error) {
	digests := make([]string, len(values))
	for index, value := range values {
		if err := value.ValidateIntegrity(); err != nil {
			return "", err
		}
		digests[index] = value.AttestationDigest
	}
	sort.Strings(digests)
	if len(digests) == 0 || !uniqueSortedStrings(digests) {
		return "", errors.New("stress held-out route-attestation set must be non-empty and unique")
	}
	return digestDocument(digests)
}

func heldOutPreflightEvidenceDigest(value HeldOutPreflightEvidence) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func heldOutPreflightCustodyDigest(value HeldOutPreflightCustody) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutPreflightSupportedClaim = "the private owner-custody parent, admission-filtered workload, authorized study record, exact execution bindings, current arm-appropriate route capabilities locked by that study, and authorization plans are verified without issuing execution authority"

var heldOutPreflightUnsupportedClaims = []string{
	"provided live authorization",
	"execution permit",
	"held-out execution",
	"provider response evidence",
	"empirical verifier reliability",
}
