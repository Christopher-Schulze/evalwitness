package reliance

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

func TestReferenceCorpusIsDeterministicOfflineAndStrict(t *testing.T) {
	preregistration := referenceTestPreregistration(t)
	first, err := BuildReferenceCorpus(preregistration)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildReferenceCorpus(preregistration)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("reference corpus is not deterministic")
	}
	if err := first.Validate(preregistration); err != nil {
		t.Fatal(err)
	}
	if first.ProviderCalls != 0 || first.NetworkRequired || len(first.Records) != ReferenceSourceTasks*ReferenceCellsPerTask {
		t.Fatalf("offline boundary or dimensions = calls %d network %t records %d", first.ProviderCalls, first.NetworkRequired, len(first.Records))
	}

	minimumMissingMass := 1.0
	maximumMissingMass := 0.0
	decisionFlips := 0
	abstentionTransitions := 0
	balance := make(map[string]map[string]float64)
	for _, record := range first.Records {
		if err := verifier.ValidateStrictEvidence(map[string]verifier.ScoreEvidence{
			referenceScoreTag: record.BaselineEvidence,
		}); err != nil {
			t.Fatalf("baseline %s: %v", record.ObservationID, err)
		}
		if err := verifier.ValidateStrictEvidence(map[string]verifier.ScoreEvidence{
			referenceScoreTag: record.InterventionEvidence,
		}); err != nil {
			t.Fatalf("intervention %s: %v", record.ObservationID, err)
		}
		if err := verifier.ValidateScoreEvidenceComparison(record.Comparison); err != nil {
			t.Fatalf("comparison %s: %v", record.ObservationID, err)
		}
		assertReferenceOutputRoundTrip(t, record.BaselineOutput, record.BaselineEvidence)
		assertReferenceOutputRoundTrip(t, record.InterventionOutput, record.InterventionEvidence)
		missingMass := record.InterventionEvidence.UnobservedProbabilityMass
		minimumMissingMass = math.Min(minimumMissingMass, missingMass)
		maximumMissingMass = math.Max(maximumMissingMass, missingMass)
		if record.DecisionFlip {
			decisionFlips++
		}
		if record.AbstentionTransition {
			abstentionTransitions++
		}
		if balance[record.SourceTaskID] == nil {
			balance[record.SourceTaskID] = make(map[string]float64)
		}
		for _, level := range record.Levels {
			balance[record.SourceTaskID][level.FactorID] += level.Level
		}
	}
	if minimumMissingMass <= 0 || maximumMissingMass <= minimumMissingMass {
		t.Fatalf("reference missing mass range = [%.17g, %.17g]", minimumMissingMass, maximumMissingMass)
	}
	if decisionFlips == 0 || abstentionTransitions == 0 {
		t.Fatalf("reference transitions = decision %d abstention %d", decisionFlips, abstentionTransitions)
	}
	for sourceTaskID, factors := range balance {
		for factorID, sum := range factors {
			if sum != 0 {
				t.Fatalf("factor %s is not balanced in %s: %.17g", factorID, sourceTaskID, sum)
			}
		}
	}
}

func TestReferenceAnalysisRecoversPlantedMainInteractionAndNullEffects(t *testing.T) {
	preregistration := referenceTestPreregistration(t)
	corpus, err := BuildReferenceCorpus(preregistration)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := AnalyzeReferenceCorpus(corpus, preregistration)
	if err != nil {
		t.Fatal(err)
	}
	if err := analysis.Validate(corpus, preregistration); err != nil {
		t.Fatal(err)
	}
	repeated, err := AnalyzeReferenceCorpus(corpus, preregistration)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(analysis, repeated) {
		t.Fatal("reference analysis is not deterministic")
	}

	scoreFit := referenceTestOutcomeFit(t, analysis, OutcomeConditionalMean)
	validMassFit := referenceTestOutcomeFit(t, analysis, OutcomeValidMass)
	for _, planted := range corpus.PlantedEffects {
		scoreEstimate := referenceTestEstimate(t, scoreFit, planted.TermID)
		assertRecoveredEffect(t, "conditional score", planted.TermID, scoreEstimate, planted.ConditionalScoreEffect)
		validMassEstimate := referenceTestEstimate(t, validMassFit, planted.TermID)
		assertRecoveredEffect(t, "valid mass", planted.TermID, validMassEstimate, planted.ValidMassEffect)
	}
	if len(analysis.OutcomeFits) != len(preregistration.PrimaryOutcomes) {
		t.Fatalf("outcome fits = %d, want %d", len(analysis.OutcomeFits), len(preregistration.PrimaryOutcomes))
	}
	for _, outcome := range analysis.OutcomeFits {
		if outcome.Fit.Observations != ReferenceSourceTasks*ReferenceCellsPerTask ||
			outcome.Fit.Clusters != ReferenceSourceTasks || outcome.Fit.Rank != len(corpus.PlantedEffects)+1 ||
			outcome.Fit.FamilySize != preregistration.Multiplicity.FamilySize {
			t.Fatalf("outcome %s fit dimensions = %+v", outcome.OutcomeID, outcome.Fit)
		}
	}
}

func TestReferenceWalshDesignProtectsMainAndDeclaredInteractionEffects(t *testing.T) {
	preregistration := referenceTestPreregistration(t)
	masks := canonicalReferenceMasks()
	maskByFactor := make(map[string]uint64, len(masks))
	pairCounts := make(map[uint64]int)
	for _, factor := range masks {
		maskByFactor[factor.FactorID] = factor.Mask
	}
	for left := 0; left < len(masks); left++ {
		for right := left + 1; right < len(masks); right++ {
			pairCounts[masks[left].Mask^masks[right].Mask]++
		}
	}
	for _, factor := range masks {
		if pairCounts[factor.Mask] != 0 {
			t.Fatalf("main factor %s aliases with a two-factor interaction", factor.FactorID)
		}
	}
	for _, interaction := range preregistration.Interactions {
		mask := uint64(0)
		for _, factorID := range interaction.Terms {
			factorMask, found := maskByFactor[factorID]
			if !found {
				t.Fatalf("interaction %s references absent factor %s", interaction.InteractionID, factorID)
			}
			mask ^= factorMask
		}
		if pairCounts[mask] != 1 {
			t.Fatalf("declared interaction %s has %d two-factor aliases, want exactly its own pair", interaction.InteractionID, pairCounts[mask])
		}
	}
}

func TestReferenceArtifactsRejectDigestPreservingTampering(t *testing.T) {
	preregistration := referenceTestPreregistration(t)
	corpus, err := BuildReferenceCorpus(preregistration)
	if err != nil {
		t.Fatal(err)
	}
	tamperedCorpus := cloneReferenceCorpus(t, corpus)
	tamperedCorpus.Records[0].InterventionEvidence.ValidScoreMass += 0.01
	tamperedCorpus.Digest, err = referenceCorpusDigest(tamperedCorpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := tamperedCorpus.Validate(preregistration); err == nil {
		t.Fatal("reference corpus accepted digest-preserving evidence tampering")
	}

	analysis, err := AnalyzeReferenceCorpus(corpus, preregistration)
	if err != nil {
		t.Fatal(err)
	}
	tamperedAnalysis := analysis
	tamperedAnalysis.OutcomeFits = append([]ReferenceOutcomeFit(nil), analysis.OutcomeFits...)
	tamperedAnalysis.OutcomeFits[0].Fit.Estimates = append([]stats.FactorialEstimate(nil), analysis.OutcomeFits[0].Fit.Estimates...)
	tamperedAnalysis.OutcomeFits[0].Fit.Estimates[0].Estimate += 0.01
	tamperedAnalysis.Digest, err = referenceAnalysisDigest(tamperedAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	if err := tamperedAnalysis.Validate(corpus, preregistration); err == nil {
		t.Fatal("reference analysis accepted digest-preserving estimate tampering")
	}
}

func TestReferenceAdapterRejectsInvalidTargetsAndClassifiesTopKTruncation(t *testing.T) {
	invalidTargets := []struct {
		name      string
		score     float64
		validMass float64
	}{
		{name: "non-finite score", score: math.NaN(), validMass: referenceBaselineValidMass},
		{name: "score above range", score: 1.01, validMass: referenceBaselineValidMass},
		{name: "valid mass below strict minimum", score: referenceBaselineConditionalScore, validMass: 0.04},
		{name: "visible mass overflow", score: referenceBaselineConditionalScore, validMass: 0.98},
	}
	for _, target := range invalidTargets {
		t.Run(target.name, func(t *testing.T) {
			if _, _, err := referenceScoreOutput(target.score, target.validMass); err == nil {
				t.Fatal("reference adapter accepted an invalid target")
			}
		})
	}

	output, _, err := referenceScoreOutput(referenceBaselineConditionalScore, referenceBaselineValidMass)
	if err != nil {
		t.Fatal(err)
	}
	truncated := output
	truncated.OrderedTokenEvidence = append([]provider.TokenEvidence(nil), output.OrderedTokenEvidence...)
	truncated.OrderedTokenEvidence[1].TopAlternatives = append(
		[]provider.TokenAlternative(nil),
		output.OrderedTokenEvidence[1].TopAlternatives[:verifier.MinimumVerifierTopK-1]...,
	)
	truncated.ObservedTopLogprobs = verifier.MinimumVerifierTopK - 1
	evidence := verifier.ExtractScoreEvidence(
		verifier.MinimumVerifierTopK,
		provider.ResponseRecord{
			RawText: truncated.RawText, HasLogprobs: truncated.HasLogprobs,
			ObservedTopLogprobs:  truncated.ObservedTopLogprobs,
			OrderedTokenEvidence: truncated.OrderedTokenEvidence,
		},
		referenceScoreTag,
		verifier.ExtractionModeVerifier,
	)
	if evidence.Extracted || !referenceTestHasDegradation(evidence, verifier.DegradationInsufficientTopK) {
		t.Fatalf("top-k truncation classification = extracted %t degradations %+v", evidence.Extracted, evidence.Degradations)
	}
	if err := verifier.ValidateStrictEvidence(map[string]verifier.ScoreEvidence{referenceScoreTag: evidence}); err == nil {
		t.Fatal("strict evidence policy accepted the truncated reference output")
	}
}

func assertReferenceOutputRoundTrip(
	t *testing.T,
	output ReferenceAdapterOutput,
	want verifier.ScoreEvidence,
) {
	t.Helper()
	digest, err := referenceAdapterOutputDigest(output)
	if err != nil {
		t.Fatal(err)
	}
	if output.Digest != digest {
		t.Fatalf("reference adapter output digest = %s, want %s", output.Digest, digest)
	}
	got := verifier.ExtractScoreEvidence(
		verifier.MinimumVerifierTopK,
		provider.ResponseRecord{
			RawText: output.RawText, HasLogprobs: output.HasLogprobs,
			ObservedTopLogprobs: output.ObservedTopLogprobs, OrderedTokenEvidence: output.OrderedTokenEvidence,
		},
		referenceScoreTag,
		verifier.ExtractionModeVerifier,
	)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reference adapter output does not reproduce score evidence\ngot=%+v\nwant=%+v", got, want)
	}
}

func assertRecoveredEffect(
	t *testing.T,
	outcome string,
	termID string,
	estimate stats.FactorialEstimate,
	want float64,
) {
	t.Helper()
	if math.Abs(estimate.Estimate-want) > 1e-12 {
		t.Fatalf("%s term %s = %.17g, want %.17g", outcome, termID, estimate.Estimate, want)
	}
	if want != 0 && estimate.StandardError <= 0 {
		t.Fatalf("%s term %s has no cluster uncertainty: %+v", outcome, termID, estimate)
	}
	if want < estimate.Lower-1e-12 || want > estimate.Upper+1e-12 {
		t.Fatalf("%s term %s planted effect %.17g is outside [%.17g, %.17g]", outcome, termID, want, estimate.Lower, estimate.Upper)
	}
	if !estimate.FamilyAdjusted {
		t.Fatalf("%s term %s lacks multiplicity adjustment", outcome, termID)
	}
}

func referenceTestPreregistration(t *testing.T) Preregistration {
	t.Helper()
	ontology, err := FrozenOntology()
	if err != nil {
		t.Fatal(err)
	}
	estimands, err := FrozenEstimands()
	if err != nil {
		t.Fatal(err)
	}
	preregistration, err := FrozenPreregistration(ontology, estimands)
	if err != nil {
		t.Fatal(err)
	}
	return preregistration
}

func referenceTestOutcomeFit(
	t *testing.T,
	analysis ReferenceAnalysis,
	outcomeID OutcomeID,
) stats.ClusteredFactorialFit {
	t.Helper()
	for _, outcome := range analysis.OutcomeFits {
		if outcome.OutcomeID == outcomeID {
			return outcome.Fit
		}
	}
	t.Fatalf("outcome %s is absent", outcomeID)
	return stats.ClusteredFactorialFit{}
}

func referenceTestEstimate(
	t *testing.T,
	fit stats.ClusteredFactorialFit,
	termID string,
) stats.FactorialEstimate {
	t.Helper()
	for _, estimate := range fit.Estimates {
		if estimate.TermID == termID {
			return estimate
		}
	}
	t.Fatalf("term %s is absent", termID)
	return stats.FactorialEstimate{}
}

func cloneReferenceCorpus(t *testing.T, value ReferenceCorpus) ReferenceCorpus {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var cloned ReferenceCorpus
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func referenceTestHasDegradation(evidence verifier.ScoreEvidence, code verifier.DegradationCode) bool {
	for _, degradation := range evidence.Degradations {
		if degradation.Code == code {
			return true
		}
	}
	return false
}
