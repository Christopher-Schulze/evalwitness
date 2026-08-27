package lineage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const ParserLockVersion = "evalwitness.verification-lineage-parser-lock.v1"

type ParserLockSourceBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type ParserLockArtifactBinding struct {
	ArtifactID    string `json:"artifact_id"`
	Path          string `json:"path"`
	ContentDigest string `json:"content_digest"`
	FileSHA256    string `json:"file_sha256"`
}

type VerificationLineageParserLock struct {
	Version                     string                      `json:"version"`
	LockID                      string                      `json:"lock_id"`
	PlanDigest                  string                      `json:"plan_digest"`
	LockedAt                    time.Time                   `json:"locked_at"`
	State                       string                      `json:"state"`
	DataRole                    DataRole                    `json:"data_role"`
	FrozenFormats               []string                    `json:"frozen_formats"`
	FrozenSemanticSurfaces      []string                    `json:"frozen_semantic_surfaces"`
	SourceBindings              []ParserLockSourceBinding   `json:"source_bindings"`
	ArtifactBindings            []ParserLockArtifactBinding `json:"artifact_bindings"`
	DevelopmentVectors          int                         `json:"development_vectors"`
	ConformanceChecks           int                         `json:"conformance_checks"`
	ConformanceFailures         int                         `json:"conformance_failures"`
	CalibrationResultsInspected bool                        `json:"calibration_results_inspected"`
	ProviderCallsAllowed        int                         `json:"provider_calls_allowed"`
	PostLockChangeRule          string                      `json:"post_lock_change_rule"`
	Digest                      string                      `json:"digest"`
}

func BuildVerificationLineageParserLock(repositoryRoot string) (VerificationLineageParserLock, error) {
	if missing(repositoryRoot) {
		return VerificationLineageParserLock{}, errors.New("parser lock requires a repository root")
	}
	sourceBindings := make([]ParserLockSourceBinding, 0, len(parserLockSourcePaths()))
	for _, relativePath := range parserLockSourcePaths() {
		content, err := readLockedRepositoryFile(repositoryRoot, relativePath)
		if err != nil {
			return VerificationLineageParserLock{}, err
		}
		sourceBindings = append(sourceBindings, ParserLockSourceBinding{Path: relativePath, SHA256: digestBytes(content)})
	}
	artifactBindings, vectorCount, conformanceChecks, conformanceFailures, err := parserLockArtifactBindings(repositoryRoot)
	if err != nil {
		return VerificationLineageParserLock{}, err
	}
	lock := VerificationLineageParserLock{
		Version: ParserLockVersion, LockID: "task_069-parser-lock-v1", PlanDigest: LockedPlanDigest,
		LockedAt: time.Date(2026, time.August, 10, 13, 28, 25, 0, time.UTC), State: "locked_before_calibration",
		DataRole:      RoleAdapterDevelopment,
		FrozenFormats: []string{"claude_code_jsonl", "codex_rollout", "opencode_export"},
		FrozenSemanticSurfaces: []string{
			"adapter_field_mapping", "bom_parent_traversal", "earliest_loss_precedence", "execution_witness_pairing",
			"failability_resolution", "native_call_result_linkage", "retention_first_loss", "strict_native_admission",
		},
		SourceBindings: sourceBindings, ArtifactBindings: artifactBindings,
		DevelopmentVectors: vectorCount, ConformanceChecks: conformanceChecks, ConformanceFailures: conformanceFailures,
		CalibrationResultsInspected: false, ProviderCallsAllowed: 0,
		PostLockChangeRule: "any_bound_source_or_artifact_change_before_calibration_requires_a_new_lock_digest_after_calibration_requires_protocol_v2",
	}
	lock.Digest, err = parserLockDigest(lock)
	if err != nil {
		return VerificationLineageParserLock{}, err
	}
	return lock, lock.Validate()
}

func (lock VerificationLineageParserLock) Validate() error {
	if lock.Version != ParserLockVersion || lock.LockID != "task_069-parser-lock-v1" || lock.PlanDigest != LockedPlanDigest ||
		!lock.LockedAt.Equal(time.Date(2026, time.August, 10, 13, 28, 25, 0, time.UTC)) || lock.State != "locked_before_calibration" ||
		lock.DataRole != RoleAdapterDevelopment || lock.DevelopmentVectors != 63 || lock.ConformanceChecks != 504 || lock.ConformanceFailures != 0 ||
		lock.CalibrationResultsInspected || lock.ProviderCallsAllowed != 0 ||
		lock.PostLockChangeRule != "any_bound_source_or_artifact_change_before_calibration_requires_a_new_lock_digest_after_calibration_requires_protocol_v2" ||
		!validDigest(lock.Digest) {
		return errors.New("verification-lineage parser lock identity or boundary is invalid")
	}
	if !slices.Equal(lock.FrozenFormats, []string{"claude_code_jsonl", "codex_rollout", "opencode_export"}) ||
		!slices.Equal(lock.FrozenSemanticSurfaces, []string{
			"adapter_field_mapping", "bom_parent_traversal", "earliest_loss_precedence", "execution_witness_pairing",
			"failability_resolution", "native_call_result_linkage", "retention_first_loss", "strict_native_admission",
		}) {
		return errors.New("verification-lineage parser lock surface changed")
	}
	paths := parserLockSourcePaths()
	if len(lock.SourceBindings) != len(paths) {
		return errors.New("verification-lineage parser lock source inventory is incomplete")
	}
	for index, binding := range lock.SourceBindings {
		if binding.Path != paths[index] || !validDigest(binding.SHA256) {
			return errors.New("verification-lineage parser lock source binding is invalid or unsorted")
		}
	}
	expectedArtifacts := []string{"adapter_conformance", "golden_vectors", "plan", "schema_inventory", "source_inventory", "source_specifications"}
	if len(lock.ArtifactBindings) != len(expectedArtifacts) {
		return errors.New("verification-lineage parser lock artifact inventory is incomplete")
	}
	for index, binding := range lock.ArtifactBindings {
		if binding.ArtifactID != expectedArtifacts[index] || missing(binding.Path) || !validDigest(binding.ContentDigest) || !validDigest(binding.FileSHA256) {
			return errors.New("verification-lineage parser lock artifact binding is invalid or unsorted")
		}
	}
	expected, err := parserLockDigest(lock)
	if err != nil {
		return err
	}
	if lock.Digest != expected {
		return errors.New("verification-lineage parser lock digest is invalid")
	}
	return nil
}

func VerifyVerificationLineageParserLock(repositoryRoot string, expected VerificationLineageParserLock) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	actual, err := BuildVerificationLineageParserLock(repositoryRoot)
	if err != nil {
		return err
	}
	if actual.Digest != expected.Digest {
		return errors.New("verification-lineage parser or mapping lock differs from current source and development vectors")
	}
	return nil
}

func parserLockArtifactBindings(repositoryRoot string) ([]ParserLockArtifactBinding, int, int, int, error) {
	plan, err := DefaultPlan()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	schemaInventory, err := DefaultSchemaInventory()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	sourceInventory, err := DefaultSourceInventory()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	sourceSpecifications, err := DefaultTraceSourceSpecificationRegistry()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	golden, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	conformance, err := BuildAdapterConformanceReport()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	definitions := []struct {
		id     string
		path   string
		digest string
		value  any
	}{
		{"adapter_conformance", "eval/governance/verification-lineage-adapter-conformance-v1.json", conformance.Digest, conformance},
		{"golden_vectors", "eval/governance/verification-lineage-golden-vectors-v1.json", golden.Digest, golden},
		{"plan", "eval/governance/verification-lineage-plan-v1.json", plan.Digest, plan},
		{"schema_inventory", "eval/governance/verification-lineage-schema-inventory-v1.json", schemaInventory.Digest, schemaInventory},
		{"source_inventory", "eval/governance/verification-lineage-source-inventory-v1.json", sourceInventory.Digest, sourceInventory},
		{"source_specifications", "eval/governance/trace-source-specifications-v1.json", sourceSpecifications.Digest, sourceSpecifications},
	}
	bindings := make([]ParserLockArtifactBinding, 0, len(definitions))
	for _, definition := range definitions {
		expectedBytes, err := EncodeIndented(definition.value)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		actualBytes, err := readLockedRepositoryFile(repositoryRoot, definition.path)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		if !bytes.Equal(actualBytes, expectedBytes) {
			return nil, 0, 0, 0, errors.New("parser lock artifact differs from its reproduced canonical bytes: " + definition.path)
		}
		bindings = append(bindings, ParserLockArtifactBinding{
			ArtifactID: definition.id, Path: definition.path, ContentDigest: definition.digest, FileSHA256: digestBytes(actualBytes),
		})
	}
	return bindings, len(golden.Vectors), conformance.Summary.Checks, conformance.Summary.FailedChecks, nil
}

func readLockedRepositoryFile(repositoryRoot, relativePath string) ([]byte, error) {
	if !validReleasePath(relativePath) {
		return nil, errors.New("parser lock contains an invalid repository path")
	}
	path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("parser lock source is not a regular file: " + relativePath)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func parserLockSourcePaths() []string {
	return []string{
		"internal/lineage/adapter_conformance.go",
		"internal/lineage/assessment.go",
		"internal/lineage/bom.go",
		"internal/lineage/candidate.go",
		"internal/lineage/classification.go",
		"internal/lineage/defaults.go",
		"internal/lineage/golden_vectors.go",
		"internal/lineage/nontriviality.go",
		"internal/lineage/parser_lock.go",
		"internal/lineage/retention.go",
		"internal/lineage/witness_pairing.go",
		"internal/preprocess/adapter_claude.go",
		"internal/preprocess/adapter_codex.go",
		"internal/preprocess/adapter_opencode.go",
		"internal/preprocess/ingest.go",
		"internal/preprocess/model.go",
		"internal/preprocess/preprocess.go",
		"internal/preprocess/render.go",
		"internal/preprocess/slice.go",
		"internal/preprocess/trace_agent.go",
		"internal/preprocess/trace_export.go",
		"internal/preprocess/trace_import.go",
		"internal/preprocess/trace_model.go",
		"internal/preprocess/trace_otlp.go",
	}
}

func parserLockDigest(lock VerificationLineageParserLock) (string, error) {
	lock.Digest = ""
	return digestJSON(lock)
}
