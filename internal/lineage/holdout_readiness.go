package lineage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
)

const HoldoutReadinessAuditVersion = "evalwitness.verification-lineage-holdout-readiness-audit.v1"

type HoldoutSelectionCandidate struct {
	ID         string `json:"id"`
	RankDigest string `json:"rank_digest"`
	Selected   bool   `json:"selected"`
}

type HoldoutReadinessFinding struct {
	HoldoutID               string                      `json:"holdout_id"`
	Kind                    HoldoutKind                 `json:"kind"`
	SelectionSeed           string                      `json:"selection_seed"`
	SelectionRule           string                      `json:"selection_rule"`
	CandidateUniverseSealed bool                        `json:"candidate_universe_sealed"`
	Candidates              []HoldoutSelectionCandidate `json:"candidates"`
	SelectedID              string                      `json:"selected_id"`
	Status                  string                      `json:"status"`
	DevelopmentContaminated bool                        `json:"development_contaminated"`
	HeldOutSourceUnits      int                         `json:"held_out_source_units"`
	Predictions             int                         `json:"predictions"`
	LabeledOutcomes         int                         `json:"labeled_outcomes"`
	FalseAcceptances        int                         `json:"false_acceptances"`
	FalseRejections         int                         `json:"false_rejections"`
	Unresolved              int                         `json:"unresolved"`
	Unsupported             int                         `json:"unsupported"`
	Blockers                []string                    `json:"blockers"`
}

type HoldoutReadinessClaimBoundary struct {
	EvidenceRole      DataRole `json:"evidence_role"`
	HoldoutExecuted   bool     `json:"holdout_executed"`
	TransferClaim     bool     `json:"transfer_claim"`
	ProviderCalls     int      `json:"provider_calls"`
	AgentLaunches     int      `json:"agent_launches"`
	SupportedClaim    string   `json:"supported_claim"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

type VerificationLineageHoldoutReadinessAudit struct {
	Version          string                        `json:"version"`
	AuditID          string                        `json:"audit_id"`
	PlanDigest       string                        `json:"plan_digest"`
	ParserLockDigest string                        `json:"parser_lock_digest"`
	GoldenSetDigest  string                        `json:"golden_set_digest"`
	Findings         []HoldoutReadinessFinding     `json:"findings"`
	ClaimBoundary    HoldoutReadinessClaimBoundary `json:"claim_boundary"`
	Digest           string                        `json:"digest"`
}

func BuildVerificationLineageHoldoutReadinessAudit(repositoryRoot string) (VerificationLineageHoldoutReadinessAudit, error) {
	plan, err := DefaultPlan()
	if err != nil {
		return VerificationLineageHoldoutReadinessAudit{}, err
	}
	lock, err := BuildVerificationLineageParserLock(repositoryRoot)
	if err != nil {
		return VerificationLineageHoldoutReadinessAudit{}, err
	}
	golden, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		return VerificationLineageHoldoutReadinessAudit{}, err
	}
	formatPlan := plan.Holdouts[0]
	formatCandidates := rankHoldoutCandidates(formatPlan.SelectionSeed, []string{"claude_code_jsonl", "codex_rollout_jsonl", "opencode_export_json"})
	audit := VerificationLineageHoldoutReadinessAudit{
		Version: HoldoutReadinessAuditVersion, AuditID: "task_069-holdout-readiness-v1", PlanDigest: plan.Digest,
		ParserLockDigest: lock.Digest, GoldenSetDigest: golden.Digest,
		Findings: []HoldoutReadinessFinding{
			{
				HoldoutID: formatPlan.ID, Kind: formatPlan.Kind, SelectionSeed: formatPlan.SelectionSeed, SelectionRule: formatPlan.SelectionRule,
				CandidateUniverseSealed: true, Candidates: formatCandidates, SelectedID: selectedHoldoutCandidate(formatCandidates),
				Status: "not_runnable_development_contamination", DevelopmentContaminated: true,
				Blockers: []string{"all_candidate_format_mappings_used_during_development", "no_held_out_research_source_units"},
			},
			{
				HoldoutID: plan.Holdouts[1].ID, Kind: plan.Holdouts[1].Kind, SelectionSeed: plan.Holdouts[1].SelectionSeed, SelectionRule: plan.Holdouts[1].SelectionRule,
				Candidates: make([]HoldoutSelectionCandidate, 0),
				Status:     "not_runnable_unsealed_candidate_universe", DevelopmentContaminated: true,
				Blockers: []string{"no_pre_result_syntax_family_candidate_universe", "no_held_out_research_source_units", "syntax_cases_used_during_development"},
			},
		},
		ClaimBoundary: HoldoutReadinessClaimBoundary{
			EvidenceRole: RoleAdapterDevelopment, ProviderCalls: 0, AgentLaunches: 0,
			SupportedClaim:    "the deterministic format selection is reproducible, but neither planned holdout is executable as a valid transfer test under the sealed v1 evidence",
			UnsupportedClaims: []string{"format transfer", "held-out parser performance", "syntax-family transfer"},
		},
	}
	audit.Digest, err = holdoutReadinessAuditDigest(audit)
	if err != nil {
		return VerificationLineageHoldoutReadinessAudit{}, err
	}
	return audit, audit.Validate(repositoryRoot)
}

func (audit VerificationLineageHoldoutReadinessAudit) Validate(repositoryRoot string) error {
	if audit.Version != HoldoutReadinessAuditVersion || audit.AuditID != "task_069-holdout-readiness-v1" ||
		!validDigest(audit.PlanDigest) || !validDigest(audit.ParserLockDigest) || !validDigest(audit.GoldenSetDigest) || !validDigest(audit.Digest) {
		return errors.New("verification-lineage holdout-readiness identity is invalid")
	}
	if len(audit.Findings) != 2 || audit.Findings[0].HoldoutID != "format-holdout-v1" || audit.Findings[1].HoldoutID != "syntax-family-holdout-v1" {
		return errors.New("verification-lineage holdout-readiness findings are incomplete or unordered")
	}
	plan, err := DefaultPlan()
	if err != nil {
		return err
	}
	lock, err := BuildVerificationLineageParserLock(repositoryRoot)
	if err != nil {
		return err
	}
	golden, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		return err
	}
	if audit.PlanDigest != plan.Digest || audit.ParserLockDigest != lock.Digest || audit.GoldenSetDigest != golden.Digest {
		return errors.New("verification-lineage holdout-readiness evidence binding is invalid")
	}
	format := audit.Findings[0]
	if format.Kind != HoldoutFormat || format.SelectionSeed != plan.Holdouts[0].SelectionSeed || format.SelectionRule != plan.Holdouts[0].SelectionRule ||
		!format.CandidateUniverseSealed || len(format.Candidates) != 3 || format.SelectedID != "claude_code_jsonl" ||
		format.Status != "not_runnable_development_contamination" || !format.DevelopmentContaminated || !zeroHoldoutOutcomes(format) ||
		!slices.Equal(format.Blockers, []string{"all_candidate_format_mappings_used_during_development", "no_held_out_research_source_units"}) {
		return errors.New("verification-lineage format-holdout readiness was promoted or changed")
	}
	selected := 0
	previous := ""
	for _, candidate := range format.Candidates {
		if candidate.ID <= previous || !validDigest(candidate.RankDigest) {
			return errors.New("verification-lineage format-holdout candidates are invalid or unsorted")
		}
		if candidate.RankDigest != holdoutRankDigest(format.SelectionSeed, candidate.ID) {
			return errors.New("verification-lineage format-holdout rank digest is invalid")
		}
		if candidate.Selected {
			selected++
			if candidate.ID != format.SelectedID {
				return errors.New("verification-lineage format-holdout selection is inconsistent")
			}
		}
		previous = candidate.ID
	}
	if selected != 1 {
		return errors.New("verification-lineage format-holdout requires exactly one deterministic selection")
	}
	syntax := audit.Findings[1]
	if syntax.Kind != HoldoutSyntaxFamily || syntax.SelectionSeed != plan.Holdouts[1].SelectionSeed || syntax.SelectionRule != plan.Holdouts[1].SelectionRule ||
		syntax.CandidateUniverseSealed || len(syntax.Candidates) != 0 || syntax.SelectedID != "" ||
		syntax.Status != "not_runnable_unsealed_candidate_universe" || !syntax.DevelopmentContaminated || !zeroHoldoutOutcomes(syntax) ||
		!slices.Equal(syntax.Blockers, []string{"no_pre_result_syntax_family_candidate_universe", "no_held_out_research_source_units", "syntax_cases_used_during_development"}) {
		return errors.New("verification-lineage syntax-family holdout readiness was promoted or changed")
	}
	if audit.ClaimBoundary.EvidenceRole != RoleAdapterDevelopment || audit.ClaimBoundary.HoldoutExecuted || audit.ClaimBoundary.TransferClaim ||
		audit.ClaimBoundary.ProviderCalls != 0 || audit.ClaimBoundary.AgentLaunches != 0 ||
		audit.ClaimBoundary.SupportedClaim == "" || !slices.Equal(audit.ClaimBoundary.UnsupportedClaims, []string{"format transfer", "held-out parser performance", "syntax-family transfer"}) {
		return errors.New("verification-lineage holdout-readiness claim boundary is invalid")
	}
	expectedDigest, err := holdoutReadinessAuditDigest(audit)
	if err != nil {
		return err
	}
	if audit.Digest != expectedDigest {
		return errors.New("verification-lineage holdout-readiness digest is invalid")
	}
	return nil
}

func VerifyVerificationLineageHoldoutReadinessAudit(repositoryRoot string, expected VerificationLineageHoldoutReadinessAudit) error {
	if err := expected.Validate(repositoryRoot); err != nil {
		return err
	}
	actual, err := BuildVerificationLineageHoldoutReadinessAudit(repositoryRoot)
	if err != nil {
		return err
	}
	if actual.Digest != expected.Digest {
		return errors.New("verification-lineage holdout-readiness audit differs from the sealed plan and evidence")
	}
	return nil
}

func rankHoldoutCandidates(seed string, ids []string) []HoldoutSelectionCandidate {
	sorted := append([]string(nil), ids...)
	slices.Sort(sorted)
	result := make([]HoldoutSelectionCandidate, len(sorted))
	greatest := ""
	selected := -1
	for index, id := range sorted {
		digest := holdoutRankDigest(seed, id)
		result[index] = HoldoutSelectionCandidate{ID: id, RankDigest: digest}
		if digest > greatest {
			greatest = digest
			selected = index
		}
	}
	if selected >= 0 {
		result[selected].Selected = true
	}
	return result
}

func selectedHoldoutCandidate(candidates []HoldoutSelectionCandidate) string {
	for _, candidate := range candidates {
		if candidate.Selected {
			return candidate.ID
		}
	}
	return ""
}

func holdoutRankDigest(seed, id string) string {
	sum := sha256.Sum256([]byte(seed + "\x00" + id))
	return hex.EncodeToString(sum[:])
}

func zeroHoldoutOutcomes(finding HoldoutReadinessFinding) bool {
	return finding.HeldOutSourceUnits == 0 && finding.Predictions == 0 && finding.LabeledOutcomes == 0 &&
		finding.FalseAcceptances == 0 && finding.FalseRejections == 0 && finding.Unresolved == 0 && finding.Unsupported == 0
}

func holdoutReadinessAuditDigest(audit VerificationLineageHoldoutReadinessAudit) (string, error) {
	audit.Digest = ""
	return digestJSON(audit)
}
