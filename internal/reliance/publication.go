package reliance

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func BuildEvidenceRelianceMap(request EvidenceRelianceMapRequest) (EvidenceRelianceMap, error) {
	parents := request.SelectorParents
	analysis, corpus, err := buildReliancePublicationAnalysis(parents, request.SelectorAudit)
	if err != nil {
		return EvidenceRelianceMap{}, err
	}
	terms, err := buildRelianceMapTerms(parents.Preregistration, analysis, request.SelectorAudit)
	if err != nil {
		return EvidenceRelianceMap{}, err
	}
	comparisons, err := buildPublishedArmComparisons(parents, request.ArmComparisons)
	if err != nil {
		return EvidenceRelianceMap{}, err
	}
	witnesses, err := buildPublishedRelianceWitnesses(analysis.Digest, request.Witnesses)
	if err != nil {
		return EvidenceRelianceMap{}, err
	}
	value := newEvidenceRelianceMap(parents, analysis, corpus, request.SelectorAudit, terms, comparisons, witnesses)
	value.ProfileDimensions = buildRelianceProfileDimensions(value)
	value.PaperRows = buildReliancePaperRows(value)
	digest, err := evidenceRelianceMapDigest(value)
	if err != nil {
		return EvidenceRelianceMap{}, err
	}
	value.Digest = digest
	return value, value.Validate()
}

func buildReliancePublicationAnalysis(
	parents EvidenceSelectorAuditParents,
	audit EvidenceSelectorAudit,
) (EvidenceRelianceAnalysis, RelianceAnalysisCorpus, error) {
	if err := audit.Validate(parents); err != nil {
		return EvidenceRelianceAnalysis{}, RelianceAnalysisCorpus{}, err
	}
	analysis, err := AnalyzeEvidenceReliance(
		parents.Registration, parents.Preregistration, parents.Executions, parents.Failures,
	)
	if err != nil {
		return EvidenceRelianceAnalysis{}, RelianceAnalysisCorpus{}, err
	}
	if audit.AnalysisDigest != analysis.Digest {
		return EvidenceRelianceAnalysis{}, RelianceAnalysisCorpus{}, errors.New("reliance publication selector audit belongs to another analysis")
	}
	corpus, err := BuildRelianceAnalysisCorpus(
		parents.Registration, parents.Preregistration, parents.Executions, parents.Failures,
	)
	return analysis, corpus, err
}

func newEvidenceRelianceMap(
	parents EvidenceSelectorAuditParents,
	analysis EvidenceRelianceAnalysis,
	corpus RelianceAnalysisCorpus,
	audit EvidenceSelectorAudit,
	terms []RelianceMapTerm,
	comparisons []RelianceMapArmComparison,
	witnesses []RelianceMapWitness,
) EvidenceRelianceMap {
	return EvidenceRelianceMap{
		SchemaVersion: EvidenceRelianceMapSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PublicationPolicy: ReliancePublicationPolicyVersion, Scope: reliancePublicationScope(parents.Registration),
		OntologyDigest: parents.Ontology.Digest, EstimandCatalogDigest: parents.Estimands.Digest,
		PreregistrationDigest: parents.Preregistration.Digest, PreflightDigest: parents.Preflight.Digest,
		RegistrationDigest: parents.Registration.Digest, CorpusDigest: corpus.Digest,
		AnalysisDigest: analysis.Digest, SelectorAuditDigest: audit.Digest,
		SourceTasks: corpus.SourceTasks, RegisteredCells: corpus.RegisteredCells,
		OutcomeBearingCells: corpus.OutcomeBearingCells, ExcludedCells: corpus.RegisteredCells - corpus.OutcomeBearingCells,
		CompletedLogicalCalls: corpus.CompletedPanelLogicalCalls + corpus.FailureAttributedLogicalCalls,
		StatusCounts:          cloneRelianceStatusCounts(corpus.StatusCounts), Terms: terms,
		ArmComparisons: comparisons, Witnesses: witnesses,
		PaperLimitations: reliancePaperLimitations(), AllowedClaims: relianceAllowedClaims(),
		ForbiddenClaims: relianceForbiddenClaims(), ProjectionProviderCalls: 0, NetworkRequired: false,
	}
}

func reliancePublicationScope(registration ReliancePanelRegistration) ReliancePublicationScope {
	arm := registration.Arm
	return ReliancePublicationScope{
		EvidenceRole: RelianceLocalMechanismRole, DataRole: RelianceLocalMechanismDataRole,
		EvidenceLevel: RelianceLocalEvidenceLevel, Domain: "coding_agent_trajectory",
		StudyManifestDigest: registration.StudyManifestDigest, Entrypoint: arm.Entrypoint,
		CriterionID: arm.CriterionID, ScoreTag: arm.ScoreTag, EvidencePolicy: string(arm.EvidencePolicy),
		ProviderID: arm.ProviderID, RouteID: arm.RouteID, RequestedModel: arm.RequestedModel, Empirical: false,
	}
}

func buildRelianceMapTerms(
	preregistration Preregistration,
	analysis EvidenceRelianceAnalysis,
	audit EvidenceSelectorAudit,
) ([]RelianceMapTerm, error) {
	terms := referenceFactorialTerms()
	result := make([]RelianceMapTerm, len(terms))
	for index, term := range terms {
		outcomes, err := buildRelianceMapTermOutcomes(term, preregistration, analysis)
		if err != nil {
			return nil, err
		}
		kind := RelianceMapMainEffect
		if len(term.Factors) > 1 {
			kind = RelianceMapInteraction
		}
		result[index] = RelianceMapTerm{
			TermID: term.ID, Kind: kind, Factors: slices.Clone(term.Factors), Outcomes: outcomes,
			Selector: publishedSelectorForTerm(term, audit),
		}
	}
	slices.SortFunc(result, func(left, right RelianceMapTerm) int { return strings.Compare(left.TermID, right.TermID) })
	return result, nil
}

func buildRelianceMapTermOutcomes(
	term stats.FactorialTerm,
	preregistration Preregistration,
	analysis EvidenceRelianceAnalysis,
) ([]RelianceMapTermOutcome, error) {
	result := make([]RelianceMapTermOutcome, len(preregistration.PrimaryOutcomes))
	for index, declared := range preregistration.PrimaryOutcomes {
		outcome, found := relianceAnalysisOutcome(analysis, declared.OutcomeID)
		if !found {
			return nil, fmt.Errorf("reliance publication lacks outcome %q", declared.OutcomeID)
		}
		mapped, err := relianceMapTermOutcome(term.ID, outcome)
		if err != nil {
			return nil, err
		}
		result[index] = mapped
	}
	slices.SortFunc(result, func(left, right RelianceMapTermOutcome) int {
		return strings.Compare(string(left.OutcomeID), string(right.OutcomeID))
	})
	return result, nil
}

func relianceMapTermOutcome(termID string, outcome EvidenceRelianceOutcomeFit) (RelianceMapTermOutcome, error) {
	value := RelianceMapTermOutcome{
		OutcomeID: outcome.OutcomeID, RegisteredCells: outcome.RegisteredCells,
		EligibleObservations: outcome.EligibleObservations, ExcludedFromFit: outcome.ExcludedFromFit,
	}
	if outcome.Status == RelianceFitInconclusive {
		value.Status, value.Reason = RelianceMapInconclusive, outcome.Reason
		return value, nil
	}
	estimate, found := factorialEstimate(outcome.Fit, termID)
	if outcome.Status != RelianceFitMeasured || outcome.Fit == nil || !found {
		return RelianceMapTermOutcome{}, fmt.Errorf("reliance publication lacks measured estimate %q/%q", termID, outcome.OutcomeID)
	}
	value.Status = RelianceMapMeasured
	value.Estimate = publishedRelianceEstimate(estimate)
	return value, nil
}

func publishedSelectorForTerm(term stats.FactorialTerm, audit EvidenceSelectorAudit) *RelianceMapSelector {
	if len(term.Factors) != 1 || term.Factors[0] == PresentationOrderTerm {
		return nil
	}
	for _, factor := range audit.Factors {
		if string(factor.FactorID) == term.Factors[0] {
			value := RelianceMapSelector{
				EffectStatus: factor.EffectStatus, AssignmentTargets: factor.AssignmentTargets,
				MinimumEventScore: factor.MinimumEventScore, MaximumEventScore: factor.MaximumEventScore,
				Budgets: publishedSelectorBudgets(factor.Budgets),
			}
			return &value
		}
	}
	return nil
}

func publishedSelectorBudgets(values []SelectorFactorBudgetAudit) []RelianceMapSelectorBudget {
	result := make([]RelianceMapSelectorBudget, len(values))
	for index, value := range values {
		result[index] = RelianceMapSelectorBudget{
			BudgetTokens: value.BudgetTokens, AssignmentTargets: value.AssignmentTargets,
			ExactTargets: value.ExactTargets, ChangedTargets: value.ChangedTargets,
			UnrenderedTargets: value.UnrenderedTargets, DroppedTargets: value.DroppedTargets,
			AssignedEventBytes: value.AssignedEventBytes, RetainedAssignedEventBytes: value.RetainedAssignedEventBytes,
			AssignedRenderedBytes:         value.AssignedRenderedBytes,
			RetainedAssignedRenderedBytes: value.RetainedAssignedRenderedBytes,
			RiskFlags:                     slices.Clone(value.RiskFlags),
		}
	}
	return result
}

func buildPublishedArmComparisons(
	parents EvidenceSelectorAuditParents,
	values []RelianceArmComparisonEvidence,
) ([]RelianceMapArmComparison, error) {
	if len(values) == 0 {
		return nil, errors.New("reliance publication requires at least one arm comparison")
	}
	result := make([]RelianceMapArmComparison, len(values))
	for index, evidence := range values {
		if err := evidence.Comparison.Validate(parents.Preregistration, parents.Preflight, evidence.Arms, evidence.Specs); err != nil {
			return nil, err
		}
		result[index] = RelianceMapArmComparison{Comparison: cloneRelianceArmComparison(evidence.Comparison)}
	}
	slices.SortFunc(result, func(left, right RelianceMapArmComparison) int {
		return strings.Compare(left.Comparison.Digest, right.Comparison.Digest)
	})
	return result, nil
}

func buildPublishedRelianceWitnesses(
	analysisDigest string,
	values []RelianceWitnessPublicationEvidence,
) ([]RelianceMapWitness, error) {
	if len(values) == 0 {
		return nil, errors.New("reliance publication requires at least one reliance witness")
	}
	result := make([]RelianceMapWitness, len(values))
	for index, evidence := range values {
		if err := evidence.Witness.Validate(evidence.Request, evidence.Execution); err != nil {
			return nil, err
		}
		if !evidence.Witness.PublicReleaseAllowed {
			return nil, errors.New("reliance publication cannot expose a witness without public-release permission")
		}
		result[index] = RelianceMapWitness{
			SourceAnalysisDigest: analysisDigest,
			BindingStatus:        RelianceWitnessRelationBinding,
			Witness:              cloneRelianceWitness(evidence.Witness),
		}
	}
	slices.SortFunc(result, func(left, right RelianceMapWitness) int {
		return strings.Compare(left.Witness.Digest, right.Witness.Digest)
	})
	return result, nil
}

func relianceAnalysisOutcome(value EvidenceRelianceAnalysis, outcomeID OutcomeID) (EvidenceRelianceOutcomeFit, bool) {
	for _, outcome := range value.OutcomeFits {
		if outcome.OutcomeID == outcomeID {
			return outcome, true
		}
	}
	return EvidenceRelianceOutcomeFit{}, false
}

func factorialEstimate(value *stats.ClusteredFactorialFit, termID string) (stats.FactorialEstimate, bool) {
	if value == nil {
		return stats.FactorialEstimate{}, false
	}
	for _, estimate := range value.Estimates {
		if estimate.TermID == termID {
			return estimate, true
		}
	}
	return stats.FactorialEstimate{}, false
}

func publishedRelianceEstimate(value stats.FactorialEstimate) *RelianceMapEstimate {
	return &RelianceMapEstimate{
		Estimate: value.Estimate, StandardError: value.StandardError, Lower: value.Lower,
		Upper: value.Upper, AdjustedPValue: value.AdjustedPValue,
	}
}

func cloneRelianceStatusCounts(values []RelianceCellStatusCount) []RelianceCellStatusCount {
	return slices.Clone(values)
}

func relianceAllowedClaims() []string {
	return []string{
		"bounded verifier-output reliance under the exact local mechanism fixture",
		"complete registered-cell and missingness accounting under the frozen estimator",
		"deterministic selector-retention comparison against the frozen local analysis",
		"one-minimal witness over declared reduction units with preserved result identity",
	}
}

func relianceForbiddenClaims() []string {
	return []string{
		"agent-step or environment causality",
		"global-minimum witness",
		"human-interpretable or model-internal reasoning attribution",
		"provider, model-family, route, entrypoint, or population transfer without separately admitted empirical evidence",
		"selector nondetection as zero effect or equivalence",
		"universal verifier faithfulness or trust",
	}
}

func reliancePaperLimitations() []string {
	result := []string{
		"The current map is E1 local mechanism evidence and contains no admitted external-model measurement.",
		"The registration freeze stage is declared but has no external timestamp proof.",
		"Selector nondetection is not evidence of zero effect or equivalence.",
		"Witnesses are one-minimal over declared units and do not establish a global minimum.",
		"Witness proof digests bind oracle evidence but do not recreate withheld private proof content.",
		"No agent-step causality, model-internal explanation, universal faithfulness, or transfer claim is supported.",
	}
	sort.Strings(result)
	return result
}
