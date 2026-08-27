package profile

// OwnerInspectionStatus enumerates readiness states.
type OwnerInspectionStatus string

const (
	OwnerAbsent           OwnerInspectionStatus = "absent"
	OwnerRevisionRequired OwnerInspectionStatus = "revision_required"
	OwnerUnresolved       OwnerInspectionStatus = "unresolved"
	OwnerPassed           OwnerInspectionStatus = "passed"
)

// OwnerInspection is the claim-safe public attestation dimension.
type OwnerInspection struct {
	Status   OwnerInspectionStatus `json:"status"`
	Verified bool                  `json:"verified"`
}

// ValidateOwnerInspection checks attestation without private journal.
func ValidateOwnerInspection(o OwnerInspection) bool {
	switch o.Status {
	case OwnerAbsent, OwnerRevisionRequired, OwnerUnresolved, OwnerPassed:
		return true
	default:
		return false
	}
}
