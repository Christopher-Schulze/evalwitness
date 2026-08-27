package main

import (
	"errors"
	"flag"
	"time"
)

type verifyLiveFlags struct {
	authorize       *string
	maxCalls        *int
	maxAttempts     *int
	maxInputTokens  *int
	maxOutputTokens *int
	maxDuration     *time.Duration
	maxConcurrent   *int
	maxCostUSD      *float64
}

func addVerifyLiveFlags(flags *flag.FlagSet) verifyLiveFlags {
	return verifyLiveFlags{
		authorize:       flags.String("authorize", "", "execute only when this digest matches the printed plan"),
		maxCalls:        flags.Int("max-calls", 0, "hard logical-call limit (0 derives exact worst case)"),
		maxAttempts:     flags.Int("max-attempts", 0, "hard HTTP-attempt limit (0 derives calls and retries)"),
		maxInputTokens:  flags.Int("max-input-tokens", 0, "hard reserved input-token limit (0 derives request set and attempts)"),
		maxOutputTokens: flags.Int("max-output-tokens", 0, "hard reserved output-token limit (0 derives request ceiling and attempts)"),
		maxDuration:     flags.Duration("max-duration", 0, "hard total deadline (0 derives timeout, attempts, and concurrency)"),
		maxConcurrent:   flags.Int("max-concurrent", 0, "hard concurrent-attempt limit (0 uses configured workers)"),
		maxCostUSD:      flags.Float64("max-cost-usd", 0, "optional hard monetary limit"),
	}
}

func validateVerifyLiveFlags(flags verifyLiveFlags) error {
	if *flags.maxCalls < 0 || *flags.maxAttempts < 0 || *flags.maxInputTokens < 0 || *flags.maxOutputTokens < 0 || *flags.maxConcurrent < 0 || *flags.maxCostUSD < 0 || *flags.maxDuration < 0 {
		return errors.New("verify live limits must be non-negative")
	}
	return nil
}
