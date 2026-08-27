package mutation

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

func TestConstructRepairCasesReproduceLegacyAcceptanceAndCorrectedRejection(t *testing.T) {
	cases, err := buildConstructRepairCases()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 {
		t.Fatalf("construct-repair cases = %d", len(cases))
	}
	for index, item := range cases {
		definition := constructRepairDefinitions[index]
		if item.LegacyManifest.Witness.LabelState != LabelProven || item.LegacyManifest.Program.Version != MutationProgramVersionV1 {
			t.Fatalf("legacy case %q was not accepted as proven v1 evidence", item.ID)
		}
		if item.CorrectedFirewall.Status != ConstructRejected || item.CorrectedFirewall.ProgramVersion != MutationProgramVersionV2 ||
			!slices.Equal(item.CorrectedFirewall.RejectionReasons, []ConstructRejectionReason{definition.rejection}) {
			t.Fatalf("corrected case %q did not reject under %q", item.ID, definition.rejection)
		}
	}
}

func TestConstructRepairEvidenceCodecSchemaAndTamperRejection(t *testing.T) {
	cases, err := buildConstructRepairCases()
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := SealConstructRepairEvidence(ConstructRepairEvidence{
		Corpus: ConstructRepairCorpusBinding{
			AuditedAt: "2026-08-09", PlanDigest: digestText("plan"), AuditDigest: digestText("audit"),
			ReleaseDigest: digestText("release"), MutationProgramDigest: mutationProgramDigest(MutationProgramVersionV2),
			Sources: 200, SourceTasks: 100, TotalAttempts: 873, AppliedAttempts: 738,
			RejectedAttempts: 135, SelectedCases: 320, CoverageCells: 16,
		},
		Cases: cases,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(evidence)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeConstructRepairEvidence(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != evidence.Digest {
		t.Fatalf("construct-repair evidence round trip failed: %v", err)
	}
	if _, err := Schema("construct-repair-evidence"); err != nil {
		t.Fatal(err)
	}

	var tampered map[string]any
	if err := json.Unmarshal(encoded, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["summary"].(map[string]any)["fixtures"] = float64(2)
	raw, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeConstructRepairEvidence(strings.NewReader(string(raw))); err == nil {
		t.Fatal("tampered construct-repair summary was accepted")
	}
	unknown := strings.Replace(string(encoded), `"digest": "`, `"unknown": true, "digest": "`, 1)
	if _, err := DecodeConstructRepairEvidence(strings.NewReader(unknown)); err == nil {
		t.Fatal("unknown construct-repair field was accepted")
	}
}

func TestV3RejectsVerificationNamesThatV2AdmitsWithoutInvocation(t *testing.T) {
	commands := []string{
		"echo go test ./...",
		"printf 'pytest completed'",
		"grep 'cargo test' README.md",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			trajectory := commandEvidenceTrajectory(t, command)
			legacy, err := ApplyV2(trajectory, constructRepairRequest(FamilyTestEvidenceOmitted))
			if err != nil {
				t.Fatal(err)
			}
			if legacy.Status != ConstructApplied {
				t.Fatalf("frozen v2 no longer reproduces the false acceptance: %#v", legacy)
			}
			outcome, err := ApplyV3(trajectory, constructRepairRequest(FamilyTestEvidenceOmitted))
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Status != ConstructRejected || !slices.Equal(outcome.Firewall.RejectionReasons, []ConstructRejectionReason{RejectionUnverifiedEvidenceRole}) {
				t.Fatalf("non-invoked verification name was admitted: %#v", outcome)
			}
			if outcome.Firewall.Invocation == nil || outcome.Firewall.Invocation.DirectInvocation {
				t.Fatalf("v3 rejection lacks a negative invocation proof: %#v", outcome.Firewall.Invocation)
			}
		})
	}
}

func TestV3RejectsTerminalCommandThatV2AdmitsAsNaturalFormatting(t *testing.T) {
	trajectory := assistantTextTrajectory(t, "$ go test ./... && echo finished because this exact terminal command must remain executable and should never be treated as natural assistant prose.")
	legacy, err := ApplyV2(trajectory, constructRepairRequest(FamilyNeutralFormatting))
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Status != ConstructApplied {
		t.Fatalf("frozen v2 no longer reproduces the terminal-command false acceptance: %#v", legacy)
	}
	outcome, err := ApplyV3(trajectory, constructRepairRequest(FamilyNeutralFormatting))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != ConstructRejected || !slices.Equal(outcome.Firewall.RejectionReasons, []ConstructRejectionReason{RejectionUnnaturalFormatting}) {
		t.Fatalf("terminal command was admitted as neutral formatting: %#v", outcome)
	}
	if outcome.Firewall.Presentation == nil || outcome.Firewall.Presentation.ContentKind != PresentationTerminalCommand {
		t.Fatalf("v3 rejection lacks terminal-command content-kind proof: %#v", outcome.Firewall.Presentation)
	}
}

func TestV3AdmitsDirectVerificationInvocationsAndAssistantProse(t *testing.T) {
	commands := []string{
		"go test ./...",
		"EVAL_MODE=closed env GOFLAGS=-count=1 go test ./...",
		"bash -lc 'python3 -m pytest -q'",
		"echo ready && cargo check --all-targets",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			outcome, err := ApplyV3(commandEvidenceTrajectory(t, command), constructRepairRequest(FamilyTestEvidenceOmitted))
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Status != ConstructApplied || outcome.Firewall.Invocation == nil || !outcome.Firewall.Invocation.DirectInvocation {
				t.Fatalf("direct verification invocation was rejected: %#v", outcome)
			}
		})
	}
	prose := "The implementation now preserves the complete evidence lineage, records each governed denominator, and explains the bounded result clearly enough for an independent reviewer."
	outcome, err := ApplyV3(assistantTextTrajectory(t, prose), constructRepairRequest(FamilyNeutralFormatting))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != ConstructApplied || outcome.Firewall.Presentation == nil || outcome.Firewall.Presentation.ContentKind != PresentationAssistantProse {
		t.Fatalf("natural assistant prose was rejected: %#v", outcome)
	}
}

func TestV3NegativePresentationProofRepresentsEmptyMessageRole(t *testing.T) {
	proof := presentationProof(preprocess.Event{
		ID: "event-empty-role",
		Message: &preprocess.MessagePayload{
			Parts: []preprocess.ContentPart{{Kind: preprocess.ContentText, Text: "not assistant prose"}},
		},
	})
	if proof.MessageRole != "<empty>" || proof.ContentKind != PresentationNonAssistantRole || proof.Decision != "message_role_not_assistant" {
		t.Fatalf("empty-role presentation proof is not explicit: %#v", proof)
	}
	report, err := sealConstructFirewallV3(ConstructFirewallReportV2{
		Family: FamilyNeutralFormatting, Status: ConstructRejected,
		SourceTrajectoryDigest: digestText("source"), TargetEventIDs: []string{proof.EventID}, ProofEventIDs: []string{proof.EventID},
		Presentation:     &proof,
		Checks:           []Check{{Name: "assistant_prose_content_kind", Expected: string(PresentationAssistantProse), Observed: string(proof.ContentKind), Passed: false}},
		RejectionReasons: []ConstructRejectionReason{RejectionUnnaturalFormatting},
	})
	if err != nil || report.Status != ConstructRejected {
		t.Fatalf("empty-role negative presentation proof did not seal: report=%#v err=%v", report, err)
	}
}

func TestV3NegativePresentationProofRepresentsMessageWithoutText(t *testing.T) {
	proof := presentationProof(preprocess.Event{
		ID: "event-without-text",
		Message: &preprocess.MessagePayload{
			Role: "assistant", Parts: []preprocess.ContentPart{{Kind: preprocess.ContentImage, Digest: digestText("image")}},
		},
	})
	if proof.TextPartCount != 0 || proof.ContentKind != PresentationUnknown || proof.Decision != "requires_one_nonempty_text_part" {
		t.Fatalf("no-text presentation proof is not explicit: %#v", proof)
	}
	_, err := sealConstructFirewallV3(ConstructFirewallReportV2{
		Family: FamilyNeutralFormatting, Status: ConstructRejected,
		SourceTrajectoryDigest: digestText("source"), TargetEventIDs: []string{proof.EventID}, ProofEventIDs: []string{proof.EventID},
		Presentation:     &proof,
		Checks:           []Check{{Name: "assistant_prose_content_kind", Expected: string(PresentationAssistantProse), Observed: string(proof.ContentKind), Passed: false}},
		RejectionReasons: []ConstructRejectionReason{RejectionUnnaturalFormatting},
	})
	if err != nil {
		t.Fatalf("no-text negative presentation proof did not seal: %v", err)
	}
}

func TestV3RejectsAssistantProseOutsideWrapEnvelope(t *testing.T) {
	outcome, err := ApplyV3(assistantTextTrajectory(t, "This assistant response is prose but deliberately too short to wrap."), constructRepairRequest(FamilyNeutralFormatting))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != ConstructRejected || outcome.Firewall.Presentation == nil ||
		outcome.Firewall.Presentation.ContentKind != PresentationAssistantProse ||
		outcome.Firewall.Presentation.Decision != "assistant_prose_outside_wrap_envelope" {
		t.Fatalf("short assistant prose did not produce a closed wrap-envelope rejection: %#v", outcome)
	}
}

func TestConstructFirewallV2CodecSchemaAndTamperRejection(t *testing.T) {
	outcome, err := ApplyV3(commandEvidenceTrajectory(t, "go test ./..."), constructRepairRequest(FamilyTestEvidenceOmitted))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(outcome.Firewall)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeConstructFirewallV2(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != outcome.Firewall.Digest {
		t.Fatalf("construct firewall v2 round trip failed: %v", err)
	}
	if _, err := Schema("construct-firewall-v2"); err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(encoded), `"direct_invocation": true`, `"direct_invocation": false`, 1)
	if _, err := DecodeConstructFirewallV2(strings.NewReader(tampered)); err == nil {
		t.Fatal("tampered v3 invocation proof was accepted")
	}
}

func TestConstructChallengeReproducesV2FalsificationAndV3Repair(t *testing.T) {
	evidence, err := BuildConstructChallengeEvidence()
	if err != nil {
		t.Fatal(err)
	}
	want := ConstructChallengeSummary{
		Cases: 14, V2Applied: 11, V2Rejected: 3, V3Applied: 6, V3Rejected: 8,
		V2FalseAcceptances: 5, V3RepairedNegatives: 5, PositiveControls: 6, SharedGuards: 3,
	}
	if evidence.Summary != want {
		t.Fatalf("construct challenge summary = %#v, want %#v", evidence.Summary, want)
	}
	for _, item := range evidence.Cases {
		switch item.Category {
		case ChallengeV2FalseAcceptance:
			if item.V2.Status != ConstructApplied || item.V3.Status != ConstructRejected {
				t.Fatalf("v2 false acceptance %q did not reproduce and repair", item.ID)
			}
		case ChallengePositiveControl:
			if item.V2.Status != ConstructApplied || item.V3.Status != ConstructApplied {
				t.Fatalf("positive control %q did not remain applied", item.ID)
			}
		case ChallengeSharedGuard:
			if item.V2.Status != ConstructRejected || item.V3.Status != ConstructRejected {
				t.Fatalf("shared guard %q changed status", item.ID)
			}
		default:
			t.Fatalf("unknown challenge category %q", item.Category)
		}
	}
}

func TestConstructChallengeCodecSchemaAndTamperRejection(t *testing.T) {
	evidence, err := BuildConstructChallengeEvidence()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(evidence)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeConstructChallengeEvidence(strings.NewReader(string(encoded)))
	if err != nil || decoded.Digest != evidence.Digest {
		t.Fatalf("construct challenge round trip failed: %v", err)
	}
	if _, err := Schema("construct-firewall-challenge"); err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(encoded), `"v3_repaired_negatives": 5`, `"v3_repaired_negatives": 4`, 1)
	if _, err := DecodeConstructChallengeEvidence(strings.NewReader(tampered)); err == nil {
		t.Fatal("tampered construct challenge summary was accepted")
	}
	unknown := strings.Replace(string(encoded), `"digest": "`, `"unknown": true, "digest": "`, 1)
	if _, err := DecodeConstructChallengeEvidence(strings.NewReader(unknown)); err == nil {
		t.Fatal("unknown construct challenge field was accepted")
	}
}

func TestV3FormalRelationAndPairOrderProofsReproduce(t *testing.T) {
	original := commandEvidenceTrajectory(t, "go test ./...")
	outcome, err := ApplyV3(original, constructRepairRequest(FamilyTestEvidenceOmitted))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied == nil {
		t.Fatal("v3 direct invocation did not produce an applied mutation")
	}
	if err := ValidateFormalRelation(outcome.Applied.Manifest, original, outcome.Applied.Mutated); err != nil {
		t.Fatalf("v3 formal relation did not reproduce: %v", err)
	}

	left := assistantTextTrajectory(t, "Candidate one provides a complete and independently verifiable implementation result with enough detail for stable pair-order testing.")
	right := assistantTextTrajectory(t, "Candidate two provides a complete and independently verifiable implementation result with enough detail for stable pair-order testing.")
	pair, err := ApplyCandidateOrderReversalV3(left, right, constructRepairRequest(FamilyCandidateOrderReversal))
	if err != nil {
		t.Fatal(err)
	}
	if pair.Status != ConstructApplied || pair.Applied == nil || pair.Applied.Manifest.Program.Version != MutationProgramVersionV3 || pair.Firewall.ProgramVersion != MutationProgramVersionV3 {
		t.Fatalf("v3 pair-order proof is incomplete: %#v", pair)
	}
}

func commandEvidenceTrajectory(t *testing.T, command string) preprocess.Trajectory {
	t.Helper()
	encodedCommand, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Join([]string{
		`{"type":"user","uuid":"user-1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"Run the public synthetic command."}}`,
		fmt.Sprintf(`{"type":"assistant","uuid":"assistant-1","parentUuid":"user-1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"shell","input":{"command":%s}}]}}`, encodedCommand),
		`{"type":"user","uuid":"result-1","parentUuid":"assistant-1","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"Completed successfully.","is_error":false}]}}`,
	}, "\n")
	trajectory, err := preprocess.IngestReader(strings.NewReader(raw), preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	return trajectory
}

func assistantTextTrajectory(t *testing.T, text string) preprocess.Trajectory {
	t.Helper()
	encoded, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprintf(`{"type":"assistant","uuid":"assistant-1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":%s}]}}`, encoded)
	trajectory, err := preprocess.IngestReader(strings.NewReader(raw), preprocess.DefaultIngestOptions())
	if err != nil {
		t.Fatal(err)
	}
	return trajectory
}
