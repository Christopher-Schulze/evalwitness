package mutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const ConstructChallengeSchemaVersion = "evalwitness.construct-firewall-challenge.v1"

const (
	ChallengeV2FalseAcceptance = "v2_false_acceptance"
	ChallengePositiveControl   = "positive_control"
	ChallengeSharedGuard       = "shared_guard"
)

type ConstructChallengePrograms struct {
	LegacyVersion          string `json:"legacy_version"`
	LegacyProgramDigest    string `json:"legacy_program_digest"`
	LegacyFirewallSchema   string `json:"legacy_firewall_schema"`
	RepairedVersion        string `json:"repaired_version"`
	RepairedProgramDigest  string `json:"repaired_program_digest"`
	RepairedFirewallSchema string `json:"repaired_firewall_schema"`
	InvocationParser       string `json:"invocation_parser"`
	PresentationClassifier string `json:"presentation_classifier"`
}

type ConstructChallengeObservationV2 struct {
	Status   ConstructStatus         `json:"status"`
	Manifest *Manifest               `json:"manifest,omitempty"`
	Firewall ConstructFirewallReport `json:"firewall"`
}

type ConstructChallengeObservationV3 struct {
	Status   ConstructStatus           `json:"status"`
	Manifest *Manifest                 `json:"manifest,omitempty"`
	Firewall ConstructFirewallReportV2 `json:"firewall"`
}

type ConstructChallengeCase struct {
	ID                         string                          `json:"id"`
	Category                   string                          `json:"category"`
	Family                     Family                          `json:"family"`
	Invariant                  string                          `json:"invariant"`
	SourceTrajectoryDigest     string                          `json:"source_trajectory_digest"`
	ExpectedV2Status           ConstructStatus                 `json:"expected_v2_status"`
	ExpectedV2RejectionReasons []ConstructRejectionReason      `json:"expected_v2_rejection_reasons"`
	ExpectedV3Status           ConstructStatus                 `json:"expected_v3_status"`
	ExpectedV3RejectionReasons []ConstructRejectionReason      `json:"expected_v3_rejection_reasons"`
	V2                         ConstructChallengeObservationV2 `json:"v2"`
	V3                         ConstructChallengeObservationV3 `json:"v3"`
}

type ConstructChallengeSummary struct {
	Cases               int `json:"cases"`
	V2Applied           int `json:"v2_applied"`
	V2Rejected          int `json:"v2_rejected"`
	V3Applied           int `json:"v3_applied"`
	V3Rejected          int `json:"v3_rejected"`
	V2FalseAcceptances  int `json:"v2_false_acceptances"`
	V3RepairedNegatives int `json:"v3_repaired_negatives"`
	PositiveControls    int `json:"positive_controls"`
	SharedGuards        int `json:"shared_guards"`
}

type ConstructChallengeClaimBoundary struct {
	EvidenceKind        string   `json:"evidence_kind"`
	ProviderCalls       string   `json:"provider_calls"`
	HumanReview         string   `json:"human_review"`
	NaturalCorpusAudit  string   `json:"natural_corpus_audit"`
	PopulationInference string   `json:"population_inference"`
	SupportedClaim      string   `json:"supported_claim"`
	UnsupportedClaims   []string `json:"unsupported_claims"`
}

type ConstructChallengeEvidence struct {
	SchemaVersion   string                          `json:"schema_version"`
	CanonicalPolicy string                          `json:"canonical_policy"`
	Programs        ConstructChallengePrograms      `json:"programs"`
	Cases           []ConstructChallengeCase        `json:"cases"`
	Summary         ConstructChallengeSummary       `json:"summary"`
	ClaimBoundary   ConstructChallengeClaimBoundary `json:"claim_boundary"`
	Digest          string                          `json:"digest"`
}

type constructChallengeDefinition struct {
	id        string
	category  string
	family    Family
	invariant string
	v2Status  ConstructStatus
	v2Reasons []ConstructRejectionReason
	v3Status  ConstructStatus
	v3Reasons []ConstructRejectionReason
	fixture   func() (preprocess.Trajectory, error)
}

var constructChallengeDefinitions = []constructChallengeDefinition{
	{id: "printed_go_test_name", category: ChallengeV2FalseAcceptance, family: FamilyTestEvidenceOmitted, invariant: "printed verification names are not invocations", v2Status: ConstructApplied, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionUnverifiedEvidenceRole}, fixture: func() (preprocess.Trajectory, error) { return constructChallengeCommandFixture("echo go test ./...") }},
	{id: "formatted_pytest_name", category: ChallengeV2FalseAcceptance, family: FamilyTestEvidenceOmitted, invariant: "formatted verification names are not invocations", v2Status: ConstructApplied, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionUnverifiedEvidenceRole}, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeCommandFixture("printf 'pytest completed'")
	}},
	{id: "searched_cargo_test_name", category: ChallengeV2FalseAcceptance, family: FamilyTestEvidenceOmitted, invariant: "searched verification names are not invocations", v2Status: ConstructApplied, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionUnverifiedEvidenceRole}, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeCommandFixture("grep 'cargo test' README.md")
	}},
	{id: "assistant_terminal_command", category: ChallengeV2FalseAcceptance, family: FamilyNeutralFormatting, invariant: "terminal commands are not assistant prose", v2Status: ConstructApplied, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionUnnaturalFormatting}, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeAssistantFixture("$ go test ./... && echo finished because this exact terminal command must remain executable and should never be treated as natural assistant prose.")
	}},
	{id: "non_assistant_long_prose", category: ChallengeV2FalseAcceptance, family: FamilyNeutralFormatting, invariant: "presentation controls require assistant-role provenance", v2Status: ConstructApplied, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionUnnaturalFormatting}, fixture: func() (preprocess.Trajectory, error) {
		return preprocess.IngestString("This long user-authored paragraph contains enough words, punctuation, and line width to satisfy the lexical wrap envelope, but it is not assistant prose and must remain ineligible.", preprocess.FrozenCanonicalizationV1IngestOptions())
	}},
	{id: "direct_go_test", category: ChallengePositiveControl, family: FamilyTestEvidenceOmitted, invariant: "direct go test invocation remains eligible", v2Status: ConstructApplied, v3Status: ConstructApplied, fixture: func() (preprocess.Trajectory, error) { return constructChallengeCommandFixture("go test ./...") }},
	{id: "environment_wrapped_go_test", category: ChallengePositiveControl, family: FamilyTestEvidenceOmitted, invariant: "environment assignments and env wrapper preserve direct invocation", v2Status: ConstructApplied, v3Status: ConstructApplied, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeCommandFixture("EVAL_MODE=closed env GOFLAGS=-count=1 go test ./...")
	}},
	{id: "shell_wrapped_pytest", category: ChallengePositiveControl, family: FamilyTestEvidenceOmitted, invariant: "allowlisted shell wrapper is recursively parsed", v2Status: ConstructApplied, v3Status: ConstructApplied, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeCommandFixture("bash -lc 'python3 -m pytest -q'")
	}},
	{id: "compound_direct_cargo_check", category: ChallengePositiveControl, family: FamilyTestEvidenceOmitted, invariant: "a verification executable in a parsed compound segment remains direct", v2Status: ConstructApplied, v3Status: ConstructApplied, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeCommandFixture("echo ready && cargo check --all-targets")
	}},
	{id: "natural_assistant_prose", category: ChallengePositiveControl, family: FamilyNeutralFormatting, invariant: "natural assistant prose remains reversibly wrappable", v2Status: ConstructApplied, v3Status: ConstructApplied, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeAssistantFixture("The implementation now preserves the complete evidence lineage, records each governed denominator, and explains the bounded result clearly enough for an independent reviewer.")
	}},
	{id: "structured_json_guard", category: ChallengeSharedGuard, family: FamilyNeutralFormatting, invariant: "structured data remains outside the prose envelope", v2Status: ConstructRejected, v2Reasons: []ConstructRejectionReason{RejectionUnnaturalFormatting}, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionUnnaturalFormatting}, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeAssistantFixture(`{"command":"go test ./...","explanation":"This structured payload is intentionally long enough to satisfy the lexical envelope but must never be treated as prose."}`)
	}},
	{id: "code_fence_guard", category: ChallengeSharedGuard, family: FamilyNeutralFormatting, invariant: "code fences remain outside the prose envelope", v2Status: ConstructRejected, v2Reasons: []ConstructRejectionReason{RejectionUnnaturalFormatting}, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionUnnaturalFormatting}, fixture: func() (preprocess.Trajectory, error) {
		return constructChallengeAssistantFixture("```sh\ngo test ./...\n```\nThis fenced command remains executable evidence and is intentionally not a natural-prose formatting target.")
	}},
	{id: "shared_transaction_guard", category: ChallengeSharedGuard, family: FamilyCausalIndependentReorder, invariant: "shared call and reference transactions remain non-swappable", v2Status: ConstructRejected, v2Reasons: []ConstructRejectionReason{RejectionTransactionDependency}, v3Status: ConstructRejected, v3Reasons: []ConstructRejectionReason{RejectionTransactionDependency}, fixture: constructRepairToolFixture},
	{id: "independent_transaction_control", category: ChallengePositiveControl, family: FamilyCausalIndependentReorder, invariant: "distinct same-time source transactions remain swappable", v2Status: ConstructApplied, v3Status: ConstructApplied, fixture: constructChallengeIndependentFixture},
}

func BuildConstructChallengeEvidence() (ConstructChallengeEvidence, error) {
	cases := make([]ConstructChallengeCase, 0, len(constructChallengeDefinitions))
	for _, definition := range constructChallengeDefinitions {
		trajectory, err := definition.fixture()
		if err != nil {
			return ConstructChallengeEvidence{}, fmt.Errorf("build construct challenge fixture %q: %w", definition.id, err)
		}
		request := constructRepairRequest(definition.family)
		request.CorpusVersion = "evalwitness-construct-firewall-challenge.v1"
		request.SourceFamily = "public-synthetic-construct-challenge"
		request.SourceLocation = "eval/fixtures/construct-firewall-challenge"
		request.Seed = "evalwitness-construct-firewall-challenge-seed-" + definition.id
		if definition.id == "shared_transaction_guard" {
			legacy, applyErr := Apply(trajectory, request)
			if applyErr != nil || len(legacy.Manifest.Affected.EventIDs) != 2 {
				return ConstructChallengeEvidence{}, errors.New("derive shared-transaction challenge target")
			}
			request.TargetEventID = legacy.Manifest.Affected.EventIDs[0]
		}
		v2, err := ApplyV2(trajectory, request)
		if err != nil {
			return ConstructChallengeEvidence{}, fmt.Errorf("apply v2 challenge fixture %q: %w", definition.id, err)
		}
		v3, err := ApplyV3(trajectory, request)
		if err != nil {
			return ConstructChallengeEvidence{}, fmt.Errorf("apply v3 challenge fixture %q: %w", definition.id, err)
		}
		item := ConstructChallengeCase{
			ID: definition.id, Category: definition.category, Family: definition.family, Invariant: definition.invariant,
			SourceTrajectoryDigest: trajectory.Digest, ExpectedV2Status: definition.v2Status,
			ExpectedV2RejectionReasons: append([]ConstructRejectionReason(nil), definition.v2Reasons...),
			ExpectedV3Status:           definition.v3Status, ExpectedV3RejectionReasons: append([]ConstructRejectionReason(nil), definition.v3Reasons...),
			V2: challengeObservationV2(v2), V3: challengeObservationV3(v3),
		}
		cases = append(cases, item)
	}
	return SealConstructChallengeEvidence(ConstructChallengeEvidence{Cases: cases})
}

func VerifyConstructChallengeEvidence(evidence ConstructChallengeEvidence) error {
	if err := evidence.Validate(); err != nil {
		return err
	}
	expected, err := BuildConstructChallengeEvidence()
	if err != nil {
		return err
	}
	if evidence.Digest != expected.Digest {
		return errors.New("construct challenge differs from reproduced v2/v3 fixtures")
	}
	return nil
}

func SealConstructChallengeEvidence(evidence ConstructChallengeEvidence) (ConstructChallengeEvidence, error) {
	evidence.SchemaVersion = ConstructChallengeSchemaVersion
	evidence.CanonicalPolicy = CanonicalPolicy
	evidence.Programs = expectedConstructChallengePrograms()
	evidence.Summary = summarizeConstructChallenge(evidence.Cases)
	evidence.ClaimBoundary = expectedConstructChallengeClaimBoundary()
	evidence.Digest = ""
	digest, err := evidence.digest()
	if err != nil {
		return ConstructChallengeEvidence{}, err
	}
	evidence.Digest = digest
	if err := evidence.Validate(); err != nil {
		return ConstructChallengeEvidence{}, err
	}
	return evidence, nil
}

func (evidence ConstructChallengeEvidence) Validate() error {
	if evidence.SchemaVersion != ConstructChallengeSchemaVersion || evidence.CanonicalPolicy != CanonicalPolicy || evidence.Programs != expectedConstructChallengePrograms() {
		return errors.New("construct challenge identity or program bindings are invalid")
	}
	if len(evidence.Cases) != len(constructChallengeDefinitions) || evidence.Summary != summarizeConstructChallenge(evidence.Cases) || evidence.Summary != (ConstructChallengeSummary{Cases: 14, V2Applied: 11, V2Rejected: 3, V3Applied: 6, V3Rejected: 8, V2FalseAcceptances: 5, V3RepairedNegatives: 5, PositiveControls: 6, SharedGuards: 3}) {
		return errors.New("construct challenge cases or exact denominators are invalid")
	}
	if !equalConstructChallengeClaimBoundary(evidence.ClaimBoundary, expectedConstructChallengeClaimBoundary()) {
		return errors.New("construct challenge claim boundary is invalid")
	}
	for index, definition := range constructChallengeDefinitions {
		if err := validateConstructChallengeCase(evidence.Cases[index], definition); err != nil {
			return fmt.Errorf("construct challenge case %d: %w", index, err)
		}
	}
	expected, err := evidence.digest()
	if err != nil {
		return err
	}
	if evidence.Digest != expected {
		return errors.New("construct challenge digest is invalid")
	}
	return nil
}

func validateConstructChallengeCase(item ConstructChallengeCase, definition constructChallengeDefinition) error {
	if item.ID != definition.id || item.Category != definition.category || item.Family != definition.family || item.Invariant != definition.invariant || !validDigest(item.SourceTrajectoryDigest) || item.ExpectedV2Status != definition.v2Status || item.ExpectedV3Status != definition.v3Status || !slices.Equal(item.ExpectedV2RejectionReasons, definition.v2Reasons) || !slices.Equal(item.ExpectedV3RejectionReasons, definition.v3Reasons) {
		return errors.New("challenge identity, invariant, expectation, or source digest changed")
	}
	if err := validateChallengeObservationV2(item); err != nil {
		return err
	}
	if err := validateChallengeObservationV3(item); err != nil {
		return err
	}
	return nil
}

func validateChallengeObservationV2(item ConstructChallengeCase) error {
	observation := item.V2
	if observation.Status != item.ExpectedV2Status || observation.Firewall.Status != observation.Status || observation.Firewall.Family != item.Family || observation.Firewall.SourceTrajectoryDigest != item.SourceTrajectoryDigest || !slices.Equal(observation.Firewall.RejectionReasons, item.ExpectedV2RejectionReasons) {
		return errors.New("v2 challenge observation differs from its frozen expectation")
	}
	if err := observation.Firewall.Validate(); err != nil {
		return fmt.Errorf("v2 firewall: %w", err)
	}
	if observation.Status == ConstructApplied {
		if observation.Manifest == nil || observation.Manifest.Program.Version != MutationProgramVersionV2 || observation.Manifest.ConstructFirewallDigest != observation.Firewall.Digest || observation.Manifest.OriginalTrajectoryDigest != item.SourceTrajectoryDigest {
			return errors.New("applied v2 challenge observation lacks its exact manifest")
		}
		return observation.Manifest.Validate()
	}
	if observation.Manifest != nil {
		return errors.New("rejected v2 challenge observation contains a manifest")
	}
	return nil
}

func validateChallengeObservationV3(item ConstructChallengeCase) error {
	observation := item.V3
	if observation.Status != item.ExpectedV3Status || observation.Firewall.Status != observation.Status || observation.Firewall.Family != item.Family || observation.Firewall.SourceTrajectoryDigest != item.SourceTrajectoryDigest || !slices.Equal(observation.Firewall.RejectionReasons, item.ExpectedV3RejectionReasons) {
		return errors.New("v3 challenge observation differs from its frozen expectation")
	}
	if err := observation.Firewall.Validate(); err != nil {
		return fmt.Errorf("v3 firewall: %w", err)
	}
	if observation.Status == ConstructApplied {
		if observation.Manifest == nil || observation.Manifest.Program.Version != MutationProgramVersionV3 || observation.Manifest.ConstructFirewallDigest != observation.Firewall.Digest || observation.Manifest.OriginalTrajectoryDigest != item.SourceTrajectoryDigest {
			return errors.New("applied v3 challenge observation lacks its exact manifest")
		}
		return observation.Manifest.Validate()
	}
	if observation.Manifest != nil {
		return errors.New("rejected v3 challenge observation contains a manifest")
	}
	return nil
}

func challengeObservationV2(outcome ApplyV2Outcome) ConstructChallengeObservationV2 {
	result := ConstructChallengeObservationV2{Status: outcome.Status, Firewall: outcome.Firewall}
	if outcome.Applied != nil {
		manifest := outcome.Applied.Manifest
		result.Manifest = &manifest
	}
	return result
}

func challengeObservationV3(outcome ApplyV3Outcome) ConstructChallengeObservationV3 {
	result := ConstructChallengeObservationV3{Status: outcome.Status, Firewall: outcome.Firewall}
	if outcome.Applied != nil {
		manifest := outcome.Applied.Manifest
		result.Manifest = &manifest
	}
	return result
}

func summarizeConstructChallenge(cases []ConstructChallengeCase) ConstructChallengeSummary {
	summary := ConstructChallengeSummary{Cases: len(cases)}
	for _, item := range cases {
		if item.V2.Status == ConstructApplied {
			summary.V2Applied++
		} else {
			summary.V2Rejected++
		}
		if item.V3.Status == ConstructApplied {
			summary.V3Applied++
		} else {
			summary.V3Rejected++
		}
		switch item.Category {
		case ChallengeV2FalseAcceptance:
			summary.V2FalseAcceptances++
			if item.V3.Status == ConstructRejected {
				summary.V3RepairedNegatives++
			}
		case ChallengePositiveControl:
			summary.PositiveControls++
		case ChallengeSharedGuard:
			summary.SharedGuards++
		}
	}
	return summary
}

func expectedConstructChallengePrograms() ConstructChallengePrograms {
	return ConstructChallengePrograms{
		LegacyVersion: MutationProgramVersionV2, LegacyProgramDigest: mutationProgramDigest(MutationProgramVersionV2), LegacyFirewallSchema: ConstructFirewallSchemaVersion,
		RepairedVersion: MutationProgramVersionV3, RepairedProgramDigest: mutationProgramDigest(MutationProgramVersionV3), RepairedFirewallSchema: ConstructFirewallSchemaVersionV2,
		InvocationParser: InvocationParserVersion, PresentationClassifier: PresentationClassifierVersion,
	}
}

func expectedConstructChallengeClaimBoundary() ConstructChallengeClaimBoundary {
	return ConstructChallengeClaimBoundary{
		EvidenceKind: "deterministic_public_adversarial_minimal_pairs", ProviderCalls: "not_run", HumanReview: "not_run",
		NaturalCorpusAudit: "not_run", PopulationInference: "not_estimated",
		SupportedClaim:    "for the 14 frozen synthetic fixtures, v3 rejects all five recorded v2 false acceptances, preserves all six positive controls, and preserves all three shared guards",
		UnsupportedClaims: []string{"defect prevalence in natural trajectories", "human construct validity", "provider or verifier performance", "universal shell-language completeness", "population generalization"},
	}
}

func equalConstructChallengeClaimBoundary(left, right ConstructChallengeClaimBoundary) bool {
	return left.EvidenceKind == right.EvidenceKind && left.ProviderCalls == right.ProviderCalls && left.HumanReview == right.HumanReview && left.NaturalCorpusAudit == right.NaturalCorpusAudit && left.PopulationInference == right.PopulationInference && left.SupportedClaim == right.SupportedClaim && slices.Equal(left.UnsupportedClaims, right.UnsupportedClaims)
}

func constructChallengeCommandFixture(command string) (preprocess.Trajectory, error) {
	encoded, err := json.Marshal(command)
	if err != nil {
		return preprocess.Trajectory{}, err
	}
	raw := strings.Join([]string{
		`{"type":"user","uuid":"user-1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"Run the public synthetic command."}}`,
		fmt.Sprintf(`{"type":"assistant","uuid":"assistant-1","parentUuid":"user-1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"shell","input":{"command":%s}}]}}`, encoded),
		`{"type":"user","uuid":"result-1","parentUuid":"assistant-1","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"Completed successfully.","is_error":false}]}}`,
	}, "\n")
	return preprocess.IngestReader(strings.NewReader(raw), preprocess.FrozenCanonicalizationV1IngestOptions())
}

func constructChallengeAssistantFixture(text string) (preprocess.Trajectory, error) {
	encoded, err := json.Marshal(text)
	if err != nil {
		return preprocess.Trajectory{}, err
	}
	raw := fmt.Sprintf(`{"type":"assistant","uuid":"assistant-1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":%s}]}}`, encoded)
	return preprocess.IngestReader(strings.NewReader(raw), preprocess.FrozenCanonicalizationV1IngestOptions())
}

func constructChallengeIndependentFixture() (preprocess.Trajectory, error) {
	raw := strings.Join([]string{
		`{"type":"assistant","uuid":"assistant-1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":"First independent public synthetic observation."}}`,
		`{"type":"assistant","uuid":"assistant-2","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":"Second independent public synthetic observation."}}`,
	}, "\n")
	return preprocess.IngestReader(strings.NewReader(raw), preprocess.FrozenCanonicalizationV1IngestOptions())
}

func (evidence ConstructChallengeEvidence) digest() (string, error) {
	evidence.Digest = ""
	return digestJSON(evidence)
}
