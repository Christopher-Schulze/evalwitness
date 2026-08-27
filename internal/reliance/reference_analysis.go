package reliance

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

const referenceNominalAlpha = 0.05

func AnalyzeReferenceCorpus(
	corpus ReferenceCorpus,
	preregistration Preregistration,
) (ReferenceAnalysis, error) {
	if err := corpus.Validate(preregistration); err != nil {
		return ReferenceAnalysis{}, err
	}
	return constructReferenceAnalysis(corpus, preregistration)
}

func (value ReferenceAnalysis) Validate(
	corpus ReferenceCorpus,
	preregistration Preregistration,
) error {
	if err := corpus.Validate(preregistration); err != nil {
		return err
	}
	if value.SchemaVersion != ReferenceAnalysisSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.CorpusDigest != corpus.Digest || value.PreregistrationDigest != preregistration.Digest ||
		len(value.OutcomeFits) != len(preregistration.PrimaryOutcomes) {
		return errors.New("reliance reference analysis identity or outcome coverage is invalid")
	}
	digest, err := referenceAnalysisDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance reference analysis digest is invalid")
	}
	expected, err := constructReferenceAnalysis(corpus, preregistration)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("reliance reference analysis differs from the deterministic clustered fit")
	}
	return nil
}

func constructReferenceAnalysis(
	corpus ReferenceCorpus,
	preregistration Preregistration,
) (ReferenceAnalysis, error) {
	terms := referenceFactorialTerms()
	fits := make([]ReferenceOutcomeFit, len(preregistration.PrimaryOutcomes))
	for outcomeIndex, outcome := range preregistration.PrimaryOutcomes {
		observations := make([]stats.FactorialObservation, len(corpus.Records))
		for recordIndex, record := range corpus.Records {
			outcomeValue, err := referenceOutcome(record, outcome.OutcomeID)
			if err != nil {
				return ReferenceAnalysis{}, err
			}
			observations[recordIndex] = stats.FactorialObservation{
				ObservationID: record.ObservationID, ClusterID: record.SourceTaskID,
				Levels: slices.Clone(record.Levels), Outcome: outcomeValue,
			}
		}
		fit, err := stats.FitClusteredFactorial(
			terms,
			observations,
			referenceNominalAlpha,
			preregistration.Multiplicity.FamilySize,
		)
		if err != nil {
			return ReferenceAnalysis{}, fmt.Errorf("fit reference outcome %q: %w", outcome.OutcomeID, err)
		}
		fits[outcomeIndex] = ReferenceOutcomeFit{OutcomeID: outcome.OutcomeID, Fit: fit}
	}
	value := ReferenceAnalysis{
		SchemaVersion: ReferenceAnalysisSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		CorpusDigest: corpus.Digest, PreregistrationDigest: preregistration.Digest, OutcomeFits: fits,
	}
	digest, err := referenceAnalysisDigest(value)
	if err != nil {
		return ReferenceAnalysis{}, err
	}
	value.Digest = digest
	return value, nil
}

func referenceOutcome(record ReferenceRecord, outcomeID OutcomeID) (float64, error) {
	switch outcomeID {
	case OutcomeAbstentionTransition:
		return binaryReferenceOutcome(record.AbstentionTransition), nil
	case OutcomeConditionalMean:
		if record.Comparison.ConditionalScoreMovement == nil {
			return 0, errors.New("reference record lacks conditional-score movement")
		}
		return *record.Comparison.ConditionalScoreMovement, nil
	case OutcomeConditionalVariance:
		if record.Comparison.ConditionalVarianceMovement == nil {
			return 0, errors.New("reference record lacks conditional-variance movement")
		}
		return *record.Comparison.ConditionalVarianceMovement, nil
	case OutcomeDecisionFlip:
		return binaryReferenceOutcome(record.DecisionFlip), nil
	case OutcomeSupportJaccard:
		return record.Comparison.SupportJaccard, nil
	case OutcomeValidMass:
		return record.Comparison.ValidScoreMassMovement, nil
	case OutcomeVisibleMass:
		return record.Comparison.VisibleMassMovement, nil
	default:
		return 0, fmt.Errorf("unknown reference outcome %q", outcomeID)
	}
}

func binaryReferenceOutcome(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func referenceFactorialTerms() []stats.FactorialTerm {
	effects := canonicalReferenceTermEffects()
	terms := make([]stats.FactorialTerm, len(effects))
	for index, effect := range effects {
		terms[index] = stats.FactorialTerm{ID: effect.Term.ID, Factors: slices.Clone(effect.Term.Factors)}
	}
	return terms
}

func referenceAnalysisDigest(value ReferenceAnalysis) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
