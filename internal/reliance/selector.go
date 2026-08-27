package reliance

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func BuildEvidenceSelectorAudit(parents EvidenceSelectorAuditParents) (EvidenceSelectorAudit, error) {
	analysis, sources, err := validateEvidenceSelectorAuditParents(parents)
	if err != nil {
		return EvidenceSelectorAudit{}, err
	}
	return constructEvidenceSelectorAudit(parents, analysis, sources)
}

func (value EvidenceSelectorAudit) Validate(parents EvidenceSelectorAuditParents) error {
	expected, err := BuildEvidenceSelectorAudit(parents)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("evidence selector audit differs from its frozen analysis and source artifacts")
	}
	return nil
}

func validateEvidenceSelectorAuditParents(
	parents EvidenceSelectorAuditParents,
) (EvidenceRelianceAnalysis, []EvidenceSelectorAuditSource, error) {
	if err := parents.Preregistration.Validate(parents.Ontology, parents.Estimands); err != nil {
		return EvidenceRelianceAnalysis{}, nil, err
	}
	if err := parents.Preflight.Validate(parents.Preregistration); err != nil {
		return EvidenceRelianceAnalysis{}, nil, err
	}
	if parents.Registration.PreflightDigest != parents.Preflight.Digest {
		return EvidenceRelianceAnalysis{}, nil, errors.New("selector audit registration does not bind the validated preflight")
	}
	analysis, err := AnalyzeEvidenceReliance(
		parents.Registration, parents.Preregistration, parents.Executions, parents.Failures,
	)
	if err != nil {
		return EvidenceRelianceAnalysis{}, nil, err
	}
	sources, err := canonicalSelectorAuditSources(parents)
	return analysis, sources, err
}

func canonicalSelectorAuditSources(parents EvidenceSelectorAuditParents) ([]EvidenceSelectorAuditSource, error) {
	sources, err := cloneSelectorAuditSources(parents.Sources)
	if err != nil {
		return nil, err
	}
	if len(sources) != parents.Registration.SourceTaskCount {
		return nil, fmt.Errorf("selector audit has %d of %d registered source tasks", len(sources), parents.Registration.SourceTaskCount)
	}
	slices.SortFunc(sources, func(left, right EvidenceSelectorAuditSource) int {
		return strings.Compare(left.SourceTaskID, right.SourceTaskID)
	})
	for index, source := range sources {
		if source.SourceTaskID != parents.Registration.SourceTasks[index].SourceTaskID {
			return nil, errors.New("selector audit source-task order or coverage differs from registration")
		}
		if err := validateSelectorAuditSource(parents, source); err != nil {
			return nil, err
		}
	}
	return sources, nil
}

func cloneSelectorAuditSources(values []EvidenceSelectorAuditSource) ([]EvidenceSelectorAuditSource, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("clone selector audit sources: %w", err)
	}
	var result []EvidenceSelectorAuditSource
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, fmt.Errorf("clone selector audit sources: %w", err)
	}
	return result, nil
}

func validateSelectorAuditSource(parents EvidenceSelectorAuditParents, source EvidenceSelectorAuditSource) error {
	if err := source.Assignments.Validate(parents.Ontology, source.Trajectory); err != nil {
		return err
	}
	if err := source.TreatmentPlan.Validate(
		parents.Ontology, parents.Estimands, source.Assignments, source.Trajectory,
	); err != nil {
		return err
	}
	registered, found := parents.Registration.sourceTask(source.SourceTaskID)
	if !found || registered.SourceTrajectoryDigest != source.Trajectory.Digest ||
		registered.AssignmentSetDigest != source.Assignments.Digest || registered.TreatmentPlanDigest != source.TreatmentPlan.Digest {
		return fmt.Errorf("selector audit source task %q differs from its registered artifacts", source.SourceTaskID)
	}
	return nil
}

func constructEvidenceSelectorAudit(
	parents EvidenceSelectorAuditParents,
	analysis EvidenceRelianceAnalysis,
	sources []EvidenceSelectorAuditSource,
) (EvidenceSelectorAudit, error) {
	retention, err := collectSelectorRetention(sources, selectorAuditBudgets())
	if err != nil {
		return EvidenceSelectorAudit{}, err
	}
	factors, assignments, err := buildSelectorFactorAudits(parents.Ontology, analysis, retention)
	if err != nil {
		return EvidenceSelectorAudit{}, err
	}
	sourceDigest, err := selectorSourceArtifactSetDigest(sources)
	if err != nil {
		return EvidenceSelectorAudit{}, err
	}
	value := EvidenceSelectorAudit{
		SchemaVersion: EvidenceSelectorAuditSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PolicyVersion: EvidenceSelectorAuditPolicyVersion, RegistrationDigest: parents.Registration.Digest,
		AnalysisDigest: analysis.Digest, PreregistrationDigest: parents.Preregistration.Digest,
		PreflightDigest: parents.Preflight.Digest, SourceArtifactSetDigest: sourceDigest,
		ProductionPolicy: preprocess.InspectEvidenceSelectionPolicies(), LegacyLinePolicy: legacyLineSelectorAudit(),
		EffectDetectionRule: SelectorEffectDetectionRule, AdjustedEffectAlpha: referenceNominalAlpha,
		Budgets: selectorAuditBudgets(), SourceTasks: len(sources), AssignmentTargets: assignments,
		EventBytesAreNonAdditive: true, Factors: factors, Categories: retention.categories,
		ProviderCalls: 0, NetworkRequired: false,
	}
	value.Digest, err = evidenceSelectorAuditDigest(value)
	return value, err
}

func buildSelectorFactorAudits(
	ontology FactorOntology,
	analysis EvidenceRelianceAnalysis,
	retention selectorRetention,
) ([]SelectorFactorAudit, int, error) {
	result := make([]SelectorFactorAudit, len(ontology.Factors))
	totalAssignments := 0
	for index, factor := range ontology.Factors {
		termOutcomes, effectStatus, err := selectorFactorTermOutcomes(factor.FactorID, analysis)
		if err != nil {
			return nil, 0, err
		}
		observed := retention.factors[factor.FactorID]
		budgets := selectorFactorBudgets(observed, effectStatus)
		result[index] = SelectorFactorAudit{
			FactorID: factor.FactorID, AssignmentTargets: observed.assignments,
			MinimumEventScore: observed.minimumScore, MaximumEventScore: observed.maximumScore,
			EffectStatus: effectStatus, TermOutcomes: termOutcomes, Budgets: budgets,
		}
		totalAssignments += observed.assignments
	}
	return result, totalAssignments, nil
}

func selectorFactorTermOutcomes(
	factorID FactorID,
	analysis EvidenceRelianceAnalysis,
) ([]SelectorTermOutcomeAudit, SelectorEffectStatus, error) {
	terms := selectorTermsForFactor(factorID)
	result := make([]SelectorTermOutcomeAudit, 0, len(terms)*len(analysis.OutcomeFits))
	status := SelectorEffectNotDetected
	for _, term := range terms {
		for _, outcome := range analysis.OutcomeFits {
			entry, err := selectorTermOutcome(term, outcome)
			if err != nil {
				return nil, "", err
			}
			result = append(result, entry)
			if outcome.Status == RelianceFitInconclusive {
				status = SelectorEffectInconclusive
			} else if status != SelectorEffectInconclusive && entry.AdjustedPValue <= referenceNominalAlpha {
				status = SelectorEffectDetected
			}
		}
	}
	return result, status, nil
}

func selectorTermsForFactor(factorID FactorID) []stats.FactorialTerm {
	result := make([]stats.FactorialTerm, 0)
	for _, term := range referenceFactorialTerms() {
		if slices.Contains(term.Factors, string(factorID)) {
			result = append(result, term)
		}
	}
	return result
}

func selectorTermOutcome(
	term stats.FactorialTerm,
	outcome EvidenceRelianceOutcomeFit,
) (SelectorTermOutcomeAudit, error) {
	entry := SelectorTermOutcomeAudit{
		TermID: term.ID, Factors: slices.Clone(term.Factors), OutcomeID: outcome.OutcomeID,
		FitStatus: outcome.Status,
	}
	if outcome.Status == RelianceFitInconclusive {
		return entry, nil
	}
	for _, estimate := range outcome.Fit.Estimates {
		if estimate.TermID == term.ID {
			entry.EstimateAvailable = true
			entry.Estimate, entry.Lower, entry.Upper = estimate.Estimate, estimate.Lower, estimate.Upper
			entry.AdjustedPValue = estimate.AdjustedPValue
			return entry, nil
		}
	}
	return SelectorTermOutcomeAudit{}, fmt.Errorf("selector audit outcome %q lacks term %q", outcome.OutcomeID, term.ID)
}

func legacyLineSelectorAudit() LegacyLineSelectorAudit {
	lines := []string{
		"ERROR: failed with exit code 1", "go test ./... passed", "diff --git a/a.go b/a.go",
		"+created: output", "FINAL summary", "neutral terminal noise",
	}
	policy := preprocess.InspectEvidenceSelectionPolicies()
	return LegacyLineSelectorAudit{
		Status: LegacyLineSelectorStatus, Selector: policy.LegacySelector,
		ScorePolicy: policy.LegacyScorePolicy, Probes: preprocess.InspectLegacyEvidenceLineScores(lines),
	}
}

func selectorAuditBudgets() []int { return []int{16_384, 32_768, 65_536} }

func selectorSourceArtifactSetDigest(sources []EvidenceSelectorAuditSource) (string, error) {
	type sourceIdentity struct {
		SourceTaskID        string `json:"source_task_id"`
		TrajectoryDigest    string `json:"trajectory_digest"`
		AssignmentSetDigest string `json:"assignment_set_digest"`
		TreatmentPlanDigest string `json:"treatment_plan_digest"`
	}
	values := make([]sourceIdentity, len(sources))
	for index, source := range sources {
		values[index] = sourceIdentity{source.SourceTaskID, source.Trajectory.Digest, source.Assignments.Digest, source.TreatmentPlan.Digest}
	}
	return referenceJSONDigest(values)
}

func evidenceSelectorAuditDigest(value EvidenceSelectorAudit) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
