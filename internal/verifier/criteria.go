package verifier

import "fmt"

type Criterion struct {
	ID              string
	Name            string
	Description     string
	GroundTruthNote string
}

// Descriptions mirror eval/python-reference (run_terminal_bench.py and
// run_swe_bench.py CRITERIA) verbatim so paper-parity runs are
// prompt-identical with the reference implementation.
var BuiltinCriteria = map[string]Criterion{
	"specification": {
		ID:   "specification",
		Name: "Specification Adherence",
		Description: "Re-read the task description and check the SPECIFIC requirements: " +
			"exact file paths, install locations, output formats, naming, and any explicit " +
			"constraints (e.g. \"no X11 support\", \"install to /usr/local/bin/X\", \"output JSON to " +
			"/app/out.json\"). Did the agent meet these specific requirements, or did they " +
			"produce a solution that solves a similar but different problem (right idea, wrong " +
			"place / wrong format / missing constraint)?",
		GroundTruthNote: "Re-read the task SPEC carefully. Reward exact compliance over similar-but-different " +
			"solutions. Penalize \"right idea, wrong place / wrong format / missing constraint\".",
	},
	"output_match": {
		ID:   "output_match",
		Name: "Output Match",
		Description: "Find the FINAL verification command the agent ran (the one " +
			"that should prove the solution works). Compare its actual " +
			"stdout/stderr output, character-by-character if needed, to " +
			"what the task description says the output should look like. " +
			"For example: if the task says it should print " +
			"\"Results: X Y Z\" with integers, did the agent's last test " +
			"actually print that? If the task asks for a JSON file, do the " +
			"values look plausible and well-formed in the cat output? " +
			"Reward trajectories whose terminal SHOWS the expected output " +
			"literally. Ignore everything except whether the observed " +
			"output matches the expected output.",
		GroundTruthNote: "Focus on TERMINAL OUTPUT or produced ARTIFACTS as ground truth. " +
			"Do NOT trust the agent's self-assessment or claims of success.",
	},
	"error_signals": {
		ID:   "error_signals",
		Name: "Error Signal Detection",
		Description: "Scan the trajectory — especially the later steps — for " +
			"explicit failure markers: error messages, exception " +
			"tracebacks, segmentation faults, \"command not found\", " +
			"\"No such file or directory\", non-zero exit codes that the " +
			"agent did not subsequently fix, compilation failures, test " +
			"failures, etc. A trajectory that ends with unresolved errors " +
			"is almost certainly broken even if the agent claims success. " +
			"Conversely, a clean trajectory whose final commands all " +
			"succeed without errors is a strong positive signal. Score " +
			"based ONLY on the presence/absence of unresolved error signals.",
		GroundTruthNote: "Scan late steps for unresolved errors, exception tracebacks, non-zero exit " +
			"codes, compile/test failures. End-state errors weigh heavily.",
	},
	"root_cause": {
		ID:   "root_cause",
		Name: "Root Cause Analysis",
		Description: "Read the issue, identify the buggy behavior it describes, " +
			"and trace it to the code that produces it. Decide whether " +
			"the patch modifies the actual code path responsible for " +
			"the bug, or only its symptoms. A patch that edits the " +
			"buggy function or branch should score HIGH; a patch that " +
			"catches the bad output downstream, special-cases the " +
			"literal example in the issue, edits a caller to work " +
			"around a buggy callee, or changes a default to dodge the " +
			"broken path should score LOW. Judge by WHERE the change " +
			"lands in the call stack — both small and larger fixes are " +
			"valid as long as the edited lines are the ones whose " +
			"behavior the issue actually depends on.",
		GroundTruthNote: "Patches must address the actual buggy code path; reject symptom-only patches, " +
			"downstream catches, or special-cased example workarounds.",
	},
	"code_review": {
		ID:   "code_review",
		Name: "Code Quality",
		Description: "Review the agent's final patch (`diff --git ...`) as an " +
			"experienced code reviewer would. Check syntactic validity, " +
			"semantic correctness (right API, right types, right control " +
			"flow, no off-by-one, no swapped arguments, no shadowed or " +
			"unbound names), preservation of existing contracts " +
			"(function signatures, return types, exception types and " +
			"messages, output formats, default behavior), and " +
			"consistency with surrounding code style. Pay attention to " +
			"silent regressions in code paths the issue did not " +
			"explicitly mention — these are the most common cause of a " +
			"patch that looks fine but breaks something else. Judge " +
			"the diff on its technical merits, not by length or " +
			"apparent effort.",
		GroundTruthNote: "Judge the final patch as a code reviewer: correctness, contracts, and silent " +
			"regressions outweigh style. Do NOT trust the agent's claims about the patch.",
	},
	// The two criteria below exist for one experiment and are not part of any
	// default bundle. Both are byte-identical to their originals except that the
	// clause telling the model to disregard patch size is removed.
	//
	// Motivation, measured 2026-08-07: on SWE-bench Verified the single most
	// informative zero-cost selector is the hunk count of the final patch, which
	// solves 56 of 86 decidable tasks against the verifier's 49. The originals
	// instruct the model to judge "not by length or apparent effort" and that
	// "both small and larger fixes are valid". The verifier is therefore told to
	// ignore precisely the signal that wins, and these variants isolate whether
	// that instruction is the cause.
	"root_cause_size_agnostic": {
		ID:   "root_cause_size_agnostic",
		Name: "Root Cause Analysis (size prior removed)",
		Description: "Read the issue, identify the buggy behavior it describes, " +
			"and trace it to the code that produces it. Decide whether " +
			"the patch modifies the actual code path responsible for " +
			"the bug, or only its symptoms. A patch that edits the " +
			"buggy function or branch should score HIGH; a patch that " +
			"catches the bad output downstream, special-cases the " +
			"literal example in the issue, edits a caller to work " +
			"around a buggy callee, or changes a default to dodge the " +
			"broken path should score LOW. Judge by WHERE the change " +
			"lands in the call stack.",
		GroundTruthNote: "Patches must address the actual buggy code path; reject symptom-only patches, " +
			"downstream catches, or special-cased example workarounds.",
	},
	"code_review_size_agnostic": {
		ID:   "code_review_size_agnostic",
		Name: "Code Quality (size prior removed)",
		Description: "Review the agent's final patch (`diff --git ...`) as an " +
			"experienced code reviewer would. Check syntactic validity, " +
			"semantic correctness (right API, right types, right control " +
			"flow, no off-by-one, no swapped arguments, no shadowed or " +
			"unbound names), preservation of existing contracts " +
			"(function signatures, return types, exception types and " +
			"messages, output formats, default behavior), and " +
			"consistency with surrounding code style. Pay attention to " +
			"silent regressions in code paths the issue did not " +
			"explicitly mention — these are the most common cause of a " +
			"patch that looks fine but breaks something else. Judge " +
			"the diff on its technical merits.",
		GroundTruthNote: "Judge the final patch as a code reviewer: correctness, contracts, and silent " +
			"regressions outweigh style. Do NOT trust the agent's claims about the patch.",
	},
	"verification": {
		ID:   "verification",
		Name: "Empirical Verification",
		Description: "Look at the commands the agent actually ran and what they " +
			"printed, not what the agent claimed in its narration. " +
			"Reward agents that (a) constructed a reproducer for the " +
			"failure described in the issue, (b) observed the failure " +
			"before applying the fix, (c) observed the expected correct " +
			"behavior after the fix, and (d) ran the existing tests in " +
			"the affected module without breaking them. Trust observed " +
			"command output over the agent's narration of it. Penalize " +
			"agents that declared success without running anything, " +
			"misread their own command output (e.g. compared a literal " +
			"string to itself, ignored a traceback, claimed a test " +
			"passed when it errored), or edited the code again after " +
			"the last successful verification step so the final patch " +
			"is untested.",
		GroundTruthNote: "Trust observed command output over agent narration. Unverified or " +
			"post-verification-edited patches score LOW.",
	},
	"test_coverage": {
		ID:   "test_coverage",
		Name: "Test Behavior Match",
		Description: "Compare actual test outputs against expected. Reward literal match. Penalize " +
			"fake-greens (assertions disabled, tests skipped, mocked-out internals).",
		GroundTruthNote: "Compare actual test outputs against expected. Reward literal match. Penalize " +
			"fake-greens or skipped tests.",
	},
	"code_quality": {
		ID:   "code_quality",
		Name: "Code Quality",
		Description: "Evaluate readability, idiom adherence, no dead code, no obvious smell, no " +
			"over-engineering, sensible naming, structural consistency with surrounding code.",
		GroundTruthNote: "Evaluate readability, idiom adherence, no dead code, no obvious smell, no " +
			"over-engineering.",
	},
	"generic": {
		ID:   "generic",
		Name: "Generic Correctness",
		Description: "Evaluate overall correctness and completeness. Look for signs of success " +
			"(working final outputs, clean error-free runs, fulfilled requirements) and signs of " +
			"failure (incomplete implementation, broken steps, unaddressed task elements).",
		GroundTruthNote: "",
	},
}

// Benchmark-level ground-truth notes from the reference implementation. In
// paper-parity mode every criterion prompt carries the benchmark note, exactly
// like the python scripts pass one note for all criteria.
const (
	PaperTerminalGroundTruthNote = "**IMPORTANT:** Focus on TERMINAL OUTPUT as ground truth. " +
		"Do NOT trust the agent's self-assessment or claims of success. " +
		"Agents often claim success when the terminal shows errors."
	PaperSWEGroundTruthNote = "**Do NOT trust the agent's self-assessment or claims that \"the patch looks correct\". " +
		"Agents routinely declare success on patches that fix the wrong file, " +
		"address only a symptom, or are subtly broken."
)

// PaperTerminalCriteria returns the Terminal-Bench criteria set of the paper
// (C1..C3) with the benchmark-level ground-truth note applied.
func PaperTerminalCriteria() []Criterion {
	return withNote([]string{"specification", "output_match", "error_signals"}, PaperTerminalGroundTruthNote)
}

// PaperSWECriteria returns the SWE-bench Verified criteria set of the paper
// with the benchmark-level ground-truth note applied.
func PaperSWECriteria() []Criterion {
	return withNote([]string{"root_cause", "code_review", "verification"}, PaperSWEGroundTruthNote)
}

func withNote(ids []string, note string) []Criterion {
	out := make([]Criterion, 0, len(ids))
	for _, id := range ids {
		c := BuiltinCriteria[id]
		c.GroundTruthNote = note
		out = append(out, c)
	}
	return out
}

func ResolveCriteria(names []string) ([]Criterion, error) {
	if len(names) == 0 {
		return []Criterion{BuiltinCriteria["generic"]}, nil
	}
	out := make([]Criterion, 0, len(names))
	for _, n := range names {
		c, ok := BuiltinCriteria[n]
		if !ok {
			return nil, fmt.Errorf("unknown criterion %q", n)
		}
		out = append(out, c)
	}
	return out, nil
}
