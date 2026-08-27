package verifier

import (
	"math"
	"testing"
)

func TestTokenValue(t *testing.T) {
	cases := []struct {
		in  string
		val int
		ok  bool
	}{
		{"A", 20, true},
		{"a", 20, true},
		{"T", 1, true},
		{"t", 1, true},
		{"M", 8, true},
		{" B ", 19, true},
		{"U", 0, false},
		{"AA", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		v, ok := TokenValue(c.in)
		if v != c.val || ok != c.ok {
			t.Errorf("TokenValue(%q) = (%d, %v), want (%d, %v)", c.in, v, ok, c.val, c.ok)
		}
	}
}

func TestExtractScore_DistributionHappy(t *testing.T) {
	dist := map[string]map[string]float64{
		"<score_A>": {
			"A": 0.7,
			"B": 0.2,
			"C": 0.1,
		},
	}
	r := ExtractScore("", dist, "<score_A>")
	if !r.Extracted {
		t.Fatalf("expected Extracted=true")
	}
	want := (19.6 - 1) / 19.0
	if math.Abs(r.Score-want) > 1e-6 {
		t.Errorf("Score = %v, want %v", r.Score, want)
	}
	if !r.FromDistribution {
		t.Fatal("expected distribution-backed extraction")
	}
	if r.Variance <= 0 {
		t.Fatalf("Variance = %v, want positive distribution variance", r.Variance)
	}
	if math.Abs(r.Mass-1) > 1e-9 {
		t.Fatalf("Mass = %v, want 1", r.Mass)
	}
}

func TestExtractScore_FallbackRegex(t *testing.T) {
	rawText := "<score_A>B</score_A>\n<score_B>K</score_B>"
	r := ExtractScore(rawText, nil, "<score_A>")
	if !r.Extracted {
		t.Fatalf("expected Extracted=true via regex")
	}
	want := 18.0 / 19.0
	if math.Abs(r.Score-want) > 1e-6 {
		t.Errorf("Score = %v, want %v", r.Score, want)
	}
	if r.FromDistribution {
		t.Fatal("regex fallback must not claim distribution evidence")
	}
	if r.Mass != 1 {
		t.Fatalf("fallback mass = %v, want 1", r.Mass)
	}
}

func TestExtractScore_NoMatch(t *testing.T) {
	r := ExtractScore("nothing here", nil, "<score_A>")
	if r.Extracted {
		t.Fatalf("expected Extracted=false")
	}
	if r.Score != 0.5 {
		t.Errorf("Score = %v, want 0.5", r.Score)
	}
}

func TestExtractScore_MixedCase(t *testing.T) {
	dist := map[string]map[string]float64{
		"<score_A>": {
			"A": 0.3,
			"a": 0.2,
		},
	}
	r := ExtractScore("", dist, "<score_A>")
	if !r.Extracted {
		t.Fatalf("expected Extracted=true")
	}
	if math.Abs(r.Score-1.0) > 1e-6 {
		t.Errorf("Score = %v, want 1.0", r.Score)
	}
}

func TestExtractScore_SparseFallsBack(t *testing.T) {
	dist := map[string]map[string]float64{
		"<score_A>": {
			"A": 0.01,
			"B": 0.005,
		},
	}
	rawText := "<score_A>C</score_A>"
	r := ExtractScore(rawText, dist, "<score_A>")
	if !r.Extracted {
		t.Fatalf("expected Extracted=true via regex fallback")
	}
	want := 17.0 / 19.0
	if math.Abs(r.Score-want) > 1e-6 {
		t.Errorf("Score = %v, want %v", r.Score, want)
	}
}

func TestResolveCriteria(t *testing.T) {
	out, err := ResolveCriteria([]string{"specification", "output_match"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(out))
	}
	if out[0].ID != "specification" || out[1].ID != "output_match" {
		t.Errorf("wrong order or ids: %v", out)
	}

	if _, err := ResolveCriteria([]string{"unknown"}); err == nil {
		t.Errorf("expected error for unknown criterion")
	}

	def, err := ResolveCriteria(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def) != 1 || def[0].ID != "generic" {
		t.Errorf("expected default generic, got %v", def)
	}
}

func TestPromptPairwise_Shape(t *testing.T) {
	prompt, tags := PromptPairwise("the task", "trace a", "trace b", BuiltinCriteria["generic"], PromptOptions{CritiqueThenScore: false})
	if len(tags) != 2 || tags[0] != "<score_A>" || tags[1] != "<score_B>" {
		t.Errorf("tags: %v", tags)
	}
	for _, must := range []string{"the task", "trace a", "trace b", "<score_A>LETTER_A_TO_T</score_A>", "<score_B>LETTER_A_TO_T</score_B>"} {
		if !contains(prompt, must) {
			t.Errorf("prompt missing %q", must)
		}
	}
}

func TestPromptPairwise_CritiqueOn(t *testing.T) {
	prompt, _ := PromptPairwise("t", "a", "b", BuiltinCriteria["generic"], PromptOptions{CritiqueThenScore: true})
	if !contains(prompt, "<critique_A>") || !contains(prompt, "<critique_B>") {
		t.Errorf("expected critique tags in prompt")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
