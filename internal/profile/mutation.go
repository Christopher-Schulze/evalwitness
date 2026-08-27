package profile

// MutationCheck verifies that deleting a dimension fails a policy requiring it.
// Returns true only if deletion is correctly detected as failure.
func MutationCheck(p Profile, requiredID string) bool {
	if len(p.Dimensions) == 0 {
		return false
	}
	// Build mutated profile without requiredID
	var mutated []Dimension
	for _, d := range p.Dimensions {
		if d.ID != requiredID {
			mutated = append(mutated, d)
		}
	}
	if len(mutated) == len(p.Dimensions) {
		return false // requiredID not found
	}
	mutatedProfile, err := Build(p.Identity, p.ProtocolVersion, p.RouteScope, mutated)
	if err != nil {
		return true // Build fails as expected when required dimension missing and policy would fail
	}
	pol, err := NewPolicy("test", map[string]string{requiredID: "measured"})
	if err != nil {
		return false
	}
	ok, _ := Evaluate(mutatedProfile, pol)
	return !ok // deletion should cause policy to fail
}
