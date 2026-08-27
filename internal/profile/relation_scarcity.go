package profile

import (
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
)

// RelationScarcity is one 058 relation-scarcity projection entry derived from
// eval/results/relation-scarcity-negative-evidence.json.
type RelationScarcity struct {
	Family       string `json:"family"`
	Status       Status `json:"status"`
	Source       string `json:"source"`
	Target       int    `json:"target"`
	Attempted    int    `json:"attempted"`
	Admitted     int    `json:"admitted"`
	Rejected     int    `json:"rejected"`
	TestZero     bool   `json:"test_zero"`
	CoreCases    int    `json:"core_cases"`
	CoreFamilies int    `json:"core_families"`
}

// LoadRelationScarcity decodes the canonical typed evidence via the shared
// relation verifier and derives scarcity dimensions from parsed data.
func LoadRelationScarcity(path string) ([]RelationScarcity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("relation scarcity open: %w", err)
	}
	defer func() { _ = f.Close() }()
	ev, err := relation.DecodeScarcityPublicEvidence(f)
	if err != nil {
		return nil, fmt.Errorf("relation scarcity decode: %w", err)
	}
	if ev.SchemaVersion != relation.ScarcityPublicEvidenceSchemaVersion || ev.CanonicalPolicy != relation.CanonicalPolicy {
		return nil, fmt.Errorf("relation scarcity identity mismatch")
	}
	if ev.Availability.Attempted != ev.Availability.Admitted+ev.Availability.Rejected {
		return nil, fmt.Errorf("scarcity invariant attempted != admitted+rejected")
	}
	if ev.Availability.Shortfall != ev.Availability.Target-ev.Availability.Admitted {
		return nil, fmt.Errorf("scarcity invariant shortfall")
	}
	one := RelationScarcity{
		Family:       string(ev.ConstructFamily),
		Status:       StatusMeasured,
		Source:       path,
		Target:       ev.Availability.Target,
		Attempted:    ev.Availability.Attempted,
		Admitted:     ev.Availability.Admitted,
		Rejected:     ev.Availability.Rejected,
		TestZero:     ev.StudyRoles.Test == 0,
		CoreCases:    ev.InferentialCore.Cases,
		CoreFamilies: ev.InferentialCore.Families,
	}
	return []RelationScarcity{one}, nil
}

// RelationScarcityDimensions returns the scarcity dimensions from the canonical
// file at ScarcityEvidencePath. Hardcoded family strings are forbidden.
func RelationScarcityDimensions() ([]RelationScarcity, error) {
	if d, err := LoadRelationScarcity(ScarcityEvidencePath); err == nil {
		return d, nil
	}
	return LoadRelationScarcity("../../" + ScarcityEvidencePath)
}
