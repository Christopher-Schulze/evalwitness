package relation

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/mutation"
)

func TestDefaultPlanIsSealedAndStrictlyDecodable(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeIndented(plan)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePlan(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Digest != plan.Digest || decoded.Objective != ReviewObjectiveControlledRelation || len(decoded.Families) != 8 || len(decoded.Axes) != 7 {
		t.Fatal("relation plan lost its objective, digest, family, or axis binding")
	}
}

func TestPlanRejectsOutcomeObjectiveAndUnknownFields(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	plan.Objective = "single_trajectory_outcome"
	if err := plan.Validate(); err == nil {
		t.Fatal("relation plan accepted the outcome-review objective")
	}
	if _, err := DecodePlan(bytes.NewBufferString(`{"unknown":true}`)); err == nil {
		t.Fatal("relation plan decoder accepted an unknown field")
	}
}

func TestSchemasBindRelationVersionsAndObjectiveEnums(t *testing.T) {
	for _, document := range []string{
		"blind-packet", "case-material", "condition-probe", "condition-probe-batch", "formal-human-comparison", "judgment-batch", "mapping-reveal", "normalized-observations", "pair-judgment", "pilot-change-receipt", "pilot-inspection", "pilot-launch-dossier", "plan", "plan-v2", "pilot-readiness", "pilot-sample", "pilot-sample-v2", "prereveal-ambiguity", "primary-sample", "primary-sample-v2", "private-mapping", "relation-resolution",
		"qualification-answer-key", "qualification-report", "qualification-set", "replay-receipt", "review-assignment", "review-bundle",
		"reviewer-handbook", "reviewer-kit", "reviewer-record", "study-amendment", "study-amendment-v2", "terminal-ledger", "translation-result",
	} {
		schema, err := Schema(document)
		if err != nil {
			t.Fatal(err)
		}
		if schema["$id"] == "" || schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("relation %s schema lacks identity", document)
		}
	}
}

func TestPlanRejectsUnknownReasonCode(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	plan.ReasonCodes[len(plan.ReasonCodes)-1] = "unknown_reason"
	if _, err := SealPlan(plan); err == nil {
		t.Fatal("relation plan accepted a noncanonical reason code")
	}
}

func TestTranslationSeparatesSupportContradictionAndUnresolved(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		observations []AxisObservation
		want         TranslationState
	}{
		{name: "support", observations: []AxisObservation{{AxisEvidenceStrength, NormalizedOriginal}, {AxisInformation, NormalizedSufficient}, {AxisSemanticQuality, NormalizedEqual}}, want: TranslationSupports},
		{name: "contradiction", observations: []AxisObservation{{AxisEvidenceStrength, NormalizedTransformed}, {AxisInformation, NormalizedSufficient}, {AxisSemanticQuality, NormalizedEqual}}, want: TranslationContradicts},
		{name: "unresolved", observations: []AxisObservation{{AxisEvidenceStrength, NormalizedOriginal}, {AxisInformation, NormalizedInsufficient}, {AxisSemanticQuality, NormalizedEqual}}, want: TranslationUnresolved},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Translate(plan, mutation.FamilyTestEvidenceOmitted, test.observations)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != test.want || result.Objective != ReviewObjectiveControlledRelation || result.Digest == "" {
				t.Fatalf("translation state = %q, want %q", result.State, test.want)
			}
		})
	}
}

func TestTranslationRejectsMissingOrOutcomeShapedObservations(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Translate(plan, mutation.FamilyTestEvidenceOmitted, []AxisObservation{{AxisInformation, NormalizedSufficient}}); err == nil {
		t.Fatal("translation accepted incomplete axis coverage")
	}
	if _, err := Translate(plan, mutation.FamilyTestEvidenceOmitted, []AxisObservation{{AxisEvidenceStrength, "solved"}, {AxisInformation, NormalizedSufficient}, {AxisSemanticQuality, NormalizedEqual}}); err == nil {
		t.Fatal("translation accepted an outcome-style rating")
	}
}

func TestStudyAmendmentLocksResolutionAndNoEmpiricalClaim(t *testing.T) {
	plan, err := DefaultPlan()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join("..", "..", "eval", "governance", "relation-primary-sample-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close primary sample: %v", err)
		}
	}()
	sample, err := DecodePrimarySample(file)
	if err != nil {
		t.Fatal(err)
	}
	pilotFile, err := os.Open(filepath.Join("..", "..", "eval", "governance", "relation-pilot-sample-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pilotFile.Close(); err != nil {
			t.Errorf("close pilot sample: %v", err)
		}
	}()
	pilot, err := DecodePilotSample(pilotFile)
	if err != nil {
		t.Fatal(err)
	}
	amendment, err := BuildStudyAmendment(plan, pilot, sample, "2026-08-09T17:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if amendment.EmpiricalStatus != "not_run" || amendment.Primary.EffectiveTaskGroups != 28 || amendment.Inference.ZeroContradictionUpperBound <= 0.10 || amendment.Inference.ZeroContradictionUpperBound >= 0.11 {
		t.Fatalf("unexpected relation design resolution: %#v", amendment.Inference)
	}
	amendment.EmpiricalStatus = "complete"
	if err := amendment.Validate(); err == nil {
		t.Fatal("relation study amendment accepted a fabricated empirical result")
	}
}
