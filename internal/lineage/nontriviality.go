package lineage

import (
	"errors"
	"path/filepath"
	"slices"
)

type FailabilityResolution string

const (
	FailabilityProven      FailabilityResolution = "failable"
	FailabilityNonFailable FailabilityResolution = "non_failable"
	FailabilityUnresolved  FailabilityResolution = "unresolved"
)

type FailabilityReasonCode string

const (
	FailabilityReasonFailable             FailabilityReasonCode = "failable_failure_path"
	FailabilityReasonInvocationAbsent     FailabilityReasonCode = "invocation_not_executed"
	FailabilityReasonInvocationUnresolved FailabilityReasonCode = "invocation_execution_unresolved"
	FailabilityReasonStateStale           FailabilityReasonCode = "stale_state_binding"
	FailabilityReasonStateUnresolved      FailabilityReasonCode = "state_binding_unresolved"
	FailabilityReasonSelfComparison       FailabilityReasonCode = "self_comparison"
	FailabilityReasonSameStateComparison  FailabilityReasonCode = "same_state_comparison"
	FailabilityReasonConstantSuccess      FailabilityReasonCode = "constant_success"
	FailabilityReasonNeutralizedPipeline  FailabilityReasonCode = "neutralized_pipeline_status"
	FailabilityReasonPipelineUnresolved   FailabilityReasonCode = "pipeline_status_unresolved"
	FailabilityReasonUncheckedDigest      FailabilityReasonCode = "unchecked_digest_generation"
	FailabilityReasonZeroTests            FailabilityReasonCode = "zero_tests_executed"
	FailabilityReasonSkippedTests         FailabilityReasonCode = "all_tests_skipped"
	FailabilityReasonCacheUnresolved      FailabilityReasonCode = "cached_result_state_unresolved"
	FailabilityReasonWrapperAbsent        FailabilityReasonCode = "wrapper_did_not_execute_inner_check"
	FailabilityReasonWrapperUnresolved    FailabilityReasonCode = "wrapper_execution_unresolved"
	FailabilityReasonWrapperNeutralized   FailabilityReasonCode = "wrapper_neutralized_inner_status"
	FailabilityReasonObservableUnbound    FailabilityReasonCode = "failure_observable_unbound"
)

type ProofState string

const (
	ProofProven     ProofState = "proven"
	ProofDisproven  ProofState = "disproven"
	ProofUnresolved ProofState = "unresolved"
)

type ChecksumMode string

const (
	ChecksumGenerate ChecksumMode = "generate"
	ChecksumVerify   ChecksumMode = "verify"
)

type PipelineStatusPolicy string

const (
	PipelineLastStage PipelineStatusPolicy = "last_stage"
	PipelinePipefail  PipelineStatusPolicy = "pipefail"
	PipelineUnknown   PipelineStatusPolicy = "unknown"
)

type DirectProbe struct{}

type ComparisonProbe struct {
	LeftSubjectAlias   string `json:"left_subject_alias"`
	RightSubjectAlias  string `json:"right_subject_alias"`
	LeftStateIdentity  string `json:"left_state_identity"`
	RightStateIdentity string `json:"right_state_identity"`
}

type ChecksumProbe struct {
	Mode          ChecksumMode `json:"mode"`
	ManifestBound bool         `json:"manifest_bound"`
}

type TestRunProbe struct {
	CountsObserved   bool       `json:"counts_observed"`
	Discovered       int        `json:"discovered"`
	Executed         int        `json:"executed"`
	Skipped          int        `json:"skipped"`
	CacheUsed        ProofState `json:"cache_used"`
	CacheStateDigest string     `json:"cache_state_digest"`
}

type PipelineStage struct {
	StageID         string `json:"stage_id"`
	ConstantSuccess bool   `json:"constant_success"`
}

type PipelineProbe struct {
	Stages             []PipelineStage      `json:"stages"`
	DecisiveStageIndex int                  `json:"decisive_stage_index"`
	StatusPolicy       PipelineStatusPolicy `json:"status_policy"`
}

type WrapperProbe struct {
	InnerExecution        ProofState            `json:"inner_execution"`
	InnerStatusPropagated ProofState            `json:"inner_status_propagated"`
	InnerResolution       FailabilityResolution `json:"inner_resolution"`
	InnerReason           FailabilityReasonCode `json:"inner_reason"`
}

type ConstantSuccessProbe struct {
	Primitive string `json:"primitive"`
}

type FailabilityProbe struct {
	ProbeID                string                `json:"probe_id"`
	Property               string                `json:"property"`
	Executable             string                `json:"executable"`
	Argv                   []string              `json:"argv"`
	Observable             string                `json:"observable"`
	FailureCondition       string                `json:"failure_condition"`
	ProofRecordIDs         []string              `json:"proof_record_ids"`
	InvocationExecution    ProofState            `json:"invocation_execution"`
	FailureObservableBound bool                  `json:"failure_observable_bound"`
	StateBindingRequired   bool                  `json:"state_binding_required"`
	ObservedStateDigest    string                `json:"observed_state_digest"`
	ClaimStateDigest       string                `json:"claim_state_digest"`
	Direct                 *DirectProbe          `json:"direct,omitempty"`
	Comparison             *ComparisonProbe      `json:"comparison,omitempty"`
	Checksum               *ChecksumProbe        `json:"checksum,omitempty"`
	TestRun                *TestRunProbe         `json:"test_run,omitempty"`
	Pipeline               *PipelineProbe        `json:"pipeline,omitempty"`
	Wrapper                *WrapperProbe         `json:"wrapper,omitempty"`
	ConstantSuccess        *ConstantSuccessProbe `json:"constant_success,omitempty"`
}

type FailabilityAnalysis struct {
	Resolution     FailabilityResolution `json:"resolution"`
	ReasonCode     FailabilityReasonCode `json:"reason_code"`
	Executable     string                `json:"executable"`
	Argv           []string              `json:"argv"`
	ProofRecordIDs []string              `json:"proof_record_ids"`
	Finding        FailabilityFinding    `json:"finding"`
}

func AnalyzeFailability(probe FailabilityProbe) (FailabilityAnalysis, error) {
	if err := validateFailabilityProbe(probe); err != nil {
		return FailabilityAnalysis{}, err
	}
	resolution, reason := resolveFailability(probe)
	operandsDigest, err := digestJSON(probe.Argv)
	if err != nil {
		return FailabilityAnalysis{}, err
	}
	analysis := FailabilityAnalysis{
		Resolution: resolution, ReasonCode: reason, Executable: probe.Executable, Argv: append([]string(nil), probe.Argv...),
		ProofRecordIDs: append([]string(nil), probe.ProofRecordIDs...),
		Finding: FailabilityFinding{
			Property: probe.Property, OperandsDigest: operandsDigest, Observable: probe.Observable,
			FailureCondition: probe.FailureCondition, Failable: resolution == FailabilityProven, Reason: string(reason),
		},
	}
	return analysis, analysis.Validate()
}

func (analysis FailabilityAnalysis) Validate() error {
	if !slices.Contains([]FailabilityResolution{FailabilityProven, FailabilityNonFailable, FailabilityUnresolved}, analysis.Resolution) ||
		!validFailabilityReason(analysis.ReasonCode) || len(analysis.ProofRecordIDs) == 0 ||
		missing(analysis.Executable) || len(analysis.Argv) == 0 || analysis.Argv[0] != analysis.Executable ||
		missing(analysis.Finding.Property, analysis.Finding.Observable, analysis.Finding.FailureCondition, analysis.Finding.Reason) ||
		!validDigest(analysis.Finding.OperandsDigest) || analysis.Finding.Reason != string(analysis.ReasonCode) ||
		analysis.Finding.Failable != (analysis.Resolution == FailabilityProven) {
		return errors.New("failability analysis is incomplete or contradictory")
	}
	if err := validateSortedUnique("failability proof record IDs", analysis.ProofRecordIDs, 1); err != nil {
		return err
	}
	for _, argument := range analysis.Argv {
		if missing(argument) {
			return errors.New("failability argv contains an empty argument")
		}
	}
	operandsDigest, err := digestJSON(analysis.Argv)
	if err != nil {
		return err
	}
	if analysis.Finding.OperandsDigest != operandsDigest {
		return errors.New("failability argv differs from its operand digest")
	}
	if analysis.Resolution == FailabilityProven && analysis.ReasonCode != FailabilityReasonFailable {
		return errors.New("failable resolution requires the failable reason")
	}
	if analysis.Resolution != FailabilityProven && analysis.ReasonCode == FailabilityReasonFailable {
		return errors.New("non-failable or unresolved resolution cannot claim a failure path")
	}
	return nil
}

func validateFailabilityProbe(probe FailabilityProbe) error {
	if missing(probe.ProbeID, probe.Property, probe.Executable, probe.Observable, probe.FailureCondition) || len(probe.Argv) == 0 ||
		probe.Argv[0] != probe.Executable || !slices.Contains([]ProofState{ProofProven, ProofDisproven, ProofUnresolved}, probe.InvocationExecution) {
		return errors.New("failability probe identity, argv, or execution proof is invalid")
	}
	if err := validateSortedUnique("failability probe proof record IDs", probe.ProofRecordIDs, 1); err != nil {
		return err
	}
	variants := 0
	for _, present := range []bool{probe.Direct != nil, probe.Comparison != nil, probe.Checksum != nil, probe.TestRun != nil, probe.Pipeline != nil, probe.Wrapper != nil, probe.ConstantSuccess != nil} {
		if present {
			variants++
		}
	}
	if variants != 1 {
		return errors.New("failability probe requires exactly one semantic variant")
	}
	if (probe.ObservedStateDigest == "") != (probe.ClaimStateDigest == "") ||
		(probe.ObservedStateDigest != "" && (!validDigest(probe.ObservedStateDigest) || !validDigest(probe.ClaimStateDigest))) {
		return errors.New("failability probe state binding must be absent or a complete digest pair")
	}
	if probe.Comparison != nil {
		if missing(probe.Comparison.LeftSubjectAlias, probe.Comparison.RightSubjectAlias) ||
			!validDigest(probe.Comparison.LeftStateIdentity) || !validDigest(probe.Comparison.RightStateIdentity) {
			return errors.New("comparison probe requires two subject and state identities")
		}
	}
	if probe.Checksum != nil && (!slices.Contains([]ChecksumMode{ChecksumGenerate, ChecksumVerify}, probe.Checksum.Mode) || (probe.Checksum.Mode == ChecksumVerify && !probe.Checksum.ManifestBound)) {
		return errors.New("checksum verification requires a bound manifest")
	}
	if probe.TestRun != nil {
		run := probe.TestRun
		if !slices.Contains([]ProofState{ProofProven, ProofDisproven, ProofUnresolved}, run.CacheUsed) ||
			(run.CountsObserved && (run.Discovered < 0 || run.Executed < 0 || run.Skipped < 0 || run.Executed+run.Skipped > run.Discovered)) ||
			(!run.CountsObserved && (run.Discovered != 0 || run.Executed != 0 || run.Skipped != 0)) ||
			(run.CacheStateDigest != "" && !validDigest(run.CacheStateDigest)) ||
			(run.CacheUsed != ProofProven && run.CacheStateDigest != "") {
			return errors.New("test-run probe counts or cache evidence are invalid")
		}
	}
	if probe.Pipeline != nil {
		pipeline := probe.Pipeline
		if len(pipeline.Stages) < 2 || pipeline.DecisiveStageIndex < 0 || pipeline.DecisiveStageIndex >= len(pipeline.Stages) ||
			!slices.Contains([]PipelineStatusPolicy{PipelineLastStage, PipelinePipefail, PipelineUnknown}, pipeline.StatusPolicy) {
			return errors.New("pipeline probe is incomplete")
		}
		seen := make(map[string]struct{}, len(pipeline.Stages))
		for _, stage := range pipeline.Stages {
			if missing(stage.StageID) {
				return errors.New("pipeline stages require unique identities")
			}
			if _, duplicate := seen[stage.StageID]; duplicate {
				return errors.New("pipeline stages require unique identities")
			}
			seen[stage.StageID] = struct{}{}
		}
	}
	if probe.Wrapper != nil {
		wrapper := probe.Wrapper
		if !slices.Contains([]ProofState{ProofProven, ProofDisproven, ProofUnresolved}, wrapper.InnerExecution) ||
			!slices.Contains([]ProofState{ProofProven, ProofDisproven, ProofUnresolved}, wrapper.InnerStatusPropagated) ||
			!slices.Contains([]FailabilityResolution{FailabilityProven, FailabilityNonFailable, FailabilityUnresolved}, wrapper.InnerResolution) ||
			!validFailabilityReason(wrapper.InnerReason) {
			return errors.New("wrapper probe inner evidence is invalid")
		}
		if (wrapper.InnerResolution == FailabilityProven) != (wrapper.InnerReason == FailabilityReasonFailable) ||
			(wrapper.InnerExecution != ProofProven && wrapper.InnerStatusPropagated == ProofProven) {
			return errors.New("wrapper probe inner resolution or propagation is contradictory")
		}
	}
	if probe.ConstantSuccess != nil && missing(probe.ConstantSuccess.Primitive) {
		return errors.New("constant-success probe requires its primitive")
	}
	return nil
}

func resolveFailability(probe FailabilityProbe) (FailabilityResolution, FailabilityReasonCode) {
	if probe.InvocationExecution == ProofDisproven {
		return FailabilityNonFailable, FailabilityReasonInvocationAbsent
	}
	if probe.InvocationExecution == ProofUnresolved {
		return FailabilityUnresolved, FailabilityReasonInvocationUnresolved
	}
	if probe.StateBindingRequired && probe.ObservedStateDigest == "" {
		return FailabilityUnresolved, FailabilityReasonStateUnresolved
	}
	if probe.ObservedStateDigest != "" && probe.ObservedStateDigest != probe.ClaimStateDigest {
		return FailabilityUnresolved, FailabilityReasonStateStale
	}
	if probe.ConstantSuccess != nil {
		return FailabilityNonFailable, FailabilityReasonConstantSuccess
	}
	if comparison := probe.Comparison; comparison != nil {
		if filepath.Clean(comparison.LeftSubjectAlias) == filepath.Clean(comparison.RightSubjectAlias) {
			return FailabilityNonFailable, FailabilityReasonSelfComparison
		}
		if comparison.LeftStateIdentity == comparison.RightStateIdentity {
			return FailabilityNonFailable, FailabilityReasonSameStateComparison
		}
	}
	if checksum := probe.Checksum; checksum != nil && checksum.Mode == ChecksumGenerate {
		return FailabilityNonFailable, FailabilityReasonUncheckedDigest
	}
	if pipeline := probe.Pipeline; pipeline != nil {
		if pipeline.StatusPolicy == PipelineUnknown {
			return FailabilityUnresolved, FailabilityReasonPipelineUnresolved
		}
		if pipeline.StatusPolicy == PipelineLastStage && pipeline.DecisiveStageIndex != len(pipeline.Stages)-1 {
			return FailabilityNonFailable, FailabilityReasonNeutralizedPipeline
		}
	}
	if run := probe.TestRun; run != nil {
		if run.CacheUsed == ProofUnresolved {
			return FailabilityUnresolved, FailabilityReasonCacheUnresolved
		}
		if run.CacheUsed == ProofProven && (run.CacheStateDigest == "" || run.CacheStateDigest != probe.ClaimStateDigest) {
			return FailabilityUnresolved, FailabilityReasonCacheUnresolved
		}
		if run.CountsObserved && run.Executed == 0 {
			if run.Discovered > 0 && run.Skipped == run.Discovered {
				return FailabilityNonFailable, FailabilityReasonSkippedTests
			}
			return FailabilityNonFailable, FailabilityReasonZeroTests
		}
	}
	if wrapper := probe.Wrapper; wrapper != nil {
		if wrapper.InnerExecution == ProofDisproven {
			return FailabilityNonFailable, FailabilityReasonWrapperAbsent
		}
		if wrapper.InnerExecution == ProofUnresolved || wrapper.InnerStatusPropagated == ProofUnresolved || wrapper.InnerResolution == FailabilityUnresolved {
			return FailabilityUnresolved, FailabilityReasonWrapperUnresolved
		}
		if wrapper.InnerStatusPropagated == ProofDisproven {
			return FailabilityNonFailable, FailabilityReasonWrapperNeutralized
		}
		if wrapper.InnerResolution == FailabilityNonFailable {
			return FailabilityNonFailable, wrapper.InnerReason
		}
	}
	if !probe.FailureObservableBound {
		return FailabilityUnresolved, FailabilityReasonObservableUnbound
	}
	return FailabilityProven, FailabilityReasonFailable
}

func validFailabilityReason(reason FailabilityReasonCode) bool {
	return slices.Contains([]FailabilityReasonCode{
		FailabilityReasonFailable, FailabilityReasonInvocationAbsent, FailabilityReasonInvocationUnresolved,
		FailabilityReasonStateStale, FailabilityReasonStateUnresolved, FailabilityReasonSelfComparison,
		FailabilityReasonSameStateComparison, FailabilityReasonConstantSuccess, FailabilityReasonNeutralizedPipeline,
		FailabilityReasonPipelineUnresolved, FailabilityReasonUncheckedDigest, FailabilityReasonZeroTests,
		FailabilityReasonSkippedTests, FailabilityReasonCacheUnresolved, FailabilityReasonWrapperAbsent,
		FailabilityReasonWrapperUnresolved, FailabilityReasonWrapperNeutralized, FailabilityReasonObservableUnbound,
	}, reason)
}
