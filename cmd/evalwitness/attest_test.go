package main

import (
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/config"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestProductionQualificationRequestUsesCanonicalProductionPlan(t *testing.T) {
	configureNoNetworkCLI(t, "https://boundary.invalid/v1")
	t.Setenv("EVALWITNESS_CRITIQUE_THEN_SCORE", "false")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	criteria := []verifier.Criterion{verifier.BuiltinCriteria["code_review"]}
	trajectories := []string{
		"candidate one passed the targeted test",
		"candidate two left the test failing",
		"candidate three passed the targeted test",
		"candidate four left the test failing",
		"candidate five passed the targeted test",
	}
	request, err := productionQualificationRequest(
		cfg, "joint_absolute", criteria, false, "repair the production defect", trajectories,
	)
	if err != nil {
		t.Fatal(err)
	}
	if request.RequestedModel != "boundary-model" {
		t.Fatalf("requested model = %q, want boundary-model", request.RequestedModel)
	}
	if len(request.ScoreTags) != 5 || len(request.EvidenceBindings) != len(trajectories) {
		t.Fatalf("production qualification = tags:%d bindings:%d", len(request.ScoreTags), len(request.EvidenceBindings))
	}
	if request.Lineage.Entrypoint != "cli.attest" || request.Lineage.SamplingSlot != "plan" {
		t.Fatalf("production qualification lineage = %+v", request.Lineage)
	}
	if !strings.Contains(request.Messages[0].Content, "repair the production defect") ||
		!strings.Contains(request.Messages[0].Content, "candidate five passed the targeted test") {
		t.Fatal("production qualification prompt does not contain the supplied task and canonical trajectory content")
	}
	for index, binding := range request.EvidenceBindings {
		if binding.SourceDigest == "" || binding.CanonicalDigest == "" {
			t.Fatalf("trajectory %d evidence binding is incomplete: %+v", index, binding)
		}
	}
}

func TestQualificationInputsRequireTaskAndTrajectoriesTogether(t *testing.T) {
	cases := []struct {
		name         string
		task         string
		trajectories []string
	}{
		{name: "task only", task: "repair"},
		{name: "trajectory only", trajectories: []string{"candidate"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := qualificationRequestFromInputs(
				config.Config{}, "joint_absolute", nil, false, testCase.task, testCase.trajectories,
			)
			if err == nil || !strings.Contains(err.Error(), "requires --task and at least one --trajectory together") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
