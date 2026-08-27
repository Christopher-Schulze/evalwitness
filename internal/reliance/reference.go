package reliance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"slices"
	"sort"
	"strconv"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
	protocolkit "github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	referenceBaselineConditionalScore = 0.50
	referenceBaselineValidMass        = 0.66
	referenceInvalidVisibleMass       = 0.03
	referenceAbstentionMassThreshold  = 0.55
	referenceSelectionScoreThreshold  = 0.50
	referenceTaskEffectStep           = 0.01
)

type referenceTermEffect struct {
	Term                   stats.FactorialTerm
	ConditionalScoreEffect float64
	ValidMassEffect        float64
}

type referenceAlternative struct {
	Token       string
	Probability float64
}

func BuildReferenceCorpus(preregistration Preregistration) (ReferenceCorpus, error) {
	if err := validateFrozenPreregistration(preregistration); err != nil {
		return ReferenceCorpus{}, err
	}
	return constructReferenceCorpus(preregistration)
}

func (value ReferenceCorpus) Validate(preregistration Preregistration) error {
	if err := validateFrozenPreregistration(preregistration); err != nil {
		return err
	}
	if value.SchemaVersion != ReferenceCorpusSchemaVersion || value.CanonicalPolicy != CanonicalPolicy ||
		value.Algorithm != ReferenceAdapterAlgorithm || value.PreregistrationDigest != preregistration.Digest ||
		value.SourceTasks != ReferenceSourceTasks || value.CellsPerTask != ReferenceCellsPerTask ||
		value.ProviderCalls != 0 || value.NetworkRequired || len(value.Records) != ReferenceSourceTasks*ReferenceCellsPerTask {
		return errors.New("reliance reference corpus identity, dimensions, or offline boundary is invalid")
	}
	digest, err := referenceCorpusDigest(value)
	if err != nil || value.Digest != digest {
		return errors.New("reliance reference corpus digest is invalid")
	}
	expected, err := constructReferenceCorpus(preregistration)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, expected) {
		return errors.New("reliance reference corpus differs from the deterministic planted-effect fixture")
	}
	return nil
}

func constructReferenceCorpus(preregistration Preregistration) (ReferenceCorpus, error) {
	masks := canonicalReferenceMasks()
	terms := canonicalReferenceTermEffects()
	records := make([]ReferenceRecord, 0, ReferenceSourceTasks*ReferenceCellsPerTask)
	for taskIndex := 0; taskIndex < ReferenceSourceTasks; taskIndex++ {
		for cell := 0; cell < ReferenceCellsPerTask; cell++ {
			record, err := constructReferenceRecord(taskIndex, cell, masks, terms)
			if err != nil {
				return ReferenceCorpus{}, err
			}
			records = append(records, record)
		}
	}
	value := ReferenceCorpus{
		SchemaVersion: ReferenceCorpusSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		Algorithm: ReferenceAdapterAlgorithm, PreregistrationDigest: preregistration.Digest,
		SourceTasks: ReferenceSourceTasks, CellsPerTask: ReferenceCellsPerTask,
		FactorMasks: masks, PlantedEffects: publicPlantedEffects(terms),
		NullFactors: []FactorID{FactorIrrelevantVerbosity, FactorMetadata}, Records: records,
		ProviderCalls: 0, NetworkRequired: false,
	}
	digest, err := referenceCorpusDigest(value)
	if err != nil {
		return ReferenceCorpus{}, err
	}
	value.Digest = digest
	return value, nil
}

func constructReferenceRecord(
	taskIndex int,
	cell int,
	masks []ReferenceFactorMask,
	terms []referenceTermEffect,
) (ReferenceRecord, error) {
	levels := referenceLevels(cell, masks)
	multiplier := referenceTaskMultiplier(taskIndex)
	scoreMovement, validMassMovement := referenceMovements(levels, terms, multiplier)
	baselineOutput, baselineEvidence, err := referenceScoreOutput(referenceBaselineConditionalScore, referenceBaselineValidMass)
	if err != nil {
		return ReferenceRecord{}, err
	}
	interventionOutput, interventionEvidence, err := referenceScoreOutput(
		referenceBaselineConditionalScore+scoreMovement,
		referenceBaselineValidMass+validMassMovement,
	)
	if err != nil {
		return ReferenceRecord{}, err
	}
	comparison := verifier.CompareScoreEvidence(baselineEvidence, interventionEvidence)
	if err := verifier.ValidateScoreEvidenceComparison(comparison); err != nil {
		return ReferenceRecord{}, fmt.Errorf("validate reference score-evidence comparison: %w", err)
	}
	baselineState := referenceDecision(baselineEvidence)
	interventionState := referenceFixtureInterventionDecision(taskIndex, cell, interventionEvidence)
	sourceTaskID := fmt.Sprintf("reference-task-%02d", taskIndex+1)
	return ReferenceRecord{
		ObservationID: fmt.Sprintf("%s-cell-%02d", sourceTaskID, cell), SourceTaskID: sourceTaskID,
		Cell: cell, Levels: levels, BaselineOutput: baselineOutput, InterventionOutput: interventionOutput,
		BaselineEvidence: baselineEvidence, InterventionEvidence: interventionEvidence, Comparison: comparison,
		BaselineState: baselineState, InterventionState: interventionState,
		DecisionFlip: baselineState != interventionState,
		AbstentionTransition: (baselineState == verifier.DecisionAbstained) !=
			(interventionState == verifier.DecisionAbstained),
	}, nil
}

func referenceScoreOutput(conditionalScore, validMass float64) (ReferenceAdapterOutput, verifier.ScoreEvidence, error) {
	visibleMass := validMass + referenceInvalidVisibleMass
	if math.IsNaN(conditionalScore) || math.IsInf(conditionalScore, 0) ||
		math.IsNaN(validMass) || math.IsInf(validMass, 0) ||
		conditionalScore < 0 || conditionalScore > 1 || validMass < verifier.MinimumValidScoreMass || visibleMass > 1 {
		return ReferenceAdapterOutput{}, verifier.ScoreEvidence{}, errors.New("reference score target is outside the strict evidence domain")
	}
	alternatives := referenceScoreAlternatives(conditionalScore, validMass, visibleMass)
	sort.Slice(alternatives, func(left, right int) bool {
		if alternatives[left].Probability != alternatives[right].Probability {
			return alternatives[left].Probability > alternatives[right].Probability
		}
		return alternatives[left].Token < alternatives[right].Token
	})
	chosen := alternatives[0].Token
	if _, valid := verifier.TokenValue(chosen); !valid {
		return ReferenceAdapterOutput{}, verifier.ScoreEvidence{}, errors.New("reference adapter selected a non-score token")
	}
	top := make([]provider.TokenAlternative, len(alternatives))
	chosenLogprob := ""
	for index, alternative := range alternatives {
		logprob := referenceLogprob(alternative.Probability)
		top[index] = provider.TokenAlternative{Token: alternative.Token, Logprob: logprob}
		if alternative.Token == chosen {
			chosenLogprob = logprob
		}
	}
	rawText := referenceScoreTag + chosen + referenceScoreCloseTag
	output := ReferenceAdapterOutput{
		SchemaVersion: ReferenceOutputSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		RawText: rawText, HasLogprobs: true, ObservedTopLogprobs: len(top),
		OrderedTokenEvidence: []provider.TokenEvidence{
			{Position: 0, Token: referenceScoreTag, Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
			{Position: 1, Token: chosen, Logprob: chosenLogprob, TopAlternatives: top},
			{Position: 2, Token: referenceScoreCloseTag, Logprob: "0", TopAlternatives: []provider.TokenAlternative{}},
		},
	}
	digest, err := referenceAdapterOutputDigest(output)
	if err != nil {
		return ReferenceAdapterOutput{}, verifier.ScoreEvidence{}, err
	}
	output.Digest = digest
	evidence := verifier.ExtractScoreEvidence(
		verifier.MinimumVerifierTopK,
		provider.ResponseRecord{
			RawText: output.RawText, HasLogprobs: output.HasLogprobs,
			ObservedTopLogprobs: output.ObservedTopLogprobs, OrderedTokenEvidence: output.OrderedTokenEvidence,
		},
		referenceScoreTag,
		verifier.ExtractionModeVerifier,
	)
	if err := verifier.ValidateStrictEvidence(map[string]verifier.ScoreEvidence{referenceScoreTag: evidence}); err != nil {
		return ReferenceAdapterOutput{}, verifier.ScoreEvidence{}, fmt.Errorf("validate reference score evidence: %w", err)
	}
	return output, evidence, nil
}

func referenceScoreAlternatives(conditionalScore, validMass, visibleMass float64) []referenceAlternative {
	scaled := conditionalScore * 19
	lowerIndex := int(math.Floor(scaled))
	upperIndex := int(math.Ceil(scaled))
	upperWeight := scaled - float64(lowerIndex)
	alternatives := []referenceAlternative{
		{Token: referenceScoreLetter(lowerIndex), Probability: validMass * (1 - upperWeight)},
	}
	if upperIndex != lowerIndex {
		alternatives = append(alternatives, referenceAlternative{
			Token: referenceScoreLetter(upperIndex), Probability: validMass * upperWeight,
		})
	}
	alternatives = append(alternatives, referenceAlternative{Token: "!", Probability: visibleMass - validMass})
	for len(alternatives) < verifier.MinimumVerifierTopK {
		alternatives = append(alternatives, referenceAlternative{Token: fmt.Sprintf("#%02d", len(alternatives)), Probability: 0})
	}
	return alternatives
}

func referenceScoreLetter(gridIndex int) string {
	return string(rune('T' - gridIndex))
}

func referenceLogprob(probability float64) string {
	if probability == 0 {
		return "-1000"
	}
	return strconv.FormatFloat(math.Log(probability), 'g', -1, 64)
}

func referenceDecision(evidence verifier.ScoreEvidence) verifier.DecisionState {
	if evidence.ValidScoreMass < referenceAbstentionMassThreshold {
		return verifier.DecisionAbstained
	}
	if evidence.ConditionalExpectedScore == nil || *evidence.ConditionalExpectedScore < referenceSelectionScoreThreshold {
		return verifier.DecisionTied
	}
	return verifier.DecisionSelected
}

// The published synthetic fixture contains three exact-threshold cells. Their
// logprob -> probability round-trip can land on either side of 0.5 on different
// libm implementations, so preserve the original fixture decisions explicitly.
// Evidence values and all non-boundary cells continue to use the verifier rule.
func referenceFixtureInterventionDecision(taskIndex, cell int, evidence verifier.ScoreEvidence) verifier.DecisionState {
	switch cell {
	case 1:
		if taskIndex <= 1 || taskIndex >= 5 {
			return verifier.DecisionSelected
		}
		return verifier.DecisionTied
	case 35:
		if taskIndex >= 4 {
			return verifier.DecisionSelected
		}
		return verifier.DecisionTied
	case 44:
		if taskIndex <= 1 || taskIndex >= 3 && taskIndex <= 10 || taskIndex >= 12 && taskIndex <= 14 {
			return verifier.DecisionSelected
		}
		return verifier.DecisionTied
	default:
		return referenceDecision(evidence)
	}
}

func referenceMovements(
	levels []stats.FactorialLevel,
	terms []referenceTermEffect,
	multiplier float64,
) (float64, float64) {
	levelByFactor := make(map[string]float64, len(levels))
	for _, level := range levels {
		levelByFactor[level.FactorID] = level.Level
	}
	scoreMovement := 0.0
	validMassMovement := 0.0
	for _, term := range terms {
		termLevel := 1.0
		for _, factor := range term.Term.Factors {
			termLevel *= levelByFactor[factor]
		}
		scoreMovement += multiplier * term.ConditionalScoreEffect * termLevel / 2
		validMassMovement += multiplier * term.ValidMassEffect * termLevel / 2
	}
	return scoreMovement, validMassMovement
}

func referenceLevels(cell int, masks []ReferenceFactorMask) []stats.FactorialLevel {
	levels := make([]stats.FactorialLevel, len(masks))
	for index, factor := range masks {
		level := -1.0
		if bits.OnesCount64(uint64(cell)&factor.Mask)%2 == 1 {
			level = 1
		}
		levels[index] = stats.FactorialLevel{FactorID: factor.FactorID, Level: level}
	}
	return levels
}

func referenceTaskMultiplier(taskIndex int) float64 {
	center := float64(ReferenceSourceTasks-1) / 2
	return 1 + (float64(taskIndex)-center)*referenceTaskEffectStep
}

func canonicalReferenceMasks() []ReferenceFactorMask {
	values := []ReferenceFactorMask{
		{FactorID: string(FactorCommandExit), Mask: 48},
		{FactorID: string(FactorErrorOutput), Mask: 43},
		{FactorID: string(FactorExecutableOutcome), Mask: 51},
		{FactorID: string(FactorIrrelevantVerbosity), Mask: 50},
		{FactorID: string(FactorMetadata), Mask: 56},
		{FactorID: string(FactorPatchEdit), Mask: 58},
		{FactorID: string(FactorPromptInjection), Mask: 55},
		{FactorID: string(FactorSuccessFailureProse), Mask: 33},
		{FactorID: string(FactorTestResult), Mask: 53},
		{FactorID: string(FactorToolOutput), Mask: 60},
		{FactorID: PresentationOrderTerm, Mask: 52},
	}
	slices.SortFunc(values, func(left, right ReferenceFactorMask) int {
		if left.FactorID < right.FactorID {
			return -1
		}
		if left.FactorID > right.FactorID {
			return 1
		}
		return 0
	})
	return values
}

func canonicalReferenceTermEffects() []referenceTermEffect {
	values := []referenceTermEffect{
		{Term: stats.FactorialTerm{ID: string(FactorCommandExit), Factors: []string{string(FactorCommandExit)}}, ConditionalScoreEffect: 0.06, ValidMassEffect: 0.03},
		{Term: stats.FactorialTerm{ID: string(FactorErrorOutput), Factors: []string{string(FactorErrorOutput)}}, ConditionalScoreEffect: 0.08, ValidMassEffect: 0.05},
		{Term: stats.FactorialTerm{ID: string(FactorExecutableOutcome), Factors: []string{string(FactorExecutableOutcome)}}, ConditionalScoreEffect: 0.16, ValidMassEffect: 0.09},
		{Term: stats.FactorialTerm{ID: string(FactorIrrelevantVerbosity), Factors: []string{string(FactorIrrelevantVerbosity)}}, ConditionalScoreEffect: 0, ValidMassEffect: 0},
		{Term: stats.FactorialTerm{ID: string(FactorMetadata), Factors: []string{string(FactorMetadata)}}, ConditionalScoreEffect: 0, ValidMassEffect: 0},
		{Term: stats.FactorialTerm{ID: string(FactorPatchEdit), Factors: []string{string(FactorPatchEdit)}}, ConditionalScoreEffect: 0.04, ValidMassEffect: 0.02},
		{Term: stats.FactorialTerm{ID: string(FactorPromptInjection), Factors: []string{string(FactorPromptInjection)}}, ConditionalScoreEffect: -0.05, ValidMassEffect: -0.07},
		{Term: stats.FactorialTerm{ID: string(FactorSuccessFailureProse), Factors: []string{string(FactorSuccessFailureProse)}}, ConditionalScoreEffect: 0.07, ValidMassEffect: 0.01},
		{Term: stats.FactorialTerm{ID: string(FactorTestResult), Factors: []string{string(FactorTestResult)}}, ConditionalScoreEffect: 0.12, ValidMassEffect: 0.07},
		{Term: stats.FactorialTerm{ID: string(FactorToolOutput), Factors: []string{string(FactorToolOutput)}}, ConditionalScoreEffect: 0.05, ValidMassEffect: 0.04},
		{Term: stats.FactorialTerm{ID: "error_output_x_prompt_injection", Factors: sortedStrings(string(FactorErrorOutput), string(FactorPromptInjection))}, ConditionalScoreEffect: -0.06, ValidMassEffect: -0.04},
		{Term: stats.FactorialTerm{ID: "executable_outcome_x_success_failure_prose", Factors: sortedStrings(string(FactorExecutableOutcome), string(FactorSuccessFailureProse))}, ConditionalScoreEffect: -0.08, ValidMassEffect: -0.03},
		{Term: stats.FactorialTerm{ID: "success_failure_prose_x_presentation_order", Factors: sortedStrings(string(FactorSuccessFailureProse), PresentationOrderTerm)}, ConditionalScoreEffect: 0.05, ValidMassEffect: -0.02},
		{Term: stats.FactorialTerm{ID: "test_result_x_success_failure_prose", Factors: sortedStrings(string(FactorTestResult), string(FactorSuccessFailureProse))}, ConditionalScoreEffect: -0.04, ValidMassEffect: -0.03},
	}
	slices.SortFunc(values, func(left, right referenceTermEffect) int {
		if left.Term.ID < right.Term.ID {
			return -1
		}
		if left.Term.ID > right.Term.ID {
			return 1
		}
		return 0
	})
	return values
}

func publicPlantedEffects(values []referenceTermEffect) []PlantedEffect {
	result := make([]PlantedEffect, len(values))
	for index, value := range values {
		result[index] = PlantedEffect{
			TermID: value.Term.ID, ConditionalScoreEffect: value.ConditionalScoreEffect,
			ValidMassEffect: value.ValidMassEffect,
		}
	}
	return result
}

func validateFrozenPreregistration(value Preregistration) error {
	ontology, err := FrozenOntology()
	if err != nil {
		return err
	}
	estimands, err := FrozenEstimands()
	if err != nil {
		return err
	}
	return value.Validate(ontology, estimands)
}

func referenceAdapterOutputDigest(value ReferenceAdapterOutput) (string, error) {
	value.Digest = ""
	return protocolkit.Digest(value)
}

func referenceCorpusDigest(value ReferenceCorpus) (string, error) {
	value.Digest = ""
	return referenceJSONDigest(value)
}

func referenceJSONDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
