package stats

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

// IdenticalResponseDesignAlgorithm names the provider-free paired design used by
// the TASK 070 identical-response counterfactual. Two decision arms read the
// same immutable completion: distribution-aware score evidence versus the
// chosen-token extraction. The paired unit is the source task group.
const IdenticalResponseDesignAlgorithm = "evalwitness.identical-response-paired-design.v1"

// IdenticalResponseDesignSpec freezes the exact statistical assumptions for the
// identical-response counterfactual before any response is observed.
type IdenticalResponseDesignSpec struct {
	SourceTaskGroups         int       `json:"source_task_groups"`
	DisagreementRates        []float64 `json:"disagreement_rates"`
	PrimaryDisagreementRate  float64   `json:"primary_disagreement_rate"`
	Alpha                    float64   `json:"alpha"`
	FamilySize               int       `json:"family_size"`
	TargetPower              float64   `json:"target_power"`
	DiscordantWinProbability float64   `json:"discordant_win_probability"`
	InvalidRate              float64   `json:"invalid_rate"`
	MissingRate              float64   `json:"missing_rate"`
	AbstentionRate           float64   `json:"abstention_rate"`
	RouteFailureRate         float64   `json:"route_failure_rate"`
}

// IdenticalResponseDesignRow reports the detectable paired effect and power for
// one prespecified disagreement rate over the effective source-task groups.
type IdenticalResponseDesignRow struct {
	DisagreementRate          float64  `json:"disagreement_rate"`
	ExpectedDiscordant        float64  `json:"expected_discordant"`
	MDENominal                *float64 `json:"minimum_detectable_paired_effect_nominal,omitempty"`
	MDEAdjusted               *float64 `json:"minimum_detectable_paired_effect_adjusted,omitempty"`
	PowerAtDeclaredQNominal   float64  `json:"power_at_declared_q_nominal"`
	PowerAtDeclaredQAdjusted  float64  `json:"power_at_declared_q_adjusted"`
	PowerAtSeparationNominal  float64  `json:"power_at_complete_separation_nominal"`
	PowerAtSeparationAdjusted float64  `json:"power_at_complete_separation_adjusted"`
}

// IdenticalResponseFailureRow reports how a combined loss fraction shrinks the
// effective source-task-group denominator and its minimum detectable effect at
// the primary disagreement rate.
type IdenticalResponseFailureRow struct {
	Scenario            string   `json:"scenario"`
	CombinedLoss        float64  `json:"combined_loss_fraction"`
	DisagreementRate    float64  `json:"disagreement_rate"`
	EffectiveTaskGroups int      `json:"effective_source_task_groups"`
	MDENominal          *float64 `json:"minimum_detectable_paired_effect_nominal,omitempty"`
	MDEAdjusted         *float64 `json:"minimum_detectable_paired_effect_adjusted,omitempty"`
}

// IdenticalResponseDesignReport is the deterministic, provider-free design
// preflight for the distribution_aware_vs_chosen_token counterfactual.
type IdenticalResponseDesignReport struct {
	Algorithm                string                        `json:"algorithm"`
	Counterfactual           string                        `json:"counterfactual"`
	SourceTaskGroups         int                           `json:"source_task_groups"`
	EffectiveTaskGroups      int                           `json:"effective_source_task_groups"`
	CombinedLoss             float64                       `json:"combined_loss_fraction"`
	Alpha                    float64                       `json:"alpha"`
	FamilySize               int                           `json:"family_size"`
	AdjustedAlpha            float64                       `json:"family_adjusted_alpha"`
	TargetPower              float64                       `json:"target_power"`
	DiscordantWinProbability float64                       `json:"discordant_win_probability"`
	Rows                     []IdenticalResponseDesignRow  `json:"disagreement_sensitivity"`
	FailureSensitivity       []IdenticalResponseFailureRow `json:"failure_sensitivity"`
	DesignDigest             string                        `json:"design_digest"`
}

// IdenticalResponseFailureScenarios returns the prespecified combined-loss grid
// used by the identical-response design: zero loss plus 5%, 10%, 15%, and 20%.
func IdenticalResponseFailureScenarios() []float64 {
	return []float64{0, 0.05, 0.10, 0.15, 0.20}
}

// ComputeIdenticalResponseDesign evaluates the frozen identical-response
// paired design without any provider access. Every row derives from the exact
// conditional McNemar machinery, so repeated runs are byte-reproducible.
func ComputeIdenticalResponseDesign(spec IdenticalResponseDesignSpec) (IdenticalResponseDesignReport, error) {
	report := IdenticalResponseDesignReport{
		Algorithm:                IdenticalResponseDesignAlgorithm,
		Counterfactual:           "distribution_aware_vs_chosen_token",
		SourceTaskGroups:         spec.SourceTaskGroups,
		Alpha:                    spec.Alpha,
		FamilySize:               spec.FamilySize,
		TargetPower:              spec.TargetPower,
		DiscordantWinProbability: spec.DiscordantWinProbability,
	}
	if spec.SourceTaskGroups < 1 {
		return report, errors.New("identical-response design requires at least one source task group")
	}
	if !(spec.Alpha > 0 && spec.Alpha < 0.5) || spec.FamilySize < 1 {
		return report, errors.New("identical-response design requires alpha in (0, 0.5) and a positive family size")
	}
	if !(spec.TargetPower > 0 && spec.TargetPower < 1) {
		return report, errors.New("identical-response design requires target power in (0, 1)")
	}
	if !(spec.DiscordantWinProbability > 0.5 && spec.DiscordantWinProbability <= 1) {
		return report, errors.New("identical-response discordant win probability must be in (0.5, 1]")
	}
	for _, rate := range []float64{spec.InvalidRate, spec.MissingRate, spec.AbstentionRate, spec.RouteFailureRate} {
		if !(rate >= 0 && rate < 1) {
			return report, errors.New("identical-response failure rates must be in [0, 1)")
		}
	}
	combined := spec.InvalidRate + spec.MissingRate + spec.AbstentionRate + spec.RouteFailureRate
	if combined >= 1 {
		return report, errors.New("combined failure rates must be below one")
	}
	report.CombinedLoss = combined
	report.AdjustedAlpha = spec.Alpha / float64(spec.FamilySize)
	report.EffectiveTaskGroups = effectiveTaskGroups(spec.SourceTaskGroups, combined)

	rates := dedupeSortedRates(spec.DisagreementRates)
	for _, rate := range rates {
		row, err := buildIdenticalResponseRow(report.EffectiveTaskGroups, rate, spec, report.AdjustedAlpha)
		if err != nil {
			return report, err
		}
		report.Rows = append(report.Rows, row)
	}

	scenarios := IdenticalResponseFailureScenarios()
	for _, loss := range scenarios {
		report.FailureSensitivity = append(report.FailureSensitivity, buildIdenticalResponseFailureRow(
			spec.SourceTaskGroups, loss, spec.PrimaryDisagreementRate, spec.Alpha, report.AdjustedAlpha, spec.TargetPower))
	}

	digest, err := canonicalIdenticalResponseDigest(spec)
	if err != nil {
		return report, err
	}
	report.DesignDigest = digest
	return report, nil
}

func buildIdenticalResponseRow(effectiveGroups int, rate float64, spec IdenticalResponseDesignSpec, adjustedAlpha float64) (IdenticalResponseDesignRow, error) {
	row := IdenticalResponseDesignRow{
		DisagreementRate:   rate,
		ExpectedDiscordant: float64(effectiveGroups) * rate,
	}
	nominal, err := MinimumDetectablePairedEffect(effectiveGroups, rate, spec.Alpha, spec.TargetPower)
	if err != nil {
		return row, err
	}
	adjusted, err := MinimumDetectablePairedEffect(effectiveGroups, rate, adjustedAlpha, spec.TargetPower)
	if err != nil {
		return row, err
	}
	row.MDENominal = nominal.MinimumPairedEffect
	row.MDEAdjusted = adjusted.MinimumPairedEffect
	row.PowerAtSeparationNominal = nominal.PowerAtCompleteSeparation
	row.PowerAtSeparationAdjusted = adjusted.PowerAtCompleteSeparation
	row.PowerAtDeclaredQNominal, err = ExactMcNemarUnconditionalPower(effectiveGroups, rate, spec.DiscordantWinProbability, spec.Alpha)
	if err != nil {
		return row, err
	}
	row.PowerAtDeclaredQAdjusted, err = ExactMcNemarUnconditionalPower(effectiveGroups, rate, spec.DiscordantWinProbability, adjustedAlpha)
	if err != nil {
		return row, err
	}
	return row, nil
}

func buildIdenticalResponseFailureRow(groups int, loss, rate, alpha, adjustedAlpha, targetPower float64) IdenticalResponseFailureRow {
	effective := effectiveTaskGroups(groups, loss)
	row := IdenticalResponseFailureRow{
		Scenario:            fmt.Sprintf("combined_loss_%.2f", loss),
		CombinedLoss:        loss,
		DisagreementRate:    rate,
		EffectiveTaskGroups: effective,
	}
	nominal, err := MinimumDetectablePairedEffect(effective, rate, alpha, targetPower)
	if err == nil {
		row.MDENominal = nominal.MinimumPairedEffect
	}
	adjusted, err := MinimumDetectablePairedEffect(effective, rate, adjustedAlpha, targetPower)
	if err == nil {
		row.MDEAdjusted = adjusted.MinimumPairedEffect
	}
	return row
}

func effectiveTaskGroups(groups int, combinedLoss float64) int {
	effective := int(math.Floor(float64(groups) * (1 - combinedLoss)))
	if effective < 0 {
		return 0
	}
	return effective
}

func dedupeSortedRates(rates []float64) []float64 {
	if len(rates) == 0 {
		return []float64{0}
	}
	seen := make(map[float64]struct{}, len(rates))
	deduped := make([]float64, 0, len(rates))
	for _, rate := range rates {
		if rate < 0 || rate > 1 {
			continue
		}
		if _, exists := seen[rate]; exists {
			continue
		}
		seen[rate] = struct{}{}
		deduped = append(deduped, rate)
	}
	sort.Float64s(deduped)
	if len(deduped) == 0 {
		return []float64{0}
	}
	return deduped
}

func canonicalIdenticalResponseDigest(spec IdenticalResponseDesignSpec) (string, error) {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
