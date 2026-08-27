package lineage

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/internal/preprocess"
)

const AdapterConformanceSchemaVersion = "evalwitness.verification-lineage-adapter-conformance.v1"

type AdapterConformanceStatus string

const (
	AdapterConformant        AdapterConformanceStatus = "conformant"
	AdapterKnownGap          AdapterConformanceStatus = "known_mapping_gap"
	AdapterExpectedRejection AdapterConformanceStatus = "expected_rejection"
	AdapterNotApplicable     AdapterConformanceStatus = "not_applicable"
)

type AdapterConformanceCheck struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
	Passed   bool   `json:"passed"`
}

type AdapterConformanceCase struct {
	VectorID                string                    `json:"vector_id"`
	Format                  preprocess.SourceFormat   `json:"format"`
	Status                  AdapterConformanceStatus  `json:"status"`
	SourceDigest            string                    `json:"source_digest"`
	TrajectoryDigest        string                    `json:"trajectory_digest"`
	EnvelopeDigest          string                    `json:"envelope_digest"`
	MappingDigest           string                    `json:"mapping_digest"`
	CanonicalEncodingDigest string                    `json:"canonical_encoding_digest"`
	Checks                  []AdapterConformanceCheck `json:"checks"`
	KnownMappingGap         string                    `json:"known_mapping_gap"`
}

type AdapterConformanceSummary struct {
	Vectors            int `json:"vectors"`
	Conformant         int `json:"conformant"`
	KnownMappingGaps   int `json:"known_mapping_gaps"`
	ExpectedRejections int `json:"expected_rejections"`
	NotApplicable      int `json:"not_applicable"`
	Checks             int `json:"checks"`
	PassedChecks       int `json:"passed_checks"`
	FailedChecks       int `json:"failed_checks"`
}

type AdapterConformanceClaimBoundary struct {
	EvidenceRole      DataRole `json:"evidence_role"`
	ProviderCalls     int      `json:"provider_calls"`
	AgentLaunch       bool     `json:"agent_launch"`
	ResearchOutcome   bool     `json:"research_outcome"`
	FormatWideClaim   bool     `json:"format_wide_claim"`
	SupportedClaim    string   `json:"supported_claim"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

type AdapterConformanceReport struct {
	SchemaVersion   string                          `json:"schema_version"`
	CanonicalPolicy string                          `json:"canonical_policy"`
	PlanDigest      string                          `json:"plan_digest"`
	GoldenSetDigest string                          `json:"golden_set_digest"`
	Cases           []AdapterConformanceCase        `json:"cases"`
	Summary         AdapterConformanceSummary       `json:"summary"`
	ClaimBoundary   AdapterConformanceClaimBoundary `json:"claim_boundary"`
	Digest          string                          `json:"digest"`
}

func BuildAdapterConformanceReport() (AdapterConformanceReport, error) {
	report, err := buildAdapterConformanceReportUnchecked()
	if err != nil {
		return AdapterConformanceReport{}, err
	}
	return report, validateAdapterConformanceReportIntegrity(report)
}

func (report AdapterConformanceReport) Validate() error {
	if err := validateAdapterConformanceReportIntegrity(report); err != nil {
		return err
	}
	expected, err := buildAdapterConformanceReportUnchecked()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(report, expected) {
		return errors.New("adapter-conformance report differs from the sealed golden-vector execution")
	}
	return nil
}

func validateAdapterConformanceReportIntegrity(report AdapterConformanceReport) error {
	if report.SchemaVersion != AdapterConformanceSchemaVersion || report.CanonicalPolicy != CanonicalPolicy || report.PlanDigest != LockedPlanDigest || report.GoldenSetDigest == "" {
		return errors.New("adapter-conformance identity is invalid")
	}
	if !reflect.DeepEqual(report.ClaimBoundary, expectedAdapterConformanceClaimBoundary()) {
		return errors.New("adapter-conformance claim boundary is invalid")
	}
	if report.Summary != summarizeAdapterConformance(report.Cases) {
		return errors.New("adapter-conformance summary is invalid")
	}
	digest, err := adapterConformanceDigest(report)
	if err != nil {
		return err
	}
	if report.Digest == "" || report.Digest != digest {
		return errors.New("adapter-conformance digest is invalid")
	}
	return nil
}

func buildAdapterConformanceReportUnchecked() (AdapterConformanceReport, error) {
	golden, err := BuildGoldenVectorFixtureSet()
	if err != nil {
		return AdapterConformanceReport{}, err
	}
	definitions := goldenCaseDefinitions()
	definitionByID := make(map[string]goldenCaseDefinition, len(definitions))
	for _, definition := range definitions {
		definitionByID[definition.id] = definition
	}
	report := AdapterConformanceReport{
		SchemaVersion: AdapterConformanceSchemaVersion, CanonicalPolicy: CanonicalPolicy,
		PlanDigest: LockedPlanDigest, GoldenSetDigest: golden.Digest,
		ClaimBoundary: expectedAdapterConformanceClaimBoundary(),
	}
	for _, vector := range golden.Vectors {
		item, buildErr := buildAdapterConformanceCase(vector, definitionByID[vector.CaseID])
		if buildErr != nil {
			return AdapterConformanceReport{}, buildErr
		}
		report.Cases = append(report.Cases, item)
	}
	report.Summary = summarizeAdapterConformance(report.Cases)
	digest, err := adapterConformanceDigest(report)
	if err != nil {
		return AdapterConformanceReport{}, err
	}
	report.Digest = digest
	return report, nil
}

func buildAdapterConformanceCase(vector VerificationLineageGoldenVector, definition goldenCaseDefinition) (AdapterConformanceCase, error) {
	item := AdapterConformanceCase{VectorID: vector.VectorID, Format: vector.Format, KnownMappingGap: vector.KnownMappingGap}
	if vector.Observation.ImportOutcome == GoldenImportNotApplicable {
		item.Status = AdapterNotApplicable
		item.Checks = []AdapterConformanceCheck{{
			Name: "representability_boundary", Expected: string(vector.Expectation.RepresentableByFormat),
			Observed: string(vector.Observation.ImportOutcome), Passed: true,
		}}
		return item, nil
	}
	raw, _, err := goldenNativeDocument(vector.Format, definition)
	if err != nil {
		return AdapterConformanceCase{}, err
	}
	options := preprocess.DefaultTraceImportOptions()
	first, firstErr := preprocess.ImportTraceBytes([]byte(raw), options)
	if vector.Observation.ImportOutcome == GoldenImportRejected {
		item.Status = AdapterExpectedRejection
		item.SourceDigest = digestBytes([]byte(raw))
		item.Checks = []AdapterConformanceCheck{{
			Name: "strict_unsupported_rejection", Expected: "rejected", Observed: importOutcome(firstErr), Passed: firstErr != nil,
		}}
		if firstErr == nil {
			return AdapterConformanceCase{}, errors.New("strict adapter accepted unsupported native syntax")
		}
		return item, nil
	}
	if firstErr != nil {
		return AdapterConformanceCase{}, firstErr
	}
	second, err := preprocess.ImportTraceBytes([]byte(raw), options)
	if err != nil {
		return AdapterConformanceCase{}, err
	}
	item.SourceDigest = first.Trajectory.SourceDigest
	item.TrajectoryDigest = first.Trajectory.Digest
	item.EnvelopeDigest = first.Envelope.Digest
	item.MappingDigest = first.Mapping.Digest
	canonical, err := json.Marshal(first.Trajectory)
	if err != nil {
		return AdapterConformanceCase{}, fmt.Errorf("encode canonical trajectory: %w", err)
	}
	item.CanonicalEncodingDigest = digestBytes(canonical)
	var roundTrip preprocess.Trajectory
	roundTripErr := json.Unmarshal(canonical, &roundTrip)
	if roundTripErr == nil {
		roundTripErr = roundTrip.Validate()
	}
	checks := []AdapterConformanceCheck{
		checkAdapter("raw_record_accounting", fmt.Sprintf("%d records", first.Trajectory.Report.SourceRecords), fmt.Sprintf("records=%d accounting=%d mapping=%d", len(first.Trajectory.Report.Records), first.Trajectory.Report.AccountedRecords, first.Mapping.SourceRecords), len(first.Trajectory.Report.Records) == first.Trajectory.Report.SourceRecords && first.Trajectory.Report.AccountedRecords == first.Trajectory.Report.SourceRecords && first.Mapping.SourceRecords == first.Trajectory.Report.SourceRecords),
		checkAdapter("field_accounting", "mapping fields equal entries and disposition totals", adapterMappingCounts(first.Mapping), validAdapterMappingCounts(first.Mapping)),
		checkAdapter("stable_import_identity", "byte-identical import result", fmt.Sprintf("trajectory=%s envelope=%s mapping=%s", second.Trajectory.Digest, second.Envelope.Digest, second.Mapping.Digest), reflect.DeepEqual(first, second)),
		checkAdapter("canonical_json_roundtrip", first.Trajectory.Digest, roundTrip.Digest, roundTripErr == nil && roundTrip.Digest == first.Trajectory.Digest),
		checkAdapter("native_call_result_linkage", fmt.Sprintf("calls>=%d results>=%d links>=%d", vector.Expectation.MinimumToolCalls, vector.Expectation.MinimumToolResults, vector.Expectation.MinimumCallResultLinks), fmt.Sprintf("calls=%d results=%d links=%d", vector.Observation.ToolCalls, vector.Observation.ToolResults, vector.Observation.CallResultLinks), vector.Observation.ToolCalls >= vector.Expectation.MinimumToolCalls && vector.Observation.ToolResults >= vector.Expectation.MinimumToolResults && vector.Observation.CallResultLinks >= vector.Expectation.MinimumCallResultLinks),
		checkAdapter("command_display_preservation", vector.Expectation.CommandDisplay, fmt.Sprint(vector.Observation.CommandDisplays), vector.Expectation.CommandDisplay == "" || slices.Contains(vector.Observation.CommandDisplays, vector.Expectation.CommandDisplay)),
		checkAdapter("redaction_accounting", fmt.Sprintf("hits=%d", vector.Expectation.RedactionHits), fmt.Sprintf("hits=%d", vector.Observation.RedactionHits), vector.Observation.RedactionHits == vector.Expectation.RedactionHits),
		checkAdapter("retention_boundary", fmt.Sprintf("truncation=%t", vector.Expectation.TruncationRequired), fmt.Sprintf("truncation=%t canonical=%s retained=%s", vector.Observation.TruncationApplied, vector.Observation.TrajectoryDigest, vector.Observation.RetainedTrajectoryDigest), vector.Observation.TruncationApplied == vector.Expectation.TruncationRequired && (!vector.Expectation.TruncationRequired || vector.Observation.TrajectoryDigest != vector.Observation.RetainedTrajectoryDigest)),
		checkAdapter("exit_status_preservation", fmt.Sprintf("required=%t", vector.Expectation.ExitStatusRequired), fmt.Sprintf("observations=%d", vector.Observation.ExitStatusObservations), !vector.Expectation.ExitStatusRequired || vector.Observation.ExitStatusObservations > 0),
		adapterIdentityIntegrityCheck(definition, vector),
	}
	item.Checks = checks
	if vector.KnownMappingGap != "" {
		item.Status = AdapterKnownGap
		if failedAdapterChecks(checks) == 0 {
			return AdapterConformanceCase{}, errors.New("known mapping gap has no failing normative check")
		}
	} else {
		item.Status = AdapterConformant
		if failedAdapterChecks(checks) != 0 {
			return AdapterConformanceCase{}, errors.New("unexpected normative conformance failure")
		}
	}
	return item, nil
}

func adapterIdentityIntegrityCheck(definition goldenCaseDefinition, vector VerificationLineageGoldenVector) AdapterConformanceCheck {
	expected := "unique native call identity with causal order"
	observed := "unique_and_ordered"
	passed := true
	switch {
	case definition.missingCallID:
		observed = "missing_call_id"
		passed = false
	case definition.duplicateCallID:
		observed = "duplicate_call_id"
		passed = false
	case definition.outOfOrderResult && vector.Format != preprocess.SourceOpenCode:
		observed = "result_precedes_call"
		passed = false
	}
	return checkAdapter("native_identity_integrity", expected, observed, passed)
}

func checkAdapter(name, expected, observed string, passed bool) AdapterConformanceCheck {
	return AdapterConformanceCheck{Name: name, Expected: expected, Observed: observed, Passed: passed}
}

func importOutcome(err error) string {
	if err != nil {
		return "rejected"
	}
	return "accepted"
}

func adapterMappingCounts(report preprocess.MappingReport) string {
	return fmt.Sprintf("fields=%d entries=%d totals=%d", report.SourceFields, adapterMappingEntryCount(report), adapterMappingTotal(report))
}

func validAdapterMappingCounts(report preprocess.MappingReport) bool {
	return report.SourceFields > 0 && report.SourceFields == adapterMappingEntryCount(report) && report.SourceFields == adapterMappingTotal(report)
}

func adapterMappingEntryCount(report preprocess.MappingReport) int {
	total := 0
	for _, entry := range report.Entries {
		total += entry.Count
	}
	return total
}

func adapterMappingTotal(report preprocess.MappingReport) int {
	return report.Totals.Exact + report.Totals.Normalized + report.Totals.Synthesized + report.Totals.Redacted + report.Totals.Unsupported + report.Totals.Ambiguous + report.Totals.Dropped
}

func failedAdapterChecks(checks []AdapterConformanceCheck) int {
	failed := 0
	for _, check := range checks {
		if !check.Passed {
			failed++
		}
	}
	return failed
}

func summarizeAdapterConformance(cases []AdapterConformanceCase) AdapterConformanceSummary {
	summary := AdapterConformanceSummary{Vectors: len(cases)}
	for _, item := range cases {
		switch item.Status {
		case AdapterConformant:
			summary.Conformant++
		case AdapterKnownGap:
			summary.KnownMappingGaps++
		case AdapterExpectedRejection:
			summary.ExpectedRejections++
		case AdapterNotApplicable:
			summary.NotApplicable++
		}
		for _, check := range item.Checks {
			summary.Checks++
			if check.Passed {
				summary.PassedChecks++
			} else {
				summary.FailedChecks++
			}
		}
	}
	return summary
}

func expectedAdapterConformanceClaimBoundary() AdapterConformanceClaimBoundary {
	return AdapterConformanceClaimBoundary{
		EvidenceRole: RoleAdapterDevelopment, ProviderCalls: 0, AgentLaunch: false,
		ResearchOutcome: false, FormatWideClaim: false,
		SupportedClaim: "the sealed synthetic native vectors reproduce exact adapter mappings and fail-closed identity boundaries",
		UnsupportedClaims: []string{
			"agent_behavior_frequency", "empirical_corpus_outcome", "format_is_lossless",
			"human_validated", "provider_quality", "universal_format_capability",
		},
	}
}

func adapterConformanceDigest(report AdapterConformanceReport) (string, error) {
	report.Digest = ""
	return digestJSON(report)
}
