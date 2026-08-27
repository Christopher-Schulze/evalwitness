package lineage

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBOMConstructionTraversesAndSealsTheCompleteEvidenceLineage(t *testing.T) {
	source, witness, pairing, candidate, assessment, audit, failability, target := completeBOMInputs(t)
	bom, err := BuildVerificationEvidenceBOM(
		"bom-complete-lineage", source, witness, pairing, candidate, assessment, audit, failability,
		target, []string{"call_id", "exit_status", "stderr", "stdout"}, []string{"exit_status", "stderr", "stdout"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateBOMLineage(bom, source, witness, pairing, candidate, assessment, audit, failability, target); err != nil {
		t.Fatal(err)
	}
	if bom.Evidence.Executable != witness.Argv[0] || len(bom.Evidence.Operands) != len(witness.Argv)-1 ||
		bom.Retention.NativeRecordDigest != source.RawRecordDigest || bom.Retention.CanonicalEventDigest != source.CanonicalTrajectoryDigest ||
		bom.RepositoryStateDigest != witness.RepositoryStateDigest || !bom.Accepted {
		t.Fatalf("constructed BOM lost execution or lineage identity: %#v", bom)
	}
}

func TestBOMConstructionRejectsLayerLossAndLineageSubstitution(t *testing.T) {
	source, witness, pairing, candidate, assessment, audit, failability, target := completeBOMInputs(t)
	tests := []struct {
		name   string
		mutate func(*LineageCandidate, *LineageAssessment, *LineageAudit, *FailabilityAnalysis)
	}{
		{name: "structured field flattening", mutate: func(candidate *LineageCandidate, _ *LineageAssessment, _ *LineageAudit, _ *FailabilityAnalysis) {
			candidate.Layers[2].StructuredPresence = false
			candidate.Header.Digest = sealCandidateForTest(t, *candidate)
		}},
		{name: "semantic truncation", mutate: func(candidate *LineageCandidate, _ *LineageAssessment, _ *LineageAudit, _ *FailabilityAnalysis) {
			candidate.Layers[3].SemanticSufficiency = false
			candidate.Header.Digest = sealCandidateForTest(t, *candidate)
		}},
		{name: "record identity loss", mutate: func(candidate *LineageCandidate, _ *LineageAssessment, _ *LineageAudit, _ *FailabilityAnalysis) {
			candidate.Layers[3].RecordIDs = []string{pairing.ToolCallEventID, pairing.CommandEventID}
			sort.Strings(candidate.Layers[3].RecordIDs)
			candidate.Header.Digest = sealCandidateForTest(t, *candidate)
		}},
		{name: "assessment substitution", mutate: func(_ *LineageCandidate, assessment *LineageAssessment, _ *LineageAudit, _ *FailabilityAnalysis) {
			assessment.Failability.Property = "different property"
			assessment.Header.Digest = sealArtifactForTest(t, *assessment)
		}},
		{name: "failability substitution", mutate: func(_ *LineageCandidate, _ *LineageAssessment, _ *LineageAudit, failability *FailabilityAnalysis) {
			failability.Finding.Property = "different property"
		}},
		{name: "cross task group", mutate: func(candidate *LineageCandidate, _ *LineageAssessment, _ *LineageAudit, _ *FailabilityAnalysis) {
			candidate.Header.TaskGroupID = "different-task-group"
			candidate.Header.Digest = sealCandidateForTest(t, *candidate)
		}},
		{name: "cross state", mutate: func(_ *LineageCandidate, assessment *LineageAssessment, _ *LineageAudit, _ *FailabilityAnalysis) {
			assessment.Freshness.ObservedStateDigest = strings.Repeat("8", 64)
			assessment.Failability.Property = "repository tests pass"
			assessment.Header.Digest = sealArtifactForTest(t, *assessment)
		}},
		{name: "audit parent substitution", mutate: func(_ *LineageCandidate, _ *LineageAssessment, audit *LineageAudit, _ *FailabilityAnalysis) {
			audit.Header.Parents[1].Digest = strings.Repeat("9", 64)
			audit.Header.Digest = sealArtifactForTest(t, *audit)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutatedCandidate := candidate
			mutatedCandidate.Layers = cloneLayerBindings(candidate.Layers)
			mutatedAssessment := assessment
			mutatedAudit := audit
			mutatedAudit.Header.Parents = append([]ParentRef(nil), audit.Header.Parents...)
			mutatedFailability := failability
			test.mutate(&mutatedCandidate, &mutatedAssessment, &mutatedAudit, &mutatedFailability)
			if _, err := BuildVerificationEvidenceBOM(
				"bom-rejected", source, witness, pairing, mutatedCandidate, mutatedAssessment, mutatedAudit, mutatedFailability,
				target, []string{"call_id", "exit_status", "stderr", "stdout"}, []string{"exit_status", "stderr", "stdout"},
			); err == nil {
				t.Fatal("weakened or substituted BOM lineage was accepted")
			}
		})
	}
}

func TestBOMFullLineageRejectsOperandReorderingEvenWhenStandaloneShapeIsValid(t *testing.T) {
	source, witness, pairing, candidate, assessment, audit, failability, target := completeBOMInputs(t)
	bom, err := BuildVerificationEvidenceBOM(
		"bom-operand-order", source, witness, pairing, candidate, assessment, audit, failability,
		target, []string{"call_id", "exit_status", "stderr", "stdout"}, []string{"exit_status", "stderr", "stdout"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(bom.Evidence.Operands) < 2 {
		t.Fatal("test requires ordered operands")
	}
	bom.Evidence.Operands[0], bom.Evidence.Operands[1] = bom.Evidence.Operands[1], bom.Evidence.Operands[0]
	bom.Header.Digest = sealBOMForTest(t, bom)
	if err := bom.Validate(); err != nil {
		t.Fatal("standalone BOM should preserve caller order until full lineage traversal:", err)
	}
	if err := ValidateBOMLineage(bom, source, witness, pairing, candidate, assessment, audit, failability, target); err == nil {
		t.Fatal("operand reordering survived full BOM lineage validation")
	}
}

func TestBOMConstructionRejectsTransformationTargetSubstitution(t *testing.T) {
	source, witness, pairing, candidate, assessment, audit, failability, _ := completeBOMInputs(t)
	substituted, err := BuildTransformationTargetBinding(
		"target-substituted", candidate.Header.TaskGroupID, strings.Repeat("9", 64), candidate.Layers[3].ObjectDigest,
		[]string{"call_id", "exit_status", "stderr", "stdout"}, []string{"exit_status", "stderr", "stdout"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildVerificationEvidenceBOM(
		"bom-target-substitution", source, witness, pairing, candidate, assessment, audit, failability, substituted,
		[]string{"call_id", "exit_status", "stderr", "stdout"}, []string{"exit_status", "stderr", "stdout"},
	); err == nil {
		t.Fatal("substituted transformation target was accepted")
	}
}

func completeBOMInputs(t *testing.T) (VerificationLineageSource, ExecutionWitness, WitnessNativePairing, LineageCandidate, LineageAssessment, LineageAudit, FailabilityAnalysis, TransformationTargetBinding) {
	t.Helper()
	witness := stdoutSuccessWitnessForPairing(t)
	witness.Argv = []string{"go", "test", "./..."}
	witness.CommandOperandsDigest, _ = digestJSON(witness.Argv)
	raw := codexPairingRaw(t, witness, witness.InvocationID, witness.Argv, witness.ExitStatus, "verification passed\n", "", witness.StartedAt, witness.EndedAt)
	source, witness := bindNativePairingSource(t, raw, witness)
	pairing, err := PairExecutionWitness(source, witness, raw, WitnessPairingPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	records := []string{pairing.CommandEventID, pairing.ToolCallEventID, pairing.ToolResultEventID}
	sort.Strings(records)
	required := []string{"call_id", "exit_status", "stderr", "stdout"}
	channels := []string{"exit_status", "stderr", "stdout"}
	candidate := LineageCandidate{
		Header: ArtifactHeader{
			SchemaVersion: CandidateSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "candidate-complete-lineage", TaskID: "TASK-069", TaskGroupID: source.Header.TaskGroupID,
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest,
			Parents: []ParentRef{parentFromArtifact("source", source.Header), parentFromArtifact("witness", witness.Header)},
		},
		CandidateID: "candidate-complete-lineage", ClaimID: "claim-tests-pass", InvocationID: witness.InvocationID,
		ParserIdentityDigest: strings.Repeat("7", 64), RequestFingerprint: strings.Repeat("e", 64),
		CanonicalRequestLineageDigest: strings.Repeat("f", 64),
		Alignment: AlignmentEvidence{
			CallIDMatched: true, ParentEdgesMatched: true, CommandOperandsDigest: witness.CommandOperandsDigest,
			TemporalWindowMillis: witness.EndedAt.Sub(witness.StartedAt).Milliseconds(),
		},
	}
	layers := []struct {
		name    string
		digest  string
		records []string
	}{
		{"runtime_witness", witness.Header.Digest, []string{witness.InvocationID}},
		{"native_export", source.RawRecordDigest, records},
		{"canonical_graph", source.CanonicalTrajectoryDigest, records},
		{"retained_bundle", strings.Repeat("d", 64), records},
		{"verifier_request", candidate.RequestFingerprint, []string{witness.InvocationID}},
	}
	for _, layer := range layers {
		candidate.Layers = append(candidate.Layers, LayerBinding{
			Layer: layer.name, ObjectDigest: layer.digest, RecordIDs: append([]string(nil), layer.records...),
			RequiredFields: append([]string(nil), required...), DecisiveChannels: append([]string(nil), channels...),
			StructuredPresence: true, SemanticSufficiency: true,
		})
	}
	candidate.Header.Digest = sealCandidateForTest(t, candidate)
	probe := FailabilityProbe{
		ProbeID: "probe-complete-lineage", Property: "repository tests pass", Executable: witness.Argv[0], Argv: append([]string(nil), witness.Argv...),
		Observable: "exit status and separated subprocess streams", FailureCondition: "non-zero exit or failing test result",
		ProofRecordIDs: []string{"proof-command", "proof-result"}, InvocationExecution: ProofProven, FailureObservableBound: true,
		StateBindingRequired: true, ObservedStateDigest: witness.RepositoryStateDigest, ClaimStateDigest: witness.RepositoryStateDigest,
		TestRun: &TestRunProbe{CountsObserved: true, Discovered: 1, Executed: 1, CacheUsed: ProofDisproven},
	}
	failability, err := AnalyzeFailability(probe)
	if err != nil {
		t.Fatal(err)
	}
	observed := witness.EndedAt.UTC()
	assessment := LineageAssessment{
		Header: ArtifactHeader{
			SchemaVersion: AssessmentSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "assessment-complete-lineage", TaskID: "TASK-069", TaskGroupID: candidate.Header.TaskGroupID,
			DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest,
			Parents: []ParentRef{parentFromArtifact("candidate", candidate.Header), parentFromArtifact("witness", witness.Header)},
		},
		CandidateID: candidate.CandidateID, TerminalState: StateDirectVerificationInvocation, StateDisposition: DispositionEligible,
		PrecedenceApplied: 10, ProofRecordIDs: append([]string(nil), failability.ProofRecordIDs...),
		ResultProvenance: []ProvenanceClass{ProvenanceExitStatus, ProvenanceStderr, ProvenanceStdout}, Failability: failability.Finding,
		Freshness:    FreshnessInterval{State: FreshnessCurrent, ObservedStateDigest: witness.RepositoryStateDigest, ValidFrom: observed, EvaluatedAt: observed.Add(time.Second)},
		ReviewerMode: "deterministic_lineage_analysis",
	}
	assessment.Header.Digest = sealArtifactForTest(t, assessment)
	classification := classificationInputForState(9, StateDirectVerificationInvocation)
	classification.UnitID = "unit-complete-lineage"
	classification.TaskGroupID = candidate.Header.TaskGroupID
	classification.Format = source.ExportFormat
	classification.SourceSessionID = source.SourceSessionID
	unit, err := ClassifyEarliestLoss(classification)
	if err != nil {
		t.Fatal(err)
	}
	auditHeader := ArtifactHeader{
		SchemaVersion: AuditSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
		ObjectID: "audit-complete-lineage", TaskID: "TASK-069", TaskGroupID: "study", DataRole: RoleAdapterDevelopment,
		PlanDigest: LockedPlanDigest,
		Parents: []ParentRef{
			{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-plan-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest},
			parentFromArtifact("assessment", assessment.Header),
			{Relation: "capability", SchemaVersion: CapabilitySchemaVersion, ObjectID: "capability-codex", TaskID: "TASK-069", TaskGroupID: "study", Digest: strings.Repeat("a", 64)},
		},
	}
	audit, err := BuildLineageAudit(auditHeader, "audit-complete-lineage", strings.Repeat("b", 64), strings.Repeat("c", 64), []string{"captured_format_only"}, []ClassifiedLineageUnit{unit})
	if err != nil {
		t.Fatal(err)
	}
	target, err := BuildTransformationTargetBinding(
		"target-complete-lineage", candidate.Header.TaskGroupID, candidate.Layers[2].ObjectDigest, candidate.Layers[3].ObjectDigest,
		required, channels,
	)
	if err != nil {
		t.Fatal(err)
	}
	return source, witness, pairing, candidate, assessment, audit, failability, target
}

func cloneLayerBindings(layers []LayerBinding) []LayerBinding {
	clone := make([]LayerBinding, len(layers))
	for index, layer := range layers {
		clone[index] = layer
		clone[index].RecordIDs = append([]string(nil), layer.RecordIDs...)
		clone[index].RequiredFields = append([]string(nil), layer.RequiredFields...)
		clone[index].DecisiveChannels = append([]string(nil), layer.DecisiveChannels...)
	}
	return clone
}
