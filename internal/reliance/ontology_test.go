package reliance

import (
	"reflect"
	"slices"
	"testing"
)

func TestFrozenOntologyIsCompleteDeterministicAndClosed(t *testing.T) {
	first, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	second, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("frozen reliance ontology changed across identical construction")
	}
	wantFactors := []FactorID{
		FactorCommandExit, FactorErrorOutput, FactorExecutableOutcome, FactorIrrelevantVerbosity, FactorMetadata,
		FactorPatchEdit, FactorPromptInjection, FactorSuccessFailureProse, FactorTestResult, FactorToolOutput,
	}
	gotFactors := make([]FactorID, len(first.Factors))
	for index, factor := range first.Factors {
		gotFactors[index] = factor.FactorID
		if len(factor.AllowedTargets) == 0 {
			t.Fatalf("factor %q has no typed evidence targets", factor.FactorID)
		}
	}
	slices.Sort(wantFactors)
	if !slices.Equal(gotFactors, wantFactors) {
		t.Fatalf("frozen factor IDs = %v, want %v", gotFactors, wantFactors)
	}
	if len(first.Operators) != 4 || len(first.AmbiguityRules) != 9 || len(first.IdentificationAssumptions) != 10 {
		t.Fatalf("frozen ontology cardinality is incomplete: %+v", first)
	}
	mutated := first
	mutated.Factors = append([]EvidenceFactor(nil), first.Factors...)
	mutated.Factors[0].Description = "changed after freeze"
	if err := mutated.Validate(); err == nil {
		t.Fatal("ontology accepted a post-freeze factor change")
	}
}

func TestFrozenOperatorsDeclareExactFieldBoundaries(t *testing.T) {
	ontology, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	all := allCanonicalTargets()
	owned := make(map[FieldTarget]struct{}, len(all))
	for _, factor := range ontology.Factors {
		for _, target := range factor.AllowedTargets {
			if !slices.Contains(all, target) {
				t.Fatalf("factor %q exposes target outside the operator catalog: %+v", factor.FactorID, target)
			}
			owned[target] = struct{}{}
		}
	}
	for _, target := range all {
		if _, found := owned[target]; !found {
			t.Fatalf("operator catalog exposes unowned evidence field %+v", target)
		}
	}
	for _, definition := range ontology.Operators {
		switch definition.Operator {
		case OperatorControlledReplacement:
			if !definition.ChangesEvidence || !definition.RequiresReplacement || !slices.Equal(definition.AllowedTargets, all) {
				t.Fatalf("controlled replacement contract = %+v", definition)
			}
		case OperatorRemove:
			for _, target := range definition.AllowedTargets {
				if !target.Optional {
					t.Fatalf("remove operator admits required target %+v", target)
				}
			}
		case OperatorRetain:
			if definition.ChangesEvidence || definition.RequiresReplacement || !slices.Equal(definition.AllowedTargets, all) {
				t.Fatalf("retain contract = %+v", definition)
			}
		case OperatorTypedMask:
			for _, target := range definition.AllowedTargets {
				if !slices.Contains(maskableCanonicalTargets(all), target) {
					t.Fatalf("typed mask admits non-maskable target %+v", target)
				}
			}
		default:
			t.Fatalf("unexpected operator %q", definition.Operator)
		}
	}
}
