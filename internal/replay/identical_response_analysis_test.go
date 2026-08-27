package replay

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestIdenticalResponseDecisionUsesLockedTieMargin(t *testing.T) {
	tests := []struct {
		name      string
		values    [][]float64
		wantState string
		wantBest  int
	}{
		{name: "selected", values: [][]float64{{0.90}, {0.87}, {0.20}}, wantState: DecisionSelected, wantBest: 0},
		{name: "margin tie", values: [][]float64{{0.90}, {0.88}, {0.20}}, wantState: DecisionTied, wantBest: -1},
		{name: "exact tie", values: [][]float64{{0.90}, {0.90}, {0.20}}, wantState: DecisionTied, wantBest: -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := identicalResponseDecision(test.values, 0.02)
			if err != nil {
				t.Fatal(err)
			}
			if decision.State != test.wantState {
				t.Fatalf("state = %q, want %q", decision.State, test.wantState)
			}
			if test.wantBest < 0 && decision.SelectedIndex != nil {
				t.Fatalf("tied decision selected index %d", *decision.SelectedIndex)
			}
			if test.wantBest >= 0 && (decision.SelectedIndex == nil || *decision.SelectedIndex != test.wantBest) {
				t.Fatalf("selected index = %v, want %d", decision.SelectedIndex, test.wantBest)
			}
		})
	}
}

func TestChosenScoreRequiresOneCanonicalChosenAlternative(t *testing.T) {
	value := 0.75
	evidence := verifier.ScoreEvidence{Alternatives: []verifier.VisibleAlternative{{Chosen: true, CanonicalValue: &value}}}
	got, err := chosenScore(evidence)
	if err != nil || got != value {
		t.Fatalf("chosen score = %v, %v; want %v", got, err, value)
	}
	evidence.Alternatives = append(evidence.Alternatives, verifier.VisibleAlternative{Chosen: true, CanonicalValue: &value})
	if _, err := chosenScore(evidence); err == nil {
		t.Fatal("duplicate chosen alternatives were accepted")
	}
}

func TestExactBinomialUpperBoundMatchesZeroEventClosedForm(t *testing.T) {
	got := exactBinomialUpperBound(0, 60, 0.05)
	want := 1 - math.Pow(0.05, 1.0/60.0)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("upper bound = %.15f, want %.15f", got, want)
	}
	if exactBinomialUpperBound(60, 60, 0.05) != 1 {
		t.Fatal("all-event upper bound must equal one")
	}
}

func TestOutcomeSensitivityExcludesTiesAndCountsDiscordance(t *testing.T) {
	distribution := 0
	chosen := 1
	row := IdenticalResponseRow{
		Status:            "disagreement",
		DistributionAware: IdenticalResponseDecision{State: DecisionSelected, SelectedIndex: &distribution},
		ChosenToken:       IdenticalResponseDecision{State: DecisionSelected, SelectedIndex: &chosen},
	}
	var summary IdenticalResponseOutcomeSummary
	applyIdenticalResponseOutcome(&row, []int{1, 0}, &summary)
	if !row.OutcomeSensitivity.Available || summary.Covered != 1 || summary.DistributionOnlySuccess != 1 {
		t.Fatalf("outcome sensitivity = %#v, summary = %#v", row.OutcomeSensitivity, summary)
	}
	row.ChosenToken.SelectedIndex = nil
	applyIdenticalResponseOutcome(&row, []int{1, 0}, &summary)
	if summary.Covered != 1 {
		t.Fatal("tied decision changed outcome coverage")
	}
}

func TestAnalyzeIdenticalResponseDerivesBothArmsFromOneRecord(t *testing.T) {
	recordFile, err := os.Open("../../eval/governance/identical-response-study-record-v5.json")
	if err != nil {
		t.Fatal(err)
	}
	base, err := study.DecodeRecord(recordFile)
	closeErr := recordFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	assignment := base.Study.Manifest.Data.Split.Assignments[0]
	taskID := assignment.TaskIDs[0]
	tags := []string{"<score_0_code_review>", "<score_1_code_review>"}
	request, err := provider.NewRequestEnvelope(provider.RequestOptions{
		ProviderID: "mock", BaseURL: "https://mock.example/v1/chat/completions", RequestedModel: "m1",
		Messages: []provider.Message{{Role: "user", Content: "fixed joint prompt"}}, MaxOutputTokens: 128,
		Logprobs: true, TopLogprobs: 20, ScoreTags: tags, ResponseFormat: provider.ResponseFormatText,
		Lineage: provider.RequestLineage{
			CriterionID: "joint_absolute:code_review", SamplingSlot: "joint_absolute:0", Entrypoint: "eval-terminal",
			AuditCaseID: assignment.GroupID, StudyCellID: "distribution-aware-vs-chosen-token",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rawText := "<score_0_code_review>A</score_0_code_review>\n<score_1_code_review>A</score_1_code_review>"
	distributions := map[string]map[string]float64{
		tags[0]: {"A": 0.51, "T": 0.49},
		tags[1]: {"A": 0.90, "B": 0.10},
	}
	response, err := provider.FinalizeResponse(request, provider.ResponseRecord{
		ServedModel: "m1", NormalizedBody: []byte(`{}`), RawText: rawText, HasLogprobs: true,
		ObservedTopLogprobs: 20, OrderedTokenEvidence: replayTokenEvidence(tags, rawText, distributions),
		Distributions: distributions,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := newFixtureEntry(request, response)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "capture.jsonl")
	if err := os.WriteFile(capture, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := base.Study.Manifest
	manifest.Arms[0].RouteID = request.RouteID()
	manifest.Arms[0].ProviderID = request.ProviderID
	manifest.Arms[0].RequestedModel = request.RequestedModel
	manifest.Arms[0].SelectionMode = "joint_absolute"
	manifest.Arms[0].Candidates = 2
	manifest.Arms[0].Repetitions = 1
	manifest.Execution.DeclaredRouteIDs = []string{request.RouteID()}
	locked, err := study.Lock(manifest, "test")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := AnalyzeIdenticalResponse(capture, locked, map[string][]int{taskID: {1, 0}}, 0.02)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Summary.TaskGroups != 1 || analysis.Summary.Disagreements != 1 || analysis.Rows[0].ResponseBodyDigest != response.ResponseBodyDigest {
		t.Fatalf("analysis did not preserve the one-record counterfactual: %#v", analysis)
	}
	if analysis.Rows[0].DistributionAware.SelectedIndex == nil || *analysis.Rows[0].DistributionAware.SelectedIndex != 1 {
		t.Fatalf("distribution-aware decision = %#v", analysis.Rows[0].DistributionAware)
	}
	if analysis.Rows[0].ChosenToken.State != DecisionTied {
		t.Fatalf("chosen-token decision = %#v, want tied", analysis.Rows[0].ChosenToken)
	}
}

func TestIdenticalResponseAnalysisValidatesMcNemarPValue(t *testing.T) {
	raw, err := os.ReadFile("../../eval/governance/identical-response-offline-analysis-v5.json")
	if err != nil {
		t.Fatal(err)
	}
	var analysis IdenticalResponseAnalysis
	if err := decodeIdenticalJSON(raw, &analysis); err != nil {
		t.Fatal(err)
	}
	if err := analysis.Validate(); err != nil {
		t.Fatal(err)
	}
	analysis.OutcomeSensitivity.McNemarExactP = 0.5
	if err := analysis.Validate(); err == nil {
		t.Fatal("analysis accepted a tampered McNemar exact p-value")
	}
}
