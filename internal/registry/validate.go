package registry

import "github.com/Christopher-Schulze/evalwitness/protocol"

const IntakeValidationSchemaVersion = "evalwitness.registry-intake-validation.v1"

type IntakeValidationReport struct {
	SchemaVersion string `json:"schema_version"`
	EntryID       string `json:"entry_id"`
	Valid         bool   `json:"valid"`
	Error         string `json:"error,omitempty"`
	Digest        string `json:"digest"`
}

func ValidateIntakeReport(entry IntakeEntry) (IntakeValidationReport, error) {
	return ValidateIntakeAgainstCatalog(entry, nil)
}

func PreflightIntake(entry IntakeEntry, catalog []IntakeEntry) (IntakeValidationReport, error) {
	report := IntakeValidationReport{
		SchemaVersion: IntakeValidationSchemaVersion,
		EntryID:       entry.EntryID,
		Valid:         true,
	}
	validator := NewIntakeValidator()
	for _, existing := range catalog {
		if err := validator.Add(existing); err != nil {
			report.Valid = false
			report.Error = err.Error()
			return digestIntakeReport(report)
		}
	}
	if err := validator.Add(entry); err != nil {
		report.Valid = false
		report.Error = err.Error()
	}
	return digestIntakeReport(report)
}

func ValidateIntakeAgainstCatalog(entry IntakeEntry, catalog []IntakeEntry) (IntakeValidationReport, error) {
	report := IntakeValidationReport{
		SchemaVersion: IntakeValidationSchemaVersion,
		EntryID:       entry.EntryID,
		Valid:         true,
	}
	if err := ValidateIntake(entry); err != nil {
		report.Valid = false
		report.Error = err.Error()
	} else if len(catalog) > 0 {
		validator := NewIntakeValidator()
		for _, existing := range catalog {
			if addErr := validator.Add(existing); addErr != nil {
				report.Valid = false
				report.Error = addErr.Error()
				break
			}
		}
		if report.Valid {
			if addErr := validator.Add(entry); addErr != nil {
				report.Valid = false
				report.Error = addErr.Error()
			}
		}
	}
	return digestIntakeReport(report)
}

func digestIntakeReport(report IntakeValidationReport) (IntakeValidationReport, error) {
	digest, err := protocol.Digest(unsignedIntakeValidationReport(report))
	if err != nil {
		return IntakeValidationReport{}, err
	}
	report.Digest = digest
	return report, nil
}

func unsignedIntakeValidationReport(report IntakeValidationReport) IntakeValidationReport {
	report.Digest = ""
	return report
}
