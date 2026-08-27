package profile

import (
	"fmt"
	"sort"
)

type DiffResult struct {
	Compatible bool     `json:"compatible"`
	Reasons    []string `json:"reasons,omitempty"`
	Added      []string `json:"added,omitempty"`
	Removed    []string `json:"removed,omitempty"`
	Changed    []string `json:"changed,omitempty"`
}

// Diff compares compatible protocol and route scopes; incompatible -> refusal, not rank.
func Diff(a, b Profile) DiffResult {
	var r DiffResult
	if a.ProtocolVersion != b.ProtocolVersion {
		r.Reasons = append(r.Reasons, fmt.Sprintf("protocol_version %q vs %q", a.ProtocolVersion, b.ProtocolVersion))
	}
	if a.RouteScope != b.RouteScope {
		r.Reasons = append(r.Reasons, fmt.Sprintf("route_scope %q vs %q", a.RouteScope, b.RouteScope))
	}
	if len(r.Reasons) > 0 {
		r.Compatible = false
		return r
	}
	r.Compatible = true
	am := make(map[string]Dimension, len(a.Dimensions))
	for _, d := range a.Dimensions {
		am[d.ID] = d
	}
	bm := make(map[string]Dimension, len(b.Dimensions))
	for _, d := range b.Dimensions {
		bm[d.ID] = d
	}
	for id := range am {
		if _, ok := bm[id]; !ok {
			r.Removed = append(r.Removed, id)
		}
	}
	for id := range bm {
		if _, ok := am[id]; !ok {
			r.Added = append(r.Added, id)
		}
	}
	for id, da := range am {
		if db, ok := bm[id]; ok {
			if da.Status != db.Status || stringPtr(da.Metric) != stringPtr(db.Metric) || da.Scope != db.Scope || da.Denominator != db.Denominator || da.CapsuleExpr != db.CapsuleExpr || da.EvidenceLevel != db.EvidenceLevel {
				r.Changed = append(r.Changed, id)
			}
		}
	}
	sort.Strings(r.Added)
	sort.Strings(r.Removed)
	sort.Strings(r.Changed)
	return r
}

func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
