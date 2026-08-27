package lineage

import (
	"slices"
	"strings"
	"testing"
)

func TestFailabilityAnalysisClosesNonTrivialityFailureModes(t *testing.T) {
	current := strings.Repeat("1", 64)
	other := strings.Repeat("2", 64)
	tests := []struct {
		name       string
		mutate     func(*FailabilityProbe)
		resolution FailabilityResolution
		reason     FailabilityReasonCode
	}{
		{name: "direct failable", mutate: func(probe *FailabilityProbe) { probe.Direct = &DirectProbe{} }, resolution: FailabilityProven, reason: FailabilityReasonFailable},
		{name: "self comparison", mutate: func(probe *FailabilityProbe) {
			probe.Comparison = &ComparisonProbe{LeftSubjectAlias: "artifact.bin", RightSubjectAlias: "./artifact.bin", LeftStateIdentity: current, RightStateIdentity: other}
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonSelfComparison},
		{name: "same state comparison", mutate: func(probe *FailabilityProbe) {
			probe.Comparison = &ComparisonProbe{LeftSubjectAlias: "before.bin", RightSubjectAlias: "after.bin", LeftStateIdentity: current, RightStateIdentity: current}
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonSameStateComparison},
		{name: "distinct comparison", mutate: func(probe *FailabilityProbe) {
			probe.Comparison = &ComparisonProbe{LeftSubjectAlias: "before.bin", RightSubjectAlias: "after.bin", LeftStateIdentity: current, RightStateIdentity: other}
		}, resolution: FailabilityProven, reason: FailabilityReasonFailable},
		{name: "stale state", mutate: func(probe *FailabilityProbe) {
			probe.Direct = &DirectProbe{}
			probe.StateBindingRequired = true
			probe.ObservedStateDigest = other
			probe.ClaimStateDigest = current
		}, resolution: FailabilityUnresolved, reason: FailabilityReasonStateStale},
		{name: "required state unresolved", mutate: func(probe *FailabilityProbe) {
			probe.Direct = &DirectProbe{}
			probe.StateBindingRequired = true
		}, resolution: FailabilityUnresolved, reason: FailabilityReasonStateUnresolved},
		{name: "constant success", mutate: func(probe *FailabilityProbe) {
			probe.ConstantSuccess = &ConstantSuccessProbe{Primitive: "true"}
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonConstantSuccess},
		{name: "neutralized pipeline", mutate: func(probe *FailabilityProbe) {
			probe.Pipeline = &PipelineProbe{Stages: []PipelineStage{{StageID: "01-check"}, {StageID: "02-true", ConstantSuccess: true}}, DecisiveStageIndex: 0, StatusPolicy: PipelineLastStage}
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonNeutralizedPipeline},
		{name: "unknown pipeline status", mutate: func(probe *FailabilityProbe) {
			probe.Pipeline = &PipelineProbe{Stages: []PipelineStage{{StageID: "01-check"}, {StageID: "02-consumer"}}, DecisiveStageIndex: 0, StatusPolicy: PipelineUnknown}
		}, resolution: FailabilityUnresolved, reason: FailabilityReasonPipelineUnresolved},
		{name: "pipefail preserves decisive stage", mutate: func(probe *FailabilityProbe) {
			probe.Pipeline = &PipelineProbe{Stages: []PipelineStage{{StageID: "check"}, {StageID: "consumer"}}, DecisiveStageIndex: 0, StatusPolicy: PipelinePipefail}
		}, resolution: FailabilityProven, reason: FailabilityReasonFailable},
		{name: "unchecked digest", mutate: func(probe *FailabilityProbe) {
			probe.Checksum = &ChecksumProbe{Mode: ChecksumGenerate}
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonUncheckedDigest},
		{name: "bound digest verification", mutate: func(probe *FailabilityProbe) {
			probe.Checksum = &ChecksumProbe{Mode: ChecksumVerify, ManifestBound: true}
		}, resolution: FailabilityProven, reason: FailabilityReasonFailable},
		{name: "zero tests", mutate: func(probe *FailabilityProbe) {
			probe.TestRun = &TestRunProbe{CountsObserved: true, CacheUsed: ProofDisproven}
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonZeroTests},
		{name: "all tests skipped", mutate: func(probe *FailabilityProbe) {
			probe.TestRun = &TestRunProbe{CountsObserved: true, Discovered: 4, Skipped: 4, CacheUsed: ProofDisproven}
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonSkippedTests},
		{name: "cached state unbound", mutate: func(probe *FailabilityProbe) {
			probe.TestRun = &TestRunProbe{CountsObserved: true, Discovered: 4, Executed: 4, CacheUsed: ProofProven}
			probe.ObservedStateDigest = current
			probe.ClaimStateDigest = current
		}, resolution: FailabilityUnresolved, reason: FailabilityReasonCacheUnresolved},
		{name: "cached state bound", mutate: func(probe *FailabilityProbe) {
			probe.TestRun = &TestRunProbe{CountsObserved: true, Discovered: 4, Executed: 4, CacheUsed: ProofProven, CacheStateDigest: current}
			probe.ObservedStateDigest = current
			probe.ClaimStateDigest = current
		}, resolution: FailabilityProven, reason: FailabilityReasonFailable},
		{name: "wrapper never executed", mutate: func(probe *FailabilityProbe) {
			probe.Wrapper = wrapperProbe(ProofDisproven, ProofUnresolved, FailabilityUnresolved, FailabilityReasonInvocationUnresolved)
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonWrapperAbsent},
		{name: "wrapper unresolved", mutate: func(probe *FailabilityProbe) {
			probe.Wrapper = wrapperProbe(ProofUnresolved, ProofUnresolved, FailabilityUnresolved, FailabilityReasonInvocationUnresolved)
		}, resolution: FailabilityUnresolved, reason: FailabilityReasonWrapperUnresolved},
		{name: "wrapper neutralizes status", mutate: func(probe *FailabilityProbe) {
			probe.Wrapper = wrapperProbe(ProofProven, ProofDisproven, FailabilityProven, FailabilityReasonFailable)
		}, resolution: FailabilityNonFailable, reason: FailabilityReasonWrapperNeutralized},
		{name: "wrapper propagates failable inner", mutate: func(probe *FailabilityProbe) {
			probe.Wrapper = wrapperProbe(ProofProven, ProofProven, FailabilityProven, FailabilityReasonFailable)
		}, resolution: FailabilityProven, reason: FailabilityReasonFailable},
		{name: "failure observable unbound", mutate: func(probe *FailabilityProbe) {
			probe.Direct = &DirectProbe{}
			probe.FailureObservableBound = false
		}, resolution: FailabilityUnresolved, reason: FailabilityReasonObservableUnbound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe := validFailabilityProbe()
			test.mutate(&probe)
			analysis, err := AnalyzeFailability(probe)
			if err != nil {
				t.Fatal(err)
			}
			if analysis.Resolution != test.resolution || analysis.ReasonCode != test.reason || analysis.Finding.Failable != (test.resolution == FailabilityProven) {
				t.Fatalf("analysis = %#v, want %s/%s", analysis, test.resolution, test.reason)
			}
			if !slices.Equal(analysis.ProofRecordIDs, probe.ProofRecordIDs) || !slices.Equal(analysis.Argv, probe.Argv) || analysis.Finding.OperandsDigest == "" {
				t.Fatal("analysis lost its proof or operand identity")
			}
		})
	}
}

func TestFailabilityAnalysisRejectsAmbiguousOrMalformedSemanticInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FailabilityProbe)
	}{
		{name: "no variant", mutate: func(probe *FailabilityProbe) {}},
		{name: "two variants", mutate: func(probe *FailabilityProbe) {
			probe.Direct = &DirectProbe{}
			probe.Checksum = &ChecksumProbe{Mode: ChecksumGenerate}
		}},
		{name: "argv executable mismatch", mutate: func(probe *FailabilityProbe) { probe.Direct = &DirectProbe{}; probe.Argv[0] = "other" }},
		{name: "partial state binding", mutate: func(probe *FailabilityProbe) {
			probe.Direct = &DirectProbe{}
			probe.ObservedStateDigest = strings.Repeat("1", 64)
		}},
		{name: "verify without manifest", mutate: func(probe *FailabilityProbe) { probe.Checksum = &ChecksumProbe{Mode: ChecksumVerify} }},
		{name: "invalid test counts", mutate: func(probe *FailabilityProbe) {
			probe.TestRun = &TestRunProbe{CountsObserved: true, Discovered: 1, Executed: 1, Skipped: 1, CacheUsed: ProofDisproven}
		}},
		{name: "invalid pipeline stage", mutate: func(probe *FailabilityProbe) {
			probe.Pipeline = &PipelineProbe{Stages: []PipelineStage{{StageID: "same"}, {StageID: "same"}}, StatusPolicy: PipelinePipefail}
		}},
		{name: "cache digest without cache", mutate: func(probe *FailabilityProbe) {
			probe.TestRun = &TestRunProbe{CacheUsed: ProofDisproven, CacheStateDigest: strings.Repeat("1", 64)}
		}},
		{name: "wrapper contradictory inner result", mutate: func(probe *FailabilityProbe) {
			probe.Wrapper = wrapperProbe(ProofProven, ProofProven, FailabilityNonFailable, FailabilityReasonFailable)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe := validFailabilityProbe()
			test.mutate(&probe)
			if _, err := AnalyzeFailability(probe); err == nil {
				t.Fatal("malformed failability probe was accepted")
			}
		})
	}
}

func TestFailabilityAnalysisRejectsPostClassificationTampering(t *testing.T) {
	probe := validFailabilityProbe()
	probe.Direct = &DirectProbe{}
	analysis, err := AnalyzeFailability(probe)
	if err != nil {
		t.Fatal(err)
	}
	analysis.Resolution = FailabilityNonFailable
	if err := analysis.Validate(); err == nil {
		t.Fatal("contradictory failability result was accepted")
	}
	analysis, err = AnalyzeFailability(probe)
	if err != nil {
		t.Fatal(err)
	}
	analysis.Argv[1] = "--different"
	if err := analysis.Validate(); err == nil {
		t.Fatal("argv substitution with a stale operand digest was accepted")
	}
}

func validFailabilityProbe() FailabilityProbe {
	return FailabilityProbe{
		ProbeID: "probe-1", Property: "repository passes the selected check", Executable: "check",
		Argv: []string{"check", "--workspace"}, Observable: "process exit status and structured result",
		FailureCondition: "non-zero exit or explicit failed result", ProofRecordIDs: []string{"proof-command", "proof-result"},
		InvocationExecution: ProofProven, FailureObservableBound: true,
	}
}

func wrapperProbe(execution, propagated ProofState, resolution FailabilityResolution, reason FailabilityReasonCode) *WrapperProbe {
	return &WrapperProbe{InnerExecution: execution, InnerStatusPropagated: propagated, InnerResolution: resolution, InnerReason: reason}
}
