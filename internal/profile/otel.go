package profile

import "fmt"

// OTelEvent is a standards-aligned evaluation event.
type OTelEvent struct {
	Name          string `json:"name"`
	TraceID       string `json:"trace_id"`
	ProfileDigest string `json:"profile_digest"`
	RouteScope    string `json:"route_scope"`
}

// ToOTel maps profile to OTel event without private evidence; fails if digest empty.
func ToOTel(p Profile) (OTelEvent, error) {
	if p.Digest == "" {
		return OTelEvent{}, fmt.Errorf("otel: digest empty")
	}
	if p.Identity == "" {
		return OTelEvent{}, fmt.Errorf("otel: identity empty")
	}
	return OTelEvent{Name: "evalwitness.profile", TraceID: p.Identity, ProfileDigest: p.Digest, RouteScope: p.RouteScope}, nil
}
