package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/baseline"
	"github.com/Christopher-Schulze/evalwitness/internal/stats"
)

const (
	defaultDesignAlpha       = 0.05
	defaultDesignPower       = 0.80
	defaultDesignAlternative = 0.75
	defaultDisagreementRates = "0,0.02,0.14,0.31"
)

type evalDesignFlags struct {
	alpha             *float64
	targetPower       *float64
	alternativeQ      *float64
	minimumEffect     *float64
	equivalenceMargin *float64
	familySize        *int
	question          *string
	disagreementRates *string
}

type evalStatisticalPlan struct {
	TotalTasks         int                     `json:"total_tasks"`
	DecidableTasks     int                     `json:"decidable_tasks"`
	DecidableShare     float64                 `json:"decidable_share"`
	Question           stats.InferenceQuestion `json:"question"`
	Margin             float64                 `json:"margin"`
	MinimumEffect      float64                 `json:"minimum_effect"`
	NominalAlpha       float64                 `json:"nominal_alpha"`
	FamilySize         int                     `json:"family_size"`
	MultiplicityMethod string                  `json:"multiplicity_method"`
	AdjustedAlpha      float64                 `json:"family_adjusted_alpha"`
	TargetPower        float64                 `json:"target_power"`
	AlternativeQ       float64                 `json:"discordant_win_probability"`
	Method             string                  `json:"method"`
	Rows               []evalPowerRow          `json:"disagreement_sensitivity"`
	Warnings           []string                `json:"warnings,omitempty"`
}

type evalPowerRow struct {
	DisagreementRate           float64  `json:"disagreement_rate"`
	ExpectedDiscordant         float64  `json:"expected_discordant"`
	AlternativePowerNominal    float64  `json:"alternative_power_nominal"`
	AlternativePowerAdjusted   float64  `json:"alternative_power_adjusted"`
	MinimumEffectPowerNominal  *float64 `json:"minimum_effect_power_nominal,omitempty"`
	MinimumEffectPowerAdjusted *float64 `json:"minimum_effect_power_adjusted,omitempty"`
	MDENominal                 *float64 `json:"minimum_detectable_paired_effect_nominal,omitempty"`
	MDEAdjusted                *float64 `json:"minimum_detectable_paired_effect_adjusted,omitempty"`
	PowerAtSeparationNominal   float64  `json:"power_at_complete_separation_nominal"`
	PowerAtSeparationAdjusted  float64  `json:"power_at_complete_separation_adjusted"`
}

type evalObservedComparison struct {
	Comparator             string                `json:"comparator"`
	PairedTasks            int                   `json:"paired_tasks"`
	Discordant             int                   `json:"discordant"`
	SubjectOnly            int                   `json:"subject_only"`
	ComparatorOnly         int                   `json:"comparator_only"`
	Effect                 float64               `json:"paired_effect"`
	Interval               stats.PairedInterval  `json:"interval"`
	PValue                 float64               `json:"mcnemar_p"`
	SmallestSignificantWin *int                  `json:"smallest_significant_subject_wins,omitempty"`
	Resolution             string                `json:"design_resolution"`
	Inference              stats.PairedInference `json:"inference"`
}

func addEvalDesignFlags(fs *flag.FlagSet) evalDesignFlags {
	return evalDesignFlags{
		alpha:             fs.Float64("design-alpha", defaultDesignAlpha, "nominal alpha for paired design"),
		targetPower:       fs.Float64("design-power", defaultDesignPower, "target design power"),
		alternativeQ:      fs.Float64("design-alternative-q", defaultDesignAlternative, "declared A-win probability among discordant tasks"),
		minimumEffect:     fs.Float64("minimum-effect", 0, "minimum task-level paired effect the study must detect"),
		equivalenceMargin: fs.Float64("equivalence-margin", 0, "prespecified non-inferiority/equivalence margin"),
		familySize:        fs.Int("primary-family-size", 1, "Bonferroni primary-comparison family size"),
		question:          fs.String("inference-question", string(stats.QuestionSuperiority), "superiority/non_inferiority/equivalence"),
		disagreementRates: fs.String("disagreement-rates", defaultDisagreementRates, "comma-separated prespecified disagreement rates"),
	}
}

func validateEvalDesignFlags(flags evalDesignFlags) error {
	if !(*flags.alpha > 0 && *flags.alpha < 0.5) {
		return fmt.Errorf("--design-alpha must be between zero and one half")
	}
	if !(*flags.targetPower > 0 && *flags.targetPower < 1) {
		return fmt.Errorf("--design-power must be between zero and one")
	}
	if !(*flags.alternativeQ > 0.5 && *flags.alternativeQ <= 1) {
		return fmt.Errorf("--design-alternative-q must be in (0.5, 1]")
	}
	if *flags.minimumEffect < 0 || *flags.minimumEffect > 1 {
		return fmt.Errorf("--minimum-effect must be between zero and one")
	}
	if *flags.equivalenceMargin < 0 || *flags.equivalenceMargin >= 1 {
		return fmt.Errorf("--equivalence-margin must be in [0, 1)")
	}
	if *flags.familySize < 1 {
		return fmt.Errorf("--primary-family-size must be positive")
	}
	question := stats.InferenceQuestion(*flags.question)
	if question != stats.QuestionSuperiority && question != stats.QuestionNonInferiority && question != stats.QuestionEquivalence {
		return fmt.Errorf("--inference-question must be superiority, non_inferiority, or equivalence")
	}
	if question != stats.QuestionSuperiority && *flags.equivalenceMargin == 0 {
		return fmt.Errorf("%s requires --equivalence-margin > 0", question)
	}
	_, err := parseDesignRates(*flags.disagreementRates)
	return err
}

func buildEvalStatisticalPlan(totalTasks int, rewardRows [][]int, flags evalDesignFlags) (evalStatisticalPlan, error) {
	rates, err := parseDesignRates(*flags.disagreementRates)
	if err != nil {
		return evalStatisticalPlan{}, err
	}
	decidable := stats.CountDecidableBinary(rewardRows)
	adjustedAlpha := *flags.alpha / float64(*flags.familySize)
	plan := evalStatisticalPlan{
		TotalTasks: totalTasks, DecidableTasks: decidable,
		Question: stats.InferenceQuestion(*flags.question), Margin: *flags.equivalenceMargin,
		MinimumEffect: *flags.minimumEffect, NominalAlpha: *flags.alpha, FamilySize: *flags.familySize,
		MultiplicityMethod: "bonferroni", AdjustedAlpha: adjustedAlpha, TargetPower: *flags.targetPower,
		AlternativeQ: *flags.alternativeQ,
		Method:       "exact conditional McNemar rejection region averaged over Binomial(decidable_tasks, disagreement_rate)",
	}
	if totalTasks > 0 {
		plan.DecidableShare = float64(decidable) / float64(totalTasks)
	}
	if decidable == 0 {
		plan.Warnings = append(plan.Warnings, "no decidable tasks: selection cannot change any observed task outcome")
	}
	for _, rate := range rates {
		row, rowErr := buildEvalPowerRow(decidable, rate, flags, adjustedAlpha)
		if rowErr != nil {
			return evalStatisticalPlan{}, rowErr
		}
		plan.Rows = append(plan.Rows, row)
		if row.MDEAdjusted == nil {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("disagreement %.1f%%: target power %.0f%% is impossible even under complete separation at family-adjusted alpha", rate*100, *flags.targetPower*100))
		}
		if *flags.minimumEffect > 0 && (row.MinimumEffectPowerAdjusted == nil || *row.MinimumEffectPowerAdjusted < *flags.targetPower) {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("disagreement %.1f%%: minimum effect %.4f is underpowered at family-adjusted alpha", rate*100, *flags.minimumEffect))
		}
		if *flags.equivalenceMargin > 0 && (row.MDEAdjusted == nil || *row.MDEAdjusted > *flags.equivalenceMargin) {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("disagreement %.1f%%: the %.4f equivalence/non-inferiority margin is below this design's superiority MDE; do not authorize the margin without TASK 049's locked joint-outcome calculation", rate*100, *flags.equivalenceMargin))
		}
	}
	return plan, nil
}

func buildEvalPowerRow(decidable int, rate float64, flags evalDesignFlags, adjustedAlpha float64) (evalPowerRow, error) {
	nominal, err := stats.MinimumDetectablePairedEffect(decidable, rate, *flags.alpha, *flags.targetPower)
	if err != nil {
		return evalPowerRow{}, err
	}
	adjusted, err := stats.MinimumDetectablePairedEffect(decidable, rate, adjustedAlpha, *flags.targetPower)
	if err != nil {
		return evalPowerRow{}, err
	}
	nominalPower, err := stats.ExactMcNemarUnconditionalPower(decidable, rate, *flags.alternativeQ, *flags.alpha)
	if err != nil {
		return evalPowerRow{}, err
	}
	adjustedPower, err := stats.ExactMcNemarUnconditionalPower(decidable, rate, *flags.alternativeQ, adjustedAlpha)
	if err != nil {
		return evalPowerRow{}, err
	}
	row := evalPowerRow{
		DisagreementRate: rate, ExpectedDiscordant: float64(decidable) * rate,
		AlternativePowerNominal: nominalPower, AlternativePowerAdjusted: adjustedPower,
		MDENominal: nominal.MinimumPairedEffect, MDEAdjusted: adjusted.MinimumPairedEffect,
		PowerAtSeparationNominal:  nominal.PowerAtCompleteSeparation,
		PowerAtSeparationAdjusted: adjusted.PowerAtCompleteSeparation,
	}
	if *flags.minimumEffect > 0 && rate > 0 && *flags.minimumEffect <= rate {
		q := (1 + *flags.minimumEffect/rate) / 2
		power, powerErr := stats.ExactMcNemarUnconditionalPower(decidable, rate, q, *flags.alpha)
		if powerErr != nil {
			return evalPowerRow{}, powerErr
		}
		adjustedEffectPower, powerErr := stats.ExactMcNemarUnconditionalPower(decidable, rate, q, adjustedAlpha)
		if powerErr != nil {
			return evalPowerRow{}, powerErr
		}
		row.MinimumEffectPowerNominal = &power
		row.MinimumEffectPowerAdjusted = &adjustedEffectPower
	}
	return row, nil
}

func buildObservedComparisons(results []baseline.Result, plan evalStatisticalPlan) ([]evalObservedComparison, error) {
	confidence, err := stats.PairedInferenceConfidence(plan.Question, plan.AdjustedAlpha)
	if err != nil {
		return nil, err
	}
	comparisons := make([]evalObservedComparison, 0, len(results))
	for _, result := range results {
		if result.PairedTasks == 0 {
			continue
		}
		interval, err := stats.PairedBinaryScoreInterval(result.BothCorrect, result.SubjectOnly, result.BaselineOnly, result.BothWrong, confidence)
		if err != nil {
			return nil, err
		}
		inference, err := stats.EvaluatePairedQuestion(plan.Question, plan.Margin, plan.AdjustedAlpha, interval, result.McNemarP)
		if err != nil {
			return nil, err
		}
		region, err := stats.ExactMcNemarRejectionRegion(result.Discordant, plan.AdjustedAlpha)
		if err != nil {
			return nil, err
		}
		comparison := evalObservedComparison{
			Comparator: result.Name, PairedTasks: result.PairedTasks, Discordant: result.Discordant,
			SubjectOnly: result.SubjectOnly, ComparatorOnly: result.BaselineOnly,
			Effect: interval.Estimate, Interval: interval, PValue: result.McNemarP, Inference: inference,
			Resolution: "no split can reject at the family-adjusted alpha for the observed discordant count",
		}
		if region.UpperMin != nil {
			comparison.SmallestSignificantWin = region.UpperMin
			comparison.Resolution = fmt.Sprintf("smallest significant subject/comparator split is %d:%d", *region.UpperMin, result.Discordant-*region.UpperMin)
		}
		comparisons = append(comparisons, comparison)
	}
	return comparisons, nil
}

func parseDesignRates(raw string) ([]float64, error) {
	parts := strings.Split(raw, ",")
	rates := make([]float64, 0, len(parts))
	seen := make(map[float64]struct{}, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || value < 0 || value > 1 {
			return nil, fmt.Errorf("--disagreement-rates values must be numbers in [0, 1]")
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		rates = append(rates, value)
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("--disagreement-rates requires at least one rate")
	}
	return rates, nil
}

func printEvalStatisticalPlan(plan evalStatisticalPlan) {
	fmt.Printf("Design: total=%d decidable=%d (%.1f%%) question=%s margin=%.4f\n", plan.TotalTasks, plan.DecidableTasks, plan.DecidableShare*100, plan.Question, plan.Margin)
	fmt.Printf("Design alpha: nominal=%.4f adjusted=%.4f (%s family=%d) target_power=%.2f alternative_q=%.3f\n", plan.NominalAlpha, plan.AdjustedAlpha, plan.MultiplicityMethod, plan.FamilySize, plan.TargetPower, plan.AlternativeQ)
	for _, row := range plan.Rows {
		fmt.Printf("  disagreement=%5.1f%% expected_discordant=%6.2f power=%.3f/%.3f MDE=%s/%s nominal/adjusted\n",
			row.DisagreementRate*100, row.ExpectedDiscordant, row.AlternativePowerNominal, row.AlternativePowerAdjusted,
			optionalFloat(row.MDENominal), optionalFloat(row.MDEAdjusted))
	}
	for _, warning := range plan.Warnings {
		fmt.Println("  WARNING:", warning)
	}
}

func printObservedComparisons(comparisons []evalObservedComparison) {
	for _, comparison := range comparisons {
		fmt.Printf("Paired vs %s: n=%d discordant=%d effect=%+.4f %.1f%% CI [%+.4f,%+.4f] p=%.4f\n", comparison.Comparator, comparison.PairedTasks, comparison.Discordant, comparison.Effect, comparison.Interval.Confidence*100, comparison.Interval.Lower, comparison.Interval.Upper, comparison.PValue)
		fmt.Printf("  resolution: %s; %s\n", comparison.Resolution, comparison.Inference.Conclusion)
	}
}

func optionalFloat(value *float64) string {
	if value == nil {
		return "none"
	}
	return fmt.Sprintf("%.4f", *value)
}
