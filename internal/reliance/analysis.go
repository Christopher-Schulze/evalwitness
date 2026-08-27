package reliance

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

func AnalyzeEvidenceReliance(
	registration ReliancePanelRegistration,
	preregistration Preregistration,
	executions []EvidenceTaskPanelExecution,
	failures []RelianceCellFailureReceipt,
) (EvidenceRelianceAnalysis, error) {
	corpus, err := BuildRelianceAnalysisCorpus(registration, preregistration, executions, failures)
	if err != nil {
		return EvidenceRelianceAnalysis{}, err
	}
	return constructEvidenceRelianceAnalysis(registration, preregistration, corpus)
}

func (value EvidenceRelianceAnalysis) Validate(
	registration ReliancePanelRegistration,
	preregistration Preregistration,
	executions []EvidenceTaskPanelExecution,
	failures []RelianceCellFailureReceipt,
) error {
	expected, err := AnalyzeEvidenceReliance(registration, preregistration, executions, failures)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("evidence-reliance analysis differs from its registered panel corpus and clustered fits")
	}
	return nil
}

func constructEvidenceRelianceAnalysis(
	registration ReliancePanelRegistration,
	preregistration Preregistration,
	corpus RelianceAnalysisCorpus,
) (EvidenceRelianceAnalysis, error) {
	fits := make([]EvidenceRelianceOutcomeFit, len(preregistration.PrimaryOutcomes))
	for outcomeIndex, outcome := range preregistration.PrimaryOutcomes {
		fit, err := fitEvidenceRelianceOutcome(corpus, preregistration, outcomeIndex, outcome.OutcomeID)
		if err != nil {
			return EvidenceRelianceAnalysis{}, err
		}
		fits[outcomeIndex] = fit
	}
	value := EvidenceRelianceAnalysis{
		SchemaVersion: EvidenceRelianceAnalysisSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RegistrationDigest: registration.Digest, CorpusDigest: corpus.Digest,
		PreregistrationDigest: preregistration.Digest, MultiplicityMethod: preregistration.Multiplicity.Method,
		MultiplicityFamilySize: preregistration.Multiplicity.FamilySize, OutcomeFits: fits,
	}
	digest, err := evidenceRelianceAnalysisDigest(value)
	if err != nil {
		return EvidenceRelianceAnalysis{}, err
	}
	value.Digest = digest
	return value, validateEvidenceRelianceAnalysisStructure(value, corpus, preregistration)
}

func fitEvidenceRelianceOutcome(
	corpus RelianceAnalysisCorpus,
	preregistration Preregistration,
	outcomeIndex int,
	outcomeID OutcomeID,
) (EvidenceRelianceOutcomeFit, error) {
	observations := make([]stats.FactorialObservation, 0, corpus.OutcomeBearingCells)
	for _, cell := range corpus.Cells {
		if len(cell.OutcomeValues) == len(preregistration.PrimaryOutcomes) {
			observations = append(observations, factorialObservationFromCell(cell, outcomeIndex))
		}
	}
	result := EvidenceRelianceOutcomeFit{
		OutcomeID: outcomeID, RegisteredCells: corpus.RegisteredCells,
		EligibleObservations: len(observations), ExcludedFromFit: corpus.RegisteredCells - len(observations),
	}
	if len(observations) == 0 {
		result.Status = RelianceFitInconclusive
		result.Reason = "clustered factorial fit unavailable: no outcome-bearing cells"
		return result, nil
	}
	fit, err := stats.FitClusteredFactorial(
		referenceFactorialTerms(), observations, referenceNominalAlpha, preregistration.Multiplicity.FamilySize,
	)
	if err != nil {
		if !errors.Is(err, stats.ErrFactorialNotEstimable) {
			return EvidenceRelianceOutcomeFit{}, fmt.Errorf("fit evidence-reliance outcome %q: %w", outcomeID, err)
		}
		result.Status = RelianceFitInconclusive
		result.Reason = "clustered factorial fit unavailable: " + err.Error()
		return result, nil
	}
	result.Status = RelianceFitMeasured
	result.Fit = &fit
	return result, nil
}

func validateEvidenceRelianceAnalysisStructure(
	value EvidenceRelianceAnalysis,
	corpus RelianceAnalysisCorpus,
	preregistration Preregistration,
) error {
	if value.SchemaVersion != EvidenceRelianceAnalysisSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.RegistrationDigest != corpus.RegistrationDigest || value.CorpusDigest != corpus.Digest ||
		value.PreregistrationDigest != preregistration.Digest || value.MultiplicityMethod != preregistration.Multiplicity.Method ||
		value.MultiplicityFamilySize != preregistration.Multiplicity.FamilySize ||
		len(value.OutcomeFits) != len(preregistration.PrimaryOutcomes) {
		return errors.New("evidence-reliance analysis identity, multiplicity, or outcome coverage is invalid")
	}
	for index, fit := range value.OutcomeFits {
		if err := validateEvidenceRelianceOutcomeFit(fit, preregistration.PrimaryOutcomes[index].OutcomeID, corpus, preregistration); err != nil {
			return err
		}
	}
	digest, err := evidenceRelianceAnalysisDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("evidence-reliance analysis digest is invalid")
	}
	return nil
}

func validateEvidenceRelianceOutcomeFit(
	value EvidenceRelianceOutcomeFit,
	outcomeID OutcomeID,
	corpus RelianceAnalysisCorpus,
	preregistration Preregistration,
) error {
	if value.OutcomeID != outcomeID || value.RegisteredCells != corpus.RegisteredCells ||
		value.EligibleObservations != corpus.OutcomeBearingCells ||
		value.ExcludedFromFit != corpus.RegisteredCells-corpus.OutcomeBearingCells {
		return fmt.Errorf("evidence-reliance outcome %q denominator accounting is invalid", outcomeID)
	}
	if value.Status == RelianceFitInconclusive {
		if value.Reason == "" || value.Fit != nil {
			return fmt.Errorf("inconclusive evidence-reliance outcome %q is malformed", outcomeID)
		}
		return nil
	}
	if value.Status != RelianceFitMeasured || value.Reason != "" || value.Fit == nil ||
		value.Fit.Observations != value.EligibleObservations || value.Fit.FamilySize != preregistration.Multiplicity.FamilySize ||
		value.Fit.Parameters != len(referenceFactorialTerms())+1 || value.Fit.Rank != len(referenceFactorialTerms())+1 {
		return fmt.Errorf("measured evidence-reliance outcome %q clustered fit is invalid", outcomeID)
	}
	return nil
}

func evidenceRelianceAnalysisDigest(value EvidenceRelianceAnalysis) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}
