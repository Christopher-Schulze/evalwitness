package replay

import (
	"math"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func replayTokenEvidence(tags []string, rawText string, distributions map[string]map[string]float64) []provider.TokenEvidence {
	var evidence []provider.TokenEvidence
	cursor := 0
	appendToken := func(token, logprob string, alternatives []provider.TokenAlternative) {
		if token == "" {
			return
		}
		evidence = append(evidence, provider.TokenEvidence{Position: len(evidence), Token: token, Logprob: logprob, TopAlternatives: alternatives})
	}
	for _, tag := range tags {
		relative := strings.Index(rawText[cursor:], tag)
		if relative < 0 {
			continue
		}
		scoreStart := cursor + relative + len(tag)
		for scoreStart < len(rawText) && strings.ContainsRune(" \t\n\r", rune(rawText[scoreStart])) {
			scoreStart++
		}
		if scoreStart >= len(rawText) {
			continue
		}
		if _, ok := verifier.TokenValue(string(rawText[scoreStart])); !ok {
			continue
		}
		appendToken(rawText[cursor:scoreStart], "0", nil)
		chosen := string(rawText[scoreStart])
		alternatives, chosenLogprob := replayAlternatives(chosen, distributions[tag])
		appendToken(chosen, chosenLogprob, alternatives)
		cursor = scoreStart + 1
	}
	appendToken(rawText[cursor:], "0", nil)
	return evidence
}

func replayAlternatives(chosen string, distribution map[string]float64) ([]provider.TokenAlternative, string) {
	ordered := []string{chosen}
	for letter := 'A'; letter <= 'T'; letter++ {
		token := string(letter)
		if token == chosen {
			continue
		}
		if _, ok := distribution[token]; ok {
			ordered = append(ordered, token)
		}
	}
	for len(ordered) < 20 {
		ordered = append(ordered, "#"+strconv.Itoa(len(ordered)))
	}
	alternatives := make([]provider.TokenAlternative, 0, 20)
	chosenLogprob := "-1000"
	for _, token := range ordered[:20] {
		probability := distribution[token]
		logprob := "-1000"
		if probability > 0 {
			logprob = strconv.FormatFloat(math.Log(probability), 'g', -1, 64)
		}
		if token == chosen {
			chosenLogprob = logprob
		}
		alternatives = append(alternatives, provider.TokenAlternative{Token: token, Logprob: logprob})
	}
	return alternatives, chosenLogprob
}
