package mutation

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const ConstructRepairEvidenceSchemaVersion = "evalwitness.construct-repair-evidence.v1"

const (
	constructRepairEvidenceKind = "deterministic_public_regression_fixtures"
	constructRepairNotRun       = "not_run"
	constructRepairNotEstimated = "not_estimated"
	constructRepairClaim        = "v1 accepts each frozen synthetic regression fixture while v2 rejects it under the recorded closed reason"
)

type ConstructRepairCorpusBinding struct {
	AuditedAt             string `json:"audited_at"`
	PlanDigest            string `json:"plan_digest"`
	AuditDigest           string `json:"audit_digest"`
	ReleaseDigest         string `json:"release_digest"`
	MutationProgramDigest string `json:"mutation_program_digest"`
	Sources               int    `json:"sources"`
	SourceTasks           int    `json:"source_tasks"`
	TotalAttempts         int    `json:"total_attempts"`
	AppliedAttempts       int    `json:"applied_attempts"`
	RejectedAttempts      int    `json:"rejected_attempts"`
	SelectedCases         int    `json:"selected_cases"`
	CoverageCells         int    `json:"coverage_cells"`
}

type ConstructRepairCase struct {
	ID                       string                     `json:"id"`
	Family                   Family                     `json:"family"`
	FailureMode              string                     `json:"failure_mode"`
	RepairInvariant          string                     `json:"repair_invariant"`
	SourceTrajectoryDigest   string                     `json:"source_trajectory_digest"`
	LegacyManifest           Manifest                   `json:"legacy_manifest"`
	CorrectedFirewall        ConstructFirewallReport    `json:"corrected_firewall"`
	ExpectedRejectionReasons []ConstructRejectionReason `json:"expected_rejection_reasons"`
}

type ConstructRepairSummary struct {
	Fixtures          int `json:"fixtures"`
	LegacyAccepted    int `json:"legacy_accepted"`
	CorrectedRejected int `json:"corrected_rejected"`
}

type ConstructRepairClaimBoundary struct {
	EvidenceKind        string   `json:"evidence_kind"`
	ProviderCalls       string   `json:"provider_calls"`
	HumanReview         string   `json:"human_review"`
	PopulationInference string   `json:"population_inference"`
	SupportedClaim      string   `json:"supported_claim"`
	UnsupportedClaims   []string `json:"unsupported_claims"`
}

type ConstructRepairEvidence struct {
	SchemaVersion   string                       `json:"schema_version"`
	CanonicalPolicy string                       `json:"canonical_policy"`
	Corpus          ConstructRepairCorpusBinding `json:"corpus"`
	Cases           []ConstructRepairCase        `json:"cases"`
	Summary         ConstructRepairSummary       `json:"summary"`
	ClaimBoundary   ConstructRepairClaimBoundary `json:"claim_boundary"`
	Digest          string                       `json:"digest"`
}

type constructRepairDefinition struct {
	id              string
	family          Family
	failureMode     string
	repairInvariant string
	rejection       ConstructRejectionReason
	fixture         func() (preprocess.Trajectory, error)
}

var constructRepairDefinitions = []constructRepairDefinition{
	{
		id: "generic_completion_evidence_role", family: FamilyTestEvidenceOmitted,
		failureMode:     "operation_completion_misclassified_as_test_evidence",
		repairInvariant: "evidence_target_requires_test_check_build_or_outcome_probe_lineage",
		rejection:       RejectionUnverifiedEvidenceRole, fixture: constructRepairToolFixture,
	},
	{
		id: "pathological_executable_text_presentation", family: FamilyNeutralFormatting,
		failureMode:     "pathological_token_per_paragraph_presentation_admitted",
		repairInvariant: "natural_reversible_wrap_preserves_tokens_and_rejects_executable_text",
		rejection:       RejectionUnnaturalFormatting, fixture: constructRepairFormattingFixture,
	},
	{
		id: "shared_tool_transaction_reorder", family: FamilyCausalIndependentReorder,
		failureMode:     "reference_and_transaction_dependencies_omitted_from_independence_graph",
		repairInvariant: "full_transaction_reference_and_call_identity_dependency_closure",
		rejection:       RejectionTransactionDependency, fixture: constructRepairToolFixture,
	},
}

func BuildConstructRepairEvidence(plan CorpusDevelopmentPlan, audit CorpusDevelopmentAudit, release CorpusRelease) (ConstructRepairEvidence, error) {
	if err := VerifyCorpusV2AgainstAudit(release, plan, audit); err != nil {
		return ConstructRepairEvidence{}, fmt.Errorf("verify construct-repair corpus parents: %w", err)
	}
	cases, err := buildConstructRepairCases()
	if err != nil {
		return ConstructRepairEvidence{}, err
	}
	return SealConstructRepairEvidence(ConstructRepairEvidence{
		Corpus: ConstructRepairCorpusBinding{
			AuditedAt: audit.AuditedAt, PlanDigest: plan.Digest, AuditDigest: audit.Digest,
			ReleaseDigest: release.Digest, MutationProgramDigest: plan.MutationProgramDigest,
			Sources: len(audit.Sources), SourceTasks: audit.SourceTasks, TotalAttempts: audit.TotalAttempts,
			AppliedAttempts: audit.AppliedAttempts, RejectedAttempts: audit.RejectedAttempts,
			SelectedCases: audit.SelectedCases, CoverageCells: len(audit.Coverage),
		},
		Cases: cases,
	})
}

func VerifyConstructRepairEvidence(evidence ConstructRepairEvidence, plan CorpusDevelopmentPlan, audit CorpusDevelopmentAudit, release CorpusRelease) error {
	if err := evidence.Validate(); err != nil {
		return err
	}
	expected, err := BuildConstructRepairEvidence(plan, audit, release)
	if err != nil {
		return err
	}
	if evidence.Digest != expected.Digest {
		return errors.New("construct-repair evidence differs from reproduced corpus parents and regression fixtures")
	}
	return nil
}

func SealConstructRepairEvidence(evidence ConstructRepairEvidence) (ConstructRepairEvidence, error) {
	evidence.SchemaVersion = ConstructRepairEvidenceSchemaVersion
	evidence.CanonicalPolicy = CanonicalPolicy
	evidence.Summary = ConstructRepairSummary{Fixtures: len(evidence.Cases), LegacyAccepted: len(evidence.Cases), CorrectedRejected: len(evidence.Cases)}
	evidence.ClaimBoundary = expectedConstructRepairClaimBoundary()
	evidence.Digest = ""
	digest, err := evidence.digest()
	if err != nil {
		return ConstructRepairEvidence{}, err
	}
	evidence.Digest = digest
	if err := evidence.Validate(); err != nil {
		return ConstructRepairEvidence{}, err
	}
	return evidence, nil
}

func (evidence ConstructRepairEvidence) Validate() error {
	if evidence.SchemaVersion != ConstructRepairEvidenceSchemaVersion || evidence.CanonicalPolicy != CanonicalPolicy {
		return errors.New("construct-repair evidence identity is unsupported")
	}
	if err := validateConstructRepairCorpusBinding(evidence.Corpus); err != nil {
		return err
	}
	if evidence.Summary != (ConstructRepairSummary{Fixtures: 3, LegacyAccepted: 3, CorrectedRejected: 3}) || len(evidence.Cases) != 3 {
		return errors.New("construct-repair evidence must contain all three legacy-acceptance and corrected-rejection fixtures")
	}
	if !equalConstructRepairClaimBoundary(evidence.ClaimBoundary, expectedConstructRepairClaimBoundary()) {
		return errors.New("construct-repair scientific claim boundary is invalid")
	}
	for index, definition := range constructRepairDefinitions {
		if err := validateConstructRepairCase(evidence.Cases[index], definition); err != nil {
			return fmt.Errorf("construct-repair case %d: %w", index, err)
		}
	}
	expected, err := evidence.digest()
	if err != nil {
		return err
	}
	if evidence.Digest != expected {
		return errors.New("construct-repair evidence digest is invalid")
	}
	return nil
}

func buildConstructRepairCases() ([]ConstructRepairCase, error) {
	cases := make([]ConstructRepairCase, 0, len(constructRepairDefinitions))
	for _, definition := range constructRepairDefinitions {
		trajectory, err := definition.fixture()
		if err != nil {
			return nil, fmt.Errorf("build construct-repair fixture %q: %w", definition.id, err)
		}
		request := constructRepairRequest(definition.family)
		legacy, err := Apply(trajectory, request)
		if err != nil {
			return nil, fmt.Errorf("apply legacy construct-repair fixture %q: %w", definition.id, err)
		}
		if legacy.Manifest.Witness.LabelState != LabelProven {
			return nil, fmt.Errorf("legacy construct-repair fixture %q was not accepted as proven", definition.id)
		}
		if definition.family == FamilyCausalIndependentReorder {
			if len(legacy.Manifest.Affected.EventIDs) != 2 {
				return nil, errors.New("legacy reorder regression did not select exactly two events")
			}
			request.TargetEventID = legacy.Manifest.Affected.EventIDs[0]
		}
		corrected, err := ApplyV2(trajectory, request)
		if err != nil {
			return nil, fmt.Errorf("apply corrected construct-repair fixture %q: %w", definition.id, err)
		}
		if corrected.Status != ConstructRejected || corrected.Applied != nil || !slices.Equal(corrected.Firewall.RejectionReasons, []ConstructRejectionReason{definition.rejection}) {
			return nil, fmt.Errorf("corrected construct-repair fixture %q did not fail closed under %q", definition.id, definition.rejection)
		}
		cases = append(cases, ConstructRepairCase{
			ID: definition.id, Family: definition.family, FailureMode: definition.failureMode,
			RepairInvariant: definition.repairInvariant, SourceTrajectoryDigest: trajectory.Digest,
			LegacyManifest: legacy.Manifest, CorrectedFirewall: corrected.Firewall,
			ExpectedRejectionReasons: []ConstructRejectionReason{definition.rejection},
		})
	}
	return cases, nil
}

func validateConstructRepairCorpusBinding(binding ConstructRepairCorpusBinding) error {
	if _, err := time.Parse(time.DateOnly, binding.AuditedAt); err != nil {
		return errors.New("construct-repair corpus binding requires YYYY-MM-DD audited_at")
	}
	if !validDigest(binding.PlanDigest) || !validDigest(binding.AuditDigest) ||
		!validDigest(binding.ReleaseDigest) || !validDigest(binding.MutationProgramDigest) ||
		binding.MutationProgramDigest != mutationProgramDigest(MutationProgramVersionV2) ||
		binding.Sources < 1 || binding.SourceTasks < 1 || binding.TotalAttempts < 1 ||
		binding.AppliedAttempts < 1 || binding.RejectedAttempts < 1 || binding.SelectedCases < 1 || binding.CoverageCells < 1 ||
		binding.TotalAttempts != binding.AppliedAttempts+binding.RejectedAttempts || binding.SelectedCases > binding.AppliedAttempts {
		return errors.New("construct-repair corpus binding or denominators are invalid")
	}
	return nil
}

func validateConstructRepairCase(item ConstructRepairCase, definition constructRepairDefinition) error {
	if item.ID != definition.id || item.Family != definition.family || item.FailureMode != definition.failureMode ||
		item.RepairInvariant != definition.repairInvariant || !validDigest(item.SourceTrajectoryDigest) ||
		!slices.Equal(item.ExpectedRejectionReasons, []ConstructRejectionReason{definition.rejection}) {
		return errors.New("identity, family, failure mode, repair invariant, or expected rejection changed")
	}
	if err := item.LegacyManifest.Validate(); err != nil {
		return fmt.Errorf("legacy manifest: %w", err)
	}
	if item.LegacyManifest.Program.Version != MutationProgramVersionV1 || item.LegacyManifest.RelationContractVersion != RelationContractVersionV1 ||
		item.LegacyManifest.Program.Family != definition.family || item.LegacyManifest.Witness.LabelState != LabelProven ||
		item.LegacyManifest.OriginalTrajectoryDigest != item.SourceTrajectoryDigest {
		return errors.New("legacy observation is not a proven v1 acceptance of the frozen fixture")
	}
	if err := item.CorrectedFirewall.Validate(); err != nil {
		return fmt.Errorf("corrected firewall: %w", err)
	}
	if item.CorrectedFirewall.ProgramVersion != MutationProgramVersionV2 || item.CorrectedFirewall.Family != definition.family ||
		item.CorrectedFirewall.Status != ConstructRejected || item.CorrectedFirewall.SourceTrajectoryDigest != item.SourceTrajectoryDigest ||
		!slices.Equal(item.CorrectedFirewall.RejectionReasons, item.ExpectedRejectionReasons) {
		return errors.New("corrected observation is not the expected closed v2 rejection of the same fixture")
	}
	return nil
}

func constructRepairRequest(family Family) ApplyRequest {
	return ApplyRequest{
		CorpusVersion: "evalwitness-construct-repair-fixtures.v1", TaskID: "public-synthetic-regression",
		RepositoryID: "evalwitness", SourceFamily: "public-synthetic-construct-repair",
		SourceLocation: "eval/fixtures/construct-repair", SourceRevision: "v1", SplitGroupID: "public-synthetic-regression",
		Seed: "evalwitness-construct-repair-frozen-seed", Family: family,
		Outcome: SourceOutcome{Kind: "synthetic_regression", Value: "not_an_empirical_outcome", WitnessDigest: digestText("construct-repair-outcome")},
		Validator: ValidatorSpec{ID: "evalwitness.construct-repair", Version: "v1", Kind: ValidationPreservation,
			ContractDigest: digestText("construct-repair-validator"), TimeoutMillis: 1_000, MaximumOutputBytes: 64 * 1024},
		License: LicenseMetadata{SPDX: "CC0-1.0", SourceURL: "https://github.com/Christopher-Schulze/evalwitness",
			SourceRevision: "v1", Redistribution: "permitted", Attribution: "EvalWitness synthetic regression fixture"},
		Privacy:               PrivacyMetadata{Classification: "public", RedactionPolicyDigest: digestText("construct-repair-redaction"), PublicReleaseAllowed: true},
		ReviewSamplingStratum: "synthetic-regression",
	}
}

func constructRepairToolFixture() (preprocess.Trajectory, error) {
	raw := strings.Join([]string{
		`{"type":"user","uuid":"user-1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"Download the public synthetic fixture."}}`,
		`{"type":"assistant","uuid":"assistant-1","parentUuid":"user-1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"I will download the public synthetic fixture."},{"type":"tool_use","id":"tool-1","name":"shell","input":{"command":"python download_fixture.py"}}]}}`,
		`{"type":"user","uuid":"result-1","parentUuid":"assistant-1","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"Done!","is_error":false}]}}`,
	}, "\n")
	return preprocess.IngestReader(strings.NewReader(raw), preprocess.FrozenCanonicalizationV1IngestOptions())
}

func constructRepairFormattingFixture() (preprocess.Trajectory, error) {
	return preprocess.IngestString("Execute [/bin/sh] ps -ef | grep verifier-worker | grep -v grep || true because this command is copied exactly for terminal execution.", preprocess.FrozenCanonicalizationV1IngestOptions())
}

func expectedConstructRepairClaimBoundary() ConstructRepairClaimBoundary {
	return ConstructRepairClaimBoundary{
		EvidenceKind: constructRepairEvidenceKind, ProviderCalls: constructRepairNotRun,
		HumanReview: constructRepairNotRun, PopulationInference: constructRepairNotEstimated,
		SupportedClaim:    constructRepairClaim,
		UnsupportedClaims: []string{"human construct validity", "population prevalence of the defects", "provider or verifier performance"},
	}
}

func equalConstructRepairClaimBoundary(left, right ConstructRepairClaimBoundary) bool {
	return left.EvidenceKind == right.EvidenceKind && left.ProviderCalls == right.ProviderCalls && left.HumanReview == right.HumanReview &&
		left.PopulationInference == right.PopulationInference && left.SupportedClaim == right.SupportedClaim &&
		slices.Equal(left.UnsupportedClaims, right.UnsupportedClaims)
}

func (evidence ConstructRepairEvidence) digest() (string, error) {
	evidence.Digest = ""
	return digestJSON(evidence)
}
