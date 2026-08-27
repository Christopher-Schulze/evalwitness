package mutation

import "slices"

type Definition struct {
	Family            Family
	Class             InterventionClass
	Relation          Relation
	Operator          string
	EvidenceOnly      bool
	RequiresGoldProof bool
	PairLevel         bool
}

var definitions = []Definition{
	{Family: FamilyPatchHunkRemoval, Class: ClassSemanticQuality, Relation: RelationOriginalBetter, Operator: "remove_required_patch_fragment", RequiresGoldProof: true},
	{Family: FamilyFailingChangeReintroduced, Class: ClassSemanticQuality, Relation: RelationOriginalBetter, Operator: "replace_patch_with_known_failure", RequiresGoldProof: true},
	{Family: FamilyTestEvidenceOmitted, Class: ClassEvidenceAvailability, Relation: RelationQualityEqualEvidenceLow, Operator: "remove_test_evidence", EvidenceOnly: true},
	{Family: FamilyTestEvidenceFalsified, Class: ClassAdversarialClaim, Relation: RelationVerifiedOutcomeWins, Operator: "falsify_test_evidence"},
	{Family: FamilyCommandFailureHidden, Class: ClassAdversarialClaim, Relation: RelationVerifiedOutcomeWins, Operator: "hide_command_failure"},
	{Family: FamilyToolOutputIncomplete, Class: ClassEvidenceAvailability, Relation: RelationQualityEqualEvidenceLow, Operator: "truncate_tool_output", EvidenceOnly: true},
	{Family: FamilyIrrelevantVerbosity, Class: ClassPresentation, Relation: RelationQualityEqual, Operator: "append_irrelevant_verbosity"},
	{Family: FamilyNeutralFormatting, Class: ClassPresentation, Relation: RelationQualityEqual, Operator: "normalize_neutral_formatting"},
	{Family: FamilyStablePathAlias, Class: ClassPresentation, Relation: RelationQualityEqual, Operator: "replace_stable_path_alias"},
	{Family: FamilyCandidateOrderReversal, Class: ClassPresentation, Relation: RelationQualityEqual, Operator: "reverse_candidate_order", PairLevel: true},
	{Family: FamilyCausalIndependentReorder, Class: ClassPresentation, Relation: RelationQualityEqual, Operator: "swap_causally_independent_events"},
	{Family: FamilyUntrustedScoreInjection, Class: ClassAdversarialClaim, Relation: RelationNoControlEffect, Operator: "inject_untrusted_score_tag"},
	{Family: FamilyAmbiguousSemanticEdit, Class: ClassSemanticQuality, Relation: RelationAmbiguous, Operator: "apply_unresolved_semantic_edit"},
}

func Definitions() []Definition {
	return append([]Definition(nil), definitions...)
}

func DefinitionFor(family Family) (Definition, bool) {
	index := slices.IndexFunc(definitions, func(definition Definition) bool {
		return definition.Family == family
	})
	if index < 0 {
		return Definition{}, false
	}
	return definitions[index], true
}
