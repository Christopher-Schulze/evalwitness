package replay

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	CaptureResearchAdmissionSchemaVersion   = "evalwitness.capture-research-admission.v1"
	CaptureResearchStampReportSchemaVersion = "evalwitness.capture-research-lineage-stamp.v1"
	CaptureResearchAdmissionAdmitted        = "admitted"
	CaptureResearchAdmissionRejected        = "rejected"
	CaptureResearchRequiredRecapture        = "recapture_with_preplanned_research_lineage"
)

// CaptureResearchLineageStamp is overlay identity for incomplete schema-3 records.
// Applying it changes request fingerprint whenever evidence bindings are written.
type CaptureResearchLineageStamp struct {
	Lineage                 provider.RequestLineage    `json:"lineage"`
	EvidenceBindings        []provider.EvidenceBinding `json:"evidence_bindings"`
	CapabilityAttestationID string                     `json:"capability_attestation_id"`
	ServedModel             string                     `json:"served_model,omitempty"`
	CheckpointAssertion     string                     `json:"checkpoint_assertion,omitempty"`
}

type CaptureResearchLineageStampReport struct {
	SchemaVersion            string `json:"schema_version"`
	SourcePayloadSHA256      string `json:"source_payload_sha256"`
	DestinationPayloadSHA256 string `json:"destination_payload_sha256"`
	SourceEntries            int    `json:"source_entries"`
	StampedEntries           int    `json:"stamped_entries"`
	AlreadyCompleteEntries   int    `json:"already_complete_entries"`
	CompleteResearchEntries  int    `json:"complete_research_entries"`
	Admission                string `json:"admission"`
	Digest                   string `json:"digest"`
}

type CaptureResearchAdmission struct {
	SchemaVersion               string   `json:"schema_version"`
	PayloadSHA256               string   `json:"payload_sha256"`
	AuthorizedCalls             int      `json:"authorized_calls"`
	ObservedCalls               int      `json:"observed_calls"`
	CompleteResearchEntries     int      `json:"complete_research_entries"`
	CapabilityAttestationIDs    []string `json:"capability_attestation_ids"`
	CaptureRunStatus            string   `json:"capture_run_status"`
	InspectionMode              string   `json:"inspection_mode"`
	Admission                   string   `json:"admission"`
	Reasons                     []string `json:"reasons"`
	RequiredAction              string   `json:"required_action,omitempty"`
	CaptureRunAttestationDigest string   `json:"capture_run_attestation_digest"`
	Digest                      string   `json:"digest"`
}

func AdmitCaptureResearchLineage(path string, authorizedCalls int) (CaptureResearchAdmission, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return CaptureResearchAdmission{}, err
	}
	return AdmitCaptureResearchLineagePayload(payload, authorizedCalls)
}

// AdmitCaptureResearchLineagePayload derives admission from exact capture
// bytes supplied by a capsule-family verifier.
func AdmitCaptureResearchLineagePayload(payload []byte, authorizedCalls int) (CaptureResearchAdmission, error) {
	attestation, err := SealCaptureRunAttestationPayload(payload, authorizedCalls)
	if err != nil {
		return CaptureResearchAdmission{}, err
	}
	admission := CaptureResearchAdmission{
		SchemaVersion:               CaptureResearchAdmissionSchemaVersion,
		PayloadSHA256:               attestation.Inspection.PayloadSHA256,
		AuthorizedCalls:             authorizedCalls,
		ObservedCalls:               attestation.ObservedCalls,
		CompleteResearchEntries:     attestation.Inspection.CompleteResearchEntries,
		CapabilityAttestationIDs:    append([]string(nil), attestation.Inspection.CapabilityAttestationIDs...),
		CaptureRunStatus:            attestation.Status,
		InspectionMode:              attestation.InspectionMode,
		CaptureRunAttestationDigest: attestation.Digest,
		Reasons:                     append([]string(nil), attestation.Limitations...),
	}
	if attestation.Status == CaptureRunStatusComplete && attestation.ResearchLineageComplete {
		admission.Admission = CaptureResearchAdmissionAdmitted
	} else {
		admission.Admission = CaptureResearchAdmissionRejected
		admission.RequiredAction = CaptureResearchRequiredRecapture
		if len(admission.Reasons) == 0 {
			admission.Reasons = []string{"research lineage is incomplete on the sealed capture"}
		}
	}
	if admission.CapabilityAttestationIDs == nil {
		admission.CapabilityAttestationIDs = []string{}
	}
	if admission.Reasons == nil {
		admission.Reasons = []string{}
	}
	digest, err := protocol.Digest(unsignedCaptureResearchAdmission(admission))
	if err != nil {
		return CaptureResearchAdmission{}, err
	}
	admission.Digest = digest
	return admission, nil
}

func StampCaptureResearchLineage(source, destination string, stamp CaptureResearchLineageStamp) (CaptureResearchLineageStampReport, error) {
	if err := validateResearchLineageStamp(stamp); err != nil {
		return CaptureResearchLineageStampReport{}, err
	}
	sourceAbs, destAbs, err := exclusiveCapturePaths(source, destination)
	if err != nil {
		return CaptureResearchLineageStampReport{}, err
	}
	payload, err := readCaptureSource(sourceAbs)
	if err != nil {
		return CaptureResearchLineageStampReport{}, err
	}
	sourceInspection, _, err := inspectCapturePayload(payload)
	if err != nil {
		return CaptureResearchLineageStampReport{}, err
	}
	var encoded bytes.Buffer
	stamped := 0
	alreadyComplete := 0
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			return CaptureResearchLineageStampReport{}, errors.New("research lineage stamp refused a blank capture record")
		}
		var entry fixtureEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return CaptureResearchLineageStampReport{}, fmt.Errorf("parse capture record: %w", err)
		}
		if responseEntryHasCompleteResearchEvidence(entry) {
			alreadyComplete++
			if _, err := encoded.Write(append(append([]byte{}, line...), '\n')); err != nil {
				return CaptureResearchLineageStampReport{}, err
			}
			continue
		}
		updated, err := applyResearchLineageStamp(entry, stamp)
		if err != nil {
			return CaptureResearchLineageStampReport{}, err
		}
		record, err := json.Marshal(updated)
		if err != nil {
			return CaptureResearchLineageStampReport{}, err
		}
		if _, err := encoded.Write(append(record, '\n')); err != nil {
			return CaptureResearchLineageStampReport{}, err
		}
		stamped++
	}
	if err := scanner.Err(); err != nil {
		return CaptureResearchLineageStampReport{}, err
	}
	if stamped+alreadyComplete == 0 {
		return CaptureResearchLineageStampReport{}, errors.New("research lineage stamp found no capture records")
	}
	destFile, err := os.OpenFile(destAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return CaptureResearchLineageStampReport{}, err
	}
	if _, err := destFile.Write(encoded.Bytes()); err != nil {
		_ = destFile.Close()
		_ = os.Remove(destAbs)
		return CaptureResearchLineageStampReport{}, err
	}
	if err := destFile.Close(); err != nil {
		_ = os.Remove(destAbs)
		return CaptureResearchLineageStampReport{}, err
	}
	destInspection, err := InspectCaptureFile(destAbs)
	if err != nil {
		_ = os.Remove(destAbs)
		return CaptureResearchLineageStampReport{}, err
	}
	report := CaptureResearchLineageStampReport{
		SchemaVersion:            CaptureResearchStampReportSchemaVersion,
		SourcePayloadSHA256:      sourceInspection.PayloadSHA256,
		DestinationPayloadSHA256: destInspection.PayloadSHA256,
		SourceEntries:            sourceInspection.Entries,
		StampedEntries:           stamped,
		AlreadyCompleteEntries:   alreadyComplete,
		CompleteResearchEntries:  destInspection.CompleteResearchEntries,
	}
	if destInspection.CompleteResearchEntries == destInspection.Entries && destInspection.Entries > 0 {
		report.Admission = CaptureResearchAdmissionAdmitted
	} else {
		report.Admission = CaptureResearchAdmissionRejected
	}
	digest, err := protocol.Digest(unsignedCaptureResearchStampReport(report))
	if err != nil {
		return CaptureResearchLineageStampReport{}, err
	}
	report.Digest = digest
	return report, nil
}

func applyResearchLineageStamp(entry fixtureEntry, stamp CaptureResearchLineageStamp) (fixtureEntry, error) {
	entry.Request.Lineage = stamp.Lineage
	if entry.Request.Lineage.SamplingSlot == "" {
		return fixtureEntry{}, errors.New("research lineage stamp requires sampling_slot")
	}
	entry.Request.EvidenceBindings = append([]provider.EvidenceBinding(nil), stamp.EvidenceBindings...)
	if strings.TrimSpace(entry.Response.ServedModel) == "" {
		entry.Response.ServedModel = stamp.ServedModel
	}
	if strings.TrimSpace(entry.Response.CheckpointAssertion) == "" {
		entry.Response.CheckpointAssertion = stamp.CheckpointAssertion
	}
	entry.Response.CapabilityAttestationID = stamp.CapabilityAttestationID
	finalized, err := provider.FinalizeResponse(entry.Request, entry.Response)
	if err != nil {
		return fixtureEntry{}, err
	}
	return newFixtureEntry(entry.Request, finalized)
}

func validateResearchLineageStamp(stamp CaptureResearchLineageStamp) error {
	if err := stamp.Lineage.ValidateResearch(); err != nil {
		return err
	}
	if !validBundleDigest(stamp.Lineage.SourceTraceHash) ||
		!validBundleDigest(stamp.Lineage.TraceMapHash) ||
		!validBundleDigest(stamp.Lineage.PolicyHash) {
		return errors.New("research lineage stamp hashes must be SHA-256 hex")
	}
	if len(stamp.EvidenceBindings) == 0 {
		return errors.New("research lineage stamp requires evidence bindings")
	}
	if !strings.HasPrefix(stamp.CapabilityAttestationID, "att-") ||
		!validBundleDigest(strings.TrimPrefix(stamp.CapabilityAttestationID, "att-")) {
		return errors.New("research lineage stamp requires capability attestation att-<sha256>")
	}
	if strings.TrimSpace(stamp.ServedModel) == "" {
		return errors.New("research lineage stamp requires served_model")
	}
	return nil
}

func exclusiveCapturePaths(source, destination string) (string, string, error) {
	if strings.TrimSpace(source) == "" || strings.TrimSpace(destination) == "" {
		return "", "", errors.New("research lineage stamp requires source and destination paths")
	}
	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return "", "", err
	}
	destAbs, err := filepath.Abs(destination)
	if err != nil {
		return "", "", err
	}
	if sourceAbs == destAbs {
		return "", "", errors.New("research lineage stamp refuses in-place rewrite of a capture")
	}
	if _, err := os.Lstat(destAbs); err == nil {
		return "", "", fmt.Errorf("research lineage stamp destination already exists: %s", destAbs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	return sourceAbs, destAbs, nil
}

func unsignedCaptureResearchAdmission(admission CaptureResearchAdmission) CaptureResearchAdmission {
	admission.Digest = ""
	return admission
}

func unsignedCaptureResearchStampReport(report CaptureResearchLineageStampReport) CaptureResearchLineageStampReport {
	report.Digest = ""
	return report
}
