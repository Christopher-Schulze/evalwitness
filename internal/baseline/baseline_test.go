package baseline

import (
	"math"
	"regexp"
	"testing"
)

func ptr(i int) *int { return &i }

func TestPickResolvesTiesToTheLowestIndex(t *testing.T) {
	// A feature the benchmark does not record is zero everywhere. The baseline
	// must then degenerate into "pick the first" deterministically, so it can be
	// recognised as reading nothing rather than looking like a real selector.
	candidates := []Candidate{{}, {}, {}}
	for _, b := range SWEbench() {
		if got := b.Pick(candidates); got != 0 {
			t.Fatalf("%s picked %d over a constant feature, want 0", b.Name, got)
		}
	}
}

func TestPickReadsBothDirections(t *testing.T) {
	candidates := []Candidate{{Steps: 5}, {Steps: 1}, {Steps: 9}}
	var fewest, most Baseline
	for _, b := range Terminal() {
		if b.Feature == "steps" && b.Order == Fewest {
			fewest = b
		}
		if b.Feature == "steps" && b.Order == Most {
			most = b
		}
	}
	if got := fewest.Pick(candidates); got != 1 {
		t.Fatalf("fewest steps picked %d, want 1", got)
	}
	if got := most.Pick(candidates); got != 2 {
		t.Fatalf("most steps picked %d, want 2", got)
	}
}

func TestBothDirectionsOfEveryFeatureAreRegistered(t *testing.T) {
	// Registering only the direction that wins is how a baseline suite becomes
	// theatre, so the pairing is a structural property and not a convention.
	for _, set := range [][]Baseline{Terminal(), SWEbench()} {
		seen := map[string]map[Order]bool{}
		for _, b := range set {
			if b.Feature == "none" {
				continue
			}
			if seen[b.Feature] == nil {
				seen[b.Feature] = map[Order]bool{}
			}
			seen[b.Feature][b.Order] = true
		}
		for feature, orders := range seen {
			if !orders[Fewest] || !orders[Most] {
				t.Fatalf("feature %q is registered in only one direction", feature)
			}
		}
	}
}

func TestPatchFeaturesCountsOnlyLineStarts(t *testing.T) {
	patch := "diff --git a/x.py b/x.py\n" +
		"--- a/x.py\n+++ b/x.py\n" +
		"@@ -1,3 +1,4 @@\n-old\n+new\n" +
		"@@ -20,2 +21,3 @@\n+more\n" +
		"diff --git a/y.py b/y.py\n@@ -1 +1 @@\n" +
		"the message body mentions @@ and diff --git a/z b/z inline\n"
	bytes, files, hunks := PatchFeatures(patch)
	if bytes != len(patch) {
		t.Fatalf("bytes = %d, want %d", bytes, len(patch))
	}
	if files != 2 {
		t.Fatalf("files = %d, want 2; inline mentions must not count", files)
	}
	if hunks != 3 {
		t.Fatalf("hunks = %d, want 3; inline mentions must not count", hunks)
	}
}

func TestCountErrorWordsMatchesWholeWordsOnly(t *testing.T) {
	trace := "Error: build failed\nTraceback (most recent call last)\nException raised\n" +
		"but errorless and unfailed and exceptional must not match"
	if got := CountErrorWords(trace); got != 4 {
		t.Fatalf("CountErrorWords = %d, want 4", got)
	}
}

func TestCountErrorWordsMatchesFrozenRegexSemantics(t *testing.T) {
	reference := regexp.MustCompile(`(?i)\b(error|traceback|failed|exception)\b`)
	cases := []string{
		"", "ERROR error Error", "error-error", "error_error", "2error error2",
		"éerroré", "failed\x00exception", "traceback/EXCEPTION/ordinary",
	}
	for _, trace := range cases {
		want := len(reference.FindAllStringIndex(trace, -1))
		if got := CountErrorWords(trace); got != want {
			t.Fatalf("CountErrorWords(%q) = %d, want frozen-regex count %d", trace, got, want)
		}
	}
}

func TestDecidableExcludesTasksSelectionCannotChange(t *testing.T) {
	cases := []struct {
		name    string
		rewards []int
		want    bool
	}{
		{"all pass", []int{1, 1, 1}, false},
		{"all fail", []int{0, 0, 0}, false},
		{"mixed", []int{1, 0, 1}, true},
		{"single trajectory", []int{1}, false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		if got := (Task{Rewards: tc.rewards}).Decidable(); got != tc.want {
			t.Fatalf("%s: Decidable = %t, want %t", tc.name, got, tc.want)
		}
	}
}

func TestEvaluateScoresConcordantTasksLikeTheEvalHarness(t *testing.T) {
	tasks := []Task{
		// Two all-pass tasks contribute to the score without being decidable.
		{Rewards: []int{1, 1}, Candidates: []Candidate{{Steps: 1}, {Steps: 2}}},
		{Rewards: []int{1, 1}, Candidates: []Candidate{{Steps: 1}, {Steps: 2}}},
		// One all-fail task contributes nothing.
		{Rewards: []int{0, 0}, Candidates: []Candidate{{Steps: 1}, {Steps: 2}}},
		// One decidable task the "most steps" baseline gets right.
		{Rewards: []int{0, 1}, Candidates: []Candidate{{Steps: 1}, {Steps: 2}}, SelectedIndex: ptr(0)},
	}
	var most Baseline
	for _, b := range Terminal() {
		if b.Feature == "steps" && b.Order == Most {
			most = b
		}
	}
	got := Evaluate([]Baseline{most}, tasks)[0]
	if got.DecidableTotal != 1 || got.DecidableSolved != 1 {
		t.Fatalf("decidable = %d/%d, want 1/1", got.DecidableSolved, got.DecidableTotal)
	}
	if got.Score != 3 {
		t.Fatalf("score = %v, want 3 (two concordant passes plus one decidable)", got.Score)
	}
	// The subject picked index 0 and was wrong; the baseline picked 1 and was right.
	if got.BaselineOnly != 1 || got.SubjectOnly != 0 || got.Discordant != 1 {
		t.Fatalf("paired counts = subject %d baseline %d discordant %d, want 0/1/1",
			got.SubjectOnly, got.BaselineOnly, got.Discordant)
	}
	if got.Detectable {
		t.Fatal("one discordant pair can never be significant, but Detectable is true")
	}
}

func TestEvaluateSkipsTasksWithoutASubjectSelection(t *testing.T) {
	// A task the run skipped still counts toward the baseline's own score, but
	// must not enter the paired comparison, where it has no counterpart.
	tasks := []Task{
		{Rewards: []int{0, 1}, Candidates: []Candidate{{Steps: 1}, {Steps: 2}}},
		{Rewards: []int{0, 1}, Candidates: []Candidate{{Steps: 1}, {Steps: 2}}, SelectedIndex: ptr(1)},
	}
	var most Baseline
	for _, b := range Terminal() {
		if b.Feature == "steps" && b.Order == Most {
			most = b
		}
	}
	got := Evaluate([]Baseline{most}, tasks)[0]
	if got.DecidableTotal != 2 || got.DecidableSolved != 2 {
		t.Fatalf("decidable = %d/%d, want 2/2", got.DecidableSolved, got.DecidableTotal)
	}
	if got.PairedTasks != 1 {
		t.Fatalf("paired tasks = %d, want 1", got.PairedTasks)
	}
	if math.Abs(got.McNemarP-1) > 1e-12 {
		t.Fatalf("p = %v with no disagreement, want 1", got.McNemarP)
	}
}

func TestStrongestReportsTheHardestBaselineToBeat(t *testing.T) {
	results := []Result{
		{Name: "weak", DecidableSolved: 3},
		{Name: "strong", DecidableSolved: 9},
		{Name: "middling", DecidableSolved: 7},
	}
	best, ok := Strongest(results)
	if !ok || best.Name != "strong" {
		t.Fatalf("Strongest = %q (%t), want \"strong\"", best.Name, ok)
	}
	if _, ok := Strongest(nil); ok {
		t.Fatal("Strongest reported a result over no baselines")
	}
}
