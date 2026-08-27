package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// Pilot readiness checks capsule without provider call.
type Pilot struct {
	CapsulePath string `json:"capsule_path"`
	Verified    bool   `json:"verified"`
}

// IsReady checks pilot readiness offline; no network. Pilot's capsule file hash must match a verified matrix entry.
func IsReady(p Pilot, m Matrix) error {
	if p.CapsulePath == "" {
		return fmt.Errorf("pilot: capsule_path empty")
	}
	b, err := os.ReadFile(p.CapsulePath)
	if err != nil {
		return fmt.Errorf("pilot: read capsule: %w", err)
	}
	h := sha256.Sum256(b)
	digest := hex.EncodeToString(h[:])
	for _, e := range m.Entries {
		if e.Digest == digest && e.Verified {
			return nil
		}
	}
	return fmt.Errorf("pilot: capsule %s not in verified matrix", digest)
}
