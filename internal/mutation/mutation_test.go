package mutation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestDeterministicMutationFamiliesProduceValidWitnesses(t *testing.T) {
	plain := ingestFixture(t, "plain.txt")
	claude := ingestFixture(t, "claude-code.jsonl")
	swe := ingestFixture(t, "swe-bench.json")
	aliased := aliasedPathTrajectory(t, swe)
	failing := failingEvidenceTrajectory(t, claude)
	failedCommand := failingCommandTrajectory(t, claude)
	independent := linklessTrajectory(t, claude)
	tests := []struct {
		name       string
		family     Family
		trajectory preprocess.Trajectory
		configure  func(*ApplyRequest)
	}{
		{name: "patch hunk removal", family: FamilyPatchHunkRemoval, trajectory: swe, configure: func(request *ApplyRequest) { request.RequiredFragment = "+new" }},
		{name: "failing change", family: FamilyFailingChangeReintroduced, trajectory: swe, configure: func(request *ApplyRequest) { request.RequiredFragment = "+new"; request.Replacement = "+broken" }},
		{name: "test evidence omitted", family: FamilyTestEvidenceOmitted, trajectory: claude},
		{name: "test evidence falsified", family: FamilyTestEvidenceFalsified, trajectory: failing},
		{name: "command failure hidden", family: FamilyCommandFailureHidden, trajectory: failedCommand},
		{name: "tool output incomplete", family: FamilyToolOutputIncomplete, trajectory: claude},
		{name: "irrelevant verbosity", family: FamilyIrrelevantVerbosity, trajectory: plain},
		{name: "neutral formatting", family: FamilyNeutralFormatting, trajectory: plain},
		{name: "stable path alias", family: FamilyStablePathAlias, trajectory: aliased},
		{name: "causal reorder", family: FamilyCausalIndependentReorder, trajectory: independent},
		{name: "score injection", family: FamilyUntrustedScoreInjection, trajectory: claude},
		{name: "ambiguous semantic edit", family: FamilyAmbiguousSemanticEdit, trajectory: plain, configure: func(request *ApplyRequest) {
			request.Replacement = "A plausible but unresolved alternative implementation."
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validRequest(test.family)
			if test.configure != nil {
				test.configure(&request)
			}
			first, err := Apply(test.trajectory, request)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Apply(test.trajectory, request)
			if err != nil {
				t.Fatal(err)
			}
			if first.Manifest.Digest != second.Manifest.Digest || first.Mutated.Digest != second.Mutated.Digest || first.Packet.Digest != second.Packet.Digest {
				t.Fatal("identical mutation inputs produced different identities")
			}
			if err := first.Manifest.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := first.Packet.Validate(); err != nil {
				t.Fatal(err)
			}
			if test.family == FamilyAmbiguousSemanticEdit {
				if first.Manifest.Witness.LabelState != LabelAmbiguous || !first.Manifest.Review.Required {
					t.Fatal("ambiguous mutation did not require blinded adjudication")
				}
			} else if first.Manifest.Witness.LabelState != LabelProven {
				t.Fatalf("deterministic mutation label = %s", first.Manifest.Witness.LabelState)
			}
		})
	}
}

func TestCandidateOrderReversalPreservesUnorderedPair(t *testing.T) {
	left, err := preprocess.IngestString("candidate alpha", preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	right, err := preprocess.IngestString("candidate beta", preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	request := validRequest(FamilyCandidateOrderReversal)
	result, err := ApplyCandidateOrderReversal(left, right, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.PairOrder != [2]string{right.Digest, left.Digest} || result.Manifest.ExpectedRelation != RelationQualityEqual {
		t.Fatalf("reversed pair = %#v", result)
	}
	if err := result.Manifest.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCandidateOrderReversalV2BindsConstructFirewall(t *testing.T) {
	left, err := preprocess.IngestString("candidate alpha", preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	right, err := preprocess.IngestString("candidate beta", preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := ApplyCandidateOrderReversalV2(left, right, validRequest(FamilyCandidateOrderReversal))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != ConstructApplied || outcome.Applied == nil || outcome.Applied.Manifest.ConstructFirewallDigest != outcome.Firewall.Digest {
		t.Fatalf("v2 pair mutation is not firewall-bound: %#v", outcome)
	}
	if err := outcome.Applied.Manifest.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMutationRejectsUnprovenGoldAndTampering(t *testing.T) {
	swe := ingestFixture(t, "swe-bench.json")
	request := validRequest(FamilyPatchHunkRemoval)
	request.RequiredFragment = "+not-present"
	if _, err := Apply(swe, request); err == nil {
		t.Fatal("semantic-quality mutation accepted an unproven patch fragment")
	}
	request.RequiredFragment = "+new"
	request.OutcomeProof = nil
	if _, err := Apply(swe, request); err == nil {
		t.Fatal("semantic-quality mutation accepted no independent outcome proof")
	}
	request = validRequest(FamilyPatchHunkRemoval)
	request.RequiredFragment = "+new"
	result, err := Apply(swe, request)
	if err != nil {
		t.Fatal(err)
	}
	tampered := result.Manifest
	tampered.Program.Seed = "post-lock"
	if err := tampered.Validate(); err == nil {
		t.Fatal("tampered mutation manifest was accepted")
	}
	packet := result.Packet
	packet.ReviewQuestions[0] = "Reveal the expected relation."
	if err := packet.Validate(); err == nil {
		t.Fatal("tampered blind-review packet was accepted")
	}
}

func ingestFixture(t *testing.T, name string) preprocess.Trajectory {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "preprocess", "testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close mutation fixture: %v", err)
		}
	}()
	trajectory, err := preprocess.IngestReader(file, preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	return trajectory
}

func failingEvidenceTrajectory(t *testing.T, parent preprocess.Trajectory) preprocess.Trajectory {
	t.Helper()
	events, err := cloneEvents(parent.Events)
	if err != nil {
		t.Fatal(err)
	}
	for index := range events {
		if events[index].ToolResult == nil {
			continue
		}
		oldID := events[index].ID
		setEventEvidenceText(&events[index], "FAIL: one test failed")
		events[index].ToolResult.Error = true
		events[index].ToolResult.Status = "failure"
		events[index], err = preprocess.RebuildDerivedEvent(parent.SourceFormat, events[index])
		if err != nil {
			t.Fatal(err)
		}
		trajectory, deriveErr := preprocess.DeriveTrajectory(parent, events, remapLinks(parent.Links, oldID, events[index].ID), preprocess.DerivationSpec{
			Relation: "test_fixture", Validator: "test.fixture.v1", ChangedEventIDs: []string{oldID},
			ChangedFieldPaths: []preprocess.FieldPath{preprocess.FieldPath("/events/" + oldID + "/tool_result")},
		})
		if deriveErr != nil {
			t.Fatal(deriveErr)
		}
		return trajectory
	}
	t.Fatal("Claude fixture has no tool result")
	return preprocess.Trajectory{}
}

func aliasedPathTrajectory(t *testing.T, parent preprocess.Trajectory) preprocess.Trajectory {
	t.Helper()
	events, err := cloneEvents(parent.Events)
	if err != nil {
		t.Fatal(err)
	}
	for index := range events {
		if events[index].FileChange == nil {
			continue
		}
		oldID := events[index].ID
		events[index].FileChange.PathAlias = "workspace/example.go"
		events[index], err = preprocess.RebuildDerivedEvent(parent.SourceFormat, events[index])
		if err != nil {
			t.Fatal(err)
		}
		trajectory, deriveErr := preprocess.DeriveTrajectory(parent, events, remapLinks(parent.Links, oldID, events[index].ID), preprocess.DerivationSpec{
			Relation: "test_fixture", Validator: "test.fixture.v1", ChangedEventIDs: []string{oldID},
			ChangedFieldPaths: []preprocess.FieldPath{preprocess.FieldPath("/events/" + oldID + "/file_change/path_alias")},
		})
		if deriveErr != nil {
			t.Fatal(deriveErr)
		}
		return trajectory
	}
	t.Fatal("SWE fixture has no file change")
	return preprocess.Trajectory{}
}

func failingCommandTrajectory(t *testing.T, parent preprocess.Trajectory) preprocess.Trajectory {
	t.Helper()
	events, err := cloneEvents(parent.Events)
	if err != nil {
		t.Fatal(err)
	}
	for index := range events {
		if events[index].Command == nil {
			continue
		}
		oldID := events[index].ID
		exitCode := 1
		events[index].Command.ExitCode = &exitCode
		events[index], err = preprocess.RebuildDerivedEvent(parent.SourceFormat, events[index])
		if err != nil {
			t.Fatal(err)
		}
		trajectory, deriveErr := preprocess.DeriveTrajectory(parent, events, remapLinks(parent.Links, oldID, events[index].ID), preprocess.DerivationSpec{
			Relation: "test_fixture", Validator: "test.fixture.v1", ChangedEventIDs: []string{oldID},
			ChangedFieldPaths: []preprocess.FieldPath{preprocess.FieldPath("/events/" + oldID + "/command/exit_code")},
		})
		if deriveErr != nil {
			t.Fatal(deriveErr)
		}
		return trajectory
	}
	t.Fatal("Claude fixture has no command")
	return preprocess.Trajectory{}
}

func linklessTrajectory(t *testing.T, parent preprocess.Trajectory) preprocess.Trajectory {
	t.Helper()
	trajectory, err := preprocess.DeriveTrajectory(parent, parent.Events, []preprocess.Link{}, preprocess.DerivationSpec{
		Relation: "test_fixture", Validator: "test.fixture.v1", ChangedEventIDs: []string{}, ChangedFieldPaths: []preprocess.FieldPath{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return trajectory
}

func validRequest(family Family) ApplyRequest {
	definition, _ := DefinitionFor(family)
	validatorKind := ValidationPreservation
	if definition.RequiresGoldProof {
		validatorKind = ValidationFormal
	}
	request := ApplyRequest{
		CorpusVersion: "evalwitness-corruption-development.v1", TaskID: "fixture-task", RepositoryID: "fixture-repository",
		SourceFamily: "evalwitness-golden", SourceLocation: "internal/preprocess/testdata/golden", SourceRevision: "fixture-v1",
		SplitGroupID: "fixture-task", Seed: "frozen-seed", Family: family,
		Outcome:               SourceOutcome{Kind: "formal_fixture", Value: "pass", WitnessDigest: digestText("outcome")},
		Validator:             ValidatorSpec{ID: "evalwitness.fixture-validator", Version: "v1", Kind: validatorKind, ContractDigest: digestText("validator"), TimeoutMillis: 1_000, MaximumOutputBytes: 64 * 1024},
		License:               LicenseMetadata{SPDX: "MIT", SourceURL: "https://github.com/example/repository", SourceRevision: "fixture-v1", Redistribution: "permitted", Attribution: "EvalWitness fixtures"},
		Privacy:               PrivacyMetadata{Classification: "public", RedactionPolicyDigest: digestText("redaction"), PublicReleaseAllowed: true},
		ReviewSamplingStratum: "fixture-proven",
	}
	if definition.RequiresGoldProof {
		request.OutcomeProof = &OutcomeProof{
			Mechanism: validatorKind, ContractDigest: request.Validator.ContractDigest,
			OriginalPassed: true, MutatedPassed: false, IndependentOfTrace: true,
			WitnessDigest: digestText("independent fixture outcome proof"),
		}
	}
	return request
}

func TestEvidenceOnlyMutationsExposeTypedChangedField(t *testing.T) {
	result, err := Apply(ingestFixture(t, "claude-code.jsonl"), validRequest(FamilyTestEvidenceOmitted))
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest.Class != ClassEvidenceAvailability || len(result.Manifest.Affected.FieldPaths) != 1 || !strings.HasSuffix(string(result.Manifest.Affected.FieldPaths[0]), "/evidence") {
		t.Fatalf("evidence boundary = %#v", result.Manifest)
	}
}

func TestV2OmittedEvidenceRequiresVerifiedExecutionLineage(t *testing.T) {
	testOutcome, err := ApplyV2(ingestFixture(t, "claude-code.jsonl"), validRequest(FamilyTestEvidenceOmitted))
	if err != nil {
		t.Fatal(err)
	}
	if testOutcome.Status != ConstructApplied || testOutcome.Applied == nil || testOutcome.Firewall.SemanticRole != "test" {
		t.Fatalf("verified test evidence was not admitted: %#v", testOutcome)
	}
	if len(testOutcome.Firewall.ProofEventIDs) != 2 {
		t.Fatalf("verified test evidence did not bind call/result lineage: %#v", testOutcome.Firewall.ProofEventIDs)
	}
	if testOutcome.Applied.Manifest.Program.Version != MutationProgramVersionV2 || testOutcome.Applied.Manifest.ConstructFirewallDigest != testOutcome.Firewall.Digest {
		t.Fatal("v2 manifest does not bind the construct firewall")
	}
	if err := testOutcome.Firewall.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceChangedRegions(testOutcome.Applied.Manifest, ingestFixture(t, "claude-code.jsonl"), testOutcome.Applied.Mutated); err != nil {
		t.Fatalf("v2 applied construct did not reproduce its formal reduction: %v", err)
	}
	encodedFirewall, err := json.Marshal(testOutcome.Firewall)
	if err != nil {
		t.Fatal(err)
	}
	decodedFirewall, err := DecodeConstructFirewall(strings.NewReader(string(encodedFirewall)))
	if err != nil || decodedFirewall.Digest != testOutcome.Firewall.Digest {
		t.Fatalf("strict construct firewall round trip failed: %v", err)
	}

	download := strings.Join([]string{
		`{"type":"user","uuid":"user-1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"Download the fixture model."}}`,
		`{"type":"assistant","uuid":"assistant-1","parentUuid":"user-1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"I will download the model."},{"type":"tool_use","id":"tool-1","name":"shell","input":{"command":"python download_model.py"}}]}}`,
		`{"type":"user","uuid":"result-1","parentUuid":"assistant-1","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"Done!","is_error":false}]}}`,
	}, "\n")
	downloadTrajectory, err := preprocess.IngestReader(strings.NewReader(download), preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	downloadOutcome, err := ApplyV2(downloadTrajectory, validRequest(FamilyTestEvidenceOmitted))
	if err != nil {
		t.Fatal(err)
	}
	if downloadOutcome.Status != ConstructRejected || downloadOutcome.Applied != nil || !slices.Contains(downloadOutcome.Firewall.RejectionReasons, RejectionUnverifiedEvidenceRole) {
		t.Fatalf("generic download completion was admitted as test evidence: %#v", downloadOutcome)
	}
}

func TestV2NeutralFormattingIsNaturalAndTokenPreserving(t *testing.T) {
	prose := "This verifier report explains the observed behavior, preserves every lexical token, and wraps only natural prose so a reviewer can inspect the same claim without a pathological style cue."
	trajectory, err := preprocess.IngestString(prose, preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := ApplyV2(trajectory, validRequest(FamilyNeutralFormatting))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != ConstructApplied || outcome.Applied == nil {
		t.Fatalf("natural prose was not formatted: %#v", outcome)
	}
	after := eventText(outcome.Applied.Mutated.Events[0])
	if strings.Join(strings.Fields(prose), "\x00") != strings.Join(strings.Fields(after), "\x00") || strings.Contains(after, "\n\n") || hasSingleTokenLine(after) {
		t.Fatalf("v2 formatting changed tokens or produced pathological lines: %q", after)
	}

	command, err := preprocess.IngestString("Execute [/bin/sh] ps -ef | grep verifier-worker | grep -v grep || true because this command is copied exactly for terminal execution.", preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := ApplyV2(command, validRequest(FamilyNeutralFormatting))
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != ConstructRejected || !slices.Contains(rejected.Firewall.RejectionReasons, RejectionUnnaturalFormatting) {
		t.Fatalf("executable command text was accepted as neutral prose: %#v", rejected)
	}
}

func TestV2ReorderRejectsSharedToolTransactionDescendants(t *testing.T) {
	trajectory := ingestFixture(t, "claude-code.jsonl")
	request := validRequest(FamilyCausalIndependentReorder)
	for _, event := range trajectory.Events {
		if event.ToolCall != nil && event.ToolCall.CallID == "tool-1" {
			request.TargetEventID = event.ID
			break
		}
	}
	if request.TargetEventID == "" {
		t.Fatal("Claude fixture has no target tool call")
	}
	outcome, err := ApplyV2(trajectory, request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != ConstructRejected || outcome.Applied != nil || !slices.Contains(outcome.Firewall.RejectionReasons, RejectionTransactionDependency) {
		t.Fatalf("call and result-wrapper transaction was admitted for reorder: %#v", outcome)
	}
}

func TestFormalValidationAndReducerReproduceControlledRelation(t *testing.T) {
	request := validRequest(FamilyPatchHunkRemoval)
	request.RequiredFragment = "+new"
	result, err := Apply(ingestFixture(t, "swe-bench.json"), request)
	if err != nil {
		t.Fatal(err)
	}
	original := ingestFixture(t, "swe-bench.json")
	if err := ValidateFormalRelation(result.Manifest, original, result.Mutated); err != nil {
		t.Fatal(err)
	}
	reduction, err := ReduceChangedRegions(result.Manifest, original, result.Mutated)
	if err != nil {
		t.Fatal(err)
	}
	if len(reduction.Regions) != 1 || reduction.Regions[0].BeforeEnd-reduction.Regions[0].BeforeStart != len("+new") {
		t.Fatalf("unexpected minimal changed region: %#v", reduction)
	}
	tampered := reduction
	tampered.Regions[0].BeforeStart++
	if err := tampered.Validate(); err == nil {
		t.Fatal("tampered reduction witness was accepted")
	}
}

func TestMutationCodecAndSchemasAreStrict(t *testing.T) {
	request := validRequest(FamilyTestEvidenceOmitted)
	result, err := Apply(ingestFixture(t, "claude-code.jsonl"), request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeManifest(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != result.Manifest.Digest {
		t.Fatalf("strict manifest round trip failed: %v", err)
	}
	unknown := strings.TrimSuffix(string(encoded), "}") + `,"unknown":true}`
	if _, err := DecodeManifest(strings.NewReader(unknown)); err == nil {
		t.Fatal("strict mutation codec accepted an unknown field")
	}
	for _, document := range []string{"manifest", "witness", "blind-review-packet", "construct-firewall", "construct-firewall-v2", "construct-repair-evidence", "construct-firewall-challenge", "corpus-spec", "corpus-development-plan", "corpus-development-audit", "corpus-development-audit-v3", "corpus-release", "corpus-release-v3", "reduction-witness", "formal-control"} {
		schema, schemaErr := Schema(document)
		if schemaErr != nil || schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("schema %q is invalid: %v", document, schemaErr)
		}
	}
}

func TestCorpusDevelopmentPlanV2RoundTripAndTamperRejection(t *testing.T) {
	plan, err := DefaultCorpusDevelopmentPlanV2()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(plan)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCorpusDevelopmentPlan(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != plan.Digest {
		t.Fatalf("strict v2 development plan round trip failed: %v", err)
	}
	decoded.CasesPerFamily++
	if err := decoded.Validate(); err == nil {
		t.Fatal("tampered v2 development plan was accepted")
	}
}

func TestCorpusDevelopmentPlanAndNaturalAuditV3AreFrozen(t *testing.T) {
	plan, err := DefaultCorpusDevelopmentPlanV3()
	if err != nil {
		t.Fatal(err)
	}
	if plan.Digest != "5a7f4f55aafabd03ef9c802a9c48cf198a778228b40ade35e8f074df8d060c85" {
		t.Fatalf("v3 development plan digest = %s", plan.Digest)
	}
	planFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v3-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := planFile.Close(); err != nil {
			t.Errorf("close v3 plan: %v", err)
		}
	}()
	committedPlan, err := DecodeCorpusDevelopmentPlan(planFile)
	if err != nil || committedPlan.Digest != plan.Digest {
		t.Fatalf("committed v3 development plan drifted: %v", err)
	}
	auditFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v3-natural-audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := auditFile.Close(); err != nil {
			t.Errorf("close v3 audit: %v", err)
		}
	}()
	audit, err := DecodeCorpusDevelopmentAuditV3(auditFile, plan)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Digest != "af0c0fd56fb498586096a8776e0d40794ee93acf5afda67cc000e576bfcef4d2" ||
		audit.TotalAttempts != 939 || audit.AppliedAttempts != 689 || audit.RejectedAttempts != 250 ||
		audit.SelectedCases != 283 || audit.QuotasSatisfied ||
		!slices.Equal(audit.QuotaShortfalls, []CorpusCount{{ID: string(FamilyTestEvidenceOmitted), Count: 37}}) {
		t.Fatalf("v3 natural-corpus audit summary drifted: %#v", audit)
	}
	audit.SelectedCases++
	if err := audit.Validate(plan); err == nil {
		t.Fatal("tampered v3 natural-corpus audit was accepted")
	}
}

func TestControlledCorpusReleaseV3IsFrozenAndRejectsSentinelOverclaim(t *testing.T) {
	plan, err := DefaultCorpusDevelopmentPlanV3()
	if err != nil {
		t.Fatal(err)
	}
	auditFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v3-natural-audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	audit, err := DecodeCorpusDevelopmentAuditV3(auditFile, plan)
	if err := auditFile.Close(); err != nil {
		t.Errorf("close v3 audit: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	releaseFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v3-release.json"))
	if err != nil {
		t.Fatal(err)
	}
	release, err := DecodeCorpusReleaseV3(releaseFile, plan, audit)
	if err := releaseFile.Close(); err != nil {
		t.Errorf("close v3 release: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if release.Digest != "9b4999dafe2d37ea04c298b80a7aba0a1769755fdfd650cd01bf3a9cc31a2e42" || release.SelectedCases != 283 ||
		release.Policy.CoreCases != 280 || release.Policy.ScarcitySentinelCases != 3 || release.Policy.SentinelInPrimaryEstimand ||
		release.Policy.HeldOutSentinelClaimAvailable || release.Policy.BalancedEightFamilyAvailable {
		t.Fatalf("v3 controlled release summary drifted: %#v", release.Policy)
	}
	release.Policy.SentinelInPrimaryEstimand = true
	if err := release.Validate(plan, audit); err == nil {
		t.Fatal("v3 controlled release accepted the scarcity sentinel in the primary estimand")
	}
}

func TestControlledCorpusSpecAndDesignArtifactsAreFrozen(t *testing.T) {
	specFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := specFile.Close(); err != nil {
			t.Errorf("close corruption spec: %v", err)
		}
	}()
	spec, err := DecodeCorpusSpec(specFile)
	if err != nil {
		t.Fatal(err)
	}
	specDigest, err := digestJSON(spec)
	defaultSpec, defaultErr := DefaultCorpusSpec()
	defaultDigest, digestErr := digestJSON(defaultSpec)
	if err != nil || defaultErr != nil || digestErr != nil || specDigest != defaultDigest {
		t.Fatal("committed corruption corpus specification drifted from the executable default")
	}
	designFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "controlled-corruption-design.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := designFile.Close(); err != nil {
			t.Errorf("close corruption design: %v", err)
		}
	}()
	var design RelationDesign
	if err := decodeStrict(designFile, &design); err != nil {
		t.Fatal(err)
	}
	digest, err := digestJSON(design)
	if err != nil || digest != spec.DesignEvidenceDigest || design != spec.Design {
		t.Fatalf("committed controlled-relation design is not bound to the corpus spec: digest=%s err=%v", digest, err)
	}
}

func TestV2CorpusSpecRequiresExactProgramAuditBinding(t *testing.T) {
	spec, err := DefaultCorpusSpec()
	if err != nil {
		t.Fatal(err)
	}
	spec.CorpusVersion = "evalwitness-controlled-corruption.v2-development"
	spec.Seed = "evalwitness-controlled-corruption-v2-development-seed"
	spec.MutationProgramDigest = mutationProgramDigest(MutationProgramVersionV2)
	if err := spec.Validate(); err == nil {
		t.Fatal("v2 corpus spec accepted the historical v1 audit without a program binding")
	}
	spec.DevelopmentAudit.MutationProgramDigest = spec.MutationProgramDigest
	spec.DevelopmentAuditDigest, err = digestJSON(spec.DevelopmentAudit)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("v2 corpus spec accepted the historical v1 audit method")
	}
	spec.DevelopmentAudit.Method = "provider_free_full_corpus_build_construct_firewall_audit.v2"
	spec.DevelopmentAudit.AuditedReleaseDigest = ""
	spec.DevelopmentAudit.ObservedPositiveControls = 0
	spec.DevelopmentAudit.ConstructAuditDigest = digestText("construct-audit")
	spec.DevelopmentAuditDigest, err = digestJSON(spec.DevelopmentAudit)
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatal(err)
	}
	request := corpusApplyRequest(spec, FamilyNeutralFormatting, CorpusSource{})
	if request.Validator.Version != RelationContractVersionV2 || request.Validator.ContractDigest != spec.MutationProgramDigest {
		t.Fatalf("v2 corpus request did not route to the v2 relation contract: %#v", request.Validator)
	}
}

func TestCorpusLineageRejectsCrossSplitTaskAndPatchReuse(t *testing.T) {
	base := CorpusSource{
		ID: "source-" + digestText("trajectory-a"), TaskID: "task", RepositoryID: "repo", SourceFamily: "family",
		SourceFormat: preprocess.SourcePlainText, SourceLocation: "upstream/a", SourceRevision: "revision",
		SourceDigest: digestText("source-a"), TrajectoryDigest: digestText("trajectory-a"), PatchDigest: digestText("patch"),
		SplitGroupID: "group", NearDuplicateID: "near-" + digestText("task"), LineageClusterID: "lineage-" + digestText("cluster-a"), Split: "development",
		Outcome: SourceOutcome{Kind: "reward", Value: "1", WitnessDigest: digestText("outcome-a")},
		License: LicenseMetadata{SPDX: "MIT", SourceURL: "https://example.invalid/source", SourceRevision: "revision", Redistribution: "reference_only", Attribution: "fixture"},
		Privacy: PrivacyMetadata{Classification: "public", RedactionPolicyDigest: digestText("redaction"), PublicReleaseAllowed: true},
	}
	other := base
	other.ID = "source-" + digestText("trajectory-b")
	other.SourceDigest = digestText("source-b")
	other.TrajectoryDigest = digestText("trajectory-b")
	other.SourceLocation = "upstream/b"
	other.SplitGroupID = "other-group"
	other.LineageClusterID = "lineage-" + digestText("cluster-b")
	other.Split = "test"
	if err := validateCorpusSources([]CorpusSource{base, other}, "seed"); err == nil {
		t.Fatal("corruption corpus accepted task or patch lineage across splits")
	}
}

func TestCorpusLineagePlanKeepsRepositoryAndNearDuplicatesTogether(t *testing.T) {
	sources := []CorpusSource{
		{ID: "source-" + digestText("a"), RepositoryID: "shared-repository", TaskID: "task-a", SplitGroupID: "group-a", NearDuplicateID: "near-a", TrajectoryDigest: digestText("a")},
		{ID: "source-" + digestText("b"), RepositoryID: "shared-repository", TaskID: "task-b", SplitGroupID: "group-b", NearDuplicateID: "near-b", TrajectoryDigest: digestText("b")},
		{ID: "source-" + digestText("c"), RepositoryID: "other-repository", TaskID: "task-c", SplitGroupID: "group-c", NearDuplicateID: "near-b", TrajectoryDigest: digestText("c")},
	}
	plan := corpusLineagePlan(sources, "seed")
	if plan[sources[0].ID] != plan[sources[1].ID] || plan[sources[1].ID] != plan[sources[2].ID] {
		t.Fatalf("transitive repository and near-duplicate lineage was split: %#v", plan)
	}
}

func TestMutationInputAndValidatorOutputBoundsFailClosed(t *testing.T) {
	oversized := strings.NewReader(strings.Repeat("x", MaximumMutationDocumentSize+1))
	if _, err := DecodeCorpusSpec(oversized); err == nil {
		t.Fatal("mutation codec accepted an oversized document")
	}
	output := &boundedOutput{maximum: 4}
	if written, err := output.Write([]byte("12345")); err == nil || written != 4 || !output.exceeded || string(output.bytes) != "1234" {
		t.Fatalf("bounded validator output did not fail closed: written=%d exceeded=%t err=%v", written, output.exceeded, err)
	}
}

func FuzzMutationStrictDecoders(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schema_version":"unknown"}`))
	f.Add([]byte("[]\n{}"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		_, _ = DecodeManifest(strings.NewReader(string(raw)))
		_, _ = DecodeBlindReviewPacket(strings.NewReader(string(raw)))
		_, _ = DecodeCorpusSpec(strings.NewReader(string(raw)))
		_, _ = DecodeCorpusRelease(strings.NewReader(string(raw)))
	})
}

func TestHermeticRegistryRunsOnlyPinnedExecutableContract(t *testing.T) {
	if os.Getenv("EVALWITNESS_HERMETIC_TEST_HELPER") == "1" {
		if _, err := os.Stat("broken.marker"); err == nil {
			fmt.Print("fixture validator failed")
			os.Exit(1)
		} else if !errors.Is(err, os.ErrNotExist) {
			os.Exit(2)
		}
		fmt.Print("fixture validator passed")
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	validator := TrustedValidator{
		Spec: ValidatorSpec{
			ID: "evalwitness.hermetic-fixture", Version: "v1", Kind: ValidationHermetic,
			TimeoutMillis: 5_000, MaximumOutputBytes: 4_096,
		},
		Executable: executable, Arguments: []string{"-test.run=TestHermeticRegistryRunsOnlyPinnedExecutableContract"},
		Environment: SortedEnvironment(map[string]string{"EVALWITNESS_HERMETIC_TEST_HELPER": "1"}), PassingExitCode: 0,
	}
	validator.Spec.ContractDigest, err = TrustedValidatorContractDigest(validator)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewHermeticRegistry([]TrustedValidator{validator})
	if err != nil {
		t.Fatal(err)
	}
	request := validRequest(FamilyPatchHunkRemoval)
	request.RequiredFragment = "+new"
	request.Validator = validator.Spec
	originalRoot := t.TempDir()
	mutatedRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(mutatedRoot, "broken.marker"), []byte("controlled defect"), 0o600); err != nil {
		t.Fatal(err)
	}
	environments := TaskEnvironmentPair{
		Original: TaskEnvironment{ID: "fixture", Revision: "v1", Root: originalRoot, Disposable: true, NetworkDisabled: true},
		Mutated:  TaskEnvironment{ID: "fixture", Revision: "v1", Root: mutatedRoot, Disposable: true, NetworkDisabled: true},
	}
	proof, observation, err := registry.Observe(context.Background(), validator.Spec, environments)
	if err != nil || !observation.Passed || !observation.Original.Passed || observation.Mutated.Passed {
		t.Fatalf("hermetic outcome observation failed: result=%+v err=%v", observation, err)
	}
	request.OutcomeProof = &proof
	original := ingestFixture(t, "swe-bench.json")
	result, err := Apply(original, request)
	if err != nil {
		t.Fatal(err)
	}
	hermetic, err := registry.Run(context.Background(), result.Manifest, original, result.Mutated, environments)
	if err != nil || !hermetic.Passed || hermetic.Original.OutputBytes == 0 || hermetic.Mutated.OutputBytes == 0 {
		t.Fatalf("hermetic validation failed: result=%+v err=%v", hermetic, err)
	}
	tampered := result.Manifest
	tampered.Validator.ContractDigest = digestText("different trusted command")
	if _, err := registry.Run(context.Background(), tampered, original, result.Mutated, environments); err == nil {
		t.Fatal("hermetic registry accepted an unpinned validator contract")
	}
}

func TestFormalPositiveControlFixturesReproduceReleaseEvidence(t *testing.T) {
	open := func(name string) FormalControlProgram {
		t.Helper()
		file, err := os.Open(filepath.Join("..", "..", "eval", "fixtures", "controlled-corruption", name))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("close formal control fixture: %v", err)
			}
		}()
		program, err := DecodeFormalControl(file)
		if err != nil {
			t.Fatal(err)
		}
		return program
	}
	original := open("original.json")
	mutated := open("mutated.json")
	proof, err := ValidateFormalControlPair(original, mutated)
	if err != nil {
		t.Fatal(err)
	}
	control, err := FormalPositiveControl()
	if err != nil {
		t.Fatal(err)
	}
	if proof != control.OutcomeProof || control.Kind != "positive" {
		t.Fatalf("formal positive control drifted: proof=%+v control=%+v", proof, control)
	}
	if err := validateCorpusControls([]CorpusControl{control}); err != nil {
		t.Fatal(err)
	}
}
