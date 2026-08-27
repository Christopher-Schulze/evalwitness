package verifier

import "testing"

// TestPromptPairwiseReferenceParity asserts byte-equality between the Go
// pairwise prompt (critique disabled, as in --paper-parity) and the python
// reference template (verifier_core.py create_prompt_for_criterion).
func TestPromptPairwiseReferenceParity(t *testing.T) {
	task := "Fix the flaky test in pkg/foo."
	traceA := "--- Agent Step 1 ---\nran tests, fixed race"
	traceB := "--- Agent Step 1 ---\nclaimed success without running tests"

	for _, crit := range PaperSWECriteria() {
		got, tags := PromptPairwise(task, traceA, traceB, crit, PromptOptions{CritiqueThenScore: false})
		want := "You are an expert evaluator of AI coding agents. " +
			"You will see a task description and two agent trajectories. " +
			"Your job is to evaluate them on ONE specific criterion: " +
			"**" + crit.Name + "**.\n\n" +
			crit.GroundTruthNote + "\n\n" +
			"**Task:**\n" + task + "\n\n" +
			"**Trajectory A:**\n" + traceA + "\n\n" +
			"**Trajectory B:**\n" + traceB + "\n\n" +
			"**Evaluation Guideline — " + crit.Name + ":**\n" +
			crit.Description + "\n\n" +
			"Score each trajectory ONLY on this specific criterion. Ignore other " +
			"aspects of the trajectory that are not relevant to " +
			"\"" + crit.Name + "\".\n\n" +
			"**Rating Scale:**\n" + ScaleDescription + "\n\n" +
			"Then output your final scores:\n" +
			"<score_A>LETTER_A_TO_T</score_A>\n" +
			"<score_B>LETTER_A_TO_T</score_B>\n\n" +
			"Begin your analysis now."
		if got != want {
			t.Errorf("criterion %s: prompt diverges from reference template\n--- got ---\n%s\n--- want ---\n%s", crit.ID, got, want)
		}
		if len(tags) != 2 || tags[0] != "<score_A>" || tags[1] != "<score_B>" {
			t.Errorf("tags = %v", tags)
		}
	}
}

func TestPaperCriteriaSets(t *testing.T) {
	terminal := PaperTerminalCriteria()
	swe := PaperSWECriteria()
	wantTerminal := []string{"specification", "output_match", "error_signals"}
	wantSWE := []string{"root_cause", "code_review", "verification"}
	for i, id := range wantTerminal {
		if terminal[i].ID != id {
			t.Errorf("terminal[%d] = %s, want %s", i, terminal[i].ID, id)
		}
		if terminal[i].GroundTruthNote != PaperTerminalGroundTruthNote {
			t.Errorf("terminal criterion %s missing benchmark note", id)
		}
	}
	for i, id := range wantSWE {
		if swe[i].ID != id {
			t.Errorf("swe[%d] = %s, want %s", i, swe[i].ID, id)
		}
		if swe[i].GroundTruthNote != PaperSWEGroundTruthNote {
			t.Errorf("swe criterion %s missing benchmark note", id)
		}
	}
	if _, err := ResolveCriteria([]string{"code_review", "verification"}); err != nil {
		t.Errorf("new criteria not resolvable: %v", err)
	}
}
