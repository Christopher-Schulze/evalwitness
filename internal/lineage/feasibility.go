package lineage

import (
	"errors"
	"slices"
)

const CorpusFeasibilityDecisionVersion = "evalwitness.verification-lineage-corpus-feasibility.v1"

type CorpusFeasibilityThreshold struct {
	RequiredCalibrationTaskGroups int  `json:"required_calibration_task_groups"`
	RequiredLockedTestTaskGroups  int  `json:"required_locked_test_task_groups"`
	RequiredEligibleTaskGroups    int  `json:"required_eligible_task_groups"`
	AdmittedCalibrationTaskGroups int  `json:"admitted_calibration_task_groups"`
	AdmittedLockedTestTaskGroups  int  `json:"admitted_locked_test_task_groups"`
	AdmittedEligibleTaskGroups    int  `json:"admitted_eligible_task_groups"`
	EligibleTaskGroupShortfall    int  `json:"eligible_task_group_shortfall"`
	Passed                        bool `json:"passed"`
}

type CorpusFeasibilityClaimBoundary struct {
	EvidenceRole        DataRole `json:"evidence_role"`
	EmpiricalAuditRun   bool     `json:"empirical_audit_run"`
	FutureImpossibility bool     `json:"future_impossibility"`
	ProviderCalls       int      `json:"provider_calls"`
	AgentLaunches       int      `json:"agent_launches"`
	SupportedClaim      string   `json:"supported_claim"`
	UnsupportedClaims   []string `json:"unsupported_claims"`
}

type VerificationLineageCorpusFeasibilityDecision struct {
	Version                string                         `json:"version"`
	DecisionID             string                         `json:"decision_id"`
	PlanDigest             string                         `json:"plan_digest"`
	ParserLockDigest       string                         `json:"parser_lock_digest"`
	SourceReadinessDigest  string                         `json:"source_readiness_digest"`
	HoldoutReadinessDigest string                         `json:"holdout_readiness_digest"`
	Decision               string                         `json:"decision"`
	AcquisitionPerformed   bool                           `json:"acquisition_performed"`
	ThresholdWeakened      bool                           `json:"threshold_weakened"`
	PostOutcomeReplacement bool                           `json:"post_outcome_replacement"`
	Threshold              CorpusFeasibilityThreshold     `json:"threshold"`
	Reasons                []string                       `json:"reasons"`
	ProtocolV2Requirements []string                       `json:"protocol_v2_requirements"`
	ClaimBoundary          CorpusFeasibilityClaimBoundary `json:"claim_boundary"`
	Digest                 string                         `json:"digest"`
}

func BuildVerificationLineageCorpusFeasibilityDecision(repositoryRoot string) (VerificationLineageCorpusFeasibilityDecision, error) {
	plan, err := DefaultPlan()
	if err != nil {
		return VerificationLineageCorpusFeasibilityDecision{}, err
	}
	parserLock, err := BuildVerificationLineageParserLock(repositoryRoot)
	if err != nil {
		return VerificationLineageCorpusFeasibilityDecision{}, err
	}
	sourceReadiness, err := BuildVerificationLineageSourceReadinessAudit(repositoryRoot)
	if err != nil {
		return VerificationLineageCorpusFeasibilityDecision{}, err
	}
	holdoutReadiness, err := BuildVerificationLineageHoldoutReadinessAudit(repositoryRoot)
	if err != nil {
		return VerificationLineageCorpusFeasibilityDecision{}, err
	}
	threshold := CorpusFeasibilityThreshold{
		RequiredCalibrationTaskGroups: plan.MinimumSupport.CalibrationTaskGroups,
		RequiredLockedTestTaskGroups:  plan.MinimumSupport.TestTaskGroups,
		RequiredEligibleTaskGroups:    plan.MinimumSupport.CalibrationTaskGroups + plan.MinimumSupport.TestTaskGroups,
		AdmittedCalibrationTaskGroups: sourceReadiness.Summary.AdmittedTaskGroups,
		AdmittedLockedTestTaskGroups:  sourceReadiness.Summary.AdmittedTaskGroups,
		AdmittedEligibleTaskGroups:    sourceReadiness.Summary.AdmittedTaskGroups,
		EligibleTaskGroupShortfall:    plan.MinimumSupport.CalibrationTaskGroups + plan.MinimumSupport.TestTaskGroups,
	}
	decision := VerificationLineageCorpusFeasibilityDecision{
		Version: CorpusFeasibilityDecisionVersion, DecisionID: "task_069-corpus-feasibility-v1", PlanDigest: plan.Digest,
		ParserLockDigest: parserLock.Digest, SourceReadinessDigest: sourceReadiness.Digest, HoldoutReadinessDigest: holdoutReadiness.Digest,
		Decision: "not_feasible_current_generation", Threshold: threshold,
		Reasons: []string{"holdouts_not_validly_runnable", "no_admitted_calibration_sources", "no_admitted_locked_test_sources", "no_empirical_task_group_denominator"},
		ProtocolV2Requirements: []string{
			"acquire_and_admit_at_least_20_calibration_and_20_locked_test_task_groups",
			"freeze_syntax_family_candidate_universe_before_any_family_outcome",
			"isolate_selected_format_from_parser_and_mapping_development",
			"preserve_task_lineage_near_duplicate_and_source_session_role_firewalls",
		},
		ClaimBoundary: CorpusFeasibilityClaimBoundary{
			EvidenceRole: RoleAdapterDevelopment, ProviderCalls: 0, AgentLaunches: 0,
			SupportedClaim:    "the current sealed TASK-069 generation cannot satisfy its unchanged 20-calibration plus 20-locked-test threshold",
			UnsupportedClaims: []string{"future corpus impossibility", "population prevalence", "provider quality", "transfer performance"},
		},
	}
	decision.Digest, err = corpusFeasibilityDecisionDigest(decision)
	if err != nil {
		return VerificationLineageCorpusFeasibilityDecision{}, err
	}
	return decision, decision.Validate(repositoryRoot)
}

func (decision VerificationLineageCorpusFeasibilityDecision) Validate(repositoryRoot string) error {
	if decision.Version != CorpusFeasibilityDecisionVersion || decision.DecisionID != "task_069-corpus-feasibility-v1" ||
		decision.Decision != "not_feasible_current_generation" || decision.AcquisitionPerformed || decision.ThresholdWeakened || decision.PostOutcomeReplacement ||
		!validDigest(decision.PlanDigest) || !validDigest(decision.ParserLockDigest) || !validDigest(decision.SourceReadinessDigest) ||
		!validDigest(decision.HoldoutReadinessDigest) || !validDigest(decision.Digest) {
		return errors.New("verification-lineage corpus-feasibility identity or decision boundary is invalid")
	}
	wantThreshold := CorpusFeasibilityThreshold{
		RequiredCalibrationTaskGroups: 20, RequiredLockedTestTaskGroups: 20, RequiredEligibleTaskGroups: 40,
		AdmittedCalibrationTaskGroups: 0, AdmittedLockedTestTaskGroups: 0, AdmittedEligibleTaskGroups: 0,
		EligibleTaskGroupShortfall: 40, Passed: false,
	}
	if decision.Threshold != wantThreshold {
		return errors.New("verification-lineage corpus-feasibility threshold was weakened or miscounted")
	}
	if !slices.Equal(decision.Reasons, []string{"holdouts_not_validly_runnable", "no_admitted_calibration_sources", "no_admitted_locked_test_sources", "no_empirical_task_group_denominator"}) ||
		!slices.Equal(decision.ProtocolV2Requirements, []string{
			"acquire_and_admit_at_least_20_calibration_and_20_locked_test_task_groups",
			"freeze_syntax_family_candidate_universe_before_any_family_outcome",
			"isolate_selected_format_from_parser_and_mapping_development",
			"preserve_task_lineage_near_duplicate_and_source_session_role_firewalls",
		}) {
		return errors.New("verification-lineage corpus-feasibility reasons or recovery requirements changed")
	}
	plan, err := DefaultPlan()
	if err != nil {
		return err
	}
	parserLock, err := BuildVerificationLineageParserLock(repositoryRoot)
	if err != nil {
		return err
	}
	sourceReadiness, err := BuildVerificationLineageSourceReadinessAudit(repositoryRoot)
	if err != nil {
		return err
	}
	holdoutReadiness, err := BuildVerificationLineageHoldoutReadinessAudit(repositoryRoot)
	if err != nil {
		return err
	}
	if decision.PlanDigest != plan.Digest || decision.ParserLockDigest != parserLock.Digest || decision.SourceReadinessDigest != sourceReadiness.Digest ||
		decision.HoldoutReadinessDigest != holdoutReadiness.Digest {
		return errors.New("verification-lineage corpus-feasibility evidence binding is invalid")
	}
	if decision.ClaimBoundary.EvidenceRole != RoleAdapterDevelopment || decision.ClaimBoundary.EmpiricalAuditRun || decision.ClaimBoundary.FutureImpossibility ||
		decision.ClaimBoundary.ProviderCalls != 0 || decision.ClaimBoundary.AgentLaunches != 0 || decision.ClaimBoundary.SupportedClaim == "" ||
		!slices.Equal(decision.ClaimBoundary.UnsupportedClaims, []string{"future corpus impossibility", "population prevalence", "provider quality", "transfer performance"}) {
		return errors.New("verification-lineage corpus-feasibility claim boundary is invalid")
	}
	expectedDigest, err := corpusFeasibilityDecisionDigest(decision)
	if err != nil {
		return err
	}
	if decision.Digest != expectedDigest {
		return errors.New("verification-lineage corpus-feasibility digest is invalid")
	}
	return nil
}

func VerifyVerificationLineageCorpusFeasibilityDecision(repositoryRoot string, expected VerificationLineageCorpusFeasibilityDecision) error {
	if err := expected.Validate(repositoryRoot); err != nil {
		return err
	}
	actual, err := BuildVerificationLineageCorpusFeasibilityDecision(repositoryRoot)
	if err != nil {
		return err
	}
	if actual.Digest != expected.Digest {
		return errors.New("verification-lineage corpus-feasibility decision differs from sealed readiness evidence")
	}
	return nil
}

func corpusFeasibilityDecisionDigest(decision VerificationLineageCorpusFeasibilityDecision) (string, error) {
	decision.Digest = ""
	return digestJSON(decision)
}
