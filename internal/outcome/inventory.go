package outcome

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	NaturalRequestSchemaVersion   = "evalwitness.outcome-natural-inventory-request.v1"
	NaturalInventorySchemaVersion = "evalwitness.outcome-natural-inventory.v1"
	maximumLegacyResultBytes      = 64 * 1024 * 1024
)

type NaturalSourcePair struct {
	Suite        string `json:"suite"`
	VerifierPath string `json:"verifier_path"`
	JudgePath    string `json:"judge_path"`
}

type NaturalInventoryRequest struct {
	SchemaVersion                string              `json:"schema_version"`
	CanonicalPolicy              string              `json:"canonical_policy"`
	SourceClass                  string              `json:"source_class"`
	BaselineRule                 string              `json:"baseline_rule"`
	HighConfidenceErrorThreshold float64             `json:"high_confidence_error_threshold"`
	SourcePairs                  []NaturalSourcePair `json:"source_pairs"`
	Digest                       string              `json:"digest"`
}

type NaturalSourceArtifact struct {
	Suite     string `json:"suite"`
	Role      string `json:"role"`
	Path      string `json:"path"`
	Digest    string `json:"digest"`
	TaskCount int    `json:"task_count"`
}

type NaturalSelection struct {
	Stratum         string `json:"stratum"`
	Rank            int    `json:"rank"`
	Suite           string `json:"suite"`
	TaskGroupDigest string `json:"task_group_digest"`
	CaseDigest      string `json:"case_digest"`
}

type NaturalShortfall struct {
	Stratum  string `json:"stratum"`
	Required int    `json:"required"`
	Eligible int    `json:"eligible"`
	Selected int    `json:"selected"`
}

type NaturalInventory struct {
	SchemaVersion      string                  `json:"schema_version"`
	CanonicalPolicy    string                  `json:"canonical_policy"`
	PlanDigest         string                  `json:"plan_digest"`
	RequestDigest      string                  `json:"request_digest"`
	SourceClass        string                  `json:"source_class"`
	TargetCases        int                     `json:"target_cases"`
	SelectedCases      int                     `json:"selected_cases"`
	Status             string                  `json:"status"`
	CandidateCounts    []SampleCount           `json:"candidate_counts"`
	SelectedCounts     []SampleCount           `json:"selected_counts"`
	Shortfalls         []NaturalShortfall      `json:"shortfalls"`
	SourceArtifacts    []NaturalSourceArtifact `json:"source_artifacts"`
	Selections         []NaturalSelection      `json:"selections"`
	SelectionDigest    string                  `json:"selection_digest"`
	ReproducibilityGap string                  `json:"reproducibility_gap"`
	Digest             string                  `json:"digest"`
}

type legacyResult struct {
	Details []legacyDetail `json:"details"`
}

type legacyDetail struct {
	TaskName        string           `json:"task_name"`
	InstanceID      string           `json:"instance_id"`
	Rewards         []int            `json:"rewards"`
	SelectedIndex   *int             `json:"selected_index"`
	SelectedReward  *int             `json:"selected_reward"`
	SkippedReason   *string          `json:"skipped_reason"`
	Selection       *legacySelection `json:"selection"`
	Abstained       bool             `json:"abstained"`
	ProviderFailure bool             `json:"provider_failure"`
}

type legacySelection struct {
	Confidence float64 `json:"confidence"`
}

type naturalCandidate struct {
	Suite        string
	TaskID       string
	TaskDigest   string
	CaseDigest   string
	VerifierData legacyDetail
	JudgeData    legacyDetail
}

func DefaultNaturalInventoryRequest() (NaturalInventoryRequest, error) {
	return SealNaturalInventoryRequest(NaturalInventoryRequest{
		SourceClass: "development_only", BaselineRule: "candidate_index_zero", HighConfidenceErrorThreshold: 0.80,
		SourcePairs: []NaturalSourcePair{
			{Suite: "swebench", VerifierPath: "eval/results/swebench-verifier.json", JudgePath: "eval/results/swebench-judge.json"},
			{Suite: "terminal-bench", VerifierPath: "eval/results/terminal-bench-verifier.json", JudgePath: "eval/results/terminal-bench-judge.json"},
		},
	})
}

func SealNaturalInventoryRequest(request NaturalInventoryRequest) (NaturalInventoryRequest, error) {
	request.SchemaVersion = NaturalRequestSchemaVersion
	request.CanonicalPolicy = CanonicalPolicy
	request.Digest = ""
	digest, err := naturalRequestDigest(request)
	if err != nil {
		return NaturalInventoryRequest{}, err
	}
	request.Digest = digest
	return request, request.Validate()
}

func (request NaturalInventoryRequest) Validate() error {
	if request.SchemaVersion != NaturalRequestSchemaVersion || request.CanonicalPolicy != CanonicalPolicy ||
		request.SourceClass != "development_only" || request.BaselineRule != "candidate_index_zero" ||
		request.HighConfidenceErrorThreshold <= 0 || request.HighConfidenceErrorThreshold >= 1 || len(request.SourcePairs) == 0 {
		return errors.New("natural inventory request identity, source class, baseline, threshold, or sources is invalid")
	}
	for index, pair := range request.SourcePairs {
		if missing(pair.Suite, pair.VerifierPath, pair.JudgePath) || index > 0 && request.SourcePairs[index-1].Suite >= pair.Suite {
			return errors.New("natural inventory source pairs must be complete, unique, and sorted by suite")
		}
	}
	expected, err := naturalRequestDigest(request)
	if err != nil || request.Digest != expected {
		return errors.New("natural inventory request digest is invalid")
	}
	return nil
}

func BuildNaturalInventory(plan Plan, request NaturalInventoryRequest) (NaturalInventory, error) {
	if err := plan.Validate(); err != nil {
		return NaturalInventory{}, err
	}
	if err := request.Validate(); err != nil {
		return NaturalInventory{}, err
	}
	if plan.NaturalSampleSize%len(plan.RequiredStrata) != 0 {
		return NaturalInventory{}, errors.New("natural sample size must divide evenly across required strata")
	}
	perStratum := plan.NaturalSampleSize / len(plan.RequiredStrata)
	candidates := make(map[string][]naturalCandidate, len(plan.RequiredStrata))
	artifacts := make([]NaturalSourceArtifact, 0, len(request.SourcePairs)*2)
	for _, pair := range request.SourcePairs {
		verifier, verifierArtifact, err := readLegacyResult(pair.Suite, "verifier", pair.VerifierPath)
		if err != nil {
			return NaturalInventory{}, err
		}
		judge, judgeArtifact, err := readLegacyResult(pair.Suite, "judge", pair.JudgePath)
		if err != nil {
			return NaturalInventory{}, err
		}
		artifacts = append(artifacts, judgeArtifact, verifierArtifact)
		if err := addNaturalCandidates(candidates, pair.Suite, verifier, judge, request.HighConfidenceErrorThreshold); err != nil {
			return NaturalInventory{}, err
		}
	}
	sort.Slice(artifacts, func(left, right int) bool {
		if artifacts[left].Suite != artifacts[right].Suite {
			return artifacts[left].Suite < artifacts[right].Suite
		}
		return artifacts[left].Role < artifacts[right].Role
	})
	counts, selectedCounts := make(map[string]int), make(map[string]int)
	var selections []NaturalSelection
	var shortfalls []NaturalShortfall
	allocationOrder := append([]string(nil), plan.RequiredStrata...)
	sort.Slice(allocationOrder, func(left, right int) bool {
		if len(candidates[allocationOrder[left]]) != len(candidates[allocationOrder[right]]) {
			return len(candidates[allocationOrder[left]]) < len(candidates[allocationOrder[right]])
		}
		return allocationOrder[left] < allocationOrder[right]
	})
	usedTaskGroups := make(map[string]struct{})
	for _, stratum := range allocationOrder {
		eligible := candidates[stratum]
		counts[stratum] = len(eligible)
		sort.Slice(eligible, func(left, right int) bool {
			leftHash := digestText(plan.Digest + "\x00" + stratum + "\x00" + eligible[left].Suite + "\x00" + eligible[left].TaskID)
			rightHash := digestText(plan.Digest + "\x00" + stratum + "\x00" + eligible[right].Suite + "\x00" + eligible[right].TaskID)
			return leftHash < rightHash
		})
		selected := 0
		for _, candidate := range eligible {
			if selected == perStratum {
				break
			}
			if _, exists := usedTaskGroups[candidate.TaskDigest]; exists {
				continue
			}
			usedTaskGroups[candidate.TaskDigest] = struct{}{}
			selected++
			selections = append(selections, NaturalSelection{
				Stratum: stratum, Rank: selected, Suite: candidate.Suite,
				TaskGroupDigest: candidate.TaskDigest, CaseDigest: candidate.CaseDigest,
			})
		}
		selectedCounts[stratum] = selected
		if selected < perStratum {
			shortfalls = append(shortfalls, NaturalShortfall{Stratum: stratum, Required: perStratum, Eligible: len(eligible), Selected: selected})
		}
	}
	sort.Slice(selections, func(left, right int) bool {
		if selections[left].Stratum != selections[right].Stratum {
			return selections[left].Stratum < selections[right].Stratum
		}
		return selections[left].Rank < selections[right].Rank
	})
	sort.Slice(shortfalls, func(left, right int) bool { return shortfalls[left].Stratum < shortfalls[right].Stratum })
	identities := make([]string, len(selections))
	for index, selection := range selections {
		identities[index] = selection.Stratum + "\x00" + selection.CaseDigest
	}
	inventory := NaturalInventory{
		PlanDigest: plan.Digest, RequestDigest: request.Digest, SourceClass: request.SourceClass,
		TargetCases: plan.NaturalSampleSize, SelectedCases: len(selections), Status: "complete",
		CandidateCounts: sampleCounts(counts), SelectedCounts: sampleCountsIncludingZero(selectedCounts, plan.RequiredStrata),
		Shortfalls: shortfalls, SourceArtifacts: artifacts, Selections: selections,
		SelectionDigest: digestText(joinWithNUL(identities)), ReproducibilityGap: "none",
	}
	if len(shortfalls) > 0 {
		inventory.Status = "incomplete"
		inventory.ReproducibilityGap = "prespecified strata without eligible real observations remain unfilled; no synthetic cases, repeated task groups, or cross-stratum replacements are allowed"
	}
	return SealNaturalInventory(inventory)
}

func addNaturalCandidates(destination map[string][]naturalCandidate, suite string, verifier, judge legacyResult, threshold float64) error {
	judgeByTask := make(map[string]legacyDetail, len(judge.Details))
	for _, detail := range judge.Details {
		identity, err := legacyTaskIdentity(detail)
		if err != nil {
			return fmt.Errorf("judge suite %s: %w", suite, err)
		}
		if _, duplicate := judgeByTask[identity]; duplicate {
			return fmt.Errorf("judge suite %s repeats task %q", suite, identity)
		}
		judgeByTask[identity] = detail
	}
	verifierTasks := make(map[string]struct{}, len(verifier.Details))
	for _, verifierDetail := range verifier.Details {
		taskID, err := legacyTaskIdentity(verifierDetail)
		if err != nil {
			return fmt.Errorf("verifier suite %s: %w", suite, err)
		}
		if _, duplicate := verifierTasks[taskID]; duplicate {
			return fmt.Errorf("verifier suite %s repeats task %q", suite, taskID)
		}
		verifierTasks[taskID] = struct{}{}
		judgeDetail, exists := judgeByTask[taskID]
		if !exists {
			return fmt.Errorf("judge suite %s omits verifier task %q", suite, taskID)
		}
		if err := validateLegacyPair(verifierDetail, judgeDetail); err != nil {
			return fmt.Errorf("suite %s task %q: %w", suite, taskID, err)
		}
		candidate := naturalCandidate{
			Suite: suite, TaskID: taskID, TaskDigest: digestText(suite + "\x00" + taskID),
			VerifierData: verifierDetail, JudgeData: judgeDetail,
		}
		candidate.CaseDigest, err = digestJSON(struct {
			Suite              string  `json:"suite"`
			TaskDigest         string  `json:"task_digest"`
			RewardsDigest      string  `json:"rewards_digest"`
			VerifierSelection  int     `json:"verifier_selection"`
			JudgeSelection     int     `json:"judge_selection"`
			VerifierConfidence float64 `json:"verifier_confidence"`
		}{
			Suite: suite, TaskDigest: candidate.TaskDigest, RewardsDigest: digestIntSlice(verifierDetail.Rewards),
			VerifierSelection: selectedIndex(verifierDetail), JudgeSelection: selectedIndex(judgeDetail),
			VerifierConfidence: selectionConfidence(verifierDetail),
		})
		if err != nil {
			return err
		}
		for _, stratum := range naturalStrata(verifierDetail, judgeDetail, threshold) {
			destination[stratum] = append(destination[stratum], candidate)
		}
	}
	if len(judgeByTask) != len(verifier.Details) {
		return fmt.Errorf("suite %s verifier/judge task counts differ", suite)
	}
	return nil
}

func naturalStrata(verifier, judge legacyDetail, threshold float64) []string {
	reason := strings.ToLower(strings.TrimSpace(pointerString(verifier.SkippedReason)))
	var strata []string
	if verifier.Abstained || reason == "abstention" || reason == "abstained" {
		strata = append(strata, "abstention")
	}
	if verifier.ProviderFailure || reason == "provider_failure" || reason == "provider_error" {
		strata = append(strata, "provider_failure")
	}
	if reason != "" {
		return strata
	}
	selectedReward := *verifier.SelectedReward
	strata = append(strata, "random_control")
	if selectedReward == 1 {
		strata = append(strata, "verifier_correct")
	} else {
		strata = append(strata, "verifier_wrong")
		if selectionConfidence(verifier) >= threshold {
			strata = append(strata, "high_confidence_error")
		}
	}
	if *verifier.SelectedIndex != *judge.SelectedIndex {
		strata = append(strata, "verifier_judge_disagreement")
	}
	if verifier.Rewards[0] != selectedReward {
		strata = append(strata, "baseline_disagreement")
	}
	return strata
}

func readLegacyResult(suite, role, path string) (legacy legacyResult, artifact NaturalSourceArtifact, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return legacyResult{}, NaturalSourceArtifact{}, fmt.Errorf("open natural %s source %q: %w", role, path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close natural %s source %q: %w", role, path, closeErr)
		}
	}()
	raw, err := io.ReadAll(io.LimitReader(file, maximumLegacyResultBytes+1))
	if err != nil {
		return legacyResult{}, NaturalSourceArtifact{}, err
	}
	if len(raw) > maximumLegacyResultBytes {
		return legacyResult{}, NaturalSourceArtifact{}, fmt.Errorf("natural %s source %q exceeds 64 MiB", role, path)
	}
	var result legacyResult
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&result); err != nil {
		return legacyResult{}, NaturalSourceArtifact{}, fmt.Errorf("decode natural %s source %q: %w", role, path, err)
	}
	if len(result.Details) == 0 {
		return legacyResult{}, NaturalSourceArtifact{}, fmt.Errorf("natural %s source %q has no details", role, path)
	}
	return result, NaturalSourceArtifact{Suite: suite, Role: role, Path: path, Digest: digestText(string(raw)), TaskCount: len(result.Details)}, nil
}

func validateLegacyPair(verifier, judge legacyDetail) error {
	if len(verifier.Rewards) < 2 || len(verifier.Rewards) != len(judge.Rewards) {
		return errors.New("verifier/judge reward vectors are absent or differ in length")
	}
	for index := range verifier.Rewards {
		if verifier.Rewards[index] != 0 && verifier.Rewards[index] != 1 || verifier.Rewards[index] != judge.Rewards[index] {
			return errors.New("verifier/judge reward vectors differ or contain non-binary values")
		}
	}
	if pointerString(verifier.SkippedReason) != pointerString(judge.SkippedReason) {
		return errors.New("verifier/judge skipped states differ")
	}
	if verifier.SkippedReason == nil {
		if verifier.SelectedIndex == nil || verifier.SelectedReward == nil || judge.SelectedIndex == nil || judge.SelectedReward == nil || verifier.Selection == nil || judge.Selection == nil {
			return errors.New("eligible verifier/judge result lacks selection fields")
		}
		for _, detail := range []legacyDetail{verifier, judge} {
			if *detail.SelectedIndex < 0 || *detail.SelectedIndex >= len(detail.Rewards) || *detail.SelectedReward != detail.Rewards[*detail.SelectedIndex] {
				return errors.New("selected index and reward are inconsistent")
			}
		}
	}
	return nil
}

func SealNaturalInventory(inventory NaturalInventory) (NaturalInventory, error) {
	inventory.SchemaVersion = NaturalInventorySchemaVersion
	inventory.CanonicalPolicy = CanonicalPolicy
	inventory.Digest = ""
	digest, err := naturalInventoryDigest(inventory)
	if err != nil {
		return NaturalInventory{}, err
	}
	inventory.Digest = digest
	return inventory, inventory.Validate()
}

func (inventory NaturalInventory) Validate() error {
	if inventory.SchemaVersion != NaturalInventorySchemaVersion || inventory.CanonicalPolicy != CanonicalPolicy ||
		!validDigest(inventory.PlanDigest) || !validDigest(inventory.RequestDigest) || !validDigest(inventory.SelectionDigest) ||
		inventory.SourceClass != "development_only" || inventory.TargetCases < 1 || inventory.SelectedCases < 1 ||
		!contains([]string{"complete", "incomplete"}, inventory.Status) || missing(inventory.ReproducibilityGap) {
		return errors.New("natural inventory identity, source class, counts, status, or limitation is invalid")
	}
	if inventory.SelectedCases != len(inventory.Selections) || inventory.Status == "complete" != (len(inventory.Shortfalls) == 0) {
		return errors.New("natural inventory status, selections, and shortfalls disagree")
	}
	selectedTotal := 0
	candidateByStratum := make(map[string]int, len(inventory.CandidateCounts))
	for index, count := range inventory.CandidateCounts {
		if missing(count.ID) || count.Count < 0 || index > 0 && inventory.CandidateCounts[index-1].ID >= count.ID {
			return errors.New("natural candidate counts must be nonnegative, unique, and sorted")
		}
		candidateByStratum[count.ID] = count.Count
	}
	selectedByStratum := make(map[string]int, len(inventory.SelectedCounts))
	for index, count := range inventory.SelectedCounts {
		if missing(count.ID) || count.Count < 0 || index > 0 && inventory.SelectedCounts[index-1].ID >= count.ID {
			return errors.New("natural selected counts must be nonnegative, unique, and sorted")
		}
		if candidate, exists := candidateByStratum[count.ID]; !exists || count.Count > candidate {
			return errors.New("natural selected counts must use candidate strata and cannot exceed eligible counts")
		}
		selectedByStratum[count.ID] = count.Count
		selectedTotal += count.Count
	}
	if selectedTotal != inventory.SelectedCases || len(candidateByStratum) != len(selectedByStratum) {
		return errors.New("natural selected counts do not match selected cases")
	}
	selectionCounts := make(map[string]int, len(selectedByStratum))
	seenTaskGroups := make(map[string]struct{}, len(inventory.Selections))
	identities := make([]string, len(inventory.Selections))
	for index, selection := range inventory.Selections {
		if missing(selection.Stratum, selection.Suite) || selection.Rank < 1 || !validDigest(selection.TaskGroupDigest) || !validDigest(selection.CaseDigest) ||
			index > 0 && (inventory.Selections[index-1].Stratum > selection.Stratum || inventory.Selections[index-1].Stratum == selection.Stratum && inventory.Selections[index-1].Rank >= selection.Rank) {
			return errors.New("natural selections must be valid and sorted by stratum and rank")
		}
		selectionCounts[selection.Stratum]++
		if _, exists := selectedByStratum[selection.Stratum]; !exists || selection.Rank != selectionCounts[selection.Stratum] {
			return errors.New("natural selection ranks must be contiguous and use declared strata")
		}
		if _, duplicate := seenTaskGroups[selection.TaskGroupDigest]; duplicate {
			return errors.New("natural selections cannot reuse a task group across strata")
		}
		seenTaskGroups[selection.TaskGroupDigest] = struct{}{}
		identities[index] = selection.Stratum + "\x00" + selection.CaseDigest
	}
	for stratum, selected := range selectedByStratum {
		if selectionCounts[stratum] != selected {
			return errors.New("natural selected counts do not reproduce the sealed selections")
		}
	}
	if inventory.SelectionDigest != digestText(joinWithNUL(identities)) {
		return errors.New("natural inventory selection digest is invalid")
	}
	for index, shortfall := range inventory.Shortfalls {
		if missing(shortfall.Stratum) || shortfall.Required < 1 || shortfall.Eligible < 0 || shortfall.Selected < 0 || shortfall.Selected > shortfall.Eligible || shortfall.Selected >= shortfall.Required ||
			index > 0 && inventory.Shortfalls[index-1].Stratum >= shortfall.Stratum {
			return errors.New("natural shortfalls must be valid, unique, and sorted")
		}
		if candidateByStratum[shortfall.Stratum] != shortfall.Eligible || selectedByStratum[shortfall.Stratum] != shortfall.Selected {
			return errors.New("natural shortfalls do not reproduce candidate and selected counts")
		}
	}
	for index, artifact := range inventory.SourceArtifacts {
		if missing(artifact.Suite, artifact.Role, artifact.Path) || !contains([]string{"judge", "verifier"}, artifact.Role) || !validDigest(artifact.Digest) || artifact.TaskCount < 1 ||
			index > 0 && inventory.SourceArtifacts[index-1].Suite+"\x00"+inventory.SourceArtifacts[index-1].Role >= artifact.Suite+"\x00"+artifact.Role {
			return errors.New("natural source artifacts must be valid, unique, and sorted")
		}
	}
	expected, err := naturalInventoryDigest(inventory)
	if err != nil || inventory.Digest != expected {
		return errors.New("natural inventory digest is invalid")
	}
	return nil
}

func DecodeNaturalInventoryRequest(reader io.Reader) (NaturalInventoryRequest, error) {
	var value NaturalInventoryRequest
	if err := decodeStrict(reader, &value); err != nil {
		return NaturalInventoryRequest{}, fmt.Errorf("decode natural inventory request: %w", err)
	}
	return value, value.Validate()
}

func DecodeNaturalInventory(reader io.Reader) (NaturalInventory, error) {
	var value NaturalInventory
	if err := decodeStrict(reader, &value); err != nil {
		return NaturalInventory{}, fmt.Errorf("decode natural inventory: %w", err)
	}
	return value, value.Validate()
}

func legacyTaskIdentity(detail legacyDetail) (string, error) {
	if detail.TaskName != "" && detail.InstanceID != "" || detail.TaskName == "" && detail.InstanceID == "" {
		return "", errors.New("legacy result must contain exactly one task identity")
	}
	if detail.TaskName != "" {
		return detail.TaskName, nil
	}
	return detail.InstanceID, nil
}

func selectedIndex(detail legacyDetail) int {
	if detail.SelectedIndex == nil {
		return -1
	}
	return *detail.SelectedIndex
}

func selectionConfidence(detail legacyDetail) float64 {
	if detail.Selection == nil {
		return 0
	}
	return detail.Selection.Confidence
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func digestIntSlice(values []int) string {
	encoded, _ := json.Marshal(values)
	return digestText(string(encoded))
}

func sampleCountsIncludingZero(values map[string]int, keys []string) []SampleCount {
	result := make([]SampleCount, 0, len(keys))
	for _, key := range keys {
		result = append(result, SampleCount{ID: key, Count: values[key]})
	}
	return result
}

func naturalRequestDigest(value NaturalInventoryRequest) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}

func naturalInventoryDigest(value NaturalInventory) (string, error) {
	value.Digest = ""
	return digestJSON(value)
}
