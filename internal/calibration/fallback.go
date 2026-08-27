package calibration

import "fmt"

// BudgetReserver is the run-budget port used to cost fallbacks.
type BudgetReserver interface {
	Reserve(estimatedInputTokens int, estimatedCostUSD float64) error
}

// FallbackKind enumerates explicit fallback arms; no implicit recovery.
type FallbackKind string

const (
	FallbackNone     FallbackKind = "none"
	FallbackJudge    FallbackKind = "judge"
	FallbackHuman    FallbackKind = "human_review_handoff"
	FallbackNoAction FallbackKind = "no_action"
)

// FallbackPolicy is an explicit arm with cost included in run budget.
type FallbackPolicy struct {
	Kind FallbackKind `json:"kind"`
	// Cost is included in run budget; human review is handoff, not zero.
	CostCalls     int `json:"cost_calls"`
	CostTokens    int `json:"cost_tokens"`
	CostLatencyMs int `json:"cost_latency_ms"`
}

func ValidateFallbackPolicy(policy FallbackPolicy) error {
	switch policy.Kind {
	case FallbackNone, FallbackNoAction:
		if policy.CostCalls != 0 || policy.CostTokens != 0 {
			return fmt.Errorf("calibration: %s fallback cannot hide a call or token cost", policy.Kind)
		}
		return nil
	case FallbackJudge, FallbackHuman:
		if policy.CostCalls < 1 {
			return fmt.Errorf("calibration: %s fallback must reserve at least one call", policy.Kind)
		}
		if policy.CostTokens < 0 || policy.CostLatencyMs < 0 {
			return fmt.Errorf("calibration: fallback costs must be non-negative")
		}
		return nil
	default:
		return fmt.Errorf("calibration: unknown fallback kind %q", policy.Kind)
	}
}

// SelectWithFallback returns SelectiveDecision with fallback when abstain.
func SelectWithFallback(m SelectiveMetrics, threshold float64, estimate float64, applicability Applicability, fallback FallbackPolicy) SelectiveDecision {
	_ = m
	abstain := false
	reason := ""
	if !applicability.Applicable {
		abstain = true
		reason = applicability.Reason
	} else if estimate < threshold {
		abstain = true
		reason = "below_threshold"
	}
	if !abstain {
		return SelectiveDecision{
			Select: true, EstimatedCorrect: estimate, Threshold: threshold, Applicability: applicability,
			Fallback: FallbackPolicy{Kind: FallbackNone},
		}
	}
	if err := ValidateFallbackPolicy(fallback); err != nil {
		return SelectiveDecision{
			Select: false, AbstainReason: "invalid_fallback", EstimatedCorrect: estimate, Threshold: threshold,
			Applicability: applicability, Fallback: FallbackPolicy{Kind: FallbackNone},
		}
	}
	return SelectiveDecision{
		Select: false, AbstainReason: reason, EstimatedCorrect: estimate, Threshold: threshold,
		Applicability: applicability, Fallback: fallback,
	}
}

// ChargeFallback reserves explicit fallback calls and tokens on the run budget.
func ChargeFallback(budget BudgetReserver, policy FallbackPolicy) error {
	if err := ValidateFallbackPolicy(policy); err != nil {
		return err
	}
	if policy.CostCalls == 0 {
		return nil
	}
	if budget == nil {
		return fmt.Errorf("calibration: fallback cost cannot be reserved without a budget")
	}
	tokensPerCall := policy.CostTokens / policy.CostCalls
	remainder := policy.CostTokens % policy.CostCalls
	for i := 0; i < policy.CostCalls; i++ {
		tokens := tokensPerCall
		if i == 0 {
			tokens += remainder
		}
		if err := budget.Reserve(tokens, 0); err != nil {
			return err
		}
	}
	return nil
}

func AccountFallback(budget BudgetReserver, decision *SelectiveDecision) error {
	if decision == nil || decision.Select {
		return nil
	}
	if err := ChargeFallback(budget, decision.Fallback); err != nil {
		return err
	}
	decision.FallbackAccounted = decision.Fallback.CostCalls > 0
	return nil
}
