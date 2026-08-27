package lineage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

const LimitationsLedgerVersion = "evalwitness.verification-lineage-limitations.v1"

type VerificationLineageLimitation struct {
	ID                 string `json:"id"`
	Status             string `json:"status"`
	Boundary           string `json:"boundary"`
	RequiredResolution string `json:"required_resolution"`
}

type VerificationLineageLimitationsLedger struct {
	Version               string                          `json:"version"`
	LedgerID              string                          `json:"ledger_id"`
	OfflineProofDigest    string                          `json:"offline_proof_digest"`
	CorpusFeasibility     string                          `json:"corpus_feasibility"`
	EvidenceRole          DataRole                        `json:"evidence_role"`
	Limitations           []VerificationLineageLimitation `json:"limitations"`
	ProviderCallsRequired int                             `json:"provider_calls_required"`
	AgentLaunchesRequired int                             `json:"agent_launches_required"`
	Digest                string                          `json:"digest"`
}

type offlinePortfolioEvidence struct {
	Matrix     VerificationLineageCapabilityMatrix
	Source     VerificationLineageSource
	Witness    ExecutionWitness
	Candidate  LineageCandidate
	Assessment LineageAssessment
	Capability TraceCapabilityVector
	Audit      LineageAudit
	BOM        VerificationEvidenceBOM
}

func BuildVerificationLineageOfflineBOM(repositoryRoot string) (VerificationEvidenceBOM, error) {
	evidence, err := buildOfflinePortfolioEvidence(repositoryRoot)
	if err != nil {
		return VerificationEvidenceBOM{}, err
	}
	return evidence.BOM, evidence.BOM.Validate()
}

func BuildVerificationLineageOfflineAudit(repositoryRoot string) (LineageAudit, error) {
	evidence, err := buildOfflinePortfolioEvidence(repositoryRoot)
	if err != nil {
		return LineageAudit{}, err
	}
	return evidence.Audit, evidence.Audit.Validate()
}

func VerifyVerificationLineageOfflineAudit(repositoryRoot string, audit LineageAudit) error {
	if err := audit.Validate(); err != nil {
		return err
	}
	expected, err := BuildVerificationLineageOfflineAudit(repositoryRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(audit, expected) {
		return errors.New("verification-lineage offline audit differs from canonical development evidence")
	}
	return nil
}

func VerifyVerificationLineageOfflineBOM(repositoryRoot string, bom VerificationEvidenceBOM) error {
	if err := bom.Validate(); err != nil {
		return err
	}
	expected, err := BuildVerificationLineageOfflineBOM(repositoryRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(bom, expected) {
		return errors.New("verification-lineage offline BOM differs from canonical development evidence")
	}
	return nil
}

func BuildVerificationLineageDevelopmentDatasetCard(repositoryRoot string) (VerificationLineageDatasetCard, error) {
	evidence, err := buildOfflinePortfolioEvidence(repositoryRoot)
	if err != nil {
		return VerificationLineageDatasetCard{}, err
	}
	parents := []ParentRef{
		lockedPlanParent(),
		parentFromArtifact("audit", evidence.Audit.Header),
	}
	capabilityParents := make([]ParentRef, len(evidence.Matrix.Vectors))
	for index, vector := range evidence.Matrix.Vectors {
		capabilityParents[index] = parentFromArtifact("capability", vector.Header)
	}
	sort.Slice(capabilityParents, func(left, right int) bool {
		return capabilityParents[left].ObjectID < capabilityParents[right].ObjectID
	})
	parents = append(parents, capabilityParents...)
	card := VerificationLineageDatasetCard{
		Header: ArtifactHeader{
			SchemaVersion: DatasetCardSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "task_069-development-dataset-card-v1", TaskID: "TASK-069", TaskGroupID: "study",
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest, Parents: parents,
		},
		DatasetID: "task_069-development-evidence-v1", Title: "EvalWitness verification-lineage development evidence",
		Purpose:           "reproduce native-format conformance, one accepted five-layer chain, and one non-failable counterexample without empirical generalization",
		SourcePopulations: []string{"checked_in_synthetic_execution_controls", "historical_vendor_adapter_goldens", "pinned_upstream_interoperability_fixtures"},
		Formats:           []string{"claude_code_jsonl", "codex_rollout_jsonl", "opencode_export_json"},
		AgentEcosystems:   []string{"claude_code_adapter_fixture", "codex_adapter_fixture", "opencode_adapter_fixture"},
		Roles:             []DataRole{RoleAdapterDevelopment},
		ClusterDimensions: []string{"lineage_id", "near_duplicate_id", "repository_id", "source_session_id", "task_id"},
		InclusionCriteria: []string{"checked_in_public_or_licensed_fixture", "deterministic_provider_free_reproduction", "strict_source_and_lineage_validation"},
		ExclusionCriteria: []string{"missing_independent_execution_witness_for_behavior_claim", "research_role_not_admitted", "restricted_or_redistribution_unresolved_content"},
		Counts: []DatasetCardCount{
			{Name: "accepted_boms", Value: 1}, {Name: "adapter_conformance_checks", Value: 504},
			{Name: "development_fixtures", Value: 2}, {Name: "empirical_task_groups", Value: 0},
			{Name: "golden_vectors", Value: 63}, {Name: "research_admitted_sources", Value: 0},
		},
		License:             "repository license plus source-specific fixture licenses recorded in the sealed source registry",
		PrivacyProjection:   "public synthetic and metadata-safe development evidence only",
		RedactionLossPolicy: "redaction remains an explicit measured loss and never proves behavior absence",
		IntendedUses:        []string{"adapter regression testing", "offline portfolio demonstration", "verification-lineage method development"},
		OutOfScopeUses:      []string{"agent behavior prevalence", "empirical lineage survival", "provider or model ranking", "task correctness"},
		KnownLimitations:    []string{"format holdout is development contaminated", "no research source is admitted", "syntax-family candidate universe was not sealed", "two fixtures are not an empirical denominator"},
		ReproductionCommand: "scripts/tests/run-claimcheck.sh", ProviderCallsRequired: 0,
	}
	card.Header.Digest, err = artifactDigest(card)
	if err != nil {
		return VerificationLineageDatasetCard{}, err
	}
	return card, card.Validate()
}

func VerifyVerificationLineageDevelopmentDatasetCard(repositoryRoot string, card VerificationLineageDatasetCard) error {
	if err := card.Validate(); err != nil {
		return err
	}
	expected, err := BuildVerificationLineageDevelopmentDatasetCard(repositoryRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(card, expected) {
		return errors.New("verification-lineage dataset card differs from canonical development evidence")
	}
	return nil
}

func BuildVerificationLineageLimitationsLedger(repositoryRoot string) (VerificationLineageLimitationsLedger, error) {
	proof, err := BuildVerificationLineageOfflineProof(repositoryRoot)
	if err != nil {
		return VerificationLineageLimitationsLedger{}, err
	}
	ledger := VerificationLineageLimitationsLedger{
		Version: LimitationsLedgerVersion, LedgerID: "task_069-development-limitations-v1", OfflineProofDigest: proof.Digest,
		CorpusFeasibility: "not_feasible_current_generation", EvidenceRole: RoleAdapterDevelopment,
		Limitations: []VerificationLineageLimitation{
			{ID: "capsule_integration", Status: "not_implemented", Boundary: "the development artifacts are content addressed but are not yet TASK 050 capsules", RequiredResolution: "implement and verify the TASK 050 capsule and claim-ledger boundary"},
			{ID: "empirical_survival", Status: "not_measured", Boundary: "two sealed development fixtures do not estimate lineage survival", RequiredResolution: "admit paired role-isolated research sources and run the prospective audit"},
			{ID: "format_transfer", Status: "unsupported", Boundary: "all v1 native mappings were used during development", RequiredResolution: "run a protocol-v2 format holdout isolated before development"},
			{ID: "human_construct_validity", Status: "not_run", Boundary: "no reviewer was contacted and no human agreement was measured", RequiredResolution: "obtain explicit authorization and complete the blinded review protocol"},
			{ID: "population_prevalence", Status: "unsupported", Boundary: "zero research task groups are admitted", RequiredResolution: "acquire the frozen minimum 20 calibration and 20 locked-test task groups"},
			{ID: "provider_quality", Status: "out_of_scope", Boundary: "format and agent implementation are not randomized provider effects", RequiredResolution: "use a separately preregistered factorized provider study"},
			{ID: "syntax_transfer", Status: "unsupported", Boundary: "the syntax-family candidate universe was not sealed before development", RequiredResolution: "freeze the complete candidate universe before protocol-v2 outcomes"},
		},
	}
	ledger.Digest, err = limitationsLedgerDigest(ledger)
	if err != nil {
		return VerificationLineageLimitationsLedger{}, err
	}
	return ledger, ledger.Validate()
}

func (ledger VerificationLineageLimitationsLedger) Validate() error {
	if ledger.Version != LimitationsLedgerVersion || ledger.LedgerID != "task_069-development-limitations-v1" ||
		!validDigest(ledger.OfflineProofDigest) || ledger.CorpusFeasibility != "not_feasible_current_generation" ||
		ledger.EvidenceRole != RoleAdapterDevelopment || ledger.ProviderCallsRequired != 0 || ledger.AgentLaunchesRequired != 0 ||
		len(ledger.Limitations) == 0 || !validDigest(ledger.Digest) {
		return errors.New("verification-lineage limitations identity or claim boundary is invalid")
	}
	previous := ""
	for _, limitation := range ledger.Limitations {
		if missing(limitation.ID, limitation.Status, limitation.Boundary, limitation.RequiredResolution) || limitation.ID <= previous {
			return errors.New("verification-lineage limitations must be complete, unique, and sorted")
		}
		previous = limitation.ID
	}
	digest, err := limitationsLedgerDigest(ledger)
	if err != nil || ledger.Digest != digest {
		return errors.New("verification-lineage limitations digest is invalid")
	}
	return nil
}

func VerifyVerificationLineageLimitationsLedger(repositoryRoot string, ledger VerificationLineageLimitationsLedger) error {
	if err := ledger.Validate(); err != nil {
		return err
	}
	expected, err := BuildVerificationLineageLimitationsLedger(repositoryRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(ledger, expected) {
		return errors.New("verification-lineage limitations differ from canonical development evidence")
	}
	return nil
}

func BuildVerificationLineageDevelopmentRelease(repositoryRoot string) (VerificationLineageRelease, error) {
	evidence, err := buildOfflinePortfolioEvidence(repositoryRoot)
	if err != nil {
		return VerificationLineageRelease{}, err
	}
	card, err := BuildVerificationLineageDevelopmentDatasetCard(repositoryRoot)
	if err != nil {
		return VerificationLineageRelease{}, err
	}
	limitations, err := BuildVerificationLineageLimitationsLedger(repositoryRoot)
	if err != nil {
		return VerificationLineageRelease{}, err
	}
	graph, err := BuildVerificationLineageGraph(repositoryRoot)
	if err != nil {
		return VerificationLineageRelease{}, err
	}
	inventory, err := DefaultSchemaInventory()
	if err != nil {
		return VerificationLineageRelease{}, err
	}
	files, err := buildVerificationLineageReleaseFiles(repositoryRoot)
	if err != nil {
		return VerificationLineageRelease{}, err
	}
	svgBytes, err := os.ReadFile(filepath.Join(repositoryRoot, "eval/results/verification-lineage-offline-graph-v1.svg"))
	if err != nil {
		return VerificationLineageRelease{}, fmt.Errorf("read verification-lineage SVG: %w", err)
	}
	release := VerificationLineageRelease{
		Header: ArtifactHeader{
			SchemaVersion: ReleaseSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "task_069-development-release-v1", TaskID: "TASK-069", TaskGroupID: "study",
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest,
			Parents: []ParentRef{lockedPlanParent(), parentFromArtifact("audit", evidence.Audit.Header), parentFromArtifact("bom", evidence.BOM.Header), parentFromArtifact("dataset_card", card.Header)},
		},
		ReleaseID: "task_069-development-release-v1", Version: "1.0.0-development", Files: files,
		SchemaInventoryDigest: inventory.Digest, AuditDigest: evidence.Audit.Header.Digest, DatasetCardDigest: card.Header.Digest,
		LimitationsDigest: limitations.Digest, LineageGraphJSONDigest: graph.Digest, LineageGraphSVGDigest: digestBytes(svgBytes),
		ReproductionCommand: "scripts/tests/run-claimcheck.sh", ProviderCallsRequired: 0, AllFilesVerified: true,
		PublicProjection: true, RestrictedMaterialExcluded: true,
	}
	release.Header.Digest, err = artifactDigest(release)
	if err != nil {
		return VerificationLineageRelease{}, err
	}
	return release, release.Validate()
}

func VerifyVerificationLineageDevelopmentRelease(repositoryRoot string, release VerificationLineageRelease) error {
	if err := release.Validate(); err != nil {
		return err
	}
	expected, err := BuildVerificationLineageDevelopmentRelease(repositoryRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(release, expected) {
		return errors.New("verification-lineage development release differs from canonical repository evidence")
	}
	return nil
}

func buildOfflinePortfolioEvidence(repositoryRoot string) (offlinePortfolioEvidence, error) {
	matrix, err := BuildVerificationLineageCapabilityMatrix()
	if err != nil {
		return offlinePortfolioEvidence{}, err
	}
	holdout, err := BuildVerificationLineageHoldoutReadinessAudit(repositoryRoot)
	if err != nil {
		return offlinePortfolioEvidence{}, err
	}
	var artifacts offlinePositiveArtifacts
	positive, err := buildOfflinePositiveProof(matrix, holdout.Digest, &artifacts)
	if err != nil {
		return offlinePortfolioEvidence{}, err
	}
	if positive.AuditDigest != artifacts.Audit.Header.Digest || positive.BOMDigest != artifacts.BOM.Header.Digest {
		return offlinePortfolioEvidence{}, errors.New("offline portfolio evidence detached from the positive proof")
	}
	return offlinePortfolioEvidence{
		Matrix: matrix, Source: artifacts.Source, Witness: artifacts.Witness,
		Candidate: artifacts.Candidate, Assessment: artifacts.Assessment, Capability: artifacts.Capability,
		Audit: artifacts.Audit, BOM: artifacts.BOM,
	}, nil
}

func buildVerificationLineageReleaseFiles(repositoryRoot string) ([]ReleaseFile, error) {
	specifications := []struct {
		path string
		role string
	}{
		{"eval/governance/synthetic-execution-witness-fixtures-v1.json", "execution_witness_controls"},
		{"eval/governance/trace-source-specifications-v1.json", "source_contract_registry"},
		{"eval/governance/verification-lineage-adapter-conformance-v1.json", "adapter_conformance"},
		{"eval/governance/verification-lineage-capability-matrix-v1.json", "capability_matrix"},
		{"eval/governance/verification-lineage-corpus-feasibility-v1.json", "feasibility_decision"},
		{"eval/governance/verification-lineage-golden-vectors-v1.json", "golden_vectors"},
		{"eval/governance/verification-lineage-holdout-readiness-audit-v1.json", "holdout_readiness"},
		{"eval/governance/verification-lineage-offline-proof-v1.json", "offline_proof"},
		{"eval/governance/verification-lineage-parser-lock-v1.json", "parser_lock"},
		{"eval/governance/verification-lineage-plan-v1.json", "research_plan"},
		{"eval/governance/verification-lineage-schema-inventory-v1.json", "schema_inventory"},
		{"eval/governance/verification-lineage-source-inventory-v1.json", "source_inventory"},
		{"eval/governance/verification-lineage-source-readiness-audit-v1.json", "source_readiness"},
		{"eval/results/verification-lineage-development-dataset-card-v1.json", "dataset_card"},
		{"eval/results/verification-lineage-limitations-v1.json", "limitations"},
		{"eval/results/verification-lineage-offline-audit-v1.json", "canonical_offline_audit"},
		{"eval/results/verification-lineage-offline-bom-example-v1.json", "accepted_bom_example"},
		{"eval/results/verification-lineage-offline-graph-v1.json", "canonical_graph"},
		{"eval/results/verification-lineage-offline-graph-v1.svg", "graph_projection"},
		{"eval/results/verification-lineage-same-path-loss-certificate-v1.json", "loss_certificate"},
	}
	files := make([]ReleaseFile, len(specifications))
	for index, specification := range specifications {
		content, err := os.ReadFile(filepath.Join(repositoryRoot, specification.path))
		if err != nil {
			return nil, fmt.Errorf("read release file %q: %w", specification.path, err)
		}
		files[index] = ReleaseFile{Path: specification.path, Role: specification.role, Bytes: int64(len(content)), Digest: digestBytes(content)}
	}
	return files, nil
}

func lockedPlanParent() ParentRef {
	return ParentRef{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-verification-lineage-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest}
}

func limitationsLedgerDigest(ledger VerificationLineageLimitationsLedger) (string, error) {
	ledger.Digest = ""
	return digestJSON(ledger)
}
