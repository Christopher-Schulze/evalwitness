// Package cost computes USD per call from per-session rates supplied by the
// caller. No model database, no embedded pricing. Callers configure rates
// for their chosen model+provider via env or library API; if rates are zero,
// est_cost_usd is reported as nil (unknown).
package cost

type Calculator struct {
	inputUSDPerMillion  float64
	outputUSDPerMillion float64
	cachedUSDPerMillion float64
	subscription        bool
	enabled             bool
}

// New constructs a Calculator. Pass per-million rates for the configured
// model. When subscription=true, all costs collapse to 0 regardless of rates
// (use this for fixed-fee plans such as an OpenCode Go subscription or a free
// tier). When all rates are zero and subscription=false, the calculator
// reports unknown cost (Estimate returns nil).
func New(inputRate, cachedRate, outputRate float64, subscription bool) *Calculator {
	return &Calculator{
		inputUSDPerMillion:  inputRate,
		cachedUSDPerMillion: cachedRate,
		outputUSDPerMillion: outputRate,
		subscription:        subscription,
		enabled:             subscription || inputRate > 0 || outputRate > 0,
	}
}

// Estimate returns the USD cost for one call given token counts. Returns
// nil if rates are not configured.
func (c *Calculator) Estimate(inputTokens, outputTokens, cachedTokens int) *float64 {
	if c == nil || !c.enabled {
		return nil
	}
	if c.subscription {
		zero := 0.0
		return &zero
	}
	billable := inputTokens - cachedTokens
	if billable < 0 {
		billable = 0
	}
	cost := float64(billable) * c.inputUSDPerMillion / 1_000_000
	cost += float64(cachedTokens) * c.cachedUSDPerMillion / 1_000_000
	cost += float64(outputTokens) * c.outputUSDPerMillion / 1_000_000
	return &cost
}

// EstimatePromptCost approximates the cost of a single call given only the
// prompt token estimate. Used for pre-call cost-cap enforcement.
func (c *Calculator) EstimatePromptCost(promptTokens, expectedOutputTokens int) *float64 {
	return c.Estimate(promptTokens, expectedOutputTokens, 0)
}
