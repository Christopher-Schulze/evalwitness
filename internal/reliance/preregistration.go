package reliance

import (
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const PresentationOrderTerm = "presentation_order"

func FrozenPreregistration(ontology FactorOntology, estimands EstimandCatalog) (Preregistration, error) {
	if err := ontology.Validate(); err != nil {
		return Preregistration{}, err
	}
	if err := estimands.Validate(); err != nil {
		return Preregistration{}, err
	}
	value := canonicalPreregistration(ontology.Digest, estimands.Digest)
	digest, err := preregistrationDigest(value)
	if err != nil {
		return Preregistration{}, err
	}
	value.Digest = digest
	if err := value.Validate(ontology, estimands); err != nil {
		return Preregistration{}, err
	}
	return value, nil
}

func (value Preregistration) Validate(ontology FactorOntology, estimands EstimandCatalog) error {
	if err := ontology.Validate(); err != nil {
		return err
	}
	if err := estimands.Validate(); err != nil {
		return err
	}
	expected := canonicalPreregistration(ontology.Digest, estimands.Digest)
	if value.SchemaVersion != PreregistrationSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!reflect.DeepEqual(preregistrationWithoutDigest(value), expected) {
		return errors.New("reliance preregistration differs from the frozen v1 design")
	}
	digest, err := preregistrationDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance preregistration digest is invalid")
	}
	return nil
}

func canonicalPreregistration(ontologyDigest, estimandDigest string) Preregistration {
	mainEffects := []FactorID{
		FactorCommandExit, FactorErrorOutput, FactorExecutableOutcome, FactorIrrelevantVerbosity, FactorMetadata,
		FactorPatchEdit, FactorPromptInjection, FactorSuccessFailureProse, FactorTestResult, FactorToolOutput,
	}
	slices.Sort(mainEffects)
	interactions := []InteractionDefinition{
		{InteractionID: "error_output_x_prompt_injection", Terms: sortedStrings(string(FactorErrorOutput), string(FactorPromptInjection))},
		{InteractionID: "executable_outcome_x_success_failure_prose", Terms: sortedStrings(string(FactorExecutableOutcome), string(FactorSuccessFailureProse))},
		{InteractionID: "success_failure_prose_x_presentation_order", Terms: sortedStrings(string(FactorSuccessFailureProse), PresentationOrderTerm)},
		{InteractionID: "test_result_x_success_failure_prose", Terms: sortedStrings(string(FactorTestResult), string(FactorSuccessFailureProse))},
	}
	slices.SortFunc(interactions, func(left, right InteractionDefinition) int {
		return strings.Compare(left.InteractionID, right.InteractionID)
	})
	outcomes := []OutcomeDefinition{
		{OutcomeID: OutcomeAbstentionTransition, Scale: ScaleBinaryTransition, EstimatorID: "paired_task_transition_risk_difference.v1", Primary: true},
		{OutcomeID: OutcomeConditionalMean, Scale: ScaleContinuous, EstimatorID: "source_task_cluster_difference.v1", Primary: true},
		{OutcomeID: OutcomeConditionalVariance, Scale: ScaleContinuous, EstimatorID: "source_task_cluster_difference.v1", Primary: true},
		{OutcomeID: OutcomeDecisionFlip, Scale: ScaleBinaryTransition, EstimatorID: "paired_task_transition_risk_difference.v1", Primary: true},
		{OutcomeID: OutcomeSupportJaccard, Scale: ScaleContinuous, EstimatorID: "source_task_cluster_difference.v1", Primary: true},
		{OutcomeID: OutcomeValidMass, Scale: ScaleContinuous, EstimatorID: "source_task_cluster_difference.v1", Primary: true},
		{OutcomeID: OutcomeVisibleMass, Scale: ScaleContinuous, EstimatorID: "source_task_cluster_difference.v1", Primary: true},
	}
	slices.SortFunc(outcomes, func(left, right OutcomeDefinition) int {
		return strings.Compare(string(left.OutcomeID), string(right.OutcomeID))
	})
	return Preregistration{
		SchemaVersion: PreregistrationSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		StudySchemaVersion: study.ManifestSchemaVersion, StudyKind: string(study.KindEvidenceReliance),
		OntologyDigest: ontologyDigest, EstimandCatalogDigest: estimandDigest,
		MainEffects: mainEffects, Interactions: interactions, PrimaryOutcomes: outcomes,
		Randomization: RandomizationPlan{
			Algorithm: "evalwitness.balanced-within-source-task.v1", Unit: "source_task",
			Balance: "factor_levels_and_presentation_order_within_source_task", SeedSource: "locked_study_manifest",
			AssignmentTiming: "before_verifier_output", Blocking: sortedStrings("entrypoint", "route", "source_task"),
		},
		PreRandomizationExclusions: sortedStrings(
			"canonical_trajectory_invalid",
			"factor_assignment_contract_invalid",
			"source_task_not_permitted_for_data_role",
			"task_identity_unavailable",
		),
		RetainedPostRandomization: sortedStrings(
			"abstained",
			"budget_exhausted",
			"incomplete_pair",
			"intervention_invalid",
			"missing_score",
			"outcome_ambiguous",
			"provider_failed",
			"relation_unresolved",
			"route_failed",
			"unsupported_cell",
		),
		Multiplicity: MultiplicityPlan{
			Method: "bonferroni", FamilyConstruction: "cartesian_product_of_all_main_effects_interactions_and_primary_outcomes",
			FamilySize: (len(mainEffects) + len(interactions)) * len(outcomes),
		},
		Missingness: MissingnessPlan{
			DenominatorPolicy: "retain_every_randomized_cell_by_admissibility_and_execution_status",
			Imputation:        "none", InvalidCaseTreatment: "retain_as_invalid_without_effect_estimate",
			IncompletePairTreatment:  "retain_as_incomplete_without_pair_contrast",
			UnsupportedCellTreatment: "retain_as_unsupported_without_imputation",
		},
		Stopping: StoppingPlan{
			Method: "fixed_panel_and_hard_resource_limits", SequentialLooks: 1,
			OutcomeDependent: false, LiveFallback: false, AdaptiveScreeningDataRole: "development_only",
		},
	}
}

func preregistrationWithoutDigest(value Preregistration) Preregistration {
	value.Digest = ""
	return value
}

func preregistrationDigest(value Preregistration) (string, error) {
	return protocolkit.Digest(preregistrationWithoutDigest(value))
}
