package capsule

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maximumReferenceDocumentBytes = 64 << 20

func addReferenceFile(repositoryRoot string, registry *Registry, name, typeID string, visibility Visibility, relative string, parents []ParentRef, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	raw, err := readReferenceFile(repositoryRoot, relative)
	if err != nil {
		return ComponentRecord{}, err
	}
	return addReferencePayload(registry, ComponentInput{
		Name: name, TypeID: typeID, Visibility: visibility, Payload: raw, Parents: parents,
	}, records, payloads)
}

func addReferencePayload(registry *Registry, input ComponentInput, records *[]ComponentRecord, payloads map[string][]byte) (ComponentRecord, error) {
	record, normalized, err := BuildComponent(registry, input)
	if err != nil {
		return ComponentRecord{}, err
	}
	if existing, found := payloads[record.Payload.Digest]; found && !bytes.Equal(existing, normalized) {
		return ComponentRecord{}, errors.New("reference package contains a payload digest collision")
	}
	payloads[record.Payload.Digest] = normalized
	*records = append(*records, record)
	return record, nil
}

func readReferenceFile(repositoryRoot, relative string) ([]byte, error) {
	local := filepath.FromSlash(relative)
	if !filepath.IsLocal(local) {
		return nil, fmt.Errorf("reference artifact path %q is not local", relative)
	}
	path := filepath.Join(repositoryRoot, local)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect reference artifact %q: %w", relative, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("reference artifact %q is not a regular file", relative)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read reference artifact %q: %w", relative, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("reference artifact %q is empty", relative)
	}
	return raw, nil
}

func mergeComponentTypes(groups ...[]ComponentType) []ComponentType {
	var merged []ComponentType
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return merged
}

func mergePayloadValidators(groups ...map[string]PayloadValidator) (map[string]PayloadValidator, error) {
	merged := make(map[string]PayloadValidator)
	for _, group := range groups {
		for id, validator := range group {
			if _, duplicate := merged[id]; duplicate {
				return nil, fmt.Errorf("duplicate reference payload validator %q", id)
			}
			merged[id] = validator
		}
	}
	return merged, nil
}

func mergeBindingValidators(groups ...map[string]BindingValidator) (map[string]BindingValidator, error) {
	merged := make(map[string]BindingValidator)
	for _, group := range groups {
		for id, validator := range group {
			if _, duplicate := merged[id]; duplicate {
				return nil, fmt.Errorf("duplicate reference binding validator %q", id)
			}
			merged[id] = validator
		}
	}
	return merged, nil
}

func decodeReferenceJSON(payload []byte, destination any) error {
	if len(payload) == 0 || len(payload) > maximumReferenceDocumentBytes {
		return errors.New("reference JSON payload is empty or exceeds its size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("reference JSON payload contains trailing data")
	}
	return nil
}
