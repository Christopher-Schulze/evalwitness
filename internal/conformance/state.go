package conformance

import "fmt"

type RouteState string

const (
	StateUnconfigured     RouteState = "unconfigured"
	StateConfigured       RouteState = "configured"
	StateProbeCompatible  RouteState = "probe_compatible"
	StateBoundedQualified RouteState = "bounded_qualified"
	StateStudyQualified   RouteState = "study_qualified"
	StateExpired          RouteState = "expired"
	StateFailed           RouteState = "failed"
)

func (s RouteState) Valid() bool {
	switch s {
	case StateUnconfigured, StateConfigured, StateProbeCompatible, StateBoundedQualified,
		StateStudyQualified, StateExpired, StateFailed:
		return true
	default:
		return false
	}
}

func (s RouteState) EvidenceRank() int {
	switch s {
	case StateConfigured:
		return 1
	case StateProbeCompatible:
		return 2
	case StateBoundedQualified:
		return 3
	case StateStudyQualified:
		return 4
	default:
		return 0
	}
}

func ValidateTransition(from, to RouteState) error {
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("invalid route state transition %q -> %q", from, to)
	}
	allowed := false
	switch from {
	case StateUnconfigured:
		allowed = to == StateConfigured
	case StateConfigured:
		allowed = to == StateProbeCompatible || to == StateBoundedQualified || to == StateFailed
	case StateProbeCompatible:
		allowed = to == StateBoundedQualified || to == StateFailed || to == StateExpired
	case StateBoundedQualified:
		allowed = to == StateStudyQualified || to == StateExpired || to == StateFailed
	case StateStudyQualified:
		allowed = to == StateExpired || to == StateFailed
	case StateExpired, StateFailed:
		allowed = to == StateConfigured
	}
	if !allowed {
		return fmt.Errorf("route state transition %q -> %q is not permitted", from, to)
	}
	return nil
}
