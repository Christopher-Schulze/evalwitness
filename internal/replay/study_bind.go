package replay

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/Christopher-Schulze/evalwitness/protocol"
)

const (
	StudyBindSchemaVersion    = "evalwitness.identical-response-050-bind.v1"
	StudyBindStatusIncomplete = "incomplete"
	StudyBindStatusComplete   = "complete"
	StudyBindEvidenceCeiling  = "mechanism_conformance"
)

type StudyBindInput struct {
	CapturePath         string
	AuthorizedCalls     int
	AttestationPath     string
	AdmissionPath       string
	ClaimLedgerPath     string
	BundlePolicyPath    string
	StudyRecordPath     string
	OfflineAnalysisPath string
}

type StudyBindCertificate struct {
	SchemaVersion               string   `json:"schema_version"`
	BindStatus                  string   `json:"bind_status"`
	EvidenceCeiling             string   `json:"evidence_ceiling"`
	CapturePayloadSHA256        string   `json:"capture_payload_sha256"`
	AuthorizedCalls             int      `json:"authorized_calls"`
	ObservedCalls               int      `json:"observed_calls"`
	CaptureRunStatus            string   `json:"capture_run_status"`
	CaptureRunAttestationDigest string   `json:"capture_run_attestation_digest"`
	ResearchLineageComplete     bool     `json:"research_lineage_complete"`
	Admission                   string   `json:"admission"`
	AdmissionDigest             string   `json:"admission_digest"`
	ClaimLedgerSHA256           string   `json:"claim_ledger_sha256"`
	BundlePolicySHA256          string   `json:"bundle_policy_sha256,omitempty"`
	StudyRecordSHA256           string   `json:"study_record_sha256,omitempty"`
	OfflineAnalysisSHA256       string   `json:"offline_analysis_sha256,omitempty"`
	MissingParents              []string `json:"missing_parents"`
	Limitations                 []string `json:"limitations"`
	Digest                      string   `json:"digest"`
}

func BindStudyEvidence(input StudyBindInput) (StudyBindCertificate, error) {
	if input.CapturePath == "" || input.AuthorizedCalls < 1 || input.AttestationPath == "" ||
		input.AdmissionPath == "" || input.ClaimLedgerPath == "" {
		return StudyBindCertificate{}, errors.New("study bind requires capture, authorized-calls, attestation, admission, and claim-ledger")
	}
	rawAttestation, err := os.ReadFile(input.AttestationPath)
	if err != nil {
		return StudyBindCertificate{}, err
	}
	var attestation CaptureRunAttestation
	if err := protocol.DecodeStrict(bytes.TrimSpace(rawAttestation), &attestation); err != nil {
		return StudyBindCertificate{}, fmt.Errorf("decode capture-run attestation: %w", err)
	}
	if err := VerifyCaptureRunAttestation(input.CapturePath, attestation); err != nil {
		return StudyBindCertificate{}, err
	}
	if input.AuthorizedCalls != attestation.AuthorizedCalls {
		return StudyBindCertificate{}, fmt.Errorf("authorized-calls %d does not match capture-run attestation %d", input.AuthorizedCalls, attestation.AuthorizedCalls)
	}
	rawAdmission, err := os.ReadFile(input.AdmissionPath)
	if err != nil {
		return StudyBindCertificate{}, err
	}
	var admission CaptureResearchAdmission
	if err := protocol.DecodeStrict(bytes.TrimSpace(rawAdmission), &admission); err != nil {
		return StudyBindCertificate{}, fmt.Errorf("decode research admission: %w", err)
	}
	freshAdmission, err := AdmitCaptureResearchLineage(input.CapturePath, input.AuthorizedCalls)
	if err != nil {
		return StudyBindCertificate{}, err
	}
	if admission.Digest != freshAdmission.Digest || admission.PayloadSHA256 != freshAdmission.PayloadSHA256 ||
		admission.Admission != freshAdmission.Admission || admission.AuthorizedCalls != freshAdmission.AuthorizedCalls {
		return StudyBindCertificate{}, errors.New("research admission does not match the capture")
	}
	ledgerSHA, err := fileSHA256(input.ClaimLedgerPath)
	if err != nil {
		return StudyBindCertificate{}, err
	}
	certificate := StudyBindCertificate{
		SchemaVersion:               StudyBindSchemaVersion,
		EvidenceCeiling:             StudyBindEvidenceCeiling,
		CapturePayloadSHA256:        attestation.Inspection.PayloadSHA256,
		AuthorizedCalls:             attestation.AuthorizedCalls,
		ObservedCalls:               attestation.ObservedCalls,
		CaptureRunStatus:            attestation.Status,
		CaptureRunAttestationDigest: attestation.Digest,
		ResearchLineageComplete:     attestation.ResearchLineageComplete,
		Admission:                   admission.Admission,
		AdmissionDigest:             admission.Digest,
		ClaimLedgerSHA256:           ledgerSHA,
		MissingParents:              []string{},
		Limitations:                 slices.Clone(attestation.Limitations),
	}
	if certificate.Limitations == nil {
		certificate.Limitations = []string{}
	}
	if input.BundlePolicyPath != "" {
		if certificate.BundlePolicySHA256, err = fileSHA256(input.BundlePolicyPath); err != nil {
			return StudyBindCertificate{}, err
		}
	} else {
		certificate.MissingParents = append(certificate.MissingParents, "bundle_policy")
	}
	if input.StudyRecordPath != "" {
		if certificate.StudyRecordSHA256, err = fileSHA256(input.StudyRecordPath); err != nil {
			return StudyBindCertificate{}, err
		}
	} else {
		certificate.MissingParents = append(certificate.MissingParents, "study_record")
	}
	if input.OfflineAnalysisPath != "" {
		if certificate.OfflineAnalysisSHA256, err = fileSHA256(input.OfflineAnalysisPath); err != nil {
			return StudyBindCertificate{}, err
		}
	} else {
		certificate.MissingParents = append(certificate.MissingParents, "offline_analysis")
	}
	if attestation.Status != CaptureRunStatusComplete || !attestation.ResearchLineageComplete ||
		admission.Admission != CaptureResearchAdmissionAdmitted {
		certificate.MissingParents = append(certificate.MissingParents, "complete_research_lineage")
		certificate.Limitations = append(certificate.Limitations,
			"TASK 050 bind does not raise the evidence ceiling: research lineage is incomplete")
	}
	certificate.Limitations = append(certificate.Limitations,
		"sidecar claim ledger is identity-bound only; it is not a sealed evalwitness.claim-ledger.v1 over a TASK 050 capsule manifest")
	if input.OfflineAnalysisPath == "" {
		certificate.Limitations = append(certificate.Limitations,
			"offline identical-response analysis is not bound")
	}
	slices.Sort(certificate.MissingParents)
	certificate.MissingParents = slices.Compact(certificate.MissingParents)
	slices.Sort(certificate.Limitations)
	certificate.Limitations = slices.Compact(certificate.Limitations)
	if attestation.Status == CaptureRunStatusComplete && attestation.ResearchLineageComplete &&
		admission.Admission == CaptureResearchAdmissionAdmitted && len(certificate.MissingParents) == 0 {
		certificate.BindStatus = StudyBindStatusComplete
	} else {
		certificate.BindStatus = StudyBindStatusIncomplete
	}
	digest, err := protocol.Digest(unsignedStudyBindCertificate(certificate))
	if err != nil {
		return StudyBindCertificate{}, err
	}
	certificate.Digest = digest
	return certificate, nil
}

func unsignedStudyBindCertificate(certificate StudyBindCertificate) StudyBindCertificate {
	certificate.Digest = ""
	return certificate
}

func fileSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return protocol.DigestBytes(raw), nil
}
