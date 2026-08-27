package drift

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// SchemaVersion is the versioned canary pack.
const SchemaVersion = "evalwitness.canary-pack.v1"

// Canary defines one bounded check.
type Canary struct {
	ID              string `json:"id"`
	Purpose         string `json:"purpose"`
	Task            string `json:"task"`
	RequestContract string `json:"request_contract"`
	MaxCalls        int    `json:"max_calls"`
	MaxTokens       int    `json:"max_tokens"`
	MaxTimeSeconds  int    `json:"max_time_seconds"`
	License         string `json:"license"`
}

// Pack is a versioned bounded canary set.
type Pack struct {
	SchemaVersion string   `json:"schema_version"`
	Version       string   `json:"version"`
	Canaries      []Canary `json:"canaries"`
	Digest        string   `json:"digest"`
}

// BuildPack creates a deterministic pack; canaries sorted by ID, digest over sorted content.
func BuildPack(version string, canaries []Canary) (Pack, error) {
	if version == "" {
		return Pack{}, fmt.Errorf("drift: version empty")
	}
	if len(canaries) == 0 {
		return Pack{}, fmt.Errorf("drift: canaries empty")
	}
	for i, c := range canaries {
		if c.ID == "" || c.Purpose == "" || c.Task == "" || c.RequestContract == "" {
			return Pack{}, fmt.Errorf("drift: canary %d missing required field", i)
		}
		if c.MaxCalls <= 0 || c.MaxTokens <= 0 {
			return Pack{}, fmt.Errorf("drift: canary %q max bounds invalid", c.ID)
		}
	}
	sorted := append([]Canary(nil), canaries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	tmp := struct {
		SchemaVersion string   `json:"schema_version"`
		Version       string   `json:"version"`
		Canaries      []Canary `json:"canaries"`
	}{
		SchemaVersion: SchemaVersion,
		Version:       version,
		Canaries:      sorted,
	}
	b, err := json.Marshal(tmp)
	if err != nil {
		return Pack{}, fmt.Errorf("drift: marshal: %w", err)
	}
	h := sha256.Sum256(b)
	return Pack{
		SchemaVersion: SchemaVersion,
		Version:       version,
		Canaries:      sorted,
		Digest:        hex.EncodeToString(h[:]),
	}, nil
}
