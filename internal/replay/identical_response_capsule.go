package replay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	IdenticalResponseStudyManifestComponentSchemaVersion    = "evalwitness.identical-response-study-manifest.v1"
	IdenticalResponseStudyRecordComponentSchemaVersion      = "evalwitness.identical-response-study-record.v1"
	IdenticalResponseAuthorizationComponentSchemaVersion    = "evalwitness.identical-response-live-authorization.v1"
	IdenticalResponseRouteAttestationComponentSchemaVersion = "evalwitness.identical-response-route-attestation.v1"
	IdenticalResponseCaptureRunComponentSchemaVersion       = "evalwitness.identical-response-capture-run-attestation.v1"
	IdenticalResponseAdmissionComponentSchemaVersion        = "evalwitness.identical-response-research-admission.v1"
	IdenticalResponseAnalysisComponentSchemaVersion         = "evalwitness.identical-response-analysis.v1"
	IdenticalResponseOuterBindSchemaVersion                 = "evalwitness.identical-response-050-bind.v2"
	IdenticalResponseRegistryID                             = "evalwitness.identical-response-v5-registry.v1"
	IdenticalResponseEvidenceCeiling                        = "record_complete_external_parents_resolved"
	identicalResponseLiveAuthorizationSchemaVersion         = "evalwitness.live-authorization.v1"

	identicalResponseStudyManifestValidatorID = "evalwitness.validator.identical-response-study-manifest.v1"
	identicalResponseStudyRecordValidatorID   = "evalwitness.validator.identical-response-study-record.v1"
	identicalResponseStudyRecordBindingID     = "evalwitness.validator.identical-response-study-record-bindings.v1"
	identicalResponseAuthorizationValidatorID = "evalwitness.validator.identical-response-live-authorization.v1"
	identicalResponseAuthorizationBindingID   = "evalwitness.validator.identical-response-live-authorization-bindings.v1"
	identicalResponseRouteValidatorID         = "evalwitness.validator.identical-response-route-attestation.v1"
	identicalResponseCaptureRunValidatorID    = "evalwitness.validator.identical-response-capture-run-attestation.v1"
	identicalResponseAdmissionValidatorID     = "evalwitness.validator.identical-response-research-admission.v1"
	identicalResponseAdmissionBindingID       = "evalwitness.validator.identical-response-research-admission-bindings.v1"
	identicalResponseAnalysisValidatorID      = "evalwitness.validator.identical-response-analysis.v1"
	identicalResponseAnalysisBindingID        = "evalwitness.validator.identical-response-analysis-bindings.v1"
	identicalResponseOuterBindValidatorID     = "evalwitness.validator.identical-response-050-bind.v2"
	identicalResponseOuterBindBindingID       = "evalwitness.validator.identical-response-050-bind-bindings.v2"
)

// IdenticalResponseCapsuleArtifacts are the independently sealed empirical
// artifacts that become child components of the TASK-050 extension capsule.
type IdenticalResponseCapsuleArtifacts struct {
	StudyManifest         []byte
	StudyRecord           []byte
	LiveAuthorization     []byte
	RouteAttestation      []byte
	CaptureRunAttestation []byte
	ResearchAdmission     []byte
	OfflineAnalysis       []byte
}

type identicalResponseAuthorizationLimits struct {
	MaxCalls                int     `json:"max_calls"`
	MaxAttempts             int     `json:"max_attempts"`
	MaxEstimatedInputTokens int     `json:"max_estimated_input_tokens"`
	MaxReservedOutputTokens int     `json:"max_reserved_output_tokens"`
	MaxConcurrent           int     `json:"max_concurrent"`
	MaxCostUSD              float64 `json:"max_cost_usd,omitempty"`
	MaxDurationSeconds      int64   `json:"max_duration_seconds"`
}

type identicalResponseAuthorizationPlan struct {
	SchemaVersion              string                               `json:"schema_version"`
	Entrypoint                 string                               `json:"entrypoint"`
	RouteID                    string                               `json:"route_id"`
	RequestFingerprint         string                               `json:"request_fingerprint"`
	RequestContractDigest      string                               `json:"request_contract_digest"`
	RequestSchemaVersion       int                                  `json:"request_schema_version"`
	PromptBuilderVersion       string                               `json:"prompt_builder_version"`
	ParserContractVersion      string                               `json:"parser_contract_version"`
	ScorePolicyVersion         string                               `json:"score_policy_version"`
	MaxRetries                 int                                  `json:"max_retries"`
	MaxWorkers                 int                                  `json:"max_workers"`
	MinDispatchIntervalSeconds int                                  `json:"min_dispatch_interval_seconds,omitempty"`
	MaxOutputTokens            int                                  `json:"max_output_tokens"`
	ExpectedCalls              int                                  `json:"expected_calls"`
	WorstCalls                 int                                  `json:"worst_calls"`
	Limits                     identicalResponseAuthorizationLimits `json:"limits"`
	StudyManifestDigest        string                               `json:"study_manifest_digest,omitempty"`
	AuthorizationDigest        string                               `json:"authorization_digest"`
}

func (plan identicalResponseAuthorizationPlan) Digest() (string, error) {
	plan.AuthorizationDigest = ""
	raw, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

type IdenticalResponseComponentRef struct {
	Name          string `json:"name"`
	TypeID        string `json:"type_id"`
	ComponentID   string `json:"component_id"`
	PayloadDigest string `json:"payload_digest"`
}

type IdenticalResponseAnalysisSummaryCommitment struct {
	TaskGroups    int `json:"task_groups"`
	Complete      int `json:"complete"`
	Agreements    int `json:"agreements"`
	Disagreements int `json:"disagreements"`
	Unresolved    int `json:"unresolved"`
}

// IdenticalResponseEvidenceRoot is the scientific commitment for the outer
// capsule. It binds every child artifact to the immutable response-bundle
// parent while keeping the claim ledger as the standard TASK-050 sidecar.
type IdenticalResponseEvidenceRoot struct {
	SchemaVersion               string                                     `json:"schema_version"`
	StudyID                     string                                     `json:"study_id"`
	CellID                      string                                     `json:"cell_id"`
	Status                      string                                     `json:"status"`
	EvidenceCeiling             string                                     `json:"evidence_ceiling"`
	BaseCapsuleID               string                                     `json:"base_capsule_id"`
	BaseIndexComponentID        string                                     `json:"base_index_component_id"`
	BasePolicyComponentID       string                                     `json:"base_policy_component_id"`
	CaptureComponentID          string                                     `json:"capture_component_id"`
	CapturePayloadSHA256        string                                     `json:"capture_payload_sha256"`
	AuthorizedCalls             int                                        `json:"authorized_calls"`
	ObservedCalls               int                                        `json:"observed_calls"`
	CompleteResearchEntries     int                                        `json:"complete_research_entries"`
	StudyManifestDigest         string                                     `json:"study_manifest_digest"`
	StudyRecordDigest           string                                     `json:"study_record_digest"`
	AuthorizationDigest         string                                     `json:"authorization_digest"`
	RouteAttestationDigest      string                                     `json:"route_attestation_digest"`
	CaptureRunAttestationDigest string                                     `json:"capture_run_attestation_digest"`
	AdmissionDigest             string                                     `json:"admission_digest"`
	AnalysisDigest              string                                     `json:"analysis_digest"`
	AnalysisSummary             IdenticalResponseAnalysisSummaryCommitment `json:"analysis_summary"`
	Components                  []IdenticalResponseComponentRef            `json:"components"`
	Limitations                 []string                                   `json:"limitations"`
	ProviderCalls               int                                        `json:"provider_calls"`
	NetworkRequired             bool                                       `json:"network_required"`
	Digest                      string                                     `json:"digest"`
}

type IdenticalResponseCapsuleBuild struct {
	Package capsule.Package
	Root    IdenticalResponseEvidenceRoot
}

func LoadResponseBundlePackage(ctx context.Context, source string) (capsule.Package, error) {
	if ctx == nil || strings.TrimSpace(source) == "" {
		return capsule.Package{}, errors.New("response bundle package requires context and source")
	}
	registry, err := ResponseBundleRegistry()
	if err != nil {
		return capsule.Package{}, err
	}
	manifest, payloads, err := capsule.LoadDirectory(ctx, source, registry, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err != nil {
		return capsule.Package{}, err
	}
	return capsule.Package{Registry: registry, Manifest: manifest, Payloads: payloads}, nil
}

func LoadIdenticalResponseCapsulePackage(ctx context.Context, source string, base *capsule.Registry) (capsule.Package, error) {
	if ctx == nil || strings.TrimSpace(source) == "" || base == nil {
		return capsule.Package{}, errors.New("identical-response capsule package requires context, source, and base registry")
	}
	registry, err := IdenticalResponseCapsuleRegistry(base)
	if err != nil {
		return capsule.Package{}, err
	}
	manifest, payloads, err := capsule.LoadDirectory(ctx, source, registry, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err != nil {
		return capsule.Package{}, err
	}
	return capsule.Package{Registry: registry, Manifest: manifest, Payloads: payloads}, nil
}

func IdenticalResponseCapsuleRegistry(base *capsule.Registry) (*capsule.Registry, error) {
	if base == nil {
		return nil, errors.New("identical-response capsule extension requires a base registry")
	}
	public := []capsule.Visibility{capsule.VisibilityPublic}
	childDerivedExternal := func(parentType string) capsule.ParentRule {
		return capsule.ParentRule{Kind: capsule.EdgeDerivedFrom, ParentType: parentType, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentExternal}}
	}
	childAttestsExternal := func(parentType string) capsule.ParentRule {
		return capsule.ParentRule{Kind: capsule.EdgeAttests, ParentType: parentType, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentExternal}}
	}
	childDerivedInternal := func(parentType string) capsule.ParentRule {
		return capsule.ParentRule{Kind: capsule.EdgeDerivedFrom, ParentType: parentType, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}}
	}
	types := []capsule.ComponentType{
		{TypeID: IdenticalResponseStudyManifestComponentSchemaVersion, SchemaID: IdenticalResponseStudyManifestComponentSchemaVersion, Role: capsule.RoleGovernance, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseStudyManifestValidatorID, ParentRules: []capsule.ParentRule{childDerivedExternal(ResponseBundlePolicySchemaVersion)}},
		{TypeID: IdenticalResponseStudyRecordComponentSchemaVersion, SchemaID: IdenticalResponseStudyRecordComponentSchemaVersion, Role: capsule.RoleGovernance, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseStudyRecordValidatorID, BindingValidatorID: identicalResponseStudyRecordBindingID, ParentRules: []capsule.ParentRule{childDerivedInternal(IdenticalResponseStudyManifestComponentSchemaVersion)}},
		{TypeID: IdenticalResponseAuthorizationComponentSchemaVersion, SchemaID: IdenticalResponseAuthorizationComponentSchemaVersion, Role: capsule.RoleGovernance, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseAuthorizationValidatorID, BindingValidatorID: identicalResponseAuthorizationBindingID, ParentRules: []capsule.ParentRule{childDerivedInternal(IdenticalResponseRouteAttestationComponentSchemaVersion), childDerivedInternal(IdenticalResponseStudyManifestComponentSchemaVersion)}},
		{TypeID: IdenticalResponseRouteAttestationComponentSchemaVersion, SchemaID: IdenticalResponseRouteAttestationComponentSchemaVersion, Role: capsule.RoleAttestation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseRouteValidatorID, ParentRules: []capsule.ParentRule{childAttestsExternal(ResponseCaptureComponentSchemaVersion)}},
		{TypeID: IdenticalResponseCaptureRunComponentSchemaVersion, SchemaID: IdenticalResponseCaptureRunComponentSchemaVersion, Role: capsule.RoleAttestation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseCaptureRunValidatorID, ParentRules: []capsule.ParentRule{childAttestsExternal(ResponseCaptureComponentSchemaVersion)}},
		{TypeID: IdenticalResponseAdmissionComponentSchemaVersion, SchemaID: IdenticalResponseAdmissionComponentSchemaVersion, Role: capsule.RoleGovernance, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseAdmissionValidatorID, BindingValidatorID: identicalResponseAdmissionBindingID, ParentRules: []capsule.ParentRule{childDerivedInternal(IdenticalResponseCaptureRunComponentSchemaVersion)}},
		{TypeID: IdenticalResponseAnalysisComponentSchemaVersion, SchemaID: IdenticalResponseAnalysisComponentSchemaVersion, Role: capsule.RoleDerivation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseAnalysisValidatorID, BindingValidatorID: identicalResponseAnalysisBindingID, ParentRules: []capsule.ParentRule{
			{Kind: capsule.EdgeDerivedFrom, ParentType: ResponseCaptureComponentSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentExternal}},
			childDerivedInternal(IdenticalResponseAdmissionComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseRouteAttestationComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseStudyRecordComponentSchemaVersion),
		}},
		{TypeID: IdenticalResponseOuterBindSchemaVersion, SchemaID: IdenticalResponseOuterBindSchemaVersion, Role: capsule.RoleDerivation, AllowedVisibilities: public, MediaType: "application/json", PayloadProfile: capsule.PayloadExactBytes, ValidatorID: identicalResponseOuterBindValidatorID, BindingValidatorID: identicalResponseOuterBindBindingID, ParentRules: []capsule.ParentRule{
			{Kind: capsule.EdgeDerivedFrom, ParentType: ResponseBundleIndexSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentExternal}},
			childDerivedInternal(IdenticalResponseAdmissionComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseAnalysisComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseCaptureRunComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseRouteAttestationComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseAuthorizationComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseStudyManifestComponentSchemaVersion),
			childDerivedInternal(IdenticalResponseStudyRecordComponentSchemaVersion),
		}},
	}
	document, err := capsule.SealRegistry(IdenticalResponseRegistryID, base.Digest(), types)
	if err != nil {
		return nil, err
	}
	validators := map[string]capsule.PayloadValidator{
		identicalResponseStudyManifestValidatorID: func(payload []byte) error {
			var manifest study.Manifest
			if err := decodeIdenticalJSON(payload, &manifest); err != nil {
				return err
			}
			return manifest.Validate()
		},
		identicalResponseStudyRecordValidatorID: func(payload []byte) error {
			var record study.Record
			if err := decodeIdenticalJSON(payload, &record); err != nil {
				return err
			}
			return record.Validate()
		},
		identicalResponseAuthorizationValidatorID: func(payload []byte) error {
			var authorization identicalResponseAuthorizationPlan
			if err := decodeIdenticalJSON(payload, &authorization); err != nil {
				return err
			}
			if authorization.SchemaVersion != identicalResponseLiveAuthorizationSchemaVersion || authorization.AuthorizationDigest == "" {
				return errors.New("identical-response live authorization identity is invalid")
			}
			want, err := authorization.Digest()
			if err != nil || want != authorization.AuthorizationDigest {
				return errors.New("identical-response live authorization digest is invalid")
			}
			return nil
		},
		identicalResponseRouteValidatorID: func(payload []byte) error {
			var attestation conformance.Attestation
			if err := decodeIdenticalJSON(payload, &attestation); err != nil {
				return err
			}
			if err := attestation.ValidateIntegrity(); err != nil {
				return err
			}
			if attestation.State != conformance.StateBoundedQualified {
				return errors.New("identical-response route attestation is not bounded qualified")
			}
			return nil
		},
		identicalResponseCaptureRunValidatorID: func(payload []byte) error {
			var attestation CaptureRunAttestation
			if err := decodeIdenticalJSON(payload, &attestation); err != nil {
				return err
			}
			return validateIdenticalCaptureRunAttestation(attestation)
		},
		identicalResponseAdmissionValidatorID: func(payload []byte) error {
			var admission CaptureResearchAdmission
			if err := decodeIdenticalJSON(payload, &admission); err != nil {
				return err
			}
			return validateIdenticalResearchAdmission(admission)
		},
		identicalResponseAnalysisValidatorID: func(payload []byte) error {
			var analysis IdenticalResponseAnalysis
			if err := decodeIdenticalJSON(payload, &analysis); err != nil {
				return err
			}
			return analysis.Validate()
		},
		identicalResponseOuterBindValidatorID: func(payload []byte) error {
			var root IdenticalResponseEvidenceRoot
			if err := decodeIdenticalJSON(payload, &root); err != nil {
				return err
			}
			return root.Validate()
		},
	}
	bindings := map[string]capsule.BindingValidator{
		identicalResponseStudyRecordBindingID:   validateIdenticalStudyRecordBindings,
		identicalResponseAuthorizationBindingID: validateIdenticalAuthorizationBindings,
		identicalResponseAdmissionBindingID:     validateIdenticalAdmissionBindings,
		identicalResponseAnalysisBindingID:      validateIdenticalAnalysisBindings,
		identicalResponseOuterBindBindingID:     validateIdenticalOuterBindBindings,
	}
	return capsule.ExtendRegistryWithBindings(base, document, validators, bindings)
}

func BuildIdenticalResponseCapsule(ctx context.Context, base capsule.Package, artifacts IdenticalResponseCapsuleArtifacts) (IdenticalResponseCapsuleBuild, error) {
	if ctx == nil || base.Registry == nil {
		return IdenticalResponseCapsuleBuild{}, errors.New("identical-response capsule build requires context and base package")
	}
	if _, err := capsule.VerifyPackage(ctx, base.Registry, base.Manifest, base.Payloads, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}); err != nil {
		return IdenticalResponseCapsuleBuild{}, fmt.Errorf("verify response-bundle parent: %w", err)
	}
	policyRecord, err := responseComponentByType(base.Manifest.Components, ResponseBundlePolicySchemaVersion)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	indexRecord, err := responseComponentByType(base.Manifest.Components, ResponseBundleIndexSchemaVersion)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(base.Payloads[policyRecord.Payload.Digest], &policy); err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	if err := policy.Validate(); err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	var index ResponseBundleIndex
	if err := protocol.DecodeStrict(base.Payloads[indexRecord.Payload.Digest], &index); err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	captureRecord, _, captureInspection, err := identicalResponseBaseCapture(base, index)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	manifest, manifestRaw, err := decodeIdenticalStudyManifest(artifacts.StudyManifest)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	record, recordRaw, err := decodeIdenticalStudyRecord(artifacts.StudyRecord)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	authorization, authorizationRaw, err := decodeIdenticalAuthorization(artifacts.LiveAuthorization)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	attestation, attestationRaw, err := decodeIdenticalRouteAttestation(artifacts.RouteAttestation)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	captureRun, captureRunRaw, err := decodeIdenticalCaptureRunAttestation(artifacts.CaptureRunAttestation)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	admission, admissionRaw, err := decodeIdenticalResearchAdmission(artifacts.ResearchAdmission)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	analysis, analysisRaw, err := decodeIdenticalAnalysis(artifacts.OfflineAnalysis)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	if err := validateIdenticalResponseInputs(base, policy, captureRecord, captureInspection, manifest, record, authorization, attestation, captureRun, admission, analysis); err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	registry, err := IdenticalResponseCapsuleRegistry(base.Registry)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	manifestRecord, normalizedManifest, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.study-manifest", TypeID: IdenticalResponseStudyManifestComponentSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: manifestRaw,
		Parents: []capsule.ParentRef{identicalExternalParent(capsule.EdgeDerivedFrom, policyRecord, base.Manifest.CapsuleID)},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	routeRecord, normalizedRoute, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.route-attestation", TypeID: IdenticalResponseRouteAttestationComponentSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: attestationRaw,
		Parents: []capsule.ParentRef{identicalExternalParent(capsule.EdgeAttests, captureRecord, base.Manifest.CapsuleID)},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	captureRunRecord, normalizedCaptureRun, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.capture-run-attestation", TypeID: IdenticalResponseCaptureRunComponentSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: captureRunRaw,
		Parents: []capsule.ParentRef{identicalExternalParent(capsule.EdgeAttests, captureRecord, base.Manifest.CapsuleID)},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	studyRecordRecord, normalizedStudyRecord, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.study-record", TypeID: IdenticalResponseStudyRecordComponentSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: recordRaw,
		Parents: []capsule.ParentRef{identicalInternalParent(capsule.EdgeDerivedFrom, manifestRecord)},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	authorizationRecord, normalizedAuthorization, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.live-authorization", TypeID: IdenticalResponseAuthorizationComponentSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: authorizationRaw,
		Parents: []capsule.ParentRef{
			identicalInternalParent(capsule.EdgeDerivedFrom, manifestRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, routeRecord),
		},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	admissionRecord, normalizedAdmission, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.research-admission", TypeID: IdenticalResponseAdmissionComponentSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: admissionRaw,
		Parents: []capsule.ParentRef{identicalInternalParent(capsule.EdgeDerivedFrom, captureRunRecord)},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	analysisRecord, normalizedAnalysis, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.offline-analysis", TypeID: IdenticalResponseAnalysisComponentSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: analysisRaw,
		Parents: []capsule.ParentRef{
			identicalExternalParent(capsule.EdgeDerivedFrom, captureRecord, base.Manifest.CapsuleID),
			identicalInternalParent(capsule.EdgeDerivedFrom, admissionRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, routeRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, studyRecordRecord),
		},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	childRecords := []capsule.ComponentRecord{manifestRecord, routeRecord, captureRunRecord, studyRecordRecord, authorizationRecord, admissionRecord, analysisRecord}
	root := IdenticalResponseEvidenceRoot{
		SchemaVersion: IdenticalResponseOuterBindSchemaVersion, StudyID: policy.StudyID, CellID: policy.CellID,
		Status: StudyBindStatusComplete, EvidenceCeiling: IdenticalResponseEvidenceCeiling,
		BaseCapsuleID: base.Manifest.CapsuleID, BaseIndexComponentID: indexRecord.ComponentID,
		BasePolicyComponentID: policyRecord.ComponentID, CaptureComponentID: captureRecord.ComponentID,
		CapturePayloadSHA256: captureInspection.PayloadSHA256, AuthorizedCalls: captureRun.AuthorizedCalls,
		ObservedCalls: captureRun.ObservedCalls, CompleteResearchEntries: captureRun.Inspection.CompleteResearchEntries,
		StudyManifestDigest: record.Study.ManifestDigest, StudyRecordDigest: record.RecordDigest,
		AuthorizationDigest:    authorization.AuthorizationDigest,
		RouteAttestationDigest: attestation.AttestationDigest, CaptureRunAttestationDigest: captureRun.Digest,
		AdmissionDigest: admission.Digest, AnalysisDigest: analysis.Digest,
		AnalysisSummary: IdenticalResponseAnalysisSummaryCommitment{
			TaskGroups: analysis.Summary.TaskGroups, Complete: analysis.Summary.Complete,
			Agreements: analysis.Summary.Agreements, Disagreements: analysis.Summary.Disagreements,
			Unresolved: analysis.Summary.Unresolved,
		},
		Limitations: []string{
			"bounded live B.AI observation over the locked development population; no provider endorsement",
			"no transfer, calibration, population, independent replication, or release claim",
			"outcome sensitivity covers only the groups recorded in the offline analysis and does not establish correctness or model quality",
		},
		ProviderCalls: 0, NetworkRequired: false,
	}
	root.Components = make([]IdenticalResponseComponentRef, 0, len(childRecords))
	for _, record := range childRecords {
		root.Components = append(root.Components, IdenticalResponseComponentRef{
			Name: record.Name, TypeID: record.TypeID, ComponentID: record.ComponentID, PayloadDigest: record.Payload.Digest,
		})
	}
	slices.SortFunc(root.Components, func(left, right IdenticalResponseComponentRef) int { return strings.Compare(left.Name, right.Name) })
	unsignedRoot := root
	unsignedRoot.Digest = ""
	root.Digest, err = protocol.Digest(unsignedRoot)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	rootRaw, err := protocol.CanonicalMarshal(root)
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	rootRecord, normalizedRoot, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "identical-response.outer-bind", TypeID: IdenticalResponseOuterBindSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: rootRaw,
		Parents: []capsule.ParentRef{
			identicalExternalParent(capsule.EdgeDerivedFrom, indexRecord, base.Manifest.CapsuleID),
			identicalInternalParent(capsule.EdgeDerivedFrom, admissionRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, analysisRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, captureRunRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, routeRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, manifestRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, studyRecordRecord),
			identicalInternalParent(capsule.EdgeDerivedFrom, authorizationRecord),
		},
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	records := append(childRecords, rootRecord)
	payloads := map[string][]byte{
		manifestRecord.Payload.Digest:      normalizedManifest,
		routeRecord.Payload.Digest:         normalizedRoute,
		captureRunRecord.Payload.Digest:    normalizedCaptureRun,
		studyRecordRecord.Payload.Digest:   normalizedStudyRecord,
		authorizationRecord.Payload.Digest: normalizedAuthorization,
		admissionRecord.Payload.Digest:     normalizedAdmission,
		analysisRecord.Payload.Digest:      normalizedAnalysis,
		rootRecord.Payload.Digest:          normalizedRoot,
	}
	childManifest, err := capsule.BuildManifest(registry, capsule.ManifestInput{
		StudyID: policy.StudyID, CellID: policy.CellID,
		ParentCapsules:  []capsule.CapsuleRef{{Relation: "extends", CapsuleID: base.Manifest.CapsuleID}},
		ScientificRoots: []string{rootRecord.ComponentID}, PresentationRoots: []string{}, Components: records,
	})
	if err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	pack := capsule.Package{Registry: registry, Manifest: childManifest, Payloads: payloads}
	if _, err := VerifyIdenticalResponseCapsuleFamily(ctx, pack, base); err != nil {
		return IdenticalResponseCapsuleBuild{}, err
	}
	return IdenticalResponseCapsuleBuild{Package: pack, Root: root}, nil
}

func VerifyIdenticalResponseCapsuleFamily(ctx context.Context, child, base capsule.Package) (capsule.VerificationReport, error) {
	if ctx == nil || child.Registry == nil || base.Registry == nil {
		return capsule.VerificationReport{}, errors.New("identical-response capsule family requires context and both packages")
	}
	report, err := capsule.VerifyPackageFamily(ctx, child, []capsule.Package{base}, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic})
	if err != nil {
		return capsule.VerificationReport{}, err
	}
	rootRecord, err := identicalComponentByName(child.Manifest, "identical-response.outer-bind")
	if err != nil {
		return capsule.VerificationReport{}, err
	}
	var root IdenticalResponseEvidenceRoot
	if err := decodeIdenticalJSON(child.Payloads[rootRecord.Payload.Digest], &root); err != nil {
		return capsule.VerificationReport{}, err
	}
	if root.BaseCapsuleID != base.Manifest.CapsuleID || root.StudyID != base.Manifest.StudyID || root.CellID != base.Manifest.CellID || child.Manifest.StudyID != root.StudyID || child.Manifest.CellID != root.CellID {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind does not match the response-bundle study identity")
	}
	policyRecord, err := componentByID(base.Manifest, root.BasePolicyComponentID)
	if err != nil || policyRecord.TypeID != ResponseBundlePolicySchemaVersion {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind policy parent is unavailable")
	}
	policyRaw, found := base.Payloads[policyRecord.Payload.Digest]
	if !found {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind policy payload is unavailable")
	}
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(policyRaw, &policy); err != nil {
		return capsule.VerificationReport{}, err
	}
	if err := policy.Validate(); err != nil || policy.StudyID != root.StudyID || policy.CellID != root.CellID {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind policy identity differs from the response bundle")
	}
	if err := validateIdenticalStudyParentBindings(child, root); err != nil {
		return capsule.VerificationReport{}, err
	}
	captureRecord, err := componentByID(base.Manifest, root.CaptureComponentID)
	if err != nil || captureRecord.TypeID != ResponseCaptureComponentSchemaVersion {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind capture parent is unavailable")
	}
	captureRaw, found := base.Payloads[captureRecord.Payload.Digest]
	if !found {
		return capsule.VerificationReport{}, errors.New("identical-response capture parent payload is unavailable")
	}
	inspection, _, err := inspectCapturePayload(captureRaw)
	if err != nil {
		return capsule.VerificationReport{}, err
	}
	if inspection.PayloadSHA256 != root.CapturePayloadSHA256 || inspection.PayloadSHA256 != captureRecord.Payload.Digest || inspection.Entries != root.ObservedCalls {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind differs from exact capture inspection")
	}
	indexRecord, err := componentByID(base.Manifest, root.BaseIndexComponentID)
	if err != nil || indexRecord.TypeID != ResponseBundleIndexSchemaVersion {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind index parent is unavailable")
	}
	var index ResponseBundleIndex
	if err := protocol.DecodeStrict(base.Payloads[indexRecord.Payload.Digest], &index); err != nil {
		return capsule.VerificationReport{}, err
	}
	var indexedCapture *ResponseBundleCapture
	for captureIndex := range index.Captures {
		if index.Captures[captureIndex].ComponentID == captureRecord.ComponentID {
			indexedCapture = &index.Captures[captureIndex]
			break
		}
	}
	if indexedCapture == nil || !reflect.DeepEqual(indexedCapture.CaptureInspection, inspection) {
		return capsule.VerificationReport{}, errors.New("identical-response capture inspection differs from the response-bundle index")
	}
	routeRecord, err := identicalComponentByName(child.Manifest, "identical-response.route-attestation")
	if err != nil {
		return capsule.VerificationReport{}, err
	}
	var attestation conformance.Attestation
	if err := decodeIdenticalJSON(child.Payloads[routeRecord.Payload.Digest], &attestation); err != nil {
		return capsule.VerificationReport{}, err
	}
	if err := attestation.ValidateIntegrity(); err != nil || root.RouteAttestationDigest != attestation.AttestationDigest || len(inspection.ServedModels) != 1 || attestation.Identity.RouteID != inspection.RouteID || attestation.Identity.ProviderID != inspection.ProviderID || attestation.Identity.RequestedModel != inspection.RequestedModel || attestation.Identity.ServedModel != inspection.ServedModels[0] {
		return capsule.VerificationReport{}, errors.New("identical-response route attestation differs from the exact capture route")
	}
	authorizationRecord, err := identicalComponentByName(child.Manifest, "identical-response.live-authorization")
	if err != nil || authorizationRecord.TypeID != IdenticalResponseAuthorizationComponentSchemaVersion {
		return capsule.VerificationReport{}, errors.New("identical-response live authorization parent is unavailable")
	}
	var authorization identicalResponseAuthorizationPlan
	if err := decodeIdenticalJSON(child.Payloads[authorizationRecord.Payload.Digest], &authorization); err != nil {
		return capsule.VerificationReport{}, err
	}
	if root.AuthorizationDigest != authorization.AuthorizationDigest {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind authorization commitment differs from its plan")
	}
	captureRunRecord, err := identicalComponentByName(child.Manifest, "identical-response.capture-run-attestation")
	if err != nil {
		return capsule.VerificationReport{}, err
	}
	var captureRun CaptureRunAttestation
	if err := decodeIdenticalJSON(child.Payloads[captureRunRecord.Payload.Digest], &captureRun); err != nil {
		return capsule.VerificationReport{}, err
	}
	if root.CaptureRunAttestationDigest != captureRun.Digest || root.AuthorizedCalls != captureRun.AuthorizedCalls || root.ObservedCalls != captureRun.ObservedCalls || root.CompleteResearchEntries != captureRun.Inspection.CompleteResearchEntries {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind capture-run commitment differs from its attestation")
	}
	if err := VerifyCaptureRunAttestationPayload(captureRaw, captureRun); err != nil {
		return capsule.VerificationReport{}, err
	}
	admissionRecord, err := identicalComponentByName(child.Manifest, "identical-response.research-admission")
	if err != nil {
		return capsule.VerificationReport{}, err
	}
	var admission CaptureResearchAdmission
	if err := decodeIdenticalJSON(child.Payloads[admissionRecord.Payload.Digest], &admission); err != nil {
		return capsule.VerificationReport{}, err
	}
	if root.AdmissionDigest != admission.Digest {
		return capsule.VerificationReport{}, errors.New("identical-response outer bind admission commitment differs from its certificate")
	}
	freshAdmission, err := AdmitCaptureResearchLineagePayload(captureRaw, captureRun.AuthorizedCalls)
	if err != nil || admission.Digest != freshAdmission.Digest || admission.Admission != CaptureResearchAdmissionAdmitted {
		return capsule.VerificationReport{}, errors.New("identical-response research admission differs from exact capture lineage")
	}
	analysisRecord, err := identicalComponentByName(child.Manifest, "identical-response.offline-analysis")
	if err != nil {
		return capsule.VerificationReport{}, err
	}
	var analysis IdenticalResponseAnalysis
	if err := decodeIdenticalJSON(child.Payloads[analysisRecord.Payload.Digest], &analysis); err != nil {
		return capsule.VerificationReport{}, err
	}
	if err := analysis.Validate(); err != nil || root.AnalysisDigest != analysis.Digest || analysis.CaptureSHA256 != inspection.PayloadSHA256 || analysis.RouteID != inspection.RouteID || analysis.StudyRecordDigest != root.StudyRecordDigest || analysis.StudyManifestDigest != root.StudyManifestDigest || root.AnalysisSummary.TaskGroups != analysis.Summary.TaskGroups || root.AnalysisSummary.Complete != analysis.Summary.Complete || root.AnalysisSummary.Agreements != analysis.Summary.Agreements || root.AnalysisSummary.Disagreements != analysis.Summary.Disagreements || root.AnalysisSummary.Unresolved != analysis.Summary.Unresolved {
		return capsule.VerificationReport{}, errors.New("identical-response offline analysis differs from the resolved family")
	}
	return report, nil
}

func validateIdenticalStudyParentBindings(child capsule.Package, root IdenticalResponseEvidenceRoot) error {
	manifestRecord, err := identicalComponentByName(child.Manifest, "identical-response.study-manifest")
	if err != nil || manifestRecord.TypeID != IdenticalResponseStudyManifestComponentSchemaVersion {
		return errors.New("identical-response study manifest parent is unavailable")
	}
	var manifest study.Manifest
	if err := decodeIdenticalJSON(child.Payloads[manifestRecord.Payload.Digest], &manifest); err != nil {
		return err
	}
	manifestDigest, err := study.ManifestDigest(manifest)
	if err != nil || manifestDigest != root.StudyManifestDigest {
		return errors.New("identical-response outer bind study manifest commitment is invalid")
	}
	recordRecord, err := identicalComponentByName(child.Manifest, "identical-response.study-record")
	if err != nil || recordRecord.TypeID != IdenticalResponseStudyRecordComponentSchemaVersion {
		return errors.New("identical-response study record parent is unavailable")
	}
	var record study.Record
	if err := decodeIdenticalJSON(child.Payloads[recordRecord.Payload.Digest], &record); err != nil {
		return err
	}
	if record.RecordDigest != root.StudyRecordDigest || record.Study.ManifestDigest != manifestDigest || !reflect.DeepEqual(record.Study.Manifest, manifest) {
		return errors.New("identical-response outer bind study record commitment is invalid")
	}
	return nil
}

func validateIdenticalStudyRecordBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 1 || context.Parents[0].Reference.Resolution != capsule.ParentInternal || context.Parents[0].Record.TypeID != IdenticalResponseStudyManifestComponentSchemaVersion {
		return errors.New("identical-response study record requires one internal study manifest parent")
	}
	var record study.Record
	if err := decodeIdenticalJSON(context.Payload, &record); err != nil {
		return err
	}
	var manifest study.Manifest
	if err := decodeIdenticalJSON(context.Parents[0].Payload, &manifest); err != nil {
		return err
	}
	manifestDigest, err := study.ManifestDigest(manifest)
	if err != nil || record.Study.ManifestDigest != manifestDigest {
		return errors.New("identical-response study record does not bind its manifest digest")
	}
	return nil
}

func validateIdenticalAuthorizationBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 2 {
		return errors.New("identical-response live authorization requires route and study parents")
	}
	var authorization identicalResponseAuthorizationPlan
	if err := decodeIdenticalJSON(context.Payload, &authorization); err != nil {
		return err
	}
	var manifest study.Manifest
	var attestation conformance.Attestation
	for index := range context.Parents {
		parent := context.Parents[index]
		if parent.Reference.Resolution != capsule.ParentInternal {
			return errors.New("identical-response live authorization parents must be internal")
		}
		switch parent.Record.TypeID {
		case IdenticalResponseStudyManifestComponentSchemaVersion:
			if manifest.SchemaVersion != "" {
				return errors.New("identical-response live authorization repeats its study parent")
			}
			if err := decodeIdenticalJSON(parent.Payload, &manifest); err != nil {
				return err
			}
		case IdenticalResponseRouteAttestationComponentSchemaVersion:
			if attestation.SchemaVersion != "" {
				return errors.New("identical-response live authorization repeats its route parent")
			}
			if err := decodeIdenticalJSON(parent.Payload, &attestation); err != nil {
				return err
			}
		default:
			return errors.New("identical-response live authorization has an unsupported parent")
		}
	}
	if manifest.SchemaVersion == "" || attestation.SchemaVersion == "" {
		return errors.New("identical-response live authorization is missing a provenance parent")
	}
	return validateIdenticalAuthorizationValues(manifest, attestation, authorization)
}

func validateIdenticalAuthorizationValues(manifest study.Manifest, attestation conformance.Attestation, authorization identicalResponseAuthorizationPlan) error {
	if len(manifest.Arms) != 1 || len(manifest.Providers) != 1 {
		return errors.New("identical-response live authorization requires one study arm and provider plan")
	}
	manifestDigest, err := study.ManifestDigest(manifest)
	if err != nil || authorization.StudyManifestDigest != manifestDigest || authorization.RouteID != attestation.Identity.RouteID || authorization.RouteID != manifest.Arms[0].RouteID || authorization.RequestContractDigest != manifest.Arms[0].RequestContractDigest || authorization.Entrypoint != manifest.Arms[0].Entrypoint || authorization.ScorePolicyVersion != manifest.Arms[0].ScorePolicyVersion || authorization.ExpectedCalls != manifest.Budget.ExpectedCalls || authorization.WorstCalls < authorization.ExpectedCalls || authorization.WorstCalls > manifest.Budget.HardCalls || authorization.MaxWorkers != manifest.Budget.HardConcurrent || authorization.MaxRetries != manifest.Providers[0].MaxRetries || authorization.MinDispatchIntervalSeconds != manifest.Providers[0].MinDispatchIntervalSeconds {
		return errors.New("identical-response live authorization does not bind the study route or execution plan")
	}
	limits := authorization.Limits
	budget := manifest.Budget
	if limits.MaxCalls != budget.HardCalls || limits.MaxAttempts != budget.HardAttempts || limits.MaxEstimatedInputTokens != budget.HardInputTokens || limits.MaxReservedOutputTokens != budget.HardOutputTokens || limits.MaxConcurrent != budget.HardConcurrent || limits.MaxDurationSeconds != budget.HardDurationSeconds || limits.MaxCostUSD != budget.HardCostUSD {
		return errors.New("identical-response live authorization limits differ from the locked study budget")
	}
	return nil
}

func validateIdenticalAdmissionBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 1 || context.Parents[0].Reference.Resolution != capsule.ParentInternal || context.Parents[0].Record.TypeID != IdenticalResponseCaptureRunComponentSchemaVersion {
		return errors.New("identical-response research admission requires one internal capture-run parent")
	}
	var admission CaptureResearchAdmission
	if err := decodeIdenticalJSON(context.Payload, &admission); err != nil {
		return err
	}
	var captureRun CaptureRunAttestation
	if err := decodeIdenticalJSON(context.Parents[0].Payload, &captureRun); err != nil {
		return err
	}
	if admission.CaptureRunAttestationDigest != captureRun.Digest || admission.PayloadSHA256 != captureRun.Inspection.PayloadSHA256 || admission.AuthorizedCalls != captureRun.AuthorizedCalls || admission.ObservedCalls != captureRun.ObservedCalls {
		return errors.New("identical-response research admission does not bind its capture-run attestation")
	}
	return nil
}

func validateIdenticalAnalysisBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 4 {
		return errors.New("identical-response analysis requires four provenance parents")
	}
	var record study.Record
	var admission CaptureResearchAdmission
	var attestation conformance.Attestation
	var captureParent *capsule.BoundParent
	for index := range context.Parents {
		parent := &context.Parents[index]
		switch parent.Record.TypeID {
		case IdenticalResponseStudyRecordComponentSchemaVersion:
			if parent.Reference.Resolution != capsule.ParentInternal || record.SchemaVersion != "" {
				return errors.New("identical-response analysis repeats its study record parent")
			}
			if err := decodeIdenticalJSON(parent.Payload, &record); err != nil {
				return err
			}
		case IdenticalResponseAdmissionComponentSchemaVersion:
			if parent.Reference.Resolution != capsule.ParentInternal || admission.SchemaVersion != "" {
				return errors.New("identical-response analysis repeats its admission parent")
			}
			if err := decodeIdenticalJSON(parent.Payload, &admission); err != nil {
				return err
			}
		case IdenticalResponseRouteAttestationComponentSchemaVersion:
			if parent.Reference.Resolution != capsule.ParentInternal || attestation.SchemaVersion != "" {
				return errors.New("identical-response analysis repeats its route parent")
			}
			if err := decodeIdenticalJSON(parent.Payload, &attestation); err != nil {
				return err
			}
		case ResponseCaptureComponentSchemaVersion:
			if parent.Reference.Resolution != capsule.ParentExternal || captureParent != nil {
				return errors.New("identical-response analysis repeats its capture parent")
			}
			captureParent = parent
		default:
			return errors.New("identical-response analysis has an unsupported parent")
		}
	}
	if captureParent == nil || record.SchemaVersion == "" || admission.SchemaVersion == "" || attestation.SchemaVersion == "" {
		return errors.New("identical-response analysis is missing a provenance parent")
	}
	var analysis IdenticalResponseAnalysis
	if err := decodeIdenticalJSON(context.Payload, &analysis); err != nil {
		return err
	}
	if analysis.StudyRecordDigest != record.RecordDigest {
		return errors.New("identical-response analysis does not bind its study record digest")
	}
	if analysis.StudyManifestDigest != record.Study.ManifestDigest {
		return errors.New("identical-response analysis does not bind its study manifest digest")
	}
	if analysis.RouteID != attestation.Identity.RouteID {
		return errors.New("identical-response analysis does not bind its route identity")
	}
	if admission.Admission != CaptureResearchAdmissionAdmitted || admission.PayloadSHA256 != analysis.CaptureSHA256 {
		return errors.New("identical-response analysis is not based on admitted capture lineage")
	}
	return nil
}

func validateIdenticalOuterBindBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 8 {
		return errors.New("identical-response outer bind requires eight parents")
	}
	var root IdenticalResponseEvidenceRoot
	if err := decodeIdenticalJSON(context.Payload, &root); err != nil {
		return err
	}
	parents := make(map[string]capsule.BoundParent, len(context.Parents))
	for index := range context.Parents {
		parent := context.Parents[index]
		key := string(parent.Reference.Resolution) + "\x00" + parent.Record.TypeID
		if _, duplicate := parents[key]; duplicate {
			return errors.New("identical-response outer bind repeats a parent type")
		}
		parents[key] = parent
	}
	for _, ref := range root.Components {
		parent, found := parents[string(capsule.ParentInternal)+"\x00"+ref.TypeID]
		if !found || parent.Record.ComponentID != ref.ComponentID || parent.Record.Name != ref.Name || parent.Record.Payload.Digest != ref.PayloadDigest {
			return fmt.Errorf("identical-response outer bind component %q differs from its parent", ref.Name)
		}
	}
	baseParent, found := parents[string(capsule.ParentExternal)+"\x00"+ResponseBundleIndexSchemaVersion]
	if !found || baseParent.Record.ComponentID != root.BaseIndexComponentID {
		return errors.New("identical-response outer bind does not resolve its response-bundle index")
	}
	return nil
}

func (root IdenticalResponseEvidenceRoot) Validate() error {
	if root.SchemaVersion != IdenticalResponseOuterBindSchemaVersion || root.Status != StudyBindStatusComplete || root.EvidenceCeiling != IdenticalResponseEvidenceCeiling ||
		!validIdenticalText(root.StudyID) || !validIdenticalText(root.CellID) || !validIdenticalDigest(root.BaseCapsuleID) || !validIdenticalDigest(root.BaseIndexComponentID) || !validIdenticalDigest(root.BasePolicyComponentID) || !validIdenticalDigest(root.CaptureComponentID) ||
		!validIdenticalDigest(root.CapturePayloadSHA256) || root.AuthorizedCalls < 1 || root.ObservedCalls != root.AuthorizedCalls || root.CompleteResearchEntries != root.ObservedCalls ||
		!validIdenticalDigest(root.StudyManifestDigest) || !validIdenticalDigest(root.StudyRecordDigest) || !validIdenticalDigest(root.AuthorizationDigest) || !validIdenticalDigest(root.RouteAttestationDigest) || !validIdenticalDigest(root.CaptureRunAttestationDigest) || !validIdenticalDigest(root.AdmissionDigest) || !validIdenticalDigest(root.AnalysisDigest) ||
		root.ProviderCalls != 0 || root.NetworkRequired || root.Limitations == nil || !validSortedIdenticalText(root.Limitations) || root.Components == nil || len(root.Components) != 7 || !validIdenticalAnalysisSummary(root.AnalysisSummary) || !validIdenticalDigest(root.Digest) {
		return errors.New("identical-response outer bind identity or boundary is invalid")
	}
	previous := ""
	for _, ref := range root.Components {
		if !validIdenticalText(ref.Name) || !validIdenticalText(ref.TypeID) || !validIdenticalDigest(ref.ComponentID) || !validIdenticalDigest(ref.PayloadDigest) || ref.Name <= previous {
			return errors.New("identical-response outer bind component references are invalid or unsorted")
		}
		previous = ref.Name
	}
	digest := root
	digest.Digest = ""
	want, err := protocol.Digest(digest)
	if err != nil || want != root.Digest {
		return errors.New("identical-response outer bind digest is invalid")
	}
	return nil
}

func (analysis IdenticalResponseAnalysis) Validate() error {
	if analysis.SchemaVersion != IdenticalResponseAnalysisSchemaVersion || analysis.Counterfactual != "distribution_aware_vs_chosen_token" || !validIdenticalDigest(analysis.StudyRecordDigest) || !validIdenticalDigest(analysis.StudyManifestDigest) || !validIdenticalDigest(analysis.CaptureSHA256) || !validRouteID(analysis.RouteID) || analysis.Epsilon <= 0 || analysis.Epsilon >= 1 || analysis.ConfidenceLevel <= 0 || analysis.ConfidenceLevel > 1 || analysis.Rows == nil || analysis.Summary.TaskGroups != len(analysis.Rows) || !validIdenticalDigest(analysis.Digest) {
		return errors.New("identical-response analysis identity is invalid")
	}
	previous := ""
	counts := IdenticalResponseAnalysisSummaryCommitment{}
	outcome := IdenticalResponseOutcomeSummary{}
	for _, row := range analysis.Rows {
		if !validIdenticalText(row.GroupID) || !validIdenticalText(row.TaskID) || row.GroupID <= previous || !validIdenticalDecision(row.DistributionAware) || !validIdenticalDecision(row.ChosenToken) {
			return errors.New("identical-response analysis rows are invalid or unsorted")
		}
		previous = row.GroupID
		counts.TaskGroups++
		switch row.Status {
		case "agreement":
			counts.Complete++
			counts.Agreements++
		case "disagreement":
			counts.Complete++
			counts.Disagreements++
		case DecisionMissing:
			counts.Unresolved++
		default:
			return errors.New("identical-response analysis row status is invalid")
		}
		if row.OutcomeSensitivity.Available != (row.OutcomeSensitivity.DistributionReward != nil && row.OutcomeSensitivity.ChosenTokenReward != nil) {
			return errors.New("identical-response outcome row availability is invalid")
		}
		if row.OutcomeSensitivity.Available {
			outcome.Covered++
			switch {
			case *row.OutcomeSensitivity.DistributionReward > 0 && *row.OutcomeSensitivity.ChosenTokenReward > 0:
				outcome.BothSucceeded++
			case *row.OutcomeSensitivity.DistributionReward <= 0 && *row.OutcomeSensitivity.ChosenTokenReward <= 0:
				outcome.BothFailed++
			case *row.OutcomeSensitivity.DistributionReward > 0:
				outcome.DistributionOnlySuccess++
			default:
				outcome.ChosenTokenOnlySuccess++
			}
		}
	}
	if counts.TaskGroups != analysis.Summary.TaskGroups || counts.Complete != analysis.Summary.Complete || counts.Agreements != analysis.Summary.Agreements || counts.Disagreements != analysis.Summary.Disagreements || counts.Unresolved != analysis.Summary.Unresolved || outcome.Covered != analysis.OutcomeSensitivity.Covered || outcome.BothSucceeded != analysis.OutcomeSensitivity.BothSucceeded || outcome.BothFailed != analysis.OutcomeSensitivity.BothFailed || outcome.DistributionOnlySuccess != analysis.OutcomeSensitivity.DistributionOnlySuccess || outcome.ChosenTokenOnlySuccess != analysis.OutcomeSensitivity.ChosenTokenOnlySuccess {
		return errors.New("identical-response analysis summaries do not match rows")
	}
	denominator := float64(analysis.Summary.TaskGroups)
	if denominator <= 0 || analysis.Summary.DisagreementRate != float64(counts.Disagreements)/denominator || analysis.OutcomeSensitivity.CoverageRate != float64(outcome.Covered)/denominator || analysis.OutcomeSensitivity.McNemarExactP != stats.McNemarExact(outcome.DistributionOnlySuccess, outcome.ChosenTokenOnlySuccess) || analysis.OutcomeSensitivity.McNemarExactP < 0 || analysis.OutcomeSensitivity.McNemarExactP > 1 || analysis.Summary.MissingnessWorstCaseExactUpperBound < 0 || analysis.Summary.MissingnessWorstCaseExactUpperBound > 1 {
		return errors.New("identical-response analysis summary rates are invalid")
	}
	want, err := identicalResponseAnalysisDigest(analysis)
	if err != nil || want != analysis.Digest {
		return errors.New("identical-response analysis digest is invalid")
	}
	return nil
}

func validateIdenticalCaptureRunAttestation(attestation CaptureRunAttestation) error {
	if attestation.SchemaVersion != CaptureRunAttestationSchemaVersion || attestation.Status != CaptureRunStatusComplete || attestation.InspectionMode != CaptureRunInspectionExactReplay || !attestation.AttemptLedgerReconciled || !attestation.ResearchLineageComplete || attestation.AuthorizedCalls < 1 || attestation.ObservedCalls != attestation.AuthorizedCalls || attestation.Limitations == nil || !validSortedIdenticalText(attestation.Limitations) || attestation.Digest == "" {
		return errors.New("identical-response capture-run attestation is incomplete")
	}
	if err := attestation.Inspection.Validate(); err != nil {
		return err
	}
	unsigned := attestation
	unsigned.Digest = ""
	want, err := protocol.Digest(unsigned)
	if err != nil || want != attestation.Digest {
		return errors.New("identical-response capture-run attestation digest is invalid")
	}
	return nil
}

func validateIdenticalResearchAdmission(admission CaptureResearchAdmission) error {
	if admission.SchemaVersion != CaptureResearchAdmissionSchemaVersion || admission.Admission != CaptureResearchAdmissionAdmitted || admission.CaptureRunStatus != CaptureRunStatusComplete || admission.InspectionMode != CaptureRunInspectionExactReplay || admission.AuthorizedCalls < 1 || admission.ObservedCalls != admission.AuthorizedCalls || admission.CompleteResearchEntries != admission.ObservedCalls || admission.CapabilityAttestationIDs == nil || admission.Reasons == nil || admission.RequiredAction != "" || !validIdenticalDigest(admission.PayloadSHA256) || !validIdenticalDigest(admission.CaptureRunAttestationDigest) || !validIdenticalDigest(admission.Digest) {
		return errors.New("identical-response research admission is invalid")
	}
	unsigned := admission
	unsigned.Digest = ""
	want, err := protocol.Digest(unsigned)
	if err != nil || want != admission.Digest {
		return errors.New("identical-response research admission digest is invalid")
	}
	return nil
}

func validateIdenticalResponseInputs(base capsule.Package, policy ResponseBundlePolicy, captureRecord capsule.ComponentRecord, inspection CaptureInspection, manifest study.Manifest, record study.Record, authorization identicalResponseAuthorizationPlan, attestation conformance.Attestation, captureRun CaptureRunAttestation, admission CaptureResearchAdmission, analysis IdenticalResponseAnalysis) error {
	manifestDigest, err := study.ManifestDigest(manifest)
	if err != nil || policy.StudyID != base.Manifest.StudyID || policy.CellID != base.Manifest.CellID || "study-"+manifestDigest != policy.StudyID {
		return errors.New("identical-response study manifest does not bind the response-bundle study")
	}
	if record.State != study.StateComplete || record.Study.ManifestDigest == "" {
		return errors.New("identical-response study record is not complete")
	}
	if record.Study.ManifestDigest != mustStudyManifestDigest(manifest) || !reflect.DeepEqual(record.Study.Manifest, manifest) {
		return errors.New("identical-response study record does not bind the supplied study manifest")
	}
	if len(inspection.ServedModels) != 1 || attestation.State != conformance.StateBoundedQualified || attestation.Identity.RouteID != inspection.RouteID || attestation.Identity.ProviderID != inspection.ProviderID || attestation.Identity.RequestedModel != inspection.RequestedModel || attestation.Identity.ServedModel != inspection.ServedModels[0] {
		return errors.New("identical-response route attestation does not bind the capture route")
	}
	if err := validateIdenticalAuthorizationValues(manifest, attestation, authorization); err != nil {
		return err
	}
	if err := validateIdenticalCaptureRunAttestation(captureRun); err != nil || captureRun.Inspection.PayloadSHA256 != captureRecord.Payload.Digest || captureRun.ObservedCalls != inspection.Entries {
		return errors.New("identical-response capture-run attestation does not bind the capture")
	}
	if captureRun.AuthorizedCalls != manifest.Budget.ExpectedCalls || captureRun.AuthorizedCalls > manifest.Budget.HardCalls {
		return errors.New("identical-response capture budget differs from the locked study budget")
	}
	freshAdmission, err := AdmitCaptureResearchLineagePayload(base.Payloads[captureRecord.Payload.Digest], captureRun.AuthorizedCalls)
	if err != nil || admission.Digest != freshAdmission.Digest || admission.Admission != CaptureResearchAdmissionAdmitted {
		return errors.New("identical-response admission does not bind the capture")
	}
	if err := analysis.Validate(); err != nil || analysis.StudyRecordDigest != record.RecordDigest || analysis.StudyManifestDigest != record.Study.ManifestDigest || analysis.CaptureSHA256 != inspection.PayloadSHA256 || analysis.RouteID != inspection.RouteID {
		return errors.New("identical-response offline analysis does not bind the sealed inputs")
	}
	return nil
}

func decodeIdenticalStudyManifest(raw []byte) (study.Manifest, []byte, error) {
	var manifest study.Manifest
	if err := decodeIdenticalJSON(raw, &manifest); err != nil {
		return study.Manifest{}, nil, err
	}
	if err := manifest.Validate(); err != nil {
		return study.Manifest{}, nil, err
	}
	return manifest, bytes.Clone(raw), nil
}

func decodeIdenticalStudyRecord(raw []byte) (study.Record, []byte, error) {
	var record study.Record
	if err := decodeIdenticalJSON(raw, &record); err != nil {
		return study.Record{}, nil, err
	}
	if err := record.Validate(); err != nil {
		return study.Record{}, nil, err
	}
	return record, bytes.Clone(raw), nil
}

func decodeIdenticalAuthorization(raw []byte) (identicalResponseAuthorizationPlan, []byte, error) {
	var authorization identicalResponseAuthorizationPlan
	if err := decodeIdenticalJSON(raw, &authorization); err != nil {
		return identicalResponseAuthorizationPlan{}, nil, err
	}
	if authorization.SchemaVersion != identicalResponseLiveAuthorizationSchemaVersion || authorization.AuthorizationDigest == "" {
		return identicalResponseAuthorizationPlan{}, nil, errors.New("live authorization is not a valid TASK-070 plan")
	}
	want, err := authorization.Digest()
	if err != nil || want != authorization.AuthorizationDigest {
		return identicalResponseAuthorizationPlan{}, nil, errors.New("live authorization digest is invalid")
	}
	return authorization, bytes.Clone(raw), nil
}

func decodeIdenticalRouteAttestation(raw []byte) (conformance.Attestation, []byte, error) {
	var attestation conformance.Attestation
	if err := decodeIdenticalJSON(raw, &attestation); err != nil {
		return conformance.Attestation{}, nil, err
	}
	if err := attestation.ValidateIntegrity(); err != nil || attestation.State != conformance.StateBoundedQualified {
		return conformance.Attestation{}, nil, errors.New("route attestation is not a valid bounded qualification")
	}
	return attestation, bytes.Clone(raw), nil
}

func decodeIdenticalCaptureRunAttestation(raw []byte) (CaptureRunAttestation, []byte, error) {
	var attestation CaptureRunAttestation
	if err := decodeIdenticalJSON(raw, &attestation); err != nil {
		return CaptureRunAttestation{}, nil, err
	}
	if err := validateIdenticalCaptureRunAttestation(attestation); err != nil {
		return CaptureRunAttestation{}, nil, err
	}
	return attestation, bytes.Clone(raw), nil
}

func decodeIdenticalResearchAdmission(raw []byte) (CaptureResearchAdmission, []byte, error) {
	var admission CaptureResearchAdmission
	if err := decodeIdenticalJSON(raw, &admission); err != nil {
		return CaptureResearchAdmission{}, nil, err
	}
	if err := validateIdenticalResearchAdmission(admission); err != nil {
		return CaptureResearchAdmission{}, nil, err
	}
	return admission, bytes.Clone(raw), nil
}

func decodeIdenticalAnalysis(raw []byte) (IdenticalResponseAnalysis, []byte, error) {
	var analysis IdenticalResponseAnalysis
	if err := decodeIdenticalJSON(raw, &analysis); err != nil {
		return IdenticalResponseAnalysis{}, nil, err
	}
	if err := analysis.Validate(); err != nil {
		return IdenticalResponseAnalysis{}, nil, err
	}
	return analysis, bytes.Clone(raw), nil
}

func decodeIdenticalJSON(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("identical-response artifact contains more than one JSON value")
		}
		return err
	}
	return nil
}

func identicalResponseBaseCapture(base capsule.Package, index ResponseBundleIndex) (capsule.ComponentRecord, []byte, CaptureInspection, error) {
	if len(index.Captures) != 1 {
		return capsule.ComponentRecord{}, nil, CaptureInspection{}, errors.New("identical-response outer capsule requires exactly one response capture")
	}
	captureRecord, err := componentByID(base.Manifest, index.Captures[0].ComponentID)
	if err != nil || captureRecord.TypeID != ResponseCaptureComponentSchemaVersion {
		return capsule.ComponentRecord{}, nil, CaptureInspection{}, errors.New("identical-response response capture parent is unavailable")
	}
	captureRaw, found := base.Payloads[captureRecord.Payload.Digest]
	if !found {
		return capsule.ComponentRecord{}, nil, CaptureInspection{}, errors.New("identical-response response capture payload is unavailable")
	}
	inspection, _, err := inspectCapturePayload(captureRaw)
	if err != nil {
		return capsule.ComponentRecord{}, nil, CaptureInspection{}, err
	}
	if !reflect.DeepEqual(index.Captures[0].CaptureInspection, inspection) {
		return capsule.ComponentRecord{}, nil, CaptureInspection{}, errors.New("identical-response response capture differs from its index")
	}
	return captureRecord, captureRaw, inspection, nil
}

func identicalExternalParent(kind capsule.EdgeKind, record capsule.ComponentRecord, capsuleID string) capsule.ParentRef {
	return capsule.ParentRef{Kind: kind, ComponentID: record.ComponentID, TypeID: record.TypeID, Role: record.Role, Visibility: record.Visibility, Resolution: capsule.ParentExternal, CapsuleID: capsuleID}
}

func identicalInternalParent(kind capsule.EdgeKind, record capsule.ComponentRecord) capsule.ParentRef {
	return capsule.ParentRef{Kind: kind, ComponentID: record.ComponentID, TypeID: record.TypeID, Role: record.Role, Visibility: record.Visibility, Resolution: capsule.ParentInternal}
}

func identicalComponentByName(manifest capsule.Manifest, name string) (capsule.ComponentRecord, error) {
	var result capsule.ComponentRecord
	found := 0
	for _, record := range manifest.Components {
		if record.Name == name {
			result, found = record, found+1
		}
	}
	if found != 1 {
		return capsule.ComponentRecord{}, fmt.Errorf("identical-response capsule requires exactly one component named %q", name)
	}
	return result, nil
}

func componentByID(manifest capsule.Manifest, id string) (capsule.ComponentRecord, error) {
	for _, record := range manifest.Components {
		if record.ComponentID == id {
			return record, nil
		}
	}
	return capsule.ComponentRecord{}, fmt.Errorf("capsule component %q is unavailable", id)
}

func mustStudyManifestDigest(manifest study.Manifest) string {
	digest, _ := study.ManifestDigest(manifest)
	return digest
}

func validIdenticalDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func validIdenticalText(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= 4096
}

func validSortedIdenticalText(values []string) bool {
	if values == nil {
		return false
	}
	copy := slices.Clone(values)
	slices.Sort(copy)
	return slices.Equal(copy, values) && len(slices.Compact(copy)) == len(values) && slices.IndexFunc(values, func(value string) bool { return !validIdenticalText(value) }) < 0
}

func validIdenticalAnalysisSummary(summary IdenticalResponseAnalysisSummaryCommitment) bool {
	return summary.TaskGroups > 0 && summary.Complete >= 0 && summary.Agreements >= 0 && summary.Disagreements >= 0 && summary.Unresolved >= 0 && summary.Complete == summary.Agreements+summary.Disagreements && summary.TaskGroups == summary.Complete+summary.Unresolved
}

func validIdenticalDecision(decision IdenticalResponseDecision) bool {
	if decision.State != DecisionSelected && decision.State != DecisionTied && decision.State != DecisionMissing || len(decision.Scores) == 0 || math.IsNaN(decision.Margin) || math.IsInf(decision.Margin, 0) {
		return false
	}
	for _, score := range decision.Scores {
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return false
		}
	}
	if decision.State == DecisionSelected {
		return decision.SelectedIndex != nil && *decision.SelectedIndex >= 0 && *decision.SelectedIndex < len(decision.Scores)
	}
	return decision.SelectedIndex == nil
}
