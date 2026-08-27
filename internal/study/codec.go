package study

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const MaximumDocumentBytes = 16 * 1024 * 1024

func DecodeManifest(reader io.Reader) (Manifest, error) {
	var manifest Manifest
	if err := DecodeStrict(reader, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode study manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func DecodeRecord(reader io.Reader) (Record, error) {
	var record Record
	if err := DecodeStrict(reader, &record); err != nil {
		return Record{}, fmt.Errorf("decode study record: %w", err)
	}
	if err := record.Validate(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func EncodeIndented(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func DecodeStrict(reader io.Reader, target any) error {
	limited := io.LimitReader(reader, MaximumDocumentBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(raw) > MaximumDocumentBytes {
		return errors.New("document exceeds 16 MiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("document contains more than one JSON value")
		}
		return fmt.Errorf("decode trailing input: %w", err)
	}
	return nil
}
