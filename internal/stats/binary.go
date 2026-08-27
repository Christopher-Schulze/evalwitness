package stats

// DecidableBinary reports whether choosing a different candidate can change a
// binary task outcome. Values must be binary and at least one zero and one one
// must be present.
func DecidableBinary(outcomes []int) bool {
	if len(outcomes) < 2 {
		return false
	}
	seenZero := false
	seenOne := false
	for _, outcome := range outcomes {
		switch outcome {
		case 0:
			seenZero = true
		case 1:
			seenOne = true
		default:
			return false
		}
	}
	return seenZero && seenOne
}

// CountDecidableBinary counts decision-informative tasks under the same rule
// used by evaluation and statistical analysis.
func CountDecidableBinary(tasks [][]int) int {
	count := 0
	for _, outcomes := range tasks {
		if DecidableBinary(outcomes) {
			count++
		}
	}
	return count
}
