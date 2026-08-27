package lineage

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const OfflineProofVersion = "evalwitness.verification-lineage-offline-proof.v1"

type OfflinePositiveProof struct {
	CaseID           string        `json:"case_id"`
	SourceDigest     string        `json:"source_digest"`
	WitnessDigest    string        `json:"witness_digest"`
	PairingDigest    string        `json:"pairing_digest"`
	CandidateDigest  string        `json:"candidate_digest"`
	AssessmentDigest string        `json:"assessment_digest"`
	AuditDigest      string        `json:"audit_digest"`
	BOMDigest        string        `json:"bom_digest"`
	TerminalState    TerminalState `json:"terminal_state"`
	LayerSurvival    []bool        `json:"layer_survival"`
	RequiredFields   []string      `json:"required_fields"`
	DecisiveChannels []string      `json:"decisive_channels"`
}

type OfflineCounterexampleProof struct {
	CaseID            string                `json:"case_id"`
	VectorID          string                `json:"vector_id"`
	NativeInputDigest string                `json:"native_input_digest"`
	Failability       FailabilityAnalysis   `json:"failability"`
	Classification    ClassifiedLineageUnit `json:"classification"`
}

type OfflineProofClaimBoundary struct {
	EvidenceRole      DataRole `json:"evidence_role"`
	EmpiricalAuditRun bool     `json:"empirical_audit_run"`
	ResearchRelease   bool     `json:"research_release"`
	ProviderCalls     int      `json:"provider_calls"`
	AgentLaunches     int      `json:"agent_launches"`
	SupportedClaim    string   `json:"supported_claim"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

type VerificationLineageOfflineProof struct {
	Version                 string                     `json:"version"`
	ProofID                 string                     `json:"proof_id"`
	PlanDigest              string                     `json:"plan_digest"`
	ParserLockDigest        string                     `json:"parser_lock_digest"`
	CapabilityMatrixDigest  string                     `json:"capability_matrix_digest"`
	SourceReadinessDigest   string                     `json:"source_readiness_digest"`
	HoldoutReadinessDigest  string                     `json:"holdout_readiness_digest"`
	CorpusFeasibilityDigest string                     `json:"corpus_feasibility_digest"`
	Positive                OfflinePositiveProof       `json:"positive"`
	Counterexample          OfflineCounterexampleProof `json:"counterexample"`
	ClaimBoundary           OfflineProofClaimBoundary  `json:"claim_boundary"`
	Digest                  string                     `json:"digest"`
}

func BuildVerificationLineageOfflineProof(repositoryRoot string) (VerificationLineageOfflineProof, error) {
	proof, err := buildOfflineProofUnchecked(repositoryRoot)
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	return proof, proof.Validate(repositoryRoot)
}

func (proof VerificationLineageOfflineProof) Validate(repositoryRoot string) error {
	if proof.Version != OfflineProofVersion || proof.ProofID != "task_069-offline-proof-v1" || proof.PlanDigest != LockedPlanDigest ||
		!validDigest(proof.ParserLockDigest) || !validDigest(proof.CapabilityMatrixDigest) || !validDigest(proof.SourceReadinessDigest) ||
		!validDigest(proof.HoldoutReadinessDigest) || !validDigest(proof.CorpusFeasibilityDigest) || !validDigest(proof.Digest) {
		return errors.New("verification-lineage offline-proof identity is invalid")
	}
	if proof.Positive.CaseID != "stdout_success" || proof.Positive.TerminalState != StateDirectVerificationInvocation ||
		!slices.Equal(proof.Positive.LayerSurvival, []bool{true, true, true, true, true}) ||
		!slices.Equal(proof.Positive.RequiredFields, []string{"call_id", "exit_status", "stderr", "stdout"}) ||
		!slices.Equal(proof.Positive.DecisiveChannels, []string{"exit_status", "stderr", "stdout"}) {
		return errors.New("verification-lineage offline positive proof is incomplete")
	}
	for _, digest := range []string{proof.Positive.SourceDigest, proof.Positive.WitnessDigest, proof.Positive.PairingDigest, proof.Positive.CandidateDigest, proof.Positive.AssessmentDigest, proof.Positive.AuditDigest, proof.Positive.BOMDigest} {
		if !validDigest(digest) {
			return errors.New("verification-lineage offline positive proof contains an invalid lineage digest")
		}
	}
	if proof.Counterexample.CaseID != "same_path_comparison_false_target" ||
		proof.Counterexample.VectorID != "codex_rollout_jsonl/same_path_comparison_false_target" || !validDigest(proof.Counterexample.NativeInputDigest) ||
		proof.Counterexample.Failability.Resolution != FailabilityNonFailable || proof.Counterexample.Failability.ReasonCode != FailabilityReasonSelfComparison ||
		proof.Counterexample.Classification.TerminalState != StateNonFailableVerification ||
		!slices.Equal(proof.Counterexample.Classification.LayerSurvival, []bool{true, true, true, false, false}) {
		return errors.New("verification-lineage offline counterexample proof is invalid")
	}
	if err := proof.Counterexample.Failability.Validate(); err != nil {
		return err
	}
	if proof.ClaimBoundary.EvidenceRole != RoleAdapterDevelopment || proof.ClaimBoundary.EmpiricalAuditRun || proof.ClaimBoundary.ResearchRelease ||
		proof.ClaimBoundary.ProviderCalls != 0 || proof.ClaimBoundary.AgentLaunches != 0 || proof.ClaimBoundary.SupportedClaim == "" ||
		!slices.Equal(proof.ClaimBoundary.UnsupportedClaims, []string{"empirical lineage survival", "format transfer", "population prevalence", "provider quality"}) {
		return errors.New("verification-lineage offline-proof claim boundary is invalid")
	}
	expected, err := buildOfflineProofUnchecked(repositoryRoot)
	if err != nil {
		return err
	}
	actualWithoutDigest := proof
	actualWithoutDigest.Digest = ""
	expectedWithoutDigest := expected
	expectedWithoutDigest.Digest = ""
	if !reflect.DeepEqual(actualWithoutDigest, expectedWithoutDigest) || proof.Digest != expected.Digest {
		return errors.New("verification-lineage offline proof differs from sealed development evidence")
	}
	return nil
}

func buildOfflineProofUnchecked(repositoryRoot string) (VerificationLineageOfflineProof, error) {
	parserLock, err := BuildVerificationLineageParserLock(repositoryRoot)
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	capabilityMatrix, err := BuildVerificationLineageCapabilityMatrix()
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	sourceReadiness, err := BuildVerificationLineageSourceReadinessAudit(repositoryRoot)
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	holdoutReadiness, err := BuildVerificationLineageHoldoutReadinessAudit(repositoryRoot)
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	feasibility, err := BuildVerificationLineageCorpusFeasibilityDecision(repositoryRoot)
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	positive, err := buildOfflinePositiveProof(capabilityMatrix, holdoutReadiness.Digest, nil)
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	counterexample, err := buildOfflineCounterexampleProof()
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	proof := VerificationLineageOfflineProof{
		Version: OfflineProofVersion, ProofID: "task_069-offline-proof-v1", PlanDigest: LockedPlanDigest,
		ParserLockDigest: parserLock.Digest, CapabilityMatrixDigest: capabilityMatrix.Digest, SourceReadinessDigest: sourceReadiness.Digest,
		HoldoutReadinessDigest: holdoutReadiness.Digest, CorpusFeasibilityDigest: feasibility.Digest,
		Positive: positive, Counterexample: counterexample,
		ClaimBoundary: OfflineProofClaimBoundary{
			EvidenceRole: RoleAdapterDevelopment, ProviderCalls: 0, AgentLaunches: 0,
			SupportedClaim:    "one sealed positive traverses the five-layer lineage and one sealed same-path comparison is rejected as non-failable entirely offline",
			UnsupportedClaims: []string{"empirical lineage survival", "format transfer", "population prevalence", "provider quality"},
		},
	}
	proof.Digest, err = offlineProofDigest(proof)
	if err != nil {
		return VerificationLineageOfflineProof{}, err
	}
	return proof, nil
}

type offlinePositiveArtifacts struct {
	Source     VerificationLineageSource
	Witness    ExecutionWitness
	Candidate  LineageCandidate
	Assessment LineageAssessment
	Capability TraceCapabilityVector
	Audit      LineageAudit
	BOM        VerificationEvidenceBOM
}

func buildOfflinePositiveProof(matrix VerificationLineageCapabilityMatrix, holdoutReadinessDigest string, artifacts *offlinePositiveArtifacts) (OfflinePositiveProof, error) {
	fixtureSet, err := buildSyntheticWitnessFixtureSet(syntheticFixtureCommandSpecs())
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	fixtureIndex := slices.IndexFunc(fixtureSet.Fixtures, func(fixture SyntheticWitnessFixture) bool { return fixture.CaseID == "stdout_success" })
	if fixtureIndex < 0 {
		return OfflinePositiveProof{}, errors.New("stdout-success witness fixture is absent")
	}
	specIndex := slices.IndexFunc(syntheticFixtureCommandSpecs(), func(spec SyntheticFixtureCommandSpec) bool { return spec.CaseID == "stdout_success" })
	if specIndex < 0 {
		return OfflinePositiveProof{}, errors.New("stdout-success command contract is absent")
	}
	witness := fixtureSet.Fixtures[fixtureIndex].Witness
	specification := syntheticFixtureCommandSpecs()[specIndex]
	raw, err := offlineCodexNativeRaw(witness, specification)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	source, witness, err := bindOfflineNativeSource(raw, witness)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	pairing, err := PairExecutionWitness(source, witness, raw, WitnessPairingPolicy{})
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	required := []string{"call_id", "exit_status", "stderr", "stdout"}
	channels := []string{"exit_status", "stderr", "stdout"}
	records := []string{pairing.CommandEventID, pairing.ToolCallEventID, pairing.ToolResultEventID}
	sort.Strings(records)
	retainedDigest, err := digestJSON(struct {
		CanonicalDigest string   `json:"canonical_digest"`
		RecordIDs       []string `json:"record_ids"`
	}{source.CanonicalTrajectoryDigest, records})
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	requestFingerprint, err := digestJSON(struct {
		ClaimID        string `json:"claim_id"`
		RetainedDigest string `json:"retained_digest"`
	}{"claim-fixed-fixture-exits-zero", retainedDigest})
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	requestLineageDigest, err := digestJSON(struct {
		TaskGroupID string `json:"task_group_id"`
		Witness     string `json:"witness"`
	}{source.Header.TaskGroupID, witness.Header.Digest})
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	candidate := LineageCandidate{
		Header: ArtifactHeader{
			SchemaVersion: CandidateSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "offline-candidate-stdout-success", TaskID: "TASK-069", TaskGroupID: source.Header.TaskGroupID,
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest,
			Parents: []ParentRef{parentFromArtifact("source", source.Header), parentFromArtifact("witness", witness.Header)},
		},
		CandidateID: "offline-candidate-stdout-success", ClaimID: "claim-fixed-fixture-exits-zero", InvocationID: witness.InvocationID,
		ParserIdentityDigest: matrix.Digest, RequestFingerprint: requestFingerprint, CanonicalRequestLineageDigest: requestLineageDigest,
		Alignment: AlignmentEvidence{CallIDMatched: true, ParentEdgesMatched: true, CommandOperandsDigest: witness.CommandOperandsDigest, TemporalWindowMillis: witness.EndedAt.Sub(witness.StartedAt).Milliseconds()},
	}
	layers := []struct {
		name    string
		digest  string
		records []string
	}{
		{"runtime_witness", witness.Header.Digest, []string{witness.InvocationID}},
		{"native_export", source.RawRecordDigest, records},
		{"canonical_graph", source.CanonicalTrajectoryDigest, records},
		{"retained_bundle", retainedDigest, records},
		{"verifier_request", requestFingerprint, []string{witness.InvocationID}},
	}
	for _, layer := range layers {
		candidate.Layers = append(candidate.Layers, LayerBinding{
			Layer: layer.name, ObjectDigest: layer.digest, RecordIDs: append([]string(nil), layer.records...),
			RequiredFields: append([]string(nil), required...), DecisiveChannels: append([]string(nil), channels...),
			StructuredPresence: true, SemanticSufficiency: true,
		})
	}
	candidate.Header.Digest, err = artifactDigest(candidate)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	if err := candidate.Validate(); err != nil {
		return OfflinePositiveProof{}, err
	}
	failability, err := AnalyzeFailability(FailabilityProbe{
		ProbeID: "offline-probe-stdout-success", Property: "fixed fixture exits zero", Executable: witness.Argv[0], Argv: append([]string(nil), witness.Argv...),
		Observable: "exit status and separated subprocess streams", FailureCondition: specification.FailureCondition,
		ProofRecordIDs: []string{"proof-command", "proof-result"}, InvocationExecution: ProofProven, FailureObservableBound: true,
		StateBindingRequired: true, ObservedStateDigest: witness.RepositoryStateDigest, ClaimStateDigest: witness.RepositoryStateDigest,
		Direct: &DirectProbe{},
	})
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	evaluatedAt := witness.EndedAt.UTC()
	assessment := LineageAssessment{
		Header: ArtifactHeader{
			SchemaVersion: AssessmentSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "offline-assessment-stdout-success", TaskID: "TASK-069", TaskGroupID: source.Header.TaskGroupID,
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest,
			Parents: []ParentRef{parentFromArtifact("candidate", candidate.Header), parentFromArtifact("witness", witness.Header)},
		},
		CandidateID: candidate.CandidateID, TerminalState: StateDirectVerificationInvocation, StateDisposition: DispositionEligible,
		PrecedenceApplied: 10, ProofRecordIDs: append([]string(nil), failability.ProofRecordIDs...),
		ResultProvenance: []ProvenanceClass{ProvenanceExitStatus, ProvenanceStderr, ProvenanceStdout}, Failability: failability.Finding,
		Freshness:    FreshnessInterval{State: FreshnessCurrent, ObservedStateDigest: witness.RepositoryStateDigest, ValidFrom: evaluatedAt, EvaluatedAt: evaluatedAt},
		ReviewerMode: "deterministic_lineage_analysis",
	}
	assessment.Header.Digest, err = artifactDigest(assessment)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	if err := assessment.Validate(); err != nil {
		return OfflinePositiveProof{}, err
	}
	unit, err := classifyOfflineState("offline-unit-stdout-success", source.Header.TaskGroupID, source.ExportFormat, source.SourceSessionID, StateDirectVerificationInvocation)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	capabilityIndex := slices.IndexFunc(matrix.Vectors, func(vector TraceCapabilityVector) bool { return vector.Format == "codex_rollout_jsonl" })
	if capabilityIndex < 0 {
		return OfflinePositiveProof{}, errors.New("codex capability vector is absent")
	}
	ledgerDigest, err := digestJSON(unit)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	auditHeader := ArtifactHeader{
		SchemaVersion: AuditSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
		ObjectID: "offline-audit-stdout-success", TaskID: "TASK-069", TaskGroupID: "study", DataRole: RoleAdapterDevelopment,
		PlanDigest: LockedPlanDigest,
		Parents: []ParentRef{
			{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-verification-lineage-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest},
			parentFromArtifact("assessment", assessment.Header), parentFromArtifact("capability", matrix.Vectors[capabilityIndex].Header),
		},
	}
	audit, err := BuildLineageAudit(auditHeader, "offline-audit-stdout-success", ledgerDigest, holdoutReadinessDigest, []string{"development_fixture_only"}, []ClassifiedLineageUnit{unit})
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	target, err := BuildTransformationTargetBinding("offline-target-stdout-success", source.Header.TaskGroupID, source.CanonicalTrajectoryDigest, retainedDigest, required, channels)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	bom, err := BuildVerificationEvidenceBOM("offline-bom-stdout-success", source, witness, pairing, candidate, assessment, audit, failability, target, required, channels)
	if err != nil {
		return OfflinePositiveProof{}, err
	}
	proof := OfflinePositiveProof{
		CaseID: "stdout_success", SourceDigest: source.Header.Digest, WitnessDigest: witness.Header.Digest, PairingDigest: pairing.Digest,
		CandidateDigest: candidate.Header.Digest, AssessmentDigest: assessment.Header.Digest, AuditDigest: audit.Header.Digest, BOMDigest: bom.Header.Digest,
		TerminalState: unit.TerminalState, LayerSurvival: append([]bool(nil), unit.LayerSurvival...), RequiredFields: required, DecisiveChannels: channels,
	}
	if artifacts != nil {
		*artifacts = offlinePositiveArtifacts{
			Source: source, Witness: witness, Candidate: candidate, Assessment: assessment,
			Capability: matrix.Vectors[capabilityIndex], Audit: audit, BOM: bom,
		}
	}
	return proof, nil
}

func buildOfflineCounterexampleProof() (OfflineCounterexampleProof, error) {
	golden, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		return OfflineCounterexampleProof{}, err
	}
	index := slices.IndexFunc(golden.Vectors, func(vector VerificationLineageGoldenVector) bool {
		return vector.VectorID == "codex_rollout_jsonl/same_path_comparison_false_target"
	})
	if index < 0 {
		return OfflineCounterexampleProof{}, errors.New("same-path counterexample vector is absent")
	}
	vector := golden.Vectors[index]
	stateDigest, err := digestJSON("same-file-state")
	if err != nil {
		return OfflineCounterexampleProof{}, err
	}
	failability, err := AnalyzeFailability(FailabilityProbe{
		ProbeID: "offline-probe-same-path", Property: "legacy file is unchanged", Executable: "cmp",
		Argv: []string{"cmp", "-s", "repository/file", "repository/file"}, Observable: "comparison exit status",
		FailureCondition: "nonzero exit when compared files differ", ProofRecordIDs: []string{"proof-command", "proof-operands"},
		InvocationExecution: ProofProven, FailureObservableBound: true, StateBindingRequired: true,
		ObservedStateDigest: stateDigest, ClaimStateDigest: stateDigest,
		Comparison: &ComparisonProbe{LeftSubjectAlias: "repository/file", RightSubjectAlias: "repository/file", LeftStateIdentity: stateDigest, RightStateIdentity: stateDigest},
	})
	if err != nil {
		return OfflineCounterexampleProof{}, err
	}
	unit, err := classifyOfflineState("offline-unit-same-path", "offline-same-path", "codex_rollout_jsonl", "offline-counterexample-session", StateNonFailableVerification)
	if err != nil {
		return OfflineCounterexampleProof{}, err
	}
	return OfflineCounterexampleProof{
		CaseID: vector.CaseID, VectorID: vector.VectorID, NativeInputDigest: vector.NativeInputDigest,
		Failability: failability, Classification: unit,
	}, nil
}

func offlineCodexNativeRaw(witness ExecutionWitness, specification SyntheticFixtureCommandSpec) ([]byte, error) {
	records := []any{
		map[string]any{"timestamp": witness.StartedAt.UTC().Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]any{"type": "exec_command_begin", "call_id": witness.InvocationID, "command": witness.Argv}},
		map[string]any{"timestamp": witness.EndedAt.UTC().Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]any{"type": "exec_command_end", "call_id": witness.InvocationID, "status": "completed", "exit_code": specification.ExitStatus, "stdout": string(specification.Stdout), "stderr": string(specification.Stderr)}},
	}
	lines := make([]string, len(records))
	for index, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		lines[index] = string(encoded)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func bindOfflineNativeSource(raw []byte, witness ExecutionWitness) (VerificationLineageSource, ExecutionWitness, error) {
	imported, err := preprocess.ImportTraceBytes(raw, preprocess.DefaultTraceImportOptions())
	if err != nil {
		return VerificationLineageSource{}, ExecutionWitness{}, err
	}
	accountingDigest, err := digestJSON(imported.Trajectory.Report)
	if err != nil {
		return VerificationLineageSource{}, ExecutionWitness{}, err
	}
	source := VerificationLineageSource{
		Header: ArtifactHeader{
			SchemaVersion: SourceSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "offline-source-stdout-success", TaskID: "TASK-069", TaskGroupID: witness.Header.TaskGroupID,
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest,
			Parents: []ParentRef{{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-verification-lineage-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest}},
		},
		SourceClass: "checked_in_controls", AgentEcosystem: "codex_fixture", RuntimeIdentityClass: "fixed_local_subprocess",
		ProviderMetadata: "none", ExportFormat: string(imported.Trajectory.SourceFormat), ExportVersion: "fixture-v1", CaptureMode: CapturePaired,
		SourceSessionID: "offline-proof-session", LineageID: "offline-proof-lineage", NearDuplicateID: "offline-proof-near-duplicate",
		RepositoryID: "evalwitness-repository", RepositoryAlias: witness.WorkingDirectoryAlias, TaskAlias: "stdout-success",
		License: "MIT", RedistributionPermission: "public_mit", PrivacyClass: "synthetic_public", RedactionPolicy: "none",
		RawRecordCount: imported.Trajectory.Report.SourceRecords, RawRecordDigest: digestBytes(raw),
		CanonicalTrajectoryDigest: imported.Trajectory.Digest, FieldAccountingDigest: accountingDigest,
	}
	source.Header.Digest, err = artifactDigest(source)
	if err != nil {
		return VerificationLineageSource{}, ExecutionWitness{}, err
	}
	if err := source.Validate(); err != nil {
		return VerificationLineageSource{}, ExecutionWitness{}, err
	}
	witness.Header.ObjectID = "offline-witness-stdout-success"
	witness.Header.Parents = []ParentRef{parentFromArtifact("source", source.Header)}
	witness.Header.Digest = ""
	witness.Header.Digest, err = artifactDigest(witness)
	if err != nil {
		return VerificationLineageSource{}, ExecutionWitness{}, err
	}
	if err := witness.Validate(); err != nil {
		return VerificationLineageSource{}, ExecutionWitness{}, err
	}
	return source, witness, nil
}

func classifyOfflineState(unitID, taskGroupID, format, sessionID string, selected TerminalState) (ClassifiedLineageUnit, error) {
	contract := terminalStateContract()
	selectedIndex := slices.IndexFunc(contract, func(rule TerminalStateRule) bool { return rule.State == selected })
	if selectedIndex < 0 {
		return ClassifiedLineageUnit{}, fmt.Errorf("unsupported offline terminal state %q", selected)
	}
	findings := make([]TerminalFinding, len(contract))
	for index, rule := range contract {
		finding := TerminalFinding{State: rule.State, Status: FindingNotEvaluated}
		if index < selectedIndex {
			finding.Status = FindingDisproven
			finding.ProofRecordIDs = []string{"proof-disproven-" + string(rule.State)}
		} else if index == selectedIndex {
			finding.Status = FindingProven
			finding.ProofRecordIDs = []string{"proof-proven-" + string(rule.State)}
		}
		findings[index] = finding
	}
	return ClassifyEarliestLoss(ClassificationInput{
		UnitID: unitID, TaskGroupID: taskGroupID, Format: format, SourceSessionID: sessionID,
		CandidateEvidenceEvents: 1, DirectInvocationObserved: stateRequiresObservedInvocation(selected), Findings: findings,
	})
}

func offlineProofDigest(proof VerificationLineageOfflineProof) (string, error) {
	proof.Digest = ""
	return digestJSON(proof)
}
