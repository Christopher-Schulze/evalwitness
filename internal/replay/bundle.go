package replay

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	ResponseBundlePolicySchemaVersion          = "evalwitness.response-bundle-policy.v1"
	ResponseBundleRedistributionSchemaVersion  = "evalwitness.redistribution-evidence.v1"
	ResponseCaptureComponentSchemaVersion      = "evalwitness.response-capture.v1"
	ResponseBundleIndexSchemaVersion           = "evalwitness.response-bundle-index.v1"
	ResponseBundleVerificationSchemaVersion    = "evalwitness.response-bundle-verification.v1"
	ResponseBundleRegistryID                   = "evalwitness.response-bundle-registry.v1"
	ResponseBundleLineageExactFixture          = "exact_fixture"
	ResponseBundleLineageCompleteResearch      = "complete_research"
	ResponseBundleEvidenceMechanismConformance = "mechanism_conformance"
	ResponseBundleEvidenceExternalUnresolved   = "record_complete_external_parents_unresolved"

	responseBundlePolicyValidatorID              = "evalwitness.validator.response-bundle-policy.v1"
	responseBundlePolicyBindingID                = "evalwitness.validator.response-bundle-policy-bindings.v1"
	responseBundleRedistributionValidatorID      = "evalwitness.validator.redistribution-evidence.v1"
	responseBundleSourceValidatorID              = "evalwitness.validator.response-bundle-source-tree.v1"
	responseBundleBuildValidatorID               = "evalwitness.validator.response-bundle-build.v1"
	responseBundleBuildBindingID                 = "evalwitness.validator.response-bundle-build-bindings.v1"
	responseCaptureValidatorID                   = "evalwitness.validator.response-capture.v1"
	responseCaptureBindingID                     = "evalwitness.validator.response-capture-bindings.v1"
	responseBundleIndexValidatorID               = "evalwitness.validator.response-bundle-index.v1"
	responseBundleIndexBindingID                 = "evalwitness.validator.response-bundle-index-bindings.v1"
	responseBundleSourceTreePolicy               = "evalwitness.git-visible-source-tree.v1"
	responseBundleArtifactKind                   = "response-bundle-producer"
	responseBundleMaxCaptureBytes                = int64(512 << 20)
	responseBundleMaxRedistributionEvidenceBytes = int64(16 << 20)
)

var responseBundleOmittedClasses = []string{
	"credentials",
	"legacy_schema1_cache",
	"local_configuration",
	"operational_logs",
	"provider_account_metadata",
}

type ResponseBundleProducer struct {
	Version             string `json:"version"`
	Toolchain           string `json:"toolchain"`
	SourceTreeAlgorithm string `json:"source_tree_algorithm"`
	SourceTreeDigest    string `json:"source_tree_digest"`
	BinarySHA256        string `json:"binary_sha256"`
}

type ResponseBundleDataset struct {
	ID                string `json:"id"`
	Digest            string `json:"digest"`
	LicenseExpression string `json:"license_expression"`
}

type ResponseBundlePolicy struct {
	SchemaVersion                string                 `json:"schema_version"`
	StudyID                      string                 `json:"study_id"`
	CellID                       string                 `json:"cell_id"`
	LineagePolicy                string                 `json:"lineage_policy"`
	Producer                     ResponseBundleProducer `json:"producer"`
	Dataset                      ResponseBundleDataset  `json:"dataset"`
	ResponseLicenseExpression    string                 `json:"response_license_expression"`
	RedistributionStatus         string                 `json:"redistribution_status"`
	RedistributionBasis          string                 `json:"redistribution_basis"`
	RedistributionEvidenceDigest string                 `json:"redistribution_evidence_digest"`
	AllowedCaptureNames          []string               `json:"allowed_capture_names"`
	OmittedClasses               []string               `json:"omitted_classes"`
	ExactCaptureOnly             bool                   `json:"exact_capture_only"`
	PublicArtifactScanRequired   bool                   `json:"public_artifact_scan_required"`
	ProviderCalls                int                    `json:"provider_calls"`
	NetworkRequired              bool                   `json:"network_required"`
	Digest                       string                 `json:"digest"`
}

type ResponseBundleProducerEvidence struct {
	SourceTree capsule.SourceTreeProvenance
	Build      capsule.BuildProvenance
}

type inspectedResponseCapture struct {
	Name        string
	Raw         []byte
	RequestKeys []string
	Inspection  CaptureInspection
}

type responseBundleRequestPartition struct {
	Name             string `json:"name"`
	Entries          int    `json:"entries"`
	RequestSetDigest string `json:"request_set_digest"`
}

type CaptureInspection struct {
	PayloadSHA256            string   `json:"payload_sha256"`
	Bytes                    int64    `json:"bytes"`
	Entries                  int      `json:"entries"`
	CompleteResearchEntries  int      `json:"complete_research_entries"`
	StudyCellIDs             []string `json:"study_cell_ids"`
	ProviderID               string   `json:"provider_id"`
	RouteID                  string   `json:"route_id"`
	BaseURLOrigin            string   `json:"base_url_origin"`
	EndpointPath             string   `json:"endpoint_path"`
	EndpointKind             string   `json:"endpoint_kind"`
	RequestedModel           string   `json:"requested_model"`
	CaptureSchemaVersion     int      `json:"capture_schema_version"`
	RequestSchemaVersion     int      `json:"request_schema_version"`
	ParserContractVersion    string   `json:"parser_contract_version"`
	FirstReceivedAt          int64    `json:"first_received_at"`
	LastReceivedAt           int64    `json:"last_received_at"`
	LogProbabilityEntries    int      `json:"logprob_entries"`
	JudgeEntries             int      `json:"judge_entries"`
	ServedModels             []string `json:"served_models"`
	CheckpointAssertions     []string `json:"checkpoint_assertions"`
	CapabilityAttestationIDs []string `json:"capability_attestation_ids"`
	RequestSetDigest         string   `json:"request_set_digest"`
	LineageSetDigest         string   `json:"lineage_set_digest"`
	ResponseBodySetDigest    string   `json:"response_body_set_digest"`
	EvidenceSetDigest        string   `json:"evidence_set_digest"`
	RecordSetDigest          string   `json:"record_set_digest"`
}

type ResponseBundleCapture struct {
	Name        string `json:"name"`
	ComponentID string `json:"component_id"`
	CaptureInspection
}

type ResponseBundleIndex struct {
	SchemaVersion     string                  `json:"schema_version"`
	PolicyComponentID string                  `json:"policy_component_id"`
	PolicyDigest      string                  `json:"policy_digest"`
	Captures          []ResponseBundleCapture `json:"captures"`
	CaptureSetDigest  string                  `json:"capture_set_digest"`
	TotalEntries      int                     `json:"total_entries"`
	TotalBytes        int64                   `json:"total_bytes"`
	ExactReplay       bool                    `json:"exact_replay"`
	ProviderCalls     int                     `json:"provider_calls"`
	Offline           bool                    `json:"offline"`
	Digest            string                  `json:"digest"`
}

type ResponseBundleBuild struct {
	Package    capsule.Package
	Index      ResponseBundleIndex
	PublicScan safety.ArtifactScanReport
}

type VerifiedResponseCapture struct {
	Name        string `json:"name"`
	ComponentID string `json:"component_id"`
	PayloadPath string `json:"payload_path"`
	CaptureInspection
}

type ResponseBundleVerificationReport struct {
	SchemaVersion                string                     `json:"schema_version"`
	Capsule                      capsule.VerificationReport `json:"capsule"`
	StudyID                      string                     `json:"study_id"`
	CellID                       string                     `json:"cell_id"`
	PolicyDigest                 string                     `json:"policy_digest"`
	IndexDigest                  string                     `json:"index_digest"`
	CaptureSetDigest             string                     `json:"capture_set_digest"`
	DatasetID                    string                     `json:"dataset_id"`
	RequestCorpusDigest          string                     `json:"request_corpus_digest"`
	LineagePolicy                string                     `json:"lineage_policy"`
	EvidenceCeiling              string                     `json:"evidence_ceiling"`
	SourceTreeDigest             string                     `json:"source_tree_digest"`
	BinarySHA256                 string                     `json:"binary_sha256"`
	RedistributionEvidenceSHA256 string                     `json:"redistribution_evidence_sha256"`
	Captures                     []VerifiedResponseCapture  `json:"captures"`
	TotalEntries                 int                        `json:"total_entries"`
	TotalBytes                   int64                      `json:"total_bytes"`
	PublicScan                   safety.ArtifactScanReport  `json:"public_scan"`
	ProviderCalls                int                        `json:"provider_calls"`
	NetworkRequired              bool                       `json:"network_required"`
	ExactReplay                  bool                       `json:"exact_replay"`
	Valid                        bool                       `json:"valid"`
}

func (policy ResponseBundlePolicy) Validate() error {
	if policy.SchemaVersion != ResponseBundlePolicySchemaVersion ||
		!validBundleIdentifier(policy.StudyID) || !validBundleIdentifier(policy.CellID) ||
		(policy.LineagePolicy != ResponseBundleLineageExactFixture &&
			policy.LineagePolicy != ResponseBundleLineageCompleteResearch) ||
		policy.Producer.Version != "sha256:"+policy.Producer.BinarySHA256 ||
		!strings.HasPrefix(policy.Producer.Toolchain, "go") || !validBundleText(policy.Producer.Toolchain) ||
		policy.Producer.SourceTreeAlgorithm != responseBundleSourceTreePolicy ||
		!validBundleDigest(policy.Producer.SourceTreeDigest) || !validBundleDigest(policy.Producer.BinarySHA256) ||
		!validBundleIdentifier(policy.Dataset.ID) || !validBundleDigest(policy.Dataset.Digest) ||
		!validBundleText(policy.Dataset.LicenseExpression) || !validBundleText(policy.ResponseLicenseExpression) ||
		policy.RedistributionStatus != "authorized" || !validBundleIdentifier(policy.RedistributionBasis) ||
		!validBundleDigest(policy.RedistributionEvidenceDigest) ||
		!validCaptureNames(policy.AllowedCaptureNames) ||
		!slices.Equal(policy.OmittedClasses, responseBundleOmittedClasses) ||
		!policy.ExactCaptureOnly || !policy.PublicArtifactScanRequired ||
		policy.ProviderCalls != 0 || policy.NetworkRequired || !validBundleDigest(policy.Digest) {
		return errors.New("response bundle policy identity or publication boundary is invalid")
	}
	digest, err := responseBundlePolicyDigest(policy)
	if err != nil || digest != policy.Digest {
		return errors.New("response bundle policy digest is invalid")
	}
	return nil
}

func SealResponseBundlePolicy(policy ResponseBundlePolicy) (ResponseBundlePolicy, error) {
	policy.Digest = ""
	digest, err := responseBundlePolicyDigest(policy)
	if err != nil {
		return ResponseBundlePolicy{}, err
	}
	policy.Digest = digest
	return policy, policy.Validate()
}

func CollectResponseBundleProducerEvidence(ctx context.Context, repositoryRoot, binaryPath string) (ResponseBundleProducerEvidence, error) {
	source, build, err := capsule.CollectBuildProvenance(ctx, repositoryRoot, binaryPath, responseBundleArtifactKind)
	if err != nil {
		return ResponseBundleProducerEvidence{}, err
	}
	evidence := ResponseBundleProducerEvidence{SourceTree: source, Build: build}
	if err := evidence.Validate(); err != nil {
		return ResponseBundleProducerEvidence{}, err
	}
	return evidence, nil
}

func (evidence ResponseBundleProducerEvidence) Producer() ResponseBundleProducer {
	return ResponseBundleProducer{
		Version: "sha256:" + evidence.Build.BinaryDigest, Toolchain: evidence.Build.GoVersion,
		SourceTreeAlgorithm: evidence.SourceTree.Algorithm, SourceTreeDigest: evidence.SourceTree.Digest,
		BinarySHA256: evidence.Build.BinaryDigest,
	}
}

func (evidence ResponseBundleProducerEvidence) Validate() error {
	if err := evidence.SourceTree.Validate(); err != nil {
		return err
	}
	if err := evidence.Build.Validate(); err != nil {
		return err
	}
	if evidence.Build.ArtifactKind != responseBundleArtifactKind || evidence.Build.SourceTreeDigest != evidence.SourceTree.Digest ||
		evidence.Build.Commit != evidence.SourceTree.Commit || evidence.Build.Dirty != evidence.SourceTree.Dirty {
		return errors.New("response bundle producer build does not attest its source tree")
	}
	return nil
}

func EncodeResponseBundlePolicy(policy ResponseBundlePolicy) ([]byte, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return protocol.CanonicalMarshal(policy)
}

func DecodeResponseBundlePolicy(raw []byte) (ResponseBundlePolicy, error) {
	raw = bytes.TrimSuffix(raw, []byte{'\n'})
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(raw, &policy); err != nil {
		return ResponseBundlePolicy{}, err
	}
	canonical, err := protocol.CanonicalMarshal(policy)
	if err != nil || !bytes.Equal(canonical, raw) {
		return ResponseBundlePolicy{}, errors.New("response bundle policy is not canonical JSON")
	}
	return policy, policy.Validate()
}

func SealResponseBundlePolicyDraft(raw []byte, evidence ResponseBundleProducerEvidence, redistributionEvidencePath string, sources map[string]string) (ResponseBundlePolicy, error) {
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(raw, &policy); err != nil {
		return ResponseBundlePolicy{}, err
	}
	if policy.Digest != "" || policy.Producer != (ResponseBundleProducer{}) || policy.Dataset.Digest != "" {
		return ResponseBundlePolicy{}, errors.New("response bundle policy draft already has sealed producer, request-corpus, or policy identity")
	}
	if err := evidence.Validate(); err != nil {
		return ResponseBundlePolicy{}, err
	}
	if err := validateResponseBundleProducerPolicy(policy, evidence); err != nil {
		return ResponseBundlePolicy{}, err
	}
	redistributionEvidence, err := readRedistributionEvidence(redistributionEvidencePath)
	if err != nil {
		return ResponseBundlePolicy{}, err
	}
	if protocol.DigestBytes(redistributionEvidence) != policy.RedistributionEvidenceDigest {
		return ResponseBundlePolicy{}, errors.New("redistribution evidence differs from the policy digest")
	}
	if !sameCaptureNames(policy.AllowedCaptureNames, sources) {
		return ResponseBundlePolicy{}, errors.New("response bundle sources differ from the policy capture set")
	}
	inspected, err := inspectResponseBundleSources(policy.AllowedCaptureNames, sources)
	if err != nil {
		return ResponseBundlePolicy{}, err
	}
	if err := validateResponseBundleLineagePolicy(policy, inspected); err != nil {
		return ResponseBundlePolicy{}, err
	}
	policy.Dataset.Digest, err = responseBundleRequestCorpusDigest(inspected)
	if err != nil {
		return ResponseBundlePolicy{}, err
	}
	policy.Producer = evidence.Producer()
	return SealResponseBundlePolicy(policy)
}

func BuildResponseBundle(policy ResponseBundlePolicy, evidence ResponseBundleProducerEvidence, redistributionEvidencePath string, sources map[string]string, knownSecrets []string, reviewedFindings []safety.ArtifactFinding) (ResponseBundleBuild, error) {
	if err := policy.Validate(); err != nil {
		return ResponseBundleBuild{}, err
	}
	if err := evidence.Validate(); err != nil {
		return ResponseBundleBuild{}, err
	}
	if policy.Producer != evidence.Producer() {
		return ResponseBundleBuild{}, errors.New("response bundle policy producer differs from the supplied source tree or binary")
	}
	if err := validateResponseBundleProducerPolicy(policy, evidence); err != nil {
		return ResponseBundleBuild{}, err
	}
	if !sameCaptureNames(policy.AllowedCaptureNames, sources) {
		return ResponseBundleBuild{}, errors.New("response bundle sources differ from the policy capture set")
	}
	redistributionEvidence, err := readRedistributionEvidence(redistributionEvidencePath)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	if protocol.DigestBytes(redistributionEvidence) != policy.RedistributionEvidenceDigest {
		return ResponseBundleBuild{}, errors.New("redistribution evidence differs from the sealed policy digest")
	}
	inspected, err := inspectResponseBundleSources(policy.AllowedCaptureNames, sources)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	if err := validateResponseBundleLineagePolicy(policy, inspected); err != nil {
		return ResponseBundleBuild{}, err
	}
	requestCorpusDigest, err := responseBundleRequestCorpusDigest(inspected)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	if requestCorpusDigest != policy.Dataset.Digest {
		return ResponseBundleBuild{}, errors.New("response bundle exact request corpus differs from the sealed dataset digest")
	}
	registry, err := ResponseBundleRegistry()
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	sourceRaw, err := protocol.CanonicalMarshal(evidence.SourceTree)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	sourceRecord, normalizedSource, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "response.producer.source-tree", TypeID: capsule.SourceTreeProvenanceSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: sourceRaw,
	})
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	buildRaw, err := protocol.CanonicalMarshal(evidence.Build)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	buildRecord, normalizedBuild, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "response.producer.build", TypeID: capsule.BuildProvenanceSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: buildRaw,
		Parents: []capsule.ParentRef{internalResponseParent(capsule.EdgeAttests, sourceRecord)},
	})
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	redistributionRecord, normalizedRedistribution, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "response.bundle.redistribution-evidence", TypeID: ResponseBundleRedistributionSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: redistributionEvidence,
	})
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	policyRaw, err := EncodeResponseBundlePolicy(policy)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	policyRecord, normalizedPolicy, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "response.bundle.policy", TypeID: ResponseBundlePolicySchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: policyRaw,
		Parents: []capsule.ParentRef{
			internalResponseParent(capsule.EdgeDerivedFrom, sourceRecord),
			internalResponseParent(capsule.EdgeDerivedFrom, buildRecord),
			internalResponseParent(capsule.EdgeDerivedFrom, redistributionRecord),
		},
	})
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	records := []capsule.ComponentRecord{sourceRecord, buildRecord, redistributionRecord, policyRecord}
	payloads := map[string][]byte{
		sourceRecord.Payload.Digest:         normalizedSource,
		buildRecord.Payload.Digest:          normalizedBuild,
		redistributionRecord.Payload.Digest: normalizedRedistribution,
		policyRecord.Payload.Digest:         normalizedPolicy,
	}
	captures := make([]ResponseBundleCapture, 0, len(policy.AllowedCaptureNames))
	captureRecords := make([]capsule.ComponentRecord, 0, len(policy.AllowedCaptureNames))
	for _, capture := range inspected {
		record, normalized, err := capsule.BuildComponent(registry, capsule.ComponentInput{
			Name: "response.capture." + capture.Name, TypeID: ResponseCaptureComponentSchemaVersion,
			Visibility: capsule.VisibilityPublic, Payload: capture.Raw,
			Parents: []capsule.ParentRef{internalResponseParent(capsule.EdgeGovernedBy, policyRecord)},
		})
		if err != nil {
			return ResponseBundleBuild{}, err
		}
		if err := addResponsePayload(payloads, record.Payload.Digest, normalized); err != nil {
			return ResponseBundleBuild{}, err
		}
		records = append(records, record)
		captureRecords = append(captureRecords, record)
		captures = append(captures, ResponseBundleCapture{Name: capture.Name, ComponentID: record.ComponentID, CaptureInspection: capture.Inspection})
	}
	index, err := buildResponseBundleIndex(policy, policyRecord, captures)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	indexRaw, err := protocol.CanonicalMarshal(index)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	parents := []capsule.ParentRef{internalResponseParent(capsule.EdgeGovernedBy, policyRecord)}
	for _, record := range captureRecords {
		parents = append(parents, internalResponseParent(capsule.EdgeDerivedFrom, record))
	}
	indexRecord, normalizedIndex, err := capsule.BuildComponent(registry, capsule.ComponentInput{
		Name: "response.bundle.index", TypeID: ResponseBundleIndexSchemaVersion,
		Visibility: capsule.VisibilityPublic, Payload: indexRaw, Parents: parents,
	})
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	if err := addResponsePayload(payloads, indexRecord.Payload.Digest, normalizedIndex); err != nil {
		return ResponseBundleBuild{}, err
	}
	records = append(records, indexRecord)
	manifest, err := capsule.BuildManifest(registry, capsule.ManifestInput{
		StudyID: policy.StudyID, CellID: policy.CellID,
		ScientificRoots: []string{indexRecord.ComponentID}, PresentationRoots: []string{}, Components: records,
	})
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	pack := capsule.Package{Registry: registry, Manifest: manifest, Payloads: payloads}
	if _, err := capsule.VerifyPackage(context.Background(), registry, manifest, payloads, capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}); err != nil {
		return ResponseBundleBuild{}, err
	}
	scan, err := scanExactResponseBundlePackage(pack, knownSecrets, reviewedFindings)
	if err != nil {
		return ResponseBundleBuild{}, err
	}
	return ResponseBundleBuild{Package: pack, Index: index, PublicScan: scan}, nil
}

func VerifyResponseBundleDirectory(ctx context.Context, source string, knownSecrets []string, reviewedFindings []safety.ArtifactFinding) (ResponseBundleVerificationReport, error) {
	if ctx == nil || source == "" {
		return ResponseBundleVerificationReport{}, errors.New("response bundle verification requires context and source")
	}
	registry, err := ResponseBundleRegistry()
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	options := capsule.VerificationOptions{MaximumVisibility: capsule.VisibilityPublic}
	capsuleReport, err := capsule.VerifyDirectory(ctx, source, registry, options)
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	manifest, payloads, err := capsule.LoadDirectory(ctx, source, registry, options)
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	scan, err := safety.ScanArtifacts(safety.ArtifactScanRequest{
		Roots: []string{source}, Class: safety.ArtifactPublic, KnownSecrets: slices.Clone(knownSecrets),
		Limits:           responseBundleArtifactScanLimits(),
		ReviewedFindings: reviewedFindings,
	})
	if err != nil {
		return ResponseBundleVerificationReport{}, fmt.Errorf("response bundle public scan: %w", err)
	}
	indexRecord, err := responseComponentByType(manifest.Components, ResponseBundleIndexSchemaVersion)
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	var index ResponseBundleIndex
	if err := protocol.DecodeStrict(payloads[indexRecord.Payload.Digest], &index); err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	policyRecord, err := responseComponentByType(manifest.Components, ResponseBundlePolicySchemaVersion)
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(payloads[policyRecord.Payload.Digest], &policy); err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	if manifest.StudyID != policy.StudyID || manifest.CellID != policy.CellID {
		return ResponseBundleVerificationReport{}, errors.New("response bundle manifest study or cell differs from its sealed policy")
	}
	sourceRecord, err := responseComponentByType(manifest.Components, capsule.SourceTreeProvenanceSchemaVersion)
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	buildRecord, err := responseComponentByType(manifest.Components, capsule.BuildProvenanceSchemaVersion)
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	redistributionRecord, err := responseComponentByType(manifest.Components, ResponseBundleRedistributionSchemaVersion)
	if err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	var sourceProvenance capsule.SourceTreeProvenance
	if err := protocol.DecodeStrict(payloads[sourceRecord.Payload.Digest], &sourceProvenance); err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	var buildProvenance capsule.BuildProvenance
	if err := protocol.DecodeStrict(payloads[buildRecord.Payload.Digest], &buildProvenance); err != nil {
		return ResponseBundleVerificationReport{}, err
	}
	verified := make([]VerifiedResponseCapture, 0, len(index.Captures))
	for _, capture := range index.Captures {
		verified = append(verified, VerifiedResponseCapture{
			Name: capture.Name, ComponentID: capture.ComponentID,
			PayloadPath:       filepath.ToSlash(filepath.Join("components", capsule.DigestAlgorithm, capture.PayloadSHA256)),
			CaptureInspection: capture.CaptureInspection,
		})
	}
	return ResponseBundleVerificationReport{
		SchemaVersion: ResponseBundleVerificationSchemaVersion, Capsule: capsuleReport,
		StudyID: policy.StudyID, CellID: policy.CellID,
		PolicyDigest: index.PolicyDigest, IndexDigest: index.Digest, CaptureSetDigest: index.CaptureSetDigest,
		DatasetID: policy.Dataset.ID, RequestCorpusDigest: policy.Dataset.Digest,
		LineagePolicy:    policy.LineagePolicy,
		EvidenceCeiling:  responseBundleEvidenceCeiling(policy.LineagePolicy),
		SourceTreeDigest: sourceProvenance.Digest, BinarySHA256: buildProvenance.BinaryDigest,
		RedistributionEvidenceSHA256: redistributionRecord.Payload.Digest, Captures: verified,
		TotalEntries: index.TotalEntries, TotalBytes: index.TotalBytes, PublicScan: scan,
		ProviderCalls: 0, NetworkRequired: false, ExactReplay: true, Valid: true,
	}, nil
}

func ResponseBundleRegistry() (*capsule.Registry, error) {
	public := []capsule.Visibility{capsule.VisibilityPublic}
	types := []capsule.ComponentType{
		{
			TypeID: capsule.SourceTreeProvenanceSchemaVersion, SchemaID: capsule.SourceTreeProvenanceSchemaVersion,
			Role: capsule.RoleObservation, AllowedVisibilities: public, MediaType: "application/json",
			PayloadProfile: capsule.PayloadCanonicalJSON, ValidatorID: responseBundleSourceValidatorID,
		},
		{
			TypeID: capsule.BuildProvenanceSchemaVersion, SchemaID: capsule.BuildProvenanceSchemaVersion,
			Role: capsule.RoleAttestation, AllowedVisibilities: public, MediaType: "application/json",
			PayloadProfile: capsule.PayloadCanonicalJSON, ValidatorID: responseBundleBuildValidatorID,
			BindingValidatorID: responseBundleBuildBindingID,
			ParentRules: []capsule.ParentRule{{
				Kind: capsule.EdgeAttests, ParentType: capsule.SourceTreeProvenanceSchemaVersion,
				Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal},
			}},
		},
		{
			TypeID: ResponseBundleRedistributionSchemaVersion, SchemaID: ResponseBundleRedistributionSchemaVersion,
			Role: capsule.RoleObservation, AllowedVisibilities: public, MediaType: "application/octet-stream",
			PayloadProfile: capsule.PayloadExactBytes, ValidatorID: responseBundleRedistributionValidatorID,
		},
		{
			TypeID: ResponseBundlePolicySchemaVersion, SchemaID: ResponseBundlePolicySchemaVersion,
			Role: capsule.RoleGovernance, AllowedVisibilities: public, MediaType: "application/json",
			PayloadProfile: capsule.PayloadCanonicalJSON, ValidatorID: responseBundlePolicyValidatorID,
			BindingValidatorID: responseBundlePolicyBindingID,
			ParentRules: []capsule.ParentRule{
				{Kind: capsule.EdgeDerivedFrom, ParentType: capsule.SourceTreeProvenanceSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
				{Kind: capsule.EdgeDerivedFrom, ParentType: capsule.BuildProvenanceSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
				{Kind: capsule.EdgeDerivedFrom, ParentType: ResponseBundleRedistributionSchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
			},
		},
		{
			TypeID: ResponseCaptureComponentSchemaVersion, SchemaID: ResponseCaptureComponentSchemaVersion,
			Role: capsule.RoleObservation, AllowedVisibilities: public, MediaType: "application/x-ndjson",
			PayloadProfile: capsule.PayloadExactBytes, ValidatorID: responseCaptureValidatorID,
			BindingValidatorID: responseCaptureBindingID,
			ParentRules: []capsule.ParentRule{{
				Kind: capsule.EdgeGovernedBy, ParentType: ResponseBundlePolicySchemaVersion,
				Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal},
			}},
		},
		{
			TypeID: ResponseBundleIndexSchemaVersion, SchemaID: ResponseBundleIndexSchemaVersion,
			Role: capsule.RoleDerivation, AllowedVisibilities: public, MediaType: "application/json",
			PayloadProfile: capsule.PayloadCanonicalJSON, ValidatorID: responseBundleIndexValidatorID,
			BindingValidatorID: responseBundleIndexBindingID,
			ParentRules: []capsule.ParentRule{
				{Kind: capsule.EdgeDerivedFrom, ParentType: ResponseCaptureComponentSchemaVersion, Minimum: 1, Maximum: -1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
				{Kind: capsule.EdgeGovernedBy, ParentType: ResponseBundlePolicySchemaVersion, Minimum: 1, Maximum: 1, Resolutions: []capsule.ParentResolution{capsule.ParentInternal}},
			},
		},
	}
	document, err := capsule.SealRegistry(ResponseBundleRegistryID, "", types)
	if err != nil {
		return nil, err
	}
	validators := map[string]capsule.PayloadValidator{
		responseBundleSourceValidatorID: func(payload []byte) error {
			var source capsule.SourceTreeProvenance
			if err := protocol.DecodeStrict(payload, &source); err != nil {
				return err
			}
			return source.Validate()
		},
		responseBundleBuildValidatorID: func(payload []byte) error {
			var build capsule.BuildProvenance
			if err := protocol.DecodeStrict(payload, &build); err != nil {
				return err
			}
			return build.Validate()
		},
		responseBundlePolicyValidatorID: func(payload []byte) error {
			var policy ResponseBundlePolicy
			if err := protocol.DecodeStrict(payload, &policy); err != nil {
				return err
			}
			return policy.Validate()
		},
		responseBundleRedistributionValidatorID: validateRedistributionEvidencePayload,
		responseCaptureValidatorID: func(payload []byte) error {
			_, _, err := inspectCapturePayload(payload)
			return err
		},
		responseBundleIndexValidatorID: func(payload []byte) error {
			var index ResponseBundleIndex
			if err := protocol.DecodeStrict(payload, &index); err != nil {
				return err
			}
			return index.Validate()
		},
	}
	bindings := map[string]capsule.BindingValidator{
		responseBundleBuildBindingID:  validateResponseBundleBuildBindings,
		responseBundlePolicyBindingID: validateResponseBundlePolicyBindings,
		responseCaptureBindingID:      validateResponseCaptureBindings,
		responseBundleIndexBindingID:  validateResponseBundleIndexBindings,
	}
	return capsule.NewRegistryWithBindings(document, validators, bindings)
}

func (index ResponseBundleIndex) Validate() error {
	if index.SchemaVersion != ResponseBundleIndexSchemaVersion || !validBundleDigest(index.PolicyComponentID) ||
		!validBundleDigest(index.PolicyDigest) || len(index.Captures) == 0 || !validBundleDigest(index.CaptureSetDigest) ||
		index.TotalEntries < 1 || index.TotalBytes < 1 || !index.ExactReplay || index.ProviderCalls != 0 ||
		!index.Offline || !validBundleDigest(index.Digest) {
		return errors.New("response bundle index identity is invalid")
	}
	previous := ""
	entries := 0
	var bytesTotal int64
	for _, capture := range index.Captures {
		if !validCaptureName(capture.Name) || capture.Name <= previous || !validBundleDigest(capture.ComponentID) ||
			capture.Validate() != nil {
			return errors.New("response bundle capture descriptors are invalid or unsorted")
		}
		previous = capture.Name
		entries += capture.Entries
		bytesTotal += capture.Bytes
	}
	if entries != index.TotalEntries || bytesTotal != index.TotalBytes {
		return errors.New("response bundle totals differ from capture descriptors")
	}
	setDigest, err := protocol.Digest(index.Captures)
	if err != nil || setDigest != index.CaptureSetDigest {
		return errors.New("response bundle capture-set digest is invalid")
	}
	digest, err := responseBundleIndexDigest(index)
	if err != nil || digest != index.Digest {
		return errors.New("response bundle index digest is invalid")
	}
	return nil
}

func (inspection CaptureInspection) Validate() error {
	if !validBundleDigest(inspection.PayloadSHA256) || inspection.Bytes < 1 || inspection.Entries < 1 ||
		inspection.CompleteResearchEntries < 0 || inspection.CompleteResearchEntries > inspection.Entries ||
		!validSortedText(inspection.StudyCellIDs) ||
		!validBundleText(inspection.ProviderID) || !validRouteID(inspection.RouteID) ||
		!validBundleText(inspection.BaseURLOrigin) || !strings.HasPrefix(inspection.EndpointPath, "/") ||
		inspection.EndpointKind != provider.EndpointOpenAIChatCompletions || !validBundleText(inspection.RequestedModel) ||
		inspection.CaptureSchemaVersion != provider.CaptureSchemaVersion || inspection.RequestSchemaVersion != provider.RequestSchemaVersion ||
		inspection.ParserContractVersion != provider.ParserContractVersion || inspection.FirstReceivedAt < 1 ||
		inspection.LastReceivedAt < inspection.FirstReceivedAt || inspection.LogProbabilityEntries < 0 || inspection.JudgeEntries < 0 ||
		inspection.LogProbabilityEntries+inspection.JudgeEntries != inspection.Entries ||
		!validSortedText(inspection.ServedModels) || !validSortedText(inspection.CheckpointAssertions) ||
		!validSortedText(inspection.CapabilityAttestationIDs) || !validBundleDigest(inspection.RequestSetDigest) ||
		!validBundleDigest(inspection.LineageSetDigest) || !validBundleDigest(inspection.ResponseBodySetDigest) ||
		!validBundleDigest(inspection.EvidenceSetDigest) || !validBundleDigest(inspection.RecordSetDigest) {
		return errors.New("response capture inspection is invalid")
	}
	return nil
}

func inspectCapturePayload(payload []byte) (CaptureInspection, []string, error) {
	if len(payload) == 0 || int64(len(payload)) > responseBundleMaxCaptureBytes || payload[len(payload)-1] != '\n' {
		return CaptureInspection{}, nil, errors.New("exact response capture must be a non-empty, newline-terminated bounded JSONL file")
	}
	inspection := CaptureInspection{
		PayloadSHA256: protocol.DigestBytes(payload), Bytes: int64(len(payload)),
		CaptureSchemaVersion:  provider.CaptureSchemaVersion,
		RequestSchemaVersion:  provider.RequestSchemaVersion,
		ParserContractVersion: provider.ParserContractVersion,
	}
	requests := make([]string, 0)
	lineages := make([]string, 0)
	responseBodies := make([]string, 0)
	evidenceDigests := make([]string, 0)
	recordDigests := make([]string, 0)
	servedModels := make(map[string]struct{})
	checkpointAssertions := make(map[string]struct{})
	capabilityAttestations := make(map[string]struct{})
	studyCellIDs := make(map[string]struct{})
	seenKeys := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := slices.Clone(scanner.Bytes())
		if len(line) == 0 {
			return CaptureInspection{}, nil, errors.New("exact response capture contains a blank record")
		}
		var entry fixtureEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return CaptureInspection{}, nil, fmt.Errorf("parse exact response capture: %w", err)
		}
		if inspection.Entries == 0 {
			inspection.ProviderID = entry.Request.ProviderID
			inspection.RouteID = entry.Request.RouteID()
			inspection.BaseURLOrigin = entry.Request.BaseURLOrigin
			inspection.EndpointPath = entry.Request.EndpointPath
			inspection.EndpointKind = entry.Request.EndpointKind
			inspection.RequestedModel = entry.Request.RequestedModel
			inspection.FirstReceivedAt = entry.Response.ReceivedAt
			inspection.LastReceivedAt = entry.Response.ReceivedAt
		}
		key, err := validateFixtureEntry(entry, inspection.ProviderID, inspection.RequestedModel)
		if err != nil {
			return CaptureInspection{}, nil, err
		}
		canonical, err := json.Marshal(entry)
		if err != nil || !bytes.Equal(line, canonical) {
			return CaptureInspection{}, nil, errors.New("exact response capture record is not in deterministic JSON form")
		}
		if entry.Request.RouteID() != inspection.RouteID || entry.Request.BaseURLOrigin != inspection.BaseURLOrigin ||
			entry.Request.EndpointPath != inspection.EndpointPath || entry.Request.EndpointKind != inspection.EndpointKind {
			return CaptureInspection{}, nil, errors.New("exact response capture combines more than one route")
		}
		if _, duplicate := seenKeys[key]; duplicate {
			return CaptureInspection{}, nil, fmt.Errorf("exact response capture repeats request and sampling slot %s", key[:16])
		}
		seenKeys[key] = struct{}{}
		inspection.Entries++
		if entry.Request.Logprobs {
			inspection.LogProbabilityEntries++
		} else {
			inspection.JudgeEntries++
		}
		inspection.FirstReceivedAt = min(inspection.FirstReceivedAt, entry.Response.ReceivedAt)
		inspection.LastReceivedAt = max(inspection.LastReceivedAt, entry.Response.ReceivedAt)
		if entry.Response.ServedModel != "" {
			servedModels[entry.Response.ServedModel] = struct{}{}
		}
		if entry.Response.CheckpointAssertion != "" {
			checkpointAssertions[entry.Response.CheckpointAssertion] = struct{}{}
		}
		if entry.Response.CapabilityAttestationID != "" {
			capabilityAttestations[entry.Response.CapabilityAttestationID] = struct{}{}
		}
		if responseEntryHasCompleteResearchEvidence(entry) {
			inspection.CompleteResearchEntries++
		}
		if entry.Request.Lineage.StudyCellID != "" {
			studyCellIDs[entry.Request.Lineage.StudyCellID] = struct{}{}
		}
		fingerprint, err := entry.Request.Fingerprint()
		if err != nil {
			return CaptureInspection{}, nil, err
		}
		lineageDigest, err := protocol.Digest(entry.Request.Lineage)
		if err != nil {
			return CaptureInspection{}, nil, err
		}
		requests = append(requests, string(fingerprint)+"\x00"+entry.Request.Lineage.SamplingSlot)
		lineages = append(lineages, lineageDigest)
		responseBodies = append(responseBodies, entry.Response.ResponseBodyDigest)
		evidenceDigests = append(evidenceDigests, entry.Response.EvidenceDigest)
		recordDigests = append(recordDigests, entry.RecordDigest)
	}
	if err := scanner.Err(); err != nil {
		return CaptureInspection{}, nil, err
	}
	if inspection.Entries == 0 {
		return CaptureInspection{}, nil, errors.New("exact response capture contains no records")
	}
	inspection.ServedModels = sortedSet(servedModels)
	inspection.CheckpointAssertions = sortedSet(checkpointAssertions)
	inspection.CapabilityAttestationIDs = sortedSet(capabilityAttestations)
	inspection.StudyCellIDs = sortedSet(studyCellIDs)
	var err error
	inspection.RequestSetDigest, err = digestStringMultiset(requests)
	if err != nil {
		return CaptureInspection{}, nil, err
	}
	inspection.LineageSetDigest, err = digestStringMultiset(lineages)
	if err != nil {
		return CaptureInspection{}, nil, err
	}
	inspection.ResponseBodySetDigest, err = digestStringMultiset(responseBodies)
	if err != nil {
		return CaptureInspection{}, nil, err
	}
	inspection.EvidenceSetDigest, err = digestStringMultiset(evidenceDigests)
	if err != nil {
		return CaptureInspection{}, nil, err
	}
	inspection.RecordSetDigest, err = digestStringMultiset(recordDigests)
	if err != nil {
		return CaptureInspection{}, nil, err
	}
	return inspection, slices.Clone(requests), inspection.Validate()
}

func validateResponseBundleBuildBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 1 || context.Parents[0].Reference.Kind != capsule.EdgeAttests ||
		context.Parents[0].Reference.Resolution != capsule.ParentInternal ||
		context.Parents[0].Record.TypeID != capsule.SourceTreeProvenanceSchemaVersion {
		return errors.New("response bundle producer build requires one internal source-tree parent")
	}
	var source capsule.SourceTreeProvenance
	if err := protocol.DecodeStrict(context.Parents[0].Payload, &source); err != nil {
		return err
	}
	var build capsule.BuildProvenance
	if err := protocol.DecodeStrict(context.Payload, &build); err != nil {
		return err
	}
	return (ResponseBundleProducerEvidence{SourceTree: source, Build: build}).Validate()
}

func validateResponseBundlePolicyBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 3 {
		return errors.New("response bundle policy requires source-tree, build, and redistribution-evidence parents")
	}
	var evidence ResponseBundleProducerEvidence
	var redistributionEvidence []byte
	for _, parent := range context.Parents {
		if parent.Reference.Kind != capsule.EdgeDerivedFrom || parent.Reference.Resolution != capsule.ParentInternal {
			return errors.New("response bundle policy requires internal provenance parents")
		}
		switch parent.Record.TypeID {
		case capsule.SourceTreeProvenanceSchemaVersion:
			if evidence.SourceTree.SchemaVersion != "" {
				return errors.New("response bundle policy repeats its source-tree parent")
			}
			if err := protocol.DecodeStrict(parent.Payload, &evidence.SourceTree); err != nil {
				return err
			}
		case capsule.BuildProvenanceSchemaVersion:
			if evidence.Build.SchemaVersion != "" {
				return errors.New("response bundle policy repeats its build parent")
			}
			if err := protocol.DecodeStrict(parent.Payload, &evidence.Build); err != nil {
				return err
			}
		case ResponseBundleRedistributionSchemaVersion:
			if redistributionEvidence != nil {
				return errors.New("response bundle policy repeats its redistribution-evidence parent")
			}
			redistributionEvidence = parent.Payload
		default:
			return errors.New("response bundle policy has an unsupported provenance parent")
		}
	}
	if err := evidence.Validate(); err != nil {
		return err
	}
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(context.Payload, &policy); err != nil {
		return err
	}
	if policy.Producer != evidence.Producer() {
		return errors.New("response bundle policy producer differs from its provenance parents")
	}
	if err := validateResponseBundleProducerPolicy(policy, evidence); err != nil {
		return err
	}
	if protocol.DigestBytes(redistributionEvidence) != policy.RedistributionEvidenceDigest {
		return errors.New("response bundle policy differs from its redistribution-evidence parent")
	}
	return nil
}

func validateResponseCaptureBindings(context capsule.BindingContext) error {
	if len(context.Parents) != 1 || context.Parents[0].Reference.Kind != capsule.EdgeGovernedBy ||
		context.Parents[0].Reference.Resolution != capsule.ParentInternal ||
		context.Parents[0].Record.TypeID != ResponseBundlePolicySchemaVersion {
		return errors.New("response capture requires one internal policy parent")
	}
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(context.Parents[0].Payload, &policy); err != nil {
		return err
	}
	if err := policy.Validate(); err != nil {
		return err
	}
	name := strings.TrimPrefix(context.Component.Name, "response.capture.")
	if "response.capture."+name != context.Component.Name || !slices.Contains(policy.AllowedCaptureNames, name) {
		return errors.New("response capture is not authorized by its policy")
	}
	inspection, _, err := inspectCapturePayload(context.Payload)
	if err != nil {
		return err
	}
	return validateResponseCaptureLineagePolicy(policy, inspection)
}

func validateResponseBundleIndexBindings(context capsule.BindingContext) error {
	var index ResponseBundleIndex
	if err := protocol.DecodeStrict(context.Payload, &index); err != nil {
		return err
	}
	if err := index.Validate(); err != nil {
		return err
	}
	var policyParent *capsule.BoundParent
	captureParents := make(map[string]capsule.BoundParent)
	for parentIndex := range context.Parents {
		parent := context.Parents[parentIndex]
		if parent.Reference.Resolution != capsule.ParentInternal {
			return errors.New("response bundle index requires internal parents")
		}
		switch parent.Record.TypeID {
		case ResponseBundlePolicySchemaVersion:
			if parent.Reference.Kind != capsule.EdgeGovernedBy || policyParent != nil {
				return errors.New("response bundle index repeats its policy parent")
			}
			policyParent = &context.Parents[parentIndex]
		case ResponseCaptureComponentSchemaVersion:
			if parent.Reference.Kind != capsule.EdgeDerivedFrom {
				return errors.New("response bundle index capture parent has the wrong edge kind")
			}
			if _, duplicate := captureParents[parent.Record.ComponentID]; duplicate {
				return errors.New("response bundle index repeats a capture parent")
			}
			captureParents[parent.Record.ComponentID] = parent
		default:
			return errors.New("response bundle index has an unsupported parent type")
		}
	}
	if policyParent == nil || policyParent.Record.ComponentID != index.PolicyComponentID {
		return errors.New("response bundle index policy component differs from its parent")
	}
	var policy ResponseBundlePolicy
	if err := protocol.DecodeStrict(policyParent.Payload, &policy); err != nil {
		return err
	}
	if err := policy.Validate(); err != nil {
		return err
	}
	if policy.Digest != index.PolicyDigest || len(index.Captures) != len(captureParents) ||
		len(index.Captures) != len(policy.AllowedCaptureNames) {
		return errors.New("response bundle index differs from its policy or capture parent set")
	}
	inspected := make([]inspectedResponseCapture, 0, len(index.Captures))
	captureNames := make([]string, 0, len(index.Captures))
	for _, descriptor := range index.Captures {
		parent, found := captureParents[descriptor.ComponentID]
		if !found || parent.Record.Name != "response.capture."+descriptor.Name ||
			parent.Record.Payload.Digest != descriptor.PayloadSHA256 || parent.Record.Payload.Bytes != descriptor.Bytes {
			return fmt.Errorf("response bundle capture %q differs from its component parent", descriptor.Name)
		}
		inspection, requestKeys, err := inspectCapturePayload(parent.Payload)
		if err != nil || !reflect.DeepEqual(inspection, descriptor.CaptureInspection) {
			return fmt.Errorf("response bundle capture %q differs from exact payload inspection", descriptor.Name)
		}
		if len(parent.Record.Parents) != 1 || parent.Record.Parents[0].Kind != capsule.EdgeGovernedBy ||
			parent.Record.Parents[0].Resolution != capsule.ParentInternal ||
			parent.Record.Parents[0].TypeID != ResponseBundlePolicySchemaVersion ||
			parent.Record.Parents[0].ComponentID != policyParent.Record.ComponentID {
			return fmt.Errorf("response bundle capture %q is not governed by its index policy", descriptor.Name)
		}
		captureNames = append(captureNames, descriptor.Name)
		inspected = append(inspected, inspectedResponseCapture{
			Name: descriptor.Name, Raw: parent.Payload, RequestKeys: requestKeys, Inspection: inspection,
		})
	}
	if !slices.Equal(captureNames, policy.AllowedCaptureNames) {
		return errors.New("response bundle index capture names differ from its policy allowlist")
	}
	if err := validateResponseBundleLineagePolicy(policy, inspected); err != nil {
		return err
	}
	requestCorpusDigest, err := responseBundleRequestCorpusDigest(inspected)
	if err != nil {
		return err
	}
	if requestCorpusDigest != policy.Dataset.Digest {
		return errors.New("response bundle index request corpus differs from its sealed policy")
	}
	return nil
}

func inspectResponseBundleSources(names []string, sources map[string]string) ([]inspectedResponseCapture, error) {
	inspected := make([]inspectedResponseCapture, 0, len(names))
	for _, name := range names {
		raw, err := readCaptureSource(sources[name])
		if err != nil {
			return nil, fmt.Errorf("response capture %q: %w", name, err)
		}
		inspection, requestKeys, err := inspectCapturePayload(raw)
		if err != nil {
			return nil, fmt.Errorf("response capture %q: %w", name, err)
		}
		inspected = append(inspected, inspectedResponseCapture{
			Name: name, Raw: raw, RequestKeys: requestKeys, Inspection: inspection,
		})
	}
	return inspected, nil
}

func validateResponseBundleLineagePolicy(policy ResponseBundlePolicy, captures []inspectedResponseCapture) error {
	for _, capture := range captures {
		if err := validateResponseCaptureLineagePolicy(policy, capture.Inspection); err != nil {
			return fmt.Errorf("response capture %q: %w", capture.Name, err)
		}
	}
	return nil
}

func validateResponseBundleProducerPolicy(policy ResponseBundlePolicy, evidence ResponseBundleProducerEvidence) error {
	if policy.LineagePolicy != ResponseBundleLineageCompleteResearch {
		return nil
	}
	if evidence.SourceTree.Dirty || evidence.Build.Dirty ||
		evidence.Build.SourceMatchStatus != "matched" ||
		evidence.Build.EmbeddedVCSRevision != evidence.SourceTree.Commit ||
		evidence.Build.EmbeddedVCSModified != "false" {
		return errors.New("complete research bundles require a clean committed source tree and its matching binary")
	}
	return nil
}

func validateResponseCaptureLineagePolicy(policy ResponseBundlePolicy, inspection CaptureInspection) error {
	switch policy.LineagePolicy {
	case ResponseBundleLineageExactFixture:
		return nil
	case ResponseBundleLineageCompleteResearch:
		if inspection.CompleteResearchEntries != inspection.Entries {
			return fmt.Errorf(
				"complete research evidence is required for every entry: got %d of %d",
				inspection.CompleteResearchEntries, inspection.Entries,
			)
		}
		if len(inspection.StudyCellIDs) != 1 || inspection.StudyCellIDs[0] != policy.CellID {
			return fmt.Errorf("complete research lineage must bind every entry to study cell %q", policy.CellID)
		}
		return nil
	default:
		return errors.New("response bundle lineage policy is invalid")
	}
}

func responseBundleEvidenceCeiling(lineagePolicy string) string {
	if lineagePolicy == ResponseBundleLineageCompleteResearch {
		return ResponseBundleEvidenceExternalUnresolved
	}
	return ResponseBundleEvidenceMechanismConformance
}

func responseEntryHasCompleteResearchEvidence(entry fixtureEntry) bool {
	lineage := entry.Request.Lineage
	if lineage.ValidateResearch() != nil ||
		!validBundleDigest(lineage.SourceTraceHash) ||
		!validBundleDigest(lineage.TraceMapHash) ||
		!validBundleDigest(lineage.PolicyHash) ||
		len(entry.Request.EvidenceBindings) == 0 ||
		!validBundleText(entry.Response.ServedModel) ||
		!strings.HasPrefix(entry.Response.CapabilityAttestationID, "att-") ||
		!validBundleDigest(strings.TrimPrefix(entry.Response.CapabilityAttestationID, "att-")) {
		return false
	}
	return true
}

func responseBundleRequestCorpusDigest(captures []inspectedResponseCapture) (string, error) {
	partitions := make([]responseBundleRequestPartition, len(captures))
	requestOwners := make(map[string]string)
	for index, capture := range captures {
		if len(capture.RequestKeys) != capture.Inspection.Entries {
			return "", fmt.Errorf("response capture %q request-key census is incomplete", capture.Name)
		}
		for _, key := range capture.RequestKeys {
			if owner, duplicate := requestOwners[key]; duplicate {
				identity := protocol.DigestBytes([]byte(key))
				return "", fmt.Errorf(
					"response captures %q and %q repeat request and sampling slot %s",
					owner, capture.Name, identity[:16],
				)
			}
			requestOwners[key] = capture.Name
		}
		partitions[index] = responseBundleRequestPartition{
			Name: capture.Name, Entries: capture.Inspection.Entries,
			RequestSetDigest: capture.Inspection.RequestSetDigest,
		}
	}
	return protocol.Digest(partitions)
}

func buildResponseBundleIndex(policy ResponseBundlePolicy, policyRecord capsule.ComponentRecord, captures []ResponseBundleCapture) (ResponseBundleIndex, error) {
	index := ResponseBundleIndex{
		SchemaVersion:     ResponseBundleIndexSchemaVersion,
		PolicyComponentID: policyRecord.ComponentID, PolicyDigest: policy.Digest,
		Captures: slices.Clone(captures), ExactReplay: true, ProviderCalls: 0, Offline: true,
	}
	slices.SortFunc(index.Captures, func(left, right ResponseBundleCapture) int { return strings.Compare(left.Name, right.Name) })
	for _, capture := range index.Captures {
		index.TotalEntries += capture.Entries
		index.TotalBytes += capture.Bytes
	}
	var err error
	index.CaptureSetDigest, err = protocol.Digest(index.Captures)
	if err != nil {
		return ResponseBundleIndex{}, err
	}
	index.Digest, err = responseBundleIndexDigest(index)
	if err != nil {
		return ResponseBundleIndex{}, err
	}
	return index, index.Validate()
}

func responseBundlePolicyDigest(policy ResponseBundlePolicy) (string, error) {
	policy.Digest = ""
	return protocol.Digest(policy)
}

func responseBundleIndexDigest(index ResponseBundleIndex) (string, error) {
	index.Digest = ""
	return protocol.Digest(index)
}

func digestStringMultiset(values []string) (string, error) {
	copy := slices.Clone(values)
	slices.Sort(copy)
	return protocol.Digest(copy)
}

func readCaptureSource(path string) ([]byte, error) {
	return readResponseBundleArtifact(path, responseBundleMaxCaptureBytes, "capture source")
}

func readRedistributionEvidence(path string) ([]byte, error) {
	raw, err := readResponseBundleArtifact(path, responseBundleMaxRedistributionEvidenceBytes, "redistribution evidence")
	if err != nil {
		return nil, err
	}
	if err := validateRedistributionEvidencePayload(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func scanExactResponseBundlePackage(pack capsule.Package, knownSecrets []string, reviewedFindings []safety.ArtifactFinding) (safety.ArtifactScanReport, error) {
	temporaryRoot, err := os.MkdirTemp("", "evalwitness-response-bundle-scan.")
	if err != nil {
		return safety.ArtifactScanReport{}, err
	}
	cleanup := func(scanErr error) (safety.ArtifactScanReport, error) {
		cleanupErr := os.RemoveAll(temporaryRoot)
		return safety.ArtifactScanReport{}, errors.Join(scanErr, cleanupErr)
	}
	bundlePath := filepath.Join(temporaryRoot, "bundle")
	if err := capsule.WriteDirectory(
		context.Background(), bundlePath, pack.Registry, pack.Manifest, pack.Payloads,
	); err != nil {
		return cleanup(err)
	}
	report, scanErr := safety.ScanArtifacts(safety.ArtifactScanRequest{
		Roots: []string{bundlePath}, Class: safety.ArtifactPublic,
		KnownSecrets: slices.Clone(knownSecrets), Limits: responseBundleArtifactScanLimits(),
		ReviewedFindings: reviewedFindings,
	})
	cleanupErr := os.RemoveAll(temporaryRoot)
	if scanErr != nil || cleanupErr != nil {
		var wrappedScanErr error
		if scanErr != nil {
			wrappedScanErr = fmt.Errorf("response bundle exact-byte public scan: %w", scanErr)
		}
		return safety.ArtifactScanReport{}, errors.Join(wrappedScanErr, cleanupErr)
	}
	return report, nil
}

func responseBundleArtifactScanLimits() safety.ArtifactScanLimits {
	limits := safety.DefaultArtifactScanLimits()
	limits.MaxFileBytes = responseBundleMaxCaptureBytes
	return limits
}

func validateRedistributionEvidencePayload(payload []byte) error {
	if len(payload) == 0 || int64(len(payload)) > responseBundleMaxRedistributionEvidenceBytes {
		return errors.New("redistribution evidence must be non-empty and bounded")
	}
	return nil
}

func readResponseBundleArtifact(path string, maximumBytes int64, kind string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() < 1 || info.Size() > maximumBytes {
		return nil, errors.Join(err, fmt.Errorf("%s is not a bounded regular file", kind))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || info.Size() != opened.Size() ||
		!info.ModTime().Equal(opened.ModTime()) {
		_ = file.Close()
		return nil, errors.Join(err, fmt.Errorf("%s changed before open", kind))
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	final, statErr := file.Stat()
	closeErr := file.Close()
	pathInfo, pathErr := os.Lstat(path)
	if readErr != nil || statErr != nil || closeErr != nil || pathErr != nil || int64(len(raw)) != info.Size() ||
		!pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(opened, final) || !os.SameFile(final, pathInfo) ||
		opened.Size() != final.Size() || final.Size() != pathInfo.Size() ||
		!opened.ModTime().Equal(final.ModTime()) || !final.ModTime().Equal(pathInfo.ModTime()) {
		return nil, errors.Join(readErr, statErr, closeErr, pathErr, fmt.Errorf("%s changed during read", kind))
	}
	return raw, nil
}

func addResponsePayload(payloads map[string][]byte, digest string, payload []byte) error {
	if existing, found := payloads[digest]; found && !bytes.Equal(existing, payload) {
		return errors.New("response bundle payload digest collision")
	}
	payloads[digest] = slices.Clone(payload)
	return nil
}

func internalResponseParent(kind capsule.EdgeKind, record capsule.ComponentRecord) capsule.ParentRef {
	return capsule.ParentRef{
		Kind: kind, ComponentID: record.ComponentID, TypeID: record.TypeID,
		Role: record.Role, Visibility: record.Visibility, Resolution: capsule.ParentInternal,
	}
}

func responseComponentByType(records []capsule.ComponentRecord, typeID string) (capsule.ComponentRecord, error) {
	var match capsule.ComponentRecord
	count := 0
	for _, record := range records {
		if record.TypeID == typeID {
			match = record
			count++
		}
	}
	if count != 1 {
		return capsule.ComponentRecord{}, fmt.Errorf("response bundle requires exactly one component of type %q, got %d", typeID, count)
	}
	return match, nil
}

func sameCaptureNames(names []string, sources map[string]string) bool {
	if len(names) != len(sources) {
		return false
	}
	for _, name := range names {
		if strings.TrimSpace(sources[name]) == "" {
			return false
		}
	}
	return true
}

func validCaptureNames(values []string) bool {
	if len(values) == 0 || !slices.IsSorted(values) || len(slices.Compact(slices.Clone(values))) != len(values) {
		return false
	}
	return slices.IndexFunc(values, func(value string) bool { return !validCaptureName(value) }) < 0
}

func validCaptureName(value string) bool {
	if value == "" || len(value) > 64 || value != strings.TrimSpace(value) {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			index > 0 && (character == '.' || character == '_' || character == '-') {
			continue
		}
		return false
	}
	return true
}

func validBundleIdentifier(value string) bool {
	if value == "" || len(value) > 256 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune("._:/-", character) {
			continue
		}
		return false
	}
	return true
}

func validBundleText(value string) bool {
	if value == "" || len(value) > 512 || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	return strings.IndexFunc(value, func(character rune) bool { return character < 0x20 || character == 0x7f }) < 0
}

func validBundleDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func validRouteID(value string) bool {
	return strings.HasPrefix(value, "route-") && validBundleDigest(strings.TrimPrefix(value, "route-"))
}

func validSortedText(values []string) bool {
	return slices.IsSorted(values) && len(slices.Compact(slices.Clone(values))) == len(values) &&
		slices.IndexFunc(values, func(value string) bool { return !validBundleText(value) }) < 0
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}
