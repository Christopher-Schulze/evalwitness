package mutation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func DecodeManifest(reader io.Reader) (Manifest, error) {
	var manifest Manifest
	if err := decodeStrict(reader, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode mutation manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func DecodeWitness(reader io.Reader) (Witness, error) {
	var witness Witness
	if err := decodeStrict(reader, &witness); err != nil {
		return Witness{}, fmt.Errorf("decode mutation witness: %w", err)
	}
	if err := witness.Validate(); err != nil {
		return Witness{}, err
	}
	return witness, nil
}

func DecodeBlindReviewPacket(reader io.Reader) (BlindReviewPacket, error) {
	var packet BlindReviewPacket
	if err := decodeStrict(reader, &packet); err != nil {
		return BlindReviewPacket{}, fmt.Errorf("decode blind-review packet: %w", err)
	}
	if err := packet.Validate(); err != nil {
		return BlindReviewPacket{}, err
	}
	return packet, nil
}

func DecodeConstructFirewall(reader io.Reader) (ConstructFirewallReport, error) {
	var report ConstructFirewallReport
	if err := decodeStrict(reader, &report); err != nil {
		return ConstructFirewallReport{}, fmt.Errorf("decode construct firewall report: %w", err)
	}
	if err := report.Validate(); err != nil {
		return ConstructFirewallReport{}, err
	}
	return report, nil
}

func DecodeConstructFirewallV2(reader io.Reader) (ConstructFirewallReportV2, error) {
	var report ConstructFirewallReportV2
	if err := decodeStrict(reader, &report); err != nil {
		return ConstructFirewallReportV2{}, fmt.Errorf("decode construct firewall v2 report: %w", err)
	}
	if err := report.Validate(); err != nil {
		return ConstructFirewallReportV2{}, err
	}
	return report, nil
}

func DecodeConstructRepairEvidence(reader io.Reader) (ConstructRepairEvidence, error) {
	var evidence ConstructRepairEvidence
	if err := decodeStrict(reader, &evidence); err != nil {
		return ConstructRepairEvidence{}, fmt.Errorf("decode construct-repair evidence: %w", err)
	}
	if err := evidence.Validate(); err != nil {
		return ConstructRepairEvidence{}, err
	}
	return evidence, nil
}

func DecodeConstructChallengeEvidence(reader io.Reader) (ConstructChallengeEvidence, error) {
	var evidence ConstructChallengeEvidence
	if err := decodeStrict(reader, &evidence); err != nil {
		return ConstructChallengeEvidence{}, fmt.Errorf("decode construct-firewall challenge: %w", err)
	}
	if err := evidence.Validate(); err != nil {
		return ConstructChallengeEvidence{}, err
	}
	return evidence, nil
}

func DecodeVerificationEvidenceAssessment(reader io.Reader) (VerificationEvidenceAssessment, error) {
	var assessment VerificationEvidenceAssessment
	if err := decodeStrict(reader, &assessment); err != nil {
		return VerificationEvidenceAssessment{}, fmt.Errorf("decode verification-evidence assessment: %w", err)
	}
	if err := assessment.Validate(); err != nil {
		return VerificationEvidenceAssessment{}, err
	}
	return assessment, nil
}

func DecodeVerificationEvidenceChallenge(reader io.Reader) (VerificationEvidenceChallenge, error) {
	var challenge VerificationEvidenceChallenge
	if err := decodeStrict(reader, &challenge); err != nil {
		return VerificationEvidenceChallenge{}, fmt.Errorf("decode verification-evidence challenge: %w", err)
	}
	if err := challenge.Validate(); err != nil {
		return VerificationEvidenceChallenge{}, err
	}
	return challenge, nil
}

func DecodeCorpusSpec(reader io.Reader) (CorpusSpec, error) {
	var spec CorpusSpec
	if err := decodeStrict(reader, &spec); err != nil {
		return CorpusSpec{}, fmt.Errorf("decode corruption corpus spec: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return CorpusSpec{}, err
	}
	return spec, nil
}

func DecodeCorpusDevelopmentPlan(reader io.Reader) (CorpusDevelopmentPlan, error) {
	var plan CorpusDevelopmentPlan
	if err := decodeStrict(reader, &plan); err != nil {
		return CorpusDevelopmentPlan{}, fmt.Errorf("decode corruption corpus development plan: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return CorpusDevelopmentPlan{}, err
	}
	return plan, nil
}

func DecodeCorpusDevelopmentAudit(reader io.Reader, plan CorpusDevelopmentPlan) (CorpusDevelopmentAudit, error) {
	var audit CorpusDevelopmentAudit
	if err := decodeStrict(reader, &audit); err != nil {
		return CorpusDevelopmentAudit{}, fmt.Errorf("decode corruption corpus development audit: %w", err)
	}
	if err := audit.Validate(plan); err != nil {
		return CorpusDevelopmentAudit{}, err
	}
	return audit, nil
}

func DecodeCorpusDevelopmentAuditV3(reader io.Reader, plan CorpusDevelopmentPlan) (CorpusDevelopmentAuditV3, error) {
	var audit CorpusDevelopmentAuditV3
	if err := decodeStrict(reader, &audit); err != nil {
		return CorpusDevelopmentAuditV3{}, fmt.Errorf("decode v3 corruption corpus development audit: %w", err)
	}
	if err := audit.Validate(plan); err != nil {
		return CorpusDevelopmentAuditV3{}, err
	}
	return audit, nil
}

func DecodeCorpusRelease(reader io.Reader) (CorpusRelease, error) {
	var release CorpusRelease
	if err := decodeStrict(reader, &release); err != nil {
		return CorpusRelease{}, fmt.Errorf("decode corruption corpus release: %w", err)
	}
	if err := release.Validate(); err != nil {
		return CorpusRelease{}, err
	}
	return release, nil
}

func DecodeCorpusReleaseV3(reader io.Reader, plan CorpusDevelopmentPlan, audit CorpusDevelopmentAuditV3) (CorpusReleaseV3, error) {
	var release CorpusReleaseV3
	if err := decodeStrict(reader, &release); err != nil {
		return CorpusReleaseV3{}, fmt.Errorf("decode v3 corruption corpus release: %w", err)
	}
	if err := release.Validate(plan, audit); err != nil {
		return CorpusReleaseV3{}, err
	}
	return release, nil
}

func EncodeIndented(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func decodeStrict(reader io.Reader, target any) error {
	limited := io.LimitReader(reader, MaximumMutationDocumentSize+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(raw) > MaximumMutationDocumentSize {
		return errors.New("mutation document exceeds 16 MiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("mutation document contains more than one JSON value")
		}
		return fmt.Errorf("decode trailing mutation input: %w", err)
	}
	return nil
}
