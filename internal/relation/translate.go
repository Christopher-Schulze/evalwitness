package relation

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

const (
	TranslationResultSchemaVersionV1 = "evalwitness.relation-translation-result.v1"
	TranslationResultSchemaVersionV2 = "evalwitness.relation-translation-result.v2"
	TranslationResultSchemaVersionV3 = "evalwitness.relation-translation-result.v3"
	TranslationResultSchemaVersion   = TranslationResultSchemaVersionV1
)

func Translate(plan Plan, family mutation.Family, observations []AxisObservation) (TranslationResult, error) {
	if err := validateVersionedPlanConsumer(plan); err != nil {
		return TranslationResult{}, err
	}
	contract, exists := familyContract(plan, family)
	if !exists {
		return TranslationResult{}, fmt.Errorf("relation family %q is not governed by the plan", family)
	}
	return translateWithContract(plan.ProtocolVersion, plan.Digest, contract, observations)
}

func translateWithContract(protocolVersion, planDigest string, contract FamilyContract, observations []AxisObservation) (TranslationResult, error) {
	if err := validateObservations(contract, observations); err != nil {
		return TranslationResult{}, err
	}
	values := make(map[Axis]NormalizedRating, len(observations))
	for _, observation := range observations {
		values[observation.Axis] = observation.Rating
	}
	supportAxes := matchedConditions(contract.SupportAll, values)
	contradictAxes := matchedConditions(contract.ContradictAny, values)
	state := TranslationUnresolved
	switch {
	case observationRequiresUnresolved(observations) || len(supportAxes) == len(contract.SupportAll) && len(contradictAxes) > 0:
		state = TranslationUnresolved
	case len(contradictAxes) > 0:
		state = TranslationContradicts
	case len(supportAxes) == len(contract.SupportAll):
		state = TranslationSupports
	}
	result := TranslationResult{
		ProtocolVersion: protocolVersion, Objective: ReviewObjectiveControlledRelation, PlanDigest: planDigest,
		Family: contract.Family, ExpectedRelation: contract.ExpectedRelation, Observations: append([]AxisObservation(nil), observations...),
		MatchedSupportAxes: supportAxes, MatchedContradictAxes: contradictAxes, State: state,
		ReasonCodes: translationReasons(contract, observations, contradictAxes, state),
	}
	return SealTranslationResult(result)
}

func SealTranslationResult(result TranslationResult) (TranslationResult, error) {
	schemaVersion, err := schemaVersionForProtocol(result.ProtocolVersion, TranslationResultSchemaVersionV1, TranslationResultSchemaVersionV2, TranslationResultSchemaVersionV3)
	if err != nil {
		return TranslationResult{}, err
	}
	result.SchemaVersion, result.CanonicalPolicy, result.Digest = schemaVersion, CanonicalPolicy, ""
	digest, err := translationResultDigest(result)
	if err != nil {
		return TranslationResult{}, err
	}
	result.Digest = digest
	return result, result.Validate()
}

func (result TranslationResult) Validate() error {
	if !validVersionedIdentity(result.SchemaVersion, result.ProtocolVersion, TranslationResultSchemaVersionV1, TranslationResultSchemaVersionV2, TranslationResultSchemaVersionV3) || result.CanonicalPolicy != CanonicalPolicy ||
		result.Objective != ReviewObjectiveControlledRelation || !validDigest(result.PlanDigest) || result.Family == "" || result.ExpectedRelation == "" ||
		len(result.Observations) == 0 || !slices.IsSorted(result.MatchedSupportAxes) || hasDuplicate(result.MatchedSupportAxes) ||
		!slices.IsSorted(result.MatchedContradictAxes) || hasDuplicate(result.MatchedContradictAxes) || len(result.ReasonCodes) == 0 ||
		!slices.IsSorted(result.ReasonCodes) || hasDuplicate(result.ReasonCodes) ||
		!slices.Contains([]TranslationState{TranslationSupports, TranslationContradicts, TranslationUnresolved}, result.State) {
		return errors.New("relation translation result identity, objective, evidence, state, or reasons are invalid")
	}
	expected, err := translationResultDigest(result)
	if err != nil || result.Digest != expected {
		return errors.New("relation translation result digest is invalid")
	}
	return nil
}

func validateObservations(contract FamilyContract, observations []AxisObservation) error {
	if len(observations) != len(contract.RequiredAxes) {
		return fmt.Errorf("relation family %q requires exactly %d observations", contract.Family, len(contract.RequiredAxes))
	}
	for index, observation := range observations {
		if observation.Axis != contract.RequiredAxes[index] || !validNormalizedRating(observation.Axis, observation.Rating) {
			return fmt.Errorf("relation family %q observation %d has the wrong axis, order, or rating", contract.Family, index)
		}
	}
	return nil
}

func validNormalizedRating(axis Axis, rating NormalizedRating) bool {
	switch axis {
	case AxisInformation:
		return slices.Contains([]NormalizedRating{NormalizedIndeterminate, NormalizedInsufficient, NormalizedSufficient}, rating)
	case AxisUntrustedControl:
		return slices.Contains([]NormalizedRating{NormalizedControlEffect, NormalizedIndeterminate, NormalizedNoControl, NormalizedNotApplicable}, rating)
	default:
		return slices.Contains([]NormalizedRating{NormalizedEqual, NormalizedIndeterminate, NormalizedNotApplicable, NormalizedOriginal, NormalizedTransformed}, rating)
	}
}

func matchedConditions(conditions []TranslationCondition, values map[Axis]NormalizedRating) []Axis {
	matched := make([]Axis, 0, len(conditions))
	for _, condition := range conditions {
		if slices.Contains(condition.Ratings, values[condition.Axis]) {
			matched = append(matched, condition.Axis)
		}
	}
	slices.Sort(matched)
	return matched
}

func observationRequiresUnresolved(observations []AxisObservation) bool {
	for _, observation := range observations {
		if observation.Rating == NormalizedIndeterminate || observation.Rating == NormalizedNotApplicable ||
			observation.Axis == AxisInformation && observation.Rating == NormalizedInsufficient {
			return true
		}
	}
	return false
}

func translationReasons(contract FamilyContract, observations []AxisObservation, contradicted []Axis, state TranslationState) []ReasonCode {
	reasons := make([]ReasonCode, 0, 2)
	switch state {
	case TranslationSupports:
		switch contract.ExpectedRelation {
		case mutation.RelationQualityEqualEvidenceLow:
			reasons = append(reasons, ReasonEvidenceOnlyChange)
		case mutation.RelationVerifiedOutcomeWins:
			reasons = append(reasons, ReasonExecutableSupportDiffers)
		default:
			reasons = append(reasons, ReasonNoJudgmentChange)
		}
	case TranslationContradicts:
		for _, axis := range contradicted {
			reasons = append(reasons, reasonForAxis(axis))
		}
	case TranslationUnresolved:
		reasons = append(reasons, ReasonHiddenContextRequired)
		for _, observation := range observations {
			if observation.Rating == NormalizedIndeterminate || observation.Rating == NormalizedNotApplicable || observation.Rating == NormalizedInsufficient {
				reasons = append(reasons, ReasonInsufficientInformation)
				break
			}
		}
	}
	slices.Sort(reasons)
	return slices.Compact(reasons)
}

func reasonForAxis(axis Axis) ReasonCode {
	switch axis {
	case AxisCausalIntegrity:
		return ReasonCausalIntegrityDiffers
	case AxisEvidenceStrength:
		return ReasonEvidenceStrengthDiffers
	case AxisExecutableSupport:
		return ReasonExecutableSupportDiffers
	case AxisPresentation:
		return ReasonPresentationDiffers
	case AxisSemanticQuality:
		return ReasonTaskQualityDiffers
	case AxisUntrustedControl:
		return ReasonUntrustedContentControls
	default:
		return ReasonInsufficientInformation
	}
}

func familyContract(plan Plan, family mutation.Family) (FamilyContract, bool) {
	for _, contract := range plan.Families {
		if contract.Family == family {
			return contract, true
		}
	}
	return FamilyContract{}, false
}

func translationResultDigest(result TranslationResult) (string, error) {
	result.Digest = ""
	return digestJSON(result)
}
