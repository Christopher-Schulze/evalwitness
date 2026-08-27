package profile

import "fmt"

// Fixtures returns deterministic reference, development, degraded profiles; fails if Build errors.
func Fixtures() ([]Profile, error) {
	metric := "1"
	p1, err := Build("reference", "v1", "r1", []Dimension{{ID: "a", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	if err != nil {
		return nil, fmt.Errorf("fixture reference: %w", err)
	}
	p2, err := Build("development", "v1", "r1", []Dimension{{ID: "a", Status: StatusMeasured, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 10, SampleUnit: "task"}})
	if err != nil {
		return nil, fmt.Errorf("fixture development: %w", err)
	}
	p3, err := Build("degraded", "v1", "r1", []Dimension{{ID: "a", Status: StatusFailed, Metric: &metric, Scope: "s", EvidenceLevel: "E1", CapsuleExpr: "c", Denominator: 1, SampleUnit: "task"}})
	if err != nil {
		return nil, fmt.Errorf("fixture degraded: %w", err)
	}
	return []Profile{p1, p2, p3}, nil
}
