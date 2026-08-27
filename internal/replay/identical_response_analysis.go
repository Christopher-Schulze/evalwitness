package replay

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"

	"github.com/Christopher-Schulze/evalwitness/internal/stats"
	"github.com/Christopher-Schulze/evalwitness/internal/study"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const IdenticalResponseAnalysisSchemaVersion = "evalwitness.identical-response-offline-analysis.v1"

const (
	DecisionSelected = "selected"
	DecisionTied     = "tied"
	DecisionMissing  = "missing"
)

type IdenticalResponseDecision struct {
	State         string    `json:"state"`
	SelectedIndex *int      `json:"selected_index,omitempty"`
	Scores        []float64 `json:"scores"`
	Margin        float64   `json:"margin"`
}

type IdenticalResponseOutcomeRow struct {
	Available          bool `json:"available"`
	DistributionReward *int `json:"distribution_reward,omitempty"`
	ChosenTokenReward  *int `json:"chosen_token_reward,omitempty"`
}

type IdenticalResponseRow struct {
	GroupID            string                      `json:"group_id"`
	TaskID             string                      `json:"task_id"`
	Status             string                      `json:"status"`
	ResponseBodyDigest string                      `json:"response_body_digest,omitempty"`
	RecordDigest       string                      `json:"record_digest,omitempty"`
	DistributionAware  IdenticalResponseDecision   `json:"distribution_aware"`
	ChosenToken        IdenticalResponseDecision   `json:"chosen_token"`
	OutcomeSensitivity IdenticalResponseOutcomeRow `json:"outcome_sensitivity"`
}

type IdenticalResponseSummary struct {
	TaskGroups                          int     `json:"task_groups"`
	Complete                            int     `json:"complete"`
	Agreements                          int     `json:"agreements"`
	Disagreements                       int     `json:"disagreements"`
	Unresolved                          int     `json:"unresolved"`
	DisagreementRate                    float64 `json:"disagreement_rate"`
	MissingnessWorstCaseExactUpperBound float64 `json:"missingness_worst_case_exact_upper_confidence_bound"`
}

type IdenticalResponseOutcomeSummary struct {
	Covered                 int     `json:"covered"`
	CoverageRate            float64 `json:"coverage_rate"`
	BothSucceeded           int     `json:"both_succeeded"`
	BothFailed              int     `json:"both_failed"`
	DistributionOnlySuccess int     `json:"distribution_only_success"`
	ChosenTokenOnlySuccess  int     `json:"chosen_token_only_success"`
	McNemarExactP           float64 `json:"mcnemar_exact_p"`
}

type IdenticalResponseAnalysis struct {
	SchemaVersion       string                          `json:"schema_version"`
	Counterfactual      string                          `json:"counterfactual"`
	StudyRecordDigest   string                          `json:"study_record_digest"`
	StudyManifestDigest string                          `json:"study_manifest_digest"`
	CaptureSHA256       string                          `json:"capture_sha256"`
	RouteID             string                          `json:"route_id"`
	Epsilon             float64                         `json:"epsilon"`
	ConfidenceLevel     float64                         `json:"confidence_level"`
	Summary             IdenticalResponseSummary        `json:"summary"`
	OutcomeSensitivity  IdenticalResponseOutcomeSummary `json:"outcome_sensitivity"`
	Rows                []IdenticalResponseRow          `json:"rows"`
	Digest              string                          `json:"digest"`
}

var jointAbsoluteTagPattern = regexp.MustCompile(`^<score_([0-9]+)_[a-z0-9_]+>$`)

func AnalyzeIdenticalResponse(path string, record study.Record, outcomes map[string][]int, epsilon float64) (IdenticalResponseAnalysis, error) {
	if err := record.Validate(); err != nil {
		return IdenticalResponseAnalysis{}, err
	}
	if epsilon <= 0 || epsilon >= 1 {
		return IdenticalResponseAnalysis{}, errors.New("identical-response epsilon must be in (0,1)")
	}
	arm, err := identicalResponseArm(record.Study.Manifest)
	if err != nil {
		return IdenticalResponseAnalysis{}, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return IdenticalResponseAnalysis{}, err
	}
	inspection, _, err := inspectCapturePayload(payload)
	if err != nil {
		return IdenticalResponseAnalysis{}, err
	}
	if inspection.RouteID != arm.RouteID || inspection.ProviderID != arm.ProviderID || inspection.RequestedModel != arm.RequestedModel {
		return IdenticalResponseAnalysis{}, errors.New("identical-response capture route differs from the locked study arm")
	}

	expected, err := expectedIdenticalResponseGroups(record.Study.Manifest, outcomes)
	if err != nil {
		return IdenticalResponseAnalysis{}, err
	}
	entries, err := identicalResponseEntries(payload, arm, expected)
	if err != nil {
		return IdenticalResponseAnalysis{}, err
	}
	analysis := IdenticalResponseAnalysis{
		SchemaVersion:     IdenticalResponseAnalysisSchemaVersion,
		Counterfactual:    "distribution_aware_vs_chosen_token",
		StudyRecordDigest: record.RecordDigest, StudyManifestDigest: record.Study.ManifestDigest,
		CaptureSHA256: inspection.PayloadSHA256, RouteID: arm.RouteID, Epsilon: epsilon,
		ConfidenceLevel: 1 - record.Study.Manifest.Inference.NominalAlpha/float64(len(record.Study.Manifest.Inference.PrimaryFamily)),
		Rows:            make([]IdenticalResponseRow, 0, len(expected)),
	}
	for _, group := range expected {
		entry, found := entries[group.GroupID]
		row := IdenticalResponseRow{
			GroupID: group.GroupID, TaskID: group.TaskID, Status: DecisionMissing,
			DistributionAware: missingIdenticalResponseDecision(arm.Candidates),
			ChosenToken:       missingIdenticalResponseDecision(arm.Candidates),
		}
		if found {
			row, err = analyzeIdenticalResponseEntry(entry, group, arm.Candidates, epsilon)
			if err != nil {
				return IdenticalResponseAnalysis{}, fmt.Errorf("group %s: %w", group.GroupID, err)
			}
		}
		applyIdenticalResponseOutcome(&row, outcomes[group.TaskID], &analysis.OutcomeSensitivity)
		analysis.Rows = append(analysis.Rows, row)
		analysis.Summary.TaskGroups++
		switch row.Status {
		case "agreement":
			analysis.Summary.Complete++
			analysis.Summary.Agreements++
		case "disagreement":
			analysis.Summary.Complete++
			analysis.Summary.Disagreements++
		default:
			analysis.Summary.Unresolved++
		}
	}
	if analysis.Summary.TaskGroups > 0 {
		denominator := float64(analysis.Summary.TaskGroups)
		analysis.Summary.DisagreementRate = float64(analysis.Summary.Disagreements) / denominator
		analysis.OutcomeSensitivity.CoverageRate = float64(analysis.OutcomeSensitivity.Covered) / denominator
	}
	analysis.Summary.MissingnessWorstCaseExactUpperBound = exactBinomialUpperBound(
		analysis.Summary.Disagreements+analysis.Summary.Unresolved, analysis.Summary.TaskGroups, 1-analysis.ConfidenceLevel,
	)
	analysis.OutcomeSensitivity.McNemarExactP = stats.McNemarExact(
		analysis.OutcomeSensitivity.DistributionOnlySuccess, analysis.OutcomeSensitivity.ChosenTokenOnlySuccess,
	)
	analysis.Digest, err = identicalResponseAnalysisDigest(analysis)
	if err != nil {
		return IdenticalResponseAnalysis{}, err
	}
	return analysis, nil
}

type identicalResponseGroup struct {
	GroupID string
	TaskID  string
}

func identicalResponseArm(manifest study.Manifest) (study.Arm, error) {
	var matched []study.Arm
	for _, arm := range manifest.Arms {
		if arm.ID == "distribution-aware-vs-chosen-token" && arm.SelectionMode == "joint_absolute" {
			matched = append(matched, arm)
		}
	}
	if len(matched) != 1 || matched[0].Candidates < 2 || matched[0].Repetitions != 1 {
		return study.Arm{}, errors.New("study requires one fixed-repetition joint_absolute identical-response arm")
	}
	return matched[0], nil
}

func expectedIdenticalResponseGroups(manifest study.Manifest, outcomes map[string][]int) ([]identicalResponseGroup, error) {
	groups := make([]identicalResponseGroup, 0, len(outcomes))
	seenTasks := make(map[string]struct{}, len(outcomes))
	for _, assignment := range manifest.Data.Split.Assignments {
		for _, taskID := range assignment.TaskIDs {
			if _, ok := outcomes[taskID]; !ok {
				continue
			}
			if _, duplicate := seenTasks[taskID]; duplicate {
				return nil, fmt.Errorf("outcome task %q has multiple split assignments", taskID)
			}
			seenTasks[taskID] = struct{}{}
			groups = append(groups, identicalResponseGroup{GroupID: assignment.GroupID, TaskID: taskID})
		}
	}
	if len(groups) != len(outcomes) || len(groups) < 1 {
		return nil, errors.New("outcome tasks do not map one-to-one onto the locked split")
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].GroupID < groups[j].GroupID })
	return groups, nil
}

func identicalResponseEntries(payload []byte, arm study.Arm, expected []identicalResponseGroup) (map[string]fixtureEntry, error) {
	allowed := make(map[string]struct{}, len(expected))
	for _, group := range expected {
		allowed[group.GroupID] = struct{}{}
	}
	entries := make(map[string]fixtureEntry, len(expected))
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		var entry fixtureEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		groupID := entry.Request.Lineage.AuditCaseID
		if _, ok := allowed[groupID]; !ok {
			return nil, fmt.Errorf("capture group %q is outside the analysis population", groupID)
		}
		if _, duplicate := entries[groupID]; duplicate {
			return nil, fmt.Errorf("capture repeats group %q", groupID)
		}
		if entry.Request.Lineage.StudyCellID != arm.ID || entry.Request.Lineage.Entrypoint != arm.Entrypoint {
			return nil, fmt.Errorf("capture group %q differs from the locked study cell", groupID)
		}
		entries[groupID] = entry
	}
	return entries, scanner.Err()
}

func analyzeIdenticalResponseEntry(entry fixtureEntry, group identicalResponseGroup, candidates int, epsilon float64) (IdenticalResponseRow, error) {
	if err := verifier.ValidateStrictEvidence(entry.ScoreEvidence); err != nil {
		return IdenticalResponseRow{}, err
	}
	distributionScores := make([][]float64, candidates)
	chosenScores := make([][]float64, candidates)
	for tag, evidence := range entry.ScoreEvidence {
		match := jointAbsoluteTagPattern.FindStringSubmatch(tag)
		if match == nil {
			return IdenticalResponseRow{}, fmt.Errorf("score tag %q is not joint_absolute", tag)
		}
		candidate, err := strconv.Atoi(match[1])
		if err != nil || candidate < 0 || candidate >= candidates {
			return IdenticalResponseRow{}, fmt.Errorf("score tag %q has an invalid candidate index", tag)
		}
		distributionScores[candidate] = append(distributionScores[candidate], *evidence.ConditionalExpectedScore)
		chosen, err := chosenScore(evidence)
		if err != nil {
			return IdenticalResponseRow{}, fmt.Errorf("score tag %q: %w", tag, err)
		}
		chosenScores[candidate] = append(chosenScores[candidate], chosen)
	}
	distribution, err := identicalResponseDecision(distributionScores, epsilon)
	if err != nil {
		return IdenticalResponseRow{}, err
	}
	chosen, err := identicalResponseDecision(chosenScores, epsilon)
	if err != nil {
		return IdenticalResponseRow{}, err
	}
	status := "disagreement"
	if distribution.State == chosen.State && (distribution.State == DecisionTied || *distribution.SelectedIndex == *chosen.SelectedIndex) {
		status = "agreement"
	}
	return IdenticalResponseRow{
		GroupID: group.GroupID, TaskID: group.TaskID, Status: status,
		ResponseBodyDigest: entry.Response.ResponseBodyDigest, RecordDigest: entry.RecordDigest,
		DistributionAware: distribution, ChosenToken: chosen,
	}, nil
}

func chosenScore(evidence verifier.ScoreEvidence) (float64, error) {
	var chosen []verifier.VisibleAlternative
	for _, alternative := range evidence.Alternatives {
		if alternative.Chosen {
			chosen = append(chosen, alternative)
		}
	}
	if len(chosen) != 1 || chosen[0].CanonicalValue == nil {
		return 0, errors.New("evidence must contain exactly one canonical chosen score token")
	}
	return *chosen[0].CanonicalValue, nil
}

func identicalResponseDecision(values [][]float64, epsilon float64) (IdenticalResponseDecision, error) {
	scores := make([]float64, len(values))
	for index, candidateValues := range values {
		if len(candidateValues) < 1 {
			return IdenticalResponseDecision{}, fmt.Errorf("candidate %d has no score evidence", index)
		}
		for _, value := range candidateValues {
			scores[index] += value
		}
		scores[index] /= float64(len(candidateValues))
	}
	best := 0
	for index := 1; index < len(scores); index++ {
		if scores[index] > scores[best] {
			best = index
		}
	}
	runnerUp := math.Inf(-1)
	for index, score := range scores {
		if index != best && score > runnerUp {
			runnerUp = score
		}
	}
	margin := scores[best] - runnerUp
	decision := IdenticalResponseDecision{State: DecisionTied, Scores: scores, Margin: margin}
	if margin > epsilon+1e-12 {
		decision.State = DecisionSelected
		decision.SelectedIndex = &best
	}
	return decision, nil
}

func missingIdenticalResponseDecision(candidates int) IdenticalResponseDecision {
	return IdenticalResponseDecision{State: DecisionMissing, Scores: make([]float64, candidates)}
}

func applyIdenticalResponseOutcome(row *IdenticalResponseRow, rewards []int, summary *IdenticalResponseOutcomeSummary) {
	if row.Status == DecisionMissing || row.DistributionAware.SelectedIndex == nil || row.ChosenToken.SelectedIndex == nil {
		return
	}
	distributionIndex := *row.DistributionAware.SelectedIndex
	chosenIndex := *row.ChosenToken.SelectedIndex
	if distributionIndex >= len(rewards) || chosenIndex >= len(rewards) {
		return
	}
	distributionReward := rewards[distributionIndex]
	chosenReward := rewards[chosenIndex]
	row.OutcomeSensitivity = IdenticalResponseOutcomeRow{
		Available: true, DistributionReward: &distributionReward, ChosenTokenReward: &chosenReward,
	}
	summary.Covered++
	switch {
	case distributionReward > 0 && chosenReward > 0:
		summary.BothSucceeded++
	case distributionReward <= 0 && chosenReward <= 0:
		summary.BothFailed++
	case distributionReward > 0:
		summary.DistributionOnlySuccess++
	default:
		summary.ChosenTokenOnlySuccess++
	}
}

func exactBinomialUpperBound(successes, trials int, alpha float64) float64 {
	if trials < 1 || successes >= trials {
		return 1
	}
	if successes < 0 {
		successes = 0
	}
	low := float64(successes) / float64(trials)
	high := 1.0
	for range 100 {
		mid := (low + high) / 2
		if binomialCDF(successes, trials, mid) > alpha {
			low = mid
		} else {
			high = mid
		}
	}
	return high
}

func binomialCDF(successes, trials int, probability float64) float64 {
	if probability <= 0 {
		return 1
	}
	if probability >= 1 {
		if successes >= trials {
			return 1
		}
		return 0
	}
	term := math.Pow(1-probability, float64(trials))
	total := term
	for index := 0; index < successes; index++ {
		term *= float64(trials-index) / float64(index+1) * probability / (1 - probability)
		total += term
	}
	return total
}

func unsignedIdenticalResponseAnalysis(analysis IdenticalResponseAnalysis) IdenticalResponseAnalysis {
	analysis.Digest = ""
	analysis.Rows = slices.Clone(analysis.Rows)
	return analysis
}

func identicalResponseAnalysisDigest(analysis IdenticalResponseAnalysis) (string, error) {
	encoded, err := json.Marshal(unsignedIdenticalResponseAnalysis(analysis))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
