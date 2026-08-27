package reliance

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

func FrozenEstimands() (EstimandCatalog, error) {
	value := EstimandCatalog{
		SchemaVersion: EstimandCatalogSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		Definitions: canonicalEstimands(),
	}
	digest, err := estimandCatalogDigest(value)
	if err != nil {
		return EstimandCatalog{}, err
	}
	value.Digest = digest
	if err := value.Validate(); err != nil {
		return EstimandCatalog{}, err
	}
	return value, nil
}

func (value EstimandCatalog) Validate() error {
	if value.SchemaVersion != EstimandCatalogSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		!reflect.DeepEqual(value.Definitions, canonicalEstimands()) {
		return errors.New("reliance estimand catalog differs from the frozen v1 contract")
	}
	digest, err := estimandCatalogDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance estimand catalog digest is invalid")
	}
	return nil
}

func (value EstimandCatalog) Definition(family EstimandFamily) (EstimandDefinition, bool) {
	for _, definition := range value.Definitions {
		if definition.Family == family {
			return cloneEstimandDefinition(definition), true
		}
	}
	return EstimandDefinition{}, false
}

func (value EstimandCatalog) ValidateClaims(family EstimandFamily, claims []string) error {
	if err := value.Validate(); err != nil {
		return err
	}
	definition, found := value.Definition(family)
	if !found {
		return fmt.Errorf("unknown reliance estimand family %q", family)
	}
	if len(claims) == 0 {
		return errors.New("reliance result must declare at least one allowed claim")
	}
	seen := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		if claim == "" || claim != strings.TrimSpace(claim) {
			return errors.New("reliance claim identity is empty or untrimmed")
		}
		if _, duplicate := seen[claim]; duplicate {
			return fmt.Errorf("reliance claim %q is duplicated", claim)
		}
		seen[claim] = struct{}{}
		if slices.Contains(definition.ForbiddenClaims, claim) {
			return fmt.Errorf("reliance claim %q is forbidden for %s", claim, family)
		}
		if !slices.Contains(definition.AllowedClaims, claim) {
			return fmt.Errorf("reliance claim %q is not registered for %s", claim, family)
		}
	}
	return nil
}

func canonicalEstimands() []EstimandDefinition {
	definitions := []EstimandDefinition{
		{
			Family: EstimandEvidenceOnly, Interpretation: "verifier_output_reliance",
			PrimaryUnit: "source_task", QualityCondition: QualityPreserved,
			RequiredEvidence: sortedStrings(
				"assignment_commitment",
				"canonical_event_lineage",
				"exact_replay_or_authorized_live_record",
				"executable_outcome_preservation",
				"relation_admission_when_applicable",
				"task_identity_preservation",
			),
			AllowedClaims: sortedStrings(
				"decision_or_abstention_change_under_admissible_intervention",
				"randomized_intervention_effect_on_verifier_output",
				"verifier_output_reliance_under_frozen_contract",
				"within_task_score_distribution_change_under_admissible_intervention",
			),
			ForbiddenClaims: sortedStrings(
				"agent_failure_responsibility",
				"agent_step_causality",
				"environment_outcome_causality",
				"human_reasoning_equivalence",
				"model_internal_explanation",
				"quality_changing_effect_pooled_with_reliance",
				"universal_faithfulness",
			),
			DenominatorPolicy: "all_randomized_source_task_cells_retained_by_admissibility_and_execution_status",
			ResultTableID:     "evidence_only_reliance",
		},
		{
			Family: EstimandQualityChanging, Interpretation: "verifier_output_response_to_quality_change",
			PrimaryUnit: "source_task", QualityCondition: QualityChanged,
			RequiredEvidence: sortedStrings(
				"assignment_commitment",
				"canonical_event_lineage",
				"exact_replay_or_authorized_live_record",
				"quality_change_admission",
				"relation_admission",
				"task_identity_preservation",
			),
			AllowedClaims: sortedStrings(
				"verifier_output_response_to_admitted_quality_change",
				"within_task_score_distribution_change_under_admitted_quality_change",
			),
			ForbiddenClaims: sortedStrings(
				"agent_failure_responsibility",
				"agent_step_causality",
				"environment_outcome_causality",
				"evidence_only_reliance",
				"human_reasoning_equivalence",
				"model_internal_explanation",
				"quality_changing_effect_pooled_with_reliance",
				"universal_faithfulness",
				"verifier_output_reliance_under_frozen_contract",
			),
			DenominatorPolicy: "all_randomized_quality_change_source_task_cells_retained_by_admission_and_execution_status",
			ResultTableID:     "quality_changing_sensitivity",
		},
	}
	slices.SortFunc(definitions, func(left, right EstimandDefinition) int {
		return strings.Compare(string(left.Family), string(right.Family))
	})
	return definitions
}

func cloneEstimandDefinition(value EstimandDefinition) EstimandDefinition {
	value.RequiredEvidence = slices.Clone(value.RequiredEvidence)
	value.AllowedClaims = slices.Clone(value.AllowedClaims)
	value.ForbiddenClaims = slices.Clone(value.ForbiddenClaims)
	return value
}

func sortedStrings(values ...string) []string {
	result := slices.Clone(values)
	slices.Sort(result)
	return result
}

func estimandCatalogDigest(value EstimandCatalog) (string, error) {
	value.Digest = ""
	return protocolkit.Digest(value)
}
