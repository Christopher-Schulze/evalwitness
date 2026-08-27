package provider

import "testing"

// TestDistributionAtStreamOffsetMergedLetterToken pins the b.ai tokenization
// case: the score letter merges with the closing-tag ">" into one token
// (">T"). The stream-offset extractor must find the letter at offset 1 inside
// that merged token, and the top-logprob alternatives must be read at the
// same offset so alternatives whose offset-1 byte is a score letter count.
func TestDistributionAtStreamOffsetMergedLetterToken(t *testing.T) {
	tokens := []openaiLogprobToken{
		{Token: "<", Logprob: -0.01, TopLogprobs: []openaiLogprobAlternative{{Token: "<", Logprob: -0.01}}},
		{Token: "score", Logprob: -0.02, TopLogprobs: []openaiLogprobAlternative{{Token: "score", Logprob: -0.02}}},
		{Token: "_A", Logprob: -0.03, TopLogprobs: []openaiLogprobAlternative{{Token: "_A", Logprob: -0.03}}},
		{Token: ">T", Logprob: -0.10, TopLogprobs: []openaiLogprobAlternative{
			{Token: ">T", Logprob: -0.10},
			{Token: ">K", Logprob: -2.0},
			{Token: ">M", Logprob: -3.5},
			{Token: "> ", Logprob: -4.0},
		}},
	}
	spans := make([]logprobTokenSpan, 0, len(tokens))
	var stream strings2Builder
	for _, tok := range tokens {
		start := len(stream.String())
		stream.WriteString(tok.Token)
		spans = append(spans, logprobTokenSpan{start: start, end: len(stream.String()), token: tok})
	}
	scoreOffset := len("<score_A>")
	dist := distributionAtStreamOffset(spans, scoreOffset, 'T', false)
	if len(dist) == 0 {
		t.Fatal("merged >T tokenization produced no distribution")
	}
	if dist["T"] <= 0 {
		t.Fatalf("chosen T missing from distribution: %v", dist)
	}
	if _, hasK := dist["K"]; !hasK {
		t.Fatalf("alternative K at same offset not extracted: %v", dist)
	}
	if _, hasSpace := dist[" "]; hasSpace {
		t.Fatalf("non-letter alternative leaked into distribution: %v", dist)
	}
}

type strings2Builder struct{ s []byte }

func (b *strings2Builder) WriteString(s string) { b.s = append(b.s, s...) }
func (b *strings2Builder) String() string       { return string(b.s) }

// TestLastRawScoreForTagSpacedValue covers the spaced variant observed on
// b.ai ("<score_A> T </score_A>"): the expected-score reader trims whitespace
// before and after the letter.
func TestLastRawScoreForTagSpacedValue(t *testing.T) {
	raw := "<score_A> T </score_A>\n<score_B> A </score_B>"
	got, ok := lastRawScoreForTag(raw, "<score_A>")
	if !ok || got != 'T' {
		t.Fatalf("spaced <score_A> value = %q ok=%v, want T", got, ok)
	}
	gotB, okB := lastRawScoreForTag(raw, "<score_B>")
	if !okB || gotB != 'A' {
		t.Fatalf("spaced <score_B> value = %q ok=%v, want A", gotB, okB)
	}
}
