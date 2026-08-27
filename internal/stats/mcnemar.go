// Package stats holds the exact tests this repository uses to decide whether a
// measured difference is a result or noise. Every comparison here pairs two
// arms over one task set with no sampling between them, so the paired tests are
// the correct instrument and a difference of two scores is not.
package stats

import "math"

// McNemarExact returns the two-sided exact p-value for a paired binary
// comparison, where b is the count of tasks only the first arm solved and c the
// count only the second solved. Concordant tasks carry no information about
// which arm is better and are deliberately not arguments.
//
// The null hypothesis is that each disagreement is a fair coin, so the p-value
// is the probability of a split at least this lopsided. With no disagreements
// there is nothing to distinguish and the result is 1.
func McNemarExact(b, c int) float64 {
	if b < 0 || c < 0 {
		return 1
	}
	n := b + c
	if n == 0 {
		return 1
	}
	k := min(b, c)
	// Sum the lower tail in log space. n stays small here, but binomial
	// coefficients overflow float64 well before the exact test stops being the
	// right choice, and a silent Inf would read as a p-value of 1.
	logTail := math.Inf(-1)
	logDenominator := float64(n) * math.Ln2
	for i := 0; i <= k; i++ {
		logTail = logSumExp(logTail, logBinomial(n, i))
	}
	p := 2 * math.Exp(logTail-logDenominator)
	return math.Min(1, p)
}

// SmallestSignificantSplit returns the smallest b, out of n discordant pairs,
// whose two-sided exact p-value falls below alpha. It reports false when no
// split of n reaches significance, which is the case that matters: below six
// discordant pairs no outcome whatsoever is significant at 0.05, so a
// comparison that small cannot produce a result however it lands.
func SmallestSignificantSplit(n int, alpha float64) (int, bool) {
	if n <= 0 {
		return 0, false
	}
	for b := (n + 1) / 2; b <= n; b++ {
		if McNemarExact(b, n-b) < alpha {
			return b, true
		}
	}
	return 0, false
}

func logBinomial(n, k int) float64 {
	if k < 0 || k > n {
		return math.Inf(-1)
	}
	a, _ := math.Lgamma(float64(n) + 1)
	b, _ := math.Lgamma(float64(k) + 1)
	c, _ := math.Lgamma(float64(n-k) + 1)
	return a - b - c
}

func logSumExp(a, b float64) float64 {
	switch {
	case math.IsInf(a, -1):
		return b
	case math.IsInf(b, -1):
		return a
	}
	if a < b {
		a, b = b, a
	}
	return a + math.Log1p(math.Exp(b-a))
}
