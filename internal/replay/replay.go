// Package replay provides deterministic test/development modes:
// fixture-driven scoring (EVALWITNESS_REPLAY_FROM) and live-call capture
// (EVALWITNESS_REPLAY_TO).
package replay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/safety"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

type fixtureEntry struct {
	CaptureSchemaVersion int                               `json:"capture_schema_version"`
	Request              provider.RequestEnvelope          `json:"request"`
	Response             provider.ResponseRecord           `json:"response"`
	ScoreEvidence        map[string]verifier.ScoreEvidence `json:"score_evidence"`
	RecordDigest         string                            `json:"record_digest"`
}

const replayEvidenceFloatTolerance = 1e-12

func fixtureKey(request provider.RequestEnvelope) (string, error) {
	fingerprint, err := request.Fingerprint()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(fingerprint))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(request.Lineage.SamplingSlot))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func newFixtureEntry(request provider.RequestEnvelope, response provider.ResponseRecord) (fixtureEntry, error) {
	if err := response.ValidateExact(request); err != nil {
		return fixtureEntry{}, err
	}
	entry := fixtureEntry{
		CaptureSchemaVersion: provider.CaptureSchemaVersion,
		Request:              request,
		Response:             response,
		ScoreEvidence:        verifier.ExtractAllScoreEvidence(request, response, replayExtractionMode(request)),
	}
	entry.Request.BeforeAttempt = nil
	entry.Request.AfterAttempt = nil
	if err := validateReplayEvidence(entry.Request, entry.ScoreEvidence); err != nil {
		return fixtureEntry{}, fmt.Errorf("score evidence: %w", err)
	}
	digest, err := entryDigest(entry)
	if err != nil {
		return fixtureEntry{}, err
	}
	entry.RecordDigest = digest
	return entry, nil
}

func entryDigest(entry fixtureEntry) (string, error) {
	canonicalRequest, err := entry.Request.CanonicalBytes()
	if err != nil {
		return "", err
	}
	lineage, err := json.Marshal(entry.Request.Lineage)
	if err != nil {
		return "", fmt.Errorf("encode request lineage: %w", err)
	}
	h := sha256.New()
	_, _ = h.Write(canonicalRequest)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(lineage)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(entry.Response.EvidenceDigest))
	_, _ = h.Write([]byte{0})
	evidence, err := json.Marshal(entry.ScoreEvidence)
	if err != nil {
		return "", fmt.Errorf("encode score evidence: %w", err)
	}
	_, _ = h.Write(evidence)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReplayProvider serves Score requests from a JSONL fixture; cache miss is an error.
type ReplayProvider struct {
	name    string
	model   string
	caps    provider.Capabilities
	entries map[string]fixtureEntry
	source  provider.ExactReplaySource
}

func LoadReplay(path, name, model string, caps provider.Capabilities) (*ReplayProvider, error) {
	payload, err := readReplaySource(path)
	if err != nil {
		return nil, err
	}
	rp := &ReplayProvider{
		name:    name,
		model:   model,
		caps:    caps,
		entries: map[string]fixtureEntry{},
	}
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e fixtureEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("replay parse: %w", err)
		}
		key, err := validateFixtureEntry(e, name, model)
		if err != nil {
			return nil, err
		}
		if _, exists := rp.entries[key]; exists {
			return nil, fmt.Errorf("replay fixture has duplicate request and sampling slot %s", key[:16])
		}
		rp.entries[key] = e
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rp.entries) > 0 {
		inspection, _, err := inspectCapturePayload(payload)
		if err != nil {
			return nil, fmt.Errorf("inspect replay source evidence: %w", err)
		}
		recordDigests := make([]string, 0, len(rp.entries))
		for _, entry := range rp.entries {
			recordDigests = append(recordDigests, entry.RecordDigest)
		}
		loadedRecordSetDigest, err := digestStringMultiset(recordDigests)
		if err != nil {
			return nil, err
		}
		if inspection.ProviderID != name || inspection.RequestedModel != model || inspection.Entries != len(rp.entries) ||
			inspection.RecordSetDigest != loadedRecordSetDigest {
			return nil, errors.New("replay fixture entries differ from their capture-source inspection")
		}
		rp.source = exactReplaySourceFromInspection(inspection)
	}
	return rp, nil
}

func readReplaySource(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(err, errors.New("replay source is not a regular file"))
	}
	if info.Size() > 0 {
		return readCaptureSource(path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	var probe [1]byte
	readBytes, readErr := file.Read(probe[:])
	final, finalStatErr := file.Stat()
	closeErr := file.Close()
	pathInfo, pathErr := os.Lstat(path)
	if statErr != nil || readErr != io.EOF || readBytes != 0 || finalStatErr != nil || closeErr != nil || pathErr != nil ||
		!pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(info, opened) || !os.SameFile(opened, final) || !os.SameFile(final, pathInfo) ||
		opened.Size() != 0 || final.Size() != 0 || pathInfo.Size() != 0 ||
		!info.ModTime().Equal(opened.ModTime()) || !opened.ModTime().Equal(final.ModTime()) || !final.ModTime().Equal(pathInfo.ModTime()) {
		return nil, errors.Join(statErr, readErr, finalStatErr, closeErr, pathErr, errors.New("empty replay source changed during read"))
	}
	return []byte{}, nil
}

func validateFixtureEntry(entry fixtureEntry, name, model string) (string, error) {
	if entry.CaptureSchemaVersion != provider.CaptureSchemaVersion {
		return "", fmt.Errorf("replay fixture is legacy or unsupported schema %d; inspect it with `evalwitness replay migrate`", entry.CaptureSchemaVersion)
	}
	if entry.Request.ProviderID != name || entry.Request.RequestedModel != model {
		return "", fmt.Errorf("replay fixture route mismatch: record is %s/%s, loader requested %s/%s", entry.Request.ProviderID, entry.Request.RequestedModel, name, model)
	}
	if err := entry.Response.ValidateExact(entry.Request); err != nil {
		return "", fmt.Errorf("replay response validation: %w", err)
	}
	expectedEvidence := verifier.ExtractAllScoreEvidence(entry.Request, entry.Response, replayExtractionMode(entry.Request))
	if !replayEvidenceEquivalent(entry.ScoreEvidence, expectedEvidence) {
		return "", errors.New("replay fixture score evidence does not match the captured token stream")
	}
	if err := validateReplayEvidence(entry.Request, entry.ScoreEvidence); err != nil {
		return "", fmt.Errorf("replay fixture score evidence: %w", err)
	}
	digest, err := entryDigest(entry)
	if err != nil || entry.RecordDigest == "" || entry.RecordDigest != digest {
		return "", errors.New("replay fixture record checksum mismatch")
	}
	return fixtureKey(entry.Request)
}

func replayEvidenceEquivalent(left, right map[string]verifier.ScoreEvidence) bool {
	if len(left) != len(right) {
		return false
	}
	for tag, leftItem := range left {
		rightItem, ok := right[tag]
		if !ok || !scoreEvidenceEquivalent(leftItem, rightItem) {
			return false
		}
	}
	return true
}

func scoreEvidenceEquivalent(left, right verifier.ScoreEvidence) bool {
	if !scoreEvidenceFloatsEquivalent(left, right) {
		return false
	}
	clearScoreEvidenceFloats(&left)
	clearScoreEvidenceFloats(&right)
	return reflect.DeepEqual(left, right)
}

func scoreEvidenceFloatsEquivalent(left, right verifier.ScoreEvidence) bool {
	if !replayFloatEqual(left.VisibleProbabilityMass, right.VisibleProbabilityMass) ||
		!replayFloatEqual(left.ValidScoreMass, right.ValidScoreMass) ||
		!replayFloatEqual(left.UnobservedProbabilityMass, right.UnobservedProbabilityMass) ||
		!replayFloatPointerEqual(left.ConditionalExpectedScore, right.ConditionalExpectedScore) ||
		!replayFloatPointerEqual(left.ConditionalVariance, right.ConditionalVariance) ||
		len(left.Support) != len(right.Support) || len(left.Alternatives) != len(right.Alternatives) {
		return false
	}
	for index := range left.Support {
		if !replayFloatEqual(left.Support[index].Value, right.Support[index].Value) ||
			!replayFloatEqual(left.Support[index].Probability, right.Support[index].Probability) {
			return false
		}
	}
	for index := range left.Alternatives {
		if !replayFloatPointerEqual(left.Alternatives[index].Probability, right.Alternatives[index].Probability) ||
			!replayFloatPointerEqual(left.Alternatives[index].CanonicalValue, right.Alternatives[index].CanonicalValue) {
			return false
		}
	}
	return true
}

func clearScoreEvidenceFloats(value *verifier.ScoreEvidence) {
	value.Support = append([]verifier.ScoreSupport(nil), value.Support...)
	value.Alternatives = append([]verifier.VisibleAlternative(nil), value.Alternatives...)
	value.VisibleProbabilityMass = 0
	value.ValidScoreMass = 0
	value.UnobservedProbabilityMass = 0
	value.ConditionalExpectedScore = zeroFloatPointer(value.ConditionalExpectedScore)
	value.ConditionalVariance = zeroFloatPointer(value.ConditionalVariance)
	for index := range value.Support {
		value.Support[index].Value = 0
		value.Support[index].Probability = 0
	}
	for index := range value.Alternatives {
		value.Alternatives[index].Probability = zeroFloatPointer(value.Alternatives[index].Probability)
		value.Alternatives[index].CanonicalValue = zeroFloatPointer(value.Alternatives[index].CanonicalValue)
	}
}

func zeroFloatPointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	zero := 0.0
	return &zero
}

func replayFloatPointerEqual(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return replayFloatEqual(*left, *right)
}

func replayFloatEqual(left, right float64) bool {
	return left == right ||
		(!math.IsNaN(left) && !math.IsNaN(right) && !math.IsInf(left, 0) && !math.IsInf(right, 0) &&
			math.Abs(left-right) <= replayEvidenceFloatTolerance)
}

func (rp *ReplayProvider) Name() string                        { return rp.name }
func (rp *ReplayProvider) Capabilities() provider.Capabilities { return rp.caps }

func (rp *ReplayProvider) ExactReplaySource() (provider.ExactReplaySource, bool) {
	if rp.source.Validate() != nil {
		return provider.ExactReplaySource{}, false
	}
	return rp.source, true
}

func exactReplaySourceFromInspection(inspection CaptureInspection) provider.ExactReplaySource {
	return provider.ExactReplaySource{
		SchemaVersion: provider.ExactReplaySourceSchemaVersion,
		CaptureSHA256: inspection.PayloadSHA256, Bytes: inspection.Bytes, Records: inspection.Entries,
		CaptureSchemaVersion: inspection.CaptureSchemaVersion, RequestSchemaVersion: inspection.RequestSchemaVersion,
		ParserContractVersion: inspection.ParserContractVersion,
		ProviderID:            inspection.ProviderID, RouteID: inspection.RouteID, RequestedModel: inspection.RequestedModel,
		RequestSetDigest: inspection.RequestSetDigest, LineageSetDigest: inspection.LineageSetDigest,
		ResponseBodySetDigest: inspection.ResponseBodySetDigest, EvidenceSetDigest: inspection.EvidenceSetDigest,
		RecordSetDigest: inspection.RecordSetDigest,
	}
}

func (rp *ReplayProvider) Score(_ context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	key, err := fixtureKey(req)
	if err != nil {
		return provider.ResponseRecord{}, &provider.ReplayLookupError{Status: provider.ReplayStatusRejected, Reason: err.Error()}
	}
	e, ok := rp.entries[key]
	if !ok {
		fingerprint, _ := req.Fingerprint()
		reason := fmt.Sprintf("no exact fixture for request fingerprint %s sampling slot %q", fingerprint, req.Lineage.SamplingSlot)
		for _, candidate := range rp.entries {
			if candidate.Request.Lineage.SamplingSlot == req.Lineage.SamplingSlot {
				reason += "; closest exact fixture differs in: " + strings.Join(semanticDifferences(candidate.Request, req), ", ")
				break
			}
		}
		return provider.ResponseRecord{}, &provider.ReplayLookupError{
			Status: provider.ReplayStatusMiss,
			Reason: reason,
		}
	}
	if err := e.Response.ValidateExact(req); err != nil {
		return provider.ResponseRecord{}, &provider.ReplayLookupError{Status: provider.ReplayStatusRejected, Reason: err.Error()}
	}
	if req.Logprobs {
		if !e.Response.HasLogprobs {
			return provider.ResponseRecord{}, &provider.ReplayLookupError{Status: provider.ReplayStatusRejected, Reason: "request requires logprobs but response has none"}
		}
	}
	if err := validateReplayEvidence(req, e.ScoreEvidence); err != nil {
		return provider.ResponseRecord{}, &provider.ReplayLookupError{Status: provider.ReplayStatusRejected, Reason: err.Error()}
	}
	response := e.Response
	response.ReplayStatus = provider.ReplayStatusExact
	response.ReplayReason = "full request fingerprint, sampling slot, response body, and parsed payload matched"
	return response, nil
}

func replayExtractionMode(request provider.RequestEnvelope) verifier.ExtractionMode {
	if request.Logprobs {
		return verifier.ExtractionModeVerifier
	}
	return verifier.ExtractionModeJudge
}

func validateReplayEvidence(request provider.RequestEnvelope, evidence map[string]verifier.ScoreEvidence) error {
	if len(request.ScoreTags) == 0 {
		return nil
	}
	if request.Logprobs {
		return verifier.ValidateStrictEvidence(evidence)
	}
	return verifier.ValidateJudgeEvidence(evidence)
}

func semanticDifferences(expected, actual provider.RequestEnvelope) []string {
	differences := make([]string, 0, 18)
	compare := func(field string, equal bool) {
		if !equal {
			differences = append(differences, field)
		}
	}
	compare("schema_version", expected.SchemaVersion == actual.SchemaVersion)
	compare("provider_id", expected.ProviderID == actual.ProviderID)
	compare("base_url_origin", expected.BaseURLOrigin == actual.BaseURLOrigin)
	compare("endpoint_path", expected.EndpointPath == actual.EndpointPath)
	compare("endpoint_kind", expected.EndpointKind == actual.EndpointKind)
	compare("requested_model", expected.RequestedModel == actual.RequestedModel)
	compare("thinking_mode", expected.ThinkingMode == actual.ThinkingMode)
	compare("messages", reflect.DeepEqual(expected.Messages, actual.Messages))
	compare("temperature", expected.Temperature == actual.Temperature)
	compare("seed", reflect.DeepEqual(expected.Seed, actual.Seed))
	compare("max_output_tokens", expected.MaxOutputTokens == actual.MaxOutputTokens)
	compare("logprobs", expected.Logprobs == actual.Logprobs)
	compare("top_logprobs", expected.TopLogprobs == actual.TopLogprobs)
	compare("stop", reflect.DeepEqual(expected.Stop, actual.Stop))
	compare("score_tags", reflect.DeepEqual(expected.ScoreTags, actual.ScoreTags))
	compare("response_format", expected.ResponseFormat == actual.ResponseFormat)
	compare("stream", expected.Stream == actual.Stream)
	compare("prompt_builder_version", expected.PromptBuilderVersion == actual.PromptBuilderVersion)
	compare("logit_bias", reflect.DeepEqual(expected.LogitBias, actual.LogitBias))
	if len(differences) == 0 {
		return []string{"none (fixture key or canonicalization contract mismatch)"}
	}
	return differences
}

// CapturingProvider stages exact request/response records and publishes the
// complete JSONL fixture only after checked finalization.
type CapturingProvider struct {
	inner           provider.Provider
	innerModel      string
	metadata        CaptureMetadata
	path            string
	mu              sync.Mutex
	file            *os.File
	writer          *bufio.Writer
	candidatePath   string
	originalDigest  string
	originalExisted bool
	closed          bool
}

// CaptureMetadata binds the current capability attestation to every response
// before the response and record digests are finalized.
type CaptureMetadata struct {
	CapabilityAttestationID string
	ServedIdentityPolicy    string
	ExpectedServedModel     string
	ExpectedServedModels    []string
	CheckpointAssertion     string
}

func WrapCapture(inner provider.Provider, model, path string, overwrite bool) (*CapturingProvider, error) {
	return wrapCapture(inner, model, path, overwrite, CaptureMetadata{})
}

func WrapResearchCapture(inner provider.Provider, model, path string, overwrite bool, metadata CaptureMetadata) (*CapturingProvider, error) {
	if !strings.HasPrefix(metadata.CapabilityAttestationID, "att-") {
		return nil, errors.New("research capture requires a capability attestation")
	}
	if err := provider.ValidateServedIdentityPolicy(metadata.ServedIdentityPolicy, metadata.ExpectedServedModel, metadata.ExpectedServedModels); err != nil {
		return nil, fmt.Errorf("research capture served identity policy: %w", err)
	}
	metadata.ExpectedServedModels = append([]string(nil), metadata.ExpectedServedModels...)
	return wrapCapture(inner, model, path, overwrite, metadata)
}

func wrapCapture(inner provider.Provider, model, path string, overwrite bool, metadata CaptureMetadata) (*CapturingProvider, error) {
	if path == "" {
		return nil, errors.New("capture: path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), safety.SensitiveDirectoryMode); err != nil {
		return nil, err
	}
	candidate, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".candidate-*")
	if err != nil {
		return nil, err
	}
	if err := candidate.Chmod(safety.SensitiveFileMode); err != nil {
		_ = candidate.Close()
		return nil, err
	}
	originalDigest, originalExisted, err := fileDigest(path)
	if err != nil {
		_ = candidate.Close()
		return nil, err
	}
	if !overwrite {
		original, openErr := os.Open(path)
		if openErr == nil {
			if _, err := LoadReplay(path, inner.Name(), model, inner.Capabilities()); err != nil {
				_ = original.Close()
				_ = candidate.Close()
				return nil, fmt.Errorf("capture append target: %w", err)
			}
			if _, err := io.Copy(candidate, original); err != nil {
				_ = original.Close()
				_ = candidate.Close()
				return nil, err
			}
			if err := original.Close(); err != nil {
				_ = candidate.Close()
				return nil, err
			}
		} else if !errors.Is(openErr, os.ErrNotExist) {
			_ = candidate.Close()
			return nil, openErr
		}
	}
	return &CapturingProvider{
		inner:           inner,
		innerModel:      model,
		metadata:        metadata,
		path:            path,
		file:            candidate,
		writer:          bufio.NewWriterSize(candidate, 256*1024),
		candidatePath:   candidate.Name(),
		originalDigest:  originalDigest,
		originalExisted: originalExisted,
	}, nil
}

func (cp *CapturingProvider) Name() string                        { return cp.inner.Name() }
func (cp *CapturingProvider) Capabilities() provider.Capabilities { return cp.inner.Capabilities() }

func (cp *CapturingProvider) CloseIdleConnections() {
	if closer, ok := cp.inner.(provider.IdleConnectionCloser); ok {
		closer.CloseIdleConnections()
	}
}

func (cp *CapturingProvider) Score(ctx context.Context, req provider.RequestEnvelope) (provider.ResponseRecord, error) {
	resp, err := cp.inner.Score(ctx, req)
	if err != nil {
		return resp, err
	}
	if cp.metadata.CapabilityAttestationID != "" {
		if err := provider.MatchServedIdentity(cp.metadata.ServedIdentityPolicy, cp.metadata.ExpectedServedModel, cp.metadata.ExpectedServedModels, resp.ServedModel); err != nil {
			return resp, fmt.Errorf("capture response served identity: %w", err)
		}
		resp.CapabilityAttestationID = cp.metadata.CapabilityAttestationID
		resp.CheckpointAssertion = cp.metadata.CheckpointAssertion
		resp, err = provider.FinalizeResponse(req, resp)
		if err != nil {
			return resp, fmt.Errorf("finalize research capture response: %w", err)
		}
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.closed {
		return resp, errors.New("capture response: capture is closed")
	}
	entry, err := newFixtureEntry(req, resp)
	if err != nil {
		return resp, fmt.Errorf("capture response: %w", err)
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return resp, fmt.Errorf("capture response: %w", err)
	}
	if _, err := cp.writer.Write(append(encoded, '\n')); err != nil {
		return resp, fmt.Errorf("capture response: %w", err)
	}
	return resp, nil
}

func (cp *CapturingProvider) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.closed {
		return nil
	}
	cp.closed = true
	if err := cp.writer.Flush(); err != nil {
		_ = cp.file.Close()
		return fmt.Errorf("capture flush: %w; candidate retained at %s", err, cp.candidatePath)
	}
	if err := cp.file.Sync(); err != nil {
		_ = cp.file.Close()
		return fmt.Errorf("capture sync: %w; candidate retained at %s", err, cp.candidatePath)
	}
	if err := cp.file.Close(); err != nil {
		return fmt.Errorf("capture close: %w; candidate retained at %s", err, cp.candidatePath)
	}
	if _, err := LoadReplay(cp.candidatePath, cp.inner.Name(), cp.innerModel, cp.inner.Capabilities()); err != nil {
		return fmt.Errorf("capture checksum: %w; candidate retained at %s", err, cp.candidatePath)
	}
	if err := cp.verifyOriginalUnchanged(); err != nil {
		return fmt.Errorf("capture publish: %w; candidate retained at %s", err, cp.candidatePath)
	}
	if err := os.Rename(cp.candidatePath, cp.path); err != nil {
		return fmt.Errorf("capture rename: %w; candidate retained at %s", err, cp.candidatePath)
	}
	directory, err := os.Open(filepath.Dir(cp.path))
	if err != nil {
		return fmt.Errorf("capture directory open: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("capture directory sync: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("capture directory close: %w", closeErr)
	}
	return nil
}

func (cp *CapturingProvider) verifyOriginalUnchanged() error {
	currentDigest, exists, err := fileDigest(cp.path)
	if err != nil {
		return err
	}
	if exists != cp.originalExisted || currentDigest != cp.originalDigest {
		return errors.New("capture target changed concurrently")
	}
	return nil
}

func fileDigest(path string) (string, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, file)
	closeErr := file.Close()
	if copyErr != nil {
		return "", false, copyErr
	}
	if closeErr != nil {
		return "", false, closeErr
	}
	return hex.EncodeToString(h.Sum(nil)), true, nil
}
