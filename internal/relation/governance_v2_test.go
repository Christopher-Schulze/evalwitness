package relation

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestFrozenRelationV2GovernanceArtifacts(t *testing.T) {
	plan := readGovernedPlan(t, "relation-audit-plan-v2.json")
	primary := readGovernedPrimarySample(t, "relation-primary-sample-v2.json")
	pilot := readGovernedPilotSample(t, "relation-pilot-sample-v2.json")
	amendment := readGovernedStudyAmendment(t, "relation-study-amendment-v2.json")

	if plan.SchemaVersion != PlanSchemaVersionV2 || plan.ProtocolVersion != ProtocolVersionV2 || plan.PrimarySampleSize != 32 ||
		plan.SourceCorpusDigest != "d0485f3484743a3d4ff907b295c0c9be11db21d2231664e5018fa2f047b6bf11" ||
		plan.SourceCorpusSpecDigest != "94989d548973dad7bfc04418781ed4f25df1b81d6ddd1fbeacc581fefaef0979" ||
		plan.SourceMutationProgramDigest != "30e368b56c42e24bb0cbaf30da1ff9d982a45d6499beca7745db95d2a30ac958" ||
		plan.SourceConstructAuditDigest != "822d8034a4a75faaf337a4abd6e51743e38104c3ffca9ce7f214e751f5d026db" {
		t.Fatal("frozen v2 plan lost its corpus, mutation-program, construct-audit, or sample contract")
	}
	if primary.PlanDigest != plan.Digest || primary.SourceCorpusDigest != plan.SourceCorpusDigest || primary.SelectedCases != 32 ||
		primary.UniqueTaskGroups != 32 || primary.UniqueLineageClusters != 24 || len(primary.FamilyCounts) != 8 ||
		countFor(primary.SplitCounts, string(study.RoleCalibration)) != 16 || countFor(primary.SplitCounts, string(study.RoleTest)) != 16 ||
		!validDigest(primary.Bindings.ConstructFirewalls) {
		t.Fatal("frozen v2 primary sample lost balance, task independence, lineage reporting, or construct bindings")
	}
	for _, count := range primary.FamilyCounts {
		if count.Count != 4 {
			t.Fatalf("v2 primary family %q has %d cases, want 4", count.ID, count.Count)
		}
	}
	if pilot.PlanDigest != plan.Digest || pilot.PrimarySampleDigest != primary.Digest || pilot.SelectedCases != 8 ||
		pilot.UniqueTaskGroups != 8 || pilot.UniqueLineageClusters != 8 || pilot.PrimaryOverlap != 0 || !validDigest(pilot.Bindings.ConstructFirewalls) {
		t.Fatal("frozen v2 pilot lost its primary binding or zero-overlap contract")
	}
	if amendment.PlanDigest != plan.Digest || amendment.PrimarySampleDigest != primary.Digest || amendment.PilotSampleDigest != pilot.Digest ||
		amendment.Primary.Cases != 32 || amendment.Primary.EffectiveTaskGroups != 32 || amendment.Primary.PrimaryLabels != 64 ||
		amendment.EmpiricalStatus != "not_run" || amendment.ExternalActionStatus != ExternalActionNotAuthorized {
		t.Fatal("frozen v2 amendment lost its design, workload, empirical, or authorization boundary")
	}
}

func TestRelationV2SchemasRequireAuditBindingsAndExcludeThemFromV1(t *testing.T) {
	v1, err := Schema("primary-sample")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := Schema("primary-sample-v2")
	if err != nil {
		t.Fatal(err)
	}
	v1Properties := v1["properties"].(map[string]any)
	if _, exists := v1Properties["source_construct_audit_digest"]; exists {
		t.Fatal("v1 primary schema exposes a v2-only construct audit field")
	}
	v2Required := v2["required"].([]string)
	for _, field := range []string{"source_corpus_spec_digest", "source_mutation_program_digest", "source_construct_audit_digest", "unique_lineage_clusters", "source_format_counts"} {
		if !slices.Contains(v2Required, field) {
			t.Fatalf("v2 primary schema does not require %q", field)
		}
	}
	v2Properties := v2["properties"].(map[string]any)
	bindings := v2Properties["bindings"].(JSONSchema)
	if !slices.Contains(bindings["required"].([]string), "construct_firewalls") {
		t.Fatal("v2 primary schema does not require the construct-firewall commitment")
	}
}

func TestRelationV2PrelaunchSchemasAreVersionMatched(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		required []string
	}{
		{"case-material", "case-material-v2", []string{"source_corpus_spec_digest", "source_mutation_program_digest", "source_construct_audit_digest", "relation_contract_version", "evidence_boundary_version", "construct_firewall_digest"}},
		{"private-mapping", "private-mapping-v2", []string{"source_corpus_spec_digest", "source_mutation_program_digest", "source_construct_audit_digest", "relation_contract_version", "evidence_boundary_version", "construct_firewall_digest"}},
		{"pilot-readiness", "pilot-readiness-v2", []string{"source_corpus_digest", "source_corpus_spec_digest", "source_mutation_program_digest", "source_construct_audit_digest", "construct_firewall_commitment_digest"}},
		{"pilot-change-receipt", "pilot-change-receipt-v2", []string{"source_corpus_digest", "source_corpus_spec_digest", "source_mutation_program_digest", "source_construct_audit_digest", "construct_firewall_commitment_digest"}},
		{"pilot-launch-dossier", "pilot-launch-dossier-v2", []string{"source_corpus_digest", "source_corpus_spec_digest", "source_mutation_program_digest", "source_construct_audit_digest", "construct_firewall_commitment_digest"}},
		{"reviewer-record", "reviewer-record-v2", nil},
		{"review-assignment", "review-assignment-v2", nil},
		{"reviewer-kit", "reviewer-kit-v2", nil},
		{"pair-judgment", "pair-judgment-v2", nil},
		{"judgment-batch", "judgment-batch-v2", nil},
		{"prereveal-ambiguity", "prereveal-ambiguity-v2", nil},
		{"condition-probe", "condition-probe-v2", nil},
		{"condition-probe-batch", "condition-probe-batch-v2", nil},
		{"mapping-reveal", "mapping-reveal-v2", nil},
		{"translation-result", "translation-result-v2", nil},
		{"relation-resolution", "relation-resolution-v2", nil},
		{"formal-human-comparison", "formal-human-comparison-v2", nil},
		{"terminal-ledger", "terminal-ledger-v2", nil},
	}
	for _, test := range tests {
		t.Run(test.v2, func(t *testing.T) {
			v1, err := Schema(test.v1)
			if err != nil {
				t.Fatal(err)
			}
			v2, err := Schema(test.v2)
			if err != nil {
				t.Fatal(err)
			}
			v1Properties := v1["properties"].(map[string]any)
			v2Properties := v2["properties"].(map[string]any)
			protocol := v2Properties["protocol_version"].(JSONSchema)
			if protocol["const"] != ProtocolVersionV2 {
				t.Fatal("v2 prelaunch schema does not freeze relation protocol v2")
			}
			v2Required := v2["required"].([]string)
			for _, field := range test.required {
				if _, exists := v1Properties[field]; exists {
					t.Fatalf("v1 schema exposes v2-only field %q", field)
				}
				if _, exists := v2Properties[field]; !exists || !slices.Contains(v2Required, field) {
					t.Fatalf("v2 schema does not require %q", field)
				}
			}
		})
	}
}

func TestRelationSchemaSurfaceHasNoUntestedDocument(t *testing.T) {
	if len(relationSchemaTypes) != 97 {
		t.Fatalf("relation schema surface has %d documents, want 97", len(relationSchemaTypes))
	}
	for document := range relationSchemaTypes {
		if _, err := Schema(document); err != nil {
			t.Fatalf("relation schema %q is not generatable: %v", document, err)
		}
	}
}

func TestRelationV2GovernanceRejectsCrossVersionAndBindingTamper(t *testing.T) {
	v1Plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	v2Plan := readGovernedPlan(t, "relation-audit-plan-v2.json")
	primary := readGovernedPrimarySample(t, "relation-primary-sample-v2.json")
	pilot := readGovernedPilotSample(t, "relation-pilot-sample-v2.json")
	if _, err := BuildStudyAmendment(v1Plan, pilot, primary, "2026-08-09T22:20:14Z"); err == nil {
		t.Fatal("relation amendment accepted a v1 plan with v2 samples")
	}
	primary.SourceConstructAuditDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tampered, err := SealPrimarySampleV2(primary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildStudyAmendment(v2Plan, pilot, tampered, "2026-08-09T22:20:14Z"); err == nil {
		t.Fatal("relation amendment accepted a primary sample bound to another construct audit")
	}
	observations := []AxisObservation{
		{Axis: AxisEvidenceStrength, Rating: NormalizedOriginal},
		{Axis: AxisInformation, Rating: NormalizedSufficient},
		{Axis: AxisSemanticQuality, Rating: NormalizedEqual},
	}
	translation, err := Translate(v2Plan, mutation.FamilyTestEvidenceOmitted, observations)
	if err != nil {
		t.Fatal(err)
	}
	if translation.SchemaVersion != TranslationResultSchemaVersionV2 || translation.ProtocolVersion != ProtocolVersionV2 {
		t.Fatal("v2 translation did not issue a version-matched artifact")
	}
	qualification, answerKey, err := DefaultQualification(v2Plan, "v2-version-test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if qualification.SchemaVersion != QualificationSetSchemaVersionV2 || qualification.ProtocolVersion != ProtocolVersionV2 ||
		answerKey.SchemaVersion != QualificationKeySchemaVersionV2 || answerKey.ProtocolVersion != ProtocolVersionV2 {
		t.Fatal("v2 qualification did not issue version-matched artifacts")
	}
	qualification.ProtocolVersion = ProtocolVersionV1
	if _, err := SealQualificationSet(qualification); err == nil {
		t.Fatal("v1 qualification envelope accepted v2 blind packets")
	}
}

func readGovernedPlan(t *testing.T, name string) Plan {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed relation plan: %v", err)
		}
	}()
	value, err := DecodePlan(file)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readGovernedPrimarySample(t *testing.T, name string) PrimarySample {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed relation primary sample: %v", err)
		}
	}()
	value, err := DecodePrimarySample(file)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readGovernedPilotSample(t *testing.T, name string) PilotSample {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed relation pilot sample: %v", err)
		}
	}()
	value, err := DecodePilotSample(file)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readGovernedStudyAmendment(t *testing.T, name string) StudyAmendment {
	t.Helper()
	file := openGovernedRelationArtifact(t, name)
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close governed relation study amendment: %v", err)
		}
	}()
	value, err := DecodeStudyAmendment(file)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func openGovernedRelationArtifact(t *testing.T, name string) *os.File {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "..", "eval", "governance", name))
	if err != nil {
		t.Fatal(err)
	}
	return file
}
