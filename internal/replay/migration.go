package replay

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
)

type MigrationReport struct {
	SourcePath      string                `json:"source_path"`
	CandidatePath   string                `json:"candidate_path"`
	SourceDigest    string                `json:"source_digest"`
	CandidateDigest string                `json:"candidate_digest"`
	Records         int                   `json:"records"`
	ReplayStatus    provider.ReplayStatus `json:"replay_status"`
	AmbiguousFields []string              `json:"ambiguous_fields"`
}

type legacyFixtureEntry struct {
	Hash     string          `json:"hash"`
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Response json.RawMessage `json:"response"`
}

type legacyInspectionRecord struct {
	InspectionSchemaVersion int                   `json:"inspection_schema_version"`
	ReplayStatus            provider.ReplayStatus `json:"replay_status"`
	SourceLine              int                   `json:"source_line"`
	SourceCaptureSchema     int                   `json:"source_capture_schema,omitempty"`
	LegacyHash              string                `json:"legacy_hash,omitempty"`
	Provider                string                `json:"provider"`
	Model                   string                `json:"model"`
	Request                 json.RawMessage       `json:"request,omitempty"`
	Response                json.RawMessage       `json:"response"`
	AmbiguousFields         []string              `json:"ambiguous_fields"`
}

var legacyAmbiguousFields = []string{
	"base_url_origin",
	"endpoint_path",
	"thinking_mode",
	"messages",
	"temperature",
	"seed",
	"max_output_tokens",
	"logprobs",
	"top_logprobs",
	"stop",
	"score_tags",
	"response_format",
	"stream",
	"prompt_builder_version",
	"logit_bias",
	"sampling_slot",
}

var scoreEvidenceAmbiguousFields = []string{
	"score_evidence_schema_version",
	"aligned_token_position",
	"returned_top_k_at_aligned_position",
	"visible_probability_mass",
	"valid_score_mass",
	"unobserved_probability_mass",
	"raw_alternative_rank_and_provenance",
	"decision_policy_version",
}

func MigrateLegacy(sourcePath, candidatePath string) (report MigrationReport, returnErr error) {
	report = MigrationReport{SourcePath: sourcePath, CandidatePath: candidatePath, ReplayStatus: provider.ReplayStatusLegacy}
	if sourcePath == "" || candidatePath == "" {
		return report, errors.New("legacy migration requires source and candidate paths")
	}
	if sourcePath == candidatePath {
		return report, errors.New("legacy migration candidate must differ from source")
	}
	if _, err := os.Lstat(candidatePath); err == nil {
		return report, errors.New("legacy migration candidate already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return report, err
	}
	if err := os.MkdirAll(filepath.Dir(candidatePath), safety.SensitiveDirectoryMode); err != nil {
		return report, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return report, err
	}
	sourceClosed := false
	defer func() {
		if !sourceClosed {
			if closeErr := source.Close(); closeErr != nil && returnErr == nil {
				returnErr = fmt.Errorf("close legacy source: %w", closeErr)
			}
		}
	}()
	temporary, err := os.CreateTemp(filepath.Dir(candidatePath), "."+filepath.Base(candidatePath)+".tmp-*")
	if err != nil {
		return report, err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Chmod(safety.SensitiveFileMode); err != nil {
		_ = temporary.Close()
		return report, err
	}
	sourceHash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(source, sourceHash))
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	writer := bufio.NewWriterSize(temporary, 256*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var envelope struct {
			CaptureSchemaVersion int             `json:"capture_schema_version"`
			Request              json.RawMessage `json:"request"`
			Response             json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			_ = temporary.Close()
			return report, fmt.Errorf("legacy line %d: %w; partial candidate retained at %s", lineNumber, err, temporaryPath)
		}
		record, ambiguous, err := inspectLegacyLine(scanner.Bytes(), envelope)
		if err != nil {
			_ = temporary.Close()
			return report, fmt.Errorf("legacy line %d: %w; partial candidate retained at %s", lineNumber, err, temporaryPath)
		}
		record.InspectionSchemaVersion = 1
		record.ReplayStatus = provider.ReplayStatusLegacy
		record.SourceLine = lineNumber
		record.AmbiguousFields = append([]string(nil), ambiguous...)
		encoded, err := json.Marshal(record)
		if err != nil {
			_ = temporary.Close()
			return report, err
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			_ = temporary.Close()
			return report, err
		}
		report.Records++
		report.AmbiguousFields = appendUniqueFields(report.AmbiguousFields, ambiguous...)
	}
	if err := scanner.Err(); err != nil {
		_ = temporary.Close()
		return report, err
	}
	closeErr := source.Close()
	sourceClosed = true
	if closeErr != nil {
		_ = temporary.Close()
		return report, fmt.Errorf("close legacy source: %w", closeErr)
	}
	sourceDigest := hex.EncodeToString(sourceHash.Sum(nil))
	currentSourceDigest, sourceExists, err := fileDigest(sourcePath)
	if err != nil || !sourceExists || currentSourceDigest != sourceDigest {
		_ = temporary.Close()
		return report, fmt.Errorf("legacy source changed during migration; partial candidate retained at %s", temporaryPath)
	}
	if err := writer.Flush(); err != nil {
		_ = temporary.Close()
		return report, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return report, err
	}
	if err := temporary.Close(); err != nil {
		return report, err
	}
	candidateDigest, _, err := fileDigest(temporaryPath)
	if err != nil {
		return report, err
	}
	if err := os.Link(temporaryPath, candidatePath); err != nil {
		return report, fmt.Errorf("publish migration candidate: %w; temporary retained at %s", err, temporaryPath)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return report, fmt.Errorf("remove published migration temporary %s: %w", temporaryPath, err)
	}
	directory, err := os.Open(filepath.Dir(candidatePath))
	if err != nil {
		return report, err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return report, err
	}
	if err := directory.Close(); err != nil {
		return report, err
	}
	report.SourceDigest = sourceDigest
	report.CandidateDigest = candidateDigest
	return report, nil
}

func inspectLegacyLine(raw []byte, envelope struct {
	CaptureSchemaVersion int             `json:"capture_schema_version"`
	Request              json.RawMessage `json:"request"`
	Response             json.RawMessage `json:"response"`
}) (legacyInspectionRecord, []string, error) {
	if envelope.CaptureSchemaVersion > 0 {
		if envelope.CaptureSchemaVersion >= provider.CaptureSchemaVersion {
			return legacyInspectionRecord{}, nil, fmt.Errorf("capture schema %d is current or newer, not legacy", envelope.CaptureSchemaVersion)
		}
		var request provider.RequestEnvelope
		if len(envelope.Request) == 0 || len(envelope.Response) == 0 || json.Unmarshal(envelope.Request, &request) != nil {
			return legacyInspectionRecord{}, nil, errors.New("versioned legacy fixture lacks a readable request or response")
		}
		return legacyInspectionRecord{
			SourceCaptureSchema: envelope.CaptureSchemaVersion,
			Provider:            request.ProviderID,
			Model:               request.RequestedModel,
			Request:             append(json.RawMessage(nil), envelope.Request...),
			Response:            append(json.RawMessage(nil), envelope.Response...),
		}, scoreEvidenceAmbiguousFields, nil
	}
	var legacy legacyFixtureEntry
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return legacyInspectionRecord{}, nil, err
	}
	if len(legacy.Hash) != sha256.Size*2 {
		return legacyInspectionRecord{}, nil, errors.New("invalid prompt hash")
	}
	if _, err := hex.DecodeString(legacy.Hash); err != nil {
		return legacyInspectionRecord{}, nil, errors.New("invalid prompt hash")
	}
	return legacyInspectionRecord{
		LegacyHash: legacy.Hash,
		Provider:   legacy.Provider,
		Model:      legacy.Model,
		Response:   append(json.RawMessage(nil), legacy.Response...),
	}, legacyAmbiguousFields, nil
}

func appendUniqueFields(fields []string, additions ...string) []string {
	for _, addition := range additions {
		found := false
		for _, field := range fields {
			if field == addition {
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, addition)
		}
	}
	return fields
}
