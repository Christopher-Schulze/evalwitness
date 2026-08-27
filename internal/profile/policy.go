package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Policy is a strict versioned content-addressed requirement set.
type Policy struct {
	Version      string            `json:"version"`
	Requirements map[string]string `json:"requirements"` // dimension ID -> required status
	Digest       string            `json:"digest"`
}

// NewPolicy creates a strict policy; unknown dimensions default to failure unless explicitly permitted.
func NewPolicy(version string, reqs map[string]string) (Policy, error) {
	if version == "" {
		return Policy{}, fmt.Errorf("policy: version empty")
	}
	if len(reqs) == 0 {
		return Policy{}, fmt.Errorf("policy: requirements empty")
	}
	p := Policy{Version: version, Requirements: reqs}
	b, err := json.Marshal(p.Requirements)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: marshal: %w", err)
	}
	h := sha256.Sum256(append([]byte(version), b...))
	p.Digest = hex.EncodeToString(h[:])
	return p, nil
}

// DigestValue recomputes the policy content digest exactly as NewPolicy seals
// it: sha256 over version plus canonical requirements JSON. It lets file-loaded
// policies prove their identity and lets callers pin a digest.
func (pol Policy) DigestValue() (string, error) {
	if pol.Version == "" {
		return "", fmt.Errorf("policy: version empty")
	}
	if len(pol.Requirements) == 0 {
		return "", fmt.Errorf("policy: requirements empty")
	}
	b, err := json.Marshal(pol.Requirements)
	if err != nil {
		return "", fmt.Errorf("policy: marshal: %w", err)
	}
	sum := sha256.Sum256(append([]byte(pol.Version), b...))
	return hex.EncodeToString(sum[:]), nil
}

// Evaluate checks profile against policy; every failed/unknown alongside passes.
func Evaluate(p Profile, policy Policy) (bool, []string) {
	var fails []string
	m := make(map[string]Dimension, len(p.Dimensions))
	for _, d := range p.Dimensions {
		m[d.ID] = d
	}
	for id, want := range policy.Requirements {
		got, ok := m[id]
		if !ok {
			fails = append(fails, fmt.Sprintf("%s missing", id))
			continue
		}
		if string(got.Status) != want {
			fails = append(fails, fmt.Sprintf("%s want %s got %s", id, want, got.Status))
		}
	}
	return len(fails) == 0, fails
}
