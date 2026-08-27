package agentstudy

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestBuildIsDeterministicAndUsesTwentyTwentyCases(t *testing.T) {
	plan, audit, release := loadReleaseFixture(t)
	inputs := BuildInputs{PlanDigest: plan.Digest, AuditDigest: audit.Digest, Release: release, CalibrationCases: 20, TestCases: 20, Seed: "agent-only-test-seed"}
	first, err := Build(inputs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || first.Counts.Cases != 40 || first.Counts.Calibration != 20 || first.Counts.Test != 20 || first.Counts.PrimaryAgreements != 40 || first.Counts.TieBreaks != 0 {
		t.Fatalf("unexpected deterministic study summary: first=%#v second=%#v", first.Counts, second.Counts)
	}
	firstBytes, err := EncodeIndented(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := EncodeIndented(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("identical agent-only builds produced different bytes")
	}
	if err := first.ValidateAgainstRelease(release); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaIsClosedAndVersionBound(t *testing.T) {
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["$id"] != "https://evalwitness.dev/schemas/agent-only-study.v1.json" {
		t.Fatalf("unexpected schema identity: %#v", schema)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties["schema_version"].(map[string]any)["const"] != SchemaVersion || properties["agent_only"].(map[string]any)["const"] != true {
		t.Fatalf("schema is not version-closed: %#v", schema)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("schema permits unknown properties")
	}
}

func TestDecodeRejectsUnknownFieldsTrailingJSONAndTamperedDigest(t *testing.T) {
	plan, audit, release := loadReleaseFixture(t)
	value, err := Build(BuildInputs{PlanDigest: plan.Digest, AuditDigest: audit.Digest, Release: release, CalibrationCases: 20, TestCases: 20, Seed: "agent-only-test-seed"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bytes.NewReader(encoded)); err != nil {
		t.Fatalf("canonical study failed strict decode: %v", err)
	}

	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	object["unknown_field"] = true
	unknown, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bytes.NewReader(unknown)); err == nil {
		t.Fatal("unknown agent-only study field was accepted")
	}

	trailing := append(append([]byte(nil), encoded...), []byte("{}\n")...)
	if _, err := Decode(bytes.NewReader(trailing)); err == nil {
		t.Fatal("trailing agent-only study JSON was accepted")
	}

	delete(object, "unknown_field")
	object["digest"] = "0000000000000000000000000000000000000000000000000000000000000000"
	tampered, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bytes.NewReader(tampered)); err == nil {
		t.Fatal("tampered agent-only study digest was accepted")
	}
}

func TestValidateAgainstReleaseRejectsReSealedSourceAuditTamper(t *testing.T) {
	plan, audit, release := loadReleaseFixture(t)
	value, err := Build(BuildInputs{PlanDigest: plan.Digest, AuditDigest: audit.Digest, Release: release, CalibrationCases: 20, TestCases: 20, Seed: "agent-only-test-seed"})
	if err != nil {
		t.Fatal(err)
	}
	tamperedDigest := value.Cases[0].SourceAudit.ManifestSourceDigest
	if tamperedDigest[0] == '0' {
		tamperedDigest = "1" + tamperedDigest[1:]
	} else {
		tamperedDigest = "0" + tamperedDigest[1:]
	}
	value.Cases[0].SourceAudit.ManifestSourceDigest = tamperedDigest
	tampered, err := Seal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := tampered.ValidateAgainstRelease(release); err == nil {
		t.Fatal("re-sealed source-audit tamper was accepted")
	}
}

func TestResolveCaseRunsAutomatedTieBreakOnIndependentDisagreement(t *testing.T) {
	plan, audit, release := loadReleaseFixture(t)
	var item mutation.CorpusCaseV3
	for _, candidate := range release.Cases {
		if candidate.Split == "test" && len(candidate.SourceIDs) == 1 {
			item = candidate
			break
		}
	}
	if item.ID == "" {
		t.Fatal("fixture has no single-source test case")
	}
	sources := sourceIndex(release.Sources)
	original := sources[item.SourceIDs[0]]
	original.SplitGroupID = "tampered-split-group"
	sources[item.SourceIDs[0]] = original
	auditRecord, err := buildSourceAudit(item, sources)
	if err != nil {
		t.Fatal(err)
	}
	primaryA := runPrimaryA(item, sources)
	primaryB := runPrimaryB(item, sources)
	if primaryA.Label == primaryB.Label {
		t.Fatalf("fixture tamper did not create independent disagreement: A=%s B=%s", primaryA.Label, primaryB.Label)
	}
	resolved := resolveCase(item, auditRecord, primaryA, primaryB)
	if resolved.TieBreak == nil || resolved.Resolution != ResolutionTie || resolved.FinalLabel != LabelAccepted {
		t.Fatalf("automated tie-break did not resolve as expected: %#v", resolved)
	}
	_ = plan
	_ = audit
}

func loadReleaseFixture(t *testing.T) (mutation.CorpusDevelopmentPlan, mutation.CorpusDevelopmentAuditV3, mutation.CorpusReleaseV3) {
	t.Helper()
	planFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v3-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := mutation.DecodeCorpusDevelopmentPlan(planFile)
	_ = planFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	auditFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v3-natural-audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	audit, err := mutation.DecodeCorpusDevelopmentAuditV3(auditFile, plan)
	_ = auditFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	releaseFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v3-release.json"))
	if err != nil {
		t.Fatal(err)
	}
	release, err := mutation.DecodeCorpusReleaseV3(releaseFile, plan, audit)
	_ = releaseFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	return plan, audit, release
}
