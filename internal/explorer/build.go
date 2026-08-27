package explorer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/capsule"
	claimledger "github.com/Christopher-Schulze/evalwitness/internal/claim"
	"github.com/Christopher-Schulze/evalwitness/internal/lineage"
	"github.com/Christopher-Schulze/evalwitness/internal/relation"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	acceptedBOMPath     = "eval/results/verification-lineage-offline-bom-example-v1.json"
	datasetCardPath     = "eval/results/verification-lineage-development-dataset-card-v1.json"
	limitationsPath     = "eval/results/verification-lineage-limitations-v1.json"
	lineageGraphPath    = "eval/results/verification-lineage-offline-graph-v1.json"
	lossCertificatePath = "eval/results/verification-lineage-same-path-loss-certificate-v1.json"
	lineageGraphSVGPath = "eval/results/verification-lineage-offline-graph-v1.svg"

	maximumReleaseFiles     = 1_024
	maximumReleaseFileBytes = int64(64 << 20)
	maximumReleaseTotal     = int64(256 << 20)
)

type verifiedReleaseFile struct {
	manifest lineage.ReleaseFile
	raw      []byte
}

type releaseFileLoader func(repositoryRoot string, file lineage.ReleaseFile) ([]byte, error)

type reportEvidence struct {
	bom                   lineage.VerificationEvidenceBOM
	bomRecord             capsule.ComponentRecord
	dataset               lineage.VerificationLineageDatasetCard
	datasetRecord         capsule.ComponentRecord
	limitations           lineage.VerificationLineageLimitationsLedger
	graph                 lineage.VerificationLineageGraph
	loss                  lineage.VerificationLineageLossCertificate
	release               lineage.VerificationLineageRelease
	releaseRecord         capsule.ComponentRecord
	ownerInspection       relation.OwnerInspectionPublicAttestation
	ownerInspectionRecord capsule.ComponentRecord
	files                 map[string]verifiedReleaseFile
	ledgerSource          ArtifactRef
	autopsySource         ArtifactRef
}

type reportSections struct {
	method        MethodView
	transport     TransportView
	limitations   []LimitationView
	stress        StressView
	bomSource     ArtifactRef
	lossSource    ArtifactRef
	releaseSource ArtifactRef
	counts        reportScopeCounts
}

type reportScopeCounts struct {
	developmentFixtures     int
	empiricalTaskGroups     int
	researchAdmittedSources int
}

type verifiedReleaseEvidence struct {
	files       map[string]verifiedReleaseFile
	limitations lineage.VerificationLineageLimitationsLedger
	graph       lineage.VerificationLineageGraph
	loss        lineage.VerificationLineageLossCertificate
}

// BuildReport verifies every input and derives a deterministic presentation
// model without provider calls or hand-maintained scientific values.
func BuildReport(
	ctx context.Context,
	repositoryRoot string,
	pack capsule.Package,
	ledger claimledger.Ledger,
	autopsy claimledger.Autopsy,
) (Report, error) {
	return buildReport(ctx, repositoryRoot, pack, ledger, autopsy, loadReleaseFile, loadStressDevelopmentCaseStudy)
}

func buildReport(
	ctx context.Context,
	repositoryRoot string,
	pack capsule.Package,
	ledger claimledger.Ledger,
	autopsy claimledger.Autopsy,
	loader releaseFileLoader,
	stressLoader stressCaseLoader,
) (Report, error) {
	if ctx == nil || repositoryRoot == "" || pack.Registry == nil || loader == nil || stressLoader == nil {
		return Report{}, errors.New("evidence explorer build requires context, repository root, capsule registry, release loader, and stress loader")
	}
	verification, err := verifyReportSources(ctx, pack, ledger, autopsy)
	if err != nil {
		return Report{}, err
	}
	challenge, err := buildChallengeView(ledger, verification.Challenges)
	if err != nil {
		return Report{}, err
	}
	evidence, err := loadReportEvidence(repositoryRoot, pack, ledger, autopsy, loader)
	if err != nil {
		return Report{}, err
	}
	if err := verifyReportBindings(pack.Manifest, autopsy, evidence); err != nil {
		return Report{}, err
	}
	sections, err := deriveReportSections(pack.Manifest, autopsy, evidence)
	if err != nil {
		return Report{}, err
	}
	caseStudy, caseStudyRaw, err := stressLoader(repositoryRoot)
	if err != nil {
		return Report{}, err
	}
	sections.stress, err = buildStressView(caseStudy, caseStudyRaw)
	if err != nil {
		return Report{}, err
	}
	return sealReport(assembleReport(pack.Manifest, ledger, autopsy, evidence, sections, challenge))
}

func verifyReportSources(ctx context.Context, pack capsule.Package, ledger claimledger.Ledger, autopsy claimledger.Autopsy) (claimledger.EvidenceVerification, error) {
	verification, err := claimledger.VerifyEvidence(ctx, pack.Registry, pack.Manifest, pack.Payloads, ledger, autopsy)
	if err != nil {
		return claimledger.EvidenceVerification{}, fmt.Errorf("verify evidence explorer sources: %w", err)
	}
	return verification, nil
}

func deriveReportSections(manifest capsule.Manifest, autopsy claimledger.Autopsy, evidence reportEvidence) (reportSections, error) {
	method, err := buildMethodView(manifest, autopsy.MethodIntegrity)
	if err != nil {
		return reportSections{}, err
	}
	bomSource := componentArtifactRef(evidence.bomRecord, evidence.bom.Header.Digest)
	lossSource := releaseArtifactRef(evidence.files[lossCertificatePath].manifest, evidence.loss.Version, evidence.loss.Digest)
	transport, err := buildTransportView(autopsy.ClaimTransport, evidence.graph, evidence.loss, bomSource, lossSource)
	if err != nil {
		return reportSections{}, err
	}
	limitationSource := releaseArtifactRef(evidence.files[limitationsPath].manifest, evidence.limitations.Version, evidence.limitations.Digest)
	limitations, err := buildLimitationViews(evidence.limitations, limitationSource, evidence.autopsySource)
	if err != nil {
		return reportSections{}, err
	}
	counts, err := deriveScopeCounts(evidence.dataset, evidence.graph.DevelopmentFixtures)
	return reportSections{
		method: method, transport: transport, limitations: limitations, bomSource: bomSource,
		lossSource: lossSource, releaseSource: componentArtifactRef(evidence.releaseRecord, evidence.release.Header.Digest),
		counts: counts,
	}, err
}

func deriveScopeCounts(dataset lineage.VerificationLineageDatasetCard, graphFixtures int) (reportScopeCounts, error) {
	development, err := datasetCount(dataset, "development_fixtures")
	if err != nil {
		return reportScopeCounts{}, err
	}
	empirical, err := datasetCount(dataset, "empirical_task_groups")
	if err != nil {
		return reportScopeCounts{}, err
	}
	research, err := datasetCount(dataset, "research_admitted_sources")
	if err != nil {
		return reportScopeCounts{}, err
	}
	if development != graphFixtures {
		return reportScopeCounts{}, errors.New("evidence explorer dataset and lineage graph fixture counts differ")
	}
	return reportScopeCounts{developmentFixtures: development, empiricalTaskGroups: empirical, researchAdmittedSources: research}, nil
}

func assembleReport(
	manifest capsule.Manifest,
	ledger claimledger.Ledger,
	autopsy claimledger.Autopsy,
	evidence reportEvidence,
	sections reportSections,
	challenge ChallengeView,
) Report {
	return Report{
		SchemaVersion:   ReportSchemaVersion,
		CanonicalPolicy: CanonicalPolicy,
		Capsule: CapsuleBinding{
			CapsuleID: manifest.CapsuleID, ManifestDigest: manifest.ManifestDigest,
			LedgerDigest: ledger.Digest, AutopsyDigest: autopsy.Digest,
			LedgerSource: evidence.ledgerSource, AutopsySource: evidence.autopsySource,
		},
		Scope: buildScopeView(evidence, sections.counts), Claim: buildClaimView(autopsy.ClaimTransport, sections.bomSource),
		BOM: buildBOMView(evidence.bom, sections.bomSource), Method: sections.method, Transport: sections.transport,
		OwnerInspection: buildOwnerInspectionView(
			evidence.ownerInspection,
			componentArtifactRef(evidence.ownerInspectionRecord, evidence.ownerInspection.Digest),
		),
		Loss:    buildLossView(evidence.loss, sections.transport.SelectedPath, sections.lossSource),
		Stress:  sections.stress,
		Dataset: buildDatasetView(evidence), Limitations: sections.limitations,
		Release: buildReleaseView(evidence, sections.releaseSource), Challenge: challenge,
		Extensions: buildExtensionViews(manifest),
		Reproduction: ReproductionView{
			Command: evidence.release.ReproductionCommand, NetworkRequired: false,
			ProviderCalls: evidence.release.ProviderCallsRequired, Source: sections.releaseSource,
		},
	}
}

func sealReport(report Report) (Report, error) {
	var err error
	report.Digest, err = reportDigest(report)
	if err != nil {
		return Report{}, err
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

func buildScopeView(evidence reportEvidence, counts reportScopeCounts) ScopeView {
	return ScopeView{
		Evaluator: evidence.bom.Evidence.Executable, Route: nil, RouteAvailability: AvailabilityNotApplicable,
		EvidenceRole: string(evidence.dataset.Header.DataRole), ReleaseVersion: evidence.release.Version,
		DevelopmentFixtures: counts.developmentFixtures, EmpiricalTaskGroups: counts.empiricalTaskGroups,
		ResearchAdmittedSources: counts.researchAdmittedSources, ProviderCalls: evidence.release.ProviderCallsRequired,
		Empirical: evidence.graph.Empirical, ProviderRanking: evidence.graph.ProviderRanking,
		RestrictedMaterialExcluded: evidence.release.RestrictedMaterialExcluded,
	}
}

func buildClaimView(lane claimledger.ClaimTransportLane, source ArtifactRef) ClaimView {
	return ClaimView{
		ClaimID: lane.ClaimID, FailableProperty: lane.FailableProperty,
		ObservableFailureCondition: lane.ObservableFailureCondition, Accepted: lane.Accepted,
		ClaimBoundary: slices.Clone(lane.ClaimBoundary), KnownLimitations: slices.Clone(lane.KnownLimitations),
		OutOfScopeUses: slices.Clone(lane.OutOfScopeUses), Source: source,
	}
}

func buildBOMView(bom lineage.VerificationEvidenceBOM, source ArtifactRef) BOMView {
	return BOMView{
		BOMID: bom.BOMID, CandidateID: bom.CandidateID, Executable: bom.Evidence.Executable,
		Operands: slices.Clone(bom.Evidence.Operands), RequiredFields: slices.Clone(bom.Evidence.RequiredFields),
		DecisiveChannels:        slices.Clone(bom.Evidence.DecisiveChannels),
		SurvivingChannels:       slices.Clone(bom.Retention.SurvivingChannels),
		TruncatedRequiredFields: slices.Clone(bom.Retention.TruncatedRequiredFields),
		RepositoryStateDigest:   bom.RepositoryStateDigest, Freshness: freshnessView(bom.ValidityInterval), Source: source,
	}
}

func buildLossView(loss lineage.VerificationLineageLossCertificate, pathID string, source ArtifactRef) LossCertificateView {
	return LossCertificateView{
		CertificateID: loss.CertificateID, PathID: pathID, FirstLossState: string(loss.FirstLossState),
		Disposition: string(loss.Disposition), Precedence: loss.Precedence,
		FailabilityResolution: string(loss.FailabilityResolution), FailabilityReason: string(loss.FailabilityReason),
		PublicSafe: loss.PublicSafe, RestrictedContent: loss.RestrictedContent, SupportedClaim: loss.SupportedClaim,
		UnsupportedClaims: slices.Clone(loss.UnsupportedClaims), Source: source,
	}
}

func buildDatasetView(evidence reportEvidence) DatasetView {
	return DatasetView{
		DatasetID: evidence.dataset.DatasetID, Title: evidence.dataset.Title,
		EvidenceRole: string(evidence.dataset.Header.DataRole), Counts: datasetCounts(evidence.dataset),
		KnownLimitations: slices.Clone(evidence.dataset.KnownLimitations),
		OutOfScopeUses:   slices.Clone(evidence.dataset.OutOfScopeUses),
		Source:           componentArtifactRef(evidence.datasetRecord, evidence.dataset.Header.Digest),
	}
}

func buildReleaseView(evidence reportEvidence, source ArtifactRef) ReleaseView {
	return ReleaseView{
		ReleaseID: evidence.release.ReleaseID, Version: evidence.release.Version,
		Files: len(evidence.release.Files), FilesVerified: len(evidence.files),
		AllFilesVerified: evidence.release.AllFilesVerified, PublicProjection: evidence.release.PublicProjection,
		RestrictedMaterialExcluded: evidence.release.RestrictedMaterialExcluded,
		Manifest:                   releaseManifest(evidence.release.Files), Source: source,
	}
}

func loadReportEvidence(
	repositoryRoot string,
	pack capsule.Package,
	ledger claimledger.Ledger,
	autopsy claimledger.Autopsy,
	loader releaseFileLoader,
) (reportEvidence, error) {
	evidence, err := loadCapsuleReportEvidence(pack)
	if err != nil {
		return reportEvidence{}, err
	}
	releaseEvidence, err := loadVerifiedReleaseEvidence(repositoryRoot, evidence.release, loader)
	if err != nil {
		return reportEvidence{}, err
	}
	if err := verifyReleaseComponentCopies(evidence, releaseEvidence.files); err != nil {
		return reportEvidence{}, err
	}
	ledgerSource, autopsySource, err := buildReportSidecars(ledger, autopsy)
	if err != nil {
		return reportEvidence{}, err
	}
	evidence.files = releaseEvidence.files
	evidence.limitations = releaseEvidence.limitations
	evidence.graph = releaseEvidence.graph
	evidence.loss = releaseEvidence.loss
	evidence.ledgerSource = ledgerSource
	evidence.autopsySource = autopsySource
	return evidence, nil
}

func loadCapsuleReportEvidence(pack capsule.Package) (reportEvidence, error) {
	bomRecord, bomRaw, err := componentByType(pack, lineage.BOMSchemaVersion)
	if err != nil {
		return reportEvidence{}, err
	}
	bom, err := lineage.DecodeOfflineBOM(bytes.NewReader(bomRaw))
	if err != nil {
		return reportEvidence{}, err
	}
	datasetRecord, datasetRaw, err := componentByType(pack, lineage.DatasetCardSchemaVersion)
	if err != nil {
		return reportEvidence{}, err
	}
	dataset, err := lineage.DecodeDevelopmentDatasetCard(bytes.NewReader(datasetRaw))
	if err != nil {
		return reportEvidence{}, err
	}
	releaseRecord, releaseRaw, err := componentByType(pack, lineage.ReleaseSchemaVersion)
	if err != nil {
		return reportEvidence{}, err
	}
	release, err := lineage.DecodeDevelopmentRelease(bytes.NewReader(releaseRaw))
	if err != nil {
		return reportEvidence{}, err
	}
	ownerInspectionRecord, ownerInspectionRaw, err := componentByType(pack, relation.OwnerInspectionPublicAttestationSchemaVersion)
	if err != nil {
		return reportEvidence{}, err
	}
	ownerInspection, err := relation.DecodeOwnerInspectionPublicAttestation(bytes.NewReader(ownerInspectionRaw))
	if err != nil {
		return reportEvidence{}, err
	}
	return reportEvidence{
		bom: bom, bomRecord: bomRecord, dataset: dataset, datasetRecord: datasetRecord,
		release: release, releaseRecord: releaseRecord,
		ownerInspection: ownerInspection, ownerInspectionRecord: ownerInspectionRecord,
	}, nil
}

func loadVerifiedReleaseEvidence(
	repositoryRoot string,
	release lineage.VerificationLineageRelease,
	loader releaseFileLoader,
) (verifiedReleaseEvidence, error) {
	files, err := loadReleaseFiles(repositoryRoot, release, loader)
	if err != nil {
		return verifiedReleaseEvidence{}, err
	}
	if err := verifyRequiredReleaseRoles(files); err != nil {
		return verifiedReleaseEvidence{}, err
	}
	limitations, err := lineage.DecodeLimitationsLedger(bytes.NewReader(files[limitationsPath].raw))
	if err != nil {
		return verifiedReleaseEvidence{}, err
	}
	graph, err := lineage.DecodeLineageGraph(repositoryRoot, bytes.NewReader(files[lineageGraphPath].raw))
	if err != nil {
		return verifiedReleaseEvidence{}, err
	}
	loss, err := lineage.DecodeLossCertificate(repositoryRoot, bytes.NewReader(files[lossCertificatePath].raw))
	if err != nil {
		return verifiedReleaseEvidence{}, err
	}
	return verifiedReleaseEvidence{files: files, limitations: limitations, graph: graph, loss: loss}, nil
}

func verifyReleaseComponentCopies(evidence reportEvidence, files map[string]verifiedReleaseFile) error {
	fileBOM, err := lineage.DecodeOfflineBOM(bytes.NewReader(files[acceptedBOMPath].raw))
	if err != nil {
		return err
	}
	fileDataset, err := lineage.DecodeDevelopmentDatasetCard(bytes.NewReader(files[datasetCardPath].raw))
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(fileBOM, evidence.bom) {
		return errors.New("evidence explorer release BOM differs from its capsule component")
	}
	if !reflect.DeepEqual(fileDataset, evidence.dataset) {
		return errors.New("evidence explorer release dataset card differs from its capsule component")
	}
	return nil
}

func buildReportSidecars(ledger claimledger.Ledger, autopsy claimledger.Autopsy) (ArtifactRef, ArtifactRef, error) {
	ledgerRaw, err := claimledger.EncodeLedger(ledger)
	if err != nil {
		return ArtifactRef{}, ArtifactRef{}, err
	}
	autopsyRaw, err := claimledger.EncodeAutopsy(autopsy)
	if err != nil {
		return ArtifactRef{}, ArtifactRef{}, err
	}
	ledgerSource := sidecarArtifactRef("claim-ledger", claimledger.LedgerSchemaVersion, ledgerRaw, ledger.Digest)
	autopsySource := sidecarArtifactRef("claim-autopsy", claimledger.AutopsySchemaVersion, autopsyRaw, autopsy.Digest)
	return ledgerSource, autopsySource, nil
}

func verifyReportBindings(manifest capsule.Manifest, autopsy claimledger.Autopsy, evidence reportEvidence) error {
	if autopsy.CapsuleID != manifest.CapsuleID || autopsy.ManifestDigest != manifest.ManifestDigest ||
		autopsy.ClaimTransport.ProviderCalls != 0 || !autopsy.ClaimTransport.ReleaseVerified {
		return errors.New("evidence explorer autopsy is not a verified offline projection of the capsule")
	}
	if evidence.release.ProviderCallsRequired != 0 || evidence.dataset.ProviderCallsRequired != 0 ||
		evidence.limitations.ProviderCallsRequired != 0 || evidence.limitations.AgentLaunchesRequired != 0 {
		return errors.New("evidence explorer source artifacts require an external action")
	}
	if evidence.release.DatasetCardDigest != evidence.dataset.Header.Digest ||
		evidence.release.LimitationsDigest != evidence.limitations.Digest ||
		evidence.release.LineageGraphJSONDigest != evidence.graph.Digest ||
		evidence.release.LineageGraphSVGDigest != evidence.files[lineageGraphSVGPath].manifest.Digest ||
		evidence.graph.LossCertificateDigest != evidence.loss.Digest ||
		evidence.graph.OfflineProofDigest != evidence.loss.OfflineProofDigest ||
		evidence.limitations.OfflineProofDigest != evidence.graph.OfflineProofDigest {
		return errors.New("evidence explorer release, graph, loss, dataset, or limitations digests are detached")
	}
	bomParentDigest, err := releaseParentDigest(evidence.release, "bom")
	if err != nil || bomParentDigest != evidence.bom.Header.Digest ||
		evidence.graph.Paths[0].EvidenceDigest != evidence.bom.Header.Digest ||
		evidence.bom.AuditDigest != evidence.release.AuditDigest {
		return errors.New("evidence explorer accepted BOM is detached from the release or graph")
	}
	transport := autopsy.ClaimTransport
	if transport.ClaimID != evidence.bom.Evidence.ClaimID ||
		transport.CandidateID != evidence.bom.CandidateID ||
		transport.FailableProperty != evidence.bom.Evidence.FailableProperty ||
		transport.Executable != evidence.bom.Evidence.Executable ||
		!slices.Equal(transport.Operands, evidence.bom.Evidence.Operands) ||
		transport.ObservableFailureCondition != evidence.bom.Evidence.ObservableFailureCondition ||
		transport.RequestFingerprint != evidence.bom.Retention.RequestFingerprint ||
		transport.CanonicalRequestLineageDigest != evidence.bom.Retention.CanonicalRequestLineageDigest ||
		transport.Accepted != evidence.bom.Accepted {
		return errors.New("evidence explorer claim transport differs from the accepted BOM")
	}
	return verifyAcceptedTransportLayers(transport.Layers, evidence.bom)
}

func verifyAcceptedTransportLayers(layers []claimledger.ClaimTransportLayer, bom lineage.VerificationEvidenceBOM) error {
	want := []struct {
		name   string
		digest string
	}{
		{"runtime_witness", bom.ExecutionWitnessDigest},
		{"native_export", bom.Retention.NativeRecordDigest},
		{"canonical_graph", bom.Retention.CanonicalEventDigest},
		{"retained_bundle", bom.Retention.RetainedBundleDigest},
		{"verifier_request", bom.Retention.RequestFingerprint},
	}
	if len(layers) != len(want) {
		return errors.New("evidence explorer accepted transport does not contain all five layers")
	}
	for index, layer := range layers {
		if layer.Layer != want[index].name || layer.ObjectDigest != want[index].digest ||
			!layer.StructuredPresence || !layer.SemanticSufficiency {
			return fmt.Errorf("evidence explorer accepted transport layer %q differs from the BOM", want[index].name)
		}
	}
	return nil
}

func buildMethodView(manifest capsule.Manifest, lane claimledger.MethodIntegrityLane) (MethodView, error) {
	evidenceByComponent := make(map[string]ArtifactRef)
	generations, err := buildMethodGenerations(manifest, lane.Generations, evidenceByComponent)
	if err != nil {
		return MethodView{}, err
	}
	transitions, err := buildMethodTransitions(lane.Transitions, evidenceByComponent)
	if err != nil {
		return MethodView{}, err
	}
	return MethodView{Current: lane.Current, Boundary: lane.Boundary, Generations: generations, Transitions: transitions}, nil
}

func buildMethodGenerations(
	manifest capsule.Manifest,
	source []claimledger.MethodGeneration,
	evidenceByComponent map[string]ArtifactRef,
) ([]MethodGenerationView, error) {
	generations := make([]MethodGenerationView, len(source))
	for index, generation := range source {
		evidence, err := buildMethodEvidence(manifest, generation.Evidence, evidenceByComponent)
		if err != nil {
			return nil, err
		}
		generations[index] = MethodGenerationView{
			Generation: generation.Generation, State: generation.State, Evidence: evidence,
			FrozenDenominators: autopsyCounts(generation.FrozenDenominators),
			Observations:       slices.Clone(generation.Observations), NonClaims: slices.Clone(generation.NonClaims),
			GuardingTests: slices.Clone(generation.GuardingTests),
			DeepLink:      "#autopsy/method/" + generation.Generation,
		}
	}
	return generations, nil
}

func buildMethodEvidence(
	manifest capsule.Manifest,
	references []claimledger.AutopsyEvidenceRef,
	evidenceByComponent map[string]ArtifactRef,
) ([]ArtifactRef, error) {
	evidence := make([]ArtifactRef, len(references))
	for index, reference := range references {
		artifact, err := autopsyArtifactRef(manifest, reference)
		if err != nil {
			return nil, err
		}
		evidence[index] = artifact
		evidenceByComponent[reference.ComponentID] = artifact
	}
	return evidence, nil
}

func buildMethodTransitions(
	source []claimledger.MethodTransition,
	evidenceByComponent map[string]ArtifactRef,
) ([]MethodTransitionView, error) {
	transitions := make([]MethodTransitionView, len(source))
	for index, transition := range source {
		evidence := make([]ArtifactRef, len(transition.EvidenceComponentIDs))
		for evidenceIndex, componentID := range transition.EvidenceComponentIDs {
			artifact, found := evidenceByComponent[componentID]
			if !found {
				return nil, fmt.Errorf("evidence explorer method transition references absent component %q", componentID)
			}
			evidence[evidenceIndex] = artifact
		}
		transitions[index] = MethodTransitionView{
			From: transition.From, To: transition.To, Reason: transition.Reason, Evidence: evidence,
			GuardingTest:   transition.GuardingTest,
			FromDeepLink:   "#autopsy/method/" + transition.From,
			TargetDeepLink: "#autopsy/method/" + transition.To,
		}
	}
	return transitions, nil
}

func buildTransportView(
	lane claimledger.ClaimTransportLane,
	graph lineage.VerificationLineageGraph,
	loss lineage.VerificationLineageLossCertificate,
	bomSource ArtifactRef,
	lossSource ArtifactRef,
) (TransportView, error) {
	acceptedLayers := make(map[string]claimledger.ClaimTransportLayer, len(lane.Layers))
	for _, layer := range lane.Layers {
		acceptedLayers[layer.Layer] = layer
	}
	paths := make([]TransportPathView, len(graph.Paths))
	selectedPath := ""
	selectedLayer := ""
	for pathIndex, graphPath := range graph.Paths {
		view, firstLoss, err := buildTransportPath(lane.ClaimID, graphPath, acceptedLayers, bomSource, lossSource, loss.Digest)
		if err != nil {
			return TransportView{}, err
		}
		if firstLoss != "" {
			if selectedPath != "" {
				return TransportView{}, errors.New("evidence explorer graph contains multiple first-loss layers")
			}
			selectedPath, selectedLayer = graphPath.PathID, firstLoss
		}
		paths[pathIndex] = view
	}
	if selectedPath == "" || selectedLayer == "" {
		return TransportView{}, errors.New("evidence explorer graph has no first-loss selection")
	}
	return TransportView{
		ClaimID: lane.ClaimID, Accepted: lane.Accepted, SelectedPath: selectedPath,
		SelectedLayer: selectedLayer, Paths: paths,
	}, nil
}

func buildTransportPath(
	claimID string,
	graphPath lineage.LineageGraphPath,
	acceptedLayers map[string]claimledger.ClaimTransportLayer,
	bomSource ArtifactRef,
	lossSource ArtifactRef,
	lossDigest string,
) (TransportPathView, string, error) {
	deepLink := "#autopsy/transport/" + claimID + "/" + graphPath.PathID
	accepted := graphPath.EvidenceDigest == bomSource.ArtifactDigest
	layers := make([]TransportLayerView, len(graphPath.Layers))
	firstLoss := ""
	for index, graphLayer := range graphPath.Layers {
		layer, err := buildTransportLayer(graphLayer, deepLink, accepted, acceptedLayers)
		if err != nil {
			return TransportPathView{}, "", err
		}
		if graphLayer.Status == "first_loss" {
			firstLoss = graphLayer.Layer
		}
		layers[index] = layer
	}
	evidence, err := transportEvidence(graphPath, bomSource, lossSource, lossDigest)
	if err != nil {
		return TransportPathView{}, "", err
	}
	return TransportPathView{
		PathID: graphPath.PathID, Label: graphPath.Label, TerminalState: string(graphPath.TerminalState),
		Evidence: evidence, Layers: layers, DeepLink: deepLink,
	}, firstLoss, nil
}

func buildTransportLayer(
	graphLayer lineage.LossCertificateLayer,
	deepLink string,
	accepted bool,
	acceptedLayers map[string]claimledger.ClaimTransportLayer,
) (TransportLayerView, error) {
	view := TransportLayerView{
		Layer: graphLayer.Layer, Status: graphLayer.Status, ObjectDigestAvailability: AvailabilityUnsupported,
		DetailAvailability: AvailabilityUnsupported, DeepLink: deepLink + "/" + graphLayer.Layer,
	}
	if accepted {
		acceptedLayer, found := acceptedLayers[graphLayer.Layer]
		if !found {
			return TransportLayerView{}, fmt.Errorf("evidence explorer graph layer %q is absent from the accepted autopsy", graphLayer.Layer)
		}
		digest := acceptedLayer.ObjectDigest
		view.ObjectDigest = &digest
		view.ObjectDigestAvailability = AvailabilityAvailable
		view.DetailAvailability = AvailabilityAvailable
		view.RequiredFields = slices.Clone(acceptedLayer.RequiredFields)
		view.DecisiveChannels = slices.Clone(acceptedLayer.DecisiveChannels)
	} else if graphLayer.Status == "first_loss" || graphLayer.Status == "not_reached" {
		view.ObjectDigestAvailability = AvailabilityNotApplicable
		view.DetailAvailability = AvailabilityNotApplicable
	}
	return view, nil
}

func transportEvidence(
	graphPath lineage.LineageGraphPath,
	bomSource ArtifactRef,
	lossSource ArtifactRef,
	lossDigest string,
) (ArtifactRef, error) {
	if graphPath.EvidenceDigest == bomSource.ArtifactDigest {
		return bomSource, nil
	}
	if graphPath.EvidenceDigest == lossDigest {
		return lossSource, nil
	}
	return ArtifactRef{}, fmt.Errorf("evidence explorer graph path %q references unknown evidence", graphPath.PathID)
}

func buildLimitationViews(
	ledger lineage.VerificationLineageLimitationsLedger,
	source ArtifactRef,
	autopsySource ArtifactRef,
) ([]LimitationView, error) {
	views := make([]LimitationView, len(ledger.Limitations))
	for index, limitation := range ledger.Limitations {
		availability, err := limitationAvailability(limitation)
		if err != nil {
			return nil, err
		}
		var resolvedBy *ArtifactRef
		if limitation.ID == "capsule_integration" {
			resolved := autopsySource
			resolvedBy = &resolved
		}
		views[index] = LimitationView{
			ID: limitation.ID, ArtifactStatus: limitation.Status, CurrentAvailability: availability,
			Boundary: limitation.Boundary, RequiredResolution: limitation.RequiredResolution,
			Source: source, ResolvedBy: resolvedBy,
		}
	}
	return views, nil
}

func limitationAvailability(limitation lineage.VerificationLineageLimitation) (Availability, error) {
	if limitation.ID == "capsule_integration" && limitation.Status == "not_implemented" {
		return AvailabilityAvailable, nil
	}
	switch limitation.Status {
	case "not_measured":
		return AvailabilityNotMeasured, nil
	case "unsupported", "not_implemented":
		return AvailabilityUnsupported, nil
	case "not_run":
		return AvailabilityNotRun, nil
	case "not_authorized":
		return AvailabilityNotAuthorized, nil
	case "out_of_scope":
		return AvailabilityNotApplicable, nil
	case "withheld_private_parent":
		return AvailabilityWithheldPrivateParent, nil
	default:
		return "", fmt.Errorf("evidence explorer limitation %q has unsupported status %q", limitation.ID, limitation.Status)
	}
}

func loadReleaseFiles(
	repositoryRoot string,
	release lineage.VerificationLineageRelease,
	loader releaseFileLoader,
) (map[string]verifiedReleaseFile, error) {
	if len(release.Files) == 0 || len(release.Files) > maximumReleaseFiles {
		return nil, errors.New("evidence explorer release file count exceeds its closed bound")
	}
	files := make(map[string]verifiedReleaseFile, len(release.Files))
	var total int64
	for _, file := range release.Files {
		if !validReleasePath(file.Path) || file.Bytes < 1 || file.Bytes > maximumReleaseFileBytes ||
			total > maximumReleaseTotal-file.Bytes {
			return nil, fmt.Errorf("evidence explorer release file %q exceeds its safety bound", file.Path)
		}
		total += file.Bytes
		raw, err := loader(repositoryRoot, file)
		if err != nil {
			return nil, fmt.Errorf("load evidence explorer release file %q: %w", file.Path, err)
		}
		if int64(len(raw)) != file.Bytes || protocol.DigestBytes(raw) != file.Digest {
			return nil, fmt.Errorf("evidence explorer release file %q differs from its manifest", file.Path)
		}
		if _, duplicate := files[file.Path]; duplicate {
			return nil, fmt.Errorf("evidence explorer release repeats file %q", file.Path)
		}
		files[file.Path] = verifiedReleaseFile{manifest: file, raw: slices.Clone(raw)}
	}
	return files, nil
}

func loadReleaseFile(repositoryRoot string, file lineage.ReleaseFile) ([]byte, error) {
	if !validReleasePath(file.Path) || file.Bytes < 1 || file.Bytes > maximumReleaseFileBytes {
		return nil, errors.New("release file path or size is invalid")
	}
	rootPath, err := resolvedRepositoryRoot(repositoryRoot)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	relative := filepath.FromSlash(file.Path)
	walked, err := inspectReleasePath(root, relative)
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	raw, readErr := readReleaseFile(root, relative, file, walked)
	return raw, errors.Join(readErr, root.Close())
}

func resolvedRepositoryRoot(repositoryRoot string) (string, error) {
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", errors.New("repository root is not a readable directory")
	}
	return root, nil
}

func inspectReleasePath(root *os.Root, relative string) (os.FileInfo, error) {
	current := ""
	var info os.FileInfo
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, segment)
		var err error
		info, err = root.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("release path contains a symbolic link")
		}
	}
	if info == nil || !info.Mode().IsRegular() {
		return nil, errors.New("release path is not a regular file")
	}
	return info, nil
}

func readReleaseFile(root *os.Root, relative string, manifest lineage.ReleaseFile, walked os.FileInfo) ([]byte, error) {
	opened, err := root.Open(relative)
	if err != nil {
		return nil, err
	}
	openedInfo, err := opened.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(walked, openedInfo) {
		return nil, errors.Join(err, opened.Close(), errors.New("release file changed while it was opened"))
	}
	raw, err := io.ReadAll(io.LimitReader(opened, maximumReleaseFileBytes+1))
	finalInfo, statErr := opened.Stat()
	pathInfo, pathErr := root.Lstat(relative)
	closeErr := opened.Close()
	if err != nil || statErr != nil || pathErr != nil || closeErr != nil ||
		int64(len(raw)) > maximumReleaseFileBytes || int64(len(raw)) != manifest.Bytes ||
		!pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(openedInfo, finalInfo) || !os.SameFile(finalInfo, pathInfo) ||
		openedInfo.Size() != finalInfo.Size() || !openedInfo.ModTime().Equal(finalInfo.ModTime()) {
		return nil, errors.Join(err, statErr, pathErr, closeErr, errors.New("release file changed while it was read"))
	}
	return raw, nil
}

func verifyRequiredReleaseRoles(files map[string]verifiedReleaseFile) error {
	required := map[string]string{
		acceptedBOMPath:     "accepted_bom_example",
		datasetCardPath:     "dataset_card",
		limitationsPath:     "limitations",
		lineageGraphPath:    "canonical_graph",
		lineageGraphSVGPath: "graph_projection",
		lossCertificatePath: "loss_certificate",
	}
	for path, role := range required {
		file, found := files[path]
		if !found || file.manifest.Role != role {
			return fmt.Errorf("evidence explorer release lacks required %q file %q", role, path)
		}
	}
	return nil
}

func componentByType(pack capsule.Package, typeID string) (capsule.ComponentRecord, []byte, error) {
	var found *capsule.ComponentRecord
	for index := range pack.Manifest.Components {
		component := &pack.Manifest.Components[index]
		if component.TypeID != typeID {
			continue
		}
		if found != nil {
			return capsule.ComponentRecord{}, nil, fmt.Errorf("evidence explorer capsule repeats component type %q", typeID)
		}
		found = component
	}
	if found == nil {
		return capsule.ComponentRecord{}, nil, fmt.Errorf("evidence explorer capsule lacks component type %q", typeID)
	}
	raw, exists := pack.Payloads[found.Payload.Digest]
	if !exists {
		return capsule.ComponentRecord{}, nil, fmt.Errorf("evidence explorer capsule lacks payload %q", found.Payload.Digest)
	}
	return *found, slices.Clone(raw), nil
}

func autopsyArtifactRef(manifest capsule.Manifest, reference claimledger.AutopsyEvidenceRef) (ArtifactRef, error) {
	for _, component := range manifest.Components {
		if component.ComponentID != reference.ComponentID {
			continue
		}
		if component.Name != reference.Name || component.TypeID != reference.TypeID ||
			component.Payload.Digest != reference.PayloadSHA256 {
			return ArtifactRef{}, fmt.Errorf("evidence explorer autopsy component %q differs from the capsule", reference.ComponentID)
		}
		return componentArtifactRef(component, reference.ArtifactDigest), nil
	}
	return ArtifactRef{}, fmt.Errorf("evidence explorer autopsy component %q is absent from the capsule", reference.ComponentID)
}

func componentArtifactRef(component capsule.ComponentRecord, artifactDigest string) ArtifactRef {
	return ArtifactRef{
		Kind: ArtifactCapsuleComponent, ID: component.ComponentID, SchemaVersion: component.TypeID,
		PayloadSHA256: component.Payload.Digest, ArtifactDigest: artifactDigest,
	}
}

func releaseArtifactRef(file lineage.ReleaseFile, schemaVersion string, artifactDigest string) ArtifactRef {
	return ArtifactRef{
		Kind: ArtifactReleaseFile, ID: file.Path, SchemaVersion: schemaVersion,
		PayloadSHA256: file.Digest, ArtifactDigest: artifactDigest,
	}
}

func sidecarArtifactRef(id string, schemaVersion string, raw []byte, artifactDigest string) ArtifactRef {
	return ArtifactRef{
		Kind: ArtifactSidecar, ID: id, SchemaVersion: schemaVersion,
		PayloadSHA256: protocol.DigestBytes(raw), ArtifactDigest: artifactDigest,
	}
}

func releaseParentDigest(release lineage.VerificationLineageRelease, relation string) (string, error) {
	found := ""
	for _, parent := range release.Header.Parents {
		if parent.Relation != relation {
			continue
		}
		if found != "" {
			return "", fmt.Errorf("evidence explorer release repeats parent relation %q", relation)
		}
		found = parent.Digest
	}
	if found == "" {
		return "", fmt.Errorf("evidence explorer release lacks parent relation %q", relation)
	}
	return found, nil
}

func datasetCount(card lineage.VerificationLineageDatasetCard, name string) (int, error) {
	for _, count := range card.Counts {
		if count.Name == name {
			return count.Value, nil
		}
	}
	return 0, fmt.Errorf("evidence explorer dataset lacks count %q", name)
}

func datasetCounts(card lineage.VerificationLineageDatasetCard) []CountView {
	counts := make([]CountView, len(card.Counts))
	for index, count := range card.Counts {
		counts[index] = CountView{Name: count.Name, Value: count.Value}
	}
	return counts
}

func autopsyCounts(counts []claimledger.AutopsyCount) []CountView {
	views := make([]CountView, len(counts))
	for index, count := range counts {
		views[index] = CountView{Name: count.Name, Value: count.Value}
	}
	return views
}

func freshnessView(interval lineage.FreshnessInterval) FreshnessView {
	var validUntil *string
	if !interval.ValidUntil.IsZero() {
		formatted := interval.ValidUntil.UTC().Format(time.RFC3339Nano)
		validUntil = &formatted
	}
	return FreshnessView{
		State: string(interval.State), ObservedStateDigest: interval.ObservedStateDigest,
		ValidFrom:         interval.ValidFrom.UTC().Format(time.RFC3339Nano),
		ValidUntil:        validUntil,
		EvaluatedAt:       interval.EvaluatedAt.UTC().Format(time.RFC3339Nano),
		InvalidationEdges: len(interval.InvalidationEdges),
	}
}

func releaseManifest(files []lineage.ReleaseFile) []ReleaseFileView {
	manifest := make([]ReleaseFileView, len(files))
	for index, file := range files {
		manifest[index] = ReleaseFileView{
			Path: file.Path, Role: file.Role, Bytes: file.Bytes, PayloadSHA256: file.Digest,
		}
	}
	return manifest
}
