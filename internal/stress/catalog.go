package stress

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	relationevidence "github.com/Christopher-Schulze/evalwitness/internal/relation"
)

const (
	catalogRepetitions            = 3
	catalogScoreEqualityTolerance = 0.05
	catalogMinimumQualityEffect   = 0.05
)

func BuildV3RelationRegistry(plan mutation.CorpusDevelopmentPlan, audit mutation.CorpusDevelopmentAuditV3, release mutation.CorpusReleaseV3) (RelationRegistry, error) {
	if err := release.Validate(plan, audit); err != nil {
		return RelationRegistry{}, err
	}
	formats := releaseFormatsByFamily(release)
	relations := make([]Relation, 0, len(release.Policy.InferentialCoreFamilies)*2+1)
	for _, family := range release.Policy.InferentialCoreFamilies {
		primary, err := sealV3CatalogRelation(family, EstimandPrimaryCore, formats[family])
		if err != nil {
			return RelationRegistry{}, err
		}
		sensitivity, err := sealV3CatalogRelation(family, EstimandSensitivity, formats[family])
		if err != nil {
			return RelationRegistry{}, err
		}
		relations = append(relations, primary, sensitivity)
	}
	sentinel, err := sealV3CatalogRelation(release.Policy.ScarcitySentinelFamily, EstimandScarcitySentinel, formats[release.Policy.ScarcitySentinelFamily])
	if err != nil {
		return RelationRegistry{}, err
	}
	return SealRelationRegistry(plan, audit, release, append(relations, sentinel))
}

func sealV3CatalogRelation(family mutation.Family, estimand Estimand, formats []preprocess.SourceFormat) (Relation, error) {
	draft, err := v3CatalogRelationDraft(family, estimand, formats)
	if err != nil {
		return Relation{}, err
	}
	return SealRelation(draft)
}

func v3CatalogRelationDraft(family mutation.Family, estimand Estimand, formats []preprocess.SourceFormat) (Relation, error) {
	definition, exists := mutation.DefinitionFor(family)
	if !exists || definition.Relation == mutation.RelationAmbiguous {
		return Relation{}, errors.New("v3 stress catalog requires one released non-ambiguous mutation family")
	}
	if family == mutation.FamilyTestEvidenceOmitted && estimand != EstimandScarcitySentinel ||
		family != mutation.FamilyTestEvidenceOmitted && estimand != EstimandPrimaryCore && estimand != EstimandSensitivity {
		return Relation{}, errors.New("v3 stress catalog family and estimand are incompatible")
	}
	unit, minimum, maximum := UnitTrajectory, 1, 1
	if definition.PairLevel {
		unit, minimum, maximum = UnitCandidatePair, 2, 2
	}
	kind := KindSensitivity
	if definition.Relation == mutation.RelationQualityEqual || definition.Relation == mutation.RelationNoControlEffect {
		kind = KindInvariance
	}
	requirements := []SourceRequirement{
		{Kind: RequirementV3Manifest, Value: mutation.ManifestSchemaVersion},
		{Kind: RequirementV3ConstructFirewall, Value: mutation.ConstructFirewallSchemaVersionV2},
		{Kind: RequirementFormalWitness, Value: mutation.WitnessSchemaVersion},
		{Kind: RequirementExactReplay, Value: "required"},
		{Kind: RequirementOwnerAttestation, Value: relationevidence.OwnerInspectionPublicAttestationSchemaVersion},
	}
	if estimand == EstimandPrimaryCore {
		requirements = append(requirements, SourceRequirement{Kind: RequirementTerminalLedger, Value: relationevidence.TerminalRelationLedgerSchemaVersionV3})
	}
	multiplicity := "bonferroni"
	denominator := DenominatorSensitivityStratified
	if estimand == EstimandPrimaryCore {
		denominator = DenominatorPrimaryHumanSupported
	}
	if estimand == EstimandScarcitySentinel {
		multiplicity = "none_descriptive"
		denominator = DenominatorScarcityAvailability
	}
	constraint, err := catalogConstraint(definition)
	if err != nil {
		return Relation{}, err
	}
	return Relation{
		ID: catalogRelationID(family, estimand), Revision: 1, Kind: kind,
		Applicability: Applicability{
			Unit: unit, MinimumTrajectories: minimum, MaximumTrajectories: maximum,
			RequiredSourceFormats: append([]preprocess.SourceFormat(nil), formats...), Requirements: requirements,
		},
		Transform: Transform{
			Kind: TransformMutation, Identifier: definition.Operator, Version: mutation.MutationProgramVersionV3,
			MutationFamily: family, InterventionClass: definition.Class, ExpectedFormalRelation: definition.Relation,
			DeclaredChangedLayer: StageIngestion,
		},
		Constraints:   []ExpectedConstraint{constraint},
		InvalidStates: catalogInvalidStates(),
		Repeat: RepeatPolicy{
			Kind: RepeatFixed, MinimumRepetitions: catalogRepetitions, MaximumRepetitions: catalogRepetitions,
			StopRule: "fixed_repetitions",
		},
		StatisticalFamily: StatisticalFamily{
			ID: catalogRelationID(family, estimand), Estimand: estimand, ClusterUnit: "source_task", MultiplicityMethod: multiplicity,
			DenominatorPolicy: denominator, FailurePolicy: canonicalFailurePolicy(),
		},
		StageExpectations: catalogStageExpectations(),
	}, nil
}

func canonicalFailurePolicy() FailurePolicy {
	return FailurePolicy{
		Invalid: OutcomeInvalid, MissingScore: OutcomeAbstained, ProviderFailure: OutcomeProviderFailed,
		RouteFailure: OutcomeProviderFailed, Timeout: OutcomeProviderFailed, Abstention: OutcomeAbstained,
		BudgetExhaustion: OutcomeInconclusive, RetryExhaustion: OutcomeProviderFailed, IncompleteCell: OutcomeInconclusive,
		Unsupported: OutcomeUnsupported, DenominatorPolicy: FailureDenominatorTreatment,
	}
}

func validateV3CatalogRelation(value Relation) error {
	expected, err := sealV3CatalogRelation(value.Transform.MutationFamily, value.StatisticalFamily.Estimand, value.Applicability.RequiredSourceFormats)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("relation fields, metric, repeat policy, evidence requirements, or stage expectations are not canonical")
	}
	return nil
}

func catalogRelationID(family mutation.Family, estimand Estimand) string {
	return "v3-" + string(estimand) + "-" + string(family)
}

func catalogConstraint(definition mutation.Definition) (ExpectedConstraint, error) {
	if definition.PairLevel && definition.Relation == mutation.RelationQualityEqual {
		return ExpectedConstraint{ID: "selected-trajectory-stability", Metric: MetricDecision, Operator: OperatorEqual, Required: true}, nil
	}
	switch definition.Relation {
	case mutation.RelationQualityEqual, mutation.RelationNoControlEffect:
		return ExpectedConstraint{
			ID: "conditional-score-stability", Metric: MetricConditionalScore, Operator: OperatorEqual,
			AbsoluteTolerance: catalogScoreEqualityTolerance, Required: true,
		}, nil
	case mutation.RelationOriginalBetter, mutation.RelationVerifiedOutcomeWins:
		return ExpectedConstraint{
			ID: "original-conditional-score-advantage", Metric: MetricConditionalScore, Operator: OperatorLessOrEqual,
			MinimumEffect: catalogMinimumQualityEffect, Required: true,
		}, nil
	case mutation.RelationQualityEqualEvidenceLow:
		return ExpectedConstraint{
			ID: "conditional-score-nonincreasing", Metric: MetricConditionalScore, Operator: OperatorLessOrEqual,
			AbsoluteTolerance: catalogScoreEqualityTolerance, Required: true,
		}, nil
	default:
		return ExpectedConstraint{}, fmt.Errorf("v3 stress catalog has no metric contract for relation %q", definition.Relation)
	}
}

func catalogInvalidStates() []InvalidState {
	return []InvalidState{
		InvalidNotApplicable, InvalidSourceUnavailable, InvalidFormalWitness, InvalidConstructRejected, InvalidCustody,
		InvalidHumanContradicted, InvalidTransform, InvalidReplayMismatch, InvalidPrivacy, InvalidCrossVersion, InvalidLockedPartitionUsed,
	}
}

func catalogStageExpectations() []StageExpectation {
	return []StageExpectation{
		{Stage: StageIngestion, Expectation: StageMustDiffer},
		{Stage: StageRequestConstruction, Expectation: StageMayDiffer},
		{Stage: StageProviderResponse, Expectation: StageMayDiffer},
		{Stage: StageScoreExtraction, Expectation: StageMayDiffer},
		{Stage: StageDecisionPolicy, Expectation: StageMayDiffer},
		{Stage: StageRendering, Expectation: StageMayDiffer},
	}
}
