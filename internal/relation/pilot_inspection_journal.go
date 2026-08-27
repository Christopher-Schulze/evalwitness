package relation

import (
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	PilotPackageInventorySchemaVersion     = "evalwitness.relation-pilot-package-inventory.v1"
	PilotPackageFormatV5                   = "evalwitness.relation-pilot-package.v5"
	PilotPackageInventoryHashAlgorithm     = "sha256"
	PilotPackageInventoryScope             = "all_package_payload_files_except_inventory_and_sha256sums"
	PilotInspectionSessionSchemaVersion    = "evalwitness.relation-pilot-inspection-session.v1"
	PilotInspectionEventSchemaVersion      = "evalwitness.relation-pilot-inspection-event.v1"
	PilotInspectionCompletionSchemaVersion = "evalwitness.relation-pilot-inspection-completion.v1"
	PilotInspectionJournalProtocol         = "evalwitness.relation-owner-inspection-journal.v1"
	PilotInspectionOwnerConfirmation       = "explicit_owner_assessment"
	PilotInspectionCompletionConfirmation  = "explicit_owner_completion"
	PilotInspectionJournalHumanStudyStatus = PilotInspectionHumanNotRun
	PilotInspectionJournalExternalAction   = ExternalActionNotAuthorized
	PilotInspectionScarcityBoundaryID      = "scarcity-boundary"
	PilotInspectionCoreAssessments         = 50
	PilotInspectionScarcityAssessments     = 12
	PilotInspectionBoundaryAssessments     = 4
	PilotInspectionRequiredAssessments     = PilotInspectionCoreAssessments + PilotInspectionScarcityAssessments + PilotInspectionBoundaryAssessments
)

type PilotPackageInventoryDirectory struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type PilotPackageInventoryFile struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

type PilotPackageInventory struct {
	SchemaVersion string                           `json:"schema_version"`
	PackageFormat string                           `json:"package_format"`
	HashAlgorithm string                           `json:"hash_algorithm"`
	Scope         string                           `json:"scope"`
	Directories   []PilotPackageInventoryDirectory `json:"directories"`
	Files         []PilotPackageInventoryFile      `json:"files"`
	PayloadFiles  int                              `json:"payload_files"`
	PayloadBytes  int64                            `json:"payload_bytes"`
	Digest        string                           `json:"digest"`
}

type PilotInspectionPackageBinding struct {
	PackageFormat          string `json:"package_format"`
	PackageInventoryDigest string `json:"package_inventory_digest"`
	ReadinessSHA256        string `json:"readiness_sha256"`
	BundleSHA256           string `json:"bundle_sha256"`
	MappingsSHA256         string `json:"mappings_sha256"`
	WorkbookSHA256         string `json:"workbook_sha256"`
	ChangeAtlasSHA256      string `json:"change_atlas_sha256"`
	ScarcitySentinelSHA256 string `json:"scarcity_sentinel_sha256"`
	ScarcityAppendixSHA256 string `json:"scarcity_appendix_sha256"`
}

type PilotInspectionDimension string

type PilotInspectionSubjectKind string

const (
	PilotInspectionSubjectCorePacket       PilotInspectionSubjectKind = "core_packet"
	PilotInspectionSubjectScarcityCase     PilotInspectionSubjectKind = "scarcity_case"
	PilotInspectionSubjectScarcityBoundary PilotInspectionSubjectKind = "scarcity_boundary"
)

const (
	PilotInspectionDimensionTaskContext                    PilotInspectionDimension = "task_context"
	PilotInspectionDimensionEvidenceAlignment              PilotInspectionDimension = "evidence_alignment"
	PilotInspectionDimensionTransformationIsolation        PilotInspectionDimension = "transformation_isolation"
	PilotInspectionDimensionInformationSufficiency         PilotInspectionDimension = "information_sufficiency"
	PilotInspectionDimensionBlindingIntegrity              PilotInspectionDimension = "blinding_integrity"
	PilotInspectionDimensionRubricApplicability            PilotInspectionDimension = "rubric_applicability"
	PilotInspectionDimensionRedistributionBoundary         PilotInspectionDimension = "redistribution_boundary"
	PilotInspectionDimensionCandidateOrder                 PilotInspectionDimension = "candidate_order"
	PilotInspectionDimensionScarcityOriginalEvidence       PilotInspectionDimension = "scarcity_original_evidence"
	PilotInspectionDimensionScarcityTargetOmission         PilotInspectionDimension = "scarcity_target_omission"
	PilotInspectionDimensionScarcityRelationPreservation   PilotInspectionDimension = "scarcity_relation_preservation"
	PilotInspectionDimensionScarcityInformationSufficiency PilotInspectionDimension = "scarcity_information_sufficiency"
	PilotInspectionDimensionScarcityExhaustiveScope        PilotInspectionDimension = "scarcity_exhaustive_scope"
	PilotInspectionDimensionScarcityRoleIntegrity          PilotInspectionDimension = "scarcity_role_integrity"
	PilotInspectionDimensionScarcityEstimandSeparation     PilotInspectionDimension = "scarcity_estimand_separation"
	PilotInspectionDimensionScarcityNonAuthorization       PilotInspectionDimension = "scarcity_non_authorization"
)

type PilotInspectionSessionPacket struct {
	PacketID      string          `json:"packet_id"`
	PacketDigest  string          `json:"packet_digest"`
	MappingDigest string          `json:"mapping_digest"`
	Family        mutation.Family `json:"family"`
	Unit          UnitType        `json:"unit"`
}

type PilotInspectionSessionScarcityCase struct {
	CaseID                  string `json:"case_id"`
	MaterialDigest          string `json:"material_digest"`
	DataRole                string `json:"data_role"`
	ConstructFirewallDigest string `json:"construct_firewall_digest"`
}

type PilotInspectionSession struct {
	SchemaVersion              string                               `json:"schema_version"`
	CanonicalPolicy            string                               `json:"canonical_policy"`
	JournalProtocol            string                               `json:"journal_protocol"`
	RelationProtocol           string                               `json:"relation_protocol"`
	Objective                  ReviewObjective                      `json:"review_objective"`
	Package                    PilotInspectionPackageBinding        `json:"package"`
	ReadinessDigest            string                               `json:"readiness_digest"`
	BundleDigest               string                               `json:"bundle_digest"`
	MappingCommitment          string                               `json:"mapping_commitment_digest"`
	InspectorAlias             string                               `json:"inspector_alias"`
	Packets                    []PilotInspectionSessionPacket       `json:"packets"`
	ScarcitySentinelDigest     string                               `json:"scarcity_sentinel_digest"`
	ScarcityCases              []PilotInspectionSessionScarcityCase `json:"scarcity_cases"`
	CoreDimensions             []PilotInspectionDimension           `json:"core_dimensions"`
	ScarcityCaseDimensions     []PilotInspectionDimension           `json:"scarcity_case_dimensions"`
	ScarcityBoundaryDimensions []PilotInspectionDimension           `json:"scarcity_boundary_dimensions"`
	HumanStudyStatus           string                               `json:"human_study_status"`
	ExternalActionStatus       ExternalActionStatus                 `json:"external_action_status"`
	CreatedAt                  string                               `json:"created_at"`
	Digest                     string                               `json:"digest"`
}

type PilotInspectionEvent struct {
	SchemaVersion     string                     `json:"schema_version"`
	CanonicalPolicy   string                     `json:"canonical_policy"`
	JournalProtocol   string                     `json:"journal_protocol"`
	SessionDigest     string                     `json:"session_digest"`
	Sequence          int                        `json:"sequence"`
	PreviousDigest    string                     `json:"previous_digest"`
	SubjectKind       PilotInspectionSubjectKind `json:"subject_kind"`
	SubjectID         string                     `json:"subject_id"`
	Dimension         PilotInspectionDimension   `json:"dimension"`
	Assessment        PilotInspectionAssessment  `json:"assessment"`
	SupersedesDigest  string                     `json:"supersedes_digest"`
	OwnerConfirmation string                     `json:"owner_confirmation"`
	RecordedAt        string                     `json:"recorded_at"`
	Digest            string                     `json:"digest"`
}

type PilotInspectionTarget struct {
	SubjectKind PilotInspectionSubjectKind `json:"subject_kind"`
	SubjectID   string                     `json:"subject_id"`
	Dimension   PilotInspectionDimension   `json:"dimension"`
}

type PilotInspectionScarcitySummary struct {
	CaseID                 string                     `json:"case_id"`
	DataRole               string                     `json:"data_role"`
	OriginalEvidence       PilotInspectionAssessment  `json:"original_evidence"`
	TargetOmission         PilotInspectionAssessment  `json:"target_omission"`
	RelationPreservation   PilotInspectionAssessment  `json:"relation_preservation"`
	InformationSufficiency PilotInspectionAssessment  `json:"information_sufficiency"`
	Disposition            PilotInspectionDisposition `json:"disposition"`
}

type PilotInspectionScarcityBoundarySummary struct {
	ExhaustiveScope    PilotInspectionAssessment  `json:"exhaustive_scope"`
	RoleIntegrity      PilotInspectionAssessment  `json:"role_integrity"`
	EstimandSeparation PilotInspectionAssessment  `json:"estimand_separation"`
	NonAuthorization   PilotInspectionAssessment  `json:"non_authorization"`
	Disposition        PilotInspectionDisposition `json:"disposition"`
}

type PilotInspectionDecisionSummary struct {
	PacketID    string                     `json:"packet_id"`
	ReasonCodes []PilotInspectionReason    `json:"reason_codes"`
	Disposition PilotInspectionDisposition `json:"disposition"`
}

type PilotInspectionJournalStatus struct {
	SessionDigest                string                                  `json:"session_digest"`
	Events                       int                                     `json:"events"`
	Corrections                  int                                     `json:"corrections"`
	RequiredAssessments          int                                     `json:"required_assessments"`
	CompletedAssessments         int                                     `json:"completed_assessments"`
	CompletedCoreAssessments     int                                     `json:"completed_core_assessments"`
	CompletedScarcityAssessments int                                     `json:"completed_scarcity_assessments"`
	CompletedBoundaryAssessments int                                     `json:"completed_boundary_assessments"`
	HeadDigest                   string                                  `json:"head_digest"`
	ReadyToFinalize              bool                                    `json:"ready_to_finalize"`
	Next                         *PilotInspectionTarget                  `json:"next,omitempty"`
	DecisionSummaries            []PilotInspectionDecisionSummary        `json:"decision_summaries,omitempty"`
	ScarcitySummaries            []PilotInspectionScarcitySummary        `json:"scarcity_summaries,omitempty"`
	ScarcityBoundary             *PilotInspectionScarcityBoundarySummary `json:"scarcity_boundary,omitempty"`
	latest                       map[string]PilotInspectionEvent
}

type PilotInspectionCompletion struct {
	SchemaVersion          string                                 `json:"schema_version"`
	CanonicalPolicy        string                                 `json:"canonical_policy"`
	JournalProtocol        string                                 `json:"journal_protocol"`
	SessionDigest          string                                 `json:"session_digest"`
	PackageInventoryDigest string                                 `json:"package_inventory_digest"`
	EventCount             int                                    `json:"event_count"`
	HeadDigest             string                                 `json:"head_digest"`
	RequiredAssessments    int                                    `json:"required_assessments"`
	DecisionSummaries      []PilotInspectionDecisionSummary       `json:"decision_summaries"`
	ScarcitySummaries      []PilotInspectionScarcitySummary       `json:"scarcity_summaries"`
	ScarcityBoundary       PilotInspectionScarcityBoundarySummary `json:"scarcity_boundary"`
	InspectionRecordDigest string                                 `json:"inspection_record_digest"`
	CoreStatus             PilotInspectionOverallStatus           `json:"core_status"`
	ScarcityStatus         PilotInspectionOverallStatus           `json:"scarcity_status"`
	OverallStatus          PilotInspectionOverallStatus           `json:"overall_status"`
	HumanStudyStatus       string                                 `json:"human_study_status"`
	ExternalActionStatus   ExternalActionStatus                   `json:"external_action_status"`
	OwnerConfirmation      string                                 `json:"owner_confirmation"`
	InspectedAt            string                                 `json:"inspected_at"`
	Digest                 string                                 `json:"digest"`
}

func PilotInspectionCoreDimensions() []PilotInspectionDimension {
	return []PilotInspectionDimension{
		PilotInspectionDimensionTaskContext,
		PilotInspectionDimensionEvidenceAlignment,
		PilotInspectionDimensionTransformationIsolation,
		PilotInspectionDimensionInformationSufficiency,
		PilotInspectionDimensionBlindingIntegrity,
		PilotInspectionDimensionRubricApplicability,
		PilotInspectionDimensionRedistributionBoundary,
		PilotInspectionDimensionCandidateOrder,
	}
}

func PilotInspectionScarcityCaseDimensions() []PilotInspectionDimension {
	return []PilotInspectionDimension{
		PilotInspectionDimensionScarcityOriginalEvidence,
		PilotInspectionDimensionScarcityTargetOmission,
		PilotInspectionDimensionScarcityRelationPreservation,
		PilotInspectionDimensionScarcityInformationSufficiency,
	}
}

func PilotInspectionScarcityBoundaryDimensions() []PilotInspectionDimension {
	return []PilotInspectionDimension{
		PilotInspectionDimensionScarcityExhaustiveScope,
		PilotInspectionDimensionScarcityRoleIntegrity,
		PilotInspectionDimensionScarcityEstimandSeparation,
		PilotInspectionDimensionScarcityNonAuthorization,
	}
}

func PilotInspectionDimensions() []PilotInspectionDimension {
	dimensions := append([]PilotInspectionDimension(nil), PilotInspectionCoreDimensions()...)
	dimensions = append(dimensions, PilotInspectionScarcityCaseDimensions()...)
	return append(dimensions, PilotInspectionScarcityBoundaryDimensions()...)
}

func DecodePilotPackageInventory(reader io.Reader) (PilotPackageInventory, error) {
	var value PilotPackageInventory
	if err := decodeStrict(reader, &value); err != nil {
		return PilotPackageInventory{}, fmt.Errorf("decode relation pilot package inventory: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotInspectionSession(reader io.Reader) (PilotInspectionSession, error) {
	var value PilotInspectionSession
	if err := decodeStrict(reader, &value); err != nil {
		return PilotInspectionSession{}, fmt.Errorf("decode relation pilot inspection session: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotInspectionEvent(reader io.Reader) (PilotInspectionEvent, error) {
	var value PilotInspectionEvent
	if err := decodeStrict(reader, &value); err != nil {
		return PilotInspectionEvent{}, fmt.Errorf("decode relation pilot inspection event: %w", err)
	}
	return value, value.Validate()
}

func DecodePilotInspectionCompletion(reader io.Reader) (PilotInspectionCompletion, error) {
	var value PilotInspectionCompletion
	if err := decodeStrict(reader, &value); err != nil {
		return PilotInspectionCompletion{}, fmt.Errorf("decode relation pilot inspection completion: %w", err)
	}
	return value, value.Validate()
}

func ValidatePilotInspectionTarget(session PilotInspectionSession, target PilotInspectionTarget) error {
	if err := session.Validate(); err != nil {
		return err
	}
	if !inspectionTargetApplies(session, target.SubjectKind, target.SubjectID, target.Dimension) {
		return errors.New("guided owner inspection target is unknown or its dimension is inapplicable")
	}
	return nil
}

func (inventory PilotPackageInventory) Validate() error {
	if inventory.SchemaVersion != PilotPackageInventorySchemaVersion || inventory.PackageFormat != PilotPackageFormatV5 ||
		inventory.HashAlgorithm != PilotPackageInventoryHashAlgorithm || inventory.Scope != PilotPackageInventoryScope ||
		inventory.PayloadFiles != len(inventory.Files) || inventory.PayloadFiles == 0 || !validDigest(inventory.Digest) {
		return errors.New("relation pilot package inventory identity or totals are invalid")
	}
	seenDirectories := make(map[string]struct{}, len(inventory.Directories))
	for index, directory := range inventory.Directories {
		if !validInventoryPath(directory.Path) || directory.Mode != "0700" || index > 0 && inventory.Directories[index-1].Path >= directory.Path {
			return errors.New("relation pilot package inventory directory entry is invalid or unsorted")
		}
		if _, duplicate := seenDirectories[directory.Path]; duplicate {
			return errors.New("relation pilot package inventory repeats a directory")
		}
		seenDirectories[directory.Path] = struct{}{}
	}
	seenFiles := make(map[string]struct{}, len(inventory.Files))
	var payloadBytes int64
	for index, file := range inventory.Files {
		if !validInventoryPath(file.Path) || file.Bytes < 0 || file.Mode != "0600" || !validDigest(file.SHA256) ||
			index > 0 && inventory.Files[index-1].Path >= file.Path || file.Path == "package-inventory.json" || file.Path == "SHA256SUMS" {
			return errors.New("relation pilot package inventory file entry is invalid or unsorted")
		}
		if _, duplicate := seenFiles[file.Path]; duplicate {
			return errors.New("relation pilot package inventory repeats a file")
		}
		seenFiles[file.Path] = struct{}{}
		payloadBytes += file.Bytes
	}
	if payloadBytes != inventory.PayloadBytes {
		return errors.New("relation pilot package inventory byte total is invalid")
	}
	expected, err := pilotPackageInventoryDigest(inventory)
	if err != nil || inventory.Digest != expected {
		return errors.New("relation pilot package inventory digest is invalid")
	}
	return nil
}

func BuildPilotInspectionSession(readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping, plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, pilot PilotSampleV3, scarcityMaterials []CaseMaterial, packageBinding PilotInspectionPackageBinding, inspectorAlias, createdAt string) (PilotInspectionSession, error) {
	if err := VerifyRelationPilotReadiness(readiness, bundle, mappings); err != nil {
		return PilotInspectionSession{}, err
	}
	if readiness.ProtocolVersion != ProtocolVersionV3 {
		return PilotInspectionSession{}, errors.New("guided owner inspection requires the protocol-v3 relation package")
	}
	if err := packageBinding.validate(); err != nil {
		return PilotInspectionSession{}, err
	}
	if err := pilot.Validate(plan, primary, sentinel); err != nil {
		return PilotInspectionSession{}, err
	}
	if readiness.PlanDigest != plan.Digest || readiness.PilotSampleDigest != pilot.Digest || pilot.ScarcitySentinelDigest != sentinel.Digest {
		return PilotInspectionSession{}, errors.New("guided owner inspection core readiness and scarcity sentinel do not share the governed v3 sample chain")
	}
	if err := VerifyScarcityInspectionMaterials(plan, primary, sentinel, scarcityMaterials); err != nil {
		return PilotInspectionSession{}, err
	}
	if strings.TrimSpace(inspectorAlias) == "" {
		return PilotInspectionSession{}, errors.New("guided owner inspection requires a pseudonymous inspector alias")
	}
	if err := requireRelationTimeAfter("guided owner inspection session", createdAt, readiness.PreparedAt); err != nil {
		return PilotInspectionSession{}, err
	}
	packets := make([]PilotInspectionSessionPacket, len(readiness.PacketChecks))
	for index, check := range readiness.PacketChecks {
		packets[index] = PilotInspectionSessionPacket{
			PacketID: check.PacketID, PacketDigest: check.PacketDigest, MappingDigest: check.MappingDigest,
			Family: check.Family, Unit: check.Unit,
		}
	}
	sort.Slice(packets, func(left, right int) bool { return packets[left].PacketID < packets[right].PacketID })
	materialByCase := make(map[string]CaseMaterial, len(scarcityMaterials))
	for _, material := range scarcityMaterials {
		materialByCase[material.CaseID] = material
	}
	scarcityCases := make([]PilotInspectionSessionScarcityCase, len(sentinel.Cases))
	for index, reference := range sentinel.Cases {
		scarcityCases[index] = PilotInspectionSessionScarcityCase{
			CaseID: reference.CaseID, MaterialDigest: materialByCase[reference.CaseID].Digest,
			DataRole: string(reference.DataRole), ConstructFirewallDigest: reference.ConstructFirewallDigest,
		}
	}
	sort.Slice(scarcityCases, func(left, right int) bool { return scarcityCases[left].CaseID < scarcityCases[right].CaseID })
	session := PilotInspectionSession{
		SchemaVersion: PilotInspectionSessionSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		JournalProtocol: PilotInspectionJournalProtocol, RelationProtocol: ProtocolVersionV3,
		Objective: ReviewObjectiveControlledRelation, Package: packageBinding,
		ReadinessDigest: readiness.Digest, BundleDigest: bundle.Digest, MappingCommitment: readiness.MappingCommitmentDigest,
		InspectorAlias: strings.TrimSpace(inspectorAlias), Packets: packets,
		ScarcitySentinelDigest: sentinel.Digest, ScarcityCases: scarcityCases,
		CoreDimensions: PilotInspectionCoreDimensions(), ScarcityCaseDimensions: PilotInspectionScarcityCaseDimensions(),
		ScarcityBoundaryDimensions: PilotInspectionScarcityBoundaryDimensions(),
		HumanStudyStatus:           PilotInspectionJournalHumanStudyStatus, ExternalActionStatus: PilotInspectionJournalExternalAction,
		CreatedAt: createdAt,
	}
	digest, err := pilotInspectionSessionDigest(session)
	if err != nil {
		return PilotInspectionSession{}, err
	}
	session.Digest = digest
	return session, session.Validate()
}

func (session PilotInspectionSession) Validate() error {
	if session.SchemaVersion != PilotInspectionSessionSchemaVersion || session.CanonicalPolicy != CanonicalPolicy ||
		session.JournalProtocol != PilotInspectionJournalProtocol || session.RelationProtocol != ProtocolVersionV3 ||
		session.Objective != ReviewObjectiveControlledRelation || session.Package.validate() != nil || !validDigest(session.ReadinessDigest) ||
		!validDigest(session.BundleDigest) || !validDigest(session.MappingCommitment) || strings.TrimSpace(session.InspectorAlias) == "" ||
		len(session.Packets) != 7 || !validDigest(session.ScarcitySentinelDigest) || len(session.ScarcityCases) != 3 ||
		!slices.Equal(session.CoreDimensions, PilotInspectionCoreDimensions()) ||
		!slices.Equal(session.ScarcityCaseDimensions, PilotInspectionScarcityCaseDimensions()) ||
		!slices.Equal(session.ScarcityBoundaryDimensions, PilotInspectionScarcityBoundaryDimensions()) ||
		session.HumanStudyStatus != PilotInspectionJournalHumanStudyStatus || session.ExternalActionStatus != PilotInspectionJournalExternalAction || !validDigest(session.Digest) {
		return errors.New("guided owner inspection session identity, package binding, or claim boundary is invalid")
	}
	if _, err := time.Parse(time.RFC3339, session.CreatedAt); err != nil {
		return errors.New("guided owner inspection session time must be RFC3339")
	}
	seen := make(map[string]struct{}, len(session.Packets))
	seenFamilies := make(map[mutation.Family]struct{}, len(session.Packets))
	seenMappings := make(map[string]struct{}, len(session.Packets))
	for index, packet := range session.Packets {
		definition, exists := mutation.DefinitionFor(packet.Family)
		expectedUnit := UnitTrajectoryPair
		if exists && definition.PairLevel {
			expectedUnit = UnitCandidatePairOrders
		}
		if !exists || packet.Unit != expectedUnit || !validOpaqueID(packet.PacketID, "relation-packet-") || !validDigest(packet.PacketDigest) ||
			!validDigest(packet.MappingDigest) || index > 0 && session.Packets[index-1].PacketID >= packet.PacketID {
			return errors.New("guided owner inspection session packet binding is invalid or unsorted")
		}
		if _, duplicate := seen[packet.PacketID]; duplicate {
			return errors.New("guided owner inspection session repeats a packet")
		}
		if _, duplicate := seenFamilies[packet.Family]; duplicate {
			return errors.New("guided owner inspection session repeats a relation family")
		}
		if _, duplicate := seenMappings[packet.MappingDigest]; duplicate {
			return errors.New("guided owner inspection session repeats a private mapping")
		}
		seen[packet.PacketID] = struct{}{}
		seenFamilies[packet.Family] = struct{}{}
		seenMappings[packet.MappingDigest] = struct{}{}
	}
	seenScarcity := make(map[string]struct{}, len(session.ScarcityCases))
	seenScarcityMaterials := make(map[string]struct{}, len(session.ScarcityCases))
	seenScarcityFirewalls := make(map[string]struct{}, len(session.ScarcityCases))
	roleCounts := map[string]int{"development": 0, "calibration": 0}
	for index, item := range session.ScarcityCases {
		if !validOpaqueID(item.CaseID, "mutation-") || !validDigest(item.MaterialDigest) || !validDigest(item.ConstructFirewallDigest) ||
			index > 0 && session.ScarcityCases[index-1].CaseID >= item.CaseID {
			return errors.New("guided owner inspection scarcity-case binding is invalid or unsorted")
		}
		if _, duplicate := seenScarcity[item.CaseID]; duplicate {
			return errors.New("guided owner inspection session repeats a scarcity case")
		}
		if _, duplicate := seenScarcityMaterials[item.MaterialDigest]; duplicate {
			return errors.New("guided owner inspection session repeats a scarcity material")
		}
		if _, duplicate := seenScarcityFirewalls[item.ConstructFirewallDigest]; duplicate {
			return errors.New("guided owner inspection session repeats a scarcity construct firewall")
		}
		if _, exists := roleCounts[item.DataRole]; !exists {
			return errors.New("guided owner inspection scarcity case has an inadmissible data role")
		}
		seenScarcity[item.CaseID] = struct{}{}
		seenScarcityMaterials[item.MaterialDigest] = struct{}{}
		seenScarcityFirewalls[item.ConstructFirewallDigest] = struct{}{}
		roleCounts[item.DataRole]++
	}
	if roleCounts["development"] != 2 || roleCounts["calibration"] != 1 {
		return errors.New("guided owner inspection scarcity cases do not preserve the frozen 2-development/1-calibration/0-test boundary")
	}
	expected, err := pilotInspectionSessionDigest(session)
	if err != nil || session.Digest != expected {
		return errors.New("guided owner inspection session digest is invalid")
	}
	return nil
}

func VerifyPilotInspectionSession(session PilotInspectionSession, readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping, plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, pilot PilotSampleV3, scarcityMaterials []CaseMaterial, packageBinding PilotInspectionPackageBinding) error {
	if err := session.Validate(); err != nil {
		return err
	}
	expected, err := BuildPilotInspectionSession(readiness, bundle, mappings, plan, primary, sentinel, pilot, scarcityMaterials, packageBinding, session.InspectorAlias, session.CreatedAt)
	if err != nil {
		return err
	}
	if session.Digest != expected.Digest {
		return errors.New("guided owner inspection session does not bind the supplied package parents")
	}
	return nil
}

func BuildPilotInspectionEvent(session PilotInspectionSession, events []PilotInspectionEvent, subjectKind PilotInspectionSubjectKind, subjectID string, dimension PilotInspectionDimension, assessment PilotInspectionAssessment, recordedAt string, correction bool) (PilotInspectionEvent, error) {
	status, err := VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return PilotInspectionEvent{}, err
	}
	if !inspectionTargetApplies(session, subjectKind, subjectID, dimension) {
		return PilotInspectionEvent{}, errors.New("guided owner inspection target is unknown or its dimension is inapplicable")
	}
	if !slices.Contains([]PilotInspectionAssessment{PilotInspectionPassed, PilotInspectionFailed, PilotInspectionIndeterminate}, assessment) {
		return PilotInspectionEvent{}, errors.New("guided owner inspection assessment must be passed, failed, or indeterminate")
	}
	key := inspectionEventKey(subjectKind, subjectID, dimension)
	previousForDimension, alreadyAnswered := status.latest[key]
	if alreadyAnswered != correction {
		if alreadyAnswered {
			return PilotInspectionEvent{}, errors.New("guided owner inspection dimension is already answered; an explicit correction is required")
		}
		return PilotInspectionEvent{}, errors.New("guided owner inspection correction requires an existing assessment")
	}
	lastTime := session.CreatedAt
	if len(events) > 0 {
		lastTime = events[len(events)-1].RecordedAt
	}
	if err := requireRelationTimeAfter("guided owner inspection event", recordedAt, lastTime); err != nil {
		return PilotInspectionEvent{}, err
	}
	previousDigest := session.Digest
	if len(events) > 0 {
		previousDigest = events[len(events)-1].Digest
	}
	supersedes := ""
	if alreadyAnswered {
		supersedes = previousForDimension.Digest
	}
	event := PilotInspectionEvent{
		SchemaVersion: PilotInspectionEventSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		JournalProtocol: PilotInspectionJournalProtocol, SessionDigest: session.Digest,
		Sequence: len(events) + 1, PreviousDigest: previousDigest, SubjectKind: subjectKind, SubjectID: subjectID, Dimension: dimension,
		Assessment: assessment, SupersedesDigest: supersedes, OwnerConfirmation: PilotInspectionOwnerConfirmation,
		RecordedAt: recordedAt,
	}
	digest, err := pilotInspectionEventDigest(event)
	if err != nil {
		return PilotInspectionEvent{}, err
	}
	event.Digest = digest
	return event, event.Validate()
}

func (event PilotInspectionEvent) Validate() error {
	if event.SchemaVersion != PilotInspectionEventSchemaVersion || event.CanonicalPolicy != CanonicalPolicy || event.JournalProtocol != PilotInspectionJournalProtocol ||
		!validDigest(event.SessionDigest) || event.Sequence < 1 || !validDigest(event.PreviousDigest) ||
		!slices.Contains([]PilotInspectionSubjectKind{PilotInspectionSubjectCorePacket, PilotInspectionSubjectScarcityCase, PilotInspectionSubjectScarcityBoundary}, event.SubjectKind) || strings.TrimSpace(event.SubjectID) == "" ||
		!slices.Contains(PilotInspectionDimensions(), event.Dimension) ||
		!slices.Contains([]PilotInspectionAssessment{PilotInspectionPassed, PilotInspectionFailed, PilotInspectionIndeterminate}, event.Assessment) ||
		event.SupersedesDigest != "" && !validDigest(event.SupersedesDigest) || event.OwnerConfirmation != PilotInspectionOwnerConfirmation || !validDigest(event.Digest) {
		return errors.New("guided owner inspection event identity, assessment, or confirmation is invalid")
	}
	if _, err := time.Parse(time.RFC3339, event.RecordedAt); err != nil {
		return errors.New("guided owner inspection event time must be RFC3339")
	}
	expected, err := pilotInspectionEventDigest(event)
	if err != nil || event.Digest != expected {
		return errors.New("guided owner inspection event digest is invalid")
	}
	return nil
}

func VerifyPilotInspectionJournal(session PilotInspectionSession, events []PilotInspectionEvent) (PilotInspectionJournalStatus, error) {
	if err := session.Validate(); err != nil {
		return PilotInspectionJournalStatus{}, err
	}
	latest := make(map[string]PilotInspectionEvent)
	previousDigest, previousTime := session.Digest, session.CreatedAt
	corrections := 0
	for index, event := range events {
		if err := event.Validate(); err != nil {
			return PilotInspectionJournalStatus{}, fmt.Errorf("guided owner inspection event %d: %w", index+1, err)
		}
		if !inspectionTargetApplies(session, event.SubjectKind, event.SubjectID, event.Dimension) || event.SessionDigest != session.Digest || event.Sequence != index+1 || event.PreviousDigest != previousDigest {
			return PilotInspectionJournalStatus{}, errors.New("guided owner inspection event chain is reordered, truncated, cross-session, or inapplicable")
		}
		recorded, _ := time.Parse(time.RFC3339, event.RecordedAt)
		previousRecorded, _ := time.Parse(time.RFC3339, previousTime)
		if !recorded.After(previousRecorded) {
			return PilotInspectionJournalStatus{}, errors.New("guided owner inspection event times are not strictly increasing")
		}
		key := inspectionEventKey(event.SubjectKind, event.SubjectID, event.Dimension)
		prior, exists := latest[key]
		if exists {
			if event.SupersedesDigest != prior.Digest {
				return PilotInspectionJournalStatus{}, errors.New("guided owner inspection correction does not supersede the latest assessment")
			}
			corrections++
		} else if event.SupersedesDigest != "" {
			return PilotInspectionJournalStatus{}, errors.New("guided owner inspection first assessment cannot supersede an event")
		}
		latest[key] = event
		previousDigest, previousTime = event.Digest, event.RecordedAt
	}
	required := requiredInspectionAssessments(session)
	coreCompleted, scarcityCompleted, boundaryCompleted := completedInspectionAssessments(latest)
	status := PilotInspectionJournalStatus{
		SessionDigest: session.Digest, Events: len(events), Corrections: corrections, RequiredAssessments: required,
		CompletedAssessments: len(latest), CompletedCoreAssessments: coreCompleted,
		CompletedScarcityAssessments: scarcityCompleted, CompletedBoundaryAssessments: boundaryCompleted,
		HeadDigest: previousDigest, ReadyToFinalize: len(latest) == required, latest: latest,
	}
	for _, packet := range session.Packets {
		for _, dimension := range session.CoreDimensions {
			if !coreDimensionApplies(packet, dimension) {
				continue
			}
			if _, exists := latest[inspectionEventKey(PilotInspectionSubjectCorePacket, packet.PacketID, dimension)]; !exists {
				status.Next = &PilotInspectionTarget{SubjectKind: PilotInspectionSubjectCorePacket, SubjectID: packet.PacketID, Dimension: dimension}
				return status, nil
			}
		}
	}
	for _, item := range session.ScarcityCases {
		for _, dimension := range session.ScarcityCaseDimensions {
			if _, exists := latest[inspectionEventKey(PilotInspectionSubjectScarcityCase, item.CaseID, dimension)]; !exists {
				status.Next = &PilotInspectionTarget{SubjectKind: PilotInspectionSubjectScarcityCase, SubjectID: item.CaseID, Dimension: dimension}
				return status, nil
			}
		}
	}
	for _, dimension := range session.ScarcityBoundaryDimensions {
		if _, exists := latest[inspectionEventKey(PilotInspectionSubjectScarcityBoundary, PilotInspectionScarcityBoundaryID, dimension)]; !exists {
			status.Next = &PilotInspectionTarget{SubjectKind: PilotInspectionSubjectScarcityBoundary, SubjectID: PilotInspectionScarcityBoundaryID, Dimension: dimension}
			return status, nil
		}
	}
	drafts, err := pilotInspectionDraftsFromStatus(session, status)
	if err != nil {
		return PilotInspectionJournalStatus{}, err
	}
	status.DecisionSummaries = pilotInspectionSummaries(session, drafts)
	status.ScarcitySummaries, status.ScarcityBoundary, err = pilotInspectionScarcitySummaries(session, status)
	if err != nil {
		return PilotInspectionJournalStatus{}, err
	}
	return status, nil
}

func BuildPilotInspectionCompletion(session PilotInspectionSession, events []PilotInspectionEvent, readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping, inspectedAt string) (PilotInspectionRecord, PilotInspectionCompletion, error) {
	status, err := VerifyPilotInspectionJournal(session, events)
	if err != nil {
		return PilotInspectionRecord{}, PilotInspectionCompletion{}, err
	}
	if !status.ReadyToFinalize {
		return PilotInspectionRecord{}, PilotInspectionCompletion{}, errors.New("guided owner inspection session is incomplete and cannot be finalized")
	}
	lastTime := session.CreatedAt
	if len(events) > 0 {
		lastTime = events[len(events)-1].RecordedAt
	}
	if err := requireRelationTimeAfter("guided owner inspection completion", inspectedAt, lastTime); err != nil {
		return PilotInspectionRecord{}, PilotInspectionCompletion{}, err
	}
	drafts, err := pilotInspectionDraftsFromStatus(session, status)
	if err != nil {
		return PilotInspectionRecord{}, PilotInspectionCompletion{}, err
	}
	record, err := BuildPilotInspectionRecord(readiness, bundle, mappings, drafts, session.InspectorAlias, inspectedAt)
	if err != nil {
		return PilotInspectionRecord{}, PilotInspectionCompletion{}, err
	}
	if err := VerifyPilotInspectionRecord(record, readiness, bundle, mappings); err != nil {
		return PilotInspectionRecord{}, PilotInspectionCompletion{}, err
	}
	scarcityStatus := pilotInspectionScarcityStatus(status.ScarcitySummaries, status.ScarcityBoundary)
	overallStatus := combinePilotInspectionStatuses(record.OverallStatus, scarcityStatus)
	completion := PilotInspectionCompletion{
		SchemaVersion: PilotInspectionCompletionSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		JournalProtocol: PilotInspectionJournalProtocol, SessionDigest: session.Digest,
		PackageInventoryDigest: session.Package.PackageInventoryDigest, EventCount: len(events), HeadDigest: status.HeadDigest,
		RequiredAssessments: status.RequiredAssessments, DecisionSummaries: status.DecisionSummaries,
		ScarcitySummaries: status.ScarcitySummaries, ScarcityBoundary: *status.ScarcityBoundary,
		InspectionRecordDigest: record.Digest, CoreStatus: record.OverallStatus, ScarcityStatus: scarcityStatus, OverallStatus: overallStatus,
		HumanStudyStatus: PilotInspectionJournalHumanStudyStatus, ExternalActionStatus: PilotInspectionJournalExternalAction,
		OwnerConfirmation: PilotInspectionCompletionConfirmation, InspectedAt: inspectedAt,
	}
	digest, err := pilotInspectionCompletionDigest(completion)
	if err != nil {
		return PilotInspectionRecord{}, PilotInspectionCompletion{}, err
	}
	completion.Digest = digest
	return record, completion, completion.Validate()
}

func (completion PilotInspectionCompletion) Validate() error {
	if completion.SchemaVersion != PilotInspectionCompletionSchemaVersion || completion.CanonicalPolicy != CanonicalPolicy || completion.JournalProtocol != PilotInspectionJournalProtocol ||
		!validDigest(completion.SessionDigest) || !validDigest(completion.PackageInventoryDigest) || completion.EventCount < completion.RequiredAssessments || !validDigest(completion.HeadDigest) ||
		completion.RequiredAssessments != PilotInspectionRequiredAssessments || len(completion.DecisionSummaries) != 7 || len(completion.ScarcitySummaries) != 3 || !validDigest(completion.InspectionRecordDigest) ||
		!slices.Contains([]PilotInspectionOverallStatus{PilotInspectionOverallPassed, PilotInspectionOverallRevisionRequired, PilotInspectionOverallUnresolved}, completion.CoreStatus) ||
		!slices.Contains([]PilotInspectionOverallStatus{PilotInspectionOverallPassed, PilotInspectionOverallRevisionRequired, PilotInspectionOverallUnresolved}, completion.ScarcityStatus) ||
		!slices.Contains([]PilotInspectionOverallStatus{PilotInspectionOverallPassed, PilotInspectionOverallRevisionRequired, PilotInspectionOverallUnresolved}, completion.OverallStatus) ||
		completion.HumanStudyStatus != PilotInspectionJournalHumanStudyStatus || completion.ExternalActionStatus != PilotInspectionJournalExternalAction ||
		completion.OwnerConfirmation != PilotInspectionCompletionConfirmation || !validDigest(completion.Digest) {
		return errors.New("guided owner inspection completion identity, summary, or claim boundary is invalid")
	}
	if _, err := time.Parse(time.RFC3339, completion.InspectedAt); err != nil {
		return errors.New("guided owner inspection completion time must be RFC3339")
	}
	revisionRequired, unresolved := 0, 0
	validReasons := []PilotInspectionReason{
		PilotInspectionReasonAlignment, PilotInspectionReasonBlinding, PilotInspectionReasonCandidateOrder,
		PilotInspectionReasonInformation, PilotInspectionReasonRedistribution, PilotInspectionReasonRubric,
		PilotInspectionReasonTaskContext, PilotInspectionReasonTransformation,
	}
	for index, summary := range completion.DecisionSummaries {
		if !validOpaqueID(summary.PacketID, "relation-packet-") || index > 0 && completion.DecisionSummaries[index-1].PacketID >= summary.PacketID ||
			!slices.Contains([]PilotInspectionDisposition{PilotInspectionAccepted, PilotInspectionRevisionRequired, PilotInspectionUnresolved}, summary.Disposition) {
			return errors.New("guided owner inspection completion decision summary is invalid or unsorted")
		}
		for reasonIndex, reason := range summary.ReasonCodes {
			if !slices.Contains(validReasons, reason) || reasonIndex > 0 && summary.ReasonCodes[reasonIndex-1] >= reason {
				return errors.New("guided owner inspection completion reason codes are invalid, duplicated, or unsorted")
			}
		}
		if summary.Disposition == PilotInspectionAccepted && len(summary.ReasonCodes) != 0 || summary.Disposition != PilotInspectionAccepted && len(summary.ReasonCodes) == 0 {
			return errors.New("guided owner inspection completion reasons disagree with disposition")
		}
		switch summary.Disposition {
		case PilotInspectionRevisionRequired:
			revisionRequired++
		case PilotInspectionUnresolved:
			unresolved++
		}
	}
	if completion.CoreStatus != pilotInspectionOverallStatus(revisionRequired, unresolved) {
		return errors.New("guided owner inspection completion overall status does not reproduce its decision summaries")
	}
	if err := validatePilotInspectionScarcityCompletion(completion); err != nil {
		return err
	}
	if completion.OverallStatus != combinePilotInspectionStatuses(completion.CoreStatus, completion.ScarcityStatus) {
		return errors.New("guided owner inspection completion overall status does not combine core and scarcity status")
	}
	expected, err := pilotInspectionCompletionDigest(completion)
	if err != nil || completion.Digest != expected {
		return errors.New("guided owner inspection completion digest is invalid")
	}
	return nil
}

func VerifyPilotInspectionCompletion(completion PilotInspectionCompletion, session PilotInspectionSession, events []PilotInspectionEvent, record PilotInspectionRecord, readiness RelationPilotReadiness, bundle ReviewBundle, mappings []PrivateMapping) error {
	if err := completion.Validate(); err != nil {
		return err
	}
	expectedRecord, expectedCompletion, err := BuildPilotInspectionCompletion(session, events, readiness, bundle, mappings, completion.InspectedAt)
	if err != nil {
		return err
	}
	if completion.Digest != expectedCompletion.Digest || record.Digest != expectedRecord.Digest || completion.InspectionRecordDigest != record.Digest {
		return errors.New("guided owner inspection completion does not reproduce from the journal and package parents")
	}
	return VerifyPilotInspectionRecord(record, readiness, bundle, mappings)
}

func (binding PilotInspectionPackageBinding) validate() error {
	if binding.PackageFormat != PilotPackageFormatV5 || !validDigest(binding.PackageInventoryDigest) || !validDigest(binding.ReadinessSHA256) ||
		!validDigest(binding.BundleSHA256) || !validDigest(binding.MappingsSHA256) || !validDigest(binding.WorkbookSHA256) ||
		!validDigest(binding.ChangeAtlasSHA256) || !validDigest(binding.ScarcitySentinelSHA256) || !validDigest(binding.ScarcityAppendixSHA256) {
		return errors.New("guided owner inspection package binding is invalid")
	}
	return nil
}

func pilotInspectionDraftsFromStatus(session PilotInspectionSession, status PilotInspectionJournalStatus) ([]PilotInspectionDecisionDraft, error) {
	if !status.ReadyToFinalize {
		return nil, errors.New("guided owner inspection decisions require a complete latest state")
	}
	drafts := make([]PilotInspectionDecisionDraft, len(session.Packets))
	for index, packet := range session.Packets {
		draft := PilotInspectionDecisionDraft{PacketID: packet.PacketID, CandidateOrder: PilotInspectionNotApplicable}
		for _, dimension := range session.CoreDimensions {
			if !coreDimensionApplies(packet, dimension) {
				continue
			}
			event, exists := status.latest[inspectionEventKey(PilotInspectionSubjectCorePacket, packet.PacketID, dimension)]
			if !exists {
				return nil, errors.New("guided owner inspection latest state is missing an applicable assessment")
			}
			setPilotInspectionDraftAssessment(&draft, dimension, event.Assessment)
		}
		decision := PilotInspectionDecision{
			TaskContext: draft.TaskContext, EvidenceAlignment: draft.EvidenceAlignment,
			TransformationIsolation: draft.TransformationIsolation, InformationSufficiency: draft.InformationSufficiency,
			BlindingIntegrity: draft.BlindingIntegrity, RubricApplicability: draft.RubricApplicability,
			RedistributionBoundary: draft.RedistributionBoundary, CandidateOrder: draft.CandidateOrder,
		}
		draft.ReasonCodes = pilotInspectionReasons(decision)
		drafts[index] = draft
	}
	return drafts, nil
}

func pilotInspectionSummaries(session PilotInspectionSession, drafts []PilotInspectionDecisionDraft) []PilotInspectionDecisionSummary {
	summaries := make([]PilotInspectionDecisionSummary, len(drafts))
	packetByID := make(map[string]PilotInspectionSessionPacket, len(session.Packets))
	for _, packet := range session.Packets {
		packetByID[packet.PacketID] = packet
	}
	for index, draft := range drafts {
		packet := packetByID[draft.PacketID]
		decision := PilotInspectionDecision{
			PacketID: draft.PacketID, Family: packet.Family, Unit: packet.Unit,
			TaskContext: draft.TaskContext, EvidenceAlignment: draft.EvidenceAlignment,
			TransformationIsolation: draft.TransformationIsolation, InformationSufficiency: draft.InformationSufficiency,
			BlindingIntegrity: draft.BlindingIntegrity, RubricApplicability: draft.RubricApplicability,
			RedistributionBoundary: draft.RedistributionBoundary, CandidateOrder: draft.CandidateOrder,
		}
		summaries[index] = PilotInspectionDecisionSummary{
			PacketID: draft.PacketID, ReasonCodes: append([]PilotInspectionReason(nil), draft.ReasonCodes...),
			Disposition: pilotInspectionDisposition(decision),
		}
	}
	sort.Slice(summaries, func(left, right int) bool { return summaries[left].PacketID < summaries[right].PacketID })
	return summaries
}

func pilotInspectionScarcitySummaries(session PilotInspectionSession, status PilotInspectionJournalStatus) ([]PilotInspectionScarcitySummary, *PilotInspectionScarcityBoundarySummary, error) {
	if !status.ReadyToFinalize {
		return nil, nil, errors.New("guided owner inspection scarcity summaries require a complete latest state")
	}
	summaries := make([]PilotInspectionScarcitySummary, len(session.ScarcityCases))
	for index, item := range session.ScarcityCases {
		summary := PilotInspectionScarcitySummary{CaseID: item.CaseID, DataRole: item.DataRole}
		for _, dimension := range session.ScarcityCaseDimensions {
			event, exists := status.latest[inspectionEventKey(PilotInspectionSubjectScarcityCase, item.CaseID, dimension)]
			if !exists {
				return nil, nil, errors.New("guided owner inspection latest state is missing a scarcity-case assessment")
			}
			setPilotInspectionScarcityAssessment(&summary, dimension, event.Assessment)
		}
		summary.Disposition = pilotInspectionAssessmentDisposition([]PilotInspectionAssessment{
			summary.OriginalEvidence, summary.TargetOmission, summary.RelationPreservation, summary.InformationSufficiency,
		})
		summaries[index] = summary
	}
	boundary := &PilotInspectionScarcityBoundarySummary{}
	for _, dimension := range session.ScarcityBoundaryDimensions {
		event, exists := status.latest[inspectionEventKey(PilotInspectionSubjectScarcityBoundary, PilotInspectionScarcityBoundaryID, dimension)]
		if !exists {
			return nil, nil, errors.New("guided owner inspection latest state is missing a scarcity-boundary assessment")
		}
		setPilotInspectionScarcityBoundaryAssessment(boundary, dimension, event.Assessment)
	}
	boundary.Disposition = pilotInspectionAssessmentDisposition([]PilotInspectionAssessment{
		boundary.ExhaustiveScope, boundary.RoleIntegrity, boundary.EstimandSeparation, boundary.NonAuthorization,
	})
	return summaries, boundary, nil
}

func setPilotInspectionScarcityAssessment(summary *PilotInspectionScarcitySummary, dimension PilotInspectionDimension, assessment PilotInspectionAssessment) {
	switch dimension {
	case PilotInspectionDimensionScarcityOriginalEvidence:
		summary.OriginalEvidence = assessment
	case PilotInspectionDimensionScarcityTargetOmission:
		summary.TargetOmission = assessment
	case PilotInspectionDimensionScarcityRelationPreservation:
		summary.RelationPreservation = assessment
	case PilotInspectionDimensionScarcityInformationSufficiency:
		summary.InformationSufficiency = assessment
	}
}

func setPilotInspectionScarcityBoundaryAssessment(summary *PilotInspectionScarcityBoundarySummary, dimension PilotInspectionDimension, assessment PilotInspectionAssessment) {
	switch dimension {
	case PilotInspectionDimensionScarcityExhaustiveScope:
		summary.ExhaustiveScope = assessment
	case PilotInspectionDimensionScarcityRoleIntegrity:
		summary.RoleIntegrity = assessment
	case PilotInspectionDimensionScarcityEstimandSeparation:
		summary.EstimandSeparation = assessment
	case PilotInspectionDimensionScarcityNonAuthorization:
		summary.NonAuthorization = assessment
	}
}

func pilotInspectionAssessmentDisposition(assessments []PilotInspectionAssessment) PilotInspectionDisposition {
	for _, assessment := range assessments {
		if assessment == PilotInspectionFailed {
			return PilotInspectionRevisionRequired
		}
	}
	for _, assessment := range assessments {
		if assessment == PilotInspectionIndeterminate {
			return PilotInspectionUnresolved
		}
	}
	return PilotInspectionAccepted
}

func pilotInspectionScarcityStatus(summaries []PilotInspectionScarcitySummary, boundary *PilotInspectionScarcityBoundarySummary) PilotInspectionOverallStatus {
	revisionRequired, unresolved := 0, 0
	for _, summary := range summaries {
		switch summary.Disposition {
		case PilotInspectionRevisionRequired:
			revisionRequired++
		case PilotInspectionUnresolved:
			unresolved++
		}
	}
	if boundary != nil {
		switch boundary.Disposition {
		case PilotInspectionRevisionRequired:
			revisionRequired++
		case PilotInspectionUnresolved:
			unresolved++
		}
	}
	return pilotInspectionOverallStatus(revisionRequired, unresolved)
}

func combinePilotInspectionStatuses(left, right PilotInspectionOverallStatus) PilotInspectionOverallStatus {
	if left == PilotInspectionOverallRevisionRequired || right == PilotInspectionOverallRevisionRequired {
		return PilotInspectionOverallRevisionRequired
	}
	if left == PilotInspectionOverallUnresolved || right == PilotInspectionOverallUnresolved {
		return PilotInspectionOverallUnresolved
	}
	return PilotInspectionOverallPassed
}

func validatePilotInspectionScarcityCompletion(completion PilotInspectionCompletion) error {
	roles := map[string]int{"development": 0, "calibration": 0}
	for index, summary := range completion.ScarcitySummaries {
		if strings.TrimSpace(summary.CaseID) == "" || index > 0 && completion.ScarcitySummaries[index-1].CaseID >= summary.CaseID {
			return errors.New("guided owner inspection scarcity summary is invalid or unsorted")
		}
		if _, exists := roles[summary.DataRole]; !exists {
			return errors.New("guided owner inspection scarcity summary has an inadmissible data role")
		}
		assessments := []PilotInspectionAssessment{summary.OriginalEvidence, summary.TargetOmission, summary.RelationPreservation, summary.InformationSufficiency}
		if !validPilotInspectionAssessments(assessments) || summary.Disposition != pilotInspectionAssessmentDisposition(assessments) {
			return errors.New("guided owner inspection scarcity summary assessments or disposition are invalid")
		}
		roles[summary.DataRole]++
	}
	if roles["development"] != 2 || roles["calibration"] != 1 {
		return errors.New("guided owner inspection scarcity summary changed the frozen role boundary")
	}
	boundaryAssessments := []PilotInspectionAssessment{
		completion.ScarcityBoundary.ExhaustiveScope, completion.ScarcityBoundary.RoleIntegrity,
		completion.ScarcityBoundary.EstimandSeparation, completion.ScarcityBoundary.NonAuthorization,
	}
	if !validPilotInspectionAssessments(boundaryAssessments) || completion.ScarcityBoundary.Disposition != pilotInspectionAssessmentDisposition(boundaryAssessments) {
		return errors.New("guided owner inspection scarcity-boundary summary is invalid")
	}
	expectedStatus := pilotInspectionScarcityStatus(completion.ScarcitySummaries, &completion.ScarcityBoundary)
	if completion.ScarcityStatus != expectedStatus {
		return errors.New("guided owner inspection scarcity status does not reproduce its summaries")
	}
	return nil
}

func validPilotInspectionAssessments(assessments []PilotInspectionAssessment) bool {
	allowed := []PilotInspectionAssessment{PilotInspectionPassed, PilotInspectionFailed, PilotInspectionIndeterminate}
	for _, assessment := range assessments {
		if !slices.Contains(allowed, assessment) {
			return false
		}
	}
	return true
}

func setPilotInspectionDraftAssessment(draft *PilotInspectionDecisionDraft, dimension PilotInspectionDimension, assessment PilotInspectionAssessment) {
	switch dimension {
	case PilotInspectionDimensionTaskContext:
		draft.TaskContext = assessment
	case PilotInspectionDimensionEvidenceAlignment:
		draft.EvidenceAlignment = assessment
	case PilotInspectionDimensionTransformationIsolation:
		draft.TransformationIsolation = assessment
	case PilotInspectionDimensionInformationSufficiency:
		draft.InformationSufficiency = assessment
	case PilotInspectionDimensionBlindingIntegrity:
		draft.BlindingIntegrity = assessment
	case PilotInspectionDimensionRubricApplicability:
		draft.RubricApplicability = assessment
	case PilotInspectionDimensionRedistributionBoundary:
		draft.RedistributionBoundary = assessment
	case PilotInspectionDimensionCandidateOrder:
		draft.CandidateOrder = assessment
	}
}

func sessionPacket(session PilotInspectionSession, packetID string) (PilotInspectionSessionPacket, bool) {
	for _, packet := range session.Packets {
		if packet.PacketID == packetID {
			return packet, true
		}
	}
	return PilotInspectionSessionPacket{}, false
}

func coreDimensionApplies(packet PilotInspectionSessionPacket, dimension PilotInspectionDimension) bool {
	if !slices.Contains(PilotInspectionCoreDimensions(), dimension) {
		return false
	}
	return dimension != PilotInspectionDimensionCandidateOrder || packet.Unit == UnitCandidatePairOrders
}

func inspectionTargetApplies(session PilotInspectionSession, subjectKind PilotInspectionSubjectKind, subjectID string, dimension PilotInspectionDimension) bool {
	switch subjectKind {
	case PilotInspectionSubjectCorePacket:
		packet, exists := sessionPacket(session, subjectID)
		return exists && coreDimensionApplies(packet, dimension)
	case PilotInspectionSubjectScarcityCase:
		_, exists := sessionScarcityCase(session, subjectID)
		return exists && slices.Contains(session.ScarcityCaseDimensions, dimension)
	case PilotInspectionSubjectScarcityBoundary:
		return subjectID == PilotInspectionScarcityBoundaryID && slices.Contains(session.ScarcityBoundaryDimensions, dimension)
	default:
		return false
	}
}

func sessionScarcityCase(session PilotInspectionSession, caseID string) (PilotInspectionSessionScarcityCase, bool) {
	for _, item := range session.ScarcityCases {
		if item.CaseID == caseID {
			return item, true
		}
	}
	return PilotInspectionSessionScarcityCase{}, false
}

func requiredInspectionAssessments(session PilotInspectionSession) int {
	required := 0
	for _, packet := range session.Packets {
		for _, dimension := range session.CoreDimensions {
			if coreDimensionApplies(packet, dimension) {
				required++
			}
		}
	}
	required += len(session.ScarcityCases) * len(session.ScarcityCaseDimensions)
	required += len(session.ScarcityBoundaryDimensions)
	return required
}

func inspectionEventKey(subjectKind PilotInspectionSubjectKind, subjectID string, dimension PilotInspectionDimension) string {
	return string(subjectKind) + "\x00" + subjectID + "\x00" + string(dimension)
}

func completedInspectionAssessments(latest map[string]PilotInspectionEvent) (int, int, int) {
	core, scarcity, boundary := 0, 0, 0
	for _, event := range latest {
		switch event.SubjectKind {
		case PilotInspectionSubjectCorePacket:
			core++
		case PilotInspectionSubjectScarcityCase:
			scarcity++
		case PilotInspectionSubjectScarcityBoundary:
			boundary++
		}
	}
	return core, scarcity, boundary
}

func validInventoryPath(value string) bool {
	return value != "" && !strings.HasPrefix(value, "/") && !strings.Contains(value, "\\") && path.Clean(value) == value && value != "." && value != ".." && !strings.HasPrefix(value, "../")
}

func pilotPackageInventoryDigest(inventory PilotPackageInventory) (string, error) {
	directories := make([]map[string]any, len(inventory.Directories))
	for index, directory := range inventory.Directories {
		directories[index] = map[string]any{"mode": directory.Mode, "path": directory.Path}
	}
	files := make([]map[string]any, len(inventory.Files))
	for index, file := range inventory.Files {
		files[index] = map[string]any{"bytes": file.Bytes, "mode": file.Mode, "path": file.Path, "sha256": file.SHA256}
	}
	return digestJSON(map[string]any{
		"directories": directories, "files": files, "hash_algorithm": inventory.HashAlgorithm,
		"package_format": inventory.PackageFormat, "payload_bytes": inventory.PayloadBytes,
		"payload_files": inventory.PayloadFiles, "schema_version": inventory.SchemaVersion, "scope": inventory.Scope,
	})
}

func pilotInspectionSessionDigest(session PilotInspectionSession) (string, error) {
	session.Digest = ""
	return digestJSON(session)
}

func pilotInspectionEventDigest(event PilotInspectionEvent) (string, error) {
	event.Digest = ""
	return digestJSON(event)
}

func pilotInspectionCompletionDigest(completion PilotInspectionCompletion) (string, error) {
	completion.Digest = ""
	return digestJSON(completion)
}
