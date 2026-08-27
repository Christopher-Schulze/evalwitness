package verifier

import (
	"fmt"
	"sort"
	"strings"
)

type CriterionTags struct {
	CriterionID string
	Tags        []string
}

func AllTags(cts []CriterionTags) []string {
	var out []string
	for _, c := range cts {
		out = append(out, c.Tags...)
	}
	return out
}

// ScaleDescription matches the reference implementation byte-for-byte
// (verifier_core.py SCALE["scale_description"]), including the two-space
// indent on the letter rows, so parity-mode prompts hash identically.
const ScaleDescription = `Rate how likely the agent correctly solved the task on a 20-point scale using letters A through T:
  A = clearly and completely succeeded with verified output (best)
  B-D = succeeded with only minor issues
  E-G = above average, mostly correct with some issues
  H-J = uncertain, leans toward success
  K-M = uncertain, leans toward failure
  N-P = below average, significant issues remain
  Q-S = failed with some partial progress
  T = clearly and completely failed (worst)`

type PromptOptions struct {
	CritiqueThenScore bool
}

func PromptPairwise(task, traceA, traceB string, criterion Criterion, opts PromptOptions) (prompt string, scoreTags []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "You are an expert evaluator of AI coding agents. You will see a task description and two agent trajectories. Your job is to evaluate them on ONE specific criterion: **%s**.\n\n", criterion.Name)
	if criterion.GroundTruthNote != "" {
		fmt.Fprintf(&b, "%s\n\n", criterion.GroundTruthNote)
	}
	fmt.Fprintf(&b, "**Task:**\n%s\n\n", task)
	fmt.Fprintf(&b, "**Trajectory A:**\n%s\n\n", traceA)
	fmt.Fprintf(&b, "**Trajectory B:**\n%s\n\n", traceB)
	fmt.Fprintf(&b, "**Evaluation Guideline — %s:**\n%s\n\n", criterion.Name, criterion.Description)
	fmt.Fprintf(&b, "Score each trajectory ONLY on this specific criterion. Ignore other aspects of the trajectory that are not relevant to %q.\n\n", criterion.Name)
	if opts.CritiqueThenScore {
		b.WriteString("Before scoring, write a 1-2 sentence critique inside <critique_A>...</critique_A> and <critique_B>...</critique_B> tags identifying the strongest evidence for your judgment.\n\n")
	}
	fmt.Fprintf(&b, "**Rating Scale:**\n%s\n\n", ScaleDescription)
	b.WriteString("Then output your final scores:\n<score_A>LETTER_A_TO_T</score_A>\n<score_B>LETTER_A_TO_T</score_B>\n\n")
	b.WriteString("Begin your analysis now.")
	return b.String(), []string{"<score_A>", "<score_B>"}
}

func PromptPairwiseBundled(task, traceA, traceB string, criteria []Criterion, opts PromptOptions) (string, []CriterionTags) {
	sorted := make([]Criterion, len(criteria))
	copy(sorted, criteria)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var b strings.Builder
	b.WriteString("You are an expert evaluator of AI coding agents. You will see a task description and two agent trajectories. Your job is to evaluate them on MULTIPLE criteria simultaneously.\n\n")
	for _, c := range sorted {
		if c.GroundTruthNote != "" {
			fmt.Fprintf(&b, "**%s**: %s\n", c.Name, c.GroundTruthNote)
		}
	}
	if len(sorted) > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "**Task:**\n%s\n\n", task)
	fmt.Fprintf(&b, "**Trajectory A:**\n%s\n\n", traceA)
	fmt.Fprintf(&b, "**Trajectory B:**\n%s\n\n", traceB)
	b.WriteString("**Evaluation Criteria:**\n")
	for _, c := range sorted {
		fmt.Fprintf(&b, "- **%s** (id `%s`): %s\n", c.Name, c.ID, c.Description)
	}
	b.WriteString("\nScore each trajectory on each criterion INDEPENDENTLY. Each criterion gets its own score for A and its own score for B.\n\n")
	if opts.CritiqueThenScore {
		b.WriteString("Before scoring, write a brief critique inside <critique_A>...</critique_A> and <critique_B>...</critique_B> identifying the strongest evidence for your judgments.\n\n")
	}
	fmt.Fprintf(&b, "**Rating Scale:**\n%s\n\n", ScaleDescription)
	b.WriteString("Output one score per (trajectory, criterion) pair using EXACTLY these tags, in this order:\n")
	tags := make([]CriterionTags, 0, len(sorted))
	for _, c := range sorted {
		ta := fmt.Sprintf("<score_A_%s>", c.ID)
		tb := fmt.Sprintf("<score_B_%s>", c.ID)
		fmt.Fprintf(&b, "%sLETTER_A_TO_T</score_A_%s>\n", ta, c.ID)
		fmt.Fprintf(&b, "%sLETTER_A_TO_T</score_B_%s>\n", tb, c.ID)
		tags = append(tags, CriterionTags{CriterionID: c.ID, Tags: []string{ta, tb}})
	}
	b.WriteString("\nBegin your analysis now.")
	return b.String(), tags
}

func PromptAbsoluteBundled(task, trace string, criteria []Criterion, opts PromptOptions) (string, []CriterionTags) {
	sorted := make([]Criterion, len(criteria))
	copy(sorted, criteria)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var b strings.Builder
	b.WriteString("You are an expert evaluator of AI coding agents. You will see a task description and one agent trajectory. Your job is to evaluate it on MULTIPLE criteria simultaneously.\n\n")
	for _, c := range sorted {
		if c.GroundTruthNote != "" {
			fmt.Fprintf(&b, "**%s**: %s\n", c.Name, c.GroundTruthNote)
		}
	}
	if len(sorted) > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "**Task:**\n%s\n\n", task)
	fmt.Fprintf(&b, "**Trajectory:**\n%s\n\n", trace)
	b.WriteString("**Evaluation Criteria:**\n")
	for _, c := range sorted {
		fmt.Fprintf(&b, "- **%s** (id `%s`): %s\n", c.Name, c.ID, c.Description)
	}
	b.WriteString("\nScore the trajectory on each criterion INDEPENDENTLY.\n\n")
	if opts.CritiqueThenScore {
		b.WriteString("Before scoring, write a brief critique inside <critique>...</critique> identifying the strongest evidence for your judgments.\n\n")
	}
	fmt.Fprintf(&b, "**Rating Scale:**\n%s\n\n", ScaleDescription)
	b.WriteString("Output one score per criterion using EXACTLY these tags, in this order:\n")
	tags := make([]CriterionTags, 0, len(sorted))
	for _, c := range sorted {
		t := fmt.Sprintf("<score_%s>", c.ID)
		fmt.Fprintf(&b, "%sLETTER_A_TO_T</score_%s>\n", t, c.ID)
		tags = append(tags, CriterionTags{CriterionID: c.ID, Tags: []string{t}})
	}
	b.WriteString("\nBegin your analysis now.")
	return b.String(), tags
}

// PromptJointAbsolute scores every candidate independently inside one immutable
// completion. The fixed candidate set makes extraction-arm comparisons immune
// to adaptive branch changes while retaining one score-token distribution per
// candidate and criterion.
func PromptJointAbsolute(task string, traces []string, criteria []Criterion, opts PromptOptions) (string, []CriterionTags) {
	sorted := make([]Criterion, len(criteria))
	copy(sorted, criteria)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var b strings.Builder
	b.WriteString("You are an expert evaluator of AI coding agents. You will see a task description and multiple agent trajectories. Score every trajectory independently on every criterion. Do not rank candidates or let one candidate change another candidate's score.\n\n")
	for _, criterion := range sorted {
		if criterion.GroundTruthNote != "" {
			fmt.Fprintf(&b, "**%s**: %s\n", criterion.Name, criterion.GroundTruthNote)
		}
	}
	if len(sorted) > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "**Task:**\n%s\n\n", task)
	for index, trace := range traces {
		fmt.Fprintf(&b, "**Trajectory %d:**\n%s\n\n", index, trace)
	}
	b.WriteString("**Evaluation Criteria:**\n")
	for _, criterion := range sorted {
		fmt.Fprintf(&b, "- **%s** (id `%s`): %s\n", criterion.Name, criterion.ID, criterion.Description)
	}
	if opts.CritiqueThenScore {
		b.WriteString("\nBefore scoring, write one brief evidence-based critique per trajectory using <critique_0>...</critique_0>, <critique_1>...</critique_1>, and so on.\n")
	} else {
		b.WriteString("\nDo not output analysis, critique, ranking, prose, or any text outside the required score tags.\n")
	}
	fmt.Fprintf(&b, "\n**Rating Scale:**\n%s\n\n", ScaleDescription)
	b.WriteString("Output one score per (criterion, trajectory) pair using EXACTLY these tags, in this order:\n")
	tags := make([]CriterionTags, 0, len(sorted))
	for _, criterion := range sorted {
		criterionTags := CriterionTags{CriterionID: criterion.ID, Tags: make([]string, 0, len(traces))}
		for index := range traces {
			tag := fmt.Sprintf("<score_%d_%s>", index, criterion.ID)
			fmt.Fprintf(&b, "%sLETTER_A_TO_T</score_%d_%s>\n", tag, index, criterion.ID)
			criterionTags.Tags = append(criterionTags.Tags, tag)
		}
		tags = append(tags, criterionTags)
	}
	if opts.CritiqueThenScore {
		b.WriteString("\nBegin your analysis now.")
	} else {
		b.WriteString("\nOutput the final score tags now.")
	}
	return b.String(), tags
}

func PromptAbsolute(task, trace string, criterion Criterion, opts PromptOptions) (prompt string, scoreTags []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "You are an expert evaluator of AI coding agents. You will see a task description and one agent trajectory. Your job is to evaluate it on ONE specific criterion: **%s**.\n\n", criterion.Name)
	if criterion.GroundTruthNote != "" {
		fmt.Fprintf(&b, "%s\n\n", criterion.GroundTruthNote)
	}
	fmt.Fprintf(&b, "**Task:**\n%s\n\n", task)
	fmt.Fprintf(&b, "**Trajectory:**\n%s\n\n", trace)
	fmt.Fprintf(&b, "**Evaluation Guideline — %s:**\n%s\n\n", criterion.Name, criterion.Description)
	b.WriteString("Score the trajectory ONLY on this specific criterion. Ignore other aspects.\n\n")
	if opts.CritiqueThenScore {
		b.WriteString("Before scoring, write a 1-2 sentence critique inside <critique>...</critique> tags identifying the strongest evidence for your judgment.\n\n")
	}
	fmt.Fprintf(&b, "**Rating Scale:**\n%s\n\n", ScaleDescription)
	b.WriteString("Then output your final score:\n<score>LETTER_A_TO_T</score>\n\n")
	b.WriteString("Begin your analysis now.")
	return b.String(), []string{"<score>"}
}
