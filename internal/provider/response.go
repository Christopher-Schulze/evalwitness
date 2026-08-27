package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	CaptureSchemaVersion   = 3
	ParserContractVersion  = "evalwitness.score-token-parser.v2"
	ResponseStatusComplete = "complete"
)

type ReplayStatus string

const (
	ReplayStatusLive     ReplayStatus = "live"
	ReplayStatusExact    ReplayStatus = "exact"
	ReplayStatusLegacy   ReplayStatus = "legacy"
	ReplayStatusMiss     ReplayStatus = "miss"
	ReplayStatusRejected ReplayStatus = "rejected"
)

type ReplayLookupError struct {
	Status ReplayStatus
	Reason string
}

func (e *ReplayLookupError) Error() string {
	return fmt.Sprintf("replay %s: %s", e.Status, e.Reason)
}

type TokenUsage struct {
	Input     int `json:"input"`
	Output    int `json:"output"`
	Cached    int `json:"cached"`
	Reasoning int `json:"reasoning,omitempty"`
}

type TokenAlternative struct {
	Token   string `json:"token"`
	Logprob string `json:"logprob"`
}

type TokenEvidence struct {
	Position        int                `json:"position"`
	Token           string             `json:"token"`
	Logprob         string             `json:"logprob"`
	TopAlternatives []TokenAlternative `json:"top_alternatives"`
}

type ResponseRecord struct {
	CaptureSchemaVersion    int                           `json:"capture_schema_version"`
	RequestFingerprint      Fingerprint                   `json:"request_fingerprint"`
	ProviderID              string                        `json:"provider_id"`
	RouteID                 string                        `json:"route_id"`
	RequestedModel          string                        `json:"requested_model"`
	ServedModel             string                        `json:"served_model,omitempty"`
	CheckpointAssertion     string                        `json:"checkpoint_assertion,omitempty"`
	ProviderRequestID       string                        `json:"provider_request_id,omitempty"`
	ReceivedAt              int64                         `json:"received_at"`
	Status                  string                        `json:"status"`
	FinishReason            string                        `json:"finish_reason,omitempty"`
	Usage                   TokenUsage                    `json:"usage"`
	UsageObserved           bool                          `json:"usage_observed,omitempty"`
	NormalizedBody          []byte                        `json:"normalized_body"`
	ResponseBodyDigest      string                        `json:"response_body_digest"`
	ParsedPayloadDigest     string                        `json:"parsed_payload_digest"`
	EvidenceDigest          string                        `json:"evidence_digest"`
	ParserContractVersion   string                        `json:"parser_contract_version"`
	ReplayStatus            ReplayStatus                  `json:"replay_status"`
	ReplayReason            string                        `json:"replay_reason,omitempty"`
	CapabilityAttestationID string                        `json:"capability_attestation_id,omitempty"`
	AuditLineage            RequestLineage                `json:"audit_lineage"`
	Distributions           map[string]map[string]float64 `json:"distributions"`
	RawText                 string                        `json:"raw_text"`
	HasLogprobs             bool                          `json:"has_logprobs"`
	ObservedTopLogprobs     int                           `json:"observed_top_logprobs"`
	DegenerateLogprobs      bool                          `json:"degenerate_logprobs"`
	OrderedTokenEvidence    []TokenEvidence               `json:"ordered_token_evidence"`
}

func FinalizeResponse(request RequestEnvelope, response ResponseRecord) (ResponseRecord, error) {
	fingerprint, err := request.Fingerprint()
	if err != nil {
		return ResponseRecord{}, err
	}
	response.CaptureSchemaVersion = CaptureSchemaVersion
	response.RequestFingerprint = fingerprint
	response.ProviderID = request.ProviderID
	response.RouteID = request.RouteID()
	response.RequestedModel = request.RequestedModel
	if response.ReceivedAt == 0 {
		response.ReceivedAt = time.Now().UnixMilli()
	}
	if response.Status == "" {
		response.Status = ResponseStatusComplete
	}
	response.AuditLineage = request.Lineage
	response.ParserContractVersion = ParserContractVersion
	if response.ReplayStatus == "" {
		response.ReplayStatus = ReplayStatusLive
	}
	if response.Distributions == nil {
		response.Distributions = map[string]map[string]float64{}
	}
	if response.NormalizedBody == nil {
		response.NormalizedBody = []byte{}
	}
	if response.OrderedTokenEvidence == nil {
		response.OrderedTokenEvidence = []TokenEvidence{}
	}
	response.ResponseBodyDigest = digestBytes(response.NormalizedBody)
	response.ParsedPayloadDigest, err = response.parsedDigest()
	if err != nil {
		return ResponseRecord{}, err
	}
	response.EvidenceDigest, err = response.evidenceDigest()
	if err != nil {
		return ResponseRecord{}, err
	}
	if err := response.ValidateExact(request); err != nil {
		return ResponseRecord{}, err
	}
	return response, nil
}

func (r ResponseRecord) ValidateExact(request RequestEnvelope) error {
	fingerprint, err := request.Fingerprint()
	if err != nil {
		return err
	}
	switch {
	case r.CaptureSchemaVersion != CaptureSchemaVersion:
		return fmt.Errorf("response capture schema version %d is unsupported", r.CaptureSchemaVersion)
	case r.RequestFingerprint == "":
		return errors.New("response request_fingerprint is required")
	case r.RequestFingerprint != fingerprint:
		return fmt.Errorf("response request fingerprint mismatch: got %s want %s", r.RequestFingerprint, fingerprint)
	case r.ProviderID != request.ProviderID:
		return errors.New("response provider_id does not match request")
	case r.RouteID != request.RouteID():
		return errors.New("response route_id does not match request")
	case r.RequestedModel != request.RequestedModel:
		return errors.New("response requested_model does not match request")
	case r.Status != ResponseStatusComplete:
		return fmt.Errorf("response status %q is not complete", r.Status)
	case r.ParserContractVersion != ParserContractVersion:
		return fmt.Errorf("response parser contract %q is unsupported", r.ParserContractVersion)
	case r.ReplayStatus != ReplayStatusLive && r.ReplayStatus != ReplayStatusExact:
		return fmt.Errorf("response replay status %q is not exact evidence", r.ReplayStatus)
	case r.ResponseBodyDigest == "" || r.ResponseBodyDigest != digestBytes(r.NormalizedBody):
		return errors.New("response body checksum mismatch")
	}
	parsedDigest, err := r.parsedDigest()
	if err != nil {
		return err
	}
	if r.ParsedPayloadDigest == "" || r.ParsedPayloadDigest != parsedDigest {
		return errors.New("response parsed payload checksum mismatch")
	}
	evidenceDigest, err := r.evidenceDigest()
	if err != nil {
		return err
	}
	if r.EvidenceDigest == "" || r.EvidenceDigest != evidenceDigest {
		return errors.New("response evidence checksum mismatch")
	}
	if r.HasLogprobs && len(r.OrderedTokenEvidence) == 0 {
		return errors.New("response claims logprobs without ordered token evidence")
	}
	return nil
}

func (r ResponseRecord) evidenceDigest() (string, error) {
	r.EvidenceDigest = ""
	r.ReplayStatus = ""
	r.ReplayReason = ""
	encoded, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("encode response evidence: %w", err)
	}
	return digestBytes(encoded), nil
}

func (r ResponseRecord) parsedDigest() (string, error) {
	payload := struct {
		RawText              string                        `json:"raw_text"`
		Distributions        map[string]map[string]float64 `json:"distributions"`
		HasLogprobs          bool                          `json:"has_logprobs"`
		ObservedTopLogprobs  int                           `json:"observed_top_logprobs"`
		DegenerateLogprobs   bool                          `json:"degenerate_logprobs"`
		OrderedTokenEvidence []TokenEvidence               `json:"ordered_token_evidence"`
	}{
		RawText:              r.RawText,
		Distributions:        r.Distributions,
		HasLogprobs:          r.HasLogprobs,
		ObservedTopLogprobs:  r.ObservedTopLogprobs,
		DegenerateLogprobs:   r.DegenerateLogprobs,
		OrderedTokenEvidence: r.OrderedTokenEvidence,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode parsed response payload: %w", err)
	}
	return digestBytes(encoded), nil
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
