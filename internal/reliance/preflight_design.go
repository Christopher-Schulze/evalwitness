package reliance

import "errors"

const (
	walsh32SumFreeSetsAudited           = 135_408
	walsh32QualifyingInteractionLayouts = 0
)

func auditReferenceWalshDesign(preregistration Preregistration) (WalshAliasAudit, error) {
	masks := canonicalReferenceMasks()
	maskByFactor := make(map[string]uint64, len(masks))
	pairCounts := make(map[uint64]int)
	for _, factor := range masks {
		maskByFactor[factor.FactorID] = factor.Mask
	}
	for left := 0; left < len(masks); left++ {
		for right := left + 1; right < len(masks); right++ {
			pairCounts[masks[left].Mask^masks[right].Mask]++
		}
	}
	mainEffectsClear := true
	for _, factor := range masks {
		if pairCounts[factor.Mask] != 0 {
			mainEffectsClear = false
		}
	}
	interactionsUnique := true
	for _, interaction := range preregistration.Interactions {
		mask := uint64(0)
		for _, factorID := range interaction.Terms {
			factorMask, found := maskByFactor[factorID]
			if !found {
				return WalshAliasAudit{}, errors.New("preregistered interaction references a factor absent from the Walsh design")
			}
			mask ^= factorMask
		}
		if pairCounts[mask] != 1 {
			interactionsUnique = false
		}
	}
	return WalshAliasAudit{
		Factors: len(masks), DeclaredInteractions: len(preregistration.Interactions), SelectedRuns: ReferenceCellsPerTask,
		MainEffectsClearOfTwoFactorTerms: mainEffectsClear, DeclaredInteractionsUnique: interactionsUnique,
		Candidates: walshDesignCandidates(),
	}, nil
}

func walshDesignCandidates() []WalshCandidateAudit {
	return []WalshCandidateAudit{
		{Runs: 16, Status: "rejected", Reason: "eleven main effects exceed the eight-element sum-free bound, so at least one main effect aliases with a two-factor term"},
		{Runs: 32, Status: "rejected", Reason: "exhaustive sum-free search found no layout with the required three-edge interaction star and disjoint interaction edge all unique", SumFreeSetsAudited: walsh32SumFreeSetsAudited, QualifyingInteractionLayouts: walsh32QualifyingInteractionLayouts},
		{Runs: 64, Status: "selected", Reason: "all main effects are clear of every two-factor term and each declared interaction is the unique pair on its Walsh column", QualifyingInteractionLayouts: 1},
	}
}

func searchThirtyTwoRunLayouts() (int, int) {
	setsAudited := 0
	qualifying := 0
	var visit func([]uint64, []uint64)
	visit = func(chosen, candidates []uint64) {
		needed := 11 - len(chosen)
		if len(candidates) < needed {
			return
		}
		if needed == 0 {
			setsAudited++
			if hasRequiredInteractionLayout(chosen) {
				qualifying++
			}
			return
		}
		for len(candidates) >= needed {
			next := candidates[0]
			rest := candidates[1:]
			selected := append(append([]uint64(nil), chosen...), next)
			admissible := make([]uint64, 0, len(rest))
			for _, candidate := range rest {
				if preservesSumFree(selected, candidate) {
					admissible = append(admissible, candidate)
				}
			}
			visit(selected, admissible)
			candidates = rest
		}
	}
	candidates := make([]uint64, 31)
	for index := range candidates {
		candidates[index] = uint64(index + 1)
	}
	visit(nil, candidates)
	return setsAudited, qualifying
}

func preservesSumFree(selected []uint64, candidate uint64) bool {
	for _, existing := range selected {
		for _, member := range selected {
			if candidate^existing == member {
				return false
			}
		}
	}
	return true
}

func hasRequiredInteractionLayout(values []uint64) bool {
	pairCounts := make(map[uint64]int)
	for left := 0; left < len(values); left++ {
		for right := left + 1; right < len(values); right++ {
			pairCounts[values[left]^values[right]]++
		}
	}
	unique := make(map[[2]uint64]struct{})
	for left := 0; left < len(values); left++ {
		for right := left + 1; right < len(values); right++ {
			if pairCounts[values[left]^values[right]] == 1 {
				unique[[2]uint64{values[left], values[right]}] = struct{}{}
			}
		}
	}
	for _, center := range values {
		neighbors := make([]uint64, 0)
		for _, candidate := range values {
			if candidate != center && uniquePair(unique, center, candidate) {
				neighbors = append(neighbors, candidate)
			}
		}
		for first := 0; first < len(neighbors); first++ {
			for second := first + 1; second < len(neighbors); second++ {
				for third := second + 1; third < len(neighbors); third++ {
					used := map[uint64]struct{}{center: {}, neighbors[first]: {}, neighbors[second]: {}, neighbors[third]: {}}
					for pair := range unique {
						if _, leftUsed := used[pair[0]]; leftUsed {
							continue
						}
						if _, rightUsed := used[pair[1]]; !rightUsed {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func uniquePair(pairs map[[2]uint64]struct{}, left, right uint64) bool {
	if left > right {
		left, right = right, left
	}
	_, found := pairs[[2]uint64{left, right}]
	return found
}
