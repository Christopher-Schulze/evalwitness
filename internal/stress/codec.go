package stress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const maximumStressDocumentBytes = 16 << 20

func EncodeIndented(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode stress document: %w", err)
	}
	return append(encoded, '\n'), nil
}

func DecodeDevelopmentCaseStudy(reader io.Reader) (DevelopmentCaseStudy, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return DevelopmentCaseStudy{}, fmt.Errorf("read stress development case study: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return DevelopmentCaseStudy{}, errors.New("stress development case study exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value DevelopmentCaseStudy
	if err := decoder.Decode(&value); err != nil {
		return DevelopmentCaseStudy{}, fmt.Errorf("decode stress development case study: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return DevelopmentCaseStudy{}, errors.New("stress development case study has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutRunReadinessRefusal(reader io.Reader) (HeldOutRunReadinessRefusal, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutRunReadinessRefusal{}, fmt.Errorf("read stress held-out readiness refusal: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutRunReadinessRefusal{}, errors.New("stress held-out readiness refusal exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutRunReadinessRefusal
	if err := decoder.Decode(&value); err != nil {
		return HeldOutRunReadinessRefusal{}, fmt.Errorf("decode stress held-out readiness refusal: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutRunReadinessRefusal{}, errors.New("stress held-out readiness refusal has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutCampaignPlan(reader io.Reader) (HeldOutCampaignPlan, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutCampaignPlan{}, fmt.Errorf("read stress held-out campaign plan: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutCampaignPlan{}, errors.New("stress held-out campaign plan exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutCampaignPlan
	if err := decoder.Decode(&value); err != nil {
		return HeldOutCampaignPlan{}, fmt.Errorf("decode stress held-out campaign plan: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutCampaignPlan{}, errors.New("stress held-out campaign plan has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutCampaignBatchBinding(reader io.Reader) (HeldOutCampaignBatchBinding, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutCampaignBatchBinding{}, fmt.Errorf("read stress held-out campaign batch binding: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutCampaignBatchBinding{}, errors.New("stress held-out campaign batch binding exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutCampaignBatchBinding
	if err := decoder.Decode(&value); err != nil {
		return HeldOutCampaignBatchBinding{}, fmt.Errorf("decode stress held-out campaign batch binding: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutCampaignBatchBinding{}, errors.New("stress held-out campaign batch binding has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutAdmissionPlan(reader io.Reader) (HeldOutAdmissionPlan, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutAdmissionPlan{}, fmt.Errorf("read stress held-out admission plan: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutAdmissionPlan{}, errors.New("stress held-out admission plan exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutAdmissionPlan
	if err := decoder.Decode(&value); err != nil {
		return HeldOutAdmissionPlan{}, fmt.Errorf("decode stress held-out admission plan: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutAdmissionPlan{}, errors.New("stress held-out admission plan has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutExecutionBatchBinding(reader io.Reader) (HeldOutExecutionBatchBinding, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutExecutionBatchBinding{}, fmt.Errorf("read stress held-out execution binding: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutExecutionBatchBinding{}, errors.New("stress held-out execution binding exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutExecutionBatchBinding
	if err := decoder.Decode(&value); err != nil {
		return HeldOutExecutionBatchBinding{}, fmt.Errorf("decode stress held-out execution binding: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutExecutionBatchBinding{}, errors.New("stress held-out execution binding has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutPreflightEvidence(reader io.Reader) (HeldOutPreflightEvidence, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutPreflightEvidence{}, fmt.Errorf("read stress held-out preflight evidence: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutPreflightEvidence{}, errors.New("stress held-out preflight evidence exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutPreflightEvidence
	if err := decoder.Decode(&value); err != nil {
		return HeldOutPreflightEvidence{}, fmt.Errorf("decode stress held-out preflight evidence: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutPreflightEvidence{}, errors.New("stress held-out preflight evidence has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutPreflightCustody(reader io.Reader) (HeldOutPreflightCustody, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutPreflightCustody{}, fmt.Errorf("read stress held-out preflight custody: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight custody exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutPreflightCustody
	if err := decoder.Decode(&value); err != nil {
		return HeldOutPreflightCustody{}, fmt.Errorf("decode stress held-out preflight custody: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutPreflightCustody{}, errors.New("stress held-out preflight custody has trailing JSON")
	}
	return value, value.Validate()
}

func DecodeHeldOutExecutionPermit(reader io.Reader) (HeldOutExecutionPermit, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maximumStressDocumentBytes+1))
	if err != nil {
		return HeldOutExecutionPermit{}, fmt.Errorf("read stress held-out execution permit: %w", err)
	}
	if len(raw) > maximumStressDocumentBytes {
		return HeldOutExecutionPermit{}, errors.New("stress held-out execution permit exceeds the maximum document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value HeldOutExecutionPermit
	if err := decoder.Decode(&value); err != nil {
		return HeldOutExecutionPermit{}, fmt.Errorf("decode stress held-out execution permit: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return HeldOutExecutionPermit{}, errors.New("stress held-out execution permit has trailing JSON")
	}
	return value, value.Validate()
}
