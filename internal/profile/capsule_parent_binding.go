package profile

import "fmt"

// CapsuleParent binds a profile to its parent capsule digest.
type CapsuleParent struct {
	ProfileDigest string `json:"profile_digest"`
	CapsuleDigest string `json:"capsule_digest"`
}

// BindCapsuleParent creates a binding; fails if digests empty.
func BindCapsuleParent(profileDigest, capsuleDigest string) (CapsuleParent, error) {
	if profileDigest == "" || capsuleDigest == "" {
		return CapsuleParent{}, fmt.Errorf("parent: digest empty")
	}
	return CapsuleParent{ProfileDigest: profileDigest, CapsuleDigest: capsuleDigest}, nil
}
