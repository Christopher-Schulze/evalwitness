package protocol

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

const VectorCorpusSchema = "evalwitness.protocol.vector-corpus.v1"

//go:embed vectors/*.json schemas/*.json
var protocolArtifacts embed.FS

type MutationVector struct {
	CaseID      string           `json:"case_id"`
	BaseCaseID  string           `json:"base_case_id"`
	JSONPointer string           `json:"json_pointer"`
	Operation   string           `json:"operation"`
	Value       json.RawMessage  `json:"value,omitempty"`
	Level       ConformanceLevel `json:"conformance_level"`
	Kind        CaseKind         `json:"kind"`
	Expected    ExpectedOutcome  `json:"expected"`
	Description string           `json:"description"`
}

type VectorCorpusManifest struct {
	SchemaVersion   string            `json:"schema_version"`
	ProtocolVersion string            `json:"protocol_version"`
	Cases           []json.RawMessage `json:"cases"`
	Mutations       []MutationVector  `json:"mutations"`
}

type RequiredFieldVector struct {
	CaseID      string `json:"case_id"`
	BaseCaseID  string `json:"base_case_id"`
	JSONPointer string `json:"json_pointer"`
}

type RequiredFieldManifest struct {
	SchemaVersion string                `json:"schema_version"`
	Cases         []RequiredFieldVector `json:"cases"`
}

type CaseVector struct {
	CaseID      string           `json:"case_id"`
	Level       ConformanceLevel `json:"conformance_level"`
	Kind        CaseKind         `json:"kind"`
	Description string           `json:"description"`
	Expected    ExpectedOutcome  `json:"expected"`
	Raw         json.RawMessage  `json:"case"`
}

type NormativeCorpus struct {
	Vectors              []CaseVector
	Digest               string
	RequestCorpusDigest  string
	SchemaArtifactDigest string
}

type frozenRequestCorpus struct {
	CorpusSchema         string `json:"corpus_schema"`
	RequestSchemaVersion int    `json:"request_schema_version"`
	Digest               string `json:"digest"`
	NumericContract      string `json:"numeric_contract"`
	Cases                []struct {
		Name             string          `json:"name"`
		Request          json.RawMessage `json:"request"`
		CanonicalUTF8Hex string          `json:"canonical_utf8_hex"`
		Fingerprint      string          `json:"fingerprint_sha256"`
	} `json:"cases"`
}

func LoadNormativeCorpus(requestCorpusRaw []byte) (NormativeCorpus, error) {
	manifestRaw, err := protocolArtifacts.ReadFile("vectors/normative-cases.json")
	if err != nil {
		return NormativeCorpus{}, fmt.Errorf("read embedded protocol vectors: %w", err)
	}
	var manifest VectorCorpusManifest
	if err := DecodeStrict(manifestRaw, &manifest); err != nil {
		return NormativeCorpus{}, fmt.Errorf("decode protocol vector manifest: %w", err)
	}
	if manifest.SchemaVersion != VectorCorpusSchema || manifest.ProtocolVersion != CurrentVersion {
		return NormativeCorpus{}, errors.New("protocol vector corpus schema or version is invalid")
	}
	vectors := make([]CaseVector, 0, len(manifest.Cases)+len(manifest.Mutations)+8)
	bases := make(map[string]json.RawMessage, len(manifest.Cases))
	baseCases := make(map[string]AuditCase, len(manifest.Cases))
	seen := make(map[string]struct{}, len(manifest.Cases)+len(manifest.Mutations)+8)
	for _, sourceRaw := range manifest.Cases {
		var auditCase AuditCase
		if err := DecodeStrict(sourceRaw, &auditCase); err != nil {
			return NormativeCorpus{}, fmt.Errorf("decode positive vector: %w", err)
		}
		if err := validateAuditCaseRequiredFields(sourceRaw); err != nil {
			return NormativeCorpus{}, fmt.Errorf("validate positive vector %s: %w", auditCase.CaseID, err)
		}
		raw, err := CanonicalizeJSON(sourceRaw)
		if err != nil {
			return NormativeCorpus{}, fmt.Errorf("canonicalize vector %s: %w", auditCase.CaseID, err)
		}
		if err := appendVector(&vectors, seen, auditCase.CaseID, auditCase.Level, auditCase.Kind, auditCase.Description, auditCase.Expected, raw); err != nil {
			return NormativeCorpus{}, err
		}
		bases[auditCase.CaseID] = raw
		baseCases[auditCase.CaseID] = auditCase
	}
	for _, mutation := range manifest.Mutations {
		base, ok := bases[mutation.BaseCaseID]
		if !ok {
			return NormativeCorpus{}, fmt.Errorf("mutation %s references unknown base %s", mutation.CaseID, mutation.BaseCaseID)
		}
		mutated, err := applyJSONMutation(base, mutation)
		if err != nil {
			return NormativeCorpus{}, fmt.Errorf("apply mutation %s: %w", mutation.CaseID, err)
		}
		if err := appendVector(&vectors, seen, mutation.CaseID, mutation.Level, mutation.Kind, mutation.Description, mutation.Expected, mutated); err != nil {
			return NormativeCorpus{}, err
		}
	}
	requestDigest := ""
	if len(requestCorpusRaw) > 0 {
		requestVectors, digest, err := requestCaseVectors(requestCorpusRaw)
		if err != nil {
			return NormativeCorpus{}, err
		}
		requestDigest = digest
		for _, vector := range requestVectors {
			if err := appendVector(&vectors, seen, vector.CaseID, vector.Level, vector.Kind, vector.Description, vector.Expected, vector.Raw); err != nil {
				return NormativeCorpus{}, err
			}
			var auditCase AuditCase
			if err := DecodeStrict(vector.Raw, &auditCase); err != nil {
				return NormativeCorpus{}, fmt.Errorf("decode generated request vector %s: %w", vector.CaseID, err)
			}
			bases[vector.CaseID] = vector.Raw
			baseCases[vector.CaseID] = auditCase
		}
	}
	requiredRaw, err := protocolArtifacts.ReadFile("vectors/required-fields.json")
	if err != nil {
		return NormativeCorpus{}, fmt.Errorf("read required-field vectors: %w", err)
	}
	var requiredManifest RequiredFieldManifest
	if err := DecodeStrict(requiredRaw, &requiredManifest); err != nil {
		return NormativeCorpus{}, fmt.Errorf("decode required-field vectors: %w", err)
	}
	if requiredManifest.SchemaVersion != "evalwitness.protocol.required-field-corpus.v1" {
		return NormativeCorpus{}, errors.New("required-field vector corpus schema is invalid")
	}
	for _, required := range requiredManifest.Cases {
		base, ok := bases[required.BaseCaseID]
		baseCase, metadataOK := baseCases[required.BaseCaseID]
		if !ok || !metadataOK {
			return NormativeCorpus{}, fmt.Errorf("required-field vector %s references unknown base %s", required.CaseID, required.BaseCaseID)
		}
		mutation := MutationVector{
			CaseID: required.CaseID, BaseCaseID: required.BaseCaseID, JSONPointer: required.JSONPointer,
			Operation: "remove", Level: baseCase.Level, Kind: baseCase.Kind,
			Expected:    ExpectedOutcome{Status: ExpectationRejected},
			Description: "rejects missing required field " + required.JSONPointer,
		}
		mutated, mutationErr := applyJSONMutation(base, mutation)
		if mutationErr != nil {
			return NormativeCorpus{}, fmt.Errorf("apply required-field mutation %s: %w", required.CaseID, mutationErr)
		}
		if err := appendVector(&vectors, seen, mutation.CaseID, mutation.Level, mutation.Kind, mutation.Description, mutation.Expected, mutated); err != nil {
			return NormativeCorpus{}, err
		}
	}
	sort.SliceStable(vectors, func(i, j int) bool { return vectors[i].CaseID < vectors[j].CaseID })
	material := make([]json.RawMessage, 0, len(vectors))
	for _, vector := range vectors {
		material = append(material, vector.Raw)
	}
	digest, err := Digest(material)
	if err != nil {
		return NormativeCorpus{}, err
	}
	schemaDigest, err := embeddedSchemaDigest()
	if err != nil {
		return NormativeCorpus{}, err
	}
	return NormativeCorpus{Vectors: vectors, Digest: digest, RequestCorpusDigest: requestDigest, SchemaArtifactDigest: schemaDigest}, nil
}

func ReadSchemaArtifact(name string) ([]byte, error) {
	if !identifierPattern.MatchString(strings.TrimSuffix(name, ".schema.json")) || !strings.HasSuffix(name, ".schema.json") {
		return nil, errors.New("protocol schema name is invalid")
	}
	raw, err := protocolArtifacts.ReadFile("schemas/" + name)
	if err != nil {
		return nil, fmt.Errorf("read protocol schema %q: %w", name, err)
	}
	return append([]byte(nil), raw...), nil
}

func ReadVectorArtifact(name string) ([]byte, error) {
	if name != "normative-cases.json" && name != "required-fields.json" {
		return nil, errors.New("protocol vector artifact name is invalid")
	}
	raw, err := protocolArtifacts.ReadFile("vectors/" + name)
	if err != nil {
		return nil, fmt.Errorf("read protocol vector artifact %q: %w", name, err)
	}
	return append([]byte(nil), raw...), nil
}

func requestCaseVectors(raw []byte) ([]CaseVector, string, error) {
	var corpus frozenRequestCorpus
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return nil, "", fmt.Errorf("decode frozen request corpus: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, "", errors.New("frozen request corpus contains trailing JSON")
	}
	if corpus.CorpusSchema != "evalwitness.request-fingerprint-corpus.v2" || corpus.RequestSchemaVersion != 2 ||
		corpus.Digest != "sha256" || corpus.NumericContract == "" || len(corpus.Cases) != 4 {
		return nil, "", errors.New("frozen request corpus contract is unsupported")
	}
	vectors := make([]CaseVector, 0, len(corpus.Cases))
	seenNames := make(map[string]struct{}, len(corpus.Cases))
	for _, item := range corpus.Cases {
		if !identifierPattern.MatchString(item.Name) || len(item.Request) == 0 {
			return nil, "", errors.New("frozen request vector identity or request is invalid")
		}
		if _, duplicate := seenNames[item.Name]; duplicate {
			return nil, "", fmt.Errorf("duplicate frozen request vector %q", item.Name)
		}
		seenNames[item.Name] = struct{}{}
		caseID := "request." + item.Name
		auditCase := AuditCase{
			SchemaVersion: CaseSchema, ProtocolVersion: CurrentVersion, CaseID: caseID,
			Level: LevelDeterministicReplay, Kind: CaseRequestFingerprint,
			Description:          "verifies frozen canonical request bytes, including evidence provenance bindings, without translating the request",
			RequiredCapabilities: []string{"sealed_replay"},
			Invocation: AuditInvocation{
				SchemaVersion: InvocationSchema, InvocationID: "invocation." + item.Name,
				Operation: CaseRequestFingerprint, Offline: true, TimeoutMillis: 1_000, MaxInputBytes: 1 << 20,
				TrajectoryRefs: []TrajectoryRef{},
				RequestVector:  &RequestVectorInput{VectorID: item.Name, CanonicalUTF8Hex: item.CanonicalUTF8Hex, ExpectedSHA256: item.Fingerprint},
				Extensions:     []Extension{},
			},
			Expected:   ExpectedOutcome{Status: ExpectationAccepted, EvidenceDigest: item.Fingerprint},
			Extensions: []Extension{},
		}
		caseRaw, err := CanonicalMarshal(auditCase)
		if err != nil {
			return nil, "", err
		}
		vectors = append(vectors, CaseVector{CaseID: caseID, Level: auditCase.Level, Kind: auditCase.Kind, Description: auditCase.Description, Expected: auditCase.Expected, Raw: caseRaw})
	}
	return vectors, DigestBytes(raw), nil
}

func appendVector(vectors *[]CaseVector, seen map[string]struct{}, caseID string, level ConformanceLevel, kind CaseKind, description string, expected ExpectedOutcome, raw []byte) error {
	if !identifierPattern.MatchString(caseID) {
		return fmt.Errorf("vector case ID %q is invalid", caseID)
	}
	if _, duplicate := seen[caseID]; duplicate {
		return fmt.Errorf("duplicate vector case ID %q", caseID)
	}
	seen[caseID] = struct{}{}
	*vectors = append(*vectors, CaseVector{
		CaseID: caseID, Level: level, Kind: kind, Description: description,
		Expected: expected, Raw: append([]byte(nil), raw...),
	})
	return nil
}

func applyJSONMutation(base []byte, mutation MutationVector) ([]byte, error) {
	var document any
	decoder := json.NewDecoder(strings.NewReader(string(base)))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	tokens, err := jsonPointerTokens(mutation.JSONPointer)
	if err != nil || len(tokens) == 0 {
		return nil, errors.New("mutation JSON pointer is invalid")
	}
	var value any
	if mutation.Operation == "replace" || mutation.Operation == "add" {
		valueDecoder := json.NewDecoder(strings.NewReader(string(mutation.Value)))
		valueDecoder.UseNumber()
		if err := valueDecoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode mutation value: %w", err)
		}
	}
	if err := mutateJSONDocument(document, tokens, mutation.Operation, value); err != nil {
		return nil, err
	}
	return CanonicalMarshal(document)
}

func mutateJSONDocument(document any, tokens []string, operation string, value any) error {
	if len(tokens) == 0 {
		return errors.New("mutation path is empty")
	}
	if len(tokens) > 1 {
		next, err := mutationChild(document, tokens[0])
		if err != nil {
			return err
		}
		return mutateJSONDocument(next, tokens[1:], operation, value)
	}
	leaf := tokens[0]
	switch container := document.(type) {
	case map[string]any:
		return mutateJSONObject(container, leaf, operation, value)
	case []any:
		index, err := mutationArrayIndex(leaf, len(container))
		if err != nil {
			return err
		}
		if operation == "remove" {
			return errors.New("array-element removal is not supported by normative mutations")
		}
		if operation != "replace" {
			return fmt.Errorf("unsupported array mutation operation %q", operation)
		}
		container[index] = value
		return nil
	default:
		return fmt.Errorf("mutation path component %q is not a container", leaf)
	}
}

func mutateJSONObject(object map[string]any, leaf, operation string, value any) error {
	switch operation {
	case "remove":
		if _, exists := object[leaf]; !exists {
			return fmt.Errorf("mutation remove target %q does not exist", leaf)
		}
		delete(object, leaf)
	case "replace", "add":
		if operation == "replace" {
			if _, exists := object[leaf]; !exists {
				return fmt.Errorf("mutation replace target %q does not exist", leaf)
			}
		}
		object[leaf] = value
	default:
		return fmt.Errorf("unsupported mutation operation %q", operation)
	}
	return nil
}

func mutationChild(document any, token string) (any, error) {
	switch container := document.(type) {
	case map[string]any:
		child, ok := container[token]
		if !ok {
			return nil, fmt.Errorf("mutation path component %q does not exist", token)
		}
		return child, nil
	case []any:
		index, err := mutationArrayIndex(token, len(container))
		if err != nil {
			return nil, err
		}
		return container[index], nil
	default:
		return nil, fmt.Errorf("mutation path component %q is not a container", token)
	}
}

func mutationArrayIndex(token string, length int) (int, error) {
	if token == "" || len(token) > 1 && token[0] == '0' {
		return 0, fmt.Errorf("mutation array index %q is not canonical", token)
	}
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 || index >= length {
		return 0, fmt.Errorf("mutation array index %q is out of bounds", token)
	}
	return index, nil
}

func jsonPointerTokens(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("JSON pointer must begin with slash")
	}
	parts := strings.Split(pointer[1:], "/")
	for index, part := range parts {
		var decoded strings.Builder
		for offset := 0; offset < len(part); offset++ {
			if part[offset] != '~' {
				decoded.WriteByte(part[offset])
				continue
			}
			if offset+1 >= len(part) || part[offset+1] != '0' && part[offset+1] != '1' {
				return nil, errors.New("JSON pointer contains invalid escape")
			}
			if part[offset+1] == '0' {
				decoded.WriteByte('~')
			} else {
				decoded.WriteByte('/')
			}
			offset++
		}
		parts[index] = decoded.String()
	}
	return parts, nil
}

func embeddedSchemaDigest() (string, error) {
	entries, err := protocolArtifacts.ReadDir("schemas")
	if err != nil {
		return "", err
	}
	material := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := protocolArtifacts.ReadFile("schemas/" + entry.Name())
		if err != nil {
			return "", err
		}
		material[entry.Name()] = DigestBytes(raw)
	}
	return Digest(material)
}
