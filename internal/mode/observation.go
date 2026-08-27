package mode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Christopher-Schulze/evalwitness/internal/provider"
	"github.com/Christopher-Schulze/evalwitness/internal/verifier"
)

const ScoreObservationSchemaVersion = "evalwitness.score-call-observation.v1"

const (
	ExtractionObservationComplete = "complete"
	ExtractionObservationRejected = "rejected"
)

// ScoreObservation exposes digest-only execution evidence to audit layers.
// Provider bodies and raw model text never cross this boundary.
type ScoreObservation struct {
	SchemaVersion          string                      `json:"schema_version"`
	Scope                  string                      `json:"scope"`
	CriterionID            string                      `json:"criterion_id"`
	SamplingSlot           string                      `json:"sampling_slot"`
	Entrypoint             string                      `json:"entrypoint"`
	RequestFingerprint     provider.Fingerprint        `json:"request_fingerprint"`
	ResponseBodyDigest     string                      `json:"response_body_digest"`
	ParsedPayloadDigest    string                      `json:"parsed_payload_digest"`
	ResponseEvidenceDigest string                      `json:"response_evidence_digest"`
	ScoreEvidenceDigest    string                      `json:"score_evidence_digest"`
	ReplayStatus           provider.ReplayStatus       `json:"replay_status"`
	ReplaySource           *provider.ExactReplaySource `json:"replay_source,omitempty"`
	ProviderRequestID      string                      `json:"provider_request_id,omitempty"`
	ExtractionStatus       string                      `json:"extraction_status"`
	ExtractionErrorDigest  string                      `json:"extraction_error_digest,omitempty"`
}

type ScoreObserver func(ScoreObservation) error

type scoreObserverKey struct{}
type observationScopeKey struct{}

func ContextWithScoreObserver(ctx context.Context, observer ScoreObserver) context.Context {
	return context.WithValue(ctx, scoreObserverKey{}, observer)
}

func ContextWithObservationScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, observationScopeKey{}, scope)
}

func (value ScoreObservation) Validate() error {
	if value.SchemaVersion != ScoreObservationSchemaVersion || !validObservationDigest(value.Scope) || strings.TrimSpace(value.CriterionID) == "" ||
		strings.TrimSpace(value.SamplingSlot) == "" || strings.TrimSpace(value.Entrypoint) == "" ||
		!validObservationDigest(string(value.RequestFingerprint)) || !validObservationDigest(value.ResponseBodyDigest) ||
		!validObservationDigest(value.ParsedPayloadDigest) || !validObservationDigest(value.ResponseEvidenceDigest) ||
		!validObservationDigest(value.ScoreEvidenceDigest) || value.ReplayStatus != provider.ReplayStatusLive && value.ReplayStatus != provider.ReplayStatusExact {
		return errors.New("score observation identity, replay status, or evidence digest is invalid")
	}
	if value.ReplayStatus == provider.ReplayStatusLive && value.ReplaySource != nil {
		return errors.New("live score observation carries an exact replay source")
	}
	if value.ReplaySource != nil {
		if err := value.ReplaySource.Validate(); err != nil {
			return err
		}
	}
	switch value.ExtractionStatus {
	case ExtractionObservationComplete:
		if value.ExtractionErrorDigest != "" {
			return errors.New("complete score observation carries an extraction error")
		}
	case ExtractionObservationRejected:
		if !validObservationDigest(value.ExtractionErrorDigest) {
			return errors.New("rejected score observation lacks an extraction-error digest")
		}
	default:
		return errors.New("score observation extraction status is invalid")
	}
	return nil
}

func observeScore(ctx context.Context, criterionID string, request provider.RequestEnvelope, response provider.ResponseRecord, evidence map[string]verifier.ScoreEvidence, replayStatus provider.ReplayStatus, replaySource *provider.ExactReplaySource, extractionErr error) error {
	observer, ok := ctx.Value(scoreObserverKey{}).(ScoreObserver)
	if !ok || observer == nil {
		return nil
	}
	evidenceDigest, err := digestScoreEvidence(evidence)
	if err != nil {
		return err
	}
	requestFingerprint, err := request.Fingerprint()
	if err != nil {
		return fmt.Errorf("fingerprint score observation request: %w", err)
	}
	status := ExtractionObservationComplete
	errorDigest := ""
	if extractionErr != nil {
		status = ExtractionObservationRejected
		digest := sha256.Sum256([]byte(extractionErr.Error()))
		errorDigest = hex.EncodeToString(digest[:])
	}
	scope, ok := ctx.Value(observationScopeKey{}).(string)
	if !ok {
		return errors.New("score observation scope is unavailable")
	}
	observation := ScoreObservation{
		SchemaVersion: ScoreObservationSchemaVersion, Scope: scope, CriterionID: criterionID,
		SamplingSlot: request.Lineage.SamplingSlot, Entrypoint: request.Lineage.Entrypoint,
		RequestFingerprint: requestFingerprint, ResponseBodyDigest: response.ResponseBodyDigest,
		ParsedPayloadDigest: response.ParsedPayloadDigest, ResponseEvidenceDigest: response.EvidenceDigest,
		ScoreEvidenceDigest: evidenceDigest, ReplayStatus: replayStatus, ReplaySource: replaySource,
		ProviderRequestID: response.ProviderRequestID, ExtractionStatus: status,
		ExtractionErrorDigest: errorDigest,
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	return observer(observation)
}

func digestScoreEvidence(evidence map[string]verifier.ScoreEvidence) (string, error) {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("encode score observation evidence: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validObservationDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}
