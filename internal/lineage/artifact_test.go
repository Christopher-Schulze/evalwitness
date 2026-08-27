package lineage

import (
	"bytes"
	"strings"
	"testing"
)

func TestSourceRoundTripRejectsUnknownFieldAndDigestMutation(t *testing.T) {
	source := validSourceForTest(t)
	encoded, err := EncodeIndented(source)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := DecodeDocument("source", bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Valid || summary.SchemaVersion != SourceSchemaVersion || summary.ObjectID != source.Header.ObjectID || summary.Digest != source.Header.Digest {
		t.Fatalf("unexpected source summary: %+v", summary)
	}
	unknown := bytes.Replace(encoded, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1)
	if _, err := DecodeDocument("source", bytes.NewReader(unknown)); err == nil {
		t.Fatal("unknown source field was accepted")
	}
	mutated := source
	mutated.ExportVersion = "changed"
	if err := mutated.Validate(); err == nil {
		t.Fatal("post-seal source mutation was accepted")
	}
	if _, err := DecodeDocument("unknown", strings.NewReader("{}")); err == nil {
		t.Fatal("unknown document type was accepted")
	}
}

func TestInventoryDAGRejectsCycleAndUnknownParent(t *testing.T) {
	entries := []SchemaInventoryEntry{
		{DocumentType: "one", SchemaVersion: "one", ParentRequirements: []ParentRequirement{{Relation: "two", SchemaVersions: []string{"two"}, Minimum: 1, Maximum: 1}}},
		{DocumentType: "two", SchemaVersion: "two", ParentRequirements: []ParentRequirement{{Relation: "one", SchemaVersions: []string{"one"}, Minimum: 1, Maximum: 1}}},
	}
	if err := validateInventoryDAG(entries); err == nil {
		t.Fatal("cyclic schema parent graph was accepted")
	}
	entries[1].ParentRequirements[0].SchemaVersions[0] = "missing"
	if err := validateInventoryDAG(entries); err == nil {
		t.Fatal("unknown parent schema was accepted")
	}
}

func validSourceForTest(t *testing.T) VerificationLineageSource {
	t.Helper()
	source := VerificationLineageSource{
		Header: ArtifactHeader{
			SchemaVersion: SourceSchemaVersion, CanonicalPolicy: CanonicalPolicy, ProtocolVersion: ProtocolVersion,
			ObjectID: "source-1", TaskID: "TASK-069", TaskGroupID: "group-1", DataRole: RoleAdapterDevelopment,
			PlanDigest: LockedPlanDigest,
			Parents:    []ParentRef{{Relation: "plan", SchemaVersion: PlanSchemaVersion, ObjectID: "task_069-plan-v1", TaskID: "TASK-069", TaskGroupID: "study", Digest: LockedPlanDigest}},
		},
		SourceClass: "checked_in_controls", AgentEcosystem: "fixture", RuntimeIdentityClass: "test_harness",
		ProviderMetadata: "not_applicable", ExportFormat: "fixture_jsonl", ExportVersion: "v1", CaptureMode: CaptureNativeExport,
		SourceSessionID: "session-1", LineageID: "lineage-1", NearDuplicateID: "near-duplicate-1", RepositoryID: "repository-id-1",
		RepositoryAlias: "repository-1", TaskAlias: "task-1", License: "MIT",
		RedistributionPermission: "permitted", PrivacyClass: "public_synthetic", RedactionPolicy: "none",
		RawRecordCount: 1, RawRecordDigest: strings.Repeat("1", 64), CanonicalTrajectoryDigest: strings.Repeat("2", 64),
		FieldAccountingDigest: strings.Repeat("3", 64),
	}
	digest, err := artifactDigest(source)
	if err != nil {
		t.Fatal(err)
	}
	source.Header.Digest = digest
	if err := source.Validate(); err != nil {
		t.Fatal(err)
	}
	return source
}
