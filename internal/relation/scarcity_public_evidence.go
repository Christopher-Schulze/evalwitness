package relation

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const (
	ScarcityPublicEvidenceSchemaVersion = "evalwitness.relation-scarcity-public-evidence.v1"
	ScarcityPublicEvidenceKind          = "deterministic_public_negative_evidence"
)

type ScarcityPublicClaimStatus string

const (
	ScarcityPublicClaimSupported     ScarcityPublicClaimStatus = "supported"
	ScarcityPublicClaimUnsupported   ScarcityPublicClaimStatus = "unsupported"
	ScarcityPublicClaimNotRun        ScarcityPublicClaimStatus = "not_run"
	ScarcityPublicClaimNotMeasured   ScarcityPublicClaimStatus = "not_measured"
	ScarcityPublicClaimNotAuthorized ScarcityPublicClaimStatus = "not_authorized"
)

type ScarcityPublicAvailability struct {
	Target     int  `json:"target"`
	Attempted  int  `json:"attempted"`
	Admitted   int  `json:"admitted"`
	Rejected   int  `json:"rejected"`
	Selected   int  `json:"selected"`
	Shortfall  int  `json:"shortfall"`
	Exhaustive bool `json:"exhaustive"`
}

type ScarcityPublicStudyRoles struct {
	Development            int `json:"development"`
	Calibration            int `json:"calibration"`
	Test                   int `json:"test"`
	PrimaryEstimandOverlap int `json:"primary_estimand_overlap"`
}

type ScarcityPublicCore struct {
	Families       int `json:"families"`
	CasesPerFamily int `json:"cases_per_family"`
	Cases          int `json:"cases"`
}

type ScarcityPublicPrimary struct {
	Cases           int `json:"cases"`
	TaskGroups      int `json:"task_groups"`
	LineageClusters int `json:"lineage_clusters"`
}

type ScarcityPublicCoverage struct {
	SourceFormat       preprocess.SourceFormat `json:"source_format"`
	Attempted          int                     `json:"attempted"`
	Admitted           int                     `json:"admitted"`
	Rejected           int                     `json:"rejected"`
	EligibleTaskGroups int                     `json:"eligible_task_groups"`
	Selected           int                     `json:"selected"`
}

type ScarcityPublicCase struct {
	Index                   int      `json:"index"`
	DataRole                string   `json:"data_role"`
	Unit                    UnitType `json:"unit"`
	CaseBindingDigest       string   `json:"case_binding_digest"`
	ConstructFirewallDigest string   `json:"construct_firewall_digest"`
}

type ScarcityPublicParent struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type ScarcityPublicClaim struct {
	ID       string                    `json:"id"`
	Claim    string                    `json:"claim"`
	Status   ScarcityPublicClaimStatus `json:"status"`
	Evidence string                    `json:"evidence"`
}

type ScarcityPublicEvidence struct {
	SchemaVersion    string                     `json:"schema_version"`
	CanonicalPolicy  string                     `json:"canonical_policy"`
	BriefPolicy      string                     `json:"brief_policy"`
	EvidenceKind     string                     `json:"evidence_kind"`
	ConstructFamily  mutation.Family            `json:"construct_family"`
	Availability     ScarcityPublicAvailability `json:"availability"`
	StudyRoles       ScarcityPublicStudyRoles   `json:"study_roles"`
	InferentialCore  ScarcityPublicCore         `json:"inferential_core"`
	PrimarySample    ScarcityPublicPrimary      `json:"primary_sample"`
	Coverage         []ScarcityPublicCoverage   `json:"coverage"`
	RejectionReasons []mutation.CorpusCount     `json:"rejection_reasons"`
	Cases            []ScarcityPublicCase       `json:"cases"`
	Parents          []ScarcityPublicParent     `json:"parents"`
	Claims           []ScarcityPublicClaim      `json:"claims"`
	Digest           string                     `json:"digest"`
}

func BuildScarcityPublicEvidence(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3) (ScarcityPublicEvidence, error) {
	if err := VerifyScarcityPublicEvidence(plan, primary, sentinel, corpusPlan, audit, release); err != nil {
		return ScarcityPublicEvidence{}, err
	}
	funnel, err := buildScarcityFunnel(audit, sentinel.Family)
	if err != nil {
		return ScarcityPublicEvidence{}, err
	}
	evidence := ScarcityPublicEvidence{
		ConstructFamily: sentinel.Family,
		Availability:    scarcityPublicAvailability(funnel, sentinel, release, audit),
		StudyRoles: ScarcityPublicStudyRoles{
			Development:            relationSplitCount(sentinel.SplitCounts, "development"),
			Calibration:            relationSplitCount(sentinel.SplitCounts, "calibration"),
			Test:                   relationSplitCount(sentinel.SplitCounts, "test"),
			PrimaryEstimandOverlap: sentinel.PrimaryOverlap,
		},
		InferentialCore: ScarcityPublicCore{
			Families: len(release.Policy.InferentialCoreFamilies), CasesPerFamily: release.Policy.CoreCasesPerFamily, Cases: release.Policy.CoreCases,
		},
		PrimarySample: ScarcityPublicPrimary{
			Cases: primary.SelectedCases, TaskGroups: primary.UniqueTaskGroups, LineageClusters: primary.UniqueLineageClusters,
		},
		Coverage: scarcityPublicCoverage(funnel), RejectionReasons: funnel.RejectionReasons,
		Cases: scarcityPublicCases(sentinel), Parents: scarcityPublicParents(plan, primary, sentinel, corpusPlan, audit, release),
	}
	return SealScarcityPublicEvidence(evidence)
}

func VerifyScarcityPublicEvidenceDocument(evidence ScarcityPublicEvidence, plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3) error {
	if err := evidence.Validate(); err != nil {
		return err
	}
	expected, err := BuildScarcityPublicEvidence(plan, primary, sentinel, corpusPlan, audit, release)
	if err != nil {
		return err
	}
	if evidence.Digest != expected.Digest {
		return errors.New("scarcity public evidence differs from its reproduced public parents")
	}
	return nil
}

func SealScarcityPublicEvidence(evidence ScarcityPublicEvidence) (ScarcityPublicEvidence, error) {
	evidence.SchemaVersion = ScarcityPublicEvidenceSchemaVersion
	evidence.CanonicalPolicy = CanonicalPolicy
	evidence.BriefPolicy = ScarcityPublicBriefPolicy
	evidence.EvidenceKind = ScarcityPublicEvidenceKind
	evidence.Claims = expectedScarcityPublicClaims()
	evidence.Digest = ""
	digest, err := evidence.digest()
	if err != nil {
		return ScarcityPublicEvidence{}, err
	}
	evidence.Digest = digest
	return evidence, evidence.Validate()
}

func (evidence ScarcityPublicEvidence) Validate() error {
	if evidence.SchemaVersion != ScarcityPublicEvidenceSchemaVersion || evidence.CanonicalPolicy != CanonicalPolicy ||
		evidence.BriefPolicy != ScarcityPublicBriefPolicy || evidence.EvidenceKind != ScarcityPublicEvidenceKind ||
		evidence.ConstructFamily != mutation.FamilyTestEvidenceOmitted {
		return errors.New("scarcity public evidence identity is unsupported")
	}
	for _, validate := range []func() error{
		func() error { return validateScarcityAvailability(evidence.Availability) },
		func() error { return validateScarcityRoles(evidence.StudyRoles, evidence.Availability.Selected) },
		func() error { return validateScarcityCore(evidence.InferentialCore, evidence.Availability.Target) },
		func() error { return validateScarcityPrimary(evidence.PrimarySample) },
		func() error { return validateScarcityCoverage(evidence.Coverage, evidence.Availability) },
		func() error {
			return validateScarcityRejections(evidence.RejectionReasons, evidence.Availability.Rejected)
		},
		func() error {
			return validateScarcityCases(evidence.Cases, evidence.StudyRoles, evidence.Availability.Selected)
		},
		func() error { return validateScarcityParents(evidence.Parents) },
	} {
		if err := validate(); err != nil {
			return err
		}
	}
	if !slices.Equal(evidence.Claims, expectedScarcityPublicClaims()) {
		return errors.New("scarcity public evidence claim boundary is invalid")
	}
	expected, err := evidence.digest()
	if err != nil || evidence.Digest != expected {
		return errors.New("scarcity public evidence digest is invalid")
	}
	return nil
}

func scarcityPublicAvailability(funnel scarcityFunnel, sentinel ScarcitySentinelV3, release mutation.CorpusReleaseV3, audit mutation.CorpusDevelopmentAuditV3) ScarcityPublicAvailability {
	return ScarcityPublicAvailability{
		Target: release.Policy.CoreCasesPerFamily, Attempted: funnel.Attempted, Admitted: funnel.Applied,
		Rejected: funnel.Rejected, Selected: funnel.Selected, Shortfall: auditShortfall(audit, string(sentinel.Family)), Exhaustive: sentinel.Exhaustive,
	}
}

func scarcityPublicCoverage(funnel scarcityFunnel) []ScarcityPublicCoverage {
	coverage := make([]ScarcityPublicCoverage, 0, len(funnel.Coverage))
	for _, row := range funnel.Coverage {
		coverage = append(coverage, ScarcityPublicCoverage{
			SourceFormat: row.SourceFormat, Attempted: row.Attempted, Admitted: row.Applied,
			Rejected: row.Rejected, EligibleTaskGroups: row.EligibleTaskGroups, Selected: row.SelectedCases,
		})
	}
	sort.Slice(coverage, func(left, right int) bool { return coverage[left].SourceFormat < coverage[right].SourceFormat })
	return coverage
}

func scarcityPublicCases(sentinel ScarcitySentinelV3) []ScarcityPublicCase {
	cases := make([]ScarcityPublicCase, 0, len(sentinel.Cases))
	for index, item := range sentinel.Cases {
		cases = append(cases, ScarcityPublicCase{
			Index: index + 1, DataRole: string(item.DataRole), Unit: item.Unit,
			CaseBindingDigest: item.CaseBindingDigest, ConstructFirewallDigest: item.ConstructFirewallDigest,
		})
	}
	return cases
}

func scarcityPublicParents(plan RelationPlanV3, primary PrimarySampleV3, sentinel ScarcitySentinelV3, corpusPlan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3) []ScarcityPublicParent {
	return []ScarcityPublicParent{
		{ID: "corpus_development_plan", Digest: corpusPlan.Digest},
		{ID: "natural_corpus_audit", Digest: audit.Digest},
		{ID: "controlled_corpus_release", Digest: release.Digest},
		{ID: "relation_governance_plan", Digest: plan.Digest},
		{ID: "relation_primary_sample", Digest: primary.Digest},
		{ID: "relation_scarcity_sentinel", Digest: sentinel.Digest},
	}
}

func validateScarcityAvailability(value ScarcityPublicAvailability) error {
	if value.Target < 1 || value.Attempted < 1 || value.Admitted < 1 || value.Rejected < 1 || value.Selected < 1 || value.Shortfall < 1 || !value.Exhaustive ||
		value.Attempted != value.Admitted+value.Rejected || value.Admitted != value.Selected || value.Shortfall != value.Target-value.Selected {
		return errors.New("scarcity public evidence availability funnel is invalid")
	}
	return nil
}

func validateScarcityRoles(value ScarcityPublicStudyRoles, selected int) error {
	if value.Development < 0 || value.Calibration < 0 || value.Test != 0 || value.PrimaryEstimandOverlap != 0 || value.Development+value.Calibration != selected {
		return errors.New("scarcity public evidence study-role boundary is invalid")
	}
	return nil
}

func validateScarcityCore(value ScarcityPublicCore, target int) error {
	if value.Families != 7 || value.CasesPerFamily != target || value.Cases != value.Families*value.CasesPerFamily {
		return errors.New("scarcity public evidence inferential-core boundary is invalid")
	}
	return nil
}

func validateScarcityPrimary(value ScarcityPublicPrimary) error {
	if value.Cases < 1 || value.Cases != value.TaskGroups || value.Cases != value.LineageClusters {
		return errors.New("scarcity public evidence primary-sample boundary is invalid")
	}
	return nil
}

func validateScarcityCoverage(values []ScarcityPublicCoverage, availability ScarcityPublicAvailability) error {
	if len(values) == 0 {
		return errors.New("scarcity public evidence coverage is empty")
	}
	attempted, admitted, rejected, selected, previous := 0, 0, 0, 0, preprocess.SourceFormat("")
	for _, row := range values {
		if strings.TrimSpace(string(row.SourceFormat)) == "" || row.SourceFormat <= previous || row.Attempted < 1 || row.Admitted < 0 || row.Rejected < 0 ||
			row.EligibleTaskGroups < 0 || row.Selected < 0 || row.Attempted != row.Admitted+row.Rejected || row.Selected > row.Admitted {
			return errors.New("scarcity public evidence coverage row is invalid or unsorted")
		}
		attempted, admitted, rejected, selected = attempted+row.Attempted, admitted+row.Admitted, rejected+row.Rejected, selected+row.Selected
		previous = row.SourceFormat
	}
	if attempted != availability.Attempted || admitted != availability.Admitted || rejected != availability.Rejected || selected != availability.Selected {
		return errors.New("scarcity public evidence coverage does not reproduce the aggregate funnel")
	}
	return nil
}

func validateScarcityRejections(values []mutation.CorpusCount, rejected int) error {
	if len(values) == 0 {
		return errors.New("scarcity public evidence rejection reasons are empty")
	}
	total, previous := 0, ""
	for _, item := range values {
		if strings.TrimSpace(item.ID) == "" || item.ID <= previous || item.Count < 1 {
			return errors.New("scarcity public evidence rejection reasons are invalid or unsorted")
		}
		total, previous = total+item.Count, item.ID
	}
	if total != rejected {
		return errors.New("scarcity public evidence rejection reasons do not reproduce the rejected denominator")
	}
	return nil
}

func validateScarcityCases(values []ScarcityPublicCase, roles ScarcityPublicStudyRoles, selected int) error {
	if len(values) != selected {
		return errors.New("scarcity public evidence case commitments are incomplete")
	}
	roleCounts, bindings, firewalls := map[string]int{}, map[string]struct{}{}, map[string]struct{}{}
	for index, item := range values {
		if item.Index != index+1 || strings.TrimSpace(item.DataRole) == "" || item.Unit != UnitTrajectoryPair ||
			!validDigest(item.CaseBindingDigest) || !validDigest(item.ConstructFirewallDigest) {
			return errors.New("scarcity public evidence case commitment is invalid")
		}
		if _, exists := bindings[item.CaseBindingDigest]; exists {
			return errors.New("scarcity public evidence reuses a case binding")
		}
		if _, exists := firewalls[item.ConstructFirewallDigest]; exists {
			return errors.New("scarcity public evidence reuses a construct firewall")
		}
		roleCounts[item.DataRole]++
		bindings[item.CaseBindingDigest], firewalls[item.ConstructFirewallDigest] = struct{}{}, struct{}{}
	}
	if roleCounts["development"] != roles.Development || roleCounts["calibration"] != roles.Calibration || roleCounts["test"] != roles.Test || len(roleCounts) != 2 {
		return errors.New("scarcity public evidence case roles do not reproduce the study-role boundary")
	}
	return nil
}

func validateScarcityParents(values []ScarcityPublicParent) error {
	expectedIDs := []string{
		"corpus_development_plan", "natural_corpus_audit", "controlled_corpus_release",
		"relation_governance_plan", "relation_primary_sample", "relation_scarcity_sentinel",
	}
	if len(values) != len(expectedIDs) {
		return errors.New("scarcity public evidence parent chain is incomplete")
	}
	for index, item := range values {
		if item.ID != expectedIDs[index] || !validDigest(item.Digest) {
			return fmt.Errorf("scarcity public evidence parent %d is invalid", index)
		}
	}
	return nil
}

func expectedScarcityPublicClaims() []ScarcityPublicClaim {
	return []ScarcityPublicClaim{
		{ID: "frozen_corpus_availability", Claim: "Exact availability in the frozen corpus", Status: ScarcityPublicClaimSupported, Evidence: "supported by the reproduced attempt, firewall, release, and sentinel chain"},
		{ID: "held_out_omitted_evidence_validity", Claim: "Held-out omitted-evidence validity", Status: ScarcityPublicClaimUnsupported, Evidence: "unsupported: zero test-role sentinel cases"},
		{ID: "human_construct_agreement", Claim: "Human construct agreement", Status: ScarcityPublicClaimNotRun, Evidence: "not run"},
		{ID: "verifier_robustness", Claim: "Verifier robustness for this construct", Status: ScarcityPublicClaimNotMeasured, Evidence: "not measured"},
		{ID: "provider_behavior", Claim: "Provider behavior", Status: ScarcityPublicClaimNotMeasured, Evidence: "not measured; no provider was invoked"},
		{ID: "population_prevalence", Claim: "Population prevalence or universal scarcity", Status: ScarcityPublicClaimUnsupported, Evidence: "unsupported: the corpus is not a probability sample"},
		{ID: "external_action", Claim: "Reviewer contact, packet sharing, or publication authority", Status: ScarcityPublicClaimNotAuthorized, Evidence: "not authorized"},
	}
}

func (evidence ScarcityPublicEvidence) digest() (string, error) {
	evidence.Digest = ""
	return digestJSON(evidence)
}
