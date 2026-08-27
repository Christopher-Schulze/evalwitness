package reliance

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestFrozenEstimandsSeparateRelianceFromQualityChange(t *testing.T) {
	first, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	second, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || len(first.Definitions) != 2 {
		t.Fatalf("frozen estimand catalog is not deterministic and complete: %+v", first)
	}
	evidenceOnly, found := first.Definition(EstimandEvidenceOnly)
	if !found {
		t.Fatal("evidence-only estimand is absent")
	}
	qualityChanging, found := first.Definition(EstimandQualityChanging)
	if !found {
		t.Fatal("quality-changing estimand is absent")
	}
	if evidenceOnly.QualityCondition != QualityPreserved || qualityChanging.QualityCondition != QualityChanged ||
		evidenceOnly.Interpretation == qualityChanging.Interpretation ||
		evidenceOnly.DenominatorPolicy == qualityChanging.DenominatorPolicy ||
		evidenceOnly.ResultTableID == qualityChanging.ResultTableID {
		t.Fatalf("estimand families are not structurally separate:\nevidence=%+v\nquality=%+v", evidenceOnly, qualityChanging)
	}
	for _, definition := range first.Definitions {
		for _, claim := range definition.AllowedClaims {
			if slices.Contains(definition.ForbiddenClaims, claim) {
				t.Fatalf("estimand %q both allows and forbids %q", definition.Family, claim)
			}
		}
	}
	mutated := first
	mutated.Definitions = append([]EstimandDefinition(nil), first.Definitions...)
	mutated.Definitions[0].ResultTableID = mutated.Definitions[1].ResultTableID
	if err := mutated.Validate(); err == nil {
		t.Fatal("estimand catalog accepted a merged result table")
	}
}

func TestEstimandClaimBoundaryRejectsOverclaiming(t *testing.T) {
	catalog, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		family  EstimandFamily
		claims  []string
		wantErr string
	}{
		{
			name: "evidence-only allowed", family: EstimandEvidenceOnly,
			claims: []string{"verifier_output_reliance_under_frozen_contract"},
		},
		{
			name: "quality-changing allowed", family: EstimandQualityChanging,
			claims: []string{"verifier_output_response_to_admitted_quality_change"},
		},
		{
			name: "agent causality forbidden", family: EstimandEvidenceOnly,
			claims: []string{"agent_step_causality"}, wantErr: "is forbidden",
		},
		{
			name: "quality cannot claim reliance", family: EstimandQualityChanging,
			claims: []string{"verifier_output_reliance_under_frozen_contract"}, wantErr: "is forbidden",
		},
		{
			name: "unregistered claim", family: EstimandEvidenceOnly,
			claims: []string{"sounds_plausible"}, wantErr: "is not registered",
		},
		{
			name: "unknown family", family: "future_estimand",
			claims: []string{"verifier_output_reliance_under_frozen_contract"}, wantErr: "unknown reliance estimand family",
		},
		{
			name: "duplicate claim", family: EstimandEvidenceOnly,
			claims: []string{"verifier_output_reliance_under_frozen_contract", "verifier_output_reliance_under_frozen_contract"}, wantErr: "is duplicated",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := catalog.ValidateClaims(test.family, test.claims)
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("claim error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}
