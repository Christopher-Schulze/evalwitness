package verifier

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
)

type probabilityAlternative struct {
	token       string
	probability float64
}

func scoreEvidenceResponse(rawText, tag, chosen string, alternatives []probabilityAlternative) provider.ResponseRecord {
	top := make([]provider.TokenAlternative, 0, len(alternatives))
	chosenLogprob := "-1000"
	for _, alternative := range alternatives {
		logprob := "-1000"
		if alternative.probability > 0 {
			logprob = strconv.FormatFloat(math.Log(alternative.probability), 'g', -1, 64)
		}
		if alternative.token == chosen {
			chosenLogprob = logprob
		}
		top = append(top, provider.TokenAlternative{Token: alternative.token, Logprob: logprob})
	}
	scoreByte := strings.Index(rawText, tag) + len(tag)
	return provider.ResponseRecord{
		RawText:             rawText,
		HasLogprobs:         true,
		ObservedTopLogprobs: len(top),
		OrderedTokenEvidence: []provider.TokenEvidence{
			{Position: 0, Token: rawText[:scoreByte], Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			{Position: 1, Token: chosen, Logprob: chosenLogprob, TopAlternatives: top},
			{Position: 2, Token: rawText[scoreByte+len(chosen):], Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
		},
	}
}

func twentyAlternatives(explicit ...probabilityAlternative) []probabilityAlternative {
	out := append([]probabilityAlternative(nil), explicit...)
	for len(out) < MinimumVerifierTopK {
		out = append(out, probabilityAlternative{token: "#" + strconv.Itoa(len(out)), probability: 0})
	}
	return out
}

func TestScoreEvidencePreservesMassSupportAndAlignedAlternatives(t *testing.T) {
	response := scoreEvidenceResponse(
		"analysis<score_A>A</score_A>",
		"<score_A>",
		"A",
		twentyAlternatives(
			probabilityAlternative{token: "A", probability: 0.6},
			probabilityAlternative{token: "B", probability: 0.2},
			probabilityAlternative{token: "!", probability: 0.1},
		),
	)
	evidence := ExtractScoreEvidence(20, response, "<score_A>", ExtractionModeVerifier)
	if !evidence.Extracted || evidence.AlignmentStatus != AlignmentExact {
		t.Fatalf("evidence rejected: %+v", evidence)
	}
	if evidence.AlignedPosition == nil || evidence.AlignedPosition.TokenIndex != 1 || evidence.AlignedPosition.TokenByte != 0 {
		t.Fatalf("aligned position = %+v", evidence.AlignedPosition)
	}
	if evidence.ReturnedTopK != 20 || len(evidence.Alternatives) != 20 {
		t.Fatalf("top-k = %d alternatives=%d", evidence.ReturnedTopK, len(evidence.Alternatives))
	}
	if !closeEnough(evidence.VisibleProbabilityMass, 0.9) || !closeEnough(evidence.ValidScoreMass, 0.8) || !closeEnough(evidence.UnobservedProbabilityMass, 0.1) {
		t.Fatalf("masses = visible %.17g valid %.17g unobserved %.17g", evidence.VisibleProbabilityMass, evidence.ValidScoreMass, evidence.UnobservedProbabilityMass)
	}
	wantMean := (0.6*1 + 0.2*(18.0/19.0)) / 0.8
	if evidence.ConditionalExpectedScore == nil || !closeEnough(*evidence.ConditionalExpectedScore, wantMean) {
		t.Fatalf("conditional mean = %v, want %.17g", evidence.ConditionalExpectedScore, wantMean)
	}
	if evidence.ConditionalVariance == nil || *evidence.ConditionalVariance <= 0 {
		t.Fatalf("conditional variance = %v", evidence.ConditionalVariance)
	}
}

func TestRawTextAlignmentUsesSliceIndexNotProviderPosition(t *testing.T) {
	response := scoreEvidenceResponse(
		"<score>A</score>",
		"<score>",
		"A",
		twentyAlternatives(probabilityAlternative{token: "A", probability: 0.9}),
	)
	response.OrderedTokenEvidence = response.OrderedTokenEvidence[1:2]
	response.OrderedTokenEvidence[0].Position = 41

	evidence := ExtractScoreEvidence(20, response, "<score>", ExtractionModeVerifier)
	if !evidence.Extracted || evidence.AlignedPosition == nil || evidence.AlignedPosition.TokenIndex != 0 {
		t.Fatalf("raw-text alignment used provider position as slice index: %+v", evidence)
	}
}

func TestScoreEvidenceDuplicateAliasesUseMaximumWithoutDoubleCounting(t *testing.T) {
	alternatives := twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.3},
		probabilityAlternative{token: "a", probability: 0.2},
		probabilityAlternative{token: "T", probability: 0.1},
		probabilityAlternative{token: "!", probability: 0.4},
	)
	evidence := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", alternatives), "<score>", ExtractionModeVerifier)
	if !evidence.Extracted {
		t.Fatalf("alias evidence rejected: %+v", evidence.Degradations)
	}
	if !closeEnough(evidence.ValidScoreMass, 0.4) {
		t.Fatalf("valid mass = %.17g, want max(A,a)+T = 0.4", evidence.ValidScoreMass)
	}
	if len(evidence.Support) != 2 || evidence.Support[0].Letter != "A" || len(evidence.Support[0].SourceRanks) != 2 {
		t.Fatalf("support = %+v", evidence.Support)
	}
	if !hasDegradation(evidence, DegradationDuplicateForm) {
		t.Fatalf("duplicate-form diagnostic missing: %+v", evidence.Degradations)
	}
}

func TestScoreEvidenceStrictGatesAreIndependentlyFailable(t *testing.T) {
	valid := twentyAlternatives(probabilityAlternative{token: "A", probability: 0.9}, probabilityAlternative{token: "!", probability: 0.1})
	tests := []struct {
		name      string
		response  provider.ResponseRecord
		requested int
		want      DegradationCode
	}{
		{name: "missing logprobs", response: provider.ResponseRecord{RawText: "<score>A</score>"}, requested: 20, want: DegradationMissingLogprobs},
		{name: "requested top-k", response: scoreEvidenceResponse("<score>A</score>", "<score>", "A", valid), requested: 19, want: DegradationInsufficientTopK},
		{name: "returned top-k", response: scoreEvidenceResponse("<score>A</score>", "<score>", "A", valid[:19]), requested: 20, want: DegradationInsufficientTopK},
		{name: "low score mass", response: scoreEvidenceResponse("<score>A</score>", "<score>", "A", twentyAlternatives(probabilityAlternative{token: "A", probability: 0.049}, probabilityAlternative{token: "!", probability: 0.951})), requested: 20, want: DegradationLowScoreMass},
		{name: "duplicate tag", response: scoreEvidenceResponse("<score>A</score><score>A</score>", "<score>", "A", valid), requested: 20, want: DegradationDuplicateTag},
		{name: "truncated", response: provider.ResponseRecord{RawText: "<score>", HasLogprobs: true, FinishReason: "length", OrderedTokenEvidence: []provider.TokenEvidence{{Position: 0, Token: "<score>", Logprob: "0", TopAlternatives: validAlternatives(valid)}}}, requested: 20, want: DegradationResponseTruncated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := ExtractScoreEvidence(test.requested, test.response, "<score>", ExtractionModeVerifier)
			if evidence.Extracted || !hasDegradation(evidence, test.want) {
				t.Fatalf("gate did not fail with %s: extracted=%t degradations=%+v", test.want, evidence.Extracted, evidence.Degradations)
			}
			if err := ValidateStrictEvidence(map[string]ScoreEvidence{"<score>": evidence}); err == nil {
				t.Fatal("strict policy accepted rejected evidence")
			}
		})
	}
}

func TestStrictPolicyRejectsMutatedEvidenceEvenWhenExtractedFlagSurvives(t *testing.T) {
	base := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.8},
		probabilityAlternative{token: "!", probability: 0.1},
	)), "<score>", ExtractionModeVerifier)
	mutations := []struct {
		name   string
		mutate func(*ScoreEvidence)
	}{
		{name: "mass conservation", mutate: func(value *ScoreEvidence) { value.UnobservedProbabilityMass = 0.7 }},
		{name: "support mass", mutate: func(value *ScoreEvidence) { value.Support[0].Probability /= 2 }},
		{name: "alignment", mutate: func(value *ScoreEvidence) { value.AlignmentStatus = AlignmentAmbiguous }},
		{name: "strict mode", mutate: func(value *ScoreEvidence) { value.ExtractionMode = ExtractionModeJudge }},
		{name: "conditional score", mutate: func(value *ScoreEvidence) { *value.ConditionalExpectedScore = 2 }},
		{name: "plausible conditional score", mutate: func(value *ScoreEvidence) { *value.ConditionalExpectedScore = 0.7 }},
		{name: "conditional variance", mutate: func(value *ScoreEvidence) { *value.ConditionalVariance += 0.01 }},
		{name: "support letter", mutate: func(value *ScoreEvidence) { value.Support[0].Letter = "T" }},
		{name: "tag identity", mutate: func(value *ScoreEvidence) { value.Tag = "<other>" }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			value := base
			value.Support = append([]ScoreSupport(nil), base.Support...)
			score := *base.ConditionalExpectedScore
			value.ConditionalExpectedScore = &score
			variance := *base.ConditionalVariance
			value.ConditionalVariance = &variance
			mutation.mutate(&value)
			if err := ValidateStrictEvidence(map[string]ScoreEvidence{"<score>": value}); err == nil {
				t.Fatalf("strict policy accepted %s mutation", mutation.name)
			}
		})
	}
	if err := ValidateStrictEvidence(map[string]ScoreEvidence{}); err == nil {
		t.Fatal("strict policy accepted an empty evidence set")
	}
}

func TestScoreEvidencePermutationAndMassConservationProperties(t *testing.T) {
	base := twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.4},
		probabilityAlternative{token: "F", probability: 0.25},
		probabilityAlternative{token: "T", probability: 0.1},
		probabilityAlternative{token: "!", probability: 0.15},
	)
	left := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", base), "<score>", ExtractionModeVerifier)
	reversed := append([]probabilityAlternative(nil), base...)
	slices.Reverse(reversed)
	right := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", reversed), "<score>", ExtractionModeVerifier)
	if !closeEnough(left.VisibleProbabilityMass, right.VisibleProbabilityMass) || !closeEnough(left.ValidScoreMass, right.ValidScoreMass) ||
		left.ConditionalExpectedScore == nil || right.ConditionalExpectedScore == nil || !closeEnough(*left.ConditionalExpectedScore, *right.ConditionalExpectedScore) {
		t.Fatalf("permutation changed evidence: left=%+v right=%+v", left, right)
	}
	for _, evidence := range []ScoreEvidence{left, right} {
		if !closeEnough(evidence.VisibleProbabilityMass+evidence.UnobservedProbabilityMass, 1) {
			t.Fatalf("mass is not conserved: %+v", evidence)
		}
	}
}

func TestScoreEvidenceMonotonicUnderUpwardMassShift(t *testing.T) {
	low := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>T</score>", "<score>", "T", twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.2},
		probabilityAlternative{token: "T", probability: 0.8},
	)), "<score>", ExtractionModeVerifier)
	high := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.8},
		probabilityAlternative{token: "T", probability: 0.2},
	)), "<score>", ExtractionModeVerifier)
	if low.ConditionalExpectedScore == nil || high.ConditionalExpectedScore == nil || *high.ConditionalExpectedScore <= *low.ConditionalExpectedScore {
		t.Fatalf("upward mass shift was not monotonic: low=%v high=%v", low.ConditionalExpectedScore, high.ConditionalExpectedScore)
	}
}

func TestJudgeEvidenceIsExplicitAndCannotMasqueradeAsVerifier(t *testing.T) {
	evidence := ExtractScoreEvidence(0, provider.ResponseRecord{RawText: "<score>C</score>"}, "<score>", ExtractionModeJudge)
	if !evidence.Extracted || evidence.ExtractionMode != ExtractionModeJudge || !hasDegradation(evidence, DegradationTextOnlyJudge) {
		t.Fatalf("judge evidence = %+v", evidence)
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"extraction_mode":"judge"`) || strings.Contains(string(encoded), `"valid_score_mass":1`) {
		t.Fatalf("judge evidence wire shape = %s", encoded)
	}
	set := map[string]ScoreEvidence{"<score>": evidence}
	if err := ValidateJudgeEvidence(set); err != nil {
		t.Fatalf("explicit judge evidence was rejected: %v", err)
	}
	if err := ValidateStrictEvidence(set); err == nil {
		t.Fatal("judge evidence satisfied the strict verifier policy")
	}
	tampered := evidence
	tampered.ValidScoreMass = 1
	if err := ValidateJudgeEvidence(map[string]ScoreEvidence{"<score>": tampered}); err == nil {
		t.Fatal("judge evidence carried verifier-only mass")
	}
	missing := ExtractScoreEvidence(0, provider.ResponseRecord{RawText: "no score"}, "<score>", ExtractionModeJudge)
	if err := ValidateJudgeEvidence(map[string]ScoreEvidence{"<score>": missing}); err == nil {
		t.Fatal("malformed judge output silently became a default score")
	}
}

func TestCompareScoreEvidenceDoesNotZeroFillCensoredSupport(t *testing.T) {
	left := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.5},
		probabilityAlternative{token: "B", probability: 0.2},
		probabilityAlternative{token: "!", probability: 0.1},
	)), "<score>", ExtractionModeVerifier)
	right := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>B</score>", "<score>", "B", twentyAlternatives(
		probabilityAlternative{token: "B", probability: 0.4},
		probabilityAlternative{token: "C", probability: 0.3},
		probabilityAlternative{token: "!", probability: 0.2},
	)), "<score>", ExtractionModeVerifier)
	comparison := CompareScoreEvidence(left, right)
	if !slices.Equal(comparison.CommonSupport, []string{"B"}) || !slices.Equal(comparison.SupportUnion, []string{"A", "B", "C"}) {
		t.Fatalf("support comparison = %+v", comparison)
	}
	if comparison.CommonSupportConditionalDivergence == nil || *comparison.CommonSupportConditionalDivergence != 0 {
		t.Fatalf("one-letter common support should have zero conditional divergence: %+v", comparison)
	}
	if comparison.LeftMissingTailBounds.Lower == comparison.LeftMissingTailBounds.Upper || comparison.RightMissingTailBounds.Lower == comparison.RightMissingTailBounds.Upper {
		t.Fatalf("censored tail collapsed to a point: %+v", comparison)
	}
}

func TestScoreEvidenceRelationsKeepDecisionAndEvidenceConstraintsSeparate(t *testing.T) {
	base := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.4},
		probabilityAlternative{token: "T", probability: 0.4},
		probabilityAlternative{token: "!", probability: 0.1},
	)), "<score>", ExtractionModeVerifier)
	caseAlias := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>a</score>", "<score>", "a", twentyAlternatives(
		probabilityAlternative{token: "a", probability: 0.4},
		probabilityAlternative{token: "t", probability: 0.4},
		probabilityAlternative{token: "!", probability: 0.1},
	)), "<score>", ExtractionModeVerifier)
	replacedSupport := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>F</score>", "<score>", "F", twentyAlternatives(
		probabilityAlternative{token: "F", probability: 0.4},
		probabilityAlternative{token: "O", probability: 0.4},
		probabilityAlternative{token: "!", probability: 0.1},
	)), "<score>", ExtractionModeVerifier)
	lowerMass := ExtractScoreEvidence(20, scoreEvidenceResponse("<score>A</score>", "<score>", "A", twentyAlternatives(
		probabilityAlternative{token: "A", probability: 0.2},
		probabilityAlternative{token: "T", probability: 0.2},
		probabilityAlternative{token: "!", probability: 0.1},
	)), "<score>", ExtractionModeVerifier)

	aliasComparison := CompareScoreEvidence(base, caseAlias)
	if err := ValidateScoreEvidenceComparison(aliasComparison); err != nil {
		t.Fatal(err)
	}
	if aliasComparison.SupportJaccard != 1 || aliasComparison.ConditionalScoreMovement == nil || !closeEnough(*aliasComparison.ConditionalScoreMovement, 0) ||
		aliasComparison.ConditionalVarianceMovement == nil || !closeEnough(*aliasComparison.ConditionalVarianceMovement, 0) {
		t.Fatalf("alphabet case remapping changed canonical evidence: %+v", aliasComparison)
	}
	replacementComparison := CompareScoreEvidence(base, replacedSupport)
	if replacementComparison.SupportJaccard != 0 || replacementComparison.ConditionalScoreMovement == nil || !closeEnough(*replacementComparison.ConditionalScoreMovement, 0) {
		t.Fatalf("stable conditional decision hid support replacement: %+v", replacementComparison)
	}
	massComparison := CompareScoreEvidence(base, lowerMass)
	if !closeEnough(massComparison.ValidScoreMassMovement, -0.4) || massComparison.ConditionalScoreMovement == nil || !closeEnough(*massComparison.ConditionalScoreMovement, 0) {
		t.Fatalf("stable conditional decision hid mass shift: %+v", massComparison)
	}
	if massComparison.ConditionalVarianceMovement == nil || !closeEnough(*massComparison.ConditionalVarianceMovement, 0) {
		t.Fatalf("stable conditional evidence introduced variance movement: %+v", massComparison)
	}
	if !massComparison.MissingTailIntervalsOverlap || lowerMass.UnobservedProbabilityMass <= base.UnobservedProbabilityMass {
		t.Fatalf("missing-tail ambiguity was collapsed: %+v", massComparison)
	}
	tamperedComparison := aliasComparison
	tamperedComparison.SupportJaccard = 0.5
	if err := ValidateScoreEvidenceComparison(tamperedComparison); err == nil {
		t.Fatal("score-evidence comparison accepted a fabricated support metric")
	}
}

func validAlternatives(alternatives []probabilityAlternative) []provider.TokenAlternative {
	out := make([]provider.TokenAlternative, len(alternatives))
	for index, alternative := range alternatives {
		logprob := "-1000"
		if alternative.probability > 0 {
			logprob = strconv.FormatFloat(math.Log(alternative.probability), 'g', -1, 64)
		}
		out[index] = provider.TokenAlternative{Token: alternative.token, Logprob: logprob}
	}
	return out
}

func hasDegradation(evidence ScoreEvidence, code DegradationCode) bool {
	for _, degradation := range evidence.Degradations {
		if degradation.Code == code {
			return true
		}
	}
	return false
}

func closeEnough(left, right float64) bool {
	return math.Abs(left-right) <= 1e-12
}
