package capsule

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	PrivateRelationRegistryID                    = "evalwitness.relation-owner-private-capsule-registry.v1"
	PrivateRelationPackageFileSchemaVersion      = "evalwitness.relation-owner-private-package-file.v1"
	PrivateRelationPackageReceiptSchemaVersion   = "evalwitness.relation-owner-private-package-receipt.v1"
	PrivateRelationEventChainSchemaVersion       = "evalwitness.relation-owner-inspection-event-chain.v1"
	PrivateRelationSourceCommitmentSchemaVersion = "evalwitness.relation-owner-private-source-commitment.v1"
	PrivateRelationProofSchemaVersion            = "evalwitness.relation-owner-private-proof.v1"

	privateRelationPackageFiles           = 53
	privateRelationFileValidatorID        = "evalwitness.validator.private-relation-file.v1"
	privateRelationReceiptValidatorID     = "evalwitness.validator.private-relation-receipt.v1"
	privateRelationEventChainValidatorID  = "evalwitness.validator.private-relation-event-chain.v1"
	privateRelationCommitmentValidatorID  = "evalwitness.validator.private-relation-source-commitment.v1"
	privateRelationSessionValidatorID     = "evalwitness.validator.private-relation-session.v1"
	privateRelationInspectionValidatorID  = "evalwitness.validator.private-relation-inspection.v1"
	privateRelationCompletionValidatorID  = "evalwitness.validator.private-relation-completion.v1"
	privateRelationProofValidatorID       = "evalwitness.validator.private-relation-proof.v1"
	privateRelationBindingValidatorID     = "evalwitness.validator.private-relation-bindings.v1"
	privateRelationSentinelCommitmentKind = "sentinel_material"
	privateRelationScarcityCommitmentKind = "owner_scarcity_inspection"
)

var privateRelationVerificationSteps = []string{
	"exact_package_inventory",
	"exact_package_payloads",
	"full_package_parent_reconstruction",
	"immutable_session",
	"ordered_append_only_event_chain",
	"seven_packet_inspection_reproduction",
	"combined_completion_reproduction",
	"closed_public_projection_reproduction",
}

type PrivateRelationFileBinding struct {
	Path          string `json:"path"`
	Mode          string `json:"mode"`
	Bytes         int64  `json:"bytes"`
	SHA256        string `json:"sha256"`
	ComponentID   string `json:"component_id"`
	PayloadSHA256 string `json:"payload_sha256"`
}

type PrivateRelationPackageReceipt struct {
	SchemaVersion               string                       `json:"schema_version"`
	PackageFormat               string                       `json:"package_format"`
	PackageInventoryDigest      string                       `json:"package_inventory_digest"`
	InventoryComponentID        string                       `json:"inventory_component_id"`
	PayloadFiles                int                          `json:"payload_files"`
	PayloadBytes                int64                        `json:"payload_bytes"`
	Files                       []PrivateRelationFileBinding `json:"files"`
	PublicCapsuleID             string                       `json:"public_capsule_id"`
	PublicCommitmentComponentID string                       `json:"public_commitment_component_id"`
	Digest                      string                       `json:"digest"`
}

type PrivateRelationEventPayload struct {
	Sequence      int    `json:"sequence"`
	Bytes         int64  `json:"bytes"`
	SHA256        string `json:"sha256"`
	PayloadBase64 string `json:"payload_base64"`
}

type PrivateRelationEventChain struct {
	SchemaVersion                string                        `json:"schema_version"`
	SessionDigest                string                        `json:"session_digest"`
	Events                       []PrivateRelationEventPayload `json:"events"`
	EventCount                   int                           `json:"event_count"`
	Corrections                  int                           `json:"corrections"`
	RequiredAssessments          int                           `json:"required_assessments"`
	CompletedAssessments         int                           `json:"completed_assessments"`
	CompletedCoreAssessments     int                           `json:"completed_core_assessments"`
	CompletedScarcityAssessments int                           `json:"completed_scarcity_assessments"`
	CompletedBoundaryAssessments int                           `json:"completed_boundary_assessments"`
	HeadDigest                   string                        `json:"head_digest"`
	ReadyToFinalize              bool                          `json:"ready_to_finalize"`
	Digest                       string                        `json:"digest"`
}

type PrivateRelationSourceCommitment struct {
	SchemaVersion     string `json:"schema_version"`
	Kind              string `json:"kind"`
	Ordinal           int    `json:"ordinal"`
	SourcePath        string `json:"source_path"`
	SourceComponentID string `json:"source_component_id"`
	SourceSHA256      string `json:"source_sha256"`
	Digest            string `json:"digest"`
}

type PrivateRelationProof struct {
	SchemaVersion                string                                `json:"schema_version"`
	PackageFormat                string                                `json:"package_format"`
	PackageInventoryDigest       string                                `json:"package_inventory_digest"`
	PackagePayloadFiles          int                                   `json:"package_payload_files"`
	SessionDigest                string                                `json:"session_digest"`
	EventCount                   int                                   `json:"event_count"`
	Corrections                  int                                   `json:"corrections"`
	RequiredAssessments          int                                   `json:"required_assessments"`
	CompletedAssessments         int                                   `json:"completed_assessments"`
	InspectionRecordDigest       string                                `json:"inspection_record_digest"`
	CompletionDigest             string                                `json:"completion_digest"`
	CoreStatus                   relation.PilotInspectionOverallStatus `json:"core_status"`
	ScarcityStatus               relation.PilotInspectionOverallStatus `json:"scarcity_status"`
	OverallStatus                relation.PilotInspectionOverallStatus `json:"overall_status"`
	HumanStudyStatus             string                                `json:"human_study_status"`
	ExternalActionStatus         relation.ExternalActionStatus         `json:"external_action_status"`
	PublicCapsuleID              string                                `json:"public_capsule_id"`
	PublicCommitmentComponentID  string                                `json:"public_commitment_component_id"`
	PublicAttestationComponentID string                                `json:"public_attestation_component_id"`
	PublicAttestationDigest      string                                `json:"public_attestation_digest"`
	VerificationSteps            []string                              `json:"verification_steps"`
	ProviderCalls                int                                   `json:"provider_calls"`
	Digest                       string                                `json:"digest"`
}

type PrivateRelationSources struct {
	InventoryPayload  []byte
	PackageFiles      map[string][]byte
	SessionPayload    []byte
	EventPayloads     [][]byte
	InspectionPayload []byte
	CompletionPayload []byte
	Chain             relation.OwnerInspectionPrivateChain
}

type PrivateRelationPackage struct {
	Registry    *Registry
	Manifest    Manifest
	Payloads    map[string][]byte
	Proof       PrivateRelationProof
	Attestation relation.OwnerInspectionPublicAttestation
}

func privateRelationTypes() []ComponentType {
	private := []Visibility{VisibilityPrivate}
	types := []ComponentType{
		{
			TypeID: PrivateRelationPackageFileSchemaVersion, SchemaID: PrivateRelationPackageFileSchemaVersion,
			Role: RoleObservation, AllowedVisibilities: private, MediaType: "application/octet-stream",
			PayloadProfile: PayloadExactBytes, ValidatorID: privateRelationFileValidatorID, ParentRules: []ParentRule{},
		},
		{
			TypeID: PrivateRelationPackageReceiptSchemaVersion, SchemaID: PrivateRelationPackageReceiptSchemaVersion,
			Role: RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: PayloadCanonicalJSON, ValidatorID: privateRelationReceiptValidatorID,
			BindingValidatorID: privateRelationBindingValidatorID,
			ParentRules: []ParentRule{
				parentRule(EdgeDerivedFrom, referencePrivateRelationInventoryType, 1, 1),
				parentRule(EdgeDerivedFrom, PrivateRelationPackageFileSchemaVersion, privateRelationPackageFiles, privateRelationPackageFiles),
				parentRuleWithResolutions(EdgeDerivedFrom, referencePrivateCommitmentType, 1, 1, ParentExternal),
			},
		},
		{
			TypeID: relation.PilotInspectionSessionSchemaVersion, SchemaID: relation.PilotInspectionSessionSchemaVersion,
			Role: RoleObservation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: PayloadExactBytes, ValidatorID: privateRelationSessionValidatorID,
			BindingValidatorID: privateRelationBindingValidatorID,
			ParentRules:        []ParentRule{parentRule(EdgeObservedFrom, referencePrivateRelationInventoryType, 1, 1)},
		},
		{
			TypeID: PrivateRelationEventChainSchemaVersion, SchemaID: PrivateRelationEventChainSchemaVersion,
			Role: RoleObservation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: PayloadCanonicalJSON, ValidatorID: privateRelationEventChainValidatorID,
			BindingValidatorID: privateRelationBindingValidatorID,
			ParentRules:        []ParentRule{parentRule(EdgeObservedFrom, relation.PilotInspectionSessionSchemaVersion, 1, 1)},
		},
		{
			TypeID: relation.PilotInspectionSchemaVersionV3, SchemaID: relation.PilotInspectionSchemaVersionV3,
			Role: RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: PayloadExactBytes, ValidatorID: privateRelationInspectionValidatorID,
			BindingValidatorID: privateRelationBindingValidatorID,
			ParentRules: []ParentRule{
				parentRule(EdgeDerivedFrom, PrivateRelationEventChainSchemaVersion, 1, 1),
				parentRule(EdgeDerivedFrom, relation.PilotInspectionSessionSchemaVersion, 1, 1),
			},
		},
		{
			TypeID: relation.PilotInspectionCompletionSchemaVersion, SchemaID: relation.PilotInspectionCompletionSchemaVersion,
			Role: RoleAttestation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: PayloadExactBytes, ValidatorID: privateRelationCompletionValidatorID,
			BindingValidatorID: privateRelationBindingValidatorID,
			ParentRules: []ParentRule{
				parentRule(EdgeAttests, PrivateRelationEventChainSchemaVersion, 1, 1),
				parentRule(EdgeAttests, PrivateRelationPackageReceiptSchemaVersion, 1, 1),
				parentRule(EdgeAttests, relation.PilotInspectionSchemaVersionV3, 1, 1),
				parentRule(EdgeAttests, relation.PilotInspectionSessionSchemaVersion, 1, 1),
			},
		},
		{
			TypeID: PrivateRelationSourceCommitmentSchemaVersion, SchemaID: PrivateRelationSourceCommitmentSchemaVersion,
			Role: RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: PayloadCanonicalJSON, ValidatorID: privateRelationCommitmentValidatorID,
			BindingValidatorID: privateRelationBindingValidatorID,
			ParentRules:        []ParentRule{parentRule(EdgeDerivedFrom, PrivateRelationPackageFileSchemaVersion, 1, 1)},
		},
		{
			TypeID: PrivateRelationProofSchemaVersion, SchemaID: PrivateRelationProofSchemaVersion,
			Role: RoleDerivation, AllowedVisibilities: private, MediaType: "application/json",
			PayloadProfile: PayloadCanonicalJSON, ValidatorID: privateRelationProofValidatorID,
			BindingValidatorID: privateRelationBindingValidatorID,
			ParentRules: []ParentRule{
				parentRule(EdgeDerivedFrom, PrivateRelationEventChainSchemaVersion, 1, 1),
				parentRule(EdgeDerivedFrom, PrivateRelationPackageFileSchemaVersion, privateRelationPackageFiles, privateRelationPackageFiles),
				parentRule(EdgeDerivedFrom, PrivateRelationPackageReceiptSchemaVersion, 1, 1),
				parentRule(EdgeDerivedFrom, PrivateRelationSourceCommitmentSchemaVersion, 4, 4),
				parentRule(EdgeDerivedFrom, referencePrivateRelationInventoryType, 1, 1),
				parentRule(EdgeDerivedFrom, relation.PilotInspectionCompletionSchemaVersion, 1, 1),
				parentRule(EdgeDerivedFrom, relation.PilotInspectionSchemaVersionV3, 1, 1),
				parentRule(EdgeDerivedFrom, relation.PilotInspectionSessionSchemaVersion, 1, 1),
				parentRuleWithResolutions(EdgeDerivedFrom, relation.OwnerInspectionPublicAttestationSchemaVersion, 1, 1, ParentExternal),
			},
		},
	}
	return types
}

func (receipt PrivateRelationPackageReceipt) Validate() error {
	if receipt.SchemaVersion != PrivateRelationPackageReceiptSchemaVersion || receipt.PackageFormat != relation.PilotPackageFormatV5 ||
		!validDigest(receipt.PackageInventoryDigest) || !validDigest(receipt.InventoryComponentID) ||
		receipt.PayloadFiles != privateRelationPackageFiles || receipt.PayloadFiles != len(receipt.Files) || receipt.PayloadBytes < 1 ||
		!validDigest(receipt.PublicCapsuleID) || !validDigest(receipt.PublicCommitmentComponentID) || !validDigest(receipt.Digest) {
		return errors.New("private relation package receipt identity or totals are invalid")
	}
	var total int64
	for index, file := range receipt.Files {
		if !validPrivateRelationPath(file.Path) || file.Mode != "0600" || file.Bytes < 1 || !validDigest(file.SHA256) ||
			!validDigest(file.ComponentID) || file.PayloadSHA256 != file.SHA256 ||
			index > 0 && receipt.Files[index-1].Path >= file.Path {
			return errors.New("private relation package receipt file binding is invalid or unsorted")
		}
		total += file.Bytes
	}
	if total != receipt.PayloadBytes {
		return errors.New("private relation package receipt byte total is invalid")
	}
	digest, err := privateRelationDigest(receipt)
	if err != nil || digest != receipt.Digest {
		return errors.New("private relation package receipt digest is invalid")
	}
	return nil
}

func (chain PrivateRelationEventChain) Validate() error {
	if chain.SchemaVersion != PrivateRelationEventChainSchemaVersion || !validDigest(chain.SessionDigest) ||
		chain.EventCount != len(chain.Events) || chain.EventCount < relation.PilotInspectionRequiredAssessments ||
		chain.Corrections != chain.EventCount-relation.PilotInspectionRequiredAssessments ||
		chain.RequiredAssessments != relation.PilotInspectionRequiredAssessments ||
		chain.CompletedAssessments != relation.PilotInspectionRequiredAssessments ||
		chain.CompletedCoreAssessments != relation.PilotInspectionCoreAssessments ||
		chain.CompletedScarcityAssessments != relation.PilotInspectionScarcityAssessments ||
		chain.CompletedBoundaryAssessments != relation.PilotInspectionBoundaryAssessments ||
		!validDigest(chain.HeadDigest) || !chain.ReadyToFinalize || !validDigest(chain.Digest) {
		return errors.New("private relation event-chain identity or assessment totals are invalid")
	}
	for index, event := range chain.Events {
		if event.Sequence != index+1 || event.Bytes < 1 || !validDigest(event.SHA256) || event.PayloadBase64 == "" {
			return errors.New("private relation event-chain payload binding is invalid or unordered")
		}
	}
	digest, err := privateRelationDigest(chain)
	if err != nil || digest != chain.Digest {
		return errors.New("private relation event-chain digest is invalid")
	}
	return nil
}

func (commitment PrivateRelationSourceCommitment) Validate() error {
	if commitment.SchemaVersion != PrivateRelationSourceCommitmentSchemaVersion || !validDigest(commitment.SourceComponentID) ||
		!validDigest(commitment.SourceSHA256) || !validDigest(commitment.Digest) {
		return errors.New("private relation source commitment identity is invalid")
	}
	switch commitment.Kind {
	case privateRelationSentinelCommitmentKind:
		if commitment.Ordinal < 1 || commitment.Ordinal > 3 || commitment.SourcePath != fmt.Sprintf("sentinel-materials/%02d.json", commitment.Ordinal) {
			return errors.New("private relation sentinel commitment is invalid")
		}
	case privateRelationScarcityCommitmentKind:
		if commitment.Ordinal != 0 || commitment.SourcePath != "owner-scarcity-inspection.md" {
			return errors.New("private relation scarcity-inspection commitment is invalid")
		}
	default:
		return errors.New("private relation source commitment kind is unknown")
	}
	digest, err := privateRelationDigest(commitment)
	if err != nil || digest != commitment.Digest {
		return errors.New("private relation source commitment digest is invalid")
	}
	return nil
}

func (proof PrivateRelationProof) Validate() error {
	validStatus := func(status relation.PilotInspectionOverallStatus) bool {
		return slices.Contains([]relation.PilotInspectionOverallStatus{
			relation.PilotInspectionOverallPassed,
			relation.PilotInspectionOverallRevisionRequired,
			relation.PilotInspectionOverallUnresolved,
		}, status)
	}
	if proof.SchemaVersion != PrivateRelationProofSchemaVersion || proof.PackageFormat != relation.PilotPackageFormatV5 ||
		!validDigest(proof.PackageInventoryDigest) || proof.PackagePayloadFiles != privateRelationPackageFiles ||
		!validDigest(proof.SessionDigest) || proof.EventCount < relation.PilotInspectionRequiredAssessments ||
		proof.Corrections != proof.EventCount-relation.PilotInspectionRequiredAssessments ||
		proof.RequiredAssessments != relation.PilotInspectionRequiredAssessments || proof.CompletedAssessments != relation.PilotInspectionRequiredAssessments ||
		!validDigest(proof.InspectionRecordDigest) || !validDigest(proof.CompletionDigest) ||
		!validStatus(proof.CoreStatus) || !validStatus(proof.ScarcityStatus) || !validStatus(proof.OverallStatus) ||
		proof.HumanStudyStatus != relation.PilotInspectionJournalHumanStudyStatus ||
		proof.ExternalActionStatus != relation.PilotInspectionJournalExternalAction ||
		!validDigest(proof.PublicCapsuleID) || !validDigest(proof.PublicCommitmentComponentID) ||
		!validDigest(proof.PublicAttestationComponentID) || !validDigest(proof.PublicAttestationDigest) ||
		!slices.Equal(proof.VerificationSteps, privateRelationVerificationSteps) || proof.ProviderCalls != 0 || !validDigest(proof.Digest) {
		return errors.New("private relation proof identity, status, or verification boundary is invalid")
	}
	digest, err := privateRelationDigest(proof)
	if err != nil || digest != proof.Digest {
		return errors.New("private relation proof digest is invalid")
	}
	return nil
}

func privateRelationDigest(value any) (string, error) {
	switch typed := value.(type) {
	case PrivateRelationPackageReceipt:
		typed.Digest = ""
		value = typed
	case PrivateRelationEventChain:
		typed.Digest = ""
		value = typed
	case PrivateRelationSourceCommitment:
		typed.Digest = ""
		value = typed
	case PrivateRelationProof:
		typed.Digest = ""
		value = typed
	default:
		return "", fmt.Errorf("unsupported private relation digest type %T", value)
	}
	return protocol.Digest(value)
}

func validPrivateRelationPath(value string) bool {
	return value != "" && !strings.HasPrefix(value, "/") && path.Clean(value) == value && value != "." && value != ".." && !strings.HasPrefix(value, "../")
}
