package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/Christopher-Schulze/evalwitness/internal/conformance"
	"github.com/Christopher-Schulze/evalwitness/internal/mode"
	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type RequestProfile struct {
	ProviderID           string
	BaseURL              string
	RequestedModel       string
	ThinkingMode         string
	Temperature          float64
	Seed                 *int
	MaxOutputTokens      int
	TopLogprobs          int
	Stream               bool
	MultiCriterionBundle bool
	CritiqueThenScore    bool
}

func (profile RequestProfile) validate() error {
	if profile.ProviderID == "" || profile.BaseURL == "" || profile.RequestedModel == "" {
		return errors.New("verification request profile requires provider, base URL, and model")
	}
	if profile.MaxOutputTokens <= 0 {
		return errors.New("verification request profile max output tokens must be positive")
	}
	if profile.TopLogprobs < verifier.MinimumVerifierTopK {
		return fmt.Errorf("verification request profile top logprobs %d is below strict minimum %d", profile.TopLogprobs, verifier.MinimumVerifierTopK)
	}
	return nil
}

func buildRequestPlan(profile RequestProfile, input Input, prepared []preprocess.Result) (RequestPlan, error) {
	if err := profile.validate(); err != nil {
		return RequestPlan{}, err
	}
	if input.Policy.BiasMitigation == "adaptive" && input.Policy.SelectionStrategy != "absolute" {
		callsPerOrder := len(input.Criteria)
		if profile.MultiCriterionBundle && len(input.Criteria) > 1 {
			callsPerOrder = 1
		}
		if callsPerOrder > input.Policy.MaxPairCalls {
			return RequestPlan{}, fmt.Errorf(
				"adaptive max_pair_calls=%d cannot fit one order requiring %d provider calls",
				input.Policy.MaxPairCalls, callsPerOrder,
			)
		}
	}
	type promptSpec struct {
		criterionID string
		prompt      string
		tags        []string
		prepared    []preprocess.Result
	}
	specs := make([]promptSpec, 0)
	appendAbsolute := func(trajectory preprocess.Result) {
		if profile.MultiCriterionBundle && len(input.Criteria) > 1 {
			prompt, tags := verifier.PromptAbsoluteBundled(input.Task, trajectory.Text, input.Criteria, verifier.PromptOptions{CritiqueThenScore: profile.CritiqueThenScore})
			specs = append(specs, promptSpec{"bundle", prompt, verifier.AllTags(tags), []preprocess.Result{trajectory}})
			return
		}
		for _, criterion := range input.Criteria {
			prompt, tags := verifier.PromptAbsolute(input.Task, trajectory.Text, criterion, verifier.PromptOptions{CritiqueThenScore: profile.CritiqueThenScore})
			specs = append(specs, promptSpec{criterion.ID + "_abs", prompt, tags, []preprocess.Result{trajectory}})
		}
	}
	appendPair := func(left, right preprocess.Result) {
		if profile.MultiCriterionBundle && len(input.Criteria) > 1 {
			prompt, tags := verifier.PromptPairwiseBundled(input.Task, left.Text, right.Text, input.Criteria, verifier.PromptOptions{CritiqueThenScore: profile.CritiqueThenScore})
			specs = append(specs, promptSpec{"bundle", prompt, verifier.AllTags(tags), []preprocess.Result{left, right}})
			return
		}
		for _, criterion := range input.Criteria {
			prompt, tags := verifier.PromptPairwise(input.Task, left.Text, right.Text, criterion, verifier.PromptOptions{CritiqueThenScore: profile.CritiqueThenScore})
			specs = append(specs, promptSpec{criterion.ID, prompt, tags, []preprocess.Result{left, right}})
		}
	}
	appendJointAbsolute := func(trajectories []preprocess.Result) {
		texts := make([]string, len(trajectories))
		for index, trajectory := range trajectories {
			texts[index] = trajectory.Text
		}
		prompt, tags := verifier.PromptJointAbsolute(input.Task, texts, input.Criteria, verifier.PromptOptions{CritiqueThenScore: profile.CritiqueThenScore})
		specs = append(specs, promptSpec{mode.JointAbsoluteCriterionID(input.Criteria), prompt, verifier.AllTags(tags), trajectories})
	}
	switch {
	case input.Mode == ModeAbsolute:
		appendAbsolute(prepared[0])
	case input.Mode == ModePairwise && input.Policy.SelectionStrategy == "joint_absolute":
		appendJointAbsolute(prepared)
	case input.Mode == ModePairwise && input.Policy.SelectionStrategy == "absolute":
		for _, trajectory := range prepared {
			appendAbsolute(trajectory)
		}
	case input.Mode == ModeDelta:
		appendPair(prepared[0], prepared[1])
		if input.Policy.BiasMitigation == "both" || input.Policy.BiasMitigation == "adaptive" {
			appendPair(prepared[1], prepared[0])
		}
	case input.Mode == ModePairwise:
		for left := 0; left < len(prepared); left++ {
			for right := left + 1; right < len(prepared); right++ {
				appendPair(prepared[left], prepared[right])
				if input.Policy.BiasMitigation == "both" || input.Policy.BiasMitigation == "adaptive" {
					appendPair(prepared[right], prepared[left])
				}
			}
		}
	}
	plan := RequestPlan{MaximumOutputTokens: profile.MaxOutputTokens, WorstLogicalCalls: worstLogicalCalls(input, profile.MultiCriterionBundle)}
	baseLineage, err := baseRequestLineage(input)
	if err != nil {
		return RequestPlan{}, err
	}
	fingerprintSet := make(map[string]struct{})
	contractSet := make(map[string]struct{})
	for _, spec := range specs {
		topLogprobs := profile.TopLogprobs
		if input.Policy.Evidence == EvidenceExplicitJudge {
			topLogprobs = 0
		}
		request, err := provider.NewRequestEnvelope(provider.RequestOptions{
			ProviderID: profile.ProviderID, BaseURL: profile.BaseURL, RequestedModel: profile.RequestedModel,
			ThinkingMode: profile.ThinkingMode, Messages: []provider.Message{{Role: "user", Content: spec.prompt}},
			Temperature: profile.Temperature, Seed: profile.Seed, MaxOutputTokens: profile.MaxOutputTokens,
			Logprobs: topLogprobs > 0, TopLogprobs: topLogprobs, ScoreTags: spec.tags,
			ResponseFormat: provider.ResponseFormatText, Stream: profile.Stream && len(spec.tags) > 0,
			EvidenceBindings: mode.EvidenceBindings(preprocess.AccountingSummaries(spec.prepared...)),
			Lineage: mode.CompleteRequestLineage(
				baseLineage, spec.criterionID, "plan", input.Entrypoint, preprocess.AccountingSummaries(spec.prepared...),
			),
		})
		if err != nil {
			return RequestPlan{}, fmt.Errorf("build verification request plan: %w", err)
		}
		fingerprint, err := request.Fingerprint()
		if err != nil {
			return RequestPlan{}, err
		}
		fingerprintSet[string(fingerprint)] = struct{}{}
		contractSet[conformance.CapabilityContractDigest(request)] = struct{}{}
		plan.Requests = append(plan.Requests, request)
		if tokens := preprocess.EstimateTokens(spec.prompt); tokens > plan.MaximumInputTokens {
			plan.MaximumInputTokens = tokens
		}
	}
	plan.Fingerprints = sortedKeys(fingerprintSet)
	plan.SetFingerprint = digestOrderedStrings(plan.Fingerprints)
	plan.ContractDigest = digestOrderedStrings(sortedKeys(contractSet))
	return plan, nil
}

func baseRequestLineage(input Input) (provider.RequestLineage, error) {
	policyBytes, err := json.Marshal(input.Policy)
	if err != nil {
		return provider.RequestLineage{}, fmt.Errorf("encode verification decision policy lineage: %w", err)
	}
	return provider.RequestLineage{
		AuditCaseID: input.Lineage.AuditCaseID, MutationID: input.Lineage.TransformationID,
		StudyCellID: input.Lineage.StudyCellID, PolicyHash: preprocess.Hash(string(policyBytes)),
	}, nil
}

func worstLogicalCalls(input Input, bundled bool) int {
	perOrder := len(input.Criteria)
	if bundled && len(input.Criteria) > 1 {
		perOrder = 1
	}
	maxReps := input.Policy.NReps
	if input.Policy.UseSPRT && input.Policy.SPRTParams.MaxReps > maxReps {
		maxReps = input.Policy.SPRTParams.MaxReps
	}
	if input.Mode == ModePairwise && input.Policy.SelectionStrategy == "joint_absolute" {
		return maxReps
	}
	if input.Mode == ModeAbsolute || input.Mode == ModePairwise && input.Policy.SelectionStrategy == "absolute" {
		return len(input.Trajectories) * perOrder * maxReps
	}
	if input.Policy.BiasMitigation == "adaptive" {
		perOrder = input.Policy.MaxPairCalls
	} else {
		perOrder *= maxReps
		if input.Policy.BiasMitigation == "both" {
			perOrder *= 2
		}
	}
	pairs := 1
	if input.Mode == ModePairwise {
		pairs = len(input.Trajectories) * (len(input.Trajectories) - 1) / 2
		if input.Policy.SingleElim {
			pairs = len(input.Trajectories) - 1
		}
	}
	return pairs * perOrder
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func digestOrderedStrings(values []string) string {
	hash := sha256.New()
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
