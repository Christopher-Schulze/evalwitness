package verifier

import (
	"regexp"
	"strings"
)

const (
	ValueMin = 1.0
	ValueMax = 20.0
)

var (
	thinkBlockRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)
	codeFenceRegex  = regexp.MustCompile("(?s)```[a-zA-Z0-9_+\\-]*\\n.*?\\n```")
)

// StripThinkBlocks removes the reasoning blocks a thinking model emits before
// its answer. A score tag inside such a block is a mid-reasoning false positive;
// only post-think output is the actual answer.
func StripThinkBlocks(s string) string {
	return thinkBlockRegex.ReplaceAllString(s, "")
}

// StripCodeFences removes triple-backtick fenced blocks. LLMs sometimes echo
// the score-tag format inside an example fence (e.g. "use this format:
// ```\n<score_A>LETTER_A_TO_T</score_A>\n```") and then emit the actual
// answer outside the fence. Stripping the fenced block lets the regex find
// the real answer rather than a literal-template match.
func StripCodeFences(s string) string {
	return codeFenceRegex.ReplaceAllString(s, "")
}

func TokenValue(s string) (int, bool) {
	t := strings.TrimSpace(s)
	if len(t) != 1 {
		return 0, false
	}
	c := t[0]
	switch {
	case c >= 'A' && c <= 'T':
		return 20 - int(c-'A'), true
	case c >= 'a' && c <= 't':
		return 20 - int(c-'a'), true
	}
	return 0, false
}

type ExtractResult struct {
	Score            float64
	Variance         float64
	Extracted        bool
	Mass             float64
	FromDistribution bool
}

func ExtractScore(rawText string, distributions map[string]map[string]float64, tag string) ExtractResult {
	if dist := distributions[tag]; len(dist) > 0 {
		// Reference parity (verifier_core.py extract_score): upper/lower
		// aliases of the same letter keep the max probability, and the
		// expectation is normalized by that same max-per-value mass. The
		// 0.05 sparsity gate rejects distributions where score letters are
		// noise rather than the actual score position.
		massPerValue := map[int]float64{}
		for tok, p := range dist {
			val, ok := TokenValue(tok)
			if !ok {
				continue
			}
			if existing, found := massPerValue[val]; !found || p > existing {
				massPerValue[val] = p
			}
		}
		massSum := 0.0
		for _, p := range massPerValue {
			massSum += p
		}
		if massSum >= 0.05 {
			expectedNum := 0.0
			for v, p := range massPerValue {
				expectedNum += float64(v) * p
			}
			e := expectedNum / massSum
			normalizedMean := (e - ValueMin) / (ValueMax - ValueMin)
			variance := 0.0
			for v, p := range massPerValue {
				normalized := (float64(v) - ValueMin) / (ValueMax - ValueMin)
				delta := normalized - normalizedMean
				variance += p * delta * delta
			}
			variance /= massSum
			return ExtractResult{
				Score:            normalizedMean,
				Variance:         variance,
				Extracted:        true,
				Mass:             massSum,
				FromDistribution: true,
			}
		}
	}
	tagName := strings.Trim(tag, "<>")
	cleaned := StripCodeFences(StripThinkBlocks(rawText))
	rx := regexp.MustCompile(`(?i)<` + regexp.QuoteMeta(tagName) + `>\s*([A-Ta-t])\s*</` + regexp.QuoteMeta(tagName) + `>`)
	matches := rx.FindAllStringSubmatch(cleaned, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		if v, ok := TokenValue(last[1]); ok {
			return ExtractResult{
				Score:     (float64(v) - ValueMin) / (ValueMax - ValueMin),
				Extracted: true,
				Mass:      1,
			}
		}
	}
	return ExtractResult{Score: 0.5, Extracted: false}
}
