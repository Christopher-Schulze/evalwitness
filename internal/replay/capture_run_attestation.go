package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	CaptureRunAttestationSchemaVersion = "evalwitness.capture-run-attestation.v1"
	CaptureRunInspectionExactReplay    = "exact_replay"
	CaptureRunInspectionSchema3Census  = "schema3_census"
	CaptureRunStatusComplete           = "complete"
	CaptureRunStatusIncomplete         = "incomplete"
)

// CaptureRunAttestation binds a schema-3 capture file to an authorized call
// budget. complete is only issued for exact-replay inspections with matching
// call counts and research lineage on every record.
type CaptureRunAttestation struct {
	SchemaVersion           string            `json:"schema_version"`
	AuthorizedCalls         int               `json:"authorized_calls"`
	ObservedCalls           int               `json:"observed_calls"`
	AttemptLedgerReconciled bool              `json:"attempt_ledger_reconciled"`
	ResearchLineageComplete bool              `json:"research_lineage_complete"`
	InspectionMode          string            `json:"inspection_mode"`
	Status                  string            `json:"status"`
	Limitations             []string          `json:"limitations"`
	Inspection              CaptureInspection `json:"inspection"`
	Digest                  string            `json:"digest"`
}

func InspectCaptureFile(path string) (CaptureInspection, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return CaptureInspection{}, err
	}
	inspection, _, err := inspectCapturePayload(payload)
	return inspection, err
}

func SealCaptureRunAttestation(path string, authorizedCalls int) (CaptureRunAttestation, error) {
	if authorizedCalls < 1 {
		return CaptureRunAttestation{}, errors.New("authorized call budget must be at least 1")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return CaptureRunAttestation{}, err
	}
	return SealCaptureRunAttestationPayload(payload, authorizedCalls)
}

// SealCaptureRunAttestationPayload seals an already loaded capture without
// introducing a path-dependent verification boundary.
func SealCaptureRunAttestationPayload(payload []byte, authorizedCalls int) (CaptureRunAttestation, error) {
	if authorizedCalls < 1 {
		return CaptureRunAttestation{}, errors.New("authorized call budget must be at least 1")
	}
	inspection, mode, limitations, err := inspectCaptureForAttestation(payload)
	if err != nil {
		return CaptureRunAttestation{}, err
	}
	attestation := CaptureRunAttestation{
		SchemaVersion:           CaptureRunAttestationSchemaVersion,
		AuthorizedCalls:         authorizedCalls,
		ObservedCalls:           inspection.Entries,
		AttemptLedgerReconciled: inspection.Entries == authorizedCalls,
		ResearchLineageComplete: inspection.CompleteResearchEntries == inspection.Entries &&
			inspection.Entries > 0 && len(inspection.CapabilityAttestationIDs) > 0,
		InspectionMode: mode,
		Limitations:    slices.Clone(limitations),
		Inspection:     inspection,
	}
	if !attestation.AttemptLedgerReconciled {
		attestation.Limitations = append(attestation.Limitations,
			fmt.Sprintf("observed %d records, authorized budget %d", inspection.Entries, authorizedCalls))
	}
	if !attestation.ResearchLineageComplete {
		attestation.Limitations = append(attestation.Limitations,
			"capability attestation and complete research lineage are not stamped on every record")
	}
	if mode != CaptureRunInspectionExactReplay {
		attestation.Limitations = append(attestation.Limitations,
			"exact-replay inspection is unavailable; schema-3 census does not raise the evidence ceiling")
	}
	slices.Sort(attestation.Limitations)
	attestation.Limitations = slices.Compact(attestation.Limitations)
	if attestation.Limitations == nil {
		attestation.Limitations = []string{}
	}
	if attestation.AttemptLedgerReconciled && attestation.ResearchLineageComplete &&
		mode == CaptureRunInspectionExactReplay {
		attestation.Status = CaptureRunStatusComplete
	} else {
		attestation.Status = CaptureRunStatusIncomplete
	}
	digest, err := protocol.Digest(unsignedCaptureRunAttestation(attestation))
	if err != nil {
		return CaptureRunAttestation{}, err
	}
	attestation.Digest = digest
	return attestation, nil
}

func VerifyCaptureRunAttestation(path string, attestation CaptureRunAttestation) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return VerifyCaptureRunAttestationPayload(payload, attestation)
}

// VerifyCaptureRunAttestationPayload verifies a sealed attestation against
// exact capture bytes supplied by a capsule-family verifier.
func VerifyCaptureRunAttestationPayload(payload []byte, attestation CaptureRunAttestation) error {
	sealed, err := SealCaptureRunAttestationPayload(payload, attestation.AuthorizedCalls)
	if err != nil {
		return err
	}
	if attestation.SchemaVersion != CaptureRunAttestationSchemaVersion ||
		attestation.Digest != sealed.Digest ||
		attestation.Status != sealed.Status ||
		attestation.InspectionMode != sealed.InspectionMode ||
		attestation.ObservedCalls != sealed.ObservedCalls ||
		attestation.Inspection.PayloadSHA256 != sealed.Inspection.PayloadSHA256 {
		return errors.New("capture-run attestation does not match the capture file")
	}
	want, err := protocol.Digest(unsignedCaptureRunAttestation(attestation))
	if err != nil || want != attestation.Digest {
		return errors.New("capture-run attestation digest is invalid")
	}
	return nil
}

func unsignedCaptureRunAttestation(attestation CaptureRunAttestation) CaptureRunAttestation {
	attestation.Digest = ""
	return attestation
}

func inspectCaptureForAttestation(payload []byte) (CaptureInspection, string, []string, error) {
	inspection, _, err := inspectCapturePayload(payload)
	if err == nil {
		return inspection, CaptureRunInspectionExactReplay, nil, nil
	}
	if !strings.Contains(err.Error(), "deterministic JSON form") {
		return CaptureInspection{}, "", nil, err
	}
	census, censusErr := inspectSchema3Census(payload)
	if censusErr != nil {
		return CaptureInspection{}, "", nil, fmt.Errorf("%w (schema-3 census also failed: %v)", err, censusErr)
	}
	return census, CaptureRunInspectionSchema3Census, []string{err.Error()}, nil
}

type schema3CaptureHeader struct {
	CaptureSchemaVersion int `json:"capture_schema_version"`
}

func inspectSchema3Census(payload []byte) (CaptureInspection, error) {
	if len(payload) == 0 || int64(len(payload)) > responseBundleMaxCaptureBytes || payload[len(payload)-1] != '\n' {
		return CaptureInspection{}, errors.New("schema-3 capture must be a non-empty, newline-terminated bounded JSONL file")
	}
	entries := 0
	for _, line := range bytes.Split(payload[:len(payload)-1], []byte("\n")) {
		if len(line) == 0 {
			return CaptureInspection{}, errors.New("schema-3 capture contains a blank record")
		}
		var header schema3CaptureHeader
		if err := json.Unmarshal(line, &header); err != nil {
			return CaptureInspection{}, fmt.Errorf("parse schema-3 capture: %w", err)
		}
		if header.CaptureSchemaVersion != provider.CaptureSchemaVersion {
			return CaptureInspection{}, fmt.Errorf("schema-3 census found capture_schema_version %d", header.CaptureSchemaVersion)
		}
		entries++
	}
	if entries == 0 {
		return CaptureInspection{}, errors.New("schema-3 capture contains no records")
	}
	return CaptureInspection{
		PayloadSHA256:            protocol.DigestBytes(payload),
		Bytes:                    int64(len(payload)),
		Entries:                  entries,
		StudyCellIDs:             []string{},
		CaptureSchemaVersion:     provider.CaptureSchemaVersion,
		ServedModels:             []string{},
		CheckpointAssertions:     []string{},
		CapabilityAttestationIDs: []string{},
	}, nil
}
