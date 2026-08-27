package lineage

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const CapabilityMatrixVersion = "evalwitness.verification-lineage-capability-matrix.v1"

type CapabilityMatrixSummary struct {
	Formats                     int `json:"formats"`
	Fields                      int `json:"fields"`
	RepresentabilityRequired    int `json:"representability_required"`
	RepresentabilityOptional    int `json:"representability_optional"`
	RepresentabilityUnsupported int `json:"representability_unsupported"`
	RepresentabilityUnspecified int `json:"representability_unspecified"`
	Observed                    int `json:"observed"`
	NotObserved                 int `json:"not_observed"`
}

type CapabilityMatrixClaimBoundary struct {
	EvidenceRole        DataRole `json:"evidence_role"`
	FormatWideClaim     bool     `json:"format_wide_claim"`
	EmpiricalPrevalence bool     `json:"empirical_prevalence"`
	ProviderCalls       int      `json:"provider_calls"`
	AgentLaunches       int      `json:"agent_launches"`
	SupportedClaim      string   `json:"supported_claim"`
	UnsupportedClaims   []string `json:"unsupported_claims"`
}

type VerificationLineageCapabilityMatrix struct {
	Version                  string                        `json:"version"`
	MatrixID                 string                        `json:"matrix_id"`
	PlanDigest               string                        `json:"plan_digest"`
	SourceRegistryDigest     string                        `json:"source_registry_digest"`
	GoldenSetDigest          string                        `json:"golden_set_digest"`
	AdapterConformanceDigest string                        `json:"adapter_conformance_digest"`
	Vectors                  []TraceCapabilityVector       `json:"vectors"`
	Summary                  CapabilityMatrixSummary       `json:"summary"`
	ClaimBoundary            CapabilityMatrixClaimBoundary `json:"claim_boundary"`
	Digest                   string                        `json:"digest"`
}

type capabilityMappingState struct {
	Field            string          `json:"field"`
	Representability CapabilityState `json:"representability"`
	Observation      CapabilityState `json:"observation"`
}

func BuildVerificationLineageCapabilityMatrix() (VerificationLineageCapabilityMatrix, error) {
	matrix, err := buildCapabilityMatrixUnchecked()
	if err != nil {
		return VerificationLineageCapabilityMatrix{}, err
	}
	return matrix, matrix.Validate()
}

func (matrix VerificationLineageCapabilityMatrix) Validate() error {
	if matrix.Version != CapabilityMatrixVersion || matrix.MatrixID != "task_069-capability-matrix-v1" || matrix.PlanDigest != LockedPlanDigest ||
		!validDigest(matrix.SourceRegistryDigest) || !validDigest(matrix.GoldenSetDigest) || !validDigest(matrix.AdapterConformanceDigest) || !validDigest(matrix.Digest) {
		return errors.New("verification-lineage capability-matrix identity is invalid")
	}
	if len(matrix.Vectors) != 3 {
		return errors.New("verification-lineage capability matrix requires the three native development formats")
	}
	previous := ""
	for _, vector := range matrix.Vectors {
		if vector.Format <= previous {
			return errors.New("verification-lineage capability vectors are unsorted")
		}
		if err := vector.Validate(); err != nil {
			return err
		}
		previous = vector.Format
	}
	wantSummary := CapabilityMatrixSummary{
		Formats: 3, Fields: 30, RepresentabilityRequired: 4, RepresentabilityOptional: 14,
		RepresentabilityUnsupported: 1, RepresentabilityUnspecified: 11, Observed: 26, NotObserved: 4,
	}
	if matrix.Summary != wantSummary {
		return fmt.Errorf("verification-lineage capability-matrix summary changed: %#v", matrix.Summary)
	}
	if matrix.ClaimBoundary.EvidenceRole != RoleAdapterDevelopment || matrix.ClaimBoundary.FormatWideClaim || matrix.ClaimBoundary.EmpiricalPrevalence ||
		matrix.ClaimBoundary.ProviderCalls != 0 || matrix.ClaimBoundary.AgentLaunches != 0 || matrix.ClaimBoundary.SupportedClaim == "" ||
		!slices.Equal(matrix.ClaimBoundary.UnsupportedClaims, []string{"format is lossless", "population prevalence", "provider quality", "universal format capability"}) {
		return errors.New("verification-lineage capability-matrix claim boundary is invalid")
	}
	expected, err := buildCapabilityMatrixUnchecked()
	if err != nil {
		return err
	}
	actualWithoutDigest := matrix
	actualWithoutDigest.Digest = ""
	expectedWithoutDigest := expected
	expectedWithoutDigest.Digest = ""
	if !reflect.DeepEqual(actualWithoutDigest, expectedWithoutDigest) || matrix.Digest != expected.Digest {
		return errors.New("verification-lineage capability matrix differs from sealed specifications and vectors")
	}
	return nil
}

func buildCapabilityMatrixUnchecked() (VerificationLineageCapabilityMatrix, error) {
	registry, err := DefaultTraceSourceSpecificationRegistry()
	if err != nil {
		return VerificationLineageCapabilityMatrix{}, err
	}
	golden, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		return VerificationLineageCapabilityMatrix{}, err
	}
	conformance, err := BuildAdapterConformanceReport()
	if err != nil {
		return VerificationLineageCapabilityMatrix{}, err
	}
	matrix := VerificationLineageCapabilityMatrix{
		Version: CapabilityMatrixVersion, MatrixID: "task_069-capability-matrix-v1", PlanDigest: LockedPlanDigest,
		SourceRegistryDigest: registry.Digest, GoldenSetDigest: golden.Digest, AdapterConformanceDigest: conformance.Digest,
		ClaimBoundary: CapabilityMatrixClaimBoundary{
			EvidenceRole: RoleAdapterDevelopment, ProviderCalls: 0, AgentLaunches: 0,
			SupportedClaim:    "pinned specification representability and fields observed in the sealed development vectors are reported as separate axes",
			UnsupportedClaims: []string{"format is lossless", "population prevalence", "provider quality", "universal format capability"},
		},
	}
	for _, format := range []string{"claude_code_jsonl", "codex_rollout_jsonl", "opencode_export_json"} {
		specification, found := specificationByFormat(registry.Specifications, format)
		if !found {
			return VerificationLineageCapabilityMatrix{}, fmt.Errorf("missing source specification for %s", format)
		}
		vector, buildErr := buildCapabilityVector(specification, golden)
		if buildErr != nil {
			return VerificationLineageCapabilityMatrix{}, buildErr
		}
		matrix.Vectors = append(matrix.Vectors, vector)
	}
	matrix.Summary = summarizeCapabilityMatrix(matrix.Vectors)
	matrix.Digest, err = capabilityMatrixDigest(matrix)
	if err != nil {
		return VerificationLineageCapabilityMatrix{}, err
	}
	return matrix, nil
}

func buildCapabilityVector(specification TraceSourceSpecification, golden GoldenVectorFixtureSet) (TraceCapabilityVector, error) {
	specificationDigest, err := digestJSON(specification)
	if err != nil {
		return TraceCapabilityVector{}, err
	}
	vectorIDs := make([]string, 0, 21)
	for _, vector := range golden.Vectors {
		if string(vector.Format) == specification.Format {
			vectorIDs = append(vectorIDs, vector.VectorID)
		}
	}
	sort.Strings(vectorIDs)
	observations, err := observeCapabilityFields(preprocess.SourceFormat(specification.Format))
	if err != nil {
		return TraceCapabilityVector{}, err
	}
	mappingStates := make([]capabilityMappingState, 0, len(specification.ExpectedCapabilities))
	vector := TraceCapabilityVector{
		Header: ArtifactHeader{
			SchemaVersion: CapabilitySchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "capability-" + specification.ID, TaskID: "TASK-069", TaskGroupID: "study", DataRole: RoleAdapterDevelopment,
			PlanDigest: LockedPlanDigest,
			Parents:    []ParentRef{{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-verification-lineage-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest}},
		},
		Format: specification.Format, FormatVersion: specification.FormatVersion, SpecificationIdentity: specification.ID,
		SpecificationDigest: specificationDigest, SpecificationStability: specification.SpecificationStatus,
	}
	for _, expected := range specification.ExpectedCapabilities {
		observed := observations[expected.Field]
		mappingStates = append(mappingStates, capabilityMappingState{Field: expected.Field, Representability: expected.Representability, Observation: observed})
		vector.Fields = append(vector.Fields, CapabilityField{
			Field: expected.Field, RepresentableByFormat: expected.Representability, ObservedInCapture: observed,
			NormativeEvidence: expected.Evidence, GoldenVectorIDs: append([]string(nil), vectorIDs...),
			Notes: fmt.Sprintf("sealed development vectors only; observed=%s; no format-wide inference", observed),
		})
	}
	vector.MappingDiffDigest, err = digestJSON(mappingStates)
	if err != nil {
		return TraceCapabilityVector{}, err
	}
	vector.Header.Digest, err = artifactDigest(vector)
	if err != nil {
		return TraceCapabilityVector{}, err
	}
	return vector, vector.Validate()
}

func observeCapabilityFields(format preprocess.SourceFormat) (map[string]CapabilityState, error) {
	observed := make(map[string]bool, len(capabilityFieldNames()))
	definitions := goldenCaseDefinitions()
	for _, definition := range definitions {
		raw, _, err := goldenNativeDocument(format, definition)
		if err != nil {
			return nil, err
		}
		if raw == "" {
			continue
		}
		result, err := preprocess.ImportTraceBytes([]byte(raw), preprocess.DefaultTraceImportOptions())
		if err != nil {
			continue
		}
		if result.Trajectory.Report.SourceRecords > 0 {
			observed["source_record"] = true
		}
		if result.Trajectory.Report.RedactionHits > 0 {
			observed["redaction_provenance"] = true
		}
		for _, event := range result.Trajectory.Events {
			if event.Timestamp != "" {
				observed["timestamps"] = true
			}
			if event.Message != nil && event.Message.Role != "" {
				observed["role"] = true
			}
			if event.ToolCall != nil {
				observed["tool_call"] = true
				if event.ToolCall.CallID != "" {
					observed["call_id"] = true
				}
			}
			if event.ToolResult != nil {
				observed["result"] = true
				if event.ToolResult.CallID != "" {
					observed["call_id"] = true
				}
				if event.ToolResult.ExitCode != nil {
					observed["exit_status"] = true
				}
			}
			if event.Command != nil && event.Command.Display != "" {
				observed["command_display"] = true
				if event.Command.ExitCode != nil {
					observed["exit_status"] = true
				}
			}
		}
		for _, link := range result.Trajectory.Links {
			if link.Kind == preprocess.LinkCallResult {
				observed["transaction_boundaries"] = true
			}
		}
	}
	states := make(map[string]CapabilityState, len(capabilityFieldNames()))
	for _, field := range capabilityFieldNames() {
		states[field] = CapabilityNotObserved
		if observed[field] {
			states[field] = CapabilityObserved
		}
	}
	return states, nil
}

func specificationByFormat(specifications []TraceSourceSpecification, format string) (TraceSourceSpecification, bool) {
	index := slices.IndexFunc(specifications, func(specification TraceSourceSpecification) bool { return specification.Format == format })
	if index < 0 {
		return TraceSourceSpecification{}, false
	}
	return specifications[index], true
}

func summarizeCapabilityMatrix(vectors []TraceCapabilityVector) CapabilityMatrixSummary {
	summary := CapabilityMatrixSummary{Formats: len(vectors)}
	for _, vector := range vectors {
		for _, field := range vector.Fields {
			summary.Fields++
			switch field.RepresentableByFormat {
			case CapabilityRequired:
				summary.RepresentabilityRequired++
			case CapabilityOptional:
				summary.RepresentabilityOptional++
			case CapabilityUnsupported:
				summary.RepresentabilityUnsupported++
			case CapabilityUnspecified:
				summary.RepresentabilityUnspecified++
			}
			if field.ObservedInCapture == CapabilityObserved {
				summary.Observed++
			} else {
				summary.NotObserved++
			}
		}
	}
	return summary
}

func capabilityMatrixDigest(matrix VerificationLineageCapabilityMatrix) (string, error) {
	matrix.Digest = ""
	return digestJSON(matrix)
}
