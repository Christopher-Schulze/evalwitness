package stress

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

func SealRelation(value Relation) (Relation, error) {
	value.SchemaVersion = RelationSchemaVersion
	value.CanonicalPolicy = CanonicalPolicy
	value.Applicability.RequiredSourceFormats = append([]preprocess.SourceFormat(nil), value.Applicability.RequiredSourceFormats...)
	value.Applicability.Requirements = append([]SourceRequirement(nil), value.Applicability.Requirements...)
	value.Constraints = cloneExpectedConstraints(value.Constraints)
	value.InvalidStates = append([]InvalidState(nil), value.InvalidStates...)
	value.StageExpectations = append([]StageExpectation(nil), value.StageExpectations...)
	slices.Sort(value.Applicability.RequiredSourceFormats)
	sort.Slice(value.Applicability.Requirements, func(left, right int) bool {
		if value.Applicability.Requirements[left].Kind != value.Applicability.Requirements[right].Kind {
			return value.Applicability.Requirements[left].Kind < value.Applicability.Requirements[right].Kind
		}
		return value.Applicability.Requirements[left].Value < value.Applicability.Requirements[right].Value
	})
	sort.Slice(value.Constraints, func(left, right int) bool { return value.Constraints[left].ID < value.Constraints[right].ID })
	slices.Sort(value.InvalidStates)
	sort.Slice(value.StageExpectations, func(left, right int) bool {
		return stageIndex(value.StageExpectations[left].Stage) < stageIndex(value.StageExpectations[right].Stage)
	})
	value.Digest = ""
	digest, err := digestDocument(value)
	if err != nil {
		return Relation{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return Relation{}, err
	}
	return value, nil
}

func (value Relation) Validate() error {
	if value.SchemaVersion != RelationSchemaVersion || value.CanonicalPolicy != CanonicalPolicy || !identifierPattern.MatchString(value.ID) || value.Revision < 1 {
		return errors.New("stress relation identity or revision is invalid")
	}
	if !slices.Contains([]RelationKind{KindInvariance, KindSensitivity, KindDifferential}, value.Kind) {
		return errors.New("stress relation kind is invalid")
	}
	if err := validateApplicability(value.Applicability); err != nil {
		return err
	}
	if err := validateTransform(value.Transform); err != nil {
		return err
	}
	if err := validateTransformApplicability(value.Transform, value.Applicability, value.Kind); err != nil {
		return err
	}
	if err := validateConstraints(value.Constraints); err != nil {
		return err
	}
	if err := validateInvalidStates(value.InvalidStates); err != nil {
		return err
	}
	if err := validateRepeatPolicy(value.Repeat); err != nil {
		return err
	}
	if err := validateStatisticalFamily(value.StatisticalFamily, value.Transform); err != nil {
		return err
	}
	if err := validateStageExpectations(value.StageExpectations); err != nil {
		return err
	}
	if err := validateTransformStageExpectations(value.Transform, value.StageExpectations); err != nil {
		return err
	}
	expected, err := relationDigest(value)
	if err != nil || value.Digest != expected {
		return errors.New("stress relation digest is invalid")
	}
	return nil
}

func validateApplicability(value Applicability) error {
	if !slices.Contains([]Unit{UnitTrajectory, UnitCandidatePair, UnitTraceMapping, UnitEntrypoint, UnitProviderRoute, UnitExtractionPolicy}, value.Unit) ||
		value.MinimumTrajectories < 1 || value.MaximumTrajectories < value.MinimumTrajectories || value.MaximumTrajectories > 10 ||
		len(value.RequiredSourceFormats) == 0 || len(value.Requirements) == 0 {
		return errors.New("stress relation applicability is incomplete")
	}
	if value.Unit == UnitCandidatePair && (value.MinimumTrajectories != 2 || value.MaximumTrajectories != 2) {
		return errors.New("candidate-pair relation applicability requires exactly two trajectories")
	}
	for index, format := range value.RequiredSourceFormats {
		if !validSourceFormat(format) || index > 0 && value.RequiredSourceFormats[index-1] >= format {
			return errors.New("stress relation source formats must be valid, unique, and sorted")
		}
	}
	for index, requirement := range value.Requirements {
		if !validSourceRequirement(requirement.Kind) || strings.TrimSpace(requirement.Value) == "" || requirement.Value != strings.TrimSpace(requirement.Value) {
			return fmt.Errorf("stress relation source requirement %d is invalid", index)
		}
		if index > 0 {
			previous := value.Requirements[index-1]
			if previous.Kind > requirement.Kind || previous.Kind == requirement.Kind && previous.Value >= requirement.Value {
				return errors.New("stress relation source requirements must be unique and sorted")
			}
		}
	}
	return nil
}

func validateTransform(value Transform) error {
	if !slices.Contains([]TransformKind{TransformMutation, TransformTraceMapping, TransformEntrypoint, TransformProviderRoute, TransformExtractionMode}, value.Kind) ||
		!identifierPattern.MatchString(value.Identifier) || strings.TrimSpace(value.Version) == "" || value.Version != strings.TrimSpace(value.Version) ||
		!validStage(value.DeclaredChangedLayer) {
		return errors.New("stress relation transform identity is invalid")
	}
	if value.Kind != TransformMutation {
		if value.MutationFamily != "" || value.InterventionClass != "" || value.ExpectedFormalRelation != "" {
			return errors.New("non-mutation stress transform carries mutation-only fields")
		}
		return nil
	}
	definition, exists := mutation.DefinitionFor(value.MutationFamily)
	if !exists || value.Version != mutation.MutationProgramVersionV3 || value.Identifier != definition.Operator ||
		value.InterventionClass != definition.Class || value.ExpectedFormalRelation != definition.Relation || value.DeclaredChangedLayer != StageIngestion {
		return errors.New("mutation stress transform differs from the registered v3 mutation contract")
	}
	return nil
}

func validateTransformApplicability(transform Transform, applicability Applicability, kind RelationKind) error {
	wantUnit := UnitTrajectory
	switch transform.Kind {
	case TransformMutation:
		definition, exists := mutation.DefinitionFor(transform.MutationFamily)
		if !exists {
			return errors.New("stress mutation transform has no registered definition")
		}
		if definition.PairLevel {
			wantUnit = UnitCandidatePair
		}
		wantKind := KindSensitivity
		if slices.Contains([]mutation.Relation{mutation.RelationQualityEqual, mutation.RelationNoControlEffect}, transform.ExpectedFormalRelation) {
			wantKind = KindInvariance
		}
		if transform.ExpectedFormalRelation != mutation.RelationAmbiguous && kind != wantKind {
			return errors.New("stress relation kind differs from the registered mutation relation semantics")
		}
		required := []SourceRequirementKind{RequirementV3Manifest, RequirementV3ConstructFirewall, RequirementFormalWitness, RequirementExactReplay, RequirementOwnerAttestation}
		for _, requirement := range required {
			if !hasSourceRequirement(applicability.Requirements, requirement) {
				return fmt.Errorf("mutation stress relation is missing required %q evidence", requirement)
			}
		}
	case TransformTraceMapping:
		wantUnit = UnitTraceMapping
	case TransformEntrypoint:
		wantUnit = UnitEntrypoint
	case TransformProviderRoute:
		wantUnit = UnitProviderRoute
	case TransformExtractionMode:
		wantUnit = UnitExtractionPolicy
	}
	if applicability.Unit != wantUnit {
		return errors.New("stress transform and applicability units disagree")
	}
	return nil
}

func validateConstraints(values []ExpectedConstraint) error {
	if len(values) == 0 {
		return errors.New("stress relation requires at least one expected constraint")
	}
	for index, value := range values {
		if !identifierPattern.MatchString(value.ID) || !validMetric(value.Metric) || !validOperator(value.Operator) ||
			math.IsNaN(value.AbsoluteTolerance) || math.IsInf(value.AbsoluteTolerance, 0) || value.AbsoluteTolerance < 0 ||
			math.IsNaN(value.MinimumEffect) || math.IsInf(value.MinimumEffect, 0) || value.MinimumEffect < 0 {
			return fmt.Errorf("stress relation constraint %d is invalid", index)
		}
		if index > 0 && values[index-1].ID >= value.ID {
			return errors.New("stress relation constraints must be unique and identity-sorted")
		}
		if value.Metric == MetricDecision {
			if value.TargetValue != nil || value.AbsoluteTolerance != 0 || value.MinimumEffect != 0 || !slices.Contains([]Operator{OperatorEqual, OperatorNotEqual, OperatorOriginalPreferred, OperatorTransformedPreferred}, value.Operator) {
				return fmt.Errorf("decision constraint %q has numeric tolerance or an invalid operator", value.ID)
			}
			if value.Operator == OperatorOriginalPreferred && value.TargetState != "original" || value.Operator == OperatorTransformedPreferred && value.TargetState != "transformed" {
				return fmt.Errorf("decision preference constraint %q requires one exact target state", value.ID)
			}
			if slices.Contains([]Operator{OperatorEqual, OperatorNotEqual}, value.Operator) && value.TargetState != "" {
				return fmt.Errorf("decision equality constraint %q cannot carry a target state", value.ID)
			}
		} else if comparisonMetric(value.Metric) {
			if value.TargetValue == nil || !finite(*value.TargetValue) || *value.TargetValue < 0 || *value.TargetValue > 1 || value.MinimumEffect != 0 ||
				!slices.Contains([]Operator{OperatorEqual, OperatorNotEqual, OperatorLessOrEqual, OperatorGreaterOrEqual}, value.Operator) || value.TargetState != "" {
				return fmt.Errorf("pair-comparison constraint %q requires one [0,1] target, zero minimum effect, and a numeric operator", value.ID)
			}
		} else if value.TargetValue != nil {
			return fmt.Errorf("movement constraint %q cannot carry a scalar comparison target", value.ID)
		} else if slices.Contains([]Operator{OperatorOriginalPreferred, OperatorTransformedPreferred}, value.Operator) && value.Metric != MetricRank {
			return fmt.Errorf("preference operator is invalid for metric %q", value.Metric)
		} else if value.TargetState != "" {
			return fmt.Errorf("numeric constraint %q cannot carry a target state", value.ID)
		}
	}
	return nil
}

func cloneExpectedConstraints(values []ExpectedConstraint) []ExpectedConstraint {
	result := append([]ExpectedConstraint(nil), values...)
	for index := range result {
		if values[index].TargetValue != nil {
			target := *values[index].TargetValue
			result[index].TargetValue = &target
		}
	}
	return result
}

func comparisonMetric(value Metric) bool {
	return slices.Contains([]Metric{MetricSupportJaccard, MetricProbabilityOverlap, MetricCommonSupportDivergence}, value)
}

func validateInvalidStates(values []InvalidState) error {
	if len(values) == 0 {
		return errors.New("stress relation requires explicit invalid states")
	}
	for index, value := range values {
		if !validInvalidState(value) || index > 0 && values[index-1] >= value {
			return errors.New("stress relation invalid states must be valid, unique, and sorted")
		}
	}
	return nil
}

func validateRepeatPolicy(value RepeatPolicy) error {
	if !slices.Contains([]RepeatKind{RepeatFixed, RepeatRegisteredAdaptive}, value.Kind) || value.MinimumRepetitions < 1 ||
		value.MaximumRepetitions < value.MinimumRepetitions || value.MaximumRepetitions > 16 || strings.TrimSpace(value.StopRule) == "" || value.StopRule != strings.TrimSpace(value.StopRule) {
		return errors.New("stress relation repeat policy is invalid")
	}
	if value.Kind == RepeatFixed && (value.MinimumRepetitions != value.MaximumRepetitions || value.StopRule != "fixed_repetitions") {
		return errors.New("fixed stress relation repeat policy must use one fixed repetition count")
	}
	if value.Kind == RepeatRegisteredAdaptive && value.MinimumRepetitions == value.MaximumRepetitions {
		return errors.New("adaptive stress relation repeat policy has no adaptive range")
	}
	return nil
}

func validateStatisticalFamily(value StatisticalFamily, transform Transform) error {
	if !identifierPattern.MatchString(value.ID) || !slices.Contains([]Estimand{EstimandPrimaryCore, EstimandScarcitySentinel, EstimandSensitivity, EstimandDiagnostic}, value.Estimand) ||
		!identifierPattern.MatchString(value.ClusterUnit) || strings.TrimSpace(value.MultiplicityMethod) == "" || value.MultiplicityMethod != strings.TrimSpace(value.MultiplicityMethod) ||
		!validDenominatorPolicy(value.DenominatorPolicy) || value.FailurePolicy != canonicalFailurePolicy() {
		return errors.New("stress relation statistical family is invalid")
	}
	if value.Estimand == EstimandPrimaryCore && value.DenominatorPolicy != DenominatorPrimaryHumanSupported ||
		value.Estimand == EstimandSensitivity && value.DenominatorPolicy != DenominatorSensitivityStratified ||
		value.Estimand == EstimandScarcitySentinel && value.DenominatorPolicy != DenominatorScarcityAvailability {
		return errors.New("stress relation estimand and denominator policy disagree")
	}
	if transform.Kind == TransformMutation && transform.MutationFamily == mutation.FamilyTestEvidenceOmitted && value.Estimand == EstimandPrimaryCore {
		return errors.New("omitted-evidence scarcity sentinel cannot enter the primary-core estimand")
	}
	if value.Estimand == EstimandScarcitySentinel && (transform.Kind != TransformMutation || transform.MutationFamily != mutation.FamilyTestEvidenceOmitted || value.MultiplicityMethod != "none_descriptive") {
		return errors.New("scarcity-sentinel estimand must be the descriptive omitted-evidence relation")
	}
	return nil
}

func validDenominatorPolicy(value DenominatorPolicy) bool {
	return slices.Contains([]DenominatorPolicy{
		DenominatorPrimaryHumanSupported, DenominatorSensitivityStratified, DenominatorScarcityAvailability,
	}, value)
}

func validateStageExpectations(values []StageExpectation) error {
	if len(values) != len(orderedStages()) {
		return errors.New("stress relation must define every stage expectation exactly once")
	}
	for index, stage := range orderedStages() {
		if values[index].Stage != stage || !slices.Contains([]StageExpectationKind{StageMustMatch, StageMustDiffer, StageMayDiffer}, values[index].Expectation) {
			return errors.New("stress relation stage expectations are incomplete or out of order")
		}
	}
	return nil
}

func validateTransformStageExpectations(transform Transform, values []StageExpectation) error {
	changedIndex := stageIndex(transform.DeclaredChangedLayer)
	for index, value := range values {
		if index < changedIndex && value.Expectation != StageMustMatch {
			return errors.New("stress relation permits divergence before its declared changed layer")
		}
		if index == changedIndex && value.Expectation != StageMustDiffer {
			return errors.New("stress relation does not require divergence at its declared changed layer")
		}
	}
	return nil
}

func validSourceFormat(value preprocess.SourceFormat) bool {
	return slices.Contains([]preprocess.SourceFormat{
		preprocess.SourcePlainText, preprocess.SourceClaudeCode, preprocess.SourceCodexRollout, preprocess.SourceOpenCode,
		preprocess.SourceTerminalBench, preprocess.SourceSWEbench, preprocess.SourceOTLPJSON, preprocess.SourceAgentTrace,
	}, value)
}

func validSourceRequirement(value SourceRequirementKind) bool {
	return slices.Contains([]SourceRequirementKind{
		RequirementV3Manifest, RequirementV3ConstructFirewall, RequirementFormalWitness, RequirementExactReplay,
		RequirementOwnerAttestation, RequirementTerminalLedger, RequirementOutcomeProof, RequirementPublicFixture,
		RequirementLiveAuthorization, RequirementCapsule,
	}, value)
}

func hasSourceRequirement(values []SourceRequirement, wanted SourceRequirementKind) bool {
	return slices.ContainsFunc(values, func(value SourceRequirement) bool { return value.Kind == wanted })
}

func validMetric(value Metric) bool {
	return slices.Contains([]Metric{
		MetricDecision, MetricRank, MetricConditionalScore, MetricConditionalVariance, MetricSupportJaccard,
		MetricProbabilityOverlap, MetricCommonSupportDivergence, MetricVisibleMass, MetricValidMass, MetricUnobservedMass,
	}, value)
}

func validOperator(value Operator) bool {
	return slices.Contains([]Operator{OperatorEqual, OperatorNotEqual, OperatorLessOrEqual, OperatorGreaterOrEqual, OperatorOriginalPreferred, OperatorTransformedPreferred}, value)
}

func validInvalidState(value InvalidState) bool {
	return slices.Contains([]InvalidState{
		InvalidNotApplicable, InvalidSourceUnavailable, InvalidFormalWitness, InvalidConstructRejected, InvalidCustody,
		InvalidHumanContradicted, InvalidTransform, InvalidReplayMismatch, InvalidPrivacy, InvalidCrossVersion, InvalidLockedPartitionUsed,
	}, value)
}

func validStage(value Stage) bool { return stageIndex(value) >= 0 }

func orderedStages() []Stage {
	return []Stage{StageIngestion, StageRequestConstruction, StageProviderResponse, StageScoreExtraction, StageDecisionPolicy, StageRendering}
}

func stageIndex(value Stage) int { return slices.Index(orderedStages(), value) }

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func digestDocument(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode stress document: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func relationDigest(value Relation) (string, error) {
	value.Digest = ""
	return digestDocument(value)
}
