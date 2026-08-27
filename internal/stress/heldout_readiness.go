package stress

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
)

const (
	HeldOutRunReadinessRefusalSchemaVersion = "evalwitness.stress-held-out-run-readiness-refusal.v1"
	heldOutReadinessReceiptID               = "task-056-current-held-out-readiness-refusal"
	heldOutReadinessStatus                  = "not_ready"
	heldOutExternalActionStatus             = "not_authorized"
)

type HeldOutReadinessGateStatus string

const (
	HeldOutReadinessPassed  HeldOutReadinessGateStatus = "passed"
	HeldOutReadinessBlocked HeldOutReadinessGateStatus = "blocked"
	HeldOutReadinessMissing HeldOutReadinessGateStatus = "missing"
)

type HeldOutReadinessGate struct {
	ID             string                     `json:"id"`
	Required       bool                       `json:"required"`
	Status         HeldOutReadinessGateStatus `json:"status"`
	EvidenceDigest string                     `json:"evidence_digest,omitempty"`
	Reason         string                     `json:"reason"`
}

type HeldOutReadinessClaimBoundary struct {
	SupportedClaim    string   `json:"supported_claim"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

type HeldOutRunReadinessRefusal struct {
	SchemaVersion               string                        `json:"schema_version"`
	CanonicalPolicy             string                        `json:"canonical_policy"`
	ReceiptID                   string                        `json:"receipt_id"`
	PartitionDigest             string                        `json:"partition_digest"`
	RegistryDigest              string                        `json:"registry_digest"`
	ReleaseDigest               string                        `json:"release_digest"`
	ArmPlanDigest               string                        `json:"arm_plan_digest"`
	AnalysisDesignDigest        string                        `json:"analysis_design_digest"`
	OwnerAttestationDigest      string                        `json:"owner_attestation_digest"`
	OwnerPackageInventoryDigest string                        `json:"owner_package_inventory_digest"`
	ExpectedOwnerPackageDigest  string                        `json:"expected_owner_package_inventory_digest"`
	TestCases                   int                           `json:"test_cases"`
	TestCells                   int                           `json:"test_cells"`
	SupportedTestCells          int                           `json:"supported_test_cells"`
	UnsupportedTestCells        int                           `json:"unsupported_test_cells"`
	Gates                       []HeldOutReadinessGate        `json:"gates"`
	PassedGates                 int                           `json:"passed_gates"`
	BlockedGates                int                           `json:"blocked_gates"`
	MissingGates                int                           `json:"missing_gates"`
	Status                      string                        `json:"status"`
	RunAuthorized               bool                          `json:"run_authorized"`
	ExecutionPermitIssued       bool                          `json:"execution_permit_issued"`
	ExternalActionStatus        string                        `json:"external_action_status"`
	ProviderCalls               int                           `json:"provider_calls"`
	EmpiricalUnits              int                           `json:"empirical_units"`
	NetworkRequired             bool                          `json:"network_required"`
	ClaimBoundary               HeldOutReadinessClaimBoundary `json:"claim_boundary"`
	Digest                      string                        `json:"digest"`
}

func BuildHeldOutRunReadinessRefusal(
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	owner relationevidence.OwnerInspectionPublicAttestation,
	expectedOwnerPackageDigest string,
) (HeldOutRunReadinessRefusal, error) {
	if err := lock.ValidateAgainst(design, plan, registry, replayed); err != nil {
		return HeldOutRunReadinessRefusal{}, err
	}
	if err := owner.Validate(); err != nil {
		return HeldOutRunReadinessRefusal{}, fmt.Errorf("validate owner-inspection projection: %w", err)
	}
	if !validDigest(expectedOwnerPackageDigest) {
		return HeldOutRunReadinessRefusal{}, errors.New("expected owner package inventory digest is invalid")
	}
	value := heldOutReadinessRefusal(lock, design, plan, registry, owner, expectedOwnerPackageDigest)
	var err error
	value.Digest, err = heldOutRunReadinessRefusalDigest(value)
	if err != nil {
		return HeldOutRunReadinessRefusal{}, err
	}
	if err := value.ValidateAgainst(lock, design, plan, registry, replayed, owner, expectedOwnerPackageDigest); err != nil {
		return HeldOutRunReadinessRefusal{}, err
	}
	return value, nil
}

func (value HeldOutRunReadinessRefusal) Validate() error {
	if value.SchemaVersion != HeldOutRunReadinessRefusalSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.ReceiptID != heldOutReadinessReceiptID || !validDigest(value.PartitionDigest) || !validDigest(value.RegistryDigest) ||
		!validDigest(value.ReleaseDigest) || !validDigest(value.ArmPlanDigest) || !validDigest(value.AnalysisDesignDigest) ||
		!validDigest(value.OwnerAttestationDigest) || !validDigest(value.OwnerPackageInventoryDigest) || !validDigest(value.ExpectedOwnerPackageDigest) ||
		value.TestCases <= 0 || value.TestCells <= 0 || value.SupportedTestCells <= 0 || value.UnsupportedTestCells <= 0 ||
		value.TestCells != value.SupportedTestCells+value.UnsupportedTestCells {
		return errors.New("stress held-out readiness refusal identity, evidence, or partition counts are invalid")
	}
	passed, blocked, missing := 0, 0, 0
	for index, gate := range value.Gates {
		if index >= len(heldOutReadinessGateIDs) || gate.ID != heldOutReadinessGateIDs[index] || !gate.Required || strings.TrimSpace(gate.Reason) == "" {
			return errors.New("stress held-out readiness gates are incomplete, reordered, or unreasoned")
		}
		switch gate.Status {
		case HeldOutReadinessPassed:
			passed++
			if !validDigest(gate.EvidenceDigest) {
				return errors.New("passed stress held-out readiness gate lacks evidence")
			}
		case HeldOutReadinessBlocked:
			blocked++
			if !validDigest(gate.EvidenceDigest) {
				return errors.New("blocked stress held-out readiness gate lacks inspected evidence")
			}
		case HeldOutReadinessMissing:
			missing++
			if gate.EvidenceDigest != "" {
				return errors.New("missing stress held-out readiness gate fabricates evidence")
			}
		default:
			return errors.New("stress held-out readiness gate status is invalid")
		}
	}
	if len(value.Gates) != len(heldOutReadinessGateIDs) || passed != value.PassedGates || blocked != value.BlockedGates || missing != value.MissingGates ||
		passed+blocked+missing != len(value.Gates) || blocked+missing == 0 {
		return errors.New("stress held-out readiness gate totals or refusal state are invalid")
	}
	if value.Status != heldOutReadinessStatus || value.RunAuthorized || value.ExecutionPermitIssued ||
		value.ExternalActionStatus != heldOutExternalActionStatus || value.ProviderCalls != 0 || value.EmpiricalUnits != 0 || value.NetworkRequired ||
		value.ClaimBoundary.SupportedClaim != heldOutReadinessSupportedClaim || !slices.Equal(value.ClaimBoundary.UnsupportedClaims, heldOutReadinessUnsupportedClaims) {
		return errors.New("stress held-out readiness refusal promoted execution, evidence, or claims")
	}
	expectedDigest, err := heldOutRunReadinessRefusalDigest(value)
	if err != nil || value.Digest != expectedDigest {
		return errors.New("stress held-out readiness refusal digest is invalid")
	}
	return nil
}

func (value HeldOutRunReadinessRefusal) ValidateAgainst(
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	replayed []ReplayedRelationCaseV3,
	owner relationevidence.OwnerInspectionPublicAttestation,
	expectedOwnerPackageDigest string,
) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := lock.ValidateAgainst(design, plan, registry, replayed); err != nil {
		return err
	}
	if err := owner.Validate(); err != nil {
		return err
	}
	if !validDigest(expectedOwnerPackageDigest) {
		return errors.New("expected owner package inventory digest is invalid")
	}
	want := heldOutReadinessRefusal(lock, design, plan, registry, owner, expectedOwnerPackageDigest)
	var err error
	want.Digest, err = heldOutRunReadinessRefusalDigest(want)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, want) {
		return errors.New("stress held-out readiness refusal differs from the current partition or owner evidence")
	}
	return nil
}

func heldOutReadinessRefusal(
	lock HeldOutPartitionLock,
	design StressAnalysisDesign,
	plan ArmComparisonPlan,
	registry RelationRegistry,
	owner relationevidence.OwnerInspectionPublicAttestation,
	expectedOwnerPackageDigest string,
) HeldOutRunReadinessRefusal {
	ownerStatus, ownerReason := HeldOutReadinessPassed, "passed_owner_projection_contract"
	if err := validateOwnerCustody(owner, expectedOwnerPackageDigest); err != nil {
		ownerStatus = HeldOutReadinessBlocked
		ownerReason = "owner_projection_custody_contract_failed"
		if owner.Outcomes.OverallStatus != relationevidence.PilotInspectionOverallPassed {
			ownerReason = "owner_inspection_overall_status_" + string(owner.Outcomes.OverallStatus)
		}
	}
	gates := []HeldOutReadinessGate{
		{ID: "held_out_partition_lock", Required: true, Status: HeldOutReadinessPassed, EvidenceDigest: lock.Digest, Reason: "exact_locked_test_partition_validated"},
		{ID: "owner_inspection", Required: true, Status: ownerStatus, EvidenceDigest: owner.Digest, Reason: ownerReason},
		{ID: "blinded_human_admission", Required: true, Status: HeldOutReadinessMissing, Reason: "primary_audit_terminal_ledger_unavailable"},
		{ID: "authorized_study_record", Required: true, Status: HeldOutReadinessMissing, Reason: "controlled_relation_study_record_unavailable"},
		{ID: "execution_and_budget_binding", Required: true, Status: HeldOutReadinessMissing, Reason: "exact_multi_arm_execution_binding_unavailable"},
		{ID: "current_route_attestations", Required: true, Status: HeldOutReadinessMissing, Reason: "current_attestation_per_provider_arm_unavailable"},
		{ID: "live_authorization", Required: true, Status: HeldOutReadinessMissing, Reason: "exact_live_authorization_digest_unavailable"},
		{ID: "private_capsule_family", Required: true, Status: HeldOutReadinessMissing, Reason: "verified_private_owner_capsule_family_unavailable"},
	}
	passed, blocked, missing := heldOutReadinessGateCounts(gates)
	return HeldOutRunReadinessRefusal{
		SchemaVersion: HeldOutRunReadinessRefusalSchemaVersion, CanonicalPolicy: CanonicalPolicy, ReceiptID: heldOutReadinessReceiptID,
		PartitionDigest: lock.Digest, RegistryDigest: registry.Digest, ReleaseDigest: registry.ReleaseDigest,
		ArmPlanDigest: plan.Digest, AnalysisDesignDigest: design.Digest, OwnerAttestationDigest: owner.Digest,
		OwnerPackageInventoryDigest: owner.PackageInventoryDigest, ExpectedOwnerPackageDigest: expectedOwnerPackageDigest,
		TestCases: lock.TestCases, TestCells: lock.TestCells, SupportedTestCells: lock.SupportedTestCells, UnsupportedTestCells: lock.UnsupportedTestCells,
		Gates: gates, PassedGates: passed, BlockedGates: blocked, MissingGates: missing, Status: heldOutReadinessStatus,
		RunAuthorized: false, ExecutionPermitIssued: false, ExternalActionStatus: heldOutExternalActionStatus,
		ProviderCalls: 0, EmpiricalUnits: 0, NetworkRequired: false,
		ClaimBoundary: HeldOutReadinessClaimBoundary{SupportedClaim: heldOutReadinessSupportedClaim, UnsupportedClaims: slices.Clone(heldOutReadinessUnsupportedClaims)},
	}
}

func heldOutReadinessGateCounts(gates []HeldOutReadinessGate) (int, int, int) {
	passed, blocked, missing := 0, 0, 0
	for _, gate := range gates {
		switch gate.Status {
		case HeldOutReadinessPassed:
			passed++
		case HeldOutReadinessBlocked:
			blocked++
		case HeldOutReadinessMissing:
			missing++
		}
	}
	return passed, blocked, missing
}

func heldOutRunReadinessRefusalDigest(value HeldOutRunReadinessRefusal) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

var heldOutReadinessGateIDs = []string{
	"held_out_partition_lock",
	"owner_inspection",
	"blinded_human_admission",
	"authorized_study_record",
	"execution_and_budget_binding",
	"current_route_attestations",
	"live_authorization",
	"private_capsule_family",
}

const heldOutReadinessSupportedClaim = "the exact held-out partition and current owner projection were inspected provider-free, and the real run is not authorized"

var heldOutReadinessUnsupportedClaims = []string{
	"held-out execution",
	"verifier reliability",
	"provider quality",
	"population generalization",
	"execution authorization",
}
