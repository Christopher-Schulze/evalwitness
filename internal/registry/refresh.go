package registry

import (
	"sort"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const CatalogRefreshSchemaVersion = "evalwitness.registry-catalog-refresh.v1"

type CatalogRefreshReport struct {
	SchemaVersion string                   `json:"schema_version"`
	Total         int                      `json:"total"`
	Current       int                      `json:"current"`
	Rejected      int                      `json:"rejected"`
	Entries       []IntakeValidationReport `json:"entries"`
	Limitations   []string                 `json:"limitations"`
	Digest        string                   `json:"digest"`
}

func RefreshCatalog(entries []IntakeEntry) (CatalogRefreshReport, error) {
	reports := make([]IntakeValidationReport, 0, len(entries))
	isolated := []IntakeEntry{}
	for _, entry := range entries {
		report, err := PreflightIntake(entry, nil)
		if err != nil {
			return CatalogRefreshReport{}, err
		}
		reports = append(reports, report)
		if report.Valid {
			isolated = append(isolated, entry)
		}
	}
	validator := NewIntakeValidator()
	rejected := map[string]string{}
	for _, entry := range isolated {
		if err := validator.Add(entry); err != nil {
			rejected[entry.EntryID] = err.Error()
		}
	}
	for i, report := range reports {
		if errText, ok := rejected[report.EntryID]; ok {
			report.Valid = false
			report.Error = errText
			digested, err := digestIntakeReport(report)
			if err != nil {
				return CatalogRefreshReport{}, err
			}
			reports[i] = digested
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].EntryID < reports[j].EntryID })
	refresh := CatalogRefreshReport{
		SchemaVersion: CatalogRefreshSchemaVersion,
		Total:         len(reports),
		Entries:       reports,
		Limitations: []string{
			"offline clock check only; no live provider call or scheduled refresh",
			"not a community registry admission",
		},
	}
	for _, report := range reports {
		if report.Valid {
			refresh.Current++
		} else {
			refresh.Rejected++
		}
	}
	digest, err := protocol.Digest(unsignedCatalogRefreshReport(refresh))
	if err != nil {
		return CatalogRefreshReport{}, err
	}
	refresh.Digest = digest
	return refresh, nil
}

func unsignedCatalogRefreshReport(report CatalogRefreshReport) CatalogRefreshReport {
	report.Digest = ""
	return report
}
