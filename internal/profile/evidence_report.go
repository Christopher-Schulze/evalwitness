package profile

import "fmt"

// EvidenceReportDetail is a detailed report for one profile.
type EvidenceReportDetail struct {
	ProfileID string `json:"profile_id"`
	Report    string `json:"report"`
}

// GenerateEvidenceReport creates a detailed report from profile.
func GenerateEvidenceReport(p Profile) (EvidenceReportDetail, error) {
	if err := p.Validate(); err != nil {
		return EvidenceReportDetail{}, err
	}
	return EvidenceReportDetail{
		ProfileID: p.Identity,
		Report:    fmt.Sprintf("Profile %s with %d dimensions digest %s", p.Identity, len(p.Dimensions), p.Digest),
	}, nil
}
