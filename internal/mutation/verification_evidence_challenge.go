package mutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const VerificationEvidenceChallengeSchemaVersion = "evalwitness.verification-evidence-challenge.v1"

const (
	VerificationChallengeNaturalNegative = "source_derived_natural_negative"
	VerificationChallengePositiveControl = "positive_control"
	VerificationChallengeSyntheticGuard  = "synthetic_guard"
)

type VerificationEvidenceSourceBinding struct {
	MutationID       string `json:"mutation_id,omitempty"`
	TrajectoryDigest string `json:"trajectory_digest,omitempty"`
	TargetEventID    string `json:"target_event_id,omitempty"`
}

type VerificationEvidenceChallengeCase struct {
	ID                       string                                `json:"id"`
	Category                 string                                `json:"category"`
	Invariant                string                                `json:"invariant"`
	SourceBinding            *VerificationEvidenceSourceBinding    `json:"source_binding,omitempty"`
	FixtureTrajectoryDigest  string                                `json:"fixture_trajectory_digest"`
	ExpectedStatus           VerificationEvidenceStatus            `json:"expected_status"`
	ExpectedRejectionReasons []VerificationEvidenceRejectionReason `json:"expected_rejection_reasons"`
	Assessment               VerificationEvidenceAssessment        `json:"assessment"`
}

type VerificationEvidenceChallengeSummary struct {
	Cases                int `json:"cases"`
	NaturalNegatives     int `json:"natural_negatives"`
	PositiveControls     int `json:"positive_controls"`
	SyntheticGuards      int `json:"synthetic_guards"`
	Eligible             int `json:"eligible"`
	Rejected             int `json:"rejected"`
	ProvenanceFailures   int `json:"provenance_failures"`
	FailabilityFailures  int `json:"failability_failures"`
	EvidenceLossFailures int `json:"evidence_loss_failures"`
}

type VerificationEvidenceChallengeClaimBoundary struct {
	EvidenceKind        string   `json:"evidence_kind"`
	ProviderCalls       string   `json:"provider_calls"`
	HumanReview         string   `json:"human_review"`
	PopulationInference string   `json:"population_inference"`
	SupportedClaim      string   `json:"supported_claim"`
	UnsupportedClaims   []string `json:"unsupported_claims"`
}

type VerificationEvidenceChallenge struct {
	SchemaVersion     string                                     `json:"schema_version"`
	CanonicalPolicy   string                                     `json:"canonical_policy"`
	ClassifierVersion string                                     `json:"classifier_version"`
	Cases             []VerificationEvidenceChallengeCase        `json:"cases"`
	Summary           VerificationEvidenceChallengeSummary       `json:"summary"`
	ClaimBoundary     VerificationEvidenceChallengeClaimBoundary `json:"claim_boundary"`
	Digest            string                                     `json:"digest"`
}

type verificationEvidenceChallengeDefinition struct {
	id              string
	category        string
	invariant       string
	command         string
	result          string
	status          string
	exitCode        *int
	sourceBinding   *VerificationEvidenceSourceBinding
	expectedStatus  VerificationEvidenceStatus
	expectedReasons []VerificationEvidenceRejectionReason
}

var verificationEvidenceChallengeDefinitions = []verificationEvidenceChallengeDefinition{
	{
		id: "natural_diff_result_contains_agent_planning", category: VerificationChallengeNaturalNegative,
		invariant: "a linked result body containing agent planning prose is not subprocess verification evidence",
		command:   "diff -u /tmp/mystery.err /tmp/reversed.err",
		result:    "Invoking verification specialist\n\nI need to invoke the verification specialist before I can mark this task complete.",
		sourceBinding: &VerificationEvidenceSourceBinding{
			MutationID:       "mutation-a5d2ee38ab2bd1242971fefcda3eb9ccb3841f2ab45a9e2d1a40f90a0fe7088f",
			TrajectoryDigest: "0f11d96e23a90a764203428631fe859dccf2edc7ef8af268fddca904566f2264",
			TargetEventID:    "evt_ba1af769f28c3b552ff7c682",
		},
		expectedStatus:  VerificationEvidenceRejected,
		expectedReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened, VerificationEvidenceProvenanceUnbound},
	},
	{
		id: "natural_same_path_cmp_with_mixed_result", category: VerificationChallengeNaturalNegative,
		invariant: "a same-path comparison cannot falsify unchanged-file provenance and mixed narration is not command output",
		command:   "cmp -s /app/climate_analyzer/analyze_climate.py /app/climate_analyzer/analyze_climate.py && printf 'legacy_unchanged\\n'",
		result:    "legacy_unchanged\nI need a couple more concrete verification checks before I can mark this complete.",
		sourceBinding: &VerificationEvidenceSourceBinding{
			MutationID:       "mutation-e9ceeeab8f3950d79d5078a1615509db091ee9c33cd6ca8dd33a4d0adbcacbd4",
			TrajectoryDigest: "d6c74056df3d46612e9c85b34f9e3bdafd65e2f06d1a5f712d14c74ba8030c34",
			TargetEventID:    "evt_0e225420a21be646db0f3139",
		},
		expectedStatus:  VerificationEvidenceRejected,
		expectedReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened, VerificationEvidenceNonFailable, VerificationEvidenceProvenanceUnbound},
	},
	{
		id: "go_test_output", category: VerificationChallengePositiveControl,
		invariant: "recognized output from a directly invoked failable test remains eligible when text is the only decisive result channel",
		command:   "go test ./...", result: "ok  example/project  0.123s", expectedStatus: VerificationEvidenceEligible,
	},
	{
		id: "distinct_cmp_command_marker", category: VerificationChallengePositiveControl,
		invariant: "a success marker emitted after a comparison of distinct operands is bound and failable",
		command:   "cmp -s /tmp/before.bin /tmp/after.bin && printf 'files_equal\\n'", result: "files_equal\n", expectedStatus: VerificationEvidenceEligible,
	},
	{
		id: "retained_command_exit_code", category: VerificationChallengeSyntheticGuard,
		invariant: "text omission does not weaken a test claim when the same command exit code remains visible",
		command:   "go test ./...", result: "ok  example/project  0.123s", exitCode: intPointer(0),
		expectedStatus:  VerificationEvidenceRejected,
		expectedReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened},
	},
	{
		id: "printed_verification_name", category: VerificationChallengeSyntheticGuard,
		invariant: "printing a verification command name is not a verification invocation",
		command:   "echo go test ./...", result: "go test ./...\n", expectedStatus: VerificationEvidenceRejected,
		expectedReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened, VerificationEvidenceProvenanceUnbound, VerificationEvidenceInvocationUnverified},
	},
	{
		id: "checksum_without_check_mode", category: VerificationChallengeSyntheticGuard,
		invariant: "printing a digest without check mode cannot falsify an integrity claim",
		command:   "sha256sum artifact.bin", result: "0123456789abcdef  artifact.bin", expectedStatus: VerificationEvidenceRejected,
		expectedReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened, VerificationEvidenceNonFailable, VerificationEvidenceProvenanceUnbound},
	},
	{
		id: "distinct_cmp_marker_with_agent_narration", category: VerificationChallengeSyntheticGuard,
		invariant: "a valid command prefix does not sanitize agent narration appended to the same result body",
		command:   "cmp -s /tmp/before.bin /tmp/after.bin && printf 'files_equal\\n'", result: "files_equal\nI should run another check before finalizing.",
		expectedStatus:  VerificationEvidenceRejected,
		expectedReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened, VerificationEvidenceProvenanceUnbound},
	},
	{
		id: "structured_success_survives_text_omission", category: VerificationChallengeSyntheticGuard,
		invariant: "text omission does not weaken a claim when an explicit structured success status remains",
		command:   "go test ./...", result: "ok  example/project  0.123s", status: "success",
		expectedStatus:  VerificationEvidenceRejected,
		expectedReasons: []VerificationEvidenceRejectionReason{VerificationEvidenceNotWeakened},
	},
}

func BuildVerificationEvidenceChallenge() (VerificationEvidenceChallenge, error) {
	cases := make([]VerificationEvidenceChallengeCase, 0, len(verificationEvidenceChallengeDefinitions))
	for _, definition := range verificationEvidenceChallengeDefinitions {
		trajectory, targetEventID, err := verificationEvidenceFixture(definition.command, definition.result, definition.status, definition.exitCode)
		if err != nil {
			return VerificationEvidenceChallenge{}, fmt.Errorf("build verification-evidence fixture %q: %w", definition.id, err)
		}
		assessment, err := AssessVerificationEvidence(trajectory, targetEventID)
		if err != nil {
			return VerificationEvidenceChallenge{}, fmt.Errorf("assess verification-evidence fixture %q: %w", definition.id, err)
		}
		expectedReasons := append([]VerificationEvidenceRejectionReason{}, definition.expectedReasons...)
		cases = append(cases, VerificationEvidenceChallengeCase{
			ID: definition.id, Category: definition.category, Invariant: definition.invariant,
			SourceBinding:           cloneVerificationEvidenceSourceBinding(definition.sourceBinding),
			FixtureTrajectoryDigest: trajectory.Digest, ExpectedStatus: definition.expectedStatus,
			ExpectedRejectionReasons: expectedReasons,
			Assessment:               assessment,
		})
	}
	return SealVerificationEvidenceChallenge(VerificationEvidenceChallenge{Cases: cases})
}

func SealVerificationEvidenceChallenge(challenge VerificationEvidenceChallenge) (VerificationEvidenceChallenge, error) {
	challenge.SchemaVersion = VerificationEvidenceChallengeSchemaVersion
	challenge.CanonicalPolicy = CanonicalPolicy
	challenge.ClassifierVersion = VerificationEvidenceClassifierVersion
	challenge.Summary = summarizeVerificationEvidenceChallenge(challenge.Cases)
	challenge.ClaimBoundary = expectedVerificationEvidenceClaimBoundary()
	challenge.Digest = ""
	digest, err := challenge.digest()
	if err != nil {
		return VerificationEvidenceChallenge{}, err
	}
	challenge.Digest = digest
	if err := challenge.Validate(); err != nil {
		return VerificationEvidenceChallenge{}, err
	}
	return challenge, nil
}

func VerifyVerificationEvidenceChallenge(challenge VerificationEvidenceChallenge) error {
	if err := challenge.Validate(); err != nil {
		return err
	}
	expected, err := BuildVerificationEvidenceChallenge()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(challenge, expected) {
		return errors.New("verification-evidence challenge differs from reproduced fixtures")
	}
	return nil
}

func (challenge VerificationEvidenceChallenge) Validate() error {
	if challenge.SchemaVersion != VerificationEvidenceChallengeSchemaVersion || challenge.CanonicalPolicy != CanonicalPolicy || challenge.ClassifierVersion != VerificationEvidenceClassifierVersion {
		return errors.New("verification-evidence challenge identity is invalid")
	}
	if len(challenge.Cases) != len(verificationEvidenceChallengeDefinitions) || challenge.Summary != summarizeVerificationEvidenceChallenge(challenge.Cases) {
		return errors.New("verification-evidence challenge cases or summary are invalid")
	}
	wantSummary := VerificationEvidenceChallengeSummary{
		Cases: 9, NaturalNegatives: 2, PositiveControls: 2, SyntheticGuards: 5,
		Eligible: 2, Rejected: 7, ProvenanceFailures: 5, FailabilityFailures: 2, EvidenceLossFailures: 7,
	}
	if challenge.Summary != wantSummary {
		return fmt.Errorf("verification-evidence challenge exact denominators changed: got %#v", challenge.Summary)
	}
	if !reflect.DeepEqual(challenge.ClaimBoundary, expectedVerificationEvidenceClaimBoundary()) {
		return errors.New("verification-evidence challenge claim boundary is invalid")
	}
	for index, definition := range verificationEvidenceChallengeDefinitions {
		item := challenge.Cases[index]
		if item.ID != definition.id || item.Category != definition.category || item.Invariant != definition.invariant || !validDigest(item.FixtureTrajectoryDigest) || item.ExpectedStatus != definition.expectedStatus || !slices.Equal(item.ExpectedRejectionReasons, definition.expectedReasons) || !reflect.DeepEqual(item.SourceBinding, definition.sourceBinding) {
			return fmt.Errorf("verification-evidence challenge case %d identity or expectation changed", index)
		}
		if err := item.Assessment.Validate(); err != nil {
			return fmt.Errorf("verification-evidence challenge case %q: %w", item.ID, err)
		}
		if item.Assessment.SourceTrajectoryDigest != item.FixtureTrajectoryDigest || item.Assessment.Status != item.ExpectedStatus || !slices.Equal(item.Assessment.RejectionReasons, item.ExpectedRejectionReasons) {
			return fmt.Errorf("verification-evidence challenge case %q differs from its expectation", item.ID)
		}
		if err := validateVerificationEvidenceSourceBinding(item.SourceBinding, item.Category); err != nil {
			return fmt.Errorf("verification-evidence challenge case %q: %w", item.ID, err)
		}
	}
	expected, err := challenge.digest()
	if err != nil {
		return err
	}
	if challenge.Digest != expected {
		return errors.New("verification-evidence challenge digest is invalid")
	}
	return nil
}

func summarizeVerificationEvidenceChallenge(cases []VerificationEvidenceChallengeCase) VerificationEvidenceChallengeSummary {
	summary := VerificationEvidenceChallengeSummary{Cases: len(cases)}
	for _, item := range cases {
		switch item.Category {
		case VerificationChallengeNaturalNegative:
			summary.NaturalNegatives++
		case VerificationChallengePositiveControl:
			summary.PositiveControls++
		case VerificationChallengeSyntheticGuard:
			summary.SyntheticGuards++
		}
		if item.Assessment.Status == VerificationEvidenceEligible {
			summary.Eligible++
		} else {
			summary.Rejected++
		}
		if slices.Contains(item.Assessment.RejectionReasons, VerificationEvidenceProvenanceUnbound) {
			summary.ProvenanceFailures++
		}
		if slices.Contains(item.Assessment.RejectionReasons, VerificationEvidenceNonFailable) {
			summary.FailabilityFailures++
		}
		if slices.Contains(item.Assessment.RejectionReasons, VerificationEvidenceNotWeakened) {
			summary.EvidenceLossFailures++
		}
	}
	return summary
}

func expectedVerificationEvidenceClaimBoundary() VerificationEvidenceChallengeClaimBoundary {
	return VerificationEvidenceChallengeClaimBoundary{
		EvidenceKind:  "deterministic_source_derived_and_synthetic_minimal_pairs",
		ProviderCalls: "not_run", HumanReview: "not_run", PopulationInference: "not_estimated",
		SupportedClaim: "for the nine frozen challenge fixtures, the classifier rejects both source-derived false targets and all five synthetic guards while preserving both positive controls",
		UnsupportedClaims: []string{
			"defect prevalence in natural trajectories", "human construct validity", "provider or verifier performance",
			"universal shell-language completeness", "v4 corpus feasibility", "population generalization",
		},
	}
}

func validateVerificationEvidenceSourceBinding(binding *VerificationEvidenceSourceBinding, category string) error {
	if category != VerificationChallengeNaturalNegative {
		if binding != nil {
			return errors.New("only source-derived natural negatives may declare a source binding")
		}
		return nil
	}
	if binding == nil || !strings.HasPrefix(binding.MutationID, "mutation-") || !validDigest(strings.TrimPrefix(binding.MutationID, "mutation-")) || !validDigest(binding.TrajectoryDigest) || strings.TrimSpace(binding.TargetEventID) == "" {
		return errors.New("source-derived natural negative lacks an exact source binding")
	}
	return nil
}

func verificationEvidenceFixture(command, result, status string, exitCode *int) (preprocess.Trajectory, string, error) {
	encodedCommand, err := json.Marshal(command)
	if err != nil {
		return preprocess.Trajectory{}, "", err
	}
	encodedResult, err := json.Marshal(result)
	if err != nil {
		return preprocess.Trajectory{}, "", err
	}
	raw := fmt.Sprintf(`{"trial_id":"verification-evidence-challenge","trajectory":{"schema_version":"1","session_id":"fixture","steps":[{"step_id":1,"source":"agent","message":"Execute verification command","tool_calls":[{"tool_call_id":"call-1","function_name":"execute_bash","arguments":{"command":%s}}],"observation":{"results":[{"content":%s}]}}]}}`, encodedCommand, encodedResult)
	trajectory, err := preprocess.IngestReader(strings.NewReader(raw), preprocess.FrozenCanonicalizationV1IngestOptions())
	if err != nil {
		return preprocess.Trajectory{}, "", err
	}
	targetEventID := ""
	commandIndex := -1
	for index, event := range trajectory.Events {
		if event.ToolResult != nil {
			targetEventID = event.ID
		}
		if event.Command != nil {
			commandIndex = index
		}
	}
	if targetEventID == "" || commandIndex < 0 {
		return preprocess.Trajectory{}, "", errors.New("fixture lacks command or result event")
	}
	if status == "" && exitCode == nil {
		return trajectory, targetEventID, nil
	}
	events, err := cloneEvents(trajectory.Events)
	if err != nil {
		return preprocess.Trajectory{}, "", err
	}
	links := append([]preprocess.Link(nil), trajectory.Links...)
	changedEventIDs := make([]string, 0, 2)
	changedFieldPaths := make([]preprocess.FieldPath, 0, 2)
	if status != "" {
		for index := range events {
			if events[index].ID != targetEventID || events[index].ToolResult == nil {
				continue
			}
			originalID := events[index].ID
			events[index].ToolResult.Status = status
			rebuilt, rebuildErr := preprocess.RebuildDerivedEvent(trajectory.SourceFormat, events[index])
			if rebuildErr != nil {
				return preprocess.Trajectory{}, "", rebuildErr
			}
			events[index] = rebuilt
			links = remapLinks(links, originalID, rebuilt.ID)
			targetEventID = rebuilt.ID
			changedEventIDs = append(changedEventIDs, originalID)
			changedFieldPaths = append(changedFieldPaths, preprocess.FieldPath("/events/"+originalID+"/tool_result/status"))
		}
	}
	if exitCode != nil {
		originalID := events[commandIndex].ID
		exitCodeValue := *exitCode
		events[commandIndex].Command.ExitCode = &exitCodeValue
		rebuilt, rebuildErr := preprocess.RebuildDerivedEvent(trajectory.SourceFormat, events[commandIndex])
		if rebuildErr != nil {
			return preprocess.Trajectory{}, "", rebuildErr
		}
		events[commandIndex] = rebuilt
		links = remapLinks(links, originalID, rebuilt.ID)
		changedEventIDs = append(changedEventIDs, originalID)
		changedFieldPaths = append(changedFieldPaths, preprocess.FieldPath("/events/"+originalID+"/command/exit_code"))
	}
	derived, err := preprocess.DeriveTrajectory(trajectory, events, links, preprocess.DerivationSpec{
		Relation: "verification_evidence_fixture", Validator: VerificationEvidenceClassifierVersion,
		ChangedEventIDs: sortedStrings(changedEventIDs), ChangedFieldPaths: sortedFieldPaths(changedFieldPaths),
	})
	if err != nil {
		return preprocess.Trajectory{}, "", err
	}
	return derived, targetEventID, nil
}

func cloneVerificationEvidenceSourceBinding(binding *VerificationEvidenceSourceBinding) *VerificationEvidenceSourceBinding {
	if binding == nil {
		return nil
	}
	copy := *binding
	return &copy
}

func intPointer(value int) *int {
	return &value
}

func (challenge VerificationEvidenceChallenge) digest() (string, error) {
	challenge.Digest = ""
	return digestJSON(challenge)
}
