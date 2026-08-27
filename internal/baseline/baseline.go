// Package baseline selects a trajectory using only features that cost nothing
// to compute, so a verifier's benefit can be stated against something other
// than random.
//
// This exists because the question is unavoidable. A selector that calls a
// model hundreds of times has to beat one that counts bytes, and until that is
// measured, a result against random says only that the model read something.
// Measured on 2026-08-07 the answer was uncomfortable on SWE-bench, which is
// exactly why the suite ships rather than staying a private script.
package baseline

import (
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

// Candidate is one trajectory reduced to the artifact features a zero-cost
// selector can see. A benchmark that does not record a feature leaves it zero
// and must not register the baselines that read it, because a baseline over a
// constant silently degenerates into "pick the first" and would be reported as
// if it were a real selector.
type Candidate struct {
	Steps        int
	OutputTokens int
	TraceBytes   int
	ErrorWords   int
	DurationSec  float64
	CostCents    float64
	PatchBytes   int
	PatchFiles   int
	PatchHunks   int
}

// Order is the direction a baseline reads its feature in. Both directions of
// every feature are always registered: "fewest steps" and "most steps" are
// different hypotheses, and keeping only the one that wins after seeing the
// results is how a baseline suite turns into theatre.
type Order string

const (
	Fewest Order = "fewest"
	Most   Order = "most"
)

// Baseline picks one trajectory from a task's candidates.
type Baseline struct {
	Name    string
	Feature string
	Order   Order
	value   func(Candidate) float64
}

// Pick returns the index of the selected candidate. Ties resolve to the lowest
// index, deterministically, so a baseline over a constant feature is exactly
// "pick the first" rather than something unreproducible.
func (b Baseline) Pick(candidates []Candidate) int {
	if len(candidates) == 0 {
		return 0
	}
	best := 0
	bestValue := b.value(candidates[0])
	for i := 1; i < len(candidates); i++ {
		v := b.value(candidates[i])
		if (b.Order == Most && v > bestValue) || (b.Order == Fewest && v < bestValue) {
			best, bestValue = i, v
		}
	}
	return best
}

func both(feature string, value func(Candidate) float64) []Baseline {
	return []Baseline{
		{Name: string(Fewest) + " " + feature, Feature: feature, Order: Fewest, value: value},
		{Name: string(Most) + " " + feature, Feature: feature, Order: Most, value: value},
	}
}

// FirstListed is the control that does no work at all. Any baseline scoring at
// its level is reading a constant, whatever its name suggests.
var FirstListed = Baseline{
	Name:    "first listed",
	Feature: "none",
	Order:   Fewest,
	value:   func(Candidate) float64 { return 0 },
}

// Terminal returns the baselines Terminal-Bench records the features for.
func Terminal() []Baseline {
	out := []Baseline{FirstListed}
	out = append(out, both("steps", func(c Candidate) float64 { return float64(c.Steps) })...)
	out = append(out, both("output tokens", func(c Candidate) float64 { return float64(c.OutputTokens) })...)
	out = append(out, both("duration", func(c Candidate) float64 { return c.DurationSec })...)
	out = append(out, both("cost", func(c Candidate) float64 { return c.CostCents })...)
	out = append(out, both("trace bytes", func(c Candidate) float64 { return float64(c.TraceBytes) })...)
	out = append(out, both("error words", func(c Candidate) float64 { return float64(c.ErrorWords) })...)
	return out
}

// SWEbench returns the baselines SWE-bench records the features for. It has no
// timing or cost, and adds the final patch, which is where the strongest
// zero-cost signal on this benchmark turned out to live.
func SWEbench() []Baseline {
	out := []Baseline{FirstListed}
	out = append(out, both("steps", func(c Candidate) float64 { return float64(c.Steps) })...)
	out = append(out, both("trace bytes", func(c Candidate) float64 { return float64(c.TraceBytes) })...)
	out = append(out, both("error words", func(c Candidate) float64 { return float64(c.ErrorWords) })...)
	out = append(out, both("patch bytes", func(c Candidate) float64 { return float64(c.PatchBytes) })...)
	out = append(out, both("patch files", func(c Candidate) float64 { return float64(c.PatchFiles) })...)
	out = append(out, both("patch hunks", func(c Candidate) float64 { return float64(c.PatchHunks) })...)
	return out
}

// CountErrorWords counts failure vocabulary in a trace. It is a crude proxy for
// "this run went badly" and is meant to be crude: the point of a baseline is
// that it costs nothing, not that it is clever.
func CountErrorWords(trace string) int {
	count := 0
	for start := 0; start < len(trace); {
		for start < len(trace) && !isASCIIWordByte(trace[start]) {
			start++
		}
		end := start
		for end < len(trace) && isASCIIWordByte(trace[end]) {
			end++
		}
		if end > start && isErrorWord(trace[start:end]) {
			count++
		}
		start = end
	}
	return count
}

func isASCIIWordByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' || value == '_'
}

func isErrorWord(value string) bool {
	switch len(value) {
	case len("error"):
		return strings.EqualFold(value, "error")
	case len("failed"):
		return strings.EqualFold(value, "failed")
	case len("traceback"):
		return strings.EqualFold(value, "traceback") || strings.EqualFold(value, "exception")
	default:
		return false
	}
}

// PatchFeatures reports the size of a unified diff. Files counts `diff --git`
// headers and hunks counts `@@` markers, both at line starts so that diff
// content quoted inside a message body does not inflate them.
func PatchFeatures(patch string) (bytes, files, hunks int) {
	bytes = len(patch)
	for line := range strings.SplitSeq(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			files++
		case strings.HasPrefix(line, "@@"):
			hunks++
		}
	}
	return bytes, files, hunks
}

// Task is one benchmark task with ground truth and, where the run made one, the
// selection under test.
type Task struct {
	Candidates    []Candidate
	Rewards       []int
	SelectedIndex *int
}

// Decidable reports whether selection can change this task's outcome. Where
// every trajectory passes or every one fails, the selector is irrelevant and
// counting the task inflates every arm toward whatever the dataset contains.
func (t Task) Decidable() bool {
	return stats.DecidableBinary(t.Rewards)
}

// Result is one baseline scored against ground truth, and paired against the
// selection under test wherever that selection exists.
type Result struct {
	Name            string  `json:"name"`
	Feature         string  `json:"feature"`
	Order           Order   `json:"order"`
	DecidableSolved int     `json:"decidable_solved"`
	DecidableTotal  int     `json:"decidable_total"`
	Score           float64 `json:"score"`

	PairedTasks  int     `json:"paired_tasks"`
	BothCorrect  int     `json:"both_correct"`
	BothWrong    int     `json:"both_wrong"`
	SubjectOnly  int     `json:"subject_only"`
	BaselineOnly int     `json:"baseline_only"`
	Discordant   int     `json:"discordant"`
	McNemarP     float64 `json:"mcnemar_p"`
	// SignificantAt05 is false whenever no split of Discordant would have
	// reached significance, so a p-value is never read without the sample that
	// could have produced it.
	DetectableSplit int  `json:"detectable_split"`
	Detectable      bool `json:"detectable"`
}

// Evaluate scores every baseline over the same tasks the subject was scored on.
// Score counts concordant tasks the same way the eval harness does, so a
// baseline figure is directly comparable to a verifier score rather than to a
// decidable-only subtotal.
func Evaluate(baselines []Baseline, tasks []Task) []Result {
	concordant := 0.0
	for _, t := range tasks {
		if !t.Decidable() && len(t.Rewards) > 0 && t.Rewards[0] > 0 {
			concordant++
		}
	}

	results := make([]Result, 0, len(baselines))
	for _, b := range baselines {
		r := Result{Name: b.Name, Feature: b.Feature, Order: b.Order, Score: concordant}
		for _, t := range tasks {
			if !t.Decidable() {
				continue
			}
			r.DecidableTotal++
			picked := b.Pick(t.Candidates)
			baselineCorrect := picked < len(t.Rewards) && t.Rewards[picked] > 0
			if baselineCorrect {
				r.DecidableSolved++
				r.Score++
			}
			if t.SelectedIndex == nil || *t.SelectedIndex >= len(t.Rewards) {
				continue
			}
			r.PairedTasks++
			subjectCorrect := t.Rewards[*t.SelectedIndex] > 0
			switch {
			case subjectCorrect && baselineCorrect:
				r.BothCorrect++
			case subjectCorrect:
				r.SubjectOnly++
			case baselineCorrect:
				r.BaselineOnly++
			default:
				r.BothWrong++
			}
		}
		r.Discordant = r.SubjectOnly + r.BaselineOnly
		r.McNemarP = stats.McNemarExact(r.SubjectOnly, r.BaselineOnly)
		r.DetectableSplit, r.Detectable = stats.SmallestSignificantSplit(r.Discordant, 0.05)
		results = append(results, r)
	}
	return results
}

// Strongest returns the baseline that solved the most decidable tasks. It is
// deliberately the only selector this package offers over its own results: the
// strongest baseline is the one a claim has to beat, whether or not that is the
// convenient one.
func Strongest(results []Result) (Result, bool) {
	if len(results) == 0 {
		return Result{}, false
	}
	best := results[0]
	for _, r := range results[1:] {
		if r.DecidableSolved > best.DecidableSolved {
			best = r
		}
	}
	return best, true
}
