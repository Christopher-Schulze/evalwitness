package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// SchemaVersion is the versioned reliability profile.
const SchemaVersion = "evalwitness.profile.v1"

// Status enumerates dimension outcomes; no global scalar.
type Status string

const (
	StatusMeasured      Status = "measured"
	StatusFailed        Status = "failed"
	StatusUnsupported   Status = "unsupported"
	StatusNotApplicable Status = "not_applicable"
	StatusNotMeasured   Status = "not_measured"
)

// Dimension is one narrow reliability question with explicit evidence.
type Dimension struct {
	ID            string  `json:"id"`
	Status        Status  `json:"status"`
	Metric        *string `json:"metric,omitempty"`
	Interval      *string `json:"interval,omitempty"`
	Scope         string  `json:"scope"`
	EvidenceLevel string  `json:"evidence_level"`
	CapsuleExpr   string  `json:"capsule_expression"`
	Caveat        string  `json:"caveat,omitempty"`
	Denominator   int     `json:"denominator"`
	SampleUnit    string  `json:"sample_unit"`
}

// Profile is the versioned multidimensional reliability artifact.
type Profile struct {
	SchemaVersion   string      `json:"schema_version"`
	Identity        string      `json:"identity"`
	ProtocolVersion string      `json:"protocol_version"`
	RouteScope      string      `json:"route_scope"`
	TimeWindow      string      `json:"time_window"`
	Domains         []string    `json:"domains"`
	DataRoles       []string    `json:"data_roles"`
	EvidenceLevels  []string    `json:"evidence_levels"`
	CapsuleParents  []string    `json:"capsule_parents"`
	Dimensions      []Dimension `json:"dimensions"`
	Digest          string      `json:"digest"`
}

// Validate checks strict invariants; unknown dimensions default to failure via policy.
func (p Profile) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("profile: schema_version must be %s", SchemaVersion)
	}
	if p.Identity == "" {
		return fmt.Errorf("profile: identity empty")
	}
	if len(p.Dimensions) == 0 {
		return fmt.Errorf("profile: dimensions empty")
	}
	for i, d := range p.Dimensions {
		if d.ID == "" {
			return fmt.Errorf("profile: dimension %d id empty", i)
		}
		switch d.Status {
		case StatusMeasured, StatusFailed, StatusUnsupported, StatusNotApplicable, StatusNotMeasured:
		default:
			return fmt.Errorf("profile: dimension %q invalid status %q", d.ID, d.Status)
		}
		if d.Status == StatusMeasured && (d.Metric == nil || d.Scope == "" || d.EvidenceLevel == "" || d.CapsuleExpr == "") {
			return fmt.Errorf("profile: dimension %q measured requires metric/scope/evidence_level/capsule_expression", d.ID)
		}
		if d.Denominator < 0 {
			return fmt.Errorf("profile: dimension %q denominator negative", d.ID)
		}
	}
	return nil
}

// Digest computes deterministic hex SHA256 over canonical JSON without Digest field.
func (p Profile) DigestValue() (string, error) {
	tmp := struct {
		SchemaVersion   string      `json:"schema_version"`
		Identity        string      `json:"identity"`
		ProtocolVersion string      `json:"protocol_version"`
		RouteScope      string      `json:"route_scope"`
		TimeWindow      string      `json:"time_window"`
		Domains         []string    `json:"domains"`
		DataRoles       []string    `json:"data_roles"`
		EvidenceLevels  []string    `json:"evidence_levels"`
		CapsuleParents  []string    `json:"capsule_parents"`
		Dimensions      []Dimension `json:"dimensions"`
	}{
		SchemaVersion:   p.SchemaVersion,
		Identity:        p.Identity,
		ProtocolVersion: p.ProtocolVersion,
		RouteScope:      p.RouteScope,
		TimeWindow:      p.TimeWindow,
		Domains:         append([]string(nil), p.Domains...),
		DataRoles:       append([]string(nil), p.DataRoles...),
		EvidenceLevels:  append([]string(nil), p.EvidenceLevels...),
		CapsuleParents:  append([]string(nil), p.CapsuleParents...),
		Dimensions:      append([]Dimension(nil), p.Dimensions...),
	}
	sort.Strings(tmp.Domains)
	sort.Strings(tmp.DataRoles)
	sort.Strings(tmp.EvidenceLevels)
	sort.Strings(tmp.CapsuleParents)
	sort.Slice(tmp.Dimensions, func(i, j int) bool { return tmp.Dimensions[i].ID < tmp.Dimensions[j].ID })
	b, err := json.Marshal(tmp)
	if err != nil {
		return "", fmt.Errorf("profile: marshal: %w", err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
