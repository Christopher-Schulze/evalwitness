package profile

import (
	"fmt"
	"os"

	"github.com/Christopher-Schulze/evalwitness/internal/relation"
)

// NegativeEvidenceProjection is the TASK 068 scarcity panel derived from its
// canonical typed parent eval/results/relation-scarcity-negative-evidence.json.
type NegativeEvidenceProjection struct {
	Target    int    `json:"target"`
	Attempts  int    `json:"attempts"`
	Applied   int    `json:"applied"`
	Rejected  int    `json:"rejected"`
	Shortfall int    `json:"shortfall"`
	Status    Status `json:"status"`
	Source    string `json:"source"`
}

const ScarcityEvidencePath = "eval/results/relation-scarcity-negative-evidence.json"

// LoadNegativeEvidence decodes the canonical typed scarcity evidence, reusing the
// existing relation verifier (canonical_policy evalwitness.relation-canonical-json.v1,
// strict JSON with DisallowUnknownFields, parent/digest/claim validation).
// Markdown is never parsed; every numeric/status field resolves from JSON or its
// capsule parents.
func LoadNegativeEvidence(path string) (NegativeEvidenceProjection, error) {
	f, err := os.Open(path)
	if err != nil {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity evidence open: %w", err)
	}
	defer func() { _ = f.Close() }()
	ev, err := relation.DecodeScarcityPublicEvidence(f)
	if err != nil {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity evidence decode: %w", err)
	}
	if ev.SchemaVersion != relation.ScarcityPublicEvidenceSchemaVersion {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity schema: %s", ev.SchemaVersion)
	}
	if ev.CanonicalPolicy != relation.CanonicalPolicy {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity canonical policy: %s", ev.CanonicalPolicy)
	}
	if ev.EvidenceKind != "deterministic_public_negative_evidence" {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity evidence_kind: %s", ev.EvidenceKind)
	}
	if ev.ConstructFamily != "omitted_test_evidence" {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity construct_family: %s", ev.ConstructFamily)
	}
	if ev.Availability.Attempted != ev.Availability.Admitted+ev.Availability.Rejected {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity invariant attempted != admitted+rejected: %d != %d+%d", ev.Availability.Attempted, ev.Availability.Admitted, ev.Availability.Rejected)
	}
	if ev.Availability.Shortfall != ev.Availability.Target-ev.Availability.Admitted {
		return NegativeEvidenceProjection{}, fmt.Errorf("scarcity invariant shortfall != target-admitted: %d != %d-%d", ev.Availability.Shortfall, ev.Availability.Target, ev.Availability.Admitted)
	}
	return NegativeEvidenceProjection{
		Target:    ev.Availability.Target,
		Attempts:  ev.Availability.Attempted,
		Applied:   ev.Availability.Admitted,
		Rejected:  ev.Availability.Rejected,
		Shortfall: ev.Availability.Shortfall,
		Status:    StatusMeasured,
		Source:    path,
	}, nil
}

// NegativeEvidence returns the projection from the canonical file at ScarcityEvidencePath.
// It probes both repo-root and package-dir relative locations so tests and CLI work
// regardless of cwd, but never falls back to hardcoded constants.
func NegativeEvidence() (NegativeEvidenceProjection, error) {
	if p, err := LoadNegativeEvidence(ScarcityEvidencePath); err == nil {
		return p, nil
	}
	return LoadNegativeEvidence("../../" + ScarcityEvidencePath)
}

var _ = LoadNegativeEvidence // ensure typed decoder path is used; Markdown parsing forbidden
