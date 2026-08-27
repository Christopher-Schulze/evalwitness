package profile

import (
	"encoding/json"
	"fmt"
)

// EvidenceReport is a human-readable report generated from profile fields.
type EvidenceReport struct {
	Identity        string          `json:"identity"`
	Summary         string          `json:"summary"`
	Dimensions      int             `json:"dimensions"`
	Digest          string          `json:"digest"`
	Text            string          `json:"text"`
	Markdown        string          `json:"markdown"`
	ClaimCheckExprs []string        `json:"claim_check_exprs"`
	OTel            OTelEvent       `json:"otel"`
	DimensionsJSON  json.RawMessage `json:"dimensions_json"`
}

// BuildEvidenceReport generates the report strictly from profile fields; it
// fails on invalid profiles, empty digests, or OTel projection errors.
func BuildEvidenceReport(p Profile) (EvidenceReport, error) {
	if err := p.Validate(); err != nil {
		return EvidenceReport{}, err
	}
	d := p.Digest
	if d == "" {
		var err error
		d, err = p.DigestValue()
		if err != nil {
			return EvidenceReport{}, fmt.Errorf("report: digest: %w", err)
		}
	}
	ev, err := ToOTel(p)
	if err != nil {
		return EvidenceReport{}, fmt.Errorf("report: otel: %w", err)
	}
	dimsJSON, err := json.Marshal(p.Dimensions)
	if err != nil {
		return EvidenceReport{}, fmt.Errorf("report: dimensions: %w", err)
	}
	return EvidenceReport{
		Identity:        p.Identity,
		Summary:         fmt.Sprintf("%d dimensions, no global score", len(p.Dimensions)),
		Dimensions:      len(p.Dimensions),
		Digest:          d,
		Text:            TextReport(p),
		Markdown:        MarkdownReport(p),
		ClaimCheckExprs: ClaimCheckExpr(p),
		OTel:            ev,
		DimensionsJSON:  dimsJSON,
	}, nil
}
