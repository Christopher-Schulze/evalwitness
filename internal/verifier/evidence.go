package verifier

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

const (
	ScoreEvidenceSchemaVersion = "evalwitness.score-evidence.v1"
	StrictPolicyVersion        = "evalwitness.strict-score-policy.v1"
	MinimumVerifierTopK        = 20
	MinimumValidScoreMass      = 0.05
	probabilityTolerance       = 1e-6
)

type ExtractionMode string

const (
	ExtractionModeVerifier ExtractionMode = "verifier"
	ExtractionModeJudge    ExtractionMode = "judge"
	ExtractionModeMixed    ExtractionMode = "mixed"
)

type AlignmentStatus string

const (
	AlignmentExact     AlignmentStatus = "exact"
	AlignmentAmbiguous AlignmentStatus = "ambiguous"
	AlignmentMissing   AlignmentStatus = "missing"
	AlignmentTruncated AlignmentStatus = "truncated"
	AlignmentInvalid   AlignmentStatus = "invalid"
)

type DegradationCode string

const (
	DegradationMissingLogprobs      DegradationCode = "missing_logprobs"
	DegradationDegenerateLogprobs   DegradationCode = "degenerate_logprobs"
	DegradationInsufficientTopK     DegradationCode = "insufficient_top_k"
	DegradationLowScoreMass         DegradationCode = "low_score_mass"
	DegradationMissingTag           DegradationCode = "missing_tag"
	DegradationDuplicateTag         DegradationCode = "duplicate_tag"
	DegradationResponseTruncated    DegradationCode = "response_truncated"
	DegradationInvalidScoreText     DegradationCode = "invalid_score_text"
	DegradationTokenFragment        DegradationCode = "token_fragment"
	DegradationMultiTokenScore      DegradationCode = "multi_token_score"
	DegradationInvalidScoreToken    DegradationCode = "invalid_score_token"
	DegradationDuplicateForm        DegradationCode = "duplicate_form"
	DegradationDuplicateAlternative DegradationCode = "duplicate_alternative"
	DegradationNonFiniteLogprob     DegradationCode = "non_finite_logprob"
	DegradationVisibleMassOverflow  DegradationCode = "visible_mass_overflow"
	DegradationTextOnlyJudge        DegradationCode = "text_only_judge"
)

type Degradation struct {
	Code   DegradationCode `json:"code"`
	Detail string          `json:"detail,omitempty"`
}

type AlignedTokenPosition struct {
	TokenIndex int `json:"token_index"`
	TokenByte  int `json:"token_byte"`
	StreamByte int `json:"stream_byte"`
	RawByte    int `json:"raw_byte"`
}

type VisibleAlternative struct {
	Rank                 int      `json:"rank"`
	Token                string   `json:"token"`
	Logprob              string   `json:"logprob"`
	Probability          *float64 `json:"probability,omitempty"`
	Chosen               bool     `json:"chosen,omitempty"`
	CanonicalLetter      string   `json:"canonical_letter,omitempty"`
	CanonicalValue       *float64 `json:"canonical_value,omitempty"`
	DuplicateOfRank      *int     `json:"duplicate_of_rank,omitempty"`
	CanonicalSourceRanks []int    `json:"canonical_source_ranks,omitempty"`
	Diagnostic           string   `json:"diagnostic,omitempty"`
}

type ScoreSupport struct {
	Letter      string   `json:"letter"`
	Value       float64  `json:"value"`
	Probability float64  `json:"probability"`
	SourceRanks []int    `json:"source_ranks"`
	SourceForms []string `json:"source_forms"`
}

type ScoreEvidence struct {
	SchemaVersion             string                `json:"schema_version"`
	PolicyVersion             string                `json:"policy_version"`
	Tag                       string                `json:"tag"`
	ExtractionMode            ExtractionMode        `json:"extraction_mode"`
	AlignmentStatus           AlignmentStatus       `json:"alignment_status"`
	AlignedPosition           *AlignedTokenPosition `json:"aligned_position,omitempty"`
	RequestedTopK             int                   `json:"requested_top_k"`
	ReturnedTopK              int                   `json:"returned_top_k"`
	VisibleProbabilityMass    float64               `json:"visible_probability_mass"`
	ValidScoreMass            float64               `json:"valid_score_mass"`
	UnobservedProbabilityMass float64               `json:"unobserved_probability_mass"`
	Support                   []ScoreSupport        `json:"score_support"`
	Alternatives              []VisibleAlternative  `json:"visible_alternatives"`
	ConditionalExpectedScore  *float64              `json:"conditional_expected_score,omitempty"`
	ConditionalVariance       *float64              `json:"conditional_variance,omitempty"`
	Extracted                 bool                  `json:"extracted"`
	Degradations              []Degradation         `json:"degradations"`
}

type EvidenceError struct {
	Tag      string
	Evidence ScoreEvidence
	Reason   string
}

func (e *EvidenceError) Error() string {
	codes := make([]string, 0, len(e.Evidence.Degradations))
	for _, degradation := range e.Evidence.Degradations {
		codes = append(codes, string(degradation.Code))
	}
	reason := strings.Join(codes, ", ")
	if e.Reason != "" {
		reason = e.Reason
	}
	if reason == "" {
		reason = "evidence invariant violation"
	}
	return fmt.Sprintf("score evidence rejected %s: %s", e.Tag, reason)
}

type evidenceCandidate struct {
	rawScoreByte int
	rawLetter    byte
	streamByte   int
	tokenIndex   int
	tokenByte    int
}

type rawScoreCandidate struct {
	scoreByte int
	letter    byte
}

type tokenSpan struct {
	tokenIndex  int
	streamStart int
	streamEnd   int
	rawStart    int
	rawEnd      int
	evidence    provider.TokenEvidence
}

func ExtractAllScoreEvidence(request provider.RequestEnvelope, response provider.ResponseRecord, mode ExtractionMode) map[string]ScoreEvidence {
	out := make(map[string]ScoreEvidence, len(request.ScoreTags))
	for _, tag := range request.ScoreTags {
		out[tag] = ExtractScoreEvidence(request.TopLogprobs, response, tag, mode)
	}
	return out
}

func ExtractScoreEvidence(requestedTopK int, response provider.ResponseRecord, tag string, mode ExtractionMode) ScoreEvidence {
	evidence := ScoreEvidence{
		SchemaVersion:  ScoreEvidenceSchemaVersion,
		PolicyVersion:  StrictPolicyVersion,
		Tag:            tag,
		ExtractionMode: mode,
		RequestedTopK:  requestedTopK,
		Support:        []ScoreSupport{},
		Alternatives:   []VisibleAlternative{},
		Degradations:   []Degradation{},
	}
	if mode == ExtractionModeJudge {
		return extractJudgeEvidence(evidence, response.RawText)
	}
	return extractVerifierEvidence(evidence, response)
}

func extractJudgeEvidence(evidence ScoreEvidence, rawText string) ScoreEvidence {
	evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationTextOnlyJudge})
	candidates := rawCandidates(rawText, evidence.Tag)
	switch len(candidates) {
	case 0:
		evidence.AlignmentStatus = AlignmentMissing
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationMissingTag})
		return evidence
	case 1:
		evidence.AlignmentStatus = AlignmentExact
	default:
		evidence.AlignmentStatus = AlignmentAmbiguous
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationDuplicateTag, Detail: fmt.Sprintf("%d complete tags", len(candidates))})
		return evidence
	}
	value, ok := TokenValue(string(candidates[0].letter))
	if !ok {
		evidence.AlignmentStatus = AlignmentInvalid
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationInvalidScoreText})
		return evidence
	}
	normalized := normalizeValue(value)
	variance := 0.0
	evidence.ConditionalExpectedScore = &normalized
	evidence.ConditionalVariance = &variance
	evidence.Extracted = true
	return evidence
}

func extractVerifierEvidence(evidence ScoreEvidence, response provider.ResponseRecord) ScoreEvidence {
	if !response.HasLogprobs || len(response.OrderedTokenEvidence) == 0 {
		evidence.AlignmentStatus = AlignmentMissing
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationMissingLogprobs})
		return evidence
	}
	if response.DegenerateLogprobs {
		evidence.AlignmentStatus = AlignmentInvalid
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationDegenerateLogprobs})
		return evidence
	}
	if evidence.RequestedTopK < MinimumVerifierTopK {
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationInsufficientTopK, Detail: fmt.Sprintf("requested %d, require %d", evidence.RequestedTopK, MinimumVerifierTopK)})
	}

	candidates := alignEvidence(response.RawText, response.OrderedTokenEvidence, evidence.Tag)
	switch len(candidates) {
	case 0:
		if responseLooksTruncated(response.FinishReason) {
			evidence.AlignmentStatus = AlignmentTruncated
			evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationResponseTruncated, Detail: response.FinishReason})
		} else {
			evidence.AlignmentStatus = AlignmentMissing
			evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationMissingTag})
		}
		return evidence
	case 1:
		evidence.AlignmentStatus = AlignmentExact
	default:
		evidence.AlignmentStatus = AlignmentAmbiguous
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationDuplicateTag, Detail: fmt.Sprintf("%d alignable tags", len(candidates))})
		return evidence
	}

	candidate := candidates[0]
	evidence.AlignedPosition = &AlignedTokenPosition{
		TokenIndex: candidate.tokenIndex,
		TokenByte:  candidate.tokenByte,
		StreamByte: candidate.streamByte,
		RawByte:    candidate.rawScoreByte,
	}
	token := response.OrderedTokenEvidence[candidate.tokenIndex]
	evidence.ReturnedTopK = len(token.TopAlternatives)
	if evidence.ReturnedTopK < MinimumVerifierTopK {
		evidence.Degradations = append(evidence.Degradations, Degradation{Code: DegradationInsufficientTopK, Detail: fmt.Sprintf("returned %d, require %d", evidence.ReturnedTopK, MinimumVerifierTopK)})
	}

	type rawAlternative struct {
		rank    int
		token   string
		logprob string
		chosen  bool
	}
	raw := make([]rawAlternative, 0, len(token.TopAlternatives)+1)
	for index, alternative := range token.TopAlternatives {
		raw = append(raw, rawAlternative{rank: index + 1, token: alternative.Token, logprob: alternative.Logprob})
	}
	chosenPresent := false
	for index := range raw {
		if raw[index].token == token.Token {
			raw[index].chosen = true
			chosenPresent = true
		}
	}
	if !chosenPresent {
		raw = append(raw, rawAlternative{rank: 0, token: token.Token, logprob: token.Logprob, chosen: true})
	}

	seenRaw := map[string]int{}
	rawMaxProbability := map[string]float64{}
	type supportAccumulator struct {
		letter      string
		value       float64
		probability float64
		ranks       []int
		forms       []string
	}
	supportByLetter := map[string]*supportAccumulator{}
	blockingNumeric := false
	blockingStructure := false
	for _, alternative := range raw {
		visible := VisibleAlternative{Rank: alternative.rank, Token: alternative.token, Logprob: alternative.logprob, Chosen: alternative.chosen}
		probability, err := strconv.ParseFloat(alternative.logprob, 64)
		if err == nil {
			probability = math.Exp(probability)
		}
		if err != nil || math.IsNaN(probability) || math.IsInf(probability, 0) || probability < 0 {
			visible.Diagnostic = string(DegradationNonFiniteLogprob)
			evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationNonFiniteLogprob, Detail: fmt.Sprintf("rank %d", alternative.rank)})
			blockingNumeric = true
			evidence.Alternatives = append(evidence.Alternatives, visible)
			continue
		}
		visible.Probability = floatPointer(probability)
		if existingRank, exists := seenRaw[alternative.token]; exists {
			visible.DuplicateOfRank = intPointer(existingRank)
			visible.Diagnostic = string(DegradationDuplicateAlternative)
			evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationDuplicateAlternative, Detail: strconv.Quote(alternative.token)})
			blockingStructure = true
		} else {
			seenRaw[alternative.token] = alternative.rank
		}
		if probability > rawMaxProbability[alternative.token] {
			rawMaxProbability[alternative.token] = probability
		}

		letter, value, diagnostic := canonicalScoreAtOffset(alternative.token, candidate.tokenByte)
		if diagnostic != "" {
			visible.Diagnostic = diagnostic
			switch diagnostic {
			case string(DegradationTokenFragment):
				evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationTokenFragment})
			case string(DegradationMultiTokenScore):
				evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationMultiTokenScore})
			default:
				evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationInvalidScoreToken})
			}
			evidence.Alternatives = append(evidence.Alternatives, visible)
			continue
		}
		visible.CanonicalLetter = letter
		visible.CanonicalValue = floatPointer(value)
		accumulator := supportByLetter[letter]
		if accumulator == nil {
			accumulator = &supportAccumulator{letter: letter, value: value}
			supportByLetter[letter] = accumulator
		}
		accumulator.ranks = append(accumulator.ranks, alternative.rank)
		accumulator.forms = appendUniqueString(accumulator.forms, alternative.token)
		if probability > accumulator.probability {
			accumulator.probability = probability
		}
		visible.CanonicalSourceRanks = append([]int(nil), accumulator.ranks...)
		if len(accumulator.ranks) > 1 {
			evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationDuplicateForm, Detail: letter})
		}
		evidence.Alternatives = append(evidence.Alternatives, visible)
	}

	letters := make([]string, 0, len(supportByLetter))
	for letter := range supportByLetter {
		letters = append(letters, letter)
	}
	sort.Slice(letters, func(i, j int) bool {
		left, _ := TokenValue(letters[i])
		right, _ := TokenValue(letters[j])
		return left > right
	})
	for _, letter := range letters {
		accumulator := supportByLetter[letter]
		evidence.ValidScoreMass += accumulator.probability
		evidence.Support = append(evidence.Support, ScoreSupport{
			Letter:      accumulator.letter,
			Value:       accumulator.value,
			Probability: accumulator.probability,
			SourceRanks: append([]int(nil), accumulator.ranks...),
			SourceForms: append([]string(nil), accumulator.forms...),
		})
	}
	rawTokens := make([]string, 0, len(rawMaxProbability))
	for token := range rawMaxProbability {
		rawTokens = append(rawTokens, token)
	}
	sort.Strings(rawTokens)
	for _, token := range rawTokens {
		evidence.VisibleProbabilityMass += rawMaxProbability[token]
	}
	for index := range evidence.Alternatives {
		letter := evidence.Alternatives[index].CanonicalLetter
		if letter == "" {
			continue
		}
		evidence.Alternatives[index].CanonicalSourceRanks = append([]int(nil), supportByLetter[letter].ranks...)
	}
	if evidence.VisibleProbabilityMass > 1+probabilityTolerance {
		evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationVisibleMassOverflow, Detail: strconv.FormatFloat(evidence.VisibleProbabilityMass, 'g', -1, 64)})
	}
	evidence.UnobservedProbabilityMass = clampProbability(1 - evidence.VisibleProbabilityMass)
	if evidence.ValidScoreMass < MinimumValidScoreMass {
		evidence.Degradations = appendUniqueDegradation(evidence.Degradations, Degradation{Code: DegradationLowScoreMass, Detail: strconv.FormatFloat(evidence.ValidScoreMass, 'g', -1, 64)})
	}
	if evidence.ValidScoreMass > 0 {
		mean := 0.0
		for _, support := range evidence.Support {
			mean += support.Value * support.Probability
		}
		mean /= evidence.ValidScoreMass
		variance := 0.0
		for _, support := range evidence.Support {
			delta := support.Value - mean
			variance += support.Probability * delta * delta
		}
		variance /= evidence.ValidScoreMass
		evidence.ConditionalExpectedScore = &mean
		evidence.ConditionalVariance = &variance
	}
	evidence.Extracted = evidence.AlignmentStatus == AlignmentExact &&
		evidence.RequestedTopK >= MinimumVerifierTopK &&
		evidence.ReturnedTopK >= MinimumVerifierTopK &&
		evidence.ValidScoreMass >= MinimumValidScoreMass &&
		!blockingNumeric &&
		!blockingStructure &&
		evidence.VisibleProbabilityMass <= 1+probabilityTolerance
	return evidence
}

func ValidateStrictEvidence(evidence map[string]ScoreEvidence) error {
	if len(evidence) == 0 {
		return &EvidenceError{Reason: "score evidence set is empty"}
	}
	tags := make([]string, 0, len(evidence))
	for tag := range evidence {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		item := evidence[tag]
		if item.Tag != tag {
			return &EvidenceError{Tag: tag, Evidence: item, Reason: "evidence tag does not match map key"}
		}
		if item.SchemaVersion != ScoreEvidenceSchemaVersion || item.PolicyVersion != StrictPolicyVersion {
			return &EvidenceError{Tag: tag, Evidence: item, Reason: "schema or policy version mismatch"}
		}
		if item.ExtractionMode != ExtractionModeVerifier {
			return &EvidenceError{Tag: tag, Evidence: item, Reason: "strict verifier requires verifier extraction mode"}
		}
		if !validStrictVerifierInvariants(item) {
			return &EvidenceError{Tag: tag, Evidence: item}
		}
	}
	return nil
}

func ValidateJudgeEvidence(evidence map[string]ScoreEvidence) error {
	if len(evidence) == 0 {
		return &EvidenceError{Reason: "score evidence set is empty"}
	}
	tags := make([]string, 0, len(evidence))
	for tag := range evidence {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		item := evidence[tag]
		if item.Tag != tag {
			return &EvidenceError{Tag: tag, Evidence: item, Reason: "evidence tag does not match map key"}
		}
		if item.SchemaVersion != ScoreEvidenceSchemaVersion || item.PolicyVersion != StrictPolicyVersion {
			return &EvidenceError{Tag: tag, Evidence: item, Reason: "schema or policy version mismatch"}
		}
		if item.ExtractionMode != ExtractionModeJudge {
			return &EvidenceError{Tag: tag, Evidence: item, Reason: "judge requires explicit judge extraction mode"}
		}
		if !item.Extracted || item.AlignmentStatus != AlignmentExact || item.AlignedPosition != nil || item.RequestedTopK != 0 || item.ReturnedTopK != 0 ||
			item.VisibleProbabilityMass != 0 || item.ValidScoreMass != 0 || item.UnobservedProbabilityMass != 0 || len(item.Support) != 0 || len(item.Alternatives) != 0 ||
			item.ConditionalExpectedScore == nil || item.ConditionalVariance == nil || !validProbability(*item.ConditionalExpectedScore) || *item.ConditionalVariance != 0 ||
			!hasDegradationCode(item.Degradations, DegradationTextOnlyJudge) {
			return &EvidenceError{Tag: tag, Evidence: item}
		}
	}
	return nil
}

func validProbability(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1+probabilityTolerance
}

func validStrictVerifierInvariants(item ScoreEvidence) bool {
	if !item.Extracted || item.AlignmentStatus != AlignmentExact || item.RequestedTopK < MinimumVerifierTopK || item.ReturnedTopK < MinimumVerifierTopK ||
		item.ValidScoreMass < MinimumValidScoreMass || item.ConditionalExpectedScore == nil || item.ConditionalVariance == nil ||
		!validProbability(item.VisibleProbabilityMass) || !validProbability(item.ValidScoreMass) || !validProbability(item.UnobservedProbabilityMass) {
		return false
	}
	if item.ValidScoreMass > item.VisibleProbabilityMass+probabilityTolerance || math.Abs(item.VisibleProbabilityMass+item.UnobservedProbabilityMass-1) > probabilityTolerance {
		return false
	}
	if !validProbability(*item.ConditionalExpectedScore) || math.IsNaN(*item.ConditionalVariance) || math.IsInf(*item.ConditionalVariance, 0) || *item.ConditionalVariance < 0 {
		return false
	}
	supportMass := 0.0
	weightedScore := 0.0
	seenLetters := map[string]bool{}
	for _, support := range item.Support {
		value, ok := TokenValue(support.Letter)
		if !ok || support.Letter != strings.ToUpper(support.Letter) || seenLetters[support.Letter] || !validProbability(support.Probability) ||
			!validProbability(support.Value) || math.Abs(support.Value-normalizeValue(value)) > probabilityTolerance {
			return false
		}
		seenLetters[support.Letter] = true
		supportMass += support.Probability
		weightedScore += support.Probability * support.Value
	}
	if math.Abs(supportMass-item.ValidScoreMass) > probabilityTolerance || supportMass <= 0 {
		return false
	}
	mean := weightedScore / supportMass
	variance := 0.0
	for _, support := range item.Support {
		delta := support.Value - mean
		variance += support.Probability * delta * delta
	}
	variance /= supportMass
	return math.Abs(mean-*item.ConditionalExpectedScore) <= probabilityTolerance && math.Abs(variance-*item.ConditionalVariance) <= probabilityTolerance
}

func hasDegradationCode(degradations []Degradation, code DegradationCode) bool {
	for _, degradation := range degradations {
		if degradation.Code == code {
			return true
		}
	}
	return false
}

func rawCandidates(rawText, tag string) []rawScoreCandidate {
	tagName := strings.Trim(tag, "<>")
	pattern := regexp.MustCompile(`(?i)<` + regexp.QuoteMeta(tagName) + `>\s*([A-Ta-t])\s*</` + regexp.QuoteMeta(tagName) + `>`)
	matches := pattern.FindAllStringSubmatchIndex(rawText, -1)
	out := make([]rawScoreCandidate, 0, len(matches))
	for _, match := range matches {
		out = append(out, rawScoreCandidate{scoreByte: match[2], letter: rawText[match[2]]})
	}
	return out
}

func alignEvidence(rawText string, tokens []provider.TokenEvidence, tag string) []evidenceCandidate {
	raw := rawCandidates(rawText, tag)
	if len(raw) == 0 {
		return nil
	}
	streamSpans, stream := buildStreamSpans(tokens)
	candidates := alignInTokenStream(stream, streamSpans, raw, tag)
	if len(candidates) > 0 {
		return candidates
	}
	return alignThroughRawText(rawText, tokens, raw)
}

func buildStreamSpans(tokens []provider.TokenEvidence) ([]tokenSpan, string) {
	spans := make([]tokenSpan, 0, len(tokens))
	var stream strings.Builder
	for tokenIndex, token := range tokens {
		start := stream.Len()
		stream.WriteString(token.Token)
		spans = append(spans, tokenSpan{tokenIndex: tokenIndex, streamStart: start, streamEnd: stream.Len(), evidence: token})
	}
	return spans, stream.String()
}

func alignInTokenStream(stream string, spans []tokenSpan, raw []rawScoreCandidate, tag string) []evidenceCandidate {
	variants := []string{tag}
	if stripped := strings.TrimPrefix(tag, "<"); stripped != tag {
		variants = append(variants, stripped)
	}
	out := []evidenceCandidate{}
	seen := map[[2]int]bool{}
	for _, variant := range variants {
		for searchFrom := 0; searchFrom < len(stream); {
			relative := strings.Index(stream[searchFrom:], variant)
			if relative < 0 {
				break
			}
			tagStart := searchFrom + relative
			scoreByte := skipWhitespace(stream, tagStart+len(variant))
			spanIndex, tokenByte, ok := spanAt(spans, scoreByte, false)
			if ok && tokenByte < len(spans[spanIndex].evidence.Token) {
				letter := spans[spanIndex].evidence.Token[tokenByte]
				if _, valid := TokenValue(string(letter)); valid && rawLetterExists(raw, letter) {
					key := [2]int{spanIndex, tokenByte}
					if !seen[key] {
						seen[key] = true
						out = append(out, evidenceCandidate{rawScoreByte: matchingRawByte(raw, letter), rawLetter: letter, streamByte: scoreByte, tokenIndex: spanIndex, tokenByte: tokenByte})
					}
				}
			}
			searchFrom = tagStart + 1
		}
	}
	return out
}

func alignThroughRawText(rawText string, tokens []provider.TokenEvidence, raw []rawScoreCandidate) []evidenceCandidate {
	spans := make([]tokenSpan, 0, len(tokens))
	cursor := 0
	streamCursor := 0
	for tokenIndex, token := range tokens {
		streamStart := streamCursor
		streamCursor += len(token.Token)
		if token.Token == "" || cursor >= len(rawText) {
			continue
		}
		relative := strings.Index(rawText[cursor:], token.Token)
		if relative < 0 {
			continue
		}
		start := cursor + relative
		end := start + len(token.Token)
		spans = append(spans, tokenSpan{tokenIndex: tokenIndex, streamStart: streamStart, streamEnd: streamCursor, rawStart: start, rawEnd: end, evidence: token})
		cursor = end
	}
	out := make([]evidenceCandidate, 0, len(raw))
	for _, candidate := range raw {
		spanIndex, tokenByte, ok := spanAt(spans, candidate.scoreByte, true)
		if !ok || tokenByte >= len(spans[spanIndex].evidence.Token) {
			continue
		}
		letter := spans[spanIndex].evidence.Token[tokenByte]
		if !strings.EqualFold(string(letter), string(candidate.letter)) {
			continue
		}
		out = append(out, evidenceCandidate{
			rawScoreByte: candidate.scoreByte,
			rawLetter:    candidate.letter,
			streamByte:   spans[spanIndex].streamStart + tokenByte,
			tokenIndex:   spans[spanIndex].tokenIndex,
			tokenByte:    tokenByte,
		})
	}
	return out
}

func spanAt(spans []tokenSpan, offset int, rawCoordinates bool) (int, int, bool) {
	for index, span := range spans {
		start, end := span.streamStart, span.streamEnd
		if rawCoordinates {
			start, end = span.rawStart, span.rawEnd
		}
		if offset >= start && offset < end {
			return index, offset - start, true
		}
	}
	return 0, 0, false
}

func canonicalScoreAtOffset(token string, offset int) (string, float64, string) {
	if offset < 0 || offset >= len(token) {
		return "", 0, string(DegradationTokenFragment)
	}
	letter := token[offset]
	value, ok := TokenValue(string(letter))
	if !ok {
		return "", 0, string(DegradationInvalidScoreToken)
	}
	trimmed := strings.TrimSpace(token)
	if len(trimmed) > 1 && !strings.Contains(trimmed, "</") {
		letters := 0
		for index := 0; index < len(trimmed); index++ {
			if _, valid := TokenValue(string(trimmed[index])); valid {
				letters++
			}
		}
		if letters > 1 {
			return "", 0, string(DegradationMultiTokenScore)
		}
	}
	canonical := strings.ToUpper(string(letter))
	return canonical, normalizeValue(value), ""
}

func normalizeValue(value int) float64 {
	return (float64(value) - ValueMin) / (ValueMax - ValueMin)
}

func responseLooksTruncated(finishReason string) bool {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "length", "max_tokens", "content_filter":
		return true
	default:
		return false
	}
}

func skipWhitespace(value string, offset int) int {
	for offset < len(value) {
		switch value[offset] {
		case ' ', '\t', '\n', '\r':
			offset++
		default:
			return offset
		}
	}
	return offset
}

func rawLetterExists(candidates []rawScoreCandidate, letter byte) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(string(candidate.letter), string(letter)) {
			return true
		}
	}
	return false
}

func matchingRawByte(candidates []rawScoreCandidate, letter byte) int {
	for _, candidate := range candidates {
		if strings.EqualFold(string(candidate.letter), string(letter)) {
			return candidate.scoreByte
		}
	}
	return -1
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueDegradation(values []Degradation, value Degradation) []Degradation {
	for _, existing := range values {
		if existing.Code == value.Code && existing.Detail == value.Detail {
			return values
		}
	}
	return append(values, value)
}

func clampProbability(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func floatPointer(value float64) *float64 { return &value }
func intPointer(value int) *int           { return &value }
