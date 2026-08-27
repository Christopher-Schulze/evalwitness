package lineage

import (
	"errors"
	"slices"
	"sort"
	"strings"
)

const SourceReadinessAuditVersion = "evalwitness.verification-lineage-source-readiness-audit.v1"

type SourceReadinessCandidate struct {
	CandidateID         string   `json:"candidate_id"`
	Materialized        bool     `json:"materialized"`
	DevelopmentUsable   bool     `json:"development_usable"`
	ResearchAdmitted    bool     `json:"research_admitted"`
	CalibrationEligible bool     `json:"calibration_eligible"`
	LockedTestEligible  bool     `json:"locked_test_eligible"`
	BlockingReasons     []string `json:"blocking_reasons"`
}

type SourceReadinessFormat struct {
	Format                string `json:"format"`
	CandidateSources      int    `json:"candidate_sources"`
	MaterializedSources   int    `json:"materialized_sources"`
	DevelopmentSources    int    `json:"development_sources"`
	ResearchSources       int    `json:"research_sources"`
	CalibrationTaskGroups int    `json:"calibration_task_groups"`
	LockedTestTaskGroups  int    `json:"locked_test_task_groups"`
	Inferential           bool   `json:"inferential"`
}

type SourceReadinessSummary struct {
	Candidates                    int `json:"candidates"`
	MaterializedCandidates        int `json:"materialized_candidates"`
	DevelopmentUsableCandidates   int `json:"development_usable_candidates"`
	ResearchAdmittedCandidates    int `json:"research_admitted_candidates"`
	CalibrationEligibleCandidates int `json:"calibration_eligible_candidates"`
	LockedTestEligibleCandidates  int `json:"locked_test_eligible_candidates"`
	AdmittedTaskGroups            int `json:"admitted_task_groups"`
}

type SourceReadinessClaimBoundary struct {
	EvidenceRole       DataRole `json:"evidence_role"`
	EmpiricalAuditRun  bool     `json:"empirical_audit_run"`
	PopulationEstimate bool     `json:"population_estimate"`
	ProviderCalls      int      `json:"provider_calls"`
	AgentLaunches      int      `json:"agent_launches"`
	SupportedClaim     string   `json:"supported_claim"`
	UnsupportedClaims  []string `json:"unsupported_claims"`
}

type VerificationLineageSourceReadinessAudit struct {
	Version                        string                       `json:"version"`
	AuditID                        string                       `json:"audit_id"`
	PlanDigest                     string                       `json:"plan_digest"`
	SourceInventoryDigest          string                       `json:"source_inventory_digest"`
	ParserLockDigest               string                       `json:"parser_lock_digest"`
	State                          string                       `json:"state"`
	TaskGroupCountsInspected       bool                         `json:"task_group_counts_inspected"`
	EmpiricalDenominatorsAvailable bool                         `json:"empirical_denominators_available"`
	ExternalActionStatus           string                       `json:"external_action_status"`
	Summary                        SourceReadinessSummary       `json:"summary"`
	Candidates                     []SourceReadinessCandidate   `json:"candidates"`
	Formats                        []SourceReadinessFormat      `json:"formats"`
	AcquisitionRequirements        []string                     `json:"acquisition_requirements"`
	ClaimBoundary                  SourceReadinessClaimBoundary `json:"claim_boundary"`
	Digest                         string                       `json:"digest"`
}

func BuildVerificationLineageSourceReadinessAudit(repositoryRoot string) (VerificationLineageSourceReadinessAudit, error) {
	inventory, err := DefaultSourceInventory()
	if err != nil {
		return VerificationLineageSourceReadinessAudit{}, err
	}
	parserLock, err := BuildVerificationLineageParserLock(repositoryRoot)
	if err != nil {
		return VerificationLineageSourceReadinessAudit{}, err
	}
	audit := VerificationLineageSourceReadinessAudit{
		Version: SourceReadinessAuditVersion, AuditID: "task_069-source-readiness-v1", PlanDigest: LockedPlanDigest,
		SourceInventoryDigest: inventory.Digest, ParserLockDigest: parserLock.Digest,
		State: "not_runnable_no_admitted_paired_research_source", TaskGroupCountsInspected: inventory.TaskGroupCountsInspected,
		EmpiricalDenominatorsAvailable: false, ExternalActionStatus: inventory.ExternalActionStatus,
		AcquisitionRequirements: []string{
			"exact_capture_command_and_export_identity", "explicit_owner_authorization_or_verified_publication_basis",
			"paired_execution_witness_or_authoritative_absence_surface", "pre_capture_privacy_and_redaction_policy",
			"source_license_and_redistribution_verification", "task_lineage_near_duplicate_and_role_assignment_before_content_inspection",
		},
		ClaimBoundary: SourceReadinessClaimBoundary{
			EvidenceRole: RoleAdapterDevelopment, ProviderCalls: 0, AgentLaunches: 0,
			SupportedClaim: "the sealed pre-acquisition inventory contains zero admitted calibration or locked-test sources and therefore no empirical TASK-069 denominator",
			UnsupportedClaims: []string{
				"agent verification prevalence", "corpus feasibility after acquisition", "format comparison", "population inference", "provider quality",
			},
		},
	}
	formats := make(map[string]*SourceReadinessFormat)
	for _, candidate := range inventory.Candidates {
		developmentUsable := candidate.Materialized && slices.Contains(candidate.PermittedRoles, RoleAdapterDevelopment)
		researchAdmitted := candidate.Materialized && candidate.AdmissionStatus == "admitted_research_corpus"
		calibrationEligible := researchAdmitted && slices.Contains(candidate.PermittedRoles, RoleCaptureCalibration)
		testEligible := researchAdmitted && slices.Contains(candidate.PermittedRoles, RoleLockedTest)
		finding := SourceReadinessCandidate{
			CandidateID: candidate.ID, Materialized: candidate.Materialized, DevelopmentUsable: developmentUsable,
			ResearchAdmitted: researchAdmitted, CalibrationEligible: calibrationEligible, LockedTestEligible: testEligible,
			BlockingReasons: append([]string(nil), candidate.Reasons...),
		}
		audit.Candidates = append(audit.Candidates, finding)
		audit.Summary.Candidates++
		if candidate.Materialized {
			audit.Summary.MaterializedCandidates++
		}
		if developmentUsable {
			audit.Summary.DevelopmentUsableCandidates++
		}
		if researchAdmitted {
			audit.Summary.ResearchAdmittedCandidates++
		}
		if calibrationEligible {
			audit.Summary.CalibrationEligibleCandidates++
		}
		if testEligible {
			audit.Summary.LockedTestEligibleCandidates++
		}
		for _, format := range candidate.Formats {
			entry, found := formats[format]
			if !found {
				entry = &SourceReadinessFormat{Format: format}
				formats[format] = entry
			}
			entry.CandidateSources++
			if candidate.Materialized {
				entry.MaterializedSources++
			}
			if developmentUsable {
				entry.DevelopmentSources++
			}
			if researchAdmitted {
				entry.ResearchSources++
			}
		}
	}
	formatIDs := make([]string, 0, len(formats))
	for format := range formats {
		formatIDs = append(formatIDs, format)
	}
	sort.Strings(formatIDs)
	for _, format := range formatIDs {
		audit.Formats = append(audit.Formats, *formats[format])
	}
	audit.Digest, err = sourceReadinessAuditDigest(audit)
	if err != nil {
		return VerificationLineageSourceReadinessAudit{}, err
	}
	return audit, audit.Validate()
}

func (audit VerificationLineageSourceReadinessAudit) Validate() error {
	if audit.Version != SourceReadinessAuditVersion || audit.AuditID != "task_069-source-readiness-v1" || audit.PlanDigest != LockedPlanDigest ||
		!validDigest(audit.SourceInventoryDigest) || !validDigest(audit.ParserLockDigest) ||
		audit.State != "not_runnable_no_admitted_paired_research_source" || audit.TaskGroupCountsInspected || audit.EmpiricalDenominatorsAvailable ||
		audit.ExternalActionStatus != ExternalActionNotAuthorized || !validDigest(audit.Digest) {
		return errors.New("verification-lineage source-readiness identity or acquisition boundary is invalid")
	}
	wantSummary := SourceReadinessSummary{Candidates: 5, MaterializedCandidates: 3, DevelopmentUsableCandidates: 3}
	if audit.Summary != wantSummary || len(audit.Candidates) != 5 || len(audit.Formats) != 5 {
		return errors.New("verification-lineage source-readiness denominators changed")
	}
	previous := ""
	expectedCandidates := []struct {
		id           string
		materialized bool
		development  bool
	}{
		{"checked_in_agent_trace_synthetic", true, true},
		{"checked_in_otlp_synthetic", true, true},
		{"checked_in_vendor_goldens", true, true},
		{"explicitly_authorized_owner_captures", false, false},
		{"public_licensed_native_exports", false, false},
	}
	for index, candidate := range audit.Candidates {
		if missing(candidate.CandidateID) || candidate.CandidateID <= previous || candidate.ResearchAdmitted || candidate.CalibrationEligible || candidate.LockedTestEligible || len(candidate.BlockingReasons) == 0 {
			return errors.New("verification-lineage source-readiness candidate findings are invalid or unsorted")
		}
		if candidate.CandidateID != expectedCandidates[index].id || candidate.Materialized != expectedCandidates[index].materialized || candidate.DevelopmentUsable != expectedCandidates[index].development {
			return errors.New("verification-lineage source-readiness candidate admission changed")
		}
		if err := validateSortedUnique("source-readiness blocking reasons", candidate.BlockingReasons, 1); err != nil {
			return err
		}
		previous = candidate.CandidateID
	}
	previous = ""
	expectedFormats := []SourceReadinessFormat{
		{Format: "agent_trace_json", CandidateSources: 1, MaterializedSources: 1, DevelopmentSources: 1},
		{Format: "claude_code_jsonl", CandidateSources: 3, MaterializedSources: 1, DevelopmentSources: 1},
		{Format: "codex_rollout_jsonl", CandidateSources: 3, MaterializedSources: 1, DevelopmentSources: 1},
		{Format: "opencode_export_json", CandidateSources: 3, MaterializedSources: 1, DevelopmentSources: 1},
		{Format: "otlp_json_genai", CandidateSources: 1, MaterializedSources: 1, DevelopmentSources: 1},
	}
	for index, format := range audit.Formats {
		if missing(format.Format) || format.Format <= previous || format.CandidateSources < 1 || format.MaterializedSources < 0 || format.DevelopmentSources < 0 ||
			format.ResearchSources != 0 || format.CalibrationTaskGroups != 0 || format.LockedTestTaskGroups != 0 || format.Inferential {
			return errors.New("verification-lineage source-readiness format findings are invalid or unsorted")
		}
		if format != expectedFormats[index] {
			return errors.New("verification-lineage source-readiness format denominator changed")
		}
		previous = format.Format
	}
	if err := validateSortedUnique("source-readiness acquisition requirements", audit.AcquisitionRequirements, 1); err != nil {
		return err
	}
	if audit.ClaimBoundary.EvidenceRole != RoleAdapterDevelopment || audit.ClaimBoundary.EmpiricalAuditRun || audit.ClaimBoundary.PopulationEstimate ||
		audit.ClaimBoundary.ProviderCalls != 0 || audit.ClaimBoundary.AgentLaunches != 0 || missing(audit.ClaimBoundary.SupportedClaim) ||
		len(audit.ClaimBoundary.UnsupportedClaims) == 0 {
		return errors.New("verification-lineage source-readiness claim boundary is invalid")
	}
	if err := validateSortedUnique("source-readiness unsupported claims", audit.ClaimBoundary.UnsupportedClaims, 1); err != nil {
		return err
	}
	if !strings.Contains(audit.ClaimBoundary.SupportedClaim, "zero admitted calibration or locked-test sources") {
		return errors.New("verification-lineage source-readiness supported claim was broadened")
	}
	expected, err := sourceReadinessAuditDigest(audit)
	if err != nil {
		return err
	}
	if audit.Digest != expected {
		return errors.New("verification-lineage source-readiness digest is invalid")
	}
	return nil
}

func VerifyVerificationLineageSourceReadinessAudit(repositoryRoot string, expected VerificationLineageSourceReadinessAudit) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	actual, err := BuildVerificationLineageSourceReadinessAudit(repositoryRoot)
	if err != nil {
		return err
	}
	if actual.Digest != expected.Digest {
		return errors.New("verification-lineage source-readiness audit differs from the sealed inventory and parser lock")
	}
	return nil
}

func sourceReadinessAuditDigest(audit VerificationLineageSourceReadinessAudit) (string, error) {
	audit.Digest = ""
	return digestJSON(audit)
}
