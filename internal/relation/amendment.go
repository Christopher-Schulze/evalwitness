package relation

import (
	"errors"
	"math"
	"slices"
	"time"
)

const (
	primaryAggregationRule    = "within each source task group, any contradiction yields contradicted; otherwise any unresolved yields unresolved; only complete support yields supported"
	primaryReplacementRule    = "none; the exact 31-case commitment is immutable and every unavailable case remains visible"
	primaryReplacementRuleV2  = "none; the exact balanced 32-case commitment is immutable and every unavailable case remains visible"
	primaryMissingnessRule    = "missing, late, partial, conflicted, or insufficient judgments resolve unresolved and remain in every case and task-group denominator"
	primaryStoppingRule       = "fixed sample; stop only after all 31 cases reach committed dual review plus required tie-break or explicit unresolved state; no sequential efficacy stopping"
	primaryStoppingRuleV2     = "fixed sample; stop only after all 32 cases reach committed dual review plus required tie-break or explicit unresolved state; no sequential efficacy stopping"
	primaryEstimand           = "source-task-group prevalence of formal-human contradiction with unresolved groups retained separately"
	intervalMethod            = "one-sided exact binomial upper confidence bound at the source-task-group level"
	multiplicityMethod        = "one prespecified global primary endpoint; family, class, split, and control rows are descriptive and cannot replace it"
	familyAnalysisRole        = "descriptive bounded construct evidence only; no per-family validation or universal error-rate claim"
	unresolvedDenominatorRule = "unresolved is never pooled with support, never dropped, and never converted by majority vote"
	independenceLimitation    = "exact binomial resolution assumes independent sampled task groups; the governed corpus is not a probability sample, so the bound is a design diagnostic rather than population generalization"
	independenceLimitationV2  = "exact binomial resolution assumes independent sampled task groups; the governed corpus is not a probability sample and its 32 task groups occupy 24 reported lineage clusters, so the bound is a task-group design diagnostic rather than an effective-lineage or population guarantee"
	claimBoundary             = "report blinded reviewers' support, contradiction, and unresolved states only for the named sampled tasks under the frozen material and rubric; do not claim universal construct validity, verifier robustness, or human ground truth"
)

func BuildStudyAmendment(plan Plan, pilot PilotSample, sample PrimarySample, issuedAt string) (StudyAmendment, error) {
	if err := plan.Validate(); err != nil {
		return StudyAmendment{}, err
	}
	if err := sample.Validate(); err != nil {
		return StudyAmendment{}, err
	}
	if err := pilot.Validate(); err != nil {
		return StudyAmendment{}, err
	}
	if sample.PlanDigest != plan.Digest || sample.SourceCorpusDigest != plan.SourceCorpusDigest || pilot.PlanDigest != plan.Digest ||
		pilot.PrimarySampleDigest != sample.Digest || pilot.SourceCorpusDigest != sample.SourceCorpusDigest {
		return StudyAmendment{}, errors.New("relation amendment plan, pilot, and primary sample bindings differ")
	}
	if sample.ProtocolVersion != plan.ProtocolVersion || pilot.ProtocolVersion != plan.ProtocolVersion {
		return StudyAmendment{}, errors.New("relation amendment plan and sample protocol versions differ")
	}
	if plan.SchemaVersion == PlanSchemaVersionV2 && (sample.SourceCorpusSpecDigest != plan.SourceCorpusSpecDigest ||
		pilot.SourceCorpusSpecDigest != plan.SourceCorpusSpecDigest || sample.SourceMutationProgramDigest != plan.SourceMutationProgramDigest ||
		pilot.SourceMutationProgramDigest != plan.SourceMutationProgramDigest || sample.SourceConstructAuditDigest != plan.SourceConstructAuditDigest ||
		pilot.SourceConstructAuditDigest != plan.SourceConstructAuditDigest) {
		return StudyAmendment{}, errors.New("v2 relation amendment corpus spec, mutation program, or construct audit bindings differ")
	}
	if _, err := time.Parse(time.RFC3339, issuedAt); err != nil {
		return StudyAmendment{}, errors.New("relation amendment issued_at must be RFC3339")
	}
	alpha := 0.05
	replacementRule, stoppingRule := primaryReplacementRule, primaryStoppingRule
	independenceBoundary := independenceLimitation
	if plan.SchemaVersion == PlanSchemaVersionV2 {
		replacementRule, stoppingRule = primaryReplacementRuleV2, primaryStoppingRuleV2
		independenceBoundary = independenceLimitationV2
	}
	amendment := StudyAmendment{
		ProtocolVersion: plan.ProtocolVersion, Objective: ReviewObjectiveControlledRelation, IssuedAt: issuedAt,
		PlanDigest: plan.Digest, PilotSampleDigest: pilot.Digest, PrimarySampleDigest: sample.Digest, SourceCorpusDigest: sample.SourceCorpusDigest,
		Pilot: PilotDesign{
			Cases: plan.PilotSampleSize, PrimaryLabels: plan.PilotSampleSize * plan.PrimaryReviewers,
			MaximumTieBreakLabels: plan.PilotSampleSize * plan.TieBreakReviewers, PostLabelProbes: plan.PilotSampleSize * plan.PrimaryReviewers,
			DataRole: "development", PrimaryOverlap: 0,
			Purpose:            "materialization, comprehension, timing, qualification, ambiguity, and leakage only",
			PrimaryAnalysisUse: "forbidden",
		},
		Primary: PrimaryDesign{
			Cases: sample.SelectedCases, EffectiveTaskGroups: sample.UniqueTaskGroups, PrimaryLabels: sample.SelectedCases * plan.PrimaryReviewers,
			MaximumTieBreakLabels: sample.SelectedCases * plan.TieBreakReviewers, PostLabelProbes: sample.SelectedCases * plan.PrimaryReviewers,
			ClusterUnit: "source_task_group", AggregationRule: primaryAggregationRule, ReplacementRule: replacementRule,
			MissingnessRule: primaryMissingnessRule, StoppingRule: stoppingRule,
		},
		Inference: RelationInference{
			PrimaryEstimand: primaryEstimand, NominalAlpha: alpha, IntervalMethod: intervalMethod,
			MultiplicityMethod: multiplicityMethod, PrimaryMultiplicityFamily: []string{"cluster_contradiction_prevalence"},
			FamilyAnalysisRole: familyAnalysisRole, ZeroContradictionUpperBound: zeroContradictionUpperBound(sample.UniqueTaskGroups, alpha),
			DetectionScenarios: []DetectionScenario{
				{ContradictionRate: 0.05, DetectionProbability: detectionProbability(sample.UniqueTaskGroups, 0.05)},
				{ContradictionRate: 0.10, DetectionProbability: detectionProbability(sample.UniqueTaskGroups, 0.10)},
				{ContradictionRate: 0.20, DetectionProbability: detectionProbability(sample.UniqueTaskGroups, 0.20)},
			},
			UnresolvedDenominatorRule: unresolvedDenominatorRule, IndependenceLimitation: independenceBoundary,
		},
		ClaimBoundary: claimBoundary, EmpiricalStatus: "not_run", ExternalActionStatus: ExternalActionNotAuthorized,
	}
	if plan.SchemaVersion == PlanSchemaVersionV2 {
		amendment.SourceCorpusSpecDigest = plan.SourceCorpusSpecDigest
		amendment.SourceMutationProgramDigest = plan.SourceMutationProgramDigest
		amendment.SourceConstructAuditDigest = plan.SourceConstructAuditDigest
		return SealStudyAmendmentV2(amendment)
	}
	return SealStudyAmendment(amendment)
}

func SealStudyAmendment(amendment StudyAmendment) (StudyAmendment, error) {
	amendment.SchemaVersion, amendment.CanonicalPolicy, amendment.Digest = StudyAmendmentSchemaVersionV1, CanonicalPolicy, ""
	digest, err := studyAmendmentDigest(amendment)
	if err != nil {
		return StudyAmendment{}, err
	}
	amendment.Digest = digest
	return amendment, amendment.Validate()
}

func SealStudyAmendmentV2(amendment StudyAmendment) (StudyAmendment, error) {
	amendment.SchemaVersion, amendment.CanonicalPolicy, amendment.Digest = StudyAmendmentSchemaVersionV2, CanonicalPolicy, ""
	digest, err := studyAmendmentDigest(amendment)
	if err != nil {
		return StudyAmendment{}, err
	}
	amendment.Digest = digest
	return amendment, amendment.Validate()
}

func (amendment StudyAmendment) Validate() error {
	issuedAt, issuedErr := time.Parse(time.RFC3339, amendment.IssuedAt)
	if amendment.CanonicalPolicy != CanonicalPolicy || amendment.Objective != ReviewObjectiveControlledRelation || issuedErr != nil || issuedAt.Location() != time.UTC || !validDigest(amendment.PlanDigest) ||
		!validDigest(amendment.PilotSampleDigest) || !validDigest(amendment.PrimarySampleDigest) || !validDigest(amendment.SourceCorpusDigest) || amendment.EmpiricalStatus != "not_run" ||
		amendment.ExternalActionStatus != ExternalActionNotAuthorized || amendment.ClaimBoundary != claimBoundary {
		return errors.New("relation study amendment identity, timing, claim, empirical, or authorization boundary is invalid")
	}
	primaryCases, primaryGroups, primaryLabels := 31, 28, 62
	replacementRule, stoppingRule := primaryReplacementRule, primaryStoppingRule
	independenceBoundary := independenceLimitation
	switch amendment.SchemaVersion {
	case StudyAmendmentSchemaVersionV1:
		if amendment.ProtocolVersion != ProtocolVersionV1 || amendment.SourceCorpusSpecDigest != "" || amendment.SourceMutationProgramDigest != "" || amendment.SourceConstructAuditDigest != "" {
			return errors.New("v1 relation study amendment identity or historical binding is invalid")
		}
	case StudyAmendmentSchemaVersionV2:
		if amendment.ProtocolVersion != ProtocolVersionV2 || !validDigest(amendment.SourceCorpusSpecDigest) || !validDigest(amendment.SourceMutationProgramDigest) ||
			!validDigest(amendment.SourceConstructAuditDigest) {
			return errors.New("v2 relation study amendment identity or corpus audit binding is invalid")
		}
		primaryCases, primaryGroups, primaryLabels = 32, 32, 64
		replacementRule, stoppingRule = primaryReplacementRuleV2, primaryStoppingRuleV2
		independenceBoundary = independenceLimitationV2
	default:
		return errors.New("unknown relation study amendment schema version")
	}
	if amendment.Pilot.Cases != 8 || amendment.Pilot.PrimaryLabels != 16 || amendment.Pilot.MaximumTieBreakLabels != 8 || amendment.Pilot.PostLabelProbes != 16 ||
		amendment.Pilot.DataRole != "development" || amendment.Pilot.PrimaryOverlap != 0 || amendment.Pilot.PrimaryAnalysisUse != "forbidden" {
		return errors.New("relation study amendment pilot design is invalid")
	}
	if amendment.Primary.Cases != primaryCases || amendment.Primary.EffectiveTaskGroups != primaryGroups || amendment.Primary.PrimaryLabels != primaryLabels ||
		amendment.Primary.MaximumTieBreakLabels != primaryCases || amendment.Primary.PostLabelProbes != primaryLabels || amendment.Primary.ClusterUnit != "source_task_group" ||
		amendment.Primary.AggregationRule != primaryAggregationRule || amendment.Primary.ReplacementRule != replacementRule ||
		amendment.Primary.MissingnessRule != primaryMissingnessRule || amendment.Primary.StoppingRule != stoppingRule {
		return errors.New("relation study amendment primary sample, cluster, missingness, replacement, or stopping design is invalid")
	}
	inference := amendment.Inference
	if inference.PrimaryEstimand != primaryEstimand || inference.NominalAlpha != 0.05 || inference.IntervalMethod != intervalMethod ||
		inference.MultiplicityMethod != multiplicityMethod || !slices.Equal(inference.PrimaryMultiplicityFamily, []string{"cluster_contradiction_prevalence"}) ||
		inference.FamilyAnalysisRole != familyAnalysisRole || inference.UnresolvedDenominatorRule != unresolvedDenominatorRule || inference.IndependenceLimitation != independenceBoundary ||
		inference.ZeroContradictionUpperBound != zeroContradictionUpperBound(amendment.Primary.EffectiveTaskGroups, inference.NominalAlpha) || len(inference.DetectionScenarios) != 3 {
		return errors.New("relation study amendment inference, multiplicity, denominator, or resolution design is invalid")
	}
	expectedRates := []float64{0.05, 0.10, 0.20}
	for index, scenario := range inference.DetectionScenarios {
		if scenario.ContradictionRate != expectedRates[index] || scenario.DetectionProbability != detectionProbability(amendment.Primary.EffectiveTaskGroups, scenario.ContradictionRate) {
			return errors.New("relation study amendment detection scenario does not reproduce")
		}
	}
	expected, err := studyAmendmentDigest(amendment)
	if err != nil || amendment.Digest != expected {
		return errors.New("relation study amendment digest is invalid")
	}
	return nil
}

func zeroContradictionUpperBound(taskGroups int, alpha float64) float64 {
	return 1 - math.Pow(alpha, 1/float64(taskGroups))
}

func detectionProbability(taskGroups int, contradictionRate float64) float64 {
	return 1 - math.Pow(1-contradictionRate, float64(taskGroups))
}

func studyAmendmentDigest(amendment StudyAmendment) (string, error) {
	amendment.Digest = ""
	return digestJSON(amendment)
}
