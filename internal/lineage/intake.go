package lineage

import (
	"errors"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const TraceIntakeReportVersion = "evalwitness.verification-lineage-trace-intake.v1"

type IntakeCapabilityPlan struct {
	Format                 string `json:"format"`
	FormatVersion          string `json:"format_version"`
	Status                 string `json:"status"`
	CapabilityVectorDigest string `json:"capability_vector_digest"`
	SpecificationDigest    string `json:"specification_digest"`
	ObservedFields         int    `json:"observed_fields"`
	NotObservedFields      int    `json:"not_observed_fields"`
	FormatWideInference    bool   `json:"format_wide_inference"`
}

type IntakeConformancePlan struct {
	Status             string `json:"status"`
	DevelopmentVectors int    `json:"development_vectors"`
	Checks             int    `json:"checks"`
	PassedChecks       int    `json:"passed_checks"`
	FailedChecks       int    `json:"failed_checks"`
	InputConformance   bool   `json:"input_conformance"`
}

type IntakeLossPlan struct {
	Status                       string   `json:"status"`
	DiagnosisCode                string   `json:"diagnosis_code"`
	ExecutionWitnessRequired     bool     `json:"execution_witness_required"`
	AuthoritativeSurfaceRequired bool     `json:"authoritative_surface_required"`
	CanClassifyTerminalState     bool     `json:"can_classify_terminal_state"`
	CanBuildBOM                  bool     `json:"can_build_bom"`
	RequiredNextInputs           []string `json:"required_next_inputs"`
	ForbiddenInferences          []string `json:"forbidden_inferences"`
}

type TraceIntakeObservation struct {
	SourceDigest      string `json:"source_digest"`
	CanonicalDigest   string `json:"canonical_digest"`
	MappingDigest     string `json:"mapping_digest"`
	SourceRecords     int    `json:"source_records"`
	CanonicalEvents   int    `json:"canonical_events"`
	CausalLinks       int    `json:"causal_links"`
	ToolCalls         int    `json:"tool_calls"`
	ToolResults       int    `json:"tool_results"`
	Commands          int    `json:"commands"`
	UnpairedToolCalls int    `json:"unpaired_tool_calls"`
	UnpairedResults   int    `json:"unpaired_results"`
}

type TraceIntakeClaimBoundary struct {
	PrivacyClass      string   `json:"privacy_class"`
	AnalysisPerformed bool     `json:"analysis_performed"`
	ProviderCalls     int      `json:"provider_calls"`
	AgentLaunches     int      `json:"agent_launches"`
	SupportedClaim    string   `json:"supported_claim"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

type VerificationLineageTraceIntakeReport struct {
	Version                  string                   `json:"version"`
	PlanDigest               string                   `json:"plan_digest"`
	CapabilityMatrixDigest   string                   `json:"capability_matrix_digest"`
	AdapterConformanceDigest string                   `json:"adapter_conformance_digest"`
	CapabilityPlan           IntakeCapabilityPlan     `json:"capability_plan"`
	ConformancePlan          IntakeConformancePlan    `json:"conformance_plan"`
	LossPlan                 IntakeLossPlan           `json:"loss_plan"`
	Observation              TraceIntakeObservation   `json:"observation"`
	ClaimBoundary            TraceIntakeClaimBoundary `json:"claim_boundary"`
	Digest                   string                   `json:"digest"`
}

func BuildVerificationLineageTraceIntakeReport(result preprocess.TraceImportResult) (VerificationLineageTraceIntakeReport, error) {
	if err := result.Trajectory.Validate(); err != nil {
		return VerificationLineageTraceIntakeReport{}, err
	}
	matrix, err := BuildVerificationLineageCapabilityMatrix()
	if err != nil {
		return VerificationLineageTraceIntakeReport{}, err
	}
	conformance, err := BuildAdapterConformanceReport()
	if err != nil {
		return VerificationLineageTraceIntakeReport{}, err
	}
	format := string(result.Trajectory.SourceFormat)
	capability := intakeCapabilityPlan(format, matrix)
	conformancePlan := intakeConformancePlan(format, conformance)
	observation := TraceIntakeObservation{
		SourceDigest: result.Envelope.SourceDigest, CanonicalDigest: result.Trajectory.Digest, MappingDigest: result.Mapping.Digest,
		SourceRecords: result.Trajectory.Report.SourceRecords, CanonicalEvents: len(result.Trajectory.Events), CausalLinks: len(result.Trajectory.Links),
		UnpairedToolCalls: result.Trajectory.Report.UnpairedToolCalls, UnpairedResults: result.Trajectory.Report.UnpairedToolResults,
	}
	for _, event := range result.Trajectory.Events {
		switch event.Kind {
		case preprocess.EventToolCall:
			observation.ToolCalls++
		case preprocess.EventToolResult:
			observation.ToolResults++
		case preprocess.EventCommand:
			observation.Commands++
		}
	}
	report := VerificationLineageTraceIntakeReport{
		Version: TraceIntakeReportVersion, PlanDigest: LockedPlanDigest, CapabilityMatrixDigest: matrix.Digest, AdapterConformanceDigest: conformance.Digest,
		CapabilityPlan: capability, ConformancePlan: conformancePlan, Observation: observation,
		LossPlan: IntakeLossPlan{
			Status: "blocked_before_lineage_classification", DiagnosisCode: "independent_execution_witness_required",
			ExecutionWitnessRequired: true, AuthoritativeSurfaceRequired: true,
			RequiredNextInputs:  []string{"capture_policy_identity", "execution_witness", "repository_state_identity", "source_manifest"},
			ForbiddenInferences: []string{"agent_behavior_absent", "format_lossless", "provider_quality", "terminal_loss_state"},
		},
		ClaimBoundary: TraceIntakeClaimBoundary{
			PrivacyClass: string(result.Envelope.PrivacyClass), ProviderCalls: 0, AgentLaunches: 0,
			SupportedClaim:    "the local export was strictly imported and its capability, conformance, and missing-input plan were emitted before lineage classification",
			UnsupportedClaims: []string{"agent behavior", "empirical prevalence", "format transfer", "verification quality"},
		},
	}
	report.Digest, err = traceIntakeReportDigest(report)
	if err != nil {
		return VerificationLineageTraceIntakeReport{}, err
	}
	return report, report.Validate()
}

func (report VerificationLineageTraceIntakeReport) Validate() error {
	if report.Version != TraceIntakeReportVersion || report.PlanDigest != LockedPlanDigest || !validDigest(report.CapabilityMatrixDigest) ||
		!validDigest(report.AdapterConformanceDigest) || !validDigest(report.Digest) || report.CapabilityPlan.Format == "" ||
		report.CapabilityPlan.FormatWideInference || report.ConformancePlan.InputConformance {
		return errors.New("verification-lineage trace-intake identity or evidence boundary is invalid")
	}
	if !slices.Contains([]string{"development_vector_available", "pinned_control_without_native_vector", "no_pinned_capability_vector"}, report.CapabilityPlan.Status) ||
		!slices.Contains([]string{"development_conformance_available", "not_available"}, report.ConformancePlan.Status) ||
		report.CapabilityPlan.ObservedFields < 0 || report.CapabilityPlan.NotObservedFields < 0 ||
		report.ConformancePlan.DevelopmentVectors < 0 || report.ConformancePlan.Checks < 0 || report.ConformancePlan.PassedChecks < 0 || report.ConformancePlan.FailedChecks < 0 ||
		report.ConformancePlan.Checks != report.ConformancePlan.PassedChecks+report.ConformancePlan.FailedChecks {
		return errors.New("verification-lineage trace-intake capability or conformance plan is invalid")
	}
	if report.CapabilityPlan.Status == "development_vector_available" && (!validDigest(report.CapabilityPlan.CapabilityVectorDigest) || !validDigest(report.CapabilityPlan.SpecificationDigest) || report.CapabilityPlan.ObservedFields+report.CapabilityPlan.NotObservedFields != 10) {
		return errors.New("verification-lineage trace-intake native capability plan is incomplete")
	}
	if report.CapabilityPlan.Status != "development_vector_available" && (report.CapabilityPlan.CapabilityVectorDigest != "" || report.CapabilityPlan.ObservedFields != 0 || report.CapabilityPlan.NotObservedFields != 0) {
		return errors.New("verification-lineage trace-intake invented a capability vector")
	}
	if report.LossPlan.Status != "blocked_before_lineage_classification" || report.LossPlan.DiagnosisCode != "independent_execution_witness_required" ||
		!report.LossPlan.ExecutionWitnessRequired || !report.LossPlan.AuthoritativeSurfaceRequired || report.LossPlan.CanClassifyTerminalState || report.LossPlan.CanBuildBOM ||
		!slices.Equal(report.LossPlan.RequiredNextInputs, []string{"capture_policy_identity", "execution_witness", "repository_state_identity", "source_manifest"}) ||
		!slices.Equal(report.LossPlan.ForbiddenInferences, []string{"agent_behavior_absent", "format_lossless", "provider_quality", "terminal_loss_state"}) {
		return errors.New("verification-lineage trace-intake loss plan was promoted beyond its inputs")
	}
	if !validDigest(report.Observation.SourceDigest) || !validDigest(report.Observation.CanonicalDigest) || !validDigest(report.Observation.MappingDigest) ||
		report.Observation.SourceRecords < 1 || report.Observation.CanonicalEvents < 0 || report.Observation.CausalLinks < 0 ||
		report.Observation.ToolCalls < 0 || report.Observation.ToolResults < 0 || report.Observation.Commands < 0 ||
		report.Observation.UnpairedToolCalls < 0 || report.Observation.UnpairedResults < 0 {
		return errors.New("verification-lineage trace-intake observation is invalid")
	}
	if report.ClaimBoundary.PrivacyClass != string(preprocess.PrivacyMetadataOnly) || report.ClaimBoundary.AnalysisPerformed ||
		report.ClaimBoundary.ProviderCalls != 0 || report.ClaimBoundary.AgentLaunches != 0 || report.ClaimBoundary.SupportedClaim == "" ||
		!slices.Equal(report.ClaimBoundary.UnsupportedClaims, []string{"agent behavior", "empirical prevalence", "format transfer", "verification quality"}) {
		return errors.New("verification-lineage trace-intake claim boundary is invalid")
	}
	expectedDigest, err := traceIntakeReportDigest(report)
	if err != nil {
		return err
	}
	if report.Digest != expectedDigest {
		return errors.New("verification-lineage trace-intake digest is invalid")
	}
	return nil
}

func intakeCapabilityPlan(format string, matrix VerificationLineageCapabilityMatrix) IntakeCapabilityPlan {
	index := slices.IndexFunc(matrix.Vectors, func(vector TraceCapabilityVector) bool { return vector.Format == format })
	if index >= 0 {
		vector := matrix.Vectors[index]
		plan := IntakeCapabilityPlan{
			Format: format, FormatVersion: vector.FormatVersion, Status: "development_vector_available",
			CapabilityVectorDigest: vector.Header.Digest, SpecificationDigest: vector.SpecificationDigest,
		}
		for _, field := range vector.Fields {
			if field.ObservedInCapture == CapabilityObserved {
				plan.ObservedFields++
			} else {
				plan.NotObservedFields++
			}
		}
		return plan
	}
	registry, err := DefaultTraceSourceSpecificationRegistry()
	if err == nil {
		if specification, found := specificationByFormat(registry.Specifications, format); found {
			digest, digestErr := digestJSON(specification)
			if digestErr == nil {
				return IntakeCapabilityPlan{Format: format, FormatVersion: specification.FormatVersion, Status: "pinned_control_without_native_vector", SpecificationDigest: digest}
			}
		}
	}
	return IntakeCapabilityPlan{Format: format, FormatVersion: "unregistered", Status: "no_pinned_capability_vector"}
}

func intakeConformancePlan(format string, report AdapterConformanceReport) IntakeConformancePlan {
	plan := IntakeConformancePlan{Status: "not_available"}
	for _, item := range report.Cases {
		if string(item.Format) != format {
			continue
		}
		plan.Status = "development_conformance_available"
		plan.DevelopmentVectors++
		for _, check := range item.Checks {
			plan.Checks++
			if check.Passed {
				plan.PassedChecks++
			} else {
				plan.FailedChecks++
			}
		}
	}
	return plan
}

func traceIntakeReportDigest(report VerificationLineageTraceIntakeReport) (string, error) {
	report.Digest = ""
	return digestJSON(report)
}
