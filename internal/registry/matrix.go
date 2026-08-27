package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
)

// Entry is a capsule with provider and digest.
type Entry struct {
	Provider string `json:"provider"`
	Digest   string `json:"digest"`
	Verified bool   `json:"verified"`
}

// Matrix holds entries without ranking; sorted by digest for determinism.
type Matrix struct {
	Entries []Entry `json:"entries"`
}

// Add inserts entry and keeps deterministic order; no ranking computed.
func (m *Matrix) Add(e Entry) {
	m.Entries = append(m.Entries, e)
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Digest < m.Entries[j].Digest })
}

// Verify checks digest is hex SHA256, hashes capsule file, compares, and marks verified.
// capsulePath is the file whose SHA256 must equal e.Digest. No ranking.
func Verify(e Entry, capsulePath string) (Entry, error) {
	if len(e.Digest) != 64 {
		return Entry{}, fmt.Errorf("registry: digest length")
	}
	if _, err := hex.DecodeString(e.Digest); err != nil {
		return Entry{}, fmt.Errorf("registry: digest hex: %w", err)
	}
	b, err := os.ReadFile(capsulePath)
	if err != nil {
		return Entry{}, fmt.Errorf("registry: read capsule: %w", err)
	}
	h := sha256.Sum256(b)
	got := hex.EncodeToString(h[:])
	if got != e.Digest {
		return Entry{}, fmt.Errorf("registry: digest mismatch got %s want %s", got, e.Digest)
	}
	e.Verified = true
	return e, nil
}
