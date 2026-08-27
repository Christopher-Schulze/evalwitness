package outcome

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func DecodePlan(reader io.Reader) (Plan, error) {
	var value Plan
	if err := decodeStrict(reader, &value); err != nil {
		return Plan{}, fmt.Errorf("decode outcome plan: %w", err)
	}
	return value, value.Validate()
}

func DecodeRecord(reader io.Reader) (Record, error) {
	var value Record
	if err := decodeStrict(reader, &value); err != nil {
		return Record{}, fmt.Errorf("decode outcome record: %w", err)
	}
	return value, value.Validate()
}

func DecodeBlindPacket(reader io.Reader) (BlindPacket, error) {
	var value BlindPacket
	if err := decodeStrict(reader, &value); err != nil {
		return BlindPacket{}, fmt.Errorf("decode outcome packet: %w", err)
	}
	return value, value.Validate()
}

func DecodeBlindBuildRequest(reader io.Reader) (BlindBuildRequest, error) {
	var value BlindBuildRequest
	if err := decodeStrict(reader, &value); err != nil {
		return BlindBuildRequest{}, fmt.Errorf("decode blind build request: %w", err)
	}
	return value, value.Validate()
}

func DecodeLabel(reader io.Reader) (Label, error) {
	var value Label
	if err := decodeStrict(reader, &value); err != nil {
		return Label{}, fmt.Errorf("decode outcome label: %w", err)
	}
	return value, value.Validate()
}

func DecodePrivateMapping(reader io.Reader) (PrivateMapping, error) {
	var value PrivateMapping
	if err := decodeStrict(reader, &value); err != nil {
		return PrivateMapping{}, fmt.Errorf("decode outcome private mapping: %w", err)
	}
	return value, value.Validate()
}

func DecodeResolution(reader io.Reader) (Resolution, error) {
	var value Resolution
	if err := decodeStrict(reader, &value); err != nil {
		return Resolution{}, fmt.Errorf("decode outcome resolution: %w", err)
	}
	return value, value.Validate()
}

func DecodeAgreementReport(reader io.Reader) (AgreementReport, error) {
	var value AgreementReport
	if err := decodeStrict(reader, &value); err != nil {
		return AgreementReport{}, fmt.Errorf("decode outcome agreement report: %w", err)
	}
	return value, value.Validate()
}

func DecodePreservation(reader io.Reader) (Preservation, error) {
	var value Preservation
	if err := decodeStrict(reader, &value); err != nil {
		return Preservation{}, fmt.Errorf("decode outcome preservation: %w", err)
	}
	return value, value.Validate()
}

func DecodeAgreementPairs(reader io.Reader) ([]AgreementPair, error) {
	var value []AgreementPair
	if err := decodeStrict(reader, &value); err != nil {
		return nil, fmt.Errorf("decode outcome agreement pairs: %w", err)
	}
	return value, nil
}

func DecodeSampleCommitment(reader io.Reader) (SampleCommitment, error) {
	var value SampleCommitment
	if err := decodeStrict(reader, &value); err != nil {
		return SampleCommitment{}, fmt.Errorf("decode outcome sample commitment: %w", err)
	}
	return value, value.Validate()
}

func EncodeIndented(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func decodeStrict(reader io.Reader, target any) error {
	raw, err := io.ReadAll(io.LimitReader(reader, MaximumDocumentSize+1))
	if err != nil {
		return err
	}
	if len(raw) > MaximumDocumentSize {
		return errors.New("outcome document exceeds 16 MiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("outcome document contains more than one JSON value")
		}
		return err
	}
	return nil
}

func digestJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
