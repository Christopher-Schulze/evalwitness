package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// PolicyChain links policy digests for audit trail.
type PolicyChain struct {
	Previous string `json:"previous"`
	Current  string `json:"current"`
	Digest   string `json:"digest"`
}

// ChainPolicy creates a chained digest.
func ChainPolicy(prev string, pol Policy) (PolicyChain, error) {
	b, err := json.Marshal(pol)
	if err != nil {
		return PolicyChain{}, fmt.Errorf("chain: marshal: %w", err)
	}
	h := sha256.Sum256(append([]byte(prev), b...))
	return PolicyChain{Previous: prev, Current: pol.Digest, Digest: hex.EncodeToString(h[:])}, nil
}
