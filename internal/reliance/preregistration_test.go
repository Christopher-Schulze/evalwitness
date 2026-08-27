package reliance

import (
	"reflect"
	"slices"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/study"
)

func TestFrozenPreregistrationBindsCompleteSparseDesign(t *testing.T) {
	ontology := relianceOntology(t)
	estimands, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	first, err := FrozenPreregistration(ontology, estimands)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FrozenPreregistration(ontology, estimands)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("frozen reliance preregistration changed across identical construction")
	}
	if first.StudySchemaVersion != study.ManifestSchemaVersion || first.StudyKind != string(study.KindEvidenceReliance) ||
		first.OntologyDigest != ontology.Digest || first.EstimandCatalogDigest != estimands.Digest {
		t.Fatalf("preregistration is detached from TASK 049 governance: %+v", first)
	}
	if len(first.MainEffects) != len(ontology.Factors) || len(first.Interactions) != 4 || len(first.PrimaryOutcomes) != 7 ||
		first.Multiplicity.Method != "bonferroni" || first.Multiplicity.FamilySize != 98 {
		t.Fatalf("preregistered design cardinality is incomplete: %+v", first)
	}
	factors := make(map[string]struct{}, len(first.MainEffects)+1)
	for _, factor := range first.MainEffects {
		factors[string(factor)] = struct{}{}
	}
	factors[PresentationOrderTerm] = struct{}{}
	hasOrderInteraction := false
	for _, interaction := range first.Interactions {
		if len(interaction.Terms) != 2 || interaction.Terms[0] == interaction.Terms[1] {
			t.Fatalf("interaction is not a two-term contrast: %+v", interaction)
		}
		for _, term := range interaction.Terms {
			if _, found := factors[term]; !found {
				t.Fatalf("interaction %q references unknown term %q", interaction.InteractionID, term)
			}
			if term == PresentationOrderTerm {
				hasOrderInteraction = true
			}
		}
	}
	if !hasOrderInteraction || first.Stopping.OutcomeDependent || first.Stopping.LiveFallback ||
		first.Missingness.Imputation != "none" || len(first.RetainedPostRandomization) != 10 {
		t.Fatalf("preregistration lost its order, missingness, or stopping boundary: %+v", first)
	}
	for _, outcome := range first.PrimaryOutcomes {
		if !outcome.Primary || outcome.EstimatorID == "" {
			t.Fatalf("primary outcome is incomplete: %+v", outcome)
		}
	}
	for _, exclusion := range first.PreRandomizationExclusions {
		if slices.Contains(first.RetainedPostRandomization, exclusion) {
			t.Fatalf("status %q is both excluded and retained", exclusion)
		}
	}
}

func TestFrozenPreregistrationRejectsResearcherDegreesOfFreedom(t *testing.T) {
	ontology := relianceOntology(t)
	estimands, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	value, err := FrozenPreregistration(ontology, estimands)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Preregistration)
	}{
		{name: "drop outcome", mutate: func(value *Preregistration) { value.PrimaryOutcomes = value.PrimaryOutcomes[1:] }},
		{name: "drop invalid denominator", mutate: func(value *Preregistration) { value.RetainedPostRandomization = value.RetainedPostRandomization[1:] }},
		{name: "change multiplicity", mutate: func(value *Preregistration) { value.Multiplicity.Method = "none" }},
		{name: "enable result stopping", mutate: func(value *Preregistration) { value.Stopping.OutcomeDependent = true }},
		{name: "enable live fallback", mutate: func(value *Preregistration) { value.Stopping.LiveFallback = true }},
		{name: "merge missing cells", mutate: func(value *Preregistration) { value.Missingness.Imputation = "zero" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := value
			test.mutate(&mutated)
			if err := mutated.Validate(ontology, estimands); err == nil {
				t.Fatal("preregistration accepted a post-freeze design change")
			}
		})
	}
}
