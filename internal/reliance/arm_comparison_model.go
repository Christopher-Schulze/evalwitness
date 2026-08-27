package reliance

import (
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

const (
	RelianceArmComparisonSchemaVersion = "evalwitness.reliance-arm-comparison.v1"
	RelianceArmContrastPolicyVersion   = "evalwitness.reliance-arm-contrast-policy.v1"
	RelianceArmContrastDirection       = "comparator_minus_reference"
)

type RelianceArmContrastKind string

const (
	RelianceContrastEvidencePolicy RelianceArmContrastKind = "evidence_policy"
	RelianceContrastEntrypoint     RelianceArmContrastKind = "entrypoint"
	RelianceContrastModelFamily    RelianceArmContrastKind = "model_family"
	RelianceContrastProvider       RelianceArmContrastKind = "provider"
	RelianceContrastRoute          RelianceArmContrastKind = "route"
)

type RelianceArmContrastSupport string

const (
	RelianceArmContrastSupported   RelianceArmContrastSupport = "supported"
	RelianceArmContrastUnsupported RelianceArmContrastSupport = "unsupported"
)

type RelianceModelIdentityStatus string

const (
	RelianceModelIdentityAliasOnly                RelianceModelIdentityStatus = "alias_only"
	RelianceModelIdentityNamedFamilyEvidenceBound RelianceModelIdentityStatus = "named_family_evidence_bound"
)

type ReliancePairingStatus string

const (
	ReliancePairBothMissing    ReliancePairingStatus = "both_missing"
	ReliancePairComparatorOnly ReliancePairingStatus = "comparator_only"
	ReliancePairPaired         ReliancePairingStatus = "paired"
	ReliancePairReferenceOnly  ReliancePairingStatus = "reference_only"
)

type RelianceArmEvidence struct {
	ArmID                  string
	ModelFamilyID          string
	ModelIdentityStatus    RelianceModelIdentityStatus
	RouteAttestationDigest string
	Registration           ReliancePanelRegistration
	Executions             []EvidenceTaskPanelExecution
	Failures               []RelianceCellFailureReceipt
}

type RelianceArmContrastSpec struct {
	ContrastID      string                  `json:"contrast_id"`
	Kind            RelianceArmContrastKind `json:"kind"`
	ReferenceArmID  string                  `json:"reference_arm_id"`
	ComparatorArmID string                  `json:"comparator_arm_id"`
}

type RelianceArmSummary struct {
	ArmID                  string                      `json:"arm_id"`
	ModelFamilyID          string                      `json:"model_family_id"`
	ModelIdentityStatus    RelianceModelIdentityStatus `json:"model_identity_status"`
	RouteAttestationDigest string                      `json:"route_attestation_digest"`
	RegistrationDigest     string                      `json:"registration_digest"`
	CorpusDigest           string                      `json:"corpus_digest"`
	AnalysisDigest         string                      `json:"analysis_digest"`
	Arm                    RelianceAnalysisArm         `json:"arm"`
}

type ReliancePairingStatusCount struct {
	Status ReliancePairingStatus `json:"status"`
	Cells  int                   `json:"cells"`
}

type RelianceArmContrastOutcomeFit struct {
	OutcomeID       OutcomeID                    `json:"outcome_id"`
	Status          RelianceFitStatus            `json:"status"`
	RegisteredPairs int                          `json:"registered_pairs"`
	EligiblePairs   int                          `json:"eligible_pairs"`
	ExcludedFromFit int                          `json:"excluded_from_fit"`
	Reason          string                       `json:"reason,omitempty"`
	Fit             *stats.ClusteredFactorialFit `json:"fit,omitempty"`
}

type RelianceArmContrast struct {
	ContrastID          string                          `json:"contrast_id"`
	Kind                RelianceArmContrastKind         `json:"kind"`
	ReferenceArmID      string                          `json:"reference_arm_id"`
	ComparatorArmID     string                          `json:"comparator_arm_id"`
	Direction           string                          `json:"direction"`
	ChangedDimensions   []string                        `json:"changed_dimensions"`
	Support             RelianceArmContrastSupport      `json:"support"`
	Reason              string                          `json:"reason,omitempty"`
	RegisteredPairs     int                             `json:"registered_pairs"`
	EligiblePairs       int                             `json:"eligible_pairs"`
	PairingStatusCounts []ReliancePairingStatusCount    `json:"pairing_status_counts"`
	OutcomeFits         []RelianceArmContrastOutcomeFit `json:"outcome_fits"`
}

type RelianceArmComparison struct {
	SchemaVersion          string                `json:"schema_version"`
	CanonicalPolicy        string                `json:"canonical_policy"`
	ContrastPolicyVersion  string                `json:"contrast_policy_version"`
	PreregistrationDigest  string                `json:"preregistration_digest"`
	PreflightDigest        string                `json:"preflight_digest"`
	MultiplicityMethod     string                `json:"multiplicity_method"`
	MultiplicityFamilySize int                   `json:"multiplicity_family_size"`
	Arms                   []RelianceArmSummary  `json:"arms"`
	Contrasts              []RelianceArmContrast `json:"contrasts"`
	ProviderCalls          int                   `json:"provider_calls"`
	NetworkRequired        bool                  `json:"network_required"`
	Digest                 string                `json:"digest"`
}
