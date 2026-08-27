package reliance

import (
	"errors"
	"slices"
)

func validatePublishedRelianceWitnesses(analysisDigest string, values []RelianceMapWitness) error {
	if len(values) == 0 {
		return errors.New("evidence reliance map lacks a published witness")
	}
	previous := ""
	for _, published := range values {
		if published.SourceAnalysisDigest != analysisDigest || published.BindingStatus != RelianceWitnessRelationBinding {
			return errors.New("published reliance witness analysis or attribution boundary is invalid")
		}
		if published.Witness.Digest <= previous {
			return errors.New("published reliance witnesses are duplicated or noncanonical")
		}
		if err := validatePublishedRelianceWitness(published.Witness); err != nil {
			return err
		}
		previous = published.Witness.Digest
	}
	return nil
}

func validatePublishedRelianceWitness(value RelianceWitness) error {
	if value.SchemaVersion != RelianceWitnessSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.EvaluationPolicy != RelianceWitnessEvaluationPolicy || !value.PublicReleaseAllowed || value.NetworkRequired ||
		!validPanelIdentifier(value.CaseID) || len(value.Evaluations) == 0 {
		return errors.New("published reliance witness identity, release permission, or execution boundary is invalid")
	}
	for _, digest := range []string{
		value.AdmissionDigest, value.RelationDigest, value.SourceReplayDigest, value.SourceBatchRunFingerprint,
		value.SourceReplayCaptureDigest, value.OutcomeIdentityDigest, value.OriginalRelianceResultDigest,
		value.FinalEvaluationDigest, value.Digest,
	} {
		if !validRelianceDigest(digest) {
			return errors.New("published reliance witness contains an invalid evidence digest")
		}
	}
	if err := validatePublishedRelianceCounterexampleBinding(value); err != nil {
		return err
	}
	if err := validateRelianceWitnessTrace(value, value.OutcomeIdentityDigest, value.OriginalRelianceResultDigest,
		value.Evaluations[0].InterventionValidityDigest, value.SourceReplayCaptureDigest); err != nil {
		return err
	}
	digest, err := relianceWitnessDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("published reliance witness digest is invalid")
	}
	return nil
}

func validatePublishedRelianceCounterexampleBinding(value RelianceWitness) error {
	if err := value.Counterexample.Validate(); err != nil {
		return err
	}
	counterexample := value.Counterexample
	if counterexample.RelationDigest != value.RelationDigest || counterexample.CaseID != value.CaseID ||
		counterexample.SourceResultDigest != value.SourceReplayDigest ||
		counterexample.OriginalObservation.ReplayResultDigest != value.SourceReplayDigest ||
		counterexample.PublicReleaseAllowed != value.PublicReleaseAllowed ||
		!slices.Equal(value.FinalUnits, counterexample.FinalUnits) {
		return errors.New("published reliance witness counterexample binding is invalid")
	}
	return nil
}
