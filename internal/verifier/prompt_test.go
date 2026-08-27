package verifier

import (
	"fmt"
	"strings"
	"testing"
)

// The contract these tests defend: the tags a prompt builder returns are exactly
// the tags a model is instructed to emit, and exactly the tags ExtractScore
// looks for. A mismatch anywhere in that chain does not fail loudly. Extraction
// finds nothing, the score falls back to 0.5, and the run reports a plausible
// number that is really "pick the first". The whole publication gate on
// unextracted_scores exists because of this failure mode.

func modelReply(tags []string, letter string) string {
	var b strings.Builder
	for _, t := range tags {
		closing := strings.Replace(t, "<", "</", 1)
		fmt.Fprintf(&b, "%s%s%s\n", t, letter, closing)
	}
	return b.String()
}

func criteria(ids ...string) []Criterion {
	out := make([]Criterion, 0, len(ids))
	for _, id := range ids {
		out = append(out, Criterion{
			ID:              id,
			Name:            strings.ToUpper(id),
			Description:     "description for " + id,
			GroundTruthNote: "note for " + id,
		})
	}
	return out
}

func TestPairwisePromptTagsRoundTripThroughExtraction(t *testing.T) {
	prompt, tags := PromptPairwise("task", "trace A", "trace B", criteria("generic")[0], PromptOptions{})
	if len(tags) != 2 {
		t.Fatalf("tags = %v, want two", tags)
	}
	for _, tag := range tags {
		if !strings.Contains(prompt, tag) {
			t.Fatalf("prompt never instructs the model to emit %s", tag)
		}
		got := ExtractScore(modelReply([]string{tag}, "A"), nil, tag)
		if !got.Extracted {
			t.Fatalf("ExtractScore could not find %s in a reply built from the prompt's own tags", tag)
		}
	}
}

func TestBundledPairwisePromptTagsRoundTrip(t *testing.T) {
	cts := criteria("root_cause", "code_review", "verification")
	prompt, tags := PromptPairwiseBundled("task", "trace A", "trace B", cts, PromptOptions{})

	if len(tags) != len(cts) {
		t.Fatalf("%d tag groups for %d criteria", len(tags), len(cts))
	}
	all := AllTags(tags)
	if len(all) != 2*len(cts) {
		t.Fatalf("AllTags returned %d tags, want two per criterion", len(all))
	}

	reply := modelReply(all, "C")
	for _, tag := range all {
		if !strings.Contains(prompt, tag) {
			t.Fatalf("prompt never instructs the model to emit %s", tag)
		}
		got := ExtractScore(reply, nil, tag)
		if !got.Extracted {
			t.Fatalf("%s is unextractable from a reply that uses the prompt's own tags", tag)
		}
	}
}

func TestBundledAbsolutePromptTagsRoundTrip(t *testing.T) {
	cts := criteria("root_cause", "verification")
	prompt, tags := PromptAbsoluteBundled("task", "trace", cts, PromptOptions{})
	all := AllTags(tags)
	if len(all) != len(cts) {
		t.Fatalf("%d tags for %d criteria, want one each", len(all), len(cts))
	}
	reply := modelReply(all, "T")
	for _, tag := range all {
		if !strings.Contains(prompt, tag) {
			t.Fatalf("prompt never instructs the model to emit %s", tag)
		}
		if !ExtractScore(reply, nil, tag).Extracted {
			t.Fatalf("%s is unextractable", tag)
		}
	}
}

func TestJointAbsolutePromptTagsRoundTripAndStayDeterministic(t *testing.T) {
	forward := criteria("verification", "specification")
	reversed := []Criterion{forward[1], forward[0]}
	traces := []string{"trace zero", "trace one", "trace two"}
	prompt, tags := PromptJointAbsolute("task", traces, forward, PromptOptions{})
	reorderedPrompt, reorderedTags := PromptJointAbsolute("task", traces, reversed, PromptOptions{})
	if prompt != reorderedPrompt || strings.Join(AllTags(tags), ",") != strings.Join(AllTags(reorderedTags), ",") {
		t.Fatal("joint-absolute prompt depends on criterion input order")
	}
	if !strings.Contains(prompt, "Do not output analysis") || strings.Contains(prompt, "Begin your analysis now") {
		t.Fatal("no-critique joint prompt permits unbounded analysis before score tags")
	}
	all := AllTags(tags)
	if len(all) != len(traces)*len(forward) {
		t.Fatalf("joint tags = %d, want %d", len(all), len(traces)*len(forward))
	}
	reply := modelReply(all, "H")
	for _, tag := range all {
		if !strings.Contains(prompt, tag) || !ExtractScore(reply, nil, tag).Extracted {
			t.Fatalf("joint tag %s is absent or unextractable", tag)
		}
	}
}

func TestAbsolutePromptTagRoundTrip(t *testing.T) {
	prompt, tags := PromptAbsolute("task", "trace", criteria("generic")[0], PromptOptions{})
	if len(tags) != 1 {
		t.Fatalf("tags = %v, want one", tags)
	}
	if !strings.Contains(prompt, tags[0]) {
		t.Fatalf("prompt never instructs the model to emit %s", tags[0])
	}
	if !ExtractScore(modelReply(tags, "M"), nil, tags[0]).Extracted {
		t.Fatal("the single absolute tag is unextractable")
	}
}

func TestBundledTagsAreUniqueAcrossCriteria(t *testing.T) {
	// Two criteria sharing a tag would make one silently overwrite the other in
	// the distribution map, and the second criterion would score whatever the
	// first did.
	cts := criteria("root_cause", "code_review", "verification", "specification")
	_, tags := PromptPairwiseBundled("t", "a", "b", cts, PromptOptions{})
	seen := map[string]string{}
	for _, ct := range tags {
		for _, tag := range ct.Tags {
			if prev, dup := seen[tag]; dup {
				t.Fatalf("tag %s is used by both %s and %s", tag, prev, ct.CriterionID)
			}
			seen[tag] = ct.CriterionID
		}
	}
}

func TestBundledPromptIsDeterministicRegardlessOfCriterionOrder(t *testing.T) {
	// The prompt is a cache key. If criterion order leaked into it, the same
	// logical request would miss the cache and every published call count would
	// depend on map iteration order.
	forward := criteria("a_first", "m_middle", "z_last")
	reversed := []Criterion{forward[2], forward[0], forward[1]}

	p1, t1 := PromptPairwiseBundled("task", "A", "B", forward, PromptOptions{})
	p2, t2 := PromptPairwiseBundled("task", "A", "B", reversed, PromptOptions{})
	if p1 != p2 {
		t.Fatal("prompt depends on the order criteria were passed in")
	}
	if strings.Join(AllTags(t1), ",") != strings.Join(AllTags(t2), ",") {
		t.Fatal("tag order depends on the order criteria were passed in")
	}
}

func TestCritiqueOptionAddsTagsWithoutDisturbingScoreTags(t *testing.T) {
	cts := criteria("root_cause")
	plain, plainTags := PromptPairwiseBundled("task", "A", "B", cts, PromptOptions{})
	withCritique, critiqueTags := PromptPairwiseBundled("task", "A", "B", cts, PromptOptions{CritiqueThenScore: true})

	if strings.Contains(plain, "<critique_A>") {
		t.Fatal("critique instructions appear without the option")
	}
	if !strings.Contains(withCritique, "<critique_A>") || !strings.Contains(withCritique, "<critique_B>") {
		t.Fatal("critique instructions missing with the option set")
	}
	if strings.Join(AllTags(plainTags), ",") != strings.Join(AllTags(critiqueTags), ",") {
		t.Fatal("the critique option changed the score tags")
	}
	// A critique block sits before the scores and mentions letters. Extraction
	// must still resolve the score tag rather than something inside the critique.
	reply := "<critique_A>Trajectory A looks like a T to me</critique_A>\n" +
		modelReply(AllTags(critiqueTags), "B")
	for _, tag := range AllTags(critiqueTags) {
		got := ExtractScore(reply, nil, tag)
		if !got.Extracted {
			t.Fatalf("%s unextractable when a critique precedes it", tag)
		}
		// B is the second letter, so its normalised score is fixed and known.
		// Picking up the T inside the critique would score 0 instead.
		if want := 1 - 1.0/19.0; got.Score < want-1e-9 || got.Score > want+1e-9 {
			t.Fatalf("%s scored %.4f, want %.4f; a letter inside the critique was picked up", tag, got.Score, want)
		}
	}
}

func TestEmptyCriteriaProduceNoTags(t *testing.T) {
	// A caller that resolves to no criteria must get an empty tag set rather
	// than a prompt asking for scores nothing will look for.
	prompt, tags := PromptPairwiseBundled("task", "A", "B", nil, PromptOptions{})
	if len(AllTags(tags)) != 0 {
		t.Fatalf("tags = %v, want none", tags)
	}
	if strings.Contains(prompt, "<score_") {
		t.Fatal("prompt asks for score tags that no caller will extract")
	}
}

func TestScaleDescriptionCarriesTheFullLetterRange(t *testing.T) {
	// Extraction maps A through T onto [0,1]. A scale that stopped short would
	// make the endpoints unreachable while every score still looked valid.
	for _, boundary := range []string{"A =", "T ="} {
		if !strings.Contains(ScaleDescription, boundary) {
			t.Fatalf("scale description is missing the %q row", boundary)
		}
	}
	prompt, _ := PromptPairwise("t", "a", "b", criteria("generic")[0], PromptOptions{})
	if !strings.Contains(prompt, ScaleDescription) {
		t.Fatal("prompt does not carry the scale the extractor assumes")
	}
}
