package reliance

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

type relianceArmComparisonState struct {
	summary       RelianceArmSummary
	registration  ReliancePanelRegistration
	corpus        RelianceAnalysisCorpus
	replaySources map[string]provider.ExactReplaySource
}

func BuildRelianceArmComparison(
	preregistration Preregistration,
	preflight ReliancePreflight,
	arms []RelianceArmEvidence,
	specs []RelianceArmContrastSpec,
) (RelianceArmComparison, error) {
	states, err := buildRelianceArmStates(preregistration, preflight, arms)
	if err != nil {
		return RelianceArmComparison{}, err
	}
	orderedSpecs, err := canonicalRelianceContrastSpecs(specs, states)
	if err != nil {
		return RelianceArmComparison{}, err
	}
	return constructRelianceArmComparison(preregistration, preflight, states, orderedSpecs)
}

func (value RelianceArmComparison) Validate(
	preregistration Preregistration,
	preflight ReliancePreflight,
	arms []RelianceArmEvidence,
	specs []RelianceArmContrastSpec,
) error {
	expected, err := BuildRelianceArmComparison(preregistration, preflight, arms, specs)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("reliance arm comparison differs from its frozen arm evidence and contrast matrix")
	}
	return nil
}

func buildRelianceArmStates(
	preregistration Preregistration,
	preflight ReliancePreflight,
	arms []RelianceArmEvidence,
) (map[string]relianceArmComparisonState, error) {
	if err := preflight.Validate(preregistration); err != nil {
		return nil, err
	}
	if len(arms) < 2 {
		return nil, errors.New("reliance arm comparison requires at least two arms")
	}
	result := make(map[string]relianceArmComparisonState, len(arms))
	for _, arm := range arms {
		state, err := buildRelianceArmState(preregistration, preflight, arm)
		if err != nil {
			return nil, err
		}
		if _, duplicate := result[arm.ArmID]; duplicate {
			return nil, fmt.Errorf("reliance arm comparison repeats arm %q", arm.ArmID)
		}
		result[arm.ArmID] = state
	}
	return result, nil
}

func buildRelianceArmState(
	preregistration Preregistration,
	preflight ReliancePreflight,
	arm RelianceArmEvidence,
) (relianceArmComparisonState, error) {
	if !validPanelIdentifier(arm.ArmID) || !validPanelIdentifier(arm.ModelFamilyID) ||
		!validRelianceModelIdentityStatus(arm.ModelIdentityStatus) || !validRelianceDigest(arm.RouteAttestationDigest) {
		return relianceArmComparisonState{}, errors.New("reliance comparison arm identity or route attestation is invalid")
	}
	if arm.Registration.PreregistrationDigest != preregistration.Digest || arm.Registration.PreflightDigest != preflight.Digest {
		return relianceArmComparisonState{}, fmt.Errorf("reliance comparison arm %q differs from the frozen design", arm.ArmID)
	}
	corpus, err := BuildRelianceAnalysisCorpus(arm.Registration, preregistration, arm.Executions, arm.Failures)
	if err != nil {
		return relianceArmComparisonState{}, err
	}
	analysis, err := constructEvidenceRelianceAnalysis(arm.Registration, preregistration, corpus)
	if err != nil {
		return relianceArmComparisonState{}, err
	}
	summary := RelianceArmSummary{
		ArmID: arm.ArmID, ModelFamilyID: arm.ModelFamilyID, ModelIdentityStatus: arm.ModelIdentityStatus,
		RouteAttestationDigest: arm.RouteAttestationDigest,
		RegistrationDigest:     arm.Registration.Digest, CorpusDigest: corpus.Digest,
		AnalysisDigest: analysis.Digest, Arm: arm.Registration.Arm,
	}
	return relianceArmComparisonState{
		summary: summary, registration: arm.Registration, corpus: corpus,
		replaySources: relianceArmReplaySources(arm.Executions),
	}, nil
}

func relianceArmReplaySources(executions []EvidenceTaskPanelExecution) map[string]provider.ExactReplaySource {
	result := make(map[string]provider.ExactReplaySource, len(executions))
	for _, execution := range executions {
		result[execution.SourceTaskID] = execution.ReplaySource
	}
	return result
}

func canonicalRelianceContrastSpecs(
	specs []RelianceArmContrastSpec,
	states map[string]relianceArmComparisonState,
) ([]RelianceArmContrastSpec, error) {
	if len(specs) == 0 {
		return nil, errors.New("reliance arm comparison requires at least one prespecified contrast")
	}
	result := slices.Clone(specs)
	slices.SortFunc(result, func(left, right RelianceArmContrastSpec) int {
		return strings.Compare(left.ContrastID, right.ContrastID)
	})
	seenContrasts := make(map[string]struct{}, len(result))
	for index, spec := range result {
		if err := validateRelianceContrastSpec(spec, states); err != nil {
			return nil, err
		}
		if index > 0 && result[index-1].ContrastID == spec.ContrastID {
			return nil, fmt.Errorf("reliance arm comparison repeats contrast %q", spec.ContrastID)
		}
		key := string(spec.Kind) + "\x00" + spec.ReferenceArmID + "\x00" + spec.ComparatorArmID
		if _, duplicate := seenContrasts[key]; duplicate {
			return nil, fmt.Errorf("reliance arm comparison repeats contrast dimensions under %q", spec.ContrastID)
		}
		seenContrasts[key] = struct{}{}
	}
	return result, nil
}

func validateRelianceContrastSpec(
	spec RelianceArmContrastSpec,
	states map[string]relianceArmComparisonState,
) error {
	if !validPanelIdentifier(spec.ContrastID) || spec.ReferenceArmID == spec.ComparatorArmID ||
		!validRelianceContrastKind(spec.Kind) {
		return fmt.Errorf("reliance arm contrast %q identity or kind is invalid", spec.ContrastID)
	}
	if _, found := states[spec.ReferenceArmID]; !found {
		return fmt.Errorf("reliance arm contrast %q has unknown reference arm", spec.ContrastID)
	}
	if _, found := states[spec.ComparatorArmID]; !found {
		return fmt.Errorf("reliance arm contrast %q has unknown comparator arm", spec.ContrastID)
	}
	return nil
}

func validRelianceContrastKind(value RelianceArmContrastKind) bool {
	return value == RelianceContrastEvidencePolicy || value == RelianceContrastEntrypoint ||
		value == RelianceContrastModelFamily || value == RelianceContrastProvider || value == RelianceContrastRoute
}

func validRelianceModelIdentityStatus(value RelianceModelIdentityStatus) bool {
	return value == RelianceModelIdentityAliasOnly || value == RelianceModelIdentityNamedFamilyEvidenceBound
}
