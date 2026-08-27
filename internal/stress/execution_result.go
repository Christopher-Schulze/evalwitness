package stress

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verification"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func (value ReplayExecution) ValidateAgainst(spec Relation) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if value.SchemaVersion != ReplayExecutionSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.RelationDigest != spec.Digest || !identifierPattern.MatchString(value.CaseID) ||
		strings.TrimSpace(value.Entrypoint) == "" || !validDigest(value.BatchRunFingerprint) ||
		!slices.Contains([]verification.EvidencePolicy{verification.EvidenceStrictVerifier, verification.EvidenceExplicitJudge}, value.EvidencePolicy) {
		return errors.New("stress replay execution identity or evidence policy is invalid")
	}
	if err := value.ReplaySource.Validate(); err != nil {
		return err
	}
	if value.Original.PlanFingerprint == value.Transformed.PlanFingerprint {
		return errors.New("stress replay execution sides share one plan fingerprint")
	}
	if err := validateReplayExecutionSide(value.Original, "original", value.Entrypoint, value.EvidencePolicy); err != nil {
		return err
	}
	if err := validateReplayExecutionSide(value.Transformed, "transformed", value.Entrypoint, value.EvidencePolicy); err != nil {
		return err
	}
	if !replaySidesBindSource(value.Original, value.Transformed, value.ReplaySource) {
		return errors.New("stress replay execution does not bind one exact capture source")
	}
	if err := value.StageComparison.Validate(); err != nil {
		return err
	}
	if value.StageComparison.RelationDigest != spec.Digest ||
		value.StageComparison.LeftTraceDigest != value.Original.StageTrace.Digest ||
		value.StageComparison.RightTraceDigest != value.Transformed.StageTrace.Digest {
		return errors.New("stress replay execution stage comparison does not bind its relation and sides")
	}
	expectedDigest, err := replayExecutionDigest(value)
	if err != nil || value.Digest != expectedDigest {
		return errors.New("stress replay execution digest is invalid")
	}
	return nil
}

func replaySidesBindSource(original, transformed ReplaySide, source provider.ExactReplaySource) bool {
	for _, side := range []ReplaySide{original, transformed} {
		for _, observation := range side.Observations {
			if observation.ReplaySource == nil || *observation.ReplaySource != source {
				return false
			}
		}
	}
	return true
}

func replayExecutionDigest(value ReplayExecution) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}

func validateReplayExecutionSide(side ReplaySide, name, entrypoint string, policy verification.EvidencePolicy) error {
	if !validDigest(side.PlanFingerprint) || side.Result.SchemaVersion != verification.RunSchemaVersion ||
		side.Result.RunFingerprint != side.PlanFingerprint || side.Result.Mode == "" || !completeLifecycle(side.Result.Lifecycle) ||
		len(side.Observations) == 0 {
		return fmt.Errorf("stress replay %s execution evidence is incomplete", name)
	}
	if err := side.StageTrace.Validate(); err != nil {
		return err
	}
	if side.StageTrace.Side != name {
		return fmt.Errorf("stress replay %s stage trace has the wrong side identity", name)
	}
	for _, observation := range side.Observations {
		if err := observation.Validate(); err != nil {
			return err
		}
		if observation.Scope != side.PlanFingerprint || observation.Entrypoint != entrypoint ||
			observation.ReplayStatus != provider.ReplayStatusExact || observation.ReplaySource == nil ||
			observation.ExtractionStatus != mode.ExtractionObservationComplete {
			return fmt.Errorf("stress replay %s observation differs from its exact execution boundary", name)
		}
	}
	strength, err := resultEvidenceStrength(side.Result)
	if err != nil {
		return err
	}
	wantMode := verifier.ExtractionModeVerifier
	if policy == verification.EvidenceExplicitJudge {
		wantMode = verifier.ExtractionModeJudge
	}
	if strength.ExtractionMode != wantMode {
		return fmt.Errorf("stress replay %s evidence mode %q differs from arm policy %q", name, strength.ExtractionMode, policy)
	}
	return nil
}

func SealReplayResult(spec Relation, admission ConstructAdmission, taskGroupID string, execution ReplayExecution) (Result, error) {
	if err := execution.ValidateAgainst(spec); err != nil {
		return Result{}, err
	}
	if err := admission.Validate(); err != nil {
		return Result{}, err
	}
	if execution.CaseID != admission.CaseID || !identifierPattern.MatchString(taskGroupID) {
		return Result{}, errors.New("stress replay result case or task-group identity is invalid")
	}
	completed, err := completedReplayRepetitions(execution)
	if err != nil {
		return Result{}, err
	}
	constraints, outcome, err := replayConstraintResults(spec, execution)
	if err != nil {
		return Result{}, err
	}
	comparison := execution.StageComparison
	comparison.Differences = append([]StageDifference(nil), execution.StageComparison.Differences...)
	originalUsage, err := verificationResultUsage(execution.Original.Result)
	if err != nil {
		return Result{}, err
	}
	transformedUsage, err := verificationResultUsage(execution.Transformed.Result)
	if err != nil {
		return Result{}, err
	}
	return SealResult(spec, Result{
		CaseID: execution.CaseID, TaskGroupID: taskGroupID, Admission: &admission, Outcome: outcome,
		ConstraintResults: constraints, DistributionComparisons: []TaggedDistributionComparison{}, StageComparison: &comparison,
		PlannedRepetitions: spec.Repeat.MaximumRepetitions, CompletedRepetitions: completed,
		ProviderCalls: originalUsage.Calls + transformedUsage.Calls,
	})
}

func replayConstraintResults(spec Relation, execution ReplayExecution) ([]ConstraintResult, Outcome, error) {
	stateOutcome, terminal := replayStateOutcome(execution.Original.Result.State, execution.Transformed.Result.State)
	if terminal == OutcomeProviderFailed {
		return []ConstraintResult{}, terminal, nil
	}
	results := make([]ConstraintResult, 0, len(spec.Constraints))
	for _, expected := range spec.Constraints {
		if stateOutcome != "" {
			results = append(results, ConstraintResult{
				ConstraintID: expected.ID, Metric: expected.Metric, Operator: expected.Operator, Status: stateOutcome,
			})
			continue
		}
		var observed ConstraintResult
		var err error
		switch expected.Metric {
		case MetricDecision:
			original, originalErr := selectedTrajectoryIdentity(execution.Original.Result)
			transformed, transformedErr := selectedTrajectoryIdentity(execution.Transformed.Result)
			if originalErr != nil || transformedErr != nil {
				return nil, "", errors.Join(originalErr, transformedErr)
			}
			observed, err = EvaluateConstraint(expected, nil, nil, original, transformed)
		case MetricConditionalScore:
			original, originalErr := absoluteConditionalScore(execution.Original.Result)
			transformed, transformedErr := absoluteConditionalScore(execution.Transformed.Result)
			if originalErr != nil || transformedErr != nil {
				return nil, "", errors.Join(originalErr, transformedErr)
			}
			observed, err = EvaluateConstraint(expected, &original, &transformed, "", "")
		default:
			observed = ConstraintResult{
				ConstraintID: expected.ID, Metric: expected.Metric, Operator: expected.Operator, Status: ConstraintUnsupported,
			}
		}
		if err != nil {
			return nil, "", err
		}
		results = append(results, observed)
	}
	return results, outcomeForConstraintResults(results), nil
}

func replayStateOutcome(original, transformed verifier.DecisionState) (ConstraintStatus, Outcome) {
	states := []verifier.DecisionState{original, transformed}
	if slices.Contains(states, verifier.DecisionProviderFailed) {
		return "", OutcomeProviderFailed
	}
	if slices.Contains(states, verifier.DecisionBudgetExhausted) || slices.Contains(states, verifier.DecisionTied) {
		return ConstraintInconclusive, ""
	}
	if slices.Contains(states, verifier.DecisionAbstained) {
		return ConstraintAbstained, ""
	}
	return "", ""
}

func outcomeForConstraintResults(values []ConstraintResult) Outcome {
	outcome := OutcomeSatisfied
	for _, value := range values {
		switch value.Status {
		case ConstraintViolated:
			return OutcomeViolated
		case ConstraintAbstained:
			if outcome == OutcomeSatisfied {
				outcome = OutcomeAbstained
			}
		case ConstraintUnsupported:
			if outcome == OutcomeSatisfied {
				outcome = OutcomeUnsupported
			}
		case ConstraintInconclusive:
			if outcome == OutcomeSatisfied {
				outcome = OutcomeInconclusive
			}
		}
	}
	return outcome
}

func selectedTrajectoryIdentity(result verification.Result) (string, error) {
	if result.Mode != verification.ModePairwise || result.Selection == nil || result.State != verifier.DecisionSelected ||
		result.Selection.BestIndex < 0 || result.Selection.BestIndex >= len(result.Selection.TrajectoryEvidence) {
		return "", errors.New("decision relation requires one selected pairwise trajectory")
	}
	digest := result.Selection.TrajectoryEvidence[result.Selection.BestIndex].TrajectoryDigest
	if !validDigest(digest) {
		return "", errors.New("selected pairwise trajectory lacks a valid identity")
	}
	return digest, nil
}

func absoluteConditionalScore(result verification.Result) (float64, error) {
	if result.Mode != verification.ModeAbsolute || result.Absolute == nil || result.State != verifier.DecisionSelected || !finite(result.Absolute.Value) ||
		result.Absolute.Value < 0 || result.Absolute.Value > 1 {
		return 0, errors.New("conditional-score relation requires one selected absolute score in [0,1]")
	}
	return result.Absolute.Value, nil
}

func completedReplayRepetitions(execution ReplayExecution) (int, error) {
	original, err := completedSideRepetitions(execution.Original.Result)
	if err != nil {
		return 0, err
	}
	transformed, err := completedSideRepetitions(execution.Transformed.Result)
	if err != nil {
		return 0, err
	}
	if original != transformed {
		return 0, errors.New("stress replay sides completed different repetition counts")
	}
	return original, nil
}

func completedSideRepetitions(result verification.Result) (int, error) {
	switch result.Mode {
	case verification.ModeAbsolute:
		if result.Absolute == nil || len(result.Absolute.CriterionEvidence) == 0 {
			return 0, errors.New("absolute replay result has no criterion evidence")
		}
		count := -1
		for _, evidence := range result.Absolute.CriterionEvidence {
			if count < 0 {
				count = len(evidence)
			}
			if len(evidence) != count {
				return 0, errors.New("absolute replay criteria completed different repetition counts")
			}
		}
		return count, nil
	case verification.ModeDelta:
		if result.Delta == nil || result.Delta.Decision == nil {
			return 0, errors.New("delta replay result has no pair decision")
		}
		return result.Delta.Decision.RepeatCount, nil
	case verification.ModePairwise:
		if result.Selection == nil || len(result.Selection.PairDecisions) != 1 {
			return 0, errors.New("pairwise relation replay requires exactly one pair decision")
		}
		return result.Selection.PairDecisions[0].RepeatCount, nil
	default:
		return 0, errors.New("stress replay result mode is unsupported")
	}
}

func resultEvidenceStrength(result verification.Result) (verifier.EvidenceStrength, error) {
	switch {
	case result.Absolute != nil:
		return result.Absolute.EvidenceStrength, nil
	case result.Delta != nil:
		return result.Delta.EvidenceStrength, nil
	case result.Selection != nil:
		return result.Selection.EvidenceStrength, nil
	default:
		return verifier.EvidenceStrength{}, errors.New("verification result has no evidence-bearing payload")
	}
}
