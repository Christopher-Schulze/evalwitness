package explorer

import (
	"errors"

	"github.com/Christopher-Schulze/evalwitness/internal/profile"
)

// ProfileExplorerView carries the profile's evidence report plus its
// structured markdown for the offline HTML explorer.
type ProfileExplorerView struct {
	Report   profile.EvidenceReport `json:"report"`
	Markdown string                 `json:"markdown"`
}

// AddProfile attaches a verified reliability profile (TASK 058) to an explorer
// report as the TASK 062 interactive-rendering consumer. The profile is
// validated strictly: Validate plus digest equality, mirroring the claimcheck
// gate in cmd/evalwitness. The rendered evidence report and markdown travel
// inside the sealed report so external readers get identical fingerprints.
func AddProfile(report Report, p profile.Profile) (Report, error) {
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	if report.Profile != nil {
		return Report{}, errors.New("explorer report already carries a profile")
	}
	if err := p.Validate(); err != nil {
		return Report{}, err
	}
	digest, err := p.DigestValue()
	if err != nil {
		return Report{}, err
	}
	if digest != p.Digest {
		return Report{}, errors.New("profile digest mismatch")
	}
	rep, err := profile.BuildEvidenceReport(p)
	if err != nil {
		return Report{}, err
	}
	if rep.Markdown == "" {
		return Report{}, errors.New("profile markdown empty")
	}
	report.Profile = &ProfileExplorerView{Report: rep, Markdown: rep.Markdown}
	report.Digest = ""
	return sealReport(report)
}
