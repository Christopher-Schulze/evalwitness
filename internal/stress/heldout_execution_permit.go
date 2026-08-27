package stress

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
)

const (
	HeldOutExecutionPermitSchemaVersion = "evalwitness.stress-held-out-execution-permit.v2"
	heldOutExecutionPermitStatus        = "exact_held_out_execution_authorized_not_started"
	heldOutExecutionAuthorizedStatus    = "authorized"
	heldOutExecutionPermitUsePolicy     = "single_use_requires_atomic_execution_reservation"
	heldOutExecutionPermitSealPolicy    = "completed_execution_requires_exact_held_out_run_seal"
	heldOutReservationAuthorityKind     = "local_owner_cache_root"
	heldOutReservationBackendVersion    = "evalwitness.exclusive-link-reservation.v1"
)

type HeldOutExecutionReservationAuthority struct {
	Kind           string `json:"kind"`
	AuthorityID    string `json:"authority_id"`
	BackendVersion string `json:"backend_version"`
}

func NewHeldOutExecutionReservationAuthority(root *safety.CacheRoot) (HeldOutExecutionReservationAuthority, error) {
	if root == nil || !validHeldOutReservationAuthorityID(root.RootID()) {
		return HeldOutExecutionReservationAuthority{}, errors.New("stress held-out execution reservation requires an initialized owner-only authority root")
	}
	return HeldOutExecutionReservationAuthority{
		Kind: heldOutReservationAuthorityKind, AuthorityID: root.RootID(), BackendVersion: heldOutReservationBackendVersion,
	}, nil
}

func (value HeldOutExecutionReservationAuthority) Validate() error {
	if value.Kind != heldOutReservationAuthorityKind || !validHeldOutReservationAuthorityID(value.AuthorityID) ||
		value.BackendVersion != heldOutReservationBackendVersion {
		return errors.New("stress held-out execution reservation authority is invalid")
	}
	return nil
}

type HeldOutExecutionPermitArm struct {
	ArmID                       string                          `json:"arm_id"`
	EvidencePolicy              verification.EvidencePolicy     `json:"evidence_policy"`
	EligibleTestCellSetDigest   string                          `json:"eligible_test_cell_set_digest"`
	EligibleTestCells           int                             `json:"eligible_test_cells"`
	VerificationInputs          int                             `json:"verification_inputs"`
	InputContractDigest         string                          `json:"input_contract_digest"`
	BatchPlanBindingDigest      string                          `json:"batch_plan_binding_digest"`
	RequestSetFingerprint       string                          `json:"request_set_fingerprint"`
	RequestContractDigest       string                          `json:"request_contract_digest"`
	CapabilityContractSetDigest string                          `json:"capability_contract_set_digest"`
	RouteID                     string                          `json:"route_id"`
	RouteConfigDigest           string                          `json:"route_config_digest"`
	WorstLogicalCalls           int                             `json:"worst_logical_calls"`
	Budget                      verification.BatchBudgetBinding `json:"budget"`
	RequiredAuthorizationDigest string                          `json:"required_authorization_digest"`
	ProvidedAuthorizationDigest string                          `json:"provided_authorization_digest"`
}

type HeldOutExecutionPermit struct {
	SchemaVersion                      string                               `json:"schema_version"`
	CanonicalPolicy                    string                               `json:"canonical_policy"`
	PreflightCapsuleID                 string                               `json:"preflight_capsule_id"`
	PreflightCapsuleManifestDigest     string                               `json:"preflight_capsule_manifest_digest"`
	PreflightCapsuleRegistryDigest     string                               `json:"preflight_capsule_registry_digest"`
	PreflightCustodyDigest             string                               `json:"preflight_custody_digest"`
	PreflightEvidenceDigest            string                               `json:"preflight_evidence_digest"`
	PrivateRelationCapsuleID           string                               `json:"private_relation_capsule_id"`
	CampaignDigest                     string                               `json:"campaign_digest"`
	AdmissionPlanDigest                string                               `json:"admission_plan_digest"`
	ExecutionBatchBindingDigest        string                               `json:"execution_batch_binding_digest"`
	PartitionDigest                    string                               `json:"partition_digest"`
	RegistryDigest                     string                               `json:"registry_digest"`
	ReleaseDigest                      string                               `json:"release_digest"`
	ArmPlanDigest                      string                               `json:"arm_plan_digest"`
	AnalysisDesignDigest               string                               `json:"analysis_design_digest"`
	StudyRecordDigest                  string                               `json:"study_record_digest"`
	StudyManifestDigest                string                               `json:"study_manifest_digest"`
	ProfilePolicyDigest                string                               `json:"profile_policy_digest"`
	ExecutionBindingDigests            []string                             `json:"execution_binding_digests"`
	RouteAttestationDigests            []string                             `json:"route_attestation_digests"`
	RequiredAuthorizationDigests       []string                             `json:"required_authorization_digests"`
	ProvidedAuthorizationDigests       []string                             `json:"provided_authorization_digests"`
	Arms                               []HeldOutExecutionPermitArm          `json:"arms"`
	LiveProviderEligibleCells          int                                  `json:"live_provider_eligible_cells"`
	LiveVerificationInputs             int                                  `json:"live_verification_inputs"`
	LiveWorstLogicalCalls              int                                  `json:"live_worst_logical_calls"`
	LiveBudget                         verification.BatchBudgetBinding      `json:"live_budget"`
	ReservationAuthority               HeldOutExecutionReservationAuthority `json:"reservation_authority"`
	IssuedAt                           string                               `json:"issued_at"`
	ExpiresAt                          string                               `json:"expires_at"`
	UsePolicy                          string                               `json:"use_policy"`
	CompletionPolicy                   string                               `json:"completion_policy"`
	SingleUse                          bool                                 `json:"single_use"`
	AtomicExecutionReservationRequired bool                                 `json:"atomic_execution_reservation_required"`
	RunSealRequired                    bool                                 `json:"run_seal_required"`
	Status                             string                               `json:"status"`
	ExternalActionStatus               string                               `json:"external_action_status"`
	RunAuthorized                      bool                                 `json:"run_authorized"`
	ExecutionPermitIssued              bool                                 `json:"execution_permit_issued"`
	ExecutionStarted                   bool                                 `json:"execution_started"`
	ProviderCalls                      int                                  `json:"provider_calls"`
	EmpiricalUnits                     int                                  `json:"empirical_units"`
	NetworkPerformed                   bool                                 `json:"network_performed"`
	NetworkRequiredForExecution        bool                                 `json:"network_required_for_execution"`
	ClaimBoundary                      HeldOutCampaignClaimBoundary         `json:"claim_boundary"`
	Digest                             string                               `json:"digest"`
}

func BuildHeldOutExecutionPermit(
	ctx context.Context,
	preflight capsule.Package,
	privateRelation capsule.Package,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
	providedAuthorizationDigests []string,
	reservationAuthority HeldOutExecutionReservationAuthority,
	issuedAt time.Time,
) (HeldOutExecutionPermit, error) {
	if ctx == nil || issuedAt.IsZero() {
		return HeldOutExecutionPermit{}, errors.New("stress held-out execution permit requires context and an issue time")
	}
	custody, err := VerifyHeldOutPreflightCapsule(ctx, preflight, privateRelation, admission, execution, evidence)
	if err != nil {
		return HeldOutExecutionPermit{}, err
	}
	issuedAt = issuedAt.UTC()
	if err := validateHeldOutPermitWindow(custody, issuedAt); err != nil {
		return HeldOutExecutionPermit{}, err
	}
	if err := verifyHeldOutPreflightLiveEvidence(admission, execution, evidence, privateRelation.Manifest.CapsuleID, issuedAt); err != nil {
		return HeldOutExecutionPermit{}, err
	}
	if err := reservationAuthority.Validate(); err != nil {
		return HeldOutExecutionPermit{}, err
	}
	provided, err := verifyHeldOutProvidedAuthorizations(execution, evidence.AuthorizationPlans, providedAuthorizationDigests)
	if err != nil {
		return HeldOutExecutionPermit{}, err
	}
	value, err := buildHeldOutExecutionPermit(preflight, custody, admission, execution, evidence, provided, reservationAuthority, issuedAt)
	if err != nil {
		return HeldOutExecutionPermit{}, err
	}
	if err := VerifyHeldOutExecutionPermit(
		ctx, value, preflight, privateRelation, admission, execution, evidence, provided, reservationAuthority, issuedAt,
	); err != nil {
		return HeldOutExecutionPermit{}, err
	}
	return value, nil
}

func VerifyHeldOutExecutionPermit(
	ctx context.Context,
	value HeldOutExecutionPermit,
	preflight capsule.Package,
	privateRelation capsule.Package,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
	providedAuthorizationDigests []string,
	reservationAuthority HeldOutExecutionReservationAuthority,
	now time.Time,
) error {
	if ctx == nil || now.IsZero() {
		return errors.New("stress held-out execution permit verification requires context and a current time")
	}
	if err := value.ValidateAt(now); err != nil {
		return err
	}
	custody, err := VerifyHeldOutPreflightCapsule(ctx, preflight, privateRelation, admission, execution, evidence)
	if err != nil {
		return err
	}
	issuedAt, err := parseHeldOutExecutionPermitTime(value.IssuedAt)
	if err != nil {
		return err
	}
	if err := validateHeldOutPermitWindow(custody, issuedAt); err != nil {
		return err
	}
	if err := verifyHeldOutPreflightLiveEvidence(admission, execution, evidence, privateRelation.Manifest.CapsuleID, issuedAt); err != nil {
		return err
	}
	if err := verifyHeldOutPreflightLiveEvidence(admission, execution, evidence, privateRelation.Manifest.CapsuleID, now.UTC()); err != nil {
		return err
	}
	if err := reservationAuthority.Validate(); err != nil {
		return err
	}
	provided, err := verifyHeldOutProvidedAuthorizations(execution, evidence.AuthorizationPlans, providedAuthorizationDigests)
	if err != nil {
		return err
	}
	want, err := buildHeldOutExecutionPermit(preflight, custody, admission, execution, evidence, provided, reservationAuthority, issuedAt)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out execution permit differs from its exact capsule, custody, workload, route, budget, or authorization parents")
	}
	return nil
}

func (value HeldOutExecutionPermit) Validate() error {
	digests := []string{
		value.PreflightCapsuleID, value.PreflightCapsuleManifestDigest, value.PreflightCapsuleRegistryDigest,
		value.PreflightCustodyDigest, value.PreflightEvidenceDigest, value.PrivateRelationCapsuleID,
		value.CampaignDigest, value.AdmissionPlanDigest, value.ExecutionBatchBindingDigest, value.PartitionDigest,
		value.RegistryDigest, value.ReleaseDigest, value.ArmPlanDigest, value.AnalysisDesignDigest,
		value.StudyRecordDigest, value.StudyManifestDigest, value.ProfilePolicyDigest,
	}
	if value.SchemaVersion != HeldOutExecutionPermitSchemaVersion || value.CanonicalPolicy != CanonicalPolicy {
		return errors.New("stress held-out execution permit identity is invalid")
	}
	for _, digest := range digests {
		if !validDigest(digest) {
			return errors.New("stress held-out execution permit lineage digest is invalid")
		}
	}
	for _, values := range [][]string{
		value.ExecutionBindingDigests, value.RouteAttestationDigests,
		value.RequiredAuthorizationDigests, value.ProvidedAuthorizationDigests,
	} {
		if len(values) == 0 || !slices.IsSorted(values) || !uniqueSortedStrings(values) {
			return errors.New("stress held-out execution permit digest sets must be non-empty, unique, and sorted")
		}
		for _, digest := range values {
			if !validDigest(digest) {
				return errors.New("stress held-out execution permit digest set contains an invalid digest")
			}
		}
	}
	if len(value.ExecutionBindingDigests) != 2 || len(value.RouteAttestationDigests) < 2 ||
		len(value.RequiredAuthorizationDigests) != 2 || !slices.Equal(value.RequiredAuthorizationDigests, value.ProvidedAuthorizationDigests) {
		return errors.New("stress held-out execution permit lacks the exact two-arm execution, route, or authorization evidence")
	}
	issuedAt, issueErr := parseHeldOutExecutionPermitTime(value.IssuedAt)
	expiresAt, expiryErr := parseHeldOutExecutionPermitTime(value.ExpiresAt)
	if issueErr != nil || expiryErr != nil || !issuedAt.Before(expiresAt) {
		return errors.New("stress held-out execution permit validity window is invalid")
	}
	if err := value.LiveBudget.Validate(); err != nil {
		return fmt.Errorf("stress held-out execution permit live budget: %w", err)
	}
	if err := value.ReservationAuthority.Validate(); err != nil {
		return err
	}
	if err := validateHeldOutExecutionPermitArms(value); err != nil {
		return err
	}
	if value.UsePolicy != heldOutExecutionPermitUsePolicy || value.CompletionPolicy != heldOutExecutionPermitSealPolicy ||
		!value.SingleUse || !value.AtomicExecutionReservationRequired || !value.RunSealRequired ||
		value.Status != heldOutExecutionPermitStatus || value.ExternalActionStatus != heldOutExecutionAuthorizedStatus ||
		!value.RunAuthorized || !value.ExecutionPermitIssued || value.ExecutionStarted || value.ProviderCalls != 0 ||
		value.EmpiricalUnits != 0 || value.NetworkPerformed || !value.NetworkRequiredForExecution ||
		value.ClaimBoundary.SupportedClaim != heldOutExecutionPermitSupportedClaim ||
		!slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutExecutionPermitUnsupportedClaims) {
		return errors.New("stress held-out execution permit weakens single-use controls or fabricates execution, observation, or claims")
	}
	expected, err := heldOutExecutionPermitDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress held-out execution permit digest is invalid")
	}
	return nil
}

func (value HeldOutExecutionPermit) ValidateAt(now time.Time) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if now.IsZero() {
		return errors.New("stress held-out execution permit requires a current time")
	}
	issuedAt, err := parseHeldOutExecutionPermitTime(value.IssuedAt)
	if err != nil {
		return err
	}
	expiresAt, err := parseHeldOutExecutionPermitTime(value.ExpiresAt)
	if err != nil {
		return err
	}
	now = now.UTC()
	if now.Before(issuedAt) || !now.Before(expiresAt) {
		return errors.New("stress held-out execution permit is not yet valid or has expired")
	}
	return nil
}

func buildHeldOutExecutionPermit(
	preflight capsule.Package,
	custody HeldOutPreflightCustody,
	admission HeldOutAdmissionPlan,
	execution HeldOutExecutionBatchBinding,
	evidence HeldOutPreflightEvidence,
	providedAuthorizationDigests []string,
	reservationAuthority HeldOutExecutionReservationAuthority,
	issuedAt time.Time,
) (HeldOutExecutionPermit, error) {
	value := HeldOutExecutionPermit{
		SchemaVersion: HeldOutExecutionPermitSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PreflightCapsuleID: preflight.Manifest.CapsuleID, PreflightCapsuleManifestDigest: preflight.Manifest.ManifestDigest,
		PreflightCapsuleRegistryDigest: preflight.Registry.Digest(), PreflightCustodyDigest: custody.Digest,
		PreflightEvidenceDigest: evidence.Digest, PrivateRelationCapsuleID: custody.PrivateRelationCapsuleID,
		CampaignDigest: admission.CampaignDigest, AdmissionPlanDigest: admission.Digest, ExecutionBatchBindingDigest: execution.Digest,
		PartitionDigest: execution.PartitionDigest, RegistryDigest: execution.RegistryDigest, ReleaseDigest: execution.ReleaseDigest,
		ArmPlanDigest: execution.ArmPlanDigest, AnalysisDesignDigest: execution.AnalysisDesignDigest,
		StudyRecordDigest: evidence.StudyRecord.RecordDigest, StudyManifestDigest: execution.StudyManifestDigest,
		ProfilePolicyDigest:          execution.ProfilePolicyDigest,
		ExecutionBindingDigests:      slices.Clone(custody.ExecutionBindingDigests),
		RouteAttestationDigests:      slices.Clone(custody.RouteAttestationDigests),
		RequiredAuthorizationDigests: slices.Clone(execution.RequiredAuthorizationDigests),
		ProvidedAuthorizationDigests: slices.Clone(providedAuthorizationDigests),
		LiveProviderEligibleCells:    execution.LiveProviderEligibleCells, LiveVerificationInputs: execution.LiveVerificationInputs,
		LiveBudget: execution.LiveBudget, ReservationAuthority: reservationAuthority,
		IssuedAt: formatHeldOutExecutionPermitTime(issuedAt), ExpiresAt: custody.RouteAttestationsExpireAt,
		UsePolicy: heldOutExecutionPermitUsePolicy, CompletionPolicy: heldOutExecutionPermitSealPolicy,
		SingleUse: true, AtomicExecutionReservationRequired: true, RunSealRequired: true,
		Status: heldOutExecutionPermitStatus, ExternalActionStatus: heldOutExecutionAuthorizedStatus,
		RunAuthorized: true, ExecutionPermitIssued: true, NetworkRequiredForExecution: true,
		ClaimBoundary: HeldOutCampaignClaimBoundary{
			SupportedClaim: heldOutExecutionPermitSupportedClaim, UnsupportedClaims: slices.Clone(heldOutExecutionPermitUnsupportedClaims),
		},
	}
	provided := stringSet(providedAuthorizationDigests)
	for _, arm := range execution.Arms {
		if arm.ExecutionClass != HeldOutExecutionLiveProvider {
			continue
		}
		providedDigest := arm.Batch.RequiredAuthorizationDigest
		if _, exists := provided[providedDigest]; !exists {
			return HeldOutExecutionPermit{}, fmt.Errorf("stress held-out execution permit lacks provided authorization for arm %q", arm.ArmID)
		}
		value.Arms = append(value.Arms, HeldOutExecutionPermitArm{
			ArmID: arm.ArmID, EvidencePolicy: arm.Batch.EvidencePolicy,
			EligibleTestCellSetDigest: arm.EligibleTestCellSetDigest, EligibleTestCells: arm.EligibleTestCells,
			VerificationInputs: arm.VerificationInputs, InputContractDigest: arm.InputContractDigest,
			BatchPlanBindingDigest: arm.Batch.Digest, RequestSetFingerprint: arm.Batch.RequestSetFingerprint,
			RequestContractDigest: arm.Batch.RequestContractDigest, CapabilityContractSetDigest: arm.Batch.CapabilityContractSetDigest,
			RouteID: arm.Batch.RouteID, RouteConfigDigest: arm.Batch.RouteConfigDigest,
			WorstLogicalCalls: arm.Batch.WorstLogicalCalls, Budget: arm.Batch.Budget,
			RequiredAuthorizationDigest: arm.Batch.RequiredAuthorizationDigest, ProvidedAuthorizationDigest: providedDigest,
		})
		value.LiveWorstLogicalCalls += arm.Batch.WorstLogicalCalls
	}
	sort.Slice(value.Arms, func(left, right int) bool { return value.Arms[left].ArmID < value.Arms[right].ArmID })
	value.Digest = ""
	digest, err := heldOutExecutionPermitDigest(value)
	if err != nil {
		return HeldOutExecutionPermit{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return HeldOutExecutionPermit{}, err
	}
	return value, nil
}

func validateHeldOutExecutionPermitArms(value HeldOutExecutionPermit) error {
	if len(value.Arms) != 2 {
		return errors.New("stress held-out execution permit must contain exactly two live arms")
	}
	previousArm := ""
	cells, inputs, calls := 0, 0, 0
	var budget verification.BatchBudgetBinding
	authorizations := make([]string, 0, len(value.Arms))
	for _, arm := range value.Arms {
		if strings.TrimSpace(arm.ArmID) == "" || arm.ArmID <= previousArm ||
			(arm.EvidencePolicy != verification.EvidenceStrictVerifier && arm.EvidencePolicy != verification.EvidenceExplicitJudge) ||
			!validDigest(arm.EligibleTestCellSetDigest) || arm.EligibleTestCells <= 0 ||
			arm.VerificationInputs != arm.EligibleTestCells*heldOutProviderSidesPerCell || !validDigest(arm.InputContractDigest) ||
			!validDigest(arm.BatchPlanBindingDigest) || !validDigest(arm.RequestSetFingerprint) ||
			!validDigest(arm.RequestContractDigest) || !validDigest(arm.CapabilityContractSetDigest) ||
			!strings.HasPrefix(arm.RouteID, "route-") || !validDigest(strings.TrimPrefix(arm.RouteID, "route-")) ||
			!validDigest(arm.RouteConfigDigest) || arm.WorstLogicalCalls <= 0 ||
			!validDigest(arm.RequiredAuthorizationDigest) || arm.RequiredAuthorizationDigest != arm.ProvidedAuthorizationDigest {
			return errors.New("stress held-out execution permit arm identity, workload, route, or authorization is invalid")
		}
		if err := arm.Budget.Validate(); err != nil || arm.WorstLogicalCalls > arm.Budget.MaxCalls {
			return errors.New("stress held-out execution permit arm budget is invalid")
		}
		previousArm = arm.ArmID
		cells += arm.EligibleTestCells
		inputs += arm.VerificationInputs
		calls += arm.WorstLogicalCalls
		budget = addHeldOutBatchBudgets(budget, arm.Budget)
		authorizations = append(authorizations, arm.RequiredAuthorizationDigest)
	}
	sort.Strings(authorizations)
	if cells != value.LiveProviderEligibleCells || inputs != value.LiveVerificationInputs || calls != value.LiveWorstLogicalCalls ||
		!reflect.DeepEqual(budget, value.LiveBudget) || !slices.Equal(authorizations, value.RequiredAuthorizationDigests) {
		return errors.New("stress held-out execution permit arms do not reproduce the aggregate live workload, budget, or authorization set")
	}
	return nil
}

func verifyHeldOutProvidedAuthorizations(
	execution HeldOutExecutionBatchBinding,
	plans []mode.AuthorizationPlan,
	providedAuthorizationDigests []string,
) ([]string, error) {
	provided := slices.Clone(providedAuthorizationDigests)
	sort.Strings(provided)
	if len(provided) != 2 || !uniqueSortedStrings(provided) || !slices.Equal(provided, execution.RequiredAuthorizationDigests) {
		return nil, errors.New("stress held-out execution permit requires the exact two authorization digests from the execution binding")
	}
	byDigest := make(map[string]mode.AuthorizationPlan, len(plans))
	for _, plan := range plans {
		byDigest[plan.AuthorizationDigest] = plan
	}
	if len(byDigest) != len(plans) || len(byDigest) != len(provided) {
		return nil, errors.New("stress held-out execution permit authorization plan set is incomplete or duplicated")
	}
	for _, digest := range provided {
		plan, exists := byDigest[digest]
		if !exists {
			return nil, errors.New("stress held-out execution permit lacks an exact authorization plan")
		}
		if err := plan.Verify(digest); err != nil {
			return nil, err
		}
	}
	return provided, nil
}

func validateHeldOutPermitWindow(custody HeldOutPreflightCustody, issuedAt time.Time) error {
	verifiedAt, err := parseHeldOutPreflightTime(custody.VerifiedAt)
	if err != nil {
		return err
	}
	expiresAt, err := parseHeldOutPreflightTime(custody.RouteAttestationsExpireAt)
	if err != nil {
		return err
	}
	issuedAt = issuedAt.UTC()
	if issuedAt.Before(verifiedAt) || !issuedAt.Before(expiresAt) {
		return errors.New("stress held-out execution permit issue time precedes custody or reaches the route-attestation expiry")
	}
	return nil
}

func formatHeldOutExecutionPermitTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseHeldOutExecutionPermitTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || value != formatHeldOutExecutionPermitTime(parsed) {
		return time.Time{}, errors.New("stress held-out execution permit time is invalid")
	}
	return parsed, nil
}

func heldOutExecutionPermitDigest(value HeldOutExecutionPermit) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

const heldOutExecutionPermitSupportedClaim = "the exact private preflight capsule and two explicit authorization digests authorize one admission-filtered two-arm held-out execution window without claiming that execution started"

var heldOutExecutionPermitUnsupportedClaims = []string{
	"held-out execution started or completed",
	"provider response evidence",
	"empirical verifier reliability",
	"atomic single-use reservation completed",
	"held-out run seal",
}
