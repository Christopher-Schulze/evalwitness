package lineage

import (
	"strings"
	"testing"
	"time"
)

func TestVerificationEvidenceBOMBindsParentsFreshnessAndDecisiveChannels(t *testing.T) {
	bom, candidate := validBOMForTest(t)
	if err := ValidateBOMCandidateBinding(bom, candidate); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*VerificationEvidenceBOM)
	}{
		{"parent digest substitution", func(value *VerificationEvidenceBOM) { value.AssessmentDigest = strings.Repeat("9", 64) }},
		{"candidate identity substitution", func(value *VerificationEvidenceBOM) { value.CandidateID = "candidate-2" }},
		{"state substitution", func(value *VerificationEvidenceBOM) { value.RepositoryStateDigest = strings.Repeat("8", 64) }},
		{"decisive channel loss", func(value *VerificationEvidenceBOM) { value.Retention.SurvivingChannels = []string{"stdout"} }},
		{"required field truncation", func(value *VerificationEvidenceBOM) {
			value.Retention.TruncatedRequiredFields = []string{"exit_status"}
		}},
		{"closed evidence", func(value *VerificationEvidenceBOM) {
			value.ValidityInterval.State = FreshnessClosed
			value.ValidityInterval.ValidUntil = value.ValidityInterval.EvaluatedAt
			value.ValidityInterval.InvalidationEdges = []InvalidationEdge{{EdgeID: "edge-1", Kind: "file_change", SubjectAlias: "file-1", BeforeStateDigest: value.RepositoryStateDigest, AfterStateDigest: strings.Repeat("7", 64), ObservedAt: value.ValidityInterval.ValidUntil}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := bom
			test.mutate(&mutated)
			mutated.Header.Digest = sealBOMForTest(t, mutated)
			if err := mutated.Validate(); err == nil {
				t.Fatal("weakened verification-evidence BOM was accepted")
			}
		})
	}
	lineageSubstitution := bom
	lineageSubstitution.Retention.CanonicalRequestLineageDigest = strings.Repeat("0", 64)
	lineageSubstitution.Header.Digest = sealBOMForTest(t, lineageSubstitution)
	if err := lineageSubstitution.Validate(); err != nil {
		t.Fatal("independently valid substituted BOM should reach candidate binding check:", err)
	}
	if err := ValidateBOMCandidateBinding(lineageSubstitution, candidate); err == nil {
		t.Fatal("canonical request-lineage substitution with unchanged request fingerprint was accepted")
	}
}

func validBOMForTest(t *testing.T) (VerificationEvidenceBOM, LineageCandidate) {
	t.Helper()
	digests := []string{strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64), strings.Repeat("4", 64), strings.Repeat("5", 64)}
	candidate := LineageCandidate{
		Header: ArtifactHeader{SchemaVersion: CandidateSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion, ObjectID: "candidate-1", TaskID: "TASK-069", TaskGroupID: "group-1", DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest, Parents: []ParentRef{
			{Relation: "source", SchemaVersion: SourceSchemaVersion, ObjectID: "source-1", TaskID: "TASK-069", TaskGroupID: "group-1", Digest: digests[0]},
			{Relation: "witness", SchemaVersion: WitnessSchemaVersion, ObjectID: "witness-1", TaskID: "TASK-069", TaskGroupID: "group-1", Digest: digests[1]},
		}},
		CandidateID: "candidate-1", ClaimID: "claim-1", InvocationID: "invocation-1", ParserIdentityDigest: strings.Repeat("7", 64),
		RequestFingerprint: strings.Repeat("e", 64), CanonicalRequestLineageDigest: strings.Repeat("f", 64),
		Alignment: AlignmentEvidence{CallIDMatched: true, ParentEdgesMatched: true, CommandOperandsDigest: strings.Repeat("4", 64), TemporalWindowMillis: 1000},
	}
	layerNames := []string{"runtime_witness", "native_export", "canonical_graph", "retained_bundle", "verifier_request"}
	layerDigests := []string{digests[1], strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("d", 64), strings.Repeat("e", 64)}
	for index, name := range layerNames {
		candidate.Layers = append(candidate.Layers, LayerBinding{Layer: name, ObjectDigest: layerDigests[index], RecordIDs: []string{"record-1"}, RequiredFields: []string{"exit_status"}, DecisiveChannels: []string{"exit_status"}, StructuredPresence: true, SemanticSufficiency: true})
	}
	candidate.Header.Digest = sealCandidateForTest(t, candidate)
	parents := []ParentRef{
		{Relation: "source", SchemaVersion: SourceSchemaVersion, ObjectID: "source-1", TaskID: "TASK-069", TaskGroupID: "group-1", Digest: digests[0]},
		{Relation: "witness", SchemaVersion: WitnessSchemaVersion, ObjectID: "witness-1", TaskID: "TASK-069", TaskGroupID: "group-1", Digest: digests[1]},
		{Relation: "candidate", SchemaVersion: CandidateSchemaVersion, ObjectID: "candidate-1", TaskID: "TASK-069", TaskGroupID: "group-1", Digest: candidate.Header.Digest},
		{Relation: "assessment", SchemaVersion: AssessmentSchemaVersion, ObjectID: "assessment-1", TaskID: "TASK-069", TaskGroupID: "group-1", Digest: digests[3]},
		{Relation: "audit", SchemaVersion: AuditSchemaVersion, ObjectID: "audit-1", TaskID: "TASK-069", TaskGroupID: "study", Digest: digests[4]},
	}
	state := strings.Repeat("6", 64)
	observed := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	bom := VerificationEvidenceBOM{
		Header: ArtifactHeader{SchemaVersion: BOMSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion, ObjectID: "bom-1", TaskID: "TASK-069", TaskGroupID: "group-1", DataRole: RoleAdapterDevelopment, PlanDigest: LockedPlanDigest, Parents: parents},
		BOMID:  "bom-1", CandidateID: "candidate-1", SourceDigest: digests[0], ExecutionWitnessDigest: digests[1], CandidateDigest: candidate.Header.Digest, AssessmentDigest: digests[3], AuditDigest: digests[4],
		Evidence:              EvidenceBinding{ClaimID: "claim-1", FailableProperty: "repository tests pass", Executable: "go", Operands: []string{"./...", "test"}, ObservableFailureCondition: "nonzero exit", CallIDs: []string{"call-1"}, ResultIDs: []string{"result-1"}, RequiredFields: []string{"exit_status"}, DecisiveChannels: []string{"exit_status"}},
		Retention:             RetentionBinding{NativeRecordDigest: strings.Repeat("a", 64), CanonicalEventDigest: strings.Repeat("b", 64), TransformationTargetDigest: strings.Repeat("c", 64), RetainedBundleDigest: strings.Repeat("d", 64), RequestFingerprint: strings.Repeat("e", 64), CanonicalRequestLineageDigest: strings.Repeat("f", 64), SurvivingChannels: []string{"exit_status", "stdout"}, TruncatedRequiredFields: []string{}},
		RepositoryStateDigest: state, ValidityInterval: FreshnessInterval{State: FreshnessCurrent, ObservedStateDigest: state, ValidFrom: observed, EvaluatedAt: observed.Add(time.Minute)}, Accepted: true,
	}
	bom.Header.Digest = sealBOMForTest(t, bom)
	if err := bom.Validate(); err != nil {
		t.Fatal(err)
	}
	return bom, candidate
}

func sealBOMForTest(t *testing.T, bom VerificationEvidenceBOM) string {
	t.Helper()
	bom.Header.Digest = ""
	return sealArtifactForTest(t, bom)
}

func sealCandidateForTest(t *testing.T, candidate LineageCandidate) string {
	t.Helper()
	candidate.Header.Digest = ""
	return sealArtifactForTest(t, candidate)
}
